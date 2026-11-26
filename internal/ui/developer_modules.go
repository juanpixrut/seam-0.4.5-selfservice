package ui

import (
	"net/http"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/features"
)

// Switching a module on at runtime (campaign DEV-27).
//
// The Demo, Work Orders and Issues modules are off unless their own flag is on
// at startup, and routes cannot be added to the mux afterwards. So, while the
// Developer module and its developer_module_switches flag are on, their routes
// are mounted in any case and each request is asked whether the module is on
// for the caller's tenant: on at startup (for everyone, as before), or switched
// on for that tenant from the Developer panel (the runtime flags module_demo,
// module_work_orders and module_issues, kept with the other switches). A request for a module
// that is off gets the same 404 an unmounted route gets. The module registry
// asks the same question, so the menus agree with the routes.
//
// With developer_module_switches off nothing here applies: the modules are
// mounted only when their own flag is on, exactly as before.

// ModuleOn reports whether a module is on for the tenant: startup is the
// module's own flag, runtimeKey the switch that can turn it on for one tenant.
//
// Who a module switched on at runtime is on for is a setting
// (developer_module_audience): the tenant's developers only, which is the
// default, since the switch is how a developer tries a module that the
// deployment has not enabled; or every user of the tenant with access to it.
func (c Config) ModuleOn(tenant int64, role string, startup bool, runtimeKey string) bool {
	if startup {
		return true
	}
	if !c.moduleSwitches() || tenant <= 0 {
		return false
	}
	if c.DevSettings.ModuleAudience != ModuleAudienceTenant && role != auth.RoleDeveloper {
		return false
	}
	on, _ := c.DevRuntime.Manager.Effective(tenant, runtimeKey)
	return on
}

// moduleSwitches reports whether modules can be switched on at runtime here.
func (c Config) moduleSwitches() bool {
	return c.EnableDeveloper && c.DevRuntime != nil && c.DevRuntime.Manager != nil && c.flagOn(features.DeveloperModuleSwitches)
}

// moduleGate returns what wraps a module's handlers: nothing when the module
// is on at startup, otherwise the per-request question. It goes inside
// requireAuth or requirePerm, which have put the caller's claims in the context.
func (r *Router) moduleGate(startup bool, runtimeKey string) func(http.HandlerFunc) http.HandlerFunc {
	if startup {
		return func(next http.HandlerFunc) http.HandlerFunc { return next }
	}
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, req *http.Request) {
			claims, ok := auth.ClaimsFromContext(req.Context())
			if !ok || !r.config.ModuleOn(claims.TenantID, claims.Role, false, runtimeKey) {
				http.NotFound(w, req)
				return
			}
			next(w, req)
		}
	}
}
