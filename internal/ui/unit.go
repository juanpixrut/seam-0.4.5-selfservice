package ui

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	mi "github.com/ha1tch/minty"
	fe "github.com/ha1tch/seam-ui/internal/formengine"
	unitschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/unit"
)

type Unit struct {
	ID               int64
	Name             string
	Symbol           string
	IsoCode          string
	AddReferenceUnit bool
	ReferenceUnit    int64
	ConversionFactor float64
}

type UnitListData struct {
	Units     []Unit
	CSRFToken string
}

type UnitFormData struct {
	ID          int64
	IsEdit      bool
	IsPartial   bool
	CSRFToken   string
	FormContent mi.H
	Error       string
}

func unitFromData(id int64, data map[string]any) Unit {
	return Unit{
		ID:               id,
		Name:             toString(data["name"]),
		Symbol:           toString(data["symbol"]),
		IsoCode:          toString(data["iso_code"]),
		AddReferenceUnit: toInt64(data["reference_unit"]) > 0,
		ReferenceUnit:    toInt64(data["reference_unit"]),
		ConversionFactor: toFloat(data["conversion_factor"]),
	}
}

func (h *UIHandler) UnitList(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	result, err := h.xoluClient.OQL(r.Context(), "SELECT * FROM unit ORDER BY name")
	if err != nil {
		h.logger.Error("failed to list consumables", "error", err)
		result = nil
	}

	data := UnitListData{CSRFToken: h.GetCSRFToken(w, r)}
	if result != nil {
		for _, row := range result.Data {
			data.Units = append(data.Units, unitFromData(toInt64(row["id"]), row))
		}
	}
	h.page.Render(w, r, t("unit.list_title"), "/unit", UnitListPage(data, t))
}

func (h *UIHandler) UnitNew(w http.ResponseWriter, r *http.Request) {
	h.renderUnitForm(w, r, 0, nil, nil, "")
}

func (h *UIHandler) UnitCreate(w http.ResponseWriter, r *http.Request) {
	h.saveUnit(w, r, 0)
}

func (h *UIHandler) UnitEdit(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	id := masterID(r.URL.Path, "/unit/", "/edit")
	if id <= 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("unit.not_found"))
		return
	}
	entity, err := h.xoluClient.Get(r.Context(), "unit", id)
	if err != nil {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("unit.not_found"))
		return
	}
	unit := unitFromData(id, entity.Data)
	values := map[string]any{
		"name":               unit.Name,
		"symbol":             unit.Symbol,
		"iso_code":           unit.IsoCode,
		"add_reference_unit": unit.AddReferenceUnit,
		"reference_unit":     unit.ReferenceUnit,
		"conversion_factor":  strconv.FormatFloat(unit.ConversionFactor, 'f', -1, 64),
	}
	h.renderUnitForm(w, r, id, values, nil, "")
}

func (h *UIHandler) UnitUpdate(w http.ResponseWriter, r *http.Request) {
	id := masterID(r.URL.Path, "/unit/", "")
	if id <= 0 {
		t := h.translateFunc(r)
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("unit.not_found"))
		return
	}
	h.saveUnit(w, r, id)
}

var unitFactorPattern = regexp.MustCompile(`^[0-9]{1,9}(\.[0-9]{1,9})?$`)

func (h *UIHandler) saveUnit(w http.ResponseWriter, r *http.Request, id int64) {
	t := h.translateFunc(r)
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		h.page.RenderError(w, http.StatusBadRequest, t("error.generic"), t("error.invalid_form_data"))
		return
	}
	if id > 0 {
		if _, err := h.xoluClient.Get(ctx, "unit", id); err != nil {
			h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("unit.not_found"))
			return
		}
	}

	name := strings.TrimSpace(r.PostForm.Get("name"))
	symbol := strings.TrimSpace(r.PostForm.Get("symbol"))
	isoCode := strings.TrimSpace(r.PostForm.Get("iso_code"))
	checkbox := r.PostForm.Get("add_reference_unit")
	addReference := checkbox == "on" || checkbox == "true" || checkbox == "1"
	referenceText := strings.TrimSpace(r.FormValue("reference_unit"))
	factorText := strings.TrimSpace(r.PostForm.Get("conversion_factor"))
	values := map[string]any{
		"name": name, "symbol": symbol, "iso_code": isoCode,
		"reference_unit":    referenceText,
		"conversion_factor": factorText,
	}
	fieldErrors := make(map[string]string)
	if name == "" || utf8.RuneCountInString(name) > 50 {
		fieldErrors["name"] = t("unit.error.invalid_name")
	}
	if symbol == "" {
		fieldErrors["symbol"] = t("unit.error.symbol_required")
	}
	if isoCode == "" || utf8.RuneCountInString(isoCode) > 3 {
		fieldErrors["iso_code"] = t("unit.error.invalid_iso_code")
	}

	var referenceID int64
	var conversionFactor float64
	if addReference {
		var err error
		referenceID, err = strconv.ParseInt(referenceText, 10, 64)
		if err != nil || referenceID <= 0 {
			fieldErrors["reference_unit"] = t("unit.error.invalid_reference")
		} else if err = h.validateUnitReference(ctx, id, referenceID); err != nil {
			fieldErrors["reference_unit"] = t(err.Error())
		}
		conversionFactor = toFloat64OrZero(factorText)
		if !unitFactorPattern.MatchString(factorText) || conversionFactor <= 0 {
			fieldErrors["conversion_factor"] = t("unit.error.invalid_factor")
		}
	}

	if len(fieldErrors) > 0 {
		h.renderUnitForm(w, r, id, values, fieldErrors, "")
		return
	}

	data := map[string]any{
		"name":              name,
		"symbol":            symbol,
		"iso_code":          isoCode,
		"reference_unit":    nil,
		"conversion_factor": nil,
	}
	if addReference {
		data["reference_unit"] = referenceID
		data["conversion_factor"] = conversionFactor
	}
	var err error
	if id == 0 {
		_, err = h.xoluClient.Create(ctx, "unit", data)
	} else {
		_, err = h.xoluClient.Patch(ctx, "unit", id, data)
	}
	if err != nil {
		key := "error.create_failed"
		if id > 0 {
			key = "error.update_failed"
		}
		h.renderUnitForm(w, r, id, values, nil, t(key))
		return
	}
	http.Redirect(w, r, "/unit", http.StatusSeeOther)
}

func (h *UIHandler) validateUnitReference(ctx context.Context, id, referenceID int64) error {
	visited := make(map[int64]bool)
	if id > 0 {
		visited[id] = true
	}
	for referenceID > 0 {
		if visited[referenceID] {
			return fmt.Errorf("unit.error.circular_reference")
		}
		visited[referenceID] = true
		entity, err := h.xoluClient.Get(ctx, "unit", referenceID)
		if err != nil {
			return fmt.Errorf("unit.error.invalid_reference")
		}
		if !toBool(entity.Data["add_reference_unit"]) {
			return nil
		}
		referenceID = toInt64(entity.Data["reference_unit"])
		if referenceID <= 0 {
			return fmt.Errorf("unit.error.invalid_reference")
		}
	}
	return nil
}

func (h *UIHandler) UnitDelete(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := masterID(r.URL.Path, "/unit/", "/delete")
	if id <= 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("unit.not_found"))
		return
	}
	if _, err := h.xoluClient.Get(ctx, "unit", id); err != nil {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("unit.not_found"))
		return
	}
	result, err := h.xoluClient.OQL(ctx, fmt.Sprintf("SELECT id FROM unit WHERE reference_unit = %d", id))
	if err != nil {
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("unit.error.load_failed"))
		return
	}
	if result != nil && len(result.Data) > 0 {
		h.page.RenderError(w, http.StatusConflict, t("error.cannot_delete"), t("unit.in_use"))
		return
	}
	if err := h.xoluClient.Delete(ctx, "unit", id); err != nil {
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.delete_failed"))
		return
	}
	http.Redirect(w, r, "/unit", http.StatusSeeOther)
}

func (h *UIHandler) renderUnitForm(w http.ResponseWriter, r *http.Request, id int64, values map[string]any, fieldErrors map[string]string, message string) {
	t := h.translateFunc(r)
	csrf := h.GetCSRFToken(w, r)
	engine := &fe.FormEngine{Translator: h.formEngineTranslator()}
	rc := fe.RenderContext{
		Locale: h.currentLocale(), CSRFToken: csrf, Module: "unit", Form: "unit",
	}
	data := UnitFormData{
		ID: id, IsEdit: id > 0, IsPartial: IsHTMXRequest(r), CSRFToken: csrf, Error: message,
		FormContent: engine.Render(r.Context(), resolveSchema(r.Context(), "unit", "create", unitschemas.Schema("create")), values, fieldErrors, rc),
	}
	if data.IsPartial {
		h.page.RenderPartial(w, UnitFormPage(data, t))
		return
	}
	title := "unit.new"
	if data.IsEdit {
		title = "unit.edit"
	}
	h.page.Render(w, r, t(title), "/unit", UnitFormPage(data, t))
}

func UnitListPage(data UnitListData, t func(string, ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}

	rows := make([]mi.H, len(data.Units))
	names := make(map[int64]string, len(data.Units))
	for _, unit := range data.Units {
		names[unit.ID] = unit.Name
	}
	for i, unit := range data.Units {
		unit := unit
		equivalence := "—"
		if unit.ReferenceUnit > 0 {
			referenceName := names[unit.ReferenceUnit]
			if referenceName == "" {
				referenceName = fmt.Sprintf("#%d", unit.ReferenceUnit)
			}
			equivalence = fmt.Sprintf("1 %s = %s %s", unit.Symbol, strconv.FormatFloat(unit.ConversionFactor, 'f', -1, 64), referenceName)
		}
		rows[i] = ListRow(i,
			ListCellLink(unit.Name, fmt.Sprintf("/unit/%d", unit.ID)),
			ListCellMuted(unit.Symbol), ListCellMuted(unit.IsoCode),
			ListCellMuted(equivalence),
			ListCellRich(func(b *mi.Builder) mi.Node {
				return b.Form(mi.Method("post"), mi.Action(fmt.Sprintf("/unit/%d/delete", unit.ID)),
					b.Input(mi.Type("hidden"), mi.Name("csrf_token"), mi.Value(data.CSRFToken)),
					b.Button(mi.Type("submit"), mi.Class("text-red-600 hover:underline"), t("action.delete")),
				)
			}),
		)
	}
	return ListPage(ListPageConfig{
		Title: t("unit.list_title"), CreateURL: "/unit/new", CreateLabel: t("unit.new"),
	}, ListTable(ListTableConfig{
		Columns: []ListColumn{
			{Label: t("unit.field.name")}, {Label: t("unit.field.symbol")},
			{Label: t("unit.field.iso_code")},
			{Label: t("unit.field.equivalence")}, {Label: t("unit.field.actions")},
		},
		EmptyIcon: "straighten", EmptyTitle: t("unit.list_title"), EmptyMessage: t("unit.empty"),
	}, rows), nil, nil)
}

func UnitFormPage(data UnitFormData, t func(string, ...any) string) mi.H {
	title, action := "unit.new", "/unit"
	if data.IsEdit {
		title, action = "unit.edit", fmt.Sprintf("/unit/%d", data.ID)
	}
	return func(b *mi.Builder) mi.Node {
		content := Card("", func(b *mi.Builder) mi.Node {
			return b.Form(mi.Method("post"), mi.Action(action), mi.Class("space-y-4"),
				b.Input(mi.Type("hidden"), mi.Name("csrf_token"), mi.Value(data.CSRFToken)),
				b.Div(mi.Class("text-red-600"), mi.Attr("role", "alert"), data.Error),
				data.FormContent(b),
				b.Div(mi.Class("flex justify-end gap-3 pt-4"),
					b.A(mi.Href("/unit"), mi.Class("px-4 py-2 border rounded-lg"), t("action.cancel")),
					b.Button(mi.Type("submit"), mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg"), t("action.save")),
				),
			)
		})(b)
		return b.Div(
			mi.Class("space-y-6 max-w-2xl"),
			b.H1(
				mi.Class("text-2xl font-bold text-gray-900 dark:text-white"),
				t(title),
			),
			content,
		)
	}
}

type UnitDetailData struct {
	Unit            Unit
	ReferenceName   string
	ReferenceSymbol string
}

func (h *UIHandler) UnitDetail(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := masterID(r.URL.Path, "/unit/", "")
	if id <= 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("unit.not_found"))
		return
	}

	entity, err := h.xoluClient.Get(ctx, "unit", id)
	if err != nil {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("unit.not_found"))
		return
	}

	data := UnitDetailData{Unit: unitFromData(id, entity.Data)}
	if data.Unit.AddReferenceUnit && data.Unit.ReferenceUnit > 0 {
		reference, err := h.xoluClient.Get(ctx, "unit", data.Unit.ReferenceUnit)
		if err != nil {
			h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("unit.error.load_failed"))
			return
		}
		data.ReferenceName = toString(reference.Data["name"])
		data.ReferenceSymbol = toString(reference.Data["symbol"])
	}

	h.page.Render(w, r, t("unit.detail_title"), "/unit", UnitDetailPage(data, t))
}

func UnitDetailPage(data UnitDetailData, t func(string, ...any) string) mi.H {
	unit := data.Unit
	return func(b *mi.Builder) mi.Node {
		equivalence := "—"
		if unit.AddReferenceUnit && unit.ReferenceUnit > 0 {
			referenceLabel := data.ReferenceSymbol
			if referenceLabel == "" {
				referenceLabel = data.ReferenceName
			}
			equivalence = fmt.Sprintf("1 %s = %s %s", unit.Symbol, strconv.FormatFloat(unit.ConversionFactor, 'f', -1, 64), referenceLabel)
		}

		return b.Div(mi.Class("space-y-6 max-w-2xl"),
			b.Nav(mi.Class("text-sm text-gray-500 dark:text-gray-400"),
				b.A(mi.Href("/unit"), mi.Class("hover:underline"), t("unit.list_title")),
				b.Span(mi.Class("mx-2"), "/"),
				b.Span(unit.Name),
			),
			b.Div(mi.Class("flex items-center justify-between gap-4"),
				b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), unit.Name),
				b.A(mi.Href(fmt.Sprintf("/unit/%d/edit", unit.ID)),
					mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg"), t("action.edit")),
			),
			Card(t("unit.detail_title"), func(b *mi.Builder) mi.Node {
				return b.Div(mi.Class("grid grid-cols-1 sm:grid-cols-2 gap-4"),
					infoRow(b, t("unit.field.name"), unit.Name),
					infoRow(b, t("unit.field.symbol"), unit.Symbol),
					infoRow(b, t("unit.field.iso_code"), unit.IsoCode),
					infoRow(b, t("unit.field.equivalence"), equivalence),
					func() mi.Node {
						if unit.ReferenceUnit <= 0 {
							return b.Div()
						}
						return b.Div(mi.Class("sm:col-span-2"),
							b.A(mi.Href(fmt.Sprintf("/unit/%d", unit.ReferenceUnit)),
								mi.Class("text-indigo-600 dark:text-indigo-400 hover:underline"),
								data.ReferenceName),
						)
					}(),
				)
			})(b),
		)
	}
}
