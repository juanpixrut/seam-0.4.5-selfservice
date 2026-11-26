package administration

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/i18n"
	"github.com/ha1tch/seam-ui/internal/ui"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

// TestMountRoutes_Delegates confirms the returned closure actually
// registers the Administration module's routes on the mux it's given,
// via router.MountAdministrationRoutes -- see internal/modules/account's
// own identical test for why this can't also cover the substantive
// wiring (that lives in internal/ui/router_test.go). EnableAdmin must
// be true here -- Administration's routes are gated behind it.
func TestMountRoutes_Delegates(t *testing.T) {
	translator, err := i18n.New("en")
	if err != nil {
		t.Fatalf("i18n.New: %v", err)
	}
	sessionMgr := auth.NewSessionManager(auth.SessionConfig{Secret: "test-secret"})
	client := xolu.New("http://127.0.0.1:1")
	cfg := ui.Config{AppName: "Test", Version: "0.0.0", EnableAdmin: true}
	router := ui.NewRouter(client, cfg, translator, nil, sessionMgr, nil, nil, nil, nil)

	mux := http.NewServeMux()
	MountRoutes(router)(mux)
	mux.Handle("/", router)

	req := httptest.NewRequest("GET", "/admin/users", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("GET /admin/users via administration.MountRoutes(router): got %d, want %d (redirect to login) -- delegation may be broken", rec.Code, http.StatusSeeOther)
	}
}
