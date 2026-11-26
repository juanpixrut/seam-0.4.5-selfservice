// Package issues provides the registry.Module.MountRoutes closure of
// the Issues module (docs/ISSUES_MODULE.md): an application module gated
// the way Work Orders is, behind a start-time flag (EnableIssues, default
// false) and a switch for one tenant. ui.Router.MountIssuesRoutes itself
// checks the flag and mounts nothing when it is off, so this closure is
// always safe to call.
package issues

import (
	"net/http"

	"github.com/ha1tch/seam-ui/internal/ui"
)

// MountRoutes returns a modules.Module.MountRoutes-compatible closure
// that registers the Issues module's routes via router's
// existing handlers and middleware, unchanged.
func MountRoutes(router *ui.Router) func(mux *http.ServeMux) {
	return func(mux *http.ServeMux) {
		router.MountIssuesRoutes(mux)

		// The anonymous QR channel (POST /s/{code}/report and
		// GET /s/{code}/done) is mounted here rather than in the base
		// router because it is public: no requireAuth, and the base
		// router has no notion of the issue QR codes.
		router.MountIssueQRReportRoute(mux)
	}
}
