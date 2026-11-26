// Package validation provides queryfy schemas for validating Seam AMS entities.
//
// This package defines validation schemas for all input data:
//   - Tenants
//   - Asset Types (with dynamic field templates)
//   - Assets
//   - Sensors and bindings
//   - Events
//   - FSM definitions
//   - Rules
//
// Schemas use queryfy's fluent builder API and support both strict
// and loose validation modes.
package validation

import (
	"fmt"
	"regexp"

	"github.com/ha1tch/queryfy"
	"github.com/ha1tch/queryfy/builders"
)

// =============================================================================
// TENANT SCHEMAS
// =============================================================================

// TenantCreateSchema validates tenant creation requests.
var TenantCreateSchema = builders.Object().
	Field("name", builders.String().Required().MinLength(2).MaxLength(100)).
	Field("slug", builders.String().Required().MinLength(2).MaxLength(50).
		Pattern(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)).
	Field("settings", builders.Object().
		Field("timezone", builders.String().Optional()).
		Field("language", builders.String().Optional().Enum("en", "es", "pt", "ja")).
		Field("date_format", builders.String().Optional()).
		Optional(),
	)

// TenantUpdateSchema validates tenant update requests.
var TenantUpdateSchema = builders.Object().
	Field("name", builders.String().Optional().MinLength(2).MaxLength(100)).
	Field("settings", builders.Object().
		Field("timezone", builders.String().Optional()).
		Field("language", builders.String().Optional().Enum("en", "es", "pt", "ja")).
		Field("date_format", builders.String().Optional()).
		Optional(),
	)

// =============================================================================
// ASSET TYPE SCHEMAS
// =============================================================================

// FieldType represents the type of a dynamic field.
type FieldType string

const (
	FieldTypeText     FieldType = "text"
	FieldTypeNumber   FieldType = "number"
	FieldTypeBoolean  FieldType = "boolean"
	FieldTypeDate     FieldType = "date"
	FieldTypeDatetime FieldType = "datetime"
	FieldTypeSelect   FieldType = "select"
	FieldTypeRef      FieldType = "ref" // Reference to another entity
)

// FieldDefinitionSchema validates a single field definition in an asset type template.
var FieldDefinitionSchema = builders.Object().
	Field("type", builders.String().Required().
		Enum("text", "number", "boolean", "date", "datetime", "select", "ref")).
	Field("label", builders.String().Required().MinLength(1).MaxLength(100)).
	Field("required", builders.Bool().Optional()).
	Field("default", builders.String().Optional().Nullable()).
	// Text-specific options
	Field("min_length", builders.Number().Optional().Min(0)).
	Field("max_length", builders.Number().Optional().Min(1)).
	Field("pattern", builders.String().Optional()).
	// Number-specific options
	Field("min", builders.Number().Optional()).
	Field("max", builders.Number().Optional()).
	Field("precision", builders.Number().Optional().Min(0).Max(10)).
	// Select-specific options
	Field("options", builders.Array().Of(builders.String()).Optional()).
	// Ref-specific options
	Field("ref_entity", builders.String().Optional()). // e.g., "sensors", "assets"
	Field("ref_display", builders.String().Optional()) // Field to display from referenced entity

// AssetTypeCreateSchema validates asset type creation requests.
var AssetTypeCreateSchema = builders.Object().
	Field("name", builders.String().Required().MinLength(2).MaxLength(100)).
	Field("slug", builders.String().Required().MinLength(2).MaxLength(50).
		Pattern(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)).
	Field("description", builders.String().Optional().MaxLength(500)).
	Field("icon", builders.String().Optional().MaxLength(50)).
	Field("color", builders.String().Optional().Pattern(`^#[0-9A-Fa-f]{6}$`)).
	// Identity fields template (defines what fields identify this asset)
	Field("identity_template", builders.Object().Required()).
	// State fields template (defines what state fields this asset tracks)
	Field("state_template", builders.Object().Optional()).
	// FSM reference (optional)
	Field("fsm_id", builders.Number().Optional()).
	// Connectivity options
	Field("connectivity_tiers", builders.Array().Of(
		builders.String().Enum("qr", "gateway", "lora", "gps"),
	).Optional()).
	Field("default_alerta_id", builders.Number().Optional())

// AssetTypeUpdateSchema validates asset type update requests.
var AssetTypeUpdateSchema = builders.Object().
	Field("name", builders.String().Optional().MinLength(2).MaxLength(100)).
	Field("description", builders.String().Optional().MaxLength(500)).
	Field("icon", builders.String().Optional().MaxLength(50)).
	Field("color", builders.String().Optional().Pattern(`^#[0-9A-Fa-f]{6}$`)).
	Field("identity_template", builders.Object().Optional()).
	Field("state_template", builders.Object().Optional()).
	Field("fsm_id", builders.Number().Optional().Nullable()).
	Field("connectivity_tiers", builders.Array().Of(
		builders.String().Enum("qr", "gateway", "lora", "gps"),
	).Optional())

// =============================================================================
// ASSET SCHEMAS
// =============================================================================

// AssetCreateSchema validates asset creation requests.
// Note: The identity fields are validated dynamically based on the asset type.
// AssetCreateSchema validates asset creation (spec blocks A–G).
// Field names are English (internal/external law, docs/KNOWN_ISSUES.md) --
// translated to xolu's Spanish-keyed storage via AssetValuesToXolu
// (internal/ui/asset_xolu_fields.go) at the point of persistence, same
// pattern the dynamic Asset form already uses. See T-24 in docs/RESOLVED.md.
var AssetCreateSchema = builders.Object().
	// Basic
	Field("name", builders.String().Required().MinLength(1).MaxLength(200)).
	Field("type_id", builders.Number().Required().Positive().Integer()).
	Field("status", builders.String().Optional().
		Enum("active", "inactive", "maintenance", "lost")).
	Field("connectivity_tier", builders.String().Optional().
		Enum("qr_only", "gateway", "lora_mesh", "gps_cellular")).
	// Block A — Identification
	Field("giai", builders.String().Optional().MaxLength(30)).
	Field("grai", builders.String().Optional().MaxLength(30)).
	Field("barcode_rfid", builders.String().Optional().MaxLength(100)).
	Field("technical_tag", builders.String().Optional().MaxLength(50)).
	// Block B — Characterisation
	Field("brand_id", builders.Number().Optional().Positive().Integer()).
	Field("model_id", builders.Number().Optional().Positive().Integer()).
	Field("serial_1", builders.String().Optional().MaxLength(100)).
	Field("serial_2", builders.String().Optional().MaxLength(100)).
	Field("lot", builders.String().Optional().MaxLength(100)).
	Field("criticality", builders.String().Required().Enum("High", "Medium", "Low")).
	// Block C — Location
	Field("location_id", builders.Number().Optional().Positive().Integer()).
	Field("region_id", builders.Number().Optional().Positive().Integer()).
	Field("parent_id", builders.Number().Optional().Positive().Integer()).
	// Block D — Custody
	Field("custodian_id", builders.Number().Optional().Positive().Integer()).
	Field("tenure_type", builders.String().Optional().
		Enum("Owned", "Loaned", "Borrowed", "Rented")).
	Field("loan_expiry", builders.String().Optional().MaxLength(10)).
	// Block E — Lifecycle
	Field("purchase_date", builders.String().Optional().MaxLength(10)).
	Field("expected_arrival_date", builders.String().Optional().MaxLength(10)).
	Field("arrival_date", builders.String().Optional().MaxLength(10)).
	Field("obsolescence_date", builders.String().Optional().MaxLength(10)).
	Field("expiration_date", builders.String().Optional().MaxLength(10)).
	Field("decommission_date", builders.String().Optional().MaxLength(10)).
	Field("decommission_reason", builders.String().Optional().
		Enum("Sale", "Scrap", "Theft", "Donation", "Other")).
	// Block F — Service/IoT
	Field("temp_min", builders.String().Optional()).
	Field("temp_max", builders.String().Optional()).
	Field("refrigerant", builders.String().Optional().MaxLength(100)).
	Field("voltage_consumption", builders.String().Optional().MaxLength(100)).
	// Block G — Finance
	Field("purchase_price", builders.String().Optional()).
	Field("currency_id", builders.Number().Optional().Positive().Integer()).
	Field("current_price", builders.String().Optional()).
	Field("cost_center_id", builders.Number().Optional().Positive().Integer()).
	Field("warranty_end", builders.String().Optional().MaxLength(10))

// AssetUpdateSchema validates asset updates — same fields, all optional except name.
// See AssetCreateSchema above re: English field names + AssetValuesToXolu.
var AssetUpdateSchema = builders.Object().
	Field("name", builders.String().Optional().MinLength(1).MaxLength(200)).
	Field("type_id", builders.Number().Optional().Positive().Integer()).
	Field("status", builders.String().Optional().
		Enum("active", "inactive", "maintenance", "lost")).
	Field("connectivity_tier", builders.String().Optional().
		Enum("qr_only", "gateway", "lora_mesh", "gps_cellular")).
	Field("giai", builders.String().Optional().MaxLength(30)).
	Field("grai", builders.String().Optional().MaxLength(30)).
	Field("barcode_rfid", builders.String().Optional().MaxLength(100)).
	Field("technical_tag", builders.String().Optional().MaxLength(50)).
	Field("brand_id", builders.Number().Optional().Positive().Integer()).
	Field("model_id", builders.Number().Optional().Positive().Integer()).
	Field("serial_1", builders.String().Optional().MaxLength(100)).
	Field("serial_2", builders.String().Optional().MaxLength(100)).
	Field("lot", builders.String().Optional().MaxLength(100)).
	Field("criticality", builders.String().Optional().Enum("High", "Medium", "Low")).
	Field("location_id", builders.Number().Optional().Positive().Integer()).
	Field("region_id", builders.Number().Optional().Positive().Integer()).
	Field("parent_id", builders.Number().Optional().Positive().Integer()).
	Field("custodian_id", builders.Number().Optional().Positive().Integer()).
	Field("tenure_type", builders.String().Optional().
		Enum("Owned", "Loaned", "Borrowed", "Rented")).
	Field("loan_expiry", builders.String().Optional().MaxLength(10)).
	Field("purchase_date", builders.String().Optional().MaxLength(10)).
	Field("expected_arrival_date", builders.String().Optional().MaxLength(10)).
	Field("arrival_date", builders.String().Optional().MaxLength(10)).
	Field("obsolescence_date", builders.String().Optional().MaxLength(10)).
	Field("expiration_date", builders.String().Optional().MaxLength(10)).
	Field("decommission_date", builders.String().Optional().MaxLength(10)).
	Field("decommission_reason", builders.String().Optional().
		Enum("Sale", "Scrap", "Theft", "Donation", "Other")).
	Field("purchase_price", builders.String().Optional()).
	Field("currency_id", builders.Number().Optional().Positive().Integer()).
	Field("current_price", builders.String().Optional()).
	Field("cost_center_id", builders.Number().Optional().Positive().Integer()).
	Field("warranty_end", builders.String().Optional().MaxLength(10))

// =============================================================================
// SENSOR SCHEMAS
// =============================================================================

// SensorCreateSchema validates sensor registration requests.
var SensorCreateSchema = builders.Object().
	Field("external_id", builders.String().Required().MinLength(1).MaxLength(200)).
	Field("type", builders.String().Required().MinLength(1).MaxLength(100)).
	Field("driver", builders.String().Required().MinLength(1).MaxLength(100)).
	Field("name", builders.String().Optional().MaxLength(200)).
	Field("metadata", builders.Object().Optional())

// SensorUpdateSchema validates sensor update requests.
var SensorUpdateSchema = builders.Object().
	Field("name", builders.String().Optional().MaxLength(200)).
	Field("metadata", builders.Object().Optional())

// SensorBindingSchema validates sensor-to-asset binding requests.
var SensorBindingSchema = builders.Object().
	Field("asset_id", builders.Number().Required()).
	Field("binding_metadata", builders.Object().Optional())

// =============================================================================
// EVENT SCHEMAS
// =============================================================================

// EventIngestSchema validates incoming event data.
var EventIngestSchema = builders.Object().
	Field("asset_id", builders.Number().Optional()).  // Either asset_id or sensor_id required
	Field("sensor_id", builders.String().Optional()). // External sensor ID
	Field("timestamp", builders.DateTime().ISO8601().Required()).
	Field("trigger_type", builders.String().Required().Enum(
		"sensor_driver",
		"custom_driver",
		"qr_scan",
		"api_call",
		"mobile_app",
		"lora_notification",
		"gateway_poll",
	)).
	Field("value", builders.Object().Required()). // Payload depends on trigger type
	Field("unit", builders.String().Optional().MaxLength(50)).
	Field("location", builders.Object().
		Field("lat", builders.Number().Min(-90).Max(90)).
		Field("lon", builders.Number().Min(-180).Max(180)).
		Optional(),
	).
	Custom(func(value any) error {
		// At least one of asset_id or sensor_id must be provided
		m, ok := value.(map[string]any)
		if !ok {
			return nil // Let type validation handle this
		}
		_, hasAsset := m["asset_id"]
		_, hasSensor := m["sensor_id"]
		if !hasAsset && !hasSensor {
			return fmt.Errorf("either asset_id or sensor_id is required")
		}
		return nil
	})

// EventBatchSchema validates batch event ingestion.
var EventBatchSchema = builders.Object().
	Field("events", builders.Array().Of(EventIngestSchema).Required().
		Custom(func(value any) error {
			arr, ok := value.([]any)
			if !ok {
				return nil
			}
			if len(arr) > 1000 {
				return fmt.Errorf("batch size exceeds maximum of 1000 events")
			}
			return nil
		}))

// =============================================================================
// FSM SCHEMAS
// =============================================================================

// FSMConditionSchema validates an FSM transition condition.
var FSMConditionSchema = builders.Object().
	Field("type", builders.String().Required().Enum(
		"value_gt",          // value > threshold
		"value_lt",          // value < threshold
		"value_eq",          // value == target
		"value_in",          // value in [list]
		"duration_exceeded", // time since last event > duration
		"trigger_type",      // event trigger type matches
		"absent",            // no event for duration
	)).
	Field("field", builders.String().Optional()).                       // Field path in event.value
	Field("threshold", builders.Number().Optional()).                   // For comparison conditions
	Field("target", builders.String().Optional()).                      // For equality conditions
	Field("values", builders.Array().Of(builders.String()).Optional()). // For value_in
	Field("duration", builders.String().Optional()).                    // Duration string, e.g., "5m", "1h"
	Field("trigger", builders.String().Optional())                      // For trigger_type condition

// FSMTransitionSchema validates an FSM transition.
var FSMTransitionSchema = builders.Object().
	Field("to", builders.String().Required().MinLength(1).MaxLength(50)).
	Field("condition", FSMConditionSchema.Required()).
	Field("priority", builders.Number().Optional().Min(0).Max(100))

// FSMStateSchema validates an FSM state.
var FSMStateSchema = builders.Object().
	Field("name", builders.String().Required().MinLength(1).MaxLength(50)).
	Field("label", builders.String().Optional().MaxLength(100)).
	Field("color", builders.String().Optional().Pattern(`^#[0-9A-Fa-f]{6}$`)).
	Field("is_initial", builders.Bool().Optional()).
	Field("is_final", builders.Bool().Optional()).
	Field("transitions", builders.Array().Of(FSMTransitionSchema).Optional())

// FSMDefinitionSchema validates a complete FSM definition.
var FSMDefinitionSchema = builders.Object().
	Field("name", builders.String().Required().MinLength(2).MaxLength(100)).
	Field("description", builders.String().Optional().MaxLength(500)).
	Field("states", builders.Array().Of(FSMStateSchema).Required().
		Custom(func(value any) error {
			arr, ok := value.([]any)
			if !ok {
				return nil
			}
			if len(arr) < 2 {
				return fmt.Errorf("FSM must have at least 2 states")
			}
			// Check for exactly one initial state
			initialCount := 0
			for _, s := range arr {
				state, ok := s.(map[string]any)
				if !ok {
					continue
				}
				if isInitial, ok := state["is_initial"].(bool); ok && isInitial {
					initialCount++
				}
			}
			if initialCount != 1 {
				return fmt.Errorf("FSM must have exactly one initial state, found %d", initialCount)
			}
			return nil
							})).
	Field("layout", builders.Object().Optional()) // For visual designer positioning

// =============================================================================
// RULE SCHEMAS
// =============================================================================

// RuleConditionSchema validates a rule condition.
var RuleConditionSchema = builders.Object().
	Field("type", builders.String().Required().Enum(
		"threshold",          // Simple threshold comparison
		"threshold_duration", // Threshold exceeded for duration
		"change_rate",        // Rate of change exceeds limit
		"geofence_exit",      // Asset exits geofence
		"geofence_enter",     // Asset enters geofence
		"status_change",      // Asset status changes
		"fsm_state",          // Asset enters FSM state
		"absent",             // No event for duration
	)).
	Field("field", builders.String().Optional()). // Field path for threshold checks
	Field("operator", builders.String().Optional().Enum("gt", "lt", "eq", "gte", "lte", "ne")).
	Field("value", builders.Number().Optional()).    // Threshold value
	Field("duration", builders.String().Optional()). // Duration string
	Field("geofence_id", builders.Number().Optional()).
	Field("from_status", builders.String().Optional()).
	Field("to_status", builders.String().Optional()).
	Field("fsm_state", builders.String().Optional())

// RuleActionSchema validates a rule action.
var RuleActionSchema = builders.Object().
	Field("type", builders.String().Required().Enum(
		"notify",       // Send notification
		"webhook",      // Call webhook
		"update_asset", // Update asset status/state
		"create_event", // Create a derived event
	)).
	// For notify action
	Field("channels", builders.Array().Of(
		builders.String().Enum("email", "sms", "push", "webhook"),
	).Optional()).
	Field("template", builders.String().Optional().MaxLength(50)).
	Field("recipients", builders.Array().Of(builders.String()).Optional()).
	// For webhook action
	Field("webhook_url", builders.String().Optional().URL()).
	Field("webhook_method", builders.String().Optional().Enum("GET", "POST", "PUT")).
	Field("webhook_headers", builders.Object().Optional()).
	// For update_asset action
	Field("set_status", builders.String().Optional().
		Enum("active", "silent", "alert", "lost")).
	Field("set_state", builders.Object().Optional())

// RuleSchema validates a complete rule definition.
var RuleSchema = builders.Object().
	Field("name", builders.String().Required().MinLength(2).MaxLength(100)).
	Field("description", builders.String().Optional().MaxLength(500)).
	Field("enabled", builders.Bool().Optional()).
	Field("priority", builders.Number().Optional().Min(0).Max(100)).
	// Scope filters
	Field("asset_type_ids", builders.Array().Of(builders.Number()).Optional()).
	Field("asset_tags", builders.Array().Of(builders.String()).Optional()).
	// Conditions (AND logic)
	Field("conditions", builders.Array().Of(RuleConditionSchema).Required().
		Custom(func(value any) error {
			arr, ok := value.([]any)
			if !ok {
				return nil
			}
			if len(arr) == 0 {
				return fmt.Errorf("rule must have at least one condition")
			}
			return nil
		})).
	// Actions to execute
	Field("actions", builders.Array().Of(RuleActionSchema).Required().
		Custom(func(value any) error {
			arr, ok := value.([]any)
			if !ok {
				return nil
			}
			if len(arr) == 0 {
				return fmt.Errorf("rule must have at least one action")
			}
			return nil
		})).
	// Rate limiting
	Field("cooldown", builders.String().Optional()) // e.g., "5m" - don't fire again within this period

// =============================================================================
// DYNAMIC SCHEMA GENERATION
// =============================================================================

// FieldDefinition represents a field definition from an asset type template.
type FieldDefinition struct {
	Type      FieldType `json:"type"`
	Label     string    `json:"label"`
	Required  bool      `json:"required"`
	Default   *string   `json:"default,omitempty"`
	MinLength *int      `json:"min_length,omitempty"`
	MaxLength *int      `json:"max_length,omitempty"`
	Pattern   *string   `json:"pattern,omitempty"`
	Min       *float64  `json:"min,omitempty"`
	Max       *float64  `json:"max,omitempty"`
	Precision *int      `json:"precision,omitempty"`
	Options   []string  `json:"options,omitempty"`
	RefEntity *string   `json:"ref_entity,omitempty"`
}

// DynamicFieldSchema generates a queryfy schema from an asset type template.
// This allows validation of asset identity and state fields based on the
// dynamic template defined in the asset type.
func DynamicFieldSchema(template map[string]FieldDefinition) queryfy.Schema {
	schema := builders.Object()

	for name, field := range template {
		var fieldSchema queryfy.Schema

		switch field.Type {
		case FieldTypeText:
			s := builders.String()
			if field.MinLength != nil {
				s = s.MinLength(*field.MinLength)
			}
			if field.MaxLength != nil {
				s = s.MaxLength(*field.MaxLength)
			}
			if field.Pattern != nil {
				s = s.Pattern(*field.Pattern)
			}
			if field.Required {
				s = s.Required()
			}
			fieldSchema = s

		case FieldTypeNumber:
			n := builders.Number()
			if field.Min != nil {
				n = n.Min(*field.Min)
			}
			if field.Max != nil {
				n = n.Max(*field.Max)
			}
			if field.Required {
				n = n.Required()
			}
			fieldSchema = n

		case FieldTypeBoolean:
			b := builders.Bool()
			if field.Required {
				b = b.Required()
			}
			fieldSchema = b

		case FieldTypeDate:
			d := builders.DateTime().DateOnly()
			if field.Required {
				d = d.Required()
			}
			fieldSchema = d

		case FieldTypeDatetime:
			d := builders.DateTime().ISO8601()
			if field.Required {
				d = d.Required()
			}
			fieldSchema = d

		case FieldTypeSelect:
			s := builders.String()
			if len(field.Options) > 0 {
				s = s.Enum(field.Options...)
			}
			if field.Required {
				s = s.Required()
			}
			fieldSchema = s

		case FieldTypeRef:
			// REF fields store an ID reference
			n := builders.Number()
			if field.Required {
				n = n.Required()
			}
			fieldSchema = n

		default:
			// Unknown type - allow any value
			fieldSchema = builders.String().Optional()
		}

		schema = schema.Field(name, fieldSchema)
	}

	return schema
}

// ParseFieldDefinitions parses a template map into FieldDefinition structs.
func ParseFieldDefinitions(template map[string]any) (map[string]FieldDefinition, error) {
	result := make(map[string]FieldDefinition)

	for name, v := range template {
		fieldMap, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("field %q: expected object, got %T", name, v)
		}

		def := FieldDefinition{}

		// Type (required)
		if t, ok := fieldMap["type"].(string); ok {
			def.Type = FieldType(t)
		} else {
			return nil, fmt.Errorf("field %q: missing or invalid type", name)
		}

		// Label (required)
		if l, ok := fieldMap["label"].(string); ok {
			def.Label = l
		} else {
			return nil, fmt.Errorf("field %q: missing or invalid label", name)
		}

		// Required (optional)
		if r, ok := fieldMap["required"].(bool); ok {
			def.Required = r
		}

		// Default (optional)
		if d, ok := fieldMap["default"].(string); ok {
			def.Default = &d
		}

		// MinLength (optional)
		if v, ok := fieldMap["min_length"].(float64); ok {
			i := int(v)
			def.MinLength = &i
		}

		// MaxLength (optional)
		if v, ok := fieldMap["max_length"].(float64); ok {
			i := int(v)
			def.MaxLength = &i
		}

		// Pattern (optional)
		if p, ok := fieldMap["pattern"].(string); ok {
			// Validate pattern is valid regex
			if _, err := regexp.Compile(p); err != nil {
				return nil, fmt.Errorf("field %q: invalid pattern: %w", name, err)
			}
			def.Pattern = &p
		}

		// Min (optional)
		if v, ok := fieldMap["min"].(float64); ok {
			def.Min = &v
		}

		// Max (optional)
		if v, ok := fieldMap["max"].(float64); ok {
			def.Max = &v
		}

		// Precision (optional)
		if v, ok := fieldMap["precision"].(float64); ok {
			i := int(v)
			def.Precision = &i
		}

		// Options (optional)
		if opts, ok := fieldMap["options"].([]any); ok {
			for _, o := range opts {
				if s, ok := o.(string); ok {
					def.Options = append(def.Options, s)
				}
			}
		}

		// RefEntity (optional)
		if r, ok := fieldMap["ref_entity"].(string); ok {
			def.RefEntity = &r
		}

		result[name] = def
	}

	return result, nil
}

// =============================================================================
// SCHEMA ALIASES FOR API HANDLERS
// =============================================================================

// FSMCreateSchema is an alias for FSMDefinitionSchema.
var FSMCreateSchema = FSMDefinitionSchema

// FSMUpdateSchema is an alias for FSMDefinitionSchema.
var FSMUpdateSchema = FSMDefinitionSchema

// RuleCreateSchema is an alias for RuleSchema.
var RuleCreateSchema = RuleSchema

// RuleUpdateSchema is an alias for RuleSchema.
var RuleUpdateSchema = RuleSchema

// IdentityTemplateSchema validates identity field templates.
var IdentityTemplateSchema = builders.Object().
	Field("fields", builders.Array().
		Of(FieldDefinitionSchema).
		Optional(),
	)

// =============================================================================
// MASTER TABLE SCHEMAS (Wave 0 — P-1 through P-8, P-17)
// =============================================================================

// RegionCreateSchema validates región creation. English field names
// (T-27, docs/RESOLVED.md) -- translated to xolu's Spanish-keyed storage
// via RegionValuesToXolu (internal/ui/masters_xolu_fields.go).
var RegionCreateSchema = builders.Object().
	Field("description", builders.String().Required().MinLength(1).MaxLength(200))

// RegionUpdateSchema validates región updates. Not currently wired to any
// live caller -- only RegionCreateSchema is, for both create and update.
var RegionUpdateSchema = builders.Object().
	Field("description", builders.String().Optional().MinLength(1).MaxLength(200))

// MonedaCreateSchema validates moneda creation. English field names
// (T-27) -- translated to xolu's Spanish-keyed storage via
// MonedaValuesToXolu (internal/ui/masters_xolu_fields.go).
var MonedaCreateSchema = builders.Object().
	Field("iso_code", builders.String().Required().MinLength(3).MaxLength(3)).
	Field("symbol", builders.String().Required().MinLength(1).MaxLength(5))

// MonedaUpdateSchema validates moneda updates. Not currently wired to any
// live caller.
var MonedaUpdateSchema = builders.Object().
	Field("iso_code", builders.String().Optional().MinLength(3).MaxLength(3)).
	Field("symbol", builders.String().Optional().MinLength(1).MaxLength(5))

// CentroCostoCreateSchema validates centro de costo creation. English
// field names (T-27) -- translated to xolu's Spanish-keyed storage via
// CentroCostoValuesToXolu (internal/ui/masters_xolu_fields.go).
var CentroCostoCreateSchema = builders.Object().
	Field("accounting_code", builders.String().Required().MinLength(1).MaxLength(50)).
	Field("name", builders.String().Required().MinLength(1).MaxLength(100))

// CentroCostoUpdateSchema validates centro de costo updates. Not
// currently wired to any live caller.
var CentroCostoUpdateSchema = builders.Object().
	Field("accounting_code", builders.String().Optional().MinLength(1).MaxLength(50)).
	Field("name", builders.String().Optional().MinLength(1).MaxLength(100))

// MarcaCreateSchema validates marca creation. English field names
// (T-27) -- translated to xolu's Spanish-keyed storage via
// MarcaValuesToXolu (internal/ui/masters_xolu_fields.go).
var MarcaCreateSchema = builders.Object().
	Field("code", builders.String().Required().MinLength(1).MaxLength(50)).
	Field("name", builders.String().Required().MinLength(1).MaxLength(100))

// MarcaUpdateSchema validates marca updates. Not currently wired to any
// live caller.
var MarcaUpdateSchema = builders.Object().
	Field("code", builders.String().Optional().MinLength(1).MaxLength(50)).
	Field("name", builders.String().Optional().MinLength(1).MaxLength(100))

// CustodioCreateSchema validates custodio creation. English field names
// (T-26, docs/RESOLVED.md) -- translated to xolu's Spanish-keyed storage
// via CustodioValuesToXolu (internal/ui/masters_xolu_fields.go) at the
// point of persistence. subject_type's own values (DNI/CI/RUT/RUC) are
// intentionally NOT translated -- regional legal document-type codes,
// not translatable prose.
var CustodioCreateSchema = builders.Object().
	Field("subject_type", builders.String().Required().Enum("DNI", "CI", "RUT", "RUC")).
	Field("legal_name", builders.String().Required().MinLength(1).MaxLength(200)).
	Field("tax_id", builders.String().Required().MinLength(1).MaxLength(50)).
	Field("alias", builders.String().Optional().MaxLength(100)).
	Field("email", builders.String().Required().MinLength(3).MaxLength(200)).
	Field("phone", builders.String().Optional().MaxLength(30)).
	Field("address", builders.String().Optional().MaxLength(300)).
	Field("active", builders.Bool().Required())

// CustodioUpdateSchema validates custodio updates. Not currently wired to
// any live caller (only CustodioCreateSchema is, for both create and
// update -- see validate.go's schemaCustodioCreate), kept in sync anyway.
var CustodioUpdateSchema = builders.Object().
	Field("subject_type", builders.String().Optional().Enum("DNI", "CI", "RUT", "RUC")).
	Field("legal_name", builders.String().Optional().MinLength(1).MaxLength(200)).
	Field("tax_id", builders.String().Optional().MinLength(1).MaxLength(50)).
	Field("alias", builders.String().Optional().MaxLength(100)).
	Field("email", builders.String().Optional().MinLength(3).MaxLength(200)).
	Field("phone", builders.String().Optional().MaxLength(30)).
	Field("address", builders.String().Optional().MaxLength(300)).
	Field("active", builders.Bool().Optional())

// AlertaCreateSchema validates alerta creation. English field names/
// values (T-26) -- translated to xolu's Spanish-keyed/valued storage via
// AlertaValuesToXolu (internal/ui/masters_xolu_fields.go).
var AlertaCreateSchema = builders.Object().
	Field("type", builders.String().Required().Enum("Maintenance", "Security", "Operational")).
	Field("description", builders.String().Required().MinLength(1).MaxLength(500)).
	Field("criticality", builders.String().Required().Enum("High", "Medium", "Low"))

// AlertaUpdateSchema validates alerta updates. Not currently wired to any
// live caller -- see CustodioUpdateSchema's own comment above.
var AlertaUpdateSchema = builders.Object().
	Field("type", builders.String().Optional().Enum("Maintenance", "Security", "Operational")).
	Field("description", builders.String().Optional().MinLength(1).MaxLength(500)).
	Field("criticality", builders.String().Optional().Enum("High", "Medium", "Low"))

// PropDefinitionCreateSchema validates property definition creation.
// English field names/values (T-26) -- translated to xolu's Spanish-keyed/
// valued storage via PropDefValuesToXolu (internal/ui/masters_xolu_fields.go).
var PropDefinitionCreateSchema = builders.Object().
	Field("short_name", builders.String().Required().MinLength(1).MaxLength(50)).
	Field("data_type", builders.String().Required().Enum(
		"numeric", "alphanumeric", "date", "datetime", "time",
		"boolean", "single_select", "multi_select",
		"system_entity", "external_api",
	)).
	Field("validation", builders.Object().Optional()).
	Field("default_value", builders.String().Optional().MaxLength(200))

// PropDefinitionUpdateSchema validates property definition updates. Not
// currently wired to any live caller -- see CustodioUpdateSchema's own
// comment above.
var PropDefinitionUpdateSchema = builders.Object().
	Field("short_name", builders.String().Optional().MinLength(1).MaxLength(50)).
	Field("data_type", builders.String().Optional().Enum(
		"numeric", "alphanumeric", "date", "datetime", "time",
		"boolean", "single_select", "multi_select",
		"system_entity", "external_api",
	)).
	Field("validation", builders.Object().Optional()).
	Field("default_value", builders.String().Optional().MaxLength(200))

// ModeloCreateSchema validates modelo creation. English field names
// (T-27) -- translated to xolu's Spanish-keyed storage via
// ModeloValuesToXolu (internal/ui/masters_xolu_fields.go). brand_id
// matches the same name asset_xolu_fields.go already uses for the
// equivalent Asset field.
var ModeloCreateSchema = builders.Object().
	Field("brand_id", builders.Number().Required().Positive().Integer()).
	Field("name", builders.String().Required().MinLength(1).MaxLength(100))

// ModeloUpdateSchema validates modelo updates.
var ModeloUpdateSchema = ModeloCreateSchema

// UbicacionCreateSchema validates ubicación creation. English field
// names (T-27) -- translated to xolu's Spanish-keyed storage via
// UbicacionValuesToXolu (internal/ui/masters_xolu_fields.go).
var UbicacionCreateSchema = builders.Object().
	Field("short_name", builders.String().Required().MinLength(1).MaxLength(50)).
	Field("description", builders.String().Optional().MaxLength(200)).
	Field("region_id", builders.Number().Required().Positive().Integer())

// UbicacionUpdateSchema validates ubicación updates.
var UbicacionUpdateSchema = UbicacionCreateSchema

// PropAssocCreateSchema validates prop_association creation. English
// field names/values (T-26) -- translated to xolu's Spanish-keyed/valued
// storage via PropAssocValuesToXolu (internal/ui/masters_xolu_fields.go).
// target_entity's own values (asset_type/asset) were already English.
var PropAssocCreateSchema = builders.Object().
	Field("prop_definition_id", builders.Number().Required().Positive().Integer()).
	Field("target_entity", builders.String().Required().
		Enum("asset_type", "asset")).
	Field("entity_id", builders.Number().Required().Positive().Integer()).
	Field("requirement", builders.String().Required().
		Enum("required", "optional"))

// PropAssocUpdateSchema validates prop_association updates.
var PropAssocUpdateSchema = PropAssocCreateSchema

// UserUpdateSchema validates admin user update requests.
// Password changes are handled separately (dedicated endpoint).
var UserUpdateSchema = builders.Object().
	Field("name", builders.String().Required().MinLength(1).MaxLength(200)).
	Field("role", builders.String().Required().
		Enum("admin", "manager", "operator", "viewer", "developer"))

// RuleUISchema validates the flat rule form submitted by the UI.
// The API schema (RuleSchema) expects structured conditions/actions arrays;
// the UI form uses a simplified flat representation with individual fields.
var RuleUISchema = builders.Object().
	Field("name", builders.String().Required().MinLength(2).MaxLength(100)).
	Field("description", builders.String().Optional().MaxLength(500)).
	Field("enabled", builders.Bool().Optional()).
	Field("priority", builders.Number().Optional().Min(0).Max(100)).
	Field("trigger_type", builders.String().Optional().
		Enum("event", "threshold", "schedule", "geofence")).
	Field("asset_type_id", builders.String().Optional()).
	Field("condition_type", builders.String().Optional().
		Enum("value_gt", "value_lt", "value_eq", "duration_exceeded", "change_rate")).
	Field("threshold", builders.Number().Optional()).
	Field("duration", builders.Number().Optional().Min(0)).
	Field("action_type", builders.String().Optional().
		Enum("notify", "webhook", "update_asset", "create_alert")).
	Field("action_config", builders.String().Optional())
