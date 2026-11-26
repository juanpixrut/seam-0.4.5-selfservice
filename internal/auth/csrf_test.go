package auth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestCSRFManager_GenerateToken(t *testing.T) {
	mgr := NewCSRFManager(CSRFConfig{})

	w := httptest.NewRecorder()
	token, err := mgr.GenerateToken(w)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	if token == "" {
		t.Error("GenerateToken() returned empty token")
	}

	// Check cookie was set
	cookies := w.Result().Cookies()
	var csrfCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == CSRFCookieName {
			csrfCookie = c
			break
		}
	}

	if csrfCookie == nil {
		t.Fatal("CSRF cookie not set")
	}

	if csrfCookie.Value != token {
		t.Errorf("Cookie value = %s, want %s", csrfCookie.Value, token)
	}

	if !csrfCookie.HttpOnly {
		t.Error("Cookie should be HttpOnly")
	}

	if csrfCookie.SameSite != http.SameSiteStrictMode {
		t.Error("Cookie should have SameSite=Strict")
	}
}

func TestCSRFManager_GetToken_ExistingCookie(t *testing.T) {
	mgr := NewCSRFManager(CSRFConfig{})

	existingToken := "existing-csrf-token"

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  CSRFCookieName,
		Value: existingToken,
	})

	w := httptest.NewRecorder()
	token, err := mgr.GetToken(w, req)
	if err != nil {
		t.Fatalf("GetToken() error = %v", err)
	}

	if token != existingToken {
		t.Errorf("GetToken() = %s, want %s", token, existingToken)
	}
}

func TestCSRFManager_GetToken_NoCookie(t *testing.T) {
	mgr := NewCSRFManager(CSRFConfig{})

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	token, err := mgr.GetToken(w, req)
	if err != nil {
		t.Fatalf("GetToken() error = %v", err)
	}

	if token == "" {
		t.Error("GetToken() returned empty token")
	}

	// Should have set a new cookie
	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Error("GetToken() should set cookie when none exists")
	}
}

func TestCSRFManager_ValidateToken_Valid(t *testing.T) {
	mgr := NewCSRFManager(CSRFConfig{})

	token := "valid-csrf-token"

	// Create request with cookie and form value
	form := url.Values{}
	form.Set(CSRFFormField, token)

	req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{
		Name:  CSRFCookieName,
		Value: token,
	})

	err := mgr.ValidateToken(req)
	if err != nil {
		t.Errorf("ValidateToken() error = %v, want nil", err)
	}
}

func TestCSRFManager_ValidateToken_Header(t *testing.T) {
	mgr := NewCSRFManager(CSRFConfig{})

	token := "valid-csrf-token"

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set(CSRFHeaderName, token)
	req.AddCookie(&http.Cookie{
		Name:  CSRFCookieName,
		Value: token,
	})

	err := mgr.ValidateToken(req)
	if err != nil {
		t.Errorf("ValidateToken() error = %v, want nil", err)
	}
}

func TestCSRFManager_ValidateToken_MissingCookie(t *testing.T) {
	mgr := NewCSRFManager(CSRFConfig{})

	form := url.Values{}
	form.Set(CSRFFormField, "some-token")

	req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	err := mgr.ValidateToken(req)
	if err != ErrMissingCSRFToken {
		t.Errorf("ValidateToken() error = %v, want %v", err, ErrMissingCSRFToken)
	}
}

func TestCSRFManager_ValidateToken_MissingFormValue(t *testing.T) {
	mgr := NewCSRFManager(CSRFConfig{})

	req := httptest.NewRequest("POST", "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  CSRFCookieName,
		Value: "some-token",
	})

	err := mgr.ValidateToken(req)
	if err != ErrMissingCSRFToken {
		t.Errorf("ValidateToken() error = %v, want %v", err, ErrMissingCSRFToken)
	}
}

func TestCSRFManager_ValidateToken_Mismatch(t *testing.T) {
	mgr := NewCSRFManager(CSRFConfig{})

	form := url.Values{}
	form.Set(CSRFFormField, "wrong-token")

	req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{
		Name:  CSRFCookieName,
		Value: "correct-token",
	})

	err := mgr.ValidateToken(req)
	if err != ErrInvalidCSRFToken {
		t.Errorf("ValidateToken() error = %v, want %v", err, ErrInvalidCSRFToken)
	}
}

func TestCSRFManager_Middleware_AllowsGET(t *testing.T) {
	mgr := NewCSRFManager(CSRFConfig{})

	called := false
	handler := mgr.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Error("Middleware blocked GET request")
	}
}

func TestCSRFManager_Middleware_BlocksInvalidPOST(t *testing.T) {
	mgr := NewCSRFManager(CSRFConfig{})

	called := false
	handler := mgr.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest("POST", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if called {
		t.Error("Middleware should block POST without CSRF token")
	}

	if w.Code != http.StatusForbidden {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestCSRFManager_Middleware_AllowsValidPOST(t *testing.T) {
	mgr := NewCSRFManager(CSRFConfig{})

	called := false
	handler := mgr.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	token := "valid-token"
	form := url.Values{}
	form.Set(CSRFFormField, token)

	req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{
		Name:  CSRFCookieName,
		Value: token,
	})

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("Middleware blocked valid POST")
	}
}
