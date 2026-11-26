package ui

// This file translates between internal/formengine/schemas/asset's
// English field names and the Spanish attribute keys the real static
// form (asset_detail.go) already reads/writes and xolu already has
// persisted. Needed because the tech team's decision (2026-07-28) is that
// code identifiers -- including the dynamic schema's own field names
// -- stay English, while bilingual UI support is entirely the i18n
// layer's job (§4, T-13), not something field names should carry. The
// static form isn't being touched as part of that decision (it's
// getting retired once FE-24 proves parity, not rewritten piecemeal),
// so during coexistence both forms need to agree on what actually gets
// persisted -- this table is that agreement.
//
// The Spanish side of this table is not invented here: it's the exact
// field list confirmed earlier this session by reading every
// mi.Name(...) call in asset_detail.go directly, the same audit that
// caught the name/type_id bug FE-22 fixed. Two fields (name, type_id)
// are already English on both sides for that same reason -- they're
// listed in the map anyway, mapping to themselves, so callers never
// need a special case for "is this field translated or not."

// assetFieldToXolu maps this package's English schema field name to
// the Spanish (or already-English) attribute key xolu actually stores
// it under. The reverse map (assetFieldFromXolu) is built from this at
// init time -- one table, not two hand-maintained copies that could
// drift apart.
var assetFieldToXolu = map[string]string{
	"name":                  "name",
	"type_id":               "type_id",
	"status":                "status",
	"criticality":           "criticality",
	"connectivity_tier":     "connectivity_tier",
	"giai":                  "giai",
	"grai":                  "grai",
	"barcode_rfid":          "codigo_barras_rfid",
	"technical_tag":         "tag_tecnico",
	"brand_id":              "brand_id",
	"model_id":              "model_id",
	"serial_1":              "serial_1",
	"serial_2":              "serial_2",
	"lot":                   "lot",
	"location_id":           "location_id",
	"region_id":             "region_id",
	"latitude":              "latitude",
	"gps_location":          "gps_location",
	"longitude":             "longitude",
	"parent_id":             "parent_id",
	"custodian_id":          "custodian_id",
	"tenure_type":           "tenure_type",
	"loan_expiry":           "loan_expiry",
	"loan_links":            "loan_links",
	"purchase_date":         "purchase_date",
	"expected_arrival_date": "expected_arrival_date",
	"arrival_date":          "arrival_date",
	"obsolescence_date":     "obsolescence_date",
	"expiration_date":       "expiration_date",
	"decommission_date":     "decommission_date",
	"decommission_reason":   "decommission_reason",
	"lifecycle_links":       "lifecycle_links",
	"purchase_price":        "purchase_price",
	"currency_id":           "currency_id",
	"current_price":         "current_price",
	"cost_center_id":        "cost_center_id",
	"warranty_end":          "warranty_end",
	"document_links":        "document_links",
	"temp_min":              "temp_min",
	"temp_max":              "temp_max",
	"refrigerant":           "refrigerant",
	"voltage_consumption":   "voltage_consumption",
	"service_configs":       "service_configs",
}

// assetFieldFromXolu is the reverse of assetFieldToXolu, built once at
// init time. Guarded for collisions: two English names must never map
// to the same Spanish key (that would make the reverse direction
// ambiguous), so init panics immediately if the table is ever edited
// into an inconsistent state, rather than silently dropping data at
// runtime.
var assetFieldFromXolu = func() map[string]string {
	rev := make(map[string]string, len(assetFieldToXolu))
	for en, es := range assetFieldToXolu {
		if existing, ok := rev[es]; ok {
			panic("asset_xolu_fields: xolu key \"" + es + "\" is mapped from both \"" +
				existing + "\" and \"" + en + "\" -- assetFieldToXolu must be one-to-one")
		}
		rev[es] = en
	}
	return rev
}()

// documentLinksItemToXolu / documentLinksItemFromXolu are identity
// mappings -- xolu's own item-level storage keys for loan_links,
// lifecycle_links, and document_links (name/url/type) now match the
// schema's English field names exactly, same identity-pass-through
// state every other completed entity's bridge reaches.
var documentLinksItemToXolu = map[string]string{
	"name": "name",
	"url":  "url",
	"type": "type",
}

var documentLinksItemFromXolu = map[string]string{
	"name": "name",
	"url":  "url",
	"type": "type",
}

// serviceConfigsItemToXolu / serviceConfigsItemFromXolu are identity
// mappings -- xolu's own item-level storage keys (measure/threshold/
// alert_id) now match the schema's English field names exactly, same
// identity-pass-through state every other completed entity's bridge
// reaches. Was {measure, threshold, alert_id} <-> {medida,
// umbral_alarma, alerta_id} until T-40 (this session) renamed the
// underlying columns; the real bug this table's own history records
// (2026-07-28, threshold's submitted field name and its persisted key
// genuinely differing) is now moot for the same reason -- both sides
// are the same string.
var serviceConfigsItemToXolu = map[string]string{
	"measure":   "measure",
	"threshold": "threshold",
	"alert_id":  "alert_id",
}

var serviceConfigsItemFromXolu = map[string]string{
	"measure":   "measure",
	"threshold": "threshold",
	"alert_id":  "alert_id",
}

// service_configs' "measure" needed a Spanish<->English value bridge
// briefly (T-33, 0.3.27) but no longer does: T-36 (2026-08-02) widened
// it to a real two-level distance/time/usage/condition classification
// and, per the same reasoning already applied to entity names (T-40),
// made the canonical value itself the stored value directly -- no
// bridge, since every demo is freshly re-seeded, not migrated. The
// enumToXolu/enumFromXolu fields below stay in the struct (still
// meaningful machinery, just unused today) rather than being removed,
// since a future array-typed field with a genuine Spanish/English
// split would need exactly this shape again.

// assetArrayFieldItems names, for each of the four array-typed schema
// fields, which item-level translation tables apply -- so the
// top-level translation functions don't need a hardcoded switch on
// field name to know how to recurse into a row. enumToXolu/enumFromXolu
// are nil for all four today (service_configs' own "measure" bridge
// was retired 2026-08-02, T-36) -- kept as fields on the struct for
// the next field that needs one, not dead weight to delete.
var assetArrayFieldItems = map[string]struct {
	toXolu       map[string]string
	fromXolu     map[string]string
	enumToXolu   map[string]map[string]string
	enumFromXolu map[string]map[string]string
}{
	"loan_links":      {documentLinksItemToXolu, documentLinksItemFromXolu, nil, nil},
	"lifecycle_links": {documentLinksItemToXolu, documentLinksItemFromXolu, nil, nil},
	"document_links":  {documentLinksItemToXolu, documentLinksItemFromXolu, nil, nil},
	"service_configs": {serviceConfigsItemToXolu, serviceConfigsItemFromXolu, nil, nil},
}

// assetEnumValueToXolu translates specific fields' English enum tokens to
// the Spanish tokens xolu already stores, for the three fields whose
// *values* -- not just their key -- were Spanish-hardcoded in
// internal/validation/schemas.go until the internal/external law fix
// (T-24, 2026-07-31; docs/RESOLVED.md).
//
// Write direction only (applied from AssetValuesToXolu). There is
// deliberately no reverse table used from AssetValuesFromXolu: the
// dynamic Asset form's own schema (asset.json) still validates these
// three fields' values as Spanish (T-23, not yet fixed), so translating
// xolu's stored Spanish values to English on the read side would break
// the dynamic form's own edit-mode value-matching against asset.json's
// still-Spanish enum options. Until T-23 does that side too, the read
// direction must keep seeing the same Spanish values it always has.
//
// This asymmetry is also why the write side is safe for the dynamic
// form's *current* traffic, which still submits Spanish values under
// these English keys (pre-T-23): a Spanish value like "Alta" simply
// isn't a key in any of these sub-tables, so translateAssetEnumValue
// passes it through unchanged -- already the correct xolu value, no
// regression.
var assetEnumValueToXolu = map[string]map[string]string{
	"criticality": {
		"High":   "Alta",
		"Medium": "Media",
		"Low":    "Baja",
	},
	"tenure_type": {
		"Owned":    "Propiedad",
		"Loaned":   "Comodato",
		"Borrowed": "Préstamo",
		"Rented":   "Alquiler",
	},
	"decommission_reason": {
		"Sale":     "Venta",
		"Scrap":    "Desguace",
		"Theft":    "Robo",
		"Donation": "Donación",
		"Other":    "Otros",
	},
}

// translateAssetEnumValue applies assetEnumValueToXolu (write) or
// assetEnumValueFromXolu (read) for the fields either table covers.
// Both directions are safe now that T-23 (docs/RESOLVED.md) makes
// asset.json's own criticality/tenure_type/decommission_reason enums
// English too -- until T-23, the read direction was deliberately a
// no-op here (see git history / T-24's original resolution note),
// because translating xolu's stored Spanish values to English would
// have broken the dynamic form's edit-mode value-matching against
// asset.json's then-still-Spanish enum options. Now both sides agree.
func translateAssetEnumValue(schemaFieldName string, val interface{}, toXolu bool) interface{} {
	table := assetEnumValueToXolu[schemaFieldName]
	if !toXolu {
		table = assetEnumValueFromXolu[schemaFieldName]
	}
	if table == nil {
		return val
	}
	s, ok := val.(string)
	if !ok {
		return val
	}
	if translated, ok := table[s]; ok {
		return translated
	}
	return val // not a recognised token for this direction -- pass through
}

// assetEnumValueFromXolu is the reverse of assetEnumValueToXolu, built once
// at init the same one-source-of-truth way as assetFieldFromXolu. Used by
// translateAssetEnumValue's read direction (AssetValuesFromXolu,
// populating the dynamic form's edit mode) -- the static Asset form's own
// POST handlers, which this table was originally also built to serve
// (internal/ui/routes.go's AssetCreate/AssetUpdate, retired 2026-07-31 --
// see /attic/static-asset-form/), are gone; this table's remaining use is
// entirely the dynamic form's read path.
var assetEnumValueFromXolu = func() map[string]map[string]string {
	rev := make(map[string]map[string]string, len(assetEnumValueToXolu))
	for field, table := range assetEnumValueToXolu {
		r := make(map[string]string, len(table))
		for en, es := range table {
			r[es] = en
		}
		rev[field] = r
	}
	return rev
}()

// AssetValuesToXolu translates a map keyed by this package's English
// schema field names (as formengine.Render's values map, or a parsed
// POST body, would produce) into the Spanish-keyed shape xolu.Create/
// xolu.Update expect, matching what the static form already persists.
// Unrecognised keys pass through unchanged rather than being dropped,
// so a caller extending the schema doesn't lose data silently before
// this table is updated to match -- but every field this schema
// currently defines IS recognised, so that fallback should never
// actually trigger for well-formed input.
func AssetValuesToXolu(schemaValues map[string]interface{}) map[string]interface{} {
	return translateAssetValues(schemaValues, assetFieldToXolu, true)
}

// AssetValuesFromXolu is AssetValuesToXolu's inverse: translates a
// Spanish-keyed xolu entity (as xolu.Get returns) into this schema's
// English-keyed shape, for feeding into formengine.Render as the
// initial values of an edit form.
func AssetValuesFromXolu(xoluValues map[string]interface{}) map[string]interface{} {
	return translateAssetValues(xoluValues, assetFieldFromXolu, false)
}

func translateAssetValues(values map[string]interface{}, fieldMap map[string]string, toXolu bool) map[string]interface{} {
	out := make(map[string]interface{}, len(values))
	for key, val := range values {
		newKey, ok := fieldMap[key]
		if !ok {
			// Unrecognised field -- pass through under its original key
			// rather than drop it silently.
			out[key] = translateAssetArrayValue(key, val, toXolu)
			continue
		}
		schemaFieldName := pickSchemaFieldName(key, newKey, toXolu)
		v := translateAssetArrayValue(schemaFieldName, val, toXolu)
		v = translateAssetEnumValue(schemaFieldName, v, toXolu)
		out[newKey] = v
	}
	return out
}

// pickSchemaFieldName returns whichever of the two keys is the
// English (schema-side) one, since that's what assetArrayFieldItems is
// keyed by regardless of translation direction.
func pickSchemaFieldName(oldKey, newKey string, toXolu bool) string {
	if toXolu {
		return oldKey // input was already English
	}
	return newKey // input was Spanish; newKey is the translated English name
}

// translateAssetArrayValue recurses into the four array-typed fields'
// rows, translating each row's item keys. Any other value (scalar,
// nil, or an array field this function doesn't recognise) passes
// through unchanged.
func translateAssetArrayValue(schemaFieldName string, val interface{}, toXolu bool) interface{} {
	tables, ok := assetArrayFieldItems[schemaFieldName]
	if !ok {
		return val
	}
	items, ok := val.([]interface{})
	if !ok {
		return val
	}
	itemMap := tables.toXolu
	enumMap := tables.enumToXolu
	if !toXolu {
		itemMap = tables.fromXolu
		enumMap = tables.enumFromXolu
	}
	out := make([]interface{}, len(items))
	for i, item := range items {
		row, ok := item.(map[string]interface{})
		if !ok {
			out[i] = item
			continue
		}
		newRow := make(map[string]interface{}, len(row))
		for k, v := range row {
			newK, hasNewKey := itemMap[k]
			if !hasNewKey {
				newRow[k] = v
				continue
			}
			// Enum tables are keyed by the schema-side (English) item
			// field name, same convention pickSchemaFieldName already
			// establishes for the top-level fields: toXolu direction
			// reads before translation (already English), fromXolu
			// direction reads after (newK is the translated English name).
			enumFieldName := k
			if !toXolu {
				enumFieldName = newK
			}
			newV := v
			if enumMap != nil {
				if table, ok := enumMap[enumFieldName]; ok {
					if s, ok := v.(string); ok {
						if translated, ok := table[s]; ok {
							newV = translated
						}
					}
				}
			}
			newRow[newK] = newV
		}
		out[i] = newRow
	}
	return out
}
