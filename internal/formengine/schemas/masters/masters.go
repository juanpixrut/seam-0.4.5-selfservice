// Package masters embeds the stage-2 flat JSON Schemas for the three
// master entities converted first (Región, Moneda, Marca), per
// docs/FORMENGINE_DEVELOPMENT_PLAN.md stage 2.
//
// This is a transitional bootstrap, not the intended long-term schema
// source. FORM_ENGINE_SPEC.md §13.4: schema authoring belongs to the
// Settings module's schema designer (T-08), and schemas ultimately live
// in xolu (POST /api/v1/schema/{entity} — verified working end to end by
// the dormant guard in engine_integration_test.go), not as files embedded
// in the Go binary. Embedding is a deliberate shortcut for stage 2, so
// the rendering engine can be proven against real handlers before the
// Settings module's schema lifecycle exists to author and persist these
// properly. Migrate call sites to loading from xolu once T-08 lands.
package masters

import "embed"

//go:embed region.json moneda.json marca.json
var schemaFS embed.FS

// Schema returns the raw JSON Schema bytes for one of the three stage-2
// master entities: "region", "moneda", or "marca". Panics on an unknown
// name — this is a small, fixed, compile-time-known set, not user input.
func Schema(name string) []byte {
	data, err := schemaFS.ReadFile(name + ".json")
	if err != nil {
		panic("masters: unknown schema " + name + ": " + err.Error())
	}
	return data
}
