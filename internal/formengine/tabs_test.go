package formengine

import (
	"context"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"
)

func TestParseTabs_PlainSections_OneRowPerSection(t *testing.T) {
	raw := []byte(`{
		"properties": {
			"nombre": {"type": "string", "title": "Nombre"},
			"tipo_id": {"type": "integer", "title": "Tipo"}
		},
		"x-seam-form": {
			"tabs": [
				{
					"id": "tab-principal",
					"label": "Principal",
					"sections": [
						{"id": "basico", "label": "Básico", "fields": ["nombre"]},
						{"id": "clasif", "label": "Clasificación", "fields": ["tipo_id"]}
					]
				}
			]
		}
	}`)
	form, err := ParseSchema(raw)
	if err != nil {
		t.Fatalf("parseSchema: %v", err)
	}
	if len(form.Tabs) != 1 {
		t.Fatalf("got %d tabs, want 1", len(form.Tabs))
	}
	tab := form.Tabs[0]
	if len(tab.Rows) != 2 {
		t.Fatalf("plain sections should become one row each, got %d rows", len(tab.Rows))
	}
	if tab.Rows[0].Widths[0] != "full" || tab.Rows[1].Widths[0] != "full" {
		t.Errorf("plain-sections rows should be full width, got %v / %v", tab.Rows[0].Widths, tab.Rows[1].Widths)
	}
	if tab.Rows[0].Sections[0].ID != "basico" || tab.Rows[1].Sections[0].ID != "clasif" {
		t.Errorf("section order wrong: %q, %q", tab.Rows[0].Sections[0].ID, tab.Rows[1].Sections[0].ID)
	}
	if len(tab.Rows[0].Sections[0].Fields) != 1 || tab.Rows[0].Sections[0].Fields[0].Name != "nombre" {
		t.Errorf("section field resolution failed: %+v", tab.Rows[0].Sections[0].Fields)
	}
}

func TestParseTabs_RowsWithSectionDefs(t *testing.T) {
	raw := []byte(`{
		"properties": {
			"a": {"type": "string"}, "b": {"type": "string"}, "c": {"type": "string"}
		},
		"x-seam-form": {
			"tabs": [{
				"id": "t1", "label": "T1",
				"rows": [
					{"sections": ["s1", "s2"], "widths": ["2/3", "1/3"]},
					{"sections": ["s3"]}
				],
				"section_defs": [
					{"id": "s1", "label": "S1", "fields": ["a"]},
					{"id": "s2", "label": "S2", "fields": ["b"]},
					{"id": "s3", "label": "S3", "fields": ["c"]}
				]
			}]
		}
	}`)
	form, err := ParseSchema(raw)
	if err != nil {
		t.Fatalf("parseSchema: %v", err)
	}
	tab := form.Tabs[0]
	if len(tab.Rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(tab.Rows))
	}
	if len(tab.Rows[0].Sections) != 2 {
		t.Fatalf("row 0 should have 2 sections, got %d", len(tab.Rows[0].Sections))
	}
	if tab.Rows[0].Widths[0] != "2/3" || tab.Rows[0].Widths[1] != "1/3" {
		t.Errorf("widths not preserved: %v", tab.Rows[0].Widths)
	}
}

func TestParseTabs_SectionDefNotReferenced_Rejected(t *testing.T) {
	// §15.1 (resolved: Option B) — a section_def not referenced by any
	// row must reject the schema at load time.
	raw := []byte(`{
		"properties": {"a": {"type": "string"}},
		"x-seam-form": {
			"tabs": [{
				"id": "t1", "label": "T1",
				"rows": [{"sections": ["s1"]}],
				"section_defs": [
					{"id": "s1", "fields": ["a"]},
					{"id": "orphan", "fields": []}
				]
			}]
		}
	}`)
	_, err := ParseSchema(raw)
	if err == nil {
		t.Fatal("expected an error for an unreferenced section_def (§15.1), got nil")
	}
	if !strings.Contains(err.Error(), "orphan") {
		t.Errorf("error should name the offending section_def, got: %v", err)
	}
}

func TestParseTabs_RowReferencesUndefinedSection_Rejected(t *testing.T) {
	raw := []byte(`{
		"properties": {"a": {"type": "string"}},
		"x-seam-form": {
			"tabs": [{
				"id": "t1", "label": "T1",
				"rows": [{"sections": ["nonexistent"]}],
				"section_defs": [{"id": "s1", "fields": ["a"]}]
			}]
		}
	}`)
	_, err := ParseSchema(raw)
	if err == nil {
		t.Fatal("expected an error when a row references an undefined section_def, got nil")
	}
}

func TestParseSchema_NoXSeamForm_NoTabs(t *testing.T) {
	raw := []byte(`{"properties": {"a": {"type": "string"}}}`)
	form, err := ParseSchema(raw)
	if err != nil {
		t.Fatalf("parseSchema: %v", err)
	}
	if form.Tabs != nil {
		t.Errorf("expected nil Tabs for a schema with no x-seam-form, got %+v", form.Tabs)
	}
}

func TestResolveWidths(t *testing.T) {
	cases := []struct {
		name   string
		widths []string
		n      int
		want   []int
	}{
		{"none given, 2 sections", nil, 2, []int{6, 6}},
		{"none given, 3 sections", nil, 3, []int{4, 4, 4}},
		{"all explicit", []string{"2/3", "1/3"}, 2, []int{8, 4}},
		{"partial, one missing", []string{"1/2"}, 2, []int{6, 6}},
		{"single full", []string{"full"}, 1, []int{12}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := resolveWidths(c.widths, c.n)
			if len(got) != len(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("index %d: got %d, want %d (full: %v)", i, got[i], c.want[i], got)
				}
			}
		})
	}
}

func renderTabbedHTML(t *testing.T, schema []byte, values map[string]interface{}, errs map[string]string, rc RenderContext) string {
	t.Helper()
	e := &FormEngine{}
	h := e.Render(context.Background(), schema, values, errs, rc)
	return mi.RenderToString(h)
}

func multiTabSchema() []byte {
	return []byte(`{
		"properties": {
			"nombre":  {"type": "string", "title": "Nombre"},
			"tipo_id": {"type": "integer", "title": "Tipo"},
			"ubicacion_fisica_id": {"type": "integer", "title": "Ubicación"},
			"custodio_id": {"type": "integer", "title": "Custodio"}
		},
		"required": ["nombre"],
		"x-seam-form": {
			"width": "2xl",
			"tabs": [
				{
					"id": "tab-principal", "label": "Principal",
					"rows": [
						{"sections": ["basico"]},
						{"sections": ["clasif"]}
					],
					"section_defs": [
						{"id": "basico", "label": "Básico", "layout": "2-col", "fields": ["nombre", "tipo_id"]},
						{"id": "clasif", "label": "Clasificación", "fields": []}
					]
				},
				{
					"id": "tab-loc", "label": "Ubicación",
					"rows": [
						{"sections": ["ubic", "custodia"], "widths": ["1/2", "1/2"]}
					],
					"section_defs": [
						{"id": "ubic", "label": "Ubicación", "fields": ["ubicacion_fisica_id"]},
						{"id": "custodia", "label": "Custodia", "fields": ["custodio_id"]}
					]
				}
			]
		}
	}`)
}

func TestMultiTab_BothTabsRender(t *testing.T) {
	html := renderTabbedHTML(t, multiTabSchema(), nil, nil, RenderContext{})
	for _, want := range []string{"Principal", "Ubicación", "field-nombre", "field-tipo_id", "field-ubicacion_fisica_id", "field-custodio_id"} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q in rendered output", want)
		}
	}
	// class="fe-tab-btn ..." only appears on actual button elements —
	// counting the bare "fe-tab-btn" substring would also match the
	// switch script's own querySelectorAll('.fe-tab-btn') text.
	if strings.Count(html, `class="fe-tab-btn`) != 2 {
		t.Errorf("expected 2 tab buttons, found %d", strings.Count(html, `class="fe-tab-btn`))
	}
	if strings.Count(html, `class="fe-tab-panel`) != 2 {
		t.Errorf("expected 2 tab panels, found %d", strings.Count(html, `class="fe-tab-panel`))
	}
}

func TestMultiTab_EmptySectionOmitted(t *testing.T) {
	// "clasif" section has no fields at all -- should not render a
	// section header for a section with nothing in it.
	html := renderTabbedHTML(t, multiTabSchema(), nil, nil, RenderContext{})
	if strings.Contains(html, "Clasificación") {
		t.Errorf("empty section should be omitted entirely, but its header rendered:\n%s", html)
	}
}

func TestMultiTab_HiddenTabOmitted(t *testing.T) {
	rc := RenderContext{HiddenTabs: []string{"tab-loc"}}
	html := renderTabbedHTML(t, multiTabSchema(), nil, nil, rc)
	if strings.Contains(html, "field-ubicacion_fisica_id") {
		t.Errorf("hidden tab's fields should not render at all:\n%s", html)
	}
	if strings.Count(html, `class="fe-tab-btn`) != 1 {
		t.Errorf("expected only 1 visible tab button, found %d", strings.Count(html, `class="fe-tab-btn`))
	}
}

func TestMultiTab_HiddenSectionWithinVisibleTab(t *testing.T) {
	rc := RenderContext{HiddenSections: []string{"ubic"}}
	html := renderTabbedHTML(t, multiTabSchema(), nil, nil, rc)
	if strings.Contains(html, "field-ubicacion_fisica_id") {
		t.Errorf("hidden section's field should not render:\n%s", html)
	}
	// custodia section, same row, must still render.
	if !strings.Contains(html, "field-custodio_id") {
		t.Errorf("sibling section in the same row should still render:\n%s", html)
	}
}

func TestMultiTab_SectionPartiallyHidden_RemainingFieldsStillRender(t *testing.T) {
	// §3c.6: hiding SOME (not all) of a section's fields must not
	// collapse the section -- distinct from HiddenSections (whole
	// section) tested above.
	rc := RenderContext{HiddenFields: []string{"tipo_id"}} // one of two fields in the "basico" section
	html := renderTabbedHTML(t, multiTabSchema(), nil, nil, rc)
	if strings.Contains(html, "field-tipo_id") {
		t.Errorf("tipo_id should not render:\n%s", html)
	}
	if !strings.Contains(html, "field-nombre") {
		t.Errorf("nombre (sibling field, same section) should still render:\n%s", html)
	}
}

func TestMultiTab_ReadOnlyTabAppliesToAllItsFields(t *testing.T) {
	rc := RenderContext{ReadOnlyTabs: []string{"tab-principal"}}
	html := renderTabbedHTML(t, multiTabSchema(), nil, nil, rc)
	// Only tab-principal's fields (nombre, tipo_id) should be wrapped
	// read-only; tab-loc's fields should not be.
	if strings.Count(html, "opacity-60") != 2 {
		t.Errorf("expected exactly 2 read-only-wrapped fields (tab-principal only), got %d",
			strings.Count(html, "opacity-60"))
	}
}

func TestMultiTab_RowWidthsApplied(t *testing.T) {
	html := renderTabbedHTML(t, multiTabSchema(), nil, nil, RenderContext{})
	if !strings.Contains(html, "col-span-6") {
		t.Errorf("expected col-span-6 for the 1/2 + 1/2 row in tab-loc:\n%s", html)
	}
}

func TestMultiTab_HiddenRowsAreTabScopedNotFlat(t *testing.T) {
	// §3c.4's own ResolvedPerms declares HiddenRows as map[string][]int
	// (tab-scoped) -- confirms row index 0 hidden in tab-principal must
	// not also hide row index 0 in tab-loc, which is a *different* row.
	rc := RenderContext{HiddenRows: map[string][]int{"tab-principal": {0}}}
	html := renderTabbedHTML(t, multiTabSchema(), nil, nil, rc)
	if strings.Contains(html, "field-nombre") || strings.Contains(html, "field-tipo_id") {
		t.Errorf("tab-principal's row 0 should be omitted:\n%s", html)
	}
	if !strings.Contains(html, "field-ubicacion_fisica_id") {
		t.Errorf("tab-loc's row 0 must still render -- HiddenRows is tab-scoped, not flat:\n%s", html)
	}
}

func TestMultiTab_AllTabsHiddenShowsMessage(t *testing.T) {
	rc := RenderContext{HiddenTabs: []string{"tab-principal", "tab-loc"}}
	html := renderTabbedHTML(t, multiTabSchema(), nil, nil, rc)
	if !strings.Contains(html, "no_visible_tabs") {
		t.Errorf("expected the no-visible-tabs message when every tab is hidden:\n%s", html)
	}
}

func TestMultiTab_AllFieldsIndividuallyHidden_TabOmittedLikeHiddenTabs(t *testing.T) {
	// §3c.6: a tab whose fields are ALL in HiddenFields is omitted the
	// same way as if it were directly in HiddenTabs -- distinct code
	// path (allTabFieldsHidden), not exercised by TestMultiTab_HiddenTabOmitted
	// which hides the tab explicitly.
	rc := RenderContext{HiddenFields: []string{"nombre", "tipo_id"}} // every field in tab-principal
	html := renderTabbedHTML(t, multiTabSchema(), nil, nil, rc)
	if strings.Contains(html, "Principal") {
		t.Errorf("tab-principal should be omitted once all its fields are individually hidden:\n%s", html)
	}
	if strings.Count(html, `class="fe-tab-btn`) != 1 {
		t.Errorf("expected only tab-loc's button to remain, found %d buttons", strings.Count(html, `class="fe-tab-btn`))
	}
	if !strings.Contains(html, "field-ubicacion_fisica_id") {
		t.Errorf("tab-loc should be unaffected:\n%s", html)
	}
}

func TestMultiTab_ActiveTabSkipsToFirstVisibleWhenFirstIsHidden(t *testing.T) {
	// §3c.6: if the tab that would normally activate by default (the
	// schema's first) is hidden, the engine activates the first
	// *visible* tab instead.
	rc := RenderContext{HiddenTabs: []string{"tab-principal"}}
	html := renderTabbedHTML(t, multiTabSchema(), nil, nil, rc)
	idx := strings.Index(html, `class="fe-tab-btn`)
	if idx == -1 {
		t.Fatal("no tab button found")
	}
	btnHTML := extractTag(html, idx)
	if !strings.Contains(btnHTML, "border-indigo-600") {
		t.Errorf("the only remaining (tab-loc) button should be marked active since it's now first-visible:\n%s", btnHTML)
	}
	if !strings.Contains(btnHTML, `data-tab="tab-loc"`) {
		t.Errorf("expected tab-loc's button here, got:\n%s", btnHTML)
	}
}

func TestMultiTab_FirstTabActiveOthersHiddenByScript(t *testing.T) {
	html := renderTabbedHTML(t, multiTabSchema(), nil, nil, RenderContext{})
	if !strings.Contains(html, "fe-tab-panel") || !strings.Contains(html, "addEventListener") {
		t.Errorf("expected the tab-switch script to be present:\n%s", html)
	}
	// The active (first) tab button should carry the active border class.
	if !strings.Contains(html, "border-indigo-600") {
		t.Errorf("expected the first tab button to carry the active class:\n%s", html)
	}
}
