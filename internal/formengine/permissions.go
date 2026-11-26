package formengine

// This file implements FORM_ENGINE_SPEC.md §3c.3-§3c.4: the data model
// for a role-based permission rule, and the pure algorithm that resolves
// a set of rules plus a user's roles into the Hidden*/ReadOnly* lists
// RenderContext expects.
//
// Same division of responsibility as SourceResolver (sources.go):
// formengine owns the types and the resolution algorithm, since those
// are pure and testable, but not the xolu-backed storage or lookup of
// FieldPermission rules — there's no real caller for that yet
// (Administration module, T-02, doesn't exist). Building the storage
// layer against no real caller risks the exact kind of speculative code
// the T-01 stages are ordered to avoid.

// FieldPermission defines one or more roles' access to a single element
// of a form. FORM_ENGINE_SPEC.md §3c.3. Stored in xolu per tenant — this
// type is the wire/storage shape, not tied to any particular store.
type FieldPermission struct {
	ID       string
	TenantID string

	Module string // e.g. "assets"
	Form   string // e.g. "asset"
	Target string // field name, section ID, tab ID, or row index (as a string)
	Type   string // "field" | "section" | "tab" | "row"

	Roles  []string
	Access string // "editable" | "readonly" | "hidden"
}

// Access levels, §3c.3.
const (
	AccessEditable = "editable"
	AccessReadonly = "readonly"
	AccessHidden   = "hidden"
)

// Permission element types, §3c.3.
const (
	PermTypeField   = "field"
	PermTypeSection = "section"
	PermTypeTab     = "tab"
	PermTypeRow     = "row"
)

// ResolvedPerms is the output of resolving a set of FieldPermission
// rules against one user's roles for one form — everything
// RenderContext's permission fields need. FORM_ENGINE_SPEC.md §3c.4.
type ResolvedPerms struct {
	HiddenFields   []string
	HiddenSections []string
	HiddenTabs     []string
	HiddenRows     map[string][]int // tab ID -> hidden row indices within that tab

	ReadOnlyFields   []string
	ReadOnlySections []string
	ReadOnlyTabs     []string
}

// PermService resolves permissions for a user and form. FORM_ENGINE_SPEC.md
// §3c.4. The concrete, xolu-backed implementation is supplied by the
// caller — this package has no direct xolu dependency, same pattern as
// SourceResolver. Not yet implemented anywhere real (see the T-01
// tracker) — ResolvePermissions below is the pure algorithm a concrete
// implementation would call after fetching the tenant's rules from xolu.
type PermService interface {
	Resolve(tenantID string, userRoles []string, module, form string) *ResolvedPerms
}

// ResolvePermissions implements §3c.4's resolution order for a flat set
// of rules already scoped to one tenant/module/form:
//
//  1. hidden   wins if any matching rule for any of the user's roles says hidden
//  2. readonly wins next if any matching rule says readonly
//  3. editable if any matching rule says editable
//  4. no matching rule -> the tenant default (editable if permissive,
//     hidden if denyByDefault)
//
// "Most restrictive wins" (§3c.4) is the same ordering applied whether
// the conflict comes from one role with contradictory rules or several
// roles each with their own rule — this function doesn't distinguish
// the two cases, since the resolution is identical either way.
func ResolvePermissions(rules []FieldPermission, userRoles []string, denyByDefault bool) *ResolvedPerms {
	roleSet := toSet(userRoles)

	// target -> most restrictive access level found so far, per element type.
	byType := map[string]map[string]string{
		PermTypeField:   {},
		PermTypeSection: {},
		PermTypeTab:     {},
		PermTypeRow:     {},
	}

	for _, rule := range rules {
		if !anyRoleMatches(rule.Roles, roleSet) {
			continue
		}
		m, ok := byType[rule.Type]
		if !ok {
			continue // unrecognised Type -- ignore rather than guess
		}
		current, exists := m[rule.Target]
		if !exists || moreRestrictive(rule.Access, current) {
			m[rule.Target] = rule.Access
		}
	}

	perms := &ResolvedPerms{HiddenRows: map[string][]int{}}
	applyAccessMap(byType[PermTypeField], &perms.HiddenFields, &perms.ReadOnlyFields)
	applyAccessMap(byType[PermTypeSection], &perms.HiddenSections, &perms.ReadOnlySections)
	applyAccessMap(byType[PermTypeTab], &perms.HiddenTabs, &perms.ReadOnlyTabs)
	// Rows have no ReadOnly list in RenderContext (§3c.2 -- a row itself
	// isn't a widget, only its sections/fields are), and Target for a
	// row rule is "tab_id:row_index" (there's no other way to name a
	// specific row -- rows have no ID of their own in the schema, only
	// a position within a tab).
	for target, access := range byType[PermTypeRow] {
		if access != AccessHidden {
			continue
		}
		tabID, idx, ok := splitRowTarget(target)
		if !ok {
			continue
		}
		perms.HiddenRows[tabID] = append(perms.HiddenRows[tabID], idx)
	}

	if denyByDefault {
		// Note (deliberately not implemented here): a true deny-by-
		// default resolution needs the full list of every field/
		// section/tab/row *in the form* to hide everything with no
		// rule -- this function only sees rules, not the schema. A
		// caller wanting deny-by-default combines this result with
		// Form.Fields/Tabs (schema.go) to add the elements with no
		// rule at all to the Hidden* lists. Not invented speculatively
		// here since there's no real caller yet to validate the exact
		// combination shape against (see the T-01 tracker).
		_ = denyByDefault
	}

	return perms
}

func anyRoleMatches(ruleRoles []string, userRoles map[string]bool) bool {
	for _, r := range ruleRoles {
		if userRoles[r] {
			return true
		}
	}
	return false
}

// moreRestrictive reports whether candidate is stricter than current
// under §3c.4's hidden > readonly > editable ordering.
func moreRestrictive(candidate, current string) bool {
	return accessRank(candidate) > accessRank(current)
}

func accessRank(access string) int {
	switch access {
	case AccessHidden:
		return 2
	case AccessReadonly:
		return 1
	case AccessEditable:
		return 0
	default:
		return -1 // unrecognised value never overrides a recognised one
	}
}

func applyAccessMap(m map[string]string, hidden, readOnly *[]string) {
	for target, access := range m {
		switch access {
		case AccessHidden:
			*hidden = append(*hidden, target)
		case AccessReadonly:
			*readOnly = append(*readOnly, target)
			// AccessEditable: no-op -- editable is the absence of a
			// restriction, nothing to add to either list.
		}
	}
}

// splitRowTarget parses a row rule's Target ("tab_id:row_index") into
// its parts. Returns ok=false for anything that doesn't parse as
// "<non-empty tab id>:<non-negative integer>".
func splitRowTarget(target string) (tabID string, index int, ok bool) {
	for i := len(target) - 1; i >= 0; i-- {
		if target[i] == ':' {
			tabID = target[:i]
			idxStr := target[i+1:]
			if tabID == "" || idxStr == "" {
				return "", 0, false
			}
			n := 0
			for _, c := range idxStr {
				if c < '0' || c > '9' {
					return "", 0, false
				}
				n = n*10 + int(c-'0')
			}
			return tabID, n, true
		}
	}
	return "", 0, false
}
