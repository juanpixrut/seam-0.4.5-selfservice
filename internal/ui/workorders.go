package ui

// Work orders are generated from the selected template's creation and
// execution fields. Orders retain template ID/version and store header
// answers and block answers separately. Template versions must remain
// immutable; editing a template version in place cannot preserve history.
// New template orders follow their ordered workflow_states directly. The
// draft/in_progress/completed FSM is retained for legacy orders. Template
// actions, business rules, numbering and consumable accounting are separate
// integrations and are not executed by this module.
import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"

	"sort"
	"strconv"
	"strings"
	"time"

	mi "github.com/ha1tch/minty"
	qf "github.com/ha1tch/queryfy"
	"github.com/ha1tch/seam-ui/internal/auth"
	fe "github.com/ha1tch/seam-ui/internal/formengine"
	workorderschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/workorders"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

// workOrderDefName identifies this playground's one FSM definition.
// Prefixed "seam_" to make it easy to spot alongside any other
// definitions a shared xolu instance might carry.
const workOrderDefName = "seam_work_order_lifecycle"

// ensureWorkOrderDefinition idempotently ensures the playground's FSM
// definition exists, creating it on first use. Called lazily from
// WorkOrderNew/WorkOrderCreate, not at application startup -- a
// playground feature gated behind EnableWorkOrders shouldn't add a
// startup-time dependency on xolu being reachable when it might not
// even be turned on. ListMachineDefs + a name match is the whole
// bootstrap; FSM definitions are immutable once created (confirmed
// directly from xolu's own docs), so there's no update path to
// consider here, only create-if-missing.
func (h *UIHandler) ensureWorkOrderDefinition(ctx context.Context) (int64, error) {
	defs, err := h.xoluClient.ListMachineDefs(ctx)
	if err != nil {
		return 0, err
	}
	for _, d := range defs {
		if d.Name == workOrderDefName {
			return d.ID, nil
		}
	}

	// CreateMachineDef has existed in xolu since v0.26.0 -- our own
	// letter (docs/XOLU_CLIENT_FEATURE_REQUEST.md, T-69) had checked
	// v0.23.0 and v0.25.0 specifically, which is exactly why it looked
	// missing; the xolu team confirmed this in their 2026-08-04
	// response rather than us catching it ourselves. Adopted here in
	// v0.27.0, now on v0.27.1 (client-side empty-Name/Initial rejection
	// added, our own local patch's one genuine improvement, pulled in
	// upstream). Takes MachineSpec directly, not the provisional
	// CreateMachineDefRequest wrapper type this session's own local
	// patch briefly used before the swap -- same field shape either
	// way, only the type name changed.
	result, err := h.xoluClient.CreateMachineDef(ctx, xolu.MachineSpec{
		Name:        workOrderDefName,
		Description: "Seam AMS Work Orders playground -- minimal lifecycle, not T-11's full Order Engine scope",
		Initial:     "draft",
		Determinism: "firstmatch",
		States: map[string]xolu.StateDef{
			"draft":       {Terminal: false},
			"in_progress": {Terminal: false},
			"completed":   {Terminal: true},
		},
		Transitions: []xolu.TransitionDef{
			{From: json.RawMessage(`"draft"`), Input: "start", To: "in_progress"},
			{
				From:  json.RawMessage(`"in_progress"`),
				Input: "complete",
				To:    "completed",
				// Single-quoted string literal -- T-SQL style, matching
				// every real example in xolu's own docs. A double-quoted
				// empty string here silently fails to match ANY payload
				// value, found the hard way during this session's own
				// proof-of-concept (a real, live guard-rejected-a-valid-
				// signature bug, not a hypothetical caution). The xolu
				// team confirmed the mechanism (2026-08-04 response to
				// docs/XOLU_CLIENT_FEATURE_REQUEST.md §4): correct T-SQL
				// lexer behaviour, not a bug -- a double-quoted token is
				// a quoted identifier, not a string literal, exactly like
				// ANSI SQL. They're weighing a validation warning for
				// this specific mix-up, not yet committed as of v0.27.1.
				Guard: "payload.technician_signature != ''",
			},
		},
	})
	if err != nil {
		return 0, err
	}
	return result.ID, nil
}

const workOrderBoardPosting = "board_posting"

// WorkOrder stores a template workflow position for new instances and keeps
// machine IDs for compatibility with existing FSM-backed orders.
type WorkOrder struct {
	ID                  int64
	Title               string
	Description         string
	AssignedTo          string
	AssignmentType      string
	ResponsibleType     string
	ResponsibleID       int64
	ClaimedByUserID     int64
	State               string
	MachineID           int64
	TechnicianSignature string
	CreatedAt           string
	TemplateID          int64
	TemplateVersion     int
	HeaderValues        map[string]any
	BlockValues         map[string]any
	WorkflowMode        string
	WorkflowIndex       int
	MobileWarehouseID   int64
}

// workOrderFromEntity converts a Xolu record into a WorkOrder, including
// stored answers and the state data used by template and legacy orders.
func workOrderFromEntity(e *xolu.Entity) WorkOrder {
	headerValues, _ := e.Data["header_values"].(map[string]any)
	blockValues, _ := e.Data["block_values"].(map[string]any)
	return WorkOrder{
		ID:                  e.ID,
		Title:               toString(e.Data["title"]),
		Description:         toString(e.Data["description"]),
		AssignedTo:          toString(e.Data["assigned_to"]),
		AssignmentType:      toString(e.Data["assignment_type"]),
		ResponsibleType:     toString(e.Data["responsible_type"]),
		ResponsibleID:       toInt64(e.Data["responsible_id"]),
		ClaimedByUserID:     toInt64(e.Data["claimed_by_user_id"]),
		State:               toString(e.Data["state"]),
		MachineID:           toInt64(e.Data["machine_id"]),
		TechnicianSignature: toString(e.Data["technician_signature"]),
		CreatedAt:           toString(e.Data["created_at"]),
		TemplateID:          toInt64(e.Data["template_id"]),
		TemplateVersion:     toInt(e.Data["template_version"]),
		HeaderValues:        headerValues,
		BlockValues:         blockValues,
		WorkflowMode:        toString(e.Data["workflow_mode"]),
		WorkflowIndex:       toInt(e.Data["workflow_index"]),
		MobileWarehouseID:   toInt64(e.Data["mobile_warehouse_id"]),
	}
}

// WorkOrdersList handles GET /workorders.
func (h *UIHandler) WorkOrdersList(w http.ResponseWriter, r *http.Request) {
	h.listWorkOrders(w, r, false)
}

func (h *UIHandler) WorkOrdersBacklog(w http.ResponseWriter, r *http.Request) {
	h.listWorkOrders(w, r, true)
}

func (h *UIHandler) listWorkOrders(w http.ResponseWriter, r *http.Request, backlog bool) {
	t := h.translateFunc(r)
	ctx := r.Context()

	query := "SELECT TOP 100 id, title, description, assigned_to, assignment_type, responsible_type, responsible_id, state, created_at, template_id, template_version, workflow_mode, mobile_warehouse_id FROM work_orders"
	if backlog {
		query += " WHERE assignment_type = 'board_posting' AND responsible_id = 0"
	}
	result, err := h.xoluClient.OQL(ctx, query+" ORDER BY created_at DESC, id DESC")
	var orders []WorkOrder
	templateNames := make(map[int64]string)
	var templateIDs []string
	if err == nil {
		for _, row := range result.Data {
			order := workOrderFromEntity(&xolu.Entity{ID: toInt64(row["id"]), Data: row})
			orders = append(orders, order)
			if _, seen := templateNames[order.TemplateID]; order.TemplateID > 0 && !seen {
				templateNames[order.TemplateID] = ""
				templateIDs = append(templateIDs, strconv.FormatInt(order.TemplateID, 10))
			}
		}
	} else if !entityNotYetCreated(err) {
		h.logger.Error("failed to list work orders", "error", err)
		if backlog {
			h.page.RenderError(w, http.StatusInternalServerError, t("workorders.backlog.title"), t("error.server_error"))
			return
		}
	}

	if len(templateIDs) > 0 {
		query := "SELECT id, name FROM work_order_templates WHERE id IN (" + strings.Join(templateIDs, ",") + ")"
		result, err := h.xoluClient.OQL(ctx, query)
		if err == nil {
			for _, row := range result.Data {
				templateNames[toInt64(row["id"])] = toString(row["name"])
			}
		} else {
			h.logger.Error("failed to load work order template names", "error", err)
		}
	}

	data := WorkOrdersListData{Orders: orders, TemplateNames: templateNames, Backlog: backlog}
	title := t("workorders.list_title")
	if backlog {
		title = t("workorders.backlog.title")
	}
	h.page.Render(w, r, title, "/workorders", WorkOrdersListPage(ctx, data, t))
}

// WorkOrderTemplatePicker handles GET /workorders/template-picker.
// It renders the template picker schema and returns the modal content
// as an HTML partial.
func (h *UIHandler) WorkOrderTemplatePicker(w http.ResponseWriter, r *http.Request) {
	formEng := &fe.FormEngine{Translator: h.formEngineTranslator(), Sources: &AssetSourceResolver{Client: h.xoluClient}}
	rc := fe.RenderContext{
		Locale:    h.currentLocale(),
		Module:    "workorders",
		Form:      "template-picker",
		IsPartial: true,
	}
	ctx := r.Context()
	schema := resolveSchema(ctx, "workorders", "template-picker", workorderschemas.Schema("template-picker"))
	content := formEng.Render(ctx, schema, nil, nil, rc)

	h.page.RenderPartial(w, WorkOrderTemplatePickerPage(content, h.translateFunc(r)))
}

// WorkOrderNew handles GET /workorders/new. When a template ID is
// provided, it loads the template and uses its description to prefill
// the create form.
func (h *UIHandler) WorkOrderNew(w http.ResponseWriter, r *http.Request) {
	var template WorkOrderTemplate
	values := map[string]any{}

	query := r.URL.Query()
	if query.Has("template_id") {
		var ok bool
		template, ok = h.loadWorkOrderTemplate(w, r, query.Get("template_id"))
		if !ok {
			return
		}
		values["description"] = template.Description
		values["title"] = template.Name
	}

	h.renderWorkOrderNew(w, r, template, values, nil)
}

// loadWorkOrderTemplate validates the template ID, fetches the record
// from xolu, and decodes it. On failure, it writes an HTTP error
// response and returns false.
func (h *UIHandler) loadWorkOrderTemplate(w http.ResponseWriter, r *http.Request, rawID string) (WorkOrderTemplate, bool) {
	t := h.translateFunc(r)
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 {
		h.page.RenderError(w, http.StatusBadRequest,
			t("workorders.new"), t("core.errors.invalid_pattern"))
		return WorkOrderTemplate{}, false
	}

	entity, err := h.xoluClient.Get(r.Context(), "work_order_templates", id)
	if err != nil {
		var xerr *xolu.Error
		if errors.As(err, &xerr) && xerr.HTTPStatus == http.StatusNotFound {
			h.page.RenderError(w, http.StatusNotFound,
				t("workorders.new"), t("error.not_found"))
		} else {
			h.logger.Error("failed to load work order template", "id", id, "error", err)
			h.page.RenderError(w, http.StatusInternalServerError,
				t("workorders.new"), t("error.server_error"))
		}
		return WorkOrderTemplate{}, false
	}

	template, err := workOrderTemplateFromEntity(entity)
	if err != nil {
		h.logger.Error("failed to decode work order template", "id", id, "error", err)
		h.page.RenderError(w, http.StatusInternalServerError,
			t("workorders.new"), t("error.server_error"))
		return WorkOrderTemplate{}, false
	}
	return template, true
}

// workOrderHeaderFieldName returns the form field name for a template
// header item by index. Rendering and submission use the same name
// to match a posted value to its field.
func workOrderHeaderFieldName(index int) string {
	return "template_header_" + strconv.Itoa(index)
}

// Inline styling keeps the template identity visible without requiring a
// stylesheet rebuild. Only a valid hex color from the template is accepted.
func workOrderTemplateBanner(template WorkOrderTemplate, t func(string, ...any) string) mi.H {
	color := strings.TrimSpace(template.Color)
	valid := (len(color) == 4 || len(color) == 7) && strings.HasPrefix(color, "#")
	if valid {
		for _, c := range color[1:] {
			if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
				valid = false
				break
			}
		}
	}
	if !valid {
		color = "#818cf8"
	}
	return func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("rounded-lg bg-gray-50 dark:bg-gray-900"),
			mi.Attr("style", "padding:20px 24px;border-left:4px solid "+color),
			b.Div(mi.Class("flex flex-wrap items-center gap-3"),
				b.Span(mi.Class("text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400"), t("workorders-templates")),
				Badge(t("workorders.template_version", template.Version), "info")(b),
			),
			b.H2(mi.Class("text-sm font-medium break-words"), mi.Attr("style", "margin-top:8px;font-size:14px;line-height:1.5;color:"+color), template.Name),
		)
	}
}

// workOrderBlockFieldName returns the form field name for a line based on
// its block and line indices. The same name identifies its stored answer.
// Used for matching each execution input to its stored answer.
func workOrderBlockFieldName(blockIndex, lineIndex int) string {
	return fmt.Sprintf("template_block_%d_line_%d", blockIndex, lineIndex)
}

// workOrderFieldSchema maps a template field type to its JSON Schema type
// and FormEngine widget. Unrecognized types use a text input. Used by
// buildWorkOrderSchema for header fields and block lines.
func workOrderFieldSchema(title, fieldType string) map[string]any {
	field := map[string]any{"type": "string", "title": title, "x-seam-input": "text"}
	switch strings.ToLower(strings.TrimSpace(fieldType)) {
	case "number":
		field["type"], field["x-seam-input"] = "number", "number"
	case "boolean":
		field["type"], field["x-seam-input"] = "boolean", "checkbox"
	case "date":
		field["format"], field["x-seam-input"] = "date", "date"
	case "asset":
		field["type"], field["x-seam-input"] = "integer", "any-picker"
		field["x-seam-source"] = "entity:assets"
	case "gps":
		field["x-seam-input"] = "gps"
	}
	return field
}

const (
	workOrderAssetIDs           = "asset_ids"
	workOrderExtraConsumableIDs = "extra_consumable_ids"
)

// workOrderAllowsExtraConsumables reports whether the template enables
// consumables and allows extras. Used by renderWorkOrderNew and
// workOrderSelectionSchema to apply the same rule to display and submission.
func workOrderAllowsExtraConsumables(template WorkOrderTemplate) bool {
	return template.ConsumablesEnabled && template.ExtraConsumables
}

// workOrderSelectionSchema builds the repeatables for assets and extra
// consumables using the selected template's limits and settings.
// Used by renderWorkOrderNew to render the pickers and by
// parseWorkOrderSelections to determine which fields to accept.
func workOrderSelectionSchema(ctx context.Context, template WorkOrderTemplate) ([]byte, error) {
	raw := resolveSchema(ctx, "workorders", "create-selections", workorderschemas.Schema("create-selections"))

	var document struct {
		Properties map[string]map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, err
	}

	assets := document.Properties[workOrderAssetIDs]
	if assets == nil {
		return nil, fmt.Errorf("missing %s in work order selection schema", workOrderAssetIDs)
	}

	// The template owns the bounds. Zero represents an unset limit.
	delete(assets, "minItems")
	delete(assets, "maxItems")
	if template.MinAssets > 0 && template.MinAssets <= fe.MaxRepeatableBound {
		assets["minItems"] = json.RawMessage(strconv.Itoa(template.MinAssets))
	}
	if template.MaxAssets > 0 && template.MaxAssets <= fe.MaxRepeatableBound {
		assets["maxItems"] = json.RawMessage(strconv.Itoa(template.MaxAssets))
	}

	properties := map[string]any{workOrderAssetIDs: assets}
	if workOrderAllowsExtraConsumables(template) {
		extras := document.Properties[workOrderExtraConsumableIDs]
		if extras == nil {
			return nil, fmt.Errorf("missing %s in work order selection schema", workOrderExtraConsumableIDs)
		}
		properties[workOrderExtraConsumableIDs] = extras
	}

	return json.Marshal(map[string]any{
		"type":       "object",
		"properties": properties,
	})
}

// parseWorkOrderSelections validates the posted asset and extra consumable
// IDs and keeps the original rows for re-rendering. Used by WorkOrderCreate
// to check asset counts, asset types, and selected IDs before saving.
func (h *UIHandler) parseWorkOrderSelections(r *http.Request, template WorkOrderTemplate, values map[string]any, errs map[string]string) (map[string][]int64, error) {
	schema, err := workOrderSelectionSchema(r.Context(), template)
	if err != nil {
		return nil, err
	}
	form, err := fe.ParseSchema(schema)
	if err != nil {
		return nil, err
	}

	t := h.translateFunc(r)
	engine := &fe.FormEngine{Translator: h.formEngineTranslator()}
	rc := fe.RenderContext{Locale: h.currentLocale()}
	selections := make(map[string][]int64, len(form.Fields))

	for _, field := range form.Fields {
		isAsset := field.Name == workOrderAssetIDs
		entity, itemName := "consumable", "consumable_id"
		if isAsset {
			entity, itemName = "assets", "asset_id"
		}
		if fe.InferWidget(field) != "repeatable" || len(field.Items) != 1 ||
			field.Items[0].Name != itemName || fe.InferWidget(field.Items[0]) != "any-picker" {
			return nil, fmt.Errorf("invalid work order selection field %s", field.Name)
		}

		// The indexed parser discards rows whose picker is empty.
		rows := fe.ParseIndexedRepeatable(r.PostForm, field)
		values[field.Name] = rows

		if isAsset {
			// Enforce the actual bounds even if they exceed the widget's limit.
			field.MinItems, field.MaxItems = template.MinAssets, template.MaxAssets
			if message := fe.RepeatableBoundsMessage(engine, rc, field, len(rows)); message != "" {
				errs[field.Name] = message
				continue
			}
		}

		ids := make([]int64, 0, len(rows))
		seen := make(map[int64]bool, len(rows))
		for _, item := range rows {
			row := item.(map[string]any)
			id, parseErr := strconv.ParseInt(strings.TrimSpace(toString(row[itemName])), 10, 64)
			if parseErr != nil || id <= 0 {
				errs[field.Name] = t("core.errors.invalid_pattern")
				break
			}
			if seen[id] {
				errs[field.Name] = t("workorders.error.duplicate_selection")
				break
			}

			record, lookupErr := h.xoluClient.Get(r.Context(), entity, id)
			if lookupErr != nil {
				errs[field.Name] = t("workorders.error.selection_unavailable")
				break
			}

			// Keep this check when the picker gains type-filtered search.
			if isAsset && template.AssetTypeID > 0 &&
				toInt64(record.Data["type_id"]) != template.AssetTypeID {
				errs[field.Name] = t("workorders.error.asset_type")
				break
			}

			seen[id] = true
			ids = append(ids, id)
		}
		selections[field.Name] = ids
	}
	return selections, nil
}

// buildWorkOrderSchema adds the template fields for the requested phase
// while preserving the base field order and required fields. Used to render
// and validate the creation and execution forms.
func buildWorkOrderSchema(baseSchema []byte, template WorkOrderTemplate, phase string) ([]byte, error) {
	baseForm, err := fe.ParseSchema(baseSchema)
	if err != nil {
		return nil, err
	}
	order := make([]string, 0, len(baseForm.Fields))
	for _, field := range baseForm.Fields {
		order = append(order, field.Name)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(baseSchema, &document); err != nil {
		return nil, fmt.Errorf("decode base schema: %w", err)
	}

	var properties map[string]json.RawMessage
	if err := json.Unmarshal(document["properties"], &properties); err != nil {
		return nil, fmt.Errorf("decode schema properties: %w", err)
	}
	if properties == nil {
		properties = make(map[string]json.RawMessage)
	}

	var required []string
	if raw, ok := document["required"]; ok {
		if err := json.Unmarshal(raw, &required); err != nil {
			return nil, fmt.Errorf("decode required fields: %w", err)
		}
	}

	for i, item := range template.HeaderItems {
		if item.ToFillOut != phase {
			continue
		}

		name := workOrderHeaderFieldName(i)

		field := workOrderFieldSchema(item.Tag, item.Type)

		raw, err := json.Marshal(field)
		if err != nil {
			return nil, fmt.Errorf("encode header field %s: %w", name, err)
		}
		properties[name] = raw
		order = append(order, name)

		if item.Required {
			required = append(required, name)
		}
	}

	if phase == "upon_executing" {
		for bi, block := range template.Blocks {
			for li, line := range block.Lines {
				raw, err := json.Marshal(workOrderFieldSchema(block.Name+" · "+line.Name, line.Type))
				if err != nil {
					return nil, err
				}
				properties[workOrderBlockFieldName(bi, li)] = raw
				order = append(order, workOrderBlockFieldName(bi, li))
			}
		}
	}
	var ordered strings.Builder
	ordered.WriteByte('{')
	for i, name := range order {
		if i > 0 {
			ordered.WriteByte(',')
		}
		key, _ := json.Marshal(name)
		ordered.Write(key)
		ordered.WriteByte(':')
		ordered.Write(properties[name])
	}
	ordered.WriteByte('}')
	document["properties"] = json.RawMessage(ordered.String())

	rawRequired, err := json.Marshal(required)
	if err != nil {
		return nil, fmt.Errorf("encode required fields: %w", err)
	}
	document["required"] = rawRequired

	return json.Marshal(document)
}

// renderWorkOrderNew builds and renders the creation schema from the
// base form and the selected template's upon_creating header fields.
// Validation errors preserve submitted values.
func (h *UIHandler) renderWorkOrderNew(w http.ResponseWriter, r *http.Request, template WorkOrderTemplate, values map[string]any, errs map[string]string) {
	t := h.translateFunc(r)
	ctx := r.Context()
	csrfToken := h.GetCSRFToken(w, r)

	formEng := &fe.FormEngine{
		Translator: h.formEngineTranslator(),
		Sources:    &AssetSourceResolver{Client: h.xoluClient},
	}
	rc := fe.RenderContext{
		Locale:    h.currentLocale(),
		CSRFToken: csrfToken,
		Module:    "workorders",
		Form:      "workorder",
	}

	baseSchema := resolveSchema(ctx, "workorders", "create", workorderschemas.Schema("create"))

	schema, err := buildWorkOrderSchema(
		baseSchema,
		template,
		"upon_creating",
	)
	if err != nil {
		h.logger.Error(
			"failed to build work order schema",
			"template_id", template.ID,
			"error", err,
		)
		h.page.RenderError(
			w,
			http.StatusInternalServerError,
			t("workorders.new"),
			t("error.server_error"),
		)
		return
	}

	formValues := make(map[string]any, len(values))
	for name, value := range values {
		formValues[name] = value
	}

	for i, item := range template.HeaderItems {
		if item.ToFillOut != "upon_creating" {
			continue
		}

		name := workOrderHeaderFieldName(i)
		if _, exists := formValues[name]; !exists {
			if strings.EqualFold(strings.TrimSpace(item.Type), "boolean") {
				defaultValue := strings.ToLower(
					strings.TrimSpace(item.DefaultValue),
				)
				formValues[name] = defaultValue == "true" ||
					defaultValue == "1" ||
					defaultValue == "on"
			} else {
				formValues[name] = item.DefaultValue
			}
		}
	}

	// Render the same schema in separate visual groups. Hidden fields are
	// omitted from each group, so every control is submitted exactly once.
	baseForm, err := fe.ParseSchema(baseSchema)
	if err != nil {
		http.Error(w, t("error.schema_error"), http.StatusInternalServerError)
		return
	}
	baseRC, headerRC := rc, rc
	headerRC.CSRFToken = ""
	creationCount, executionCount := 0, 0
	for i, item := range template.HeaderItems {
		switch item.ToFillOut {
		case "upon_creating":
			baseRC.HiddenFields = append(baseRC.HiddenFields, workOrderHeaderFieldName(i))
			creationCount++
		case "upon_executing":
			executionCount++
		}
	}
	for _, field := range baseForm.Fields {
		headerRC.HiddenFields = append(headerRC.HiddenFields, field.Name)
	}
	formContent := formEng.Render(ctx, schema, formValues, errs, baseRC)
	headerContent := formEng.Render(ctx, schema, formValues, errs, headerRC)

	var assetsContent, extrasContent mi.H
	var assetTypeName string

	if template.ID != 0 {
		selectionSchema, err := workOrderSelectionSchema(ctx, template)
		if err != nil {
			h.logger.Error("failed to build work order selection schema", "template_id", template.ID, "error", err)
			h.page.RenderError(w, http.StatusInternalServerError, t("workorders.new"), t("error.server_error"))
			return
		}

		selectionRC := rc
		selectionRC.CSRFToken = ""
		selectionRC.HiddenFields = []string{workOrderExtraConsumableIDs}
		assetsContent = formEng.Render(ctx, selectionSchema, formValues, errs, selectionRC)

		if workOrderAllowsExtraConsumables(template) {
			selectionRC.HiddenFields = []string{workOrderAssetIDs}
			extrasContent = formEng.Render(ctx, selectionSchema, formValues, errs, selectionRC)
		}

		if template.AssetTypeID > 0 {
			assetTypeName = "#" + strconv.FormatInt(template.AssetTypeID, 10)
			if entity, err := h.xoluClient.Get(ctx, "asset_types", template.AssetTypeID); err == nil {
				if name := toString(entity.Data["name"]); name != "" {
					assetTypeName = name
				}
			}
		}
	}

	content := func(b *mi.Builder) mi.Node {
		if template.ID == 0 {
			return formContent(b)
		}

		sections := []interface{}{
			mi.Class("space-y-6"),
			b.Input(mi.Type("hidden"), mi.Name("template_version"), mi.Value(strconv.Itoa(template.Version))),
			b.Input(
				mi.Type("hidden"),
				mi.Name("template_id"),
				mi.Value(strconv.FormatInt(template.ID, 10)),
			),
			workOrderTemplateBanner(template, t)(b),
			b.Div(mi.Class("space-y-4"),
				b.H2(mi.Class("text-sm font-medium text-gray-900 dark:text-white"), t("workorders-templates.info_title")),
				formContent(b),
			),
		}

		assetSection := []interface{}{
			mi.Class("space-y-2 border-t border-gray-200 dark:border-gray-700 pt-6"),
		}
		if assetTypeName != "" {
			assetSection = append(assetSection, b.P(
				mi.Class("text-sm text-gray-500 dark:text-gray-400"),
				t("workorders.allowed_asset_type", assetTypeName),
			))
		}
		assetSection = append(assetSection, assetsContent(b))
		sections = append(sections, b.Div(assetSection...))

		if creationCount > 0 {
			sections = append(sections, b.Div(mi.Class("space-y-4 border-t border-gray-200 dark:border-gray-700 pt-6"),
				b.H2(mi.Class("text-sm font-medium text-gray-900 dark:text-white"), t("workorders-templates.header_title")),
				b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400"), t("workorders-templates.header_fields_hint")),
				headerContent(b),
			))
		}

		if extrasContent != nil {
			sections = append(sections, b.Div(
				mi.Class("border-t border-gray-200 dark:border-gray-700 pt-6"),
				extrasContent(b),
			))
		}

		if executionCount > 0 || len(template.Blocks) > 0 {
			items := []interface{}{}
			for _, item := range template.HeaderItems {
				if item.ToFillOut == "upon_executing" {
					items = append(items, b.Li(item.Tag))
				}
			}
			for _, block := range template.Blocks {
				items = append(items, b.Li(block.Name))
			}
			sections = append(sections, b.Div(mi.Class("space-y-3 border-t border-gray-200 dark:border-gray-700 pt-6"),
				b.H2(mi.Class("text-sm font-medium text-gray-900 dark:text-white"), t("workorders-template.to_fill_out.upon_executing")),
				b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400"), t("workorders.execution_hint")),
				b.Ul(append([]interface{}{mi.Class("list-disc pl-5 space-y-1 text-sm text-gray-600 dark:text-gray-300")}, items...)...),
			))
		}
		return b.Div(sections...)
	}

	responsibleID := strings.TrimSpace(toString(formValues["responsible_id"]))

	responsibleName := ""

	if toString(formValues["assignment_type"]) == "direct_assignment" {
		if id, err := strconv.ParseInt(responsibleID, 10, 64); err == nil && id > 0 {
			name, resolveErr := h.resolveWorkOrderResponsible(ctx, toString(formValues["responsible_type"]), id)

			if resolveErr == nil {
				responsibleType := toString(formValues["responsible_type"])

				typeLabel := t("direct-assignment.direct_responsible_type." + responsibleType)

				responsibleName = typeLabel + ": " + name
			}
		}
	}

	data := WorkOrderFormData{
		CSRFToken:       csrfToken,
		FormContent:     content,
		Errors:          errs,
		ResponsibleID:   responsibleID,
		ResponsibleName: responsibleName,
	}

	h.page.Render(w, r, t("workorders.new"), "/workorders", WorkOrderFormPage(data, t))
}

// WorkOrderCreate validates creation fields and instantiates the selected
// template at its first state. Orders without templates retain the legacy FSM.
func (h *UIHandler) WorkOrderCreate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, t("error.invalid_form_data"), http.StatusBadRequest)
		return
	}
	title := r.PostForm.Get("title")
	description := r.PostForm.Get("description")

	var template WorkOrderTemplate

	if r.PostForm.Has("template_id") {
		var ok bool
		template, ok = h.loadWorkOrderTemplate(w, r, r.PostForm.Get("template_id"))
		if !ok {
			return
		}
		if rawVersion := r.PostForm.Get("template_version"); rawVersion != "" {
			version, parseErr := strconv.Atoi(rawVersion)
			if parseErr != nil || version != template.Version {
				http.Error(w, t("workorders.error.template_version"), http.StatusConflict)
				return
			}
		}
	}

	baseSchema := resolveSchema(ctx, "workorders", "create", workorderschemas.Schema("create"))

	schema, err := buildWorkOrderSchema(baseSchema, template, "upon_creating")
	if err != nil {
		h.logger.Error(
			"failed to build work order schema",
			"error", err,
		)
		h.page.RenderError(w, http.StatusInternalServerError, t("workorders.new"), t("error.server_error"))
		return
	}

	values, typedValues, errs, err := h.parseWorkOrderExecution(r, schema, true)
	if err != nil {
		h.logger.Error("failed to validate work order", "error", err)
		http.Error(w, t("error.validation"), http.StatusInternalServerError)
		return
	}

	var selections map[string][]int64
	if template.ID != 0 {
		selections, err = h.parseWorkOrderSelections(r, template, values, errs)
		if err != nil {
			h.logger.Error("failed to validate work order selections", "template_id", template.ID, "error", err)
			http.Error(w, t("error.validation"), http.StatusInternalServerError)
			return
		}
	}

	assignment, assignmentErr := h.parseWorkOrderAssignment(r, values, errs, t)

	if assignmentErr != nil {
		h.logger.Error("failed to resolve work order responsible", "error", assignmentErr)

		errs["_"] = t("error.server_error")
	}

	if template.ID != 0 {
		if _, dependencyErr := workOrderBlockPredecessors(template.Blocks); dependencyErr != nil {
			h.logger.Error("invalid block dependencies in selected template", "template_id", template.ID, "template_version", template.Version, "error", dependencyErr)

			errs["blocks"] = t("blocks.error.invalid_dependencies")
		}
	}

	if len(errs) > 0 {
		h.renderWorkOrderNew(w, r, template, values, errs)
		return
	}

	title = strings.TrimSpace(title)
	if len(errs) > 0 {
		h.renderWorkOrderNew(w, r, template, values, errs)
		return
	}
	title = strings.TrimSpace(title)

	headerValues := map[string]any{}
	for i, item := range template.HeaderItems {
		if item.ToFillOut != "upon_creating" {
			continue
		}

		name := workOrderHeaderFieldName(i)
		if value, exists := typedValues[name]; exists {
			headerValues[item.Tag] = value
		}
	}

	initialState := "draft"
	machineID := int64(0)
	if template.ID != 0 {
		if err := validateWorkOrderWorkflow(template); err != nil {
			h.renderWorkOrderNew(w, r, template, values, map[string]string{"_": t("workorders.error.invalid_workflow")})
			return
		}
		initialState = strings.TrimSpace(template.WorkflowStates[0].Name)
	} else {
		defID, err := h.ensureWorkOrderDefinition(ctx)
		if err != nil {
			h.logger.Error("failed to bootstrap work order FSM definition", "error", err)
			h.renderWorkOrderNew(w, r, template, values, map[string]string{"_": t("error.server_error")})
			return
		}
		machine, err := h.xoluClient.CreateMachine(ctx, xolu.CreateMachineRequest{Definition: defID})
		if err != nil {
			h.renderWorkOrderNew(w, r, template, values, map[string]string{"_": t("error.server_error")})
			return
		}
		initialState, machineID = machine.State, machine.ID
	}

	orderData := map[string]any{
		"title":            title,
		"description":      description,
		"assigned_to":      assignment.ResponsibleName,
		"state":            initialState,
		"machine_id":       machineID,
		"created_at":       time.Now().UTC().Format(time.RFC3339),
		"updated_at":       time.Now().UTC().Format(time.RFC3339),
		"block_values":     map[string]any{},
		"assignment_type":  assignment.AssignmentType,
		"responsible_type": assignment.ResponsibleType,
		"responsible_id":   assignment.ResponsibleID,
	}

	if template.ID != 0 {
		orderData["workflow_mode"] = "template"
		orderData["workflow_index"] = 0
		orderData["template_id"] = template.ID
		orderData["template_version"] = template.Version
		orderData["header_values"] = headerValues
		for name, ids := range selections {
			orderData[name] = ids
		}
	}

	entity, err := h.createWorkOrderWithWarehouse(ctx, template, orderData)
	if err != nil {
		h.logger.Error("failed to create work order", "error", err)

		message := t("error.create_failed") + t("workorders.detail_title")

		var uncertain *workOrderCreationUncertainError

		if errors.As(err, &uncertain) {
			message = t("workorders.error.creation_unconfirmed", uncertain.OrderID)
		}

		h.renderWorkOrderNew(w, r, template, values, map[string]string{"_": message})
		return
	}

	// Deliberately no audit logging here -- adding new AuditEventType
	// constants to internal/auth's shared enum for a playground feature
	// would be scope creep ahead of T-11's own real scoping. The team
	// building out the production module can add this properly once
	// the event taxonomy for Work Orders is actually decided.
	http.Redirect(w, r, "/workorders/"+strconv.FormatInt(entity.ID, 10), http.StatusSeeOther)
}

// createWorkOrderWithWarehouse saves the order, its block records and its
// optional mobile warehouse in one entity commit.
//
// FSM instances are created beforehand because CreateMachine is a separate
// API operation. Known commit rejections trigger best-effort cleanup;
// ambiguous outcomes are checked before any machines are discarded.
func (h *UIHandler) createWorkOrderWithWarehouse(ctx context.Context, template WorkOrderTemplate, data map[string]any) (*xolu.Entity, error) {
	hasBlocks := template.ID != 0 && len(template.Blocks) > 0
	hasWarehouse := template.ID != 0 && template.ConsumablesEnabled

	if !hasBlocks && !hasWarehouse {
		return h.xoluClient.Create(ctx, "work_orders", data)
	}

	// Validate dependencies and commit capacity before creating machines.
	if _, err := workOrderBlockPredecessors(template.Blocks); err != nil {
		return nil, err
	}

	appendCount := len(template.Blocks)
	if hasWarehouse {
		appendCount++
	}

	// Xolu currently accepts at most 25 appended entities per commit.
	if appendCount > 25 {
		return nil, fmt.Errorf("work order needs %d commit appends; maximum is 25", appendCount)
	}

	// The order ID must be available before preparing its related records.
	// Keep it within JavaScript's exact integer range.
	var raw [24]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return nil, fmt.Errorf("generate work order identity: %w", err)
	}

	orderID := int64(binary.BigEndian.Uint64(raw[:8])&((1<<52)-1)) + 1

	// The independent token identifies this creation attempt when a commit
	// response is lost and the order must be read back.
	creationToken := fmt.Sprintf("%x", raw[8:])

	// Commit's primary update is an upsert. Refuse an existing ID before
	// preparing any related machines.
	result, err := h.xoluClient.OQL(ctx, fmt.Sprintf("SELECT TOP 1 id FROM work_orders WHERE id = %d", orderID))

	if err != nil && !entityNotYetCreated(err) {
		return nil, err
	}

	if result != nil && len(result.Data) != 0 {
		return nil, fmt.Errorf("generated work order ID already exists")
	}

	orderData := make(map[string]any, len(data)+4)
	for key, value := range data {
		orderData[key] = value
	}

	orderData["creation_token"] = creationToken

	appends, machineIDs, err := h.prepareWorkOrderBlocks(ctx, orderID, template)

	if err != nil {
		// No entity commit has been attempted at this point.
		h.discardWorkOrderBlockMachines(ctx, orderID, machineIDs)
		return nil, err
	}

	orderData["block_execution_version"] = 1
	orderData["block_count"] = len(template.Blocks)

	if hasWarehouse {
		// IDs are scoped by entity, so the warehouse may share the order ID.
		orderData["mobile_warehouse_id"] = orderID

		warehouseData := map[string]any{
			"name":          toString(data["title"]) + " - warehouse",
			"type":          "mobile",
			"work_order_id": orderID,
			"enabled":       true,
			"consumables":   []WarehouseConsumableBalance{},
		}

		if claims, ok := auth.ClaimsFromContext(ctx); ok &&
			claims.UserID > 0 {
			warehouseData["responsible_user_id"] = claims.UserID
		}

		appends = append(appends, xolu.CommitAppend{
			Entity: "warehouse",
			ID:     &orderID,
			Data:   warehouseData,
		})
	}

	_, commitErr := h.xoluClient.Commit(ctx,
		xolu.CommitRequest{
			Update: xolu.CommitUpdate{
				Entity: "work_orders",
				ID:     orderID,
				Data:   orderData,
			},
			Append: appends,
		},
	)

	if commitErr == nil {
		return &xolu.Entity{
			ID:   orderID,
			Data: orderData,
		}, nil
	}

	// These responses represent explicit request rejection. The entity
	// commit did not succeed, so its newly created machines can be removed.
	var xerr *xolu.Error

	if errors.As(commitErr, &xerr) {
		switch xerr.HTTPStatus {
		case 400, 401, 403, 404, 409, 413, 422:
			h.discardWorkOrderBlockMachines(ctx, orderID, machineIDs)

			return nil, fmt.Errorf("commit work order and blocks: %w", commitErr)
		}
	}

	// A timeout or server error does not prove that the commit rolled back.
	// Confirm this attempt by reading the order with a fresh context.
	checkCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	saved, lookupErr := h.xoluClient.Get(checkCtx, "work_orders", orderID)

	if lookupErr == nil &&
		saved != nil &&
		toString(saved.Data["creation_token"]) == creationToken {
		return saved, nil
	}

	// Preserve the machines: the commit may still have succeeded, even if
	// this read failed or happened before the server finished processing it.
	h.logger.Error(
		"work order commit outcome is uncertain; block FSMs preserved",
		"work_order_id", orderID,
		"machine_ids", machineIDs,
		"commit_error", commitErr,
		"lookup_error", lookupErr,
	)

	return nil, &workOrderCreationUncertainError{
		OrderID: orderID,
		Cause:   commitErr,
	}
}

// WorkOrderDetail handles GET /workorders/{id}.
func (h *UIHandler) WorkOrderDetail(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("error.not_found"))
		return
	}

	entity, err := h.xoluClient.Get(ctx, "work_orders", id)
	if err != nil {
		h.logger.Error("failed to load work order", "id", id, "error", err)
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("error.not_found"))
		return
	}
	wo := workOrderFromEntity(entity)

	template, err := h.orderTemplate(ctx, wo)
	if err != nil {
		http.Error(w, t("workorders.error.template_version"), http.StatusConflict)
		return
	}
	if wo.WorkflowMode == "template" {
		if _, err := workOrderWorkflowPosition(wo, template); err != nil {
			http.Error(w, t("workorders.error.invalid_workflow"), http.StatusConflict)
			return
		}
	}
	h.renderWorkOrderExecution(w, r, wo, template, nil, nil, "")
}

func (wo WorkOrder) awaitingClaim() bool {
	return wo.AssignmentType == workOrderBoardPosting && wo.ResponsibleID == 0
}

func (wo WorkOrder) canExecute(ctx context.Context) bool {
	if wo.AssignmentType != workOrderBoardPosting {
		return true
	}
	claims, ok := auth.ClaimsFromContext(ctx)
	return ok && claims.UserID > 0 && wo.ResponsibleID > 0 && claims.UserID == wo.ClaimedByUserID
}

type workOrderClaimOption struct {
	id   int64
	name string
}

func (h *UIHandler) workOrderClaimOptions(ctx context.Context, responsibleType string) ([]workOrderClaimOption, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok || claims.UserID <= 0 {
		return nil, nil
	}
	var entity string
	switch responsibleType {
	case "user":
		name := claims.Email
		if name == "" {
			name = fmt.Sprintf("#%d", claims.UserID)
		}
		return []workOrderClaimOption{{id: claims.UserID, name: name}}, nil
	case "crew":
		entity = "crews"
	case "third_party":
		entity = "outsourced"
	default:
		return nil, nil
	}
	result, err := h.xoluClient.OQL(ctx, fmt.Sprintf("SELECT id, name FROM %s WHERE responsible_user_id = %d ORDER BY name, id", entity, claims.UserID))
	if err != nil {
		if entityNotYetCreated(err) {
			return nil, nil
		}
		return nil, err
	}
	options := make([]workOrderClaimOption, 0, len(result.Data))
	for _, row := range result.Data {
		if id := toInt64(row["id"]); id > 0 {
			options = append(options, workOrderClaimOption{id: id, name: toString(row["name"])})
		}
	}
	return options, nil
}

func (h *UIHandler) WorkOrderClaim(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok || claims.UserID <= 0 {
		http.Error(w, t("error.forbidden"), http.StatusForbidden)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, t("error.not_found"), http.StatusNotFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, t("error.invalid_form_data"), http.StatusBadRequest)
		return
	}
	responsibleID, err := strconv.ParseInt(r.PostForm.Get("responsible_id"), 10, 64)
	if err != nil || responsibleID <= 0 {
		http.Error(w, t("error.invalid_form_data"), http.StatusBadRequest)
		return
	}
	orderURL := fmt.Sprintf("/workorders/%d", id)
	entity, err := h.xoluClient.Get(ctx, "work_orders", id)
	if err != nil {
		var xerr *xolu.Error
		if errors.As(err, &xerr) && xerr.HTTPStatus == http.StatusNotFound {
			http.Error(w, t("error.not_found"), http.StatusNotFound)
		} else {
			h.logger.Error("failed to load work order for claim", "id", id, "error", err)
			http.Error(w, t("error.server_error"), http.StatusInternalServerError)
		}
		return
	}
	wo := workOrderFromEntity(entity)
	if !wo.awaitingClaim() {
		if wo.AssignmentType == workOrderBoardPosting && wo.ClaimedByUserID == claims.UserID && wo.ResponsibleID == responsibleID {
			http.Redirect(w, r, orderURL, http.StatusSeeOther)
			return
		}
		http.Error(w, t("workorders.backlog.unavailable"), http.StatusConflict)
		return
	}
	options, err := h.workOrderClaimOptions(ctx, wo.ResponsibleType)
	if err != nil {
		h.logger.Error("failed to resolve work order claim options", "id", id, "error", err)
		http.Error(w, t("error.server_error"), http.StatusInternalServerError)
		return
	}
	for _, option := range options {
		if option.id != responsibleID {
			continue
		}
		version := toInt64(entity.Data["_version"])
		if version <= 0 {
			h.logger.Error("missing work order version during claim", "id", id)
			http.Error(w, t("error.server_error"), http.StatusInternalServerError)
			return
		}
		_, err = h.xoluClient.Patch(ctx, "work_orders", id, map[string]any{
			"_version":           version,
			"assigned_to":        option.name,
			"claimed_by_user_id": claims.UserID,
			"responsible_id":     option.id,
			"updated_at":         time.Now().UTC().Format(time.RFC3339),
		})
		if err == nil {
			http.Redirect(w, r, orderURL, http.StatusSeeOther)
			return
		}
		var xerr *xolu.Error
		if errors.As(err, &xerr) && xerr.HTTPStatus == http.StatusConflict {
			http.Error(w, t("workorders.backlog.unavailable"), http.StatusConflict)
			return
		}
		h.logger.Error("failed to claim work order", "id", id, "error", err)
		http.Error(w, t("error.server_error"), http.StatusInternalServerError)
		return
	}
	http.Error(w, t("workorders.backlog.ineligible"), http.StatusForbidden)
}

// WorkOrderStart handles POST /workorders/{id}/start -- walks the
// machine from "draft" to "in_progress", no payload/guard involved.
func (h *UIHandler) WorkOrderStart(w http.ResponseWriter, r *http.Request) {
	h.walkWorkOrder(w, r, "start", nil)
}

// WorkOrderComplete handles POST /workorders/{id}/complete -- the
// actual composition proof point: a real form submission (the
// signature widget's hidden-input value) becomes the walk payload the
// FSM guard evaluates.
func (h *UIHandler) WorkOrderComplete(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, t("error.invalid_form_data"), http.StatusBadRequest)
		return
	}
	signature := r.PostForm.Get("technician_signature")
	h.walkWorkOrder(w, r, "complete", map[string]any{"technician_signature": signature})
}

// walkWorkOrder is the shared fetch-walk-denormalize sequence both
// WorkOrderStart and WorkOrderComplete use -- kept as one function so
// the "always re-fetch and PATCH the denormalized state after a walk"
// rule can't drift between the two call sites the way T-49's original
// PUT-vs-PATCH bug drifted across 17 independently-written call sites
// earlier this session.
func (h *UIHandler) walkWorkOrder(w http.ResponseWriter, r *http.Request, input string, payload map[string]any) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("error.not_found"))
		return
	}

	entity, err := h.xoluClient.Get(ctx, "work_orders", id)
	if err != nil {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("error.not_found"))
		return
	}
	wo := workOrderFromEntity(entity)
	if wo.awaitingClaim() {
		http.Error(w, t("workorders.backlog.pending"), http.StatusConflict)
		return
	}
	if !wo.canExecute(ctx) {
		http.Error(w, t("error.forbidden"), http.StatusForbidden)
		return
	}

	template, err := h.orderTemplate(ctx, wo)
	if err != nil {
		http.Error(w, t("workorders.error.template_version"), http.StatusConflict)
		return
	}
	if wo.WorkflowMode == "template" {
		h.advanceTemplateWorkOrder(w, r, entity, wo, template, input)
		return
	}
	patchValues := map[string]any{}
	var postedValues map[string]any
	var fieldErrors map[string]string
	if input == "complete" {
		if wo.State != "in_progress" {
			http.Error(w, t("error.validation")+": "+t("workorders.state.in_progress"), http.StatusConflict)
			return
		}
		save := r.PostForm.Get("intent") == "save"
		schema, buildErr := workOrderExecutionSchema(ctx, wo, template)
		if buildErr != nil {
			http.Error(w, t("error.schema_error"), 500)
			return
		}
		var typed map[string]any
		postedValues, typed, fieldErrors, err = h.parseWorkOrderExecution(r, schema, !save && template.ID != 0)
		if err != nil {
			http.Error(w, t("error.validation"), 500)
			return
		}
		if len(fieldErrors) > 0 {
			h.renderWorkOrderExecution(w, r, wo, template, postedValues, fieldErrors, t("error.validation"))
			return
		}
		header := map[string]any{}
		for key, value := range wo.HeaderValues {
			header[key] = value
		}
		for i, item := range template.HeaderItems {
			if item.ToFillOut != "upon_executing" {
				continue
			}
			delete(header, item.Tag)
			if value, exists := typed[workOrderHeaderFieldName(i)]; exists {
				header[item.Tag] = value
			}
		}
		blocks := map[string]any{}
		for bi, block := range template.Blocks {
			for li := range block.Lines {
				name := workOrderBlockFieldName(bi, li)
				if value, exists := typed[name]; exists {
					blocks[name] = value
				}
			}
		}
		patchValues["header_values"], patchValues["block_values"] = header, blocks
		patchValues["technician_signature"] = toString(typed["technician_signature"])
		if save {
			patchValues["updated_at"] = time.Now().UTC().Format(time.RFC3339)
			if _, err := h.xoluClient.Patch(ctx, "work_orders", id, patchValues); err != nil {
				http.Error(w, t("error.update_failed")+t("workorders.detail_title"), 500)
				return
			}
			http.Redirect(w, r, fmt.Sprintf("/workorders/%d", id), http.StatusSeeOther)
			return
		}
	}

	result, err := h.xoluClient.WalkMachine(ctx, wo.MachineID, xolu.WalkRequest{Input: input, Payload: payload})
	if err != nil {
		h.logger.Info("work order walk rejected", "id", id, "input", input, "error", err)
		message := t("error.update_failed")

		var xerr *xolu.Error
		if errors.As(err, &xerr) && xerr.Code == "XOLU-FSM005" {
			message = t("workorders.error.transition")
			if input == "complete" && strings.TrimSpace(toString(payload["technician_signature"])) == "" {
				message = t("field.technician_signature") + " — " + t("core.errors.required")
			}
		}

		h.renderWorkOrderExecution(w, r, wo, template, postedValues, fieldErrors, message)
		return
	}

	patchData := map[string]any{
		"state":      result.Current,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	}
	for key, value := range patchValues {
		patchData[key] = value
	}
	if sig, ok := payload["technician_signature"]; ok {
		patchData["technician_signature"] = sig
	}
	if _, err := h.xoluClient.Patch(ctx, "work_orders", id, patchData); err != nil {
		h.logger.Error("work order walked but denormalized state patch failed", "id", id, "error", err)
		http.Error(w, t("error.update_failed")+t("workorders.detail_title"), http.StatusInternalServerError)
		return
	}

	// Same audit-logging scope decision as WorkOrderCreate above.
	http.Redirect(w, r, "/workorders/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// orderTemplate resolves the exact record/version selected at creation.
func (h *UIHandler) orderTemplate(ctx context.Context, wo WorkOrder) (WorkOrderTemplate, error) {
	if wo.TemplateID == 0 {
		return WorkOrderTemplate{}, nil
	}
	entity, err := h.xoluClient.Get(ctx, "work_order_templates", wo.TemplateID)
	if err != nil {
		return WorkOrderTemplate{}, err
	}
	template, err := workOrderTemplateFromEntity(entity)
	if err != nil {
		return WorkOrderTemplate{}, err
	}
	if wo.TemplateVersion > 0 && wo.TemplateVersion != template.Version {
		return WorkOrderTemplate{}, fmt.Errorf("work order template version mismatch")
	}
	return template, nil
}

// validateWorkOrderWorkflow checks that the template defines at least two
// states with distinct, non-empty names. Used by WorkOrderCreate before
// choosing the initial state and by workOrderWorkflowPosition before
// checking a saved order's position.
func validateWorkOrderWorkflow(template WorkOrderTemplate) error {
	if len(template.WorkflowStates) < 2 {
		return fmt.Errorf("workflow requires at least two states")
	}
	seen := map[string]bool{}
	for _, state := range template.WorkflowStates {
		name := strings.TrimSpace(state.Name)
		if name == "" || seen[name] {
			return fmt.Errorf("workflow states must have distinct nonempty names")
		}
		seen[name] = true
	}
	return nil
}

// workOrderWorkflowPosition checks that the saved index points to the
// order's current state in the template. Used by WorkOrderDetail and
// advanceTemplateWorkOrder before rendering or advancing the order.
func workOrderWorkflowPosition(wo WorkOrder, template WorkOrderTemplate) (int, error) {
	if err := validateWorkOrderWorkflow(template); err != nil {
		return 0, err
	}
	i := wo.WorkflowIndex
	if i < 0 || i >= len(template.WorkflowStates) || wo.State != strings.TrimSpace(template.WorkflowStates[i].Name) {
		return 0, fmt.Errorf("workflow position does not match state")
	}
	return i, nil
}

// workOrderExecutionSchema builds the schema for the current execution
// step, including the completion fields when applicable. Used to render
// the execution form and validate its submission.
func workOrderExecutionSchema(ctx context.Context, wo WorkOrder, template WorkOrderTemplate) ([]byte, error) {
	if wo.WorkflowMode == "template" && wo.WorkflowIndex < len(template.WorkflowStates)-2 {
		base := []byte(`{"type":"object","properties":{}}`)
		return buildWorkOrderSchema(base, template, "upon_executing")
	}
	base := resolveSchema(ctx, "workorders", "complete", workorderschemas.Schema("complete"))
	return buildWorkOrderSchema(base, template, "upon_executing")
}

// advanceTemplateWorkOrder persists answers and the next state in one entity
// update. It never calls the playground FSM or changes legacy machines.
func (h *UIHandler) advanceTemplateWorkOrder(w http.ResponseWriter, r *http.Request, entity *xolu.Entity, wo WorkOrder, template WorkOrderTemplate, input string) {
	t := h.translateFunc(r)
	i, err := workOrderWorkflowPosition(wo, template)
	if err != nil {
		http.Error(w, t("workorders.error.invalid_workflow"), http.StatusConflict)
		return
	}
	if input != "complete" || i == len(template.WorkflowStates)-1 {
		http.Error(w, t("workorders.error.transition"), http.StatusConflict)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, t("error.invalid_form_data"), 400)
		return
	}
	postedIndex, err := strconv.Atoi(r.PostForm.Get("workflow_index"))
	if err != nil || postedIndex != i {
		http.Error(w, t("workorders.error.stale_state"), http.StatusConflict)
		return
	}
	save := r.PostForm.Get("intent") == "save"
	schema, err := workOrderExecutionSchema(r.Context(), wo, template)
	if err != nil {
		http.Error(w, t("error.schema_error"), 500)
		return
	}
	values, typed, errs, err := h.parseWorkOrderExecution(r, schema, !save)
	if err != nil {
		http.Error(w, t("error.validation"), 500)
		return
	}
	if len(errs) > 0 {
		h.renderWorkOrderExecution(w, r, wo, template, values, errs, t("error.validation"))
		return
	}
	header := map[string]any{}
	for key, value := range wo.HeaderValues {
		header[key] = value
	}
	for j, item := range template.HeaderItems {
		if item.ToFillOut != "upon_executing" {
			continue
		}
		delete(header, item.Tag)
		if value, ok := typed[workOrderHeaderFieldName(j)]; ok {
			header[item.Tag] = value
		}
	}
	blocks := map[string]any{}
	for bi, block := range template.Blocks {
		for li := range block.Lines {
			name := workOrderBlockFieldName(bi, li)
			if value, ok := typed[name]; ok {
				blocks[name] = value
			}
		}
	}
	patch := map[string]any{"header_values": header, "block_values": blocks, "updated_at": time.Now().UTC().Format(time.RFC3339)}
	if _, ok := values["technician_signature"]; ok {
		patch["technician_signature"] = toString(typed["technician_signature"])
	}
	if !save {
		patch["state"] = strings.TrimSpace(template.WorkflowStates[i+1].Name)
		patch["workflow_index"] = i + 1
		if i+1 == len(template.WorkflowStates)-1 {
			patch["completed_at"] = time.Now().UTC().Format(time.RFC3339)
		}
	}
	if version := toInt64(entity.Data["_version"]); version > 0 {
		patch["_version"] = version
	}
	_, err = h.xoluClient.Patch(r.Context(), "work_orders", wo.ID, patch)
	if err != nil {
		message := t("workorders.error.save")
		var xerr *xolu.Error
		if errors.As(err, &xerr) && xerr.HTTPStatus == 409 {
			message = t("workorders.error.stale_state")
		}
		h.logger.Error("failed to update template work order", "id", wo.ID, "error", err)
		h.renderWorkOrderExecution(w, r, wo, template, values, nil, message)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/workorders/%d", wo.ID), http.StatusSeeOther)
}

// workOrderExecutionValues prepares the execution form from stored answers
// and template defaults. Used by renderWorkOrderExecution when it has no
// submitted values to redisplay.
func workOrderExecutionValues(wo WorkOrder, template WorkOrderTemplate) map[string]any {
	values := map[string]any{"technician_signature": wo.TechnicianSignature}
	for i, item := range template.HeaderItems {
		if item.ToFillOut != "upon_executing" {
			continue
		}
		value, exists := wo.HeaderValues[item.Tag]
		if !exists {
			value = item.DefaultValue
			if strings.EqualFold(item.Type, "boolean") {
				normalized := strings.ToLower(strings.TrimSpace(item.DefaultValue))
				value = normalized == "true" || normalized == "1" || normalized == "on"
			}
		}
		values[workOrderHeaderFieldName(i)] = value
	}
	for name, value := range wo.BlockValues {
		values[name] = value
	}
	return values
}

// parseWorkOrderExecution keeps raw input for errors and typed input for storage.
// Saving progress relaxes required fields, but never accepts malformed values.
func (h *UIHandler) parseWorkOrderExecution(r *http.Request, schema []byte, finish bool) (map[string]any, map[string]any, map[string]string, error) {
	if !finish {
		var document map[string]json.RawMessage
		if err := json.Unmarshal(schema, &document); err != nil {
			return nil, nil, nil, err
		}
		delete(document, "required")
		var err error
		schema, err = json.Marshal(document)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	form, err := fe.ParseSchema(schema)
	if err != nil {
		return nil, nil, nil, err
	}
	t := h.translateFunc(r)
	rawValues, typed, errs := map[string]any{}, map[string]any{}, map[string]string{}
	for _, field := range form.Fields {
		raw := r.PostForm.Get(field.Name)
		rawValues[field.Name] = raw
		text := strings.TrimSpace(raw)
		if field.JSONType == "boolean" {
			switch strings.ToLower(text) {
			case "", "false", "0", "off":
				typed[field.Name], rawValues[field.Name] = false, false
			case "true", "1", "on":
				typed[field.Name], rawValues[field.Name] = true, true
			default:
				errs[field.Name] = t("core.errors.invalid_pattern")
			}
			continue
		}
		if text == "" {
			if field.Required {
				errs[field.Name] = t("core.errors.required")
			}
			continue
		}
		if field.Format == "date" {
			if _, err := time.Parse("2006-01-02", text); err != nil {
				errs[field.Name] = t("core.errors.invalid_pattern")
				continue
			}
		}
		switch field.JSONType {
		case "number":
			number, err := strconv.ParseFloat(text, 64)
			if err != nil || math.IsInf(number, 0) || math.IsNaN(number) {
				errs[field.Name] = t("core.errors.invalid_pattern")
				continue
			}
			typed[field.Name] = number
		case "integer":
			number, err := strconv.ParseInt(text, 10, 64)
			if err != nil || (field.XSeamInput == "any-picker" && number <= 0) {
				errs[field.Name] = t("core.errors.invalid_pattern")
				continue
			}
			typed[field.Name] = number
		default:
			typed[field.Name] = raw
		}
		if field.XSeamInput == "gps" {
			coordinates := strings.Split(text, ",")
			valid := len(coordinates) == 2
			if valid {
				lat, e1 := strconv.ParseFloat(strings.TrimSpace(coordinates[0]), 64)
				lon, e2 := strconv.ParseFloat(strings.TrimSpace(coordinates[1]), 64)
				valid = e1 == nil && e2 == nil && !math.IsNaN(lat) && !math.IsNaN(lon) && lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180
			}
			if !valid {
				errs[field.Name] = t("core.errors.invalid_pattern")
			}
		}
		if field.XSeamInput == "any-picker" {
			if _, err := h.xoluClient.Get(r.Context(), "assets", toInt64(typed[field.Name])); err != nil {
				errs[field.Name] = t("core.errors.invalid_pattern")
			}
		}
	}
	if err := qf.ValidateWithMode(typed, form.Compiled, qf.Loose); err != nil {
		var validation *qf.ValidationError
		if !errors.As(err, &validation) {
			return nil, nil, nil, err
		}
		engine := &fe.FormEngine{Translator: h.formEngineTranslator()}
		for _, field := range validation.Errors {
			if _, exists := errs[field.Path]; !exists {
				message := fe.ResolveQueryfyError(engine, fe.RenderContext{Locale: h.currentLocale()}, field.Message)
				if message == field.Message {
					message = t("error.validation")
				}
				errs[field.Path] = message
			}
		}
	}
	return rawValues, typed, errs, nil
}

// renderWorkOrderExecution renders the order detail and its execution form
// when applicable. Used by WorkOrderDetail for the initial view and by
// transition handlers to redisplay submitted values and errors.
func (h *UIHandler) renderWorkOrderExecution(w http.ResponseWriter, r *http.Request, wo WorkOrder, template WorkOrderTemplate, values map[string]any, errs map[string]string, message string) {
	t := h.translateFunc(r)
	schema, err := workOrderExecutionSchema(r.Context(), wo, template)
	if err != nil {
		http.Error(w, t("error.schema_error"), 500)
		return
	}
	if values == nil {
		values = workOrderExecutionValues(wo, template)
	}
	csrf := h.GetCSRFToken(w, r)
	engine := &fe.FormEngine{Translator: h.formEngineTranslator(), Sources: &AssetSourceResolver{Client: h.xoluClient}}
	var content mi.H
	canExecute := wo.canExecute(r.Context())
	if canExecute && ((wo.WorkflowMode == "template" && wo.WorkflowIndex < len(template.WorkflowStates)-1) || (wo.WorkflowMode != "template" && wo.State == "in_progress")) {
		content = engine.Render(r.Context(), schema, values, errs, fe.RenderContext{Locale: h.currentLocale(), CSRFToken: csrf, Module: "workorders", Form: "workorder"})
	}
	data := WorkOrderDetailData{WorkOrder: wo, Template: template, CSRFToken: csrf, CompleteForm: content, WalkError: message, ReadOnly: !canExecute}
	if wo.awaitingClaim() {
		options, err := h.workOrderClaimOptions(r.Context(), wo.ResponsibleType)
		if err != nil {
			h.logger.Error("failed to load work order claim options", "id", wo.ID, "error", err)
			data.WalkError = t("error.server_error")
		} else {
			data.ClaimForm = workOrderClaimForm(wo, options, csrf, t)
		}
	}
	h.page.Render(w, r, t("workorders.detail_title"), "/workorders", WorkOrderDetailPage(data, t))
}

// WorkOrdersListData holds data for the work orders list page.
type WorkOrdersListData struct {
	Orders        []WorkOrder
	TemplateNames map[int64]string
	Backlog       bool
}

// WorkOrdersListPage renders the work orders list page.
func WorkOrdersListPage(ctx context.Context, data WorkOrdersListData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	rows := make([]mi.H, len(data.Orders))
	for i, order := range data.Orders {
		assignedTo := order.AssignedTo
		if strings.TrimSpace(assignedTo) == "" {
			assignedTo = "—"
		}
		if order.awaitingClaim() {
			assignedTo = t("workorder.responsible_type." + order.ResponsibleType)
		}
		created := order.CreatedAt
		if date, err := time.Parse(time.RFC3339, created); err == nil {
			created = date.Format(time.DateOnly)
		} else if created == "" {
			created = "—"
		}
		templateCell := ListCellMuted("—")
		if order.TemplateID > 0 {
			name := data.TemplateNames[order.TemplateID]
			if name == "" {
				name = fmt.Sprintf("#%d", order.TemplateID)
			}
			if order.TemplateVersion > 0 {
				name += " · " + t("workorders.template_version", order.TemplateVersion)
			}
			templateCell = ListCellLink(name, fmt.Sprintf("/workorders-templates/%d", order.TemplateID))
		}
		warehouseCell := ListCellMuted("—")
		if order.MobileWarehouseID > 0 {
			warehouseCell = ListCellLink(t("workorders.mobile_warehouse"), fmt.Sprintf("/warehouse/%d", order.MobileWarehouseID))
		}
		state := workOrderStateBadge(order.State, t)
		if order.WorkflowMode == "template" {
			state = Badge(order.State, "info")
		}
		rows[i] = ListRow(i, ListCellRich(func(b *mi.Builder) mi.Node {
			return b.Div(mi.Class("min-w-48 max-w-sm whitespace-normal"),
				b.A(mi.Href(fmt.Sprintf("/workorders/%d", order.ID)),
					mi.Class("font-medium text-indigo-600 hover:underline dark:text-indigo-400"), order.Title),
				b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400 line-clamp-2"), order.Description),
			)
		}), ListCellRich(state), ListCell(assignedTo), templateCell, ListCellMuted(created), warehouseCell)
	}
	title, empty, assignedToLabel := t("workorders.list_title"), t("workorders.empty"), t("workorders.assigned_to_label")
	if data.Backlog {
		title, empty = t("workorders.backlog.title"), t("workorders.backlog.empty")
		assignedToLabel = t("workorders.forms.workorder.field.responsible_type")
	}
	table := ListTable(ListTableConfig{
		Columns: []ListColumn{
			{Label: t("field.title")},
			{Label: t("field.status")},
			{Label: assignedToLabel},
			{Label: t("workorders-templates")},
			{Label: t("field.created")},
			{Label: t("workorders.mobile_warehouse")},
		},
		EmptyIcon:  "assignment",
		EmptyTitle: empty,
	}, rows)
	return func(b *mi.Builder) mi.Node {
		var actions mi.Node
		linkClass := mi.Class("inline-flex items-center justify-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2 bg-indigo-600 text-white hover:bg-indigo-700 focus:ring-indigo-500")
		if data.Backlog {
			actions = b.A(mi.Href("/workorders"), linkClass, t("workorders.list_title"))
		} else {
			actions = b.Div(mi.Class("flex flex-wrap gap-2"),
				b.A(mi.Href("/workorders/backlog"), linkClass, t("workorders.backlog.title")),
				Button(t("workorders.new"), "primary", fe.ModalTriggerAttrs(
					resolveSchema(ctx, "workorders", "template-picker", workorderschemas.Schema("template-picker")),
					"/workorders/template-picker", t("workorders.new"),
				)...)(b),
			)
		}
		return b.Div(mi.Class("space-y-6"),
			b.Div(mi.Class("flex items-center justify-between"),
				b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), title),
				actions,
			),
			table(b),
		)
	}
}

// workOrderStateBadge maps this module's own state vocabulary to the
// shared Badge(text, variant) primitive (internal/ui/components.go) --
// Badge's own default (gray) for draft, "info" (blue) for in_progress,
// "success" (green) for completed. Was a hand-rolled duplicate of
// Badge's own pill markup until 2026-08-06; now a thin mapping, no
// markup of its own.
func workOrderStateBadge(state string, t func(key string, args ...any) string) mi.H {
	switch state {
	case "in_progress":
		return Badge(t("workorders.state.in_progress"), "info")
	case "completed":
		return Badge(t("workorders.state.completed"), "success")
	default:
		return Badge(t("workorders.state.draft"), "draft")
	}
}

// WorkOrderTemplatePickerPage wraps the picker fields in a GET form.
// Next sends the selected template ID to /workorders/new. SeamModal2
// moves the action buttons to its footer after loading the partial.
func WorkOrderTemplatePickerPage(content mi.H, t func(string, ...any) string) mi.H {
	const formID = "wo-template-picker-form"

	return func(b *mi.Builder) mi.Node {
		return b.Div(
			b.Style(mi.Raw(workOrderPickerModalStyle)),
			b.Form(
				mi.ID(formID),
				mi.Method("get"),
				mi.Action("/workorders/new"),

				mi.Class("space-y-4"),

				mi.Attr("onsubmit", `
					if (!this.elements.namedItem('template_id').value) {
                        this.querySelector('[data-template-required]').hidden = false;
                        this.querySelector('input[type="text"]').focus();
                        return false;
                    }`),

				content(b),

				b.P(
					mi.Attr("data-template-required", ""),
					mi.Attr("hidden", "hidden"),
					mi.Attr("role", "alert"),
					mi.Class("mt-2 text-sm text-red-600"),
					t("core.errors.required"),
				),
			),
			b.Div(
				mi.Attr("data-modal-footer", ""),
				b.Button(t("action.cancel"),
					mi.Type("button"),
					mi.Class("px-4 py-2 bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600 text-sm font-medium"),
					mi.Attr("onclick", "SeamModal2.close()"),
				),
				b.Button(t("action.save"),
					mi.Type("submit"),
					mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 text-sm font-medium"),
					mi.Attr("form", formID),
				),
			),
		)
	}
}

// WorkOrderFormData holds data for the create-work-order form.
type WorkOrderFormData struct {
	ResponsibleID   string
	ResponsibleName string
	CSRFToken       string
	FormContent     mi.H
	Errors          map[string]string
}

// workOrderErrorSummary renders distinct error messages in a single alert.
// Used by WorkOrderFormPage to show what needs attention after submission.
func workOrderErrorSummary(errs map[string]string, t func(string, ...any) string) mi.H {
	return func(b *mi.Builder) mi.Node {
		if len(errs) == 0 {
			return b.Div()
		}
		keys := make([]string, 0, len(errs))
		for key := range errs {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		messages := []interface{}{}
		seen := map[string]bool{}
		for _, key := range keys {
			message := errs[key]
			if message != "" && !seen[message] {
				messages = append(messages, b.Li(message))
				seen[message] = true
			}
		}
		return b.Div(mi.Attr("role", "alert"), mi.Class("rounded-lg border border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900/20 p-4 text-sm text-red-700 dark:text-red-300"),
			b.P(mi.Class("font-semibold"), t("error.create_failed")+t("workorders.detail_title")),
			b.Ul(append([]interface{}{mi.Class("mt-2 list-disc pl-5 space-y-1")}, messages...)...),
		)
	}
}

// WorkOrderFormPage renders the create-work-order form.
func WorkOrderFormPage(data WorkOrderFormData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	return func(b *mi.Builder) mi.Node {
		summary := data.ResponsibleName
		if summary == "" {
			summary = t("workorders.assignment.pending")
		}

		return b.Div(mi.Class("space-y-6 max-w-3xl"),
			b.Div(mi.Class("flex items-center gap-4"),
				b.A(mi.Href("/workorders"), mi.Class("text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"),
					b.Span(mi.Class("material-icons"), "arrow_back"),
				),
				b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), t("workorders.new")),
			),
			Card("", func(b *mi.Builder) mi.Node {
				return b.Form(
					mi.ID("wo-create-form"),
					mi.Class("space-y-4"),
					mi.Method("post"),
					mi.Action("/workorders"),
					mi.Attr("data-assignment-title", t("workorders.assignment.modal_title")),
					mi.Attr("data-assignment-pending", t("workorders.assignment.pending")),
					b.Input(
						mi.Type("hidden"),
						mi.Name("responsible_id"),
						mi.Value(data.ResponsibleID),
					),
					workOrderErrorSummary(data.Errors, t)(b),
					data.FormContent(b),
					b.Div(mi.Attr("data-direct-assignment", ""), mi.Attr("hidden", "hidden"), mi.Class("space-y-3 rounded-lg border p-4 border-gray-200 dark:border-gray-700"),

						b.P(mi.Attr("data-assignment-summary", ""), mi.Class("text-sm text-gray-700 dark:text-gray-200"), summary),

						b.Button(t("workorders.assignment.select"), mi.Type("button"), mi.Class("text-sm font-medium text-indigo-600 dark:text-indigo-400"),
							mi.Attr("onclick", "SeamWorkOrderAssignment.open()"),
						),
					),

					b.Div(mi.Class("flex justify-end gap-3 pt-6 border-t border-gray-200 dark:border-gray-700"),
						b.A(mi.Href("/workorders"), mi.Class("px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg font-medium hover:bg-gray-50 dark:hover:bg-gray-700"),
							t("action.cancel"),
						),
						b.Button(mi.Type("submit"), mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 font-medium"),
							t("workorders.action.create"),
						),
					),
				)
			})(b),

			b.Script(mi.Raw(workOrderAssignmentScript)),
		)
	}
}

// WorkOrderDetailData holds data for the work-order detail page.
type WorkOrderDetailData struct {
	ClaimForm    mi.H
	ReadOnly     bool
	WorkOrder    WorkOrder
	CSRFToken    string
	CompleteForm mi.H   // non-nil only when WorkOrder.State == "in_progress"
	WalkError    string // set when a transition attempt (e.g. Complete without a signature) was rejected
	Template     WorkOrderTemplate
}

// workOrderHeaderValuesContent renders saved answers in template order.
// Used by WorkOrderDetailPage to display both header values and block
// answers.
func workOrderHeaderValuesContent(template WorkOrderTemplate, values map[string]any, t func(string, ...any) string) mi.H {
	keys := make([]string, 0, len(values))
	seen := make(map[string]bool)

	// Follow the template's creation-field order.
	for _, item := range template.HeaderItems {
		if seen[item.Tag] {
			continue
		}

		keys = append(keys, item.Tag)
		seen[item.Tag] = true
	}

	// Preserve stored answers absent from the selected fields.
	extraKeys := make([]string, 0)
	for key := range values {
		if !seen[key] {
			extraKeys = append(extraKeys, key)
		}
	}
	sort.Strings(extraKeys)
	keys = append(keys, extraKeys...)

	return func(b *mi.Builder) mi.Node {
		if len(keys) == 0 {
			return b.Div()
		}

		rows := make([]interface{}, 0, len(keys))

		for _, key := range keys {
			raw, exists := values[key]
			value := "—"

			if exists && raw != nil {
				if boolean, ok := raw.(bool); ok {
					if boolean {
						value = t("action.yes")
					} else {
						value = t("action.no")
					}
				} else {
					text := fmt.Sprint(raw)
					if strings.TrimSpace(text) != "" {
						value = text
					}
				}
			}

			rows = append(rows,
				b.Div(
					b.Dt(
						mi.Class("text-sm font-medium text-gray-500 dark:text-gray-400"),
						key,
					),
					b.Dd(
						mi.Class("mt-1 text-sm text-gray-900 dark:text-white whitespace-pre-wrap break-words"),
						value,
					),
				),
			)
		}

		return b.Div(
			mi.Class("mt-4 border-t border-gray-200 dark:border-gray-700 pt-4"),
			b.Dl(append([]interface{}{mi.Class("grid grid-cols-1 sm:grid-cols-2 gap-4")}, rows...)...),
		)
	}
}

func workOrderClaimForm(wo WorkOrder, options []workOrderClaimOption, csrf string, t func(string, ...any) string) mi.H {
	return func(b *mi.Builder) mi.Node {
		if len(options) == 0 {
			return b.P(mi.Class("mt-4 text-sm text-gray-500"), t("workorders.backlog.ineligible"))
		}
		label := t("workorder.responsible_type." + wo.ResponsibleType)
		var responsible mi.Node
		if len(options) == 1 {
			responsible = b.Div(
				b.Input(mi.Type("hidden"), mi.Name("responsible_id"), mi.Value(strconv.FormatInt(options[0].id, 10))),
				b.P(mi.Class("text-sm"), label+": "+options[0].name),
			)
		} else {
			choices := []any{mi.Name("responsible_id"), mi.Attr("required", "required"), mi.Class("rounded border px-2 py-1 dark:bg-gray-800")}
			for _, option := range options {
				choices = append(choices, b.Option(mi.Value(strconv.FormatInt(option.id, 10)), option.name))
			}
			responsible = b.Label(mi.Class("flex flex-col gap-1 text-sm"), label, b.Select(choices...))
		}
		return b.Form(mi.Class("mt-4 flex flex-wrap items-end gap-3"), mi.Method("post"), mi.Action(fmt.Sprintf("/workorders/%d/claim", wo.ID)),
			b.Input(mi.Type("hidden"), mi.Name("_csrf"), mi.Value(csrf)),
			responsible,
			b.Button(mi.Type("submit"), mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700"), t("workorders.backlog.claim")),
		)
	}
}

// WorkOrderDetailPage renders the work-order detail page -- the state,
// its transition buttons (Start/Complete, whichever applies to the
// current state), and the signature once captured.
func WorkOrderDetailPage(data WorkOrderDetailData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	wo := data.WorkOrder
	stateBadge := workOrderStateBadge(wo.State, t)
	if wo.WorkflowMode == "template" {
		variant := "info"
		if wo.WorkflowIndex == len(data.Template.WorkflowStates)-1 {
			variant = "success"
		}
		stateBadge = Badge(wo.State, variant)
	}
	return func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("space-y-6 max-w-2xl"),
			b.Div(mi.Class("flex items-center gap-4"),
				b.A(mi.Href("/workorders"), mi.Class("text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"),
					b.Span(mi.Class("material-icons"), "arrow_back"),
				),
				b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), wo.Title),
				stateBadge(b),
			),
			Card("", func(b *mi.Builder) mi.Node {
				var children []interface{}
				assignedTo := wo.AssignedTo
				if wo.awaitingClaim() {
					assignedTo = t("workorders.backlog.pending")
				}
				children = append(children,
					b.Div(mi.Class("text-sm text-gray-500 dark:text-gray-400"), wo.Description),
					b.Div(mi.Class("text-sm text-gray-700 dark:text-gray-300 mt-2"),
						t("workorders.assigned_to_label")+": "+assignedTo,
					),
				)
				if data.Template.ID != 0 {
					children = append(children, b.Div(mi.Class("mt-6"), workOrderTemplateBanner(data.Template, t)(b)))
				}
				if wo.MobileWarehouseID > 0 {
					children = append(children, b.Div(mi.Class("mt-4"),
						b.A(mi.Href(fmt.Sprintf("/warehouse/%d", wo.MobileWarehouseID)),
							mi.Class("text-indigo-600 hover:underline dark:text-indigo-400"),
							t("workorders.mobile_warehouse")),
					))
				}
				if len(data.Template.HeaderItems) > 0 || len(wo.HeaderValues) > 0 {
					children = append(children,
						workOrderHeaderValuesContent(
							data.Template,
							wo.HeaderValues,
							t,
						)(b),
					)
				}
				if data.WalkError != "" {
					children = append(children, b.Div(
						mi.Attr("role", "alert"),
						mi.Class("mt-4 px-3 py-2 rounded-lg bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-sm text-red-700 dark:text-red-400"),
						data.WalkError,
					))
				}
				for bi, block := range data.Template.Blocks {
					content := func(b *mi.Builder) mi.Node {
						rows := []interface{}{}
						for li, line := range block.Lines {
							definition := WorkOrderTemplate{HeaderItems: []HeaderItem{{Tag: line.Name}}}
							answers := map[string]any{}
							if value, exists := wo.BlockValues[workOrderBlockFieldName(bi, li)]; exists {
								answers[line.Name] = value
							}
							rows = append(rows, workOrderHeaderValuesContent(definition, answers, t)(b))
						}
						return b.Div(rows...)
					}
					children = append(children, Card(block.Name, content)(b))
				}
				if wo.TechnicianSignature != "" {
					children = append(children, b.Div(mi.Class("mt-4"),
						b.Div(mi.Class("text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"), t("field.technician_signature")),
						b.Img(mi.Src(wo.TechnicianSignature), mi.Class("border border-gray-200 dark:border-gray-700 rounded-lg bg-white max-w-xs")),
					))
				}
				switch {
				case wo.awaitingClaim():
					if data.ClaimForm != nil {
						children = append(children, data.ClaimForm(b))
					}
				case data.ReadOnly:
				case wo.WorkflowMode == "template" && wo.WorkflowIndex == len(data.Template.WorkflowStates)-1:
					children = append(children, b.P(mi.Class("mt-4 text-sm text-gray-500 dark:text-gray-400"), t("workorders.workflow.finished")))
				case wo.WorkflowMode != "template" && wo.State == "draft":
					children = append(children, b.Form(mi.Class("mt-4"), mi.Method("post"), mi.Action(fmt.Sprintf("/workorders/%d/start", wo.ID)),
						b.Input(mi.Type("hidden"), mi.Name("_csrf"), mi.Value(data.CSRFToken)),
						b.Button(mi.Type("submit"), mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 font-medium"),
							t("workorders.action.start"),
						),
					))
				case wo.WorkflowMode == "template" || wo.State == "in_progress":
					if data.CompleteForm != nil {
						label := t("workorders.action.complete")
						if wo.WorkflowMode == "template" {
							label = t("workorders.action.advance", data.Template.WorkflowStates[wo.WorkflowIndex+1].Name)
						}
						children = append(children, b.Form(mi.Class("mt-4 space-y-4"), mi.Method("post"), mi.Action(fmt.Sprintf("/workorders/%d/complete", wo.ID)),
							b.Input(mi.Type("hidden"), mi.Name("workflow_index"), mi.Value(strconv.Itoa(wo.WorkflowIndex))),
							data.CompleteForm(b),
							b.Button(mi.Type("submit"), mi.Name("intent"), mi.Value("save"), mi.Attr("formnovalidate", "formnovalidate"), mi.Class("px-4 py-2 border border-gray-300 rounded-lg mr-3"), t("workorders.action.save_progress")),
							b.Button(mi.Type("submit"), mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 font-medium"),
								label,
							),
						))
					}
				}
				return b.Div(children...)
			})(b),
		)
	}
}
