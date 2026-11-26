package api

import (
	"fmt"
	"net/http"

	"github.com/ha1tch/queryfy"
	"github.com/ha1tch/seam-ui/internal/rules"
	"github.com/ha1tch/seam-ui/internal/validation"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

// RuleHandler handles rule-related API endpoints.
type RuleHandler struct {
	*Handler
	Engine *rules.Engine
}

// NewRuleHandler creates a new rule handler.
func NewRuleHandler(h *Handler, engine *rules.Engine) *RuleHandler {
	return &RuleHandler{
		Handler: h,
		Engine:  engine,
	}
}

// List handles GET /api/v1/rules
func (h *RuleHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	params := GetPaginationParams(r)

	// Filters
	enabled := r.URL.Query().Get("enabled")

	if enabled != "" {
		enabledBool := enabled == "true"
		fetchCount := params.Limit + params.Offset
		query := fmt.Sprintf("SELECT TOP %d * FROM rules WHERE enabled = %t",
			fetchCount, enabledBool)

		result, err := client.OQL(ctx, query)
		if err != nil {
			WriteXOLUError(w, err)
			return
		}

		// Apply offset by slicing
		data := result.Data
		if params.Offset > 0 && params.Offset < len(data) {
			data = data[params.Offset:]
		} else if params.Offset >= len(data) {
			data = []map[string]any{}
		}
		if len(data) > params.Limit {
			data = data[:params.Limit]
		}

		WriteJSON(w, http.StatusOK, NewListResponse(data, len(data), params.Limit, params.Offset))
		return
	}

	rulesResult, err := client.List(ctx, "rules", &xolu.ListParams{
		Limit:  params.Limit,
		Offset: params.Offset,
		Sort:   params.Sort,
	})
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, NewListResponse(rulesResult.Entities, rulesResult.TotalItems, params.Limit, params.Offset))
}

// Get handles GET /api/v1/rules/{id}
func (h *RuleHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid rule ID")
		return
	}

	rule, err := client.Get(ctx, "rules", id)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, rule)
}

// Create handles POST /api/v1/rules
func (h *RuleHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	var data map[string]any
	if err := ReadJSON(r, &data); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	if err := queryfy.Validate(data, validation.RuleCreateSchema); err != nil {
		WriteValidationError(w, err)
		return
	}

	rule, err := client.Create(ctx, "rules", data)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	// Load rule into engine if enabled
	if enabled, ok := data["enabled"].(bool); ok && enabled {
		h.loadRuleIntoEngine(rule)
	}

	WriteJSON(w, http.StatusCreated, rule)
}

// Update handles PUT /api/v1/rules/{id}
func (h *RuleHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid rule ID")
		return
	}

	var data map[string]any
	if err := ReadJSON(r, &data); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	if err := queryfy.Validate(data, validation.RuleUpdateSchema); err != nil {
		WriteValidationError(w, err)
		return
	}

	rule, err := client.Update(ctx, "rules", id, data)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	// Reload rule in engine
	h.Engine.UnloadRule(id)
	if enabled, ok := data["enabled"].(bool); ok && enabled {
		h.loadRuleIntoEngine(rule)
	}

	WriteJSON(w, http.StatusOK, rule)
}

// Delete handles DELETE /api/v1/rules/{id}
func (h *RuleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid rule ID")
		return
	}

	if err := client.Delete(ctx, "rules", id); err != nil {
		WriteXOLUError(w, err)
		return
	}

	// Remove from engine
	h.Engine.UnloadRule(id)

	w.WriteHeader(http.StatusNoContent)
}

// Enable handles POST /api/v1/rules/{id}/enable
func (h *RuleHandler) Enable(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid rule ID")
		return
	}

	rule, err := client.Patch(ctx, "rules", id, map[string]any{"enabled": true})
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	h.loadRuleIntoEngine(rule)

	WriteJSON(w, http.StatusOK, rule)
}

// Disable handles POST /api/v1/rules/{id}/disable
func (h *RuleHandler) Disable(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid rule ID")
		return
	}

	rule, err := client.Patch(ctx, "rules", id, map[string]any{"enabled": false})
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	h.Engine.UnloadRule(id)

	WriteJSON(w, http.StatusOK, rule)
}

// Test handles POST /api/v1/rules/{id}/test
// Tests a rule against sample data without persisting.
func (h *RuleHandler) Test(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid rule ID")
		return
	}

	var testData struct {
		Event map[string]any `json:"event"`
		Asset map[string]any `json:"asset"`
	}
	if err := ReadJSON(r, &testData); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	// Fetch rule
	rule, err := client.Get(ctx, "rules", id)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	// Build test context and evaluate
	evalResult := h.testRuleEvaluation(rule, testData.Event, testData.Asset)

	WriteJSON(w, http.StatusOK, evalResult)
}

// loadRuleIntoEngine converts an xolu entity to a rules.Rule and loads it.
func (h *RuleHandler) loadRuleIntoEngine(entity *xolu.Entity) {
	if h.Engine == nil {
		return
	}

	// Convert entity to rules.Rule
	rule := &rules.Rule{
		ID:      entity.ID,
		Name:    getString(entity.Data, "name"),
		Enabled: getBool(entity.Data, "enabled"),
	}

	// Parse conditions
	if conditions, ok := entity.Data["conditions"].([]any); ok {
		for _, c := range conditions {
			if cond, ok := c.(map[string]any); ok {
				rule.Conditions = append(rule.Conditions, parseCondition(cond))
			}
		}
	}

	// Parse actions
	if actions, ok := entity.Data["actions"].([]any); ok {
		for _, a := range actions {
			if act, ok := a.(map[string]any); ok {
				rule.Actions = append(rule.Actions, parseAction(act))
			}
		}
	}

	h.Engine.LoadRule(rule)
}

// testRuleEvaluation evaluates a rule against test data.
func (h *RuleHandler) testRuleEvaluation(rule *xolu.Entity, eventData, assetData map[string]any) map[string]any {
	result := map[string]any{
		"rule_id":    rule.ID,
		"rule_name":  rule.Data["name"],
		"would_fire": false,
		"conditions": []map[string]any{},
		"actions":    rule.Data["actions"],
	}

	// Evaluate each condition
	conditions, ok := rule.Data["conditions"].([]any)
	if !ok {
		return result
	}

	allMet := true
	condResults := []map[string]any{}

	for _, c := range conditions {
		cond, ok := c.(map[string]any)
		if !ok {
			continue
		}

		met := h.evaluateTestCondition(cond, eventData, assetData)
		condResults = append(condResults, map[string]any{
			"condition": cond,
			"met":       met,
		})

		if !met {
			allMet = false
		}
	}

	result["conditions"] = condResults
	result["would_fire"] = allMet

	return result
}

// evaluateTestCondition evaluates a single condition for testing.
func (h *RuleHandler) evaluateTestCondition(cond, eventData, assetData map[string]any) bool {
	condType, _ := cond["type"].(string)

	switch condType {
	case "threshold":
		field, _ := cond["field"].(string)
		op, _ := cond["operator"].(string)
		threshold, _ := cond["value"].(float64)

		value, ok := eventData[field].(float64)
		if !ok {
			return false
		}

		switch op {
		case "gt":
			return value > threshold
		case "lt":
			return value < threshold
		case "eq":
			return value == threshold
		case "gte":
			return value >= threshold
		case "lte":
			return value <= threshold
		}

	case "status_change":
		fromStatus, _ := cond["from_status"].(string)
		toStatus, _ := cond["to_status"].(string)

		prevStatus, _ := assetData["previous_status"].(string)
		currStatus, _ := assetData["status"].(string)

		return prevStatus == fromStatus && currStatus == toStatus

	case "fsm_state":
		expectedState, _ := cond["fsm_state"].(string)
		actualState, _ := assetData["fsm_state"].(string)
		return expectedState == actualState
	}

	return false
}

// Helper functions
func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getBool(m map[string]any, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

func parseCondition(m map[string]any) rules.Condition {
	cond := rules.Condition{
		Type:     rules.ConditionType(getString(m, "type")),
		Field:    getString(m, "field"),
		Operator: rules.Operator(getString(m, "operator")),
	}

	if v, ok := m["value"].(float64); ok {
		cond.Value = v
	}

	return cond
}

func parseAction(m map[string]any) rules.Action {
	action := rules.Action{
		Type: rules.ActionType(getString(m, "type")),
	}

	if channels, ok := m["channels"].([]any); ok {
		for _, ch := range channels {
			if s, ok := ch.(string); ok {
				action.Channels = append(action.Channels, s)
			}
		}
	}

	action.Template = getString(m, "template")
	action.WebhookURL = getString(m, "webhook_url")

	return action
}
