// Package settings provides the Settings module's registry.Module.
// MountRoutes closure -- tenant configuration (general/notifications/
// features). Split out 2026-08-04 from the former internal/modules/
// system, per the tech team's decision that Account, Settings, and
// Administration are three separate modules -- see ui.Router.
// MountSettingsRoutes' own doc comment for why. Deliberately thin,
// matching every other modules/* package's shape.
package settings

import (
	"net/http"

	"github.com/ha1tch/seam-ui/internal/ui"
)

// MountRoutes returns a modules.Module.MountRoutes-compatible closure
// that registers the Settings module's routes via router's existing
// handlers and middleware, unchanged -- only the registration call
// site is this package's own.
func MountRoutes(router *ui.Router) func(mux *http.ServeMux) {
	return func(mux *http.ServeMux) {
		router.MountSettingsRoutes(mux)
	}
}
