package formengine

import (
	"context"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"
)

func TestFormEngine_Render_FlatForm(t *testing.T) {
	schema := []byte(`{
		"properties": {
			"codigo":     {"type": "string", "title": "Código", "required": false},
			"nombre":     {"type": "string", "title": "Nombre"},
			"secreto":    {"type": "string", "title": "Campo Oculto"},
			"solo_lectura": {"type": "string", "title": "Solo Lectura"}
		},
		"required": ["nombre"]
	}`)

	e := &FormEngine{}
	rc := RenderContext{
		Module: "masters", Form: "marca",
		HiddenFields:   []string{"secreto"},
		ReadOnlyFields: []string{"solo_lectura"},
	}
	values := map[string]interface{}{"codigo": "M001", "nombre": "Acme"}
	errs := map[string]string{}

	h := e.Render(context.Background(), schema, values, errs, rc)
	html := mi.RenderToString(h)

	if strings.Contains(html, "Campo Oculto") || strings.Contains(html, `name="secreto"`) {
		t.Errorf("hidden field must not appear in output at all:\n%s", html)
	}
	if !strings.Contains(html, "Solo Lectura") {
		t.Errorf("read-only field should still render (visually disabled, not omitted):\n%s", html)
	}
	if !strings.Contains(html, "opacity-60") {
		t.Errorf("read-only field should carry the read-only wrapper class:\n%s", html)
	}
	if !strings.Contains(html, `value="M001"`) {
		t.Errorf("expected the codigo field's current value to render:\n%s", html)
	}

	// Field order must match schema.properties order (codigo, nombre,
	// secreto[hidden], solo_lectura) -- codigo before nombre before
	// solo_lectura in the rendered output.
	iCodigo := strings.Index(html, "field-codigo")
	iNombre := strings.Index(html, "field-nombre")
	iReadOnly := strings.Index(html, "field-solo_lectura")
	if !(iCodigo < iNombre && iNombre < iReadOnly) {
		t.Errorf("fields out of schema order: codigo=%d nombre=%d solo_lectura=%d\n%s", iCodigo, iNombre, iReadOnly, html)
	}
}

func TestFormEngine_Render_InvalidSchemaShowsErrorBanner(t *testing.T) {
	e := &FormEngine{}
	h := e.Render(context.Background(), []byte(`{not valid json`), nil, nil, RenderContext{})
	html := mi.RenderToString(h)
	if !strings.Contains(html, "formengine:") {
		t.Errorf("expected a visible formengine error banner, got:\n%s", html)
	}
}

func TestFormEngine_Render_RCReadOnlyAppliesEverywhere(t *testing.T) {
	schema := []byte(`{"properties": {"a": {"type": "string"}, "b": {"type": "string"}}}`)
	e := &FormEngine{}
	h := e.Render(context.Background(), schema, nil, nil, RenderContext{ReadOnly: true})
	html := mi.RenderToString(h)
	if strings.Count(html, "opacity-60") != 2 {
		t.Errorf("RenderContext.ReadOnly=true should mark every field read-only, got %d matches:\n%s",
			strings.Count(html, "opacity-60"), html)
	}
}

func TestFormEngine_Render_CSRFTokenEmittedWhenPresent(t *testing.T) {
	schema := []byte(`{"properties": {"a": {"type": "string"}}}`)
	e := &FormEngine{}
	h := e.Render(context.Background(), schema, nil, nil, RenderContext{CSRFToken: "tok123"})
	html := mi.RenderToString(h)
	if !strings.Contains(html, `name="_csrf"`) || !strings.Contains(html, `value="tok123"`) {
		t.Errorf("expected a _csrf hidden input carrying the token:\n%s", html)
	}
}

func TestFormEngine_Render_NoCSRFInputWhenTokenEmpty(t *testing.T) {
	schema := []byte(`{"properties": {"a": {"type": "string"}}}`)
	e := &FormEngine{}
	h := e.Render(context.Background(), schema, nil, nil, RenderContext{})
	html := mi.RenderToString(h)
	if strings.Contains(html, "_csrf") {
		t.Errorf("did not expect a _csrf input with no token set:\n%s", html)
	}
}

// TestRenderFieldRow_SelfLabeledWidgetUsesResolvedTitle guards a real
// bug found 2026-08-03 (MOD-8, building the first checkbox/toggle
// fields ever wired into a live route in this codebase): renderFieldRow
// computed resolvedTitle but only ever passed it to fkPopoverAwareLabel,
// which selfLabeledWidgets (checkbox/toggle) skip entirely since they
// render their own label -- those widgets read f.Title directly
// (renderCheckboxLike), which was still the raw, untranslated schema
// literal. No test before this one exercised a real translation
// dictionary against a self-labeled widget (TestRenderCheckbox_
// ChecksBoolValue uses a nil-Translator FormEngine, so literal ==
// "resolved" either way and the bug had nowhere to show up).
func TestRenderFieldRow_SelfLabeledWidgetUsesResolvedTitle(t *testing.T) {
	tr := fakeTranslator{
		"es": {"field.email_alerts": "Alertas por correo"},
	}
	e := &FormEngine{Translator: tr}
	f := Field{Name: "email_alerts", Title: "Email alerts", JSONType: "boolean", XSeamInput: "toggle"}
	values := map[string]interface{}{"email_alerts": true}
	h := func(b *mi.Builder) mi.Node {
		return renderFieldRow(b, context.Background(), e, f, values, "", false, RenderContext{Locale: "es"})
	}
	html := mi.RenderToString(h)
	if strings.Contains(html, "Email alerts") {
		t.Errorf("toggle rendered the raw untranslated literal \"Email alerts\" instead of the resolved Spanish title:\n%s", html)
	}
	if !strings.Contains(html, "Alertas por correo") {
		t.Errorf("toggle did not render the resolved Spanish title \"Alertas por correo\":\n%s", html)
	}
}

// TestRenderFieldRow_XSeamUnit_NoOpWhenAbsent confirms the vast
// majority of fields (no x-seam-unit) render with no extra wrapper --
// same principle as the input-mask no-op test, applied at the
// renderFieldRow layer this time since that's where units are wired.
func TestRenderFieldRow_XSeamUnit_NoOpWhenAbsent(t *testing.T) {
	e := &FormEngine{}
	f := Field{Name: "quantity", Title: "Quantity", JSONType: "number"}
	values := map[string]interface{}{"quantity": 5}
	h := func(b *mi.Builder) mi.Node {
		return renderFieldRow(b, context.Background(), e, f, values, "", false, RenderContext{})
	}
	html := mi.RenderToString(h)
	if strings.Contains(html, "flex items-center gap-2") {
		t.Errorf("field with no x-seam-unit must not get the unit wrapper\n%s", html)
	}
}

// TestRenderFieldRow_XSeamUnit_RendersSuffix confirms the unit label
// appears beside the widget when x-seam-unit is set -- this is T-10's
// extension and directly resolves T-35's underlying problem (unit
// baked into translatable title prose instead of being structured).
func TestRenderFieldRow_XSeamUnit_RendersSuffix(t *testing.T) {
	e := &FormEngine{}
	f := Field{Name: "temp_min", Title: "Min. Temp.", JSONType: "number", XSeamUnit: "°C"}
	values := map[string]interface{}{"temp_min": -18}
	h := func(b *mi.Builder) mi.Node {
		return renderFieldRow(b, context.Background(), e, f, values, "", false, RenderContext{})
	}
	html := mi.RenderToString(h)
	for _, want := range []string{`name="temp_min"`, `>°C<`} {
		if !strings.Contains(html, want) {
			t.Errorf("output missing %q\n%s", want, html)
		}
	}
	// The title itself must stay unit-free -- the whole point of the
	// extension is decoupling the unit from the translatable label.
	if strings.Contains(html, "Min. Temp. (°C)") {
		t.Errorf("unit must not be baked into the title text\n%s", html)
	}
}
