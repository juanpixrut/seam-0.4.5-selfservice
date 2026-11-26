package ui

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	fe "github.com/ha1tch/seam-ui/internal/formengine"
	"github.com/ha1tch/seam-ui/internal/overrides"
	"github.com/ha1tch/seam-ui/internal/schemaoverlay"
)

const (
	gateUnknownWidget = `{"type":"object","properties":{"alpha":{"type":"string","title":"A","x-seam-input":"text"},"zz":{"type":"string","title":"Z","x-seam-input":"nope"}}}`
	gateUnknownSource = `{"type":"object","properties":{"alpha":{"type":"string","title":"A","x-seam-input":"text"},"zz":{"type":"integer","title":"Z","x-seam-input":"select","x-seam-source":"entity:nothing_here"}}}`
	gateNoFields      = `{"type":"object","properties":{}}`
	gateNested        = `{"type":"object","properties":{"lines":{"type":"array","title":"L","x-seam-input":"repeatable","items":{"type":"object","properties":{"zz":{"type":"string","title":"Z","x-seam-input":"nope"}}}}}}`
)

// Every form of the catalogue passes the gate against itself: the gate must
// never refuse what ships, or the designer could not touch a built-in form.
func TestGate_EveryBuiltInFormPasses(t *testing.T) {
	for _, ref := range developerEmbeddedSchemas() {
		base, _ := developerEmbeddedSchema(ref.Module, ref.Name)
		if err := developerFormCheck(ref, base, false, base); err != nil {
			t.Errorf("%s: %v", ref, err)
		}
	}
	if err := developerFormCheck(developerSchemaRef{"inspections", "x"}, []byte(developerStarterForm), true, []byte(developerStarterForm)); err != nil {
		t.Errorf("the starter of a new form: %v", err)
	}
}

// A definition the engine can parse but not render is refused, with the
// reason, by every way the tools write one (T-115).
func TestGate_UnrenderableDefinitionsAreRefusedEverywhere(t *testing.T) {
	h, store := designerHarness(t)
	for _, scope := range []string{"file", "tenant"} {
		for _, c := range []struct{ name, doc, want string }{
			{"unknown widget", gateUnknownWidget, `no widget &#34;nope&#34;`},
			{"unknown source", gateUnknownSource, `entity:nothing_here`},
			{"no fields", gateNoFields, `has no fields`},
			{"unknown widget inside a repeatable", gateNested, `lines.zz`},
		} {
			rec := formSave(h, t, scope, c.doc, "")
			if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), c.want) {
				t.Errorf("%s, %s: status %d, want 422 naming %q", scope, c.name, rec.Code, c.want)
			}
			if !strings.Contains(rec.Body.String(), "nope") && !strings.Contains(rec.Body.String(), "nothing_here") && !strings.Contains(rec.Body.String(), "properties") {
				t.Errorf("%s, %s: what was typed must be kept in the editor", scope, c.name)
			}
		}
	}
	if fileCount(t, h.schDir) != 0 {
		t.Errorf("a refusal writes no file")
	}
	if cur, _ := store.CurrentSchemas(ctx_, 1); len(cur) != 0 {
		t.Errorf("a refusal stores nothing: %d", len(cur))
	}
	// a preview says the same, and renders nothing
	rec := h.post(t, "/developer/forms/preview", url.Values{"schema": {"system/user_create"}, "scope": {"file"}, "json": {gateUnknownWidget}})
	if !strings.Contains(rec.Body.String(), "no widget") || strings.Contains(rec.Body.String(), "dev-preview") {
		t.Errorf("a preview of an unrenderable definition shows the problem, not a form")
	}
	// the designer refuses the action that would leave one: the last field cannot go
	if loc := location(h.post(t, "/developer/forms/create", url.Values{"module": {"inspections"}, "name": {"one"}, "scope": {"file"}})); !strings.Contains(loc, "msg=created") {
		t.Fatal(loc)
	}
	s := openDesigner(t, h, "inspections/one", "file")
	before := s.doc
	if s.do("remove:name", nil); s.doc != before || !strings.Contains(s.body, "has no fields") {
		t.Errorf("removing the last field must be refused with the reason")
	}
	// a source the form's page resolves is accepted: the asset form may use the application's sources
	base, _ := developerEmbeddedSchema("asset", "asset")
	withSource := strings.Replace(string(base), `"properties": {`, `"properties": {"zz_region":{"type":"integer","title":"Z","x-seam-input":"select","x-seam-source":"entity:regions"},`, 1)
	if err := developerFormCheck(developerSchemaRef{"asset", "asset"}, base, false, []byte(withSource)); err != nil {
		t.Errorf("the asset form may use a source of the application's resolver: %v", err)
	}
	// while a form whose page has no resolver may not gain one
	ub, _ := developerEmbeddedSchema("system", "user_create")
	if err := developerFormCheck(developerSchemaRef{"system", "user_create"}, ub, false, []byte(strings.Replace(string(ub), `"properties": {`, `"properties": {"zz_region":{"type":"integer","title":"Z","x-seam-input":"select","x-seam-source":"entity:regions"},`, 1))); err == nil {
		t.Errorf("a form whose page resolves no source must not gain a sourced select")
	}
}

// An override written by hand that the tools would have refused is flagged in
// the list and in its editor instead of trusted; a version saved under the
// older rule cannot be restored.
func TestGate_AnOverrideAlreadyInForceIsFlagged(t *testing.T) {
	h, store := scopedHarness(t)
	if err := os.MkdirAll(filepath.Join(h.schDir, "system"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.schDir, "system", "user_create.json"), []byte(gateUnknownWidget), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := h.rt.Schemas.Reload(); err != nil {
		t.Fatalf("the loader keeps its own, older rule: %v", err)
	}
	list := h.get(t, "/developer/forms?q=user_").Body.String()
	if strings.Count(list, ">Cannot be rendered<") != 1 || !strings.Contains(list, "no widget") {
		t.Errorf("the list flags the one unrenderable override, with the reason")
	}
	if ed := formView(h, t, "file"); !strings.Contains(ed, "cannot be rendered") || !strings.Contains(ed, "no widget") {
		t.Errorf("its editor says why")
	}
	// a tenant version stored before the gate existed is not restorable
	if _, err := store.AppendSchema(ctx_, overrides.SchemaVersion{Tenant: 1, Module: "system", Name: "user_edit", Doc: []byte(gateUnknownWidget)}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendSchema(ctx_, overrides.SchemaVersion{Tenant: 1, Module: "system", Name: "user_edit", Removed: true}); err != nil {
		t.Fatal(err)
	}
	loc := location(h.post(t, "/developer/forms/restore", url.Values{"schema": {"system/user_edit"}, "scope": {"tenant"}, "version": {"1"}}))
	if !strings.Contains(loc, "error=invalid_json") {
		t.Errorf("an unrenderable version must not be restored: %s", loc)
	}
	if hist, _ := store.SchemaHistory(ctx_, 1, "system", "user_edit"); len(hist) != 2 {
		t.Errorf("the refused restore adds no version: %d", len(hist))
	}
}

// The file scope keeps what the tools did to a form, and an earlier version
// can be restored there too.
func TestForms_FileScopeHistoryAndRestore(t *testing.T) {
	h, _ := scopedHarness(t)
	second := strings.Replace(e2eOverride, "Alpha OVERRIDE", "Second version", 1)
	formSave(h, t, "file", e2eOverride, "first")
	formSave(h, t, "file", second, "second")
	fv := formView(h, t, "file")
	if strings.Count(fv, `action="/developer/forms/restore"`) != 1 || !strings.Contains(fv, ">first<") || !strings.Contains(fv, ">Current<") {
		t.Fatalf("two versions, the newer one live: %d restore forms", strings.Count(fv, `action="/developer/forms/restore"`))
	}
	if strings.Contains(formView(h, t, "tenant"), `action="/developer/forms/restore"`) {
		t.Errorf("the tenant scope has no versions of its own here")
	}
	restore := func(scope, version string) string {
		return location(h.post(t, "/developer/forms/restore", url.Values{"schema": {"system/user_create"}, "scope": {scope}, "version": {version}}))
	}
	if loc := restore("file", "1"); !strings.Contains(loc, "msg=restored") || !strings.Contains(loc, "scope=file") {
		t.Fatalf("restore: %s", loc)
	}
	if b, _ := os.ReadFile(filepath.Join(h.schDir, "system", "user_create.json")); !strings.Contains(string(b), "Alpha OVERRIDE") {
		t.Errorf("the restored definition is the file again")
	}
	hist, _ := h.rt.Schemas.History("system", "user_create")
	if len(hist) != 3 || hist[0].Note != "restored from version 1" || hist[0].ChangedBy == 0 {
		t.Errorf("a restore is a new version, with who: %+v", hist[0])
	}
	// a removal is a version, and what was there can come back
	if loc := location(h.post(t, "/developer/forms/remove", url.Values{"schema": {"system/user_create"}, "scope": {"file"}})); !strings.Contains(loc, "msg=removed") {
		t.Fatal(loc)
	}
	if fv := formView(h, t, "file"); !strings.Contains(fv, ">Removed<") || strings.Count(fv, `action="/developer/forms/restore"`) != 3 {
		t.Errorf("after a removal every version with a definition can be restored: %d", strings.Count(fv, `action="/developer/forms/restore"`))
	}
	for _, c := range [][2]string{{"file", "4"}, {"file", "99"}, {"file", "0"}, {"file", "x"}, {"tenant", "1"}, {"", "1"}, {"everyone", "1"}} {
		if loc := restore(c[0], c[1]); !strings.Contains(loc, "error=") {
			t.Errorf("restore %v must be refused: %s", c, loc)
		}
	}
	// a file changed by hand since is not shown as the live version
	if loc := restore("file", "2"); !strings.Contains(loc, "msg=restored") {
		t.Fatal(loc)
	}
	if err := os.WriteFile(filepath.Join(h.schDir, "system", "user_create.json"), []byte(strings.Replace(e2eOverride, "Alpha OVERRIDE", "By hand", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(formView(h, t, "file"), ">Current<") {
		t.Errorf("no recorded version is the live one while a hand change has not been loaded")
	}
	// once loaded, the hand change is a version of its own, and the live one
	_ = h.rt.Schemas.Reload()
	hist, _ = h.rt.Schemas.History("system", "user_create")
	if hist[0].ChangedBy != schemaoverlay.ChangedOutside || !strings.Contains(hist[0].Doc, "By hand") {
		t.Fatalf("a change made in the directory is recorded when it is loaded: %+v", hist[0])
	}
	if fv := formView(h, t, "file"); strings.Count(fv, ">Current<") != 1 || !strings.Contains(fv, "changed in the files") {
		t.Errorf("the versions list names a change made in the files, as the current one")
	}
}

// The file scope keeps what the tools did to a text too.
func TestTranslations_FileScopeHistoryAndRestore(t *testing.T) {
	h, _ := scopedHarness(t)
	view := func() string {
		return h.get(t, "/developer/translations?locale=en&scope=file&key="+trKey).Body.String()
	}
	for _, text := range []string{"First file wording", "Second file wording"} {
		if loc := trSave(h, t, "file", trKey, text); !strings.Contains(loc, "msg=saved") {
			t.Fatal(loc)
		}
	}
	if v := view(); strings.Count(v, `action="/developer/translations/restore"`) != 1 || !strings.Contains(v, "First file wording") || !strings.Contains(v, ">Current<") {
		t.Fatalf("two versions, the newer one live")
	}
	restore := func(scope, version string) string {
		return location(h.post(t, "/developer/translations/restore", url.Values{"locale": {"en"}, "key": {trKey}, "scope": {scope}, "version": {version}}))
	}
	if loc := restore("file", "1"); !strings.Contains(loc, "msg=restored") {
		t.Fatalf("restore: %s", loc)
	}
	if got := h.tr.T("en", trKey); got != "First file wording" {
		t.Errorf("the restored wording is live for everyone, got %q", got)
	}
	if hist, _ := h.rt.Locales.History("en", trKey); len(hist) != 3 || hist[0].ChangedBy == 0 {
		t.Errorf("a restore is a new version, with who: %+v", hist)
	}
	for _, c := range [][2]string{{"file", "99"}, {"file", "0"}, {"tenant", "1"}, {"", "1"}} {
		if loc := restore(c[0], c[1]); !strings.Contains(loc, "error=") {
			t.Errorf("restore %v must be refused: %s", c, loc)
		}
	}
}

// A tenant's text overridden before the history existed shows its wording as
// its one version, and the next change keeps it restorable.
func TestTranslations_HistoryStartsFromWhatIsThere(t *testing.T) {
	h, store := scopedHarness(t)
	if err := store.SetTranslation(ctx_, overrides.Translation{Tenant: 1, Locale: "en", Key: trKey, Value: "Older wording", ChangedBy: 4}); err != nil {
		t.Fatal(err)
	}
	if hist, _ := h.rt.Overrides.TranslationHistory(ctx_, 1, "en", trKey); len(hist) != 1 || hist[0].Value != "Older wording" || hist[0].Version != 1 {
		t.Fatalf("what is there is shown as version 1: %+v", hist)
	}
	if loc := trSave(h, t, "tenant", trKey, "Newer wording"); !strings.Contains(loc, "msg=saved") {
		t.Fatal(loc)
	}
	hist, _ := h.rt.Overrides.TranslationHistory(ctx_, 1, "en", trKey)
	if len(hist) != 2 || hist[1].Value != "Older wording" || hist[1].ChangedBy != 4 || hist[0].Value != "Newer wording" {
		t.Fatalf("the older wording is recorded before the change: %+v", hist)
	}
	if loc := location(h.post(t, "/developer/translations/restore", url.Values{"locale": {"en"}, "key": {trKey}, "scope": {"tenant"}, "version": {"1"}})); !strings.Contains(loc, "msg=restored") {
		t.Errorf("and can be restored: %s", loc)
	}
	if got := h.tr.ForTenant(1).T("en", trKey); got != "Older wording" {
		t.Errorf("got %q", got)
	}
}

// The editors say when a field's label comes from a translation key, since a
// title in the definition then changes nothing.
func TestForms_LabelsThatComeFromATranslationKeyAreNamed(t *testing.T) {
	h, _ := designerHarness(t)
	// the Developer module's own form: developer.forms.view_as.field.role exists
	ed := h.get(t, "/developer/forms?schema=developer/view_as").Body.String()
	if !strings.Contains(ed, "comes from a translation key") || !strings.Contains(ed, "key=developer.forms.view_as.field.role") {
		t.Errorf("the editor names the field and links to its text")
	}
	// a form's own keys are found, and for a form that has none, the general field keys
	found := map[string]string{}
	for _, lk := range h.handlerLabelKeys(t, "masters", "marca") {
		found[lk.Field] = lk.Key
	}
	if found["code"] != "masters.forms.marca.field.code" || found["name"] != "masters.forms.marca.field.name" {
		t.Errorf("masters/marca's labels come from its own keys, got %v", found)
	}
	found = map[string]string{}
	for _, lk := range h.handlerLabelKeys(t, "system", "user_create") {
		found[lk.Field] = lk.Key
	}
	if found["email"] != "field.email" {
		t.Errorf("system/user_create's email label comes from the general key, got %q", found["email"])
	}
	for field, key := range found {
		if h.tr.T("en", key) == key {
			t.Errorf("%s: %s is not a text", field, key)
		}
	}
	// a field no key covers is not listed
	doc := `{"type":"object","properties":{"zz_unknown_field_name":{"type":"string","title":"Z","x-seam-input":"text"}}}`
	for _, lk := range h.labelKeysOf(t, developerSchemaRef{"system", "user_create"}, []byte(doc)) {
		if lk.Field == "zz_unknown_field_name" {
			t.Errorf("a field with no key must not be listed: %+v", lk)
		}
	}
	// the designer's inspector says it on the selected field
	s := openDesigner(t, h, "developer/view_as", "file").do("select:role", nil)
	if !strings.Contains(s.body, "developer.forms.view_as.field.role") || !strings.Contains(s.body, "instead") {
		t.Errorf("the inspector names the key that supplies the label")
	}
}

// One text, every language: the editor shows which have their own text.
func TestTranslations_EveryLanguageOfATextIsShown(t *testing.T) {
	h, _ := scopedHarness(t)
	if loc := location(h.post(t, "/developer/translations/create", url.Values{"key": {"orders.list.title"}, "value": {"Orders"}, "scope": {"tenant"}})); !strings.Contains(loc, "msg=created") {
		t.Fatal(loc)
	}
	states := map[string]string{}
	for _, ls := range h.localeStates(t, "orders.list.title") {
		states[ls.Locale] = ls.State
	}
	if states["en"] != textTranslated || states["es"] != textMissing || states["ja"] != textMissing || states["pt"] != textMissing {
		t.Fatalf("a new text exists in the reference language only: %v", states)
	}
	body := h.get(t, "/developer/translations?locale=en&key=orders.list.title").Body.String()
	if strings.Count(body, `class="dev-lang"`) != 4 || !strings.Contains(body, "locale=es") || !strings.Contains(body, "No text: the reference text is shown") {
		t.Errorf("the editor links to each language and says which lack the text: %d links", strings.Count(body, `class="dev-lang"`))
	}
	if loc := location(h.post(t, "/developer/translations/save", url.Values{"locale": {"es"}, "key": {"orders.list.title"}, "value": {"Pedidos"}, "scope": {"tenant"}})); !strings.Contains(loc, "msg=saved") {
		t.Fatal(loc)
	}
	if loc := location(h.post(t, "/developer/translations/save", url.Values{"locale": {"pt"}, "key": {"orders.list.title"}, "value": {"Orders"}, "scope": {"tenant"}})); !strings.Contains(loc, "msg=saved") {
		t.Fatal(loc)
	}
	states = map[string]string{}
	for _, ls := range h.localeStates(t, "orders.list.title") {
		states[ls.Locale] = ls.State
	}
	if states["es"] != textTranslated || states["pt"] != textUntranslated || states["ja"] != textMissing {
		t.Errorf("translated, repeated and missing are told apart: %v", states)
	}
}

// Without a directory or tenant overrides the editor is read only, and still
// shows the definition.
func TestForms_ReadOnlyEditorShowsTheDefinition(t *testing.T) {
	h := newDevHarness(t, true, false)
	body := h.get(t, "/developer/forms?schema=system/user_create").Body.String()
	if strings.Contains(body, "/developer/forms/save") || !strings.Contains(body, `class="dev-code"`) || !strings.Contains(body, "&#34;email&#34;") {
		t.Errorf("the read-only editor shows the definition as text, with no way to save")
	}
}

// The two forms that used to be written inside their handlers are definitions
// like the others, and no other handler holds a fixed one.
func TestCatalogue_NoFormDefinitionIsWrittenInAHandler(t *testing.T) {
	for _, ref := range []developerSchemaRef{{"issues", "assign"}, {"outsourced", "account"}} {
		doc, ok := developerEmbeddedSchema(ref.Module, ref.Name)
		if !ok {
			t.Errorf("%s is not in the catalogue", ref)
			continue
		}
		if _, err := fe.ParseSchema(doc); err != nil {
			t.Errorf("%s: %v", ref, err)
		}
	}
	// These build a definition from data (a template's or a type's own fields),
	// which is not a form to override; the catalogue's starter is the one constant.
	generated := map[string]bool{"issue_templates.go": true, "workorders.go": true, "workorders_templates_helper.go": true, "developer_catalog.go": true}
	inline := regexp.MustCompile(`x-seam-input`)
	for name, src := range productionGoFiles(t, ".") {
		if generated[name] || strings.HasPrefix(name, "demo") {
			continue
		}
		for i, line := range strings.Split(src, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") || !inline.MatchString(line) {
				continue
			}
			t.Errorf("%s:%d builds a form definition in code; put it in internal/formengine/schemas so it can be overridden: %s", name, i+1, strings.TrimSpace(line))
		}
	}
}

func (h *devHarness) labelKeysOf(t *testing.T, ref developerSchemaRef, doc []byte) []formLabelKey {
	t.Helper()
	return h.ui.developerLabelKeys(1, ref, doc)
}

func (h *devHarness) handlerLabelKeys(t *testing.T, module, name string) []formLabelKey {
	t.Helper()
	doc, ok := developerEmbeddedSchema(module, name)
	if !ok {
		t.Fatalf("%s/%s is not in the catalogue", module, name)
	}
	return h.labelKeysOf(t, developerSchemaRef{module, name}, doc)
}

func (h *devHarness) localeStates(t *testing.T, key string) []translationLocaleState {
	t.Helper()
	return h.ui.translationLocaleStates(1, key)
}
