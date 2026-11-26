package ui

// This file translates between the Custodio/Alerta/PropDefinition/
// PropAssoc schemas' English field names and values (internal/validation/
// schemas.go, English per the internal/external language law,
// docs/KNOWN_ISSUES.md) and the Spanish attribute keys/values xolu
// actually stores them under -- the same pattern asset_xolu_fields.go
// established for Asset (T-24, docs/RESOLVED.md), applied to Part 2's
// four master-table schemas. Write direction only, same reason as
// asset_xolu_fields.go: none of these entities have display-boundary
// translation yet (that's the same widget-dispatch-locale fork recorded
// against T-22/T-23), so the read/edit-form side must keep seeing the
// same Spanish values it always has.
//
// Unlike Asset, none of these four entities have array-typed fields, so
// the shared helpers below don't need asset_xolu_fields.go's recursive
// item-translation machinery -- a flat key (and, where relevant, a
// per-field enum value) translation is enough.

// ── Custodio ─────────────────────────────────────────────────────────

// custodioFieldToXolu is empty, not removed: subject_type/legal_name/
// tax_id/phone/address/active are now the xolu storage keys directly
// (T-40, 2026-08-02), matching every prior entity's precedent. alias and
// email were always English on both sides, needing no entry even before
// this. subject_type's own values (DNI/CI/RUT/RUC) remain deliberately
// untranslated -- they're Uruguayan/regional legal document-type codes,
// not translatable prose (there's no English word for "RUT"), per T-24's
// resolution note in docs/RESOLVED.md -- unaffected by this rename since
// it only ever concerned the field name, not its values.
var custodioFieldToXolu = map[string]string{}

// CustodioValuesToXolu translates a map keyed by CustodioCreateSchema's
// English field names into the Spanish-keyed shape xolu.Create/Update
// expect. Unrecognised keys pass through unchanged.
func CustodioValuesToXolu(schemaValues map[string]interface{}) map[string]interface{} {
	return translateFlatFields(schemaValues, custodioFieldToXolu)
}

// ── Alerta ───────────────────────────────────────────────────────────

// alertaFieldToXolu is now identity, not empty: type/description/
// criticality are the xolu storage keys directly (T-40, 2026-08-02),
// matching every prior entity's precedent for the field NAMES. Unlike
// Region/Moneda/Centro de Costo/Marca/Ubicacion/Custodio, this table
// can't be emptied outright -- translateFlatFieldsAndEnums (below) gates
// enum-VALUE translation behind a successful fieldMap lookup, so an
// empty map here would silently stop translating "type"/"criticality"'s
// English values (Maintenance/High/...) to the Spanish values xolu still
// stores (Mantenimiento/Alta/...) via alertaEnumValueToXolu -- caught
// before shipping, not after. The VALUES remaining Spanish is T-23's
// separate, not-yet-fixed concern, not this item's scope.
var alertaFieldToXolu = map[string]string{
	"type":        "type",
	"description": "description",
	"criticality": "criticality",
}

// alertaEnumValueToXolu covers alerta's two enum-valued fields.
// criticality reuses the identical High/Medium/Low <-> Alta/Media/Baja
// mapping asset_xolu_fields.go already established for Asset's own
// criticality field -- the same underlying concept, kept as a separate
// table (not a shared import) matching this codebase's existing
// per-entity i18n-key convention (see asset.status.* vs a hypothetical
// shared status.* group).
var alertaEnumValueToXolu = map[string]map[string]string{
	"type": {
		"Maintenance": "Mantenimiento",
		"Security":    "Seguridad",
		"Operational": "Operativa",
	},
	"criticality": {
		"High":   "Alta",
		"Medium": "Media",
		"Low":    "Baja",
	},
}

var alertaEnumValueFromXolu = reverseEnumTable(alertaEnumValueToXolu)

// AlertaValuesToXolu translates a map keyed by AlertaCreateSchema's
// English field names/values into the Spanish-keyed/valued shape xolu
// stores.
func AlertaValuesToXolu(schemaValues map[string]interface{}) map[string]interface{} {
	return translateFlatFieldsAndEnums(schemaValues, alertaFieldToXolu, alertaEnumValueToXolu)
}

// AlertaEnumValueFromXoluValue translates a single raw Spanish enum value
// for the named field (type or criticality) to English. Used both for
// the master form's POST handlers (raw form submissions, pre-fix, were
// Spanish) and for the read side (list rows, edit-form pre-fill,
// downstream consumers like AssetType's default-alert picker) now that
// T-18 Stage 9's tier-1 pass wires the read direction too -- already-
// English input passes through unchanged (a lookup miss on the
// Spanish-keyed reverse table), so this one function is safe on both
// legacy-Spanish and current-English stored data.
func AlertaEnumValueFromXoluValue(schemaFieldName, val string) string {
	table, ok := alertaEnumValueFromXolu[schemaFieldName]
	if !ok {
		return val
	}
	if en, ok := table[val]; ok {
		return en
	}
	return val
}

// AlertaTypeLabel translates alerta's English canonical "type" value
// (Maintenance/Security/Operational) to a display label via t. Accepts
// legacy-Spanish stored values too (converts first), so callers reading
// raw xolu data don't need to call AlertaEnumValueFromXoluValue
// separately first.
func AlertaTypeLabel(typeVal string, t func(key string, args ...any) string) string {
	switch AlertaEnumValueFromXoluValue("type", typeVal) {
	case "Maintenance":
		return t("alert_type.category.maintenance")
	case "Security":
		return t("alert_type.category.security")
	case "Operational":
		return t("alert_type.category.operational")
	default:
		return typeVal
	}
}

// AlertaCriticalityLabel translates alerta's English canonical
// "criticality" value (High/Medium/Low) to a display label via t. Same
// legacy-Spanish tolerance as AlertaTypeLabel.
func AlertaCriticalityLabel(criticality string, t func(key string, args ...any) string) string {
	switch AlertaEnumValueFromXoluValue("criticality", criticality) {
	case "High":
		return t("alert_type.criticality.high")
	case "Medium":
		return t("alert_type.criticality.medium")
	case "Low":
		return t("alert_type.criticality.low")
	default:
		return criticality
	}
}

// ── PropDefinition ───────────────────────────────────────────────────

// propDefFieldToXolu is an explicit identity map, not truly empty --
// xolu's own storage keys now match the schema's English field names
// exactly (short_name, data_type, default_value), but
// translateFlatFieldsAndEnums (this file) only applies enum-VALUE
// translation when the key is found in this map at all; a truly empty
// map (what Marca's own bridge correctly uses, since Marca has no
// enum-value table) would silently skip data_type's own value
// translation entirely. Found live: a newly-created property's
// data_type was stored as "numeric" instead of "numerico" after
// emptying this to map[string]string{} -- caught by checking the
// actual persisted record, not assumed correct. Matches Alerta's own
// established pattern for exactly this situation (Alerta also has an
// enum-value table, and its own bridge is an identity map, not empty,
// for the same reason).
var propDefFieldToXolu = map[string]string{
	"short_name":    "short_name",
	"data_type":     "data_type",
	"default_value": "default_value",
}

var propDefEnumValueToXolu = map[string]map[string]string{
	"data_type": {
		"numeric":       "numerico",
		"alphanumeric":  "alfanumerico",
		"date":          "fecha",
		"datetime":      "fecha_hora",
		"time":          "hora",
		"boolean":       "booleano",
		"single_select": "seleccion_unica",
		"multi_select":  "seleccion_multiple",
		"system_entity": "entidad_sistema",
		"external_api":  "api_externa",
	},
}

var propDefEnumValueFromXolu = reverseEnumTable(propDefEnumValueToXolu)

// PropDefValuesToXolu translates a map keyed by PropDefinitionCreateSchema's
// English field names/values into the Spanish-keyed/valued shape xolu
// stores.
func PropDefValuesToXolu(schemaValues map[string]interface{}) map[string]interface{} {
	return translateFlatFieldsAndEnums(schemaValues, propDefFieldToXolu, propDefEnumValueToXolu)
}

// PropDefEnumValueFromXoluValue translates a single raw Spanish data_type
// value to English, for the master form's POST handlers and (T-18 Stage
// 9 tier-1) the read side -- list rows and edit-form pre-fill. Already-
// English input passes through unchanged, same reasoning as
// AlertaEnumValueFromXoluValue.
func PropDefEnumValueFromXoluValue(schemaFieldName, val string) string {
	table, ok := propDefEnumValueFromXolu[schemaFieldName]
	if !ok {
		return val
	}
	if en, ok := table[val]; ok {
		return en
	}
	return val
}

// PropDefDataTypeLabel translates PropDef's English canonical data_type
// value to a display label via t. Accepts legacy-Spanish stored values
// too (converts first), same tolerance as AlertaTypeLabel.
func PropDefDataTypeLabel(tipoDato string, t func(key string, args ...any) string) string {
	switch PropDefEnumValueFromXoluValue("data_type", tipoDato) {
	case "numeric":
		return t("prop_definition.data_type.numeric")
	case "alphanumeric":
		return t("prop_definition.data_type.alphanumeric")
	case "date":
		return t("prop_definition.data_type.date")
	case "datetime":
		return t("prop_definition.data_type.datetime")
	case "time":
		return t("prop_definition.data_type.time")
	case "boolean":
		return t("prop_definition.data_type.boolean")
	case "single_select":
		return t("prop_definition.data_type.single_select")
	case "multi_select":
		return t("prop_definition.data_type.multi_select")
	case "system_entity":
		return t("prop_definition.data_type.system_entity")
	case "external_api":
		return t("prop_definition.data_type.external_api")
	default:
		return tipoDato
	}
}

// ── PropAssoc ────────────────────────────────────────────────────────

// propAssocFieldToXolu is an explicit identity map, not truly empty --
// same reason as propDefFieldToXolu above: PropAssociation also has an
// enum-value table (propAssocEnumValueToXolu, for requirement), which
// translateFlatFieldsAndEnums only applies when the key exists in this
// map at all.
var propAssocFieldToXolu = map[string]string{
	"prop_definition_id": "prop_definition_id",
	"target_entity":      "target_entity",
	"entity_id":          "entity_id",
	"requirement":        "requirement",
}

// propAssocEnumValueToXolu: target_entity's own values (asset_type/asset)
// are already English on both sides -- no translation needed, so only
// requirement is listed here.
var propAssocEnumValueToXolu = map[string]map[string]string{
	"requirement": {
		"required": "requerida",
		"optional": "opcional",
	},
}

var propAssocEnumValueFromXolu = reverseEnumTable(propAssocEnumValueToXolu)

// PropAssocValuesToXolu translates a map keyed by PropAssocCreateSchema's
// English field names/values into the Spanish-keyed/valued shape xolu
// stores.
func PropAssocValuesToXolu(schemaValues map[string]interface{}) map[string]interface{} {
	return translateFlatFieldsAndEnums(schemaValues, propAssocFieldToXolu, propAssocEnumValueToXolu)
}

// PropAssocEnumValueFromXoluValue translates a single raw Spanish
// requirement value to English, for the master form's POST handlers
// and (T-18 Stage 9 tier-1) the read side -- list rows and edit-form
// pre-fill. Already-English input passes through unchanged, same
// reasoning as AlertaEnumValueFromXoluValue.
func PropAssocEnumValueFromXoluValue(schemaFieldName, val string) string {
	table, ok := propAssocEnumValueFromXolu[schemaFieldName]
	if !ok {
		return val
	}
	if en, ok := table[val]; ok {
		return en
	}
	return val
}

// PropAssocRequirementLabel translates PropAssoc's English canonical
// requirement value to a display label via t. Accepts legacy-Spanish
// stored values too (converts first), same tolerance as AlertaTypeLabel.
func PropAssocRequirementLabel(requirement string, t func(key string, args ...any) string) string {
	switch PropAssocEnumValueFromXoluValue("requirement", requirement) {
	case "required":
		return t("masters.propassoc.requirement.required")
	case "optional":
		return t("masters.propassoc.requirement.optional")
	default:
		return requirement
	}
}

// PropAssocTargetEntityLabel translates PropAssoc's target_entity value
// (already English on both the schema and xolu-storage sides -- no
// bridge table needed) to a display label via t.
func PropAssocTargetEntityLabel(targetEntity string, t func(key string, args ...any) string) string {
	switch targetEntity {
	case "asset_type":
		return t("masters.propassoc.target_entity.asset_type")
	case "asset":
		return t("masters.propassoc.target_entity.asset")
	default:
		return targetEntity
	}
}

// ── Region ───────────────────────────────────────────────────────────

// regionFieldToXolu is empty, not removed: Region's "description" is now
// the xolu storage key directly (T-40, 2026-08-02), matching the
// entity-name precedent -- no bridge needed since every demo is freshly
// re-seeded. translateFlatFields passes any key absent from this map
// through unchanged, so an empty map here is the correct "no
// translation" case, kept as a function (not deleted outright) so
// RegionValuesToXolu's call sites don't need touching if a future
// Region-only field ever does need a real Spanish/English split again.
var regionFieldToXolu = map[string]string{}

// RegionValuesToXolu is currently a pass-through (see regionFieldToXolu
// above) -- kept as a named function, not inlined, so call sites read
// the same regardless of whether the map is empty or populated.
func RegionValuesToXolu(schemaValues map[string]interface{}) map[string]interface{} {
	return translateFlatFields(schemaValues, regionFieldToXolu)
}

// ── Moneda ───────────────────────────────────────────────────────────

// monedaFieldToXolu is empty, not removed: iso_code/symbol are now the
// xolu storage keys directly (T-40, 2026-08-02), matching Region's and
// entity names' precedent -- no bridge needed since every demo is
// freshly re-seeded.
var monedaFieldToXolu = map[string]string{}

// MonedaValuesToXolu translates a map keyed by MonedaCreateSchema's
// English field names into the Spanish-keyed shape xolu.Create/Update
// expect.
func MonedaValuesToXolu(schemaValues map[string]interface{}) map[string]interface{} {
	return translateFlatFields(schemaValues, monedaFieldToXolu)
}

// ── CentroCosto ──────────────────────────────────────────────────────

// centroCostoFieldToXolu is empty, not removed: accounting_code/name are
// now the xolu storage keys directly (T-40, 2026-08-02), matching
// Region's/Moneda's precedent.
var centroCostoFieldToXolu = map[string]string{}

// CentroCostoValuesToXolu translates a map keyed by CentroCostoCreateSchema's
// English field names into the Spanish-keyed shape xolu.Create/Update
// expect.
func CentroCostoValuesToXolu(schemaValues map[string]interface{}) map[string]interface{} {
	return translateFlatFields(schemaValues, centroCostoFieldToXolu)
}

// ── Marca ────────────────────────────────────────────────────────────

// marcaFieldToXolu is empty, not removed: code/name are now the xolu
// storage keys directly (T-40, 2026-08-02), matching Region's/Moneda's/
// Centro de Costo's precedent.
var marcaFieldToXolu = map[string]string{}

// MarcaValuesToXolu translates a map keyed by MarcaCreateSchema's English
// field names into the Spanish-keyed shape xolu.Create/Update expect.
func MarcaValuesToXolu(schemaValues map[string]interface{}) map[string]interface{} {
	return translateFlatFields(schemaValues, marcaFieldToXolu)
}

// ── Modelo ───────────────────────────────────────────────────────────

// modeloFieldToXolu: brand_id matches the same English name
// asset_xolu_fields.go's assetFieldToXolu already uses for the
// equivalent Asset field (xolu's own storage column for both is
// "brand_id" now, T-40 Block B, this session -- was "marca_id" for
// both until then), for consistency across the app rather than
// inventing a second name for the same concept.
// modeloFieldToXolu is empty -- xolu's own storage keys now match the
// schema's English field names exactly (brand_id, name), same
// identity-pass-through state Marca's own bridge reached once its
// rename completed.
var modeloFieldToXolu = map[string]string{}

// ModeloValuesToXolu translates a map keyed by ModeloCreateSchema's
// English field names into the Spanish-keyed shape xolu.Create/Update
// expect.
func ModeloValuesToXolu(schemaValues map[string]interface{}) map[string]interface{} {
	return translateFlatFields(schemaValues, modeloFieldToXolu)
}

// ── Ubicacion ────────────────────────────────────────────────────────

// ubicacionFieldToXolu is empty, not removed: short_name/description are
// now the xolu storage keys directly (T-40, 2026-08-02), matching
// every prior entity's precedent. region_id was always the same on
// both sides, needing no entry even before this.
var ubicacionFieldToXolu = map[string]string{}

// UbicacionValuesToXolu translates a map keyed by UbicacionCreateSchema's
// English field names into the Spanish-keyed shape xolu.Create/Update
// expect.
func UbicacionValuesToXolu(schemaValues map[string]interface{}) map[string]interface{} {
	return translateFlatFields(schemaValues, ubicacionFieldToXolu)
}

// ── shared helpers ───────────────────────────────────────────────────

// translateFlatFields translates a flat (no arrays, no enum values) map's
// keys via fieldMap. Unrecognised keys pass through unchanged, same
// fallback philosophy as AssetValuesToXolu.
func translateFlatFields(values map[string]interface{}, fieldMap map[string]string) map[string]interface{} {
	out := make(map[string]interface{}, len(values))
	for k, v := range values {
		if nk, ok := fieldMap[k]; ok {
			out[nk] = v
		} else {
			out[k] = v
		}
	}
	return out
}

// translateFlatFieldsAndEnums translates a flat map's keys via fieldMap,
// and additionally translates specific fields' own values via
// enumValueMap (keyed by the English/schema-side field name). Write
// direction only -- see this file's own top comment for why the reverse
// isn't wired into a general FromXolu here.
func translateFlatFieldsAndEnums(values map[string]interface{}, fieldMap map[string]string, enumValueMap map[string]map[string]string) map[string]interface{} {
	out := make(map[string]interface{}, len(values))
	for k, v := range values {
		nk, ok := fieldMap[k]
		if !ok {
			out[k] = v
			continue
		}
		if table, ok := enumValueMap[k]; ok {
			if s, ok := v.(string); ok {
				if es, ok := table[s]; ok {
					v = es
				}
			}
		}
		out[nk] = v
	}
	return out
}

// reverseEnumTable builds the Spanish->English reverse of a
// map[englishField]map[english]spanish table, for the *EnumValueFromXoluValue
// helpers above.
func reverseEnumTable(fwd map[string]map[string]string) map[string]map[string]string {
	rev := make(map[string]map[string]string, len(fwd))
	for field, table := range fwd {
		r := make(map[string]string, len(table))
		for en, es := range table {
			r[es] = en
		}
		rev[field] = r
	}
	return rev
}
