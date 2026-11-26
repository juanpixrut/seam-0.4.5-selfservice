package accounting

import (
	"context"
	"errors"
	"fmt"
	"strconv"
)

// SyncResult reports what one pull did.
type SyncResult struct {
	TenantsNew        int
	TenantsUpdated    int
	ItemsNew          int
	ItemsUpdated      int
	ItemsSkippedClose int      // belong to a closed month: never re-read (D-35)
	ExternallyPaid    int      // the instance says Collected but Admin has not booked it
	Problems          []string // things staff must look at; a pull never aborts on them
}

func (r *SyncResult) problemf(format string, args ...any) {
	r.Problems = append(r.Problems, fmt.Sprintf(format, args...))
}

// resolveAE finds the AE a tenant snapshot refers to: a virtual AE by slug, a
// real one by id.
func (s *Service) resolveAE(ctx context.Context, ref string) (AE, error) {
	if ref == SlugSeamAE || ref == SlugSeamWeb {
		return s.AEBySlug(ctx, ref)
	}
	id, err := strconv.ParseInt(ref, 10, 64)
	if err != nil || id <= 0 {
		return AE{}, fmt.Errorf("unknown AE reference %q", ref)
	}
	ae, err := s.AEByID(ctx, id)
	if err != nil {
		return AE{}, fmt.Errorf("unknown AE reference %q", ref)
	}
	return ae, nil
}

// Sync pulls tenants and billable items from the instance and mirrors them
// (section 6.2, D-02, D-05). Admin never invents an item. Rules:
//
//   - A tenant belongs to one AE for life: if the instance later reports a
//     different AE, the tenant keeps the original and the difference is a
//     problem for staff to look at (D-12).
//   - A tenant that names no known AE is not imported, nor are its items:
//     every tenant has an AE (P2).
//   - An item already Collected in Admin never changes again, whatever the
//     instance says. An item the instance calls collected that Admin has not
//     booked is mirrored as "collection in progress" and counted, so that
//     staff verify the money and mark it themselves.
//   - Items of a closed month are not read again (D-35).
func (s *Service) Sync(ctx context.Context) (SyncResult, error) {
	var res SyncResult
	if s.instance == nil {
		return res, ErrNoInstance
	}
	snap, err := s.instance.Pull(ctx)
	if err != nil {
		return res, err
	}
	inst, err := s.DefaultInstance(ctx)
	if err != nil {
		return res, err
	}
	frozen, err := s.FrozenThrough(ctx)
	if err != nil {
		return res, err
	}

	// tenants
	tdocs, err := s.find(ctx, Query{Entity: EntTenant, Where: []Cond{eq("instance_id", inst.ID)}, Limit: 20000})
	if err != nil {
		return res, err
	}
	known := map[string]Tenant{}
	tenants, err := decodeAll[Tenant](tdocs)
	if err != nil {
		return res, err
	}
	tdocVersion := map[int64]int{}
	for i, t := range tenants {
		known[t.ExternalRef] = t
		tdocVersion[t.ID] = tdocs[i].Version
	}
	for _, ts := range snap.Tenants {
		if ts.ExternalRef == "" {
			res.problemf("a tenant came without an id")
			continue
		}
		ae, err := s.resolveAE(ctx, ts.AERef)
		if err != nil {
			res.problemf("tenant %s (%s) skipped: %v", ts.ExternalRef, ts.Name, err)
			continue
		}
		if old, ok := known[ts.ExternalRef]; ok {
			if old.AEID != ae.ID {
				res.problemf("tenant %s (%s) now reports AE %q but belongs to AE #%d for life; kept", ts.ExternalRef, old.Name, ts.AERef, old.AEID)
			}
			old.Name, old.Country, old.Account, old.Registration = ts.Name, ts.Country, ts.Account, ts.Registration
			if err := s.store.Put(ctx, EntTenant, old.ID, encode(old), 0); err != nil {
				return res, err
			}
			known[ts.ExternalRef] = old
			res.TenantsUpdated++
			continue
		}
		t := Tenant{InstanceID: inst.ID, ExternalRef: ts.ExternalRef, Name: ts.Name, Country: ts.Country,
			AEID: ae.ID, Account: ts.Account, Registration: ts.Registration}
		id, err := s.store.Create(ctx, EntTenant, encode(t))
		if err != nil {
			return res, err
		}
		t.ID = id
		known[t.ExternalRef] = t
		res.TenantsNew++
	}

	// items
	idocs, err := s.find(ctx, Query{Entity: EntItem, Where: []Cond{eq("instance_id", inst.ID)}, Limit: 50000})
	if err != nil {
		return res, err
	}
	items, err := decodeAll[Item](idocs)
	if err != nil {
		return res, err
	}
	byRef := map[string]Item{}
	for _, it := range items {
		byRef[it.ExternalRef] = it
	}
	for _, is := range snap.Items {
		if is.ExternalRef == "" || !validPeriod(is.Period) || !is.Concept.Valid() || is.Amount < 0 {
			res.problemf("item %q skipped: incomplete or invalid", is.ExternalRef)
			continue
		}
		if frozen != "" && is.Period <= frozen {
			res.ItemsSkippedClose++
			continue
		}
		tenant, ok := known[is.TenantRef]
		if !ok {
			res.problemf("item %s skipped: its tenant %q is not imported", is.ExternalRef, is.TenantRef)
			continue
		}
		state := is.State
		if state == ItemCollected {
			res.ExternallyPaid++
			state = ItemInProgress
		}
		if !state.Valid() {
			res.problemf("item %s skipped: unknown state %q", is.ExternalRef, is.State)
			continue
		}
		if old, ok := byRef[is.ExternalRef]; ok {
			if old.State == ItemCollected {
				continue // Admin's own decision; never regressed
			}
			if old.Amount == is.Amount && old.State == state && old.Description == is.Description && old.Concept == is.Concept {
				continue
			}
			old.Amount, old.State, old.Description, old.Concept = is.Amount, state, is.Description, is.Concept
			if err := s.store.Put(ctx, EntItem, old.ID, encode(old), 0); err != nil {
				return res, err
			}
			res.ItemsUpdated++
			continue
		}
		it := Item{InstanceID: inst.ID, ExternalRef: is.ExternalRef, TenantID: tenant.ID, AEID: tenant.AEID,
			Period: is.Period, Concept: is.Concept, Description: is.Description, Amount: is.Amount, State: state}
		id, err := s.store.Create(ctx, EntItem, encode(it))
		if err != nil {
			return res, err
		}
		it.ID = id
		byRef[it.ExternalRef] = it
		res.ItemsNew++
	}
	return res, nil
}

// ItemFilter narrows ListItems.
type ItemFilter struct {
	State    ItemState
	AEID     int64
	TenantID int64
	Period   string
	Limit    int
}

// ListItems returns billable items, newest period first.
func (s *Service) ListItems(ctx context.Context, f ItemFilter) ([]Item, error) {
	var where []Cond
	if f.State != "" {
		where = append(where, eq("state", string(f.State)))
	}
	if f.AEID != 0 {
		where = append(where, eq("ae_id", f.AEID))
	}
	if f.TenantID != 0 {
		where = append(where, eq("tenant_id", f.TenantID))
	}
	if f.Period != "" {
		where = append(where, eq("period", f.Period))
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 500
	}
	docs, err := s.find(ctx, Query{Entity: EntItem, Where: where, OrderBy: "period", Desc: true, Limit: limit})
	if err != nil {
		return nil, err
	}
	return decodeAll[Item](docs)
}

// GetItem returns one item.
func (s *Service) GetItem(ctx context.Context, id int64) (Item, error) {
	d, err := s.store.Get(ctx, EntItem, id)
	if err != nil {
		return Item{}, err
	}
	return decode[Item](d)
}

// maxCommissionLines keeps one collection (item update, two entries with
// their movements, and the lines) inside xolu's 25-append commit.
const maxCommissionLines = 15

// CollectResult is what MarkCollected reports.
type CollectResult struct {
	Item            Item
	CollectionEntry Entry
	CommissionEntry *Entry // nil when the formula produced no commission
	Lines           []CommissionLine
	// NotifyErr is set when the books were updated but the instance could not
	// be told. The item stays flagged so that NotifyInstance can be retried.
	NotifyErr error
}

// MarkCollected records that Seam verified the money for an item (section 6.3,
// D-07). In ONE atomic write it:
//
//   - sets the item to Collected,
//   - books the collection (debit the cash account the money arrived in, credit
//     4.1 Ingresos por servicios), and
//   - when the commission formula returns lines, stores them and accrues them
//     to the AE's account (debit 5.1 Comisiones de AE, credit 2.1.x).
//
// Then it tells the instance. If that last call fails the books are already
// right; the result carries NotifyErr and NotifyInstance retries it. date is
// the day the money arrived ("" means today).
func (s *Service) MarkCollected(ctx context.Context, itemID int64, cashCode, date string, actor int64) (CollectResult, error) {
	var res CollectResult
	if date == "" {
		date = s.today()
	}
	d, err := s.store.Get(ctx, EntItem, itemID)
	if err != nil {
		return res, err
	}
	item, err := decode[Item](d)
	if err != nil {
		return res, err
	}
	if item.State == ItemCollected {
		return res, ErrAlreadyCollected
	}
	ae, err := s.AEByID(ctx, item.AEID)
	if err != nil {
		return res, fmt.Errorf("accounting: item #%d has no AE: %w", item.ID, err)
	}
	if !ae.CanOperate() {
		return res, ErrAENotOperational
	}
	td, err := s.store.Get(ctx, EntTenant, item.TenantID)
	if err != nil {
		return res, fmt.Errorf("accounting: item #%d has no tenant: %w", item.ID, err)
	}
	tenant, err := decode[Tenant](td)
	if err != nil {
		return res, err
	}
	if item.Amount <= 0 {
		return res, fmt.Errorf("%w: the item has no amount to collect", ErrBadMovement)
	}
	chart, err := s.Chart(ctx)
	if err != nil {
		return res, err
	}

	results, err := s.commissionResults(ctx, item, tenant, ae)
	if err != nil {
		return res, fmt.Errorf("accounting: commission formula: %w", err)
	}
	if len(results) > maxCommissionLines {
		return res, ErrTooManyLines
	}
	var commission Cents
	for _, r := range results {
		if r.Amount < 0 {
			return res, fmt.Errorf("%w: a commission line is negative", ErrBadMovement)
		}
		commission += r.Amount
	}

	ref := fmt.Sprintf("item:%d", item.ID)
	collection, err := CustomerCollection(chart, date,
		fmt.Sprintf("Collection %s · %s · %s", item.Period, tenant.Name, item.Concept), ref, cashCode, item.Amount)
	if err != nil {
		return res, err
	}
	collection, err = s.prepare(ctx, chart, collection, actor)
	if err != nil {
		return res, err
	}
	appends := entryAppends(collection)

	var accrual *Entry
	var lines []CommissionLine
	if commission > 0 {
		if ae.AccountCode == "" {
			return res, ErrAENoAccount
		}
		e, err := CommissionAccrual(chart, date,
			fmt.Sprintf("Commission %s · %s · %s", item.Period, tenant.Name, ae.Name), ref, ae.AccountCode, commission)
		if err != nil {
			return res, err
		}
		e, err = s.prepare(ctx, chart, e, actor)
		if err != nil {
			return res, err
		}
		accrual = &e
		appends = append(appends, entryAppends(e)...)
		for _, r := range results {
			if r.Amount == 0 {
				continue
			}
			line := CommissionLine{AEID: ae.ID, ItemID: item.ID, TenantID: tenant.ID, Period: item.Period,
				Concept: item.Concept, Detail: r.Detail, Amount: r.Amount, EntryUID: e.UID}
			lines = append(lines, line)
			appends = append(appends, Append{Entity: EntCommission, Data: encode(line)})
		}
	}

	item.State = ItemCollected
	item.CollectedAt, item.CollectedBy = s.stamp(), actor
	item.CollectedAccount = cashCode
	item.EntryUID = collection.UID
	item.InstanceNotified = false

	ids, err := s.store.Commit(ctx, Batch{
		Update:        &Doc{Entity: EntItem, ID: item.ID, Data: encode(item)},
		ExpectVersion: d.Version,
		Appends:       appends,
	})
	if err != nil {
		if errors.Is(err, ErrConflict) {
			return res, ErrAlreadyCollected
		}
		return res, err
	}
	collection.ID = ids[0]
	next := 1 + len(collection.Movements)
	if accrual != nil {
		accrual.ID = ids[next]
		next += 1 + len(accrual.Movements)
		for i := range lines {
			lines[i].ID = ids[next+i]
		}
	}
	res = CollectResult{Item: item, CollectionEntry: collection, CommissionEntry: accrual, Lines: lines}
	if s.NotifiesOnCollect() {
		res.NotifyErr = s.NotifyInstance(ctx, item.ID)
		if res.NotifyErr == nil {
			res.Item.InstanceNotified = true
		}
	}
	return res, nil
}

// NotifyInstance tells the instance that an item is collected and records that
// it did. It is safe to call again: the instance call is idempotent.
func (s *Service) NotifyInstance(ctx context.Context, itemID int64) error {
	if s.instance == nil {
		return ErrNoInstance
	}
	d, err := s.store.Get(ctx, EntItem, itemID)
	if err != nil {
		return err
	}
	item, err := decode[Item](d)
	if err != nil {
		return err
	}
	if item.State != ItemCollected {
		return fmt.Errorf("accounting: item #%d is not collected", itemID)
	}
	if err := s.instance.MarkCollected(ctx, item.ExternalRef); err != nil {
		return err
	}
	item.InstanceNotified = true
	return s.store.Put(ctx, EntItem, item.ID, encode(item), 0)
}

// CommissionFilter narrows ListCommission.
type CommissionFilter struct {
	AEID   int64
	Period string
	Limit  int
}

// CommissionRow is a commission line with its settlement, if any.
type CommissionRow struct {
	CommissionLine
	SettlementID int64
}

// ListCommission returns commission lines newest first, each with the
// settlement that claimed it (0 when still unsettled).
func (s *Service) ListCommission(ctx context.Context, f CommissionFilter) ([]CommissionRow, error) {
	var where []Cond
	if f.AEID != 0 {
		where = append(where, eq("ae_id", f.AEID))
	}
	if f.Period != "" {
		where = append(where, eq("period", f.Period))
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 1000
	}
	docs, err := s.find(ctx, Query{Entity: EntCommission, Where: where, OrderBy: "period", Desc: true, Limit: limit})
	if err != nil {
		return nil, err
	}
	lines, err := decodeAll[CommissionLine](docs)
	if err != nil {
		return nil, err
	}
	var cw []Cond
	if f.AEID != 0 {
		cw = append(cw, eq("ae_id", f.AEID))
	}
	cdocs, err := s.find(ctx, Query{Entity: EntClaim, Where: cw, Limit: 50000})
	if err != nil {
		return nil, err
	}
	claimed := map[int64]int64{}
	for _, d := range cdocs {
		c, err := decode[Claim](d)
		if err != nil {
			return nil, err
		}
		claimed[c.LineID] = c.SettlementID
	}
	out := make([]CommissionRow, 0, len(lines))
	for _, l := range lines {
		out = append(out, CommissionRow{CommissionLine: l, SettlementID: claimed[l.ID]})
	}
	return out, nil
}
