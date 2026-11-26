package auth

import (
	"errors"
	"net/http"
	"strings"
)

// Middleware provides HTTP middleware for authentication.
type Middleware struct {
	service      *Service
	apiKeyHeader string
	optional     bool
}

// NewMiddleware creates a new auth middleware.
func NewMiddleware(service *Service, apiKeyHeader string) *Middleware {
	if apiKeyHeader == "" {
		apiKeyHeader = "X-API-Key"
	}
	return &Middleware{
		service:      service,
		apiKeyHeader: apiKeyHeader,
	}
}

// Required returns middleware that requires authentication.
func (m *Middleware) Required(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := m.authenticate(r)
		if err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		ctx := ContextWithClaims(r.Context(), claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Optional returns middleware that authenticates if credentials are present.
func (m *Middleware) Optional(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := m.authenticate(r)
		if err == nil && claims != nil {
			ctx := ContextWithClaims(r.Context(), claims)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

// RequireRole returns middleware that requires a specific role.
func (m *Middleware) RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			if !claims.HasRole(role) {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequirePermission returns middleware that requires a specific permission.
func (m *Middleware) RequirePermission(perm Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			if !HasPermission(claims.Role, perm) {
				http.Error(w, `{"error":"forbidden","required_permission":"`+string(perm)+`"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAnyPermission returns middleware that requires any of the specified permissions.
func (m *Middleware) RequireAnyPermission(perms ...Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			for _, perm := range perms {
				if HasPermission(claims.Role, perm) {
					next.ServeHTTP(w, r)
					return
				}
			}

			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		})
	}
}

// RequireAllPermissions returns middleware that requires all specified permissions.
func (m *Middleware) RequireAllPermissions(perms ...Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			for _, perm := range perms {
				if !HasPermission(claims.Role, perm) {
					http.Error(w, `{"error":"forbidden","missing_permission":"`+string(perm)+`"}`, http.StatusForbidden)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireActiveAccount returns middleware that checks the user account is active.
// This requires the User to be loaded into context (typically by a prior middleware).
func (m *Middleware) RequireActiveAccount(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFromContext(r.Context())
		if !ok {
			// No user in context - fall back to claims check
			claims, claimsOk := ClaimsFromContext(r.Context())
			if !claimsOk {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			// If we only have claims, we can't check active status
			// The caller should ensure user is loaded first
			_ = claims
			next.ServeHTTP(w, r)
			return
		}

		if !user.Active {
			http.Error(w, `{"error":"account_disabled","message":"Your account has been disabled"}`, http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequireTenant returns middleware that ensures tenant context.
// If the user is authenticated, it validates that the requested tenant
// matches the user's tenant from their claims (prevents horizontal escalation).
func (m *Middleware) RequireTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := TenantIDFromContext(r.Context())
		if !ok || tenantID == 0 {
			http.Error(w, `{"error":"tenant context required"}`, http.StatusBadRequest)
			return
		}

		// If user is authenticated, validate tenant matches their claims
		if claims, claimsOk := ClaimsFromContext(r.Context()); claimsOk {
			if claims.TenantID != tenantID {
				http.Error(w, `{"error":"tenant mismatch","message":"You do not have access to this tenant"}`, http.StatusForbidden)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// authenticate extracts and validates credentials from the request.
func (m *Middleware) authenticate(r *http.Request) (*Claims, error) {
	// Try Bearer token first
	if auth := r.Header.Get("Authorization"); auth != "" {
		if strings.HasPrefix(auth, "Bearer ") {
			token := strings.TrimPrefix(auth, "Bearer ")
			return m.service.ValidateToken(token)
		}
	}

	// API key authentication is not yet implemented.
	// When implemented, this will look up the key in storage and validate it.
	// For now, we explicitly reject API key attempts with a clear error.
	if apiKey := r.Header.Get(m.apiKeyHeader); apiKey != "" {
		// TODO: Implement API key storage and validation
		// 1. Look up API key by prefix in storage
		// 2. Validate full key against stored hash
		// 3. Check expiry and scopes
		// 4. Return claims derived from API key
		return nil, errors.New("API key authentication is not yet implemented")
	}

	return nil, ErrUnauthorised
}

// TenantExtractor extracts tenant from various sources.
type TenantExtractor struct {
	headerName   string
	queryParam   string
	pathParam    string
	subdomainPos int // Position of tenant in subdomain (0 = first)
}

// NewTenantExtractor creates a new tenant extractor.
func NewTenantExtractor() *TenantExtractor {
	return &TenantExtractor{
		headerName:   "X-Tenant-ID",
		queryParam:   "tenant_id",
		subdomainPos: 0,
	}
}

// WithHeader sets the header name for tenant extraction.
func (e *TenantExtractor) WithHeader(name string) *TenantExtractor {
	e.headerName = name
	return e
}

// WithQuery sets the query parameter for tenant extraction.
func (e *TenantExtractor) WithQuery(param string) *TenantExtractor {
	e.queryParam = param
	return e
}

// Extract extracts tenant identifier from the request.
// Priority: JWT claims > Header > Query param > Subdomain
func (e *TenantExtractor) Extract(r *http.Request) string {
	// From JWT claims (highest priority)
	if claims, ok := ClaimsFromContext(r.Context()); ok {
		if claims.TenantID > 0 {
			return formatInt64(claims.TenantID)
		}
	}

	// From header
	if tenant := r.Header.Get(e.headerName); tenant != "" {
		return tenant
	}

	// From query param
	if tenant := r.URL.Query().Get(e.queryParam); tenant != "" {
		return tenant
	}

	// From subdomain
	if tenant := e.extractFromSubdomain(r.Host); tenant != "" {
		return tenant
	}

	return ""
}

// extractFromSubdomain extracts tenant from subdomain.
func (e *TenantExtractor) extractFromSubdomain(host string) string {
	// Remove port if present
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	parts := strings.Split(host, ".")
	// Need at least 3 parts for subdomain (e.g., tenant.example.com)
	if len(parts) < 3 {
		return ""
	}

	if e.subdomainPos < len(parts)-2 {
		return parts[e.subdomainPos]
	}

	return ""
}

func formatInt64(n int64) string {
	if n == 0 {
		return "0"
	}

	var result []byte
	negative := n < 0
	if negative {
		n = -n
	}

	for n > 0 {
		result = append([]byte{byte('0' + n%10)}, result...)
		n /= 10
	}

	if negative {
		result = append([]byte{'-'}, result...)
	}

	return string(result)
}
