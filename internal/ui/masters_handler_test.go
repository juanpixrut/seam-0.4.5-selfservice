package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	xolu "github.com/ha1tch/xolu/pkg/client"
)

// ── Shared helpers ────────────────────────────────────────────────────────────

// mockOLU builds an httptest.Server that dispatches by method+path prefix.
func mockOLU(t *testing.T, routes map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		for pattern, handler := range routes {
			if matchPrefix(key, pattern) {
				handler(w, r)
				return
			}
		}
		// Unmatched — return a valid OQL empty result so handlers don't crash
		if r.Method == "POST" && containsHelper(r.URL.Path, "/oql") {
			oqlEmpty(w)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
}

func matchPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func oqlEmpty(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status": "completed",
		"data":   []map[string]any{},
		"stats":  map[string]any{"rows_returned": 0},
	})
}

func oqlCount(w http.ResponseWriter, count int) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status": "completed",
		"data":   []map[string]any{{"count": float64(count)}},
		"stats":  map[string]any{"rows_returned": 1},
	})
}

func oqlCreated(w http.ResponseWriter, id int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{"id": float64(id), "message": "created"})
}

func newHandler(t *testing.T, server *httptest.Server) *UIHandler {
	t.Helper()
	client := xolu.New(server.URL)
	cfg := Config{AppName: "Test", Version: "0.0.0"}
	return NewUIHandler(client, cfg, nil, nil, nil, nil, nil, nil, nil)
}

func postForm(t *testing.T, h http.HandlerFunc, target string, fields url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", target, strings.NewReader(fields.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

// ── Test 13: RegionCreate redirects on success ────────────────────────────────

func TestRegionCreate_Redirects(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{
		"POST /api/v1": func(w http.ResponseWriter, r *http.Request) {
			oqlCreated(w, 1)
		},
	})
	defer server.Close()

	h := newHandler(t, server)
	rec := postForm(t, h.RegionCreate, "/regions",
		url.Values{"description": {"Región Test"}})

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303 SeeOther, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/regions" {
		t.Errorf("expected redirect to /regions, got %q", loc)
	}
}

// ── Test 14: RegionDelete returns 404 for invalid ID ─────────────────────────

func TestRegionDelete_NotFound(t *testing.T) {
	server := mockOLU(t, nil)
	defer server.Close()

	h := newHandler(t, server)

	req := httptest.NewRequest("POST", "/regions/0/delete", nil)
	rec := httptest.NewRecorder()
	h.RegionDelete(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for ID=0, got %d", rec.Code)
	}
}

// ── Test 15: RegionDelete returns 409 when region is in use ──────────────────

func TestRegionDelete_ConflictGuard(t *testing.T) {
	// OQL will return count > 0 for both ubicaciones and assets queries
	server := mockOLU(t, map[string]http.HandlerFunc{
		"POST /api/v1": func(w http.ResponseWriter, r *http.Request) {
			oqlCount(w, 2) // 2 ubicaciones reference this region
		},
	})
	defer server.Close()

	h := newHandler(t, server)

	req := httptest.NewRequest("POST", "/regions/1/delete", nil)
	rec := httptest.NewRecorder()
	h.RegionDelete(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict when region is in use, got %d", rec.Code)
	}
}

// ── Test 16: RegionDelete redirects on success ────────────────────────────────

func TestRegionDelete_Success(t *testing.T) {
	callCount := 0
	server := mockOLU(t, map[string]http.HandlerFunc{
		"POST /api/v1": func(w http.ResponseWriter, r *http.Request) {
			// All OQL count queries return 0 (no references)
			oqlCount(w, 0)
		},
		"DELETE /api/v1": func(w http.ResponseWriter, r *http.Request) {
			callCount++
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer server.Close()

	h := newHandler(t, server)

	req := httptest.NewRequest("POST", "/regions/1/delete", nil)
	rec := httptest.NewRecorder()
	h.RegionDelete(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303 SeeOther on successful delete, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/regions" {
		t.Errorf("expected redirect to /regions, got %q", loc)
	}
	if callCount == 0 {
		t.Error("xolu DELETE was never called")
	}
}

// ── Additional: MarcaDelete conflict guard ────────────────────────────────────

func TestMarcaDelete_ConflictGuard(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{
		"POST /api/v1": func(w http.ResponseWriter, r *http.Request) {
			oqlCount(w, 3) // 3 modelos reference this marca
		},
	})
	defer server.Close()

	h := newHandler(t, server)

	req := httptest.NewRequest("POST", "/brands/1/delete", nil)
	rec := httptest.NewRecorder()
	h.MarcaDelete(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict when marca is in use, got %d", rec.Code)
	}
}

// ── Additional: PropDefDelete conflict guard ──────────────────────────────────

func TestPropDefDelete_ConflictGuard(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{
		"POST /api/v1": func(w http.ResponseWriter, r *http.Request) {
			oqlCount(w, 1) // 1 association references this prop_def
		},
	})
	defer server.Close()

	h := newHandler(t, server)

	req := httptest.NewRequest("POST", "/properties/1/delete", nil)
	rec := httptest.NewRecorder()
	h.PropDefDelete(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict when prop definition is in use, got %d", rec.Code)
	}
}

func TestRegionEdit_ViewAndEditRoutes(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{
		"GET /api/v1/regions/": mockEntityGet(map[string]any{
			"id":          1.0,
			"description": "Montevideo",
		}),
	})
	defer server.Close()

	h := newHandler(t, server)

	// Direct view URL: /regions/1 (as linked from list table rows)
	req1 := httptest.NewRequest("GET", "/regions/1", nil)
	rec1 := httptest.NewRecorder()
	h.RegionEdit(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Errorf("expected 200 OK for /regions/1, got %d", rec1.Code)
	}
	if !strings.Contains(rec1.Body.String(), "Montevideo") {
		t.Errorf("expected body to contain 'Montevideo'")
	}

	// Edit URL: /regions/1/edit
	req2 := httptest.NewRequest("GET", "/regions/1/edit", nil)
	rec2 := httptest.NewRecorder()
	h.RegionEdit(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Errorf("expected 200 OK for /regions/1/edit, got %d", rec2.Code)
	}
	if !strings.Contains(rec2.Body.String(), "Montevideo") {
		t.Errorf("expected body to contain 'Montevideo'")
	}
}
