package ui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"

	"github.com/ha1tch/seam-ui/internal/auth"
	fe "github.com/ha1tch/seam-ui/internal/formengine"
	demoschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/demo"
	"github.com/ha1tch/seam-ui/internal/i18n"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

func demoTestMux(t *testing.T, enable bool) (*http.ServeMux, *auth.SessionManager) {
	t.Helper()
	translator, err := i18n.New("en")
	if err != nil {
		t.Fatalf("i18n.New: %v", err)
	}
	sessionMgr := auth.NewSessionManager(auth.SessionConfig{Secret: "test-secret-demo"})
	cfg := Config{AppName: "Test", Version: "0.0.0", Locale: "en", EnableDemo: enable}
	router := NewRouter(xolu.New("http://127.0.0.1:1"), cfg, translator, nil, sessionMgr, nil, nil, nil, nil)
	mux := http.NewServeMux()
	router.MountDemoRoutes(mux)
	mux.Handle("/", router)
	return mux, sessionMgr
}

func TestDemoRoutes_GatedByFlagAndAuth(t *testing.T) {
	mux, _ := demoTestMux(t, false)
	for _, c := range []struct{ method, path string }{{"GET", "/demo"}, {"POST", "/demo/echo"}} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(c.method, c.path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s %s with EnableDemo=false: got %d, want 404", c.method, c.path, rec.Code)
		}
	}
	mux, _ = demoTestMux(t, true)
	for _, c := range []struct{ method, path string }{{"GET", "/demo"}, {"POST", "/demo/echo"}} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(c.method, c.path, nil))
		if rec.Code != http.StatusSeeOther {
			t.Errorf("%s %s unauthenticated: got %d, want 303 to login", c.method, c.path, rec.Code)
		}
	}
}

// Every role reaches the page (requireAuth only), and the page carries both
// tabs, both explanatory texts and the real engine's output for each form.
func TestDemoIndex_RendersBothTabsForEveryRole(t *testing.T) {
	for _, role := range auth.ValidRoles() {
		// Components is on the Pickers page, Repeatables on the Rows page
		body := demoGroupBody(t, "pickers", role) + demoGroupBody(t, "rows", role)
		for _, want := range []string{
			`id="demo-tab-components"`, `id="demo-tab-repeatable"`,
			"Components", "Repeatables", // tab captions
			"Demonstrates the Selector dialog",     // components text
			"Answer to the Work Orders developers", // repeatable text
			"use indexed field names",
			`action="/demo/echo"`,
			// components form: any-picker, one with a Browse button into the Selector
			`name="owner_id"`, `name="reviewer_id"`, `data-selector="users"`, "SeamPicker.browse(this)",
			// repeatable form: indexed names for blocks (with a picker and nested lines)...
			`name="blocks[0][name]"`, `name="blocks[0][responsible_id]"`,
			`name="blocks[0][lines][0][name]"`, `name="blocks[0][lines][0][type]"`,
			`name="blocks[__D0__][lines][__D1__][name]"`, "SeamRepeatable.add(this)",
			// ...and the unchanged flat names for a repeatable without nesting
			`name="workflow_states_name[]"`,
			// the new features, shown in context
			`name="code"`, `placeholder="e.g. SN-0042"`, // x-seam-placeholder on a plain field
			`name="icon"`, `data-endpoint="/picker/icons"`, "SeamPicker.toggleGrid(this)", // icon-picker with its grid
			`name="blocks[0][icon]"`, `placeholder="Block name (e.g. Preparation)"`, `placeholder="Tag (e.g. serial_no)"`,
			">Lines<", `class="fe-rep-nested"`, // nested repeatable: heading and rule
			`data-layout="form"`, "fe-cell-label", // Blocks use x-seam-layout: form
			`class="fe-btn fe-btn--icon"`, "manage_search", `data-hl="background"`, // Reviewer: icon Browse, marker highlight
			"Each cell names itself", "the icon-picker", // the explanatory texts mention them
			// the repeatable form can be saved as a demo record
			`name="name"`, `formaction="/demo/templates"`, "Saved demo records", "Save as demo record",
			// the reported shape is shown as text
			"line_items",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("role %s: page missing %s", role, want)
			}
		}
		if strings.Contains(body, "demo.components.intro") || strings.Contains(body, "demo.repeatable.intro") {
			t.Errorf("role %s: untranslated demo keys on the page", role)
		}
	}
}

func TestDemoEcho_ShowsPostedValuesWithoutCSRFToken(t *testing.T) {
	mux, sessionMgr := demoTestMux(t, true)
	form := url.Values{
		"blocks[0][name]":           {"Prep"},
		"blocks[0][lines][0][name]": {"t1"},
		"blocks[0][lines][0][type]": {"Text"},
		"blocks[2][name]":           {"Run"},
		"workflow_states_name[]":    {"Open", "Closed"},
		"_csrf":                     {"csrf-secret-123"},
	}
	req := httptest.NewRequest("POST", "/demo/echo", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sessionCookie(t, sessionMgr, auth.RoleViewer))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"blocks[0][lines][0][name]", "workflow_states_name[]", "Prep", "Run", "Posted values",
		"Structured value rebuilt by the server",
		`&#34;lines&#34;: [`, // the structure was rebuilt: lines nested under their block
	} {
		if !strings.Contains(body, want) {
			t.Errorf("echo page missing %s", want)
		}
	}
	if strings.Contains(body, "csrf-secret-123") {
		t.Errorf("the CSRF token must not be echoed")
	}
}

func TestDemoEcho_EmptyPostShowsEmptyNotice(t *testing.T) {
	mux, sessionMgr := demoTestMux(t, true)
	req := httptest.NewRequest("POST", "/demo/echo", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sessionCookie(t, sessionMgr, auth.RoleViewer))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "(no values posted)") {
		t.Errorf("expected the empty notice: %s", rec.Body.String())
	}
}

// repeatablesWithoutRowFields lists every repeatable field (at any depth)
// that has no row fields: the defect the Work Orders developers reported.
func repeatablesWithoutRowFields(fields []fe.Field) []string {
	var bad []string
	for _, f := range fields {
		if f.XSeamInput == "repeatable" && len(f.Items) == 0 {
			bad = append(bad, f.Name)
		}
		bad = append(bad, repeatablesWithoutRowFields(f.Items)...)
	}
	return bad
}

// The corrected schema must give every repeatable (nested ones included)
// row fields. This is the regression test for the reported defect.
func TestDemoSchemas_CorrectedRepeatableHasRowFieldsAtEveryDepth(t *testing.T) {
	for _, name := range []string{"components", "repeatable"} {
		form, err := fe.ParseSchema(demoschemas.Schema(name))
		if err != nil {
			t.Fatalf("%s: ParseSchema: %v", name, err)
		}
		if bad := repeatablesWithoutRowFields(form.Fields); len(bad) != 0 {
			t.Errorf("%s: repeatables without row fields: %v", name, bad)
		}
	}
	form, _ := fe.ParseSchema(demoschemas.Schema("repeatable"))
	var blocks *fe.Field
	for i := range form.Fields {
		if form.Fields[i].Name == "blocks" {
			blocks = &form.Fields[i]
		}
	}
	if blocks == nil || len(blocks.Items) != 4 || blocks.Items[1].Name != "responsible_id" || blocks.Items[2].Name != "icon" || blocks.Items[3].Name != "lines" || len(blocks.Items[3].Items) != 2 {
		t.Errorf("blocks must have 4 row fields (name, responsible_id, icon, lines), lines having 2: %+v", blocks)
	}
}

// The reported shape is kept only as text. If the engine can parse it, it
// must show the defect (a repeatable with no row fields); if it cannot, say so.
func TestDemoSchemas_ReportedShapeShowsTheDefect(t *testing.T) {
	form, err := fe.ParseSchema(demoschemas.Schema("repeatable_bug"))
	if err != nil {
		t.Logf("the reported schema does not parse in the engine (%v); it is only displayed as text", err)
		return
	}
	if bad := repeatablesWithoutRowFields(form.Fields); len(bad) == 0 {
		t.Errorf("the reported schema was expected to contain a repeatable without row fields")
	}
}

// The Demo code blocks take their colours from the theme (CSS class), not from
// an inline style that could leave dark text on a dark background.
func TestDemoCode_UsesTheThemedClass(t *testing.T) {
	html := mi.RenderToString(func(b *mi.Builder) mi.Node { return demoCode(b, "x") })
	if !strings.Contains(html, `class="fe-code"`) || strings.Contains(html, "style=") {
		t.Errorf("demoCode must use the fe-code class and no inline style: %s", html)
	}
}
