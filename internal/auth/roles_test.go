package auth

import (
	"reflect"
	"testing"
)

// Every role ValidRoles can return must have a rank, otherwise adding a
// fifth role without ranking it would silently hide its users from every
// hierarchy-filtered list.
func TestRoleRank_CoversEveryValidRole(t *testing.T) {
	for _, r := range ValidRoles() {
		if RoleRank(r) == 0 {
			t.Errorf("valid role %q has no rank", r)
		}
	}
}

func TestRoleRank_Order(t *testing.T) {
	if !(RoleRank(RoleAdmin) > RoleRank(RoleManager) &&
		RoleRank(RoleManager) > RoleRank(RoleOperator) &&
		RoleRank(RoleOperator) > RoleRank(RoleViewer) &&
		RoleRank(RoleViewer) > 0) {
		t.Errorf("want admin > manager > operator > viewer > 0")
	}
}

func TestRoleRank_UnknownAndEmptyAreZero(t *testing.T) {
	for _, r := range []string{"", "root", "ADMIN", " admin"} {
		if got := RoleRank(r); got != 0 {
			t.Errorf("RoleRank(%q) = %d, want 0", r, got)
		}
	}
}

func TestRolesAtOrBelow(t *testing.T) {
	cases := []struct {
		role string
		want []string
	}{
		{RoleAdmin, []string{RoleAdmin, RoleManager, RoleOperator, RoleViewer, RoleAdminStaff, RoleAccountant, RoleAccountExecutive}},
		{RoleManager, []string{RoleManager, RoleOperator, RoleViewer}},
		{RoleOperator, []string{RoleOperator, RoleViewer}},
		{RoleViewer, []string{RoleViewer}},
		{RoleAdminStaff, []string{RoleAdminStaff, RoleAccountant, RoleAccountExecutive}},
		{RoleAccountant, []string{RoleAccountant, RoleAccountExecutive}},
		{RoleAccountExecutive, []string{RoleAccountExecutive}},
	}
	for _, c := range cases {
		if got := RolesAtOrBelow(c.role); !reflect.DeepEqual(got, c.want) {
			t.Errorf("RolesAtOrBelow(%q) = %v, want %v", c.role, got, c.want)
		}
	}
}

// Unknown or empty caller role lists nobody. The result must be a
// non-nil empty slice: UserFilter treats a nil Roles as "no role filter"
// and a non-nil empty Roles as "match nothing".
func TestRolesAtOrBelow_UnknownCallerSeesNobody(t *testing.T) {
	for _, r := range []string{"", "root", "ADMIN"} {
		got := RolesAtOrBelow(r)
		if got == nil || len(got) != 0 {
			t.Errorf("RolesAtOrBelow(%q) = %#v, want non-nil empty slice", r, got)
		}
	}
}

func TestRoleAtOrBelow(t *testing.T) {
	cases := []struct {
		caller, target string
		want           bool
	}{
		{RoleAdmin, RoleAdmin, true},
		{RoleAdmin, RoleViewer, true},
		{RoleManager, RoleAdmin, false},
		{RoleManager, RoleManager, true},
		{RoleOperator, RoleManager, false},
		{RoleOperator, RoleViewer, true},
		{RoleViewer, RoleOperator, false},
		{RoleViewer, RoleViewer, true},
		{"", RoleViewer, false},
		{RoleAdmin, "", false},
		{"root", RoleViewer, false},
		{RoleAdmin, "root", false},

		// The two ladders never see each other; admin sees both.
		{RoleManager, RoleAccountExecutive, false},
		{RoleAdminStaff, RoleViewer, false},
		{RoleAccountant, RoleOperator, false},
		{RoleAdmin, RoleAdminStaff, true},
		{RoleAdminStaff, RoleAccountant, true},
		{RoleAccountant, RoleAdminStaff, false},
		{RoleAccountExecutive, RoleAccountant, false},
	}
	for _, c := range cases {
		if got := RoleAtOrBelow(c.caller, c.target); got != c.want {
			t.Errorf("RoleAtOrBelow(%q, %q) = %v, want %v", c.caller, c.target, got, c.want)
		}
	}
}

// ---- Developer role (campaign DEV-6) ----

func TestDeveloper_RankAndValidity(t *testing.T) {
	if RoleRank(RoleDeveloper) <= RoleRank(RoleAdmin) {
		t.Error("developer must outrank admin")
	}
	if !IsValidRole(RoleDeveloper) {
		t.Error("developer must be a valid role")
	}
	if got := ValidRoles(); got[0] != RoleDeveloper {
		t.Errorf("ValidRoles must list the most senior first, got %v", got)
	}
}

func TestDeveloper_PermissionsAreAdminPlusDeveloperAccess(t *testing.T) {
	dev := map[Permission]bool{}
	for _, p := range rolePermissions[RoleDeveloper] {
		dev[p] = true
	}
	for _, p := range rolePermissions[RoleAdmin] {
		if !dev[p] {
			t.Errorf("developer lacks admin permission %q", p)
		}
	}
	if len(rolePermissions[RoleDeveloper]) != len(rolePermissions[RoleAdmin])+1 {
		t.Errorf("developer should hold exactly admin's permissions plus one, got %d vs %d",
			len(rolePermissions[RoleDeveloper]), len(rolePermissions[RoleAdmin]))
	}
	if !HasPermission(RoleDeveloper, PermDeveloperAccess) {
		t.Error("developer must hold developer.access")
	}
	for _, r := range []string{RoleAdmin, RoleManager, RoleOperator, RoleViewer, "", "root"} {
		if HasPermission(r, PermDeveloperAccess) {
			t.Errorf("role %q must not hold developer.access", r)
		}
	}
}

func TestDeveloper_ModuleAccess(t *testing.T) {
	if !HasModuleAccess(RoleDeveloper, "developer") {
		t.Error("developer should have access to the developer module")
	}
	for _, r := range []string{RoleAdmin, RoleManager, RoleOperator, RoleViewer} {
		if HasModuleAccess(r, "developer") {
			t.Errorf("%s must not have access to the developer module", r)
		}
	}
	// and everything admin can reach, the developer can reach
	for _, m := range []string{"assets", "settings", "administration", "documents"} {
		if !HasModuleAccess(RoleDeveloper, m) {
			t.Errorf("developer lost access to module %q", m)
		}
	}
}

func TestDeveloper_AdminDoesNotInheritDeveloperGrant(t *testing.T) {
	before := len(rolePermissions[RoleAdmin])
	for _, p := range rolePermissions[RoleAdmin] {
		if p == PermDeveloperAccess {
			t.Fatal("admin must not hold developer.access")
		}
	}
	// the init() derivation must not alias admin's slice
	rolePermissions[RoleDeveloper] = append(rolePermissions[RoleDeveloper], Permission("x.y.z"))
	if len(rolePermissions[RoleAdmin]) != before {
		t.Error("developer's permission slice aliases admin's")
	}
	rolePermissions[RoleDeveloper] = rolePermissions[RoleDeveloper][:len(rolePermissions[RoleDeveloper])-1]
}

func TestClaimsHasRole_DeveloperIsASuperuserLikeAdmin(t *testing.T) {
	cases := []struct {
		role, check string
		want        bool
	}{
		{RoleDeveloper, RoleViewer, true},
		{RoleDeveloper, RoleAdmin, true},
		{RoleDeveloper, "anything", true},
		{RoleAdmin, RoleDeveloper, true}, // unchanged: admin passes every HasRole check
		{RoleManager, RoleAdmin, false},
		{RoleManager, RoleManager, true},
		{"", RoleAdmin, false},
		{"root", RoleAdmin, false},
	}
	for _, c := range cases {
		cl := &Claims{Role: c.role}
		if got := cl.HasRole(c.check); got != c.want {
			t.Errorf("Claims{%q}.HasRole(%q) = %v, want %v", c.role, c.check, got, c.want)
		}
	}
}

func TestRolesAtOrBelow_DeveloperSeesEveryoneAdminDoesNotSeeDeveloper(t *testing.T) {
	want := []string{RoleDeveloper, RoleAdmin, RoleManager, RoleOperator, RoleViewer, RoleAdminStaff, RoleAccountant, RoleAccountExecutive}
	if got := RolesAtOrBelow(RoleDeveloper); !reflect.DeepEqual(got, want) {
		t.Errorf("RolesAtOrBelow(developer) = %v, want %v", got, want)
	}
	for _, r := range RolesAtOrBelow(RoleAdmin) {
		if r == RoleDeveloper {
			t.Error("admin must not see developer accounts")
		}
	}
	if RoleAtOrBelow(RoleAdmin, RoleDeveloper) {
		t.Error("RoleAtOrBelow(admin, developer) must be false")
	}
	if !RoleAtOrBelow(RoleDeveloper, RoleAdmin) {
		t.Error("RoleAtOrBelow(developer, admin) must be true")
	}
}

func TestCanAssignRole(t *testing.T) {
	cases := []struct {
		actor, target string
		want          bool
	}{
		{RoleDeveloper, RoleDeveloper, true},
		{RoleDeveloper, RoleAdmin, true},
		{RoleAdmin, RoleDeveloper, false}, // the escalation this rule exists to stop
		{RoleManager, RoleDeveloper, false},
		{"", RoleDeveloper, false},
		{RoleAdmin, RoleAdmin, true}, // unchanged behaviour
		{RoleAdmin, RoleViewer, true},
		{RoleAdmin, "root", false}, // invalid role is never assignable
		{RoleDeveloper, "", false},
	}
	for _, c := range cases {
		if got := CanAssignRole(c.actor, c.target); got != c.want {
			t.Errorf("CanAssignRole(%q,%q) = %v, want %v", c.actor, c.target, got, c.want)
		}
	}
}

func TestCanManageUser(t *testing.T) {
	cases := []struct {
		actor, target string
		want          bool
	}{
		{RoleDeveloper, RoleDeveloper, true},
		{RoleDeveloper, RoleAdmin, true},
		{RoleAdmin, RoleDeveloper, false}, // an admin must not reset a developer's password
		{RoleManager, RoleDeveloper, false},
		{"", RoleDeveloper, false},
		{RoleAdmin, RoleAdmin, true}, // unchanged behaviour
		{RoleAdmin, RoleViewer, true},
	}
	for _, c := range cases {
		if got := CanManageUser(c.actor, c.target); got != c.want {
			t.Errorf("CanManageUser(%q,%q) = %v, want %v", c.actor, c.target, got, c.want)
		}
	}
}
