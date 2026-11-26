package schemaoverlay

import (
	"encoding/json"
	"fmt"
	"sort"
)

// Compat compares an override with the schema it would replace and returns
// warnings about changes that can break existing data entry or stored data:
// a top-level property that disappeared, a property whose "type" changed, and
// a property that became required. It is advisory, not a gate: an override
// may legitimately do any of these. The full lifecycle policy (ADMIN_MODULE_SPEC
// 15.5) belongs to the Admin module; this is the early warning for developers.
//
// Only top-level "properties" and "required" are compared. If either document
// cannot be parsed, a single warning says so.
func Compat(base, override []byte) []string {
	type doc struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	var b, o doc
	if err := json.Unmarshal(base, &b); err != nil {
		return []string{fmt.Sprintf("the base schema could not be read for comparison: %v", err)}
	}
	if err := json.Unmarshal(override, &o); err != nil {
		return []string{fmt.Sprintf("the override could not be read for comparison: %v", err)}
	}

	typeOf := func(raw json.RawMessage) string {
		var p struct {
			Type any `json:"type"`
		}
		if json.Unmarshal(raw, &p) != nil || p.Type == nil {
			return ""
		}
		return fmt.Sprint(p.Type)
	}

	var warns []string
	names := make([]string, 0, len(b.Properties))
	for name := range b.Properties {
		names = append(names, name)
	}
	sort.Strings(names)

	baseRequired := map[string]bool{}
	for _, r := range b.Required {
		baseRequired[r] = true
	}

	for _, name := range names {
		oraw, ok := o.Properties[name]
		if !ok {
			warns = append(warns, fmt.Sprintf("property %q was removed", name))
			continue
		}
		if bt, ot := typeOf(b.Properties[name]), typeOf(oraw); bt != ot && bt != "" {
			warns = append(warns, fmt.Sprintf("property %q changed type from %s to %s", name, bt, orNone(ot)))
		}
	}

	var newlyRequired []string
	for _, r := range o.Required {
		if !baseRequired[r] {
			newlyRequired = append(newlyRequired, r)
		}
	}
	sort.Strings(newlyRequired)
	for _, r := range newlyRequired {
		warns = append(warns, fmt.Sprintf("property %q became required", r))
	}
	return warns
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}
