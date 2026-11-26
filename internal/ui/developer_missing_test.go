package ui

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	fe "github.com/ha1tch/seam-ui/internal/formengine"
)

// The forms list answers "which forms use this widget, this list or this
// field", from the definitions in force.
func TestForms_SearchByWhatADefinitionUses(t *testing.T) {
	h, _ := scopedHarness(t)
	list := func(q string) string { return h.get(t, "/developer/forms?q="+q).Body.String() }
	// by source
	body := list("entity:users")
	for _, want := range []string{"crew/create", "issues/assign", "outsourced/account", "source:entity:users"} {
		if !strings.Contains(body, want) {
			t.Errorf("searching a source lists %s", want)
		}
	}
	if strings.Contains(body, "masters/marca") {
		t.Errorf("forms that do not use the source are left out")
	}
	// by widget
	if body := list("any-picker"); !strings.Contains(body, "widget:any-picker") || !strings.Contains(body, "issues/assign") {
		t.Errorf("searching a widget lists the forms that use it")
	}
	// by name, as before, without a note of what matched
	if body := list("marca"); !strings.Contains(body, "masters/marca") || strings.Contains(body, "dev-uses") {
		t.Errorf("a match on the name needs no explanation")
	}
	// an override in force is what is searched
	if loc := location(formSave(h, t, "tenant", e2eOverride, "")); !strings.Contains(loc, "msg=saved") {
		t.Fatal(loc)
	}
	if body := list("alpha"); !strings.Contains(body, "system/user_create") || !strings.Contains(body, "field:alpha") {
		t.Errorf("the definition in force is the one searched")
	}
	if got := developerFormUses([]byte(`{"type":"object","properties":{"a":{"type":"string","x-seam-input":"select","x-seam-source":"entity:brands"}}}`), "brands"); len(got) != 1 || got[0] != "source:entity:brands" {
		t.Errorf("uses: %v", got)
	}
	if developerFormUses([]byte(`{`), "x") != nil || developerFormUses(nil, "x") != nil {
		t.Errorf("a definition that cannot be read uses nothing")
	}
}

// The identity table is checked against the pages themselves wherever a page
// renders without data: what the engine reports must be what the table says.
func TestSeed_AgreesWithThePagesThatRender(t *testing.T) {
	h, _ := designerHarness(t)
	table := &developerLabelTrace{docs: developerTrace.docs, pairs: map[developerSchemaRef][]developerRenderIdentity{}}
	table.mergeLocked(developerPagesSeed)
	developerTrace.reset()
	t.Cleanup(developerTrace.reseed)
	for _, p := range []string{"/brands/new", "/regions/new", "/currencies/new", "/admin/users/new", "/developer/overview",
		"/developer/translations?new=1", "/developer/translations?key=" + trKey, "/developer/forms?new=1", "/developer/forms?schema=masters/marca"} {
		h.get(t, p)
	}
	learnt := 0
	for _, ref := range developerEmbeddedSchemas() {
		for _, got := range developerTrace.identities(ref) {
			learnt++
			found := false
			for _, want := range table.identities(ref) {
				found = found || want == got
			}
			if !found {
				t.Errorf("%s is rendered as %+v, the table says %+v (update developer_pages_seed.json)", ref, got, table.identities(ref))
			}
		}
	}
	if learnt < 7 {
		t.Errorf("only %d identities were learnt from the pages; the check has stopped seeing them", learnt)
	}
}

// The try page can leave out the record being edited, as a page that edits
// a record leaves it out of its own lists.
func TestTry_TheRecordBeingEditedIsLeftOut(t *testing.T) {
	h, _ := designerHarness(t)
	s := &developerTrySources{h: h.ui, empty: map[string]bool{}, exclude: "7"}
	in := []fe.Option{{Value: "6", Label: "a"}, {Value: "7", Label: "b"}, {Value: "8", Label: "c"}}
	if out := s.without(in); len(out) != 2 || out[0].Value != "6" || out[1].Value != "8" || len(in) != 3 {
		t.Errorf("the record is left out and the list given is untouched: %v", out)
	}
	if out := (&developerTrySources{}).without(in); len(out) != 3 {
		t.Errorf("with no record every row is offered")
	}
	body := h.get(t, "/developer/forms/try?schema=masters/marca&record=7").Body.String()
	if !strings.Contains(body, `id="dev-try-record"`) || !strings.Contains(body, `value="7"`) || !strings.Contains(body, "masters%2Fmarca&amp;record=7") {
		t.Errorf("the record is kept in the form's address")
	}
	if body := h.get(t, "/developer/forms/try?schema=masters/marca&record=7%20OR%201").Body.String(); strings.Contains(body, "OR 1") {
		t.Errorf("only an ID is accepted as a record")
	}
}

// Where a deployment asks for it, a form made with the tools has a page of
// its own for the tenant's users; without the setting there is none.
func TestCustomForms_HaveAPageWhereAsked(t *testing.T) {
	h, _ := designerHarness(t)
	if loc := location(h.post(t, "/developer/forms/create", url.Values{"module": {"inspections"}, "name": {"checklist"}, "scope": {"tenant"}})); strings.Contains(loc, "error=") {
		t.Fatalf("create: %s", loc)
	}
	if _, _, _, ok := h.ui.customFormInForce(1, "inspections", "checklist"); ok {
		t.Fatalf("without the setting a custom form has no page")
	}
	if ed := h.get(t, "/developer/forms?schema=inspections/checklist").Body.String(); strings.Contains(ed, `id="dev-custom-page"`) {
		t.Errorf("and its editor offers none")
	}
	h.ui.config.DevSettings.CustomPages = true
	ref, doc, form, ok := h.ui.customFormInForce(1, "inspections", "checklist")
	if !ok || doc == nil || form == nil || customFormURL(ref) != "/forms/inspections/checklist" {
		t.Fatalf("with the setting the form in force has a page: %v", ok)
	}
	for _, c := range []struct {
		tenant       int64
		module, name string
	}{{2, "inspections", "checklist"}, {1, "masters", "marca"}, {1, "inspections", "nothing"}, {1, "../etc", "passwd"}, {0, "inspections", "checklist"}} {
		if _, _, _, ok := h.ui.customFormInForce(c.tenant, c.module, c.name); ok {
			t.Errorf("no page for %+v: another tenant, a built-in form, an unknown or unsafe name", c)
		}
	}
	ed := h.get(t, "/developer/forms?schema=inspections/checklist").Body.String()
	if !strings.Contains(ed, `id="dev-custom-page"`) || !strings.Contains(ed, `href="/forms/inspections/checklist"`) || !strings.Contains(ed, "Nothing received yet") {
		t.Errorf("the editor links to the page and lists what it received")
	}
	if ed := h.get(t, "/developer/forms?schema=masters/marca").Body.String(); strings.Contains(ed, `id="dev-custom-page"`) {
		t.Errorf("a built-in form has no such page")
	}
}

// The designer edits the block tree: tabs, sections side by side, fields
// placed by the controls or by a drop, and the engine has the last word.
func TestDesigner_LayoutBlockTree(t *testing.T) {
	h, _ := designerHarness(t)
	s := openDesigner(t, h, "masters/marca", "file")
	if !strings.Contains(s.body, `id="dev-layout"`) || !strings.Contains(s.body, "The form is flat") {
		t.Fatalf("a flat form says so and offers the first tab")
	}
	s.do("tab-add", url.Values{"new_tab_id": {"general"}, "new_tab_label": {"General"}})
	if !strings.Contains(s.doc, `"tabs"`) || !strings.Contains(s.body, `data-tab="general"`) || !strings.Contains(s.body, `data-field="code"`) {
		t.Fatalf("the first tab takes every field and the tree is drawn")
	}
	s.do("sec-add:general", url.Values{"new_sec_id:general": {"names"}, "new_sec_label:general": {"Names"}})
	s.do("place:name", url.Values{"place_to:name": {"general/names"}})
	if !strings.Contains(s.doc, `"names"`) {
		t.Fatalf("the section was added: %s", s.doc)
	}
	s.do("sec-join:general/names", nil)
	if !strings.Contains(s.doc, "section_defs") {
		t.Errorf("two sections side by side use the rows spelling: %s", s.doc)
	}
	// a drop posts the form with no button: the field and where it landed
	rec := h.post(t, "/developer/designer", url.Values{"schema": {"masters/marca"}, "scope": {"file"}, "json": {s.doc}, "drop_field": {"code"}, "drop_to": {"general/names/name"}})
	doc := docOf(t, rec.Body.String())
	if i, j := strings.Index(doc, `"code"`), strings.Index(doc, `"names"`); rec.Code != http.StatusOK || i < 0 || j < 0 {
		t.Fatalf("drop: %d", rec.Code)
	}
	if !strings.Contains(strings.Join(strings.Fields(doc), ""), `"fields":["code","name"]`) {
		t.Errorf("the dropped field goes before the one it was dropped on: %s", doc)
	}
	s.doc = doc
	// details and embeds are edited as lists; a bad one is refused and nothing changes
	before := s.doc
	s.do("extras:general", url.Values{"details:general": {`{"not":"a list"}`}})
	if s.doc != before || !strings.Contains(s.body, "must be a JSON list of objects") {
		t.Errorf("a refusal says why and leaves the document alone")
	}
	// hostile input is refused by the model, not echoed
	s.do("tab-add", url.Values{"new_tab_id": {`"><script>x</script>`}, "new_tab_label": {"x"}})
	if strings.Contains(s.body, "<script>x</script>") || s.doc != before {
		t.Errorf("a hostile tab ID is refused and escaped")
	}
	// saved through the existing route, the engine renders the tab
	if loc := location(h.post(t, "/developer/forms/save", url.Values{"schema": {"masters/marca"}, "scope": {"file"}, "json": {s.doc}})); !strings.Contains(loc, "msg=saved") {
		t.Fatalf("save: %s", loc)
	}
	if page := h.get(t, "/brands/new").Body.String(); !strings.Contains(page, "General") || !strings.Contains(page, `name="code"`) {
		t.Errorf("the brand page renders the layout")
	}
	// removing the last tab makes the form flat again
	s.do("tab-remove:general", nil)
	if strings.Contains(s.doc, "x-seam-form") && strings.Contains(s.doc, `"tabs"`) {
		t.Errorf("with no tab the form is flat: %s", s.doc)
	}
}
