package issues

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
// registers the Issues module's routes on the mux it's given, via
// router.MountIssuesRoutes -- see internal/modules/account's own
// identical test for why this can't also cover the substantive wiring
// (that lives in internal/ui/router_test.go). EnableIssues must be
// true here -- this module's routes are gated behind it, default false.
func TestMountRoutes_Delegates(t *testing.T) {
	translator, err := i18n.New("en")
	if err != nil {
		t.Fatalf("i18n.New: %v", err)
	}
	sessionMgr := auth.NewSessionManager(auth.SessionConfig{Secret: "test-secret"})
	client := xolu.New("http://127.0.0.1:1")
	cfg := ui.Config{AppName: "Test", Version: "0.0.0", EnableIssues: true}
	router := ui.NewRouter(client, cfg, translator, nil, sessionMgr, nil, nil, nil, nil)

	mux := http.NewServeMux()
	MountRoutes(router)(mux)
	mux.Handle("/", router)

	req := httptest.NewRequest("GET", "/issues", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("GET /issues via issues.MountRoutes(router): got %d, want %d (redirect to login) -- delegation may be broken", rec.Code, http.StatusSeeOther)
	}
}

// TestMountRoutes_NoOpWhenDisabled confirms the flag actually gates
// route registration, not just some downstream behavior -- with
// EnableIssues false (the real default), the route must not exist
// at all.
func TestMountRoutes_NoOpWhenDisabled(t *testing.T) {
	translator, err := i18n.New("en")
	if err != nil {
		t.Fatalf("i18n.New: %v", err)
	}
	sessionMgr := auth.NewSessionManager(auth.SessionConfig{Secret: "test-secret"})
	client := xolu.New("http://127.0.0.1:1")
	cfg := ui.Config{AppName: "Test", Version: "0.0.0"} // EnableIssues left false
	router := ui.NewRouter(client, cfg, translator, nil, sessionMgr, nil, nil, nil, nil)

	mux := http.NewServeMux()
	MountRoutes(router)(mux)
	mux.Handle("/", router)

	req := httptest.NewRequest("GET", "/issues", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /issues with EnableIssues=false: got %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// TestMountRoutes_PublicReportFollowsTheFlag confirms the anonymous QR
// channel is mounted with the module and absent without it: with
// EnableIssues false there is nowhere to send a report.
func TestMountRoutes_PublicReportFollowsTheFlag(t *testing.T) {
	translator, err := i18n.New("en")
	if err != nil {
		t.Fatalf("i18n.New: %v", err)
	}
	for _, on := range []bool{true, false} {
		sessionMgr := auth.NewSessionManager(auth.SessionConfig{Secret: "test-secret"})
		client := xolu.New("http://127.0.0.1:1")
		cfg := ui.Config{AppName: "Test", Version: "0.0.0", EnableIssues: on}
		router := ui.NewRouter(client, cfg, translator, nil, sessionMgr, nil, nil, nil, nil)

		mux := http.NewServeMux()
		MountRoutes(router)(mux)
		_, pattern := mux.Handler(httptest.NewRequest("GET", "/s/AAAAAAAAAAAAAAAA/done", nil))
		if mounted := pattern != ""; mounted != on {
			t.Errorf("EnableIssues=%v: GET /s/{code}/done mounted=%v (pattern %q)", on, mounted, pattern)
		}
	}
}
