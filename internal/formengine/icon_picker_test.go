package formengine

import (
	"strings"
	"testing"
)

func TestIconPicker_RegisteredAndRendersInertMarkupForSeamPicker(t *testing.T) {
	if widgetRegistry["icon-picker"] == nil {
		t.Fatalf("icon-picker is not registered")
	}
	f := Field{Name: "icon", Title: "Icon", XSeamInput: "icon-picker"}
	html := renderWidgetHTML(renderIconPicker, f, "home")
	for _, want := range []string{
		`data-seam-picker="1"`, `data-endpoint="/picker/icons"`, `data-preview="icon"`,
		`type="hidden"`, `name="icon"`, `value="home"`, `id="field-icon"`,
		`type="text"`, `id="picker-search-icon"`, `placeholder="Search icons`,
		`data-icon-preview="1"`, ">home<", // the preview shows the current icon
		"SeamPicker.init(document)",
		`onclick="SeamPicker.toggleGrid(this)"`, `title="Browse icons"`, ">apps<", // the Browse grid button
		"fe-icon-preview",             // a large preview of the chosen icon (sized in app.css)
		`class="fe-btn fe-btn--icon"`, // the grid button is the shared icon button
	} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %s:\n%s", want, html)
		}
	}
	if strings.Contains(html, "getElementById") {
		t.Errorf("no per-instance id lookups")
	}
}

// With no icon chosen the preview is an empty element, which the stylesheet
// draws as a slot (.fe-icon-preview:empty): the place where the icon will be.
func TestIconPicker_WithoutAValueLeavesThePreviewEmptyForTheSlot(t *testing.T) {
	f := Field{Name: "icon", Title: "Icon", XSeamInput: "icon-picker"}
	for _, value := range []interface{}{"", nil} {
		html := renderWidgetHTML(renderIconPicker, f, value)
		if !strings.Contains(html, `data-icon-preview="1"></span>`) {
			t.Errorf("value %v: the preview must be an empty element, got: %s", value, html[strings.Index(html, "fe-icon-preview")-30:strings.Index(html, "fe-icon-preview")+120])
		}
	}
	if html := renderWidgetHTML(renderIconPicker, f, "home"); !strings.Contains(html, `data-icon-preview="1">home</span>`) {
		t.Errorf("with a value the preview holds the icon")
	}
}

func TestIconPicker_PlaceholderAndHostileValue(t *testing.T) {
	f := Field{Name: "icon", XSeamInput: "icon-picker", XSeamPlaceholder: "Pick an icon"}
	if html := renderWidgetHTML(renderIconPicker, f, nil); !strings.Contains(html, `placeholder="Pick an icon"`) {
		t.Errorf("x-seam-placeholder must be the placeholder:\n%s", html)
	}
	html := renderWidgetHTML(renderIconPicker, f, `x"><script>alert(1)</script>`)
	if strings.Contains(html, "<script>alert(1)</script>") {
		t.Errorf("a current value reached the output unescaped:\n%s", html)
	}
}

// Inside a repeatable row the picker is renamed like any other cell.
func TestIconPicker_InsideAnIndexedRowGetsIndexedNames(t *testing.T) {
	form, err := ParseSchema([]byte(`{"type":"object","properties":{"steps":{"type":"array","x-seam-input":"repeatable","x-seam-naming":"indexed",
	  "items":{"type":"object","properties":{"name":{"type":"string","title":"Name","x-seam-input":"text"},
	  "icon":{"type":"string","title":"Icon","x-seam-input":"icon-picker"}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	html := renderRepeatableHTML(form.Fields[0], nil)
	for _, want := range []string{`name="steps[0][icon]"`, `name="steps[__D0__][icon]"`, `data-endpoint="/picker/icons"`, `placeholder="Icon"`} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %s", want)
		}
	}
}
