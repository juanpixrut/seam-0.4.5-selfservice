package auth

import "context"

// View-as (campaign DEV-22). A developer can look at the application as a user
// with a lower role would, to check what that role sees and can do. The viewed-as
// role travels in the signed session; the request's claims carry it as their
// role, so every existing permission check and module gate sees the lower role
// unchanged, and the real role sits under a separate context key that only the
// Developer module's own access check and the layout's notice read. The user ID,
// the tenant and the account stay the developer's.

type realRoleKey struct{}

// CanViewAs reports whether a session whose real role is realRole may view the
// application as target: only a developer may, and only as another valid role.
// Choosing the developer role is not viewing as anything; it is the real role.
func CanViewAs(realRole, target string) bool {
	return realRole == RoleDeveloper && IsValidRole(target) && target != RoleDeveloper
}

// ContextWithRealRole records the role the session really has, when it is not
// the one the request's claims carry.
func ContextWithRealRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, realRoleKey{}, role)
}

// RealRoleFromContext returns the real role recorded by ContextWithRealRole.
func RealRoleFromContext(ctx context.Context) (string, bool) {
	r, ok := ctx.Value(realRoleKey{}).(string)
	return r, ok && r != ""
}

// RealRole is the role the session really has: the recorded one while viewing
// as another role, otherwise the claims' own.
func RealRole(ctx context.Context, claims *Claims) string {
	if r, ok := RealRoleFromContext(ctx); ok {
		return r
	}
	if claims != nil {
		return claims.Role
	}
	return ""
}
