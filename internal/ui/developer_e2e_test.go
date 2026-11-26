package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/devruntime"
	"github.com/ha1tch/seam-ui/internal/devstate"
	"github.com/ha1tch/seam-ui/internal/features"
	fe "github.com/ha1tch/seam-ui/internal/formengine"
	"github.com/ha1tch/seam-ui/internal/i18n"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

// End-to-end tests of the Developer sections: the real router and handlers,
// the real form engine, a real translator and real override directories.

// devHarnessConfig, when a test sets it, adds flags to the set the harness resolves
// (the Developer module is always on).
var devHarnessConfig map[string]bool

type devHarness struct {
	mux            *http.ServeMux
	sess           *auth.SessionManager
	tr             *i18n.Translator
	locDir, schDir string
	rt             *devruntime.Runtime
	ui             *UIHandler // for tests of what the pages compute
}

func (h *devHarness) runtimeForTest() *devruntime.Runtime { return h.rt }

func newDevHarness(t *testing.T, withLocales, withSchemas bool) *devHarness {
	t.Helper()
	tr, err := i18n.New("en")
	if err != nil {
		t.Fatal(err)
	}
	h := &devHarness{tr: tr}
	if withLocales {
		h.locDir = t.TempDir()
	}
	if withSchemas {
		h.schDir = t.TempDir()
	}
	flagConfig := map[string]bool{features.Developer: true}
	for k, v := range devHarnessConfig {
		flagConfig[k] = v
	}
	set, err := features.Catalog().Resolve(features.Inputs{Config: flagConfig, AuthEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	rt, err := devruntime.Start(ctx, devruntime.Options{
		Flags: set, Translator: tr, Store: devstate.NewMemoryStore(),
		ValidateSchema: func(b []byte) error { _, err := fe.ParseSchema(b); return err },
		LocalesDir:     h.locDir, SchemasDir: h.schDir, PollInterval: time.Hour, HistoryInterval: -1, Watch: "poll", SchemaCheck: DeveloperFileCheck,
	})
	if err != nil {
		t.Fatal(err)
	}
	h.rt = rt
	SetSchemaResolver(rt.Resolver)
	t.Cleanup(func() { SetSchemaResolver(nil) })
	h.sess = auth.NewSessionManager(auth.SessionConfig{Secret: "test-secret"})
	cfg := Config{AppName: "Test", Version: "0.0.0", Locale: "en", EnableDeveloper: true, EnableAdmin: true,
		EnableOperational: true, Flags: set, DevRuntime: rt}
	router := NewRouter(xolu.New("http://127.0.0.1:1"), cfg, tr, nil, h.sess, nil, nil, nil, nil)
	h.ui = router.handler
	h.mux = http.NewServeMux()
	router.MountDeveloperRoutes(h.mux)
	router.MountAdministrationRoutes(h.mux) // /admin/users/new renders system/user_create
	router.MountAssetRoutes(h.mux)          // /brands/new renders masters/marca
	router.MountDemoRoutes(h.mux)           // off at startup: mounted for the runtime switch (DEV-27)
	router.MountIssuesRoutes(h.mux)         // the same, for an application module
	h.mux.Handle("/", router)
	return h
}

func (h *devHarness) do(t *testing.T, method, path, role string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if form != nil {
		req = httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if role != "" {
		req.AddCookie(sessionCookie(t, h.sess, role))
	}
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, req)
	return rec
}

func (h *devHarness) get(t *testing.T, path string) *httptest.ResponseRecorder {
	return h.do(t, "GET", path, "developer", nil)
}
func (h *devHarness) post(t *testing.T, path string, form url.Values) *httptest.ResponseRecorder {
	return h.do(t, "POST", path, "developer", form)
}

func location(rec *httptest.ResponseRecorder) string { return rec.Header().Get("Location") }

func dirIsEmpty(t *testing.T, dir string) bool {
	t.Helper()
	n := 0
	// the layer's own history (.history) is a record of its changes, not an override
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() && info.Name() == ".history" {
			return filepath.SkipDir
		}
		if err == nil && p != dir {
			n++
		}
		return nil
	})
	return n == 0
}

const e2eOverride = `{"type":"object","properties":{"alpha":{"type":"string","title":"Alpha OVERRIDE","x-seam-input":"text"}}}`

// ---- access ----------------------------------------------------------------

func TestSections_AreForDevelopersOnly(t *testing.T) {
	h := newDevHarness(t, true, true)
	for _, p := range []string{"/developer", "/developer/overview", "/developer/startup", "/developer/translations", "/developer/forms"} {
		if rec := h.do(t, "GET", p, "developer", nil); rec.Code != http.StatusOK {
			t.Errorf("GET %s as developer: %d", p, rec.Code)
		}
		if rec := h.do(t, "GET", p, "admin", nil); rec.Code != http.StatusForbidden {
			t.Errorf("GET %s as admin must be forbidden, got %d", p, rec.Code)
		}
		if rec := h.do(t, "GET", p, "", nil); rec.Code != http.StatusSeeOther {
			t.Errorf("GET %s anonymous must go to login, got %d", p, rec.Code)
		}
	}
	// every state-changing route refuses everyone but a developer, and writes nothing
	form := url.Values{"locale": {"en"}, "key": {"developer.nav.overview"}, "value": {"Home"}, "schema": {"system/user_edit"}, "json": {e2eOverride}}
	for _, p := range []string{"/developer/translations/save", "/developer/translations/remove", "/developer/forms/save", "/developer/forms/remove", "/developer/forms/preview", "/developer/flags", "/developer/reload"} {
		if rec := h.do(t, "POST", p, "admin", form); rec.Code != http.StatusForbidden {
			t.Errorf("POST %s as admin must be forbidden, got %d", p, rec.Code)
		}
		if rec := h.do(t, "POST", p, "", form); rec.Code != http.StatusSeeOther || !strings.Contains(location(rec), "login") {
			t.Errorf("POST %s anonymous must go to login, got %d %s", p, rec.Code, location(rec))
		}
	}
	if !dirIsEmpty(t, h.locDir) || !dirIsEmpty(t, h.schDir) {
		t.Errorf("refused requests must write nothing")
	}
}

// Regression: the switch forms say where to return to, from a fixed list, so
// the field can never redirect off-site.
func TestFlags_ReturnToTheirSectionAndNeverOffSite(t *testing.T) {
	h := newDevHarness(t, true, true)
	for back, want := range map[string]string{"translations": "/developer/translations", "forms": "/developer/forms", "overview": "/developer/overview", "": "/developer/overview",
		"//evil.example/x": "/developer/overview", "https://evil.example": "/developer/overview", "../../x": "/developer/overview"} {
		rec := h.post(t, "/developer/flags", url.Values{"key": {"hotload_i18n"}, "action": {"on"}, "back": {back}})
		if rec.Code != http.StatusSeeOther || !strings.HasPrefix(location(rec), want+"?") {
			t.Errorf("back=%q: want a redirect to %s, got %d %q", back, want, rec.Code, location(rec))
		}
	}
}

// ---- Translations ----------------------------------------------------------

func TestTranslations_ListFilterAndHostileParameters(t *testing.T) {
	h := newDevHarness(t, true, true)
	rec := h.get(t, "/developer/translations?q=developer.nav.forms")
	body := rec.Body.String()
	if rec.Code != 200 || !strings.Contains(body, "developer.nav.forms") || strings.Contains(body, "developer.nav.startup") {
		t.Errorf("the filter must narrow the list (%d)", rec.Code)
	}
	for _, q := range []string{
		"locale=../../etc/passwd", "locale=%00", "page=abc", "page=-5", "page=999999", "page=0",
		"q=" + strings.Repeat("x", 5000), "key=%3Cscript%3Ealert(1)%3C/script%3E", "key=" + strings.Repeat("k", 3000),
		"locale=es&q=%25&page=2", "q=%5B%28", "key=developer.nav.forms&locale=zz",
	} {
		rec := h.get(t, "/developer/translations?"+q)
		if rec.Code != 200 {
			t.Errorf("?%.40s: got %d", q, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "<script>alert(1)") {
			t.Errorf("?%.40s: a parameter was echoed unescaped", q)
		}
	}
	if rec := h.get(t, "/developer/translations?key=no.such.key"); !strings.Contains(rec.Body.String(), "That text does not exist.") {
		t.Errorf("an unknown key says so")
	}
	if rec := h.get(t, "/developer/translations?key=developer.nav.forms"); !strings.Contains(rec.Body.String(), "/developer/translations/save") {
		t.Errorf("a known key opens the editor")
	}
}

func TestTranslations_SaveAppliesAtOnceAndRemoveRestores(t *testing.T) {
	h := newDevHarness(t, true, true)
	key := "developer.nav.overview"
	rec := h.post(t, "/developer/translations/save", url.Values{"locale": {"en"}, "key": {key}, "value": {"Home"}, "q": {"nav"}, "page": {"1"}})
	if rec.Code != http.StatusSeeOther || !strings.Contains(location(rec), "msg=saved") || !strings.HasPrefix(location(rec), "/developer/translations?") {
		t.Fatalf("a save returns to the list: %d %s", rec.Code, location(rec))
	}
	if got := h.tr.T("en", key); got != "Home" {
		t.Errorf("the override must be live at once, got %q", got)
	}
	if b, _ := os.ReadFile(filepath.Join(h.locDir, "en.json")); !strings.Contains(string(b), `"Home"`) {
		t.Errorf("en.json must hold the override: %s", b)
	}
	rec = h.post(t, "/developer/translations/remove", url.Values{"locale": {"en"}, "key": {key}})
	if !strings.Contains(location(rec), "msg=removed") || h.tr.T("en", key) != "Overview" {
		t.Errorf("removing restores the built-in text: %s %q", location(rec), h.tr.T("en", key))
	}
	if !dirIsEmpty(t, h.locDir) {
		t.Errorf("removing the last override removes the file")
	}
}

// Regression: a browser submits a textarea's newlines as CRLF; they used to be
// refused as control characters.
func TestTranslations_BrowserNewlinesAreAccepted(t *testing.T) {
	h := newDevHarness(t, true, true)
	rec := h.post(t, "/developer/translations/save", url.Values{"locale": {"en"}, "key": {"developer.nav.overview"}, "value": {"Line one\r\nLine two"}})
	if !strings.Contains(location(rec), "msg=saved") {
		t.Fatalf("CRLF must be accepted: %s", location(rec))
	}
	if b, _ := os.ReadFile(filepath.Join(h.locDir, "en.json")); strings.Contains(string(b), `\r`) {
		t.Errorf("stored text must use plain newlines: %s", b)
	}
}

// Adversarial: refused saves leave the directory untouched.
func TestTranslations_SaveRefusals(t *testing.T) {
	h := newDevHarness(t, true, true)
	cases := []struct {
		name string
		form url.Values
		want string
	}{
		{"placeholders dropped", url.Values{"locale": {"en"}, "key": {"developer.layers.keys"}, "value": {"no placeholder"}}, "error=verbs"},
		{"placeholders reordered", url.Values{"locale": {"en"}, "key": {"developer.layers.loaded"}, "value": {"%s %s"}}, "error="},
		{"empty", url.Values{"locale": {"en"}, "key": {"developer.nav.overview"}, "value": {"  "}}, "error=invalid_text"},
		{"control characters", url.Values{"locale": {"en"}, "key": {"developer.nav.overview"}, "value": {"a\x00b"}}, "error=invalid_text"},
		{"too long", url.Values{"locale": {"en"}, "key": {"developer.nav.overview"}, "value": {strings.Repeat("x", 4001)}}, "error=invalid_text"},
		{"unknown key", url.Values{"locale": {"en"}, "key": {"no.such.key"}, "value": {"x"}}, "error=unknown_key"},
		{"key that is a path", url.Values{"locale": {"en"}, "key": {"../../etc/passwd"}, "value": {"x"}}, "error=unknown_key"},
		{"unknown locale", url.Values{"locale": {"zz"}, "key": {"developer.nav.overview"}, "value": {"x"}}, "error=unknown_locale"},
		{"locale that is a path", url.Values{"locale": {"../x"}, "key": {"developer.nav.overview"}, "value": {"x"}}, "error=unknown_locale"},
		{"no locale", url.Values{"key": {"developer.nav.overview"}, "value": {"x"}}, "error=unknown_locale"},
	}
	for _, c := range cases {
		rec := h.post(t, "/developer/translations/save", c.form)
		if rec.Code != http.StatusSeeOther || !strings.Contains(location(rec), c.want) {
			t.Errorf("%s: want a redirect with %q, got %d %q", c.name, c.want, rec.Code, location(rec))
		}
	}
	if !dirIsEmpty(t, h.locDir) {
		t.Errorf("no refused save may leave a file behind")
	}
	if got := h.tr.T("en", "developer.nav.overview"); got != "Overview" {
		t.Errorf("no refused save may change a text, got %q", got)
	}
	big := url.Values{"locale": {"en"}, "key": {"developer.nav.overview"}, "value": {strings.Repeat("x", 100<<10)}}
	if rec := h.post(t, "/developer/translations/save", big); rec.Code != http.StatusSeeOther || !strings.Contains(location(rec), "error=bad_request") {
		t.Errorf("an oversize body is refused cleanly: %d %q", rec.Code, location(rec))
	}
}

func TestTranslations_WithoutADirectoryNothingCanBeSaved(t *testing.T) {
	h := newDevHarness(t, false, true)
	rec := h.post(t, "/developer/translations/save", url.Values{"locale": {"en"}, "key": {"developer.nav.overview"}, "value": {"Home"}})
	if !strings.Contains(location(rec), "error=not_configured") {
		t.Errorf("got %q", location(rec))
	}
	if body := h.get(t, "/developer/translations?key=developer.nav.overview").Body.String(); strings.Contains(body, "/developer/translations/save") {
		t.Errorf("the editor must not offer a form it cannot honour")
	}
}

// ---- Form definitions ------------------------------------------------------

func TestForms_CatalogueListsRealFormsAndNotTheDemo(t *testing.T) {
	h := newDevHarness(t, true, true)
	body := h.get(t, "/developer/forms").Body.String()
	for _, want := range []string{"system/user_edit", "developer/schema_edit", "asset/asset", "masters/region", "workorders/create"} {
		if !strings.Contains(body, want) {
			t.Errorf("the catalogue must list %s", want)
		}
	}
	if strings.Contains(body, "demo/") {
		t.Errorf("the Demo's fixed examples cannot be overridden and must not be offered")
	}
}

// Adversarial: the schema name is a catalogue entry, never a path.
func TestForms_HostileSchemaNames(t *testing.T) {
	h := newDevHarness(t, true, true)
	for _, name := range []string{"../../etc/passwd", "system/../asset/asset", "system/nope", "nope/user_edit", "system/", "/user_edit", "system/user_edit/x", "%00", "demo/repeatable", strings.Repeat("a", 5000)} {
		rec := h.get(t, "/developer/forms?schema="+url.QueryEscape(name))
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), "That form definition does not exist.") {
			t.Errorf("GET schema=%.30q: %d", name, rec.Code)
		}
		for _, p := range []string{"/developer/forms/save", "/developer/forms/preview", "/developer/forms/remove"} {
			rec := h.post(t, p, url.Values{"schema": {name}, "json": {e2eOverride}})
			if rec.Code != http.StatusSeeOther || !strings.Contains(location(rec), "error=unknown_form") {
				t.Errorf("POST %s schema=%.30q: %d %q", p, name, rec.Code, location(rec))
			}
		}
	}
	if !dirIsEmpty(t, h.schDir) {
		t.Errorf("a hostile name must write nothing")
	}
}

func TestForms_PreviewValidatesAndRendersButNeverWrites(t *testing.T) {
	h := newDevHarness(t, true, true)
	rec := h.post(t, "/developer/forms/preview", url.Values{"schema": {"system/user_create"}, "json": {e2eOverride}})
	body := rec.Body.String()
	if rec.Code != 200 || !strings.Contains(body, "Alpha OVERRIDE") || !strings.Contains(body, "dev-preview") {
		t.Errorf("a valid definition is rendered by the real engine (%d)", rec.Code)
	}
	bad := h.post(t, "/developer/forms/preview", url.Values{"schema": {"system/user_create"}, "json": {`{"type":`}})
	if bad.Code != 200 || !strings.Contains(bad.Body.String(), "Problem") || strings.Contains(bad.Body.String(), "dev-preview") {
		t.Errorf("an invalid definition shows the problem and no preview")
	}
	if !dirIsEmpty(t, h.schDir) {
		t.Errorf("a preview must never write")
	}
}

// DEV-32 end to end: a saved definition changes the real page, and removing it
// restores the built-in one.
func TestForms_SaveChangesTheRealPageAndRemoveRestoresIt(t *testing.T) {
	h := newDevHarness(t, true, true)
	page := func() string { return h.get(t, "/admin/users/new").Body.String() }
	if strings.Contains(page(), "Alpha OVERRIDE") {
		t.Fatal("precondition: the built-in form has no such field")
	}
	rec := h.post(t, "/developer/forms/save", url.Values{"schema": {"system/user_create"}, "json": {e2eOverride}})
	if rec.Code != http.StatusSeeOther || !strings.Contains(location(rec), "msg=saved") {
		t.Fatalf("save: %d %s", rec.Code, location(rec))
	}
	if b, err := os.ReadFile(filepath.Join(h.schDir, "system", "user_create.json")); err != nil || !strings.Contains(string(b), "Alpha OVERRIDE") {
		t.Fatalf("the file must exist: %v %s", err, b)
	}
	if !strings.Contains(page(), "Alpha OVERRIDE") {
		t.Errorf("the real page must now render the saved definition")
	}
	// the list, narrowed to that one form, names the override as its source
	if list := h.get(t, "/developer/forms?module=system&q=user_create").Body.String(); !strings.Contains(list, "Override (disk)") || strings.Contains(list, ">Built-in<") || strings.Contains(list, "system/user_edit") {
		t.Errorf("the list must show the override as the source, and the filters must narrow it")
	}
	rec = h.post(t, "/developer/forms/remove", url.Values{"schema": {"system/user_create"}})
	if !strings.Contains(location(rec), "msg=removed") || strings.Contains(page(), "Alpha OVERRIDE") {
		t.Errorf("removing restores the built-in form: %s", location(rec))
	}
}

func TestForms_SaveRefusals(t *testing.T) {
	h := newDevHarness(t, true, true)
	for name, doc := range map[string]string{"not json": `{"type":`, "empty": ``, "array": `[1,2]`, "null": `null`} {
		rec := h.post(t, "/developer/forms/save", url.Values{"schema": {"system/user_create"}, "json": {doc}})
		if rec.Code != http.StatusUnprocessableEntity && !(rec.Code == http.StatusSeeOther && strings.Contains(location(rec), "error=")) {
			t.Errorf("%s: must be refused, got %d %q", name, rec.Code, location(rec))
		}
	}
	if rec := h.post(t, "/developer/forms/save", url.Values{"schema": {"system/user_create"}, "json": {`{"type":`}}); rec.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(rec.Body.String(), `{&#34;type&#34;:`) {
		t.Errorf("a refused save keeps what was typed (%d)", rec.Code)
	}
	if !dirIsEmpty(t, h.schDir) {
		t.Errorf("a refused save must write nothing")
	}
	huge := url.Values{"schema": {"system/user_create"}, "json": {`{"properties":{},"pad":"` + strings.Repeat("x", 2<<20) + `"}`}}
	if rec := h.post(t, "/developer/forms/save", huge); rec.Code != http.StatusSeeOther || !strings.Contains(location(rec), "error=bad_request") {
		t.Errorf("an oversize body is refused cleanly: %d %q", rec.Code, location(rec))
	}
	if !dirIsEmpty(t, h.schDir) {
		t.Errorf("an oversize body must write nothing")
	}
}

func TestForms_WithoutADirectoryNothingCanBeSaved(t *testing.T) {
	h := newDevHarness(t, true, false)
	rec := h.post(t, "/developer/forms/save", url.Values{"schema": {"system/user_create"}, "json": {e2eOverride}})
	if !strings.Contains(location(rec), "error=not_configured") {
		t.Errorf("got %q", location(rec))
	}
	if body := h.get(t, "/developer/forms?schema=system/user_create").Body.String(); strings.Contains(body, "/developer/forms/save") {
		t.Errorf("the editor must not offer a form it cannot honour")
	}
}

// The tools' own forms are schemas, so the tools can be edited with themselves.
func TestForms_TheToolsOwnFormsCanBeOverridden(t *testing.T) {
	h := newDevHarness(t, true, true)
	// The engine prefers a translation key over a literal title, so the visible
	// proof is a field the keys do not cover.
	override := `{"type":"object","required":["value"],"properties":{"locale":{"type":"string","title":"Locale","x-seam-input":"text"},"key":{"type":"string","title":"Key","x-seam-input":"text"},"value":{"type":"string","title":"Override","x-seam-input":"textarea"},"zzextra":{"type":"string","title":"Zz Extra Field","x-seam-input":"text"}}}`
	if rec := h.post(t, "/developer/forms/save", url.Values{"schema": {"developer/translation_edit"}, "json": {override}}); !strings.Contains(location(rec), "msg=saved") {
		t.Fatalf("save: %s", location(rec))
	}
	if !strings.Contains(h.get(t, "/developer/translations?key=developer.nav.forms").Body.String(), "Zz Extra Field") {
		t.Errorf("the Translations editor must render the overridden form definition")
	}
}
