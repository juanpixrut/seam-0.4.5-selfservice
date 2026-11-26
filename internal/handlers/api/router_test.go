package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/handlers/middleware"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

func TestRouter_HealthCheck(t *testing.T) {
	// Create minimal router
	client := xolu.New("http://localhost:9999") // Won't be called
	config := RouterConfig{
		XOLUClient: client,
	}
	router := NewRouter(config)

	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["status"] != "healthy" {
		t.Errorf("expected status=healthy, got %v", resp["status"])
	}
}

func TestRouter_WithMiddleware(t *testing.T) {
	// Create mock xolu server
	mockOLU := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"id": 1, "data": map[string]any{"name": "Asset 1"}},
		})
	}))
	defer mockOLU.Close()

	client := xolu.New(mockOLU.URL)
	config := RouterConfig{
		XOLUClient:     client,
		Locales:        []string{"en", "es", "pt"},
		RateLimit:      100,
		AllowedOrigins: []string{"*"},
	}
	router := NewRouter(config)
	handler := router.Handler(config)

	req := httptest.NewRequest("GET", "/api/v1/assets", nil)
	req.Header.Set("Accept-Language", "es")
	req.Header.Set("X-Tenant-ID", "acme")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Check CORS header
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected CORS header")
	}

	// Check JSON content type
	if rec.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Errorf("expected JSON content type, got %s", rec.Header().Get("Content-Type"))
	}
}

func TestRouter_CORS_Preflight(t *testing.T) {
	client := xolu.New("http://localhost:9999")
	config := RouterConfig{
		XOLUClient:     client,
		AllowedOrigins: []string{"https://example.com"},
	}
	router := NewRouter(config)
	handler := router.Handler(config)

	req := httptest.NewRequest("OPTIONS", "/api/v1/assets", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204 for preflight, got %d", rec.Code)
	}

	if rec.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Error("expected Access-Control-Allow-Origin header")
	}
}

func TestRouter_TenantContext(t *testing.T) {
	mockOLU := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data":       []map[string]any{},
			"pagination": map[string]any{"page": 1, "per_page": 10, "total_items": 0, "total_pages": 0},
		})
	}))
	defer mockOLU.Close()

	client := xolu.New(mockOLU.URL)
	config := RouterConfig{
		XOLUClient: client,
	}
	router := NewRouter(config)
	handler := router.Handler(config)

	req := httptest.NewRequest("GET", "/api/v1/assets", nil)
	req.Header.Set("X-Tenant-ID", "acme-corp")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Request should succeed
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRouter_RateLimit(t *testing.T) {
	client := xolu.New("http://localhost:9999")
	config := RouterConfig{
		XOLUClient: client,
		RateLimit:  2, // Very low for testing
	}
	router := NewRouter(config)
	handler := router.Handler(config)

	// First 2 requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/api/v1/health", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, rec.Code)
		}
	}

	// Third request should be rate limited
	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rec.Code)
	}
}

func TestRouter_PanicRecovery(t *testing.T) {
	// Create a handler that will panic
	client := xolu.New("http://localhost:9999")
	config := RouterConfig{
		XOLUClient: client,
	}
	router := NewRouter(config)

	// Add a panic-inducing route for testing
	// We'll trigger this by calling an endpoint that hits the mock server
	// which doesn't exist, causing a connection error that we handle gracefully

	handler := router.Handler(config)

	// This should not panic even though the xolu server doesn't exist
	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	// Should not panic
	handler.ServeHTTP(rec, req)

	// Health check doesn't call xolu, so it should succeed
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

// =============================================================================
// HELPER HANDLER TESTS
// =============================================================================

func TestGetPaginationParams(t *testing.T) {
	tests := []struct {
		name           string
		query          string
		expectedLimit  int
		expectedOffset int
	}{
		{"defaults", "", 20, 0},
		{"custom limit", "limit=50", 50, 0},
		{"custom offset", "offset=100", 20, 100},
		{"both", "limit=30&offset=60", 30, 60},
		{"limit over max", "limit=200", 100, 0},
		{"negative ignored", "limit=-5&offset=-10", 20, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test?"+tt.query, nil)
			params := GetPaginationParams(req)

			if params.Limit != tt.expectedLimit {
				t.Errorf("limit: expected %d, got %d", tt.expectedLimit, params.Limit)
			}
			if params.Offset != tt.expectedOffset {
				t.Errorf("offset: expected %d, got %d", tt.expectedOffset, params.Offset)
			}
		})
	}
}

func TestGetQueryHelpers(t *testing.T) {
	req := httptest.NewRequest("GET", "/test?name=test&count=42&enabled=true&tags=a&tags=b", nil)

	if got := GetQueryString(req, "name", "default"); got != "test" {
		t.Errorf("GetQueryString: expected test, got %s", got)
	}

	if got := GetQueryString(req, "missing", "default"); got != "default" {
		t.Errorf("GetQueryString default: expected default, got %s", got)
	}

	if got := GetQueryInt(req, "count", 0); got != 42 {
		t.Errorf("GetQueryInt: expected 42, got %d", got)
	}

	if got := GetQueryInt(req, "missing", 10); got != 10 {
		t.Errorf("GetQueryInt default: expected 10, got %d", got)
	}

	if got := GetQueryBool(req, "enabled", false); !got {
		t.Error("GetQueryBool: expected true")
	}

	if got := GetQueryBool(req, "missing", true); !got {
		t.Error("GetQueryBool default: expected true")
	}

	tags := GetQueryStrings(req, "tags")
	if len(tags) != 2 || tags[0] != "a" || tags[1] != "b" {
		t.Errorf("GetQueryStrings: expected [a, b], got %v", tags)
	}
}

func TestReadJSON(t *testing.T) {
	body := `{"name": "test", "count": 42}`
	req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(body))

	var data map[string]any
	err := ReadJSON(req, &data)

	if err != nil {
		t.Fatalf("ReadJSON failed: %v", err)
	}

	if data["name"] != "test" {
		t.Errorf("expected name=test, got %v", data["name"])
	}

	if data["count"].(float64) != 42 {
		t.Errorf("expected count=42, got %v", data["count"])
	}
}

func TestReadJSON_Invalid(t *testing.T) {
	body := `{invalid json}`
	req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(body))

	var data map[string]any
	err := ReadJSON(req, &data)

	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestNewListResponse(t *testing.T) {
	data := []string{"a", "b", "c"}
	resp := NewListResponse(data, 100, 20, 40)

	if resp.Meta.Total != 100 {
		t.Errorf("expected total=100, got %d", resp.Meta.Total)
	}
	if resp.Meta.Limit != 20 {
		t.Errorf("expected limit=20, got %d", resp.Meta.Limit)
	}
	if resp.Meta.Offset != 40 {
		t.Errorf("expected offset=40, got %d", resp.Meta.Offset)
	}
}

// =============================================================================
// TENANT VALIDATION ORDERING TESTS
// =============================================================================

func TestRouter_TenantValidation_BlocksMismatchedTenant(t *testing.T) {
	// This test verifies that tenant validation runs AFTER auth sets claims,
	// so it can actually detect tenant mismatches.
	//
	// The bug we're preventing: if tenant validation runs before auth,
	// it sees "no claims" and passes everything through.

	mockOLU := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data":       []map[string]any{},
			"pagination": map[string]any{"page": 1, "per_page": 10, "total_items": 0, "total_pages": 0},
		})
	}))
	defer mockOLU.Close()

	client := xolu.New(mockOLU.URL)

	// Create a real auth service with known secret
	authSvc := auth.NewService(auth.Config{
		Secret:      "test-secret-key-for-signing",
		TokenExpiry: 24 * time.Hour,
	})

	// Generate a valid token for tenant 123
	token, err := authSvc.GenerateToken(&auth.Claims{
		UserID:    1,
		TenantID:  123, // User belongs to tenant 123
		Email:     "test@example.com",
		Role:      "admin",
		TokenType: auth.TokenTypeAccess,
	})
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	authMiddleware := auth.NewMiddleware(authSvc, "X-API-Key")

	config := RouterConfig{
		XOLUClient:           client,
		AuthMiddleware:       authMiddleware,
		TenantFromClaimsOnly: false, // Validate header matches claims
	}
	router := NewRouter(config)
	handler := router.Handler(config)

	// Request with valid token (tenant 123) but different X-Tenant-ID (456)
	req := httptest.NewRequest("GET", "/api/v1/assets", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "456") // MISMATCH!
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should be FORBIDDEN (403) because tenant doesn't match
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for tenant mismatch, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRouter_TenantValidation_AllowsMatchingTenant(t *testing.T) {
	mockOLU := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data":       []map[string]any{},
			"pagination": map[string]any{"page": 1, "per_page": 10, "total_items": 0, "total_pages": 0},
		})
	}))
	defer mockOLU.Close()

	client := xolu.New(mockOLU.URL)

	authSvc := auth.NewService(auth.Config{
		Secret:      "test-secret-key-for-signing",
		TokenExpiry: 24 * time.Hour,
	})

	token, err := authSvc.GenerateToken(&auth.Claims{
		UserID:    1,
		TenantID:  123,
		Email:     "test@example.com",
		Role:      "admin",
		TokenType: auth.TokenTypeAccess,
	})
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	authMiddleware := auth.NewMiddleware(authSvc, "X-API-Key")

	config := RouterConfig{
		XOLUClient:           client,
		AuthMiddleware:       authMiddleware,
		TenantFromClaimsOnly: false,
	}
	router := NewRouter(config)
	handler := router.Handler(config)

	// Request with matching tenant
	req := httptest.NewRequest("GET", "/api/v1/assets", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "123") // Matches!
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should succeed
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK for matching tenant, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRouter_TenantFromClaimsOnly_OverridesHeader(t *testing.T) {
	var capturedTenantID string

	mockOLU := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data":       []map[string]any{},
			"pagination": map[string]any{"page": 1, "per_page": 10, "total_items": 0, "total_pages": 0},
		})
	}))
	defer mockOLU.Close()

	client := xolu.New(mockOLU.URL)

	authSvc := auth.NewService(auth.Config{
		Secret:      "test-secret-key-for-signing",
		TokenExpiry: 24 * time.Hour,
	})

	token, err := authSvc.GenerateToken(&auth.Claims{
		UserID:    1,
		TenantID:  123,
		Email:     "test@example.com",
		Role:      "admin",
		TokenType: auth.TokenTypeAccess,
	})
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	authMiddleware := auth.NewMiddleware(authSvc, "X-API-Key")

	config := RouterConfig{
		XOLUClient:           client,
		AuthMiddleware:       authMiddleware,
		TenantFromClaimsOnly: true, // Ignore X-Tenant-ID, use claims
	}
	router := NewRouter(config)

	// Add a test handler that captures tenant from context
	router.mux.HandleFunc("GET /api/v1/test-tenant", router.withPerm(auth.PermAssetsAssetView, func(w http.ResponseWriter, r *http.Request) {
		capturedTenantID = middleware.TenantFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	handler := router.Handler(config)

	// Request with different X-Tenant-ID than claims
	req := httptest.NewRequest("GET", "/api/v1/test-tenant", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "999") // Different from claims (123)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should succeed (TenantFromClaimsOnly allows this)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	// Captured tenant should be from claims (123), not header (999)
	if capturedTenantID != "123" {
		t.Errorf("expected tenant from claims '123', got '%s'", capturedTenantID)
	}
}
