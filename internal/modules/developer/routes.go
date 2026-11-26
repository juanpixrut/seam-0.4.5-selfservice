// Package developer provides the Developer module's registry.Module.MountRoutes
// closure (campaign DEV-7). The module gives development teams their tooling:
// translation and form-definition editing, hot loading, and the feature-flag
// control panel. Opt-in: router.MountDeveloperRoutes no-ops unless
// EnableDeveloper is true, and every route requires the developer role.
// Deliberately thin, like every other modules/* package.
package developer

import (
	"net/http"

	"github.com/ha1tch/seam-ui/internal/ui"
)

// MountRoutes returns a modules.Module.MountRoutes-compatible closure that
// registers the Developer module's routes via router's existing handlers
// and middleware. Always safe to register: the flag check is inside
// router.MountDeveloperRoutes.
func MountRoutes(router *ui.Router) func(mux *http.ServeMux) {
	return func(mux *http.ServeMux) {
		router.MountDeveloperRoutes(mux)
	}
}
