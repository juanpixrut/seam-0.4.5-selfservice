# RBAC Completion Plan

## Current State

The RBAC infrastructure is in place:
- ✓ bcrypt password hashing
- ✓ UserService with full account management
- ✓ Permission middleware (`RequirePermission`, `RequireRole`, etc.)
- ✓ API routes wrapped with permission checks
- ✓ Admin UI for user management
- ✓ CLI tool (seam-admin)

What's missing is the **authentication flow** and **UI route protection**.

---

## Phase 5: Authentication Flow

### 5.1 Login Page & Session Management

**Goal:** Users can log in via the UI and maintain a session.

**Files to create/modify:**
- `internal/ui/login.go` — login page template
- `internal/ui/routes.go` — login/logout handlers
- `internal/ui/router.go` — public vs protected route groups
- `internal/auth/session.go` — session token generation/validation

**Implementation:**

```
GET  /login          → LoginPage (public)
POST /login          → LoginSubmit → validate credentials → set session cookie → redirect
GET  /logout         → clear session cookie → redirect to /login
```

**Session approach:** Signed cookie containing user ID + expiry. Simple, stateless, no Redis needed.

```go
// Session cookie structure (base64-encoded, HMAC-signed)
type SessionData struct {
    UserID    int64     `json:"uid"`
    TenantID  int64     `json:"tid"`
    Role      string    `json:"role"`
    ExpiresAt time.Time `json:"exp"`
}
```

**Estimated operations:** 8-10

---

### 5.2 UI Auth Middleware

**Goal:** Protect UI routes, redirect unauthenticated users to login.

**Files to modify:**
- `internal/ui/router.go` — add auth middleware
- `internal/ui/middleware.go` — new file for UI-specific middleware

**Implementation:**

```go
// UI auth middleware - redirects to /login instead of returning 401
func (r *Router) requireAuth(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, req *http.Request) {
        session, err := r.validateSession(req)
        if err != nil {
            http.Redirect(w, req, "/login?next="+url.QueryEscape(req.URL.Path), http.StatusSeeOther)
            return
        }
        ctx := auth.ContextWithClaims(req.Context(), session.ToClaims())
        next.ServeHTTP(w, req.WithContext(ctx))
    }
}
```

**Route groups:**
```go
// Public routes (no auth)
r.mux.HandleFunc("GET /login", r.handler.LoginPage)
r.mux.HandleFunc("POST /login", r.handler.LoginSubmit)
r.mux.HandleFunc("GET /health", r.handler.Health)

// Protected routes (require auth)
r.mux.HandleFunc("GET /", r.requireAuth(r.handler.Dashboard))
r.mux.HandleFunc("GET /assets", r.requireAuth(r.handler.AssetsList))
// ... all other routes
```

**Estimated operations:** 6-8

---

### 5.3 UI Permission Middleware

**Goal:** Protect admin routes from non-admin users.

**Implementation:**

```go
// Require specific permission for UI routes
func (r *Router) requirePerm(perm auth.Permission, next http.HandlerFunc) http.HandlerFunc {
    return r.requireAuth(func(w http.ResponseWriter, req *http.Request) {
        claims, _ := auth.ClaimsFromContext(req.Context())
        if !auth.HasPermission(claims.Role, perm) {
            r.handler.Error403(w, req)
            return
        }
        next.ServeHTTP(w, req)
    })
}

// Usage
r.mux.HandleFunc("GET /admin/users", r.requirePerm(auth.PermUserRead, r.handler.AdminUsersList))
r.mux.HandleFunc("POST /admin/users", r.requirePerm(auth.PermUserCreate, r.handler.AdminUserCreate))
```

**Estimated operations:** 4-5

---

## Phase 6: Security Hardening

### 6.1 CSRF Protection

**Goal:** Prevent cross-site request forgery on state-changing operations.

**Files to create/modify:**
- `internal/ui/middleware.go` — CSRF token generation/validation
- `internal/ui/components.go` — hidden CSRF field in forms

**Implementation:**
- Generate CSRF token per session, store in cookie
- Include token in all forms as hidden field
- Validate on POST/PUT/DELETE requests

```go
// In form rendering
mi.Input(mi.TypeHidden, mi.Name("_csrf"), mi.Value(csrfToken))

// In middleware
func (r *Router) validateCSRF(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, req *http.Request) {
        if req.Method == "POST" || req.Method == "PUT" || req.Method == "DELETE" {
            token := req.FormValue("_csrf")
            expected := getCSRFFromCookie(req)
            if !secureCompare(token, expected) {
                http.Error(w, "Invalid CSRF token", http.StatusForbidden)
                return
            }
        }
        next.ServeHTTP(w, req)
    }
}
```

**Estimated operations:** 5-6

---

### 6.2 Brute-Force Protection

**Goal:** Prevent password guessing attacks.

**Files to create/modify:**
- `internal/auth/ratelimit.go` — login attempt tracking
- `internal/ui/routes.go` — integrate with login handler

**Implementation:**
- Track failed attempts per email (in-memory or olu)
- After 5 failures: 1-minute lockout
- After 10 failures: 15-minute lockout
- After 20 failures: 1-hour lockout
- Successful login resets counter

```go
type LoginAttemptTracker struct {
    attempts map[string]*attemptRecord
    mu       sync.RWMutex
}

type attemptRecord struct {
    Count     int
    LastAt    time.Time
    LockedUntil time.Time
}
```

**Estimated operations:** 4-5

---

### 6.3 Audit Logging

**Goal:** Record security-relevant events.

**Files to create/modify:**
- `internal/audit/audit.go` — audit log service
- `internal/audit/events.go` — event type definitions
- Integration points in auth handlers

**Events to log:**
- `auth.login.success` — user, IP, timestamp
- `auth.login.failure` — email attempted, IP, reason
- `auth.logout` — user, IP
- `auth.password.changed` — user, changed_by
- `auth.password.reset` — user, reset_by (admin)
- `user.created` — user, created_by
- `user.updated` — user, updated_by, fields changed
- `user.deleted` — user, deleted_by
- `user.disabled` — user, disabled_by
- `permission.denied` — user, resource, permission required

**Storage:** Write to olu as `audit_logs` entity, or separate file.

**Estimated operations:** 8-10

---

## Phase 7: Self-Service Features

### 7.1 Password Reset Flow

**Goal:** Users can reset forgotten passwords via email.

**Files to create/modify:**
- `internal/auth/reset.go` — reset token generation/validation
- `internal/ui/password_reset.go` — forgot password pages
- `internal/notify/email.go` — email sending (if not exists)

**Flow:**
```
GET  /forgot-password     → form to enter email
POST /forgot-password     → generate token, send email, show confirmation
GET  /reset-password?t=X  → validate token, show new password form
POST /reset-password      → validate token, set new password, redirect to login
```

**Token structure:**
```go
type ResetToken struct {
    UserID    int64
    Token     string    // random 32-byte hex
    ExpiresAt time.Time // 1 hour from creation
    Used      bool
}
```

**Estimated operations:** 10-12

---

### 7.2 Email Verification (Optional)

**Goal:** Verify email addresses on account creation.

Lower priority — can be added later if needed.

---

## Implementation Order

| Phase | Description | Operations | Priority | Status |
|-------|-------------|------------|----------|--------|
| 5.1 | Login page & sessions | 8-10 | **Critical** | ✓ DONE |
| 5.2 | UI auth middleware | 6-8 | **Critical** | ✓ DONE |
| 5.3 | UI permission middleware | 4-5 | **Critical** | ✓ DONE (merged into 5.2) |
| 6.1 | CSRF protection | 5-6 | High | ✓ DONE |
| 6.2 | Brute-force protection | 4-5 | High | ✓ DONE |
| 6.3 | Audit logging | 8-10 | Medium | ✓ DONE |
| 7.1 | Password reset flow | 10-12 | Medium | ✓ DONE |

**Total estimated operations:** 45-56

---

## Testing Requirements

### Integration Tests Needed

1. **Login flow test**
   - Valid credentials → session created → can access protected route
   - Invalid credentials → error shown → no session
   - Session expiry → redirected to login

2. **Permission test**
   - Viewer tries to access /admin/users → 403
   - Admin accesses /admin/users → 200
   - Operator creates asset → 200
   - Viewer creates asset → 403

3. **Brute-force test**
   - 5 failed logins → temporary lockout
   - Lockout expires → can try again

4. **CSRF test**
   - POST without token → 403
   - POST with valid token → success
   - POST with old token → 403

---

## Configuration Additions

```json
{
  "auth": {
    "session_secret": "change-me-in-production",
    "session_duration": "24h",
    "csrf_enabled": true,
    "lockout_threshold": 5,
    "lockout_duration": "15m",
    "password_reset_expiry": "1h"
  }
}
```

---

## Migration Notes

For existing deployments:

1. **Before upgrade:** Create admin user via CLI
   ```bash
   seam-admin user create --email=admin@example.com --name="Admin" --role=admin
   ```

2. **After upgrade:** All UI access requires login

3. **First login:** Use credentials created in step 1

---

## Definition of Done

RBAC is complete when:

- [x] Users must log in to access any UI page except /login
- [x] Session persists across browser refresh (cookie-based)
- [x] Logout clears session
- [x] /admin/* routes require user:* permissions
- [x] CSRF tokens protect all forms
- [x] Failed logins trigger temporary lockout
- [x] Security events are logged
- [x] Integration tests pass for all auth flows

**ALL PHASES COMPLETE** ✓
