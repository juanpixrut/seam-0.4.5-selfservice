// Package workorders provides the Work Orders playground module's
// registry.Module.MountRoutes closure (2026-08-04, T-67's follow-on).
// A real, working slice proving form + FSM + xolu commit composition
// -- not T-11's own scoped production module, which remains entirely
// separate, unbuilt work. Deliberately thin, matching every other
// modules/* package's shape.
package workorders

import (
	"net/http"

	"github.com/ha1tch/seam-ui/internal/ui"
)

// MountRoutes returns a modules.Module.MountRoutes-compatible closure
// that registers the Work Orders module's routes via router's existing
// handlers and middleware, unchanged -- only the registration call
// site is this package's own. router.MountWorkOrdersRoutes itself
// checks EnableWorkOrders and no-ops if it's false, so this closure is
// always safe to register regardless of whether the flag is on.
func MountRoutes(router *ui.Router) func(mux *http.ServeMux) {
	return func(mux *http.ServeMux) {
		router.MountWorkOrdersRoutes(mux)
	}
}
