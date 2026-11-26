package accounting

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeRunner answers OQL queries from a table keyed by the exact bound text and
// records every query it was asked.
type fakeRunner struct {
	answers map[string][]map[string]any
	asked   []string
	err     error
}

func (f *fakeRunner) Query(_ context.Context, q string) ([]map[string]any, error) {
	f.asked = append(f.asked, q)
	if f.err != nil {
		return nil, f.err
	}
	return f.answers[q], nil
}

func row(v any) []map[string]any { return []map[string]any{{"v": v}} }

func rule(sit Situation, name string, eff FormulaEffect, m FormulaMethod) Formula {
	return Formula{Situation: sit, Name: name, Enabled: true, Effect: eff, Method: m}
}

func pct(sit Situation, name string, eff FormulaEffect, bps int) Formula {
	f := rule(sit, name, eff, MethodPercent)
	f.BPS = bps
	return f
}

func fixed(sit Situation, name string, eff FormulaEffect, c Cents) Formula {
	f := rule(sit, name, eff, MethodFixed)
	f.Amount = c
	return f
}

func eval(t *testing.T, svc *Service, sit Situation, rules []Formula, in FormulaInput) FormulaResult {
	t.Helper()
	r, err := svc.EvaluateFormulas(context.Background(), sit, rules, in)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestFormulaPercentFixedAndEffects(t *testing.T) {
	svc := NewService(NewMemStore())
	in := FormulaInput{Amount: 100000} // USD 1,000.00

	// commission: set 10% of what was paid, then a 5% surcharge on the running
	// commission, then a fixed discount of USD 2
	r := eval(t, svc, SituationCommission, []Formula{
		pct(SituationCommission, "base", EffectSet, 1000),
		pct(SituationCommission, "bonus", EffectSurcharge, 500),
		fixed(SituationCommission, "fee", EffectDiscount, 200),
	}, in)
	if r.Amount != 10300 { // 10000 + 500 - 200
		t.Errorf("amount = %d, want 10300", r.Amount)
	}
	if len(r.Steps) != 3 || r.Steps[0].Before != 0 || r.Steps[0].After != 10000 || r.Steps[1].After != 10500 {
		t.Errorf("steps = %+v", r.Steps)
	}
	if got := r.Detail(); got != "base → bonus → fee" {
		t.Errorf("detail = %q", got)
	}

	// price: the running amount starts at the base price; 20% off, then +USD 5.00
	r = eval(t, svc, SituationPrice, []Formula{
		pct(SituationPrice, "py", EffectDiscount, 2000),
		fixed(SituationPrice, "handling", EffectSurcharge, 500),
	}, in)
	if r.Amount != 80500 {
		t.Errorf("price = %d, want 80500", r.Amount)
	}

	// fixed replacement (design 10.2): a price set outright
	r = eval(t, svc, SituationPrice, []Formula{fixed(SituationPrice, "flat", EffectSet, 99900)}, in)
	if r.Amount != 99900 {
		t.Errorf("replacement = %d", r.Amount)
	}

	// a discount never goes below zero; rounding is half up to the cent
	r = eval(t, svc, SituationPrice, []Formula{fixed(SituationPrice, "big", EffectDiscount, 999999)}, in)
	if r.Amount != 0 {
		t.Errorf("clamped = %d", r.Amount)
	}
	r = eval(t, svc, SituationCommission, []Formula{pct(SituationCommission, "p", EffectSet, 1250)}, FormulaInput{Amount: 1001})
	if r.Amount != 125 { // 125.125 → 125
		t.Errorf("rounding = %d", r.Amount)
	}
	r = eval(t, svc, SituationCommission, []Formula{pct(SituationCommission, "p", EffectSet, 1250)}, FormulaInput{Amount: 1004})
	if r.Amount != 126 { // 125.5 → 126
		t.Errorf("half up = %d", r.Amount)
	}
}

func TestFormulaConditionsDisabledAndStop(t *testing.T) {
	svc := NewService(NewMemStore())
	base := pct(SituationCommission, "base", EffectSet, 1000)
	py := pct(SituationCommission, "py discount", EffectDiscount, 2000)
	py.Country = "py"
	web := fixed(SituationCommission, "web only", EffectSurcharge, 100)
	web.AEKind = SlugSeamWeb
	addon := fixed(SituationCommission, "addons", EffectSurcharge, 7)
	addon.Concept = ConceptAddon
	off := fixed(SituationCommission, "off", EffectSurcharge, 1000)
	off.Enabled = false
	rules := []Formula{base, py, web, addon, off}

	us := FormulaInput{Amount: 10000, Country: "US", Concept: ConceptSubscription, AEKind: string(AEReal)}
	if r := eval(t, svc, SituationCommission, rules, us); r.Amount != 1000 {
		t.Errorf("US real subscription = %d", r.Amount)
	}
	pyIn := us
	pyIn.Country = "PY"
	if r := eval(t, svc, SituationCommission, rules, pyIn); r.Amount != 800 { // country condition is case-insensitive
		t.Errorf("PY = %d", r.Amount)
	}
	webIn := us
	webIn.AEKind = SlugSeamWeb
	webIn.Concept = ConceptAddon
	if r := eval(t, svc, SituationCommission, rules, webIn); r.Amount != 1107 {
		t.Errorf("web add-on = %d", r.Amount)
	}

	// Stop: nothing after the rule that stops applies
	base.Stop = true
	if r := eval(t, svc, SituationCommission, []Formula{base, py}, pyIn); r.Amount != 1000 {
		t.Errorf("stop = %d", r.Amount)
	}
	// an AE condition does not apply to prices (they have no AE)
	p := fixed(SituationPrice, "x", EffectSurcharge, 1)
	p.AEKind = SlugSeamWeb
	if r := eval(t, svc, SituationPrice, []Formula{p}, FormulaInput{Amount: 100}); r.Amount != 101 {
		t.Errorf("price with stale AE condition = %d", r.Amount)
	}
}

func oqlRule(sit Situation, eff FormulaEffect, qs ...string) Formula {
	f := rule(sit, "oql", eff, MethodOQL)
	f.Queries = qs
	return f
}

func TestOQLFormulaChainsQueriesAndBindsParameters(t *testing.T) {
	run := &fakeRunner{answers: map[string][]map[string]any{
		// what this AE collected before, as a number
		"SELECT SUM(amount) AS v FROM adm_commission WHERE ae_id = 7 AND period = '2026-09'": row(float64(250000)),
		// a tier looked up from that total (prev is substituted as an integer literal)
		"SELECT bps AS v FROM adm_item WHERE amount <= 250000 AND note = ':amount'": row("1500"),
		// the amount: running * prev / 10000 is spelled out by the query author
		"SELECT 12345 AS v FROM adm_item WHERE x = 1500 AND y = 100000 AND z = 2": row(12345.4),
	}}
	svc := NewService(NewMemStore(), WithQueryRunner(run))
	f := oqlRule(SituationCommission, EffectSet,
		"SELECT SUM(amount) AS v FROM adm_commission WHERE ae_id = :ae_id AND period = :period",
		"SELECT bps AS v FROM adm_item WHERE amount <= :prev AND note = ':amount'", // :amount inside quotes stays text
		"SELECT 12345 AS v FROM adm_item WHERE x = :prev AND y = :amount AND z = :q1 / 125000",
	)
	// the third query's last condition is not valid arithmetic for the fake; use a simpler last query
	f.Queries[2] = "SELECT 12345 AS v FROM adm_item WHERE x = :prev AND y = :amount AND z = 2"
	if err := ValidateOQLFormula(SituationCommission, f.Queries); err != nil {
		t.Fatal(err)
	}
	r, err := svc.EvaluateFormulas(context.Background(), SituationCommission, []Formula{f},
		FormulaInput{Amount: 100000, AEID: 7, Period: "2026-09"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Amount != 12345 { // 12345.4 rounds to the cent
		t.Errorf("amount = %d", r.Amount)
	}
	if len(run.asked) != 3 || !strings.Contains(run.asked[1], "note = ':amount'") {
		t.Errorf("asked = %q", run.asked)
	}
	if !strings.Contains(r.Steps[0].Note, "3 OQL") {
		t.Errorf("note = %q", r.Steps[0].Note)
	}
}

func TestOQLFormulaResultRules(t *testing.T) {
	q := "SELECT amount AS v FROM adm_item"
	for name, c := range map[string]struct {
		rows    []map[string]any
		runErr  error
		want    Cents
		wantErr bool
	}{
		"number":       {rows: row(float64(1234)), want: 1234},
		"decimal text": {rows: row("12.5"), want: 13},
		"null sum":     {rows: row(nil), want: 0},
		"no rows":      {rows: nil, want: 0},
		"two rows":     {rows: []map[string]any{{"v": 1}, {"v": 2}}, wantErr: true},
		"two columns":  {rows: []map[string]any{{"v": 1, "w": 2}}, wantErr: true},
		"system cols":  {rows: []map[string]any{{"v": 5, "id": 9, "_version": 1}}, want: 5},
		"negative":     {rows: row(float64(-5)), wantErr: true},
		"not a number": {rows: row("abc"), wantErr: true},
		"a bool":       {rows: row(true), wantErr: true},
		"store error":  {runErr: errors.New("boom"), wantErr: true},
	} {
		run := &fakeRunner{answers: map[string][]map[string]any{q: c.rows}, err: c.runErr}
		svc := NewService(NewMemStore(), WithQueryRunner(run))
		r, err := svc.EvaluateFormulas(context.Background(), SituationCommission,
			[]Formula{oqlRule(SituationCommission, EffectSurcharge, q)}, FormulaInput{Amount: 1})
		if c.wantErr {
			if !errors.Is(err, ErrFormulaFailed) {
				t.Errorf("%s: err = %v, want ErrFormulaFailed", name, err)
			}
			continue
		}
		if err != nil || r.Amount != c.want {
			t.Errorf("%s: %d, %v; want %d", name, r.Amount, err, c.want)
		}
	}
	// without a runner, or with OQL switched off, the rule fails loudly
	svc := NewService(NewMemStore())
	if _, err := svc.EvaluateFormulas(context.Background(), SituationCommission,
		[]Formula{oqlRule(SituationCommission, EffectSet, q)}, FormulaInput{}); !errors.Is(err, ErrNoOQL) {
		t.Errorf("no runner: %v", err)
	}
	svc = NewService(NewMemStore(), WithQueryRunner(&fakeRunner{}), WithoutOQLFormulas())
	if _, err := svc.EvaluateFormulas(context.Background(), SituationCommission,
		[]Formula{oqlRule(SituationCommission, EffectSet, q)}, FormulaInput{}); !errors.Is(err, ErrOQLDisabled) {
		t.Errorf("disabled: %v", err)
	}
}

func TestValidateOQLFormulaRejectsWhatCouldHurt(t *testing.T) {
	ok := "SELECT SUM(amount) AS v FROM adm_item WHERE tenant_id = :tenant_id"
	if err := ValidateOQLFormula(SituationCommission, []string{ok}); err != nil {
		t.Fatalf("a plain query is refused: %v", err)
	}
	codes := map[string][]string{
		"oql_empty":     {},
		"oql_count":     {ok, ok, ok, ok, ok, ok},
		"oql_length":    {"SELECT " + strings.Repeat("a", MaxOQLQueryChars) + " FROM adm_item"},
		"oql_syntax":    {"SELECT 1 FROM [users]", `SELECT 1 FROM "users"`, "SELECT 1 FROM `users`", "SELECT 1 FROM adm_item; DELETE FROM adm_item", "SELECT 1 FROM adm_item -- x", "SELECT 1 /* x */ FROM adm_item", "SELECT 1 FROM adm_item WHERE a = 'open"},
		"oql_select":    {"DELETE FROM adm_item", "  update adm_item set amount = 1", "WITH x AS (SELECT 1) SELECT 1"},
		"oql_forbidden": {"SELECT 1 FROM adm_item WHERE a IN (UPDATE adm_item)", "SELECT NEXT VALUE FOR s FROM adm_item", "SELECT @SEQ('s') FROM adm_item", "SELECT 1 FROM adm_item UNION DELETE FROM adm_item"},
		"oql_table": {"SELECT 1 FROM users", "SELECT 1 FROM adm_item JOIN users ON 1 = 1", "SELECT x FROM (SELECT password FROM auth_users) AS t",
			// the module's own people and files are not for formulas
			"SELECT payout_details FROM adm_ae", "SELECT 1 FROM adm_document", "SELECT 1 FROM adm_company", "SELECT 1 FROM adm_instance", "SELECT 1 FROM adm_item JOIN adm_ae ON 1 = 1",
			// a second table smuggled in with a comma
			"SELECT 1 FROM adm_item, users", "SELECT 1 FROM adm_item i, adm_ae a", "SELECT 1 FROM (SELECT 1 FROM adm_item) t, users"},
		"oql_param": {"SELECT :nope FROM adm_item", "SELECT :prev FROM adm_item", "SELECT :q1 FROM adm_item", "SELECT :tenant_id FROM adm_item"},
	}
	for code, qs := range codes {
		if code == "oql_count" || code == "oql_empty" || code == "oql_length" {
			sit := SituationCommission
			err := ValidateOQLFormula(sit, qs)
			var p *FormulaProblem
			if !errors.As(err, &p) || p.Code != code {
				t.Errorf("%s: got %v", code, err)
			}
			continue
		}
		for _, q := range qs {
			sit := SituationCommission
			if strings.Contains(q, ":tenant_id") && code == "oql_param" {
				sit = SituationPrice // tenant_id is not available to prices
			}
			err := ValidateOQLFormula(sit, []string{q})
			var p *FormulaProblem
			if !errors.As(err, &p) || p.Code != code || !errors.Is(err, ErrBadFormula) {
				t.Errorf("%s: %q gave %v", code, q, err)
			}
		}
	}
	// commas that are not a table list stay legal
	for _, q := range []string{
		"SELECT ROUND(SUM(amount), 0) AS v FROM adm_item WHERE concept IN ('a', 'b') ORDER BY period, id",
		"SELECT COALESCE(SUM(c.amount), 0) FROM adm_commission c JOIN adm_item i ON c.item_id = i.id WHERE i.period IN ('2026-08', '2026-09') GROUP BY i.tenant_id, i.period",
		"SELECT SUM(amount) FROM (SELECT amount, id FROM adm_item WHERE tenant_id = :tenant_id) t",
	} {
		if err := ValidateOQLFormula(SituationCommission, []string{q}); err != nil {
			t.Errorf("%q refused: %v", q, err)
		}
	}
	// earlier queries are visible to later ones
	if err := ValidateOQLFormula(SituationCommission, []string{ok, "SELECT :prev FROM adm_item", "SELECT :q1 + :q2 FROM adm_item"}); err != nil {
		t.Errorf("chain refused: %v", err)
	}
	// forbidden words inside a string literal are just text
	if err := ValidateOQLFormula(SituationCommission, []string{"SELECT 1 FROM adm_item WHERE note = 'delete; -- /* @x users'"}); err != nil {
		t.Errorf("text in quotes refused: %v", err)
	}
}

func TestBindOQLEscapesValues(t *testing.T) {
	got, err := bindOQL("SELECT 1 FROM adm_item WHERE c = :country AND n = ':country' AND a = :amount", map[string]any{
		"country": "O'Brien'; DROP", "amount": Cents(42)})
	if err != nil {
		t.Fatal(err)
	}
	want := "SELECT 1 FROM adm_item WHERE c = 'O''Brien''; DROP' AND n = ':country' AND a = 42"
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestSaveFormulaValidationAndNormalization(t *testing.T) {
	e := newEnv(t)
	bad := map[string]Formula{
		"name":      {Situation: SituationCommission, Effect: EffectSet, Method: MethodFixed},
		"effect":    {Situation: SituationCommission, Name: "x", Effect: "x", Method: MethodFixed},
		"method":    {Situation: SituationCommission, Name: "x", Effect: EffectSet, Method: "x"},
		"percent":   {Situation: SituationCommission, Name: "x", Effect: EffectSet, Method: MethodPercent},
		"amount":    {Situation: SituationCommission, Name: "x", Effect: EffectDiscount, Method: MethodFixed},
		"country":   {Situation: SituationCommission, Name: "x", Effect: EffectSet, Method: MethodFixed, Country: "Paraguay"},
		"concept":   {Situation: SituationCommission, Name: "x", Effect: EffectSet, Method: MethodFixed, Concept: "nope"},
		"ae_kind":   {Situation: SituationPrice, Name: "x", Effect: EffectSet, Method: MethodFixed, AEKind: "real"},
		"situation": {Name: "x", Effect: EffectSet, Method: MethodFixed},
	}
	over := Formula{Situation: SituationCommission, Name: "x", Effect: EffectDiscount, Method: MethodPercent, BPS: 10001}
	bad["percent over 100% discount"] = over
	for name, f := range bad {
		_, err := e.svc.SaveFormula(e.ctx, f, 1)
		var p *FormulaProblem
		if !errors.As(err, &p) {
			t.Errorf("%s: got %v", name, err)
		}
	}
	f, err := e.svc.SaveFormula(e.ctx, Formula{Situation: SituationCommission, Name: "  Base  ", Enabled: true, Effect: EffectSet,
		Method: MethodPercent, BPS: 1000, Amount: 5, Queries: []string{"junk"}, Country: "py"}, 9)
	must(t, err)
	if f.Name != "Base" || f.Country != "PY" || f.Amount != 0 || f.Queries != nil || f.Position != 10 || f.UpdatedBy != 9 {
		t.Errorf("not normalized: %+v", f)
	}
	g, err := e.svc.SaveFormula(e.ctx, Formula{Situation: SituationCommission, Name: "Two", Effect: EffectSurcharge, Method: MethodFixed, Amount: 100}, 9)
	must(t, err)
	if g.Position != 20 {
		t.Errorf("second position = %d", g.Position)
	}
	// edit keeps situation and position
	g.Name, g.Situation = "Two!", SituationPrice
	g2, err := e.svc.SaveFormula(e.ctx, g, 9)
	must(t, err)
	if g2.Situation != SituationCommission || g2.Position != 20 {
		t.Errorf("edit changed situation/position: %+v", g2)
	}
	got, err := e.svc.GetFormula(e.ctx, g.ID)
	must(t, err)
	if got.Name != "Two!" || got.Enabled {
		t.Errorf("stored: %+v", got)
	}
	// the OQL method is refused when switched off, and its rules cannot be enabled
	svc := NewService(NewMemStore(), WithoutOQLFormulas())
	if _, err := svc.SaveFormula(e.ctx, oqlRule(SituationCommission, EffectSet, "SELECT 1 FROM adm_item"), 1); !errors.Is(err, ErrOQLDisabled) {
		t.Errorf("off: %v", err)
	}
	// limit per situation
	svc = NewService(NewMemStore())
	for i := 0; i < MaxFormulasPerSituation; i++ {
		_, err := svc.SaveFormula(e.ctx, fixed(SituationPrice, "r", EffectSurcharge, 1), 1)
		must(t, err)
	}
	if _, err := svc.SaveFormula(e.ctx, fixed(SituationPrice, "r", EffectSurcharge, 1), 1); !errors.Is(err, ErrBadFormula) {
		t.Errorf("limit: %v", err)
	}
}

func TestFormulaOrderEnableMoveDelete(t *testing.T) {
	e := newEnv(t)
	mk := func(name string) Formula {
		f, err := e.svc.SaveFormula(e.ctx, fixed(SituationPrice, name, EffectSurcharge, 1), 1)
		must(t, err)
		return f
	}
	a, b, c := mk("a"), mk("b"), mk("c")
	names := func() string {
		rs, err := e.svc.Formulas(e.ctx, SituationPrice)
		must(t, err)
		var s []string
		for _, r := range rs {
			s = append(s, r.Name)
		}
		return strings.Join(s, ",")
	}
	if names() != "a,b,c" {
		t.Fatal(names())
	}
	must(t, e.svc.MoveFormula(e.ctx, c.ID, -1))
	if names() != "a,c,b" {
		t.Errorf("after up: %s", names())
	}
	must(t, e.svc.MoveFormula(e.ctx, a.ID, -1)) // already first: no change
	must(t, e.svc.MoveFormula(e.ctx, b.ID, +1)) // already last: no change
	must(t, e.svc.MoveFormula(e.ctx, a.ID, +1))
	if names() != "c,a,b" {
		t.Errorf("after down: %s", names())
	}
	// other situations are separate
	must(t, e.svc.MoveFormula(e.ctx, c.ID, -1))
	rs, _ := e.svc.Formulas(e.ctx, SituationCommission)
	if len(rs) != 0 {
		t.Error("price rules leaked into commission")
	}
	must(t, e.svc.SetFormulaEnabled(e.ctx, a.ID, false, 5))
	got, _ := e.svc.GetFormula(e.ctx, a.ID)
	if got.Enabled || got.UpdatedBy != 5 {
		t.Errorf("%+v", got)
	}
	must(t, e.svc.DeleteFormula(e.ctx, b.ID))
	if names() != "c,a" {
		t.Errorf("after delete: %s", names())
	}
	wantErr(t, e.svc.SetFormulaEnabled(e.ctx, 99999, true, 1), ErrNotFound)
}

func TestAdjustPrice(t *testing.T) {
	e := newEnv(t)
	r, err := e.svc.AdjustPrice(e.ctx, 10000, "PY", ConceptSubscription)
	must(t, err)
	if r.Amount != 10000 || len(r.Steps) != 0 {
		t.Errorf("no rules should leave the price alone: %+v", r)
	}
	f := pct(SituationPrice, "Paraguay -20%", EffectDiscount, 2000)
	f.Country = "PY"
	_, err = e.svc.SaveFormula(e.ctx, f, 1)
	must(t, err)
	r, _ = e.svc.AdjustPrice(e.ctx, 10000, "PY", ConceptSubscription)
	if r.Amount != 8000 {
		t.Errorf("PY = %d", r.Amount)
	}
	r, _ = e.svc.AdjustPrice(e.ctx, 10000, "US", ConceptSubscription)
	if r.Amount != 10000 {
		t.Errorf("US = %d", r.Amount)
	}
}

// Commission rules take over from the configured calculator once one is
// enabled; with none enabled, the configured calculator is used as before.
func TestCommissionUsesRulesWhenEnabled(t *testing.T) {
	e := newEnv(t) // the env's calculator is a flat 10%
	collect := func(period string, amount Cents) CommissionLine {
		items := e.pull(itoaInt64(e.ae.ID), "t-"+period, period, amount)
		_, err := e.svc.MarkCollected(e.ctx, items[0].ID, CodeBank, "2026-10-01", 1)
		must(t, err)
		lines, err := e.svc.UnsettledLines(e.ctx, e.ae.ID)
		must(t, err)
		for _, l := range lines {
			if l.Period == period {
				return l
			}
		}
		t.Fatalf("no line for %s", period)
		return CommissionLine{}
	}
	if l := collect("2026-01", 100000); l.Amount != 10000 {
		t.Fatalf("legacy calculator: %d", l.Amount)
	}
	base, err := e.svc.SaveFormula(e.ctx, pct(SituationCommission, "Base 15%", EffectSet, 1500), 1)
	must(t, err)
	py := pct(SituationCommission, "Paraguay -10%", EffectDiscount, 1000)
	py.Country = "PY"
	_, err = e.svc.SaveFormula(e.ctx, py, 1)
	must(t, err)
	l := collect("2026-02", 100000) // tenants of the env are in PY
	if l.Amount != 13500 || l.Detail != "Base 15% → Paraguay -10%" {
		t.Errorf("rules: %d %q", l.Amount, l.Detail)
	}
	// switching every rule off returns to the configured calculator
	must(t, e.svc.SetFormulaEnabled(e.ctx, base.ID, false, 1))
	rs, _ := e.svc.Formulas(e.ctx, SituationCommission)
	for _, r := range rs {
		must(t, e.svc.SetFormulaEnabled(e.ctx, r.ID, false, 1))
	}
	if l := collect("2026-03", 100000); l.Amount != 10000 {
		t.Errorf("all off: %d", l.Amount)
	}
	// a rule that nets to nothing books no line
	z, err := e.svc.SaveFormula(e.ctx, fixed(SituationCommission, "zero", EffectSet, 0), 1)
	must(t, err)
	must(t, e.svc.SetFormulaEnabled(e.ctx, z.ID, true, 1))
	items := e.pull(itoaInt64(e.ae.ID), "t-zero", "2026-04", 100000)
	res, err := e.svc.MarkCollected(e.ctx, items[0].ID, CodeBank, "2026-10-01", 1)
	must(t, err)
	if len(res.Lines) != 0 {
		t.Errorf("zero commission booked %d lines", len(res.Lines))
	}
}

// A failing formula must stop the collection rather than book a guess.
func TestFailingCommissionFormulaBlocksCollection(t *testing.T) {
	e := newEnv(t)
	run := &fakeRunner{err: errors.New("xolu down")}
	e.svc.runner = run
	f := oqlRule(SituationCommission, EffectSet, "SELECT 1 FROM adm_item")
	f.Enabled = true
	_, err := e.svc.SaveFormula(e.ctx, f, 1)
	must(t, err)
	must(t, e.svc.SetFormulaEnabled(e.ctx, 1, true, 1))
	items := e.pull(itoaInt64(e.ae.ID), "t-fail", "2026-05", 100000)
	_, err = e.svc.MarkCollected(e.ctx, items[0].ID, CodeBank, "2026-10-01", 1)
	if !errors.Is(err, ErrFormulaFailed) {
		t.Fatalf("err = %v", err)
	}
	it, err := e.svc.GetItem(e.ctx, items[0].ID)
	must(t, err)
	if it.State == ItemCollected {
		t.Error("the item was collected although the formula failed")
	}
	// once the store is back the same item collects
	run.err = nil
	run.answers = map[string][]map[string]any{"SELECT 1 FROM adm_item": row(float64(777))}
	res, err := e.svc.MarkCollected(e.ctx, items[0].ID, CodeBank, "2026-10-01", 1)
	must(t, err)
	if len(res.Lines) != 1 || res.Lines[0].Amount != 777 {
		t.Errorf("lines = %+v", res.Lines)
	}
}

// The Xolu store is picked up as the OQL runner, and a bound query reaches the
// wire exactly as built.
func TestXoluStoreRunsOQLFormulas(t *testing.T) {
	st, f := newFake(t)
	svc := NewService(st)
	if svc.runner == nil {
		t.Fatal("the xolu store should be the query runner")
	}
	f.reply = `{"status":"ok","data":[{"v":"4321.00"}]}`
	rule := oqlRule(SituationCommission, EffectSet, "SELECT SUM(amount) AS v FROM adm_item WHERE tenant_id = :tenant_id AND period = :period")
	r, err := svc.EvaluateFormulas(context.Background(), SituationCommission, []Formula{rule},
		FormulaInput{Amount: 1, TenantID: 12, Period: "2026-09"})
	must(t, err)
	if r.Amount != 4321 {
		t.Errorf("amount = %d", r.Amount)
	}
	if f.path != "/api/v1/oql/query" || f.body["query"] != "SELECT SUM(amount) AS v FROM adm_item WHERE tenant_id = 12 AND period = '2026-09'" {
		t.Errorf("%s %v", f.path, f.body)
	}
	// a never-written entity answers with no rows, which is zero
	f.status, f.reply = 404, `{"error":{"code":"XOLU-ST008","message":"schema not found","status":404}}`
	r, err = svc.EvaluateFormulas(context.Background(), SituationCommission, []Formula{rule}, FormulaInput{Amount: 1})
	must(t, err)
	if r.Amount != 0 || !strings.Contains(r.Steps[0].Note, "no rows") {
		t.Errorf("empty entity: %d %q", r.Amount, r.Steps[0].Note)
	}
}

// What the database answers is data, not a message: a text value that is not an
// amount must not be copied into the error that editors of formulas see.
func TestOQLFormulaErrorDoesNotEchoValues(t *testing.T) {
	q := "SELECT amount AS v FROM adm_item"
	run := &fakeRunner{answers: map[string][]map[string]any{q: row("IBAN-PY00-SECRET")}}
	svc := NewService(NewMemStore(), WithQueryRunner(run))
	_, err := svc.EvaluateFormulas(context.Background(), SituationCommission,
		[]Formula{oqlRule(SituationCommission, EffectSet, q)}, FormulaInput{})
	if !errors.Is(err, ErrFormulaFailed) || strings.Contains(err.Error(), "SECRET") {
		t.Errorf("err = %v", err)
	}
}
