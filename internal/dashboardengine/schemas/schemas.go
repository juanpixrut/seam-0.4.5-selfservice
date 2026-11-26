// Package schemas embeds the x-seam-dashboard JSON definitions
// (DASHBOARD_SPEC.md) MOD-2 and later stages ship. Mirrors
// internal/formengine/schemas/masters's own pattern exactly -- a
// transitional bootstrap (that package's own doc comment explains why:
// schema authoring ultimately belongs to the Settings module once T-08
// lands, not files embedded in the Go binary). Same shortcut, same
// reasoning, applied here rather than reinvented.
package schemas

import "embed"

//go:embed assets-dashboard.json documents-dashboard.json
var schemaFS embed.FS

// Assets returns the Assets dashboard's JSON definition (DASHBOARD_SPEC.md
// §8). Panics on read failure -- the file is embedded at build time, so a
// failure here means the binary itself is broken, not a runtime condition
// to recover from (same reasoning any other embed.FS panic-on-missing-file
// pattern in this codebase already uses).
func Assets() []byte {
	b, err := schemaFS.ReadFile("assets-dashboard.json")
	if err != nil {
		panic("dashboardengine/schemas: assets-dashboard.json: " + err.Error())
	}
	return b
}

// Documents returns the Documents dashboard's JSON definition
// (DASHBOARD_SPEC.md §2.2, MOD-7) -- "sections": [], the empty-dashboard
// case: Documents has no widgets defined at all yet (DOCUMENTS_MODULE_
// SPEC.md v0.5 is a design document, T-04 to build it is still open).
// Same panic-on-read-failure reasoning as Assets.
func Documents() []byte {
	b, err := schemaFS.ReadFile("documents-dashboard.json")
	if err != nil {
		panic("dashboardengine/schemas: documents-dashboard.json: " + err.Error())
	}
	return b
}
