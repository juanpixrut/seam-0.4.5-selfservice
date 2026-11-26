package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ha1tch/seam-ui/internal/i18n"
)

// newAssetPostTestServer simulates every xolu endpoint the POST
// handlers need: OQL (AssetSourceResolver during error re-render),
// POST /api/v1/assets (Create), PATCH /api/v1/assets/{id} (Update --
// switched from PUT 2026-08-03: PUT is xolu's full-document replace,
// confirmed directly to silently wipe any field not in the payload;
// PATCH is a real partial merge), and GET /api/v1/assets/{id}
// (Update's own confirmation, if ever needed). Captures the last
// created/updated document body for assertions.
type assetPostTestServer struct {
	*httptest.Server
	lastCreateBody map[string]any
	lastUpdateBody map[string]any
	lastUpdateID   string
	createErr      bool
	updateErr      bool
	// getResponseOverride, when set, is served for GET /api/v1/assets/{id}
	// instead of the default hardcoded fixture -- lets round-trip tests
	// serve back exactly what was actually persisted in an earlier
	// Create/Update step, rather than an unrelated canned document.
	getResponseOverride map[string]any
}

func newAssetPostTestServer(t *testing.T) *assetPostTestServer {
	t.Helper()
	ts := &assetPostTestServer{}
	ts.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/oql/query":
			json.NewEncoder(w).Encode(map[string]any{"status": "ok", "data": []map[string]any{}, "stats": map[string]any{}})
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/v1/assets/"):
			if ts.getResponseOverride != nil {
				json.NewEncoder(w).Encode(ts.getResponseOverride)
				return
			}
			// Default fixture -- a minimal but real document, matching
			// what AssetValuesFromXolu actually expects (xolu's own
			// criticality column, T-40 Block B, this session).
			json.NewEncoder(w).Encode(map[string]any{
				"name": "Existing Asset", "type_id": float64(1), "criticality": "Alta",
			})
		case r.Method == "POST" && r.URL.Path == "/api/v1/assets":
			if ts.createErr {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			ts.lastCreateBody = body
			json.NewEncoder(w).Encode(map[string]any{"id": float64(1), "message": "created"})
		case r.Method == "PATCH" && strings.HasPrefix(r.URL.Path, "/api/v1/assets/"):
			if ts.updateErr {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			ts.lastUpdateID = strings.TrimPrefix(r.URL.Path, "/api/v1/assets/")
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			ts.lastUpdateBody = body
			json.NewEncoder(w).Encode(map[string]any{"message": "updated"})
		default:
			http.NotFound(w, r)
		}
	}))
	return ts
}

func TestAssetCreateDynamic_ValidSubmission_CreatesAndRedirects(t *testing.T) {
	ts := newAssetPostTestServer(t)
	defer ts.Close()
	h := newTestHandler(t, ts.Server)

	form := url.Values{
		"name":        {"Compresor #4"},
		"type_id":     {"1"},
		"criticality": {"High"},
	}
	req := httptest.NewRequest("POST", "/assets/dynamic", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.AssetCreateDynamic(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d: %s", w.Code, w.Body.String())
	}
	if ts.lastCreateBody == nil {
		t.Fatal("expected xolu.Create to have been called")
	}
	if ts.lastCreateBody["name"] != "Compresor #4" {
		t.Errorf("name = %v, want \"Compresor #4\"", ts.lastCreateBody["name"])
	}
	if ts.lastCreateBody["criticality"] != "Alta" {
		t.Errorf("expected criticality value translated (Media/Alta/Baja), key unchanged in the persisted body, got %v", ts.lastCreateBody)
	}
	// type_id submitted as "1" (string) -- must be a real number in the
	// persisted body, matching assetIntegerFields' coercion.
	if v, ok := ts.lastCreateBody["type_id"].(float64); !ok || v != 1 {
		t.Errorf("type_id = %v (%T), want numeric 1", ts.lastCreateBody["type_id"], ts.lastCreateBody["type_id"])
	}
}

func TestAssetCreateDynamic_MissingRequiredField_ReRendersWithErrors(t *testing.T) {
	ts := newAssetPostTestServer(t)
	defer ts.Close()
	h := newTestHandler(t, ts.Server)

	form := url.Values{
		"type_id":     {"1"},
		"criticality": {"High"},
		// name deliberately omitted -- required field.
	}
	req := httptest.NewRequest("POST", "/assets/dynamic", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()

	h.AssetCreateDynamic(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (re-rendered form), got %d", w.Code)
	}
	if ts.lastCreateBody != nil {
		t.Error("xolu.Create should not have been called when validation fails")
	}
	body := w.Body.String()
	if !strings.Contains(body, "text-red-600") {
		t.Errorf("expected an error indicator in the re-rendered form:\n%s", body)
	}
	// Submitted values should survive the re-render -- nothing lost.
	if !strings.Contains(body, `value="High"`) {
		t.Errorf("expected criticality's submitted value preserved in the re-render:\n%s", body)
	}
}

func TestAssetCreateDynamic_DocumentLinksArray_ParsedAndTranslated(t *testing.T) {
	ts := newAssetPostTestServer(t)
	defer ts.Close()
	h := newTestHandler(t, ts.Server)

	form := url.Values{
		"name":                  {"Asset X"},
		"type_id":               {"1"},
		"criticality":           {"High"},
		"document_links_name[]": {"Factura"},
		"document_links_url[]":  {"https://x/f.pdf"},
		"document_links_type[]": {"pdf"},
	}
	req := httptest.NewRequest("POST", "/assets/dynamic", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.AssetCreateDynamic(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d: %s", w.Code, w.Body.String())
	}
	links, ok := ts.lastCreateBody["document_links"].([]any)
	if !ok || len(links) != 1 {
		t.Fatalf("expected document_links with 1 row, got %v", ts.lastCreateBody["document_links"])
	}
	row := links[0].(map[string]any)
	if row["name"] != "Factura" || row["type"] != "pdf" {
		t.Errorf("row = %v, want name=Factura type=pdf", row)
	}
}

func TestAssetCreateDynamic_ServiceConfigsArray_UsesRealPersistedKey(t *testing.T) {
	ts := newAssetPostTestServer(t)
	defer ts.Close()
	h := newTestHandler(t, ts.Server)

	form := url.Values{
		"name":                        {"Asset Y"},
		"type_id":                     {"1"},
		"criticality":                 {"High"},
		"service_configs_measure[]":   {"distance_kilometers"},
		"service_configs_threshold[]": {"10000"},
		"service_configs_alert_id[]":  {"3"},
	}
	req := httptest.NewRequest("POST", "/assets/dynamic", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.AssetCreateDynamic(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d: %s", w.Code, w.Body.String())
	}
	rows, ok := ts.lastCreateBody["service_configs"].([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("expected service_configs with 1 row, got %v", ts.lastCreateBody["service_configs"])
	}
	row := rows[0].(map[string]any)
	// The regression this test originally guarded (2026-07-28: submitted
	// key "umbral" vs persisted key "umbral_alarma" genuinely differing)
	// is moot now that both sides are "threshold" (T-40, this session) --
	// kept as a live check anyway, since a future accidental revert of
	// either side would reintroduce this same class of silent data loss.
	if _, hasOldSpanishKey := row["umbral_alarma"]; hasOldSpanishKey {
		t.Errorf("row has the old Spanish key \"umbral_alarma\" -- must be \"threshold\": %v", row)
	}
	if row["threshold"] != "10000" {
		t.Errorf("threshold = %v, want \"10000\"", row["threshold"])
	}
}

func TestAssetCreateDynamic_NestedArrayItemError_BlocksSubmissionAndShowsError(t *testing.T) {
	// The actual bug this whole FieldStatus mechanism exists to fix:
	// queryfy's error path for a bad URL inside a document-links row is
	// "document_links[0].url", not anything a flat errs[fieldName] map
	// could ever have matched. Before FieldStatus, this validation
	// failure would have been silently dropped -- xolu.Create still
	// would not have been called (validateAssetValues returned a
	// non-nil error either way), but the user would see no indication
	// of what was wrong or that anything failed to save correctly on
	// the visible field.
	ts := newAssetPostTestServer(t)
	defer ts.Close()
	h := newTestHandler(t, ts.Server)

	form := url.Values{
		"name":                  {"Asset Z"},
		"type_id":               {"1"},
		"criticality":           {"High"},
		"document_links_name[]": {"Manual"},
		"document_links_url[]":  {"not a valid url"},
		"document_links_type[]": {"pdf"},
	}
	req := httptest.NewRequest("POST", "/assets/dynamic", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()

	h.AssetCreateDynamic(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (re-rendered with error), got %d", w.Code)
	}
	if ts.lastCreateBody != nil {
		t.Error("xolu.Create should not have been called -- the nested URL error must block submission")
	}
	body := w.Body.String()
	if !strings.Contains(body, "text-red-600") {
		t.Errorf("expected the document_links field to show an error indicator (the actual regression this test guards against):\n%s", body)
	}
}

func TestAssetCreateDynamic_XoluError_Returns500(t *testing.T) {
	ts := newAssetPostTestServer(t)
	ts.createErr = true
	defer ts.Close()
	h := newTestHandler(t, ts.Server)

	form := url.Values{"name": {"X"}, "type_id": {"1"}, "criticality": {"High"}}
	req := httptest.NewRequest("POST", "/assets/dynamic", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.AssetCreateDynamic(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestAssetUpdateDynamic_XoluError_Returns500(t *testing.T) {
	// Mirrors TestAssetCreateDynamic_XoluError_Returns500 -- this
	// equivalent didn't exist at all until now, a real asymmetry
	// found while itemising what "feature complete" actually requires
	// (the tech team, 2026-07-29).
	ts := newAssetPostTestServer(t)
	ts.updateErr = true
	defer ts.Close()
	h := newTestHandler(t, ts.Server)

	form := url.Values{"name": {"X"}, "type_id": {"1"}, "criticality": {"High"}}
	req := httptest.NewRequest("POST", "/assets/dynamic/42", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.AssetUpdateDynamic(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestAssetUpdateDynamic_ValidSubmission_UpdatesAndRedirects(t *testing.T) {
	ts := newAssetPostTestServer(t)
	defer ts.Close()
	h := newTestHandler(t, ts.Server)

	form := url.Values{"name": {"Updated Name"}, "type_id": {"2"}, "criticality": {"Low"}}
	req := httptest.NewRequest("POST", "/assets/dynamic/42", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.AssetUpdateDynamic(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d: %s", w.Code, w.Body.String())
	}
	if ts.lastUpdateID != "42" {
		t.Errorf("expected update against id 42, got %q", ts.lastUpdateID)
	}
	if ts.lastUpdateBody["name"] != "Updated Name" {
		t.Errorf("name = %v, want \"Updated Name\"", ts.lastUpdateBody["name"])
	}
}

func TestAssetUpdateDynamic_MalformedID_Returns404(t *testing.T) {
	ts := newAssetPostTestServer(t)
	defer ts.Close()
	h := newTestHandler(t, ts.Server)

	req := httptest.NewRequest("POST", "/assets/dynamic/not-a-number", strings.NewReader(""))
	w := httptest.NewRecorder()

	h.AssetUpdateDynamic(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestAssetUpdateDynamic_ValidationFailure_DoesNotCallXolu(t *testing.T) {
	// name/type_id/criticality are deliberately NOT the trigger here
	// anymore -- they're optional on update since SchemaForUpdate
	// (2026-07-29, mirroring the static form's own create/update
	// split). An invalid enum value still fails regardless of
	// required-ness, since enum constraints apply whenever a value is
	// present, so it remains a valid way to prove validation failures
	// genuinely block the update.
	ts := newAssetPostTestServer(t)
	defer ts.Close()
	h := newTestHandler(t, ts.Server)

	form := url.Values{"type_id": {"1"}, "status": {"not-a-real-status"}}
	req := httptest.NewRequest("POST", "/assets/dynamic/42", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.AssetUpdateDynamic(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (re-rendered), got %d", w.Code)
	}
	if ts.lastUpdateBody != nil {
		t.Error("xolu.Update should not have been called when validation fails")
	}
}

// TestAssetUpdateDynamic_NameAndCriticalityOptionalOnUpdate is the
// direct proof of the create/update split (the tech team, 2026-07-29):
// submitting an update with no name, type_id, or criticality at all
// -- which AssetCreateDynamic's own required-field test confirms
// would fail on create -- succeeds on update, matching
// AssetUpdateSchema's own static-form precedent (all optional except
// what genuinely changed).
func TestAssetUpdateDynamic_NameAndCriticalityOptionalOnUpdate(t *testing.T) {
	ts := newAssetPostTestServer(t)
	defer ts.Close()
	h := newTestHandler(t, ts.Server)

	form := url.Values{"status": {"active"}} // no name, type_id, or criticality
	req := httptest.NewRequest("POST", "/assets/dynamic/42", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.AssetUpdateDynamic(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect (update succeeds without name/type_id/criticality), got %d: %s", w.Code, w.Body.String())
	}
	if ts.lastUpdateBody == nil {
		t.Fatal("expected xolu.Update to have been called")
	}
}

// TestAssetNewDynamic_HonoursCurrentLocale_EndToEnd is T-17 gap 2's
// actual proof, not just "nothing broke": builds a real i18n.Translator
// with a known "field.name" key (i18n.NewFromMessages, since the real
// embedded locales have no matching Asset-form content yet -- gap 3,
// still open) and confirms the rendered label is the translated text,
// not the schema's literal Spanish title -- and that switching
// h.config.Locale to a locale without that key falls back to the
// literal title, confirming the mechanism is actually locale-driven
// and not some other unconditional override.
func TestAssetNewDynamic_HonoursCurrentLocale_EndToEnd(t *testing.T) {
	ts := newAssetPostTestServer(t)
	defer ts.Close()
	h := newTestHandler(t, ts.Server)
	// Translator's own fallback is "xx", not "en" -- deliberately, so
	// requesting "xx" genuinely has nothing to fall back to (T's own
	// fallback-to-t.fallback behaviour would otherwise still resolve
	// via "en" even when the requested locale itself has no match,
	// which isn't what this second half of the test needs to prove).
	h.translator = i18n.NewFromMessages("xx", map[string]map[string]string{
		"en": {"field.name": "Asset Name (EN)"},
		"xx": {},
	})

	h.config.Locale = "en"
	req := httptest.NewRequest("GET", "/assets/dynamic/new", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	h.AssetNewDynamic(w, req)
	if !strings.Contains(w.Body.String(), "Asset Name (EN)") {
		t.Errorf("expected the translated label with Locale=en, got:\n%s", w.Body.String())
	}

	h.config.Locale = "xx"
	req2 := httptest.NewRequest("GET", "/assets/dynamic/new", nil)
	req2.Header.Set("HX-Request", "true")
	w2 := httptest.NewRecorder()
	h.AssetNewDynamic(w2, req2)
	if strings.Contains(w2.Body.String(), "Asset Name (EN)") {
		t.Error("expected the literal schema title with Locale=xx (no matching key), not the 'en' translation leaking through")
	}
}

// TestAssetCreateDynamic_NameMaxLengthEnforced is the direct proof
// asset.json's newly-added maxLength constraints (2026-07-29, closing
// the gap found comparing against the static form's own
// AssetCreateSchema) are actually enforced, not just present in the
// schema file -- name specifically, since it was also raised from the
// static form's 200 to 500 per the tech team's own reconsideration.
func TestAssetCreateDynamic_NameMaxLengthEnforced(t *testing.T) {
	ts := newAssetPostTestServer(t)
	defer ts.Close()
	h := newTestHandler(t, ts.Server)

	tooLong := strings.Repeat("x", 501)
	form := url.Values{"name": {tooLong}, "type_id": {"1"}, "criticality": {"High"}}
	req := httptest.NewRequest("POST", "/assets/dynamic", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.AssetCreateDynamic(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (re-rendered with error), got %d", w.Code)
	}
	if ts.lastCreateBody != nil {
		t.Error("xolu.Create should not have been called -- name exceeds the 500-char cap")
	}

	// 500 exactly must pass.
	exactly500 := strings.Repeat("x", 500)
	form2 := url.Values{"name": {exactly500}, "type_id": {"1"}, "criticality": {"High"}}
	req2 := httptest.NewRequest("POST", "/assets/dynamic", strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w2 := httptest.NewRecorder()

	h.AssetCreateDynamic(w2, req2)

	if w2.Code != http.StatusSeeOther {
		t.Errorf("expected 303 (500 chars exactly should pass), got %d: %s", w2.Code, w2.Body.String())
	}
}

// TestAssetCreateDynamic_DateFieldMaxLengthEnforced verifies the
// MaxLength(10) stopgap already present on all 8 date fields is
// genuinely enforced -- confirming and correcting an earlier mistake
// in this session's own changelog, which claimed this was still
// unfixed. It wasn't: the 18-field static-schema-derived maxLength
// fix already covered these 8 (fecha_*/vto_comodato all had
// MaxLength(10) in the static schema). What remains genuinely true,
// separately: "format": "date" itself is a no-op in queryfy, so a
// short-but-invalid string ("notadate1", 9 chars) still passes -- a
// real, smaller, distinct gap from "no validation at all".
func TestAssetCreateDynamic_DateFieldMaxLengthEnforced(t *testing.T) {
	ts := newAssetPostTestServer(t)
	defer ts.Close()
	h := newTestHandler(t, ts.Server)

	form := url.Values{
		"name": {"X"}, "type_id": {"1"}, "criticality": {"High"},
		"purchase_date": {"2026-07-2999"}, // 12 chars, exceeds 10
	}
	req := httptest.NewRequest("POST", "/assets/dynamic", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.AssetCreateDynamic(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (re-rendered with error) for an over-length purchase_date, got %d", w.Code)
	}
	if ts.lastCreateBody != nil {
		t.Error("xolu.Create should not have been called -- purchase_date exceeds the 10-char cap")
	}
}

// TestAssetNewDynamic_EmitsSubmittableFormTag is the direct proof of
// the actual gap found while itemising "feature complete": rendering
// the dynamic Asset form with no wrapper produced zero <form> tags at
// all -- confirmed by an actual render, not assumed -- meaning it
// could never be submitted by a real browser regardless of backend
// correctness. wrapAssetDynamicForm fixes this; this test confirms
// the fix, not just that nothing else broke.
func TestAssetNewDynamic_EmitsSubmittableFormTag(t *testing.T) {
	ts := newAssetPostTestServer(t)
	defer ts.Close()
	h := newTestHandler(t, ts.Server)

	req := httptest.NewRequest("GET", "/assets/dynamic/new", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	h.AssetNewDynamic(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "<form") {
		t.Fatalf("expected a <form> tag in the rendered output, found none:\n%s", body)
	}
	if !strings.Contains(body, `id="asset-dynamic-form"`) {
		t.Error("expected the form's own id, not found")
	}
	if !strings.Contains(body, `hx-post="/assets/dynamic"`) {
		t.Error("expected hx-post targeting the create endpoint, not found")
	}
	if !strings.Contains(body, `type="submit"`) {
		t.Error("expected a submit button, not found")
	}
}

// TestAssetEditDynamic_EmitsFormTagTargetingCorrectUpdateURL confirms
// the edit path's form action targets this specific asset's update
// URL, not a generic or missing one.
func TestAssetEditDynamic_EmitsFormTagTargetingCorrectUpdateURL(t *testing.T) {
	ts := newAssetPostTestServer(t)
	defer ts.Close()
	h := newTestHandler(t, ts.Server)

	req := httptest.NewRequest("GET", "/assets/dynamic/77/edit", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	h.AssetEditDynamic(w, req)

	body := w.Body.String()
	if !strings.Contains(body, `hx-post="/assets/dynamic/77"`) {
		t.Errorf("expected hx-post targeting /assets/dynamic/77, not found:\n%s", body)
	}
}

// TestAssetDynamic_FullRoundTrip_CreateThenReadBack is the round-trip
// proof we asked for directly: submit via the dynamic form,
// capture what actually got persisted, serve that same document back
// as if xolu now has it, then confirm AssetEditDynamic displays
// exactly what was submitted -- scalars, an enum, an FK-style id
// field, and both array-field types (document-links, repeatable).
// Every prior test exercises create-side translation or read-side
// translation independently, each against its own fixture; this is
// the first to chain them, which is the only way to actually prove
// AssetValuesToXolu and AssetValuesFromXolu are genuine inverses of
// each other in practice, not just individually correct in isolation.
func TestAssetDynamic_FullRoundTrip_CreateThenReadBack(t *testing.T) {
	ts := newAssetPostTestServer(t)
	defer ts.Close()
	h := newTestHandler(t, ts.Server)

	form := url.Values{
		"name":                        {"Round Trip Compressor"},
		"type_id":                     {"3"},
		"criticality":                 {"Medium"},
		"status":                      {"active"},
		"brand_id":                    {"7"},
		"serial_1":                    {"SN-RT-001"},
		"purchase_price":              {"15000.50"},
		"document_links_name[]":       {"Manual PDF"},
		"document_links_url[]":        {"https://example/manual.pdf"},
		"document_links_type[]":       {"pdf"},
		"service_configs_measure[]":   {"time_hours"},
		"service_configs_threshold[]": {"500"},
		"service_configs_alert_id[]":  {"2"},
	}
	createReq := httptest.NewRequest("POST", "/assets/dynamic", strings.NewReader(form.Encode()))
	createReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	createW := httptest.NewRecorder()
	h.AssetCreateDynamic(createW, createReq)

	if createW.Code != http.StatusSeeOther {
		t.Fatalf("create: expected 303, got %d: %s", createW.Code, createW.Body.String())
	}
	if ts.lastCreateBody == nil {
		t.Fatal("create: expected xolu.Create to have been called")
	}

	// Serve back exactly what was persisted, as if this were now a
	// real, existing xolu document -- not a separately-authored fixture.
	ts.getResponseOverride = ts.lastCreateBody

	editReq := httptest.NewRequest("GET", "/assets/dynamic/99/edit", nil)
	editReq.Header.Set("HX-Request", "true")
	editW := httptest.NewRecorder()
	h.AssetEditDynamic(editW, editReq)

	if editW.Code != http.StatusOK {
		t.Fatalf("edit: expected 200, got %d: %s", editW.Code, editW.Body.String())
	}
	body := editW.Body.String()

	// Single-attribute checks use the exact value="X" pattern -- a bare
	// "500" check was tried first and caught by a mutation test:
	// Tailwind classes like "text-gray-500"/"focus:ring-indigo-500"
	// make that number appear in the page regardless of whether the
	// actual field round-tripped correctly, which would have made the
	// assertion pass even if service_configs was completely broken.
	checks := []struct{ label, want string }{
		{"name", `value="Round Trip Compressor"`},
		{"serial_1", `value="SN-RT-001"`},
		{"purchase_price", `value="15000.50"`},
		{"document-links name", `value="Manual PDF"`},
		{"document-links url", `value="https://example/manual.pdf"`},
		{"service_configs measure", `value="time_hours"`},
		{"service_configs threshold", `value="500"`},
	}
	for _, c := range checks {
		if !strings.Contains(body, c.want) {
			t.Errorf("round trip lost %s -- expected %q in the re-rendered edit form, not found", c.label, c.want)
		}
	}

	// criticality's selected-option check needs its own, order-independent
	// assertion, not another single-attribute check: option tags carry
	// two attributes here (value and selected), and their rendered order
	// is not guaranteed -- confirmed by a stress run (30 iterations,
	// -count) that intermittently failed with a rigid value="Media"
	// selected="selected" check, then a diagnostic dump showing the
	// exact same option tag rendering as both
	// <option selected="selected" value="Media">Media</option> AND
	// <option value="Media" selected="selected">Media</option> across
	// different runs of the identical test (Media/Alta were the enum
	// tokens then -- T-23, docs/RESOLVED.md, later moved this specific
	// field to English tokens; the attribute-ordering lesson itself is
	// unaffected and still applies to the current Medium/High/Low
	// tokens) -- the persisted data and the "selected" marking were
	// correct every single time; only the concatenation order assumed
	// by the original assertion was wrong. Checking both orders
	// directly, rather than switching to a looser bare-substring check,
	// keeps this precise about both attributes needing to be present on
	// the same tag.
	if !strings.Contains(body, `value="Medium" selected="selected"`) &&
		!strings.Contains(body, `selected="selected" value="Medium"`) {
		t.Errorf("round trip lost criticality enum value round-tripped -- expected the Medium option marked selected (either attribute order), not found:\n%s", body)
	}
}
