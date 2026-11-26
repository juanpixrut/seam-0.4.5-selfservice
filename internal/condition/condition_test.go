package condition

import "testing"

func TestEvaluate_NilConditionAlwaysTrue(t *testing.T) {
	if !Evaluate(nil, map[string]interface{}{}) {
		t.Error("nil condition should always be satisfied")
	}
}

func TestEvaluateOp_AllOperators(t *testing.T) {
	cases := []struct {
		name   string
		op     string
		actual interface{}
		want   interface{}
		result bool
	}{
		{"eq true", "eq", "Comodato", "Comodato", true},
		{"eq false", "eq", "Venta", "Comodato", false},
		{"eq numeric loose", "eq", float64(5), float64(5), true},
		{"neq true", "neq", "Venta", "Comodato", true},
		{"neq false", "neq", "Comodato", "Comodato", false},
		{"in true", "in", "Venta", []interface{}{"Venta", "Comodato"}, true},
		{"in false", "in", "Alquiler", []interface{}{"Venta", "Comodato"}, false},
		{"not_in true", "not_in", "Alquiler", []interface{}{"Venta", "Comodato"}, true},
		{"not_in false", "not_in", "Venta", []interface{}{"Venta", "Comodato"}, false},
		{"empty nil", "empty", nil, nil, true},
		{"empty string", "empty", "", nil, true},
		{"empty zero", "empty", float64(0), nil, true},
		{"empty non-empty string", "empty", "x", nil, false},
		{"not_empty true", "not_empty", "x", nil, true},
		{"not_empty false", "not_empty", "", nil, false},
		{"gt true", "gt", float64(10), float64(5), true},
		{"gt false", "gt", float64(3), float64(5), false},
		{"gte equal", "gte", float64(5), float64(5), true},
		{"lt true", "lt", float64(3), float64(5), true},
		{"lte equal", "lte", float64(5), float64(5), true},
		{"unknown op fails closed", "bogus", "x", "x", false},
		{"gt non-numeric fails closed", "gt", "abc", float64(5), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := evaluateOp(c.op, c.actual, c.want)
			if got != c.result {
				t.Errorf("evaluateOp(%q, %v, %v) = %v, want %v", c.op, c.actual, c.want, got, c.result)
			}
		})
	}
}

// TestLooseEqual_BoolNeverCoercesToNumber is a permanent regression test
// for a mistake caught and corrected during this package's own
// development, not a hypothetical: an explicit boolean-exclusion guard
// was added to looseEqual, reasoning (incorrectly, without checking)
// from the client-side JS mirror's own genuine need for one. Verified
// directly that Go's asFloat has no case for bool at all, so this
// already holds with no guard present -- the guard was removed as
// unnecessary, and this test exists so a future "cleanup" doesn't
// silently reintroduce numeric coercion across booleans by touching
// asFloat's own switch statement without realizing what depends on its
// current shape.
func TestLooseEqual_BoolNeverCoercesToNumber(t *testing.T) {
	if looseEqual(false, float64(0)) {
		t.Error("false should never loosely-equal 0 -- a checkbox's unchecked state is real data, not an absence of data coincidentally equal to zero")
	}
	if looseEqual(true, float64(1)) {
		t.Error("true should never loosely-equal 1")
	}
	if looseEqual(float64(0), false) {
		t.Error("the same check with arguments reversed should also hold")
	}
}

func TestEvaluate_MissingFieldDegradesToFalse(t *testing.T) {
	cond := &Condition{Field: "does_not_exist", Op: "eq", Value: "x"}
	if Evaluate(cond, map[string]interface{}{}) {
		t.Error("a condition on a missing field should evaluate false, not panic or default true")
	}
}

func TestEvaluate_AllOf(t *testing.T) {
	cond := &Condition{AllOf: []Condition{
		{Field: "tenure_type", Op: "eq", Value: "Comodato"},
		{Field: "custodian_id", Op: "not_empty"},
	}}
	both := map[string]interface{}{"tenure_type": "Comodato", "custodian_id": float64(3)}
	if !Evaluate(cond, both) {
		t.Error("allOf with both true should be satisfied")
	}
	oneFalse := map[string]interface{}{"tenure_type": "Comodato", "custodian_id": nil}
	if Evaluate(cond, oneFalse) {
		t.Error("allOf with one false should not be satisfied")
	}
}

func TestEvaluate_AnyOf(t *testing.T) {
	cond := &Condition{AnyOf: []Condition{
		{Field: "status", Op: "eq", Value: "alert"},
		{Field: "status", Op: "eq", Value: "lost"},
	}}
	if !Evaluate(cond, map[string]interface{}{"status": "lost"}) {
		t.Error("anyOf matching the second branch should be satisfied")
	}
	if Evaluate(cond, map[string]interface{}{"status": "ok"}) {
		t.Error("anyOf matching neither branch should not be satisfied")
	}
}

func TestEvaluate_NestedComposite(t *testing.T) {
	cond := &Condition{AllOf: []Condition{
		{Field: "status", Op: "neq", Value: "archived"},
		{AnyOf: []Condition{
			{Field: "tenure_type", Op: "eq", Value: "Comodato"},
			{Field: "tenure_type", Op: "eq", Value: "Alquiler"},
		}},
	}}
	visible := map[string]interface{}{"status": "active", "tenure_type": "Alquiler"}
	if !Evaluate(cond, visible) {
		t.Error("nested allOf/anyOf should be satisfied when both branches hold")
	}
	archived := map[string]interface{}{"status": "archived", "tenure_type": "Alquiler"}
	if Evaluate(cond, archived) {
		t.Error("nested condition should not be satisfied when the outer allOf branch fails")
	}
}

func TestParseJSON_NilForEmptyOrNull(t *testing.T) {
	v, err := ParseJSON(nil)
	if err != nil || v != nil {
		t.Errorf("empty raw should give (nil, nil), got (%v, %v)", v, err)
	}
	v2, err2 := ParseJSON([]byte("null"))
	if err2 != nil || v2 != nil {
		t.Errorf("JSON null should give (nil, nil), got (%v, %v)", v2, err2)
	}
}

func TestParseJSON_ValidCondition(t *testing.T) {
	v, err := ParseJSON([]byte(`{"field":"x","op":"eq","value":"y"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v == nil || v.Field != "x" || v.Op != "eq" || v.Value != "y" {
		t.Errorf("unexpected parse result: %+v", v)
	}
}

func TestParseJSON_InvalidJSONReturnsError(t *testing.T) {
	_, err := ParseJSON([]byte(`{not valid json`))
	if err == nil {
		t.Error("expected an error for malformed JSON")
	}
}

func TestCollectFields_SimpleLeaf(t *testing.T) {
	cond := &Condition{Field: "x", Op: "eq", Value: "y"}
	got := CollectFields(cond)
	if len(got) != 1 || got[0] != "x" {
		t.Errorf("expected [\"x\"], got %v", got)
	}
}

func TestCollectFields_NestedComposite(t *testing.T) {
	cond := &Condition{AllOf: []Condition{
		{Field: "a", Op: "eq", Value: 1},
		{AnyOf: []Condition{
			{Field: "b", Op: "eq", Value: 2},
			{Field: "c", Op: "eq", Value: 3},
		}},
	}}
	got := CollectFields(cond)
	want := map[string]bool{"a": true, "b": true, "c": true}
	if len(got) != 3 {
		t.Fatalf("expected 3 fields, got %d: %v", len(got), got)
	}
	for _, f := range got {
		if !want[f] {
			t.Errorf("unexpected field %q in result", f)
		}
	}
}

func TestCollectFields_NilCondition(t *testing.T) {
	if got := CollectFields(nil); got != nil {
		t.Errorf("expected nil for a nil condition, got %v", got)
	}
}

// TestIsEmpty_FalseIsNotEmpty is a permanent regression test for the
// package's own deliberately-documented rule -- verified against every
// other "empty" definition compared during this package's own
// development (including mintydyn's, which treats false as empty), not
// just asserted.
func TestIsEmpty_FalseIsNotEmpty(t *testing.T) {
	if IsEmpty(false) {
		t.Error("false must not be treated as empty -- it's meaningful, real data (e.g. an unchecked checkbox), not an absence of data")
	}
	if !IsEmpty(nil) {
		t.Error("nil should be empty")
	}
	if !IsEmpty("") {
		t.Error("empty string should be empty")
	}
	if !IsEmpty(float64(0)) {
		t.Error("numeric zero should be empty")
	}
}
