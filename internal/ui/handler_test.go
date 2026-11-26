package ui

import (
	"net/http/httptest"
	"testing"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/modules"
)

func TestViewerConfig_NoClaims_ReturnsBaseConfigUnchanged(t *testing.T) {
	h := &PageHandler{Config: Config{AppName: "Test"}}
	req := httptest.NewRequest("GET", "/", nil) // no claims injected

	got := h.viewerConfig(req)
	if got.UserRole != "" {
		t.Errorf("UserRole = %q, want empty (no claims in context)", got.UserRole)
	}
	if got.VisibleModules != nil {
		t.Errorf("VisibleModules = %#v, want nil (no claims in context)", got.VisibleModules)
	}
}

func TestViewerConfig_SetsUserRoleFromClaims(t *testing.T) {
	h := &PageHandler{Config: Config{AppName: "Test"}}
	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{UserID: 1, Role: "manager"}))

	got := h.viewerConfig(req)
	if got.UserRole != "manager" {
		t.Errorf("UserRole = %q, want %q", got.UserRole, "manager")
	}
}

func TestViewerConfig_NilModuleRegistry_LeavesVisibleModulesNil(t *testing.T) {
	h := &PageHandler{Config: Config{AppName: "Test"}} // no ModuleRegistry set
	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{UserID: 1, Role: "admin"}))

	got := h.viewerConfig(req)
	if got.VisibleModules != nil {
		t.Errorf("VisibleModules = %#v, want nil when no ModuleRegistry is configured", got.VisibleModules)
	}
}

func TestViewerConfig_PopulatesVisibleModulesFromRegistry(t *testing.T) {
	reg := modules.NewRegistry()
	reg.Register(modules.Module{ID: "assets", Group: "apps", HasAccess: func(role string) bool { return true }})
	reg.Register(modules.Module{ID: "documents", Group: "apps", HasAccess: func(role string) bool { return role == "admin" }})

	h := &PageHandler{Config: Config{AppName: "Test"}, ModuleRegistry: reg}

	adminReq := httptest.NewRequest("GET", "/", nil)
	adminReq = adminReq.WithContext(auth.ContextWithClaims(adminReq.Context(), &auth.Claims{UserID: 1, Role: "admin"}))
	adminCfg := h.viewerConfig(adminReq)
	if len(adminCfg.VisibleModules) != 2 {
		t.Errorf("admin: got %d visible modules, want 2", len(adminCfg.VisibleModules))
	}

	viewerReq := httptest.NewRequest("GET", "/", nil)
	viewerReq = viewerReq.WithContext(auth.ContextWithClaims(viewerReq.Context(), &auth.Claims{UserID: 2, Role: "viewer"}))
	viewerCfg := h.viewerConfig(viewerReq)
	if len(viewerCfg.VisibleModules) != 1 {
		t.Errorf("viewer: got %d visible modules, want 1 (documents excluded)", len(viewerCfg.VisibleModules))
	}
}

func TestViewerConfig_DoesNotMutateSharedConfig(t *testing.T) {
	// h.Config is shared across requests (held on PageHandler); viewerConfig
	// must return a copy, never write UserRole/VisibleModules back onto it,
	// or one request's role would leak into another's rendering.
	reg := modules.NewRegistry()
	reg.Register(modules.Module{ID: "assets", Group: "apps", HasAccess: func(string) bool { return true }})
	h := &PageHandler{Config: Config{AppName: "Test"}, ModuleRegistry: reg}

	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{UserID: 1, Role: "admin"}))
	_ = h.viewerConfig(req)

	if h.Config.UserRole != "" {
		t.Errorf("h.Config.UserRole was mutated to %q, viewerConfig must not write back to the shared Config", h.Config.UserRole)
	}
	if h.Config.VisibleModules != nil {
		t.Error("h.Config.VisibleModules was mutated, viewerConfig must not write back to the shared Config")
	}
}
