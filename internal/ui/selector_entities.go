package ui

import (
	"context"
	"errors"
	"strconv"
	"strings"

	mi "github.com/ha1tch/minty"
)

// =============================================================================
// SELECTOR SOURCES BACKED BY XOLU ENTITIES
// =============================================================================
// The Users source goes through auth.UserService. The sources here are plain
// xolu entities of the asset-management data (assets, cost centres,
// locations, custodians), so they share one builder: a spec names the table,
// the columns the search text is matched against and how a row is shown.
//
// Same trust boundary as the picker (asset_picker_search.go): the table and
// column names are fixed in this file, never taken from the request; only the
// search text is client input, and it goes through escapeOQL. LIKE is
// case-sensitive in OQL. Paging fetches TOP (page*size+1) rows and slices, as
// OQL has no OFFSET.
//
// Visibility is the same as /picker/search: every signed-in user can read
// these lists (the caller's tenant is applied by the xolu client).

var errSelectorNoXolu = errors.New("selector: xolu client not configured")

type xoluSelectorSpec struct {
	Key           string
	TitleKey      string
	EmptyIcon     string
	Table         string
	SearchColumns []string
	OrderColumn   string
	Label         func(row map[string]any) string
	Columns       []SelectorColumn
}

func (s xoluSelectorSpec) source() SelectorSource {
	return SelectorSource{
		Key:             s.Key,
		TitleKey:        s.TitleKey,
		EmptyIcon:       s.EmptyIcon,
		EmptyTitleKey:   "selector.empty_title",
		EmptyMessageKey: "selector.empty_message",
		PageSize:        selectorDefaultPageSize,
		Columns:         s.Columns,
		Fetch:           s.fetch,
	}
}

// query builds the OQL for one page: TOP (page*pageSize+1) rows, filtered by q
// on the spec's search columns (and on id when q is a number), ordered. OQL
// has no OFFSET, so selectorPage slices the page out of the result.
func (s xoluSelectorSpec) query(q string, page, pageSize int) string {
	if page < 1 {
		page = 1
	}
	query := "SELECT TOP " + strconv.Itoa(page*pageSize+1) + " * FROM " + s.Table
	if q != "" {
		escaped := escapeOQL(q)
		conds := make([]string, 0, len(s.SearchColumns)+1)
		for _, col := range s.SearchColumns {
			conds = append(conds, col+" LIKE '%"+escaped+"%'")
		}
		if id, err := strconv.ParseInt(q, 10, 64); err == nil {
			conds = append(conds, "id = "+strconv.FormatInt(id, 10))
		}
		query += " WHERE (" + strings.Join(conds, " OR ") + ")"
	}
	return query + " ORDER BY " + s.OrderColumn
}

// selectorPage returns the rows of the 1-based page out of data (which holds
// everything up to one row past that page) and whether another page follows.
func selectorPage(data []map[string]any, page, pageSize int) (rows []map[string]any, hasMore bool) {
	if page < 1 {
		page = 1
	}
	start := (page - 1) * pageSize
	if start > len(data) {
		start = len(data)
	}
	end := start + pageSize
	if end > len(data) {
		end = len(data)
	}
	return data[start:end], len(data) > page*pageSize
}

func (s xoluSelectorSpec) fetch(ctx context.Context, h *UIHandler, q string, page, pageSize int) ([]SelectorRow, bool, error) {
	if h.xoluClient == nil {
		return nil, false, errSelectorNoXolu
	}
	result, err := h.xoluClient.OQL(ctx, s.query(q, page, pageSize))
	if err != nil {
		if entityNotYetCreated(err) {
			return []SelectorRow{}, false, nil
		}
		return nil, false, err
	}
	pageData, hasMore := selectorPage(result.Data, page, pageSize)
	rows := make([]SelectorRow, 0, len(pageData))
	for _, row := range pageData {
		rows = append(rows, SelectorRow{ID: toInt64(row["id"]), Label: s.Label(row), Data: row})
	}
	return rows, hasMore, nil
}

// selectorTwoLineCell is a cell with a main line and a muted second line.
func selectorTwoLineCell(primary, secondary string) mi.H {
	return ListCellRich(func(b *mi.Builder) mi.Node {
		if secondary == "" {
			return b.Div(mi.Class("font-medium text-gray-900 dark:text-white text-sm"), primary)
		}
		return b.Div(
			b.Div(mi.Class("font-medium text-gray-900 dark:text-white text-sm"), primary),
			b.Div(mi.Class("text-xs text-gray-500 dark:text-gray-400"), secondary),
		)
	})
}

// selectorTextCell is a plain text cell reading one field of the row.
func selectorTextCell(field string) func(r SelectorRow, t func(key string, args ...any) string) mi.H {
	return func(r SelectorRow, t func(key string, args ...any) string) mi.H {
		text := toString(r.Data[field])
		return ListCellRich(func(b *mi.Builder) mi.Node {
			return b.Span(mi.Class("text-sm text-gray-600 dark:text-gray-300"), text)
		})
	}
}

// Labels shown once a row is chosen (and by the picker's live search).

func costCenterLabel(row map[string]any) string {
	code, name := toString(row["accounting_code"]), toString(row["name"])
	if code == "" {
		return name
	}
	if name == "" {
		return code
	}
	return code + " — " + name
}

func locationLabel(row map[string]any) string {
	short, desc := toString(row["short_name"]), toString(row["description"])
	if desc == "" {
		return short
	}
	if short == "" {
		return desc
	}
	return short + " — " + desc
}

func custodianLabel(row map[string]any) string {
	legal, alias := toString(row["legal_name"]), toString(row["alias"])
	if alias == "" || alias == legal {
		return legal
	}
	if legal == "" {
		return alias
	}
	return legal + " (" + alias + ")"
}

func assetSelectorLabel(row map[string]any) string {
	name, status := toString(row["name"]), toString(row["status"])
	if status == "" {
		return name
	}
	return name + " — " + status
}

func assetTypeSelectorLabel(row map[string]any) string {
	name := toString(row["name"])
	return name
}
func woActionSelectorLabel(row map[string]any) string {
	name := toString(row["name"])
	return name
}

func warehouseSelectorLabel(row map[string]any) string {
	name := toString(row["name"])
	return name
}

func unitSelectorLabel(row map[string]any) string {
	name := toString(row["name"])
	return name
}

func consumableSelectorLabel(row map[string]any) string {
	name := toString(row["name"])
	return name
}

func woTemplateSelectorLabel(row map[string]any) string {
	name := toString(row["name"])
	version := strconv.Itoa(toInt(row["version"]))
	return name + " (v" + version + ".0)"
}

var assetsSelectorSource = xoluSelectorSpec{
	Key: "assets", TitleKey: "selector.title.assets", EmptyIcon: "inventory_2",
	Table: "assets", SearchColumns: []string{"name", "tag_tecnico", "serial_1"}, OrderColumn: "name",
	Label: assetSelectorLabel,
	Columns: []SelectorColumn{
		{LabelKey: "selector.col.asset", Cell: func(r SelectorRow, t func(key string, args ...any) string) mi.H {
			return selectorTwoLineCell(toString(r.Data["name"]), toString(r.Data["tag_tecnico"]))
		}},
		{LabelKey: "field.status", Cell: selectorTextCell("status")},
		{LabelKey: "selector.col.criticality", Cell: selectorTextCell("criticality")},
	},
}.source()

var costCentersSelectorSource = xoluSelectorSpec{
	Key: "cost_centers", TitleKey: "selector.title.cost_centers", EmptyIcon: "account_balance_wallet",
	Table: "cost_centers", SearchColumns: []string{"name", "accounting_code"}, OrderColumn: "accounting_code",
	Label: costCenterLabel,
	Columns: []SelectorColumn{
		{LabelKey: "field.code", Cell: selectorTextCell("accounting_code")},
		{LabelKey: "field.name", Cell: selectorTextCell("name")},
	},
}.source()

var locationsSelectorSource = xoluSelectorSpec{
	Key: "locations", TitleKey: "selector.title.locations", EmptyIcon: "place",
	Table: "locations", SearchColumns: []string{"short_name", "description"}, OrderColumn: "short_name",
	Label: locationLabel,
	Columns: []SelectorColumn{
		{LabelKey: "selector.col.location", Cell: func(r SelectorRow, t func(key string, args ...any) string) mi.H {
			return selectorTwoLineCell(toString(r.Data["short_name"]), toString(r.Data["description"]))
		}},
	},
}.source()

var custodiansSelectorSource = xoluSelectorSpec{
	Key: "custodians", TitleKey: "selector.title.custodians", EmptyIcon: "badge",
	Table: "custodians", SearchColumns: []string{"legal_name", "alias", "tax_id"}, OrderColumn: "legal_name",
	Label: custodianLabel,
	Columns: []SelectorColumn{
		{LabelKey: "selector.col.custodian", Cell: func(r SelectorRow, t func(key string, args ...any) string) mi.H {
			return selectorTwoLineCell(toString(r.Data["legal_name"]), toString(r.Data["alias"]))
		}},
		{LabelKey: "selector.col.tax_id", Cell: selectorTextCell("tax_id")},
		{LabelKey: "field.email", Cell: selectorTextCell("email")},
	},
}.source()
