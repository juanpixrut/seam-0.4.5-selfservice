package ui

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	mi "github.com/ha1tch/minty"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/devstate"
	"github.com/ha1tch/seam-ui/internal/features"
	"github.com/ha1tch/seam-ui/internal/i18n"
	"github.com/ha1tch/seam-ui/internal/overrides"
	"github.com/ha1tch/seam-ui/internal/schemaoverlay"
)

// The Developer module's pages (campaign DEV-7, DEV-9, DEV-18 to DEV-20). They
// are reached only through Router.MountDeveloperRoutes, which guards every
// route with auth.PermDeveloperAccess and only mounts them when the module is
// switched on (which itself requires authentication).
//
// Design decisions (recorded in docs/DEVELOPER_MODULE_CAMPAIGN.md, DEC-12 to
// DEC-15):
//
//   - One page per section, each with its own left-menu entry, instead of one
//     long page: Overview (the runtime switches), Translations, Form
//     definitions, Startup configuration. The Overview is served at both
//     /developer and /developer/overview, and renders with the latter as its
//     active path, because the sidebar marks an item active by URL prefix and
//     an item at /developer would stay lit on every other page.
//   - Every input form is a form-engine schema (internal/formengine/schemas/
//     developer), rendered by the engine and resolved through resolveSchema,
//     so the tools' own forms are overridable with the tools themselves. What
//     stays built with minty is what is not a form field: the status cards,
//     the segmented Enable | Disable switch, tables and links.
//   - The editors write only through i18n.FileLayer.SetOverride/RemoveOverride
//     and schemaoverlay.FileLayer.Save/Remove (validated, atomic, followed by
//     a reload) and only for texts and forms that exist. A text under a new
//     key and a form under a new name are created on purpose, through their
//     own forms (developer_create.go, DEC-31).

// developerRefLocale is the reference locale of the completeness report and
// of the placeholder check: the locale every other one falls back to.
const developerRefLocale = "en"

// DeveloperPageData holds what the Developer pages share.
type DeveloperPageData struct {
	Settings DeveloperSettings // the settings that are not on/off, as resolved at startup

	Flags   []features.State // every flag as resolved at startup; empty if unavailable
	Profile string           // active profile name, "" for none

	TenantID   int64
	CSRFToken  string
	ViewAs     string                    // the role being viewed as, "" when the developer is themselves (DEV-22)
	ViewAsForm mi.H                      // the Overview's role select
	Runtime    []devstate.FlagView       // the runtime flags for TenantID
	Locales    *i18n.FileStatus          // nil when SEAM_LOCALES_DIR is not set
	Schemas    *schemaoverlay.FileStatus // nil when SEAM_SCHEMAS_DIR is not set
	Message    string                    // already translated; "" for none
	Error      string                    // already translated; "" for none

	// Completeness is how each locale differs from the reference locale, as a
	// tenant sees it; nil when it was not asked for or is unavailable.
	Completeness *i18n.Report

	// FormOverrides is how many forms the tenant sees overridden (a tile of the Overview).
	FormOverrides int
}

// developerMessages maps the short codes carried in ?msg= and ?error= to
// translation keys. Only these codes are ever shown, so nothing from the
// query string is echoed into the page.
var developerMessages = map[string]string{
	"saved":           "developer.msg.saved",
	"reset":           "developer.msg.reset",
	"reloaded":        "developer.msg.reloaded",
	"removed":         "developer.msg.removed",
	"bad_request":     "developer.err.bad_request",
	"pinned":          "developer.err.pinned",
	"inactive":        "developer.err.inactive",
	"not_runtime":     "developer.err.not_runtime",
	"unknown_flag":    "developer.err.unknown_flag",
	"no_tenant":       "developer.err.no_tenant",
	"save_failed":     "developer.err.save_failed",
	"reload_failed":   "developer.err.reload_failed",
	"not_configured":  "developer.err.not_configured",
	"unknown_key":     "developer.err.unknown_key",
	"unknown_locale":  "developer.err.unknown_locale",
	"verbs":           "developer.err.verbs",
	"invalid_text":    "developer.err.invalid_text",
	"unknown_form":    "developer.err.unknown_form",
	"invalid_json":    "developer.err.invalid_json",
	"restored":        "developer.msg.restored",
	"view_as_set":     "developer.msg.view_as_set",
	"view_as_exit":    "developer.msg.view_as_exit",
	"bad_scope":       "developer.err.bad_scope",
	"unknown_version": "developer.err.unknown_version",
	"not_restorable":  "developer.err.not_restorable",
	"created":         "developer.msg.created",
	"invalid_key":     "developer.err.invalid_key",
	"key_exists":      "developer.err.key_exists",
	"invalid_name":    "developer.err.invalid_name",
	"form_exists":     "developer.err.form_exists",
	"export_failed":   "developer.err.export_failed",
}

// developerStateErrorCode maps a Manager error to a ?error= code.
func developerStateErrorCode(err error) string {
	switch {
	case errors.Is(err, devstate.ErrPinned):
		return "pinned"
	case errors.Is(err, devstate.ErrInactive):
		return "inactive"
	case errors.Is(err, devstate.ErrNotRuntime):
		return "not_runtime"
	case errors.Is(err, devstate.ErrUnknownFlag):
		return "unknown_flag"
	case errors.Is(err, devstate.ErrNoTenant):
		return "no_tenant"
	}
	return "save_failed"
}

// developerErrorCode maps any error from the module's operations to a ?error=
// code: the editors' refusals first, then the runtime state's.
func developerErrorCode(err error) string {
	switch {
	case errors.Is(err, i18n.ErrVerbs):
		return "verbs"
	case errors.Is(err, i18n.ErrInvalid):
		return "invalid_text"
	case errors.Is(err, schemaoverlay.ErrInvalidName):
		return "unknown_form"
	case errors.Is(err, schemaoverlay.ErrRejected):
		return "invalid_json"
	case errors.Is(err, overrides.ErrNoTenant):
		return "no_tenant"
	case errors.Is(err, overrides.ErrUnknownVersion):
		return "unknown_version"
	case errors.Is(err, overrides.ErrNotRestorable):
		return "not_restorable"
	}
	return developerStateErrorCode(err)
}

// developerBackURL is where a POST returns to: the section named by its "back"
// field, from a fixed list (anything else is the Overview), so the field can
// never be used to redirect somewhere else.
func developerBackURL(r *http.Request) string {
	switch r.FormValue("back") {
	case "translations":
		return "/developer/translations"
	case "forms":
		return "/developer/forms"
	}
	return "/developer/overview"
}

func developerRedirect(w http.ResponseWriter, r *http.Request, base, key, code string) {
	http.Redirect(w, r, base+"?"+key+"="+url.QueryEscape(code), http.StatusSeeOther)
}

// developerData gathers what every page shows; each page picks what it needs.
func (h *UIHandler) developerData(w http.ResponseWriter, r *http.Request, t func(key string, args ...any) string) DeveloperPageData {
	data := DeveloperPageData{CSRFToken: h.GetCSRFToken(w, r), Settings: h.config.DevSettings}
	if h.config.Flags != nil {
		data.Flags = h.config.Flags.States()
		data.Profile = h.config.Flags.Profile()
	}
	if rt := h.config.DevRuntime; rt != nil {
		if claims, ok := auth.ClaimsFromContext(r.Context()); ok {
			data.TenantID = claims.TenantID
			data.Runtime = developerPanelFlags(h.config, rt.Manager.Panel(claims.TenantID))
			if _, viewing := auth.RealRoleFromContext(r.Context()); viewing {
				data.ViewAs = claims.Role
			}
		}
		if rt.Locales != nil {
			st := rt.Locales.Status()
			data.Locales = &st
		}
		if rt.Schemas != nil {
			st := rt.Schemas.Status()
			data.Schemas = &st
		}
	}
	if key, ok := developerMessages[r.URL.Query().Get("msg")]; ok {
		data.Message = t(key)
	}
	if key, ok := developerMessages[r.URL.Query().Get("error")]; ok {
		data.Error = t(key)
	}
	return data
}

// DeveloperIndex handles GET /developer and GET /developer/overview.
func (h *UIHandler) DeveloperIndex(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	data := h.developerData(w, r, t)
	if h.config.flagOn(features.DeveloperViewAs) {
		data.ViewAsForm = h.viewAsForm(r.Context(), data)
	}
	if rt := h.config.DevRuntime; rt != nil && rt.Resolver != nil {
		data.FormOverrides = len(rt.Resolver.Overridden(data.TenantID))
	}
	if h.translator != nil {
		if rep, err := h.translator.Compare(data.TenantID, developerRefLocale); err == nil {
			data.Completeness = &rep
		}
	}
	h.page.Render(w, r, t("developer.nav.overview"), "/developer/overview", DeveloperOverviewPage(data, t))
}

// DeveloperStartup handles GET /developer/startup.
func (h *UIHandler) DeveloperStartup(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	data := h.developerData(w, r, t)
	h.page.Render(w, r, t("developer.nav.startup"), "/developer/startup", DeveloperStartupPage(data, t))
}

// DeveloperFlagSet handles POST /developer/flags: form fields "key" and
// "action" (on, off or reset). It acts on the caller's own tenant.
func (h *UIHandler) DeveloperFlagSet(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	rt := h.config.DevRuntime
	if !ok || rt == nil {
		h.Error403(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		developerRedirect(w, r, "/developer/overview", "error", "bad_request")
		return
	}
	back := developerBackURL(r)
	key, action := r.FormValue("key"), r.FormValue("action")

	var err error
	msg := "saved"
	switch action {
	case "on":
		err = rt.Manager.Change(r.Context(), claims.TenantID, key, true, claims.UserID)
	case "off":
		err = rt.Manager.Change(r.Context(), claims.TenantID, key, false, claims.UserID)
	case "reset":
		err = rt.Manager.Reset(r.Context(), claims.TenantID, key)
		msg = "reset"
	default:
		developerRedirect(w, r, back, "error", "bad_request")
		return
	}
	if err != nil {
		h.logger.Warn("developer: flag change refused", "flag", key, "action", action, "error", err)
		developerRedirect(w, r, back, "error", developerErrorCode(err))
		return
	}
	// Changing runtime behaviour is always audited.
	h.auditLogger.LogDeveloperChange(r.Context(), r, claims.UserID, claims.TenantID, "feature_flag", key,
		map[string]any{"action": action})
	developerRedirect(w, r, back, "msg", msg)
}

// DeveloperReload handles POST /developer/reload: form field "target"
// (locales or schemas) re-reads that filesystem layer now.
func (h *UIHandler) DeveloperReload(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	rt := h.config.DevRuntime
	if !ok || rt == nil {
		h.Error403(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		developerRedirect(w, r, "/developer/overview", "error", "bad_request")
		return
	}
	back := developerBackURL(r)
	target := r.FormValue("target")
	var err error
	switch {
	case target == "locales" && rt.Locales != nil:
		err = rt.Locales.Reload()
	case target == "schemas" && rt.Schemas != nil:
		err = rt.Schemas.Reload()
	case target == "locales" || target == "schemas":
		developerRedirect(w, r, back, "error", "not_configured")
		return
	default:
		developerRedirect(w, r, back, "error", "bad_request")
		return
	}
	h.auditLogger.LogDeveloperChange(r.Context(), r, claims.UserID, claims.TenantID, "reload", target,
		map[string]any{"ok": err == nil})
	if err != nil {
		// The reason is on the page, in the layer's status.
		developerRedirect(w, r, back, "error", "reload_failed")
		return
	}
	developerRedirect(w, r, back, "msg", "reloaded")
}

// ---------------------------------------------------------------------------
// Shared page pieces
// ---------------------------------------------------------------------------

// developerFlagText is the translated title or description of a runtime
// flag, falling back to the catalogue's English text for a flag without keys.
func developerFlagText(t func(key string, args ...any) string, flagKey, part, fallback string) string {
	k := "developer.flag." + flagKey + "." + part
	if s := t(k); s != k {
		return s
	}
	return fallback
}

// developerFlagIcons gives each runtime switch its icon; a switch without one
// gets the generic toggle.
var developerFlagIcons = map[string]string{
	"hotload_i18n":       "autorenew",
	"hotload_schemas":    "sync",
	"overlay_filesystem": "folder_open",
	"overlay_xolu":       "storage",
	"show_i18n_keys":     "key",
	"module_demo":        "science",
	"module_work_orders": "assignment",
	"module_issues":      "bug_report",
}

// developerPanelFlags drops the switches that have nothing to switch here: a
// module switch while modules cannot be switched at runtime, or for a module
// that is on at startup for everyone.
func developerPanelFlags(cfg Config, views []devstate.FlagView) []devstate.FlagView {
	out := views[:0:0]
	for _, fv := range views {
		switch fv.Flag.Key {
		case features.ModuleDemo:
			if !cfg.moduleSwitches() || cfg.EnableDemo {
				continue
			}
		case features.ModuleWorkOrders:
			if !cfg.moduleSwitches() || cfg.EnableWorkOrders {
				continue
			}
		case features.ModuleIssues:
			if !cfg.moduleSwitches() || cfg.EnableIssues {
				continue
			}
		}
		out = append(out, fv)
	}
	return out
}

// developerRuntimeCard renders one runtime switch as a card: what it does, its
// state and source, and a single Enable | Disable control (or the reason it is
// locked).
func developerRuntimeCard(b *mi.Builder, data DeveloperPageData, fv devstate.FlagView, t func(key string, args ...any) string) mi.Node {
	state, variant := t("developer.flag.off"), ""
	if fv.Value {
		state, variant = t("developer.flag.on"), "success"
	}
	icon := developerFlagIcons[fv.Flag.Key]
	if icon == "" {
		icon = "toggle_on"
	}
	meta := []interface{}{
		mi.Class("flex flex-wrap items-center gap-2"),
		Badge(t("developer.flag.source")+": "+fv.Source.String(), "")(b),
	}
	if !fv.ChangedAt.IsZero() {
		meta = append(meta, b.Span(mi.Class("text-xs text-gray-500 dark:text-gray-400"), fv.ChangedAt.UTC().Format("2006-01-02 15:04")+" UTC"))
	}

	var control mi.Node
	switch {
	case fv.Inactive:
		control = b.P(mi.Class("dev-locked"), b.Span(mi.Class("material-icons text-sm"), "lock"), t("developer.runtime.locked_inactive"))
	case fv.Pinned:
		control = b.P(mi.Class("dev-locked"), b.Span(mi.Class("material-icons text-sm"), "push_pin"), t("developer.runtime.locked_pinned"))
	default:
		onCls, offCls, onPressed, offPressed := "dev-seg-btn", "dev-seg-btn", "false", "false"
		if fv.Value {
			onCls, onPressed = onCls+" is-active-on", "true"
		} else {
			offCls, offPressed = offCls+" is-active-off", "true"
		}
		form := []interface{}{
			mi.Class("flex flex-wrap items-center gap-3"), mi.Method("post"), mi.Action("/developer/flags"),
			CSRFField(data.CSRFToken)(b),
			b.Input(mi.Type("hidden"), mi.Name("key"), mi.Value(fv.Flag.Key)),
			b.Input(mi.Type("hidden"), mi.Name("back"), mi.Value("overview")),
			b.Div(mi.Class("dev-seg"), mi.Attr("role", "group"),
				b.Button(mi.Type("submit"), mi.Name("action"), mi.Value("on"), mi.Class(onCls), mi.Attr("aria-pressed", onPressed), t("developer.action.on")),
				b.Button(mi.Type("submit"), mi.Name("action"), mi.Value("off"), mi.Class(offCls), mi.Attr("aria-pressed", offPressed), t("developer.action.off")),
			),
		}
		if fv.Source == features.SourcePanel {
			form = append(form, b.Button(mi.Type("submit"), mi.Name("action"), mi.Value("reset"), mi.Class("dev-linkbtn"), t("developer.action.reset")))
		}
		control = b.Form(form...)
	}
	return b.Div(mi.Class("dev-card "+devCardClass+" p-4"),
		b.Div(mi.Class("flex items-start justify-between gap-3"),
			b.Div(mi.Class("flex items-start gap-3 min-w-0"),
				b.Span(mi.Class("material-icons text-indigo-600 dark:text-indigo-400"), icon),
				b.Div(mi.Class("min-w-0"),
					b.H3(mi.Class("text-sm font-semibold text-gray-900 dark:text-white"), developerFlagText(t, fv.Flag.Key, "title", fv.Flag.Key)),
					b.Div(mi.Class(devMonoClass), fv.Flag.Key))),
			Badge(state, variant)(b)),
		b.P(mi.Class(devMutedClass+" dev-card-desc"), developerFlagText(t, fv.Flag.Key, "desc", fv.Flag.Description)),
		b.Div(meta...),
		control,
	)
}

// developerLayer is what a filesystem layer's panel shows.
type developerLayer struct {
	Title, Target, EnvVar, Back string
	Configured                  bool
	Dir, LastError              string
	LoadedAt                    time.Time
	Summary, ListLabel          string
	Items                       []string
	// Refused are the files the layer set aside, each with its reason, under
	// RefusedLabel.
	Refused      []string
	RefusedLabel string
}

func developerLocalesLayer(data DeveloperPageData, t func(key string, args ...any) string) developerLayer {
	l := developerLayer{Title: t("developer.layers.locales"), Target: "locales", EnvVar: "SEAM_LOCALES_DIR", Back: "translations"}
	if data.Locales != nil {
		l.Configured, l.Dir, l.LoadedAt, l.LastError = true, data.Locales.Dir, data.Locales.LoadedAt, data.Locales.LastError
		l.Summary = t("developer.layers.keys", data.Locales.Keys)
		l.ListLabel, l.Items = t("developer.layers.new_keys"), data.Locales.NewKeys
		l.RefusedLabel = t("developer.layers.refused_locales")
		for _, f := range data.Locales.Refused {
			l.Refused = append(l.Refused, developerTruncate(f.Reason, 600))
		}
	}
	return l
}

func developerSchemasLayer(data DeveloperPageData, t func(key string, args ...any) string) developerLayer {
	l := developerLayer{Title: t("developer.layers.schemas"), Target: "schemas", EnvVar: "SEAM_SCHEMAS_DIR", Back: "forms"}
	if data.Schemas != nil {
		names := make([]string, 0, len(data.Schemas.Schemas))
		for _, k := range data.Schemas.Schemas {
			names = append(names, k.String())
		}
		l.Configured, l.Dir, l.LoadedAt, l.LastError = true, data.Schemas.Dir, data.Schemas.LoadedAt, data.Schemas.LastError
		l.Summary = t("developer.layers.schema_count", len(names))
		l.ListLabel, l.Items = t("developer.layers.overridden"), names
		l.RefusedLabel = t("developer.layers.refused")
		for _, f := range data.Schemas.Refused {
			// a reason of the loader already starts with the file it is about
			line := developerProblemString(f.Reason)
			if !strings.HasPrefix(line, f.Key.String()) {
				line = f.Key.String() + ": " + line
			}
			l.Refused = append(l.Refused, line)
		}
	}
	return l
}

// developerLayerCard renders a filesystem layer (translations or form
// definitions) as a panel: its directory, load status, last error and a reload
// button, or how to switch it on when no directory is configured.
func developerLayerCard(b *mi.Builder, data DeveloperPageData, l developerLayer, t func(key string, args ...any) string) mi.Node {
	if !l.Configured {
		return developerPanel(b, "folder_open", l.Title, "", []interface{}{Badge(t("developer.overview.off"), "")(b)}, false,
			b.P(mi.Class(devMutedClass), t("developer.layers.hint", l.EnvVar)))
	}
	status := Badge(t("developer.layers.status_ok"), "success")(b)
	if l.LastError != "" || len(l.Refused) > 0 {
		status = Badge(t("developer.layers.status_error"), "danger")(b)
	}
	reload := b.Form(mi.Method("post"), mi.Action("/developer/reload"),
		CSRFField(data.CSRFToken)(b),
		b.Input(mi.Type("hidden"), mi.Name("target"), mi.Value(l.Target)),
		b.Input(mi.Type("hidden"), mi.Name("back"), mi.Value(l.Back)),
		IconButton("refresh", t("developer.layers.reload"), "secondary", mi.Type("submit"))(b))
	var body []interface{}
	body = append(body, b.Div(mi.Class(devMonoClass+" dev-wrap"), l.Dir))
	if l.LoadedAt.IsZero() {
		body = append(body, b.P(mi.Class(devMutedClass), t("developer.layers.not_loaded")))
	} else {
		body = append(body, b.P(mi.Class(devMutedClass),
			t("developer.layers.loaded", l.LoadedAt.UTC().Format("2006-01-02 15:04:05"))+" UTC. "+l.Summary))
	}
	if l.LastError != "" {
		body = append(body, b.Div(mi.Class("dev-problem"),
			b.P(mi.Class("font-semibold"), t("developer.layers.error")),
			b.Div(mi.Class("dev-pre"), l.LastError)))
	}
	if len(l.Refused) > 0 {
		body = append(body, b.Div(mi.Class("dev-problem"), mi.ID("dev-refused-"+l.Target),
			b.P(mi.Class("font-semibold"), l.RefusedLabel),
			b.Div(mi.Class("dev-pre"), strings.Join(l.Refused, "\n"))))
	}
	if len(l.Items) > 0 {
		body = append(body, b.Div(
			b.P(mi.Class(devMutedClass), l.ListLabel),
			b.Div(mi.Class(devMonoClass+" dev-wrap"), strings.Join(l.Items, ", "))))
	}
	return developerPanel(b, "folder_open", l.Title, "", []interface{}{status, reload}, false, body...)
}

// developerCompletenessTable shows, per locale, how it differs from the
// reference locale (internal/i18n.Compare). A count above zero is a link to the
// texts it counts.
func developerCompletenessTable(b *mi.Builder, rep *i18n.Report, t func(key string, args ...any) string) mi.Node {
	count := func(locale, show string, n int) mi.H {
		return func(b *mi.Builder) mi.Node {
			if n == 0 {
				return b.Td(mi.Class("px-4 py-3 text-sm text-right text-gray-500 dark:text-gray-400 dev-num"), "0")
			}
			return b.Td(mi.Class("px-4 py-3 text-sm text-right dev-num"),
				b.A(mi.Href(translationsURL(locale, "", show, 1, "")+"#texts"), mi.Class("dev-num-warn"), mi.Attr("title", t("developer.completeness.open")), strconv.Itoa(n)))
		}
	}
	var rows []mi.H
	for i, d := range rep.Diffs {
		d := d
		rows = append(rows, ListRow(i,
			ListCellRich(func(b *mi.Builder) mi.Node {
				return b.Span(mi.Class("font-mono text-sm font-medium text-gray-900 dark:text-white"), d.Locale)
			}),
			count(d.Locale, showMissing, len(d.Missing)), count(d.Locale, showExtra, len(d.Extra)),
			count(d.Locale, showUntranslated, len(d.Untranslated)), count(d.Locale, showFormat, len(d.FormatMismatch)),
		))
	}
	return ListTable(ListTableConfig{
		Columns: []ListColumn{
			{Label: t("developer.completeness.locale")}, {Label: t("developer.completeness.missing"), Right: true},
			{Label: t("developer.completeness.extra"), Right: true}, {Label: t("developer.completeness.untranslated"), Right: true},
			{Label: t("developer.completeness.format"), Right: true},
		},
		EmptyIcon: "translate", EmptyTitle: t("developer.translations.empty"), SkipCard: true,
	}, rows)(b)
}

// developerStartupTable lists the flags fixed when the server started: value,
// source and the setting that controls each. The runtime switches are on the
// Overview and are not listed again.
func developerStartupTable(b *mi.Builder, flags []features.State, t func(key string, args ...any) string) mi.Node {
	var rows []mi.H
	for _, st := range flags {
		if st.Flag.Mode == features.ModeRuntime {
			continue
		}
		st := st
		value, variant := t("developer.flag.off"), ""
		if st.Value {
			value, variant = t("developer.flag.on"), "success"
		}
		rows = append(rows, ListRow(len(rows),
			ListCellRich(func(b *mi.Builder) mi.Node {
				return b.Div(
					b.Div(mi.Class("font-mono text-sm font-medium text-gray-900 dark:text-white"), st.Flag.Key),
					b.Div(mi.Class("text-xs text-gray-500 dark:text-gray-400"), st.Flag.Description))
			}),
			ListCellRich(Badge(value, variant)),
			ListCellRich(func(b *mi.Builder) mi.Node {
				src := []interface{}{mi.Class("flex flex-wrap items-center gap-2"), Badge(st.Source.String(), "")(b)}
				if st.Pinned {
					src = append(src, IconBadge("push_pin", t("developer.flag.pinned"), "warning")(b))
				}
				return b.Div(src...)
			}),
			ListCellRich(func(b *mi.Builder) mi.Node { return b.Span(mi.Class(devMonoClass), st.Flag.EnvVar) }),
		))
	}
	return ListTable(ListTableConfig{
		Columns: []ListColumn{{Label: t("developer.table.flag")}, {Label: t("developer.table.value")},
			{Label: t("developer.table.source")}, {Label: t("developer.table.setting")}},
		EmptyIcon: "tune", EmptyTitle: t("developer.section.startup"),
	}, rows)(b)
}

// ---------------------------------------------------------------------------
// The Overview and the Startup configuration pages
// ---------------------------------------------------------------------------

// DeveloperOverviewPage renders the summary tiles (each the way into a
// section), the runtime switches, the role to view the application as and the
// two override directories.
func DeveloperOverviewPage(data DeveloperPageData, t func(key string, args ...any) string) mi.H {
	t = developerTrans(t)
	return func(b *mi.Builder) mi.Node {
		var parts []interface{}

		// the tiles
		on := 0
		for _, fv := range data.Runtime {
			if fv.Value {
				on++
			}
		}
		off := t("developer.overview.off")
		trValue, trSub := off, ""
		if data.Locales != nil {
			trValue = strconv.Itoa(data.Locales.Keys)
		}
		if data.Completeness != nil {
			n := 0
			for _, d := range data.Completeness.Diffs {
				n += len(d.Untranslated) + len(d.Missing)
			}
			trSub = t("developer.overview.untranslated", n)
		}
		startOn, startAll := 0, 0
		for _, st := range data.Flags {
			if st.Flag.Mode == features.ModeRuntime {
				continue
			}
			startAll++
			if st.Value {
				startOn++
			}
		}
		profile := data.Profile
		if profile == "" {
			profile = t("developer.flags.profile_none")
		}
		parts = append(parts, b.Div(mi.Class("dev-tiles"),
			developerStat(b, "toggle_on", t("developer.overview.switches_on"), strconv.Itoa(on)+" / "+strconv.Itoa(len(data.Runtime)), t("developer.overview.switches_sub"), "/developer/overview#switches"),
			developerStat(b, "translate", t("developer.overview.translation_overrides"), trValue, trSub, "/developer/translations"),
			developerStat(b, "dynamic_form", t("developer.overview.form_overrides"), strconv.Itoa(data.FormOverrides), t("developer.overview.forms_total", len(developerEmbeddedSchemas())), "/developer/forms"),
			developerStat(b, "tune", t("developer.overview.startup_flags"), strconv.Itoa(startOn)+" / "+strconv.Itoa(startAll), t("developer.flags.profile", profile), "/developer/startup"),
		))

		if len(data.Runtime) > 0 {
			cards := []interface{}{mi.Class("dev-grid")}
			for _, fv := range data.Runtime {
				cards = append(cards, developerRuntimeCard(b, data, fv, t))
			}
			parts = append(parts, b.Div(mi.ID("switches"),
				developerSection(b, t("developer.section.runtime"), t("developer.runtime.note"), b.Div(cards...))))
		}

		// the override directories, reloadable from here, and the role to view as
		locales, schemas := developerLocalesLayer(data, t), developerSchemasLayer(data, t)
		locales.Back, schemas.Back = "overview", "overview"
		cols := []interface{}{mi.Class("dev-cols"),
			b.Div(mi.Class("space-y-6"), developerLayerCard(b, data, locales, t), developerLayerCard(b, data, schemas, t))}
		if data.ViewAsForm != nil {
			cols = append(cols, developerViewAsSection(b, data, t))
		}
		parts = append(parts, b.Div(cols...))
		return developerFrame(b, developerHeader{Title: t("developer.nav.overview"), Intro: t("developer.intro"),
			Actions: []interface{}{developerExportLink(b, t)}}, data, parts...)
	}
}

// DeveloperStartupPage renders the flags fixed when the server started.
func DeveloperStartupPage(data DeveloperPageData, t func(key string, args ...any) string) mi.H {
	t = developerTrans(t)
	return func(b *mi.Builder) mi.Node {
		profile := data.Profile
		if profile == "" {
			profile = t("developer.flags.profile_none")
		}
		return developerFrame(b, developerHeader{Title: t("developer.nav.startup"), Intro: t("developer.startup.intro"),
			Actions: []interface{}{IconBadge("tune", t("developer.flags.profile", profile), "info")(b)}}, data,
			developerStartupTable(b, data.Flags, t), developerSettingsPanel(b, data.Settings, t))
	}
}
