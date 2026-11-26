package ui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	mi "github.com/ha1tch/minty"

	fe "github.com/ha1tch/seam-ui/internal/formengine"
)

// Trying a form. A form made with the tools is used by no page until code asks
// for it, and an override of a built-in one is only seen where its page is, with
// the records that page needs. The try page renders the definition in force for
// the tenant as a working form, with the identity its page uses when that is
// known, and shows what a submission brings to the server: the fields as the
// engine parses them (repeatables rebuilt into their rows), with the checks the
// engine makes on a post. Nothing is stored.
//
// Lists. A select's x-seam-source is resolved by the page that renders the
// form, and pages differ (developer_check.go). The try page resolves the
// application's own sources as the asset pages do, and any other
// "entity:<name>" by reading that entity for the tenant and labelling each row
// with the first of the usual naming fields it has. That fills the lists a
// form's own page fills, leaving out the rows switched off (enabled or active
// false), as those pages do; a filter that depends on the record being edited
// applies to the record named in the address ("as if editing record"): that
// record is left out of every list. A list that still comes back empty is
// named on the page.

// tryPageData is what the try page shows.
type tryPageData struct {
	DeveloperPageData
	Ref     developerSchemaRef
	Form    mi.H
	Result  string   // the submission as JSON, "" before the first one
	Refused bool     // the submission had fields the checks did not accept
	Foreign []string // sources only the form's own page resolves
	Record  string   // the record the form is tried as editing ("" for a new one)
	Editor  string
	Posted  bool
	NoneYet bool // nothing renderable is in force for the form
}

func tryURL(ref developerSchemaRef) string {
	return "/developer/forms/try?schema=" + url.QueryEscape(ref.String())
}

// DeveloperFormTry handles GET and POST /developer/forms/try.
func (h *UIHandler) DeveloperFormTry(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.developerClaims(w, r)
	if !ok {
		return
	}
	t := h.translateFunc(r)
	posted := r.Method == http.MethodPost
	if posted {
		r.Body = http.MaxBytesReader(w, r.Body, developerMaxBody)
		if err := r.ParseForm(); err != nil {
			developerRedirect(w, r, "/developer/forms", "error", "bad_request")
			return
		}
	}
	// the form's own fields may be named anything, "schema" included: the
	// form tried is always the one in the address
	ref, base, custom, ok := h.developerForm(r.Context(), claims.TenantID, r.URL.Query().Get("schema"))
	if !ok {
		developerRedirect(w, r, "/developer/forms", "error", "unknown_form")
		return
	}
	data := tryPageData{DeveloperPageData: h.developerData(w, r, t), Ref: ref, Editor: formsURL(ref, ""), Posted: posted}
	ctx := withoutTrace(r.Context())
	var pageBase []byte
	if !custom {
		pageBase = base
	}
	doc := resolveSchema(ctx, ref.Module, ref.Name, pageBase)
	form, err := fe.ParseSchema(doc)
	if doc == nil || err != nil || developerFormCheck(ref, base, custom, doc) != nil {
		// nothing in force, or something the editor explains better
		data.NoneYet = true
		h.page.Render(w, r, t("developer.try.title", ref.String()), "/developer/forms", developerTryPage(data, t))
		return
	}
	rc := fe.RenderContext{Locale: h.currentLocale(), CSRFToken: data.CSRFToken, Module: ref.Module, Form: ref.Name}
	if id, seen := developerTrace.identity(ref); seen {
		rc.Module, rc.Form = id.Module, id.Form
	}
	// a page that edits a record leaves that record out of its own lists (a
	// warehouse is not moved to itself): the try page does the same for the
	// record named in the address
	if id := r.URL.Query().Get("record"); developerRecordRE.MatchString(id) {
		data.Record = id
	}
	sources := &developerTrySources{h: h, empty: map[string]bool{}, exclude: data.Record}
	engine := &fe.FormEngine{Sources: sources, Translator: h.formEngineTranslator()}
	var values map[string]interface{}
	var errs map[string]string
	if posted {
		values, errs = h.developerSubmission(r, form, engine, rc, t)
		out := map[string]interface{}{"values": values}
		if len(errs) > 0 {
			out["errors"], data.Refused = errs, true
		}
		if text, err := json.MarshalIndent(out, "", "  "); err == nil {
			data.Result = string(text)
		}
	}
	// rendered now, so the lists that came back empty are known to the page
	html := mi.RenderToString(engine.Render(ctx, doc, values, errs, rc))
	data.Form = func(b *mi.Builder) mi.Node { return mi.Raw(html) }
	for s := range sources.empty {
		data.Foreign = append(data.Foreign, s)
	}
	sort.Strings(data.Foreign)
	h.page.Render(w, r, t("developer.try.title", ref.String()), "/developer/forms", developerTryPage(data, t))
}

func developerTryPage(data tryPageData, t func(key string, args ...any) string) mi.H {
	return func(b *mi.Builder) mi.Node {
		head := developerHeader{Title: t("developer.try.title", data.Ref.String()), Intro: t("developer.try.intro"),
			BackURL: data.Editor, BackText: data.Ref.String()}
		if data.NoneYet {
			return developerFrame(b, head, data.DeveloperPageData, Alert(t("developer.try.none"), "warning", false)(b))
		}
		var formBody []interface{}
		if len(data.Foreign) > 0 {
			formBody = append(formBody, b.P(mi.Class(devMutedClass), t("developer.try.sources", strings.Join(data.Foreign, ", "))))
		}
		action := tryURL(data.Ref)
		if data.Record != "" {
			action += "&record=" + data.Record
		}
		formBody = append(formBody, b.Form(mi.ID("dev-try-record"), mi.Class("flex flex-wrap items-end gap-3"), mi.Method("get"), mi.Action("/developer/forms/try"),
			b.Input(mi.Type("hidden"), mi.Name("schema"), mi.Value(data.Ref.String())),
			b.Div(b.Label(mi.For("dev-try-record-id"), mi.Class(devLabelClass), t("developer.try.record")),
				b.Input(mi.Type("text"), mi.ID("dev-try-record-id"), mi.Name("record"), mi.Attr("inputmode", "numeric"), mi.Attr("maxlength", "18"),
					mi.Attr("style", "width:8rem"), mi.Class(devInputClass+" mt-1"), mi.Value(data.Record))),
			Button(t("developer.try.record_apply"), "secondary", mi.Type("submit"))(b),
			b.P(mi.Class(devMutedClass), t("developer.try.record_hint"))))
		formBody = append(formBody, b.Form(mi.ID("dev-try"), mi.Class("space-y-4"), mi.Method("post"), mi.Action(action),
			mi.Attr("novalidate", "novalidate"), data.Form(b),
			b.Div(Button(t("developer.try.submit"), "primary", mi.Type("submit"))(b))))
		var result []interface{}
		switch {
		case !data.Posted:
			result = append(result, b.P(mi.Class(devMutedClass), t("developer.try.empty")))
		default:
			if data.Refused {
				result = append(result, Alert(t("developer.try.errors"), "warning", false)(b))
			}
			result = append(result, b.Div(mi.Class("dev-code"), mi.ID("dev-try-result"), data.Result))
		}
		return developerFrame(b, head, data.DeveloperPageData,
			b.Div(mi.Class("dev-cols"),
				developerPanel(b, "dynamic_form", t("developer.try.form"), "", nil, false, formBody...),
				developerPanel(b, "data_object", t("developer.try.result"), "", nil, false, result...)))
	}
}

// developerTryMaxOptions bounds a list read for the try page.
const developerTryMaxOptions = 200

// developerTryLabelFields are the fields a row is labelled with, in order of
// preference.
var developerTryLabelFields = []string{"name", "short_name", "legal_name", "description", "title", "email", "code"}

// developerTrySources resolves the lists of a form on the try page; empty
// collects the sources that brought nothing.
type developerTrySources struct {
	h       *UIHandler
	empty   map[string]bool
	exclude string // the ID of the record being edited, left out of every list
}

var developerRecordRE = regexp.MustCompile(`^[0-9]{1,18}$`)

// without drops the record being edited from a list.
func (s *developerTrySources) without(opts []fe.Option) []fe.Option {
	if s.exclude == "" {
		return opts
	}
	out := opts[:0:0]
	for _, o := range opts {
		if o.Value != s.exclude {
			out = append(out, o)
		}
	}
	return out
}

// Resolve implements formengine.SourceResolver. It never fails: a list that
// cannot be read is an empty one, named on the page.
func (s *developerTrySources) Resolve(ctx context.Context, source string) ([]fe.Option, error) {
	none := func() ([]fe.Option, error) {
		s.empty[source] = true
		return []fe.Option{}, nil
	}
	if s.h.xoluClient == nil {
		return none()
	}
	if _, ok := assetSourceQueries[source]; ok {
		opts, err := (&AssetSourceResolver{Client: s.h.xoluClient}).Resolve(ctx, source)
		if opts = s.without(opts); err != nil || len(opts) == 0 {
			return none()
		}
		return opts, nil
	}
	entity, ok := strings.CutPrefix(source, "entity:")
	entity = strings.ReplaceAll(entity, "-", "_")
	if !ok || !developerNameRE.MatchString(entity) {
		return none()
	}
	result, err := s.h.xoluClient.OQL(ctx, "SELECT * FROM "+entity)
	if err != nil || len(result.Data) == 0 {
		return none()
	}
	opts := make([]fe.Option, 0, len(result.Data))
	for _, row := range result.Data {
		if len(opts) == developerTryMaxOptions {
			break
		}
		if developerTryRowIsOff(row) {
			continue
		}
		id := strconv.FormatInt(toInt64(row["id"]), 10)
		label := "#" + id
		for _, f := range developerTryLabelFields {
			if v := strings.TrimSpace(toString(row[f])); v != "" {
				label = v
				break
			}
		}
		opts = append(opts, fe.Option{Value: id, Label: label})
	}
	if opts = s.without(opts); len(opts) == 0 {
		return none()
	}
	return opts, nil
}

// developerTryRowIsOff reports a row its own page would not offer: one
// switched off (enabled or active stored as false). It is the filter the
// pages that fill these lists share; a row without such a field is offered.
func developerTryRowIsOff(row map[string]any) bool {
	for _, f := range []string{"enabled", "active", "is_active"} {
		if v, ok := row[f].(bool); ok && !v {
			return true
		}
	}
	return false
}
