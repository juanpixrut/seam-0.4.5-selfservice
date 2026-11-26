package ui

import (
	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/features"
)

// flagOn reports whether a flag is on: from the resolved flags when there are
// any, otherwise the catalogue's default, for the tests and hand-built configs
// that have no flag set. The Developer module's decisions are read through it,
// so each can be chosen by a deployment (see the comment on the flags in
// internal/features/catalog.go for what each default is and why).
// mayViewAs reports whether a session whose real role is realRole may choose to
// view the application as role: the decision is switched on, the Developer
// module is on, and the role is one a developer may view as. Leaving (an empty
// role) is never a choice and is always allowed, so it is not asked here.
func (c Config) mayViewAs(realRole, role string) bool {
	if !c.EnableDeveloper || !c.flagOn(features.DeveloperViewAs) || !auth.CanViewAs(realRole, role) {
		return false
	}
	for _, r := range c.viewAsRoles() {
		if r == role {
			return true
		}
	}
	return false
}

// viewAsRoles lists the roles a developer of this instance may view the
// application as, in the order the form offers them: the roles of the
// platform, and those of Accounting where that module is on (on an instance
// without it they have nothing to look at).
func (c Config) viewAsRoles() []string {
	roles := []string{auth.RoleAdmin, auth.RoleManager, auth.RoleOperator, auth.RoleViewer}
	if c.EnableAccounting {
		roles = append(roles, auth.RoleAdminStaff, auth.RoleAccountant, auth.RoleAccountExecutive)
	}
	return roles
}

func (c Config) flagOn(key string) bool {
	if c.Flags != nil {
		return c.Flags.On(key)
	}
	return features.DefaultOf(key)
}
