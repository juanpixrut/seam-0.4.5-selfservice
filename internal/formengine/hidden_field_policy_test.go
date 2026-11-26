package formengine

import (
	"reflect"
	"sort"
	"testing"
)

func hiddenPolicyFields() []Field {
	cond := &VisibleWhen{Field: "calidad_tenencia", Op: "eq", Value: "Comodato"}
	return []Field{
		{Name: "nombre"}, // no condition -- never in scope for this check
		{Name: "vto_comodato", XSeamVisibleWhen: cond},                            // default policy: discard
		{Name: "links_comodato", XSeamVisibleWhen: cond, XSeamHiddenReject: true}, // opted into reject
	}
}

func TestCheckHiddenFieldSubmission_ConditionHolds_NothingFlagged(t *testing.T) {
	submitted := map[string]interface{}{
		"calidad_tenencia": "Comodato",
		"nombre":           "x",
		"vto_comodato":     "2026-01-01",
		"links_comodato":   "http://example.com",
	}
	rejected, discarded := CheckHiddenFieldSubmission(hiddenPolicyFields(), submitted)
	if len(rejected) != 0 || len(discarded) != 0 {
		t.Errorf("condition holds -- nothing should be flagged, got rejected=%v discarded=%v", rejected, discarded)
	}
}

func TestCheckHiddenFieldSubmission_ConditionFalse_DefaultDiscards(t *testing.T) {
	submitted := map[string]interface{}{
		"calidad_tenencia": "Venta", // condition false
		"vto_comodato":     "2026-01-01",
	}
	rejected, discarded := CheckHiddenFieldSubmission(hiddenPolicyFields(), submitted)
	if len(rejected) != 0 {
		t.Errorf("vto_comodato has no x-seam-hidden-reject -- should discard, not reject: %v", rejected)
	}
	if len(discarded) != 1 || discarded[0] != "vto_comodato" {
		t.Errorf("expected vto_comodato discarded, got %v", discarded)
	}
}

func TestCheckHiddenFieldSubmission_ConditionFalse_HiddenRejectOptIn(t *testing.T) {
	submitted := map[string]interface{}{
		"calidad_tenencia": "Venta", // condition false
		"links_comodato":   "http://example.com",
	}
	rejected, discarded := CheckHiddenFieldSubmission(hiddenPolicyFields(), submitted)
	if len(discarded) != 0 {
		t.Errorf("links_comodato has x-seam-hidden-reject -- should reject, not discard: %v", discarded)
	}
	if len(rejected) != 1 || rejected[0] != "links_comodato" {
		t.Errorf("expected links_comodato rejected, got %v", rejected)
	}
}

func TestCheckHiddenFieldSubmission_ConditionFalse_NothingSubmitted_NotFlagged(t *testing.T) {
	// Condition false, but the client correctly didn't send a value at
	// all (or sent an empty one) -- nothing to discard or reject.
	submitted := map[string]interface{}{
		"calidad_tenencia": "Venta",
	}
	rejected, discarded := CheckHiddenFieldSubmission(hiddenPolicyFields(), submitted)
	if len(rejected) != 0 || len(discarded) != 0 {
		t.Errorf("nothing submitted for the hidden fields -- nothing should be flagged, got rejected=%v discarded=%v", rejected, discarded)
	}

	submittedEmpty := map[string]interface{}{
		"calidad_tenencia": "Venta",
		"vto_comodato":     "",
		"links_comodato":   nil,
	}
	rejected2, discarded2 := CheckHiddenFieldSubmission(hiddenPolicyFields(), submittedEmpty)
	if len(rejected2) != 0 || len(discarded2) != 0 {
		t.Errorf("empty values submitted for hidden fields -- nothing should be flagged, got rejected=%v discarded=%v", rejected2, discarded2)
	}
}

func TestCheckHiddenFieldSubmission_BothFlaggedTogether(t *testing.T) {
	submitted := map[string]interface{}{
		"calidad_tenencia": "Venta",
		"vto_comodato":     "2026-01-01",
		"links_comodato":   "http://example.com",
	}
	rejected, discarded := CheckHiddenFieldSubmission(hiddenPolicyFields(), submitted)
	if !reflect.DeepEqual(rejected, []string{"links_comodato"}) {
		t.Errorf("rejected = %v", rejected)
	}
	if !reflect.DeepEqual(discarded, []string{"vto_comodato"}) {
		t.Errorf("discarded = %v", discarded)
	}
}

func TestStripDiscardedFields(t *testing.T) {
	submitted := map[string]interface{}{
		"a": 1, "b": 2, "c": 3,
	}
	cleaned := StripDiscardedFields(submitted, []string{"b"})
	if _, ok := cleaned["b"]; ok {
		t.Error("b should have been stripped")
	}
	var keys []string
	for k := range cleaned {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if !reflect.DeepEqual(keys, []string{"a", "c"}) {
		t.Errorf("remaining keys = %v", keys)
	}
	// Original map must be untouched.
	if len(submitted) != 3 {
		t.Error("StripDiscardedFields must not mutate the original map")
	}
}

func TestStripDiscardedFields_EmptyListReturnsOriginal(t *testing.T) {
	submitted := map[string]interface{}{"a": 1}
	cleaned := StripDiscardedFields(submitted, nil)
	if !reflect.DeepEqual(cleaned, submitted) {
		t.Error("empty discard list should return the map unchanged")
	}
}

func TestRequiredFieldsExemptByPermission_HiddenRequiredFieldExempted(t *testing.T) {
	fields := []Field{
		{Name: "nombre", Required: true},
		{Name: "salario", Required: true}, // hidden from this user
		{Name: "notas", Required: false},
	}
	exempt := RequiredFieldsExemptByPermission(fields, []string{"salario"})
	if !reflect.DeepEqual(exempt, []string{"salario"}) {
		t.Errorf("exempt = %v, want [salario]", exempt)
	}
}

func TestRequiredFieldsExemptByPermission_NotRequiredFieldNeverExempted(t *testing.T) {
	// A field can be hidden without being required -- nothing to exempt.
	fields := []Field{{Name: "notas", Required: false}}
	exempt := RequiredFieldsExemptByPermission(fields, []string{"notas"})
	if len(exempt) != 0 {
		t.Errorf("a non-required hidden field should not appear in the exempt list, got %v", exempt)
	}
}

func TestRequiredFieldsExemptByPermission_VisibleRequiredFieldNeverExempted(t *testing.T) {
	fields := []Field{{Name: "nombre", Required: true}}
	exempt := RequiredFieldsExemptByPermission(fields, nil)
	if len(exempt) != 0 {
		t.Errorf("a visible required field should never be exempted, got %v", exempt)
	}
}
