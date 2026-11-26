package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	mi "github.com/ha1tch/minty"

	"github.com/ha1tch/seam-ui/internal/auth"
	fe "github.com/ha1tch/seam-ui/internal/formengine"
)

// A page for a form made with the tools.
//
// A custom form (one that exists only as an override) is rendered by no page
// until code asks for it, which is right for a form that is part of a feature
// being written and too little for one that is the whole feature: a checklist,
// a short survey, a request. Where a deployment asks for it (the setting
// developer_custom_pages; off by default, since it is a page the tenant's users
// can reach), each custom form in force for a tenant has a page of its own,
//
//	/forms/<module>/<name>
//
// for any signed-in user of that tenant: the form as the engine renders it,
// with its lists filled as on the try page, checked on submission the same
// way, and each accepted submission kept as one record of the entity
// form_submission (tenant, form, who, when, and the values as JSON). The
// form's editor links to the page and lists the latest submissions.
//
// Only custom forms have such a page. A built-in form's page is its own; an
// override of one changes that page, not this.

// EntityFormSubmission is the entity that holds what custom form pages receive.
const EntityFormSubmission = "form_submission"

// developerSubmissionsShown is how many submissions the editor lists.
const developerSubmissionsShown = 20

// developerSubmission reads a posted form as the engine's pages do (repeatables
// rebuilt into their rows) and applies the checks made on a post: required
// fields, the bounds of repeatables, and query-picker values. The try page and
// the custom form pages share it.
func (h *UIHandler) developerSubmission(r *http.Request, form *fe.Form, engine *fe.FormEngine, rc fe.RenderContext, t func(key string, args ...any) string) (map[string]interface{}, map[string]string) {
	values := map[string]interface{}{}
	for k, v := range fe.ParseIndexedForm(r.PostForm, form.Fields) {
		values[k] = v
	}
	for _, f := range form.Fields {
		if len(f.Items) > 0 || !r.PostForm.Has(f.Name) {
			continue
		}
		if vs := r.PostForm[f.Name]; len(vs) > 1 {
			values[f.Name] = vs
		} else {
			values[f.Name] = r.PostForm.Get(f.Name)
		}
	}
	errs := map[string]string{}
	for _, f := range form.Fields {
		if !f.Required || len(f.Items) > 0 {
			continue
		}
		if v, ok := values[f.Name].(string); !ok && values[f.Name] == nil || ok && strings.TrimSpace(v) == "" {
			errs[f.Name] = t("developer.try.required")
		}
	}
	for k, v := range fe.ValidatePostedBounds(engine, rc, form, r.PostForm) {
		errs[k] = v
	}
	for k, v := range h.QueryPickerErrors(r.Context(), form, values, t) {
		errs[k] = v
	}
	return values, errs
}

// customPagesOn reports whether custom forms have pages here.
func (c Config) customPagesOn() bool {
	return c.EnableDeveloper && c.DevRuntime != nil && c.DevRuntime.Resolver != nil && c.DevSettings.CustomPages
}

func customFormURL(ref developerSchemaRef) string {
	return "/forms/" + url.PathEscape(ref.Module) + "/" + url.PathEscape(ref.Name)
}

// customFormInForce returns the definition of a custom form in force for the
// tenant, or false: not a custom form, nothing in force, or not renderable.
func (h *UIHandler) customFormInForce(tenant int64, module, name string) (developerSchemaRef, []byte, *fe.Form, bool) {
	ref := developerSchemaRef{module, name}
	if !h.config.customPagesOn() || tenant <= 0 || !developerNameRE.MatchString(module) || !developerNameRE.MatchString(name) {
		return ref, nil, nil, false
	}
	if _, builtIn := developerEmbeddedSchema(module, name); builtIn {
		return ref, nil, nil, false
	}
	doc := h.config.DevRuntime.Resolver.Resolve(tenant, module, name, nil)
	if doc == nil || developerFormCheck(ref, []byte(developerStarterForm), true, doc) != nil {
		return ref, nil, nil, false
	}
	form, err := fe.ParseSchema(doc)
	if err != nil {
		return ref, nil, nil, false
	}
	return ref, doc, form, true
}

// CustomFormPage handles GET and POST /forms/{module}/{name}.
func (h *UIHandler) CustomFormPage(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		http.NotFound(w, r)
		return
	}
	ref, doc, form, ok := h.customFormInForce(claims.TenantID, r.PathValue("module"), r.PathValue("name"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	t := h.translateFunc(r)
	ctx := withoutTrace(r.Context())
	rc := fe.RenderContext{Locale: h.currentLocale(), CSRFToken: h.GetCSRFToken(w, r), Module: ref.Module, Form: ref.Name}
	engine := &fe.FormEngine{Sources: &developerTrySources{h: h, empty: map[string]bool{}}, Translator: h.formEngineTranslator()}
	title := ref.String()
	var meta struct {
		Title string `json:"title"`
	}
	if json.Unmarshal(doc, &meta) == nil && strings.TrimSpace(meta.Title) != "" {
		title = meta.Title
	}
	status := http.StatusOK
	var values map[string]interface{}
	var errs map[string]string
	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, developerMaxBody)
		if err := r.ParseForm(); err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		values, errs = h.developerSubmission(r, form, engine, rc, t)
		if len(errs) == 0 {
			body, err := json.Marshal(values)
			if err == nil {
				_, err = h.xoluClient.Create(r.Context(), EntityFormSubmission, map[string]interface{}{
					"tenant_id": claims.TenantID, "module": ref.Module, "name": ref.Name, "user_id": claims.UserID,
					"values": string(body), "created_at": time.Now().UTC().Format(time.RFC3339),
				})
			}
			if err == nil {
				http.Redirect(w, r, customFormURL(ref)+"?saved=1", http.StatusSeeOther)
				return
			}
			h.logger.Error("custom form: submission not stored", "form", ref.String(), "error", err)
			errs = map[string]string{"": t("developer.custom.not_saved")}
		}
		status = http.StatusUnprocessableEntity
	}
	formHTML := engine.Render(ctx, doc, values, errs, rc)
	saved := r.Method == http.MethodGet && r.URL.Query().Get("saved") != ""
	failed := errs[""]
	h.page.RenderStatus(w, r, status, title, customFormURL(ref), func(b *mi.Builder) mi.Node {
		parts := []interface{}{mi.Class("space-y-6"), mi.Attr("style", "max-width:48rem"),
			b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), title)}
		if saved {
			parts = append(parts, Alert(t("developer.custom.saved"), "success", false)(b))
		}
		if failed != "" {
			parts = append(parts, Alert(failed, "danger", false)(b))
		}
		parts = append(parts, b.Div(mi.Class(devCardClass+" p-6"),
			b.Form(mi.ID("custom-form"), mi.Class("space-y-4"), mi.Method("post"), mi.Action(customFormURL(ref)), mi.Attr("novalidate", "novalidate"),
				formHTML(b), b.Div(Button(t("developer.custom.submit"), "primary", mi.Type("submit"))(b)))))
		return b.Div(parts...)
	})
}

// customSubmission is one stored submission, as the editor lists it.
type customSubmission struct {
	At     string
	By     int64
	Values string
}

// customSubmissions returns the latest submissions of a custom form for the
// tenant, newest first; none when the entity does not exist yet.
func (h *UIHandler) customSubmissions(ctx context.Context, tenant int64, ref developerSchemaRef) []customSubmission {
	if h.xoluClient == nil || !developerNameRE.MatchString(ref.Module) || !developerNameRE.MatchString(ref.Name) {
		return nil
	}
	res, err := h.xoluClient.OQL(ctx, fmt.Sprintf("SELECT * FROM %s WHERE tenant_id = %d AND module = '%s' AND name = '%s'",
		EntityFormSubmission, tenant, ref.Module, ref.Name))
	if err != nil {
		return nil // no submission yet (the entity appears with the first), or xolu is away
	}
	out := make([]customSubmission, 0, len(res.Data))
	for _, row := range res.Data {
		out = append(out, customSubmission{At: toString(row["created_at"]), By: toInt64(row["user_id"]), Values: toString(row["values"])})
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 { // stored oldest first
		out[i], out[j] = out[j], out[i]
	}
	if len(out) > developerSubmissionsShown {
		out = out[:developerSubmissionsShown]
	}
	return out
}

// developerCustomPagePanel is what a custom form's editor shows about its
// page: the link, and what it has received.
func developerCustomPagePanel(b *mi.Builder, ref developerSchemaRef, subs []customSubmission, t func(key string, args ...any) string) mi.Node {
	var rows []mi.H
	for i, s := range subs {
		s := s
		rows = append(rows, ListRow(i,
			ListCellMuted(s.At),
			ListCellMuted("#"+strconv.FormatInt(s.By, 10)),
			ListCellRich(func(b *mi.Builder) mi.Node {
				return b.Span(mi.Class(devMonoClass+" dev-wrap"), developerTruncate(s.Values, 400))
			})))
	}
	open := b.A(mi.Href(customFormURL(ref)), mi.Class(devLinkClass), mi.ID("dev-custom-page"),
		b.Span(mi.Class("material-icons text-lg"), "open_in_new"), b.Span(t("developer.custom.open")))
	return developerPanel(b, "inbox", t("developer.custom.submissions"), customFormURL(ref), []interface{}{open}, true,
		b.Div(mi.ID("dev-custom-submissions"), ListTable(ListTableConfig{
			Columns:   []ListColumn{{Label: t("developer.forms.when")}, {Label: t("developer.forms.by")}, {Label: t("developer.custom.values")}},
			EmptyIcon: "inbox", EmptyTitle: t("developer.custom.none"),
		}, rows)(b)))
}
