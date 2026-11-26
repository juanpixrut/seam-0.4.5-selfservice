package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"time"
)

// CSRF errors
var (
	ErrInvalidCSRFToken = errors.New("invalid CSRF token")
	ErrMissingCSRFToken = errors.New("missing CSRF token")
)

const (
	// CSRFCookieName is the name of the CSRF cookie
	CSRFCookieName = "seam_csrf"
	// CSRFFormField is the name of the hidden form field
	CSRFFormField = "_csrf"
	// CSRFHeaderName is the header name for AJAX requests
	CSRFHeaderName = "X-CSRF-Token"
	// CSRFTokenLength is the length of the token in bytes
	CSRFTokenLength = 32
)

// CSRFManager handles CSRF token generation and validation.
type CSRFManager struct {
	secure     bool          // Use secure cookies (HTTPS only)
	cookiePath string        // Cookie path
	duration   time.Duration // Token validity duration
}

// CSRFConfig holds CSRF manager configuration.
type CSRFConfig struct {
	Secure     bool
	CookiePath string
	Duration   time.Duration
}

// NewCSRFManager creates a new CSRF manager.
func NewCSRFManager(cfg CSRFConfig) *CSRFManager {
	if cfg.CookiePath == "" {
		cfg.CookiePath = "/"
	}
	if cfg.Duration == 0 {
		cfg.Duration = 24 * time.Hour
	}

	return &CSRFManager{
		secure:     cfg.Secure,
		cookiePath: cfg.CookiePath,
		duration:   cfg.Duration,
	}
}

// GenerateToken generates a new CSRF token and sets it as a cookie.
// Returns the token value to be embedded in forms.
func (m *CSRFManager) GenerateToken(w http.ResponseWriter) (string, error) {
	// Generate random bytes
	tokenBytes := make([]byte, CSRFTokenLength)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}

	// Encode as base64
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)

	// Set as cookie
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    token,
		Path:     m.cookiePath,
		Expires:  time.Now().Add(m.duration),
		HttpOnly: true, // Cookie not accessible via JavaScript
		Secure:   m.secure,
		SameSite: http.SameSiteStrictMode, // Strict for CSRF protection
	})

	return token, nil
}

// GetToken returns the current CSRF token from the cookie.
// If no token exists, generates a new one.
func (m *CSRFManager) GetToken(w http.ResponseWriter, r *http.Request) (string, error) {
	// Try to get existing token from cookie
	cookie, err := r.Cookie(CSRFCookieName)
	if err == nil && cookie.Value != "" {
		return cookie.Value, nil
	}

	// Generate new token
	return m.GenerateToken(w)
}

// ValidateToken validates the CSRF token from the request.
// Checks form field first, then header (for AJAX).
func (m *CSRFManager) ValidateToken(r *http.Request) error {
	// Get token from cookie
	cookie, err := r.Cookie(CSRFCookieName)
	if err != nil {
		return ErrMissingCSRFToken
	}
	expectedToken := cookie.Value

	// Get token from form or header
	providedToken := r.FormValue(CSRFFormField)
	if providedToken == "" {
		providedToken = r.Header.Get(CSRFHeaderName)
	}

	if providedToken == "" {
		return ErrMissingCSRFToken
	}

	// Constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare([]byte(expectedToken), []byte(providedToken)) != 1 {
		return ErrInvalidCSRFToken
	}

	return nil
}

// Middleware returns HTTP middleware that validates CSRF tokens on
// state-changing requests (POST, PUT, PATCH, DELETE).
func (m *CSRFManager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only validate on state-changing methods
		switch r.Method {
		case "POST", "PUT", "PATCH", "DELETE":
			if err := m.ValidateToken(r); err != nil {
				http.Error(w, "CSRF validation failed", http.StatusForbidden)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// MiddlewareFunc is the same as Middleware but for http.HandlerFunc.
func (m *CSRFManager) MiddlewareFunc(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only validate on state-changing methods
		switch r.Method {
		case "POST", "PUT", "PATCH", "DELETE":
			if err := m.ValidateToken(r); err != nil {
				http.Error(w, "CSRF validation failed", http.StatusForbidden)
				return
			}
		}

		next.ServeHTTP(w, r)
	}
}

// ClearToken clears the CSRF cookie.
func (m *CSRFManager) ClearToken(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    "",
		Path:     m.cookiePath,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteStrictMode,
	})
}
