package formengine

import (
	"encoding/json"

	"github.com/ha1tch/seam-ui/internal/condition"
)

// VisibleWhen is formengine's own name for a condition tree controlling
// conditional visibility of a field, section, row, or tab.
// FORM_ENGINE_SPEC.md §3b.2-§3b.5. Either a leaf (Field/Op/Value set) or
// a composite (AllOf or AnyOf set) -- not both on the same node.
//
// A type alias, not a new type: the actual definition and every
// evaluation rule now live in internal/condition, formengine's own
// package-level dependency on Seam's shared condition-evaluation
// primitives (previously implemented independently, right here, before
// the same pattern was found duplicated -- with real, undocumented
// semantic drift -- in at least three other places in this codebase).
// Kept as an alias, not a rename at every call site, specifically so
// nothing outside this file needed to change: schema.go's struct fields,
// renderer.go's render-time checks, and hidden_field_policy.go's
// submit-time checks all still read exactly as before.
type VisibleWhen = condition.Condition

// parseVisibleWhen decodes a visible_when / x-seam-visible-when value.
// Returns (nil, nil) when raw is empty or JSON null -- "no condition" is
// not an error.
func parseVisibleWhen(raw json.RawMessage) (*VisibleWhen, error) {
	return condition.ParseJSON(raw)
}

// EvaluateVisibleWhen evaluates a condition tree against the current
// field values (§3b.4-§3b.5). A nil condition is always true -- "no
// condition" means "always visible". values should be the same snapshot
// used to populate the form's widgets; per §3b.8, a condition that
// references a field the current user can't see (permission-hidden) is
// evaluated against that field's persisted value, which is exactly what
// this snapshot already is -- the render-time values map, not live form
// input -- so no special case is needed for that edge case.
func EvaluateVisibleWhen(cond *VisibleWhen, values map[string]interface{}) bool {
	return condition.Evaluate(cond, values)
}

// isEmpty matches §3b.4's "empty" definition exactly: null, "", or 0 --
// not Go's broader zero-value notion (false is NOT empty by this rule).
// Used directly by hidden_field_policy.go, independent of full condition
// evaluation.
func isEmpty(v interface{}) bool {
	return condition.IsEmpty(v)
}
