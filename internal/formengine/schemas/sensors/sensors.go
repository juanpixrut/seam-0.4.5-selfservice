// Package sensors embeds the formengine JSON Schema for binding a
// sensor to an asset (T-19, 2026-08-04 -- the first real consumer of
// the asset-picker widget beyond parent_id, proving it's genuinely
// generic rather than accidentally asset-specific). Sensors themselves
// remain hand-coded (SensorFormPage, internal/ui/pages.go) -- this
// package covers only the one, focused bind action, not a full
// migration of the sensor form to formengine.
package sensors

import "embed"

//go:embed bind.json
var schemaFS embed.FS

// Schema returns the raw JSON Schema bytes for this package's one
// form-shaped view: "bind". Panics on an unknown name -- a single,
// compile-time-known file, not user input.
func Schema(name string) []byte {
	data, err := schemaFS.ReadFile(name + ".json")
	if err != nil {
		panic("sensors: unknown schema " + name + ": " + err.Error())
	}
	return data
}
