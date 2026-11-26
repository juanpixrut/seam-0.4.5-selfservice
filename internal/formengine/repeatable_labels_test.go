package formengine

import (
	"context"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"
)

// Cells of a repeatable row say what they are (placeholder, accessible name,
// select empty option), and nested rows are visually distinct from the rows
// of the repeatable that holds them.

func identityRowsField(t *testing.T, keyPlaceholder string) Field {
	t.Helper()
	extra := ""
	if keyPlaceholder != "" {
		extra = `, "x-seam-placeholder": "` + keyPlaceholder + `"`
	}
	form, err := ParseSchema([]byte(`{
	  "type": "object",
	  "properties": {
	    "identity": {"type": "array", "title": "Identity", "x-seam-input": "repeatable",
	      "items": {"type": "object", "properties": {
	        "key": {"type": "string", "title": "Key", "x-seam-input": "text"` + extra + `},
	        "kind": {"type": "string", "title": "Type", "x-seam-input": "select", "enum": ["text", "number"]},
	        "amount": {"type": "number", "title": "Amount", "x-seam-input": "number"},
	        "required": {"type": "boolean", "title": "Required", "x-seam-input": "checkbox"}}}}
	  }}`))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	return form.Fields[0]
}

func TestRepeatableFlatRow_CellsAreNamedByPlaceholderAndEmptyOption(t *testing.T) {
	html := renderRepeatableHTML(identityRowsField(t, ""), nil)
	for _, want := range []string{
		`placeholder="Key"`, `aria-label="Key"`, // text input
		`placeholder="Amount"`, `aria-label="Amount"`, // number input
		"\u2014 Type \u2014", `aria-label="Type"`, // select: the empty option names the field
		">Required<", // checkbox: its own title text
	} {
		if !strings.Contains(html, want) {
			t.Errorf("flat row missing %s:\n%s", want, html)
		}
	}
}

func TestRepeatableFlatRow_XSeamPlaceholderOverridesTheTitle(t *testing.T) {
	html := renderRepeatableHTML(identityRowsField(t, "key (e.g. serial_no)"), nil)
	if !strings.Contains(html, `placeholder="key (e.g. serial_no)"`) {
		t.Errorf("x-seam-placeholder must be the cell's placeholder:\n%s", html)
	}
	if strings.Contains(html, `placeholder="Key"`) {
		t.Errorf("the title placeholder must not also appear")
	}
}

// Row visuals are CSS (app.css), driven by classes; the markup carries no
// inline colours that could go wrong in dark mode.
func TestRepeatableFlatRow_UsesTheSharedIconButtonsAndNoInlineColours(t *testing.T) {
	html := renderRepeatableHTML(identityRowsField(t, ""), nil)
	for _, want := range []string{
		`class="flex gap-2 items-start repeatable-row"`,
		`class="fe-btn fe-add"`, ">add<", // add: outlined icon button
		`class="fe-remove"`, ">remove_circle_outline<", `aria-label="Remove"`, // remove: the Identity Fields' icon
	} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %s:\n%s", want, html)
		}
	}
	if strings.Contains(html, "rgba(") || strings.Contains(html, "\u2715") {
		t.Errorf("no inline colours or text glyph buttons any more:\n%s", html)
	}
}

// Outside a repeatable a field has no placeholder unless it sets one.
func TestPlaceholder_OutsideRowsOnlyWhenSet(t *testing.T) {
	plain := renderWidgetHTML(renderTextLike("text"), Field{Name: "a", Title: "A", XSeamInput: "text"}, nil)
	if strings.Contains(plain, "placeholder=") || strings.Contains(plain, "aria-label=") {
		t.Errorf("a plain field must not get a placeholder: %s", plain)
	}
	set := renderWidgetHTML(renderTextLike("text"), Field{Name: "a", Title: "A", XSeamInput: "text", XSeamPlaceholder: "e.g. 42"}, nil)
	if !strings.Contains(set, `placeholder="e.g. 42"`) {
		t.Errorf("x-seam-placeholder must be used: %s", set)
	}
	sel := renderWidgetHTML(renderSelect, Field{Name: "s", Title: "S", XSeamInput: "select", Enum: []string{"x"}, XSeamPlaceholder: "Pick one"}, nil)
	if !strings.Contains(sel, ">Pick one<") {
		t.Errorf("a select's empty option must use x-seam-placeholder: %s", sel)
	}
}

func TestRepeatableIndexed_NestedRowsAreVisuallyDistinctAndNamed(t *testing.T) {
	html := renderRepeatableHTML(blocksSchema(t), nil)
	for _, want := range []string{
		`class="fe-rep-nested"`,             // nested box: indented under a rule (CSS)
		`class="fe-rep-heading"`, ">Lines<", // nested heading
		`placeholder="Block name"`, `placeholder="Tag"`,
		"\u2014 Type \u2014",
		`placeholder="Responsible"`, // any-picker cell
	} {
		if !strings.Contains(html, want) {
			t.Errorf("indexed render missing %s:\n%s", want, html)
		}
	}
	if strings.Count(html, `class="fe-rep-nested"`) < 2 {
		t.Errorf("the nested box must be styled both in the live row and in the template")
	}
}

func TestRepeatableRow_CheckboxAndToggleTitleTranslation(t *testing.T) {
	form, err := ParseSchema([]byte(`{
	  "type": "object",
	  "properties": {
	    "items": {"type": "array", "x-seam-input": "repeatable",
	      "items": {"type": "object", "properties": {
	        "is_active": {"type": "boolean", "title": "Active Schema Title", "x-seam-input": "checkbox"},
	        "is_enabled": {"type": "boolean", "title": "Enabled Schema Title", "x-seam-input": "toggle"}
	      }}}
	  }}`))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	tr := fakeTranslator{"es": {
		"repeatable_item.items.is_active": "Requerido Activo",
		"repeatable_item.is_enabled":      "Habilitado Toggle",
	}}
	e := &FormEngine{Translator: tr}
	rc := RenderContext{Locale: "es"}
	node := renderRepeatable(context.Background(), e, form.Fields[0], nil, false, rc)
	html := mi.RenderToString(node)

	if !strings.Contains(html, ">Requerido Activo<") {
		t.Errorf("expected translated checkbox title, got:\n%s", html)
	}
	if !strings.Contains(html, ">Habilitado Toggle<") {
		t.Errorf("expected translated toggle title, got:\n%s", html)
	}
	if strings.Contains(html, "Active Schema Title") || strings.Contains(html, "Enabled Schema Title") {
		t.Errorf("raw schema title should not be rendered when translation exists:\n%s", html)
	}
}

func TestRepeatableIndexed_CheckboxAndToggleTitleTranslation(t *testing.T) {
	form, err := ParseSchema([]byte(`{
	  "type": "object",
	  "properties": {
	    "items": {"type": "array", "x-seam-input": "repeatable", "x-seam-naming": "indexed",
	      "items": {"type": "object", "properties": {
	        "is_active": {"type": "boolean", "title": "Active Schema Title", "x-seam-input": "checkbox"},
	        "is_enabled": {"type": "boolean", "title": "Enabled Schema Title", "x-seam-input": "toggle"}
	      }}}
	  }}`))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	tr := fakeTranslator{"es": {
		"repeatable_item.items.is_active": "Requerido Activo",
		"repeatable_item.is_enabled":      "Habilitado Toggle",
	}}
	e := &FormEngine{Translator: tr}
	rc := RenderContext{Locale: "es"}
	node := renderRepeatable(context.Background(), e, form.Fields[0], nil, false, rc)
	html := mi.RenderToString(node)

	if !strings.Contains(html, ">Requerido Activo<") {
		t.Errorf("expected translated checkbox title, got:\n%s", html)
	}
	if !strings.Contains(html, ">Habilitado Toggle<") {
		t.Errorf("expected translated toggle title, got:\n%s", html)
	}
	if strings.Contains(html, "Active Schema Title") || strings.Contains(html, "Enabled Schema Title") {
		t.Errorf("raw schema title should not be rendered when translation exists:\n%s", html)
	}
}

func TestRepeatableRow_DecimalPlaceholderTranslation(t *testing.T) {
	form, err := ParseSchema([]byte(`{
	  "type": "object",
	  "properties": {
	    "items": {"type": "array", "x-seam-input": "repeatable",
	      "items": {"type": "object", "properties": {
	        "price": {"type": "string", "format": "decimal", "title": "Price", "decimalPrecision": 10, "decimalScale": 2}
	      }}}
	  }}`))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	tr := fakeTranslator{"es": {
		"repeatable_item.items.price": "Precio Unitario",
	}}
	e := &FormEngine{Translator: tr}
	rc := RenderContext{Locale: "es"}
	node := renderRepeatable(context.Background(), e, form.Fields[0], nil, false, rc)
	html := mi.RenderToString(node)

	if !strings.Contains(html, `placeholder="Precio Unitario"`) {
		t.Errorf("expected translated decimal placeholder, got:\n%s", html)
	}
	if !strings.Contains(html, `aria-label="Precio Unitario"`) {
		t.Errorf("expected translated decimal aria-label, got:\n%s", html)
	}
}

func TestRepeatableIndexed_DecimalPlaceholderTranslation(t *testing.T) {
	form, err := ParseSchema([]byte(`{
	  "type": "object",
	  "properties": {
	    "items": {"type": "array", "x-seam-input": "repeatable", "x-seam-naming": "indexed",
	      "items": {"type": "object", "properties": {
	        "price": {"type": "string", "format": "decimal", "title": "Price", "decimalPrecision": 10, "decimalScale": 2}
	      }}}
	  }}`))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	tr := fakeTranslator{"es": {
		"repeatable_item.items.price": "Precio Unitario",
	}}
	e := &FormEngine{Translator: tr}
	rc := RenderContext{Locale: "es"}
	node := renderRepeatable(context.Background(), e, form.Fields[0], nil, false, rc)
	html := mi.RenderToString(node)

	if !strings.Contains(html, `placeholder="Precio Unitario"`) {
		t.Errorf("expected translated decimal placeholder, got:\n%s", html)
	}
	if !strings.Contains(html, `aria-label="Precio Unitario"`) {
		t.Errorf("expected translated decimal aria-label, got:\n%s", html)
	}
}
