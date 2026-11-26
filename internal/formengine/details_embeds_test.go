package formengine

import (
	"context"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"
)

func detailEmbedSchema() []byte {
	return []byte(`{
		"properties": {
			"nombre": {"type": "string", "title": "Nombre"}
		},
		"x-seam-form": {
			"tabs": [{
				"id": "tab-loc", "label": "Ubicación",
				"sections": [
					{"id": "basico", "label": "Básico", "fields": ["nombre"]}
				],
				"details": [
					{
						"id": "custodio_history",
						"label": "Historial de custodia",
						"source": "/assets/{id}/custody-history",
						"columns": ["fecha", "custodio"],
						"empty": "assets.forms.asset.detail.custodio_history.empty",
						"position": "bottom",
						"collapsible": true
					},
					{
						"id": "fsm_state",
						"label": "Estado FSM",
						"source": "/assets/{id}/fsm-state",
						"position": "right"
					}
				],
				"embeds": [
					{
						"id": "sensor_binding",
						"label": "Vincular sensor",
						"endpoint": "/assets/{id}/sensors",
						"method": "post",
						"trigger": "button",
						"trigger_label": "Vincular",
						"mode": "modal2",
						"schema": "sensor_binding",
						"refresh_detail": "custodio_history"
					},
					{
						"id": "qr_assign",
						"label": "Asignar QR",
						"endpoint": "/assets/{id}/qr",
						"method": "post",
						"trigger": "inline",
						"mode": "inline",
						"schema": "qr_assign"
					}
				]
			}]
		}
	}`)
}

func renderDetailEmbedHTML(t *testing.T, rc RenderContext) string {
	t.Helper()
	e := &FormEngine{}
	h := e.Render(context.Background(), detailEmbedSchema(), nil, nil, rc)
	return mi.RenderToString(h)
}

func TestDetails_ParsedWithDefaults(t *testing.T) {
	form, err := ParseSchema(detailEmbedSchema())
	if err != nil {
		t.Fatalf("parseSchema: %v", err)
	}
	tab := form.Tabs[0]
	if len(tab.Details) != 2 {
		t.Fatalf("got %d details, want 2", len(tab.Details))
	}
	if tab.Details[0].Position != "bottom" || tab.Details[1].Position != "right" {
		t.Errorf("positions: %q, %q", tab.Details[0].Position, tab.Details[1].Position)
	}
	if tab.Details[1].Load != "eager" {
		t.Errorf("Load default should be %q, got %q", "eager", tab.Details[1].Load)
	}
	if !tab.Details[0].Collapsible {
		t.Error("custodio_history should be collapsible per the schema")
	}
}

func TestEmbeds_ParsedWithDefaults(t *testing.T) {
	form, err := ParseSchema(detailEmbedSchema())
	if err != nil {
		t.Fatalf("parseSchema: %v", err)
	}
	tab := form.Tabs[0]
	if len(tab.Embeds) != 2 {
		t.Fatalf("got %d embeds, want 2", len(tab.Embeds))
	}
	if tab.Embeds[0].Mode != "modal2" {
		t.Errorf("sensor_binding mode = %q, want modal2", tab.Embeds[0].Mode)
	}
	if tab.Embeds[0].RefreshDetail != "custodio_history" {
		t.Errorf("refresh_detail = %q, want custodio_history", tab.Embeds[0].RefreshDetail)
	}
}

func TestDetailsEmbeds_HiddenInCreateMode(t *testing.T) {
	// §3.7: details/embeds only render when IsEdit is true.
	html := renderDetailEmbedHTML(t, RenderContext{IsEdit: false})
	for _, absent := range []string{"custody-history", "fsm-state", "Vincular sensor", "Asignar QR", "detail-custodio_history", "embed-qr_assign"} {
		if strings.Contains(html, absent) {
			t.Errorf("create mode (IsEdit=false) should not render details/embeds, but found %q:\n%s", absent, html)
		}
	}
}

func TestDetailsEmbeds_VisibleInEditMode(t *testing.T) {
	rc := RenderContext{IsEdit: true, EntityID: 42}
	html := renderDetailEmbedHTML(t, rc)
	for _, want := range []string{"Historial de custodia", "Estado FSM", "Vincular", "detail-custodio_history", "detail-fsm_state", "embed-qr_assign"} {
		if !strings.Contains(html, want) {
			t.Errorf("edit mode missing %q:\n%s", want, html)
		}
	}
	// {id} substitution.
	if !strings.Contains(html, "/assets/42/custody-history") {
		t.Errorf("expected {id} substituted with EntityID in the detail source:\n%s", html)
	}
	if !strings.Contains(html, "/assets/42/sensors") {
		t.Errorf("expected {id} substituted with EntityID in the embed endpoint:\n%s", html)
	}
}

func TestDetails_RightPositionSeparatesIntoSideColumn(t *testing.T) {
	html := renderDetailEmbedHTML(t, RenderContext{IsEdit: true, EntityID: 1})
	if !strings.Contains(html, "lg:w-80") {
		t.Errorf("expected the right-column layout when a detail uses position:right:\n%s", html)
	}
}

func TestEmbeds_ModalButtonUsesSeamModal2Convention(t *testing.T) {
	html := renderDetailEmbedHTML(t, RenderContext{IsEdit: true, EntityID: 7})
	if !strings.Contains(html, "SeamModal2.open(") {
		t.Errorf("expected the established SeamModal2.open(...) trigger convention:\n%s", html)
	}
	if !strings.Contains(html, "/assets/7/sensors") {
		t.Errorf("expected the embed endpoint with substituted id inside the onclick:\n%s", html)
	}
}

func TestEmbeds_RefreshDetailHookEmitted(t *testing.T) {
	html := renderDetailEmbedHTML(t, RenderContext{IsEdit: true, EntityID: 1})
	if !strings.Contains(html, `data-refresh-detail="detail-custodio_history"`) {
		t.Errorf("expected the discoverable refresh_detail hook on sensor_binding's trigger:\n%s", html)
	}
	// qr_assign has no refresh_detail — must not emit the attribute at all.
	// (Loose check: the attribute value naming qr_assign's own id should
	// never appear, since it has no RefreshDetail configured.)
	if strings.Contains(html, `data-refresh-detail="detail-qr_assign"`) {
		t.Errorf("qr_assign has no refresh_detail configured, should not emit the hook:\n%s", html)
	}
}

func TestEmbeds_SuppressedOnReadOnlyTab(t *testing.T) {
	// §3.7 / §10.1 step v: embeds never render on a read-only tab —
	// they have their own submit cycle. Details still render (they're
	// read-only content anyway).
	rc := RenderContext{IsEdit: true, EntityID: 1, ReadOnlyTabs: []string{"tab-loc"}}
	html := renderDetailEmbedHTML(t, rc)
	if strings.Contains(html, "embed-qr_assign") || strings.Contains(html, "SeamModal2.open(") {
		t.Errorf("embeds must not render on a read-only tab:\n%s", html)
	}
	if !strings.Contains(html, "detail-custodio_history") {
		t.Errorf("detail panels should still render on a read-only tab:\n%s", html)
	}
}

func TestEmbeds_InlineTriggerRevealsContainer(t *testing.T) {
	html := renderDetailEmbedHTML(t, RenderContext{IsEdit: true, EntityID: 1})
	if !strings.Contains(html, `id="embed-qr_assign"`) {
		t.Errorf("expected an inline embed container for qr_assign:\n%s", html)
	}
	if !strings.Contains(html, `hx-trigger="revealed"`) {
		t.Errorf("expected the inline embed to load via hx-trigger=revealed:\n%s", html)
	}
}
