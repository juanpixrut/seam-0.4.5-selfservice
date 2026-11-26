package accounting

import (
	"context"
	"errors"
	"testing"
)

func collectOne(t *testing.T, svc *Service, inst *fakeInstance) CollectResult {
	t.Helper()
	ctx := context.Background()
	must(t, svc.Bootstrap(ctx))
	inst.snap = InstanceSnapshot{
		Tenants: []TenantSnapshot{{ExternalRef: "t1", Name: "Acme", Country: "PY", AERef: SlugSeamWeb}},
		Items:   []ItemSnapshot{{ExternalRef: "i1", TenantRef: "t1", Period: "2026-09", Concept: ConceptSubscription, Amount: 10000, State: ItemPending}},
	}
	_, err := svc.Sync(ctx)
	must(t, err)
	items, err := svc.ListItems(ctx, ItemFilter{})
	must(t, err)
	res, err := svc.MarkCollected(ctx, items[0].ID, CodeBank, "2026-10-01", 1)
	must(t, err)
	return res
}

func TestNotifyOnCollectOptions(t *testing.T) {
	// default with a platform: told once
	inst := &fakeInstance{}
	svc := NewService(NewMemStore(), WithInstance(inst), WithBlobs(NewMemBlobs()))
	if !svc.NotifiesOnCollect() || !svc.HasInstance() {
		t.Fatal("a connected platform is notified by default")
	}
	res := collectOne(t, svc, inst)
	if len(inst.notified) != 1 || !res.Item.InstanceNotified || res.NotifyErr != nil {
		t.Fatalf("default: notified=%v res=%+v", inst.notified, res)
	}

	// WithoutNotify: the platform is not called and nothing is flagged as failed
	inst = &fakeInstance{}
	svc = NewService(NewMemStore(), WithInstance(inst), WithoutNotify())
	if svc.NotifiesOnCollect() || !svc.HasInstance() {
		t.Fatal("WithoutNotify keeps the pull but stops the callback")
	}
	res = collectOne(t, svc, inst)
	if len(inst.notified) != 0 || res.NotifyErr != nil {
		t.Fatalf("without notify: notified=%v err=%v", inst.notified, res.NotifyErr)
	}

	// no platform at all: nothing to notify, and no warning
	svc = NewService(NewMemStore())
	if svc.HasInstance() || svc.NotifiesOnCollect() {
		t.Fatal("no instance configured")
	}
}

// failCommitStore makes the Commit of claims fail, as a lost race would.
type failCommitStore struct {
	DocStore
	fail error
}

func (f *failCommitStore) Commit(ctx context.Context, b Batch) ([]int64, error) {
	if f.fail != nil && len(b.Appends) > 0 && b.Appends[0].Entity == EntClaim {
		return nil, f.fail
	}
	return f.DocStore.Commit(ctx, b)
}

// A failed operation must not leave a draft settlement or an unused document.
func TestFailedSettlementLeavesNothingBehind(t *testing.T) {
	e := newEnv(t)
	blobs := NewMemBlobs()
	e.svc.blobs = blobs
	fs := &failCommitStore{DocStore: e.svc.store}
	e.svc.store = fs
	items := e.pull(itoaInt64(e.ae.ID), "t9", "2026-09", 100000)
	_, err := e.svc.MarkCollected(e.ctx, items[0].ID, CodeBank, "2026-10-01", 1)
	must(t, err)
	lines, err := e.svc.UnsettledLines(e.ctx, e.ae.ID)
	must(t, err)
	in := DocumentInput{Type: DocInvoice, Filename: "i.pdf", Data: []byte("%PDF-1.4 test"), AmountUSD: lines[0].Amount}

	for name, failure := range map[string]error{"lost race": ErrConflict, "store down": errors.New("xolu down")} {
		fs.fail = failure
		sets, docs, files := countDocs(t, e, EntSettlement), countDocs(t, e, EntDocument), blobs.Len()
		if _, err := e.svc.CreateSettlement(e.ctx, e.ae.ID, []int64{lines[0].ID}, in, 77); err == nil {
			t.Fatalf("%s: want an error", name)
		}
		if countDocs(t, e, EntSettlement) != sets || countDocs(t, e, EntDocument) != docs || blobs.Len() != files {
			t.Errorf("%s: a draft settlement, document or file was left behind", name)
		}
	}
	// and the line is still free: the same request works once the store is back
	fs.fail = nil
	if _, err := e.svc.CreateSettlement(e.ctx, e.ae.ID, []int64{lines[0].ID}, in, 77); err != nil {
		t.Fatal(err)
	}
}

func countDocs(t *testing.T, e *env, entity string) int {
	t.Helper()
	docs, err := e.svc.store.Find(e.ctx, Query{Entity: entity, Limit: 1000})
	must(t, err)
	return len(docs)
}

func itoaInt64(i int64) string { return itoaInt(int(i)) }

func TestSaveCompanyValidation(t *testing.T) {
	e := newEnv(t)
	for name, c := range map[string]Company{
		"no name":       {},
		"bad country":   {LegalName: "A", Country: "Paraguay"},
		"bad email":     {LegalName: "A", BillingEmail: "not an email"},
		"name+email":    {LegalName: "A", BillingEmail: "Bob <bob@x.com>"},
		"too long name": {LegalName: string(make([]byte, 201))},
	} {
		if err := e.svc.SaveCompany(e.ctx, c); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	must(t, e.svc.SaveCompany(e.ctx, Company{LegalName: " Aberton SA ", Country: "uy", BillingEmail: "billing@aberton.example"}))
	c, err := e.svc.Company(e.ctx)
	must(t, err)
	if c.LegalName != "Aberton SA" || c.Country != "UY" {
		t.Errorf("not normalized: %+v", c)
	}
}
