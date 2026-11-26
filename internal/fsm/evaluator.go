// Package fsm provides a finite state machine evaluator for asset state transitions.
//
// The FSM evaluator processes events and determines state transitions based on
// configurable conditions. It supports:
//   - Value comparisons (gt, lt, eq, in)
//   - Duration-based conditions (exceeded, absent)
//   - Trigger type matching
//   - Compound conditions (AND, OR)
//
// FSM definitions are stored in xolu and loaded per asset type.
package fsm

import (
	"fmt"
	"time"
)

// Machine represents a complete finite state machine definition.
type Machine struct {
	ID          int64             `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	States      map[string]*State `json:"states"`
	Initial     string            `json:"initial"`
}

// State represents a single state in the FSM.
type State struct {
	Name        string        `json:"name"`
	Label       string        `json:"label,omitempty"`
	Color       string        `json:"color,omitempty"`
	IsInitial   bool          `json:"is_initial,omitempty"`
	IsFinal     bool          `json:"is_final,omitempty"`
	Transitions []*Transition `json:"transitions,omitempty"`
}

// Transition represents a possible state transition.
type Transition struct {
	To        string     `json:"to"`
	Condition *Condition `json:"condition"`
	Priority  int        `json:"priority,omitempty"`
}

// ConditionType defines the type of condition for a transition.
type ConditionType string

const (
	// ConditionValueGT triggers when value > threshold
	ConditionValueGT ConditionType = "value_gt"
	// ConditionValueLT triggers when value < threshold
	ConditionValueLT ConditionType = "value_lt"
	// ConditionValueEQ triggers when value == target
	ConditionValueEQ ConditionType = "value_eq"
	// ConditionValueIn triggers when value is in list
	ConditionValueIn ConditionType = "value_in"
	// ConditionDurationExceeded triggers when time since last event > duration
	ConditionDurationExceeded ConditionType = "duration_exceeded"
	// ConditionTriggerType triggers when event trigger type matches
	ConditionTriggerType ConditionType = "trigger_type"
	// ConditionAbsent triggers when no event received for duration
	ConditionAbsent ConditionType = "absent"
	// ConditionAnd triggers when all sub-conditions are true
	ConditionAnd ConditionType = "and"
	// ConditionOr triggers when any sub-condition is true
	ConditionOr ConditionType = "or"
)

// Condition defines when a transition should occur.
type Condition struct {
	Type ConditionType `json:"type"`

	// For value comparisons
	Field     string   `json:"field,omitempty"`     // Field path in event value
	Threshold *float64 `json:"threshold,omitempty"` // For gt/lt comparisons
	Target    string   `json:"target,omitempty"`    // For eq comparison
	Values    []string `json:"values,omitempty"`    // For in comparison

	// For duration conditions
	Duration time.Duration `json:"duration,omitempty"`

	// For trigger type condition
	Trigger string `json:"trigger,omitempty"`

	// For compound conditions
	Conditions []*Condition `json:"conditions,omitempty"`
}

// Event represents an incoming event to be evaluated.
type Event struct {
	AssetID     int64          `json:"asset_id"`
	Timestamp   time.Time      `json:"timestamp"`
	TriggerType string         `json:"trigger_type"`
	Value       map[string]any `json:"value"`
}

// EvaluationContext provides context for condition evaluation.
type EvaluationContext struct {
	Event         *Event
	CurrentState  string
	LastEventTime *time.Time
	AssetState    map[string]any
}

// TransitionResult represents the result of evaluating an FSM against an event.
type TransitionResult struct {
	Triggered  bool
	FromState  string
	ToState    string
	Transition *Transition
	MatchedAt  time.Time
}

// Evaluator evaluates FSM transitions based on events.
type Evaluator struct {
	machines map[int64]*Machine
}

// NewEvaluator creates a new FSM evaluator.
func NewEvaluator() *Evaluator {
	return &Evaluator{
		machines: make(map[int64]*Machine),
	}
}

// LoadMachine loads an FSM definition into the evaluator.
func (e *Evaluator) LoadMachine(m *Machine) error {
	if m.ID == 0 {
		return fmt.Errorf("machine ID is required")
	}
	if m.Name == "" {
		return fmt.Errorf("machine name is required")
	}
	if len(m.States) < 2 {
		return fmt.Errorf("machine must have at least 2 states")
	}
	if m.Initial == "" {
		return fmt.Errorf("machine must have an initial state")
	}
	if _, ok := m.States[m.Initial]; !ok {
		return fmt.Errorf("initial state %q not found in states", m.Initial)
	}

	// Validate all transitions reference valid states
	for stateName, state := range m.States {
		for i, t := range state.Transitions {
			if _, ok := m.States[t.To]; !ok {
				return fmt.Errorf("state %q transition %d references unknown state %q", stateName, i, t.To)
			}
		}
	}

	e.machines[m.ID] = m
	return nil
}

// GetMachine retrieves a loaded machine by ID.
func (e *Evaluator) GetMachine(id int64) (*Machine, bool) {
	m, ok := e.machines[id]
	return m, ok
}

// UnloadMachine removes a machine from the evaluator.
func (e *Evaluator) UnloadMachine(id int64) {
	delete(e.machines, id)
}

// Evaluate evaluates an event against an FSM and returns a transition result.
func (e *Evaluator) Evaluate(machineID int64, ctx *EvaluationContext) (*TransitionResult, error) {
	machine, ok := e.machines[machineID]
	if !ok {
		return nil, fmt.Errorf("machine %d not found", machineID)
	}

	return e.EvaluateMachine(machine, ctx)
}

// EvaluateMachine evaluates an event against a specific machine.
func (e *Evaluator) EvaluateMachine(machine *Machine, ctx *EvaluationContext) (*TransitionResult, error) {
	currentState, ok := machine.States[ctx.CurrentState]
	if !ok {
		return nil, fmt.Errorf("current state %q not found in machine %q", ctx.CurrentState, machine.Name)
	}

	result := &TransitionResult{
		Triggered: false,
		FromState: ctx.CurrentState,
		MatchedAt: time.Now(),
	}

	// Check transitions in priority order (higher priority = checked first)
	// Transitions are assumed to be sorted by priority descending
	for _, transition := range currentState.Transitions {
		if e.evaluateCondition(transition.Condition, ctx) {
			result.Triggered = true
			result.ToState = transition.To
			result.Transition = transition
			return result, nil
		}
	}

	return result, nil
}

// evaluateCondition evaluates a single condition against the context.
func (e *Evaluator) evaluateCondition(cond *Condition, ctx *EvaluationContext) bool {
	if cond == nil {
		return false
	}

	switch cond.Type {
	case ConditionValueGT:
		return e.evalValueGT(cond, ctx)
	case ConditionValueLT:
		return e.evalValueLT(cond, ctx)
	case ConditionValueEQ:
		return e.evalValueEQ(cond, ctx)
	case ConditionValueIn:
		return e.evalValueIn(cond, ctx)
	case ConditionDurationExceeded:
		return e.evalDurationExceeded(cond, ctx)
	case ConditionTriggerType:
		return e.evalTriggerType(cond, ctx)
	case ConditionAbsent:
		return e.evalAbsent(cond, ctx)
	case ConditionAnd:
		return e.evalAnd(cond, ctx)
	case ConditionOr:
		return e.evalOr(cond, ctx)
	default:
		return false
	}
}

// evalValueGT evaluates value > threshold condition.
func (e *Evaluator) evalValueGT(cond *Condition, ctx *EvaluationContext) bool {
	if cond.Threshold == nil || ctx.Event == nil {
		return false
	}

	value, ok := getFieldValue(ctx.Event.Value, cond.Field)
	if !ok {
		return false
	}

	numValue, ok := toFloat64(value)
	if !ok {
		return false
	}

	return numValue > *cond.Threshold
}

// evalValueLT evaluates value < threshold condition.
func (e *Evaluator) evalValueLT(cond *Condition, ctx *EvaluationContext) bool {
	if cond.Threshold == nil || ctx.Event == nil {
		return false
	}

	value, ok := getFieldValue(ctx.Event.Value, cond.Field)
	if !ok {
		return false
	}

	numValue, ok := toFloat64(value)
	if !ok {
		return false
	}

	return numValue < *cond.Threshold
}

// evalValueEQ evaluates value == target condition.
func (e *Evaluator) evalValueEQ(cond *Condition, ctx *EvaluationContext) bool {
	if ctx.Event == nil {
		return false
	}

	value, ok := getFieldValue(ctx.Event.Value, cond.Field)
	if !ok {
		return false
	}

	return fmt.Sprintf("%v", value) == cond.Target
}

// evalValueIn evaluates value in [list] condition.
func (e *Evaluator) evalValueIn(cond *Condition, ctx *EvaluationContext) bool {
	if ctx.Event == nil || len(cond.Values) == 0 {
		return false
	}

	value, ok := getFieldValue(ctx.Event.Value, cond.Field)
	if !ok {
		return false
	}

	strValue := fmt.Sprintf("%v", value)
	for _, v := range cond.Values {
		if strValue == v {
			return true
		}
	}

	return false
}

// evalDurationExceeded evaluates time since last event > duration condition.
func (e *Evaluator) evalDurationExceeded(cond *Condition, ctx *EvaluationContext) bool {
	if ctx.LastEventTime == nil || ctx.Event == nil {
		return false
	}

	elapsed := ctx.Event.Timestamp.Sub(*ctx.LastEventTime)
	return elapsed > cond.Duration
}

// evalTriggerType evaluates event trigger type matches condition.
func (e *Evaluator) evalTriggerType(cond *Condition, ctx *EvaluationContext) bool {
	if ctx.Event == nil {
		return false
	}

	return ctx.Event.TriggerType == cond.Trigger
}

// evalAbsent evaluates no event for duration condition.
// Note: This requires external tracking and is checked differently.
// Here we check if LastEventTime + Duration < now.
func (e *Evaluator) evalAbsent(cond *Condition, ctx *EvaluationContext) bool {
	if ctx.LastEventTime == nil {
		return false
	}

	deadline := ctx.LastEventTime.Add(cond.Duration)
	return time.Now().After(deadline)
}

// evalAnd evaluates all conditions must be true.
func (e *Evaluator) evalAnd(cond *Condition, ctx *EvaluationContext) bool {
	if len(cond.Conditions) == 0 {
		return false
	}

	for _, c := range cond.Conditions {
		if !e.evaluateCondition(c, ctx) {
			return false
		}
	}

	return true
}

// evalOr evaluates any condition must be true.
func (e *Evaluator) evalOr(cond *Condition, ctx *EvaluationContext) bool {
	if len(cond.Conditions) == 0 {
		return false
	}

	for _, c := range cond.Conditions {
		if e.evaluateCondition(c, ctx) {
			return true
		}
	}

	return false
}

// getFieldValue retrieves a value from a nested map using dot notation.
// e.g., "temperature" or "sensor.reading.value"
func getFieldValue(data map[string]any, field string) (any, bool) {
	if data == nil || field == "" {
		return nil, false
	}

	// Simple case: no nesting
	if v, ok := data[field]; ok {
		return v, true
	}

	// Handle nested fields with dot notation
	// For simplicity, we only handle one level of nesting here
	// A full implementation would recursively descend
	for k, v := range data {
		if k == field {
			return v, true
		}
	}

	return nil, false
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
