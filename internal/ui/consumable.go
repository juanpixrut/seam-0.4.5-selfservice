package ui

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	mi "github.com/ha1tch/minty"
	"github.com/ha1tch/seam-ui/internal/auth"
	fe "github.com/ha1tch/seam-ui/internal/formengine"
	consumableschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/consumable"
)

type Consumable struct {
	ID               int64
	Name             string
	Description      string
	ConsumableType   string
	ConsumableTypeID int64
	ConsumableUnitID int64
	ConsumableUnit   string
	StockMin         float64
	Enabled          bool
	LowStock         bool
}

type ConsumableWarehouseStock struct {
	WarehouseID   int64
	WarehouseName string
	Quantity      float64
}

func (c *Consumable) setConsumableTypeName(name string) {
	if name != "" {
		c.ConsumableType = name
	} else if c.ConsumableType == "" && c.ConsumableTypeID > 0 {
		c.ConsumableType = fmt.Sprintf("#%d", c.ConsumableTypeID)
	}
}

func (h *UIHandler) ConsumableList(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()

	result, err := h.xoluClient.OQL(
		ctx,
		"SELECT TOP 100 * FROM consumable ORDER BY id DESC",
	)

	if err != nil {
		h.logger.Error("failed to list consumables", "error", err)
		result = nil
	}

	var consumables []Consumable

	if result != nil {
		for _, row := range result.Data {

			consumables = append(consumables, Consumable{
				ID:               toInt64(row["id"]),
				Name:             toString(row["name"]),
				Description:      toString(row["description"]),
				ConsumableType:   toString(row["consumable_type"]),
				ConsumableTypeID: toInt64(row["consumable_type_id"]),
				ConsumableUnitID: toInt64(row["consumable_unit"]),
				StockMin:         toFloat(row["stock_min"]),
				Enabled:          toBool(row["enabled"]),
			})

		}
	}

	if result != nil {
		inventory, err := h.inventoryByConsumable(ctx, result.Data)
		if err != nil {
			h.logger.Error("failed to load inventory", "error", err)
			h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.server_error"))
			return
		}

		for i := range consumables {
			for _, units := range inventory[consumables[i].ID] {
				if float64(units)/100 <= consumables[i].StockMin {
					consumables[i].LowStock = true
					break
				}
			}
		}
	}

	typeIDs := make(map[int64]bool)
	unitIDs := make(map[int64]bool)

	for _, consumable := range consumables {
		if consumable.ConsumableTypeID > 0 {
			typeIDs[consumable.ConsumableTypeID] = true
		}
		if consumable.ConsumableUnitID > 0 {
			unitIDs[consumable.ConsumableUnitID] = true
		}
	}

	if len(typeIDs) > 0 {
		ids := make([]string, 0, len(typeIDs))
		for id := range typeIDs {
			ids = append(ids, strconv.FormatInt(id, 10))
		}
		query := "SELECT id, name FROM consumable_types WHERE id IN (" + strings.Join(ids, ",") + ")"
		types, err := h.xoluClient.OQL(ctx, query)
		labels := make(map[int64]string)
		if err == nil {
			for _, row := range types.Data {
				labels[toInt64(row["id"])] = toString(row["name"])
			}
		} else if !entityNotYetCreated(err) {
			h.logger.Error("failed to load consumable type names", "error", err)
		}
		for i := range consumables {
			consumables[i].setConsumableTypeName(labels[consumables[i].ConsumableTypeID])
		}
	}

	if len(unitIDs) > 0 {
		ids := make([]string, 0, len(unitIDs))

		for id := range unitIDs {
			ids = append(ids, strconv.FormatInt(id, 10))
		}

		units, err := h.xoluClient.OQL(
			ctx,
			fmt.Sprintf(
				"SELECT id, name, symbol FROM unit WHERE id IN (%s)",
				strings.Join(ids, ","),
			),
		)

		if err != nil {
			h.page.RenderError(
				w,
				http.StatusInternalServerError,
				t("error.generic"),
				t("unit.error.load_failed"),
			)
			return
		}

		labels := make(map[int64]string)

		if units != nil {
			for _, row := range units.Data {
				label := toString(row["symbol"])
				if label == "" {
					label = toString(row["name"])
				}

				labels[toInt64(row["id"])] = label
			}
		}

		for i := range consumables {
			id := consumables[i].ConsumableUnitID
			consumables[i].ConsumableUnit = labels[id]

			if consumables[i].ConsumableUnit == "" && id > 0 {
				consumables[i].ConsumableUnit = fmt.Sprintf("#%d", id)
			}
		}
	}

	data := ConsumableListData{
		Consumables: consumables,
	}

	h.page.Render(
		w,
		r,
		t("consumable.list_title"),
		"/consumable",
		ConsumableListPage(data, t),
	)
}

func (h *UIHandler) ConsumableNew(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	csrfToken := h.GetCSRFToken(w, r)

	formEng := &fe.FormEngine{
		Translator: h.formEngineTranslator(),
	}

	rc := fe.RenderContext{
		Locale:    h.currentLocale(),
		CSRFToken: csrfToken,
		Module:    "consumable",
		Form:      "consumable",
	}

	formContent := formEng.Render(
		ctx,
		resolveSchema(ctx, "consumable", "create", consumableschemas.Schema("create")),
		nil,
		nil,
		rc,
	)

	data := ConsumableFormData{
		CSRFToken:   csrfToken,
		FormContent: formContent,
	}

	h.page.Render(
		w,
		r,
		t("consumable.new"),
		"/consumable",
		ConsumableFormPage(data, t),
	)
}

func (h *UIHandler) ConsumableCreate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, h.errText(r, "error.invalid_form_data", "Invalid form data"), http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	description := r.FormValue("description")
	consumableType := strings.TrimSpace(r.PostForm.Get("consumable_type"))
	consumableUnitText := strings.TrimSpace(r.FormValue("consumable_unit"))
	stockMinText := strings.TrimSpace(r.FormValue("stock_min"))
	enabled := r.FormValue("enabled") == "on"
	_, addReceivingWarehouse := r.PostForm["add_receiving_warehouse"]
	receivingWarehouseText := strings.TrimSpace(r.FormValue("receiving_warehouse"))
	stockText := strings.TrimSpace(r.FormValue("stock"))
	csrfToken := h.GetCSRFToken(w, r)

	errors := make(map[string]string)

	if addReceivingWarehouse && !enabled {
		errors["enabled"] = t("movement.error.consumable_disabled")
	}

	if name == "" {
		errors["name"] = t("consumable.error.name_required")
	} else if utf8.RuneCountInString(name) > 200 {
		errors["name"] = t("consumable.error.name_too_long")
	}
	var consumableTypeID int64
	if consumableType != "" {
		id, err := strconv.ParseInt(consumableType, 10, 64)
		if err != nil || id <= 0 {
			errors["consumable_type"] = t("consumable_types.invalid")
		} else if _, err := h.xoluClient.Get(ctx, consumableTypesEntity, id); err != nil {
			errors["consumable_type"] = t("consumable_types.invalid")
		} else {
			consumableTypeID = id
		}
	}
	consumableUnitID, err := strconv.ParseInt(consumableUnitText, 10, 64)

	if err != nil || consumableUnitID <= 0 {
		errors["consumable_unit"] = t("consumable.error.invalid_unit")
	} else {
		if _, err := h.xoluClient.Get(ctx, "unit", consumableUnitID); err != nil {
			errors["consumable_unit"] = t("consumable.error.invalid_unit")
		}
	}

	minStock := toFloat64OrZero(stockMinText)

	if minStock < 0 {
		errors["stock_min"] = t("consumable.error.stock_min")
	}

	var warehouseID int64

	var initialStock float64

	if addReceivingWarehouse {
		var err error

		warehouseID, err = strconv.ParseInt(receivingWarehouseText, 10, 64)

		if err != nil || warehouseID <= 0 {
			errors["receiving_warehouse"] = t("consumable.error.invalid_warehouse")
		} else {
			warehouse, loadErr := h.movementGetFresh(ctx, "warehouse", warehouseID)

			if loadErr != nil {
				h.logger.Warn("receiving warehouse unavailable", "warehouse_id", warehouseID, "error", loadErr)

				errors["receiving_warehouse"] = t("consumable.error.invalid_warehouse")
			} else if !toBool(warehouse.Data["enabled"]) {
				errors["receiving_warehouse"] = t("movement.error.warehouse_disabled")
			}
		}

		initialStock = toFloat64OrZero(stockText)

		if initialStock <= 0 || initialStock < minStock {
			errors["stock"] = t("consumable.error.invalid_stock")
		}
	}

	if len(errors) > 0 {
		formEng := &fe.FormEngine{Translator: h.formEngineTranslator()}
		rc := fe.RenderContext{
			Locale: h.currentLocale(), CSRFToken: csrfToken,
			Module: "consumable", Form: "consumable",
		}
		formValues := map[string]interface{}{
			"name":                    name,
			"description":             description,
			"consumable_type":         consumableType,
			"consumable_unit":         consumableUnitText,
			"stock_min":               stockMinText,
			"add_receiving_warehouse": addReceivingWarehouse,
			"receiving_warehouse":     receivingWarehouseText,
			"stock":                   stockText,
			"enabled":                 enabled,
		}
		formContent := formEng.Render(ctx, resolveSchema(ctx, "consumable", "create", consumableschemas.Schema("create")), formValues, errors, rc)
		data := ConsumableFormData{CSRFToken: csrfToken, FormContent: formContent, Errors: errors}
		h.page.Render(w, r, t("consumable.new"), "/consumable", ConsumableFormPage(data, t))
		return
	}

	var actorID int64

	if addReceivingWarehouse {
		claims, ok := auth.ClaimsFromContext(ctx)

		if !ok || claims.UserID <= 0 {
			h.logger.Error("cannot create initial movement: authenticated user id missing")
			http.Error(w, h.errText(r, "error.auth_required", "Authenticated user ID unavailable"), http.StatusUnauthorized)
			return
		}

		actorID = claims.UserID
	}

	consumableData := map[string]any{
		"name":            name,
		"description":     description,
		"consumable_unit": consumableUnitID,
		"stock_min":       minStock,
		"enabled":         enabled,
	}

	if consumableTypeID > 0 {
		consumableData["consumable_type_id"] = consumableTypeID
	}
	entity, err := h.xoluClient.Create(ctx, "consumable", consumableData)
	if err != nil {
		h.logger.Error("failed to create consumable", "error", err)
		http.Error(w, h.errText(r, "error.save_failed", "Failed to save consumable"), http.StatusInternalServerError)
		return
	}

	if addReceivingWarehouse {
		err := h.registerInitialConsumableStock(
			ctx,
			actorID,
			warehouseID,
			entity.ID,
			initialStock,
			t("movement.reason.initial_stock"),
		)
		if err != nil {
			h.logger.Error(
				"consumable created but initial inventory registration failed",
				"consumable_id", entity.ID, "warehouse_id", warehouseID, "error", err,
			)
			http.Error(
				w, t("consumable.error.inventory_incomplete"), http.StatusInternalServerError,
			)
			return
		}
	}

	http.Redirect(w, r, "/consumable/"+strconv.FormatInt(entity.ID, 10), http.StatusSeeOther)
}

func (h *UIHandler) ConsumableDetail(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)

	if err != nil || id <= 0 {
		h.page.RenderError(
			w,
			http.StatusNotFound,
			t("error.not_found"),
			t("error.not_found"),
		)
		return
	}

	entity, err := h.movementGetFresh(ctx, "consumable", id)

	if err != nil {
		h.page.RenderError(
			w,
			http.StatusNotFound,
			t("error.not_found"),
			t("error.not_found"),
		)
		return
	}

	consumable := Consumable{
		ID:               entity.ID,
		Name:             toString(entity.Data["name"]),
		Description:      toString(entity.Data["description"]),
		ConsumableType:   toString(entity.Data["consumable_type"]),
		ConsumableTypeID: toInt64(entity.Data["consumable_type_id"]),
		ConsumableUnitID: toInt64(entity.Data["consumable_unit"]),
		StockMin:         toFloat(entity.Data["stock_min"]),
		Enabled:          toBool(entity.Data["enabled"]),
	}

	if consumable.ConsumableTypeID > 0 {
		typ, err := h.xoluClient.Get(ctx, consumableTypesEntity, consumable.ConsumableTypeID)
		if err == nil {
			consumable.setConsumableTypeName(toString(typ.Data["name"]))
		} else {
			h.logger.Warn("failed to resolve consumable type", "id", consumable.ConsumableTypeID, "error", err)
			consumable.setConsumableTypeName("")
		}
	}

	inventory, err := h.inventoryByConsumable(ctx, []map[string]any{entity.Data})
	if err != nil {
		h.logger.Error("failed to load inventory", "error", err)
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.server_error"))
		return
	}

	balances := make(map[int64]float64)
	for warehouseID, units := range inventory[consumable.ID] {
		balances[warehouseID] = float64(units) / 100
	}

	ids := make([]int64, 0, len(balances))
	for id := range balances {
		if id > 0 {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	var warehouses []ConsumableWarehouseStock
	if len(ids) > 0 {
		idStrings := make([]string, 0, len(ids))
		for _, id := range ids {
			idStrings = append(idStrings, strconv.FormatInt(id, 10))
		}
		query := fmt.Sprintf("SELECT id, name FROM warehouse WHERE id IN (%s)", strings.Join(idStrings, ","))
		result, err := h.xoluClient.OQL(ctx, query)
		if err != nil {
			h.logger.Error("failed to fetch warehouse names", "error", err)
			h.page.RenderError(w, http.StatusInternalServerError, "Warehouses unavailable", "Failed to load warehouse names")
			return
		}
		names := make(map[int64]string)
		if result != nil {
			for _, row := range result.Data {
				names[toInt64(row["id"])] = toString(row["name"])
			}
		}
		for _, id := range ids {
			warehouses = append(warehouses, ConsumableWarehouseStock{
				WarehouseID:   id,
				WarehouseName: names[id],
				Quantity:      balances[id],
			})
		}
	}

	if consumable.ConsumableUnitID > 0 {
		unit, err := h.xoluClient.Get(ctx, "unit", consumable.ConsumableUnitID)
		if err != nil {
			h.page.RenderError(
				w,
				http.StatusInternalServerError,
				t("error.generic"),
				t("unit.error.load_failed"),
			)
			return
		}

		consumable.ConsumableUnit = toString(unit.Data["symbol"])

		if consumable.ConsumableUnit == "" {
			consumable.ConsumableUnit = toString(unit.Data["name"])
		}
	}

	data := ConsumableDetailData{
		Consumable: consumable,
		Warehouses: warehouses,
		CSRFToken:  h.GetCSRFToken(w, r),
	}

	h.page.Render(
		w,
		r,
		t("consumable.detail_title"),
		"/consumable",
		ConsumableDetailPage(data, t),
	)
}

type ConsumableListData struct {
	Consumables []Consumable
}

func ConsumableListPage(
	data ConsumableListData,
	t func(key string, args ...any) string,
) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}

	rows := make([]mi.H, len(data.Consumables))

	for i, consumable := range data.Consumables {
		enabledLabel := t("consumable.inactive")
		variant := "default"

		if consumable.Enabled {
			enabledLabel = t("consumable.enabled")
			variant = "success"
		}

		rows[i] = ListRow(
			i,
			ListCellRich(func(b *mi.Builder) mi.Node {
				if consumable.LowStock {
					return b.Div(
						mi.Class("space-y-1"),
						b.A(
							mi.Href(fmt.Sprintf("/consumable/%d", consumable.ID)),
							mi.Class("font-medium text-indigo-600 dark:text-indigo-400 hover:underline"),
							consumable.Name,
						),
						b.Div(
							mi.Class("flex items-center gap-1 text-xs text-amber-700 dark:text-amber-400"),
							b.Span(
								mi.Class("material-icons text-sm"),
								mi.Attr("aria-hidden", "true"),
								"warning",
							),
							t("consumable.low_stock_warning"),
						),
					)
				}

				return b.A(
					mi.Href(fmt.Sprintf("/consumable/%d", consumable.ID)),
					mi.Class("font-medium text-indigo-600 dark:text-indigo-400 hover:underline"),
					consumable.Name,
				)
			}),
			ListCellMuted(consumable.ConsumableType),
			ListCellMuted(consumable.ConsumableUnit),
			ListCellMuted(strconv.FormatFloat(consumable.StockMin, 'f', -1, 64)),
			ListCellRich(func(b *mi.Builder) mi.Node {
				return Badge(enabledLabel, variant)(b)
			}),
		)
	}

	return ListPage(
		ListPageConfig{
			Title:       t("consumable.list_title"),
			CreateURL:   "/consumable/new",
			CreateLabel: t("consumable.new"),
		},
		ListTable(ListTableConfig{
			Columns: []ListColumn{
				{Label: t("field.name")},
				{Label: t("consumable.field.consumable_type")},
				{Label: t("consumable.field.consumable_unit")},
				{Label: t("consumable.field.stock_min")},
				{Label: t("consumable.forms.consumable.field.enabled")},
			},
			EmptyIcon:    "shelves",
			EmptyTitle:   t("consumable.list_title"),
			EmptyMessage: t("consumable.empty"),
		}, rows),
		nil,
		nil,
	)
}

type ConsumableFormData struct {
	CSRFToken   string
	FormContent mi.H
	Errors      map[string]string
}

func ConsumableFormPage(
	data ConsumableFormData,
	t func(key string, args ...any) string,
) mi.H {

	if t == nil {
		t = func(key string, args ...any) string { return key }
	}

	return func(b *mi.Builder) mi.Node {
		return b.Div(
			mi.Class("space-y-6 max-w-2xl"),

			b.Div(
				mi.Class("flex items-center gap-4"),

				b.A(
					mi.Href("/consumable"),
					mi.Class("text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"),
					b.Span(
						mi.Class("material-icons"),
						"arrow_back",
					),
				),

				b.H1(
					mi.Class("text-2xl font-bold text-gray-900 dark:text-white"),
					t("consumable.new"),
				),
			),

			Card("", func(b *mi.Builder) mi.Node {
				return b.Form(
					mi.Class("space-y-4"),
					mi.Method("post"),
					mi.Action("/consumable"),

					data.FormContent(b),

					b.Div(
						mi.Class("flex justify-end gap-3 pt-4"),

						b.A(
							mi.Href("/consumable"),
							mi.Class("px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg font-medium hover:bg-gray-50 dark:hover:bg-gray-700"),
							t("action.cancel"),
						),

						b.Button(
							mi.Type("submit"),
							mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 font-medium"),
							t("consumable.action.create"),
						),
					),
				)
			})(b),
		)
	}
}

type ConsumableDetailData struct {
	Consumable Consumable
	Warehouses []ConsumableWarehouseStock
	CSRFToken  string
}

func ConsumableDetailPage(
	data ConsumableDetailData,
	t func(key string, args ...any) string,
) mi.H {

	if t == nil {
		t = func(key string, args ...any) string { return key }
	}

	c := data.Consumable

	enabledLabel := t("consumable.inactive")
	variant := "default"

	if c.Enabled {
		enabledLabel = t("consumable.enabled")
		variant = "success"
	}

	return func(b *mi.Builder) mi.Node {
		return b.Div(
			mi.Class("space-y-6"),

			b.Div(
				mi.Class("flex items-center justify-between"),

				b.Div(
					b.Nav(
						mi.Class("text-sm text-gray-500 dark:text-gray-400 mb-2"),

						b.A(
							mi.Href("/consumable"),
							mi.Class("hover:text-gray-700 dark:hover:text-gray-300"),
							t("consumable.list_title"),
						),

						b.Span(mi.Class("mx-2"), "/"),

						b.Span(
							mi.Class("text-gray-900 dark:text-white"),
							c.Name,
						),
					),

					b.Div(
						mi.Class("flex items-center gap-3"),
						b.H1(
							mi.Class("text-2xl font-bold text-gray-900 dark:text-white"),
							c.Name,
						),
						Badge(enabledLabel, variant)(b),
					),
				),
			),

			b.Div(
				mi.Class("grid grid-cols-1 lg:grid-cols-3 gap-6"),

				b.Div(
					mi.Class("lg:col-span-2 space-y-6"),

					Card(
						t("consumable.info_title"),
						consumableInfoContent(c, t),
					)(b),
				),

				b.Div(
					mi.Class("space-y-6"),

					Card(
						t("consumable.warehouse_title"),
						consumableWarehouseContent(c.ConsumableUnit, data.Warehouses, c.StockMin, t),
					)(b),
				),
			),
		)
	}
}

func consumableInfoContent(
	c Consumable,
	t func(key string, args ...any) string,
) mi.H {

	return func(b *mi.Builder) mi.Node {
		description := c.Description

		if description == "" {
			description = t("consumable.no_description")
		}

		return b.Dl(
			mi.Class("grid grid-cols-1 sm:grid-cols-2 gap-4"),

			infoRow(b, t("field.name"), c.Name),

			infoRow(
				b,
				t("consumable.field.consumable_type"),
				c.ConsumableType,
			),

			infoRow(
				b,
				t("consumable.field.stock_min"),
				strconv.FormatFloat(c.StockMin, 'f', -1, 64)+" "+c.ConsumableUnit,
			),

			b.Div(
				mi.Class("sm:col-span-2"),
				infoRow(b, t("field.description"), description),
			),
		)
	}
}

func consumableWarehouseContent(
	c string,
	warehouses []ConsumableWarehouseStock,
	stockMin float64,
	t func(key string, args ...any) string,
) mi.H {
	textCell := func(value string) mi.H {
		return ListCellRich(func(b *mi.Builder) mi.Node {
			return b.Span(
				mi.Class("text-gray-900 dark:text-gray-100"),
				value,
			)
		})
	}

	rows := make([]mi.H, len(warehouses))

	for i, wh := range warehouses {
		name := wh.WarehouseName
		if name == "" {
			name = fmt.Sprintf("#%d", wh.WarehouseID)
		}

		detailURL := fmt.Sprintf("/warehouse/%d", wh.WarehouseID)

		rows[i] = ListRow(
			i,
			textCell(name),
			ListCellRich(func(b *mi.Builder) mi.Node {
				quantity := strconv.FormatFloat(wh.Quantity, 'f', -1, 64)

				if wh.Quantity <= stockMin {
					return b.Span(
						mi.Class("inline-flex items-center gap-1 text-amber-700 dark:text-amber-400"),
						b.Span(
							mi.Class("material-icons text-base"),
							mi.Attr("title", t("consumable.low_stock")),
							mi.Attr("aria-label", t("consumable.low_stock")),
							"warning",
						),
						quantity+" "+c,
					)
				}

				return b.Span(
					mi.Class("text-gray-900 dark:text-gray-100"),
					quantity+" "+c,
				)
			}),
			ListCellRich(func(b *mi.Builder) mi.Node {
				return b.A(
					mi.Href(detailURL),
					mi.Class("inline-flex items-center justify-center rounded-lg p-2 text-indigo-600 dark:text-indigo-400 hover:bg-indigo-50 dark:hover:bg-gray-700"),
					mi.Attr("title", t("consumable.warehouse.view_details")),
					mi.Attr("aria-label", t("consumable.warehouse.view_details")),
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
			{Label: t("consumable.warehouse.stock"), Right: true},
			{Label: t("workorders-templates.info_title"), Right: true},
		},
		EmptyIcon:    "warehouse",
		EmptyTitle:   t("consumable.warehouse_title"),
		EmptyMessage: t("consumable.empty_stock"),
	}, rows)

	return func(b *mi.Builder) mi.Node {
		return b.Div(
			mi.Class("overflow-x-auto"),
			table(b),
		)
	}
}
