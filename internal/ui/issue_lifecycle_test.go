package ui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ha1tch/seam-ui/internal/features"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

// newIssuesHandler is newHandler with the Issues module on. The module is
// off by default, the way Work Orders is, so a test of its pages says so.
func newIssuesHandler(t *testing.T, server *httptest.Server) *UIHandler {
	t.Helper()
	cfg := Config{AppName: "Test", Version: "0.0.0", EnableIssues: true}
	return NewUIHandler(xolu.New(server.URL), cfg, nil, nil, nil, nil, nil, nil, nil)
}

// With the Issues module off a scanned code still identifies its asset, but
// there is nowhere to send a report: the page shows the asset and no form.
func TestQRScanLanding_WithoutIssuesShowsTheAsset(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{
		"GET /api/v1/assets/5": func(w http.ResponseWriter, r *http.Request) {
			qrTestJSON(w, map[string]any{
				"id": 5, "name": "Pump 5", "type_name": "Pump",
				"status": "active", "location": "Plant A",
			})
		},
		"POST /api/v1/events": func(w http.ResponseWriter, r *http.Request) {
			oqlCreated(w, 1)
		},
		"POST /api/v1/oql/query": func(w http.ResponseWriter, r *http.Request) {
			q := qrTestQuery(t, r)
			if strings.Contains(q, "FROM issue_qr_codes") {
				qrTestRows(w, []map[string]any{{
					"id": float64(1), "code": "AAAAAAAAAAAAAAAA", "asset_id": float64(5),
					"asset_category": "critico", "active": true,
				}})
				return
			}
			qrTestRows(w, nil)
		},
	})
	defer server.Close()

	h := newHandler(t, server) // EnableIssues left false, the default
	rec := qrTestGet(t, h.QRScan, "/s/AAAAAAAAAAAAAAAA", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("a live code must still render the asset, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Pump 5") || !strings.Contains(body, "Plant A") {
		t.Errorf("the page must show the asset")
	}
	for _, not := range []string{`/report"`, `name="contact_email"`, `name="description"`} {
		if strings.Contains(body, not) {
			t.Errorf("with Issues off the page must not offer the report form, found %q", not)
		}
	}
}

// The Issues module is switched on for one tenant from the Developer panel
// the same way the Demo and Work Orders modules are: 404 until then, for the
// tenant's developers once on, and 404 again once off.
func TestModules_IssuesSwitchedOnForOneTenantAtRuntime(t *testing.T) {
	h := newDevHarness(t, false, false)
	cfg := h.ui.config
	if cfg.EnableIssues || !cfg.moduleSwitches() {
		t.Fatalf("the harness starts with Issues off and module switches on")
	}
	if rec := h.get(t, "/issues"); rec.Code != http.StatusNotFound {
		t.Fatalf("off: the module's routes answer 404, got %d", rec.Code)
	}
	flip := func(action string) {
		t.Helper()
		if rec := h.post(t, "/developer/flags", url.Values{"key": {features.ModuleIssues}, "action": {action}}); rec.Code != http.StatusSeeOther || strings.Contains(location(rec), "error=") {
			t.Fatalf("%s: %d %s", action, rec.Code, location(rec))
		}
	}
	flip("on")
	if !cfg.ModuleOn(1, "developer", false, features.ModuleIssues) || cfg.ModuleOn(2, "developer", false, features.ModuleIssues) {
		t.Errorf("on for the caller's tenant only")
	}
	if cfg.ModuleOn(1, "developer", false, features.ModuleWorkOrders) || cfg.ModuleOn(1, "developer", false, features.ModuleDemo) {
		t.Errorf("the other modules are untouched")
	}
	if rec := h.get(t, "/issues"); rec.Code == http.StatusNotFound {
		t.Errorf("on: the module's routes answer, got %d", rec.Code)
	}
	if rec := h.do(t, http.MethodGet, "/issues", "admin", nil); rec.Code != http.StatusNotFound {
		t.Errorf("an admin of the tenant gets 404 by default, got %d", rec.Code)
	}
	flip("off")
	if rec := h.get(t, "/issues"); rec.Code != http.StatusNotFound {
		t.Errorf("off again: 404, got %d", rec.Code)
	}
	// on at startup: on for every tenant, and no switch to show
	on := cfg
	on.EnableIssues = true
	if !on.ModuleOn(2, "viewer", on.EnableIssues, features.ModuleIssues) {
		t.Errorf("on at startup is on for every tenant")
	}
	for _, fv := range developerPanelFlags(on, h.rt.Manager.Panel(1)) {
		if fv.Flag.Key == features.ModuleIssues {
			t.Errorf("a module on at startup has no runtime switch")
		}
	}
}
