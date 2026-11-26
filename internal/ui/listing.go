package ui

import (
	mi "github.com/ha1tch/minty"
)

// =============================================================================
// LISTING ENGINE
// =============================================================================
// One set of composable primitives used by every table-style listing page.
// Card-grid listings (AssetTypes) are a separate invariant and do not use this.
//
//   ListPage           — outer shell: header, filter bar, content, pagination
//   ListTable          — card-wrapped table: thead + striped tbody + empty state
//   ListRow            — striped <tr> of cell mi.H values
//   ListCell           — standard text <td>
//   ListCellMuted      — muted text <td> (IDs, codes, timestamps)
//   ListCellRich       — <td> wrapping arbitrary mi.H content (badges, icons)
//   ListCellRight      — right-aligned <td> wrapping arbitrary mi.H
//   ListActions        — standard icon-button actions cell (view / edit / delete)
//   ListFilterBar      — GET form: optional search input + auto-submit selects
//   ListPaginationBar  — "Showing N of M" count + Pagination widget
// =============================================================================

// ─── Page shell ──────────────────────────────────────────────────────────────

// ListPageConfig configures the ListPage outer shell.
type ListPageConfig struct {
	Title              string
	Subtitle           string // shown below title as small muted line (optional)
	CreateURL          string // empty → no primary action button
	CreateLabel        string
	CreateHTMX         bool   // true → open SeamModal; false → navigate to CreateURL
	CreateIcon         string // defaults to "add"
	SecondaryURL       string // optional secondary action (e.g. export); empty → omit
	SecondaryLabel     string
	SecondaryIcon      string // defaults to "download"
	MaxWidth           string // Tailwind max-w-* class e.g. "max-w-2xl"; empty → full width
	CreateTargetModal2 bool   // true → open SeamModal2 for create (masters, sensors)
	ModalWidth         string // pixel width for SeamModal2 dialogs e.g. "600px"
}

// ListPage renders the standard listing page shell.
// Pass nil for filterBar or pagination to omit those slots.
func ListPage(cfg ListPageConfig, filterBar mi.H, content mi.H, pagination mi.H) mi.H {
	return func(b *mi.Builder) mi.Node {
		icon := cfg.CreateIcon
		if icon == "" {
			icon = "add"
		}

		// Primary action
		var primaryBtn mi.Node
		if cfg.CreateURL != "" {
			if cfg.CreateTargetModal2 {
				var m2co string
				if cfg.ModalWidth != "" {
					m2co = "SeamModal2.open('" + cfg.CreateLabel + "','" + cfg.CreateURL +
						"',{maxWidth:'" + cfg.ModalWidth + "',anchorEl:this.closest('[data-list-modal-anchor]')})"
				} else {
					m2co = "SeamModal2.open('" + cfg.CreateLabel + "','" + cfg.CreateURL + "')"
				}
				primaryBtn = IconButton(icon, cfg.CreateLabel, "primary",
					mi.Attr("onclick", m2co),
				)(b)
			} else if cfg.CreateHTMX {
				primaryBtn = IconButton(icon, cfg.CreateLabel, "primary",
					mi.Attr("hx-get", cfg.CreateURL),
					mi.Attr("hx-target", "#seam-modal-body"),
					mi.Attr("hx-swap", "innerHTML"),
					mi.Attr("onclick", "SeamModal.open('"+cfg.CreateLabel+"')"),
				)(b)
			} else {
				primaryBtn = b.A(
					mi.Href(cfg.CreateURL),
					mi.Class("inline-flex items-center gap-2 px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 text-sm font-medium"),
					b.Span(mi.Class("material-icons text-lg"), icon),
					b.Span(cfg.CreateLabel),
				)
			}
		}

		// Secondary action
		var secondaryBtn mi.Node
		if cfg.SecondaryURL != "" {
			secIcon := cfg.SecondaryIcon
			if secIcon == "" {
				secIcon = "download"
			}
			secondaryBtn = b.A(
				mi.Href(cfg.SecondaryURL),
				mi.Class("inline-flex items-center gap-2 px-4 py-2 bg-gray-100 text-gray-700 rounded-lg hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600 text-sm font-medium"),
				b.Span(mi.Class("material-icons text-lg"), secIcon),
				b.Span(cfg.SecondaryLabel),
			)
		}

		// Right side of header
		var headerRight mi.Node
		if primaryBtn != nil || secondaryBtn != nil {
			items := []interface{}{mi.Class("flex items-center gap-2")}
			if secondaryBtn != nil {
				items = append(items, secondaryBtn)
			}
			if primaryBtn != nil {
				items = append(items, primaryBtn)
			}
			headerRight = b.Div(items...)
		} else {
			headerRight = b.Span()
		}

		// Left side: title + optional subtitle
		var headerLeft mi.Node
		if cfg.Subtitle != "" {
			headerLeft = b.Div(
				b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), cfg.Title),
				b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400 mt-0.5"), cfg.Subtitle),
			)
		} else {
			headerLeft = b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), cfg.Title)
		}

		pageCls := "space-y-6"
		if cfg.MaxWidth != "" {
			pageCls = "space-y-6 " + cfg.MaxWidth + " mx-auto"
		}
		nodes := []interface{}{
			mi.Class(pageCls),
			b.Div(mi.Class("flex items-center justify-between"), headerLeft, headerRight),
		}
		// Issue 4: anchor marker for SeamModal2 positioning
		if cfg.ModalWidth != "" {
			nodes = append([]interface{}{nodes[0], mi.Attr("data-list-modal-anchor", "1")}, nodes[1:]...)
		}
		if filterBar != nil {
			nodes = append(nodes, filterBar(b))
		}
		if content != nil {
			nodes = append(nodes, content(b))
		}
		if pagination != nil {
			nodes = append(nodes, pagination(b))
		}
		return b.Div(nodes...)
	}
}

// ─── Table ────────────────────────────────────────────────────────────────────

// ListColumn defines a single column header in ListTable.
type ListColumn struct {
	Label string
	Right bool // right-align the header and signal right-aligned content
}

// ListTableConfig configures ListTable.
type ListTableConfig struct {
	Columns      []ListColumn
	EmptyIcon    string
	EmptyTitle   string
	EmptyMessage string
	EmptyAction  mi.H // optional CTA shown in the empty state (pass nil to omit)
	SkipCard     bool // true → omit the card wrapper; use when caller provides the card
}

// ListTable renders a card-wrapped table with a consistent thead and striped rows.
// When rows is empty it renders the empty state instead of the table.
// Build rows with ListRow(); cells with ListCell / ListCellMuted /
// ListCellRich / ListCellRight / ListActions.
func ListTable(cfg ListTableConfig, rows []mi.H) mi.H {
	const card = "bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 overflow-hidden"
	const thBase = "px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider"

	return func(b *mi.Builder) mi.Node {
		if len(rows) == 0 {
			empty := EmptyState(cfg.EmptyIcon, cfg.EmptyTitle, cfg.EmptyMessage, cfg.EmptyAction)(b)
			if cfg.SkipCard {
				return empty
			}
			return b.Div(mi.Class(card), empty)
		}

		headerCells := make([]interface{}, len(cfg.Columns))
		for i, col := range cfg.Columns {
			cls := thBase + " text-left"
			if col.Right {
				cls = thBase + " text-right"
			}
			headerCells[i] = b.Th(mi.Class(cls), col.Label)
		}

		rowNodes := make([]interface{}, len(rows))
		for i, row := range rows {
			rowNodes[i] = row(b)
		}

		table := b.Table(mi.Class("w-full text-sm"),
			b.Thead(mi.Class("bg-gray-50 dark:bg-gray-900"),
				b.Tr(headerCells...),
			),
			b.Tbody(rowNodes...),
		)

		if cfg.SkipCard {
			return table
		}
		return b.Div(mi.Class(card), table)
	}
}

// ─── Row and cell helpers ─────────────────────────────────────────────────────

// ListRow builds a consistently striped <tr> from the provided cell values.
// index is the zero-based row index used for even/odd striping.
func ListRow(index int, cells ...mi.H) mi.H {
	return ListRowWithAttrs(index, nil, cells...)
}

// ListRowWithAttrs is ListRow plus extra attributes on the <tr> (T-96: the
// Selector dialog marks each row with data-id / data-label / onclick).
// attrs are mi.Attr(...) values (or any other attribute the builder
// accepts), applied after the stripe class; nil or empty gives exactly the
// same <tr> as ListRow. Do not pass a mi.Class here: the stripe class is
// already set, and a second class attribute would conflict with it.
func ListRowWithAttrs(index int, attrs []interface{}, cells ...mi.H) mi.H {
	return func(b *mi.Builder) mi.Node {
		rowClass := "bg-white dark:bg-gray-800 hover:bg-indigo-50 dark:hover:bg-indigo-900/20 transition-colors"
		if index%2 == 1 {
			rowClass = "bg-gray-50 dark:bg-gray-900 hover:bg-indigo-50 dark:hover:bg-indigo-900/20 transition-colors"
		}
		items := []interface{}{mi.Class(rowClass)}
		items = append(items, attrs...)
		for _, cell := range cells {
			items = append(items, cell(b))
		}
		return b.Tr(items...)
	}
}

// ListCell renders a standard left-aligned text <td> (primary colour).
func ListCell(content string) mi.H {
	return func(b *mi.Builder) mi.Node {
		return b.Td(mi.Class("px-4 py-3 text-sm text-gray-900 dark:text-white whitespace-nowrap"), content)
	}
}

// ListCellMuted renders a left-aligned text <td> in muted gray.
// Use for secondary columns: IDs, codes, timestamps, protocol names.
func ListCellMuted(content string) mi.H {
	return func(b *mi.Builder) mi.Node {
		return b.Td(mi.Class("px-4 py-3 text-sm text-gray-500 dark:text-gray-400 whitespace-nowrap"), content)
	}
}

// ListCellRich renders a <td> wrapping arbitrary mi.H content.
// Use for badges, coloured text, icons, or any composite cell content.
func ListCellRich(content mi.H) mi.H {
	return func(b *mi.Builder) mi.Node {
		return b.Td(mi.Class("px-4 py-3 text-sm"), content(b))
	}
}

// ListCellLink renders a text cell whose content is a hyperlink: the KEY LINK
// of a navigation list. It carries data-row-link, which makes the whole row
// clickable (SeamList in app.js): clicking the row anywhere that is not itself
// interactive follows this link. See docs/LIST_SELECTION.md for the criteria.
func ListCellLink(text, href string) mi.H {
	return func(b *mi.Builder) mi.Node {
		return b.Td(mi.Class("px-4 py-3 text-sm"),
			b.A(mi.Href(href), mi.Attr("data-row-link", "1"), mi.Class("font-medium text-indigo-600 dark:text-indigo-400 hover:underline"), text),
		)
	}
}

// ListCellRight renders a right-aligned <td> wrapping arbitrary mi.H content.
func ListCellRight(content mi.H) mi.H {
	return func(b *mi.Builder) mi.Node {
		return b.Td(mi.Class("px-4 py-3 text-right"), content(b))
	}
}

// ─── Actions cell ─────────────────────────────────────────────────────────────

// ListActionsConfig declares which icon buttons to show in the actions column.
// Leave a URL field empty to omit that button.
type ListActionsConfig struct {
	ViewURL          string // "visibility" icon; empty → omit
	EditURL          string // "edit" icon; empty → omit
	EditHTMX         bool   // true → open SeamModal (v1, full-size)
	EditTargetModal2 bool   // true → call SeamModal2.open(title,url) instead
	EditModalTitle   string // modal title (default "Editar")
	EditModalWidth   string // optional pixel width for SeamModal2 e.g. "600px"
	DeleteURL        string // "delete" icon as a POST form; empty → omit
	DeleteName       string // entity name for "Are you sure you want to delete X?" confirmation
	CSRFToken        string
	// T is the current request's translate function. A struct field, not
	// a second ListActions parameter -- this config is used across many
	// unrelated files (Assets, Events, Sensors, every masters entity);
	// adding a field keeps every existing call site valid with its
	// zero-value default (nil, same graceful fallback as everywhere
	// else) rather than forcing every caller to be touched at once.
	T func(key string, args ...any) string
}

// ListActions returns the standard right-aligned actions cell (a complete <td>).
// It replaces masterDeleteButton + text "Editar" links in masters, and
// harmonises with the icon-button style used in Assets and Events.
func ListActions(cfg ListActionsConfig) mi.H {
	// Every list page passes cfg.T; the literals are what a caller without
	// a translator shows, in English, the interface's base language.
	view, edit, del, confirm := "View", "Edit", "Delete", "Are you sure you want to delete this?"
	if cfg.T != nil {
		view, edit, del, confirm = cfg.T("action.view"), cfg.T("action.edit"), cfg.T("action.delete"), cfg.T("confirm.delete")
	}
	return func(b *mi.Builder) mi.Node {
		iconSpan := func(icon, title string) mi.Node {
			return b.Span(
				mi.Class("material-icons text-lg"),
				mi.Attr("title", title),
				icon,
			)
		}
		btnCls := "p-1 rounded text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700 "

		var btns []interface{}
		btns = append(btns, mi.Class("flex items-center justify-end gap-1"))

		if cfg.ViewURL != "" {
			btns = append(btns, b.A(
				mi.Href(cfg.ViewURL),
				mi.Class(btnCls+"hover:text-indigo-600 dark:hover:text-indigo-400"),
				iconSpan("visibility", view),
			))
		}

		if cfg.EditURL != "" {
			title := cfg.EditModalTitle
			if title == "" {
				title = edit
			}
			if cfg.EditHTMX && cfg.EditTargetModal2 {
				var m2eo string
				if cfg.EditModalWidth != "" {
					m2eo = ",{maxWidth:'" + cfg.EditModalWidth + "',anchorEl:this.closest('[data-list-modal-anchor]')}"
				}
				btns = append(btns, b.Button(
					mi.Class(btnCls+"hover:text-indigo-600 dark:hover:text-indigo-400"),
					mi.Attr("onclick", "SeamModal2.open('"+title+"','"+cfg.EditURL+"'"+m2eo+")"),
					iconSpan("edit", edit),
				))
			} else if cfg.EditHTMX {
				btns = append(btns, b.Button(
					mi.Class(btnCls+"hover:text-indigo-600 dark:hover:text-indigo-400"),
					mi.Attr("hx-get", cfg.EditURL),
					mi.Attr("hx-target", "#seam-modal-body"),
					mi.Attr("hx-swap", "innerHTML"),
					mi.Attr("onclick", "SeamModal.open('"+title+"')"),
					iconSpan("edit", edit),
				))
			} else {
				btns = append(btns, b.A(
					mi.Href(cfg.EditURL),
					mi.Class(btnCls+"hover:text-indigo-600 dark:hover:text-indigo-400"),
					iconSpan("edit", edit),
				))
			}
		}

		if cfg.DeleteURL != "" {
			btns = append(btns, b.Form(
				mi.Method("post"),
				mi.Attr("action", cfg.DeleteURL),
				mi.Class("inline"),
				b.Input(mi.Type("hidden"), mi.Name("_csrf"), mi.Attr("value", cfg.CSRFToken)),
				b.Button(
					mi.Type("submit"),
					mi.Class(btnCls+"hover:text-red-600 dark:hover:text-red-400"),
					mi.Attr("onclick", func() string {
						if cfg.DeleteName != "" {
							return "event.preventDefault();SeamConfirm.showForm(this.form,'" + confirm + "',{dangerName:'" + cfg.DeleteName + "'})"
						}
						return "event.preventDefault();SeamConfirm.showForm(this.form,'" + confirm + "')"
					}()),
					iconSpan("delete", del),
				),
			))
		}

		return b.Td(mi.Class("px-4 py-3"),
			b.Div(btns...),
		)
	}
}

// ─── Filter bar ───────────────────────────────────────────────────────────────

// ListFilterField is a single <select> control in the filter bar.
type ListFilterField struct {
	Name    string
	Options []SelectOption
}

// ListFilterConfig configures ListFilterBar.
type ListFilterConfig struct {
	Action      string // GET form action URL (the listing's base path)
	SearchName  string // query-param name for search input; empty → no search box
	SearchHint  string
	SearchValue string
	Filters     []ListFilterField // zero or more <select> filters
}

// ListFilterBar renders a GET form row with an optional search input and
// zero or more <select> filters that auto-submit on change.
// For listings with more complex filter needs (date ranges, tabs) pass a
// custom mi.H as filterBar to ListPage instead of using this helper.
func ListFilterBar(cfg ListFilterConfig) mi.H {
	return func(b *mi.Builder) mi.Node {
		children := []interface{}{
			mi.Method("get"),
			mi.Action(cfg.Action),
			mi.Class("flex flex-wrap items-center gap-4"),
		}
		if cfg.SearchName != "" {
			children = append(children,
				b.Div(mi.Class("flex-1 max-w-md"),
					SearchInput(cfg.SearchName, cfg.SearchHint, cfg.SearchValue)(b),
				),
			)
		}
		for _, f := range cfg.Filters {
			opts := []interface{}{
				mi.Name(f.Name),
				mi.Class("rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm"),
				mi.Attr("onchange", "this.form.submit()"),
			}
			opts = append(opts, selectOptions(b, f.Options)...)
			children = append(children, b.Select(opts...))
		}
		return b.Form(children...)
	}
}

// ─── Pagination bar ───────────────────────────────────────────────────────────

// ListPaginationConfig configures ListPaginationBar.
type ListPaginationConfig struct {
	Page     int
	PageSize int
	Visible  int // number of items on the current page (len(data.Rows))
	Total    int
	BaseURL  string
	Label    string // plural noun for the count line e.g. "assets", "events" -- caller's own responsibility to pass an already-translated value
}

// ListPaginationBar renders a translated "Showing N of M label" count and the Pagination widget.
func ListPaginationBar(cfg ListPaginationConfig, t func(key string, args ...any) string) mi.H {
	return func(b *mi.Builder) mi.Node {
		totalPages := (cfg.Total + cfg.PageSize - 1) / cfg.PageSize
		countText := t("pagination.showing", cfg.Visible, cfg.Total, cfg.Label)
		return b.Div(mi.Class("flex items-center justify-between"),
			b.Div(mi.Class("text-sm text-gray-500 dark:text-gray-400"), countText),
			Pagination(cfg.Page, totalPages, cfg.BaseURL, t)(b),
		)
	}
}
