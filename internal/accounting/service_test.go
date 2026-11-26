package accounting

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"
)

// ----------------------------------------------------------------------------
// fixtures
// ----------------------------------------------------------------------------

var pdfBytes = []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\ntrailer\n<<>>\n%%EOF\n")

type fakeInstance struct {
	snap     InstanceSnapshot
	notified []string
	failMark bool
}

func (f *fakeInstance) Pull(context.Context) (InstanceSnapshot, error) { return f.snap, nil }
func (f *fakeInstance) MarkCollected(_ context.Context, ref string) error {
	if f.failMark {
		return errors.New("instance down")
	}
	f.notified = append(f.notified, ref)
	return nil
}

// tenPercent: every AE earns 10% of what the customer pays, the same call for
// real and virtual AEs.
var tenPercent = CalculatorFunc(func(_ context.Context, it Item, _ Tenant, _ AE) ([]CommissionResult, error) {
	return []CommissionResult{{Detail: "10%", Amount: it.Amount / 10}}, nil
})

type env struct {
	t     *testing.T
	ctx   context.Context
	svc   *Service
	inst  *fakeInstance
	clock *time.Time
	ae    AE // an authorized real AE
}

func newEnv(t *testing.T) *env {
	t.Helper()
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	e := &env{t: t, ctx: context.Background(), inst: &fakeInstance{}, clock: &now}
	e.svc = NewService(NewMemStore(),
		WithInstance(e.inst), WithCalculator(tenPercent), WithBlobs(NewMemBlobs()),
		WithClock(func() time.Time { return *e.clock }))
	must(t, e.svc.Bootstrap(e.ctx))
	must(t, e.svc.SaveCompany(e.ctx, Company{LegalName: "Aberton SA"}))

	ae, err := e.svc.RegisterAE(e.ctx, 77, "Ana AE", "ana@example.com")
	must(t, err)
	ae, err = e.svc.SaveAEProfile(e.ctx, ae.ID, AEProfile{Name: "Ana AE", Email: "ana@example.com",
		Countries: []string{"py", "PY", "bo"}, PayoutMethod: PayoutPayPal, PayoutDetails: "ana@paypal.example"})
	must(t, err)
	ae, err = e.svc.AuthorizeAE(e.ctx, ae.ID, 1)
	must(t, err)
	e.ae = ae
	return e
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func wantErr(t *testing.T, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

// pull loads one tenant of AE ref with the given item amounts (in cents).
func (e *env) pull(aeRef, tenantRef string, period string, amounts ...Cents) []Item {
	e.t.Helper()
	e.inst.snap.Tenants = append(e.inst.snap.Tenants, TenantSnapshot{ExternalRef: tenantRef, Name: "Cliente " + tenantRef, Country: "PY", AERef: aeRef})
	for i, a := range amounts {
		e.inst.snap.Items = append(e.inst.snap.Items, ItemSnapshot{
			ExternalRef: fmt.Sprintf("%s-%s-%d", tenantRef, period, i), TenantRef: tenantRef, Period: period,
			Concept: ConceptSubscription, Amount: a, State: ItemPending})
	}
	_, err := e.svc.Sync(e.ctx)
	must(e.t, err)
	items, err := e.svc.ListItems(e.ctx, ItemFilter{Period: period, TenantID: e.tenantID(tenantRef)})
	must(e.t, err)
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID }) // creation order
	return items
}

func (e *env) tenantID(ref string) int64 {
	ts, err := e.svc.ListTenants(e.ctx, 0)
	must(e.t, err)
	for _, t := range ts {
		if t.ExternalRef == ref {
			return t.ID
		}
	}
	e.t.Fatalf("no tenant %s", ref)
	return 0
}

func (e *env) balances() map[string]Cents {
	e.t.Helper()
	_, b, err := e.svc.Balances(e.ctx, "")
	must(e.t, err)
	return b
}

func (e *env) aeRef() string { return fmt.Sprint(e.ae.ID) }

func invoice(amount Cents) DocumentInput {
	return DocumentInput{Number: "F-1", Filename: "factura.pdf", Data: pdfBytes, AmountUSD: amount}
}

// ----------------------------------------------------------------------------
// chart and bootstrap
// ----------------------------------------------------------------------------

func TestBootstrap_SeedsChartVirtualAEsAndIsIdempotent(t *testing.T) {
	e := newEnv(t)
	must(t, e.svc.Bootstrap(e.ctx))
	must(t, e.svc.Bootstrap(e.ctx))
	chart, err := e.svc.Chart(e.ctx)
	must(t, err)
	if got, want := len(chart.Accounts()), len(DefaultChart())+1; got != want { // + the real AE's account
		t.Errorf("accounts = %d, want %d (no duplicates after three Bootstraps)", got, want)
	}
	aes, err := e.svc.ListAEs(e.ctx)
	must(t, err)
	virtual := 0
	for _, a := range aes {
		if a.Kind == AEVirtual {
			virtual++
		}
	}
	if virtual != 2 || aes[0].Slug != SlugSeamAE || aes[1].Slug != SlugSeamWeb {
		t.Errorf("want exactly the two fixed virtual AEs first, got %+v", aes)
	}
}

func TestChart_MovementsOnlyInLastLevel(t *testing.T) {
	e := newEnv(t)
	for _, code := range []string{"1", "1.1", "2.1", "4", "5", "3.1"} {
		_, err := e.svc.PostEntry(e.ctx, Entry{Date: "2026-10-11", Description: "x", Movements: []Movement{
			{AccountCode: code, Side: Debit, Amount: 100}, {AccountCode: CodeBank, Side: Credit, Amount: 100}}}, 1)
		if err == nil {
			t.Errorf("posting to header account %s must fail (D-38)", code)
		}
	}
}

// ----------------------------------------------------------------------------
// authorization of AEs
// ----------------------------------------------------------------------------

func TestAE_RegisteredCannotOperateUntilAuthorized(t *testing.T) {
	e := newEnv(t)
	ae, err := e.svc.RegisterAE(e.ctx, 88, "Beto", "beto@example.com")
	must(t, err)
	if ae.CanOperate() || ae.Status != AERegistered || ae.AccountCode != "" {
		t.Fatalf("a registered AE must not operate and has no account yet: %+v", ae)
	}
	again, err := e.svc.RegisterAE(e.ctx, 88, "Other name", "x@example.com")
	must(t, err)
	if again.ID != ae.ID || again.Name != "Beto" {
		t.Errorf("registering twice must return the same AE untouched, got %+v", again)
	}
	ae, err = e.svc.AuthorizeAE(e.ctx, ae.ID, 1)
	must(t, err)
	if !ae.CanOperate() || ae.AccountCode == "" {
		t.Errorf("an authorized AE operates and owns an account: %+v", ae)
	}
	if ae.AccountCode == e.ae.AccountCode {
		t.Errorf("each AE has its own account, both got %s", ae.AccountCode)
	}
	_, err = e.svc.AuthorizeAE(e.ctx, ae.ID, 1)
	wantErr(t, err, ErrAEAlreadyAuthed)
}

func TestAE_VirtualAEsNeedNoAuthorizationAndCannotBeAuthorized(t *testing.T) {
	e := newEnv(t)
	v, err := e.svc.AEBySlug(e.ctx, SlugSeamWeb)
	must(t, err)
	if !v.CanOperate() {
		t.Error("virtual AEs exist from the start and always operate")
	}
	_, err = e.svc.AuthorizeAE(e.ctx, v.ID, 1)
	wantErr(t, err, ErrNotRealAE)
	_, err = e.svc.SaveAEProfile(e.ctx, v.ID, AEProfile{Name: "Hacked"})
	wantErr(t, err, ErrNotRealAE)
}

func TestAE_ProfileNormalizesCountriesAndRejectsUnknownPayout(t *testing.T) {
	e := newEnv(t)
	if got := e.ae.Countries; len(got) != 2 || got[0] != "PY" || got[1] != "BO" {
		t.Errorf("countries = %v, want [PY BO]", got)
	}
	_, err := e.svc.SaveAEProfile(e.ctx, e.ae.ID, AEProfile{Name: "Ana", PayoutMethod: "bitcoin"})
	if err == nil {
		t.Error("only transfer and PayPal are supported (D-23)")
	}
	// The profile never touches authorization, user link or account.
	ae, err := e.svc.SaveAEProfile(e.ctx, e.ae.ID, AEProfile{Name: "Ana B", PayoutMethod: PayoutTransfer})
	must(t, err)
	if ae.Status != AEAuthorized || ae.UserID != 77 || ae.AccountCode != e.ae.AccountCode {
		t.Errorf("profile edit changed protected fields: %+v", ae)
	}
	// Coverage is advisory: it answers questions, it never blocks (D-13).
	if !e.ae.CoversCountry("py") || e.ae.CoversCountry("AR") {
		t.Error("CoversCountry")
	}
}

// ----------------------------------------------------------------------------
// pull
// ----------------------------------------------------------------------------

func TestSync_MirrorsAndIsIdempotent(t *testing.T) {
	e := newEnv(t)
	e.pull(e.aeRef(), "T1", "2026-10", 10000, 5000)
	items, _ := e.svc.ListItems(e.ctx, ItemFilter{})
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	res, err := e.svc.Sync(e.ctx)
	must(t, err)
	if res.ItemsNew != 0 || res.TenantsNew != 0 {
		t.Errorf("second pull must create nothing: %+v", res)
	}
	items2, _ := e.svc.ListItems(e.ctx, ItemFilter{})
	if len(items2) != 2 {
		t.Errorf("items after second pull = %d", len(items2))
	}
	for _, it := range items {
		if it.AEID != e.ae.ID {
			t.Errorf("an item carries the AE of its tenant, got %d", it.AEID)
		}
	}
}

func TestSync_TenantWithoutKnownAEIsNotImported(t *testing.T) {
	e := newEnv(t)
	e.inst.snap.Tenants = []TenantSnapshot{
		{ExternalRef: "X1", Name: "Orphan", AERef: "9999"},
		{ExternalRef: "X2", Name: "Nobody", AERef: ""},
		{ExternalRef: "W1", Name: "Web customer", AERef: SlugSeamWeb},
	}
	e.inst.snap.Items = []ItemSnapshot{
		{ExternalRef: "i1", TenantRef: "X1", Period: "2026-10", Concept: ConceptSubscription, Amount: 100, State: ItemPending},
		{ExternalRef: "i2", TenantRef: "W1", Period: "2026-10", Concept: ConceptSubscription, Amount: 200, State: ItemPending},
	}
	res, err := e.svc.Sync(e.ctx)
	must(t, err)
	if res.TenantsNew != 1 || res.ItemsNew != 1 || len(res.Problems) < 2 {
		t.Errorf("every tenant needs an AE (P2); got %+v", res)
	}
}

func TestSync_TenantKeepsItsAEForLife(t *testing.T) {
	e := newEnv(t)
	e.pull(e.aeRef(), "T1", "2026-10", 1000)
	e.inst.snap.Tenants[0].AERef = SlugSeamAE // the instance now claims another AE
	res, err := e.svc.Sync(e.ctx)
	must(t, err)
	if len(res.Problems) == 0 {
		t.Error("a changed AE must be reported to staff")
	}
	ts, _ := e.svc.ListTenants(e.ctx, e.ae.ID)
	if len(ts) != 1 {
		t.Errorf("the tenant must still belong to its original AE (D-12), got %d", len(ts))
	}
}

func TestSync_CollectedIsDecidedInAdminNeverImported(t *testing.T) {
	e := newEnv(t)
	e.pull(e.aeRef(), "T1", "2026-10", 1000)
	e.inst.snap.Items[0].State = ItemCollected // the instance says collected
	res, err := e.svc.Sync(e.ctx)
	must(t, err)
	if res.ExternallyPaid != 1 {
		t.Errorf("want 1 externally-paid warning, got %+v", res)
	}
	items, _ := e.svc.ListItems(e.ctx, ItemFilter{})
	if items[0].State != ItemInProgress {
		t.Errorf("state = %s: Collected is only ever set by staff (D-07)", items[0].State)
	}
}

func TestSync_WithoutInstance(t *testing.T) {
	svc := NewService(NewMemStore())
	_, err := svc.Sync(context.Background())
	wantErr(t, err, ErrNoInstance)
}

// ----------------------------------------------------------------------------
// collection and commission
// ----------------------------------------------------------------------------

func TestMarkCollected_BooksCollectionAndCommissionAtomically(t *testing.T) {
	e := newEnv(t)
	items := e.pull(e.aeRef(), "T1", "2026-10", 10000)
	res, err := e.svc.MarkCollected(e.ctx, items[0].ID, CodePayPal, "2026-10-11", 5)
	must(t, err)
	if res.Item.State != ItemCollected || res.NotifyErr != nil || !res.Item.InstanceNotified {
		t.Fatalf("%+v", res)
	}
	if len(e.inst.notified) != 1 {
		t.Errorf("the instance must be told once, got %v", e.inst.notified)
	}
	b := e.balances()
	if b[CodePayPal] != 10000 || b[CodeServiceInc] != 10000 {
		t.Errorf("collection: PayPal=%d income=%d, want 10000 each", b[CodePayPal], b[CodeServiceInc])
	}
	if b[CodeAECommExp] != 1000 || b[e.ae.AccountCode] != 1000 {
		t.Errorf("commission: expense=%d AE payable=%d, want 1000 each", b[CodeAECommExp], b[e.ae.AccountCode])
	}
	if len(res.Lines) != 1 || res.Lines[0].Amount != 1000 || res.Lines[0].AEID != e.ae.ID {
		t.Errorf("commission line: %+v", res.Lines)
	}
	// The whole ledger balances: assets + expenses = liabilities + equity + income.
	assertBooksBalance(t, e)
}

func assertBooksBalance(t *testing.T, e *env) {
	t.Helper()
	posted, err := e.svc.Posted(e.ctx, "")
	must(t, err)
	var d, c Cents
	for _, p := range posted {
		d += p.Debit
		c += p.Credit
	}
	if d != c {
		t.Errorf("total debits %d != total credits %d", d, c)
	}
}

func TestMarkCollected_Twice(t *testing.T) {
	e := newEnv(t)
	items := e.pull(e.aeRef(), "T1", "2026-10", 10000)
	_, err := e.svc.MarkCollected(e.ctx, items[0].ID, CodeBank, "", 5)
	must(t, err)
	_, err = e.svc.MarkCollected(e.ctx, items[0].ID, CodeBank, "", 5)
	wantErr(t, err, ErrAlreadyCollected)
	if e.balances()[CodeServiceInc] != 10000 {
		t.Error("the second attempt must not book income again")
	}
}

func TestMarkCollected_Concurrent_OnlyOneWins(t *testing.T) {
	e := newEnv(t)
	items := e.pull(e.aeRef(), "T1", "2026-10", 10000)
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := e.svc.MarkCollected(e.ctx, items[0].ID, CodeBank, "", 5); err == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Errorf("%d concurrent collections succeeded, want exactly 1", wins)
	}
	if got := e.balances()[CodeServiceInc]; got != 10000 {
		t.Errorf("income = %d after a race, want 10000", got)
	}
}

func TestMarkCollected_RejectsNonCashAccountAndBadAmounts(t *testing.T) {
	e := newEnv(t)
	items := e.pull(e.aeRef(), "T1", "2026-10", 10000, 0)
	_, err := e.svc.MarkCollected(e.ctx, items[0].ID, CodeServiceInc, "", 5)
	if err == nil {
		t.Error("money can only arrive in a 1.1.x account")
	}
	_, err = e.svc.MarkCollected(e.ctx, items[1].ID, CodeBank, "", 5)
	if err == nil {
		t.Error("an item of 0 cannot be collected")
	}
	if e.balances()[CodeServiceInc] != 0 {
		t.Error("failed collections must leave no trace")
	}
}

func TestMarkCollected_VirtualAEsEarnByTheSameMechanism(t *testing.T) {
	e := newEnv(t)
	items := e.pull(SlugSeamWeb, "W1", "2026-10", 20000)
	_, err := e.svc.MarkCollected(e.ctx, items[0].ID, CodeBank, "", 5)
	must(t, err)
	if got := e.balances()[CodeSeamWeb]; got != 2000 {
		t.Errorf("Seam Web account = %d, want 2000 (P2: no special cases)", got)
	}
}

func TestMarkCollected_NoFormulaMeansNoCommission(t *testing.T) {
	e := newEnv(t)
	e.svc.calc = NoFormula{}
	items := e.pull(e.aeRef(), "T1", "2026-10", 10000)
	res, err := e.svc.MarkCollected(e.ctx, items[0].ID, CodeBank, "", 5)
	must(t, err)
	if len(res.Lines) != 0 || res.CommissionEntry != nil || e.balances()[CodeAECommExp] != 0 {
		t.Errorf("no formula, no commission: %+v", res)
	}
}

func TestMarkCollected_InstanceDownKeepsTheBooksAndCanRetry(t *testing.T) {
	e := newEnv(t)
	items := e.pull(e.aeRef(), "T1", "2026-10", 10000)
	e.inst.failMark = true
	res, err := e.svc.MarkCollected(e.ctx, items[0].ID, CodeBank, "", 5)
	must(t, err)
	if res.NotifyErr == nil || res.Item.InstanceNotified {
		t.Fatal("the failure must be reported")
	}
	if e.balances()[CodeBank] != 10000 {
		t.Error("the books are right even if the instance could not be told")
	}
	e.inst.failMark = false
	must(t, e.svc.NotifyInstance(e.ctx, items[0].ID))
	got, _ := e.svc.GetItem(e.ctx, items[0].ID)
	if !got.InstanceNotified || len(e.inst.notified) != 1 {
		t.Errorf("retry: %+v %v", got, e.inst.notified)
	}
}

// ----------------------------------------------------------------------------
// settlements
// ----------------------------------------------------------------------------

// collectAll collects every item of the AE and returns the commission lines.
func (e *env) collectAll(items []Item) {
	for _, it := range items {
		_, err := e.svc.MarkCollected(e.ctx, it.ID, CodeBank, "2026-10-11", 5)
		must(e.t, err)
	}
}

func (e *env) lineIDs(aeID int64) []int64 {
	lines, err := e.svc.UnsettledLines(e.ctx, aeID)
	must(e.t, err)
	var ids []int64
	for _, l := range lines {
		ids = append(ids, l.ID)
	}
	return ids
}

func TestSettlement_FullCycle(t *testing.T) {
	e := newEnv(t)
	items := e.pull(e.aeRef(), "T1", "2026-10", 10000, 5000)
	e.collectAll(items)
	ids := e.lineIDs(e.ae.ID)
	if len(ids) != 2 {
		t.Fatalf("unsettled lines = %d", len(ids))
	}

	st, err := e.svc.CreateSettlement(e.ctx, e.ae.ID, ids, invoice(1500), 77)
	must(t, err)
	if st.State != SettlementInvoiceLoaded || st.Total != 1500 || st.BilledTo != "Aberton SA" || st.Method != PayoutPayPal || st.InvoiceDocID == 0 {
		t.Fatalf("settlement: %+v", st)
	}
	if left := e.lineIDs(e.ae.ID); len(left) != 0 {
		t.Errorf("claimed lines must leave the unsettled list, %d remain", len(left))
	}
	if e.balances()[e.ae.AccountCode] != 1500 {
		t.Error("creating the settlement moves no money")
	}

	paid, entry, err := e.svc.PaySettlement(e.ctx, st.ID,
		DocumentInput{Filename: "paypal.pdf", Data: pdfBytes}, "2026-10-12", 5)
	must(t, err)
	if paid.State != SettlementPaid || paid.PaymentDocID == 0 || paid.PaidBy != 5 {
		t.Errorf("paid: %+v", paid)
	}
	if entry.Source != SourceSettlement || entry.ID == 0 {
		t.Errorf("entry: %+v", entry)
	}
	b := e.balances()
	if b[e.ae.AccountCode] != 0 {
		t.Errorf("the AE is paid in full, payable = %d", b[e.ae.AccountCode])
	}
	// A PayPal AE is paid from PayPal, not from the bank (section 6.5).
	if b[CodePayPal] != -1500 || b[CodeBank] != 15000 {
		t.Errorf("PayPal=%d bank=%d, want -1500 and 15000", b[CodePayPal], b[CodeBank])
	}
	assertBooksBalance(t, e)

	_, _, err = e.svc.PaySettlement(e.ctx, st.ID, DocumentInput{Data: pdfBytes}, "", 5)
	wantErr(t, err, ErrNotPayable)
	if e.balances()[CodePayPal] != -1500 {
		t.Error("a second payment must not post again")
	}
}

func TestSettlement_TransferPaysFromTheBank(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.SaveAEProfile(e.ctx, e.ae.ID, AEProfile{Name: "Ana", PayoutMethod: PayoutTransfer, PayoutDetails: "IBAN"})
	must(t, err)
	e.collectAll(e.pull(e.aeRef(), "T1", "2026-10", 10000))
	st, err := e.svc.CreateSettlement(e.ctx, e.ae.ID, e.lineIDs(e.ae.ID), invoice(1000), 77)
	must(t, err)
	_, _, err = e.svc.PaySettlement(e.ctx, st.ID, DocumentInput{Data: pdfBytes}, "2026-10-12", 5)
	must(t, err)
	if e.balances()[CodeBank] != 9000 {
		t.Errorf("bank = %d, want 9000", e.balances()[CodeBank])
	}
}

func TestSettlement_InvoiceMustEqualTotal(t *testing.T) {
	e := newEnv(t)
	e.collectAll(e.pull(e.aeRef(), "T1", "2026-10", 10000))
	ids := e.lineIDs(e.ae.ID)
	for _, amt := range []Cents{999, 1001, 0} {
		_, err := e.svc.CreateSettlement(e.ctx, e.ae.ID, ids, invoice(amt), 77)
		wantErr(t, err, ErrInvoiceMismatch)
	}
	if len(e.lineIDs(e.ae.ID)) != 1 {
		t.Error("a rejected invoice must not claim the line")
	}
	sts, _ := e.svc.ListSettlements(e.ctx, SettlementFilter{})
	if len(sts) != 0 {
		t.Errorf("nothing must be listed: %+v", sts)
	}
}

func TestSettlement_ALineIsPaidAtMostOnce(t *testing.T) {
	e := newEnv(t)
	e.collectAll(e.pull(e.aeRef(), "T1", "2026-10", 10000, 10000))
	ids := e.lineIDs(e.ae.ID)
	_, err := e.svc.CreateSettlement(e.ctx, e.ae.ID, ids[:1], invoice(1000), 77)
	must(t, err)
	_, err = e.svc.CreateSettlement(e.ctx, e.ae.ID, ids, invoice(2000), 77) // overlaps the first
	wantErr(t, err, ErrLineUnavailable)
	_, err = e.svc.CreateSettlement(e.ctx, e.ae.ID, ids[:1], invoice(1000), 77)
	wantErr(t, err, ErrLineUnavailable)
}

func TestSettlement_Concurrent_OneWinnerPerLine(t *testing.T) {
	e := newEnv(t)
	e.collectAll(e.pull(e.aeRef(), "T1", "2026-10", 10000))
	ids := e.lineIDs(e.ae.ID)
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := e.svc.CreateSettlement(e.ctx, e.ae.ID, ids, invoice(1000), 77); err == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Errorf("%d settlements claimed the same line, want 1", wins)
	}
	sts, _ := e.svc.ListSettlements(e.ctx, SettlementFilter{})
	if len(sts) != 1 {
		t.Errorf("listed settlements = %d, want 1 (drafts are never shown)", len(sts))
	}
}

func TestSettlement_AEsCannotSettleEachOthersLines(t *testing.T) {
	e := newEnv(t)
	other, _ := e.svc.RegisterAE(e.ctx, 99, "Beto", "b@example.com")
	other, _ = e.svc.SaveAEProfile(e.ctx, other.ID, AEProfile{Name: "Beto", PayoutMethod: PayoutTransfer})
	other, _ = e.svc.AuthorizeAE(e.ctx, other.ID, 1)
	e.collectAll(e.pull(e.aeRef(), "T1", "2026-10", 10000))
	_, err := e.svc.CreateSettlement(e.ctx, other.ID, e.lineIDs(e.ae.ID), invoice(1000), 99)
	wantErr(t, err, ErrLineUnavailable)
}

func TestSettlement_PreconditionsOfTheAE(t *testing.T) {
	e := newEnv(t)
	e.collectAll(e.pull(e.aeRef(), "T1", "2026-10", 10000))
	ids := e.lineIDs(e.ae.ID)

	_, err := e.svc.CreateSettlement(e.ctx, e.ae.ID, nil, invoice(0), 77)
	wantErr(t, err, ErrNothingSelected)
	_, err = e.svc.CreateSettlement(e.ctx, e.ae.ID, []int64{424242}, invoice(1), 77)
	wantErr(t, err, ErrLineUnavailable)

	_, err = e.svc.SaveAEProfile(e.ctx, e.ae.ID, AEProfile{Name: "Ana"}) // no payout method
	must(t, err)
	_, err = e.svc.CreateSettlement(e.ctx, e.ae.ID, ids, invoice(1000), 77)
	wantErr(t, err, ErrNoPayoutMethod)
	_, err = e.svc.SaveAEProfile(e.ctx, e.ae.ID, AEProfile{Name: "Ana", PayoutMethod: PayoutPayPal})
	must(t, err)

	// No legal name configured: there is nobody to bill.
	s2 := NewService(e.svc.store, WithBlobs(NewMemBlobs()))
	d, _ := e.svc.store.Find(e.ctx, Query{Entity: EntCompany})
	for _, x := range d {
		must(t, e.svc.store.Put(e.ctx, EntCompany, x.ID, map[string]any{"legal_name": ""}, 0))
	}
	_, err = s2.CreateSettlement(e.ctx, e.ae.ID, ids, invoice(1000), 77)
	wantErr(t, err, ErrNoCompany)
}

func TestSettlement_BilledToIsTheLegalNameAtThatTime(t *testing.T) {
	e := newEnv(t)
	e.collectAll(e.pull(e.aeRef(), "T1", "2026-10", 10000, 10000))
	ids := e.lineIDs(e.ae.ID)
	a, err := e.svc.CreateSettlement(e.ctx, e.ae.ID, ids[:1], invoice(1000), 77)
	must(t, err)
	must(t, e.svc.SaveCompany(e.ctx, Company{LegalName: "  Nueva SA  "}))
	b, err := e.svc.CreateSettlement(e.ctx, e.ae.ID, ids[1:], invoice(1000), 77)
	must(t, err)
	if a.BilledTo != "Aberton SA" || b.BilledTo != "Nueva SA" {
		t.Errorf("billed to %q then %q", a.BilledTo, b.BilledTo)
	}
	got, _ := e.svc.GetSettlement(e.ctx, a.ID)
	if got.BilledTo != "Aberton SA" {
		t.Error("an old settlement keeps the name it was billed to")
	}
}

func TestSettlement_TooManyLines(t *testing.T) {
	e := newEnv(t)
	ids := make([]int64, MaxSettlementLines+1)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	_, err := e.svc.CreateSettlement(e.ctx, e.ae.ID, ids, invoice(1), 77)
	wantErr(t, err, ErrTooManyLines)
}

// ----------------------------------------------------------------------------
// documents
// ----------------------------------------------------------------------------

func TestDocument_ContentIsSniffedAndNamesAreSanitized(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.SaveDocument(e.ctx, DocumentInput{Type: DocInvoice, Filename: "x.pdf", Data: []byte("<html><script>alert(1)</script>")}, 1)
	wantErr(t, err, ErrBadDocument)
	_, err = e.svc.SaveDocument(e.ctx, DocumentInput{Type: DocInvoice, Filename: "x.pdf"}, 1)
	wantErr(t, err, ErrBadDocument)
	_, err = e.svc.SaveDocument(e.ctx, DocumentInput{Type: DocInvoice, Data: make([]byte, MaxDocumentBytes+1)}, 1)
	wantErr(t, err, ErrBadDocument)
	_, err = e.svc.SaveDocument(e.ctx, DocumentInput{Type: "weird", Data: pdfBytes}, 1)
	wantErr(t, err, ErrBadDocument)
	_, err = e.svc.SaveDocument(e.ctx, DocumentInput{Type: DocInvoice, Data: pdfBytes, Date: "11/10/2026"}, 1)
	wantErr(t, err, ErrBadDocument)

	d, err := e.svc.SaveDocument(e.ctx, DocumentInput{Type: DocInvoice, Filename: `..\..\etc/pass wd<>.html`, Data: pdfBytes}, 1)
	must(t, err)
	if d.Filename != "pass wd.pdf" || d.ContentType != "application/pdf" {
		t.Errorf("filename = %q type = %q: the extension follows the sniffed content", d.Filename, d.ContentType)
	}
	doc, data, err := e.svc.DocumentContent(e.ctx, d.ID)
	must(t, err)
	if doc.ID != d.ID || !bytes.Equal(data, pdfBytes) {
		t.Error("round trip")
	}
}

func TestDocument_CurrencyMetadataOnlyWhenNotUSD(t *testing.T) {
	e := newEnv(t)
	usd, err := e.svc.SaveDocument(e.ctx, DocumentInput{Type: DocReceipt, Data: pdfBytes, AmountUSD: 100,
		Currency: "usd", ForeignTotal: "100", FxRate: "1", FxDate: "2026-10-01"}, 1)
	must(t, err)
	if usd.Currency != "" || usd.ForeignTotal != "" || usd.FxRate != "" || usd.FxDate != "" {
		t.Errorf("a USD document records nothing extra (D-33): %+v", usd)
	}
	pyg, err := e.svc.SaveDocument(e.ctx, DocumentInput{Type: DocReceipt, Data: pdfBytes, AmountUSD: 100,
		Currency: "pyg", ForeignTotal: "730.000", FxRate: "7300", FxDate: "2026-10-01"}, 1)
	must(t, err)
	if pyg.Currency != "PYG" || pyg.ForeignTotal != "730.000" || pyg.FxRate != "7300" || pyg.AmountUSD != 100 {
		t.Errorf("%+v", pyg)
	}
}

func TestDocument_NoBlobStore(t *testing.T) {
	svc := NewService(NewMemStore())
	_, err := svc.SaveDocument(context.Background(), DocumentInput{Type: DocInvoice, Data: pdfBytes}, 1)
	wantErr(t, err, ErrNoBlobs)
}

func TestFileBlobs_RefusesPathTricksAndOverwrites(t *testing.T) {
	fb, err := NewFileBlobs(t.TempDir())
	must(t, err)
	for _, k := range []string{"../x", "a/b", "", "ZZZZ", "0123456789abcdef0123456"} {
		if err := fb.Put(context.Background(), k, []byte("x")); err == nil {
			t.Errorf("key %q must be refused", k)
		}
		if _, err := fb.Get(context.Background(), k); err == nil {
			t.Errorf("key %q must be refused on read", k)
		}
	}
	k := newUID()
	must(t, fb.Put(context.Background(), k, []byte("one")))
	if err := fb.Put(context.Background(), k, []byte("two")); err == nil {
		t.Error("a blob is never overwritten")
	}
	got, err := fb.Get(context.Background(), k)
	must(t, err)
	if string(got) != "one" {
		t.Errorf("got %q", got)
	}
	_, err = fb.Get(context.Background(), newUID())
	wantErr(t, err, ErrNotFound)
}

// ----------------------------------------------------------------------------
// the ledger by hand
// ----------------------------------------------------------------------------

func TestManualEntries_AreValidatedAndUnbalancedOnesRefused(t *testing.T) {
	e := newEnv(t)
	good := Entry{Date: "2026-10-11", Description: "hosting", Movements: []Movement{
		{AccountCode: CodeOperatingEx, Side: Debit, Amount: 5000}, {AccountCode: CodeBank, Side: Credit, Amount: 5000}}}
	if _, err := e.svc.PostEntry(e.ctx, good, 1); err != nil {
		t.Fatal(err)
	}
	bad := []Entry{
		{Date: "2026-10-11", Movements: []Movement{{AccountCode: CodeBank, Side: Debit, Amount: 5}, {AccountCode: CodeOtherInc, Side: Credit, Amount: 4}}},
		{Date: "2026-10-11", Movements: []Movement{{AccountCode: CodeBank, Side: Debit, Amount: 5}}},
		{Date: "2026-10-11", Movements: []Movement{{AccountCode: CodeBank, Side: Debit, Amount: -5}, {AccountCode: CodeOtherInc, Side: Credit, Amount: -5}}},
		{Date: "2026-10-11", Movements: []Movement{{AccountCode: "9.9.9", Side: Debit, Amount: 5}, {AccountCode: CodeOtherInc, Side: Credit, Amount: 5}}},
		{Date: "not a date", Movements: []Movement{{AccountCode: CodeBank, Side: Debit, Amount: 5}, {AccountCode: CodeOtherInc, Side: Credit, Amount: 5}}},
		{Date: "2026-10-11", Movements: []Movement{{AccountCode: CodeBank, Side: "sideways", Amount: 5}, {AccountCode: CodeOtherInc, Side: Credit, Amount: 5}}},
	}
	for i, en := range bad {
		if _, err := e.svc.PostEntry(e.ctx, en, 1); err == nil {
			t.Errorf("bad entry %d was accepted", i)
		}
	}
	if got := e.balances()[CodeBank]; got != -5000 {
		t.Errorf("bank = %d: refused entries must leave no partial movements", got)
	}
	assertBooksBalance(t, e)
}

func TestReversal_CancelsOnceAndOnlyOnce(t *testing.T) {
	e := newEnv(t)
	orig, err := e.svc.PostEntry(e.ctx, Entry{Date: "2026-10-11", Description: "x", Movements: []Movement{
		{AccountCode: CodeOperatingEx, Side: Debit, Amount: 700}, {AccountCode: CodeBank, Side: Credit, Amount: 700}}}, 1)
	must(t, err)
	rev, err := e.svc.ReverseEntry(e.ctx, orig.ID, "", 2)
	must(t, err)
	if rev.Source != SourceReversal || e.balances()[CodeBank] != 0 || e.balances()[CodeOperatingEx] != 0 {
		t.Errorf("reversal: %+v", e.balances())
	}
	_, err = e.svc.ReverseEntry(e.ctx, orig.ID, "", 2)
	wantErr(t, err, ErrAlreadyReversed)
	_, err = e.svc.ReverseEntry(e.ctx, rev.ID, "", 2)
	wantErr(t, err, ErrNotReversible)
	got, err := e.svc.GetEntry(e.ctx, orig.ID)
	must(t, err)
	if len(got.Movements) != 2 {
		t.Errorf("the original is untouched, append-only: %+v", got)
	}
}

func TestLedger_RunningBalanceAndOpening(t *testing.T) {
	e := newEnv(t)
	post := func(date string, amt Cents) {
		_, err := e.svc.PostEntry(e.ctx, Entry{Date: date, Description: "in", Movements: []Movement{
			{AccountCode: CodeBank, Side: Debit, Amount: amt}, {AccountCode: CodeOtherInc, Side: Credit, Amount: amt}}}, 1)
		must(t, err)
	}
	post("2026-09-30", 1000)
	post("2026-10-02", 250)
	post("2026-10-03", 750)
	l, err := e.svc.Ledger(e.ctx, CodeBank, "2026-10-01", "2026-10-31")
	must(t, err)
	if l.Opening != 1000 || l.Closing != 2000 || len(l.Lines) != 2 || l.Lines[0].Balance != 1250 || l.Lines[1].Balance != 2000 {
		t.Errorf("%+v", l)
	}
	_, err = e.svc.Ledger(e.ctx, "1.1", "", "")
	wantErr(t, err, ErrNotPostable)
	_, err = e.svc.Ledger(e.ctx, "7.7", "", "")
	wantErr(t, err, ErrUnknownAccount)
}

func TestBalances_RollUpTheHierarchy(t *testing.T) {
	e := newEnv(t)
	e.collectAll(e.pull(e.aeRef(), "T1", "2026-10", 10000))
	_, err := e.svc.PostEntry(e.ctx, Entry{Date: "2026-10-11", Description: "aporte", Movements: []Movement{
		{AccountCode: CodePayPal, Side: Debit, Amount: 300}, {AccountCode: "3.1.1", Side: Credit, Amount: 300}}}, 1)
	must(t, err)
	b := e.balances()
	if b["1.1"] != 10300 || b["1"] != 10300 {
		t.Errorf("assets: 1.1=%d 1=%d", b["1.1"], b["1"])
	}
	if b["2.1"] != 1000 || b["2"] != 1000 || b["3"] != 300 || b["4"] != 10000 || b["5"] != 1000 {
		t.Errorf("rollups: %v", b)
	}
	// Accounting equation: assets = liabilities + equity + (income - expenses).
	if b["1"] != b["2"]+b["3"]+b["4"]-b["5"] {
		t.Errorf("A=%d, P+PN+I-E=%d", b["1"], b["2"]+b["3"]+b["4"]-b["5"])
	}
}

// ----------------------------------------------------------------------------
// monthly close
// ----------------------------------------------------------------------------

func TestClose_FreezesTheMonthAndTheMonthsBefore(t *testing.T) {
	e := newEnv(t)
	items := e.pull(e.aeRef(), "T1", "2026-09", 10000, 4000)
	_, err := e.svc.MarkCollected(e.ctx, items[0].ID, CodeBank, "2026-09-15", 5)
	must(t, err)

	_, err = e.svc.CloseMonth(e.ctx, "2026-10", 5) // October has not ended
	wantErr(t, err, ErrPeriodNotFinished)
	_, err = e.svc.CloseMonth(e.ctx, "oct", 5)
	wantErr(t, err, ErrBadPeriod)

	c, err := e.svc.CloseMonth(e.ctx, "2026-09", 5)
	must(t, err)
	if c.Items != 2 || c.ItemsCollected != 1 || c.BilledTotal != 14000 || c.CollectedTotal != 10000 || c.CommissionTotal != 1000 {
		t.Errorf("consolidated figures: %+v", c)
	}
	if c.Balances[CodeBank] != 10000 || c.Balances["1"] != 10000 {
		t.Errorf("saved balances: %v", c.Balances)
	}
	_, err = e.svc.CloseMonth(e.ctx, "2026-09", 5)
	wantErr(t, err, ErrAlreadyClosed)

	// Nothing can be posted in, or before, a closed month...
	for _, d := range []string{"2026-09-30", "2026-09-01", "2026-08-15"} {
		_, err = e.svc.PostEntry(e.ctx, Entry{Date: d, Description: "late", Movements: []Movement{
			{AccountCode: CodeBank, Side: Debit, Amount: 1}, {AccountCode: CodeOtherInc, Side: Credit, Amount: 1}}}, 1)
		wantErr(t, err, ErrPeriodClosed)
	}
	// ...so a late collection of a September item is refused, not silently re-dated.
	_, err = e.svc.MarkCollected(e.ctx, items[1].ID, CodeBank, "2026-09-20", 5)
	wantErr(t, err, ErrPeriodClosed)
	// A correction goes in the open month.
	_, err = e.svc.PostEntry(e.ctx, Entry{Date: "2026-10-01", Description: "ajuste", Movements: []Movement{
		{AccountCode: CodeBank, Side: Debit, Amount: 1}, {AccountCode: CodeOtherInc, Side: Credit, Amount: 1}}}, 1)
	must(t, err)

	// The closed month is never pulled again.
	e.inst.snap.Items[0].Amount = 99999
	res, err := e.svc.Sync(e.ctx)
	must(t, err)
	if res.ItemsSkippedClose != 2 {
		t.Errorf("skipped = %d, want 2", res.ItemsSkippedClose)
	}
	got, _ := e.svc.GetItem(e.ctx, items[0].ID)
	if got.Amount != 10000 {
		t.Error("a closed month's item must not change")
	}
	closes, _ := e.svc.ListCloses(e.ctx)
	if len(closes) != 1 || closes[0].Period != "2026-09" {
		t.Errorf("%+v", closes)
	}
}

func TestClose_InOrder(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.CloseMonth(e.ctx, "2026-09", 5)
	must(t, err)
	_, err = e.svc.CloseMonth(e.ctx, "2026-08", 5) // before the last closed one
	wantErr(t, err, ErrCloseOrder)
}

func TestClose_ConcurrentOnlyOne(t *testing.T) {
	e := newEnv(t)
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := e.svc.CloseMonth(e.ctx, "2026-09", 5); err == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Errorf("%d closes of the same month, want 1", wins)
	}
}

// ----------------------------------------------------------------------------
// company and money
// ----------------------------------------------------------------------------

func TestCompany_NameIsRequiredAndTrimmed(t *testing.T) {
	e := newEnv(t)
	wantErr(t, e.svc.SaveCompany(e.ctx, Company{LegalName: "   "}), ErrNoCompany)
	must(t, e.svc.SaveCompany(e.ctx, Company{LegalName: " Aberton SA ", TaxID: "123"}))
	c, err := e.svc.Company(e.ctx)
	must(t, err)
	if c.LegalName != "Aberton SA" || c.TaxID != "123" {
		t.Errorf("%+v", c)
	}
}

func TestCommissionList_ShowsSettlementOfEachLine(t *testing.T) {
	e := newEnv(t)
	e.collectAll(e.pull(e.aeRef(), "T1", "2026-10", 10000, 10000))
	ids := e.lineIDs(e.ae.ID)
	st, err := e.svc.CreateSettlement(e.ctx, e.ae.ID, ids[:1], invoice(1000), 77)
	must(t, err)
	rows, err := e.svc.ListCommission(e.ctx, CommissionFilter{AEID: e.ae.ID})
	must(t, err)
	claimed := 0
	for _, r := range rows {
		if r.SettlementID == st.ID {
			claimed++
		} else if r.SettlementID != 0 {
			t.Errorf("unexpected settlement %d", r.SettlementID)
		}
	}
	if len(rows) != 2 || claimed != 1 {
		t.Errorf("rows=%d claimed=%d", len(rows), claimed)
	}
}
