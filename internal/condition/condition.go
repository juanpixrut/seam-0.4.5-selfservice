// Package condition provides Seam's own value-comparison and composite
// (AND/OR) condition-evaluation primitives.
//
// Extracted from formengine's visibility system (FORM_ENGINE_SPEC.md
// §3b), which is still this package's only real consumer today and the
// origin of every semantic choice below -- but kept here, one level up
// from formengine specifically, because the same shape of problem
// ("compare a value against a rule, possibly nested under AND/OR") is a
// recurring one, not unique to form-field visibility.
//
// Deliberately not built on or kept in sync with any external library's
// own condition-evaluation semantics (specifically: github.com/ha1tch/
// minty's mintydyn package, which was investigated as a shared
// dependency and found to have already diverged in ways that matter --
// its "empty" treats false as empty, where Seam's own rule explicitly
// doesn't; it lacks gte/lte entirely; its equals allows numeric coercion
// across booleans, which Seam's own looseEqual deliberately guards
// against). This package is Seam's own single source of truth for these
// semantics, not a shim around someone else's.
//
// Currently internal, not exported as its own module -- see docs/
// TRACKING.md T-20 for the open question of whether it should become one
// later, once there's more evidence it's earned that. No deadline.
package condition

import "encoding/json"

// Condition is a composite condition tree: either a leaf (Field/Op/Value
// set) or a composite (AllOf or AnyOf set) -- not both on the same node.
type Condition struct {
	Field string      `json:"field,omitempty"`
	Op    string      `json:"op,omitempty"`
	Value interface{} `json:"value,omitempty"`

	AllOf []Condition `json:"all_of,omitempty"`
	AnyOf []Condition `json:"any_of,omitempty"`
}

// ParseJSON decodes a condition from raw JSON. Returns (nil, nil) when
// raw is empty or JSON null -- "no condition" is not an error.
func ParseJSON(raw json.RawMessage) (*Condition, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var c Condition
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Evaluate evaluates a condition tree against a set of values. A nil
// condition is always true -- "no condition" means "always satisfied".
//
// values should be the same snapshot the caller wants conditions judged
// against (a form's current field values, a submitted payload, etc.) --
// this package has no opinion on where that snapshot comes from or when
// it was taken, only on how to evaluate a condition against it.
//
// A field referenced in a leaf condition that doesn't exist in values
// degrades to "condition not met", not an error -- evaluateOp already
// receives nil for a missing key, which every operator below treats as
// empty/not-equal/not-in as appropriate for that operator, rather than
// needing a separate "field exists" check first.
func Evaluate(cond *Condition, values map[string]interface{}) bool {
	if cond == nil {
		return true
	}
	if len(cond.AllOf) > 0 {
		for _, c := range cond.AllOf {
			if !Evaluate(&c, values) {
				return false
			}
		}
		return true
	}
	if len(cond.AnyOf) > 0 {
		for _, c := range cond.AnyOf {
			if Evaluate(&c, values) {
				return true
			}
		}
		return false
	}
	actual := values[cond.Field]
	return evaluateOp(cond.Op, actual, cond.Value)
}

// CollectFields returns every field name referenced anywhere in a
// condition tree, walking AllOf/AnyOf recursively -- the Go-side
// equivalent of knowing which fields a condition depends on, e.g. for
// deciding which fields' changes should trigger re-evaluation.
func CollectFields(cond *Condition) []string {
	if cond == nil {
		return nil
	}
	var out []string
	if len(cond.AllOf) > 0 {
		for _, c := range cond.AllOf {
			out = append(out, CollectFields(&c)...)
		}
		return out
	}
	if len(cond.AnyOf) > 0 {
		for _, c := range cond.AnyOf {
			out = append(out, CollectFields(&c)...)
		}
		return out
	}
	if cond.Field != "" {
		out = append(out, cond.Field)
	}
	return out
}

func evaluateOp(op string, actual, want interface{}) bool {
	switch op {
	case "eq":
		return looseEqual(actual, want)
	case "neq":
		return !looseEqual(actual, want)
	case "in":
		return inList(actual, want)
	case "not_in":
		return !inList(actual, want)
	case "empty":
		return IsEmpty(actual)
	case "not_empty":
		return !IsEmpty(actual)
	case "gt", "gte", "lt", "lte":
		af, aok := asFloat(actual)
		wf, wok := asFloat(want)
		if !aok || !wok {
			return false
		}
		switch op {
		case "gt":
			return af > wf
		case "gte":
			return af >= wf
		case "lt":
			return af < wf
		default: // "lte"
			return af <= wf
		}
	default:
		// Unknown operator: fail closed (condition not met). An
		// authoring mistake should hide something, not crash or
		// silently show it -- the same reasoning as a missing field
		// above.
		return false
	}
}

// IsEmpty matches this package's own "empty" definition exactly: nil,
// "", or a numeric zero -- not Go's broader zero-value notion. false is
// deliberately NOT empty by this rule, unlike a naive falsy check --
// this is the one semantic difference from every other JS-side "empty"
// helper (including mintydyn's own) checked against while building this
// package, and it's deliberate, not an oversight: a checkbox's unchecked
// state is meaningful, real data, not an absence of data.
//
// Exported because callers legitimately need this check independent of
// full condition evaluation (e.g. deciding whether a submitted value for
// a hidden field is worth caring about at all, before asking whether the
// field's own visibility condition holds).
func IsEmpty(v interface{}) bool {
	if v == nil {
		return true
	}
	switch t := v.(type) {
	case string:
		return t == ""
	case float64:
		return t == 0
	case int, int64:
		return asInt(t) == 0
	}
	return false
}

func asInt(v interface{}) int64 {
	switch t := v.(type) {
	case int:
		return int64(t)
	case int64:
		return t
	}
	return 0
}

func asFloat(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	}
	return 0, false
}

// looseEqual allows numeric coercion (so "5" and 5 and 5.0 all compare
// equal) via asFloat. Unlike the hand-written client-side JS mirror of
// this same logic, no explicit guard against boolean/numeric coercion is
// needed here: JS's Number(false) coerces to 0, so that side needs an
// explicit typeof check to stop false == 0 from holding; Go's asFloat
// has no case for bool at all, so asFloat(false) already fails cleanly,
// falling through to a plain == comparison -- and Go's own interface
// equality is type-strict, so a bool value is never == to a float64
// value regardless of their underlying values. Verified directly
// (looseEqual(false, float64(0)) returns false with no guard present)
// before assuming the two languages needed identical treatment here --
// they don't, and adding a redundant guard here was tried and reverted
// during this package's own development for exactly that reason.
func looseEqual(a, b interface{}) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if af, aok := asFloat(a); aok {
		if bf, bok := asFloat(b); bok {
			return af == bf
		}
	}
	return a == b
}

func inList(v interface{}, list interface{}) bool {
	items, ok := list.([]interface{})
	if !ok {
		return false
	}
	for _, item := range items {
		if looseEqual(v, item) {
			return true
		}
	}
	return false
}
