package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	xolu "github.com/ha1tch/xolu/pkg/client"
)

// This file exists because XOLUAuditStore (the real, xolu-backed
// AuditStore implementation) had zero test coverage before 2026-08-03 --
// only the in-memory mockAuditStore (audit_test.go) was ever exercised.
// That's exactly why ListAuditEvents' broken sort (client.List()'s Sort
// param never reaching the server at all -- see oql-orderby-diagnostic-
// report.md) went unnoticed: nothing ever sent a real request and
// checked what came back. These tests build a minimal mock xolu server
// (POST /oql/query) and check both the query XOLUAuditStore actually
// sends and what it does with a realistic response.

// mockXOLUForAudit returns an httptest.Server implementing just enough
// of xolu's OQL endpoint for ListAuditEvents: it captures the received
// query string and returns rows (already in the order a real, correctly
// functioning "ORDER BY id DESC" would return them -- this file tests
// XOLUAuditStore's own query-construction and response-handling logic,
// not xolu's sort correctness, which was verified separately and
// directly against the real server).
func mockXOLUForAudit(t *testing.T, rows []map[string]any) (*httptest.Server, *string) {
	t.Helper()
	var capturedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/oql/query" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var body struct {
			Query string `json:"query"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		capturedQuery = body.Query
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": "completed",
			"data":   rows,
			"stats":  map[string]any{},
		})
	}))
	return server, &capturedQuery
}

func auditRow(id int, eventType, resource, email, timestamp string) map[string]any {
	return map[string]any{
		"id":         id,
		"event_type": eventType,
		"resource":   resource,
		"email":      email,
		"timestamp":  timestamp,
		"user_id":    float64(1),
		"tenant_id":  float64(0),
		"success":    true,
	}
}

func TestListAuditEvents_QueryShape(t *testing.T) {
	// The specific, previously-regressed thing: the query sent to xolu
	// must ask for ORDER BY id DESC (proven correct, see the diagnostic
	// report), not ORDER BY timestamp DESC (proven broken) or rely on
	// client.List()'s Sort param (proven never transmitted at all).
	server, query := mockXOLUForAudit(t, []map[string]any{
		auditRow(3, "login_success", "", "a@example.com", "2026-08-03T03:00:00Z"),
	})
	defer server.Close()

	store := NewXOLUAuditStore(xolu.New(server.URL))
	_, err := store.ListAuditEvents(t.Context(), AuditFilter{Limit: 5})
	if err != nil {
		t.Fatalf("ListAuditEvents: %v", err)
	}

	if !strings.Contains(*query, "ORDER BY id DESC") {
		t.Errorf("query = %q, want it to contain %q", *query, "ORDER BY id DESC")
	}
	if strings.Contains(*query, "created_at") || strings.Contains(*query, "timestamp DESC") {
		t.Errorf("query = %q, should not sort by created_at or timestamp DESC (both proven broken)", *query)
	}
	if !strings.Contains(*query, "TOP 5") {
		t.Errorf("query = %q, want it to contain %q (Limit=5, Offset=0)", *query, "TOP 5")
	}
}

func TestListAuditEvents_PreservesServerOrder(t *testing.T) {
	// ListAuditEvents must not re-sort or reverse what xolu returns --
	// it's already newest-first by construction of the query.
	server, _ := mockXOLUForAudit(t, []map[string]any{
		auditRow(10, "role_changed", "user", "c@example.com", "2026-08-03T03:00:00Z"),
		auditRow(9, "user_updated", "user", "c@example.com", "2026-08-03T02:59:00Z"),
		auditRow(6, "user_created", "user", "c@example.com", "2026-08-03T02:57:00Z"),
	})
	defer server.Close()

	store := NewXOLUAuditStore(xolu.New(server.URL))
	events, err := store.ListAuditEvents(t.Context(), AuditFilter{Limit: 5})
	if err != nil {
		t.Fatalf("ListAuditEvents: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}
	gotIDs := []int64{events[0].ID, events[1].ID, events[2].ID}
	wantIDs := []int64{10, 9, 6}
	for i := range wantIDs {
		if gotIDs[i] != wantIDs[i] {
			t.Errorf("events[%d].ID = %d, want %d (order = %v, want %v)", i, gotIDs[i], wantIDs[i], gotIDs, wantIDs)
		}
	}
}

func TestListAuditEvents_FilterByResource(t *testing.T) {
	server, _ := mockXOLUForAudit(t, []map[string]any{
		auditRow(5, "login_success", "", "u@example.com", "2026-08-03T03:00:00Z"),
		auditRow(4, "role_changed", "user", "u@example.com", "2026-08-03T02:59:00Z"),
		auditRow(3, "settings_changed", "settings", "u@example.com", "2026-08-03T02:58:00Z"),
		auditRow(2, "user_created", "user", "u@example.com", "2026-08-03T02:57:00Z"),
	})
	defer server.Close()

	store := NewXOLUAuditStore(xolu.New(server.URL))
	events, err := store.ListAuditEvents(t.Context(), AuditFilter{Limit: 10, Resource: "user"})
	if err != nil {
		t.Fatalf("ListAuditEvents: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2 (resource=user only)", len(events))
	}
	for _, ev := range events {
		if ev.Resource != "user" {
			t.Errorf("event %d has Resource = %q, want %q", ev.ID, ev.Resource, "user")
		}
	}
}

func TestListAuditEvents_RespectsOffset(t *testing.T) {
	server, query := mockXOLUForAudit(t, []map[string]any{
		auditRow(5, "e", "", "u@example.com", "2026-08-03T03:00:00Z"),
		auditRow(4, "e", "", "u@example.com", "2026-08-03T02:59:00Z"),
		auditRow(3, "e", "", "u@example.com", "2026-08-03T02:58:00Z"),
		auditRow(2, "e", "", "u@example.com", "2026-08-03T02:57:00Z"),
		auditRow(1, "e", "", "u@example.com", "2026-08-03T02:56:00Z"),
	})
	defer server.Close()

	store := NewXOLUAuditStore(xolu.New(server.URL))
	events, err := store.ListAuditEvents(t.Context(), AuditFilter{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("ListAuditEvents: %v", err)
	}
	// TOP (limit+offset) = TOP 4, so the mock only needs rows id 5..2 to
	// answer it, but returning 5 exercises that ListAuditEvents doesn't
	// itself assume the server always returns exactly fetchN rows.
	if !strings.Contains(*query, "TOP 4") {
		t.Errorf("query = %q, want it to contain %q (Limit=2, Offset=2 -> fetchN=4)", *query, "TOP 4")
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	// offset=2 skips id 5, 4 (the two newest); next two are id 3, 2.
	if events[0].ID != 3 || events[1].ID != 2 {
		t.Errorf("events = [%d, %d], want [3, 2] (offset=2 should skip the 2 newest)", events[0].ID, events[1].ID)
	}
}

func TestListAuditEvents_EmailFieldRoundTrips(t *testing.T) {
	// Guards the actual bug this session was asked to fix (LogUserCreated
	// etc. not setting AuditEvent.Email) at the read path too: if the
	// stored row has a top-level "email" key, ListAuditEvents must
	// surface it on the returned AuditEvent.
	server, _ := mockXOLUForAudit(t, []map[string]any{
		auditRow(1, "user_created", "user", "postfix@example.com", "2026-08-03T03:00:00Z"),
	})
	defer server.Close()

	store := NewXOLUAuditStore(xolu.New(server.URL))
	events, err := store.ListAuditEvents(t.Context(), AuditFilter{Limit: 5})
	if err != nil {
		t.Fatalf("ListAuditEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if events[0].Email != "postfix@example.com" {
		t.Errorf("Email = %q, want %q", events[0].Email, "postfix@example.com")
	}
}
