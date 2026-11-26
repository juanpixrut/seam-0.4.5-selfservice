package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionManager_CreateAndValidate(t *testing.T) {
	mgr := NewSessionManager(SessionConfig{
		Secret:   "test-secret-key-32-bytes-long!!",
		Duration: time.Hour,
	})

	user := &User{
		ID:       123,
		TenantID: 1,
		Email:    "test@example.com",
		Role:     RoleAdmin,
		Name:     "Test User",
	}

	// Create session
	w := httptest.NewRecorder()
	err := mgr.CreateSession(w, user)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	// Get the cookie
	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("No cookies set")
	}

	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "seam_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("Session cookie not found")
	}

	// Validate session
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(sessionCookie)

	session, err := mgr.ValidateSession(req)
	if err != nil {
		t.Fatalf("ValidateSession() error = %v", err)
	}

	if session.UserID != user.ID {
		t.Errorf("UserID = %d, want %d", session.UserID, user.ID)
	}
	if session.Email != user.Email {
		t.Errorf("Email = %s, want %s", session.Email, user.Email)
	}
	if session.Role != user.Role {
		t.Errorf("Role = %s, want %s", session.Role, user.Role)
	}
}

func TestSessionManager_InvalidSignature(t *testing.T) {
	mgr := NewSessionManager(SessionConfig{
		Secret:   "test-secret-key-32-bytes-long!!",
		Duration: time.Hour,
	})

	// Create a fake session cookie with wrong signature
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  "seam_session",
		Value: "eyJ1aWQiOjEyM30.invalid-signature",
	})

	_, err := mgr.ValidateSession(req)
	if err != ErrInvalidSession {
		t.Errorf("ValidateSession() error = %v, want %v", err, ErrInvalidSession)
	}
}

func TestSessionManager_ExpiredSession(t *testing.T) {
	mgr := NewSessionManager(SessionConfig{
		Secret:   "test-secret-key-32-bytes-long!!",
		Duration: -time.Hour, // Already expired
	})

	user := &User{
		ID:       123,
		TenantID: 1,
		Email:    "test@example.com",
		Role:     RoleAdmin,
	}

	// Create expired session
	w := httptest.NewRecorder()
	err := mgr.CreateSession(w, user)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	cookies := w.Result().Cookies()
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(cookies[0])

	_, err = mgr.ValidateSession(req)
	if err != ErrSessionExpired {
		t.Errorf("ValidateSession() error = %v, want %v", err, ErrSessionExpired)
	}
}

func TestSessionManager_NoSession(t *testing.T) {
	mgr := NewSessionManager(SessionConfig{
		Secret:   "test-secret-key-32-bytes-long!!",
		Duration: time.Hour,
	})

	req := httptest.NewRequest("GET", "/", nil)

	_, err := mgr.ValidateSession(req)
	if err != ErrNoSession {
		t.Errorf("ValidateSession() error = %v, want %v", err, ErrNoSession)
	}
}

func TestSessionManager_ClearSession(t *testing.T) {
	mgr := NewSessionManager(SessionConfig{
		Secret:   "test-secret-key-32-bytes-long!!",
		Duration: time.Hour,
	})

	w := httptest.NewRecorder()
	mgr.ClearSession(w)

	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("No cookies set")
	}

	// Cookie should have MaxAge -1 (delete)
	if cookies[0].MaxAge != -1 {
		t.Errorf("Cookie MaxAge = %d, want -1", cookies[0].MaxAge)
	}
}

func TestSessionData_ToClaims(t *testing.T) {
	session := &SessionData{
		UserID:    123,
		TenantID:  1,
		Email:     "test@example.com",
		Role:      RoleAdmin,
		Name:      "Test User",
		ExpiresAt: time.Now().Add(time.Hour),
	}

	claims := session.ToClaims()

	if claims.UserID != session.UserID {
		t.Errorf("UserID = %d, want %d", claims.UserID, session.UserID)
	}
	if claims.TenantID != session.TenantID {
		t.Errorf("TenantID = %d, want %d", claims.TenantID, session.TenantID)
	}
	if claims.Email != session.Email {
		t.Errorf("Email = %s, want %s", claims.Email, session.Email)
	}
	if claims.Role != session.Role {
		t.Errorf("Role = %s, want %s", claims.Role, session.Role)
	}
	if claims.TokenType != TokenTypeAccess {
		t.Errorf("TokenType = %s, want %s", claims.TokenType, TokenTypeAccess)
	}
}
