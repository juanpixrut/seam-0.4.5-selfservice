package dashboardengine

import (
	"fmt"

	mi "github.com/ha1tch/minty"
)

// This file renders the five widget types (§4.1-4.5) from their compiled
// JSON (compile.go) -- every function reads a plain map[string]any/[]any
// shape, never a typed resolver result directly, so rendering is fully
// decoupled from how a widget's data was produced (cached or freshly
// resolved, look identical here). Same HTML structure, same Tailwind
// classes, same visual rules as the original internal/ui/dashboard.go
// (dashStatCard, dashRecentAlerts, dashVencimientos, dashTotalCard,
// dashStatusBreakdown) this whole package ports -- only where the data
// comes from changed in this rework, not what gets drawn from it.

func statCardWidget(b *mi.Builder, w Widget, compiled map[string]any) mi.Node {
	count := compiledInt(compiled, "count")
	borderClass := "border-gray-200 dark:border-gray-700"
	if w.Urgent && count > 0 {
		borderClass = "border-red-400 dark:border-red-500"
	}
	iconBg, iconFg := "bg-indigo-50 dark:bg-indigo-900/30", "text-indigo-500 dark:text-indigo-400"
	if w.Urgent && count > 0 {
		iconBg, iconFg = "bg-red-50 dark:bg-red-900/30", "text-red-500 dark:text-red-400"
	}
	countClass := "text-3xl font-bold text-gray-900 dark:text-white"
	if w.Urgent && count > 0 {
		countClass = "text-3xl font-bold text-red-600 dark:text-red-400"
	}
	return b.A(mi.Href(w.Href),
		mi.Class("block bg-white dark:bg-gray-800 rounded-lg shadow-sm border-2 "+borderClass+
			" p-6 hover:shadow-md transition-shadow"),
		b.Div(mi.Class("flex items-center gap-4"),
			b.Div(mi.Class("flex-shrink-0 w-12 h-12 rounded-xl flex items-center justify-center "+iconBg),
				b.Span(mi.Class("material-icons text-2xl "+iconFg), w.Icon),
			),
			b.Div(mi.Class("min-w-0"),
				b.Div(mi.Class("text-sm font-medium text-gray-500 dark:text-gray-400 mb-1"), w.Label),
				b.Div(mi.Class(countClass), formatNumber(count)),
			),
		),
	)
}

func recentListWidget(b *mi.Builder, w Widget, t func(key string, args ...any) string, compiled map[string]any) mi.Node {
	rows := compiledRows(compiled)
	header := b.Div(mi.Class("flex items-center justify-between mb-4"),
		b.H2(mi.Class("text-base font-semibold text-gray-900 dark:text-white"), w.Title),
		viewAllLink(b, w, t),
	)
	if len(rows) == 0 {
		return b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-5"),
			header,
			b.Div(mi.Class("flex items-center gap-3 py-4 text-gray-500 dark:text-gray-400"),
				b.Span(mi.Class("material-icons text-green-500 text-2xl"), w.EmptyIcon),
				b.Span(w.EmptyMessage),
			),
		)
	}
	items := make([]interface{}, len(rows))
	for i, row := range rows {
		badgeVal := mapString(row, w.BadgeField)
		cc := w.BadgeColors[badgeVal]
		if cc == "" {
			cc = "default"
		}
		secondary := mapString(row, w.SecondaryField)
		if secondary == "" && w.SecondaryFallbackField != "" {
			secondary = mapString(row, w.SecondaryFallbackField)
		}
		items[i] = b.A(mi.Href(fmt.Sprintf("%s/%v", w.ItemHrefPrefix, row["id"])),
			mi.Class("flex items-center justify-between py-3 border-b border-gray-100 dark:border-gray-700 last:border-0 hover:bg-gray-50 dark:hover:bg-gray-700/40 -mx-5 px-5 transition-colors"),
			b.Div(mi.Class("flex items-center gap-3 min-w-0"),
				b.Span(mi.Class("material-icons text-red-500 text-lg flex-shrink-0"), w.ItemIcon),
				b.Div(mi.Class("min-w-0"),
					b.Div(mi.Class("text-sm font-medium text-gray-900 dark:text-white truncate"), mapString(row, w.PrimaryField)),
					b.Div(mi.Class("text-xs text-gray-500 dark:text-gray-400"), secondary),
				),
			),
			b.Div(mi.Class("flex items-center gap-3 flex-shrink-0"),
				b.Span(mi.Class(badgeClass(cc)), badgeVal),
				b.Span(mi.Class("text-xs text-gray-400 dark:text-gray-500"), mapString(row, w.TimestampField)),
			),
		)
	}
	return b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-5"),
		header,
		b.Div(append([]interface{}{mi.Class("-my-1")}, items...)...),
	)
}

func urgencyListWidget(b *mi.Builder, w Widget, t func(key string, args ...any) string, compiled map[string]any) mi.Node {
	rows := compiledRows(compiled)
	header := b.Div(mi.Class("flex items-center justify-between mb-4"),
		b.H2(mi.Class("text-base font-semibold text-gray-900 dark:text-white"), w.Title),
		b.Span(mi.Class("text-xs text-gray-400 dark:text-gray-500"), w.Subtitle),
	)
	if len(rows) == 0 {
		return b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-5"),
			header,
			b.Div(mi.Class("py-4 text-sm text-gray-400 dark:text-gray-500 italic"), w.EmptyMessage),
		)
	}
	items := make([]interface{}, len(rows))
	for i, row := range rows {
		daysLeft := compiledInt(row, "days_left")
		urgencyClass, label := matchThreshold(w.Thresholds, daysLeft, t)
		items[i] = b.A(mi.Href(fmt.Sprintf("%s/%v", w.ItemHrefPrefix, row["entity_id"])),
			mi.Class("flex items-center justify-between py-3 border-b border-gray-100 dark:border-gray-700 last:border-0 hover:bg-gray-50 dark:hover:bg-gray-700/40 -mx-5 px-5 transition-colors"),
			b.Div(mi.Class("flex items-center gap-3 min-w-0"),
				b.Span(mi.Class("material-icons text-yellow-500 text-lg flex-shrink-0"), w.ItemIcon),
				b.Div(mi.Class("min-w-0"),
					b.Div(mi.Class("text-sm font-medium text-gray-900 dark:text-white truncate"), mapString(row, "entity_name")),
					b.Div(mi.Class("text-xs text-gray-500 dark:text-gray-400"), t("dashboard.field."+mapString(row, "field_name"))+" · "+mapString(row, "date")),
				),
			),
			b.Span(mi.Class("text-sm flex-shrink-0 "+urgencyClass), label),
		)
	}
	return b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-5"),
		header,
		b.Div(append([]interface{}{mi.Class("-my-1")}, items...)...),
	)
}

func totalCardWidget(b *mi.Builder, w Widget, compiled map[string]any) mi.Node {
	total := compiledInt(compiled, "total")
	return b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6"),
		b.Div(mi.Class("flex items-center gap-4 mb-4"),
			b.Div(mi.Class("flex-shrink-0 w-12 h-12 rounded-xl flex items-center justify-center bg-indigo-50 dark:bg-indigo-900/30"),
				b.Span(mi.Class("material-icons text-2xl text-indigo-500 dark:text-indigo-400"), w.Icon),
			),
			b.Div(
				b.Div(mi.Class("text-sm font-medium text-gray-500 dark:text-gray-400 mb-1"), w.Label),
				b.Div(mi.Class("text-4xl font-bold text-gray-900 dark:text-white"), formatNumber(total)),
			),
		),
		b.A(mi.Href(w.Href), mi.Class("inline-flex items-center gap-1 text-sm text-indigo-600 dark:text-indigo-400 hover:underline"),
			w.ViewLinkLabel),
	)
}

func breakdownListWidget(b *mi.Builder, w Widget, compiled map[string]any) mi.Node {
	rows := compiledRows(compiled)
	if len(rows) == 0 {
		return b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-5"),
			b.Div(mi.Class("text-sm text-gray-400 dark:text-gray-500 italic"), w.EmptyMessage),
		)
	}
	items := make([]interface{}, len(rows))
	for i, row := range rows {
		value := mapString(row, "value")
		label := w.Labels[value]
		if label == "" {
			label = value
		}
		dot := w.Colors[value]
		if dot == "" {
			dot = "gray-300"
		}
		items[i] = b.Div(mi.Class("flex items-center justify-between py-2"),
			b.Div(mi.Class("flex items-center gap-2"),
				b.Span(mi.Class("w-2.5 h-2.5 rounded-full bg-"+dot)),
				b.Span(mi.Class("text-sm text-gray-700 dark:text-gray-300"), label),
			),
			b.Span(mi.Class("text-sm font-medium text-gray-900 dark:text-white"), formatNumber(compiledInt(row, "count"))),
		)
	}
	return b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-5"),
		b.Div(mi.Class("text-sm font-medium text-gray-500 dark:text-gray-400 mb-3"), w.Title),
		b.Div(append([]interface{}{mi.Class("divide-y divide-gray-100 dark:divide-gray-700")}, items...)...),
	)
}

// -- shared helpers --

func viewAllLink(b *mi.Builder, w Widget, t func(key string, args ...any) string) mi.Node {
	if w.ViewAllHref == "" {
		return b.Span() // no link -- e.g. System's own dashboard sections (§9), which have nowhere to "view all" to
	}
	label := "dashboard.view_all"
	if t != nil {
		label = t("dashboard.view_all")
	}
	return b.A(mi.Href(w.ViewAllHref),
		mi.Class("text-sm text-indigo-600 dark:text-indigo-400 hover:underline"), label)
}

// compiledRows reads compiled["rows"] as []map[string]any -- the shape
// every rows-mode compile template (compile.go) produces. Absent or
// wrongly-shaped is treated as zero rows, not an error: a widget with no
// data renders its empty state, the same as an empty resolver result
// always did.
func compiledRows(compiled map[string]any) []map[string]any {
	raw, _ := compiled["rows"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, r := range raw {
		if m, ok := r.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// compiledInt reads an integer field from compiled JSON data.
// encoding/json unmarshals every JSON number as float64 by default (Go's
// standard, not something this package can avoid without a custom
// decoder) -- this is the one place that's converted back.
func compiledInt(m map[string]any, field string) int {
	switch v := m[field].(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

func mapString(row map[string]any, field string) string {
	if field == "" {
		return ""
	}
	v, ok := row[field]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func badgeClass(color string) string {
	classes := map[string]string{
		"danger":  "bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300",
		"warning": "bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300",
		"success": "bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300",
		"default": "bg-gray-100 text-gray-700 dark:bg-gray-700/40 dark:text-gray-300",
	}
	base := "inline-flex items-center px-2 py-0.5 rounded text-xs font-medium "
	if c, ok := classes[color]; ok {
		return base + c
	}
	return base + classes["default"]
}

// matchThreshold implements DASHBOARD_SPEC.md §4.3's evaluation rule:
// first entry whose MaxDays is nil or >= daysLeft wins.
func matchThreshold(thresholds []Threshold, daysLeft int, t func(key string, args ...any) string) (class, label string) {
	for _, th := range thresholds {
		if th.MaxDays == nil || *th.MaxDays >= daysLeft {
			return urgencyClassFor(th.Class), t(th.Label, daysLeft)
		}
	}
	return "text-gray-600 dark:text-gray-400", fmt.Sprintf("%d", daysLeft)
}

func urgencyClassFor(class string) string {
	switch class {
	case "urgent":
		return "text-red-600 dark:text-red-400 font-semibold"
	case "warning":
		return "text-yellow-600 dark:text-yellow-400 font-medium"
	default:
		return "text-gray-600 dark:text-gray-400"
	}
}

func formatNumber(n int) string {
	if n >= 1000000 {
		return formatFloat(float64(n)/1000000) + "M"
	}
	if n >= 1000 {
		return formatFloat(float64(n)/1000) + "K"
	}
	return intToString(n)
}

func formatFloat(f float64) string {
	whole := int(f)
	frac := int((f - float64(whole)) * 10)
	if frac == 0 {
		return intToString(whole)
	}
	return intToString(whole) + "." + intToString(frac)
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + intToString(-n)
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
