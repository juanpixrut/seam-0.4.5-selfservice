package ui

import (
	mi "github.com/ha1tch/minty"

	"github.com/ha1tch/seam-ui/internal/modules"
)

// ModuleSelector renders the module menu of the top bar (MODULE_SPEC.md §4.1):
// a button that opens a panel titled "Modules" with one tile per application
// module (icon, name and a line saying what it is for) and, under a rule, the
// system modules (Settings, Administration) as plain links. current is the ID
// of the module the page belongs to, which is marked; "" marks none.
//
// visible is already role-filtered (Registry.VisibleFor, §4.2): this function
// does no access checking of its own, it only groups and renders what it is
// given. A module whose Group is neither "apps" nor "system" is a
// configuration mistake and is not rendered. Nothing at all is rendered when
// there is nothing left to choose from.
//
// The look is in app.css (.seam-mod-*); SeamModuleSelector (app.js) opens and
// closes the panel.
func ModuleSelector(visible []modules.Module, current string, t func(key string, args ...any) string) mi.H {
	return func(b *mi.Builder) mi.Node {
		var apps, system []modules.Module
		for _, m := range visible {
			if m.HideFromSelector {
				// Real access, real routes: just not a second entry beside
				// chrome that already covers it (Account lives in the
				// user-avatar menu).
				continue
			}
			switch m.Group {
			case "apps":
				apps = append(apps, m)
			case "system":
				system = append(system, m)
			}
		}
		if len(apps) == 0 && len(system) == 0 {
			return b.Span()
		}

		panel := []interface{}{
			mi.ID("seam-module-menu"),
			mi.Style("display:none"),
			mi.Class(moduleSelectorPanelClass(len(apps))),
		}
		if len(apps) > 0 {
			tiles := []interface{}{mi.Class("seam-mod-grid")}
			for _, m := range apps {
				tiles = append(tiles, moduleSelectorTile(b, m, m.ID == current, t))
			}
			panel = append(panel,
				b.Div(mi.Class("seam-mod-title"), t("nav.module_group.apps")),
				b.Div(tiles...),
			)
		}
		if len(system) > 0 {
			links := []interface{}{mi.Class("seam-mod-foot")}
			for _, m := range system {
				links = append(links, moduleSelectorLink(b, m, m.ID == current, t))
			}
			panel = append(panel, b.Div(links...))
		}

		return b.Div(mi.Class("relative"),
			b.Button(
				mi.Type("button"),
				mi.ID("seam-module-btn"),
				mi.Class("seam-mod-btn"),
				mi.Attr("aria-label", t("nav.module_selector")),
				mi.Attr("aria-haspopup", "true"),
				mi.Attr("aria-expanded", "false"),
				mi.Attr("aria-controls", "seam-module-menu"),
				mi.Attr("onclick", "SeamModuleSelector.toggle()"),
				b.Span(mi.Class("material-icons seam-mod-btn-icon"), "apps"),
				b.Span(mi.Class("seam-mod-btn-label"), t("nav.module_group.apps")),
				b.Span(mi.Class("material-icons seam-mod-btn-caret"), "expand_more"),
			),
			b.Div(panel...),
		)
	}
}

// moduleSelectorPanelClass sizes the panel for what it holds: two columns of
// tiles, or one narrow column when there is a single application (or none).
func moduleSelectorPanelClass(apps int) string {
	if apps < 2 {
		return "seam-mod-panel seam-mod-panel--narrow"
	}
	return "seam-mod-panel"
}

// moduleSelectorTile is one application module: its icon on a tinted chip, its
// name and, when the language has one (<Label>.desc), a line about it.
func moduleSelectorTile(b *mi.Builder, m modules.Module, current bool, t func(key string, args ...any) string) mi.Node {
	text := []interface{}{mi.Class("seam-mod-text"), b.Span(mi.Class("seam-mod-name"), t(m.Label))}
	if key := m.Label + ".desc"; t(key) != key && t(key) != "" {
		text = append(text, b.Span(mi.Class("seam-mod-desc"), t(key)))
	}
	attrs := []interface{}{mi.Href(m.Path), mi.Class("seam-mod-tile")}
	if current {
		attrs = []interface{}{mi.Href(m.Path), mi.Class("seam-mod-tile is-current"), mi.Attr("aria-current", "page")}
	}
	return b.A(append(attrs,
		b.Span(mi.Class("seam-mod-chip"), b.Span(mi.Class("material-icons"), mi.Attr("aria-hidden", "true"), m.Icon)),
		b.Span(text...),
	)...)
}

// moduleSelectorLink is one system module, in the row under the tiles.
func moduleSelectorLink(b *mi.Builder, m modules.Module, current bool, t func(key string, args ...any) string) mi.Node {
	attrs := []interface{}{mi.Href(m.Path), mi.Class("seam-mod-link")}
	if current {
		attrs = []interface{}{mi.Href(m.Path), mi.Class("seam-mod-link is-current"), mi.Attr("aria-current", "page")}
	}
	return b.A(append(attrs,
		b.Span(mi.Class("material-icons"), mi.Attr("aria-hidden", "true"), m.Icon),
		b.Span(t(m.Label)),
	)...)
}

// activeModuleID is the ID of the module activePath belongs to, or "".
func activeModuleID(activePath string, cfg Config) string {
	if m := activeModule(activePath, cfg); m != nil {
		return m.ID
	}
	return ""
}
