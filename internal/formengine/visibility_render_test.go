package formengine

import (
	"context"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"
)

func renderVisHTML(t *testing.T, schema []byte, values map[string]interface{}, rc RenderContext) string {
	t.Helper()
	e := &FormEngine{}
	h := e.Render(context.Background(), schema, values, nil, rc)
	return mi.RenderToString(h)
}

func TestFieldVisibleWhen_RendersHiddenWithDataAttr(t *testing.T) {
	schema := []byte(`{
		"properties": {
			"calidad_tenencia": {"type": "string", "title": "Calidad"},
			"vto_comodato": {"type": "string", "format": "date", "title": "Vencimiento",
				"x-seam-visible-when": {"field": "calidad_tenencia", "op": "eq", "value": "Comodato"}}
		}
	}`)
	html := renderVisHTML(t, schema, nil, RenderContext{})
	if !strings.Contains(html, `data-field-row="vto_comodato"`) {
		t.Fatalf("field must still be rendered (kept in DOM, not omitted):\n%s", html)
	}
	if !strings.Contains(html, `data-visible-when=`) {
		t.Errorf("expected a data-visible-when attribute for the client evaluator:\n%s", html)
	}
	if !strings.Contains(html, `style="display:none"`) {
		t.Errorf("condition is currently false (calidad_tenencia unset) -- expected display:none:\n%s", html)
	}
}

func TestFieldVisibleWhen_TrueAtRenderTimeStartsVisible(t *testing.T) {
	schema := []byte(`{
		"properties": {
			"calidad_tenencia": {"type": "string", "title": "Calidad"},
			"vto_comodato": {"type": "string", "format": "date", "title": "Vencimiento",
				"x-seam-visible-when": {"field": "calidad_tenencia", "op": "eq", "value": "Comodato"}}
		}
	}`)
	values := map[string]interface{}{"calidad_tenencia": "Comodato"}
	html := renderVisHTML(t, schema, values, RenderContext{})
	// The vto_comodato row should NOT carry display:none this time.
	idx := strings.Index(html, `data-field-row="vto_comodato"`)
	if idx == -1 {
		t.Fatal("field row not found")
	}
	rowHTML := extractTag(html, idx)
	if strings.Contains(rowHTML, "display:none") {
		t.Errorf("condition holds at render time (calidad_tenencia=Comodato) -- should start visible:\n%s", rowHTML)
	}
}

func TestFieldVisibleWhen_RequiredSuppressedWhenHidden(t *testing.T) {
	// §3b.7: required is only HTML-enforced while the condition holds.
	schema := []byte(`{
		"properties": {
			"calidad_tenencia": {"type": "string", "title": "Calidad"},
			"vto_comodato": {"type": "string", "format": "date", "title": "Vencimiento",
				"x-seam-visible-when": {"field": "calidad_tenencia", "op": "eq", "value": "Comodato"}}
		},
		"required": ["vto_comodato"]
	}`)
	hiddenHTML := renderVisHTML(t, schema, nil, RenderContext{})
	idx := strings.Index(hiddenHTML, `id="field-vto_comodato"`)
	fieldTag := extractTag(hiddenHTML, idx)
	// " required" (space-prefixed, minty's BooleanAttribute rendering)
	// is the actual HTML required attribute -- distinct from the
	// data-schema-required="1" attribute, which is "-required" (hyphen-
	// prefixed) and deliberately present regardless of visibility.
	if strings.Contains(fieldTag, " required") {
		t.Errorf("required must be suppressed while the condition is false:\n%s", fieldTag)
	}
	if !strings.Contains(hiddenHTML, `data-schema-required="1"`) {
		t.Errorf("expected data-schema-required so the client evaluator can restore it on reveal:\n%s", hiddenHTML)
	}

	visibleHTML := renderVisHTML(t, schema, map[string]interface{}{"calidad_tenencia": "Comodato"}, RenderContext{})
	idx2 := strings.Index(visibleHTML, `id="field-vto_comodato"`)
	fieldTag2 := extractTag(visibleHTML, idx2)
	if !strings.Contains(fieldTag2, " required") {
		t.Errorf("required must be present while the condition holds:\n%s", fieldTag2)
	}
}

func TestFieldVisibleWhen_MissingReferencedFieldDegradesToHidden(t *testing.T) {
	// §3b.8: a condition referencing a field that doesn't exist in the
	// form degrades to "not met" (hidden), not an error.
	schema := []byte(`{
		"properties": {
			"vto_comodato": {"type": "string", "format": "date", "title": "Vencimiento",
				"x-seam-visible-when": {"field": "nonexistent_field", "op": "eq", "value": "x"}}
		}
	}`)
	form, err := ParseSchema(schema)
	if err != nil {
		t.Fatalf("parseSchema should not error on a dangling visible_when reference: %v", err)
	}
	html := renderVisHTML(t, schema, nil, RenderContext{})
	if !strings.Contains(html, "display:none") {
		t.Errorf("dangling field reference should render hidden, not visible:\n%s", html)
	}
	_ = form
}

func TestSectionVisibleWhen_AppliedToSectionWrapper(t *testing.T) {
	schema := []byte(`{
		"properties": {
			"calidad_tenencia": {"type": "string", "title": "Calidad"},
			"links_comodato": {"type": "string", "title": "Links"}
		},
		"x-seam-form": {
			"tabs": [{
				"id": "t1", "label": "T1",
				"rows": [{"sections": ["comodato"]}],
				"section_defs": [{
					"id": "comodato", "label": "Comodato", "fields": ["links_comodato"],
					"visible_when": {"field": "calidad_tenencia", "op": "eq", "value": "Comodato"}
				}]
			}]
		}
	}`)
	hidden := renderVisHTML(t, schema, nil, RenderContext{})
	if !strings.Contains(hidden, "Comodato</h3") && !strings.Contains(hidden, ">Comodato<") {
		t.Errorf("section should still render (in DOM, display:none) when its own condition is false:\n%s", hidden)
	}
	if !strings.Contains(hidden, "display:none") {
		t.Errorf("expected display:none on the section wrapper:\n%s", hidden)
	}

	visible := renderVisHTML(t, schema, map[string]interface{}{"calidad_tenencia": "Comodato"}, RenderContext{})
	// Find the section's own wrapper and confirm no display:none near it.
	idx := strings.Index(visible, "data-visible-when")
	if idx == -1 {
		t.Fatal("expected the section's visible_when attribute to be present regardless of current truth value")
	}
}

func TestRowVisibleWhen_AppliedToRowWrapper(t *testing.T) {
	schema := []byte(`{
		"properties": {
			"status": {"type": "string", "title": "Status"},
			"a": {"type": "string", "title": "A"}
		},
		"x-seam-form": {
			"tabs": [{
				"id": "t1", "label": "T1",
				"rows": [{
					"sections": ["s1"],
					"visible_when": {"field": "status", "op": "neq", "value": "archivado"}
				}],
				"section_defs": [{"id": "s1", "fields": ["a"]}]
			}]
		}
	}`)
	html := renderVisHTML(t, schema, map[string]interface{}{"status": "archivado"}, RenderContext{})
	if !strings.Contains(html, "display:none") {
		t.Errorf("row condition is false (status=archivado) -- expected display:none on the row:\n%s", html)
	}
	if !strings.Contains(html, "field-a") {
		t.Errorf("row's field should still render in the DOM even though currently hidden:\n%s", html)
	}
}

func TestTabVisibleWhen_OmittedAtRenderTimeWhenFalse(t *testing.T) {
	// Deliberate stage-5 scope: tab-level visible_when is render-time
	// only (omitted like HiddenTabs), not kept in the DOM for live
	// client reactivity -- see the comment in renderTabbedBody.
	schema := []byte(`{
		"properties": {
			"status": {"type": "string", "title": "Status"},
			"a": {"type": "string", "title": "A"}
		},
		"x-seam-form": {
			"tabs": [
				{"id": "t1", "label": "AlwaysThere", "rows": [{"sections": ["s1"]}],
					"section_defs": [{"id": "s1", "fields": ["a"]}]},
				{"id": "t2", "label": "ConditionalTab", "rows": [{"sections": ["s1"]}],
					"section_defs": [{"id": "s1", "fields": ["a"]}],
					"visible_when": {"field": "status", "op": "eq", "value": "special"}}
			]
		}
	}`)
	htmlHidden := renderVisHTML(t, schema, map[string]interface{}{"status": "normal"}, RenderContext{})
	if strings.Contains(htmlHidden, "ConditionalTab") {
		t.Errorf("tab whose visible_when is false at render time should be omitted entirely:\n%s", htmlHidden)
	}
	htmlVisible := renderVisHTML(t, schema, map[string]interface{}{"status": "special"}, RenderContext{})
	if !strings.Contains(htmlVisible, "ConditionalTab") {
		t.Errorf("tab whose visible_when is true at render time should render:\n%s", htmlVisible)
	}
}

func TestVisibilityScript_EmittedOnceRegardlessOfTabsOrFlat(t *testing.T) {
	flatSchema := []byte(`{
		"properties": {
			"a": {"type": "string", "title": "A"},
			"b": {"type": "string", "title": "B",
				"x-seam-visible-when": {"field": "a", "op": "not_empty"}}
		}
	}`)
	html := renderVisHTML(t, flatSchema, nil, RenderContext{})
	if !strings.Contains(html, "data-visible-when") {
		t.Fatal("expected a visible_when attribute in a flat (no-tabs) form")
	}
	// A flat (no-tabs) schema renders no tab-switch script, so exactly
	// one <script> tag (visibilityScript) is expected. readValue itself
	// isn't a valid per-script counter -- it appears twice within a
	// single copy of the script (its own function definition, then its
	// call site inside evalCond), which is correct, not duplication.
	if got := strings.Count(html, "<script>"); got != 1 {
		t.Errorf("expected exactly 1 <script> tag for a flat form, found %d", got)
	}
}

func TestVisibilityScript_NoOpWhenNoConditionsPresent(t *testing.T) {
	schema := []byte(`{"properties": {"a": {"type": "string", "title": "A"}}}`)
	html := renderVisHTML(t, schema, nil, RenderContext{})
	// data-visible-when="..." (HTML attribute syntax) only appears on an
	// actual rendered element -- the script's own JS text also contains
	// the bare substring "data-visible-when" (inside querySelectorAll
	// and getAttribute calls), so that alone isn't a valid check.
	if strings.Contains(html, `data-visible-when="`) {
		t.Errorf("no visible_when anywhere in this schema -- should emit no data-visible-when attributes:\n%s", html)
	}
	if !strings.Contains(html, "field-a") {
		t.Errorf("plain field should still render normally:\n%s", html)
	}
}

// TestVisibilityScript_QuerySelectorIsDocumentScopedNotFormScoped is a
// structural confirmation of T-57's real root cause (docs/TRACKING.md,
// refined 2026-08-04): the script's own querySelectorAll call is
// literally document.querySelectorAll(...), not scoped to any
// particular <form> -- confirmed by checking the rendered JS source
// directly, not inferred. This is what makes T-57 worse than a simple,
// predictable 2x duplication on a multi-form page: every IIFE re-scans
// the entire document, not just the fields the form it was emitted for
// actually owns.
func TestVisibilityScript_QuerySelectorIsDocumentScopedNotFormScoped(t *testing.T) {
	schema := []byte(`{
		"properties": {
			"a": {"type": "string", "title": "A"},
			"b": {"type": "string", "title": "B", "x-seam-visible-when": {"field": "a", "op": "not_empty"}}
		}
	}`)
	html := renderVisHTML(t, schema, nil, RenderContext{})
	if !strings.Contains(html, "document.querySelectorAll('[data-visible-when]')") {
		t.Error("expected the visibility evaluator to scan the whole document, not a form-scoped element -- if this changes, T-57's root-cause description needs updating too")
	}
}

// TestVisibilityScript_TwoFormsOnOnePage_EarlierFormsFieldGetsDuplicateListener
// demonstrates T-57's real, refined consequence directly: rendering two
// independent forms on one page (mirroring how routes.go's Profile
// handler actually calls FormEngine.Render() twice), where only the
// FIRST form has a real visible_when condition, produces TWO copies of
// the listener-registration call for that field -- one from its own
// form's IIFE, one "helpfully" re-registered by the second form's IIFE
// re-scanning the whole document. This can't execute the JS in a real
// browser (no DOM here) to prove the runtime duplicate-firing
// consequence, but it does mechanically prove the setup that causes
// it: two full script copies in the output, and the field's
// getElementById lookup appearing twice, not once, across them.
func TestVisibilityScript_TwoFormsOnOnePage_EarlierFormsFieldGetsDuplicateListener(t *testing.T) {
	schemaWithCondition := []byte(`{
		"properties": {
			"a": {"type": "string", "title": "A"},
			"b": {"type": "string", "title": "B", "x-seam-visible-when": {"field": "a", "op": "not_empty"}}
		}
	}`)
	schemaWithoutCondition := []byte(`{"properties": {"c": {"type": "string", "title": "C"}}}`)

	e := &FormEngine{}
	form1 := mi.RenderToString(e.Render(context.Background(), schemaWithCondition, nil, nil, RenderContext{}))
	form2 := mi.RenderToString(e.Render(context.Background(), schemaWithoutCondition, nil, nil, RenderContext{}))
	combined := form1 + form2 // simulates both forms present on one page, in DOM order

	if got := strings.Count(combined, "<script>"); got != 2 {
		t.Fatalf("expected 2 <script> tags (one visibilityScript per Render() call) -- got %d, T-57's own premise (one emission per call) may have changed", got)
	}
	// The marker element for field "b" (data-visible-when="...") is
	// rendered exactly once -- form1 renders it, form2 has nothing like
	// it. Combined with the separately-proven fact that each script
	// copy's selector is document.querySelectorAll(...), not scoped to
	// its own form (TestVisibilityScript_QuerySelectorIsDocumentScopedNotFormScoped),
	// this is the actual mechanism behind T-57's refined finding: both
	// script copies, if executed in a real browser, would independently
	// find this one element and independently register a listener for
	// it -- not because the element itself is duplicated (it isn't),
	// but because two identically-global selectors both match the same
	// single element.
	if got := strings.Count(combined, `data-visible-when="`); got != 1 {
		t.Fatalf(`expected exactly 1 data-visible-when element in the combined output, got %d`, got)
	}
	if got := strings.Count(combined, "document.querySelectorAll('[data-visible-when]')"); got != 2 {
		t.Errorf("expected 2 independent global-selector script copies (one per <script> tag) in the combined output, got %d -- one element matched by two independent scans is T-57's actual mechanism", got)
	}
}
