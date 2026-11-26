package formengine

import (
	"context"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"
)

func TestInferWidget(t *testing.T) {
	cases := []struct {
		name string
		f    Field
		want string
	}{
		{"explicit x-seam-input wins", Field{JSONType: "string", XSeamInput: "textarea"}, "textarea"},
		{"string with no format", Field{JSONType: "string"}, "text"},
		{"string email format", Field{JSONType: "string", Format: "email"}, "email"},
		{"string uri format", Field{JSONType: "string", Format: "uri"}, "url"},
		{"string date format", Field{JSONType: "string", Format: "date"}, "date"},
		{"string date-time format", Field{JSONType: "string", Format: "date-time"}, "datetime-local"},
		{"string time format", Field{JSONType: "string", Format: "time"}, "time"},
		{"string tel format", Field{JSONType: "string", Format: "tel"}, "tel"},
		{"string with enum", Field{JSONType: "string", Enum: []string{"a", "b"}}, "select"},
		{"string decimal format", Field{JSONType: "string", Format: "decimal"}, "decimal"},
		{"integer", Field{JSONType: "integer"}, "number"},
		{"number", Field{JSONType: "number"}, "number"},
		{"boolean", Field{JSONType: "boolean"}, "checkbox"},
		{"array", Field{JSONType: "array"}, "repeatable"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := inferWidget(c.f)
			if got != c.want {
				t.Errorf("inferWidget(%+v) = %q, want %q", c.f, got, c.want)
			}
		})
	}
}

// renderWidget is a small test helper: builds the field row for f and
// returns the HTML string, for substring assertions below.
func renderWidget(t *testing.T, e *FormEngine, f Field, value interface{}, errMsg string, readOnly bool) string {
	t.Helper()
	values := map[string]interface{}{f.Name: value}
	h := func(b *mi.Builder) mi.Node {
		return renderFieldRow(b, context.Background(), e, f, values, errMsg, readOnly, RenderContext{})
	}
	return mi.RenderToString(h)
}

func TestRenderTextLike_ValueAndRequired(t *testing.T) {
	e := &FormEngine{}
	f := Field{Name: "nombre", Title: "Nombre", JSONType: "string", Required: true}
	html := renderWidget(t, e, f, "Grúa 12", "", false)
	for _, want := range []string{`name="nombre"`, `value="Grúa 12"`, `required`, `type="text"`} {
		if !strings.Contains(html, want) {
			t.Errorf("output missing %q\n%s", want, html)
		}
	}
	// No x-seam-input-mask set: must not emit a wrapper <div> or
	// <script> at all -- confirms T-10's mask addition is a true no-op
	// for the overwhelming majority of existing text-like fields.
	if strings.Contains(html, "<script") {
		t.Errorf("unmasked text-like field must not emit a <script>\n%s", html)
	}
}

// TestRenderTextLike_InputMask_LiteralsAndMaxlength confirms the mask
// wrapper only appears when XSeamInputMask is set, and that the
// placeholder/maxlength hints match the mask's own length.
func TestRenderTextLike_InputMask_LiteralsAndMaxlength(t *testing.T) {
	e := &FormEngine{}
	f := Field{Name: "phone", Title: "Phone", JSONType: "string", XSeamInputMask: "999-999-9999"}
	html := renderWidget(t, e, f, "", "", false)
	for _, want := range []string{`name="phone"`, `placeholder="999-999-9999"`, `maxlength="12"`, `data-seam-mask="999-999-9999"`, `id="field-phone"`} {
		if !strings.Contains(html, want) {
			t.Errorf("output missing %q\n%s", want, html)
		}
	}
	// The markup is inert (the mask rides in a data attribute that SeamWidgets
	// reads), so the mask also works in a row a repeatable adds later.
	if strings.Contains(html, "<script") {
		t.Errorf("a masked input must not carry an inline script\n%s", html)
	}
}

func TestRenderCheckbox_ChecksBoolValue(t *testing.T) {
	e := &FormEngine{}
	f := Field{Name: "activo", Title: "Activo", JSONType: "boolean"}
	htmlChecked := renderWidget(t, e, f, true, "", false)
	if !strings.Contains(htmlChecked, "checked") {
		t.Errorf("expected checked attribute when value=true:\n%s", htmlChecked)
	}
	htmlUnchecked := renderWidget(t, e, f, false, "", false)
	if strings.Contains(htmlUnchecked, "checked") {
		t.Errorf("did not expect checked attribute when value=false:\n%s", htmlUnchecked)
	}
	// Checkbox is self-labeled -- renderFieldRow must not also emit a
	// top <label for=...> (that would show the title twice).
	if strings.Count(htmlChecked, f.Title) != 1 {
		t.Errorf("expected title to appear exactly once (self-labeled widget), got %d times:\n%s",
			strings.Count(htmlChecked, f.Title), htmlChecked)
	}
}

func TestRenderSelect_Enum_RequiredOmitsEmptyOption(t *testing.T) {
	e := &FormEngine{}
	f := Field{
		Name: "calidad", Title: "Calidad", JSONType: "string",
		Enum: []string{"Propiedad", "Comodato"}, Required: true,
	}
	html := renderWidget(t, e, f, "Comodato", "", false)
	if strings.Contains(html, "— Select —") {
		t.Errorf("required select should not have an empty option:\n%s", html)
	}
	if !strings.Contains(html, "selected") {
		t.Errorf("expected the current value's <option> to be marked selected:\n%s", html)
	}
	for _, want := range []string{"Propiedad", "Comodato"} {
		if !strings.Contains(html, want) {
			t.Errorf("output missing option %q\n%s", want, html)
		}
	}
}

func TestRenderSelect_Optional_HasEmptyOption(t *testing.T) {
	e := &FormEngine{}
	f := Field{Name: "calidad", Title: "Calidad", JSONType: "string", Enum: []string{"A", "B"}}
	html := renderWidget(t, e, f, nil, "", false)
	if !strings.Contains(html, "— Select —") {
		t.Errorf("optional select should have the §5.3 empty option:\n%s", html)
	}
}

func TestRenderSelect_Source_ResolvesViaSourceResolver(t *testing.T) {
	e := &FormEngine{Sources: StaticSourceResolver{
		"entity:brands": {{Value: "1", Label: "Acme"}, {Value: "2", Label: "Globex"}},
	}}
	f := Field{Name: "marca_id", Title: "Marca", JSONType: "integer", XSeamInput: "select", XSeamSource: "entity:brands"}
	html := renderWidget(t, e, f, "1", "", false)
	if !strings.Contains(html, "— Unassigned —") {
		t.Errorf("FK select should use the §5.3 FK empty-option label:\n%s", html)
	}
	for _, want := range []string{"Acme", "Globex"} {
		if !strings.Contains(html, want) {
			t.Errorf("output missing resolved option %q\n%s", want, html)
		}
	}
}

func TestRenderSelect_Source_NoResolverConfigured(t *testing.T) {
	e := &FormEngine{} // Sources left nil
	f := Field{Name: "marca_id", Title: "Marca", JSONType: "integer", XSeamInput: "select", XSeamSource: "entity:brands"}
	html := renderWidget(t, e, f, nil, "", false)
	if !strings.Contains(html, "formengine:") {
		t.Errorf("expected a visible formengine error, not a silently broken select:\n%s", html)
	}
}

func TestRenderFieldRow_ErrorState(t *testing.T) {
	e := &FormEngine{}
	f := Field{Name: "email", Title: "Email", JSONType: "string", Format: "email"}
	html := renderWidget(t, e, f, "not-an-email", "formato de email inválido", false)
	if !strings.Contains(html, "border-red-500") {
		t.Errorf("expected error styling class on the input:\n%s", html)
	}
	if !strings.Contains(html, "formato de email inválido") {
		t.Errorf("expected the error message to be rendered:\n%s", html)
	}
}

func TestRenderFieldRow_UnsupportedWidgetDoesNotPanic(t *testing.T) {
	e := &FormEngine{}
	f := Field{Name: "mystery", Title: "Mystery", XSeamInput: "widget-that-will-never-exist"} // explicit bogus x-seam-input, not tied to whatever happens to be unimplemented at any given stage
	html := renderWidget(t, e, f, nil, "", false)
	if !strings.Contains(html, "not implemented") {
		t.Errorf("expected a visible not-implemented notice, got:\n%s", html)
	}
}

// TestRenderPassword_NeverEchoesValue is the security-critical property
// of the "password" widget (added 2026-08-03, MOD-8): unlike every
// other text-like widget (renderTextLike), which correctly echoes the
// current value back into value= for redisplay after a validation
// error, a password's value must never appear in rendered HTML under
// any circumstance. Passes a real, distinctive value deliberately, to
// prove it's actually absent from the output, not just untested.
func TestRenderPassword_NeverEchoesValue(t *testing.T) {
	e := &FormEngine{}
	f := Field{Name: "new_password", Title: "New Password", JSONType: "string", XSeamInput: "password"}
	html := renderWidget(t, e, f, "hunter2-the-actual-secret", "", false)
	if strings.Contains(html, "hunter2") {
		t.Fatalf("password widget echoed the submitted value into rendered HTML -- this must never happen:\n%s", html)
	}
	if !strings.Contains(html, `type="password"`) {
		t.Errorf("expected type=\"password\" on the input:\n%s", html)
	}
	if strings.Contains(html, "value=") {
		t.Errorf("expected no value= attribute at all on a password input:\n%s", html)
	}
}

func TestRenderPassword_AutocompleteHintApplied(t *testing.T) {
	e := &FormEngine{}
	f := Field{Name: "current_password", Title: "Current Password", JSONType: "string", XSeamInput: "password", XSeamAutocomplete: "current-password"}
	html := renderWidget(t, e, f, nil, "", false)
	if !strings.Contains(html, `autocomplete="current-password"`) {
		t.Errorf("expected autocomplete=\"current-password\" to be applied:\n%s", html)
	}
}

func TestRenderPassword_NoAutocompleteAttrWhenUnset(t *testing.T) {
	// Confirms the field-level doc comment's claim: empty means no
	// attribute at all, not autocomplete="off".
	e := &FormEngine{}
	f := Field{Name: "some_password", Title: "Some Password", JSONType: "string", XSeamInput: "password"}
	html := renderWidget(t, e, f, nil, "", false)
	if strings.Contains(html, "autocomplete") {
		t.Errorf("expected no autocomplete attribute when XSeamAutocomplete is unset:\n%s", html)
	}
}

func TestRenderPassword_RequiredApplied(t *testing.T) {
	e := &FormEngine{}
	f := Field{Name: "new_password", Title: "New Password", JSONType: "string", XSeamInput: "password", Required: true}
	html := renderWidget(t, e, f, nil, "", false)
	if !strings.Contains(html, "required") {
		t.Errorf("expected the required attribute:\n%s", html)
	}
}

// TestRenderSignature_RendersCanvasAndHiddenField confirms the basic
// structure: a canvas to draw on, a hidden input carrying the actual
// submitted value, and the clear button -- built 2026-08-04 as T-10's
// first concrete widget (signature capture), not just a priority
// re-triage.
// TestRenderDecimal_TextTypeNotNumber confirms the deliberate choice
// (widgets.go's own renderDecimal comment): a decimal field renders as
// type="text", never type="number", so the browser can't silently
// coerce or reformat a precise decimal value.
func TestRenderDecimal_TextTypeNotNumber(t *testing.T) {
	e := &FormEngine{}
	f := Field{Name: "price", Title: "Price", JSONType: "string", Format: "decimal", DecimalScale: 2, Required: true}
	html := renderWidget(t, e, f, "10.99", "", false)
	for _, want := range []string{`type="text"`, `name="price"`, `value="10.99"`, `inputmode="decimal"`, `required`} {
		if !strings.Contains(html, want) {
			t.Errorf("output missing %q\n%s", want, html)
		}
	}
	if strings.Contains(html, `type="number"`) {
		t.Errorf("decimal widget must never render type=\"number\"\n%s", html)
	}
}

// TestRenderDecimal_PatternMatchesScale confirms the generated pattern
// attribute tracks f.DecimalScale rather than being hardcoded.
func TestRenderDecimal_PatternMatchesScale(t *testing.T) {
	e := &FormEngine{}
	f := Field{Name: "qty", JSONType: "string", Format: "decimal", DecimalScale: 3}
	html := renderWidget(t, e, f, nil, "", false)
	want := `pattern="-?\d+(\.\d{1,3})?"`
	if !strings.Contains(html, want) {
		t.Errorf("output missing %q\n%s", want, html)
	}
}

// TestRenderMultiSelect_EnumChecksSelectedOnly confirms checkbox
// name/value shape (f.Name+"[]", per-option) and that only currently-
// selected values render checked.
func TestRenderMultiSelect_EnumChecksSelectedOnly(t *testing.T) {
	e := &FormEngine{}
	f := Field{Name: "tags", Title: "Tags", JSONType: "array", XSeamInput: "multi-select", Enum: []string{"a", "b", "c"}}
	html := renderWidget(t, e, f, []interface{}{"a", "c"}, "", false)
	for _, want := range []string{`name="tags[]"`, `value="a"`, `value="b"`, `value="c"`} {
		if !strings.Contains(html, want) {
			t.Errorf("output missing %q\n%s", want, html)
		}
	}
	// Exactly two checked attributes (for a and c, not b). Counting the
	// full `checked="checked"` attribute, not the bare substring
	// "checked" -- minty renders it as checked="checked", which
	// contains the word "checked" twice per occurrence and would
	// silently double-count.
	if got := strings.Count(html, `checked="checked"`); got != 2 {
		t.Errorf("checked count = %d, want 2\n%s", got, html)
	}
}

// TestRenderMultiSelect_NoOptionsSourceIsAnError confirms the same
// inline-error convention renderSelectWithExtraAttrs uses for a
// misconfigured field (neither enum nor x-seam-source) -- a silently
// empty control would be worse than a diagnosable one.
func TestRenderMultiSelect_NoOptionsSourceIsAnError(t *testing.T) {
	e := &FormEngine{}
	f := Field{Name: "broken", JSONType: "array", XSeamInput: "multi-select"}
	html := renderWidget(t, e, f, nil, "", false)
	if !strings.Contains(html, "no enum or x-seam-source") {
		t.Errorf("expected inline misconfiguration error, got:\n%s", html)
	}
}

// TestRenderBarcode_DefaultsToEAN13 confirms the default symbology and
// that both the value input and the SVG preview element render.
func TestRenderBarcode_DefaultsToEAN13(t *testing.T) {
	e := &FormEngine{}
	f := Field{Name: "product_code", Title: "Product Code", JSONType: "string", XSeamInput: "barcode"}
	html := renderWidget(t, e, f, "012345678905", "", false)
	for _, want := range []string{
		`name="product_code"`, `value="012345678905"`, `type="text"`,
		`<svg id="barcode-svg-product_code">`,
		`/static/vendor/jsbarcode-ean-upc@3.12.3.min.js`,
		`data-seam-widget="barcode"`, `data-format="EAN13"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("output missing %q\n%s", want, html)
		}
	}
}

// TestRenderBarcode_ExplicitFormat confirms x-seam-barcode-format
// overrides the EAN13 default.
func TestRenderBarcode_ExplicitFormat(t *testing.T) {
	e := &FormEngine{}
	f := Field{Name: "upc_code", JSONType: "string", XSeamInput: "barcode", XSeamBarcodeFormat: "UPC"}
	html := renderWidget(t, e, f, "", "", false)
	if !strings.Contains(html, `data-format="UPC"`) {
		t.Errorf("output missing UPC format selection\n%s", html)
	}
}

// TestRenderGPS_RendersMapAndInput confirms the field input, map
// container, and Leaflet vendor includes all render, and that
// L.Icon.Default is explicitly configured (this widget is the first
// real consumer of the vendored marker icons -- AssetGeoPage only uses
// L.circleMarker, which needs no icon image at all).
func TestRenderGPS_RendersMapAndInput(t *testing.T) {
	e := &FormEngine{}
	f := Field{Name: "gps_location", Title: "Location", JSONType: "string", XSeamInput: "gps"}
	html := renderWidget(t, e, f, "-34.9011,-56.1645", "", false)
	for _, want := range []string{
		`name="gps_location"`, `value="-34.9011,-56.1645"`, `type="text"`,
		`id="gps-map-gps_location"`,
		`/static/vendor/leaflet@1.9.4.js`,
		`/static/vendor/leaflet@1.9.4.css`,
		`data-seam-widget="gps"`, `data-gps-map="1"`, `data-gps-locate="1"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("output missing %q\n%s", want, html)
		}
	}
}

// TestRenderGPS_ButtonIsScopedNotGlobal confirms the "use my location"
// button is wired via a field-scoped element ID and addEventListener
// inside the field's own closure, not a shared window-level namespace
// function -- the specific bug class T-57 documents for
// visibilityScript, deliberately avoided here from the start.
func TestRenderGPS_ButtonIsScopedNotGlobal(t *testing.T) {
	e := &FormEngine{}
	f := Field{Name: "gps_location", JSONType: "string", XSeamInput: "gps"}
	html := renderWidget(t, e, f, "", "", false)
	if !strings.Contains(html, `id="gps-locate-gps_location"`) {
		t.Errorf("expected a field-scoped button id\n%s", html)
	}
	if strings.Contains(html, "window.SeamGPS") {
		t.Errorf("must not use a shared window-level namespace for per-field state\n%s", html)
	}
}

// TestRenderImage_CreateMode_ShowsSaveFirstMessage confirms the
// create-mode (rc.IsEdit == false) fallback: an informational message,
// no vendor script/link tags, no custom element at all -- matching
// Details/Embeds' own established edit-mode-only precedent
// (renderer.go §3.7), but as an explicit message rather than silent
// omission, since Image is field-level (a bare, empty control under a
// label would look broken) where Details/Embeds are tab-level (can
// simply not exist).
func TestRenderImage_CreateMode_ShowsSaveFirstMessage(t *testing.T) {
	e := &FormEngine{}
	f := Field{Name: "photos", Title: "Photos", JSONType: "string", XSeamInput: "image"}
	values := map[string]interface{}{"photos": nil}
	h := func(b *mi.Builder) mi.Node {
		return renderFieldRow(b, context.Background(), e, f, values, "", false, RenderContext{IsEdit: false})
	}
	got := mi.RenderToString(h)
	if !strings.Contains(got, "Save the record first to add photos") {
		t.Errorf("expected the save-first message, got:\n%s", got)
	}
	for _, unwanted := range []string{"dropzone@5.9.3", "seam-file-upload", "<script"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("create mode must not emit %q\n%s", unwanted, got)
		}
	}
}

// TestRenderImage_EditMode_RendersCustomElement confirms the edit-mode
// path: vendor includes present, and the <seam-file-upload> element
// carries the entity's own owner-type/owner-id/owner-field/csrf-token,
// sourced from RenderContext (EntityType/EntityID/CSRFToken) and the
// field's own name -- not anything client-supplied.
func TestRenderImage_EditMode_RendersCustomElement(t *testing.T) {
	e := &FormEngine{}
	f := Field{Name: "photos", Title: "Photos", JSONType: "string", XSeamInput: "image"}
	values := map[string]interface{}{"photos": nil}
	rc := RenderContext{IsEdit: true, EntityID: 42, EntityType: "asset", CSRFToken: "tok123"}
	h := func(b *mi.Builder) mi.Node {
		return renderFieldRow(b, context.Background(), e, f, values, "", false, rc)
	}
	got := mi.RenderToString(h)
	for _, want := range []string{
		`/static/vendor/dropzone@5.9.3.min.css`,
		`/static/vendor/dropzone@5.9.3.min.js`,
		`type="module"`,
		`/static/js/file-upload.js`,
		`<seam-file-upload owner-type="asset" owner-id="42" owner-field="photos" csrf-token="tok123"`,
		`label-add="Add photos" label-hint="or drag them here"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n%s", want, got)
		}
	}
}

// TestRenderImage_EscapesAdversarialCSRFToken is this widget's
// T-63/T-64-shaped adversarial test, matching
// TestRenderSignature_ExistingValueGoesThroughNormalAttributeEscaping's
// own established pattern for any value interpolated into a raw HTML
// string via mi.Raw: a token containing a double-quote must not be
// able to break out of the csrf-token="..." attribute and inject a
// new one.
func TestRenderImage_EscapesAdversarialCSRFToken(t *testing.T) {
	e := &FormEngine{}
	f := Field{Name: "photos", JSONType: "string", XSeamInput: "image"}
	values := map[string]interface{}{"photos": nil}
	adversarial := `tok" onmouseover="alert(1)`
	rc := RenderContext{IsEdit: true, EntityID: 1, EntityType: "asset", CSRFToken: adversarial}
	h := func(b *mi.Builder) mi.Node {
		return renderFieldRow(b, context.Background(), e, f, values, "", false, rc)
	}
	got := mi.RenderToString(h)
	if strings.Contains(got, `onmouseover="alert(1)"`) {
		t.Fatalf("adversarial CSRF token broke out of its attribute into injected markup:\n%s", got)
	}
	if !strings.Contains(got, "&#34;") && !strings.Contains(got, "&quot;") {
		t.Errorf("expected the double-quote to be HTML-escaped, got:\n%s", got)
	}
}

func TestRenderSignature_RendersCanvasAndHiddenField(t *testing.T) {
	e := &FormEngine{}
	f := Field{Name: "sign_off", Title: "Signature", JSONType: "string", XSeamInput: "signature"}
	html := renderWidget(t, e, f, nil, "", false)
	if !strings.Contains(html, `<canvas`) {
		t.Errorf("expected a <canvas> element:\n%s", html)
	}
	if !strings.Contains(html, `type="hidden"`) || !strings.Contains(html, `name="sign_off"`) {
		t.Errorf("expected a hidden input named sign_off carrying the actual value:\n%s", html)
	}
	if !strings.Contains(html, `data-seam-widget="signature"`) {
		t.Errorf("expected the signature widget marker SeamWidgets initialises from:\n%s", html)
	}
}

// TestRenderSignature_ExistingValueGoesThroughNormalAttributeEscaping
// is the adversarial one, guarding the exact lesson T-63/T-64 (this
// session) established: an existing value must go through minty's
// normal, automatic attribute escaping (value="..." on the hidden
// input), never be interpolated into the accompanying <script> block
// as a string. A malicious "existing signature" value (script-tag-
// breakout shaped, since a real signature is xolu-stored data,
// editable indirectly by anything that can write to that field) must
// not be able to break out of the <script> tag the way the Map page's
// asset names once could.
func TestRenderSignature_ExistingValueGoesThroughNormalAttributeEscaping(t *testing.T) {
	e := &FormEngine{}
	f := Field{Name: "sign_off", Title: "Signature", JSONType: "string", XSeamInput: "signature"}
	malicious := `data:image/png;base64,x</script><script>alert(document.cookie)</script>`
	html := renderWidget(t, e, f, malicious, "", false)

	if strings.Contains(html, "</script><script>alert") {
		t.Fatalf("the malicious existing value's literal script-tag-breakout sequence appears unescaped in the output:\n%s", html)
	}
	// html.EscapeString (minty's own attribute-escaping, confirmed
	// directly from its source during the T-63/T-64 audit) turns "<"
	// into "&lt;" -- confirming the escaped form is present (not just
	// the raw form's absence) proves real escaping happened, not the
	// value silently vanishing.
	if !strings.Contains(html, "&lt;/script&gt;&lt;script&gt;") {
		t.Errorf("expected the escaped form of the malicious value in the hidden input's value attribute -- got neither form, suggesting the value went missing:\n%s", html)
	}
}

// Two signature widgets on one page (a Work Order form with separate
// technician and customer sign-off fields) must not clear each other: the Clear
// button finds its own widget by DOM position (closest wrapper), never by a name
// or an id, so it needs no per-field script and works in repeatable rows too.
func TestRenderSignature_FieldNameNotOtherFieldsMixedUp(t *testing.T) {
	e := &FormEngine{}
	a := renderWidget(t, e, Field{Name: "customer_signature", Title: "Customer", JSONType: "string", XSeamInput: "signature"}, nil, "", false)
	b := renderWidget(t, e, Field{Name: "tech_signature", Title: "Technician", JSONType: "string", XSeamInput: "signature"}, nil, "", false)
	for name, html := range map[string]string{"customer_signature": a, "tech_signature": b} {
		if !strings.Contains(html, `onclick="SeamWidgets.clearSignature(this)"`) {
			t.Errorf("%s: the Clear button must locate its own widget from the element:\n%s", name, html)
		}
		if strings.Contains(html, "<script") || strings.Contains(html, "fieldName") {
			t.Errorf("%s: the widget must be inert markup, with no per-field script:\n%s", name, html)
		}
		if strings.Count(html, `data-seam-widget="signature"`) != 1 {
			t.Errorf("%s: expected exactly one widget root:\n%s", name, html)
		}
	}
}

func TestRenderAnyPicker_RequiredApplied(t *testing.T) {
	e := &FormEngine{}
	fReq := Field{Name: "approver_id", Title: "Approver", JSONType: "integer", XSeamInput: "any-picker", XSeamSource: "entity:users", Required: true}
	htmlReq := renderWidget(t, e, fReq, nil, "", false)
	searchReqTag := htmlReq[strings.Index(htmlReq, `id="picker-search-approver_id"`):]
	if end := strings.Index(searchReqTag, ">"); end > 0 {
		searchReqTag = searchReqTag[:end]
	}
	if !strings.Contains(searchReqTag, "required") {
		t.Errorf("expected required attribute on any-picker search input:\n%s", searchReqTag)
	}

	fOpt := Field{Name: "approver_id", Title: "Approver", JSONType: "integer", XSeamInput: "any-picker", XSeamSource: "entity:users", Required: false}
	htmlOpt := renderWidget(t, e, fOpt, nil, "", false)
	searchOptTag := htmlOpt[strings.Index(htmlOpt, `id="picker-search-approver_id"`):]
	if end := strings.Index(searchOptTag, ">"); end > 0 {
		searchOptTag = searchOptTag[:end]
	}
	if strings.Contains(searchOptTag, "required") {
		t.Errorf("optional any-picker must not have required attribute:\n%s", searchOptTag)
	}
}

func TestRenderIconPicker_RequiredApplied(t *testing.T) {
	e := &FormEngine{}
	fReq := Field{Name: "icon", Title: "Icon", JSONType: "string", XSeamInput: "icon-picker", Required: true}
	htmlReq := renderWidget(t, e, fReq, nil, "", false)
	searchReqTag := htmlReq[strings.Index(htmlReq, `id="picker-search-icon"`):]
	if end := strings.Index(searchReqTag, ">"); end > 0 {
		searchReqTag = searchReqTag[:end]
	}
	if !strings.Contains(searchReqTag, "required") {
		t.Errorf("expected required attribute on icon-picker search input:\n%s", searchReqTag)
	}

	fOpt := Field{Name: "icon", Title: "Icon", JSONType: "string", XSeamInput: "icon-picker", Required: false}
	htmlOpt := renderWidget(t, e, fOpt, nil, "", false)
	searchOptTag := htmlOpt[strings.Index(htmlOpt, `id="picker-search-icon"`):]
	if end := strings.Index(searchOptTag, ">"); end > 0 {
		searchOptTag = searchOptTag[:end]
	}
	if strings.Contains(searchOptTag, "required") {
		t.Errorf("optional icon-picker must not have required attribute:\n%s", searchOptTag)
	}
}

func TestRenderRepeatable_RequiredAndItemFields(t *testing.T) {
	schema := []byte(`{
		"type": "object",
		"properties": {
			"tags": {
				"type": "array",
				"title": "Tags",
				"x-seam-input": "repeatable",
				"x-seam-layout": "form",
				"items": {
					"type": "object",
					"properties": {
						"tag": {"type": "string", "title": "Tag"}
					},
					"required": ["tag"]
				}
			}
		},
		"required": ["tags"]
	}`)
	form, err := ParseSchema(schema)
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	if !form.Fields[0].Required || form.Fields[0].MinItems != 1 {
		t.Errorf("expected required repeatable to have MinItems=1, got %+v", form.Fields[0])
	}
	if len(form.Fields[0].Items) != 1 || !form.Fields[0].Items[0].Required {
		t.Errorf("expected item field 'tag' to have Required=true")
	}

	e := &FormEngine{}
	html := mi.RenderToString(e.Render(context.Background(), schema, nil, nil, RenderContext{}))
	if !strings.Contains(html, `data-min="1"`) {
		t.Errorf("expected data-min=\"1\" in repeatable container:\n%s", html)
	}
	if !strings.Contains(html, "Tag *") {
		t.Errorf("expected 'Tag *' label in form layout cell:\n%s", html)
	}
}
