package ui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	mi "github.com/ha1tch/minty"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/features"
	fe "github.com/ha1tch/seam-ui/internal/formengine"
	"github.com/ha1tch/seam-ui/internal/formengine/designer"
	"github.com/ha1tch/seam-ui/internal/overrides"
)

// The form designer (campaign DEV-21), first version: the field level.
//
// It is one stateless form (designerFormID; its controls are spread over the
// page's panels and belong to it by name, so the live preview is not inside
// it). The document being built travels in a hidden field
// named json (the name the save route reads), every action is a button named act
// ("add", "up:email", "remove:email", "select:email", "apply:email", "rename:email",
// "preview"), and the server applies the action to the document with
// formengine/designer, re-parses the result with the real engine and refuses any
// change that leaves a form the engine would not accept. Nothing is stored until
// Save, which is the existing save route with its scopes, note, history and
// validation. The block tree (tabs, rows and sections of x-seam-form) is the
// next step; the designer leaves x-seam-form untouched and will not remove or
// rename a field it names.

func designerURL(ref developerSchemaRef, scope string) string {
	u := "/developer/designer?schema=" + url.QueryEscape(ref.String())
	if scope != "" {
		u += "&scope=" + url.QueryEscape(scope)
	}
	return u
}

type designerView struct {
	Ref       developerSchemaRef
	Chosen    bool
	Forms     []developerSchemaRef
	Scope     string
	Scopes    []string
	Doc       string
	Fields    []designer.FieldInfo
	Layout    []designer.LayoutTab // the block tree; nil while the form is flat
	Selected  string
	Values    map[string]string // the selected field's properties, as the inspector shows them
	Widget    string
	LabelKey  string // the translation key that supplies the selected field's label, "" for none
	Required  bool
	Preview   mi.H
	Previewed bool
	PreviewIn string   // the language the preview is rendered in
	Locales   []string // the languages it can be rendered in
	Note      string
	Problem   string
}

// designerCurrentText is the text the designer starts from: what the selected
// scope holds for the form, or, when it holds nothing, what the tenant sees now.
func (h *UIHandler) designerCurrentText(ctx context.Context, tenant int64, ref developerSchemaRef, base []byte, scope string) string {
	rt := h.config.DevRuntime
	text := developerPretty(base)
	if rt == nil {
		return text
	}
	if rt.Resolver != nil {
		text = developerPretty(rt.Resolver.Resolve(tenant, ref.Module, ref.Name, base))
	}
	switch scope {
	case scopeTenant:
		if rt.Overrides != nil && tenant > 0 {
			if cur, ok, _ := rt.Overrides.Current(ctx, tenant, ref.Module, ref.Name); ok {
				text = developerPretty(cur.Doc)
			}
		}
	case scopeGlobal:
		text = developerPretty(base)
		if rt.Overrides != nil {
			if cur, ok, _ := rt.Overrides.Current(ctx, overrides.GlobalTenant, ref.Module, ref.Name); ok {
				text = developerPretty(cur.Doc)
			}
		}
	case scopeFile:
		if rt.Schemas != nil {
			if doc, ok, _ := rt.Schemas.Read(ref.Module, ref.Name); ok {
				text = developerPretty(doc)
			}
		}
	}
	return text
}

// DeveloperDesigner handles GET and POST /developer/designer.
func (h *UIHandler) DeveloperDesigner(w http.ResponseWriter, r *http.Request) {
	if !h.config.flagOn(features.DeveloperFormDesigner) {
		http.NotFound(w, r)
		return
	}
	h.applyDeveloperPrefs(w, r, false)
	r.Body = http.MaxBytesReader(w, r.Body, designer.MaxDocBytes+(64<<10))
	if err := r.ParseForm(); err != nil {
		developerRedirect(w, r, "/developer/designer", "error", "bad_request")
		return
	}
	ctx := r.Context()
	t := h.translateFunc(r)
	data := h.developerData(w, r, t)
	claims, _ := auth.ClaimsFromContext(ctx)
	rt := h.config.DevRuntime
	var v designerView
	ref, base, custom, ok := h.developerForm(ctx, claims.TenantID, r.FormValue("schema"))
	if !ok {
		v.Forms = append(developerEmbeddedSchemas(), h.developerCustomForms(claims.TenantID)...)
		h.page.Render(w, r, t("developer.nav.designer"), "/developer/designer", DeveloperDesignerPage(data, v, t))
		return
	}
	v.Ref, v.Chosen = ref, true
	v.Scopes = h.scopesWithGlobal(ctx, developerScopes(rt, rt != nil && rt.Schemas != nil))
	v.Scope = developerScope(r.FormValue("scope"), v.Scopes)
	v.Note = r.FormValue("note")
	text := r.FormValue("json")
	if r.Method != http.MethodPost || strings.TrimSpace(text) == "" {
		text = h.designerCurrentText(ctx, claims.TenantID, ref, base, v.Scope)
	}
	doc, err := designer.Parse([]byte(text))
	if err != nil {
		v.Problem = err.Error()
		doc, _ = designer.Parse([]byte(h.designerCurrentText(ctx, claims.TenantID, ref, base, v.Scope)))
	}
	v.Selected = r.FormValue("selected")
	act := r.FormValue("act")
	if act == "" && r.Method == http.MethodPost && r.FormValue("drop_field") != "" {
		act = designerDrop // a field was dragged to a section: the form was posted by the drop
	}
	if r.Method == http.MethodPost && act != "" && act != "preview" {
		before := doc.Bytes()
		selected, aerr := applyDesignerAct(doc, act, r)
		if aerr == nil {
			// the engine has the last word: what it could not render is refused
			aerr = developerFormCheck(ref, base, custom, doc.Bytes())
		}
		if aerr != nil {
			v.Problem = aerr.Error()
			doc, _ = designer.Parse(before)
		} else {
			v.Selected = selected
		}
	}
	if !doc.Has(v.Selected) {
		v.Selected = ""
	}
	v.Doc = string(doc.Bytes())
	v.Fields = doc.Fields()
	v.Layout, _ = doc.Layout()
	if v.Selected != "" {
		raw, _ := doc.Field(v.Selected)
		v.Widget = designer.WidgetOf(raw)
		var props map[string]json.RawMessage
		_ = json.Unmarshal(raw, &props)
		v.Values = map[string]string{}
		if p, ok := props["title"]; ok {
			var s string
			_ = json.Unmarshal(p, &s)
			v.Values["title"] = s
		}
		for _, p := range designer.PropsFor(v.Widget) {
			if val, ok := props[p.Key]; ok {
				v.Values[p.Key] = designer.TextFromValue(p, val)
			}
		}
		for _, f := range v.Fields {
			if f.Name == v.Selected {
				v.Required = f.Required
			}
		}
		for _, lk := range h.developerLabelKeys(claims.TenantID, ref, doc.Bytes()) {
			if lk.Field == v.Selected {
				v.LabelKey = lk.Key
			}
		}
	}
	if h.config.flagOn(features.DesignerLivePreview) || act == "preview" {
		// in the application's language, or the one chosen on the panel: the
		// choice travels with every action, since its select belongs to the
		// designer's form
		v.PreviewIn = h.currentLocale()
		if h.translator != nil {
			v.Locales = h.translator.Locales()
			sort.Strings(v.Locales)
			if in := r.FormValue("preview_locale"); in != "" && h.translator.HasLocale(in) {
				v.PreviewIn = in
			}
		}
		engine := &fe.FormEngine{Translator: h.formEngineTranslator()}
		v.Preview = engine.Render(withoutTrace(ctx), []byte(v.Doc), nil, nil,
			fe.RenderContext{Locale: v.PreviewIn, CSRFToken: data.CSRFToken, Module: "preview", Form: ref.Name})
		v.Previewed = true
	}
	if v.Problem != "" {
		data.Error = t("developer.designer.problem", v.Problem)
	}
	h.page.Render(w, r, t("developer.nav.designer"), "/developer/designer", DeveloperDesignerPage(data, v, t))
}

// applyDesignerAct applies one action to the document and returns the field to
// leave selected.
func applyDesignerAct(doc *designer.Doc, act string, r *http.Request) (selected string, err error) {
	verb, arg, _ := strings.Cut(act, ":")
	if ok, err := applyLayoutAct(doc, verb, arg, r); ok {
		return r.FormValue("selected"), err
	}
	switch verb {
	case "add":
		name := strings.TrimSpace(r.FormValue("new_name"))
		return name, doc.AddField(name, r.FormValue("new_widget"))
	case "remove":
		return "", doc.RemoveField(arg)
	case "dup":
		to, err := doc.DuplicateField(arg)
		if err != nil {
			return arg, err
		}
		return to, nil
	case "up":
		return arg, doc.MoveField(arg, -1)
	case "down":
		return arg, doc.MoveField(arg, 1)
	case "select":
		return arg, nil
	case "rename":
		to := strings.TrimSpace(r.FormValue("rename_to"))
		return to, doc.Rename(arg, to)
	case "apply":
		return arg, applyInspector(doc, arg, r)
	}
	return r.FormValue("selected"), nil
}

// applyInspector turns what the inspector posted into property changes.
func applyInspector(doc *designer.Doc, field string, r *http.Request) error {
	raw, ok := doc.Field(field)
	if !ok {
		return designer.ErrField
	}
	set := map[string]json.RawMessage{}
	var unset []string
	for _, p := range append([]designer.Prop{{Key: "title", Title: "Label", Kind: designer.KindString}}, designer.PropsFor(designer.WidgetOf(raw))...) {
		val, clear, err := designer.ValueFromText(p, r.FormValue("prop:"+p.Key))
		if err != nil {
			return err
		}
		if clear {
			unset = append(unset, p.Key)
		} else {
			set[p.Key] = val
		}
	}
	if err := doc.SetProps(field, set, unset); err != nil {
		return err
	}
	return doc.SetRequired(field, r.FormValue("required") == "on")
}

func designerText(t func(key string, args ...any) string, key, fallback string) string {
	if s := t(key); s != "" && s != key {
		return s
	}
	return fallback
}

// designerFormID is the id of the designer's form. Its controls sit in several
// panels of the page and name it with their form attribute, so the live preview
// can be drawn between them without being inside it: the preview's own inputs
// are then neither posted with the document nor validated by the browser when
// a designer button is pressed.
const designerFormID = "dev-designer"

// DeveloperDesignerPage renders the designer, or the choice of a form to design.
func DeveloperDesignerPage(data DeveloperPageData, v designerView, t func(key string, args ...any) string) mi.H {
	t = developerTrans(t)
	return func(b *mi.Builder) mi.Node {
		if !v.Chosen {
			var rows []mi.H
			for i, ref := range v.Forms {
				ref := ref
				rows = append(rows, ListRow(i,
					ListCellRich(func(b *mi.Builder) mi.Node {
						return b.A(mi.Href(designerURL(ref, "")), mi.Attr("data-row-link", "1"), mi.Class("font-mono text-xs font-medium text-indigo-600 dark:text-indigo-400 hover:underline"), ref.String())
					}),
					ListCellRich(func(b *mi.Builder) mi.Node {
						return Badge(ref.Module, "")(b)
					}),
					ListCellRight(func(b *mi.Builder) mi.Node {
						return b.A(mi.Href(designerURL(ref, "")), mi.Class("inline-flex "+devIconBtn), mi.Attr("title", t("developer.overview.open")), mi.Attr("aria-label", t("developer.overview.open")+" "+ref.String()),
							b.Span(mi.Class("material-icons text-lg"), "design_services"))
					})))
			}
			return developerShell(b, t("developer.nav.designer"), t("developer.designer.intro"), data,
				b.Div(mi.Class("space-y-4"),
					b.H2(mi.Class("text-lg font-semibold text-gray-900 dark:text-white"), t("developer.designer.choose")),
					ListTable(ListTableConfig{
						Columns:   []ListColumn{{Label: t("developer.forms.col.form")}, {Label: t("developer.forms.col.module")}, {Label: t("asset.list.actions"), Right: true}},
						EmptyIcon: "design_services", EmptyTitle: t("developer.forms.empty"),
					}, rows)(b)))
		}
		own := mi.Attr("form", designerFormID)
		noValidate := mi.Attr("formnovalidate", "formnovalidate")

		// the form itself holds only what travels with every action
		form := b.Form(mi.ID(designerFormID), mi.Method("post"), mi.Action("/developer/designer"), mi.Attr("novalidate", "novalidate"),
			CSRFField(data.CSRFToken)(b),
			b.Input(mi.Type("hidden"), mi.Name("schema"), mi.Value(v.Ref.String())),
			b.Input(mi.Type("hidden"), mi.Name("scope"), mi.Value(v.Scope)),
			b.Input(mi.Type("hidden"), mi.Name("selected"), mi.Value(v.Selected)),
			b.Input(mi.Type("hidden"), mi.Name("drop_field")), b.Input(mi.Type("hidden"), mi.Name("drop_to")),
			b.Textarea(mi.Name("json"), mi.Attr("hidden", "hidden"), v.Doc))

		// where it is saved, and Save
		saveBar := []interface{}{mi.Class("flex flex-wrap items-end justify-between gap-4")}
		saveBar = append(saveBar, b.Div(developerScopeTabs(b, t, v.Scopes, v.Scope, func(s string) string { return designerURL(v.Ref, s) })...))
		saveRight := []interface{}{mi.Class("flex flex-wrap items-end gap-3")}
		if scopeStored(v.Scope) {
			saveRight = append(saveRight, b.Div(
				b.Label(mi.For("dev-designer-note"), mi.Class(devLabelClass), t("developer.forms.note")),
				b.Input(mi.Type("text"), mi.ID("dev-designer-note"), mi.Name("note"), own, mi.Class(devInputClass+" mt-1"), mi.Attr("maxlength", "200"), mi.Value(v.Note))))
		}
		saveRight = append(saveRight,
			IconButton("preview", t("developer.designer.preview"), "secondary", mi.Type("submit"), own, noValidate, mi.ID("dev-designer-preview"), mi.Name("act"), mi.Value("preview"))(b),
			IconButton("save", t("developer.designer.save"), "primary", mi.Type("submit"), own, noValidate, mi.Attr("formaction", "/developer/forms/save"))(b))
		saveBar = append(saveBar, b.Div(saveRight...))
		toolbar := b.Div(mi.Class(devCardClass+" p-4"), b.Div(saveBar...))

		// the fields
		rows := []interface{}{mi.Class("dev-field-list")}
		if len(v.Fields) == 0 {
			rows = append(rows, b.P(mi.Class(devMutedClass+" p-4"), t("developer.designer.empty")))
		}
		for _, f := range v.Fields {
			w, _ := designer.LookupWidget(f.Widget)
			cls := "dev-field-row"
			if f.Name == v.Selected {
				cls += " is-selected"
			}
			act := func(verb, icon, label, look string) mi.Node {
				return b.Button(mi.Type("submit"), own, noValidate, mi.Name("act"), mi.Value(verb+":"+f.Name), mi.Class(look), mi.Attr("title", label), mi.Attr("aria-label", label+" "+f.Name),
					b.Span(mi.Class("material-icons text-lg"), icon))
			}
			name := []interface{}{mi.Class("font-mono text-xs font-medium text-gray-900 dark:text-white"), f.Name}
			if f.Required {
				name = append(name, b.Span(mi.Class("text-red-500"), mi.Attr("title", t("developer.designer.required")), " *"))
			}
			rows = append(rows, b.Div(mi.Class(cls),
				b.Span(mi.Class("material-icons text-gray-400"), w.Icon),
				b.Div(mi.Class("min-w-0 flex-1"),
					b.Div(name...),
					b.Div(mi.Class("text-xs text-gray-500 dark:text-gray-400 truncate"), f.Title)),
				Badge(designerText(t, "designer.widget."+f.Widget+".title", w.Title), "")(b),
				b.Span(mi.Class("dev-field-actions"),
					act("select", "edit", t("developer.designer.edit"), devIconBtn),
					act("up", "arrow_upward", t("developer.designer.up"), devIconBtn),
					act("down", "arrow_downward", t("developer.designer.down"), devIconBtn),
					act("dup", "content_copy", t("developer.designer.duplicate"), devIconBtn),
					act("remove", "delete", t("developer.designer.remove"), devIconDanger))))
		}
		opts := []interface{}{mi.Name("new_widget"), own, mi.ID("dev-designer-widget"), mi.Class(devInputClass), mi.Attr("data-seam-select", "1")}
		for _, w := range designer.Widgets() {
			opts = append(opts, b.Option(mi.Value(w.Name), mi.Attr("data-icon", w.Icon), designerText(t, "designer.widget."+w.Name+".title", w.Title)))
		}
		add := b.Div(mi.Class("dev-field-add"),
			b.H3(mi.Class("text-sm font-semibold text-gray-900 dark:text-white"), t("developer.designer.palette")),
			b.Div(b.Label(mi.For("dev-designer-widget"), mi.Class(devLabelClass), t("developer.designer.widget")), b.Div(mi.Class("mt-1"), b.Select(opts...))),
			b.Div(b.Label(mi.For("dev-designer-name"), mi.Class(devLabelClass), t("developer.designer.name")),
				b.Input(mi.Type("text"), mi.ID("dev-designer-name"), mi.Name("new_name"), own, mi.Class(devInputClass+" mt-1 font-mono"), mi.Attr("maxlength", "64"), mi.Attr("placeholder", "field_name"))),
			b.Div(IconButton("add", t("developer.designer.add"), "primary", mi.Type("submit"), own, noValidate, mi.Name("act"), mi.Value("add"))(b)))
		fields := developerPanel(b, "list", t("developer.designer.fields"), "", []interface{}{Badge(strconv.Itoa(len(v.Fields)), "info")(b)}, true, b.Div(rows...), add)

		// the inspector
		var inspector mi.Node
		if v.Selected == "" {
			inspector = b.Div(mi.Class(devCardClass), EmptyState("tune", t("developer.designer.inspector_title"), t("developer.designer.select_hint"), nil)(b))
		} else {
			w, _ := designer.LookupWidget(v.Widget)
			label := designerField(b, t("developer.designer.title"), "", b.Input(mi.Type("text"), mi.Name("prop:title"), own, mi.Class(devInputClass), mi.Value(v.Values["title"])))
			if v.LabelKey != "" {
				// the engine prefers this key to the title typed here
				label = b.Div(mi.Class("dev-field"), b.Label(mi.Class(devLabelClass), t("developer.designer.title")),
					b.Input(mi.Type("text"), mi.Name("prop:title"), own, mi.Class(devInputClass), mi.Value(v.Values["title"])),
					b.Span(mi.Class("text-xs text-amber-700 dark:text-amber-400"), t("developer.designer.label_key")+" ",
						b.A(mi.Href(translationsURL("", "", showAll, 1, v.LabelKey)+"#edit"), mi.Class("font-mono underline"), v.LabelKey)))
			}
			props := []interface{}{mi.Class("dev-props"),
				label,
				designerCheck(b, t("developer.designer.required"), b.Input(mi.Type("checkbox"), mi.Name("required"), own, mi.Value("on"), mi.Class("h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-500"), designerChecked(v.Required)))}
			for _, p := range designer.PropsFor(v.Widget) {
				title := designerText(t, "designer.prop."+p.Key+".title", p.Title)
				help := designerText(t, "designer.prop."+p.Key+".help", p.Help)
				if p.Kind == designer.KindBool {
					props = append(props, designerCheck(b, title, designerControl(b, p, v.Values[p.Key])))
					continue
				}
				props = append(props, designerField(b, title, help, designerControl(b, p, v.Values[p.Key])))
			}
			ins := []interface{}{b.Div(props...),
				b.Div(IconButton("check_circle", t("developer.designer.apply"), "primary", mi.Type("submit"), own, noValidate, mi.Name("act"), mi.Value("apply:"+v.Selected))(b)),
				b.Div(mi.Class("dev-rename"),
					b.Div(mi.Class("flex-1"), b.Label(mi.For("dev-designer-rename"), mi.Class(devLabelClass), t("developer.designer.rename")),
						b.Input(mi.Type("text"), mi.ID("dev-designer-rename"), mi.Name("rename_to"), own, mi.Class(devInputClass+" mt-1 font-mono"), mi.Attr("maxlength", "64"), mi.Value(v.Selected))),
					Button(t("developer.designer.rename"), "secondary", mi.Type("submit"), own, noValidate, mi.Name("act"), mi.Value("rename:"+v.Selected))(b))}
			inspector = developerPanel(b, "tune", t("developer.designer.inspector", v.Selected), "",
				[]interface{}{Badge(designerText(t, "designer.widget."+v.Widget+".title", w.Title), "info")(b)}, false, ins...)
		}

		// the preview: outside the designer's form, in a form that never submits
		var preview mi.Node
		if v.Previewed {
			// the language: a control of the designer's form, so it is sent
			// with whatever is done next; changing it asks for the preview
			var in []interface{}
			if len(v.Locales) > 1 {
				opts := []interface{}{own, mi.Name("preview_locale"), mi.ID("dev-preview-locale"), mi.Class(devInputClass), mi.Attr("style", "width:auto"),
					mi.Attr("aria-label", t("developer.designer.preview_language")), mi.Attr("title", t("developer.designer.preview_language")),
					mi.Attr("onchange", "document.getElementById('dev-designer-preview').click()")}
				for _, l := range v.Locales {
					o := []interface{}{mi.Value(l)}
					if l == v.PreviewIn {
						o = append(o, mi.Attr("selected", "selected"))
					}
					opts = append(opts, b.Option(append(o, l)...))
				}
				in = []interface{}{b.Select(opts...)}
			}
			preview = developerPanel(b, "preview", t("developer.designer.preview"), t("developer.forms.preview_note"), in, false,
				b.Div(mi.Class("dev-preview"), b.Form(mi.Attr("onsubmit", "return false"), mi.Attr("novalidate", "novalidate"), v.Preview(b))))
		} else {
			preview = developerPanel(b, "preview", t("developer.designer.preview"), "", nil, false,
				b.P(mi.Class(devMutedClass), t("developer.designer.preview_off")))
		}

		head := developerHeader{Title: v.Ref.String(), Intro: t("developer.designer.intro"),
			BackURL: "/developer/designer", BackText: t("developer.nav.designer"),
			Actions: []interface{}{b.A(mi.Href(formsURL(v.Ref, v.Scope)), mi.Class(devLinkClass),
				b.Span(mi.Class("material-icons text-lg"), "data_object"), b.Span(t("developer.designer.json")))}}
		parts := []interface{}{form}
		parts = append(parts, toolbar, b.Div(mi.Class("dev-designer"), fields, b.Div(mi.Class("space-y-6"), inspector, preview)),
			developerLayoutPanel(b, v, t))
		return developerFrame(b, head, data, parts...)
	}
}

func designerChecked(on bool) mi.Attribute {
	if on {
		return mi.Attr("checked", "checked")
	}
	return mi.Attr("data-unchecked", "1")
}

// designerField is one property of the inspector: its label, its control and
// what the property does.
func designerField(b *mi.Builder, title, help string, control mi.Node) mi.Node {
	parts := []interface{}{mi.Class("dev-field"), b.Label(mi.Class(devLabelClass), title), control}
	if help != "" {
		parts = append(parts, b.Span(mi.Class("text-xs text-gray-500 dark:text-gray-400"), help))
	}
	return b.Div(parts...)
}

// designerCheck is a yes/no property: the box, then its label.
func designerCheck(b *mi.Builder, title string, control mi.Node) mi.Node {
	return b.Label(mi.Class("dev-check text-sm text-gray-700 dark:text-gray-300"), control, b.Span(title))
}

// designerControl draws the control for one property by its kind. Every
// control belongs to the designer's form by name (designerFormID).
func designerControl(b *mi.Builder, p designer.Prop, value string) mi.Node {
	name, own := "prop:"+p.Key, mi.Attr("form", designerFormID)
	switch p.Kind {
	case designer.KindBool:
		return b.Input(mi.Type("checkbox"), mi.Name(name), own, mi.Value("on"), mi.Class("h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-500"), designerChecked(value == "on"))
	case designer.KindEnum:
		opts := []interface{}{mi.Name(name), own, mi.Class(devInputClass), b.Option(mi.Value(""), "")}
		for _, e := range p.Enum {
			attrs := []interface{}{mi.Value(e)}
			if e == value {
				attrs = append(attrs, mi.Attr("selected", "selected"))
			}
			opts = append(opts, b.Option(append(attrs, e)...))
		}
		return b.Select(opts...)
	case designer.KindInt:
		return b.Input(mi.Type("number"), mi.Name(name), own, mi.Class(devInputClass), mi.Attr("min", "0"), mi.Value(value))
	case designer.KindList, designer.KindMap, designer.KindJSON:
		return b.Textarea(mi.Name(name), own, mi.Class(devInputClass+" font-mono"), mi.Attr("rows", "3"), value)
	}
	return b.Input(mi.Type("text"), mi.Name(name), own, mi.Class(devInputClass), mi.Value(value))
}
