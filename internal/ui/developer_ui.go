package ui

import (
	"strconv"

	mi "github.com/ha1tch/minty"
)

// The Developer module's page pieces. Every page is built from the same
// components and listing primitives as the rest of the application (ListTable,
// ListRow, Badge, Button, Alert, EmptyState, the card and the tab strip), so
// the module looks like the product it serves. What is specific to it and has
// no shared counterpart is here: the page frame, the titled panel, the summary
// tile, the segmented choice, the tab strip that switches panels in place and
// the pager whose links keep the page's filters.

const (
	devCardClass  = "bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700"
	devMutedClass = "text-sm text-gray-500 dark:text-gray-400"
	devMonoClass  = "font-mono text-xs text-gray-500 dark:text-gray-400"
	devInputClass = "block w-full rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
	devLabelClass = "block text-sm font-medium text-gray-700 dark:text-gray-300"
	devLinkClass  = "inline-flex items-center gap-2 px-4 py-2 bg-gray-100 text-gray-700 rounded-lg hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600 text-sm font-medium"
	devIconBtn    = "p-1 rounded text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700 hover:text-indigo-600 dark:hover:text-indigo-400"
	devIconDanger = "p-1 rounded text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700 hover:text-red-600 dark:hover:text-red-400"
)

func developerTrans(t func(key string, args ...any) string) func(key string, args ...any) string {
	if t == nil {
		return func(key string, args ...any) string { return key }
	}
	return t
}

// developerHeader is what a page's frame shows above its parts: where the page
// hangs from (Back, optional) and the actions on its right (optional).
type developerHeader struct {
	Title, Intro      string
	BackURL, BackText string
	Actions           []interface{}
}

// developerShell is the frame every section shares: title, intro, the message
// and error banners, then the section's own parts.
func developerShell(b *mi.Builder, title, intro string, data DeveloperPageData, parts ...interface{}) mi.Node {
	return developerFrame(b, developerHeader{Title: title, Intro: intro}, data, parts...)
}

func developerFrame(b *mi.Builder, h developerHeader, data DeveloperPageData, parts ...interface{}) mi.Node {
	left := []interface{}{mi.Class("min-w-0")}
	if h.BackURL != "" {
		left = append(left, b.A(mi.Href(h.BackURL), mi.Class("inline-flex items-center gap-1 text-sm text-indigo-600 dark:text-indigo-400 hover:underline"),
			b.Span(mi.Class("material-icons text-sm"), "arrow_back"), b.Span(h.BackText)))
	}
	left = append(left, b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), h.Title))
	if h.Intro != "" {
		left = append(left, b.P(mi.Class(devMutedClass+" mt-0.5"), h.Intro))
	}
	head := []interface{}{mi.Class("flex items-start justify-between gap-4"), b.Div(left...)}
	if len(h.Actions) > 0 {
		head = append(head, b.Div(append([]interface{}{mi.Class("flex items-center gap-2 shrink-0")}, h.Actions...)...))
	}
	body := []interface{}{mi.Class("dev-page space-y-6"), b.Div(head...)}
	if data.Message != "" {
		body = append(body, b.Div(mi.Class("dev-msg dev-msg-ok"), Alert(data.Message, "success", true)(b)))
	}
	if data.Error != "" {
		body = append(body, b.Div(mi.Class("dev-msg dev-msg-err"), Alert(data.Error, "error", true)(b)))
	}
	return b.Div(append(body, parts...)...)
}

// developerPanel is a card with a header (icon, title, an optional note and
// optional actions on the right) and a body. flush leaves the body without
// padding, for a table or a list of rows that reaches the card's edges.
func developerPanel(b *mi.Builder, icon, title, note string, actions []interface{}, flush bool, body ...interface{}) mi.Node {
	text := []interface{}{mi.Class("min-w-0"), b.H2(mi.Class("text-base font-semibold text-gray-900 dark:text-white dev-wrap"), title)}
	if note != "" {
		text = append(text, b.P(mi.Class(devMutedClass), note))
	}
	left := []interface{}{mi.Class("dev-panel-title flex items-start gap-3")}
	if icon != "" {
		left = append(left, b.Span(mi.Class("material-icons text-indigo-600 dark:text-indigo-400"), icon))
	}
	left = append(left, b.Div(text...))
	head := []interface{}{mi.Class("dev-panel-head px-4 py-3 border-b border-gray-200 dark:border-gray-700"), b.Div(left...)}
	if len(actions) > 0 {
		head = append(head, b.Div(append([]interface{}{mi.Class("flex items-center gap-2 shrink-0")}, actions...)...))
	}
	bodyClass := "p-4 space-y-4"
	if flush {
		bodyClass = "overflow-x-auto"
	}
	return b.Div(mi.Class(devCardClass+" overflow-hidden"), b.Div(head...), b.Div(append([]interface{}{mi.Class(bodyClass)}, body...)...))
}

// developerSection is a titled block of a page without a card of its own: a
// heading, an optional note, then the content (usually a grid of cards).
func developerSection(b *mi.Builder, title, note string, content mi.Node) mi.Node {
	parts := []interface{}{mi.Class("dev-section space-y-3"),
		b.H2(mi.Class("text-lg font-semibold text-gray-900 dark:text-white"), title)}
	if note != "" {
		parts = append(parts, b.P(mi.Class(devMutedClass), note))
	}
	return b.Div(append(parts, content)...)
}

// developerStat is a summary tile that is also the way into a section: an
// icon, a label, the figure and a line under it.
func developerStat(b *mi.Builder, icon, label, value, sub, href string) mi.Node {
	inner := []interface{}{
		b.Div(mi.Class("flex items-center justify-between"),
			b.Span(mi.Class("text-sm font-medium text-gray-500 dark:text-gray-400"), label),
			b.Span(mi.Class("material-icons text-indigo-600 dark:text-indigo-400"), icon)),
		b.Div(mi.Class("mt-2 text-3xl font-bold text-gray-900 dark:text-white"), value),
	}
	if sub != "" {
		inner = append(inner, b.Div(mi.Class("mt-1 text-xs text-gray-500 dark:text-gray-400"), sub))
	}
	return b.A(append([]interface{}{mi.Href(href), mi.Class("dev-stat block " + devCardClass + " p-4")}, inner...)...)
}

// developerSegItem is one choice of a segmented control drawn as links.
type developerSegItem struct {
	Label, Href string
	Active      bool
}

func developerSeg(b *mi.Builder, items []developerSegItem) mi.Node {
	seg := []interface{}{mi.Class("dev-seg"), mi.Attr("role", "group")}
	for _, it := range items {
		cls, cur := "dev-seg-btn", "false"
		if it.Active {
			cls, cur = cls+" is-active", "true"
		}
		seg = append(seg, b.A(mi.Class(cls), mi.Href(it.Href), mi.Attr("aria-current", cur), it.Label))
	}
	return b.Div(seg...)
}

// developerTab is one panel of a tab strip that switches in place (SeamTabs).
type developerTab struct {
	ID, Label string
	Count     int // shown next to the label when above zero
	Body      []interface{}
}

// developerTabs renders the strip and its panels inside one card; open is the
// ID of the panel to show first ("" for the first one).
func developerTabs(b *mi.Builder, tabs []developerTab, open string) mi.Node {
	const (
		base = "dev-tab-btn px-4 py-3 text-sm font-medium border-b-2 whitespace-nowrap cursor-pointer transition-colors "
		// Only the classes SeamTabs swaps: a dark: variant left on a button would
		// outlive the swap and show the wrong tab as open in dark mode, so the dark
		// colours are in app.css (.dev-tab-btn).
		active   = "border-indigo-600 text-indigo-600"
		inactive = "border-transparent text-gray-500"
	)
	strip := []interface{}{mi.Class("flex gap-2 px-2 border-b border-gray-200 dark:border-gray-700 overflow-x-auto")}
	panels := []interface{}{}
	for i, tab := range tabs {
		cls := base + inactive
		if i == 0 {
			cls = base + active
		}
		label := []interface{}{mi.Type("button"), mi.Class(cls), mi.Attr("data-tab", tab.ID), b.Span(tab.Label)}
		if tab.Count > 0 {
			label = append(label, " ", Badge(strconv.Itoa(tab.Count), "info")(b))
		}
		strip = append(strip, b.Button(label...))
		panels = append(panels, b.Div(append([]interface{}{mi.ID(tab.ID), mi.Class("dev-tab-panel p-4 space-y-4")}, tab.Body...)...))
	}
	// app.js is deferred: on a full page load SeamTabs does not exist yet when
	// this inline script runs, so wait for DOMContentLoaded then.
	openJS := ""
	if open != "" {
		openJS = `var o=document.querySelector('.dev-tab-btn[data-tab="` + open + `"]');if(o){o.click();}`
	}
	script := b.Script(mi.Raw(`(function(){function go(){SeamTabs.wire('.dev-tab-panel','.dev-tab-btn');` + openJS +
		`}if(window.SeamTabs){go();}else{document.addEventListener('DOMContentLoaded',go);}})();`))
	return b.Div(append([]interface{}{mi.Class(devCardClass + " overflow-hidden"), b.Div(strip...)}, append(panels, script)...)...)
}

// developerPager is the count line and the page links of a filtered list;
// urlFor builds a page's address with the filters kept. The links are only
// drawn when there is more than one page.
func developerPager(b *mi.Builder, t func(key string, args ...any) string, page, pages int, count string, urlFor func(page int) string) mi.Node {
	bar := []interface{}{mi.Class("flex items-center justify-between"), b.Div(mi.Class(devMutedClass), count)}
	if pages > 1 {
		const (
			link = "px-3 py-2 rounded-lg text-sm font-medium text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-700"
			off  = "px-3 py-2 rounded-lg text-sm font-medium text-gray-400 cursor-not-allowed"
		)
		nav := []interface{}{mi.Class("dev-pager flex items-center justify-center gap-1")}
		if page > 1 {
			nav = append(nav, b.A(mi.Href(urlFor(page-1)), mi.Class(link), t("developer.translations.prev")))
		} else {
			nav = append(nav, b.Span(mi.Class(off), t("developer.translations.prev")))
		}
		for i := 1; i <= pages; i++ {
			switch {
			case i == page:
				nav = append(nav, b.Span(mi.Class("px-3 py-2 rounded-lg text-sm font-medium bg-indigo-600 text-white"), strconv.Itoa(i)))
			case i == 1 || i == pages || (i >= page-2 && i <= page+2):
				nav = append(nav, b.A(mi.Href(urlFor(i)), mi.Class(link), strconv.Itoa(i)))
			case i == page-3 || i == page+3:
				nav = append(nav, b.Span(mi.Class("px-2 text-gray-400"), "…"))
			}
		}
		if page < pages {
			nav = append(nav, b.A(mi.Href(urlFor(page+1)), mi.Class(link), t("developer.translations.next")))
		} else {
			nav = append(nav, b.Span(mi.Class(off), t("developer.translations.next")))
		}
		bar = append(bar, b.Div(nav...))
	}
	return b.Div(bar...)
}

// developerExportLink is the header action that downloads the tenant's
// overrides (developer_export.go).
func developerExportLink(b *mi.Builder, t func(key string, args ...any) string) mi.Node {
	return b.A(mi.Href("/developer/export"), mi.Class(devLinkClass), mi.Attr("download", "download"), mi.Attr("title", t("developer.export.hint")),
		b.Span(mi.Class("material-icons text-lg"), "download"), b.Span(t("developer.export.action")))
}

// developerNewLink is the header's primary action: a link that opens a page's
// "new" panel.
func developerNewLink(b *mi.Builder, href, label string) mi.Node {
	return b.A(mi.Href(href), mi.Class("inline-flex items-center gap-2 px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 text-sm font-medium"),
		b.Span(mi.Class("material-icons text-lg"), "add"), b.Span(label))
}

// developerScopeSelect is the choice of where something new is created: a
// select of the scopes on offer, or a hidden field when there is only one.
func developerScopeSelect(b *mi.Builder, t func(key string, args ...any) string, scopes []string, scope string) mi.Node {
	if len(scopes) < 2 {
		return b.Div(b.Input(mi.Type("hidden"), mi.Name("scope"), mi.Value(scope)),
			b.P(mi.Class(devMutedClass), t("developer.scope."+scope+".hint")))
	}
	opts := []interface{}{mi.Name("scope"), mi.ID("dev-new-scope"), mi.Class(devInputClass + " mt-1")}
	for _, s := range scopes {
		o := []interface{}{mi.Value(s)}
		if s == scope {
			o = append(o, mi.Attr("selected", "selected"))
		}
		opts = append(opts, b.Option(append(o, t("developer.scope."+s)+": "+t("developer.scope."+s+".hint"))...))
	}
	return b.Div(b.Label(mi.For("dev-new-scope"), mi.Class(devLabelClass), t("developer.scope.label")), b.Select(opts...))
}

// developerLayerBadge is the pill that says which layer supplies a text or a
// form: grey for the built-in one, coloured for an override.
func developerLayerBadge(b *mi.Builder, t func(key string, args ...any) string, layer string) mi.Node {
	variant := "info"
	switch layer {
	case "embedded":
		variant = ""
	case "filesystem":
		variant = "warning"
	}
	return Badge(t("developer.layer."+layer), variant)(b)
}

// developerDL is a short list of label and value pairs, one under the other.
func developerDL(b *mi.Builder, pairs ...[2]string) mi.Node {
	rows := []interface{}{mi.Class("space-y-3")}
	for _, p := range pairs {
		rows = append(rows, b.Div(
			b.Div(mi.Class("text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider"), p[0]),
			b.Div(mi.Class("mt-1 text-sm text-gray-900 dark:text-white dev-wrap"), p[1])))
	}
	return b.Div(rows...)
}
