package auth

import (
	"bytes"
	"context"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"
)

// mockAuditStore is a test implementation of AuditStore.
type mockAuditStore struct {
	events []*AuditEvent
}

func (m *mockAuditStore) SaveAuditEvent(ctx context.Context, event *AuditEvent) error {
	event.ID = int64(len(m.events) + 1)
	m.events = append(m.events, event)
	return nil
}

func (m *mockAuditStore) ListAuditEvents(ctx context.Context, filter AuditFilter) ([]*AuditEvent, error) {
	return m.events, nil
}

func TestAuditLogger_Log(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	store := &mockAuditStore{}
	auditLogger := NewAuditLogger(logger, store)

	event := &AuditEvent{
		EventType: AuditLoginSuccess,
		UserID:    123,
		Email:     "test@example.com",
		IPAddress: "192.168.1.1",
		Success:   true,
	}

	auditLogger.Log(context.Background(), event)

	// Check event was stored
	if len(store.events) != 1 {
		t.Errorf("Expected 1 event in store, got %d", len(store.events))
	}

	// Check event has ID
	if event.ID != 1 {
		t.Errorf("Event ID = %d, want 1", event.ID)
	}

	// Check timestamp was set
	if event.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}

	// Check log output contains event type
	logOutput := buf.String()
	if !bytes.Contains([]byte(logOutput), []byte("login_success")) {
		t.Errorf("Log should contain event type, got: %s", logOutput)
	}
}

func TestAuditLogger_LogLogin(t *testing.T) {
	store := &mockAuditStore{}
	auditLogger := NewAuditLogger(nil, store)

	req := httptest.NewRequest("POST", "/login", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	req.Header.Set("User-Agent", "TestBrowser/1.0")

	auditLogger.LogLogin(context.Background(), req, 123, "test@example.com", true, "Login successful")

	if len(store.events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(store.events))
	}

	event := store.events[0]
	if event.EventType != AuditLoginSuccess {
		t.Errorf("EventType = %s, want %s", event.EventType, AuditLoginSuccess)
	}
	if event.UserID != 123 {
		t.Errorf("UserID = %d, want 123", event.UserID)
	}
	if event.Email != "test@example.com" {
		t.Errorf("Email = %s, want test@example.com", event.Email)
	}
	if event.IPAddress != "10.0.0.1" {
		t.Errorf("IPAddress = %s, want 10.0.0.1", event.IPAddress)
	}
	if event.UserAgent != "TestBrowser/1.0" {
		t.Errorf("UserAgent = %s, want TestBrowser/1.0", event.UserAgent)
	}
}

func TestAuditLogger_LogLoginFailed(t *testing.T) {
	store := &mockAuditStore{}
	auditLogger := NewAuditLogger(nil, store)

	req := httptest.NewRequest("POST", "/login", nil)
	auditLogger.LogLogin(context.Background(), req, 0, "test@example.com", false, "Invalid password")

	if len(store.events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(store.events))
	}

	event := store.events[0]
	if event.EventType != AuditLoginFailed {
		t.Errorf("EventType = %s, want %s", event.EventType, AuditLoginFailed)
	}
	if event.Success {
		t.Error("Success should be false")
	}
}

func TestAuditLogger_LogLoginLocked(t *testing.T) {
	store := &mockAuditStore{}
	auditLogger := NewAuditLogger(nil, store)

	req := httptest.NewRequest("POST", "/login", nil)
	auditLogger.LogLoginLocked(context.Background(), req, "test@example.com", 5*time.Minute)

	if len(store.events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(store.events))
	}

	event := store.events[0]
	if event.EventType != AuditLoginLocked {
		t.Errorf("EventType = %s, want %s", event.EventType, AuditLoginLocked)
	}
	if event.Metadata == nil {
		t.Fatal("Metadata should not be nil")
	}
	if event.Metadata["remaining_lock_seconds"] != 300 {
		t.Errorf("remaining_lock_seconds = %v, want 300", event.Metadata["remaining_lock_seconds"])
	}
}

func TestAuditLogger_LogPasswordChanged(t *testing.T) {
	store := &mockAuditStore{}
	auditLogger := NewAuditLogger(nil, store)

	req := httptest.NewRequest("POST", "/profile/password", nil)

	// User changing own password
	auditLogger.LogPasswordChanged(context.Background(), req, 123, "test@example.com", false)

	event := store.events[0]
	if event.EventType != AuditPasswordChanged {
		t.Errorf("EventType = %s, want %s", event.EventType, AuditPasswordChanged)
	}
	if event.Metadata["by_admin"] != false {
		t.Error("by_admin should be false")
	}

	// Admin resetting password
	auditLogger.LogPasswordChanged(context.Background(), req, 123, "test@example.com", true)

	event = store.events[1]
	if event.Metadata["by_admin"] != true {
		t.Error("by_admin should be true")
	}
}

func TestAuditLogger_LogSettingsChanged(t *testing.T) {
	store := &mockAuditStore{}
	auditLogger := NewAuditLogger(nil, store)

	req := httptest.NewRequest("POST", "/settings/general", nil)
	auditLogger.LogSettingsChanged(context.Background(), req, 123, "admin@example.com", "general")

	event := store.events[0]
	if event.EventType != AuditSettingsChanged {
		t.Errorf("EventType = %s, want %s", event.EventType, AuditSettingsChanged)
	}
	if event.UserID != 123 {
		t.Errorf("UserID = %d, want 123", event.UserID)
	}
	// email param added 2026-08-03 (MOD-5 live verification): the
	// System dashboard's settings-activity widget needs the actor's
	// email (there's no separate "target user" for tenant-wide
	// settings) to render its primary_field.
	if event.Email != "admin@example.com" {
		t.Errorf("Email = %q, want %q", event.Email, "admin@example.com")
	}
	if event.Resource != "settings" {
		t.Errorf("Resource = %q, want %q", event.Resource, "settings")
	}
	if event.Metadata["section"] != "general" {
		t.Errorf("Metadata[section] = %v, want %q", event.Metadata["section"], "general")
	}
	if !event.Success {
		t.Error("Success should be true")
	}
}

func TestAuditLogger_LogPreferencesChanged(t *testing.T) {
	store := &mockAuditStore{}
	auditLogger := NewAuditLogger(nil, store)

	req := httptest.NewRequest("POST", "/profile/preferences", nil)
	auditLogger.LogPreferencesChanged(context.Background(), req, 456)

	event := store.events[0]
	if event.EventType != AuditPreferencesChanged {
		t.Errorf("EventType = %s, want %s", event.EventType, AuditPreferencesChanged)
	}
	if event.UserID != 456 {
		t.Errorf("UserID = %d, want 456", event.UserID)
	}
	if event.Resource != "profile" {
		t.Errorf("Resource = %q, want %q", event.Resource, "profile")
	}
	if !event.Success {
		t.Error("Success should be true")
	}
}

func TestAuditLogger_LogAccessDenied(t *testing.T) {
	store := &mockAuditStore{}
	auditLogger := NewAuditLogger(nil, store)

	req := httptest.NewRequest("GET", "/admin/users", nil)
	auditLogger.LogAccessDenied(context.Background(), req, 123, "users", "user:read")

	event := store.events[0]
	if event.EventType != AuditAccessDenied {
		t.Errorf("EventType = %s, want %s", event.EventType, AuditAccessDenied)
	}
	if event.Success {
		t.Error("Success should be false for access denied")
	}
	if event.Resource != "users" {
		t.Errorf("Resource = %s, want users", event.Resource)
	}
}

func TestAuditLogger_LogCSRFViolation(t *testing.T) {
	store := &mockAuditStore{}
	auditLogger := NewAuditLogger(nil, store)

	req := httptest.NewRequest("POST", "/profile", nil)
	auditLogger.LogCSRFViolation(context.Background(), req, 123)

	event := store.events[0]
	if event.EventType != AuditCSRFViolation {
		t.Errorf("EventType = %s, want %s", event.EventType, AuditCSRFViolation)
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		want       string
	}{
		{
			name:       "X-Forwarded-For single",
			headers:    map[string]string{"X-Forwarded-For": "10.0.0.1"},
			remoteAddr: "192.168.1.1:12345",
			want:       "10.0.0.1",
		},
		{
			name:       "X-Forwarded-For multiple",
			headers:    map[string]string{"X-Forwarded-For": "10.0.0.1, 10.0.0.2, 10.0.0.3"},
			remoteAddr: "192.168.1.1:12345",
			want:       "10.0.0.1",
		},
		{
			name:       "X-Real-IP",
			headers:    map[string]string{"X-Real-IP": "10.0.0.5"},
			remoteAddr: "192.168.1.1:12345",
			want:       "10.0.0.5",
		},
		{
			name:       "RemoteAddr with port",
			headers:    map[string]string{},
			remoteAddr: "192.168.1.1:12345",
			want:       "192.168.1.1",
		},
		{
			name:       "RemoteAddr without port",
			headers:    map[string]string{},
			remoteAddr: "192.168.1.1",
			want:       "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			got := getClientIP(req)
			if got != tt.want {
				t.Errorf("getClientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetClientIP_NilRequest(t *testing.T) {
	got := getClientIP(nil)
	if got != "" {
		t.Errorf("getClientIP(nil) = %q, want empty string", got)
	}
}

func TestAuditLogger_LogDeveloperChange(t *testing.T) {
	store := &mockAuditStore{}
	auditLogger := NewAuditLogger(nil, store)
	req := httptest.NewRequest("POST", "/developer/flags", nil)

	auditLogger.LogDeveloperChange(context.Background(), req, 7, 3, "feature_flag", "hotload_i18n",
		map[string]any{"value": true})

	if len(store.events) != 1 {
		t.Fatalf("got %d events", len(store.events))
	}
	e := store.events[0]
	if e.EventType != AuditDeveloperChanged || e.UserID != 7 || e.TenantID != 3 ||
		e.Resource != "developer" || e.Action != "feature_flag" || e.Details != "hotload_i18n" || !e.Success {
		t.Errorf("event = %+v", e)
	}
	if e.Metadata["value"] != true {
		t.Errorf("metadata = %v", e.Metadata)
	}
}

func TestAuditLogger_LogDeveloperChange_NilLoggerIsSafe(t *testing.T) {
	var a *AuditLogger
	a.LogDeveloperChange(context.Background(), httptest.NewRequest("POST", "/x", nil), 1, 1, "feature_flag", "k", nil)
}
