// Package ui provides server-rendered HTML views for Seam AMS.
//
// The UI is built using minty for HTML generation and includes:
//   - Base layouts with navigation and theming
//   - Page templates for each major section
//   - Reusable components (cards, tables, forms)
//   - HTMX integration for dynamic updates
package ui

import (
	"encoding/json"
	"strings"

	mi "github.com/ha1tch/minty"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/devruntime"
	"github.com/ha1tch/seam-ui/internal/features"
	"github.com/ha1tch/seam-ui/internal/i18n"
	"github.com/ha1tch/seam-ui/internal/modules"
)

// Config holds UI configuration.
type Config struct {
	AppName           string
	Version           string
	Locale            string
	TenantID          string
	TenantName        string
	UserName          string
	UserRole          string
	CSRFToken         string
	BaseURL           string
	StaticURL         string
	DarkMode          bool
	DevMode           bool                // DevMode enables development fallbacks - MUST be false in production
	UseVendoredJS     bool                // UseVendoredJS serves JS deps from /static/vendor/ instead of CDN
	FullHeight        bool                // FullHeight removes main content padding/scroll for full-height pages (e.g. editors)
	ExtraHead         mi.H                // ExtraHead is injected at the end of <head> (e.g. import maps)
	EnableOperational bool                // EnableOperational registers operational UI routes (assets, events, sensors, alerts)
	EnableAdmin       bool                // EnableAdmin registers admin UI routes (asset types, FSM, rules, user management)
	EnableAccounting  bool                // EnableAccounting mounts the Accounting & Admin module (/accounting), opt-in, see internal/config.FeatureConfig
	RowEffects        string              // RowEffects: "scoped" (default, Demo page only), "all" or "off"; see rowFxMode
	EnableDemo        bool                // EnableDemo registers the Demo module (/demo), opt-in, see internal/config.FeatureConfig
	EnableDeveloper   bool                // EnableDeveloper registers the Developer module (/developer), opt-in, requires auth, see internal/config.FeatureConfig
	Flags             *features.Set       // resolved feature flags (internal/features); read by the Developer module
	DevRuntime        *devruntime.Runtime // Developer module runtime; nil unless EnableDeveloper
	DevSettings       DeveloperSettings   // the Developer module's settings that are not on/off (developer_settings.go)
	ViewerTenantID    int64               // render-time only: the viewer's tenant, set by PageHandler.viewerConfig; never configuration
	DevActive         []string            // render-time only: the behaviour-changing Developer switches on for the viewer's tenant (developer_banner.go)
	RealRole          string              // render-time only: the role the session really has, set only while a developer is viewing as another role (DEV-22)
	EnableWorkOrders  bool                // EnableWorkOrders registers the Work Orders playground module (2026-08-04, opt-in, see internal/config.FeatureConfig's own field doc for why)
	EnableIssues      bool                // EnableIssues registers the Issues module (opt-in like EnableWorkOrders, see internal/config.FeatureConfig)
	EnableMap         bool                // EnableMap shows the Mapa nav item and /map page; toggleable from settings
	SeamAPIURL        string              // SeamAPIURL is the base URL of seam-api for runtime state lookups (empty = disabled)
	QRBaseURL         string              // QRBaseURL is the public prefix the per-asset QR codes encode (empty = in-app /s/{code})
	QRRenderURL       string              // QRRenderURL is the base URL of the 2D-code renderer (empty = no QR images)

	// VisibleModules is the module selector's own data (MODULE_SPEC.md
	// §4.2) -- computed fresh per request by PageHandler.Render from
	// ModuleRegistry.VisibleTo(claims.Role), not stored persistently on
	// the shared Config PageHandler holds (unlike every other Config
	// field, this one varies per viewer, not per tenant). Nil/empty
	// renders no selector at all, not an empty one -- MOD-4 doesn't
	// require every page to show it, just support pages that do.
	VisibleModules []modules.Module

	// Translator resolves nav/chrome i18n keys against the current
	// Locale. A shared, locale-independent object (ForLocale(cfg.Locale)
	// does the per-request work) -- set once at UIHandler construction,
	// not per-request. Nil-safe: BaseLayout falls back to literal text
	// exactly like every other resolver in this codebase when nil.
	Translator *i18n.Translator
}

// DefaultConfig returns default UI configuration.
func DefaultConfig() Config {
	return Config{
		AppName:           "Seam AMS",
		Version:           "0.1.0",
		Locale:            "en",
		BaseURL:           "",
		StaticURL:         "/static",
		UseVendoredJS:     false, // CDN by default; set true to use vendored copies
		EnableOperational: true,
		EnableAdmin:       true,
		EnableMap:         true,
		// VisibleModules (2026-08-06, SIDEBAR_SYNC_PLAN_2026-08-06.md):
		// activeModuleSidebarItems needs a real module to match
		// activePath against -- without this, DefaultConfig()-based
		// tests silently render an empty sidebar regardless of what
		// they're actually testing (found exactly this way:
		// TestMapNavItemHiddenWhenDisabled was passing because
		// nothing rendered at all, not because Map was specifically
		// hidden while other items correctly remained). Deliberately
		// minimal, not a mirror of production's real Assets
		// registration in internal/server/server.go -- these tests
		// exercise BaseLayout's own rendering, not the full module
		// registry, and internal/ui can't import internal/server to
		// share the real list anyway (internal/server already imports
		// internal/ui). Just enough for "/" to resolve as the active
		// module with a "/map" item present, which is what every
		// current DefaultConfig()-based sidebar test actually needs.
		VisibleModules: []modules.Module{
			{
				ID:   "assets",
				Path: "/",
				SidebarItems: []modules.SidebarItem{
					{Label: "nav.section.operational", Section: true},
					{Label: "nav.dashboard", URL: "/", Icon: "dashboard"},
					{Label: "nav.map", URL: "/map", Icon: "map"},
				},
			},
		},
	}
}

// litVendorFile is the vendored Lit ESM bundle served from /static/vendor/.
const litVendorFile = "lit@3.js"

// LitImportMap renders a minimal import map providing only the Lit
// dependency needed by the FSM editor Lit shell.
func LitImportMap(cfg Config) mi.H {
	return func(b *mi.Builder) mi.Node {
		url := cfg.StaticURL + "/vendor/" + litVendorFile
		return b.Script(
			mi.Type("importmap"),
			mi.Raw(`{"imports":{"lit":"`+url+`"}}`),
		)
	}
}

// NavItem represents a navigation menu item.
type NavItem struct {
	Label    string
	URL      string
	Icon     string
	Active   bool
	Badge    string
	Children []NavItem
	Section  bool // if true, renders as a section label, not a link
}

// Agregado por equipo de WO 22/10/2026
func matchesNavigationPath(path, prefix string) bool {
	if prefix == "" {
		return false
	}
	if prefix == "/" {
		return strings.HasPrefix(path, "/")
	}
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

// activeModuleSidebarItems replaces MainNav (2026-08-06,
// SIDEBAR_SYNC_PLAN_2026-08-06.md): rather than one static, 19-item
// list shown regardless of which module the request is actually in,
// this finds whichever module in cfg.VisibleModules is active (its
// own Path is the longest matching prefix of activePath -- Assets'
// own Path, "/", is the universal fallback at length 1, correctly
// losing to any more specific module whose Path also matches) and
// converts that module's own declared SidebarItems into NavItems,
// computing Active fresh per request exactly as MainNav's own static
// list used to. A module with no SidebarItems (Documents, deliberately)
// simply returns nil -- Sidebar/MobileSidebar already render an empty
// list as no section at all, so this needs no special case.
// activeModule finds the module of cfg.VisibleModules that activePath belongs
// to, or nil: the one whose Path, or one of whose sidebar addresses, is the
// longest match. The sidebar (activeModuleSidebarItems) and the module
// selector (which marks it as the current one) both ask here.
func activeModule(activePath string, cfg Config) *modules.Module {
	var active *modules.Module
	longest := -1

	// Modificado por equipo de WO 22/10/2026
	for i := range cfg.VisibleModules {
		m := &cfg.VisibleModules[i]

		// La ruta principal también identifica al módulo.
		prefixes := []string{m.Path}

		// Sus opciones del sidebar identifican las otras rutas
		// que pertenecen al mismo módulo.
		for _, si := range m.SidebarItems {
			if si.Section || si.URL == "" || si.URL == "/" {
				continue
			}
			prefixes = append(prefixes, si.URL)
		}

		for _, prefix := range prefixes {
			if matchesNavigationPath(activePath, prefix) &&
				len(prefix) > longest {
				active = m
				longest = len(prefix)
			}
		}
	}
	// Anteriormente era:
	// for i := range cfg.VisibleModules {
	// 	m := &cfg.VisibleModules[i]
	// 	if hasPrefix(activePath, m.Path) && len(m.Path) > longest {
	// 		active = m
	// 		longest = len(m.Path)
	// 	}
	// }
	return active
}

func activeModuleSidebarItems(activePath string, cfg Config, t func(key string, args ...any) string) []NavItem {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	active := activeModule(activePath, cfg)
	if active == nil {
		return nil
	}
	// Resolve the single active sidebar item first. /issues and
	// /issues/new both prefix-match /issues/new, so marking every match
	// lit up two entries at once. The longest matching href wins, which
	// keeps /issues/42 on Incidencias and /issues/new on Nueva Incidencia.
	activeHref := ""
	for _, si := range active.SidebarItems {
		if si.Section || si.URL == "" {
			continue
		}
		if si.URL == "/map" && !cfg.EnableMap {
			continue
		}
		if si.RequiredPerm != "" && !auth.HasPermission(cfg.UserRole, auth.Permission(si.RequiredPerm)) {
			continue
		}
		if navItemMatchesPath(activePath, si.URL) && len(si.URL) > len(activeHref) {
			activeHref = si.URL
		}
	}
	items := make([]NavItem, 0, len(active.SidebarItems))
	for _, si := range active.SidebarItems {
		// Map's own visibility is runtime-mutable per request (toggled
		// from Settings, h.config.EnableMap -- confirmed by checking,
		// not assumed: routes.go mutates it live, it's not a startup
		// flag), so it can't be baked into the static SidebarItems
		// slice the way every other item correctly is. The original
		// MainNav handled this the same way, just inline rather than
		// as a filter -- this is that same one-off, not new
		// complexity, kept explicit rather than building a generic
		// "conditional item" mechanism for what is genuinely one case.
		if si.URL == "/map" && !cfg.EnableMap {
			continue
		}
		if si.Section {
			items = append(items, NavItem{Label: t(si.Label), Section: true})
			continue
		}
		if si.RequiredPerm != "" && !auth.HasPermission(cfg.UserRole, auth.Permission(si.RequiredPerm)) {
			continue
		}
		items = append(items, NavItem{
			Label:  t(si.Label),
			URL:    si.URL,
			Icon:   si.Icon,
			Active: si.URL != "" && si.URL == activeHref,
		})
	}
	return items
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// navItemMatchesPath reports whether activePath is the sidebar item's own
// page or a descendant of it. The boundary check keeps "/issues" from
// matching "/issues-archive"; the longest-match pass above keeps nested
// items such as "/issues" and "/issues/new" from both being active.
func navItemMatchesPath(activePath, href string) bool {
	if href == "/" {
		return activePath == "/"
	}
	return activePath == href || strings.HasPrefix(activePath, href+"/")
}

// Create dark mode handler using minty's built-in support
var darkMode = mi.DarkModeTailwind(
	mi.DarkModeStorage("seam-dark-mode"),
	mi.DarkModeDefault("system"),
	// Use Material Icons glyphs instead of inline SVG.
	// DarkModeSVGIcons() emits SVGs with class="w-5 h-5"; those classes are
	// absent from the Tailwind-purged CSS (minty is not in the scan path),
	// making the icon invisible. Material Icons renders via the vendored font
	// and has no CSS dimension dependency.
	//   dark_mode  = crescent moon — shown in light mode ("switch to dark")
	//   light_mode = sun           — shown in dark mode  ("switch to light")
	mi.DarkModeIcons(
		`<span class="material-icons text-xl">dark_mode</span>`,
		`<span class="material-icons text-xl">light_mode</span>`,
	),
)

// extraHead renders optional extra head content, or nothing if nil.
func extraHead(b *mi.Builder, h mi.H) mi.Node {
	if h == nil {
		return b.Span()
	}
	return h(b)
}

func mainContentClass(cfg Config) string {
	if cfg.FullHeight {
		return "flex-1 overflow-hidden flex flex-col pt-16 lg:pt-0"
	}
	return "flex-1 overflow-y-auto p-4 lg:p-6 pt-16 lg:pt-6"
}

// PublicLayout creates a minimal layout for public pages (QR scan, etc).
func PublicLayout(cfg Config, title string, content mi.H) mi.H {
	return func(b *mi.Builder) mi.Node {
		fullTitle := title
		if title != "" {
			fullTitle = title + " | " + cfg.AppName
		} else {
			fullTitle = cfg.AppName
		}

		cancelLabel := "Cancel"
		if cfg.Translator != nil {
			if cl := chromeTranslate(cfg)("action.cancel"); cl != "" && cl != "action.cancel" {
				cancelLabel = cl
			}
		}

		return b.Html(mi.Lang(cfg.Locale), mi.Attr("data-i18n-cancel", cancelLabel), mi.Attr("data-i18n-texts", scriptTextsFor(cfg)),
			b.Head(
				b.Meta(mi.Charset("utf-8")),
				b.Meta(mi.Name("viewport"), mi.Content("width=device-width, initial-scale=1")),
				b.Title(fullTitle),
				b.Link(mi.Rel("icon"), mi.Type("image/png"), mi.Attr("sizes", "32x32"),
					mi.Href(cfg.StaticURL+"/icons/icon-32.png")),
				b.Link(mi.Rel("icon"), mi.Type("image/png"), mi.Attr("sizes", "32x32"),
					mi.Href(cfg.StaticURL+"/icons/icon-32-dark.png"),
					mi.Attr("media", "(prefers-color-scheme: dark)")),
				b.Link(mi.Rel("shortcut icon"), mi.Href(cfg.StaticURL+"/favicon.ico")),
				b.Link(mi.Rel("apple-touch-icon"), mi.Attr("sizes", "180x180"),
					mi.Href(cfg.StaticURL+"/icons/icon-180.png")),
				b.Link(mi.Rel("apple-touch-icon"), mi.Attr("sizes", "180x180"),
					mi.Href(cfg.StaticURL+"/icons/icon-180-dark.png"),
					mi.Attr("media", "(prefers-color-scheme: dark)")),
				b.Link(mi.Rel("stylesheet"), mi.Href(cfg.StaticURL+"/css/tailwind.css")),
				darkMode.Script(b),
				// Apply saved transition duration before first paint (no-flash IIFE)
				b.Script(mi.Raw(`(function(){
					var d=localStorage.getItem('seam-transition');
					if(d)document.documentElement.style.setProperty('--seam-transition',d);
				})()`)),
				b.Link(mi.Rel("stylesheet"), mi.Href(cfg.StaticURL+"/css/app.css")),
				b.Link(mi.Rel("stylesheet"), mi.Href(cfg.StaticURL+"/vendor/material-icons.css")),
			),
			b.Body(mi.Class("bg-gray-50 dark:bg-gray-900 min-h-screen"),
				b.Div(mi.Class("max-w-lg mx-auto px-4 py-8"),
					// Logo header
					b.Div(mi.Class("text-center mb-8"),
						b.A(mi.Href("/"), mi.Class("inline-flex flex-col items-center gap-2"),
							b.Img(
								mi.Src(cfg.StaticURL+"/images/logo-160.png"),
								mi.Alt(cfg.AppName),
								mi.Class("w-28 h-28 object-contain dark:hidden"),
							),
							b.Img(
								mi.Src(cfg.StaticURL+"/images/logo-160-dark.png"),
								mi.Alt(cfg.AppName),
								mi.Class("w-28 h-28 object-contain hidden dark:block"),
							),
						),
					),
					// Content
					content(b),
				),
			),
		)
	}
}

// chromeTranslate is the translation function of the navigation and layout
// chrome (the page's own text uses UIHandler.translateFunc). With the
// Developer module on and a tenant known, it is that tenant's view, so the
// module's per-tenant switches (show translation keys, filesystem and database
// overrides) reach the chrome too (DEV-31); otherwise it is the global lookup,
// exactly as before. The caller has checked cfg.Translator is not nil.
func chromeTranslate(cfg Config) func(key string, args ...any) string {
	if cfg.DevRuntime != nil && cfg.ViewerTenantID > 0 {
		return cfg.Translator.ForTenant(cfg.ViewerTenantID).ForLocale(cfg.Locale)
	}
	return cfg.Translator.ForLocale(cfg.Locale)
}

// chromeText is a text of the page's own frame (header, menus, skip link) in
// the viewer's language: the translation of key, or fallback, in English,
// when there is no translator or the key is missing.
func chromeText(cfg Config, key, fallback string) string {
	if cfg.Translator == nil {
		return fallback
	}
	if s := chromeTranslate(cfg)(key); s != "" && s != key {
		return s
	}
	return fallback
}

// scriptTexts are the texts the browser scripts show (SeamText in app.js):
// the name a script asks for, and its key in the locale files.
var scriptTexts = [][2]string{
	{"copied", "js.copied"}, {"copy-failed", "js.copy_failed"}, {"load-failed", "js.load_failed"},
	{"select-item", "js.select_item"}, {"confirm", "js.confirm"}, {"confirm-delete", "js.confirm_delete"},
	{"delete", "action.delete"}, {"ok", "js.ok"}, {"delete-named", "js.delete_named"},
	{"upload-load-failed", "js.upload.load_failed"}, {"upload-failed", "js.upload.failed"}, {"upload-delete-failed", "js.upload.delete_failed"},
}

// scriptTextsJSON is scriptTexts in the viewer's language, as the JSON object
// the layout puts in <html data-i18n-texts>. A text with no translation is
// left out, and the script shows its own English.
func scriptTextsJSON(t func(key string, args ...any) string) string {
	texts := map[string]string{}
	if t != nil {
		for _, st := range scriptTexts {
			if s := t(st[1]); s != "" && s != st[1] {
				texts[st[0]] = s
			}
		}
	}
	out, err := json.Marshal(texts)
	if err != nil {
		return "{}"
	}
	return string(out)
}

// scriptTextsFor is scriptTextsJSON for the viewer of a page.
func scriptTextsFor(cfg Config) string {
	if cfg.Translator == nil {
		return scriptTextsJSON(nil)
	}
	return scriptTextsJSON(chromeTranslate(cfg))
}

// BaseLayout creates the base HTML layout for all pages.
func BaseLayout(cfg Config, title string, activePath string, content mi.H) mi.H {
	var t func(key string, args ...any) string
	if cfg.Translator != nil {
		t = chromeTranslate(cfg)
	}
	return func(b *mi.Builder) mi.Node {
		fullTitle := title
		if title != "" {
			fullTitle = title + " | " + cfg.AppName
		} else {
			fullTitle = cfg.AppName
		}

		cancelLabel := "Cancel"
		if t != nil {
			if cl := t("action.cancel"); cl != "" && cl != "action.cancel" {
				cancelLabel = cl
			}
		}

		return b.Html(mi.Lang(cfg.Locale), mi.Attr("data-i18n-cancel", cancelLabel), mi.Attr("data-i18n-texts", scriptTextsFor(cfg)),
			b.Head(
				b.Meta(mi.Charset("utf-8")),
				b.Meta(mi.Name("viewport"), mi.Content("width=device-width, initial-scale=1")),
				b.Meta(mi.Name("csrf-token"), mi.Content(cfg.CSRFToken)),
				b.Meta(mi.Name("theme-color"), mi.Content("#4f46e5")),
				b.Title(fullTitle),

				// Favicon — PNG at 32×32 for modern browsers, ICO for legacy
				b.Link(mi.Rel("icon"), mi.Type("image/png"), mi.Attr("sizes", "32x32"),
					mi.Href(cfg.StaticURL+"/icons/icon-32.png")),
				b.Link(mi.Rel("icon"), mi.Type("image/png"), mi.Attr("sizes", "32x32"),
					mi.Href(cfg.StaticURL+"/icons/icon-32-dark.png"),
					mi.Attr("media", "(prefers-color-scheme: dark)")),
				b.Link(mi.Rel("icon"), mi.Type("image/png"), mi.Attr("sizes", "16x16"),
					mi.Href(cfg.StaticURL+"/icons/icon-16.png")),
				b.Link(mi.Rel("shortcut icon"), mi.Href(cfg.StaticURL+"/favicon.ico")),
				b.Link(mi.Rel("apple-touch-icon"), mi.Attr("sizes", "180x180"),
					mi.Href(cfg.StaticURL+"/icons/icon-180.png")),
				b.Link(mi.Rel("apple-touch-icon"), mi.Attr("sizes", "180x180"),
					mi.Href(cfg.StaticURL+"/icons/icon-180-dark.png"),
					mi.Attr("media", "(prefers-color-scheme: dark)")),
				b.Link(mi.Rel("manifest"), mi.Href(cfg.StaticURL+"/manifest.json")),

				// Tailwind CSS — pre-generated by CLI, fully vendored
				b.Link(mi.Rel("stylesheet"), mi.Href(cfg.StaticURL+"/css/tailwind.css")),

				// Dark mode script (before app CSS so no flash)
				darkMode.Script(b),

				// App CSS overrides (after Tailwind so specificity wins)
				b.Link(mi.Rel("stylesheet"), mi.Href(cfg.StaticURL+"/css/app.css")),

				// Material Icons — vendored @font-face; class rules are in app.css
				b.Link(mi.Rel("stylesheet"), mi.Href(cfg.StaticURL+"/vendor/material-icons.css")),

				// HTMX — vendored
				b.Script(mi.Src(cfg.StaticURL+"/vendor/htmx@1.9.10.min.js")),

				// App JS
				b.Script(mi.Src(cfg.StaticURL+"/js/app.js"), mi.Defer()),

				// Nav-group collapse — runs synchronously in <head> before the
				// sidebar renders, so collapsed groups never flash visible.
				b.Script(mi.Raw(`(function(){try{var s=JSON.parse(localStorage.getItem('seam-nav-groups')||'{}'),c='';Object.keys(s).forEach(function(id){if(s[id]===false){c+='[data-nav-group-id="'+id+'"]{max-height:0!important;opacity:0!important;}[data-nav-arrow-id="'+id+'"]{transform:rotate(-90deg)!important;}';}});if(c){var el=document.createElement('style');el.textContent=c;document.head.appendChild(el);}}catch(e){}}());`)),

				// Extra head elements (e.g. import maps for specific pages)
				extraHead(b, cfg.ExtraHead),
			),
			b.Body(mi.Class("bg-gray-50 dark:bg-gray-900 min-h-screen"), mi.Attr("data-fe-fxmode", rowFxMode(cfg)),
				// Skip link for accessibility
				b.A(mi.Href("#main-content"), mi.Class("sr-only focus:not-sr-only"),
					chromeText(cfg, "layout.skip_to_content", "Skip to main content"),
				),

				// Mobile header (visible on small screens)
				MobileHeader(cfg)(b),

				// App shell
				b.Div(mi.Class("flex h-screen overflow-hidden"),
					// Sidebar (hidden on mobile, shown on lg+)
					Sidebar(cfg, activeModuleSidebarItems(activePath, cfg, t))(b),

					// Mobile sidebar overlay
					MobileSidebar(cfg, activeModuleSidebarItems(activePath, cfg, t))(b),

					// Main content area
					b.Div(append([]interface{}{mi.Class("flex-1 flex flex-col overflow-hidden lg:ml-64"),
						// Top header (hidden on mobile)
						TopHeader(cfg, activePath)(b)},
						// Developer switches that change what this tenant sees (DEV-10); nothing otherwise
						append(devBannerNodes(b, cfg, t),
							// Main content
							b.Main(mi.ID("main-content"), mi.Class(mainContentClass(cfg)),
								content(b),
							))...)...),
				),

				// Toast container for notifications
				b.Div(mi.ID("toast-container"), mi.Class("fixed bottom-4 right-4 z-50")),

				// Mobile menu + UI utilities script
				b.Script(mi.Raw(`
					// SeamUI — client-side UI utilities
					var SeamUI = {
						setTransition: function(val) {
							document.documentElement.style.setProperty('--seam-transition', val);
							localStorage.setItem('seam-transition', val);
							document.querySelectorAll('.seam-transition-select').forEach(function(el) {
								el.value = val;
							});
						}
					};
					function toggleMobileMenu() {
						var sidebar = document.getElementById('mobile-sidebar');
						var overlay = document.getElementById('mobile-overlay');
						if (sidebar.classList.contains('-translate-x-full')) {
							sidebar.classList.remove('-translate-x-full');
							overlay.classList.remove('hidden');
						} else {
							sidebar.classList.add('-translate-x-full');
							overlay.classList.add('hidden');
						}
					}
					// Issue 6: faster crossfade within same section
					document.addEventListener('htmx:beforeSwap', function(evt) {
						if (evt.detail.target.id !== 'main-content') return;
						var newPath = evt.detail.requestConfig && evt.detail.requestConfig.path || '';
						var curSec = window.location.pathname.split('/')[1] || '';
						var newSec = newPath.split('/')[1] || '';
						var saved = localStorage.getItem('seam-transition') || '500ms';
						var dur = (newSec && newSec === curSec) ? '200ms' : saved;
						document.documentElement.style.setProperty('--seam-transition', dur);
					});
					// HTMX partial navigation handlers
					document.addEventListener('htmx:afterSwap', function(evt) {
						if (evt.detail.target.id !== 'main-content') return;
						// 1. Restart entrance animation — only needed when View Transitions
						//    did NOT handle the swap (browser lacks support, e.g. Firefox).
						//    When transition:true is active, ::view-transition-new(seam-main)
						//    provides the crossfade; seam-page-enter would double-animate.
						if (!document.startViewTransition) {
							var mc = evt.detail.target;
							mc.style.animation = 'none';
							void mc.offsetWidth;
							mc.style.animation = '';
						}
						// 2. Update active nav state in both sidebars. Match the
						// single longest href so a nested page cannot light up
						// both its parent and its own entry.
						var path = window.location.pathname;
						var bestHref = '';
						document.querySelectorAll('[data-nav-href]').forEach(function(el) {
							var href = el.getAttribute('data-nav-href');
							if (!href) return;
							var matches = href === '/'
								? path === '/'
								: (path === href || path.startsWith(href + '/'));
							if (matches && href.length > bestHref.length) bestHref = href;
						});
						document.querySelectorAll('[data-nav-href]').forEach(function(el) {
							var isActive = el.getAttribute('data-nav-href') === bestHref;
							el.className = isActive
								? el.getAttribute('data-active-class')
								: el.getAttribute('data-inactive-class');
						});
						// 3. Close mobile menu if open
						var sb = document.getElementById('mobile-sidebar');
						var ov = document.getElementById('mobile-overlay');
						if (sb && !sb.classList.contains('-translate-x-full')) {
							sb.classList.add('-translate-x-full');
							if (ov) ov.classList.add('hidden');
						}
						// Restore saved duration after transition completes
						var _saved = localStorage.getItem('seam-transition') || '500ms';
						document.documentElement.style.setProperty('--seam-transition', _saved);
					});
				`)),
			),
		)
	}
}

// MobileHeader creates the mobile header with hamburger menu.
func MobileHeader(cfg Config) mi.H {
	return func(b *mi.Builder) mi.Node {
		return b.Header(mi.Class("lg:hidden fixed top-0 left-0 right-0 z-40 h-14 bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between px-4"),
			b.Button(
				mi.Type("button"),
				mi.Class("p-2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"),
				mi.Attr("onclick", "toggleMobileMenu()"),
				mi.Attr("aria-label", chromeText(cfg, "layout.open_menu", "Open menu")),
				b.Span(mi.Class("material-icons"), "menu"),
			),
			b.A(mi.Href("/"), mi.Class("block w-[125px] h-[30px] overflow-hidden"),
				b.Img(
					mi.Src(cfg.StaticURL+"/images/logo-landscape-sm.png"),
					mi.Alt(cfg.AppName),
					mi.Attr("width", "125"),
					mi.Attr("height", "30"),
					mi.Class("object-contain dark:hidden"),
				),
				b.Img(
					mi.Src(cfg.StaticURL+"/images/logo-landscape-sm-dark.png"),
					mi.Alt(cfg.AppName),
					mi.Attr("width", "125"),
					mi.Attr("height", "30"),
					mi.Class("object-contain hidden dark:block"),
				),
			),
			darkMode.Toggle(b,
				mi.Class("p-2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"),
			),
		)
	}
}

// MobileSidebar creates the mobile sidebar overlay.
func MobileSidebar(cfg Config, items []NavItem) mi.H {
	return func(b *mi.Builder) mi.Node {
		return mi.NewFragment(
			// Overlay
			b.Div(
				mi.ID("mobile-overlay"),
				mi.Class("hidden lg:hidden fixed inset-0 z-40 bg-black/50"),
				mi.Attr("onclick", "toggleMobileMenu()"),
			),
			// Sidebar
			b.Aside(
				mi.ID("mobile-sidebar"),
				mi.Class("lg:hidden fixed top-0 left-0 z-50 w-64 h-full bg-white dark:bg-gray-800 border-r border-gray-200 dark:border-gray-700 transform -translate-x-full transition-transform duration-200 ease-in-out"),
				// Header: tenant name + close button
				// Logo is already visible in the persistent top bar behind this overlay;
				// showing it again here was redundant. Tenant name gives operational context.
				b.Div(mi.Class("h-14 flex items-center justify-between px-4 border-b border-gray-200 dark:border-gray-700"),
					b.Div(mi.Class("flex flex-col min-w-0"),
						b.Span(mi.Class("text-xs text-gray-400 dark:text-gray-500 leading-none mb-0.5"), "workspace"),
						b.Span(mi.Class("text-sm font-bold text-indigo-600 dark:text-indigo-400 truncate leading-tight"),
							cfg.TenantName,
						),
					),
					b.Button(
						mi.Type("button"),
						mi.Class("p-2 flex-shrink-0 text-gray-500 hover:text-gray-700 dark:text-gray-400"),
						mi.Attr("onclick", "toggleMobileMenu()"),
						b.Span(mi.Class("material-icons"), "close"),
					),
				),
				// Navigation
				b.Nav(mi.Class("py-4 px-3 overflow-y-auto flex-1"),
					b.Ul(append([]interface{}{mi.Class("space-y-1")}, navItems(b, items)...)...),
				),
				// Footer: logout link
				b.Div(mi.Class("border-t border-gray-200 dark:border-gray-700 p-3"),
					b.A(mi.Href("/logout"),
						mi.Class("flex items-center gap-2 px-3 py-2 text-sm text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg"),
						b.Span(mi.Class("material-icons text-base"), "logout"),
						chromeText(cfg, "layout.logout", "Log Out"),
					),
				),
			),
		)
	}
}

// Sidebar creates the navigation sidebar (hidden on mobile).
func Sidebar(cfg Config, items []NavItem) mi.H {
	return func(b *mi.Builder) mi.Node {
		return b.Aside(
			mi.ID("main-sidebar"),
			mi.Class("hidden lg:flex w-64 bg-white dark:bg-gray-800 border-r border-gray-200 dark:border-gray-700 flex-col fixed left-0 top-0 h-full z-30"),
			// Logo/Brand — landscape logo (icon + brand name in one image)
			// overflow-hidden prevents the dual-img pair from spilling into
			// the main content area if both images briefly render during a
			// view-transition capture (combined width = 300px > 256px sidebar).
			b.Div(mi.Class("h-16 flex items-center justify-center px-4 border-b border-gray-200 dark:border-gray-700 overflow-hidden"),
				b.A(mi.Href("/"), mi.Class("block w-[150px] h-[36px] overflow-hidden"),
					b.Img(
						mi.Src(cfg.StaticURL+"/images/logo-landscape.png"),
						mi.Alt(cfg.AppName),
						mi.Attr("width", "150"),
						mi.Attr("height", "36"),
						mi.Class("object-contain dark:hidden"),
					),
					b.Img(
						mi.Src(cfg.StaticURL+"/images/logo-landscape-dark.png"),
						mi.Alt(cfg.AppName),
						mi.Attr("width", "150"),
						mi.Attr("height", "36"),
						mi.Class("object-contain hidden dark:block"),
					),
				),
			),

			// Navigation
			b.Nav(mi.Class("flex-1 py-4 px-3 overflow-y-auto"),
				b.Ul(append([]interface{}{mi.Class("space-y-1")}, navItems(b, items)...)...),
			),

			// Version footer
			b.Div(mi.Class("px-4 py-3 border-t border-gray-200 dark:border-gray-700"),
				b.Div(mi.Class("text-xs text-gray-400 dark:text-gray-500"),
					"v"+cfg.Version,
				),
			),
		)
	}
}

// navGroupID converts a section label to a stable localStorage key.
// Must produce the same value in JS and Go for the flash-free init script.
func navGroupID(label string) string {
	r := strings.NewReplacer(
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u",
		"ñ", "n", "ü", "u",
		" ", "-", "(", "", ")", "",
	)
	return strings.ToLower(r.Replace(label))
}

// navItems groups the flat NavItem list by Section headers and renders each
// group as a collapsible block.
func navItems(b *mi.Builder, items []NavItem) []interface{} {
	type navGroup struct {
		label     string
		id        string
		children  []NavItem
		hasActive bool
	}

	var groups []navGroup
	idx := -1

	for _, item := range items {
		if !item.Section && item.URL == "" && item.Label == "" {
			continue // zero-value placeholder — feature disabled
		}
		if item.Section {
			groups = append(groups, navGroup{label: item.Label, id: navGroupID(item.Label)})
			idx = len(groups) - 1
		} else if idx >= 0 {
			groups[idx].children = append(groups[idx].children, item)
			if item.Active {
				groups[idx].hasActive = true
			}
		}
	}

	result := make([]interface{}, len(groups))
	for i, g := range groups {
		result[i] = renderNavGroup(b, g.label, g.id, g.children, g.hasActive)
	}
	return result
}

// renderNavGroup renders a single collapsible nav group.
func renderNavGroup(b *mi.Builder, label, id string, children []NavItem, hasActive bool) mi.Node {
	activeStr := "false"
	if hasActive {
		activeStr = "true"
	}

	childNodes := make([]interface{}, len(children))
	for i, child := range children {
		childNodes[i] = navItem(b, child)
	}

	return b.Li(
		// Group header — clickable, full-width button
		b.Button(
			mi.Type("button"),
			mi.Class("w-full flex items-center gap-1 px-3 pt-4 pb-1 text-xs font-semibold text-gray-400 dark:text-gray-500 uppercase tracking-wider hover:text-gray-600 dark:hover:text-gray-300 transition-colors"),
			mi.Attr("data-nav-group", id),
			mi.Attr("data-nav-group-active", activeStr),
			mi.Attr("onclick", "SeamNav.toggle('"+id+"')"),
			b.Span(mi.Class("flex-1 text-left"), label),
			b.Span(
				mi.Attr("data-nav-arrow-id", id),
				mi.Class("material-icons text-base leading-none transition-transform duration-200"),
				"expand_more",
			),
		),
		// Group item list — max-height driven by JS for smooth collapse
		b.Ul(
			append([]interface{}{
				mi.Attr("data-nav-group-id", id),
				mi.Class("space-y-1 overflow-hidden"),
				// Default open; JS overrides immediately from localStorage
				mi.Style("max-height:600px;opacity:1;transition:max-height 200ms ease,opacity 150ms ease;"),
			}, childNodes...)...,
		),
	)
}

func navItem(b *mi.Builder, item NavItem) mi.Node {
	baseClass := "flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium transition-colors"
	activeClass := baseClass + " bg-indigo-50 text-indigo-700 dark:bg-indigo-900/50 dark:text-indigo-300"
	inactiveClass := baseClass + " text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-700"

	class := inactiveClass
	if item.Active {
		class = activeClass
	}

	children := []interface{}{
		mi.Href(item.URL),
		mi.Class(class),
		// HTMX partial navigation: swap only #main-content, preserve sidebar/topbar
		mi.Attr("hx-get", item.URL),
		mi.Attr("hx-target", "#main-content"),
		// transition:true wraps the swap in document.startViewTransition()
		// giving a true same-document crossfade via ::view-transition-old/new(seam-main)
		mi.Attr("hx-swap", "innerHTML transition:true"),
		mi.Attr("hx-push-url", "true"),
		// Data attrs for client-side active-state update after HTMX swap
		mi.Attr("data-nav-href", item.URL),
		mi.Attr("data-active-class", activeClass),
		mi.Attr("data-inactive-class", inactiveClass),
	}

	if item.Icon != "" {
		children = append(children,
			b.Span(mi.Class("material-icons text-lg"), item.Icon),
		)
	}

	children = append(children, b.Span(item.Label))

	if item.Badge != "" {
		children = append(children,
			b.Span(mi.Class("ml-auto bg-red-500 text-white text-xs px-2 py-0.5 rounded-full"),
				item.Badge,
			),
		)
	}

	return b.Li(b.A(children...))
}

// TopHeader creates the top header bar (hidden on mobile).
func TopHeader(cfg Config, activePath string) mi.H {
	t := func(key string, args ...any) string { return key }
	if cfg.Translator != nil {
		t = chromeTranslate(cfg)
	}
	return func(b *mi.Builder) mi.Node {
		return b.Header(mi.Class("hidden lg:flex h-16 bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 items-center justify-between px-6"),
			// Tenant name — left side
			b.Div(mi.Class("flex items-center gap-2 min-w-0"),
				b.Span(mi.Class("material-icons text-indigo-500 text-xl flex-shrink-0"), "domain"),
				b.Span(mi.Class("text-base font-semibold text-gray-800 dark:text-gray-100 truncate"),
					cfg.TenantName,
				),
			),

			// Right side: search + actions
			b.Div(mi.Class("flex items-center gap-4"),
				// Search — right-aligned
				b.Form(mi.Action("/search"), mi.Method("get"), mi.Class("relative"),
					b.Input(
						mi.Type("search"),
						mi.Name("q"),
						mi.Placeholder(t("layout.search.placeholder")),
						mi.Class("w-64 pl-10 pr-4 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-gray-50 dark:bg-gray-700 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"),
					),
					b.Span(mi.Class("absolute left-3 top-1/2 -translate-y-1/2 material-icons text-gray-400 text-lg"),
						"search",
					),
				),

				// Notifications
				b.Button(
					mi.Type("button"),
					mi.Class("relative p-2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"),
					mi.Attr("aria-label", chromeText(cfg, "layout.notifications", "Notifications")),
					b.Span(mi.Class("material-icons"), "notifications"),
					b.Span(mi.Class("absolute top-1 right-1 w-2 h-2 bg-red-500 rounded-full")),
				),

				// Dark mode toggle (using minty's built-in)
				darkMode.Toggle(b,
					mi.Class("p-2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 rounded-lg"),
				),

				// Module selector (MODULE_SPEC.md §4.3 -- placed here,
				// left of the user menu, next to the dark-mode toggle)
				ModuleSelector(cfg.VisibleModules, activeModuleID(activePath, cfg), t)(b),

				// User menu — avatar button opens dropdown
				b.Div(mi.Class("relative"),
					b.Button(
						mi.Type("button"),
						mi.ID("seam-user-btn"),
						mi.Class("flex items-center gap-2 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg px-2 py-1 focus:outline-none"),
						mi.Attr("onclick", "SeamUserMenu.toggle()"),
						b.Div(mi.Class("w-8 h-8 rounded-full bg-indigo-500 flex items-center justify-center text-white text-sm font-semibold select-none"),
							userInitials(cfg.UserName),
						),
						b.Span(mi.Class("text-sm font-medium text-gray-700 dark:text-gray-300 hidden xl:block"),
							cfg.UserName,
						),
						b.Span(mi.Class("material-icons text-sm text-gray-400"), "expand_more"),
					),
					// Dropdown panel (hidden by default)
					b.Div(
						mi.ID("seam-user-menu"),
						mi.Style("display:none"),
						mi.Class("absolute right-0 top-full mt-1 w-48 bg-white dark:bg-gray-800 rounded-lg shadow-lg border border-gray-200 dark:border-gray-700 py-1 z-50"),
						b.A(mi.Href("/profile"),
							mi.Class("flex items-center gap-2 px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700"),
							b.Span(mi.Class("material-icons text-base"), "person"),
							chromeText(cfg, "layout.user_profile", "User Profile"),
						),
						b.Div(mi.Class("border-t border-gray-100 dark:border-gray-700 my-1")),
						b.A(mi.Href("/logout"),
							mi.Class("flex items-center gap-2 px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700"),
							b.Span(mi.Class("material-icons text-base"), "logout"),
							chromeText(cfg, "layout.logout", "Log Out"),
						),
					),
				),
			),
		)
	}
}

func userInitials(name string) string {
	if name == "" {
		return "?"
	}
	// Get first letter of first and last name
	parts := splitWords(name)
	if len(parts) >= 2 {
		return string(parts[0][0]) + string(parts[len(parts)-1][0])
	}
	return string(name[0])
}

func splitWords(s string) []string {
	var words []string
	var word string
	for _, c := range s {
		if c == ' ' {
			if word != "" {
				words = append(words, word)
				word = ""
			}
		} else {
			word += string(c)
		}
	}
	if word != "" {
		words = append(words, word)
	}
	return words
}
