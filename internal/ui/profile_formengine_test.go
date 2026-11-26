package ui

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// This file guards a real bug found 2026-08-03 while building MOD-8
// (formengine rebuild of Profile/Preferences): the package-level,
// nil-Translator FormEngine pattern masters_pages_dynamic.go's own
// mastersEngine uses (a real, existing precedent in this codebase, not
// invented to have something to test) silently breaks both
// resolveEnumOptionLabel and resolveSelectPlaceholder -- confirmed
// directly by rendering with it: every select showed raw enum values
// ("en" instead of "English") and the hardcoded-Spanish placeholder
// fallback regardless of requested locale. That precedent never
// surfaced this bug because it's explicitly not wired into any live
// route. Profile/Preferences construct their FormEngine per-request
// with a real Translator instead (routes.go's Profile handler,
// mirroring AssetCreateDynamic/AssetUpdateDynamic's own proven
// pattern) -- these tests would have caught the original mistake.

func TestProfilePage_SelectPlaceholderIsLocaleCorrect(t *testing.T) {
	router, sessionMgr := buildRoutingTestRouter(t, false, false)
	mux := mountedMux(router)
	cookie := sessionCookie(t, sessionMgr, "admin")
	req := httptest.NewRequest("GET", "/profile", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("GET /profile: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "— Seleccionar —") {
		t.Error(`response contains the Spanish placeholder fallback ("— Seleccionar —") for an English request -- the FormEngine's Translator is not wired correctly`)
	}
	if !strings.Contains(body, "— Select —") {
		t.Error(`response doesn't contain the correct English placeholder ("— Select —")`)
	}
}

func TestProfilePage_EnumOptionsHaveRealLabels(t *testing.T) {
	router, sessionMgr := buildRoutingTestRouter(t, false, false)
	mux := mountedMux(router)
	cookie := sessionCookie(t, sessionMgr, "admin")
	req := httptest.NewRequest("GET", "/profile", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("GET /profile: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	// Raw enum values as visible option text (not just as the value=
	// attribute, which must still be the raw value for correct
	// round-tripping) would mean resolveEnumOptionLabel's i18n lookup
	// silently failed and fell back to the literal.
	for _, want := range []string{
		`value="en">English<`,
		`value="es">Spanish<`,
		`value="system">System<`,
		`value="light">Light<`,
		`value="dark">Dark<`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("response missing expected labeled option %q -- enum option labels may not be resolving", want)
		}
	}
}

func TestProfilePage_NameFieldPrefilledWithCurrentValue(t *testing.T) {
	// Confirms the formengine-rendered "name" field round-trips real
	// user data through to the rendered HTML, not a blank/default --
	// the same class of "form renders but doesn't actually show current
	// data" gap checked for other dynamic forms this session.
	// buildRoutingTestRouter's xolu client points at an unreachable
	// address, so Profile's own "create a default user" fallback fires
	// (its own hardcoded "Demo User") -- that's still a genuine value
	// flowing from Go data into the schema's values map into the
	// rendered <input>, which is what this test actually checks; it
	// isn't exercising a real xolu round-trip.
	router, sessionMgr := buildRoutingTestRouter(t, false, false)
	mux := mountedMux(router)
	cookie := sessionCookie(t, sessionMgr, "admin")
	req := httptest.NewRequest("GET", "/profile", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, `id="field-name"`) {
		t.Fatal(`response has no formengine-rendered name field (id="field-name") at all`)
	}
	if !strings.Contains(body, `value="Demo User"`) {
		t.Error(`name field doesn't carry the fetched user's actual name -- value interpolation into the formengine-rendered field may be broken`)
	}
}

// TestSettingsPage_FieldTitlesAreTranslated guards the field.* i18n keys
// added alongside Settings' formengine rebuild (MOD-8) -- these are
// separate from, and in addition to, renderFieldRow's own engine-level
// fix: the engine fix makes resolution possible, but resolution still
// needs the actual keys to exist. Found and fixed together: neither
// alone was sufficient, confirmed by checking both.
//
// buildRoutingTestRouter hardcodes Locale: "en" (no parameter to
// change it), so this can't test English-schema-literal-vs-Spanish-
// resolved the way the live curl check did -- it instead checks for
// the specific *English* resolved text reused verbatim from the old
// hand-coded settings.field.* keys, which deliberately differs in
// wording from each schema's own literal title (e.g. "Tenant Name" vs
// the schema's "Organization name", "Asset Map" vs "Map view"): seeing
// the reused wording, not the schema's own literal, proves the i18n
// key resolved rather than falling back.
func TestSettingsPage_FieldTitlesAreTranslated(t *testing.T) {
	router, sessionMgr := buildRoutingTestRouter(t, true, false)
	mux := mountedMux(router)
	cookie := sessionCookie(t, sessionMgr, "admin")
	req := httptest.NewRequest("GET", "/settings", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("GET /settings: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Tenant Name", "Slack Integration", "Asset Map"} {
		if !strings.Contains(body, want) {
			t.Errorf("response missing expected resolved title %q -- field title i18n keys may be missing or not resolving (schema literal would show instead)", want)
		}
	}
}

// TestSettingsPage_LocaleOptionsHaveRealLabels guards the general.locale.*
// enum-label keys specifically -- these are form-scoped
// (resolveEnumOptionLabel's key is rc.Form + "." + fieldName + "." +
// value), so the identical preferences.locale.* keys added for the
// Preferences form do NOT cover this form too, even though both
// schemas declare a "locale" field with the same enum values -- found
// live (curl against the real running server, not assumed) when the
// General settings page still showed raw "en"/"es" after Preferences'
// labels had already been added and verified working.
func TestSettingsPage_LocaleOptionsHaveRealLabels(t *testing.T) {
	router, sessionMgr := buildRoutingTestRouter(t, true, false)
	mux := mountedMux(router)
	cookie := sessionCookie(t, sessionMgr, "admin")
	req := httptest.NewRequest("GET", "/settings", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	body := rec.Body.String()
	for _, want := range []string{`value="en">English<`, `value="es">Spanish<`} {
		if !strings.Contains(body, want) {
			t.Errorf("response missing expected labeled option %q in the General settings locale select", want)
		}
	}
}
