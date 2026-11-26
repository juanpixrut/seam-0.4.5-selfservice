package developer

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/i18n"
	"github.com/ha1tch/seam-ui/internal/ui"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

func developerTestSetup(t *testing.T, enable bool) (*http.ServeMux, *auth.SessionManager) {
	t.Helper()
	translator, err := i18n.New("en")
	if err != nil {
		t.Fatalf("i18n.New: %v", err)
	}
	sessionMgr := auth.NewSessionManager(auth.SessionConfig{Secret: "test-secret"})
	client := xolu.New("http://127.0.0.1:1")
	cfg := ui.Config{AppName: "Test", Version: "0.0.0", EnableDeveloper: enable}
	router := ui.NewRouter(client, cfg, translator, nil, sessionMgr, nil, nil, nil, nil)

	mux := http.NewServeMux()
	MountRoutes(router)(mux)
	mux.Handle("/", router)
	return mux, sessionMgr
}

// With EnableDeveloper true the route exists and an unauthenticated request
// is redirected to login; with the flag off (the default) it must not exist.
func TestMountRoutes_DelegatesAndIsGatedByFlag(t *testing.T) {
	mux, _ := developerTestSetup(t, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/developer", nil))
	if rec.Code != http.StatusSeeOther {
		t.Errorf("GET /developer with EnableDeveloper=true: got %d, want %d", rec.Code, http.StatusSeeOther)
	}

	mux, _ = developerTestSetup(t, false)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/developer", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /developer with EnableDeveloper=false: got %d, want %d", rec.Code, http.StatusNotFound)
	}
}
