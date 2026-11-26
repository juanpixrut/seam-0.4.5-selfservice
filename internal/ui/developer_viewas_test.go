package ui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/modules"
)

func (h *devHarness) doWith(t *testing.T, method, path string, c *http.Cookie, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if form != nil {
		req = httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if c != nil {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, req)
	return rec
}

func newCookie(t *testing.T, rec *httptest.ResponseRecorder, old *http.Cookie) *http.Cookie {
	t.Helper()
	cs := rec.Result().Cookies()
	if len(cs) == 0 {
		return old
	}
	return cs[0]
}

func TestViewAs_EndToEnd(t *testing.T) {
	h := newDevHarness(t, true, true)
	c := sessionCookie(t, h.sess, "developer")
	if rec := h.doWith(t, "GET", "/admin/users", c, nil); rec.Code != http.StatusOK {
		t.Fatalf("precondition: a developer reaches user administration: %d", rec.Code)
	}
	rec := h.doWith(t, "POST", "/developer/view-as", c, url.Values{"role": {"viewer"}})
	if !strings.Contains(location(rec), "msg=view_as_set") {
		t.Fatalf("choosing a role: %d %s", rec.Code, location(rec))
	}
	c = newCookie(t, rec, c)

	if rec := h.doWith(t, "GET", "/admin/users", c, nil); rec.Code != http.StatusForbidden {
		t.Errorf("as a viewer the user administration is closed: %d", rec.Code)
	}
	over := h.doWith(t, "GET", "/developer/overview", c, nil)
	if over.Code != http.StatusOK {
		t.Fatalf("the Developer module must stay reachable while viewing as a viewer: %d", over.Code)
	}
	body := over.Body.String()
	for _, want := range []string{"dev-banner-viewas", "/developer/view-as/exit", "Currently viewing as"} {
		if !strings.Contains(body, want) {
			t.Errorf("the overview must show %q while viewing as", want)
		}
	}
	forms := h.doWith(t, "GET", "/developer/forms", c, nil)
	if forms.Code != http.StatusOK || !strings.Contains(forms.Body.String(), "dev-banner-viewas") {
		t.Errorf("the notice is on every page: %d", forms.Code)
	}
	// the module still works: saving a form definition is a developer's act
	save := h.doWith(t, "POST", "/developer/forms/save", c, url.Values{"schema": {"system/user_create"}, "json": {e2eOverride}, "scope": {"file"}})
	if !strings.Contains(location(save), "msg=saved") {
		t.Errorf("the module acts with the real role: %s", location(save))
	}

	rec = h.doWith(t, "GET", "/developer/view-as/exit", c, nil)
	if !strings.Contains(location(rec), "msg=view_as_exit") {
		t.Fatalf("exit: %s", location(rec))
	}
	c = newCookie(t, rec, c)
	if rec := h.doWith(t, "GET", "/admin/users", c, nil); rec.Code != http.StatusOK {
		t.Errorf("after exiting the developer is a developer again: %d", rec.Code)
	}
	if strings.Contains(h.doWith(t, "GET", "/developer/forms", c, nil).Body.String(), "dev-banner-viewas") {
		t.Errorf("no notice after exiting")
	}

	// as an admin the user administration is open, and the notice says so
	rec = h.doWith(t, "POST", "/developer/view-as", c, url.Values{"role": {"admin"}})
	c = newCookie(t, rec, c)
	adm := h.doWith(t, "GET", "/admin/users", c, nil)
	if adm.Code != http.StatusOK || !strings.Contains(adm.Body.String(), "dev-banner-viewas") {
		t.Errorf("as an admin: %d, and the notice is there on an ordinary page", adm.Code)
	}
	// choosing the developer role is leaving
	rec = h.doWith(t, "POST", "/developer/view-as", c, url.Values{"role": {"developer"}})
	if !strings.Contains(location(rec), "msg=view_as_exit") {
		t.Errorf("choosing developer is leaving: %s", location(rec))
	}
}

func TestViewAs_RefusalsAndHostileInput(t *testing.T) {
	h := newDevHarness(t, true, true)
	c := sessionCookie(t, h.sess, "developer")
	for _, role := range []string{"root", "ADMIN", " viewer", "viewer ", "admin' OR '1'='1", "<script>", strings.Repeat("v", 10000), "../developer"} {
		rec := h.doWith(t, "POST", "/developer/view-as", c, url.Values{"role": {role}})
		if !strings.Contains(location(rec), "error=bad_request") || len(rec.Result().Cookies()) != 0 {
			t.Errorf("%.20q must be refused and change nothing: %s, %d cookies", role, location(rec), len(rec.Result().Cookies()))
		}
	}
	// empty means leaving, which is always allowed
	if rec := h.doWith(t, "POST", "/developer/view-as", c, url.Values{}); !strings.Contains(location(rec), "msg=view_as_exit") {
		t.Errorf("no role leaves: %s", location(rec))
	}
	// nobody but a developer, whatever they post
	a := sessionCookie(t, h.sess, "admin")
	if rec := h.doWith(t, "POST", "/developer/view-as", a, url.Values{"role": {"viewer"}}); rec.Code != http.StatusForbidden {
		t.Errorf("an admin must not view as anything: %d", rec.Code)
	}
	if rec := h.doWith(t, "GET", "/developer/view-as/exit", a, nil); rec.Code != http.StatusForbidden {
		t.Errorf("nor leave a view they never had: %d", rec.Code)
	}
	if rec := h.doWith(t, "POST", "/developer/view-as", nil, url.Values{"role": {"viewer"}}); rec.Code == http.StatusOK || rec.Code == http.StatusSeeOther && !strings.HasPrefix(location(rec), "/login") {
		t.Errorf("no session: %d %s", rec.Code, location(rec))
	}
}

func TestRouterContextFor_OnlyAnAuthenticDeveloperSessionWithTheModuleOnViewsAs(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	sess := func(role, viewAs string) *auth.SessionData {
		return &auth.SessionData{UserID: 7, TenantID: 3, Email: "d@example.com", Role: role, ViewAs: viewAs}
	}
	on, off := &Router{config: Config{EnableDeveloper: true}}, &Router{config: Config{EnableDeveloper: false}}
	check := func(name string, r *Router, s *auth.SessionData, wantRole, wantReal string) {
		t.Helper()
		ctx := r.contextFor(req, s)
		claims, ok := auth.ClaimsFromContext(ctx)
		if !ok || claims.Role != wantRole || claims.UserID != 7 || claims.TenantID != 3 {
			t.Errorf("%s: claims %+v", name, claims)
		}
		real, viewing := auth.RealRoleFromContext(ctx)
		if wantReal == "" && viewing || wantReal != "" && real != wantReal {
			t.Errorf("%s: real role %q (recorded %v), want %q", name, real, viewing, wantReal)
		}
	}
	check("viewing as", on, sess("developer", "viewer"), "viewer", "developer")
	check("the module is off", off, sess("developer", "viewer"), "developer", "")
	check("nothing chosen", on, sess("developer", ""), "developer", "")
	check("an admin session carrying one", on, sess("admin", "viewer"), "admin", "")
	check("a session claiming to view as developer", on, sess("viewer", "developer"), "viewer", "")
	check("an unknown role", on, sess("developer", "root"), "developer", "")
}

func TestViewerConfig_ViewingAsKeepsTheDeveloperModuleInTheSelector(t *testing.T) {
	reg := modules.NewRegistry()
	reg.Register(modules.Module{ID: "developer", Label: "nav.module.developer", Path: "/developer", Group: "apps", HasAccess: func(r string) bool { return r == auth.RoleDeveloper }})
	reg.Register(modules.Module{ID: "assets", Label: "nav.module.assets", Path: "/assets", Group: "apps", HasAccess: func(string) bool { return true }})
	h := NewPageHandler(Config{AppName: "T"})
	h.SetModuleRegistry(reg)
	has := func(cfg Config) bool {
		for _, m := range cfg.VisibleModules {
			if m.ID == "developer" {
				return true
			}
		}
		return false
	}
	base := httptest.NewRequest("GET", "/x", nil)
	viewer := auth.ContextWithClaims(base.Context(), &auth.Claims{UserID: 1, TenantID: 1, Role: auth.RoleViewer})
	if cfg := h.viewerConfig(base.WithContext(viewer)); has(cfg) || cfg.RealRole != "" {
		t.Errorf("an ordinary viewer has no Developer module and no notice")
	}
	viewing := auth.ContextWithRealRole(viewer, auth.RoleDeveloper)
	cfg := h.viewerConfig(base.WithContext(viewing))
	if !has(cfg) || cfg.RealRole != auth.RoleDeveloper || cfg.UserRole != auth.RoleViewer {
		t.Errorf("viewing as a viewer: module kept=%v real=%q role=%q", has(cfg), cfg.RealRole, cfg.UserRole)
	}
	if len(cfg.VisibleModules) != 2 {
		t.Errorf("only the Developer module is added, nothing else the real role could see: %d", len(cfg.VisibleModules))
	}
	// being a developer, not viewing as anything, changes nothing
	dev := auth.ContextWithClaims(base.Context(), &auth.Claims{UserID: 1, TenantID: 1, Role: auth.RoleDeveloper})
	if cfg := h.viewerConfig(base.WithContext(dev)); !has(cfg) || cfg.RealRole != "" {
		t.Errorf("a developer as themselves: module=%v real=%q", has(cfg), cfg.RealRole)
	}
}

func TestDevBanner_ViewAsNoticeNamesTheRoleAndHowToLeave(t *testing.T) {
	html := bannerHTML(Config{UserRole: auth.RoleViewer, RealRole: auth.RoleDeveloper})
	for _, want := range []string{"dev-banner-viewas", `role="status"`, "user.role.viewer", `href="/developer/view-as/exit"`, "developer.viewas.exit"} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q:\n%s", want, html)
		}
	}
	if strings.Contains(bannerHTML(Config{UserRole: auth.RoleDeveloper}), "viewas") {
		t.Errorf("no notice when not viewing as anything")
	}
	both := bannerHTML(Config{UserRole: auth.RoleViewer, RealRole: auth.RoleDeveloper, DevActive: []string{"show_i18n_keys"}})
	if !strings.Contains(both, "developer.banner.manage") || !strings.Contains(both, "dev-banner-viewas") {
		t.Errorf("a developer viewing as another role still gets the way to the switches, and both notices")
	}
}
