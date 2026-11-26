package ui

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"

	fe "github.com/ha1tch/seam-ui/internal/formengine"
)

const unrenderable = `{"type":"object","properties":{"zz":{"type":"string","title":"Z","x-seam-input":"nope"}}}`

func (h *devHarness) writeSchema(t *testing.T, module, name, doc string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(h.schDir, module), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.schDir, module, name+".json"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A definition the engine could not render, put in the directory by hand,
// never reaches a page: it is set aside when the directory loads.
func TestDirectory_AnUnrenderableFileIsSetAside(t *testing.T) {
	h, _ := scopedHarness(t)
	inForce := func(module, name string) string { return string(h.rt.Resolver.Resolve(1, module, name, nil)) }
	h.writeSchema(t, "system", "user_create", e2eOverride)
	if err := h.rt.Schemas.Reload(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inForce("system", "user_create"), "Alpha OVERRIDE") {
		t.Fatal("the good file is in force")
	}
	// the same file, broken by hand: what was in force stays
	h.writeSchema(t, "system", "user_create", unrenderable)
	// and one for a form nothing else defines
	h.writeSchema(t, "sensors", "probe", unrenderable)
	if err := h.rt.Schemas.Reload(); err != nil {
		t.Fatalf("a file set aside does not fail the load: %v", err)
	}
	if st := h.rt.Schemas.Status(); len(st.Refused) != 2 || st.LastError != "" {
		t.Fatalf("both are set aside: %+v", st)
	}
	if !strings.Contains(inForce("system", "user_create"), "Alpha OVERRIDE") || inForce("sensors", "probe") != "" {
		t.Errorf("no page is given the definitions set aside")
	}
	// the tools say so: the list, the Overview, and the editor, where it can be mended
	if list := h.get(t, "/developer/forms?q=user_create").Body.String(); !strings.Contains(list, "Cannot be rendered") {
		t.Errorf("the list flags the form whose file was set aside")
	}
	if list := h.get(t, "/developer/forms?q=probe").Body.String(); !strings.Contains(list, "sensors/probe") || !strings.Contains(list, "Cannot be rendered") {
		t.Errorf("a file set aside with nothing in force is still listed")
	}
	if ov := h.get(t, "/developer/overview").Body.String(); !strings.Contains(ov, `id="dev-refused-schemas"`) || !strings.Contains(ov, "system/user_create: ") || !strings.Contains(ov, "sensors/probe: ") {
		t.Errorf("the Overview lists the files set aside, with the reason")
	}
	rec := h.get(t, "/developer/forms?schema=sensors/probe&scope=file")
	if body := rec.Body.String(); rec.Code != http.StatusOK || !strings.Contains(body, "cannot be rendered") || !strings.Contains(body, "nope") {
		t.Errorf("the editor opens the file and says why (%d)", rec.Code)
	}
	// mended through the editor
	if loc := location(h.post(t, "/developer/forms/save", url.Values{"schema": {"sensors/probe"}, "scope": {"file"}, "json": {e2eOverride}})); !strings.Contains(loc, "msg=saved") {
		t.Fatalf("save: %s", loc)
	}
	if st := h.rt.Schemas.Status(); len(st.Refused) != 1 || !strings.Contains(inForce("sensors", "probe"), "Alpha OVERRIDE") {
		t.Errorf("once mended it is in force: %+v", st.Refused)
	}
}

// Once a form's page has been rendered, the editors name the keys that page
// asks for; before that they follow the naming convention and say so.
func TestForms_LabelKeysFollowThePageOnceItHasBeenSeen(t *testing.T) {
	h, _ := designerHarness(t)
	developerTrace.reset()
	t.Cleanup(developerTrace.reseed)
	ref := developerSchemaRef{"masters", "marca"}
	base, _ := developerEmbeddedSchema("masters", "marca")
	keyOf := func(field string) string {
		for _, lk := range h.handlerLabelKeys(t, "masters", "marca") {
			if lk.Field == field {
				return lk.Key
			}
		}
		return ""
	}
	editor := func() string { return h.get(t, "/developer/forms?schema=masters/marca").Body.String() }
	if keyOf("code") != "masters.forms.marca.field.code" || !strings.Contains(editor(), "follow the naming convention") {
		t.Fatalf("before the page is seen the convention is used, and said: %q", keyOf("code"))
	}
	engine := &fe.FormEngine{}
	render := func(ctx context.Context, module, form string) {
		_ = mi.RenderToString(engine.Render(ctx, resolveSchema(ctx, "masters", "marca", base), nil, nil, fe.RenderContext{Module: module, Form: form}))
	}
	// the tools' own renders do not count as the page
	render(withoutTrace(ctx_), "preview", "preview")
	_ = h.get(t, "/developer/forms/try?schema=masters/marca")
	if _, seen := developerTrace.identity(ref); seen {
		t.Fatalf("a preview or the try page is not the form's page")
	}
	// the page, with the identity it really uses
	render(ctx_, "masters", "marca")
	if id, seen := developerTrace.identity(ref); !seen || id.Module != "masters" || id.Form != "marca" {
		t.Fatalf("the page's identity is learnt: %+v %v", id, seen)
	}
	if keyOf("code") != "masters.forms.marca.field.code" || strings.Contains(editor(), "follow the naming convention") {
		t.Errorf("once seen, the keys are the page's and the note about the convention goes")
	}
	// a second page that renders the form under another identity is known too
	render(ctx_, "catalog", "brand")
	if ids := developerTrace.identities(ref); len(ids) != 2 || ids[1].Module != "catalog" {
		t.Errorf("both identities are kept, the newest last: %+v", ids)
	}
	if keyOf("code") != "masters.forms.marca.field.code" {
		t.Errorf("the keys of the first page are still listed")
	}
}

// Any form can be tried: rendered as it is in force, and a submission shown
// as the server receives it, with nothing stored.
func TestForms_TryPage(t *testing.T) {
	h, _ := designerHarness(t)
	if ed := h.get(t, "/developer/forms?schema=masters/marca").Body.String(); !strings.Contains(ed, `href="/developer/forms/try?schema=masters%2Fmarca"`) {
		t.Fatalf("the editor links to the try page")
	}
	rec := h.get(t, "/developer/forms/try?schema=masters/marca")
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `id="dev-try"`) || !strings.Contains(body, `name="code"`) || strings.Contains(body, `id="dev-try-result"`) {
		t.Fatalf("the form is shown, and no result before a submission (%d)", rec.Code)
	}
	rec = h.post(t, "/developer/forms/try?schema=masters/marca", url.Values{"code": {"AB"}, "name": {"<b>Acme</b>"}})
	body = rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `id="dev-try-result"`) || !strings.Contains(body, "&#34;code&#34;: &#34;AB&#34;") || strings.Contains(body, "<b>Acme</b>") {
		t.Errorf("a submission is shown as received, escaped (%d)", rec.Code)
	}
	if !strings.Contains(body, `value="AB"`) {
		t.Errorf("the form keeps what was submitted")
	}
	// a required field left empty is named
	rec = h.post(t, "/developer/forms/try?schema=masters/marca", url.Values{"code": {""}, "name": {""}})
	if body := rec.Body.String(); !strings.Contains(body, "&#34;errors&#34;") || !strings.Contains(body, "Required") {
		t.Errorf("the checks made on a post are shown")
	}
	// a custom form, used by no page, can be tried too
	if loc := location(h.post(t, "/developer/forms/create", url.Values{"module": {"inspections"}, "name": {"checklist"}, "scope": {"tenant"}})); !strings.Contains(loc, "inspections") {
		t.Fatalf("create: %s", loc)
	}
	if rec := h.get(t, "/developer/forms/try?schema=inspections/checklist"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `id="dev-try"`) {
		t.Errorf("a custom form can be tried (%d)", rec.Code)
	}
	// nothing else is accepted, and only developers get in
	if loc := location(h.get(t, "/developer/forms/try?schema=../etc/passwd")); !strings.Contains(loc, "error=unknown_form") {
		t.Errorf("an unknown form is refused: %s", loc)
	}
	if rec := h.do(t, http.MethodGet, "/developer/forms/try?schema=masters/marca", "admin", nil); rec.Code == http.StatusOK {
		t.Errorf("the try page is for developers only (%d)", rec.Code)
	}
}

// A new text can be written in every language when it is created.
func TestTranslations_ANewTextInEveryLanguage(t *testing.T) {
	h, _ := scopedHarness(t)
	const key = "orders.list.heading"
	panel := h.get(t, "/developer/translations?new=1").Body.String()
	for _, l := range []string{"es", "pt", "ja"} {
		if !strings.Contains(panel, `name="value_`+l+`"`) {
			t.Errorf("the new-text form offers %s", l)
		}
	}
	if strings.Contains(panel, `name="value_en"`) || strings.Contains(panel, `name="value_locale"`) {
		t.Errorf("the reference language has its own field, and the template is not a field")
	}
	loc := location(h.post(t, "/developer/translations/create", url.Values{"key": {key}, "value": {"Orders of %s"}, "value_es": {"Pedidos de %s"},
		"value_ja": {"  "}, "scope": {"tenant"}, "locale": {"en"}}))
	if !strings.Contains(loc, "msg=created") {
		t.Fatalf("create: %s", loc)
	}
	if got := h.tr.ForTenant(1).T("es", key, "mayo"); got != "Pedidos de mayo" {
		t.Errorf("the Spanish text is its own, got %q", got)
	}
	states := map[string]string{}
	for _, s := range h.localeStates(t, key) {
		states[s.Locale] = s.State
	}
	if states["es"] != textTranslated || states["pt"] == textTranslated || states["ja"] == textTranslated {
		t.Errorf("only the languages written are translated: %v", states)
	}
	// a translation that drops the marker is refused, with the key created and its editor open there
	loc = location(h.post(t, "/developer/translations/create", url.Values{"key": {"orders.list.other"}, "value": {"Orders of %s"}, "value_pt": {"Pedidos"},
		"scope": {"tenant"}, "locale": {"en"}}))
	if !strings.Contains(loc, "error=") || !strings.Contains(loc, "locale=pt") || !strings.Contains(loc, "key=orders.list.other") {
		t.Errorf("a refused language opens the editor there: %s", loc)
	}
	if got := h.tr.ForTenant(1).T("en", "orders.list.other", "May"); got != "Orders of May" {
		t.Errorf("the key itself was created, got %q", got)
	}
}

// The topbar's search field speaks the page's language.
func TestTopHeader_SearchPlaceholderIsTranslated(t *testing.T) {
	h := newDevHarness(t, false, false)
	for _, l := range []string{"en", "es", "pt", "ja"} {
		if got := h.tr.T(l, "layout.search.placeholder"); got == "layout.search.placeholder" || (l != "en" && got == h.tr.T("en", "layout.search.placeholder")) {
			t.Errorf("%s has no text of its own for the search field: %q", l, got)
		}
	}
	if body := h.get(t, "/developer/overview").Body.String(); strings.Contains(body, `placeholder="layout.search.placeholder"`) {
		t.Errorf("the placeholder is shown as a key")
	}
}
