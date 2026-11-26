package ui

import (
	"net/http/httptest"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/devruntime"
	"github.com/ha1tch/seam-ui/internal/i18n"
	"github.com/ha1tch/seam-ui/internal/modules"
)

func chromeFixture() (Config, *i18n.Translator) {
	tr := i18n.NewFromMessages("en", map[string]map[string]string{"en": {"greet": "Hello", "nav.module.probe": "Probe Module"}})
	return Config{AppName: "Test", Locale: "en", Translator: tr, DevRuntime: &devruntime.Runtime{}, ViewerTenantID: 3}, tr
}

func TestChromeTranslate_FollowsTheTenantsSwitchesOnlyWhenTheModuleIsOn(t *testing.T) {
	cfg, tr := chromeFixture()
	tr.SetOptions(3, i18n.Options{ShowKeys: true})
	if got := chromeTranslate(cfg)("greet"); got != "greet" {
		t.Errorf("tenant 3 asked for keys, the chrome must show them, got %q", got)
	}
	other := cfg
	other.ViewerTenantID = 4
	if got := chromeTranslate(other)("greet"); got != "Hello" {
		t.Errorf("another tenant is unaffected, got %q", got)
	}
	off := cfg
	off.DevRuntime = nil
	if got := chromeTranslate(off)("greet"); got != "Hello" {
		t.Errorf("with the module off nothing changes, got %q", got)
	}
	none := cfg
	none.ViewerTenantID = 0
	if got := chromeTranslate(none)("greet"); got != "Hello" {
		t.Errorf("without a tenant it is the global lookup, got %q", got)
	}
	neg := cfg
	neg.ViewerTenantID = -7
	if got := chromeTranslate(neg)("greet"); got != "Hello" {
		t.Errorf("a negative tenant must not select a tenant view, got %q", got)
	}
}

// The visible proof: a module label in the navigation is the key when the
// tenant switched show-keys on, and the text otherwise.
func TestBaseLayout_NavigationFollowsShowKeys(t *testing.T) {
	cfg, tr := chromeFixture()
	reg := modules.NewRegistry()
	reg.Register(modules.Module{ID: "probe", Label: "nav.module.probe", Icon: "code", Group: "apps", Path: "/probe",
		HasAccess: func(string) bool { return true }})
	cfg.VisibleModules = reg.VisibleTo("developer")
	render := func(c Config) string {
		return mi.RenderToString(BaseLayout(c, "T", "/probe", func(b *mi.Builder) mi.Node { return b.Div("body") }))
	}
	if html := render(cfg); !strings.Contains(html, "Probe Module") {
		t.Fatalf("precondition: the module label shows as text")
	}
	tr.SetOptions(3, i18n.Options{ShowKeys: true})
	html := render(cfg)
	if strings.Contains(html, "Probe Module") || !strings.Contains(html, "nav.module.probe") {
		t.Errorf("the navigation must show keys for a tenant that switched show-keys on")
	}
	cfg.ViewerTenantID = 4
	if !strings.Contains(render(cfg), "Probe Module") {
		t.Errorf("another tenant keeps the text")
	}
}

func TestViewerConfig_CarriesTheTenant(t *testing.T) {
	h := NewPageHandler(Config{AppName: "Test"})
	req := httptest.NewRequest("GET", "/x", nil)
	if got := h.viewerConfig(req).ViewerTenantID; got != 0 {
		t.Errorf("no claims, no tenant: %d", got)
	}
	req = req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{UserID: 1, TenantID: 4, Role: "admin"}))
	if got := h.viewerConfig(req).ViewerTenantID; got != 4 {
		t.Errorf("the viewer's tenant must reach the layout config, got %d", got)
	}
	if h.Config.ViewerTenantID != 0 {
		t.Errorf("the handler's own config must never be written to")
	}
}

func TestBaseLayout_DataI18nCancel(t *testing.T) {
	tr := i18n.NewFromMessages("en", map[string]map[string]string{
		"en": {"action.cancel": "Cancel"},
		"es": {"action.cancel": "Cancelar"},
	})
	cfgEs := Config{AppName: "Test", Locale: "es", Translator: tr}
	htmlEs := mi.RenderToString(BaseLayout(cfgEs, "T", "/", func(b *mi.Builder) mi.Node { return b.Div("body") }))
	if !strings.Contains(htmlEs, `data-i18n-cancel="Cancelar"`) {
		t.Errorf("expected data-i18n-cancel=\"Cancelar\", got:\n%s", htmlEs)
	}

	cfgEn := Config{AppName: "Test", Locale: "en", Translator: tr}
	htmlEn := mi.RenderToString(BaseLayout(cfgEn, "T", "/", func(b *mi.Builder) mi.Node { return b.Div("body") }))
	if !strings.Contains(htmlEn, `data-i18n-cancel="Cancel"`) {
		t.Errorf("expected data-i18n-cancel=\"Cancel\", got:\n%s", htmlEn)
	}
}
