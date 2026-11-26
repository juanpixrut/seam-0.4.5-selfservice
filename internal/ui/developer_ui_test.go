package ui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"
)

// The designer's buttons must work whatever the preview holds (T-114): the live
// preview's inputs carry "required", and when they were inside the designer's
// form the browser refused every submit until they were filled in.
func TestDesigner_ThePreviewIsOutsideTheDesignersForm(t *testing.T) {
	h, _ := designerHarness(t)
	body := openDesigner(t, h, "system/user_create", "file").body

	form := regexp.MustCompile(`(?s)<form[^>]*id="` + designerFormID + `"[^>]*>(.*?)</form>`).FindStringSubmatch(body)
	if form == nil {
		t.Fatal("the designer's form is not on the page")
	}
	if regexp.MustCompile(`<(?:input|select|textarea)[^>]*\srequired[\s=>/]`).MatchString(form[1]) || strings.Contains(form[1], `name="email"`) {
		t.Errorf("the designer's form must hold only what travels with every action:\n%s", form[1])
	}
	if !strings.Contains(body, `name="email"`) {
		t.Fatal("the preview must still render the form's own fields")
	}
	// every submit button names the designer's form and skips validation
	buttons := regexp.MustCompile(`<button[^>]*type="submit"[^>]*>`).FindAllString(body, -1)
	if len(buttons) < 6 {
		t.Fatalf("want the designer's buttons, got %d", len(buttons))
	}
	for _, btn := range buttons {
		if !strings.Contains(btn, `form="`+designerFormID+`"`) || !strings.Contains(btn, "formnovalidate") {
			t.Errorf("a designer button that the preview could block: %s", btn)
		}
	}
	// and every control of the designer does too: none is left without a form
	for _, name := range []string{"new_widget", "new_name"} {
		ctl := regexp.MustCompile(`<(?:input|select|textarea)[^>]*name="` + name + `"[^>]*>`).FindString(body)
		if !strings.Contains(ctl, `form="`+designerFormID+`"`) {
			t.Errorf("%s does not belong to the designer's form: %s", name, ctl)
		}
	}
	// a field of the form being designed may be named like one of the designer's
	// own (note, schema, json): it must not reach the POST
	s := openDesigner(t, h, "system/user_create", "file").do("add", url.Values{"new_name": {"note"}, "new_widget": {"text"}})
	if s.code != http.StatusOK || !strings.Contains(s.doc, `"note"`) {
		t.Fatalf("adding a field named note: %d", s.code)
	}
	form = regexp.MustCompile(`(?s)<form[^>]*id="` + designerFormID + `"[^>]*>(.*?)</form>`).FindStringSubmatch(s.body)
	if form == nil || strings.Contains(form[1], `name="note"`) {
		t.Errorf("the previewed field named note must stay outside the designer's form")
	}
}

// The region, currency and brand forms are rendered by the form engine on
// their live routes, so an override of their definition reaches them (T-113):
// they used to be hand-built, and an override was saved, listed and ignored.
func TestMasters_OverridesReachTheLiveForms(t *testing.T) {
	h, _ := designerHarness(t)
	for _, c := range []struct{ form, path, field string }{
		{"region", "/regions/new", "description"},
		{"moneda", "/currencies/new", "iso_code"},
		{"marca", "/brands/new", "code"},
	} {
		before := h.get(t, c.path)
		if before.Code != http.StatusOK || !strings.Contains(before.Body.String(), `name="`+c.field+`"`) || strings.Contains(before.Body.String(), "Zz Extra Field") {
			t.Fatalf("%s: the built-in form must render its own field (%d)", c.path, before.Code)
		}
		base, _ := developerEmbeddedSchema("masters", c.form)
		doc := strings.Replace(string(base), `"properties": {`, `"properties": {"zzextra":{"type":"string","title":"Zz Extra Field","x-seam-input":"text"},`, 1)
		if rec := h.post(t, "/developer/forms/save", url.Values{"schema": {"masters/" + c.form}, "scope": {"file"}, "json": {doc}}); !strings.Contains(location(rec), "msg=saved") {
			t.Fatalf("%s: save: %s", c.form, location(rec))
		}
		after := h.get(t, c.path).Body.String()
		if !strings.Contains(after, "Zz Extra Field") || !strings.Contains(after, `name="`+c.field+`"`) {
			t.Errorf("%s: an override of masters/%s must show on the live form", c.path, c.form)
		}
		// one CSRF field, the page's own: the engine is not given the token
		if n := strings.Count(after, `name="_csrf"`); n != 1 {
			t.Errorf("%s: want one CSRF field, got %d", c.path, n)
		}
	}
}

// The forms list narrows by module and by part of a name, and ignores a module
// the catalogue does not have.
func TestForms_ListFilters(t *testing.T) {
	h, _ := designerHarness(t)
	rows := func(q string) int {
		return strings.Count(h.get(t, "/developer/forms"+q).Body.String(), `data-row-link="1"`)
	}
	all := rows("")
	if all != len(developerEmbeddedSchemas()) {
		t.Fatalf("want every form listed, got %d of %d", all, len(developerEmbeddedSchemas()))
	}
	if n := rows("?module=system"); n == 0 || n >= all {
		t.Errorf("module=system: %d of %d", n, all)
	}
	// by name whatever the case; the search also finds forms by what they use,
	// so a name that is nobody's field, widget or source isolates the two
	if n := rows("?q=USER_CREA"); n != 1 {
		t.Errorf("q=USER_CREA must match user_create whatever the case, got %d", n)
	}
	if n := rows("?q=USER_"); n < 2 || n >= all {
		t.Errorf("q=USER_ must match user_create, user_edit and the forms with such a field, got %d", n)
	}
	if n := rows("?module=nope"); n != all {
		t.Errorf("an unknown module is no filter, got %d", n)
	}
	if body := h.get(t, "/developer/forms?q=zzzz").Body.String(); strings.Contains(body, "<table") || !strings.Contains(body, "No form matches.") {
		t.Errorf("no match must show the empty state")
	}
	if body := h.get(t, `/developer/forms?q=`+url.QueryEscape(`"><script>alert(1)</script>`)).Body.String(); strings.Contains(body, "<script>alert(1)") {
		t.Errorf("the search text must be escaped")
	}
}

// The translations list's "show" filter keeps only what a count of the
// completeness report, or an override, refers to; the counts link to it.
func TestTranslations_ShowFilter(t *testing.T) {
	h := newDevHarness(t, true, true)
	key := "developer.nav.overview"
	count := regexp.MustCompile(`· (\d+) texts`)
	total := func(q string) string {
		m := count.FindStringSubmatch(h.get(t, "/developer/translations"+q).Body.String())
		if m == nil {
			return ""
		}
		return m[1]
	}
	all := total("?locale=en")
	if all == "" || all == "0" {
		t.Fatalf("the list must count its texts, got %q", all)
	}
	if got := total("?locale=en&show=overridden"); got != "" {
		t.Errorf("nothing is overridden yet, got %q", got)
	}
	if rec := h.post(t, "/developer/translations/save", url.Values{"locale": {"en"}, "key": {key}, "value": {"Home"}, "show": {"overridden"}}); !strings.Contains(location(rec), "msg=saved") || !strings.Contains(location(rec), "show=overridden") {
		t.Fatalf("a save returns to the same filter: %s", location(rec))
	}
	body := h.get(t, "/developer/translations?locale=en&show=overridden").Body.String()
	if m := count.FindStringSubmatch(body); m == nil || m[1] != "1" || !strings.Contains(body, "key="+key) {
		t.Errorf("the overridden filter must list exactly the overridden text")
	}
	if got := total("?locale=en&show=nonsense"); got != all {
		t.Errorf("an unknown filter value is no filter: %q, want %q", got, all)
	}
	// a count of the completeness report that is above zero links to its texts
	if body := h.get(t, "/developer/translations").Body.String(); strings.Contains(body, "dev-num-warn") && !strings.Contains(body, "show=") {
		t.Errorf("a count above zero must link to the texts it counts")
	}
}

// The Overview's tiles are the way into each section, and its override
// directories can be reloaded from there and return there.
func TestOverview_TilesAndReloadFromTheOverview(t *testing.T) {
	h := newDevHarness(t, true, true)
	body := h.get(t, "/developer/overview").Body.String()
	if strings.Count(body, "dev-stat") != 4 {
		t.Errorf("want four tiles, got %d", strings.Count(body, "dev-stat"))
	}
	backs := regexp.MustCompile(`(?s)<form[^>]*action="/developer/reload"[^>]*>.*?</form>`).FindAllString(body, -1)
	if len(backs) != 2 || !strings.Contains(backs[0], `value="overview"`) || !strings.Contains(backs[1], `value="overview"`) {
		t.Errorf("both directories must be reloadable from the Overview and return to it")
	}
	rec := h.post(t, "/developer/reload", url.Values{"target": {"locales"}, "back": {"overview"}})
	if !strings.HasPrefix(location(rec), "/developer/overview?msg=reloaded") {
		t.Errorf("a reload from the Overview returns to it: %s", location(rec))
	}
}

// headerCounter counts the WriteHeader calls a handler makes.
type headerCounter struct {
	http.ResponseWriter
	calls int
}

func (c *headerCounter) WriteHeader(status int) {
	c.calls++
	c.ResponseWriter.WriteHeader(status)
}

// A page that answers with a status of its own writes it once, with its
// Content-Type (T-117): the refused form save used to write 422 itself and then
// let the renderer write 200, which the server logged as superfluous.
func TestRenderStatus_WritesTheStatusOnce(t *testing.T) {
	page := &PageHandler{Config: Config{AppName: "Seam"}}
	content := func(b *mi.Builder) mi.Node { return b.Div("refused") }
	for _, htmx := range []bool{false, true} {
		rec := httptest.NewRecorder()
		w := &headerCounter{ResponseWriter: rec}
		r := httptest.NewRequest("POST", "/developer/forms/save", nil)
		if htmx {
			r.Header.Set("HX-Request", "true")
		}
		if err := page.RenderStatus(w, r, http.StatusUnprocessableEntity, "Forms", "/developer/forms", content); err != nil {
			t.Fatal(err)
		}
		if w.calls != 1 || rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("htmx=%v: %d WriteHeader call(s), status %d", htmx, w.calls, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") || !strings.Contains(rec.Body.String(), "refused") {
			t.Errorf("htmx=%v: Content-Type %q", htmx, ct)
		}
	}
	// and Render is the same page with 200
	rec := httptest.NewRecorder()
	w := &headerCounter{ResponseWriter: rec}
	if err := page.Render(w, httptest.NewRequest("GET", "/x", nil), "T", "/x", content); err != nil || w.calls != 1 || rec.Code != http.StatusOK {
		t.Errorf("Render: %v, %d call(s), status %d", err, w.calls, rec.Code)
	}
}

// Every schema package is either in the Developer catalogue or left out by
// name with a reason (T-116): eight packages were once in neither, so their
// forms could not be overridden and nobody was told.
func TestCatalogue_CoversEverySchemaPackage(t *testing.T) {
	entries, err := os.ReadDir("../formengine/schemas")
	if err != nil {
		t.Fatal(err)
	}
	inCatalogue := map[string]bool{}
	for _, m := range developerSchemaModules {
		inCatalogue[m.module] = true
		if len(m.names()) == 0 {
			t.Errorf("%s is in the catalogue but lists no form", m.module)
		}
		for _, n := range m.names() {
			if doc, ok := developerEmbeddedSchema(m.module, n); !ok || len(doc) == 0 {
				t.Errorf("%s/%s cannot be read", m.module, n)
			}
		}
	}
	dirs := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dirs++
		_, out := developerSchemaPackagesLeftOut[e.Name()]
		switch {
		case inCatalogue[e.Name()] && out:
			t.Errorf("%s is both in the catalogue and left out", e.Name())
		case !inCatalogue[e.Name()] && !out:
			t.Errorf("schema package %s is neither in the Developer catalogue nor in developerSchemaPackagesLeftOut", e.Name())
		}
	}
	if dirs != len(developerSchemaModules)+len(developerSchemaPackagesLeftOut) {
		t.Errorf("%d schema directories, %d catalogued and %d left out", dirs, len(developerSchemaModules), len(developerSchemaPackagesLeftOut))
	}
}
