package formengine

import (
	"context"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"
)

func linksField() Field {
	return Field{
		Name: "links_comodato", Title: "Links Comodato", JSONType: "array",
		XSeamInput: "document-links",
		Items: []Field{
			{Name: "nombre", Title: "Nombre", JSONType: "string"},
			{Name: "url", Title: "URL", JSONType: "string", Format: "uri"},
			{Name: "tipo", Title: "Tipo (opcional)", JSONType: "string"},
		},
	}
}

func renderLinksRow(t *testing.T, value interface{}) string {
	t.Helper()
	e := &FormEngine{}
	values := map[string]interface{}{"links_comodato": value}
	h := func(b *mi.Builder) mi.Node {
		return renderFieldRow(b, context.Background(), e, linksField(), values, "", false, RenderContext{})
	}
	return mi.RenderToString(h)
}

func TestDocumentLinks_NoValue_RendersOneEmptyRow(t *testing.T) {
	html := renderLinksRow(t, nil)
	if strings.Count(html, "repeatable-row") == 0 {
		t.Fatalf("expected at least one row:\n%s", html)
	}
	if !strings.Contains(html, `name="links_comodato_nombre[]"`) {
		t.Errorf("expected the parallel-array field name for nombre:\n%s", html)
	}
	if !strings.Contains(html, `name="links_comodato_url[]"`) {
		t.Errorf("expected the parallel-array field name for url:\n%s", html)
	}
	if !strings.Contains(html, `name="links_comodato_tipo[]"`) {
		t.Errorf("expected the parallel-array field name for tipo:\n%s", html)
	}
}

func TestDocumentLinks_URLInputType(t *testing.T) {
	html := renderLinksRow(t, nil)
	idx := strings.Index(html, `name="links_comodato_url[]"`)
	if idx == -1 {
		t.Fatal("url input not found")
	}
	tag := extractTag(html, idx)
	if !strings.Contains(tag, `type="url"`) {
		t.Errorf("url sub-field (format:uri) should render as type=url, got:\n%s", tag)
	}
}

func TestDocumentLinks_ExistingValuesPopulateRows(t *testing.T) {
	value := []interface{}{
		map[string]interface{}{"nombre": "Contrato", "url": "https://example.com/a.pdf", "tipo": "pdf"},
		map[string]interface{}{"nombre": "Foto", "url": "https://example.com/b.jpg", "tipo": ""},
	}
	html := renderLinksRow(t, value)
	if strings.Count(html, `class="flex gap-2 items-start repeatable-row"`) != 2 {
		t.Errorf("expected exactly 2 rows for 2 existing links, got %d\n%s", strings.Count(html, `class="flex gap-2 items-start repeatable-row"`), html)
	}
	if !strings.Contains(html, `value="Contrato"`) {
		t.Errorf("expected the first row's nombre value populated:\n%s", html)
	}
	if !strings.Contains(html, `value="https://example.com/b.jpg"`) {
		t.Errorf("expected the second row's url value populated:\n%s", html)
	}
}

func TestDocumentLinks_RemoveButtonPresent(t *testing.T) {
	html := renderLinksRow(t, nil)
	if !strings.Contains(html, "b.closest(&#39;.repeatable-row&#39;)") {
		t.Errorf("expected the per-row remove button:\n%s", html)
	}
}

func TestDocumentLinks_AddButtonClonesFirstRow(t *testing.T) {
	html := renderLinksRow(t, nil)
	if !strings.Contains(html, "Add link") || strings.Contains(html, "+ Add link") {
		t.Errorf("expected the add-row label without its own plus (the button draws one):\n%s", html)
	}
	if !strings.Contains(html, "cloneNode(true)") {
		t.Errorf("expected the add-row JS to clone the first row:\n%s", html)
	}
}

func TestArrayOfMaps(t *testing.T) {
	got := arrayOfMaps([]interface{}{
		map[string]interface{}{"a": 1},
		"not a map", // should be skipped, not crash
		map[string]interface{}{"b": 2},
	})
	if len(got) != 2 {
		t.Fatalf("expected 2 valid maps (non-map entries skipped), got %d", len(got))
	}

	if got := arrayOfMaps(nil); got != nil {
		t.Errorf("nil input should give nil, got %v", got)
	}
	if got := arrayOfMaps("not an array"); got != nil {
		t.Errorf("wrongly-typed input should give nil, got %v", got)
	}
}

// Regression: same zero-rows failure as repeatable (see repeatable_test.go).
func TestDocumentLinks_AddFallsBackToStashedTemplateAfterLastRowRemoved(t *testing.T) {
	html := renderLinksRow(t, nil)
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

// The widget is the engine's repeatable underneath: same chrome, none of the
// old hand-coded look (red cross, bare inputs, text-link add button).
func TestDocumentLinks_UsesTheRepeatableChrome(t *testing.T) {
	html := renderLinksRow(t, nil)
	for _, want := range []string{`fe-remove`, `fe-btn fe-add`, `material-icons`, `placeholder="Nombre"`, `placeholder="URL"`} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %s:\n%s", want, html)
		}
	}
	for _, gone := range []string{"doc-link-row", "\u2715", "text-red-500"} {
		if strings.Contains(html, gone) {
			t.Errorf("old hand-coded markup %q must be gone:\n%s", gone, html)
		}
	}
}

// Cells stay plain inputs even when an item carries an enum: the old widget
// never rendered a select here and the handlers expect a string.
func TestDocumentLinks_CellsStayPlainInputs(t *testing.T) {
	f := linksField()
	f.Items[2].Enum = []string{"pdf", "img"}
	e := &FormEngine{}
	html := mi.RenderToString(renderDocumentLinks(context.Background(), e, f, nil, false, RenderContext{}))
	if strings.Contains(html, "<select") {
		t.Errorf("document-links cells must not become selects:\n%s", html)
	}
}
