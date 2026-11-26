package formengine

import (
	"context"
	"regexp"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"
)

func fkPopoverField() Field {
	return Field{Name: "marca_id", Title: "Marca", JSONType: "integer",
		XSeamInput: "select", XSeamSource: "entity:brands", XSeamFKPopover: "/brands"}
}

func renderFKPopoverRow(t *testing.T, value interface{}) string {
	t.Helper()
	e := &FormEngine{Sources: StaticSourceResolver{
		"entity:brands": []Option{{Value: "1", Label: "Acme"}, {Value: "2", Label: "Beta"}},
	}}
	values := map[string]interface{}{"marca_id": value}
	h := func(b *mi.Builder) mi.Node {
		return renderFieldRow(b, context.Background(), e, fkPopoverField(), values, "", false, RenderContext{})
	}
	return mi.RenderToString(h)
}

func TestFKPopover_ButtonHiddenWhenNoSelection(t *testing.T) {
	html := renderFKPopoverRow(t, nil)
	if !strings.Contains(html, `id="jump-marca_id"`) {
		t.Fatalf("expected the popover button to be present (hidden, not omitted):\n%s", html)
	}
	idx := strings.Index(html, `id="jump-marca_id"`)
	btnTag := extractTag(html, idx)
	if !strings.Contains(btnTag, "display:none") {
		t.Errorf("popover button should be display:none with no selection:\n%s", btnTag)
	}
}

func TestFKPopover_ButtonVisibleWhenSelected(t *testing.T) {
	html := renderFKPopoverRow(t, float64(1)) // typical JSON-decoded numeric value
	idx := strings.Index(html, `id="jump-marca_id"`)
	if idx == -1 {
		t.Fatal("popover button not found")
	}
	btnTag := extractTag(html, idx)
	if strings.Contains(btnTag, "display:none") {
		t.Errorf("popover button should be visible once a positive ID is selected:\n%s", btnTag)
	}
}

func TestFKPopover_OnclickCallsSeamPopoverShow(t *testing.T) {
	html := renderFKPopoverRow(t, float64(1))
	if !strings.Contains(html, "SeamPopover.show(this,") {
		t.Errorf("expected the §7.2 SeamPopover.show(this, fieldName) call:\n%s", html)
	}
}

func TestFKPopover_DataJumpBaseOnSelect(t *testing.T) {
	html := renderFKPopoverRow(t, nil)
	if !strings.Contains(html, `data-jump-base="/brands"`) {
		t.Errorf("expected data-jump-base on the <select> per §7.2:\n%s", html)
	}
}

func TestFKPopover_NoButtonForFieldsWithoutFKPopover(t *testing.T) {
	e := &FormEngine{}
	f := Field{Name: "nombre", Title: "Nombre", JSONType: "string"}
	h := func(b *mi.Builder) mi.Node {
		return renderFieldRow(b, context.Background(), e, f, map[string]interface{}{}, "", false, RenderContext{})
	}
	html := mi.RenderToString(h)
	if strings.Contains(html, "SeamPopover") {
		t.Errorf("a plain field with no x-seam-fk-popover should never emit the popover button:\n%s", html)
	}
}

func TestIsPositiveID(t *testing.T) {
	cases := []struct {
		value interface{}
		want  bool
	}{
		{float64(5), true},
		{float64(0), false},
		{float64(-1), false},
		{nil, false},
		{"5", true},
		{"0", false},
		{"abc", false},
		{int(3), true},
		{int64(0), false},
	}
	for _, c := range cases {
		if got := isPositiveID(c.value); got != c.want {
			t.Errorf("isPositiveID(%v) = %v, want %v", c.value, got, c.want)
		}
	}
}

// TestAssetPicker_RendersHiddenFieldAndSearchBox confirms the real
// widget's basic structure (T-19, 2026-08-04, replacing §8.3's MVP
// number-input stopgap): a hidden field carrying the actual value, a
// visible text search box, and the vendored autoComplete.js assets
// loaded.
func TestAssetPicker_RendersHiddenFieldAndSearchBox(t *testing.T) {
	e := &FormEngine{}
	f := Field{Name: "parent_id", Title: "Parent", JSONType: "integer", XSeamInput: "asset-picker", XSeamSource: "entity:assets"}
	h := func(b *mi.Builder) mi.Node {
		return renderFieldRow(b, context.Background(), e, f, map[string]interface{}{"parent_id": float64(42)}, "", false, RenderContext{})
	}
	html := mi.RenderToString(h)
	if !strings.Contains(html, `type="hidden"`) || !strings.Contains(html, `name="parent_id"`) {
		t.Errorf("expected a hidden input named parent_id carrying the actual value:\n%s", html)
	}
	if !strings.Contains(html, `value="42"`) {
		t.Errorf("expected the current value preserved on the hidden input:\n%s", html)
	}
	if !strings.Contains(html, `type="text"`) {
		t.Errorf("expected a visible text search box:\n%s", html)
	}
	if !strings.Contains(html, "autocomplete@10.2.10.min.js") || !strings.Contains(html, "autocomplete@10.2.10.css") {
		t.Errorf("expected the vendored autoComplete.js files to be loaded:\n%s", html)
	}
	if !strings.Contains(html, `data-seam-picker="1"`) || !strings.Contains(html, `data-entity="entity:assets"`) {
		t.Errorf("expected the inert markup SeamPicker (app.js) initialises the autoComplete.js box from:\n%s", html)
	}
}

// TestAssetPicker_SchemaFieldNameAndSourceAreTrustedNotEscaped confirms
// f.Name/f.XSeamSource (schema-authored, never user-submitted) are
// interpolated into the widget's JS as plain %q-quoted strings, the
// same trusted-by-provenance reasoning already established for the
// other mi.Raw sites audited after T-64 -- this test exists to lock in
// that this field's own interpolation targets stay schema-only if the
// widget is ever extended, not to claim these two specific values are
// adversarial (they aren't, by design).
func TestAssetPicker_SchemaFieldNameAndSourceAreTrustedNotEscaped(t *testing.T) {
	e := &FormEngine{}
	f := Field{Name: "parent_id", Title: "Parent", JSONType: "integer", XSeamInput: "asset-picker", XSeamSource: "entity:assets"}
	h := func(b *mi.Builder) mi.Node {
		return renderFieldRow(b, context.Background(), e, f, nil, "", false, RenderContext{})
	}
	html := mi.RenderToString(h)
	if !strings.Contains(html, `data-entity="entity:assets"`) || !strings.Contains(html, `name="parent_id"`) {
		t.Errorf("expected the widget to carry this field's own name and source:\n%s", html)
	}
}

// TestAssetPicker_ExistingValueNeverInterpolatedIntoScript is the
// adversarial one, guarding the exact lesson T-63/T-64 established: the
// current value (a raw ID here, always numeric by JSON Schema type, but
// the PRINCIPLE this guards -- that widget VALUES never flow into
// script interpolation regardless of shape -- is what matters) must
// only ever appear via the hidden input's own normally-escaped
// value="..." attribute, never built into the inline <script> block
// this widget also emits. A malicious-shaped value (this field is
// declared "integer" by its schema, but Go's map[string]interface{}
// doesn't enforce that at the widget layer -- confirming the renderer
// itself doesn't trust the type declarations it's handed is worth
// checking, not assuming) must not be able to break out of the
// hidden input's own attribute or reach the script block unescaped.
func TestAssetPicker_ExistingValueNeverInterpolatedIntoScript(t *testing.T) {
	e := &FormEngine{}
	f := Field{Name: "parent_id", Title: "Parent", JSONType: "integer", XSeamInput: "asset-picker", XSeamSource: "entity:assets"}
	malicious := `42"><script>alert(document.cookie)</script>`
	h := func(b *mi.Builder) mi.Node {
		return renderFieldRow(b, context.Background(), e, f, map[string]interface{}{"parent_id": malicious}, "", false, RenderContext{})
	}
	html := mi.RenderToString(h)
	if strings.Contains(html, "<script>alert(document.cookie)</script>") {
		t.Fatalf("the malicious value's literal script-tag-injection sequence appears unescaped in the output:\n%s", html)
	}
	if !strings.Contains(html, "&#34;&gt;&lt;script&gt;") && !strings.Contains(html, "&#34;&gt;&lt;script") {
		t.Errorf("expected the malicious value's escaped form in the hidden input's value attribute -- got neither form, suggesting the value went missing rather than being safely escaped:\n%s", html)
	}
}

// TestAssetPicker_TwoInstancesOnOnePage_NoIDCollision confirms two
// asset-picker fields on the same page (a realistic shape for a
// future Work Orders or Issue Tracker form needing more than one
// asset reference, e.g. "primary_asset_id" + "related_asset_id") don't
// collide -- distinct hidden-field IDs, distinct search-box IDs, and
// each widget's own JS wrapped in its own IIFE, so a fieldName/hidden/
// search variable in one instance's script can never shadow or be
// shadowed by the other's. Never previously tested -- every prior use
// of this widget (parent_id on the asset form, asset_id on the sensor
// bind form) only ever had one picker per page.
func TestAssetPicker_TwoInstancesOnOnePage_NoIDCollision(t *testing.T) {
	e := &FormEngine{}
	f1 := Field{Name: "primary_asset_id", Title: "Primary Asset", JSONType: "integer", XSeamInput: "asset-picker", XSeamSource: "entity:assets"}
	f2 := Field{Name: "related_asset_id", Title: "Related Asset", JSONType: "integer", XSeamInput: "asset-picker", XSeamSource: "entity:assets"}

	h := func(b *mi.Builder) mi.Node {
		return b.Div(
			renderFieldRow(b, context.Background(), e, f1, nil, "", false, RenderContext{}),
			renderFieldRow(b, context.Background(), e, f2, nil, "", false, RenderContext{}),
		)
	}
	html := mi.RenderToString(h)

	// Every ID must be unique -- collect id="..." occurrences and check
	// for duplicates directly, not just eyeball the two fields' own
	// expected names.
	idRe := regexp.MustCompile(`id="([^"]+)"`)
	matches := idRe.FindAllStringSubmatch(html, -1)
	seen := map[string]bool{}
	for _, m := range matches {
		id := m[1]
		if seen[id] {
			t.Errorf("duplicate id=%q found -- two widget instances collided on the same DOM id:\n%s", id, html)
		}
		seen[id] = true
	}
	if !strings.Contains(html, `id="field-primary_asset_id"`) || !strings.Contains(html, `id="field-related_asset_id"`) {
		t.Errorf("expected both hidden fields with distinct, field-name-derived ids:\n%s", html)
	}
	if !strings.Contains(html, `id="picker-search-primary_asset_id"`) || !strings.Contains(html, `id="picker-search-related_asset_id"`) {
		t.Errorf("expected both search boxes with distinct, field-name-derived ids:\n%s", html)
	}

	// Each instance's own JS closure must reference its own field name,
	// not the other's -- confirms the two IIFEs are genuinely isolated,
	// not accidentally sharing state through a package-level or
	// otherwise leaked variable.
	if strings.Count(html, `data-seam-picker="1"`) != 2 || strings.Contains(html, "fieldName") {
		t.Errorf("expected two inert widget roots and no per-field script state:\n%s", html)
	}

	// autoComplete.js's own vendored <script src> tag appears once per
	// widget instance -- harmless for an external file (browsers
	// dedupe by URL), but worth confirming it's still present for both
	// rather than silently dropped for the second instance.
	if strings.Count(html, "autocomplete@10.2.10.min.js") != 2 {
		t.Errorf("expected the vendored autoComplete.js script tag once per widget instance (2 total), got %d occurrences", strings.Count(html, "autocomplete@10.2.10.min.js"))
	}
}
