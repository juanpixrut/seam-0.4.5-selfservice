package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/i18n"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

// This file tests the real change MOD-5 (and its 2026-08-04 follow-on
// split into Account/Settings/Administration) made: each MountXRoutes
// method registers its module's routes onto an EXTERNAL mux (the
// server's shared top-level one), not r.mux (Router's own private mux,
// still used by every route registerRoutes() itself registers).
// Existing handler-level tests (audit_wiring_test.go, validation_handler_
// test.go, etc.) call UIHandler methods directly and so never exercise
// routing at all -- they would stay green even if every route this file
// covers were silently unregistered. This file exists specifically to
// close that gap: it proves each route is actually reachable through the
// real mux, wrapped in the SAME requireAuth/requirePerm middleware
// registerRoutes()'s inline calls used to apply directly.
//
// Deliberately tests only as far as the middleware boundary (redirect-to-
// login for an invalid session, 403 for an authenticated-but-unauthorised
// role, 404 for a route EnableAdmin has disabled) -- never a full
// authenticated success path, which would need a real userService/
// dashboardEngine and duplicate what handler-level tests already cover.
// requireAuth/requirePerm run and make their redirect/403/next-handler
// decision BEFORE the real handler is ever invoked, so this boundary is
// sufficient to prove the wiring without needing those dependencies.

// buildRoutingTestRouter builds a real *Router (via NewRouter, so
// registerRoutes() runs exactly as it does in production) plus a real
// *auth.SessionManager for minting valid session cookies. userService,
// csrfManager, loginLimiter, auditLogger, and resetManager are nil --
// safe here because every assertion in this file stops at the requireAuth/
// requirePerm boundary, before any of those would be touched.
func buildRoutingTestRouter(t *testing.T, enableAdmin, enableOperational bool) (*Router, *auth.SessionManager) {
	t.Helper()
	client := xolu.New("http://127.0.0.1:1") // unreachable; never dialed by this file's assertions
	translator, err := i18n.New("en")
	if err != nil {
		t.Fatalf("i18n.New: %v", err)
	}
	sessionMgr := auth.NewSessionManager(auth.SessionConfig{Secret: "test-secret-routing"})
	cfg := Config{
		AppName:           "Test",
		Version:           "0.0.0",
		Locale:            "en",
		EnableAdmin:       enableAdmin,
		EnableOperational: enableOperational,
	}
	router := NewRouter(client, cfg, translator, nil, sessionMgr, nil, nil, nil, nil)
	return router, sessionMgr
}

// mountedMux builds the same split-mux wiring internal/server/server.go
// itself constructs: MountAccountRoutes, MountSettingsRoutes,
// MountAdministrationRoutes (all three split 2026-08-04 out of the
// former single MountSystemRoutes, per the tech team's decision -- Account/
// Settings/Administration are separate modules, not one), MountAssetRoutes
// (MOD-6), and MountDocumentsRoutes (MOD-7) onto the outer mux, router's
// own "/" catch-all on the same outer mux -- so a request for a route
// none of the MountXRoutes methods cover still reaches router via
// delegation, exactly as it does in production. Registering all five on
// the same mux here is itself a check: a duplicate pattern between any
// of them would panic at registration time, not silently misroute.
func mountedMux(router *Router) *http.ServeMux {
	mux := http.NewServeMux()
	router.MountAccountRoutes(mux)
	router.MountSettingsRoutes(mux)
	router.MountAdministrationRoutes(mux)
	router.MountAssetRoutes(mux)
	router.MountDocumentsRoutes(mux)
	mux.Handle("/", router)
	return mux
}

// sessionCookie mints a real, valid session cookie for role via
// sessionMgr, so a request carrying it passes requireAuth's session
// check and reaches requirePerm's permission check (or the real handler,
// for routes with no permission gate).
func sessionCookie(t *testing.T, sessionMgr *auth.SessionManager, role string) *http.Cookie {
	t.Helper()
	rec := httptest.NewRecorder()
	user := &auth.User{ID: 1, TenantID: 1, Email: "test@example.com", Name: "Test User", Role: role, Active: true}
	if err := sessionMgr.CreateSession(rec, user); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("CreateSession set no cookie")
	}
	return cookies[0]
}

func TestMountedMux_NoDuplicateRegistrationPanic(t *testing.T) {
	router, _ := buildRoutingTestRouter(t, true, true)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("mounting all five modules + registerRoutes panicked (likely a duplicate pattern across the muxes, or a leftover inline registration): %v", r)
		}
	}()
	mountedMux(router)
}

func TestMountAccountRoutes_ProfileReachableAndAuthGated(t *testing.T) {
	router, _ := buildRoutingTestRouter(t, false, false)
	mux := mountedMux(router)

	// Profile has no permission gate and is unconditional (the one
	// module whose HasAccess is always true, unchanged by the
	// 2026-08-04 split) -- unauthenticated should redirect to /login,
	// not 404. A 404 here would mean the route was never registered at
	// all.
	req := httptest.NewRequest("GET", "/profile", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("GET /profile unauthenticated: got %d, want %d (redirect to login) -- route may not be registered", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc == "" || loc[:6] != "/login" {
		t.Errorf("GET /profile unauthenticated: Location = %q, want a /login redirect", loc)
	}
}

// TestSystemDashboard_NoLongerRegistered confirms /system is genuinely
// gone, not just unreachable through mountedMux by omission. MOD-5
// built a "System" module with its own dashboard (three audit_log
// sections); the 2026-08-04 split retired both the module and the
// route deliberately -- there's no module left for /system to be the
// landing page of, and each of the three replacement modules (Account/
// Settings/Administration) lands directly on its own real content
// instead. A 404 here is the intended outcome, not a regression to
// investigate.
func TestSystemDashboard_NoLongerRegistered(t *testing.T) {
	router, sessionMgr := buildRoutingTestRouter(t, true, false)
	mux := mountedMux(router)
	cookie := sessionCookie(t, sessionMgr, "admin")
	req := httptest.NewRequest("GET", "/system", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /system: got %d, want %d -- /system was deliberately retired 2026-08-04, this route should no longer exist", rec.Code, http.StatusNotFound)
	}
}

func TestMountSettingsRoutes_GatedByEnableAdmin(t *testing.T) {
	t.Run("EnableAdmin=true: registered, redirects unauthenticated to login", func(t *testing.T) {
		router, _ := buildRoutingTestRouter(t, true, false)
		mux := mountedMux(router)
		req := httptest.NewRequest("GET", "/settings", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusSeeOther {
			t.Errorf("GET /settings (EnableAdmin=true) unauthenticated: got %d, want %d", rec.Code, http.StatusSeeOther)
		}
	})

	t.Run("EnableAdmin=false: not registered at all", func(t *testing.T) {
		router, _ := buildRoutingTestRouter(t, false, false)
		mux := mountedMux(router)
		req := httptest.NewRequest("GET", "/settings", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET /settings (EnableAdmin=false): got %d, want %d (route should not exist)", rec.Code, http.StatusNotFound)
		}
	})
}

func TestMountAdministrationRoutes_GatedByEnableAdminAndPermission(t *testing.T) {
	t.Run("EnableAdmin=false: not registered at all", func(t *testing.T) {
		router, _ := buildRoutingTestRouter(t, false, false)
		mux := mountedMux(router)
		req := httptest.NewRequest("GET", "/admin/users", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET /admin/users (EnableAdmin=false): got %d, want %d", rec.Code, http.StatusNotFound)
		}
	})

	t.Run("EnableAdmin=true, unauthenticated: redirects to login", func(t *testing.T) {
		router, _ := buildRoutingTestRouter(t, true, false)
		mux := mountedMux(router)
		req := httptest.NewRequest("GET", "/admin/users", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusSeeOther {
			t.Errorf("GET /admin/users unauthenticated: got %d, want %d", rec.Code, http.StatusSeeOther)
		}
	})

	t.Run("EnableAdmin=true, authenticated but lacking administration.user.view: 403", func(t *testing.T) {
		router, sessionMgr := buildRoutingTestRouter(t, true, false)
		mux := mountedMux(router)
		cookie := sessionCookie(t, sessionMgr, "viewer") // viewer holds no administration.user.* permission (auth.go rolePermissions)
		req := httptest.NewRequest("GET", "/admin/users", nil)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("GET /admin/users as viewer: got %d, want %d (requirePerm should block, not just requireAuth)", rec.Code, http.StatusForbidden)
		}
	})
}

func TestMountedMux_PreexistingRouteStillReachableViaCatchAll(t *testing.T) {
	// GET /login is registered inline in registerRoutes() on r.mux,
	// reached only via the outer mux's "/" delegation to
	// router.ServeHTTP. Proves the split between the outer mux (every
	// MountXRoutes method's registrations) and r.mux (everything still
	// inline -- login, password reset, QR scan, static files) doesn't
	// break routes that were never part of any module. No requireAuth
	// wrapper on a public route, so the assertion is "reached the real
	// handler" (200), not a login redirect.
	router, _ := buildRoutingTestRouter(t, false, true)
	mux := mountedMux(router)
	req := httptest.NewRequest("GET", "/login", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /login: got %d, want %d -- \"/\" catch-all delegation may be broken", rec.Code, http.StatusOK)
	}
}

// The tests below cover MountAssetRoutes (MOD-6) -- the same class of
// wiring test the Account/Settings/Administration split got, applied to
// the far larger surface (98 routes vs 5 each): dashboard reachability,
// EnableOperational gating, EnableAdmin gating (covering both a
// non-master route, rules, and a master-table route, regions, since the
// two use different permission constants), and requirePerm genuinely
// blocking an under-privileged role, not just requireAuth.

func TestMountAssetRoutes_DashboardReachableAndAuthGated(t *testing.T) {
	// Dashboard is unconditional -- no EnableOperational/EnableAdmin
	// dependency, matching its prior placement outside any feature-flag
	// gate in registerRoutes().
	router, _ := buildRoutingTestRouter(t, false, false)
	mux := mountedMux(router)
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("GET / unauthenticated: got %d, want %d (redirect to login) -- route may not be registered", rec.Code, http.StatusSeeOther)
	}
}

func TestMountAssetRoutes_AssetsGatedByEnableOperational(t *testing.T) {
	t.Run("EnableOperational=true: registered, redirects unauthenticated to login", func(t *testing.T) {
		router, _ := buildRoutingTestRouter(t, false, true)
		mux := mountedMux(router)
		req := httptest.NewRequest("GET", "/assets", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusSeeOther {
			t.Errorf("GET /assets (EnableOperational=true) unauthenticated: got %d, want %d", rec.Code, http.StatusSeeOther)
		}
	})

	t.Run("EnableOperational=false: not registered at all", func(t *testing.T) {
		router, _ := buildRoutingTestRouter(t, false, false)
		mux := mountedMux(router)
		req := httptest.NewRequest("GET", "/assets", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET /assets (EnableOperational=false): got %d, want %d (route should not exist)", rec.Code, http.StatusNotFound)
		}
	})
}

func TestMountAssetRoutes_AssetCreateRequiresPermission(t *testing.T) {
	router, sessionMgr := buildRoutingTestRouter(t, false, true)
	mux := mountedMux(router)
	cookie := sessionCookie(t, sessionMgr, "viewer") // viewer holds assets.asset.view but not assets.asset.create (auth.go rolePermissions)
	req := httptest.NewRequest("POST", "/assets/dynamic", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("POST /assets/dynamic as viewer: got %d, want %d (requirePerm should block, not just requireAuth)", rec.Code, http.StatusForbidden)
	}
}

func TestMountAssetRoutes_RulesGatedByEnableAdmin(t *testing.T) {
	t.Run("EnableAdmin=true: registered, redirects unauthenticated to login", func(t *testing.T) {
		router, _ := buildRoutingTestRouter(t, true, false)
		mux := mountedMux(router)
		req := httptest.NewRequest("GET", "/rules", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusSeeOther {
			t.Errorf("GET /rules (EnableAdmin=true) unauthenticated: got %d, want %d", rec.Code, http.StatusSeeOther)
		}
	})

	t.Run("EnableAdmin=false: not registered at all", func(t *testing.T) {
		router, _ := buildRoutingTestRouter(t, false, false)
		mux := mountedMux(router)
		req := httptest.NewRequest("GET", "/rules", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET /rules (EnableAdmin=false): got %d, want %d (route should not exist)", rec.Code, http.StatusNotFound)
		}
	})
}

func TestMountAssetRoutes_MasterTableGatedByEnableAdminAndPermission(t *testing.T) {
	// Regions stands in for all 9 master tables -- same PermAssetsMasterEdit
	// gate, same EnableAdmin block, same handler-registration pattern.
	t.Run("EnableAdmin=false: not registered at all", func(t *testing.T) {
		router, _ := buildRoutingTestRouter(t, false, false)
		mux := mountedMux(router)
		req := httptest.NewRequest("GET", "/regions", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET /regions (EnableAdmin=false): got %d, want %d", rec.Code, http.StatusNotFound)
		}
	})

	t.Run("EnableAdmin=true, authenticated but lacking assets.master.edit: 403", func(t *testing.T) {
		router, sessionMgr := buildRoutingTestRouter(t, true, false)
		mux := mountedMux(router)
		cookie := sessionCookie(t, sessionMgr, "viewer") // viewer holds no assets.master.* permission
		req := httptest.NewRequest("POST", "/regions", strings.NewReader("description=Test"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("POST /regions as viewer: got %d, want %d (requirePerm should block, not just requireAuth)", rec.Code, http.StatusForbidden)
		}
	})
}

// TestMountDocumentsRoutes_ReachableAndAuthGated covers MountDocumentsRoutes
// (MOD-7) -- the same class of wiring test the Account/Settings/
// Administration split and MountAssetRoutes got, at the opposite end of
// the size spectrum: 1 route, unconditional (no EnableOperational/
// EnableAdmin gate, matching Account's own reasoning -- a read-only
// landing page).
func TestMountDocumentsRoutes_ReachableAndAuthGated(t *testing.T) {
	router, _ := buildRoutingTestRouter(t, false, false)
	mux := mountedMux(router)
	req := httptest.NewRequest("GET", "/documents", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("GET /documents unauthenticated: got %d, want %d (redirect to login) -- route may not be registered", rec.Code, http.StatusSeeOther)
	}
}

// TestMountDocumentsRoutes_RendersRealEmptyMessage guards a real gap
// found while building MOD-7: DASHBOARD_SPEC.md §2.2's empty-dashboard
// rendering path (dashboardengine.renderEmptyDashboard) already existed
// in code from MOD-2, but the i18n key it references,
// "dashboard.empty_message", didn't exist in any locale file -- nothing
// had ever exercised this path against a real "sections": [] schema
// before Documents. Without the key, the translator's own documented
// fallback (internal/i18n/i18n.go) returns the raw key string, so an
// authenticated request would have rendered the literal text
// "dashboard.empty_message" to the user instead of a real message.
func TestMountDocumentsRoutes_RendersRealEmptyMessage(t *testing.T) {
	router, sessionMgr := buildRoutingTestRouter(t, false, false)
	mux := mountedMux(router)
	cookie := sessionCookie(t, sessionMgr, "viewer")
	req := httptest.NewRequest("GET", "/documents", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /documents as viewer: got %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if strings.Contains(body, "dashboard.empty_message") {
		t.Error(`response contains the raw i18n key "dashboard.empty_message" -- the translation is missing, the user would see the literal key text`)
	}
	if !strings.Contains(body, "No data to display in this module at this time.") {
		t.Error("response doesn't contain the real empty-dashboard message")
	}
}

// ============================================================================
// Adversarial tests below: what happens if the same MountXRoutes method
// is (by mistake -- a duplicate registry.MountAll(mux) call, a retry
// loop, a copy-pasted server.go edit) invoked twice against the same
// mux, instead of the intended once-per-startup call.
// ============================================================================

// TestMountAccountRoutes_CalledTwiceOnSameMux_Panics confirms Go's own
// http.ServeMux does exactly what's wanted here: registering the exact
// same pattern twice on the same mux panics, loudly, at the second
// call -- not silently overwritten, not silently ignored. A "mounted
// twice by accident" bug fails at startup, not as a mysterious runtime
// routing inconsistency discovered later.
func TestMountAccountRoutes_CalledTwiceOnSameMux_Panics(t *testing.T) {
	router, _ := buildRoutingTestRouter(t, true, false)
	mux := http.NewServeMux()
	router.MountAccountRoutes(mux) // first call: fine

	defer func() {
		if recover() == nil {
			t.Error("expected the second MountAccountRoutes call on the same mux to panic (duplicate pattern registration), got none")
		}
	}()
	router.MountAccountRoutes(mux) // second call on the SAME mux: must panic
}

// TestMountSettingsRoutes_CalledTwiceOnSameMux_Panics is the same
// confirmation for Settings specifically -- its own module, its own
// permission namespace (settings.*), split out 2026-08-04.
func TestMountSettingsRoutes_CalledTwiceOnSameMux_Panics(t *testing.T) {
	router, _ := buildRoutingTestRouter(t, true, false)
	mux := http.NewServeMux()
	router.MountSettingsRoutes(mux)

	defer func() {
		if recover() == nil {
			t.Error("expected the second MountSettingsRoutes call on the same mux to panic, got none")
		}
	}()
	router.MountSettingsRoutes(mux)
}

// TestMountAdministrationRoutes_CalledTwiceOnSameMux_Panics is the same
// confirmation for Administration specifically -- what T-02 was asking
// for, and its own permission namespace (administration.*).
func TestMountAdministrationRoutes_CalledTwiceOnSameMux_Panics(t *testing.T) {
	router, _ := buildRoutingTestRouter(t, true, false)
	mux := http.NewServeMux()
	router.MountAdministrationRoutes(mux)

	defer func() {
		if recover() == nil {
			t.Error("expected the second MountAdministrationRoutes call on the same mux to panic, got none")
		}
	}()
	router.MountAdministrationRoutes(mux)
}

// TestMountAssetRoutes_CalledTwiceOnSameMux_Panics is the same
// confirmation for the much larger (98-route) Assets surface --
// proving the same "fail loud, not silent" property holds at scale,
// not just for the small modules' handful of routes each.
func TestMountAssetRoutes_CalledTwiceOnSameMux_Panics(t *testing.T) {
	router, _ := buildRoutingTestRouter(t, true, true)
	mux := http.NewServeMux()
	router.MountAssetRoutes(mux)

	defer func() {
		if recover() == nil {
			t.Error("expected the second MountAssetRoutes call on the same mux to panic, got none")
		}
	}()
	router.MountAssetRoutes(mux)
}

// TestMountXRoutes_OnSeparateMuxes_BothSucceed is the inverse check: the
// same router mounting its routes onto two DIFFERENT mux instances (a
// legitimate pattern -- e.g. two independent test setups, or a
// hypothetical future multi-listener configuration) must not panic.
// The failure mode above is specifically "same mux twice," not "this
// router can only ever mount once, anywhere."
func TestMountXRoutes_OnSeparateMuxes_BothSucceed(t *testing.T) {
	router, _ := buildRoutingTestRouter(t, true, true)
	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("mounting the same router's routes onto two separate mux instances should not panic, got: %v", rec)
		}
	}()
	mux1 := http.NewServeMux()
	mux2 := http.NewServeMux()
	router.MountAccountRoutes(mux1)
	router.MountSettingsRoutes(mux1)
	router.MountAdministrationRoutes(mux1)
	router.MountAssetRoutes(mux1)
	router.MountAccountRoutes(mux2)
	router.MountSettingsRoutes(mux2)
	router.MountAdministrationRoutes(mux2)
	router.MountAssetRoutes(mux2)
}

// TestMountAssetRoutes_MasterViewAndEditRoutesRegistered confirms that both
// /{entity}/{id} (direct navigation from list rows) and /{entity}/{id}/edit
// are properly registered and reachable for all master tables.
func TestMountAssetRoutes_MasterViewAndEditRoutesRegistered(t *testing.T) {
	masterEntities := []string{
		"regions",
		"currencies",
		"cost-centers",
		"brands",
		"custodians",
		"alert-types",
		"properties",
		"models",
		"locations",
	}

	t.Run("EnableAdmin=true: registered, redirects unauthenticated to login", func(t *testing.T) {
		router, _ := buildRoutingTestRouter(t, true, false)
		mux := mountedMux(router)
		for _, entity := range masterEntities {
			for _, path := range []string{"/" + entity + "/1", "/" + entity + "/1/edit"} {
				req := httptest.NewRequest("GET", path, nil)
				rec := httptest.NewRecorder()
				mux.ServeHTTP(rec, req)
				if rec.Code != http.StatusSeeOther {
					t.Errorf("route %s (EnableAdmin=true) unauthenticated: got %d, want %d", path, rec.Code, http.StatusSeeOther)
				}
			}
		}
	})

	t.Run("EnableAdmin=false: not registered at all", func(t *testing.T) {
		router, _ := buildRoutingTestRouter(t, false, false)
		mux := mountedMux(router)
		for _, entity := range masterEntities {
			for _, path := range []string{"/" + entity + "/1", "/" + entity + "/1/edit"} {
				req := httptest.NewRequest("GET", path, nil)
				rec := httptest.NewRecorder()
				mux.ServeHTTP(rec, req)
				if rec.Code != http.StatusNotFound {
					t.Errorf("route %s (EnableAdmin=false): got %d, want %d (route should not exist)", path, rec.Code, http.StatusNotFound)
				}
			}
		}
	})
}
