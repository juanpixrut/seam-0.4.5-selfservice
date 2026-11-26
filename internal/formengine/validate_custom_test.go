package formengine

import (
	"errors"
	"testing"
)

// TestValidateCustomFields_NoOpWhenNoFieldUsesIt confirms the
// overwhelming-majority case (no field in the form sets
// x-seam-custom-validator) returns nil, not an empty-but-non-nil map --
// callers that do `if errs != nil` (matching validateAssetValues'
// own convention in internal/ui/asset_dynamic_post.go) must see the
// clean, no-error signal.
func TestValidateCustomFields_NoOpWhenNoFieldUsesIt(t *testing.T) {
	e := &FormEngine{}
	form := &Form{Fields: []Field{
		{Name: "name", JSONType: "string"},
		{Name: "quantity", JSONType: "number"},
	}}
	got := ValidateCustomFields(e, form, map[string]interface{}{"name": "x", "quantity": 5})
	if got != nil {
		t.Errorf("expected nil, got %#v", got)
	}
}

// TestValidateCustomFields_RegisteredValidatorPasses confirms a
// registered validator that returns nil produces no error entry.
func TestValidateCustomFields_RegisteredValidatorPasses(t *testing.T) {
	e := &FormEngine{
		CustomValidators: map[string]func(interface{}) error{
			"always_ok": func(value interface{}) error { return nil },
		},
	}
	form := &Form{Fields: []Field{
		{Name: "serial_1", JSONType: "string", XSeamCustomValidator: "always_ok"},
	}}
	got := ValidateCustomFields(e, form, map[string]interface{}{"serial_1": "ABC123"})
	if got != nil {
		t.Errorf("expected nil (validator passed), got %#v", got)
	}
}

// TestValidateCustomFields_RegisteredValidatorFails confirms a
// registered validator's returned error becomes the field's message,
// and that the value actually passed to the validator is the field's
// submitted value, not something else.
func TestValidateCustomFields_RegisteredValidatorFails(t *testing.T) {
	var gotValue interface{}
	e := &FormEngine{
		CustomValidators: map[string]func(interface{}) error{
			"reject_dupes": func(value interface{}) error {
				gotValue = value
				return errors.New("serial already in use")
			},
		},
	}
	form := &Form{Fields: []Field{
		{Name: "serial_1", JSONType: "string", XSeamCustomValidator: "reject_dupes"},
	}}
	got := ValidateCustomFields(e, form, map[string]interface{}{"serial_1": "DUPLICATE-001"})
	if got["serial_1"] != "serial already in use" {
		t.Errorf("errs[serial_1] = %q, want %q", got["serial_1"], "serial already in use")
	}
	if gotValue != "DUPLICATE-001" {
		t.Errorf("validator received %#v, want the field's actual submitted value", gotValue)
	}
}

// TestValidateCustomFields_UnregisteredNameFailsClosed confirms a
// field naming a validator that isn't in the registry is itself a
// validation error, not a silent pass -- the documented, deliberate
// fail-closed behaviour (engine.go's own CustomValidators comment),
// so a schema-authoring typo surfaces in testing, not in production.
func TestValidateCustomFields_UnregisteredNameFailsClosed(t *testing.T) {
	e := &FormEngine{} // no CustomValidators registered at all
	form := &Form{Fields: []Field{
		{Name: "serial_1", JSONType: "string", XSeamCustomValidator: "typo_name"},
	}}
	got := ValidateCustomFields(e, form, map[string]interface{}{"serial_1": "ABC123"})
	if got == nil {
		t.Fatal("expected a fail-closed error for an unregistered validator name, got nil")
	}
	if got["serial_1"] == "" {
		t.Errorf("expected an error message for serial_1, got none: %#v", got)
	}
}

// TestValidateCustomFields_NilFormReturnsNil is a defensive-nil check
// -- ValidateCustomFields must not panic on a nil *Form (a caller
// might reach this after a ParseSchema failure it didn't guard).
func TestValidateCustomFields_NilFormReturnsNil(t *testing.T) {
	e := &FormEngine{}
	if got := ValidateCustomFields(e, nil, nil); got != nil {
		t.Errorf("expected nil for a nil form, got %#v", got)
	}
}
