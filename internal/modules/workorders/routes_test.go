package workorders

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
// registers the Work Orders module's routes on the mux it's given, via
// router.MountWorkOrdersRoutes -- see internal/modules/account's own
// identical test for why this can't also cover the substantive wiring
// (that lives in internal/ui/router_test.go). EnableWorkOrders must be
// true here -- this module's routes are gated behind it, default false.
func TestMountRoutes_Delegates(t *testing.T) {
	translator, err := i18n.New("en")
	if err != nil {
		t.Fatalf("i18n.New: %v", err)
	}
	sessionMgr := auth.NewSessionManager(auth.SessionConfig{Secret: "test-secret"})
	client := xolu.New("http://127.0.0.1:1")
	cfg := ui.Config{AppName: "Test", Version: "0.0.0", EnableWorkOrders: true}
	router := ui.NewRouter(client, cfg, translator, nil, sessionMgr, nil, nil, nil, nil)

	mux := http.NewServeMux()
	MountRoutes(router)(mux)
	mux.Handle("/", router)

	req := httptest.NewRequest("GET", "/workorders", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("GET /workorders via workorders.MountRoutes(router): got %d, want %d (redirect to login) -- delegation may be broken", rec.Code, http.StatusSeeOther)
	}
}

// TestMountRoutes_NoOpWhenDisabled confirms the flag actually gates
// route registration, not just some downstream behavior -- with
// EnableWorkOrders false (the real default), the route must not exist
// at all.
func TestMountRoutes_NoOpWhenDisabled(t *testing.T) {
	translator, err := i18n.New("en")
	if err != nil {
		t.Fatalf("i18n.New: %v", err)
	}
	sessionMgr := auth.NewSessionManager(auth.SessionConfig{Secret: "test-secret"})
	client := xolu.New("http://127.0.0.1:1")
	cfg := ui.Config{AppName: "Test", Version: "0.0.0"} // EnableWorkOrders left false
	router := ui.NewRouter(client, cfg, translator, nil, sessionMgr, nil, nil, nil, nil)

	mux := http.NewServeMux()
	MountRoutes(router)(mux)
	mux.Handle("/", router)

	req := httptest.NewRequest("GET", "/workorders", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /workorders with EnableWorkOrders=false: got %d, want %d", rec.Code, http.StatusNotFound)
	}
}
