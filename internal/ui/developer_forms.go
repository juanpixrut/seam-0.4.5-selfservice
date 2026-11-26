package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	mi "github.com/ha1tch/minty"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/features"
	fe "github.com/ha1tch/seam-ui/internal/formengine"
	developerschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/developer"
	"github.com/ha1tch/seam-ui/internal/overrides"
	"github.com/ha1tch/seam-ui/internal/schemaoverlay"
)

// The Form definitions section (campaign DEV-20): the filesystem layer's status
// and reload, the catalogue of every form with the layer that supplies it, and
// an editor for one form at a time: raw JSON, a Preview that validates and
// renders it with the real form engine without saving, Save, and Remove. The
// editor's own form is a form-engine schema (developer/schema_edit).
//
// Saving goes through schemaoverlay.FileLayer.Save, which validates with the
// form engine's own parser before anything is written, so a definition the
// engine cannot read never reaches the disk. Only forms in the catalogue can
// be opened or saved, so the schema parameter can never name a path. A saved
// definition replaces the whole form on its pages for every tenant (the
// filesystem layer is global, DEC-9).

const developerMaxSchemaBody = schemaoverlay.MaxSchemaBytes + 64<<10

type formsRow struct {
	Ref    developerSchemaRef
	Layer  string
	Unused bool // not a built-in form: one created with the tools, or an override under a misspelt name

	// Problem is why the definition in force cannot be rendered ("" when it
	// can): an override written by hand that the tools would have refused.
	Problem string

	// Uses is what the search found in the definition when it did not match
	// the form's name: "widget:select", "source:entity:users", "field:email".
	Uses []string
}

type formsEditor struct {
	Ref         developerSchemaRef
	HasOverride bool
	Custom      bool // not built in: it exists only as an override (developerForm)
	Form        mi.H
	Text        string // the definition shown in the editor
	// Broken is why the override in force cannot be rendered ("" when it can).
	Broken string
	// LabelKeys are the fields whose label a translation key supplies: a title
	// in the definition does not change them.
	LabelKeys []formLabelKey
	// LabelKeysSeen reports that the keys are the ones the form's page asked
	// for, not the naming convention's.
	LabelKeysSeen bool
	// CustomPage says the form has a page of its own (developer_custom.go);
	// Submissions are the latest that page received.
	CustomPage  bool
	Submissions []customSubmission
	Warnings    []string
	Problem     string // why the posted definition is not acceptable; "" if it is
	Previewed   bool   // a preview was asked for and the definition is valid
	PreviewHTML string // the rendered preview
	PreviewErr  string // the engine could not render it

	// Diff is what the text in the editor changes against the built-in
	// definition (empty when it changes nothing); DiffTooLarge says it was too
	// big to compare line by line.
	Diff         []diffLine
	DiffTooLarge bool

	// Scope is the selected scope (scopeTenant or scopeFile) and Scopes the ones
	// on offer; HasOverride, the text and the diff are that scope's. Shadow is
	// the translation key of a note when the other scope's override hides or is
	// hidden by this one (a file override outranks a tenant's, DEC-4). History is
	// the scope's versions of this form, newest first (the tenant's, or the ones
	// the tools recorded for the file), and CurrentVersion the one that is live
	// (0 for none, or for a file changed by hand since).
	Scope          string
	Scopes         []string
	Shadow         string
	History        []overrides.SchemaVersion
	CurrentVersion int
}

// FormsPageData holds what the Form definitions page renders.
type FormsPageData struct {
	DeveloperPageData
	Rows     []formsRow
	Selected string
	Editor   *formsEditor
	Writable bool
	Designer bool // the form designer is on: offer to open the form in it

	// The list's filters: part of a form's name, and one module ("" for all).
	Query, Module string
	Modules       []string

	// New, when not nil, is the form that creates a form: its two fields, and
	// the scopes it can be created in.
	New *formsNew
}

// formsNew is what the "new form" panel shows.
type formsNew struct {
	Form   mi.H
	Scopes []string
	Scope  string
}

// developerPretty indents a JSON document for editing; anything that is not
// valid JSON comes back unchanged.
func developerPretty(doc []byte) string {
	var out bytes.Buffer
	if err := json.Indent(&out, doc, "", "  "); err != nil {
		return string(doc)
	}
	return out.String()
}

// developerPreview renders a candidate definition with the real form engine,
// to a string, and never lets a definition that confuses the engine take the
// page down.
func (h *UIHandler) developerPreview(ctx context.Context, doc []byte) (out string, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("the form engine could not render it: %v", rec)
		}
	}()
	engine := &fe.FormEngine{Translator: h.formEngineTranslator()}
	return mi.RenderToString(engine.Render(withoutTrace(ctx), doc, nil, nil, fe.RenderContext{Locale: h.currentLocale(), Module: "preview", Form: "preview"})), nil
}

// renderDeveloperForms renders the Form definitions page. schemaParam is the
// selected form ("module/name"); posted, when not nil, is the definition text
// the editor was posted with (a preview, or a save that was refused); problem
// is a refusal to show; status, when not 0, is the response status.
func (h *UIHandler) renderDeveloperForms(w http.ResponseWriter, r *http.Request, schemaParam string, posted *string, problem string, status int) {
	t := h.translateFunc(r)
	ctx := r.Context()
	data := FormsPageData{DeveloperPageData: h.developerData(w, r, t), Selected: schemaParam}
	rt := h.config.DevRuntime
	data.Writable = len(h.scopesWithGlobal(ctx, developerScopes(rt, rt != nil && rt.Schemas != nil))) > 0
	data.Designer = h.config.flagOn(features.DeveloperFormDesigner)
	tenant := data.TenantID

	if rt != nil && rt.Resolver != nil {
		for _, m := range developerSchemaModules {
			data.Modules = append(data.Modules, m.module)
		}
		data.Query = developerTruncate(strings.TrimSpace(r.URL.Query().Get("q")), developerMaxQuery)
		if m := r.URL.Query().Get("module"); developerHasModule(m) {
			data.Module = m
		}
		needle := strings.ToLower(data.Query)
		// the search matches a form by its name, or by what its definition in
		// force uses: a widget, a source, a field (uses says which)
		uses := map[developerSchemaRef][]string{}
		shown := func(ref developerSchemaRef) bool {
			if data.Module != "" && ref.Module != data.Module {
				return false
			}
			if needle == "" || strings.Contains(strings.ToLower(ref.String()), needle) {
				return true
			}
			base, _ := developerEmbeddedSchema(ref.Module, ref.Name)
			if m := developerFormUses(rt.Resolver.Resolve(tenant, ref.Module, ref.Name, base), needle); len(m) > 0 {
				uses[ref] = m
				return true
			}
			return false
		}
		listed := map[developerSchemaRef]bool{}
		// an override in force is checked, so one written by hand that the
		// tools would have refused is flagged here instead of found on its page
		problem := func(ref developerSchemaRef, base []byte, custom bool) string {
			doc := rt.Resolver.Resolve(tenant, ref.Module, ref.Name, nil)
			if doc == nil {
				return ""
			}
			if err := developerFormCheck(ref, base, custom, doc); err != nil {
				return developerProblemText(err)
			}
			return ""
		}
		for _, ref := range developerEmbeddedSchemas() {
			listed[ref] = true
			if shown(ref) {
				base, _ := developerEmbeddedSchema(ref.Module, ref.Name)
				data.Rows = append(data.Rows, formsRow{Ref: ref, Layer: string(rt.Resolver.EffectiveLayer(tenant, ref.Module, ref.Name)), Problem: problem(ref, base, false)})
			}
		}
		for _, k := range rt.Resolver.Overridden(tenant) {
			ref := developerSchemaRef{k.Module, k.Name}
			if !listed[ref] && shown(ref) {
				listed[ref] = true
				data.Rows = append(data.Rows, formsRow{Ref: ref, Layer: string(rt.Resolver.EffectiveLayer(tenant, k.Module, k.Name)), Unused: true, Problem: problem(ref, nil, true)})
			}
		}
		// files the directory set aside: flagged on their row, and listed even
		// when nothing of theirs is in force, so they can be opened and mended
		if rt.Schemas != nil {
			refused := map[developerSchemaRef]string{}
			for _, f := range rt.Schemas.Status().Refused {
				refused[developerSchemaRef{f.Key.Module, f.Key.Name}] = developerProblemString(f.Reason)
			}
			for i := range data.Rows {
				if why, ok := refused[data.Rows[i].Ref]; ok {
					data.Rows[i].Problem = why
					delete(refused, data.Rows[i].Ref)
				}
			}
			var rest []developerSchemaRef
			for ref := range refused {
				// a file under a name no form can have is on the Overview only
				if !listed[ref] && shown(ref) && developerNameRE.MatchString(ref.Module) && developerNameRE.MatchString(ref.Name) {
					rest = append(rest, ref)
				}
			}
			sort.Slice(rest, func(i, j int) bool { return rest[i].String() < rest[j].String() })
			for _, ref := range rest {
				data.Rows = append(data.Rows, formsRow{Ref: ref, Unused: true, Problem: refused[ref]})
			}
		}
		for i := range data.Rows {
			data.Rows[i].Uses = uses[data.Rows[i].Ref]
		}
	}

	if schemaParam != "" {
		ref, base, custom, ok := h.developerForm(ctx, tenant, schemaParam)
		if !ok {
			data.Error = t("developer.err.unknown_form")
			data.Selected = ""
		} else {
			data.Editor = h.formsEditor(ctx, data, ref, base, custom, posted, problem, r.FormValue("scope"))
		}
	} else if data.Writable && r.URL.Query().Get("new") != "" {
		scopes := h.scopesWithGlobal(ctx, developerScopes(rt, rt != nil && rt.Schemas != nil))
		engine := &fe.FormEngine{Translator: h.formEngineTranslator()}
		data.New = &formsNew{Scopes: scopes, Scope: developerScope(r.URL.Query().Get("scope"), scopes),
			Form: engine.Render(ctx, resolveSchema(ctx, "developer", "form_new", developerschemas.Schema("form_new")),
				map[string]interface{}{"module": data.Module}, nil,
				fe.RenderContext{Locale: h.currentLocale(), CSRFToken: data.CSRFToken, Module: "developer", Form: "form_new"})}
	}
	if problem != "" && data.Error == "" {
		data.Error = t("developer.err.invalid_json") + ": " + developerTruncate(problem, 600)
	}
	if status == 0 {
		status = http.StatusOK
	}
	h.page.RenderStatus(w, r, status, t("developer.nav.forms"), "/developer/forms", DeveloperFormsPage(data, t))
}

func (h *UIHandler) formsEditor(ctx context.Context, data FormsPageData, ref developerSchemaRef, base []byte, custom bool, posted *string, problem string, scopeParam string) *formsEditor {
	rt := h.config.DevRuntime
	effective := base
	e := &formsEditor{Ref: ref, Custom: custom}
	if rt != nil && rt.Resolver != nil {
		effective = rt.Resolver.Resolve(data.TenantID, ref.Module, ref.Name, base)
	}
	text := developerPretty(effective)
	// the override of each scope is read from its own source, not inferred
	e.Scopes = h.scopesWithGlobal(ctx, developerScopes(rt, rt != nil && rt.Schemas != nil))
	e.Scope = developerScope(scopeParam, e.Scopes)
	var tenantCur overrides.SchemaVersion
	hasTenant := false
	if rt != nil && rt.Overrides != nil && data.TenantID > 0 {
		tenantCur, hasTenant, _ = rt.Overrides.Current(ctx, data.TenantID, ref.Module, ref.Name)
	}
	var fileDoc []byte
	hasFile := false
	if rt != nil && rt.Schemas != nil {
		fileDoc, hasFile, _ = rt.Schemas.Read(ref.Module, ref.Name)
	}
	switch e.Scope {
	case scopeTenant:
		if hasTenant {
			text, e.HasOverride, e.CurrentVersion = developerPretty(tenantCur.Doc), true, tenantCur.Version
		}
		if hasFile {
			e.Shadow = "developer.forms.shadowed_by_file"
		}
		e.History, _ = rt.Overrides.History(ctx, data.TenantID, ref.Module, ref.Name)
	case scopeGlobal:
		if cur, has, _ := rt.Overrides.Current(ctx, overrides.GlobalTenant, ref.Module, ref.Name); has {
			text, e.HasOverride, e.CurrentVersion = developerPretty(cur.Doc), true, cur.Version
		} else {
			text = developerPretty(base) // what the scope would start from, not what this tenant sees
		}
		if hasTenant || hasFile {
			e.Shadow = "developer.scope.global.shadowed"
		}
		e.History, _ = rt.Overrides.History(ctx, overrides.GlobalTenant, ref.Module, ref.Name)
	case scopeFile:
		if hasFile {
			text, e.HasOverride = developerPretty(fileDoc), true
		}
		if hasTenant {
			e.Shadow = "developer.forms.shadows_tenant"
		}
		e.History = fileSchemaHistory(rt.Schemas, ref)
		// the newest version is the live one unless the file was changed by hand since
		if hasFile && len(e.History) > 0 && !e.History[0].Removed && sameJSONDoc(e.History[0].Doc, fileDoc) {
			e.CurrentVersion = e.History[0].Version
		}
	default:
		e.HasOverride = !bytes.Equal(effective, base)
	}
	if e.HasOverride && !custom {
		e.Warnings = schemaoverlay.Compat(base, []byte(text))
	}
	if e.HasOverride && posted == nil {
		if err := developerFormCheck(ref, base, custom, []byte(text)); err != nil {
			e.Broken = developerProblemText(err)
		}
	}
	if posted != nil {
		text = *posted
		doc := []byte(text)
		switch {
		case problem != "":
			e.Problem = problem
		case rt == nil || rt.Resolver == nil:
		default:
			if err := rt.Resolver.Validate(doc); err != nil {
				e.Problem = developerTruncate(err.Error(), 600)
			} else if err := developerFormCheck(ref, base, custom, doc); err != nil {
				e.Problem = developerProblemText(err)
			} else {
				if !custom {
					e.Warnings = schemaoverlay.Compat(base, doc)
				}
				e.Previewed = true
				html, err := h.developerPreview(ctx, doc)
				if err != nil {
					e.PreviewErr = err.Error()
				} else {
					e.PreviewHTML = html
				}
			}
		}
	}
	if !custom { // a custom form has no built-in definition to compare with
		e.Diff, e.DiffTooLarge = developerLineDiff(developerPretty(base), text)
	}
	e.Text = text
	if custom && h.config.customPagesOn() {
		e.CustomPage = true
		e.Submissions = h.customSubmissions(ctx, data.TenantID, ref)
	}
	e.LabelKeys = h.developerLabelKeys(data.TenantID, ref, []byte(text))
	_, e.LabelKeysSeen = developerTrace.identity(ref)
	engine := &fe.FormEngine{Translator: h.formEngineTranslator()}
	// the form's name is posted but not shown; the note is kept with the
	// version in both scopes
	e.Form = engine.Render(ctx, resolveSchema(ctx, "developer", "schema_edit", developerschemas.Schema("schema_edit")),
		map[string]interface{}{"schema": ref.String(), "json": text}, nil,
		fe.RenderContext{Locale: h.currentLocale(), CSRFToken: data.CSRFToken, Module: "developer", Form: "schema_edit",
			HiddenFields: []string{"schema"}})
	return e
}

// DeveloperForms handles GET /developer/forms.
func (h *UIHandler) DeveloperForms(w http.ResponseWriter, r *http.Request) {
	h.applyDeveloperPrefs(w, r, false)
	h.renderDeveloperForms(w, r, r.URL.Query().Get("schema"), nil, "", 0)
}

// formsPost reads and bounds a POST of the editor. ok is false when it has
// already answered.
func (h *UIHandler) formsPost(w http.ResponseWriter, r *http.Request) (claims *auth.Claims, ref developerSchemaRef, text, scope string, ok bool) {
	claims, has := auth.ClaimsFromContext(r.Context())
	rt := h.config.DevRuntime
	if !has || rt == nil {
		h.Error403(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, developerMaxSchemaBody)
	if err := r.ParseForm(); err != nil {
		developerRedirect(w, r, "/developer/forms", "error", "bad_request")
		return
	}
	ref, _, _, found := h.developerForm(r.Context(), claims.TenantID, r.FormValue("schema"))
	if !found {
		developerRedirect(w, r, "/developer/forms", "error", "unknown_form")
		return
	}
	scopes := h.scopesWithGlobal(r.Context(), developerScopes(rt, rt.Schemas != nil))
	if len(scopes) == 0 {
		http.Redirect(w, r, developerAppend(formsURL(ref, ""), "error", "not_configured"), http.StatusSeeOther)
		return
	}
	if !developerScopeValid(r.FormValue("scope"), scopes) {
		http.Redirect(w, r, developerAppend(formsURL(ref, ""), "error", "bad_scope"), http.StatusSeeOther)
		return
	}
	scope = developerScope(r.FormValue("scope"), scopes)
	return claims, ref, strings.TrimSpace(r.FormValue("json")), scope, true
}

// formsURL is the editor of one form, in a scope when one is given.
func formsURL(ref developerSchemaRef, scope string) string {
	u := "/developer/forms?schema=" + url.QueryEscape(ref.String())
	if scope != "" {
		u += "&scope=" + url.QueryEscape(scope)
	}
	return u
}

// DeveloperFormPreview handles POST /developer/forms/preview: validates the
// posted definition and renders it, writing nothing.
func (h *UIHandler) DeveloperFormPreview(w http.ResponseWriter, r *http.Request) {
	_, ref, text, _, ok := h.formsPost(w, r)
	if !ok {
		return
	}
	h.renderDeveloperForms(w, r, ref.String(), &text, "", 0)
}

// DeveloperFormSave handles POST /developer/forms/save.
func (h *UIHandler) DeveloperFormSave(w http.ResponseWriter, r *http.Request) {
	claims, ref, text, scope, ok := h.formsPost(w, r)
	if !ok {
		return
	}
	rt := h.config.DevRuntime
	_, base, custom, _ := h.developerForm(r.Context(), claims.TenantID, ref.String())
	if custom {
		base = nil // nothing built in to compare with: no compatibility notes
	}
	var err error
	details := map[string]any{"action": "set", "scope": scope}
	// the gate first: a definition the engine could parse but not render is
	// refused like one it cannot parse (it needs valid JSON to say anything)
	if rt.Resolver != nil && rt.Resolver.Validate([]byte(text)) == nil {
		_, checkBase, _, _ := h.developerForm(r.Context(), claims.TenantID, ref.String())
		err = developerFormCheck(ref, checkBase, custom, []byte(text))
	}
	switch {
	case err != nil:
	case scopeStored(scope):
		var v overrides.SchemaVersion
		v, _, err = rt.Overrides.SaveSchema(r.Context(), scopeTenantID(scope, claims.TenantID), ref.Module, ref.Name, []byte(text), base, claims.UserID, r.FormValue("note"))
		details["version"] = v.Version
	default:
		_, err = rt.Schemas.SaveAs(ref.Module, ref.Name, []byte(text), base, claims.UserID, r.FormValue("note"))
	}
	if err != nil {
		h.logger.Warn("developer: form definition refused", "form", ref.String(), "scope", scope, "error", err)
		if errors.Is(err, schemaoverlay.ErrRejected) || isNotRenderable(err) {
			// Keep what was typed and say why: nothing was written.
			h.renderDeveloperForms(w, r, ref.String(), &text, developerProblemText(err), http.StatusUnprocessableEntity)
			return
		}
		http.Redirect(w, r, developerAppend(formsURL(ref, scope), "error", developerErrorCode(err)), http.StatusSeeOther)
		return
	}
	h.auditLogger.LogDeveloperChange(r.Context(), r, claims.UserID, claims.TenantID, "form_definition", ref.String(), details)
	http.Redirect(w, r, developerAppend(formsURL(ref, scope), "msg", "saved"), http.StatusSeeOther)
}

// DeveloperFormRemove handles POST /developer/forms/remove.
func (h *UIHandler) DeveloperFormRemove(w http.ResponseWriter, r *http.Request) {
	claims, ref, _, scope, ok := h.formsPost(w, r)
	if !ok {
		return
	}
	rt := h.config.DevRuntime
	var err error
	if scopeStored(scope) {
		err = rt.Overrides.RemoveSchema(r.Context(), scopeTenantID(scope, claims.TenantID), ref.Module, ref.Name, claims.UserID)
	} else {
		err = rt.Schemas.RemoveAs(ref.Module, ref.Name, claims.UserID)
	}
	if err != nil {
		h.logger.Warn("developer: form definition not removed", "form", ref.String(), "scope", scope, "error", err)
		http.Redirect(w, r, developerAppend(formsURL(ref, scope), "error", developerErrorCode(err)), http.StatusSeeOther)
		return
	}
	h.auditLogger.LogDeveloperChange(r.Context(), r, claims.UserID, claims.TenantID, "form_definition", ref.String(),
		map[string]any{"action": "remove", "scope": scope})
	http.Redirect(w, r, developerAppend(formsURL(ref, scope), "msg", "removed"), http.StatusSeeOther)
}

// DeveloperFormRestore handles POST /developer/forms/restore: an older version
// of the override becomes current again, as a new version, in the scope it
// belongs to (the tenant's history, or the one the tools keep for the files).
// The scope must be named: a restore is never guessed. The old definition is
// checked again, since the engine may have changed since it was saved.
func (h *UIHandler) DeveloperFormRestore(w http.ResponseWriter, r *http.Request) {
	claims, ref, _, scope, ok := h.formsPost(w, r)
	if !ok {
		return
	}
	if r.FormValue("scope") != scope {
		http.Redirect(w, r, developerAppend(formsURL(ref, ""), "error", "bad_scope"), http.StatusSeeOther)
		return
	}
	fail := func(code string) {
		http.Redirect(w, r, developerAppend(formsURL(ref, scope), "error", code), http.StatusSeeOther)
	}
	version, err := strconv.Atoi(r.FormValue("version"))
	if err != nil || version < 1 {
		fail("unknown_version")
		return
	}
	rt := h.config.DevRuntime
	_, base, custom, _ := h.developerForm(r.Context(), claims.TenantID, ref.String())
	var history []overrides.SchemaVersion
	if scopeStored(scope) {
		history, err = rt.Overrides.History(r.Context(), scopeTenantID(scope, claims.TenantID), ref.Module, ref.Name)
		if err != nil {
			fail(developerErrorCode(err))
			return
		}
	} else {
		history = fileSchemaHistory(rt.Schemas, ref)
	}
	var old *overrides.SchemaVersion
	for i := range history {
		if history[i].Version == version {
			old = &history[i]
		}
	}
	switch {
	case old == nil:
		fail("unknown_version")
		return
	case old.Removed:
		fail("not_restorable")
		return
	}
	if err := developerFormCheck(ref, base, custom, old.Doc); err != nil {
		h.logger.Warn("developer: form definition not restored", "form", ref.String(), "version", version, "error", err)
		fail("invalid_json")
		return
	}
	newVersion := 0
	if scopeStored(scope) {
		var v overrides.SchemaVersion
		v, err = rt.Overrides.RestoreSchema(r.Context(), scopeTenantID(scope, claims.TenantID), ref.Module, ref.Name, version, claims.UserID)
		newVersion = v.Version
	} else {
		compat := base
		if custom {
			compat = nil
		}
		_, err = rt.Schemas.SaveAs(ref.Module, ref.Name, old.Doc, compat, claims.UserID, fmt.Sprintf("restored from version %d", version))
	}
	if err != nil {
		h.logger.Warn("developer: form definition not restored", "form", ref.String(), "version", version, "error", err)
		fail(developerErrorCode(err))
		return
	}
	h.auditLogger.LogDeveloperChange(r.Context(), r, claims.UserID, claims.TenantID, "form_definition", ref.String(),
		map[string]any{"action": "restore", "scope": scope, "from": version, "version": newVersion})
	http.Redirect(w, r, developerAppend(formsURL(ref, scope), "msg", "restored"), http.StatusSeeOther)
}

// developerWhen is when a version was made, or a dash for one that was there
// before the history began.
func developerWhen(at time.Time) string {
	if at.IsZero() {
		return "—"
	}
	return at.UTC().Format("2006-01-02 15:04") + " UTC"
}

// developerWho is who made a version: a user, the files themselves for a
// change made in the directory without the tools (ChangedOutside), or a dash
// when nobody is recorded.
func developerWho(t func(key string, args ...any) string, by int64) string {
	if by == schemaoverlay.ChangedOutside {
		return t("developer.history.outside")
	}
	if by <= 0 {
		return "—"
	}
	return "#" + strconv.FormatInt(by, 10)
}

// developerVersionsTable lists the scope's versions of the form, newest first,
// with a Restore button on every version that has a definition and is not the
// one that is live.
func developerVersionsTable(b *mi.Builder, data FormsPageData, t func(key string, args ...any) string) mi.Node {
	e := data.Editor
	var rows []mi.H
	for i, v := range e.History {
		v := v
		action := func(b *mi.Builder) mi.Node {
			switch {
			case v.Version == e.CurrentVersion:
				return Badge(t("developer.forms.current"), "success")(b)
			case v.Removed:
				return Badge(t("developer.forms.removed_version"), "")(b)
			}
			return b.Form(mi.Class("inline"), mi.Method("post"), mi.Action("/developer/forms/restore"), CSRFField(data.CSRFToken)(b),
				b.Input(mi.Type("hidden"), mi.Name("schema"), mi.Value(e.Ref.String())),
				b.Input(mi.Type("hidden"), mi.Name("scope"), mi.Value(e.Scope)),
				b.Input(mi.Type("hidden"), mi.Name("version"), mi.Value(strconv.Itoa(v.Version))),
				IconButton("restore", t("developer.forms.restore"), "secondary", mi.Type("submit"))(b))
		}
		rows = append(rows, ListRow(i,
			ListCell("v"+strconv.Itoa(v.Version)),
			ListCellMuted(developerWhen(v.ChangedAt)),
			ListCellMuted(developerWho(t, v.ChangedBy)),
			ListCellRich(func(b *mi.Builder) mi.Node { return b.Span(mi.Class("text-gray-900 dark:text-white dev-wrap"), v.Note) }),
			ListCellRight(action)))
	}
	return ListTable(ListTableConfig{
		Columns: []ListColumn{{Label: t("developer.forms.version")}, {Label: t("developer.forms.when")}, {Label: t("developer.forms.by")},
			{Label: t("developer.forms.note")}, {Label: "", Right: true}},
		EmptyIcon: "history", EmptyTitle: t("developer.forms.versions"),
	}, rows)(b)
}

// DeveloperFormsPage renders the Form definitions section: the list of forms,
// or, when one is open, its editor on a page of its own.
func DeveloperFormsPage(data FormsPageData, t func(key string, args ...any) string) mi.H {
	t = developerTrans(t)
	return func(b *mi.Builder) mi.Node {
		if data.Editor != nil {
			return developerFormEditorPage(b, data, t)
		}
		var parts []interface{}
		if data.New != nil {
			parts = append(parts, b.Div(mi.ID("new"), developerPanel(b, "add", t("developer.forms.new"), t("developer.forms.new_note"),
				[]interface{}{b.A(mi.Href("/developer/forms"), mi.Class("inline-flex "+devIconBtn),
					mi.Attr("title", t("action.close")), mi.Attr("aria-label", t("action.close")), b.Span(mi.Class("material-icons text-lg"), "close"))}, false,
				b.Form(mi.Class("space-y-4"), mi.Attr("style", "max-width:40rem"), mi.Method("post"), mi.Action("/developer/forms/create"),
					data.New.Form(b),
					developerScopeSelect(b, t, data.New.Scopes, data.New.Scope),
					b.Div(IconButton("add", t("developer.forms.create"), "primary", mi.Type("submit"))(b))))))
		}
		parts = append(parts, developerLayerCard(b, data.DeveloperPageData, developerSchemasLayer(data.DeveloperPageData, t), t))

		list := []interface{}{mi.ID("forms"), mi.Class("space-y-4"),
			b.H2(mi.Class("text-lg font-semibold text-gray-900 dark:text-white"), t("developer.forms.list"))}
		if len(data.Modules) > 0 {
			modules := []SelectOption{{Value: "", Label: t("developer.forms.all_modules"), Selected: data.Module == ""}}
			for _, m := range data.Modules {
				modules = append(modules, SelectOption{Value: m, Label: m, Selected: m == data.Module})
			}
			list = append(list, ListFilterBar(ListFilterConfig{
				Action: "/developer/forms", SearchName: "q", SearchHint: t("developer.forms.search"), SearchValue: data.Query,
				Filters: []ListFilterField{{Name: "module", Options: modules}},
			})(b))
		}
		var rows []mi.H
		for i, row := range data.Rows {
			row := row
			var attrs []interface{}
			if data.Selected == row.Ref.String() {
				attrs = []interface{}{mi.Attr("data-dev-selected", "1")}
			}
			rows = append(rows, ListRowWithAttrs(i, attrs,
				ListCellRich(func(b *mi.Builder) mi.Node {
					link := b.A(mi.Href(formsURL(row.Ref, "")), mi.Attr("data-row-link", "1"), mi.Class("font-mono text-xs font-medium text-indigo-600 dark:text-indigo-400 hover:underline"), row.Ref.String())
					if len(row.Uses) == 0 {
						return link
					}
					// found by what the definition uses, not by its name
					return b.Div(link, b.Div(mi.Class("dev-uses text-xs text-gray-500 dark:text-gray-400 font-mono"), strings.Join(row.Uses, " · ")))
				}),
				ListCellRich(func(b *mi.Builder) mi.Node { return Badge(row.Ref.Module, "")(b) }),
				ListCellRich(func(b *mi.Builder) mi.Node {
					source := []interface{}{mi.Class("flex flex-wrap items-center gap-2"), developerLayerBadge(b, t, row.Layer)}
					if row.Unused {
						source = append(source, IconBadge("info", t("developer.forms.custom"), "info", mi.Attr("title", t("developer.forms.custom_note")))(b))
					}
					if row.Problem != "" {
						source = append(source, IconBadge("error", t("developer.forms.broken"), "danger", mi.Attr("title", row.Problem))(b))
					}
					return b.Div(source...)
				}),
				ListCellRight(func(b *mi.Builder) mi.Node {
					acts := []interface{}{mi.Class("flex items-center justify-end gap-1"),
						b.A(mi.Href(formsURL(row.Ref, "")), mi.Class(devIconBtn), mi.Attr("title", t("developer.forms.open")), mi.Attr("aria-label", t("developer.forms.open")+" "+row.Ref.String()),
							b.Span(mi.Class("material-icons text-lg"), "data_object"))}
					if data.Designer {
						acts = append(acts, b.A(mi.Href(designerURL(row.Ref, "")), mi.Class(devIconBtn), mi.Attr("title", t("developer.designer.open")), mi.Attr("aria-label", t("developer.designer.open")+" "+row.Ref.String()),
							b.Span(mi.Class("material-icons text-lg"), "design_services")))
					}
					return b.Div(acts...)
				}),
			))
		}
		list = append(list, ListTable(ListTableConfig{
			Columns: []ListColumn{{Label: t("developer.forms.col.form")}, {Label: t("developer.forms.col.module")},
				{Label: t("developer.forms.col.source")}, {Label: t("asset.list.actions"), Right: true}},
			EmptyIcon: "dynamic_form", EmptyTitle: t("developer.forms.empty"), EmptyMessage: t("developer.forms.empty_hint"),
		}, rows)(b))
		if len(data.Rows) > 0 {
			list = append(list, b.Div(mi.Class(devMutedClass), t("developer.forms.count", len(data.Rows))))
		}
		parts = append(parts, b.Div(list...))
		head := developerHeader{Title: t("developer.nav.forms"), Intro: t("developer.forms.intro"),
			Actions: []interface{}{developerExportLink(b, t)}}
		if data.Writable {
			head.Actions = append(head.Actions, developerNewLink(b, "/developer/forms?new=1#new", t("developer.forms.new")))
		}
		return developerFrame(b, head, data.DeveloperPageData, parts...)
	}
}

// developerFormEditorPage is the editor of one form: where it is saved, then
// the definition, its preview, what it changes and its versions as tabs.
func developerFormEditorPage(b *mi.Builder, data FormsPageData, t func(key string, args ...any) string) mi.Node {
	e := data.Editor
	head := developerHeader{Title: e.Ref.String(), Intro: t("developer.forms.editor_intro"),
		BackURL: "/developer/forms", BackText: t("developer.nav.forms")}
	head.Actions = append(head.Actions, b.A(mi.Href(tryURL(e.Ref)), mi.Class(devLinkClass), mi.ID("dev-try-link"),
		b.Span(mi.Class("material-icons text-lg"), "play_arrow"), b.Span(t("developer.try.action"))))
	if data.Designer {
		head.Actions = append(head.Actions, b.A(mi.Href(designerURL(e.Ref, e.Scope)), mi.Class(devLinkClass),
			b.Span(mi.Class("material-icons text-lg"), "design_services"), b.Span(t("developer.designer.open"))))
	}
	var parts []interface{}
	if e.Problem != "" {
		parts = append(parts, b.Div(mi.Class("dev-problem"),
			b.P(mi.Class("font-semibold"), t("developer.forms.problem")), b.Div(mi.Class("dev-pre"), e.Problem)))
	}
	if e.Broken != "" {
		parts = append(parts, b.Div(mi.Class("dev-problem"),
			b.P(mi.Class("font-semibold"), t("developer.forms.broken_note")), b.Div(mi.Class("dev-pre"), e.Broken)))
	}
	if e.Custom {
		// what it takes to use a form that only the tools know
		parts = append(parts, developerPanel(b, "integration_instructions", t("developer.forms.custom"), t("developer.forms.custom_note"), nil, false,
			b.P(mi.Class(devMutedClass), t("developer.forms.use.render")),
			b.Div(mi.Class("fe-code"), `engine.Render(ctx, resolveSchema(ctx, "`+e.Ref.Module+`", "`+e.Ref.Name+`", nil), values, errs, rc)`),
			b.P(mi.Class(devMutedClass), t("developer.forms.use.source")),
			b.Div(mi.Class("fe-code"), "internal/formengine/schemas/"+e.Ref.Module+"/"+e.Ref.Name+".json")))
	}
	if e.CustomPage {
		parts = append(parts, developerCustomPagePanel(b, e.Ref, e.Submissions, t))
	}
	if len(e.LabelKeys) > 0 {
		keys := []interface{}{mi.Class("flex flex-wrap items-center gap-2")}
		for _, lk := range e.LabelKeys {
			keys = append(keys, b.A(mi.Href(translationsURL("", "", showAll, 1, lk.Key)+"#edit"), mi.Class("dev-keylink"), mi.Attr("title", lk.Key),
				b.Span(mi.Class("font-mono"), lk.Field), b.Span(mi.Class("material-icons text-sm"), "translate")))
		}
		parts = append(parts, b.Div(mi.Class("dev-labelkeys"),
			b.Div(mi.Class("flex items-start gap-3"), b.Span(mi.Class("material-icons"), "info"),
				b.Div(mi.Class("space-y-2"), b.P(t("developer.forms.label_keys")), b.Div(keys...),
					mi.If(!e.LabelKeysSeen, func(b *mi.Builder) mi.Node {
						return b.P(mi.Class("text-xs"), t("developer.forms.label_keys_guess"))
					})(b)))))
	}

	// the definition
	var def []interface{}
	if e.Problem == "" && e.Previewed {
		def = append(def, Alert(t("developer.forms.valid"), "success", false)(b))
	}
	if !data.Writable {
		// read only: the definition is shown, with its preview and its changes
		def = append(def, Alert(t("developer.forms.not_writable"), "warning", false)(b), b.Div(mi.Class("dev-code"), e.Text))
	} else {
		scope := developerScopeTabs(b, t, e.Scopes, e.Scope, func(scope string) string { return formsURL(e.Ref, scope) })
		if e.Shadow != "" {
			scope = append(scope, Alert(t(e.Shadow), "warning", false)(b))
		}
		parts = append(parts, b.Div(append([]interface{}{mi.Class(devCardClass + " p-4 space-y-3")}, scope...)...))

		schemaField := b.Input(mi.Type("hidden"), mi.Name("schema"), mi.Value(e.Ref.String()))
		scopeField := b.Input(mi.Type("hidden"), mi.Name("scope"), mi.Value(e.Scope))
		buttons := []interface{}{mi.Class("flex flex-wrap items-center gap-3"),
			IconButton("preview", t("developer.forms.preview"), "secondary", mi.Type("submit"), mi.Attr("formaction", "/developer/forms/preview"))(b),
			IconButton("save", t("developer.forms.save"), "primary", mi.Type("submit"))(b)}
		if e.HasOverride {
			buttons = append(buttons, IconButton("delete", t("developer.forms.remove"), "secondary", mi.Type("submit"), mi.Attr("form", "dev-form-remove"))(b))
		}
		def = append(def, b.Form(mi.Class("space-y-4 dev-json"), mi.Method("post"), mi.Action("/developer/forms/save"), schemaField, scopeField,
			e.Form(b), b.Div(buttons...)))
		if e.HasOverride {
			def = append(def, b.Form(mi.ID("dev-form-remove"), mi.Method("post"), mi.Action("/developer/forms/remove"),
				CSRFField(data.CSRFToken)(b), schemaField, scopeField))
		}
	}

	// the preview
	var view []interface{}
	switch {
	case e.PreviewErr != "":
		view = append(view, b.Div(mi.Class("dev-problem"), b.Div(mi.Class("dev-pre"), e.PreviewErr)))
	case e.PreviewHTML != "":
		view = append(view, b.P(mi.Class(devMutedClass), t("developer.forms.preview_note")),
			b.Div(mi.Class("dev-preview"), b.Form(mi.Attr("onsubmit", "return false"), mi.Attr("novalidate", "novalidate"), mi.Raw(e.PreviewHTML))))
	default:
		view = append(view, b.P(mi.Class(devMutedClass), t("developer.forms.preview_hint")))
	}

	// what it changes
	var changes []interface{}
	changed := 0
	if len(e.Warnings) > 0 {
		notes := []interface{}{mi.Class("dev-notes")}
		for _, w := range e.Warnings {
			notes = append(notes, b.Li(w))
		}
		changes = append(changes, b.Div(mi.Class("dev-warnings"),
			b.P(mi.Class("font-semibold"), t("developer.forms.warnings")), b.Ul(notes...)))
	}
	switch {
	case len(e.Diff) > 0:
		lines := []interface{}{mi.Class("dev-diff")}
		for _, l := range e.Diff {
			switch l.Op {
			case '+':
				changed++
				lines = append(lines, b.Div(mi.Class("dev-diff-line dev-diff-add"), "+ "+l.Text))
			case '-':
				changed++
				lines = append(lines, b.Div(mi.Class("dev-diff-line dev-diff-del"), "- "+l.Text))
			case '~':
				lines = append(lines, b.Div(mi.Class("dev-diff-line dev-diff-sep"), "… "+t("developer.forms.diff_unchanged", l.Text)))
			default:
				lines = append(lines, b.Div(mi.Class("dev-diff-line"), "  "+l.Text))
			}
		}
		changes = append(changes, b.P(mi.Class(devMutedClass), t("developer.forms.diff")), b.Div(lines...))
	case e.DiffTooLarge:
		changes = append(changes, b.P(mi.Class(devMutedClass), t("developer.forms.diff_too_large")))
	case e.Custom:
		changes = append(changes, b.P(mi.Class(devMutedClass), t("developer.forms.diff_custom")))
	default:
		changes = append(changes, b.P(mi.Class(devMutedClass), t("developer.forms.diff_none")))
	}

	tabs := []developerTab{
		{ID: "devtab-def", Label: t("developer.forms.tab.definition"), Body: def},
		{ID: "devtab-view", Label: t("developer.forms.preview_title"), Body: view},
		{ID: "devtab-changes", Label: t("developer.forms.tab.changes"), Count: changed, Body: changes},
	}
	if len(e.History) > 0 {
		tabs = append(tabs, developerTab{ID: "devtab-versions", Label: t("developer.forms.versions"), Count: len(e.History),
			Body: []interface{}{developerVersionsTable(b, data, t)}})
	}
	open := ""
	if e.Problem == "" && (e.PreviewHTML != "" || e.PreviewErr != "") {
		open = "devtab-view"
	}
	parts = append(parts, b.Div(mi.ID("edit"), developerTabs(b, tabs, open)))
	return developerFrame(b, head, data.DeveloperPageData, parts...)
}
