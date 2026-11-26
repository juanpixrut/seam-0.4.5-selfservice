package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

// Session errors
var (
	ErrInvalidSession = errors.New("invalid session")
	ErrSessionExpired = errors.New("session expired")
	ErrNoSession      = errors.New("no session found")
)

// SessionData holds the data stored in a session cookie.
type SessionData struct {
	UserID    int64     `json:"uid"`
	TenantID  int64     `json:"tid"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Name      string    `json:"name"`
	ExpiresAt time.Time `json:"exp"`

	// ViewAs is the role a developer chose to view the application as (see
	// viewas.go); empty for none. It is only honoured for a developer session.
	ViewAs string `json:"va,omitempty"`
}

// ToClaims converts session data to auth claims.
func (s *SessionData) ToClaims() *Claims {
	return &Claims{
		UserID:    s.UserID,
		TenantID:  s.TenantID,
		Email:     s.Email,
		Role:      s.Role,
		TokenType: TokenTypeAccess,
		IssuedAt:  time.Now(),
		ExpiresAt: s.ExpiresAt,
	}
}

// SessionManager handles session creation and validation.
type SessionManager struct {
	secret     []byte
	duration   time.Duration
	cookieName string
	secure     bool // Use secure cookies (HTTPS only)
}

// SessionConfig holds session manager configuration.
type SessionConfig struct {
	Secret     string
	Duration   time.Duration
	CookieName string
	Secure     bool
}

// NewSessionManager creates a new session manager.
func NewSessionManager(cfg SessionConfig) *SessionManager {
	if cfg.CookieName == "" {
		cfg.CookieName = "seam_session"
	}
	if cfg.Duration == 0 {
		cfg.Duration = 24 * time.Hour
	}

	secret := []byte(cfg.Secret)
	if len(secret) == 0 {
		// Generate random secret if none provided (development only)
		secret = make([]byte, 32)
		rand.Read(secret)
	}

	return &SessionManager{
		secret:     secret,
		duration:   cfg.Duration,
		cookieName: cfg.CookieName,
		secure:     cfg.Secure,
	}
}

// CreateSession creates a session for a user and sets the cookie.
func (m *SessionManager) CreateSession(w http.ResponseWriter, user *User) error {
	data := SessionData{
		UserID:    user.ID,
		TenantID:  user.TenantID,
		Email:     user.Email,
		Role:      user.Role,
		Name:      user.Name,
		ExpiresAt: time.Now().Add(m.duration),
	}

	// Encode session data
	encoded, err := m.encode(data)
	if err != nil {
		return err
	}

	m.setCookie(w, encoded, data.ExpiresAt)
	return nil
}

func (m *SessionManager) setCookie(w http.ResponseWriter, value string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     m.cookieName,
		Value:    value,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ErrViewAs is returned when a session may not view the application as the
// role asked for.
var ErrViewAs = errors.New("auth: this session may not view the application as that role")

// SetViewAs sets, or with an empty role clears, the role the session views the
// application as, and sends the updated cookie. The expiry is the session's own:
// choosing a role does not extend it. Only a developer session may set one.
func (m *SessionManager) SetViewAs(w http.ResponseWriter, r *http.Request, role string) error {
	data, err := m.ValidateSession(r)
	if err != nil {
		return err
	}
	if role != "" && !CanViewAs(data.Role, role) {
		return ErrViewAs
	}
	data.ViewAs = role
	encoded, err := m.encode(*data)
	if err != nil {
		return err
	}
	m.setCookie(w, encoded, data.ExpiresAt)
	return nil
}

// ValidateSession validates the session from the request.
func (m *SessionManager) ValidateSession(r *http.Request) (*SessionData, error) {
	cookie, err := r.Cookie(m.cookieName)
	if err != nil {
		return nil, ErrNoSession
	}

	data, err := m.decode(cookie.Value)
	if err != nil {
		return nil, err
	}

	if time.Now().After(data.ExpiresAt) {
		return nil, ErrSessionExpired
	}

	return data, nil
}

// ClearSession clears the session cookie.
func (m *SessionManager) ClearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     m.cookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// encode encodes and signs session data.
func (m *SessionManager) encode(data SessionData) (string, error) {
	// JSON encode the data
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	// Base64 encode
	encoded := base64.RawURLEncoding.EncodeToString(jsonData)

	// Create HMAC signature
	sig := m.sign(encoded)

	// Return data.signature format
	return encoded + "." + sig, nil
}

// decode decodes and verifies session data.
func (m *SessionManager) decode(value string) (*SessionData, error) {
	// Split into data and signature
	parts := strings.SplitN(value, ".", 2)
	if len(parts) != 2 {
		return nil, ErrInvalidSession
	}

	encoded, providedSig := parts[0], parts[1]

	// Verify signature
	expectedSig := m.sign(encoded)
	if !hmac.Equal([]byte(providedSig), []byte(expectedSig)) {
		return nil, ErrInvalidSession
	}

	// Decode base64
	jsonData, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, ErrInvalidSession
	}

	// Unmarshal JSON
	var data SessionData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, ErrInvalidSession
	}

	return &data, nil
}

// sign creates an HMAC signature for the data.
func (m *SessionManager) sign(data string) string {
	h := hmac.New(sha256.New, m.secret)
	h.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
