package ui

import (
	"strings"

	mi "github.com/ha1tch/minty"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/devruntime"
	"github.com/ha1tch/seam-ui/internal/features"
)

// The Developer module's notice (campaign DEV-10). While a tenant has a runtime
// switch on that changes what its users see, every page of that tenant says so,
// so nobody takes translation keys on the screen, or text that ignores an
// override, for a defect. Only switches that change visible behaviour count:
// show-keys on, or a filesystem or database layer switched off. The hot-load
// switches change nothing a user sees and are not announced. When nothing is
// active the layout gains no markup at all.

// developerActiveFlags returns the keys of the behaviour-changing runtime
// switches that differ from their default for the tenant, in catalogue order.
func developerActiveFlags(rt *devruntime.Runtime, tenant int64) []string {
	if rt == nil || rt.Manager == nil || tenant <= 0 {
		return nil
	}
	var out []string
	for _, fv := range rt.Manager.Panel(tenant) {
		if fv.Inactive || fv.Value == fv.Flag.Default {
			continue
		}
		switch fv.Flag.Key {
		case features.ShowI18NKeys, features.OverlayFilesystem, features.OverlayXolu:
			out = append(out, fv.Flag.Key)
		}
	}
	return out
}

// devBannerNodes are the notices above the page, or nothing: a slice so the
// layout can splice them in without changing its output when there are none.
// There are two kinds: the runtime switches that change what a tenant sees, and,
// while a developer is viewing as another role (DEV-22), who they are really and
// how to leave.
func devBannerNodes(b *mi.Builder, cfg Config, t func(key string, args ...any) string) []interface{} {
	if t == nil {
		return nil
	}
	isDeveloper := cfg.UserRole == auth.RoleDeveloper || cfg.RealRole == auth.RoleDeveloper
	var out []interface{}
	if len(cfg.DevActive) > 0 {
		titles := make([]string, 0, len(cfg.DevActive))
		for _, k := range cfg.DevActive {
			titles = append(titles, developerFlagText(t, k, "title", k))
		}
		parts := []interface{}{
			mi.Class("dev-banner"), mi.Attr("role", "status"),
			b.Span(mi.Class("material-icons"), "code"),
			b.Span(t("developer.banner.active", strings.Join(titles, ", "))),
		}
		if isDeveloper {
			parts = append(parts, b.A(mi.Href("/developer/overview"), mi.Class("dev-linkbtn"), t("developer.banner.manage")))
		}
		out = append(out, b.Div(parts...))
	}
	if cfg.RealRole != "" {
		out = append(out, b.Div(mi.Class("dev-banner dev-banner-viewas"), mi.Attr("role", "status"),
			b.Span(mi.Class("material-icons"), "visibility"),
			b.Span(t("developer.viewas.banner", t("user.role."+cfg.UserRole))),
			b.A(mi.Href("/developer/view-as/exit"), mi.Class("dev-linkbtn"), t("developer.viewas.exit"))))
	}
	return out
}
