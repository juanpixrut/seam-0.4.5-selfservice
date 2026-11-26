package ui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/i18n"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

// TestAssetForm_DumpPage writes the translated Assets edit form (the fragment htmx
// loads into the page) to the file named by SEAM_ASSET_DUMP, for
// scripts/browser-test-assets.js. A dormant guard: skipped unless the variable is set.
func TestAssetForm_DumpPage(t *testing.T) {
	out := os.Getenv("SEAM_ASSET_DUMP")
	if out == "" {
		t.Skip("SEAM_ASSET_DUMP not set")
	}
	srv := newAssetDynamicTestServer(t, 7, map[string]any{
		"id": float64(7), "name": "Pump 7", "type_id": float64(1), "parent_id": float64(3),
		"gps_location": "-34.9011,-56.1645", "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
	}, nil)
	defer srv.Close()
	translator, err := i18n.New("en")
	if err != nil {
		t.Fatal(err)
	}
	sessionMgr := auth.NewSessionManager(auth.SessionConfig{Secret: "test-secret-asset"})
	cfg := Config{AppName: "Test", Version: "0.0.0", Locale: "en", EnableOperational: true}
	router := NewRouter(xolu.New(srv.URL), cfg, translator, nil, sessionMgr, nil, nil, nil, nil)
	req := httptest.NewRequest("GET", "/assets/dynamic/7/edit?tab=tab-geo", nil)
	req.Header.Set("HX-Request", "true")
	req.AddCookie(sessionCookie(t, sessionMgr, auth.RoleAdmin))
	rec := httptest.NewRecorder()
	mux := http.NewServeMux()
	router.MountAssetRoutes(mux)
	mux.Handle("/", router)
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, rec.Body.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}
