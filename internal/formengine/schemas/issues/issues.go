// Package issues embeds the formengine JSON Schema for the Issue
// Tracker module (docs/ISSUES_MODULE.md), which is gated the way Work
// Orders is. One schema: creating an
// issue, optionally linked to an asset via the asset-picker widget
// (T-72's own generic design, this its third real consumer after
// parent_id and sensor binding).
package issues

import "embed"

//go:embed create.json assign.json
var schemaFS embed.FS

// Schema returns the raw JSON Schema bytes for one of this package's
// form-shaped views: "create", and "assign" (the single-field form of the
// assign page, an any-picker over the users entity with its Browse dialog).
// Panics on an unknown name -- compile-time-known files, not user input.
func Schema(name string) []byte {
	data, err := schemaFS.ReadFile(name + ".json")
	if err != nil {
		panic("issues: unknown schema " + name + ": " + err.Error())
	}
	return data
}
