package ui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ha1tch/seam-ui/internal/features"
)

// A module that is off at startup can be switched on for one tenant from the
// panel, and off again: its routes answer 404 until then, and for every other
// tenant throughout.
func TestModules_SwitchedOnForOneTenantAtRuntime(t *testing.T) {
	h := newDevHarness(t, false, false)
	cfg := h.ui.config
	if cfg.EnableDemo || !cfg.moduleSwitches() {
		t.Fatalf("the harness starts with the Demo off and module switches on")
	}
	if rec := h.get(t, "/demo"); rec.Code != http.StatusNotFound {
		t.Fatalf("off: the module's routes answer 404, got %d", rec.Code)
	}
	ov := h.get(t, "/developer/overview").Body.String()
	if !strings.Contains(ov, "Demo module") || !strings.Contains(ov, "Work Orders module") || !strings.Contains(ov, "Issues module") {
		t.Errorf("the Overview offers the three module switches")
	}
	flip := func(action string) {
		t.Helper()
		if rec := h.post(t, "/developer/flags", url.Values{"key": {features.ModuleDemo}, "action": {action}}); rec.Code != http.StatusSeeOther || strings.Contains(location(rec), "error=") {
			t.Fatalf("%s: %d %s", action, rec.Code, location(rec))
		}
	}
	flip("on")
	if !cfg.ModuleOn(1, "developer", false, features.ModuleDemo) || cfg.ModuleOn(2, "developer", false, features.ModuleDemo) || cfg.ModuleOn(0, "developer", false, features.ModuleDemo) {
		t.Errorf("on for the caller's tenant only")
	}
	if cfg.ModuleOn(1, "developer", false, features.ModuleWorkOrders) || cfg.ModuleOn(1, "developer", false, features.ModuleIssues) {
		t.Errorf("the other modules are untouched")
	}
	if rec := h.get(t, "/demo"); rec.Code == http.StatusNotFound {
		t.Errorf("on: the module's routes answer, got %d", rec.Code)
	}
	// by default it is on for the tenant's developers only; a deployment may widen it
	if cfg.ModuleOn(1, "admin", false, features.ModuleDemo) {
		t.Errorf("by default a module switched on at runtime is for developers")
	}
	if rec := h.do(t, http.MethodGet, "/demo", "admin", nil); rec.Code != http.StatusNotFound {
		t.Errorf("an admin of the tenant gets 404 by default, got %d", rec.Code)
	}
	wide := cfg
	wide.DevSettings.ModuleAudience = ModuleAudienceTenant
	if !wide.ModuleOn(1, "viewer", false, features.ModuleDemo) || wide.ModuleOn(2, "viewer", false, features.ModuleDemo) {
		t.Errorf("with the tenant audience every user of that tenant gets it, and no other tenant")
	}
	flip("off")
	if rec := h.get(t, "/demo"); rec.Code != http.StatusNotFound {
		t.Errorf("off again: 404, got %d", rec.Code)
	}
	// a module on at startup is on for everyone and has no switch to show
	on := cfg
	on.EnableDemo = true
	if !on.ModuleOn(2, "viewer", on.EnableDemo, features.ModuleDemo) {
		t.Errorf("on at startup is on for every tenant")
	}
	for _, fv := range developerPanelFlags(on, h.rt.Manager.Panel(1)) {
		if fv.Flag.Key == features.ModuleDemo {
			t.Errorf("a module on at startup has no runtime switch")
		}
	}
	// with the decision switched off nothing is offered and nothing is honoured
	flip("on")
	off := cfg
	off.EnableDeveloper = false
	if off.ModuleOn(1, "developer", false, features.ModuleDemo) {
		t.Errorf("without module switches a stored switch is inert")
	}
	for _, fv := range developerPanelFlags(off, h.rt.Manager.Panel(1)) {
		if fv.Flag.Key == features.ModuleDemo || fv.Flag.Key == features.ModuleWorkOrders || fv.Flag.Key == features.ModuleIssues {
			t.Errorf("no module switch is shown when modules cannot be switched")
		}
	}
}

// The Startup page lists the settings that are not on/off, as in force.
func TestStartup_ListsTheSettings(t *testing.T) {
	h := newDevHarness(t, false, false)
	body := h.get(t, "/developer/startup").Body.String()
	for _, want := range []string{`id="dev-settings"`, "developer_poll_ms", "developer_history_ms", "developer_sync_seconds", "developer_global_tenant", "developer_source_dir", "not set"} {
		if !strings.Contains(body, want) {
			t.Errorf("the Startup page lacks %q", want)
		}
	}
}

// With a source checkout configured the Translations list can show the texts
// nothing in the source refers to; without one the filter is not offered.
func TestTranslations_NotReferencedInTheSource(t *testing.T) {
	h := newDevHarness(t, true, false)
	if body := h.get(t, "/developer/translations").Body.String(); strings.Contains(body, `value="unused"`) {
		t.Fatalf("without a source checkout the filter is not offered")
	}
	if body := h.get(t, "/developer/translations?show=unused").Body.String(); !strings.Contains(body, "key=") {
		t.Errorf("asked for anyway, the filter means every text")
	}
	h.ui.config.DevSettings.SourceDir = "../.." // the repository, from this package
	body := h.get(t, "/developer/translations").Body.String()
	if !strings.Contains(body, `value="unused"`) {
		t.Fatalf("with a source checkout the filter is offered")
	}
	unused := h.ui.unreferencedKeys(1)
	for _, k := range unused {
		if k == "action.cancel" || k == "developer.nav.forms" || strings.HasPrefix(k, "developer.scope.") {
			t.Errorf("%s is referred to by the source", k)
		}
	}
	t.Logf("%d of the built-in texts are not referenced in the source", len(unused))
	list := h.get(t, "/developer/translations?show=unused").Body.String()
	if strings.Contains(list, "key=action.cancel") {
		t.Errorf("the filtered list leaves out the texts in use")
	}
	if len(unused) > 0 && !strings.Contains(list, url.QueryEscape(unused[0])) && !strings.Contains(list, unused[0]) {
		t.Errorf("the filtered list shows a text that is not referenced (%s)", unused[0])
	}
}

// What a developer last chose (the scope to save into, the language of the
// Translations list) is remembered in their browser and used when the address
// does not say.
func TestPreferences_ScopeAndLanguageAreRemembered(t *testing.T) {
	h, _ := scopedHarness(t)
	first := h.get(t, "/developer/translations?locale=es&scope=file&key="+trKey)
	var cookie *http.Cookie
	for _, c := range first.Result().Cookies() {
		if c.Name == developerPrefsCookie {
			cookie = c
		}
	}
	if cookie == nil || cookie.Path != "/developer" || !cookie.HttpOnly {
		t.Fatalf("a choice made in the address is remembered in a cookie of the module: %+v", cookie)
	}
	if p := readDeveloperPrefs(&http.Request{Header: http.Header{"Cookie": {cookie.String()}}}); p.Scope != "file" || p.Locale != "es" {
		t.Fatalf("remembered: %+v", p)
	}
	withPrefs := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(sessionCookie(t, h.sess, "developer"))
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		h.mux.ServeHTTP(rec, req)
		return rec
	}
	// with the cookie and no choice in the address, the page opens the same way
	rec := withPrefs("/developer/translations?key=" + trKey)
	body := rec.Body.String()
	if !strings.Contains(body, `name="scope" type="hidden" value="file"`) && !strings.Contains(body, `value="file"`) {
		t.Errorf("the editor opens in the remembered scope")
	}
	if !strings.Contains(body, "locale=es") {
		t.Errorf("the list opens in the remembered language")
	}
	// the address wins, and nothing odd is believed
	if rec := withPrefs("/developer/translations?locale=en&scope=tenant&key=" + trKey); !strings.Contains(rec.Body.String(), "locale=en") {
		t.Errorf("a choice in the address wins over the remembered one")
	}
	for _, bad := range []string{"scope=everyone&locale=es", "scope=file&locale=../../etc", "%zz"} {
		p := readDeveloperPrefs(&http.Request{Header: http.Header{"Cookie": {developerPrefsCookie + "=" + url.QueryEscape(bad)}}})
		if p.Scope == "everyone" || strings.Contains(p.Locale, "/") {
			t.Errorf("a forged preference is not believed: %q -> %+v", bad, p)
		}
	}
	// a POST neither reads nor writes preferences
	if rec := h.post(t, "/developer/translations/save", url.Values{"locale": {"en"}, "key": {trKey}, "value": {"X"}, "scope": {"tenant"}}); len(rec.Result().Cookies()) != 0 {
		for _, c := range rec.Result().Cookies() {
			if c.Name == developerPrefsCookie {
				t.Errorf("a save does not touch the preferences")
			}
		}
	}
}
