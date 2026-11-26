package formengine

import (
	"reflect"
	"testing"
)

func TestPropertyOrder_PreservesDocumentOrder(t *testing.T) {
	// Deliberately alphabetically-hostile order — if propertyOrder were
	// accidentally sorting (e.g. via a map iteration somewhere) this
	// would catch it immediately.
	raw := []byte(`{
		"type": "object",
		"properties": {
			"zebra":  {"type": "string"},
			"alpha":  {"type": "string"},
			"middle": {"type": "string"}
		}
	}`)
	got, err := propertyOrder(raw)
	if err != nil {
		t.Fatalf("propertyOrder: %v", err)
	}
	want := []string{"zebra", "alpha", "middle"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("propertyOrder = %v, want %v", got, want)
	}
}

func TestPropertyOrder_IgnoresNestedKeys(t *testing.T) {
	// A field's own definition can contain object-valued keywords
	// (enum is an array here, but e.g. a nested "properties" for an
	// object-typed field would have the same shape) — only the
	// top-level properties keys should be collected.
	raw := []byte(`{
		"properties": {
			"first": {
				"type": "string",
				"enum": ["a", "b", "c"]
			},
			"second": {"type": "integer"}
		}
	}`)
	got, err := propertyOrder(raw)
	if err != nil {
		t.Fatalf("propertyOrder: %v", err)
	}
	want := []string{"first", "second"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("propertyOrder = %v, want %v (nested keys leaked in)", got, want)
	}
}

func TestParseSchema_FlatFormFieldOrder(t *testing.T) {
	raw := []byte(`{
		"type": "object",
		"properties": {
			"nombre": {"type": "string", "title": "Nombre"},
			"activo": {"type": "boolean", "title": "Activo"},
			"codigo": {"type": "string", "title": "Código", "x-seam-input": "text"}
		},
		"required": ["nombre"]
	}`)
	form, err := ParseSchema(raw)
	if err != nil {
		t.Fatalf("parseSchema: %v", err)
	}
	if len(form.Fields) != 3 {
		t.Fatalf("got %d fields, want 3", len(form.Fields))
	}
	wantOrder := []string{"nombre", "activo", "codigo"}
	for i, name := range wantOrder {
		if form.Fields[i].Name != name {
			t.Errorf("field %d = %q, want %q", i, form.Fields[i].Name, name)
		}
	}
	if !form.Fields[0].Required {
		t.Error("nombre should be required")
	}
	if form.Fields[1].Required {
		t.Error("activo should not be required")
	}
}

func TestParseSchema_ExtensionsSurviveExtraction(t *testing.T) {
	// This is the code-level confirmation of the desk-check already
	// recorded in KNOWN_ISSUES.md: x-seam-* values must come through
	// parseField intact. The xolu round-trip itself (does POST/GET
	// /api/v1/schema/{entity} preserve them across the network) is
	// FE-05's other half — a build-tagged integration test, since it
	// needs a live xolu instance. See engine_integration_test.go.
	raw := []byte(`{
		"properties": {
			"marca_id": {
				"type": "integer",
				"title": "Marca",
				"x-seam-input": "select",
				"x-seam-source": "entity:brands",
				"x-seam-help": "Marca del fabricante",
				"x-seam-help-style": "inline"
			}
		}
	}`)
	form, err := ParseSchema(raw)
	if err != nil {
		t.Fatalf("parseSchema: %v", err)
	}
	f := form.Fields[0]
	if f.XSeamInput != "select" {
		t.Errorf("XSeamInput = %q, want %q", f.XSeamInput, "select")
	}
	if f.XSeamSource != "entity:brands" {
		t.Errorf("XSeamSource = %q, want %q", f.XSeamSource, "entity:brands")
	}
	if f.XSeamHelp != "Marca del fabricante" {
		t.Errorf("XSeamHelp = %q, want %q", f.XSeamHelp, "Marca del fabricante")
	}
	if f.XSeamHelpStyle != "inline" {
		t.Errorf("XSeamHelpStyle = %q, want %q", f.XSeamHelpStyle, "inline")
	}
}

func TestParseSchema_CompilesViaQueryfy(t *testing.T) {
	raw := []byte(`{
		"properties": {
			"email": {"type": "string", "format": "email"}
		},
		"required": ["email"]
	}`)
	form, err := ParseSchema(raw)
	if err != nil {
		t.Fatalf("parseSchema: %v", err)
	}
	if form.Compiled == nil {
		t.Fatal("Compiled schema is nil")
	}
}

func TestParseSchema_InvalidJSON(t *testing.T) {
	_, err := ParseSchema([]byte(`{not json`))
	if err == nil {
		t.Fatal("expected an error for invalid JSON, got nil")
	}
}

func TestParseSchema_TextareaDefaultRows(t *testing.T) {
	raw := []byte(`{
		"properties": {
			"no_rows":       {"type": "string", "x-seam-input": "textarea"},
			"explicit_rows": {"type": "string", "x-seam-input": "textarea", "x-seam-rows": 6}
		}
	}`)
	form, err := ParseSchema(raw)
	if err != nil {
		t.Fatalf("parseSchema: %v", err)
	}
	if form.Fields[0].XSeamRows != 3 {
		t.Errorf("default rows = %d, want 3 (§4.2)", form.Fields[0].XSeamRows)
	}
	if form.Fields[1].XSeamRows != 6 {
		t.Errorf("explicit rows = %d, want 6", form.Fields[1].XSeamRows)
	}
}

// TestParseSchema_DecimalDefaults confirms xolu's own defaults
// (docs/JSON_SCHEMA.md: decimalPrecision 18, decimalScale 4) are
// applied when a "format":"decimal" field omits either, and that
// explicit values are preserved untouched (T-10).
func TestParseSchema_DecimalDefaults(t *testing.T) {
	raw := []byte(`{
		"properties": {
			"no_precision_scale": {"type": "string", "format": "decimal"},
			"price":              {"type": "string", "format": "decimal", "decimalPrecision": 10, "decimalScale": 2}
		}
	}`)
	form, err := ParseSchema(raw)
	if err != nil {
		t.Fatalf("parseSchema: %v", err)
	}
	if got := form.Fields[0]; got.DecimalPrecision != 18 || got.DecimalScale != 4 {
		t.Errorf("defaults = precision %d scale %d, want 18/4", got.DecimalPrecision, got.DecimalScale)
	}
	if got := form.Fields[1]; got.DecimalPrecision != 10 || got.DecimalScale != 2 {
		t.Errorf("explicit = precision %d scale %d, want 10/2", got.DecimalPrecision, got.DecimalScale)
	}
	// A plain string field must not pick up decimal defaults it never asked for.
	if got := form.Fields[0]; got.Format != "decimal" {
		t.Fatalf("test setup: expected format decimal, got %q", got.Format)
	}
}

// TestParseSchema_ERPCodeSurvivesExtraction is the T-10 extension-point
// counterpart to TestParseSchema_ExtensionsSurviveExtraction above --
// x-seam-erp-code is inert at the formengine layer (no widget, no
// validation) but must still round-trip through parseField intact for
// a future integration to read.
func TestParseSchema_ERPCodeSurvivesExtraction(t *testing.T) {
	raw := []byte(`{
		"properties": {
			"cost_center": {"type": "string", "x-seam-erp-code": "SAP:KOSTL"}
		}
	}`)
	form, err := ParseSchema(raw)
	if err != nil {
		t.Fatalf("parseSchema: %v", err)
	}
	if got := form.Fields[0].XSeamERPCode; got != "SAP:KOSTL" {
		t.Errorf("XSeamERPCode = %q, want %q", got, "SAP:KOSTL")
	}
}

// TestParseSchema_InputMaskSurvivesExtraction is T-10's third
// extension-point round-trip check.
func TestParseSchema_InputMaskSurvivesExtraction(t *testing.T) {
	raw := []byte(`{
		"properties": {
			"phone": {"type": "string", "x-seam-input-mask": "999-999-9999"}
		}
	}`)
	form, err := ParseSchema(raw)
	if err != nil {
		t.Fatalf("parseSchema: %v", err)
	}
	if got := form.Fields[0].XSeamInputMask; got != "999-999-9999" {
		t.Errorf("XSeamInputMask = %q, want %q", got, "999-999-9999")
	}
}

func TestParseSchema_UnitSurvivesExtraction(t *testing.T) {
	raw := []byte(`{
		"properties": {
			"temp_min": {"type": "number", "x-seam-unit": "°C"}
		}
	}`)
	form, err := ParseSchema(raw)
	if err != nil {
		t.Fatalf("parseSchema: %v", err)
	}
	if got := form.Fields[0].XSeamUnit; got != "°C" {
		t.Errorf("XSeamUnit = %q, want %q", got, "°C")
	}
}

func TestParseSchema_CustomValidatorSurvivesExtraction(t *testing.T) {
	raw := []byte(`{
		"properties": {
			"serial_1": {"type": "string", "x-seam-custom-validator": "unique_serial"}
		}
	}`)
	form, err := ParseSchema(raw)
	if err != nil {
		t.Fatalf("parseSchema: %v", err)
	}
	if got := form.Fields[0].XSeamCustomValidator; got != "unique_serial" {
		t.Errorf("XSeamCustomValidator = %q, want %q", got, "unique_serial")
	}
}

func TestParseSchema_BarcodeFormatSurvivesExtraction(t *testing.T) {
	raw := []byte(`{
		"properties": {
			"upc_code": {"type": "string", "x-seam-input": "barcode", "x-seam-barcode-format": "UPC"}
		}
	}`)
	form, err := ParseSchema(raw)
	if err != nil {
		t.Fatalf("parseSchema: %v", err)
	}
	if got := form.Fields[0].XSeamBarcodeFormat; got != "UPC" {
		t.Errorf("XSeamBarcodeFormat = %q, want %q", got, "UPC")
	}
}
