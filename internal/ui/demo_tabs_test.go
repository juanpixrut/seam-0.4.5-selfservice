package ui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ha1tch/seam-ui/internal/auth"
)

// The four single-schema tabs: panels, tab buttons, texts and the real
// engine's output for each variant.
func TestDemoIndex_RendersTheSchemaTabs(t *testing.T) {
	body := demoPageBody(t)
	for _, want := range []string{
		// panels and their tab buttons
		`id="demo-tab-layouts"`, `id="demo-tab-buttons"`, `id="demo-tab-colors"`, `id="demo-tab-selectors"`, `id="demo-tab-reorder"`, `data-tab="demo-tab-reorder"`, `value="reorder"`,
		// reorder: handles, the marker, the position input of indexed rows, searchable selects
		"Reordering rows and searchable selects", `class="fe-drag"`, `data-reorder="1"`, `name="checklist[0][_pos]"`,
		`name="steps_step[]"`, `data-seam-select="1"`, `data-ss-search="`,
		`data-tab="demo-tab-layouts"`, `data-tab="demo-tab-selectors"`,
		"Repeat Layouts", "Custom Buttons", "Colors", "Selectors", `data-fe-fx="1"`, // captions; row effects on
		"Row and form layouts", "Add and remove buttons", "Accent colours", "Selectors over several entities",
		// each form says which schema it came from
		`name="_demo_schema"`, `value="layouts"`, `value="selectors"`,
		// layouts: row lines with a picker, form cards, a row layout nested in a form layout
		`name="expenses[0][description]"`, `name="expenses[0][cost_center_id]"`, `name="checklist[0][task]"`,
		`name="contacts[0][primary]"`, `name="phases[0][tasks][0][hours]"`, `class="fe-rep-nested"`, `data-layout="form"`,
		// buttons: labels, icons, styles
		">Add tag<", ">New step<", ">Add approver<", "Remove approver", `aria-label="Attach a file"`,
		"fe-btn--filled", "fe-btn--link", "fe-btn--icon", "fe-remove--text", ">delete_outline<", ">person_add<", `name="tags_tag[]"`,
		// colours: named, hex, nested, remove colour
		"--fe-accent:#f43f5e", "--fe-accent:#10b981", "--fe-accent:#f59e0b", "--fe-accent:#7c3aed", "--fe-accent:#14b8a6",
		"--fe-remove:#64748b", `data-fe-accent="1"`,
		// selectors: one Browse button per entity, and a picker without one
		`data-selector="assets"`, `data-selector="cost_centers"`, `data-selector="locations"`, `data-selector="custodians"`,
		`name="asset_id"`, `name="spare_location_id"`, `name="assignments[0][custodian_id]"`, `name="assignments[0][location_id]"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page missing %s", want)
		}
	}
	for _, name := range demoAllTabs() {
		if name == "components" || name == "repeatable" {
			continue // panels of their own (TestDemoIndex_RendersBothTabsForEveryRole)
		}
		for _, key := range []string{"demo.tab." + name, "demo." + name + ".heading", "demo." + name + ".intro", "demo." + name + ".notes"} {
			if strings.Contains(body, key) {
				t.Errorf("untranslated key %s on the page", key)
			}
		}
	}
	if strings.Contains(body, "demo.schema.heading") {
		t.Errorf("untranslated demo.schema.heading on the page")
	}
}

func demoEchoBody(t *testing.T, form url.Values) string {
	t.Helper()
	mux, sessionMgr := demoTestMux(t, true)
	req := httptest.NewRequest("POST", "/demo/echo", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sessionCookie(t, sessionMgr, auth.RoleViewer))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
	return rec.Body.String()
}

// The echo page rebuilds the structure with the schema the form came from.
func TestDemoEcho_UsesTheSubmittedSchema(t *testing.T) {
	body := demoEchoBody(t, url.Values{
		"_demo_schema":               {"layouts"},
		"phases[0][name]":            {"Commissioning"},
		"phases[0][tasks][0][task]":  {"Wire"},
		"phases[0][tasks][0][hours]": {"3"},
		"expenses[0][description]":   {"Cable"},
		"_csrf":                      {"csrf-secret-123"},
	})
	for _, want := range []string{"Structured value rebuilt by the server", `&#34;phases&#34;: [`, `&#34;tasks&#34;: [`, `&#34;expenses&#34;: [`, "Cable"} {
		if !strings.Contains(body, want) {
			t.Errorf("echo page missing %s", want)
		}
	}
	if strings.Contains(body, "_demo_schema") || strings.Contains(body, "csrf-secret-123") {
		t.Errorf("underscore fields must not be echoed")
	}
}

// An unknown schema name falls back to the original repeatable schema.
func TestDemoEcho_UnknownSchemaFallsBack(t *testing.T) {
	body := demoEchoBody(t, url.Values{"_demo_schema": {"../../etc/passwd"}, "blocks[0][name]": {"Prep"}})
	if !strings.Contains(body, `&#34;blocks&#34;: [`) {
		t.Errorf("fallback to the repeatable schema failed:\n%s", body)
	}
}
