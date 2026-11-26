package ui

// Accounting & Admin: the formulas master panel. Rules (percentage, fixed
// amount or OQL; discount, surcharge or replace) are grouped by the situation
// they serve; a simulator evaluates the saved rules for sample inputs.

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	mi "github.com/ha1tch/minty"

	"github.com/ha1tch/seam-ui/internal/accounting"
	"github.com/ha1tch/seam-ui/internal/auth"
)

const formulasPath = "/accounting/formulas"

func (q *acctReq) situationLabel(s accounting.Situation) string {
	return q.t("accounting.formula.situation." + string(s))
}

func (q *acctReq) effectLabel(e accounting.FormulaEffect) string {
	return q.t("accounting.formula.effect." + string(e))
}

func (q *acctReq) aeKindLabel(k string) string {
	if k == "" {
		return q.t("accounting.formula.any")
	}
	return q.t("accounting.formula.ae_kind." + k)
}

func (q *acctReq) conceptLabel(c accounting.Concept) string {
	if c == "" {
		return q.t("accounting.formula.any")
	}
	return q.t("accounting.concept." + string(c))
}

// figureText is what a rule yields, for the list.
func (q *acctReq) figureText(f accounting.Formula) string {
	if f.Method == accounting.MethodOQL {
		return q.t("accounting.formula.figure_oql", fmtInt(len(f.Queries)))
	}
	return f.Figure()
}

// appliesText summarizes a rule's conditions.
func (q *acctReq) appliesText(f accounting.Formula) string {
	var parts []string
	if f.AEKind != "" {
		parts = append(parts, q.aeKindLabel(f.AEKind))
	}
	if f.Country != "" {
		parts = append(parts, f.Country)
	}
	if f.Concept != "" {
		parts = append(parts, q.conceptLabel(f.Concept))
	}
	if len(parts) == 0 {
		return q.t("accounting.formula.always")
	}
	return strings.Join(parts, " · ")
}

func effectVariant(e accounting.FormulaEffect) string {
	switch e {
	case accounting.EffectSurcharge:
		return "warning"
	case accounting.EffectDiscount:
		return "info"
	}
	return "draft"
}

// ---------------------------------------------------------------------------
// the panel
// ---------------------------------------------------------------------------

func (h *UIHandler) AcctFormulas(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	t := q.t
	canEdit := q.can(auth.PermAccountingFormulaEdit)
	parts := []mi.H{acctTitle(t, "accounting.formula.title"), q.banner(),
		func(b *mi.Builder) mi.Node {
			return b.P(mi.Class("text-sm text-gray-600 dark:text-gray-300"), t("accounting.formula.intro"))
		}}
	rulesBySit := map[accounting.Situation][]accounting.Formula{}
	for _, sit := range accounting.Situations() {
		rules, err := q.svc.Formulas(q.ctx(), sit)
		if err != nil {
			q.fail(err)
			return
		}
		rulesBySit[sit] = rules
		parts = append(parts, q.situationPanel(sit, rules, canEdit))
	}
	parts = append(parts, q.tryPanel(rulesBySit))
	q.render(t("accounting.formula.title"), formulasPath, acctPage(parts...))
}

func (q *acctReq) situationPanel(sit accounting.Situation, rules []accounting.Formula, canEdit bool) mi.H {
	t := q.t
	csrf := q.csrf()
	var rows [][]mi.H
	for i, f := range rules {
		f, i := f, i
		id := itoa(f.ID)
		status := []mi.H{Badge(t("accounting.formula.off"), "draft")}
		if f.Enabled {
			status = []mi.H{Badge(t("accounting.formula.on"), "success")}
		}
		if f.Stop {
			status = append(status, Badge(t("accounting.formula.stop"), "warning"))
		}
		var btns []mi.H
		btns = append(btns, acctLink(formulasPath+"?situation="+string(sit)+"&rule="+id+"#try", t("accounting.formula.try_rule")))
		if canEdit {
			btns = append(btns,
				q.formulaIconForm(formulasPath+"/"+id+"/move", csrf, "dir", "up", "arrow_upward", t("accounting.formula.move_up"), i == 0),
				q.formulaIconForm(formulasPath+"/"+id+"/move", csrf, "dir", "down", "arrow_downward", t("accounting.formula.move_down"), i == len(rules)-1),
				acctLink(formulasPath+"/"+id+"/edit", t("accounting.formula.edit")))
			toggle, label := "1", t("accounting.formula.enable")
			if f.Enabled {
				toggle, label = "0", t("accounting.formula.disable")
			}
			btns = append(btns, acctForm(formulasPath+"/"+id+"/enable", csrf, false, "",
				func(b *mi.Builder) mi.Node { return b.Input(mi.Type("hidden"), mi.Name("enabled"), mi.Value(toggle)) },
				func(b *mi.Builder) mi.Node {
					return b.Button(mi.Type("submit"), mi.Class(acctBtnGrey), label)
				}))
		}
		rows = append(rows, []mi.H{
			cellN(itoa(int64(i + 1))),
			cellT(f.Name),
			cellH(func(b *mi.Builder) mi.Node {
				return b.Div(mi.Class("whitespace-nowrap"), Badge(q.effectLabel(f.Effect), effectVariant(f.Effect))(b))
			}),
			cellH(func(b *mi.Builder) mi.Node {
				return b.Div(mi.Class("whitespace-nowrap"),
					b.Div(mi.Class("font-medium"), q.figureText(f)),
					b.Div(mi.Class("text-xs text-gray-500 dark:text-gray-400"), t("accounting.formula.method."+string(f.Method))))
			}),
			cellT(q.appliesText(f)),
			cellH(func(b *mi.Builder) mi.Node {
				args := []interface{}{mi.Class("flex flex-wrap gap-1")}
				for _, s := range status {
					args = append(args, s(b))
				}
				return b.Div(args...)
			}),
			cellRight(func(b *mi.Builder) mi.Node {
				args := []interface{}{mi.Class("flex justify-end gap-2")}
				for _, x := range btns {
					args = append(args, x(b))
				}
				return b.Div(args...)
			}),
		})
	}
	table := acctTable([]string{"#", t("accounting.formula.col.name"), t("accounting.formula.col.effect"),
		t("accounting.formula.col.figure"), t("accounting.formula.col.applies"), t("accounting.formula.col.status"), ""},
		rows, t("accounting.formula.none"))
	return acctPanel(q.situationLabel(sit), func(b *mi.Builder) mi.Node {
		args := []interface{}{b.Div(mi.Class("px-6 pt-4 text-sm text-gray-500 dark:text-gray-400"), t("accounting.formula.situation."+string(sit)+"_help")),
			table(b)}
		if canEdit {
			args = append(args, b.Div(mi.Class("px-6 pb-4 pt-2"),
				acctLink(formulasPath+"/new?situation="+string(sit), t("accounting.formula.new"))(b)))
		}
		return b.Div(args...)
	})
}

// formulaIconForm is a one-button form with an icon, for moving a rule.
func (q *acctReq) formulaIconForm(action, csrf, name, value, icon, label string, disabled bool) mi.H {
	return acctForm(action, csrf, false, "",
		func(b *mi.Builder) mi.Node { return b.Input(mi.Type("hidden"), mi.Name(name), mi.Value(value)) },
		func(b *mi.Builder) mi.Node {
			args := []interface{}{mi.Type("submit"), mi.Class(acctBtnGrey), mi.Attr("aria-label", label), mi.Attr("title", label)}
			if disabled {
				args = append(args, mi.Attr("disabled", "disabled"))
			}
			return b.Button(append(args, b.Span(mi.Class("material-icons text-base align-middle"), icon))...)
		})
}

// ---------------------------------------------------------------------------
// the simulator
// ---------------------------------------------------------------------------

func (q *acctReq) tryPanel(rulesBySit map[accounting.Situation][]accounting.Formula) mi.H {
	t := q.t
	sit := accounting.Situation(q.queryText("situation"))
	if !sit.Valid() {
		sit = accounting.SituationCommission
	}
	ruleID, _ := strconv.ParseInt(q.queryText("rule"), 10, 64)
	var only *accounting.Formula
	if ruleID != 0 {
		for _, f := range rulesBySit[sit] {
			if f.ID == ruleID {
				f := f
				only = &f
			}
		}
	}
	amount := q.queryText("amount")
	if amount == "" {
		amount = "1000.00"
	}
	country, concept, aeKind, period := strings.ToUpper(q.queryText("country")), q.queryText("concept"), q.queryText("ae_kind"), q.queryText("period")
	sitOpts := []acctOpt{}
	for _, s := range accounting.Situations() {
		sitOpts = append(sitOpts, acctOpt{string(s), q.situationLabel(s)})
	}
	conceptOpts := []acctOpt{{"", t("accounting.formula.any")}}
	for _, c := range []accounting.Concept{accounting.ConceptSubscription, accounting.ConceptStartup, accounting.ConceptAddon, accounting.ConceptServices, accounting.ConceptOther} {
		conceptOpts = append(conceptOpts, acctOpt{string(c), q.conceptLabel(c)})
	}
	kindOpts := []acctOpt{{"", t("accounting.formula.any")}}
	for _, k := range []string{string(accounting.AEReal), accounting.SlugSeamAE, accounting.SlugSeamWeb} {
		kindOpts = append(kindOpts, acctOpt{k, q.aeKindLabel(k)})
	}
	fields := []mi.H{
		func(b *mi.Builder) mi.Node {
			return b.Div(mi.Class("grid grid-cols-1 sm:grid-cols-3 gap-4"),
				acctField(t("accounting.formula.try.situation"), "situation", "select", string(sit), sitOpts)(b),
				acctField(t("accounting.formula.try.amount"), "amount", "text", amount, mi.Attr("inputmode", "decimal"))(b),
				acctField(t("accounting.formula.field.concept"), "concept", "select", concept, conceptOpts)(b),
				acctField(t("accounting.formula.field.country"), "country", "text", country, mi.Attr("maxlength", "2"), mi.Placeholder("PY"))(b),
				acctField(t("accounting.formula.field.ae_kind"), "ae_kind", "select", aeKind, kindOpts)(b),
				acctField(t("accounting.formula.try.period"), "period", "text", period, mi.Placeholder("YYYY-MM"))(b))
		},
		func(b *mi.Builder) mi.Node {
			return b.Div(b.Input(mi.Type("hidden"), mi.Name("run"), mi.Value("1")))
		},
		func(b *mi.Builder) mi.Node {
			if only == nil {
				return b.Text("")
			}
			return b.Div(mi.Class("flex items-center gap-3 text-sm text-gray-600 dark:text-gray-300"),
				b.Input(mi.Type("hidden"), mi.Name("rule"), mi.Value(itoa(only.ID))),
				b.Span(t("accounting.formula.try.only", only.Name)),
				acctLink(formulasPath+"?situation="+string(sit)+"#try", t("accounting.formula.try.all"))(b))
		},
		acctSubmit(t("accounting.formula.try.run")),
	}
	var result mi.H
	if q.queryText("run") == "1" {
		result = q.tryResult(sit, only, rulesBySit[sit], amount, country, concept, aeKind, period)
	}
	form := func(b *mi.Builder) mi.Node {
		args := []interface{}{mi.Method("get"), mi.Action(formulasPath + "#try"), mi.Class("space-y-4")}
		for _, f := range fields {
			args = append(args, f(b))
		}
		return b.Form(args...)
	}
	return acctPanel(t("accounting.formula.try.title"), acctBody(
		func(b *mi.Builder) mi.Node {
			return b.Div(mi.ID("try"), mi.Class("text-sm text-gray-500 dark:text-gray-400"), t("accounting.formula.try.hint"))
		}, form, result))
}

func (q *acctReq) tryResult(sit accounting.Situation, only *accounting.Formula, rules []accounting.Formula, amount, country, concept, aeKind, period string) mi.H {
	t := q.t
	cents, err := accounting.ParseUSD(strings.TrimSpace(amount))
	if err != nil || cents < 0 {
		return Alert(t("accounting.err.bad_amount"), "error", false)
	}
	if only != nil {
		rules = []accounting.Formula{*only}
		o := *only
		o.Enabled = true // a rule that is switched off can still be tried
		rules[0] = o
	}
	in := accounting.FormulaInput{Amount: cents, Country: country, Concept: accounting.Concept(concept), AEKind: aeKind, Period: period}
	res, err := q.svc.EvaluateFormulas(q.ctx(), sit, rules, in)
	if err != nil {
		msg := t(acctErrKey(err))
		if q.can(auth.PermAccountingFormulaEdit) {
			msg += " " + err.Error() // the detail (which query, what the store said) is for those who can fix it
		}
		return Alert(msg, "error", false)
	}
	var rows [][]mi.H
	for _, s := range res.Steps {
		rows = append(rows, []mi.H{
			cellT(s.Formula.Name), cellH(Badge(q.effectLabel(s.Formula.Effect), effectVariant(s.Formula.Effect))),
			cellN(s.Note), moneyCell(s.Before), moneyCell(s.After),
		})
	}
	table := acctTable([]string{t("accounting.formula.col.name"), t("accounting.formula.col.effect"), t("accounting.formula.try.note"),
		rh(t("accounting.formula.try.before")), rh(t("accounting.formula.try.after"))}, rows, t("accounting.formula.try.none"))
	return func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("space-y-3 pt-2"),
			b.Div(mi.Class("flex items-baseline justify-between rounded-lg bg-gray-50 dark:bg-gray-900 px-4 py-3"),
				b.Span(mi.Class("text-sm text-gray-600 dark:text-gray-300"), t("accounting.formula.try.result")),
				b.Span(mi.Class("text-xl font-semibold text-gray-900 dark:text-white tabular-nums"), money(res.Amount))),
			b.Div(mi.Class("overflow-x-auto"), table(b)))
	}
}

// ---------------------------------------------------------------------------
// the rule form
// ---------------------------------------------------------------------------

var (
	percentRE = regexp.MustCompile(`^([0-9]{1,4})(?:[.,]([0-9]{1,2}))?$`)
	oqlSepRE  = regexp.MustCompile(`(?m)^[ \t]*;[ \t]*$`)
)

// parsePercent reads "12.5" or "12,5%" as basis points.
func parsePercent(s string) (int, bool) {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "%"))
	m := percentRE.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	whole, _ := strconv.Atoi(m[1])
	frac := 0
	if m[2] != "" {
		frac, _ = strconv.Atoi((m[2] + "0")[:2])
	}
	return whole*100 + frac, true
}

// splitQueries cuts the text of the queries box at lines holding only ";".
func splitQueries(s string) []string {
	var out []string
	for _, p := range oqlSepRE.Split(strings.ReplaceAll(s, "\r\n", "\n"), -1) {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func joinQueries(qs []string) string { return strings.Join(qs, "\n;\n") }

// readFormula reads the rule form. A figure that cannot be read is reported
// like any other invalid rule.
func readFormula(r *http.Request, sit accounting.Situation) (accounting.Formula, error) {
	f := accounting.Formula{
		Situation: sit,
		Name:      r.FormValue("name"),
		Effect:    accounting.FormulaEffect(r.FormValue("effect")),
		Method:    accounting.FormulaMethod(r.FormValue("method")),
		AEKind:    r.FormValue("ae_kind"),
		Country:   r.FormValue("country"),
		Concept:   accounting.Concept(r.FormValue("concept")),
		Enabled:   r.FormValue("enabled") == "1",
		Stop:      r.FormValue("stop") == "1",
	}
	var bad error
	switch f.Method {
	case accounting.MethodPercent:
		bps, ok := parsePercent(r.FormValue("percent"))
		if !ok {
			bad = &accounting.FormulaProblem{Code: "percent"}
		}
		f.BPS = bps
	case accounting.MethodFixed:
		c, err := accounting.ParseUSD(strings.TrimSpace(r.FormValue("amount")))
		if err != nil {
			bad = &accounting.FormulaProblem{Code: "amount"}
		}
		f.Amount = c
	case accounting.MethodOQL:
		f.Queries = splitQueries(r.FormValue("queries"))
	}
	return f, bad
}

func (h *UIHandler) AcctFormulaNew(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	sit := accounting.Situation(q.queryText("situation"))
	if !sit.Valid() {
		q.notFound()
		return
	}
	q.formulaForm(accounting.Formula{Situation: sit, Enabled: true, Effect: accounting.EffectSet, Method: accounting.MethodPercent}, "")
}

func (h *UIHandler) AcctFormulaEdit(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	id, ok := pathID(r)
	if !ok {
		q.notFound()
		return
	}
	f, err := q.svc.GetFormula(q.ctx(), id)
	if err != nil {
		if errors.Is(err, accounting.ErrNotFound) {
			q.notFound()
		} else {
			q.fail(err)
		}
		return
	}
	q.formulaForm(f, "")
}

// formulaForm draws the rule form with f's values, and an error above it.
func (q *acctReq) formulaForm(f accounting.Formula, errKey string) {
	t := q.t
	action := formulasPath
	if f.ID != 0 {
		action = formulasPath + "/" + itoa(f.ID)
	}
	effectOpts := []acctOpt{}
	for _, e := range []accounting.FormulaEffect{accounting.EffectSet, accounting.EffectSurcharge, accounting.EffectDiscount} {
		effectOpts = append(effectOpts, acctOpt{string(e), q.effectLabel(e)})
	}
	methodOpts := []acctOpt{{string(accounting.MethodPercent), t("accounting.formula.method.percent")},
		{string(accounting.MethodFixed), t("accounting.formula.method.fixed")}}
	oqlOn := q.svc.OQLFormulasEnabled()
	if oqlOn || f.Method == accounting.MethodOQL {
		methodOpts = append(methodOpts, acctOpt{string(accounting.MethodOQL), t("accounting.formula.method.oql")})
	}
	conceptOpts := []acctOpt{{"", t("accounting.formula.any")}}
	for _, c := range []accounting.Concept{accounting.ConceptSubscription, accounting.ConceptStartup, accounting.ConceptAddon, accounting.ConceptServices, accounting.ConceptOther} {
		conceptOpts = append(conceptOpts, acctOpt{string(c), q.conceptLabel(c)})
	}
	percent, amount := "", ""
	if f.BPS > 0 {
		percent = strings.TrimSuffix(strings.TrimSuffix(f.Figure(), "%"), "")
	}
	if f.Amount > 0 || f.Method == accounting.MethodFixed {
		amount = f.Amount.Display()
		amount = strings.ReplaceAll(amount, ",", "")
	}
	hint := func(key string) mi.H {
		return func(b *mi.Builder) mi.Node {
			return b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400"), t(key))
		}
	}
	block := func(id string, children ...mi.H) mi.H {
		return func(b *mi.Builder) mi.Node {
			args := []interface{}{mi.ID(id), mi.Class("space-y-2")}
			for _, c := range children {
				args = append(args, c(b))
			}
			return b.Div(args...)
		}
	}
	check := func(name, label, help string, on bool) mi.H {
		return func(b *mi.Builder) mi.Node {
			in := []mi.Attribute{mi.Type("checkbox"), mi.Name(name), mi.Value("1"), mi.ID("f_" + name)}
			if on {
				in = append(in, mi.Checked())
			}
			return b.Div(b.Label(mi.Class("inline-flex items-center gap-2 text-sm text-gray-900 dark:text-white"), b.Input(in...), label),
				b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400"), help))
		}
	}
	params := make([]string, 0, 8)
	for _, n := range accounting.OQLParamNames(f.Situation) {
		params = append(params, ":"+n)
	}
	oqlBlock := block("fm-oql",
		acctField(t("accounting.formula.field.queries"), "queries", "textarea", joinQueries(f.Queries),
			mi.Attr("rows", "8"), mi.Attr("spellcheck", "false"), acctClass("font-mono text-sm")),
		hint("accounting.formula.field.queries_help"),
		func(b *mi.Builder) mi.Node {
			return b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400"), t("accounting.formula.field.queries_params", strings.Join(params, " ")))
		},
		hint("accounting.formula.field.queries_example"))
	fields := []mi.H{
		func(b *mi.Builder) mi.Node {
			return b.Div(mi.Class("grid grid-cols-1 md:grid-cols-3 gap-4"),
				acctField(t("accounting.formula.field.name"), "name", "text", f.Name, mi.Required(), mi.Attr("maxlength", "80"))(b),
				acctField(t("accounting.formula.field.effect"), "effect", "select", string(f.Effect), effectOpts)(b),
				acctField(t("accounting.formula.field.method"), "method", "select", string(f.Method), methodOpts)(b))
		},
		hint("accounting.formula.field.effect_help"),
		block("fm-percent", acctField(t("accounting.formula.field.percent"), "percent", "text", percent, mi.Attr("inputmode", "decimal"), mi.Placeholder("12.5")),
			hint("accounting.formula.field.percent_help")),
		block("fm-fixed", acctField(t("accounting.formula.field.amount"), "amount", "text", amount, mi.Attr("inputmode", "decimal"), mi.Placeholder("0.00")),
			hint("accounting.formula.field.amount_help")),
	}
	if oqlOn || f.Method == accounting.MethodOQL {
		if !oqlOn {
			fields = append(fields, func(b *mi.Builder) mi.Node {
				return Alert(t("accounting.formula.oql_off"), "warning", false)(b)
			})
		}
		fields = append(fields, oqlBlock)
	}
	cond := []mi.H{hint("accounting.formula.conditions_help")}
	grid := []mi.H{}
	if f.Situation == accounting.SituationCommission {
		kindOpts := []acctOpt{{"", t("accounting.formula.any")}}
		for _, k := range []string{string(accounting.AEReal), accounting.SlugSeamAE, accounting.SlugSeamWeb} {
			kindOpts = append(kindOpts, acctOpt{k, q.aeKindLabel(k)})
		}
		grid = append(grid, acctField(t("accounting.formula.field.ae_kind"), "ae_kind", "select", f.AEKind, kindOpts))
	}
	grid = append(grid,
		acctField(t("accounting.formula.field.country"), "country", "text", f.Country, mi.Attr("maxlength", "2"), mi.Placeholder("PY")),
		acctField(t("accounting.formula.field.concept"), "concept", "select", string(f.Concept), conceptOpts))
	cond = append(cond, func(b *mi.Builder) mi.Node {
		args := []interface{}{mi.Class("grid grid-cols-1 sm:grid-cols-3 gap-4")}
		for _, g := range grid {
			args = append(args, g(b))
		}
		return b.Div(args...)
	})
	fields = append(fields,
		func(b *mi.Builder) mi.Node {
			args := []interface{}{mi.Class("rounded-lg border border-gray-200 dark:border-gray-700 p-4 space-y-3"),
				b.H3(mi.Class("text-sm font-semibold text-gray-900 dark:text-white"), t("accounting.formula.conditions"))}
			for _, c := range cond {
				args = append(args, c(b))
			}
			return b.Div(args...)
		},
		check("enabled", t("accounting.formula.field.enabled"), t("accounting.formula.field.enabled_help"), f.Enabled),
		check("stop", t("accounting.formula.field.stop"), t("accounting.formula.field.stop_help"), f.Stop),
		func(b *mi.Builder) mi.Node {
			return b.Div(mi.Class("flex gap-2"), acctSubmit(t("accounting.save"))(b), acctLink(formulasPath, t("accounting.cancel"))(b))
		},
		func(b *mi.Builder) mi.Node { return b.Script(mi.Raw(formulaFormScript)) })
	title := t("accounting.formula.new_in", q.situationLabel(f.Situation))
	if f.ID != 0 {
		title = t("accounting.formula.edit_in", q.situationLabel(f.Situation))
	}
	// Deleting lives on the rule's own page, away from the row of everyday
	// actions, and asks first.
	var deleteRule mi.H
	if f.ID != 0 {
		deleteRule = func(b *mi.Builder) mi.Node {
			return b.Div(mi.Class("pt-4 border-t border-gray-200 dark:border-gray-700"),
				acctConfirmForm(acctConfirm{Message: t("accounting.formula.delete_confirm", f.Name), Title: t("accounting.formula.delete"),
					OK: t("accounting.formula.delete"), Danger: true},
					formulasPath+"/"+itoa(f.ID)+"/delete", q.csrf(), false, "", acctBtnDangerSubmit(t("accounting.formula.delete")))(b))
		}
	}
	var alert mi.H
	if errKey != "" {
		alert = Alert(t(errKey), "error", false)
	}
	q.render(title, formulasPath, acctPage(
		func(b *mi.Builder) mi.Node {
			return b.Div(mi.Class("flex items-center justify-between"),
				b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), title),
				acctLink(formulasPath, t("accounting.back"))(b))
		}, alert,
		acctPanel("", acctBody(acctForm(action, q.csrf(), false, "space-y-4", append([]mi.H{func(b *mi.Builder) mi.Node {
			return b.Input(mi.Type("hidden"), mi.Name("situation"), mi.Value(string(f.Situation)))
		}}, fields...)...), deleteRule))))
}

// formulaFormScript shows only the block of the chosen method. Without it every
// block shows, and only the chosen method's is used.
const formulaFormScript = `(function(){
var m=document.getElementById('f_method');if(!m)return;
function show(){var v=m.value;['percent','fixed','oql'].forEach(function(k){var e=document.getElementById('fm-'+k);if(e)e.style.display=(k===v)?'':'none';});}
m.addEventListener('change',show);show();})();`

func (h *UIHandler) AcctFormulaSave(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	if err := r.ParseForm(); err != nil {
		q.redirectErr(formulasPath, err)
		return
	}
	var id int64
	if v := r.PathValue("id"); v != "" {
		var ok bool
		if id, ok = pathID(r); !ok {
			q.notFound()
			return
		}
	}
	sit := accounting.Situation(r.FormValue("situation"))
	if id != 0 {
		old, err := q.svc.GetFormula(q.ctx(), id)
		if err != nil {
			q.notFound()
			return
		}
		sit = old.Situation
	}
	f, perr := readFormula(r, sit)
	f.ID = id
	if perr == nil {
		_, perr = q.svc.SaveFormula(q.ctx(), f, q.user.ID)
	}
	if perr != nil {
		var p *accounting.FormulaProblem
		if errors.As(perr, &p) || errors.Is(perr, accounting.ErrOQLDisabled) {
			if !sit.Valid() {
				q.notFound()
				return
			}
			q.formulaForm(f, acctErrKey(perr)) // keep what was typed
			return
		}
		q.redirectErr(formulasPath, perr)
		return
	}
	q.acctAudit("formula.save", f.Name)
	q.redirectMsg(formulasPath, "ok", "formula_saved")
}

func (h *UIHandler) AcctFormulaEnable(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	id, ok := pathID(r)
	if !ok {
		q.notFound()
		return
	}
	if err := r.ParseForm(); err != nil {
		q.redirectErr(formulasPath, err)
		return
	}
	on := r.FormValue("enabled") == "1"
	if err := q.svc.SetFormulaEnabled(q.ctx(), id, on, q.user.ID); err != nil {
		q.redirectErr(formulasPath, err)
		return
	}
	key := "formula_disabled"
	if on {
		key = "formula_enabled"
	}
	q.acctAudit("formula.enable", itoa(id)+" "+strconv.FormatBool(on))
	q.redirectMsg(formulasPath, "ok", key)
}

func (h *UIHandler) AcctFormulaMove(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	id, ok := pathID(r)
	if !ok {
		q.notFound()
		return
	}
	if err := r.ParseForm(); err != nil {
		q.redirectErr(formulasPath, err)
		return
	}
	dir := 1
	if r.FormValue("dir") == "up" {
		dir = -1
	}
	if err := q.svc.MoveFormula(q.ctx(), id, dir); err != nil {
		q.redirectErr(formulasPath, err)
		return
	}
	q.acctAudit("formula.move", itoa(id)+" "+r.FormValue("dir"))
	q.redirectMsg(formulasPath, "ok", "formula_moved")
}

func (h *UIHandler) AcctFormulaDelete(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	id, ok := pathID(r)
	if !ok {
		q.notFound()
		return
	}
	if err := q.svc.DeleteFormula(q.ctx(), id); err != nil {
		q.redirectErr(formulasPath, err)
		return
	}
	q.acctAudit("formula.delete", itoa(id))
	q.redirectMsg(formulasPath, "ok", "formula_deleted")
}
