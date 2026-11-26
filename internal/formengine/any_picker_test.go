package formengine

import (
	"context"
	"regexp"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"
)

// T-97: the "any-picker" widget. Markup is inert and carries data attributes;
// one shared script (SeamPicker, web/static/js/app.js) initialises it, so it
// works inside repeatable rows. An optional Browse button (x-seam-selector)
// opens the Selector dialog.

func anyPickerField(selector string) Field {
	return Field{
		Name: "owner_id", Title: "Owner", JSONType: "integer",
		XSeamInput: "any-picker", XSeamSource: "entity:users", XSeamSelector: selector,
	}
}

func renderWidgetHTML(r widgetRenderer, f Field, value interface{}) string {
	e := &FormEngine{}
	return mi.RenderToString(r(context.Background(), e, f, value, false, RenderContext{}))
}

func TestAnyPicker_RegisteredAlongsideAssetPicker(t *testing.T) {
	if widgetRegistry["any-picker"] == nil {
		t.Errorf("any-picker is not registered")
	}
	if widgetRegistry["asset-picker"] == nil {
		t.Errorf("asset-picker must stay registered")
	}
}

func TestParseSchema_ReadsXSeamSelectorAndNaming(t *testing.T) {
	form, err := ParseSchema([]byte(`{
		"type": "object",
		"properties": {
			"owner_id": {"type": "integer", "title": "Owner", "x-seam-input": "any-picker",
				"x-seam-source": "entity:users", "x-seam-selector": "users"},
			"parent_id": {"type": "integer", "x-seam-input": "any-picker", "x-seam-source": "entity:assets"},
			"rows": {"type": "array", "x-seam-input": "repeatable", "x-seam-naming": "indexed",
				"items": {"type": "object", "properties": {"a": {"type": "string"}}}}
		}
	}`))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	sel, naming := map[string]string{}, map[string]string{}
	for _, f := range form.Fields {
		sel[f.Name], naming[f.Name] = f.XSeamSelector, f.XSeamNaming
	}
	if sel["owner_id"] != "users" || sel["parent_id"] != "" || naming["rows"] != "indexed" {
		t.Errorf("selector = %v, naming = %v", sel, naming)
	}
}

func TestAnyPicker_RendersInertMarkupForTheSharedScript(t *testing.T) {
	html := renderWidgetHTML(renderAnyPicker, anyPickerField(""), float64(7))
	for _, want := range []string{
		`data-seam-picker="1"`, `data-entity="entity:users"`,
		`type="hidden"`, `name="owner_id"`, `value="7"`, `id="field-owner_id"`,
		`type="text"`, `id="picker-search-owner_id"`,
		"autocomplete@10.2.10.min.js", "SeamPicker.init(document)",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %s:\n%s", want, html)
		}
	}
	if strings.Contains(html, "data-selector=") || strings.Contains(html, "SeamPicker.browse") {
		t.Errorf("no Browse button without x-seam-selector")
	}
	// Nothing per-widget is looked up by id any more.
	if strings.Contains(html, "getElementById") || strings.Contains(html, "new autoComplete(") {
		t.Errorf("the widget must not carry its own per-instance script logic:\n%s", html)
	}
}

func TestAnyPicker_WithSelectorAddsBrowseButton(t *testing.T) {
	html := renderWidgetHTML(renderAnyPicker, anyPickerField("users"), nil)
	for _, want := range []string{`data-selector="users"`, `onclick="SeamPicker.browse(this)"`, `data-title="Browse`} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %s:\n%s", want, html)
		}
	}
}

// Schema-authored strings only ever land in escaped attributes.
func TestAnyPicker_SelectorKeyAndSourceCannotBreakOut(t *testing.T) {
	f := anyPickerField(`x" onmouseover="alert(1)`)
	f.XSeamSource = `entity:users" onfocus="alert(2)`
	html := renderWidgetHTML(renderAnyPicker, f, nil)
	if strings.Contains(html, `onmouseover="alert`) || strings.Contains(html, `onfocus="alert`) {
		t.Errorf("a schema string broke out of its attribute:\n%s", html)
	}
}

// A current value is never interpolated into script (T-63/T-64 lesson).
func TestAnyPicker_ExistingValueNeverInterpolatedIntoScript(t *testing.T) {
	malicious := `42"><script>alert(document.cookie)</script>`
	html := renderWidgetHTML(renderAnyPicker, anyPickerField("users"), malicious)
	if strings.Contains(html, "<script>alert(document.cookie)</script>") {
		t.Fatalf("a malicious value reached the output unescaped:\n%s", html)
	}
}

func TestAnyPicker_TwoInstancesOnOnePage_NoIDCollision(t *testing.T) {
	f1 := anyPickerField("users")
	f2 := anyPickerField("users")
	f2.Name = "reviewer_id"
	e := &FormEngine{}
	h := func(b *mi.Builder) mi.Node {
		return b.Div(
			renderFieldRow(b, context.Background(), e, f1, nil, "", false, RenderContext{}),
			renderFieldRow(b, context.Background(), e, f2, nil, "", false, RenderContext{}),
		)
	}
	html := mi.RenderToString(h)
	seen := map[string]bool{}
	for _, m := range regexp.MustCompile(`id="([^"]+)"`).FindAllStringSubmatch(html, -1) {
		if seen[m[1]] {
			t.Errorf("duplicate id=%q", m[1])
		}
		seen[m[1]] = true
	}
	if strings.Count(html, `data-seam-picker="1"`) != 2 {
		t.Errorf("expected two pickers")
	}
}
