package auth

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// AuditEventType represents the type of audit event.
type AuditEventType string

const (
	// Authentication events
	AuditLoginSuccess   AuditEventType = "login_success"
	AuditLoginFailed    AuditEventType = "login_failed"
	AuditLoginLocked    AuditEventType = "login_locked"
	AuditLogout         AuditEventType = "logout"
	AuditSessionExpired AuditEventType = "session_expired"

	// Password events
	AuditPasswordChanged  AuditEventType = "password_changed"
	AuditPasswordReset    AuditEventType = "password_reset"
	AuditPasswordResetReq AuditEventType = "password_reset_requested"

	// User management events
	AuditUserCreated  AuditEventType = "user_created"
	AuditUserUpdated  AuditEventType = "user_updated"
	AuditUserDeleted  AuditEventType = "user_deleted"
	AuditUserEnabled  AuditEventType = "user_enabled"
	AuditUserDisabled AuditEventType = "user_disabled"
	AuditRoleChanged  AuditEventType = "role_changed"

	// Settings/preferences events (MOD-3, DASHBOARD_SPEC.md §9.3 -- added
	// alongside wiring these handlers to actually call the logger; the
	// event types existed nowhere to call before this)
	AuditSettingsChanged    AuditEventType = "settings_changed"
	AuditPreferencesChanged AuditEventType = "preferences_changed"

	// Developer module events (campaign DEV-8): runtime flag changes and,
	// later, override saves. Changing runtime behaviour is always audited.
	AuditDeveloperChanged AuditEventType = "developer_changed"

	// Access events
	AuditAccessDenied  AuditEventType = "access_denied"
	AuditCSRFViolation AuditEventType = "csrf_violation"

	// API events
	AuditAPIKeyUsed    AuditEventType = "api_key_used"
	AuditAPIKeyCreated AuditEventType = "api_key_created"
	AuditAPIKeyRevoked AuditEventType = "api_key_revoked"
)

// AuditEvent represents a security audit event.
type AuditEvent struct {
	ID        int64          `json:"id,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
	EventType AuditEventType `json:"event_type"`
	UserID    int64          `json:"user_id,omitempty"`
	Email     string         `json:"email,omitempty"`
	TenantID  int64          `json:"tenant_id,omitempty"`
	IPAddress string         `json:"ip_address,omitempty"`
	UserAgent string         `json:"user_agent,omitempty"`
	Resource  string         `json:"resource,omitempty"`
	Action    string         `json:"action,omitempty"`
	Success   bool           `json:"success"`
	Details   string         `json:"details,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// AuditLogger handles audit event logging.
type AuditLogger struct {
	logger *slog.Logger
	store  AuditStore
}

// AuditStore is the interface for persisting audit events.
type AuditStore interface {
	SaveAuditEvent(ctx context.Context, event *AuditEvent) error
	ListAuditEvents(ctx context.Context, filter AuditFilter) ([]*AuditEvent, error)
}

// AuditFilter holds filtering options for audit events.
type AuditFilter struct {
	UserID    int64
	TenantID  int64
	EventType AuditEventType
	StartTime time.Time
	EndTime   time.Time
	Limit     int
	Offset    int

	// Resource and EventTypes, added for MOD-3's audit_log dashboard
	// source (DASHBOARD_SPEC.md §5.2): Resource matches AuditEvent's own
	// Resource field ("settings", "user", "profile"); EventTypes is the
	// plural form the source's own JSON property is named, checked by
	// membership rather than exact match -- kept alongside the existing
	// singular EventType rather than replacing it, since nothing in
	// production set EventType before this and there's no reason to
	// force a rename on an unused field.
	Resource   string
	EventTypes []AuditEventType
}

// NewAuditLogger creates a new audit logger.
func NewAuditLogger(logger *slog.Logger, store AuditStore) *AuditLogger {
	if logger == nil {
		logger = slog.Default()
	}
	return &AuditLogger{
		logger: logger,
		store:  store,
	}
}

// Log logs an audit event. Nil-receiver safe -- a nil *AuditLogger is a
// legitimate value (test harnesses construct UIHandler with one; any
// future caller might too) and audit logging is a cross-cutting,
// non-critical-path concern that must never be capable of crashing the
// request it's observing. Found necessary, not assumed: this method had
// no such guard until MOD-3 wired real callers into it and confirmed, by
// tracing why a first pass of tests didn't catch a missing one, that
// existing tests only avoided the nil-pointer panic by accident (no
// auth context to satisfy the caller's own claims check), not because
// this method was actually safe.
func (a *AuditLogger) Log(ctx context.Context, event *AuditEvent) {
	if a == nil {
		return
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	// Always log to structured logger
	attrs := []any{
		"event_type", event.EventType,
		"success", event.Success,
	}
	if event.UserID > 0 {
		attrs = append(attrs, "user_id", event.UserID)
	}
	if event.Email != "" {
		attrs = append(attrs, "email", event.Email)
	}
	if event.TenantID > 0 {
		attrs = append(attrs, "tenant_id", event.TenantID)
	}
	if event.IPAddress != "" {
		attrs = append(attrs, "ip_address", event.IPAddress)
	}
	if event.Resource != "" {
		attrs = append(attrs, "resource", event.Resource)
	}
	if event.Action != "" {
		attrs = append(attrs, "action", event.Action)
	}
	if event.Details != "" {
		attrs = append(attrs, "details", event.Details)
	}

	if event.Success {
		a.logger.Info("audit", attrs...)
	} else {
		a.logger.Warn("audit", attrs...)
	}

	// Persist to store if available
	if a.store != nil {
		if err := a.store.SaveAuditEvent(ctx, event); err != nil {
			a.logger.Error("failed to save audit event", "error", err)
		}
	}
}

// LogLogin logs a login attempt.
// List queries audit events via the configured store, for callers outside
// this package (the dashboard engine's audit_log source, MOD-3). Nil-
// receiver safe and a nil store both degrade to an empty result rather
// than a panic or an error -- same reasoning Log's own nil-receiver guard
// gives: a caller holding a not-yet-configured AuditLogger shouldn't
// crash on it, and "no audit data available" is a legitimate, renderable
// state for a dashboard widget (its own empty-state handling), not a
// fatal one.
func (a *AuditLogger) List(ctx context.Context, filter AuditFilter) ([]*AuditEvent, error) {
	if a == nil || a.store == nil {
		return nil, nil
	}
	return a.store.ListAuditEvents(ctx, filter)
}

func (a *AuditLogger) LogLogin(ctx context.Context, r *http.Request, userID int64, email string, success bool, details string) {
	eventType := AuditLoginSuccess
	if !success {
		eventType = AuditLoginFailed
	}

	a.Log(ctx, &AuditEvent{
		EventType: eventType,
		UserID:    userID,
		Email:     email,
		IPAddress: getClientIP(r),
		UserAgent: r.UserAgent(),
		Success:   success,
		Details:   details,
	})
}

// LogLoginLocked logs a login attempt blocked due to lockout.
func (a *AuditLogger) LogLoginLocked(ctx context.Context, r *http.Request, email string, remainingLock time.Duration) {
	a.Log(ctx, &AuditEvent{
		EventType: AuditLoginLocked,
		Email:     email,
		IPAddress: getClientIP(r),
		UserAgent: r.UserAgent(),
		Success:   false,
		Details:   "Account locked due to too many failed attempts",
		Metadata: map[string]any{
			"remaining_lock_seconds": int(remainingLock.Seconds()),
		},
	})
}

// LogLogout logs a logout event.
func (a *AuditLogger) LogLogout(ctx context.Context, r *http.Request, userID int64, email string) {
	a.Log(ctx, &AuditEvent{
		EventType: AuditLogout,
		UserID:    userID,
		Email:     email,
		IPAddress: getClientIP(r),
		UserAgent: r.UserAgent(),
		Success:   true,
	})
}

// LogPasswordChanged logs a password change.
func (a *AuditLogger) LogPasswordChanged(ctx context.Context, r *http.Request, userID int64, email string, byAdmin bool) {
	details := "User changed their own password"
	if byAdmin {
		details = "Password reset by administrator"
	}

	a.Log(ctx, &AuditEvent{
		EventType: AuditPasswordChanged,
		UserID:    userID,
		Email:     email,
		IPAddress: getClientIP(r),
		UserAgent: r.UserAgent(),
		Success:   true,
		Details:   details,
		Metadata: map[string]any{
			"by_admin": byAdmin,
		},
	})
}

// LogUserCreated logs user creation.
func (a *AuditLogger) LogUserCreated(ctx context.Context, r *http.Request, actorID int64, newUserID int64, email string, role string) {
	a.Log(ctx, &AuditEvent{
		EventType: AuditUserCreated,
		UserID:    actorID,
		Email:     email,
		IPAddress: getClientIP(r),
		UserAgent: r.UserAgent(),
		Resource:  "user",
		Action:    "create",
		Success:   true,
		Metadata: map[string]any{
			"new_user_id": newUserID,
			"email":       email,
			"role":        role,
		},
	})
}

// LogUserUpdated logs user update.
func (a *AuditLogger) LogUserUpdated(ctx context.Context, r *http.Request, actorID int64, targetUserID int64, email string, changes map[string]any) {
	a.Log(ctx, &AuditEvent{
		EventType: AuditUserUpdated,
		UserID:    actorID,
		Email:     email,
		IPAddress: getClientIP(r),
		UserAgent: r.UserAgent(),
		Resource:  "user",
		Action:    "update",
		Success:   true,
		Metadata: map[string]any{
			"target_user_id": targetUserID,
			"changes":        changes,
		},
	})
}

// LogDeveloperChange logs a change made from the Developer module. what names
// the kind of change ("feature_flag", "locale_override", ...), key the thing
// changed, and detail carries the values (for a flag: the new value, or that
// it was reset).
func (a *AuditLogger) LogDeveloperChange(ctx context.Context, r *http.Request, actorID, tenantID int64, what, key string, detail map[string]any) {
	a.Log(ctx, &AuditEvent{
		EventType: AuditDeveloperChanged,
		UserID:    actorID,
		TenantID:  tenantID,
		IPAddress: getClientIP(r),
		UserAgent: r.UserAgent(),
		Resource:  "developer",
		Action:    what,
		Success:   true,
		Details:   key,
		Metadata:  detail,
	})
}

// LogUserDisabled logs user account disable.
func (a *AuditLogger) LogUserDisabled(ctx context.Context, r *http.Request, actorID int64, targetUserID int64, email string) {
	a.Log(ctx, &AuditEvent{
		EventType: AuditUserDisabled,
		UserID:    actorID,
		Email:     email,
		IPAddress: getClientIP(r),
		UserAgent: r.UserAgent(),
		Resource:  "user",
		Action:    "disable",
		Success:   true,
		Details:   "User account disabled",
		Metadata: map[string]any{
			"target_user_id": targetUserID,
			"email":          email,
		},
	})
}

// LogUserEnabled logs user account enable.
func (a *AuditLogger) LogUserEnabled(ctx context.Context, r *http.Request, actorID int64, targetUserID int64, email string) {
	a.Log(ctx, &AuditEvent{
		EventType: AuditUserEnabled,
		UserID:    actorID,
		Email:     email,
		IPAddress: getClientIP(r),
		UserAgent: r.UserAgent(),
		Resource:  "user",
		Action:    "enable",
		Success:   true,
		Details:   "User account enabled",
		Metadata: map[string]any{
			"target_user_id": targetUserID,
			"email":          email,
		},
	})
}

// LogRoleChanged logs role change.
func (a *AuditLogger) LogRoleChanged(ctx context.Context, r *http.Request, actorID int64, targetUserID int64, email, oldRole, newRole string) {
	a.Log(ctx, &AuditEvent{
		EventType: AuditRoleChanged,
		UserID:    actorID,
		Email:     email,
		IPAddress: getClientIP(r),
		UserAgent: r.UserAgent(),
		Resource:  "user",
		Action:    "role_change",
		Success:   true,
		Details:   "User role changed from " + oldRole + " to " + newRole,
		Metadata: map[string]any{
			"target_user_id": targetUserID,
			"old_role":       oldRole,
			"new_role":       newRole,
		},
	})
}

// LogSettingsChanged logs a tenant settings save (general/notifications/
// features). section names which settings tab was saved -- the actual
// field-level diff isn't tracked, matching LogRoleChanged's own level of
// detail (old/new role, not every changed field) rather than building a
// full change-diff mechanism for a first pass at wiring this up at all.
func (a *AuditLogger) LogSettingsChanged(ctx context.Context, r *http.Request, actorID int64, email, section string) {
	a.Log(ctx, &AuditEvent{
		EventType: AuditSettingsChanged,
		UserID:    actorID,
		Email:     email,
		IPAddress: getClientIP(r),
		UserAgent: r.UserAgent(),
		Resource:  "settings",
		Action:    section,
		Success:   true,
		Details:   "Tenant settings (" + section + ") updated",
		Metadata: map[string]any{
			"section": section,
		},
	})
}

// LogPreferencesChanged logs a user updating their own profile
// preferences (locale, theme, etc.) -- self-only, no actor/target
// distinction the way LogUserUpdated needs one.
func (a *AuditLogger) LogPreferencesChanged(ctx context.Context, r *http.Request, userID int64) {
	a.Log(ctx, &AuditEvent{
		EventType: AuditPreferencesChanged,
		UserID:    userID,
		IPAddress: getClientIP(r),
		UserAgent: r.UserAgent(),
		Resource:  "profile",
		Action:    "preferences_update",
		Success:   true,
		Details:   "User updated their own preferences",
	})
}

// LogAccessDenied logs an access denied event.
func (a *AuditLogger) LogAccessDenied(ctx context.Context, r *http.Request, userID int64, resource, requiredPerm string) {
	a.Log(ctx, &AuditEvent{
		EventType: AuditAccessDenied,
		UserID:    userID,
		IPAddress: getClientIP(r),
		UserAgent: r.UserAgent(),
		Resource:  resource,
		Success:   false,
		Details:   "Permission denied: " + requiredPerm,
		Metadata: map[string]any{
			"required_permission": requiredPerm,
			"path":                r.URL.Path,
			"method":              r.Method,
		},
	})
}

// LogCSRFViolation logs a CSRF token violation.
func (a *AuditLogger) LogCSRFViolation(ctx context.Context, r *http.Request, userID int64) {
	a.Log(ctx, &AuditEvent{
		EventType: AuditCSRFViolation,
		UserID:    userID,
		IPAddress: getClientIP(r),
		UserAgent: r.UserAgent(),
		Resource:  r.URL.Path,
		Success:   false,
		Details:   "CSRF token validation failed",
		Metadata: map[string]any{
			"path":   r.URL.Path,
			"method": r.Method,
		},
	})
}

// getClientIP extracts the client IP from the request.
func getClientIP(r *http.Request) string {
	if r == nil {
		return ""
	}

	// Check X-Forwarded-For header (for proxied requests)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	addr := r.RemoteAddr
	// Remove port if present
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}

// MarshalJSON implements json.Marshaler for AuditEvent.
func (e *AuditEvent) MarshalJSON() ([]byte, error) {
	type alias AuditEvent
	return json.Marshal((*alias)(e))
}
