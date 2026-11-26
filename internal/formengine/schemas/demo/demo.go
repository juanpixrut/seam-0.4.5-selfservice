// Package demo embeds the formengine JSON Schemas used by the Demo module
// (/demo, opt-in via EnableDemo): "components" (any-picker with and without
// a Selector), "repeatable" (the corrected shape for nested repeatable
// fields) and "repeatable_bug" (the shape reported by the Work Orders
// developers, shown as text only and never rendered by the engine).
// The four later tabs each have one schema: "layouts" (row and form layouts,
// nesting), "buttons" (add and remove button variants), "colors" (accent
// colours), "selectors" (pickers over several entities) and "reorder"
// (reorderable repeatables and searchable selects).
// Four "Kitchen" tabs show the remaining widgets of the catalogue, one schema
// each: "kitchen_inputs" (text, numbers, dates, switches), "kitchen_choices"
// (selects, pickers, conditional visibility), "kitchen_field" (GPS, barcodes,
// signature, images) and "kitchen_rows" (the script-driven widgets inside the
// rows of a repeatable). Repeatables and buttons stay in their own tabs.
// Same transitional-bootstrap reasoning as the other schema packages.
package demo

import "embed"

//go:embed components.json repeatable.json repeatable_bug.json layouts.json buttons.json colors.json selectors.json reorder.json kitchen_inputs.json kitchen_choices.json kitchen_field.json kitchen_rows.json query_picker.json
var schemaFS embed.FS

// Schema returns the raw JSON Schema bytes for one of this package's
// schemas: "components", "repeatable", "repeatable_bug", "layouts",
// "buttons", "colors", "selectors", "reorder", "kitchen_inputs",
// "kitchen_choices", "kitchen_field" or "kitchen_rows". Panics on an
// unknown name -- a small, fixed, compile-time-known set, not user input.
func Schema(name string) []byte {
	data, err := schemaFS.ReadFile(name + ".json")
	if err != nil {
		panic("demo: unknown schema " + name + ": " + err.Error())
	}
	return data
}
