# /attic/static-asset-form

Full snapshots of every file touched by retiring the static Asset
create/edit form (FE-24, docs/formengine/TRACKING.md), taken
immediately before the removal, 2026-07-31 — kept as a reference of
what the codebase looked like before, per the tech team's explicit request
("keep the static asset form in an /attic folder as a memory of what
we used to have, in case we need it for reference"), not as a
reactivatable fallback.

## What this is

Whole-file `.pre-retirement` copies, not surgical diffs — each file
mixed static-form-only code with code that stays active (the read-only
Asset detail/view page, master-data forms, the dynamic form's own
shared endpoints), so a clean isolated extract would lose the
surrounding context that makes the original code make sense on its
own. If you need to see exactly how the static form's `AssetFormPage`,
its widget helpers (`formFieldWithError`, `assetFKSelectWithJump`,
`statusSelectErr`, etc.), or its POST handlers worked, these are the
real, complete files as they stood the day before retirement — not a
reconstruction.

## What was actually removed from the live codebase

- **Routes** (`internal/ui/router.go`): `GET /assets/new`,
  `POST /assets`, `GET /assets/{id}/edit`, `POST /assets/{id}`.
- **Handlers** (`internal/ui/routes.go`): `AssetNew`, `AssetEdit`,
  `AssetCreate`, `AssetUpdate`, `collectDynamicFieldValues`.
- **Rendering** (`internal/ui/asset_detail.go`): `AssetFormData`,
  `ServiceConfig`, `AssetFormPage`, and every widget helper used
  exclusively by it (`assetTypeSelectWithHTMX`,
  `conditionalMethodOverride`, `conditionalDeleteButton`,
  `formFieldWithError`, `conditionalRequired`, `assetTypeSelect`,
  `conditionalError`, `statusSelect`/`statusSelectErr`,
  `connectivitySelect`/`connectivitySelectErr`, `assetFKSelect`,
  `assetFKSelectWithJump`, `assetEnumSelect`/`assetEnumSelectErr`,
  `serviceConfigsField`, `assetTabBtn`).
- **Validation aliases** (`internal/ui/validate.go`):
  `schemaAssetCreate`/`schemaAssetUpdate` (local package aliases —
  the underlying `validation.AssetCreateSchema`/`AssetUpdateSchema`
  themselves are untouched; the REST API still uses them directly).
- **`AssetEnumValueFromXoluValue`** (`internal/ui/asset_xolu_fields.go`)
  — built specifically for the static form's raw-Spanish-POST-value
  handling; the underlying `assetEnumValueFromXolu` table it wrapped
  stays, since the dynamic form's own read-path translation
  (`AssetValuesFromXolu`) uses it independently.
- **Tests** (`internal/ui/validation_handler_test.go`):
  `TestAssetCreate_MissingName_ReturnsForm`,
  `TestAssetCreate_MissingTypeID_ReturnsForm`,
  `TestAssetCreate_InvalidCriticidad_ReturnsForm`,
  `TestAssetCreate_Valid_Redirects` — the other ~40 test functions in
  that shared file (Region/Moneda/Marca/Custodio/AlertaMaster/
  PropDef/Modelo/Ubicacion/PropAssoc/CentroCosto/AssetType/Rule/
  Sensor/AdminUser, plus `errorsToSummary`'s own direct tests) are
  untouched.

## What deliberately did NOT go anywhere near here

- `internal/ui/asset_detail.go`'s own **read-only** Asset detail/view
  page (`AssetDetailData`, `AssetDetail`, `AssetDetailPage`,
  `assetInfoContent`, `infoRow`, `assetIdentityContent`,
  `assetStateContent`, `assetEventsContent`, `assetQRContent`,
  `assetFSMContent`) — never in FE-24's scope, stays exactly as it
  was, permanently, by design.
- `assetDynamicFields`/`assetDynamicField` and the
  `GET /assets/type-fields` route/handler (`AssetTypeFields`) — found
  to be genuinely **shared**: the dynamic form's own `type_id` select
  widget (`renderAssetTypeSelect`, `internal/formengine/widgets.go`)
  targets this exact same endpoint to reload identity/state fields on
  change. Removing it would have broken the dynamic form, not just
  retired the static one.
- `conditionalButtonText` — reused by both
  `asset_dynamic_form_wrapper.go` (the dynamic form's own wrapper) and
  `pages.go` (Rules). Stays as a shared utility.
- `AssetTypeOption` (the type) — used broadly across the list page,
  dashboard, and several unrelated forms' asset-type dropdowns, not
  form-specific despite being declared in the same file as the form
  code that was removed.
- `validation.AssetCreateSchema`/`AssetUpdateSchema`
  (`internal/validation/schemas.go`) — the REST API
  (`internal/handlers/api/assets.go`) validates against these
  directly, independent of the UI layer entirely.

Every one of the "stays" decisions above was verified by grepping
actual call sites across the whole `internal/ui` package before
anything was removed, not assumed from function names or file
location.
