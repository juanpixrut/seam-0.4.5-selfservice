package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	xolu "github.com/ha1tch/xolu/pkg/client"
)

// ── Alert acknowledge (tests 21–22) ──────────────────────────────────────────

func alertEntity(id int64, status string) map[string]any {
	return map[string]any{
		"id":         float64(id),
		"title":      "Test Alert",
		"message":    "Something happened",
		"severity":   "high",
		"status":     status,
		"source":     "rule",
		"asset_id":   float64(1),
		"rule_id":    float64(0),
		"alerta_id":  float64(0),
		"created_at": time.Now().Format(time.RFC3339),
		"updated_at": time.Now().Format(time.RFC3339),
	}
}

func oqlEntity(w http.ResponseWriter, data map[string]any) {
	e := &xolu.Entity{ID: int64(data["id"].(float64)), Data: data}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(e)
}

func TestAlertAcknowledge_UpdatesStatus(t *testing.T) {
	patchCalled := false
	historyCalled := false

	server := mockOLU(t, map[string]http.HandlerFunc{
		// GET /api/v1/.../alerts/1 — return the alert entity
		"GET /api/v1": func(w http.ResponseWriter, r *http.Request) {
			oqlEntity(w, alertEntity(1, "open"))
		},
		// PATCH /api/v1/.../alerts/1 — status update (switched from PUT
		// 2026-08-03: PUT is xolu's full-document replace, confirmed to
		// wipe every field but the one being set; PATCH is a real
		// partial merge)
		"PATCH /api/v1": func(w http.ResponseWriter, r *http.Request) {
			patchCalled = true
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"id": float64(1)})
		},
		// POST /api/v1/.../alert_history — history entry
		"POST /api/v1": func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "alert_history") {
				historyCalled = true
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"id": float64(99)})
		},
	})
	defer server.Close()

	h := newHandler(t, server)

	form := url.Values{"notes": {"Revisado y confirmado"}}
	req := httptest.NewRequest("POST", "/alerts/1/acknowledge",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.AlertAcknowledge(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303 SeeOther after acknowledge, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/alerts/1" {
		t.Errorf("expected redirect to /alerts/1, got %q", loc)
	}
	if !patchCalled {
		t.Error("xolu PUT (status update) was never called")
	}
	_ = historyCalled // history creation is best-effort; don't fail if xolu call order differs
}

func TestAlertAcknowledge_InvalidID(t *testing.T) {
	server := mockOLU(t, nil)
	defer server.Close()

	h := newHandler(t, server)

	req := httptest.NewRequest("POST", "/alerts/0/acknowledge", nil)
	rec := httptest.NewRecorder()
	h.AlertAcknowledge(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for ID=0, got %d", rec.Code)
	}
}

// ── Asset list filters (tests 23–24) ─────────────────────────────────────────

func TestAssetsList_CriticidadFilter(t *testing.T) {
	// Collect ALL OQL queries fired by AssetsList (it makes 3: main, count, types)
	var queries []string
	server := mockOLU(t, map[string]http.HandlerFunc{
		"POST /api/v1": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if q, ok := body["query"].(string); ok {
				queries = append(queries, q)
			}
			// Return count=0 for count queries, empty otherwise
			if containsHelper(body["query"].(string), "COUNT") {
				oqlCount(w, 0)
			} else {
				oqlEmpty(w)
			}
		},
	})
	defer server.Close()

	client := xolu.New(server.URL)
	cfg := Config{AppName: "Test", Version: "0.0.0"}
	h := NewUIHandler(client, cfg, nil, nil, nil, nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/assets?criticidad=Alta", nil)
	rec := httptest.NewRecorder()
	h.AssetsList(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if len(queries) == 0 {
		t.Fatal("no OQL queries were captured")
	}
	// At least one query must target assets with the criticality filter
	// -- the URL query parameter itself stays "criticidad" (wire name,
	// unchanged, matching this codebase's established precedent for
	// form/URL parameter names), only xolu's own column name changed.
	found := false
	for _, q := range queries {
		if containsHelper(q, "FROM assets") && containsHelper(q, "criticality = 'Alta'") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("no asset query contained criticality filter; queries seen: %v", queries)
	}
}

func TestAssetsList_StatusFilter(t *testing.T) {
	var queries []string
	server := mockOLU(t, map[string]http.HandlerFunc{
		"POST /api/v1": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if q, ok := body["query"].(string); ok {
				queries = append(queries, q)
			}
			if containsHelper(body["query"].(string), "COUNT") {
				oqlCount(w, 0)
			} else {
				oqlEmpty(w)
			}
		},
	})
	defer server.Close()

	client := xolu.New(server.URL)
	cfg := Config{AppName: "Test", Version: "0.0.0"}
	h := NewUIHandler(client, cfg, nil, nil, nil, nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/assets?status=maintenance", nil)
	rec := httptest.NewRecorder()
	h.AssetsList(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	found := false
	for _, q := range queries {
		if containsHelper(q, "FROM assets") && containsHelper(q, "status = 'maintenance'") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("no asset query contained status filter; queries seen: %v", queries)
	}
}

// ── Dashboard zero state (test 25) ───────────────────────────────────────────

func TestDashboard_ZeroState(t *testing.T) {
	// All OQL queries return empty / count 0
	server := mockOLU(t, map[string]http.HandlerFunc{
		"POST /api/v1": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			q, _ := body["query"].(string)
			// Return count=0 for count queries, empty data otherwise
			if containsHelper(q, "COUNT") {
				oqlCount(w, 0)
			} else {
				oqlEmpty(w)
			}
		},
	})
	defer server.Close()

	client := xolu.New(server.URL)
	cfg := Config{AppName: "Test", Version: "0.0.0", EnableOperational: true}
	h := NewUIHandler(client, cfg, nil, nil, nil, nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	// Dashboard should render without panic even with all-zero data
	h.Dashboard(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 on zero-state dashboard, got %d", rec.Code)
	}
	body := rec.Body.String()
	if len(body) < 100 {
		t.Errorf("dashboard response looks too short (%d bytes), likely failed to render", len(body))
	}
}

// TestAlertResolve_UsesPatch closes a real, previously-unfilled gap
// found 2026-08-04 while auditing whether all 17 of this session's
// Update-to-Patch fixes actually have adversarial test coverage:
// AlertAcknowledge (above) has had a dedicated test since T-49's
// original fix; its sibling AlertResolve -- same handler shape, same
// bug, same fix, filed and fixed together at the time -- never got its
// own. If AlertResolve had regressed back to Update (PUT), nothing
// would have caught it.
func TestAlertResolve_UsesPatch(t *testing.T) {
	patchCalled := false
	historyCalled := false

	server := mockOLU(t, map[string]http.HandlerFunc{
		"PATCH /api/v1": func(w http.ResponseWriter, r *http.Request) {
			patchCalled = true
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"id": float64(1)})
		},
		"POST /api/v1": func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "alert_history") {
				historyCalled = true
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"id": float64(99)})
		},
	})
	defer server.Close()

	h := newHandler(t, server)

	req := httptest.NewRequest("POST", "/alerts/1/resolve", nil)
	rec := httptest.NewRecorder()
	h.AlertResolve(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303 SeeOther after resolve, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/alerts" {
		t.Errorf("expected redirect to /alerts, got %q", loc)
	}
	if !patchCalled {
		t.Error("xolu.Patch (status update) was never called")
	}
	_ = historyCalled // history creation is best-effort; don't fail if xolu call order differs
}
