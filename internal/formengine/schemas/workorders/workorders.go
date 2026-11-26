// Package workorders embeds the formengine JSON Schemas for the Work
// Orders playground module (2026-08-04, T-67's follow-on). A deliberate
// playground for the Work Orders team, not a scoped production module
// -- T-11 (Order Engine/CMMS, docs/TRACKING.md) still has five open
// design questions and no module name settled. This package covers the
// two form-shaped views the playground actually needs: creating a work
// order, and completing one (technician sign-off via the signature
// widget built the same session, T-10).
//
// Same transitional-bootstrap reasoning as internal/formengine/schemas/
// system's own doc comment: embedding is the deliberate shortcut until
// a real schema designer (T-08) exists, not a new pattern invented here.
package workorders

import "embed"

//go:embed create.json create-selections.json complete.json template-picker.json direct-assignment.json
var schemaFS embed.FS

// Schema returns the raw JSON Schema bytes for one of this module's
// form-shaped views: "create" or "complete". Panics on an unknown name
// -- a small, fixed, compile-time-known set, not user input.
func Schema(name string) []byte {
	data, err := schemaFS.ReadFile(name + ".json")
	if err != nil {
		panic("workorders: unknown schema " + name + ": " + err.Error())
	}
	return data
}
