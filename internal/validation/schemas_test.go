package validation

import (
	"testing"

	"github.com/ha1tch/queryfy"
)

// Note: Schemas with dynamic nested objects (identity, value, identity_template)
// must be validated with Loose mode to allow arbitrary fields inside those objects.

func TestTenantCreateSchema(t *testing.T) {
	tests := []struct {
		name    string
		data    map[string]any
		wantErr bool
	}{
		{
			name: "valid tenant",
			data: map[string]any{
				"name": "Acme Corp",
				"slug": "acme-corp",
				"settings": map[string]any{
					"timezone": "America/Montevideo",
					"language": "es",
				},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			data: map[string]any{
				"slug": "acme-corp",
			},
			wantErr: true,
		},
		{
			name: "invalid slug format",
			data: map[string]any{
				"name": "Acme Corp",
				"slug": "Acme Corp",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := queryfy.Validate(tt.data, TenantCreateSchema)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAssetTypeCreateSchema(t *testing.T) {
	tests := []struct {
		name    string
		data    map[string]any
		wantErr bool
	}{
		{
			name: "valid refrigerator type",
			data: map[string]any{
				"name":  "Commercial Refrigerator",
				"slug":  "commercial-fridge",
				"color": "#3B82F6",
				"identity_template": map[string]any{
					"serial_number": map[string]any{
						"type":  "text",
						"label": "Serial Number",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing identity_template",
			data: map[string]any{
				"name": "Fridge",
				"slug": "fridge",
			},
			wantErr: true,
		},
		{
			name: "invalid color format",
			data: map[string]any{
				"name":              "Fridge",
				"slug":              "fridge",
				"color":             "blue",
				"identity_template": map[string]any{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := queryfy.ValidateWithMode(tt.data, AssetTypeCreateSchema, queryfy.Loose)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAssetCreateSchema(t *testing.T) {
	tests := []struct {
		name    string
		data    map[string]any
		wantErr bool
	}{
		{
			// name, type_id, criticality are required in the updated spec.
			// English field names/enum values per the internal/external
			// language law (T-24, docs/RESOLVED.md) -- these were
			// "criticidad"/"Alta" etc. before that fix.
			name: "valid asset",
			data: map[string]any{
				"name":        "Compresor Norte",
				"type_id":     float64(1),
				"criticality": "High",
			},
			wantErr: false,
		},
		{
			name: "missing type_id",
			data: map[string]any{
				"name":        "Activo sin tipo",
				"criticality": "Medium",
			},
			wantErr: true,
		},
		{
			name: "missing name",
			data: map[string]any{
				"type_id":     float64(1),
				"criticality": "Low",
			},
			wantErr: true,
		},
		{
			name: "invalid criticality",
			data: map[string]any{
				"name":        "Activo",
				"type_id":     float64(1),
				"criticality": "Extrema",
			},
			wantErr: true,
		},
		{
			name: "invalid connectivity tier",
			data: map[string]any{
				"name":              "Activo",
				"type_id":           float64(1),
				"criticality":       "Medium",
				"connectivity_tier": "satellite",
			},
			wantErr: true,
		},
		{
			name: "invalid decommission_reason",
			data: map[string]any{
				"name":                "Activo",
				"type_id":             float64(1),
				"criticality":         "Medium",
				"decommission_reason": "Extraviado", // not in enum
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := queryfy.ValidateWithMode(tt.data, AssetCreateSchema, queryfy.Loose)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEventIngestSchema(t *testing.T) {
	tests := []struct {
		name    string
		data    map[string]any
		wantErr bool
	}{
		{
			name: "valid event with asset_id",
			data: map[string]any{
				"asset_id":     1,
				"timestamp":    "2025-01-07T12:00:00Z",
				"trigger_type": "sensor_driver",
				"value":        map[string]any{"temperature": 4.5},
			},
			wantErr: false,
		},
		{
			name: "valid event with sensor_id",
			data: map[string]any{
				"sensor_id":    "TEMP-001",
				"timestamp":    "2025-01-07T12:00:00Z",
				"trigger_type": "sensor_driver",
				"value":        map[string]any{"temperature": 4.5},
			},
			wantErr: false,
		},
		{
			name: "missing both asset_id and sensor_id",
			data: map[string]any{
				"timestamp":    "2025-01-07T12:00:00Z",
				"trigger_type": "sensor_driver",
				"value":        map[string]any{},
			},
			wantErr: true,
		},
		{
			name: "missing timestamp",
			data: map[string]any{
				"asset_id":     1,
				"trigger_type": "sensor_driver",
				"value":        map[string]any{},
			},
			wantErr: true,
		},
		{
			name: "invalid trigger type",
			data: map[string]any{
				"asset_id":     1,
				"timestamp":    "2025-01-07T12:00:00Z",
				"trigger_type": "unknown",
				"value":        map[string]any{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := queryfy.ValidateWithMode(tt.data, EventIngestSchema, queryfy.Loose)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFSMDefinitionSchema(t *testing.T) {
	tests := []struct {
		name    string
		data    map[string]any
		wantErr bool
	}{
		{
			name: "valid FSM",
			data: map[string]any{
				"name": "Test FSM",
				"states": []any{
					map[string]any{"name": "normal", "is_initial": true},
					map[string]any{"name": "alert"},
				},
			},
			wantErr: false,
		},
		{
			name: "only one state",
			data: map[string]any{
				"name":   "Invalid",
				"states": []any{map[string]any{"name": "only", "is_initial": true}},
			},
			wantErr: true,
		},
		{
			name: "no initial state",
			data: map[string]any{
				"name": "Invalid",
				"states": []any{
					map[string]any{"name": "a"},
					map[string]any{"name": "b"},
				},
			},
			wantErr: true,
		},
		{
			name: "multiple initial states",
			data: map[string]any{
				"name": "Invalid",
				"states": []any{
					map[string]any{"name": "a", "is_initial": true},
					map[string]any{"name": "b", "is_initial": true},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := queryfy.ValidateWithMode(tt.data, FSMDefinitionSchema, queryfy.Loose)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRuleSchema(t *testing.T) {
	tests := []struct {
		name    string
		data    map[string]any
		wantErr bool
	}{
		{
			name: "valid rule",
			data: map[string]any{
				"name":       "Temperature Alert",
				"conditions": []any{map[string]any{"type": "threshold"}},
				"actions":    []any{map[string]any{"type": "notify"}},
			},
			wantErr: false,
		},
		{
			name: "empty conditions",
			data: map[string]any{
				"name":       "Invalid",
				"conditions": []any{},
				"actions":    []any{map[string]any{"type": "notify"}},
			},
			wantErr: true,
		},
		{
			name: "empty actions",
			data: map[string]any{
				"name":       "Invalid",
				"conditions": []any{map[string]any{"type": "threshold"}},
				"actions":    []any{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := queryfy.ValidateWithMode(tt.data, RuleSchema, queryfy.Loose)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDynamicFieldSchema(t *testing.T) {
	template := map[string]FieldDefinition{
		"serial_number": {
			Type:      FieldTypeText,
			Label:     "Serial Number",
			Required:  true,
			MinLength: intPtr(5),
			MaxLength: intPtr(50),
		},
		"capacity": {
			Type:  FieldTypeNumber,
			Label: "Capacity",
			Min:   floatPtr(0),
			Max:   floatPtr(10000),
		},
		"category": {
			Type:     FieldTypeSelect,
			Label:    "Category",
			Required: true,
			Options:  []string{"small", "medium", "large"},
		},
	}

	schema := DynamicFieldSchema(template)

	tests := []struct {
		name    string
		data    map[string]any
		wantErr bool
	}{
		{
			name: "valid data",
			data: map[string]any{
				"serial_number": "RF-12345",
				"capacity":      500.0,
				"category":      "medium",
			},
			wantErr: false,
		},
		{
			name: "missing required serial_number",
			data: map[string]any{
				"category": "medium",
			},
			wantErr: true,
		},
		{
			name: "serial_number too short",
			data: map[string]any{
				"serial_number": "RF",
				"category":      "medium",
			},
			wantErr: true,
		},
		{
			name: "capacity exceeds max",
			data: map[string]any{
				"serial_number": "RF-12345",
				"capacity":      15000.0,
				"category":      "medium",
			},
			wantErr: true,
		},
		{
			name: "invalid category",
			data: map[string]any{
				"serial_number": "RF-12345",
				"category":      "extra-large",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := queryfy.Validate(tt.data, schema)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseFieldDefinitions(t *testing.T) {
	template := map[string]any{
		"serial_number": map[string]any{
			"type":       "text",
			"label":      "Serial Number",
			"required":   true,
			"min_length": 5.0,
		},
		"temperature": map[string]any{
			"type":  "number",
			"label": "Temperature",
			"min":   -40.0,
			"max":   100.0,
		},
	}

	defs, err := ParseFieldDefinitions(template)
	if err != nil {
		t.Fatalf("ParseFieldDefinitions() error = %v", err)
	}

	if len(defs) != 2 {
		t.Errorf("expected 2 definitions, got %d", len(defs))
	}

	sn := defs["serial_number"]
	if sn.Type != FieldTypeText {
		t.Errorf("serial_number type = %v, want text", sn.Type)
	}
	if !sn.Required {
		t.Error("serial_number should be required")
	}
}

func TestParseFieldDefinitionsErrors(t *testing.T) {
	tests := []struct {
		name     string
		template map[string]any
	}{
		{
			name:     "missing type",
			template: map[string]any{"field": map[string]any{"label": "Field"}},
		},
		{
			name:     "missing label",
			template: map[string]any{"field": map[string]any{"type": "text"}},
		},
		{
			name:     "invalid pattern",
			template: map[string]any{"field": map[string]any{"type": "text", "label": "F", "pattern": "[invalid"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseFieldDefinitions(tt.template)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func intPtr(i int) *int           { return &i }
func floatPtr(f float64) *float64 { return &f }
