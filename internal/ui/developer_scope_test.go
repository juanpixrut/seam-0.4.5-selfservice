package ui

import (
	"context"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/seam-ui/internal/overrides"
)

// scopedHarness is the Developer module with both kinds of override: files on
// disk (all tenants) and a store (the session's tenant, which is 1).
func scopedHarness(t *testing.T) (*devHarness, *overrides.MemoryStore) {
	t.Helper()
	h := newDevHarness(t, true, true)
	store := overrides.NewMemoryStore()
	h.rt.Overrides = overrides.NewService(store, h.tr, h.rt.Resolver)
	return h, store
}

var ctx_ = context.Background()

const (
	trKey     = "developer.nav.translations" // "Translations": no placeholders
	trVerbKey = "developer.banner.active"    // "...: %s."
)

func trSave(h *devHarness, t *testing.T, scope, key, value string) string {
	t.Helper()
	v := url.Values{"locale": {"en"}, "key": {key}, "value": {value}}
	if scope != "" {
		v.Set("scope", scope)
	}
	return location(h.post(t, "/developer/translations/save", v))
}

func fileCount(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	// the layer's own history (.history) is a record of its changes, not an override
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() && info.Name() == ".history" {
			return filepath.SkipDir
		}
		if err == nil && !info.IsDir() {
			n++
		}
		return nil
	})
	return n
}

func TestScopes_TranslationTenantSaveGoesToTheDatabaseOnly(t *testing.T) {
	h, store := scopedHarness(t)
	loc := trSave(h, t, "tenant", trKey, "Textos del tenant")
	if !strings.Contains(loc, "msg=saved") || !strings.Contains(loc, "scope=tenant") {
		t.Fatalf("saved and back in the same scope: %s", loc)
	}
	if rows, _ := store.Translations(ctx_, 1); len(rows) != 1 || rows[0].Value != "Textos del tenant" || rows[0].ChangedBy == 0 {
		t.Errorf("the text is stored for the session's tenant, with who: %+v", rows)
	}
	if fileCount(t, h.locDir) != 0 {
		t.Errorf("a tenant save must write no file")
	}
	if got := h.tr.ForTenant(1).ForLocale("en")(trKey); got != "Textos del tenant" {
		t.Errorf("it is live for the tenant, got %q", got)
	}
	if got := h.tr.ForTenant(2).ForLocale("en")(trKey); got == "Textos del tenant" {
		t.Errorf("another tenant must not see it")
	}
	if got := h.tr.ForLocale("en")(trKey); got == "Textos del tenant" {
		t.Errorf("the global lookup must not see it")
	}
	if !strings.Contains(trRemove(h, t, "tenant", trKey), "msg=removed") {
		t.Errorf("removing it")
	}
	if rows, _ := store.Translations(ctx_, 1); len(rows) != 0 {
		t.Errorf("gone from the store: %+v", rows)
	}
}

func trRemove(h *devHarness, t *testing.T, scope, key string) string {
	t.Helper()
	return location(h.post(t, "/developer/translations/remove", url.Values{"locale": {"en"}, "key": {key}, "scope": {scope}}))
}

func TestScopes_TranslationFileSaveStaysAFileAndShadowsTheTenant(t *testing.T) {
	h, store := scopedHarness(t)
	trSave(h, t, "tenant", trKey, "From the database")
	if !strings.Contains(trSave(h, t, "file", trKey, "From a file"), "msg=saved") {
		t.Fatal("file save")
	}
	if rows, _ := store.Translations(ctx_, 1); len(rows) != 1 || rows[0].Value != "From the database" {
		t.Errorf("a file save leaves the database alone: %+v", rows)
	}
	if fileCount(t, h.locDir) != 1 {
		t.Errorf("and writes exactly one file")
	}
	if got := h.tr.ForTenant(1).ForLocale("en")(trKey); got != "From a file" {
		t.Errorf("a file override outranks the tenant's (DEC-4), got %q", got)
	}
	tenantView := h.get(t, "/developer/translations?locale=en&key="+url.QueryEscape(trKey)+"&scope=tenant").Body.String()
	if !strings.Contains(tenantView, "From the database") || !strings.Contains(tenantView, "outranks this one") {
		t.Errorf("the tenant scope shows its own text and says a file override hides it")
	}
	fileView := h.get(t, "/developer/translations?locale=en&key="+url.QueryEscape(trKey)+"&scope=file").Body.String()
	if !strings.Contains(fileView, "From a file") || !strings.Contains(fileView, "outranks this tenant") {
		t.Errorf("the file scope shows the file's text and says it hides the tenant's")
	}
	if strings.Count(fileView, "dev-seg-btn") != 2 {
		t.Errorf("two scopes, two tabs")
	}
	trRemove(h, t, "file", trKey)
	if got := h.tr.ForTenant(1).ForLocale("en")(trKey); got != "From the database" {
		t.Errorf("removing the file override lets the tenant's show, got %q", got)
	}
}

func TestScopes_TranslationRefusalsStoreAndWriteNothing(t *testing.T) {
	h, store := scopedHarness(t)
	cases := []struct{ name, scope, key, value, want string }{
		{"unknown scope", "everyone", trKey, "x", "error=bad_scope"},
		{"scope with markup", `"><script>`, trKey, "x", "error=bad_scope"},
		{"missing placeholder", "tenant", trVerbKey, "no placeholder here", "error=verbs"},
		{"extra placeholder", "tenant", trVerbKey, "%s and %s", "error=verbs"},
		{"empty text", "tenant", trKey, "   ", "error=invalid_text"},
		{"control character", "tenant", trKey, "a\x00b", "error=invalid_text"},
		{"a key that does not exist", "tenant", "no.such.key", "x", "error=unknown_key"},
	}
	for _, c := range cases {
		if loc := trSave(h, t, c.scope, c.key, c.value); !strings.Contains(loc, c.want) {
			t.Errorf("%s: %s", c.name, loc)
		}
	}
	if rows, _ := store.Translations(ctx_, 1); len(rows) != 0 || fileCount(t, h.locDir) != 0 {
		t.Errorf("a refusal reaches neither the store (%d) nor the disk (%d files)", len(rows), fileCount(t, h.locDir))
	}
	// the tenant scope needs a store: without one it is not a scope at all
	plain := newDevHarness(t, true, true)
	if loc := trSave(plain, t, "tenant", trKey, "x"); !strings.Contains(loc, "error=bad_scope") {
		t.Errorf("tenant scope with no store: %s", loc)
	}
	if loc := trSave(plain, t, "", trKey, "Plain file text"); !strings.Contains(loc, "msg=saved") {
		t.Errorf("a post with no scope still means the file when that is all there is: %s", loc)
	}
}

func formSave(h *devHarness, t *testing.T, scope, doc, note string) *httptest.ResponseRecorder {
	t.Helper()
	v := url.Values{"schema": {"system/user_create"}, "json": {doc}, "scope": {scope}}
	if note != "" {
		v.Set("note", note)
	}
	return h.post(t, "/developer/forms/save", v)
}

func formView(h *devHarness, t *testing.T, scope string) string {
	t.Helper()
	return h.get(t, "/developer/forms?schema=system/user_create&scope="+scope).Body.String()
}

func TestScopes_FormTenantVersionCycle(t *testing.T) {
	h, store := scopedHarness(t)
	docB := strings.Replace(e2eOverride, "Alpha OVERRIDE", "Beta OVERRIDE", 1)
	if loc := location(formSave(h, t, "tenant", e2eOverride, "first try")); !strings.Contains(loc, "msg=saved") || !strings.Contains(loc, "scope=tenant") {
		t.Fatalf("save: %s", loc)
	}
	formSave(h, t, "tenant", docB, "second try")
	if fileCount(t, h.schDir) != 0 {
		t.Errorf("a tenant save must write no file")
	}
	view := formView(h, t, "tenant")
	for _, want := range []string{"Beta OVERRIDE", "second try", "first try", "v2", "v1", "Current", "/developer/forms/restore"} {
		if !strings.Contains(view, want) {
			t.Errorf("the tenant editor must show %q", want)
		}
	}
	if strings.Count(view, `action="/developer/forms/restore"`) != 1 {
		t.Errorf("only the version that is not current can be restored: %d", strings.Count(view, `action="/developer/forms/restore"`))
	}
	if loc := location(h.post(t, "/developer/forms/restore", url.Values{"schema": {"system/user_create"}, "scope": {"tenant"}, "version": {"1"}})); !strings.Contains(loc, "msg=restored") {
		t.Fatalf("restore: %s", loc)
	}
	h1, _ := store.SchemaHistory(ctx_, 1, "system", "user_create")
	if len(h1) != 3 || h1[0].Note != "restored from version 1" || !strings.Contains(string(h1[0].Doc), "Alpha OVERRIDE") {
		t.Errorf("a restore is a new version that says where it came from: %+v", h1[0])
	}
	if got := string(h.rt.Resolver.Resolve(1, "system", "user_create", []byte(`{}`))); !strings.Contains(got, "Alpha OVERRIDE") {
		t.Errorf("version 1's content is live again")
	}
	if loc := location(h.post(t, "/developer/forms/remove", url.Values{"schema": {"system/user_create"}, "scope": {"tenant"}})); !strings.Contains(loc, "msg=removed") {
		t.Fatalf("remove: %s", loc)
	}
	after := formView(h, t, "tenant")
	if !strings.Contains(after, "Removed") || strings.Count(after, `action="/developer/forms/restore"`) != 3 {
		t.Errorf("after a removal the history stays and every version with a definition can come back: %d restore forms", strings.Count(after, `action="/developer/forms/restore"`))
	}
}

func TestScopes_FormRefusalsStoreAndWriteNothing(t *testing.T) {
	h, store := scopedHarness(t)
	rec := formSave(h, t, "tenant", "{not json", "n")
	if rec.Code != 422 || !strings.Contains(rec.Body.String(), "{not json") {
		t.Errorf("a rejected definition keeps what was typed and says 422: %d", rec.Code)
	}
	if loc := location(formSave(h, t, "everyone", e2eOverride, "")); !strings.Contains(loc, "error=bad_scope") {
		t.Errorf("unknown scope: %s", loc)
	}
	if rows, _ := store.CurrentSchemas(ctx_, 1); len(rows) != 0 || fileCount(t, h.schDir) != 0 {
		t.Errorf("a refusal reaches neither the store nor the disk")
	}
	formSave(h, t, "tenant", e2eOverride, "")
	rest := func(scope, version string) string {
		return location(h.post(t, "/developer/forms/restore", url.Values{"schema": {"system/user_create"}, "scope": {scope}, "version": {version}}))
	}
	for name, c := range map[string][2]string{
		"restoring in the file scope": {"file", "1"}, "no scope": {"", "1"}, "not a number": {"tenant", "abc"},
		"zero": {"tenant", "0"}, "negative": {"tenant", "-4"}, "huge": {"tenant", "99999999999999999999"}, "unknown": {"tenant", "99"},
	} {
		if loc := rest(c[0], c[1]); !strings.Contains(loc, "error=") {
			t.Errorf("%s must be refused: %s", name, loc)
		}
	}
	h.post(t, "/developer/forms/remove", url.Values{"schema": {"system/user_create"}, "scope": {"tenant"}})
	if loc := rest("tenant", "2"); !strings.Contains(loc, "error=not_restorable") {
		t.Errorf("a removal has nothing to restore: %s", loc)
	}
	if hist, _ := store.SchemaHistory(ctx_, 1, "system", "user_create"); len(hist) != 2 {
		t.Errorf("no refused restore adds a version: %d", len(hist))
	}
}

func TestScopes_FormFileScopeShadowsTheTenantAndBothKeepNotes(t *testing.T) {
	h, _ := scopedHarness(t)
	formSave(h, t, "tenant", e2eOverride, "kept")
	formSave(h, t, "file", strings.Replace(e2eOverride, "Alpha OVERRIDE", "From a file", 1), "file note")
	if got := string(h.rt.Resolver.Resolve(1, "system", "user_create", []byte(`{}`))); !strings.Contains(got, "From a file") {
		t.Errorf("a file definition outranks the tenant's")
	}
	if v := formView(h, t, "tenant"); !strings.Contains(v, "outranks this one") || !strings.Contains(v, "Saved with this version in the history") {
		t.Errorf("the tenant editor says it is hidden, and offers a note")
	}
	fv := formView(h, t, "file")
	if !strings.Contains(fv, "outranks this tenant") {
		t.Errorf("the file editor says it hides the tenant's")
	}
	// both scopes keep a note with each version, and the file's versions are the file's own
	if !strings.Contains(fv, `name="note"`) || !strings.Contains(formView(h, t, "tenant"), `name="note"`) {
		t.Errorf("both scopes offer the note")
	}
	if !strings.Contains(fv, "file note") || strings.Contains(fv, ">kept<") || strings.Contains(fv, "/developer/forms/restore") {
		t.Errorf("the file scope lists its own one version, which is the live one")
	}
}
