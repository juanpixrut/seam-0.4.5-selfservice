package ui

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/features"
	developerschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/developer"
	"github.com/ha1tch/seam-ui/internal/i18n"
	"github.com/ha1tch/seam-ui/internal/overrides"
	"github.com/ha1tch/seam-ui/internal/schemaoverlay"
)

// Creating what does not exist yet (T-118): a text under a new key, and a form
// under a new module/name. Both are overrides with nothing built in beneath
// them, written through the same validated writers as every other override,
// in the scope the developer chooses. A new text is created in the reference
// locale, so every other language falls back to it until it is translated with
// the editor; a new form starts from developerStarterForm and is then built in
// the designer or the JSON editor. Neither can replace something that exists:
// a key or a form that is already there is refused, and the editor is where it
// is changed. Until code asks for them (t("the.key"), resolveSchema(ctx,
// module, name, ...)) they are used by nothing; the export (developer_export.go)
// is how they reach the source tree.

// developerClaims is the caller's claims, for a route that changes something;
// it answers 403 itself when there are none or the module's runtime is absent.
func (h *UIHandler) developerClaims(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || h.config.DevRuntime == nil {
		h.Error403(w, r)
		return nil, false
	}
	return claims, true
}

// DeveloperTranslationCreate handles POST /developer/translations/create: form
// fields "key" and "value" (the text in the reference locale), "scope", and the
// list's "locale", "q" and "show" to return to.
func (h *UIHandler) DeveloperTranslationCreate(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.developerClaims(w, r)
	if !ok {
		return
	}
	rt := h.config.DevRuntime
	r.Body = http.MaxBytesReader(w, r.Body, developerMaxBody)
	if err := r.ParseForm(); err != nil {
		developerRedirect(w, r, "/developer/translations", "error", "bad_request")
		return
	}
	locale := r.FormValue("locale")
	if h.translator == nil || !h.translator.HasLocale(locale) {
		locale = developerRefLocale
	}
	list := translationsURL(locale, developerTruncate(r.FormValue("q"), developerMaxQuery), translationShow(r.FormValue("show")), 1, "")
	fail := func(code string) {
		http.Redirect(w, r, developerAppend(developerAppend(list, "new", "1"), "error", code), http.StatusSeeOther)
	}
	scopes := h.scopesWithGlobal(r.Context(), developerScopes(rt, rt.Locales != nil))
	if len(scopes) == 0 {
		fail("not_configured")
		return
	}
	if !developerScopeValid(r.FormValue("scope"), scopes) {
		fail("bad_scope")
		return
	}
	scope := developerScope(r.FormValue("scope"), scopes)
	key, value := strings.TrimSpace(r.FormValue("key")), r.FormValue("value")
	if !i18n.ValidNewKey(key) {
		fail("invalid_key")
		return
	}
	editor := developerAppend(translationsURL(locale, "", showAll, 1, key), "scope", scope)
	if h.translator != nil {
		_, inRef := h.translator.Effective(claims.TenantID, developerRefLocale)[key]
		_, inLoc := h.translator.Effective(claims.TenantID, locale)[key]
		if inRef || inLoc {
			http.Redirect(w, r, developerAppend(editor, "error", "key_exists")+"#edit", http.StatusSeeOther)
			return
		}
	}
	var err error
	if scopeStored(scope) {
		err = rt.Overrides.SetTranslation(r.Context(), scopeTenantID(scope, claims.TenantID), developerRefLocale, key, value, "", claims.UserID)
	} else {
		err = rt.Locales.SetOverrideAs(developerRefLocale, key, value, "", claims.UserID)
	}
	if err != nil {
		h.logger.Warn("developer: new text refused", "key", key, "scope", scope, "error", err)
		fail(developerErrorCode(err))
		return
	}
	h.auditLogger.LogDeveloperChange(r.Context(), r, claims.UserID, claims.TenantID, "translation", key,
		map[string]any{"locale": developerRefLocale, "action": "create", "scope": scope})
	// the other languages, where the form brought them: each is an override of
	// the new key in its own language, checked against the reference text. One
	// that is refused stops there, with the key created and its editor open in
	// that language.
	for _, other := range h.translationOtherLocales() {
		text := r.FormValue(translationNewField(other))
		if strings.TrimSpace(text) == "" {
			continue
		}
		if scopeStored(scope) {
			err = rt.Overrides.SetTranslation(r.Context(), scopeTenantID(scope, claims.TenantID), other, key, text, value, claims.UserID)
		} else {
			err = rt.Locales.SetOverrideAs(other, key, text, value, claims.UserID)
		}
		if err != nil {
			h.logger.Warn("developer: new text refused", "key", key, "locale", other, "scope", scope, "error", err)
			back := developerAppend(translationsURL(other, "", showAll, 1, key), "scope", scope)
			http.Redirect(w, r, developerAppend(back, "error", developerErrorCode(err))+"#edit", http.StatusSeeOther)
			return
		}
		h.auditLogger.LogDeveloperChange(r.Context(), r, claims.UserID, claims.TenantID, "translation", key,
			map[string]any{"locale": other, "action": "create", "scope": scope})
	}
	http.Redirect(w, r, developerAppend(editor, "msg", "created")+"#edit", http.StatusSeeOther)
}

// translationNewField is the name of the new-text form's field for a language
// other than the reference one.
func translationNewField(locale string) string { return "value_" + locale }

// translationOtherLocales are the languages a new text can be written in
// besides the reference one, sorted.
func (h *UIHandler) translationOtherLocales() []string {
	if h.translator == nil {
		return nil
	}
	var out []string
	for _, l := range h.translator.Locales() {
		if l != developerRefLocale {
			out = append(out, l)
		}
	}
	sort.Strings(out)
	return out
}

// translationNewTemplate is the field of the new-text definition that stands
// for "the text in another language": the form shows one copy of it per
// language besides the reference one, named translationNewField(locale).
const translationNewTemplate = "value_locale"

// translationNewSchema is the new-text form: the embedded definition (or its
// override) with its language field repeated for every language besides the
// reference one, so a text can be written in every language when it is
// created instead of being left to fall back. A definition without the
// template field asks for the reference text only.
func (h *UIHandler) translationNewSchema(ctx context.Context, t func(key string, args ...any) string) []byte {
	doc := resolveSchema(ctx, "developer", "translation_new", developerschemas.Schema("translation_new"))
	var m map[string]any
	if json.Unmarshal(doc, &m) != nil {
		return doc
	}
	props, ok := m["properties"].(map[string]any)
	if !ok {
		return doc
	}
	template, ok := props[translationNewTemplate].(map[string]any)
	delete(props, translationNewTemplate)
	if ok {
		for _, l := range h.translationOtherLocales() {
			field := make(map[string]any, len(template)+1)
			for k, v := range template {
				field[k] = v
			}
			field["title"] = t("developer.translations.new.text_in", strings.ToUpper(l))
			props[translationNewField(l)] = field
		}
	}
	out, err := json.Marshal(m)
	if err != nil {
		return doc
	}
	// what is rendered is not the stored definition byte for byte: it is noted
	// under the form's name so its page identity is learnt like any other
	developerTrace.noteDoc("developer", "translation_new", out)
	return out
}

// DeveloperTranslationRestore handles POST /developer/translations/restore: an
// older version of the override of a text becomes current again, as a new
// version, in the scope it belongs to (the tenant's history, or the one the
// tools keep for the files). The scope must be named: a restore is never
// guessed. The old wording is checked again against the text it replaces.
func (h *UIHandler) DeveloperTranslationRestore(w http.ResponseWriter, r *http.Request) {
	claims, locale, key, back, scope, ok := h.translationsPost(w, r)
	if !ok {
		return
	}
	if r.FormValue("scope") != scope {
		http.Redirect(w, r, developerAppend(back, "error", "bad_scope"), http.StatusSeeOther)
		return
	}
	version, err := strconv.Atoi(r.FormValue("version"))
	if err != nil || version < 1 {
		http.Redirect(w, r, developerAppend(back, "error", "unknown_version")+"#edit", http.StatusSeeOther)
		return
	}
	rt := h.config.DevRuntime
	reference := h.translationReference(claims.TenantID, locale, key)
	var v overrides.TranslationVersion
	if scopeStored(scope) {
		v, err = rt.Overrides.RestoreTranslation(r.Context(), scopeTenantID(scope, claims.TenantID), locale, key, version, reference, claims.UserID)
	} else {
		var old *overrides.TranslationVersion
		history := fileTextHistory(rt.Locales, locale, key)
		for i := range history {
			if history[i].Version == version {
				old = &history[i]
			}
		}
		switch {
		case old == nil:
			err = overrides.ErrUnknownVersion
		case old.Removed:
			err = overrides.ErrNotRestorable
		default:
			err = rt.Locales.SetOverrideAs(locale, key, old.Value, reference, claims.UserID)
		}
	}
	if err != nil {
		h.logger.Warn("developer: translation override not restored", "locale", locale, "key", key, "version", version, "error", err)
		http.Redirect(w, r, developerAppend(back, "error", developerErrorCode(err))+"#edit", http.StatusSeeOther)
		return
	}
	h.auditLogger.LogDeveloperChange(r.Context(), r, claims.UserID, claims.TenantID, "translation", key,
		map[string]any{"locale": locale, "action": "restore", "scope": scope, "from": version, "version": v.Version})
	http.Redirect(w, r, developerAppend(back, "msg", "restored")+"#edit", http.StatusSeeOther)
}

// DeveloperFormCreate handles POST /developer/forms/create: form fields
// "module", "name" and "scope". The new form opens in the designer when it is
// on, otherwise in the JSON editor.
func (h *UIHandler) DeveloperFormCreate(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.developerClaims(w, r)
	if !ok {
		return
	}
	rt := h.config.DevRuntime
	r.Body = http.MaxBytesReader(w, r.Body, developerMaxBody)
	if err := r.ParseForm(); err != nil {
		developerRedirect(w, r, "/developer/forms", "error", "bad_request")
		return
	}
	fail := func(code string) { developerRedirect(w, r, "/developer/forms?new=1", "error", code) }
	if rt.Resolver == nil {
		fail("not_configured")
		return
	}
	scopes := h.scopesWithGlobal(r.Context(), developerScopes(rt, rt.Schemas != nil))
	if len(scopes) == 0 {
		fail("not_configured")
		return
	}
	if !developerScopeValid(r.FormValue("scope"), scopes) {
		fail("bad_scope")
		return
	}
	scope := developerScope(r.FormValue("scope"), scopes)
	ref := developerSchemaRef{strings.TrimSpace(r.FormValue("module")), strings.TrimSpace(r.FormValue("name"))}
	if !developerNameRE.MatchString(ref.Module) || !developerNameRE.MatchString(ref.Name) || len(ref.Module) > 64 || len(ref.Name) > 64 {
		fail("invalid_name")
		return
	}
	if _, _, _, exists := h.developerForm(r.Context(), claims.TenantID, ref.String()); exists {
		http.Redirect(w, r, developerAppend(formsURL(ref, ""), "error", "form_exists"), http.StatusSeeOther)
		return
	}
	doc := []byte(developerStarterForm)
	var err error
	details := map[string]any{"action": "create", "scope": scope}
	if scopeStored(scope) {
		var v overrides.SchemaVersion
		v, _, err = rt.Overrides.SaveSchema(r.Context(), scopeTenantID(scope, claims.TenantID), ref.Module, ref.Name, doc, doc, claims.UserID, "created")
		details["version"] = v.Version
	} else {
		_, err = rt.Schemas.SaveAs(ref.Module, ref.Name, doc, nil, claims.UserID, "created")
	}
	if err != nil {
		h.logger.Warn("developer: new form refused", "form", ref.String(), "scope", scope, "error", err)
		code := developerErrorCode(err)
		if errors.Is(err, schemaoverlay.ErrInvalidName) {
			code = "invalid_name"
		}
		fail(code)
		return
	}
	h.auditLogger.LogDeveloperChange(r.Context(), r, claims.UserID, claims.TenantID, "form_definition", ref.String(), details)
	next := formsURL(ref, scope)
	if h.config.flagOn(features.DeveloperFormDesigner) {
		next = designerURL(ref, scope)
	}
	http.Redirect(w, r, developerAppend(next, "msg", "created"), http.StatusSeeOther)
}
