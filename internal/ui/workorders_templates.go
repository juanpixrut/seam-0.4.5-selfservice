package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	mi "github.com/ha1tch/minty"
	fe "github.com/ha1tch/seam-ui/internal/formengine"
	workordertemplateschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/workorders_templates"
)

type WorkOrderTemplate struct {
	ID                 int64
	Version            int
	Latest             bool
	Name               string
	Description        string
	Color              string
	Icon               string
	AssetTypeID        int64
	MinAssets          int
	MaxAssets          int
	HeaderItems        []HeaderItem
	Blocks             []TemplateBlock
	WorkflowStates     []WorkflowState
	Actions            []WorkOrderTemplateAction
	Prefix             string
	Suffix             string
	SeqLength          int
	ResetPolicy        string
	ConsumablesEnabled bool
	Consumables        []TemplateConsumable
	ExtraConsumables   bool
}

type WorkflowState struct {
	Name string `json:"name"`
}

type HeaderItem struct {
	Tag          string `json:"tag"`
	Type         string `json:"type"`
	DefaultValue string `json:"default_value"`
	ToFillOut    string `json:"to_fill_out"`
	Required     bool   `json:"required"`
}

type TemplateBlock struct {
	Name            string      `json:"name"`
	ResponsableType string      `json:"responsible_type"`
	Predecessor     string      `json:"predecessor"`
	Lines           []BlockLine `json:"lines"`
}

type BlockLine struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type TemplateConsumable struct {
	ConsumableID int64   `json:"consumable_id"`
	Quantity     float64 `json:"quantity"`
}

// WorkOrdersTemplatesList handles GET /workorders-templates.
func (h *UIHandler) WorkOrdersTemplatesList(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()

	result, err := h.xoluClient.OQL(ctx, "SELECT TOP 100 * FROM work_order_templates ORDER BY id DESC")
	var templates []WorkOrderTemplateRow
	if err == nil {
		for _, row := range result.Data {
			templates = append(templates, WorkOrderTemplateRow{
				ID:          toInt64(row["id"]),
				Version:     toInt(row["version"]),
				Name:        toString(row["name"]),
				Description: toString(row["description"]),
				Color:       toString(row["color"]),
				Icon:        toString(row["icon"]),
			})
		}
	} else {
		h.logger.Error("failed to list work orders templates", "error", err)
	}

	if len(templates) > 0 {
		ids := make([]string, len(templates))
		for i, template := range templates {
			ids[i] = strconv.FormatInt(template.ID, 10)
		}
		query := "SELECT template_id, COUNT(*) AS order_count FROM work_orders WHERE template_id IN (" +
			strings.Join(ids, ",") + ") GROUP BY template_id"
		result, err := h.xoluClient.OQL(ctx, query)
		if err == nil {
			counts := make(map[int64]int, len(result.Data))
			for _, row := range result.Data {
				counts[toInt64(row["template_id"])] = toInt(row["order_count"])
			}
			for i := range templates {
				templates[i].OrderCount = counts[templates[i].ID]
			}
		} else if !entityNotYetCreated(err) {
			h.logger.Error("failed to count work orders by template", "error", err)
			for i := range templates {
				templates[i].OrderCountUnavailable = true
			}
		}
	}

	data := WorkOrderTemplateListData{Templates: templates}
	h.page.Render(w, r, t("workorders-templates.list_title"), "/workorders-templates", WorkOrdersTemplateListPage(data, t))
}

// WorkOrdersTemplatesNew handles GET /workorders-templates/new.
func (h *UIHandler) WorkOrdersTemplatesNew(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	csrfToken := h.GetCSRFToken(w, r)

	formEng := &fe.FormEngine{
		Translator: h.formEngineTranslator(),
	}
	rc := fe.RenderContext{Locale: h.currentLocale(), CSRFToken: csrfToken, Module: "workorders-templates", Form: "workorders-template"}
	formContent := formEng.Render(ctx, resolveSchema(ctx, "workorders_templates", "create", workordertemplateschemas.Schema("create")), nil, nil, rc)

	data := WorkOrderTemplateFormData{CSRFToken: csrfToken, FormContent: formContent}
	h.page.Render(w, r, t("workorders-templates.new"), "/workorders-templates/new", WorkOrderTemplateFormPage(data, t))
}

// WorkOrdersTemplatesStep2 handles POST /workorders-templates/step-2.
// It derives field options from the submitted first step and renders
// step-2.json as a modal fragment. It does not persist the template.
func (h *UIHandler) WorkOrdersTemplatesStep2(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, h.errText(r, "error.invalid_form_data", "Invalid form data"), http.StatusBadRequest)
		return
	}

	firstStep, err := fe.ParseSchema(resolveSchema(ctx, "workorders_templates", "create", workordertemplateschemas.Schema("create")))
	if err != nil {
		h.logger.Error("failed to parse template schema", "error", err)
		http.Error(w, "Invalid schema", http.StatusInternalServerError)
		return
	}

	parsed := parseWorkOrderTemplateSecondStep(
		r.PostForm,
		firstStep.Fields,
	)

	headerItems, _ := parsed["header_items"].([]interface{})
	normalizeTemplateHeaderRequired(headerItems)

	blocks, _ := parsed["blocks"].([]interface{})
	actions, _ := parsed["actions"].([]interface{})
	actions, validActions, actionErrors := h.validateWorkOrderTemplateActions(ctx, actions, blocks, t)

	if len(actionErrors) > 0 {
		http.Error(w, errorsToSummary(actionErrors), http.StatusBadRequest)
		return
	}

	mappingPlan, err := h.buildTemplateActionMappingPlan(ctx, actions, validActions, headerItems, blocks, t)

	if err != nil {
		h.logger.Error("failed to build template action mappings", "error", err)
		http.Error(w, t("error.generic"), http.StatusInternalServerError)
		return
	}

	options := workOrderTemplateFieldOptions(headerItems, blocks, t)
	targets := workOrderTemplateTargetOptions(options, blocks, t)
	fieldTypes := workOrderTemplateConditionFieldTypes(headerItems, blocks, t)

	formEng := &fe.FormEngine{
		Translator: h.formEngineTranslator(),
		Sources: mappingPlan.sourceResolver(workOrderTemplateSources{
			fields:  workOrderTemplateConditionOptions(options, fieldTypes, t),
			targets: targets,
			t:       t,
		}),
	}

	rc := fe.RenderContext{
		Locale:    h.currentLocale(),
		CSRFToken: h.GetCSRFToken(w, r),
		Module:    "workorders-templates",
		Form:      "workorders-template-step-two",
		IsPartial: true,
		ActiveTab: "tab-step2-rules",
	}

	// Open Actions when there are parameters to map;
	// otherwise, keep Rules as the initial tab.
	if len(mappingPlan.Fields) > 0 {
		rc.ActiveTab = "tab-step2-actions"
	}

	content := formEng.Render(ctx, mappingPlan.Schema, mappingPlan.initialValues(), nil, rc)
	h.page.RenderPartial(w, WorkOrderTemplateStep2Page(content, fieldTypes, t))
}

// WorkOrderTemplateCreate handles POST /workorders-templates.
// It delegates validation and persistence of both steps to workOrderTemplateSubmit.
func (h *UIHandler) WorkOrderTemplateCreate(w http.ResponseWriter, r *http.Request) {
	h.workOrderTemplateSubmit(w, r, false)
}

// WorkOrdersTemplatesValidateStep1 handles POST /workorders-templates/validate-step-1.
// It validates the first step without creating an entity. Invalid data
// re-renders the first-step fields; valid data triggers woTemplateStep1Valid.
func (h *UIHandler) WorkOrdersTemplatesValidateStep1(w http.ResponseWriter, r *http.Request) {
	h.workOrderTemplateSubmit(w, r, true)
}

// workOrderTemplateSubmit shares first-step parsing and validation between
// the Next and Create handlers. validateOnly stops before processing the
// second step or writing to Xolu.
//
// On creation, it also validates and persists actions and rules. HTMX success
// closes the modal and redirects the full page; errors preserve entered values.
func (h *UIHandler) workOrderTemplateSubmit(w http.ResponseWriter, r *http.Request, validateOnly bool) {
	t := h.translateFunc(r)
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, h.errText(r, "error.invalid_form_data", "Invalid form data"), http.StatusBadRequest)
		return
	}

	version := 1
	name := r.FormValue("name")
	description := r.FormValue("description")
	color := r.FormValue("color")
	icon := r.FormValue("icon")
	assetTypeID := r.FormValue("assetType")
	minAssets := r.FormValue("minAssets")
	maxAssets := r.FormValue("maxAssets")
	prefix := r.FormValue("prefix")
	sequence_length := r.FormValue("sequence_length")
	suffix := r.FormValue("suffix")
	reset_policy := r.FormValue("reset_policy")
	isChecked := func(name string) bool {
		for _, value := range r.PostForm[name] {
			switch strings.ToLower(strings.TrimSpace(value)) {
			case "on", "true", "1":
				return true
			}
		}
		return false
	}

	consumablesAllowed := isChecked("consumables_allowed")
	addAnyConsumablesAllowed := consumablesAllowed &&
		isChecked("add_any_consumables_allowed")

	csrfToken := h.GetCSRFToken(w, r)

	form, err := fe.ParseSchema(resolveSchema(ctx, "workorders_templates", "create", workordertemplateschemas.Schema("create")))
	if err != nil {
		h.logger.Error("failed to parse work order template schema", "error", err)
		http.Error(w, h.errText(r, "error.server_error", "internal server error"), http.StatusInternalServerError)
		return
	}

	parsedFirstStep := parseWorkOrderTemplateSecondStep(
		r.PostForm,
		form.Fields,
	)

	// HEADER ITEMS
	rawHeaderItems, _ := parsedFirstStep["header_items"].([]interface{})
	normalizeTemplateHeaderRequired(rawHeaderItems)

	headerItems := make([]interface{}, 0, len(rawHeaderItems))
	seenTags := make(map[string]bool)

	for _, item := range rawHeaderItems {
		row, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		tag := strings.TrimSpace(toString(row["tag"]))
		if tag == "" || seenTags[tag] {
			continue
		}

		seenTags[tag] = true
		row["tag"] = tag
		if strings.TrimSpace(toString(row["to_fill_out"])) == "" {
			row["to_fill_out"] = "upon_executing"
		}
		headerItems = append(headerItems, row)
	}

	// BLOCKS
	blocks := parsedFirstStep["blocks"]
	if blocks == nil {
		blocks = []interface{}{}
	}

	//WORKFLOW STATES
	states := []interface{}{}
	for _, v := range r.PostForm["workflow_states_name[]"] {
		if v = strings.TrimSpace(v); v != "" {
			states = append(states, map[string]interface{}{"name": v})
		}
	}

	// ACTIONS
	actions, _ := parsedFirstStep["actions"].([]interface{})
	if actions == nil {
		actions = []interface{}{}
	}

	// CONSUMABLES
	consumables, _ := parsedFirstStep["consumables"].([]interface{})
	if consumables == nil {
		consumables = []interface{}{}
	}

	if !consumablesAllowed {
		consumables = []interface{}{}
	}

	/// FORM CONTROL ///

	errors := map[string]string{}

	//PRINCIPAL
	if name == "" {
		errors["name"] = t("users.error.name_required")
	}

	//SCOPE
	minA, _ := strconv.ParseInt(minAssets, 10, 64)
	maxA, _ := strconv.ParseInt(maxAssets, 10, 64)
	if (minA != 0 || maxA != 0) && ((minA > maxA && maxA != 0) || minA < 0 || maxA < 0) {
		errors["minAssets"] = t("workorders-templates.error.max_min_assets")
	}

	//BLOCKS
	blocksList, ok := blocks.([]interface{})
	if !ok {
		blocksList = []interface{}{}
	}

	validateFieldType := func(row map[string]interface{}, errorKey string) {
		fieldType := toString(row["type"])
		row["type"] = fieldType

		if !validWOFieldType(fieldType) {
			errors[errorKey] = t(
				"workorders-templates.error.invalid_field_type",
			)
		}
	}

	for _, item := range headerItems {
		row, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		validateFieldType(row, "header_items")
	}

	for _, item := range blocksList {
		block, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		lines, _ := block["lines"].([]interface{})

		for _, item := range lines {
			line, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			validateFieldType(line, "blocks")
		}
	}
	blocksNames := make(map[string]bool)

	for _, block := range blocksList {
		blockData, ok := block.(map[string]interface{})
		if !ok {
			continue
		}

		name := strings.TrimSpace(toString(blockData["name"]))
		if name == "" {
			errors["blocks"] = t("blocks.error.name_required")
			continue
		}
		blockData["name"] = name
		lineNames := map[string]bool{}
		lines, _ := blockData["lines"].([]interface{})
		for _, item := range lines {
			line := item.(map[string]interface{})
			lineName := strings.TrimSpace(toString(line["name"]))
			line["name"] = lineName
			if lineNames[lineName] {
				errors["blocks"] = t("workorders-templates.error.same_named_lines")
			}
			lineNames[lineName] = true
		}

		normalizedName := strings.ToLower(name)

		predecessor := strings.TrimSpace(toString(blockData["predecessor"]))
		if strings.ToLower(predecessor) == normalizedName {
			errors["blocks"] = t("blocks.error.self_predecessor")
			break
		}

		if blocksNames[normalizedName] {
			errors["blocks"] = t("blocks.error.duplicate_names")
			break
		}

		blocksNames[normalizedName] = true
	}
	for _, block := range blocksList {
		blockData, ok := block.(map[string]interface{})
		if !ok {
			continue
		}
		predecessor := strings.TrimSpace(toString(blockData["predecessor"]))
		if predecessor != "" && !blocksNames[strings.ToLower(predecessor)] {
			errors["blocks"] = t("blocks.error.invalid_predecessor")
			break
		}
	}

	// Validate the graph only if no other block error already exists.
	if _, exists := errors["blocks"]; !exists {
		decodedBlocks, dependencyErr := decodeBlocks(blocksList)

		if dependencyErr == nil {
			_, dependencyErr = workOrderBlockPredecessors(decodedBlocks)
		}

		if dependencyErr != nil {
			h.logger.Info("invalid work order template block dependencies", "error", dependencyErr)

			errors["blocks"] = t("blocks.error.invalid_dependencies")
		}
	}

	responsibleErrors := validateWorkOrderTemplateBlockResponsibleTypes(blocksList, t)

	for field, message := range responsibleErrors {
		// Preserve another block error already detected.
		if _, exists := errors[field]; !exists {
			errors[field] = message
		}
	}

	// ACTIONS
	actions, validActions, actionErrors := h.validateWorkOrderTemplateActions(ctx, actions, blocksList, t)

	for field, message := range actionErrors {
		errors[field] = message
	}

	// WORKFLOW
	if len(states) < 2 {
		errors["workflow_states"] = t("workflow_states.error.min_states")
	}

	statesNames := make(map[string]bool)

	for _, state := range states {
		stateData, ok := state.(map[string]interface{})
		if !ok {
			continue
		}

		name := strings.TrimSpace(toString(stateData["name"]))
		normalizedName := strings.ToLower(name)

		if statesNames[normalizedName] {
			errors["workflow_states"] = t("workflow_states.error.duplicate_states")
			break
		}

		statesNames[normalizedName] = true
	}

	//NUMBERING
	seqLen, _ := strconv.ParseInt(sequence_length, 10, 64)
	if seqLen < 2 || seqLen > 10 {
		errors["sequence_length"] = t("workorders-templates.error.sequence_length")
	}

	// CONSUMABLES
	if consumablesAllowed && !addAnyConsumablesAllowed && len(consumables) == 0 {
		errors["consumables"] = t("workorders-templates.error.consumable_required")
	}

	seenConsumables := make(map[int64]bool)

	for _, item := range consumables {
		row, ok := item.(map[string]interface{})
		if !ok {
			errors["consumables"] = t("workorders-templates.error.invalid_consumable")
			continue
		}

		idText := ""
		if value := row["consumable_id"]; value != nil {
			idText = strings.TrimSpace(fmt.Sprint(value))
		}

		quantityText := ""
		if value := row["quantity"]; value != nil {
			quantityText = strings.TrimSpace(fmt.Sprint(value))
		}

		id, idErr := strconv.ParseInt(idText, 10, 64)
		quantity, quantityErr := strconv.ParseFloat(quantityText, 64)

		valid := true

		if idErr != nil || id <= 0 {
			errors["consumables"] = t("workorders-templates.error.invalid_consumable")
			valid = false
		}

		if quantityErr != nil || quantity <= 0 {
			valid = false
		}

		if !valid {
			continue
		}

		if seenConsumables[id] {
			errors["consumables"] = t("workorders-templates.error.duplicate_consumable")
			continue
		}
		seenConsumables[id] = true

		if _, err := h.xoluClient.Get(ctx, "consumable", id); err != nil {
			errors["consumables"] = t("workorders-templates.error.consumable_unavailable")
			continue
		}

		row["consumable_id"] = id
		row["quantity"] = quantity
	}

	// Enters only if comes from WorkOrdersTemplatesValidateStep1 — that is, when the user wishes to proceed to the second part of the form.
	if validateOnly {
		if len(errors) == 0 {
			// Retains the page fields and enables the second step.
			w.Header().Set("HX-Reswap", "none")
			HTMXTrigger(w, "woTemplateStep1Valid")
			w.WriteHeader(http.StatusOK)
			return
		}

		// Reconstructs the values ​​from the first step, including the
		// numbering fields and any other simple fields from the schema.
		firstValues := make(map[string]interface{})

		for _, field := range form.Fields {
			if field.JSONType != "array" {
				firstValues[field.Name] = r.FormValue(field.Name)
			}
		}

		firstValues["header_items"] = headerItems
		firstValues["blocks"] = blocks
		firstValues["workflow_states"] = states
		firstValues["actions"] = actions
		firstValues["consumables_allowed"] = consumablesAllowed
		firstValues["add_any_consumables_allowed"] = addAnyConsumablesAllowed
		firstValues["consumables"] = consumables

		rc := fe.RenderContext{
			Locale:    h.currentLocale(),
			CSRFToken: csrfToken,
			Module:    "workorders-templates",
			Form:      "workorders-template",
		}

		// Opens the first tab with errors.
		errorTabs := workOrderTemplateErrorTabs(errors)

		for _, tabID := range []string{
			"tab-principal",
			"tab-scope",
			"tab-header",
			"tab-workflow",
			"tab-rules",
			"tab-consumables",
		} {
			if errorTabs[tabID] {
				rc.ActiveTab = tabID
				break
			}
		}

		formEng := &fe.FormEngine{
			Translator: h.formEngineTranslator(),
		}

		content := formEng.Render(
			ctx,
			resolveSchema(ctx, "workorders_templates", "create", workordertemplateschemas.Schema("create")),
			firstValues,
			errors,
			rc,
		)

		withTabErrors := func(b *mi.Builder) mi.Node {
			return b.Div(
				content(b),
				workOrderTemplateTabErrorsScript(errorTabs)(b),
			)
		}

		h.page.RenderPartial(w, withTabErrors)
		return
	}

	mappingPlan, err := h.buildTemplateActionMappingPlan(ctx, actions, validActions, headerItems, blocksList, t)
	if err != nil {
		h.logger.Error("failed to build template action mappings", "error", err)
		http.Error(w, t("error.generic"), http.StatusInternalServerError)
		return
	}
	// Grabs fields from header and block lines created dynamically in the first part of the form

	secondStep, err := fe.ParseSchema(mappingPlan.Schema)
	if err != nil {
		h.logger.Error("failed to parse template second-step schema", "error", err)
		http.Error(w, h.errText(r, "error.server_error", "internal server error"), http.StatusInternalServerError)
		return
	}
	secondValues := parseWorkOrderTemplateSecondStep(r.PostForm, secondStep.Fields)
	rules, _ := secondValues["rule_item"].([]interface{})
	mappingValues, mappingErrors := h.readTemplateActionMappings(ctx, r.PostForm, secondValues, actions, mappingPlan, t)

	if rules == nil {
		rules = []interface{}{}
	}

	for field, message := range mappingErrors {
		errors[field] = message
	}

	options := workOrderTemplateFieldOptions(headerItems, blocksList, t)
	targets := workOrderTemplateTargetOptions(options, blocksList, t)
	fieldTypes := workOrderTemplateConditionFieldTypes(headerItems, blocksList, t)

	_, secondErrors := h.validateWorkOrderTemplateStep2(ctx, nil, rules, fieldTypes, targets, secondStep, t)
	for field, message := range secondErrors {
		errors[field] = message
	}

	values := map[string]interface{}{
		"name":                        name,
		"description":                 description,
		"color":                       color,
		"icon":                        icon,
		"assetType":                   assetTypeID,
		"minAssets":                   minAssets,
		"maxAssets":                   maxAssets,
		"header_items":                headerItems,
		"blocks":                      blocks,
		"workflow_states":             states,
		"actions":                     actions,
		"prefix":                      prefix,
		"sequence_length":             sequence_length,
		"suffix":                      suffix,
		"reset_policy":                reset_policy,
		"rule_item":                   rules,
		"consumables_allowed":         consumablesAllowed,
		"add_any_consumables_allowed": addAnyConsumablesAllowed,
		"consumables":                 consumables,
	}
	for field, value := range mappingValues {
		values[field] = value
	}
	renderErrors := func(errs map[string]string) {
		formEng := &fe.FormEngine{
			Translator: h.formEngineTranslator(),
			Sources: mappingPlan.sourceResolver(workOrderTemplateSources{
				fields:  workOrderTemplateConditionOptions(options, fieldTypes, t),
				targets: targets,
				t:       t,
			}),
		}
		rc := fe.RenderContext{Locale: h.currentLocale(), CSRFToken: csrfToken, Module: "workorders-templates", Form: "workorders-template"}
		if IsHTMXRequest(r) {
			rc.Form, rc.IsPartial = "workorders-template-step-two", true
			rc.ActiveTab = "tab-step2-rules"

			for field := range errs {
				if strings.HasPrefix(field, "action_mapping_") {
					rc.ActiveTab = "tab-step2-actions"
					break
				}
			}

			content := formEng.Render(ctx, mappingPlan.Schema, values, errs, rc)

			withSummary := func(b *mi.Builder) mi.Node {
				return b.Div(
					mi.Class("space-y-4"),
					workOrderTemplateErrorSummary(errs, t)(b),
					content(b),
				)
			}
			h.page.RenderPartial(w, WorkOrderTemplateStep2Page(withSummary, fieldTypes, t))
			return
		}
		content := formEng.Render(ctx, resolveSchema(ctx, "workorders_templates", "create", workordertemplateschemas.Schema("create")), values, errs, rc)
		data := WorkOrderTemplateFormData{CSRFToken: csrfToken, FormContent: content, Errors: errs}
		h.page.Render(w, r, t("workorders-templates.new"), "/workorders-templates", WorkOrderTemplateFormPage(data, t))
	}
	if len(errors) > 0 {
		renderErrors(errors)
		return
	}

	entity, err := h.xoluClient.Create(ctx, "work_order_templates", map[string]any{
		"version":                     version,
		"latest":                      true,
		"name":                        name,
		"description":                 description,
		"color":                       color,
		"icon":                        icon,
		"asset_type_id":               assetTypeID,
		"min_assets":                  minAssets,
		"max_assets":                  maxAssets,
		"header_items":                headerItems,
		"blocks":                      blocks,
		"workflow_states":             states,
		"actions":                     actions,
		"prefix":                      prefix,
		"sequence_length":             sequence_length,
		"suffix":                      suffix,
		"reset_policy":                reset_policy,
		"rule_item":                   rules,
		"consumables_allowed":         consumablesAllowed,
		"add_any_consumables_allowed": addAnyConsumablesAllowed,
		"consumables":                 consumables,
	})
	if err != nil {
		h.logger.Error("failed to create work order template", "error", err)
		renderErrors(map[string]string{"_": t("error.create_failed")})
		return
	}

	detailURL := "/workorders-templates/" + strconv.FormatInt(entity.ID, 10)
	if IsHTMXRequest(r) {
		HTMXTrigger(w, "seamModal2Saved")
		HTMXRedirect(w, detailURL)
		return
	}
	http.Redirect(w, r, detailURL, http.StatusSeeOther)

}

// WorkOrderDetail handles GET /workorders-templates/{id}.
func (h *UIHandler) WorkOrderTemplateDetail(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("error.not_found"))
		return
	}

	entity, err := h.xoluClient.Get(ctx, "work_order_templates", id)
	if err != nil {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("error.not_found"))
		return
	}
	wo, err := workOrderTemplateFromEntity(entity)
	if err != nil {
		h.logger.Error("failed to decode work order template", "error", err)
		http.Error(w, t("error.generic"), http.StatusInternalServerError)
		return
	}
	csrfToken := h.GetCSRFToken(w, r)

	var assetType AssetTypeBasicData
	if wo.AssetTypeID > 0 {
		assetTypeEntity, err := h.xoluClient.Get(ctx, "asset_types", wo.AssetTypeID)
		if err != nil {
			h.logger.Error("failed to get asset type",
				"asset_type_id", wo.AssetTypeID,
				"error", err,
			)
		} else {
			assetType = assetTypeFromEntity(assetTypeEntity)
		}
	}

	var consumableItems []TemplateConsumableDetail
	if wo.ConsumablesEnabled {
		for _, item := range wo.Consumables {
			name := fmt.Sprintf("%s #%d", t("workorders-templates.forms.workorders-template.field.consumable_id"), item.ConsumableID)

			consumable, err := h.xoluClient.Get(ctx, "consumable", item.ConsumableID)
			if err != nil {
				h.logger.Error("failed to get template consumable",
					"consumable_id", item.ConsumableID,
					"error", err,
				)
			} else if storedName := toString(consumable.Data["name"]); storedName != "" {
				name = storedName
			}

			consumableItems = append(consumableItems, TemplateConsumableDetail{
				ConsumableID: item.ConsumableID,
				Name:         name,
				Quantity:     item.Quantity,
			})
		}
	}
	actionItems := h.workOrderTemplateActionDetails(ctx, wo.Actions, t)
	ruleItems := workOrderTemplateRulesDetails(entity.Data, t)

	data := WorkOrderTemplateDetailData{
		WorkOrderTemplate: wo,
		AssetType:         assetType,
		Consumables:       consumableItems,
		Actions:           actionItems,
		Rules:             ruleItems,
		CSRFToken:         csrfToken}
	h.page.Render(w, r, t("workorders-templates.detail_title"), "/workorders-templates", WorkOrderTemplateDetailPage(data, t))
}

type WorkOrderTemplateListData struct {
	Templates []WorkOrderTemplateRow
	Total     int
}

type WorkOrderTemplateRow struct {
	ID                    int64
	Version               int
	Name                  string
	Description           string
	Color                 string
	Icon                  string
	OrderCount            int
	OrderCountUnavailable bool
}

// WorkOrdersTemplateListPage renders the work order templates management page.
func WorkOrdersTemplateListPage(data WorkOrderTemplateListData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	return func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("space-y-6"),
			// Header
			b.Div(mi.Class("flex items-center justify-between"),
				b.Div(
					b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), t("workorders-templates.list_title")),
					b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400"), t("workorders-templates.subtitle")),
				),
				b.A(mi.Href("/workorders-templates/new"), mi.Class("inline-flex items-center gap-2 px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 text-sm font-medium"),
					b.Span(mi.Class("material-icons text-lg"), "add"),
					t("workorders-templates.new"),
				),
			),

			// Templates grid
			WorkOrderTemplateGrid(b, data.Templates, t),
		)
	}
}

// WorkOrderTemplateGrid arranges template cards in a responsive grid.
// Used by WorkOrdersTemplateListPage to display the templates or an
// empty state when there are none.
func WorkOrderTemplateGrid(b *mi.Builder, types []WorkOrderTemplateRow, t func(key string, args ...any) string) mi.Node {
	if len(types) == 0 {
		return Card("", EmptyState("category", t("workorders-templates.empty_title"), t("workorders-templates.empty_message"), nil))(b)
	}

	cards := make([]interface{}, len(types))
	for i, row := range types {
		cards[i] = WorkOrderTemplateCard(b, row, t)
	}

	return b.Div(append([]interface{}{mi.Class("grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4")}, cards...)...)
}

// WorkOrderTemplateCard renders a linked summary with the template's name,
// description, and order count when available. Used by WorkOrderTemplateGrid
// to link each entry to its detail page.
func WorkOrderTemplateCard(b *mi.Builder, row WorkOrderTemplateRow, t func(key string, args ...any) string) mi.Node {
	icon := "category"
	if row.Icon != "" {
		icon = row.Icon
	}

	color := strings.TrimSpace(row.Color)
	if color == "" {
		color = "#6366f1"
	}

	count := "—"
	if !row.OrderCountUnavailable {
		count = t("workorders-templates.field_count", row.OrderCount)
	}
	return b.A(mi.Href(fmt.Sprintf("/workorders-templates/%d", row.ID)),
		mi.Class("block bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-5 hover:shadow-md hover:border-indigo-300 dark:hover:border-indigo-600 transition-all"),
		b.Div(mi.Class("flex items-start justify-between mb-3"),
			b.Div(
				mi.Class("w-10 h-10 rounded-lg flex items-center justify-center"),
				mi.Style("background-color: "+color+";"),
				b.Span(
					mi.Class("material-icons text-indigo-600 dark:text-indigo-400"),
					icon,
				),
			),
			b.Span(mi.Class("material-icons text-gray-400"), "chevron_right"),
		),
		b.H3(mi.Class("font-semibold text-gray-900 dark:text-white mb-1"), row.Name),
		b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400 mb-3 line-clamp-2"), row.Description),
		b.Div(mi.Class("flex items-center gap-4 text-xs text-gray-500 dark:text-gray-400"),
			b.Span(count),
		),
	)
}

// WorkOrderTemplateFormData holds data for the work order template create/edit form.
type WorkOrderTemplateFormData struct {
	// Form state
	ID          int64
	IsEdit      bool
	IsPartial   bool // true when rendered inside SeamModal2
	CSRFToken   string
	Errors      map[string]string
	FormContent mi.H

	// Field values
	Name        string
	Description string
	Icon        string
	Color       string
	FSMID       string
	// Prefix map[string]bool enrealidad el prefijo se elije uno u otro no seria este lo dejo comentado por ahora
	HeaderFields []FieldRow
	StateFields  []FieldRow
	// P-21: default alert configuration
	// DefaultAlertaID int64
	// Alertas         []AlertaRow // dropdown data
}

// WorkOrderTemplateFormPage renders the create or edit form for an work order template.
func WorkOrderTemplateFormPage(data WorkOrderTemplateFormData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	errorTabs := workOrderTemplateErrorTabs(data.Errors)

	return func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("space-y-6 max-w"),
			b.Div(mi.Class("flex items-center gap-4"),
				b.A(mi.Href("/workorders-templates"), mi.Class("text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"),
					b.Span(mi.Class("material-icons"), "arrow_back"),
				),
				b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), t("workorders-templates.new")),
			),
			Card("", func(b *mi.Builder) mi.Node {
				return b.Form(
					mi.ID("wo-template-step1-form"),
					mi.Class("space-y-4"),
					mi.Method("post"),
					mi.Action("/workorders-templates/validate-step-1"),
					mi.Attr("hx-post", "/workorders-templates/validate-step-1"),
					mi.Attr("hx-target", "#wo-template-step1-fields"),
					mi.Attr("hx-swap", "innerHTML settle:0ms"),
					mi.Attr("data-step2-title", t("workorders-templates.complete_title")),

					b.Div(
						mi.ID("wo-template-step1-fields"),
						data.FormContent(b),
					),

					workOrderTemplateTabErrorsScript(errorTabs)(b),

					b.Div(mi.Class("flex justify-end gap-3 pt-4"),
						b.A(
							mi.Href("/workorders-templates"),
							mi.Class("px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg font-medium hover:bg-gray-50 dark:hover:bg-gray-700"),
							t("action.cancel"),
						),
						b.Button(
							mi.Type("submit"),
							mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg"),
							t("action.next"),
						),
					),
				)
			})(b),
			b.Script(mi.Raw(`
				(function () {
					// Avoid registering the listener multiple times when reloading via HTMX..
					if (window.WorkOrderTemplateWizard) return;

					window.WorkOrderTemplateWizard = {};

					document.body.addEventListener('woTemplateStep1Valid', function () {
						var form = document.getElementById('wo-template-step1-form');
    					if (!form) return;

						SeamModal2.open(
							form.dataset.step2Title,
							'/workorders-templates/step-2',
							{maxWidth: '1000px'}
						);
					});

					document.body.addEventListener('htmx:configRequest', function (event) {
						var request = event.detail;

						// Only modifies the initial load of this modal.
						if (request.path !== '/workorders-templates/step-2') return;
						if (request.verb.toLowerCase() !== 'get') return;
						if (!request.target ||
							request.target.id !== 'seam-modal2-body') return;

						var form = document.getElementById('wo-template-step1-form');
						if (!form) {
							event.preventDefault();
							return;
						}

						request.verb = 'post';
						request.useUrlParams = false;
						request.headers['Content-Type'] =
							'application/x-www-form-urlencoded';

						Object.assign(
							request.parameters,
							htmx.values(form, 'post')
						);
					});
				})();
				`)),
		)
	}
}

// WorkOrderTemplateStep2Page wraps FormEngine content for SeamModal2.
// The form includes first-step controls in its POST so both steps are saved
// together. Action buttons are moved to the modal footer after the HTMX swap.
func WorkOrderTemplateStep2Page(content mi.H, fieldTypes map[string]string, t func(string, ...any) string) mi.H {
	// All values are strings or collections of strings.
	conditionConfig, _ := json.Marshal(map[string]interface{}{
		"fieldTypes": fieldTypes,
		"operators":  workOrderTemplateConditionOperators,
		"yes":        t("wo_action.parameters.yes"),
		"no":         t("wo_action.parameters.no"),
		"empty":      t("field.select.empty"),
		"valueLabel": t("repeatable_item.conditions.comparison_value"),
		"assetID":    t("events.field.asset_id"),
	})

	return func(b *mi.Builder) mi.Node {
		return b.Div(
			b.Form(
				mi.ID("wo-template-step2-form"),
				mi.Class("space-y-4"),
				mi.Attr("hx-post", "/workorders-templates"),
				mi.Attr("hx-target", "#seam-modal2-body"),
				mi.Attr("hx-swap", "innerHTML settle:0ms"),
				mi.Attr("data-rule-conditions", string(conditionConfig)),

				// Also includes the page fields.
				mi.Attr("hx-include", "#wo-template-step1-form"),

				content(b),
				b.Script(mi.Raw(templateActionMappingScript)),
				b.Script(mi.Raw(templateRuleTriggerScript)),
				b.Script(mi.Raw(templateRuleConditionScript)),
			),
			b.Div(
				mi.Attr("data-modal-footer", ""),
				b.Button(
					mi.Type("button"),
					mi.Class("px-4 py-2 bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600 text-sm font-medium"),
					mi.Attr("onclick", "SeamModal2.close()"),
					t("action.cancel"),
				),
				b.Button(
					mi.Type("submit"),
					mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 text-sm font-medium"),
					mi.Attr("form", "wo-template-step2-form"),
					t("action.save"),
				),
			),
		)
	}
}

// WorkOrderTemplateDetailData holds data for the work-order template detail page.
type WorkOrderTemplateDetailData struct {
	WorkOrderTemplate WorkOrderTemplate
	AssetType         AssetTypeBasicData
	Consumables       []TemplateConsumableDetail
	Actions           []WorkOrderTemplateActionDetail
	Rules             []WorkOrderTemplateRulesDetail
	CSRFToken         string
	// CompleteForm      mi.H   // non-nil only when WorkOrder.State == "in_progress"
	WalkError string // set when a transition attempt (e.g. Complete without a signature) was rejected
}

// AssetTypeBasicData holds basic data for the asset type section in the Work Order Template detail/view page.
type AssetTypeBasicData struct {
	ID          int64
	Name        string
	Slug        string
	Description string
	Icon        string
	Color       string
	FieldCount  int
}

// TemplateConsumableDetail holds basic data for the consumable section in the Work Order Template detail/view page.
type TemplateConsumableDetail struct {
	ConsumableID int64
	Name         string
	Quantity     float64
}

type WorkOrderTemplateAction struct {
	ActionID       int64                            `json:"action"`
	TriggerPoint   string                           `json:"trigger_point"`
	BlockReference string                           `json:"block_reference"`
	MappingValues  []WorkOrderTemplateActionMapping `json:"mapping_values"`
}

type WorkOrderTemplateActionMapping struct {
	Parameter   string      `json:"parameter"`
	Source      string      `json:"source"`
	ContextPath string      `json:"context_path"`
	FixedValue  interface{} `json:"fixed_value"`
	Transform   string      `json:"transform"`
}

type WorkOrderTemplateActionDetail struct {
	ActionID       int64
	Name           string
	TriggerPoint   string
	BlockReference string
	Mappings       []WorkOrderTemplateActionMappingDetail
}

type WorkOrderTemplateActionMappingDetail struct {
	Parameter  string
	Origin     string
	Value      string
	Conversion string
}

type WorkOrderTemplateRulesDetail struct {
	Name          string
	Trigger       string
	Conditions    []WorkOrderTemplateRuleConditionDetail
	Action        string
	Target        string
	StopExecution bool
	Severity      string
	Message       string
}

type WorkOrderTemplateRuleConditionDetail struct {
	Expression string
	Connector  string
}

// WorkOrderTemplateDetailPage renders the work-order template detail page
func WorkOrderTemplateDetailPage(data WorkOrderTemplateDetailData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	wo := data.WorkOrderTemplate

	latestLabel := t("workorders-templates.outdated")
	var variant = "default"

	if wo.Latest {
		latestLabel = t("workorders-templates.latest")
		variant = "success"
	}

	return func(b *mi.Builder) mi.Node {

		return b.Div(mi.Class("space-y-6"),
			b.Div(mi.Class("flex items-center justify-between"),
				b.Div(
					// Breadcrumb
					b.Nav(mi.Class("text-sm text-gray-500 dark:text-gray-400 mb-2"),
						b.A(mi.Href("/workorders-templates"), mi.Class("hover:text-gray-700 dark:hover:text-gray-300"), t("workorders-templates")),
						b.Span(mi.Class("mx-2"), "/"),
						b.Span(mi.Class("text-gray-900 dark:text-white"), wo.Name),
					),
					// Title with status
					b.Div(mi.Class("flex items-center gap-3"),
						b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"),
							wo.Name,
						),
						Badge(latestLabel, variant)(b),
					),
				),
				// Actions
				b.Div(mi.Class("flex items-center gap-2"),
					b.Button(
						mi.Class("inline-flex items-center gap-1 px-3 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700"),
						mi.Attr("hx-get", fmt.Sprintf("/workorders-templates/%d/edit", wo.ID)),
						mi.Attr("hx-target", "#seam-modal-body"),
						mi.Attr("hx-swap", "innerHTML"),
						mi.Attr("onclick", fmt.Sprintf("SeamModal.open('%s')", t("workorders-templates.edit"))),
						b.Span(mi.Class("material-icons text-base leading-none"), "edit"),
						t("action.edit"),
					),
					Button(t("action.delete"), "danger",
						mi.Attr("onclick", fmt.Sprintf("SeamConfirm.showDanger('%s',function(){htmx.ajax('DELETE','/workorders-templates/%d/delete',{target:document.body,swap:'innerHTML'})},{title:'%s'})", t("workorders-templates.confirm_delete"), wo.ID, t("workorders-templates.delete_title"))),
					)(b),
				),
			),

			b.Div(mi.Class("grid grid-cols-1 lg:grid-cols-3 gap-6"),

				// Main information: 2/3 width
				b.Div(mi.Class("lg:col-span-2 min-w-0 space-y-6"),
					Card(
						t("workorders-templates.info_title"),
						templateInfoContent(wo, t),
					)(b),
					Card(
						t("workorders-templates.header_title"),
						templateHeaderContent(wo, t),
					)(b),
					Card(
						t("workorders-templates.block_title"),
						templateBlockContent(wo, t),
					)(b),
				),

				// Sidebar: 1/3 width
				b.Div(mi.Class("space-y-6"),
					Card(
						t("workorders-templates.scope_title"),
						templateScopeContent(wo, data.AssetType, t),
					)(b),
					Card(
						t("workorders-templates.numbering_title"),
						templateNumberingContent(wo, t),
					)(b),
					Card(
						t("workorders-templates.workflow_title"),
						templateWorkflowContent(wo),
					)(b),
					Card(
						t("workorders-templates.consumables"),
						templateConsumablesContent(wo, data.Consumables, t),
					)(b),
				),
			),
			Card(t("workorders-templates.actions.title"), templateActionsContent(data.Actions, t))(b),
			Card(t("workorders-templates.rules.title"), templateRulesContent(data.Rules, t))(b),
		)

	}
}

// templateInfoContent renders the template name, description, and version.
// Used by WorkOrderTemplateDetailPage for the Information card.
func templateInfoContent(wo WorkOrderTemplate, t func(key string, args ...any) string) mi.H {
	description := wo.Description
	if description == "" {
		description = t("field.no_description")
	}

	return func(b *mi.Builder) mi.Node {
		return b.Dl(mi.Class("grid grid-cols-1 sm:grid-cols-2 gap-4"),
			infoRow(b, t("field.name"), wo.Name),
			infoRow(b, t("workorders-templates.version"), tLatest(wo.Latest, wo.Version, t)),

			b.Div(mi.Class("sm:col-span-2"),
				infoRow(b, t("field.description"), description),
			),
		)
	}
}

// templateHeaderContent renders the configured header fields with their
// types, defaults, filling phase, and required status. Used by
// WorkOrderTemplateDetailPage for the Header Fields card.
func templateHeaderContent(wo WorkOrderTemplate, t func(key string, args ...any) string) mi.H {
	rows := make([]mi.H, len(wo.HeaderItems))

	for i, hi := range wo.HeaderItems {
		toFillOutLabel := t("workorders-template.to_fill_out.upon_creating")
		var variant1 = "success"

		if hi.ToFillOut == "upon_executing" {
			toFillOutLabel = t("workorders-template.to_fill_out.upon_executing")
			variant1 = "primary"
		}

		requiredLabel := t("action.no")
		var variant2 = "default"

		if hi.Required {
			requiredLabel = t("action.yes")
			variant2 = "success"
		}

		rows[i] = ListRow(i,

			ListCell(hi.Tag),

			ListCellMuted(workOrderTemplateFieldTypeLabel(hi.Type, t)),

			ListCell(hi.DefaultValue),

			ListCellRich(func(b *mi.Builder) mi.Node {
				return Badge(toFillOutLabel, variant1)(b)
			}),

			ListCellRich(func(b *mi.Builder) mi.Node {
				return Badge(requiredLabel, variant2)(b)
			}),
		)
	}

	return ListPage(
		ListPageConfig{},
		ListTable(ListTableConfig{
			Columns: []ListColumn{
				{Label: t("repeatable_item.header_items.tag")},
				{Label: t("repeatable_item.header_items.type")},
				{Label: t("repeatable_item.header_items.default_value")},
				{Label: t("repeatable_item.header_items.to_fill_out")},
				{Label: t("repeatable_item.header_items.required")},
			},
			EmptyIcon:    "code_off",
			EmptyTitle:   t("workorders-templates.header_title"),
			EmptyMessage: t("workorders-templates.header_items.empty"),
		}, rows),
		nil,
		nil)
}

// templateBlockContent renders each block with its predecessor, responsible
// type, and configured lines. Used by WorkOrderTemplateDetailPage for
// the Blocks card.
func templateBlockContent(wo WorkOrderTemplate, t func(key string, args ...any) string) mi.H {

	return func(b *mi.Builder) mi.Node {
		if len(wo.Blocks) == 0 {
			return b.P(
				mi.Class("text-sm text-gray-500 dark:text-gray-400"),
				t("workorders-templates.blocks.empty"),
			)
		}

		//List of blocks
		children := []interface{}{
			mi.Class("flex w-full min-w-0 gap-4 overflow-x-auto pb-4"),
			mi.Attr("tabindex", "0"),
			mi.Attr("aria-label", t("workorders-templates.block_title")),
		}

		//Individual Blocks
		for _, block := range wo.Blocks {
			predecessor := block.Predecessor
			if predecessor == "" {
				predecessor = "—"
			}

			responsible := "—"
			if block.ResponsableType != "" {
				responsible = t(
					"workorders-template.responsible_type." +
						block.ResponsableType,
				)
			}

			//Lines
			rows := make([]mi.H, len(block.Lines))
			for i, line := range block.Lines {
				rows[i] = ListRow(
					i,
					ListCell(line.Name),
					ListCellMuted(workOrderTemplateFieldTypeLabel(line.Type, t)),
				)
			}

			table := ListTable(ListTableConfig{
				Columns: []ListColumn{
					{Label: t("field.name")},
					{Label: t("field.type")},
				},
				EmptyIcon:    "list",
				EmptyTitle:   t("repeatable_item.blocks.lines"),
				EmptyMessage: t("workorders-templates.blocks.lines_empty"),
			}, rows)

			// Add block to list of blocks
			children = append(children,
				b.Div(
					mi.Class("w-100 max-w-full shrink-0 self-stretch rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 p-4"),

					b.Div(
						mi.Class("grid grid-cols-2 gap-3 mb-4"),
						b.H4(
							mi.Class("mb-4 font-semibold break-words text-gray-900 dark:text-white"),
							block.Name,
						),
						b.Div(
							mi.Class("min-w-0 text-right"),
							b.P(
								mi.Class("text-xs text-gray-500 dark:text-gray-400"),
								t("repeatable_item.blocks.responsible_type"),
							),
							b.P(
								mi.Class("text-sm text-gray-900 dark:text-white"),
								responsible,
							),
						),
					),

					b.Div(
						mi.Class("min-w-0"),
						b.P(
							mi.Class("text-xs text-gray-500 dark:text-gray-400"),
							t("repeatable_item.blocks.predecessor"),
						),
						b.P(
							mi.Class("mb-4 text-sm break-words text-gray-900 dark:text-white"),
							predecessor,
						),
					),

					b.H5(
						mi.Class("mb-2 text-sm font-medium text-gray-700 dark:text-gray-300"),
						t("repeatable_item.blocks.lines"),
					),

					table(b),
				),
			)
		}

		return b.Div(children...)
	}
}

// templateScopeContent shows the permitted asset type and asset count limits.
// Used by WorkOrderTemplateDetailPage for the Scope card.
func templateScopeContent(wo WorkOrderTemplate, at AssetTypeBasicData, t func(key string, args ...any) string) mi.H {
	assetsPermitted := at.Name
	if assetsPermitted == "" {
		assetsPermitted = t("workorders-templates.asset_types_any_allowed")
	}

	return func(b *mi.Builder) mi.Node {
		return b.Dl(mi.Class("grid grid-cols-1 sm:grid-cols-2 gap-4"),
			infoRow(b, t("workorders-templates.asset_types_allowed"), assetsPermitted),
			infoRow(b, t("workorders-templates.asset_types_amount"), tAmountAssets(wo.MinAssets, wo.MaxAssets, t)),
		)
	}
}

// templateNumberingContent formats an example order number using the
// template's prefix, sequence length, and suffix. Used by
// WorkOrderTemplateDetailPage to preview the numbering format.
func templateNumberingContent(wo WorkOrderTemplate, t func(key string, args ...any) string) mi.H {
	var numbers string = "0"
	for range wo.SeqLength - 2 {
		numbers = numbers + "0"
	}
	numbers = numbers + "1"

	mask := fmt.Sprintf("%s-%s-%s", wo.Prefix, numbers, wo.Suffix)

	return func(b *mi.Builder) mi.Node {
		return b.Dl(mi.Class("grid grid-cols-1 sm:grid-cols-2 gap-4"),
			infoRow(b, t("workorders-templates.numbering"), mask),
		)
	}
}

// templateWorkflowContent renders the workflow states in their configured
// order. Used by WorkOrderTemplateDetailPage for the workflow card.
func templateWorkflowContent(wo WorkOrderTemplate) mi.H {
	states := make([]SimpleListItem, len(wo.WorkflowStates))
	for i, iss := range wo.WorkflowStates {
		states[i] = SimpleListItem{
			Title: iss.Name,
		}
	}

	return func(b *mi.Builder) mi.Node {

		return b.Dl(mi.Class("grid grid-cols-1 sm:grid-cols-2 gap-4"),
			SimpleListCard(states, "error")(b),
		)
	}
}

// templateConsumablesContent renders the consumable settings, including
// suggested quantities and whether extras are allowed. Used by
// WorkOrderTemplateDetailPage for the Consumables card.
func templateConsumablesContent(wo WorkOrderTemplate, items []TemplateConsumableDetail, t func(key string, args ...any) string) mi.H {
	rows := make([]mi.H, len(items))
	for i, item := range items {
		rows[i] = ListRow(
			i,
			ListCell(item.Name),
			ListCell(strconv.FormatFloat(item.Quantity, 'f', -1, 64)),
		)
	}

	extraLabel := t("action.no")
	if wo.ExtraConsumables {
		extraLabel = t("action.yes")
	}

	table := ListTable(ListTableConfig{
		Columns: []ListColumn{
			{Label: t("repeatable_item.consumables.consumable_id")},
			{Label: t("repeatable_item.consumables.quantity")},
		},
		EmptyIcon:    "shelves",
		EmptyTitle:   t("workorders-templates.consumables"),
		EmptyMessage: t("workorders-templates.consumables.empty"),
	}, rows)

	return func(b *mi.Builder) mi.Node {
		if !wo.ConsumablesEnabled {
			return b.P(
				mi.Class("text-sm text-gray-500 dark:text-gray-400"),
				t("workorders-templates.consumables.not_enabled"),
			)
		}

		return b.Div(
			mi.Class("space-y-4"),

			b.Dl(
				infoRow(b, t("workorders-templates.consumables.extra_allowed"), extraLabel),
			),

			b.Div(
				mi.Class("overflow-x-auto"),
				table(b),
			),
		)
	}
}

// templateActionsContent renders configured actions with their triggers,
// references, and parameter mappings. Used by WorkOrderTemplateDetailPage
// for the Actions card.
func templateActionsContent(items []WorkOrderTemplateActionDetail, t func(key string, args ...any) string) mi.H {
	textCell := func(value string) mi.H {
		return ListCellRich(func(b *mi.Builder) mi.Node {
			return b.Span(
				mi.Class(
					"whitespace-pre-wrap break-words "+
						"text-gray-900 dark:text-gray-100",
				),
				value,
			)
		})
	}

	return func(b *mi.Builder) mi.Node {
		if len(items) == 0 {
			return b.P(
				mi.Class("text-sm text-gray-500 dark:text-gray-400"),
				t("workorders-templates.actions.empty"),
			)
		}

		children := []interface{}{
			mi.Class("space-y-4"),
		}

		for index, item := range items {
			when := item.TriggerPoint

			switch when {
			case "before_work_order", "after_work_order",
				"before_block", "after_block":
				when = t("workorders-template.trigger_point." + when)
			case "":
				when = "—"
			}

			if item.BlockReference != "" {
				when += " · " + t(
					"workorders-templates.option.block",
					item.BlockReference,
				)
			}

			rows := make([]mi.H, len(item.Mappings))

			for i, mapping := range item.Mappings {
				rows[i] = ListRow(
					i,
					textCell(mapping.Parameter),
					textCell(mapping.Origin),
					textCell(mapping.Value),
					textCell(mapping.Conversion),
				)
			}

			table := ListTable(ListTableConfig{
				Columns: []ListColumn{
					{
						Label: t(
							"workorders-templates.actions.parameter",
						),
					},
					{
						Label: t(
							"workorders-templates.actions.origin",
						),
					},
					{
						Label: t(
							"workorders-templates.actions.value",
						),
					},
					{
						Label: t(
							"workorders-templates.actions.conversion",
						),
					},
				},
				EmptyIcon:  "link",
				EmptyTitle: t("wo_action.parameters.title"),
				EmptyMessage: t(
					"workorders-templates.actions.no_mappings",
				),
			}, rows)

			children = append(children,
				b.Div(
					mi.Class(
						"min-w-0 rounded-lg border "+
							"border-gray-200 dark:border-gray-700 p-4",
					),
					b.H3(
						mi.Class(
							"font-semibold text-gray-900 dark:text-white",
						),
						fmt.Sprintf("%d. %s", index+1, item.Name),
					),
					b.P(
						mi.Class(
							"mt-1 mb-4 text-sm "+
								"text-gray-500 dark:text-gray-400",
						),
						when,
					),
					b.Div(
						mi.Class("overflow-x-auto"),
						table(b),
					),
				),
			)
		}

		return b.Div(children...)
	}
}

// templateRulesContent renders business rules with their conditions,
// logical connectors, actions, and alert details. Used by
// WorkOrderTemplateDetailPage for the Business Rules card.
func templateRulesContent(items []WorkOrderTemplateRulesDetail, t func(string, ...any) string) mi.H {
	textCell := func(value string) mi.H {
		return ListCellRich(func(b *mi.Builder) mi.Node {
			return b.Span(
				mi.Class(
					"whitespace-pre-wrap break-words "+
						"text-gray-900 dark:text-gray-100",
				),
				value,
			)
		})
	}

	return func(b *mi.Builder) mi.Node {
		if len(items) == 0 {
			return b.P(
				mi.Class("text-sm text-gray-500 dark:text-gray-400"),
				t("workorders-templates.rules.empty"),
			)
		}

		cards := []any{mi.Class("min-w-0 space-y-4")}
		for _, rule := range items {
			conditions := []any{mi.Class("flex items-center gap-3")}
			for i, condition := range rule.Conditions {
				row := make([]mi.H, 1)
				row[0] = ListRow(
					0,
					textCell(condition.Expression),
				)

				table := ListTable(ListTableConfig{
					Columns: []ListColumn{
						{
							Label: t("workorders-templates.validation.condition", i+1),
						},
					},
					EmptyIcon:  "link",
					EmptyTitle: t("wo_action.parameters.title"),
					EmptyMessage: t(
						"workorders-templates.actions.no_mappings",
					),
				}, row)
				conditions = append(conditions, b.Div(
					mi.Class("overflow-x-auto"),
					table(b),
				))

				if i+1 < len(rule.Conditions) && (condition.Connector == "And" || condition.Connector == "Or") {
					conditions = append(conditions, b.Span(
						mi.Class("shrink-0 rounded-md border border-gray-300 dark:border-gray-600 px-2 py-1 text-xs font-semibold text-gray-900 dark:text-white"),
						strings.ToUpper(condition.Connector),
					))
				}
			}

			stop := t("action.no")
			if rule.StopExecution {
				stop = t("action.yes")
			}

			alert := b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400"), "—")
			if rule.Severity != "" || rule.Message != "" {
				variant := map[string]string{"INFO": "info", "WARNING": "warning", "ERROR": "danger"}[rule.Severity]
				alert = b.Div(
					mi.Class("space-y-2"),
					Badge(rule.Severity, variant)(b),
					b.P(mi.Class("text-sm whitespace-pre-wrap break-words text-gray-900 dark:text-white"), rule.Message),
				)
			}

			cards = append(cards, b.Article(
				mi.Class("min-w-0 space-y-4 rounded-lg border border-gray-200 dark:border-gray-700 p-4"),
				b.Div(
					b.H4(mi.Class("font-semibold text-gray-900 dark:text-white"), rule.Name),
					b.P(mi.Class("mt-1 text-sm text-gray-500 dark:text-gray-400"), rule.Trigger),
				),
				b.Div(
					mi.Class("overflow-x-auto pb-2"),
					mi.Attr("tabindex", "0"),
					mi.Attr("role", "region"),
					mi.Attr("aria-label", t("repeatable_item.rule_item.conditions")+" · "+rule.Name),
					b.Div(conditions...),
				),
				b.Div(
					mi.Class("grid grid-cols-1 gap-4 border-t border-gray-200 dark:border-gray-700 pt-4 sm:grid-cols-3"),
					b.Dl(mi.Class("space-y-3"),
						infoRow(b, t("repeatable_item.rule_item.action_type"), rule.Action),
						infoRow(b, t("repeatable_item.rule_item.target_field"), rule.Target),
					),
					b.Dl(
						infoRow(b, t("repeatable_item.rule_item.stop_execution"), stop),
					),
					b.Dl(
						b.Dt(mi.Class("mb-1 text-sm font-medium text-gray-500 dark:text-gray-400"),
							t("workorders-templates.rules.message"),
						),
						b.Dd(alert),
					),
				),
			))
		}

		return b.Div(cards...)
	}
}
