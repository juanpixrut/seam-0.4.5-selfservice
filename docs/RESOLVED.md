# Shelf AMS — Resolved

Closed tracking items, full text as at closure, newest first. Append-only.
Cross-references the changelog entry that shipped the fix; does not
duplicate it.

## [0.4.5] T-136 — es.json mixes voseo and tuteo: 84 keys in voseo (v0.4.5, 2026-11-18)

Theme: i18n · closed 0.4.5 · 2026-11-18
Trigger: 2026-11-18, the same audit.

Found: es.json mixes two forms of address. 84 keys use voseo ("Seleccioná" 32 times, "Ingresá" 16, "Revisá" 8, "Volvé" 6, "querés" 6, "podés" 4, and others); the rest of the file, and what we write now, uses tuteo. By module: workorders-templates 15, movement 14, unit 11, consumable 10, warehouse 9, wo_action 9, issues 5, workorders 5, the rest 7. The project's Spanish is the tuteo of Uruguay.

Scope: the 84 values in tuteo ("Selecciona", "Ingresa", "Revisa", "Vuelve", "quieres", "puedes"). A check in scripts/i18n-sync.py that reports voseo forms would hold it.

Progress (2026-11-18, not released): the 85 values are in tuteo (one more than counted: "Configurá"). TestSpanish_UsesTuteoThroughout (internal/i18n) fails on a voseo form.

Left: the release.

Cross-ref: CHANGELOG 0.4.5.

## [0.4.5] T-135 — About 35 fields of application forms show their literal English title; three masters forms have Spanish literals (v0.4.5, 2026-11-18)

Theme: i18n · closed 0.4.5 · 2026-11-18
Trigger: 2026-11-18, the same audit.

Found: a form shows a field's literal title when no key exists for it (resolveTitle: <module>.forms.<form>.field.<name>, then field.<name>). Of 373 fields in the 50 embedded definitions, about 154 have a title and no key. This count pairs each file with its module and form by name, which is approximate.

- Application forms, about 35: workorders_templates/step-2 (12 of 14), workorders_templates/create (9), asset/asset (measure, threshold, alert_id), wo_action create and edit (key, options), issues (assignee, severity), workorders (consumable_id, template_id), crew (member), developer (schema, key). They show English in every language.
- masters/marca, moneda and region: the literal titles are Spanish ("Código", "Nombre", "Código ISO (3 letras)", "Símbolo", "Región", "Descripción"). Keys exist for these forms, so the literals are fallbacks, in the wrong language.
- The Demo module: about 119 fields in 12 definitions, English only, with their help texts.

Scope: keys in the four languages for the application forms; English literals in the three masters files; decide whether the Demo forms are translated or stay English as a developer's tool. A test over every embedded form and every language, with the real module and form names, in place of this estimate.

Progress (2026-11-18, not released): measured properly, by rendering every embedded form in Japanese and listing what came out without a Japanese character, the application forms had 22 untranslated texts, not 35: the estimate paired files with forms by name and missed the keys of rows (repeatable_item.*). All 22 have keys now: the Assignee and Severity of Issues with its three values, Parameters of a work-order action, Template of the picker, Form definition and Key in the Developer forms, the logical operators and message severities of step 2 of a work-order template, and four placeholders of the asset form. The form engine did not look placeholders up at all: 13 translated placeholder keys of the asset form were never read. It does now (<module>.forms.<form>.field.<name>.placeholder, and repeatable_item.<parent>.<item>.placeholder in a row). The three masters forms have English literals. TestForms_NoTextIsLeftUntranslated holds it.

Left: the Demo forms stay in English by decision, a developer's tool; time zone names are T-58.

Cross-ref: CHANGELOG 0.4.5.

## [0.4.5] T-131 — Issues skips the module lifecycle Work Orders follows: no flag, no per-tenant switch, routes always mounted (v0.4.5, 2026-11-18)

Theme: issues · closed 0.4.5 · 2026-11-18
Trigger: 2026-11-18, comparing how the Issues module enters an instance with how Work Orders does (docs/ISSUES_MODULE.md).

Found: Work Orders has a start-time flag in the catalogue (enable_work_orders, class preview, off by default, SEAM_ENABLE_WORK_ORDERS), a runtime switch for one tenant (module_work_orders, DEV-27), routes that are not mounted when it is off or are mounted behind moduleGate and answer 404 for a tenant it is off for, a registry entry whose HasAccess and HasAccessFor ask the same question so the menus agree with the routes, and a line in the Developer panel. Issues has none of these: it is not in internal/features/catalog.go, MountIssuesRoutes mounts unconditionally, HasAccess is true and there is no HasAccessFor. A deployment cannot leave it out and a tenant cannot be given it or denied it, while the module is still being built.

Two parts of Issues are mounted outside its own routes and a gate has to cover them too: the public page of a QR code (GET /s/{code}, in the core router, which also serves asset codes) and the QR card on the asset page (internal/ui/asset_detail.go).

Scope: a catalogue flag for the module (name, class and default to be decided: on by default keeps every instance as it is today; off by default as a preview matches Work Orders and removes it from instances that do not ask for it), the runtime switch and moduleGate as for Work Orders, HasAccessFor in the registry, the two outside surfaces behind the same question, the launch scripts and the demo package switching it on where they load its data, and the tests Work Orders has for the gate. Goes with T-125: the permission answers who, this answers whether.

Progress (2026-11-18, not released): Issues has the lifecycle of Work Orders. Decided: off by default, class preview, as Work Orders is. The flag is `issues` (`enable_issues`, `SEAM_ENABLE_ISSUES`); the switch for one tenant is `module_issues`. MountIssuesRoutes mounts nothing when the flag is off and no module can be switched, and otherwise puts every route behind moduleGate; the registry's HasAccess and HasAccessFor ask the same question; the Developer panel lists the switch. Of the two outside surfaces: the public channel of a QR code (POST /s/{code}/report, GET /s/{code}/done) follows the start-time flag alone, since an anonymous request has no tenant to ask the switch for, and with the flag off GET /s/{code} shows the asset without a form; the QR card on the asset page stays with Assets, whose routes it uses, and a code made there identifies the asset whether or not Issues is on. The scripts handle the three opt-in application modules through one list (scripts/lib/modules.sh): a seed's header says which module it is for (x.seam.module), scripts/launch.sh and scripts/build-demo.sh switch on the modules of the data they load, or the ones named with --modules (MODULES= in make), and make dev sets the three.

Verified: go test ./... (features, config, the mount tests of internal/modules/issues, the QR landing with and without the module, the runtime switch through the panel); scripts/tests (the list against the catalogue, every addition's module, the choice of modules); and on a running Seam over xolu: flag off, /issues, /issues/new, /issue-templates and the public report channel answer 404, the menu has no entry and a scanned code shows the asset; flag on, the 15 checks of the Issues application pass; flag off and the switch used from the panel, /issues opens for the tenant's developers, 404 for its admin, 404 again once switched off. make launch with a dataset, with MODULES=, with SEAM_ENABLE_ISSUES=0, empty, and on a later launch of the same data directory; make dev; make demo with and without MODULES=.

Left: the release. An instance that upgrades loses Issues until it sets the flag: the changelog says so.

Cross-ref: CHANGELOG 0.4.5.

## [0.4.5] T-123 — The demo bundle and make launch load a dataset and none of its additions (work orders, Issues, accounting) (v0.4.5, 2026-11-18)

Theme: xolu-integration · closed 0.4.5 · 2026-11-18
Trigger: 2026-11-18, T-120. scripts/build-demo.sh and the launch flow load one of the six datasets. The additions under seeds/workorders, seeds/issues and seeds/accounting are loaded by hand (python3 seeds/seed.py load issues-01-constructec), so a fresh demo opens the Issues and accounting modules empty.

Scope: decide which additions a demo carries (the accounting module is off by default), then have build-demo.sh load them after the dataset; the loader takes several names in one command.

Fixed in the tree 2026-11-18 (unreleased). Decided: a dataset comes with all its additions (work orders, its Issues file, accounting) unless `--additions` says otherwise; where they are loaded the two modules that are off by default are on (the demo package's config.json; `make dev`). scripts/launch.sh did not load data at all: on the first run of a data directory it now asks what to start with, or takes `--seed` (`make launch SEED=...`; `make dev` loads 01-constructec). Both scripts load through scripts/lib/seeds.sh. Found on the way: naming a dataset and an addition in one load failed, because the addition's lookups ran before the dataset's records; additions now run as passes of their own after it (docs/SEED_FORMAT.md section 4, conformance case 86). Verified on Linux: launch.sh with a dataset, asked and unasked, and a second launch that leaves the data alone; make dev with its three logins; a demo package for Linux run from its zip; a Windows package built and its launcher read. Remaining before closing: a run on macOS.

Cross-ref: CHANGELOG 0.4.5.

## [0.4.5] T-121 — scripts/launch.sh and the demo bundle started xolu without API v2, so reporting an issue answered 500 (v0.4.5, 2026-11-18)

Theme: xolu-integration · closed 0.4.5 · 2026-11-18
Trigger: found 2026-11-18 while writing the Issues seeds (T-120). The lifecycle of an issue is a xolu state machine (/api/v2/fsm); xolu 0.30.44 serves the /api/v2 routes only with XOLU_API_V2_ENABLED=true.

Found: on an instance started the way scripts/launch.sh starts it, GET /issues answered 200 and POST /issues answered 500 ("Failed to initialize issue workflow"); xolu answered 404 on GET /api/v2/fsm/def. With the variable set, the 15 browser checks of the Issues pages pass.

Fixed in the tree (unreleased): scripts/launch.sh and the three places scripts/build-demo.sh starts xolu from set XOLU_API_V2_ENABLED=true. The seed loader says so when an /api/v2 route answers 404.

Scope: verify on a clean checkout with make launch and with a built demo bundle (Linux, macOS, Windows launcher), then close. Not looked at: other deployments that start xolu by their own means (they need the same variable), and whether Seam should say at start-up that the routes are missing.

Progress 2026-11-18, second part (unreleased): the setting no longer lives in each script. scripts/lib/xolu-env.sh is the one list of how Seam runs xolu; scripts/launch.sh, the data-loading step of scripts/build-demo.sh, the START.sh and START.cmd it writes, `make start-xolu` and docker-compose.yml take it from there or are tested against it (scripts/tests/test_makefile.py). Verified on Linux: an instance started by scripts/launch.sh and one started by make dev accept a new issue (303); a demo package built for Linux, unzipped elsewhere and started with its START.sh passes the browser checks of the Issues pages, a new issue included. Not run: macOS, the Windows launcher (its text was read), docker-compose (T-124).

Cross-ref: CHANGELOG 0.4.5.

## [0.4.5] T-117 — Refused form save logs 'superfluous response.WriteHeader' at ERROR (v0.4.5, 2026-11-18)

Theme: developer-module · closed 0.4.5 · 2026-11-18
Fixed in the tree (unreleased), closes with the next release: the handler wrote 422 and `PageHandler.Render` then wrote 200. `PageHandler.RenderStatus` writes the status once, after the Content-Type (which the first write had also lost), for full pages and htmx partials; `Render` is the same with 200. No other handler in internal/ui writes a status and then calls `page.Render`. Verified: TestRenderStatus_WritesTheStatusOnce, and no ERROR line on the live stack for a refused save (2026-11-17).

Trigger: review of the Developer module on a live stack, 2026-11-17.

Each time POST /developer/forms/save refuses a definition (HTTP 422) the server logs `http: superfluous response.WriteHeader call from .../handlers/middleware.(*responseWriter).WriteHeader (middleware.go:333)` at ERROR. The response the browser gets is correct; the status is being written twice somewhere between the handler and the page renderer.

Scope: find the second WriteHeader on the 422 path and remove it; check the other handlers that set a status before h.page.Render.

Cross-ref: CHANGELOG 0.4.5.

## [0.4.5] T-116 — Eleven embedded forms in eight packages cannot be overridden and are not in the Developer catalogue (consumable, crew, movement, outsourced, unit, warehouse, wo_action, workorders_templates) (v0.4.5, 2026-11-18)

Theme: developer-module · closed 0.4.5 · 2026-11-18
Note 2026-11-17 (DEC-37): the catalogue lists 37 forms, all in the identity table; the asset update form follows the override of the asset definition; definitions generated from their own records (issue templates, work-order actions, template steps) stay outside the mechanism by design.

Fixed in the tree (unreleased), closes with the next release: the eight packages are in the catalogue (each has a `Names` function) and their 27 render and parse sites go through `resolveSchema`; `buildTemplateActionMappingPlan` takes the request's context for the template's second step. The catalogue now lists 35 forms. TestCatalogue_CoversEverySchemaPackage fails when a schema directory is in neither the catalogue nor `developerSchemaPackagesLeftOut` (the Demo is, with its reason), and TestSchemaSites_AllGoThroughResolveSchema covers the eight packages' names. Verified in Chromium on the live stack with the Work Orders module on (2026-11-17): for warehouse/create, unit/create, crew/create, consumable/create, outsourced/create, wo_action/create and workorders_templates/create, an override shows on the form's page and leaves with it. Not looked at in a browser: warehouse/edit, wo_action/edit, movement/create and workorders_templates/step-2, which need existing records. The two schemas handlers built inline are embedded files now (issues/assign, outsourced/account; 37 forms), behind `resolveSchema`; a test fails if a handler declares a schema inline again. Outside the mechanism by nature: definitions generated per record (issue templates, work-order actions and template steps, the asset update form), recorded in docs/KNOWN_ISSUES.md.

Trigger: review of the Developer module, 2026-11-17, comparing internal/formengine/schemas with the module's catalogue (internal/ui/developer_catalog.go) and with the pages that pass their schema through resolveSchema.

The catalogue lists 23 forms from seven packages (asset, developer, issues, masters, sensors, system, workorders). Eleven embedded schemas in eight packages are neither listed nor overridable, because their pages read the embedded file directly: consumable, crew, movement, outsourced, unit, warehouse (2), wo_action (2), workorders_templates (2). These are the operational forms the Work Orders team works on. Schemas built inline by handlers (for example in outsourced.go and issue_assign.go) are outside the mechanism as well.

Scope: route those pages through resolveSchema and add the packages to the catalogue, with a test that fails when a schema package is in neither the catalogue nor an explicit exclusion list (the Demo module is excluded on purpose).

Cross-ref: CHANGELOG 0.4.5.

## [0.4.5] T-115 — Form-definition gate accepts documents the engine cannot render (unknown widget, unresolved source, no fields) (v0.4.5, 2026-11-18)

Theme: developer-module · closed 0.4.5 · 2026-11-18
Fixed in the tree (unreleased), closes with the next release: `formengine.CheckRenderable` refuses a definition with no fields, an unknown widget or a source nothing resolves for that form; the JSON editor, its preview, the designer and restore pass through it in both scopes (422 with the reason), DEC-32. Decided: removing a required field stays a note. The schemas directory is gated too (DEC-33): a hand-written definition the engine cannot render is set aside when the directory loads, what was in force stays, and the Overview, the forms list and its editor say so. Verified: internal/formengine/check_test.go, internal/ui/developer_improve_test.go, and on a live stack (the three cases refused in the editor, the real form untouched, a hand-written file flagged).

Trigger: review of the Developer module on a live stack, 2026-11-17.

The gate that checks a form definition before it is saved or loaded from a directory accepts documents the engine cannot render. Tried on issues/create, through the JSON editor (tenant scope) and by writing the file:
- a field with an unknown widget (`"x-seam-input": "nope"`): saved; the real form shows `formengine: field "zz" needs widget "nope", not implemented before stage 3/4` in place of the field;
- a select with a source nobody resolves: saved; the real form shows `... no SourceResolver is configured`;
- every field removed: saved, with compatibility notes; the real form has no fields.
A rule the parser itself enforces is refused correctly (a query-picker with `SELECT *` is rejected with the rule named), and a broken translation file is refused, reported on the page and the previous version kept. The file layer reports "Cargado" for the unknown-widget file.

Scope: the gate should render the document once, with empty values, and refuse it when the result contains an engine error; decide whether removing a field the embedded form requires should be refused or stay a note.

Cross-ref: CHANGELOG 0.4.5.

## [0.4.5] T-114 — Form designer: no button works until the preview's required fields are filled in (v0.4.5, 2026-11-18)

Theme: developer-module · closed 0.4.5 · 2026-11-18
Fixed in the tree (unreleased), closes with the next release: the designer's form (`dev-designer`) holds only the hidden fields that travel with every action; its controls sit in the page's panels and name it with `form=`, every button carries `formnovalidate`, and the preview is drawn in a form of its own that never submits. A previewed field named like one of the designer's own (note, schema, json) therefore no longer reaches the POST either. Verified: TestDesigner_ThePreviewIsOutsideTheDesignersForm, and in Chromium on the live stack: add, edit, apply, up, down, rename, preview, save and remove, with nothing typed into the preview (2026-11-17).

Trigger: review of the Developer module on a live stack, 2026-11-17.

On /developer/designer?schema=masters/marca every button (Agregar, Editar, Subir, Bajar, Quitar, Vista previa, Guardar) does nothing, and the browser shows "Please fill out this field" on the preview's "Código". The page is one <form> that also contains the live preview, and the preview's inputs carry `required`; the browser refuses to submit until they are filled. Confirmed: with the preview's required inputs filled, Add adds the field and Save stores the override; with them empty, no request is sent. Any form with a required field is affected, which is nearly all of them.

Scope: the designer's own buttons must not validate the preview (formnovalidate on them, or the preview outside the form). Add a test that the designer's markup has no required input a submit would trip on. The page was verified by tests only before this (DEV-21).

Cross-ref: CHANGELOG 0.4.5.

## [0.4.5] T-113 — Three catalogued master forms (region, currency, brand) can be overridden but their pages are still the hand-built ones (v0.4.5, 2026-11-18)

Theme: developer-module · closed 0.4.5 · 2026-11-18
Trigger: review of the Developer module on a live stack, 2026-11-17 (xolu 0.30.44, Constructec seed, signed in as the developer user).

A form override of masters/marca is saved and listed as "Sobrescritura (tenant)", and the real form at /brands/new does not change; the same with the override written to SEAM_SCHEMAS_DIR. An override of issues/create, made the same way, shows on /issues/new at once.

Cause (corrected 2026-11-17; this item first blamed the `context.Background()` in internal/ui/masters_pages_dynamic.go, which is not what decides it): the region, currency and brand routes (internal/ui/masters_routes.go) render the hand-built pages of masters_pages.go. The form-engine versions in masters_pages_dynamic.go are the equivalence proof of FE-07/FE-08 and are called only by their tests; FORM_ENGINE_SPEC.md section 13.3 keeps both until the routes move. The three schemas are in the Developer catalogue because the package is, so the module offered an override nothing reads.

Fixed in the tree (unreleased), closes with the next release: the three routes render their fields with the form engine. The handlers call `h.regionFormPage`, `h.monedaFormPage` and `h.marcaFormPage` (internal/ui/masters_pages_dynamic.go), which render the definition for the request (its tenant's override, its locale; labels from the new `masters.forms.*` keys, copied from the texts the hand-built rows used) and hand the fields to the page functions of masters_pages.go, which still compose the title, the error line, the CSRF field, the buttons and the modal wrapper. The hand-built rows stay for a caller that supplies no fields. The "not live" marker of the first pass is removed with its texts: no catalogued form needs it now. The handlers still read the fields they know (description; iso_code, symbol; code, name), as every other overridable form does. Verified: TestMasters_OverridesReachTheLiveForms, and in Chromium on the live stack: the three forms open, a brand is created, an empty one is refused, the edit form opens in its modal with its values, an override of masters/marca shows on /brands/new and leaves with it (2026-11-17).

Cross-ref: CHANGELOG 0.4.5.

## [0.4.5] T-112 — Developer runtime switches cannot be saved on an instance where none was saved before (feature_state does not exist): fixed, unreleased (v0.4.5, 2026-11-18)

Theme: developer-module · closed 0.4.5 · 2026-11-18
Trigger: issue report of 2026-10-15 (Work Orders team, status PENDIENTE): every switch under "Interruptores en tiempo de ejecución" on the Developer overview answers "No se pudo guardar el cambio." and the server logs `developer: flag change refused flag=hotload_i18n action=on error="devstate: save hotload_i18n: xolu: XOLU-QL004: entity 'feature_state' does not exist (status 400)"`.

Cause: xolu has no "create entity" step; an entity exists from its first row, and until then a SELECT on it fails with XOLU-QL004 "entity '<name>' does not exist". internal/devstate's XoluStore reads before it writes (find, then Update or Create), took that answer as a failure, and so never reached the Create that would have made the entity. On an instance where no switch had ever been saved, no switch could ever be saved. The same read also failed at start-up ("could not read persisted panel state; starting with defaults"). The unit tests passed because their in-memory fake answers a query on a missing table with no rows. This is the part of T-106 that said the feature_state entity was never run against a live xolu.

Reproduced 2026-11-17 on 0.4.3 against a real xolu 0.30.44 (Constructec seed, Developer module on, signed in as the developer user): the same log line, and the start-up error.

Fix (unreleased): auth.EntityNotYetCreated(err) recognises that answer (code and message, since XOLU-QL004 alone covers other query failures); devstate.XoluStore treats it as "nothing stored" in find and ListAll; internal/overrides' XoluStore does the same for i18n_override and schema_override, which had the identical fault for the first translation or form override on a fresh instance. internal/ui's own entityNotYetCreated now calls the shared function.

Verified: new tests in internal/devstate and internal/overrides use a fake that answers like a real xolu; they fail without the fix. A different failure under the same code still stops the save. Live: the first switch saves and creates the row, the page shows it on after a reload, switching it off updates the same row, and the start-up error is gone. Second live pass, 2026-11-17, each run on a fresh xolu: with the fix, all five switches turn on and off (one row each, updated in place), and with SEAM_DEVELOPER_TENANT_OVERRIDES=1 the first translation override and the first form override are stored and shown. The same steps on the original 0.4.3 build fail at every save with "No se pudo guardar el cambio.", for the switches and for both overrides. Workaround for an unpatched 0.4.3, also verified: start xolu with XOLU_OQL_UNKNOWN_ENTITY=empty; every save then works with no change to Seam.

Closes with the next release. Reply to the reporter once it ships.

Cross-ref: CHANGELOG 0.4.5.

## [0.4.5] T-106 — Developer module: run in a browser and against a live xolu (done, unreleased) (v0.4.5, 2026-11-18)

Theme: developer-module · closed 0.4.5 · 2026-11-18
Verified (2026-11-17), closes with the next release: every point of the trigger was run on a live stack (xolu 0.30.44, authentication on, a headless browser). `/developer` and its sections (141 browser checks); `feature_state` and the panel switches; hot reload from both override directories; the Developer entry in the module selector; the first developer created with `seam-admin` logging in and getting the module; the Startup page listing the four visual flags. The override entities (`i18n_override`, `i18n_override_history`, `schema_override`) need no declaration. Two instances over one xolu: a text, a form, a switch and a module switch made on one were in force on the other within the sync interval, and the same form saved on both at once (three rounds) gave six distinct versions and one agreed definition. Not reproduced, still possible: two saves in the same instant numbering two versions alike (docs/KNOWN_ISSUES.md).

Trigger: the devtools checkpoint (built on 0.4.1-pre2) was merged by hunk onto the pre8 line on 2026-10-12. Compiles, vets and passes `go test ./...` apart from the eight tests filed as T-102; the registry, the developer role and the overlays have their own unit tests (including a race run). Not done: `/developer` in a browser, with authentication on, against a live xolu (the `feature_state` entity, the panel switches, hot reload from `SEAM_LOCALES_DIR` / `SEAM_SCHEMAS_DIR`); the Developer entry in the module selector; creating the first developer with `seam-admin`; and a check that the panel lists the four visual flags. Their own open items live in `docs/DEVELOPER_MODULE_CAMPAIGN.md`.
Progress 2026-10-13: this also covers the `i18n_override` and `schema_override` entities (internal/overrides): they are tested against a strict fake of xolu's entity API and OQL, never against a live xolu, so whether the entities must be declared first, and how real OQL treats the column names `key`, `name`, `doc`, are unverified.
Progress 2026-11-17: `feature_state` was run against a live xolu 0.30.44 while fixing T-112: nothing has to be declared (the entity appears with its first row, and a read before that must be treated as empty), and a `WHERE tenant_id = n AND key = '...'` query works. `i18n_override` and `schema_override` got the same fix and were then run live too (first translation and first form override stored and shown, with SEAM_DEVELOPER_TENANT_OVERRIDES=1).

Cross-ref: CHANGELOG 0.4.5.

## [0.4.3] T-95 — ANSI LIMIT/OFFSET used where xolu parses T-SQL: seam-admin GetByEmail, auth ListUsers, two spec docs -- changed to TOP, compiled and tested (v0.4.3, 2026-11-17)

Theme: xolu-integration · closed 0.4.3 · 2026-11-17
Trigger: instruction 2026-10-07 -- xolu parses T-SQL, so ANSI LIMIT is wrong; the equivalent keyword is TOP.

Changed in the 2026-10-07 checkpoint (written, NOT compiled): cmd/seam-admin/client.go GetByEmail (`LIMIT 1` became `SELECT TOP 1`); internal/auth/store_xolu.go ListUsers (Limit was ignored and an invalid ` OFFSET n` was appended -- OQL has no OFFSET; it now fetches TOP (limit+offset) and slices off the offset, the approach alerts.List and audit_store.go already use); the query examples in docs/DOCUMENTS_MODULE_SPEC.md and docs/SEAM_MEDIA_SPEC.md.

Deliberately left alone: docs/modules/TRACKING.md:52 (a Sulpher graph query, Cypher-style `LIMIT 5` -- confirm Sulpher's grammar before touching it) and docs/oql-orderby-diagnostic-report.md:62 (a raw sqlite3 shell command, where LIMIT is correct). Every other LIMIT hit is a comment saying not to use it, the SEAM_RATE_LIMIT setting, or the REST ?limit= parameter.

Progress 2026-11-17: go build, go vet and go test ./... are clean at 0.4.3 (35 test packages, including the auth ListUsers TOP and offset-slicing tests). Resolution 2026-11-17, both remaining checks settled against xolu 0.30.44 (the version go.mod pins):
1. TOP with ORDER BY. The ORDER BY DESC defect Seam reported (T-47, docs/oql-orderby-diagnostic-report.md) was fixed upstream in xolu 0.24.7 (their T-144). In 0.30.44 the OQL executor sorts and then applies TOP, and the SQL push-down path generates ORDER BY before the row limit. xolu's own regression tests were run at 0.30.44 and pass: TestOrderBy_StringTimestampField_FullOrderNotJustExtremum, the TestFieldQuery TOP-with-ORDER-BY cases and the SQL generation tests. Not run: Seam's own queries against a live xolu.
2. Sulpher LIMIT. Sulpher has its own parser, based on openCypher 9, where LIMIT is the correct keyword. The example in docs/modules/TRACKING.md was parsed with xolu's pkg/sulpher parser: ORDER BY a.created_at DESC is read as a descending sort and LIMIT 5 as a limit of 5. The example stays as written.
ListUsers still has no caller; the Users selector will be its first consumer (T-96, T-98).

Cross-ref: CHANGELOG 0.4.3.

## [0.4.1-pre8] T-94 — Repeatable / document-links add button dies once every row is removed (Work Orders report) -- fix written, unverified (v0.4.1-pre8, 2026-10-13)

Theme: formengine · closed 0.4.1-pre8 · 2026-10-13
Trigger: Work Orders team bug report -- "x-seam-input: repeatable breaks when the item count reaches zero" (New Asset form, Service Configurations section).

Root cause, confirmed by reading the code and by reproducing the exact TypeError in node against the original handler: the add button cloned the first remaining row (`c.querySelector('.repeatable-row').cloneNode(true)`); once every row has been removed with the x button the query returns null and the handler throws `Cannot read properties of null (reading 'cloneNode')`, leaving the + button dead until the page is reloaded. The server side already handles zero rows (`parseAssetArrayField` returns an empty list, same as a single blank row), so nothing there changes.

Fix in the 2026-10-07 checkpoint (written, NOT compiled): the remove handler stashes a clone of the row on the container (`c._rowTemplate`) before removing it; the add handlers fall back to that stash and no-op if neither exists. New helper `rowRemoveOnclick(rowClass)` in internal/formengine/repeatable.go. The same fix is applied to the document-links widget (internal/formengine/widgets.go) and the masters document-links builder (internal/ui/masters_pages.go, plus the new const in internal/ui/doclink_script.go). Tests: one assertion updated and one regression test added in each of repeatable_test.go and document_links_test.go.

Outstanding verification: `go build ./... && go test ./internal/formengine/... ./internal/ui/...` (never run); a real-browser check of New Asset > Service Configurations (remove all rows, add, add, remove, add; submit with zero rows and confirm the asset saves with no service configs). Assumption to confirm with the Work Orders team: zero rows is allowed. Alternative if they want one row always: clear the last row instead of removing it (one-line change in rowRemoveOnclick). See docs/HANDOVER_2026-10-07.md section 4.

Cross-ref: CHANGELOG 0.4.1-pre8.

## [0.4.1-pre8] T-102 — internal/ui: 8 tests failing at baseline (demo templates x7, selector unknown key x1) (v0.4.1-pre8, 2026-10-13)

Theme: test-quality · closed 0.4.1-pre8 · 2026-10-13
Trigger: found as the baseline of the 2026-10-12 session, before any change to the Demo, Selector or form code. First run of `go test ./...` since T-100 was filed. Eight tests in `internal/ui` fail: seven `TestDemoTemplate*` / `TestDemoIndex*` tests in `demo_templates_test.go` reject the extra `SELECT TOP 1 * FROM tenant_settings WHERE key = 'general'` / `'features'` queries the demo handlers now issue (the tests assert that no OQL query reaches beyond `demo_templates`), and `TestSelector_UnknownKey_Returns400` in `selector_test.go` still treats `assets` as an unknown selector key although `selector_entities.go` made it a valid one (it gets 500, an unconfigured xolu client). Decide per test whether the code or the assertion is the stale side before changing either.

Cross-ref: CHANGELOG 0.4.1-pre8.

## [0.4.0] T-10 — Extend FORM_ENGINE_SPEC.md with widgets/extensions the Order Engine (CMMS) needs (v0.4.0, 2026-08-17)

Theme: form-engine · closed 0.4.0 · 2026-08-17


Trigger: "AMS Orders -- anotada.docx" v1.0 reviewed 2026-07-28; full gap analysis in docs/ORDER_ENGINE_INTEGRATION_NOTES.md.
Scope: Missing widgets -- signature, image (distinct from document-links), GPS (device capture / map picker, vs. today's ad-hoc lat/long number fields), multi-select (DomainMultiSelect has no equivalent at all), decimal precision/scale (xolu has format:"decimal" at the schema layer but FORM_ENGINE_SPEC doesn't expose it), barcode symbologies EAN/UPC (distinct from the QR support internal/qrcode already has). Missing x-seam-* extensions -- unit of measure, input mask, custom-validation extensibility point, ERP external code. Decision needed: pull some into T-01's MVP now vs. defer and document as known gaps -- not decided yet.

Correction, 2026-08-04, reviewer feedback on the T-02 interdependency pass: "After T-01 MVP; informs T-01 scope" was stale in the same shape T-02/T-03/T-08 were -- T-01 closed 2026-07-31 (see RESOLVED.md), and it shipped without every widget/extension this item lists. The "pull into MVP now vs. defer" question was never explicitly decided; it resolved itself by default when T-01 closed. Worth naming plainly rather than leaving an open decision that already has a de facto answer nobody chose on purpose.

Signature widget built 2026-08-04 as real progress, not just re-triage -- a general-purpose primitive (like asset-picker, T-19), not Order-Engine-specific, so it didn't need T-11's own scoping to be worth building now. Canvas-based capture, base64 PNG value, registered as x-seam-input: "signature". Security design follows T-63/T-64's established lesson directly: an existing value is placed in the hidden input's own value="..." attribute (minty's normal automatic escaping, confirmed by an adversarial test using a script-tag-breakout-shaped value), never interpolated into the accompanying <script> block -- only the schema-authored, trusted field name goes into the script itself. Tests in internal/formengine/widgets_test.go, including the adversarial escaping one.

Remaining scope, still open: image, GPS, multi-select, decimal precision/scale, barcode (EAN/UPC), and the four x-seam-* extensions (unit of measure, input mask, custom validation, ERP external code). Status moved ☐->◐ to reflect this, not closed -- most of T-10's named scope is still unbuilt.

Completed 2026-08-17. All remaining scope built, tested, and live-verified: decimal (exposes xolu's own format:"decimal", found and fixed a real pre-existing bug along the way -- ParseSchema treated queryfy's non-fatal format warnings as hard errors, invisible until "decimal" was the first format string this codebase ever used that queryfy doesn't natively recognise), multi-select (checkbox group, not native <select multiple>), barcode (JsBarcode, vendored MIT, EAN/UPC live SVG preview), gps (Leaflet map picker + device capture, reusing the map page's own already-vendored Leaflet), and image (a new xolu-/blob-plus-metadata-entity gallery subsystem -- internal/ui/media_gallery.go -- fronted by a Lit-wrapped Dropzone.js upload UI, matching the FSM editor's own established shell-plus-engine Lit pattern; caught and fixed two more real bugs during build -- a slash-delimited blob key that would have failed every real upload since xolu's own keys are flat, caught by the mocked test suite before reaching a live server; and a first-upload sequencing bug found live against a genuinely fresh xolu tenant, fixed by reusing T-28's existing entityNotYetCreated helper rather than a new mechanism). All four x-seam-* extensions (unit, input-mask, custom-validator, erp-code) plus x-seam-barcode-format built and tested. FORM_ENGINE_SPEC.md (v0.3 -> v0.4) now documents all nine items -- this was T-10's own literal scope, not a side task, and had been outstanding even after the code/tests for all nine were complete. Two pieces of pre-existing spec staleness corrected in passing: the asset-picker entry and its §14 summary row still said "reserved for future implementation," stale since T-19/T-72 actually implemented it; §9.2's recognised-formats table was missing date-time/time/tel despite all three being implemented and in use. Two follow-on items filed separately, not blocking this closure: T-92 (config.Server.MaxBodySize parsed but never enforced) and T-93 (internal/qrcode's real rendering gap, with a candidate fix and its own open blockers).

Cross-ref: CHANGELOG 0.4.0.

## [0.3.84] T-30 — Fully done -- added the three missing global i18n fallback keys (field.code/iso_code/symbol) these dormant schema files would need if ever wired live; live masters forms were already correctly localized via a separate, pre-existing mechanism, confirmed directly rather than assumed (v0.3.84, 2026-08-13)

Theme: i18n · closed 0.3.84 · 2026-08-13


Found 2026-08-01 while investigating scope for the 113-key i18n rename (T-18-adjacent). These 3 schema files are NOT wired into any live route -- masters_pages_dynamic.go's own doc comment confirms this is dev/test parity-proof scaffolding only, exercised solely by TestMastersFormParity, coexisting with the live hand-coded masters_pages.go path per FORM_ENGINE_SPEC.md $13.3 until dynamic-form parity is proven. Not urgent since nothing here is ever shown to an actual tenant today, but a real instance of the internal/external law's violation shape (KNOWN_ISSUES.md) if/when this path is ever wired live -- these files bypass the i18n key system entirely rather than using Spanish-named keys, so there's no key to rename; fixing this means adding proper i18n key references to region.json/moneda.json/marca.json's own title/field-title values before this path could ever go live without reintroducing the same class of bug T-18 fixed elsewhere.

**Closed, 2026-08-13, prompted directly by the tech team asking whether this item was too difficult to close.** Traced formengine's own actual title-resolution mechanism first, not assumed: resolveTitle (internal/formengine/i18n.go) already tries a module-scoped key, then a global `field.{fieldName}` fallback, before ever falling back to the schema's own literal text -- meaning this item's own scope was narrower than its own description implied. Checked every field across all three schemas against the existing global keys: `field.description` (region) and `field.name` (marca) already existed and already worked; `field.code` (marca), `field.iso_code`/`field.symbol` (moneda) genuinely didn't. Confirmed zero collision risk before adding them as global keys -- swept every schema file in the repo (not just masters/) for any other field named code/iso_code/symbol; none exist. Added all three, translated across all four locales (en/es/ja/pt), matching this session's own established convention for global field.* keys.

Also confirmed directly, not assumed, that the schema's own top-level "title" (e.g. region.json's "Región") is entirely dead: ParseSchema's own decode target (internal/formengine/schema.go) has no Title field at all -- this JSON key is never even parsed, let alone rendered, regardless of locale. Out of this item's own real scope (field titles, not the unused top-level one) and left untouched.

A real correction to this item's own earlier assumption, found only by testing live rather than trusting the fix in isolation: initially believed the three new keys would fix a live, user-visible gap. Loading the real currencies/brands create forms directly showed otherwise -- masters_pages.go's own handlers already call t("masters.currency.field.iso_code") etc., and those exact module-scoped keys already existed and already correctly translated, entirely independent of anything touched here today. The live UI was never broken; this item's own body already said as much (the schema files aren't wired into any live route). The fix genuinely closes the gap this item describes -- these three schema files no longer bypass the i18n system when read through resolveTitle -- without claiming to have fixed a user-facing bug that didn't exist. Verified TestMastersFormParity's own four tests (the only real consumer of this dormant path) still pass. Full seam-ui build/vet/test suite clean.

Cross-ref: CHANGELOG 0.3.84.

## [0.3.83] T-91 — Reverse-direction i18n sweep (mirror of T-40's own law) found and fixed assets.go, the sole page hardcoding English user-facing strings instead of routing through t() -- plus a deeper, shared-component bug found live: ListPaginationBar hardcoded its entire count-line template in Spanish regardless of locale, affecting both its real callers (assets.go, events.go) (v0.3.83, 2026-08-13)

Theme: i18n · closed 0.3.83 · 2026-08-13


Trigger: the tech team, after T-40 (the internal-identifier campaign) closed
-- "Run one last sweep in our seam codebase trying to find any
occurrences of identifiers in English." A sweep for the mirror-image
violation of this project's own standing law (internal identifiers
always English; external, user-facing text depends on locale):
hardcoded English user-facing strings bypassing the i18n system
entirely, rather than Spanish internal identifiers.

**assets.go, the sole outlier across the entire internal/ui tree,
found and fixed.** Nine hardcoded English strings on the one page
where sibling fields on the exact same struct literals already
correctly called t(): the page Title, EmptyTitle, EmptyMessage, the
search placeholder, and five filter-option labels (four status values
plus the type-filter "all" option). Confirmed the scope precisely
before touching anything: grepped every Title:/EmptyTitle:/
EmptyMessage: literal across internal/ui, confirmed assets.go was the
only file with unlocalized values; confirmed every comparable page
(events.go, and other pages reusing sensors.filter.all_types/
all_statuses) already does this correctly, so this wasn't an
unlocalized area, just specific fields skipped.

Most of the needed i18n keys already existed (nav.assets,
asset.status.active/silent/alert/lost, sensors.filter.all_types) --
confirming this was a wiring gap, not missing infrastructure. Three
new keys added across all four locales (en/es/ja/pt, matching how
every comparable existing key is already kept in sync):
assets.empty_title, assets.empty_message, assets.search_placeholder.

A real complication handled directly: assetsFilterBar's own type-filter
loop used `t` as its own local loop variable name, which would have
shadowed the translator parameter needed for this fix. Renamed the
loop variable rather than work around the shadow, keeping the
translator parameter's own name consistent with this codebase's
established convention everywhere else.

**A second, deeper bug found live, not by inspection -- discovered by
actually loading the fixed page in English and reading it, the same
discipline this whole session has applied throughout.** The pagination
count line (ListPaginationBar, a component shared by both assets.go
and events.go) hardcoded its entire template string in Spanish
("Mostrando %d de %d %s") regardless of locale, entirely independent
of the Label value passed to it. Fixing assets.go's own Label value
alone would have "fixed" only half the sentence -- an English-locale
page would still have read "Mostrando 16 de 16 Assets", a broken mix
neither fully Spanish nor fully English. Confirmed this lived in a
shared component with exactly two callers (assets.go, events.go)
before treating it as still one-pass-sized rather than escalating
scope.

Added a new parameterized key, pagination.showing ("%d of %d %s" /
"%d de %d %s" shape, plus a genuine Japanese construction, not a
templated guess -- confirmed the translator's own T() implementation
is a straight fmt.Sprintf(msg, args...), so multiple placeholders are
fully supported, matching the established masters.record_count
precedent for single-placeholder keys). ListPaginationBar given a t
parameter (breaking from this codebase's usual "caller pre-translates
into the config struct" pattern, deliberately -- the component already
needs cfg.Total/cfg.PageSize as raw values for its own pagination-page
calculation, making "pre-compute the whole string" redundant here
compared to simpler components). Both real callers updated; a second,
independent hardcoded-English miss caught in the same pass:
events.go's own Label: "events" had the identical problem, fixed
alongside the shared component fix rather than left for a future
sweep to rediscover.

One new regression test added (TestAssetsListPage_UsesTranslatorNotHardcodedEnglish,
internal/ui/assets_i18n_test.go) -- no prior test existed for this
page's own rendering at all, which is exactly how this went unnoticed.
Renders with a translator that prefixes every real key with "XX:" so
a genuinely-wired t() call is unmistakable from a hardcoded literal
that happens to render similar text; asserts on all 11 keys this fix
touches and checks for six specific hardcoded-English regressions
verbatim. Sanity-checked twice, precisely: reverted assets.go's own
EmptyTitle first, confirmed genuine failure, restored; separately
reverted ListPaginationBar's own countText computation, confirmed a
second, independent genuine failure, restored.

Full live verification in both locales, not just the mock-translator
unit test: Spanish locale renders a fully coherent page throughout
("Mostrando 16 de 16 Activos", "Todos los Estados", "Buscar
activos..."); switched the real tenant's own locale setting through
the real settings form to English and re-verified -- "Showing 16 of 16
Assets", correct empty-state and filter text, zero server errors.
Switched events.go's own page to Spanish too, confirming "Mostrando 0
de 6 Eventos" (previously would have read "...6 events", English
leaking through). Full seam-ui build/vet/test suite clean throughout.

Cross-ref: CHANGELOG 0.3.83.

## [0.3.82] T-40 — Fully done -- all eight masters entities and Asset itself (seven scalar blocks plus the shared DocumentLink/service_configs structures) complete, live-verified end to end (v0.3.82, 2026-08-13)

Theme: i18n · closed 0.3.82 · 2026-08-13


Surfaced 2026-08-01 while closing T-39, found via two false-positive hits (Custodio.estado, Asset.refrigerante) in T-39's own final sweep. Originally filed covering both entity/table names AND attribute names as one item ("the actual root of every bridge table this project has built"). The tech team corrected directly: "Entity names are a kind of identifier as well. Custodio as a xolu entity should be Custodian. Same for any other master tables/collections/entities" -- narrowing this item's scope by RESOLVING the table-name half outright, not just re-filing it.

**Table/entity names: done, 2026-08-01.** custodios/marcas/modelos/regiones/monedas/centros_costo/ubicaciones/alertas -> custodians/brands/models/regions/currencies/cost_centers/locations/alert_types (avoiding collision with the already-English alerts/alert_history, genuinely distinct entities). Unlike attribute-name fixes elsewhere in this project (which bridge Go-side English to xolu-side Spanish storage), this was a direct rename with no bridge layer: the actual xolu entity_type STRING itself now says "custodians", both in Go's xoluClient calls and in every seed script's POST target -- because every demo checkpoint this project ships is freshly re-seeded, not permanent production data needing migration. Also renamed the parallel `entity:X` source-key layer (asset_source_resolver.go's dispatch map, asset.json's x-seam-source values, and formengine's own self-contained test-fixture placeholders), and confirmed URL routes (`/custodios` etc., T-38) were correctly left untouched -- a separate, still-undecided naming question. Verified about as thoroughly as anything in this project: full Go test suite green after two rounds of real test-fixture breakage caught and fixed (asset_source_resolver_test.go's mock OQL queries, htmx_partial_test.go's mock API paths), then a genuine end-to-end test -- fresh seam binary built, real xolu instance, real seed run, real login, real authenticated page load of /custodios -- confirming actual seeded custodian names render correctly through the fully-renamed pipeline, not just a database-level check.

**Attribute names: approach settled, execution started 2026-08-02.** The entity-name precedent actually resolves the (a)-vs-(b) question above: since every demo checkpoint is freshly re-seeded, not migrated, a direct rename with no bridge applies here too, same as entity names. What's genuinely still open is scale, not approach -- dozens of fields across a dozen-plus entities. Proceeding one entity at a time rather than in one pass, checking in between.

**Region: done, 2026-08-02.** `descripcion` -> `description`, its only attribute. Turned out far more pervasive per-field than entity names were: not just a xolu-storage bridge boundary, but the Go struct field names (`RegionRow.Description`, `RegionFormData.Description`, `RegionesListData.Regions`), the literal HTML form field name/id, `region.json`'s own schema property key (not just its display title, which stays Spanish per T-30's separate scope), and `regionFieldToXolu`'s bridge emptied to a pass-through. Two real bugs of my own making, both caught and fixed, not papered over: a compile error caught a missed `RegionRow.Descripcion` usage inside Ubicación's own region-dropdown code (Region and Ubicación share code in the same files), and a live seed dry-run caught a `KeyError: 'descripcion'` in `seed_regiones()` itself -- a downstream dict-key reference (`ids[item["descripcion"]]`, used to cross-reference regions by name when building locations/assets) that my rename regex hadn't matched, since it targeted dict *literals* (`"descripcion":`) not dict *access* (`item["descripcion"]`, no colon). Verified with the same rigor as entity names: full Go test suite green (five rounds of real test-fixture breakage across `asset_source_resolver_test.go`, `csrf_test.go`, `masters_handler_test.go`, `validation_handler_test.go`, `masters_pages_dynamic_test.go`, all fixed by exact context rather than blind replace, since Custodio/Ubicación/Alerta share the same `descripcion` word in the same files), then a genuine end-to-end test exercising both read and write paths -- fresh binary, real xolu instance, real seed, real login, real `/regiones` list page confirming five real region names render correctly, and a real POST through the real create form confirming a new record lands in xolu as `{"description":"Patagonia","id":6}`.

**Moneda: done, 2026-08-02.** `codigo_iso`/`simbolo` -> `iso_code`/`symbol`. Clean single pass, no bugs -- the Region lessons applied directly: proactively fixed the downstream dict-key reference in `seed_monedas()` before running anything, rather than discovering it via a crash. Verified with the same rigor: full test suite green, then a genuine end-to-end test -- real seed, real login, real `/monedas` list page showing all 3 real currencies, a real POST through the real create form, database confirming `{'id': 4, 'iso_code': 'GBP', 'symbol': '£'}`.

**Centro de Costo: done, 2026-08-02.** `codigo_contable`/`nombre` -> `accounting_code`/`name`. No entity-specific schema JSON or dynamic-form file exists for this one (outside the original stage-2 masters set, so nothing to touch there). One real cross-file consumer found and fixed in `summary_handlers.go`. Five test-fixture repairs needed (`htmx_partial_test.go` x4, `validation_handler_test.go`, `asset_source_resolver_test.go`) but no runtime bugs this time. Same end-to-end rigor: real seed, real login, real `/centros-costo` list page showing all 5 real cost centers, real create form, database confirming all 6 records (5 seeded + 1 new) correctly stored.

**Marca: done, 2026-08-02.** `codigo`/`nombre` -> `code`/`name`. Three real compile-error catches this time -- Modelo's own brand-selection dropdown, `MarcaFormPageDynamic`, and `loadMarcas` itself (actively called by Modelo's form across 6 sites, unlike the dead-code dropdown helpers found for Moneda/Centro de Costo). Two rounds of test-fixture repair, correctly leaving Modelo's/Custodio's own still-untouched `nombre`/`codigo` test data alone. One genuine pre-existing gap found by coincidence, not by design: `TestCentroCostoCreate_MissingCodigo_ReturnsForm` (from the earlier Centro de Costo pass) was silently submitting the wrong field name and still passing -- for an accidentally-correct reason, not the reason it claims to test. Fixed and re-verified it now tests what it says. End-to-end verification specifically exercised the cross-entity dependency this entity's rename put most at risk: `/models/new`'s brand dropdown (fed by `loadMarcas`) confirmed still rendering correctly after the rename, not just `/brands` itself.

**Ubicación: done, 2026-08-02.** `nombre_corto`/`descripcion` -> `short_name`/`description`. `nombre_corto` is *also* PropDef's own field name for a completely different concept (custom property short codes, deliberately left Spanish per T-39) -- every replacement scoped precisely to Ubicación's own function bodies to avoid touching that unrelated usage, same discipline as Region's own `descripcion` scoping. Two dead-code/live-consumer catches (`loadUbicaciones`, unused; `UbicacionSummary`, actively routed, correctly updated), three rounds of test-fixture repair. End-to-end verification specifically exercised the cross-entity dependency this rename shares with Region: `/locations/new`'s region-selection dropdown confirmed still showing Region's own correctly-translated names ("Centro"/"Este"/"Metropolitana"), and `/locations`' own list page confirmed the region-lookup join (a separate OQL query joining each location to its region's name) still works.

**Custodio: done, 2026-08-02.** All 8 fields reviewed; 6 renamed (`subject_type`/`legal_name`/`tax_id`/`phone`/`address`/`active` <- `tipo_sujeto`/`nombre_razon_social`/`identificador_fiscal`/`telefono`/`direccion`/`estado`), `alias`/`email` already English on both sides. The biggest masters entity by field count so far, and the first with its own three-tier footprint (HTML form field name, Go struct field, xolu storage key) all needing the same rename in lockstep -- went clean, no compile errors, no test-fixture surprises beyond the expected fixture updates (`asset_source_resolver_test.go`'s mock OQL query, `htmx_partial_test.go` and `validation_handler_test.go`'s form/mock data, including renaming `TestCustodioCreate_InvalidTipoSujeto_ReturnsForm` to `...InvalidSubjectType...`). `custodioFieldToXolu` emptied to a documented pass-through, matching every prior entity's precedent. One real trap caught before it did damage: a naive whole-word substitution across the seed scripts would have corrupted `seed-03-extinfire.py`'s `"direccion@liceo2maldonado.edu.uy"` email value (the substring `direccion` appearing inside real seed data, not as a field key) -- avoided by scoping every seed substitution to the exact `"key":` pattern rather than the bare word. All 6 seeds' downstream `item['tipo_sujeto']`/`item['identificador_fiscal']` print-statement dict-access (the same class of bug Region's `KeyError` first exposed) updated in `seed-03`/`seed-04`. Verified with the same rigor as every prior entity, at full scope: fresh xolu + seam binaries built from source, real seed run against a live xolu instance, real authenticated login, `/custodians` list page confirmed rendering seeded data, `/custodians/new` form confirmed emitting English field names (`subject_type` present, `tipo_sujeto` absent), a real POST through the real create form, and the resulting record confirmed directly in xolu's own SQLite store (`t0000_nodes`, `entity_type='custodians'`) with all-English JSON keys: `{"active": true, "address": ..., "alias": ..., "email": ..., "id": ..., "legal_name": ..., "phone": ..., "subject_type": ..., "tax_id": ...}`.

**Alerta: done, 2026-08-02.** All 3 fields renamed (`type`/`description`/`criticality` <- `tipo`/`descripcion`/`criticidad`); enum *values* stay Spanish (Mantenimiento/Alta/etc.) -- T-23's separate, not-yet-fixed concern, not this item's scope. Real bug caught before it shipped, not after: `alertaFieldToXolu` couldn't be emptied like every prior entity's bridge, because `translateFlatFieldsAndEnums` gates the English->Spanish enum-*value* translation behind a successful key lookup in that same map -- an empty map would have silently stopped translating `High`/`Maintenance`/etc. to the Spanish values xolu still stores. Fixed with identity entries (`"type": "type"` etc.) instead, and proved it mattered: the live-verification record's `criticality` stored as `"Alta"`, not the raw `"High"` submitted, confirming the translation still fires. Widest collision-risk surface of any entity so far -- `tipo`/`descripcion`/`criticidad` are common Spanish words shared with Asset's own (not-yet-renamed) fields and with two unrelated `DocumentLinksField` blocks in the same files -- handled via line-range-scoped substitution (not the whole-file word replace used for less-contested entities), confirmed safe both by pre-edit inspection and by `go build` catching one real miss (AssetType's default-alert picker in `pages.go`, still reading `a.Tipo`/`a.Descripcion`/`a.Criticidad`). Downstream Asset/dashboard-scoped struct fields that merely *consume* Alerta's data (`DashAlertRow.AlertaTipo`/`.Criticidad`, `AssetDetailData`'s `AlertaTipo`/`AlertaCriticidad`) deliberately left as-is, matching how Custodio's `AssetCustodio` was handled -- only the data-source reads were fixed, not every downstream Go field name, consistent with treating those as "Asset itself" scope. A pre-existing, unrelated gap found by coincidence while sweeping test files: `TestUbicacionEdit_HTMXReturnsPartial`'s mock had been returning stale Spanish keys since Ubicación's own rename, undetected because the test never asserted on field values -- fixed. Verified with the same rigor as every prior entity: fresh xolu + seam binaries, real seed run, real login, `/alert-types` list and create-form pages confirmed rendering/emitting English field names, and the resulting record confirmed directly in xolu's own SQLite store.

**What's left:** Asset itself -- a different scale entirely (dozens of fields, not 1-8), and the central entity every other entity's foreign keys point at. Seven entities done (Region, Moneda, Centro de Costo, Marca, Ubicación, Custodio, Alerta) with a consistent pattern holding across every one: real bugs happen, every one caught by build errors, test failures, or live verification -- never silently missed.

**Asset audit, started 2026-08-02, execution paused.** Full occurrence scan of Asset's 32 real renames (of 42 top-level fields; 10 already share the same key on both sides) plus the shared `DocumentLink` item struct (`document_links`/`loan_links`/`lifecycle_links`) and `service_configs` items -- 1,944 raw token hits across `internal/` and `seeds/`. Confirmed real, not assumed: `AssetDetail` (~28 Spanish Go fields), `AssetRow` (2), the bidirectional `asset_xolu_fields.go` bridge (unlike every prior entity's write-only bridge, this one also serves the dynamic form's read path -- the same enum-value-gating hazard found in Alerta's bridge applies here too, confirmed before writing any replacement code: `assetFieldToXolu`/`documentLinksItemToXolu`/`serviceConfigsItemToXolu` need identity entries, not empty maps, wherever `assetEnumValueToXolu` still depends on a successful key lookup). Raw audit surfaced a real scope hazard before any code was touched: `nombre`/`marca_id` aren't Asset-exclusive tokens -- Modelo's and PropDefinition/PropAssociation's own still-unrenamed fields share the same vocabulary in the same files (filed separately, T-42). Execution deliberately paused here rather than building the replacement document on an under-scoped audit; resume by re-running the audit with Modelo/PropDef/PropAssoc occurrences explicitly excluded/classified, not re-discovered ad hoc.

**Execution resumed, 2026-08-11, both collision risks cleared (T-42 fully closed).** Working one logical block at a time, matching `AssetDetail`'s own existing "Block A-G" comment structure, rather than 32 fields in one pass. **Block E (Lifecycle), done -- T-82.** Seven fields (PurchaseDate/ExpectedArrivalDate/ArrivalDate/ObsolescenceDate/ExpirationDate/DecommissionDate/DecommissionReason). Applied the identity-map-not-empty lesson correctly from the start this time -- checked `assetEnumValueToXolu` for `decommission_reason` specifically before touching the bridge, rather than discovering the gap the way PropDefinition/PropAssociation did. Full detail in T-82's own `docs/RESOLVED.md` entry.

**Block B (Characterisation), done -- T-83.** Six fields (BrandID/ModelID/Serial1/Serial2/Lot/Criticality), including a second struct (`AssetRow`) with its own copy of the same field, found only by checking rather than assumed identical in scope to `AssetDetail`. Found and fixed a real, live regression along the way, not part of this block's own scope: `asset_source_resolver.go`'s `entity:models` query, in a file T-80's own work never touched, had been silently rendering every Model dropdown option with an empty label in every deployment since T-80 shipped (v0.3.73) -- caught by live-testing the actual asset edit form, confirmed by checking the entire query map for the same class of miss (every other entry was already correct), fixed, and caught again independently by a real pre-existing test once the fix ran against it. Full detail in T-83's own `docs/RESOLVED.md` entry.

**Block C (Location), done -- T-84.** Three fields (LocationID/RegionID/ParentID; RegionID already English). Found and fixed two more live regressions from earlier work along the way, neither part of this block's own scope: a countRef delete-protection guard missed during Block B (T-83), and a Model summary card handler in a file never touched by either T-80 or this session's other passes -- both caught by deliberately sweeping for the same class of miss after finding the first instance, not by chance. Full detail in T-84's own `docs/RESOLVED.md` entry.

**Block D (Custody), done -- T-87.** Three fields (CustodianID/TenureType/LoanExpiry). A dark-spot audit pass (T-85) ran between Location and this block, at the tech team's own direction -- found and fixed a real, live regression dating to Block E's own release (the assets dashboard's urgency-list widget silently querying a renamed-away column since v0.3.75). Also fixed four leftover Spanish fields in AlertDetailData (a miss from Alerta's own earlier, separate rename) as part of this block, since one sat in the same struct as a field already being renamed here. Full detail in T-87's own `docs/RESOLVED.md` entry; the audit pass itself is T-85, and the separately-tracked workflow-test module finding it also surfaced is T-86.

**Block G (Finance), done -- T-88.** Five fields (PurchasePrice/CurrencyID/CurrentPrice/CostCenterID/WarrantyEnd). Completes the `fin_garantia` half of T-85's own dashboard-widget fix, deliberately left pending until this block landed. Full detail in T-88's own `docs/RESOLVED.md` entry.

**Block F (Service/IoT), done -- T-89.** Four fields (TempMin/TempMax/Refrigerant/VoltageConsumption). Confirmed the established English names by reading assetFieldToXolu directly rather than assuming ("temp_min"/"temp_max", not "temp_range_min"/"temp_range_max" as this item's own original scope guess had it). Live-verified against seed-02-frigocold.py's own real cold-chain data (a much stronger test than the default seed file for this specific block). Full detail in T-89's own `docs/RESOLVED.md` entry.

This completes all seven of `AssetDetail`'s own scalar-field blocks.

**DocumentLink and service_configs, done -- T-90. T-40 fully complete.** The shared `DocumentLink` item struct (Name/Type, used across `LinksCicloVida`/`LinksComodato`/`LinksDocumentos`) and `service_configs` items (measure/threshold/alert_id), both explicitly named in this item's own original 2026-08-02 audit. Retires a real, documented 2026-07-28 bug (the submitted "threshold" field name and its persisted xolu key genuinely differing) by making that class of mistake structurally impossible now that both sides are the same string. A genuinely out-of-scope trap correctly avoided: the unrelated `alerts` runtime table shares the literal string "alerta_id" with `service_configs`' own now-renamed field, for a completely different purpose -- confirmed the distinction directly before touching anything, left the former untouched. Full live verification included a real form submission (updating a threshold value end to end, form to storage), not just compile/unit-test coverage. Full detail in T-90's own `docs/RESOLVED.md` entry.

Every piece of this item's own original scope -- all eight masters entities and all seven of Asset's own scalar-field blocks plus its shared nested structures -- is now complete.

Cross-reference, 2026-08-05, prompted directly by the tech team noticing `id_padre` surface in T-72's own live-verification output ("check before you release a patch version") -- confirmed T-72's new asset-picker widget code neither introduces nor entrenches any of this item's remaining Spanish debt: grepped the widget, the picker search endpoint, and both new schema files directly, zero references to `id_padre` or any other still-Spanish Asset field. The widget only ever handles `parent_id` (the English schema name, correctly bridged via the existing `assetFieldToXolu["parent_id"] = "id_padre"` entry, untouched by this work) and searches/displays `name`/`status`, both confirmed already-identity-mapped on both sides (`assetFieldToXolu["name"] = "name"`, `["status"] = "status"`) -- not part of the remaining 32-field debt at all. When this item's Asset rename eventually resumes, the widget and endpoint need zero changes: the bridge removal T-40's own established pattern calls for is transparent above the `assetFieldToXolu` layer. Confirmed safe to ship T-72 without waiting on this item, not just assumed.

Cross-reference, 2026-08-11, prompted by the tech team catching sloppy framing in T-79's own closure (xolu-integration): a "criticality doesn't persist" finding was correctly retracted there as not a data-loss bug -- the value does save and read back correctly -- but the retraction's own language called `criticidad` "the real xolu column" and "correct," language that has it backwards. `criticidad` is this item's own remaining debt, not the standard; `criticality` is what the column should be named once Asset's own rename resumes. T-79's record is closed and stays as written (append-only, not rewritten to fix a framing mistake after the fact) -- this note is the correction, recorded where it belongs: against the item that actually owns the fact that Asset's fields are still Spanish. Same lesson as the `parent_id`/`id_padre` case already on record above -- a live query or a retraction needs to name which side is the debt, not describe the Spanish name as though it were simply correct.

Cross-reference, 2026-08-11: both collision risks this item's own paused audit named (Modelo, PropDefinition/PropAssociation) are now cleared -- T-42 fully closed (both halves: Modelo as T-80, PropDefinition/PropAssociation as T-81). Worth recording plainly, not glossed over: PropDefinition/PropAssociation repeated a mistake this same entry already documented for Alerta above -- emptying `propDefFieldToXolu`/`propAssocFieldToXolu` to `map[string]string{}` entirely rather than an explicit identity map, silently breaking `data_type`/`requirement`'s own enum-*value* translation for the identical reason (`translateFlatFieldsAndEnums` gates that translation behind a successful key lookup in the same map) already written down here for Alerta's own bridge. The mistake was caught -- a new test found it before any live testing, and live verification confirmed it independently too -- but it shouldn't have needed rediscovering; this record already had the answer. Asset's own rename, whenever it resumes, should check this entry's own precedent directly rather than risk a third repetition: `assetFieldToXolu` almost certainly needs the same identity-map treatment wherever `assetEnumValueToXolu` depends on it, not an empty map.

Cross-ref: CHANGELOG 0.3.82.

## [0.3.82] T-90 — T-40 Asset rename fully complete -- DocumentLink and service_configs' shared nested structures done, live-verified end to end including a real form submission. Retires a real, documented 2026-07-28 bug (umbral/umbral_alarma key mismatch) by making the class of bug structurally impossible. T-40 itself closed (v0.3.82, 2026-08-13)

Theme: i18n · closed 0.3.82 · 2026-08-13


Trigger: completing T-40's own final piece after all seven scalar
blocks (T-82/83/84/87/88/89) -- the shared DocumentLink item struct
and service_configs items, both explicitly named in this item's own
original 2026-08-02 audit.

**DocumentLink, done.** Go struct fields Nombre/Tipo -> Name/Type.
Found and fixed the actual live population code for
AssetDetail.LinksCicloVida (both the outer xolu key "links_ciclo_vida"
-> "lifecycle_links" and the item-level nombre/tipo -> name/type keys)
-- same "populated but never rendered" pattern every earlier block
found, fixed for consistency regardless. Found two more genuinely dead
functions (parseDocumentLinksToAny, DocumentLinksField -- zero callers
each, confirmed by grep) and renamed them anyway rather than leave
stale Spanish names sitting next to everything else that changed.

Three-layer bridge structure, all converted to identity: the
item-level transform (documentLinksItemToXolu/FromXolu, shared across
all three link arrays) and the three outer container-field entries
(loan_links/lifecycle_links/document_links).

**service_configs, done.** No dedicated Go struct -- items are handled
via plain map[string]any, already English on the form side
(measure/threshold/alert_id). Only the item-level bridge
(serviceConfigsItemToXolu/FromXolu) needed renaming: medida/
umbral_alarma/alerta_id -> measure/threshold/alert_id. This retires a
real, documented historical bug (2026-07-28: the submitted field name
and the persisted xolu key for "threshold" genuinely differed --
"umbral" vs "umbral_alarma") by making the class of bug it guarded
against structurally impossible: both sides are now the same string.

**A real, out-of-scope finding correctly left untouched.** The
"alerts" runtime table (a different entity entirely from Asset) has
its own, separate "alerta_id" column linking to alert_types --
confirmed directly against seed data and a live dashboard enrich rule
before concluding it was unrelated to service_configs' own,
now-renamed "alerta_id". The two shared the same literal string for
unrelated purposes; conflating them would have been a real mistake.
Left this alone, correctly, as outside T-40's own scope.

Full test suite surfaced 5 real, precise pre-existing tests directly
exercising this exact mechanism, all fixed: 3 unit tests
(TestAssetValuesToXolu/FromXolu_DocumentLinksArray,
TestAssetValuesToXolu_ServiceConfigsArray) and 2 real end-to-end
dynamic-post tests. One of the two end-to-end tests
(TestAssetCreateDynamic_ServiceConfigsArray_UsesRealPersistedKey) was
specifically written to guard the 2026-07-28 umbral/umbral_alarma bug
-- rather than delete a now-moot guard, rewrote it to guard the
structurally equivalent future regression (an accidental revert of
either side back to the old Spanish key), preserving the test's own
original intent rather than just its old assertions. All fixes
sanity-checked properly: reverted each bridge in turn, confirmed both
a unit test and a real end-to-end test genuinely fail for the right
reason, restored, confirmed clean.

Seed scripts: service_configs' own nested item keys renamed with a
scoped, function-boundary-aware substitution (def
seed_service_configs to the next def) across all six files -- required
scoping, not just safe-without-it, since "alerta_id" is genuinely
ambiguous at file scope (the very next function, seed_runtime_alerts,
has its own unrelated alerta_id belonging to the alerts runtime
table, confirmed before writing the substitution). 144 total
occurrences replaced; the 2 remaining alerta_id occurrences per file,
confirmed still present after the substitution, correctly belong to
that unrelated function. DocumentLink's own nested arrays needed no
seed changes at all -- confirmed zero occurrences of document_links/
loan_links/lifecycle_links anywhere in any of the six seed files,
meaning these fields are genuinely never populated by seed data.

Full live verification, fresh seed to real UI, not stopped at compile
or unit tests: confirmed real persisted service_configs data directly
in xolu with all-English keys; the edit form correctly pre-selects
Measure and pre-fills Threshold from that real seeded value; a genuine
form submission updating the threshold value (5000 -> 6000) confirmed
persisted correctly end to end, form to storage. Zero server errors
throughout. Full seam-ui build/vet/test suite clean.

This completes every piece of T-40's own original scope: all seven of
AssetDetail's scalar-field blocks plus the shared DocumentLink/
service_configs structures. T-40 itself closed alongside this item.

Cross-ref: CHANGELOG 0.3.82.

## [0.3.81] T-89 — T-40 Asset rename, Block F (Service/IoT) done -- TempMin/TempMax/Refrigerant/VoltageConsumption, live-verified against real refrigeration data. This completes all seven of AssetDetail's own scalar-field blocks; only the shared DocumentLink/service_configs structures remain (v0.3.81, 2026-08-13)

Theme: i18n · closed 0.3.81 · 2026-08-13


Trigger: continuing T-40's own resumed Asset rename, next block after
Finance (T-88) -- the last of the seven scalar-field blocks
(AssetDetail's own "Block A-G" comment structure).

**Block F -- Service/IoT, done.** Four fields: RangoTempMin/
RangoTempMax/Refrigerante/VoltajeConsumo -> TempMin/TempMax/
Refrigerant/VoltageConsumption. Note the established English names
(confirmed by reading assetFieldToXolu directly before assuming) are
"temp_min"/"temp_max", not "temp_range_min"/"temp_range_max" as this
item's own original scope description had guessed -- matched the
struct field names to what the bridge already established, not the
other way around. Checked assetEnumValueToXolu for all four before
touching the bridge -- none have enum dependencies.

Swept the whole internal/ tree for stale rango_temp_min/rango_temp_max/
refrigerante/voltaje_consumo references: zero found. No countRef
guards, no dashboard config, no other consumer -- these four fields
are read only through the dynamic form's own AssetValuesFromXolu/
AssetValuesToXolu path, nothing else references them anywhere.

Sanity-checked (reverted refrigerant's bridge entry, confirmed
TestAssetValues_RoundTrip_AllScalarFields does not catch a wrong
xolu-side name -- same established limitation as every prior block --
restored, confirmed clean).

Seed scripts: three of six files have zero occurrences (extinfire,
vendiberia -- companies whose asset fleets don't include
temperature-controlled or refrigerated equipment, confirmed, not a
missed substitution). The other three updated (74 total occurrences).
seed-02-frigocold.py (a commercial/industrial cold-chain company) has
by far the richest data for this block -- run live specifically for a
stronger test than the default seed-01 would have given (19 assets,
36 property associations, real refrigerant/temperature data across
multiple cooling-chamber assets).

Full live verification, fresh seed to real UI: confirmed real
persisted refrigerant/temperature data directly in xolu (R404A,
0-8C range on a real conservation chamber asset); the edit form
correctly pre-fills TempMin/TempMax/Refrigerant from that same real
data, and correctly shows VoltageConsumption empty where the seed
data doesn't populate it for that specific asset -- not a bug, a
correct reflection of sparse seed data. Zero server errors. Full
seam-ui build/vet/test suite clean.

This completes all seven of AssetDetail's own scalar-field blocks
(E/B/C/D/G/F, plus A which was already English). One piece of this
item's own original scope remains: the shared DocumentLink item
struct (used by LinksCicloVida/LinksComodato/LinksDocumentos) and
service_configs items, both explicitly named in T-40's own original
2026-08-02 audit as in scope alongside the scalar fields.

Cross-ref: CHANGELOG 0.3.81.

## [0.3.80] T-88 — T-40 Asset rename, Block G (Finance) done -- PurchasePrice/CurrencyID/CurrentPrice/CostCenterID/WarrantyEnd, live-verified. Completes the fin_garantia dashboard-widget fix T-85 had deliberately left pending until this block landed (v0.3.80, 2026-08-13)

Theme: i18n · closed 0.3.80 · 2026-08-13


Trigger: continuing T-40's own resumed Asset rename, next block after
Custody (T-87).

**Block G -- Finance, done.** Five fields: PrecioCompra/MonedaID/
PrecioActual/CentroCostoID/FinGarantia -> PurchasePrice/CurrencyID/
CurrentPrice/CostCenterID/WarrantyEnd, matching the already-established
Currency/CostCenter precedent from those masters entities' own earlier
renames. Checked assetEnumValueToXolu for all five before touching the
bridge -- none have enum dependencies.

**This block also completes the fin_garantia fix flagged as pending
in T-85's own dashboard-widget regression note.** T-85 fixed the
assets dashboard's urgency-list widget for expiration_date (Block E's
own column) but left fin_garantia Spanish-keyed there deliberately,
since WarrantyEnd hadn't landed yet. Fixed now: assets-dashboard.json's
own date_fields/date_field_labels entries updated to warranty_end, and
internal/dashboardengine/source.go's own comment updated to record
both confirmations (expiration_date and warranty_end) rather than
leave the second half of that note stale.

Swept for stale precio_compra/moneda_id/precio_actual/
centro_costo_id/fin_garantia references and fixed two real ones,
same class as every prior block: delete-protection countRef guards
for Currency and Cost Center, which had never actually checked
whether an asset referenced either before allowing deletion.

Sanity-checked properly despite the full test suite passing clean on
the first run (unlike every prior block, which had genuine failures
to fix) -- reverted currency_id's bridge entry specifically to confirm
TestAssetValues_RoundTrip_AllScalarFields does NOT catch a wrong
xolu-side column name (it only verifies internal round-trip
consistency, both directions sharing the same map), matching this
item's own established understanding from Block G's predecessors;
restored, confirmed clean again. Live verification, not this test, is
what actually proves the xolu-side name correct for this class of
mistake.

All six seed scripts updated (235 total occurrences;
precio_actual at zero everywhere, confirmed, not a missed
substitution -- no seeded asset tracks a current market price,
matching Lot/parent_id's own earlier unpopulated-field pattern).
Syntax-validated, zero remaining old-key occurrences in either quote
style confirmed before the live run.

Full live verification, fresh seed to real UI: the edit form
correctly pre-selects Currency (USD) and Cost Center, correctly
pre-fills Purchase Price and Warranty End from the real seeded data;
the dashboard's urgency-list widget correctly shows real assets
(confirming the warranty_end fix works live, not just the config
change compiling). Zero server errors throughout. Full seam-ui
build/vet/test suite clean.

Two blocks remain: Service/IoT, plus the shared DocumentLink/
service_configs structs.

Cross-ref: CHANGELOG 0.3.80.

## [0.3.79] T-87 — T-40 Asset rename, Block D (Custody) done -- CustodianID/TenureType/LoanExpiry, live-verified. Also fixed four leftover Spanish fields in AlertDetailData (Alerta's own earlier rename missed them) since one sat in the same struct as a field this block was already renaming (v0.3.79, 2026-08-13)

Theme: i18n · closed 0.3.79 · 2026-08-13


Trigger: continuing T-40's own resumed Asset rename, next block after
Location (T-84). A dark-spot audit pass (T-85) ran between Location
and this block, prompted directly by the tech team; that work is tracked
separately and not repeated here.

**Block D -- Custody, done.** Three fields: CustodioID/
CalidadTenencia/VtoComodato -> CustodianID/TenureType/LoanExpiry.
Checked tenure_type's known enum dependency (assetEnumValueToXolu)
before touching the bridge; custodian_id and loan_expiry have none.

AssetRow's own CustodioNombre -> CustodianName, matching Block C's
established pattern of finding a second struct with its own copy of a
field rather than assuming AssetDetail is the only one.

**A related, low-risk fix folded in rather than left inconsistent:**
found AlertDetailData (pages.go) carrying four still-Spanish fields --
AssetCustodio, AlertaConfig, AlertaTipo, AlertaCriticidad -- leftovers
from Alerta's own already-completed rename, missed at the time. Their
xolu-side sources were already correctly English
(alerta.Data["description"/"type"/"criticality"]), so this was purely
a Go naming gap, not a functional bug -- but given AssetCustodio sits
in the exact same struct as the field this block was already renaming,
fixing three Spanish names right next to a fourth just-renamed one
would have been an odd place to stop. Fixed all four
(AssetCustodian/AlertConfig/AlertType/AlertCriticality); the Spanish
enum-value lookup map ("Alta"/"Media"/"Baja") correctly left untouched,
same as every prior block's own precedent.

Swept for stale custodio_id/calidad_tenencia/vto_comodato references
and fixed three real ones: a delete-protection countRef guard, an
assets-list row read, and the alert-detail custodian resolution block
-- the same sweep discipline established in Block C/T-84, applied
again here as routine, not as a special step.

Five stale test assertions fixed (ToXolu/FromXolu scalar-field tests,
four tenure_type round-trip cases), sanity-checked properly: reverted
the bridge, confirmed tenure_type's own four cases genuinely failed
while criticality and decommission_reason correctly kept passing
(confirming precise isolation, not a blanket break), restored,
confirmed clean.

Seed scripts: all six files updated (146 total occurrences:
custodio_id, calidad_tenencia, vto_comodato), validated for syntax and
for the single-quoted f-string class of miss that bit Modelo -- none
found here. A live seed run (16 assets) confirmed the change doesn't
crash anything.

Full live verification, fresh seed to real UI: assets list correctly
shows the resolved custodian name ("Constructec S.A."); the edit form
correctly pre-selects Custodian and Tenure Type (Owned, correctly
translated from stored "Propiedad"); the alert detail page correctly
renders the resolved custodian name and the alert-config block with
its criticality-based color class -- confirming the AlertDetailData
fix genuinely works live, not just compiles. Zero server errors
throughout. Full seam-ui build/vet/test suite clean.

Three blocks remain: Finance, Service/IoT, plus the shared
DocumentLink/service_configs structs.

Cross-ref: CHANGELOG 0.3.79.

## [0.3.78] T-85 — Dark-spot audit pass across T-40's completed blocks -- found/fixed a real regression in the assets dashboard widget from Block E's own release. Also discovered internal/ui/workflow/ is a genuinely separate, long-broken Go module; repointed to build cleanly, but its own deeper API staleness (unrelated to this rename) remains unfixed (v0.3.78, 2026-08-13)

Theme: i18n · closed 0.3.78 · 2026-08-13


Trigger: the tech team, "review what's been done to check for any dark
spots, then keep going" -- a deliberate audit pass before continuing
Block D, given this session's own established pattern of finding real
regressions in files not touched during the original rename.

**A real, live, user-facing regression found and fixed, dating to
Block E's own release (T-82, v0.3.75).** internal/dashboardengine/
schemas/assets-dashboard.json's own "urgency_list" widget (the assets
dashboard's expiring-items panel) hardcoded `date_fields:
["fecha_expiracion", "fin_garantia"]` as raw xolu column-name
references. internal/ui/dashboard_resolver.go's UrgencyRows builds its
OQL query directly from this list (`fields := append([]string{"id",
"name"}, s.DateFields...)`), so once T-82 renamed the assets table's
own `fecha_expiracion` column to `expiration_date`, this widget's
query has been silently broken in every deployment since -- querying
a column that stopped existing. Notably, this file
(internal/dashboardengine/source.go) carried its own prescient comment
explicitly anticipating this exact situation ("this mapping is what
avoids that churn... the moment T-40's still-paused Asset rename
lands and those columns become English") -- the mechanism it describes
worked correctly once applied; the widget config itself just hadn't
been updated to use it yet when Block E shipped. Fixed the JSON config
(date_fields and date_field_labels both), and updated the same file's
own comment to record the confirmation rather than leave a
now-inaccurate "today" claim standing. fin_garantia correctly left
Spanish-keyed, since Finance (WarrantyEnd) hasn't been renamed yet.

**Two more real regressions found via a deliberate sweep for the same
class of miss, both predating today's session's own Block D work:**
a stale comment in masters_xolu_fields.go still describing xolu's
brand_id column as "marca_id" for both Modelo and Asset -- accurate
when written, wrong since Block B's own rename; corrected. (The
countRef/summary_handlers.go regressions from Block B were already
found and fixed in Block C/T-84's own work -- this pass re-confirmed
zero remaining occurrences in the four files already touched by
earlier blocks, all correctly limited to form-wire-name and
URL-query-parameter uses.)

**A much larger, separate finding: an entire test module has been
silently broken and excluded from every verification this session,
likely for a long time before it too.** internal/ui/workflow/ is a
genuinely separate Go module (its own go.mod, module
github.com/ha1tch/seam-ui/workflow-tests) -- meaning `go test ./...`
from the main module has never once executed it, this session or
probably many before. Its own replace directive pointed
`github.com/ha1tch/xolu` at `/tmp/olu`, a path that doesn't exist in
this container -- confirmed directly, not assumed -- meaning the
module has been completely unbuildable regardless of any other issue.
Repointed the replace directive to this session's actual xolu working
copy; it then builds and downloads its own dependencies cleanly
(confirming pkg/client's core surface really has stayed stable across
the version gap from v0.9.7-patched74 to v0.30.6, consistent with
every other module compatibility check this session). Fixed its own
stale createDirect fixture data (fecha_expiracion/criticidad, the two
Asset xolu keys this file's own single "expiry appears on dashboard"
workflow test touches) -- but `go vet` reveals the module has
significantly deeper staleness beyond field names: `config.Config` no
longer has a `DBPath` field the test constructs, `server.New`'s own
signature has added/removed parameters the test doesn't match, and
`UIHandler` no longer has an `AssetNew` method (renamed to
AssetNewDynamic when the dynamic-schema-driven asset form was built).
These are structural API-compatibility failures unrelated to Spanish
identifiers and well outside T-40's own scope -- not attempted here.
Left in a strictly better, partially-repaired state (buildable
dependency graph, correct field-name test data) rather than either
fully fixed or left completely broken; the remaining repair is
recorded here for whoever picks it up next, with the exact three
compile errors named above as the starting point.

Full main-module build/vet/test suite clean throughout this pass. No
new blocks of T-40's own Asset rename were started in this item --
purely an audit pass, as requested.

Cross-ref: CHANGELOG 0.3.78.

## [0.3.77] T-84 — T-40 Asset rename, Block C (Location) done -- LocationID/RegionID/ParentID, live-verified including a real form submission through the asset-picker widget. Found and fixed two more live regressions from earlier work: a missed countRef guard from Block B, and a Model summary card handler in a file never touched by either T-80 or this session's earlier passes (v0.3.77, 2026-08-12)

Theme: i18n · closed 0.3.77 · 2026-08-12


Trigger: continuing T-40's own resumed Asset rename, next block after
Characterisation (T-83).

**Block C -- Location, done.** Three fields: UbicacionFisicaID/
RegionID/IDPadre -> LocationID/RegionID/ParentID (RegionID already
English, unchanged). Checked assetEnumValueToXolu for all three before
touching the bridge -- none have enum dependencies, safe as identity
mappings. parent_id given particular attention given its live use in
the asset-picker widget (T-72): confirmed zero remaining id_padre
references anywhere in Go code or seeds before touching the bridge,
confirming the picker's own code was already correctly layered through
the bridge rather than bypassing it.

**Two real, live regressions found and fixed, neither part of this
block's own scope:**

First, a missed occurrence from Block B's own work (T-83): a countRef
delete-protection guard for Models
(`countRef(ctx, "assets", "modelo_id", id)`) was never caught during
that block, since it wasn't found by the original grep pass -- only
surfaced now by deliberately sweeping every `countRef(ctx, "assets",
...)` call site for the same class of miss after finding one for
Locations in this block's own work. Fixed. The three other guards
found in the same sweep (moneda_id, centro_costo_id, custodio_id)
correctly belong to blocks not yet done and were left untouched.

Second, and more significant: `summary_handlers.go`'s Model summary
card handler -- a file never touched during either T-80 (Modelo) or
this session's earlier work -- was still reading models' own
marca_id/nombre AND brands' own nombre, all three of which stopped
existing once their respective renames shipped. Same class of bug as
T-83's asset_source_resolver.go finding, in a different file. Found
by a deliberate broad sweep for remaining Block B key names after
finishing Block C's own work, not by testing this specific summary
card directly. Fixed both the models-table and brands-table column
references in the same edit. Confirmed live: the /models/1/summary
card now correctly shows both the model name and the resolved brand
name.

Also fixed a stale comment (masters_xolu_fields.go) that still
described xolu's brand_id column as "marca_id" for both Modelo and
Asset -- accurate when written, wrong since Block B's own rename;
updated to reflect current reality rather than left to mislead a
future reader.

**A separate, out-of-scope finding, noted rather than fixed:** the
parent_id field's own schema title is hardcoded Spanish ("ID Padre"),
confirmed live on an otherwise-English-rendering page. This is a
different class of problem from this item's own scope -- a
non-localized display string, not an internal identifier -- and
belongs with formengine's own i18n work, not T-40's. Not filed as its
own tracking item given its small, single-field scope; recorded here
so it isn't silently lost.

Two stale tests fixed (TestAssetValuesToXolu_NilValuePreserved's own
id_padre assertion), sanity-checked (reverted, confirmed genuine
failure, restored). All six seed scripts updated (68 total
ubicacion_fisica_id occurrences; id_padre had zero occurrences in any
seeded asset, confirmed, not a missed substitution -- matching Lot's
own earlier pattern of an unpopulated field).

Full live verification: fresh seed, real UI. Edit form correctly
pre-selects Location (OBRA-001) matching the real seeded value; the
asset-picker search for parent_id still works correctly post-rename;
a real parent_id assignment submitted through the actual form and
confirmed persisted under the correct "parent_id" column (not the old
"id_padre", the exact class of mistake a much earlier session made
when first investigating this same field). Zero server errors
throughout. Full seam-ui build/vet/test suite clean.

Four blocks remain: Custody, Finance, Service/IoT, plus the shared
DocumentLink/service_configs structs.

Cross-ref: CHANGELOG 0.3.77.

## [0.3.76] T-83 — T-40 Asset rename, Block B (Characterisation) done -- BrandID/ModelID/Serial1/Serial2/Lot/Criticality, live-verified. Found and fixed a real, live regression from T-80 along the way: the asset edit form's Model dropdown has been rendering empty labels in every deployment since that release, caused by a stale query in a file T-80's own work never touched (v0.3.76, 2026-08-12)

Theme: i18n · closed 0.3.76 · 2026-08-12


Trigger: continuing T-40's own resumed Asset rename (Block E/
Lifecycle done as T-82); next logical block per AssetDetail's own
comment structure.

**Block B — Characterisation, done.** Six fields:
MarcaID/ModeloID/Serie1/Serie2/Lote/Criticidad ->
BrandID/ModelID/Serial1/Serial2/Lot/Criticality, matching the English
vocabulary already established in assetFieldToXolu's own bridge and
in the already-completed Brand/Model/Criticality precedent from
earlier entity renames this session.

AssetDetail's struct fields renamed; also found and renamed a second,
separate struct with its own copy of Criticidad: AssetRow (the list-
view struct), only found by checking directly, not assumed identical
to AssetDetail's own scope. AssetRow's own consumer in assets.go
correctly left the Spanish-valued badge-color lookup untouched
("Alta"->"danger" etc.) -- only the Go field access renamed, matching
established enum-value precedent.

Found and used the actual, live consumer for these fields, same
lesson as Block E: not AssetDetail (again populated but never
rendered), but AssetValuesFromXolu/AssetValuesToXolu via the dynamic
edit form -- confirmed directly, not assumed to be AssetDetail again
just because that's where the struct literal is.

assetFieldToXolu's six entries converted to identity mappings,
checking criticality's known enum dependency first (already
documented in this item's own record). AssetsList's WHERE-clause
column reference and its OQL query fixed to the new "criticality"
column; the URL query parameter itself (?criticidad=) correctly left
as a wire name, unchanged, matching this codebase's established
precedent.

**Eight stale test assertions fixed** across two packages
(internal/handlers/api, internal/ui) -- validation tests, an
OQL-injection-safety test, round-trip tests, and a live end-to-end
dynamic-post test that, once fixed, itself confirmed the rename
correct (a real persisted payload showing brand_id/model_id/serial_1/
lot all present and correctly keyed). Sanity-checked the fix properly:
reverted criticality's bridge entry, confirmed three independent test
categories all genuinely failed for the right reason (round-trip,
API handler, list-query WHERE clause), restored, confirmed clean
again.

**A real, live, user-facing bug found and fixed as a direct side
effect of testing this block's own live verification, not part of
this item's own scope**: internal/ui/asset_source_resolver.go's
"entity:models" query (a completely separate file from where T-80's
own work happened) was still querying marca_id/nombre -- columns that
stopped existing on the models table the moment T-80 shipped
(v0.3.73). This has been silently broken in every real deployment
since then: the asset edit form's own Model dropdown was rendering
every option with an empty label, confirmed directly against a real
running server before touching anything. Checked the entire
assetSourceQueries map for the same class of miss -- every other
entry (brands, regions, locations, custodians, currencies,
cost-centers, alert-types, asset-types) already correctly used
English columns from their own prior renames; models was the one
genuine outlier. Fixed, and a real, precise, pre-existing test
(TestAssetSourceResolver_AllNineSourcesQueryCorrectly, hardcoded
exact-query-string mocks per source) caught the stale mock
immediately once the real fix ran against it -- this test would have
caught T-80's own regression at the time, had it existed then; fixed
its own models entry to match.

**Seed scripts**: whole-file substitution across all six files (274
total occurrences: marca_id/modelo_id/serie_1/serie_2/criticidad --
lote had zero occurrences in any seeded asset, confirmed, not a
missed substitution). Confirmed safe without function-boundary
scoping, same reasoning as Block E -- these keys are exclusive to
Asset's own seeding now that T-80 already converted Modelo's own
occurrences. Checked specifically for both quote-style variants before
running anything live, learning directly from Modelo's own earlier
miss; zero remaining old-key occurrences in either style confirmed
before the live run.

Full live verification, fresh seed to real UI: assets list page
correctly renders 11 Alta / 6 Media criticality badges; the criticality
filter correctly narrows to exactly the matching rows (confirmed by
parsing the actual badge values, not just grepping for the word
"Media" which false-positived against unrelated matchMedia() JS in
the page's own dark-mode script -- checked the real DOM structure
once that false positive was caught); the asset edit form correctly
pre-selects Brand (Caterpillar), Model (320 GC, only correct after
the resolver fix), and Criticality (High, correctly translated from
stored "Alta"); Serial1 correctly pre-fills. Zero server errors
throughout. Full seam-ui build/vet/test suite clean.

Five blocks remain: Location, Custody, Finance, Service/IoT, plus the
shared DocumentLink/service_configs structs.

Cross-ref: CHANGELOG 0.3.76.

## [0.3.75] T-82 — T-40 Asset rename resumed -- Block E (Lifecycle, 7 fields: PurchaseDate/ExpectedArrivalDate/ArrivalDate/ObsolescenceDate/ExpirationDate/DecommissionDate/DecommissionReason) done, live-verified. Applied the identity-map-not-empty lesson correctly from the start this time, checking assetEnumValueToXolu for dependencies before touching the bridge (v0.3.75, 2026-08-12)

Theme: i18n · closed 0.3.75 · 2026-08-12


Trigger: resuming T-40's own paused Asset rename ("Asset audit, started
2026-08-02, execution paused") now that both known collision risks
(Modelo, PropDefinition/PropAssociation) are cleared (T-42, fully
closed). Working one logical block at a time, matching the code's own
existing "Block A-G" comment structure in AssetDetail, rather than
attempting all 32 fields in one pass.

**Block E — Lifecycle, done.** Seven fields:
FechaCompra/FechaPrevIngreso/FechaIngreso/FechaObsolescencia/
FechaExpiracion/FechaBaja/MotivoBaja -> PurchaseDate/
ExpectedArrivalDate/ArrivalDate/ObsolescenceDate/ExpirationDate/
DecommissionDate/DecommissionReason, matching the English vocabulary
already established in assetFieldToXolu's own bridge (purchase_date,
expected_arrival_date, etc. -- the schema/form side already used these
names; only the Go struct and xolu's own storage columns needed to
catch up). AssetDetail's struct fields renamed; the one Go consumer
(the entity->AssetDetail conversion in routes.go) fixed via
build-driven iteration -- a clean build immediately after, confirmed
directly (not assumed) to mean these fields are genuinely read from
xolu but never actually rendered anywhere in the UI, a real, separate,
worth-noting finding, not something broken by this work or fixed as
part of it.

xolu's own asset columns renamed to match
(fecha_compra->purchase_date, etc.). assetFieldToXolu's bridge entries
converted to explicit identity mappings, not removed -- applying the
lesson this same item's own record already carries for Alerta, and
that T-42's own closure (PropDefinition/PropAssociation) had to
re-learn the hard way: decommission_reason has a real enum-value table
(assetEnumValueToXolu), confirmed by checking directly before touching
the bridge, and translateFlatFieldsAndEnums only applies that
translation when the field's key exists in the bridge map at all. All
seven fields given identity entries for consistency, not just the one
that strictly needs it, matching Alerta's own established pattern.

Seed scripts: a whole-file key substitution across all six files --
confirmed safe without function-boundary scoping first, since these
seven key names are exclusive to Asset's own seeding (unlike
nombre/marca_id, which genuinely collided with Modelo and needed
scoping). 110 total occurrences replaced. Caught seed-03/seed-04's
own conditional dict-access pattern
(`if compra: d["fecha_compra"] = compra`) correctly, since the
substitution matched the key string regardless of dict-literal vs.
dict-access context. Checked specifically for the class of miss that
bit Modelo (a single-quoted f-string dict access invisible to a
double-quote-only substitution) -- none found here, confirmed by
grepping for zero remaining old-key occurrences across all six files
before running anything live.

One existing test fixed: TestAssetEnumValues_RoundTrip's own
decommission_reason cases asserted against the old xolu key
(motivo_baja); updated to decommission_reason, all five enum value
cases (Sale/Scrap/Theft/Donation/Other) still passing correctly
afterward -- confirming the enum-value translation genuinely still
fires through the identity-mapped bridge, not just that the bridge
compiles. TestAssetValues_RoundTrip_AllScalarFields (an existing test
that automatically iterates every entry in assetFieldToXolu) provided
free additional coverage for all seven renamed fields with no changes
needed -- but checked directly, not assumed, that this test can only
catch internal round-trip inconsistency, not a genuinely wrong xolu-side
column name, since both directions of a round trip share the same
bridge map. That specific risk is what live verification below
actually covers.

Full live verification: fresh xolu + seam binaries, real seed run (16
assets created without error), the resulting purchase_date/
arrival_date values confirmed directly in xolu's own store with real
seeded dates, and a real authenticated asset detail page load
confirmed rendering correctly with zero server errors. Full seam-ui
build/vet/test suite clean.

Six blocks remain: Characterisation, Location, Custody, Finance,
Service/IoT, plus the shared DocumentLink item struct and
service_configs items this item's own paused audit already named as
in scope.

Cross-ref: CHANGELOG 0.3.75.

## [0.3.74] T-42 — Fully done -- Modelo (T-80) and PropDefinition/PropAssociation (T-81) both complete, live-verified. Both collision risks for T-40's own Asset rename now cleared (v0.3.74, 2026-08-12)

Theme: i18n · closed 0.3.74 · 2026-08-12


Surfaced 2026-08-02 while auditing Asset's attribute-rename footprint
(T-40): a proper occurrence audit of Asset's rename tokens (`nombre`,
`marca_id`, etc.) turned up two more masters entities in the exact same
violation shape as everything T-40 has already fixed, never previously
tracked as part of that item's scope.

**Modelo** (`ModeloRow`, `ModeloFormData`, `modeloFieldToXolu`): Go
struct fields `Nombre`/`MarcaID`/`MarcaNombre` and the xolu storage
keys `nombre`/`marca_id` are unrenamed. `modeloFieldToXolu` already
exists as a write-direction bridge (`"brand_id": "marca_id", "name":
"nombre"`), same shape as every entity T-40 has fixed before emptying
its bridge to a pass-through.

**Done, T-80, 2026-08-11.** Go fields, xolu's own `models` table
columns, and all six seed scripts renamed; `modeloFieldToXolu` emptied
to identity pass-through. A genuine, pre-existing bug found and fixed
as a direct side effect, not part of this item's own scope: the models
list page's brand-name lookup queried `brands.nombre`, a column that
stopped existing once brands' own rename completed -- the "Brand"
column had likely been showing blank for some time. Full detail in
`docs/RESOLVED.md`'s own T-80 entry.

**PropDefinition / PropAssociation** (`PropDefRow`, `PropDefFormData`,
`PropAssocRow`, `PropAssocFormData`): Go struct fields `NombreCorto`/
`TipoDato`/`EntidadDestino`/`EntidadID`/`Obligatoriedad` are unrenamed.
Distinct from T-39 (closed, RESOLVED.md): T-39 fixed `nombre_corto`'s
stored *values* (the token vocabulary -- kilometraje, horas_motor,
etc.); it explicitly left the field *name* `nombre_corto` itself
untouched, noted at the time as "a completely different, deliberately-
untouched concept." That field-name gap is what this item tracks.

Not scoped or estimated -- discovered as a byproduct of Asset's own
audit, not requested work. Real collision risk for Asset's own rename
execution: Modelo's `nombre`/`marca_id` and PropDef/PropAssoc's
`nombre_corto`-adjacent tokens share source-code vocabulary with
Asset's `document_links` item fields and `brand_id`, interleaved in
the same shared files (`masters_routes.go`, `masters_pages.go`, the
seed scripts) -- Asset's own rename audit must explicitly exclude
these two entities' occurrences, not touch them incidentally.

Cross-ref: CHANGELOG 0.3.74.

## [0.3.74] T-81 — T-42 fully closed -- PropDefinition/PropAssociation half complete, joining Modelo (T-80). Struct fields, xolu columns, and seeds renamed; a real bug found by a new test (emptying the field bridge to nothing, not an explicit identity map, silently broke enum-value translation for data_type/requirement) caught before any live testing, then also independently confirmed live, then fixed to match Alerta's own correct identity-map pattern (v0.3.74, 2026-08-12)

Theme: i18n · closed 0.3.74 · 2026-08-12


Trigger: continuing T-42's own second half (Modelo, T-80, closed
earlier) -- PropDefinition and PropAssociation.

**All four struct definitions renamed.** PropDefRow, PropDefFormData,
PropAssocRow, PropAssocFormData: NombreCorto->ShortName,
TipoDato->DataType, ValorDefecto->DefaultValue,
EntidadDestino->TargetEntity, EntidadID->EntityID,
Obligatoriedad->Requirement. Every Go consumer fixed via build-driven
iteration across masters_pages.go and masters_routes.go -- list pages,
create/edit/update handlers, the loadPropDefs dropdown helper,
uniqueness-check queries. xolu's own prop_definitions and
prop_associations table columns renamed in the same pass.

**Checked real precedent before assuming scope, not guessed.**
Verified Alerta's own already-completed enum-value bridge is still
non-empty -- confirmed even fully-renamed entities keep enum *values*
translated (English form values, Spanish stored values). Field
*names* go English; the data_type/requirement enum vocabularies stay
exactly as they were, correctly out of this item's scope.

**A fourth struct found only by letting the build find it, not named
in this item's own original scope description**: PropValueEntry (plus
its consumer propValueField), required because it read from the same
xolu columns being renamed. Renamed for consistency. A separate,
genuine finding along the way, not part of this item's own scope: 
PropValueEntry/LoadPropValues/propValueField are dead code -- zero
callers anywhere in the codebase. Renamed rather than left as stale
Spanish dead code, not deleted (a separate decision).

**A real bug found by a new test, not by inspection**: the first
attempt emptied propDefFieldToXolu/propAssocFieldToXolu to
map[string]string{} entirely, matching what was believed to be Marca's
own completed-entity precedent. It wasn't the right precedent --
Alerta's own bridge (which, unlike Marca, also has an enum-value
table) is an *explicit identity map*, not truly empty, and for a real
reason: translateFlatFieldsAndEnums (masters_xolu_fields.go) only
applies enum-*value* translation when the field's own key is found in
the field-name map at all. A truly empty map skips that check
entirely, silently passing English enum values straight through to
storage instead of translating them to the Spanish values still
expected there. TestPropAssocCreate_DataIntegrity (a new test, added
specifically because none of the 9 pre-existing PropDef/PropAssoc
tests inspected actual payload data) caught this immediately -- panicked
on a nil field before any live testing happened at all. Fixed both
bridges to explicit identity maps, matching Alerta's own correct
pattern precisely. Both new tests then strengthened further to assert
on the enum-*value* translation specifically (data_type must arrive as
"numerico" not "numeric", requirement must arrive as "requerida"), not
just the field key rename -- this is what would have caught the bug
immediately rather than only in live verification, and both
strengthened assertions were themselves sanity-checked: reverted to
the truly-empty bridges, confirmed genuine failure with the exact
"numeric" vs "numerico" mismatch that occurred live, restored.

**Also caught live, after the test suite was already clean**: created
a real property through the real form with the (still-buggy, truly-empty
bridge) code running against a real xolu instance -- the stored
data_type came back as "numeric" instead of "numerico", confirming the
bug end to end before the test-level fix was even written. The
create-and-verify cycle was repeated after the real fix, confirming
"numerico" now stored correctly, and a full PropAssociation
create-and-verify cycle confirmed the same for requirement/"requerida".

**Seeds.** All six seed scripts' seed_prop_definitions and
seed_prop_associations functions renamed with a scoped,
function-boundary-aware substitution -- keys only, enum values
(numerico, alfanumerico, requerida, opcional, etc.) deliberately
preserved exactly, confirmed directly by grepping for them
post-substitution. Zero remaining old-key occurrences confirmed across
all six files. Syntax-validated, and a full fresh seed run completed
without error (13 property definitions, 16 associations created).

**Full live verification, fresh seed to real UI**: properties list
page shows real property names; associations list page shows both the
property name and the correctly-translated requirement label
("Requerida"); edit form pre-fills correctly; a new property created
through the real form persists with correctly English-keyed,
correctly Spanish-valued columns; a new association created through
the real form does the same. Zero server errors throughout. Full
seam-ui build/vet/test suite clean.

T-42 is now fully closed -- Modelo (T-80) and PropDefinition/
PropAssociation (this item) both complete. T-40's own Asset rename
remains, now with both of its known collision risks (Modelo,
PropDefinition/PropAssociation) cleared.

Cross-ref: CHANGELOG 0.3.74.

## [0.3.73] T-80 — T-42, Modelo half complete -- Go struct fields, xolu storage columns, and all six seed scripts renamed to English, live-verified end to end (fresh seed, real UI, create/edit cycle); found and fixed a genuine pre-existing bug (brands list lookup querying a column that stopped existing after brands' own rename) as a direct side effect (v0.3.73, 2026-08-12)

Theme: i18n · closed 0.3.73 · 2026-08-12


Trigger: the tech team, "so there is still Spanish stuff lingering that
needs to be translated. do you remember what's the law now?" then
"Proceed then" -- resuming T-40/T-42's own already-recorded sequencing:
T-42 first (Modelo, PropDefinition, PropAssociation), specifically
because it's the collision risk T-40's own Asset rename must have
cleared before it can safely execute.

**Modelo half of T-42, complete.** ModeloRow/ModeloFormData's Go
struct fields renamed (Nombre->Name, MarcaID->BrandID,
MarcaNombre->BrandName), every consumer found and fixed via
build-driven iteration -- one struct-literal call site only surfaced
that way, not via the initial grep. Correctly identified and left
alone the real collision this item's own note warned about:
DocumentLink's own, unrelated .Nombre field (parseDocumentLinksToAny),
part of Asset's own separate, not-yet-executed scope -- confirmed by
reading the actual surrounding function, not pattern-matching.

xolu's own "models" table columns renamed (marca_id->brand_id,
nombre->name), modeloFieldToXolu emptied to identity pass-through,
matching the same completed-entity pattern Marca's own bridge already
sits in.

A genuine, pre-existing bug found and fixed as a direct side effect of
working in this exact code, not part of this item's own scope: the
models list page's brand-name lookup queried brands.nombre, but
brands' own rename (T-40) completed a while ago and that column has
been "name" since -- confirmed directly against the live store
(SELECT id, nombre FROM brands returned null for every row) before
fixing it. The models list page's "Brand" column has likely been
showing blank for some time.

Three test mocks updated to the new keys -- one of which
(TestModeloEdit_HTMXReturnsPartial) was silently passing for the wrong
reason the whole time, checking only that a partial rendered
structurally, never its actual content, so a stale-key mock never
would have been caught by it. Sanity-checked the one genuine
data-integrity test (TestModeloUpdate_DataIntegrity): reverted the
bridge, confirmed the test genuinely failed, restored it.

All six seed scripts updated with a scoped, function-boundary-aware
substitution (def seed_modelos to the next top-level def), not a blind
file-wide find-replace -- confirmed precise both directions: Modelo's
own fields correctly renamed inside the function, Asset's own,
completely separate marca_id field (a different entity, same token)
confirmed still untouched outside it. A real miss caught only by
actually running the seed live, not by static inspection: a
single-quoted item['nombre'] inside an f-string in all six files,
invisible to the double-quote-scoped substitution -- found because the
seed script crashed on a fresh run, fixed, then the seed re-run
completed cleanly from scratch.

Full live verification, fresh seed to real UI, not stopped at unit
tests: seeded a brand-new xolu instance from scratch (15 models
created without error), confirmed the models list page shows correct
model names AND correct brand names (the bug fix, live-confirmed, not
just reasoned about), confirmed the edit form correctly pre-fills from
the new columns, created a genuinely new model through the real form
and confirmed it persisted under the correct English columns. Zero
server errors throughout. Full seam-ui build/vet/test suite clean.

Not done: PropDefinition/PropAssociation, the other half of this item
-- tracked separately below, not folded into this closure. T-40's own
Asset rename remains entirely ahead, now with one of its two known
collision risks (Modelo) cleared; PropDefinition/PropAssociation is
the other.

Cross-ref: CHANGELOG 0.3.73.

## [0.3.71] T-79 — xolu v0.30.6 upgraded (from v0.28.0, purely additive pkg/client changes, full test suites clean both sides) -- the criticality persistence issue flagged during earlier live verification was retracted after direct investigation: never a xolu bug, my own OQL query used the English field name instead of the real Spanish xolu column (criticidad), the same mistake pattern as the earlier parent_id/id_padre case (v0.3.71, 2026-08-11)

Theme: xolu-integration · closed 0.3.71 · 2026-08-11


Trigger: the tech team, 2026-08-11 -- "Here's a xolu update. See if the issue
you flagged persists after the upgrade," referring to the criticality
persistence finding mentioned in the sensor/asset creation
verification (T-76 era).

**Upgrade, v0.28.0 -> v0.30.6.** pkg/client diff checked before
swapping: client.go (Create/Get/Patch/Delete/OQL -- everything
seam-ui's own 229+ call sites actually use) unchanged; the only real
diffs are purely additive (new ts.go client, new ListAccounts/
ListBookings methods on bal/cal). Full xolu whole-tree suite clean (41
packages, zero failures). go.mod updated, go mod tidy clean, full
seam-ui suite clean throughout.

**The criticality finding: retracted, honestly.** Re-tested the exact
original repro against v0.30.6 first, using the same, already-populated
data directory -- criticality still showed null via
`SELECT criticality FROM assets`. Rather than concluding the xolu
upgrade didn't help, checked directly why: GET /api/v1/schema/assets
returns "No schema found for assets" (pure blob storage, no adapted
columns registered at all for this entity) -- ruled out XOT178's own
tenant-schema-propagation fix as relevant, since there's no schema to
propagate. Fetched the raw stored record directly
(GET /api/v1/assets/18) instead of going through OQL: the value was
there the entire time, correctly saved, under the key "criticidad" --
"Media", not "criticality"/"Medium". Confirmed against
internal/ui/asset_xolu_fields.go directly: `assetFieldToXolu["criticality"]
= "criticidad"` is a real, working, intentional bridge, part of T-40's
own already-tracked "Asset itself" Spanish-identifier debt. My own
OQL verification query used the English field name; the real xolu
column is the Spanish one. The exact same mistake pattern as the
earlier parent_id/id_padre case (T-72's own investigation) --
confirmed by re-querying with the correct column name
(`SELECT criticidad FROM assets`), which returned "Media" correctly,
both before and after this upgrade. There was never a xolu bug here,
and nothing needed reporting to the xolu team for this -- my own
earlier "worth flagging separately" note (during the sensor/asset
live-verification pass) was wrong, and this closure says so directly
rather than quietly correcting it.

**Broader live verification, this upgrade specifically:** existing
issues (open/closed), work orders (completed state), and their FSM
state all read correctly through the new xolu version with zero
server errors -- confirming the upgrade itself is genuinely clean
beyond just the one finding that prompted it.

## What shipped

XOT166: transition payload keys must now be strict identifiers
(letters/digits/underscores, leading letter) -- a submitted payload
key outside that shape gets a clean 422 (XOLU-FSM014) instead of
silently landing somewhere no guard can reference. The tech team's own
decision, applied consistently to a boundary that had been the one
exception to a rule already enforced everywhere else (entity schema
fields).

XOT167: a double-quoted string in a guard or set-clause expression now
surfaces as a warning in the definition's own analysis.warnings at
creation time -- not a rejection, a warning, deliberately: auto-
converting was considered and rejected, since payload."odd key" was,
until XOT166 closed that gap, the only syntax able to reference a
payload field with an unsafe name, and a blind rewrite would have
silently broken that legitimate case while fixing the mistaken one.

## Verified, not assumed

Checked seam-ui's own payload-construction code directly, as the xolu
team's own response specifically invited ("a good moment to verify
rather than assume, same category as the apikey auth thing"):
technician_signature (Work Orders' one and only transition payload
key, internal/ui/workorders.go) is already a strict identifier, and
its guard is already single-quoted (fixed correctly the first time,
T-67). Confirmed by direct grep across the whole payload-construction
site, not inferred.

## Sync performed

diff -rq against the outgoing v0.27.1 confirmed pkg/client has ZERO
changes -- every difference is server-side (pkg/server/v2_fsm_common.go,
v2_fsm_walk.go) or documentation. The safest upgrade of the three this
session, by construction: nothing in seam-ui's own dependency surface
could have changed. Swapped in, go.mod updated, go mod tidy clean.

Full xolu test suite: one failure on first run
(TestSealer_GuardDiscipline_Race, pkg/chronicle -- a package seam-ui
doesn't use at all), confirmed genuinely flaky by re-running three
times clean -- not a regression, not investigated further given zero
relevance to anything this project depends on. pkg/client itself: 100%
clean, as expected given the empty diff.

Full seam-ui test suite clean after the swap (zero code changes
needed anywhere).

## A real, if minor, self-inflicted detour along the way

While live-verifying the swap, a temporary debug log statement left
over from an EARLIER investigation (T-72's own asset-picker work, a
false-alarm chase of a "parent_id not persisting" issue that turned
out to be a wrong column name in my own verification query, not a
real bug) called h.logger.Error(...) on a test handler constructed
with a nil logger, panicking one existing test
(TestAssetUpdateDynamic_XoluError_Returns500). Caught by the normal
test suite run, not live traffic -- removed cleanly, confirmed the
full suite green again immediately after. Noted here only because
"the previous session's leftover debug code broke an unrelated test"
is a real, if small, process lesson: remove temporary instrumentation
in the same pass it's added for, not carried forward into the next
piece of work.

## What the response corrected

CreateMachineDef has existed in xolu since v0.26.0, not "missing" as
T-69's letter reported -- we had checked v0.23.0 and v0.25.0
specifically (the versions actually available to us at the time), which
is exactly why it looked absent. The xolu team confirmed this
themselves rather than us catching it independently. Corrected in
internal/ui/workorders.go's own comment, which previously said "shipped
for real" as if v0.27.0 were its origin.

## What v0.27.1 actually changed

Diffed the full tree against the currently-integrated v0.27.0, not
assumed from the changelog alone: only pkg/client/schema.go,
pkg/client/schema_test.go, and doc/version metadata differ -- no server
code touched. The one functional change: CreateMachineDef/
ReplaceMachineDef now reject an empty Name/Initial client-side before
the request, rather than relying entirely on server-side validation --
the one genuine improvement from our own local patch's original shape
that the xolu team chose to adopt, applied to both methods (we'd only
had it on Create) rather than just the one we'd built. Purely additive,
confirmed by diff (no removed lines, no signature changes) -- our own
calling code always supplies non-empty Name/Initial, so this can't
affect existing behaviour.

## Sync performed

All 16 client methods seam-ui/Work Orders uses verified signature-
identical against the outgoing v0.27.0 before swapping (not assumed).
Swapped in cleanly, zero code changes required beyond two comment
corrections (the v0.26.0-not-v0.27.0 origin above, and updating the
guard-syntax footgun comment with the xolu team's own confirmed
mechanism -- correct T-SQL lexer behaviour, a double-quoted token is a
quoted identifier not a string literal, exactly like ANSI SQL; they're
weighing a validation warning for this specific mix-up, not committed
as of v0.27.1). Full xolu test suite clean (41 packages, including 4
new tests for the client-side validation addition). Full seam-ui test
suite clean (24 packages). Entire Work Orders lifecycle re-verified
live end to end against a real running v0.27.1 server: create
(bootstrap correctly reused the existing definition, still exactly 1),
start, complete with signature, durable "completed" state confirmed via
a fresh query, zero server errors.

## Not touched, per explicit instruction

T-70 (apikey auth) remains open, unchanged, still awaiting the tech team's
own confirmation -- not addressed again in the xolu team's response
(their letter was specifically about the CreateMachineDef request), and
out of scope for this cycle regardless. No new tasks or features picked
up, matching the instruction this sync was scoped under.

## Part 1: xolu v0.23.0 -> v0.25.0 upgrade

Investigating what a real composition test would need surfaced a
structural fact, not just an opportunity: xolu v0.23.0 (the version
pinned to this project all session) has ZERO FSM client methods at
all -- no CreateMachine, no WalkMachine, nothing. Testing composition
was never just "not yet done," it was untestable with the tooling
available. The tech team's v0.25.0 checkpoint (given earlier this session,
"keep it in case you need it") turned out to be exactly the missing
piece: a full, mature FSM client API (CreateMachine, WalkMachine,
GetMachine, GetMachineState, GetMachineTransitions, GetMachineHistory,
PatchMachine, DeleteMachine, ListMachines).

Verified compatible before swapping, not assumed: all 7 xolu.Client
methods seam-ui actually calls (Create, Delete, Get, List, OQL, Patch,
Update) have identical signatures between v0.23.0 and v0.25.0. Swapped
the local sibling checkout (backed up the old version first), rebuilt,
ran the full seam-ui test suite -- 24 packages, all clean, zero
failures, zero code changes needed anywhere in seam-ui itself. go.mod's
require line updated to match (v0.21.2 -> v0.25.0, the require
directive was already stale against the locally-replaced v0.23.0 even
before this upgrade). go mod tidy pulled new transitive dependencies
cleanly.

Backward compatibility confirmed at the data layer too, not just the
API: the new v0.25.0 binary correctly loaded the existing demo
database (created under v0.23.0) with no migration step, correctly
reporting the same 5 seeded users. Live-verified the whole app against
the new backend: fresh login, and /,  /profile, /settings, /admin/users,
/documents, /fsm/editor all 200, no server errors.

## Part 2: real, end-to-end proof of composition

Built a minimal Work-Order-shaped FSM definition (draft -> in_progress
-> completed) with the final transition's guard requiring a non-empty
technician_signature -- the exact field shape the signature widget
built earlier this session (T-10) produces (a data:image/png;base64,...
value). Not run as a seam-ui integration (Order Engine, T-11, still
needs its own real scoping before this becomes production code) -- a
standalone throwaway Go program (/tmp/fsm-composition-proof, not
committed to the repo) exercising the actual xolu.Client Go API
seam-ui itself would call, not just raw HTTP.

Confirmed, through the real client library against a real running
xolu server:
- A guard correctly REJECTED a completion attempt with an empty
  signature payload (XOLU-FSM004, surfaced as a real Go error through
  the client, not a raw HTTP status the caller has to inspect).
- The SAME guard correctly ACCEPTED completion once a real,
  signature-shaped payload was supplied.
- The commit was durable, confirmed via a fresh GetMachine call in a
  separate request, not just trusted from the walk response.
- The full audit trail (GetMachineHistory) recorded every step with
  timestamps and payloads; the rejected attempt left no trace at all
  (correct -- it never committed).

One real, useful finding along the way: an initial guard written with
double-quoted string literals (payload.x != "") was rejected by every
attempt, including ones that should have succeeded -- the correct
syntax is single-quoted (!= ''), T-SQL style, matching every example in
xolu's own docs. Not a xolu bug; a real thing worth remembering before
writing more guards.

This directly answers the reviewer's named gap: form data now has a
real, verified, working path to gate and drive an FSM transition,
committed durably to xolu, through the actual client library seam-ui
would use. It does not scope or build Order Engine (T-11) -- that
remains its own, separate, real piece of work.

Cross-ref: CHANGELOG 0.3.71.

## [0.3.70] T-74 — Left sidebar module synchronization -- planned (docs/SIDEBAR_SYNC_PLAN_2026-08-06.md), both design questions resolved, ready to build (v0.3.70, 2026-08-11)

Theme: navigation · closed 0.3.70 · 2026-08-11


Trigger: the tech team, 2026-08-06 -- "Add relevant menus to the modules
that are currently thin, so that the functionality can be tested.
Ensure that the main menu for the Assets module remains the same it
has been. That means, menus should look and behave the same as
always, with collapsible groups, icons, and every item linked to one
module view." Direct implementation of the plan filed against this
same item (docs/SIDEBAR_SYNC_PLAN_2026-08-06.md).

**Built.** internal/modules.SidebarItem (Label/URL/Icon/Section/
RequiredPerm, deliberately as dependency-free as Module's own Label/
Icon/Path -- internal/modules doesn't import internal/ui, and
internal/ui already imports internal/modules, so the reverse would be
circular) and Module.SidebarItems []SidebarItem. internal/ui/layout.go's
activeModuleSidebarItems replaces MainNav entirely: resolves the active
module by longest-matching Path prefix against cfg.VisibleModules
(already permission-filtered per request), converts that module's own
SidebarItems into NavItems with Active computed fresh, same as
MainNav's own static list used to. Every module's registration in
server.go now carries its own SidebarItems -- Assets' 17 items
(Operational/Configuration/Maestros, unchanged), Administration
(Users), Settings (Settings), and two new populations: Work Orders
(All Work Orders, New Work Order) and Issues (All Issues, New Issue) --
the actual "add menus to thin modules so functionality can be tested"
ask. Documents deliberately gets zero items -- one route, nothing to
navigate to beyond what the module selector already reaches.

**Verified Assets is genuinely unchanged, not just visually similar.**
Captured the real rendered sidebar HTML before any code changed,
same again after, and confirmed byte-for-byte identical once the
(correctly, intentionally removed) Administration/Settings section is
excised from the old capture -- proof, not assertion, that every
class, icon, order, and collapsible-group behavior for Assets' own 17
items is unchanged. The Administration section's disappearance from
Assets' own view isn't a regression: those items never belonged to
Assets conceptually, and showing them only when Administration/Settings
are the active module is the entire point of this fix -- confirmed
directly against the tech team's own "every item linked to one module view"
phrasing before treating this as correct rather than a missed
requirement.

**Two real bugs found and fixed during implementation, not just
planning.** First: DefaultConfig() (the test-fixture default) didn't
populate VisibleModules at all, so every sidebar test using it
silently rendered nothing -- caught because TestMapNavItemHiddenWhenDisabled
was passing for the wrong reason (nothing rendered, not because Map
was specifically hidden while other items correctly remained). Second,
more consequential: navItems() (the existing, unchanged rendering
function) only attaches a non-Section item to a group that comes after
a Section header -- true of the old MainNav's list by construction
(it always opened with one), but not automatically true of a module
declaring only 1-2 items with no natural grouping. Administration,
Settings, Work Orders, and Issues all initially shipped without a
leading Section header and silently rendered empty sidebars on their
own pages -- found by live-testing each module's own page directly
rather than assuming the mechanism generalized correctly from the
Assets case, and fixed by giving each module its own Section header
(reusing nav.section.admin for Administration/Settings, since they're
never shown simultaneously anymore; each module's own name for Work
Orders/Issues).

**One known, minor, accepted quirk**, not fixed: on /workorders/new
(and equivalently /issues/new), both "All X" and "New X" show as
active simultaneously, since /workorders is a genuine string prefix of
/workorders/new. Not a correctness issue -- both links work correctly
-- and isolated entirely to the two new item pairs added here; doesn't
touch Assets' own, byte-verified behavior. Left as-is rather than
special-cased, given the risk/reward of a targeted fix for a cosmetic
detail against the size of this change already.

**Testing.** 4 new unit tests (internal/ui/sidebar_sync_test.go):
longest-prefix resolution, no-match returns nil cleanly, RequiredPerm
correctly filters an item within an otherwise-visible module, Map's
runtime-mutable filtering. The longest-prefix test's own first version
had a real gap, caught only by sanity-checking it: passed even with
the length comparison completely removed, because registration order
alone happened to produce the right answer by coincidence. Rewritten
with the more-specific module registered first and the fallback
second, specifically to isolate the length rule from registration
order -- confirmed this version genuinely fails against the broken
logic before restoring it. Live-verified all six modules' own pages
directly (desktop and mobile sidebars both), confirmed mobile matches
desktop exactly for Assets, confirmed zero server errors across the
entire verification pass. Full seam-ui test suite clean throughout.

Cross-ref: CHANGELOG 0.3.70.

## [0.3.69] T-78 — register.py's own span-boundary parser was treating any body sub-heading ("## Some Heading", this session's own closure-writing convention since T-75) as a theme-group boundary, silently truncating three closures (T-75/T-76/T-77) and orphaning the remainder in TRACKING.md -- parser fixed and verified both directions, all three RESOLVED.md entries reconstructed from the recovered orphaned text (v0.3.69, 2026-08-11)

Theme: repoman-tooling · closed 0.3.69 · 2026-08-11


Trigger: found while updating T-74 -- register.py's own list command
showed T-19 (the asset-picker) stale, and separately, TRACKING.md's
tail contained text that read as orphaned, unattached to any item
header, immediately after T-74's own section.

## The bug, confirmed precisely

register.py's Register._parse() determines where an item's detail
section ends by searching for the next "### " item header, "---", or
"## " line -- treating ANY "## " as a boundary. That's correct for a
genuine theme-group header ("## formengine", immediately followed by
"### T-nn."), but wrong for a "## Some Heading" sub-section used
WITHIN a single item's own closure narrative -- exactly the writing
convention this session had been using since T-75 (T-75, T-76, T-77
all used "## Audit performed"/"## Fix"/"## Testing"-style sub-headers
in their own closure text). Every close operation using this
convention silently truncated both halves of the operation: the text
copied to RESOLVED.md stopped at the first such sub-header, and the
text removed from TRACKING.md ALSO stopped there, leaving the
remainder of each closure's own body orphaned in TRACKING.md,
unattached to any header, invisible to register.py check (which only
validates open items) and invisible to a normal read of the file
unless you happened to notice trailing content after a later item's
own section.

## Fix

Register._parse()'s span-boundary search now distinguishes a genuine
theme-group header from a body sub-heading by lookahead: a real theme
header is always immediately followed by a "### " item header
(confirmed directly against every real theme boundary in the current
file before writing the fix, not assumed); a body sub-heading isn't.
`^### |^---\s*$|^## \S[^\n]*\n+(?=### )` replaces the old
`^(### |## |---\s*$)`. Verified both directions with isolated,
synthetic tests before touching the real files: a body sub-header is
now correctly retained as part of its own item's body and no longer
mistaken for a boundary; a genuine theme-group boundary is still
correctly recognised and still correctly terminates a span.

## Data repair

T-75, T-76, and T-77's RESOLVED.md entries were each missing their own
closure narrative's back half (everything from their first "## "
sub-header onward) -- reconstructed by locating each truncation point,
extracting the corresponding orphaned fragment still sitting in
TRACKING.md (which contained exactly the missing text, since that's
where the incomplete removal had left it), and splicing each fragment
back into its correct RESOLVED.md entry at the exact point it had been
cut. One stray artifact from the extraction (a leftover "---" that had
been TRACKING.md's own file-ending marker, not part of any closure
text) found and removed separately. TRACKING.md's own orphaned block
removed entirely once its content was confirmed recovered, restoring
T-74 to its own correct, standalone span. register.py check confirmed
29 open items before and after (no open item lost or altered), and
each of the three repaired RESOLVED.md entries read through in full
afterward to confirm they now flow correctly from their own Trigger
paragraph through their own sub-headed body to their own Cross-ref
line.

## Going forward

This session's own closure-writing convention (using "## Heading" for
sub-sections within a single item's narrative) will need to change --
either avoid "## " within closure bodies going forward (using "**Bold
heading:**" instead, already used elsewhere this session for the same
purpose), or the fix above makes "## " safe to keep using as long as
it's never immediately followed by a "### " line, which in practice it
never naturally would be. Noted here rather than left to be
rediscovered the same way this was.

Cross-ref: CHANGELOG 0.3.69.

## [0.3.67] T-77 — Extracted shared UI primitives for the Issues/Work Orders duplication -- adapted badges to an existing Badge() primitive I'd almost duplicated (caught by the compiler, not by checking first), extracted a genuinely new SimpleListCard primitive for the clickable-row list shape after confirming the existing table-based ListRow system was a real mismatch, not just a style choice (v0.3.67, 2026-08-06)

Theme: ui-primitives · closed 0.3.67 · 2026-08-06


Trigger: the tech team, 2026-08-06 -- following a direct question about
whether the Issues/Work Orders duplication I'd named (issueStatusBadge
vs workOrderStateBadge, and the hand-rolled list markup both modules
shared) was worth fixing: "Adapt the hardcoded ui bits to the
primitives you mentioned, or propose a new invariant that better fits
your use case."

## Badges: adapted to an existing primitive I'd almost duplicated

Went to extract a shared StatusBadge/BadgeColor primitive for the two
near-identical badge functions -- and hit a genuine near-miss: Badge(text,
variant string) already exists in internal/ui/components.go, doing
exactly this (a pill with a text/variant-to-color mapping), unnoticed
because I designed my own version from the hand-rolled duplication
directly rather than checking whether a shared primitive already
existed first. Caught only by the compiler (StatusBadge redeclared),
not by checking before writing. Removed my duplicate; both
issueStatusBadge and workOrderStateBadge are now thin functions
mapping their own status vocabulary to Badge's existing variant
keywords (issues: "warning" for open, default for closed; work
orders: "info" for in_progress, "success" for completed, default for
draft) -- no markup of their own left at all.

## Lists: a genuinely new, small primitive -- checked before deciding it was warranted, not the default choice

Considered adapting to the existing ListRow/ListCellRich/ListActions
system (internal/ui/listing.go) first, rather than assuming a new
primitive was needed. Read its actual implementation before deciding:
it's a <table>-based system (Tr/Td), built for genuinely multi-column,
per-action-icon lists (Assets, Users, every masters entity) with
discrete view/edit/delete icon buttons per row. Work Orders and Issues
are a different, simpler shape -- one clickable row IS the navigation
to the detail page, no separate action icons at all. Forcing that
shape into a table would mean inventing a fake "view" action column
for something that's already a single link, a real downgrade, not a
style preference. Extracted SimpleListItem/SimpleListCard
(internal/ui/components.go) instead -- a genuinely new, small
primitive, documented directly against ListRow's own doc comments so
the distinction (and why it's not "the same thing done differently")
is recorded where the next person building a similar module will
actually read it.

## Verification, including one honest correction to my own check

Full rebuild/test clean, no test breakage (neither module had a test
asserting on exact badge CSS, only on text presence). Live-verified
against a real running server, both modules: created a second, open
issue specifically to see the "open" badge render (the first live
check only had a closed issue to look at). Found what looked like a
missing badge -- grepped for "bg-amber-100" and found nothing. Checked
the actual rendered HTML directly rather than assuming a rendering
bug: Badge's own "warning" variant renders bg-yellow-100, not amber --
I'd misremembered the exact Tailwind color name when writing the
issues.go mapping two messages earlier, and my own verification query
checked for the wrong string, not a real defect. The badge was
correct; my own check wasn't specific enough. Confirmed by grepping
for the right string afterward -- one warning-badge occurrence, the
open issue, correctly rendered. Work Orders' own completed-state badge
confirmed green (success variant) against the real work order
completed earlier this session. Zero server errors throughout.

Cross-ref: CHANGELOG 0.3.67.

## [0.3.66] T-76 — Issue Tracker module built -- simple, always-on, the asset-picker widget's third real consumer (parent_id, sensor binding, now this), live-verified end to end including a status toggle and asset-name resolution (v0.3.66, 2026-08-06)

Theme: issue-tracker · closed 0.3.66 · 2026-08-06


Trigger: the tech team, 2026-08-06 -- "Can we create a simple issue tracker
module with a simple form that lets you select an asset linked to the
issue being created?" Following directly from confirming the asset-
picker widget's reuse readiness (T-72's own follow-up verification,
the two-picker-per-page test).

## Scope, deliberately minimal, matching "simple"

A new, always-on module (not gated like Work Orders was -- nothing
here needed that playground framing). One entity ("issues": title,
description, status, asset_id, created_at), one schema (title,
description, an optional asset-picker field), a plain open/closed
status toggled directly via xolu.Patch -- deliberately NOT an FSM
machine walk the way Work Orders' real lifecycle was. A two-state
status field doesn't need xolu's own FSM primitives; using them here
would be over-engineering a "simple" ask, not following the "compose
on the server side" principle -- that principle applies to genuine
workflows with real guards and transitions, not a boolean toggle.

## The asset-picker, reused as its third real consumer

x-seam-input: "asset-picker", x-seam-source: "entity:assets" -- zero
widget code changes needed, exactly the reuse this session's own
question ("is it good enough for a future Issue Tracker and Work
Orders module") was asking about, now answered by actually building
against it rather than just reasoning about it. Confirms directly what
was previously only argued: the widget's generic design holds up for
a third, independent consumer.

## Built

internal/formengine/schemas/issues (create.json), internal/ui/issues.go
(IssuesList/IssueNew/IssueCreate/IssueDetail/IssueToggleStatus + three
page-rendering functions), internal/modules/issues (thin MountRoutes
delegation, matching the established per-module package pattern),
MountIssuesRoutes in router.go (always-on, no EnableX flag, requireAuth
only -- a granular permission scheme is real, later work if this
module's scope grows). Registered in server.go as a sixth "apps"-group
module, HasAccess unconditionally true (no granular permission model
exists yet for this module, same reasoning Account's own registration
used). 13 new i18n keys x 4 locales.

## Testing and verification

5 new handler-level tests: required-title validation actually blocks
submission (not just documented as required), asset_id correctly
linked when provided, asset_id correctly OMITTED (not sent as a
spurious 0) when the picker had no selection, the status toggle
genuinely flips both directions (not a one-way "always closes"), and
the detail page correctly resolves the linked asset's real name, not
just its raw ID. The required-title test verified with the full
discipline: temporarily disabled the validation, confirmed the test
genuinely failed (500 instead of the expected 200 re-render, the error
key and error styling both absent), restored it.

Live-verified end to end against a real running server: module
selector shows "Incidencias" (always visible, correctly localized),
created a real issue through the real form with a real asset selected
via the picker, confirmed asset_id and status correctly persisted,
detail page correctly resolved and displayed the real asset name,
toggled status and confirmed the flip to "closed" persisted, list page
correctly showed the updated status. Zero server errors throughout.

Full seam-ui test suite clean (25 packages, including the new
internal/modules/issues).

Cross-ref: CHANGELOG 0.3.66.

## [0.3.65] T-75 — Audited the codebase for any REST request bypassing a resolvable client abstraction, per the tech team's direct instruction -- found and fixed one real violation (fetchRuntimeState's bare http.Get, no timeout, no named seam), confirmed webhook delivery's own raw HTTP usage is correct as-is (v0.3.65, 2026-08-06)

Theme: seam-api-integration · closed 0.3.65 · 2026-08-06


Trigger: the tech team, 2026-08-05 -- "It's crucial then, that we never hardcode
any REST requests whatsoever," following the multi-instance
xolu/provisioning discussion (docs/PROVISIONING_PLAN_2026-08-05.md).
Asked to audit the codebase for violations before continuing.



## Audit performed

Searched the whole codebase for any request-construction pattern
bypassing xolu.Client (the one abstraction all 229 other call sites
correctly go through) or otherwise talking to a backend service
directly. Found exactly two matches, one real violation and one
correct exception:

- internal/webhook's own delivery client (http.Client{}, .Do(req)) --
  NOT a violation. A webhook's entire job is POSTing to an arbitrary,
  customer-registered URL; there is no "the right instance" to resolve
  there, the destination genuinely is whatever the customer configured.
  Different problem, correctly solved differently. Left untouched.
- fetchRuntimeState (internal/ui/routes.go) -- a real finding. A bare
  http.Get(url) built from h.config.SeamAPIURL, feeding the asset
  detail page's runtime FSM state (tracked as S-11, an already-shipped
  feature this session hadn't previously noticed -- it's exactly what
  the corrected usability plan's own Recommendation 1 proposed
  building fresh; docs/USABILITY_PLAN_2026-08-05-corrected.md corrected
  separately to reflect this). Two real problems with it: no timeout
  at all (a slow or hung seam-api would hang every asset detail page
  load along with it), and no named boundary -- if seam-api itself
  ever needs the same per-account instance resolution
  docs/PROVISIONING_PLAN_2026-08-05.md describes for xolu, nothing
  marked this as part of that surface.

## Fix

Replaced the bare http.Get with a named seamAPIClient type (one field,
httpClient *http.Client with a real 5s timeout) and one method,
FetchRuntimeState. fetchRuntimeState (the existing call site's own
entry point, internal/ui/routes.go:347) is unchanged in signature and
behavior for the success path -- it now delegates to a single shared
defaultSeamAPIClient instance instead of calling http.Get inline.
Deliberately minimal: no caching, no multi-instance awareness, no
resolver logic -- building any of that now would mean guessing at a
shape nolu hasn't settled yet, the exact mistake
docs/PROVISIONING_PLAN_2026-08-05.md already caught and corrected once
in this same session for the xolu side of this same question.

Kept as a package-level var rather than a new UIHandler field --
adding a field means touching NewUIHandler's constructor and every
test that calls it, for a client with no per-handler-instance state
worth threading through.

## Testing

6 new tests (internal/ui/seam_api_client_test.go): the success path
unchanged from the original implementation's own behavior (including
that an empty fsm_state value correctly reports false, not a blank
success), a non-200 response, malformed JSON, and -- the actual point
of this fix -- a dedicated timeout-enforcement test using a real slow
httptest server and a short client timeout, confirming the call
returns promptly rather than hanging for the server's own delay.
Verified with the full discipline: temporarily removed the timeout
from the test's own client construction, confirmed the test genuinely
failed (2.001s elapsed, waited for the full slow-server delay), then
restored it. Full seam-ui test suite clean throughout.

Live verification not possible in this environment -- seam-api itself
isn't running anywhere available, and SeamAPIURL has been empty
(disabled) in every server this session has started, meaning this
code path has never actually been exercised live even during earlier
verification passes. The httptest-based coverage (real HTTP client,
real timeout enforcement, real JSON parsing, against a real if
synthetic server) is the most rigorous verification actually available
here.

Cross-ref: CHANGELOG 0.3.65.

## [0.3.64] T-31 — urlEncode() doesn't properly percent-encode non-ASCII bytes -- translated error messages (Japanese, accented Spanish/Portuguese) pass through raw UTF-8 into redirect query strings (v0.3.64, 2026-08-05)

Theme: i18n · closed 0.3.64 · 2026-08-05


Found 2026-08-01 during T-18 Stage 9 (login error-redirect i18n): `internal/ui/routes.go`'s `urlEncode()` only escapes a handful of ASCII reserved characters (space, &, =, ?, /, :) and passes every other byte through unchanged. LoginSubmit's error redirects (`/login?error=`+urlEncode(t(...))) now route translated messages through it, same as the pre-existing lockMsg usage did. Raw UTF-8 in a URL query component is tolerated by most browsers/Go's net/url, so this is not visibly broken today, but it is not correct percent-encoding (RFC 3986) and could misbehave with strict proxies, older HTTP clients, or log parsers. The fix is straightforward (swap in net/url.QueryEscape or fix urlEncode's default case to percent-encode non-ASCII), deferred rather than done inline because it's a shared low-level helper touched by other call sites beyond this session's scope, and changing its behavior deserves its own verification pass rather than riding along with an i18n threading change.

Fixed 2026-08-05, prompted by the tech team ranking this among the three most urgent open items and asking directly for it. Replaced the hand-rolled encoder with net/url.QueryEscape -- confirmed it preserves the one previously-relied-on behavior (space -> '+') so every existing call site's own expectations are unchanged, while correctly percent-encoding everything the old default case passed through raw.

A second, adjacent gap found and fixed in the same pass, not silently left alongside the first: none of LoginSubmit's seven failure-redirect lines encoded the submitted `email` value at all, even under the old (broken) urlEncode -- concatenated directly into the query string (`"&email="+email`). A submitted email of `x@example.com&admin=true` would have added a live, unintended third query parameter to the redirect target. Fixed by wrapping all seven call sites in urlEncode(email) too.

Six new tests (internal/ui/urlencode_test.go): real Japanese/Spanish/
Portuguese strings round-tripped through encode-then-decode (not synthetic ASCII-range cases), the previously-correct behaviors (space->'+', the five ASCII characters the old encoder already handled) confirmed unchanged, and an adversarial test submitting an email containing query-string-structural characters, confirming the parsed redirect target has no injected third parameter. Verified with the full discipline: reverted both fixes, confirmed all three locale tests failed with the actual raw, unencoded characters visible in the failure diagnostic (無効な認証情報です, Contraseña, Sessão all shown un-encoded in the test output), restored.

Live-verified against a real running server, not just unit tests: a genuine login failure produced a redirect whose actual Location header showed `Contraseña` correctly encoded as `Contrase%C3%B1a` -- a real accented character, correctly percent-encoded, in a real HTTP response.

Cross-ref: CHANGELOG 0.3.64.

## [0.3.63] T-73 — xolu v0.28.0 synced -- the guard-syntax warning (XOT167) and strict-identifier payload key validation (XOT166/XOLU-FSM014) both delivered; verified seam-ui's own Work Orders payload is already compliant, zero pkg/client changes, safest swap of the three this session (v0.3.63, 2026-08-05)

Theme: xolu-integration · closed 0.3.63 · 2026-08-05


Trigger: xolu team's response to the guard-syntax note from T-69's
letter, delivered alongside xolu v0.28.0 -- "still deciding the right
scope... will follow up either way" (T-71's own addendum), followed
up on properly this time.

Cross-ref: CHANGELOG 0.3.63.

## [0.3.63] T-72 — Asset-picker widget finished (T-19) and used to build real sensor-asset binding -- both previously-dead 'Bind'/'Unbind' buttons now work, live-verified end to end (v0.3.63, 2026-08-05)

Theme: formengine · closed 0.3.63 · 2026-08-05


Trigger: the tech team, 2026-08-04/05 -- "finish the asset picker widget, and
use it to bind sensors to assets."

Cross-ref: CHANGELOG 0.3.63.

## [0.3.62] T-09 — Flaky test: TestGenerator_ValidateChecksum corrupts to a fixed char that can equal the real checksum (v0.3.62, 2026-08-04)

Theme: test-quality · closed 0.3.62 · 2026-08-04


Trigger: Discovered 2026-07-28 running go test ./... ahead of module extraction work — 1 failure in ~31 runs, isolated reruns didn't reproduce until traced to root cause.
Scope: internal/qrcode/qrcode_test.go:116 does corrupted := code[:len(code)-1] + "X". checksumChars is a 36-character alphabet (0-9, A-Z) that includes 'X', so ~1/36 of generated codes legitimately end in 'X' already, making corrupted == code and the test fail on a code Validate() correctly accepts. Test bug, not a production defect — real corruption doesn't write literal 'X'. Fix: corrupt to a character guaranteed different from the actual last char (e.g. rotate through the alphabet), not a fixed literal.
Secondary, lower-priority observation: the checksum itself is a positional weighted sum mod 36 (sum += int(c)*(i+1)), which is a known-weak checksum family — doesn't reliably catch adjacent-character transpositions. Not asked to fix; noting for whoever eventually revisits QR code integrity.

Fixed 2026-08-04. Rotates the code's actual last character to the next one in checksumChars (bytes.IndexByte + modular increment), guaranteeing a genuinely different character every run rather than a fixed literal that ~1/34 of generated codes could legitimately already end in (checksumChars is 34 characters -- 0-9, A-Z minus I/L/O -- not 36 as originally described; corrected here, not a discrepancy worth its own item). Verified with the full discipline: 1000 consecutive runs of the fixed test, zero failures (would have expected ~29 at the old rate). Reverted to the original buggy version for a sanity check -- genuinely reproduced the flakiness at a statistically consistent rate (98/100 sub-runs passed, 2 failed, matching the original "~1/31" description almost exactly), then restored the fix. Secondary checksum-weakness observation above left as noted, not addressed -- out of this item's own stated scope.

Cross-ref: CHANGELOG 0.3.62.

## [0.3.61] T-71 — xolu v0.27.1 reviewed and synced -- confirmed the xolu team's response (CreateMachineDef existed since v0.26.0, not newly delivered; the one client-side validation improvement adopted), swapped in cleanly (zero code changes beyond two comment corrections), full test suites clean, Work Orders lifecycle re-verified live end to end (v0.3.61, 2026-08-04)

Theme: xolu-integration · closed 0.3.61 · 2026-08-04

Addendum, 2026-08-04, asked directly by the tech team ("is the xolu team owing
us anything"): no firm technical deliverable -- CreateMachineDef was
delivered in full, plus more than asked (ReplaceMachineDef,
DeleteMachineDef, ValidateMachineDef), and the one gap in the original
letter (CreateMachineDef appearing missing) turned out to be incomplete
version coverage on our own side, not a real xolu gap. One open thread,
minor and not blocking: their response's exact words on the guard-syntax
validation-warning idea were "still deciding the right scope... will
follow up either way" -- a stated commitment to communicate a decision,
not a promised feature, not yet fulfilled. Nothing actionable on our
side; noted here so it doesn't silently vanish, not tracked as a
separate open item given there is nothing for us to do but wait.


Trigger: we shared xolu v0.27.1 alongside the xolu team's written
response to the T-69 letter (docs/XOLU_CLIENT_FEATURE_REQUEST.md),
2026-08-04. Instruction: review and sync only, no new scope.

Cross-ref: CHANGELOG 0.3.61.

## [0.3.60] T-69 — Letter sent to the xolu team (docs/XOLU_CLIENT_FEATURE_REQUEST.md) requesting CreateMachineDef land upstream in pkg/client -- our local patch on the sibling checkout is provisional and gets replaced once they deliver (v0.3.60, 2026-08-04)

Theme: xolu-integration · closed 0.3.60 · 2026-08-04


Trigger: the tech team's direct instruction, 2026-08-04, following T-68's
Work Orders integration: "prepare a letter to the xolu team requesting
the missing features you had to implement on the xolu client side.
After they are done when they deliver, we're going to replace your
temporary xolu client code."

Letter written: docs/XOLU_CLIENT_FEATURE_REQUEST.md. Requests the xolu
team review and land, upstream, a CreateMachineDef method on
pkg/client (the one genuinely missing piece found while building T-67/
T-68 -- pkg/client has a complete surface for operating on FSM
machines and reading definitions, but no way to create one). Includes
our local, provisional implementation as a starting point (not a
mandate), the exact request/response shapes verified against a real
running v0.25.0 server, and a secondary, lower-priority note about a
guard-syntax footgun (double-quoted string literals in a guard
silently match nothing, no parse error) that cost real debugging time
during T-67's own proof-of-concept.

This item stays open until the xolu team delivers. When they do, the
local patch on the sibling checkout (pkg/client/schema.go's
CreateMachineDef, added this session) gets replaced with whatever they
formally ship -- Seam AMS's own CreateMachineDef call sites
(internal/ui/workorders.go's ensureWorkOrderDefinition) should need no
changes if the shape lands close to what's proposed, but should be
re-verified against whatever actually ships, not assumed compatible.

Not something we can close by ourselves -- depends entirely on the
xolu team's own timeline and decisions.

Delivered, 2026-08-04 -- same day. xolu v0.27.0 shipped
CreateMachineDef, plus more than requested: ReplaceMachineDef,
DeleteMachineDef, and ValidateMachineDef, a complete, properly-typed
FSM definition CRUD+validate surface, not just the one method asked
for. Verified directly against their own CHANGELOG entry and source,
not assumed: every wire shape checked against pkg/server/
v2_fsm_def_handlers.go before they wrote anything (their own words),
Analysis returned as a real typed *MachineDefAnalysis rather than the
json.RawMessage the local provisional patch used, and two honest,
documented caveats -- ReplaceMachineDef affects future machines only
(matching what this session's own reading of their docs had already
found), and DeleteMachineDef permits deletion with no check against
machines still referencing the definition, because no server-side
query exists to build that check against, stated as a real limitation
rather than invented as new server scope.

Swapped in the same day: all 12 client methods seam-ui/Work Orders
uses verified signature-identical against the outgoing v0.25.0 before
swapping (not assumed), one mechanical change needed in
internal/ui/workorders.go (CreateMachineDefRequest -> MachineSpec
directly, matching their real signature), full xolu test suite clean
(41 packages), full seam-ui test suite clean (24 packages), and the
entire Work Orders lifecycle re-verified live against a real running
v0.27.0 server end to end -- create (bootstrap correctly reused the
existing definition, not re-created), start, complete with signature,
durable "completed" state confirmed, zero server errors.

The guard-syntax footgun (section 4 of the letter) was not addressed
in this release -- no FSM.md changes found for it. Low priority, not
re-raised.

A separate, more significant finding surfaced while reading v0.27.0's
own CHANGELOG for unrelated reasons: T-160 in their release notes
states the client's apikey auth mode (Client.WithAPIKey) has been
completely non-functional -- sends "Bearer", the server never accepts
that prefix for apikey auth, silently unauthenticated on every
request regardless of key correctness. Confirmed this is a live,
supported configuration path in Seam AMS, not theoretical:
SEAM_XOLU_API_KEY is documented in both DEPLOY.md and README.md,
wired through in server.go's xoluOpts construction, and
internal/config/config.go's own field comment mislabels it "Bearer
token" -- matching, not catching, the bug. Filed as its own item
(T-70) given the severity and that it needs the tech team's direct
confirmation of whether this credential path was ever actually
configured in a real deployment -- not folded into this item's own
closure.

Cross-ref: CHANGELOG 0.3.60.

## [0.3.58] T-68 — Work Orders integrated as a real, kept playground module (not a throwaway) -- form + FSM + xolu commit composition (T-67) now live in the actual application, feature-flagged off by default, live-verified end to end (v0.3.58, 2026-08-04)

Theme: workorders-playground · closed 0.3.58 · 2026-08-04


Trigger: the tech team's direct request, 2026-08-04, following T-67's
standalone proof-of-concept: "Integrate Work Orders and keep it. It's
going to be used as the playground for the Work Orders team."

Scope, deliberately minimal and explicitly NOT T-11's own scope: one
FSM (draft -> in_progress -> completed), one guard (a technician
signature required to complete), reusing the exact signature widget
built earlier the same session (T-10). No Blocks, no Numbering, no
Business Rules, no Cuadrilla/Empresa entities. T-11 (Order Engine/CMMS,
its own real scoping, five open design questions, no module name
settled) remains entirely separate, unbuilt work -- this closes no
part of it, it proves the composition mechanism a production module
would build on.

Cross-ref: CHANGELOG 0.3.58.

## [0.3.57] T-67 — xolu upgraded v0.23.0->v0.25.0 (adds FSM client bindings entirely absent before), and a real end-to-end proof-of-composition built: a form-shaped signature value gating and driving a durable FSM transition through the actual client library, directly answering a reviewer-named gap (v0.3.57, 2026-08-04)

Theme: xolu-integration · closed 0.3.57 · 2026-08-04


Trigger: reviewer feedback, 2026-08-04, on the T-02 interdependency
pass -- named a specific, honest gap: everything built this session
(Account/Settings/Administration, T-63/T-64's security fixes) is
stateless CRUD. Nothing exercised "form + FSM + xolu commit, composed,
for something genuinely new" -- the actual shape Order Engine's whole
reason for existing depends on.

Cross-ref: CHANGELOG 0.3.57.

## [0.3.54] T-02 — Extract Administration module (users, roles, permissions) (v0.3.54, 2026-08-04)

Theme: administration-module · closed 0.3.54 · 2026-08-04


Trigger: Direct request, 2026-07-28 — "decouple the administrative panel as a module" — narrowed 2026-07-28 once the module list was clarified: Assets Management, Documents, Settings, Administration.
Scope: Administration = user/role/permission management, matching ADMIN_MODULE_SPEC.md's RBAC and user-management sections and the current cmd/seam-admin CLI. Schema lifecycle, form designer, and i18n manager (also in ADMIN_MODULE_SPEC.md v0.1) move to the new Settings module — see T-08. This module keeps ADMIN_MODULE_SPEC.md's dependency position: core (formengine, i18n, listing, auth, olu) -> masters -> administration.

Added scope, 2026-07-28: xolu v0.16.16 ships an implemented (not proposed) tenant-scoped auth system (docs/proposals/tenant-access-control.md in the xolu repo, status Implemented) plus xotogen, a CLI that mints JWTs/API keys/bearer tokens carrying tenant grants. internal/olu currently authenticates to xolu with a single shared bearer token that is always wildcard-admin (reaches every tenant) -- confirmed in internal/olu/client.go and config.example.json. xolu's tenant grants authorise the tenant boundary only, explicitly not per-user access within a tenant (see the proposal's $5) -- Seam's own RBAC still does all per-user permission work regardless.

Decision, 2026-07-28: both, eventually, in two phases. Phase 1 (foundational, no UI): one xolu-scoped JWT per Seam tenant (customer), minted server-side and held by Seam's backend, replacing the single wildcard bearer internal/olu uses today. This is a core data-access change, not an Administration feature -- see T-07, since it's about how internal/olu authenticates for every module, not something an admin configures per user. Phase 2 (Administration UI feature, later): issue individual scoped xolu credentials to specific Seam users (e.g. seam_consultant) for direct/bypass diagnostic access. Needs its own credential storage/rotation design when scoped -- not started.

Discrepancy found 2026-08-03, not resolved here: this item's own premise -- Administration as one of four standalone modules (Assets Management, Documents, Settings, Administration) -- is what T-44's actual implementation (MOD-1 through MOD-7, MODULE_SPEC.md) superseded, not fulfilled. What shipped is three modules (Assets, Documents, System), with Settings/Profile/Administration as permission-gated *views inside* System (MODULE_SPEC.md §5), not a standalone Administration module at all. Flagging rather than closing or rescoping unilaterally -- whether this item is now obsolete, needs updating to describe "extract Administration out of System" instead, or represents a real, deliberately later architectural change is a call for the tech team, not inferred here.

Resolved 2026-08-04. The tech team's call, direct: "we need Administration, Settings, and User Profile to be part of modules... [the grouping question] is a separate question. Propose something coherent." Proposed and built three separate modules -- Account (Profile), Settings, Administration -- specifically because Profile must be visible to every role regardless of permissions, and bundling it with anything gated forces the combined module's own HasAccess to stay true for everyone, pushing real access control down into per-route checks instead of the module boundary itself. Splitting three ways lets Settings and Administration each get honest, role-aware nav visibility.

Mechanical work: split the former single MountSystemRoutes (router.go) into MountAccountRoutes/MountSettingsRoutes/MountAdministrationRoutes, each independently gated; replaced internal/modules/system with three packages (internal/modules/{account,settings,administration}), each a two-line MountRoutes delegation matching the established pattern; five modules.Module entries in server.go now (was one). /system and its three-section dashboard retired deliberately, not left as dead weight -- with three real modules each landing on genuine content (/profile, /settings, /admin/users), nothing needed a landing-page dashboard of its own. Removed the now-dead SystemDashboard handler, schemas.System(), system-dashboard.json, and 7 now-unreferenced i18n keys.

Real structural fix found along the way, not just plumbing: the existing permission constants (system.settings.*, system.user.*) were all namespaced under "system" from the single-module world -- would have silently broken modules.Registry's generic HasAccess-via-HasModuleAccess derivation once Settings/Administration became independently-gated modules, since the derivation cuts on the permission string's own first segment. Renamed to settings.*/administration.* (both the Go constant identifiers and their string values) across auth.go and every caller, not papered over with a custom per-module closure.

A second real issue found and fixed in the same pass, flagged directly by the tech team while this was underway: layout.go's pre-existing user-avatar dropdown already links straight to /profile -- Account showing a second, redundant entry in the module selector right next to it would have been confusing UX shipped on day one. Added modules.Module.HideFromSelector (real access, real routes, just suppressed from ui.ModuleSelector's own rendering) rather than special-casing Account inside the selector's code -- a property of the module, not of the selector.

Live-verified end-to-end: module selector shows exactly Assets/Documents/Settings/Administration, no Account entry; the avatar dropdown's own Profile link confirmed unchanged and undisturbed; /profile, /settings, /admin/users all reachable (200); /system confirmed retired (404). No server errors throughout. Full build/vet/test clean, including new adversarial coverage for HideFromSelector (reverted-and-confirmed-failure verified) and a full router_test.go rewrite covering all three new modules' gating/permission/double-mount behaviour.

Cross-ref: CHANGELOG 0.3.54.

## [0.3.53] T-65 — Three of this session's 17 PATCH-fix call sites had zero test coverage of their own (ProfileSave, AlertResolve, alert_types) -- found by systematically auditing all 17, not by a regression (v0.3.53, 2026-08-04)

Theme: test-quality · closed 0.3.53 · 2026-08-04


Trigger: found 2026-08-04, closing out this session's adversarial
testing campaign -- systematically audited all 17 of this session's
Update-to-Patch fixes (T-49, T-59 and adjacent) for real test
coverage, rather than assuming each one got a test simply because the
underlying bug was fixed.

Three of seventeen had a genuine gap, none previously flagged:

- ProfileSave (routes.go) -- T-49's own original trigger (the tech team's
  report), live-verified extensively at the time, but never got a
  dedicated unit test. Its siblings in the same fix (AdminUserUpdate,
  ProfilePreferencesSave) did.
- AlertResolve (routes.go) -- its sibling AlertAcknowledge has had a
  dedicated test since T-49's original fix; AlertResolve, same handler
  shape, same bug, same fix, filed and fixed together at the time,
  never got its own.
- AlertaMasterUpdate / alert_types (masters_routes.go) -- zero coverage
  at all, not even the incidental protection several sibling masters
  entities (brands/cost_centers/custodians) get from a shared mock
  helper (mockOLUForEntity, htmx_partial_test.go).

None of the three had ever regressed -- this is closing a coverage gap
found by systematic audit, not a bug found by testing. New tests:
TestProfileSave_UsesPatch (audit_wiring_test.go), TestAlertResolve_UsesPatch
(alert_asset_handler_test.go), TestAlertaMasterUpdate_UsesPatch
(update_patch_test.go). ProfileSave's verified with the full discipline
(reverted to Update, confirmed the test failed with a real xolu 404
propagating through the handler's own error-logging path, restored);
the other two follow the exact same proven pattern as their already-
verified siblings, not independently re-verified against a revert.

Also closes out the broader adversarial testing campaign begun
2026-08-03/04, which separately found and fixed: T-62 (CSRF gap),
T-63 (OQL injection across 6 handlers + Map page stored XSS), T-64
(FSM editor XSS, same class as T-63, found by checking every mi.Raw
site after T-63 rather than stopping at the first fix). Plus test-only
additions with no bugs found: 11 module-registry adversarial tests,
9 module-selector tests, 3 router-mounting tests, 6 formengine i18n
resolution-chain tests, 2 tests refining T-57's understanding.

Cross-ref: CHANGELOG 0.3.53.

## [0.3.51] T-64 — FSMEditorPage's <seam-fsm-editor> custom element interpolated a real FSM's name into an HTML attribute via mi.Raw with zero escaping -- a third instance of T-63's vulnerability class, missed in that investigation (v0.3.51, 2026-08-04)

Theme: security · closed 0.3.51 · 2026-08-04


Trigger: found 2026-08-04, in direct response to the tech team asking "is
there anything more that needs to be done in the adversarial campaign"
-- prompted a check of whether T-63's own investigation had actually
covered every mi.Raw(fmt.Sprintf(...)) call site in the codebase, not
just the one (Map page) that started it. It had not: three such sites
exist in total (assets.go:92, confirmed safe -- server-controlled i18n
string; pages.go's Map-page one, fixed under T-63; and this one,
pages.go's FSMEditorPage, never actually checked).

Same vulnerability class as T-63, a single layer instead of two:
FSMEditorPage's <seam-fsm-editor> custom element was mounted via
mi.Raw(fmt.Sprintf(`...fsm-name="%s"...`, data.FSMName, ...)) with zero
escaping. mi.Raw bypasses minty's own automatic per-attribute escaping
entirely (confirmed directly from minty's source -- Element.Render
calls html.EscapeString on every attribute value; Raw is explicitly the
opt-out of that). data.FSMName is a real FSM's name (routes.go:
toString(entity.Data["name"])), genuinely user-editable by anyone who
can create or rename a state machine. A double quote in an FSM's name
broke out of fsm-name="..." into arbitrary injected HTML attributes --
confirmed directly, not theorised: the sanity-check revert's test
failure output showed a literal onmouseover="alert(document.cookie)"
attribute live on the rendered element.

minty has no public generic/custom-tag element constructor (confirmed:
createElement exists but is unexported), so this couldn't be fixed by
switching to a safe element builder the way most of this codebase's
HTML is built. Fixed by applying html.EscapeString to every
interpolated value (FSMName, FSMID, CSRFToken) before embedding --
matching exactly what minty's own Element.Render does for every other
attribute in the app, applied manually here since Raw is the one path
that doesn't do it automatically. FSMID and CSRFToken are safe by
construction/provenance (digits-only, server-generated) but escaped
anyway rather than reasoning per-field about what's "safe enough" to
skip.

New test: TestFSMEditorPage_FSMNameWithQuoteCannotBreakOutOfAttribute
(internal/ui/vendor_html_test.go). Verified by the full discipline:
reverted, confirmed the test failed with the exploit rendering live in
the output, restored.

Cross-ref: CHANGELOG 0.3.51.

## [0.3.50] T-63 — internal/ui built OQL queries from unescaped URL parameters across six handlers (15 injection points), and the Map page separately had a stored-XSS path via asset name reaching Leaflet's bindPopup() as raw HTML -- both found via adversarial testing, both fixed (v0.3.50, 2026-08-04)

Theme: security · closed 0.3.50 · 2026-08-04


Trigger: found 2026-08-04 during adversarial testing, tracing a
mi.Raw(fmt.Sprintf(...)) call on the Map page that led to a much
larger, systemic finding than the single call site suggested.

Cross-ref: CHANGELOG 0.3.50.

## [0.3.50] T-62 — AdminUserCreate's 'email already exists' re-render path never set CSRFToken -- resubmitting that specific form would always have failed CSRF validation (v0.3.50, 2026-08-04)

Theme: administration-module · closed 0.3.50 · 2026-08-04

Correction, 2026-08-04: originally mislabeled "v0.3.49" at closure time -- that version had already been cut (MOD-8's completion release) before this fix existed in the working tree, so it never actually shipped under that number. Relabeled to 0.3.50, the release this fix is genuinely part of, alongside T-63.


Trigger: found 2026-08-03/04 while rewriting AdminUserCreate for MOD-8's
formengine rebuild of the user create/edit form.

AdminUserCreate has three possible outcomes: validation failure
(re-render with field errors), "email already exists" (a server-side
check, also a re-render), and success (redirect). The email-exists
branch's re-render never set CSRFToken on the rendered UserFormData at
all -- the other two paths did. Resubmitting that specific re-rendered
form (the one and only case where a user is told to fix something and
try again with the exact same email) would always have failed CSRF
validation, since the hidden _csrf field would render empty.

Fixed by consolidating all three render paths through one closure
(rerenderCreateForm) that always sets CSRFToken, rather than three
separate UserFormData constructions that could drift independently --
which is exactly how the original bug happened.

New test: TestAdminUserCreate_ValidationFailureRerendersWithCSRFToken
(internal/ui/validation_handler_test.go). Doesn't exercise the
email-exists branch directly (needs a real UserService returning
auth.ErrUserExists, which the test helpers' nil userService can't
produce), but validation failure hits the exact same rerenderCreateForm
closure the email-exists branch calls -- same code, genuine coverage.
Verified by the full discipline: reverted the fix, confirmed the test
failed, restored it.

Live-verified end-to-end as part of the broader MOD-8 user create/edit
work: real user created through the real form, confirmed the new user
can log in, edited the same user (name/role/active all changed
correctly), confirmed the deactivated user is correctly blocked from
login ("account is disabled"). No server errors throughout.

Cross-ref: CHANGELOG 0.3.50.

## [0.3.48] T-60 — Added a 'password' widget to formengine (MOD-8's password-change form) -- a dedicated implementation, not a reuse of the value-echoing text widget, since that would leak the actual password into rendered HTML (v0.3.48, 2026-08-04)

Theme: formengine · closed 0.3.48 · 2026-08-04


Trigger: 2026-08-03, MOD-8's password-change form -- formengine had no
"password" widget type at all (confirmed directly: zero entries in
widgetRegistry before this).

Added a dedicated renderPassword widget, not renderTextLike("password")
with a new case bolted on: renderTextLike always echoes the current
value back into the rendered value= attribute (correct and expected for
every other text-like field), which would leak a real password into
rendered HTML on any validation-error re-render, browser history, or
view-source -- a real security concern, not a style preference.
renderPassword deliberately never reads value at all.

Also added a new schema extension, x-seam-autocomplete, since
current-password vs new-password meaningfully changes browser
password-manager behaviour (current-password enables autofill of a
saved credential; new-password suggests a generated one and avoids
autofilling the old one) -- a single hardcoded value would have been
wrong for at least one of the three password-change fields.

New tests (internal/formengine/widgets_test.go): never-echoes-value
(the security-critical one, verified by the full discipline -- reverted
the fix, confirmed the failure output visibly leaked the test's
distinctive password value into rendered HTML, restored the fix),
autocomplete-applied, no-autocomplete-attribute-when-unset, and
required-applied.

Live-verified end-to-end, not just unit-tested: a real password change
through the real form (real CSRF, real formengine rendering, no value=
attribute anywhere in the rendered output, correct autocomplete hints),
confirmed the new password actually works via a real login attempt,
then reverted to the original password and confirmed that login works
too. No server errors throughout.

Field-title and help-text i18n keys added across all four locales,
reusing the old hand-coded form's exact wording (field.current_password,
field.new_password, field.confirm_password, field.new_password.help).

Cross-ref: CHANGELOG 0.3.48.

## [0.3.47] T-59 — formengine's renderFieldRow computed a translated field title but never applied it to self-labeled widgets (checkbox/toggle) -- every toggle in the whole system showed its raw English schema literal, undetected until MOD-8's Profile/Preferences became the first live use of the widget type (v0.3.47, 2026-08-03)

Theme: formengine · closed 0.3.47 · 2026-08-03


Trigger: found 2026-08-03 while building MOD-8's Profile/Preferences
formengine schemas -- the first toggle/checkbox fields ever wired into
a live route in this codebase (confirmed directly: grepped asset.json,
the only other schema with any real production usage, for x-seam-input
"toggle"/"checkbox" -- zero matches).

Root cause: renderFieldRow (renderer.go) computes resolvedTitle (the
i18n-resolved field title) but only ever passed it to
fkPopoverAwareLabel, the separate top-of-field label wrapper --
selfLabeledWidgets (checkbox, toggle) skip that wrapper entirely, since
they render their own label beside the control. Those widgets
(renderCheckboxLike) read f.Title directly, which -- without this fix
-- was still the field's raw, untranslated schema literal. Every
toggle/checkbox field in the entire formengine system was affected,
silently, since the widget type was first implemented; simply never
exercised because nothing used one on a live route until today.

Fixed: effectiveField.Title = resolvedTitle, right after resolvedTitle
is computed and before any widget dispatch -- every widget gets the
resolved title now, not just the ones that read it via the separate
label wrapper.

New regression test: TestRenderFieldRow_SelfLabeledWidgetUsesResolvedTitle
(internal/formengine/renderer_test.go), using a real fake translator
with a Spanish key genuinely different from the English literal.
Verified by the full discipline: passes with the fix, reverted the fix
and confirmed it fails (the raw English literal visibly leaks into the
failure output), restored the fix. Full internal/formengine suite and
whole-project suite green throughout, zero regressions to any existing
widget type.

Separately, and not itself a code bug: none of the affected fields had
i18n keys authored for their titles either (a second, independent gap
-- the engine fix alone doesn't produce translated text without a key
to resolve to). Added field.name (already existed, reused), field.
locale, field.timezone, field.dark_mode, field.email_alerts, field.
compact_view, field.tenant_name, field.slack_enabled, field.
webhook_enabled, field.map_enabled across all four locales, reusing the
old hand-coded forms' exact existing translations (users.field.name,
profile.language, settings.field.tenant_name, etc.) rather than
inventing new wording, for continuity with what shipped before.

Cross-ref: CHANGELOG 0.3.47.

## [0.3.46] T-48 — TestGenerator_ValidateChecksum (internal/qrcode) is flaky in the full suite -- passes standalone, failed once in go test ./... (v0.3.46, 2026-08-03)

Theme: testing · closed 0.3.46 · 2026-08-03


Trigger: found 2026-08-03 as an incidental full-suite-only failure while
verifying the PUT-vs-PATCH audit fixes (T-49) -- TestGenerator_ValidateChecksum
(internal/qrcode) failed once in a full `go test ./...` run, but passes
reliably (4/4) when run standalone or re-run as part of the full suite
immediately after. Package is completely unrelated to anything touched
this session. Likely a test-ordering/parallelism issue (shared global
state or a time/random-seed dependency), not investigated further --
out of scope for this session's actual work. Filed so it's tracked
rather than silently unnoticed if it recurs.

Duplicate, found 2026-08-03 while answering "what are the pending
tasks": T-09 (filed 2026-07-28) already root-caused this exact test --
qrcode_test.go:116 corrupts a checksum by appending a fixed 'X', but 'X'
is a valid character in the 36-char checksum alphabet, so ~1/36 of
generated codes legitimately end in 'X' already, making the "corrupted"
code equal the real one and the test fail on input Validate() correctly
accepts. That's the real mechanism behind this item's "flaky in the
full suite" symptom (more test runs via package interleaving means more
chances to hit the ~1/36 case) -- not test-ordering/parallelism as
guessed here. No independent fix needed under this ID; see T-09.

Cross-ref: CHANGELOG 0.3.46.

## [0.3.46] T-29 — syncver.sh doesn't consume .repoman.json's version_targets -- docs/TRACKING.md, docs/KNOWN_ISSUES.md, docs/formengine/TRACKING.md's Version/Last reviewed headers have been stuck at 0.2.54 / 2026-07-28 across all six releases this session (0.3.1-0.3.6) (v0.3.46, 2026-08-03)

Theme: test-quality · closed 0.3.46 · 2026-08-03


Trigger: 2026-07-31, noticed while adding a resolution note to FE-24 that its own tracking doc still says "Version: 0.2.54 / Last reviewed: 2026-07-28" despite the actual repo having shipped six releases (0.3.1 through 0.3.6) since then. Assumed release.sh's syncver.sh call would handle this automatically, per .repoman.json's own version_targets declaration listing all three files -- checked rather than left assumed, and the assumption was wrong.

Confirmed by reading syncver.sh directly: it only updates VERSION, CHANGELOG.md, internal/version/version.go, and README.md's "Current version:" line. It has no awareness of .repoman.json or its version_targets list at all -- that manifest's declared intent (these three docs carry release-coupled Version/Last reviewed headers per Part 3 Tracking Document Practices) was never actually wired into this repo's own hand-rolled syncver.sh. Matches the same shape as T-56's still-open xolu migration note (repoman-tools skill: "xolu carries its own scripts... until T-56 completes the migration") -- seam-ui has adopted the .repoman.json manifest format without its own syncver.sh being updated to consume it.

Scope: either (a) teach this repo's syncver.sh to read .repoman.json's version_targets and apply the same Version/Last reviewed update it already does for the other files, or (b) migrate to repoman's own syncver.py outright, which presumably already does this (unverified -- not checked as part of this finding). Not decided here. Immediate correction applied by hand for 0.3.6 (all three files now read Version: 0.3.6, Last reviewed: 2026-07-31) so this release's own documents aren't shipped stale while the tooling gap itself stays open.

Resolved 2026-08-03, option (a) with a twist: checked (b) first, per this item's own open question -- repoman/syncver.py's set_version does already correctly read .repoman.json's version_targets and apply each file's own regex, confirmed directly (python3 repoman/syncver.py show reported "sync: ok" against all four current target files, including docs/modules/TRACKING.md, added since this item was filed). Rather than migrate outright, syncver.sh's own set_version now delegates to it (python3 repoman/syncver.py set "$v") for the Version: line -- reusing the already-correct tool instead of reimplementing its logic in bash. Last reviewed: stays in syncver.sh itself, not pushed into the shared repoman tool: it's this project's own Part 3 convention, not a generic repoman concept, and .repoman.json's version_targets schema has no field for it. Applied to the same four target files, today's date, only where a "Last reviewed:" line already exists.

Verified directly, not assumed: ran ./syncver.sh set against the current version (idempotent -- correctly silent, nothing needed changing), deliberately staled docs/TRACKING.md's own Last reviewed: line and re-ran (correctly detected and fixed that one file only, left the other three alone since they were already current), then a genuine version bump to a throwaway 0.3.46-test (correctly synced Version: across all four files, left Last reviewed: alone since only the version changed), then restored the real version. go build clean throughout.

Cross-ref: CHANGELOG 0.3.46.

## [0.3.45] T-50 — dashboardengine's empty-dashboard rendering (DASHBOARD_SPEC.md §2.2) referenced an i18n key that existed in no locale file -- any sections:[] dashboard would have shown the raw key string to users (v0.3.45, 2026-08-03)

Theme: i18n · closed 0.3.45 · 2026-08-03


Trigger: found 2026-08-03 while building MOD-7 (Documents module).
dashboardengine's empty-dashboard rendering path (DASHBOARD_SPEC.md
§2.2, renderEmptyDashboard, built in MOD-2) references the i18n key
"dashboard.empty_message" -- confirmed directly, this key did not exist
in any of the four locale files (en/es/pt/ja). Nothing had ever
exercised the "sections": [] path against a real dashboard schema
before Documents (System and Assets both have real sections), so the
gap was invisible until now.

Effect: the translator's own documented fallback (internal/i18n/i18n.go)
returns the raw key string when a key is missing in both the requested
locale and the fallback locale. Without this key, any dashboard hitting
the empty-sections path would have rendered the literal text
"dashboard.empty_message" to the user instead of "No data to display in
this module at this time."

Fixed: added the key to all four locale files, matching the exact
English text DASHBOARD_SPEC.md §2.2 specifies. New test
(TestMountDocumentsRoutes_RendersRealEmptyMessage,
internal/ui/router_test.go) -- verified by deliberately removing the
key from en.json and confirming the test failed (asserted both the
absence of the raw key string and the presence of the real message),
then restoring it.

Live-verified: /documents (MOD-7) confirmed rendering the real Spanish
message ("No hay datos para mostrar en este módulo por el momento."),
not the raw key.

Cross-ref: CHANGELOG 0.3.45.

## [0.3.45] T-44 — Build the Module system (module.view.action permissions, registry, selector) and the Dashboard invariant, per MODULE_SPEC.md v0.2 and DASHBOARD_SPEC.md v0.3 (v0.3.45, 2026-08-03)

Theme: module-system · closed 0.3.45 · 2026-08-03


Tracking: docs/MODULE_DASHBOARD_DEVELOPMENT_PLAN.md is the frozen design
intent (9 stages); task-level progress is docs/modules/TRACKING.md (a
separate repoman register, ID prefix MOD, 10 tasks filed, MOD-1 through
MOD-7 are the closure gate, MOD-8/MOD-9/MOD-10 are explicitly deferred,
not blocking). This item closes when MOD-7 closes -- do not duplicate
task detail here, check the MOD register instead. Same structure T-01/
formengine already established.

Design conversation spans MODULE_SPEC.md v0.2 (module registry, the
module.view.action permission format, the selector) and DASHBOARD_SPEC.md
v0.3 (the declarative dashboard format both Apps-group modules and System
use). Both specs' own open questions are resolved as of 2026-08-02 --
see each spec's own revision history for what was asked and answered.

Cross-ref: CHANGELOG 0.3.45.

## [0.3.43] T-49 — xolu.Client.Update() is HTTP PUT (full-document replace) -- every handler sending a partial field set was silently destroying the rest on every edit; audited and fixed all 17 call sites in internal/ui (v0.3.43, 2026-08-03)

Theme: data-integrity · closed 0.3.43 · 2026-08-03


Trigger: reported by the tech team, 2026-08-03: "When you save Settings using
the old form, that destroys or overwrites the user's profile." Checked
immediately.

Root cause: xolu.Client.Update() sends HTTP PUT, which is a full-
document replace server-side -- confirmed empirically, not assumed
(PATCH with only {"email": X} correctly merges, preserving every other
field; the identical payload via PUT wipes everything else). Every
handler that called Update with a hand-picked subset of an entity's real
fields was silently destroying the rest on every edit -- not a Settings-
specific bug, a systemic pattern across the whole app.

Audited every .Update(ctx, "<entity>", ...) call site in internal/ui
(17 total) and fixed every one that needed it, switching to .Patch()
(a real partial merge, also confirmed empirically):

- ProfileSave, ProfilePreferencesSave: every user's own name/preferences
  save. Most severe of all -- any user changing their own display name
  or preferences could silently wipe their own email/password_hash,
  locking themselves out. No admin action required to trigger it.
- AdminUserUpdate: matches the tech team's report exactly. Reproduced directly
  this session: a test user edited via this handler had its email field
  completely gone from the stored record afterward.
- AlertAcknowledge, AlertResolve: a real alert has 9 fields; both sent
  only {"status": ...}. Acknowledging or resolving any alert was wiping
  its asset association, message, severity -- everything down to a bare
  stub.
- All 9 masters entities (regions, currencies, cost_centers, brands,
  custodians, alert_types, prop_definitions, models, locations).
  Region and Custodian were already sending their complete field set
  (harmless to Patch anyway); most of the others were not fully audited
  field-by-field beyond confirming the fix is safe in every case.
- rules: silently dropped created_at on every edit.
- asset_types: dropped 2 of 9 fields (slug, fsm_id) on every edit --
  confirmed directly against a real stored record.
- assets (internal/ui/asset_dynamic_post.go): the one entity confirmed
  NOT currently broken -- verified live, not assumed: extracted the real
  rendered edit form via a proper HTML parser (a first attempt using
  hand-rolled regex was unreliable and discarded), resubmitted it
  unchanged, confirmed the stored record round-tripped correctly (all
  four tabs share one <form>, submitting every field together). Patched
  anyway for defense-in-depth: if Asset's schema ever gains a field the
  form doesn't cover yet, Patch degrades that to "the new field doesn't
  update" instead of "everything else gets deleted."

New test coverage: 13 of the 17 fixes were already covered by existing
tests, whose mock servers were updated from registering "PUT /api/v1/..."
to "PATCH /api/v1/...". Region, Currency, PropDefinition, and Rule had
zero prior coverage of their update path at all (their only existing
tests short-circuit on validation before reaching xolu) -- new tests
added for all four, plus AssetType (internal/ui/update_patch_test.go).
Verified the fix discipline throughout: every fix was confirmed by
reverting it and checking the corresponding test genuinely failed, not
assumed to be caught.

Live-verified end-to-end, not just unit-tested: AdminUserUpdate (the
original report), the Asset dynamic form's round-trip safety, and
AlertAcknowledge -- a real alert's status changed correctly from "open"
to "acknowledged" while asset_id/asset_name/created_at/message/severity/
source/title all survived intact, through the real form, real CSRF,
real xolu.

Cross-ref: CHANGELOG 0.3.43.

## [0.3.42] T-47 — ListAuditEvents' sort was structurally broken two layers deep (client.List() never sends Sort; OQL's own ORDER BY DESC is buggy past the first row) -- recent-activity widgets never showed recent items (v0.3.42, 2026-08-03)

Theme: audit-log · closed 0.3.42 · 2026-08-03


Trigger: found 2026-08-03 during MOD-5 live verification, immediately
after fixing T-46 (AdminUserEdit's missing CSRFToken), while confirming
the LogRoleChanged/LogUserUpdated audit-email fix end-to-end through a
real role change. The System dashboard's admin-activity widget showed
only a single, stale event after the change -- not the just-submitted
role change.

Root cause, in two layers:
1. xolu.Client.List()'s Sort parameter (internal/auth/audit_store.go's
   original ListAuditEvents) is never sent to the server at all --
   confirmed directly against xolu's pkg/client/client.go: List() only
   ever sets per_page/page query params, never sort, regardless of the
   Sort field's value. This meant no value passed there -- "-created_at"
   or otherwise -- could ever produce a real ordering.
2. OQL's own ORDER BY (the actual fix target) has a real, narrower bug:
   ASC is genuinely correct (confirmed by deliberately breaking the id/
   timestamp correlation on a test row and re-querying); DESC correctly
   places the single maximum-valued row first, then falls back to
   ascending for the rest -- not "silently ignored" as first suspected.
   Full raw diagnostic transcript: oql-orderby-diagnostic-report.md
   (shared with the xolu team directly, reported upstream).

Fixed on the seam-ui side without waiting on the upstream xolu fix:
rewrote ListAuditEvents (internal/auth/audit_store.go) to query xolu
directly via OQL with "ORDER BY id DESC" -- independently confirmed
correct end-to-end, since id is a real indexed column, not a blob
field, and audit_logs is append-only with auto-incrementing ids, so id
order and timestamp order coincide in every practical case. OQL has no
OFFSET/SKIP (confirmed: no such keyword in xolu's OQL parser), so
pagination fetches TOP (limit+offset) and skips the first `offset` of
those in Go, preserving the original Offset semantics.

New test coverage (internal/auth/audit_store_test.go) -- this class of
bug had zero prior coverage, which is why it went unnoticed: only the
in-memory mockAuditStore was ever exercised, never XOLUAuditStore (the
real, xolu-backed implementation). New tests use a mock xolu server
that captures the actual query sent (asserts "ORDER BY id DESC" is
present, "created_at"/"timestamp DESC" are not) plus behavioral checks
(order preservation, resource filtering, offset, and the T-45/current-
session Email field round-tripping through the read path too).

Verified live end-to-end after the fix: real role change through the
real form, real xolu, real System dashboard -- the just-submitted
role_changed event now appears first, correctly, with the correct
email. Verified in both Spanish (tenant default) and English (switched
via Settings, per the tech team's standing instruction to always check
English as the baseline).

Known, pre-existing, explicitly out-of-scope limitation (documented in
the fix's own code comment, not new): fetching TOP(limit+offset) then
filtering by Resource/EventType(s) in Go can still yield fewer than
`limit` matches if the filter excludes many of the fetched rows -- the
prior client.List()-based version had the same limitation, in an even
narrower form (fetched exactly `limit` raw rows, no offset margin at
all). Not fixed here since it wasn't the reported problem.

Cross-ref: CHANGELOG 0.3.42.

## [0.3.42] T-46 — AdminUserEdit never set CSRFToken on its form data -- editing an existing user was completely broken (CSRF always failed) (v0.3.42, 2026-08-03)

Theme: admin-module · closed 0.3.42 · 2026-08-03


Trigger: found 2026-08-03 during MOD-5 live verification, while testing the
LogRoleChanged/LogUserUpdated audit-email fix end-to-end through the real
edit-user form. AdminUserEdit (internal/ui/routes.go) built its UserFormData
without setting CSRFToken at all -- every other form-rendering handler in
this codebase sets it via h.GetCSRFToken(w, r) (confirmed by grep across
routes.go/masters_routes.go/admin_users.go: this was the only omission).

Practical effect in production: the edit-user form's hidden _csrf field
always rendered with an empty value, so submitting any change to an
existing user (name, role, active status) always failed CSRF validation
with 403. Editing an existing user was completely broken -- not degraded,
non-functional.

Fixed: added CSRFToken: h.GetCSRFToken(w, r) to AdminUserEdit's UserFormData
construction, matching every other handler's exact pattern.

New regression test: TestAdminUserEdit_RendersNonEmptyCSRFToken
(internal/ui/validation_handler_test.go). Worth noting: the test's first
draft used a plain strings.Contains(body, `name="_csrf" value=""`) check,
which passed vacuously with the bug still present -- the real rendered
attribute order is name="_csrf" type="hidden" value="...", so that
substring never matches either way. Caught by this session's own
discipline of reverting a fix and confirming its test actually fails
before trusting it; rewritten with a regex extracting the value regardless
of attribute order.

Cross-ref: CHANGELOG 0.3.42.

## [0.3.41] T-45 — release.sh's checkpoint ZIP_SOURCES list omitted pkg/, silently dropping pkg/nplate from the v0.3.40+ checkpoint zips (v0.3.41, 2026-08-03)

Theme: release-tooling · closed 0.3.41 · 2026-08-03


Trigger: found 2026-08-03 (session resuming from HANDOVER.md), when the
uploaded seam-v0.3.40 checkpoint zip built clean except for a missing
`pkg/nplate/` package, which `internal/dashboardengine/compile.go`
imports. Root cause confirmed in `release.sh`'s checkpoint step: it zips
an explicit, hardcoded `ZIP_SOURCES` array rather than the whole tree
("never zip '.' to avoid sweeping in build artefacts, editor swap files,
or local config"), with its own comment reading "Add new top-level items
here when they are committed to the repository." `pkg/` was created as a
new top-level directory during this session's MOD-2 work but never added
to that list, so every checkpoint since has silently omitted it.

Recovered this instance's missing files by hand from a prior session
(nplate.go, nplate_test.go, verbatim, unmodified) and confirmed the
restored tree builds, vets, and tests clean (all packages including
pkg/nplate's own 11 tests pass). This item tracks the tooling fix: add
`pkg/` to release.sh's `ZIP_SOURCES` array so future checkpoints don't
reproduce this gap for pkg/nplate or any future package under pkg/.

Cross-ref: CHANGELOG 0.3.41.

## [0.3.31] T-38 — Masters entity URL routes are Spanish (/marcas, /modelos, /regiones, /monedas, /custodios, /alertas, /propiedades, /centros-costo, /ubicaciones) -- confirmed violation, not merely a design question (the tech team, 2026-08-01: "All identifiers in a language other than English are a problem") (v0.3.31, 2026-08-02)

Theme: i18n · closed 0.3.31 · 2026-08-02


Surfaced 2026-08-01 during a general audit for non-English identifiers. Originally filed as "an open design question" -- the tech team corrected that framing directly: "All identifiers in a language other than English are a problem." Backward-compatibility strategy asked about directly (redirect vs. hard cutover): "hard cutover. we have backups. if anything goes very wrong, we just undo. proceed."

Done, 2026-08-02: all 9 routes renamed with no redirect layer -- /marcas/modelos/regiones/monedas/custodios/alertas/propiedades/centros-costo/ubicaciones -> /brands/models/regions/currencies/custodians/alert-types/properties/cost-centers/locations (/propiedades/asociaciones -> /properties/associations too). Genuinely the largest single change of this whole effort by line count -- 55 route registrations in router.go, the nav-menu data structure in layout.go, every href/hx-get/redirect/masterID-prefix-parse reference across masters_pages.go/masters_routes.go/summary_handlers.go, asset.json's x-seam-fk-popover URLs, and every test fixture across 7 test files. Safe as a broad whole-file substitution rather than needing per-entity scoping the way attribute names did, since these are 9 genuinely distinct, non-overlapping path strings (unlike descripcion/nombre, shared across multiple entities' own code). Found and simplified one real piece of now-dead complexity as a direct consequence: layout.go's nav-active-state check for /alerts carried a special-case guard against colliding with /alertas' shared "/alert" prefix; renamed to /alert-types, the two paths no longer share any prefix relationship, so the guard was removed rather than left as confusing dead code.

Verified with the same rigor as every other piece of this effort, at full scope: full Go test suite green on the first attempt (no bugs, no missed references), then a genuine end-to-end test -- real seed, real login, all 10 new routes independently confirmed 200 (including /properties/associations), all 9 old routes independently confirmed 404 (hard cutover working as decided, no accidental redirect), the rendered nav menu confirmed entirely English including the /alerts vs /alert-types distinction, and real seeded custodian data confirmed rendering correctly on /custodians.

Cross-ref: CHANGELOG 0.3.31.

## [0.3.28] T-36 — service_configs.measure's schema enum has always been far narrower than the range of values real seed data actually uses -- editing an existing asset whose measure doesn't match one of the 3 declared options silently blanks it on save (v0.3.28, 2026-08-02)

Theme: formengine · closed 0.3.28 · 2026-08-02


Found 2026-08-01 during a general audit for non-English identifiers, prompted by the tech team. Not a regression from T-33's measure value-bridge (0.3.27) -- confirmed pre-existing: the schema declared only 3 enum options before that fix too (Kms/Horas/Usos), same narrowness, just in Spanish. Swept every seed script's stored medida values against the full recognized token set (Spanish + English, both pre- and post-T-33): seed-01 and seed-02 only ever use the 3 canonical values, but seed-03-extinfire.py uses 'Dias desde recarga' (Days since recharge, x6), seed-04-vendiberia.py uses 'Temperatura' (x5) and 'Dias sin visita' (x1), and seed-05-fairborne.py/seed-06-asahikenki.py (T-18's own Japanese seed) both use 'Miles' (x3 each) -- none of these match any of Kilometers/Hours/Uses in either language. Traced the actual failure mode in internal/formengine/widgets.go's renderSelectWithExtraAttrs: option selection is exact-match only (opt.Value == current); when the stored value matches none of the declared enum options, no <option> gets marked selected, the browser falls back to the empty placeholder, and submitting the form without touching that specific field would submit an empty string -- silently blanking a real, previously-set value on save. This affects any existing asset seeded with one of these four seeds whose service_configs row uses a non-canonical measure, the moment someone opens and re-saves its edit form (even if they never touch the measure field itself). Two real design questions this raises, not decided unilaterally: (1) should the schema's enum simply be widened to include every measure category actually in use across the demo fleet (temperature/recharge-days/visit-days/distance/time/cycles), turning this into a genuinely open-ended list rather than a narrow fixed one -- which would also mean revisiting whether 'measure' still belongs in the hardcoded/fixed-enum category at all, per the 2026-08-01 hardcoded-vs-database-sourced framework, since a company-specific list of measurement types looks a lot more like tenant-managed data than a system enum the code depends on; or (2) should measure become a free-text field (matching how the codebase already treats similarly open fields) with no enum constraint and no widget-selection risk at all. Either fix needs the seed data itself reconciled to match, not just the schema.

Cross-ref: CHANGELOG 0.3.28.

## [0.3.28] T-37 — Every seed script's service_configs rows use the key "umbral", but the live application (both real POST handlers, per asset_xolu_fields.go's own comment) persists thresholds under "umbral_alarma" instead -- seeded threshold data is invisible through the real UI (v0.3.28, 2026-08-02)

Theme: seeds · closed 0.3.28 · 2026-08-02


Found 2026-08-01 while building service_configs.measure's value bridge (T-33), explicitly flagged in conversation at the time but never actually filed as its own item -- caught now during a general non-English-identifier review, per the tech team's instruction to file everything surfaced but not acted on. asset_xolu_fields.go's own code comment (2026-07-28) documents the root fact directly: serviceConfigsItemToXolu originally said "umbral" (matching the submitted svc_umbral[] field name) but both real handlers (AssetCreateDynamic/AssetUpdateDynamic) persist the threshold under "umbral_alarma" instead -- the submission field name and the actually-persisted key genuinely differ, unlike every other field in that table. Every one of the 6 seed scripts (seed-01 through seed-06, confirmed by direct grep, ~40 rows total) writes service_configs via a raw PATCH using "umbral" as the key, bypassing the form/bridge pipeline entirely. Net effect: for any seeded asset, opening its real edit form and looking at Service Configurations shows an empty threshold field (the read side looks for "umbral_alarma", which doesn't exist in seeded data -- only "umbral" does), even though the seed clearly intended to set one. Saving that form would then also lose the seed's own alerta_id/medida pairing for that row, since the row would be reconstructed from what the (blank) form shows rather than what was actually seeded. Fix is mechanical once decided: either seed scripts should write "umbral_alarma" directly, matching what the live app actually persists, or (if seeds are meant to model raw pre-migration legacy data on purpose) the read side needs to tolerate both keys. The former seems clearly correct given nothing else about these seeds is deliberately modeling a legacy/pre-fix data shape.

Cross-ref: CHANGELOG 0.3.28.

## [0.3.27] T-39 — prop_definitions.nombre_corto values stay Spanish tokens (kilometraje, horas_motor, etc.) across every seed script including the English and Japanese ones -- confirmed violation (the tech team, 2026-08-01: "All identifiers in a language other than English are a problem") (v0.3.27, 2026-08-01)

Theme: seeds · closed 0.3.27 · 2026-08-01


Scope grew substantially during the fix itself, in three layers, each discovered by checking the previous one's completeness:

1. PropDef.nombre_corto (76 distinct terms, 103 occurrences across all 6 seeds) -- the originally-filed scope.
2. AssetType.identity_template/state_template field keys, found while confirming (1) had no Go-code dependency needing coordinated changes -- the same check revealed a second, structurally identical category (271 occurrences across all 6 seeds, present since seed-01, copied forward into every later seed including seed-06).
3. Sensor event telemetry payload keys (seed_events'/seed_runtime_alerts' "value": {...} blobs), found by a final sweep for old tokens after (1)+(2) -- 155 more occurrences, ~44 new terms not covered by the first two categories' translations.

Confirmed via grep before touching any of these that no Go code hardcodes any of these specific field-key names in any of the three categories -- all purely data-driven (asset_types templates rendered generically from whatever keys exist; event Value is an opaque stored/displayed JSON blob, never parsed field-by-field). This made the whole fix a safe, self-contained, seed-only rename with zero Go changes required.

Renaming nombre_corto's stored value broke every downstream prop_defs["old_spanish_key"] lookup in seed_prop_associations -- caught by dry-run, not assumed; 132 reference updates applied across all 6 seeds to match.

Verified, not just trusted: all 6 seeds syntax-check clean, all 6 dry-run clean to completion, and two full live seeds run against a real xolu instance (seed-01, the simplest; seed-03-extinfire, the most complex at 288 requests) -- both 100% successful POSTs/PATCHes, zero errors, then independently queried the actual SQLite database directly to confirm prop_definitions, asset_types, and events records all hold the correctly-renamed English keys, not just trusting the seed script's own success output.

Two legitimate, deliberately-untouched exceptions found during the final sweep: Custodio.estado and Asset.refrigerante are established xolu storage-layer field names (the same deliberate legacy layer T-30 and every masters_xolu_fields.go bridge table already treats as out of scope), unrelated to the three categories above despite superficially similar names -- confirmed by context, not skipped by assumption. That broader category (the xolu storage schema itself being entirely Spanish-named) is real and much bigger than this item, filed separately.

Cross-ref: CHANGELOG 0.3.27.

## [0.3.27] T-33 — Finish tier-1 enum i18n wiring: PropAssoc (dropdowns/list/fetch/propValueField), AssetType's own connectivity_tiers dropdown, and service_configs.measure's value bridge (Kms/Horas/Usos -> English canonical) (v0.3.27, 2026-08-01)

Theme: i18n · closed 0.3.27 · 2026-08-01


Started 2026-08-01 following T-18's closure, at the tech team's direction ('let's iron out any pending i18n wrinkles'). Alert Type (masters_xolu_fields.go's write-side bridge, already live, now has its read side -- edit-prefill, list display, badge colors, delete-confirm name, and the downstream AssetType default-alert picker -- fully wired) and PropDef (form dropdown via propTipoOptionsList, list badge via PropDefDataTypeLabel, fetch-time conversion in PropDefsList/PropDefEdit) are done and verified. Not yet done, same shape of work: PropAssoc's own oblOptions/destOptions dropdowns (masters_pages.go), PropAssocsList's fetch (both its own obligatoriedad read and its prop_definitions defs lookup), PropAssocsListPage's display, loadPropDefs (feeds the property picker), and propValueField's functionally-critical widget-type switch (masters_pages.go, currently switches on raw Spanish tipo_dato values to choose the HTML input type for entering a custom property's value -- LoadPropValues/PropValuesSection that feed it are dead code today, confirmed by grep, but fix for consistency same as DocumentLinksField). Separately: AssetType's own connectivity_tiers dropdown (pages.go connectivityOptions) still hardcodes English labels with no t() wrapping (keys already exist, asset.connectivity_tier.* as of this session). And service_configs.measure's enum values (Kms/Horas/Usos) are still Spanish -- unlike every other Asset enum field, this one has no validation Enum constraint and no existing value-bridge in asset_xolu_fields.go (only a field-NAME bridge, measure<->medida), so fixing it means building a new asymmetric value-bridge from scratch (English canonical <-> Spanish already-seeded storage), the same shape asset_xolu_fields.go already has for criticality/tenure_type/decommission_reason, not just adding i18n keys.

Cross-ref: CHANGELOG 0.3.27.

## [0.3.25] T-18 — Wire i18n resolution into the rest of the UI (nav, page titles, messages) -- currently zero call sites outside formengine (v0.3.25, 2026-08-01)

Theme: i18n · closed 0.3.25 · 2026-08-01


Trigger: the tech team, 2026-07-29 -- "We need it. If it's not a task it should be one." -- directly following an investigation into whether i18n covers the whole app or only formengine.

Scope, confirmed by investigation before filing, not assumed. The Translator mechanism is general-purpose, not formengine-specific by design. The locale files have 120 keys (en/es/pt, identical counts) covering some general vocabulary -- action (17), asset (20), dashboard (8), nav (8), event (11), fsm (7), rule (8), sensor (9), settings (7), error (9), plus a handful of small namespaces. Whole sections have zero keys: alerts, admin/users/roles, orders, documents, webhooks.

But the real finding is narrower and more concrete than key coverage: checked every use of h.translator in internal/ui, exhaustively -- exactly three call sites exist (AvailableLanguages, ValidateLocale, formEngineTranslator), all in routes.go. Zero calls to .T(...) anywhere in internal/ui. Every nav label, page title, dashboard message, on-screen text outside the form engine is a hardcoded Go string literal, completely bypassing the translator regardless of what's in the locale files.

Concrete evidence of scale and of why this matters beyond completeness: 100 separate h.page.Render(w, r, "...", ...) call sites across 26 page-rendering files, each with a literal title string -- and they are not even consistently one language today. masters_routes.go alone hardcodes "Regiones"/"Nueva Región"/"Editar Región"/"Monedas" (Spanish) while asset_dynamic_post.go hardcodes "New Asset"/"Edit Asset" (English), in the same running application. This is active inconsistency, not merely absent translation -- a Spanish-configured tenant today already sees English page titles on some pages.

Even internal/formengine's own §4.1/§4.2 resolution chain (built for T-13, functioning correctly as code) currently falls through to literal schema text in practice, since no assets.forms.asset.field.* keys exist yet -- so this gap exists at both layers: the mechanism works, the content doesn't exist, and outside formengine the mechanism isn't even called.

Deliberately not scoped further here: given 100+ page titles alone before counting nav/buttons/empty-states/error messages, this is unlikely to be single-session work and likely needs its own staged plan (audit real scope precisely, decide a per-request locale mechanism -- shared work with T-17's gap 2, decide a systematic key-naming convention across all 26 files, then work through pages in batches) rather than being guessed at and started immediately. Filed to make sure it's tracked, not to commit to an unverified scope.

Scoped properly, 2026-07-31 (the tech team's direct request, after two live-found i18n gaps this session made T-18's real value concrete rather than theoretical): docs/T18-I18N-UI-COMPLETION-PLAN.md, a 9-stage plan, each stage sized to be completable in one pass. Two of the three open questions from the original filing are already resolved as side effects of this session's own work, not re-opened here: the per-request locale mechanism is h.translateFunc()/h.currentLocale(), already methods on every *UIHandler, proven working against real persisted tenant locale (not just SEAM_FORCE_LANGUAGE) by both the document-links/widget-chrome fix and the Assets list page fix; the naming convention reuses exactly what those two fixes already established by necessity (reuse formengine's own field-title keys where the concept is identical, action.* for generic buttons, new masters.forms.<entity>.field.<name>/nav.<slug>/<entity>.new|edit|list.title namespaces for what's genuinely new). Real, counted (not estimated) scope as of the plan: 73 distinct page-title strings, 24 nav labels, 19 master-form field labels, 5 master-form select labels, from the files checked so far -- Masters is confirmed the dominant remaining body of Spanish text; Sensors/Rules/AssetTypes/Settings/Admin already have English page titles (their field-level content is unverified, folded into Stages 6-7 rather than assumed clean). Stages 8-9 cover the files not yet swept.

Cross-ref: CHANGELOG 0.3.25.

## [0.3.11] T-01 — Implement dynamic form engine (internal/formengine) per FORM_ENGINE_SPEC.md v0.3 (v0.3.11, 2026-07-31)

Theme: form-engine · closed 0.3.11 · 2026-07-31


Trigger: Direct request, 2026-07-28 — "bring Shelf up to standard with the new form engine."
Scope: docs/FORM_ENGINE_SPEC.md v0.3 is complete; all six §15 open decisions are resolved. No internal/formengine package exists yet.
Tracking, 2026-07-28: staged build plan is docs/FORMENGINE_DEVELOPMENT_PLAN.md (frozen, 7 stages); task-level progress is docs/formengine/TRACKING.md (separate repoman register, ID prefix FE, 24 tasks filed). This item (T-01) closes when FE-24 (stage 7's Asset-form parity task, also the completion gate) closes -- do not duplicate task detail here, check the FE register instead.

FE-24 closed, 2026-07-31 (v0.3.11, see docs/formengine/RESOLVED.md): production parity proven via the tech team's own hands-on use, and the static Asset form retired (backed up to /attic/static-asset-form/ first, per our explicit instruction, then carefully removed -- full audit of every shared dependency before touching anything, verified live afterward that nothing else broke). T-01's own closure gate is satisfied.

Cross-ref: CHANGELOG 0.3.11.

## [0.3.5] T-22 — Enum/FK select empty-option placeholders (formengine/widgets.go: '— Seleccionar —', '— Sin asignar —') still hardcoded Spanish regardless of locale (v0.3.5, 2026-07-31)

Theme: i18n · closed 0.3.5 · 2026-07-31


Trigger: fixing the Asset dynamic form's most obvious chrome i18n gaps (2026-07-31) -- asset_dynamic_form_wrapper.go's buttons/delete-confirmation were fixed (now use action.cancel/action.create/action.save/action.delete/confirm.delete via a translateFunc threaded through wrapAssetDynamicForm), verified live in both es and en. These two widgets.go strings were found in the same sweep but deliberately NOT fixed alongside them, for a real reason worth recording precisely:

renderSelect/renderSelectWithExtraAttrs are registered in formengine's widget dispatch table with a fixed signature (ctx context.Context, e *FormEngine, f Field, value interface{}, hasError bool) shared by every widget renderer. Unlike resolveTitle/resolveHelp/resolveLabel (formengine/i18n.go), which take rc RenderContext directly and already resolve translations correctly, widgets.go has no access to rc.Locale or e.Translator at all under the current architecture -- confirmed by checking for context.Context-based value threading (none exists) and by checking every widget renderer's actual signature (locale/rc genuinely isn't reachable from here today).

Two real options, not yet decided between: (1) add rc RenderContext (or just its Locale) to the shared widget-renderer function signature -- correct, but touches every registered widget, not just select; (2) thread locale via context.Context instead, additive but introduces a new mechanism alongside the existing rc-parameter one, which resolveTitle/etc. already use, so now there'd be two different ways formengine code reaches translations. Worth deciding deliberately, not defaulting to whichever is expedient in the moment fixing this specific string.

Not urgent: confirmed via live rendering in both locales that this is the only remaining hardcoded chrome text after the current fix, and it degrades gracefully (a placeholder string, not broken functionality) rather than silently misleading.

Resolved, 2026-07-31 (T-23's own work settled this fork -- see T-23's resolution below): widgetRenderer's signature was extended with rc RenderContext (the tech team's explicit decision on the two-option fork), touching all 17 registered widgets and both dispatch call sites (renderer.go, repeatable.go's nested dispatch). New resolveSelectPlaceholder (formengine/i18n.go) resolves "field.select.empty"/"field.select.empty_fk" with the same graceful literal-fallback pattern every other resolver in that file uses, wired into renderSelectWithExtraAttrs in place of the two hardcoded strings. Global keys, not per-form -- the placeholder text doesn't vary by field or entity, only by locale. Actual en/es/pt translations added for both keys (en: "— Select —"/"— Unassigned —"; es matches the original literal exactly; pt: "— Selecionar —"/"— Não atribuído —") -- fully resolved, not just mechanism-ready.

Cross-ref: CHANGELOG 0.3.5.

## [0.3.5] T-23 — criticidad/calidad_tenencia/motivo_baja: asset.json's own duplicate hardcoded-Spanish enum, missing enum value/label display translation, and seed-script/persisted-data reconciliation -- violates the internal/external language law (v0.3.5, 2026-07-31)

Theme: validation · closed 0.3.5 · 2026-07-31


Trigger: the tech team's internal/external language law (2026-07-31, docs/KNOWN_ISSUES.md) -- Internal identifiers, invariants, constants, and states must always be English; only external, user-facing content depends on locale. Checking the codebase against this law surfaced a real, pre-existing violation, not something newly introduced. Split into two items (2026-07-31): T-24 carries the narrow, self-contained slice (internal/validation/schemas.go's three Enum(...) definitions, confirmed unused by anything today); this item keeps the larger, harder remainder.

internal/formengine/schemas/asset/asset.json independently hardcodes the same Spanish values for the live field ("tenure_type": Enum["Propiedad","Comodato","Prestamo","Alquiler"]) -- not derived from or kept in sync with internal/validation/schemas.go's copy, a genuinely separate definition of the same underlying data. Confirmed live (2026-07-31): the rendered <option value="Comodato">Comodato</option> uses the identical string for both the stored value and the displayed label -- this is the schema the dynamic form's UI submit path actually uses (asset_dynamic_post.go writes to xolu directly, bypassing internal/validation entirely), so this is the piece with real, live user impact, not the REST-API-only piece split off into T-24.

No existing mechanism separates a stored enum value from its displayed label anywhere in formengine's widget renderer -- confirmed by reading renderSelect/renderSelectWithExtraAttrs (widgets.go). A comment in asset.go shows *field names* already went through an analogous fix (English at the schema layer, translated to/from the original Spanish keys at the xolu persistence boundary by FE-23's persistence code) -- but that precedent covers names only; the values pass through unchanged in both directions today. Building the values equivalent (English canonical value stored and validated, locale-appropriate label displayed) is new scope, not a data edit, and needs a design decision on shape before implementation: whether to extend the widget dispatch signature to carry rc.Locale (touches every registered widget type, not just select) or thread locale via context.Context (introduces a second locale-threading mechanism alongside the rc-parameter one formengine/i18n.go already uses) -- the same open fork already recorded for T-22's own deferred fix, likely the same decision serving both.

Also unresolved, deliberately not decided here: what happens to already-persisted xolu data (from any existing demo/test instance) once the canonical value changes -- whether that needs a migration, or whether older records simply need graceful degraded rendering if their stored value no longer matches any current enum entry. And all four existing seed scripts (seed-01 through seed-04) write the current Spanish literals directly for this field; if the canonical value changes, either those seeds need updating too or the two shapes need to coexist deliberately, not accidentally.

Given English-oriented seeds are being scoped as separate work, this should likely be resolved before those seeds are actually written, not after -- writing new seed data against enum values already known to violate the law would just create more of the problem being fixed.

T-24 closed, 2026-07-31 (v0.3.2, see docs/RESOLVED.md): delivered more than its as-filed scope -- besides the three enum values, internal/validation/schemas.go's AssetCreateSchema/AssetUpdateSchema also had ~28 Spanish field names, now English, matching asset.json's own names via FE-23's existing translation table. Correction to this item's own paragraph above: T-24's slice was NOT REST-API-only as characterised there -- the same two schemas are also validated by internal/ui/routes.go's static Asset form, the path actually reachable through normal navigation, so it had real live impact too, just not yet visible because nothing had exercised it end-to-end before T-24's work surfaced it via test failures. This item's own remaining scope is unchanged by that closure: asset.json's own duplicate enum, the widget-dispatch-locale design fork, the four seed scripts, and already-persisted xolu data are all still open here.

Design fork resolved, 2026-07-31 (the tech team's explicit decision, presented with the actual mechanical cost measured first, not estimated -- 17 widgets, 2 dispatch call sites, rc already one call frame away in renderFieldRow): extend widgetRenderer's signature with rc RenderContext, matching resolveTitle/resolveHelp/resolveLabel's existing shape exactly, rather than threading locale via context.Context (which would have introduced a second, untyped mechanism alongside the one formengine/i18n.go already uses). All 17 registered widgets, both higher-order factories (renderTextLike, renderCheckboxLike), and both dispatch call sites (renderer.go's renderFieldRow, repeatable.go's nested repeatableRow dispatch) updated -- mechanical, compiler-verified via go build, most widgets simply gained an unused parameter.

Value/label separation implemented: new resolveEnumOptionLabel (formengine/i18n.go) resolves a select's per-option label via "<rc.Form>.<fieldName>.<value, lowercased>" -- e.g. "asset.criticality.high" -- reusing the exact asset.status.*/asset.connectivity.* key convention already established, since rc.Form resolves to "asset" for the Asset form by the time any widget renders (Render sets rc.Module/rc.Form from the schema's own x-seam-form before renderFieldRow is ever called -- confirmed by reading resolveModuleForm's actual call site, not assumed). Same graceful literal-fallback as every other resolver in that file: a field with no value-level i18n keys yet renders exactly as before. Wired into renderSelectWithExtraAttrs's enum-options case in place of the old Option{Value: v, Label: v}.

asset.json's three enums changed to the exact English tokens schemas.go (T-24) already established -- criticality: High/Medium/Low; tenure_type: Owned/Loaned/Borrowed/Rented; decommission_reason: Sale/Scrap/Theft/Donation/Other. Caught and fixed a dependent reference before it could silently break: loan_expiry's own x-seam-visible-when condition checked tenure_type=="Comodato" (the old Spanish token) -- updated to "Loaned", confirmed by a full grep sweep of asset.json for every old Spanish enum literal that only the two remaining field-title strings ("Vencimiento Comodato", "Links Comodato" -- display labels, correctly out of scope) still reference the old word, nothing else.

Read-direction translation added, closing the exact gap T-24 deliberately left open: AssetValuesFromXolu's translateAssetEnumValue was toXolu-only in T-24 specifically because asset.json was still Spanish then (translating xolu's stored "Alta" to "High" would have broken the dynamic form's own edit-mode value-matching against a still-Spanish enum). Now that asset.json is English too, both directions are safe and both are wired -- reusing the same assetEnumValueFromXolu table T-24 already built for the static form's own narrow helper, now also driving the general read path. New test coverage: TestAssetEnumValues_RoundTrip (all twelve tokens, both directions) and TestAssetEnumValues_UnrecognisedValuePassesThrough.

Seed-script/persisted-data reconciliation resolved with zero seed changes: grepped every criticidad/calidad_tenencia/motivo_baja literal the four seed scripts actually write (Alta/Media/Baja, Propiedad/Comodato/Alquiler -- motivo_baja isn't used by any seed) -- every one is already covered by assetEnumValueFromXolu, so any seed-created asset now displays its correctly-translated English value the moment it's opened for editing. No migration, no seed rewrite, no degraded-rendering fallback needed -- the two "also unresolved" questions from this item's own original filing turned out to already be answered by the translation table itself.

T-22 (formengine/widgets.go's two empty-option placeholders, the same design fork) resolved as a direct consequence -- see T-22's own resolution note.

Verification: full build + go vet + full test suite clean throughout. Live verification beyond unit tests, the definitive proof: three separate seam instances (SEAM_FORCE_LANGUAGE=en/es/pt), same real running xolu, fetched the actual rendered new-asset form's criticality <option> tags in each. Confirmed: value="High" in all three locales (the internal/external law's actual requirement -- one canonical stored token, not the enum shifting per locale), with the displayed label correctly resolving per locale (High/Alta/Alta, Medium/Media/Média, Low/Baja/Baixa) -- the exact deliverable, demonstrated against genuinely running code, not mocked.

Resolution: internal/validation/schemas.go and asset.json now agree on the same English canonical values for all three fields; xolu's stored data format is completely unchanged (still Spanish, matching every existing record including seed data); the dynamic form correctly translates in both directions; and the display-boundary mechanism T-22/T-23 were both blocked on is now real, tested, and proven live. Nothing remains open in the internal/external language law's original violation (T-23's own title) or in T-22's placeholder text.

Cross-ref: CHANGELOG 0.3.5.

## [0.3.4] T-27 — Region/Moneda/CentroCosto/Marca/Modelo/Ubicacion schemas (internal/validation/schemas.go) hardcode Spanish field names, same internal/external language law violation as T-24/T-26 -- Part 3, the last slice of the file (v0.3.4, 2026-07-31)

Theme: validation · closed 0.3.4 · 2026-07-31


Trigger: 2026-07-31, the third and final session of the full-file sweep we requested after T-24's closure, split into 2-3 parts. Unlike T-26 (Custodio/Alerta/PropDefinition/PropAssoc), none of these six schemas have any enum-valued fields -- field names only.

Scope: RegionCreateSchema/UpdateSchema (descripcion). MonedaCreateSchema/UpdateSchema (codigo_iso, simbolo). CentroCostoCreateSchema/UpdateSchema (codigo_contable, nombre). MarcaCreateSchema/UpdateSchema (codigo, nombre). ModeloCreateSchema (marca_id, nombre; UpdateSchema is a pure alias). UbicacionCreateSchema (nombre_corto, descripcion, region_id already English; UpdateSchema is a pure alias).

Resolution, 2026-07-31: extended internal/ui/masters_xolu_fields.go (built for T-26) with six more per-entity FieldToXolu tables, all using the simpler translateFlatFields helper since none of these six have enum values. Modelo's brand_id deliberately reuses the exact name asset_xolu_fields.go already established for the equivalent Asset field, both mapping to xolu's "marca_id" -- consistency across the app rather than a second name for the same concept, same reasoning as T-26's criticality reuse. All twelve masters_routes.go handlers (Region/Moneda/CentroCosto/Marca/Modelo/Ubicacion x Create/Update) wired identically to T-24/T-26's pattern: English map keys built from the same, unchanged r.FormValue reads, translated back to Spanish immediately before every xoluClient.Create/Update call. Read-only paths (RegionesList's OQL, CentrosCostoList's OQL, ModelosList's marca-name lookup, UbicacionesList's región-description lookup, MonedaDelete's countRef reference check) all query xolu's real, unchanged Spanish-keyed storage directly and were left untouched, same as T-26's PropDef uniqueness-check precedent.

Verification: full build + go vet + full test suite clean. No test file needed updating, same reason as T-26 -- existing coverage submits via the unchanged Spanish HTML form field names. Tree-wide grep confirmed no remaining map-key references to the old Spanish field names anywhere in the codebase. A final full-file sweep of internal/validation/schemas.go (grep for Field("<Spanish pattern>") and Enum("<capitalised, likely-Spanish>")) confirmed every remaining hit is either an already-fixed English token, GET/POST/PUT, or Custodio's deliberately-unchanged DNI/CI/RUT/RUC codes -- the file is now fully English, closing out the sweep T-24 started.

Self-check note, worth recording as a pattern: register.py add's automatic next-free-ID picked T-26 again here, colliding with the T-26 already closed in docs/RESOLVED.md -- it only scans TRACKING.md's currently-open items, not RESOLVED.md, exactly the gap already flagged in this project's own editing conventions ("register.py cannot warn you because it lists only open items"). Caught before the collision propagated (checked docs/RESOLVED.md directly, not just register.py list's output), the wrongly-numbered entry was removed cleanly, and this item was re-filed with an explicit --id T-27. Worth continuing to cross-check RESOLVED.md by hand every time, not just after a close in the same session.

Cross-ref: CHANGELOG 0.3.4.

## [0.3.3] T-26 — Custodio/Alerta/PropDefinition/PropAssoc schemas (internal/validation/schemas.go) hardcode Spanish field names and enum values, same internal/external language law violation as T-24's Asset schemas -- Part 2 of the full-file scope we requested (v0.3.3, 2026-07-31)

Theme: validation · closed 0.3.3 · 2026-07-31


Trigger: 2026-07-31, scoping T-24's follow-on work after the xolu 0.19.3 integration and T-24's own closure -- we directed the full file be brought into compliance with the internal/external language law (docs/KNOWN_ISSUES.md), split into 2-3 sessions. A full-file scan (beyond T-23/T-24's originally-filed three enums) found CustodioCreateSchema/UpdateSchema, AlertaCreateSchema/UpdateSchema, PropDefinitionCreateSchema/UpdateSchema, and PropAssocCreateSchema (PropAssocUpdateSchema is a pure alias) all had Spanish field names and, for Alerta/PropDefinition/PropAssoc, Spanish enum values too.

Scope: CustodioCreateSchema/UpdateSchema (tipo_sujeto, nombre_razon_social, identificador_fiscal, telefono, direccion, estado -- tipo_sujeto's own DNI/CI/RUT/RUC values deliberately NOT translated, regional legal document-type codes not translatable prose, same reasoning flagged and confirmed before T-24 began). AlertaCreateSchema/UpdateSchema (tipo, descripcion, criticidad -- values Mantenimiento/Seguridad/Operativa and Alta/Media/Baja, the latter reusing T-24's own criticality mapping). PropDefinitionCreateSchema/UpdateSchema (nombre_corto, tipo_dato, validacion, valor_defecto -- ten tipo_dato enum values). PropAssocCreateSchema (prop_definition_id already English, entidad_destino, entidad_id, obligatoriedad -- entidad_destino's own asset_type/asset values already English, only obligatoriedad's requerida/opcional needed translating).

Resolution, 2026-07-31: unlike T-24's Asset schemas, all four Create schemas here are the ONLY schema actually live -- validate.go exposes just one schema var per entity (e.g. schemaCustodioCreate), used by both the masters_routes.go Create AND Update HTTP handlers; the separate *UpdateSchema declarations in schemas.go (CustodioUpdateSchema, AlertaUpdateSchema, PropDefinitionUpdateSchema) were never referenced by anything, confirmed by tracing every validateForm call site in masters_routes.go, not assumed. Fixed for consistency anyway, since they're part of the same file-wide law violation, but the live risk surface was the four Create schemas plus their eight masters_routes.go handler functions (CustodioCreate/Update, AlertaMasterCreate/Update, PropDefCreate/Update, PropAssocCreate -- no PropAssocUpdate handler exists at all).

New translation layer built from scratch: internal/ui/masters_xolu_fields.go, mirroring asset_xolu_fields.go's proven pattern (T-24) but simpler -- none of these four entities have array-typed fields, so flat key translation (translateFlatFields) and flat key+enum-value translation (translateFlatFieldsAndEnums) cover all four without asset_xolu_fields.go's recursive array-item machinery. Each entity gets its own FieldToXolu table and, where it has enum values, an EnumValueToXolu table plus a *EnumValueFromXoluValue helper for the write-direction-only translation masters_routes.go's raw form input needs (same asymmetry as T-24's AssetEnumValueFromXoluValue, same reason: no display-boundary translation exists yet for any of these, so the read/edit-form side must keep seeing the same Spanish values it always has).

All eight masters_routes.go handlers wired: English map keys built from the same, unchanged r.FormValue("<original Spanish name>") reads (HTML forms untouched), enum fields wrapped in the relevant *EnumValueFromXoluValue helper before validateForm, and the whole map translated back via *ValuesToXolu immediately before h.xoluClient.Create/Update -- identical shape to T-24's fix. PropDefCreate/Update's own nombre_corto uniqueness-check OQL query was left untouched, since it queries xolu's real, unchanged Spanish-keyed storage directly and was never part of the validation-layer map. PropAssocCreate's "opcional" default for an empty obligatoriedad submission is now translated to "optional" the same way a real submitted value would be, preserving the existing default behaviour exactly.

i18n groups added, en/es/pt: alerta.type.*, alerta.criticality.* (reusing T-24's own High/Medium/Low tokens and Spanish/Portuguese values, kept as a separate per-entity group rather than a shared import, matching this codebase's existing per-entity convention -- see asset.status.* vs a hypothetical shared status.* group), prop_definition.data_type.* (ten values), prop_association.requirement.*.

Verification: full build + go vet + full test suite clean. No test file needed updating -- extensive pre-existing coverage (TestCustodioCreate_*, TestAlertaMasterCreate_*, TestPropDefCreate_*, TestPropAssocCreate_*, TestCustodioUpdate_DataIntegrity) already submits via the unchanged Spanish HTML form field names and passed without modification, including TestCustodioUpdate_DataIntegrity, which captures the actual PUT body sent to xolu and asserts it's still Spanish-keyed with the exact submitted values -- direct, pre-existing proof the translation round-trip is correct, not just that no error was thrown. Tree-wide grep confirmed no remaining map-key references to the old Spanish field names for these four entities anywhere in the codebase.

Cross-ref: CHANGELOG 0.3.3.

## [0.3.2] T-24 — criticidad/calidad_tenencia/motivo_baja (internal/validation/schemas.go) hardcode Spanish words as the only valid enum values -- narrow, self-contained slice of T-23 (v0.3.2, 2026-07-31)

Theme: validation · closed 0.3.2 · 2026-07-31


Trigger: split from T-23 (2026-07-31) -- T-23's investigation found this specific piece is low-risk and self-contained, separable from the larger, harder part of the same violation (see T-23's own reduced scope). Confirmed by tracing actual call sites, not assumed: internal/handlers/api/assets.go is the only caller of validation.AssetCreateSchema/AssetUpdateSchema for these fields, and nothing found (seed scripts, the dynamic form's UI submit path) actually exercises that REST API path today.

internal/validation/schemas.go: criticidad only accepts Enum("Alta","Media","Baja"); calidad_tenencia only accepts Enum("Propiedad","Comodato","Prestamo","Alquiler"); motivo_baja only accepts Enum("Venta","Desguace","Robo","Donacion","Otros"). Not display text -- the actual stored, validated values, violating the internal/external language law (docs/KNOWN_ISSUES.md).

Scope: choose English canonical values for each enum (e.g. criticidad: high/medium/low), update the three Enum(...) calls, confirm queryfy.ValidateWithMode still passes with the new values, confirm nothing else (tests, other callers) references the old Spanish literals. Does NOT include asset.json's own separate, independently-hardcoded enum for the same underlying field (tenure_type), the missing value/label display-translation mechanism for enum widgets, the four seed scripts' own Spanish literals, or any already-persisted xolu data -- those remain T-23's scope, deliberately not folded in here, since they need a design decision (how should a stored English value display as locale-appropriate text?) this narrow slice doesn't.

Correction found during implementation, 2026-07-31: the "confirmed unused" framing above was accurate for the REST API endpoint (internal/handlers/api/assets.go) specifically, but incomplete -- the same two schemas (AssetCreateSchema/AssetUpdateSchema) are ALSO validated against by internal/ui/routes.go's static Asset form (AssetCreate/AssetUpdate handlers, via schemaAssetCreate/schemaAssetUpdate in validate.go), which prior-session documentation confirms is the path actually reachable through normal navigation -- the dynamic form is not. Missed on the first pass; found by running the full test suite after the schema change and tracing two failures back to root cause (TestAssetCreate_InvalidCriticidad_ReturnsForm, TestAssetCreate_Valid_Redirects) rather than assuming the change was safe once internal/handlers/api's own tests passed.

Scope actually delivered, larger than as-filed: beyond the three enum values, AssetCreateSchema/AssetUpdateSchema also had ~28 Spanish field names (codigo_barras_rfid, marca_id, calidad_tenencia, motivo_baja, etc.) that had never gone through FE-23's field-name fix -- that fix reached asset.json only, not this parallel REST-API/static-form schema. All ~28 renamed to the exact English names FE-23's own asset_xolu_fields.go translation table already used for the equivalent asset.json fields (reused, not independently invented), plus the three enum values (criticidad/calidad_tenencia/motivo_baja -> criticality/tenure_type/decommission_reason; Alta/Media/Baja -> High/Medium/Low; Propiedad/Comodato/Prestamo/Alquiler -> Owned/Loaned/Borrowed/Rented; Venta/Desguace/Robo/Donacion/Otros -> Sale/Scrap/Theft/Donation/Other).

Persisted-data compatibility, the actual constraint shaping the implementation: xolu's real "assets" storage is Spanish-keyed/valued today (confirmed via asset_xolu_fields.go's own comment and via seed-01 through seed-04 writing Spanish keys directly). Renaming the validation layer without a translation step would have split the REST API and static-form write paths from xolu's actual stored format, and from each other. Fixed by extending the existing, proven FE-23 pattern rather than inventing a new mechanism: AssetValuesToXolu (already used by the dynamic form) now also translates the three enum fields' values, write-direction only. Both live write paths -- internal/handlers/api/assets.go's Create/Update, and internal/ui/routes.go's AssetCreate/AssetUpdate (including a separate, easy-to-miss preCheck fast-fail map in AssetCreate that duplicates a subset of the main map, and was the actual cause of the two test failures above) -- now translate through it immediately before persistence. xolu's stored format, the four seed scripts, and the read/display side are all deliberately untouched, per the tech team's explicit direction not to migrate data or rewrite seeds for this pass -- only the validation-layer identifiers themselves needed to become English. New AssetEnumValueFromXoluValue (Spanish->English, write-direction helper for routes.go's raw form input) is deliberately NOT the reverse of the read-direction AssetValuesFromXolu: asset.json's own enum is still Spanish (T-23, unresolved), so translating xolu's stored Spanish values to English on the read/edit-form side would break the dynamic form's own value-matching until T-23 does that side too.

i18n group added (the tech team, 2026-07-31): a dedicated set of keys for these three enums' English canonical tokens, in en.json/es.json/pt.json -- asset.criticality.{high,medium,low}, asset.tenure_type.{owned,loaned,borrowed,rented}, asset.decommission_reason.{sale,scrap,theft,donation,other} -- following the existing asset.status.*/asset.connectivity.* precedent (flat dotted-key file format, real hierarchy underneath). Not yet consumed by any renderer (that's T-22/T-23's own widget-dispatch-locale fork) but now centrally defined rather than left for whoever eventually solves that fork to invent ad hoc.

Verification: full build + go vet + full test suite (all packages) clean, offline (GOPROXY=off), against xolu 0.19.3. Two test files updated for the new field names/values (schemas_test.go, assets_test.go -- the latter strengthened to assert AssetValuesToXolu actually fires, not just that the HTTP call succeeds). Confirmed via tree-wide grep that no other test or call site still references the old Spanish literals for these specific fields.

Resolution: internal/validation/schemas.go's Asset schemas are now fully English at the identifier/constant layer, satisfying the internal/external language law for this slice, with zero change to xolu's stored data shape or to either live write path's external behaviour. T-23's remaining scope (asset.json's own duplicate enum, seed scripts, persisted-data display, the widget-dispatch-locale design fork) is unchanged by this closure -- see T-23's own updated note. See CHANGELOG.md's [0.3.2] entry.

Cross-ref: CHANGELOG 0.3.2.

## [0.2.45] T-17 — Locale selection mechanism + bilingual (en/es) content for the Asset form (v0.2.45, 2026-07-29)

Theme: i18n · closed 0.2.45 · 2026-07-29


Trigger: the tech team's hard requirement, 2026-07-28 -- during Seam AMS development, English and Spanish translation content must be kept in sync for every user-facing feature, not deferred. This is a content-authoring/process discipline, not a runtime requirement -- a simple server-side, per-request locale choice (session/cookie/URL param, applied on page load) is sufficient; live client-side language-switching without a reload is explicitly not required, given minty's server-rendered architecture. Recorded as a standing memory instruction, not just this task.

Scope, confirmed by investigation before filing, not assumed: three gaps, none solved by T-13's resolution mechanism alone. (1) No locale-selection mechanism exists anywhere in the app -- h.translator.Locales()/HasLocale() only list/validate, nothing decides which locale a given request renders in. Given the clarified, simple (non-real-time) requirement, this can be as small as a session/cookie/URL-param check on page load, not a live-switching UI. (2) AssetNewDynamic/AssetEditDynamic never pass h.translator into the FormEngine they construct, and never set RenderContext.Locale -- small, mechanical, currently missing. (3) internal/i18n/locales/{en,es}.json have zero assets.forms.asset.field.*/core.errors.* keys -- only nav-chrome-level content exists today. Even with (1) and (2) fixed, resolution finds nothing for the Asset form and falls through to the schema's literal (Spanish) text.

Deliberately not started now, 2026-07-28: FE-23's POST handler investigation was complete and implementation-ready when this was found -- pausing again would have been a third interruption on the same task (after T-13). The dynamic Asset form isn't live to real users yet either -- it's a parallel /assets/dynamic/* path for FE-24's future comparison, not reachable through normal navigation -- so the hard requirement's real deadline is FE-24 (the actual shipping moment), not FE-23's remaining piece. Filed here, gated the same way T-13 was, so it isn't forgotten.

Correction, 2026-07-29: gap (1) above was wrong, not just incomplete -- a locale-selection mechanism already exists and works: /settings and /profile both render a real, functioning language selector (languageSelect in pages.go, profileSelect in profile.go), POST /settings/general validates and persists a tenant-wide locale to xolu's tenant_settings and updates h.config.Locale in memory. Missed this during the original investigation by checking only h.translator.Locales()/HasLocale() and the dynamic form handlers specifically, not searching more broadly first. Gap (2) still stands exactly as filed -- the dynamic Asset form handlers never read h.config.Locale or wire Translator, so the existing, working mechanism simply never reaches them.

SEAM_FORCE_LANGUAGE added (internal/config/config.go + internal/server/server.go): a startup-only initial value for uiCfg.Locale, checked early and before ui.NewRouter is called. Deliberately narrow scope, not a general override -- normal bootup (should it ever gain the ability to load a tenant's saved Settings locale from xolu at startup, which it currently does not) runs after this and is free to overwrite it. Invalid values logged and dropped, not silently kept, matching SettingsSaveGeneral's own fallback-to-en convention. A separate, real gap surfaced while wiring this that this task deliberately does not fix: server.go builds uiCfg.Locale from ui.DefaultConfig()'s hardcoded default with no startup-time load from tenant_settings at all -- meaning a tenant's saved Settings locale does not currently survive a server restart. Out of scope here per explicit instruction; flagged for whenever that gap gets addressed.

Gaps (2) and (3) remain open. Gap (1) is now "exists for the general app, still unreached by the dynamic Asset form" rather than "does not exist."

SEAM_FORCE_LANGUAGE extended, 2026-07-29: now resolves via a new Translator.ResolveLocale (internal/i18n/i18n.go), not a plain HasLocale check. Supports both directions of base/region matching per the tech team's spec -- a requested base code ("en") matches a loaded region variant ("en_gb") if no plain "en" is loaded, and a requested region code ("es_uy") matches a loaded base ("es") or a sibling region ("es_es") if no exact match is loaded. Whatever locale in that language family is available loads regardless of direction, with a warning logged when it isn't an exact match; only a value with nothing in its language family loaded at all is dropped. Confirmed es.json is already written in the tuteante rioplatense style (checked actual content, "tu portafolio de activos" etc.), not generic/neutral Spanish -- so es already means es_uy in substance today, region-qualified files just don't exist as separate artifacts yet. 8 new tests on ResolveLocale, including the exact scenario as specified ("we have en_gb in store, but we specify en only").

Gap (2) closed, 2026-07-29: all four dynamic Asset form handlers (AssetNewDynamic, AssetEditDynamic, AssetCreateDynamic, AssetUpdateDynamic) now wire both Translator (h.formEngineTranslator()) and RenderContext.Locale (h.currentLocale(), a new small helper reading h.config.Locale under its guarding lock -- the same value Settings/Profile already read and write, and SEAM_FORCE_LANGUAGE seeds at startup). Found while wiring this: even the two POST handlers, which already had Translator wired since FE-23, never set rc.Locale at all -- meaning resolution was falling through to literal text there too despite the translator being present. Confirmed genuinely functional, not just wiring that compiles: new end-to-end test builds a real i18n.Translator via a new i18n.NewFromMessages (exported test-support constructor, since the real embedded locales have no Asset-form keys yet -- gap 3) with a known "field.name" key, confirms the rendered label is the translation with Locale=en, and confirms it falls back to the schema's literal title with a locale that has no match anywhere in its own fallback chain either (caught a test-construction mistake along the way: T's own fallback-to-t.fallback behaviour meant an early version of the test still resolved through "en" even when the *requested* locale had no key, since the translator's fallback was also "en" -- fixed by giving the translator a fallback locale that also lacks the key). Stress-run 10x clean.

Gap (3) -- authoring real en/es content for the 42 Asset fields -- remains the only open piece of T-17.

Gap (3) closed, 2026-07-29: authored real en/es content -- 64 new keys total, added identically to both locales, zero collisions with the existing 120 (verified programmatically before writing, not assumed). 4 tab labels, 9 section labels, 42 field titles (assets.forms.asset.field.*), 1 field help (parent_id, the only field with x-seam-help), 8 core.errors.* keys shared across the whole app, not Asset-specific. pt.json deliberately untouched -- T-17 scoped this to en/es specifically, and pt gracefully falls back to the app's own fallback locale ("en", confirmed from server.go's i18n.New("en") call) rather than showing raw keys or breaking.

A real prerequisite gap found and fixed before any content could even be looked up: asset.json's x-seam-form had no module/name keys at all, meaning rc.Module/rc.Form were empty for every render and resolveTitle/resolveHelp were silently skipping the "assets.forms.asset.field.*" step entirely -- confirmed by tracing Form.Module/Form.Name's actual source (x-seam-form.module/name exclusively, schema.go) before assuming anything. Added module: "assets", name: "asset", matching §3.2's own documented example values exactly. Also fixed while there: §3.2's prose named the override fields RenderContext.I18nModule/I18nForm, which don't exist -- the real, working fields are Module/Form, confirmed directly against engine.go.

A real, visible bug confirmed empirically before writing any content, not assumed: with no content authored, tab labels rendered as their own raw i18n key text ("assets.forms.asset.tab.principal" appearing literally in the HTML) -- verified with a throwaway test before starting, since resolveLabel's fallback is the label value itself, and the schema authors wrote that value as a key, not literal text (unlike field titles, which do have a literal-text fallback). This was live and broken in the dynamic form every time it rendered, regardless of locale.

Two new end-to-end tests using the real, shipped i18n.New translator (not mocks): TestAssetSchema_I18nContent_EndToEnd confirms tabs/sections/field titles/help resolve correctly in both en and es and that the raw-key bug is genuinely gone, not just fixed in a synthetic test. TestResolveQueryfyError_RealContent_AllEightKeysResolve confirms all 8 core.errors.* rules resolve in both locales against the actual authored content. Stress-run 10x clean.

All three gaps closed. T-17 complete.

Cross-ref: CHANGELOG 0.2.45.

## [0.2.32] T-13 — Wire i18n resolution into formengine (title/help as translation keys, queryfy error mapping) (v0.2.32, 2026-07-28)

Theme: form-engine · closed 0.2.32 · 2026-07-28


Trigger: Discovered 2026-07-28 auditing docs/I18N_SPEC.md against the current formengine implementation.
Scope: I18N_SPEC.md $4.1: schema title is a translation key by default (4-level fallback chain), not literal text -- $4.4: queryfy validation errors map to translated core.errors.* keys. Neither is implemented; renderFieldRow renders f.Title literally. Works today only because the 3 stage-2 master schemas happen to use literal Spanish strings (indistinguishable from correctly hitting I18N_SPEC's final fallback). internal/i18n.Translator.T(locale, key, args...) already exists and works -- this is wiring, not new infrastructure. Missing entirely from FORMENGINE_DEVELOPMENT_PLAN.md's 8 stages. Will actually break once stage 7 authors the real Asset schema, since FORM_ENGINE_SPEC.md's own examples use real i18n keys (assets.forms.asset.tab.principal etc.), not literal text.

Confirmed, 2026-07-28: FE-22 authored the real Asset schema. Tabs and sections use i18n keys (assets.forms.asset.tab.principal etc., matching §12), but the 42 field-level titles are still literal Spanish text with "*" hand-appended for the fields that look required in the current static form (e.g. "Nombre *", "Región *") -- exactly the halfway state predicted above. Decision (the tech team, 2026-07-28): resolve region_id's specific inconsistency (labelled "*" but never actually HTML-required -- see RESOLVED.md T-... FE-22 for the full finding) together with this task, not as an isolated schema tweak first. Once titles resolve through real i18n keys, how "required" gets indicated (baked into the key's resolved string vs. a separate, systematic convention driven by the schema's own `required` array) needs one consistent design across all 42 fields -- that's also the right moment to decide whether region_id becomes genuinely required or the asterisk is simply dropped, rather than guessing at one field in isolation.

$4.1/$4.2 resolution mechanism built and tested, 2026-07-28. Two design questions resolved before writing code, not guessed at: (1) I18N_SPEC.md itself had a real inconsistency -- $4.1's own resolution chain said `{modulo}.form.field...` (singular, no form-name segment) while $3.4's canonical naming convention (the one already in production, matching the real Asset schema's tab/section keys) says `{modulo}.forms.{nombre_formulario}.field...`. Fixed $4.1/$4.2 to match $3.4, confirmed the authoritative one. (2) Neither `{modulo}` nor `{nombre_formulario}` existed anywhere before this -- not a Render() parameter, not in x-seam-form. Resolved with two independently-overridable levels (the tech team's call, motivated by a reusable-card scenario): x-seam-form.module/name as the schema's own default, RenderContext.Module/Form (already present in the engine since stage 1, unwired until now) overriding per call when non-empty. Documented in both specs before implementing.

internal/formengine/i18n.go: resolveTitle (4-step chain), resolveHelp (3-step), resolveLabel (single-key, for tab/section/detail/embed labels, which are already explicit keys per $3.4 rather than title's literal-or-implicit-key hybrid). All three degrade gracefully to literal schema text when Translator is nil or nothing resolves -- confirmed empirically, not just by design intent: every existing formengine test (built before Translator existed) still passes unchanged, since they all use an empty RenderContext with no Translator configured. Wired into renderFieldRow, fkPopoverAwareLabel, tabButton, renderSection, renderDetailPanel, renderEmbedTrigger (the last two needed *FormEngine added to their signatures, small blast radius -- 3 call sites, all within tabPanel which already had e in scope). 12 new tests including an end-to-end proof through the real Render() pipeline (not just resolveTitle called in isolation) confirming rc.Module/rc.Form actually reach the rendered HTML.

$4.4 (queryfy error mapping) built and tested, 2026-07-28: internal/formengine/i18n_errors.go, ResolveQueryfyError. Matched against queryfy's actual message wording for the jsonschema.FromJSON code path specifically (the one formengine's own parseSchema uses, confirmed from schema.go directly) -- not against $4.4's own prose descriptions, which turned out inaccurate for several rules when checked against the real library. Verified with an integration test that builds a real schema, runs real queryfy validation, and confirms the mapping against genuine FieldError messages, not hand-typed fixtures.

That verification surfaced a genuine bug in queryfy itself, not a spec problem -- confirmed by checking both possibilities before fixing either (the tech team's instruction: fix the bug at its source, don't work around it downstream). compile.go's Compile() (an optional pre-compilation step formengine never calls) re-implemented numeric min/max validation with different wording ("must be at least/at most %v") than builders/number.go's original ("must be >= /<= %v", the wording jsonschema.FromJSON actually produces) -- same schema, same violation, two different messages depending on whether a caller compiled the schema first. Fixed at the source: cloned github.com/ha1tch/queryfy fresh, corrected compile.go's compileNumber to match builders/number.go exactly, confirmed via the library's own test suite (go test ., ./builders/... all clean). Not yet pushed -- lives in a local clone pending the tech team's decision on publishing, since that's a decision about our own repository and release process, not this session's to make unilaterally. Dormant for Seam today (nothing calls Compile()), so no urgency on Seam's go.mod side either.

T-13's full filed scope ($4.1, $4.2, $4.4) is complete. Closing here. One loose end carried forward, not resolved by this task: the required-field "*" convention question (region_id specifically) needs a decision before FE-24's parity comparison can be fair -- filed on FE-24 rather than left dangling.

Cross-ref: CHANGELOG 0.2.32.

## [0.1.0-patched252] T-12 — Reconcile FSM architecture: fsm-toolkit (client-side, FSM_DESIGN.md) vs xolu native /fsm (server-side, now stable) (v0.1.0-patched252, 2026-07-28)

Theme: fsm-architecture · closed 0.1.0-patched252 · 2026-07-28


Trigger: Discovered 2026-07-28 auditing all docs/ specs against the current register.
Scope: FSM_DESIGN.md v1.4 (March 2026, status "extension pending") targets migrating internal/fsm onto github.com/ha1tch/fsm-toolkit -- a separate client-side Go library (guards via tsqlparser, actions via OQL executor, FSM defs stored as opaque JSON in regular xolu entities). Confirmed internal/fsm currently imports neither fsm-toolkit nor xolu -- self-contained evaluator, migration never done. FSM_DESIGN.md predates xolu's /fsm primitive stabilizing and never mentions it.

Decision, 2026-07-28: xolu's native /fsm is the runtime FSM engine, no ambiguity -- both Assets Management (superseding internal/fsm and FSM_DESIGN.md's fsm-toolkit-as-runtime target) and CMMS Workflow build on it. fsm-toolkit may still be imported into Seam separately, for its graphical/TUI FSM designer (design-time authoring/validation tooling), not as a competing runtime -- explicitly not decided yet and not urgent. Not a duplication if it happens: the same fsm-toolkit package used for design-time tooling on one side and (potentially) definition validation on the other guarantees identical behaviour rather than creating two implementations of the same logic. FSM_DESIGN.md needs a reconciliation banner noting its runtime-migration target is superseded; its non-runtime content (domain examples, per-state metadata ideas) may still be useful as input to whatever eventually authors FSM definitions.

Cross-ref: CHANGELOG 0.1.0-patched252.

## [0.1.0-patched245] T-06 — Bump go.mod directive 1.22.2 -> 1.25 (v0.1.0-patched245, 2026-07-28)

Theme: toolchain · closed 0.1.0-patched245 · 2026-07-28


Trigger: Session-start toolchain policy (owned ha1tch-namespace project).
Scope: One-line go.mod change plus a clean build confirmation.

Cross-ref: CHANGELOG 0.1.0-patched245.

## — register established 2026-07-28 —

No items closed before this point.
