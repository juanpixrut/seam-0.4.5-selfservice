package nplate

import (
	"encoding/json"
	"testing"
)

func TestRender_DocExample(t *testing.T) {
	plate := []byte(`{
	  "widget": "recent_list",
	  "rows": {"$each": "alerts[*]", "as": "a", "template": {
	    "title": {"$ref": "a.title"},
	    "asset": {"$ref": "a.asset_name"}
	  }}
	}`)
	data := map[string]interface{}{
		"alerts": []interface{}{
			map[string]interface{}{"title": "Temp exceeded", "asset_name": "Chiller 5"},
			map[string]interface{}{"title": "Low battery", "asset_name": "Sensor 12"},
		},
	}
	out, err := Render(plate, data)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	rows, ok := got["rows"].([]interface{})
	if !ok || len(rows) != 2 {
		t.Fatalf("rows = %#v, want a 2-element array", got["rows"])
	}
	first := rows[0].(map[string]interface{})
	if first["title"] != "Temp exceeded" || first["asset"] != "Chiller 5" {
		t.Errorf("first row = %#v", first)
	}
	if got["widget"] != "recent_list" {
		t.Errorf("sibling literal not preserved: widget = %v", got["widget"])
	}
}

func TestRender_DefaultAsBinding(t *testing.T) {
	plate := []byte(`{"names": {"$each": "items[*]", "template": {"n": {"$ref": "item.name"}}}}`)
	data := map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{"name": "one"},
			map[string]interface{}{"name": "two"},
		},
	}
	out, err := Render(plate, data)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	var got map[string]interface{}
	json.Unmarshal(out, &got)
	names := got["names"].([]interface{})
	if len(names) != 2 || names[0].(map[string]interface{})["n"] != "one" {
		t.Errorf("names = %#v, want [{n:one} {n:two}]", got["names"])
	}
}

func TestRender_NestedEach(t *testing.T) {
	plate := []byte(`{
	  "sections": {"$each": "sections[*]", "as": "s", "template": {
	    "id": {"$ref": "s.id"},
	    "widgets": {"$each": "s.widgets[*]", "as": "w", "template": {"label": {"$ref": "w.label"}}}
	  }}
	}`)
	data := map[string]interface{}{
		"sections": []interface{}{
			map[string]interface{}{
				"id": "top",
				"widgets": []interface{}{
					map[string]interface{}{"label": "Open Alerts"},
					map[string]interface{}{"label": "Maintenance"},
				},
			},
		},
	}
	out, err := Render(plate, data)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	var got map[string]interface{}
	json.Unmarshal(out, &got)
	sections := got["sections"].([]interface{})
	if len(sections) != 1 {
		t.Fatalf("sections = %#v, want 1 element", got["sections"])
	}
	widgets := sections[0].(map[string]interface{})["widgets"].([]interface{})
	if len(widgets) != 2 || widgets[0].(map[string]interface{})["label"] != "Open Alerts" {
		t.Errorf("widgets = %#v", widgets)
	}
}

func TestRender_EmptyArray(t *testing.T) {
	plate := []byte(`{"rows": {"$each": "alerts[*]", "template": {"t": {"$ref": "item.title"}}}}`)
	data := map[string]interface{}{"alerts": []interface{}{}}
	out, err := Render(plate, data)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	var got map[string]interface{}
	json.Unmarshal(out, &got)
	rows, ok := got["rows"].([]interface{})
	if !ok || len(rows) != 0 {
		t.Errorf("rows = %#v, want an empty array, not null or missing", got["rows"])
	}
}

func TestRender_RefOutsideEachStillResolves(t *testing.T) {
	plate := []byte(`{
	  "title": {"$ref": "dashboardTitle"},
	  "rows": {"$each": "items[*]", "template": {"n": {"$ref": "item.name"}}}
	}`)
	data := map[string]interface{}{
		"dashboardTitle": "Assets",
		"items":          []interface{}{map[string]interface{}{"name": "x"}},
	}
	out, err := Render(plate, data)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	var got map[string]interface{}
	json.Unmarshal(out, &got)
	if got["title"] != "Assets" {
		t.Errorf("title = %v, want Assets (a $ref sibling to $each must still resolve)", got["title"])
	}
}

func TestValidate_MalformedEach(t *testing.T) {
	tests := []struct {
		name  string
		plate string
	}{
		{"each not a string", `{"x": {"$each": 5, "template": {}}}`},
		{"missing template", `{"x": {"$each": "a[*]"}}`},
		{"as not a string", `{"x": {"$each": "a[*]", "as": 5, "template": {}}}`},
		{"unrecognised key", `{"x": {"$each": "a[*]", "template": {}, "extra": 1}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Validate([]byte(tt.plate)); err == nil {
				t.Errorf("Validate(%s) = nil, want an error", tt.plate)
			}
		})
	}
}

func TestValidate_WellFormed(t *testing.T) {
	plate := `{
	  "title": {"$ref": "x"},
	  "rows": {"$each": "items[*]", "as": "it", "template": {
	    "n": {"$ref": "it.name"},
	    "nested": {"$each": "it.sub[*]", "template": {"s": {"$ref": "item.v"}}}
	  }}
	}`
	if err := Validate([]byte(plate)); err != nil {
		t.Errorf("Validate: %v, want nil for a well-formed document", err)
	}
}

func TestValidate_MalformedRefOutsideEach(t *testing.T) {
	// jsonplate's own $ref shape rule ("$ref" among other keys is
	// malformed) must still apply outside an $each block -- confirms
	// stripEach doesn't accidentally swallow this check.
	plate := `{"x": {"$ref": "a", "other": 1}}`
	if err := Validate([]byte(plate)); err == nil {
		t.Error("Validate = nil, want an error for a malformed $ref sibling outside $each")
	}
}

func TestRender_InvalidJSON(t *testing.T) {
	if _, err := Render([]byte(`{not json`), nil); err == nil {
		t.Error("Render = nil error, want an error for invalid JSON")
	}
}
