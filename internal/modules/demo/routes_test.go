package demo

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/i18n"
	"github.com/ha1tch/seam-ui/internal/ui"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

func demoTestMux(t *testing.T, enable bool) *http.ServeMux {
	t.Helper()
	translator, err := i18n.New("en")
	if err != nil {
		t.Fatalf("i18n.New: %v", err)
	}
	sessionMgr := auth.NewSessionManager(auth.SessionConfig{Secret: "test-secret"})
	client := xolu.New("http://127.0.0.1:1")
	cfg := ui.Config{AppName: "Test", Version: "0.0.0", EnableDemo: enable}
	router := ui.NewRouter(client, cfg, translator, nil, sessionMgr, nil, nil, nil, nil)

	mux := http.NewServeMux()
	MountRoutes(router)(mux)
	mux.Handle("/", router)
	return mux
}

// With EnableDemo true the closure registers the routes (an unauthenticated
// request is redirected to login); with the flag off (the default) the route
// must not exist at all.
func TestMountRoutes_DelegatesAndIsGatedByFlag(t *testing.T) {
	rec := httptest.NewRecorder()
	demoTestMux(t, true).ServeHTTP(rec, httptest.NewRequest("GET", "/demo", nil))
	if rec.Code != http.StatusSeeOther {
		t.Errorf("GET /demo with EnableDemo=true: got %d, want %d", rec.Code, http.StatusSeeOther)
	}
	rec = httptest.NewRecorder()
	demoTestMux(t, false).ServeHTTP(rec, httptest.NewRequest("GET", "/demo", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /demo with EnableDemo=false: got %d, want %d", rec.Code, http.StatusNotFound)
	}
}
