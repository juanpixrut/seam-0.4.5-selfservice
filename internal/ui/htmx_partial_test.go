package ui

// htmx_partial_test.go — tests for SeamModal2 partial rendering.
//
// Each entity that supports in-context editing via SeamModal2 must satisfy:
//   1. Edit (GET) with HX-Request header → returns partial fragment (no <html>)
//   2. Edit (GET) without HX-Request header → returns full page (has <html>)
//   3. Update (POST) with HX-Request + valid data → 200 + HX-Trigger:seamModal2Saved
//   4. Update (POST) with HX-Request + invalid data → partial with error (no <html>)
//   5. Summary (GET) → 200 with HTML card containing key field
//   6. Summary (GET) ID=0 → 404
//
// Entities covered: Marca, CentroCosto, Custodio, Modelo, Ubicacion, AssetType.

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// htmxGet fires a GET request with the HX-Request header set.
func htmxGet(t *testing.T, h http.HandlerFunc, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

// htmxPost fires a form POST with the HX-Request header set.
func htmxPost(t *testing.T, h http.HandlerFunc, path string, fields url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", path, strings.NewReader(fields.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

// oqlEntity returns a mock GET /api/v1/{collection}/{id} response handler.
func mockEntityGet(data map[string]any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
	}
}

// oqlPutOK returns a 200 OK response for a PUT (update) call.
func oqlPutOK(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// captureBody reads all bytes from the response recorder.
func captureBody(rec *httptest.ResponseRecorder) string {
	return rec.Body.String()
}

// assertPartial verifies the response is a partial (no <html> tag).
func assertPartial(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK for partial, got %d", rec.Code)
	}
	body := captureBody(rec)
	if strings.Contains(strings.ToLower(body), "<html") {
		t.Error("expected partial response (no <html>), but got full page")
	}
}

// assertFullPage verifies the response contains a full HTML page.
func assertFullPage(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", rec.Code)
	}
	body := captureBody(rec)
	if !strings.Contains(strings.ToLower(body), "<html") {
		t.Error("expected full page response (with <html>), but got partial")
	}
}

// assertHTMXSaved verifies the 200 + seamModal2Saved trigger on update success.
func assertHTMXSaved(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK for HTMX save, got %d; body: %s", rec.Code, rec.Body.String())
	}
	trigger := rec.Header().Get("HX-Trigger")
	if trigger != "seamModal2Saved" {
		t.Errorf("expected HX-Trigger: seamModal2Saved, got %q", trigger)
	}
}

// assertPartialContains verifies a partial response contains a substring.
func assertPartialContains(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	assertPartial(t, rec)
	body := captureBody(rec)
	if !strings.Contains(body, want) {
		t.Errorf("partial body does not contain %q\nbody: %s", want, body[:min(300, len(body))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// mockOLUForEntity builds a mock XOLU that serves GET entity data and accepts PUT.
func mockOLUForEntity(t *testing.T, collection string, entityData map[string]any) *httptest.Server {
	t.Helper()
	return mockOLU(t, map[string]http.HandlerFunc{
		"GET /api/v1/" + collection + "/":   mockEntityGet(entityData),
		"PATCH /api/v1/" + collection + "/": oqlPutOK,
		"POST /api/v1":                      func(w http.ResponseWriter, r *http.Request) { oqlEmpty(w) },
	})
}

// ── Marca ─────────────────────────────────────────────────────────────────────

func TestMarcaEdit_HTMXReturnsPartial(t *testing.T) {
	srv := mockOLUForEntity(t, "brands", map[string]any{"id": 1.0, "code": "ACM", "name": "ACME Corp"})
	defer srv.Close()
	h := newHandler(t, srv)

	rec := htmxGet(t, h.MarcaEdit, "/brands/1/edit")
	assertPartial(t, rec)
	if strings.Contains(captureBody(rec), "seam-modal-body") {
		t.Error("partial should not reference seam-modal-body directly")
	}
}

func TestMarcaEdit_FullPageWithoutHTMX(t *testing.T) {
	srv := mockOLUForEntity(t, "brands", map[string]any{"id": 1.0, "code": "ACM", "name": "ACME Corp"})
	defer srv.Close()
	h := newHandler(t, srv)

	req := httptest.NewRequest("GET", "/brands/1/edit", nil)
	rec := httptest.NewRecorder()
	h.MarcaEdit(rec, req)
	assertFullPage(t, rec)
}

func TestMarcaUpdate_HTMXValidDataTriggersClose(t *testing.T) {
	srv := mockOLUForEntity(t, "brands", nil)
	defer srv.Close()
	h := newHandler(t, srv)

	rec := htmxPost(t, h.MarcaUpdate, "/brands/1",
		url.Values{"code": {"ACM"}, "name": {"ACME Corp"}})
	assertHTMXSaved(t, rec)
}

func TestMarcaUpdate_HTMXValidationErrorReturnsPartial(t *testing.T) {
	srv := mockOLU(t, nil)
	defer srv.Close()
	h := newHandler(t, srv)

	rec := htmxPost(t, h.MarcaUpdate, "/brands/1",
		url.Values{"code": {""}, "name": {""}}) // both required — fails validation
	assertPartial(t, rec)
}

// ── CentroCosto ───────────────────────────────────────────────────────────────

func TestCentroCostoEdit_HTMXReturnsPartial(t *testing.T) {
	srv := mockOLUForEntity(t, "cost_centers", map[string]any{"id": 1.0, "accounting_code": "6100", "name": "IT"})
	defer srv.Close()
	h := newHandler(t, srv)

	rec := htmxGet(t, h.CentroCostoEdit, "/cost-centers/1/edit")
	assertPartial(t, rec)
}

func TestCentroCostoEdit_FullPageWithoutHTMX(t *testing.T) {
	srv := mockOLUForEntity(t, "cost_centers", map[string]any{"id": 1.0, "accounting_code": "6100", "name": "IT"})
	defer srv.Close()
	h := newHandler(t, srv)

	req := httptest.NewRequest("GET", "/cost-centers/1/edit", nil)
	rec := httptest.NewRecorder()
	h.CentroCostoEdit(rec, req)
	assertFullPage(t, rec)
}

func TestCentroCostoUpdate_HTMXSuccess(t *testing.T) {
	srv := mockOLUForEntity(t, "cost_centers", nil)
	defer srv.Close()
	h := newHandler(t, srv)

	rec := htmxPost(t, h.CentroCostoUpdate, "/cost-centers/1",
		url.Values{"accounting_code": {"6100"}, "name": {"IT"}})
	assertHTMXSaved(t, rec)
}

func TestCentroCostoUpdate_HTMXValidationError(t *testing.T) {
	srv := mockOLU(t, nil)
	defer srv.Close()
	h := newHandler(t, srv)

	rec := htmxPost(t, h.CentroCostoUpdate, "/cost-centers/1",
		url.Values{"accounting_code": {""}, "name": {""}})
	assertPartial(t, rec)
}

// ── Custodio ──────────────────────────────────────────────────────────────────

var custodioEntity = map[string]any{
	"id": 1.0, "subject_type": "RUT", "legal_name": "INTEC S.A.",
	"tax_id": "21567890-1", "email": "activos@intec.com.uy",
	"active": true,
}

func TestCustodioEdit_HTMXReturnsPartial(t *testing.T) {
	srv := mockOLUForEntity(t, "custodians", custodioEntity)
	defer srv.Close()
	h := newHandler(t, srv)

	rec := htmxGet(t, h.CustodioEdit, "/custodians/1/edit")
	assertPartial(t, rec)
}

func TestCustodioEdit_FullPageWithoutHTMX(t *testing.T) {
	srv := mockOLUForEntity(t, "custodians", custodioEntity)
	defer srv.Close()
	h := newHandler(t, srv)

	req := httptest.NewRequest("GET", "/custodians/1/edit", nil)
	rec := httptest.NewRecorder()
	h.CustodioEdit(rec, req)
	assertFullPage(t, rec)
}

func TestCustodioUpdate_HTMXSuccess(t *testing.T) {
	srv := mockOLUForEntity(t, "custodians", nil)
	defer srv.Close()
	h := newHandler(t, srv)

	rec := htmxPost(t, h.CustodioUpdate, "/custodians/1", url.Values{
		"subject_type": {"RUT"},
		"legal_name":   {"INTEC S.A."},
		"tax_id":       {"21567890-1"},
		"email":        {"activos@intec.com.uy"},
	})
	assertHTMXSaved(t, rec)
}

func TestCustodioUpdate_HTMXValidationError(t *testing.T) {
	srv := mockOLU(t, nil)
	defer srv.Close()
	h := newHandler(t, srv)

	// Missing required fields → validation error
	rec := htmxPost(t, h.CustodioUpdate, "/custodians/1",
		url.Values{"subject_type": {"RUT"}, "legal_name": {""}, "email": {""}})
	assertPartial(t, rec)
}

// Verify the correct data is sent to XOLU on a successful Custodio update.
func TestCustodioUpdate_DataIntegrity(t *testing.T) {
	var capturedBody []byte
	srv := mockOLU(t, map[string]http.HandlerFunc{
		"PATCH /api/v1/custodians/": func(w http.ResponseWriter, r *http.Request) {
			capturedBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
		},
	})
	defer srv.Close()
	h := newHandler(t, srv)

	htmxPost(t, h.CustodioUpdate, "/custodians/1", url.Values{
		"subject_type": {"RUT"},
		"legal_name":   {"Frigocold S.A."},
		"tax_id":       {"210045678-9"},
		"email":        {"admin@frigocold.com"},
		"phone":        {"+598 2 900 1234"},
		"active":       {"on"},
	})

	if len(capturedBody) == 0 {
		t.Fatal("XOLU PUT was never called — data was not saved")
	}
	var sent map[string]any
	if err := json.Unmarshal(capturedBody, &sent); err != nil {
		t.Fatalf("PUT body is not valid JSON: %v\nbody: %s", err, capturedBody)
	}
	checks := map[string]string{
		"legal_name":   "Frigocold S.A.",
		"tax_id":       "210045678-9",
		"email":        "admin@frigocold.com",
		"phone":        "+598 2 900 1234",
		"subject_type": "RUT",
	}
	for key, want := range checks {
		got, ok := sent[key]
		if !ok {
			t.Errorf("PUT body missing field %q", key)
			continue
		}
		if got != want {
			t.Errorf("field %q: got %q, want %q", key, got, want)
		}
	}
	if got, ok := sent["active"].(bool); !ok || !got {
		t.Errorf("field 'active' should be true, got %v", sent["active"])
	}
}

// ── Modelo ────────────────────────────────────────────────────────────────────

func TestModeloEdit_HTMXReturnsPartial(t *testing.T) {
	srv := mockOLU(t, map[string]http.HandlerFunc{
		"GET /api/v1/models/": mockEntityGet(map[string]any{"id": 1.0, "brand_id": 2.0, "name": "Modelo X"}),
		"POST /api/v1":        func(w http.ResponseWriter, r *http.Request) { oqlEmpty(w) }, // loadMarcas OQL
	})
	defer srv.Close()
	h := newHandler(t, srv)

	rec := htmxGet(t, h.ModeloEdit, "/models/1/edit")
	assertPartial(t, rec)
}

func TestModeloUpdate_HTMXSuccess(t *testing.T) {
	srv := mockOLU(t, map[string]http.HandlerFunc{
		"PATCH /api/v1/models/": oqlPutOK,
		"POST /api/v1":          func(w http.ResponseWriter, r *http.Request) { oqlEmpty(w) },
	})
	defer srv.Close()
	h := newHandler(t, srv)

	rec := htmxPost(t, h.ModeloUpdate, "/models/1",
		url.Values{"marca_id": {"2"}, "nombre": {"Modelo X Pro"}})
	assertHTMXSaved(t, rec)
}

func TestModeloUpdate_DataIntegrity(t *testing.T) {
	var capturedBody []byte
	srv := mockOLU(t, map[string]http.HandlerFunc{
		"PATCH /api/v1/models/": func(w http.ResponseWriter, r *http.Request) {
			capturedBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
		},
		"POST /api/v1": func(w http.ResponseWriter, r *http.Request) { oqlEmpty(w) },
	})
	defer srv.Close()
	h := newHandler(t, srv)

	htmxPost(t, h.ModeloUpdate, "/models/1",
		url.Values{"marca_id": {"3"}, "nombre": {"Chiller Serie 5"}})

	if len(capturedBody) == 0 {
		t.Fatal("XOLU PUT was never called")
	}
	var sent map[string]any
	json.Unmarshal(capturedBody, &sent)
	if sent["name"] != "Chiller Serie 5" {
		t.Errorf("name: got %v, want Chiller Serie 5", sent["name"])
	}
	if int(sent["brand_id"].(float64)) != 3 {
		t.Errorf("brand_id: got %v, want 3", sent["brand_id"])
	}
}

// ── Ubicacion ─────────────────────────────────────────────────────────────────

func TestUbicacionEdit_HTMXReturnsPartial(t *testing.T) {
	srv := mockOLU(t, map[string]http.HandlerFunc{
		"GET /api/v1/locations/": mockEntityGet(map[string]any{
			"id": 1.0, "short_name": "Sala Srv A", "description": "Server room", "region_id": 1.0,
		}),
		"POST /api/v1": func(w http.ResponseWriter, r *http.Request) { oqlEmpty(w) }, // loadRegiones
	})
	defer srv.Close()
	h := newHandler(t, srv)

	rec := htmxGet(t, h.UbicacionEdit, "/locations/1/edit")
	assertPartial(t, rec)
}

func TestUbicacionUpdate_HTMXSuccess(t *testing.T) {
	srv := mockOLU(t, map[string]http.HandlerFunc{
		"PATCH /api/v1/locations/": oqlPutOK,
		"POST /api/v1":             func(w http.ResponseWriter, r *http.Request) { oqlEmpty(w) },
	})
	defer srv.Close()
	h := newHandler(t, srv)

	rec := htmxPost(t, h.UbicacionUpdate, "/locations/1",
		url.Values{"short_name": {"Sala Srv A"}, "description": {"Server room"}, "region_id": {"1"}})
	assertHTMXSaved(t, rec)
}

// ── AssetType ─────────────────────────────────────────────────────────────────

func TestAssetTypeEdit_HTMXReturnsPartial(t *testing.T) {
	srv := mockOLU(t, map[string]http.HandlerFunc{
		"GET /api/v1/asset_types/": mockEntityGet(map[string]any{
			"id": 1.0, "name": "Chiller", "slug": "chiller",
			"description": "Industrial chiller", "color": "#4f46e5",
		}),
		"POST /api/v1": func(w http.ResponseWriter, r *http.Request) { oqlEmpty(w) }, // loadAlertas
	})
	defer srv.Close()
	h := newHandler(t, srv)

	rec := htmxGet(t, h.AssetTypeEdit, "/asset-types/1/edit")
	assertPartial(t, rec)
}

func TestAssetTypeEdit_FullPageWithoutHTMX(t *testing.T) {
	srv := mockOLU(t, map[string]http.HandlerFunc{
		"GET /api/v1/asset_types/": mockEntityGet(map[string]any{
			"id": 1.0, "name": "Chiller", "slug": "chiller",
		}),
		"POST /api/v1": func(w http.ResponseWriter, r *http.Request) { oqlEmpty(w) },
	})
	defer srv.Close()
	h := newHandler(t, srv)

	req := httptest.NewRequest("GET", "/asset-types/1/edit", nil)
	rec := httptest.NewRecorder()
	h.AssetTypeEdit(rec, req)
	assertFullPage(t, rec)
}

func TestAssetTypeUpdate_HTMXSuccess(t *testing.T) {
	srv := mockOLU(t, map[string]http.HandlerFunc{
		"PATCH /api/v1/asset_types/": oqlPutOK,
		"POST /api/v1":               func(w http.ResponseWriter, r *http.Request) { oqlEmpty(w) },
	})
	defer srv.Close()
	h := newHandler(t, srv)

	rec := htmxPost(t, h.AssetTypeUpdate, "/asset-types/1",
		url.Values{"name": {"Chiller"}, "description": {"Industrial chiller"}, "color": {"#4f46e5"}})
	assertHTMXSaved(t, rec)
}

// ── Summary endpoints ─────────────────────────────────────────────────────────

func TestCustodioSummary_ContainsName(t *testing.T) {
	srv := mockOLUForEntity(t, "custodians", map[string]any{
		"id": 1.0, "legal_name": "Frigocold S.A.",
		"subject_type": "RUT", "tax_id": "210045678-9",
		"email": "admin@frigocold.com", "active": true,
	})
	defer srv.Close()
	h := newHandler(t, srv)

	req := httptest.NewRequest("GET", "/custodians/1/summary", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	h.CustodioSummary(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := captureBody(rec)
	if !strings.Contains(body, "Frigocold S.A.") {
		t.Errorf("summary should contain entity name; body: %s", body[:min(200, len(body))])
	}
}

func TestCustodioSummary_NotFoundForZeroID(t *testing.T) {
	srv := mockOLU(t, nil)
	defer srv.Close()
	h := newHandler(t, srv)

	req := httptest.NewRequest("GET", "/custodians/0/summary", nil)
	rec := httptest.NewRecorder()
	h.CustodioSummary(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for ID=0, got %d", rec.Code)
	}
}

func TestMarcaSummary_ContainsCode(t *testing.T) {
	srv := mockOLUForEntity(t, "brands", map[string]any{
		"id": 2.0, "code": "LGR", "name": "LG Refrigeration",
	})
	defer srv.Close()
	h := newHandler(t, srv)

	req := httptest.NewRequest("GET", "/brands/2/summary", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	h.MarcaSummary(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(captureBody(rec), "LGR") {
		t.Error("marca summary should contain the codigo field")
	}
}

func TestCentroCostoSummary_ContainsCode(t *testing.T) {
	srv := mockOLUForEntity(t, "cost_centers", map[string]any{
		"id": 3.0, "accounting_code": "6200", "name": "Facilities",
	})
	defer srv.Close()
	h := newHandler(t, srv)

	req := httptest.NewRequest("GET", "/cost-centers/3/summary", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	h.CentroCostoSummary(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(captureBody(rec), "6200") {
		t.Error("centro de costo summary should contain the codigo_contable")
	}
}

func TestAssetTypeSummary_ContainsName(t *testing.T) {
	srv := mockOLUForEntity(t, "asset_types", map[string]any{
		"id": 1.0, "name": "Chiller", "description": "Industrial chiller unit",
	})
	defer srv.Close()
	h := newHandler(t, srv)

	req := httptest.NewRequest("GET", "/asset-types/1/summary", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	h.AssetTypeSummary(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := captureBody(rec)
	if !strings.Contains(body, "Chiller") {
		t.Error("asset type summary should contain the name")
	}
	// editURL must point to the edit form (/edit suffix), not the detail page.
	// Without /edit the link opens AssetTypeDetail (full page with nav/logo).
	if !strings.Contains(body, "/asset-types/1/edit") {
		t.Error("asset type summary editURL must use /asset-types/{id}/edit, not /asset-types/{id}")
	}
}

// ── Non-HTMX update redirects (regression: ensure full-page path untouched) ──

func TestMarcaUpdate_NonHTMXRedirects(t *testing.T) {
	srv := mockOLUForEntity(t, "brands", nil)
	defer srv.Close()
	h := newHandler(t, srv)

	rec := postForm(t, h.MarcaUpdate, "/brands/1",
		url.Values{"code": {"ACM"}, "name": {"ACME Corp"}})

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303 SeeOther for non-HTMX success, got %d", rec.Code)
	}
}

func TestCentroCostoUpdate_NonHTMXRedirects(t *testing.T) {
	srv := mockOLUForEntity(t, "cost_centers", nil)
	defer srv.Close()
	h := newHandler(t, srv)

	rec := postForm(t, h.CentroCostoUpdate, "/cost-centers/1",
		url.Values{"accounting_code": {"6100"}, "name": {"IT"}})

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303 SeeOther for non-HTMX success, got %d", rec.Code)
	}
}

func TestCustodioUpdate_NonHTMXRedirects(t *testing.T) {
	srv := mockOLUForEntity(t, "custodians", nil)
	defer srv.Close()
	h := newHandler(t, srv)

	rec := postForm(t, h.CustodioUpdate, "/custodians/1", url.Values{
		"subject_type": {"RUT"},
		"legal_name":   {"INTEC S.A."},
		"tax_id":       {"21567890-1"},
		"email":        {"activos@intec.com.uy"},
	})

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303 SeeOther for non-HTMX success, got %d", rec.Code)
	}
}
