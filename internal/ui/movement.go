package ui

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	mi "github.com/ha1tch/minty"
	"github.com/ha1tch/seam-ui/internal/auth"
	fe "github.com/ha1tch/seam-ui/internal/formengine"
	movementschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/movement"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

type MovementType string

const (
	MovementEntry    MovementType = "entry"
	MovementExit     MovementType = "exit"
	MovementTransfer MovementType = "transfer"
	MovementAdjust   MovementType = "adjust"
)

var (
	ErrInvalidMovement   = errors.New("invalid inventory movement")
	ErrInsufficientStock = errors.New("insufficient stock")
	ErrEntityNotFound    = errors.New("consumable, warehouse or user not found")
)

type MovementRequest struct {
	ResponsibleUserID int64
	Type              MovementType
	OriginWarehouseID int64
	TargetWarehouseID int64
	ConsumableID      int64
	Quantity          int64
	Reason            string
	Reference         string
	RequestID         string
}

func ValidateMovement(req MovementRequest) error {
	if req.ResponsibleUserID <= 0 || req.ConsumableID <= 0 {
		return fmt.Errorf("%w: responsible user and consumable are required", ErrInvalidMovement)
	}
	if strings.TrimSpace(req.Reason) == "" {
		return fmt.Errorf("%w: reason is required", ErrInvalidMovement)
	}
	if req.Quantity < 0 {
		return fmt.Errorf("%w: quantity cannot be negative", ErrInvalidMovement)
	}
	switch req.Type {
	case MovementEntry:
		if req.Quantity == 0 || req.TargetWarehouseID <= 0 || req.OriginWarehouseID != 0 {
			return fmt.Errorf("%w: entry requires positive quantity and destination only", ErrInvalidMovement)
		}
	case MovementExit:
		if req.Quantity == 0 || req.OriginWarehouseID <= 0 || req.TargetWarehouseID != 0 {
			return fmt.Errorf("%w: exit requires positive quantity and origin only", ErrInvalidMovement)
		}
	case MovementTransfer:
		if req.Quantity == 0 || req.OriginWarehouseID <= 0 || req.TargetWarehouseID <= 0 || req.OriginWarehouseID == req.TargetWarehouseID {
			return fmt.Errorf("%w: transfer requires different origin and destination and positive quantity", ErrInvalidMovement)
		}
	case MovementAdjust:
		if req.TargetWarehouseID <= 0 || req.OriginWarehouseID != 0 {
			return fmt.Errorf("%w: adjustment requires destination warehouse only", ErrInvalidMovement)
		}
	default:
		return fmt.Errorf("%w: unknown type %q", ErrInvalidMovement, req.Type)
	}
	return nil
}

func (h *UIHandler) registerInitialConsumableStock(
	ctx context.Context,
	userID, warehouseID, consumableID int64,
	quantity float64,
	reason string,
) error {
	units, err := movementStoredUnits(quantity)
	if err != nil || units <= 0 {
		return ErrInvalidMovement
	}

	requestID, err := newMovementRequestID()
	if err != nil {
		return err
	}

	return h.saveMovementAtomic(ctx, MovementRequest{
		RequestID:         requestID,
		ResponsibleUserID: userID,
		Type:              MovementEntry,
		TargetWarehouseID: warehouseID,
		ConsumableID:      consumableID,
		Quantity:          units,
		Reason:            reason,
	})
}

func (h *UIHandler) WarehouseMovementNew(
	w http.ResponseWriter,
	r *http.Request,
) {
	t := h.translateFunc(r)

	warehouseID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || warehouseID <= 0 {
		http.Error(
			w,
			t("movement.error.invalid_warehouse"),
			http.StatusBadRequest,
		)
		return
	}

	h.renderMovementForm(w, r, warehouseID, nil, nil, "")
}

func (h *UIHandler) renderMovementForm(
	w http.ResponseWriter,
	r *http.Request,
	warehouseID int64,
	values map[string]interface{},
	fieldErrors map[string]string,
	message string,
) {
	t := h.translateFunc(r)
	ctx := r.Context()

	if _, ok := h.movementActor(w, r); !ok {
		return
	}

	warehouse, err := h.movementGetFresh(ctx, "warehouse", warehouseID)
	if err != nil {
		http.Error(
			w,
			t("movement.error.warehouse_load_failed"),
			http.StatusNotFound,
		)
		return
	}
	transferOnly := r.PathValue("movement_mode") == "transfer-only"

	data := MovementFormData{
		WarehouseID:   warehouseID,
		WarehouseName: toString(warehouse.Data["name"]),
		TransferOnly:  transferOnly,
	}

	if !toBool(warehouse.Data["enabled"]) {
		data.FormContent = movementFormMessage(
			t("movement.error.warehouse_disabled"),
		)
		_ = h.page.RenderPartial(w, MovementFormModal(data, t))
		return
	}

	sources, err := h.loadMovementFormSources(ctx, warehouseID, t)
	if err != nil {
		h.logger.Error(
			t("movement.error.options_load_failed"),
			"warehouse_id", warehouseID,
			"error", err,
		)

		data.FormContent = movementFormMessage(
			t("movement.error.options_load_failed"),
		)
		_ = h.page.RenderPartial(w, MovementFormModal(data, t))
		return
	}

	if transferOnly {
		items, err := h.inventoryForWarehouse(ctx, warehouse)
		if err != nil {
			h.logger.Error("failed to load warehouse stock for transfer", "warehouse_id", warehouseID, "error", err)

			data.FormContent = movementFormMessage(t("warehouse.consumables.load_failed"))
			h.page.RenderPartial(w, MovementFormModal(data, t))
			return
		}

		available := make(map[int64]bool)
		for _, item := range items {
			if item.Stock > 0 {
				available[item.ConsumableID] = true
			}
		}

		options := make([]fe.Option, 0)

		for _, option := range sources["entity:consumable"] {
			if option.Value == "" {
				options = append(options, option)
				continue
			}

			id, err := strconv.ParseInt(option.Value, 10, 64)
			if err == nil && available[id] {
				options = append(options, option)
			}
		}

		sources["entity:consumable"] = options

		hasConsumable := false
		for _, option := range options {
			if option.Value != "" {
				hasConsumable = true
				break
			}
		}

		if !hasConsumable {
			data.FormContent = movementFormMessage(t("warehouse.transfer_before_disable.no_available"))
			h.page.RenderPartial(w, MovementFormModal(data, t))
			return
		}

		if data.Message == "" {
			data.Message = t("warehouse.transfer_before_disable.help")
		}
	}

	formEng := &fe.FormEngine{
		Translator: h.formEngineTranslator(),
		Sources:    sources,
	}

	rc := fe.RenderContext{
		Locale:    h.currentLocale(),
		CSRFToken: h.GetCSRFToken(w, r),
		Module:    "movement",
		Form:      "movement",
		IsPartial: true,
	}

	if transferOnly {
		rc.HiddenFields = []string{"type", "physical_quantity"}
	}

	if values == nil {
		values = map[string]interface{}{
			"type": string(MovementEntry),
		}

		if transferOnly {
			values["type"] = string(MovementTransfer)
		}
	}

	requestID, _ := values["request_id"].(string)

	if _, valid := movementReceiptID(requestID); !valid {
		var err error
		requestID, err = newMovementRequestID()
		if err != nil {
			data.FormContent = movementFormMessage(
				t("movement.error.save_failed"),
			)
			_ = h.page.RenderPartial(w, MovementFormModal(data, t))
			return
		}
		values["request_id"] = requestID
	}

	data.RequestID = requestID

	data.CanSubmit = true
	data.Message = message
	data.FormContent = formEng.Render(
		ctx,
		resolveSchema(ctx, "movement", "create", movementschemas.Schema("create")),
		values,
		fieldErrors,
		rc,
	)

	_ = h.page.RenderPartial(w, MovementFormModal(data, t))
}

type MovementFormData struct {
	WarehouseID   int64
	WarehouseName string
	FormContent   mi.H
	CanSubmit     bool
	Message       string
	RequestID     string
	TransferOnly  bool
}

func MovementFormModal(
	data MovementFormData,
	t func(key string, args ...any) string,
) mi.H {
	return func(b *mi.Builder) mi.Node {
		children := []interface{}{
			mi.Class("space-y-4"),
			b.P(
				mi.Class("text-gray-900 dark:text-white"),
				t("movement.warehouse_label"),
				" ",
				b.Strong(data.WarehouseName),
			),
		}

		if data.Message != "" {
			children = append(children,
				b.P(
					mi.Class("text-sm text-gray-700 dark:text-gray-300"),
					data.Message,
				),
			)
		}

		content := data.FormContent(b)

		if data.CanSubmit {
			action := fmt.Sprintf(
				"/warehouses/%d/movements",
				data.WarehouseID,
			)
			movementTypeInput := b.Span()

			if data.TransferOnly {
				action = fmt.Sprintf("/warehouses/%d/empty", data.WarehouseID)

				movementTypeInput = b.Input(mi.Type("hidden"), mi.Name("type"), mi.ID("field-type"), mi.Value(string(MovementTransfer)))
			}
			content = b.Form(
				mi.ID("movement-form"),
				mi.Class("space-y-4"),
				mi.Method("post"),
				mi.Action(action),
				mi.Attr("hx-post", action),
				mi.Attr("hx-target", "#seam-modal-body"),
				mi.Attr("hx-swap", "innerHTML"),
				mi.Attr("hx-disabled-elt", "#movement-submit"),
				b.Input(
					mi.Type("hidden"),
					mi.Name("request_id"),
					mi.Value(data.RequestID),
				),
				movementTypeInput,
				content,
			)
		}

		children = append(children, content)

		footer := []interface{}{
			mi.Attr("data-modal-footer", ""),
			mi.Class("flex justify-end gap-3 w-full"),
			b.Button(
				mi.Type("button"),
				mi.Class(
					"px-4 py-2 rounded-lg bg-indigo-600 "+
						"text-white hover:bg-indigo-700",
				),
				mi.Attr("onclick", "SeamModal.close()"),
				t("action.close"),
			),
		}

		if data.CanSubmit {
			footer = append(footer,
				b.Button(
					mi.ID("movement-submit"),
					mi.Type("submit"),
					mi.Attr("form", "movement-form"),
					mi.Class(
						"px-4 py-2 rounded-lg bg-indigo-600 "+
							"text-white hover:bg-indigo-700",
					),
					t("movement.action.save"),
				),
			)
		}

		children = append(children, b.Div(footer...))

		return b.Div(children...)
	}
}

type movementFormSources map[string][]fe.Option

func (s movementFormSources) Resolve(
	_ context.Context,
	source string,
) ([]fe.Option, error) {
	return s[source], nil
}

func (h *UIHandler) loadMovementFormSources(
	ctx context.Context,
	warehouseID int64,
	t func(key string, args ...any) string,
) (movementFormSources, error) {
	sources := make(movementFormSources)

	configs := []struct {
		source string
		query  string
	}{
		{
			source: "entity:warehouse",
			query: fmt.Sprintf(
				"SELECT id, name, enabled FROM warehouse "+
					"WHERE enabled = true AND id != %d ORDER BY name",
				warehouseID,
			),
		},
		{
			source: "entity:consumable",
			query: "SELECT id, name, enabled FROM consumable " +
				"WHERE enabled = true ORDER BY name",
		},
	}

	for _, config := range configs {
		options := []fe.Option{
			{
				Value: "",
				Label: t("movement.select.placeholder"),
			},
		}

		result, err := h.xoluClient.OQL(ctx, config.query)
		if err != nil {
			if !entityNotYetCreated(err) {
				return nil, err
			}
		} else {
			for _, row := range result.Data {
				id := toInt64(row["id"])
				if id <= 0 || !toBool(row["enabled"]) {
					continue
				}

				if config.source == "entity:warehouse" &&
					id == warehouseID {
					continue
				}

				options = append(options, fe.Option{
					Value: strconv.FormatInt(id, 10),
					Label: toString(row["name"]),
				})
			}
		}

		if len(options) == 1 {
			options[0].Label = t("movement.select.no_options")
		}

		sources[config.source] = options
	}

	return sources, nil
}

func movementFormMessage(message string) mi.H {
	return func(b *mi.Builder) mi.Node {
		return b.P(
			mi.Class("text-sm text-red-600 dark:text-red-400"),
			message,
		)
	}
}

func (h *UIHandler) movementActor(
	w http.ResponseWriter,
	r *http.Request,
) (int64, bool) {
	t := h.translateFunc(r)

	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims.UserID <= 0 {
		http.Error(w, t("error.unauthorized"), http.StatusUnauthorized)
		return 0, false
	}

	if !claims.HasRole(auth.RoleAdmin) {
		http.Error(w, t("error.forbidden"), http.StatusForbidden)
		return 0, false
	}

	return claims.UserID, true
}

func parseMovementQuantity(raw string) (int64, bool) {
	text := strings.TrimSpace(raw)
	parts := strings.Split(text, ".")

	if len(parts) > 2 || len(parts[0]) == 0 || len(parts[0]) > 10 {
		return 0, false
	}

	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
		if len(fraction) == 0 || len(fraction) > 2 {
			return 0, false
		}
	}

	for _, part := range parts {
		for _, digit := range part {
			if digit < '0' || digit > '9' {
				return 0, false
			}
		}
	}

	digits := parts[0] + fraction + strings.Repeat("0", 2-len(fraction))
	quantity, err := strconv.ParseInt(digits, 10, 64)

	return quantity, err == nil
}

func (h *UIHandler) WarehouseMovementCreate(
	w http.ResponseWriter,
	r *http.Request,
) {
	t := h.translateFunc(r)
	ctx := r.Context()

	actorID, ok := h.movementActor(w, r)
	if !ok {
		return
	}

	warehouseID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || warehouseID <= 0 {
		http.Error(
			w,
			t("movement.error.invalid_warehouse"),
			http.StatusBadRequest,
		)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(
			w,
			t("error.invalid_form_data"),
			http.StatusBadRequest,
		)
		return
	}

	movementType := MovementType(r.PostForm.Get("type"))
	if r.PathValue("movement_mode") == "transfer-only" &&
		movementType != MovementTransfer {
		http.Error(w, t("movement.error.invalid_type"), http.StatusBadRequest)
		return
	}
	consumableText := r.PostForm.Get("consumable_id")
	targetText := r.PostForm.Get("target_warehouse_id")
	quantityText := strings.TrimSpace(r.PostForm.Get("quantity"))
	physicalText := strings.TrimSpace(r.PostForm.Get("physical_quantity"))
	reason := strings.TrimSpace(r.PostForm.Get("reason"))
	reference := strings.TrimSpace(r.PostForm.Get("reference"))

	values := map[string]interface{}{
		"type":                string(movementType),
		"consumable_id":       consumableText,
		"target_warehouse_id": targetText,
		"quantity":            quantityText,
		"physical_quantity":   physicalText,
		"reason":              reason,
		"reference":           reference,
		"request_id":          r.PostForm.Get("request_id"),
	}

	fieldErrors := make(map[string]string)

	switch movementType {
	case MovementEntry, MovementExit, MovementTransfer, MovementAdjust:
	default:
		fieldErrors["type"] = t("movement.error.invalid_type")
	}

	if reason == "" {
		fieldErrors["reason"] = t("error.required")
	}

	quantityField := "quantity"
	quantityInput := quantityText
	quantityError := "movement.error.invalid_quantity"

	if movementType == MovementAdjust {
		quantityField = "physical_quantity"
		quantityInput = physicalText
		quantityError = "movement.error.invalid_physical_quantity"
	}

	quantity, validQuantity := parseMovementQuantity(quantityInput)
	if !validQuantity ||
		(movementType != MovementAdjust && quantity == 0) {
		fieldErrors[quantityField] = t(quantityError)
	}

	consumableID, err := strconv.ParseInt(consumableText, 10, 64)
	if err != nil || consumableID <= 0 {
		fieldErrors["consumable_id"] = t(
			"movement.error.invalid_consumable",
		)
	} else {
		consumable, err := h.movementGetFresh(
			ctx,
			"consumable",
			consumableID,
		)
		if err != nil {
			fieldErrors["consumable_id"] = t(
				"movement.error.consumable_load_failed",
			)
		} else if !toBool(consumable.Data["enabled"]) {
			fieldErrors["consumable_id"] = t(
				"movement.error.consumable_disabled",
			)
		}
	}

	if movementType == MovementTransfer {
		targetID, err := strconv.ParseInt(targetText, 10, 64)

		switch {
		case err != nil || targetID <= 0:
			fieldErrors["target_warehouse_id"] = t(
				"movement.error.invalid_target",
			)

		case targetID == warehouseID:
			fieldErrors["target_warehouse_id"] = t(
				"movement.error.same_warehouse",
			)

		default:
			target, err := h.movementGetFresh(ctx, "warehouse", targetID)
			if err != nil {
				fieldErrors["target_warehouse_id"] = t(
					"movement.error.warehouse_load_failed",
				)
			} else if !toBool(target.Data["enabled"]) {
				fieldErrors["target_warehouse_id"] = t(
					"movement.error.target_disabled",
				)
			}
		}
	}

	if len(fieldErrors) > 0 {
		h.renderMovementForm(
			w,
			r,
			warehouseID,
			values,
			fieldErrors,
			"",
		)
		return
	}

	req := MovementRequest{
		RequestID:         r.PostForm.Get("request_id"),
		ResponsibleUserID: actorID,
		Type:              movementType,
		ConsumableID:      consumableID,
		Quantity:          quantity,
		Reason:            reason,
		Reference:         reference,
	}

	switch movementType {
	case MovementEntry, MovementAdjust:
		req.TargetWarehouseID = warehouseID

	case MovementExit:
		req.OriginWarehouseID = warehouseID

	case MovementTransfer:
		targetID, _ := strconv.ParseInt(targetText, 10, 64)
		req.OriginWarehouseID = warehouseID
		req.TargetWarehouseID = targetID
	}

	if err := h.saveMovementAtomic(ctx, req); err != nil {
		key := "movement.error.save_failed"
		field := ""

		var saveError *movementSaveError
		if errors.As(err, &saveError) {
			key = saveError.Key
			field = saveError.Field
		}

		h.logger.Error(
			t(key),
			"warehouse_id", warehouseID,
			"error", err,
		)

		message := ""
		if field != "" {
			fieldErrors[field] = t(key)
		} else {
			message = t(key)
		}

		h.renderMovementForm(
			w,
			r,
			warehouseID,
			values,
			fieldErrors,
			message,
		)
		return
	}

	HTMXRedirect(
		w,
		fmt.Sprintf("/warehouse/%d", warehouseID),
	)
}

const movementMaxUnits int64 = 999999999999

type movementSaveError struct {
	Field string
	Key   string
	Cause error
}

func (e *movementSaveError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Key, e.Cause)
	}
	return e.Key
}

func (e *movementSaveError) Unwrap() error {
	return e.Cause
}

func movementFailure(field, key string, cause error) error {
	return &movementSaveError{
		Field: field,
		Key:   key,
		Cause: cause,
	}
}

func newMovementRequestID() (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", data), nil
}

func movementReceiptID(requestID string) (int64, bool) {
	if len(requestID) != 32 {
		return 0, false
	}

	for _, c := range requestID {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return 0, false
		}
	}

	id, err := strconv.ParseInt(requestID[:13], 16, 64)
	return id + 1, err == nil
}

func movementStoredUnits(stock float64) (int64, error) {
	if math.IsNaN(stock) || math.IsInf(stock, 0) || stock < 0 {
		return 0, ErrInvalidMovement
	}

	scaled := stock * 100
	rounded := math.Round(scaled)

	if rounded > float64(movementMaxUnits) ||
		math.Abs(scaled-rounded) > 0.001 {
		return 0, ErrInvalidMovement
	}

	return int64(rounded), nil
}

func movementVersion(data map[string]any) (int, error) {
	version := toInt64(data["_version"])
	if version <= 0 || int64(int(version)) != version {
		return 0, ErrInvalidMovement
	}
	return int(version), nil
}

const movementStockField = "stock_by_warehouse"

func decodeMovementStock(raw any) (map[int64]int64, error) {
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}

	var stored map[int64]*int64
	if err := json.Unmarshal(encoded, &stored); err != nil {
		return nil, err
	}
	if stored == nil {
		return nil, ErrInvalidMovement
	}

	stock := make(map[int64]int64, len(stored))
	for warehouseID, units := range stored {
		if warehouseID <= 0 || units == nil ||
			*units < 0 || *units > movementMaxUnits {
			return nil, ErrInvalidMovement
		}
		stock[warehouseID] = *units
	}
	return stock, nil
}

func legacyWarehouseStock(raw any) (map[int64]int64, error) {
	stock := make(map[int64]int64)
	if raw == nil {
		return stock, nil
	}

	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}

	var balances []WarehouseConsumableBalance
	if err := json.Unmarshal(encoded, &balances); err != nil {
		return nil, err
	}

	for _, balance := range balances {
		if _, duplicate := stock[balance.ConsumableID]; duplicate ||
			balance.ConsumableID <= 0 {
			return nil, ErrInvalidMovement
		}
		units, err := movementStoredUnits(balance.Stock)
		if err != nil {
			return nil, err
		}
		stock[balance.ConsumableID] = units
	}
	return stock, nil
}

func (h *UIHandler) inventoryByConsumable(
	ctx context.Context,
	rows []map[string]any,
) (map[int64]map[int64]int64, error) {
	inventory := make(map[int64]map[int64]int64, len(rows))
	legacy := make(map[int64]map[int64]int64)

	for _, row := range rows {
		id := toInt64(row["id"])
		if id <= 0 {
			return nil, ErrInvalidMovement
		}

		raw, migrated := row[movementStockField]
		if !migrated {
			stock := make(map[int64]int64)
			inventory[id] = stock
			legacy[id] = stock
			continue
		}

		stock, err := decodeMovementStock(raw)
		if err != nil {
			return nil, err
		}
		inventory[id] = stock
	}

	if len(legacy) == 0 {
		return inventory, nil
	}

	result, err := h.xoluClient.OQL(ctx, "SELECT * FROM warehouse")
	if err != nil {
		if entityNotYetCreated(err) {
			return inventory, nil
		}
		return nil, err
	}

	for _, row := range result.Data {
		warehouseID := toInt64(row["id"])
		if warehouseID <= 0 {
			return nil, ErrInvalidMovement
		}

		stock, err := legacyWarehouseStock(row["consumables"])
		if err != nil {
			return nil, err
		}
		for consumableID, units := range stock {
			if balances, ok := legacy[consumableID]; ok {
				balances[warehouseID] = units
			}
		}
	}
	return inventory, nil
}

func (h *UIHandler) inventoryForWarehouse(
	ctx context.Context,
	warehouse *xolu.Entity,
) ([]WarehouseConsumableStock, error) {
	stock, err := legacyWarehouseStock(warehouse.Data["consumables"])
	if err != nil {
		return nil, err
	}

	result, err := h.xoluClient.OQL(ctx, "SELECT * FROM consumable")
	if err != nil && !entityNotYetCreated(err) {
		return nil, err
	}
	if result != nil {
		for _, row := range result.Data {
			raw, migrated := row[movementStockField]
			if !migrated {
				continue
			}

			consumableID := toInt64(row["id"])
			if consumableID <= 0 {
				return nil, ErrInvalidMovement
			}
			balances, err := decodeMovementStock(raw)
			if err != nil {
				return nil, err
			}

			delete(stock, consumableID)
			if units, exists := balances[warehouse.ID]; exists {
				stock[consumableID] = units
			}
		}
	}

	balances := make([]WarehouseConsumableBalance, 0, len(stock))
	for consumableID, units := range stock {
		balances = append(balances, WarehouseConsumableBalance{
			ConsumableID: consumableID,
			Stock:        float64(units) / 100,
		})
	}
	return h.warehouseConsumableStocks(ctx, balances)
}

type movementWarehouseState struct {
	ID    int64
	Stock int64
}

func (h *UIHandler) loadMovementWarehouse(
	ctx context.Context,
	warehouseID int64,
	stock map[int64]int64,
	field string,
) (*movementWarehouseState, error) {
	entity, err := h.movementGetFresh(ctx, "warehouse", warehouseID)
	if err != nil {
		return nil, movementFailure(field, "movement.error.warehouse_load_failed", err)
	}

	if !toBool(entity.Data["enabled"]) {
		key := "movement.error.warehouse_disabled"
		if field == "target_warehouse_id" {
			key = "movement.error.target_disabled"
		}
		return nil, movementFailure(field, key, nil)
	}

	return &movementWarehouseState{ID: warehouseID, Stock: stock[warehouseID]}, nil
}

func (h *UIHandler) movementReceipt(
	ctx context.Context,
	receiptID int64,
	requestID, fingerprint string,
) (bool, error) {
	entity, err := h.movementGetFresh(
		ctx,
		"movement_request",
		receiptID,
	)

	if err != nil {
		if errors.Is(err, ErrEntityNotFound) {
			return false, nil
		}
		return false, err
	}

	if toString(entity.Data["request_id"]) != requestID ||
		toString(entity.Data["fingerprint"]) != fingerprint {
		return false, movementFailure(
			"",
			"movement.error.request_reused",
			nil,
		)
	}

	return true, nil
}

func (h *UIHandler) saveMovementAtomic(
	ctx context.Context,
	req MovementRequest,
) error {
	req.Reason = strings.TrimSpace(req.Reason)
	req.Reference = strings.TrimSpace(req.Reference)

	inventoryMutationMu.Lock()
	defer inventoryMutationMu.Unlock()

	if err := ValidateMovement(req); err != nil {
		return movementFailure(
			"",
			"movement.error.invalid_request",
			err,
		)
	}

	if req.Quantity > movementMaxUnits {
		return movementFailure(
			"",
			"movement.error.stock_overflow",
			nil,
		)
	}

	receiptID, validRequestID := movementReceiptID(req.RequestID)
	if !validRequestID {
		return movementFailure(
			"",
			"movement.error.invalid_request",
			nil,
		)
	}

	rawRequest, err := json.Marshal(req)
	if err != nil {
		return movementFailure(
			"",
			"movement.error.save_failed",
			err,
		)
	}

	fingerprint := fmt.Sprintf("%x", sha256.Sum256(rawRequest))

	saved, err := h.movementReceipt(
		ctx,
		receiptID,
		req.RequestID,
		fingerprint,
	)
	if err != nil || saved {
		return err
	}

	consumable, err := h.movementGetFresh(
		ctx,
		"consumable",
		req.ConsumableID,
	)
	if err != nil {
		return movementFailure(
			"consumable_id",
			"movement.error.consumable_load_failed",
			err,
		)
	}

	if !toBool(consumable.Data["enabled"]) {
		return movementFailure(
			"consumable_id",
			"movement.error.consumable_disabled",
			nil,
		)
	}

	consumableVersion, err := movementVersion(consumable.Data)
	if err != nil {
		return movementFailure(
			"",
			"movement.error.save_failed",
			err,
		)
	}

	inventory, err := h.inventoryByConsumable(ctx, []map[string]any{consumable.Data})
	if err != nil {
		return movementFailure("", "movement.error.invalid_stock", err)
	}
	stock := inventory[req.ConsumableID]

	warehouseID := req.TargetWarehouseID
	if req.Type == MovementExit || req.Type == MovementTransfer {
		warehouseID = req.OriginWarehouseID
	}

	primary, err := h.loadMovementWarehouse(
		ctx,
		warehouseID,
		stock,
		"",
	)
	if err != nil {
		return err
	}

	states := []*movementWarehouseState{primary}
	after := []int64{primary.Stock}

	switch req.Type {
	case MovementEntry:
		after[0] += req.Quantity

	case MovementExit:
		if primary.Stock < req.Quantity {
			return movementFailure(
				"quantity",
				"movement.error.insufficient_stock",
				ErrInsufficientStock,
			)
		}
		after[0] -= req.Quantity

	case MovementTransfer:
		if primary.Stock < req.Quantity {
			return movementFailure(
				"quantity",
				"movement.error.insufficient_stock",
				ErrInsufficientStock,
			)
		}

		target, err := h.loadMovementWarehouse(
			ctx,
			req.TargetWarehouseID,
			stock,
			"target_warehouse_id",
		)
		if err != nil {
			return err
		}

		states = append(states, target)
		after[0] -= req.Quantity
		after = append(after, target.Stock+req.Quantity)

	case MovementAdjust:
		after[0] = req.Quantity
	}

	for _, stock := range after {
		if stock < 0 || stock > movementMaxUnits {
			return movementFailure(
				"",
				"movement.error.stock_overflow",
				nil,
			)
		}
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)

	movementData := map[string]interface{}{
		"request_id":    req.RequestID,
		"consumable_id": req.ConsumableID,
		"type":          string(req.Type),
		"quantity":      float64(req.Quantity) / 100,
		"delta":         float64(after[0]-primary.Stock) / 100,
		"stock_before":  float64(primary.Stock) / 100,
		"stock_after":   float64(after[0]) / 100,
		"user_id":       req.ResponsibleUserID,
		"reason":        req.Reason,
		"reference":     req.Reference,
		"created_at":    now,
	}

	if req.OriginWarehouseID > 0 {
		movementData["origin_warehouse_id"] = req.OriginWarehouseID
	}
	if req.TargetWarehouseID > 0 {
		movementData["target_warehouse_id"] = req.TargetWarehouseID
	}
	if len(states) == 2 {
		movementData["target_stock_before"] =
			float64(states[1].Stock) / 100
		movementData["target_stock_after"] =
			float64(after[1]) / 100
	}

	for i, state := range states {
		stock[state.ID] = after[i]
	}

	data := make(map[string]any, len(consumable.Data)+1)
	for key, value := range consumable.Data {
		if key != "_version" {
			data[key] = value
		}
	}
	data[movementStockField] = stock

	_, saveErr := h.xoluClient.Commit(ctx, xolu.CommitRequest{
		Update: xolu.CommitUpdate{
			Entity:  "consumable",
			ID:      req.ConsumableID,
			Version: &consumableVersion,
			Data:    data,
		},
		Append: []xolu.CommitAppend{
			{Entity: "movement", Data: movementData},
			{
				Entity: "movement_request",
				ID:     &receiptID,
				Data: map[string]any{
					"request_id":  req.RequestID,
					"fingerprint": fingerprint,
					"user_id":     req.ResponsibleUserID,
					"created_at":  now,
				},
			},
		},
	})
	if saveErr == nil {
		return nil
	}

	checkCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx),
		3*time.Second,
	)
	defer cancel()

	saved, receiptErr := h.movementReceipt(
		checkCtx,
		receiptID,
		req.RequestID,
		fingerprint,
	)
	if saved {
		return nil
	}
	if receiptErr != nil {
		var knownError *movementSaveError
		if errors.As(receiptErr, &knownError) {
			return receiptErr
		}
		return movementFailure(
			"",
			"movement.error.save_unknown",
			receiptErr,
		)
	}

	var apiError *xolu.Error
	if errors.As(saveErr, &apiError) {
		switch apiError.HTTPStatus {
		case http.StatusConflict:
			return movementFailure("", "movement.error.save_rejected", saveErr)
		case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden:
			return movementFailure("", "movement.error.save_failed", saveErr)
		}
	}

	return movementFailure(
		"",
		"movement.error.save_unknown",
		saveErr,
	)
}

func (h *UIHandler) movementGetFresh(
	ctx context.Context,
	entityType string,
	id int64,
) (*xolu.Entity, error) {
	switch entityType {
	case "warehouse", "consumable", "movement_request":
	default:
		return nil, ErrInvalidMovement
	}

	if id <= 0 {
		return nil, ErrEntityNotFound
	}

	query := fmt.Sprintf(
		"SELECT TOP 1 * FROM %s WHERE id = %d",
		entityType,
		id,
	)

	result, err := h.xoluClient.OQL(ctx, query)
	if err != nil {
		if entityNotYetCreated(err) {
			return nil, ErrEntityNotFound
		}
		return nil, err
	}

	if result == nil || len(result.Data) == 0 {
		return nil, ErrEntityNotFound
	}

	data := result.Data[0]
	if toInt64(data["id"]) != id {
		return nil, ErrEntityNotFound
	}

	return &xolu.Entity{
		ID:   id,
		Data: data,
	}, nil
}

// WarehouseEmptyNew opens the transfer-only form used before disabling.
func (h *UIHandler) WarehouseEmptyNew(w http.ResponseWriter, r *http.Request) {
	r.SetPathValue("movement_mode", "transfer-only")
	h.WarehouseMovementNew(w, r)
}

// WarehouseEmptyCreate submits a transfer through the existing movement flow.
func (h *UIHandler) WarehouseEmptyCreate(w http.ResponseWriter, r *http.Request) {
	r.SetPathValue("movement_mode", "transfer-only")
	h.WarehouseMovementCreate(w, r)
}
