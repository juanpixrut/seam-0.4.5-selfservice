# Seam AMS — Known Issues and Recorded Decisions

Version: 0.4.5
Last reviewed: 2026-11-18

Intentional limits, invariant boundaries, and decisions that aren't
open work items but should be visible.

## Non-negotiable invariants

Not open decisions. Settled, permanent, and binding on all future work
regardless of who or what is doing it.

### Internal vs. external: language boundary

Trigger: the tech team, 2026-07-31 — stated as law, not a preference, while
scoping English-locale demo seeds.

**Internal** — identifiers, invariants, constants, states. Not visible
to an end user; not rendered in any UI a user looks at. **Always
English, with no exception, regardless of the codebase's own history or
which developer wrote it.** This includes, and this is the part that's
easy to get wrong: enum *values* used as validation constants or stored
state, not just field *names*. A field being named in English
(`tenure_type`) does not satisfy this law if its own valid values are
hardcoded in a single language baked into the validation layer
(`"Comodato"`, `"Alta"`) rather than being English canonical tokens
translated at the display boundary.

**External** — anything visible, anything user-facing. Depends on the
active locale. This is what `internal/i18n` and `internal/condition`'s
own `Value` fields (once display-translated) exist to serve — the
locale a specific request or user has chosen, not a property of the
codebase itself.

**A concrete, already-identified violation, not a hypothetical**:
`internal/validation/schemas.go`'s `criticidad`
(`Enum("Alta","Media","Baja")`), `calidad_tenencia`
(`Enum("Propiedad","Comodato","Préstamo","Alquiler")`), and
`motivo_baja` (`Enum("Venta","Desguace","Robo","Donación","Otros")`)
all hardcode Spanish words as the *only* valid stored values for these
fields — not display text, the actual enum constants the validation
layer accepts or rejects. Under this law, that's non-compliant: an
English-oriented (or Portuguese-oriented) tenant's data would be forced
to store `"Comodato"` regardless of locale, because nothing else
validates. Filed as T-23, not yet fixed, later split (2026-07-31) into
T-24 (the narrow, self-contained `internal/validation/schemas.go` slice,
confirmed unused by anything today) and a reduced T-23 (the harder
remainder: `asset.json`'s own separate, independently-hardcoded copy of
the same values, which is what the live UI actually uses; no existing
mechanism to display a stored value under a different, locale-appropriate
label; and reconciling every existing seed's already-written Spanish
enum literals) — fixing either is real, separate scope from just stating
the law. Flagging the violation and fixing it are different acts; this
document does the former.

Applies to anything with the same shape found elsewhere in this
codebase or in future code, not just these three fields — the law is
general, this is the first worked example of applying it, not the
extent of it.

**A second worked example, found and fixed 2026-08-01**: 113 of
`internal/i18n`'s own translation *keys* — not values — were
Spanish-language segments (`masters.custodio.*`, `masters.marca.*`,
`assets.forms.asset.section.basico`, and 110 others), inherited from
pre-T-18 session work and extended without question across T-18's own
eight stages. A different violation shape than T-23/T-24's (this is
about the *identifier*, not a stored value the validation layer
accepts), but the same law: an identifier isn't end-user-visible by
definition, so its language is arbitrary from a user's perspective —
which is exactly why defaulting to the language the schema author
happened to be thinking in, rather than English, is the mistake.
Renamed via `scripts/rename_spanish_keys.py` (kept in the repo,
not a throwaway script) across all four locale JSON files (keys
renamed, every value left untouched — confirmed keys are pure
identifiers, not display text, before doing this), every `.go` call
site, and `asset.json`'s own tab/section `label` fields (confirmed via
reading `formengine/i18n.go`'s `resolveLabel` that these are literal
key strings written directly into the schema, not Go-constructed —
missing this would have silently broken section titles with no
compile error to catch it). `masters.propdef.*`/`masters.propassoc.*`
kept as-is — already English abbreviations, only their Spanish
field-level segments needed renaming. `region` (singular, identical in
English and Spanish) deliberately left alone; `regiones` (the plural,
which does differ from English) renamed.

**Explicitly not touched by this pass, and why**:
`internal/formengine/schemas/masters/{region,moneda,marca}.json`
embed raw literal Spanish text directly as schema field titles rather
than referencing any i18n key at all — confirmed via
`masters_pages_dynamic.go`'s own doc comment that this path isn't
wired into any live route (dev/test parity-proof only, exercised
solely by `TestMastersFormParity`). A real instance of the same
underlying carelessness, but a different mechanism (no key exists to
rename) and genuinely out of user-facing scope today since nothing
here is ever rendered to an actual tenant. Filed as a separate,
open item rather than silently fixed alongside the key rename or
silently left unmentioned.

### Internal documentation language

Trigger: the tech team, 2026-08-02 — stated while reviewing the first draft
of `DASHBOARD_SPEC.md`, connecting it directly to the internal/external
language law above: Spanish-language internal documentation is judged
to have been a real contributing cause of the identifier-language
problems this project has spent significant effort finding and fixing
(T-18, T-23/T-24, T-26, T-38, T-39, T-40 and its still-open remainder).
Every existing spec (`FORM_ENGINE_SPEC.md`, `ADMIN_MODULE_SPEC.md`,
`DOCUMENTS_MODULE_SPEC.md`, `I18N_SPEC.md`) was written in Spanish
prose while defining schemas whose actual field names were meant to be
English (`x-seam-form`, `criticality`, `subject_type`) — the prose
authors were thinking in Spanish while writing specs for an
English-identifier system, and Spanish leaked into the identifiers
themselves repeatedly as a result. Internal documentation is not
end-user-facing, so by the same logic as the identifier law above, its
language should never have been anything other than an arbitrary
authoring choice — and an authoring choice that happened to default to
the same language the identifiers needed to *not* be in was actively
counterproductive, not neutral.

**All new internal documentation — specs, plans, tracking documents,
architecture notes, register entries — is written in English, with no
exception, starting 2026-08-02.** This is now the same category of law
as the identifier language rule: not a style preference, binding
regardless of who or what is authoring.

**Existing Spanish-language documents are not retroactively translated
by this decision alone.** `FORM_ENGINE_SPEC.md`, `ADMIN_MODULE_SPEC.md`,
`DOCUMENTS_MODULE_SPEC.md`, `I18N_SPEC.md`, and `RBAC-COMPLETION-PLAN.md`
remain in Spanish (the last already partly in English) until translated
as their own explicit, scoped piece of work — filed as T-43, not
scoped or estimated, deliberately not done reflexively alongside this
decision the way over-broad reflexive fixes have caused problems
elsewhere in this project's history. `DASHBOARD_SPEC.md` itself,
being newly drafted and not yet referenced by any shipped code, was
translated to English immediately as part of this decision rather than
carried forward as a second Spanish document alongside a rule saying
not to write new ones.

## Open decisions

### Rename from the old name to Seam

Both passes complete 2026-07-28 — see T-05 for full detail, including a
real CSS/JS mismatch and a version-template bug caught along the way.
Four items still open, all needing a decision rather than a text
substitution: the Go module path suffix choice made unilaterally during
Pass 1 (not corrected when checked in — treating as accepted, flagging
once more); the `qr_base_url` domain (currently the old domain, no
confirmed Seam replacement); the QR prefix `"SHF"` (`qrcode.go`
`DefaultConfig`) — derived from the old name but not caught by any text
search since it doesn't spell that name out literally, and has
physical-label implications if codes are already printed; and
`web/static/favicon.svg`, whose comments describe an actual visual pun on
the old name (three lines with boxes, drawn as shelves) — needs real
icon redesign, not substitution, and was deliberately left untouched.

### Asset Management module boundary

No spec exists for the AMS module (see T-03). Until one is written and
agreed, do not assume any specific package or handler belongs to
`assets` versus `masters`/`monitoring`/`reports` — the names appear in
`ADMIN_MODULE_SPEC.md`'s dependency diagram but aren't yet defined
anywhere.

### xolu v0.16.16 — primitives now available that weren't when Seam's olu client was last touched

`internal/olu` was last brought to parity with xolu ~v0.9.7. As of
2026-07-28, xolu is at v0.16.16 with an official client
(`github.com/ha1tch/xolu/pkg/client`) and `/ts`, `/fsm`, `/cal`, `/bal`
all stable (`/fsm` was early-stage and `/cal`/`/bal` didn't exist at
v0.9.7). `/dxp` (distributed cross-substrate transactions) is still in
progress upstream. See T-07 before designing the core data-access layer
for the module split — `/cal` in particular is a plausible fit for the
future CMMS-like module's maintenance scheduling.

### Verified non-blockers (checked 2026-07-28, don't re-derive)

- **x-seam-* schema round-trip**: xolu's `POST /api/v1/schema/{entity}`
  decodes into `map[string]interface{}` (not a fixed struct) and
  `validateSchemaFieldNames` only inspects `properties` keys — extension
  properties anywhere else in the schema (`x-seam-form` at the root,
  `x-seam-input` etc. on individual fields) pass through untouched, are
  persisted as posted, and `GetEntitySchema` returns them back as raw,
  verbatim JSON. FORM_ENGINE_SPEC.md's whole premise (schema-with-
  extensions stored in xolu) holds. Confirmed twice now: first by reading
  xolu's handler source, then in code — `TestSchemaRoundTrip_XSeamExtensionsSurvive`
  (internal/formengine/engine_integration_test.go, dormant guard, table
  below) ran against a real local xolu v0.16.16 instance 2026-07-28 and
  passed, including a nested `x-seam-form.tabs[0].id`. The one real gap:
  `pkg/client` has no write wrapper for schema registration yet (only
  `GetEntitySchema` reads) — Seam needs its own call to the raw
  endpoint, which works fine over plain HTTP.
- **RBAC infrastructure underlying Administration (T-02)**: checked
  `docs/RBAC-COMPLETION-PLAN.md` — all phases complete, all
  definition-of-done items checked off (login, session, CSRF, lockout,
  audit logging, integration tests). Not a gap T-02 needs to absorb.

## Recorded decisions

Settled; written down so they are not re-derived.

### Repeatable bounds are enforced twice, and only the server's counts

Trigger: the tech team, 2026-10-12 -- minimum and maximum rows for a repeatable.

`minItems` and `maxItems` (the standard JSON Schema keywords, which queryfy
already honours) configure the bounds. The form pads to the minimum and
disables Remove at it and Add at the maximum, in the markup and again in
`SeamRepeatable`, but that only keeps the buttons honest: a request is whatever
the client posts. A handler must call `ValidatePostedBounds` (or
`RepeatableBoundsMessage` when it counts rows itself); the engine cannot do it
for a handler that never asks. Rows whose first field is empty are dropped on
parse and do not count. Rows that already exceed the maximum are shown, never
hidden.

### Option icons need the combobox

Trigger: the tech team, 2026-10-12 -- icons in select options.

A native `<select>` cannot draw an icon, so `x-seam-option-icons` is drawn by
`SeamSelect`. With `features.searchable_select` off, or for a select that keeps
its native control (`asset-type-select`), the options keep their text and no
icon. This is intentional, not a bug to chase. Icon names are validated twice
(`ParseSchema` refuses a bad one; a name from a resolver is dropped; the script
checks again and writes text, never markup).

### A translation key beats a form definition's literal title

Trigger: the tech team, 2026-10-12 -- the Developer module's form-definition editor.

The engine resolves a field's label from translation keys first and uses the
schema's `title` only when no key exists. A form-definition override therefore
cannot change a label that has a key: change the locale text (Translations
section) instead. The form editor and the designer's inspector name those fields
and link to the text. Once the form's page has been opened, the keys listed are
the ones that page asks for; until then they come from the two key conventions
(`<module>.forms.<form>.field.<field>`, `field.<field>`) and the editor says so
(DEC-33). The build carries the identities of 30 of the built-in forms
(`internal/ui/developer_pages_seed.json`, DEC-35), so for those the keys are
right from the first start; what an instance learns beyond that is kept in
`.history/pages.json` under an override directory (DEC-34). The other built-in
forms and new forms follow the convention until their page is opened. This is the engine's designed precedence (FORM_ENGINE_SPEC
section 4.1), recorded here because the editor makes it visible.

### Tenant overrides: instance sync is by polling

Trigger: the tech team, 2026-10-13 -- tenant-scope overrides (`internal/overrides`), DEV-28.

An instance loads every tenant's overrides at start and then checks every 15 seconds
(`devruntime.DefaultSyncInterval`) whether another instance changed them, so a change is live
elsewhere within about that time; a check reads the switch rows and both override entities.
Version numbers are the highest stored plus one; two instances saving the same form at the same
instant could number two versions alike, and the history then orders them by version and time.
Run against a live xolu with two instances (2026-11-17): a text, a form, a switch and a module
switch made on one were in force on the other within the interval, nothing has to be declared
before first use, and three rounds of the same form saved on both at once produced six distinct
versions and one agreed definition. Two saves landing in the same instant can still number two
versions alike; it was not reproduced. A failure to read the entities at start is logged and the
module starts without tenant overrides.

### The global scope and the module switches

Trigger: the tech team, 2026-11-17 -- DEV-26, DEV-27, DEC-36.

The global scope (`developer_global_tenant`) is a third place to save an override: for every
tenant, below each tenant's own override and below the files. Only the named tenant's developers
are offered it; a deployment that later sets it back to 0 leaves the stored global overrides
inert, not deleted. The export includes them, below the tenant's own overrides and the files
(DEC-37). A module switched on from the panel (`module_demo`, `module_work_orders`, `module_issues`) is on for the
developers of that tenant, or for every user of it who has access to the module when
`developer_module_audience` is `tenant`, until it is switched off; a module on at startup is on
for everyone and has no switch.

The switch for one tenant does not reach the public page of a QR code (T-131). Whoever scans a
code is not signed in, so there is no tenant to ask the switch for: `/s/{code}` offers the
report form, and `POST /s/{code}/report` exists, only when Issues is on at startup
(`enable_issues`). A tenant given Issues by the switch alone has the application and not the
anonymous channel. Accounting has the start-time flag and no switch (T-132).

Accounting is the one module that can run as the whole system, without Assets (`make
launch-accounting`). Work Orders and Issues cannot: an order and an issue are about an asset.
In such an instance Documents, Settings and Administration remain, because they have no flag;
only an administrator sees them.

### Texts not referenced in the source are candidates

Trigger: the tech team, 2026-11-17 -- DEV-19, DEC-36.

The "not referenced in the source" filter needs a checkout (`developer_source_dir`) and lists
the built-in texts no string of the source accounts for, by the rules in
`internal/i18n/keyusage`. A key assembled in a way those rules do not see is listed although it
is used, so the list is where to look, not what to delete. Nothing removes a text. The editor of
a text says which rule accounts for it and where in the source (a literal, a prefix, a naming
convention, parts joined at run time), or that none does (DEC-37). A scan of the source cannot
know more than that, so this is the feature's limit and not a defect (DEC-38).

### Developer preferences are read from the account on each page that offers them

Trigger: the tech team, 2026-11-17 -- DEV-29, DEC-36, DEC-37, DEC-38.

The scope and the language a developer last chose are kept with the account
(`developer_preferences`: `account`, the default), and a choice made in one browser is what any
other opens with. The cost is one read of the user record on each page that offers the choices;
the cookie of the module's pages is a copy, used alone when the account cannot be read.
`browser` keeps them in the cookie only, with no read.

### A text or a form created with the Developer module is used by nothing until code asks for it

Trigger: the tech team, 2026-11-17 -- creating texts and forms, DEC-31.

A new key exists only as an override, in the reference locale and in the languages it was then
translated into: the others fall back to the reference text. A new form exists only as an override
and no page renders it, unless `developer_custom_pages` is on: then it has a page of its own
(`/forms/{module}/{name}`) for any user of the tenant, and what the page receives is stored and
listed in the form's editor (DEC-37). Nothing else is done with a submission, by design: the
page is for trying a form with real users, and what a submission becomes is for the code that
adopts the form to decide (DEC-38). Both are for building a feature before its code is merged: the code asks
for them by name (`t("the.key")`, `resolveSchema(ctx, module, name, nil)`), and the export puts
them where the source tree keeps them, where a new form still needs its package to embed the file.
Removing the last override of either makes it unknown again. The new-text form takes the text in
every language at once and the text editor shows which languages have it; a form can be tried,
submission included, from its editor ("Try it"), which stores nothing and fills its lists leaving out
the rows switched off; given a record ("as if editing record"), it applies the filters that depend on the record being edited. The export brings the Go package of a new form's module and
lists the steps left; writing the page that renders it is the part that stays.

### Override history: what it holds

Trigger: the tech team, 2026-11-17 -- DEC-32.

A tenant's override of a text that predates the history is shown as version 1 and recorded as such
when it is next changed; what it said before that is not known. File-scope history
(`.history/texts.jsonl` and `.history/forms.jsonl` in the override directories) records what the
Developer tools wrote, with user and note, and what was changed in the files without them, as
"changed in the files", whether hot loading is on or off (DEC-33, DEC-34). On Linux the
directories are watched (`developer_watch`, DEC-37) and each change to a file is a version; where
there are no file-system events (macOS, or `developer_watch` set to `poll`) they are looked at
every 250 ms, so two changes to a file inside that time are one version (DEC-35). That is
accepted for macOS, a development machine (DEC-38). What a directory holds at its first load is not a version until it changes. The
`.history` directories belong with the override directories when these are copied.

### Forms outside the override mechanism

Trigger: the tech team, 2026-11-17 -- T-116, DEC-32.

Definitions that code generates per record have no embedded file and cannot be overridden: issue
templates, work-order actions and template steps. They are defined by their own records, which
is where they are edited, so this is by design (DEC-37). The asset update form is derived from
the asset definition in force, override included.
The gate (T-115) covers what the tools write and what the schemas directory loads: a file there
that cannot be read, is not a definition or could not be rendered is set aside alone: what was in
force for that form stays, the other files load, and the Overview and the forms list name it
(DEC-33, DEC-34). The translations directory follows the same rule: a broken locale file is set
aside alone and that language keeps its texts (DEC-35).

### The form designer: what it leaves to JSON

Trigger: the tech team, 2026-10-13 -- DEV-21, DEC-29, DEC-37, DEC-39.

The designer edits the fields of a form and its layout (`x-seam-form`): tabs, sections, sections
side by side in a row, and the fields in each, by drag and drop or by buttons. The details and
embeds of a tab are edited as the JSON lists the engine reads, not with controls of their own,
and that is the form they keep (DEC-38). A field, a section and a tab can be duplicated; the copy
of a section or a tab holds copies of its fields under the next free names. The preview can be
rendered in any language; it has no role selector, because no page resolves form permissions
per role (DEC-39). Keys of the layout the designer does not know are kept as they are.

### Seeds: what the `.seed` files and their loader do not cover

Trigger: the tech team, 2026-11-18 -- T-120, docs/SEED_FORMAT.md.

The seven seeds, and the additions for work orders, Issues and accounting, are `.seed` files loaded by
`seeds/seed_system/`. xolu's own `scripts/seed.py` reads format version 1 and refuses these files
(version 2) until xolu takes the newer format. The Issues seeds need xolu's API v2
(`XOLU_API_V2_ENABLED=true`); the history of each seeded issue carries the time of the load, since its
steps are made then, while the issue's own dates are in the past. The accounting seed has fixed dates
(July to November 2026) because the books are records of those months; it carries the record of each
invoice and payment proof and not the file, which Seam keeps on disk, so opening one answers "not
found". `scripts/launch.sh` loads data only on the first run of a data directory: what a newer
version of the seeds adds is loaded by hand (`python3 seeds/seed.py load NAME`). A load
records itself in the entity type `seed_loads`, so a seeded tenant has one
more entity type than before. After a load, the next id xolu assigns in each entity type skips one (a xolu
behavior after a save with an explicit id, reported in docs/SEED_FORMAT_PROPOSAL.md, P-6). The password
hashes in the files are fixed bcrypt hashes of the demonstration passwords the catalog publishes.
Loading has been run on Linux only; the loader uses nothing outside the Python standard library.

## Dormant guards

| Guard | Build tag / gating | Requirements | Canonical invocation | Last exercised |
|---|---|---|---|---|
| `seeds/conformance/run.py` (35 cases, T-120) | None -- plain Python 3; `make test` does not run it | A xolu binary, 0.30.44 or later (`XOLU_BIN`); it starts one per case on a free port, with API v2 on | `make test-seeds-live XOLU_BIN=/path/to/xolu` (or `XOLU_BIN=... python3 seeds/conformance/run.py`) | 2026-11-18, xolu 0.30.44 built from source, 35 of 35 passed, sandbox session (Linux) |
| `TestSchemaRoundTrip_XSeamExtensionsSurvive` | `integration` build tag; skips if `XOLU_URL` unset | A running xolu instance (any storage backend) | `XOLU_URL=http://host:port go test -tags integration ./internal/formengine/... -run TestSchemaRoundTrip -v` | 2026-07-28, local xolu v0.16.16 built from source (`XOLU_AUTH_TYPE=none`), sandbox session |
| `scripts/olu_to_xolu_docs.py` | None -- plain Python 3, no build tag | Nothing (pure text scan of the living spec docs) | `python3 scripts/olu_to_xolu_docs.py` (add `--apply` to fix, default is check-only, exit 1 if anything found) | 2026-07-28, initial run found and fixed 148 occurrences across 7 files, sandbox session |
| `internal/ui/workflow` (6 end-to-end tests, T-25) | Separate Go module (`internal/ui/workflow/go.mod`); `make test` does not run it | A local xolu checkout; guards with an error if not found at `XOLU_PATH` (default `/tmp/olu`) | `make test-workflow XOLU_PATH=/path/to/xolu` | Not run this session (2026-08-13) -- module's own `go.mod` pins are current again (xolu v0.30.6, minty v0.2.1, queryfy v0.3.2, go 1.26.5, confirmed directly via `go mod tidy`, T-85), but the suite still does not compile: `go vet` surfaces real API staleness in xolu's internal packages (`pkg/config`/`pkg/server`/handler surface) this module imports directly, beyond `pkg/client`'s stable boundary. Tracked as T-86, not yet fixed; do not trust a pass until that closes |
| `scripts/browser-test-demo.js` (88 checks in the light and dark themes: every script-driven widget in server-rendered and added repeatable rows, barcode colours and invalid state, the GPS dropdown and pin, picker dropdowns, any failed request) | Needs the page dumped by `TestDemoKitchen_DumpPage` (skips unless `SEAM_DEMO_DUMP` is set); not part of `make test` | Node 18+, `puppeteer-core` and a Chromium (`CHROME_BIN`); no network needed (map tiles that fail to load are ignored) | `SEAM_DEMO_DUMP=$PWD/build/demo.html go test ./internal/ui -run TestDemoKitchen_DumpPage && PUPPETEER_CORE=<path to puppeteer-core> CHROME_BIN=<chromium> node scripts/browser-test-demo.js build/demo.html` (the dump is one file per page of the Demo module, `build/demo-inputs.html` and so on, written next to the named file) | 2026-11-18, 88/88 over the Fields and Rows pages, headless Chrome on Linux (container). Earlier, 2026-10-21, 84/84, headless Chromium 1194 on Linux (container); mutation-checked: removing the `SeamWidgets.init(row)` call fails the added-row checks, removing the aria-invalid CSS rule or the stale-drawing clear fails the barcode checks, removing the row spanning of a wide GPS panel fails the whole-row width checks |
| `scripts/browser-test-assets.js` (30 checks in the light and dark themes: the Assets edit form loaded by an htmx swap; the related-asset picker's stored label, list on focus and when emptied, typed search and stored choice; the Map & photos tab's GPS dropdown, compact buttons and SVG pin; the photo uploader's owner; any failed request) | Needs the fragment dumped by `TestAssetForm_DumpPage` (skips unless `SEAM_ASSET_DUMP` is set); not part of `make test` | Node 18+, `puppeteer-core` and a Chromium (`CHROME_BIN`); no network needed (map tiles that fail to load are ignored) | `SEAM_ASSET_DUMP=$PWD/build/asset-form.html go test ./internal/ui -run TestAssetForm_DumpPage && PUPPETEER_CORE=<path to puppeteer-core> CHROME_BIN=<chromium> node scripts/browser-test-assets.js build/asset-form.html` (`THEMES=light,light,light` repeats the run in one browser) | 2026-10-21, 30/30 and three consecutive light runs 45/45, headless Chromium 1194 on Linux (container); the picker failure was reproduced first (second and later page loads failed) and is fixed |
| `scripts/browser-test-assets-modal.js` (22 checks per theme: the Assets form opened in the real SeamModal by htmx.ajax, served by the real handlers over a stateful fake xolu; real clicks on Create from the first tab and from Map & photos, Create with a required field empty from another tab, Create and add photos, Cancel, Edit; the photo uploader inside the modal, with Dropzone served slowly; a click on each part of the drop area (icon, button, hint, empty area) opening the file chooser, a real photo uploaded through it, enlarged, and still there when the asset is reopened; the Photos section titled once; any failed request) | Needs the server started by `TestAssetForm_Serve` (skips unless `SEAM_ASSET_SERVE_FILE` is set; it runs until `<file>.stop` exists or 150 s pass); not part of `make test` | Node 18+, `puppeteer-core` and a Chromium (`CHROME_BIN`); no network needed | `SEAM_ASSET_SERVE_FILE=$PWD/build/serve.txt go test ./internal/ui -run TestAssetForm_Serve -timeout 200s &` then, once the file exists, `PUPPETEER_CORE=<path> CHROME_BIN=<chromium> node scripts/browser-test-assets-modal.js build/serve.txt`, then `touch build/serve.txt.stop` (`THEMES=light,dark` for both themes) | 2026-10-21, 44/44 (both themes), headless Chromium 1194 on Linux (container); mutation-checked: copying only the text of re-created scripts fails the uploader checks, removing the invalid-field handler fails the hidden-tab check, removing the reload on close fails the reload check, removing the wait for Dropzone fails the first-open check, letting the drop area's children take the click fails the upload, removing the click on thumbnails fails the enlarge checks, removing both the CSS and the click forwarding on the drop area leaves the icon, button and hint dead (either one alone is enough) |
