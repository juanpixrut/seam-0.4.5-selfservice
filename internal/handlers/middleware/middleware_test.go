package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ha1tch/seam-ui/internal/auth"
)

func TestTenantFromContext(t *testing.T) {
	ctx := context.Background()
	if got := TenantFromContext(ctx); got != "" {
		t.Errorf("expected empty, got %q", got)
	}

	ctx = context.WithValue(ctx, TenantIDKey, "tenant-123")
	if got := TenantFromContext(ctx); got != "tenant-123" {
		t.Errorf("expected tenant-123, got %q", got)
	}
}

func TestLocaleFromContext(t *testing.T) {
	ctx := context.Background()
	if got := LocaleFromContext(ctx); got != "en" {
		t.Errorf("expected en (default), got %q", got)
	}

	ctx = context.WithValue(ctx, LocaleKey, "es")
	if got := LocaleFromContext(ctx); got != "es" {
		t.Errorf("expected es, got %q", got)
	}
}

func TestTenantContextHeader(t *testing.T) {
	handler := TenantContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenant := TenantFromContext(r.Context())
		w.Write([]byte(tenant))
	}))

	req := httptest.NewRequest("GET", "/api/assets", nil)
	req.Header.Set("X-Tenant-ID", "acme-corp")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Body.String() != "acme-corp" {
		t.Errorf("expected acme-corp, got %s", rec.Body.String())
	}
}

func TestTenantContextPath(t *testing.T) {
	handler := TenantContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenant := TenantFromContext(r.Context())
		w.Write([]byte(tenant))
	}))

	req := httptest.NewRequest("GET", "/tenant/acme-corp/assets", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Body.String() != "acme-corp" {
		t.Errorf("expected acme-corp, got %s", rec.Body.String())
	}
}

func TestTenantContextSubdomain(t *testing.T) {
	handler := TenantContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenant := TenantFromContext(r.Context())
		w.Write([]byte(tenant))
	}))

	req := httptest.NewRequest("GET", "/api/assets", nil)
	req.Host = "acme.seam.example.com"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Body.String() != "acme" {
		t.Errorf("expected acme, got %s", rec.Body.String())
	}
}

func TestRequireTenant(t *testing.T) {
	handler := RequireTenant(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Without tenant
	req := httptest.NewRequest("GET", "/api/assets", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}

	// With tenant
	ctx := context.WithValue(context.Background(), TenantIDKey, "tenant-1")
	req = httptest.NewRequest("GET", "/api/assets", nil).WithContext(ctx)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestLocaleDetector(t *testing.T) {
	supported := []string{"en", "es", "pt"}

	tests := []struct {
		name         string
		acceptLang   string
		queryLang    string
		expectedLang string
	}{
		{"default to English", "", "", "en"},
		{"exact match", "es", "", "es"},
		{"with region", "es-AR", "", "es"},
		{"quality preference", "en-US,es;q=0.9", "", "en"},
		{"query override", "en", "pt", "pt"},
		{"unsupported language", "fr", "", "en"},
		{"complex header", "fr-FR,fr;q=0.9,pt-BR;q=0.8", "", "pt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := LocaleDetector(supported)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				locale := LocaleFromContext(r.Context())
				w.Write([]byte(locale))
			}))

			url := "/api/test"
			if tt.queryLang != "" {
				url += "?lang=" + tt.queryLang
			}

			req := httptest.NewRequest("GET", url, nil)
			if tt.acceptLang != "" {
				req.Header.Set("Accept-Language", tt.acceptLang)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Body.String() != tt.expectedLang {
				t.Errorf("expected %s, got %s", tt.expectedLang, rec.Body.String())
			}
		})
	}
}

func TestRequestLogger(t *testing.T) {
	logger := NewRequestLogger(nil)

	handler := logger.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request ID was set
		reqID := RequestIDFromContext(r.Context())
		if reqID == "" {
			t.Error("request ID should be set")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestRecoverer(t *testing.T) {
	handler := Recoverer(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	rec := httptest.NewRecorder()

	// Should not panic
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}

	var resp ErrorResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Code != "internal_error" {
		t.Errorf("expected internal_error code, got %s", resp.Code)
	}
}

func TestRateLimiter(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)

	handler := rl.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First 3 requests should succeed
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/api/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, rec.Code)
		}
	}

	// 4th request should be rate limited
	req := httptest.NewRequest("GET", "/api/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rec.Code)
	}

	// Check Retry-After header
	if rec.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header")
	}
}

func TestRateLimiterPerTenant(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)

	handler := rl.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Tenant A uses 2 requests
	for i := 0; i < 2; i++ {
		ctx := context.WithValue(context.Background(), TenantIDKey, "tenant-a")
		req := httptest.NewRequest("GET", "/api/test", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("tenant-a request %d: expected 200, got %d", i+1, rec.Code)
		}
	}

	// Tenant B should still be able to make requests
	ctx := context.WithValue(context.Background(), TenantIDKey, "tenant-b")
	req := httptest.NewRequest("GET", "/api/test", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("tenant-b should not be rate limited, got %d", rec.Code)
	}
}

func TestCORS(t *testing.T) {
	handler := CORS([]string{"https://example.com"}, true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Allowed origin
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Error("expected Access-Control-Allow-Origin header")
	}
	if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Error("expected Access-Control-Allow-Credentials header")
	}

	// Preflight request
	req = httptest.NewRequest("OPTIONS", "/api/test", nil)
	req.Header.Set("Origin", "https://example.com")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204 for preflight, got %d", rec.Code)
	}
}

func TestCORSAllowAll(t *testing.T) {
	handler := CORS([]string{"*"}, false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Origin", "https://any-domain.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected * for Access-Control-Allow-Origin")
	}
}

func TestContentType(t *testing.T) {
	handler := ContentType("application/xml")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Type") != "application/xml" {
		t.Errorf("expected application/xml, got %s", rec.Header().Get("Content-Type"))
	}
}

func TestJSON(t *testing.T) {
	handler := JSON(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Errorf("expected JSON content type, got %s", rec.Header().Get("Content-Type"))
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, http.StatusBadRequest, "invalid_input", "Invalid input data")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}

	var resp ErrorResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Error != "Invalid input data" {
		t.Errorf("unexpected error message: %s", resp.Error)
	}
	if resp.Code != "invalid_input" {
		t.Errorf("unexpected code: %s", resp.Code)
	}
}

func TestWriteErrorWithDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	details := map[string]string{"field": "email", "reason": "invalid format"}
	WriteErrorWithDetails(rec, http.StatusUnprocessableEntity, "validation_error", "Validation failed", details)

	var resp ErrorResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Details == nil {
		t.Error("expected details in response")
	}
}

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	data := map[string]any{"id": 1, "name": "test"}
	WriteJSON(rec, http.StatusOK, data)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["name"] != "test" {
		t.Errorf("unexpected data: %v", resp)
	}
}

func TestValidateTenantAccess_NoTenant(t *testing.T) {
	validator := NewValidateTenantAccess()
	handler := validator.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/assets", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestValidateTenantAccess_NoAuth(t *testing.T) {
	validator := NewValidateTenantAccess()
	handler := validator.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Set tenant in context but no auth
	req := httptest.NewRequest("GET", "/api/assets", nil)
	ctx := context.WithValue(req.Context(), TenantIDKey, "123")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should allow - auth middleware will catch unauthenticated requests
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestValidateTenantAccess_MatchingTenant(t *testing.T) {
	validator := NewValidateTenantAccess()
	handler := validator.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/assets", nil)
	// Set tenant in context
	ctx := context.WithValue(req.Context(), TenantIDKey, "123")
	// Set matching claims using auth package
	ctx = auth.ContextWithClaims(ctx, &auth.Claims{
		UserID:    1,
		TenantID:  123,
		Email:     "test@example.com",
		Role:      "admin",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for matching tenant, got %d", rec.Code)
	}
}

func TestValidateTenantAccess_MismatchedTenant(t *testing.T) {
	// Test header validation mode (TenantFromClaimsOnly=false)
	validator := NewValidateTenantAccess().WithTenantFromClaimsOnly(false)
	handler := validator.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/assets", nil)
	// Set tenant in context (different from claims)
	ctx := context.WithValue(req.Context(), TenantIDKey, "456")
	// Set different tenant in claims
	ctx = auth.ContextWithClaims(ctx, &auth.Claims{
		UserID:    1,
		TenantID:  123, // Different from requested 456
		Email:     "test@example.com",
		Role:      "admin",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for mismatched tenant, got %d", rec.Code)
	}
}

func TestValidateTenantAccess_TenantFromClaimsOnly(t *testing.T) {
	// Default is now TenantFromClaimsOnly=true, so NewValidateTenantAccess() should work
	validator := NewValidateTenantAccess()

	var capturedTenant string
	handler := validator.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTenant = TenantFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/assets", nil)
	// Set tenant in context to 456 (simulating X-Tenant-ID header)
	ctx := context.WithValue(req.Context(), TenantIDKey, "456")
	// Set claims with tenant 123
	ctx = auth.ContextWithClaims(ctx, &auth.Claims{
		UserID:    1,
		TenantID:  123,
		Email:     "test@example.com",
		Role:      "admin",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should succeed AND override tenant to 123 (from claims)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if capturedTenant != "123" {
		t.Errorf("expected tenant to be overridden to '123', got '%s'", capturedTenant)
	}
}

func TestValidateTenantAccess_InvalidTenantFormat(t *testing.T) {
	// Test header validation mode (TenantFromClaimsOnly=false)
	validator := NewValidateTenantAccess().WithTenantFromClaimsOnly(false)
	handler := validator.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/assets", nil)
	// Set invalid tenant format (contains non-numeric chars)
	ctx := context.WithValue(req.Context(), TenantIDKey, "123abc")
	// Set claims
	ctx = auth.ContextWithClaims(ctx, &auth.Claims{
		UserID:    1,
		TenantID:  123,
		Email:     "test@example.com",
		Role:      "admin",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid tenant format, got %d", rec.Code)
	}
}

func TestValidateTenantAccess_DefaultIsSecure(t *testing.T) {
	// Verify that the default mode is TenantFromClaimsOnly=true
	// This test ensures we don't accidentally regress to insecure default
	validator := NewValidateTenantAccess()

	var capturedTenant string
	handler := validator.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTenant = TenantFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/assets", nil)
	// Set malicious tenant in context (attacker sets X-Tenant-ID: 999)
	ctx := context.WithValue(req.Context(), TenantIDKey, "999")
	// User's claims say they belong to tenant 123
	ctx = auth.ContextWithClaims(ctx, &auth.Claims{
		UserID:    1,
		TenantID:  123,
		Email:     "test@example.com",
		Role:      "admin",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// With secure default, request should succeed but tenant should be
	// overridden to 123 (from claims), NOT 999 (from header)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if capturedTenant != "123" {
		t.Errorf("SECURITY: default mode should ignore X-Tenant-ID header; expected '123', got '%s'", capturedTenant)
	}
}
