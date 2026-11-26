package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newAssetDynamicTestServer simulates both endpoints AssetEditDynamic
// needs: OQL (for AssetSourceResolver's option lookups during render)
// and Get (for loading the asset itself). oqlRows may be nil for tests
// that don't care about populated dropdown options.
func newAssetDynamicTestServer(t *testing.T, assetID int64, assetData map[string]any, oqlRows map[string][]map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/oql/query":
			var body struct {
				Query string `json:"query"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			json.NewEncoder(w).Encode(map[string]any{
				"status": "ok",
				"data":   oqlRows[body.Query],
				"stats":  map[string]any{},
			})
		case strings.HasPrefix(r.URL.Path, "/api/v1/assets/"):
			if assetData == nil {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			json.NewEncoder(w).Encode(assetData)
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestAssetNewDynamic_HTMXRequest_RendersForm(t *testing.T) {
	srv := newAssetDynamicTestServer(t, 0, nil, nil)
	defer srv.Close()
	h := newTestHandler(t, srv)

	req := httptest.NewRequest("GET", "/assets/dynamic/new", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()

	h.AssetNewDynamic(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, `name="name"`) {
		t.Errorf("expected the name field in the rendered form:\n%s", body)
	}
	if strings.Contains(body, "text-red-600") {
		t.Errorf("rendered output contains a real formengine error:\n%s", body)
	}
}

func TestAssetNewDynamic_NonHTMXRequest_Redirects(t *testing.T) {
	srv := newAssetDynamicTestServer(t, 0, nil, nil)
	defer srv.Close()
	h := newTestHandler(t, srv)

	req := httptest.NewRequest("GET", "/assets/dynamic/new", nil)
	w := httptest.NewRecorder()

	h.AssetNewDynamic(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("expected a redirect (303), got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/assets?create=1" {
		t.Errorf("expected redirect to /assets?create=1, got %q", loc)
	}
}

func TestAssetEditDynamic_LoadsAndTranslatesRealAsset(t *testing.T) {
	assetData := map[string]any{
		"id":         float64(42),
		"name":       "Compresor #4",
		"type_id":    float64(1),
		"criticidad": "Alta",
		"created_at": "2026-01-01T00:00:00Z",
		"updated_at": "2026-01-01T00:00:00Z",
	}
	srv := newAssetDynamicTestServer(t, 42, assetData, nil)
	defer srv.Close()
	h := newTestHandler(t, srv)

	req := httptest.NewRequest("GET", "/assets/dynamic/42/edit", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()

	h.AssetEditDynamic(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	// criticidad (xolu-persisted, Spanish) must have come through
	// AssetValuesFromXolu as the "criticality" (English, schema-side)
	// field's value -- the actual point of this handler existing.
	if !strings.Contains(body, `value="Compresor #4"`) {
		t.Errorf("expected the asset's name pre-filled:\n%s", body)
	}
	if !strings.Contains(body, "High") {
		t.Errorf("expected criticidad's value (Alta) translated to High in the criticality field:\n%s", body)
	}
}

func TestAssetEditDynamic_UnknownID_Returns404(t *testing.T) {
	srv := newAssetDynamicTestServer(t, 0, nil, nil)
	defer srv.Close()
	h := newTestHandler(t, srv)

	req := httptest.NewRequest("GET", "/assets/dynamic/999/edit", nil)
	w := httptest.NewRecorder()

	h.AssetEditDynamic(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestAssetEditDynamic_MalformedID_Returns404(t *testing.T) {
	srv := newAssetDynamicTestServer(t, 0, nil, nil)
	defer srv.Close()
	h := newTestHandler(t, srv)

	req := httptest.NewRequest("GET", "/assets/dynamic/not-a-number/edit", nil)
	w := httptest.NewRecorder()

	h.AssetEditDynamic(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for a non-numeric id, got %d", w.Code)
	}
}
