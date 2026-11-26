package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	mi "github.com/ha1tch/minty"
	"github.com/ha1tch/seam-ui/internal/auth"
	fe "github.com/ha1tch/seam-ui/internal/formengine"
	warehouseschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/warehouse"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

// inventoryMutationMu serializes inventory writes and warehouse state changes
// within this Seam process.
var inventoryMutationMu sync.Mutex

type Warehouse struct {
	ID            int64
	Name          string
	Description   string
	Location      string
	ResponsibleId int64
	Enabled       bool
}

func (h *UIHandler) WarehouseList(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()

	result, err := h.xoluClient.OQL(
		ctx,
		"SELECT TOP 100 * FROM warehouse ORDER BY id DESC",
	)
	if err != nil {
		h.logger.Error("failed to list warehouses", "error", err)
		result = nil
	}

	var warehouses []WarehouseListItem

	// Guardamos los IDs únicos para consultar sus nombres en conjunto.
	userIDs := make(map[int64]bool)

	if result != nil {
		for _, row := range result.Data {
			responsibleID := toInt64(row["responsible_user_id"])

			warehouses = append(warehouses, WarehouseListItem{
				ID:                  toInt64(row["id"]),
				Name:                toString(row["name"]),
				Enabled:             toBool(row["enabled"]),
				ResponsibleUserName: "",
			})

			if responsibleID > 0 {
				userIDs[responsibleID] = true
			}

			// Guardamos los IDs temporalmente en los campos
			// del almacén, para asociarlos después con sus nombres.
			warehouses[len(warehouses)-1].ResponsibleID = responsibleID
		}
	}

	// Consultar los nombres de los usuarios responsables.
	usersMap := make(map[int64]string)

	if len(userIDs) > 0 {
		var ids []string
		for id := range userIDs {
			ids = append(ids, strconv.FormatInt(id, 10))
		}

		query := fmt.Sprintf(
			"SELECT id, name FROM users WHERE id IN (%s)",
			strings.Join(ids, ","),
		)

		userResult, err := h.xoluClient.OQL(ctx, query)
		if err != nil {
			h.logger.Error("failed to fetch warehouse responsible users", "error", err)
		} else {
			for _, row := range userResult.Data {
				usersMap[toInt64(row["id"])] = toString(row["name"])
			}
		}
	}

	// Completar los nombres que se mostrarán en la tabla.
	for i := range warehouses {
		warehouses[i].ResponsibleUserName = usersMap[warehouses[i].ResponsibleID]
	}

	data := WarehousesListData{Warehouses: warehouses}
	h.page.Render(w, r, t("warehouses.list_title"), "/warehouses", WarehousesListPage(data, t))

}

func (h *UIHandler) WarehouseNew(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	csrfToken := h.GetCSRFToken(w, r)

	formEng := &fe.FormEngine{
		Translator: h.formEngineTranslator(),
	}
	rc := fe.RenderContext{Locale: h.currentLocale(), CSRFToken: csrfToken, Module: "warehouse", Form: "warehouse"}
	formContent := formEng.Render(ctx, resolveSchema(ctx, "warehouse", "create", warehouseschemas.Schema("create")), nil, nil, rc)

	data := WarehouseFormData{CSRFToken: csrfToken, FormContent: formContent}
	h.page.Render(w, r, t("warehouses.new"), "/warehouses/new", WarehouseFormPage(data, t))
}

func (h *UIHandler) WarehouseCreate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, h.errText(r, "error.invalid_form_data", "Invalid form data"), http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	description := r.FormValue("description")
	location := r.FormValue("location")
	responsible := r.FormValue("responsible_user_id")
	enabled := r.FormValue("enabled") == "on"
	csrfToken := h.GetCSRFToken(w, r)

	var responsibleID *int64
	if responsible != "" {
		if id, err := strconv.ParseInt(responsible, 10, 64); err == nil && id > 0 {
			responsibleID = &id
		}
	}

	errors := map[string]string{}
	if name == "" {
		errors["name"] = t("warehouse.error.name_required")
	}

	if responsibleID == nil {
		errors["responsible_user_id"] = t("warehouse.error.responsible_required")
	}

	if location == "" {
		errors["location"] = t("warehouse.error.location_required")
	}

	if len(errors) > 0 {
		formEng := &fe.FormEngine{
			Translator: h.formEngineTranslator(),
		}
		rc := fe.RenderContext{Locale: h.currentLocale(), CSRFToken: csrfToken, Module: "warehouse", Form: "warehouse"}

		formContent := formEng.Render(ctx, resolveSchema(ctx, "warehouse", "create", warehouseschemas.Schema("create")),
			map[string]interface{}{
				"name":                name,
				"description":         description,
				"location":            location,
				"responsible_user_id": responsible,
				"enabled":             enabled,
			}, errors, rc)

		data := WarehouseFormData{CSRFToken: csrfToken, FormContent: formContent, Errors: errors}
		h.page.Render(w, r, t("warehouses.new"), "/warehouses", WarehouseFormPage(data, t))
		return
	}

	warehouseData := map[string]any{
		"name":                name,
		"description":         description,
		"location":            location,
		"responsible_user_id": responsibleID,
		"enabled":             enabled,
		"consumables":         []WarehouseConsumableBalance{},
	}

	entity, err := h.xoluClient.Create(ctx, "warehouse", warehouseData)
	if err != nil {
		h.logger.Error("failed to create warehouse", "error", err)
		http.Error(w, h.errText(r, "error.save_failed", "Failed to save warehouse"), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/warehouse/"+strconv.FormatInt(entity.ID, 10), http.StatusSeeOther)
}

func (h *UIHandler) WarehouseDetail(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("error.not_found"))
		return
	}

	entity, err := h.movementGetFresh(ctx, "warehouse", id)
	if err != nil {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("error.not_found"))
		return
	}

	warehouse := Warehouse{
		ID:            entity.ID,
		Name:          toString(entity.Data["name"]),
		Description:   toString(entity.Data["description"]),
		Location:      toString(entity.Data["location"]),
		ResponsibleId: toInt64(entity.Data["responsible_user_id"]),
		Enabled:       toBool(entity.Data["enabled"]),
	}

	responsibleName := ""
	responsibleEmail := ""
	if warehouse.ResponsibleId > 0 {
		result, err := h.xoluClient.OQL(ctx, fmt.Sprintf(
			"SELECT id, name, email FROM users WHERE id = %d",
			warehouse.ResponsibleId,
		))
		if err != nil {
			h.logger.Error("failed to fetch warehouse responsible", "error", err)
		} else if len(result.Data) > 0 {
			responsibleName = toString(result.Data[0]["name"])
			responsibleEmail = toString(result.Data[0]["email"])
		}
	}

	consumables, err := h.inventoryForWarehouse(ctx, entity)
	if err != nil {
		h.logger.Error("failed to load warehouse consumable stocks", "warehouse_id", warehouse.ID, "error", err)
		http.Error(w, t("warehouse.consumables.load_failed"), http.StatusInternalServerError)
		return
	}
	data := WarehouseDetailData{Warehouse: warehouse, ResponsibleName: responsibleName, ResponsibleEmail: responsibleEmail, Consumables: consumables, CSRFToken: h.GetCSRFToken(w, r)}
	h.page.Render(w, r, t("warehouse.detail_title"), "/warehouses", WarehouseDetailPage(data, t))
}

type WarehouseListItem struct {
	ID                  int64
	Name                string
	ResponsibleID       int64
	ResponsibleUserName string
	Enabled             bool
}

type WarehousesListData struct {
	Warehouses []WarehouseListItem
}

func WarehousesListPage(data WarehousesListData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}

	rows := make([]mi.H, len(data.Warehouses))

	for i, wh := range data.Warehouses {
		enabledLabel := t("warehouse.inactive")
		var variant = "default"

		if wh.Enabled {
			enabledLabel = t("warehouse.enabled")
			variant = "success"
		}

		rows[i] = ListRow(i,
			// Nombre del almacén
			ListCellLink(
				wh.Name,
				fmt.Sprintf("/warehouse/%d", wh.ID),
			),

			// Responsable
			ListCellMuted(wh.ResponsibleUserName),

			// Habilitado
			ListCellRich(func(b *mi.Builder) mi.Node {
				return Badge(enabledLabel, variant)(b)
			}),

			// Acciones
			ListActions(ListActionsConfig{
				EditURL:        fmt.Sprintf("/warehouses/%d/edit", wh.ID),
				EditHTMX:       true,
				EditModalTitle: t("warehouse.edit"),
				T:              t,
			}),
		)
	}

	return ListPage(
		ListPageConfig{
			Title:       t("warehouses.list_title"),
			CreateURL:   "/warehouses/new",
			CreateLabel: t("warehouses.new"),
		},
		ListTable(ListTableConfig{
			Columns: []ListColumn{
				{Label: t("field.name")},
				{Label: t("warehouse.forms.warehouse.field.responsible_user_id")},
				{Label: t("warehouse.forms.warehouse.field.enabled")},
				{Label: t("asset.list.actions"), Right: true},
			},
			EmptyIcon:    "warehouse",
			EmptyTitle:   t("warehouses.list_title"),
			EmptyMessage: t("warehouse.empty"),
		}, rows),
		nil,
		nil)
}

type WarehouseFormData struct {
	ID          int64
	IsPartial   bool
	CSRFToken   string
	FormContent mi.H
	Errors      map[string]string
}

func WarehouseFormPage(data WarehouseFormData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}

	title := t("warehouses.new")
	submitLabel := t("warehouse.action.create")
	actionURL := "/warehouses"
	cancelURL := "/warehouses"

	if data.ID > 0 {
		title = t("warehouse.edit")
		submitLabel = t("action.save")
		actionURL = fmt.Sprintf("/warehouses/%d", data.ID)
		cancelURL = fmt.Sprintf("/warehouse/%d", data.ID)
	}

	formID := "warehouse-form"

	return func(b *mi.Builder) mi.Node {
		formChildren := []interface{}{
			mi.ID(formID),
			mi.Class("space-y-4"),
			mi.Method("post"),
			mi.Action(actionURL),
		}

		if data.IsPartial {
			formChildren = append(
				formChildren,
				mi.Attr("hx-post", actionURL),
				mi.Attr("hx-target", "#seam-modal-body"),
				mi.Attr("hx-swap", "innerHTML settle:0ms"),
			)
		}

		if message := data.Errors["_"]; message != "" {
			formChildren = append(
				formChildren,
				b.P(mi.Attr("role", "alert"), mi.Class("text-sm text-red-600 dark:text-red-400"), message),
			)
		}

		formChildren = append(formChildren, data.FormContent(b))
		form := b.Form(formChildren...)

		actions := func(b *mi.Builder) mi.Node {
			cancelClass := "px-4 py-2 border border-gray-300 " +
				"dark:border-gray-600 text-gray-700 dark:text-gray-300 " +
				"rounded-lg font-medium"

			cancel := b.A(mi.Href(cancelURL), mi.Class(cancelClass), t("action.cancel"))

			if data.IsPartial {
				cancel = b.Button(mi.Type("button"), mi.Class(cancelClass), mi.Attr("onclick", "SeamModal.close()"), t("action.cancel"))
			}

			return b.Div(
				mi.Class("flex justify-end gap-3 pt-4 w-full"),
				cancel,
				b.Button(
					mi.Type("submit"),
					mi.Attr("form", formID),
					mi.Class(
						"px-4 py-2 bg-indigo-600 text-white rounded-lg "+
							"hover:bg-indigo-700 font-medium",
					),
					submitLabel,
				),
			)
		}

		if data.IsPartial {
			return b.Div(form, b.Div(mi.Attr("data-modal-footer", ""), actions(b)))
		}

		return b.Div(
			mi.Class("space-y-6 max-w-2xl"),
			b.Div(
				mi.Class("flex items-center gap-4"),
				b.A(
					mi.Href(cancelURL),
					mi.Class(
						"text-gray-500 hover:text-gray-700 "+
							"dark:text-gray-400 dark:hover:text-gray-200",
					),
					b.Span(mi.Class("material-icons"), "arrow_back"),
				),
				b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), title),
			),
			Card("", func(b *mi.Builder) mi.Node {
				return b.Div(form, actions(b))
			})(b),
		)
	}
}

type WarehouseConsumableBalance struct {
	ConsumableID int64   `json:"consumable_id"`
	Stock        float64 `json:"stock"`
}

type WarehouseConsumableStock struct {
	ConsumableID int64
	Name         string
	Stock        float64
	Unit         string
}

type WarehouseDetailData struct {
	Warehouse        Warehouse
	ResponsibleName  string
	ResponsibleEmail string
	Location         string
	Consumables      []WarehouseConsumableStock
	CSRFToken        string
}

func WarehouseDetailPage(data WarehouseDetailData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	wh := data.Warehouse
	enabledLabel := t("warehouse.inactive")
	var variant = "default"

	statusLabel := t("warehouse.enable")
	statusMessage := t("warehouse.confirm_enable")
	statusVariant := "success"

	if wh.Enabled {
		enabledLabel = t("warehouse.enabled")
		variant = "success"
		statusLabel = t("warehouse.disable")
		statusMessage = t("warehouse.confirm_disable")
		statusVariant = "danger"
	}

	return func(b *mi.Builder) mi.Node {

		return b.Div(mi.Class("space-y-6"),
			b.Div(mi.Class("flex items-center justify-between"),
				b.Div(
					// Breadcrumb
					b.Nav(mi.Class("text-sm text-gray-500 dark:text-gray-400 mb-2"),
						b.A(mi.Href("/warehouses"), mi.Class("hover:text-gray-700 dark:hover:text-gray-300"), t("nav.module.warehouses")),
						b.Span(mi.Class("mx-2"), "/"),
						b.Span(mi.Class("text-gray-900 dark:text-white"), wh.Name),
					),
					// Title with status
					b.Div(mi.Class("flex items-center gap-3"),
						b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"),
							wh.Name,
						),
						Badge(enabledLabel, variant)(b),
					),
				),
				// Actions
				b.Div(mi.Class("flex items-center gap-2"),
					b.Button(
						mi.Class("inline-flex items-center gap-1 px-3 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700"),
						mi.Attr("hx-get", fmt.Sprintf("/warehouses/%d/edit", wh.ID)),
						mi.Attr("hx-target", "#seam-modal-body"),
						mi.Attr("hx-swap", "innerHTML"),
						mi.Attr("onclick", fmt.Sprintf("SeamModal.open(%q)", t("warehouse.edit"))),
						b.Span(mi.Class("material-icons text-base leading-none"), "edit"),
						t("action.edit"),
					),
					Button(
						statusLabel,
						statusVariant,
						mi.Type("button"),
						mi.Attr("onclick", fmt.Sprintf(`
							SeamConfirm.show(%q, function () {
								htmx.ajax('POST', '/warehouses/%d/enabled', {
									target: '#warehouse-status-feedback',
									swap: 'innerHTML',
									values: {
										enabled: %q,
										_csrf: %q
									}
								});
							}, {
								title: %q,
								okLabel: %q,
								danger: %t
							});
						`,
							statusMessage,
							wh.ID,
							strconv.FormatBool(!wh.Enabled),
							data.CSRFToken,
							statusLabel,
							statusLabel,
							wh.Enabled,
						)),
					)(b),
				),
			),
			b.Div(mi.ID("warehouse-status-feedback")),

			b.Div(mi.Class("grid grid-cols-1 lg:grid-cols-3 gap-6"),

				// Información principal: 2/3 del ancho
				b.Div(mi.Class("lg:col-span-2 space-y-6"),
					Card(
						t("warehouse.info_title"),
						warehouseInfoContent(wh, t),
					)(b),
					Card(
						t("warehouse.location_title"),
						warehouseLocationContent(wh, t),
					)(b),
				),

				// Panel lateral: 1/3 del ancho
				b.Div(mi.Class("space-y-6"),
					Card(
						t("warehouse.responsible_title"),
						warehouseResponsibleContent(data, t),
					)(b),

					CardWithActions(
						t("warehouse.consumables.title"),

						func(b *mi.Builder) mi.Node {
							if !wh.Enabled {
								return b.Button(
									mi.Type("button"),
									mi.Attr("disabled", "disabled"),
									mi.Attr(
										"title",
										t("movement.error.warehouse_disabled"),
									),
									mi.Class(
										"px-3 py-2 text-sm font-medium rounded-lg "+
											"bg-indigo-600 text-white opacity-50 "+
											"cursor-not-allowed",
									),
									t("movement.action.open"),
								)
							}
							return b.Button(
								mi.Type("button"),
								mi.Class(
									"px-3 py-2 text-sm font-medium rounded-lg "+
										"bg-indigo-600 text-white hover:bg-indigo-700",
								),
								mi.Attr(
									"hx-get",
									fmt.Sprintf("/warehouses/%d/movements/new", wh.ID),
								),
								mi.Attr("hx-target", "#seam-modal-body"),
								mi.Attr("hx-swap", "innerHTML"),
								mi.Attr(
									"onclick",
									fmt.Sprintf("SeamModal.open(%q)", t("movement.new")),
								),
								t("movement.action.open"),
							)
						},

						warehouseConsumablesContent(data.Consumables, t),
					)(b),
				),
			),
		)
	}
}
func warehouseInfoContent(wh Warehouse, t func(key string, args ...any) string) mi.H {
	return func(b *mi.Builder) mi.Node {
		description := wh.Description
		if description == "" {
			description = t("warehouse.no_description")
		}

		return b.Dl(mi.Class("grid grid-cols-1 sm:grid-cols-2 gap-4"),
			infoRow(b, t("field.name"), wh.Name),
			infoRow(b, t("warehouse.enabled"), tEnabled(wh.Enabled, t)),

			b.Div(mi.Class("sm:col-span-2"),
				infoRow(b, t("field.description"), description),
			),
		)
	}
}

func tEnabled(enabled bool, t func(key string, args ...any) string) string {
	if enabled {
		return t("warehouse.enabled")
	}
	return t("warehouse.inactive")
}

func warehouseResponsibleContent(data WarehouseDetailData, t func(key string, args ...any) string) mi.H {
	return func(b *mi.Builder) mi.Node {
		name := data.ResponsibleName
		if name == "" {
			name = t("warehouse.unknown_responsible")
		}

		return b.Div(mi.Class("space-y-3"),
			b.Div(mi.Class("flex items-start gap-3"),
				b.Div(mi.Class("w-9 h-9 shrink-0 rounded-full bg-indigo-100 dark:bg-indigo-900 flex items-center justify-center"),
					b.Span(mi.Class("material-icons text-indigo-600 dark:text-indigo-300"), "person"),
				),
				b.Div(mi.Class("min-w-0 flex-1 pt-1"),
					b.P(mi.Class("text-sm font-medium text-gray-900 dark:text-white break-words"), name),
					b.P(mi.Class("mt-1 text-xs text-gray-500 dark:text-gray-400"), t("warehouse.responsible_caption")),
				),
			),
			b.Dl(mi.Class("border-t border-gray-200 dark:border-gray-700 pt-3"),
				infoRow(b, t("field.email"), data.ResponsibleEmail),
			),
		)
	}
}

func parseWarehouseGPS(location string) (float64, float64, bool) {
	parts := strings.Split(location, ",")
	if len(parts) != 2 {
		return 0, 0, false
	}

	lat, latErr := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	lng, lngErr := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)

	if latErr != nil || lngErr != nil {
		return 0, 0, false
	}

	// También rechaza NaN e infinitos; 0,0 es una ubicación válida.
	valid := lat >= -90 && lat <= 90 &&
		lng >= -180 && lng <= 180

	return lat, lng, valid
}

func warehouseLocationContent(wh Warehouse, t func(key string, args ...any) string) mi.H {
	return func(b *mi.Builder) mi.Node {
		lat, lng, valid := parseWarehouseGPS(wh.Location)

		if !valid {
			return b.P(
				mi.Class("text-sm text-gray-500 dark:text-gray-400"),
				t("warehouse.location_unavailable"),
			)
		}

		mapData := struct {
			MapID string  `json:"mapId"`
			Name  string  `json:"name"`
			Lat   float64 `json:"lat"`
			Lng   float64 `json:"lng"`
		}{
			MapID: fmt.Sprintf("warehouse-location-map-%d", wh.ID),
			Name:  wh.Name,
			Lat:   lat,
			Lng:   lng,
		}

		locationJSON, err := json.Marshal(mapData)
		if err != nil {
			return b.P(
				mi.Class("text-sm text-gray-500 dark:text-gray-400"),
				t("warehouse.location_unavailable"),
			)
		}

		return b.Div(mi.Class("space-y-3"),
			b.Div(
				mi.ID(mapData.MapID),
				mi.Class("relative rounded-lg overflow-hidden border border-gray-200 dark:border-gray-700"),
				mi.Attr("style", "height: 320px; width: 100%; z-index: 0;"),
			),

			b.P(
				mi.Class("text-sm text-gray-500 dark:text-gray-400"),
				wh.Location,
			),

			b.Link(
				mi.Rel("stylesheet"),
				mi.Href("/static/vendor/leaflet@1.9.4.css"),
			),
			b.Script(
				mi.Src("/static/vendor/leaflet@1.9.4.js"),
			),

			b.Script(mi.Raw(fmt.Sprintf(`
				(function () {
					var data = %s;

					function initializeMap() {
						var element = document.getElementById(data.mapId);

						if (!element ||
							element.dataset.warehouseMapInitialized === 'true') {
							return;
						}

						var coordinates = [data.lat, data.lng];
						var map = L.map(element).setView(coordinates, 16);

						element.dataset.warehouseMapInitialized = 'true';

						L.tileLayer(
							'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png',
							{
								attribution: '© OpenStreetMap contributors',
								maxZoom: 19
							}
						).addTo(map);

						var locationIcon = L.divIcon({
							className: 'warehouse-location-marker',
							html: '<span class="material-icons" aria-hidden="true" ' +
								'style="display:block;font-size:40px;line-height:40px;' +
								'color:#4f46e5;text-shadow:0 1px 3px rgba(0,0,0,0.5);">' +
								'place</span>',
							iconSize: [40, 40],
							iconAnchor: [20, 40],
							popupAnchor: [0, -36]
						});

						var marker = L.marker(coordinates, {
							icon: locationIcon,
							title: data.name
						}).addTo(map);

						var popup = document.createElement('strong');
						popup.textContent = data.name;
						marker.bindPopup(popup);

						requestAnimationFrame(function () {
							map.invalidateSize();
						});
					}

					if (window.L) {
						initializeMap();
					} else {
						var script = document.querySelector(
							'script[src="/static/vendor/leaflet@1.9.4.js"]'
						);

						if (script) {
							script.addEventListener(
								'load',
								initializeMap,
								{ once: true }
							);
						}
					}
				})();
			`, string(locationJSON)))),
		)
	}
}

func warehouseConsumablesContent(items []WarehouseConsumableStock, t func(key string, args ...any) string) mi.H {
	textCell := func(value string) mi.H {
		return ListCellRich(func(b *mi.Builder) mi.Node {
			return b.Span(
				mi.Class("text-gray-900 dark:text-gray-100"),
				value,
			)
		})
	}

	rows := make([]mi.H, len(items))

	for i, item := range items {
		detailURL := fmt.Sprintf("/consumable/%d", item.ConsumableID)

		stockLabel := strconv.FormatFloat(item.Stock, 'f', -1, 64)
		if item.Unit != "" {
			stockLabel += " " + item.Unit
		}

		rows[i] = ListRow(
			i,
			textCell(item.Name),
			textCell(stockLabel),

			ListCellRich(func(b *mi.Builder) mi.Node {
				return b.A(
					mi.Href(detailURL),
					mi.Class("inline-flex items-center justify-center rounded-lg p-2 text-indigo-600 dark:text-indigo-400 hover:bg-indigo-50 dark:hover:bg-gray-700"),
					mi.Attr("title", t("warehouse.consumables.view_details")),
					mi.Attr("aria-label", t("warehouse.consumables.view_details")),

					b.Span(
						mi.Class("material-icons"),
						mi.Attr("aria-hidden", "true"),
						"info",
					),
				)
			}),
		)
	}

	table := ListTable(ListTableConfig{
		Columns: []ListColumn{
			{Label: t("field.name")},
			{Label: t("warehouse.consumables.stock"), Right: true},
			{Label: t("asset.list.actions"), Right: true},
		},
		EmptyIcon:    "shelves",
		EmptyTitle:   t("warehouse.consumables.title"),
		EmptyMessage: t("warehouse.consumables.empty"),
	}, rows)

	return func(b *mi.Builder) mi.Node {
		return b.Div(
			mi.Class("overflow-x-auto"),
			table(b),
		)
	}
}

// warehouseConsumableStocks reads the warehouse's stored balances
// and resolves consumable names for display.
func (h *UIHandler) warehouseConsumableStocks(ctx context.Context, stored interface{}) ([]WarehouseConsumableStock, error) {
	if stored == nil {
		return []WarehouseConsumableStock{}, nil
	}

	raw, err := json.Marshal(stored)
	if err != nil {
		return nil, fmt.Errorf("encode warehouse consumables: %w", err)
	}

	var balances []WarehouseConsumableBalance
	if err := json.Unmarshal(raw, &balances); err != nil {
		return nil, fmt.Errorf("decode warehouse consumables: %w", err)
	}

	if len(balances) == 0 {
		return []WarehouseConsumableStock{}, nil
	}

	ids := make([]string, 0, len(balances))
	seen := make(map[int64]bool, len(balances))

	for _, balance := range balances {
		if balance.ConsumableID <= 0 {
			return nil, fmt.Errorf("invalid stored consumable ID")
		}

		if balance.Stock < 0 {
			return nil, fmt.Errorf(
				"negative stored stock for consumable %d",
				balance.ConsumableID,
			)
		}

		if seen[balance.ConsumableID] {
			return nil, fmt.Errorf(
				"duplicate stored consumable %d",
				balance.ConsumableID,
			)
		}

		seen[balance.ConsumableID] = true
		ids = append(
			ids,
			strconv.FormatInt(balance.ConsumableID, 10),
		)
	}

	result, err := h.xoluClient.OQL(ctx, fmt.Sprintf(
		"SELECT id, name, consumable_unit FROM consumable WHERE id IN (%s)",
		strings.Join(ids, ","),
	))
	if err != nil {
		return nil, fmt.Errorf("query warehouse consumables: %w", err)
	}

	names := make(map[int64]string)
	unitByConsumable := make(map[int64]int64)
	unitIDs := make(map[int64]bool)

	if result != nil {
		for _, row := range result.Data {
			consumableID := toInt64(row["id"])
			unitID := toInt64(row["consumable_unit"])

			names[consumableID] = toString(row["name"])
			unitByConsumable[consumableID] = unitID

			if unitID > 0 {
				unitIDs[unitID] = true
			}
		}
	}

	unitLabels := make(map[int64]string)

	if len(unitIDs) > 0 {
		unitIDStrings := make([]string, 0, len(unitIDs))

		for unitID := range unitIDs {
			unitIDStrings = append(
				unitIDStrings,
				strconv.FormatInt(unitID, 10),
			)
		}

		unitResult, err := h.xoluClient.OQL(ctx, fmt.Sprintf(
			"SELECT id, symbol, name FROM unit WHERE id IN (%s)",
			strings.Join(unitIDStrings, ","),
		))
		if err != nil {
			return nil, fmt.Errorf("query consumable units: %w", err)
		}

		if unitResult != nil {
			for _, row := range unitResult.Data {
				label := strings.TrimSpace(toString(row["symbol"]))
				if label == "" {
					label = strings.TrimSpace(toString(row["name"]))
				}

				unitLabels[toInt64(row["id"])] = label
			}
		}
	}

	items := make([]WarehouseConsumableStock, 0, len(balances))

	for _, balance := range balances {
		name := names[balance.ConsumableID]

		if strings.TrimSpace(name) == "" {
			name = fmt.Sprintf("#%d", balance.ConsumableID)
		}

		items = append(items, WarehouseConsumableStock{
			ConsumableID: balance.ConsumableID,
			Name:         name,
			Stock:        balance.Stock,
			Unit:         unitLabels[unitByConsumable[balance.ConsumableID]],
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].ConsumableID < items[j].ConsumableID
		}
		return items[i].Name < items[j].Name
	})

	return items, nil
}

// warehouseEditEntity checks administrative access and loads the warehouse
// addressed by the edit routes. It uses the same fresh-read helper as the
// detail page and distinguishes missing records from backend failures.
func (h *UIHandler) warehouseEditEntity(w http.ResponseWriter, r *http.Request) (*xolu.Entity, bool) {
	t := h.translateFunc(r)

	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims.UserID <= 0 {
		http.Error(w, t("error.unauthorized"), http.StatusUnauthorized)
		return nil, false
	}

	if !claims.HasRole(auth.RoleAdmin) {
		http.Error(w, t("error.forbidden"), http.StatusForbidden)
		return nil, false
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, t("warehouse.not_found"), http.StatusNotFound)
		return nil, false
	}

	entity, err := h.movementGetFresh(r.Context(), "warehouse", id)
	if errors.Is(err, ErrEntityNotFound) {
		http.Error(w, t("warehouse.not_found"), http.StatusNotFound)
		return nil, false
	}

	if err != nil {
		h.logger.Error("failed to load warehouse for editing", "warehouse_id", id, "error", err)
		http.Error(w, t("warehouse.error.load_failed"), http.StatusInternalServerError)

		return nil, false
	}

	if entity == nil {
		http.Error(w, t("warehouse.not_found"), http.StatusNotFound)
		return nil, false
	}

	return entity, true
}

// WarehouseEdit handles GET /warehouses/{id}/edit.
func (h *UIHandler) WarehouseEdit(w http.ResponseWriter, r *http.Request) {
	entity, ok := h.warehouseEditEntity(w, r)
	if !ok {
		return
	}

	values := map[string]interface{}{
		"name":                toString(entity.Data["name"]),
		"description":         toString(entity.Data["description"]),
		"location":            toString(entity.Data["location"]),
		"responsible_user_id": toInt64(entity.Data["responsible_user_id"]),
	}

	h.renderWarehouseEditForm(w, r, entity.ID, values, nil)
}

// WarehouseUpdate handles POST /warehouses/{id}.
//
// Only descriptive fields are patched. Enabled state and inventory data are
// excluded even if the request contains additional form fields.
func (h *UIHandler) WarehouseUpdate(w http.ResponseWriter, r *http.Request) {
	entity, ok := h.warehouseEditEntity(w, r)
	if !ok {
		return
	}

	t := h.translateFunc(r)
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, t("error.invalid_form_data"), http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.PostForm.Get("name"))
	description := strings.TrimSpace(r.PostForm.Get("description"))
	location := strings.TrimSpace(r.PostForm.Get("location"))
	responsibleText := strings.TrimSpace(
		r.PostForm.Get("responsible_user_id"),
	)

	values := map[string]interface{}{
		"name":                name,
		"description":         description,
		"location":            location,
		"responsible_user_id": responsibleText,
	}

	fieldErrors := make(map[string]string)

	if name == "" {
		fieldErrors["name"] = t("warehouse.error.name_required")
	} else if utf8.RuneCountInString(name) > 200 {
		fieldErrors["name"] = t("warehouse.error.name_too_long")
	}

	responsibleID, err := strconv.ParseInt(responsibleText, 10, 64)

	switch {
	case responsibleText == "":
		fieldErrors["responsible_user_id"] =
			t("warehouse.error.responsible_required")

	case err != nil || responsibleID <= 0:
		fieldErrors["responsible_user_id"] =
			t("warehouse.error.responsible_invalid")

	default:
		user, err := h.xoluClient.Get(ctx, "users", responsibleID)

		if err != nil {
			h.logger.Warn("failed to validate warehouse responsible", "user_id", responsibleID, "error", err)

			var clientErr *xolu.Error
			if errors.As(err, &clientErr) &&
				clientErr.HTTPStatus == http.StatusNotFound {
				fieldErrors["responsible_user_id"] =
					t("warehouse.error.responsible_invalid")
			} else {
				fieldErrors["responsible_user_id"] =
					t("warehouse.error.responsible_load_failed")
			}
		} else if user == nil {
			fieldErrors["responsible_user_id"] =
				t("warehouse.error.responsible_invalid")
		}
	}

	if location == "" {
		fieldErrors["location"] = t("warehouse.error.location_required")
	} else {
		lat, lng, valid := parseWarehouseGPS(location)

		if !valid {
			fieldErrors["location"] = t("warehouse.error.location_invalid")
		} else {
			location = strconv.FormatFloat(lat, 'f', -1, 64) + "," +
				strconv.FormatFloat(lng, 'f', -1, 64)
		}
	}

	if len(fieldErrors) > 0 {
		h.renderWarehouseEditForm(w, r, entity.ID, values, fieldErrors)
		return
	}

	changes := map[string]any{
		"name":                name,
		"description":         description,
		"location":            location,
		"responsible_user_id": responsibleID,
	}

	if _, err := h.xoluClient.Patch(
		ctx, "warehouse", entity.ID, changes,
	); err != nil {
		h.logger.Error("failed to update warehouse", "warehouse_id", entity.ID, "error", err)

		fieldErrors["_"] = t("warehouse.error.update_failed")

		h.renderWarehouseEditForm(w, r, entity.ID, values, fieldErrors)
		return
	}

	detailURL := "/warehouse/" + strconv.FormatInt(entity.ID, 10)

	if IsHTMXRequest(r) {
		HTMXRedirect(w, detailURL)
		return
	}

	http.Redirect(w, r, detailURL, http.StatusSeeOther)
}

// renderWarehouseEditForm renders the same edit form for initial loading and
// validation failures. HTMX requests receive modal content; direct navigation
// receives a complete page.
func (h *UIHandler) renderWarehouseEditForm(w http.ResponseWriter, r *http.Request, warehouseID int64, values map[string]interface{}, fieldErrors map[string]string) {
	t := h.translateFunc(r)
	csrfToken := h.GetCSRFToken(w, r)
	isPartial := IsHTMXRequest(r)

	formEng := &fe.FormEngine{
		Translator: h.formEngineTranslator(),
	}

	rc := fe.RenderContext{
		Locale:     h.currentLocale(),
		CSRFToken:  csrfToken,
		Module:     "warehouse",
		Form:       "warehouse",
		IsEdit:     true,
		EntityID:   warehouseID,
		EntityType: "warehouse",
		IsPartial:  isPartial,
	}

	content := formEng.Render(r.Context(), resolveSchema(r.Context(), "warehouse", "edit", warehouseschemas.Schema("edit")), values, fieldErrors, rc)

	data := WarehouseFormData{
		ID:          warehouseID,
		IsPartial:   isPartial,
		CSRFToken:   csrfToken,
		FormContent: content,
		Errors:      fieldErrors,
	}

	page := WarehouseFormPage(data, t)

	if isPartial {
		h.page.RenderPartial(w, page)
		return
	}

	h.page.Render(w, r, t("warehouse.edit"), "/warehouses", page)
}

// warehouseHasStock reports whether any consumable has a positive balance.
// Zero-balance entries do not prevent disabling the warehouse.
func warehouseHasStock(items []WarehouseConsumableStock) bool {
	for _, item := range items {
		if item.Stock > 0 {
			return true
		}
	}
	return false
}

// warehouseDisableBlockedContent explains why disabling was rejected and
// provides access to the transfer-only movement form.
func warehouseDisableBlockedContent(warehouseID int64, t func(key string, args ...any) string) mi.H {
	return func(b *mi.Builder) mi.Node {
		return b.Div(
			mi.Attr("role", "alert"),
			mi.Class(
				"rounded-lg border border-amber-300 dark:border-amber-700 "+
					"bg-amber-50 dark:bg-amber-900/20 p-4 space-y-3",
			),
			b.P(mi.Class("text-sm text-amber-900 dark:text-white"), t("warehouse.error.stock_remaining")),
			Button(t("warehouse.action.transfer_stock"), "primary", mi.Type("button"), mi.Attr("hx-get", fmt.Sprintf("/warehouses/%d/empty", warehouseID)),
				mi.Attr("hx-target", "#seam-modal-body"),
				mi.Attr("hx-swap", "innerHTML"),
				mi.Attr("onclick", fmt.Sprintf("SeamModal.open(%q)", t("warehouse.action.transfer_stock"))),
			)(b),
		)
	}
}

// WarehouseSetEnabled handles POST /warehouses/{id}/enabled.
//
// The request specifies the desired state instead of toggling the stored
// value, so repeating the same request does not reverse the previous change.
func (h *UIHandler) WarehouseSetEnabled(w http.ResponseWriter, r *http.Request) {
	entity, ok := h.warehouseEditEntity(w, r)
	if !ok {
		return
	}

	t := h.translateFunc(r)

	if err := r.ParseForm(); err != nil {
		http.Error(w, t("error.invalid_form_data"), http.StatusBadRequest)
		return
	}

	var enabled bool

	switch r.PostForm.Get("enabled") {
	case "true":
		enabled = true
	case "false":
		enabled = false
	default:
		http.Error(w, t("error.invalid_form_data"), http.StatusBadRequest)
		return
	}

	inventoryMutationMu.Lock()
	defer inventoryMutationMu.Unlock()

	if !enabled {
		items, err := h.inventoryForWarehouse(r.Context(), entity)
		if err != nil {
			h.logger.Error("failed to check warehouse stock before disabling", "warehouse_id", entity.ID, "error", err)

			http.Error(w, t("warehouse.consumables.load_failed"), http.StatusInternalServerError)
			return
		}

		if warehouseHasStock(items) {
			if IsHTMXRequest(r) {
				h.page.RenderPartial(w, warehouseDisableBlockedContent(entity.ID, t))
			} else {
				http.Error(w, t("warehouse.error.stock_remaining"), http.StatusConflict)
			}
			return
		}
	}

	if _, err := h.xoluClient.Patch(r.Context(), "warehouse", entity.ID, map[string]any{"enabled": enabled}); err != nil {
		h.logger.Error("failed to change warehouse enabled state", "warehouse_id", entity.ID, "enabled", enabled, "error", err)

		http.Error(w, t("warehouse.error.update_failed"), http.StatusInternalServerError)
		return
	}

	detailURL := fmt.Sprintf("/warehouse/%d", entity.ID)

	if IsHTMXRequest(r) {
		HTMXRedirect(w, detailURL)
		return
	}

	http.Redirect(w, r, detailURL, http.StatusSeeOther)
}
