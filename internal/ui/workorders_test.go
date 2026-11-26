package ui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ha1tch/seam-ui/internal/auth"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

func backlogTestHandler(t *testing.T, routes map[string]http.HandlerFunc) *UIHandler {
	t.Helper()
	loadingSettings := true
	client := xolu.New("http://xolu.test", xolu.WithHTTPClient(&http.Client{
		Transport: templateTestTransport(func(r *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()
			if loadingSettings && r.URL.Path == "/api/v1/oql/query" {
				oqlEmpty(rec)
			} else if handler := routes[r.Method+" "+r.URL.Path]; handler != nil {
				handler(rec, r)
			} else {
				t.Errorf("unexpected Xolu request: %s %s", r.Method, r.URL.Path)
				rec.WriteHeader(http.StatusNotFound)
			}
			return rec.Result(), nil
		}),
	}))
	h := NewUIHandler(client, Config{AppName: "Test"}, nil, nil, nil, nil, nil, nil, nil)
	loadingSettings = false
	return h
}

func TestWorkOrderCreatePublishesBacklogOrder(t *testing.T) {
	var order map[string]any
	h := backlogTestHandler(t, map[string]http.HandlerFunc{
		"GET /api/v1/work_order_templates/7": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{
				"id": 7, "version": 1, "name": "Inspección",
				"workflow_states": []map[string]string{{"name": "Pendiente"}, {"name": "Lista"}},
			})
		},
		"POST /api/v1/work_orders": func(w http.ResponseWriter, r *http.Request) {
			json.NewDecoder(r.Body).Decode(&order)
			oqlCreated(w, 42)
		},
	})
	rec := postForm(t, h.WorkOrderCreate, "/workorders", url.Values{
		"title":            {"Inspección"},
		"template_id":      {"7"},
		"assignment_type":  {"board_posting"},
		"responsible_type": {"crew"},
		"assigned_to":      {"Debe quedar vacío"},
	})
	assertRedirectsTo(t, rec, "/workorders/42")
	if order["assignment_type"] != "board_posting" || order["responsible_type"] != "crew" ||
		order["responsible_id"] != float64(0) || order["assigned_to"] != "" || order["state"] != "Pendiente" {
		t.Fatalf("unexpected backlog order: %#v", order)
	}
}

func TestWorkOrdersBacklogListsAvailableOrders(t *testing.T) {
	h := backlogTestHandler(t, map[string]http.HandlerFunc{
		"POST /api/v1/oql/query": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if query := toString(body["query"]); !strings.Contains(query, "WHERE assignment_type = 'board_posting' AND responsible_id = 0") {
				t.Errorf("backlog query must filter available postings: %q", query)
			}
			json.NewEncoder(w).Encode(map[string]any{
				"status": "completed", "data": []map[string]any{{
					"id": 42, "title": "Inspección", "assignment_type": "board_posting", "responsible_type": "crew", "responsible_id": 0,
				}},
			})
		},
	})
	rec := httptest.NewRecorder()
	h.WorkOrdersBacklog(rec, httptest.NewRequest(http.MethodGet, "/workorders/backlog", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `href="/workorders/42"`) {
		t.Fatalf("available work order is missing from backlog: status=%d", rec.Code)
	}
}

func TestWorkOrderBacklogClaim(t *testing.T) {
	for _, tc := range []struct {
		name       string
		kind       string
		collection string
		selected   string
		conflict   bool
		wantStatus int
	}{
		{name: "user claims self", kind: "user", selected: "11", wantStatus: http.StatusSeeOther},
		{name: "user cannot claim for someone else", kind: "user", selected: "12", wantStatus: http.StatusForbidden},
		{name: "crew representative claims", kind: "crew", collection: "crews", selected: "21", wantStatus: http.StatusSeeOther},
		{name: "crew representative cannot claim for another crew", kind: "crew", collection: "crews", selected: "22", wantStatus: http.StatusForbidden},
		{name: "contractor representative claims", kind: "third_party", collection: "outsourced", selected: "21", wantStatus: http.StatusSeeOther},
		{name: "concurrent claim wins first", kind: "user", selected: "11", conflict: true, wantStatus: http.StatusConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var patch map[string]any
			h := backlogTestHandler(t, map[string]http.HandlerFunc{
				"GET /api/v1/work_orders/42": func(w http.ResponseWriter, r *http.Request) {
					json.NewEncoder(w).Encode(map[string]any{
						"id": 42, "_version": 3, "assignment_type": "board_posting",
						"responsible_type": tc.kind, "responsible_id": 0,
					})
				},
				"POST /api/v1/oql/query": func(w http.ResponseWriter, r *http.Request) {
					var body map[string]any
					json.NewDecoder(r.Body).Decode(&body)
					query := toString(body["query"])
					if tc.collection == "" || !strings.Contains(query, "FROM "+tc.collection+" WHERE responsible_user_id = 11") {
						t.Errorf("unexpected query: %q", query)
					}
					json.NewEncoder(w).Encode(map[string]any{
						"status": "completed", "data": []map[string]any{{"id": 21, "name": "Equipo"}},
					})
				},
				"PATCH /api/v1/work_orders/42": func(w http.ResponseWriter, r *http.Request) {
					json.NewDecoder(r.Body).Decode(&patch)
					if tc.conflict {
						w.WriteHeader(http.StatusConflict)
						w.Write([]byte(`{"error":{"code":"XOLU-ST005","message":"version conflict"}}`))
						return
					}
					w.Write([]byte(`{"id":42}`))
				},
			})
			form := url.Values{"responsible_id": {tc.selected}}
			req := httptest.NewRequest(http.MethodPost, "/workorders/42/claim", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.SetPathValue("id", "42")
			ctx := auth.ContextWithClaims(req.Context(), &auth.Claims{UserID: 11, Email: "user@example.test"})
			rec := httptest.NewRecorder()
			h.WorkOrderClaim(rec, req.WithContext(ctx))
			if rec.Code != tc.wantStatus {
				t.Fatalf("status=%d, want %d: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantStatus == http.StatusForbidden {
				if patch != nil {
					t.Fatalf("ineligible user updated the order: %v", patch)
				}
				return
			}
			if patch["_version"] != float64(3) || patch["claimed_by_user_id"] != float64(11) ||
				toString(patch["assigned_to"]) == "" ||
				(tc.kind == "user" && patch["responsible_id"] != float64(11)) ||
				(tc.kind != "user" && patch["responsible_id"] != float64(21)) {
				t.Fatalf("unexpected claim patch: %#v", patch)
			}
		})
	}
}

func TestOutsourcedLinkAccountForBacklog(t *testing.T) {
	var patch map[string]any
	h := backlogTestHandler(t, map[string]http.HandlerFunc{
		"GET /api/v1/users/11": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"id":11}`))
		},
		"PATCH /api/v1/outsourced/21": func(w http.ResponseWriter, r *http.Request) {
			json.NewDecoder(r.Body).Decode(&patch)
			w.Write([]byte(`{"id":21}`))
		},
	})
	form := url.Values{"responsible_user_id": {"11"}}
	req := httptest.NewRequest(http.MethodPost, "/outsourced/21/account", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()
	h.OutsourcedLinkAccount(rec, req)
	assertRedirectsTo(t, rec, "/outsourced/21")
	if patch["responsible_user_id"] != float64(11) {
		t.Fatalf("unexpected linked account: %#v", patch)
	}
}

// TestEnsureWorkOrderDefinition_CreatesWhenMissing confirms the bootstrap
// helper creates the definition on first use when ListMachineDefs comes
// back empty.
func TestEnsureWorkOrderDefinition_CreatesWhenMissing(t *testing.T) {
	created := false
	server := mockOLU(t, map[string]http.HandlerFunc{
		"GET /api/v2/fsm/def": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"definitions": []any{}})
		},
		"POST /api/v2/fsm/def": func(w http.ResponseWriter, r *http.Request) {
			created = true
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["name"] != workOrderDefName {
				t.Errorf("expected definition name %q, got %v", workOrderDefName, body["name"])
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"id": float64(1), "name": workOrderDefName})
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	id, err := h.ensureWorkOrderDefinition(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 1 {
		t.Errorf("expected definition ID 1, got %d", id)
	}
	if !created {
		t.Error("expected CreateMachineDef to be called when no definition exists yet")
	}
}

// TestEnsureWorkOrderDefinition_IdempotentWhenAlreadyExists confirms the
// bootstrap helper does NOT recreate the definition if one with the
// right name already exists -- FSM definitions are immutable, and a
// second, redundant create would be wasteful at best.
func TestEnsureWorkOrderDefinition_IdempotentWhenAlreadyExists(t *testing.T) {
	created := false
	server := mockOLU(t, map[string]http.HandlerFunc{
		"GET /api/v2/fsm/def": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"definitions": []any{
				map[string]any{"id": float64(7), "name": workOrderDefName, "created_at": "2026-08-01T00:00:00Z"},
			}})
		},
		"POST /api/v2/fsm/def": func(w http.ResponseWriter, r *http.Request) {
			created = true
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"id": float64(99), "name": workOrderDefName})
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	id, err := h.ensureWorkOrderDefinition(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 7 {
		t.Errorf("expected the EXISTING definition's ID (7), got %d", id)
	}
	if created {
		t.Error("CreateMachineDef should not have been called -- a matching definition already existed")
	}
}

// TestWorkOrderCreate_CreatesMachineThenEntityWithLinkedID confirms the
// create flow's real sequence: bootstrap the definition, create the
// FSM machine, then create the xolu entity with the machine's own ID
// and initial state embedded from the start (not a create-then-patch
// round trip).
func TestWorkOrderCreate_CreatesMachineThenEntityWithLinkedID(t *testing.T) {
	var createdEntity map[string]any
	server := mockOLU(t, map[string]http.HandlerFunc{
		"GET /api/v2/fsm/def": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"definitions": []any{
				map[string]any{"id": float64(1), "name": workOrderDefName, "created_at": "2026-08-01T00:00:00Z"},
			}})
		},
		"POST /api/v2/fsm/machine": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"id": float64(42), "definition": float64(1), "state": "draft", "vars": map[string]any{}})
		},
		"POST /api/v1/work_orders": func(w http.ResponseWriter, r *http.Request) {
			json.NewDecoder(r.Body).Decode(&createdEntity)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"id": float64(1)})
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.WorkOrderCreate, "/workorders", url.Values{
		"title":            {"Replace conveyor belt motor"},
		"description":      {"Bearing noise"},
		"assignment_type":  {"board_posting"},
		"responsible_type": {"user"},
	})

	assertRedirectsTo(t, rec, "/workorders/1")
	if createdEntity == nil {
		t.Fatal("xolu.Create for work_orders was never called")
	}
	if createdEntity["machine_id"] != 42.0 {
		t.Errorf("expected machine_id=42 embedded in the entity create, got %v", createdEntity["machine_id"])
	}
	if createdEntity["state"] != "draft" {
		t.Errorf("expected state=draft embedded in the entity create, got %v", createdEntity["state"])
	}
}

// TestWorkOrderComplete_GuardRejected_ReRendersWithErrorNot500 confirms
// a guard-rejected walk (e.g. an empty signature) is treated as a real,
// expected outcome -- a 200 re-render showing the error, not a raw 500.
func TestWorkOrderComplete_GuardRejected_ReRendersWithErrorNot500(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{
		"GET /api/v1/work_orders/1": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"id": float64(1), "title": "Test WO", "state": "in_progress", "machine_id": float64(42),
			})
		},
		"POST /api/v2/fsm/machine/42/walk": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{
				"code": "XOLU-FSM005", "message": "transition guard rejected: payload.technician_signature != ''", "status": float64(422),
			}})
		},
		"PATCH /api/v1/work_orders/1": func(w http.ResponseWriter, r *http.Request) {
			t.Error("a rejected transition must not update the work order")
			w.WriteHeader(http.StatusInternalServerError)
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	req := httptest.NewRequest("POST", "/workorders/1/complete", strings.NewReader(url.Values{
		"technician_signature": {""},
	}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()
	h.WorkOrderComplete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("guard-rejected complete: got status %d, want 200 (re-render with error)", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `role="alert"`) ||
		!strings.Contains(body, "field.technician_signature — core.errors.required") {
		t.Error("expected a visible signature error in the re-rendered page")
	}
}

// TestWorkOrderComplete_Success_PatchesStateAndSignature confirms a
// successful completion denormalizes both the new state and the
// signature onto the xolu entity, not just the state.
func TestWorkOrderComplete_Success_PatchesStateAndSignature(t *testing.T) {
	var patchedData map[string]any
	sigValue := "data:image/png;base64,iVBORw0KGgo="
	server := mockOLU(t, map[string]http.HandlerFunc{
		"GET /api/v1/work_orders/1": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"id": float64(1), "title": "Test WO", "state": "in_progress", "machine_id": float64(42),
			})
		},
		"POST /api/v2/fsm/machine/42/walk": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"previous": "in_progress", "current": "completed", "terminal": true, "history_id": float64(5)})
		},
		"PATCH /api/v1/work_orders/1": func(w http.ResponseWriter, r *http.Request) {
			json.NewDecoder(r.Body).Decode(&patchedData)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"id": float64(1)})
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	req := httptest.NewRequest("POST", "/workorders/1/complete", strings.NewReader(url.Values{
		"technician_signature": {sigValue},
	}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()
	h.WorkOrderComplete(rec, req)

	assertRedirectsTo(t, rec, "/workorders/1")
	if patchedData == nil {
		t.Fatal("xolu.Patch for work_orders/1 was never called")
	}
	if patchedData["state"] != "completed" {
		t.Errorf("expected state=completed in the patch, got %v", patchedData["state"])
	}
	if patchedData["technician_signature"] != sigValue {
		t.Errorf("expected the signature to be patched onto the entity, got %v", patchedData["technician_signature"])
	}
}
