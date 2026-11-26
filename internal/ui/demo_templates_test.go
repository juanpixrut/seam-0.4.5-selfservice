package ui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/ha1tch/seam-ui/internal/auth"
)

// Demo records live only in the demo_templates entity. These tests run the
// handlers against a recording fake xolu and assert both what they do and,
// above all, that they never touch any other entity.

type demoXoluLog struct {
	mu      sync.Mutex
	calls   []string // "METHOD /path"
	bodies  []string
	queries []string // OQL text
}

func (l *demoXoluLog) count(prefix string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, c := range l.calls {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}

// newDemoXolu serves: OQL (answering oqlRows), POST create (id 42), GET one
// (getData, or 404 when nil) and DELETE (200).
func newDemoXolu(t *testing.T, log *demoXoluLog, oqlRows []map[string]any, getData map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		log.mu.Lock()
		log.calls = append(log.calls, r.Method+" "+r.URL.Path)
		log.bodies = append(log.bodies, string(raw))
		if r.URL.Path == "/api/v1/oql/query" {
			var q struct {
				Query string `json:"query"`
			}
			_ = json.Unmarshal(raw, &q)
			log.queries = append(log.queries, q.Query)
		}
		log.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/oql/query":
			rows := oqlRows
			if rows == nil {
				rows = []map[string]any{}
			}
			json.NewEncoder(w).Encode(map[string]any{"status": "ok", "data": rows, "stats": map[string]any{}})
		case r.Method == "POST":
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"id": float64(42), "message": "created"})
		case r.Method == "GET":
			if getData == nil {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			json.NewEncoder(w).Encode(getData)
		case r.Method == "DELETE":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"message":"deleted"}`))
		default:
			http.NotFound(w, r)
		}
	}))
}

// chromeSettingsRead reports whether q is one of the page layout's own
// read-only settings queries (the tenant's general and features rows). Every
// page render makes them, whichever handler produced the page; they are not the
// demo handler's, and they are the one exception the demo's isolation checks
// allow. A write to tenant_settings, or any other entity, is still refused.
func chromeSettingsRead(q string) bool {
	return strings.HasPrefix(q, "SELECT TOP 1 * FROM tenant_settings WHERE key = '") && strings.Count(strings.ToUpper(q), "FROM ") == 1
}

// kitchenSourceRead is the one read the Kitchen tabs' foreign-key select makes
// (its x-seam-source options). It is not part of the saved-records list, so
// the list's isolation assertions leave it out, like the layout's own reads.
func kitchenSourceRead(q string) bool {
	return q == "SELECT * FROM brands ORDER BY name"
}

// demoQueries returns the recorded OQL queries without the layout's settings reads
// and the Kitchen source read.
func demoQueries(log *demoXoluLog) []string {
	log.mu.Lock()
	defer log.mu.Unlock()
	var out []string
	for _, q := range log.queries {
		if !chromeSettingsRead(q) && !kitchenSourceRead(q) {
			out = append(out, q)
		}
	}
	return out
}

// demoCalls returns what the demo handler itself called: every recorded call
// except the OQL calls (which demoQueries accounts for, without the layout's
// settings reads), then one entry per remaining query.
func demoCalls(log *demoXoluLog) []string {
	log.mu.Lock()
	var out []string
	for _, c := range log.calls {
		if c != "POST /api/v1/oql/query" {
			out = append(out, c)
		}
	}
	log.mu.Unlock()
	for _, q := range demoQueries(log) {
		out = append(out, "OQL "+q)
	}
	return out
}

// assertOnlyDemoEntity fails if any recorded call or query names an entity
// other than demo_templates (apart from the layout's settings reads, see
// chromeSettingsRead).
func assertOnlyDemoEntity(t *testing.T, log *demoXoluLog) {
	t.Helper()
	log.mu.Lock()
	defer log.mu.Unlock()
	for _, c := range log.calls {
		path := c[strings.Index(c, " ")+1:]
		if path == "/api/v1/oql/query" {
			continue
		}
		if !strings.HasPrefix(path, "/api/v1/"+demoEntity) {
			t.Errorf("the demo touched something other than %s: %s", demoEntity, c)
		}
	}
	for _, q := range log.queries {
		if chromeSettingsRead(q) || kitchenSourceRead(q) {
			continue
		}
		if !strings.Contains(q, "FROM "+demoEntity) || strings.Count(strings.ToUpper(q), "FROM ") != 1 {
			t.Errorf("an OQL query reaches beyond %s: %s", demoEntity, q)
		}
	}
}

func demoClaims(r *http.Request, tenant int64) *http.Request {
	return r.WithContext(auth.ContextWithClaims(r.Context(), &auth.Claims{UserID: 9, TenantID: tenant, Role: auth.RoleViewer}))
}

func demoPost(h http.HandlerFunc, target string, form url.Values, tenant int64, pathID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if pathID != "" {
		req.SetPathValue("id", pathID)
	}
	if tenant != 0 {
		req = demoClaims(req, tenant)
	}
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func TestDemoTemplateSave_StoresRebuiltStructureInTheDemoEntityOnly(t *testing.T) {
	log := &demoXoluLog{}
	srv := newDemoXolu(t, log, nil, nil)
	defer srv.Close()
	h := newHandler(t, srv)

	form := url.Values{
		"name":                      {"Tpl A"},
		"blocks[0][name]":           {"Prep"},
		"blocks[0][responsible_id]": {"7"},
		"blocks[0][lines][0][name]": {"t1"},
		"blocks[0][lines][0][type]": {"Text"},
		"blocks[3][name]":           {"Run"},
		"workflow_states_name[]":    {"Open", "", "Closed"},
		"_csrf":                     {"tok"},
	}
	rec := demoPost(h.DemoTemplateSave, "/demo/templates", form, 5, "")
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/demo/rows?tab=repeatable" {
		t.Fatalf("got %d, Location %q", rec.Code, rec.Header().Get("Location"))
	}
	if log.count("POST /api/v1/"+demoEntity) != 1 {
		t.Fatalf("expected exactly one create in %s, calls: %v", demoEntity, log.calls)
	}
	body := strings.Join(log.bodies, "\n")
	for _, want := range []string{
		`"tenant_id":5`, `"demo":true`, `"name":"Tpl A"`, `"created_by":9`,
		`"blocks":[`, `"lines":[`, `"Prep"`, `"Run"`, `"t1"`, `"Open"`, `"Closed"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("stored record missing %s: %s", want, body)
		}
	}
	if strings.Contains(body, "csrf") || strings.Contains(body, "tok") {
		t.Errorf("the CSRF token must not be stored: %s", body)
	}
	assertOnlyDemoEntity(t, log)
}

func TestDemoTemplateSave_RejectsMissingNameAndMissingTenant(t *testing.T) {
	log := &demoXoluLog{}
	srv := newDemoXolu(t, log, nil, nil)
	defer srv.Close()
	h := newHandler(t, srv)

	if rec := demoPost(h.DemoTemplateSave, "/demo/templates", url.Values{"name": {"   "}}, 5, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("blank name: got %d, want 400", rec.Code)
	}
	if rec := demoPost(h.DemoTemplateSave, "/demo/templates", url.Values{"name": {"X"}}, 0, ""); rec.Code != http.StatusForbidden {
		t.Errorf("no tenant: got %d, want 403", rec.Code)
	}
	if calls := demoCalls(log); len(calls) != 0 {
		t.Errorf("nothing may be stored on a rejected save: %v", calls)
	}
}

func TestDemoIndex_ListsOnlyTheTenantsDemoRecords(t *testing.T) {
	log := &demoXoluLog{}
	rows := []map[string]any{{
		"id": 7.0, "name": "Tpl A", "created_at": "2026-10-08T00:00:00Z", "tenant_id": 5.0, "demo": true,
		"blocks":          []any{map[string]any{"name": "Prep", "lines": []any{map[string]any{"name": "t1", "type": "Text"}}}},
		"workflow_states": []any{map[string]any{"name": "Open"}},
	}}
	srv := newDemoXolu(t, log, rows, nil)
	defer srv.Close()
	h := newHandler(t, srv)

	req := demoClaims(httptest.NewRequest("GET", "/demo/rows?tab=reorder", nil), 5)
	req.SetPathValue("group", "rows")
	rec := httptest.NewRecorder()
	h.DemoGroupPage(rec, req)
	body := rec.Body.String()

	for _, want := range []string{
		"#7 Tpl A", `&#34;lines&#34;: [`, `action="/demo/templates/7/delete"`, `action="/demo/templates/reset"`,
		"demo.saved.heading",                         // keys, as the test handler has no translator
		`.demo-tab-btn[data-tab="demo-tab-reorder"]`, // opens the tab the address asks for
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page missing %s", want)
		}
	}
	if qs := demoQueries(log); len(qs) != 1 || !strings.Contains(qs[0], "FROM demo_templates WHERE tenant_id = 5") {
		t.Errorf("the list must be filtered by tenant: %v", qs)
	}
	assertOnlyDemoEntity(t, log)
}

func TestDemoIndex_NoSessionTenantListsNothingAndDoesNotQuery(t *testing.T) {
	log := &demoXoluLog{}
	srv := newDemoXolu(t, log, []map[string]any{{"id": 1.0, "name": "X", "tenant_id": 5.0, "demo": true}}, nil)
	defer srv.Close()
	h := newHandler(t, srv)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/demo/rows", nil)
	req.SetPathValue("group", "rows")
	h.DemoGroupPage(rec, req)
	if qs := demoQueries(log); len(qs) != 0 || strings.Contains(rec.Body.String(), "#1 X") {
		t.Errorf("without a tenant nothing may be listed or queried: %v", qs)
	}
	if !strings.Contains(rec.Body.String(), "demo.saved.empty") {
		t.Errorf("expected the empty notice")
	}
}

func TestDemoIndex_OtherTabValuesDoNotInjectScript(t *testing.T) {
	log := &demoXoluLog{}
	srv := newDemoXolu(t, log, nil, nil)
	defer srv.Close()
	h := newHandler(t, srv)
	// neither on a page, nor on the overview, nor for a tab of another page
	for _, target := range []string{"/demo/rows?tab=" + url.QueryEscape(`x');alert(1);//`), "/demo?tab=" + url.QueryEscape(`x');alert(1);//`), "/demo/rows?tab=colors", "/demo/rows?tab=repeatable"} {
		req := demoClaims(httptest.NewRequest("GET", target, nil), 5)
		rec := httptest.NewRecorder()
		if strings.HasPrefix(target, "/demo/rows") {
			req.SetPathValue("group", "rows")
			h.DemoGroupPage(rec, req)
		} else {
			h.DemoIndex(rec, req)
		}
		if strings.Contains(rec.Body.String(), "alert(1)") || strings.Contains(rec.Body.String(), "tb.click") {
			t.Errorf("%s: an unknown tab, another page's tab or the first tab must produce no script", target)
		}
	}
}

func TestDemoTemplateDelete_OnlyOwnedDemoRecords(t *testing.T) {
	cases := []struct {
		name       string
		data       map[string]any
		wantStatus int
		wantDelete int
	}{
		{"own demo record", map[string]any{"id": 7.0, "tenant_id": 5.0, "demo": true}, http.StatusSeeOther, 1},
		{"other tenant", map[string]any{"id": 7.0, "tenant_id": 6.0, "demo": true}, http.StatusNotFound, 0},
		{"not marked demo", map[string]any{"id": 7.0, "tenant_id": 5.0}, http.StatusNotFound, 0},
		{"demo flag not boolean", map[string]any{"id": 7.0, "tenant_id": 5.0, "demo": "true"}, http.StatusNotFound, 0},
		{"missing record", nil, http.StatusNotFound, 0},
	}
	for _, c := range cases {
		log := &demoXoluLog{}
		srv := newDemoXolu(t, log, nil, c.data)
		h := newHandler(t, srv)
		rec := demoPost(h.DemoTemplateDelete, "/demo/templates/7/delete", url.Values{}, 5, "7")
		if rec.Code != c.wantStatus {
			t.Errorf("%s: got %d, want %d", c.name, rec.Code, c.wantStatus)
		}
		if got := log.count("DELETE "); got != c.wantDelete {
			t.Errorf("%s: %d deletes, want %d", c.name, got, c.wantDelete)
		}
		assertOnlyDemoEntity(t, log)
		srv.Close()
	}
}

func TestDemoTemplateDelete_BadIDOrNoTenantDoesNothing(t *testing.T) {
	log := &demoXoluLog{}
	srv := newDemoXolu(t, log, nil, map[string]any{"id": 7.0, "tenant_id": 5.0, "demo": true})
	defer srv.Close()
	h := newHandler(t, srv)
	for _, id := range []string{"abc", "0", "-3", ""} {
		if rec := demoPost(h.DemoTemplateDelete, "/demo/templates/x/delete", url.Values{}, 5, id); rec.Code != http.StatusNotFound {
			t.Errorf("id %q: got %d, want 404", id, rec.Code)
		}
	}
	if rec := demoPost(h.DemoTemplateDelete, "/demo/templates/7/delete", url.Values{}, 0, "7"); rec.Code != http.StatusNotFound {
		t.Errorf("no tenant: got %d, want 404", rec.Code)
	}
	if calls := demoCalls(log); len(calls) != 0 {
		t.Errorf("nothing may be called: %v", calls)
	}
}

func TestDemoTemplateReset_DeletesOnlyMarkedDemoRecordsOfTheTenant(t *testing.T) {
	log := &demoXoluLog{}
	rows := []map[string]any{
		{"id": 1.0, "tenant_id": 5.0, "demo": true},
		{"id": 2.0, "tenant_id": 5.0, "demo": true},
		{"id": 3.0, "tenant_id": 5.0},               // not marked demo: skipped
		{"id": 4.0, "tenant_id": 6.0, "demo": true}, // other tenant: skipped
	}
	srv := newDemoXolu(t, log, rows, nil)
	defer srv.Close()
	h := newHandler(t, srv)

	rec := demoPost(h.DemoTemplateReset, "/demo/templates/reset", url.Values{}, 5, "")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("got %d", rec.Code)
	}
	if log.count("DELETE /api/v1/demo_templates/1") != 1 || log.count("DELETE /api/v1/demo_templates/2") != 1 || log.count("DELETE ") != 2 {
		t.Errorf("expected deletes of 1 and 2 only: %v", log.calls)
	}
	if qs := demoQueries(log); len(qs) != 1 || !strings.Contains(qs[0], "tenant_id = 5") {
		t.Errorf("the reset must select by tenant: %v", qs)
	}
	assertOnlyDemoEntity(t, log)

	if rec := demoPost(h.DemoTemplateReset, "/demo/templates/reset", url.Values{}, 0, ""); rec.Code != http.StatusForbidden {
		t.Errorf("no tenant: got %d, want 403", rec.Code)
	}
}

// The new routes exist only with the flag and always need a session.
func TestDemoTemplateRoutes_GatedByFlagAndAuth(t *testing.T) {
	routes := []struct{ method, path string }{
		{"POST", "/demo/templates"}, {"POST", "/demo/templates/reset"}, {"POST", "/demo/templates/7/delete"},
	}
	mux, _ := demoTestMux(t, false)
	for _, c := range routes {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(c.method, c.path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s %s with EnableDemo=false: got %d, want 404", c.method, c.path, rec.Code)
		}
	}
	mux, _ = demoTestMux(t, true)
	for _, c := range routes {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(c.method, c.path, nil))
		if rec.Code != http.StatusSeeOther {
			t.Errorf("%s %s unauthenticated: got %d, want 303 to login", c.method, c.path, rec.Code)
		}
	}
}
