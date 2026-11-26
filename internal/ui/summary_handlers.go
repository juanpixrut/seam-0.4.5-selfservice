package ui

// summary_handlers.go — compact summary card partials used by SeamPopover.
//
// Each GET /{entity}/{id}/summary endpoint returns a small HTML fragment
// (no layout wrapper) rendered as a popover card. The card shows the most
// important read-only fields and provides two action buttons:
//   - "New tab" — opens the full edit page in a new browser tab
//   - "Edit →"  — opens SeamModal2 (stacked modal) for in-context editing
//
// t threading (T-18 Stage 9): every label, the Active/Inactive status
// value, the empty state, and both action buttons now resolve through
// t, matching the rest of the app. This file previously had zero t()
// call sites -- a 100% hardcoded-Spanish gap the same shape as Stage
// 6's Sensors/Rules/AssetTypes finding, just never named in any of
// T-18's nine stages since summary popovers weren't listed as their
// own area.

import (
	"fmt"
	"net/http"

	mi "github.com/ha1tch/minty"
)

// ── shared summary card renderer ──────────────────────────────────────────────

type summaryRow struct{ Label, Value string }

func summaryCardNode(b *mi.Builder, title, editURL string, rows []summaryRow, t func(key string, args ...any) string) mi.Node {
	var rowNodes []interface{}
	for _, r := range rows {
		if r.Value == "" {
			continue
		}
		rowNodes = append(rowNodes,
			b.Div(mi.Class("flex gap-2 text-xs py-0.5"),
				b.Span(mi.Class("text-gray-400 dark:text-gray-500 min-w-20 flex-shrink-0"), r.Label),
				b.Span(mi.Class("text-gray-700 dark:text-gray-200"), r.Value),
			),
		)
	}
	if len(rowNodes) == 0 {
		rowNodes = append(rowNodes, b.Span(mi.Class("text-xs text-gray-400"), t("summary.no_data")))
	}
	bodyAttrs := []interface{}{mi.Class("space-y-0.5 mt-2")}
	bodyAttrs = append(bodyAttrs, rowNodes...)

	// "Edit →" triggers SeamModal2.open — escape single quotes in title
	openModal := fmt.Sprintf("SeamModal2.open(%q, %q); SeamPopover.close()", title, editURL)

	return b.Div(
		b.Div(mi.Class("flex items-start justify-between gap-2"),
			b.Div(mi.Class("font-semibold text-sm text-gray-900 dark:text-white leading-snug"), title),
			b.Button(mi.Type("button"),
				mi.Attr("onclick", "SeamPopover.close()"),
				mi.Class("text-gray-400 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300 flex-shrink-0"),
				b.Span(mi.Class("material-icons"), mi.Style("font-size:15px;line-height:1"), "close"),
			),
		),
		b.Div(bodyAttrs...),
		b.Div(mi.Class("mt-3 pt-2 border-t border-gray-100 dark:border-gray-700 flex items-center justify-end gap-4"),
			b.A(mi.Href(editURL), mi.Target("_blank"),
				mi.Class("flex items-center gap-1 text-xs text-gray-400 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300"),
				b.Span(mi.Class("material-icons"), mi.Style("font-size:11px;line-height:1"), "open_in_new"),
				t("summary.new_tab"),
			),
			b.Button(mi.Type("button"),
				mi.Attr("onclick", openModal),
				mi.Class("text-xs font-medium text-indigo-600 hover:text-indigo-800 dark:text-indigo-400 dark:hover:text-indigo-300"),
				t("summary.edit_arrow"),
			),
		),
	)
}

// renderSummaryCard is called from each summary handler.
func (h *UIHandler) renderSummaryCard(w http.ResponseWriter, title, editURL string, rows []summaryRow, t func(key string, args ...any) string) {
	_ = h.page.RenderPartial(w, func(b *mi.Builder) mi.Node {
		return summaryCardNode(b, title, editURL, rows, t)
	})
}

// ── custodios ─────────────────────────────────────────────────────────────────

func (h *UIHandler) CustodioSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	t := h.translateFunc(r)
	id := masterID(r.URL.Path, "/custodians/", "/summary")
	if id == 0 {
		http.Error(w, h.errText(r, "error.not_found", "not found"), http.StatusNotFound)
		return
	}
	entity, err := h.xoluClient.Get(ctx, "custodians", id)
	if err != nil {
		http.Error(w, h.errText(r, "error.not_found", "not found"), http.StatusNotFound)
		return
	}
	d := entity.Data
	activeStr := t("status.inactive")
	if toBool(d["active"]) {
		activeStr = t("status.active") + " ✓"
	}
	title := toString(d["legal_name"])
	editURL := fmt.Sprintf("/custodians/%d/edit", id)
	h.renderSummaryCard(w, title, editURL, []summaryRow{
		{t("masters.custodian.field.document_type"), toString(d["subject_type"])},
		{t("masters.custodian.field.fiscal_id"), toString(d["tax_id"])},
		{t("field.alias"), toString(d["alias"])},
		{t("field.email"), toString(d["email"])},
		{t("field.phone"), toString(d["phone"])},
		{t("field.address"), toString(d["address"])},
		{t("field.status"), activeStr},
	}, t)
}

// ── marcas ────────────────────────────────────────────────────────────────────

func (h *UIHandler) MarcaSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	t := h.translateFunc(r)
	id := masterID(r.URL.Path, "/brands/", "/summary")
	if id == 0 {
		http.Error(w, h.errText(r, "error.not_found", "not found"), http.StatusNotFound)
		return
	}
	entity, err := h.xoluClient.Get(ctx, "brands", id)
	if err != nil {
		http.Error(w, h.errText(r, "error.not_found", "not found"), http.StatusNotFound)
		return
	}
	d := entity.Data
	title := toString(d["code"]) + " — " + toString(d["name"])
	editURL := fmt.Sprintf("/brands/%d/edit", id)
	h.renderSummaryCard(w, title, editURL, []summaryRow{
		{t("masters.brand.field.code"), toString(d["code"])},
		{t("field.name"), toString(d["name"])},
	}, t)
}

// ── modelos ───────────────────────────────────────────────────────────────────

func (h *UIHandler) ModeloSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	t := h.translateFunc(r)
	id := masterID(r.URL.Path, "/models/", "/summary")
	if id == 0 {
		http.Error(w, h.errText(r, "error.not_found", "not found"), http.StatusNotFound)
		return
	}
	entity, err := h.xoluClient.Get(ctx, "models", id)
	if err != nil {
		http.Error(w, h.errText(r, "error.not_found", "not found"), http.StatusNotFound)
		return
	}
	d := entity.Data
	// Resolve brand name. Fixed 2026-08-11 -- same class of miss as
	// asset_source_resolver.go's entity:models query (T-83): this
	// handler reads models' own marca_id/nombre and brands' own
	// nombre, all of which stopped existing once T-80 (Modelo's rename)
	// and brands' own earlier rename shipped. Found by sweeping for
	// remaining marca_id/modelo_id/criticidad/serie_1/serie_2/lote
	// occurrences after Block C, not by testing this specific summary
	// card live -- worth a live check before considering this closed.
	brandName := ""
	if mid := toInt64(d["brand_id"]); mid > 0 {
		if m, err := h.xoluClient.Get(ctx, "brands", mid); err == nil {
			brandName = toString(m.Data["name"])
		}
	}
	title := toString(d["name"])
	editURL := fmt.Sprintf("/models/%d/edit", id)
	h.renderSummaryCard(w, title, editURL, []summaryRow{
		{t("assets.forms.asset.field.brand_id"), brandName},
		{t("assets.forms.asset.field.model_id"), toString(d["name"])},
	}, t)
}

// ── ubicaciones ───────────────────────────────────────────────────────────────

func (h *UIHandler) UbicacionSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	t := h.translateFunc(r)
	id := masterID(r.URL.Path, "/locations/", "/summary")
	if id == 0 {
		http.Error(w, h.errText(r, "error.not_found", "not found"), http.StatusNotFound)
		return
	}
	entity, err := h.xoluClient.Get(ctx, "locations", id)
	if err != nil {
		http.Error(w, h.errText(r, "error.not_found", "not found"), http.StatusNotFound)
		return
	}
	d := entity.Data
	// Resolve region
	regionDesc := ""
	if rid := toInt64(d["region_id"]); rid > 0 {
		if reg, err := h.xoluClient.Get(ctx, "regions", rid); err == nil {
			regionDesc = toString(reg.Data["description"])
		}
	}
	title := toString(d["short_name"])
	editURL := fmt.Sprintf("/locations/%d/edit", id)
	h.renderSummaryCard(w, title, editURL, []summaryRow{
		{t("field.description"), toString(d["description"])},
		{t("assets.forms.asset.field.region_id"), regionDesc},
	}, t)
}

// ── centros-costo ─────────────────────────────────────────────────────────────

func (h *UIHandler) CentroCostoSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	t := h.translateFunc(r)
	id := masterID(r.URL.Path, "/cost-centers/", "/summary")
	if id == 0 {
		http.Error(w, h.errText(r, "error.not_found", "not found"), http.StatusNotFound)
		return
	}
	entity, err := h.xoluClient.Get(ctx, "cost_centers", id)
	if err != nil {
		http.Error(w, h.errText(r, "error.not_found", "not found"), http.StatusNotFound)
		return
	}
	d := entity.Data
	title := toString(d["accounting_code"]) + " — " + toString(d["name"])
	editURL := fmt.Sprintf("/cost-centers/%d/edit", id)
	h.renderSummaryCard(w, title, editURL, []summaryRow{
		{t("masters.cost_center.field.accounting_code"), toString(d["accounting_code"])},
		{t("field.name"), toString(d["name"])},
	}, t)
}

// ── asset types ───────────────────────────────────────────────────────────────

func (h *UIHandler) AssetTypeSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	t := h.translateFunc(r)
	id := masterID(r.URL.Path, "/asset-types/", "/summary")
	if id == 0 {
		http.Error(w, h.errText(r, "error.not_found", "not found"), http.StatusNotFound)
		return
	}
	entity, err := h.xoluClient.Get(ctx, "asset_types", id)
	if err != nil {
		http.Error(w, h.errText(r, "error.not_found", "not found"), http.StatusNotFound)
		return
	}
	d := entity.Data
	tiers := ""
	// Loop variable deliberately not named t -- t is this function's own
	// translate function (established convention: rename the loop
	// variable, not the translator, on any collision -- Stage 6 first
	// found this exact shape with a different loop).
	if ct, ok := d["connectivity_tiers"].([]interface{}); ok {
		for i, tier := range ct {
			if i > 0 {
				tiers += ", "
			}
			tiers += fmt.Sprintf("%v", tier)
		}
	}
	title := toString(d["name"])
	editURL := fmt.Sprintf("/asset-types/%d/edit", id)
	h.renderSummaryCard(w, title, editURL, []summaryRow{
		{t("field.description"), toString(d["description"])},
		{t("assets.forms.asset.field.connectivity_tier"), tiers},
	}, t)
}
