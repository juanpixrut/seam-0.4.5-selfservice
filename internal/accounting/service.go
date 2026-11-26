package accounting

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/mail"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Service holds the business rules of Seam Admin. It is deliberately free of
// HTTP and of authorization: the web layer decides who may call what (the
// accounting.* permissions) and passes the acting user's id in. Rules that
// protect the books, such as debit = credit, closed months and "a commission
// line is paid once", live here and cannot be bypassed by a route.
type Service struct {
	store    DocStore
	instance InstanceClient
	calc     CommissionCalculator
	noNotify bool
	runner   QueryRunner // runs OQL formulas; nil when the store cannot
	noOQL    bool        // the OQL method is switched off
	blobs    BlobStore
	now      func() time.Time

	// mu serializes the few operations that allocate a number which must be
	// unique (the next 2.1.n account code) or seed data. Seam Admin is a
	// single process in front of one xolu; if that ever changes this becomes
	// a storage-level uniqueness check instead.
	mu sync.Mutex
}

// Option configures a Service.
type Option func(*Service)

// WithInstance sets the client used to pull from, and notify, the platform.
func WithInstance(c InstanceClient) Option { return func(s *Service) { s.instance = c } }

// WithoutNotify stops MarkCollected from telling the instance that an item was
// collected (for platforms that learn about payments by themselves).
func WithoutNotify() Option { return func(s *Service) { s.noNotify = true } }

// HasInstance reports whether a platform client is connected.
func (s *Service) HasInstance() bool { return s.instance != nil }

// NotifiesOnCollect reports whether collecting an item calls the instance.
func (s *Service) NotifiesOnCollect() bool { return s.instance != nil && !s.noNotify }

// WithCalculator sets the commission formula (P6: it lives outside the schema).
func WithCalculator(c CommissionCalculator) Option { return func(s *Service) { s.calc = c } }

// WithClock replaces the clock; tests use it.
func WithClock(now func() time.Time) Option { return func(s *Service) { s.now = now } }

// NewService returns a Service over store. Without options there is no
// instance (pull and notify report ErrNoInstance) and no commission formula
// (items are collected with no commission line).
func NewService(store DocStore, opts ...Option) *Service {
	s := &Service{store: store, calc: NoFormula{}, now: time.Now}
	if r, ok := store.(QueryRunner); ok {
		s.runner = r
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Errors of the business rules. Test with errors.Is.
var (
	ErrAlreadyCollected  = errors.New("accounting: the item is already collected")
	ErrAENotOperational  = errors.New("accounting: the AE is not authorized to operate")
	ErrAENoAccount       = errors.New("accounting: the AE has no account yet")
	ErrAEAlreadyAuthed   = errors.New("accounting: the AE is already authorized")
	ErrNotRealAE         = errors.New("accounting: only real AEs register and need authorization")
	ErrNoCompany         = errors.New("accounting: the company legal name is not configured")
	ErrNoPayoutMethod    = errors.New("accounting: the AE has not chosen a payout method")
	ErrInvoiceMismatch   = errors.New("accounting: the invoice amount does not equal the settlement total")
	ErrLineUnavailable   = errors.New("accounting: a selected commission line is not available for settlement")
	ErrNothingSelected   = errors.New("accounting: select at least one commission line")
	ErrNotPayable        = errors.New("accounting: the settlement is not waiting for payment")
	ErrAlreadyReversed   = errors.New("accounting: the entry was already reversed")
	ErrNotReversible     = errors.New("accounting: this entry cannot be reversed")
	ErrAlreadyClosed     = errors.New("accounting: the month is already closed")
	ErrPeriodNotFinished = errors.New("accounting: the month has not ended yet")
	ErrNoInstance        = errors.New("accounting: no platform instance is connected")
	ErrBadDocument       = errors.New("accounting: invalid document")
	ErrTooManyLines      = errors.New("accounting: too many commission lines for one operation")
	ErrBadPeriod         = errors.New("accounting: the period must be YYYY-MM")
)

// ----------------------------------------------------------------------------
// plumbing
// ----------------------------------------------------------------------------

func newUID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("accounting: no randomness: " + err.Error())
	}
	return hex.EncodeToString(b[:])
}

func (s *Service) today() string  { return s.now().UTC().Format(DateLayout) }
func (s *Service) stamp() string  { return s.now().UTC().Format(time.RFC3339) }
func (s *Service) period() string { return s.now().UTC().Format("2006-01") }

// encode turns a record into stored fields. The id is never stored as a
// field: the store owns it.
func encode(v any) map[string]any {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("accounting: cannot encode %T: %v", v, err))
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		panic(fmt.Sprintf("accounting: cannot encode %T: %v", v, err))
	}
	delete(m, "id")
	delete(m, "_version")
	return m
}

// decode rebuilds a record from a stored doc, setting its id.
func decode[T any](d Doc) (T, error) {
	var v T
	m := make(map[string]any, len(d.Data)+1)
	for k, val := range d.Data {
		m[k] = val
	}
	delete(m, "_version")
	m["id"] = float64(d.ID)
	b, err := json.Marshal(m)
	if err != nil {
		return v, err
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return v, fmt.Errorf("accounting: decoding %s #%d: %w", d.Entity, d.ID, err)
	}
	return v, nil
}

func decodeAll[T any](docs []Doc) ([]T, error) {
	out := make([]T, 0, len(docs))
	for _, d := range docs {
		v, err := decode[T](d)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (s *Service) find(ctx context.Context, q Query) ([]Doc, error) { return s.store.Find(ctx, q) }

func eq(field string, v any) Cond { return Cond{Field: field, Op: "=", Value: v} }

func toCents(v any) Cents {
	switch n := v.(type) {
	case float64:
		return Cents(math.Round(n))
	case int64:
		return Cents(n)
	case int:
		return Cents(n)
	}
	return 0
}

func validPeriod(p string) bool {
	if len(p) != 7 || p[4] != '-' {
		return false
	}
	_, err := time.Parse("2006-01", p)
	return err == nil
}

// ----------------------------------------------------------------------------
// seed
// ----------------------------------------------------------------------------

// Bootstrap makes sure the chart of accounts (section 12.3), the two virtual
// AEs (D-11), the ledger anchor and a default instance exist. It is
// idempotent and cheap: call it at start-up and before the first request.
func (s *Service) Bootstrap(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	docs, err := s.find(ctx, Query{Entity: EntAccount, Limit: 5000})
	if err != nil {
		return err
	}
	have := map[string]bool{}
	for _, d := range docs {
		a, err := decode[Account](d)
		if err != nil {
			return err
		}
		have[a.Code] = true
	}
	for _, a := range DefaultChart() {
		if have[a.Code] {
			continue
		}
		if _, err := s.store.Create(ctx, EntAccount, encode(a)); err != nil {
			return fmt.Errorf("accounting: seeding account %s: %w", a.Code, err)
		}
	}

	aeDocs, err := s.find(ctx, Query{Entity: EntAE, Where: []Cond{eq("kind", string(AEVirtual))}})
	if err != nil {
		return err
	}
	haveAE := map[string]bool{}
	for _, d := range aeDocs {
		a, err := decode[AE](d)
		if err != nil {
			return err
		}
		haveAE[a.Slug] = true
	}
	for _, v := range VirtualAEs() {
		if haveAE[v.Slug] {
			continue
		}
		v.CreatedAt = s.stamp()
		if _, err := s.store.Create(ctx, EntAE, encode(v)); err != nil {
			return fmt.Errorf("accounting: seeding virtual AE %s: %w", v.Slug, err)
		}
	}

	if _, err := s.headLocked(ctx); err != nil {
		return err
	}
	if _, err := s.defaultInstanceLocked(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Service) headLocked(ctx context.Context) (int64, error) {
	docs, err := s.find(ctx, Query{Entity: EntHead, Limit: 1})
	if err != nil {
		return 0, err
	}
	if len(docs) > 0 {
		return docs[0].ID, nil
	}
	return s.store.Create(ctx, EntHead, map[string]any{"created_at": s.stamp()})
}

func (s *Service) headID(ctx context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.headLocked(ctx)
}

func (s *Service) defaultInstanceLocked(ctx context.Context) (Instance, error) {
	docs, err := s.find(ctx, Query{Entity: EntInstance, Limit: 1})
	if err != nil {
		return Instance{}, err
	}
	if len(docs) > 0 {
		return decode[Instance](docs[0])
	}
	in := Instance{Name: "Main"}
	id, err := s.store.Create(ctx, EntInstance, encode(in))
	if err != nil {
		return Instance{}, err
	}
	in.ID = id
	return in, nil
}

// DefaultInstance returns the single instance Admin works with today.
func (s *Service) DefaultInstance(ctx context.Context) (Instance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.defaultInstanceLocked(ctx)
}

// ----------------------------------------------------------------------------
// company
// ----------------------------------------------------------------------------

// Company returns the company settings; the zero Company when none were saved.
func (s *Service) Company(ctx context.Context) (Company, error) {
	docs, err := s.find(ctx, Query{Entity: EntCompany, Limit: 1})
	if err != nil || len(docs) == 0 {
		return Company{}, err
	}
	return decode[Company](docs[0])
}

// ErrBadCompany is returned when the company data is not valid.
var ErrBadCompany = errors.New("accounting: invalid company data")

var countryRE = regexp.MustCompile(`^[A-Z]{2}$`)

// SaveCompany stores the company settings. The legal name is required: AE
// invoices are made out to it.
func (s *Service) SaveCompany(ctx context.Context, c Company) error {
	if trim(c.LegalName) == "" {
		return ErrNoCompany
	}
	c.LegalName, c.TaxID = trim(c.LegalName), trim(c.TaxID)
	c.Address, c.Country, c.BillingEmail = trim(c.Address), strings.ToUpper(trim(c.Country)), trim(c.BillingEmail)
	if len(c.LegalName) > 200 || len(c.TaxID) > 50 || len(c.Address) > 300 {
		return fmt.Errorf("%w: a field is too long", ErrBadCompany)
	}
	if c.Country != "" && !countryRE.MatchString(c.Country) {
		return fmt.Errorf("%w: the country is a two-letter code", ErrBadCompany)
	}
	if c.BillingEmail != "" {
		if a, err := mail.ParseAddress(c.BillingEmail); err != nil || a.Address != c.BillingEmail {
			return fmt.Errorf("%w: the billing email is not valid", ErrBadCompany)
		}
	}
	docs, err := s.find(ctx, Query{Entity: EntCompany, Limit: 1})
	if err != nil {
		return err
	}
	if len(docs) == 0 {
		_, err = s.store.Create(ctx, EntCompany, encode(c))
		return err
	}
	return s.store.Put(ctx, EntCompany, docs[0].ID, encode(c), 0)
}
