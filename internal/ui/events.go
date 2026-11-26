package ui

import (
	"fmt"

	mi "github.com/ha1tch/minty"
)

// EventsListData holds data for the events list page.
type EventsListData struct {
	Events       []EventRow
	Total        int
	Page         int
	PageSize     int
	AssetFilter  string
	TypeFilter   string
	FromDate     string
	ToDate       string
	TriggerTypes []string
}

// EventRow represents an event in the list.
type EventRow struct {
	ID          int64
	AssetID     int64
	AssetName   string
	TriggerType string
	Value       string
	Timestamp   string
	Processed   bool
}

// EventsListPage renders the events list page.
func EventsListPage(data EventsListData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	rows := make([]mi.H, len(data.Events))
	for i, evt := range data.Events {
		rows[i] = ListRow(i,
			ListCellMuted(evt.Timestamp),
			ListCellRich(func(b *mi.Builder) mi.Node {
				return b.A(mi.Href(fmt.Sprintf("/assets/%d", evt.AssetID)),
					mi.Class("text-sm text-indigo-600 hover:text-indigo-500 dark:text-indigo-400"),
					evt.AssetName,
				)
			}),
			ListCellRich(func(b *mi.Builder) mi.Node {
				return Badge(evt.TriggerType, "info")(b)
			}),
			ListCellMuted(truncateValue(evt.Value, 40)),
			ListCellRich(func(b *mi.Builder) mi.Node {
				if evt.Processed {
					return Badge(t("events.status.processed"), "success")(b)
				}
				return Badge(t("status.pending"), "warning")(b)
			}),
		)
	}
	return ListPage(
		ListPageConfig{
			Title:          t("events.list_title"),
			SecondaryURL:   "/api/v1/events/export?format=csv",
			SecondaryLabel: t("events.export_csv"),
			SecondaryIcon:  "file_download",
		},
		eventsFilterBar(data, t),
		ListTable(ListTableConfig{
			Columns: []ListColumn{
				{Label: t("events.column.timestamp")}, {Label: t("events.column.asset")}, {Label: t("field.type")},
				{Label: t("events.column.value")}, {Label: t("field.status")},
			},
			EmptyIcon:    "timeline",
			EmptyTitle:   t("events.empty_title"),
			EmptyMessage: t("events.empty_message"),
		}, rows),
		ListPaginationBar(ListPaginationConfig{
			Page: data.Page, PageSize: data.PageSize,
			Visible: len(data.Events), Total: data.Total,
			BaseURL: "/events", Label: t("nav.events"),
		}, t),
	)
}

func eventsFilterBar(data EventsListData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	return func(b *mi.Builder) mi.Node {
		// Trigger type options
		typeOptions := []SelectOption{{Value: "", Label: t("sensors.filter.all_types")}}
		for _, ty := range data.TriggerTypes {
			typeOptions = append(typeOptions, SelectOption{
				Value:    ty,
				Label:    ty,
				Selected: data.TypeFilter == ty,
			})
		}

		return b.Form(mi.Method("get"), mi.Action("/events"), mi.Class("flex flex-wrap items-center gap-4"),
			// Asset filter
			b.Div(
				b.Label(mi.Class("block text-xs text-gray-500 dark:text-gray-400 mb-1"), t("events.field.asset_id")),
				b.Input(
					mi.Type("text"),
					mi.Name("asset_id"),
					mi.Value(data.AssetFilter),
					mi.Placeholder(t("events.filter.asset_placeholder")),
					mi.Class("rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm w-32"),
				),
			),

			// Trigger type filter
			b.Div(
				b.Label(mi.Class("block text-xs text-gray-500 dark:text-gray-400 mb-1"), t("field.type")),
				b.Select(append([]interface{}{
					mi.Name("type"),
					mi.Class("rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm"),
				}, selectOptions(b, typeOptions)...)...),
			),

			// Date range
			b.Div(
				b.Label(mi.Class("block text-xs text-gray-500 dark:text-gray-400 mb-1"), t("events.field.from")),
				b.Input(
					mi.Type("date"),
					mi.Name("from"),
					mi.Value(data.FromDate),
					mi.Class("rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm"),
				),
			),

			b.Div(
				b.Label(mi.Class("block text-xs text-gray-500 dark:text-gray-400 mb-1"), t("events.field.to")),
				b.Input(
					mi.Type("date"),
					mi.Name("to"),
					mi.Value(data.ToDate),
					mi.Class("rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm"),
				),
			),

			// Submit button
			b.Div(mi.Class("self-end"),
				Button(t("action.filter"), "primary", mi.Type("submit"))(b),
			),
		)
	}
}

func truncateValue(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
