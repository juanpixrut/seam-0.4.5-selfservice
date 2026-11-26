package ui

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/devruntime"
	"github.com/ha1tch/seam-ui/internal/devstate"
	"github.com/ha1tch/seam-ui/internal/features"
	"github.com/ha1tch/seam-ui/internal/i18n"
)

func flagSet(t *testing.T, env map[string]string) *features.Set {
	t.Helper()
	set, err := features.Catalog().Resolve(features.Inputs{AuthEnabled: true, LookupEnv: func(n string) (string, bool) { v, ok := env[n]; return v, ok }})
	if err != nil {
		t.Fatal(err)
	}
	return set
}

func TestConfigFlagOn_FallsBackToTheCatalogueDefaultWithoutAFlagSet(t *testing.T) {
	var c Config
	for key, want := range map[string]bool{features.DeveloperViewAs: true, features.DeveloperChangeNotice: true, features.DeveloperProtectOwnRole: true,
		features.DeveloperTenantOverrides: false, features.DeveloperFormDesigner: false} {
		if got := c.flagOn(key); got != want {
			t.Errorf("%s = %v, want %v", key, got, want)
		}
	}
	c.Flags = flagSet(t, map[string]string{"SEAM_ENABLE_DEVELOPER": "1", "SEAM_DEVELOPER_VIEW_AS": "0", "SEAM_DEVELOPER_TENANT_OVERRIDES": "1"})
	if c.flagOn(features.DeveloperViewAs) || !c.flagOn(features.DeveloperTenantOverrides) {
		t.Errorf("a resolved set wins over the defaults")
	}
}

func TestConfigMayViewAs_NeedsTheModuleTheDecisionAndTheRight(t *testing.T) {
	on := Config{EnableDeveloper: true}
	off := Config{EnableDeveloper: true, Flags: flagSet(t, map[string]string{"SEAM_ENABLE_DEVELOPER": "1", "SEAM_DEVELOPER_VIEW_AS": "0"})}
	for _, c := range []struct {
		name       string
		cfg        Config
		real, role string
		want       bool
	}{
		{"all three", on, auth.RoleDeveloper, auth.RoleViewer, true},
		{"the decision is off", off, auth.RoleDeveloper, auth.RoleViewer, false},
		{"the module is off", Config{}, auth.RoleDeveloper, auth.RoleViewer, false},
		{"not a developer", on, auth.RoleAdmin, auth.RoleViewer, false},
		{"not a role", on, auth.RoleDeveloper, "root", false},
		{"no role", on, auth.RoleDeveloper, "", false},
		{"an Accounting role without Accounting", on, auth.RoleDeveloper, auth.RoleAccountant, false},
		{"an Accounting role with Accounting", Config{EnableDeveloper: true, EnableAccounting: true}, auth.RoleDeveloper, auth.RoleAccountExecutive, true},
		{"a platform role with Accounting", Config{EnableDeveloper: true, EnableAccounting: true}, auth.RoleDeveloper, auth.RoleManager, true},
	} {
		if got := c.cfg.mayViewAs(c.real, c.role); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	// and the router ignores a view-as already in a session when the decision is off
	r := &Router{config: off}
	ctx := r.contextFor(httptest.NewRequest("GET", "/", nil), &auth.SessionData{UserID: 1, TenantID: 1, Role: auth.RoleDeveloper, ViewAs: auth.RoleViewer})
	claims, _ := auth.ClaimsFromContext(ctx)
	if _, viewing := auth.RealRoleFromContext(ctx); viewing || claims.Role != auth.RoleDeveloper {
		t.Errorf("turning the decision off must end every view-as at once: role %q", claims.Role)
	}
}

func TestViewerConfig_TheChangeNoticeIsADecision(t *testing.T) {
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	run := func(env map[string]string) []string {
		set := flagSet(t, env)
		tr := i18n.NewFromMessages("en", map[string]map[string]string{"en": {"x": "y"}})
		rt, err := devruntime.Start(context.Background(), devruntime.Options{Flags: set, Translator: tr, Store: devstate.NewMemoryStore(), Logger: quiet})
		if err != nil || rt == nil {
			t.Fatalf("Start: %v", err)
		}
		if err := rt.Manager.Change(context.Background(), 1, features.ShowI18NKeys, true, 1); err != nil {
			t.Fatal(err)
		}
		h := NewPageHandler(Config{AppName: "T", Flags: set, DevRuntime: rt})
		req := httptest.NewRequest("GET", "/x", nil)
		req = req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{UserID: 1, TenantID: 1, Role: auth.RoleAdmin}))
		return h.viewerConfig(req).DevActive
	}
	if got := run(map[string]string{"SEAM_ENABLE_DEVELOPER": "1"}); len(got) != 1 {
		t.Errorf("by default the notice is on: %v", got)
	}
	if got := run(map[string]string{"SEAM_ENABLE_DEVELOPER": "1", "SEAM_DEVELOPER_CHANGE_NOTICE": "0"}); len(got) != 0 {
		t.Errorf("with the decision off there is no notice: %v", got)
	}
}

func TestAdminUserUpdate_ProtectingTheOwnRoleIsADecision(t *testing.T) {
	post := func(set *features.Set) (code int, patches int32) {
		var n atomic.Int32
		var last atomic.Value
		server := guardServer(t, auth.RoleDeveloper, &n, &last)
		defer server.Close()
		h := newHandler(t, server)
		h.config.Flags = set
		req := httptest.NewRequest("POST", "/admin/users/5", strings.NewReader(url.Values{"name": {"Me"}, "role": {"admin"}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetPathValue("id", "5")
		rec := httptest.NewRecorder()
		h.AdminUserUpdate(rec, asUser(req, 5, auth.RoleDeveloper))
		return rec.Code, n.Load()
	}
	if code, n := post(nil); code != http.StatusUnprocessableEntity || n != 0 {
		t.Errorf("by default a developer cannot remove their own role: %d, %d writes", code, n)
	}
	off := flagSet(t, map[string]string{"SEAM_ENABLE_DEVELOPER": "1", "SEAM_DEVELOPER_PROTECT_OWN_ROLE": "0"})
	if code, n := post(off); code == http.StatusUnprocessableEntity || n != 1 {
		t.Errorf("with the decision off they can: %d, %d writes", code, n)
	}
}
