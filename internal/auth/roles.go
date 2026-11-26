package auth

// Role ordering for "who may see whom" rules (T-96 Users selector).
//
// The core roles form a strict hierarchy: developer > admin > manager >
// operator > viewer. The three Accounting & Admin roles form a second one, admin_staff >
// accountant > account_executive, and admin and developer stand above both. This is
// separate from the permission matrix in auth.go (which stays the single
// source of truth for RBAC): rank is only used where a caller's visibility of
// other users depends on how senior they are.
//
// An unknown or empty role has rank 0 and is treated as "sees nobody" /
// "is seen by nobody" everywhere below, never as "no restriction".

// roleRanks maps each valid role to its rank; higher means more senior.
var roleRanks = map[string]int{
	RoleViewer:    1,
	RoleOperator:  2,
	RoleManager:   3,
	RoleAdmin:     4,
	RoleDeveloper: 5,

	// Accounting & Admin family. A second ladder, not a continuation of the
	// first: see roleFamily.
	RoleAccountExecutive: 1,
	RoleAccountant:       2,
	RoleAdminStaff:       3,
}

// roleFamily says which ladder a role stands on. Ranks are only compared
// inside one family, so a manager never sees the Accounting users (an AE's
// name, email and payout data) and an accountant never sees tenant users;
// admin and developer, the roles above both ladders, see everybody.
func roleFamily(role string) string {
	switch role {
	case RoleAccountExecutive, RoleAccountant, RoleAdminStaff:
		return "accounting"
	case RoleViewer, RoleOperator, RoleManager, RoleAdmin, RoleDeveloper:
		return "core"
	}
	return ""
}

// RoleRank returns the rank of a role on its own ladder (developer 5, admin 4, manager 3,
// operator 2, viewer 1; admin_staff 3, accountant 2, account_executive 1).
// Unknown and empty roles return 0.
func RoleRank(role string) int {
	return roleRanks[role]
}

// RolesAtOrBelow returns the valid roles whose rank is equal to or lower
// than the given role's, most senior first (the order of ValidRoles).
// An unknown or empty role yields an empty, non-nil slice: the caller
// sees nobody.
func RolesAtOrBelow(role string) []string {
	out := []string{}
	if RoleRank(role) == 0 {
		return out
	}
	for _, r := range ValidRoles() {
		if RoleAtOrBelow(role, r) {
			out = append(out, r)
		}
	}
	return out
}

// RoleAtOrBelow reports whether targetRole is visible to callerRole.
// Developer sees everybody. Admin stands above both ladders and sees
// everybody except developer. Other roles only see roles within their
// family up to their own rank. It is false whenever either role is unknown or empty.
func RoleAtOrBelow(callerRole, targetRole string) bool {
	c, t := RoleRank(callerRole), RoleRank(targetRole)
	if c == 0 || t == 0 {
		return false
	}
	if callerRole == RoleDeveloper {
		return true
	}
	if callerRole == RoleAdmin {
		return targetRole != RoleDeveloper
	}
	return roleFamily(callerRole) == roleFamily(targetRole) && t <= c
}

// PermDeveloperAccess gates the Developer module (campaign DEV-6). Only the
// developer role holds it; admin deliberately does not.
const PermDeveloperAccess Permission = "developer.access"

func init() {
	// The developer is an admin plus the Developer module. Deriving the
	// grant here, rather than listing it, means a permission added to admin
	// later is inherited automatically.
	rolePermissions[RoleDeveloper] = append(
		append([]Permission(nil), rolePermissions[RoleAdmin]...),
		PermDeveloperAccess,
	)
}

// CanAssignRole reports whether an actor holding actorRole may give
// targetRole to a user. The target must be a valid role, and the developer
// role may only be assigned by a developer: otherwise an admin could
// promote themselves, or anyone, past admin. Every other assignment is
// unchanged from before the developer role existed.
func CanAssignRole(actorRole, targetRole string) bool {
	if !IsValidRole(targetRole) {
		return false
	}
	return targetRole != RoleDeveloper || actorRole == RoleDeveloper
}

// CanManageUser reports whether an actor holding actorRole may edit,
// disable, delete or reset the password of a user whose current role is
// targetRole. Developer accounts can only be managed by developers;
// without this, an admin could reset a developer's password and sign in
// as one. Every other case is unchanged.
func CanManageUser(actorRole, targetRole string) bool {
	return targetRole != RoleDeveloper || actorRole == RoleDeveloper
}
