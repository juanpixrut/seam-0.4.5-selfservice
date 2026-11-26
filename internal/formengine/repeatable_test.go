package formengine

import (
	"context"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"
)

func serviceConfigsField() Field {
	return Field{
		Name: "service_configs", Title: "Configuraciones de Service", JSONType: "array",
		XSeamInput: "repeatable",
		Items: []Field{
			{Name: "medida", Title: "Medida", JSONType: "string", XSeamInput: "select", Enum: []string{"Kms", "Horas", "Usos"}},
			{Name: "umbral", Title: "Umbral", JSONType: "number"},
			{Name: "alerta_id", Title: "Alerta", JSONType: "integer", XSeamInput: "select", XSeamSource: "entity:alert-types"},
		},
	}
}

func renderRepeatableRow(t *testing.T, value interface{}) string {
	t.Helper()
	e := &FormEngine{Sources: StaticSourceResolver{
		"entity:alert-types": []Option{{Value: "1", Label: "Temp alta (Alta)"}},
	}}
	values := map[string]interface{}{"service_configs": value}
	h := func(b *mi.Builder) mi.Node {
		return renderFieldRow(b, context.Background(), e, serviceConfigsField(), values, "", false, RenderContext{})
	}
	return mi.RenderToString(h)
}

func TestRepeatable_NoValue_RendersOneEmptyRow(t *testing.T) {
	html := renderRepeatableRow(t, nil)
	if strings.Count(html, `class="flex gap-2 items-start repeatable-row"`) != 1 {
		t.Fatalf("expected exactly 1 empty row:\n%s", html)
	}
}

func TestRepeatable_MixedWidgetTypesInOneRow(t *testing.T) {
	html := renderRepeatableRow(t, nil)
	if !strings.Contains(html, `name="service_configs_medida[]"`) {
		t.Errorf("expected the select sub-field's array name:\n%s", html)
	}
	if !strings.Contains(html, `name="service_configs_umbral[]"`) {
		t.Errorf("expected the number sub-field's array name:\n%s", html)
	}
	if !strings.Contains(html, `name="service_configs_alerta_id[]"`) {
		t.Errorf("expected the source-select sub-field's array name:\n%s", html)
	}
	if !strings.Contains(html, `type="number"`) {
		t.Errorf("expected umbral to render as a real number input (via the normal widget dispatch):\n%s", html)
	}
	if !strings.Contains(html, "Kms") || !strings.Contains(html, "Horas") || !strings.Contains(html, "Usos") {
		t.Errorf("expected medida's enum options:\n%s", html)
	}
	if !strings.Contains(html, "Temp alta (Alta)") {
		t.Errorf("expected alerta_id's x-seam-source options resolved via the configured SourceResolver:\n%s", html)
	}
}

func TestRepeatable_ExistingValuesPopulateMultipleRows(t *testing.T) {
	value := []interface{}{
		map[string]interface{}{"medida": "Kms", "umbral": float64(10000), "alerta_id": float64(1)},
		map[string]interface{}{"medida": "Horas", "umbral": float64(500), "alerta_id": float64(0)},
	}
	html := renderRepeatableRow(t, value)
	if strings.Count(html, `class="flex gap-2 items-start repeatable-row"`) != 2 {
		t.Errorf("expected exactly 2 rows for 2 existing configs:\n%s", html)
	}
	if !strings.Contains(html, `value="10000"`) {
		t.Errorf("expected the first row's umbral value populated:\n%s", html)
	}
}

func TestRepeatable_RemoveButtonPerRow(t *testing.T) {
	value := []interface{}{
		map[string]interface{}{"medida": "Kms"},
		map[string]interface{}{"medida": "Horas"},
	}
	html := renderRepeatableRow(t, value)
	want := "b.closest(&#39;.repeatable-row&#39;)"
	if strings.Count(html, want) != 2 {
		t.Errorf("expected 2 remove buttons (one per row), found %d", strings.Count(html, want))
	}
}

func TestRepeatable_AddButtonClonesAndResetsBothInputsAndSelects(t *testing.T) {
	html := renderRepeatableRow(t, nil)
	// The add button is the outlined icon button: the add icon plus the label.
	if !strings.Contains(html, `class="fe-btn fe-add"`) || !strings.Contains(html, ">add<") || !strings.Contains(html, "Add") {
		t.Errorf("expected the add-row icon button with its label:\n%s", html)
	}
	if !strings.Contains(html, "cloneNode(true)") {
		t.Errorf("expected the add-row JS to clone the first row:\n%s", html)
	}
	if !strings.Contains(html, "selectedIndex=0") {
		t.Errorf("expected the add-row JS to reset select elements too, not just inputs:\n%s", html)
	}
}

func TestRepeatable_UnsupportedItemWidgetFallsBackGracefully(t *testing.T) {
	f := Field{
		Name: "weird", Title: "Weird", JSONType: "array", XSeamInput: "repeatable",
		Items: []Field{{Name: "x", Title: "X", XSeamInput: "totally-bogus-widget"}},
	}
	e := &FormEngine{}
	h := func(b *mi.Builder) mi.Node {
		return renderFieldRow(b, context.Background(), e, f, map[string]interface{}{}, "", false, RenderContext{})
	}
	html := mi.RenderToString(h) // must not panic
	if !strings.Contains(html, "not implemented") {
		t.Errorf("expected the unsupported-widget fallback within the row, not a crash:\n%s", html)
	}
}

// Regression: removing every row used to break the widget -- the add button
// cloned the first remaining .repeatable-row, which is null once all rows
// are gone. The remove handler now stashes a template row on the container
// before removing, and the add handler falls back to it.
func TestRepeatable_AddFallsBackToStashedTemplateAfterLastRowRemoved(t *testing.T) {
	html := renderRepeatableRow(t, nil)
	si := strings.Index(html, "c._rowTemplate=r.cloneNode(true)")
	ri := strings.Index(html, "r.remove()")
	if si < 0 || ri < 0 || si > ri {
		t.Fatalf("expected the remove handler to stash a template row before removing it:\n%s", html)
	}
	if !strings.Contains(html, "c._rowTemplate||c.querySelector(&#39;.repeatable-row&#39;)") {
		t.Errorf("expected the add handler to prefer the stashed template:\n%s", html)
	}
	if !strings.Contains(html, "if(!src){return;}") {
		t.Errorf("expected the add handler to guard a missing source row:\n%s", html)
	}
}
