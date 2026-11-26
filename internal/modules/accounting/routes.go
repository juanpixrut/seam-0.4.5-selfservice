// Package accounting provides the Accounting & Admin module's
// modules.Module.MountRoutes closure (Seam Admin: Account Executives,
// collections, commissions, settlements and the books). Deliberately thin,
// like every other modules/* package: the routes live in
// ui.Router.MountAccountingRoutes.
package accounting

import (
	"net/http"

	"github.com/ha1tch/seam-ui/internal/ui"
)

// MountRoutes returns the module's route-registration closure.
func MountRoutes(router *ui.Router) func(mux *http.ServeMux) {
	return func(mux *http.ServeMux) {
		router.MountAccountingRoutes(mux)
	}
}
