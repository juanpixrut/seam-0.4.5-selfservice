package account

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
// registers the Account module's routes on the mux it's given, via
// router.MountAccountRoutes -- the substantive wiring test lives in
// internal/ui/router_test.go (this package can't import internal/ui's
// test file since it's package ui, not ui_test, and importing
// internal/modules/account from there would be a cycle); this just
// confirms the two-line delegation itself isn't silently broken (wrong
// mux, nil router, etc.).
func TestMountRoutes_Delegates(t *testing.T) {
	translator, err := i18n.New("en")
	if err != nil {
		t.Fatalf("i18n.New: %v", err)
	}
	sessionMgr := auth.NewSessionManager(auth.SessionConfig{Secret: "test-secret"})
	client := xolu.New("http://127.0.0.1:1")
	cfg := ui.Config{AppName: "Test", Version: "0.0.0"}
	router := ui.NewRouter(client, cfg, translator, nil, sessionMgr, nil, nil, nil, nil)

	mux := http.NewServeMux()
	MountRoutes(router)(mux)
	mux.Handle("/", router)

	req := httptest.NewRequest("GET", "/profile", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("GET /profile via account.MountRoutes(router): got %d, want %d (redirect to login) -- delegation may be broken", rec.Code, http.StatusSeeOther)
	}
}
