package ui

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/modules"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

// On an instance that runs without the Assets module (enable_operational
// off, as an Accounting-only one does) "/" has no dashboard to show: it sends
// each viewer to their first application, or to their profile.
func TestDashboard_WithoutAssetsGoesToTheViewersApplication(t *testing.T) {
	cfg := Config{AppName: "Test", Version: "0.0.0"} // EnableOperational left false
	h := NewUIHandler(xolu.New("http://127.0.0.1:1"), cfg, nil, nil, nil, nil, nil, nil, nil)
	reg := modules.NewRegistry()
	reg.Register(modules.Module{ID: "assets", Path: "/", HasAccess: func(string) bool { return false }})
	reg.Register(modules.Module{ID: "documents", Path: "/documents", HasAccess: func(role string) bool { return role == "admin" }})
	reg.Register(modules.Module{ID: "accounting", Path: "/accounting", Primary: true, HasAccess: func(role string) bool { return role != "viewer" }})
	h.page.SetModuleRegistry(reg)

	for role, want := range map[string]string{"admin": "/accounting", "accountant": "/accounting", "viewer": "/profile"} {
		req := httptest.NewRequest("GET", "/", nil)
		req = req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{UserID: 1, TenantID: 1, Role: role}))
		rec := httptest.NewRecorder()
		h.Dashboard(rec, req)
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != want {
			t.Errorf("%s: got %d to %q, want 303 to %q", role, rec.Code, rec.Header().Get("Location"), want)
		}
	}
}
