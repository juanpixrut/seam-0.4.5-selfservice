package ui

import (
	"context"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	mi "github.com/ha1tch/minty"

	"github.com/ha1tch/seam-ui/internal/auth"
	fe "github.com/ha1tch/seam-ui/internal/formengine"
	developerschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/developer"
	"github.com/ha1tch/seam-ui/internal/i18n"
	"github.com/ha1tch/seam-ui/internal/overrides"
)

// The Translations section (campaign DEV-18, DEV-19): the filesystem layer's
// status and reload, the completeness report, a filterable list of every text
// with the layer that supplies it, and an editor for one text at a time. The
// list and its filters are the listing primitives every list page uses
// (ListFilterBar, ListTable); the editor's form is a form-engine schema
// (developer/translation_edit); saving goes through i18n.FileLayer.SetOverride,
// which validates, writes atomically and reloads.
//
// An override is global: it replaces the text for every tenant, because the
// filesystem layer is global (campaign DEC-9). The editor therefore only
// exists when SEAM_LOCALES_DIR is set, only edits texts that exist in the
// reference locale or in the locale itself (a typo cannot invent a key; a new
// key is created with its own form, developer_create.go), and keeps the
// original's printf placeholders. A tenant's override keeps its versions.

const (
	developerPageSize = 50
	developerMaxQuery = 100
	developerMaxBody  = 64 << 10
)

type translationRow struct{ Key, Text, Layer string }

type translationEditor struct {
	Key, Reference, Embedded, Override string
	HasOverride                        bool // an override exists in the selected scope
	Form                               mi.H

	// Scope is the selected scope (scopeTenant or scopeFile) and Scopes the ones
	// on offer; Override and HasOverride are that scope's. Shadow is the
	// translation key of a note when the other scope's override hides or is
	// hidden by this one (a file override outranks a tenant's, DEC-4).
	Scope  string
	Scopes []string
	Shadow string

	// History is the scope's versions of this text, newest first: the tenant's,
	// or the ones the tools recorded for the file.
	History []overrides.TranslationVersion

	// Locales is how every language stands for this key: its own text, only the
	// reference text repeated, or none.
	Locales []translationLocaleState
	// Source is how the source tree refers to the text (nil without a
	// checkout): what makes "not referenced" checkable.
	Source *translationSource
}

// translationNew is what the "new text" panel shows: the form (key and text)
// and the scopes the text can be created in.
type translationNew struct {
	Form   mi.H
	Scopes []string
	Scope  string
}

// TranslationsPageData holds what the Translations page renders.
type TranslationsPageData struct {
	DeveloperPageData
	Locale, Query      string
	Show               string // the list's "show" filter (showAll, showOverridden, ...)
	SourceScan         bool   // a source checkout is configured, so "not referenced" can be offered
	LocaleList         []string
	Page, Pages, Total int
	Rows               []translationRow
	Editor             *translationEditor
	New                *translationNew
	Writable           bool
}

// developerTruncate shortens s to at most n characters, on a character boundary.
func developerTruncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "…"
}

// developerAppend adds one query parameter to a URL that may already have some.
func developerAppend(base, key, value string) string {
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return base + sep + key + "=" + url.QueryEscape(value)
}

func translationsURL(locale, q, show string, page int, key string) string {
	v := url.Values{}
	if locale != "" {
		v.Set("locale", locale)
	}
	if q != "" {
		v.Set("q", q)
	}
	if show != showAll {
		v.Set("show", show)
	}
	if page > 1 {
		v.Set("page", strconv.Itoa(page))
	}
	if key != "" {
		v.Set("key", key)
	}
	s := "/developer/translations"
	if e := v.Encode(); e != "" {
		s += "?" + e
	}
	return s
}

// The list's "show" filter: every text, or only the ones a count of the
// completeness report (or an override) refers to.
const (
	showAll          = ""
	showOverridden   = "overridden"
	showUntranslated = "untranslated"
	showMissing      = "missing"
	showExtra        = "extra"
	showFormat       = "format"
	// showUnused lists the texts nothing in the source refers to. It needs a
	// source checkout (the setting developer_source_dir); without one it is
	// not offered and means "all".
	showUnused = "unused"
)

// translationShow accepts only the filter's own values; anything else is "all".
func translationShow(s string) string {
	switch s {
	case showOverridden, showUntranslated, showMissing, showExtra, showFormat, showUnused:
		return s
	}
	return showAll
}

// translationShowName is the filter value as the last part of its translation key.
func translationShowName(s string) string {
	if s == showAll {
		return "all"
	}
	return s
}

// translationShown is the set of keys the "show" filter keeps for a locale,
// or nil when it keeps them all.
func (h *UIHandler) translationShown(ctx context.Context, tenant int64, locale, show string, rep *i18n.Report) map[string]struct{} {
	if show == showAll {
		return nil
	}
	only := map[string]struct{}{}
	if show == showUnused {
		for _, k := range h.unreferencedKeys(tenant) {
			only[k] = struct{}{}
		}
		return only
	}
	if show == showOverridden {
		rt := h.config.DevRuntime
		if rt != nil && rt.Locales != nil {
			if m, err := rt.Locales.Overrides(locale); err == nil {
				for k := range m {
					only[k] = struct{}{}
				}
			}
		}
		if rt != nil && rt.Overrides != nil && tenant > 0 {
			if rows, err := rt.Overrides.Translations(ctx, tenant); err == nil {
				for _, row := range rows {
					if row.Locale == locale {
						only[row.Key] = struct{}{}
					}
				}
			}
		}
		return only
	}
	if rep == nil {
		return only
	}
	for _, d := range rep.Diffs {
		if d.Locale != locale {
			continue
		}
		var keys []string
		switch show {
		case showUntranslated:
			keys = d.Untranslated
		case showMissing:
			keys = d.Missing
		case showExtra:
			keys = d.Extra
		case showFormat:
			keys = d.FormatMismatch
		}
		for _, k := range keys {
			only[k] = struct{}{}
		}
	}
	return only
}

// translationLayer is the name of the layer that supplies a key's text.
func (h *UIHandler) translationLayer(tenant int64, locale, key string) string {
	for _, c := range h.translator.Explain(tenant, locale, key) {
		if c.Effective {
			return string(c.Layer)
		}
	}
	return string(i18n.LayerEmbedded)
}

// translationReference is the embedded text an override replaces: the one its
// printf placeholders must match. Empty when there is none.
func (h *UIHandler) translationReference(tenant int64, locale, key string) string {
	for _, c := range h.translator.Explain(tenant, locale, key) {
		if c.Layer == i18n.LayerEmbedded {
			return c.Message
		}
	}
	// A text created with the tools has nothing built in. A translation of it
	// must still keep the placeholders of the reference locale's text.
	if locale != developerRefLocale {
		return h.translator.Effective(tenant, developerRefLocale)[key]
	}
	return ""
}

// DeveloperTranslations handles GET /developer/translations.
func (h *UIHandler) DeveloperTranslations(w http.ResponseWriter, r *http.Request) {
	h.applyDeveloperPrefs(w, r, true)
	t := h.translateFunc(r)
	ctx := r.Context()
	data := TranslationsPageData{DeveloperPageData: h.developerData(w, r, t)}
	data.Writable = len(developerScopes(h.config.DevRuntime, h.config.DevRuntime != nil && h.config.DevRuntime.Locales != nil)) > 0
	tr := h.translator
	if tr == nil {
		h.page.Render(w, r, t("developer.nav.translations"), "/developer/translations", DeveloperTranslationsPage(data, t))
		return
	}
	tenant := data.TenantID
	data.LocaleList = tr.Locales()
	sort.Strings(data.LocaleList)

	q := r.URL.Query()
	locale := q.Get("locale")
	if !tr.HasLocale(locale) {
		locale = h.currentLocale()
		if !tr.HasLocale(locale) {
			locale = developerRefLocale
		}
	}
	data.Locale = locale
	data.Query = developerTruncate(strings.TrimSpace(q.Get("q")), developerMaxQuery)
	data.Show = translationShow(q.Get("show"))
	data.SourceScan = h.config.DevSettings.SourceDir != ""
	if data.Show == showUnused && !data.SourceScan {
		data.Show = showAll
	}
	if rep, err := tr.Compare(tenant, developerRefLocale); err == nil {
		data.Completeness = &rep
	}

	ref := tr.Effective(tenant, developerRefLocale)
	cur := tr.Effective(tenant, locale)
	known := make(map[string]struct{}, len(ref)+len(cur))
	for k := range ref {
		known[k] = struct{}{}
	}
	for k := range cur {
		known[k] = struct{}{}
	}
	needle := strings.ToLower(data.Query)
	only := h.translationShown(ctx, tenant, locale, data.Show, data.Completeness)
	var keys []string
	for k := range known {
		if only != nil {
			if _, ok := only[k]; !ok {
				continue
			}
		}
		if needle != "" {
			text := cur[k]
			if text == "" {
				text = ref[k]
			}
			if !strings.Contains(strings.ToLower(k), needle) && !strings.Contains(strings.ToLower(text), needle) {
				continue
			}
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	data.Total = len(keys)
	data.Pages = (len(keys) + developerPageSize - 1) / developerPageSize
	if data.Pages == 0 {
		data.Pages = 1
	}
	data.Page, _ = strconv.Atoi(q.Get("page"))
	if data.Page < 1 {
		data.Page = 1
	}
	if data.Page > data.Pages {
		data.Page = data.Pages
	}
	lo := (data.Page - 1) * developerPageSize
	hi := lo + developerPageSize
	if hi > len(keys) {
		hi = len(keys)
	}
	for _, k := range keys[lo:hi] {
		text := cur[k]
		if text == "" {
			text = ref[k]
		}
		data.Rows = append(data.Rows, translationRow{Key: k, Text: developerTruncate(text, 120), Layer: h.translationLayer(tenant, locale, k)})
	}

	engine := &fe.FormEngine{Translator: h.formEngineTranslator()}
	if key := q.Get("key"); key != "" {
		if _, ok := known[key]; !ok {
			data.Error = t("developer.err.unknown_key")
		} else {
			data.Editor = h.translationEditor(r, engine, data, key)
		}
	} else if data.Writable && q.Get("new") != "" {
		rt := h.config.DevRuntime
		scopes := h.scopesWithGlobal(ctx, developerScopes(rt, rt != nil && rt.Locales != nil))
		data.New = &translationNew{Scopes: scopes, Scope: developerScope(q.Get("scope"), scopes),
			Form: engine.Render(ctx, h.translationNewSchema(ctx, t),
				nil, nil, fe.RenderContext{Locale: h.currentLocale(), CSRFToken: data.CSRFToken, Module: "developer", Form: "translation_new"})}
	}
	h.page.Render(w, r, t("developer.nav.translations"), "/developer/translations", DeveloperTranslationsPage(data, t))
}

func (h *UIHandler) translationEditor(r *http.Request, engine *fe.FormEngine, data TranslationsPageData, key string) *translationEditor {
	ctx := r.Context()
	e := &translationEditor{Key: key}
	effective := ""
	for _, c := range h.translator.Explain(data.TenantID, data.Locale, key) {
		if c.Layer == i18n.LayerEmbedded {
			e.Embedded = c.Message
		}
		if c.Effective {
			effective = c.Message
		}
	}
	// the overrides of each scope are read from their own source, not inferred
	rt := h.config.DevRuntime
	fileText, hasFile := "", false
	if rt != nil && rt.Locales != nil {
		if m, err := rt.Locales.Overrides(data.Locale); err == nil {
			fileText, hasFile = m[key]
		}
	}
	tenantText, hasTenant := "", false
	if rt != nil && rt.Overrides != nil && data.TenantID > 0 {
		if rows, err := rt.Overrides.Translations(ctx, data.TenantID); err == nil {
			for _, row := range rows {
				if row.Locale == data.Locale && row.Key == key {
					tenantText, hasTenant = row.Value, true
				}
			}
		}
	}
	e.Scopes = h.scopesWithGlobal(ctx, developerScopes(rt, rt != nil && rt.Locales != nil))
	e.Scope = developerScope(r.URL.Query().Get("scope"), e.Scopes)
	switch e.Scope {
	case scopeFile:
		e.Override, e.HasOverride = fileText, hasFile
		if hasTenant {
			e.Shadow = "developer.translations.shadows_tenant"
		}
		if rt != nil {
			e.History = fileTextHistory(rt.Locales, data.Locale, key)
		}
	case scopeGlobal:
		if rt != nil && rt.Overrides != nil {
			e.History, _ = rt.Overrides.TranslationHistory(ctx, overrides.GlobalTenant, data.Locale, key)
			if len(e.History) > 0 && !e.History[0].Removed {
				e.Override, e.HasOverride = e.History[0].Value, true
			}
		}
		if hasTenant || hasFile {
			e.Shadow = "developer.scope.global.shadowed"
		}
	case scopeTenant:
		e.Override, e.HasOverride = tenantText, hasTenant
		if hasFile {
			e.Shadow = "developer.translations.shadowed_by_file"
		}
		if rt != nil && rt.Overrides != nil && data.TenantID > 0 {
			e.History, _ = rt.Overrides.TranslationHistory(ctx, data.TenantID, data.Locale, key)
		}
	}
	e.Reference = h.translator.Effective(data.TenantID, developerRefLocale)[key]
	e.Locales = h.translationLocaleStates(data.TenantID, key)
	e.Source = h.translationSourceOf(key)
	value := effective
	if e.HasOverride {
		value = e.Override
	}
	e.Form = engine.Render(ctx, resolveSchema(ctx, "developer", "translation_edit", developerschemas.Schema("translation_edit")),
		map[string]interface{}{"locale": data.Locale, "key": key, "value": value}, nil,
		fe.RenderContext{Locale: h.currentLocale(), CSRFToken: data.CSRFToken, Module: "developer", Form: "translation_edit",
			HiddenFields: []string{"locale", "key"}})
	return e
}

// translationShows are the choices of the list's "show" filter, in the order
// they are offered.
var translationShows = []string{showAll, showOverridden, showUntranslated, showMissing, showExtra, showFormat}

// DeveloperTranslationsPage renders the Translations section: the editor of
// the selected text first (so it is in view when a row is opened), then the
// override directory and the completeness report side by side, then the list
// of texts with its filters.
func DeveloperTranslationsPage(data TranslationsPageData, t func(key string, args ...any) string) mi.H {
	t = developerTrans(t)
	return func(b *mi.Builder) mi.Node {
		var parts []interface{}
		if data.Editor != nil {
			parts = append(parts, developerTranslationEditorCard(b, data, t))
		}
		if data.New != nil {
			parts = append(parts, b.Div(mi.ID("new"), developerPanel(b, "add", t("developer.translations.new"), t("developer.translations.new_note", developerRefLocale),
				[]interface{}{b.A(mi.Href(translationsURL(data.Locale, data.Query, data.Show, data.Page, "")), mi.Class("inline-flex "+devIconBtn),
					mi.Attr("title", t("action.close")), mi.Attr("aria-label", t("action.close")), b.Span(mi.Class("material-icons text-lg"), "close"))}, false,
				b.Form(mi.Class("space-y-4"), mi.Attr("style", "max-width:40rem"), mi.Method("post"), mi.Action("/developer/translations/create"),
					b.Input(mi.Type("hidden"), mi.Name("locale"), mi.Value(data.Locale)),
					b.Input(mi.Type("hidden"), mi.Name("q"), mi.Value(data.Query)),
					b.Input(mi.Type("hidden"), mi.Name("show"), mi.Value(data.Show)),
					data.New.Form(b),
					developerScopeSelect(b, t, data.New.Scopes, data.New.Scope),
					b.Div(IconButton("add", t("developer.translations.create"), "primary", mi.Type("submit"))(b))))))
		}

		top := []interface{}{mi.Class("dev-cols"),
			developerLayerCard(b, data.DeveloperPageData, developerLocalesLayer(data.DeveloperPageData, t), t)}
		if data.Completeness != nil {
			top = append(top, developerPanel(b, "fact_check", t("developer.section.completeness"),
				t("developer.completeness.note", data.Completeness.Ref), nil, true, developerCompletenessTable(b, data.Completeness, t)))
		}
		parts = append(parts, b.Div(top...))

		// the list of texts
		list := []interface{}{mi.ID("texts"), mi.Class("space-y-4"),
			b.H2(mi.Class("text-lg font-semibold text-gray-900 dark:text-white"), t("developer.translations.texts"))}
		if len(data.LocaleList) > 0 {
			locales := make([]SelectOption, 0, len(data.LocaleList))
			for _, l := range data.LocaleList {
				locales = append(locales, SelectOption{Value: l, Label: l, Selected: l == data.Locale})
			}
			shows := make([]SelectOption, 0, len(translationShows))
			for _, s := range translationShows {
				shows = append(shows, SelectOption{Value: s, Label: t("developer.translations.show." + translationShowName(s)), Selected: s == data.Show})
			}
			if data.SourceScan {
				shows = append(shows, SelectOption{Value: showUnused, Label: t("developer.translations.show.unused"), Selected: data.Show == showUnused})
			}
			list = append(list, ListFilterBar(ListFilterConfig{
				Action: "/developer/translations", SearchName: "q", SearchHint: t("developer.translations.search"), SearchValue: data.Query,
				Filters: []ListFilterField{{Name: "locale", Options: locales}, {Name: "show", Options: shows}},
			})(b))
		}
		var rows []mi.H
		for i, row := range data.Rows {
			row := row
			href := translationsURL(data.Locale, data.Query, data.Show, data.Page, row.Key) + "#edit"
			var attrs []interface{}
			if data.Editor != nil && data.Editor.Key == row.Key {
				attrs = []interface{}{mi.Attr("data-dev-selected", "1")}
			}
			rows = append(rows, ListRowWithAttrs(i, attrs,
				ListCellRich(func(b *mi.Builder) mi.Node {
					return b.A(mi.Href(href), mi.Attr("data-row-link", "1"), mi.Class("font-mono text-xs font-medium text-indigo-600 dark:text-indigo-400 hover:underline"), row.Key)
				}),
				ListCellRich(func(b *mi.Builder) mi.Node {
					return b.Span(mi.Class("text-gray-900 dark:text-white dev-wrap"), row.Text)
				}),
				ListCellRich(func(b *mi.Builder) mi.Node { return developerLayerBadge(b, t, row.Layer) }),
				ListCellRight(func(b *mi.Builder) mi.Node {
					return b.A(mi.Href(href), mi.Class("inline-flex "+devIconBtn), mi.Attr("title", t("developer.translations.edit")), mi.Attr("aria-label", t("developer.translations.edit")+" "+row.Key),
						b.Span(mi.Class("material-icons text-lg"), "edit"))
				}),
			))
		}
		list = append(list, ListTable(ListTableConfig{
			Columns: []ListColumn{{Label: t("developer.translations.col.key")}, {Label: t("developer.translations.col.text")},
				{Label: t("developer.translations.col.layer")}, {Label: t("developer.translations.edit"), Right: true}},
			EmptyIcon: "translate", EmptyTitle: t("developer.translations.empty"), EmptyMessage: t("developer.translations.empty_hint"),
		}, rows)(b))
		if len(data.Rows) > 0 {
			list = append(list, developerPager(b, t, data.Page, data.Pages,
				t("developer.translations.page", data.Page, data.Pages)+" · "+t("developer.translations.count", data.Total),
				func(page int) string { return translationsURL(data.Locale, data.Query, data.Show, page, "") + "#texts" }))
		}
		parts = append(parts, b.Div(list...))
		head := developerHeader{Title: t("developer.nav.translations"), Intro: t("developer.translations.intro"),
			Actions: []interface{}{developerExportLink(b, t)}}
		if data.Writable {
			head.Actions = append(head.Actions, developerNewLink(b, developerAppend(translationsURL(data.Locale, data.Query, data.Show, data.Page, ""), "new", "1")+"#new", t("developer.translations.new")))
		}
		return developerFrame(b, head, data.DeveloperPageData, parts...)
	}
}

// developerTranslationEditorCard is the editor of one text: what it replaces
// on the left, the scope, the override and its buttons on the right.
func developerTranslationEditorCard(b *mi.Builder, data TranslationsPageData, t func(key string, args ...any) string) mi.Node {
	e := data.Editor
	pairs := [][2]string{{t("developer.translations.reference", developerRefLocale), e.Reference}}
	if e.Embedded != "" && e.Embedded != e.Reference {
		pairs = append(pairs, [2]string{t("developer.translations.embedded"), e.Embedded})
	}
	actions := []interface{}{Badge(data.Locale, "info")(b),
		b.A(mi.Href(translationsURL(data.Locale, data.Query, data.Show, data.Page, "")+"#texts"), mi.Class("inline-flex "+devIconBtn),
			mi.Attr("title", t("action.close")), mi.Attr("aria-label", t("action.close")), b.Span(mi.Class("material-icons text-lg"), "close"))}
	leftParts := []interface{}{mi.Class("space-y-4"), developerDL(b, pairs...)}
	if len(e.Locales) > 1 {
		// every language at a glance, each a way into its own editor
		langs := []interface{}{mi.Class("flex flex-wrap items-center gap-2")}
		for _, ls := range e.Locales {
			variant, icon := "success", "check_circle"
			switch ls.State {
			case textUntranslated:
				variant, icon = "warning", "warning"
			case textMissing:
				variant, icon = "danger", "error"
			}
			href := translationsURL(ls.Locale, data.Query, data.Show, data.Page, e.Key)
			if e.Scope != "" {
				href = developerAppend(href, "scope", e.Scope)
			}
			langs = append(langs, b.A(mi.Href(href+"#edit"), mi.Class("dev-lang"), mi.Attr("title", t("developer.translations.state."+ls.State)),
				IconBadge(icon, ls.Locale, variant)(b)))
		}
		leftParts = append(leftParts, b.Div(mi.Class("space-y-1"),
			b.Div(mi.Class("text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider"), t("developer.translations.languages")),
			b.Div(langs...)))
	}
	if e.Source != nil {
		name := e.Source.Rule
		if name == "" {
			name = "none"
		}
		src := []interface{}{mi.Class("space-y-1"), mi.ID("dev-text-source"),
			b.Div(mi.Class("text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider"), t("developer.translations.source")),
			b.P(mi.Class(devMutedClass), t("developer.translations.source."+name, e.Source.Detail))}
		if len(e.Source.Where) > 0 {
			src = append(src, b.Div(mi.Class(devMonoClass+" dev-wrap"), strings.Join(e.Source.Where, ", ")))
		}
		leftParts = append(leftParts, b.Div(src...))
	}
	if len(e.History) > 0 {
		leftParts = append(leftParts, developerTranslationVersions(b, data, t))
	}
	left := b.Div(leftParts...)
	if !data.Writable {
		return b.Div(mi.ID("edit"), developerPanel(b, "edit", e.Key, t("developer.translations.editor"), actions, false,
			left, Alert(t("developer.translations.not_writable"), "warning", false)(b)))
	}
	right := []interface{}{mi.Class("space-y-4")}
	right = append(right, developerScopeTabs(b, t, e.Scopes, e.Scope, func(scope string) string {
		return developerAppend(translationsURL(data.Locale, data.Query, data.Show, data.Page, e.Key), "scope", scope) + "#edit"
	})...)
	if e.Shadow != "" {
		right = append(right, Alert(t(e.Shadow), "warning", false)(b))
	}
	hidden := func(extra ...interface{}) []interface{} {
		return append([]interface{}{
			b.Input(mi.Type("hidden"), mi.Name("scope"), mi.Value(e.Scope)),
			b.Input(mi.Type("hidden"), mi.Name("locale"), mi.Value(data.Locale)),
			b.Input(mi.Type("hidden"), mi.Name("key"), mi.Value(e.Key)),
			b.Input(mi.Type("hidden"), mi.Name("q"), mi.Value(data.Query)),
			b.Input(mi.Type("hidden"), mi.Name("show"), mi.Value(data.Show)),
			b.Input(mi.Type("hidden"), mi.Name("page"), mi.Value(strconv.Itoa(data.Page))),
		}, extra...)
	}
	buttons := []interface{}{mi.Class("flex flex-wrap items-center gap-3"),
		IconButton("save", t("developer.translations.save"), "primary", mi.Type("submit"))(b)}
	if e.HasOverride {
		buttons = append(buttons, IconButton("delete", t("developer.translations.remove"), "secondary", mi.Type("submit"), mi.Attr("form", "dev-tr-remove"))(b))
	}
	right = append(right, b.Form(append([]interface{}{mi.Class("space-y-4"), mi.Method("post"), mi.Action("/developer/translations/save")},
		hidden(e.Form(b), b.Div(buttons...))...)...))
	if e.HasOverride {
		right = append(right, b.Form(append([]interface{}{mi.ID("dev-tr-remove"), mi.Method("post"), mi.Action("/developer/translations/remove"),
			CSRFField(data.CSRFToken)(b)}, hidden()...)...))
	}
	return b.Div(mi.ID("edit"), developerPanel(b, "edit", e.Key, t("developer.translations.editor"), actions, false,
		b.Div(mi.Class("dev-cols"), left, b.Div(right...))))
}

// developerTranslationVersions lists the scope's versions of the text, newest
// first, with a Restore button on every version that has a text and is not the
// one that is live.
func developerTranslationVersions(b *mi.Builder, data TranslationsPageData, t func(key string, args ...any) string) mi.Node {
	e := data.Editor
	var rows []mi.H
	for i, v := range e.History {
		v, newest := v, i == 0
		rows = append(rows, ListRow(i,
			ListCellMuted("v"+strconv.Itoa(v.Version)),
			ListCellRich(func(b *mi.Builder) mi.Node {
				return b.Div(
					b.Div(mi.Class("text-xs text-gray-500 dark:text-gray-400"), developerWhen(v.ChangedAt)+" · "+developerWho(t, v.ChangedBy)),
					b.Div(mi.Class("text-gray-900 dark:text-white dev-wrap"), v.Value))
			}),
			ListCellRight(func(b *mi.Builder) mi.Node {
				switch {
				case v.Removed:
					return Badge(t("developer.forms.removed_version"), "")(b)
				case newest && e.HasOverride && v.Value == e.Override:
					return Badge(t("developer.forms.current"), "success")(b)
				}
				return b.Form(mi.Class("inline"), mi.Method("post"), mi.Action("/developer/translations/restore"), CSRFField(data.CSRFToken)(b),
					b.Input(mi.Type("hidden"), mi.Name("scope"), mi.Value(e.Scope)),
					b.Input(mi.Type("hidden"), mi.Name("locale"), mi.Value(data.Locale)),
					b.Input(mi.Type("hidden"), mi.Name("key"), mi.Value(e.Key)),
					b.Input(mi.Type("hidden"), mi.Name("q"), mi.Value(data.Query)),
					b.Input(mi.Type("hidden"), mi.Name("show"), mi.Value(data.Show)),
					b.Input(mi.Type("hidden"), mi.Name("page"), mi.Value(strconv.Itoa(data.Page))),
					b.Input(mi.Type("hidden"), mi.Name("version"), mi.Value(strconv.Itoa(v.Version))),
					IconButton("restore", t("developer.forms.restore"), "secondary", mi.Type("submit"))(b))
			})))
	}
	return b.Div(mi.Class("space-y-2"),
		b.Div(mi.Class("text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider"), t("developer.forms.versions")),
		ListTable(ListTableConfig{
			Columns:   []ListColumn{{Label: t("developer.forms.version")}, {Label: t("developer.translations.col.text")}, {Label: "", Right: true}},
			EmptyIcon: "history", EmptyTitle: t("developer.forms.versions"),
		}, rows)(b))
}

// translationsPost reads and bounds a POST of the editor and resolves what it
// refers to. ok is false when it has already answered.
func (h *UIHandler) translationsPost(w http.ResponseWriter, r *http.Request) (claims *auth.Claims, locale, key, back, scope string, ok bool) {
	claims, has := auth.ClaimsFromContext(r.Context())
	rt := h.config.DevRuntime
	if !has || rt == nil {
		h.Error403(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, developerMaxBody)
	if err := r.ParseForm(); err != nil {
		developerRedirect(w, r, "/developer/translations", "error", "bad_request")
		return
	}
	locale, key = r.FormValue("locale"), r.FormValue("key")
	page, _ := strconv.Atoi(r.FormValue("page"))
	back = translationsURL(locale, developerTruncate(r.FormValue("q"), developerMaxQuery), translationShow(r.FormValue("show")), page, key)
	scopes := h.scopesWithGlobal(r.Context(), developerScopes(rt, rt.Locales != nil))
	if len(scopes) == 0 {
		http.Redirect(w, r, developerAppend(back, "error", "not_configured"), http.StatusSeeOther)
		return
	}
	if !developerScopeValid(r.FormValue("scope"), scopes) {
		http.Redirect(w, r, developerAppend(back, "error", "bad_scope"), http.StatusSeeOther)
		return
	}
	scope = developerScope(r.FormValue("scope"), scopes)
	back = developerAppend(back, "scope", scope)
	if h.translator == nil || !h.translator.HasLocale(locale) {
		http.Redirect(w, r, developerAppend("/developer/translations", "error", "unknown_locale"), http.StatusSeeOther)
		return
	}
	_, inRef := h.translator.Effective(claims.TenantID, developerRefLocale)[key]
	_, inLoc := h.translator.Effective(claims.TenantID, locale)[key]
	if !inRef && !inLoc {
		http.Redirect(w, r, developerAppend("/developer/translations", "error", "unknown_key"), http.StatusSeeOther)
		return
	}
	return claims, locale, key, back, scope, true
}

// DeveloperTranslationSave handles POST /developer/translations/save.
func (h *UIHandler) DeveloperTranslationSave(w http.ResponseWriter, r *http.Request) {
	claims, locale, key, back, scope, ok := h.translationsPost(w, r)
	if !ok {
		return
	}
	value, ref := r.FormValue("value"), h.translationReference(claims.TenantID, locale, key)
	var err error
	if scopeStored(scope) {
		err = h.config.DevRuntime.Overrides.SetTranslation(r.Context(), scopeTenantID(scope, claims.TenantID), locale, key, value, ref, claims.UserID)
	} else {
		err = h.config.DevRuntime.Locales.SetOverrideAs(locale, key, value, ref, claims.UserID)
	}
	if err != nil {
		h.logger.Warn("developer: translation override refused", "locale", locale, "key", key, "error", err)
		http.Redirect(w, r, developerAppend(back, "error", developerErrorCode(err)), http.StatusSeeOther)
		return
	}
	h.auditLogger.LogDeveloperChange(r.Context(), r, claims.UserID, claims.TenantID, "translation", key,
		map[string]any{"locale": locale, "action": "set", "scope": scope})
	http.Redirect(w, r, developerAppend(back, "msg", "saved")+"#edit", http.StatusSeeOther)
}

// DeveloperTranslationRemove handles POST /developer/translations/remove.
func (h *UIHandler) DeveloperTranslationRemove(w http.ResponseWriter, r *http.Request) {
	claims, locale, key, back, scope, ok := h.translationsPost(w, r)
	if !ok {
		return
	}
	var err error
	if scopeStored(scope) {
		err = h.config.DevRuntime.Overrides.RemoveTranslation(r.Context(), scopeTenantID(scope, claims.TenantID), locale, key, claims.UserID)
	} else {
		err = h.config.DevRuntime.Locales.RemoveOverrideAs(locale, key, claims.UserID)
	}
	if err != nil {
		h.logger.Warn("developer: translation override not removed", "locale", locale, "key", key, "error", err)
		http.Redirect(w, r, developerAppend(back, "error", developerErrorCode(err)), http.StatusSeeOther)
		return
	}
	h.auditLogger.LogDeveloperChange(r.Context(), r, claims.UserID, claims.TenantID, "translation", key,
		map[string]any{"locale": locale, "action": "remove", "scope": scope})
	http.Redirect(w, r, developerAppend(back, "msg", "removed")+"#edit", http.StatusSeeOther)
}
