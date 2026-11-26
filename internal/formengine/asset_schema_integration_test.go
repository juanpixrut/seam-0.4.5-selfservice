package formengine_test

import (
	"context"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"
	fe "github.com/ha1tch/seam-ui/internal/formengine"
	asset "github.com/ha1tch/seam-ui/internal/formengine/schemas/asset"
	"github.com/ha1tch/seam-ui/internal/i18n"
)

// extractTag returns the full enclosing HTML tag around byte offset idx
// -- local copy of internal/formengine's own test helper (unexported,
// so not reachable from this external formengine_test package). Needed
// because minty's Element.Attributes is map[string]string internally,
// and Go randomizes map iteration order per process -- a fixed byte
// window around a match is not reliable. See CHANGELOG 0.2.16 for the
// full finding.
func extractTag(html string, idx int) string {
	if idx < 0 || idx >= len(html) {
		return ""
	}
	start := strings.LastIndex(html[:idx], "<")
	if start == -1 {
		start = 0
	}
	end := strings.Index(html[idx:], ">")
	if end == -1 {
		return html[start:]
	}
	return html[start : idx+end+1]
}

// TestAssetSchema_RendersEndToEnd is the real proof case for stage 7:
// the actual embedded Asset schema, run through the actual FormEngine,
// not a hand-written test fixture standing in for it. Catches parse
// errors, §15.1 validation failures, and rendering panics that a
// smaller synthetic schema wouldn't necessarily exercise.
func TestAssetSchema_RendersEndToEnd(t *testing.T) {
	schema := asset.Schema()
	sources := fe.StaticSourceResolver{
		"entity:asset-types":  []fe.Option{{Value: "1", Label: "Compresor"}},
		"entity:brands":       []fe.Option{{Value: "1", Label: "Acme"}},
		"entity:models":       []fe.Option{{Value: "1", Label: "X100"}},
		"entity:locations":    []fe.Option{{Value: "1", Label: "Planta A"}},
		"entity:regions":      []fe.Option{{Value: "1", Label: "Montevideo"}},
		"entity:custodians":   []fe.Option{{Value: "1", Label: "Juan Perez"}},
		"entity:currencies":   []fe.Option{{Value: "1", Label: "USD $"}},
		"entity:cost-centers": []fe.Option{{Value: "1", Label: "CC-01"}},
		"entity:alert-types":  []fe.Option{{Value: "1", Label: "Temp alta"}},
	}
	e := &fe.FormEngine{Sources: sources}

	values := map[string]interface{}{
		"name":        "Compresor #4",
		"type_id":     float64(1),
		"criticality": "Alta",
		"tenure_type": "Comodato",
		"loan_expiry": "2027-01-01",
		"loan_links":  []interface{}{map[string]interface{}{"name": "Contrato", "url": "https://example.com/c.pdf"}},
		"service_configs": []interface{}{
			map[string]interface{}{"measure": "Kms", "threshold": float64(10000), "alerta_id": float64(1)},
		},
	}

	h := e.Render(context.Background(), schema, values, nil, fe.RenderContext{CSRFToken: "tok"})
	html := mi.RenderToString(h)

	// text-red-600 is the actual error styling (unsupported widget,
	// missing SourceResolver, etc.) -- distinct from asset-picker's
	// deliberate amber "not implemented in the MVP" note (§8.3), which
	// also happens to use a "formengine:" prefix and would false-positive
	// a bare substring check for that prefix alone.
	if strings.Contains(html, "text-red-600") {
		t.Errorf("rendered output contains a real formengine error (text-red-600 styling):\n%s", html)
	}
	if len(html) < 5000 {
		t.Errorf("rendered output looks too short for a 4-tab, 42-property form (got %d bytes)", len(html))
	}

	// Spot-check a representative field from each tab actually rendered.
	for _, want := range []string{
		`name="name"`,                // tab-principal
		`name="location_id"`,         // tab-loc
		`name="decommission_reason"`, // tab-ciclo
		`name="currency_id"`,         // tab-finserv
	} {
		if !strings.Contains(html, want) {
			t.Errorf("expected %s in the rendered output", want)
		}
	}
}

func TestAssetSchema_VtoComodatoVisibleWhenWorks(t *testing.T) {
	schema := asset.Schema()
	e := &fe.FormEngine{Sources: fe.StaticSourceResolver{}}

	// tenure_type = Loaned -> loan_expiry should render visible. English
	// tokens as of T-23 (docs/RESOLVED.md) -- was "Comodato" before.
	visible := map[string]interface{}{"tenure_type": "Loaned"}
	h1 := e.Render(context.Background(), schema, visible, nil, fe.RenderContext{})
	html1 := mi.RenderToString(h1)
	idx1 := strings.Index(html1, `data-field-row="loan_expiry"`)
	if idx1 == -1 {
		t.Fatal("loan_expiry row not found")
	}
	tag1 := extractTag(html1, idx1)
	if strings.Contains(tag1, "display:none") {
		t.Errorf("loan_expiry should be visible when tenure_type=Loaned:\n%s", tag1)
	}

	// Anything else -> hidden.
	hidden := map[string]interface{}{"tenure_type": "Owned"}
	h2 := e.Render(context.Background(), schema, hidden, nil, fe.RenderContext{})
	html2 := mi.RenderToString(h2)
	idx2 := strings.Index(html2, `data-field-row="loan_expiry"`)
	tag2 := extractTag(html2, idx2)
	if !strings.Contains(tag2, "display:none") {
		t.Errorf("loan_expiry should be hidden when tenure_type=Owned:\n%s", tag2)
	}
}

func TestAssetSchema_RequiredFieldsMatchRealForm(t *testing.T) {
	schema := asset.Schema()
	e := &fe.FormEngine{Sources: fe.StaticSourceResolver{}}
	html := mi.RenderToString(e.Render(context.Background(), schema, nil, nil, fe.RenderContext{}))

	requiredIDs := []string{"field-name", "field-type_id", "field-criticality"}
	for _, id := range requiredIDs {
		idx := strings.Index(html, `id="`+id+`"`)
		if idx == -1 {
			t.Fatalf("%s not found", id)
		}
		tag := extractTag(html, idx)
		if !strings.Contains(tag, " required") {
			t.Errorf("%s should carry the required attribute:\n%s", id, tag)
		}
	}

	// region_id: never actually HTML-enforced in the real static form
	// -- schema reproduces that faithfully, not "fixed". (The real
	// form's own asterisk on this field WAS an inconsistency, though --
	// see TestAssetSchema_RequiredIndicatorMatchesRequiredArray below,
	// which covers that it no longer gets one here.)
	idx := strings.Index(html, `id="field-region_id"`)
	if idx == -1 {
		t.Fatal("field-region_id not found")
	}
	tag := extractTag(html, idx)
	if strings.Contains(tag, " required") {
		t.Errorf("region_id should NOT carry required (faithful to the real form's actual, if inconsistent, behaviour):\n%s", tag)
	}
}

// TestAssetSchema_RequiredIndicatorMatchesRequiredArray covers the
// design decision T-13 carried forward to FE-24, 2026-07-28: the
// visual "*" on a field's label is now driven entirely by f.Required
// (fkPopoverAwareLabel), never hand-typed into the schema's literal
// title text. This is deliberately a DIFFERENT concern from the
// TestAssetSchema_RequiredFieldsMatchRealForm test above, which covers
// the HTML required attribute -- that test's region_id case correctly
// stays "faithful to an inconsistency" (no required attribute, since
// the real static form never enforced it); this test covers that the
// same field's asterisk no longer shows one, which is a fix, not a
// faithful reproduction, since the schema's own required array is now
// the single source of truth for both.
func TestAssetSchema_RequiredIndicatorMatchesRequiredArray(t *testing.T) {
	// The asterisk lives in the <label>'s own text content, a
	// different element entirely from the <input>/<select> extractTag
	// finds via id="field-X" -- checking the label's rendered text
	// directly rather than reusing extractTag against the wrong
	// element (caught by this test actually failing on the first
	// attempt with the wrong approach, not assumed to work).
	schema := asset.Schema()
	e := &fe.FormEngine{Sources: fe.StaticSourceResolver{}}
	html := mi.RenderToString(e.Render(context.Background(), schema, nil, nil, fe.RenderContext{}))

	requiredLabels := map[string]string{
		"field-name":        "Name *",
		"field-type_id":     "Asset Type *",
		"field-criticality": "Criticality *",
	}
	for id, wantLabel := range requiredLabels {
		if !strings.Contains(html, `for="`+id+`"`) {
			t.Fatalf("label for=%q not found", id)
		}
		if !strings.Contains(html, wantLabel) {
			t.Errorf("expected label text %q for a required field, not found in rendered output", wantLabel)
		}
	}

	if !strings.Contains(html, `for="field-region_id"`) {
		t.Fatal("label for=\"field-region_id\" not found")
	}
	if strings.Contains(html, "Region *") {
		t.Error("region_id is NOT in the schema's required array and should no longer show an asterisk -- that was the actual inconsistency being fixed")
	}
	if !strings.Contains(html, ">Region<") {
		t.Error("expected region_id's label text without an asterisk, not found")
	}
}

// TestAssetSchema_I18nContent_EndToEnd is T-17 gap 3's actual proof,
// using the real, shipped i18n.New (embedded en/es/pt files), not a
// synthetic test translator -- confirms the genuine content authored
// for this gap actually resolves, and that the raw-i18n-key-as-visible-
// text bug found empirically before authoring any content (tab labels
// rendered literally as "assets.forms.asset.tab.principal" with no
// Translator content backing them) is now fixed for real, not just in
// a mock.
func TestAssetSchema_I18nContent_EndToEnd(t *testing.T) {
	translator, err := i18n.New("en")
	if err != nil {
		t.Fatalf("i18n.New: %v", err)
	}
	schema := asset.Schema()
	e := &fe.FormEngine{Sources: fe.StaticSourceResolver{}, Translator: translator}

	cases := []struct {
		locale           string
		wantTab          string
		wantMaintTab     string
		wantField        string
		wantSection      string
		wantLifecycleDoc string
		wantHelp         string
		notRawKeys       []string
	}{
		{
			locale:           "en",
			wantTab:          "Main",
			wantMaintTab:     "Maintenance",
			wantField:        "Name *",
			wantSection:      "Basic Details",
			wantLifecycleDoc: "Lifecycle Documents",
			wantHelp:         "The asset that contains this one in the hierarchy",
			notRawKeys: []string{
				"assets.forms.asset.tab.principal",
				"assets.forms.asset.tab.maintenance",
				"assets.forms.asset.section.lifecycle_documents",
			},
		},
		{
			locale:           "es",
			wantTab:          "Principal",
			wantMaintTab:     "Mantenimiento",
			wantField:        "Nombre *",
			wantSection:      "Datos Básicos",
			wantLifecycleDoc: "Documentos de Ciclo de Vida",
			wantHelp:         "Asset que contiene este activo en la jerarquía",
			notRawKeys: []string{
				"assets.forms.asset.tab.principal",
				"assets.forms.asset.tab.maintenance",
				"assets.forms.asset.section.lifecycle_documents",
			},
		},
	}
	for _, c := range cases {
		html := mi.RenderToString(e.Render(context.Background(), schema, nil, nil, fe.RenderContext{Locale: c.locale}))
		if !strings.Contains(html, c.wantTab) {
			t.Errorf("[%s] expected tab label %q, not found", c.locale, c.wantTab)
		}
		if !strings.Contains(html, c.wantMaintTab) {
			t.Errorf("[%s] expected maintenance tab label %q, not found", c.locale, c.wantMaintTab)
		}
		if !strings.Contains(html, c.wantField) {
			t.Errorf("[%s] expected field label %q, not found", c.locale, c.wantField)
		}
		if !strings.Contains(html, c.wantSection) {
			t.Errorf("[%s] expected section label %q, not found", c.locale, c.wantSection)
		}
		if !strings.Contains(html, c.wantLifecycleDoc) {
			t.Errorf("[%s] expected lifecycle documents section label %q, not found", c.locale, c.wantLifecycleDoc)
		}
		if !strings.Contains(html, c.wantHelp) {
			t.Errorf("[%s] expected parent_id help text %q, not found", c.locale, c.wantHelp)
		}
		for _, rawKey := range c.notRawKeys {
			if strings.Contains(html, rawKey) {
				t.Errorf("[%s] raw i18n key %q visible in rendered output -- the bug this gap fixes", c.locale, rawKey)
			}
		}
	}
}
