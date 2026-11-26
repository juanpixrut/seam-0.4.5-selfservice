package ui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/i18n"
)

// demoAllTabs is every demonstration, in page and tab order.
func demoAllTabs() []string {
	var all []string
	for _, g := range demoGroups {
		all = append(all, g.Tabs...)
	}
	return all
}

// Every demo schema is a tab of exactly one page, and every tab is a schema;
// repeatable_bug is the one exception, shown as text on the Repeatables tab.
func TestDemoGroups_HoldEveryDemonstrationOnce(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "formengine", "schemas", "demo", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no demo schemas found: %v", err)
	}
	var schemas []string
	for _, f := range files {
		if name := strings.TrimSuffix(filepath.Base(f), ".json"); name != "repeatable_bug" {
			schemas = append(schemas, name)
		}
	}
	tabs := demoAllTabs()
	seen := map[string]bool{}
	for _, name := range tabs {
		if seen[name] {
			t.Errorf("%s is on more than one page", name)
		}
		seen[name] = true
	}
	sorted := append([]string(nil), tabs...)
	sort.Strings(sorted)
	sort.Strings(schemas)
	if strings.Join(sorted, ",") != strings.Join(schemas, ",") {
		t.Errorf("the pages hold %v, the schemas are %v", sorted, schemas)
	}
	for _, g := range demoGroups {
		if len(g.Tabs) < 2 || len(g.Tabs) > 6 {
			t.Errorf("page %s has %d tabs: a page is a handful of related demonstrations", g.ID, len(g.Tabs))
		}
		if found, ok := demoGroupByID(g.ID); !ok || found.ID != g.ID {
			t.Errorf("page %s is not found by its id", g.ID)
		}
		for _, name := range g.Tabs {
			if of, ok := demoGroupOfTab(name); !ok || of.ID != g.ID {
				t.Errorf("%s should be found on page %s", name, g.ID)
			}
		}
	}
	// every text of every page and tab exists in every language
	for _, locale := range []string{"en", "es", "pt", "ja"} {
		tr, err := i18n.New(locale)
		if err != nil {
			t.Fatal(err)
		}
		tl := tr.ForLocale(locale)
		keys := []string{"demo.overview.title", "demo.overview.intro", "demo.group.not_found"}
		for _, g := range demoGroups {
			keys = append(keys, "demo.group."+g.ID+".title", "demo.group."+g.ID+".intro")
		}
		for _, name := range tabs {
			keys = append(keys, "demo.tab."+name)
		}
		for _, k := range keys {
			if tl(k) == k || tl(k) == "" {
				t.Errorf("%s: no text for %s", locale, k)
			}
		}
	}
}

func demoGet(t *testing.T, target string) *httptest.ResponseRecorder {
	t.Helper()
	mux, sessionMgr := demoTestMux(t, true)
	req := httptest.NewRequest("GET", target, nil)
	req.AddCookie(sessionCookie(t, sessionMgr, auth.RoleViewer))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// /demo is the overview: each page with what it holds and a link to every
// demonstration, and no form of its own.
func TestDemoOverview_ListsThePagesAndTheirDemonstrations(t *testing.T) {
	rec := demoGet(t, "/demo")
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
	body := rec.Body.String()
	for _, g := range demoGroups {
		if !strings.Contains(body, `href="/demo/`+g.ID+`"`) {
			t.Errorf("the overview has no link to the page %s", g.ID)
		}
		for _, name := range g.Tabs {
			if !strings.Contains(body, `href="/demo/`+g.ID+`?tab=`+name+`"`) {
				t.Errorf("the overview has no link to %s on %s", name, g.ID)
			}
		}
	}
	for _, want := range []string{"Fields", "Pickers", "Rows", "Inputs", "Query picker", "Repeat Layouts"} {
		if !strings.Contains(body, want) {
			t.Errorf("the overview lacks %q", want)
		}
	}
	if strings.Contains(body, `action="/demo/echo"`) || strings.Contains(body, "demo-tab-panel") || strings.Contains(body, "demo.group.") || strings.Contains(body, "demo.overview.") {
		t.Errorf("the overview renders no form, no tab, and no untranslated key")
	}
}

// Each page shows its own tabs and nobody else's, the first one open.
func TestDemoGroupPage_ShowsItsOwnTabs(t *testing.T) {
	for _, g := range demoGroups {
		body := demoGroupBody(t, g.ID, auth.RoleViewer)
		for _, name := range demoAllTabs() {
			has := strings.Contains(body, `id="demo-tab-`+name+`"`) && strings.Contains(body, `data-tab="demo-tab-`+name+`"`)
			if has != g.has(name) {
				t.Errorf("page %s: tab %s present=%v, want %v", g.ID, name, has, g.has(name))
			}
		}
		if n := strings.Count(body, demoTabActive); n != 1 {
			t.Errorf("page %s: %d tabs drawn as open, want 1", g.ID, n)
		}
		first := strings.Index(body, `data-tab="demo-tab-`+g.Tabs[0]+`"`)
		if first < 0 || first > strings.Index(body, `data-tab="demo-tab-`+g.Tabs[1]+`"`) {
			t.Errorf("page %s: its tabs are not in order", g.ID)
		}
		if strings.Contains(body, "demo.group.") {
			t.Errorf("page %s: untranslated key", g.ID)
		}
		// asked for one of its tabs, the page opens it
		rec := demoGet(t, demoTabURL(g, g.Tabs[1]))
		if !strings.Contains(rec.Body.String(), `.demo-tab-btn[data-tab="demo-tab-`+g.Tabs[1]+`"]`) {
			t.Errorf("page %s: ?tab=%s does not open that tab", g.ID, g.Tabs[1])
		}
	}
}

func TestDemoGroupPage_UnknownPageIsNotFound(t *testing.T) {
	for _, target := range []string{"/demo/nothing", "/demo/templates", "/demo/echo"} {
		if rec := demoGet(t, target); rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("GET %s: got %d, want 404 or 405", target, rec.Code)
		}
	}
	// the pages are gated with the module
	mux, _ := demoTestMux(t, false)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/demo/rows", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /demo/rows with the module off: got %d, want 404", rec.Code)
	}
	mux, _ = demoTestMux(t, true)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/demo/rows", nil))
	if rec.Code != http.StatusSeeOther {
		t.Errorf("GET /demo/rows unauthenticated: got %d, want 303 to login", rec.Code)
	}
}

// An address from before the pages existed goes to the demonstration's page.
func TestDemoIndex_OldTabAddressesGoToTheirPage(t *testing.T) {
	for tab, want := range map[string]string{"repeatable": "/demo/rows?tab=repeatable", "query_picker": "/demo/pickers?tab=query_picker", "colors": "/demo/inputs?tab=colors"} {
		rec := demoGet(t, "/demo?tab="+tab)
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != want {
			t.Errorf("/demo?tab=%s: got %d to %q, want 303 to %q", tab, rec.Code, rec.Header().Get("Location"), want)
		}
	}
	if rec := demoGet(t, "/demo?tab=nothing"); rec.Code != http.StatusOK {
		t.Errorf("an unknown tab shows the overview, got %d", rec.Code)
	}
}

// The echo page leads back to the demonstration the form was on.
func TestDemoEcho_LinksBackToTheDemonstration(t *testing.T) {
	for schema, want := range map[string]string{"layouts": `href="/demo/rows?tab=layouts"`, "components": `href="/demo/pickers?tab=components"`, "nothing": `href="/demo"`} {
		if body := demoEchoBody(t, url.Values{"_demo_schema": {schema}, "x": {"1"}}); !strings.Contains(body, want) {
			t.Errorf("echo of %s: no link %s", schema, want)
		}
	}
}
