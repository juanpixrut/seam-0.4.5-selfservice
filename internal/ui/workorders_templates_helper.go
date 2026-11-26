package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	mi "github.com/ha1tch/minty"
	fe "github.com/ha1tch/seam-ui/internal/formengine"
	workordertemplateschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/workorders_templates"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

// Work order template helpers.
//
// This file supports template creation and detail rendering: it decodes stored
// entities, rebuilds submitted repeatables, validates template configuration,
// and generates the action-parameter mapping form used in the second step.
//
// Action mapping has two separate representations. The generated schema and
// its controls exist only while editing the template; mapping_values stores
// the parameter bindings that are saved with each selected action.
//
// These helpers configure and describe action bindings. Executing the actions
// and resolving their context values at runtime belong to the execution flow.

// =============================================================================
// ENTITY CONVERSION AND STORED COLLECTIONS
//
// Convert Xolu's generic entity data into the typed structures used by the UI.
// Collection decoders belong here so the stored template shape can be reviewed
// without mixing it with form parsing or action-mapping presentation.
// =============================================================================

// workOrderTemplateFromEntity converts a stored Xolu entity into the template
// model used by the detail page.
//
// Nested collections are decoded separately so malformed data produces an
// error identifying the affected field instead of silently omitting it.
//
// Used by WorkOrderTemplateDetail and the work order handlers to load the template configuration from Xolu.
func workOrderTemplateFromEntity(e *xolu.Entity) (WorkOrderTemplate, error) {
	states, err := decodeWorkflowStates(e.Data["workflow_states"])
	if err != nil {
		return WorkOrderTemplate{}, fmt.Errorf("decode workflow_states: %w", err)
	}

	headerItems, err := decodeHeaderItems(e.Data["header_items"])
	if err != nil {
		return WorkOrderTemplate{}, fmt.Errorf("decode header_items: %w", err)
	}

	blocks, err := decodeBlocks(e.Data["blocks"])
	if err != nil {
		return WorkOrderTemplate{}, fmt.Errorf("decode blocks: %w", err)
	}

	consumables, err := decodeConsumables(e.Data["consumables"])
	if err != nil {
		return WorkOrderTemplate{}, fmt.Errorf("decode consumables: %w", err)
	}

	actions, err := decodeWorkOrderTemplateActions(e.Data["actions"])
	if err != nil {
		return WorkOrderTemplate{}, fmt.Errorf("decode actions: %w", err)
	}

	return WorkOrderTemplate{
		ID:                 e.ID,
		Version:            toInt(e.Data["version"]),
		Latest:             toBool(e.Data["latest"]),
		Name:               toString(e.Data["name"]),
		Description:        toString(e.Data["description"]),
		Color:              toString(e.Data["color"]),
		Icon:               toString(e.Data["icon"]),
		AssetTypeID:        toInt64(e.Data["asset_type_id"]),
		MinAssets:          toInt(e.Data["min_assets"]),
		MaxAssets:          toInt(e.Data["max_assets"]),
		HeaderItems:        headerItems,
		Blocks:             blocks,
		WorkflowStates:     states,
		Actions:            actions,
		Prefix:             toString(e.Data["prefix"]),
		Suffix:             toString(e.Data["suffix"]),
		SeqLength:          toInt(e.Data["sequence_length"]),
		ResetPolicy:        toString(e.Data["reset_policy"]),
		ConsumablesEnabled: toBool(e.Data["consumables_allowed"]),
		Consumables:        consumables,
		ExtraConsumables:   toBool(e.Data["add_any_consumables_allowed"]),
	}, nil
}

// assetTypeFromEntity extracts the asset-type information displayed alongside
// a work order template.
//
// Used by WorkOrderTemplateDetail to show the permitted asset type in the Scope card.
func assetTypeFromEntity(e *xolu.Entity) AssetTypeBasicData {
	return AssetTypeBasicData{
		ID:          e.ID,
		Name:        toString(e.Data["name"]),
		Slug:        toString(e.Data["slug"]),
		Description: toString(e.Data["description"]),
		Color:       toString(e.Data["color"]),
	}
}

// decodeWorkflowStates converts a stored workflow_states value into typed
// states. An absent value returns a nil slice without an error.
//
// Used by workOrderTemplateFromEntity to restore the workflow states in their stored order.
func decodeWorkflowStates(value any) ([]WorkflowState, error) {
	if value == nil {
		return nil, nil
	}

	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	var states []WorkflowState
	if err := json.Unmarshal(raw, &states); err != nil {
		return nil, err
	}

	return states, nil
}

// decodeHeaderItems converts a stored header_items value into typed header
// definitions. An absent value returns a nil slice without an error.
//
// Used by workOrderTemplateFromEntity to restore the header definitions used when creating and executing an order.
func decodeHeaderItems(value any) ([]HeaderItem, error) {
	if value == nil {
		return nil, nil
	}

	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	var items []HeaderItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}

	return items, nil
}

// decodeBlocks converts a stored blocks value into typed block definitions,
// including their nested lines. An absent value is accepted.
//
// Used by workOrderTemplateFromEntity to load the blocks and lines that define the execution form.
func decodeBlocks(value any) ([]TemplateBlock, error) {
	if value == nil {
		return nil, nil
	}

	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	var blocks []TemplateBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil, err
	}

	return blocks, nil
}

// decodeConsumables converts a stored consumables value into the template's
// configured consumable entries. An absent value is accepted.
//
// Used by workOrderTemplateFromEntity to load the template's consumables and suggested quantities.
func decodeConsumables(value any) ([]TemplateConsumable, error) {
	if value == nil {
		return nil, nil
	}

	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	var items []TemplateConsumable
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}

	return items, nil
}

// decodeWorkOrderTemplateActions decodes the action calls and parameter bindings
// saved with a template.
//
// UseNumber keeps numbers decoded into interface{} values as json.Number.
// This avoids introducing a float64 conversion at this decoding step, which
// also matters for asset IDs stored inside fixed_value.
//
// An absent value returns a nil slice without an error.
//
// Used by workOrderTemplateFromEntity to load action bindings and by woActionInUse to check whether a template references an action.
func decodeWorkOrderTemplateActions(value interface{}) ([]WorkOrderTemplateAction, error) {
	if value == nil {
		return nil, nil
	}

	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	var actions []WorkOrderTemplateAction

	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()

	if err := decoder.Decode(&actions); err != nil {
		return nil, err
	}

	return actions, nil
}

// =============================================================================
// FORM PARSING AND NORMALIZATION
//
// Rebuild indexed repeatables and normalize posted values before validation.
// These helpers adapt the template forms to the shared FormEngine parser;
// they do not decide whether the resulting template is valid.
// =============================================================================

// parseWorkOrderTemplateSecondStep rebuilds indexed repeatables using the
// supplied schema fields. Despite its name, it also supports first-step
// collections such as headers, blocks, workflow states, and selected actions.
//
// The shared parser drops rows whose first field is empty. For template
// validation, a partially completed row must survive so the missing first
// value can be reported and the user's remaining input can be rendered again.
//
// A temporary sentinel preserves those rows during parsing and is removed
// afterwards. The posted values are copied before modification, leaving the
// caller's form data unchanged. This keeps the adaptation local instead of
// changing the shared parser's behavior for other forms.
//
// Used by WorkOrdersTemplatesStep2 and workOrderTemplateSubmit to rebuild indexed form rows when opening the modal and submitting the template.
func parseWorkOrderTemplateSecondStep(values url.Values, fields []fe.Field) map[string]interface{} {
	posted := url.Values{}
	for key, value := range values {
		posted[key] = append([]string(nil), value...)
	}
	const emptyCell = "\x00template-empty-cell"
	var keepRows func([]fe.Field, string)
	keepRows = func(fields []fe.Field, prefix string) {
		for _, field := range fields {
			if len(field.Items) == 0 {
				continue
			}
			base := prefix + field.Name
			if prefix != "" {
				base += "]"
			}
			base += "["
			rows := map[string]bool{}
			for key, vals := range values {
				if !strings.HasPrefix(key, base) {
					continue
				}
				rest := strings.TrimPrefix(key, base)
				end := strings.Index(rest, "]")
				if end < 0 {
					continue
				}
				if index, err := strconv.Atoi(rest[:end]); err != nil || index < 0 {
					continue
				}
				for _, value := range vals {
					if strings.TrimSpace(value) != "" && !(strings.HasSuffix(key, "[stop_execution]") && (value == "false" || value == "0")) {
						rows[base+rest[:end]+"]"] = true
					}
				}
			}
			for row := range rows {
				firstKey := row + "[" + field.Items[0].Name + "]"
				if strings.TrimSpace(posted.Get(firstKey)) == "" {
					posted.Set(firstKey, emptyCell)
				}
				keepRows(field.Items, row+"[")
			}
		}
	}
	keepRows(fields, "")
	parsed := fe.ParseIndexedForm(posted, fields)
	var restore func(interface{})
	restore = func(value interface{}) {
		switch value := value.(type) {
		case map[string]interface{}:
			for key, item := range value {
				if item == emptyCell {
					value[key] = ""
				} else {
					restore(item)
				}
			}
		case []interface{}:
			for _, item := range value {
				restore(item)
			}
		}
	}
	restore(parsed)
	return parsed
}

// normalizeTemplateHeaderRequired converts posted header required flags into
// booleans in place. Values already represented as bool are preserved;
// string values "on", "true", and "1" are treated as true.
//
// Run this after parsing and before rendering or saving the header definitions,
// so those consumers receive a consistent representation.
//
// Used by WorkOrdersTemplatesStep2 and workOrderTemplateSubmit so header checkboxes are represented consistently when rendering and saving.
func normalizeTemplateHeaderRequired(items []interface{}) {
	for _, item := range items {
		row, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		if _, ok := row["required"].(bool); ok {
			continue
		}

		value := strings.ToLower(
			strings.TrimSpace(toString(row["required"])),
		)

		row["required"] = value == "on" ||
			value == "true" ||
			value == "1"
	}
}

// =============================================================================
// TEMPLATE VALIDATION
//
// Validate supported field types, block configuration, selected action calls,
// and second-step rules. Validation may normalize row values in place, but
// persistence remains the responsibility of the calling handler.
// =============================================================================

// validWOFieldType reports whether value belongs to the supported work order
// field-type catalog. Template headers, block lines, and WO Action parameters
// use the same type names so their mapping compatibility can be checked.
//
// Used for checking types in template definitions and WO Action parameters, and for matching compatible mapping sources.
func validWOFieldType(value string) bool {
	switch value {
	case "boolean", "number", "text", "asset", "gps", "date":
		return true
	default:
		return false
	}
}

// validateWorkOrderTemplateBlockResponsibleTypes requires every block to
// declare one of the supported responsible types: user, crew, or third_party.
//
// Values are trimmed in place. The first invalid block produces a collection
// error that identifies the block by name, or by position when unnamed.
// This checks the responsible type, not an individual assignee.
//
// Used by workOrderTemplateSubmit to validate the responsible type for each block before accepting the first step.
func validateWorkOrderTemplateBlockResponsibleTypes(blocks []interface{}, t func(key string, args ...any) string) map[string]string {
	errs := make(map[string]string)

	for index, item := range blocks {
		block, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		name := strings.TrimSpace(toString(block["name"]))
		if name == "" {
			name = fmt.Sprintf("#%d", index+1)
		}

		responsibleType := strings.TrimSpace(
			toString(block["responsible_type"]),
		)
		block["responsible_type"] = responsibleType

		switch responsibleType {
		case "user", "crew", "third_party":
			// Supported responsible types.

		case "":
			errs["blocks"] = t(
				"blocks.error.responsible_type_required",
				name,
			)
			return errs

		default:
			errs["blocks"] = t(
				"blocks.error.responsible_type_invalid",
				name,
			)
			return errs
		}
	}

	return errs
}

// validateWorkOrderTemplateActions validates the action calls selected in
// the first step and returns normalized rows together with collection errors.
//
// Entirely empty rows are ignored; incomplete rows are preserved for rendering.
// Each selected action must exist and be enabled. Block execution points must
// reference an existing block, whose stored spelling is restored after a
// case-insensitive match. Order-level execution points clear block_reference.
//
// Repeated calls to the same action are allowed. Their execution points and
// parameter bindings may differ, while entity checks are cached per call.
//
// Used by WorkOrdersTemplatesStep2 and workOrderTemplateSubmit to check action selections and trigger references before building the mapping plan.
func (h *UIHandler) validateWorkOrderTemplateActions(ctx context.Context, items, blocks []interface{}, t func(key string, args ...any) string,
) ([]interface{}, map[int]*xolu.Entity, map[string]string) {
	errs := make(map[string]string)
	actions := make([]interface{}, 0, len(items))
	validActions := make(map[int]*xolu.Entity)

	blockNames := make(map[string]string)
	for _, item := range blocks {
		row, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		name := strings.TrimSpace(toString(row["name"]))
		if name != "" {
			blockNames[strings.ToLower(name)] = name
		}
	}

	checkedActions := make(map[int64]*xolu.Entity)
	setError := func(message string) {
		if _, exists := errs["actions"]; !exists {
			errs["actions"] = message
		}
	}

	for _, item := range items {
		row, ok := item.(map[string]interface{})
		if !ok {
			setError(t("workorders-templates.error.invalid_action"))
			continue
		}

		actionIDText := ""
		if value := row["action"]; value != nil {
			actionIDText = strings.TrimSpace(fmt.Sprint(value))
		}
		trigger := strings.TrimSpace(toString(row["trigger_point"]))
		blockReference := strings.TrimSpace(toString(row["block_reference"]))

		if actionIDText == "" && blockReference == "" &&
			(trigger == "" || trigger == "before_work_order") {
			continue
		}

		row["action"] = actionIDText
		row["trigger_point"] = trigger
		row["block_reference"] = blockReference
		actionIndex := len(actions)
		actions = append(actions, row)

		actionID, err := strconv.ParseInt(actionIDText, 10, 64)
		if err != nil || actionID <= 0 {
			setError(t("workorders-templates.error.invalid_action"))
			continue
		}
		row["action"] = actionID

		entity, checked := checkedActions[actionID]
		if !checked {
			entity, err = h.xoluClient.Get(ctx, "wo_actions", actionID)
			if err != nil || entity == nil || !toBool(entity.Data["enabled"]) {
				entity = nil
			}
			checkedActions[actionID] = entity
		}
		if entity == nil {
			setError(t("workorders-templates.error.action_unavailable"))
			continue
		}

		switch trigger {
		case "before_work_order", "after_work_order":
			row["block_reference"] = ""
		case "before_block", "after_block":
			if blockReference == "" {
				setError(t("workorders-templates.error.action_block_required"))
				continue
			}
			name, exists := blockNames[strings.ToLower(blockReference)]
			if !exists {
				setError(t("workorders-templates.error.action_block_invalid"))
				continue
			}
			row["block_reference"] = name
		case "":
			setError(t("workorders-templates.error.missing_trigger"))
			continue
		default:
			setError(t("workorders-templates.error.invalid_option"))
			continue
		}

		validActions[actionIndex] = entity
	}

	return actions, validActions, errs
}

// workOrderTemplateConditionOperators lists the operators allowed for each
// field type. The rule validator checks this map on submit, and the modal
// uses it to filter the operator selector.
var workOrderTemplateConditionOperators = map[string][]string{
	"number":  {"equals", "not_equals", "greater_than", "less_than", "is_empty", "is_not_empty"},
	"text":    {"equals", "not_equals", "contains", "is_empty", "is_not_empty"},
	"boolean": {"equals", "not_equals", "is_empty", "is_not_empty"},
	"date":    {"equals", "not_equals", "greater_than", "less_than", "is_empty", "is_not_empty"},
	"asset":   {"equals", "not_equals", "is_empty", "is_not_empty"},
	"gps":     {"is_empty", "is_not_empty"},
}

// workOrderTemplateConditionFieldTypes maps header and block-line references
// to the data types declared in the template. The modal and the rule validator
// use this map to look up the type of a selected field.
//
// Used for populating condition controls, validating comparisons, and resolving field labels on the template detail page.
func workOrderTemplateConditionFieldTypes(headerItems, blocks []interface{}, t func(string, ...any) string) map[string]string {
	fieldTypes := make(map[string]string)
	for _, source := range templateMappingCatalog(headerItems, blocks, t) {
		if !source.Multiple {
			fieldTypes[source.Option.Value] = source.Type
		}
	}
	return fieldTypes
}

// validateWorkOrderTemplateStep2 validates rules against the submitted
// template's field and block references, then applies schema and enum checks.
//
// Rule actions must target the appropriate kind of reference: a field action
// cannot target a whole block, and a block action cannot target a field.
// Conditions, logical operators, messages, and stop_execution are also checked.
//
// The actions argument supports action-selection validation when supplied.
// Parameter bindings generated by the mapping plan are validated separately
// by readTemplateActionMappings. The returned slice contains only the actions
// supplied to this function; callers validating rules alone pass nil.
//
// Used by workOrderTemplateSubmit to validate the submitted business rules before saving the template.
func (h *UIHandler) validateWorkOrderTemplateStep2(ctx context.Context, actions, rules []interface{}, fieldTypes map[string]string, targetOptions []fe.Option, form *fe.Form, t func(key string, args ...any) string) ([]interface{}, map[string]string) {
	errs := map[string]string{}

	targetKinds := make(map[string]string, len(targetOptions))
	for _, option := range targetOptions {
		if _, exists := fieldTypes[option.Value]; exists {
			targetKinds[option.Value] = "field"
		} else {
			targetKinds[option.Value] = "block"
		}
	}

	actionTargetKinds := map[string]string{
		"show_field":    "field",
		"hide_field":    "field",
		"set_mandatory": "field",
		"set_optional":  "field",
		"show_block":    "block",
		"hide_block":    "block",
	}

	keptActions := make([]interface{}, 0, len(actions))
	checkedActions := map[int64]bool{}
	for _, item := range actions {
		action := item.(map[string]interface{})
		rawID := strings.TrimSpace(toString(action["action"]))
		// FormEngine renders one empty action row by default.
		if rawID == "" && (toString(action["trigger_point"]) == "" || toString(action["trigger_point"]) == "before_work_order") {
			continue
		}
		path := fmt.Sprintf("actions[%d].action", len(keptActions))
		if toString(action["trigger_point"]) == "" {
			errs[fmt.Sprintf("actions[%d].trigger_point", len(keptActions))] = t("workorders-templates.error.missing_trigger")
		}
		id, err := strconv.ParseInt(rawID, 10, 64)
		if err != nil || id <= 0 {
			errs[path] = t("workorders-templates.error.invalid_trigger")
		} else {
			action["action"] = id
			if !checkedActions[id] {
				if _, err := h.xoluClient.Get(ctx, "wo_actions", id); err != nil {
					errs[path] = t("workorders-templates.error.missing_action")
				} else {
					checkedActions[id] = true
				}
			}
		}
		keptActions = append(keptActions, action)
	}

	//RULES CHECK

	names := map[string]bool{}
	for i, item := range rules {
		rule := item.(map[string]interface{})
		path := fmt.Sprintf("rule_item[%d]", i)

		required := map[string]string{
			"name":         t("workorders-templates.error.rule_name_required"),
			"trigger":      t("workorders-templates.error.rule_trigger_required"),
			"action_type":  t("workorders-templates.error.rule_action_required"),
			"target_field": t("workorders-templates.error.target_required"),
		}

		for field, message := range required {
			value := strings.TrimSpace(toString(rule[field]))
			rule[field] = value

			if value == "" {
				errs[path+"."+field] = message
			}
		}

		name := toString(rule["name"])
		if name != "" {
			if names[name] {
				errs[path+".name"] = t("workorders-templates.error.same_rule_name")
			}
			names[name] = true
		}

		target := toString(rule["target_field"])
		actionType := toString(rule["action_type"])

		expectedKind, validAction := actionTargetKinds[actionType]

		if actionType != "" && !validAction {
			errs[path+".action_type"] = t("workorders-templates.error.invalid_trigger")
		}

		if target != "" {
			actualKind, validTarget := targetKinds[target]

			if !validTarget {
				errs[path+".target_field"] =
					t("workorders-templates.error.target_field_invalid")
			} else if validAction && actualKind != expectedKind {
				if expectedKind == "block" {
					errs[path+".action_type"] =
						t("workorders-templates.error.action_block")
				} else {
					errs[path+".action_type"] =
						t("workorders-templates.error.action_field")
				}
			}
		}

		trigger := toString(rule["trigger"])
		reference := strings.TrimSpace(toString(rule["trigger_reference"]))

		switch trigger {
		case "on_evaluate_field", "on_evaluate_block":
			rule["trigger_reference"] = reference
			if targetKinds[reference] == "" || workOrderTemplateTriggerForReference(reference) != trigger {
				errs[path+".trigger_reference"] = t("workorders-templates.error.trigger_reference." + trigger)
			}
		case "on_evaluate_header", "on_evaluate_workflow":
			delete(rule, "trigger_reference")
		}

		stop := strings.ToLower(toString(rule["stop_execution"]))
		rule["stop_execution"] = stop == "on" || stop == "true" || stop == "1"

		conditions, _ := rule["conditions"].([]interface{})

		if len(conditions) == 0 {
			errs[path+".conditions"] = t("workorders-templates.error.missing_condition")
		}

		for j, item := range conditions {
			condition := item.(map[string]interface{})
			conditionPath := fmt.Sprintf("%s.conditions[%d]", path, j)

			// field := strings.TrimSpace(toString(condition["field"]))
			// operator := strings.TrimSpace(toString(condition["operator"]))
			// comparison := strings.TrimSpace(toString(condition["comparison_value"]))
			// logical := strings.TrimSpace(toString(condition["logical_operator_next"]))

			// if field == "" {
			// 	errs[conditionPath+".field"] = t("workorders-templates.error.field_not_selected")
			// } else {
			// 	checkReference(conditionPath+".field", field)
			// }

			// if operator == "" {
			// 	errs[conditionPath+".operator"] = t("workorders-templates.error.operator_not_selected")
			// }

			// // is_empty and is_not_empty don't need values. Other options do.
			// if operator != "" && operator != "is_empty" && operator != "is_not_empty" && comparison == "" {
			// 	errs[conditionPath+".comparison_value"] = t("workorders-templates.error.comparison_not_selected")
			// }

			field := strings.TrimSpace(toString(condition["field"]))
			operator := strings.TrimSpace(toString(condition["operator"]))
			comparison := toString(condition["comparison_value"])
			logical := strings.TrimSpace(toString(condition["logical_operator_next"]))

			fieldType, fieldExists := fieldTypes[field]
			switch {
			case field == "":
				errs[conditionPath+".field"] = t("workorders-templates.error.field_not_selected")
			case !fieldExists:
				errs[conditionPath+".field"] = t("workorders-templates.error.missing_field")
			}

			switch {
			case operator == "":
				errs[conditionPath+".operator"] = t("workorders-templates.error.operator_not_selected")

			case fieldExists && !slices.Contains(workOrderTemplateConditionOperators[fieldType], operator):
				errs[conditionPath+".operator"] = t(
					"workorders-templates.error.operator_for_field_type",
					workOrderTemplateFieldTypeLabel(fieldType, t),
				)

			case operator == "is_empty" || operator == "is_not_empty":
				delete(condition, "comparison_value")

			case fieldExists && strings.TrimSpace(comparison) == "":
				errs[conditionPath+".comparison_value"] = t("workorders-templates.error.comparison_not_selected")

			case fieldExists:
				if _, err := h.parseTemplateMappingFixedValue(ctx, fieldType, comparison); err != nil {
					errs[conditionPath+".comparison_value"] = t(
						"workorders-templates.error.comparison_for_field_type",
						workOrderTemplateFieldTypeLabel(fieldType, t),
					)
				} else if fieldType != "text" {
					condition["comparison_value"] = strings.TrimSpace(comparison)
				}
			}

			if j < len(conditions)-1 {
				if logical != "And" && logical != "Or" {
					errs[conditionPath+".logical_operator_next"] = t("workorders-templates.error.logical_operator_not_selected")
				}
			} else if logical != "" && logical != "None" {
				errs[conditionPath+".logical_operator_next"] = t("workorders-templates.error.invalid_logical_operator")
			}
		}

		message := strings.TrimSpace(toString(rule["message_text"]))
		severity := strings.TrimSpace(toString(rule["message_type"]))

		if message != "" && severity == "" {
			errs[path+".message_type"] = t("workorders-templates.error.message_type_required")
		}

		if severity != "" && message == "" {
			errs[path+".message_text"] = t("workorders-templates.error.message_text_required")
		}
	}

	engine := &fe.FormEngine{Translator: h.formEngineTranslator()}
	rc := fe.RenderContext{Module: "workorders-templates", Form: "workorders-template-step-two", Locale: h.currentLocale()}
	schemaErrors, statuses := validateAssetValues(engine, rc, map[string]interface{}{"actions": keptActions, "rule_item": rules}, form.Compiled)
	for path, message := range schemaErrors {
		errs[path] = message
	}

	for _, status := range statuses {
		if _, exists := errs[status.Element]; !exists {
			errs[status.Element] = status.Message
		}
	}
	validateWorkOrderTemplateEnums(form.Fields, map[string]interface{}{"actions": keptActions, "rule_item": rules}, "", errs, t)
	return keptActions, errs
}

var workOrderTemplateErrorPath = regexp.MustCompile(
	`^(rule_item|actions)\[(\d+)\](?:\.conditions\[(\d+)\])?(?:\.([a-zA-Z_][a-zA-Z0-9_]*))?$`,
)

// workOrderTemplateErrorLabel converts an indexed validation path into a
// translated label with one-based rule, action, and condition numbers.
//
// Used by workOrderTemplateErrorSummary to identify the rule, action, or condition that needs correction.
func workOrderTemplateErrorLabel(path string, t func(key string, args ...any) string) string {
	match := workOrderTemplateErrorPath.FindStringSubmatch(path)

	if match == nil {
		key := "workorders-templates.forms.workorders-template.field." + path
		if label := t(key); label != key {
			return label
		}

		return t("workorders-templates.validation.general")
	}

	row, _ := strconv.Atoi(match[2])
	parent := match[1]

	labelKey := "workorders-templates.validation.rule"
	if parent == "actions" {
		labelKey = "workorders-templates.validation.action"
	}

	parts := []string{t(labelKey, row+1)}

	if match[3] != "" {
		condition, _ := strconv.Atoi(match[3])
		parts = append(parts, t("workorders-templates.validation.condition", condition+1))
		parent = "conditions"
	}

	if match[4] != "" {
		key := "repeatable_item." + parent + "." + match[4]
		if label := t(key); label != key {
			parts = append(parts, label)
		}
	}

	return strings.Join(parts, " · ")
}

// workOrderTemplateErrorSummary renders the second-step validation card.
// Each entry shows the translated location and message for one error.
// Indexed rules, actions, and conditions are ordered by row number.
//
// Used by workOrderTemplateSubmit to show validation errors when re-rendering the second-step modal.
func workOrderTemplateErrorSummary(errs map[string]string, t func(key string, args ...any) string) mi.H {
	return func(b *mi.Builder) mi.Node {
		if len(errs) == 0 {
			return b.Span()
		}

		keys := make([]string, 0, len(errs))
		for key := range errs {
			keys = append(keys, key)
		}

		// Keep rows in numeric order: rule 2 before rule 10.
		sort.Slice(keys, func(i, j int) bool {
			a := workOrderTemplateErrorPath.FindStringSubmatch(keys[i])
			c := workOrderTemplateErrorPath.FindStringSubmatch(keys[j])

			if a != nil && c != nil && a[1] == c[1] {
				rowA, _ := strconv.Atoi(a[2])
				rowC, _ := strconv.Atoi(c[2])
				if rowA != rowC {
					return rowA < rowC
				}

				if a[3] != "" && c[3] != "" {
					conditionA, _ := strconv.Atoi(a[3])
					conditionC, _ := strconv.Atoi(c[3])
					if conditionA != conditionC {
						return conditionA < conditionC
					}
				}
			}

			return keys[i] < keys[j]
		})

		items := []interface{}{mi.Class("space-y-2")}

		for _, key := range keys {
			items = append(items, b.Li(
				mi.Class("rounded-lg bg-white dark:bg-gray-800 p-3"),
				b.P(
					mi.Class("text-sm font-semibold text-gray-900 dark:text-gray-100"),
					workOrderTemplateErrorLabel(key, t),
				),
				b.P(
					mi.Class("mt-1 text-sm text-red-700 dark:text-red-300"),
					errs[key],
				),
			))
		}

		return b.Div(
			mi.Attr("role", "alert"),
			mi.Class("rounded-lg border border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900/20 p-4"),
			b.Div(
				mi.Class("flex items-center gap-2"),
				b.Span(
					mi.Attr("aria-hidden", "true"),
					mi.Class("material-icons text-red-600 dark:text-red-400"),
					"error_outline",
				),
				b.P(
					mi.Class("text-sm font-semibold text-red-800 dark:text-red-200"),
					t("workorders-templates.validation.title"),
				),
			),
			b.P(
				mi.Class("mt-1 mb-3 text-sm text-red-700 dark:text-red-300"),
				t("workorders-templates.validation.hint"),
			),
			b.Ul(items...),
		)
	}
}

// validateWorkOrderTemplateEnums recursively checks non-empty values against
// the schema's static enums, including fields inside nested repeatables.
//
// Errors use indexed field paths so they can be associated with the submitted
// row. Empty values are left to required-field validation; dynamic source
// membership is checked by the relevant action or rule validator.
//
// Used by validateWorkOrderTemplateStep2 to check that posted values belong to the schema's static option lists.
func validateWorkOrderTemplateEnums(fields []fe.Field, values map[string]interface{}, prefix string, errs map[string]string, t func(key string, args ...any) string) {
	for _, field := range fields {
		path := prefix + field.Name
		value := values[field.Name]
		if rows, ok := value.([]interface{}); ok {
			for i, item := range rows {
				if row, ok := item.(map[string]interface{}); ok {
					validateWorkOrderTemplateEnums(field.Items, row, fmt.Sprintf("%s[%d].", path, i), errs, t)
				}
			}
		}
		text := toString(value)
		if text == "" || len(field.Enum) == 0 {
			continue
		}
		valid := false
		for _, option := range field.Enum {
			if text == option {
				valid = true
				break
			}
		}
		if !valid {
			errs[path] = t("workorders-templates.error.invalid_option")
		}
	}
}

// =============================================================================
// RULE FIELD AND TARGET OPTIONS
//
// Build the reference lists used by second-step rules. Conditions select
// individual fields; rule actions may target individual fields or whole blocks.
//
// These lists describe the template structure. Action-parameter compatibility
// and execution-point availability are handled by the mapping catalog below.
// =============================================================================

// workOrderTemplateSources supplies request-specific options derived from the
// submitted template, allowing FormEngine to render rule controls without
// querying a persisted template.
type workOrderTemplateSources struct {
	fields  []fe.Option
	targets []fe.Option
	t       func(key string, args ...any) string
}

// workOrderTemplateRulesDetails converts stored rules into display data,
// resolving field and target references to readable labels. Used by
// WorkOrderTemplateDetail to prepare the Business Rules card.
//
// Used by WorkOrderTemplateDetail to prepare the saved rules for the Business Rules card.
func workOrderTemplateRulesDetails(data map[string]any, t func(string, ...any) string) []WorkOrderTemplateRulesDetail {
	rows, _ := data["rule_item"].([]any)
	if len(rows) == 0 {
		return nil
	}

	headers, _ := data["header_items"].([]any)
	blocks, _ := data["blocks"].([]any)
	fields := workOrderTemplateFieldOptions(headers, blocks, t)
	fieldTypes := workOrderTemplateConditionFieldTypes(headers, blocks, t)

	conditionLabels := make(map[string]string, len(fields))
	for _, option := range workOrderTemplateConditionOptions(fields, fieldTypes, t) {
		conditionLabels[option.Value] = option.Label
	}

	targetLabels := make(map[string]string)
	for _, option := range workOrderTemplateTargetOptions(fields, blocks, t) {
		targetLabels[option.Value] = option.Label
	}

	referenceLabel := func(labels map[string]string, ref string) string {
		if label := labels[ref]; label != "" {
			return label
		}
		if ref == "" {
			return "—"
		}
		return ref
	}

	rules := make([]WorkOrderTemplateRulesDetail, 0, len(rows))
	for _, item := range rows {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}

		trigger := t("workorders-template-step-two.trigger." + toString(row["trigger"]))
		if ref := toString(row["trigger_reference"]); ref != "" {
			trigger += " (" + referenceLabel(targetLabels, ref) + ")"
		}

		rule := WorkOrderTemplateRulesDetail{
			Name:          toString(row["name"]),
			Trigger:       trigger,
			Action:        t("workorders-template-step-two.action_type." + toString(row["action_type"])),
			Target:        referenceLabel(targetLabels, toString(row["target_field"])),
			StopExecution: toBool(row["stop_execution"]),
			Severity:      toString(row["message_type"]),
			Message:       toString(row["message_text"]),
		}

		conditions, _ := row["conditions"].([]any)
		for _, item := range conditions {
			condition, ok := item.(map[string]any)
			if !ok {
				continue
			}

			field := referenceLabel(conditionLabels, toString(condition["field"]))
			operator := toString(condition["operator"])
			rule.Conditions = append(rule.Conditions, WorkOrderTemplateRuleConditionDetail{
				Expression: templateRuleExpression(field, operator, toString(condition["comparison_value"]), t),
				Connector:  toString(condition["logical_operator_next"]),
			})
		}
		rules = append(rules, rule)
	}

	return rules
}

// templateRuleExpression formats a rule condition for display using its
// field label, operator, and comparison value when applicable. Used by
// workOrderTemplateRulesDetails to build the condition text for each rule.
//
// Used by workOrderTemplateRulesDetails to turn each saved comparison into readable text for the detail page.
func templateRuleExpression(field, operator, value string, t func(string, ...any) string) string {
	switch operator {
	case "equals":
		return field + " = " + value
	case "not_equals":
		return field + " ≠ " + value
	case "greater_than":
		return field + " > " + value
	case "less_than":
		return field + " < " + value
	case "contains":
		return field + " " + strings.ToLower(t("workorders-template-step-two.operator.contains")) + " '" + value + "'"
	case "is_empty", "is_not_empty":
		return field + " " + strings.ToLower(t("workorders-template-step-two.operator."+operator))
	default:
		return field + " " + strings.ToLower(t("workorders-template-step-two.operator."+operator)) + " " + value
	}
}

// workOrderTemplateFieldOptions builds references to named header fields and
// block lines. Duplicate reference values are omitted.
//
// Names are URL-escaped inside the reference components so separators in a
// field name do not become structural separators. Display labels remain
// readable. Rendering and validation must use the same reference format.
//
// Used for building the field choices shown in the modal and resolving the same references during validation and display.
func workOrderTemplateFieldOptions(headerItems, blocks []interface{}, t func(key string, args ...any) string) []fe.Option {
	options := make([]fe.Option, 0)
	seen := make(map[string]bool)

	addOption := func(value, label string) {
		if seen[value] {
			return
		}
		seen[value] = true
		options = append(options, fe.Option{
			Value: value,
			Label: label,
		})
	}

	for _, item := range headerItems {
		field, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		tag := strings.TrimSpace(toString(field["tag"]))
		if tag == "" {
			continue
		}

		addOption(
			"header:"+url.QueryEscape(tag),
			t("workorders-templates.option.header", tag),
		)
	}

	for _, item := range blocks {
		block, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		blockName := strings.TrimSpace(toString(block["name"]))
		if blockName == "" {
			continue
		}

		lines, _ := block["lines"].([]interface{})
		for _, item := range lines {
			line, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			lineName := strings.TrimSpace(toString(line["name"]))
			if lineName == "" {
				continue
			}

			addOption(
				"block:"+url.QueryEscape(blockName)+
					":line:"+url.QueryEscape(lineName),
				blockName+" · "+lineName,
			)
		}
	}

	return options
}

// workOrderTemplateConditionOptions copies the field options and appends the
// translated data type to the label of each field with a known type.
//
// Used for labeling the condition field selector and displaying saved condition references in rule details.
func workOrderTemplateConditionOptions(options []fe.Option, fieldTypes map[string]string, t func(string, ...any) string) []fe.Option {
	labeled := slices.Clone(options)
	for i := range labeled {
		fieldType := fieldTypes[labeled[i].Value]
		if fieldType != "" {
			labeled[i].Label += " (" + workOrderTemplateFieldTypeLabel(fieldType, t) + ")"
		}
	}
	return labeled
}

// workOrderTemplateTargetOptions adds whole-block references to the field
// options without modifying the supplied slice.
//
// Field references retain their existing format; whole blocks use
// block:<escaped-name>. Validation uses these lists to distinguish target kinds.
//
// Used for populating rule targets and trigger references, validating submissions, and resolving stored target labels.
func workOrderTemplateTargetOptions(fields []fe.Option, blocks []interface{}, t func(key string, args ...any) string) []fe.Option {
	targets := append([]fe.Option{}, fields...)
	seen := make(map[string]bool)

	for _, option := range fields {
		seen[option.Value] = true
	}

	for _, item := range blocks {
		block, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		name := strings.TrimSpace(toString(block["name"]))
		if name == "" {
			continue
		}

		value := "block:" + url.QueryEscape(name)
		if seen[value] {
			continue
		}

		seen[value] = true
		targets = append(targets, fe.Option{
			Value: value,
			Label: t("workorders-templates.option.block", name),
		})
	}

	return targets
}

// Resolve implements formengine.SourceResolver for rule field and target lists.
// Unknown source names return an error instead of an unrelated option list.
//
// Used by FormEngine to populate rule selectors with the fields, targets, and trigger references available for the current template.
func (s workOrderTemplateSources) Resolve(_ context.Context, source string) ([]fe.Option, error) {
	switch source {
	case "request:template-fields":
		return s.fields, nil
	case "request:template-targets":
		return s.targets, nil
	case "request:template-rule-trigger-references":
		options := make([]fe.Option, 0, len(s.targets))
		for _, option := range s.targets {
			if workOrderTemplateTriggerForReference(option.Value) != "" {
				options = append(options, option)
			}
		}
		return options, nil
	default:
		return nil, fmt.Errorf("%s", s.t("workorders-templates.error.unsupported_source", source))
	}
}

// workOrderTemplateTriggerForReference returns the trigger associated with a
// block-line or whole-block reference. It returns an empty string for other
// references.
//
// Used by workOrderTemplateSources.Resolve to filter trigger references and by validateWorkOrderTemplateStep2 to check the selected reference.
func workOrderTemplateTriggerForReference(reference string) string {
	if !strings.HasPrefix(reference, "block:") {
		return ""
	}
	if strings.Contains(reference, ":line:") {
		return "on_evaluate_field"
	}
	return "on_evaluate_block"
}

// =============================================================================
// ACTION MAPPING PLAN AND GENERATED FORM
//
// Build a request-specific extension of the base second-step schema from the
// selected WO Actions and their parameter definitions.
//
// The plan connects generated control names with action positions, parameter
// keys, and compatible context sources. Rebuilding it during submission lets
// validation use server-generated options rather than trusting posted choices.
// =============================================================================

// templateMappingSource describes one possible context value, including its
// reference, display label, data type, cardinality, and availability metadata.
//
// Catalog membership alone does not make a source selectable. The plan builder
// also checks parameter compatibility and execution-point availability.
type templateMappingSource struct {
	Option     fe.Option
	Type       string
	Multiple   bool
	Block      string
	AtCreation bool
}

// templateParameterMapping connects a generated field name to one parameter
// of one action call.
//
// ActionIndex identifies the call's position, allowing the same WO Action to
// appear several times without sharing bindings. SourceMultiple records each
// allowed context source's cardinality for scalar-to-list conversion.
type templateParameterMapping struct {
	Field          string
	ActionIndex    int
	Parameter      WOActionParameter
	Options        []fe.Option
	SourceMultiple map[string]bool
}

// templateActionMappingPlan contains the generated second-step schema and
// the metadata needed to validate its submitted mapping controls.
//
// Schema describes the temporary form. Fields connects that form to the
// mapping_values payload saved with each selected action.
type templateActionMappingPlan struct {
	Schema []byte
	Fields []templateParameterMapping
}

// templateActionMappingSources extends the rule source resolver with mapping
// origins, boolean choices, and parameter-specific context options.
type templateActionMappingSources struct {
	base    workOrderTemplateSources
	options map[string][]fe.Option
}

// buildTemplateActionMappingPlan extends the base second-step schema with
// mapping controls for each parameter of each selected action call.
//
// Each parameter receives an origin selector, a context-field selector, and
// a fixed-value control. Context options must match the parameter type and
// be available at the configured execution point. Scalar sources may supply
// multiple-value parameters; multiple-value sources cannot supply scalars.
//
// Generated names include the action and parameter positions, keeping repeated
// calls independent. Rendering and submission must preserve that action order.
//
// The existing rule tab is retained. The Actions tab is added only when at
// least one parameter needs mapping; actions without parameters add no section.
// json.RawMessage preserves the property order inside existing schema content.
//
// Used by WorkOrdersTemplatesStep2 to generate mapping controls and by workOrderTemplateSubmit to rebuild the same plan for validation.
func (h *UIHandler) buildTemplateActionMappingPlan(ctx context.Context, actions []interface{}, validActions map[int]*xolu.Entity, headerItems, blocks []interface{}, t func(key string, args ...any) string,
) (templateActionMappingPlan, error) {
	plan := templateActionMappingPlan{}

	// RawMessage preserves the property order inside the existing rule schema.
	var document struct {
		Type       string                     `json:"type"`
		Title      string                     `json:"title"`
		Form       map[string]json.RawMessage `json:"x-seam-form"`
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}

	if err := json.Unmarshal(
		resolveSchema(ctx, "workorders_templates", "step-2", workordertemplateschemas.Schema("step-2")),
		&document,
	); err != nil {
		return plan, err
	}

	if document.Form == nil || document.Properties == nil {
		return plan, fmt.Errorf("invalid template second-step schema")
	}

	delete(document.Properties, "actions")

	catalog := templateMappingCatalog(headerItems, blocks, t)
	cache := make(map[int64]WOAction)
	sections := make([]map[string]interface{}, 0)

	for actionIndex, item := range actions {
		entity := validActions[actionIndex]
		if entity == nil {
			continue
		}
		row := item.(map[string]interface{})
		actionID := entity.ID

		action, found := cache[actionID]
		if !found {
			parameters, err := decodeWOActionParameters(entity.Data["parameters"])
			if err != nil {
				return plan, fmt.Errorf(
					"decode action %d parameters: %w",
					actionID,
					err,
				)
			}

			action = WOAction{
				ID:         entity.ID,
				Name:       toString(entity.Data["name"]),
				Parameters: parameters,
			}
			cache[actionID] = action
		}

		trigger := toString(row["trigger_point"])
		blockReference := toString(row["block_reference"])

		fieldNames := make([]string, 0, len(action.Parameters))
		seenKeys := make(map[string]bool)

		for parameterIndex, parameter := range action.Parameters {
			if parameter.Key == "" ||
				seenKeys[parameter.Key] ||
				!validWOFieldType(parameter.Type) {
				return plan, fmt.Errorf(
					"invalid parameter definition in action %d",
					actionID,
				)
			}
			seenKeys[parameter.Key] = true

			fieldName := fmt.Sprintf(
				"action_mapping_%d_%d",
				actionIndex,
				parameterIndex,
			)

			options := make([]fe.Option, 0)
			sourceMultiple := make(map[string]bool)

			for _, source := range catalog {
				// A scalar may supply a list, but a list cannot supply a scalar.
				if source.Type != parameter.Type || (source.Multiple && !parameter.Multiple) {
					continue
				}

				if !templateMappingSourceAvailable(source, trigger, blockReference) {
					continue
				}

				option := source.Option

				if parameter.Multiple && !source.Multiple {
					option.Label += " · " + t(
						"workorders-templates.mapping.as_single_item_list",
					)
				}

				options = append(options, option)
				sourceMultiple[option.Value] = source.Multiple
			}

			requiredLabel := t("workorders-templates.mapping.optional")
			if parameter.Required {
				requiredLabel = t("workorders-templates.mapping.required")
			}

			multipleLabel := t("workorders-templates.mapping.single")
			if parameter.Multiple {
				multipleLabel = t("workorders-templates.mapping.multiple")
			}

			help := fmt.Sprintf(
				"%s · %s · %s",
				t("wo-action.type."+parameter.Type),
				requiredLabel,
				multipleLabel,
			)

			if len(options) == 0 {
				help += ". " + t("workorders-templates.mapping.no_sources")
			}

			title := parameter.Name
			if title == "" {
				title = parameter.Key
			}

			sourceField := fieldName + "_source"
			fixedField := fieldName + "_fixed"

			fixedSchema := templateMappingFixedSchema(parameter, t)
			fixedSchema["x-seam-visible-when"] = map[string]interface{}{
				"field": sourceField,
				"op":    "eq",
				"value": "fixed",
			}

			properties := map[string]interface{}{
				sourceField: map[string]interface{}{
					"type":          "string",
					"title":         title,
					"x-seam-input":  "select",
					"x-seam-source": "request:template-mapping-origins",
					"x-seam-help":   help,
				},
				fieldName: map[string]interface{}{
					"type":          "string",
					"title":         t("workorders-templates.mapping.context_field"),
					"x-seam-input":  "select",
					"x-seam-source": "request:" + fieldName,
					"x-seam-visible-when": map[string]interface{}{
						"field": sourceField,
						"op":    "eq",
						"value": "context",
					},
				},
				fixedField: fixedSchema,
			}

			for propertyName, definition := range properties {
				property, err := json.Marshal(definition)
				if err != nil {
					return plan, err
				}

				document.Properties[propertyName] = property
			}

			fieldNames = append(
				fieldNames,
				sourceField,
				fieldName,
				fixedField,
			)

			plan.Fields = append(plan.Fields, templateParameterMapping{
				Field:          fieldName,
				ActionIndex:    actionIndex,
				Parameter:      parameter,
				Options:        options,
				SourceMultiple: sourceMultiple,
			})
		}

		// Actions without parameters do not need a mapping section.
		if len(fieldNames) == 0 {
			continue
		}

		label := fmt.Sprintf(
			"%d. %s · %s",
			actionIndex+1,
			action.Name,
			t("workorders-template.trigger_point."+trigger),
		)
		if blockReference != "" {
			label += " · " + blockReference
		}

		sections = append(sections, map[string]interface{}{
			"id":     "action-mapping-" + strconv.Itoa(actionIndex),
			"label":  label,
			"layout": "2-col",
			"fields": fieldNames,
		})
	}

	var tabs []json.RawMessage
	if err := json.Unmarshal(document.Form["tabs"], &tabs); err != nil {
		return plan, err
	}

	keptTabs := make([]json.RawMessage, 0, len(tabs)+1)

	if len(sections) > 0 {
		actionTab, err := json.Marshal(map[string]interface{}{
			"id":       "tab-step2-actions",
			"label":    "workorders-templates.forms.tab.actions",
			"sections": sections,
		})
		if err != nil {
			return plan, err
		}

		keptTabs = append(keptTabs, actionTab)
	}

	for _, rawTab := range tabs {
		var tab struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(rawTab, &tab); err != nil {
			return plan, err
		}

		if tab.ID != "tab-step2-actions" {
			keptTabs = append(keptTabs, rawTab)
		}
	}

	tabsJSON, err := json.Marshal(keptTabs)
	if err != nil {
		return plan, err
	}
	document.Form["tabs"] = tabsJSON

	plan.Schema, err = json.Marshal(document)
	return plan, err
}

// templateMappingCatalog builds candidate context sources from work-order
// assets, header definitions, and block lines.
//
// It records types, cardinality, block ownership, and whether a header is filled
// upon creation. Parameter-specific filtering happens in the plan builder.
//
// Used by buildTemplateActionMappingPlan to find candidate context sources and by workOrderTemplateConditionFieldTypes to identify their types.
func templateMappingCatalog(headerItems, blocks []interface{}, t func(key string, args ...any) string) []templateMappingSource {
	sources := []templateMappingSource{
		{
			Option: fe.Option{
				Value: "work_order.assets",
				Label: t("workorders-templates.mapping.wo_assets"),
			},
			Type:       "asset",
			Multiple:   true,
			AtCreation: true,
		},
	}

	for _, item := range headerItems {
		row, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		tag := strings.TrimSpace(toString(row["tag"]))
		if tag == "" {
			continue
		}

		sources = append(sources, templateMappingSource{
			Option: fe.Option{
				Value: "header:" + url.QueryEscape(tag),
				Label: t("workorders-templates.option.header", tag),
			},
			Type:       toString(row["type"]),
			AtCreation: toString(row["to_fill_out"]) == "upon_creating",
		})
	}

	for _, item := range blocks {
		block, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		blockName := strings.TrimSpace(toString(block["name"]))
		if blockName == "" {
			continue
		}

		lines, _ := block["lines"].([]interface{})

		for _, item := range lines {
			line, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			lineName := strings.TrimSpace(toString(line["name"]))
			if lineName == "" {
				continue
			}

			sources = append(sources, templateMappingSource{
				Option: fe.Option{
					Value: "block:" + url.QueryEscape(blockName) +
						":line:" + url.QueryEscape(lineName),
					Label: blockName + " · " + lineName,
				},
				Type:  toString(line["type"]),
				Block: blockName,
			})
		}
	}

	return sources
}

// templateMappingSourceAvailable checks availability at an execution point.
//
// Before the work order, only creation-time header values and other catalog
// sources marked AtCreation are accepted. At later points, header sources
// are accepted. Block lines are accepted after their own block or after the
// whole work order.
//
// Availability does not currently follow predecessor relationships. A line
// from another block is therefore not accepted merely because that block
// appears earlier in the template.
//
// Used by buildTemplateActionMappingPlan to offer only context sources available at the selected action trigger.
func templateMappingSourceAvailable(source templateMappingSource, trigger, blockReference string) bool {
	if source.Block != "" {
		return trigger == "after_work_order" ||
			(trigger == "after_block" &&
				strings.EqualFold(source.Block, blockReference))
	}

	if trigger == "before_work_order" {
		return source.AtCreation
	}

	return true
}

// templateMappingFixedSchema builds the input schema for a parameter's fixed
// value, selecting the appropriate widget for its declared data type.
//
// Multiple-value parameters wrap that control in an indexed repeatable with
// one "value" field per row. This preserves row ownership when reading lists
// of fixed values and initializes row widgets through the indexed renderer.
//
// These controls describe posted input. Conversion into typed values happens
// separately in parseTemplateMappingFixedValue.
//
// Used by buildTemplateActionMappingPlan to choose the input for each fixed parameter value, including list values.
func templateMappingFixedSchema(parameter WOActionParameter, t func(key string, args ...any) string) map[string]interface{} {
	title := t("workorders-templates.mapping.fixed_value")

	valueSchema := map[string]interface{}{
		"type":         "string",
		"title":        title,
		"x-seam-input": "text",
	}

	switch parameter.Type {
	case "boolean":
		valueSchema["x-seam-input"] = "select"
		valueSchema["x-seam-source"] = "request:template-mapping-booleans"

	case "number":
		valueSchema["format"] = "decimal"
		valueSchema["decimalPrecision"] = 18
		valueSchema["decimalScale"] = 9
		valueSchema["x-seam-input"] = "decimal"

	case "asset":
		valueSchema["type"] = "integer"
		valueSchema["x-seam-input"] = "any-picker"
		valueSchema["x-seam-source"] = "entity:assets"

	case "gps":
		valueSchema["x-seam-input"] = "gps"

	case "date":
		valueSchema["format"] = "date"
		valueSchema["x-seam-input"] = "date"
	}

	if !parameter.Multiple {
		return valueSchema
	}

	return map[string]interface{}{
		"type":          "array",
		"title":         title,
		"x-seam-input":  "repeatable",
		"x-seam-naming": "indexed",
		"x-seam-layout": "form",
		"items": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"value": valueSchema,
			},
		},
	}
}

// sourceResolver builds the request-specific option lists referenced by the
// generated schema: mapping origins, boolean values, and each parameter's
// compatible context fields. Rule sources remain available through the base
// resolver.
//
// Used when opening or re-rendering the second-step modal to supply FormEngine with the mapping and rule options for that request.
func (p templateActionMappingPlan) sourceResolver(base workOrderTemplateSources) templateActionMappingSources {
	options := make(map[string][]fe.Option, len(p.Fields)+2)

	options["request:template-mapping-origins"] = []fe.Option{
		{
			Value: "context",
			Label: base.t("workorders-templates.mapping.from_context"),
		},
		{
			Value: "fixed",
			Label: base.t("workorders-templates.mapping.from_fixed"),
		},
	}

	options["request:template-mapping-booleans"] = []fe.Option{
		{Value: "true", Label: base.t("true")},
		{Value: "false", Label: base.t("false")},
	}

	for _, field := range p.Fields {
		options["request:"+field.Field] = field.Options
	}

	return templateActionMappingSources{
		base:    base,
		options: options,
	}
}

// Resolve returns a mapping-specific option list when one is registered,
// otherwise delegating to the base rule source resolver.
//
// Used by FormEngine to look up options for generated mapping controls and to delegate rule options to the base resolver.
func (s templateActionMappingSources) Resolve(ctx context.Context, source string) ([]fe.Option, error) {
	if options, found := s.options[source]; found {
		return options, nil
	}

	return s.base.Resolve(ctx, source)
}

// initialValues selects context mapping as the initial origin for each
// parameter. It leaves the actual context reference and fixed value unset;
// required parameters still need a valid assignment before saving.
//
// Used by WorkOrdersTemplatesStep2 to preselect context mapping when the modal first opens.
func (p templateActionMappingPlan) initialValues() map[string]interface{} {
	values := make(map[string]interface{}, len(p.Fields))

	for _, field := range p.Fields {
		values[field.Field+"_source"] = "context"
	}

	return values
}

// =============================================================================
// SUBMITTED ACTION BINDINGS
//
// Convert the generated form's posted values into stored parameter bindings.
// Keep the original control values separately so validation failures can render
// the user's input without replacing it with the normalized storage payload.
// =============================================================================

// readTemplateActionMappings validates submitted mapping controls against the
// rebuilt plan and writes mapping_values into the supplied action rows.
//
// Context references must belong to that parameter's allowed options. A scalar
// context source assigned to a multiple-value parameter stores wrap_in_array;
// the actual context value is resolved and converted during action execution.
//
// Fixed values are converted according to the parameter type. Multiple fixed
// values are read from indexed repeatable rows. Unassigned optional parameters
// produce no binding, while unassigned required parameters produce errors.
//
// The returned values preserve form input for rendering. Action rows are
// updated even when errors exist, so callers must check the error map before
// persisting the template.
//
// Used by workOrderTemplateSubmit to turn the posted mapping controls into validated bindings and retain input when corrections are needed.
func (h *UIHandler) readTemplateActionMappings(ctx context.Context, posted url.Values, parsed map[string]interface{}, actions []interface{}, plan templateActionMappingPlan,
	t func(key string, args ...any) string,
) (map[string]interface{}, map[string]string) {
	values := make(map[string]interface{})
	errs := make(map[string]string)
	bindings := make([][]interface{}, len(actions))

	for i := range bindings {
		bindings[i] = make([]interface{}, 0)
	}

	for _, field := range plan.Fields {
		sourceField := field.Field + "_source"
		fixedField := field.Field + "_fixed"

		source := strings.TrimSpace(posted.Get(sourceField))
		selected := strings.TrimSpace(posted.Get(field.Field))

		values[sourceField] = source
		values[field.Field] = selected

		var fixedRaw interface{} = posted.Get(fixedField)

		if field.Parameter.Multiple {
			fixedRaw = parsed[fixedField]
			if fixedRaw == nil {
				fixedRaw = []interface{}{}
			}
		}

		// Preserve form values independently of their normalized payload.
		values[fixedField] = fixedRaw

		switch source {
		case "":
			if field.Parameter.Required {
				errs[sourceField] = t("workorders-templates.error.mapping_required")
			}

		case "context":
			if selected == "" {
				if field.Parameter.Required {
					errs[field.Field] = t("workorders-templates.error.mapping_required")
				}
				continue
			}

			valid := false
			for _, option := range field.Options {
				if option.Value == selected {
					valid = true
					break
				}
			}

			if !valid {
				errs[field.Field] = t("workorders-templates.error.invalid_mapping")
				continue
			}

			binding := map[string]interface{}{
				"parameter":    field.Parameter.Key,
				"source":       "context",
				"context_path": selected,
			}

			if field.Parameter.Multiple &&
				!field.SourceMultiple[selected] {
				binding["transform"] = "wrap_in_array"
			}

			bindings[field.ActionIndex] = append(
				bindings[field.ActionIndex],
				binding,
			)

		case "fixed":
			rawValues := make([]string, 0)
			valid := true

			if field.Parameter.Multiple {
				rows, ok := fixedRaw.([]interface{})
				if !ok {
					valid = false
				} else {
					for _, item := range rows {
						row, ok := item.(map[string]interface{})
						if !ok {
							valid = false
							break
						}

						text, ok := row["value"].(string)
						if !ok {
							valid = false
							break
						}

						rawValues = append(rawValues, text)
					}
				}
			} else {
				text := posted.Get(fixedField)
				if strings.TrimSpace(text) != "" {
					rawValues = append(rawValues, text)
				}
			}

			if !valid {
				errs[fixedField] = t("workorders-templates.error.fixed_value_invalid")
				continue
			}

			if len(rawValues) == 0 {
				if field.Parameter.Required {
					errs[fixedField] = t("workorders-templates.error.mapping_required")
				}
				continue
			}

			fixedValues := make([]interface{}, 0, len(rawValues))

			for _, raw := range rawValues {
				value, err := h.parseTemplateMappingFixedValue(
					ctx,
					field.Parameter.Type,
					raw,
				)
				if err != nil {
					valid = false
					errs[fixedField] = t("workorders-templates.error.fixed_value_invalid")
					break
				}

				fixedValues = append(fixedValues, value)
			}

			if !valid {
				continue
			}

			var fixedValue interface{} = fixedValues[0]
			if field.Parameter.Multiple {
				fixedValue = fixedValues
			}

			bindings[field.ActionIndex] = append(
				bindings[field.ActionIndex],
				map[string]interface{}{
					"parameter":   field.Parameter.Key,
					"source":      "fixed",
					"fixed_value": fixedValue,
				},
			)

		default:
			errs[sourceField] = t(
				"workorders-templates.error.invalid_mapping",
			)
		}
	}

	for i, item := range actions {
		if row, ok := item.(map[string]interface{}); ok {
			row["mapping_values"] = bindings[i]
		}
	}

	return values, errs
}

// parseTemplateMappingFixedValue converts one non-empty posted value into the
// representation expected by a parameter: text, bool, number, asset ID, GPS
// coordinates, or date.
//
// Numbers must be finite. Asset IDs must identify an existing asset. GPS values
// are checked for latitude/longitude ranges and normalized to "lat,lng".
// Dates must use YYYY-MM-DD. Text retains its original spacing.
//
// Empty input is handled by the caller's optional/required logic; this function
// rejects it rather than treating it as false, zero, or an empty list.
//
// Used by readTemplateActionMappings to convert fixed parameter values and by validateWorkOrderTemplateStep2 to check typed rule comparisons.
func (h *UIHandler) parseTemplateMappingFixedValue(ctx context.Context, fieldType, raw string) (interface{}, error) {
	text := strings.TrimSpace(raw)

	if text == "" {
		return nil, fmt.Errorf("empty fixed value")
	}

	switch fieldType {
	case "text":
		return raw, nil

	case "boolean":
		switch text {
		case "true":
			return true, nil
		case "false":
			return false, nil
		default:
			return nil, fmt.Errorf("invalid boolean")
		}

	case "number":
		value, err := strconv.ParseFloat(text, 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, fmt.Errorf("invalid number")
		}
		return value, nil

	case "asset":
		id, err := strconv.ParseInt(text, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("invalid asset ID")
		}

		entity, err := h.xoluClient.Get(ctx, "assets", id)
		if err != nil {
			return nil, fmt.Errorf("load fixed asset: %w", err)
		}
		if entity == nil {
			return nil, fmt.Errorf("fixed asset not found")
		}

		return id, nil

	case "gps":
		parts := strings.Split(text, ",")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid GPS value")
		}

		lat, latErr := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		lng, lngErr := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)

		if latErr != nil || lngErr != nil ||
			math.IsNaN(lat) || math.IsNaN(lng) ||
			math.IsInf(lat, 0) || math.IsInf(lng, 0) ||
			lat < -90 || lat > 90 || lng < -180 || lng > 180 {
			return nil, fmt.Errorf("invalid GPS coordinates")
		}

		return strconv.FormatFloat(lat, 'f', -1, 64) + "," +
			strconv.FormatFloat(lng, 'f', -1, 64), nil

	case "date":
		if _, err := time.Parse("2006-01-02", text); err != nil {
			return nil, fmt.Errorf("invalid date: %w", err)
		}
		return text, nil

	default:
		return nil, fmt.Errorf("unsupported parameter type %q", fieldType)
	}
}

// =============================================================================
// TEMPLATE FORM INTERACTION AND VALIDATION FEEDBACK
//
// Client-side behavior for the template forms: switch action-mapping origins,
// filter rule references and operators, adapt comparison controls to field
// types, and highlight tabs containing validation errors.
//
// Scripts are scoped to their containing form and initialize dynamic controls
// as needed. Server-side validation remains responsible for deciding which
// configuration is valid and may be saved.
// =============================================================================

// templateActionMappingScript shows the controls for the selected origin and
// disables the inactive controls so they do not participate in submission or
// native form validation.
//
// All lookups are scoped to the containing form, keeping the modal's mapping
// controls separate from the first-step form behind it. Synchronization runs
// immediately after rendering and whenever an origin selector changes.
const templateActionMappingScript = `
(function () {
	var form = document.currentScript &&
		document.currentScript.closest('form');

	if (!form) return;

	var sourcePattern = /^action_mapping_[0-9]+_[0-9]+_source$/;

	function sync() {
		var rows = form.querySelectorAll('[data-field-row]');

		form.querySelectorAll('select[name]').forEach(function (select) {
			if (!sourcePattern.test(select.name)) return;

			var prefix = select.name.replace(/_source$/, '');

			rows.forEach(function (row) {
				var name = row.getAttribute('data-field-row');
				var active;

				if (name === prefix) {
					active = select.value === 'context';
				} else if (name === prefix + '_fixed') {
					active = select.value === 'fixed';
				} else {
					return;
				}

				row.style.display = active ? '' : 'none';

				row.querySelectorAll(
					'input, select, textarea, button'
				).forEach(function (control) {
					control.disabled = !active;
				});
			});
		});
	}

	form.addEventListener('change', function (event) {
		if (event.target && sourcePattern.test(event.target.name || '')) {
			sync();
		}
	});

	sync();
})();
`

// templateRuleTriggerScript filters the reference selector for each rule.
// It shows block lines for field triggers and whole blocks for block triggers.
// Header and workflow triggers hide and disable the selector.
const templateRuleTriggerScript = `
(function () {
    const form = document.currentScript?.closest('form');
    if (!form || form.dataset.ruleTriggerInitialized === '1') return;

    const triggerSelector = 'select[name^="rule_item["][name$="[trigger]"]';
    const firstTrigger = form.querySelector(triggerSelector);
    const rows = firstTrigger?.closest('.repeatable-row')?.parentElement;
    if (!rows) return;

	form.dataset.ruleTriggerInitialized = '1';
    const allOptions = new WeakMap();

    function syncReference(trigger) {
        const name = trigger.name.replace(/\[trigger\]$/, '[trigger_reference]');
        const reference = form.elements.namedItem(name);
        if (!(reference instanceof HTMLSelectElement)) return;

        const cell = reference.closest('.fe-cell');
        if (!cell) return;

        if (!allOptions.has(reference)) {
            allOptions.set(reference, Array.from(reference.options));
        }

        const isField = trigger.value === 'on_evaluate_field';
        const active = isField || trigger.value === 'on_evaluate_block';
        const selected = reference.value;
        const allowed = allOptions.get(reference).filter(option =>
            option.value === '' || (active && option.value.includes(':line:') === isField)
        );

        reference.replaceChildren(...allowed);
        reference.value = allowed.some(option => option.value === selected) ? selected : '';
        reference.disabled = !active;
        cell.style.display = active ? '' : 'none';
        window.SeamSelect?.refresh(reference);
    }

    form.addEventListener('change', event => {
        if (event.target.matches(triggerSelector)) syncReference(event.target);
    });

    function syncRows() {
        rows.querySelectorAll(triggerSelector).forEach(syncReference);
    }

    new MutationObserver(syncRows).observe(rows, { childList: true });
    syncRows();
})();
`

// templateRuleConditionScript keeps each condition's operator and value
// controls in sync with the selected field type. It reads the configuration
// from the modal form and initializes conditions as rows are added.
const templateRuleConditionScript = `
(function () {
    const form = document.currentScript?.closest('form');
    if (!form || form.dataset.ruleConditionsInitialized === '1') return;

    const config = JSON.parse(form.dataset.ruleConditions);
    form.dataset.ruleConditionsInitialized = '1';

    const fieldSelector = 'select[name^="rule_item["][name$="[field]"]';
    const operatorSelector = 'select[name^="rule_item["][name$="[operator]"]';
    const allOptions = new WeakMap();

    function peer(control, name) {
        return form.elements.namedItem(
            control.name.replace(/\[[^\]]+\]$/, '[' + name + ']')
        );
    }

    function syncValue(operator) {
        const type = config.fieldTypes[peer(operator, 'field').value];
        const input = peer(operator, 'comparison_value');
        const cell = input.closest('.fe-cell');
        const active = Boolean(operator.value) &&
            operator.value !== 'is_empty' && operator.value !== 'is_not_empty';
        const isBoolean = type === 'boolean';

        if (!active) input.value = '';
        input.disabled = !active;
        cell.style.display = active ? '' : 'none';
        input.style.display = isBoolean ? 'none' : '';
        input.type = type === 'date' ? 'date' : 'text';
        input.inputMode = type === 'number' ? 'decimal' : type === 'asset' ? 'numeric' : 'text';
        input.placeholder = type === 'asset' ? config.assetID : (input.getAttribute('aria-label') || '');

        let choice = cell.querySelector('select');
        if (isBoolean && !choice) {
            choice = document.createElement('select');
            choice.className = input.className;
            choice.setAttribute('aria-label', config.valueLabel);
            choice.add(new Option(config.empty, ''));
            choice.add(new Option(config.yes, 'true'));
            choice.add(new Option(config.no, 'false'));
            choice.addEventListener('change', () => { input.value = choice.value; });
            input.after(choice);
        }

        if (choice) {
            choice.style.display = isBoolean ? '' : 'none';
            choice.disabled = !active || !isBoolean;
            if (isBoolean) {
                choice.value = ['true', 'false'].includes(input.value) ? input.value : '';
            }
        }
    }

    function syncField(field, changed = false) {
        const operator = peer(field, 'operator');
        if (!allOptions.has(operator)) {
            allOptions.set(operator, Array.from(operator.options));
        }

        const allowed = config.operators[config.fieldTypes[field.value]] || [];
        const selected = operator.value;
        const options = allOptions.get(operator).filter(option =>
            option.value === '' || allowed.includes(option.value)
        );

        operator.replaceChildren(...options);
        operator.value = allowed.includes(selected) ? selected : '';
        operator.disabled = allowed.length === 0;
        window.SeamSelect?.refresh(operator);

        if (changed) peer(field, 'comparison_value').value = '';
        syncValue(operator);
    }

    form.addEventListener('change', event => {
        if (event.target.matches(fieldSelector)) syncField(event.target, true);
        if (event.target.matches(operatorSelector)) syncValue(event.target);
    });

    const init = root => root.querySelectorAll(fieldSelector).forEach(field => syncField(field));
    new MutationObserver(mutations => {
        for (const mutation of mutations) {
            for (const node of mutation.addedNodes) {
                if (node instanceof Element && node.matches('.repeatable-row')) init(node);
            }
        }
    }).observe(form, { childList: true, subtree: true });

    init(form);
})();
`

// templateTabErrorsScript highlights tabs containing validation errors and
// opens the first affected tab after normal tab initialization.
//
// The script is scoped to its containing form and creates no global API.
// Form-local state prevents duplicate listeners and ignores stale scheduled
// initialization when HTMX renders a newer set of errors.
const templateTabErrorsScript = `
(function (errorTabs) {
	'use strict';

	var script = document.currentScript;
	var form = script && script.closest('form');
	if (!form) return;

	errorTabs = errorTabs || {};

	var selector = '.fe-tab-btn';
	var state = form._woTemplateTabErrors;

	if (!state) {
		state = {
			revision: 0,
			onClick: null
		};
		form._woTemplateTabErrors = state;
	}

	var revision = ++state.revision;

	function initialize() {
		if (!form.isConnected || state.revision !== revision) return;

		if (state.onClick) {
			form.removeEventListener('click', state.onClick);
		}

		function buttons() {
			return Array.from(form.querySelectorAll(selector));
		}

		function paint() {
			buttons().forEach(function (btn) {
				var id = btn.getAttribute('data-tab');

				if (errorTabs[id] === true) {
					btn.classList.remove('text-indigo-600','text-gray-500','dark:text-indigo-400','dark:text-gray-400');

					btn.classList.add('text-red-600','dark:text-red-400');

					btn.setAttribute('data-seam-tab-error', 'true');

				} else if (btn.hasAttribute('data-seam-tab-error')) {
					btn.classList.remove('text-red-600','dark:text-red-400','text-indigo-600','text-gray-500','dark:text-indigo-400','dark:text-gray-400');

					btn.removeAttribute('data-seam-tab-error');

					var active = btn.classList.contains('border-indigo-600');

					btn.classList.add(active ? 'text-indigo-600' : 'text-gray-500', active ? 'dark:text-indigo-400' : 'dark:text-gray-400');
				}
			});
		}

		// Reapply error colors after the button's tab-switching listener.
		state.onClick = function (event) {
			var target = event.target;
			var btn = target && target.closest ? target.closest(selector) : null;

			if (!btn || !form.contains(btn)) return;

			paint();
		};

		form.addEventListener('click', state.onClick);

		var firstError = buttons().find(function (btn) {
			return errorTabs[btn.getAttribute('data-tab')] === true;
		});

		if (firstError) {
			firstError.click();
		}

		paint();
	}

	function schedule() {
		requestAnimationFrame(initialize);
	}

	if (document.readyState === 'loading') {
		document.addEventListener('DOMContentLoaded', schedule, { once: true });
	} else {
		schedule();
	}
})(%s);
`

// workOrderTemplateTabErrorsScript embeds the tab-error map in the local
// script. Emit an empty map too, so previously highlighted errors can clear.
//
// Used when rendering or replacing the first-step form to refresh the error markers on its tabs.
func workOrderTemplateTabErrorsScript(errorTabs map[string]bool) mi.H {
	encoded := []byte("{}")

	if len(errorTabs) > 0 {
		if data, err := json.Marshal(errorTabs); err == nil {
			encoded = data
		}
	}

	return func(b *mi.Builder) mi.Node {
		return b.Script(mi.Raw(fmt.Sprintf(
			templateTabErrorsScript,
			string(encoded),
		)))
	}
}

// =============================================================================
// DISPLAY LABELS AND DETAIL PRESENTATION
//
// Turn template configuration into readable UI values. Stored references and
// bindings remain unchanged; entity lookups here supply display information.
// =============================================================================

// tLatest formats a template version with its translated latest/outdated label.
//
// Used by templateInfoContent to show the version and its status in the Information card.
func tLatest(latest bool, version int, t func(key string, args ...any) string) string {
	if latest {
		return fmt.Sprintf("v%d (%s)", version, t("workorders-templates.latest"))
	}
	return fmt.Sprintf("v%d (%s)", version, t("workorders-templates.outdated"))
}

// tAmountAssets formats the configured asset-count range. Zero represents
// an open bound: min == 0 allows no assets, and max == 0 means no upper limit.
//
// Used by templateScopeContent to display the configured asset count range in the Scope card.
func tAmountAssets(min int, max int, t func(key string, args ...any) string) string {
	if min == 0 {
		if max == 0 {
			return t("workorders-templates.asset_types_any_amount")
		} else {
			return fmt.Sprintf("0 - %d", max)
		}
	} else {
		if max == 0 {
			return fmt.Sprintf("%d - %s", min, t("workorders-templates.asset_types_no_max_amount"))
		} else {
			return fmt.Sprintf("%d - %d", min, max)
		}
	}
}

// workOrderTemplateFieldTypeLabel returns the translated label for a supported
// work order field type. Unknown types retain their stored value.
//
// Used for consistent field type labels in template details, condition selectors, and validation messages.
func workOrderTemplateFieldTypeLabel(fieldType string, t func(key string, args ...any) string) string {
	if validWOFieldType(fieldType) {
		return t("workorders-template.type." + fieldType)
	}

	return fieldType
}

// workOrderTemplateErrorTabs maps known top-level error keys to first-step
// tabs for highlighting and initial error navigation.
//
// Keys without an explicit entry are ignored. This function does not parse
// indexed field paths or select the second-step modal's active tab.
//
// Used by WorkOrderTemplateFormPage to mark tabs with errors and by workOrderTemplateSubmit to open the first affected tab.
func workOrderTemplateErrorTabs(errors map[string]string) map[string]bool {
	fieldTabs := map[string]string{
		"name":        "tab-principal",
		"color":       "tab-principal",
		"description": "tab-principal",
		"icon":        "tab-principal",

		"assetType": "tab-scope",
		"minAssets": "tab-scope",
		"maxAssets": "tab-scope",

		"header_items": "tab-header",

		"blocks":          "tab-workflow",
		"workflow_states": "tab-workflow",
		"actions":         "tab-workflow",

		"prefix":          "tab-rules",
		"sequence_length": "tab-rules",
		"suffix":          "tab-rules",
		"reset_policy":    "tab-rules",
		"rule_item":       "tab-rules",

		"consumables_allowed":         "tab-consumables",
		"add_any_consumables_allowed": "tab-consumables",
		"consumables":                 "tab-consumables",
	}

	tabs := make(map[string]bool)

	for field := range errors {
		if tab, ok := fieldTabs[field]; ok {
			tabs[tab] = true
		}
	}

	return tabs
}

// workOrderTemplateActionDetails builds the action and mapping rows displayed
// on the template detail page, preserving the stored call and binding order.
//
// Action definitions supply current action/parameter labels, and asset lookups
// supply names for fixed asset values. Lookups are cached within this call.
// Missing metadata falls back to identifiers so saved bindings remain visible.
//
// The stored bindings determine what is displayed. This function does not
// rebuild their allowed options, revalidate them, or execute the actions.
//
// Used by WorkOrderTemplateDetail to prepare the saved actions and parameter bindings for the Actions card.
func (h *UIHandler) workOrderTemplateActionDetails(ctx context.Context, actions []WorkOrderTemplateAction, t func(key string, args ...any) string,
) []WorkOrderTemplateActionDetail {
	items := make([]WorkOrderTemplateActionDetail, 0, len(actions))

	actionCache := make(map[int64]WOAction)
	assetNames := make(map[int64]string)

	assetLabel := func(id int64) string {
		if name, found := assetNames[id]; found {
			return name
		}

		name := t("workorders-templates.actions.asset_fallback", id)

		entity, err := h.xoluClient.Get(ctx, "assets", id)
		if err != nil {
			h.logger.Warn("failed to load fixed mapping asset", "asset_id", id, "error", err)
		} else if entity != nil {
			if storedName := toString(entity.Data["name"]); storedName != "" {
				name = storedName
			}
		}

		assetNames[id] = name
		return name
	}

	for _, saved := range actions {
		action, found := actionCache[saved.ActionID]

		if !found {
			action = WOAction{
				ID: saved.ActionID,
				Name: t(
					"workorders-templates.actions.fallback",
					saved.ActionID,
				),
			}

			if saved.ActionID > 0 {
				entity, err := h.xoluClient.Get(
					ctx,
					"wo_actions",
					saved.ActionID,
				)

				if err != nil {
					h.logger.Warn("failed to load template action labels", "action_id", saved.ActionID, "error", err)
				} else if entity != nil {
					if name := toString(entity.Data["name"]); name != "" {
						action.Name = name
					}

					parameters, err := decodeWOActionParameters(
						entity.Data["parameters"],
					)
					if err != nil {
						h.logger.Warn("failed to decode template action labels", "action_id", saved.ActionID, "error", err)
					} else {
						action.Parameters = parameters
					}
				}
			}

			actionCache[saved.ActionID] = action
		}

		parameters := make(map[string]WOActionParameter)
		for _, parameter := range action.Parameters {
			parameters[parameter.Key] = parameter
		}

		item := WorkOrderTemplateActionDetail{
			ActionID:       saved.ActionID,
			Name:           action.Name,
			TriggerPoint:   saved.TriggerPoint,
			BlockReference: saved.BlockReference,
			Mappings: make(
				[]WorkOrderTemplateActionMappingDetail,
				0,
				len(saved.MappingValues),
			),
		}

		for _, mapping := range saved.MappingValues {
			parameter := parameters[mapping.Parameter]

			name := parameter.Name
			if name == "" {
				name = mapping.Parameter
			}

			row := WorkOrderTemplateActionMappingDetail{
				Parameter:  name,
				Origin:     "—",
				Value:      "—",
				Conversion: "—",
			}

			switch mapping.Source {
			case "context":
				row.Origin = t("workorders-templates.mapping.from_context")
				row.Value = templateActionContextLabel(mapping.ContextPath, t)

			case "fixed":
				row.Origin = t(
					"workorders-templates.mapping.from_fixed",
				)
				row.Value = templateActionFixedValueLabel(mapping.FixedValue, parameter.Type, assetLabel, t)

			default:
				row.Origin = mapping.Source
			}

			switch mapping.Transform {
			case "wrap_in_array":
				row.Conversion = t("workorders-templates.mapping.as_single_item_list")
			case "", "identity":
				// No conversion to display.
			default:
				row.Conversion = mapping.Transform
			}

			item.Mappings = append(item.Mappings, row)
		}

		items = append(items, item)
	}

	return items
}

// templateActionContextLabel turns a stored context reference into a readable
// label for work-order assets, a header field, or a block line.
//
// Escaped names are decoded for display. Unknown or malformed references retain
// their original value so the detail page does not hide stored information.
//
// Used by workOrderTemplateActionDetails to display the context source assigned to each mapped parameter.
func templateActionContextLabel(path string, t func(key string, args ...any) string) string {
	if path == "work_order.assets" {
		return t("workorders-templates.mapping.wo_assets")
	}

	parts := strings.Split(path, ":")

	if len(parts) == 2 && parts[0] == "header" {
		tag, err := url.QueryUnescape(parts[1])
		if err == nil {
			return t("workorders-templates.option.header", tag)
		}
	}

	if len(parts) == 4 &&
		parts[0] == "block" &&
		parts[2] == "line" {
		block, blockErr := url.QueryUnescape(parts[1])
		line, lineErr := url.QueryUnescape(parts[3])

		if blockErr == nil && lineErr == nil {
			return t("workorders-templates.option.block", block) +
				" · " + line
		}
	}

	return path
}

// templateActionFixedValueLabel formats scalar and list fixed values for the
// action detail table. Lists are rendered as numbered lines; asset IDs use the
// supplied label resolver, and booleans use translated Yes/No labels.
//
// False and zero remain visible values. Only nil represents a missing value;
// an empty list is displayed explicitly as [].
//
// Used by workOrderTemplateActionDetails to display fixed parameter values, including each element of a list.
func templateActionFixedValueLabel(value interface{}, fieldType string, assetLabel func(int64) string, t func(key string, args ...any) string) string {
	if value == nil {
		return "—"
	}

	if items, ok := value.([]interface{}); ok {
		if len(items) == 0 {
			return "[]"
		}

		lines := make([]string, 0, len(items))

		for index, item := range items {
			label := templateActionFixedValueLabel(item, fieldType, assetLabel, t)

			lines = append(lines, fmt.Sprintf("%d. %s", index+1, label))
		}

		return strings.Join(lines, "\n")
	}

	if fieldType == "asset" {
		id, err := strconv.ParseInt(fmt.Sprint(value), 10, 64)
		if err == nil && id > 0 {
			return assetLabel(id)
		}
	}

	switch v := value.(type) {
	case bool:
		if v {
			return t("wo_action.parameters.yes")
		}
		return t("wo_action.parameters.no")

	case string:
		return v

	case json.Number:
		return v.String()

	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)

	default:
		raw, err := json.Marshal(value)
		if err == nil {
			return string(raw)
		}
		return fmt.Sprint(value)
	}
}
