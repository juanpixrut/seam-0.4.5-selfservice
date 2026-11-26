// Package documents provides the Documents module's registry.Module.
// MountRoutes closure (MODULE_SPEC.md §7, MOD-7). Same thin-wrapper
// shape as internal/modules/{system,assets}, at the opposite end of the
// size spectrum: one route, not 13 or 98. Documents proves the registry
// pattern works for a module that is genuinely almost nothing -- the
// point isn't the size, it's that the same pattern holds at both ends
// (MODULE_SPEC.md §7.3).
package documents

import (
	"net/http"

	"github.com/ha1tch/seam-ui/internal/ui"
)

// MountRoutes returns a modules.Module.MountRoutes-compatible closure
// that registers the Documents module's one route (the dashboard
// landing page) via router's existing handler and middleware, unchanged
// -- only the registration call site is this package's own.
func MountRoutes(router *ui.Router) func(mux *http.ServeMux) {
	return func(mux *http.ServeMux) {
		router.MountDocumentsRoutes(mux)
	}
}
