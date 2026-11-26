// Package rules provides a rule engine for evaluating alert conditions.
//
// The rule engine processes events and asset state to determine if
// alert conditions are met. When conditions are met, it returns
// actions to execute (notifications, webhooks, asset updates).
//
// Rules support:
//   - Threshold conditions (value comparisons)
//   - Duration-based conditions (threshold exceeded for time)
//   - Rate of change conditions
//   - Geofence conditions (enter/exit)
//   - Status change conditions
//   - FSM state conditions
//
// Rules can be scoped to specific asset types and tags.
package rules

import (
	"fmt"
	"sync"
	"time"
)

// Rule represents an alert rule definition.
type Rule struct {
	ID           int64         `json:"id"`
	Name         string        `json:"name"`
	Description  string        `json:"description,omitempty"`
	Enabled      bool          `json:"enabled"`
	Priority     int           `json:"priority,omitempty"`
	TenantID     string        `json:"tenant_id"`
	AssetTypeIDs []int64       `json:"asset_type_ids,omitempty"`
	AssetTags    []string      `json:"asset_tags,omitempty"`
	Conditions   []Condition   `json:"conditions"`
	Actions      []Action      `json:"actions"`
	Cooldown     time.Duration `json:"cooldown,omitempty"`
}

// ConditionType defines the type of rule condition.
type ConditionType string

const (
	// ConditionThreshold triggers when value crosses threshold.
	ConditionThreshold ConditionType = "threshold"
	// ConditionThresholdDuration triggers when threshold exceeded for duration.
	ConditionThresholdDuration ConditionType = "threshold_duration"
	// ConditionChangeRate triggers when rate of change exceeds limit.
	ConditionChangeRate ConditionType = "change_rate"
	// ConditionGeofenceExit triggers when asset exits geofence.
	ConditionGeofenceExit ConditionType = "geofence_exit"
	// ConditionGeofenceEnter triggers when asset enters geofence.
	ConditionGeofenceEnter ConditionType = "geofence_enter"
	// ConditionStatusChange triggers when asset status changes.
	ConditionStatusChange ConditionType = "status_change"
	// ConditionFSMState triggers when asset enters FSM state.
	ConditionFSMState ConditionType = "fsm_state"
	// ConditionAbsent triggers when no event for duration.
	ConditionAbsent ConditionType = "absent"
)

// Operator defines comparison operators.
type Operator string

const (
	OpGT  Operator = "gt"
	OpLT  Operator = "lt"
	OpEQ  Operator = "eq"
	OpGTE Operator = "gte"
	OpLTE Operator = "lte"
	OpNE  Operator = "ne"
)

// Condition defines a single rule condition.
type Condition struct {
	Type       ConditionType `json:"type"`
	Field      string        `json:"field,omitempty"`
	Operator   Operator      `json:"operator,omitempty"`
	Value      float64       `json:"value,omitempty"`
	Duration   time.Duration `json:"duration,omitempty"`
	GeofenceID int64         `json:"geofence_id,omitempty"`
	FromStatus string        `json:"from_status,omitempty"`
	ToStatus   string        `json:"to_status,omitempty"`
	FSMState   string        `json:"fsm_state,omitempty"`
}

// ActionType defines the type of action to execute.
type ActionType string

const (
	// ActionNotify sends a notification.
	ActionNotify ActionType = "notify"
	// ActionWebhook calls a webhook.
	ActionWebhook ActionType = "webhook"
	// ActionUpdateAsset updates asset status/state.
	ActionUpdateAsset ActionType = "update_asset"
	// ActionCreateEvent creates a derived event.
	ActionCreateEvent ActionType = "create_event"
)

// Action defines what to do when rule conditions are met.
type Action struct {
	Type           ActionType        `json:"type"`
	Channels       []string          `json:"channels,omitempty"`
	Template       string            `json:"template,omitempty"`
	Recipients     []string          `json:"recipients,omitempty"`
	WebhookURL     string            `json:"webhook_url,omitempty"`
	WebhookMethod  string            `json:"webhook_method,omitempty"`
	WebhookHeaders map[string]string `json:"webhook_headers,omitempty"`
	SetStatus      string            `json:"set_status,omitempty"`
	SetState       map[string]any    `json:"set_state,omitempty"`
}

// Event represents an incoming event to evaluate.
type Event struct {
	AssetID     int64          `json:"asset_id"`
	Timestamp   time.Time      `json:"timestamp"`
	TriggerType string         `json:"trigger_type"`
	Value       map[string]any `json:"value"`
	Location    *Location      `json:"location,omitempty"`
}

// Location represents geographic coordinates.
type Location struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// Asset represents the current asset state for evaluation.
type Asset struct {
	ID             int64          `json:"id"`
	TypeID         int64          `json:"type_id"`
	Status         string         `json:"status"`
	PreviousStatus string         `json:"previous_status,omitempty"`
	FSMState       string         `json:"fsm_state,omitempty"`
	Tags           []string       `json:"tags,omitempty"`
	State          map[string]any `json:"state,omitempty"`
	LastEventTime  *time.Time     `json:"last_event_time,omitempty"`
	Location       *Location      `json:"location,omitempty"`
}

// EvaluationContext provides context for rule evaluation.
type EvaluationContext struct {
	Event         *Event
	Asset         *Asset
	PreviousValue map[string]any // For rate of change calculations
}

// EvaluationResult represents the result of evaluating rules.
type EvaluationResult struct {
	RuleID      int64
	RuleName    string
	Triggered   bool
	Actions     []Action
	TriggeredAt time.Time
	Conditions  []ConditionResult
}

// ConditionResult represents the result of a single condition check.
type ConditionResult struct {
	Condition Condition
	Met       bool
	Reason    string
}

// Engine evaluates rules against events and asset state.
type Engine struct {
	rules     map[int64]*Rule
	cooldowns map[string]time.Time // key: "rule:asset" -> last trigger time
	mu        sync.RWMutex
}

// NewEngine creates a new rule engine.
func NewEngine() *Engine {
	return &Engine{
		rules:     make(map[int64]*Rule),
		cooldowns: make(map[string]time.Time),
	}
}

// LoadRule loads a rule into the engine.
func (e *Engine) LoadRule(rule *Rule) error {
	if rule.ID == 0 {
		return fmt.Errorf("rule ID is required")
	}
	if rule.Name == "" {
		return fmt.Errorf("rule name is required")
	}
	if len(rule.Conditions) == 0 {
		return fmt.Errorf("rule must have at least one condition")
	}
	if len(rule.Actions) == 0 {
		return fmt.Errorf("rule must have at least one action")
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules[rule.ID] = rule
	return nil
}

// UnloadRule removes a rule from the engine.
func (e *Engine) UnloadRule(id int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.rules, id)
}

// GetRule retrieves a rule by ID.
func (e *Engine) GetRule(id int64) (*Rule, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	rule, ok := e.rules[id]
	return rule, ok
}

// RuleCount returns the number of loaded rules.
func (e *Engine) RuleCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.rules)
}

// Evaluate evaluates all applicable rules against an event and asset.
func (e *Engine) Evaluate(ctx *EvaluationContext) []*EvaluationResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var results []*EvaluationResult

	for _, rule := range e.rules {
		if !rule.Enabled {
			continue
		}

		// Check if rule applies to this asset
		if !e.ruleApplies(rule, ctx.Asset) {
			continue
		}

		// Check cooldown
		if e.isOnCooldown(rule.ID, ctx.Asset.ID) {
			continue
		}

		result := e.evaluateRule(rule, ctx)
		if result.Triggered {
			e.setCooldown(rule.ID, ctx.Asset.ID, rule.Cooldown)
		}
		results = append(results, result)
	}

	return results
}

// ruleApplies checks if a rule applies to the given asset.
func (e *Engine) ruleApplies(rule *Rule, asset *Asset) bool {
	// Check asset type filter
	if len(rule.AssetTypeIDs) > 0 {
		found := false
		for _, typeID := range rule.AssetTypeIDs {
			if typeID == asset.TypeID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check asset tags filter
	if len(rule.AssetTags) > 0 {
		found := false
		for _, ruleTag := range rule.AssetTags {
			for _, assetTag := range asset.Tags {
				if ruleTag == assetTag {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// isOnCooldown checks if a rule is on cooldown for an asset.
func (e *Engine) isOnCooldown(ruleID, assetID int64) bool {
	key := fmt.Sprintf("%d:%d", ruleID, assetID)
	lastTrigger, ok := e.cooldowns[key]
	if !ok {
		return false
	}
	return time.Now().Before(lastTrigger)
}

// setCooldown sets the cooldown for a rule-asset pair.
func (e *Engine) setCooldown(ruleID, assetID int64, cooldown time.Duration) {
	if cooldown <= 0 {
		return
	}
	key := fmt.Sprintf("%d:%d", ruleID, assetID)
	e.cooldowns[key] = time.Now().Add(cooldown)
}

// evaluateRule evaluates a single rule against the context.
func (e *Engine) evaluateRule(rule *Rule, ctx *EvaluationContext) *EvaluationResult {
	result := &EvaluationResult{
		RuleID:      rule.ID,
		RuleName:    rule.Name,
		Triggered:   true, // Assume true, set false if any condition fails
		TriggeredAt: time.Now(),
	}

	// All conditions must be met (AND logic)
	for _, cond := range rule.Conditions {
		condResult := e.evaluateCondition(cond, ctx)
		result.Conditions = append(result.Conditions, condResult)
		if !condResult.Met {
			result.Triggered = false
		}
	}

	if result.Triggered {
		result.Actions = rule.Actions
	}

	return result
}

// evaluateCondition evaluates a single condition.
func (e *Engine) evaluateCondition(cond Condition, ctx *EvaluationContext) ConditionResult {
	result := ConditionResult{Condition: cond}

	switch cond.Type {
	case ConditionThreshold:
		result.Met, result.Reason = e.evalThreshold(cond, ctx)
	case ConditionThresholdDuration:
		result.Met, result.Reason = e.evalThresholdDuration(cond, ctx)
	case ConditionChangeRate:
		result.Met, result.Reason = e.evalChangeRate(cond, ctx)
	case ConditionGeofenceExit:
		result.Met, result.Reason = e.evalGeofenceExit(cond, ctx)
	case ConditionGeofenceEnter:
		result.Met, result.Reason = e.evalGeofenceEnter(cond, ctx)
	case ConditionStatusChange:
		result.Met, result.Reason = e.evalStatusChange(cond, ctx)
	case ConditionFSMState:
		result.Met, result.Reason = e.evalFSMState(cond, ctx)
	case ConditionAbsent:
		result.Met, result.Reason = e.evalAbsent(cond, ctx)
	default:
		result.Reason = fmt.Sprintf("unknown condition type: %s", cond.Type)
	}

	return result
}

// evalThreshold evaluates a threshold condition.
func (e *Engine) evalThreshold(cond Condition, ctx *EvaluationContext) (bool, string) {
	if ctx.Event == nil {
		return false, "no event"
	}

	value, ok := getFieldValue(ctx.Event.Value, cond.Field)
	if !ok {
		return false, fmt.Sprintf("field %q not found", cond.Field)
	}

	numValue, ok := toFloat64(value)
	if !ok {
		return false, fmt.Sprintf("field %q is not a number", cond.Field)
	}

	met := compareValues(numValue, cond.Operator, cond.Value)
	reason := fmt.Sprintf("%s: %.2f %s %.2f", cond.Field, numValue, cond.Operator, cond.Value)
	return met, reason
}

// evalThresholdDuration evaluates a threshold duration condition.
func (e *Engine) evalThresholdDuration(cond Condition, ctx *EvaluationContext) (bool, string) {
	// This requires historical tracking - simplified implementation
	// In production, this would check if threshold has been exceeded continuously
	met, reason := e.evalThreshold(cond, ctx)
	if !met {
		return false, reason
	}
	// For now, assume threshold check is sufficient
	// Real implementation would track duration
	return true, fmt.Sprintf("%s (duration check simplified)", reason)
}

// evalChangeRate evaluates a rate of change condition.
func (e *Engine) evalChangeRate(cond Condition, ctx *EvaluationContext) (bool, string) {
	if ctx.Event == nil || ctx.PreviousValue == nil {
		return false, "insufficient data for rate calculation"
	}

	currentVal, ok := getFieldValue(ctx.Event.Value, cond.Field)
	if !ok {
		return false, fmt.Sprintf("field %q not found in current event", cond.Field)
	}

	previousVal, ok := getFieldValue(ctx.PreviousValue, cond.Field)
	if !ok {
		return false, fmt.Sprintf("field %q not found in previous event", cond.Field)
	}

	current, ok := toFloat64(currentVal)
	if !ok {
		return false, "current value not a number"
	}

	previous, ok := toFloat64(previousVal)
	if !ok {
		return false, "previous value not a number"
	}

	rate := current - previous
	if rate < 0 {
		rate = -rate
	}

	met := compareValues(rate, cond.Operator, cond.Value)
	return met, fmt.Sprintf("rate of change: %.2f %s %.2f", rate, cond.Operator, cond.Value)
}

// evalGeofenceExit evaluates a geofence exit condition.
func (e *Engine) evalGeofenceExit(cond Condition, ctx *EvaluationContext) (bool, string) {
	// Simplified - real implementation would check against geofence boundaries
	if ctx.Event == nil || ctx.Event.Location == nil {
		return false, "no location data"
	}
	// Placeholder - would need geofence lookup
	return false, "geofence check not implemented"
}

// evalGeofenceEnter evaluates a geofence enter condition.
func (e *Engine) evalGeofenceEnter(cond Condition, ctx *EvaluationContext) (bool, string) {
	// Simplified - real implementation would check against geofence boundaries
	if ctx.Event == nil || ctx.Event.Location == nil {
		return false, "no location data"
	}
	// Placeholder - would need geofence lookup
	return false, "geofence check not implemented"
}

// evalStatusChange evaluates a status change condition.
func (e *Engine) evalStatusChange(cond Condition, ctx *EvaluationContext) (bool, string) {
	if ctx.Asset == nil {
		return false, "no asset context"
	}

	// Check from_status if specified
	if cond.FromStatus != "" && ctx.Asset.PreviousStatus != cond.FromStatus {
		return false, fmt.Sprintf("previous status %q != %q", ctx.Asset.PreviousStatus, cond.FromStatus)
	}

	// Check to_status if specified
	if cond.ToStatus != "" && ctx.Asset.Status != cond.ToStatus {
		return false, fmt.Sprintf("current status %q != %q", ctx.Asset.Status, cond.ToStatus)
	}

	// If both specified, check transition
	if cond.FromStatus != "" && cond.ToStatus != "" {
		return true, fmt.Sprintf("status changed from %q to %q", cond.FromStatus, cond.ToStatus)
	}

	// If only to_status, check if different from previous
	if cond.ToStatus != "" && ctx.Asset.Status != ctx.Asset.PreviousStatus {
		return true, fmt.Sprintf("status changed to %q", cond.ToStatus)
	}

	return false, "no status change detected"
}

// evalFSMState evaluates an FSM state condition.
func (e *Engine) evalFSMState(cond Condition, ctx *EvaluationContext) (bool, string) {
	if ctx.Asset == nil {
		return false, "no asset context"
	}

	if ctx.Asset.FSMState == cond.FSMState {
		return true, fmt.Sprintf("asset in FSM state %q", cond.FSMState)
	}

	return false, fmt.Sprintf("asset FSM state %q != %q", ctx.Asset.FSMState, cond.FSMState)
}

// evalAbsent evaluates an absent (no event) condition.
func (e *Engine) evalAbsent(cond Condition, ctx *EvaluationContext) (bool, string) {
	if ctx.Asset == nil || ctx.Asset.LastEventTime == nil {
		return false, "no last event time"
	}

	elapsed := time.Since(*ctx.Asset.LastEventTime)
	if elapsed > cond.Duration {
		return true, fmt.Sprintf("no event for %v (threshold: %v)", elapsed.Round(time.Second), cond.Duration)
	}

	return false, fmt.Sprintf("last event %v ago (threshold: %v)", elapsed.Round(time.Second), cond.Duration)
}

// compareValues compares two values using the given operator.
func compareValues(a float64, op Operator, b float64) bool {
	switch op {
	case OpGT:
		return a > b
	case OpLT:
		return a < b
	case OpEQ:
		return a == b
	case OpGTE:
		return a >= b
	case OpLTE:
		return a <= b
	case OpNE:
		return a != b
	default:
		return false
	}
}

// getFieldValue retrieves a value from a nested map.
func getFieldValue(data map[string]any, field string) (any, bool) {
	if data == nil || field == "" {
		return nil, false
	}
	v, ok := data[field]
	return v, ok
}

// toFloat64 converts a value to float64.
func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	default:
		return 0, false
	}
}
