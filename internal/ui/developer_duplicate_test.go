package ui

import (
	"net/url"
	"strings"
	"testing"
)

// Duplicating a field, a section and a tab from the designer, and the
// preview in another language.
func TestDesigner_DuplicateBlocks(t *testing.T) {
	h, _ := designerHarness(t)
	s := openDesigner(t, h, "masters/marca", "file")
	for _, want := range []string{`value="dup:name"`, `id="dev-preview-locale"`, `id="dev-designer-preview"`} {
		if !strings.Contains(s.body, want) {
			t.Fatalf("the page offers %s", want)
		}
	}
	s.do("dup:name", nil)
	if !strings.Contains(s.doc, `"name_2"`) || !strings.Contains(s.body, `value="apply:name_2"`) {
		t.Fatalf("the copy exists and is the field selected: %s", s.doc)
	}
	s.do("tab-add", url.Values{"new_tab_id": {"general"}, "new_tab_label": {"General"}})
	if !strings.Contains(s.body, `value="tab-dup:general"`) || !strings.Contains(s.body, `value="sec-dup:general/general-main"`) {
		t.Fatalf("a tab and a section can be duplicated")
	}
	s.do("sec-dup:general/general-main", nil)
	if !strings.Contains(s.doc, `"general-main-2"`) || !strings.Contains(s.doc, `"code_2"`) || !strings.Contains(s.doc, `"name_3"`) {
		t.Fatalf("the section's copy holds copies of its fields: %s", s.doc)
	}
	s.do("tab-dup:general", nil)
	if !strings.Contains(s.doc, `"general-2"`) || !strings.Contains(s.body, `data-tab="general-2"`) {
		t.Fatalf("the tab's copy is in the tree: %s", s.doc)
	}
	before := s.doc
	s.do("dup:nothing", nil)
	if s.doc != before {
		t.Errorf("a copy of a field that does not exist changes nothing")
	}
}

func TestDesigner_PreviewLanguage(t *testing.T) {
	h, _ := designerHarness(t)
	s := openDesigner(t, h, "masters/marca", "file")
	selected := func(body, locale string) bool {
		i := strings.Index(body, `id="dev-preview-locale"`)
		if i < 0 {
			return false
		}
		sel := body[i:]
		if j := strings.Index(sel, "</select>"); j >= 0 {
			sel = sel[:j]
		}
		for _, o := range strings.Split(sel, "<option")[1:] {
			if strings.Contains(o, `value="`+locale+`"`) {
				return strings.Contains(o, "selected")
			}
		}
		return false
	}
	en := s.body
	s.do("preview", url.Values{"preview_locale": {"es"}})
	if !selected(s.body, "es") {
		t.Fatalf("the chosen language is the one selected")
	}
	if !strings.Contains(s.body, `for="field-name">Nombre`) || !strings.Contains(en, `for="field-name">Name`) {
		t.Errorf("the preview is rendered in the chosen language")
	}
	// the choice travels with the next action; one that is not a language is ignored
	s.do("dup:name", url.Values{"preview_locale": {"es"}})
	if !selected(s.body, "es") {
		t.Errorf("the language stays chosen after another action")
	}
	s.do("preview", url.Values{"preview_locale": {`"><script>`}})
	if selected(s.body, "es") || strings.Contains(s.body, `"><script>`) {
		t.Errorf("an unknown language falls back to the application's and is not echoed")
	}
}
