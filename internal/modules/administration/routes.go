// Package administration provides the Administration module's
// registry.Module.MountRoutes closure -- user/role management. Split
// out 2026-08-04 from the former internal/modules/system, per
// The tech team's decision that Account, Settings, and Administration are
// three separate modules -- see ui.Router.MountAdministrationRoutes'
// own doc comment for why. Deliberately thin, matching every other
// modules/* package's shape. This is what T-02 (docs/TRACKING.md) was
// asking for; T-02 is closed by this work, not left open alongside it.
package administration

import (
	"net/http"

	"github.com/ha1tch/seam-ui/internal/ui"
)

// MountRoutes returns a modules.Module.MountRoutes-compatible closure
// that registers the Administration module's routes via router's
// existing handlers and middleware, unchanged -- only the registration
// call site is this package's own.
func MountRoutes(router *ui.Router) func(mux *http.ServeMux) {
	return func(mux *http.ServeMux) {
		router.MountAdministrationRoutes(mux)
	}
}
