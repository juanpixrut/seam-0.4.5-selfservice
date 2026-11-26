package api

import (
	"github.com/ha1tch/seam-ui/internal/fsm"
	"github.com/ha1tch/seam-ui/internal/rules"
	"net/http"
	"time"

	"github.com/ha1tch/seam-ui/internal/alerts"
	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/handlers/middleware"
	"github.com/ha1tch/seam-ui/internal/qrcode"
	"github.com/ha1tch/seam-ui/internal/reports"
	appversion "github.com/ha1tch/seam-ui/internal/version"
	"github.com/ha1tch/seam-ui/internal/webhook"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

// Router sets up all API routes.
type Router struct {
	mux             *http.ServeMux
	baseHandler     *Handler
	authMiddleware  *auth.Middleware
	tenantValidator *middleware.ValidateTenantAccess

	// Route registry for testing and introspection
	protectedRoutes []RouteInfo
	publicRoutes    []RouteInfo

	// Entity handlers
	Assets     *AssetHandler
	Events     *EventHandler
	Sensors    *SensorHandler
	AssetTypes *AssetTypeHandler
	Rules      *RuleHandler
	FSM        *FSMHandler
	Auth       *AuthHandler
	Webhooks   *WebhookHandler
	QR         *QRHandler
	Reports    *ReportsHandler
	Alerts     *AlertsHandler
}

// RouteInfo describes a registered route for testing and introspection.
type RouteInfo struct {
	Method     string
	Path       string
	Permission auth.Permission // Empty for public routes
	AuthOnly   bool            // True if auth required but no specific permission
}

// RouterConfig holds router configuration.
type RouterConfig struct {
	XOLUClient           *xolu.Client
	AuthService          *auth.Service
	AuthMiddleware       *auth.Middleware
	WebhookService       *webhook.Service
	QRGenerator          *qrcode.Generator
	ReportsService       *reports.Service
	AlertsService        *alerts.Service
	Locales              []string
	RateLimit            int // requests per minute, 0 = unlimited
	AllowedOrigins       []string
	TenantFromClaimsOnly bool // if true, ignore X-Tenant-ID header for authenticated requests
}

// NewRouter creates a new API router with all handlers registered.
func NewRouter(config RouterConfig) *Router {
	baseHandler := NewHandler(config.XOLUClient)

	// Create tenant validator for use in per-route wrappers
	tenantValidator := middleware.NewValidateTenantAccess().
		WithTenantFromClaimsOnly(config.TenantFromClaimsOnly)

	r := &Router{
		mux:             http.NewServeMux(),
		baseHandler:     baseHandler,
		authMiddleware:  config.AuthMiddleware,
		tenantValidator: tenantValidator,
		Assets:          NewAssetHandler(baseHandler),
		Events:          NewEventHandler(baseHandler),
		Sensors:         NewSensorHandler(baseHandler),
		AssetTypes:      NewAssetTypeHandler(baseHandler),
		Rules:           NewRuleHandler(baseHandler, rules.NewEngine()),
		FSM:             NewFSMHandler(baseHandler, fsm.NewEvaluator()),
	}

	// Add auth handler if service is provided
	if config.AuthService != nil {
		r.Auth = NewAuthHandler(config.XOLUClient, config.AuthService)
	}

	// Add webhook handler if service is provided
	if config.WebhookService != nil {
		r.Webhooks = NewWebhookHandler(config.WebhookService)
	}

	// Add QR handler if generator is provided
	if config.QRGenerator != nil {
		r.QR = NewQRHandler(baseHandler, config.QRGenerator)
	}

	// Add Reports handler if service is provided
	if config.ReportsService != nil {
		r.Reports = NewReportsHandler(baseHandler, config.ReportsService)
	}

	// Add Alerts handler if service is provided
	if config.AlertsService != nil {
		r.Alerts = NewAlertsHandler(baseHandler, config.AlertsService)
	}

	r.registerRoutes()

	return r
}

// Handler returns the http.Handler with all middleware applied.
func (r *Router) Handler(config RouterConfig) http.Handler {
	var handler http.Handler = r.mux

	// Apply middleware in reverse order (last applied runs first)

	// JSON content type for API responses
	handler = middleware.JSON(handler)

	// Locale detection
	locales := config.Locales
	if len(locales) == 0 {
		locales = []string{"en", "es", "pt"}
	}
	handler = middleware.LocaleDetector(locales)(handler)

	// Tenant context extraction (extracts from header/path/subdomain)
	// Note: Tenant validation happens in per-route wrappers (withPerm/withAuth)
	// AFTER auth sets claims, ensuring correct ordering.
	handler = middleware.TenantContext(handler)

	// Rate limiting (if configured)
	if config.RateLimit > 0 {
		rateLimiter := middleware.NewRateLimiter(config.RateLimit, time.Minute)
		handler = rateLimiter.Handler(handler)
	}

	// CORS
	origins := config.AllowedOrigins
	if len(origins) == 0 {
		origins = []string{"*"}
	}
	handler = middleware.CORS(origins, true)(handler)

	// Request logging
	logger := middleware.NewRequestLogger(nil)
	handler = logger.Handler(handler)

	// Panic recovery
	handler = middleware.Recoverer(nil)(handler)

	return handler
}

// registerRoutes registers all API routes.
func (r *Router) registerRoutes() {
	// Health check - intentionally public
	r.PublicGet("/api/v1/health", r.healthCheck)

	// Assets - permission protected
	r.ProtectedGet("/api/v1/assets/search", auth.PermAssetsAssetView, r.Assets.Search)
	r.ProtectedPost("/api/v1/assets/batch", auth.PermAssetsAssetEdit, r.Assets.BatchUpdate)
	r.ProtectedGet("/api/v1/assets", auth.PermAssetsAssetView, r.Assets.List)
	r.ProtectedPost("/api/v1/assets", auth.PermAssetsAssetCreate, r.Assets.Create)
	r.ProtectedGet("/api/v1/assets/{id}", auth.PermAssetsAssetView, r.Assets.Get)
	r.ProtectedPut("/api/v1/assets/{id}", auth.PermAssetsAssetEdit, r.Assets.Update)
	r.ProtectedPatch("/api/v1/assets/{id}", auth.PermAssetsAssetEdit, r.Assets.Patch)
	r.ProtectedDelete("/api/v1/assets/{id}", auth.PermAssetsAssetDelete, r.Assets.Delete)
	r.ProtectedPatch("/api/v1/assets/{id}/status", auth.PermAssetsAssetEdit, r.Assets.UpdateStatus)
	r.ProtectedGet("/api/v1/assets/{id}/events", auth.PermAssetsAssetView, r.Assets.GetEvents)

	// Events - permission protected
	r.ProtectedPost("/api/v1/events/batch", auth.PermAssetsAssetEdit, r.Events.IngestBatch)
	r.ProtectedGet("/api/v1/events/stats", auth.PermAssetsAssetView, r.Events.GetStats)
	r.ProtectedGet("/api/v1/events", auth.PermAssetsAssetView, r.Events.List)
	r.ProtectedPost("/api/v1/events", auth.PermAssetsAssetEdit, r.Events.Ingest)
	r.ProtectedGet("/api/v1/events/{id}", auth.PermAssetsAssetView, r.Events.Get)

	// Sensors - permission protected
	r.ProtectedGet("/api/v1/sensors", auth.PermAssetsAssetView, r.Sensors.List)
	r.ProtectedPost("/api/v1/sensors", auth.PermAssetsAssetCreate, r.Sensors.Create)
	r.ProtectedGet("/api/v1/sensors/{id}", auth.PermAssetsAssetView, r.Sensors.Get)
	r.ProtectedPut("/api/v1/sensors/{id}", auth.PermAssetsAssetEdit, r.Sensors.Update)
	r.ProtectedDelete("/api/v1/sensors/{id}", auth.PermAssetsAssetDelete, r.Sensors.Delete)
	r.ProtectedPost("/api/v1/sensors/{id}/bind", auth.PermAssetsAssetEdit, r.Sensors.Bind)
	r.ProtectedPost("/api/v1/sensors/{id}/unbind", auth.PermAssetsAssetEdit, r.Sensors.Unbind)
	r.ProtectedGet("/api/v1/sensors/{id}/bindings", auth.PermAssetsAssetView, r.Sensors.GetBindings)

	// Asset Types - permission protected
	r.ProtectedGet("/api/v1/asset-types", auth.PermAssetsAssetView, r.AssetTypes.List)
	r.ProtectedPost("/api/v1/asset-types", auth.PermAssetsAssetCreate, r.AssetTypes.Create)
	r.ProtectedGet("/api/v1/asset-types/{id}", auth.PermAssetsAssetView, r.AssetTypes.Get)
	r.ProtectedPut("/api/v1/asset-types/{id}", auth.PermAssetsAssetEdit, r.AssetTypes.Update)
	r.ProtectedDelete("/api/v1/asset-types/{id}", auth.PermAssetsAssetDelete, r.AssetTypes.Delete)
	r.ProtectedGet("/api/v1/asset-types/{id}/assets", auth.PermAssetsAssetView, r.AssetTypes.GetAssets)
	r.ProtectedGet("/api/v1/asset-types/{id}/identity-template", auth.PermAssetsAssetView, r.AssetTypes.GetIdentityTemplate)
	r.ProtectedPut("/api/v1/asset-types/{id}/identity-template", auth.PermAssetsAssetEdit, r.AssetTypes.UpdateIdentityTemplate)

	// Rules - permission protected
	r.ProtectedGet("/api/v1/rules", auth.PermAssetsRuleView, r.Rules.List)
	r.ProtectedPost("/api/v1/rules", auth.PermAssetsRuleCreate, r.Rules.Create)
	r.ProtectedGet("/api/v1/rules/{id}", auth.PermAssetsRuleView, r.Rules.Get)
	r.ProtectedPut("/api/v1/rules/{id}", auth.PermAssetsRuleEdit, r.Rules.Update)
	r.ProtectedDelete("/api/v1/rules/{id}", auth.PermAssetsRuleDelete, r.Rules.Delete)
	r.ProtectedPost("/api/v1/rules/{id}/enable", auth.PermAssetsRuleEdit, r.Rules.Enable)
	r.ProtectedPost("/api/v1/rules/{id}/disable", auth.PermAssetsRuleEdit, r.Rules.Disable)
	r.ProtectedPost("/api/v1/rules/{id}/test", auth.PermAssetsRuleView, r.Rules.Test)

	// FSM - permission protected (uses rule permissions)
	r.ProtectedGet("/api/v1/fsm", auth.PermAssetsRuleView, r.FSM.List)
	r.ProtectedPost("/api/v1/fsm", auth.PermAssetsRuleCreate, r.FSM.Create)
	r.ProtectedGet("/api/v1/fsm/{id}", auth.PermAssetsRuleView, r.FSM.Get)
	r.ProtectedPut("/api/v1/fsm/{id}", auth.PermAssetsRuleEdit, r.FSM.Update)
	r.ProtectedDelete("/api/v1/fsm/{id}", auth.PermAssetsRuleDelete, r.FSM.Delete)
	r.ProtectedGet("/api/v1/fsm/{id}/states", auth.PermAssetsRuleView, r.FSM.GetStates)
	r.ProtectedGet("/api/v1/fsm/{id}/visualize", auth.PermAssetsRuleView, r.FSM.Visualize)
	r.ProtectedPost("/api/v1/fsm/{id}/simulate", auth.PermAssetsRuleView, r.FSM.Simulate)

	// Auth (if enabled) - mixed public and auth-required endpoints
	if r.Auth != nil {
		r.PublicPost("/api/v1/auth/login", r.Auth.Login)
		r.PublicPost("/api/v1/auth/refresh", r.Auth.Refresh)
		r.AuthPost("/api/v1/auth/logout", r.Auth.Logout)
		r.AuthGet("/api/v1/auth/me", r.Auth.Me)
		r.AuthPost("/api/v1/auth/change-password", r.Auth.ChangePassword)
	}

	// Webhooks (if enabled) - permission protected
	if r.Webhooks != nil {
		r.ProtectedGet("/api/v1/webhooks", auth.PermAssetsWebhookView, r.Webhooks.List)
		r.ProtectedPost("/api/v1/webhooks", auth.PermAssetsWebhookEdit, r.Webhooks.Create)
		r.ProtectedGet("/api/v1/webhooks/{id}", auth.PermAssetsWebhookView, r.Webhooks.Get)
		r.ProtectedPut("/api/v1/webhooks/{id}", auth.PermAssetsWebhookEdit, r.Webhooks.Update)
		r.ProtectedDelete("/api/v1/webhooks/{id}", auth.PermAssetsWebhookEdit, r.Webhooks.Delete)
		r.ProtectedPost("/api/v1/webhooks/{id}/enable", auth.PermAssetsWebhookEdit, r.Webhooks.Enable)
		r.ProtectedPost("/api/v1/webhooks/{id}/disable", auth.PermAssetsWebhookEdit, r.Webhooks.Disable)
		r.ProtectedPost("/api/v1/webhooks/{id}/test", auth.PermAssetsWebhookEdit, r.Webhooks.Test)
	}

	// QR Codes (if enabled) - permission protected
	if r.QR != nil {
		r.ProtectedPost("/api/v1/qr/generate", auth.PermAssetsAssetCreate, r.QR.Generate)
		r.ProtectedGet("/api/v1/qr/labels", auth.PermAssetsAssetView, r.QR.Labels)
		r.ProtectedGet("/api/v1/qr", auth.PermAssetsAssetView, r.QR.List)
		r.ProtectedGet("/api/v1/qr/{code}", auth.PermAssetsAssetView, r.QR.Get)
		r.ProtectedGet("/api/v1/qr/{code}/scan", auth.PermAssetsAssetView, r.QR.Scan)
		r.ProtectedPost("/api/v1/qr/{code}/assign", auth.PermAssetsAssetEdit, r.QR.Assign)
		r.ProtectedPost("/api/v1/qr/{code}/unassign", auth.PermAssetsAssetEdit, r.QR.Unassign)
		r.ProtectedPost("/api/v1/qr/{code}/enable", auth.PermAssetsAssetEdit, r.QR.Enable)
		r.ProtectedPost("/api/v1/qr/{code}/disable", auth.PermAssetsAssetEdit, r.QR.Disable)
	}

	// Reports (if enabled) - permission protected
	if r.Reports != nil {
		r.ProtectedGet("/api/v1/reports/dashboard", auth.PermAssetsAssetView, r.Reports.Dashboard)
		r.ProtectedGet("/api/v1/reports/assets", auth.PermAssetsAssetView, r.Reports.AssetReport)
		r.ProtectedGet("/api/v1/reports/events", auth.PermAssetsAssetView, r.Reports.EventReport)
		r.ProtectedGet("/api/v1/reports/export/assets", auth.PermAssetsReportExport, r.Reports.ExportAssets)
		r.ProtectedGet("/api/v1/reports/export/events", auth.PermAssetsReportExport, r.Reports.ExportEvents)
	}

	// Alerts (if enabled) - permission protected
	if r.Alerts != nil {
		r.ProtectedGet("/api/v1/alerts", auth.PermAssetsAssetView, r.Alerts.List)
		r.ProtectedPost("/api/v1/alerts", auth.PermAssetsAssetEdit, r.Alerts.Create)
		r.ProtectedGet("/api/v1/alerts/summary", auth.PermAssetsAssetView, r.Alerts.Summary)
		r.ProtectedGet("/api/v1/alerts/{id}", auth.PermAssetsAssetView, r.Alerts.Get)
		r.ProtectedPost("/api/v1/alerts/{id}/acknowledge", auth.PermAssetsAssetEdit, r.Alerts.Acknowledge)
		r.ProtectedPost("/api/v1/alerts/{id}/resolve", auth.PermAssetsAssetEdit, r.Alerts.Resolve)
		r.ProtectedPost("/api/v1/alerts/{id}/dismiss", auth.PermAssetsAssetEdit, r.Alerts.Dismiss)
	}
}

// healthCheck handles GET /api/v1/health
func (r *Router) healthCheck(w http.ResponseWriter, req *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]any{
		"status":  "healthy",
		"version": appversion.Version,
	})
}

// ServeHTTP implements http.Handler.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

// =============================================================================
// ROUTE REGISTRATION API
// =============================================================================
// These methods provide a structured way to register routes with explicit
// security posture. Using these methods instead of direct mux access ensures:
// 1. All routes are tracked for testing/introspection
// 2. Security intent is explicit (protected vs public)
// 3. Middleware wrapping is consistent

// ProtectedGet registers a GET route that requires authentication and permission.
func (r *Router) ProtectedGet(path string, perm auth.Permission, handler http.HandlerFunc) {
	r.protectedRoutes = append(r.protectedRoutes, RouteInfo{Method: "GET", Path: path, Permission: perm})
	r.mux.HandleFunc("GET "+path, r.withPerm(perm, handler))
}

// ProtectedPost registers a POST route that requires authentication and permission.
func (r *Router) ProtectedPost(path string, perm auth.Permission, handler http.HandlerFunc) {
	r.protectedRoutes = append(r.protectedRoutes, RouteInfo{Method: "POST", Path: path, Permission: perm})
	r.mux.HandleFunc("POST "+path, r.withPerm(perm, handler))
}

// ProtectedPut registers a PUT route that requires authentication and permission.
func (r *Router) ProtectedPut(path string, perm auth.Permission, handler http.HandlerFunc) {
	r.protectedRoutes = append(r.protectedRoutes, RouteInfo{Method: "PUT", Path: path, Permission: perm})
	r.mux.HandleFunc("PUT "+path, r.withPerm(perm, handler))
}

// ProtectedPatch registers a PATCH route that requires authentication and permission.
func (r *Router) ProtectedPatch(path string, perm auth.Permission, handler http.HandlerFunc) {
	r.protectedRoutes = append(r.protectedRoutes, RouteInfo{Method: "PATCH", Path: path, Permission: perm})
	r.mux.HandleFunc("PATCH "+path, r.withPerm(perm, handler))
}

// ProtectedDelete registers a DELETE route that requires authentication and permission.
func (r *Router) ProtectedDelete(path string, perm auth.Permission, handler http.HandlerFunc) {
	r.protectedRoutes = append(r.protectedRoutes, RouteInfo{Method: "DELETE", Path: path, Permission: perm})
	r.mux.HandleFunc("DELETE "+path, r.withPerm(perm, handler))
}

// AuthGet registers a GET route that requires authentication but no specific permission.
func (r *Router) AuthGet(path string, handler http.HandlerFunc) {
	r.protectedRoutes = append(r.protectedRoutes, RouteInfo{Method: "GET", Path: path, AuthOnly: true})
	r.mux.HandleFunc("GET "+path, r.withAuth(handler))
}

// AuthPost registers a POST route that requires authentication but no specific permission.
func (r *Router) AuthPost(path string, handler http.HandlerFunc) {
	r.protectedRoutes = append(r.protectedRoutes, RouteInfo{Method: "POST", Path: path, AuthOnly: true})
	r.mux.HandleFunc("POST "+path, r.withAuth(handler))
}

// PublicGet registers a GET route that is intentionally public (no auth required).
func (r *Router) PublicGet(path string, handler http.HandlerFunc) {
	r.publicRoutes = append(r.publicRoutes, RouteInfo{Method: "GET", Path: path})
	r.mux.HandleFunc("GET "+path, r.withPublic(handler))
}

// PublicPost registers a POST route that is intentionally public (no auth required).
func (r *Router) PublicPost(path string, handler http.HandlerFunc) {
	r.publicRoutes = append(r.publicRoutes, RouteInfo{Method: "POST", Path: path})
	r.mux.HandleFunc("POST "+path, r.withPublic(handler))
}

// GetProtectedRoutes returns all registered protected routes for testing.
func (r *Router) GetProtectedRoutes() []RouteInfo {
	return r.protectedRoutes
}

// GetPublicRoutes returns all registered public routes for testing.
func (r *Router) GetPublicRoutes() []RouteInfo {
	return r.publicRoutes
}

// withPerm wraps a handler function with authentication, permission checking, and tenant validation.
// The wrapping order is: auth (sets claims) → permission check → tenant validation → handler
// This ensures tenant validation sees the authenticated user's claims.
// If authMiddleware is nil, the handler is returned unwrapped.
func (r *Router) withPerm(perm auth.Permission, handler http.HandlerFunc) http.HandlerFunc {
	if r.authMiddleware == nil {
		return handler
	}
	// Wrap handler with tenant validation first (innermost)
	var wrapped http.Handler = http.HandlerFunc(handler)
	if r.tenantValidator != nil {
		wrapped = r.tenantValidator.Handler(wrapped)
	}
	// Then wrap with permission check (checks claims have required permission)
	wrapped = r.authMiddleware.RequirePermission(perm)(wrapped)
	// Then wrap with auth (outermost - runs first, sets claims in context)
	wrapped = r.authMiddleware.Required(wrapped)
	return wrapped.ServeHTTP
}

// withAuth wraps a handler function with authentication requirement and tenant validation.
// The wrapping order is: auth (sets claims) → tenant validation → handler
// This ensures tenant validation sees the authenticated user's claims.
// If authMiddleware is nil, the handler is returned unwrapped.
func (r *Router) withAuth(handler http.HandlerFunc) http.HandlerFunc {
	if r.authMiddleware == nil {
		return handler
	}
	// Wrap handler with tenant validation first (innermost)
	var wrapped http.Handler = http.HandlerFunc(handler)
	if r.tenantValidator != nil {
		wrapped = r.tenantValidator.Handler(wrapped)
	}
	// Then wrap with auth (outermost - runs first, sets claims in context)
	wrapped = r.authMiddleware.Required(wrapped)
	return wrapped.ServeHTTP
}

// withPublic wraps a handler function for public routes.
// This marks the route as intentionally public and applies minimal middleware.
// The wrapper exists to distinguish "public by design" from "accidentally unprotected".
func (r *Router) withPublic(handler http.HandlerFunc) http.HandlerFunc {
	// Public routes get no auth wrapping, but we could add logging/metrics here
	return handler
}
