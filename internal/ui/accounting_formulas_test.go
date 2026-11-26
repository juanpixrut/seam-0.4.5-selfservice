package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ha1tch/seam-ui/internal/accounting"
	"github.com/ha1tch/seam-ui/internal/auth"
)

type fmEnv struct {
	mux *http.ServeMux
	svc *accounting.Service
}

func newFormulaEnv(t *testing.T, opts ...accounting.Option) *fmEnv {
	t.Helper()
	svc := accounting.NewService(accounting.NewMemStore(), append([]accounting.Option{accounting.WithBlobs(accounting.NewMemBlobs())}, opts...)...)
	if err := svc.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	r := testRouter(t, "http://127.0.0.1:1", nil)
	r.config = Config{AppName: "Test", Version: "0", EnableAccounting: true}
	r.handler.config = r.config
	r.SetAccounting(svc)
	mux := http.NewServeMux()
	r.MountAccountingRoutes(mux)
	return &fmEnv{mux: mux, svc: svc}
}

func (e *fmEnv) do(role, method, path string, form url.Values) *httptest.ResponseRecorder {
	var req *http.Request
	if form != nil {
		req = httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req = req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{UserID: 1, TenantID: 1, Role: role}))
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req)
	return rec
}

func (e *fmEnv) rules(t *testing.T, sit accounting.Situation) []accounting.Formula {
	t.Helper()
	rs, err := e.svc.Formulas(context.Background(), sit)
	if err != nil {
		t.Fatal(err)
	}
	return rs
}

func loc(rec *httptest.ResponseRecorder) string { return rec.Header().Get("Location") }

// simPart is the simulator's half of the panel page: the rule list above it
// names every rule, applied or not.
func simPart(body string) string {
	if i := strings.Index(body, `id="try"`); i >= 0 {
		return body[i:]
	}
	return ""
}

func TestFormulaPanelAccess(t *testing.T) {
	e := newFormulaEnv(t)
	body := e.do(auth.RoleAdmin, "GET", "/accounting/formulas", nil)
	if body.Code != http.StatusOK || !strings.Contains(body.Body.String(), "/accounting/formulas/new?situation=commission") {
		t.Fatalf("admin: %d", body.Code)
	}
	// the accountant and admin staff may look, not change
	for _, role := range []string{auth.RoleAccountant, auth.RoleAdminStaff} {
		rec := e.do(role, "GET", "/accounting/formulas", nil)
		if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "/accounting/formulas/new") {
			t.Errorf("%s: %d, new-rule link shown or panel missing", role, rec.Code)
		}
		for _, p := range []struct{ m, path string }{{"GET", "/accounting/formulas/new?situation=commission"}, {"POST", "/accounting/formulas"}} {
			if got := e.do(role, p.m, p.path, url.Values{}).Code; got != http.StatusForbidden {
				t.Errorf("%s %s %s = %d, want 403", role, p.m, p.path, got)
			}
		}
	}
	// an AE sees nothing of it
	if got := e.do(auth.RoleAccountExecutive, "GET", "/accounting/formulas", nil).Code; got != http.StatusForbidden {
		t.Errorf("AE: %d", got)
	}
}

// The three methods, saved through the form, are stored as typed.
func TestFormulaSaveThreeMethods(t *testing.T) {
	e := newFormulaEnv(t)
	save := func(f url.Values) {
		t.Helper()
		f.Set("situation", "commission")
		rec := e.do(auth.RoleAdmin, "POST", "/accounting/formulas", f)
		if rec.Code != http.StatusSeeOther || !strings.Contains(loc(rec), "ok=formula_saved") {
			t.Fatalf("%v: %d %s", f, rec.Code, loc(rec))
		}
	}
	save(url.Values{"name": {"Simple"}, "effect": {"set"}, "method": {"percent"}, "percent": {"12,5%"}, "enabled": {"1"}})
	save(url.Values{"name": {"Fixed"}, "effect": {"surcharge"}, "method": {"fixed"}, "amount": {"25.00"}, "country": {"py"}, "concept": {"addon"}})
	save(url.Values{"name": {"Queries"}, "effect": {"discount"}, "method": {"oql"}, "ae_kind": {"real"},
		"queries": {"SELECT SUM(amount) AS v FROM adm_item WHERE tenant_id = :tenant_id\r\n;\r\nSELECT :prev AS v FROM adm_item\r\n"}})
	rs := e.rules(t, accounting.SituationCommission)
	if len(rs) != 3 {
		t.Fatalf("rules = %d", len(rs))
	}
	if r := rs[0]; r.Method != accounting.MethodPercent || r.BPS != 1250 || r.Effect != accounting.EffectSet || !r.Enabled || r.Amount != 0 || len(r.Queries) != 0 {
		t.Errorf("percent rule: %+v", r)
	}
	if r := rs[1]; r.Method != accounting.MethodFixed || r.Amount != 2500 || r.Country != "PY" || r.Concept != accounting.ConceptAddon || r.BPS != 0 || r.Enabled {
		t.Errorf("fixed rule: %+v", r)
	}
	if r := rs[2]; r.Method != accounting.MethodOQL || len(r.Queries) != 2 || r.AEKind != "real" || r.Effect != accounting.EffectDiscount {
		t.Errorf("oql rule: %+v", r)
	}
	// positions follow creation order
	if !(rs[0].Position < rs[1].Position && rs[1].Position < rs[2].Position) {
		t.Errorf("positions: %d %d %d", rs[0].Position, rs[1].Position, rs[2].Position)
	}
	// editing keeps the id and the situation and can change the method
	rec := e.do(auth.RoleAdmin, "POST", "/accounting/formulas/"+itoa(rs[0].ID), url.Values{
		"situation": {"price"}, "name": {"Simple, edited"}, "effect": {"discount"}, "method": {"fixed"}, "amount": {"3"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("edit: %d %s", rec.Code, loc(rec))
	}
	got, _ := e.svc.GetFormula(context.Background(), rs[0].ID)
	if got.Situation != accounting.SituationCommission || got.Name != "Simple, edited" || got.Method != accounting.MethodFixed || got.Amount != 300 || got.BPS != 0 {
		t.Errorf("edited: %+v", got)
	}
}

// A rule that is wrong is not saved, the form comes back with what was typed
// and says what is wrong.
func TestFormulaSaveRejectsAndKeepsInput(t *testing.T) {
	e := newFormulaEnv(t)
	for name, c := range map[string]struct {
		form url.Values
		key  string
	}{
		"percent text":      {url.Values{"name": {"X"}, "effect": {"set"}, "method": {"percent"}, "percent": {"abc"}}, "accounting.err.formula_percent"},
		"discount over 100": {url.Values{"name": {"X"}, "effect": {"discount"}, "method": {"percent"}, "percent": {"150"}}, "accounting.err.formula_percent"},
		"amount text":       {url.Values{"name": {"X"}, "effect": {"set"}, "method": {"fixed"}, "amount": {"lots"}}, "accounting.err.formula_amount"},
		"zero surcharge":    {url.Values{"name": {"X"}, "effect": {"surcharge"}, "method": {"fixed"}, "amount": {"0"}}, "accounting.err.formula_amount"},
		"no name":           {url.Values{"name": {" "}, "effect": {"set"}, "method": {"fixed"}, "amount": {"1"}}, "accounting.err.formula_name"},
		"bad country":       {url.Values{"name": {"X"}, "effect": {"set"}, "method": {"fixed"}, "amount": {"1"}, "country": {"PRY"}}, "accounting.err.formula_country"},
		"oql delete":        {url.Values{"name": {"X"}, "effect": {"set"}, "method": {"oql"}, "queries": {"DELETE FROM adm_item"}}, "accounting.err.formula_oql_select"},
		"oql stacked":       {url.Values{"name": {"X"}, "effect": {"set"}, "method": {"oql"}, "queries": {"SELECT 1 FROM adm_item; DELETE FROM adm_item"}}, "accounting.err.formula_oql_syntax"},
		"oql other table":   {url.Values{"name": {"X"}, "effect": {"set"}, "method": {"oql"}, "queries": {"SELECT payout_details FROM adm_ae"}}, "accounting.err.formula_oql_table"},
		"oql comma join":    {url.Values{"name": {"X"}, "effect": {"set"}, "method": {"oql"}, "queries": {"SELECT 1 FROM adm_item, adm_ae"}}, "accounting.err.formula_oql_table"},
		"oql none":          {url.Values{"name": {"X"}, "effect": {"set"}, "method": {"oql"}, "queries": {"  \n ; \n"}}, "accounting.err.formula_oql_empty"},
		"oql bad param":     {url.Values{"name": {"X"}, "effect": {"set"}, "method": {"oql"}, "queries": {"SELECT :nope FROM adm_item"}}, "accounting.err.formula_oql_param"},
	} {
		c.form.Set("situation", "commission")
		rec := e.do(auth.RoleAdmin, "POST", "/accounting/formulas", c.form)
		body := rec.Body.String()
		if rec.Code != http.StatusOK || !strings.Contains(body, c.key) {
			t.Errorf("%s: %d, want the form back with %s", name, rec.Code, c.key)
		}
		if n := len(e.rules(t, accounting.SituationCommission)); n != 0 {
			t.Fatalf("%s: a bad rule was stored", name)
		}
	}
	// what was typed is still in the form
	rec := e.do(auth.RoleAdmin, "POST", "/accounting/formulas", url.Values{
		"situation": {"commission"}, "name": {"Keep me"}, "effect": {"set"}, "method": {"percent"}, "percent": {"oops"}})
	if !strings.Contains(rec.Body.String(), `value="Keep me"`) {
		t.Error("the name typed was lost")
	}
}

func TestFormulaOQLSwitchedOff(t *testing.T) {
	e := newFormulaEnv(t, accounting.WithoutOQLFormulas())
	form := e.do(auth.RoleAdmin, "GET", "/accounting/formulas/new?situation=commission", nil).Body.String()
	if strings.Contains(form, `value="oql"`) || strings.Contains(form, `name="queries"`) {
		t.Error("the OQL method is offered although it is off")
	}
	rec := e.do(auth.RoleAdmin, "POST", "/accounting/formulas", url.Values{
		"situation": {"commission"}, "name": {"Q"}, "effect": {"set"}, "method": {"oql"}, "queries": {"SELECT 1 FROM adm_item"}})
	if !strings.Contains(rec.Body.String(), "accounting.err.oql_disabled") || len(e.rules(t, accounting.SituationCommission)) != 0 {
		t.Error("an OQL rule was saved while OQL is off")
	}
	// the two other methods are unaffected
	rec = e.do(auth.RoleAdmin, "POST", "/accounting/formulas", url.Values{
		"situation": {"commission"}, "name": {"P"}, "effect": {"set"}, "method": {"percent"}, "percent": {"5"}})
	if rec.Code != http.StatusSeeOther {
		t.Errorf("percent with OQL off: %d", rec.Code)
	}
}

func TestFormulaOrderEnableDelete(t *testing.T) {
	e := newFormulaEnv(t)
	ctx := context.Background()
	var ids []int64
	for _, n := range []string{"A", "B", "C"} {
		f, err := e.svc.SaveFormula(ctx, accounting.Formula{Situation: accounting.SituationPrice, Name: n, Effect: accounting.EffectDiscount,
			Method: accounting.MethodPercent, BPS: 100, Enabled: true}, 1)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, f.ID)
	}
	names := func() string {
		var s string
		for _, r := range e.rules(t, accounting.SituationPrice) {
			s += r.Name
		}
		return s
	}
	e.do(auth.RoleAdmin, "POST", "/accounting/formulas/"+itoa(ids[2])+"/move", url.Values{"dir": {"up"}})
	if names() != "ACB" {
		t.Errorf("after move up: %s", names())
	}
	e.do(auth.RoleAdmin, "POST", "/accounting/formulas/"+itoa(ids[0])+"/move", url.Values{"dir": {"down"}})
	if names() != "CAB" {
		t.Errorf("after move down: %s", names())
	}
	rec := e.do(auth.RoleAdmin, "POST", "/accounting/formulas/"+itoa(ids[1])+"/enable", url.Values{"enabled": {"0"}})
	if !strings.Contains(loc(rec), "ok=formula_disabled") {
		t.Errorf("disable: %s", loc(rec))
	}
	if f, _ := e.svc.GetFormula(ctx, ids[1]); f.Enabled {
		t.Error("still enabled")
	}
	// deletion goes through the same permission and removes only that rule
	if got := e.do(auth.RoleAccountant, "POST", "/accounting/formulas/"+itoa(ids[1])+"/delete", url.Values{}).Code; got != http.StatusForbidden {
		t.Errorf("accountant delete: %d", got)
	}
	rec = e.do(auth.RoleAdmin, "POST", "/accounting/formulas/"+itoa(ids[1])+"/delete", url.Values{})
	if !strings.Contains(loc(rec), "ok=formula_deleted") || names() != "CA" {
		t.Errorf("delete: %s %s", loc(rec), names())
	}
	// deleting is offered on the rule's own page, and asks first
	edit := e.do(auth.RoleAdmin, "GET", "/accounting/formulas/"+itoa(ids[0])+"/edit", nil).Body.String()
	if !strings.Contains(edit, `action="/accounting/formulas/`+itoa(ids[0])+`/delete"`) || !strings.Contains(edit, "danger:true") {
		t.Error("deleting a rule must be offered on its page and ask first")
	}
	if strings.Contains(e.do(auth.RoleAdmin, "GET", "/accounting/formulas/new?situation=price", nil).Body.String(), "/delete") {
		t.Error("a new rule has nothing to delete")
	}
}

// The simulator applies saved rules in order: a base percentage, a discount
// and a fixed surcharge, each on the running amount.
func TestFormulaSimulatorShowsEachStep(t *testing.T) {
	e := newFormulaEnv(t)
	ctx := context.Background()
	for _, f := range []accounting.Formula{
		{Situation: accounting.SituationCommission, Name: "Base", Effect: accounting.EffectSet, Method: accounting.MethodPercent, BPS: 1000, AEKind: "real", Enabled: true},
		{Situation: accounting.SituationCommission, Name: "PY promo", Effect: accounting.EffectDiscount, Method: accounting.MethodPercent, BPS: 2000, Country: "PY", Enabled: true},
		{Situation: accounting.SituationCommission, Name: "Add-on bonus", Effect: accounting.EffectSurcharge, Method: accounting.MethodFixed, Amount: 500, Concept: accounting.ConceptAddon, Enabled: true},
		{Situation: accounting.SituationCommission, Name: "Off rule", Effect: accounting.EffectSurcharge, Method: accounting.MethodFixed, Amount: 99900, Enabled: false},
	} {
		if _, err := e.svc.SaveFormula(ctx, f, 1); err != nil {
			t.Fatal(err)
		}
	}
	q := "/accounting/formulas?situation=commission&run=1&amount=1000.00&ae_kind=real&country=py&concept=addon"
	body := e.do(auth.RoleAccountant, "GET", q, nil).Body.String() // viewing is enough to try
	for _, want := range []string{"USD 85.00", "USD 100.00", "USD 80.00", "Base", "PY promo", "Add-on bonus"} {
		if !strings.Contains(body, want) {
			t.Errorf("simulator lacks %q", want)
		}
	}
	if sim := simPart(body); strings.Contains(sim, "Off rule") || strings.Contains(sim, "USD 999.00") {
		t.Error("a switched-off rule was applied")
	}
	// another country and concept: only the base applies
	body = e.do(auth.RoleAdmin, "GET", "/accounting/formulas?situation=commission&run=1&amount=1000.00&ae_kind=real&country=US", nil).Body.String()
	if sim := simPart(body); !strings.Contains(sim, "USD 100.00") || strings.Contains(sim, "PY promo") {
		t.Error("conditions were not respected")
	}
	// a bad amount is reported, not computed
	body = e.do(auth.RoleAdmin, "GET", "/accounting/formulas?situation=commission&run=1&amount=abc", nil).Body.String()
	if !strings.Contains(body, "accounting.err.bad_amount") {
		t.Error("bad amount not reported")
	}
	// try one rule only, even a switched-off one
	rs := e.rules(t, accounting.SituationCommission)
	body = e.do(auth.RoleAdmin, "GET", "/accounting/formulas?situation=commission&run=1&amount=10&rule="+itoa(rs[3].ID), nil).Body.String()
	if !strings.Contains(simPart(body), "USD 999.00") {
		t.Error("trying a single rule failed")
	}
}

// OQL rules run through the simulator with the real chain of parameters.
func TestFormulaSimulatorRunsOQL(t *testing.T) {
	run := &uiRunner{rows: []map[string]any{{"v": 4321}}}
	e := newFormulaEnv(t, accounting.WithQueryRunner(run))
	if _, err := e.svc.SaveFormula(context.Background(), accounting.Formula{Situation: accounting.SituationCommission, Name: "By tenant",
		Effect: accounting.EffectSet, Method: accounting.MethodOQL, Enabled: true,
		Queries: []string{"SELECT SUM(amount) AS v FROM adm_item WHERE period = :period AND amount <= :amount"}}, 1); err != nil {
		t.Fatal(err)
	}
	body := e.do(auth.RoleAdmin, "GET", "/accounting/formulas?situation=commission&run=1&amount=1000.00&period=2026-09", nil).Body.String()
	if !strings.Contains(body, "USD 43.21") {
		t.Errorf("OQL amount missing from the result")
	}
	if len(run.asked) != 1 || run.asked[0] != "SELECT SUM(amount) AS v FROM adm_item WHERE period = '2026-09' AND amount <= 100000" {
		t.Errorf("asked = %q", run.asked)
	}
	// a failing store is reported to the person who can fix it, not hidden
	run.err = context.DeadlineExceeded
	body = e.do(auth.RoleAdmin, "GET", "/accounting/formulas?situation=commission&run=1&amount=1000.00", nil).Body.String()
	if !strings.Contains(body, "accounting.err.formula_failed") {
		t.Error("a failing query must be reported")
	}
}

type uiRunner struct {
	rows  []map[string]any
	err   error
	asked []string
}

func (r *uiRunner) Query(_ context.Context, q string) ([]map[string]any, error) {
	r.asked = append(r.asked, q)
	return r.rows, r.err
}

func TestFormulaPagesEscapeAndHaveUniqueIDs(t *testing.T) {
	e := newFormulaEnv(t)
	f, err := e.svc.SaveFormula(context.Background(), accounting.Formula{Situation: accounting.SituationCommission, Name: `<script>alert(1)</script>`,
		Effect: accounting.EffectSet, Method: accounting.MethodOQL, Enabled: false,
		Queries: []string{"SELECT 1 FROM adm_item WHERE note = '</textarea><script>alert(2)</script>'"}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"/accounting/formulas", "/accounting/formulas?situation=commission&run=1&amount=1&rule=" + itoa(f.ID),
		"/accounting/formulas/new?situation=price", "/accounting/formulas/" + itoa(f.ID) + "/edit"} {
		rec := e.do(auth.RoleAdmin, "GET", p, nil)
		body := rec.Body.String()
		if rec.Code != http.StatusOK {
			t.Errorf("%s: %d", p, rec.Code)
		}
		if strings.Contains(body, "<script>alert(") || strings.Contains(body, "</textarea><script>") {
			t.Errorf("%s: unescaped text", p)
		}
		assertUniqueIDs(t, p, body)
	}
	if got := e.do(auth.RoleAdmin, "GET", "/accounting/formulas/999/edit", nil).Code; got != http.StatusNotFound {
		t.Errorf("unknown rule: %d", got)
	}
	if got := e.do(auth.RoleAdmin, "GET", "/accounting/formulas/new?situation=bogus", nil).Code; got != http.StatusNotFound {
		t.Errorf("unknown situation: %d", got)
	}
}

func TestFormulaErrorKeys(t *testing.T) {
	for err, want := range map[error]string{
		accounting.ErrFormulaFailed:                 "accounting.err.formula_failed",
		accounting.ErrOQLDisabled:                   "accounting.err.oql_disabled",
		accounting.ErrNoOQL:                         "accounting.err.no_oql",
		&accounting.FormulaProblem{Code: "percent"}: "accounting.err.formula_percent",
	} {
		if got := acctErrKey(err); got != want {
			t.Errorf("%v: %s, want %s", err, got, want)
		}
	}
}
