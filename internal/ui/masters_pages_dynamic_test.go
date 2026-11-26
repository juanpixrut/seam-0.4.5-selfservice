package ui

// masters_pages_dynamic_test.go — FE-08: structural parity between the
// static forms (masters_pages.go) and their formengine-backed equivalents
// (masters_pages_dynamic.go). Checks what actually matters for a form to
// behave the same way: the same fields, in the same order, carrying the
// same required/value/error state -- not byte-identical HTML, which
// would be a brittle, meaningless bar (class names, whitespace, and
// attribute ordering are free to differ between the two renderers).

import (
	"regexp"
	"testing"

	mi "github.com/ha1tch/minty"
)

var fieldNameRe = regexp.MustCompile(`name="([a-z_]+)"`)

// fieldNamesInOrder returns the ordered list of form field names appearing
// in html (via their name="..." attribute), skipping "_csrf" -- present in
// both renderers but not part of the schema-driven field set being
// compared here.
func fieldNamesInOrder(html string) []string {
	var names []string
	for _, m := range fieldNameRe.FindAllStringSubmatch(html, -1) {
		if m[1] == "_csrf" {
			continue
		}
		names = append(names, m[1])
	}
	return names
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestMastersFormParity_Region(t *testing.T) {
	data := RegionFormData{Description: "Litoral", CSRFToken: "tok"}
	staticHTML := mi.RenderToString(RegionFormPage(data, nil))
	dynamicHTML := mi.RenderToString(RegionFormPageDynamic(data))

	staticFields := fieldNamesInOrder(staticHTML)
	dynamicFields := fieldNamesInOrder(dynamicHTML)
	if !sameStrings(staticFields, dynamicFields) {
		t.Errorf("field sets/order differ:\n  static:  %v\n  dynamic: %v", staticFields, dynamicFields)
	}
	if !containsAll(dynamicHTML, "Litoral") {
		t.Errorf("dynamic render missing the current value:\n%s", dynamicHTML)
	}
}

func TestMastersFormParity_Moneda(t *testing.T) {
	data := MonedaFormData{IsoCode: "USD", Symbol: "$", CSRFToken: "tok"}
	staticHTML := mi.RenderToString(MonedaFormPage(data, nil))
	dynamicHTML := mi.RenderToString(MonedaFormPageDynamic(data))

	staticFields := fieldNamesInOrder(staticHTML)
	dynamicFields := fieldNamesInOrder(dynamicHTML)
	if !sameStrings(staticFields, dynamicFields) {
		t.Errorf("field sets/order differ:\n  static:  %v\n  dynamic: %v", staticFields, dynamicFields)
	}
	if !containsAll(dynamicHTML, "USD", "$") {
		t.Errorf("dynamic render missing current values:\n%s", dynamicHTML)
	}
}

func TestMastersFormParity_Marca(t *testing.T) {
	data := MarcaFormData{Code: "ACM", Name: "Acme Corp", CSRFToken: "tok"}
	staticHTML := mi.RenderToString(MarcaFormPage(data, nil))
	dynamicHTML := mi.RenderToString(MarcaFormPageDynamic(data))

	staticFields := fieldNamesInOrder(staticHTML)
	dynamicFields := fieldNamesInOrder(dynamicHTML)
	if !sameStrings(staticFields, dynamicFields) {
		t.Errorf("field sets/order differ:\n  static:  %v\n  dynamic: %v", staticFields, dynamicFields)
	}
	if !containsAll(dynamicHTML, "ACM", "Acme Corp") {
		t.Errorf("dynamic render missing current values:\n%s", dynamicHTML)
	}
}

func TestMastersFormParity_ErrorsRenderInBoth(t *testing.T) {
	data := MarcaFormData{
		Code: "", Name: "Acme",
		Errors: map[string]string{"code": "El código es obligatorio"},
	}
	staticHTML := mi.RenderToString(MarcaFormPage(data, nil))
	dynamicHTML := mi.RenderToString(MarcaFormPageDynamic(data))

	if !containsAll(staticHTML, "El código es obligatorio") {
		t.Errorf("static render should show the field error:\n%s", staticHTML)
	}
	if !containsAll(dynamicHTML, "El código es obligatorio") {
		t.Errorf("dynamic render should show the field error:\n%s", dynamicHTML)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !regexp.MustCompile(regexp.QuoteMeta(sub)).MatchString(s) {
			return false
		}
	}
	return true
}
