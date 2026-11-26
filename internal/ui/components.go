package ui

import (
	"fmt"
	"strconv"

	mi "github.com/ha1tch/minty"
	"github.com/ha1tch/seam-ui/internal/auth"
)

// =============================================================================
// CSRF PROTECTION
// =============================================================================

// CSRFField creates a hidden input field for CSRF token.
func CSRFField(token string) mi.H {
	return func(b *mi.Builder) mi.Node {
		if token == "" {
			return b.Text("")
		}
		return b.Input(mi.Type("hidden"), mi.Name(auth.CSRFFormField), mi.Value(token))
	}
}

// =============================================================================
// CARDS
// =============================================================================

// Card creates a basic card component.
func Card(title string, content mi.H) mi.H {
	return func(b *mi.Builder) mi.Node {
		children := []interface{}{
			mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700"),
		}

		if title != "" {
			children = append(children,
				b.Div(mi.Class("px-6 py-4 border-b border-gray-200 dark:border-gray-700"),
					b.H3(mi.Class("text-lg font-semibold text-gray-900 dark:text-white"), title),
				),
			)
		}

		children = append(children,
			b.Div(mi.Class("p-6"), content(b)),
		)

		return b.Div(children...)
	}
}

// CardWithActions creates a card with header actions.
func CardWithActions(title string, actions mi.H, content mi.H) mi.H {
	return func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700"),
			b.Div(mi.Class("px-6 py-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between"),
				b.H3(mi.Class("text-lg font-semibold text-gray-900 dark:text-white"), title),
				actions(b),
			),
			b.Div(mi.Class("p-6"), content(b)),
		)
	}
}

// StatCard creates a statistics card.
func StatCard(label, value, change string, positive bool) mi.H {
	return func(b *mi.Builder) mi.Node {
		changeColor := "text-red-500"
		changeIcon := "trending_down"
		if positive {
			changeColor = "text-green-500"
			changeIcon = "trending_up"
		}

		return b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6"),
			b.Div(mi.Class("text-sm font-medium text-gray-500 dark:text-gray-400"), label),
			b.Div(mi.Class("mt-2 flex items-baseline gap-2"),
				b.Span(mi.Class("text-3xl font-bold text-gray-900 dark:text-white"), value),
				b.Span(mi.Class("flex items-center text-sm "+changeColor),
					b.Span(mi.Class("material-icons text-sm"), changeIcon),
					change,
				),
			),
		)
	}
}

// =============================================================================
// BUTTONS
// =============================================================================

// Button creates a styled button.
func Button(text, variant string, attrs ...mi.Attribute) mi.H {
	return func(b *mi.Builder) mi.Node {
		baseClass := "inline-flex items-center justify-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2"

		variantClass := ""
		switch variant {
		case "primary":
			variantClass = "bg-indigo-600 text-white hover:bg-indigo-700 focus:ring-indigo-500"
		case "secondary":
			variantClass = "bg-gray-100 text-gray-700 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600 focus:ring-gray-500"
		case "danger":
			variantClass = "bg-red-600 text-white hover:bg-red-700 focus:ring-red-500"
		case "success":
			variantClass = "bg-green-600 text-white hover:bg-green-700 focus:ring-green-500"
		case "ghost":
			variantClass = "text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-700 focus:ring-gray-500"
		default:
			variantClass = "bg-indigo-600 text-white hover:bg-indigo-700 focus:ring-indigo-500"
		}

		args := []interface{}{mi.Class(baseClass + " " + variantClass)}
		for _, attr := range attrs {
			args = append(args, attr)
		}
		args = append(args, text)

		return b.Button(args...)
	}
}

// IconButton creates a button with an icon.
func IconButton(icon, text, variant string, attrs ...mi.Attribute) mi.H {
	return func(b *mi.Builder) mi.Node {
		baseClass := "inline-flex items-center justify-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2"

		variantClass := ""
		switch variant {
		case "primary":
			variantClass = "bg-indigo-600 text-white hover:bg-indigo-700 focus:ring-indigo-500"
		case "secondary":
			variantClass = "bg-gray-100 text-gray-700 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600 focus:ring-gray-500"
		default:
			variantClass = "bg-indigo-600 text-white hover:bg-indigo-700 focus:ring-indigo-500"
		}

		args := []interface{}{mi.Class(baseClass + " " + variantClass)}
		for _, attr := range attrs {
			args = append(args, attr)
		}
		args = append(args,
			b.Span(mi.Class("material-icons text-lg"), icon),
		)
		if text != "" {
			args = append(args, b.Span(text))
		}

		return b.Button(args...)
	}
}

// =============================================================================
// BADGES
// =============================================================================

func badgeVariantClass(variant string) string {
	switch variant {
	case "success", "active":
		return "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300"
	case "warning", "silent":
		return "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-300"
	case "danger", "alert", "lost":
		return "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-300"
	case "info":
		return "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300"
	default:
		return "bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300"
	}
}

// Badge creates a status badge.
func Badge(text, variant string) mi.H {
	return func(b *mi.Builder) mi.Node {
		return b.Span(mi.Class("inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium "+badgeVariantClass(variant)), text)
	}
}

func IconBadge(icon, text, variant string, attrs ...mi.Attribute) mi.H {
	return func(b *mi.Builder) mi.Node {
		args := []interface{}{mi.Class("inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium " + badgeVariantClass(variant))}
		for _, attr := range attrs {
			args = append(args, attr)
		}
		args = append(args, b.Span(mi.Class("material-icons text-sm leading-none"), icon))
		if text != "" {
			args = append(args, b.Span(text))
		}
		return b.Span(args...)
	}
}

// StatusBadge creates a badge based on asset status.
func StatusBadge(status string) mi.H {
	return Badge(status, status)
}

// =============================================================================
// TABLES
// =============================================================================

// TableColumn defines a table column.
type TableColumn struct {
	Key      string
	Label    string
	Sortable bool
	Class    string
}

// Table creates a data table.
func Table(columns []TableColumn, rows []map[string]interface{}, actions func(row map[string]interface{}) mi.H) mi.H {
	return func(b *mi.Builder) mi.Node {
		// Header
		headerCells := make([]interface{}, 0, len(columns)+1)
		for _, col := range columns {
			cellClass := "px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider"
			if col.Class != "" {
				cellClass += " " + col.Class
			}

			if col.Sortable {
				headerCells = append(headerCells, b.Th(mi.Class(cellClass),
					b.Button(mi.Class("flex items-center gap-1 hover:text-gray-700 dark:hover:text-gray-200"),
						mi.Attr("data-sort", col.Key),
						col.Label,
						b.Span(mi.Class("material-icons text-sm"), "unfold_more"),
					),
				))
			} else {
				headerCells = append(headerCells, b.Th(mi.Class(cellClass), col.Label))
			}
		}

		// Add actions column if provided
		if actions != nil {
			headerCells = append(headerCells,
				b.Th(mi.Class("px-6 py-3 text-right text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider"),
					"Actions",
				),
			)
		}

		// Body rows
		bodyRows := make([]interface{}, len(rows))
		for i, row := range rows {
			cells := make([]interface{}, 0, len(columns)+1)
			for _, col := range columns {
				value := ""
				if v, ok := row[col.Key]; ok {
					value = fmt.Sprintf("%v", v)
				}
				cells = append(cells, b.Td(mi.Class("px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-gray-100"),
					value,
				))
			}

			if actions != nil {
				cells = append(cells,
					b.Td(mi.Class("px-6 py-4 whitespace-nowrap text-right text-sm"),
						actions(row)(b),
					),
				)
			}

			rowClass := "bg-white dark:bg-gray-800"
			if i%2 == 1 {
				rowClass = "bg-gray-50 dark:bg-gray-900"
			}

			rowArgs := append([]interface{}{mi.Class(rowClass)}, cells...)
			bodyRows[i] = b.Tr(rowArgs...)
		}

		return b.Div(mi.Class("overflow-x-auto"),
			b.Table(mi.Class("min-w-full divide-y divide-gray-200 dark:divide-gray-700"),
				b.Thead(mi.Class("bg-gray-50 dark:bg-gray-900"),
					b.Tr(headerCells...),
				),
				b.Tbody(append([]interface{}{mi.Class("divide-y divide-gray-200 dark:divide-gray-700")}, bodyRows...)...),
			),
		)
	}
}

// =============================================================================
// PAGINATION
// =============================================================================

// Pagination creates pagination controls. t translates "Previous" and "Next"
// (action.previous, action.next); nil leaves them in English.
func Pagination(currentPage, totalPages int, baseURL string, t func(key string, args ...any) string) mi.H {
	previous, next := "Previous", "Next"
	if t != nil {
		previous, next = t("action.previous"), t("action.next")
	}
	return func(b *mi.Builder) mi.Node {
		if totalPages <= 1 {
			return b.Div() // Empty
		}

		pages := make([]interface{}, 0)

		// Previous button
		prevClass := "px-3 py-2 rounded-lg text-sm font-medium"
		if currentPage <= 1 {
			prevClass += " text-gray-400 cursor-not-allowed"
			pages = append(pages, b.Span(mi.Class(prevClass), previous))
		} else {
			prevClass += " text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-700"
			pages = append(pages, b.A(mi.Href(baseURL+"?page="+strconv.Itoa(currentPage-1)), mi.Class(prevClass), previous))
		}

		// Page numbers
		for i := 1; i <= totalPages; i++ {
			if i == currentPage {
				pages = append(pages,
					b.Span(mi.Class("px-3 py-2 rounded-lg text-sm font-medium bg-indigo-600 text-white"),
						strconv.Itoa(i),
					),
				)
			} else if i == 1 || i == totalPages || (i >= currentPage-2 && i <= currentPage+2) {
				pages = append(pages,
					b.A(
						mi.Href(baseURL+"?page="+strconv.Itoa(i)),
						mi.Class("px-3 py-2 rounded-lg text-sm font-medium text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-700"),
						strconv.Itoa(i),
					),
				)
			} else if i == currentPage-3 || i == currentPage+3 {
				pages = append(pages, b.Span(mi.Class("px-2"), "..."))
			}
		}

		// Next button
		nextClass := "px-3 py-2 rounded-lg text-sm font-medium"
		if currentPage >= totalPages {
			nextClass += " text-gray-400 cursor-not-allowed"
			pages = append(pages, b.Span(mi.Class(nextClass), next))
		} else {
			nextClass += " text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-700"
			pages = append(pages, b.A(mi.Href(baseURL+"?page="+strconv.Itoa(currentPage+1)), mi.Class(nextClass), next))
		}

		return b.Div(append([]interface{}{mi.Class("flex items-center justify-center gap-1")}, pages...)...)
	}
}

// =============================================================================
// ALERTS
// =============================================================================

// Alert creates an alert message.
func Alert(message, variant string, dismissible bool) mi.H {
	return func(b *mi.Builder) mi.Node {
		baseClass := "p-4 rounded-lg flex items-start gap-3"

		var icon, colorClass string
		switch variant {
		case "success":
			icon = "check_circle"
			colorClass = "bg-green-50 text-green-800 dark:bg-green-900/50 dark:text-green-300"
		case "warning":
			icon = "warning"
			colorClass = "bg-yellow-50 text-yellow-800 dark:bg-yellow-900/50 dark:text-yellow-300"
		case "error", "danger":
			icon = "error"
			colorClass = "bg-red-50 text-red-800 dark:bg-red-900/50 dark:text-red-300"
		default:
			icon = "info"
			colorClass = "bg-blue-50 text-blue-800 dark:bg-blue-900/50 dark:text-blue-300"
		}

		children := []interface{}{
			mi.Class(baseClass + " " + colorClass),
			b.Span(mi.Class("material-icons flex-shrink-0"), icon),
			b.Span(mi.Class("flex-1"), message),
		}

		if dismissible {
			children = append(children,
				b.Button(
					mi.Type("button"),
					mi.Class("flex-shrink-0 hover:opacity-70"),
					mi.Attr("onclick", "this.parentElement.remove()"),
					b.Span(mi.Class("material-icons"), "close"),
				),
			)
		}

		return b.Div(children...)
	}
}

// =============================================================================
// EMPTY STATE
// =============================================================================

// EmptyState creates an empty state placeholder.
func EmptyState(icon, title, description string, action mi.H) mi.H {
	return func(b *mi.Builder) mi.Node {
		children := []interface{}{
			mi.Class("text-center py-12"),
			b.Div(mi.Class("text-gray-400 dark:text-gray-500 mb-4"),
				b.Span(mi.Class("material-icons text-6xl"), icon),
			),
			b.H3(mi.Class("text-lg font-medium text-gray-900 dark:text-white mb-2"), title),
			b.P(mi.Class("text-gray-500 dark:text-gray-400 mb-6"), description),
		}

		if action != nil {
			children = append(children, action(b))
		}

		return b.Div(children...)
	}
}

// =============================================================================
// LOADING
// =============================================================================

// LoadingSpinner creates a loading spinner.
func LoadingSpinner(size string) mi.H {
	return func(b *mi.Builder) mi.Node {
		sizeClass := "w-6 h-6"
		switch size {
		case "sm":
			sizeClass = "w-4 h-4"
		case "lg":
			sizeClass = "w-8 h-8"
		case "xl":
			sizeClass = "w-12 h-12"
		}

		return b.Div(mi.Class("flex items-center justify-center"),
			b.Div(mi.Class(sizeClass+" animate-spin rounded-full border-2 border-gray-300 border-t-indigo-600")),
		)
	}
}

// LoadingOverlay creates a loading overlay for HTMX requests.
func LoadingOverlay() mi.H {
	return func(b *mi.Builder) mi.Node {
		return b.Div(
			mi.Class("htmx-indicator absolute inset-0 bg-white/50 dark:bg-gray-900/50 flex items-center justify-center"),
			LoadingSpinner("lg")(b),
		)
	}
}

// =============================================================================
// SIMPLE CLICKABLE LISTS
// =============================================================================
//
// Added 2026-08-06. The same hand-rolled card+row structure appeared
// once in workOrdersListBody and once in issuesListBody, differing
// only in what each row actually shows. Their own status pills
// (workOrderStateBadge, issueStatusBadge) turned out to be a near-
// duplicate of an *already-existing* primitive here, Badge(text,
// variant) -- caught only by checking for a collision when adding
// this section, not planned; both modules now call Badge directly
// instead of hand-rolling their own pill markup.
//
// This list-card shape is deliberately NOT built on ListRow/
// ListCellRich/ListActions (this file's own <table>-based system,
// Tr/Td, per-cell columns, discrete view/edit/delete action icons per
// row) -- that system is the right fit for Assets, Users, and every
// masters entity, all genuinely multi-column, sortable-looking,
// per-action-icon lists. Work Orders and Issues are a different,
// simpler shape entirely: one clickable row IS the navigation to the
// detail page, no separate action icons, a title line, an optional
// subtitle line, and a badge on the right. Forcing that into a table
// would mean inventing a fake "view" action column for something
// that's already a single link -- a real downgrade, not a style
// preference, which is why this is a new, small primitive rather than
// an adaptation of the existing one.

// SimpleListItem is one row in a SimpleListCard -- a title, an
// optional subtitle, the href the whole row links to, and a
// pre-rendered badge (callers build their own via StatusBadge, since
// the badge's own color/label logic is each module's own domain
// knowledge, the same reasoning StatusBadge itself was extracted
// under).
type SimpleListItem struct {
	Href     string
	Title    string
	Subtitle string
	Meta     string
	Search   string
	Badge    mi.H
	Aside    mi.H
}

// SimpleListCard renders a bordered card containing a list of
// SimpleListItem rows, each a single clickable link, or an empty-state
// message when items is empty. The one shared primitive for this
// specific list shape -- Work Orders and Issues both use it; any
// future module with the same "clickable row -> detail page" shape
// should too, rather than a third hand-rolled copy.
func SimpleListCard(items []SimpleListItem, emptyMessage string) mi.H {
	return func(b *mi.Builder) mi.Node {
		if len(items) == 0 {
			return b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-8 text-center text-gray-500 dark:text-gray-400"),
				emptyMessage,
			)
		}
		rows := make([]interface{}, len(items))
		for i, item := range items {
			left := []interface{}{
				b.Div(mi.Class("font-medium text-gray-900 dark:text-white text-sm"), item.Title),
			}
			if item.Subtitle != "" {
				left = append(left, b.Div(mi.Class("text-xs text-gray-500 dark:text-gray-400"), item.Subtitle))
			}
			if item.Meta != "" {
				left = append(left, b.Div(mi.Class("text-xs text-gray-500 dark:text-gray-400"), item.Meta))
			}
			rowChildren := []interface{}{
				mi.Class("flex items-center justify-between gap-3 px-4 py-3 border-b border-gray-100 dark:border-gray-700 last:border-b-0 hover:bg-gray-50 dark:hover:bg-gray-700/50"),
				b.Div(append([]interface{}{mi.Class("flex min-w-0 flex-col")}, left...)...),
			}
			right := []interface{}{mi.Class("flex items-center gap-3 shrink-0")}
			if item.Badge != nil {
				right = append(right, item.Badge(b))
			}
			if item.Aside != nil {
				right = append(right, b.Div(mi.Class("border-l border-gray-200 dark:border-gray-700 pl-3"), item.Aside(b)))
			}
			if len(right) > 1 {
				rowChildren = append(rowChildren, b.Div(right...))
			}
			rowAttrs := []interface{}{mi.Href(item.Href)}
			if item.Search != "" {
				rowAttrs = append(rowAttrs, mi.Attr("data-search", item.Search))
			}
			rows[i] = b.A(append(rowAttrs, rowChildren...)...)
		}
		return b.Div(append([]interface{}{mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700")}, rows...)...)
	}
}
