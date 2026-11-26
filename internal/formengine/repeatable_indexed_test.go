package formengine

import (
	"context"
	"net/url"
	"reflect"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"
)

// A repeatable "blocks" with a picker and a nested repeatable "lines".
func blocksSchema(t *testing.T) Field {
	t.Helper()
	form, err := ParseSchema([]byte(`{
	  "type": "object",
	  "properties": {
	    "blocks": {"type": "array", "title": "Blocks", "x-seam-input": "repeatable",
	      "items": {"type": "object", "properties": {
	        "name": {"type": "string", "title": "Block name", "x-seam-input": "text"},
	        "responsible_id": {"type": "integer", "title": "Responsible", "x-seam-input": "any-picker",
	                           "x-seam-source": "entity:users", "x-seam-selector": "users"},
	        "lines": {"type": "array", "title": "Lines", "x-seam-input": "repeatable",
	          "items": {"type": "object", "properties": {
	            "name": {"type": "string", "title": "Tag", "x-seam-input": "text"},
	            "type": {"type": "string", "title": "Type", "x-seam-input": "select", "enum": ["Number", "Text"]}}}}}}}
	  }}`))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	return form.Fields[0]
}

func renderRepeatableHTML(f Field, value interface{}) string {
	e := &FormEngine{}
	return mi.RenderToString(renderRepeatable(context.Background(), e, f, value, false, RenderContext{}))
}

func TestNeedsIndexedNaming(t *testing.T) {
	flat := Field{Name: "w", XSeamInput: "repeatable", Items: []Field{{Name: "a", XSeamInput: "text"}}}
	if needsIndexedNaming(flat) {
		t.Errorf("a plain repeatable keeps the flat names")
	}
	forced := flat
	forced.XSeamNaming = "indexed"
	if !needsIndexedNaming(forced) {
		t.Errorf("x-seam-naming: indexed must force indexed names")
	}
	if !needsIndexedNaming(blocksSchema(t)) {
		t.Errorf("a repeatable with a nested repeatable or an any-picker must be indexed")
	}
	withPicker := Field{Name: "w", XSeamInput: "repeatable", Items: []Field{{Name: "p", XSeamInput: "any-picker"}}}
	if !needsIndexedNaming(withPicker) {
		t.Errorf("a repeatable holding an any-picker must be indexed")
	}
}

// Flat repeatables are untouched (the Asset service configurations form).
func TestRepeatable_PlainOneStillUsesFlatNames(t *testing.T) {
	html := renderRepeatableRow(t, nil)
	if strings.Contains(html, "data-rep=") || strings.Contains(html, "<template>") {
		t.Errorf("a plain repeatable must not switch to indexed rendering:\n%s", html)
	}
	if !strings.Contains(html, "service_configs_") {
		t.Errorf("expected the flat {field}_{item}[] names:\n%s", html)
	}
}

func TestRepeatableIndexed_NamesTemplateAndCounters(t *testing.T) {
	html := renderRepeatableHTML(blocksSchema(t), nil)
	for _, want := range []string{
		// first (blank) row, concrete indices
		`name="blocks[0][name]"`, `name="blocks[0][responsible_id]"`,
		`name="blocks[0][lines][0][name]"`, `name="blocks[0][lines][0][type]"`,
		// the template: outer token, nested rows at index 0, nested token
		`name="blocks[__D0__][name]"`, `name="blocks[__D0__][lines][0][name]"`, `name="blocks[__D0__][lines][__D1__][name]"`,
		// containers
		`data-rep="1"`, `data-token="__D0__"`, `data-token="__D1__"`, `data-next="1"`,
		`onclick="SeamRepeatable.add(this)"`, `onclick="SeamRepeatable.remove(this)"`,
		"<template>", "</template>",
		// the picker inside a row
		`data-seam-picker="1"`, `data-selector="users"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %s:\n%s", want, html)
		}
	}
	if strings.Contains(html, "getElementById") {
		t.Errorf("no id lookups may be emitted for rows")
	}
}

func TestRepeatableIndexed_RendersExistingValuesWithTheirIndices(t *testing.T) {
	value := []interface{}{
		map[string]interface{}{"name": "Prep", "lines": []interface{}{
			map[string]interface{}{"name": "t1", "type": "Text"},
			map[string]interface{}{"name": "t2", "type": "Number"},
		}},
		map[string]interface{}{"name": "Run"},
	}
	html := renderRepeatableHTML(blocksSchema(t), value)
	for _, want := range []string{
		`value="Prep"`, `value="t1"`, `value="t2"`, `value="Run"`,
		`name="blocks[1][name]"`, `name="blocks[0][lines][1][name]"`,
		`data-next="2"`, // outer container: two rows
	} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %s:\n%s", want, html)
		}
	}
}

func TestParseIndexedRepeatable_RebuildsNestedRows(t *testing.T) {
	f := blocksSchema(t)
	form := url.Values{
		"blocks[0][name]":                {"Prep"},
		"blocks[0][responsible_id]":      {"7"},
		"blocks[0][lines][0][name]":      {"t1"},
		"blocks[0][lines][0][type]":      {"Number"},
		"blocks[0][lines][1][name]":      {"t2"},
		"blocks[0][lines][1][type]":      {"Text"},
		"blocks[3][name]":                {"Run"},
		"blocks[3][lines][0][name]":      {""}, // blank first field: dropped
		"blocks[3][lines][0][type]":      {"Number"},
		"blocks[5][name]":                {""}, // blank first field: whole row dropped
		"blocks[5][lines][0][name]":      {"orphan"},
		"unrelated":                      {"x"},
		"blocks[bad":                     {"x"},
		"blocks[0][lines][1][name][oops": {"x"},
	}
	got := ParseIndexedRepeatable(form, f)
	want := []interface{}{
		map[string]interface{}{
			"name": "Prep", "responsible_id": "7",
			"lines": []interface{}{
				map[string]interface{}{"name": "t1", "type": "Number"},
				map[string]interface{}{"name": "t2", "type": "Text"},
			},
		},
		map[string]interface{}{"name": "Run", "lines": []interface{}{}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %#v\nwant %#v", got, want)
	}
}

// Rows are ordered by numeric index: 2 before 10.
func TestParseIndexedRepeatable_OrdersByNumericIndex(t *testing.T) {
	f := Field{Name: "r", XSeamInput: "repeatable", XSeamNaming: "indexed",
		Items: []Field{{Name: "a", XSeamInput: "text"}}}
	form := url.Values{"r[10][a]": {"ten"}, "r[2][a]": {"two"}, "r[0][a]": {"zero"}}
	got := ParseIndexedRepeatable(form, f)
	if len(got) != 3 || got[0].(map[string]interface{})["a"] != "zero" ||
		got[1].(map[string]interface{})["a"] != "two" || got[2].(map[string]interface{})["a"] != "ten" {
		t.Errorf("order wrong: %#v", got)
	}
}

func TestParseIndexedRepeatable_NothingPostedIsEmptyNotNil(t *testing.T) {
	got := ParseIndexedRepeatable(url.Values{}, blocksSchema(t))
	if got == nil || len(got) != 0 {
		t.Errorf("want an empty non-nil slice, got %#v", got)
	}
}

// A value that is both a leaf and a branch is malformed and must not panic.
func TestParseIndexedRepeatable_ConflictingPathsDoNotPanic(t *testing.T) {
	f := blocksSchema(t)
	form := url.Values{"blocks[0][name]": {"A"}, "blocks[0][name][x]": {"B"}, "blocks[0]": {"C"}}
	_ = ParseIndexedRepeatable(form, f)
}

func TestParseIndexedForm_OnlyIndexedRepeatablesWithPostedValues(t *testing.T) {
	flat := Field{Name: "w", XSeamInput: "repeatable", Items: []Field{{Name: "a", XSeamInput: "text"}}}
	blocks := blocksSchema(t)
	form := url.Values{"blocks[0][name]": {"A"}, "w_a[]": {"x"}}
	got := ParseIndexedForm(form, []Field{flat, blocks})
	if _, ok := got["w"]; ok {
		t.Errorf("a flat repeatable must not be parsed here")
	}
	if rows, ok := got["blocks"].([]interface{}); !ok || len(rows) != 1 {
		t.Errorf("blocks = %#v", got["blocks"])
	}
	if len(ParseIndexedForm(url.Values{"w_a[]": {"x"}}, []Field{blocks})) != 0 {
		t.Errorf("nothing posted for blocks: no entry expected")
	}
}

// What the parser returns is what the renderer takes back as a value.
func TestRepeatableIndexed_ParseThenRenderRoundTrip(t *testing.T) {
	f := blocksSchema(t)
	form := url.Values{"blocks[0][name]": {"Prep"}, "blocks[0][lines][0][name]": {"t1"}, "blocks[0][lines][0][type]": {"Text"}}
	html := renderRepeatableHTML(f, ParseIndexedRepeatable(form, f))
	for _, want := range []string{`value="Prep"`, `value="t1"`, `name="blocks[0][lines][0][name]"`} {
		if !strings.Contains(html, want) {
			t.Errorf("round trip lost %s:\n%s", want, html)
		}
	}
}
