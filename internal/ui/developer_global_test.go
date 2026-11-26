package ui

import (
	"net/url"
	"strings"
	"testing"

	"github.com/ha1tch/seam-ui/internal/overrides"
)

// The global scope is offered only to the developers of the tenant the
// setting names, and what they save there reaches every tenant, below each
// tenant's own override.
func TestScopes_GlobalIsForTheHomeTenantAndReachesEveryTenant(t *testing.T) {
	h, _ := scopedHarness(t)
	editor := func() string { return h.get(t, "/developer/translations?locale=en&key="+trKey).Body.String() }
	saveGlobal := func() string {
		return location(h.post(t, "/developer/translations/save", url.Values{"locale": {"en"}, "key": {trKey}, "value": {"For everyone"}, "scope": {"global"}}))
	}
	// off by default: not offered, and refused when asked for by hand
	if strings.Contains(editor(), "scope=global") {
		t.Fatalf("the global scope is not offered unless a home tenant is configured")
	}
	if loc := saveGlobal(); !strings.Contains(loc, "error=bad_scope") {
		t.Fatalf("a global save is refused when the scope is off: %s", loc)
	}
	// configured for another tenant: still not this developer's to use
	h.rt.Overrides.AllowGlobal()
	h.ui.config.DevSettings.GlobalTenant = 2
	if strings.Contains(editor(), "scope=global") || !strings.Contains(saveGlobal(), "error=bad_scope") {
		t.Fatalf("only the home tenant's developers may use the global scope")
	}
	// the home tenant
	h.ui.config.DevSettings.GlobalTenant = 1
	if !strings.Contains(editor(), "scope=global") {
		t.Fatalf("the home tenant's developers are offered the global scope")
	}
	if loc := saveGlobal(); !strings.Contains(loc, "msg=saved") {
		t.Fatalf("save: %s", loc)
	}
	for _, tenant := range []int64{1, 2, 9} {
		if got := h.tr.ForTenant(tenant).T("en", trKey); got != "For everyone" {
			t.Errorf("tenant %d sees the global text, got %q", tenant, got)
		}
	}
	// a tenant's own override wins for that tenant, and the global editor says so
	if loc := trSave(h, t, "tenant", trKey, "Only here"); !strings.Contains(loc, "msg=saved") {
		t.Fatal(loc)
	}
	if a, b := h.tr.ForTenant(1).T("en", trKey), h.tr.ForTenant(2).T("en", trKey); a != "Only here" || b != "For everyone" {
		t.Errorf("tenant beats global: %q %q", a, b)
	}
	ed := h.get(t, "/developer/translations?locale=en&scope=global&key="+trKey).Body.String()
	if !strings.Contains(ed, "For everyone") || !strings.Contains(ed, "outranks this one here") {
		t.Errorf("the global editor shows the global text and says it is outranked here")
	}
	// forms: saved, in force elsewhere, with history, and removable
	if loc := location(formSave(h, t, "global", e2eOverride, "for all")); !strings.Contains(loc, "msg=saved") {
		t.Fatalf("form save: %s", loc)
	}
	if got := string(h.rt.Resolver.Resolve(7, "system", "user_create", nil)); !strings.Contains(got, "Alpha OVERRIDE") {
		t.Errorf("another tenant gets the global form")
	}
	if hist, _ := h.rt.Overrides.History(ctx_, overrides.GlobalTenant, "system", "user_create"); len(hist) != 1 || hist[0].Note != "for all" {
		t.Errorf("the global form has its history: %+v", hist)
	}
	if fv := formView(h, t, "global"); !strings.Contains(fv, "Alpha OVERRIDE") || !strings.Contains(fv, "for all") {
		t.Errorf("the global editor shows the definition and its version")
	}
	if loc := location(h.post(t, "/developer/forms/remove", url.Values{"schema": {"system/user_create"}, "scope": {"global"}})); !strings.Contains(loc, "msg=removed") {
		t.Fatalf("remove: %s", loc)
	}
	if got := h.rt.Resolver.Resolve(7, "system", "user_create", nil); got != nil {
		t.Errorf("removed for every tenant")
	}
	// the tenant scope is untouched by all of it
	if hist, _ := h.rt.Overrides.History(ctx_, 1, "system", "user_create"); len(hist) != 0 {
		t.Errorf("nothing was written to the tenant's own scope: %+v", hist)
	}
}

// The export includes what the global scope puts in force for the tenant,
// except where the tenant's own override or a file replaces it.
func TestExport_IncludesTheGlobalScope(t *testing.T) {
	h, _ := scopedHarness(t)
	h.rt.Overrides.AllowGlobal()
	h.ui.config.DevSettings.GlobalTenant = 1
	save := func(scope, key, value string) {
		t.Helper()
		if loc := location(h.post(t, "/developer/translations/save", url.Values{"locale": {"en"}, "key": {key}, "value": {value}, "scope": {scope}})); !strings.Contains(loc, "msg=saved") {
			t.Fatalf("save %s %s: %s", scope, key, loc)
		}
	}
	save("global", "developer.nav.overview", "Global home")
	save("global", "developer.nav.forms", "Global forms")
	save("tenant", "developer.nav.forms", "Tenant forms")
	if loc := location(formSave(h, t, "global", e2eOverride, "")); !strings.Contains(loc, "msg=saved") {
		t.Fatal(loc)
	}
	files := zipFiles(t, h.get(t, "/developer/export").Body.Bytes())
	over := files["overrides/locales/en.json"]
	if !strings.Contains(over, `"developer.nav.overview": "Global home"`) || !strings.Contains(over, `"developer.nav.forms": "Tenant forms"`) || strings.Contains(over, "Global forms") {
		t.Errorf("the global text is exported, and the tenant's replaces the global one of the same key:\n%s", over)
	}
	if _, ok := files["overrides/schemas/system/user_create.json"]; !ok {
		t.Errorf("the global form is exported")
	}
	if !strings.Contains(files["README.txt"], "1 text(s) and 1 form(s) of the global scope") {
		t.Errorf("the note says what comes from the global scope:\n%s", files["README.txt"])
	}
}
