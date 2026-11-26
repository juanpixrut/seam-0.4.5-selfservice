// Package demo provides the Demo module's registry.Module.MountRoutes
// closure. The Demo module shows developers new components and fixes
// (Selector, any-picker, the repeatable-rows fix) on real pages rendered by
// the real form engine. Opt-in: router.MountDemoRoutes no-ops unless
// EnableDemo is true. Deliberately thin, like every other modules/* package.
package demo

import (
	"net/http"

	"github.com/ha1tch/seam-ui/internal/ui"
)

// MountRoutes returns a modules.Module.MountRoutes-compatible closure that
// registers the Demo module's routes via router's existing handlers and
// middleware. Always safe to register: the flag check is inside
// router.MountDemoRoutes.
func MountRoutes(router *ui.Router) func(mux *http.ServeMux) {
	return func(mux *http.ServeMux) {
		router.MountDemoRoutes(mux)
	}
}
