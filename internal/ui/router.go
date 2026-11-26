package ui

import (
	"context"
	"crypto/subtle"
	"net/http"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/features"
	"github.com/ha1tch/seam-ui/internal/i18n"
	"github.com/ha1tch/seam-ui/internal/modules"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

// Router sets up all UI routes.
type Router struct {
	mux            *http.ServeMux
	handler        *UIHandler
	config         Config
	translator     *i18n.Translator
	userService    *auth.UserService
	sessionManager *auth.SessionManager
	csrfManager    *auth.CSRFManager
	loginLimiter   *auth.LoginLimiter
	auditLogger    *auth.AuditLogger
	resetManager   *auth.PasswordResetManager
}

// NewRouter creates a new UI router.
func NewRouter(client *xolu.Client, cfg Config, translator *i18n.Translator, userService *auth.UserService, sessionMgr *auth.SessionManager, csrfMgr *auth.CSRFManager, loginLimiter *auth.LoginLimiter, auditLogger *auth.AuditLogger, resetMgr *auth.PasswordResetManager) *Router {
	r := &Router{
		mux:            http.NewServeMux(),
		handler:        NewUIHandler(client, cfg, translator, userService, sessionMgr, csrfMgr, loginLimiter, auditLogger, resetMgr),
		config:         cfg,
		translator:     translator,
		userService:    userService,
		sessionManager: sessionMgr,
		csrfManager:    csrfMgr,
		loginLimiter:   loginLimiter,
		auditLogger:    auditLogger,
		resetManager:   resetMgr,
	}
	r.registerRoutes()
	return r
}

// SetModuleRegistry wires a *modules.Registry into the router's underlying
// page handler, so the module selector (MODULE_SPEC.md §4) has data to
// render. r.handler is unexported (internal/server can't reach it
// directly), so this delegates on the caller's behalf -- same reasoning
// PageHandler.SetModuleRegistry itself gives for being a setter rather
// than a constructor parameter: not disrupting NewRouter's already-long
// signature for a stage (MOD-4) that doesn't require every caller to
// supply one.
func (r *Router) SetModuleRegistry(reg *modules.Registry) {
	r.handler.page.SetModuleRegistry(reg)
}

// ServeHTTP implements http.Handler.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

// requireAuth wraps a handler to require authentication.
// Redirects to /login if no valid session exists.
// Also validates CSRF tokens on state-changing requests.
func (r *Router) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Skip auth check if session manager not configured
		if r.sessionManager == nil {
			next.ServeHTTP(w, req)
			return
		}

		session, err := r.sessionManager.ValidateSession(req)
		if err != nil {
			// Redirect to login with return URL
			returnURL := req.URL.Path
			if req.URL.RawQuery != "" {
				returnURL += "?" + req.URL.RawQuery
			}
			http.Redirect(w, req, "/login?next="+returnURL, http.StatusSeeOther)
			return
		}

		// Validate CSRF on state-changing requests
		if r.csrfManager != nil {
			switch req.Method {
			case "POST", "PUT", "PATCH", "DELETE":
				if err := r.csrfManager.ValidateToken(req); err != nil {
					// Audit log: CSRF violation
					if r.auditLogger != nil {
						r.auditLogger.LogCSRFViolation(req.Context(), req, session.UserID)
					}
					r.handler.ErrorCSRF(w, req)
					return
				}
			}
		}

		// Add session claims to context
		ctx := r.contextFor(req, session)
		next.ServeHTTP(w, req.WithContext(ctx))
	}
}

// contextFor is the request context for an authenticated session: its claims,
// and, while a developer is viewing as another role (campaign DEV-22), claims
// that carry that role with the real one recorded beside them. Viewing as is
// ignored unless the Developer module is on and the session is a developer's.
func (r *Router) contextFor(req *http.Request, session *auth.SessionData) context.Context {
	claims := session.ToClaims()
	ctx := req.Context()
	if r.config.mayViewAs(session.Role, session.ViewAs) {
		claims.Role = session.ViewAs
		ctx = auth.ContextWithRealRole(ctx, session.Role)
	}
	return auth.ContextWithClaims(ctx, claims)
}

// requireAuthRawBody is like requireAuth but validates CSRF only from the
// request header, never calling ParseForm. Use for endpoints where the
// request body is not a form (e.g. Content-Type: image/svg+xml).
// Calling r.FormValue() on such requests discards the body before the
// handler can read it.
func (r *Router) requireAuthRawBody(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if r.sessionManager == nil {
			next.ServeHTTP(w, req)
			return
		}
		session, err := r.sessionManager.ValidateSession(req)
		if err != nil {
			returnURL := req.URL.Path
			if req.URL.RawQuery != "" {
				returnURL += "?" + req.URL.RawQuery
			}
			http.Redirect(w, req, "/login?next="+returnURL, http.StatusSeeOther)
			return
		}
		// CSRF: header only — do not call ParseForm which would consume the body
		if r.csrfManager != nil {
			switch req.Method {
			case "POST", "PUT", "PATCH", "DELETE":
				providedToken := req.Header.Get(auth.CSRFHeaderName)
				cookie, cookieErr := req.Cookie(auth.CSRFCookieName)
				if cookieErr != nil || providedToken == "" {
					if r.auditLogger != nil {
						r.auditLogger.LogCSRFViolation(req.Context(), req, session.UserID)
					}
					r.handler.ErrorCSRF(w, req)
					return
				}
				if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(providedToken)) != 1 {
					if r.auditLogger != nil {
						r.auditLogger.LogCSRFViolation(req.Context(), req, session.UserID)
					}
					r.handler.ErrorCSRF(w, req)
					return
				}
			}
		}
		ctx := r.contextFor(req, session)
		next.ServeHTTP(w, req.WithContext(ctx))
	}
}

// requirePerm wraps a handler to require a specific permission.
// Must be used after requireAuth.
func (r *Router) requirePerm(perm auth.Permission, next http.HandlerFunc) http.HandlerFunc {
	return r.requireAuth(func(w http.ResponseWriter, req *http.Request) {
		claims, ok := auth.ClaimsFromContext(req.Context())
		if !ok {
			http.Redirect(w, req, "/login", http.StatusSeeOther)
			return
		}

		if !auth.HasPermission(claims.Role, perm) {
			// Audit log: access denied
			if r.auditLogger != nil {
				r.auditLogger.LogAccessDenied(req.Context(), req, claims.UserID, req.URL.Path, string(perm))
			}
			r.handler.Error403(w, req)
			return
		}

		next.ServeHTTP(w, req)
	})
}

// requireRealPerm is requirePerm for the Developer module: it checks the role
// the session really has, not the one it is viewing as, so a developer who is
// looking at the application as a viewer can still reach the module to leave.
func (r *Router) requireRealPerm(perm auth.Permission, next http.HandlerFunc) http.HandlerFunc {
	return r.requireAuth(func(w http.ResponseWriter, req *http.Request) {
		claims, ok := auth.ClaimsFromContext(req.Context())
		if !ok {
			http.Redirect(w, req, "/login", http.StatusSeeOther)
			return
		}
		if !auth.HasPermission(auth.RealRole(req.Context(), claims), perm) {
			if r.auditLogger != nil {
				r.auditLogger.LogAccessDenied(req.Context(), req, claims.UserID, req.URL.Path, string(perm))
			}
			r.handler.Error403(w, req)
			return
		}
		next.ServeHTTP(w, req)
	})
}

// registerRoutes registers all UI routes.
func (r *Router) registerRoutes() {
	// Public routes (no auth required)
	r.mux.HandleFunc("GET /login", r.handler.LoginPage)
	r.mux.HandleFunc("POST /login", r.handler.LoginSubmit)
	r.mux.HandleFunc("GET /logout", r.handler.Logout)

	// Password reset (public)
	r.mux.HandleFunc("GET /forgot-password", r.handler.ForgotPasswordPage)
	r.mux.HandleFunc("POST /forgot-password", r.handler.ForgotPasswordSubmit)
	r.mux.HandleFunc("GET /reset-password", r.handler.ResetPasswordPage)
	r.mux.HandleFunc("POST /reset-password", r.handler.ResetPasswordSubmit)

	// QR Scan (public - for scanning QR codes)
	r.mux.HandleFunc("GET /s/{code}", r.handler.QRScan)

	// Static files (public)
	staticFS := http.FileServer(http.Dir("web/static"))
	r.mux.Handle("GET /static/", http.StripPrefix("/static/", staticFS))

	// Protected routes (require auth)

	// Asset-picker search (T-19, 2026-08-04) -- a shared utility
	// endpoint, not tied to any one module: the Asset form's own
	// parent_id field and the Sensor bind page both call it. Kept
	// inline here rather than moved into a MountXRoutes method for
	// that reason -- it doesn't belong to Assets, Sensors, or any
	// other single module more than the others.
	r.mux.HandleFunc("GET /picker/search", r.requireAuth(r.handler.PickerSearch))
	// query-picker widget (docs/QUERY_PICKER.md): search, value lookup and the
	// Browse dialog over a field's own sealed query. Cross-module, like
	// /picker/search: what a field may read is fixed by its schema.
	r.mux.HandleFunc("GET /picker/query", r.requireAuth(r.handler.PickerQuery))
	r.mux.HandleFunc("GET /picker/query/browse", r.requireAuth(r.handler.PickerQueryBrowse))
	// Icon search for the icon-picker widget (names from the vendored font's ligature list).
	r.mux.HandleFunc("GET /picker/icons", r.requireAuth(r.handler.PickerIcons))

	// Selector dialog (T-96) -- a generic "choose one record" modal body,
	// registry-driven (selectorSources in selector.go). Open to every
	// authenticated role; what each source lists is restricted inside the
	// source itself (the Users source lists only users at or below the
	// caller's role). Same cross-module reasoning as /picker/search above.
	r.mux.HandleFunc("GET /selector/{key}", r.requireAuth(r.handler.Selector))

	// Gallery — backend for the "image" formengine widget (T-10), same
	// reasoning as /picker/search directly above: a generic, cross-
	// module endpoint (any owner_type in galleryOwnerTypes, not tied to
	// Assets specifically despite that being the only real consumer
	// today), kept inline here rather than under MountAssetRoutes.
	// GalleryUpload is multipart/form-data -- requireAuth (not
	// requireAuthRawBody) is correct here, not requireAuthRawBody's
	// raw-body case: Go's own r.FormValue (called inside
	// csrfManager.ValidateToken) already parses multipart bodies
	// transparently and caches the result, so the handler's own later
	// r.ParseMultipartForm call is a safe, cheap no-op, not a second
	// real parse. GalleryDelete has no body at all (id is a query
	// param) -- CSRF for it comes through ValidateToken's own header
	// fallback (X-CSRF-Token), which the widget's JS sets explicitly
	// since there's no form body for a bodyless DELETE to carry it in.
	r.mux.HandleFunc("GET /gallery/items", r.requireAuth(r.handler.GalleryList))
	r.mux.HandleFunc("POST /gallery/upload", r.requireAuth(r.handler.GalleryUpload))
	r.mux.HandleFunc("DELETE /gallery/item", r.requireAuth(r.handler.GalleryDelete))
	r.mux.HandleFunc("GET /gallery/blob", r.requireAuth(r.handler.GalleryBlobProxy))

	// Dashboard, Assets, Events, Sensors, Alerts, Map, FSM Editor, Rules,
	// Asset Types, and every master table are registered by
	// MountAssetRoutes (MOD-6), called from internal/modules/assets via
	// internal/server/server.go -- not here. See MountAssetRoutes' own
	// doc comment for why the call site moved but the handlers/
	// middleware didn't (same reasoning as MountAccountRoutes/
	// MountSettingsRoutes/MountAdministrationRoutes, MOD-5's original
	// single MountSystemRoutes before T-02 split it three ways).

	// User Profile, Settings, and Administration are registered by
	// MountAccountRoutes/MountSettingsRoutes/MountAdministrationRoutes
	// respectively (T-02, 2026-08-04, splitting the former single
	// MountSystemRoutes), each called from its own internal/modules/*
	// package via internal/server/server.go -- not here.
}

// MountSystemRoutes registers the System module's routes -- User Profile
// (always available), Settings and Administration (gated behind
// r.config.EnableAdmin, exactly as they were when registered inline here),
// and the System dashboard itself (GET /system, new in MOD-5) -- onto mux.
//
// This is MOD-5's answer to MODULE_SPEC.md §6.1's "only the registration
// call site moves" pattern (written for Assets, applied here to System):
// the handler logic (r.handler.Settings etc.) and the auth/CSRF/permission
// middleware (r.requireAuth, r.requirePerm) are exactly what they always
// were -- unchanged, still private to this package, still the same
// Router instance's own session/CSRF state. Only WHERE these HandleFunc
// calls happen moved, from inline in registerRoutes() (r.mux, this
// Router's own private mux) to here, called via a closure internal/
// server/server.go constructs (system.MountRoutes(uiRouter)) against
// mux -- the server's shared top-level mux (modules.Registry's own
// stated contract), not r.mux. Registered exactly once: registerRoutes()
// no longer registers any of these paths, so there is no duplicate
// registration and no shadowed/dead route on either mux.
// MountAccountRoutes registers the Account module's routes -- User
// Profile only. Unconditional, no permission gate at all beyond being
// authenticated (MODULE_SPEC.md §5.1's original rule for the Profile
// view, preserved unchanged): every user needs to reach their own
// profile regardless of role, so this is deliberately the one module
// whose HasAccess is always true.
//
// Split out from the former MountSystemRoutes (MOD-5/MOD-8) 2026-08-04,
// per the tech team's decision: Account, Settings, and Administration are
// three separate modules, not one -- specifically because bundling
// Profile with anything permission-gated forces the combined module's
// own HasAccess to stay true for everyone, pushing all real access
// control down into per-route checks instead of the module boundary
// itself (unlike Assets/Documents, which gate at the module level).
// Splitting preserves the permission granularity system.settings.*/
// system.user.* already have, rather than collapsing it back down.
func (r *Router) MountAccountRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /profile", r.requireAuth(r.handler.Profile))
	mux.HandleFunc("POST /profile", r.requireAuth(r.handler.ProfileSave))
	mux.HandleFunc("POST /profile/preferences", r.requireAuth(r.handler.ProfilePreferencesSave))
	mux.HandleFunc("GET /profile/password", r.requireAuth(r.handler.PasswordChange))
	mux.HandleFunc("POST /profile/password", r.requireAuth(r.handler.PasswordChangeSave))
}

// MountSettingsRoutes registers the Settings module's routes -- tenant
// configuration (general/notifications/features). Gated behind
// r.config.EnableAdmin (same flag Settings/Administration always
// shared, unchanged by this split) and, per route, system.settings.*
// -- a permission namespace that already existed independently of
// system.user.*, not invented for this split. See MountAccountRoutes'
// doc comment for why this is a separate module from Account.
func (r *Router) MountSettingsRoutes(mux *http.ServeMux) {
	if !r.config.EnableAdmin {
		return
	}
	mux.HandleFunc("GET /settings", r.requireAuth(r.handler.Settings))
	mux.HandleFunc("POST /settings/general", r.requirePerm(auth.PermSettingsEdit, r.handler.SettingsSaveGeneral))
	mux.HandleFunc("POST /settings/notifications", r.requirePerm(auth.PermSettingsEdit, r.handler.SettingsSaveNotifications))
	mux.HandleFunc("POST /settings/features", r.requirePerm(auth.PermSettingsEdit, r.handler.SettingsSaveFeatures))
}

// MountAdministrationRoutes registers the Administration module's
// routes -- user/role management. Gated behind r.config.EnableAdmin
// and, per route, system.user.* -- see MountSettingsRoutes' own
// comment for the same reasoning applied to this permission namespace.
func (r *Router) MountAdministrationRoutes(mux *http.ServeMux) {
	if !r.config.EnableAdmin {
		return
	}
	mux.HandleFunc("GET /admin/users", r.requirePerm(auth.PermAdministrationUserView, r.handler.AdminUsersList))
	mux.HandleFunc("GET /admin/users/new", r.requirePerm(auth.PermAdministrationUserCreate, r.handler.AdminUserNew))
	mux.HandleFunc("POST /admin/users", r.requirePerm(auth.PermAdministrationUserCreate, r.handler.AdminUserCreate))
	mux.HandleFunc("GET /admin/users/{id}", r.requirePerm(auth.PermAdministrationUserView, r.handler.AdminUserEdit))
	mux.HandleFunc("POST /admin/users/{id}", r.requirePerm(auth.PermAdministrationUserEdit, r.handler.AdminUserUpdate))
	mux.HandleFunc("GET /admin/users/{id}/password", r.requirePerm(auth.PermAdministrationUserEdit, r.handler.AdminUserPasswordReset))
	mux.HandleFunc("POST /admin/users/{id}/password", r.requirePerm(auth.PermAdministrationUserEdit, r.handler.AdminUserPasswordResetSave))
}

// MountAssetRoutes registers the Assets module's routes -- the
// dashboard (GET /{$}), Assets/Events/Sensors/Alerts/Map (gated behind
// r.config.EnableOperational, exactly as they were when registered
// inline here), and FSM Editor/Rules/Asset Types/every master table
// (gated behind r.config.EnableAdmin, same reasoning) -- onto mux.
//
// This is MOD-6's application of the exact pattern MountSystemRoutes
// (MOD-5) established, per MODULE_SPEC.md §6.1's "only the
// registration call site moves" reasoning and §5.4's concrete
// mechanism: the handler logic and the auth/permission middleware are
// exactly what they always were -- unchanged, still private to this
// package. Only WHERE these HandleFunc calls happen moved, from inline
// in registerRoutes() (r.mux, this Router's own private mux) to here,
// called via a closure internal/server/server.go constructs
// (assets.MountRoutes(uiRouter)) against mux -- the server's shared
// top-level mux, not r.mux. Registered exactly once: registerRoutes()
// no longer registers any of these paths.
//
// By far the largest of the two MountXRoutes methods (98 routes vs
// System's 13) -- confirmed directly against MODULE_SPEC.md §6.1's own
// description ("assets, sensors, rules, alerts, masters... ~3,000+
// lines"), not a narrower reading of "Assets module" than the spec
// intends. The lines below were extracted mechanically from their
// original location in registerRoutes(), not retyped, to eliminate
// transcription risk across a surface this size.
func (r *Router) MountAssetRoutes(mux *http.ServeMux) {
	// Dashboard (unconditional, matching its prior placement outside any
	// feature-flag gate)
	mux.HandleFunc("GET /{$}", r.requireAuth(r.handler.Dashboard))

	// Assets, Events, Sensors, Alerts, Map (SEAM_ENABLE_OPERATIONAL=true)
	if r.config.EnableOperational {
		// Assets
		mux.HandleFunc("GET /assets", r.requireAuth(r.handler.AssetsList))
		mux.HandleFunc("GET /assets/type-fields", r.requireAuth(r.handler.AssetTypeFields))
		mux.HandleFunc("GET /assets/{id}", r.requireAuth(r.handler.AssetDetail))
		mux.HandleFunc("GET /assets/{id}/qr/card", r.requireAuth(r.handler.AssetQRCodeCard))
		mux.HandleFunc("GET /assets/{id}/qr/image", r.requireAuth(r.handler.AssetQRCodeImage))
		mux.HandleFunc("POST /assets/{id}/qr/image", r.requirePerm(auth.PermAssetsAssetEdit, r.handler.AssetQRCodeRenderImage))
		mux.HandleFunc("POST /assets/{id}/qr/generate", r.requirePerm(auth.PermAssetsAssetEdit, r.handler.AssetQRCodeGenerate))
		mux.HandleFunc("POST /assets/{id}/qr/regenerate", r.requirePerm(auth.PermAssetsAssetEdit, r.handler.AssetQRCodeRegenerate))
		mux.HandleFunc("POST /assets/{id}/qr/revoke", r.requirePerm(auth.PermAssetsAssetEdit, r.handler.AssetQRCodeRevoke))
		mux.HandleFunc("GET /assets/dynamic/new", r.requireAuth(r.handler.AssetNewDynamic))
		mux.HandleFunc("GET /assets/dynamic/{id}/edit", r.requireAuth(r.handler.AssetEditDynamic))
		mux.HandleFunc("POST /assets/dynamic", r.requirePerm(auth.PermAssetsAssetCreate, r.handler.AssetCreateDynamic))
		mux.HandleFunc("POST /assets/dynamic/{id}", r.requirePerm(auth.PermAssetsAssetEdit, r.handler.AssetUpdateDynamic))

		// Events
		mux.HandleFunc("GET /events", r.requireAuth(r.handler.EventsList))

		// Sensors
		mux.HandleFunc("GET /sensors", r.requireAuth(r.handler.SensorsList))
		mux.HandleFunc("GET /sensors/new", r.requirePerm(auth.PermAssetsAssetCreate, r.handler.SensorNew))
		mux.HandleFunc("POST /sensors", r.requirePerm(auth.PermAssetsAssetCreate, r.handler.SensorCreate))
		mux.HandleFunc("GET /sensors/{id}", r.requireAuth(r.handler.SensorDetail))
		mux.HandleFunc("POST /sensors/{id}/delete", r.requirePerm(auth.PermAssetsAssetCreate, r.handler.SensorDelete))
		mux.HandleFunc("GET /sensors/{id}/bind", r.requirePerm(auth.PermAssetsAssetCreate, r.handler.SensorBindForm))
		mux.HandleFunc("POST /sensors/{id}/bind", r.requirePerm(auth.PermAssetsAssetCreate, r.handler.SensorBind))
		mux.HandleFunc("POST /sensors/{id}/unbind", r.requirePerm(auth.PermAssetsAssetCreate, r.handler.SensorUnbind))
		// Alerts
		mux.HandleFunc("GET /alerts", r.requireAuth(r.handler.AlertsList))
		mux.HandleFunc("GET /alerts/{id}", r.requireAuth(r.handler.AlertDetail))
		mux.HandleFunc("POST /alerts/{id}/acknowledge", r.requireAuth(r.handler.AlertAcknowledge))
		mux.HandleFunc("POST /alerts/{id}/resolve", r.requireAuth(r.handler.AlertResolve))
		// Map
		mux.HandleFunc("GET /map", r.requireAuth(r.handler.MapView))
	}

	// FSM Editor, Rules, Asset Types, master tables (SEAM_ENABLE_ADMIN=true)
	if r.config.EnableAdmin {
		// Rules Editor (Rete.js node graph)

		// FSM Editor (canvas)
		mux.HandleFunc("GET /fsm/editor", r.requirePerm(auth.PermAssetsRuleCreate, r.handler.FSMEditorNew))
		mux.HandleFunc("GET /fsm/editor/{id}", r.requireAuth(r.handler.FSMEditorEdit))
		mux.HandleFunc("POST /fsm/export/png", r.requireAuthRawBody(r.handler.FSMExportPNG))

		// Rules
		mux.HandleFunc("GET /rules", r.requireAuth(r.handler.RulesList))
		mux.HandleFunc("GET /rules/new", r.requirePerm(auth.PermAssetsRuleCreate, r.handler.RuleNew))
		mux.HandleFunc("POST /rules", r.requirePerm(auth.PermAssetsRuleCreate, r.handler.RuleCreate))
		mux.HandleFunc("GET /rules/{id}", r.requireAuth(r.handler.RuleEdit))
		mux.HandleFunc("POST /rules/{id}", r.requirePerm(auth.PermAssetsRuleEdit, r.handler.RuleUpdate))

		// Asset Types
		mux.HandleFunc("GET /asset-types", r.requireAuth(r.handler.AssetTypesList))
		mux.HandleFunc("GET /asset-types/new", r.requireAuth(r.handler.AssetTypeNew))
		mux.HandleFunc("POST /asset-types", r.requireAuth(r.handler.AssetTypeCreate))
		mux.HandleFunc("GET /asset-types/{id}", r.requireAuth(r.handler.AssetTypeDetail))
		mux.HandleFunc("GET /asset-types/{id}/summary", r.requireAuth(r.handler.AssetTypeSummary))
		mux.HandleFunc("GET /asset-types/{id}/edit", r.requireAuth(r.handler.AssetTypeEdit))
		mux.HandleFunc("POST /asset-types/{id}", r.requireAuth(r.handler.AssetTypeUpdate))
		mux.HandleFunc("POST /asset-types/{id}/delete", r.requireAuth(r.handler.AssetTypeDelete))

		// Master tables — reference data managed by admins/managers
		mux.HandleFunc("GET /regions", r.requireAuth(r.handler.RegionesList))
		mux.HandleFunc("GET /regions/new", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.RegionNew))
		mux.HandleFunc("POST /regions", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.RegionCreate))
		mux.HandleFunc("GET /regions/{id}", r.requireAuth(r.handler.RegionEdit))
		mux.HandleFunc("GET /regions/{id}/edit", r.requireAuth(r.handler.RegionEdit))
		mux.HandleFunc("POST /regions/{id}", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.RegionUpdate))
		mux.HandleFunc("POST /regions/{id}/delete", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.RegionDelete))

		mux.HandleFunc("GET /currencies", r.requireAuth(r.handler.MonedasList))
		mux.HandleFunc("GET /currencies/new", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.MonedaNew))
		mux.HandleFunc("POST /currencies", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.MonedaCreate))
		mux.HandleFunc("GET /currencies/{id}", r.requireAuth(r.handler.MonedaEdit))
		mux.HandleFunc("GET /currencies/{id}/edit", r.requireAuth(r.handler.MonedaEdit))
		mux.HandleFunc("POST /currencies/{id}", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.MonedaUpdate))
		mux.HandleFunc("POST /currencies/{id}/delete", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.MonedaDelete))

		mux.HandleFunc("GET /cost-centers", r.requireAuth(r.handler.CentrosCostoList))
		mux.HandleFunc("GET /cost-centers/new", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.CentroCostoNew))
		mux.HandleFunc("POST /cost-centers", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.CentroCostoCreate))
		mux.HandleFunc("GET /cost-centers/{id}", r.requireAuth(r.handler.CentroCostoEdit))
		mux.HandleFunc("GET /cost-centers/{id}/summary", r.requireAuth(r.handler.CentroCostoSummary))
		mux.HandleFunc("GET /cost-centers/{id}/edit", r.requireAuth(r.handler.CentroCostoEdit))
		mux.HandleFunc("POST /cost-centers/{id}", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.CentroCostoUpdate))
		mux.HandleFunc("POST /cost-centers/{id}/delete", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.CentroCostoDelete))

		mux.HandleFunc("GET /brands", r.requireAuth(r.handler.MarcasList))
		mux.HandleFunc("GET /brands/new", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.MarcaNew))
		mux.HandleFunc("POST /brands", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.MarcaCreate))
		mux.HandleFunc("GET /brands/{id}", r.requireAuth(r.handler.MarcaEdit))
		mux.HandleFunc("GET /brands/{id}/summary", r.requireAuth(r.handler.MarcaSummary))
		mux.HandleFunc("GET /brands/{id}/edit", r.requireAuth(r.handler.MarcaEdit))
		mux.HandleFunc("POST /brands/{id}", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.MarcaUpdate))
		mux.HandleFunc("POST /brands/{id}/delete", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.MarcaDelete))

		mux.HandleFunc("GET /custodians", r.requireAuth(r.handler.CustodiosList))
		mux.HandleFunc("GET /custodians/new", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.CustodioNew))
		mux.HandleFunc("POST /custodians", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.CustodioCreate))
		mux.HandleFunc("GET /custodians/{id}", r.requireAuth(r.handler.CustodioEdit))
		mux.HandleFunc("GET /custodians/{id}/summary", r.requireAuth(r.handler.CustodioSummary))
		mux.HandleFunc("GET /custodians/{id}/edit", r.requireAuth(r.handler.CustodioEdit))
		mux.HandleFunc("POST /custodians/{id}", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.CustodioUpdate))
		mux.HandleFunc("POST /custodians/{id}/delete", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.CustodioDelete))

		mux.HandleFunc("GET /alert-types", r.requireAuth(r.handler.AlertasMasterList))
		mux.HandleFunc("GET /alert-types/new", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.AlertaMasterNew))
		mux.HandleFunc("POST /alert-types", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.AlertaMasterCreate))
		mux.HandleFunc("GET /alert-types/{id}", r.requireAuth(r.handler.AlertaMasterEdit))
		mux.HandleFunc("GET /alert-types/{id}/edit", r.requireAuth(r.handler.AlertaMasterEdit))
		mux.HandleFunc("POST /alert-types/{id}", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.AlertaMasterUpdate))
		mux.HandleFunc("POST /alert-types/{id}/delete", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.AlertaMasterDelete))

		mux.HandleFunc("GET /properties", r.requireAuth(r.handler.PropDefsList))
		mux.HandleFunc("GET /properties/new", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.PropDefNew))
		mux.HandleFunc("POST /properties", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.PropDefCreate))
		mux.HandleFunc("GET /properties/{id}", r.requireAuth(r.handler.PropDefEdit))
		mux.HandleFunc("GET /properties/{id}/edit", r.requireAuth(r.handler.PropDefEdit))
		mux.HandleFunc("POST /properties/{id}", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.PropDefUpdate))
		mux.HandleFunc("POST /properties/{id}/delete", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.PropDefDelete))

		mux.HandleFunc("GET /models", r.requireAuth(r.handler.ModelosList))
		mux.HandleFunc("GET /models/new", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.ModeloNew))
		mux.HandleFunc("POST /models", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.ModeloCreate))
		mux.HandleFunc("GET /models/{id}", r.requireAuth(r.handler.ModeloEdit))
		mux.HandleFunc("GET /models/{id}/summary", r.requireAuth(r.handler.ModeloSummary))
		mux.HandleFunc("GET /models/{id}/edit", r.requireAuth(r.handler.ModeloEdit))
		mux.HandleFunc("POST /models/{id}", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.ModeloUpdate))
		mux.HandleFunc("POST /models/{id}/delete", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.ModeloDelete))

		mux.HandleFunc("GET /locations", r.requireAuth(r.handler.UbicacionesList))
		mux.HandleFunc("GET /locations/new", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.UbicacionNew))
		mux.HandleFunc("POST /locations", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.UbicacionCreate))
		mux.HandleFunc("GET /locations/{id}", r.requireAuth(r.handler.UbicacionEdit))
		mux.HandleFunc("GET /locations/{id}/summary", r.requireAuth(r.handler.UbicacionSummary))
		mux.HandleFunc("GET /locations/{id}/edit", r.requireAuth(r.handler.UbicacionEdit))
		mux.HandleFunc("POST /locations/{id}", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.UbicacionUpdate))
		mux.HandleFunc("POST /locations/{id}/delete", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.UbicacionDelete))

		mux.HandleFunc("GET /properties/associations", r.requireAuth(r.handler.PropAssocsList))
		mux.HandleFunc("GET /properties/associations/new", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.PropAssocNew))
		mux.HandleFunc("POST /properties/associations", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.PropAssocCreate))
		mux.HandleFunc("POST /properties/associations/{id}/delete", r.requirePerm(auth.PermAssetsMasterEdit, r.handler.PropAssocDelete))
	}
}

// MountWorkOrdersRoutes registers the Work Orders playground module's
// routes (2026-08-04, T-67's follow-on) -- gated behind
// r.config.EnableWorkOrders, default false (see internal/config's own
// field doc comment for why). All routes use requireAuth only, no
// requirePerm -- the outer EnableWorkOrders gate is this playground's
// real safety boundary, not a granular per-action permission scheme,
// which T-11's own real scoping would need to design properly.
func (r *Router) MountWorkOrdersRoutes(mux *http.ServeMux) {
	if !r.config.EnableWorkOrders && !r.config.moduleSwitches() {
		return
	}
	// on at startup the handlers are mounted as they are; otherwise each
	// request is asked whether the module is on for its tenant (DEV-27)
	gate := r.moduleGate(r.config.EnableWorkOrders, features.ModuleWorkOrders)
	authed := func(next http.HandlerFunc) http.HandlerFunc { return r.requireAuth(gate(next)) }

	//Templates
	mux.HandleFunc("GET /workorders-templates", authed(r.handler.WorkOrdersTemplatesList))
	mux.HandleFunc("GET /workorders-templates/new", authed(r.handler.WorkOrdersTemplatesNew))
	mux.HandleFunc("POST /workorders-templates/validate-step-1", authed(r.handler.WorkOrdersTemplatesValidateStep1))
	mux.HandleFunc("POST /workorders-templates/step-2", authed(r.handler.WorkOrdersTemplatesStep2))
	mux.HandleFunc("POST /workorders-templates", authed(r.handler.WorkOrderTemplateCreate))
	mux.HandleFunc("GET /workorders-templates/{id}", authed(r.handler.WorkOrderTemplateDetail))

	//Work Orders
	mux.HandleFunc("GET /workorders", authed(r.handler.WorkOrdersList))
	mux.HandleFunc("GET /workorders/backlog", authed(r.handler.WorkOrdersBacklog))
	mux.HandleFunc("GET /workorders/template-picker", authed(r.handler.WorkOrderTemplatePicker))
	mux.HandleFunc("GET /workorders/new", authed(r.handler.WorkOrderNew))
	mux.HandleFunc("POST /workorders", authed(r.handler.WorkOrderCreate))
	mux.HandleFunc("GET /workorders/{id}", authed(r.handler.WorkOrderDetail))
	mux.HandleFunc("POST /workorders/{id}/claim", authed(r.handler.WorkOrderClaim))
	mux.HandleFunc("POST /workorders/{id}/start", authed(r.handler.WorkOrderStart))
	mux.HandleFunc("POST /workorders/{id}/complete", authed(r.handler.WorkOrderComplete))
	mux.HandleFunc("GET /workorders/direct-assignment", authed(r.handler.WorkOrderDirectAssignment))

	//Crews
	mux.HandleFunc("GET /crews/new", authed(r.handler.CrewNew))
	mux.HandleFunc("POST /crews", authed(r.handler.CrewCreate))
	mux.HandleFunc("GET /crews", authed(r.handler.CrewList))
	mux.HandleFunc("GET /crew/{id}", authed(r.handler.CrewDetail))
	mux.HandleFunc("POST /crew/{id}/delete", authed(r.handler.CrewDelete))
	mux.HandleFunc("GET /crews/dynamic/{id}/edit", authed(r.handler.CrewEditDynamic))
	mux.HandleFunc("POST /crews/dynamic/{id}", authed(r.handler.CrewUpdateDynamic))

	//Outsourced
	mux.HandleFunc("GET /outsourced/new", authed(r.handler.OutsourcedNew))
	mux.HandleFunc("POST /outsourced", authed(r.handler.OutsourcedCreate))
	mux.HandleFunc("GET /outsourced", authed(r.handler.OutsourcedList))
	mux.HandleFunc("GET /outsourced/{id}", authed(r.handler.OutsourcedDetail))
	mux.HandleFunc("POST /outsourced/{id}/account", r.requirePerm(auth.PermAdministrationUserEdit, gate(r.handler.OutsourcedLinkAccount)))
	mux.HandleFunc("POST /outsourced/{id}/delete", authed(r.handler.OutsourcedDelete))
	mux.HandleFunc("GET /outsourced/dynamic/{id}/edit", authed(r.handler.OutsourcedEditDynamic))
	mux.HandleFunc("POST /outsourced/dynamic/{id}/update", authed(r.handler.OutsourcedUpdateDynamic))

	//Warehouses
	mux.HandleFunc("GET /warehouses/new", authed(r.handler.WarehouseNew))
	mux.HandleFunc("POST /warehouses", authed(r.handler.WarehouseCreate))
	mux.HandleFunc("GET /warehouses", authed(r.handler.WarehouseList))
	mux.HandleFunc("GET /warehouse/{id}", authed(r.handler.WarehouseDetail))
	mux.HandleFunc("GET /warehouses/{id}/movements/new", authed(r.handler.WarehouseMovementNew))
	mux.HandleFunc("POST /warehouses/{id}/movements", authed(r.handler.WarehouseMovementCreate))
	mux.HandleFunc("GET /warehouses/{id}/edit", authed(r.handler.WarehouseEdit))
	mux.HandleFunc("POST /warehouses/{id}", authed(r.handler.WarehouseUpdate))
	mux.HandleFunc("POST /warehouses/{id}/enabled", authed(r.handler.WarehouseSetEnabled))
	mux.HandleFunc("GET /warehouses/{id}/empty", authed(r.handler.WarehouseEmptyNew))
	mux.HandleFunc("POST /warehouses/{id}/empty", authed(r.handler.WarehouseEmptyCreate))

	//WOActions
	mux.HandleFunc("GET /wo-actions", authed(r.handler.WOActionList))
	mux.HandleFunc("GET /wo-actions/new", authed(r.handler.WOActionNew))
	mux.HandleFunc("POST /wo-actions", authed(r.handler.WOActionCreate))
	mux.HandleFunc("GET /wo-actions/{id}", authed(r.handler.WOActionDetail))
	mux.HandleFunc("GET /wo-actions/{id}/edit", authed(r.handler.WOActionEdit))
	mux.HandleFunc("POST /wo-actions/{id}", authed(r.handler.WOActionUpdate))
	mux.HandleFunc("POST /wo-actions/{id}/enabled", authed(r.handler.WOActionSetEnabled))
	mux.HandleFunc("POST /wo-actions/{id}/delete", authed(r.handler.WOActionDelete))

	//Consumable
	mux.HandleFunc("GET /consumable/new", authed(r.handler.ConsumableNew))
	mux.HandleFunc("POST /consumable", authed(r.handler.ConsumableCreate))
	mux.HandleFunc("GET /consumable", authed(r.handler.ConsumableList))
	mux.HandleFunc("GET /consumable/{id}", authed(r.handler.ConsumableDetail))

	mux.HandleFunc("GET /consumable-types", authed(r.handler.ConsumableTypesList))
	mux.HandleFunc("GET /consumable-types/new", authed(r.handler.ConsumableTypeNew))
	mux.HandleFunc("POST /consumable-types", authed(r.handler.ConsumableTypeCreate))
	mux.HandleFunc("GET /consumable-types/{id}", authed(r.handler.ConsumableTypeDetail))

	//Unit
	mux.HandleFunc("GET /unit/new", authed(r.handler.UnitNew))
	mux.HandleFunc("POST /unit", authed(r.handler.UnitCreate))
	mux.HandleFunc("GET /unit", authed(r.handler.UnitList))
	mux.HandleFunc("GET /unit/{id}", authed(r.handler.UnitDetail))

}

// MountDemoRoutes registers the Demo module's routes, gated behind
// r.config.EnableDemo (default false, see internal/config's field doc).
// requireAuth only: any authenticated role may open the demo once enabled.
func (r *Router) MountDemoRoutes(mux *http.ServeMux) {
	if !r.config.EnableDemo && !r.config.moduleSwitches() {
		return
	}
	// on at startup the handlers are mounted as they are; otherwise each
	// request is asked whether the module is on for its tenant (DEV-27)
	gate := r.moduleGate(r.config.EnableDemo, features.ModuleDemo)
	authed := func(next http.HandlerFunc) http.HandlerFunc { return r.requireAuth(gate(next)) }
	// the overview, and one page per group of demonstrations (demo.go)
	mux.HandleFunc("GET /demo", authed(r.handler.DemoIndex))
	mux.HandleFunc("GET /demo/{group}", authed(r.handler.DemoGroupPage))
	mux.HandleFunc("POST /demo/echo", authed(r.handler.DemoEcho))
	// Demo records: the only data the demo persists, all in the dedicated
	// demo_templates entity (see demo_templates.go).
	mux.HandleFunc("POST /demo/templates", authed(r.handler.DemoTemplateSave))
	mux.HandleFunc("POST /demo/templates/reset", authed(r.handler.DemoTemplateReset))
	mux.HandleFunc("POST /demo/templates/{id}/delete", authed(r.handler.DemoTemplateDelete))
}

// MountDeveloperRoutes registers the Developer module's routes (campaign
// DEV-7), gated behind r.config.EnableDeveloper (default false, and refused at
// startup unless authentication is enabled). Every route uses requirePerm with
// auth.PermDeveloperAccess, which fails closed: no claims redirects to login,
// and any role without the permission (everyone but the developer) gets 403.
func (r *Router) MountDeveloperRoutes(mux *http.ServeMux) {
	if !r.config.EnableDeveloper {
		return
	}
	guard := func(h http.HandlerFunc) http.HandlerFunc { return r.requireRealPerm(auth.PermDeveloperAccess, h) }
	h := r.handler
	// The Overview is served at /developer (the module's entry point) and at
	// /developer/overview, the path its left-menu item points at.
	mux.HandleFunc("GET /developer", guard(h.DeveloperIndex))
	mux.HandleFunc("GET /developer/overview", guard(h.DeveloperIndex))
	mux.HandleFunc("GET /developer/startup", guard(h.DeveloperStartup))
	mux.HandleFunc("GET /developer/translations", guard(h.DeveloperTranslations))
	mux.HandleFunc("POST /developer/translations/save", guard(h.DeveloperTranslationSave))
	mux.HandleFunc("POST /developer/translations/remove", guard(h.DeveloperTranslationRemove))
	mux.HandleFunc("POST /developer/translations/create", guard(h.DeveloperTranslationCreate))
	mux.HandleFunc("POST /developer/translations/restore", guard(h.DeveloperTranslationRestore))
	mux.HandleFunc("POST /developer/forms/create", guard(h.DeveloperFormCreate))
	mux.HandleFunc("GET /developer/export", guard(h.DeveloperExport))
	mux.HandleFunc("GET /developer/forms", guard(h.DeveloperForms))
	mux.HandleFunc("POST /developer/forms/preview", guard(h.DeveloperFormPreview))
	mux.HandleFunc("POST /developer/forms/save", guard(h.DeveloperFormSave))
	mux.HandleFunc("POST /developer/forms/remove", guard(h.DeveloperFormRemove))
	mux.HandleFunc("POST /developer/forms/restore", guard(h.DeveloperFormRestore))
	mux.HandleFunc("GET /developer/forms/try", guard(h.DeveloperFormTry))
	mux.HandleFunc("POST /developer/forms/try", guard(h.DeveloperFormTry))
	// a page for each form made with the tools, for the tenant's users
	// (developer_custom.go); only where the deployment asks for it
	if r.config.customPagesOn() {
		mux.HandleFunc("GET /forms/{module}/{name}", r.requireAuth(h.CustomFormPage))
		mux.HandleFunc("POST /forms/{module}/{name}", r.requireAuth(h.CustomFormPage))
	}
	mux.HandleFunc("POST /developer/view-as", guard(h.DeveloperViewAs))
	mux.HandleFunc("GET /developer/designer", guard(h.DeveloperDesigner))
	mux.HandleFunc("POST /developer/designer", guard(h.DeveloperDesigner))
	mux.HandleFunc("GET /developer/view-as/exit", guard(h.DeveloperViewAsExit))
	mux.HandleFunc("POST /developer/flags", guard(h.DeveloperFlagSet))
	mux.HandleFunc("POST /developer/reload", guard(h.DeveloperReload))
}

// MountIssuesRoutes registers the routes of the Issues module
// (docs/ISSUES_MODULE.md), gated the way MountWorkOrdersRoutes gates its
// own: behind r.config.EnableIssues, default false. With the flag off and
// no module switches nothing is mounted; where a module can be switched on
// at runtime the routes are mounted and each request is asked whether the
// module is on for its tenant (DEV-27), and gets a 404 where it is not.
//
// Inside that gate the issue routes ask for a signed-in user and nothing
// more; who may act on an issue is decided in the handlers, by role rank
// and by who the issue belongs to (canActOnIssue, canAssignIssues,
// canManageIssueGroups). Permissions of its own are T-125.
func (r *Router) MountIssuesRoutes(mux *http.ServeMux) {
	if !r.config.EnableIssues && !r.config.moduleSwitches() {
		return
	}
	// on at startup the handlers are mounted as they are; otherwise each
	// request is asked whether the module is on for its tenant (DEV-27)
	gate := r.moduleGate(r.config.EnableIssues, features.ModuleIssues)
	authed := func(next http.HandlerFunc) http.HandlerFunc { return r.requireAuth(gate(next)) }
	permed := func(perm auth.Permission, next http.HandlerFunc) http.HandlerFunc {
		return r.requirePerm(perm, gate(next))
	}

	mux.HandleFunc("GET /issues", authed(r.handler.IssuesList))
	mux.HandleFunc("GET /issues/new", authed(r.handler.IssueNew))
	mux.HandleFunc("GET /issues/sla-estimate", authed(r.handler.IssueSLAEstimate))
	mux.HandleFunc("POST /issues", authed(r.handler.IssueCreate))
	mux.HandleFunc("GET /issues/{id}", authed(r.handler.IssueDetail))
	mux.HandleFunc("POST /issues/{id}/start", authed(r.handler.IssueStart))
	mux.HandleFunc("POST /issues/{id}/resolve", authed(r.handler.IssueResolve))
	mux.HandleFunc("POST /issues/{id}/close", authed(r.handler.IssueClose))
	mux.HandleFunc("GET /issues/{id}/reject", authed(r.handler.IssueRejectFormPage))
	mux.HandleFunc("POST /issues/{id}/reject", authed(r.handler.IssueReject))
	mux.HandleFunc("GET /issues/group", authed(r.handler.IssueGroupFormPage))
	mux.HandleFunc("POST /issues/group", authed(r.handler.IssueGroup))
	mux.HandleFunc("POST /issues/{id}/ungroup", authed(r.handler.IssueUngroup))
	mux.HandleFunc("GET /issues/{id}/assign", authed(r.handler.IssueAssignFormPage))
	mux.HandleFunc("POST /issues/{id}/assign", authed(r.handler.IssueAssign))

	// Issue templates (2026-10-10) -- versioned field sets assignable to
	// asset types. The HTMX partial that swaps fields into the create form
	// is open to any authenticated user (it only reads the assignment);
	// managing templates is gated to admin/manager via system.settings.edit.
	// A separate /issue-templates namespace, not /issues/templates/*: the
	// latter would overlap the issue lifecycle routes (e.g.
	// /issues/{id}/reject and /issues/templates/{key} both match
	// /issues/templates/reject) and Go 1.22 ServeMux panics on that.
	mux.HandleFunc("GET /issue-qr-codes", permed(auth.PermSettingsEdit, r.handler.IssueQRCodesList))
	mux.HandleFunc("POST /issue-qr-codes/revoke", permed(auth.PermSettingsEdit, r.handler.IssueQRCodesBulkRevoke))
	mux.HandleFunc("POST /issue-qr-codes/{id}/revoke", permed(auth.PermSettingsEdit, r.handler.IssueQRCodeAdminRevoke))

	mux.HandleFunc("GET /issue-templates/fields", authed(r.handler.IssueTemplateFields))
	mux.HandleFunc("GET /issue-templates", permed(auth.PermSettingsEdit, r.handler.IssueTemplatesList))
	mux.HandleFunc("GET /issue-templates/new", permed(auth.PermSettingsEdit, r.handler.IssueTemplateNew))
	mux.HandleFunc("POST /issue-templates", permed(auth.PermSettingsEdit, r.handler.IssueTemplateCreate))
	mux.HandleFunc("GET /issue-templates/{key}", permed(auth.PermSettingsEdit, r.handler.IssueTemplateDetail))
	mux.HandleFunc("GET /issue-templates/{key}/edit", permed(auth.PermSettingsEdit, r.handler.IssueTemplateEdit))
	mux.HandleFunc("POST /issue-templates/{key}/edit", permed(auth.PermSettingsEdit, r.handler.IssueTemplateUpdate))
	mux.HandleFunc("POST /issue-templates/{key}/assign", permed(auth.PermSettingsEdit, r.handler.IssueTemplateAssign))
	mux.HandleFunc("POST /issue-templates/{key}/delete", permed(auth.PermSettingsEdit, r.handler.IssueTemplateDelete))
}

// MountDocumentsRoutes registers the Documents module's routes -- just
// the dashboard landing page (GET /documents) -- onto mux.
//
// This is MOD-7's application of the same MountXRoutes pattern MOD-5/
// MOD-6 established, at the opposite end of the size spectrum: 1 route,
// not 13 or 98. Documents has no other views yet (DOCUMENTS_MODULE_
// SPEC.md v0.5 is a design document, T-04 to build it is still open),
// so there is nothing else to register -- MODULE_SPEC.md §7.2's own
// framing, "registers only the dashboard landing route." Unconditional,
// same reasoning as the System dashboard and Profile: a read-only
// landing page, not gated behind EnableAdmin/EnableOperational.
func (r *Router) MountDocumentsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /documents", r.requireAuth(r.handler.DocumentsDashboard))
}

// CombinedRouter combines API and UI routers.
type CombinedRouter struct {
	api    http.Handler
	ui     http.Handler
	static http.Handler
}

// NewCombinedRouter creates a router that serves both API and UI.
func NewCombinedRouter(api, ui http.Handler) *CombinedRouter {
	return &CombinedRouter{
		api:    api,
		ui:     ui,
		static: http.FileServer(http.Dir("web/static")),
	}
}

// ServeHTTP routes requests to API or UI based on path.
func (r *CombinedRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path

	// API routes
	if len(path) >= 8 && path[:8] == "/api/v1/" {
		r.api.ServeHTTP(w, req)
		return
	}

	// Static files
	if len(path) >= 8 && path[:8] == "/static/" {
		http.StripPrefix("/static/", r.static).ServeHTTP(w, req)
		return
	}

	// UI routes
	r.ui.ServeHTTP(w, req)
}
