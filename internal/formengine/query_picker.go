package formengine

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	mi "github.com/ha1tch/minty"

	"github.com/ha1tch/seam-ui/internal/querypicker"
)

// The "query-picker" widget (docs/QUERY_PICKER.md): choose ONE record from the
// result of an OQL query, shown with several typed columns.
//
// The query never reaches the browser. parseQueryPicker checks it against the
// rules when the schema is parsed, so a query that breaks one makes the schema
// fail to load; the renderer then seals the field's specification into an
// opaque token (querypicker.Seal) that the browser sends back with each search
// and that only the server can read or produce. The markup is inert: every
// behaviour is in SeamWidgets' "query-picker" kind (web/static/js/app.js),
// found relative to the widget's own root, so it works in a repeatable row
// added later and in a form swapped in by htmx.

// QueryPickerMinChars is how many characters the search box needs before the
// widget asks the server.
const QueryPickerMinChars = 2

// parseQueryPicker builds and validates the specification of a query-picker
// field. It returns nil for any other widget.
func parseQueryPicker(input, jsonType, query string, columns []querypicker.Column, valueField string, labelFields []string, limit int, dropdown string) (*querypicker.Spec, error) {
	if input != "query-picker" {
		return nil, nil
	}
	if query == "" {
		return nil, fmt.Errorf("x-seam-query: a query-picker needs a query")
	}
	switch dropdown {
	case "", "table", "rows":
	default:
		return nil, fmt.Errorf("x-seam-dropdown: must be \"table\" or \"rows\", got %q", dropdown)
	}
	spec := &querypicker.Spec{
		Query:        query,
		ValueField:   valueField,
		ValueNumeric: jsonType == "integer" || jsonType == "number",
		LabelFields:  labelFields,
		Columns:      columns,
		Limit:        limit,
	}
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	return spec, nil
}

// QueryPickerFields lists the query-picker fields of a form, including those
// inside repeatables, for the handler that checks submitted values.
func QueryPickerFields(form *Form) []Field {
	if form == nil {
		return nil
	}
	var out []Field
	var walk func(fields []Field)
	walk = func(fields []Field) {
		for _, f := range fields {
			if f.QueryPicker != nil {
				out = append(out, f)
			}
			walk(f.Items)
		}
	}
	walk(form.Fields)
	return out
}

// queryPickerColumn is what the browser needs to draw one column.
type queryPickerColumn struct {
	Title string `json:"t"`
	Type  string `json:"k"`
	Key   bool   `json:"key,omitempty"`
	Muted bool   `json:"muted,omitempty"`
}

// queryPickerHeading resolves a column's heading: the form's own key, the
// shared key, the column's literal title, then its name.
func queryPickerHeading(e *FormEngine, rc RenderContext, f Field, c querypicker.Column) string {
	var keys []string
	if rc.Module != "" && rc.Form != "" {
		keys = append(keys, rc.Module+".forms."+rc.Form+".field."+f.Name+".column."+c.Field)
	}
	keys = append(keys, "query_picker.column."+c.Field)
	if resolved, ok := tryKeys(e.Translator, rc.Locale, keys...); ok {
		return resolved
	}
	if c.Title != "" {
		return c.Title
	}
	return c.Field
}

func renderQueryPicker(ctx context.Context, e *FormEngine, f Field, value interface{}, hasError bool, rc RenderContext) mi.H {
	return func(b *mi.Builder) mi.Node {
		spec := f.QueryPicker
		if spec == nil {
			return b.P(mi.Class("mt-1 text-sm text-red-600"), "formengine: query-picker field has no query")
		}
		// The headings are resolved here, for this request's locale, and sealed
		// with the rest: the Browse dialog, which has no form to ask, then shows
		// the same headings as the dropdown.
		sealed := *spec
		sealed.Columns = make([]querypicker.Column, len(spec.Columns))
		cols := make([]queryPickerColumn, len(spec.Columns))
		for i, c := range spec.Columns {
			c.Title = queryPickerHeading(e, rc, f, c)
			sealed.Columns[i] = c
			cols[i] = queryPickerColumn{Title: c.Title, Type: c.Type, Key: c.Key, Muted: c.Muted}
		}
		token, err := querypicker.Seal(&sealed)
		if err != nil {
			return b.P(mi.Class("mt-1 text-sm text-red-600"), "formengine: "+err.Error())
		}
		colsJSON, _ := json.Marshal(cols)
		layout := f.XSeamDropdown
		if layout == "" {
			layout = "auto"
		}
		chrome := func(key, literal string) string {
			return resolveWidgetChrome(e, rc, "field.query_picker."+key, literal)
		}

		attrs := []interface{}{
			mi.Class("fe-qp space-y-1"),
			mi.Attr("data-seam-widget", "query-picker"),
			mi.Attr("data-qp-token", token),
			mi.Attr("data-qp-cols", string(colsJSON)),
			mi.Attr("data-qp-layout", layout),
			mi.Attr("data-qp-min", strconv.Itoa(QueryPickerMinChars)),
			mi.Attr("data-t-more", chrome("more", "Showing the first %d. More match: keep typing to narrow the list.")),
			mi.Attr("data-t-more-browse", chrome("more_browse", "Showing the first %d. More match: keep typing, or use Browse for the full list.")),
			mi.Attr("data-t-none", chrome("no_results", "No results")),
			mi.Attr("data-t-unavailable", chrome("unavailable", "no longer available")),
			mi.Attr("data-t-failed", chrome("failed", "The search could not be completed")),
			mi.Attr("data-t-keys", chrome("keys", "↑ ↓ move · Enter choose · Esc close")),
			mi.Attr("data-t-select", chrome("select", "Please choose an item from the list")),
		}
		if f.XSeamDetail {
			attrs = append(attrs, mi.Attr("data-qp-detail", "1"))
		}
		searchAttrs := []mi.Attribute{
			mi.Type("text"), mi.ID("qp-search-" + f.Name), mi.Class(inputClass(hasError)),
			mi.Attr("autocomplete", "off"),
			mi.Attr("role", "combobox"),
			mi.Attr("aria-autocomplete", "list"),
			mi.Attr("aria-expanded", "false"),
			mi.Attr("data-hl", highlightMode(f)),
			mi.Attr("placeholder", anyPickerPlaceholder(e, rc, f)),
		}
		if f.Required {
			searchAttrs = append(searchAttrs, mi.Required())
		}
		row := []interface{}{
			mi.Class("fe-picker-row"),
			b.Input(mi.Type("hidden"), mi.Name(f.Name), mi.ID("field-"+f.Name), mi.Value(stringValue(value))),
			b.Input(searchAttrs...),
		}
		var link interface{}
		if f.XSeamSelector != "" {
			label := resolveWidgetChrome(e, rc, "field.any_picker.browse", "Browse…")
			switch browseStyle(f) {
			case "icon":
				row = append(row, b.Button(mi.Type("button"), mi.Class("fe-btn fe-btn--icon"),
					mi.Attr("data-qp-browse", "1"), mi.Attr("data-title", label), mi.Attr("title", label), mi.Attr("aria-label", label),
					b.Span(mi.Class("material-icons"), "manage_search")))
			case "link":
				link = b.Button(mi.Type("button"),
					mi.Class("text-sm text-indigo-600 dark:text-indigo-400 hover:underline"),
					mi.Attr("data-qp-browse", "1"), mi.Attr("data-title", label), label)
			default:
				row = append(row, b.Button(mi.Type("button"), mi.Class("fe-btn"),
					mi.Attr("data-qp-browse", "1"), mi.Attr("data-title", label),
					b.Span(mi.Class("material-icons"), "manage_search"), label))
			}
		}
		attrs = append(attrs, b.Div(row...))
		if link != nil {
			attrs = append(attrs, link)
		}
		attrs = append(attrs, b.P(mi.Class("fe-qp-detail mt-1 text-xs text-gray-500 dark:text-gray-400"), mi.Attr("data-qp-detail-line", "1"), mi.Attr("hidden", "hidden")))
		return b.Div(attrs...)
	}
}
