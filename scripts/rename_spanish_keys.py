#!/usr/bin/env python3
"""
rename_spanish_keys.py — one-shot i18n key rename: Spanish-language key
segments -> English, values untouched.

Scope, precisely:
  1. All internal/i18n/locales/*.json files: rename the KEY, leave the
     VALUE exactly as-is in every locale.
  2. All *.go files under internal/ and cmd/: replace the exact quoted
     string literal old key with the new key (e.g. `"masters.custodio.edit"`
     -> `"masters.custodian.edit"`), wherever it appears as a Go string.
  3. internal/formengine/schemas/asset/asset.json: the tab/section
     "label" fields store the i18n key as a literal string value (not
     a Go identifier) -- resolveLabel() in formengine/i18n.go does a
     direct one-key lookup against whatever string is written there,
     confirmed by reading that function before writing this script.
     Same replace, applied to a JSON *value* instead of a JSON *key*.

Deliberately NOT touched, and why:
  - internal/formengine/schemas/masters/{region,moneda,marca}.json:
    confirmed via masters_pages_dynamic.go's own doc comment that this
    path is not wired into any live route (dev/test parity-proof only,
    exercised solely by TestMastersFormParity). These files embed raw
    literal Spanish text directly as schema field titles -- they don't
    use the i18n key system at all, so there's no key here to rename.
    This is a separate, real finding (hardcoded non-i18n Spanish text)
    that's out of scope for a key-renaming pass; flagged, not fixed,
    here.

Safety:
  - Every replacement in .go/.json-schema files matches the OLD key
    wrapped in double quotes, so a key that happens to be a substring
    of a longer, unrelated key/string cannot be partially matched.
  - Locale JSON key rename: guarded by an assertion that the full set
    of keys renamed is exactly the intended 113, and that the new key
    doesn't already exist (would silently overwrite, not append).
  - Prints every single change made, file by file, key by key, so the
    diff is auditable line-by-line rather than trusted blind.
  - Does not run go build/test itself -- caller does that as a
    separate, explicit step after reviewing this script's output.

Usage:
  python3 scripts/rename_spanish_keys.py            # dry run, prints only
  python3 scripts/rename_spanish_keys.py --apply     # writes changes
"""
import glob
import json
import os
import re
import sys
import collections

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__))) \
    if __file__.endswith("scripts/rename_spanish_keys.py") \
    else os.getcwd()

LOCALE_DIR = os.path.join(REPO_ROOT, "internal/i18n/locales")
ASSET_SCHEMA = os.path.join(REPO_ROOT, "internal/formengine/schemas/asset/asset.json")

# ---------------------------------------------------------------------
# The complete old -> new key mapping. 113 keys total, grouped by the
# entity/namespace they belong to for readability. English segment
# choices match this project's own existing EN locale values where a
# direct match exists (e.g. "Custodian", "Brand", "Cost Center").
# ---------------------------------------------------------------------
RENAME = {}

def add(old_prefix, new_prefix, *suffixes):
    for s in suffixes:
        RENAME[old_prefix + s] = new_prefix + s

# masters.alerta.* -> masters.alert_type.* (avoids collision with the
# unrelated runtime "alerts.*" namespace, which is about alert
# instances, not the Alert Type master-data entity)
add("masters.alerta.", "masters.alert_type.",
    "edit", "empty_message", "empty_title", "in_use", "list_title",
    "new", "not_found")
RENAME["masters.alerta.field.descripcion"] = "masters.alert_type.field.description"
RENAME["masters.alerta.field.tipo"] = "masters.alert_type.field.type"

# bare alerta.criticality.* / alerta.type.* -- orphaned (no live Go
# caller found), prospective enum-option-label keys for a formengine
# Alerta form that doesn't exist yet. Renamed for consistency anyway;
# "type" -> "category" avoids the awkward "alert_type.type" collision
# with the entity-name segment itself.
add("alerta.criticality.", "alert_type.criticality.", "high", "low", "medium")
add("alerta.type.", "alert_type.category.", "maintenance", "operational", "security")

# masters.centro_costo.* -> masters.cost_center.*
add("masters.centro_costo.", "masters.cost_center.",
    "edit", "empty_message", "empty_title", "field.accounting_code",
    "field.name", "in_use", "list_title", "new", "not_found")
RENAME["nav.centros_costo"] = "nav.cost_centers"

# masters.custodio.* -> masters.custodian.*
add("masters.custodio.", "masters.custodian.",
    "column.tipo_doc", "edit", "empty_message", "empty_title",
    "field.alias", "in_use", "list_title", "new", "not_found",
    "field.email")
RENAME["masters.custodio.field.direccion"] = "masters.custodian.field.address"
RENAME["masters.custodio.field.identificador_fiscal"] = "masters.custodian.field.tax_id"
RENAME["masters.custodio.field.nombre_razon_social"] = "masters.custodian.field.legal_name"
RENAME["masters.custodio.field.telefono"] = "masters.custodian.field.phone"
RENAME["masters.custodio.field.tipo_sujeto"] = "masters.custodian.field.document_type"
RENAME["nav.custodios"] = "nav.custodians"

# masters.marca.* -> masters.brand.*
add("masters.marca.", "masters.brand.",
    "edit", "empty_message", "empty_title", "in_use", "list_title",
    "new", "not_found")
RENAME["masters.marca.field.codigo"] = "masters.brand.field.code"
RENAME["masters.marca.field.nombre"] = "masters.brand.field.name"
RENAME["nav.marcas"] = "nav.brands"

# masters.modelo.* -> masters.model.*
add("masters.modelo.", "masters.model.",
    "edit", "empty_message", "empty_title", "in_use", "list_title",
    "new", "not_found")
RENAME["masters.modelo.column.marca"] = "masters.model.column.brand"
RENAME["masters.modelo.column.nombre"] = "masters.model.column.name"
RENAME["masters.modelo.field.marca"] = "masters.model.field.brand"
RENAME["masters.modelo.field.nombre"] = "masters.model.field.name"
RENAME["masters.modelo.select_marca"] = "masters.model.select_brand"
RENAME["nav.modelos"] = "nav.models"

# masters.moneda.* -> masters.currency.* (field/column names underneath
# were already English -- iso_code, symbol -- only the entity segment
# itself needs renaming)
add("masters.moneda.", "masters.currency.",
    "column.iso_code", "edit", "empty_message", "empty_title",
    "field.iso_code", "field.symbol", "in_use", "list_title", "new",
    "not_found")
RENAME["nav.monedas"] = "nav.currencies"

# masters.propassoc.* -- "propassoc" itself is already an English
# abbreviation (property association), only the Spanish field-level
# segments underneath need renaming
RENAME["masters.propassoc.column.destino"] = "masters.propassoc.column.target"
RENAME["masters.propassoc.column.propiedad"] = "masters.propassoc.column.property"
RENAME["masters.propassoc.field.entidad_destino"] = "masters.propassoc.field.target_entity"
RENAME["masters.propassoc.field.entidad_id"] = "masters.propassoc.field.entity_id"
RENAME["masters.propassoc.field.obligatoriedad"] = "masters.propassoc.field.requirement"
RENAME["masters.propassoc.field.propiedad"] = "masters.propassoc.field.property"
RENAME["masters.propassoc.select_propiedad"] = "masters.propassoc.select_property"
RENAME["nav.propiedades"] = "nav.properties"

# masters.propdef.* -- same story, "propdef" already English
RENAME["masters.propdef.column.nombre_corto"] = "masters.propdef.column.short_name"
RENAME["masters.propdef.column.tipo"] = "masters.propdef.column.type"
RENAME["masters.propdef.field.nombre_corto"] = "masters.propdef.field.short_name"
RENAME["masters.propdef.field.tipo_dato"] = "masters.propdef.field.data_type"
RENAME["masters.propdef.field.valor_defecto"] = "masters.propdef.field.default_value"
RENAME["masters.propdef.select_tipo"] = "masters.propdef.select_type"

# masters.ubicacion.* -> masters.location.* ("region" itself, singular,
# not renamed -- identical spelling in English and Spanish, unlike the
# plural "regiones" caught separately below)
add("masters.ubicacion.", "masters.location.",
    "edit", "empty_message", "empty_title", "field.region", "in_use",
    "list_title", "new", "select_no_region", "not_found")
RENAME["masters.ubicacion.column.nombre"] = "masters.location.column.name"
RENAME["masters.ubicacion.field.descripcion"] = "masters.location.field.description"
RENAME["masters.ubicacion.field.nombre_corto"] = "masters.location.field.short_name"
RENAME["nav.ubicaciones"] = "nav.locations"

# nav.alertas_config -- the nav label for the Alert Type master list
RENAME["nav.alertas_config"] = "nav.alert_types"

# nav.regiones -- distinct Spanish plural form (English "regions"
# spells differently); masters.region.* itself already uses the
# singular "region", identical in both languages, left alone
RENAME["nav.regiones"] = "nav.regions"

# assets.forms.asset.section.*/.tab.* -- literal key strings written
# directly into asset.json's own "label" fields (confirmed via
# resolveLabel's doc comment before assuming), not Go strings
RENAME["assets.forms.asset.section.basico"] = "assets.forms.asset.section.basic"
RENAME["assets.forms.asset.section.caracterizacion"] = "assets.forms.asset.section.characterization"
RENAME["assets.forms.asset.section.custodia"] = "assets.forms.asset.section.custody"
RENAME["assets.forms.asset.section.docs_ciclo"] = "assets.forms.asset.section.lifecycle_docs"
RENAME["assets.forms.asset.section.fechas"] = "assets.forms.asset.section.dates"
RENAME["assets.forms.asset.section.finanzas"] = "assets.forms.asset.section.finance"
RENAME["assets.forms.asset.section.identificacion"] = "assets.forms.asset.section.identification"
RENAME["assets.forms.asset.section.ubicacion"] = "assets.forms.asset.section.location"
RENAME["assets.forms.asset.tab.ciclo"] = "assets.forms.asset.tab.lifecycle"
# tab.loc and tab.finserv deliberately NOT renamed: ambiguous/English-
# derived abbreviations, not classified as Spanish segments.


def verify_mapping():
    assert len(RENAME) == 113, f"expected 113 mapped keys, got {len(RENAME)}"
    assert len(set(RENAME.values())) == 113, "collision: two old keys map to the same new key"
    for old, new in RENAME.items():
        assert old != new, f"no-op mapping: {old}"
    print(f"Mapping verified: {len(RENAME)} keys, no collisions, no no-ops.\n")


def rename_locale_files(apply):
    print("=" * 70)
    print("STEP 1: Locale JSON files (keys renamed, values untouched)")
    print("=" * 70)
    for path in sorted(glob.glob(os.path.join(LOCALE_DIR, "*.json"))):
        loc = os.path.splitext(os.path.basename(path))[0]
        with open(path, encoding="utf-8") as f:
            data = json.load(f, object_pairs_hook=collections.OrderedDict)

        missing = [old for old in RENAME if old not in data]
        if missing:
            print(f"  {loc}: MISSING {len(missing)} expected old keys: {missing[:5]}...")
            sys.exit(1)
        collide = [new for new in RENAME.values() if new in data]
        if collide:
            print(f"  {loc}: COLLISION, new key(s) already exist: {collide}")
            sys.exit(1)

        new_data = collections.OrderedDict()
        for k, v in data.items():
            new_key = RENAME.get(k, k)
            new_data[new_key] = v
        new_data = collections.OrderedDict(sorted(new_data.items()))

        print(f"  {loc}: {len(data)} keys -> {len(new_data)} keys, "
              f"{len(RENAME)} renamed, values unchanged")

        if apply:
            with open(path, "w", encoding="utf-8") as f:
                json.dump(new_data, f, ensure_ascii=False, indent=2)
                f.write("\n")
    print()


def rename_in_go_files(apply):
    print("=" * 70)
    print("STEP 2: Go source files (exact quoted string literal replace)")
    print("=" * 70)
    go_files = []
    for base in ("internal", "cmd"):
        go_files.extend(glob.glob(os.path.join(REPO_ROOT, base, "**/*.go"), recursive=True))

    total_changes = 0
    for path in sorted(go_files):
        with open(path, encoding="utf-8") as f:
            content = f.read()
        original = content
        file_changes = []
        for old, new in RENAME.items():
            old_lit = f'"{old}"'
            new_lit = f'"{new}"'
            count = content.count(old_lit)
            if count:
                content = content.replace(old_lit, new_lit)
                file_changes.append((old, new, count))
        if file_changes:
            rel = os.path.relpath(path, REPO_ROOT)
            print(f"  {rel}:")
            for old, new, count in file_changes:
                print(f"    {old} -> {new}  ({count}x)")
                total_changes += count
            if apply:
                with open(path, "w", encoding="utf-8") as f:
                    f.write(content)
    print(f"\n  Total Go string-literal replacements: {total_changes}\n")


def rename_in_asset_schema(apply):
    print("=" * 70)
    print("STEP 3: internal/formengine/schemas/asset/asset.json (label values)")
    print("=" * 70)
    with open(ASSET_SCHEMA, encoding="utf-8") as f:
        content = f.read()
    original = content
    changes = []
    for old, new in RENAME.items():
        old_lit = f'"{old}"'
        new_lit = f'"{new}"'
        count = content.count(old_lit)
        if count:
            content = content.replace(old_lit, new_lit)
            changes.append((old, new, count))
    for old, new, count in changes:
        print(f"  {old} -> {new}  ({count}x)")
    print(f"\n  Total schema label replacements: {sum(c for _, _, c in changes)}\n")
    if apply and content != original:
        with open(ASSET_SCHEMA, "w", encoding="utf-8") as f:
            f.write(content)


def main():
    apply = "--apply" in sys.argv
    print(f"Mode: {'APPLY (writing files)' if apply else 'DRY RUN (no files written)'}\n")
    verify_mapping()
    rename_locale_files(apply)
    rename_in_go_files(apply)
    rename_in_asset_schema(apply)
    if not apply:
        print("Dry run complete. Re-run with --apply to write changes.")
    else:
        print("Applied. Now run: go build ./... && go vet ./... && go test ./...")
        print("And: python3 scripts/diff-locales.py")


if __name__ == "__main__":
    main()
