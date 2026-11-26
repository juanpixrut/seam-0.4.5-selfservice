package ui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/dashboardengine"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

// This file proves MOD-3's audit wiring actually fires -- not just that
// the handlers it touched still redirect correctly (validation_handler_
// test.go's own coverage, unchanged by this file). A first pass of
// changes here passed the existing test suite without any of these
// audit calls ever executing -- newHandler's auditLogger is nil and none
// of those tests inject auth.Claims into the request context, so every
// `if claims, ok := auth.ClaimsFromContext(ctx); ok { ... }` guard this
// pass added was silently skipped. That's not evidence the wiring works;
// these tests are.

// captureStore implements auth.AuditStore by recording every event.
type captureStore struct {
	events []*auth.AuditEvent
}

func (s *captureStore) SaveAuditEvent(ctx context.Context, event *auth.AuditEvent) error {
	s.events = append(s.events, event)
	return nil
}

func (s *captureStore) ListAuditEvents(ctx context.Context, filter auth.AuditFilter) ([]*auth.AuditEvent, error) {
	return s.events, nil
}

// newHandlerWithAudit builds a UIHandler like newHandler, but with a real
// *auth.AuditLogger backed by store, instead of the nil auditLogger
// newHandler's own tests correctly don't need.
func newHandlerWithAudit(t *testing.T, server *httptest.Server, store *captureStore) *UIHandler {
	t.Helper()
	client := xolu.New(server.URL)
	cfg := Config{AppName: "Test", Version: "0.0.0"}
	auditLogger := auth.NewAuditLogger(nil, store)
	return NewUIHandler(client, cfg, nil, nil, nil, nil, nil, auditLogger, nil)
}

func TestAdminUserCreate_LogsUserCreated(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{
		"POST /api/v1": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"id": float64(42), "message": "created"})
		},
	})
	defer server.Close()

	store := &captureStore{}
	h := newHandlerWithAudit(t, server, store)

	req := httptest.NewRequest("POST", "/admin/users", strings.NewReader(url.Values{
		"email": {"new@example.com"}, "name": {"New User"}, "role": {"operator"},
		"password": {"longenough1"}, "password_confirm": {"longenough1"},
	}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{UserID: 1, TenantID: 1, Role: "admin"}))
	rec := httptest.NewRecorder()
	h.AdminUserCreate(rec, req)

	assertRedirectsTo(t, rec, "/admin/users")
	if len(store.events) != 1 {
		t.Fatalf("got %d audit events, want 1", len(store.events))
	}
	ev := store.events[0]
	if ev.EventType != auth.AuditUserCreated {
		t.Errorf("EventType = %s, want %s", ev.EventType, auth.AuditUserCreated)
	}
	if ev.UserID != 1 {
		t.Errorf("actor UserID = %d, want 1", ev.UserID)
	}
	// LogUserCreated puts the new user's email in both Metadata (kept for
	// backward compat with anything reading Metadata directly) and the
	// top-level Email field -- fixed 2026-08-03 during MOD-5's live
	// verification of the System dashboard's admin-activity widget
	// (primary_field: "email"), which found the top-level field was
	// never set despite the parameter existing. This assertion used to
	// document the bug as expected behaviour; it now asserts the fix.
	if ev.Email != "new@example.com" {
		t.Errorf("Email = %q, want %q", ev.Email, "new@example.com")
	}
	if ev.Metadata["email"] != "new@example.com" {
		t.Errorf("Metadata[email] = %v, want %q", ev.Metadata["email"], "new@example.com")
	}
	if ev.Metadata["new_user_id"] != int64(42) {
		t.Errorf("Metadata[new_user_id] = %v, want 42", ev.Metadata["new_user_id"])
	}
	if ev.Metadata["role"] != "operator" {
		t.Errorf("Metadata[role] = %v, want %q", ev.Metadata["role"], "operator")
	}
}

func TestAdminUserUpdate_LogsRoleChangeWhenRoleDiffers(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{
		"GET /api/v1": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"id": 1, "name": "Ana", "email": "ana@example.com", "role": "operator"})
		},
		"PATCH /api/v1": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"message": "updated"})
		},
	})
	defer server.Close()

	store := &captureStore{}
	h := newHandlerWithAudit(t, server, store)

	req := httptest.NewRequest("POST", "/admin/users/1",
		strings.NewReader(url.Values{"name": {"Ana"}, "role": {"manager"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "1")
	req = req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{UserID: 9, TenantID: 1, Role: "admin"}))
	rec := httptest.NewRecorder()
	h.AdminUserUpdate(rec, req)

	assertRedirectsTo(t, rec, "/admin/users")
	if len(store.events) != 2 {
		t.Fatalf("got %d audit events, want 2 (user_updated + role_changed)", len(store.events))
	}
	if store.events[0].EventType != auth.AuditUserUpdated {
		t.Errorf("events[0].EventType = %s, want %s", store.events[0].EventType, auth.AuditUserUpdated)
	}
	// Email is the TARGET user's (fetched via existing.Data["email"]),
	// not the actor's (UserID=9) -- fixed 2026-08-03 alongside
	// LogUserCreated's identical gap, same root cause: an email
	// parameter that existed on the method but was never assigned to
	// AuditEvent.Email.
	if store.events[0].Email != "ana@example.com" {
		t.Errorf("events[0].Email = %q, want %q", store.events[0].Email, "ana@example.com")
	}
	roleEv := store.events[1]
	if roleEv.EventType != auth.AuditRoleChanged {
		t.Errorf("events[1].EventType = %s, want %s", roleEv.EventType, auth.AuditRoleChanged)
	}
	if roleEv.Email != "ana@example.com" {
		t.Errorf("events[1].Email = %q, want %q", roleEv.Email, "ana@example.com")
	}
	if roleEv.Metadata["old_role"] != "operator" || roleEv.Metadata["new_role"] != "manager" {
		t.Errorf("role change metadata = %#v, want old_role=operator new_role=manager", roleEv.Metadata)
	}
}

func TestAdminUserUpdate_NoRoleChangeEvent_WhenRoleUnchanged(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{
		"GET /api/v1": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"id": 1, "name": "Ana", "role": "manager"})
		},
		"PATCH /api/v1": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"message": "updated"})
		},
	})
	defer server.Close()

	store := &captureStore{}
	h := newHandlerWithAudit(t, server, store)

	req := httptest.NewRequest("POST", "/admin/users/1",
		strings.NewReader(url.Values{"name": {"Ana"}, "role": {"manager"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "1")
	req = req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{UserID: 9, TenantID: 1, Role: "admin"}))
	rec := httptest.NewRecorder()
	h.AdminUserUpdate(rec, req)

	assertRedirectsTo(t, rec, "/admin/users")
	if len(store.events) != 1 {
		t.Fatalf("got %d audit events, want exactly 1 (user_updated only, no role_changed since role didn't change)", len(store.events))
	}
	if store.events[0].EventType != auth.AuditUserUpdated {
		t.Errorf("EventType = %s, want %s", store.events[0].EventType, auth.AuditUserUpdated)
	}
}

func TestSettingsSaveGeneral_LogsSettingsChanged(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{
		"POST /api/v1": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"message": "created"})
		},
	})
	defer server.Close()

	store := &captureStore{}
	h := newHandlerWithAudit(t, server, store)

	req := httptest.NewRequest("POST", "/settings/general",
		strings.NewReader(url.Values{"tenant_name": {"Acme"}, "locale": {"en"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{UserID: 5, TenantID: 1, Role: "admin"}))
	rec := httptest.NewRecorder()
	h.SettingsSaveGeneral(rec, req)

	assertRedirectsTo(t, rec, "/settings?saved=general")
	if len(store.events) != 1 {
		t.Fatalf("got %d audit events, want 1", len(store.events))
	}
	ev := store.events[0]
	if ev.EventType != auth.AuditSettingsChanged {
		t.Errorf("EventType = %s, want %s", ev.EventType, auth.AuditSettingsChanged)
	}
	if ev.Metadata["section"] != "general" {
		t.Errorf("Metadata[section] = %v, want %q", ev.Metadata["section"], "general")
	}
}

func TestProfilePreferencesSave_LogsPreferencesChanged(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{
		"PATCH /api/v1": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"message": "updated"})
		},
	})
	defer server.Close()

	store := &captureStore{}
	h := newHandlerWithAudit(t, server, store)

	req := httptest.NewRequest("POST", "/profile/preferences",
		strings.NewReader(url.Values{"locale": {"en"}, "dark_mode": {"system"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{UserID: 7, TenantID: 1, Role: "viewer"}))
	rec := httptest.NewRecorder()
	h.ProfilePreferencesSave(rec, req)

	assertRedirectsTo(t, rec, "/profile?saved=preferences")
	if len(store.events) != 1 {
		t.Fatalf("got %d audit events, want 1", len(store.events))
	}
	if store.events[0].EventType != auth.AuditPreferencesChanged {
		t.Errorf("EventType = %s, want %s", store.events[0].EventType, auth.AuditPreferencesChanged)
	}
}

// -- audit_log dashboard source (MOD-3 part 2) --

// filteringStore is unlike captureStore (which returns everything
// unfiltered) -- this one applies UserID/Resource/EventTypes the same
// way auth.XOLUAuditStore's real filter loop does, so these tests
// actually exercise whether auditLogRows built the right auth.AuditFilter,
// not just that events pass through untouched.
type filteringStore struct {
	events []*auth.AuditEvent
}

func (s *filteringStore) SaveAuditEvent(ctx context.Context, event *auth.AuditEvent) error {
	s.events = append(s.events, event)
	return nil
}

func (s *filteringStore) ListAuditEvents(ctx context.Context, filter auth.AuditFilter) ([]*auth.AuditEvent, error) {
	var out []*auth.AuditEvent
	for _, ev := range s.events {
		if filter.UserID > 0 && ev.UserID != filter.UserID {
			continue
		}
		if filter.Resource != "" && ev.Resource != filter.Resource {
			continue
		}
		if len(filter.EventTypes) > 0 {
			matched := false
			for _, et := range filter.EventTypes {
				if ev.EventType == et {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		out = append(out, ev)
	}
	if filter.Limit > 0 && len(out) > filter.Limit {
		out = out[:filter.Limit]
	}
	return out, nil
}

func TestAuditLogRows_ScopeSelf_FiltersToCurrentUser(t *testing.T) {
	store := &filteringStore{events: []*auth.AuditEvent{
		{ID: 1, EventType: auth.AuditPreferencesChanged, UserID: 7, Resource: "profile", Timestamp: time.Now()},
		{ID: 2, EventType: auth.AuditPreferencesChanged, UserID: 9, Resource: "profile", Timestamp: time.Now()},
	}}
	h := &UIHandler{auditLogger: auth.NewAuditLogger(nil, store)}
	r := &xoluDashboardResolver{h: h}

	rows, err := r.Rows(context.Background(), dashboardengine.EntitySource{
		Type: "audit_log", Resource: "profile", Scope: "self", CurrentUserID: 7,
	})
	if err != nil {
		t.Fatalf("Rows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1 (only user 7's own event)", len(rows))
	}
	if rows[0]["id"] != int64(1) {
		t.Errorf("expected event id 1, got %#v", rows[0])
	}
}

func TestAuditLogRows_ScopeTenant_ReturnsEveryUsersEvents(t *testing.T) {
	store := &filteringStore{events: []*auth.AuditEvent{
		{ID: 1, EventType: auth.AuditUserCreated, UserID: 7, Resource: "user", Timestamp: time.Now()},
		{ID: 2, EventType: auth.AuditUserUpdated, UserID: 9, Resource: "user", Timestamp: time.Now()},
	}}
	h := &UIHandler{auditLogger: auth.NewAuditLogger(nil, store)}
	r := &xoluDashboardResolver{h: h}

	rows, err := r.Rows(context.Background(), dashboardengine.EntitySource{
		Type: "audit_log", Resource: "user", Scope: "tenant", CurrentUserID: 7,
	})
	if err != nil {
		t.Fatalf("Rows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (scope:tenant must not filter by CurrentUserID)", len(rows))
	}
}

func TestAuditLogRows_FieldMapping(t *testing.T) {
	ts := time.Date(2026, 8, 2, 14, 30, 0, 0, time.UTC)
	store := &filteringStore{events: []*auth.AuditEvent{
		{ID: 5, EventType: auth.AuditRoleChanged, Email: "a@example.com", Resource: "user", Action: "role_change", Details: "changed", Timestamp: ts},
	}}
	h := &UIHandler{auditLogger: auth.NewAuditLogger(nil, store)}
	r := &xoluDashboardResolver{h: h}

	rows, err := r.Rows(context.Background(), dashboardengine.EntitySource{Type: "audit_log", Scope: "tenant"})
	if err != nil {
		t.Fatalf("Rows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	row := rows[0]
	checks := map[string]any{
		"id": int64(5), "event_type": "role_changed", "email": "a@example.com",
		"resource": "user", "action": "role_change", "details": "changed",
		"timestamp": "02/08/2026 14:30",
	}
	for field, want := range checks {
		if row[field] != want {
			t.Errorf("row[%q] = %#v, want %#v", field, row[field], want)
		}
	}
}

func TestAuditLogRows_EventTypesFilter(t *testing.T) {
	store := &filteringStore{events: []*auth.AuditEvent{
		{ID: 1, EventType: auth.AuditUserCreated, Resource: "user", Timestamp: time.Now()},
		{ID: 2, EventType: auth.AuditRoleChanged, Resource: "user", Timestamp: time.Now()},
	}}
	h := &UIHandler{auditLogger: auth.NewAuditLogger(nil, store)}
	r := &xoluDashboardResolver{h: h}

	rows, err := r.Rows(context.Background(), dashboardengine.EntitySource{
		Type: "audit_log", Scope: "tenant", EventTypes: []string{"role_changed"},
	})
	if err != nil {
		t.Fatalf("Rows: %v", err)
	}
	if len(rows) != 1 || rows[0]["id"] != int64(2) {
		t.Errorf("EventTypes filter didn't narrow correctly: %#v", rows)
	}
}

// TestProfileSave_UsesPatch closes a real, previously-unfilled gap
// found 2026-08-04 while auditing whether all 17 of this session's
// Update-to-Patch fixes actually have adversarial test coverage:
// ProfileSave was T-49's original trigger (the tech team's own report) and
// was live-verified extensively at the time (a real name change,
// confirmed persisted with every other field surviving), but never
// got a dedicated unit test of its own -- unlike AdminUserUpdate and
// ProfilePreferencesSave, its siblings in the same fix, which did.
func TestProfileSave_UsesPatch(t *testing.T) {
	patched := false
	server := mockOLU(t, map[string]http.HandlerFunc{
		"PATCH /api/v1/users/": func(w http.ResponseWriter, r *http.Request) {
			patched = true
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"id": float64(1)})
		},
	})
	defer server.Close()
	h := newHandlerWithAudit(t, server, &captureStore{})

	req := httptest.NewRequest("POST", "/profile", strings.NewReader(url.Values{
		"name": {"Updated Name"},
	}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{UserID: 1, TenantID: 1, Role: "admin"}))
	rec := httptest.NewRecorder()
	h.ProfileSave(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ProfileSave: got status %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/profile?saved=profile" {
		t.Errorf("expected redirect to /profile?saved=profile, got %q", loc)
	}
	if !patched {
		t.Error("xolu.Patch was not called")
	}
}
