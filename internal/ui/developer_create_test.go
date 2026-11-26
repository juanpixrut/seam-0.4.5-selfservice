package ui

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/seam-ui/internal/i18n"
)

func zipFiles(t *testing.T, data []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("not a zip: %v", err)
	}
	out := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(rc)
		rc.Close()
		out[f.Name] = string(b)
	}
	return out
}

// A text under a new key can be created, in either scope, and is then a text
// like any other: listed, editable, translatable, removable.
func TestTranslations_CreateANewKey(t *testing.T) {
	h, _ := scopedHarness(t)
	const key = "orders.list.title"
	create := func(v url.Values) string {
		return location(h.post(t, "/developer/translations/create", v))
	}
	if body := h.get(t, "/developer/translations?new=1").Body.String(); !strings.Contains(body, `action="/developer/translations/create"`) || !strings.Contains(body, `name="key"`) {
		t.Fatalf("the new-text panel must offer its form")
	}
	if body := h.get(t, "/developer/translations").Body.String(); strings.Contains(body, `action="/developer/translations/create"`) || !strings.Contains(body, "new=1") {
		t.Errorf("the panel is shown on request only; the header links to it")
	}
	loc := create(url.Values{"key": {key}, "value": {"Orders of %s"}, "scope": {"tenant"}, "locale": {"es"}})
	if !strings.Contains(loc, "msg=created") || !strings.Contains(loc, "key="+key) || !strings.Contains(loc, "locale=es") {
		t.Fatalf("create: %s", loc)
	}
	if got := h.tr.ForTenant(1).T("en", key, "May"); got != "Orders of May" {
		t.Errorf("the new text is live for the tenant in the reference locale, got %q", got)
	}
	if got := h.tr.ForTenant(1).T("es", key, "mayo"); got != "Orders of mayo" {
		t.Errorf("other languages fall back to it, got %q", got)
	}
	if got := h.tr.ForTenant(2).T("en", key, "May"); got == "Orders of May" {
		t.Errorf("a tenant-scope text must not reach another tenant")
	}
	if fileCount(t, h.locDir) != 0 {
		t.Errorf("a tenant-scope text is not written to the directory")
	}
	// it is now a text like any other: the editor opens it and a translation keeps its placeholders
	if body := h.get(t, "/developer/translations?locale=es&key="+key).Body.String(); !strings.Contains(body, "/developer/translations/save") || !strings.Contains(body, "Orders of %s") {
		t.Errorf("the editor must open the new key")
	}
	es := func(value string) string {
		return location(h.post(t, "/developer/translations/save", url.Values{"locale": {"es"}, "key": {key}, "value": {value}, "scope": {"tenant"}}))
	}
	if loc := es("sin marcador"); !strings.Contains(loc, "error=verbs") {
		t.Errorf("a translation of the new text keeps its placeholders: %s", loc)
	}
	if loc := es("Pedidos de %s"); !strings.Contains(loc, "msg=saved") || h.tr.ForTenant(1).T("es", key, "mayo") != "Pedidos de mayo" {
		t.Errorf("the new text can be translated: %s", loc)
	}
	// creating it again is refused and leads to its editor
	if loc := create(url.Values{"key": {key}, "value": {"Other"}, "scope": {"file"}}); !strings.Contains(loc, "error=key_exists") || !strings.Contains(loc, "key="+key) {
		t.Errorf("an existing key: %s", loc)
	}
	// a built-in key cannot be created either
	if loc := create(url.Values{"key": {"developer.nav.forms"}, "value": {"X"}, "scope": {"tenant"}}); !strings.Contains(loc, "error=key_exists") {
		t.Errorf("a built-in key: %s", loc)
	}
	// file scope writes the reference locale's file
	if loc := create(url.Values{"key": {"orders.list.empty"}, "value": {"No orders"}, "scope": {"file"}}); !strings.Contains(loc, "msg=created") {
		t.Fatalf("create in files: %s", loc)
	}
	if b, _ := os.ReadFile(filepath.Join(h.locDir, "en.json")); !strings.Contains(string(b), `"orders.list.empty": "No orders"`) {
		t.Errorf("the file scope writes en.json: %s", b)
	}
	// refusals store nothing
	before, _ := h.rt.Overrides.Translations(ctx_, 1)
	for _, c := range []struct {
		v    url.Values
		want string
	}{
		{url.Values{"key": {"NoDots"}, "value": {"x"}, "scope": {"tenant"}}, "error=invalid_key"},
		{url.Values{"key": {"Upper.Case"}, "value": {"x"}, "scope": {"tenant"}}, "error=invalid_key"},
		{url.Values{"key": {"../../etc.passwd"}, "value": {"x"}, "scope": {"file"}}, "error=invalid_key"},
		{url.Values{"key": {"a.b\"><script>"}, "value": {"x"}, "scope": {"tenant"}}, "error=invalid_key"},
		{url.Values{"key": {"a." + strings.Repeat("b", 300)}, "value": {"x"}, "scope": {"tenant"}}, "error=invalid_key"},
		{url.Values{"key": {"orders.new.one"}, "value": {"  "}, "scope": {"tenant"}}, "error=invalid_text"},
		{url.Values{"key": {"orders.new.two"}, "value": {"x"}, "scope": {"everyone"}}, "error=bad_scope"},
	} {
		if loc := create(c.v); !strings.Contains(loc, c.want) || !strings.Contains(loc, "new=1") {
			t.Errorf("%v: %s, want %s back on the panel", c.v["key"], loc, c.want)
		}
	}
	if after, _ := h.rt.Overrides.Translations(ctx_, 1); len(after) != len(before) {
		t.Errorf("a refusal stored something: %d -> %d", len(before), len(after))
	}
	// another role cannot create
	if rec := h.do(t, "POST", "/developer/translations/create", "admin", url.Values{"key": {"x.y"}, "value": {"x"}, "scope": {"tenant"}}); rec.Code != http.StatusForbidden {
		t.Errorf("an admin must get 403, got %d", rec.Code)
	}
}

// The tenant's versions of a text are kept and an older wording can be restored.
func TestTranslations_HistoryAndRestore(t *testing.T) {
	h, _ := scopedHarness(t)
	key := trKey
	view := func() string {
		return h.get(t, "/developer/translations?locale=en&scope=tenant&key="+key).Body.String()
	}
	if strings.Contains(view(), "/developer/translations/restore") {
		t.Fatalf("a text with no history offers nothing to restore")
	}
	for _, text := range []string{"First wording", "Second wording"} {
		if loc := trSave(h, t, "tenant", key, text); !strings.Contains(loc, "msg=saved") {
			t.Fatalf("save: %s", loc)
		}
	}
	v := view()
	if strings.Count(v, `action="/developer/translations/restore"`) != 1 || !strings.Contains(v, "First wording") || !strings.Contains(v, ">Current<") {
		t.Errorf("the live version is marked and only the older one can be restored: %d restore forms", strings.Count(v, `action="/developer/translations/restore"`))
	}
	restore := func(scope, version string) string {
		return location(h.post(t, "/developer/translations/restore", url.Values{"locale": {"en"}, "key": {key}, "scope": {scope}, "version": {version}}))
	}
	if loc := restore("tenant", "1"); !strings.Contains(loc, "msg=restored") {
		t.Fatalf("restore: %s", loc)
	}
	if got := h.tr.ForTenant(1).T("en", key); got != "First wording" {
		t.Errorf("the restored wording is live, got %q", got)
	}
	hist, _ := h.rt.Overrides.TranslationHistory(ctx_, 1, "en", key)
	if len(hist) != 3 || hist[0].Value != "First wording" || hist[0].Version != 3 {
		t.Errorf("a restore is a new version: %+v", hist)
	}
	// a removal is a version too, and what was there can come back
	if !strings.Contains(trRemove(h, t, "tenant", key), "msg=removed") {
		t.Fatal("remove")
	}
	if v := view(); !strings.Contains(v, ">Removed<") || strings.Count(v, `action="/developer/translations/restore"`) != 3 {
		t.Errorf("after a removal every version with a text can be restored: %d", strings.Count(v, `action="/developer/translations/restore"`))
	}
	for _, c := range [][2]string{{"tenant", "4"}, {"tenant", "99"}, {"tenant", "0"}, {"tenant", "x"}, {"file", "1"}, {"everyone", "1"}} {
		if loc := restore(c[0], c[1]); !strings.Contains(loc, "error=") {
			t.Errorf("restore %v must be refused: %s", c, loc)
		}
	}
	if loc := restore("tenant", "4"); !strings.Contains(loc, "error=not_restorable") {
		t.Errorf("a removal has nothing to restore: %s", loc)
	}
	// the file scope keeps no history
	if loc := trSave(h, t, "file", key, "From a file"); !strings.Contains(loc, "msg=saved") {
		t.Fatal(loc)
	}
	if fv := h.get(t, "/developer/translations?locale=en&scope=file&key="+key).Body.String(); strings.Contains(fv, "/developer/translations/restore") {
		t.Errorf("the file scope has no versions")
	}
}

// A form under a new module/name can be created, opened, designed, previewed,
// saved, removed and restored like a built-in one; it has no built-in
// definition, so there is no diff and no compatibility note.
func TestForms_CreateANewForm(t *testing.T) {
	h, _ := designerHarness(t)
	create := func(v url.Values) string { return location(h.post(t, "/developer/forms/create", v)) }
	if body := h.get(t, "/developer/forms?new=1").Body.String(); !strings.Contains(body, `action="/developer/forms/create"`) || !strings.Contains(body, `name="module"`) {
		t.Fatalf("the new-form panel must offer its form")
	}
	loc := create(url.Values{"module": {"inspections"}, "name": {"checklist"}, "scope": {"file"}})
	if !strings.Contains(loc, "msg=created") || !strings.HasPrefix(loc, "/developer/designer?schema=inspections%2Fchecklist") {
		t.Fatalf("create opens the designer: %s", loc)
	}
	if b, err := os.ReadFile(filepath.Join(h.schDir, "inspections", "checklist.json")); err != nil || !strings.Contains(string(b), `"name"`) {
		t.Fatalf("the starter definition is written: %v", err)
	}
	// the list, the editor and the designer know it
	list := h.get(t, "/developer/forms?q=checklist").Body.String()
	if !strings.Contains(list, "inspections/checklist") || !strings.Contains(list, ">Not built in<") || !strings.Contains(list, "/developer/designer?schema=inspections%2Fchecklist") {
		t.Errorf("the list shows the new form, marked, with its designer link")
	}
	editor := h.get(t, "/developer/forms?schema=inspections/checklist&scope=file").Body.String()
	if !strings.Contains(editor, "exists only as an override") || strings.Contains(editor, "dev-diff-add") || !strings.Contains(editor, "no built-in definition to compare") {
		t.Errorf("the editor says the form is not built in and shows no diff")
	}
	if !strings.Contains(h.get(t, "/developer/designer").Body.String(), "inspections/checklist") {
		t.Errorf("the designer's chooser lists it")
	}
	// the designer works on it, and the engine resolves it for code that asks by name
	s := openDesigner(t, h, "inspections/checklist", "file").do("add", url.Values{"new_name": {"inspector"}, "new_widget": {"text"}})
	if s.code != http.StatusOK || !strings.Contains(s.doc, `"inspector"`) {
		t.Fatalf("adding a field to the new form: %d", s.code)
	}
	if rec := h.post(t, "/developer/forms/save", url.Values{"schema": {"inspections/checklist"}, "scope": {"file"}, "json": {s.doc}}); !strings.Contains(location(rec), "msg=saved") {
		t.Fatalf("save: %s", location(rec))
	}
	if got := string(h.rt.Resolver.Resolve(1, "inspections", "checklist", nil)); !strings.Contains(got, "inspector") {
		t.Errorf("resolveSchema must return the new form: %s", got)
	}
	// it cannot be created twice, nor over a built-in form
	for _, v := range []url.Values{
		{"module": {"inspections"}, "name": {"checklist"}, "scope": {"file"}},
		{"module": {"system"}, "name": {"user_create"}, "scope": {"file"}},
	} {
		if loc := create(v); !strings.Contains(loc, "error=form_exists") {
			t.Errorf("%v must be refused as existing: %s", v, loc)
		}
	}
	if b, _ := developerEmbeddedSchema("system", "user_create"); !bytes.Equal(h.rt.Resolver.Resolve(1, "system", "user_create", b), b) {
		t.Errorf("a refused create must not override a built-in form")
	}
	for _, c := range []struct{ module, name, scope, want string }{
		{"Inspections", "x", "file", "error=invalid_name"},
		{"../etc", "passwd", "file", "error=invalid_name"},
		{"a", "b/c", "file", "error=invalid_name"},
		{"", "x", "file", "error=invalid_name"},
		{"a", strings.Repeat("b", 80), "file", "error=invalid_name"},
		{"a", "b", "everyone", "error=bad_scope"},
	} {
		if loc := create(url.Values{"module": {c.module}, "name": {c.name}, "scope": {c.scope}}); !strings.Contains(loc, c.want) {
			t.Errorf("%q/%q: %s, want %s", c.module, c.name, loc, c.want)
		}
	}
	modules := 0
	entries, _ := os.ReadDir(h.schDir)
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			modules++
		}
	}
	if modules != 1 {
		t.Errorf("refusals must write nothing: %d module directories", modules)
	}
	// removing its only override makes it unknown again: there is nothing left to open
	if rec := h.post(t, "/developer/forms/remove", url.Values{"schema": {"inspections/checklist"}, "scope": {"file"}}); !strings.Contains(location(rec), "msg=removed") {
		t.Fatalf("remove: %s", location(rec))
	}
	if rec := h.get(t, "/developer/forms?schema=inspections/checklist"); !strings.Contains(rec.Body.String(), "That form definition does not exist.") {
		t.Errorf("a custom form with no override left is unknown")
	}
	if rec := h.do(t, "POST", "/developer/forms/create", "admin", url.Values{"module": {"a"}, "name": {"b"}, "scope": {"file"}}); rec.Code != http.StatusForbidden {
		t.Errorf("an admin must get 403, got %d", rec.Code)
	}
}

// A tenant-scope custom form keeps its history, so a removed one can be opened
// and restored.
func TestForms_ARemovedCustomFormCanBeRestored(t *testing.T) {
	h, _ := scopedHarness(t)
	if loc := location(h.post(t, "/developer/forms/create", url.Values{"module": {"inspections"}, "name": {"round"}, "scope": {"tenant"}})); !strings.Contains(loc, "msg=created") {
		t.Fatalf("create: %s", loc)
	}
	if fileCount(t, h.schDir) != 0 {
		t.Errorf("a tenant-scope form is not written to the directory")
	}
	if loc := location(h.post(t, "/developer/forms/remove", url.Values{"schema": {"inspections/round"}, "scope": {"tenant"}})); !strings.Contains(loc, "msg=removed") {
		t.Fatalf("remove: %s", loc)
	}
	view := h.get(t, "/developer/forms?schema=inspections/round&scope=tenant").Body.String()
	if !strings.Contains(view, `action="/developer/forms/restore"`) {
		t.Fatalf("the removed form must still open, with its history")
	}
	if loc := location(h.post(t, "/developer/forms/restore", url.Values{"schema": {"inspections/round"}, "scope": {"tenant"}, "version": {"1"}})); !strings.Contains(loc, "msg=restored") {
		t.Errorf("restore: %s", loc)
	}
	if got := string(h.rt.Resolver.Resolve(1, "inspections", "round", nil)); !strings.Contains(got, `"name"`) {
		t.Errorf("the restored form is live: %s", got)
	}
}

// With no overrides applied, the export's source locale file is the built-in
// file, byte for byte: the export must not reformat what it did not change.
func TestExport_SourceLocaleRoundTripsTheBuiltInFiles(t *testing.T) {
	tr, err := i18n.New("en")
	if err != nil {
		t.Fatal(err)
	}
	for _, locale := range tr.Locales() {
		raw, ok := i18n.EmbeddedLocale(locale)
		if !ok {
			t.Fatalf("%s: no built-in file", locale)
		}
		out, changed, err := exportSourceLocale(raw, nil)
		if err != nil || changed {
			t.Fatalf("%s: %v, changed=%v", locale, err, changed)
		}
		if !bytes.Equal(out, raw) {
			t.Errorf("%s: the export reformats the built-in file (%d bytes against %d)", locale, len(out), len(raw))
		}
		// an override equal to the built-in text changes nothing
		if _, changed, _ := exportSourceLocale(raw, map[string]string{"action.save": tr.T(locale, "action.save")}); changed {
			t.Errorf("%s: an override equal to the built-in text is not a change", locale)
		}
	}
	for _, bad := range []string{"../en", "en/../../x", "", "EN", strings.Repeat("a", 50)} {
		if _, ok := i18n.EmbeddedLocale(bad); ok {
			t.Errorf("EmbeddedLocale(%q) must find nothing", bad)
		}
	}
}

func TestExport_ZipHoldsBothLayouts(t *testing.T) {
	h, _ := scopedHarness(t)
	empty := h.get(t, "/developer/export")
	if empty.Code != http.StatusOK || empty.Header().Get("Content-Type") != "application/zip" || !strings.Contains(empty.Header().Get("Content-Disposition"), "seam-overrides-tenant1-") {
		t.Fatalf("export: %d %q %q", empty.Code, empty.Header().Get("Content-Type"), empty.Header().Get("Content-Disposition"))
	}
	if files := zipFiles(t, empty.Body.Bytes()); len(files) != 1 || !strings.Contains(files["README.txt"], "nothing is overridden") {
		t.Errorf("with nothing overridden the zip holds only the note: %d files", len(files))
	}
	// a tenant text, a file text that outranks a tenant one, a new key, a tenant form and a new file form
	must := func(loc string) {
		t.Helper()
		if !strings.Contains(loc, "msg=") {
			t.Fatalf("setup failed: %s", loc)
		}
	}
	must(trSave(h, t, "tenant", "developer.nav.overview", "Home <b>&</b> \"start\""))
	must(trSave(h, t, "tenant", "developer.nav.forms", "Tenant wording"))
	must(trSave(h, t, "file", "developer.nav.forms", "File wording"))
	must(location(h.post(t, "/developer/translations/create", url.Values{"key": {"orders.list.title"}, "value": {"Orders"}, "scope": {"tenant"}})))
	must(location(formSave(h, t, "tenant", e2eOverride, "")))
	must(location(h.post(t, "/developer/forms/create", url.Values{"module": {"inspections"}, "name": {"checklist"}, "scope": {"file"}})))

	rec := h.get(t, "/developer/export")
	files := zipFiles(t, rec.Body.Bytes())
	over := files["overrides/locales/en.json"]
	for _, want := range []string{`"developer.nav.overview": "Home <b>&</b> \"start\""`, `"developer.nav.forms": "File wording"`, `"orders.list.title": "Orders"`} {
		if !strings.Contains(over, want) {
			t.Errorf("overrides/locales/en.json lacks %s:\n%s", want, over)
		}
	}
	if strings.Contains(over, "Tenant wording") {
		t.Errorf("where a file and a tenant override both exist the file's is exported")
	}
	src := files["source/internal/i18n/locales/en.json"]
	raw, _ := i18n.EmbeddedLocale("en")
	if src == "" || !strings.HasSuffix(strings.TrimSpace(src), "\"orders.list.title\": \"Orders\"\n}") {
		t.Errorf("the source file ends with the new key")
	}
	// the source file is the built-in one with exactly those three lines different
	want := strings.Replace(string(raw), `"developer.nav.overview": "Overview"`, `"developer.nav.overview": "Home <b>&</b> \"start\""`, 1)
	want = strings.Replace(want, `"developer.nav.forms": "Form definitions"`, `"developer.nav.forms": "File wording"`, 1)
	want = strings.TrimSuffix(want, "\n}\n") + ",\n  \"orders.list.title\": \"Orders\"\n}\n"
	if src != want {
		t.Errorf("the source file must be the built-in one with only the overrides applied")
	}
	if _, has := files["source/internal/i18n/locales/es.json"]; has {
		t.Errorf("a locale with nothing overridden is not exported")
	}
	for _, name := range []string{"overrides/schemas/system/user_create.json", "source/internal/formengine/schemas/system/user_create.json",
		"overrides/schemas/inspections/checklist.json", "source/internal/formengine/schemas/inspections/checklist.json"} {
		if files[name] == "" {
			t.Errorf("the zip lacks %s", name)
		}
	}
	if !strings.Contains(files["overrides/schemas/system/user_create.json"], "Alpha OVERRIDE") {
		t.Errorf("the tenant's definition is exported")
	}
	if readme := files["README.txt"]; !strings.Contains(readme, "a new form") || !strings.Contains(readme, "replaces the built-in form") || !strings.Contains(readme, "3 text(s)") {
		t.Errorf("the note lists what is inside:\n%s", readme)
	}
	for name := range files {
		if strings.Contains(name, "..") || strings.HasPrefix(name, "/") {
			t.Errorf("unsafe name in the zip: %q", name)
		}
	}
	// another role gets none
	if rec := h.do(t, "GET", "/developer/export", "admin", nil); rec.Code != http.StatusForbidden {
		t.Errorf("an admin must get 403, got %d", rec.Code)
	}
}
