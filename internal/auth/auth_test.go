package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClaims_Valid(t *testing.T) {
	tests := []struct {
		name    string
		claims  Claims
		wantErr bool
	}{
		{
			name: "valid claims",
			claims: Claims{
				UserID:    1,
				TenantID:  1,
				Email:     "test@example.com",
				Role:      "user",
				ExpiresAt: time.Now().Add(time.Hour),
			},
			wantErr: false,
		},
		{
			name: "missing user_id",
			claims: Claims{
				TenantID:  1,
				ExpiresAt: time.Now().Add(time.Hour),
			},
			wantErr: true,
		},
		{
			name: "missing tenant_id",
			claims: Claims{
				UserID:    1,
				ExpiresAt: time.Now().Add(time.Hour),
			},
			wantErr: true,
		},
		{
			name: "expired",
			claims: Claims{
				UserID:    1,
				TenantID:  1,
				ExpiresAt: time.Now().Add(-time.Hour),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.claims.Valid()
			if (err != nil) != tt.wantErr {
				t.Errorf("Valid() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClaims_HasRole(t *testing.T) {
	tests := []struct {
		name       string
		claimRole  string
		checkRole  string
		wantResult bool
	}{
		{"exact match", "editor", "editor", true},
		{"admin has all", "admin", "editor", true},
		{"user not admin", "user", "admin", false},
		{"mismatch", "user", "editor", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Claims{Role: tt.claimRole}
			if got := c.HasRole(tt.checkRole); got != tt.wantResult {
				t.Errorf("HasRole(%s) = %v, want %v", tt.checkRole, got, tt.wantResult)
			}
		})
	}
}

func TestAPIKey_HasScope(t *testing.T) {
	tests := []struct {
		name       string
		scopes     []string
		checkScope string
		wantResult bool
	}{
		{"exact match", []string{"read", "write"}, "read", true},
		{"wildcard", []string{"*"}, "anything", true},
		{"not found", []string{"read"}, "write", false},
		{"empty scopes", []string{}, "read", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := &APIKey{Scopes: tt.scopes}
			if got := k.HasScope(tt.checkScope); got != tt.wantResult {
				t.Errorf("HasScope(%s) = %v, want %v", tt.checkScope, got, tt.wantResult)
			}
		})
	}
}

func TestService_GenerateAndValidateToken(t *testing.T) {
	svc := NewService(Config{
		Secret:      "test-secret-key",
		TokenExpiry: time.Hour,
	})

	claims := &Claims{
		UserID:    123,
		TenantID:  456,
		Email:     "user@example.com",
		Role:      "user",
		TokenType: TokenTypeAccess,
	}

	// Generate token
	token, err := svc.GenerateToken(claims)
	if err != nil {
		t.Fatalf("GenerateToken() error: %v", err)
	}

	if token == "" {
		t.Fatal("GenerateToken() returned empty token")
	}

	// Validate token
	validatedClaims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken() error: %v", err)
	}

	if validatedClaims.UserID != claims.UserID {
		t.Errorf("UserID = %d, want %d", validatedClaims.UserID, claims.UserID)
	}

	if validatedClaims.TenantID != claims.TenantID {
		t.Errorf("TenantID = %d, want %d", validatedClaims.TenantID, claims.TenantID)
	}
}

func TestService_ValidateToken_Invalid(t *testing.T) {
	svc := NewService(Config{Secret: "test-secret"})

	tests := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"no dot", "invalidtoken"},
		{"invalid base64", "!!!.signature"},
		{"wrong signature", "dGVzdA.wrongsig"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.ValidateToken(tt.token)
			if err == nil {
				t.Error("ValidateToken() should return error for invalid token")
			}
		})
	}
}

func TestService_GenerateAPIKey(t *testing.T) {
	svc := NewService(Config{Secret: "test-secret"})

	key1, prefix1, hash1, err := svc.GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey() error: %v", err)
	}

	if len(key1) < 32 {
		t.Errorf("key too short: %d", len(key1))
	}

	if len(prefix1) != 8 {
		t.Errorf("prefix should be 8 chars, got %d", len(prefix1))
	}

	if !svc.ValidateAPIKey(key1, hash1) {
		t.Error("ValidateAPIKey() should return true for valid key")
	}

	// Generate another key - should be different
	key2, _, _, _ := svc.GenerateAPIKey()
	if key1 == key2 {
		t.Error("GenerateAPIKey() should generate unique keys")
	}
}

func TestContext_Claims(t *testing.T) {
	claims := &Claims{
		UserID:   1,
		TenantID: 2,
		Role:     "admin",
	}

	ctx := ContextWithClaims(context.Background(), claims)

	retrieved, ok := ClaimsFromContext(ctx)
	if !ok {
		t.Fatal("ClaimsFromContext() returned false")
	}

	if retrieved.UserID != claims.UserID {
		t.Errorf("UserID = %d, want %d", retrieved.UserID, claims.UserID)
	}

	// Test with empty context
	_, ok = ClaimsFromContext(context.Background())
	if ok {
		t.Error("ClaimsFromContext() should return false for empty context")
	}
}

func TestContext_Tenant(t *testing.T) {
	tenant := &Tenant{
		ID:   1,
		Name: "Test Tenant",
		Slug: "test",
	}

	ctx := ContextWithTenant(context.Background(), tenant)

	retrieved, ok := TenantFromContext(ctx)
	if !ok {
		t.Fatal("TenantFromContext() returned false")
	}

	if retrieved.ID != tenant.ID {
		t.Errorf("ID = %d, want %d", retrieved.ID, tenant.ID)
	}
}

func TestTenantIDFromContext(t *testing.T) {
	// From claims
	claims := &Claims{TenantID: 123}
	ctx := ContextWithClaims(context.Background(), claims)

	id, ok := TenantIDFromContext(ctx)
	if !ok || id != 123 {
		t.Errorf("TenantIDFromContext() = %d, %v; want 123, true", id, ok)
	}

	// From tenant
	tenant := &Tenant{ID: 456}
	ctx = ContextWithTenant(context.Background(), tenant)

	id, ok = TenantIDFromContext(ctx)
	if !ok || id != 456 {
		t.Errorf("TenantIDFromContext() = %d, %v; want 456, true", id, ok)
	}

	// Empty context
	_, ok = TenantIDFromContext(context.Background())
	if ok {
		t.Error("TenantIDFromContext() should return false for empty context")
	}
}

func TestMiddleware_Required(t *testing.T) {
	svc := NewService(Config{Secret: "test-secret", TokenExpiry: time.Hour})
	mw := NewMiddleware(svc, "X-API-Key")

	// Create valid token
	claims := &Claims{
		UserID:    1,
		TenantID:  1,
		Email:     "test@example.com",
		Role:      "user",
		TokenType: TokenTypeAccess,
	}
	token, _ := svc.GenerateToken(claims)

	handler := mw.Required(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, ok := ClaimsFromContext(r.Context())
		if !ok {
			t.Error("claims not in context")
			return
		}
		if c.UserID != 1 {
			t.Errorf("UserID = %d, want 1", c.UserID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	// With valid token
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Without token
	req = httptest.NewRequest("GET", "/", nil)
	rec = httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMiddleware_RequireRole(t *testing.T) {
	svc := NewService(Config{Secret: "test-secret"})
	mw := NewMiddleware(svc, "")

	handler := mw.RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Admin user
	claims := &Claims{UserID: 1, TenantID: 1, Role: "admin"}
	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(ContextWithClaims(req.Context(), claims))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("admin: status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Regular user
	claims = &Claims{UserID: 1, TenantID: 1, Role: "user"}
	req = httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(ContextWithClaims(req.Context(), claims))
	rec = httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("user: status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestTenantExtractor_Extract(t *testing.T) {
	extractor := NewTenantExtractor().WithHeader("X-Tenant-ID").WithQuery("tenant")

	tests := []struct {
		name   string
		setup  func(*http.Request) *http.Request
		wantID string
	}{
		{
			name: "from header",
			setup: func(r *http.Request) *http.Request {
				r.Header.Set("X-Tenant-ID", "tenant-123")
				return r
			},
			wantID: "tenant-123",
		},
		{
			name: "from query",
			setup: func(r *http.Request) *http.Request {
				r.URL.RawQuery = "tenant=tenant-456"
				return r
			},
			wantID: "tenant-456",
		},
		{
			name: "from claims (priority)",
			setup: func(r *http.Request) *http.Request {
				r.Header.Set("X-Tenant-ID", "header-tenant")
				claims := &Claims{TenantID: 789}
				return r.WithContext(ContextWithClaims(r.Context(), claims))
			},
			wantID: "789",
		},
		{
			name: "from subdomain",
			setup: func(r *http.Request) *http.Request {
				r.Host = "acme.seam.example.com"
				return r
			},
			wantID: "acme",
		},
		{
			name:   "empty",
			setup:  func(r *http.Request) *http.Request { return r },
			wantID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req = tt.setup(req)

			got := extractor.Extract(req)
			if got != tt.wantID {
				t.Errorf("Extract() = %q, want %q", got, tt.wantID)
			}
		})
	}
}

func TestHasPermission(t *testing.T) {
	tests := []struct {
		name string
		role string
		perm Permission
		want bool
	}{
		{"admin can create assets", RoleAdmin, PermAssetsAssetCreate, true},
		{"admin can delete users", RoleAdmin, PermAdministrationUserDelete, true},

		{"manager can create assets", RoleManager, PermAssetsAssetCreate, true},
		{"manager can read users", RoleManager, PermAdministrationUserView, true},
		{"manager cannot delete users", RoleManager, PermAdministrationUserDelete, false},
		{"manager can export data", RoleManager, PermAssetsReportExport, true},

		{"operator can create assets", RoleOperator, PermAssetsAssetCreate, true},
		{"operator can read assets", RoleOperator, PermAssetsAssetView, true},
		{"operator cannot delete assets", RoleOperator, PermAssetsAssetDelete, false},
		{"operator can read rules", RoleOperator, PermAssetsRuleView, true},
		{"operator cannot create rules", RoleOperator, PermAssetsRuleCreate, false},

		{"viewer can read assets", RoleViewer, PermAssetsAssetView, true},
		{"viewer cannot create assets", RoleViewer, PermAssetsAssetCreate, false},
		{"viewer cannot update assets", RoleViewer, PermAssetsAssetEdit, false},

		{"unknown role has no permissions", "unknown", PermAssetsAssetView, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasPermission(tt.role, tt.perm)
			if got != tt.want {
				t.Errorf("HasPermission(%q, %q) = %v, want %v", tt.role, tt.perm, got, tt.want)
			}
		})
	}
}

// TestPermissionMatches covers MODULE_SPEC.md §2.3's wildcard matching
// directly -- new behavior this migration introduces, not present under
// the old flat exact-match-only model.
func TestPermissionMatches(t *testing.T) {
	tests := []struct {
		name    string
		granted string
		request string
		want    bool
	}{
		{"exact match", "assets.asset.view", "assets.asset.view", true},
		{"exact mismatch, different action", "assets.asset.view", "assets.asset.edit", false},
		{"view-level wildcard matches any action", "assets.asset.*", "assets.asset.edit", true},
		{"view-level wildcard does not cross views", "assets.asset.*", "assets.rule.edit", false},
		{"module-level wildcard matches any view and action", "assets.*", "assets.sensor.delete", true},
		{"module-level wildcard does not cross modules", "assets.*", "system.settings.edit", false},
		{"no partial-segment match", "assets.asset", "assets.asset.view", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := permissionMatches(tt.granted, strings.Split(tt.request, "."))
			if got != tt.want {
				t.Errorf("permissionMatches(%q, %q) = %v, want %v", tt.granted, tt.request, got, tt.want)
			}
		})
	}
}

// TestHasModuleAccess covers MODULE_SPEC.md §2.4's derivation rule --
// module access is derived from holding at least one permission scoped to
// that module, not a separately stored grant.
func TestHasModuleAccess(t *testing.T) {
	tests := []struct {
		name   string
		role   string
		module string
		want   bool
	}{
		{"admin has assets access", RoleAdmin, "assets", true},
		{"admin has settings access", RoleAdmin, "settings", true},
		{"admin has administration access", RoleAdmin, "administration", true},
		{"admin has documents access", RoleAdmin, "documents", true},
		{"viewer has assets access (holds assets.asset.view)", RoleViewer, "assets", true},
		{"viewer has documents access (holds the placeholder permission)", RoleViewer, "documents", true},
		{"viewer has settings access (holds settings.view)", RoleViewer, "settings", true},
		{"viewer has no administration access (no administration.user.* grant)", RoleViewer, "administration", false},
		{"unknown role has no module access", "unknown", "assets", false},
		{"unknown module has no grantees", RoleAdmin, "cmms", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasModuleAccess(tt.role, tt.module)
			if got != tt.want {
				t.Errorf("HasModuleAccess(%q, %q) = %v, want %v", tt.role, tt.module, got, tt.want)
			}
		})
	}
}

func TestUser_HasPermission(t *testing.T) {
	admin := &User{ID: 1, Role: RoleAdmin}
	viewer := &User{ID: 2, Role: RoleViewer}

	if !admin.HasPermission(PermAdministrationUserDelete) {
		t.Error("admin should have PermAdministrationUserDelete")
	}

	if viewer.HasPermission(PermAdministrationUserDelete) {
		t.Error("viewer should not have PermAdministrationUserDelete")
	}
}

func TestDefaultUserPreferences(t *testing.T) {
	prefs := DefaultUserPreferences()

	if prefs.Locale != "en" {
		t.Errorf("expected default locale 'en', got %q", prefs.Locale)
	}
	if prefs.Timezone != "UTC" {
		t.Errorf("expected default timezone 'UTC', got %q", prefs.Timezone)
	}
	if prefs.DarkMode != "system" {
		t.Errorf("expected default dark_mode 'system', got %q", prefs.DarkMode)
	}
	if !prefs.EmailAlerts {
		t.Error("expected default email_alerts to be true")
	}
}
