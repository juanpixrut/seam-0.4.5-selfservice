package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ha1tch/seam-ui/internal/accounting"
	"github.com/ha1tch/seam-ui/internal/auth"
)

type acctFakeInstance struct{ snap accounting.InstanceSnapshot }

func (f *acctFakeInstance) Pull(context.Context) (accounting.InstanceSnapshot, error) {
	return f.snap, nil
}
func (f *acctFakeInstance) MarkCollected(context.Context, string) error { return nil }

type acctEnv struct {
	mux          *http.ServeMux
	svc          *accounting.Service
	aeA, aeB     accounting.AE
	setA         accounting.Settlement
	invoiceDocID int64
	inst         *acctFakeInstance
	router       *Router
}

const (
	acctUserA = 701
	acctUserB = 702
)

func newAcctEnv(t *testing.T) *acctEnv {
	t.Helper()
	ctx := context.Background()
	inst := &acctFakeInstance{}
	svc := accounting.NewService(accounting.NewMemStore(),
		accounting.WithInstance(inst), accounting.WithBlobs(accounting.NewMemBlobs()),
		accounting.WithCalculator(accounting.CalculatorFunc(func(_ context.Context, it accounting.Item, _ accounting.Tenant, _ accounting.AE) ([]accounting.CommissionResult, error) {
			return []accounting.CommissionResult{{Detail: "10%", Amount: it.Amount / 10}}, nil
		})))
	if err := svc.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	if err := svc.SaveCompany(ctx, accounting.Company{LegalName: "Aberton SA"}); err != nil {
		t.Fatal(err)
	}
	mk := func(user int64, name string) accounting.AE {
		ae, err := svc.RegisterAE(ctx, user, name, name+"@example.com")
		if err != nil {
			t.Fatal(err)
		}
		if ae, err = svc.SaveAEProfile(ctx, ae.ID, accounting.AEProfile{Name: name, PayoutMethod: accounting.PayoutPayPal, PayoutDetails: "x@y"}); err != nil {
			t.Fatal(err)
		}
		if ae, err = svc.AuthorizeAE(ctx, ae.ID, 1); err != nil {
			t.Fatal(err)
		}
		return ae
	}
	e := &acctEnv{svc: svc, inst: inst, aeA: mk(acctUserA, "Ana"), aeB: mk(acctUserB, "Beto")}
	inst.snap = accounting.InstanceSnapshot{
		Tenants: []accounting.TenantSnapshot{
			{ExternalRef: "t1", Name: "<script>alert(1)</script>", Country: "PY", AERef: itoa(e.aeA.ID)},
			{ExternalRef: "t2", Name: "Beta SA", Country: "PY", AERef: itoa(e.aeB.ID)},
		},
		Items: []accounting.ItemSnapshot{
			{ExternalRef: "i1", TenantRef: "t1", Period: "2026-09", Concept: accounting.ConceptSubscription, Amount: 100000, State: accounting.ItemPending},
			{ExternalRef: "i2", TenantRef: "t2", Period: "2026-09", Concept: accounting.ConceptSubscription, Amount: 50000, State: accounting.ItemPending},
		},
	}
	if _, err := svc.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	items, _ := svc.ListItems(ctx, accounting.ItemFilter{})
	for _, it := range items {
		if _, err := svc.MarkCollected(ctx, it.ID, accounting.CodeBank, "2026-10-01", 1); err != nil {
			t.Fatal(err)
		}
	}
	lines, err := svc.UnsettledLines(ctx, e.aeA.ID)
	if err != nil || len(lines) != 1 {
		t.Fatalf("lines = %v, %v", lines, err)
	}
	e.setA, err = svc.CreateSettlement(ctx, e.aeA.ID, []int64{lines[0].ID}, accounting.DocumentInput{
		Type: accounting.DocInvoice, Filename: "inv.pdf", Data: []byte("%PDF-1"), AmountUSD: lines[0].Amount}, acctUserA)
	if err != nil {
		t.Fatal(err)
	}
	e.invoiceDocID = e.setA.InvoiceDocID

	r := testRouter(t, "http://127.0.0.1:1", nil)
	r.config = Config{AppName: "Test", Version: "0", EnableAccounting: true}
	r.handler.config = r.config
	r.SetAccounting(svc)
	e.mux = http.NewServeMux()
	e.router = r
	r.MountAccountingRoutes(e.mux)
	return e
}

func (e *acctEnv) get(role string, uid int64, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", path, nil)
	req = req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{UserID: uid, TenantID: 1, Role: role}))
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req)
	return rec
}

func TestAccountingAEIsolation(t *testing.T) {
	e := newAcctEnv(t)
	sp := "/accounting/settlements/" + itoa(e.setA.ID)
	dp := "/accounting/documents/" + itoa(e.invoiceDocID)

	if rec := e.get(auth.RoleAccountExecutive, acctUserA, sp); rec.Code != 200 {
		t.Fatalf("owner settlement = %d", rec.Code)
	}
	if rec := e.get(auth.RoleAccountExecutive, acctUserB, sp); rec.Code != 404 {
		t.Fatalf("other AE settlement = %d, want 404", rec.Code)
	}
	rec := e.get(auth.RoleAccountExecutive, acctUserA, dp)
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Disposition"), "attachment") || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("owner document = %d %v", rec.Code, rec.Header())
	}
	if rec := e.get(auth.RoleAccountExecutive, acctUserB, dp); rec.Code != 404 {
		t.Fatalf("other AE document = %d, want 404", rec.Code)
	}
	// Staff areas are closed to AEs.
	for _, p := range []string{"/accounting/settlements", "/accounting/ledger", "/accounting/aes", "/accounting/collections", "/accounting/commissions", "/accounting/company", "/accounting/closes", "/accounting/accounts"} {
		if rec := e.get(auth.RoleAccountExecutive, acctUserA, p); rec.Code != 403 {
			t.Errorf("AE on %s = %d, want 403", p, rec.Code)
		}
	}
	// A staff-only user cannot use the AE portal.
	if rec := e.get(auth.RoleAccountant, 5, "/accounting/portal"); rec.Code != 403 {
		t.Errorf("accountant on portal = %d, want 403", rec.Code)
	}
}

func TestAccountingPortalShowsOnlyOwnData(t *testing.T) {
	e := newAcctEnv(t)
	rec := e.get(auth.RoleAccountExecutive, acctUserB, "/accounting/portal")
	if rec.Code != 200 {
		t.Fatalf("portal = %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "&lt;script&gt;") || strings.Contains(rec.Body.String(), "script>alert") {
		t.Error("another AE's tenant leaked into the portal")
	}
	if !strings.Contains(rec.Body.String(), "Beta SA") {
		t.Error("own tenant missing")
	}
}

func TestAccountingStaffAccessAndEscaping(t *testing.T) {
	e := newAcctEnv(t)
	for _, role := range []string{auth.RoleAccountant, auth.RoleAdminStaff, auth.RoleAdmin} {
		rec := e.get(role, 3, "/accounting/collections")
		if rec.Code != 200 {
			t.Fatalf("%s collections = %d", role, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "<script>alert(1)") {
			t.Fatalf("%s: tenant name is not escaped", role)
		}
	}
	if rec := e.get(auth.RoleAccountant, 3, "/accounting/ledger"); rec.Code != 200 {
		t.Errorf("accountant ledger = %d", rec.Code)
	}
	if rec := e.get(auth.RoleViewer, 3, "/accounting/ledger"); rec.Code != 403 {
		t.Errorf("viewer ledger = %d, want 403", rec.Code)
	}
}

func TestAccountingBannerOnlyShowsKnownShape(t *testing.T) {
	e := newAcctEnv(t)
	rec := e.get(auth.RoleAdmin, 1, "/accounting/ledger?err=%3Cscript%3E&ok=Bad-Key")
	if rec.Code != 200 || strings.Contains(rec.Body.String(), "accounting.err.<script>") || strings.Contains(rec.Body.String(), "Bad-Key") || strings.Contains(rec.Body.String(), "accounting.msg.") {
		t.Fatalf("banner: %d", rec.Code)
	}
}

func TestAccountingSettleRejectsForeignLine(t *testing.T) {
	e := newAcctEnv(t)
	ctx := context.Background()
	// Beta's commission line must not be settled by Ana.
	linesB, _ := e.svc.UnsettledLines(ctx, e.aeB.ID)
	if len(linesB) != 1 {
		t.Fatal("setup")
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.WriteField("line", itoa(linesB[0].ID))
	mw.WriteField("amount", linesB[0].Amount.Display())
	fw, _ := mw.CreateFormFile("file", "inv.pdf")
	fw.Write([]byte("%PDF-1"))
	mw.Close()
	req := httptest.NewRequest("POST", "/accounting/portal/settle", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req = req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{UserID: acctUserA, TenantID: 1, Role: auth.RoleAccountExecutive}))
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "err=") {
		t.Fatalf("foreign line: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if left, _ := e.svc.UnsettledLines(ctx, e.aeB.ID); len(left) != 1 {
		t.Fatal("Beta's line was claimed by Ana")
	}
}

// Every i18n key the module's Go code asks for must exist in the locale files.
func TestAccountingI18nKeysExist(t *testing.T) {
	raw, err := os.ReadFile("../i18n/locales/en.json")
	if err != nil {
		t.Fatal(err)
	}
	var en map[string]string
	if err := json.Unmarshal(raw, &en); err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`"((?:accounting|nav\.module\.accounting|users\.(?:role|tab))\.[a-z_.]*[a-z_])"`)
	files, _ := filepath.Glob("accounting_*.go")
	srv, _ := os.ReadFile("../server/server.go")
	var srcs [][]byte
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, _ := os.ReadFile(f)
		srcs = append(srcs, b)
	}
	srcs = append(srcs, srv)
	need := map[string]bool{}
	for _, b := range srcs {
		for _, m := range re.FindAllSubmatch(b, -1) {
			k := string(m[1])
			if strings.HasSuffix(k, ".") || strings.HasSuffix(k, "_") { // dynamic prefix, enumerated below
				continue
			}
			need[k] = true
		}
	}
	for _, c := range accounting.Concepts() {
		need["accounting.concept."+string(c)] = true
	}
	for _, k := range []string{"ae.registered", "ae.authorized", "item.pending", "item.collection_in_progress", "item.collected",
		"settlement.invoice_loaded", "settlement.paid"} {
		need["accounting.state."+k] = true
	}
	for _, k := range []string{"manual", "collection", "commission", "settlement", "reversal"} {
		need["accounting.source."+k] = true
	}
	for _, k := range []string{"real", "virtual"} {
		need["accounting.ae.kind."+k] = true
	}
	// formulas: situations, effects, methods, AE kinds, validation codes, messages
	for _, sit := range accounting.Situations() {
		need["accounting.formula.situation."+string(sit)] = true
		need["accounting.formula.situation."+string(sit)+"_help"] = true
	}
	for _, k := range []string{"set", "surcharge", "discount"} {
		need["accounting.formula.effect."+k] = true
	}
	for _, k := range []string{"percent", "fixed", "oql"} {
		need["accounting.formula.method."+k] = true
	}
	for _, k := range []string{"real", "seam_ae", "seam_web"} {
		need["accounting.formula.ae_kind."+k] = true
	}
	for _, k := range []string{"situation", "name", "effect", "method", "percent", "amount", "country", "concept", "ae_kind", "too_many",
		"oql_empty", "oql_count", "oql_length", "oql_syntax", "oql_select", "oql_forbidden", "oql_table", "oql_param"} {
		need["accounting.err.formula_"+k] = true
	}
	for _, k := range []string{"saved", "enabled", "disabled", "moved", "deleted"} {
		need["accounting.msg.formula_"+k] = true
	}
	// every domain error maps to an existing message
	for _, err := range []error{accounting.ErrAlreadyCollected, accounting.ErrInvoiceMismatch, accounting.ErrNotFound, accounting.ErrConflict,
		accounting.ErrBadMovement, accounting.ErrPeriodClosed, accounting.ErrNoBlobs, accounting.ErrLineUnavailable,
		accounting.ErrFormulaFailed, accounting.ErrOQLDisabled, accounting.ErrNoOQL, errors.New("x")} {
		need[acctErrKey(err)] = true
	}
	for k := range need {
		if _, ok := en[k]; !ok {
			t.Errorf("missing en.json key %q", k)
		}
	}
}

func TestAccountingTenantRestriction(t *testing.T) {
	e := newAcctEnv(t)
	r := testRouter(t, "http://127.0.0.1:1", nil)
	r.config = Config{AppName: "Test", Version: "0", EnableAccounting: true}
	r.handler.config = r.config
	r.SetAccounting(e.svc)
	r.SetAccountingOptions(AccountingOptions{TenantID: 7})
	mux := http.NewServeMux()
	r.MountAccountingRoutes(mux)
	call := func(tenant int64) int {
		req := httptest.NewRequest("GET", "/accounting/ledger", nil)
		req = req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{UserID: 1, TenantID: tenant, Role: auth.RoleAdmin}))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec.Code
	}
	if c := call(7); c != 200 {
		t.Errorf("own tenant = %d", c)
	}
	if c := call(8); c != 403 {
		t.Errorf("other tenant = %d, want 403", c)
	}
}

func TestAccountingAESelfRegisterCanBeDisabled(t *testing.T) {
	e := newAcctEnv(t)
	r := testRouter(t, "http://127.0.0.1:1", nil)
	r.config = Config{AppName: "Test", Version: "0", EnableAccounting: true}
	r.handler.config = r.config
	r.SetAccounting(e.svc)
	r.SetAccountingOptions(AccountingOptions{DisableAESelfRegister: true})
	mux := http.NewServeMux()
	r.MountAccountingRoutes(mux)
	get := func(uid int64) int {
		req := httptest.NewRequest("GET", "/accounting/portal", nil)
		req = req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{UserID: uid, TenantID: 1, Role: auth.RoleAccountExecutive}))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec.Code
	}
	if c := get(acctUserA); c != 200 {
		t.Errorf("known AE = %d", c)
	}
	if c := get(9999); c != 403 {
		t.Errorf("unknown AE with self-registration off = %d, want 403", c)
	}
	if aes, _ := e.svc.ListAEs(context.Background()); len(aes) != 4 { // 2 virtual + 2 real
		t.Errorf("an AE was registered anyway: %d", len(aes))
	}
}

// newAcctRouterMux builds a mux over e's service with the given options.
func (e *acctEnv) withOptions(t *testing.T, o AccountingOptions) *http.ServeMux {
	t.Helper()
	r := testRouter(t, "http://127.0.0.1:1", nil)
	r.config = Config{AppName: "Test", Version: "0", EnableAccounting: true}
	r.handler.config = r.config
	r.SetAccounting(e.svc)
	r.SetAccountingOptions(o)
	mux := http.NewServeMux()
	r.MountAccountingRoutes(mux)
	return mux
}

func serve(mux *http.ServeMux, role string, uid int64, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req = req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{UserID: uid, TenantID: 1, Role: role}))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestAccountingHomeDashboardAndRedirects(t *testing.T) {
	e := newAcctEnv(t)
	// default options: staff see the summary, AEs go to the portal; the admin is
	// not sent to the AE portal (which would be a 403).
	for _, role := range []string{auth.RoleAdmin, auth.RoleAccountant, auth.RoleAdminStaff} {
		rec := e.get(role, 3, "/accounting")
		if rec.Code != 200 {
			t.Fatalf("%s home = %d %s", role, rec.Code, rec.Header().Get("Location"))
		}
	}
	rec := e.get(auth.RoleAccountExecutive, acctUserA, "/accounting")
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/accounting/portal" {
		t.Fatalf("AE home = %d %s", rec.Code, rec.Header().Get("Location"))
	}
	// a figure shows only to users who may see its source
	if body := e.get(auth.RoleAccountant, 3, "/accounting").Body.String(); strings.Contains(body, "accounting.dash.to_pay") == false {
		t.Error("accountant has settlement view; the tile should show")
	}
	if body := e.get(auth.RoleAdminStaff, 3, "/accounting").Body.String(); !strings.Contains(body, "accounting.dash.payable") {
		t.Error("admin_staff has ledger view (read-only); the ledger tiles should show")
	}
	// dashboard off: a redirect to the first screen the user may use
	off := e.withOptions(t, AccountingOptions{DisableDashboard: true})
	rec = serve(off, auth.RoleAccountant, 3, "GET", "/accounting")
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/accounting/collections" {
		t.Fatalf("dashboard off: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	// a viewer has nothing to see
	if rec := e.get(auth.RoleViewer, 3, "/accounting"); rec.Code != 403 {
		t.Errorf("viewer home = %d", rec.Code)
	}
}

func TestAccountingPaging(t *testing.T) {
	e := newAcctEnv(t)
	ctx := context.Background()
	for i := 0; i < 25; i++ {
		if _, err := e.svc.PostEntry(ctx, accounting.Entry{Date: "2026-10-01", Description: "e", Source: accounting.SourceManual,
			Movements: []accounting.Movement{{AccountCode: accounting.CodeBank, Side: accounting.Debit, Amount: 100},
				{AccountCode: accounting.CodeOtherInc, Side: accounting.Credit, Amount: 100}}}, 1); err != nil {
			t.Fatal(err)
		}
	}
	mux := e.withOptions(t, AccountingOptions{PageSize: 10})
	count := func(path string) int {
		return strings.Count(serve(mux, auth.RoleAccountant, 3, "GET", path).Body.String(), `/reverse"`)
	}
	if n := count("/accounting/ledger"); n != 10 {
		t.Errorf("page 1 has %d entries, want 10", n)
	}
	if n := count("/accounting/ledger?page=3"); n == 0 || n >= 10 {
		t.Errorf("last page has %d entries, want a partial page (the 2 collection entries are included)", n)
	}
	body := serve(mux, auth.RoleAccountant, 3, "GET", "/accounting/ledger?page=2").Body.String()
	if !strings.Contains(body, "page=1") || !strings.Contains(body, "page=3") {
		t.Error("page 2 must link to pages 1 and 3")
	}
	// hostile page values fall back, they do not fail
	for _, p := range []string{"page=-4", "page=abc", "page=999999999999"} {
		if rec := serve(mux, auth.RoleAccountant, 3, "GET", "/accounting/ledger?"+p); rec.Code != 200 {
			t.Errorf("%s = %d", p, rec.Code)
		}
	}
}

func TestAccountingSyncProblemsPanel(t *testing.T) {
	e := newAcctEnv(t)
	build := func(o AccountingOptions) (*http.ServeMux, *Router) {
		r := testRouter(t, "http://127.0.0.1:1", nil)
		r.config = Config{AppName: "Test", Version: "0", EnableAccounting: true}
		r.handler.config = r.config
		r.SetAccounting(e.svc)
		r.SetAccountingOptions(o)
		mux := http.NewServeMux()
		r.MountAccountingRoutes(mux)
		return mux, r
	}
	for _, hide := range []bool{false, true} {
		mux, r := build(AccountingOptions{HideSyncProblems: hide})
		r.handler.acct.last = &acctSyncInfo{At: time.Now(), Res: accounting.SyncResult{
			Problems: []string{"unknown AE <img src=x onerror=alert(1)>"}}}
		body := serve(mux, auth.RoleAdminStaff, 3, "GET", "/accounting/collections").Body.String()
		shown := strings.Contains(body, "unknown AE")
		if shown == hide {
			t.Errorf("hide=%v but panel shown=%v", hide, shown)
		}
		if strings.Contains(body, "<img src=x") {
			t.Error("a problem text from the platform is not escaped")
		}
	}
}

func TestAccountingStaffEditAEProfiles(t *testing.T) {
	e := newAcctEnv(t)
	path := "/accounting/aes/" + itoa(e.aeA.ID) + "/edit"
	// default: admin_staff cannot, admin can
	auth.ConfigureAccounting(auth.AccountingGrants{})
	if rec := e.get(auth.RoleAdminStaff, 3, path); rec.Code != 403 {
		t.Errorf("admin_staff edit by default = %d, want 403", rec.Code)
	}
	if rec := e.get(auth.RoleAdmin, 1, path); rec.Code != 200 {
		t.Errorf("admin edit = %d", rec.Code)
	}
	if rec := e.get(auth.RoleAccountExecutive, acctUserB, path); rec.Code != 403 {
		t.Errorf("an AE editing another AE = %d, want 403", rec.Code)
	}
	if rec := e.get(auth.RoleAccountant, 3, path); rec.Code != 403 {
		t.Errorf("accountant edit = %d, want 403", rec.Code)
	}
	auth.ConfigureAccounting(auth.AccountingGrants{StaffEditAEProfiles: true})
	defer auth.ConfigureAccounting(auth.AccountingGrants{})
	if rec := e.get(auth.RoleAdminStaff, 3, path); rec.Code != 200 {
		t.Errorf("admin_staff edit when enabled = %d", rec.Code)
	}
	// virtual AEs cannot be edited
	aes, _ := e.svc.ListAEs(context.Background())
	for _, a := range aes {
		if a.Kind == accounting.AEVirtual {
			if rec := e.get(auth.RoleAdmin, 1, "/accounting/aes/"+itoa(a.ID)+"/edit"); rec.Code != 404 {
				t.Errorf("virtual AE edit = %d, want 404", rec.Code)
			}
		}
	}
}
