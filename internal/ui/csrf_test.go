package ui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ha1tch/seam-ui/internal/auth"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

// testRouter builds a minimal Router.
// sessionManager=nil bypasses session validation; csrfManager still validates.
func testRouter(t *testing.T, xoluURL string, csrfMgr *auth.CSRFManager) *Router {
	t.Helper()
	client := xolu.New(xoluURL)
	cfg := Config{AppName: "Test", Version: "0.0.0"}
	h := NewUIHandler(client, cfg, nil, nil, nil, csrfMgr, nil, nil, nil)
	return &Router{
		mux:         http.NewServeMux(),
		handler:     h,
		csrfManager: csrfMgr,
	}
}

// csrfProtect wraps a handler with CSRF validation only — no session check.
// This mirrors the CSRF branch inside requireAuth when a session is present.
func csrfProtect(csrfMgr *auth.CSRFManager, h http.HandlerFunc, renderError func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	if csrfMgr == nil {
		return h
	}
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST", "PUT", "PATCH", "DELETE":
			if err := csrfMgr.ValidateToken(r); err != nil {
				if renderError != nil {
					renderError(w, r)
				} else {
					http.Error(w, "CSRF validation failed", http.StatusForbidden)
				}
				return
			}
		}
		h(w, r)
	}
}

func testCSRFManager() *auth.CSRFManager {
	return auth.NewCSRFManager(auth.CSRFConfig{
		Secure:     false,
		CookiePath: "/",
	})
}

// postWithCSRF fires a POST with matching cookie + form token.
func postWithCSRF(t *testing.T, handler http.HandlerFunc, target, token string, extraFields url.Values) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{}
	form.Set(auth.CSRFFormField, token)
	for k, vs := range extraFields {
		for _, v := range vs {
			form.Add(k, v)
		}
	}
	req := httptest.NewRequest("POST", target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: token})

	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

// TestCSRF_MissingTokenBlocked verifies that POST without CSRF cookie+token → 403.
func TestCSRF_MissingTokenBlocked(t *testing.T) {
	csrfMgr := testCSRFManager()
	// Wire a minimal handler that would succeed if CSRF passes
	reached := false
	protected := csrfProtect(csrfMgr, func(w http.ResponseWriter, req *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}, nil)

	req := httptest.NewRequest("POST", "/regions", strings.NewReader("descripcion=Test"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// No CSRF cookie, no CSRF form field
	rec := httptest.NewRecorder()
	protected(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for missing CSRF token, got %d", rec.Code)
	}
	if reached {
		t.Error("handler was reached despite missing CSRF token")
	}
}

// TestCSRF_WrongTokenBlocked verifies that mismatched cookie/form tokens → 403.
func TestCSRF_WrongTokenBlocked(t *testing.T) {
	csrfMgr := testCSRFManager()
	reached := false
	protected := csrfProtect(csrfMgr, func(w http.ResponseWriter, req *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}, nil)

	form := url.Values{}
	form.Set(auth.CSRFFormField, "wrong-token")
	req := httptest.NewRequest("POST", "/regions", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: "correct-token"})

	rec := httptest.NewRecorder()
	protected(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for mismatched CSRF tokens, got %d", rec.Code)
	}
	if reached {
		t.Error("handler was reached despite wrong CSRF token")
	}
}

// TestCSRF_ValidTokenPasses verifies that matching cookie+form token → handler executes.
func TestCSRF_ValidTokenPasses(t *testing.T) {
	csrfMgr := testCSRFManager()
	const token = "valid-csrf-token-abc123"
	reached := false
	protected := csrfProtect(csrfMgr, func(w http.ResponseWriter, req *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}, nil)

	rec := postWithCSRF(t, protected, "/regions", token, url.Values{
		"description": {"Test Region"},
	})

	if !reached {
		t.Errorf("handler was not reached with valid CSRF token (got %d)", rec.Code)
	}
	if rec.Code == http.StatusForbidden {
		t.Errorf("got 403 Forbidden despite valid CSRF token")
	}
}

// TestCSRF_NilManagerSkips verifies that nil csrfManager → POST proceeds without token.
func TestCSRF_NilManagerSkips(t *testing.T) {
	reached := false
	protected := csrfProtect(nil, func(w http.ResponseWriter, req *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}, nil)

	// POST with no CSRF fields at all
	req := httptest.NewRequest("POST", "/regions", strings.NewReader("descripcion=Test"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	protected(rec, req)

	if !reached {
		t.Errorf("handler was not reached with nil CSRF manager (got %d)", rec.Code)
	}
}

// TestCSRF_GETRequestSkipsValidation verifies GET requests are never CSRF-checked.
func TestCSRF_GETRequestSkipsValidation(t *testing.T) {
	csrfMgr := testCSRFManager()
	reached := false
	protected := csrfProtect(csrfMgr, func(w http.ResponseWriter, req *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}, nil)

	req := httptest.NewRequest("GET", "/regions", nil)
	// No CSRF token — GET should never need one
	rec := httptest.NewRecorder()
	protected(rec, req)

	if !reached {
		t.Errorf("GET handler was blocked by CSRF check (got %d)", rec.Code)
	}
}
