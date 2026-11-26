package fsm

import (
	"testing"
	"time"
)

func TestNewEvaluator(t *testing.T) {
	e := NewEvaluator()
	if e == nil {
		t.Fatal("NewEvaluator returned nil")
	}
	if e.machines == nil {
		t.Error("machines map not initialized")
	}
}

func TestLoadMachine(t *testing.T) {
	e := NewEvaluator()

	tests := []struct {
		name    string
		machine *Machine
		wantErr bool
	}{
		{
			name: "valid machine",
			machine: &Machine{
				ID:      1,
				Name:    "Test FSM",
				Initial: "normal",
				States: map[string]*State{
					"normal": {Name: "normal", IsInitial: true},
					"alert":  {Name: "alert"},
				},
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			machine: &Machine{
				Name:    "Test",
				Initial: "normal",
				States: map[string]*State{
					"normal": {Name: "normal"},
					"alert":  {Name: "alert"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing name",
			machine: &Machine{
				ID:      2,
				Initial: "normal",
				States: map[string]*State{
					"normal": {Name: "normal"},
					"alert":  {Name: "alert"},
				},
			},
			wantErr: true,
		},
		{
			name: "too few states",
			machine: &Machine{
				ID:      3,
				Name:    "Test",
				Initial: "only",
				States: map[string]*State{
					"only": {Name: "only"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing initial state",
			machine: &Machine{
				ID:   4,
				Name: "Test",
				States: map[string]*State{
					"normal": {Name: "normal"},
					"alert":  {Name: "alert"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid initial state reference",
			machine: &Machine{
				ID:      5,
				Name:    "Test",
				Initial: "nonexistent",
				States: map[string]*State{
					"normal": {Name: "normal"},
					"alert":  {Name: "alert"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid transition target",
			machine: &Machine{
				ID:      6,
				Name:    "Test",
				Initial: "normal",
				States: map[string]*State{
					"normal": {
						Name: "normal",
						Transitions: []*Transition{
							{To: "nonexistent", Condition: &Condition{Type: ConditionValueGT}},
						},
					},
					"alert": {Name: "alert"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := e.LoadMachine(tt.machine)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadMachine() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetMachine(t *testing.T) {
	e := NewEvaluator()
	machine := &Machine{
		ID:      1,
		Name:    "Test",
		Initial: "normal",
		States: map[string]*State{
			"normal": {Name: "normal"},
			"alert":  {Name: "alert"},
		},
	}
	e.LoadMachine(machine)

	m, ok := e.GetMachine(1)
	if !ok {
		t.Error("GetMachine failed for existing machine")
	}
	if m.Name != "Test" {
		t.Errorf("GetMachine returned wrong machine, got name %q", m.Name)
	}

	_, ok = e.GetMachine(999)
	if ok {
		t.Error("GetMachine should return false for non-existent machine")
	}
}

func TestUnloadMachine(t *testing.T) {
	e := NewEvaluator()
	machine := &Machine{
		ID:      1,
		Name:    "Test",
		Initial: "normal",
		States: map[string]*State{
			"normal": {Name: "normal"},
			"alert":  {Name: "alert"},
		},
	}
	e.LoadMachine(machine)

	e.UnloadMachine(1)

	_, ok := e.GetMachine(1)
	if ok {
		t.Error("Machine should be unloaded")
	}
}

func TestEvaluateValueGT(t *testing.T) {
	e := NewEvaluator()
	threshold := 8.0

	machine := &Machine{
		ID:      1,
		Name:    "Temperature Monitor",
		Initial: "normal",
		States: map[string]*State{
			"normal": {
				Name: "normal",
				Transitions: []*Transition{
					{
						To: "alert",
						Condition: &Condition{
							Type:      ConditionValueGT,
							Field:     "temperature",
							Threshold: &threshold,
						},
					},
				},
			},
			"alert": {Name: "alert"},
		},
	}
	e.LoadMachine(machine)

	tests := []struct {
		name        string
		temperature float64
		wantTrigger bool
		wantTo      string
	}{
		{"below threshold", 5.0, false, ""},
		{"at threshold", 8.0, false, ""},
		{"above threshold", 10.0, true, "alert"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &EvaluationContext{
				CurrentState: "normal",
				Event: &Event{
					AssetID:     1,
					Timestamp:   time.Now(),
					TriggerType: "sensor_driver",
					Value:       map[string]any{"temperature": tt.temperature},
				},
			}

			result, err := e.Evaluate(1, ctx)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}

			if result.Triggered != tt.wantTrigger {
				t.Errorf("Triggered = %v, want %v", result.Triggered, tt.wantTrigger)
			}

			if tt.wantTrigger && result.ToState != tt.wantTo {
				t.Errorf("ToState = %q, want %q", result.ToState, tt.wantTo)
			}
		})
	}
}

func TestEvaluateValueLT(t *testing.T) {
	e := NewEvaluator()
	threshold := 2.0

	machine := &Machine{
		ID:      1,
		Name:    "Temperature Monitor",
		Initial: "normal",
		States: map[string]*State{
			"normal": {
				Name: "normal",
				Transitions: []*Transition{
					{
						To: "cold",
						Condition: &Condition{
							Type:      ConditionValueLT,
							Field:     "temperature",
							Threshold: &threshold,
						},
					},
				},
			},
			"cold": {Name: "cold"},
		},
	}
	e.LoadMachine(machine)

	ctx := &EvaluationContext{
		CurrentState: "normal",
		Event: &Event{
			Value: map[string]any{"temperature": 1.0},
		},
	}

	result, _ := e.Evaluate(1, ctx)
	if !result.Triggered {
		t.Error("should trigger for value < threshold")
	}
	if result.ToState != "cold" {
		t.Errorf("ToState = %q, want cold", result.ToState)
	}
}

func TestEvaluateValueEQ(t *testing.T) {
	e := NewEvaluator()

	machine := &Machine{
		ID:      1,
		Name:    "Status Monitor",
		Initial: "normal",
		States: map[string]*State{
			"normal": {
				Name: "normal",
				Transitions: []*Transition{
					{
						To: "error",
						Condition: &Condition{
							Type:   ConditionValueEQ,
							Field:  "status",
							Target: "error",
						},
					},
				},
			},
			"error": {Name: "error"},
		},
	}
	e.LoadMachine(machine)

	ctx := &EvaluationContext{
		CurrentState: "normal",
		Event: &Event{
			Value: map[string]any{"status": "error"},
		},
	}

	result, _ := e.Evaluate(1, ctx)
	if !result.Triggered {
		t.Error("should trigger for value == target")
	}
}

func TestEvaluateValueIn(t *testing.T) {
	e := NewEvaluator()

	machine := &Machine{
		ID:      1,
		Name:    "Priority Monitor",
		Initial: "normal",
		States: map[string]*State{
			"normal": {
				Name: "normal",
				Transitions: []*Transition{
					{
						To: "urgent",
						Condition: &Condition{
							Type:   ConditionValueIn,
							Field:  "priority",
							Values: []string{"high", "critical"},
						},
					},
				},
			},
			"urgent": {Name: "urgent"},
		},
	}
	e.LoadMachine(machine)

	tests := []struct {
		priority    string
		wantTrigger bool
	}{
		{"low", false},
		{"medium", false},
		{"high", true},
		{"critical", true},
	}

	for _, tt := range tests {
		t.Run(tt.priority, func(t *testing.T) {
			ctx := &EvaluationContext{
				CurrentState: "normal",
				Event: &Event{
					Value: map[string]any{"priority": tt.priority},
				},
			}

			result, _ := e.Evaluate(1, ctx)
			if result.Triggered != tt.wantTrigger {
				t.Errorf("Triggered = %v, want %v", result.Triggered, tt.wantTrigger)
			}
		})
	}
}

func TestEvaluateTriggerType(t *testing.T) {
	e := NewEvaluator()

	machine := &Machine{
		ID:      1,
		Name:    "QR Scanner",
		Initial: "idle",
		States: map[string]*State{
			"idle": {
				Name: "idle",
				Transitions: []*Transition{
					{
						To: "scanned",
						Condition: &Condition{
							Type:    ConditionTriggerType,
							Trigger: "qr_scan",
						},
					},
				},
			},
			"scanned": {Name: "scanned"},
		},
	}
	e.LoadMachine(machine)

	ctx := &EvaluationContext{
		CurrentState: "idle",
		Event: &Event{
			TriggerType: "qr_scan",
			Value:       map[string]any{},
		},
	}

	result, _ := e.Evaluate(1, ctx)
	if !result.Triggered {
		t.Error("should trigger for matching trigger type")
	}
}

func TestEvaluateDurationExceeded(t *testing.T) {
	e := NewEvaluator()

	machine := &Machine{
		ID:      1,
		Name:    "Silence Monitor",
		Initial: "normal",
		States: map[string]*State{
			"normal": {
				Name: "normal",
				Transitions: []*Transition{
					{
						To: "silent",
						Condition: &Condition{
							Type:     ConditionDurationExceeded,
							Duration: 5 * time.Minute,
						},
					},
				},
			},
			"silent": {Name: "silent"},
		},
	}
	e.LoadMachine(machine)

	// Last event was 10 minutes ago
	lastEvent := time.Now().Add(-10 * time.Minute)

	ctx := &EvaluationContext{
		CurrentState:  "normal",
		LastEventTime: &lastEvent,
		Event: &Event{
			Timestamp: time.Now(),
			Value:     map[string]any{},
		},
	}

	result, _ := e.Evaluate(1, ctx)
	if !result.Triggered {
		t.Error("should trigger when duration exceeded")
	}

	// Last event was 2 minutes ago
	recentEvent := time.Now().Add(-2 * time.Minute)
	ctx.LastEventTime = &recentEvent

	result, _ = e.Evaluate(1, ctx)
	if result.Triggered {
		t.Error("should not trigger when duration not exceeded")
	}
}

func TestEvaluateCompoundAnd(t *testing.T) {
	e := NewEvaluator()
	tempThreshold := 8.0

	machine := &Machine{
		ID:      1,
		Name:    "Compound Monitor",
		Initial: "normal",
		States: map[string]*State{
			"normal": {
				Name: "normal",
				Transitions: []*Transition{
					{
						To: "critical",
						Condition: &Condition{
							Type: ConditionAnd,
							Conditions: []*Condition{
								{
									Type:      ConditionValueGT,
									Field:     "temperature",
									Threshold: &tempThreshold,
								},
								{
									Type:    ConditionTriggerType,
									Trigger: "sensor_driver",
								},
							},
						},
					},
				},
			},
			"critical": {Name: "critical"},
		},
	}
	e.LoadMachine(machine)

	tests := []struct {
		name        string
		temp        float64
		trigger     string
		wantTrigger bool
	}{
		{"both conditions met", 10.0, "sensor_driver", true},
		{"only temp met", 10.0, "api_call", false},
		{"only trigger met", 5.0, "sensor_driver", false},
		{"neither met", 5.0, "api_call", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &EvaluationContext{
				CurrentState: "normal",
				Event: &Event{
					TriggerType: tt.trigger,
					Value:       map[string]any{"temperature": tt.temp},
				},
			}

			result, _ := e.Evaluate(1, ctx)
			if result.Triggered != tt.wantTrigger {
				t.Errorf("Triggered = %v, want %v", result.Triggered, tt.wantTrigger)
			}
		})
	}
}

func TestEvaluateCompoundOr(t *testing.T) {
	e := NewEvaluator()
	highThreshold := 10.0
	lowThreshold := 2.0

	machine := &Machine{
		ID:      1,
		Name:    "Range Monitor",
		Initial: "normal",
		States: map[string]*State{
			"normal": {
				Name: "normal",
				Transitions: []*Transition{
					{
						To: "out_of_range",
						Condition: &Condition{
							Type: ConditionOr,
							Conditions: []*Condition{
								{
									Type:      ConditionValueGT,
									Field:     "temperature",
									Threshold: &highThreshold,
								},
								{
									Type:      ConditionValueLT,
									Field:     "temperature",
									Threshold: &lowThreshold,
								},
							},
						},
					},
				},
			},
			"out_of_range": {Name: "out_of_range"},
		},
	}
	e.LoadMachine(machine)

	tests := []struct {
		name        string
		temp        float64
		wantTrigger bool
	}{
		{"too high", 12.0, true},
		{"too low", 1.0, true},
		{"in range", 5.0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &EvaluationContext{
				CurrentState: "normal",
				Event: &Event{
					Value: map[string]any{"temperature": tt.temp},
				},
			}

			result, _ := e.Evaluate(1, ctx)
			if result.Triggered != tt.wantTrigger {
				t.Errorf("Triggered = %v, want %v", result.Triggered, tt.wantTrigger)
			}
		})
	}
}

func TestEvaluateMultipleTransitions(t *testing.T) {
	e := NewEvaluator()
	warningThreshold := 8.0
	alertThreshold := 12.0

	machine := &Machine{
		ID:      1,
		Name:    "Multi-level Monitor",
		Initial: "normal",
		States: map[string]*State{
			"normal": {
				Name: "normal",
				Transitions: []*Transition{
					// Higher priority - checked first
					{
						To:       "alert",
						Priority: 10,
						Condition: &Condition{
							Type:      ConditionValueGT,
							Field:     "temperature",
							Threshold: &alertThreshold,
						},
					},
					// Lower priority
					{
						To:       "warning",
						Priority: 5,
						Condition: &Condition{
							Type:      ConditionValueGT,
							Field:     "temperature",
							Threshold: &warningThreshold,
						},
					},
				},
			},
			"warning": {Name: "warning"},
			"alert":   {Name: "alert"},
		},
	}
	e.LoadMachine(machine)

	// Temperature of 15 should match alert (priority 10), not warning
	ctx := &EvaluationContext{
		CurrentState: "normal",
		Event: &Event{
			Value: map[string]any{"temperature": 15.0},
		},
	}

	result, _ := e.Evaluate(1, ctx)
	if !result.Triggered {
		t.Error("should trigger")
	}
	if result.ToState != "alert" {
		t.Errorf("ToState = %q, want alert (higher priority)", result.ToState)
	}
}

func TestGetFieldValue(t *testing.T) {
	data := map[string]any{
		"temperature": 25.5,
		"status":      "ok",
		"nested": map[string]any{
			"value": 100,
		},
	}

	tests := []struct {
		field string
		want  any
		ok    bool
	}{
		{"temperature", 25.5, true},
		{"status", "ok", true},
		{"nonexistent", nil, false},
		{"", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			got, ok := getFieldValue(data, tt.field)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
			if ok && got != tt.want {
				t.Errorf("value = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name string
		val  any
		want float64
		ok   bool
	}{
		{"float64", 3.14, 3.14, true},
		{"float32", float32(2.5), 2.5, true},
		{"int", 42, 42.0, true},
		{"int64", int64(100), 100.0, true},
		{"string", "not a number", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toFloat64(tt.val)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
			if ok && got != tt.want {
				t.Errorf("value = %v, want %v", got, tt.want)
			}
		})
	}
}
