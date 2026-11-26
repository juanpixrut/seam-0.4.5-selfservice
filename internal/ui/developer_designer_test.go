package ui

import (
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/ha1tch/seam-ui/internal/features"
	"github.com/ha1tch/seam-ui/internal/overrides"
)

func designerHarness(t *testing.T) (*devHarness, *overrides.MemoryStore) {
	t.Helper()
	devHarnessConfig = map[string]bool{features.DeveloperFormDesigner: true}
	t.Cleanup(func() { devHarnessConfig = nil })
	return scopedHarness(t)
}

var jsonArea = regexp.MustCompile(`(?s)<textarea[^>]*name="json"[^>]*>(.*?)</textarea>`)

func docOf(t *testing.T, body string) string {
	t.Helper()
	m := jsonArea.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no document in the page:\n%.400s", body)
	}
	return html.UnescapeString(m[1])
}

// designer drives the page the way a browser does: every step posts the document
// the previous page carried.
type designerSession struct {
	t    *testing.T
	h    *devHarness
	ref  string
	doc  string
	body string
	code int
}

func openDesigner(t *testing.T, h *devHarness, ref, scope string) *designerSession {
	rec := h.get(t, "/developer/designer?schema="+url.QueryEscape(ref)+"&scope="+scope)
	if rec.Code != http.StatusOK {
		t.Fatalf("opening %s: %d", ref, rec.Code)
	}
	return &designerSession{t: t, h: h, ref: ref, doc: docOf(t, rec.Body.String()), body: rec.Body.String(), code: rec.Code}
}

func (s *designerSession) do(act string, extra url.Values) *designerSession {
	s.t.Helper()
	v := url.Values{"schema": {s.ref}, "scope": {"file"}, "json": {s.doc}, "act": {act}}
	for k, vs := range extra {
		v[k] = vs
	}
	rec := s.h.post(s.t, "/developer/designer", v)
	s.body, s.code = rec.Body.String(), rec.Code
	if rec.Code == http.StatusOK {
		s.doc = docOf(s.t, s.body)
	}
	return s
}

func TestDesigner_IsOffUnlessItsFlagIsOn(t *testing.T) {
	h := newDevHarness(t, true, true)
	if rec := h.get(t, "/developer/designer"); rec.Code != http.StatusNotFound {
		t.Errorf("GET with the flag off: %d", rec.Code)
	}
	if rec := h.post(t, "/developer/designer", url.Values{"schema": {"system/user_create"}, "act": {"add"}}); rec.Code != http.StatusNotFound {
		t.Errorf("POST with the flag off: %d", rec.Code)
	}
	if body := h.get(t, "/developer/forms?schema=system/user_create").Body.String(); strings.Contains(body, "/developer/designer") {
		t.Errorf("no link to a page that is off")
	}
}

func TestDesigner_ChooserOpeningAndTheFormLink(t *testing.T) {
	h, _ := designerHarness(t)
	if body := h.get(t, "/developer/designer").Body.String(); !strings.Contains(body, "system/user_create") || strings.Contains(body, `name="json"`) {
		t.Errorf("with no form chosen the page offers the forms")
	}
	s := openDesigner(t, h, "system/user_create", "file")
	for _, want := range []string{`data-icon="short_text"`, `data-seam-select`, `name="act"`, "email", "password", `name="email"`} {
		if !strings.Contains(s.body, want) {
			t.Errorf("the designer page lacks %q", want)
		}
	}
	if !strings.Contains(h.get(t, "/developer/forms?schema=system/user_create").Body.String(), "/developer/designer?schema=system%2Fuser_create") {
		t.Errorf("the form editor links to the designer while it is on")
	}
	// anything that is not a catalogued form is the chooser, never an error or a read
	for _, bad := range []string{"../../etc/passwd", "system/../x", "nope/nope", "system/", "/user_create", "system/user_create/extra", strings.Repeat("a", 5000)} {
		rec := h.get(t, "/developer/designer?schema="+url.QueryEscape(bad))
		if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), `name="json"`) {
			t.Errorf("%.20q: %d", bad, rec.Code)
		}
	}
}

func TestDesigner_ActionsAndTheEngineHasTheLastWord(t *testing.T) {
	h, _ := designerHarness(t)
	s := openDesigner(t, h, "system/user_create", "file")

	s.do("add", url.Values{"new_widget": {"toggle"}, "new_name": {"notify_me"}})
	if !strings.Contains(s.doc, `"notify_me"`) || !strings.Contains(s.body, "Properties of notify_me") {
		t.Fatalf("add selects the new field and shows its properties")
	}
	s.do("add", url.Values{"new_widget": {"repeatable"}, "new_name": {"rows_x"}})
	s.do("up:notify_me", url.Values{"selected": {"notify_me"}})
	if strings.Index(s.doc, `"notify_me"`) > strings.Index(s.doc, `"rows_x"`) {
		t.Errorf("up moved it above the field after it")
	}

	// apply: label, help, required
	s.do("apply:notify_me", url.Values{"prop:title": {"Tell me"}, "prop:x-seam-help": {"We will email you"}, "required": {"on"}})
	if !strings.Contains(s.doc, `"title": "Tell me"`) || !strings.Contains(s.doc, `"x-seam-help": "We will email you"`) || !regexp.MustCompile(`(?s)"required":\s*\[[^\]]*"notify_me"`).MatchString(s.doc) {
		t.Errorf("apply wrote the label, the help and the required mark:\n%s", s.doc)
	}
	if !strings.Contains(s.body, "Tell me") {
		t.Errorf("and the preview shows the new label")
	}

	// a bad value is refused and the document is unchanged
	before := s.doc
	s.do("apply:rows_x", url.Values{"prop:minItems": {"abc"}})
	if s.doc != before || !strings.Contains(s.body, "was not applied") {
		t.Errorf("a value of the wrong kind must be refused and say why")
	}
	// the engine vetoes a change that parses as JSON but is not a form: min above max
	s.do("apply:rows_x", url.Values{"prop:minItems": {"5"}, "prop:maxItems": {"2"}})
	if s.doc != before || !strings.Contains(s.body, "greater than maxItems") {
		t.Errorf("the engine must veto minItems above maxItems, and the document stay as it was: %q", s.body[:min(len(s.body), 200)])
	}
	s.do("apply:rows_x", url.Values{"prop:minItems": {"1"}, "prop:maxItems": {"4"}, "prop:x-seam-reorder": {"on"}})
	if !strings.Contains(s.doc, `"minItems": 1`) || !strings.Contains(s.doc, `"x-seam-reorder": true`) {
		t.Errorf("a valid change goes through:\n%s", s.doc)
	}

	s.do("rename:rows_x", url.Values{"rename_to": {"lines"}})
	if !strings.Contains(s.doc, `"lines"`) || strings.Contains(s.doc, `"rows_x"`) {
		t.Errorf("rename")
	}
	s.do("remove:lines", nil)
	s.do("remove:notify_me", nil)
	if strings.Contains(s.doc, `"lines"`) || strings.Contains(s.doc, `"notify_me"`) {
		t.Errorf("remove")
	}
	s.do("add", url.Values{"new_widget": {"nonsense"}, "new_name": {"x"}})
	s.do("add", url.Values{"new_widget": {"text"}, "new_name": {"Bad Name"}})
	s.do("add", url.Values{"new_widget": {"text"}, "new_name": {"email"}}) // taken
	s.do("explode:email", nil)
	if strings.Count(s.doc, `"email"`) != strings.Count(before, `"email"`) {
		t.Errorf("refused additions and unknown actions change nothing")
	}
}

func TestDesigner_SavesThroughTheExistingRouteAndTheEngineUsesIt(t *testing.T) {
	h, store := designerHarness(t)
	s := openDesigner(t, h, "system/user_create", "file")
	s.do("add", url.Values{"new_widget": {"textarea"}, "new_name": {"remarks"}})
	s.do("apply:remarks", url.Values{"prop:title": {"Remarks"}, "prop:x-seam-rows": {"6"}})
	rec := h.post(t, "/developer/forms/save", url.Values{"schema": {s.ref}, "json": {s.doc}, "scope": {"file"}})
	if !strings.Contains(location(rec), "msg=saved") {
		t.Fatalf("save: %s", location(rec))
	}
	if got := string(h.rt.Resolver.Resolve(1, "system", "user_create", []byte(`{}`))); !strings.Contains(got, "remarks") {
		t.Errorf("the saved form is the live one")
	}
	// the tenant scope keeps its history
	s2 := openDesigner(t, h, "system/user_create", "tenant")
	s2.do("add", url.Values{"new_widget": {"date"}, "new_name": {"start_on"}, "scope": {"tenant"}})
	rec = h.post(t, "/developer/forms/save", url.Values{"schema": {s2.ref}, "json": {s2.doc}, "scope": {"tenant"}, "note": {"from the designer"}})
	if !strings.Contains(location(rec), "msg=saved") {
		t.Fatalf("tenant save: %s", location(rec))
	}
	if hist, _ := store.SchemaHistory(ctx_, 1, "system", "user_create"); len(hist) != 1 || hist[0].Note != "from the designer" {
		t.Errorf("the designer's save is a version with its note: %+v", hist)
	}
}

func TestDesigner_HostileInputIsEscapedAndBounded(t *testing.T) {
	h, _ := designerHarness(t)
	s := openDesigner(t, h, "system/user_create", "file")
	s.do("add", url.Values{"new_widget": {"text"}, "new_name": {"xss_field"}})
	s.do("apply:xss_field", url.Values{"prop:title": {`<script>alert(1)</script>`}, "prop:x-seam-help": {`"><img src=x onerror=alert(2)>`}})
	for _, raw := range []string{"<script>alert(1)", "<img src=x"} {
		if strings.Contains(s.body, raw) {
			t.Errorf("unescaped %q in the page", raw)
		}
	}
	// a hostile or broken document is replaced by the form's current one, with a reason
	rec := h.post(t, "/developer/designer", url.Values{"schema": {s.ref}, "json": {`{"properties":`}, "act": {"select:email"}})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "was not applied") || !strings.Contains(docOf(t, rec.Body.String()), `"email"`) {
		t.Errorf("a broken document must fall back to the stored form: %d", rec.Code)
	}
	big := `{"x":"` + strings.Repeat("a", 2<<20) + `"}`
	req := httptest.NewRequest("POST", "/developer/designer", strings.NewReader(url.Values{"schema": {s.ref}, "json": {big}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sessionCookie(t, h.sess, "developer"))
	big2 := httptest.NewRecorder()
	h.mux.ServeHTTP(big2, req)
	if big2.Code == http.StatusOK && strings.Contains(big2.Body.String(), `name="json"`) {
		t.Errorf("an oversize document must not be opened")
	}
	if rec := h.do(t, "POST", "/developer/designer", "admin", url.Values{"schema": {s.ref}}); rec.Code != http.StatusForbidden {
		t.Errorf("only developers: %d", rec.Code)
	}
}
