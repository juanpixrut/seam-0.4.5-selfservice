package ui

import (
	"bytes"
	"net/http"

	mi "github.com/ha1tch/minty"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/features"
	"github.com/ha1tch/seam-ui/internal/modules"
)

// PageHandler handles rendering pages with the base layout.
type PageHandler struct {
	Config Config

	// ModuleRegistry backs the module selector (MODULE_SPEC.md §4.2). May
	// be nil -- Render simply renders no selector, same "nil means don't
	// show this" pattern the rest of this codebase already uses for
	// optional collaborators (Translator, dashboardEngine.Resolver, etc.).
	// Set via SetModuleRegistry after construction, not a constructor
	// parameter -- NewUIHandler's own parameter list is already long, and
	// dozens of existing test call sites construct a PageHandler/UIHandler
	// without one; adding a required param here would force every one of
	// them to change for a stage (MOD-4) that doesn't yet register any
	// real module (that's MOD-5/6/7's job).
	ModuleRegistry *modules.Registry
}

// NewPageHandler creates a new page handler.
func NewPageHandler(cfg Config) *PageHandler {
	return &PageHandler{Config: cfg}
}

// SetModuleRegistry wires a Registry into the page handler after
// construction. Safe to call with nil (clears it, same as never calling
// this at all).
func (h *PageHandler) SetModuleRegistry(r *modules.Registry) {
	h.ModuleRegistry = r
}

// viewerConfig returns a copy of h.Config with the per-viewer fields
// (UserRole, VisibleModules) populated from r's auth claims and
// ModuleRegistry -- computed fresh per request, never stored back onto
// h.Config itself, since (unlike every other Config field) these vary by
// who's asking, not by tenant. A copy, not a pointer -- Config is already
// passed by value into BaseLayout everywhere else in this file, so this
// doesn't change that contract, it just fills in two more fields before
// the existing pass-by-value happens.
func (h *PageHandler) viewerConfig(r *http.Request) Config {
	cfg := h.Config
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		return cfg
	}
	cfg.UserRole = claims.Role
	if real, ok := auth.RealRoleFromContext(r.Context()); ok && real != claims.Role {
		cfg.RealRole = real
	}
	cfg.ViewerTenantID = claims.TenantID
	if h.Config.flagOn(features.DeveloperChangeNotice) {
		cfg.DevActive = developerActiveFlags(h.Config.DevRuntime, claims.TenantID)
	}
	if h.ModuleRegistry != nil {
		cfg.VisibleModules = h.ModuleRegistry.VisibleFor(claims.TenantID, claims.Role)
		if cfg.RealRole != "" {
			// viewing as a lower role never hides the Developer module: it is the way back
			have := false
			for _, m := range cfg.VisibleModules {
				have = have || m.ID == "developer"
			}
			if !have {
				for _, m := range h.ModuleRegistry.VisibleFor(claims.TenantID, cfg.RealRole) {
					if m.ID == "developer" {
						cfg.VisibleModules = append(cfg.VisibleModules, m)
					}
				}
			}
		}
	}
	return cfg
}

// Render renders a full page or a partial for HTMX requests.
// When HX-Request: true is present, only the content is rendered and
// HX-Title is set so the browser updates the document title.
func (h *PageHandler) Render(w http.ResponseWriter, r *http.Request, title, activePath string, content mi.H) error {
	return h.RenderStatus(w, r, http.StatusOK, title, activePath, content)
}

// RenderStatus is Render with the response status given: for a page that
// answers a request it refused (422 with what was typed and why, say). The
// status is written here, once, after the Content-Type; a handler that calls
// w.WriteHeader itself before Render gets a second WriteHeader logged as
// superfluous and loses the Content-Type (T-117).
func (h *PageHandler) RenderStatus(w http.ResponseWriter, r *http.Request, status int, title, activePath string, content mi.H) error {
	if IsHTMXRequest(r) {
		w.Header().Set("HX-Title", title+" | "+h.Config.AppName)
		if status == http.StatusOK {
			return h.RenderPartial(w, content)
		}
		var part bytes.Buffer
		if err := content(&mi.Builder{}).Render(&part); err != nil {
			return err
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(status)
		_, err := w.Write(part.Bytes())
		return err
	}
	page := BaseLayout(h.viewerConfig(r), title, activePath, content)

	var buf bytes.Buffer
	b := &mi.Builder{}
	buf.WriteString("<!DOCTYPE html>\n")
	if err := page(b).Render(&buf); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, err := w.Write(buf.Bytes())
	return err
}

// RenderWithConfig renders a page using the provided Config override.
// Use this when a page needs non-default layout options (e.g. FullHeight,
// ExtraHead, UseVendoredJS) without modifying the handler's base config.
func (h *PageHandler) RenderWithConfig(w http.ResponseWriter, r *http.Request, cfg Config, title, activePath string, content mi.H) error {
	if IsHTMXRequest(r) {
		w.Header().Set("HX-Title", title+" | "+cfg.AppName)
		return h.RenderPartial(w, content)
	}
	page := BaseLayout(cfg, title, activePath, content)
	var buf bytes.Buffer
	b := &mi.Builder{}
	buf.WriteString("<!DOCTYPE html>\n")
	if err := page(b).Render(&buf); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(buf.Bytes())
	return err
}

// RenderPartial renders a partial template (no layout) for HTMX requests.
func (h *PageHandler) RenderPartial(w http.ResponseWriter, content mi.H) error {
	var buf bytes.Buffer
	b := &mi.Builder{}

	if err := content(b).Render(&buf); err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(buf.Bytes())
	return err
}

// RenderError renders an error page.
func (h *PageHandler) RenderError(w http.ResponseWriter, status int, title, message string) error {
	t := func(key string, args ...any) string { return key }
	if h.Config.Translator != nil {
		t = h.Config.Translator.ForLocale(h.Config.Locale)
	}
	content := func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("flex flex-col items-center justify-center min-h-[60vh]"),
			b.Div(mi.Class("text-6xl font-bold text-gray-300 dark:text-gray-600 mb-4"),
				statusText(status, t),
			),
			b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white mb-2"),
				title,
			),
			b.P(mi.Class("text-gray-500 dark:text-gray-400 mb-6"),
				message,
			),
			Button(t("action.go_home"), "primary", mi.Attr("onclick", "window.location='/'"))(b),
		)
	}

	page := BaseLayout(h.Config, title, "", content)

	var buf bytes.Buffer
	b := &mi.Builder{}
	buf.WriteString("<!DOCTYPE html>\n")

	if err := page(b).Render(&buf); err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, err := w.Write(buf.Bytes())
	return err
}

// RenderPublic renders a public page (no sidebar, minimal layout).
func (h *PageHandler) RenderPublic(w http.ResponseWriter, title string, content mi.H) error {
	page := PublicLayout(h.Config, title, content)

	var buf bytes.Buffer
	b := &mi.Builder{}
	buf.WriteString("<!DOCTYPE html>\n")

	if err := page(b).Render(&buf); err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(buf.Bytes())
	return err
}

// RenderPublicError renders a public error page.
func (h *PageHandler) RenderPublicError(w http.ResponseWriter, status int, title, message string) error {
	content := func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("flex flex-col items-center justify-center min-h-[60vh] text-center px-4"),
			b.Div(mi.Class("text-6xl mb-4"), "⚠️"),
			b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white mb-2"), title),
			b.P(mi.Class("text-gray-500 dark:text-gray-400"), message),
		)
	}

	page := PublicLayout(h.Config, title, content)

	var buf bytes.Buffer
	b := &mi.Builder{}
	buf.WriteString("<!DOCTYPE html>\n")

	if err := page(b).Render(&buf); err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, err := w.Write(buf.Bytes())
	return err
}

func statusText(code int, t func(key string, args ...any) string) string {
	switch code {
	case 400:
		return "400"
	case 401:
		return "401"
	case 403:
		return "403"
	case 404:
		return "404"
	case 500:
		return "500"
	default:
		return t("error.generic")
	}
}

// IsHTMXRequest checks if the request is an HTMX request.
func IsHTMXRequest(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// HTMXRedirect sends an HTMX redirect response.
func HTMXRedirect(w http.ResponseWriter, url string) {
	w.Header().Set("HX-Redirect", url)
	w.WriteHeader(http.StatusOK)
}

// HTMXRefresh sends an HTMX refresh response.
func HTMXRefresh(w http.ResponseWriter) {
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// HTMXTrigger sends an HTMX trigger event.
func HTMXTrigger(w http.ResponseWriter, event string) {
	w.Header().Set("HX-Trigger", event)
}
