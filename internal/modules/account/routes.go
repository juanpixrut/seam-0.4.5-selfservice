// Package account provides the Account module's registry.Module.
// MountRoutes closure -- User Profile, the one module that must be
// visible to every authenticated user regardless of role (see
// ui.Router.MountAccountRoutes' own doc comment for why this is a
// separate module from Settings/Administration, not bundled with
// either). Split out 2026-08-04 from the former internal/modules/system,
// per the tech team's decision that Account, Settings, and Administration are
// three separate modules. Deliberately thin, matching every other
// modules/* package's shape: the actual route-registration logic stays
// inside internal/ui, where the private auth middleware and handlers
// already live.
package account

import (
	"net/http"

	"github.com/ha1tch/seam-ui/internal/ui"
)

// MountRoutes returns a modules.Module.MountRoutes-compatible closure
// that registers the Account module's routes via router's existing
// handlers and middleware, unchanged -- only the registration call
// site is this package's own.
func MountRoutes(router *ui.Router) func(mux *http.ServeMux) {
	return func(mux *http.ServeMux) {
		router.MountAccountRoutes(mux)
	}
}
