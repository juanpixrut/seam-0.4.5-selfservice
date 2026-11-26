// Package asset embeds the stage-7 Asset schema — the form engine's
// proof case against the real, complex, currently-shipping static form
// (internal/ui/asset_detail.go), per docs/FORMENGINE_DEVELOPMENT_PLAN.md
// stage 7.
//
// Extracted from FORM_ENGINE_SPEC.md §12's example fragment, then
// verified field by field against the real static form rather than
// trusted as-is — three real inaccuracies were found and fixed, not
// just the two originally caught:
//
//   - §12's status enum ("active"/"silent"/"alert"/"lost") and
//     connectivity_tier enum ("qr"/"gateway"/"lora"/"gps") don't match
//     the actual values internal/ui/routes.go hardcodes and persists
//     ("active"/"inactive"/"maintenance"/"lost" and
//     "qr_only"/"gateway"/"lora_mesh"/"gps_cellular").
//   - §12 used "nombre" and "tipo_id" as the property names for the
//     asset's name and type — but the real static form's actual HTML
//     field names for exactly these two fields are "name" and
//     "type_id" (English), confirmed against every mi.Name(...) call
//     in asset_detail.go, not assumed from the otherwise-consistent
//     Spanish naming the rest of the form uses. Found while starting
//     FE-23 (handler conversion): had this gone uncaught, a dynamic
//     form POSTing "nombre"/"tipo_id" against the real AssetCreate
//     handler (which reads r.FormValue("name")/r.FormValue("type_id"))
//     would have silently submitted every asset with an empty name.
//   - vto_comodato's x-seam-visible-when was missing from §12's
//     fragment despite being §3b.2's own worked example for this exact
//     field — added here.
//
// FORM_ENGINE_SPEC.md §12 itself was corrected for all three (dated
// note at the section heading) rather than left to drift, since by this
// point it was clear §12 wasn't a one-off approximation but had
// accumulated enough real inaccuracies to actively mislead the next
// person extracting from it.
//
// One further known discrepancy carried forward deliberately, not
// silently: the real form's region_id label reads "Región *" but
// nothing in assetFKSelect actually enforces it as required (no
// `required` HTML attribute, empty option always present) — the
// asterisk appears to be a pre-existing cosmetic bug in the static
// form, not a real constraint. This schema reproduces that exact
// behaviour (title carries "*", region_id is absent from `required`)
// for faithful parity rather than silently "fixing" it mid-migration,
// which would undermine FE-24's regression comparison. Deliberately
// deferred to T-13 (i18n wiring), not decided here: once all 42 field
// titles resolve through real i18n keys instead of the literal Spanish
// text this schema currently uses, how "required" gets indicated needs
// one consistent design across every field, not a one-off guess at
// region_id in isolation (the tech team, 2026-07-28).
//
// All 42 property names are English (the tech team, 2026-07-28): code
// identifiers -- including schema field names -- stay in English
// throughout; bilingual (Spanish/English) UI support is entirely the
// i18n layer's job (§4, T-13), not something field names should carry.
// The real static form, and what xolu already has persisted, still use
// the original Spanish attribute keys (region_id, calidad_tenencia,
// etc.) -- FE-23's persistence code translates between this schema's
// English names and those Spanish keys at the xolu Create/Update/Get
// boundary specifically, so the coexisting static form keeps reading
// and writing the same underlying data. That translation table isn't
// this package's concern; it lives in FE-23's handler code.
//
// Same transitional-bootstrap caveat as internal/formengine/schemas/masters:
// this is not the long-term schema source (FORM_ENGINE_SPEC.md §13.4 —
// that's the Settings module's schema designer, T-08, with schemas
// ultimately persisted in xolu). Embedding lets the engine be proven
// against the real Asset form before that lifecycle exists.
package asset

import (
	"embed"
	"encoding/json"
	"fmt"
)

//go:embed asset.json
var schemaFS embed.FS

// Schema returns the raw JSON Schema bytes for the Asset form.
func Schema() []byte {
	data, err := schemaFS.ReadFile("asset.json")
	if err != nil {
		panic("asset: reading asset.json: " + err.Error())
	}
	return data
}

// SchemaForUpdate returns the Asset schema with its top-level
// "required" array emptied, for validation use on update only --
// mirrors the static form's own AssetCreateSchema/AssetUpdateSchema
// split (name/type_id/criticality required on create, optional on
// update), confirmed from internal/validation/schemas.go directly
// before building this, not assumed (the tech team, 2026-07-29: "Build a
// separate update-specific schema variant, mirroring the static
// form's own create/update split").
//
// Rendering still uses Schema() unmodified -- the visual "*"
// indicator (driven by the schema's required array, per FE-24's
// earlier fix) stays the same on both create and edit. This matches
// how the static form's own UI already behaves: region_id carries a
// "*" in its title while never being HTML-required, a pre-existing,
// deliberately-preserved discrepancy in this same package -- the
// asterisk reflects a field's conceptual importance, not literal
// backend enforcement at that exact moment. Only the compiled schema
// used for POST validation differs between create and update; the
// rendered form looks identical either way.
func SchemaForUpdate() []byte {
	data, err := SchemaForUpdateOf(Schema())
	if err != nil {
		panic("asset: " + err.Error())
	}
	return data
}

// SchemaForUpdateOf is the update variant of any asset definition: the same
// document with nothing required. The update handler uses it on the
// definition in force for the request, so an override of the asset form
// (Developer module) governs what an update accepts as it governs what the
// form shows; SchemaForUpdate is the same for the built-in definition.
func SchemaForUpdateOf(schema []byte) ([]byte, error) {
	var doc map[string]interface{}
	if err := json.Unmarshal(schema, &doc); err != nil {
		return nil, fmt.Errorf("parsing schema for update variant: %w", err)
	}
	doc["required"] = []string{}
	data, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshaling update schema variant: %w", err)
	}
	return data, nil
}
