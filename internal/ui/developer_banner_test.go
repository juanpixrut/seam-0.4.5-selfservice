package ui

import (
	"fmt"
	"net/url"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"

	"github.com/ha1tch/seam-ui/internal/auth"
)

// probeT echoes the key, followed by its arguments, so the test can see what the
// notice passes to its message.
func probeT(k string, a ...any) string {
	if len(a) > 0 {
		return k + ": " + fmt.Sprint(a...)
	}
	return k
}

func bannerHTML(cfg Config) string {
	return mi.RenderToString(mi.H(func(b *mi.Builder) mi.Node {
		return b.Div(append([]interface{}{mi.Class("probe")}, devBannerNodes(b, cfg, probeT)...)...)
	}))
}

func TestDevBanner_NothingAtAllWhenNothingIsActive(t *testing.T) {
	if html := bannerHTML(Config{}); strings.Contains(html, "dev-banner") {
		t.Errorf("no active switch, no notice: %s", html)
	}
}

func TestDevBanner_NamesTheSwitchesAndOffersManageToDevelopersOnly(t *testing.T) {
	cfg := Config{DevActive: []string{"show_i18n_keys", "overlay_filesystem"}, UserRole: auth.RoleDeveloper}
	html := bannerHTML(cfg)
	if !strings.Contains(html, `role="status"`) || !strings.Contains(html, "show_i18n_keys, overlay_filesystem") {
		t.Errorf("the notice names each switch and is a status region:\n%s", html)
	}
	if !strings.Contains(html, `href="/developer/overview"`) {
		t.Errorf("a developer can go and change it")
	}
	cfg.UserRole = auth.RoleAdmin
	if strings.Contains(bannerHTML(cfg), "/developer/overview") {
		t.Errorf("other roles are told, not offered a link they cannot use")
	}
}

// Adversarial: the notice only ever names catalogue keys, and escapes them anyway.
func TestDevBanner_EscapesWhatItNames(t *testing.T) {
	html := bannerHTML(Config{DevActive: []string{`"><script>alert(1)</script>`}})
	if strings.Contains(html, "<script>alert(1)") {
		t.Errorf("unescaped switch name in the notice")
	}
}

// The whole path: switching a behaviour-changing switch on makes every page of
// the tenant say so, for every role; switching it back removes the notice.
func TestDevBanner_FollowsTheTenantsSwitchesEndToEnd(t *testing.T) {
	h := newDevHarness(t, true, true)
	page := func(role string) string { return h.do(t, "GET", "/admin/users/new", role, nil).Body.String() }
	flag := func(key, action string) {
		t.Helper()
		rec := h.post(t, "/developer/flags", url.Values{"key": {key}, "action": {action}})
		if !strings.Contains(location(rec), "msg=") {
			t.Fatalf("%s %s refused: %s", key, action, location(rec))
		}
	}
	if strings.Contains(page("admin"), "dev-banner") {
		t.Fatal("precondition: no notice by default")
	}
	flag("show_i18n_keys", "on")
	if !strings.Contains(page("admin"), "dev-banner") || !strings.Contains(page("developer"), "dev-banner") {
		t.Errorf("show-keys changes what the tenant sees: every role gets the notice")
	}
	if strings.Contains(page("admin"), "/developer/overview") || !strings.Contains(page("developer"), "/developer/overview") {
		t.Errorf("only a developer is offered the way back to the switch")
	}
	flag("show_i18n_keys", "reset")
	if strings.Contains(page("admin"), "dev-banner") {
		t.Errorf("resetting the switch removes the notice")
	}
	flag("overlay_filesystem", "off")
	if !strings.Contains(page("admin"), "dev-banner") {
		t.Errorf("switching a layer off changes what the tenant sees")
	}
	flag("overlay_filesystem", "reset")
	flag("hotload_i18n", "on")
	flag("hotload_schemas", "on")
	if strings.Contains(page("admin"), "dev-banner") {
		t.Errorf("hot loading changes nothing a user sees and is not announced")
	}
}

func TestDeveloperActiveFlags_EdgeCases(t *testing.T) {
	if developerActiveFlags(nil, 1) != nil {
		t.Errorf("no runtime, no flags")
	}
	h := newDevHarness(t, false, false)
	rt := h.runtimeForTest()
	for _, tenant := range []int64{0, -1} {
		if developerActiveFlags(rt, tenant) != nil {
			t.Errorf("tenant %d has no state", tenant)
		}
	}
}
