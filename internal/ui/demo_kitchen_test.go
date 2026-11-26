package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ha1tch/seam-ui/internal/auth"
	fe "github.com/ha1tch/seam-ui/internal/formengine"
)

// kitchenPage renders the pages of the Demo module with a static source
// resolver, so the foreign-key select has options without a live xolu.
func kitchenPage(t *testing.T) string {
	t.Helper()
	prev := demoSourceResolver
	demoSourceResolver = func(context.Context, *UIHandler) fe.SourceResolver {
		return fe.StaticSourceResolver{"entity:brands": {{Value: "7", Label: "ACM \u2014 Acme"}}}
	}
	t.Cleanup(func() { demoSourceResolver = prev })
	return demoPageBody(t)
}

// demoPageBody is every page of demonstrations, one after another: what the
// single /demo page held before the demonstrations were grouped into pages.
func demoPageBody(t *testing.T) string {
	t.Helper()
	var all strings.Builder
	for _, g := range demoGroups {
		all.WriteString(demoGroupBody(t, g.ID, auth.RoleViewer))
	}
	return all.String()
}

// demoGroupBody is one page of the module (GET /demo/<group>), as role sees it.
func demoGroupBody(t *testing.T, group, role string) string {
	t.Helper()
	mux, sessionMgr := demoTestMux(t, true)
	req := httptest.NewRequest("GET", "/demo/"+group, nil)
	req.AddCookie(sessionCookie(t, sessionMgr, role))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /demo/%s: got %d", group, rec.Code)
	}
	return rec.Body.String()
}

func requireAll(t *testing.T, body string, wants []string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("page missing %s", want)
		}
	}
}

// The three Kitchen tabs exist, in order, with captions and one form each.
func TestDemoKitchen_PanelsAndTabs(t *testing.T) {
	body := kitchenPage(t)
	requireAll(t, body, []string{
		`id="demo-tab-kitchen_inputs"`, `id="demo-tab-kitchen_choices"`, `id="demo-tab-kitchen_field"`,
		`data-tab="demo-tab-kitchen_inputs"`, `data-tab="demo-tab-kitchen_choices"`, `data-tab="demo-tab-kitchen_field"`,
		`value="kitchen_inputs"`, `value="kitchen_choices"`, `value="kitchen_field"`,
		">Inputs</button>", ">Choices</button>", ">Capture</button>", ">In Rows</button>",
		"Kitchen sink: inputs", "Kitchen sink: choices and pickers", "Kitchen sink: location, barcodes and signature",
	})
	// the three form-widget tabs open the Inputs page, in this order
	inputs := demoGroupBody(t, "inputs", auth.RoleViewer)
	a, b, c := strings.Index(inputs, `data-tab="demo-tab-kitchen_inputs"`), strings.Index(inputs, `data-tab="demo-tab-kitchen_choices"`), strings.Index(inputs, `data-tab="demo-tab-kitchen_field"`)
	if a < 0 || a > b || b > c {
		t.Errorf("Inputs, Choices, Capture must be the first tabs of the Inputs page, in that order: %d %d %d", a, b, c)
	}
	if strings.Contains(inputs, `id="demo-tab-kitchen_rows"`) || !strings.Contains(demoGroupBody(t, "rows", auth.RoleViewer), `id="demo-tab-kitchen_rows"`) {
		t.Errorf("widgets inside rows belong to the Rows page")
	}
}

func TestDemoKitchen_InputsRenderEveryWidget(t *testing.T) {
	body := kitchenPage(t)
	requireAll(t, body, []string{
		// masks: the mask is the placeholder and the maxlength
		`name="tag_code"`, `placeholder="AA-9999"`, `maxlength="7"`, `name="phone"`, `type="tel"`, `placeholder="999-999-9999"`,
		`type="email"`, `type="url"`, `name="new_password"`, `type="password"`, `autocomplete="new-password"`,
		`type="date"`, `type="datetime-local"`, `type="time"`,
		// numbers with a unit, and exact decimals at two scales
		`name="load_capacity"`, `type="number"`, `name="unit_price"`, `inputmode="decimal"`,
		`pattern="-?\d+(\.\d{1,2})?"`, `pattern="-?\d+(\.\d{1,6})?"`,
		`<textarea`, `rows="5"`, `type="color"`, `name="cert_required"`, `name="cert_number"`, `data-visible-when="{&#34;field&#34;:&#34;cert_required&#34;,&#34;op&#34;:&#34;eq&#34;,&#34;value&#34;:true}"`, `name="notify_owner"`, `name="inspector_note"`,
	})
}

func TestDemoKitchen_ChoicesRenderEveryWidget(t *testing.T) {
	body := kitchenPage(t)
	requireAll(t, body, []string{
		`name="status"`, `name="discipline"`, `data-seam-select="1"`, `<optgroup`, `name="failure_mode"`,
		`name="severity"`, `name="escalation_contact"`, `name="shutdown_notes"`, `name="shutdown_hours"`,
		// foreign-key select with its popover base and the resolved option
		`name="brand_id"`, `data-jump-base="/brands"`, `value="7"`, "ACM \u2014 Acme",
		// multi-select, pickers, icon picker
		`name="skills[]"`, `picker-search-linked_asset_id`, `picker-search-marked_asset_id`, `data-hl="background"`,
		`name="reviewer_id"`, `data-selector="users"`, `name="icon"`,
	})
}

func TestDemoKitchen_FieldWidgetsAndPresetValues(t *testing.T) {
	body := kitchenPage(t)
	requireAll(t, body, []string{
		// two maps, one opened on a stored point
		`gps-map-site_location`, `gps-map-depot_location`, `-34.9011,-56.1645`,
		// three barcodes with a valid stored value each, and their symbologies
		`barcode-svg-ean13_code`, `5901234123457`, `barcode-svg-ean8_code`, `96385074`, `barcode-svg-upc_code`, `123456789999`,
		`data-format="EAN13"`, `data-format="EAN8"`, `data-format="UPC"`,
		`sigcanvas-handover_signature`,
	})
	// the UPC-E encoder is missing from the vendored bundle: no UPC-E field in the demo
	if strings.Contains(body, "upce_code") || strings.Contains(body, `data-format="UPCE"`) {
		t.Errorf("the demo must not offer UPC-E while the vendored JsBarcode bundle cannot draw it")
	}
	// image is edit-only: the demo shows its create-mode message, never the uploader
	if strings.Contains(body, "<seam-file-upload") {
		t.Errorf("the image widget must show its create-mode message on the demo page")
	}
}

// A source that cannot be resolved (here: no xolu behind the client) costs the
// FK select only; every other field of every Kitchen tab still renders.
func TestDemoKitchen_UnresolvableSourceDegradesOnlyThatField(t *testing.T) {
	body := demoPageBody(t)
	requireAll(t, body, []string{`name="tag_code"`, `name="discipline"`, `name="skills[]"`, `gps-map-site_location`, `sigcanvas-handover_signature`})
}

// The echo page rebuilds Kitchen posts: array values and the signature string.
func TestDemoEcho_KitchenChoicesAndField(t *testing.T) {
	body := demoEchoBody(t, url.Values{
		"_demo_schema":       {"kitchen_choices"},
		"skills[]":           {"Welding", "Hot work"},
		"severity":           {"High"},
		"escalation_contact": {"Duty manager"},
		"_csrf":              {"csrf-secret-123"},
	})
	requireAll(t, body, []string{"Welding", "Hot work", "Duty manager"})
	if strings.Contains(body, "csrf-secret-123") {
		t.Errorf("underscore fields must not be echoed")
	}
	body = demoEchoBody(t, url.Values{
		"_demo_schema":       {"kitchen_field"},
		"depot_location":     {"-34.901100,-56.164500"},
		"handover_signature": {"data:image/png;base64,AAAA"},
	})
	requireAll(t, body, []string{"-34.901100,-56.164500", "data:image/png;base64,AAAA"})
}
