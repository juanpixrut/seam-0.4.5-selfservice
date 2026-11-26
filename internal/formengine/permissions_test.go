package formengine

import (
	"reflect"
	"sort"
	"testing"
)

func TestResolvePermissions_SingleRoleHidden(t *testing.T) {
	rules := []FieldPermission{
		{Module: "assets", Form: "asset", Target: "salario", Type: PermTypeField, Roles: []string{"technician"}, Access: AccessHidden},
	}
	perms := ResolvePermissions(rules, []string{"technician"}, false)
	if !reflect.DeepEqual(perms.HiddenFields, []string{"salario"}) {
		t.Errorf("HiddenFields = %v", perms.HiddenFields)
	}
}

func TestResolvePermissions_SingleRoleReadonly(t *testing.T) {
	rules := []FieldPermission{
		{Target: "nombre", Type: PermTypeField, Roles: []string{"viewer"}, Access: AccessReadonly},
	}
	perms := ResolvePermissions(rules, []string{"viewer"}, false)
	if !reflect.DeepEqual(perms.ReadOnlyFields, []string{"nombre"}) {
		t.Errorf("ReadOnlyFields = %v", perms.ReadOnlyFields)
	}
	if len(perms.HiddenFields) != 0 {
		t.Errorf("HiddenFields should be empty, got %v", perms.HiddenFields)
	}
}

func TestResolvePermissions_EditableIsNoOp(t *testing.T) {
	rules := []FieldPermission{
		{Target: "nombre", Type: PermTypeField, Roles: []string{"admin"}, Access: AccessEditable},
	}
	perms := ResolvePermissions(rules, []string{"admin"}, false)
	if len(perms.HiddenFields) != 0 || len(perms.ReadOnlyFields) != 0 {
		t.Errorf("an editable rule should add nothing to either list, got hidden=%v readonly=%v", perms.HiddenFields, perms.ReadOnlyFields)
	}
}

func TestResolvePermissions_NoMatchingRole_PermissiveDefault(t *testing.T) {
	rules := []FieldPermission{
		{Target: "salario", Type: PermTypeField, Roles: []string{"admin"}, Access: AccessHidden},
	}
	// user has "technician", not "admin" -- rule doesn't apply to them
	perms := ResolvePermissions(rules, []string{"technician"}, false)
	if len(perms.HiddenFields) != 0 {
		t.Errorf("rule for a role the user doesn't have should not apply, got %v", perms.HiddenFields)
	}
}

func TestResolvePermissions_MultiRoleConflict_MostRestrictiveWins(t *testing.T) {
	// §3c.4: hidden > readonly > editable, regardless of which rule
	// order they're supplied in.
	rules := []FieldPermission{
		{Target: "salario", Type: PermTypeField, Roles: []string{"technician"}, Access: AccessEditable},
		{Target: "salario", Type: PermTypeField, Roles: []string{"supervisor"}, Access: AccessReadonly},
		{Target: "salario", Type: PermTypeField, Roles: []string{"guest"}, Access: AccessHidden},
	}
	// A user holding all three roles should get the most restrictive: hidden.
	perms := ResolvePermissions(rules, []string{"technician", "supervisor", "guest"}, false)
	if !reflect.DeepEqual(perms.HiddenFields, []string{"salario"}) {
		t.Errorf("expected hidden to win over readonly/editable, got hidden=%v readonly=%v", perms.HiddenFields, perms.ReadOnlyFields)
	}

	// Same rules, user only holds technician+supervisor (not guest) ->
	// readonly should win over editable.
	perms2 := ResolvePermissions(rules, []string{"technician", "supervisor"}, false)
	if !reflect.DeepEqual(perms2.ReadOnlyFields, []string{"salario"}) {
		t.Errorf("expected readonly to win over editable, got readonly=%v hidden=%v", perms2.ReadOnlyFields, perms2.HiddenFields)
	}
}

func TestResolvePermissions_ContradictoryRulesSameRole(t *testing.T) {
	// §3c.4 doesn't distinguish "one role, two rules" from "two roles,
	// two rules" -- most-restrictive-wins applies identically.
	rules := []FieldPermission{
		{Target: "x", Type: PermTypeField, Roles: []string{"admin"}, Access: AccessEditable},
		{Target: "x", Type: PermTypeField, Roles: []string{"admin"}, Access: AccessHidden},
	}
	perms := ResolvePermissions(rules, []string{"admin"}, false)
	if !reflect.DeepEqual(perms.HiddenFields, []string{"x"}) {
		t.Errorf("expected hidden to win even from the same role's contradictory rules, got %v", perms.HiddenFields)
	}
}

func TestResolvePermissions_SectionsAndTabs(t *testing.T) {
	rules := []FieldPermission{
		{Target: "custodia", Type: PermTypeSection, Roles: []string{"guest"}, Access: AccessHidden},
		{Target: "tab-admin", Type: PermTypeTab, Roles: []string{"guest"}, Access: AccessReadonly},
	}
	perms := ResolvePermissions(rules, []string{"guest"}, false)
	if !reflect.DeepEqual(perms.HiddenSections, []string{"custodia"}) {
		t.Errorf("HiddenSections = %v", perms.HiddenSections)
	}
	if !reflect.DeepEqual(perms.ReadOnlyTabs, []string{"tab-admin"}) {
		t.Errorf("ReadOnlyTabs = %v", perms.ReadOnlyTabs)
	}
}

func TestResolvePermissions_RowsAreTabScoped(t *testing.T) {
	rules := []FieldPermission{
		{Target: "tab-principal:0", Type: PermTypeRow, Roles: []string{"guest"}, Access: AccessHidden},
		{Target: "tab-loc:2", Type: PermTypeRow, Roles: []string{"guest"}, Access: AccessHidden},
	}
	perms := ResolvePermissions(rules, []string{"guest"}, false)
	if !reflect.DeepEqual(perms.HiddenRows["tab-principal"], []int{0}) {
		t.Errorf("HiddenRows[tab-principal] = %v", perms.HiddenRows["tab-principal"])
	}
	if !reflect.DeepEqual(perms.HiddenRows["tab-loc"], []int{2}) {
		t.Errorf("HiddenRows[tab-loc] = %v", perms.HiddenRows["tab-loc"])
	}
}

func TestResolvePermissions_UnrecognisedTypeIgnored(t *testing.T) {
	rules := []FieldPermission{
		{Target: "x", Type: "bogus", Roles: []string{"admin"}, Access: AccessHidden},
	}
	perms := ResolvePermissions(rules, []string{"admin"}, false)
	if len(perms.HiddenFields)+len(perms.HiddenSections)+len(perms.HiddenTabs)+len(perms.HiddenRows) != 0 {
		t.Errorf("a rule with an unrecognised Type should be ignored entirely, got %+v", perms)
	}
}

func TestSplitRowTarget(t *testing.T) {
	cases := []struct {
		target string
		tabID  string
		idx    int
		wantOK bool
	}{
		{"tab-principal:0", "tab-principal", 0, true},
		{"tab-loc:12", "tab-loc", 12, true},
		{"no-colon-here", "", 0, false},
		{":5", "", 0, false},
		{"tab:", "", 0, false},
		{"tab:abc", "", 0, false},
		{"weird:tab:3", "weird:tab", 3, true}, // last colon wins -- tab IDs could theoretically contain one
	}
	for _, c := range cases {
		tabID, idx, ok := splitRowTarget(c.target)
		if ok != c.wantOK {
			t.Errorf("splitRowTarget(%q) ok = %v, want %v", c.target, ok, c.wantOK)
			continue
		}
		if ok && (tabID != c.tabID || idx != c.idx) {
			t.Errorf("splitRowTarget(%q) = (%q, %d), want (%q, %d)", c.target, tabID, idx, c.tabID, c.idx)
		}
	}
}

func TestResolvePermissions_MultipleFieldsSorted(t *testing.T) {
	rules := []FieldPermission{
		{Target: "b", Type: PermTypeField, Roles: []string{"x"}, Access: AccessHidden},
		{Target: "a", Type: PermTypeField, Roles: []string{"x"}, Access: AccessHidden},
	}
	perms := ResolvePermissions(rules, []string{"x"}, false)
	got := append([]string{}, perms.HiddenFields...)
	sort.Strings(got)
	if !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("HiddenFields (sorted) = %v", got)
	}
}
