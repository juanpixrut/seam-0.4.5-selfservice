package rules

import (
	"fmt"
	"testing"
	"time"
)

func TestNewEngine(t *testing.T) {
	e := NewEngine()
	if e == nil {
		t.Fatal("NewEngine returned nil")
	}
	if e.RuleCount() != 0 {
		t.Error("new engine should have 0 rules")
	}
}

func TestLoadRule(t *testing.T) {
	e := NewEngine()

	tests := []struct {
		name    string
		rule    *Rule
		wantErr bool
	}{
		{
			name: "valid rule",
			rule: &Rule{
				ID:      1,
				Name:    "Test Rule",
				Enabled: true,
				Conditions: []Condition{
					{Type: ConditionThreshold, Field: "temperature", Operator: OpGT, Value: 8.0},
				},
				Actions: []Action{
					{Type: ActionNotify, Channels: []string{"email"}},
				},
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			rule: &Rule{
				Name:       "No ID",
				Conditions: []Condition{{Type: ConditionThreshold}},
				Actions:    []Action{{Type: ActionNotify}},
			},
			wantErr: true,
		},
		{
			name: "missing name",
			rule: &Rule{
				ID:         2,
				Conditions: []Condition{{Type: ConditionThreshold}},
				Actions:    []Action{{Type: ActionNotify}},
			},
			wantErr: true,
		},
		{
			name: "no conditions",
			rule: &Rule{
				ID:      3,
				Name:    "No Conditions",
				Actions: []Action{{Type: ActionNotify}},
			},
			wantErr: true,
		},
		{
			name: "no actions",
			rule: &Rule{
				ID:         4,
				Name:       "No Actions",
				Conditions: []Condition{{Type: ConditionThreshold}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := e.LoadRule(tt.rule)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadRule() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetRule(t *testing.T) {
	e := NewEngine()
	rule := &Rule{
		ID:         1,
		Name:       "Test",
		Conditions: []Condition{{Type: ConditionThreshold}},
		Actions:    []Action{{Type: ActionNotify}},
	}
	e.LoadRule(rule)

	r, ok := e.GetRule(1)
	if !ok {
		t.Error("GetRule failed for existing rule")
	}
	if r.Name != "Test" {
		t.Errorf("GetRule returned wrong rule")
	}

	_, ok = e.GetRule(999)
	if ok {
		t.Error("GetRule should return false for non-existent rule")
	}
}

func TestUnloadRule(t *testing.T) {
	e := NewEngine()
	rule := &Rule{
		ID:         1,
		Name:       "Test",
		Conditions: []Condition{{Type: ConditionThreshold}},
		Actions:    []Action{{Type: ActionNotify}},
	}
	e.LoadRule(rule)
	e.UnloadRule(1)

	if e.RuleCount() != 0 {
		t.Error("rule should be unloaded")
	}
}

func TestEvaluateThreshold(t *testing.T) {
	e := NewEngine()
	rule := &Rule{
		ID:      1,
		Name:    "High Temperature",
		Enabled: true,
		Conditions: []Condition{
			{Type: ConditionThreshold, Field: "temperature", Operator: OpGT, Value: 8.0},
		},
		Actions: []Action{
			{Type: ActionNotify, Channels: []string{"email"}},
		},
	}
	e.LoadRule(rule)

	tests := []struct {
		name        string
		temperature float64
		wantTrigger bool
	}{
		{"below threshold", 5.0, false},
		{"at threshold", 8.0, false},
		{"above threshold", 10.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &EvaluationContext{
				Event: &Event{
					AssetID:   1,
					Timestamp: time.Now(),
					Value:     map[string]any{"temperature": tt.temperature},
				},
				Asset: &Asset{
					ID:     1,
					TypeID: 1,
					Status: "active",
				},
			}

			results := e.Evaluate(ctx)
			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(results))
			}

			if results[0].Triggered != tt.wantTrigger {
				t.Errorf("Triggered = %v, want %v", results[0].Triggered, tt.wantTrigger)
			}
		})
	}
}

func TestEvaluateMultipleOperators(t *testing.T) {
	e := NewEngine()

	tests := []struct {
		name     string
		operator Operator
		value    float64
		event    float64
		want     bool
	}{
		{"gt true", OpGT, 5.0, 10.0, true},
		{"gt false", OpGT, 5.0, 3.0, false},
		{"lt true", OpLT, 10.0, 5.0, true},
		{"lt false", OpLT, 5.0, 10.0, false},
		{"eq true", OpEQ, 5.0, 5.0, true},
		{"eq false", OpEQ, 5.0, 6.0, false},
		{"gte true equal", OpGTE, 5.0, 5.0, true},
		{"gte true greater", OpGTE, 5.0, 6.0, true},
		{"gte false", OpGTE, 5.0, 4.0, false},
		{"lte true equal", OpLTE, 5.0, 5.0, true},
		{"lte true less", OpLTE, 5.0, 4.0, true},
		{"lte false", OpLTE, 5.0, 6.0, false},
		{"ne true", OpNE, 5.0, 6.0, true},
		{"ne false", OpNE, 5.0, 5.0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &Rule{
				ID:      1,
				Name:    "Test",
				Enabled: true,
				Conditions: []Condition{
					{Type: ConditionThreshold, Field: "value", Operator: tt.operator, Value: tt.value},
				},
				Actions: []Action{{Type: ActionNotify}},
			}
			e.rules = map[int64]*Rule{1: rule}

			ctx := &EvaluationContext{
				Event: &Event{Value: map[string]any{"value": tt.event}},
				Asset: &Asset{ID: 1, TypeID: 1},
			}

			results := e.Evaluate(ctx)
			if results[0].Triggered != tt.want {
				t.Errorf("Triggered = %v, want %v", results[0].Triggered, tt.want)
			}
		})
	}
}

func TestEvaluateStatusChange(t *testing.T) {
	e := NewEngine()
	rule := &Rule{
		ID:      1,
		Name:    "Alert Status",
		Enabled: true,
		Conditions: []Condition{
			{Type: ConditionStatusChange, ToStatus: "alert"},
		},
		Actions: []Action{{Type: ActionNotify}},
	}
	e.LoadRule(rule)

	tests := []struct {
		name           string
		currentStatus  string
		previousStatus string
		wantTrigger    bool
	}{
		{"changed to alert", "alert", "active", true},
		{"already alert", "alert", "alert", false},
		{"changed to different", "silent", "active", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &EvaluationContext{
				Event: &Event{Value: map[string]any{}},
				Asset: &Asset{
					ID:             1,
					TypeID:         1,
					Status:         tt.currentStatus,
					PreviousStatus: tt.previousStatus,
				},
			}

			results := e.Evaluate(ctx)
			if results[0].Triggered != tt.wantTrigger {
				t.Errorf("Triggered = %v, want %v", results[0].Triggered, tt.wantTrigger)
			}
		})
	}
}

func TestEvaluateFSMState(t *testing.T) {
	e := NewEngine()
	rule := &Rule{
		ID:      1,
		Name:    "Critical State",
		Enabled: true,
		Conditions: []Condition{
			{Type: ConditionFSMState, FSMState: "critical"},
		},
		Actions: []Action{{Type: ActionNotify}},
	}
	e.LoadRule(rule)

	tests := []struct {
		name        string
		fsmState    string
		wantTrigger bool
	}{
		{"in critical state", "critical", true},
		{"in normal state", "normal", false},
		{"in warning state", "warning", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &EvaluationContext{
				Event: &Event{Value: map[string]any{}},
				Asset: &Asset{
					ID:       1,
					TypeID:   1,
					FSMState: tt.fsmState,
				},
			}

			results := e.Evaluate(ctx)
			if results[0].Triggered != tt.wantTrigger {
				t.Errorf("Triggered = %v, want %v", results[0].Triggered, tt.wantTrigger)
			}
		})
	}
}

func TestEvaluateAbsent(t *testing.T) {
	e := NewEngine()
	rule := &Rule{
		ID:      1,
		Name:    "No Signal",
		Enabled: true,
		Conditions: []Condition{
			{Type: ConditionAbsent, Duration: 5 * time.Minute},
		},
		Actions: []Action{{Type: ActionNotify}},
	}
	e.LoadRule(rule)

	tests := []struct {
		name        string
		lastEvent   time.Duration // ago
		wantTrigger bool
	}{
		{"event 10 minutes ago", 10 * time.Minute, true},
		{"event 2 minutes ago", 2 * time.Minute, false},
		{"event just now", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lastEventTime := time.Now().Add(-tt.lastEvent)
			ctx := &EvaluationContext{
				Event: &Event{Value: map[string]any{}},
				Asset: &Asset{
					ID:            1,
					TypeID:        1,
					LastEventTime: &lastEventTime,
				},
			}

			results := e.Evaluate(ctx)
			if results[0].Triggered != tt.wantTrigger {
				t.Errorf("Triggered = %v, want %v", results[0].Triggered, tt.wantTrigger)
			}
		})
	}
}

func TestEvaluateMultipleConditions(t *testing.T) {
	e := NewEngine()
	rule := &Rule{
		ID:      1,
		Name:    "Multi-condition",
		Enabled: true,
		Conditions: []Condition{
			{Type: ConditionThreshold, Field: "temperature", Operator: OpGT, Value: 8.0},
			{Type: ConditionFSMState, FSMState: "warning"},
		},
		Actions: []Action{{Type: ActionNotify}},
	}
	e.LoadRule(rule)

	tests := []struct {
		name        string
		temp        float64
		fsmState    string
		wantTrigger bool
	}{
		{"both met", 10.0, "warning", true},
		{"only temp met", 10.0, "normal", false},
		{"only state met", 5.0, "warning", false},
		{"neither met", 5.0, "normal", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &EvaluationContext{
				Event: &Event{Value: map[string]any{"temperature": tt.temp}},
				Asset: &Asset{ID: 1, TypeID: 1, FSMState: tt.fsmState},
			}

			results := e.Evaluate(ctx)
			if results[0].Triggered != tt.wantTrigger {
				t.Errorf("Triggered = %v, want %v", results[0].Triggered, tt.wantTrigger)
			}
		})
	}
}

func TestRuleAppliesAssetTypeFilter(t *testing.T) {
	e := NewEngine()
	rule := &Rule{
		ID:           1,
		Name:         "Fridge Only",
		Enabled:      true,
		AssetTypeIDs: []int64{1, 2}, // Only type 1 and 2
		Conditions:   []Condition{{Type: ConditionThreshold, Field: "temp", Operator: OpGT, Value: 0}},
		Actions:      []Action{{Type: ActionNotify}},
	}
	e.LoadRule(rule)

	tests := []struct {
		name     string
		typeID   int64
		wantEval bool
	}{
		{"matching type 1", 1, true},
		{"matching type 2", 2, true},
		{"non-matching type", 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &EvaluationContext{
				Event: &Event{Value: map[string]any{"temp": 10.0}},
				Asset: &Asset{ID: 1, TypeID: tt.typeID},
			}

			results := e.Evaluate(ctx)
			if tt.wantEval {
				if len(results) == 0 {
					t.Error("expected rule to be evaluated")
				}
			} else {
				if len(results) > 0 {
					t.Error("expected rule to be skipped")
				}
			}
		})
	}
}

func TestRuleAppliesTagFilter(t *testing.T) {
	e := NewEngine()
	rule := &Rule{
		ID:         1,
		Name:       "Critical Assets",
		Enabled:    true,
		AssetTags:  []string{"critical", "monitored"},
		Conditions: []Condition{{Type: ConditionThreshold, Field: "temp", Operator: OpGT, Value: 0}},
		Actions:    []Action{{Type: ActionNotify}},
	}
	e.LoadRule(rule)

	tests := []struct {
		name     string
		tags     []string
		wantEval bool
	}{
		{"has critical tag", []string{"critical"}, true},
		{"has monitored tag", []string{"monitored"}, true},
		{"has both tags", []string{"critical", "monitored"}, true},
		{"no matching tags", []string{"normal"}, false},
		{"no tags", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &EvaluationContext{
				Event: &Event{Value: map[string]any{"temp": 10.0}},
				Asset: &Asset{ID: 1, TypeID: 1, Tags: tt.tags},
			}

			results := e.Evaluate(ctx)
			if tt.wantEval {
				if len(results) == 0 {
					t.Error("expected rule to be evaluated")
				}
			} else {
				if len(results) > 0 {
					t.Error("expected rule to be skipped")
				}
			}
		})
	}
}

func TestCooldown(t *testing.T) {
	e := NewEngine()
	rule := &Rule{
		ID:         1,
		Name:       "With Cooldown",
		Enabled:    true,
		Cooldown:   5 * time.Minute,
		Conditions: []Condition{{Type: ConditionThreshold, Field: "temp", Operator: OpGT, Value: 8.0}},
		Actions:    []Action{{Type: ActionNotify}},
	}
	e.LoadRule(rule)

	ctx := &EvaluationContext{
		Event: &Event{Value: map[string]any{"temp": 10.0}},
		Asset: &Asset{ID: 1, TypeID: 1},
	}

	// First evaluation should trigger
	results := e.Evaluate(ctx)
	if !results[0].Triggered {
		t.Error("first evaluation should trigger")
	}

	// Second evaluation should be on cooldown
	results = e.Evaluate(ctx)
	if len(results) > 0 {
		t.Error("second evaluation should be skipped due to cooldown")
	}
}

func TestDisabledRule(t *testing.T) {
	e := NewEngine()
	rule := &Rule{
		ID:         1,
		Name:       "Disabled",
		Enabled:    false,
		Conditions: []Condition{{Type: ConditionThreshold, Field: "temp", Operator: OpGT, Value: 0}},
		Actions:    []Action{{Type: ActionNotify}},
	}
	e.LoadRule(rule)

	ctx := &EvaluationContext{
		Event: &Event{Value: map[string]any{"temp": 10.0}},
		Asset: &Asset{ID: 1, TypeID: 1},
	}

	results := e.Evaluate(ctx)
	if len(results) > 0 {
		t.Error("disabled rule should not be evaluated")
	}
}

func TestCompareValues(t *testing.T) {
	tests := []struct {
		a    float64
		op   Operator
		b    float64
		want bool
	}{
		{10, OpGT, 5, true},
		{5, OpGT, 10, false},
		{5, OpLT, 10, true},
		{10, OpLT, 5, false},
		{5, OpEQ, 5, true},
		{5, OpEQ, 6, false},
		{5, OpGTE, 5, true},
		{6, OpGTE, 5, true},
		{4, OpGTE, 5, false},
		{5, OpLTE, 5, true},
		{4, OpLTE, 5, true},
		{6, OpLTE, 5, false},
		{5, OpNE, 6, true},
		{5, OpNE, 5, false},
	}

	for _, tt := range tests {
		name := fmt.Sprintf("%.0f %s %.0f", tt.a, tt.op, tt.b)
		t.Run(name, func(t *testing.T) {
			got := compareValues(tt.a, tt.op, tt.b)
			if got != tt.want {
				t.Errorf("compareValues(%.0f, %s, %.0f) = %v, want %v", tt.a, tt.op, tt.b, got, tt.want)
			}
		})
	}
}
