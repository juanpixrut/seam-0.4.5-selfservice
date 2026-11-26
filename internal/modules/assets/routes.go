// Package assets provides the Assets module's registry.Module.MountRoutes
// closure (MODULE_SPEC.md §6, MOD-6). Deliberately thin, mirroring
// internal/modules/system's own shape exactly: the actual route-
// registration logic -- which needs internal/ui.Router's own private
// requireAuth/requirePerm middleware and handler methods -- stays inside
// internal/ui as the exported Router.MountAssetRoutes method (see its own
// doc comment for the full reasoning). This package exists only to give
// internal/server/server.go a modules.Module-shaped value to register.
package assets

import (
	"net/http"

	"github.com/ha1tch/seam-ui/internal/ui"
)

// MountRoutes returns a modules.Module.MountRoutes-compatible closure that
// registers the Assets module's routes (dashboard, assets, events,
// sensors, alerts, map, FSM editor, rules, asset types, master tables) via
// router's existing handlers and middleware, unchanged -- only the
// registration call site is this package's own.
func MountRoutes(router *ui.Router) func(mux *http.ServeMux) {
	return func(mux *http.ServeMux) {
		router.MountAssetRoutes(mux)
	}
}
