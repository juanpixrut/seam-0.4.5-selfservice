package accounting

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// AEByID returns an AE.
func (s *Service) AEByID(ctx context.Context, id int64) (AE, error) {
	d, err := s.store.Get(ctx, EntAE, id)
	if err != nil {
		return AE{}, err
	}
	return decode[AE](d)
}

// AEBySlug returns a virtual AE by its slug ("seam_ae", "seam_web").
func (s *Service) AEBySlug(ctx context.Context, slug string) (AE, error) {
	docs, err := s.find(ctx, Query{Entity: EntAE, Where: []Cond{eq("slug", slug)}, Limit: 1})
	if err != nil {
		return AE{}, err
	}
	if len(docs) == 0 {
		return AE{}, ErrNotFound
	}
	return decode[AE](docs[0])
}

// AEByUser returns the AE linked to a login. ok is false when the user has
// not registered as an AE.
func (s *Service) AEByUser(ctx context.Context, userID int64) (ae AE, ok bool, err error) {
	if userID <= 0 {
		return AE{}, false, nil
	}
	docs, err := s.find(ctx, Query{Entity: EntAE, Where: []Cond{eq("user_id", userID)}, Limit: 1})
	if err != nil || len(docs) == 0 {
		return AE{}, false, err
	}
	ae, err = decode[AE](docs[0])
	return ae, err == nil, err
}

// ListAEs returns every AE: the two virtual ones first, then the real ones in
// registration order.
func (s *Service) ListAEs(ctx context.Context) ([]AE, error) {
	docs, err := s.find(ctx, Query{Entity: EntAE, Limit: 5000})
	if err != nil {
		return nil, err
	}
	aes, err := decodeAll[AE](docs)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(aes, func(i, j int) bool {
		if (aes[i].Kind == AEVirtual) != (aes[j].Kind == AEVirtual) {
			return aes[i].Kind == AEVirtual
		}
		return aes[i].ID < aes[j].ID
	})
	return aes, nil
}

// RegisterAE creates the AE record for a login with the account_executive
// role, in the registered state (section 6.1, step 1). It is idempotent: a
// user that already has an AE record gets it back untouched. A registered AE
// cannot operate until Seam authorizes it.
func (s *Service) RegisterAE(ctx context.Context, userID int64, name, email string) (AE, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if userID <= 0 {
		return AE{}, fmt.Errorf("accounting: a registered AE needs a user")
	}
	if ae, ok, err := s.AEByUser(ctx, userID); err != nil || ok {
		return ae, err
	}
	ae := AE{
		Kind: AEReal, Name: trim(name), Email: trim(email), UserID: userID,
		Status: AERegistered, CreatedAt: s.stamp(),
	}
	if ae.Name == "" {
		ae.Name = trim(email)
	}
	if err := ae.ValidateProfile(); err != nil {
		return AE{}, err
	}
	id, err := s.store.Create(ctx, EntAE, encode(ae))
	if err != nil {
		return AE{}, err
	}
	ae.ID = id
	return ae, nil
}

// AEProfile is the data an AE loads about itself (D-10).
type AEProfile struct {
	Name          string
	Email         string
	Countries     []string
	PayoutMethod  PayoutMethod
	PayoutDetails string
}

// SaveAEProfile stores what a real AE typed about itself. Authorization state,
// user link and account code are never touched here.
func (s *Service) SaveAEProfile(ctx context.Context, aeID int64, p AEProfile) (AE, error) {
	d, err := s.store.Get(ctx, EntAE, aeID)
	if err != nil {
		return AE{}, err
	}
	ae, err := decode[AE](d)
	if err != nil {
		return AE{}, err
	}
	if ae.Kind != AEReal {
		return AE{}, ErrNotRealAE
	}
	ae.Name, ae.Email = trim(p.Name), trim(p.Email)
	ae.PayoutMethod, ae.PayoutDetails = p.PayoutMethod, trim(p.PayoutDetails)
	ae.Countries = ae.Countries[:0]
	seen := map[string]bool{}
	for _, c := range p.Countries {
		c = strings.ToUpper(trim(c))
		if c != "" && !seen[c] {
			seen[c] = true
			ae.Countries = append(ae.Countries, c)
		}
	}
	if err := ae.ValidateProfile(); err != nil {
		return AE{}, err
	}
	if err := s.store.Put(ctx, EntAE, aeID, encode(ae), d.Version); err != nil {
		return AE{}, err
	}
	return ae, nil
}

// AuthorizeAE gives the OK to a registered AE (section 6.1, step 3): the AE
// becomes authorized and its own account is created under 2.1 Comisiones a
// pagar a AE (D-39). Both writes land atomically. The evaluation that comes
// before the OK is outside the system.
func (s *Service) AuthorizeAE(ctx context.Context, aeID, actor int64) (AE, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, err := s.store.Get(ctx, EntAE, aeID)
	if err != nil {
		return AE{}, err
	}
	ae, err := decode[AE](d)
	if err != nil {
		return AE{}, err
	}
	if ae.Kind != AEReal {
		return AE{}, ErrNotRealAE
	}
	if ae.Status == AEAuthorized {
		return AE{}, ErrAEAlreadyAuthed
	}
	chart, err := s.Chart(ctx)
	if err != nil {
		return AE{}, err
	}
	var codes []string
	for _, a := range chart.Accounts() {
		codes = append(codes, a.Code)
	}
	code := NextAEAccountCode(codes)

	ae.Status = AEAuthorized
	ae.AuthorizedAt, ae.AuthorizedBy = s.stamp(), actor
	ae.AccountCode = code
	acct := Account{Code: code, Name: ae.Name, ParentCode: CodeAEPayables, Postable: true, AEID: ae.ID}
	if _, err := s.store.Commit(ctx, Batch{
		Update:        &Doc{Entity: EntAE, ID: ae.ID, Data: encode(ae)},
		ExpectVersion: d.Version,
		Appends:       []Append{{Entity: EntAccount, Data: encode(acct)}},
	}); err != nil {
		return AE{}, err
	}
	return ae, nil
}

// TenantByID returns one tenant.
func (s *Service) TenantByID(ctx context.Context, id int64) (Tenant, error) {
	d, err := s.store.Get(ctx, EntTenant, id)
	if err != nil {
		return Tenant{}, err
	}
	return decode[Tenant](d)
}

// ListTenants returns the tenants of an AE (aeID 0 returns all of them).
func (s *Service) ListTenants(ctx context.Context, aeID int64) ([]Tenant, error) {
	var where []Cond
	if aeID != 0 {
		where = []Cond{eq("ae_id", aeID)}
	}
	docs, err := s.find(ctx, Query{Entity: EntTenant, Where: where, Limit: 5000})
	if err != nil {
		return nil, err
	}
	return decodeAll[Tenant](docs)
}
