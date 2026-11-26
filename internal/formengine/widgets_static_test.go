package formengine

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func readStatic(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile("../../web/static/" + rel)
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	return string(b)
}

// The GPS widget is an input plus a button that opens a dropdown panel, like the
// icon-picker's grid; the map lives inside the panel, which starts hidden, and the
// markup carries no script of its own.
func TestGPS_IsADropdownPanelWithNoInlineScript(t *testing.T) {
	html := renderWidgetHTML(renderGPS, Field{Name: "loc", XSeamInput: "gps"}, "-34.9,-56.1")
	for _, want := range []string{`data-seam-widget="gps"`, `data-gps-toggle="1"`, `aria-expanded="false"`, `data-gps-panel="1"`, `hidden="hidden"`, `data-gps-locate="1"`, `data-gps-close="1"`, `data-gps-map="1"`, `title="Pick on map"`, `data-panel="dropdown"`, `data-gps-expand="1"`, `aria-pressed="false"`} {
		if !strings.Contains(html, want) {
			t.Errorf("gps markup missing %s:\n%s", want, html)
		}
	}
	if strings.Contains(html, "<script>") {
		t.Errorf("gps must be inert markup, no inline script:\n%s", html)
	}
	if strings.Index(html, `data-gps-panel`) > strings.Index(html, `data-gps-map`) {
		t.Errorf("the map must sit inside the panel:\n%s", html)
	}
}

// asset-picker lists its first results as soon as it gets focus (a dropdown); the
// any-picker keeps its own behaviour (type to search, or Browse).
func TestAssetPicker_OpensOnFocusAnyPickerDoesNot(t *testing.T) {
	asset := renderWidgetHTML(renderAssetPicker, Field{Name: "a", XSeamInput: "asset-picker", XSeamSource: "entity:assets"}, nil)
	if !strings.Contains(asset, `data-open-on-focus="1"`) {
		t.Errorf("asset-picker must ask SeamPicker to open on focus:\n%s", asset)
	}
	anyp := renderWidgetHTML(renderAnyPicker, Field{Name: "p", XSeamInput: "any-picker", XSeamSource: "entity:assets"}, nil)
	if strings.Contains(anyp, "data-open-on-focus") {
		t.Errorf("any-picker must not change behaviour:\n%s", anyp)
	}
}

// x-seam-panel: wide makes the map panel start at the full width of the field; the default is
// the dropdown under the pin button. The attribute is read by the parser and offered by the designer.
func TestGPS_WidePanelOptionIsParsedAndRendered(t *testing.T) {
	form, err := ParseSchema([]byte(`{"type":"object","properties":{"loc":{"type":"string","x-seam-input":"gps","x-seam-panel":"wide"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(form.Fields) != 1 || form.Fields[0].XSeamPanel != "wide" {
		t.Fatalf("x-seam-panel not parsed: %+v", form.Fields)
	}
	if html := renderWidgetHTML(renderGPS, form.Fields[0], nil); !strings.Contains(html, `data-panel="wide"`) {
		t.Errorf("wide mode must reach the markup:\n%s", html)
	}
}

// The panel opens under the pin button aligned to its right edge; wide mode leaves the overlay
// and takes the full width of the field, in the page flow.
func TestAppCSS_GPSPanelRightAlignedAndWide(t *testing.T) {
	css := readStatic(t, "css/app.css")
	for name, re := range map[string]string{
		"right-aligned default": `\.fe-gps-panel \{[^}]*top: 100%;[^}]*right: 0;`,
		"left fallback":         `\.fe-gps-panel--left \{\s*right: auto;\s*left: 0;`,
		"wide in the flow":      `\.fe-gps-panel--wide \{\s*position: static;\s*width: auto;`,
	} {
		if !regexp.MustCompile(re).MatchString(css) {
			t.Errorf("app.css: missing the %s rule", name)
		}
	}
}

// The map pin is an inline SVG in a divIcon. Leaflet's own default marker builds its
// image URL from a detected path, which produced /static/vendor/images//static/vendor/
// images/marker-icon.png (404): no marker image file may be referenced at all.
func TestAppJS_PinIsInlineNotAnImageFile(t *testing.T) {
	js := readStatic(t, "js/app.js")
	if !strings.Contains(js, "L.divIcon") || !strings.Contains(js, "PIN_SVG") {
		t.Errorf("the pin must be an inline SVG divIcon")
	}
	for _, bad := range []string{"mergeOptions", "marker-icon", "marker-shadow"} {
		if strings.Contains(js, bad) {
			t.Errorf("app.js must not reference %q: the pin must not depend on an image path", bad)
		}
	}
}

// A barcode is drawn black on white (readable and scannable in any theme); an invalid
// value flags the input through aria-invalid and clears the previous drawing, because
// JsBarcode does not throw when a valid callback is given.
func TestAppJS_BarcodeInvalidStateAndWhiteBackground(t *testing.T) {
	js := readStatic(t, "js/app.js")
	for _, want := range []string{"background: '#ffffff'", "lineColor: '#000000'", "valid: function (ok) { setInvalid(!ok); if (!ok) svg.innerHTML = ''; }", "setAttribute('aria-invalid', 'true')"} {
		if !strings.Contains(js, want) {
			t.Errorf("app.js missing %q", want)
		}
	}
}

// The red border must beat dark:border-gray-600 and focus:border-indigo-500.
func TestAppCSS_InvalidRuleBeatsThemeAndFocus(t *testing.T) {
	css := readStatic(t, "css/app.css")
	if !regexp.MustCompile(`input\[aria-invalid="true"\]\s*\{[^}]*border-color:\s*#ef4444\s*!important`).MatchString(css) {
		t.Errorf("app.css needs the aria-invalid rule with !important")
	}
	if !strings.Contains(css, ".fe-gps-panel") || !strings.Contains(css, ".fe-gps-panel[hidden]") {
		t.Errorf("app.css needs the gps panel rules, including [hidden]")
	}
}

// Rows a repeatable adds go through SeamWidgets.init, since scripts inserted with
// innerHTML do not run.
func TestAppJS_RepeatableAddInitialisesWidgets(t *testing.T) {
	js := readStatic(t, "js/app.js")
	if !regexp.MustCompile(`(?s)function add\(btn\).*?SeamWidgets\.init\(row\).*?function remove`).MatchString(js) {
		t.Errorf("SeamRepeatable.add must call SeamWidgets.init(row)")
	}
}

// A form swapped in by htmx brings its autocomplete <script src> with it, and htmx:load does
// not wait for it: the picker must wait for the library, not give up (the related-asset box
// of the Assets form never initialised, and showed no stored label and no list).
func TestAppJS_PickerWaitsForTheAutocompleteLibrary(t *testing.T) {
	js := readStatic(t, "js/app.js")
	if !regexp.MustCompile(`(?s)function initOne\(root\).*?SeamWidgets\.need\('autoComplete'.*?initOne\(root\)`).MatchString(js) {
		t.Errorf("SeamPicker.initOne must load/wait for window.autoComplete and then initialise")
	}
	// htmx replaces the script elements of a swapped fragment, so a listener attached to an
	// element already in the page never fires: need() must not wait on one.
	if strings.Contains(js, "script[src=\"' + src") {
		t.Errorf("SeamWidgets.need must not wait on a <script> element already in the page")
	}
}

// The photo uploader: inside a modal Dropzone may load after the uploader asks for it, so it waits
// (the "Dropzone did not load" message on the first open of a form); a click on the icon or the
// label must reach the drop area, which is the only thing that opens Dropzone's file chooser; the
// thumbnails show a hand cursor and open the photo larger, and Escape closes only that.
func TestFileUploadJS_WaitsForDropzoneOpensChooserAndEnlargesPhotos(t *testing.T) {
	js := readStatic(t, "js/file-upload.js")
	for _, want := range []string{
		"SeamWidgets.need('Dropzone'", ".seam-dropzone > * { pointer-events: none; }", "cursor: pointer;",
		"seam-lightbox", "document.addEventListener('keydown', this._onPreviewKey, true)", "disconnectedCallback()",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("file-upload.js missing %q", want)
		}
	}
	if !strings.Contains(readStatic(t, "js/app.js"), "if (n.src && n.type !== 'module') n.async = false;") {
		t.Errorf("SeamModal must keep the order of the classic scripts it re-creates")
	}
}

// Two things the browser tests also exercise. The uploader forwards a click that lands on the icon,
// the label or the hint to the file chooser itself, so the button does not depend on a stylesheet
// (Dropzone only reacts to a click on the drop area). And a wide GPS panel inside a repeatable's
// row makes its cell take the whole row, in both row layouts, while it is open.
func TestUploaderForwardsClicksAndWideGPSSpansItsRow(t *testing.T) {
	up := readStatic(t, "js/file-upload.js")
	if !strings.Contains(up, "if (e.target !== dz && instance.hiddenFileInput) instance.hiddenFileInput.click();") {
		t.Errorf("file-upload.js must forward clicks from the icon, label and hint to the file chooser")
	}
	js, css := readStatic(t, "js/app.js"), readStatic(t, "css/app.css")
	for _, want := range []string{"function spanRow(on)", "el.classList.toggle('fe-gps-span', on)", "spanRow(wide && !panel.hidden);", "spanRow(false); }"} {
		if !strings.Contains(js, want) {
			t.Errorf("app.js missing %q", want)
		}
	}
	for _, want := range []string{`[data-layout="form"] > .repeatable-row > .fe-gps-span { grid-column: 1 / -1; }`, `.fe-gps-span { flex: 1 0 100% !important; }`, `[data-seam-widget="barcode"] svg:empty { display: none; }`} {
		if !strings.Contains(css, want) {
			t.Errorf("app.css missing %q", want)
		}
	}
}

// Pressing Save or Create from a tab that is not showing must not do nothing: a required field in
// a hidden tab cannot take focus, so the browser silently refuses to submit. The first invalid
// field's tab is opened and the message shown there. And the modal refreshes the page behind it
// when it closes after a record was created.
func TestAppJS_InvalidFieldInHiddenTabOpensItsTabAndModalRefreshesTheList(t *testing.T) {
	js := readStatic(t, "js/app.js")
	for _, want := range []string{"document.addEventListener('invalid'", "if (first !== el) return;", "btn.click();", "el.reportValidity()", "seam:record-created", "window.location.reload()"} {
		if !strings.Contains(js, want) {
			t.Errorf("app.js missing %q", want)
		}
	}
}

// SeamModal re-creates the <script> elements of a form loaded into it. Copying only their text
// turned <script src> into an empty script, so the photo uploader (a module) and every library a
// form brought never loaded inside the modal: all their attributes must be copied.
func TestAppJS_ModalRecreatesScriptsWithTheirAttributes(t *testing.T) {
	js := readStatic(t, "js/app.js")
	if !regexp.MustCompile(`(?s)function rerunScript\(s\).*?setAttribute\(s\.attributes\[i\]\.name, s\.attributes\[i\]\.value\)`).MatchString(js) {
		t.Errorf("SeamModal must copy every attribute of a script it re-creates")
	}
	if strings.Count(js, "forEach(rerunScript)") != 2 {
		t.Errorf("both modals (SeamModal and SeamModal2) must re-create scripts with rerunScript")
	}
	if strings.Contains(js, "n.textContent = s.textContent;\n        s.parentNode.replaceChild(n, s);\n      });") {
		t.Errorf("a text-only script copy is back")
	}
}

// The picker highlights the matched part with DOM nodes (SeamHighlight), never innerHTML.
func TestAppJS_PickerHighlightsThroughSeamHighlight(t *testing.T) {
	js := readStatic(t, "js/app.js")
	for _, want := range []string{"SeamHighlight.into(textHost, label, search.value)", "highlight: false"} {
		if !strings.Contains(js, want) {
			t.Errorf("SeamPicker missing %q", want)
		}
	}
}

// If a picker's search input is required, SeamPicker sets custom validity when
// the search input has text typed without a selected entity ID, clearing it
// upon valid selection.
func TestAppJS_PickerEnforcesRequiredSelection(t *testing.T) {
	js := readStatic(t, "js/app.js")
	for _, want := range []string{
		"function syncValidity()",
		"search.hasAttribute('required') || search.required",
		"!hidden.value && search.value",
		"search.setCustomValidity(SeamText('select-item', 'Please select an item from the list'))",
		"search.setCustomValidity('')",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("SeamPicker missing %q", want)
		}
	}
}

func TestAppJS_SeamConfirmSupportsCancelLabel(t *testing.T) {
	js := readStatic(t, "js/app.js")
	for _, want := range []string{
		"function _defaultCancel()",
		"var cancelLabel = opts.cancelLabel || _defaultCancel();",
		"cancelBtn.textContent = cancelLabel;",
		"data-i18n-cancel",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("SeamConfirm missing %q in app.js", want)
		}
	}
}
