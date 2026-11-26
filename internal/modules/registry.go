// Package modules defines the Module and Registry types (MODULE_SPEC.md
// §3) that let the application be a switchable set of top-level areas
// (Assets, Documents, System) instead of one monolithic router.
//
// This package is deliberately dependency-free with respect to any one
// module's own implementation -- it doesn't import internal/ui or any
// future internal/modules/{assets,documents,system} package. A Module's
// MountRoutes closure is constructed by whoever registers it (internal/
// server, today), and can internally call into whatever router
// abstraction that module actually needs; this package only needs
// *http.ServeMux, the same top-level mux internal/server/server.go
// already builds everything else on.
//
// MOD-4 scope (MODULE_DASHBOARD_DEVELOPMENT_PLAN.md): this package and
// the selector UI that reads it. It does not yet replace internal/server/
// server.go's existing single call into internal/ui's own router setup --
// that replacement is only safe once Assets is actually re-registered
// through this mechanism (MOD-6), not before. Introducing the mechanism
// and proving it correct are this stage's job; migrating the ~3,000 lines
// of already-working Assets routes onto it is a separate, deliberately
// lower-risk stage.
package modules

import "net/http"

// Module is a switchable top-level area of the application. See
// MODULE_SPEC.md §1.1 for the full terminology (Module vs View vs
// Action) and §3.1 for this struct's own design.
type Module struct {
	// ID is the module's own identifier ("assets", "documents",
	// "system") -- also the first segment of every permission string
	// scoped to it (MODULE_SPEC.md §2.1).
	ID string

	// Label is an i18n key, resolved by the selector at render time --
	// never a literal string (this project's own governing rule,
	// MODULE_SPEC.md §8.5).
	Label string

	// Icon is a Material Icons name, matching every other icon reference
	// in this codebase (dashboardengine's widgets, the existing nav).
	Icon string

	// Group is "apps" or "system" -- which selector group this module's
	// entry renders under (§4.1). Any other value is not rendered by
	// SelectorItems (fails closed: an unrecognised group is a
	// configuration mistake, not a reason to show the entry somewhere
	// unintended).
	Group string

	// Primary marks an application a deployment may run Seam for on its
	// own (Work Orders, Issues, Accounting): Registry.Home prefers these
	// over the modules every instance has (Documents, Settings, ...).
	Primary bool

	// DashboardID names the x-seam-dashboard (DASHBOARD_SPEC.md) this
	// module lands on when selected -- an internal schema identifier
	// used by that dashboard's own handler to pick which JSON definition
	// to render, not itself a URL. See Path for where the selector
	// actually navigates.
	DashboardID string

	// Path is the URL the selector navigates to when this module is
	// chosen (§4.3). Deliberately separate from DashboardID -- found
	// necessary implementing the selector itself: Assets' dashboard
	// lives at "/" (the existing root route), not at a path derived from
	// its DashboardID ("assets-dashboard"); conflating the two would
	// have produced a broken href.
	Path string

	// MountRoutes registers this module's own routes onto the shared
	// top-level mux. May be nil for a module still being scaffolded
	// (Registry.MountAll skips a nil MountRoutes rather than panicking).
	MountRoutes func(mux *http.ServeMux)

	// HasAccess reports whether role has access to this module at all --
	// for an Apps-group module, a thin wrapper around auth.HasModuleAccess
	// (MODULE_SPEC.md §2.4's derivation); for System, a function that
	// always returns true (§2.4's own stated exception, expressed as code
	// on System's own registry entry rather than special-cased inside a
	// shared derivation function). Required -- Registry.VisibleTo treats
	// a nil HasAccess as "visible to no one," matching the same fail-
	// closed reasoning dashboardengine.PermissionChecker's own nil case
	// already established (a permission-relevant nil should never
	// silently grant access).
	HasAccess func(role string) bool

	// HasAccessFor, when set, is asked instead of HasAccess by VisibleFor,
	// which knows the viewer's tenant: a module that can be switched on
	// for one tenant at runtime answers per tenant. HasAccess is still
	// required and is what a caller without a tenant gets.
	HasAccessFor func(tenant int64, role string) bool

	// HideFromSelector suppresses this module's own entry in
	// ui.ModuleSelector's rendered dropdown without affecting anything
	// else about it -- VisibleTo still includes it, its routes still
	// mount, its permissions are still real and enforced. Added
	// 2026-08-04 for Account specifically: the layout already has a
	// separate, pre-existing user-avatar dropdown (layout.go) linking
	// straight to /profile, right next to where the module selector
	// itself renders -- showing Account as a second, redundant entry
	// there would be confusing, not informative. A module-level flag
	// rather than a special case in ModuleSelector's own code, since
	// "this module's nav presence is redundant with existing chrome
	// elsewhere" is a property of the module, not of the selector.
	HideFromSelector bool

	// SidebarItems declares this module's own left-sidebar entries
	// (SIDEBAR_SYNC_PLAN_2026-08-06.md), rendered only while this
	// module is the active one (internal/ui determines that by
	// longest-matching Path prefix against the current request).
	// Empty is a fully valid, common choice -- a module with no
	// natural sub-navigation (Documents, one route; Work Orders/
	// Issues, list+create only) simply shows no sidebar section at
	// all rather than one invented to fill space. Order in the slice
	// is render order, top to bottom, exactly as MainNav's own former
	// static list order was.
	SidebarItems []SidebarItem
}

// SidebarItem is one entry a module can declare for the left sidebar.
// Deliberately as dependency-free as Module's own Label/Icon/Path
// fields -- no minty/mi.H types -- matching this package's own
// documented principle of not depending on internal/ui (see this
// file's own top-of-file comment). internal/ui converts each into its
// own NavItem at render time, computing Active fresh per request; that
// conversion is the correct direction for the dependency to run.
type SidebarItem struct {
	// Label is an i18n key, matching Module.Label's own convention --
	// never a literal string.
	Label string

	// URL this item links to. Ignored when Section is true.
	URL string

	// Icon is a Material Icons name, matching Module.Icon's own
	// convention. Ignored when Section is true.
	Icon string

	// Section marks this entry as a collapsible group label rather
	// than a link -- matching NavItem.Section's own existing meaning
	// in internal/ui, carried over unchanged.
	Section bool

	// RequiredPerm, optional, gates this specific item within an
	// already-visible module -- e.g. a Viewer can see the Assets
	// module but not its Rules or FSM Editor sub-items. Empty means
	// always visible within the module, the same "empty means no
	// gate" convention HideFromSelector's own default already uses.
	// A permission string, matching auth.Permission's own values
	// (declared as plain string here, not auth.Permission itself, for
	// the same dependency-direction reason SidebarItem as a whole is
	// dependency-free of internal/ui and its own imports).
	RequiredPerm string
}

// Registry holds the ordered list of registered modules -- the single
// source of truth every other part of the system (selector, route
// mounting, entitlement checks) reads from, per MODULE_SPEC.md §1.3's
// "registry driven" note: a static, compile-time list, not a dynamic
// plugin system.
type Registry struct {
	modules []Module
	seen    map[string]bool
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{seen: make(map[string]bool)}
}

// Register adds m to the registry. Panics on a duplicate ID -- this is a
// programming error caught at startup (every registration happens during
// bootstrap, MODULE_SPEC.md §3.2), not a runtime condition to handle
// gracefully; two modules silently sharing one ID would corrupt
// entitlement derivation (MODULE_SPEC.md §2.4) for both of them.
func (r *Registry) Register(m Module) {
	if r.seen == nil {
		r.seen = make(map[string]bool)
	}
	if r.seen[m.ID] {
		panic("modules: duplicate module ID " + m.ID)
	}
	r.seen[m.ID] = true
	r.modules = append(r.modules, m)
}

// All returns every registered module, in registration order, regardless
// of access -- for callers that need the full list (e.g. an admin view
// of "every module that exists"), not the viewer-filtered list
// VisibleTo returns.
func (r *Registry) All() []Module {
	return r.modules
}

// VisibleTo returns the modules role has access to, in registration
// order, via each module's own HasAccess check -- the selector's own
// data source (§4.2), with no separate selector-specific model to keep
// in sync.
//
// VisibleFor is the same for a viewer whose tenant is known.
func (r *Registry) VisibleFor(tenant int64, role string) []Module {
	var out []Module
	for _, m := range r.modules {
		switch {
		case m.HasAccessFor != nil:
			if m.HasAccessFor(tenant, role) {
				out = append(out, m)
			}
		case m.HasAccess != nil && m.HasAccess(role):
			out = append(out, m)
		}
	}
	return out
}

// Home returns where "/" sends a viewer on an instance that runs without the
// Assets module, whose dashboard "/" otherwise is: the first module the
// viewer has that is Primary, or failing that the first one with a path of
// its own. It returns "" when the viewer has no such module.
func (r *Registry) Home(tenant int64, role string) string {
	visible := r.VisibleFor(tenant, role)
	for _, m := range visible {
		if m.Primary && m.Path != "" && m.Path != "/" {
			return m.Path
		}
	}
	for _, m := range visible {
		if m.Path != "" && m.Path != "/" {
			return m.Path
		}
	}
	return ""
}

// VisibleTo returns the modules role has access to when no tenant is known.
func (r *Registry) VisibleTo(role string) []Module {
	var out []Module
	for _, m := range r.modules {
		if m.HasAccess != nil && m.HasAccess(role) {
			out = append(out, m)
		}
	}
	return out
}

// MountAll calls MountRoutes on every registered module with a non-nil
// one, in registration order.
func (r *Registry) MountAll(mux *http.ServeMux) {
	for _, m := range r.modules {
		if m.MountRoutes != nil {
			m.MountRoutes(mux)
		}
	}
}
