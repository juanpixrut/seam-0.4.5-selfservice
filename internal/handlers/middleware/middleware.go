// Package middleware provides HTTP middleware for Seam AMS.
//
// Included middleware:
//   - TenantContext: Extracts and validates tenant from request
//   - RequestLogger: Structured request/response logging
//   - Recoverer: Panic recovery with error response
//   - LocaleDetector: Detects user locale from Accept-Language header
//   - RateLimiter: Per-tenant rate limiting
package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/ha1tch/seam-ui/internal/auth"
)

// Context keys for request-scoped values.
type contextKey string

const (
	TenantIDKey  contextKey = "tenant_id"
	LocaleKey    contextKey = "locale"
	RequestIDKey contextKey = "request_id"
	UserIDKey    contextKey = "user_id"
)

// TenantFromContext extracts tenant ID from context.
func TenantFromContext(ctx context.Context) string {
	if v := ctx.Value(TenantIDKey); v != nil {
		return v.(string)
	}
	return ""
}

// LocaleFromContext extracts locale from context.
func LocaleFromContext(ctx context.Context) string {
	if v := ctx.Value(LocaleKey); v != nil {
		return v.(string)
	}
	return "en"
}

// RequestIDFromContext extracts request ID from context.
func RequestIDFromContext(ctx context.Context) string {
	if v := ctx.Value(RequestIDKey); v != nil {
		return v.(string)
	}
	return ""
}

// UserIDFromContext extracts user ID from context.
func UserIDFromContext(ctx context.Context) string {
	if v := ctx.Value(UserIDKey); v != nil {
		return v.(string)
	}
	return ""
}

// TenantContext extracts tenant ID from the request.
// It looks for tenant in: X-Tenant-ID header, URL path parameter, or subdomain.
// Note: Use ValidateTenantAccess middleware after authentication to ensure
// users can only access their own tenant.
func TenantContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var tenantID string

		// 1. Check X-Tenant-ID header
		if h := r.Header.Get("X-Tenant-ID"); h != "" {
			tenantID = h
		}

		// 2. Check URL path for /tenant/{id}/... pattern
		if tenantID == "" {
			parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
			if len(parts) >= 2 && parts[0] == "tenant" {
				tenantID = parts[1]
			}
		}

		// 3. Check subdomain (tenant.example.com)
		if tenantID == "" {
			host := r.Host
			if idx := strings.Index(host, ":"); idx > 0 {
				host = host[:idx]
			}
			parts := strings.Split(host, ".")
			if len(parts) >= 3 {
				// Assume first part is tenant subdomain
				subdomain := parts[0]
				if subdomain != "www" && subdomain != "api" {
					tenantID = subdomain
				}
			}
		}

		if tenantID != "" {
			ctx := context.WithValue(r.Context(), TenantIDKey, tenantID)
			r = r.WithContext(ctx)
		}

		next.ServeHTTP(w, r)
	})
}

// ValidateTenantAccess ensures the authenticated user can only access their own tenant.
// This middleware should be applied AFTER authentication middleware.
// It prevents horizontal privilege escalation by validating that any tenant ID
// in the request context matches the user's tenant from their auth claims.
type ValidateTenantAccess struct {
	// TenantFromClaimsOnly if true (the default), ignores X-Tenant-ID header and uses
	// only the tenant from claims. This is the secure default. Set to false only if
	// you need to allow header-based tenant selection (requires explicit opt-in).
	TenantFromClaimsOnly bool
}

// NewValidateTenantAccess creates a new tenant validation middleware.
// By default, TenantFromClaimsOnly is true (secure mode).
func NewValidateTenantAccess() *ValidateTenantAccess {
	return &ValidateTenantAccess{
		TenantFromClaimsOnly: true, // Secure default: ignore X-Tenant-ID header
	}
}

// WithTenantFromClaimsOnly configures whether to ignore X-Tenant-ID headers.
// When true (default), tenant is derived exclusively from authenticated claims.
// When false, X-Tenant-ID header is validated against claims (must match).
func (v *ValidateTenantAccess) WithTenantFromClaimsOnly(enabled bool) *ValidateTenantAccess {
	v.TenantFromClaimsOnly = enabled
	return v
}

// Handler returns the middleware handler.
func (v *ValidateTenantAccess) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if user is authenticated using the auth package
		claims, authenticated := auth.ClaimsFromContext(r.Context())

		// If TenantFromClaimsOnly mode and user is authenticated, override tenant context
		if v.TenantFromClaimsOnly && authenticated && claims != nil && claims.TenantID > 0 {
			// Override the tenant in context with the one from claims
			// This effectively ignores any X-Tenant-ID header
			tenantStr := formatTenantID(claims.TenantID)
			ctx := context.WithValue(r.Context(), TenantIDKey, tenantStr)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Standard validation mode: check requested tenant matches claims
		requestedTenant := TenantFromContext(r.Context())
		if requestedTenant == "" {
			// No tenant in request - allow (RequireTenant will catch if needed)
			next.ServeHTTP(w, r)
			return
		}

		if !authenticated || claims == nil {
			// Not authenticated - allow (auth middleware will catch if needed)
			next.ServeHTTP(w, r)
			return
		}

		// Validate tenant matches
		userTenant := claims.TenantID
		if userTenant > 0 {
			// Parse requested tenant strictly - must be purely numeric
			requestedTenantID, valid := parseStrictInt64(requestedTenant)
			if !valid {
				// Invalid tenant ID format - reject
				http.Error(w, `{"error":"bad_request","message":"Invalid tenant ID format"}`, http.StatusBadRequest)
				return
			}

			if requestedTenantID != userTenant {
				http.Error(w, `{"error":"forbidden","message":"You do not have access to this tenant"}`, http.StatusForbidden)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// formatTenantID converts int64 to string for context storage
func formatTenantID(id int64) string {
	if id == 0 {
		return "0"
	}
	var result []byte
	for id > 0 {
		result = append([]byte{byte('0' + id%10)}, result...)
		id /= 10
	}
	return string(result)
}

// parseStrictInt64 parses a string as int64, returning false if the string
// contains any non-numeric characters. This prevents "123abc" from being
// parsed as 123.
func parseStrictInt64(s string) (int64, bool) {
	if len(s) == 0 {
		return 0, false
	}

	var result int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false // Invalid character
		}
		result = result*10 + int64(c-'0')
	}
	return result, true
}

// RequireTenant ensures a tenant ID is present in the context.
func RequireTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := TenantFromContext(r.Context())
		if tenantID == "" {
			WriteError(w, http.StatusBadRequest, "tenant_required", "Tenant ID is required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// LocaleDetector detects the user's preferred locale from Accept-Language header.
func LocaleDetector(supportedLocales []string) func(http.Handler) http.Handler {
	supported := make(map[string]bool)
	for _, loc := range supportedLocales {
		supported[loc] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			locale := "en" // default

			// Check Accept-Language header
			acceptLang := r.Header.Get("Accept-Language")
			if acceptLang != "" {
				// Parse Accept-Language (simplified)
				// Format: en-US,en;q=0.9,es;q=0.8
				parts := strings.Split(acceptLang, ",")
				for _, part := range parts {
					lang := strings.TrimSpace(strings.Split(part, ";")[0])
					// Try exact match
					if supported[lang] {
						locale = lang
						break
					}
					// Try language without region (es-AR -> es)
					if idx := strings.Index(lang, "-"); idx > 0 {
						lang = lang[:idx]
						if supported[lang] {
							locale = lang
							break
						}
					}
				}
			}

			// Check query parameter override
			if q := r.URL.Query().Get("lang"); q != "" && supported[q] {
				locale = q
			}

			ctx := context.WithValue(r.Context(), LocaleKey, locale)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequestLogger logs HTTP requests with structured logging.
type RequestLogger struct {
	Logger *slog.Logger
}

// NewRequestLogger creates a new request logger middleware.
func NewRequestLogger(logger *slog.Logger) *RequestLogger {
	if logger == nil {
		logger = slog.Default()
	}
	return &RequestLogger{Logger: logger}
}

// Handler returns the middleware handler.
func (l *RequestLogger) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Generate request ID
		requestID := fmt.Sprintf("%d", time.Now().UnixNano())
		ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
		r = r.WithContext(ctx)

		// Wrap response writer to capture status
		ww := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		// Process request
		next.ServeHTTP(ww, r)

		// Log request
		duration := time.Since(start)
		l.Logger.Info("http request",
			slog.String("request_id", requestID),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", ww.status),
			slog.Int("bytes", ww.bytes),
			slog.Duration("duration", duration),
			slog.String("tenant_id", TenantFromContext(r.Context())),
			slog.String("remote_addr", r.RemoteAddr),
		)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code and bytes written.
type responseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *responseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

// Recoverer recovers from panics and returns a 500 error.
func Recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("panic recovered",
						slog.Any("error", err),
						slog.String("stack", string(debug.Stack())),
						slog.String("path", r.URL.Path),
						slog.String("method", r.Method),
					)

					WriteError(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimiter provides per-tenant rate limiting.
type RateLimiter struct {
	mu          sync.RWMutex
	limits      map[string]*bucket
	maxRequests int
	window      time.Duration
}

type bucket struct {
	count     int
	resetTime time.Time
}

// NewRateLimiter creates a new rate limiter.
// maxRequests is the maximum requests per window duration.
func NewRateLimiter(maxRequests int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limits:      make(map[string]*bucket),
		maxRequests: maxRequests,
		window:      window,
	}
}

// Handler returns the middleware handler.
func (rl *RateLimiter) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use tenant ID as key, fall back to remote IP
		key := TenantFromContext(r.Context())
		if key == "" {
			key = r.RemoteAddr
		}

		if !rl.allow(key) {
			w.Header().Set("Retry-After", "60")
			WriteError(w, http.StatusTooManyRequests, "rate_limit_exceeded", "Too many requests")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.limits[key]

	if !ok || now.After(b.resetTime) {
		rl.limits[key] = &bucket{
			count:     1,
			resetTime: now.Add(rl.window),
		}
		return true
	}

	if b.count >= rl.maxRequests {
		return false
	}

	b.count++
	return true
}

// CORS adds CORS headers to responses.
func CORS(allowedOrigins []string, allowCredentials bool) func(http.Handler) http.Handler {
	originSet := make(map[string]bool)
	for _, o := range allowedOrigins {
		originSet[o] = true
	}
	allowAll := len(allowedOrigins) == 0 || originSet["*"]

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if allowAll {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else if originSet[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}

			if allowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-Tenant-ID, X-Request-ID")
			w.Header().Set("Access-Control-Max-Age", "86400")

			// Handle preflight
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ContentType sets the Content-Type header for JSON responses.
func ContentType(contentType string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", contentType)
			next.ServeHTTP(w, r)
		})
	}
}

// JSON is a convenience middleware for JSON API endpoints.
func JSON(next http.Handler) http.Handler {
	return ContentType("application/json; charset=utf-8")(next)
}

// ErrorResponse represents a JSON error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details any    `json:"details,omitempty"`
}

// WriteError writes a JSON error response.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error: message,
		Code:  code,
	})
}

// WriteErrorWithDetails writes a JSON error response with additional details.
func WriteErrorWithDetails(w http.ResponseWriter, status int, code, message string, details any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   message,
		Code:    code,
		Details: details,
	})
}

// WriteJSON writes a JSON response.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
