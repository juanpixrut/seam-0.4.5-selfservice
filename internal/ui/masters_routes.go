package ui

// masters_routes.go — CRUD handlers for Seam AMS master table entities.
//
// Wave 0: P-1 Región, P-2 Moneda, P-3 Centro de Costo, P-4 Marca,
//         P-7 Custodio, P-8 Alerta, P-17 PropDefinition
//
// Each master follows the same five-handler pattern:
//   List   GET  /{entity}
//   New    GET  /{entity}/new
//   Create POST /{entity}
//   Edit   GET  /{entity}/{id}/edit
//   Update POST /{entity}/{id}
//   Delete POST /{entity}/{id}/delete

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	mi "github.com/ha1tch/minty"
)

// ── Data structs ───────────────────────────────────────────────────────────────

// RegionRow is a row in the Regiones list.
type RegionRow struct {
	ID          int64
	Description string
}

// RegionesListData holds data for the Regiones list page.
type RegionesListData struct {
	Regions   []RegionRow
	Total     int
	CSRFToken string
}

// RegionFormData holds data for the Región create/edit form.
type RegionFormData struct {
	ID          int64
	IsEdit      bool
	Description string
	Error       string
	Errors      map[string]string
	CSRFToken   string
	IsPartial   bool
	Fields      mi.H // the form-engine fields for the request; nil for the hand-built rows
}

// MonedaRow is a row in the Monedas list.
type MonedaRow struct {
	ID      int64
	IsoCode string
	Symbol  string
}

// MonedasListData holds data for the Monedas list page.
type MonedasListData struct {
	Currencies []MonedaRow
	Total      int
	CSRFToken  string
}

// MonedaFormData holds data for the Moneda create/edit form.
type MonedaFormData struct {
	ID        int64
	IsEdit    bool
	IsoCode   string
	Symbol    string
	Error     string
	Errors    map[string]string
	CSRFToken string
	IsPartial bool
	Fields    mi.H // the form-engine fields for the request; nil for the hand-built rows
}

// CentroCostoRow is a row in the Centros de Costo list.
type CentroCostoRow struct {
	ID             int64
	AccountingCode string
	Name           string
}

// CentrosCostoListData holds data for the Centros de Costo list page.
type CentrosCostoListData struct {
	CostCenters []CentroCostoRow
	Total       int
	CSRFToken   string
}

// CentroCostoFormData holds data for the Centro de Costo create/edit form.
type CentroCostoFormData struct {
	ID             int64
	IsEdit         bool
	IsPartial      bool
	AccountingCode string
	Name           string
	Error          string
	Errors         map[string]string
	CSRFToken      string
}

// MarcaRow is a row in the Marcas list.
type MarcaRow struct {
	ID   int64
	Code string
	Name string
}

// MarcasListData holds data for the Marcas list page.
type MarcasListData struct {
	Brands    []MarcaRow
	Total     int
	CSRFToken string
}

// MarcaFormData holds data for the Marca create/edit form.
type MarcaFormData struct {
	ID        int64
	IsEdit    bool
	IsPartial bool
	Code      string
	Name      string
	Error     string
	Errors    map[string]string
	CSRFToken string
	Fields    mi.H // the form-engine fields for the request; nil for the hand-built rows
}

// CustodioRow is a row in the Custodios list.
type CustodioRow struct {
	ID          int64
	SubjectType string
	LegalName   string
	TaxId       string
	Email       string
	Active      bool
}

// CustodiosListData holds data for the Custodios list page.
type CustodiosListData struct {
	Custodios []CustodioRow
	Total     int
	CSRFToken string
}

// CustodioFormData holds data for the Custodio create/edit form.
type CustodioFormData struct {
	ID          int64
	IsEdit      bool
	IsPartial   bool
	SubjectType string
	LegalName   string
	TaxId       string
	Alias       string
	Email       string
	Phone       string
	Address     string
	Active      bool
	Error       string
	Errors      map[string]string
	CSRFToken   string
}

// AlertaRow is a row in the Alertas list.
type AlertaRow struct {
	ID          int64
	Type        string
	Description string
	Criticality string
}

// AlertasListData holds data for the Alertas list page.
type AlertasListData struct {
	Alertas   []AlertaRow
	Total     int
	CSRFToken string
}

// AlertaMasterFormData holds data for the Alerta master create/edit form.
// Named AlertaMasterFormData to avoid collision with AlertFormData in other files.
type AlertaMasterFormData struct {
	ID          int64
	IsEdit      bool
	Type        string
	Description string
	Criticality string
	Error       string
	Errors      map[string]string
	CSRFToken   string
	IsPartial   bool
}

// PropDefRow is a row in the PropDefinitions list.
type PropDefRow struct {
	ID        int64
	ShortName string
	DataType  string
}

// PropDefsListData holds data for the PropDefinitions list page.
type PropDefsListData struct {
	Props     []PropDefRow
	Total     int
	CSRFToken string
}

// PropDefFormData holds data for the PropDefinition create/edit form.
type PropDefFormData struct {
	ID           int64
	IsEdit       bool
	ShortName    string
	DataType     string
	DefaultValue string
	Error        string
	Errors       map[string]string
	CSRFToken    string
	IsPartial    bool
}

// ── Región ────────────────────────────────────────────────────────────────────

// RegionesList handles GET /regions
func (h *UIHandler) RegionesList(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	data := RegionesListData{}
	result, err := h.xoluClient.OQL(ctx, "SELECT * FROM regions ORDER BY description")
	if err == nil {
		for _, row := range result.Data {
			data.Regions = append(data.Regions, RegionRow{
				ID:          toInt64(row["id"]),
				Description: toString(row["description"]),
			})
		}
	}
	data.Total = len(data.Regions)
	data.CSRFToken = h.GetCSRFToken(w, r)
	h.page.Render(w, r, t("masters.region.list_title"), "/regions", RegionesListPage(data, t))
}

// RegionNew handles GET /regions/new
func (h *UIHandler) RegionNew(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	if IsHTMXRequest(r) {
		h.page.RenderPartial(w, h.regionFormPage(r, RegionFormData{IsPartial: true, CSRFToken: h.GetCSRFToken(w, r)}, t))
		return
	}
	h.page.Render(w, r, t("masters.region.new"), "/regions", h.regionFormPage(r, RegionFormData{CSRFToken: h.GetCSRFToken(w, r)}, t))
}

// RegionCreate handles POST /regions
func (h *UIHandler) RegionCreate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		h.page.RenderError(w, http.StatusBadRequest, t("error.generic"), t("error.invalid_form_data"))
		return
	}
	// English map key for RegionCreateSchema (T-27/T-40); RegionValuesToXolu
	// (below, after validation) is now a pass-through -- xolu storage uses
	// "description" directly, no bridge, matching entity names' precedent.
	data := map[string]any{"description": r.FormValue("description")}
	if errs := validateForm(data, schemaRegionCreate); errs != nil {
		h.page.Render(w, r, t("masters.region.new"), "/regions", h.regionFormPage(r, RegionFormData{
			Description: r.FormValue("description"),
			Error:       firstError(errs), Errors: errs,
			CSRFToken: h.GetCSRFToken(w, r),
		}, t))
		return
	}
	data = RegionValuesToXolu(data)
	if _, err := h.xoluClient.Create(ctx, "regions", data); err != nil {
		h.page.Render(w, r, t("masters.region.new"), "/regions", h.regionFormPage(r, RegionFormData{
			Description: r.FormValue("description"),
			Error:       t("error.create_failed") + err.Error(),
			CSRFToken:   h.GetCSRFToken(w, r),
		}, t))
		return
	}
	http.Redirect(w, r, "/regions", http.StatusSeeOther)
}

// RegionEdit handles GET /regions/{id}/edit
func (h *UIHandler) RegionEdit(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := masterID(r.URL.Path, "/regions/", "/edit")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.region.not_found"))
		return
	}
	entity, err := h.xoluClient.Get(ctx, "regions", id)
	if err != nil {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.region.not_found"))
		return
	}
	fd := RegionFormData{
		ID: id, IsEdit: true,
		Description: toString(entity.Data["description"]),
		CSRFToken:   h.GetCSRFToken(w, r),
	}
	fd.IsPartial = IsHTMXRequest(r)
	if fd.IsPartial {
		h.page.RenderPartial(w, h.regionFormPage(r, fd, t))
		return
	}
	h.page.Render(w, r, t("masters.region.edit"), "/regions", h.regionFormPage(r, fd, t))
}

// RegionUpdate handles POST /regions/{id}
func (h *UIHandler) RegionUpdate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := masterID(r.URL.Path, "/regions/", "")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.region.not_found"))
		return
	}
	if err := r.ParseForm(); err != nil {
		h.page.RenderError(w, http.StatusBadRequest, t("error.generic"), t("error.invalid_form_data"))
		return
	}
	data := map[string]any{"description": r.FormValue("description")}
	if errs := validateForm(data, schemaRegionCreate); errs != nil {
		h.page.Render(w, r, t("masters.region.edit"), "/regions", h.regionFormPage(r, RegionFormData{
			ID: id, IsEdit: true, Description: r.FormValue("description"),
			Error: firstError(errs), Errors: errs, CSRFToken: h.GetCSRFToken(w, r),
		}, t))
		return
	}
	data = RegionValuesToXolu(data)
	if _, err := h.xoluClient.Patch(ctx, "regions", id, data); err != nil {
		h.page.Render(w, r, t("masters.region.edit"), "/regions", h.regionFormPage(r, RegionFormData{
			ID:          id,
			IsEdit:      true,
			Description: r.FormValue("description"),
			Error:       t("error.update_failed") + err.Error(),
			CSRFToken:   h.GetCSRFToken(w, r),
		}, t))
		return
	}
	http.Redirect(w, r, "/regions", http.StatusSeeOther)
}

// RegionDelete handles POST /regions/{id}/delete
func (h *UIHandler) RegionDelete(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := masterID(r.URL.Path, "/regions/", "/delete")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.region.not_found"))
		return
	}
	// Guard: check if any ubicacion or asset references this region
	count := h.countRef(ctx, "locations", "region_id", id) +
		h.countRef(ctx, "assets", "region_id", id)
	if count > 0 {
		h.page.RenderError(w, http.StatusConflict, t("error.cannot_delete"), t("masters.region.in_use", count))
		return
	}
	if err := h.xoluClient.Delete(ctx, "regions", id); err != nil {
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.delete_failed"))
		return
	}
	http.Redirect(w, r, "/regions", http.StatusSeeOther)
}

// ── Moneda ────────────────────────────────────────────────────────────────────

func (h *UIHandler) MonedasList(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	data := MonedasListData{}
	result, err := h.xoluClient.OQL(ctx, "SELECT * FROM currencies ORDER BY iso_code")
	if err == nil {
		for _, row := range result.Data {
			data.Currencies = append(data.Currencies, MonedaRow{
				ID:      toInt64(row["id"]),
				IsoCode: toString(row["iso_code"]),
				Symbol:  toString(row["symbol"]),
			})
		}
	}
	data.Total = len(data.Currencies)
	data.CSRFToken = h.GetCSRFToken(w, r)
	h.page.Render(w, r, t("masters.currency.list_title"), "/currencies", MonedasListPage(data, t))
}

func (h *UIHandler) MonedaNew(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	if IsHTMXRequest(r) {
		h.page.RenderPartial(w, h.monedaFormPage(r, MonedaFormData{IsPartial: true, CSRFToken: h.GetCSRFToken(w, r)}, t))
		return
	}
	h.page.Render(w, r, t("masters.currency.new"), "/currencies", h.monedaFormPage(r, MonedaFormData{CSRFToken: h.GetCSRFToken(w, r)}, t))
}

func (h *UIHandler) MonedaCreate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		h.page.RenderError(w, http.StatusBadRequest, t("error.generic"), t("error.invalid_form_data"))
		return
	}
	// English map keys for MonedaCreateSchema (T-27/T-40); MonedaValuesToXolu
	// (below, after validation) is now a pass-through -- xolu storage uses
	// iso_code/symbol directly, no bridge, matching entity names' precedent.
	data := map[string]any{
		"iso_code": r.FormValue("iso_code"),
		"symbol":   r.FormValue("symbol"),
	}
	if errs := validateForm(data, schemaMonedaCreate); errs != nil {
		h.page.Render(w, r, t("masters.currency.new"), "/currencies", h.monedaFormPage(r, MonedaFormData{
			IsoCode: r.FormValue("iso_code"), Symbol: r.FormValue("symbol"),
			Error: firstError(errs), Errors: errs,
			CSRFToken: h.GetCSRFToken(w, r),
		}, t))
		return
	}
	data = MonedaValuesToXolu(data)
	if _, err := h.xoluClient.Create(ctx, "currencies", data); err != nil {
		h.page.Render(w, r, t("masters.currency.new"), "/currencies", h.monedaFormPage(r, MonedaFormData{
			IsoCode:   r.FormValue("iso_code"),
			Symbol:    r.FormValue("symbol"),
			Error:     t("error.create_failed") + err.Error(),
			CSRFToken: h.GetCSRFToken(w, r),
		}, t))
		return
	}
	http.Redirect(w, r, "/currencies", http.StatusSeeOther)
}

func (h *UIHandler) MonedaEdit(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := masterID(r.URL.Path, "/currencies/", "/edit")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.currency.not_found"))
		return
	}
	entity, err := h.xoluClient.Get(ctx, "currencies", id)
	if err != nil {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.currency.not_found"))
		return
	}
	fd := MonedaFormData{
		ID: id, IsEdit: true,
		IsoCode:   toString(entity.Data["iso_code"]),
		Symbol:    toString(entity.Data["symbol"]),
		CSRFToken: h.GetCSRFToken(w, r),
	}
	fd.IsPartial = IsHTMXRequest(r)
	if fd.IsPartial {
		h.page.RenderPartial(w, h.monedaFormPage(r, fd, t))
		return
	}
	h.page.Render(w, r, t("masters.currency.edit"), "/currencies", h.monedaFormPage(r, fd, t))
}

func (h *UIHandler) MonedaUpdate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := masterID(r.URL.Path, "/currencies/", "")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.currency.not_found"))
		return
	}
	if err := r.ParseForm(); err != nil {
		h.page.RenderError(w, http.StatusBadRequest, t("error.generic"), t("error.invalid_form_data"))
		return
	}
	data := map[string]any{
		"iso_code": r.FormValue("iso_code"),
		"symbol":   r.FormValue("symbol"),
	}
	if errs := validateForm(data, schemaMonedaCreate); errs != nil {
		h.page.Render(w, r, t("masters.currency.edit"), "/currencies", h.monedaFormPage(r, MonedaFormData{
			ID: id, IsEdit: true, IsoCode: r.FormValue("iso_code"), Symbol: r.FormValue("symbol"),
			Error: firstError(errs), Errors: errs, CSRFToken: h.GetCSRFToken(w, r),
		}, t))
		return
	}
	data = MonedaValuesToXolu(data)
	if _, err := h.xoluClient.Patch(ctx, "currencies", id, data); err != nil {
		h.page.Render(w, r, t("masters.currency.edit"), "/currencies", h.monedaFormPage(r, MonedaFormData{
			ID:        id,
			IsEdit:    true,
			IsoCode:   r.FormValue("iso_code"),
			Symbol:    r.FormValue("symbol"),
			Error:     t("error.update_failed") + err.Error(),
			CSRFToken: h.GetCSRFToken(w, r),
		}, t))
		return
	}
	http.Redirect(w, r, "/currencies", http.StatusSeeOther)
}

func (h *UIHandler) MonedaDelete(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := masterID(r.URL.Path, "/currencies/", "/delete")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.currency.not_found"))
		return
	}
	if h.countRef(ctx, "assets", "currency_id", id) > 0 {
		h.page.RenderError(w, http.StatusConflict, t("error.cannot_delete"), t("masters.currency.in_use"))
		return
	}
	if err := h.xoluClient.Delete(ctx, "currencies", id); err != nil {
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.delete_failed"))
		return
	}
	http.Redirect(w, r, "/currencies", http.StatusSeeOther)
}

// ── Centro de Costo ───────────────────────────────────────────────────────────

func (h *UIHandler) CentrosCostoList(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	data := CentrosCostoListData{}
	result, err := h.xoluClient.OQL(ctx, "SELECT * FROM cost_centers ORDER BY name")
	if err == nil {
		for _, row := range result.Data {
			data.CostCenters = append(data.CostCenters, CentroCostoRow{
				ID:             toInt64(row["id"]),
				AccountingCode: toString(row["accounting_code"]),
				Name:           toString(row["name"]),
			})
		}
	}
	data.Total = len(data.CostCenters)
	data.CSRFToken = h.GetCSRFToken(w, r)
	h.page.Render(w, r, t("masters.cost_center.list_title"), "/cost-centers", CentrosCostoListPage(data, t))
}

func (h *UIHandler) CentroCostoNew(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	h.page.Render(w, r, t("masters.cost_center.new"), "/cost-centers", CentroCostoFormPage(CentroCostoFormData{
		CSRFToken: h.GetCSRFToken(w, r),
	}, t))
}

func (h *UIHandler) CentroCostoCreate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		h.page.RenderError(w, http.StatusBadRequest, t("error.generic"), t("error.invalid_form_data"))
		return
	}
	// English map keys for CentroCostoCreateSchema (T-27/T-40);
	// CentroCostoValuesToXolu (below, after validation) is now a
	// pass-through -- xolu storage uses accounting_code/name directly.
	data := map[string]any{
		"accounting_code": r.FormValue("accounting_code"),
		"name":            r.FormValue("name"),
	}
	if errs := validateForm(data, schemaCentroCreate); errs != nil {
		h.page.Render(w, r, t("masters.cost_center.new"), "/cost-centers", CentroCostoFormPage(CentroCostoFormData{
			AccountingCode: r.FormValue("accounting_code"), Name: r.FormValue("name"),
			Error: firstError(errs), Errors: errs,
			CSRFToken: h.GetCSRFToken(w, r),
		}, t))
		return
	}
	data = CentroCostoValuesToXolu(data)
	if _, err := h.xoluClient.Create(ctx, "cost_centers", data); err != nil {
		h.page.Render(w, r, t("masters.cost_center.new"), "/cost-centers", CentroCostoFormPage(CentroCostoFormData{
			AccountingCode: r.FormValue("accounting_code"),
			Name:           r.FormValue("name"),
			Error:          t("error.create_failed") + err.Error(),
			CSRFToken:      h.GetCSRFToken(w, r),
		}, t))
		return
	}
	http.Redirect(w, r, "/cost-centers", http.StatusSeeOther)
}

func (h *UIHandler) CentroCostoEdit(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := masterID(r.URL.Path, "/cost-centers/", "/edit")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.cost_center.not_found"))
		return
	}
	entity, err := h.xoluClient.Get(ctx, "cost_centers", id)
	if err != nil {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.cost_center.not_found"))
		return
	}
	fd := CentroCostoFormData{
		ID: id, IsEdit: true,
		AccountingCode: toString(entity.Data["accounting_code"]),
		Name:           toString(entity.Data["name"]),
		CSRFToken:      h.GetCSRFToken(w, r),
	}
	fd.IsPartial = IsHTMXRequest(r)
	if fd.IsPartial {
		h.page.RenderPartial(w, CentroCostoFormPage(fd, t))
	} else {
		h.page.Render(w, r, t("masters.cost_center.edit"), "/cost-centers", CentroCostoFormPage(fd, t))
	}
}

func (h *UIHandler) CentroCostoUpdate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := masterID(r.URL.Path, "/cost-centers/", "")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.cost_center.not_found"))
		return
	}
	if err := r.ParseForm(); err != nil {
		h.page.RenderError(w, http.StatusBadRequest, t("error.generic"), t("error.invalid_form_data"))
		return
	}
	data := map[string]any{
		"accounting_code": r.FormValue("accounting_code"),
		"name":            r.FormValue("name"),
	}
	if errs := validateForm(data, schemaCentroCreate); errs != nil {
		fd2 := CentroCostoFormData{
			ID: id, IsEdit: true, AccountingCode: r.FormValue("accounting_code"), Name: r.FormValue("name"),
			Error: firstError(errs), Errors: errs, CSRFToken: h.GetCSRFToken(w, r),
		}
		if IsHTMXRequest(r) {
			fd2.IsPartial = true
			h.page.RenderPartial(w, CentroCostoFormPage(fd2, t))
			return
		}
		h.page.Render(w, r, t("masters.cost_center.edit"), "/cost-centers", CentroCostoFormPage(fd2, t))
		return
	}
	data = CentroCostoValuesToXolu(data)
	if _, err := h.xoluClient.Patch(ctx, "cost_centers", id, data); err != nil {
		fd3 := CentroCostoFormData{
			ID: id, IsEdit: true, AccountingCode: r.FormValue("accounting_code"), Name: r.FormValue("name"),
			Error: t("error.update_failed") + err.Error(), CSRFToken: h.GetCSRFToken(w, r),
		}
		if IsHTMXRequest(r) {
			fd3.IsPartial = true
			h.page.RenderPartial(w, CentroCostoFormPage(fd3, t))
			return
		}
		h.page.Render(w, r, t("masters.cost_center.edit"), "/cost-centers", CentroCostoFormPage(fd3, t))
		return
	}
	if IsHTMXRequest(r) {
		HTMXTrigger(w, "seamModal2Saved")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/cost-centers", http.StatusSeeOther)
}

func (h *UIHandler) CentroCostoDelete(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := masterID(r.URL.Path, "/cost-centers/", "/delete")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.cost_center.not_found"))
		return
	}
	if h.countRef(ctx, "assets", "cost_center_id", id) > 0 {
		h.page.RenderError(w, http.StatusConflict, t("error.cannot_delete"), t("masters.cost_center.in_use"))
		return
	}
	if err := h.xoluClient.Delete(ctx, "cost_centers", id); err != nil {
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.delete_failed"))
		return
	}
	http.Redirect(w, r, "/cost-centers", http.StatusSeeOther)
}

// ── Marca ─────────────────────────────────────────────────────────────────────

func (h *UIHandler) MarcasList(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	data := MarcasListData{}
	result, err := h.xoluClient.OQL(ctx, "SELECT * FROM brands ORDER BY name")
	if err == nil {
		for _, row := range result.Data {
			data.Brands = append(data.Brands, MarcaRow{
				ID:   toInt64(row["id"]),
				Code: toString(row["code"]),
				Name: toString(row["name"]),
			})
		}
	}
	data.Total = len(data.Brands)
	data.CSRFToken = h.GetCSRFToken(w, r)
	h.page.Render(w, r, t("masters.brand.list_title"), "/brands", MarcasListPage(data, t))
}

func (h *UIHandler) MarcaNew(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	h.page.Render(w, r, t("masters.brand.new"), "/brands", h.marcaFormPage(r, MarcaFormData{
		CSRFToken: h.GetCSRFToken(w, r),
	}, t))
}

func (h *UIHandler) MarcaCreate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		h.page.RenderError(w, http.StatusBadRequest, t("error.generic"), t("error.invalid_form_data"))
		return
	}
	// English map keys for MarcaCreateSchema (T-27/T-40); MarcaValuesToXolu
	// (below, after validation) is now a pass-through -- xolu storage uses
	// code/name directly.
	data := map[string]any{
		"code": r.FormValue("code"),
		"name": r.FormValue("name"),
	}
	if errs := validateForm(data, schemaMarcaCreate); errs != nil {
		h.page.Render(w, r, t("masters.brand.new"), "/brands", h.marcaFormPage(r, MarcaFormData{
			Code: r.FormValue("code"), Name: r.FormValue("name"),
			Error: firstError(errs), Errors: errs,
			CSRFToken: h.GetCSRFToken(w, r),
		}, t))
		return
	}
	data = MarcaValuesToXolu(data)
	if _, err := h.xoluClient.Create(ctx, "brands", data); err != nil {
		h.page.Render(w, r, t("masters.brand.new"), "/brands", h.marcaFormPage(r, MarcaFormData{
			Code:      r.FormValue("code"),
			Name:      r.FormValue("name"),
			Error:     t("error.create_failed") + err.Error(),
			CSRFToken: h.GetCSRFToken(w, r),
		}, t))
		return
	}
	http.Redirect(w, r, "/brands", http.StatusSeeOther)
}

func (h *UIHandler) MarcaEdit(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := masterID(r.URL.Path, "/brands/", "/edit")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.brand.not_found"))
		return
	}
	entity, err := h.xoluClient.Get(ctx, "brands", id)
	if err != nil {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.brand.not_found"))
		return
	}
	fd := MarcaFormData{
		ID: id, IsEdit: true,
		Code: toString(entity.Data["code"]), Name: toString(entity.Data["name"]),
		CSRFToken: h.GetCSRFToken(w, r),
	}
	fd.IsPartial = IsHTMXRequest(r)
	if fd.IsPartial {
		h.page.RenderPartial(w, h.marcaFormPage(r, fd, t))
	} else {
		h.page.Render(w, r, t("masters.brand.edit"), "/brands", h.marcaFormPage(r, fd, t))
	}
}

func (h *UIHandler) MarcaUpdate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := masterID(r.URL.Path, "/brands/", "")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.brand.not_found"))
		return
	}
	if err := r.ParseForm(); err != nil {
		h.page.RenderError(w, http.StatusBadRequest, t("error.generic"), t("error.invalid_form_data"))
		return
	}
	data := map[string]any{
		"code": r.FormValue("code"),
		"name": r.FormValue("name"),
	}
	if errs := validateForm(data, schemaMarcaCreate); errs != nil {
		fd2 := MarcaFormData{
			ID: id, IsEdit: true, Code: r.FormValue("code"), Name: r.FormValue("name"),
			Error: firstError(errs), Errors: errs, CSRFToken: h.GetCSRFToken(w, r),
		}
		if IsHTMXRequest(r) {
			fd2.IsPartial = true
			h.page.RenderPartial(w, h.marcaFormPage(r, fd2, t))
			return
		}
		h.page.Render(w, r, t("masters.brand.edit"), "/brands", h.marcaFormPage(r, fd2, t))
		return
	}
	data = MarcaValuesToXolu(data)
	if _, err := h.xoluClient.Patch(ctx, "brands", id, data); err != nil {
		fd3 := MarcaFormData{
			ID: id, IsEdit: true, Code: r.FormValue("code"), Name: r.FormValue("name"),
			Error: t("error.update_failed") + err.Error(), CSRFToken: h.GetCSRFToken(w, r),
		}
		if IsHTMXRequest(r) {
			fd3.IsPartial = true
			h.page.RenderPartial(w, h.marcaFormPage(r, fd3, t))
			return
		}
		h.page.Render(w, r, t("masters.brand.edit"), "/brands", h.marcaFormPage(r, fd3, t))
		return
	}
	if IsHTMXRequest(r) {
		HTMXTrigger(w, "seamModal2Saved")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/brands", http.StatusSeeOther)
}

func (h *UIHandler) MarcaDelete(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := masterID(r.URL.Path, "/brands/", "/delete")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.brand.not_found"))
		return
	}
	if h.countRef(ctx, "models", "brand_id", id) > 0 {
		h.page.RenderError(w, http.StatusConflict, t("error.cannot_delete"), t("masters.brand.in_use"))
		return
	}
	if err := h.xoluClient.Delete(ctx, "brands", id); err != nil {
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.delete_failed"))
		return
	}
	http.Redirect(w, r, "/brands", http.StatusSeeOther)
}

// ── Custodio ──────────────────────────────────────────────────────────────────

func (h *UIHandler) CustodiosList(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	data := CustodiosListData{}
	result, err := h.xoluClient.OQL(ctx, "SELECT * FROM custodians ORDER BY legal_name")
	if err == nil {
		for _, row := range result.Data {
			data.Custodios = append(data.Custodios, CustodioRow{
				ID:          toInt64(row["id"]),
				SubjectType: toString(row["subject_type"]),
				LegalName:   toString(row["legal_name"]),
				TaxId:       toString(row["tax_id"]),
				Email:       toString(row["email"]),
				Active:      toBool(row["active"]),
			})
		}
	}
	data.Total = len(data.Custodios)
	data.CSRFToken = h.GetCSRFToken(w, r)
	h.page.Render(w, r, t("masters.custodian.list_title"), "/custodians", CustodiosListPage(data, t))
}

func (h *UIHandler) CustodioNew(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	h.page.Render(w, r, t("masters.custodian.new"), "/custodians", CustodioFormPage(CustodioFormData{
		Active:    true,
		CSRFToken: h.GetCSRFToken(w, r),
	}, t))
}

func (h *UIHandler) CustodioCreate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		h.page.RenderError(w, http.StatusBadRequest, t("error.generic"), t("error.invalid_form_data"))
		return
	}
	// English map keys for CustodioCreateSchema (T-26). r.FormValue reads
	// unchanged (still the original Spanish HTML field names) -- only the
	// Go-level map key changes here. CustodioValuesToXolu (below, after
	// validation) converts back to xolu's Spanish-keyed storage shape.
	data := map[string]any{
		"subject_type": r.FormValue("subject_type"),
		"legal_name":   r.FormValue("legal_name"),
		"tax_id":       r.FormValue("tax_id"),
		"alias":        r.FormValue("alias"),
		"email":        r.FormValue("email"),
		"phone":        r.FormValue("phone"),
		"address":      r.FormValue("address"),
		"active":       r.FormValue("active") == "on",
	}
	if errs := validateForm(data, schemaCustodioCreate); errs != nil {
		h.page.Render(w, r, t("masters.custodian.new"), "/custodians", CustodioFormPage(CustodioFormData{
			SubjectType: r.FormValue("subject_type"), LegalName: r.FormValue("legal_name"),
			TaxId: r.FormValue("tax_id"), Email: r.FormValue("email"),
			Error: firstError(errs), Errors: errs,
			CSRFToken: h.GetCSRFToken(w, r),
		}, t))
		return
	}
	data = CustodioValuesToXolu(data)
	if _, err := h.xoluClient.Create(ctx, "custodians", data); err != nil {
		h.page.Render(w, r, t("masters.custodian.new"), "/custodians", CustodioFormPage(CustodioFormData{
			SubjectType: r.FormValue("subject_type"),
			LegalName:   r.FormValue("legal_name"),
			TaxId:       r.FormValue("tax_id"),
			Alias:       r.FormValue("alias"),
			Email:       r.FormValue("email"),
			Phone:       r.FormValue("phone"),
			Address:     r.FormValue("address"),
			Active:      r.FormValue("active") == "on",
			Error:       t("error.create_failed") + err.Error(),
			CSRFToken:   h.GetCSRFToken(w, r),
		}, t))
		return
	}
	http.Redirect(w, r, "/custodians", http.StatusSeeOther)
}

func (h *UIHandler) CustodioEdit(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := masterID(r.URL.Path, "/custodians/", "/edit")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.custodian.not_found"))
		return
	}
	entity, err := h.xoluClient.Get(ctx, "custodians", id)
	if err != nil {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.custodian.not_found"))
		return
	}
	fd := CustodioFormData{
		ID: id, IsEdit: true,
		SubjectType: toString(entity.Data["subject_type"]),
		LegalName:   toString(entity.Data["legal_name"]),
		TaxId:       toString(entity.Data["tax_id"]),
		Alias:       toString(entity.Data["alias"]),
		Email:       toString(entity.Data["email"]),
		Phone:       toString(entity.Data["phone"]),
		Address:     toString(entity.Data["address"]),
		Active:      toBool(entity.Data["active"]),
		CSRFToken:   h.GetCSRFToken(w, r),
	}
	fd.IsPartial = IsHTMXRequest(r)
	if fd.IsPartial {
		h.page.RenderPartial(w, CustodioFormPage(fd, t))
	} else {
		h.page.Render(w, r, t("masters.custodian.edit"), "/custodians", CustodioFormPage(fd, t))
	}
}

func (h *UIHandler) CustodioUpdate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := masterID(r.URL.Path, "/custodians/", "")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.custodian.not_found"))
		return
	}
	if err := r.ParseForm(); err != nil {
		h.page.RenderError(w, http.StatusBadRequest, t("error.generic"), t("error.invalid_form_data"))
		return
	}
	// See CustodioCreate's comment above.
	data := map[string]any{
		"subject_type": r.FormValue("subject_type"),
		"legal_name":   r.FormValue("legal_name"),
		"tax_id":       r.FormValue("tax_id"),
		"alias":        r.FormValue("alias"),
		"email":        r.FormValue("email"),
		"phone":        r.FormValue("phone"),
		"address":      r.FormValue("address"),
		"active":       r.FormValue("active") == "on",
	}
	if errs := validateForm(data, schemaCustodioCreate); errs != nil {
		fd2 := CustodioFormData{
			ID: id, IsEdit: true, SubjectType: r.FormValue("subject_type"),
			LegalName: r.FormValue("legal_name"),
			TaxId:     r.FormValue("tax_id"), Email: r.FormValue("email"),
			Error: firstError(errs), Errors: errs, CSRFToken: h.GetCSRFToken(w, r),
		}
		if IsHTMXRequest(r) {
			fd2.IsPartial = true
			h.page.RenderPartial(w, CustodioFormPage(fd2, t))
			return
		}
		h.page.Render(w, r, t("masters.custodian.edit"), "/custodians", CustodioFormPage(fd2, t))
		return
	}
	data = CustodioValuesToXolu(data)
	if _, err := h.xoluClient.Patch(ctx, "custodians", id, data); err != nil {
		fd3 := CustodioFormData{
			ID: id, IsEdit: true, SubjectType: r.FormValue("subject_type"),
			LegalName: r.FormValue("legal_name"),
			TaxId:     r.FormValue("tax_id"),
			Alias:     r.FormValue("alias"), Email: r.FormValue("email"),
			Phone: r.FormValue("phone"), Address: r.FormValue("address"),
			Active: r.FormValue("active") == "on",
			Error:  t("error.update_failed") + err.Error(), CSRFToken: h.GetCSRFToken(w, r),
		}
		if IsHTMXRequest(r) {
			fd3.IsPartial = true
			h.page.RenderPartial(w, CustodioFormPage(fd3, t))
			return
		}
		h.page.Render(w, r, t("masters.custodian.edit"), "/custodians", CustodioFormPage(fd3, t))
		return
	}
	if IsHTMXRequest(r) {
		HTMXTrigger(w, "seamModal2Saved")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/custodians", http.StatusSeeOther)
}

func (h *UIHandler) CustodioDelete(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := masterID(r.URL.Path, "/custodians/", "/delete")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.custodian.not_found"))
		return
	}
	if h.countRef(ctx, "assets", "custodian_id", id) > 0 {
		h.page.RenderError(w, http.StatusConflict, t("error.cannot_delete"), t("masters.custodian.in_use"))
		return
	}
	if err := h.xoluClient.Delete(ctx, "custodians", id); err != nil {
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.delete_failed"))
		return
	}
	http.Redirect(w, r, "/custodians", http.StatusSeeOther)
}

// ── Alerta ────────────────────────────────────────────────────────────────────

func (h *UIHandler) AlertasMasterList(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	data := AlertasListData{}
	result, err := h.xoluClient.OQL(ctx, "SELECT * FROM alert_types ORDER BY type, criticality")
	if err == nil {
		for _, row := range result.Data {
			data.Alertas = append(data.Alertas, AlertaRow{
				ID:          toInt64(row["id"]),
				Type:        AlertaEnumValueFromXoluValue("type", toString(row["type"])),
				Description: toString(row["description"]),
				Criticality: AlertaEnumValueFromXoluValue("criticality", toString(row["criticality"])),
			})
		}
	}
	data.Total = len(data.Alertas)
	data.CSRFToken = h.GetCSRFToken(w, r)
	h.page.Render(w, r, t("masters.alert_type.list_title"), "/alert-types", AlertasMasterListPage(data, t))
}

func (h *UIHandler) AlertaMasterNew(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	if IsHTMXRequest(r) {
		h.page.RenderPartial(w, AlertaMasterFormPage(AlertaMasterFormData{IsPartial: true, CSRFToken: h.GetCSRFToken(w, r)}, t))
		return
	}
	h.page.Render(w, r, t("masters.alert_type.new"), "/alert-types", AlertaMasterFormPage(AlertaMasterFormData{CSRFToken: h.GetCSRFToken(w, r)}, t))
}

func (h *UIHandler) AlertaMasterCreate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		h.page.RenderError(w, http.StatusBadRequest, t("error.generic"), t("error.invalid_form_data"))
		return
	}
	// English map keys for AlertaCreateSchema (T-26); AlertaEnumValueFromXoluValue
	// translates the two enum fields' Spanish form values to English so
	// they validate. AlertaValuesToXolu (below, after validation) converts
	// back to xolu's Spanish-keyed/valued storage shape.
	data := map[string]any{
		"type":        AlertaEnumValueFromXoluValue("type", r.FormValue("type")),
		"description": r.FormValue("description"),
		"criticality": AlertaEnumValueFromXoluValue("criticality", r.FormValue("criticality")),
	}
	if errs := validateForm(data, schemaAlertaCreate); errs != nil {
		h.page.Render(w, r, t("masters.alert_type.new"), "/alert-types", AlertaMasterFormPage(AlertaMasterFormData{
			Type: r.FormValue("type"), Description: r.FormValue("description"), Criticality: r.FormValue("criticality"),
			Error: firstError(errs), Errors: errs,
			CSRFToken: h.GetCSRFToken(w, r),
		}, t))
		return
	}
	data = AlertaValuesToXolu(data)
	if _, err := h.xoluClient.Create(ctx, "alert_types", data); err != nil {
		h.page.Render(w, r, t("masters.alert_type.new"), "/alert-types", AlertaMasterFormPage(AlertaMasterFormData{
			Type:        r.FormValue("type"),
			Description: r.FormValue("description"),
			Criticality: r.FormValue("criticality"),
			Error:       t("error.create_failed") + err.Error(),
			CSRFToken:   h.GetCSRFToken(w, r),
		}, t))
		return
	}
	http.Redirect(w, r, "/alert-types", http.StatusSeeOther)
}

func (h *UIHandler) AlertaMasterEdit(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := masterID(r.URL.Path, "/alert-types/", "/edit")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.alert_type.not_found"))
		return
	}
	entity, err := h.xoluClient.Get(ctx, "alert_types", id)
	if err != nil {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.alert_type.not_found"))
		return
	}
	fd := AlertaMasterFormData{
		ID: id, IsEdit: true,
		Type:        AlertaEnumValueFromXoluValue("type", toString(entity.Data["type"])),
		Description: toString(entity.Data["description"]),
		Criticality: AlertaEnumValueFromXoluValue("criticality", toString(entity.Data["criticality"])),
		CSRFToken:   h.GetCSRFToken(w, r),
	}
	fd.IsPartial = IsHTMXRequest(r)
	if fd.IsPartial {
		h.page.RenderPartial(w, AlertaMasterFormPage(fd, t))
		return
	}
	h.page.Render(w, r, t("masters.alert_type.edit"), "/alert-types", AlertaMasterFormPage(fd, t))
}

func (h *UIHandler) AlertaMasterUpdate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := masterID(r.URL.Path, "/alert-types/", "")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.alert_type.not_found"))
		return
	}
	if err := r.ParseForm(); err != nil {
		h.page.RenderError(w, http.StatusBadRequest, t("error.generic"), t("error.invalid_form_data"))
		return
	}
	// See AlertaMasterCreate's comment above.
	data := map[string]any{
		"type":        AlertaEnumValueFromXoluValue("type", r.FormValue("type")),
		"description": r.FormValue("description"),
		"criticality": AlertaEnumValueFromXoluValue("criticality", r.FormValue("criticality")),
	}
	if errs := validateForm(data, schemaAlertaCreate); errs != nil {
		h.page.Render(w, r, t("masters.alert_type.edit"), "/alert-types", AlertaMasterFormPage(AlertaMasterFormData{
			ID: id, IsEdit: true, Type: r.FormValue("type"), Description: r.FormValue("description"), Criticality: r.FormValue("criticality"),
			Error: firstError(errs), Errors: errs, CSRFToken: h.GetCSRFToken(w, r),
		}, t))
		return
	}
	data = AlertaValuesToXolu(data)
	if _, err := h.xoluClient.Patch(ctx, "alert_types", id, data); err != nil {
		h.page.Render(w, r, t("masters.alert_type.edit"), "/alert-types", AlertaMasterFormPage(AlertaMasterFormData{
			ID:          id,
			IsEdit:      true,
			Type:        r.FormValue("type"),
			Description: r.FormValue("description"),
			Criticality: r.FormValue("criticality"),
			Error:       t("error.update_failed") + err.Error(),
			CSRFToken:   h.GetCSRFToken(w, r),
		}, t))
		return
	}
	http.Redirect(w, r, "/alert-types", http.StatusSeeOther)
}

func (h *UIHandler) AlertaMasterDelete(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := masterID(r.URL.Path, "/alert-types/", "/delete")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.alert_type.not_found"))
		return
	}
	// Guard: check service_configs within assets that reference this alert
	// service_configs is a JSON array — OQL JSON_CONTAINS check
	countResult, _ := h.xoluClient.OQL(ctx,
		fmt.Sprintf("SELECT COUNT(*) as count FROM assets WHERE JSON_CONTAINS(service_configs, '{\"alerta_id\": %d}')", id))
	if countResult != nil && len(countResult.Data) > 0 && toInt(countResult.Data[0]["count"]) > 0 {
		h.page.RenderError(w, http.StatusConflict, t("error.cannot_delete"), t("masters.alert_type.in_use"))
		return
	}
	if err := h.xoluClient.Delete(ctx, "alert_types", id); err != nil {
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.delete_failed"))
		return
	}
	http.Redirect(w, r, "/alert-types", http.StatusSeeOther)
}

// ── PropDefinition ────────────────────────────────────────────────────────────

func (h *UIHandler) PropDefsList(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	data := PropDefsListData{}
	result, err := h.xoluClient.OQL(ctx, "SELECT * FROM prop_definitions ORDER BY short_name")
	if err == nil {
		for _, row := range result.Data {
			data.Props = append(data.Props, PropDefRow{
				ID:        toInt64(row["id"]),
				ShortName: toString(row["short_name"]),
				DataType:  PropDefEnumValueFromXoluValue("data_type", toString(row["data_type"])),
			})
		}
	}
	data.Total = len(data.Props)
	data.CSRFToken = h.GetCSRFToken(w, r)
	h.page.Render(w, r, t("masters.propdef.list_title"), "/properties", PropDefsListPage(data, t))
}

func (h *UIHandler) PropDefNew(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	if IsHTMXRequest(r) {
		h.page.RenderPartial(w, PropDefFormPage(PropDefFormData{IsPartial: true, CSRFToken: h.GetCSRFToken(w, r)}, t))
		return
	}
	h.page.Render(w, r, t("masters.propdef.new"), "/properties", PropDefFormPage(PropDefFormData{CSRFToken: h.GetCSRFToken(w, r)}, t))
}

func (h *UIHandler) PropDefCreate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		h.page.RenderError(w, http.StatusBadRequest, t("error.generic"), t("error.invalid_form_data"))
		return
	}
	// Enforce short_name uniqueness within tenant
	existing, _ := h.xoluClient.OQL(ctx, fmt.Sprintf(
		"SELECT COUNT(*) as count FROM prop_definitions WHERE short_name = '%s'",
		r.FormValue("nombre_corto"),
	))
	if existing != nil && len(existing.Data) > 0 && toInt(existing.Data[0]["count"]) > 0 {
		h.page.Render(w, r, t("masters.propdef.new"), "/properties", PropDefFormPage(PropDefFormData{
			ShortName:    r.FormValue("nombre_corto"),
			DataType:     r.FormValue("tipo_dato"),
			DefaultValue: r.FormValue("valor_defecto"),
			Error:        t("masters.propdef.duplicate_name"),
			CSRFToken:    h.GetCSRFToken(w, r),
		}, t))
		return
	}
	// English map keys for PropDefinitionCreateSchema (T-26); the
	// short_name uniqueness check above queries xolu's now-English-keyed
	// storage directly. PropDefValuesToXolu (below, after validation)
	// converts back to xolu's Spanish-*valued* (but now English-keyed)
	// storage shape -- data_type's own enum values stay translated,
	// matching Alerta's own precedent (field names go English, enum
	// values stay bridged).
	data := map[string]any{
		"short_name":    r.FormValue("nombre_corto"),
		"data_type":     PropDefEnumValueFromXoluValue("data_type", r.FormValue("tipo_dato")),
		"default_value": r.FormValue("valor_defecto"),
	}
	if errs := validateForm(data, schemaPropDefCreate); errs != nil {
		h.page.Render(w, r, t("masters.propdef.new"), "/properties", PropDefFormPage(PropDefFormData{
			ShortName: r.FormValue("nombre_corto"), DataType: r.FormValue("tipo_dato"),
			Error: firstError(errs), Errors: errs,
			CSRFToken: h.GetCSRFToken(w, r),
		}, t))
		return
	}
	data = PropDefValuesToXolu(data)
	if _, err := h.xoluClient.Create(ctx, "prop_definitions", data); err != nil {
		h.page.Render(w, r, t("masters.propdef.new"), "/properties", PropDefFormPage(PropDefFormData{
			ShortName:    r.FormValue("nombre_corto"),
			DataType:     r.FormValue("tipo_dato"),
			DefaultValue: r.FormValue("valor_defecto"),
			Error:        t("error.create_failed") + err.Error(),
			CSRFToken:    h.GetCSRFToken(w, r),
		}, t))
		return
	}
	http.Redirect(w, r, "/properties", http.StatusSeeOther)
}

func (h *UIHandler) PropDefEdit(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := masterID(r.URL.Path, "/properties/", "/edit")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.propdef.not_found"))
		return
	}
	entity, err := h.xoluClient.Get(ctx, "prop_definitions", id)
	if err != nil {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.propdef.not_found"))
		return
	}
	fd := PropDefFormData{
		ID: id, IsEdit: true,
		ShortName:    toString(entity.Data["short_name"]),
		DataType:     PropDefEnumValueFromXoluValue("data_type", toString(entity.Data["data_type"])),
		DefaultValue: toString(entity.Data["default_value"]),
		CSRFToken:    h.GetCSRFToken(w, r),
	}
	fd.IsPartial = IsHTMXRequest(r)
	if fd.IsPartial {
		h.page.RenderPartial(w, PropDefFormPage(fd, t))
		return
	}
	h.page.Render(w, r, t("masters.propdef.edit"), "/properties", PropDefFormPage(fd, t))
}

func (h *UIHandler) PropDefUpdate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := masterID(r.URL.Path, "/properties/", "")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.propdef.not_found"))
		return
	}
	if err := r.ParseForm(); err != nil {
		h.page.RenderError(w, http.StatusBadRequest, t("error.generic"), t("error.invalid_form_data"))
		return
	}
	// Check uniqueness excluding self
	existing, _ := h.xoluClient.OQL(ctx, fmt.Sprintf(
		"SELECT COUNT(*) as count FROM prop_definitions WHERE short_name = '%s' AND id != %d",
		r.FormValue("nombre_corto"), id,
	))
	if existing != nil && len(existing.Data) > 0 && toInt(existing.Data[0]["count"]) > 0 {
		h.page.Render(w, r, t("masters.propdef.edit"), "/properties", PropDefFormPage(PropDefFormData{
			ID:           id,
			IsEdit:       true,
			ShortName:    r.FormValue("nombre_corto"),
			DataType:     r.FormValue("tipo_dato"),
			DefaultValue: r.FormValue("valor_defecto"),
			Error:        t("masters.propdef.duplicate_name_other"),
			CSRFToken:    h.GetCSRFToken(w, r),
		}, t))
		return
	}
	// See PropDefCreate's comment above.
	data := map[string]any{
		"short_name":    r.FormValue("nombre_corto"),
		"data_type":     PropDefEnumValueFromXoluValue("data_type", r.FormValue("tipo_dato")),
		"default_value": r.FormValue("valor_defecto"),
	}
	if errs := validateForm(data, schemaPropDefCreate); errs != nil {
		h.page.Render(w, r, t("masters.propdef.edit"), "/properties", PropDefFormPage(PropDefFormData{
			ID: id, IsEdit: true, ShortName: r.FormValue("nombre_corto"), DataType: r.FormValue("tipo_dato"),
			Error: firstError(errs), Errors: errs, CSRFToken: h.GetCSRFToken(w, r),
		}, t))
		return
	}
	data = PropDefValuesToXolu(data)
	if _, err := h.xoluClient.Patch(ctx, "prop_definitions", id, data); err != nil {
		h.page.Render(w, r, t("masters.propdef.edit"), "/properties", PropDefFormPage(PropDefFormData{
			ID:           id,
			IsEdit:       true,
			ShortName:    r.FormValue("nombre_corto"),
			DataType:     r.FormValue("tipo_dato"),
			DefaultValue: r.FormValue("valor_defecto"),
			Error:        t("error.update_failed") + err.Error(),
			CSRFToken:    h.GetCSRFToken(w, r),
		}, t))
		return
	}
	http.Redirect(w, r, "/properties", http.StatusSeeOther)
}

func (h *UIHandler) PropDefDelete(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := masterID(r.URL.Path, "/properties/", "/delete")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.propdef.not_found"))
		return
	}
	// Guard: refuse if any associations exist
	if h.countRef(ctx, "prop_associations", "prop_definition_id", id) > 0 {
		h.page.RenderError(w, http.StatusConflict, t("error.cannot_delete"), t("masters.propdef.in_use"))
		return
	}
	if err := h.xoluClient.Delete(ctx, "prop_definitions", id); err != nil {
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.delete_failed"))
		return
	}
	http.Redirect(w, r, "/properties", http.StatusSeeOther)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// masterID extracts the integer ID from a URL path of the form
// {prefix}{id}{suffix}. Returns 0 if parsing fails.
func masterID(path, prefix, suffix string) int64 {
	s := path
	if prefix != "" && strings.HasPrefix(s, prefix) {
		s = s[len(prefix):]
	}
	s = strings.TrimSuffix(s, "/edit")
	s = strings.TrimSuffix(s, "/delete")
	s = strings.TrimSuffix(s, "/summary")
	if suffix != "" && strings.HasSuffix(s, suffix) {
		s = s[:len(s)-len(suffix)]
	}
	s = strings.Trim(s, "/")
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id <= 0 {
		return 0
	}
	return id
}

// countRef returns the count of rows in entity where field = id.
// Used for delete guards across all master table handlers.
func (h *UIHandler) countRef(ctx context.Context, entity, field string, id int64) int {
	result, err := h.xoluClient.OQL(ctx,
		fmt.Sprintf("SELECT COUNT(*) as count FROM %s WHERE %s = %d", entity, field, id),
	)
	if err != nil || len(result.Data) == 0 {
		return 0
	}
	return toInt(result.Data[0]["count"])
}

// ── Modelo (P-5) ──────────────────────────────────────────────────────────────

// ModeloRow is a row in the Modelos list.
type ModeloRow struct {
	ID        int64
	BrandID   int64
	BrandName string
	Name      string
}

// ModelosListData holds data for the Modelos list page.
type ModelosListData struct {
	Modelos   []ModeloRow
	Total     int
	CSRFToken string
}

// ModeloFormData holds data for the Modelo create/edit form.
type ModeloFormData struct {
	ID        int64
	IsEdit    bool
	IsPartial bool
	BrandID   int64
	Name      string
	Marcas    []MarcaRow // dropdown data
	Error     string
	Errors    map[string]string
	CSRFToken string
}

func (h *UIHandler) ModelosList(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	data := ModelosListData{}
	// Join-style OQL: fetch modelos with brand name
	result, err := h.xoluClient.OQL(ctx, "SELECT * FROM models ORDER BY name")
	if err == nil {
		// Build a brand name lookup. Query column fixed here from
		// "nombre" to "name" -- a real, pre-existing bug found as a
		// direct side effect of this rename work, not part of T-42's
		// own scope: brands' OWN rename already completed (T-40,
		// marcaFieldToXolu is empty, an identity pass-through), so its
		// xolu column has been "name" for a while -- this one
		// cross-reference query was never updated to match, confirmed
		// directly (SELECT id, nombre FROM brands returns null for
		// every row against the real running store) before fixing it.
		marcas := map[int64]string{}
		if mr, e := h.xoluClient.OQL(ctx, "SELECT id, name FROM brands"); e == nil {
			for _, row := range mr.Data {
				marcas[toInt64(row["id"])] = toString(row["name"])
			}
		}
		for _, row := range result.Data {
			mid := toInt64(row["brand_id"])
			data.Modelos = append(data.Modelos, ModeloRow{
				ID:        toInt64(row["id"]),
				BrandID:   mid,
				BrandName: marcas[mid],
				Name:      toString(row["name"]),
			})
		}
	}
	data.Total = len(data.Modelos)
	data.CSRFToken = h.GetCSRFToken(w, r)
	h.page.Render(w, r, t("masters.model.list_title"), "/models", ModelosListPage(data, t))
}

func (h *UIHandler) ModeloNew(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	data := ModeloFormData{CSRFToken: h.GetCSRFToken(w, r)}
	data.Marcas = h.loadMarcas(ctx)
	h.page.Render(w, r, t("masters.model.new"), "/models", ModeloFormPage(data, t))
}

func (h *UIHandler) ModeloCreate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		h.page.RenderError(w, http.StatusBadRequest, t("error.generic"), t("error.invalid_form_data"))
		return
	}
	// English map keys for ModeloCreateSchema (T-27); ModeloValuesToXolu
	// (below, after validation) converts back to xolu's Spanish-keyed
	// storage shape.
	marcaID := toInt64FromString(r.FormValue("marca_id"))
	entity := map[string]any{
		"brand_id": marcaID,
		"name":     r.FormValue("nombre"),
	}
	if errs := validateForm(entity, schemaModeloCreate); errs != nil {
		fd := ModeloFormData{
			BrandID: marcaID, Name: r.FormValue("nombre"),
			Marcas: h.loadMarcas(ctx),
			Error:  firstError(errs), Errors: errs, CSRFToken: h.GetCSRFToken(w, r),
		}
		h.page.Render(w, r, t("masters.model.new"), "/models", ModeloFormPage(fd, t))
		return
	}
	entity = ModeloValuesToXolu(entity)
	if _, err := h.xoluClient.Create(ctx, "models", entity); err != nil {
		fd := ModeloFormData{
			BrandID:   marcaID,
			Name:      r.FormValue("nombre"),
			Marcas:    h.loadMarcas(ctx),
			Error:     t("error.create_failed") + err.Error(),
			CSRFToken: h.GetCSRFToken(w, r),
		}
		h.page.Render(w, r, t("masters.model.new"), "/models", ModeloFormPage(fd, t))
		return
	}
	http.Redirect(w, r, "/models", http.StatusSeeOther)
}

func (h *UIHandler) ModeloEdit(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := masterID(r.URL.Path, "/models/", "/edit")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.model.not_found"))
		return
	}
	entity, err := h.xoluClient.Get(ctx, "models", id)
	if err != nil {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.model.not_found"))
		return
	}
	fd := ModeloFormData{
		ID:        id,
		IsEdit:    true,
		BrandID:   toInt64(entity.Data["brand_id"]),
		Name:      toString(entity.Data["name"]),
		Marcas:    h.loadMarcas(ctx),
		CSRFToken: h.GetCSRFToken(w, r),
	}
	fd.IsPartial = IsHTMXRequest(r)
	if fd.IsPartial {
		h.page.RenderPartial(w, ModeloFormPage(fd, t))
	} else {
		h.page.Render(w, r, t("masters.model.edit"), "/models", ModeloFormPage(fd, t))
	}
}

func (h *UIHandler) ModeloUpdate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := masterID(r.URL.Path, "/models/", "")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.model.not_found"))
		return
	}
	if err := r.ParseForm(); err != nil {
		h.page.RenderError(w, http.StatusBadRequest, t("error.generic"), t("error.invalid_form_data"))
		return
	}
	marcaID := toInt64FromString(r.FormValue("marca_id"))
	entity := map[string]any{
		"brand_id": marcaID,
		"name":     r.FormValue("nombre"),
	}
	if errs := validateForm(entity, schemaModeloCreate); errs != nil {
		fd := ModeloFormData{
			ID: id, IsEdit: true, BrandID: marcaID, Name: r.FormValue("nombre"),
			Marcas: h.loadMarcas(ctx),
			Error:  firstError(errs), Errors: errs, CSRFToken: h.GetCSRFToken(w, r),
		}
		if IsHTMXRequest(r) {
			fd.IsPartial = true
			h.page.RenderPartial(w, ModeloFormPage(fd, t))
			return
		}
		h.page.Render(w, r, t("masters.model.edit"), "/models", ModeloFormPage(fd, t))
		return
	}
	entity = ModeloValuesToXolu(entity)
	if _, err := h.xoluClient.Patch(ctx, "models", id, entity); err != nil {
		fd := ModeloFormData{
			ID: id, IsEdit: true,
			BrandID:   marcaID,
			Name:      r.FormValue("nombre"),
			Marcas:    h.loadMarcas(ctx),
			Error:     t("error.update_failed") + err.Error(),
			CSRFToken: h.GetCSRFToken(w, r),
		}
		if IsHTMXRequest(r) {
			fd.IsPartial = true
			h.page.RenderPartial(w, ModeloFormPage(fd, t))
			return
		}
		h.page.Render(w, r, t("masters.model.edit"), "/models", ModeloFormPage(fd, t))
		return
	}
	if IsHTMXRequest(r) {
		HTMXTrigger(w, "seamModal2Saved")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/models", http.StatusSeeOther)
}

func (h *UIHandler) ModeloDelete(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := masterID(r.URL.Path, "/models/", "/delete")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.model.not_found"))
		return
	}
	if h.countRef(ctx, "assets", "model_id", id) > 0 {
		h.page.RenderError(w, http.StatusConflict, t("error.cannot_delete"), t("masters.model.in_use"))
		return
	}
	if err := h.xoluClient.Delete(ctx, "models", id); err != nil {
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.delete_failed"))
		return
	}
	http.Redirect(w, r, "/models", http.StatusSeeOther)
}

// loadMarcas returns all marcas for use in dropdowns.
func (h *UIHandler) loadMarcas(ctx context.Context) []MarcaRow {
	result, err := h.xoluClient.OQL(ctx, "SELECT * FROM brands ORDER BY name")
	if err != nil {
		return nil
	}
	var marcas []MarcaRow
	for _, row := range result.Data {
		marcas = append(marcas, MarcaRow{
			ID:   toInt64(row["id"]),
			Code: toString(row["code"]),
			Name: toString(row["name"]),
		})
	}
	return marcas
}

// ── Ubicación (P-6) ───────────────────────────────────────────────────────────

// UbicacionRow is a row in the Ubicaciones list.
type UbicacionRow struct {
	ID          int64
	ShortName   string
	Description string
	RegionID    int64
	RegionDesc  string
}

// UbicacionesListData holds data for the Ubicaciones list page.
type UbicacionesListData struct {
	Locations []UbicacionRow
	Total     int
	CSRFToken string
}

// UbicacionFormData holds data for the Ubicación create/edit form.
type UbicacionFormData struct {
	ID          int64
	IsEdit      bool
	IsPartial   bool
	ShortName   string
	Description string
	RegionID    int64
	Regiones    []RegionRow // dropdown data
	Error       string
	Errors      map[string]string
	CSRFToken   string
}

func (h *UIHandler) UbicacionesList(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	data := UbicacionesListData{}
	result, err := h.xoluClient.OQL(ctx, "SELECT * FROM locations ORDER BY short_name")
	if err == nil {
		regiones := map[int64]string{}
		if rr, e := h.xoluClient.OQL(ctx, "SELECT id, description FROM regions"); e == nil {
			for _, row := range rr.Data {
				regiones[toInt64(row["id"])] = toString(row["description"])
			}
		}
		for _, row := range result.Data {
			rid := toInt64(row["region_id"])
			data.Locations = append(data.Locations, UbicacionRow{
				ID:          toInt64(row["id"]),
				ShortName:   toString(row["short_name"]),
				Description: toString(row["description"]),
				RegionID:    rid,
				RegionDesc:  regiones[rid],
			})
		}
	}
	data.Total = len(data.Locations)
	data.CSRFToken = h.GetCSRFToken(w, r)
	h.page.Render(w, r, t("masters.location.list_title"), "/locations", UbicacionesListPage(data, t))
}

func (h *UIHandler) UbicacionNew(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	data := UbicacionFormData{
		CSRFToken: h.GetCSRFToken(w, r),
		Regiones:  h.loadRegiones(ctx),
	}
	h.page.Render(w, r, t("masters.location.new"), "/locations", UbicacionFormPage(data, t))
}

func (h *UIHandler) UbicacionCreate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		h.page.RenderError(w, http.StatusBadRequest, t("error.generic"), t("error.invalid_form_data"))
		return
	}
	// English map keys for UbicacionCreateSchema (T-27/T-40);
	// UbicacionValuesToXolu (below, after validation) is now a
	// pass-through -- xolu storage uses short_name/description directly.
	entity := map[string]any{
		"short_name":  r.FormValue("short_name"),
		"description": r.FormValue("description"),
		"region_id":   toInt64FromString(r.FormValue("region_id")),
	}
	if errs := validateForm(entity, schemaUbicacionCreate); errs != nil {
		fd := UbicacionFormData{
			ShortName: r.FormValue("short_name"), Description: r.FormValue("description"),
			RegionID: toInt64FromString(r.FormValue("region_id")),
			Regiones: h.loadRegiones(ctx),
			Error:    firstError(errs), Errors: errs, CSRFToken: h.GetCSRFToken(w, r),
		}
		h.page.Render(w, r, t("masters.location.new"), "/locations", UbicacionFormPage(fd, t))
		return
	}
	entity = UbicacionValuesToXolu(entity)
	if _, err := h.xoluClient.Create(ctx, "locations", entity); err != nil {
		fd := UbicacionFormData{
			ShortName:   r.FormValue("short_name"),
			Description: r.FormValue("description"),
			RegionID:    toInt64FromString(r.FormValue("region_id")),
			Regiones:    h.loadRegiones(ctx),
			Error:       t("error.create_failed") + err.Error(),
			CSRFToken:   h.GetCSRFToken(w, r),
		}
		h.page.Render(w, r, t("masters.location.new"), "/locations", UbicacionFormPage(fd, t))
		return
	}
	http.Redirect(w, r, "/locations", http.StatusSeeOther)
}

func (h *UIHandler) UbicacionEdit(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := masterID(r.URL.Path, "/locations/", "/edit")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.location.not_found"))
		return
	}
	entity, err := h.xoluClient.Get(ctx, "locations", id)
	if err != nil {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.location.not_found"))
		return
	}
	fd := UbicacionFormData{
		ID: id, IsEdit: true,
		ShortName:   toString(entity.Data["short_name"]),
		Description: toString(entity.Data["description"]),
		RegionID:    toInt64(entity.Data["region_id"]),
		Regiones:    h.loadRegiones(ctx),
		CSRFToken:   h.GetCSRFToken(w, r),
	}
	fd.IsPartial = IsHTMXRequest(r)
	if fd.IsPartial {
		h.page.RenderPartial(w, UbicacionFormPage(fd, t))
	} else {
		h.page.Render(w, r, t("masters.location.edit"), "/locations", UbicacionFormPage(fd, t))
	}
}

func (h *UIHandler) UbicacionUpdate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := masterID(r.URL.Path, "/locations/", "")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.location.not_found"))
		return
	}
	if err := r.ParseForm(); err != nil {
		h.page.RenderError(w, http.StatusBadRequest, t("error.generic"), t("error.invalid_form_data"))
		return
	}
	entity := map[string]any{
		"short_name":  r.FormValue("short_name"),
		"description": r.FormValue("description"),
		"region_id":   toInt64FromString(r.FormValue("region_id")),
	}
	if errs := validateForm(entity, schemaUbicacionCreate); errs != nil {
		fd := UbicacionFormData{
			ID: id, IsEdit: true,
			ShortName: r.FormValue("short_name"), Description: r.FormValue("description"),
			RegionID: toInt64FromString(r.FormValue("region_id")),
			Regiones: h.loadRegiones(ctx),
			Error:    firstError(errs), Errors: errs, CSRFToken: h.GetCSRFToken(w, r),
		}
		if IsHTMXRequest(r) {
			fd.IsPartial = true
			h.page.RenderPartial(w, UbicacionFormPage(fd, t))
			return
		}
		h.page.Render(w, r, t("masters.location.edit"), "/locations", UbicacionFormPage(fd, t))
		return
	}
	entity = UbicacionValuesToXolu(entity)
	if _, err := h.xoluClient.Patch(ctx, "locations", id, entity); err != nil {
		fd := UbicacionFormData{
			ID: id, IsEdit: true,
			ShortName:   r.FormValue("short_name"),
			Description: r.FormValue("description"),
			RegionID:    toInt64FromString(r.FormValue("region_id")),
			Regiones:    h.loadRegiones(ctx),
			Error:       t("error.update_failed") + err.Error(),
			CSRFToken:   h.GetCSRFToken(w, r),
		}
		if IsHTMXRequest(r) {
			fd.IsPartial = true
			h.page.RenderPartial(w, UbicacionFormPage(fd, t))
			return
		}
		h.page.Render(w, r, t("masters.location.edit"), "/locations", UbicacionFormPage(fd, t))
		return
	}
	if IsHTMXRequest(r) {
		HTMXTrigger(w, "seamModal2Saved")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/locations", http.StatusSeeOther)
}

func (h *UIHandler) UbicacionDelete(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := masterID(r.URL.Path, "/locations/", "/delete")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.location.not_found"))
		return
	}
	if h.countRef(ctx, "assets", "location_id", id) > 0 {
		h.page.RenderError(w, http.StatusConflict, t("error.cannot_delete"), t("masters.location.in_use"))
		return
	}
	if err := h.xoluClient.Delete(ctx, "locations", id); err != nil {
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.delete_failed"))
		return
	}
	http.Redirect(w, r, "/locations", http.StatusSeeOther)
}

// loadRegiones returns all regiones for use in dropdowns.
func (h *UIHandler) loadRegiones(ctx context.Context) []RegionRow {
	result, err := h.xoluClient.OQL(ctx, "SELECT * FROM regions ORDER BY description")
	if err != nil {
		return nil
	}
	var regiones []RegionRow
	for _, row := range result.Data {
		regiones = append(regiones, RegionRow{
			ID:          toInt64(row["id"]),
			Description: toString(row["description"]),
		})
	}
	return regiones
}

// toInt64FromString parses a string to int64, returning 0 on failure.
func toInt64FromString(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

// ── DocumentLinksField form parsing (P-16) ────────────────────────────────────

// parseDocumentLinks assembles []DocumentLink from multi-value form arrays
// submitted by the DocumentLinksField component. prefix is e.g. "loan_links".
func parseDocumentLinks(r *http.Request, prefix string) []DocumentLink {
	nombres := r.Form[prefix+"_nombre[]"]
	urls := r.Form[prefix+"_url[]"]
	tipos := r.Form[prefix+"_tipo[]"]
	var links []DocumentLink
	for i, nombre := range nombres {
		url := ""
		if i < len(urls) {
			url = urls[i]
		}
		tipo := ""
		if i < len(tipos) {
			tipo = tipos[i]
		}
		// Skip entirely blank rows
		if nombre == "" && url == "" {
			continue
		}
		links = append(links, DocumentLink{
			Name: nombre,
			URL:  url,
			Type: tipo,
		})
	}
	return links
}

// ── PropAssociation (P-18) ────────────────────────────────────────────────────

// PropAssocRow is a row in the PropAssociations list.
type PropAssocRow struct {
	ID           int64
	PropDefID    int64
	ShortName    string
	DataType     string
	TargetEntity string
	EntityID     int64
	Requirement  string
}

// PropAssocsListData holds data for the PropAssociations list page.
type PropAssocsListData struct {
	Assocs    []PropAssocRow
	Total     int
	CSRFToken string
}

// PropAssocFormData holds data for the PropAssociation create/edit form.
type PropAssocFormData struct {
	ID           int64
	IsEdit       bool
	PropDefID    int64
	TargetEntity string
	EntityID     int64
	Requirement  string
	PropDefs     []PropDefRow // dropdown data
	Error        string
	Errors       map[string]string
	CSRFToken    string
}

func (h *UIHandler) PropAssocsList(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	data := PropAssocsListData{}
	result, err := h.xoluClient.OQL(ctx, "SELECT * FROM prop_associations ORDER BY target_entity, entity_id")
	if err == nil {
		// Build prop_def lookup
		defs := map[int64]PropDefRow{}
		if dr, e := h.xoluClient.OQL(ctx, "SELECT id, short_name, data_type FROM prop_definitions"); e == nil {
			for _, row := range dr.Data {
				id := toInt64(row["id"])
				defs[id] = PropDefRow{ID: id, ShortName: toString(row["short_name"]), DataType: PropDefEnumValueFromXoluValue("data_type", toString(row["data_type"]))}
			}
		}
		for _, row := range result.Data {
			pid := toInt64(row["prop_definition_id"])
			def := defs[pid]
			data.Assocs = append(data.Assocs, PropAssocRow{
				ID:           toInt64(row["id"]),
				PropDefID:    pid,
				ShortName:    def.ShortName,
				DataType:     def.DataType,
				TargetEntity: toString(row["target_entity"]),
				EntityID:     toInt64(row["entity_id"]),
				Requirement:  PropAssocEnumValueFromXoluValue("requirement", toString(row["requirement"])),
			})
		}
	}
	data.Total = len(data.Assocs)
	data.CSRFToken = h.GetCSRFToken(w, r)
	h.page.Render(w, r, t("masters.propassoc.list_title"), "/properties/associations", PropAssocsListPage(data, t))
}

func (h *UIHandler) PropAssocNew(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	data := PropAssocFormData{
		CSRFToken: h.GetCSRFToken(w, r),
		PropDefs:  h.loadPropDefs(ctx),
	}
	h.page.Render(w, r, t("masters.propassoc.new"), "/properties/associations", PropAssocFormPage(data, t))
}

func (h *UIHandler) PropAssocCreate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		h.page.RenderError(w, http.StatusBadRequest, t("error.generic"), t("error.invalid_form_data"))
		return
	}
	// English map keys for PropAssocCreateSchema (T-26). The form now
	// submits English values directly (T-18 Stage 9 tier-1), so the
	// default matches -- PropAssocEnumValueFromXoluValue below stays as
	// a tolerant pass-through for any caller still posting the legacy
	// Spanish value, not something this default needs to rely on.
	obligatoriedad := r.FormValue("obligatoriedad")
	if obligatoriedad == "" {
		obligatoriedad = "optional"
	}
	entity := map[string]any{
		"prop_definition_id": toInt64FromString(r.FormValue("prop_definition_id")),
		"target_entity":      r.FormValue("entidad_destino"),
		"entity_id":          toInt64FromString(r.FormValue("entidad_id")),
		"requirement":        PropAssocEnumValueFromXoluValue("requirement", obligatoriedad),
	}
	if errs := validateForm(entity, schemaPropAssocCreate); errs != nil {
		fd := PropAssocFormData{
			PropDefID:    toInt64FromString(r.FormValue("prop_definition_id")),
			TargetEntity: r.FormValue("entidad_destino"),
			EntityID:     toInt64FromString(r.FormValue("entidad_id")),
			Requirement:  obligatoriedad,
			PropDefs:     h.loadPropDefs(ctx),
			Error:        firstError(errs), Errors: errs, CSRFToken: h.GetCSRFToken(w, r),
		}
		h.page.Render(w, r, t("masters.propassoc.new"), "/properties/associations", PropAssocFormPage(fd, t))
		return
	}
	entity = PropAssocValuesToXolu(entity)
	if _, err := h.xoluClient.Create(ctx, "prop_associations", entity); err != nil {
		fd := PropAssocFormData{
			PropDefID:    toInt64FromString(r.FormValue("prop_definition_id")),
			TargetEntity: r.FormValue("entidad_destino"),
			EntityID:     toInt64FromString(r.FormValue("entidad_id")),
			Requirement:  obligatoriedad,
			PropDefs:     h.loadPropDefs(ctx),
			Error:        t("error.create_failed") + err.Error(),
			CSRFToken:    h.GetCSRFToken(w, r),
		}
		h.page.Render(w, r, t("masters.propassoc.new"), "/properties/associations", PropAssocFormPage(fd, t))
		return
	}
	http.Redirect(w, r, "/properties/associations", http.StatusSeeOther)
}

func (h *UIHandler) PropAssocDelete(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := masterID(r.URL.Path, "/properties/associations/", "/delete")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("masters.propassoc.not_found"))
		return
	}
	if err := h.xoluClient.Delete(ctx, "prop_associations", id); err != nil {
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.delete_failed"))
		return
	}
	http.Redirect(w, r, "/properties/associations", http.StatusSeeOther)
}

// loadPropDefs returns all prop_definitions for dropdowns.
func (h *UIHandler) loadPropDefs(ctx context.Context) []PropDefRow {
	result, err := h.xoluClient.OQL(ctx, "SELECT id, short_name, data_type FROM prop_definitions ORDER BY short_name")
	if err != nil {
		return nil
	}
	var defs []PropDefRow
	for _, row := range result.Data {
		defs = append(defs, PropDefRow{
			ID:        toInt64(row["id"]),
			ShortName: toString(row["short_name"]),
			DataType:  PropDefEnumValueFromXoluValue("data_type", toString(row["data_type"])),
		})
	}
	return defs
}

// loadAlertas returns all alertas for use in dropdowns.
func (h *UIHandler) loadAlertas(ctx context.Context) []AlertaRow {
	result, err := h.xoluClient.OQL(ctx, "SELECT * FROM alert_types ORDER BY type, criticality")
	if err != nil {
		return nil
	}
	var alertas []AlertaRow
	for _, row := range result.Data {
		alertas = append(alertas, AlertaRow{
			ID:          toInt64(row["id"]),
			Type:        AlertaEnumValueFromXoluValue("type", toString(row["type"])),
			Description: toString(row["description"]),
			Criticality: AlertaEnumValueFromXoluValue("criticality", toString(row["criticality"])),
		})
	}
	return alertas
}

// loadModelos returns all modelos for use in dropdowns.
func (h *UIHandler) loadModelos(ctx context.Context) []ModeloRow {
	result, err := h.xoluClient.OQL(ctx, "SELECT id, brand_id, name FROM models ORDER BY name")
	if err != nil {
		return nil
	}
	var rows []ModeloRow
	for _, row := range result.Data {
		rows = append(rows, ModeloRow{
			ID:      toInt64(row["id"]),
			BrandID: toInt64(row["brand_id"]),
			Name:    toString(row["name"]),
		})
	}
	return rows
}

// loadUbicaciones returns all ubicaciones for dropdowns.
func (h *UIHandler) loadUbicaciones(ctx context.Context) []UbicacionRow {
	result, err := h.xoluClient.OQL(ctx, "SELECT id, short_name, region_id FROM locations ORDER BY short_name")
	if err != nil {
		return nil
	}
	var rows []UbicacionRow
	for _, row := range result.Data {
		rows = append(rows, UbicacionRow{
			ID:        toInt64(row["id"]),
			ShortName: toString(row["short_name"]),
			RegionID:  toInt64(row["region_id"]),
		})
	}
	return rows
}

// loadCustodiosSimple returns custodios for dropdowns (active only).
func (h *UIHandler) loadCustodiosSimple(ctx context.Context) []CustodioRow {
	result, err := h.xoluClient.OQL(ctx, "SELECT id, legal_name, subject_type FROM custodians WHERE active = true ORDER BY legal_name")
	if err != nil {
		return nil
	}
	var rows []CustodioRow
	for _, row := range result.Data {
		rows = append(rows, CustodioRow{
			ID:          toInt64(row["id"]),
			LegalName:   toString(row["legal_name"]),
			SubjectType: toString(row["subject_type"]),
		})
	}
	return rows
}

// loadMonedasSimple returns monedas for dropdowns.
func (h *UIHandler) loadMonedasSimple(ctx context.Context) []MonedaRow {
	result, err := h.xoluClient.OQL(ctx, "SELECT id, iso_code, symbol FROM currencies ORDER BY iso_code")
	if err != nil {
		return nil
	}
	var rows []MonedaRow
	for _, row := range result.Data {
		rows = append(rows, MonedaRow{
			ID:      toInt64(row["id"]),
			IsoCode: toString(row["iso_code"]),
			Symbol:  toString(row["symbol"]),
		})
	}
	return rows
}

// loadCentrosCosto returns centros de costo for dropdowns.
func (h *UIHandler) loadCentrosCosto(ctx context.Context) []CentroCostoRow {
	result, err := h.xoluClient.OQL(ctx, "SELECT id, accounting_code, name FROM cost_centers ORDER BY name")
	if err != nil {
		return nil
	}
	var rows []CentroCostoRow
	for _, row := range result.Data {
		rows = append(rows, CentroCostoRow{
			ID:             toInt64(row["id"]),
			AccountingCode: toString(row["accounting_code"]),
			Name:           toString(row["name"]),
		})
	}
	return rows
}

// parseDocumentLinksToAny converts form document link arrays to []any for
// xolu storage. Dead code -- zero callers anywhere in this codebase,
// confirmed by grep before touching it -- but renamed for consistency
// with the rest of this block's own rename (T-40) rather than left with
// stale Spanish keys sitting next to everything else that changed.
func parseDocumentLinksToAny(r *http.Request, prefix string) []any {
	links := parseDocumentLinks(r, prefix)
	if len(links) == 0 {
		return nil
	}
	result := make([]any, len(links))
	for i, l := range links {
		result[i] = map[string]any{
			"name": l.Name,
			"url":  l.URL,
			"type": l.Type,
		}
	}
	return result
}

// toFloatFromString parses a string to float64, returning 0 on failure.
func toFloatFromString(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// toFloat64OrZero parses a string to float64, returning 0 on failure.
// Alias for toFloatFromString; used when queryfy expects a number field.
func toFloat64OrZero(s string) float64 { return toFloatFromString(s) }

// ── Property value rendering (P-19) ──────────────────────────────────────────

// PropValueEntry holds one resolved property with its current value.
type PropValueEntry struct {
	ID           int64
	ShortName    string
	DataType     string
	Requirement  string
	DefaultValue string
	CurrentValue string
}

// LoadPropValues loads prop_associations for a given entity and resolves
// current values from a prop_values map (keyed by short_name).
func (h *UIHandler) LoadPropValues(ctx context.Context, entidadDestino string, entidadID int64, propValues map[string]interface{}) []PropValueEntry {
	// Load associations for this entity type+ID
	result, err := h.xoluClient.OQL(ctx, fmt.Sprintf(
		"SELECT * FROM prop_associations WHERE target_entity = '%s' AND entity_id = %d ORDER BY orden",
		entidadDestino, entidadID,
	))
	if err != nil || result == nil {
		return nil
	}

	// Build prop_def lookup
	defs := map[int64]PropDefRow{}
	if dr, e := h.xoluClient.OQL(ctx, "SELECT id, short_name, data_type, default_value FROM prop_definitions"); e == nil {
		for _, row := range dr.Data {
			id := toInt64(row["id"])
			defs[id] = PropDefRow{
				ID:        id,
				ShortName: toString(row["short_name"]),
				DataType:  PropDefEnumValueFromXoluValue("data_type", toString(row["data_type"])),
			}
		}
	}

	var entries []PropValueEntry
	for _, row := range result.Data {
		pid := toInt64(row["prop_definition_id"])
		def := defs[pid]
		currentVal := ""
		if propValues != nil {
			if v, ok := propValues[def.ShortName]; ok {
				currentVal = fmt.Sprintf("%v", v)
			}
		}
		if currentVal == "" {
			currentVal = toString(row["default_value"])
		}
		entries = append(entries, PropValueEntry{
			ID:           pid,
			ShortName:    def.ShortName,
			DataType:     def.DataType,
			Requirement:  PropAssocEnumValueFromXoluValue("requirement", toString(row["requirement"])),
			CurrentValue: currentVal,
		})
	}
	return entries
}
