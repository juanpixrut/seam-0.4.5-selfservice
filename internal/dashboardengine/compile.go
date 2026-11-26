package dashboardengine

import (
	"encoding/json"
	"fmt"

	"github.com/ha1tch/seam-ui/pkg/nplate"
)

// This file is the "compile" phase between resolving a widget's source
// (source.go/EntityResolver) and rendering it (widgets.go): resolved data
// -- Go-typed (Row, GroupCountRow, UrgencyRow) or a bare int for count
// sources -- is converted to a plain data context and rendered through a
// fixed, per-widget-type nplate template into a canonical compiled JSON
// shape. widgets.go consumes only that compiled shape, never the
// resolver's own typed results directly.
//
// Two reasons this exists as its own phase rather than widgets.go reading
// resolved data straight from EntityResolver, as it did before this
// rework:
//
//  1. It's the natural cache boundary (cache.go): a compiled widget is
//     pure data, safe to store and reuse for a few seconds without
//     re-touching xolu, which the raw EntityResolver call cannot be
//     (Rows/UrgencyRows do real, sometimes multi-query, work every call).
//  2. It's genuinely declarative rather than hand-written per widget type,
//     using github.com/ha1tch/seam-ui/pkg/nplate -- iteration (recent_list,
//     urgency_list, breakdown_list all resolve to N rows) plus jsonplate's
//     existing $ref substitution, not Go loops re-implemented per widget.
//
// The five templates below are fixed per widget *type*, not configurable
// per widget *instance* -- a widget's compiled shape is entirely
// determined by which of the five types it is, never something a
// dashboard author needs to customise in the JSON schema. They are not
// part of DASHBOARD_SPEC.md's own JSON format for that reason: this is an
// internal implementation detail of how MOD-2 renders a widget, not a new
// authoring surface.

var compileTemplates = map[string][]byte{
	"stat_card":      []byte(`{"count": {"$ref": "count"}}`),
	"total_card":     []byte(`{"total": {"$ref": "total"}}`),
	"breakdown_list": []byte(`{"rows": {"$each": "rows[*]", "template": {"$ref": "item"}}}`),
	"recent_list":    []byte(`{"rows": {"$each": "rows[*]", "template": {"$ref": "item"}}}`),
	"urgency_list":   []byte(`{"rows": {"$each": "rows[*]", "template": {"$ref": "item"}}}`),
}

// compileScalar compiles a stat_card/total_card's resolved count.
func compileScalar(widgetType string, count int) (map[string]any, error) {
	return runCompile(widgetType, map[string]any{"count": count, "total": count})
}

// compileRows compiles a recent_list/breakdown_list/urgency_list's
// resolved rows. rows must already be []map[string]any -- callers convert
// their own typed results (GroupCountRow, UrgencyRow) via
// groupCountRowsToMaps/urgencyRowsToMaps first, same shape Row already is.
func compileRows(widgetType string, rows []map[string]any) (map[string]any, error) {
	data := make([]any, len(rows))
	for i, r := range rows {
		data[i] = r
	}
	return runCompile(widgetType, map[string]any{"rows": data})
}

func runCompile(widgetType string, data map[string]any) (map[string]any, error) {
	tmpl, ok := compileTemplates[widgetType]
	if !ok {
		return nil, fmt.Errorf("dashboardengine: no compile template for widget type %q", widgetType)
	}
	out, err := nplate.Render(tmpl, data)
	if err != nil {
		return nil, fmt.Errorf("dashboardengine: compile %s: %w", widgetType, err)
	}
	var compiled map[string]any
	if err := json.Unmarshal(out, &compiled); err != nil {
		return nil, fmt.Errorf("dashboardengine: unmarshal compiled %s: %w", widgetType, err)
	}
	return compiled, nil
}

// -- typed-result to generic-map adapters --
//
// queryfy's struct-field access (which nplate's $each drives) uses
// reflect.FieldByName -- exact, case-sensitive Go field names, no JSON
// tag awareness. Rather than rely on that inside a template (fragile,
// couples the template's field names to Go's own naming), every typed
// resolver result is converted to a plain map[string]any with the same
// lowercase field names Row (already a map) uses, before it ever reaches
// nplate. This is the only place that conversion happens.

func groupCountRowsToMaps(rows []GroupCountRow) []map[string]any {
	out := make([]map[string]any, len(rows))
	for i, r := range rows {
		out[i] = map[string]any{"value": r.Value, "count": r.Count}
	}
	return out
}

func urgencyRowsToMaps(rows []UrgencyRow) []map[string]any {
	out := make([]map[string]any, len(rows))
	for i, r := range rows {
		out[i] = map[string]any{
			"entity_id":   r.EntityID,
			"entity_name": r.EntityName,
			"field_name":  r.FieldName,
			"date":        r.Date,
			"days_left":   r.DaysLeft,
		}
	}
	return out
}

func rowsToMaps(rows []Row) []map[string]any {
	out := make([]map[string]any, len(rows))
	for i, r := range rows {
		out[i] = map[string]any(r)
	}
	return out
}
