package formengine

import (
	"context"
	"net/url"
	"reflect"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"
)

// reorderField parses a one-repeatable schema; extra is spliced into the
// repeatable's own properties (e.g. `"x-seam-reorder": true,`).
func reorderField(t *testing.T, extra string) Field {
	t.Helper()
	form, err := ParseSchema([]byte(`{"type":"object","properties":{
	  "steps": {"type":"array","title":"Steps","x-seam-input":"repeatable",` + extra + `
	    "items":{"type":"object","properties":{
	      "step":{"type":"string","title":"Step","x-seam-input":"text"},
	      "status":{"type":"string","title":"Status","x-seam-input":"select","enum":["Open","Done"]}}}}}}`))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	return form.Fields[0]
}

func TestReorder_SchemaPropertyDefaultsOff(t *testing.T) {
	if reorderField(t, "").XSeamReorder {
		t.Errorf("x-seam-reorder must default to false")
	}
	if !reorderField(t, `"x-seam-reorder": true,`).XSeamReorder {
		t.Errorf("x-seam-reorder: true must be read")
	}
}

func TestReorder_FlatOffChangesNothing(t *testing.T) {
	html := renderRepeatableHTML(reorderField(t, ""), nil)
	for _, s := range []string{"fe-drag", "data-reorder", "_pos"} {
		if strings.Contains(html, s) {
			t.Errorf("a repeatable without x-seam-reorder must not output %q:\n%s", s, html)
		}
	}
}

func TestReorder_FlatOnHandlePerRowAndMarker(t *testing.T) {
	f := reorderField(t, `"x-seam-reorder": true,`)
	html := renderRepeatableHTML(f, []interface{}{map[string]interface{}{"step": "a"}, map[string]interface{}{"step": "b"}, map[string]interface{}{"step": "c"}})
	if n := strings.Count(html, `class="fe-drag"`); n != 3 {
		t.Errorf("want one handle per row (3), got %d:\n%s", n, html)
	}
	if strings.Count(html, `data-reorder="1"`) != 1 {
		t.Errorf("want exactly one data-reorder marker on the rows container:\n%s", html)
	}
	if strings.Contains(html, "_pos") {
		t.Errorf("flat names post in DOM order and need no position input:\n%s", html)
	}
	if !strings.Contains(html, `type="button"`) || !strings.Contains(html, "drag_indicator") {
		t.Errorf("handle must be a button with the grip icon:\n%s", html)
	}
}

func TestReorder_IndexedOnHandleMarkerAndPosInputs(t *testing.T) {
	f := reorderField(t, `"x-seam-reorder": true, "x-seam-naming": "indexed",`)
	html := renderRepeatableHTML(f, []interface{}{map[string]interface{}{"step": "a"}, map[string]interface{}{"step": "b"}})
	if strings.Count(html, `data-reorder="1"`) != 1 {
		t.Errorf("want one data-reorder marker:\n%s", html)
	}
	for _, name := range []string{`name="steps[0][_pos]"`, `name="steps[1][_pos]"`, `name="steps[__D0__][_pos]"`} {
		if !strings.Contains(html, name) {
			t.Errorf("missing position input %s (rows and the template):\n%s", name, html)
		}
	}
	// 2 rows + the template row each carry a handle
	if n := strings.Count(html, `class="fe-drag"`); n != 3 {
		t.Errorf("want 3 handles (2 rows + template), got %d", n)
	}
}

func TestReorder_IndexedOffHasNoPositionInputs(t *testing.T) {
	f := reorderField(t, `"x-seam-naming": "indexed",`)
	html := renderRepeatableHTML(f, []interface{}{map[string]interface{}{"step": "a"}})
	for _, s := range []string{"fe-drag", "data-reorder", "_pos"} {
		if strings.Contains(html, s) {
			t.Errorf("indexed without x-seam-reorder must not output %q", s)
		}
	}
}

func TestReorder_FormLayoutUsesBarIcon(t *testing.T) {
	f := reorderField(t, `"x-seam-reorder": true, "x-seam-layout": "form",`)
	html := renderRepeatableHTML(f, nil)
	if !strings.Contains(html, "drag_handle") || strings.Contains(html, "drag_indicator") {
		t.Errorf("form layout must use the bar grip:\n%s", html)
	}
}

func TestReorder_ParseOrdersByPositionAndStripsIt(t *testing.T) {
	f := reorderField(t, `"x-seam-reorder": true, "x-seam-naming": "indexed",`)
	form := url.Values{}
	// DOM order was 0,1,2; the user moved the third row to the top.
	form.Set("steps[0][step]", "first")
	form.Set("steps[0][_pos]", "1")
	form.Set("steps[1][step]", "second")
	form.Set("steps[1][_pos]", "2")
	form.Set("steps[2][step]", "third")
	form.Set("steps[2][_pos]", "0")
	got := ParseIndexedRepeatable(form, f)
	want := []interface{}{
		map[string]interface{}{"step": "third"},
		map[string]interface{}{"step": "first"},
		map[string]interface{}{"step": "second"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v\nwant %#v", got, want)
	}
}

func TestReorder_ParseNumericPositionsNotLexical(t *testing.T) {
	f := reorderField(t, `"x-seam-reorder": true, "x-seam-naming": "indexed",`)
	form := url.Values{}
	// positions 10 and 2: numeric order puts 2 first
	form.Set("steps[0][step]", "ten")
	form.Set("steps[0][_pos]", "10")
	form.Set("steps[1][step]", "two")
	form.Set("steps[1][_pos]", "2")
	got := ParseIndexedRepeatable(form, f)
	if len(got) != 2 || got[0].(map[string]interface{})["step"] != "two" {
		t.Errorf("positions must sort numerically: %#v", got)
	}
}

func TestReorder_ParseFallsBackToIndexWhenAPositionIsMissing(t *testing.T) {
	f := reorderField(t, `"x-seam-reorder": true, "x-seam-naming": "indexed",`)
	form := url.Values{}
	form.Set("steps[0][step]", "a")
	form.Set("steps[0][_pos]", "1")
	form.Set("steps[1][step]", "b") // no _pos on this row
	form.Set("steps[2][step]", "c")
	form.Set("steps[2][_pos]", "0")
	got := ParseIndexedRepeatable(form, f)
	order := ""
	for _, r := range got {
		order += r.(map[string]interface{})["step"].(string)
	}
	if order != "abc" {
		t.Errorf("one row without a position means index order, got %q", order)
	}
}

func TestReorder_ParseTiesBreakByIndex(t *testing.T) {
	f := reorderField(t, `"x-seam-reorder": true, "x-seam-naming": "indexed",`)
	form := url.Values{}
	form.Set("steps[3][step]", "later")
	form.Set("steps[3][_pos]", "0")
	form.Set("steps[1][step]", "earlier")
	form.Set("steps[1][_pos]", "0")
	got := ParseIndexedRepeatable(form, f)
	if got[0].(map[string]interface{})["step"] != "earlier" {
		t.Errorf("equal positions must keep index order: %#v", got)
	}
}

func TestReorder_ParseWithoutPositionsIsUnchanged(t *testing.T) {
	f := reorderField(t, `"x-seam-naming": "indexed",`)
	form := url.Values{}
	form.Set("steps[10][step]", "ten")
	form.Set("steps[2][step]", "two")
	got := ParseIndexedRepeatable(form, f)
	if got[0].(map[string]interface{})["step"] != "two" {
		t.Errorf("without positions rows still go by numeric index: %#v", got)
	}
}

func TestReorder_ParseNestedRepeatablesReorderIndependently(t *testing.T) {
	form, err := ParseSchema([]byte(`{"type":"object","properties":{
	  "stages":{"type":"array","title":"Stages","x-seam-input":"repeatable","x-seam-reorder":true,
	    "items":{"type":"object","properties":{
	      "name":{"type":"string","title":"Name","x-seam-input":"text"},
	      "tasks":{"type":"array","title":"Tasks","x-seam-input":"repeatable","x-seam-reorder":true,
	        "items":{"type":"object","properties":{"task":{"type":"string","title":"Task","x-seam-input":"text"}}}}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	f := form.Fields[0]
	v := url.Values{}
	v.Set("stages[0][name]", "S0")
	v.Set("stages[0][_pos]", "1")
	v.Set("stages[0][tasks][0][task]", "a")
	v.Set("stages[0][tasks][0][_pos]", "1")
	v.Set("stages[0][tasks][1][task]", "b")
	v.Set("stages[0][tasks][1][_pos]", "0")
	v.Set("stages[1][name]", "S1")
	v.Set("stages[1][_pos]", "0")
	v.Set("stages[1][tasks][0][task]", "c")
	v.Set("stages[1][tasks][0][_pos]", "0")
	got := ParseIndexedRepeatable(v, f)
	want := []interface{}{
		map[string]interface{}{"name": "S1", "tasks": []interface{}{map[string]interface{}{"task": "c"}}},
		map[string]interface{}{"name": "S0", "tasks": []interface{}{
			map[string]interface{}{"task": "b"}, map[string]interface{}{"task": "a"}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v\nwant %#v", got, want)
	}
	// the nested field also renders its own marker and handles
	html := renderRepeatableHTML(f, nil)
	if strings.Count(html, `data-reorder="1"`) < 2 {
		t.Errorf("outer and nested containers must both be marked:\n%s", html)
	}
}

func TestSelect_EnhancedByDefaultButNotWithExtraAttrs(t *testing.T) {
	f := Field{Name: "unit", Title: "Unit", JSONType: "string", XSeamInput: "select", Enum: []string{"a", "b"}}
	e := &FormEngine{}
	plain := mi.RenderToString(renderSelect(context.Background(), e, f, "", false, RenderContext{}))
	for _, s := range []string{`data-seam-select="1"`, `data-ss-search="`, `data-ss-empty="`} {
		if !strings.Contains(plain, s) {
			t.Errorf("plain select must carry %s:\n%s", s, plain)
		}
	}
	extra := mi.RenderToString(renderSelectWithExtraAttrs(context.Background(), e, f, "", false, RenderContext{},
		[]mi.Attribute{mi.Attr("data-x", "1")}))
	if strings.Contains(extra, "data-seam-select") {
		t.Errorf("a select given extra attributes (asset-type-select) stays native:\n%s", extra)
	}
}
