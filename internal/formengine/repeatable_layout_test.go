package formengine

import (
	"regexp"
	"strings"
	"testing"
)

func layoutField(t *testing.T, layout string) Field {
	t.Helper()
	attr := ""
	if layout != "" {
		attr = `, "x-seam-layout": "` + layout + `"`
	}
	form, err := ParseSchema([]byte(`{"type":"object","properties":{"items":{"type":"array","title":"Items","x-seam-input":"repeatable"` + attr + `,
	  "items":{"type":"object","properties":{
	    "name":{"type":"string","title":"Name","x-seam-input":"text"},
	    "kind":{"type":"string","title":"Kind","x-seam-input":"select","enum":["a","b"]},
	    "required":{"type":"boolean","title":"Required","x-seam-input":"checkbox"}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	return form.Fields[0]
}

// x-seam-layout: "row" (default) puts every cell on one line, as before;
// "form" makes each row a card with its cells under their labels.
func TestRepeatableLayout_RowIsTheDefaultAndAddsNoWrappers(t *testing.T) {
	for _, layout := range []string{"", "row", "unknown"} {
		html := renderRepeatableHTML(layoutField(t, layout), nil)
		if strings.Contains(html, "data-layout") || strings.Contains(html, "fe-cell") {
			t.Errorf("layout %q must render the default one-line rows:\n%s", layout, html)
		}
	}
}

func TestRepeatableLayout_FormLabelsEachCellAndMarksTheRows(t *testing.T) {
	flat := renderRepeatableHTML(layoutField(t, "form"), nil)
	if !strings.Contains(flat, `data-layout="form"`) {
		t.Errorf("the rows container must carry data-layout=form:\n%s", flat)
	}
	// Name and Kind get a label; the checkbox keeps its own title text.
	for _, want := range []string{`<span class="fe-cell-label">Name</span>`, `<span class="fe-cell-label">Kind</span>`, ">Required<"} {
		if !strings.Contains(flat, want) {
			t.Errorf("missing %s:\n%s", want, flat)
		}
	}
	if strings.Contains(flat, `<span class="fe-cell-label">Required</span>`) {
		t.Errorf("a checkbox already shows its title; it needs no extra label")
	}

	// Indexed rows (a nested repeatable forces indexed naming) honour it too,
	// and the nested repeatable keeps its own layout (row by default).
	idx := renderRepeatableHTML(blocksSchemaWithLayout(t, "form"), nil)
	if strings.Count(idx, `data-layout="form"`) < 1 { // the outer repeatable's rows container
		t.Errorf("the indexed rows container must carry data-layout=form:\n%s", idx)
	}
	if !strings.Contains(idx, `<span class="fe-cell-label">Block name</span>`) {
		t.Errorf("indexed form rows must label their cells:\n%s", idx)
	}
}

// Add and remove are the shared icon buttons, in both naming modes.
func TestRepeatableButtons_AreIconButtonsInBothModes(t *testing.T) {
	for name, html := range map[string]string{
		"flat":    renderRepeatableHTML(layoutField(t, ""), nil),
		"indexed": renderRepeatableHTML(blocksSchema(t), nil),
	} {
		for _, want := range []string{`class="fe-btn fe-add"`, ">add<", `class="fe-remove"`, ">remove_circle_outline<"} {
			if !strings.Contains(html, want) {
				t.Errorf("%s: missing %s", name, want)
			}
		}
	}
}

func blocksSchemaWithLayout(t *testing.T, layout string) Field {
	t.Helper()
	f := blocksSchema(t)
	f.XSeamLayout = layout
	return f
}

// Edit fields set their own text colour: black text on a dark background came
// from inheriting nothing.
func TestInputClass_SetsExplicitTextColoursForBothThemes(t *testing.T) {
	for _, hasErr := range []bool{false, true} {
		c := inputClass(hasErr)
		if !strings.Contains(c, "text-gray-900") || !strings.Contains(c, "dark:text-white") {
			t.Errorf("inputClass(%v) = %q must set light and dark text colours", hasErr, c)
		}
	}
}

func TestAnyPicker_BrowseStyles(t *testing.T) {
	f := anyPickerField("users")
	btn := renderWidgetHTML(renderAnyPicker, f, nil)
	if !strings.Contains(btn, `class="fe-btn"`) || !strings.Contains(btn, ">manage_search<") ||
		!strings.Contains(btn, "Browse") || strings.Contains(btn, "hover:underline") {
		t.Errorf("default Browse must be an outlined button with an icon and the label:\n%s", btn)
	}
	f.XSeamBrowse = "icon"
	icon := renderWidgetHTML(renderAnyPicker, f, nil)
	if !strings.Contains(icon, `class="fe-btn fe-btn--icon"`) || !strings.Contains(icon, `aria-label="Browse`) ||
		strings.Contains(regexp.MustCompile(`(?s)<button.*?</button>`).FindString(icon), ">Browse") {
		t.Errorf("icon Browse must be icon-only with an accessible name:\n%s", icon)
	}
	f.XSeamBrowse = "link"
	link := renderWidgetHTML(renderAnyPicker, f, nil)
	if !strings.Contains(link, "hover:underline") || strings.Contains(link, "fe-btn") {
		t.Errorf("link Browse keeps the original text link:\n%s", link)
	}
	f.XSeamBrowse = "bogus"
	if !strings.Contains(renderWidgetHTML(renderAnyPicker, f, nil), `class="fe-btn"`) {
		t.Errorf("an unknown x-seam-browse falls back to the button")
	}
	// The picker row holds the box and the button side by side.
	if !strings.Contains(btn, `class="fe-picker-row"`) {
		t.Errorf("missing the picker row")
	}
}

func TestPickers_HighlightModeIsConfigurable(t *testing.T) {
	for mode, want := range map[string]string{"": "color", "color": "color", "background": "background", "none": "none", "weird": "color"} {
		f := anyPickerField("")
		f.XSeamHighlight = mode
		if html := renderWidgetHTML(renderAnyPicker, f, nil); !strings.Contains(html, `data-hl="`+want+`"`) {
			t.Errorf("any-picker mode %q: want data-hl=%q:\n%s", mode, want, html)
		}
		fi := Field{Name: "icon", XSeamInput: "icon-picker", XSeamHighlight: mode}
		if html := renderWidgetHTML(renderIconPicker, fi, nil); !strings.Contains(html, `data-hl="`+want+`"`) {
			t.Errorf("icon-picker mode %q: want data-hl=%q", mode, want)
		}
		fa := Field{Name: "p", XSeamInput: "asset-picker", XSeamSource: "entity:assets", XSeamHighlight: mode}
		if html := renderWidgetHTML(renderAssetPicker, fa, nil); !strings.Contains(html, `data-hl="`+want+`"`) {
			t.Errorf("asset-picker mode %q: want data-hl=%q", mode, want)
		}
	}
}

// The asset picker highlights again, safely: the matched part is built with
// DOM nodes by SeamHighlight, never with innerHTML, and "none" keeps plain text.
func TestAssetPicker_HighlightsThroughSeamHighlightNotInnerHTML(t *testing.T) {
	html := renderWidgetHTML(renderAssetPicker, Field{Name: "p", XSeamInput: "asset-picker", XSeamSource: "entity:assets"}, nil)
	// the markup is the inert SeamPicker root; the highlighting code itself
	// (SeamHighlight, highlight: false) is asserted on app.js in widgets_js_test.go
	if !strings.Contains(html, `data-seam-picker="1"`) || strings.Contains(html, "<script>") {
		t.Errorf("asset picker must be inert SeamPicker markup:\n%s", html)
	}
	if strings.Contains(html, "innerHTML") {
		t.Errorf("a result label must never be written as HTML")
	}
}

func TestParseSchema_ReadsLayoutBrowseAndHighlight(t *testing.T) {
	form, err := ParseSchema([]byte(`{"type":"object","properties":{
	  "a":{"type":"integer","x-seam-input":"any-picker","x-seam-source":"entity:users","x-seam-selector":"users","x-seam-browse":"icon","x-seam-highlight":"background"},
	  "r":{"type":"array","x-seam-input":"repeatable","x-seam-layout":"form","items":{"type":"object","properties":{"n":{"type":"string"}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Field{}
	for _, f := range form.Fields {
		got[f.Name] = f
	}
	if got["a"].XSeamBrowse != "icon" || got["a"].XSeamHighlight != "background" || got["r"].XSeamLayout != "form" {
		t.Errorf("parsed fields: %+v", got)
	}
}
