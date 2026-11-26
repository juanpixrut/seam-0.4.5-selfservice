package formengine

import (
	"context"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"
)

func assetTypeField() Field {
	return Field{
		Name: "type_id", Title: "Tipo de Activo", JSONType: "integer",
		XSeamInput: "asset-type-select", XSeamSource: "entity:asset-types", XSeamFKPopover: "/asset-types",
		Required: true,
	}
}

func renderAssetTypeRow(t *testing.T, value interface{}) string {
	t.Helper()
	e := &FormEngine{Sources: StaticSourceResolver{
		"entity:asset-types": []Option{{Value: "1", Label: "Compresor"}, {Value: "2", Label: "Bomba"}},
	}}
	values := map[string]interface{}{"type_id": value}
	h := func(b *mi.Builder) mi.Node {
		return renderFieldRow(b, context.Background(), e, assetTypeField(), values, "", false, RenderContext{})
	}
	return mi.RenderToString(h)
}

func TestAssetTypeSelect_HTMXTriggerAttributes(t *testing.T) {
	html := renderAssetTypeRow(t, nil)
	idx := strings.Index(html, `id="field-type_id"`)
	if idx == -1 {
		t.Fatal("select element not found")
	}
	tag := extractTag(html, idx)
	if !strings.Contains(tag, `hx-get="/assets/type-fields"`) {
		t.Errorf("expected the dynamic-fields htmx-get, matched against assetTypeSelectWithHTMX's real attributes:\n%s", tag)
	}
	if !strings.Contains(tag, `hx-trigger="change"`) {
		t.Errorf("expected hx-trigger=change:\n%s", tag)
	}
	if !strings.Contains(tag, `hx-target="#asset-type-fields"`) {
		t.Errorf("expected hx-target=#asset-type-fields:\n%s", tag)
	}
	if !strings.Contains(tag, `hx-include="[name=type_id]"`) {
		t.Errorf("expected hx-include scoped to this field's own name:\n%s", tag)
	}
}

func TestAssetTypeSelect_OptionsResolvedViaSource(t *testing.T) {
	html := renderAssetTypeRow(t, nil)
	if !strings.Contains(html, "Compresor") || !strings.Contains(html, "Bomba") {
		t.Errorf("expected options resolved via x-seam-source, same as a plain select:\n%s", html)
	}
}

func TestAssetTypeSelect_RequiredOmitsEmptyOption(t *testing.T) {
	// type_id is required in the real Asset schema (§12) -- §5.3/§5.4:
	// no empty option, same rule as a plain required select.
	html := renderAssetTypeRow(t, nil)
	if strings.Contains(html, "— Unassigned —") {
		t.Errorf("required field should not get the empty FK option:\n%s", html)
	}
	idx := strings.Index(html, `id="field-type_id"`)
	tag := extractTag(html, idx)
	if !strings.Contains(tag, " required") {
		t.Errorf("expected the required attribute:\n%s", tag)
	}
}

func TestAssetTypeSelect_FKPopoverStillWorks(t *testing.T) {
	// asset-type-select carries x-seam-fk-popover too (§12's example) --
	// confirms the label-level popover logic, which is widget-agnostic,
	// composes correctly with the htmx-extended select.
	html := renderAssetTypeRow(t, float64(1))
	if !strings.Contains(html, "SeamPopover.show(this,") {
		t.Errorf("expected the fk-popover button to still render for asset-type-select:\n%s", html)
	}
	idx := strings.Index(html, `id="jump-type_id"`)
	if idx == -1 {
		t.Fatal("popover button not found")
	}
	tag := extractTag(html, idx)
	if strings.Contains(tag, "display:none") {
		t.Errorf("popover button should be visible once a positive ID is selected:\n%s", tag)
	}
}

func TestAssetTypeSelect_PlainSelectUnaffectedByRefactor(t *testing.T) {
	// Regression check for the renderSelect/renderSelectWithExtraAttrs
	// split: a plain select (no asset-type-select involved) must not
	// pick up any of the htmx attributes.
	e := &FormEngine{}
	f := Field{Name: "status", Title: "Status", JSONType: "string", XSeamInput: "select", Enum: []string{"active", "lost"}}
	h := func(b *mi.Builder) mi.Node {
		return renderFieldRow(b, context.Background(), e, f, map[string]interface{}{}, "", false, RenderContext{})
	}
	html := mi.RenderToString(h)
	if strings.Contains(html, "hx-get") || strings.Contains(html, "asset-type-fields") {
		t.Errorf("a plain select must not carry asset-type-select's htmx attributes:\n%s", html)
	}
}
