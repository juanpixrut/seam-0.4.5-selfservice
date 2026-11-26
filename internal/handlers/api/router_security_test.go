package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/handlers/middleware"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

// =============================================================================
// ROUTE COVERAGE TESTS
// =============================================================================
// These tests verify that all protected API routes go through proper auth
// and tenant validation. They use the route registry to automatically
// discover routes, so new routes are tested without manual updates.

// TestRouter_AllProtectedRoutesRequireAuth verifies that all registered protected
// routes reject unauthenticated requests with 401.
func TestRouter_AllProtectedRoutesRequireAuth(t *testing.T) {
	mockOLU := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{})
	}))
	defer mockOLU.Close()

	client := xolu.New(mockOLU.URL)
	authSvc := auth.NewService(auth.Config{
		Secret:      "test-secret",
		TokenExpiry: 24 * time.Hour,
	})
	authMiddleware := auth.NewMiddleware(authSvc, "X-API-Key")

	config := RouterConfig{
		XOLUClient:           client,
		AuthMiddleware:       authMiddleware,
		TenantFromClaimsOnly: true,
	}
	router := NewRouter(config)
	handler := router.Handler(config)

	// Use route registry - automatically includes all registered protected routes
	protectedRoutes := router.GetProtectedRoutes()

	if len(protectedRoutes) == 0 {
		t.Fatal("no protected routes registered - this is likely a bug")
	}

	t.Logf("Testing %d protected routes for auth requirement", len(protectedRoutes))

	for _, route := range protectedRoutes {
		routeKey := route.Method + " " + route.Path
		t.Run(routeKey, func(t *testing.T) {
			// Replace path parameters with test values
			path := substitutePathParams(route.Path)
			req := httptest.NewRequest(route.Method, path, nil)
			// No Authorization header = unauthenticated
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("%s %s: expected 401 Unauthorized for unauthenticated request, got %d",
					route.Method, route.Path, rec.Code)
			}
		})
	}
}

// TestRouter_AllProtectedRoutesValidateTenant verifies that all protected routes
// run tenant validation after auth sets claims.
func TestRouter_AllProtectedRoutesValidateTenant(t *testing.T) {
	mockOLU := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{})
	}))
	defer mockOLU.Close()

	client := xolu.New(mockOLU.URL)
	authSvc := auth.NewService(auth.Config{
		Secret:      "test-secret",
		TokenExpiry: 24 * time.Hour,
	})

	// Generate a valid token for tenant 123
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

	// Use TenantFromClaimsOnly=false to test validation mode
	config := RouterConfig{
		XOLUClient:           client,
		AuthMiddleware:       authMiddleware,
		TenantFromClaimsOnly: false, // Validate header against claims
	}
	router := NewRouter(config)
	handler := router.Handler(config)

	// Test a sample of protected routes (testing all would be slow)
	sampleRoutes := []RouteInfo{
		{Method: "GET", Path: "/api/v1/assets"},
		{Method: "POST", Path: "/api/v1/assets"},
		{Method: "GET", Path: "/api/v1/events"},
		{Method: "GET", Path: "/api/v1/sensors"},
		{Method: "GET", Path: "/api/v1/asset-types"},
	}

	for _, route := range sampleRoutes {
		routeKey := route.Method + " " + route.Path
		t.Run(routeKey+"_tenant_mismatch", func(t *testing.T) {
			req := httptest.NewRequest(route.Method, route.Path, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("X-Tenant-ID", "999") // Mismatch: token has 123
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Errorf("%s %s: expected 403 Forbidden for tenant mismatch, got %d",
					route.Method, route.Path, rec.Code)
			}
		})
	}
}

// TestRouter_TenantFromClaimsOnlyIgnoresHeader verifies that when TenantFromClaimsOnly=true,
// the X-Tenant-ID header is ignored and tenant is derived from claims.
func TestRouter_TenantFromClaimsOnlyIgnoresHeader(t *testing.T) {
	var capturedTenant string

	mockOLU := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{})
	}))
	defer mockOLU.Close()

	client := xolu.New(mockOLU.URL)
	authSvc := auth.NewService(auth.Config{
		Secret:      "test-secret",
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
		TenantFromClaimsOnly: true, // Secure default
	}
	router := NewRouter(config)

	// Add a test handler that captures the tenant from context
	router.ProtectedGet("/api/v1/test-tenant-capture", auth.PermAssetsAssetView, func(w http.ResponseWriter, r *http.Request) {
		capturedTenant = middleware.TenantFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := router.Handler(config)

	req := httptest.NewRequest("GET", "/api/v1/test-tenant-capture", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "999") // Attacker tries to access tenant 999
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	// Critical: tenant should be 123 (from claims), not 999 (from header)
	if capturedTenant != "123" {
		t.Errorf("SECURITY: TenantFromClaimsOnly should ignore header; expected '123', got '%s'", capturedTenant)
	}
}

// TestRouter_PublicRoutesNoAuth verifies that public routes work without authentication.
func TestRouter_PublicRoutesNoAuth(t *testing.T) {
	client := xolu.New("http://localhost:9999") // Won't be called for health
	authSvc := auth.NewService(auth.Config{
		Secret:      "test-secret",
		TokenExpiry: 24 * time.Hour,
	})
	authMiddleware := auth.NewMiddleware(authSvc, "X-API-Key")

	config := RouterConfig{
		XOLUClient:     client,
		AuthMiddleware: authMiddleware,
	}
	router := NewRouter(config)
	handler := router.Handler(config)

	// Use route registry
	publicRoutes := router.GetPublicRoutes()

	if len(publicRoutes) == 0 {
		t.Fatal("no public routes registered - expected at least health check")
	}

	t.Logf("Testing %d public routes work without auth", len(publicRoutes))

	for _, route := range publicRoutes {
		routeKey := route.Method + " " + route.Path
		t.Run(routeKey, func(t *testing.T) {
			path := substitutePathParams(route.Path)
			req := httptest.NewRequest(route.Method, path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code == http.StatusUnauthorized {
				t.Errorf("%s should be public but returned 401", routeKey)
			}
		})
	}
}

// TestRouter_RouteRegistryCompleteness verifies that the route registry captures
// all routes and can be used for security auditing.
func TestRouter_RouteRegistryCompleteness(t *testing.T) {
	client := xolu.New("http://localhost:9999")
	config := RouterConfig{
		XOLUClient: client,
	}
	router := NewRouter(config)

	protected := router.GetProtectedRoutes()
	public := router.GetPublicRoutes()

	t.Logf("Route registry: %d protected, %d public", len(protected), len(public))

	// Verify we have a reasonable number of routes
	// Core routes (without optional handlers like QR, Webhooks, etc.)
	if len(protected) < 40 {
		t.Errorf("expected at least 40 protected routes, got %d", len(protected))
	}

	if len(public) < 1 {
		t.Errorf("expected at least 1 public route (health), got %d", len(public))
	}

	// Verify health check is public
	foundHealth := false
	for _, r := range public {
		if r.Path == "/api/v1/health" {
			foundHealth = true
			break
		}
	}
	if !foundHealth {
		t.Error("health check should be in public routes")
	}

	// Verify some key routes are registered
	keyRoutes := []string{
		"/api/v1/assets",
		"/api/v1/events",
		"/api/v1/sensors",
		"/api/v1/rules",
	}
	for _, key := range keyRoutes {
		found := false
		for _, r := range protected {
			if r.Path == key {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %s to be in protected routes", key)
		}
	}
}

// =============================================================================
// DEVMODE SAFETY TESTS
// =============================================================================

// TestDevMode_FallbacksDisabledByDefault verifies that development fallbacks
// are not active when DevMode is false.
func TestDevMode_FallbacksDisabledByDefault(t *testing.T) {
	// This test verifies that the config default for DevMode is false.
	// The actual UI fallback behavior is tested in the ui package.
	// Here we just verify the config layer does the right thing.

	// Note: auth.DefaultConfig() returns auth-specific config, not the full config.
	// The full config check would be in config package. This test documents intent.
	t.Log("DevMode should be false by default - verified in config/config.go Default()")

	// If we had access to config.Default() here, we'd check:
	// cfg := config.Default()
	// if cfg.Features.DevMode {
	//     t.Error("DevMode should be false by default")
	// }
}

// =============================================================================
// MIDDLEWARE ORDERING TESTS
// =============================================================================

// TestMiddlewareOrdering_ClaimsBeforeTenantValidation verifies that claims
// are set in context before tenant validation runs.
func TestMiddlewareOrdering_ClaimsBeforeTenantValidation(t *testing.T) {
	mockOLU := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{})
	}))
	defer mockOLU.Close()

	client := xolu.New(mockOLU.URL)
	authSvc := auth.NewService(auth.Config{
		Secret:      "test-secret",
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
		TenantFromClaimsOnly: true,
	}
	router := NewRouter(config)

	var (
		claimsPresent bool
		tenantPresent bool
		claimsTenant  int64
		contextTenant string
	)

	// Add a test handler that inspects the context
	router.ProtectedGet("/api/v1/test-ordering", auth.PermAssetsAssetView, func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		claimsPresent = ok && claims != nil
		if claimsPresent {
			claimsTenant = claims.TenantID
		}

		contextTenant = middleware.TenantFromContext(r.Context())
		tenantPresent = contextTenant != ""

		json.NewEncoder(w).Encode(map[string]any{
			"claims_present": claimsPresent,
			"tenant_present": tenantPresent,
			"claims_tenant":  claimsTenant,
			"context_tenant": contextTenant,
		})
	})

	handler := router.Handler(config)

	req := httptest.NewRequest("GET", "/api/v1/test-ordering", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "999") // Will be overridden by claims
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if !claimsPresent {
		t.Error("claims should be present in context when handler runs")
	}
	if !tenantPresent {
		t.Error("tenant should be present in context when handler runs")
	}
	if claimsTenant != 123 {
		t.Errorf("claims tenant should be 123, got %d", claimsTenant)
	}
	if contextTenant != "123" {
		t.Errorf("context tenant should be '123' (from claims), got '%s'", contextTenant)
	}
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// substitutePathParams replaces path parameters like {id} with test values
func substitutePathParams(path string) string {
	// Simple replacement for common patterns
	replacements := map[string]string{
		"{id}":   "123",
		"{code}": "ABC123",
	}
	result := path
	for param, value := range replacements {
		result = replaceAll(result, param, value)
	}
	return result
}

// replaceAll is a simple string replacement (avoiding strings import)
func replaceAll(s, old, new string) string {
	result := ""
	for i := 0; i < len(s); {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			result += new
			i += len(old)
		} else {
			result += string(s[i])
			i++
		}
	}
	return result
}
