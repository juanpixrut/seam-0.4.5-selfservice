package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	mi "github.com/ha1tch/minty"

	fe "github.com/ha1tch/seam-ui/internal/formengine"
	demoschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/demo"
)

// =============================================================================
// DEMO MODULE (/demo, opt-in via EnableDemo)
// =============================================================================
// Shows developers new components and fixes on real pages rendered by the
// real form engine, each with a short text saying what is being demonstrated.
// The demonstrations are grouped into pages (demoGroups), one tab each:
//
//   - /demo           the overview: what each page holds, with links
//   - /demo/inputs    the widgets a person types or draws into, and the
//                     accent colours
//   - /demo/pickers   choosing a record: the Selector dialog and any-picker
//                     (T-96, T-97), selectors over several entities, the
//                     query picker
//   - /demo/rows      repeatable rows: the answer to the Work Orders
//                     developers' report (with the records it can save),
//                     layouts, buttons, reordering, widgets inside rows
//
// Each form posts to POST /demo/echo, which shows the values the server
// received and saves nothing.

// DemoGroup is one page of the Demo module: related demonstrations, one per
// tab. Its texts are under demo.group.<ID>.title and .intro.
type DemoGroup struct {
	ID   string   // the page is /demo/<ID>
	Icon string   // Material icon, for the menu and the overview
	Tabs []string // the demonstrations, by schema name, in tab order
}

// demoGroups are the pages, in menu order. Every demonstration is in exactly
// one of them (TestDemoGroups_HoldEveryDemonstrationOnce).
var demoGroups = []DemoGroup{
	{ID: "inputs", Icon: "edit_note", Tabs: []string{"kitchen_inputs", "kitchen_choices", "kitchen_field", "colors"}},
	{ID: "pickers", Icon: "manage_search", Tabs: []string{"components", "selectors", "query_picker"}},
	{ID: "rows", Icon: "table_rows", Tabs: []string{"repeatable", "layouts", "buttons", "reorder", "kitchen_rows"}},
}

// DemoGroups returns the pages of the Demo module, for the module's menu.
func DemoGroups() []DemoGroup { return demoGroups }

func demoGroupByID(id string) (DemoGroup, bool) {
	for _, g := range demoGroups {
		if g.ID == id {
			return g, true
		}
	}
	return DemoGroup{}, false
}

// demoGroupOfTab finds the page a demonstration is on.
func demoGroupOfTab(schema string) (DemoGroup, bool) {
	for _, g := range demoGroups {
		for _, name := range g.Tabs {
			if name == schema {
				return g, true
			}
		}
	}
	return DemoGroup{}, false
}

// demoTabURL is the address of one demonstration: its page, opened at its tab.
func demoTabURL(g DemoGroup, schema string) string {
	return "/demo/" + g.ID + "?tab=" + schema
}

func (g DemoGroup) has(schema string) bool {
	for _, name := range g.Tabs {
		if name == schema {
			return true
		}
	}
	return false
}

// DemoPageData holds what DemoPage renders: one page of the module.
type DemoPageData struct {
	Group      DemoGroup
	Components mi.H   // form content for the "components" schema, when on this page
	Repeatable mi.H   // form content for the "repeatable" schema, when on this page
	BugSchema  string // the schema as reported, shown as text
	GoodSchema string // the corrected schema, shown as text
	Saved      []DemoTemplate
	Tabs       []DemoSchemaTab // the single-schema tabs of this page
	ActiveTab  string          // the tab to open, when it is one of this page's (?tab=)
	CSRFToken  string
}

// DemoIndex handles GET /demo: the overview, which lists the pages and what
// each one demonstrates. An address from before the pages existed
// (/demo?tab=repeatable) goes to that demonstration's page.
func (h *UIHandler) DemoIndex(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	if tab := r.URL.Query().Get("tab"); tab != "" {
		if g, ok := demoGroupOfTab(tab); ok {
			http.Redirect(w, r, demoTabURL(g, tab), http.StatusSeeOther)
			return
		}
	}
	h.page.Render(w, r, t("demo.title"), "/demo", DemoOverviewPage(t))
}

// DemoGroupPage handles GET /demo/{group}: one page of demonstrations.
func (h *UIHandler) DemoGroupPage(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	group, ok := demoGroupByID(r.PathValue("group"))
	if !ok {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("demo.group.not_found"))
		return
	}
	ctx := r.Context()
	csrfToken := h.GetCSRFToken(w, r)

	eng := &fe.FormEngine{Sources: demoSourceResolver(ctx, h), Translator: h.formEngineTranslator()}
	render := func(schema string) mi.H {
		rc := fe.RenderContext{Locale: h.currentLocale(), CSRFToken: csrfToken, Module: "demo", Form: schema}
		return eng.Render(ctx, demoschemas.Schema(schema), demoPresetValues[schema], nil, rc)
	}
	data := DemoPageData{Group: group, CSRFToken: csrfToken}
	if tab := r.URL.Query().Get("tab"); group.has(tab) {
		data.ActiveTab = tab
	}
	if group.has("components") {
		data.Components = render("components")
	}
	if group.has("repeatable") {
		data.Repeatable = render("repeatable")
		data.BugSchema = string(demoschemas.Schema("repeatable_bug"))
		data.GoodSchema = string(demoschemas.Schema("repeatable"))
		if tenantID, _, ok := demoTenant(ctx); ok {
			data.Saved = h.demoSavedTemplates(ctx, tenantID)
		}
	}
	data.Tabs = demoSchemaTabs(group, render)
	h.page.Render(w, r, t("demo.group."+group.ID+".title"), "/demo/"+group.ID, DemoPage(data, t))
}

// DemoEcho handles POST /demo/echo: it shows the values the server received
// (keys starting with "_", such as the CSRF token, are omitted) and saves
// nothing.
func (h *UIHandler) DemoEcho(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, h.errText(r, "error.invalid_form_data", "bad form"), http.StatusBadRequest)
		return
	}
	keys := make([]string, 0, len(r.PostForm))
	for k := range r.PostForm {
		if strings.HasPrefix(k, "_") {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&sb, "%s = %q\n", k, r.PostForm[k])
	}
	if sb.Len() == 0 {
		sb.WriteString(t("demo.echo.empty"))
	}
	// Indexed repeatables rebuilt into their structure (blocks -> lines).
	structured := ""
	schemaName := r.PostForm.Get("_demo_schema")
	if !demoStructuredSchemas[schemaName] {
		schemaName = "repeatable"
	}
	if form, err := fe.ParseSchema(demoschemas.Schema(schemaName)); err == nil {
		result := map[string]interface{}{}
		for k, v := range fe.ParseIndexedForm(r.PostForm, form.Fields) {
			result[k] = v
		}
		// the server-side half of minItems / maxItems (the form only keeps its buttons honest)
		engine := &fe.FormEngine{Translator: h.formEngineTranslator()}
		if errs := fe.ValidatePostedBounds(engine, fe.RenderContext{Locale: h.currentLocale()}, form, r.PostForm); len(errs) > 0 {
			result["_validation"] = errs
		}
		// query-picker values: each must be a row its field's query returns
		// (the Query picker tab). Top-level fields are read from the form;
		// the rows of a repeatable are already in result.
		values := map[string]interface{}{}
		for k, v := range result {
			values[k] = v
		}
		for _, f := range form.Fields {
			if len(f.Items) == 0 && r.PostForm.Has(f.Name) {
				values[f.Name] = r.PostForm.Get(f.Name)
			}
		}
		if errs := h.QueryPickerErrors(r.Context(), form, values, t); len(errs) > 0 {
			result["_query_picker"] = errs
		}
		if len(result) > 0 {
			if out, err := json.MarshalIndent(result, "", "  "); err == nil {
				structured = string(out)
			}
		}
	}
	// back to the demonstration the form was on
	back := "/demo"
	if g, ok := demoGroupOfTab(r.PostForm.Get("_demo_schema")); ok {
		back = demoTabURL(g, r.PostForm.Get("_demo_schema"))
	}
	h.page.Render(w, r, t("demo.echo.title"), back, DemoEchoPage(sb.String(), structured, back, t))
}

const (
	demoTabActive   = "border-indigo-600 text-indigo-600 dark:text-indigo-400 dark:border-indigo-400"
	demoTabInactive = "border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300"
	demoTabBase     = "demo-tab-btn px-4 py-3 text-sm font-medium border-b-2 whitespace-nowrap cursor-pointer transition-colors "
	demoText        = "text-sm text-gray-600 dark:text-gray-300"
	demoGridStyle   = "display:grid;grid-template-columns:repeat(auto-fit,minmax(320px,1fr));gap:16px"
)

func demoTabButton(b *mi.Builder, id, label string, active bool) mi.Node {
	cls := demoTabBase + demoTabInactive
	if active {
		cls = demoTabBase + demoTabActive
	}
	return b.Button(mi.Type("button"), mi.Class(cls), mi.Attr("data-tab", id), label)
}

func demoCode(b *mi.Builder, text string) mi.Node {
	return b.Div(mi.Class("fe-code"), text)
}

func demoHeading(b *mi.Builder, text string) mi.Node {
	return b.H3(mi.Class("text-sm font-semibold text-gray-900 dark:text-white mb-2"), text)
}

// demoForm wraps rendered form content in a form that posts to /demo/echo.
// schema names the demo schema the form was rendered from: a hidden field
// carries it to the echo page, which rebuilds the posted structure with it.
// saveAction, when not empty, adds a second submit button that posts the same
// form to that URL instead (the "Save as demo record" action).
func demoForm(content mi.H, schema, saveAction string, t func(key string, args ...any) string) mi.H {
	return func(b *mi.Builder) mi.Node {
		buttons := []interface{}{
			mi.Class("flex justify-end gap-3 pt-2"),
			b.Button(mi.Type("submit"), mi.Class("px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg font-medium hover:bg-gray-50 dark:hover:bg-gray-700"),
				t("demo.submit"),
			),
		}
		if saveAction != "" {
			buttons = append(buttons, b.Button(mi.Type("submit"), mi.Attr("formaction", saveAction),
				mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 font-medium"),
				t("demo.save"),
			))
		}
		return b.Form(mi.Class("space-y-4"), mi.Method("post"), mi.Action("/demo/echo"),
			content(b),
			b.Input(mi.Type("hidden"), mi.Name("_demo_schema"), mi.Value(schema)),
			b.Div(buttons...),
		)
	}
}

// demoPostButton is a one-button POST form (with the CSRF token) for the
// delete and reset actions of the saved-records list.
func demoPostButton(b *mi.Builder, action, csrfToken, label string) mi.Node {
	return b.Form(mi.Method("post"), mi.Action(action),
		b.Input(mi.Type("hidden"), mi.Name("_csrf"), mi.Value(csrfToken)),
		b.Button(mi.Type("submit"), mi.Class("text-sm text-red-600 dark:text-red-400 hover:underline"), label),
	)
}

// demoSavedSection lists the saved demo records of the caller's tenant.
func demoSavedSection(b *mi.Builder, d DemoPageData, t func(key string, args ...any) string) mi.Node {
	items := []interface{}{
		mi.Class("space-y-4 pt-2"),
		demoHeading(b, t("demo.saved.heading")),
		b.P(mi.Class(demoText), t("demo.saved.note")),
	}
	if len(d.Saved) == 0 {
		items = append(items, b.P(mi.Class(demoText), t("demo.saved.empty")))
	}
	for _, s := range d.Saved {
		id := strconv.FormatInt(s.ID, 10)
		items = append(items, b.Div(mi.Class("space-y-2"),
			b.Div(mi.Class("flex items-center justify-between gap-3"),
				b.Span(mi.Class("text-sm font-medium text-gray-900 dark:text-white"), "#"+id+" "+s.Name),
				b.Span(mi.Class(demoText), s.CreatedAt),
				demoPostButton(b, "/demo/templates/"+id+"/delete", d.CSRFToken, t("demo.saved.delete")),
			),
			demoCode(b, s.Data),
		))
	}
	if len(d.Saved) > 0 {
		items = append(items, demoPostButton(b, "/demo/templates/reset", d.CSRFToken, t("demo.saved.reset")))
	}
	return b.Div(items...)
}

// demoOpenTabScript opens a tab other than the first when the page is asked
// for one (?tab=). active has been checked against the page's own tabs, so
// only a known schema name ever reaches the script.
func demoOpenTabScript(g DemoGroup, active string) string {
	if !g.has(active) || active == g.Tabs[0] {
		return ""
	}
	return `var tb=document.querySelector('.demo-tab-btn[data-tab="demo-tab-` + active + `"]');if(tb){tb.click();}`
}

// DemoPage renders one page of the Demo module: its demonstrations, one per
// tab, each with explanatory texts and a working form.
func DemoPage(data DemoPageData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	return func(b *mi.Builder) mi.Node {
		components := func() mi.Node {
			return b.Div(mi.ID("demo-tab-components"), mi.Class("demo-tab-panel space-y-4 pt-4"),
				demoHeading(b, t("demo.components.heading")),
				b.P(mi.Class(demoText), t("demo.components.intro")),
				demoForm(data.Components, "components", "", t)(b),
			)
		}
		repeatable := func() mi.Node {
			return b.Div(mi.ID("demo-tab-repeatable"), mi.Class("demo-tab-panel space-y-4 pt-4"),
				demoHeading(b, t("demo.repeatable.heading")),
				b.P(mi.Class(demoText), t("demo.repeatable.intro")),
				b.P(mi.Class(demoText), t("demo.repeatable.naming")),
				b.P(mi.Class(demoText), t("demo.repeatable.rows")),
				b.Div(mi.Attr("style", demoGridStyle),
					b.Div(
						demoHeading(b, t("demo.repeatable.before")),
						demoCode(b, data.BugSchema),
					),
					b.Div(
						demoHeading(b, t("demo.repeatable.after")),
						demoForm(data.Repeatable, "repeatable", "/demo/templates", t)(b),
					),
				),
				demoSavedSection(b, data, t),
				b.Div(
					demoHeading(b, t("demo.repeatable.schema")),
					demoCode(b, data.GoodSchema),
				),
			)
		}
		single := map[string]DemoSchemaTab{}
		for _, tab := range data.Tabs {
			single[tab.Schema] = tab
		}
		// the tab bar, then the panels in the same order
		body := []interface{}{demoTabBar(b, data.Group, t)}
		for _, name := range data.Group.Tabs {
			switch name {
			case "components":
				body = append(body, components())
			case "repeatable":
				body = append(body, repeatable())
			default:
				body = append(body, demoSchemaPanel(b, single[name], t))
			}
		}
		// app.js is deferred: on a full page load SeamTabs does not exist yet
		// when this inline script runs, so wait for DOMContentLoaded then.
		body = append(body, b.Script(mi.Raw(`(function(){function go(){SeamTabs.wire('.demo-tab-panel','.demo-tab-btn');`+demoOpenTabScript(data.Group, data.ActiveTab)+`}if(window.SeamTabs){go();}else{document.addEventListener('DOMContentLoaded',go);}})();`)))
		// data-fe-fx: repeatable rows on this page grow in and shrink out (SeamRowFx).
		return b.Div(mi.Class("space-y-6"), mi.Attr("style", "max-width:64rem"), mi.Attr("data-fe-fx", "1"),
			b.Div(mi.Class("space-y-1"),
				b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), t("demo.group."+data.Group.ID+".title")),
				b.P(mi.Class(demoText), t("demo.group."+data.Group.ID+".intro")),
			),
			Card("", func(b *mi.Builder) mi.Node { return b.Div(body...) })(b),
		)
	}
}

// DemoOverviewPage renders /demo: one card per page of the module, saying
// what it demonstrates, with a link to each demonstration.
func DemoOverviewPage(t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	return func(b *mi.Builder) mi.Node {
		cards := []interface{}{mi.Attr("style", demoGridStyle)}
		for _, g := range demoGroups {
			g := g
			links := []interface{}{mi.Class("space-y-1 pt-2")}
			for _, name := range g.Tabs {
				links = append(links, b.Li(b.A(mi.Href(demoTabURL(g, name)), mi.Class("text-sm text-indigo-600 dark:text-indigo-400 hover:underline"),
					t("demo.tab."+name))))
			}
			cards = append(cards, Card("", func(b *mi.Builder) mi.Node {
				return b.Div(mi.Class("space-y-2"),
					b.A(mi.Href("/demo/"+g.ID), mi.Class("flex items-center gap-2 text-gray-900 dark:text-white hover:underline"),
						b.Span(mi.Class("material-icons text-xl"), g.Icon),
						b.Span(mi.Class("text-base font-semibold"), t("demo.group."+g.ID+".title")),
					),
					b.P(mi.Class(demoText), t("demo.group."+g.ID+".intro")),
					b.Ul(links...),
				)
			})(b))
		}
		return b.Div(mi.Class("space-y-6"), mi.Attr("style", "max-width:64rem"),
			b.Div(mi.Class("space-y-1"),
				b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), t("demo.title")),
				b.P(mi.Class(demoText), t("demo.overview.intro")),
			),
			b.Div(cards...),
		)
	}
}

// demoStructured renders the rebuilt structure, or nothing when there is none.
func demoStructured(b *mi.Builder, structured string, t func(key string, args ...any) string) mi.Node {
	if structured == "" {
		return b.Div()
	}
	return b.Div(demoHeading(b, t("demo.echo.structured")), demoCode(b, structured))
}

// DemoEchoPage renders the values POST /demo/echo received.
func DemoEchoPage(values, structured, back string, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	return func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("space-y-6"), mi.Attr("style", "max-width:64rem"),
			b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), t("demo.echo.title")),
			Card("", func(b *mi.Builder) mi.Node {
				return b.Div(mi.Class("space-y-4"),
					b.P(mi.Class(demoText), t("demo.echo.intro")),
					demoCode(b, values),
					demoStructured(b, structured, t),
					b.A(mi.Href(back), mi.Class("text-sm text-indigo-600 dark:text-indigo-400 hover:underline"), t("demo.echo.back")),
				)
			})(b),
		)
	}
}

// DemoSchemaTab is one of the tabs that show a single schema: the rendered
// form and the schema as text, with texts under the keys demo.tab.<schema>,
// demo.<schema>.heading, demo.<schema>.intro and demo.<schema>.notes.
type DemoSchemaTab struct {
	ID         string // panel id, "demo-tab-<schema>"
	Schema     string // schema name in the demo schema package
	Form       mi.H   // the form rendered by the real engine
	SchemaText string // the schema, shown as text
}

// demoSourceResolver supplies the options of x-seam-source selects (the
// Kitchen tabs' foreign-key select). A session without a tenant gets an
// empty static resolver, so nothing is queried for it, the same rule the
// saved-records list follows. A variable so a test can swap in a static
// resolver instead of a live xolu.
var demoSourceResolver = func(ctx context.Context, h *UIHandler) fe.SourceResolver {
	if _, _, ok := demoTenant(ctx); !ok {
		return fe.StaticSourceResolver{}
	}
	return &AssetSourceResolver{Client: h.xoluClient}
}

// demoPresetValues are the stored values some demo forms open with, by
// schema name: a GPS field already holding a point and barcodes whose
// preview is drawn on load. Fields not listed start empty.
var demoPresetValues = map[string]map[string]interface{}{
	"kitchen_field": {
		"depot_location": "-34.9011,-56.1645",
		"ean13_code":     "5901234123457",
		"ean8_code":      "96385074",
		"upc_code":       "123456789999",
	},
	// a stored value, so the Model picker shows its label when the form opens
	"query_picker": {
		"model_id": 1,
	},
}

// demoStructuredSchemas are the schemas whose posted values the echo page
// rebuilds into a structure; the hidden _demo_schema field names one. Any
// other value falls back to "repeatable".
var demoStructuredSchemas = map[string]bool{
	"repeatable":      true,
	"layouts":         true,
	"buttons":         true,
	"colors":          true,
	"selectors":       true,
	"reorder":         true,
	"kitchen_inputs":  true,
	"kitchen_choices": true,
	"kitchen_field":   true,
	"kitchen_rows":    true,
	"query_picker":    true,
}

// demoSchemaTabs renders the single-schema tabs of a page with the given
// renderer. "components" and "repeatable" have panels of their own (DemoPage).
func demoSchemaTabs(g DemoGroup, render func(schema string) mi.H) []DemoSchemaTab {
	tabs := make([]DemoSchemaTab, 0, len(g.Tabs))
	for _, name := range g.Tabs {
		if name == "components" || name == "repeatable" {
			continue
		}
		tabs = append(tabs, DemoSchemaTab{
			ID:         "demo-tab-" + name,
			Schema:     name,
			Form:       render(name),
			SchemaText: string(demoschemas.Schema(name)),
		})
	}
	return tabs
}

// demoTabBar is the row of tab buttons of a page; the first is the open one.
// It scrolls sideways when the buttons do not fit.
func demoTabBar(b *mi.Builder, g DemoGroup, t func(key string, args ...any) string) mi.Node {
	items := []interface{}{mi.Class("flex gap-2 border-b border-gray-200 dark:border-gray-700 overflow-x-auto")}
	for i, name := range g.Tabs {
		items = append(items, demoTabButton(b, "demo-tab-"+name, t("demo.tab."+name), i == 0))
	}
	return b.Div(items...)
}

// demoSchemaPanel renders the panel of one single-schema tab.
func demoSchemaPanel(b *mi.Builder, tab DemoSchemaTab, t func(key string, args ...any) string) mi.Node {
	return b.Div(mi.ID(tab.ID), mi.Class("demo-tab-panel space-y-4 pt-4"),
		demoHeading(b, t("demo."+tab.Schema+".heading")),
		b.P(mi.Class(demoText), t("demo."+tab.Schema+".intro")),
		b.P(mi.Class(demoText), t("demo."+tab.Schema+".notes")),
		demoForm(tab.Form, tab.Schema, "", t)(b),
		b.Div(demoHeading(b, t("demo.schema.heading")), demoCode(b, tab.SchemaText)),
	)
}
