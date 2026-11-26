# Changelog

All notable changes to Seam AMS are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [0.4.5] — 2026-11-18

### Upgrading
- **The Issues module is now off unless it is switched on** (T-131), as Work Orders and
  Accounting are. An instance that uses it sets `SEAM_ENABLE_ISSUES=1`, or
  `"enable_issues": true` under `features` in its `config.json`, before starting this version;
  without it `/issues` answers 404, the menu has no entry, and a scanned QR code shows the asset
  with no form to report from. No data is touched either way. Instances started with
  `make launch`, `make dev` or a demo package over a dataset need nothing: those switch on the
  modules their data is for.

### Fixed
- **Interface text that did not follow the language** (T-133 to T-138). The frame of every page
  was English whatever the language (the skip link, "Open menu", "Notifications", "User Profile",
  "Log Out", the pager's "Previous" and "Next"); a tooltip and one error reply were Spanish for
  everyone; the scripts held eleven texts of their own, one in Spanish. All of them come from the
  locale files now. The scripts get theirs from the page (`data-i18n-texts` on `<html>`, read
  by `SeamText` in app.js). 89 error replies go through one helper and a small family of keys
  (`error.save_failed`, `error.invalid_form_data` and so on) in place of 36 English literals.
- **The form engine never looked placeholders up.** 13 translated placeholders of the asset
  form existed in the locale files and were not read; a placeholder now has a key
  (`<module>.forms.<form>.field.<name>.placeholder`, and
  `repeatable_item.<parent>.<item>.placeholder` in a row). 22 other form texts that had no key
  have one (Issues' Assignee and Severity, the rule operators of work-order templates and others).
- **Spanish is in one form of address.** 85 texts used voseo ("Seleccioná", "Ingresá", "querés");
  they are in tuteo, as the rest of the file. A test keeps it so.
- **An asset's criticality and the dates written by the pages follow the language.** The stored
  words (Alta, Media, Baja) are translated at display; dates are "2 Nov 2026", "2 nov 2026" or
  "2026年11月2日".
- The literals a component shows when it has no translator are English, the base language:
  they were Spanish in lists, masters forms and the form engine's row buttons.
- **A new issue answered 500 on an instance started by `scripts/launch.sh` or a demo bundle**
  (T-121). The lifecycle of an issue is a xolu state machine, under `/api/v2`, and xolu serves
  those routes only with `XOLU_API_V2_ENABLED=true`, which neither script set. How Seam runs
  xolu is now one list, `scripts/lib/xolu-env.sh`, used by `scripts/launch.sh`, by
  `scripts/build-demo.sh` when it loads the data and in the two launchers it writes, and by
  `make start-xolu`; `docker-compose.yml` is tested against it.
- **`make olu` and `make start-olu` did not work.** They built `./cmd/olu` into `xolu/olu` and
  started it with `OLU_*` variables, none of which xolu reads, on a port variable the Makefile
  never set. `make xolu` and `make start-xolu` replace them and go through
  `scripts/launch.sh` (`--xolu-only`); the old names are aliases.
- **`make user-list XOLU_URL=...` and the other user targets ignored the address.** They passed
  `SEAM_OLU_URL`, and `seam-admin` reads `SEAM_XOLU_URL`, so every one of them talked to
  `localhost:9090` whatever was asked. `OLU_URL=` still works as a name for `XOLU_URL=`.
- **`make launch-dev`, `--seam-port` and `--xolu-port` did nothing on a data directory that
  already had a `config.json`**, which is written once: the launcher waited on the new port
  while Seam listened on the old one. The launcher now passes ports and dev mode to Seam in
  its environment, which wins over the file.
- **`scripts/launch.sh` refused Japanese** (`--lang ja`), which Seam has and the demo
  packages offer.
- **A demo package could carry an old Seam.** `scripts/build-demo.sh` skipped compiling when a
  binary was in `demo/build/`, whatever had changed since; it compiles on every run (Go's
  build cache makes an unchanged binary quick).
- **`make bundle` zipped `demo/`** (packages and their build cache, hundreds of megabytes),
  Python caches and the edit journal of the working copy. They are left out, and `launch.sh`
  in the zip is marked runnable.
- **`seam-admin user list` cut emails and broke its columns**: an email longer than 23
  characters was shortened to something that cannot be typed back, a role longer than 10
  pushed the next column out, and a name was cut by bytes, which can split a character. The
  columns are as wide as what they hold, measured by screen columns (`cmd/seam-admin/table.go`).
- **`docker-compose.yml` gave Seam four variables it does not read** (`SEAM_SERVER_HOST`,
  `SEAM_SERVER_PORT`, `SEAM_XOLU_URL`, `SEAM_AUTH_JWT_SECRET`), so the address of xolu and the
  JWT secret came from `config.json` whatever the environment said, and it started xolu
  without API v2, full text, time series or cascading delete. The names and the settings are
  corrected. The file still cannot be run: there is no `Dockerfile` (T-124).
- **A dataset and an addition named in one `seed.py load` failed**: the addition's lookups
  ran before the dataset's records. Additions run as passes of their own, after the files they
  add to (`docs/SEED_FORMAT.md`, section 4).
- **Signing in removed fields from the user record** (`internal/auth/store_xolu.go`,
  `internal/server/server.go`): the user store replaced the whole record on every update, and
  every sign-in makes one, so a field the store does not list was lost at the next sign-in. It
  now merges its fields into the record. Found with the Developer preferences kept in the account.
- **Developer module** (DEC-37): the override directories are watched on Linux, so two changes to
  a file in quick succession are two versions (`internal/dirwatch`; polling elsewhere and as the
  safety net). A module switched on at runtime is on for the tenant's developers by default, not
  for every user. The export includes the global-scope overrides. The asset update form follows
  the override of the asset definition. All 37 built-in forms are in the identity table, held by
  a test. Developer preferences follow the account to another browser, including one that already
  holds an older copy (DEC-38).
- **A broken locale file no longer holds back the other languages** (DEC-35). The translations
  directory loads file by file, as the schemas directory does: a file that cannot be read, is not a flat
  object of texts or is not named after a locale is set aside alone, that language keeps the texts it
  had, the others load, and the Overview names the file. Files set aside at start are logged.
- **The schemas directory sets aside a definition the engine cannot render** (T-115, DEC-33). The file
  layer runs the same check as the editors when it loads (`devruntime.Options.SchemaCheck`,
  `schemaoverlay.FileLayer.SetCheck`): a file that fails it is not put in force, what was in force for
  that form stays, the rest of the directory loads, and the Overview and the forms list name the file
  and the reason. A file that cannot be read or is not a definition is set aside the same way: it no
  longer stops the reload of the other files (DEC-34).
- **The topbar search placeholder is translated** (`layout.search.placeholder`, four locales).
- **A form definition the engine cannot render is refused when it is written** (T-115, DEC-32).
  `formengine.CheckRenderable` refuses a definition with no fields, with a widget the engine does not
  have (repeatable items included) or with a select whose source nothing resolves for that form. The
  JSON editor, its preview, the designer and restoring a version all pass through it and answer 422 with
  the reason; the form in use is left as it was. A definition written by hand into the schemas
  directory is still loaded, and the forms list and its editor mark it "cannot be shown" with the reason.
- **The two forms that handlers built inline can be overridden** (T-116): the issue assignment form and
  the outsourced account form are embedded schemas (`issues/assign`, `outsourced/account`) and go through
  `resolveSchema`. The catalogue lists 37 forms.
- **Region, currency and brand forms follow their overrides** (T-113). Their routes render the fields with
  the form engine, for the request's tenant and locale (`h.regionFormPage`, `h.monedaFormPage`,
  `h.marcaFormPage`); the page functions keep the envelope. Labels come from new `masters.forms.*` keys with
  the same texts. The "not live" marker added earlier in this release is gone.
- **Eleven forms in eight packages could not be overridden** (T-116): consumable, crew, movement,
  outsourced, unit, warehouse, wo_action and workorders_templates are in the Developer catalogue and their
  27 render and parse sites go through `resolveSchema`. A test fails when a schema package is in neither
  the catalogue nor an explicit left-out list.
- **Form designer: every button was dead until the preview's required fields were filled in** (T-114). The
  page was one form that also held the live preview, whose inputs carry `required`, so the browser refused
  each submit. The designer's form now holds only its hidden fields; its controls name it with `form=`, its
  buttons carry `formnovalidate`, and the preview is in a form of its own that never submits.
- **A refused form-definition save logged `superfluous response.WriteHeader`** (T-117) and lost its
  Content-Type: the handler wrote 422 and the page renderer wrote 200. New `PageHandler.RenderStatus` writes
  the status once; `Render` is the same with 200.
- **Developer runtime switches could not be saved on a fresh instance** (T-112; issue report of 2026-10-15,
  "No se pudo guardar el cambio", `entity 'feature_state' does not exist`). xolu answers a query on an entity
  with no rows yet with an error; the switch store read before writing and stopped there, so the first row was
  never created. `auth.EntityNotYetCreated` now recognises that answer and `internal/devstate` treats it as
  "nothing stored"; the start-up error "could not read persisted panel state" goes with it. The override
  stores (`internal/overrides`: `i18n_override`, `schema_override`) had the same fault and the same fix.
- **Repeatable checkbox and toggle field title translation**
  (`internal/formengine/repeatable.go`, `repeatable_indexed.go`): `repeatableRow` and
  `indexedRow` now resolve `itemField.Title` using `resolveRowItemTitle` for `checkbox`
  and `toggle` widgets, allowing `repeatable_item.<repeatable>.<field>` i18n keys to
  translate the label instead of falling back to the schema literal title.
- **Decimal widget placeholder in repeatables and forms**
  (`internal/formengine/widgets.go`, `repeatable.go`): `renderDecimal` now wraps input
  attributes with `withCellPlaceholder`, rendering `placeholder` and `aria-label` attributes
  from `x-seam-placeholder` or repeatable item i18n keys (`repeatable_item.<repeatable>.<field>`).
- **`SeamConfirm` cancel button i18n translation**
  (`web/static/js/app.js`, `internal/ui/layout.go`): `SeamConfirm.show` now supports
  `opts.cancelLabel` and resolves the cancel button text dynamically from `defaults.cancelLabel`
  or the layout's `data-i18n-cancel` attribute (`action.cancel`), rather than displaying
  hardcoded `"Cancel"`.

### Added
- **A seventh dataset, in Portuguese: Beira Magalhães Hotéis e Resorts Ltda.**
  (`seeds/seed-07-beiramagalhaes.seed`), a hotel and resort group in Bahia, Brazil. It is the
  first dataset whose content is Portuguese and the first from the hospitality sector: 16 assets
  of 7 types (air conditioning, kitchen, elevators, laundry, generators, boilers, transfer vans)
  in 7 locations, 166 records. `make launch SEED=07-beiramagalhaes` starts Seam in Portuguese over
  it. Its Issues seed (`issues-07-beiramagalhaes`) has the same 12 issues in every state as the
  others, written for a hotel; the work-order and accounting seeds load over it as over any
  dataset, and the books of the accounting seed keep the six companies they had. A conformance
  case pins the dataset with its Issues seed.
- **`docs/ISSUES_MODULE.md`.** Issues is treated as an application module from here on, not
  as the small tracker it began as: the document describes it as it is (entities, lifecycle,
  deadlines, templates, QR codes, cases, who can do what, what it needs of xolu) and lists
  what it lacks against the other application modules, each gap filed: permissions of its
  own (T-125: checked on a running instance, a read-only viewer and an account executive
  can act on an issue nobody is responsible for), lifecycle steps no screen sends (T-126),
  no audit log (T-127), no check at start of what it needs of xolu (T-128), a Spanish word
  stored as a constant (T-129), no dashboard or API (T-130). The comments in
  `internal/server/server.go`, `internal/ui/router.go` and `internal/modules/issues` that
  called it "simple" say what it is and point to T-125. No behavior changes.
- **One output library for the scripts** (`scripts/lib/ui.py`, `scripts/lib/ui.sh`). Boxes,
  tables, two-column layouts, rules, status lines and questions, measured by the columns text
  takes on the screen and not by its bytes: the hand-drawn boxes came out crooked wherever a
  line had an accent, a dash or a box character, and broke when a path was longer than the
  box. Its invariants are stated at the top of `ui.py` and tested (`make test-scripts`):
  every line of a block has the same width whatever the text; text wraps instead of breaking
  a border and a block is never wider than the terminal; colour is decoration only and off
  through a pipe or with `NO_COLOR`; red is for errors only; `SEAM_ASCII=1` or a locale that
  is not UTF-8 gives ASCII. `scripts/launch.sh`, `scripts/build-demo.sh`, `scripts/bundle.sh`
  and `make help` print through it; the launchers of a demo package, which run where there is
  no Python, carry boxes drawn at build time.
- **`make help` is written from the Makefile** (`scripts/make-help.py`): `##@` starts a
  section, `target: ## text` describes a target, `##>` is a note. A target cannot be added or
  renamed without its help following, and the help no longer names variables nothing reads.
- **A launch can start with data** (T-123). The first run of a data directory asks what the
  instance starts with: empty with an administrator, or one of the seven datasets with its work
  orders, issues and accounting. `--seed NAME` (`make launch SEED=...`) answers it, `--yes`
  asks nothing, `--additions` chooses what comes with the dataset. `make dev` loads
  `01-constructec` on the first run and turns on Work Orders and Accounting, so every module
  has data and the developer login it names exists. The demo packages load the same
  additions (`make demo SEED=... PLATFORM=...` builds one without questions). Both go
  through `scripts/lib/seeds.sh`.
- **`scripts/launch.sh --xolu-only`, `--no-start`, `--verbose`.** What xolu and Seam print
  goes to `xolu.log` and `seam.log` in the data directory instead of the terminal, where
  loading a dataset used to scroll some 700 lines past the launcher's own; `--verbose` puts
  it back, and a server that fails to start has the end of its log shown. The language of
  the interface is asked once for a data directory and remembered. Stopping with Ctrl-C is
  no longer reported by make as an error.
- **`make test-scripts`**, 37 tests: the output library, and that the Makefile, the launcher,
  the demo builder and `docker-compose.yml` agree with each other (one list of xolu settings,
  one way to load data, the same languages, every public target described, options in the
  help equal to the options parsed, nothing bash 3.2 lacks). `make test` now runs it and
  `make test-seeds` after the Go tests.
- **`.seed` format version 2, its loader and its conformance suite** (T-120). `docs/SEED_FORMAT.md`
  specifies the format: version 1 is xolu's; version 2 adds records over several lines, placeholders
  for dates, times, variables and text, descriptive header keys, and the rules of a load (what is
  refused, what a second load does, what is recorded). `seeds/conformance/` holds 35 cases that any
  loader with the same command line can be held to, run on a real xolu
  (`make test-seeds-live XOLU_BIN=...`); `make test-seeds` runs 25 tests that need no server.
  `docs/SEED_FORMAT_PROPOSAL.md` is the review that led to it.
- **Seeds that add to a dataset** (T-120). The format gains what a file needs to load over data
  whose ids it cannot know: `match` (a record found by fields of its own and left alone if it is
  there), `find` (a lookup that writes nothing and stops the load if the record is missing), the
  header keys `addition` and `requires`, a bound id inside a text (`${seed:name}`), and
  `fsm_def` found by name. `seed.py` finds a seed by name in the module directories under
  `seeds/`, and `seed.py list` shows them.
- **Issues module seeds** (`seeds/issues/seed-issues-NN-name.seed`, one for each of the seven
  datasets, in its language: Spanish, English, Japanese, Portuguese). Each requires its dataset and adds 12
  issues in every state of the lifecycle with their state machines and histories, one case with
  two issues grouped under it, 2 issue templates (one with two versions) assigned to 2 asset
  types, and 4 QR codes (one revoked). `python3 seeds/seed.py load issues-01-constructec`.
- **Accounting & Admin module seed** (`seeds/accounting/seed-accounting.seed`, 369 records):
  5 users of the module's roles, 5 AEs (two virtual, two authorized, one waiting), 7 tenants,
  38 billable items from July to November 2026, 72 journal entries, 30 commission lines,
  4 settlements (three paid), 9 document records, 4 monthly closes and 5 formula rules. It is
  written by `cmd/accounting-seedgen` (`make seed-accounting`), which runs the module's own
  Service over a store in memory with a clock it moves through the five months, so the books
  follow every rule of the module; no record carries an id, and the file loads before or after
  the module has made its chart. `python3 seeds/seed.py load accounting`.
- **Developer module: what was missing for daily use** (DEC-37). *Form designer:* the layout of a
  form is editable as a tree: tabs, sections, sections side by side in a row, and the place of each
  field, by drag and drop or by buttons; details and embeds of a tab as JSON lists
  (`internal/formengine/designer/layout.go`, `internal/ui/developer_designer_layout.go`). A field
  the layout places can be removed or renamed and the layout follows. *Forms list:* the search
  also matches what a definition uses (widget, source, field) and says which. *Custom form pages*
  (`developer_custom_pages`, `SEAM_DEVELOPER_CUSTOM_PAGES`, off): a form that exists only as an
  override has a page, `/forms/{module}/{name}`, and its submissions are stored
  (`form_submission`) and listed in its editor. *Try page:* "as if editing record" applies the
  filters that depend on the record. *Translations:* the editor of a text says which rule of the
  source scan accounts for it, and where. *Settings:* `developer_watch` (auto, events, poll),
  `developer_module_audience` (developers, tenant), `developer_preferences` (account, browser).
  *Designer, last items* (DEC-39): a field, a section and a tab can be duplicated
  (`internal/formengine/designer/duplicate.go`); the preview has a language select. DEV-21 is done.
- **Developer module: the campaign's remaining items** (DEC-36). *Global scope* (DEV-26): overrides of
  texts and forms that apply to every tenant, below each tenant's own, with the same history, restore
  and sync; offered only to the developers of the tenant named by `developer_global_tenant`
  (`SEAM_DEVELOPER_GLOBAL_TENANT`, none by default). *Module switches* (DEV-27): the Demo and Work
  Orders modules can be switched on for one tenant from the panel when they are off at startup
  (`developer_module_switches`, on by default; runtime flags `module_demo`, `module_work_orders`).
  *Texts not referenced in the source* (DEV-19): a filter of the Translations list, offered when
  `developer_source_dir` (`SEAM_DEVELOPER_SOURCE_DIR`) names a checkout; `make dev` sets it.
  *Preferences* (DEV-29): the scope and the language a developer last chose are remembered in their
  browser. *Settings* (DEV-9): `developer_poll_ms` and `developer_history_ms` configure how often the
  override directories are loaded and looked at; the Startup page lists every setting in force.
- **Seeds: a manager and a viewer account in each of the six** (DEV-23), and the catalogue describes
  seeds 05 and 06. `docs/I18N_SPEC.md`, `docs/ADMIN_MODULE_SPEC.md` and `ROADMAP.md` point at the
  Developer module where it already provides what they plan (DEV-24).
- **Developer module: page identities ship with the build, finer history, try lists filtered** (T-118,
  DEC-35). The identities each form's page renders with are in `internal/ui/developer_pages_seed.json`
  (30 forms), so the editors name the right label keys from the first start, with or without an
  override directory; a form rendered by two pages keeps both identities. The override directories
  are looked at every 250 ms for the history, whatever the load interval. Try-page lists leave out
  rows switched off (`enabled`, `active` or `is_active` false).
- **Developer module: what is learnt and recorded no longer depends on a switch or a restart**
  (T-118, DEC-34). The page identities the editors use for label keys are kept in
  `.history/pages.json` under an override directory and read at start. Changes made in the files are
  recorded while hot loading is off too (the poller observes without loading). The try page fills the
  lists of any `entity:<name>` source and names the ones that come back empty. The export includes the
  Go package of a new form's module and lists the steps left for each new form.
- **Developer module: try a form, write a new text in every language, history of changes made in the
  files, label keys from the page** (T-118, DEC-33). "Try it" (`/developer/forms/try`) renders any form
  as it is in force for the tenant and shows a submission as the server receives it, with the checks
  made on a post; nothing is stored. The new-text form has an optional field per language besides the
  reference one. A text or a definition changed, added or removed in an override directory without the
  tools is recorded as a version ("changed in the files") when the layer loads it, including changes
  made while the server was down. Once a form's page has been rendered, the editors name the label keys
  that page asks for (`formengine.SetRenderObserver`); before that they follow the naming convention
  and say so.
- **Developer module: history in the file scope, and the editors say more** (T-118, DEC-32). File-scope
  overrides keep their versions beside the files (`.history/texts.jsonl`, `.history/forms.jsonl`), with
  who, when and the note, and an older version can be restored; a tenant's override of a text made
  before text history existed becomes version 1 the first time it is changed. The form editor and the
  designer's inspector name the fields whose label comes from a translation key and link to that text.
  The text editor shows the text's state in every language (translated, not translated, missing) and
  opens each one. A custom form shows the line of code that renders it. With no schemas directory the
  form editor shows the definition, read-only.
- **Designer texts in four languages**: widget names and property titles and help texts
  (`designer.widget.*`, `designer.prop.*`; 133 keys per locale with the new editor texts).
- **Developer module: new texts, new forms, text history and export** (T-118, DEC-31). "New text" creates a
  text under a key that does not exist yet, in the reference locale, in either scope; "New form" creates a
  form under a new module/name and opens it in the designer (forms `developer/translation_new`,
  `developer/form_new`). A tenant's override of a text keeps its versions (`i18n_override_history`) and an
  older wording can be restored. "Export overrides" (`GET /developer/export`) downloads the tenant's
  overrides as a zip in the override directories' layout and in the source tree's. New
  `i18n.EmbeddedLocale`, `i18n.ValidNewKey`, `overrides.Service.TranslationHistory` and
  `RestoreTranslation`; `overrides.Service.RemoveTranslation` takes who removed. 30 new keys per locale.
- **Root `.repoman.json`** recreated (it was missing from the checkpoint): `version_targets` for
  `docs/TRACKING.md`, `docs/KNOWN_ISSUES.md`, `docs/formengine/TRACKING.md` and
  `docs/modules/TRACKING.md`; every other key takes gorepoman's default. The `Version:` headers of
  those four documents, stale at 0.4.1-pre8, are synced to 0.4.3.

- **`query-picker` form engine widget** (T-111, `docs/QUERY_PICKER.md`): choose one record from the rows of a
  field's own OQL query, shown with typed columns (text, number, money, date, badge, icon) in a table or a
  two-line dropdown, with keyboard and pointer choice, an optional detail line and an optional Browse dialog
  (the Selector, over the same query, with paging). Schema keys `x-seam-query`, `x-seam-columns`,
  `x-seam-value-field`, `x-seam-label-fields`, `x-seam-dropdown`, `x-seam-detail`, `x-seam-limit`. The query is
  checked when the schema is parsed (one read-only `SELECT`, named columns, every `LIKE` in its own
  parentheses, listed entities only, at most one `JOIN`) and reaches the page only as an encrypted token
  (`internal/querypicker`); `GET /picker/query` and `GET /picker/query/browse` (`internal/ui/picker_query.go`)
  serve it. Columns that reference a small entity use a cached lookup instead of a `JOIN`; an identical
  search within five seconds is answered from memory. `QueryPickerErrors` checks submitted values. New
  "Query picker" Demo tab; 21 keys in `en`, `es`, `pt`, `ja`; designer catalogue entry.
- **`internal/oqlguard`**: the read-only OQL checks, shared by accounting's OQL formulas (which now use them
  from here) and the query-picker.
- **Selector dialog**: a source may supply its own links (`SelectorSource.URL`) and a non-numeric row value
  (`SelectorRow.Value`); a list wider than the dialog scrolls sideways.
- **Entity schemas for the Constructec seed** (`seeds/schemas/seed-01-constructec.schemas.json`, 20 entities,
  115 typed fields) and an opt-in `--schemas` flag on `seeds/seed-01-constructec.py` that registers them in
  xolu before seeding. Off by default: see T-110 for the measurements and what must change first.
- **Accounting as the whole system: `make launch-accounting` and `make demo-accounting`.** Seam
  starts without the Assets module (`enable_operational` off, the flag it already had) and with
  Accounting on, over the accounting seed alone, with no dataset under it. `make
  launch-accounting` uses a data directory of its own, `data/accounting-only`
  (`ACCOUNTING_DATA_DIR=`), which stays that way on later launches; the launcher refuses to mix
  it with a dataset, or to turn a whole instance into one. `make demo-accounting` builds the
  same thing as a package. Both are `--only accounting` in `scripts/launch.sh` and
  `scripts/build-demo.sh`; the modules that can run this way are one list
  (`modules_standalone`, `scripts/lib/modules.sh`), which holds Accounting alone, since Work
  Orders and Issues are about assets.

- **A developer can view Accounting as each of its roles.** "View as a role" in the Developer
  panel offers admin staff, accountant and account executive where Accounting is on (it
  offered the four platform roles alone); on an instance without Accounting they are not
  offered and not accepted. The accounting seed now has the developer account of the other
  seeds (`dev@ual.li`, found by its email when a dataset is loaded, created otherwise), and
  that account is itself an authorized account executive with a customer of its own, Sandbox
  Demo Co., so the portal viewed as an account executive has commissions, a paid settlement
  and a balance. Its collections are kept apart from the story of the other tenants, whose
  entries are what they were. `make dev-accounting` starts Accounting alone with the Developer
  module on.

### Changed
- **The module menu of the top bar is called "Modules" and is a panel.** Its heading said
  "Apps" ("Aplicaciones", "Aplicativos", "アプリ"), a word nothing else in Seam uses for them; it
  now says "Modules", and so does the button, beside its icon. The list of small rows is a
  panel 35rem wide with the application modules as tiles in two columns (a large icon on a
  tinted chip, the name, and a line saying what the module is for, in the four languages), the
  system modules as links in a row under them, and the module the page belongs to marked.
  With one application it is a single narrow column. Escape closes it, as a click outside did.
- **The icon field shows where the icon will be.** With no icon chosen, the preview beside the
  search box was blank space; it is now an empty slot of the icon's own size, a dashed outline
  with a muted ellipsis, in the light and dark themes. Choosing an icon replaces it and
  clearing the field brings it back; nothing moves either way. It is one CSS rule
  (`.fe-icon-preview:empty`), so it holds in every form and in rows added later.
- **The Demo module is an overview and three pages** in place of one page with twelve tabs in a
  row that scrolled sideways. `/demo` lists the pages and links to every demonstration;
  `/demo/inputs` (Fields: inputs, choices, capture, colours), `/demo/pickers` (the Selector and
  any-picker, selectors over several entities, the query picker) and `/demo/rows` (repeatable
  rows and their saved records, layouts, buttons, reordering, widgets inside rows) each hold
  their own tabs, and are in the module's left menu. A page renders only its own forms.
  `?tab=` opens a tab of the page; the echo page links back to the demonstration the form
  was on; an address from before (`/demo?tab=repeatable`) goes to its page. No demonstration,
  schema or text was changed. `scripts/browser-test-demo.js` reads one dump per page.
- **`/` and the menus follow the Assets flag.** With `enable_operational` off the routes of
  Assets were already unmounted, but `/` still showed its dashboard and the module selector
  still offered it. Now the selector has no Assets entry, and `/` sends each user to their first
  application (Work Orders, Issues or Accounting, whichever they have; otherwise the first
  module they can open, or their profile). An instance with Assets on is unchanged.
- **Issues follows the module lifecycle of Work Orders** (T-131; `docs/ISSUES_MODULE.md`). It has
  a flag in the catalogue (`issues`: `enable_issues`, `SEAM_ENABLE_ISSUES`, class preview, off by
  default) and a switch for one tenant in the Developer panel (`module_issues`). Its routes are
  not mounted when it is off, or are mounted behind the same gate as Work Orders' and answer 404
  for a tenant it is off for; the menus ask the same question. The public page of a QR code
  follows the flag: with Issues off it shows the asset and offers no report form.
- **Work Orders, Issues and Accounting are switched on in one manner by every script**
  (`scripts/lib/modules.sh`). The header of a seed says which module its data is for
  (`x.seam.module`; the two work-order seeds now say it too). `scripts/launch.sh` and
  `scripts/build-demo.sh` switch on the modules of the data they load, so `make launch
  SEED=03-extinfire` no longer loads work orders and accounting into an instance that hides
  them. `--modules all|none|issues,work_orders` (`MODULES=` in `make launch`, `make launch-fast`,
  `make launch-dev` and `make demo`) names them instead; the launcher keeps the choice with the
  data directory (`.modules`) and uses it on later launches. A variable of Seam set by the
  caller (`SEAM_ENABLE_ISSUES=0`) wins. `make dev` sets the three. An empty instance has none on,
  as a new installation of Seam does.
- **The seeds are `.seed` files, loaded by a common module** (T-120; `docs/SEED_FORMAT.md`).
  `seeds/seed-01-constructec.seed` to `seed-06-asahikenki.seed` hold the data, one JSON record per
  entity with its id, laid out one field per line. `seeds/seed_system/` loads them: Python 3.8,
  standard library only (`requests` and `bcrypt` are no longer needed). The six commands keep their
  names and options (`python3 seeds/seed-01-constructec.py --url ... [--dry-run] [--schemas]`) and
  gain `--check`, `--replace`, `--allow-existing` and `--as-of`; `seeds/seed.py` is the general
  command (`load`, `check`, `dump`, `list`, `info`). Behavior that changed: a seed is refused on an
  instance that holds another seed or other data, instead of being loaded over it; run a second
  time, it leaves the records that exist alone instead of duplicating them; each load is recorded
  in the tenant (`seed_loads`); dates that depend on the day of the load are placeholders in the
  file (`{"$date": "+180d"}`), resolved at load time. Each file was checked against the Python
  script it replaces: loaded as of a fixed moment, it gives the same records, field for field.
  `scripts/build-demo.sh` reads the company and the login of a seed from its header and no longer
  needs Python modules. The seeds catalog is updated. The schemas of seed 01 moved from
  `seeds/schemas/seed-01-constructec.schemas.json` to `seeds/seed-01-constructec.schemas.seed`.
- **The work-order seeds are `.seed` files** (`seeds/workorders/seed-wo-actions.seed`,
  `seed-wo-templates.seed`; T-120). The two commands keep their names and options and load the
  files; `--update-existing` is `--replace`. Checked against the scripts they replace on an
  empty instance and over seeds 01 and 03: the same records, field for field.
- **Seeds catalogue rewritten and generated without dependencies** (`seeds/SEEDS-CATALOGO.md`,
  `seeds/md2pdf.py`). The catalogue's figures are now the ones each seed creates, counted on its
  dry run: seeds 01 and 02 had approximate counts and listed brands, sensors and rules they do not
  have. It covers all six seeds in the comparison tables and in the commands, the admin accounts
  of seeds 05 and 06, the work-order seeds, the entity schemas of seed 01 and the companion
  document of seed 04. `md2pdf.py` uses only the Python standard library: it writes a
  self-contained `SEEDS-CATALOGO.html` and, when a Chrome, Chromium or Edge is installed (Linux or
  macOS), prints it to `SEEDS-CATALOGO.pdf`. `markdown` and `weasyprint` are no longer needed.
- **`make dev` turns on per-tenant overrides and the form designer**
  (`SEAM_DEVELOPER_TENANT_OVERRIDES=1`, `SEAM_DEVELOPER_FORM_DESIGNER=1`).
- **Developer module pages rebuilt with the application's components** (DEC-30 in
  `docs/DEVELOPER_MODULE_CAMPAIGN.md`; `internal/ui/developer_ui.go`). Overview: four summary tiles that open
  each section, switch cards with an icon and a state badge, both override directories reloadable from there,
  the view-as panel. Translations: the editor opens above the list, with the original on the left and the
  scope, the override and its buttons on the right; the list uses the listing filter bar (search, language,
  and a new "show" filter: overridden, not translated, missing, extra, different formats), striped rows that
  open on click and a numbered pager; each count of the completeness table links to the texts it counts. Form
  definitions: a list with search and a module filter, and a page per form with Definition, Preview, Changes
  (with a count) and Versions as tabs; a preview opens its tab. Designer: a save bar, the fields with icon
  actions and the add-field controls in one panel, the inspector on a grid, the preview under it. Startup
  configuration: the listing table. Messages are the shared alert. 33 new keys in en, es, pt and ja; the
  `.dev-*` rules of `app.css` are rewritten.
- **`syncver.sh` calls the gorepoman binary directly** (`REPOMAN_BIN`, else `repoman` on `PATH`) and
  stops with a clear message and exit 127 when it is missing; `release.sh` no longer ships `repoman/`.
- **gofmt applied** to 12 files in `internal/formengine` and `internal/ui` (alignment and whitespace only).
- **Test fixture user name** in `internal/ui/issues_test.go` changed to a neutral name, so
  `repoman badcode check` passes.

### Closed
- **T-95** (ANSI `LIMIT`/`OFFSET` changed to `TOP`): closed after compile and tests, a check of xolu 0.30.44's
  `TOP`-after-`ORDER BY` behaviour and a parse of the Sulpher `LIMIT` example. See `docs/RESOLVED.md`.

### Removed
- **`developer/translation_filter` form schema** and its two keys per locale: the translations list is
  filtered with the listing filter bar (DEC-30, a departure from DEC-13).
- **`repoman/` directory**: the gorepoman shims, `repoman/VERSION` and the unused `config.py`.

## [0.4.3] — 2026-11-11

Accounting & Admin module (Seam Admin) and form/dashboard fixes.

### Added
- **Accounting & Admin module** (`internal/accounting`, `internal/modules/accounting`,
  `internal/ui/accounting_*.go`; documented in `docs/ACCOUNTING_ADMIN_MODULE.md`):
  opt-in behind `features.enable_accounting` (`SEAM_ENABLE_ACCOUNTING=1`), configurable
  under `accounting` (`SEAM_ACCOUNTING_*` in `internal/config/config.go` and
  `config.example.json`). Covers Account Executives (self-registration, authorization,
  payout profile), collections (platform snapshot sync, pending /
  `collection_in_progress` / collected states, atomic collection and commission accrual,
  optional collected notifications), commission and price-adjustment formula rules (set,
  surcharge, discount via percentage, fixed amount, or guarded read-only OQL queries,
  plus a simulator), settlements (claiming up to 25 commission lines, invoice upload,
  proof upload and payment), and simplified double-entry books (chart groups 1–5,
  balanced manual entries with optional supporting documents, reversals, and sequential
  monthly closes). Includes file-backed document storage (`data/accounting-docs`), xolu
  store, HTTP platform client, and unit/route/class/portal tests.
- **Accounting RBAC roles and configurable duty flags** (`internal/auth/roles.go`,
  `internal/auth/grants.go`): adds `admin_staff`, `accountant`, and `account_executive`
  roles on a separate accounting role ladder (`RoleAtOrBelow`, `roleFamily`), optional
  duty overrides via `AccountingGrants` (`accountant_operate`, `staff_post_entries`,
  `staff_edit_ae_profiles`, `accountant_edit_formulas`), and AE row-level scoping in
  handlers.
- **Accounting translations and styles**: new locale keys across `en`, `es`, `pt`, and
  `ja` (`internal/i18n/locales/*.json`) and utility classes in `web/static/css/app.css`.

### Fixed
- **Dashboard `recent_list` "View all" link** (`internal/dashboardengine/widgets.go`,
  `renderer.go`): `recentListWidget` and `viewAllLink` now translate `dashboard.view_all`
  instead of rendering the raw key.
- **Assets form missing i18n keys** (`internal/i18n/locales/{en,es,pt,ja}.json`,
  `internal/formengine/asset_schema_integration_test.go`): added
  `assets.forms.asset.tab.maintenance` and `assets.forms.asset.section.lifecycle_documents`
  in all four locales.
- **Form engine `required` enforcement for pickers and repeatables**
  (`internal/formengine/schema.go`, `widgets.go`, `repeatable.go`, `web/static/js/app.js`):
  `renderAssetPicker`, `renderIconPicker`, and `renderAnyPicker` forward `required` to
  the search input and `SeamPicker` (`syncValidity`) blocks submission when text is typed
  without selecting an item; required repeatables default `MinItems` to `1`, propagate
  `items.required` to row fields, and mark required repeatable cell labels with `*`.

## [0.4.2] — 2026-10-22

### Changed
- **Document Links** (`document-links` widget, e.g. the Assets form) is now the flat
  repeatable underneath: the same card rows, circled-minus remove button and add button
  as every other repeatable, in place of the red cross, bare inputs and text-link add
  button. Field names (`name_item[]`), the plain text/url cells, the
  `document_link_item.*` placeholders and the "Add link" wording are unchanged, so no
  POST handler is touched; the add label loses its own "+" because the button draws one.
  The hand-coded `DocumentLinksField` on the masters pages (`internal/ui/masters_pages.go`)
  is a separate component and still has the old look.
- Demo tab captions shortened: Components, Repeatables, Repeat Layouts, Custom Buttons,
  Colors, Selectors (and their es, pt, ja equivalents).

### Added
- **Kitchen tabs in the Demo module** (Inputs, Choices, Capture, In Rows) show the widgets the other tabs do not cover. Inputs: masked text and tel, email, url,
  password, date, datetime-local, time, number and decimal with units, textarea, colour,
  checkbox and toggle. Choices: static, searchable, grouped and icon selects, a foreign-key
  select with its preview popover, multi-select, asset-picker, any-picker, icon-picker, and
  conditional visibility (`eq`, `in`, `all_of`, and a boolean). Field and media: two GPS maps
  (one opened on a stored point), EAN-13, EAN-8 and UPC-A barcodes, signature and image.
  Texts in en, es, pt and ja. The foreign-key select reads the tenant's brands, and only for
  a session that has a tenant. The image widget shows its create-mode message there, as the
  page saves nothing. UPC-E is left out until T-107 is fixed; number bounds are T-108. The In Rows tab holds the script-driven widgets inside the rows
  of a repeatable.
- **"Add photos": the icon and the button now open the file chooser.** Dropzone opens it only for
  a click on the drop area itself, so the icon, the label and the hint were kept from taking
  clicks with CSS. The uploader now also forwards any click that lands on them, so the button no
  longer depends on that stylesheet: with neither mechanism the icon, the button and the hint
  are dead while the empty area works, which is what was reported.
- **A wide GPS panel in a repeatable takes the whole row.** "Wide" used to mean the width of the
  field, which in a row is one narrow cell: the map only grew downwards under it. While a wide
  panel is open its cell now takes the whole row (form layout: every grid column; row layout:
  a line of its own) and goes back when it is closed.
- **An empty barcode field takes no room.** An empty `<svg>` still takes the browser's default
  300x150, which showed as a big blank under "Product barcode" in every row.
- **Static files are revalidated on every use.** Files embedded in the binary have no
  modification time, so Seam sent no `Last-Modified`, no `ETag` and no `Cache-Control`, and a
  browser could not tell that a file had changed between two builds served from the same
  address. They now carry a content `ETag` and `Cache-Control: no-cache` (a cheap 304 when
  nothing changed; ranges and errors untouched).
- **Photos in the Assets form: four fixes.** (1) The first time a form was opened on a fresh
  page the uploader said "Upload is unavailable (Dropzone did not load)": the modal re-creates
  the form's scripts and they load in parallel, so Dropzone could arrive after the uploader had
  asked for it. The uploader now waits for it (and SeamModal keeps the order of classic
  scripts). (2) Clicking "Add photos" did not open the file chooser: Dropzone only reacts to a
  click on the drop area itself, and the icon and label I put inside it took the click away;
  dragging worked, which hid it. They no longer take clicks, and Enter/Space work on the area.
  (3) Clicking a thumbnail shows the photo larger in front of the form (Escape or a click closes
  only the photo; the thumbnail has a hand cursor). (4) The Photos section was titled twice, by
  the engine's section title and by the field's label: the new `x-seam-hide-label` keeps a
  field's label for screen readers only, and the photos field uses it (the form designer
  offers it). Found with an end-to-end test that uploads a real photo through the real modal,
  serving Dropzone with a delay as a real network does.
- **xolu is now started with its blob store enabled.** The photo gallery (the Photos field of the
  Assets form) keeps its files in xolu's `/blob`, which is off by default, so every upload failed
  with a red "upload failed" and nothing more (xolu answers `XOLU-BL001`, 501, and Seam turns
  it into a 500). `XOLU_BLOB_ENABLED=true` is now set by `scripts/launch.sh`, by the demo
  bundle's Windows and Unix start scripts (`scripts/build-demo.sh`) and by `docker-compose.yml`;
  DEPLOY.md lists it as a requirement and has a troubleshooting entry. A test fails if any of
  them stops setting it.
- **`scripts/launch.sh` can use a different version of xolu.** `XOLU_VERSION` is `latest` (the
  default: the latest release, or the default branch built from source), `previous` (the previous
  commit on the default branch), or any tag, branch or commit SHA, e.g.
  `XOLU_VERSION=v0.30.23 make launch`. Any value other than `latest` builds from source, so no
  release download is attempted. The launcher also checks that options taking a value were given
  one, and `--help` prints the whole header.
- **The photo uploader did not exist inside the modal.** SeamModal re-creates the `<script>`
  elements of every form it loads, and copied only their text, so `<script type="module"
  src="/static/js/file-upload.js">` became an empty script: the `seam-file-upload` element was
  never defined and the Photos field was blank, in create-and-edit alike. It now copies all
  attributes (SeamModal and SeamModal2). Found by an end-to-end browser test that opens the form
  in the real modal; the earlier tests loaded the form into a bare page.
- **Create (and Save) from another tab did nothing.** A required field inside a tab that is not
  showing cannot take focus, so the browser refuses to submit without a word (`An invalid form
  control with name='name' is not focusable`): pressing Create on the Map & photos tab with the
  name still empty sent no request and showed no message. When the first invalid field is in a
  hidden tab, that tab now opens and the browser's message is shown on the field. This applies
  to every form with tabs.
- **Create and add photos really did the same as Create.** The footer buttons sit outside the
  `<form>` and htmx sends the request from the form, so a value on the button never reached the
  server. The button now sets a hidden `_then` field that belongs to the form, and Create empties
  it. Closing the modal after a record was created (HX-Trigger `seam:record-created`) reloads the
  page behind it, so the list shows the new asset.
- **GPS map panel: under the button, two modes.** The panel opens under the pin button, aligned
  to its right edge (it falls back to the left edge only when that would leave the screen). It
  has a second mode, the full width of the field in the page flow like the icon-picker's grid:
  every panel has an expand button, and `x-seam-panel: "wide"` makes a field start that way
  (the form designer offers the attribute; the Demo's Capture tab has an example).
- **Adding photos in the Assets form is now something you can see.** In edit mode the uploader
  is an "Add photos" action with an icon and a "or drag them here" hint, translated, readable
  on the dark theme (it was a thin dashed strip with fixed English text). In create mode, where
  there is no record to attach photos to, the grey line is a visible notice that says why. The
  create form also has a **Create and add photos** button: it saves the asset and stays in the
  modal on the new asset's edit form, opened on the Map & photos tab (plain Create still goes
  back to the list). To open a given tab, the engine takes `RenderContext.ActiveTab` (the edit
  route reads `?tab=`); it is only honoured when it names one of the form's own tabs.
- **Assets form: a "Map & photos" tab**, after Location, with a `gps_location` field (the GPS
  map dropdown; stored on the asset as `gps_location`, separate from the existing latitude and
  longitude numbers) and a `photos` field (the photo gallery). The gallery needs the owner it
  uploads against: the edit form's render context now sets `EntityType: "asset"`, which it had
  never set, so an image field there would have uploaded with an empty owner. `photos` is
  deliberately not mapped onto the asset row (the gallery keeps its files in /blob and its own
  entity). English, Spanish, Portuguese and Japanese texts.
- **GPS buttons are single icons.** The location button next to the coordinates is one small pin;
  "use my location" inside the map panel is an icon-only button with a tooltip, so the
  coordinates keep as much of the row as possible.
- **The related-asset box of the Assets form did not work.** Its picker is loaded into the page
  by an htmx swap, and the picker's own library (autocomplete) arrives in that fragment as a
  `<script src>` that has not run when the picker starts. The picker gave up silently, so the
  stored asset never showed its name, nothing listed and typing did nothing. It now waits for
  the library (`SeamWidgets.need`, which polls briefly and then loads the script itself,
  because htmx replaces the script elements of a fragment and a listener on the original would
  never fire). Checked in headless Chromium on the real edit form, three runs in a row (the
  failure only showed from the second page load on), in both themes; the server side of the
  box (empty query on focus, typed query, id lookup) has Go tests.
- **Barcode, GPS map and asset-picker fixes.** The barcode is drawn black on white (it was
  invisible on a dark theme), and a wrong code now turns the field red whatever the theme or
  focus state and clears the old drawing, which used to stay on screen. The map pin is an
  inline SVG: Leaflet's default marker asked for
  `/static/vendor/images//static/vendor/images/marker-icon.png` (404), so only its shadow
  showed. The GPS field is an input plus a map button, and the map opens in a dropdown panel
  like the icon-picker's grid, built on first open. The asset-picker lists its first assets as
  soon as its box gets focus, then narrows as you type. The picker in the In Rows tab is an
  any-picker with a Browse dialog.
- **Widgets that need a script now work inside repeatable rows.** The input mask (text and
  tel), barcode, gps, signature and asset-picker each carried an inline script that looked
  its elements up by id. Browsers do not run scripts inserted with `innerHTML`, so those
  widgets only worked in the first, server-rendered row: a row added with the Add button had
  no mask, no map, no barcode preview and no signature pad. The markup is now inert
  (`data-seam-mask`, `data-seam-widget`, and the `data-seam-picker` markup `any-picker`
  already used), and one shared initialiser, `SeamWidgets` in `app.js`, finds each widget
  relative to its own root: at page load, after htmx swaps and in every row
  `SeamRepeatable.add` creates. A repeatable holding any of these, or an icon-picker, uses
  indexed field names, as with any-picker; no embedded schema outside the Demo module had
  such a repeatable. Leaflet and JsBarcode are loaded on first use when the page has none,
  and the GPS map is built when its panel first opens, so it never measures a hidden box. Still
  unsuitable inside a row: `image` (needs a saved record) and `asset-type-select` (reloads
  the whole form). Checked in headless Chromium (`scripts/browser-test-demo.js`, a dormant
  guard), including against deliberately broken copies of `app.js`.
- The form designer's widget catalogue gains `multi-select`, `color`, `gps` and `image`, the
  four engine widgets it did not list.
- **Developer login in the seeds and `make dev`.** All six seeds create `dev@ual.li` /
  `holaquetal` with the `developer` role (seeds 05 and 06 hash it separately, since their
  admins use other passwords), listed in `seeds/SEEDS-CATALOGO.md`. `make dev` runs
  `scripts/launch.sh` with `SEAM_ENABLE_DEVELOPER=1`, `SEAM_ENABLE_DEMO=1` and
  `SEAM_LOCALES_DIR` / `SEAM_SCHEMAS_DIR` pointing at `data/dev/locales` and
  `data/dev/schemas` (override with `DEV_LOCALES_DIR`, `DEV_SCHEMAS_DIR`). The account's
  password is published in the scripts and the catalogue: on any instance others can reach,
  change it or remove it. The demo builder now offers it as the second login of seeds 01 to
  04 (it used to show a team member's address there). `make help` describes `launch-dev` as
  what it does (`dev_mode`: with no session the UI acts as user 1), not as "verbose, CSRF".
- **The Developer module's decisions are flags (DEC-28).** `developer_view_as` (on),
  `developer_tenant_overrides` (off; on in the `development` profile), `developer_instance_sync` (on),
  `developer_change_notice` (on), `developer_protect_own_role` (on), `developer_form_designer` (off; on in
  `development`) and `designer_live_preview` (on), plus the setting `developer_sync_seconds` (default 15).
  Each default is reasoned in `internal/features/catalog.go` and DEPLOY.md, and a test pins them. Behaviour
  change: tenant-scope overrides are no longer implicitly on when xolu is present; set
  `developer_tenant_overrides`. `Flag.Requires` is documented as what it does: only switching a flag on while
  its prerequisite is off is refused.
- **Form designer, field level (DEV-21, DEC-29).** Behind `developer_form_designer`: `/developer/designer`, with a
  palette (a select that shows an icon per widget, using the new option icons), the field list with up, down and
  remove, an inspector generated from a catalogue of every widget and property, a live preview and Save through
  the existing route into either scope. The catalogue (`internal/formengine/designer`) is held to the engine by
  tests, which already found that real schemas use `minLength` and `maxLength`; the document model is lossless
  (every embedded schema round-trips) and any change the engine would not accept is refused.
- **View as a role (DEV-22).** A developer can view the application as a user with a lower
  role would, chosen on the Developer Overview. The role rides in the signed session and the
  request's claims carry it, so every permission check and module gate sees the lower role
  unchanged; the Developer routes check the real role and the module stays in the selector, so
  there is always a way back. A notice on every page names the role and links to Exit. Only a
  developer session can set it, and it is inert unless the Developer module is on. DEC-25.
- **Instances stay in step (DEV-28).** Each instance checks every 15 seconds (when the state is
  in xolu) whether another has changed the switches, the translation overrides or the form
  definitions, and applies the change, including clearing a tenant whose overrides were all
  removed elsewhere. Also fixed: `Manager.Load` never told a tenant that had lost all its state,
  so its overlay switches stayed stale. DEC-27.
- **Developer option in the role selects (DEV-30).** A developer is offered the developer role
  when creating or editing a user; nobody else is. The rule that forced a developer's role to
  stay is replaced by one refusal: a developer cannot remove their own developer role. DEC-26.
- **Tenant-scope overrides, with history (DEV-13, DEV-16).** Translation and form-definition
  overrides can now be stored per tenant in xolu (`i18n_override`, and an append-only
  `schema_override`), loaded at start and applied to the xolu-tenant layers the moment they are
  saved. The editors gained scope tabs ("This tenant" in the database, "All tenants" in files),
  say when a file override hides a tenant's (and the reverse), and the form editor keeps a
  version history with a note per version, removal as a version and Restore. File and database
  overrides share one definition of what is valid (`i18n.CheckOverride`,
  `schemaoverlay.Resolver.CheckDocument`). The tenant is always the session's. New package
  `internal/overrides`: memory and xolu stores behind one conformance suite, the service, 15
  tests of its own plus end-to-end tests of every route. DEC-20 to DEC-24.
- **Developer notice (DEV-10) and tenant-aware chrome (DEV-31).** While a tenant has show
  translation keys on, or a filesystem or database layer switched off, every page of that tenant
  carries a notice naming the switches (developers also get a link to change them); hot loading
  is not announced; with nothing active the layout is unchanged. The navigation and layout now
  follow the tenant's switches like the page body does (`Config.ViewerTenantID`,
  `chromeTranslate`). DEC-18 and DEC-19.
- **Form-definition editor shows a diff (DEV-20).** The text in the editor is compared line by
  line with the built-in definition, for a saved override and for a preview before saving;
  bounded at 2000 lines, with unchanged runs collapsed and a few lines of context kept.
- **Browser pass (T-94, T-100).** In Chromium, on all eight Demo tabs: removing every row of
  each of the 18 unbounded repeatables and then adding works (T-94), nothing throws, and every
  select is a combobox exactly once (79 checks).
- **Fixed (T-102): the eight tests that had failed since before this campaign.** The seven
  Demo template tests asserted that no OQL query reaches beyond `demo_templates`, which stopped
  being true when the page layout began loading the tenant's `general` and `features` settings
  for every page; those two read-only `SELECT`s are now the one named exception
  (`chromeSettingsRead`), and a write to `tenant_settings` or any other entity is still refused.
  `TestSelector_UnknownKey_Returns400` listed `assets` as an unknown selector key, which it
  stopped being when the selector entities landed; it now lists keys that really are unknown
  (and a few hostile ones). No production code changed: in both cases the assertion was the
  stale side. `internal/ui` passes in full.
- **Minimum and maximum rows for a repeatable.** `minItems` and `maxItems` on the repeatable's
  property (the standard JSON Schema keywords; queryfy already honours them, so there is one
  declaration for the form and the validator). The form shows at least `minItems` rows,
  disables each Remove at the minimum and Add at the maximum, and keeps rows that already
  exceed the maximum visible; `SeamRepeatable` enforces the same limits for both kinds of
  repeatable (a forced click on a disabled button does nothing), including through the
  animated remove. Bounds must satisfy 0 <= min <= max <= 1000 and max >= 1 or the schema is
  refused when parsed. The check that counts is the server's: `ValidatePostedBounds` counts
  the rows of flat and indexed repeatables (blank rows excluded); the Demo echo shows it.
  With no bound the output is byte-identical to before.
- **Icons in a select's options.** `x-seam-option-icons` (option value to Material icon name)
  and `x-seam-icon-position` (`left`, the default, or `right`); an option from a
  `SourceResolver` can carry `Option.Icon`. Drawn by the combobox in the closed box and the
  list (a native select cannot show one), as text never markup; names are validated when the
  schema is parsed, dropped when they come from data, and checked again in the browser.
  Search matches option text, never icon names. With no icons the output is unchanged.
  Demo tab "Reordering" shows both features.
- **Developer module in sections, with editors.** Four pages with their own left-menu
  entries (Overview, Translations, Form definitions, Startup configuration), a translation
  editor and a form-definition editor (raw JSON, preview, save, remove), all of whose input
  forms are form-engine schemas (`schemas/developer`). They write only through validated,
  atomic writers; both layers' loaders now ignore dot-files and tolerate files vanishing
  mid-scan, found by concurrent-writer tests. Design decisions DEC-12 to DEC-17 in
  `docs/DEVELOPER_MODULE_CAMPAIGN.md`; recorded limits in `docs/KNOWN_ISSUES.md`.
- **Developer page reworked, and its overrides now take effect.** Each runtime switch is a
  card on a grid with one Enable | Disable control and a Reset link when it was set from the
  panel; the override directories (with a hint when one isn't configured), a per-locale
  translation completeness table (DEV-19) and the startup flags (now one table, without the
  runtime switches) are separate sections, and the stale "(planned)" line for the control
  panel is gone. DEV-32: the 25 production render and parse sites go through
  `resolveSchema`, so a form-definition override replaces the embedded form (the Demo's
  examples are left alone). DEV-31: `translateFunc(r)` uses the tenant's view while the module
  is on, so the per-tenant switches and tenant overrides reach page bodies (the navigation
  chrome still uses the global lookup). 26 new keys in each locale; "Switch on/off" became
  "Enable/Disable". `make launch-dev`'s help no longer claims "verbose, CSRF" (it only sets
  `dev_mode`) and `make user-create` lists the developer role. `DEPLOY.md` documents the flags,
  how to enable and reach the module, and the override directories.
- **Fixed:** `auth.ClaimsFromContext` reported a stored nil `*Claims` as present, so a caller
  that checked only `ok` could dereference nil; it now reports it as absent (regression test in
  `internal/auth`, found by an adversarial test of the tenant lookups).
- **Developer module groundwork, merged from the devtools checkpoint** (built on the pre2
  line, so merged by hunk onto this one; none of the visual-controls work was replaced).
  Opt-in with `SEAM_ENABLE_DEVELOPER=1`, and it refuses to start unless authentication is
  enabled. The `developer` role is admin's permissions plus `developer.access`, ranked above
  admin; only developers can create, edit, reset the password of, or even see developer
  accounts (`auth.CanAssignRole`, `auth.CanManageUser`). A feature-flag registry
  (`internal/features`) holds one catalogue with strict boolean parsing and the precedence
  default, profile, config file, environment variable, `SEAM_FEATURES` (for example
  `developer,demo,-work_orders`); `SEAM_PROFILE=production` refuses development-class flags.
  The runtime control panel switches hot loading of translations and form definitions and
  the filesystem and xolu layers per tenant (`internal/devstate`, `internal/devruntime`,
  audited as `developer_changed`). Translations (`internal/i18n`) and form definitions
  (`internal/schemaoverlay`) gain layered overrides from `SEAM_LOCALES_DIR` and
  `SEAM_SCHEMAS_DIR`, validated before they take effect. 46 keys in each locale file.
  Campaign tracker: `docs/DEVELOPER_MODULE_CAMPAIGN.md`.
- **Registry and visual flags.** `searchable_select`, `reorder_rows`,
  `document_links_legacy` and `compress_static` are entries in that registry with the same
  environment variable names, so `SEAM_FEATURES` and the Developer panel see them;
  `select_search_min` and `row_effects` stay plain settings. The release scripts accept
  hyphenated pre-release names (`0.4.1-pre-devtools1`).
- **No personal name or machine path in the sources.** The seeded admin user is now `HL` /
  `hl@example.com` (same password); `scripts/build-demo.sh` swaps that literal for the login
  you give it, so the two changed together. `SEEDS-CATALOGO.md` and its PDF (regenerated with
  `seeds/md2pdf.py`) list the new login. Docs and code comments say "we" or "the tech team",
  including the historical `RESOLVED.md` and `CHANGELOG.md` entries, reworded in place.
  `go.mod` and `internal/ui/workflow/go.mod` point `replace github.com/ha1tch/xolu` at a
  relative sibling checkout (`../xolu`) instead of a machine-specific absolute path.
  Databases seeded earlier keep the old login until they are re-seeded.
- **UI feature flags** (`features.*` in the config file, or the env var in brackets):
  `searchable_select` (default true) [`SEAM_SEARCHABLE_SELECT`]; `select_search_min` (8;
  0 = default, negative or above 10000 is refused at startup) [`SEAM_SELECT_SEARCH_MIN`];
  `reorder_rows` (true; a kill switch, `x-seam-reorder` stays opt-in per field)
  [`SEAM_REORDER_ROWS`]; `document_links_legacy` (false) [`SEAM_DOCUMENT_LINKS_LEGACY`];
  `row_effects` (`scoped`, i.e. the Demo page only; `all` or `off`) [`SEAM_ROW_EFFECTS`];
  `compress_static` (true) [`SEAM_COMPRESS_STATIC`]. The four booleans are also entries in the flag registry
  (`internal/features`). The engine-side ones live in
  `formengine.Options`, set once at startup (`SetOptions`) because a `FormEngine` is built
  in a dozen places.
- **Gzip for `/static/`** (`internal/server/compress.go`): text-like files (JS, CSS, SVG,
  JSON, ...) for clients that accept it. Range and HEAD requests, non-200 answers, files
  that are already compressed and malformed or oversized `Accept-Encoding` headers are
  served as they were; compressible types always carry `Vary: Accept-Encoding`.
- Adversarial tests for the above: forged `_pos` values, hostile `Accept-Encoding`
  headers (including a 1 MB one), options flipped while forms render and gzip writers
  shared across concurrent requests (both run clean under `-race`), out-of-range
  configuration values, and escaping of translated chrome strings in attributes.
  The select threshold and the three row-effects modes were also checked in Chromium.
- **Searchable select** (`SeamSelect` in `app.js`, `.fe-ss-*` in `app.css`): every
  `select` widget becomes a combobox: search box from 8 options up, option groups,
  keyboard handling (Enter/Space/Arrows open; Arrows, Home, End, Enter, Escape inside),
  and a panel on `<body>` that flips above the trigger near the bottom of the screen.
  The native select stays in the DOM, hidden, and is still what the form submits; input
  and change fire as before, so FK jumps and htmx triggers are unaffected. Rows cloned
  or added to a repeatable are enhanced once, whatever they were cloned from.
  `asset-type-select` stays native. New keys `select.search`, `select.no_results`.
- **Reorderable repeatables** (`x-seam-reorder`, `internal/formengine/repeatable_reorder.go`,
  `SeamReorder` in `app.js`): off by default. Each row gets a drag handle; pointer drag
  (the page scrolls near its edges, Escape puts the row back), and ArrowUp/ArrowDown/Home/End
  on the focused handle. Flat repeatables post in DOM order. Indexed ones also post
  `name[i][_pos]`, written after every move and again at submit; `ParseIndexedRepeatable`
  orders rows by it (index order if any row lacks one) and drops it from the result.
  Works for nested repeatables independently. New key `action.reorder`; Demo tab
  "Reordering" (`schemas/demo/reorder.json`) with its texts in en, es, pt and ja.
- Checked in Chromium through Playwright: select open/filter/choose/Escape/outside click,
  form reset, cloned rows, pointer and keyboard moves, nested moves, dark mode, and the
  posted order parsed back by the server. The harness is not part of the repository.
  The pre-existing T-100 items elsewhere in this release are still unchecked.
- **Row effects** (`SeamRowFx` in `app.js`, `.fe-row-in`/`.fe-row-out` in `app.css`):
  repeatable rows grow in when added and shrink out before they are removed, 150 ms,
  only inside an element marked `data-fe-fx` (the Demo page). Works for flat and indexed
  repeatables alike; off under `prefers-reduced-motion`. Not yet run in a browser (T-100).

## [0.4.1-pre8] — 2026-10-11

Pre-release. Repeatable variants, selectors over more entities, Demo tabs.
**Written without a Go toolchain or a browser: nothing in this release has been
compiled, tested or looked at yet (T-100).**

### Added
- **Repeatable buttons and colours** (`internal/formengine/repeatable_custom.go`):
  `x-seam-add-label`, `x-seam-add-icon`, `x-seam-add-style` (`outline`, `filled`,
  `link`, `icon`), `x-seam-remove-icon`, `x-seam-remove-label`, `x-seam-remove-color`
  and `x-seam-accent`. Colours are a name (`indigo`, `emerald`, `amber`, `rose`, `sky`,
  `violet`, `teal`, `slate`) or `#rgb`/`#rrggbb`; anything else, and any icon name that
  is not lower-case letters, digits and underscores, is ignored. With none of the
  properties set the output is unchanged.
- **Selector sources** for `assets`, `cost_centers`, `locations` and `custodians`
  (`internal/ui/selector_entities.go`, one spec-driven builder), and the matching
  `entity:cost_centers`, `entity:locations` and `entity:custodians` any-picker configs.
- **Demo tabs**: Repeatable layouts, Custom buttons, Custom colours and Selectors, each
  with its own schema (`schemas/demo/{layouts,buttons,colors,selectors}.json`), texts and
  the schema shown as text. The echo page rebuilds the structure with the schema of the
  form that was submitted (hidden `_demo_schema`, whitelisted).
- 26 locale keys in en, es, pt and ja (`demo.*`, `selector.title.*`, `selector.col.*`).
- Tests: `repeatable_custom_test.go`, `demo_tabs_test.go`, `selector_entities_test.go`
  (query and paging, labels, registry, locale coverage, demo schemas' pickers and icons).

### Fixed
- Row-layout repeatables with more than one cell stacked their cells (every widget is
  `w-full` inside a wrapping flex row, so each took a line). Cells now share the line
  (`flex: 1 1 0`); a nested repeatable keeps its own line. This also affects every other
  multi-cell row repeatable, for example Service Configurations on the Asset form.

## [0.4.1-pre7] — 2026-10-11

Pre-release. Seeds.

### Added
- Seeds: the development team (Issue Tracking, Work Orders and AI teams, 12 people) is
  added as users with the `operator` role in all six seeds, in recognition of their
  contribution. They sign in with the same seed password as the other seeded users.

## [0.4.1-pre6] — 2026-10-11

Pre-release. Repeatable layouts, picker buttons, dark mode, list selection.

### Added
- **`x-seam-layout`** on repeatables: `row` (default) or `form` (each row a card with
  labelled cells in a responsive grid).
- **`x-seam-browse`** on any-picker: `button` (default, outlined with an icon), `icon`
  (small, icon-only) or `link` (the old text link).
- **`x-seam-highlight`**: `color` (default), `background` or `none`; the match is
  highlighted again in the asset-picker, built safely with DOM nodes (`SeamHighlight`).
- **List selection criteria** (`docs/LIST_SELECTION.md`): a navigation list's key link
  makes the whole row clickable (`SeamList`); applied to Assets, Sensors and, through
  `ListCellLink`, the masters. The Assets and Sensors "view" buttons are gone.

### Changed
- Repeatable add and remove are the Identity Fields' icon buttons (`add`,
  `remove_circle_outline`), in flat and indexed repeatables.
- The icon-picker's dropdown and Browse grid are never open together.
- Row, button, grid and code-block styles moved from inline styles to `app.css`, with
  theme variables for light and dark.

### Fixed
- Dark mode: edit fields and pickers had no explicit text colour (black on dark); the
  picker input, icons and grid now set theirs.

## [0.4.1-pre5] — 2026-10-10

Pre-release. Demo module updated for the new features.

### Changed
- Demo: "New components" now also shows the icon-picker (with its Browse grid) and a plain
  field with `x-seam-placeholder`; "Repeatable fix" rows show an icon-picker inside the
  repeatable, explicit placeholders, the nested heading and rule, and an explanatory text
  for each. Texts updated in the four languages (added with `scripts/i18n-sync.py`).

## [0.4.1-pre4] — 2026-10-10

Pre-release. Icon picker: browse grid.

### Added
- `icon-picker`: a Browse button opens a scrollable grid of every available icon,
  filtered by the text in the box, with the current icon outlined; the chosen icon stays
  visible beside the box in a larger size. `GET /picker/icons?all=1`.

## [0.4.1-pre3] — 2026-10-09

Pre-release. Repeatable rows with labels, icon search, translation tooling.

### Added
- **Repeatable rows say what each cell is**: placeholder, accessible name and select
  empty option from `x-seam-placeholder`, an optional i18n key
  (`repeatable_item.<repeatable>.<item>`) or the item title. Rows are tinted cards with
  an outlined add button; a nested repeatable is indented under a coloured rule with its
  own heading and a different tint.
- **`x-seam-placeholder`** for text, number, textarea, select, any-picker and icon-picker.
- **`icon-picker` widget** and `GET /picker/icons`: live search of the Material Icons
  names the vendored font can draw, with a glyph preview; works inside repeatable rows.
  Demo: the "New components" tab.
- **`scripts/i18n-sync.py`**: add a key to every locale file in one step (`add`, `new`),
  copy missing keys behind a `[TODO]` marker (`scaffold`), list them (`todo`), and check
  that all locales have the same keys (`check`). `make i18n-check` (also part of
  `make check`) and `make i18n-todo`.

### Fixed
- Users seeds 03 (extinfire) and 04 (vendiberia) now give their users `tenant_id: 1`.

## [0.4.1-pre2] — 2026-10-08

Pre-release. Selector dialog, any-picker widget, indexed repeatables, Demo module.

### Added
- **Selector dialog** (T-96): `GET /selector/{key}`, a registry-driven modal for
  choosing one record, with search, paging and a Choose action per row
  (`internal/ui/selector.go`, `SeamSelector.choose` in `app.js`). The Users source
  lists the caller's tenant's active users whose role is equal to or lower than the
  caller's; the search ignores case and accents. `docs/SELECTOR_SPEC.md`.
- **`any-picker` widget** (T-97) with an optional Browse button (`x-seam-selector`),
  an `entity:users` picker source that applies the same visibility rule to search and
  to the id lookup, and `SeamPicker` in `app.js`, which also works inside repeatable
  rows. `asset-picker` is unchanged.
- **Indexed repeatables**: repeatables that hold a nested repeatable or an any-picker
  (or set `x-seam-naming: "indexed"`) post names such as `blocks[0][lines][1][name]`;
  `ParseIndexedRepeatable` / `ParseIndexedForm` rebuild the structure on the server.
  Rows are added from a `<template>` (`SeamRepeatable` in `app.js`). Plain repeatables
  keep the flat `{field}_{item}[]` names.
- **Demo module** (`/demo`, opt-in with `SEAM_ENABLE_DEMO=1`): one tab for the new
  components, one for the repeatable fix. The repeatable form can be saved as a demo
  record in its own `demo_templates` entity (tenant-scoped, marked `demo`), listed,
  deleted one by one or all at once, without touching any other entity.
- Role ranking helpers (`auth.RoleRank`, `RolesAtOrBelow`, `RoleAtOrBelow`);
  `UserFilter.Roles` and `UserFilter.Search`; `ListRowWithAttrs`.

### Fixed
- Repeatable rows lost their fields when the schema declared the row fields outside
  `"type": "array"` + `items.properties`; documented in `docs/FORM_ENGINE_SPEC.md`.
- The repeatable "+ Add" button now finds its own rows container instead of the first
  container with the same id (nested repeatables repeat the id).
- Repeatable add after removing every row (T-94); `doclink_js.go` renamed to
  `doclink_script.go` (the `_js` suffix made Go exclude it on Linux).
- Tabs did nothing on a full page load (`SeamTabs` was used before the deferred
  `app.js` defined it): the Demo page and the form engine's tab renderer now wait.
- Users search in the picker and the Selector found nobody; it is now done in Go.
- Users listing: `LIMIT`/`OFFSET` replaced with `TOP` (T-95).

## [0.4.0] — 2026-08-17

### T-10 closed — six new formengine widgets, four new x-seam-* extensions, full spec documentation

All of T-10's named scope is now built, tested, live-verified, and documented
— closed in `docs/RESOLVED.md`.

**New widgets** (`internal/formengine/widgets.go`): `decimal` (exposes xolu's
own `format:"decimal"` schema-layer support, `type="text"` deliberately, never
`type="number"`, to avoid float64 round-trip precision loss), `multi-select`
(checkbox group, not native `<select multiple>`), `barcode` (EAN/UPC live SVG
preview via JsBarcode, vendored MIT), `gps` (Leaflet map picker + device
capture, reuses the map page's own already-vendored Leaflet), and `image` (a
photo gallery backed by a new xolu-`/blob`-plus-metadata-entity subsystem —
`internal/ui/media_gallery.go` — fronted by a Lit-wrapped Dropzone.js upload
UI, `web/static/js/file-upload.js`, matching the FSM editor's own established
shell-plus-engine Lit pattern). `signature` (canvas-based capture) shipped
earlier in T-10's own history (2026-08-04) and is now also documented in
`FORM_ENGINE_SPEC.md` for the first time, alongside the other five.

**New extensions**: `x-seam-unit` (static unit-of-measure suffix, applied
uniformly at the `renderFieldRow` layer so it works with any widget), `x-seam-
input-mask` (client-side formatting mask, hand-rolled, no new dependency),
`x-seam-custom-validator` (a formengine-level extensibility point —
`FormEngine.CustomValidators` plus `ValidateCustomFields` — deliberately not
injected into the compiled queryfy schema, since that library's `Custom()`
hook doesn't compose with this package's declarative `jsonschema.FromJSON`
conversion path), `x-seam-erp-code` (inert pass-through for a future
integration), and `x-seam-barcode-format` (EAN13/EAN8/UPC/UPCE symbology
selection for the `barcode` widget).

**Two real bugs found and fixed along the way, not just written and hoped
clean:**
- `ParseSchema` treated every queryfy `ConversionError` as fatal, including
  non-fatal warnings (`IsWarning: true`) — invisible until `format:"decimal"`
  became the first format string this codebase ever used that queryfy
  doesn't natively recognise. Fixed to only fail on genuine errors.
- The `image` widget's gallery backend used slash-delimited xolu blob keys;
  xolu's own keys are flat, so every real upload would have failed. Caught
  by the mocked test suite before reaching a live server, not after.
- A live-verify pass against a real, genuinely fresh xolu tenant then found
  a third: the gallery's own position-lookup OQL query ran before the first
  `Create` of its own metadata entity ever happened, so the very first
  upload for any tenant failed outright (`entity does not exist`). Fixed by
  reusing `asset_source_resolver.go`'s existing `entityNotYetCreated` helper
  — the identical shape T-28 already solved for FK-sourced select fields on
  a fresh instance, not a new mechanism.

**Vendored:** `dropzone@5.9.3` (MIT), `jsbarcode-ean-upc@3.12.3` (MIT) — both
license-verified and byte-identity-confirmed via `npm install` + `diff`, full
`VENDOR.md` entries. Also corrected two pieces of stale `VENDOR.md` content
found in passing: `autoComplete.js` was still documented as "not yet wired,"
though T-19/T-72 wired it before this session began.

**`docs/FORM_ENGINE_SPEC.md` (v0.3 → v0.4):** all nine T-10 items documented
for the first time — this was T-10's own literal scope ("Extend
FORM_ENGINE_SPEC.md..."), not a side task, and had been outstanding even
after the code/tests for all nine were complete. Also corrected in passing:
the `asset-picker` entry (§4.2) and its row in the widget summary table
(§14) still said "reserved for future implementation, renders as `number`
in the MVP" — stale since T-19/T-72 actually implemented it; and §9.2's
"formats queryfy recognises" table was missing `date-time`, `time`, and
`tel`, despite all three being implemented and in use in the example schema
below it.

**Filed, not fixed (separate scope, tracked for later):**
- T-92 — `config.Server.MaxBodySize` is parsed from config but never
  actually enforced anywhere in the server; a dead config value, found while
  the gallery upload endpoint needed its own size ceiling and there was no
  platform-wide one to lean on.
- T-93 — `internal/qrcode`'s actual QR image rendering is a placeholder
  (`generateQRBase64` returns `base64("QR:"+data)`, not a real image). A
  separate proof-of-concept project (`seam-2dcodes`) is a real, working
  candidate fix, pending its own two open blockers (auth model undecided;
  its xolu `/blob` output path never verified against a real server). A
  standalone design-notes document (not part of this codebase) was written
  for whoever picks up the underlying `/label`-primitive question on the
  xolu side.

## [0.3.85] — 2026-08-13

### Session handover document added

`docs/HANDOVER_2026-08-13.md` — a detailed handover for the next
session, written directly rather than auto-generated:
environment/tooling setup, the full identifier-rename campaign (T-40/
T-42) and its hard-won lessons (the identity-map-not-empty-map rule,
mass-substitution collision scoping, the dark-spot sweep method that
found several real, live regressions after the fact), the
`internal/ui/workflow/` module's own genuine, still-open compile
breakage with its three named starting-point errors, the
reverse-direction i18n sweep (T-91) and the shared-component bug it
uncovered, and a full table of all 26 currently open register items
with dependency notes already reviewed for staleness against this
session's own changes.

No code changes. Full narrative in `docs/RESOLVED.md` is not
applicable here — this is a documentation-only release, not a closed
tracking item.

## [0.3.84] — 2026-08-13

### Register audit — T-25 corrected, T-30 closed

Prompted by a direct question: had any of the 27 open register items
been unwittingly resolved by this session's own, unrelated work?
Checked every plausible overlap directly rather than assumed.

**T-25** (workflow-tests dormant guard, stale pins): partially
affected, corrected rather than closed. The dormant-guards table entry
already existed before this session; not something done here. But the
pins themselves are now genuinely current — confirmed directly against
`go.mod` (xolu v0.30.6, minty v0.2.1, queryfy v0.3.2, go 1.26.5), a
real side effect of T-85's `go mod tidy`. The module still doesn't
compile, though — real internal-API breakage remains, tracked
separately as T-86. `docs/KNOWN_ISSUES.md`'s own "Last exercised" text
corrected to stop claiming stale pins that are no longer stale.

**T-30** (masters schema files hardcoding Spanish): closed. Traced
`formengine`'s own title-resolution mechanism first — it already tries
a global `field.{name}` fallback before ever using the schema's raw
text. Two of four affected fields already had this coverage; added the
three genuinely missing keys (`field.code`, `field.iso_code`,
`field.symbol`) across all four locales, after confirming zero
collision risk against every schema in the repository. A real
correction to an initial assumption, caught only by testing live: the
new keys don't fix a live bug, because the actual masters forms
already had their own, separately pre-existing, correctly-translated
keys — the schema files this item names were never wired into any live
route at all, exactly as the item's own original body already said.
`TestMastersFormParity`'s four tests, the only real consumer of this
dormant path, confirmed still passing. Full test suite clean.

Full narrative in `docs/RESOLVED.md`.

## [0.3.83] — 2026-08-13

### Closed — T-91: reverse-direction i18n sweep, mirroring T-40's own law

A final sweep after T-40 closed, looking for the opposite violation:
hardcoded English user-facing text bypassing the i18n system, rather
than Spanish internal identifiers. Found `assets.go` as the sole
outlier across the entire `internal/ui` tree — nine strings (page
title, empty state, search placeholder, five filter labels) hardcoded
directly in English while every sibling field on the same struct
literals already correctly called `t(...)`. Most needed i18n keys
already existed; three new ones added across all four locales
(en/es/ja/pt).

A deeper bug found live, not by inspection: the shared
`ListPaginationBar` component hardcoded its entire count-line template
in Spanish ("Mostrando %d de %d %s") regardless of locale — an
English-locale page would have read "Mostrando 16 de 16 Assets" even
after the label fix alone. Added a new parameterized key
(`pagination.showing`), gave the component a translator parameter,
fixed both its real callers — and caught a second, independent miss
in the same pass (`events.go`'s own hardcoded `Label: "events"`).

One new regression test added — no prior test existed for this page's
own rendering at all. Sanity-checked twice, both fixes independently.
Full live verification in both locales through the real settings
form, not just the unit test: Spanish renders fully coherently
throughout, English correctly shows "Showing 16 of 16 Assets" with no
Spanish leaking through. Zero server errors. Full test suite clean.

Full narrative in `docs/RESOLVED.md`.

## [0.3.82] — 2026-08-13

### Closed — T-40: the Spanish-identifier rename campaign is complete

The final piece: the shared `DocumentLink` item struct (Name/Type,
used across `LinksCicloVida`/`LinksComodato`/`LinksDocumentos`) and
`service_configs` items (measure/threshold/alert_id), both named in
this item's own original 2026-08-02 audit. Retires a real, documented
2026-07-28 bug — the submitted "threshold" field name and its
persisted xolu key genuinely differing — by making that class of
mistake structurally impossible now that both sides are the same
string.

A genuine trap correctly avoided: the unrelated `alerts` runtime table
shares the literal string "alerta_id" with `service_configs`' own
now-renamed field, for a completely different purpose. Confirmed the
distinction directly before touching anything.

Five real pre-existing tests fixed and sanity-checked. Seed scripts
updated with function-boundary scoping, since "alerta_id" is genuinely
ambiguous at file scope. Full live verification included a real form
submission — updating a threshold value end to end, form to storage —
not just compile or unit-test coverage. Zero server errors. Full test
suite clean.

**This closes T-40 itself.** Every piece of its own original scope —
all eight masters entities and all seven of Asset's own scalar-field
blocks plus its shared nested structures — is now complete. Full
narrative in `docs/RESOLVED.md`.

## [0.3.81] — 2026-08-11

### T-40 Asset rename — Block F (Service/IoT) done, completing all scalar-field blocks

Four fields renamed: `TempMin`/`TempMax`/`Refrigerant`/
`VoltageConsumption`. The established English names were confirmed
directly from the bridge rather than assumed — `temp_min`/`temp_max`,
not the `temp_range_min`/`temp_range_max` this item's own original
scope had guessed.

A whole-tree sweep found zero stale references anywhere — these four
fields are consumed only through the dynamic form's own translation
path. Live-verified against `seed-02-frigocold.py`'s real cold-chain
data specifically, since it has by far the richest data for this
block (19 assets, real refrigerant and temperature-range values across
multiple cooling-chamber assets) — confirmed both in xolu directly and
on the real edit form. Zero server errors. Full test suite clean.

This completes all seven of `AssetDetail`'s own scalar-field blocks.
One piece of T-40's original scope remains: the shared `DocumentLink`
item struct and `service_configs` items. Full narrative in
`docs/RESOLVED.md`.

## [0.3.80] — 2026-08-11

### T-40 Asset rename — Block G (Finance) done

Five fields renamed: `PurchasePrice`/`CurrencyID`/`CurrentPrice`/
`CostCenterID`/`WarrantyEnd`, matching the already-established
Currency/CostCenter precedent. None had enum dependencies.

This block also completes the `fin_garantia` half of the dashboard
urgency-list widget fix T-85 deliberately left pending — the widget
config and its own explanatory comment both updated now that
`WarrantyEnd` has landed.

Two more delete-protection guards fixed (Currency, Cost Center),
same class as every prior block. Sanity-checked properly even though
the full test suite passed clean on the first run this time —
confirmed directly that the existing round-trip test does *not* catch
a wrong xolu-side column name (only live verification does for this
class of mistake).

All six seed scripts updated (235 occurrences). Full live
verification: the edit form correctly pre-selects Currency and Cost
Center and pre-fills both price/date fields from real seeded data;
the dashboard's urgency-list widget correctly shows real assets,
confirming the warranty_end fix works live. Zero server errors. Full
test suite clean.

Two blocks remain: Service/IoT, plus the shared
`DocumentLink`/`service_configs` structs. Full narrative in
`docs/RESOLVED.md`.

## [0.3.79] — 2026-08-11

### T-40 Asset rename — Block D (Custody) done

Three fields renamed: `CustodianID`/`TenureType`/`LoanExpiry`, plus
`AssetRow.CustodioNombre`→`CustodianName`. `tenure_type`'s enum
dependency checked before touching the bridge.

Also fixed four leftover Spanish fields in `AlertDetailData`
(`AssetCustodian`/`AlertConfig`/`AlertType`/`AlertCriticality`) — a
miss from Alerta's own earlier, separate rename, folded in here since
one sat in the same struct as a field this block was already renaming.

Swept for stale references and fixed three real ones (a
delete-protection guard, a list-row read, an alert-detail resolution
block). Five stale tests fixed, sanity-checked properly. All six seed
scripts updated (146 occurrences).

Full live verification: assets list shows the resolved custodian
name, the edit form correctly pre-selects Custodian and Tenure Type
(translated from stored "Propiedad" to "Owned"), and the alert detail
page correctly renders both the resolved custodian name and the
criticality-colored config block — confirming the AlertDetailData fix
works live, not just compiles. Zero server errors. Full test suite
clean.

Three blocks remain: Finance, Service/IoT, plus the shared
`DocumentLink`/`service_configs` structs. Full narrative in
`docs/RESOLVED.md`.

## [0.3.78] — 2026-08-11

### Closed — T-85: dark-spot audit pass, plus a real regression from Block E fixed

A deliberate audit pass across T-40's completed blocks before
continuing further, prompted by this session's own pattern of finding
regressions in files earlier renames never touched. Found and fixed a
real, live one: the assets dashboard's urgency-list widget had been
silently broken since Block E's own release (v0.3.75, three releases
ago) — a hardcoded JSON config still referenced `fecha_expiracion`
after that column was renamed to `expiration_date`, so the widget's
own query was quietly failing in every deployment since. Fixed, along
with a now-stale comment nearby.

Also discovered `internal/ui/workflow/` — six genuine end-to-end,
cross-handler workflow tests, run against a real embedded xolu — is a
separate Go module that has been completely unbuildable (a stale
`replace` directive pointing at a path that doesn't exist), invisible
to every test run this session. Repointed it to build cleanly and
fixed its one directly-relevant stale fixture. Its own deeper API
staleness, unrelated to this rename, is tracked separately as T-86
(effort estimation for a full repair, not yet performed).

Full narrative in `docs/RESOLVED.md`.

## [0.3.77] — 2026-08-11

### T-40 Asset rename — Block C (Location) done

Three fields renamed: `LocationID`/`RegionID`/`ParentID`
(`RegionID` was already English). `parent_id` given particular
attention given its live use in the asset-picker widget (T-72) —
confirmed zero remaining `id_padre` references anywhere before
touching the bridge.

Two more live regressions found and fixed along the way, neither part
of this block's own scope: a delete-protection guard missed during
Block B (T-83) — deleting a Model never actually checked whether any
asset referenced it — and a Model summary card handler, in a file
untouched by either T-80 or this session's other passes, still reading
columns that stopped existing once earlier renames shipped. Both
caught by deliberately sweeping for the same class of miss after
finding the first instance.

A separate, out-of-scope finding recorded rather than fixed: the
`parent_id` field's own schema title is hardcoded Spanish ("ID Padre")
on an otherwise-English page — a display-text i18n gap, not an
internal identifier, belongs with formengine's own i18n work.

Full live verification: correct Location pre-selection on the edit
form, the asset-picker search still working post-rename, a real
`parent_id` assignment submitted through the actual form and confirmed
persisted under the correct column. Zero server errors. Full test
suite clean.

Four blocks remain: Custody, Finance, Service/IoT, plus the shared
`DocumentLink`/`service_configs` structs. Full narrative in
`docs/RESOLVED.md`.

## [0.3.76] — 2026-08-11

### T-40 Asset rename — Block B (Characterisation) done

Six fields renamed: `BrandID`/`ModelID`/`Serial1`/`Serial2`/`Lot`/
`Criticality`, across `AssetDetail` and a second struct (`AssetRow`)
found to have its own separate copy. Bridge entries converted to
identity mappings, `criticality`'s enum dependency checked first.
Eight stale test assertions fixed across two packages, sanity-checked
properly.

A real, live regression found and fixed along the way, not part of
this block's own scope: the asset edit form's Model dropdown has been
rendering every option with an empty label in every deployment since
T-80 shipped (v0.3.73) — a stale query in a file that release's own
work never touched. Checked the entire query map for the same class
of miss; every other entry was already correct. Fixed, and caught
independently by a real pre-existing test once the fix ran against
it — that test would have caught T-80's regression at the time, had
it existed then.

Seed scripts updated across all six files (274 occurrences). Full
live verification: correct criticality badges and filtering on the
assets list, correct Brand/Model/Criticality/Serial pre-selection on
the edit form (Criticality correctly translated from stored "Alta" to
"High"). Zero server errors. Full test suite clean.

Five blocks remain: Location, Custody, Finance, Service/IoT, plus the
shared `DocumentLink`/`service_configs` structs. Full narrative in
`docs/RESOLVED.md`.

## [0.3.75] — 2026-08-11

### T-40 Asset rename resumed — Block E (Lifecycle) done

Both collision risks T-40's own paused audit named (Modelo,
PropDefinition/PropAssociation) are now cleared, so execution resumed
on Asset itself — the big one, 32 fields, the central entity every
other entity's foreign keys point at. Working one logical block at a
time rather than all fields in one pass.

Block E (Lifecycle), seven fields, done: `PurchaseDate`/
`ExpectedArrivalDate`/`ArrivalDate`/`ObsolescenceDate`/
`ExpirationDate`/`DecommissionDate`/`DecommissionReason`. Applied the
identity-map-not-empty lesson correctly from the start this time —
checked `assetEnumValueToXolu` for `decommission_reason` specifically
before touching the bridge, rather than repeating the mistake T-42's
own closure had to learn the hard way.

All six seed scripts updated (110 occurrences), confirmed safe as a
whole-file substitution since these key names are exclusive to Asset's
own seeding. One existing test fixed
(`TestAssetEnumValues_RoundTrip`), still passing correctly for all
five `decommission_reason` enum cases afterward. Full live
verification: fresh seed (16 assets), real data confirmed directly in
xolu, real authenticated page load, zero server errors.

A real, separate finding along the way: these seven fields are read
from xolu but never actually rendered anywhere in the UI — not
something this work broke or needed to fix, just worth recording.

Six blocks remain: Characterisation, Location, Custody, Finance,
Service/IoT, plus the shared `DocumentLink`/`service_configs`
structs. Full narrative in `docs/RESOLVED.md`.

## [0.3.74] — 2026-08-11

### Closed — T-42 fully: PropDefinition/PropAssociation renamed, T-42 itself closed

Completing T-42 (Modelo half already shipped as T-80): PropDefRow,
PropDefFormData, PropAssocRow, PropAssocFormData renamed to English,
plus a fourth struct (PropValueEntry) found only by letting the build
find it — dead code, but renamed for consistency rather than left
stale. xolu's own `prop_definitions`/`prop_associations` columns
renamed; all six seed scripts updated with enum values deliberately
preserved.

A real bug caught by a new test before any live testing: emptying
`propDefFieldToXolu`/`propAssocFieldToXolu` to nothing (rather than an
explicit identity map) silently broke `data_type`/`requirement`'s own
enum-value translation — the exact same failure mode this project's
own Alerta entity (T-40) already had documented, and this work
repeated it before catching it independently. Fixed to match Alerta's
established correct pattern. Both new tests sanity-checked: reverted,
confirmed genuine failure with the exact "numeric" vs "numerico"
mismatch seen live, restored.

Full live verification: fresh seed, real UI, both list pages showing
correct data, a new property and a new association each created
through the real forms and confirmed correctly persisted — English
columns, correctly-translated Spanish enum values. Zero server errors.

T-42 itself closed. Both known collision risks for T-40's own Asset
rename (Modelo, PropDefinition/PropAssociation) are now cleared — a
cross-reference on T-40 records the repeated-mistake lesson directly,
so Asset's own eventual rename checks it first rather than risking a
third repetition.

Full narrative in `docs/RESOLVED.md`.

## [0.3.73] — 2026-08-11

### T-42, Modelo half — Spanish internal identifiers renamed to English

Continuing T-40/T-42's own established sequencing: Modelo first,
since it was the collision risk blocking Asset's own rename.
`ModeloRow`/`ModeloFormData`'s Go fields, xolu's own `models` table
columns, and all six seed scripts renamed (`nombre`→`name`,
`marca_id`→`brand_id`). `modeloFieldToXolu` emptied to identity
pass-through.

Found and fixed a genuine, pre-existing bug as a direct side effect:
the models list page's brand-name lookup queried `brands.nombre`, a
column that stopped existing once brands' own rename completed —
confirmed directly against the live store before fixing. The "Brand"
column had likely been showing blank for some time.

One real miss caught only by running the seed scripts live, not by
static inspection: a single-quoted `item['nombre']` inside an f-string
in all six files, invisible to the double-quote-scoped substitution.
Found because a fresh seed run crashed; fixed, then verified with a
clean re-run.

Full live verification: fresh seed to real UI — list page shows
correct model and brand names, edit form pre-fills correctly, a new
model created through the real form persists under the correct
English columns. Zero server errors. Full test suite clean.

PropDefinition/PropAssociation — the other half of T-42 — remain.
Full narrative in `docs/RESOLVED.md`'s own T-80 entry.

## [0.3.72] — 2026-08-11

### T-40 cross-referenced — a framing correction on T-79's own retraction

No code change. T-79 (xolu v0.30.6 upgrade, criticality-persistence
retraction) correctly concluded there was no data-loss bug, but
described `criticidad` as "the real xolu column" and "correct" — the
opposite of this project's own standing rule that internal identifiers
should be English. `criticidad` is T-40's own remaining debt, not the
standard; `criticality` is what the column should be named once
Asset's rename resumes. T-79 stays as written (closed records are
append-only); the correction is recorded against T-40 instead, where
it belongs.

Full narrative in `docs/TRACKING.md`'s own T-40 entry (still open) — cross-reference only.

## [0.3.71] — 2026-08-11

### xolu upgraded to v0.30.6 — and a self-flagged bug retracted, honestly

Upgraded from v0.28.0. `pkg/client`'s core methods
(Create/Get/Patch/Delete/OQL) unchanged; the only diffs are purely
additive (new `ts` client, new `bal`/`cal` list methods). Full xolu
suite clean (41 packages), full seam-ui suite clean.

The actual reason for this upgrade: re-testing a "criticality doesn't
persist" issue noted during earlier live verification. It didn't
reproduce as a xolu bug — direct investigation found there was never a
bug at all. The value was correctly saved the entire time, under its
real xolu column name, `criticidad` (part of the already-tracked
Spanish-identifier bridge, T-40). The earlier finding used the English
field name in its own verification query — the same mistake pattern as
an earlier `parent_id`/`id_padre` case this session. Confirmed
directly: raw record inspection, a corrected OQL query, and the actual
field-mapping code all agree. Nothing needed reporting to the xolu
team.

Full narrative in `docs/RESOLVED.md`.

## [0.3.70] — 2026-08-06

### Closed — T-74: left sidebar synchronized with modules

Each module now owns its own sidebar items (`modules.SidebarItem`,
declared at registration, matching the existing `MountRoutes`-per-module
pattern) instead of one static, 19-item list shown regardless of which
module the request is actually in. Resolution: whichever module's own
`Path` is the longest matching prefix of the current URL.

Assets confirmed genuinely unchanged, not just visually similar —
captured the real sidebar HTML before and after, byte-for-byte
identical once the (correctly, intentionally removed) Administration/
Settings section is excised from the old capture. Work Orders and
Issues — previously thin, hard to reach — each got two real
navigation items (list + create). Documents deliberately gets none:
one route, nothing to navigate to.

Two real bugs found and fixed during implementation: `DefaultConfig()`
silently omitted `VisibleModules`, making sidebar tests pass for the
wrong reason; and every newly-populated module (Administration,
Settings, Work Orders, Issues) initially rendered an empty sidebar,
because the existing `navItems()` rendering function only attaches an
item to a group that comes after a Section header — true by
construction for the old static list, not automatic for a module
declaring just 1–2 items. Found by testing each module's own page
directly, not assumed to generalize.

One test bug also caught: the first version of the longest-prefix-wins
test passed even with that logic removed, since registration order
alone produced the right answer by coincidence. Rewritten to isolate
the rule being tested, confirmed it genuinely fails against broken
logic first.

4 new unit tests, all six modules live-verified individually (desktop
and mobile), zero server errors. One accepted, documented cosmetic
quirk: `/workorders/new` highlights both "All Work Orders" and "New
Work Order" simultaneously (one URL is a genuine prefix of the other)
— not fixed, doesn't touch Assets' verified behavior.

Full narrative in `docs/RESOLVED.md`.

## [0.3.69] — 2026-08-06

### Closed — T-78: register.py's own closure-narrative bug, found and repaired

`register.py`'s span-boundary parser treated any `## Heading` line as
a theme-group boundary — correct for a genuine theme header, wrong for
a sub-section inside a single item's own closure narrative, which is
exactly the convention this session had been using since T-75. Every
close operation using it silently truncated both the `RESOLVED.md`
copy and the `TRACKING.md` removal at the first such sub-header,
orphaning the remainder — invisible to `register.py check`, which only
validates open items.

Parser fixed with a lookahead distinguishing a genuine theme header
(always immediately followed by a `### ` item header, confirmed
against every real boundary in the file) from a body sub-heading.
Verified both directions with isolated synthetic tests before touching
anything real, then confirmed live against a real closure using the
same convention (this entry's own source item, T-78).

T-75, T-76, and T-77's `RESOLVED.md` entries — all missing their own
closure's back half — reconstructed from the orphaned text still
sitting in `TRACKING.md`, which contained exactly what was missing.
Register consistent at 29→30→29 open items throughout (nothing lost).

Also: T-19 (asset-picker) and T-74 (sidebar sync) register accuracy
corrected in this same pass — see `docs/RESOLVED.md` and
`docs/SIDEBAR_SYNC_PLAN_2026-08-06.md`.

Full narrative in `docs/RESOLVED.md`.

## [0.3.68] — 2026-08-06

### T-19 register accuracy corrected

Found while grounding a "what's next" answer in the actual register
rather than memory: T-19 (the asset-picker widget) had never been
updated when its work was completed under T-72/T-76 — it sat at ☐
describing already-finished work as unstarted. Corrected: status moved
to ◐, three of five original items marked done with their real
closure references, two genuinely still-open items preserved
(self-reference exclusion, the separate listing-search-bar question).

No code changes.

## [0.3.67] — 2026-08-06

### Issues/Work Orders duplication resolved — adapted where a primitive already fit, extracted where it genuinely didn't

Badges: adapted to `Badge(text, variant)`, an existing primitive
almost duplicated by mistake — caught by the compiler
(`StatusBadge redeclared`), not by checking first. Both modules'
status badges are now thin mappings to existing variant keywords, no
markup of their own.

Lists: checked whether the existing `ListRow`/`ListCellRich`/
`ListActions` table-based system fit before assuming it didn't — it's
built for genuinely multi-column, per-action-icon lists (Assets,
Users, masters). Work Orders and Issues are a different shape
entirely — one clickable row is the whole navigation, no separate
action icons. Extracted `SimpleListItem`/`SimpleListCard` as a
genuinely new, small primitive instead, documented against the
existing system's own comments so the distinction is recorded where
it'll actually be read.

Live-verified against a real server, including one honest correction
along the way: a "missing" open-issue badge turned out to be my own
verification query checking for the wrong color name (`Badge`'s
"warning" variant renders yellow, not amber) — the render was correct,
my check wasn't specific enough. Confirmed properly afterward. Zero
server errors throughout.

Full narrative in `docs/RESOLVED.md`.

## [0.3.66] — 2026-08-06

### Issue Tracker module — simple, always-on, the asset-picker's third real consumer

A new module: create an issue (title, description, optional linked
asset via the asset-picker widget), view it, toggle open/closed. Not
gated behind a feature flag like Work Orders was — nothing here needed
that framing. Deliberately not FSM-based: a two-state status toggle
doesn't need xolu's own FSM primitives, and using them here would be
over-engineering a "simple" ask.

The asset-picker's third real consumer (parent_id, sensor binding, now
this) — zero widget code changes needed, directly confirming what the
two-picker-per-page test had only argued for previously.

5 new tests, including the full discipline on the one that actually
matters — required-title validation verified by temporarily disabling
it and confirming the test genuinely failed.

Live-verified end to end: created a real issue with a real asset
selected via the picker, confirmed persistence, resolved asset name on
the detail page, toggled status both directions, confirmed the list
page reflects it. Zero server errors.

Full narrative in `docs/RESOLVED.md`.

## [0.3.65] — 2026-08-05

### Closed — T-75: audited for hardcoded REST requests, fixed the one real finding

Per the tech team's direct instruction following the multi-instance xolu
discussion: searched the whole codebase for anything bypassing a
resolvable client abstraction. Found exactly one real violation —
`fetchRuntimeState`'s bare `http.Get`, feeding the asset detail page's
runtime FSM state, with no timeout and no named boundary. Confirmed
`internal/webhook`'s own raw HTTP usage is correct as-is (a webhook's
destination is inherently customer-arbitrary, not something to
resolve).

Fixed with a small, deliberately minimal `seamAPIClient` type — a real
5-second timeout, and a single named place to change if `seam-api`
ever needs the same per-account resolution the xolu provisioning plan
describes. No caching, no resolver logic — building either now would
mean guessing at a shape `nolu` hasn't settled yet.

6 new tests, including a dedicated timeout-enforcement test verified
with the full discipline — temporarily removed the timeout, confirmed
the test genuinely failed (waited the full 2s for a slow server),
restored it.

Also corrected `USABILITY_PLAN_2026-08-05-corrected.md`'s own
Recommendation 1, which had proposed building this fresh — it already
existed (tracked as `S-11`), just dormant behind an unset config value.

Full narrative in `docs/RESOLVED.md`.

## [0.3.64] — 2026-08-05

### Closed — T-31: urlEncode() now correctly percent-encodes non-ASCII

Replaced the hand-rolled encoder (a fixed set of ASCII characters,
everything else passed through raw) with `net/url.QueryEscape`.
Real impact: every translated login-error redirect in Japanese, or
Spanish/Portuguese with an accented character, carried raw UTF-8
straight into a URL query string — three of this project's four
supported locales, on one of the most frequently hit paths.

Found and fixed an adjacent gap in the same pass: none of the seven
login-failure redirects encoded the submitted email at all, even
under the old broken function — a real query-string injection surface.

Verified with the full discipline: reverted, confirmed all three
locales genuinely failed with raw characters visible in the test
output, restored. Live-verified against a real server — a real
redirect showed `Contraseña` correctly encoded as `Contrase%C3%B1a`.

### T-70 update — apikey auth fix verified working, core question still open

Started a real xolu server with apikey auth actually enabled, and
confirmed empirically: the old `Bearer` format genuinely gets
rejected (401), the fixed `ApiKey` format succeeds (200), and the
real xolu Go client — configured exactly the way this project
configures it — authenticates correctly. Searched the repository for
any evidence this credential path was ever actually turned on: none
found, consistent with every server started this session. That's
real signal, not proof — the actual question, whether this was ever
configured in a real deployment, still requires the tech team's own answer
and isn't resolved by any of this verification.

Full narratives in `docs/RESOLVED.md`/`docs/TRACKING.md`.

## [0.3.63] — 2026-08-05

### Asset-picker widget finished (T-19/T-72) and used for real sensor-asset binding

The real "asset-picker" widget, replacing the MVP number-input stopgap
— a generic, live search-and-select widget extending the existing
`x-seam-source: entity:<name>` convention. Backend search endpoint
deliberately never trusts client-supplied field names for query
construction, following T-63's lesson directly. Widget resolves the
current value's label entirely client-side, never interpolating it
into inline script, following T-63/T-64's lesson.

Used to build real sensor-asset binding — found the "Bind"/"Unbind"
buttons already existed in the UI, completely non-functional (no
`onclick`, no form, no href). Both now work: bind via a real form
using the new picker, unbind via a real CSRF-protected POST.

Live-verified end to end, twice: the picker on a real asset's
`parent_id`, and the full sensor bind/unbind cycle. Zero server
errors either time.

Before releasing, checked directly (prompted by a real observation
during live verification): the widget's own code was confirmed to
neither introduce nor entrench any of T-40's already-tracked Spanish
identifier debt — grepped directly, zero references to `id_padre` or
any other still-Spanish Asset field. The widget only ever touches
`parent_id` (already-English, correctly bridged) and `name`/`status`
(confirmed already-identity-mapped on both sides). Safe to ship
without waiting on T-40's eventual Asset rename.

### xolu v0.28.0 synced

The xolu team delivered on their stated follow-up: a guard-syntax
warning (double-quoted string literals in guard/set expressions now
surface in `analysis.warnings`) and strict-identifier validation on
transition payload keys (`XOLU-FSM014`). Verified our own Work Orders
payload construction directly, as invited — already fully compliant.
Zero `pkg/client` changes in this release (confirmed by diff), the
safest of the three xolu upgrades this session.

Full narratives in `docs/RESOLVED.md` (T-72, T-73).

## [0.3.62] — 2026-08-04

### Closed — T-09: flaky checksum test fixed

`TestGenerator_ValidateChecksum` corrupted a code by appending a fixed
literal `"X"` — legitimately the real last character in ~1/34 of
generated codes, making `corrupted == code` and the test fail on a
code `Validate()` correctly accepts. Fixed by rotating the actual last
character to the next one in the checksum alphabet, guaranteeing a
genuinely different character every run.

Verified with the full discipline: 1000 consecutive runs of the fix,
zero failures. Reverted to the original version for a sanity check —
genuinely reproduced the flakiness at a statistically consistent rate
(2/100 sub-runs failed, matching the original description almost
exactly), then restored the fix.

Full narrative in `docs/RESOLVED.md`.

## [0.3.61] — 2026-08-04

### xolu v0.27.1 synced — pure maintenance, no new scope

Per the tech team's explicit instruction: review and sync only. `CreateMachineDef`
existed in xolu since v0.26.0, not newly delivered — our own letter had
checked v0.23.0 and v0.25.0 specifically, which is why it looked missing;
the xolu team corrected this themselves. v0.27.1's actual change: both
`CreateMachineDef` and `ReplaceMachineDef` now reject an empty `Name`/
`Initial` client-side, adopting the one genuine improvement from our own
original patch. Purely additive — diffed against the outgoing v0.27.0
directly, no removed lines, no signature changes.

All 16 client methods verified signature-identical before swapping.
Zero code changes required beyond two comment corrections. Full xolu
suite clean (41 packages, including 4 new tests for the validation
addition), full seam-ui suite clean (24 packages), and the entire Work
Orders lifecycle re-verified live end to end against a real running
v0.27.1 server — zero server errors throughout.

T-70 (apikey auth) untouched, still awaiting the tech team's own confirmation.
No new tasks or features picked up.

Full narrative in `docs/RESOLVED.md` (T-71).

## [0.3.60] — 2026-08-04

### xolu v0.27.0 delivered (T-69 closed) and a real tooling bug found and fixed along the way

The xolu team delivered `CreateMachineDef` — plus more than requested:
`ReplaceMachineDef`, `DeleteMachineDef`, `ValidateMachineDef`, a
complete, properly-typed FSM definition surface. Swapped in and
verified the same way as the previous upgrade: all 12 client methods
seam-ui uses confirmed signature-identical before swapping, one
mechanical change needed in `workorders.go`, full xolu suite clean (41
packages), full seam-ui suite clean (24 packages), and the entire Work
Orders lifecycle re-verified live end to end against the real v0.27.0
server. T-69 closed.

**A separate, serious finding surfaced while reading their changelog**:
their own T-160 entry states the client's apikey auth mode has been
completely non-functional — silently unauthenticated on every request
regardless of key correctness. Confirmed this is a real, documented,
wired-through configuration path in Seam AMS (`SEAM_XOLU_API_KEY`), not
theoretical. Filed as **T-70**, P1, requiring the tech team's direct
confirmation of whether this credential path was ever actually
configured in a real deployment — not something determinable from the
repository alone.

**Filing T-70 itself surfaced a real bug in `register.py`**: `next_id()`
scanned `RESOLVED.md`'s entire text for anything shaped like `T-\d+`,
including inside a closure entry's own free-form prose — and T-69's own
closure narrative quotes xolu's changelog verbatim, including *their*
internal "T-160"/"T-161". The scanner mistook xolu's own numbering for
ours, nearly filing the new item as "T-161" instead of "T-70". Caught
before it did lasting damage (corrected by hand, `register.py check`
confirmed consistent), then fixed properly: the `RESOLVED.md` scan is
now anchored to the closure header's structured position, matching the
same standard `TRACKING.md`'s own parsing already used. Verified three
ways — a synthetic reproduction of the exact bug, a direct run against
this project's own real files (which contain the exact triggering
text), and an end-to-end pass through the real CLI. An independent,
checksummed checkpoint of the fixed `repoman/` was packaged and
delivered separately, per the tech team's request.

## [0.3.59] — 2026-08-04

### Letter to the xolu team: docs/XOLU_CLIENT_FEATURE_REQUEST.md

Requests the xolu team review and land upstream a `CreateMachineDef`
method on `pkg/client` — the one genuinely missing client-side piece
found while building T-67/T-68's Work Orders feature. Includes the
local, provisional implementation as a reference (not a mandate), the
exact request/response shapes verified against a real running v0.25.0
server, and a secondary note about a guard-syntax footgun (double-quoted
string literals in a guard silently match nothing) that cost real
debugging time.

Tracked as T-69, left open — depends entirely on the xolu team's own
delivery. Once they ship, the local patch on the sibling checkout gets
replaced.

## [0.3.58] — 2026-08-04

### Work Orders: from proof-of-concept to a real, kept playground module

Per the tech team's direct request following T-67's standalone proof: "Integrate
Work Orders and keep it. It's going to be used as the playground for the
Work Orders team." Deliberately minimal, deliberately not T-11's own
scope — one FSM (draft → in_progress → completed), one guard (a
technician signature required to complete), reusing the signature
widget built earlier the same session.

Feature-flagged off by default (`EnableWorkOrders`/`SEAM_ENABLE_WORK_ORDERS`)
— a real bug caught before shipping: the module's nav visibility wasn't
actually checking the flag, which would have shown it in the selector
even when disabled. Fixed to check directly.

Live-verified end to end through the real running application: created
a work order → FSM definition auto-bootstrapped on first use → Start →
attempted Complete with an empty signature (correctly rejected, shown
on the page, no 500) → completed with a real signature → durable
state confirmed via a fresh query → list page shows it correctly. Zero
server errors throughout.

Full seam-ui suite clean (24 packages). Full narrative in `docs/RESOLVED.md` (T-68).

## [0.3.57] — 2026-08-04

### xolu upgraded v0.23.0 → v0.25.0, and a real proof of form+FSM+xolu composition

Responding directly to reviewer feedback: everything built this
session before now was stateless CRUD — nothing exercised "form + FSM
+ xolu commit, composed, for something genuinely new."

Investigating what a real test would need surfaced a structural fact:
the xolu version pinned all session had **zero FSM client methods at
all**. Composition wasn't just untested, it was untestable with the
available tooling. Upgraded to v0.25.0 (which has a full FSM client
API), after confirming compatibility first: all 7 `xolu.Client` methods
seam-ui actually calls have identical signatures across both versions.
Full seam-ui test suite clean (24 packages, zero failures, zero code
changes needed). Backward compatible at the data layer too — the new
binary correctly loaded the existing demo database with no migration
step. Live-verified the whole app against the new backend: fresh
login, six key pages all 200, no server errors.

Built a minimal Work-Order-shaped FSM definition (`draft → in_progress
→ completed`) with the final transition guarded on a non-empty
`technician_signature` — the exact field shape the signature widget
built earlier today produces. Through the real Go client library
against a real running xolu server: a guard correctly rejected an
empty-signature completion attempt, correctly accepted a real
signature-shaped one, the commit was durably confirmed via a fresh
fetch in a separate request, and the full audit trail recorded every
step. This directly answers the reviewer's named gap.

Also corrected a separate, pre-existing register inconsistency noticed
in passing: T-07's status field said unstarted while its own body
described substantial completed work from an earlier session.

Full narrative in `docs/RESOLVED.md` (T-67).

## [0.3.56] — 2026-08-04

### Responding to reviewer feedback: ROADMAP.md staleness, T-10 progress

**ROADMAP.md** — header corrected (was still "May 2026,
0.1.0-patched243" against actual v0.3.55) and a clear staleness notice
added, pointing readers to `CHANGELOG.md`/`docs/TRACKING.md`/
`docs/RESOLVED.md` instead. A full content regeneration is real,
separate work — tracked as T-66, not rushed here, since a hurried
rewrite of ~200 lines against 65+ resolved items risks introducing
fresh inaccuracies of exactly the kind being fixed.

**T-10** — its "After T-01 MVP" framing was stale in the same shape
T-02/T-03/T-08 were: T-01 closed 2026-07-31 without any of the widgets
this item lists, meaning the "pull into MVP now vs. defer" question
resolved itself by default, never as an explicit choice. Corrected.

Built the **signature widget** as real, concrete progress — a
general-purpose canvas-based capture primitive, not Order-Engine-
specific, so it didn't need CMMS's own scoping (T-11) to be worth
building now. Security design follows T-63/T-64's established lesson
directly: an existing value goes through the hidden input's normal,
automatically-escaped `value="..."` attribute, never interpolated into
the accompanying `<script>` block — verified with a dedicated
adversarial test using a script-tag-breakout-shaped value. T-10 moved
from unstarted to partial; most of its named scope (image, GPS,
multi-select, decimal precision, barcode, four `x-seam-*` extensions)
remains open.

Full narratives in `docs/TRACKING.md`.

## [0.3.55] — 2026-08-04

### Register: interdependency review across Settings/Administration/Account after T-02

No code changes — a priority and cross-reference pass on the register
after examining what actually depends on the three-module split.

- **T-03, T-08**: priority downgraded P1→P2. Both had "boundary
  reconciled with T-02" as their blocking note, written while T-02 was
  open. T-02 is now closed and the Administration/Settings boundary is
  real, in code — the architecturally-urgent question both items were
  really about is answered. What remains for each (Assets' fuller spec,
  Settings' schema-lifecycle/form-designer/i18n-manager tooling) is
  real feature work, not an open structural gap.
- **T-57**: priority raised P3→P2. Its own text named "Settings' three
  sections, once built" as the trigger for its duplication bug to stop
  being hypothetical — those three sections were built the same day
  (MOD-8) and are live today. `/settings` now renders 3 independent
  `visibilityScript` copies per page load, not 2. Still not user-visible
  (no `visible_when` condition exists anywhere yet), so not bumped
  further, but no longer purely speculative.
- **T-41, T-11**: cross-reference notes only, no priority change.
  T-41's foundational jurisdiction-setting gap now has a concrete home
  (Settings' `settings_general.json`) that didn't exist before T-02.
  T-11's open "Administration vs Settings vs core" question for two
  unplaced domain entities is now more answerable, given both modules'
  scopes are real and narrow rather than one undifferentiated System.

Full narratives in `docs/TRACKING.md`.

## [0.3.54] — 2026-08-04

### Closed — T-02: Account, Settings, and Administration are now three separate modules

The tech team's decision: decouple "these three things need to be real modules"
from "how they're grouped." Proposed and built accordingly — mechanical
route extraction first (`MountAccountRoutes`/`MountSettingsRoutes`/
`MountAdministrationRoutes`, split out of the former single
`MountSystemRoutes`), grouping decisions expressed only in how
`server.go` composes them into registry entries.

Three modules, not one: **Account** (Profile, visible to every role
unconditionally), **Settings** (tenant config, gated on `settings.*`),
**Administration** (user/role management, gated on `administration.*`
— what T-02 was originally asking for). `/system` and its dashboard
retired deliberately, not left as dead weight — each of the three
replacement modules lands on genuine content of its own, so nothing
needed a separate landing-page dashboard.

Two real issues found and fixed along the way, not just plumbing:

- The existing permission constants (`system.settings.*`,
  `system.user.*`) were namespaced under the old single-module world —
  would have silently broken the registry's generic module-access
  derivation once Settings/Administration became independently gated.
  Renamed to `settings.*`/`administration.*` throughout.
- **A real UX issue we flagged directly**, handled in the same
  iteration: the layout's pre-existing user-avatar dropdown already
  links straight to `/profile` — showing Account a second time in the
  module selector would have been redundant. Added
  `modules.Module.HideFromSelector` (real access and routes, just
  suppressed from the selector's own rendering) rather than a special
  case buried in the selector's code.

Live-verified end to end: module selector shows exactly Assets/
Documents/Settings/Administration with no Account entry; the avatar
dropdown's own link confirmed untouched; `/profile`/`/settings`/
`/admin/users` all reachable; `/system` confirmed retired (404). No
server errors. Full build/vet/test clean, including a complete rewrite
of the routing test suite and new adversarial coverage for
`HideFromSelector`, verified by reverting the fix and confirming it
failed before restoring it.

Full narrative in `docs/RESOLVED.md`.

## [0.3.53] — 2026-08-04

### Closed — T-65: three PATCH-fix call sites had zero test coverage of their own

Closing out this session's adversarial testing campaign: systematically
audited all 17 of the session's Update-to-Patch fixes for real coverage
rather than assuming each got a test. Three genuine gaps, none
previously flagged, none ever regressed:

- `ProfileSave` — T-49's own original trigger, live-verified at the
  time but never given a dedicated unit test, unlike its siblings in
  the same fix.
- `AlertResolve` — its sibling `AlertAcknowledge` has had a test since
  T-49's original fix; this one never got its own.
- `AlertaMasterUpdate`/`alert_types` — zero coverage at all, not even
  the incidental protection several sibling masters entities get from
  a shared mock helper.

New tests for all three. `ProfileSave`'s verified with the full
discipline — reverted to `Update`, confirmed the test failed with a
real xolu 404 propagating through the handler's own error path,
restored.

This closes out the adversarial campaign begun 2026-08-03/04, which
separately found and fixed real security issues (T-62 CSRF gap, T-63
OQL injection + stored XSS, T-64 a second XSS instance found by
checking every remaining `mi.Raw` site rather than stopping at the
first fix), plus substantial test-only additions with no further bugs
found across the module registry, module selector, router mounting,
and formengine's i18n resolution chain.

Full narrative in `docs/RESOLVED.md`.

## [0.3.52] — 2026-08-04

### Adversarial test coverage: escape-bypass sweep, module selector, router-level mounting

Completed a full audit of every `mi.Raw`/`RawHTML` call site in the
codebase (the mechanism T-63/T-64 exploited) after finding the FSM
editor bug via an incomplete first pass. Confirmed the remaining sites
— including one using string concatenation rather than `fmt.Sprintf`,
which the original search had missed entirely — are all genuinely safe:
server-controlled config constants or hardcoded literals, verified by
tracing each one's data provenance directly rather than assuming.
Also confirmed no other raw-output-bypass mechanism (direct `w.Write`,
`io.WriteString`) exists beyond a safe, static DOCTYPE prefix pattern.

New adversarial tests, no further bugs found (all confirming existing
correct behavior — minty's baseline HTML escaping, Go's own
`http.ServeMux` duplicate-pattern panic):

- `internal/ui/module_selector_test.go`: HTML/attribute escaping via a
  malicious Label/Path, caller-supplied duplicate modules (no dedup of
  its own — a real, documented trust-boundary fact, not a bug), multiple
  system-group modules, an empty Path's degrade-gracefully behavior.
- `internal/ui/router_test.go`: mounting the same routes twice on one
  mux panics loudly (confirmed at both System's 13-route and Assets'
  98-route scale); mounting onto two separate mux instances succeeds.

Full suite clean, zero regressions.

## [0.3.51] — 2026-08-04

### Closed — T-64: FSM editor had the same XSS class as T-63, missed in that investigation

Prompted by a direct question ("is there anything more?") to check
whether T-63's fix had actually covered every `mi.Raw(fmt.Sprintf(...))`
call site in the codebase. It had not — three exist; only one (the Map
page) had been checked and fixed.

`FSMEditorPage`'s `<seam-fsm-editor>` custom element interpolated a real
FSM's name directly into an HTML attribute with zero escaping. `mi.Raw`
bypasses minty's own automatic escaping entirely, and minty has no
public custom-tag builder to use instead. Fixed with `html.EscapeString`
on every interpolated value, matching what minty's own rendering does
for every other attribute in the app.

Verified with the full discipline — reverted, confirmed the exploit
rendered live in the test failure output (`onmouseover=` as a real,
executing attribute), restored.

Full narrative in `docs/RESOLVED.md`.

## [0.3.50] — 2026-08-04

### Security: OQL injection (six handlers, fifteen points) and stored XSS via asset name — both found via adversarial testing, both fixed

**OQL injection.** Every list-with-filters handler (`AssetsList`,
`EventsList`, `SensorsList`, `AlertsList`, `MapView`,
`AdminUsersList`) built its OQL `WHERE` clause by directly string-
concatenating URL query parameters — `query += " AND status = '" +
statusFilter + "'"`, unescaped. `internal/auth` already had an
`escapeOQL` helper for exactly this; `internal/ui` simply never had
an equivalent, across 15 separate injection points. Two (unquoted
numeric comparisons) needed a type check instead, since escaping
quotes does nothing where there's no quote to escape out of. Fixed by
adding `escapeOQL`/`validOQLIntFilter` and applying one or the other
everywhere.

**Stored XSS on the Map page**, layered on top of the same underlying
issue. `markersJSON` was hand-assembled with no escaping at all, then
the client-side JS concatenated an asset's name directly into
Leaflet's `bindPopup()` as raw HTML. An asset named with a script
payload — settable by any user who can edit assets — would execute in
the browser of anyone who opened the map. Fixing the JSON embedding
alone (via `encoding/json`, which HTML-escapes by default) wasn't
sufficient: `JSON.parse` reverses that escaping client-side, so
`bindPopup()`'s raw-HTML-string construction was still exploitable.
Fixed by rebuilding the popup via DOM APIs (`createElement`/
`textContent`) instead of an HTML string at all.

Every fix verified with the full discipline — reverted, confirmed the
test failed (one sanity check's diagnostic output showed the literal
working injection, `WHERE type_id = 1 OR 1=1`), restored. New tests in
`internal/ui/oql_injection_test.go` and `internal/ui/vendor_html_test.go`.

### Closed — T-62: AdminUserCreate's "email already exists" path never set CSRFToken

Found while rewriting the handler for MOD-8. Of three possible render
paths, one never set `CSRFToken` — resubmitting that specific
re-rendered form would always have failed CSRF validation. Fixed by
routing all three paths through one closure. (Originally mislabeled as
part of 0.3.49 at closure time; that version had already shipped
before this fix existed — corrected in `docs/RESOLVED.md`.)

Full narratives in `docs/RESOLVED.md`.

## [0.3.49] — 2026-08-04

### MOD-8 complete: user create/edit (Administration) — the last piece of the formengine rebuild

Two schemas, not one: `user_create.json` (email, name, role, password,
confirmation) and `user_edit.json` (name, role, active) — genuinely
different field sets, since create and edit have never shared one
form's worth of fields. Email stays hand-coded and disabled on edit,
same readonly-field gap Profile hit — formengine still has no such
widget type.

This completes MOD-8's entire original scope: profile, preferences,
settings, password, and now user create/edit are all rebuilt on
formengine.

### Closed — T-62: AdminUserCreate's "email already exists" path never set CSRFToken

Of its three possible render paths (validation failure, email-exists,
success), the email-exists one never set `CSRFToken` — resubmitting
that specific re-rendered form would always have failed CSRF
validation. Consolidated all three paths through one closure so this
can't drift again. New test verified by the full discipline: reverted
the fix, confirmed it failed, restored it.

### Filed — T-61: some test mocks for xolu.Client.Get responses have an inaccurate shape

`Client.Get` uses the entire response body as `Entity.Data` directly —
no `"data"` sub-object unwrapping. Several existing mocks (including
one from this session's own earlier T-46 work) wrap fields under a
nested `"data"` key that's never actually read. Currently invisible —
none of those tests assert on the affected field — but a real
inaccuracy. Not fixed everywhere as a side effect of this session;
filed for a dedicated pass.

Live-verified end-to-end: created a real user through the real form,
confirmed the new user could log in, edited the same user (name/role/
active all changed correctly, everything else survived), confirmed the
deactivated user is correctly blocked from login. No server errors
throughout.

Full narrative in `docs/modules/TRACKING.md`'s MOD-8 entry.

## [0.3.48] — 2026-08-03

### Closed — T-60: added a real "password" widget to formengine (MOD-8's password-change form)

formengine had no password widget at all. Added one deliberately, not
as a reuse of the existing text widget: `renderTextLike` echoes the
current value back into the rendered `value=` attribute — correct for
every other field, a real security problem for a password (validation-
error re-render, browser history, view-source). `renderPassword` never
reads the value at all.

Also added a new schema extension, `x-seam-autocomplete` —
`current-password` vs `new-password` meaningfully changes browser
password-manager behavior, so a single hardcoded value would have been
wrong for at least one of the three password-change fields.

New tests, the security-critical one verified by the full discipline:
reverted the fix and confirmed the failure output visibly leaked the
test's distinctive password value into rendered HTML, then restored it.

Live-verified end-to-end: a real password change through the real form,
confirmed the new password actually works via a real login attempt,
then reverted to the original and confirmed that works too. No server
errors throughout.

This completes profile/preferences/settings/password of MOD-8's
original scope — user create/edit (Administration) remains. Full
narrative in `docs/modules/TRACKING.md`'s MOD-8 entry.

## [0.3.47] — 2026-08-03

### MOD-8 (in progress): formengine rebuild of Profile, Preferences, and Settings

Per the tech team's clarification that Settings/Profile/Administration are one
module with three views — confirming the fuller formengine-based
separation this stage works toward, not MOD-5's hand-coded shape as a
final state. Rebuilt Profile, Preferences, and all three Settings
sections (general/notifications/features) on formengine, live-verified
end-to-end: real name/preferences/settings changes persisted correctly,
including the unchecked-checkbox-omission case, with every other field
on the record surviving intact.

### Closed — T-59: formengine toggle/checkbox fields never showed translated titles

Found while building the above — the first toggle/checkbox fields ever
wired into a live route in this codebase. `renderFieldRow` computed a
translated field title but never applied it to self-labeled widgets
(checkbox/toggle), which render their own label and read the schema's
raw literal directly. Every toggle in the entire formengine system was
affected, silently, since the widget type was first built. Fixed with
one line; new regression test using a real translator, verified by
reverting the fix and confirming it fails (the raw English literal
visibly leaks into the failure output).

Separately: none of the affected fields had i18n title keys authored at
all — a second, independent gap. Added across all four locales, reusing
the old hand-coded forms' exact existing wording. Also found live:
enum-option labels are form-scoped, so Settings' own `locale` field
needed its own label set separate from Preferences' identical-looking
one.

Submit-side handlers needed zero new code — field names already
matched. Full narrative in `docs/modules/TRACKING.md`'s MOD-8 entry.

## [0.3.46] — 2026-08-03

### Closed — T-29: syncver.sh never actually synced the tracking docs' Version/Last reviewed headers

Found 2026-07-31, corrected by hand every release since (0.3.6 through
0.3.45) rather than left stale. `repoman/syncver.py`'s `set_version`
already correctly reads `.repoman.json`'s `version_targets` and applies
each file's own regex — confirmed directly (`sync: ok` against all four
current target files). `syncver.sh`'s own `set_version` now delegates to
it for the `Version:` line instead of reimplementing the same logic in
bash. `Last reviewed:` stays local to `syncver.sh` — it's this project's
own convention, not a generic repoman concept — synced to today's date
across the same four files, only where the line already exists.

Verified directly: idempotent re-set (correctly silent), a deliberately
staled single file (correctly detected and fixed, others left alone), a
genuine version bump across all four files, then restored. This
release's own `Version:`/`Last reviewed:` headers were synced
automatically for the first time, not by hand.

### Closed — T-48: duplicate of T-09

The flaky `TestGenerator_ValidateChecksum` finding filed this session
turned out to be the same test T-09 (2026-07-28) already root-caused:
the test corrupts a checksum with a fixed `'X'`, but `'X'` is a valid
character in the 36-char checksum alphabet, so ~1/36 of generated codes
legitimately end in `'X'` already. No independent fix under this ID —
see T-09.

### Noted, not resolved — T-02/T-03/T-08 predate the shipped module design

While answering "what are the pending tasks," found these three items
(2026-07-28) describe an earlier four-module split (Assets Management,
Documents, Settings, Administration as separate modules) that
`MODULE_SPEC.md`'s actual shipped design (T-44, MOD-1 through MOD-7)
superseded rather than fulfilled — three modules (Assets, Documents,
System), with Settings/Profile/Administration as views inside System.
Annotated each item with the discrepancy rather than closing or
rescoping unilaterally; whether they're now obsolete or still represent
real separate future work is a call for the tech team.

## [0.3.45] — 2026-08-03

### MOD-7: Documents module — minimal stub proving the registry pattern for a genuinely empty module

Applied MOD-5/MOD-6's `MountXRoutes` pattern at the opposite end of the
size spectrum: one route (`GET /documents`), unconditional. The
permission (`documents.document.view`) and selector i18n key
(`nav.module.documents`) already existed from earlier work, narrowing
this stage to `documents-dashboard.json` (`"sections": []`),
`schemas.Documents()`, a `DocumentsDashboard` handler, and
`Router.MountDocumentsRoutes(mux)`.

### Closed — T-50: empty-dashboard rendering referenced an i18n key that existed in no locale file

Found while building MOD-7: `dashboardengine`'s empty-dashboard
rendering path (`DASHBOARD_SPEC.md` §2.2, built in MOD-2) references
`"dashboard.empty_message"` — confirmed directly, this key existed in
none of the four locale files. Nothing had exercised the `"sections":
[]` path against a real schema before Documents (System and Assets both
have real sections), so the gap was invisible until now. Without the
key, any empty dashboard would have shown the literal text
`"dashboard.empty_message"` to users instead of a real message. Added
to all four locales; new test verified by deliberately removing the key
and confirming it failed, then restoring it.

### Closed — T-44: Module system + Dashboard invariant (parent tracking item)

All of MOD-1 through MOD-7 are now complete — the closure gate this
item was waiting on. MOD-8/MOD-9/MOD-10 remain explicitly deferred, per
the plan's own framing, not blocking.

Live-verified beyond unit tests: `/documents` reachable as an
authenticated user, module selector showing all three modules correctly,
and the empty-dashboard rendering confirmed showing the real Spanish
message, not the raw key.

Full narratives in `docs/RESOLVED.md` and `docs/modules/TRACKING.md`'s
MOD-7 entry.

## [0.3.44] — 2026-08-03

### MOD-6: Assets module — thin wrapper registering internal/ui's existing routes through the registry

Applied MOD-5's resolved `MountXRoutes` mechanism (`MODULE_SPEC.md` §5.4)
to a far larger surface, confirmed directly against §6.1's own
description ("assets, sensors, rules, alerts, masters... ~3,000+
lines"), not a narrower reading: **98 routes** (dashboard, assets,
events, sensors, alerts, map, FSM editor, rules, asset types, and all 9
master tables) versus System's 13.

`Router.MountAssetRoutes(mux)` — extracted mechanically from
`registerRoutes()`'s original inline lines via a Python line-slice-and-
transform script rather than retyped, specifically to eliminate
transcription risk at this size. The exact `EnableOperational`/
`EnableAdmin` gating each route already had was preserved unchanged.
`internal/modules/assets` is the same two-line closure wrapper as
`internal/modules/system`. `MODULE_SPEC.md` bumped to v0.5 to fix its
own stale registry-entry snippet and document the resolved mechanism
(§6.4).

By this stage, `registerRoutes()` has nothing `requireAuth`-gated left
inline at all — only genuinely public routes (login, password reset, QR
scan, static files) remain.

New test coverage mirroring MOD-5's own pattern: 5 new tests in
`internal/ui/router_test.go` covering dashboard reachability,
`EnableOperational` gating, `EnableAdmin` gating (both a non-master
route and a master-table route, since they use different permission
constants), and `requirePerm` genuinely blocking an under-privileged
role — each verified by deliberately breaking it and confirming the
test caught it. `internal/modules/assets` also got its own delegation
test, matching System's.

Live-verified beyond unit tests: all 19 top-level routes across every
category confirmed reachable as authenticated admin; module selector
confirmed still correct; a real region created through the real form
confirmed landing correctly in xolu.

Full narrative in `docs/modules/TRACKING.md`'s MOD-6 entry.

## [0.3.43] — 2026-08-03

### Closed — T-49: xolu.Client.Update() silently destroyed data on every partial edit

Reported by the tech team: saving Settings appeared to destroy or overwrite
the user's profile. Root cause: `xolu.Client.Update()` sends HTTP `PUT`
— a full-document replace, confirmed empirically. Every handler calling
`Update` with a hand-picked subset of an entity's fields was silently
wiping everything else on every edit — not a Settings-specific bug, a
systemic pattern across the app.

Audited all 17 `.Update(ctx, "<entity>", ...)` call sites in
`internal/ui` and switched every one that needed it to `.Patch()` (a
real partial merge, also confirmed empirically):

- `ProfileSave`/`ProfilePreferencesSave` — the most severe: any user
  editing their own name or preferences could silently wipe their own
  email and password hash, locking themselves out
- `AdminUserUpdate` — matches the original report exactly; reproduced
  directly this session
- `AlertAcknowledge`/`AlertResolve` — a real alert has 9 fields, these
  sent only `{"status": ...}`
- All 9 masters entities, `rules` (was dropping `created_at`), and
  `asset_types` (was dropping `slug`/`fsm_id`)
- `assets` — the one entity confirmed *not* currently broken, verified
  live rather than assumed (the dynamic form genuinely submits every
  field across all four tabs together). Patched anyway for
  defense-in-depth against future schema fields the form might not
  cover yet.

New test coverage for the four entities (`Region`, `Currency`,
`PropDefinition`, `Rule`) that had zero prior coverage of their update
path at all, plus `AssetType`. The other 13 fixes are protected by
existing tests whose mocks were updated from `PUT` to `PATCH`.

Live-verified end-to-end beyond unit tests: the original report's exact
scenario (`AdminUserUpdate`), the Asset form's round-trip safety, and
`AlertAcknowledge` — a real alert's status changed correctly while every
other field (asset association, message, severity, timestamps) survived
intact.

Full closure narrative in `docs/RESOLVED.md`.

## [0.3.42] — 2026-08-03

### Closed — T-46: AdminUserEdit never set CSRFToken — editing an existing user was completely broken

`AdminUserEdit` built its `UserFormData` without setting `CSRFToken` at
all, unlike every other form-rendering handler. The edit-user form's
hidden `_csrf` field always rendered empty, so submitting any change to
an existing user always failed CSRF validation with 403 — not degraded,
non-functional. Fixed with the one-line pattern every other handler
already uses. New regression test (`TestAdminUserEdit_RendersNonEmptyCSRFToken`)
— its first draft used a `strings.Contains` check that passed vacuously
regardless of the bug (real attribute order is `name="_csrf" type="hidden"
value="..."`, not `name="_csrf" value="..."`); caught by deliberately
reverting the fix and confirming the test failed, then rewritten with a
proper regex.

### Closed — T-47: ListAuditEvents' sort was broken two layers deep

`xolu.Client.List()`'s `Sort` parameter is never transmitted to the
server at all (confirmed directly against the client library — it only
ever sets `per_page`/`page`). Underneath that, OQL's own `ORDER BY`
has a narrower, real bug: `ASC` is genuinely correct; `DESC` places the
single maximum-valued row correctly, then falls back to ascending for
the rest. Full raw diagnostic transcript shared with the xolu team
directly (`oql-orderby-diagnostic-report.md`), reported upstream.

Fixed on this side without waiting on the upstream fix: `ListAuditEvents`
now queries xolu directly via OQL with `ORDER BY id DESC` — independently
confirmed correct end-to-end, since `id` is a real indexed column and
`audit_logs` is append-only with auto-incrementing ids, so id order and
timestamp order coincide in every practical case. New test coverage
(`internal/auth/audit_store_test.go`) — this class of bug had zero prior
coverage (only the in-memory `mockAuditStore` was ever exercised, never
the real xolu-backed implementation), which is exactly why it went
unnoticed.

Live-verified end-to-end after both fixes, together: a real role change
through the real edit form now correctly appears first in the System
dashboard's admin-activity widget, with the correct email — checked in
both Spanish (tenant default) and English (switched via Settings, per
The tech team's standing instruction to always verify the English baseline
alongside any other locale).

Full closure narratives in `docs/RESOLVED.md`.

## [0.3.41] — 2026-08-03

### Closed — T-45: release.sh checkpoint zip was silently dropping `pkg/`

`release.sh`'s checkpoint step zips an explicit, hardcoded `ZIP_SOURCES`
array rather than the whole tree, by design (avoids sweeping in build
artefacts, editor swap files, local config). `pkg/` was created as a new
top-level directory during the MOD-2 stage but never added to that list,
so the `v0.3.40` checkpoint zip silently omitted `pkg/nplate/` in its
entirety — the package `internal/dashboardengine/compile.go` imports.
Recovered `pkg/nplate/nplate.go` and `pkg/nplate/nplate_test.go` by hand
from a prior session (verbatim, unmodified); confirmed the restored tree
builds, `go vet`s, and tests clean, including `nplate`'s own 11 tests.

Fixed `release.sh` by adding `pkg/` to `ZIP_SOURCES`. Tree-wide sweep
confirms every other top-level directory (`api/`, `cmd/`, `docs/`,
`internal/`, `pkg/`, `repoman/`, `scripts/`, `seeds/`, `web/`,
`.repoman.json`) is now accounted for.

Also found and fixed the same shape of bug one level down: `.repoman.json`'s
`version_targets` list (consumed by `syncver.sh`'s intended per-file version
sync, per T-29) was missing `docs/modules/TRACKING.md` — the MOD sub-register
created alongside `pkg/nplate` in the same stage, same "new top-level thing,
list never updated" pattern. Added it. Manually synced all four
`version_targets` files' `Version:` headers to `0.3.41` this release, since
T-29 (still open, untouched by this release) means `syncver.sh` doesn't do
this automatically yet.

Full closure narrative in `docs/RESOLVED.md`.

## [0.3.40] — 2026-08-02

### Closed — MOD-4: module registry and selector UI

New `internal/modules` package (`Module`, `Registry` — dependency-free,
`MountRoutes` takes `*http.ServeMux` directly, no coupling to `internal/
ui`). New selector UI wired into `TopHeader` between the dark-mode toggle
and the user menu, with a `SeamModuleSelector` JS toggle mirroring the
existing `SeamUserMenu` exactly. `Config` gained `VisibleModules`,
computed fresh per request; its own pre-existing `UserRole` field —
defined but never populated anywhere before this — is now populated by
the same mechanism.

One scope refinement, recorded not silently redefined: fully "replacing
today's single direct call into `internal/ui`" turned out to be `MOD-6`'s
own dependency — doing it now, before Assets is actually re-registered
through the new mechanism, would have broken the whole app. `MOD-4`
introduces the mechanism; `MOD-6` completes the migration onto it, per
the plan's own stated dependency order.

Two real bugs caught before shipping:
- A Go limitation (can't mix explicit args with a spread in one call) —
  fixed the same way already used in `dashboardengine`.
- A real spec bug: `Module` had only `DashboardID`, and the selector's
  first draft built hrefs from it — wrong for Assets, whose dashboard
  lives at `/`, not `/assets-dashboard`. Added a distinct `Path` field,
  corrected `MODULE_SPEC.md` (now v0.3) including all three worked
  examples.

12 new tests. Live-verified against a real xolu instance and a real
authenticated session — correct Spanish translations from the tenant's
own locale, correct icon, and `href="/"` confirming the `Path` fix
specifically. Only Assets registered this stage — System/Documents don't
have real routes yet, and a selector entry that 404s isn't a working
feature. Full closure narrative in `docs/modules/RESOLVED.md`.

## [0.3.39] — 2026-08-02

### Closed — MOD-3 (part 2): audit_log dashboard source + permission-based visibility

`dashboardengine`'s second source type (`DASHBOARD_SPEC.md` §5.2) and
`visible_when`'s permission form (§7.2) — the piece System's dashboard
(MOD-5) needs. Real design work first: `AuditFilter` couldn't filter by
`Resource` or multiple `EventTypes`, and nothing outside `internal/auth`
could query audit events at all (`ListAuditEvents` existed, never called).
Extended `AuditFilter`, added `AuditLogger.List` (nil-safe), extended
`XOLUAuditStore`'s filter loop to match.

Two real bugs caught and fixed *before* they shipped, both found by
questioning a first draft rather than trusting it compiled:

- Wrote `PermissionChecker`'s nil-fallback as fail-open at first
  (matching `Translator`'s own nil fallback) — realized mid-edit that's a
  security anti-pattern for a permission gate, unlike `Translator`'s
  cosmetic worst case. Corrected to fail closed.
- `sourceCacheKey`'s `json.Marshal(EntitySource)` would never vary by
  `CurrentUserID` (it carries `json:"-"`), meaning two different users'
  `scope: "self"` results would have collided in the cache — one user
  seeing another's audit trail. Fixed with an explicit append outside the
  marshal, and a dedicated regression test.

9 new tests (6 `dashboardengine`, 4 `internal/ui`) — one test bug caught
in the same pass: an early `visible_when` test never set
`RenderContext.Role`, "passing" by coincidence rather than proving
anything. Full closure narrative in `docs/modules/RESOLVED.md`.

## [0.3.38] — 2026-08-02

### Added — MOD-3 (part 1): audit log wiring

Real `auditLogger.Log(...)` calls added to the handlers `DASHBOARD_SPEC.md`
§9.3 identified as silently un-audited, using convenience methods that
already existed in `internal/auth/audit.go` — `LogUserCreated`,
`LogUserUpdated`, `LogRoleChanged`, `LogPasswordChanged` — written but
never called before this pass:

- `AdminUserCreate` — both the `UserService` and xolu-fallback paths;
  captured the previously-discarded new user ID, needed for the event.
- `AdminUserUpdate` — now fetches the existing record first, needed to
  detect a real role change (`LogRoleChanged` fires only when the role
  actually differs, not on every save).
- `AdminUserPasswordResetSave` — `byAdmin=true`.
- `SettingsSaveGeneral`/`Notifications`/`Features` — new
  `AuditSettingsChanged` event type and `LogSettingsChanged` method
  (didn't exist before this pass).
- `ProfilePreferencesSave` — new `AuditPreferencesChanged` event type and
  `LogPreferencesChanged` method, guarded on save success.

Same pass fixes `/settings/{general,notifications,features}`'s missing
permission gate (`requireAuth` only until now) to
`requirePerm(auth.PermSystemSettingsEdit, ...)`, per this project's own
"one fix, not two trips through the same code" note when MOD-3 was filed.
`GET /settings` stays `requireAuth` — every current role already holds
`PermSystemSettingsView`, so gating it now would be a no-op dressed up as
a fix, not something this pass's actual scope covers.

Two real findings, not assumed clean:
- A genuine regression: adding the `Get()` call in `AdminUserUpdate`
  broke `TestAdminUserUpdate_Valid_Redirects`, whose mock server didn't
  handle the new request. Fixed the test's mock, not the production code.
- A latent nil-receiver bug in `AuditLogger.Log` itself, found while
  investigating why a first pass of new tests passed without the audit
  calls ever actually executing (no auth claims in bare test requests,
  so the calling code's own guard silently skipped them — not evidence
  the wiring worked). Added `if a == nil { return }` — audit logging is a
  cross-cutting, non-critical-path concern that must never be capable of
  crashing the request it's observing. Then wrote real end-to-end tests
  (`internal/ui/audit_wiring_test.go`, 5 tests) that inject real
  `auth.Claims` into the request context and assert on a recording
  `auth.AuditStore`, proving each event actually fires with correct
  data — not just that handlers still redirect. One test's own first
  assertion was wrong, not the code under test: `LogUserCreated`
  (pre-existing, unused before this pass) puts the new user's email in
  `Metadata`, not the event's top-level `Email` field — checked against
  its actual implementation and fixed the test, not the method.

## [0.3.37] — 2026-08-02

### Closed — MOD-2: dashboard engine core

Live-verified a second time immediately after `dashboard.go`'s retirement
(0.3.36) — restarted the verification environment, re-seeded real data,
confirmed identical stat counts, badge colors, and translated labels
before and after. `attic/old-dashboard/README.md` written, matching the
`static-asset-form` precedent. `DASHBOARD_SPEC.md` bumped to v0.6,
documenting `date_field_labels` (found live in 0.3.36, not yet written up
in the spec itself). Full closure narrative in `docs/modules/RESOLVED.md`
— this entry says what changed, not the full story.

## [0.3.36] — 2026-08-02

### Added — pkg/nplate: iteration for jsonplate templates

New exported package, `github.com/ha1tch/seam-ui/pkg/nplate`. Adds one
construct beyond `github.com/ha1tch/xolu/pkg/jsonplate` — `{"$each":
"path", "as": "name", "template": {...}}` — built entirely on
`queryfy.Collect` (iteration) and `jsonplate.Render` (substitution),
re-implementing neither. Extending `jsonplate` itself was considered and
rejected: its no-loop constraint is a deliberate safety guarantee for its
real production consumers elsewhere in xolu (`event_dispatch.go`,
`v2_dxp_def_handlers.go`, `pkg/storage/sqlite.go`), not this package's
call to weaken. 11 tests, nested `$each` support falls out of the
recursive design rather than needing separate implementation.

### Added — MOD-2: dashboard engine core (`internal/dashboardengine`)

`DASHBOARD_SPEC.md`'s declarative dashboard format, implemented as
resolve → compile → render:

- **Resolve** (`source.go`'s `EntityResolver`, real implementation in
  `internal/ui/dashboard_resolver.go`) — no xolu dependency in the
  package itself, same decoupling `formengine.SourceResolver` already
  established.
- **Compile** (`compile.go`) — resolved data rendered through a fixed,
  per-widget-type `nplate` template into canonical JSON.
- **Render** (`widgets.go`) — the five widget types (`stat_card`,
  `recent_list`, `urgency_list`, `total_card`, `breakdown_list`), ported
  from `dashboard.go`'s own `dash*` functions, now consuming only
  compiled data.
- **Cache** (`cache.go`) — `DashboardEngine.CacheTTL`, keyed by widget
  type + `EntitySource`; zero (default) disables caching entirely.

19 tests, including a call-counting fake resolver proving cache hits
skip `EntityResolver` entirely, and end-to-end renders through the real
`Render` pipeline, not just unit tests of individual functions.

`internal/dashboardengine/schemas/assets-dashboard.json` is the real
definition now live at `GET /`, using *current* field names (`criticidad`,
`fecha_expiracion`, `fin_garantia` — T-40's Asset rename is still paused),
not the spec's own aspirational English worked example.

Real bugs found and fixed before/during implementation, not assumed away:
- `entity` sources couldn't express a 3-entity join (`alerts`→
  `alert_types`→`assets`); confirmed directly that xolu's OQL supports
  only two-table joins. Added `enrich` to the source format
  (`DASHBOARD_SPEC.md` §5.1, v0.4).
- `recent_list` had no way to name which row field is primary/secondary/
  timestamp text; added `primary_field`/`secondary_field`/
  `secondary_fallback_field`/`timestamp_field`.
- A row's own link and the header's "view all" link were conflated
  (`ItemHrefPrefix` vs `ViewAllHref`) — would have produced a broken
  `/alerts?status=open/5` href; caught before shipping.
- `date_field`/`label_field`/`group_field` (originally speculative widget
  properties) turned out unused by any real rendering code once built —
  removed rather than left as dead schema surface.
- An earlier spec draft's `urgency_list` example listed three date fields
  where the real `loadVencimientos` only ever checked two.
- Live verification (real xolu, real seed data, authenticated session)
  caught two more: `badge_colors` used English keys against actual
  Spanish stored criticality values (`Alta`/`Media`/`Baja`, per T-40's own
  note that field *values* weren't touched, only field *names*); and
  `date_fields`' raw xolu column names don't match the pre-existing
  `dashboard.field.expiration`/`warranty_end` i18n keys — added
  `DateFieldLabels` to decouple the two.

### Removed — internal/ui/dashboard.go (retired)

Superseded by `dashboardengine`, proven live against real data first.
Whole-file `.pre-retirement` copies in `attic/old-dashboard/` (matching
the precedent set by the static Asset form's own retirement, FE-24):
`dashboard.go`, `dashboard_test.go`, and `masters_routes.go`'s
`loadVencimientos` (extracted separately — that file stays, only the one
function left with it). `EventSummary` (genuinely used elsewhere) moved
to `asset_detail.go`; `AlertSummary` (zero real callers, already marked
"kept for backward compatibility" in its own original comment) stays
retired. Three shared test helpers (`contains`, `containsHelper`,
`newTestHandler`) moved to `helpers_test.go`; `TestFormatNumber` ported to
`dashboardengine`'s own tests, since it now tests an unexported function
in a different package. `web/static_test.go`'s icon-font scan extended to
also walk `internal/dashboardengine/schemas/*.json` — icons that used to
be Go string literals now live in JSON, which the existing Go-source-only
scanner couldn't see.

## [0.3.35] — 2026-08-02

### Changed — MOD-1: permission model reshaped to module.view.action

New hierarchical permission format (`internal/auth/auth.go`) replacing
the old flat `resource:action` strings — `assets.asset.view` instead of
`asset:read`, wildcard matching at the view or action position
(`assets.*`, `assets.asset.*`), and module-access derivation (does a role
hold at least one permission scoped to a given module) for the upcoming
module selector. See `MODULE_SPEC.md` §2 for the full design; this closes
`MOD-1` of `MODULE_DASHBOARD_DEVELOPMENT_PLAN.md` (see
`docs/modules/RESOLVED.md` for the full closure narrative — this entry
says what shipped, not the full story).

Every existing `Permission` constant migrated, every `requirePerm`/
`ProtectedGet`/etc. call site updated (127 across `internal/ui/router.go`
and `internal/handlers/api/router.go`, plus 3 direct references in
`internal/ui/routes.go` caught by `go build`, not the router-only sweep).
Role grants preserved exactly — this reshapes the string format, it does
not change who can do what.

Three real findings, not assumed going in:
- `PermDataImport` was dead code (zero usages anywhere) — dropped, not
  migrated.
- Every master-data route's reused `PermSettingsUpdate` (48 sites, all
  writes — the list/view routes were never permission-gated at all) gets
  its own real `assets.master.edit`, not carrying the conflation forward.
- Webhooks (`internal/handlers/api/router.go`, 8 sites) turned out to be
  a *second*, separate misuse of the same old `PermSettingsRead`/`Update`
  — not anticipated by `MODULE_SPEC.md`'s own migration table, found
  during implementation. Given its own `assets.webhook.view`/`edit`.

One deliberate deviation from `MODULE_SPEC.md` §6.2's own illustrative
mapping: sensors stay under `assets.asset.*` rather than the spec's
sketched `assets.sensor.*` split, preserving exact current behaviour
rather than introducing a new access boundary as a side effect of a
format migration.

Two more dead-code removals: `User.CanAccess` (constructed the old
two-segment format directly, zero callers) removed; `Middleware.
RequireAnyPermission` kept as live infrastructure despite having zero
current callers, since it's a generic primitive, not a single obsolete
permission.

New test coverage, not just preserved: `TestPermissionMatches` (the
wildcard algorithm itself) and `TestHasModuleAccess` (the derivation
rule) — both genuinely new behaviour with zero prior coverage.
`TestUser_CanAccess` removed (tested a removed method); `TestHasPermission`
lost its two `PermDataImport` cases for the same reason the constant was
dropped.

### Fixed — repoman/register.py: cmd_list crashed on multi-character ID prefixes

`cmd_list`'s sort key hardcoded `int(t[2:])`, assuming a one-character
`id_prefix`. Silently wrong (sorted by a coincidental negative number,
not crashing) for two-character prefixes like `FE`; a hard `ValueError`
for three-character prefixes like `MOD` — found setting up
`docs/modules/`'s own `MOD`-prefixed register this session. Fixed to
derive the slice from the register's own configured prefix
(`int(t[len(_P)+1:])`). No dedicated test suite exists for `register.py`
to extend (unlike `ed.py`'s own `selftest`); verified manually against
both the `MOD` and main `T` registers, and flagged in the file's own
header as diverging from upstream, not yet synced back
(`github.com/ha1tch/repoman`).

## [0.3.34] — 2026-08-02

### Fixed — T-40 progress: Alerta attribute names are English
(`type`/`description`/`criticality` -> `tipo`/`descripcion`/`criticidad`)

See `docs/TRACKING.md`'s T-40 entry for the full narrative -- this
closes nothing, T-40 stays open. All eight masters entities' attribute
names now done (Region, Moneda, Centro de Costo, Marca, Ubicación,
Custodio, Alerta); Asset itself is the one piece left.

Real bug caught before shipping: `alertaFieldToXolu` couldn't be
emptied like every prior entity's bridge, since `translateFlatFieldsAndEnums`
gates enum-*value* translation (English `High`/`Maintenance`/etc. to the
Spanish values xolu still stores) behind a successful key lookup in
that same map. Fixed with identity entries instead of an empty map,
and proved it mattered: the live-verification record's `criticality`
stored as `"Alta"`, not the raw `"High"` submitted.

Widest collision-risk surface of any entity so far -- `tipo`/
`descripcion`/`criticidad` are shared with Asset's own not-yet-renamed
fields and with two unrelated `DocumentLinksField` blocks in the same
files. Handled via line-range-scoped substitution rather than a
whole-file word replace. `go build` caught one real miss regardless:
AssetType's default-alert picker in `pages.go`.

A pre-existing, unrelated gap found by coincidence: `TestUbicacionEdit_
HTMXReturnsPartial`'s mock had been returning stale Spanish keys since
Ubicación's own rename, undetected because the test never asserted on
field values. Fixed.

End-to-end verification at full scope: fresh xolu + seam binaries,
real seed run, real login, `/alert-types` list and create-form pages
confirmed rendering/emitting English field names, and the resulting
record confirmed directly in xolu's own SQLite store.

## [0.3.33] — 2026-08-02

### Fixed — T-40 progress: Custodio attribute names are English
(`subject_type`/`legal_name`/`tax_id`/`phone`/`address`/`active` ->
`tipo_sujeto`/`nombre_razon_social`/`identificador_fiscal`/`telefono`/
`direccion`/`estado`)

See `docs/TRACKING.md`'s T-40 entry for the full narrative -- this
closes nothing, T-40 stays open. Six entities in (Region, Moneda,
Centro de Costo, Marca, Ubicación, Custodio), Alerta/Asset itself
still ahead.

The biggest masters entity by field count so far. `custodioFieldToXolu`
emptied to a documented pass-through, matching every prior entity's
precedent. One real trap caught before it did damage: a naive
whole-word substitution across the seed scripts would have corrupted
`seed-03-extinfire.py`'s `"direccion@liceo2maldonado.edu.uy"` email
value -- avoided by scoping every seed substitution to the exact
`"key":` pattern rather than the bare word.

End-to-end verification at full scope: fresh xolu + seam binaries,
real seed run, real login, `/custodians` list and create-form pages
confirmed rendering/emitting English field names, and the resulting
record confirmed directly in xolu's own SQLite store with all-English
JSON keys.

### Added — T-41 filed
formengine needs jurisdiction-conditional field visibility + a tenant
jurisdiction setting distinct from `tenant_settings.locale`. Not
scoped or estimated. See `docs/TRACKING.md`'s T-41 entry.

### Changed — xolu bumped 0.20.5 -> 0.21.2
No breaking changes for this project's usage (`pkg/client`, `pkg/cache`,
`pkg/config`, `pkg/graph`, `pkg/server`, `pkg/storage` -- none of the
primitives touched by 0.21.0-0.21.2 upstream). Full build/vet/test
clean against the new version before proceeding with any other work
this session.

## [0.3.32] — 2026-08-02

### Fixed — T-40 progress: Ubicación attribute names are English
(`nombre_corto`/`descripcion` -> `short_name`/`description`)

See `docs/TRACKING.md`'s T-40 entry for the full narrative -- this
closes nothing, T-40 stays open. Five entities in (Region, Moneda,
Centro de Costo, Marca, Ubicación), Custodio/Alerta/Asset itself
still ahead.

`nombre_corto` is *also* PropDef's own field name for a completely
different concept (custom property short codes, deliberately left
Spanish per T-39) -- every replacement scoped precisely to Ubicación's
own function bodies to avoid touching that unrelated usage, same
discipline as Region's own `descripcion` scoping.

End-to-end verification specifically exercised the cross-entity
dependency this rename shares with Region: the region-selection
dropdown on the create form confirmed still showing Region's own
correctly-translated names, and the list page's region-lookup join
(a separate query joining each location to its region's name)
confirmed still working.

### Fixed — T-40 progress: Marca attribute names are English
(`codigo`/`nombre` -> `code`/`name`)

See `docs/TRACKING.md`'s T-40 entry for the full narrative -- this
closes nothing, T-40 stays open. Four entities in (Region, Moneda,
Centro de Costo, Marca), Ubicación/Custodio/Alerta/Asset itself
still ahead.

Three real compile-error catches: Modelo's own brand-selection
dropdown, `MarcaFormPageDynamic`, and `loadMarcas` itself -- the
latter actively called by Modelo's form across 6 call sites, unlike
the dead-code dropdown helpers found for Moneda/Centro de Costo.

One genuine pre-existing gap found by coincidence: a test from the
earlier Centro de Costo pass was silently submitting the wrong field
name and still passing, for an accidentally-correct reason rather
than the reason it claims to test. Fixed and re-verified.

End-to-end verification specifically exercised the cross-entity
dependency this rename put most at risk: `/models/new`'s brand
dropdown (fed by the shared `loadMarcas` helper) confirmed still
rendering correctly, not just `/brands` itself.

### Fixed — Closes T-38: masters URL routes are English, hard cutover,
no redirects

See `docs/RESOLVED.md`'s `T-38` entry for the full closure narrative.

Backward-compatibility strategy asked about directly, answered
directly: "hard cutover. we have backups. if anything goes very wrong,
we just undo. proceed." All 9 routes renamed with no redirect layer:
`/marcas`/`/modelos`/`/regiones`/`/monedas`/`/custodios`/`/alertas`/
`/propiedades`/`/centros-costo`/`/ubicaciones` ->
`/brands`/`/models`/`/regions`/`/currencies`/`/custodians`/
`/alert-types`/`/properties`/`/cost-centers`/`/locations`
(`/propiedades/asociaciones` -> `/properties/associations` too).

The largest single change of this whole effort by line count -- 55
route registrations, the nav-menu data structure, every href/hx-get/
redirect/masterID-prefix-parse reference across the masters handlers,
`asset.json`'s FK-popover URLs, and every affected test fixture across
7 test files. Safe as a broad whole-file substitution rather than
needing the per-entity scoping T-40's attribute-name work required,
since these are 9 genuinely distinct, non-overlapping path strings.

One real piece of dead complexity found and removed as a direct
consequence: `layout.go`'s nav-active-state check for `/alerts`
carried a special-case guard against colliding with `/alertas`' shared
`/alert` prefix. Renamed to `/alert-types`, the two paths no longer
share any prefix relationship, so the guard was removed rather than
left as confusing dead code nobody would know was safe to delete.

Verified at full scope: full Go test suite green on the first attempt,
then a genuine end-to-end test -- all 10 new routes independently
confirmed 200, all 9 old routes independently confirmed 404 (hard
cutover working exactly as decided), the rendered nav menu confirmed
entirely English, and real seeded data confirmed rendering correctly.

### Fixed — T-40 progress: Moneda and Centro de Costo attribute names
are English (`codigo_iso`/`simbolo` -> `iso_code`/`symbol`;
`codigo_contable`/`nombre` -> `accounting_code`/`name`)

See `docs/TRACKING.md`'s T-40 entry for the full narrative -- this
closes nothing, T-40 stays open, three entities in (Region, Moneda,
Centro de Costo) with Marca, Ubicación, Custodio, Alerta, and Asset
itself still ahead.

Moneda: a clean single pass, no bugs -- Region's lesson applied
proactively this time, fixing the downstream seed dict-key reference
before running anything rather than discovering it via a crash.

Centro de Costo: no entity-specific schema JSON or dynamic-form file
exists for this one (outside the original stage-2 masters set). One
real cross-file consumer found and fixed in `summary_handlers.go`.
Five test-fixture repairs needed, no runtime bugs.

Both verified with the same rigor as every entity in this effort:
full Go test suite green, then a genuine end-to-end test per entity --
real seed, real login, real list page rendering real seeded records,
a real POST through the real create form, and direct database
confirmation of the newly-created record's field names.

### Fixed — T-40 progress: Region's attribute name is English
(`descripcion` -> `description`), first entity of the larger effort

See `docs/TRACKING.md`'s T-40 entry for the full narrative and what's
still ahead -- this closes nothing, T-40 stays open, but the approach
is now settled and proven on a real entity, not just proposed.

The entity-name precedent from [0.3.27]/[0.3.28] turned out to answer
T-40's own original blocking question: since every demo checkpoint is
freshly re-seeded rather than migrated, a direct rename with no
Spanish/English bridge applies to attribute names too, not just entity
names. What remains genuinely open is scale, not approach -- proceeding
one entity at a time rather than in a single pass.

Region turned out more pervasive per-field than entity names were:
not just a xolu-storage bridge boundary, but Go struct field names,
the literal HTML form field name/id, and `region.json`'s own schema
property key. Two real bugs of my own making, both caught and fixed:
a compile error caught a missed struct-field usage inside Ubicación's
own region-dropdown code (Region and Ubicación share code in the same
files), and a live seed dry-run caught a `KeyError` in `seed_regiones()`
itself -- a downstream dict-key reference my rename regex hadn't
matched, since it targeted dict literals, not dict access.

Verified with the same rigor as the entity-name work: full Go test
suite green after five rounds of real test-fixture repair (all fixed
by exact context, since Custodio/Ubicación/Alerta share the same
`descripcion` word in the same files), then a genuine end-to-end test
covering both read and write -- real seed, real login, real
`/regiones` list page rendering five real region names, and a real
POST through the real create form landing correctly in xolu as
`{"description":"Patagonia","id":6}`.

## [0.3.28] — 2026-08-02

### Fixed — Closes T-36: service_configs.measure widened to a genuine
two-level distance/time/usage/condition classification

See `docs/RESOLVED.md`'s `T-36` entry for the full closure narrative.

Directed specifically: widen the enum, but recognize the real problem
underneath -- "we can't have all measurement units and periods under
the same category, we are conflating things... this seems to be
something begging for at least two levels of classification."

New canonical values, encoding both levels via a naming convention
(`category_unit`) rather than a schema-format change:
`distance_kilometers`, `distance_miles`, `time_hours`, `time_days`,
`usage_cycles`, `condition_temperature` -- covering every value found
across all 6 seeds (Kms/Miles -> distance, Horas/Hours -> time_hours,
both "Días desde recarga" and "Días sin visita" -> time_days, since
what differs between them is which event resets the clock, not the
unit itself, Temperatura -> condition_temperature).

New, genuinely reusable formengine capability rather than a one-off:
a `x-seam-option-groups` schema attribute, opt-in per field, that
groups `<select>` options into `<optgroup>` sections inferred from
each value's own category prefix. Deliberately not automatic for
every underscore-containing enum -- `connectivity_tier`'s
`qr_only`/`lora_mesh`/etc. have underscores with no such two-level
meaning and must keep rendering flat, exactly as before. `minty`
already had a native `Optgroup` builder method sitting unused; no
frontend library gap to work around.

Retired the field's Spanish/English value bridge entirely -- matching
the same reasoning already applied to entity names in this release:
every demo is freshly re-seeded, so the canonical value can just *be*
the stored value on both the Go and xolu-storage sides, no bridge
needed. All 6 seeds' `medida` values updated directly to the new
canonical tokens (48 occurrences).

Verified with the deepest end-to-end check in this project to date:
seeded a real xolu instance with seed-03-extinfire (the one using
`time_days`, the exact value shape T-36 originally found broken),
built a fresh `seam` binary, logged into a real running server, and
loaded the real authenticated edit form for a seeded asset. The
actual rendered HTML confirms both fixes at once --
`<optgroup label="Tiempo"><option value="time_hours">Horas</option>
<option selected="selected" value="time_days">Días</option>
</optgroup>` -- correct two-level grouping, correct Spanish
translation, and the previously-silently-blanked value now correctly
pre-selected in its real edit form.

### Fixed — Closes T-37: seeded service thresholds were invisible
through the real UI

See `docs/RESOLVED.md`'s `T-37` entry for the full closure narrative.

Every one of the 6 seed scripts wrote `service_configs` rows via a
raw PATCH using the key `"umbral"`, but the live application (both
real POST handlers, per `asset_xolu_fields.go`'s own long-standing
code comment) has only ever persisted the threshold under
`"umbral_alarma"` -- the submission field name and the actually-
persisted key genuinely differ for this one field, unlike every other
field in that bridge table. Net effect before this fix: opening any
seeded asset's real edit form and looking at Service Configurations
showed an empty threshold, even though the seed clearly intended to
set one -- and re-saving that form would have silently dropped the
seed's own `alerta_id`/`medida` pairing for the row too, since it
would reconstruct from what the (blank) form showed rather than what
was actually seeded.

Simple, mechanical fix once identified: renamed the key in all 6
seeds (48 occurrences total). Verified directly, not assumed: ran a
real seed against a live xolu instance and queried the actual stored
record -- `service_configs: [{'alerta_id': 1, 'medida': 'Horas',
'umbral_alarma': 250}]` -- confirming the key now matches exactly
what the real application's `serviceConfigsItemFromXolu` bridge reads.

## [0.3.27] — 2026-08-01

### Fixed — Entity names are identifiers too: custodios/marcas/modelos/
regiones/monedas/centros_costo/ubicaciones/alertas -> English

Narrows T-40 (docs/RESOLVED.md doesn't have this one yet -- see
`docs/TRACKING.md`'s T-40 entry for the still-open remainder).

Directed explicitly: "Entity names are a kind of identifier as well.
Custodio as a xolu entity should be Custodian. Same for any other
master tables/collections/entities." Renamed all eight remaining
Spanish xolu entity_type names to English: `custodios`->`custodians`,
`marcas`->`brands`, `modelos`->`models`, `regiones`->`regions`,
`monedas`->`currencies`, `centros_costo`->`cost_centers`,
`ubicaciones`->`locations`, `alertas`->`alert_types` (the last chosen
specifically to avoid colliding with the already-English `alerts`/
`alert_history` -- genuinely distinct entities, raised alert instances
rather than alert type definitions).

Unlike every other identifier fix this project has done, this one is
a direct rename with no bridge layer: the xolu entity_type string
itself now says `custodians`, in both Go's `xoluClient` calls and
every seed script's POST target. That's safe specifically because
every demo checkpoint this project ships gets freshly re-seeded, not
migrated -- there's no permanent production data this would orphan.
Also renamed the parallel `entity:X` source-key layer end to end:
`asset_source_resolver.go`'s dispatch map, `asset.json`'s
`x-seam-source` values, and (found by a completeness sweep, not
assumed) formengine's own self-contained test-fixture placeholders in
a separate package that use the same strings purely as generic
examples. Confirmed URL routes (`/custodios` etc., T-38) were
correctly left untouched throughout -- a different, still-undecided
naming question, deliberately not conflated with this one.

Two rounds of real test breakage, both expected and both fixed, not
patched around: `asset_source_resolver_test.go`'s mock OQL-query
fixtures and `htmx_partial_test.go`'s mock `METHOD /api/v1/entity/`
registration keys both still expected the old Spanish query/path
strings the renamed source code no longer produces.

Verified about as thoroughly as anything in this project. Beyond
`go build`/`go vet`/`go test ./...` clean: built a fresh `seam`
binary, started a real xolu instance, ran a real seed, then actually
logged into a real running Seam server and loaded the real
authenticated `/custodios` page -- confirmed actual seeded custodian
names (Constructec S.A., Fernández García Roberto, etc.) render
correctly through the fully-renamed pipeline end to end, not just a
database-level check.

### Fixed — Closes T-39: every seed's internal identifiers (not just
visible content) are now English

See `docs/RESOLVED.md`'s `T-39` entry for the full closure narrative.

Prompted directly: "All identifiers in a language other than English
are a problem." Corrected T-38's and T-39's own filed framing first --
both had recorded genuine violations as "open design questions,"
which they weren't. Then fixed T-39 itself, which grew through three
layers as each was checked for completeness:

1. `PropDef.nombre_corto` (76 terms, 103 occurrences across all 6
   seeds) -- the originally-filed scope.
2. `AssetType.identity_template`/`state_template` field keys, found
   while confirming (1) had no Go-code dependency -- the same check
   revealed a structurally identical second category (271 more
   occurrences, present in every seed since seed-01, copied forward
   into seed-06 unquestioned).
3. Sensor event telemetry payload keys (`seed_events`'/
   `seed_runtime_alerts`' `"value": {...}` blobs), found by a final
   sweep after (1)+(2) -- 155 more occurrences, ~44 new terms.

Confirmed via grep before touching anything that no Go code hardcodes
any of these specific field names in any category -- purely data-
driven throughout, making this a safe, self-contained, seed-only
rename with zero Go changes needed. Renaming `nombre_corto`'s stored
value broke every downstream `prop_defs["old_key"]` lookup in
`seed_prop_associations` -- caught by dry-run, not assumed; 132
reference updates applied to match.

Verified with two full live seeds against a real xolu instance
(seed-01; seed-03-extinfire, the most complex, 288 requests) --
100% successful, zero errors, then the actual SQLite database queried
directly to confirm `prop_definitions`, `asset_types`, and `events`
records all hold the renamed English keys, not just trusting the
seed script's own output. Two legitimate exceptions found and
deliberately left alone: `Custodio.estado`/`Asset.refrigerante` are
established xolu storage-layer field names (the same deliberate
legacy layer T-30 already covers), unrelated to the three categories
above despite similar names. That broader category -- the xolu
storage schema itself being entirely Spanish-named -- is real and
much bigger than this item; filed separately as T-40.

### Fixed — Closes T-33: PropAssoc, AssetType connectivity, and
service_configs.measure's value bridge

See `docs/RESOLVED.md`'s `T-33` entry for the full closure narrative.

Completes the tier-1 enum i18n wiring started for Alert Type and
PropDef in [0.3.26], per the tech team's explicit standing rule: every
listbox's stored values are English identifiers, every displayed
label resolves through `t()`.

**PropAssoc**, same shape as Alert Type/PropDef: `PropAssocFormPage`'s
`oblOptions`/`destOptions` now submit `required`/`optional` (matching
what `PropAssocCreateSchema` and the existing write-side bridge
already expected) with labels via new
`masters.propassoc.requirement.*`/`target_entity.*` keys.
`por_regla`/rule-based requirement was dropped rather than translated
-- `PropAssocCreateSchema` only ever validated
`Enum("required", "optional")`, so selecting it always failed at the
backend; its own "(próximamente)" label already admitted it wasn't
functional. `PropAssocsList`, `loadPropDefs`, and the previously dead
`LoadPropValues`/`PropValuesSection`/`propValueField` (fixed for
consistency, same as `DocumentLinksField` earlier) all convert on
read now via two new helpers,
`PropAssocRequirementLabel`/`PropAssocTargetEntityLabel`. Caught and
fixed one more small inconsistency while in there:
`PropAssocCreate`'s empty-value default was still the Spanish
`"opcional"` rather than the English token.

**AssetType's own `connectivity_tiers`** (`pages.go`'s
`connectivityOptions`) turned out to be the simple case -- values were
already English (`qr`/`gateway`/`lora`/`gps`, matching
`AssetTypeCreateSchema`), just needed `t()`-wrapped labels. New
`assettypes.connectivity_tier.*` keys, kept deliberately separate from
Asset's own `asset.connectivity_tier.*` (added in [0.3.26]) since
they're two different enums with two different value sets, not the
same field.

**`service_configs.measure`**, the piece that actually needed new
engine capability rather than just new keys: values (`Kms`/`Horas`/
`Usos`) had no value-bridge at all, unlike criticality/tenure_type/
decommission_reason -- only a field-*name* bridge (`measure`↔`medida`).
`translateAssetArrayValue` (`asset_xolu_fields.go`) translated array-
item *keys* but never item *values*; extended it to also translate
enum-valued items via a new per-field `enumToXolu`/`enumFromXolu`
table, added for `service_configs` only (`nil` for the three document-
links-shaped fields, which have no enum items -- zero behaviour change
for those). New canonical values `Kilometers`/`Hours`/`Uses`, new
`asset.measure.*` keys, `asset.json`'s own `measure` enum updated to
match. Two tests needed their fixture values updated as a direct,
correct consequence (`"Kms"`→`"Kilometers"`, `"Horas"`→`"Hours"`) --
written before this fix, when the schema still declared the values in
Spanish; both explicitly re-verified passing after.

**Filed rather than fixed:** T-34 (the demo's "Select UI language"
menu in `START.sh`/`START.cmd` is cosmetic --
`SEAM_FORCE_LANGUAGE` can't override an already-seeded locale, and
every seed writes one unconditionally at build time; Japanese restored
as a real menu option regardless, and `seed-06-asahikenki.py` added as
the actual working path to a Japanese demo). T-35 (formengine has no
unit-of-measurement concept -- `temp_min`/`temp_max` bake `"(°C)"`
into the translatable title text itself rather than a structured,
convertible unit, unlike the better patterns the codebase already uses
elsewhere for the same underlying problem: `purchase_price`'s
`currency_id` reference, and `service_configs.threshold`'s unit being
determined by its own sibling `measure` field).

**Verified:** every fix built/vetted/tested individually, full tree-
wide sweep confirming no remaining hardcoded old-token references
anywhere outside the bridge tables' own correct Spanish-storage side
(the two genuine hits in a final Spanish-diacritics re-sweep,
`asset_xolu_fields.go`'s `Borrowed`→`Préstamo`/`Donation`→`Donación`,
are `tenure_type`/`decommission_reason`'s own pre-existing, already-
verified bridge entries, unrelated to this pass). All four locales end
this release at 745 keys each, `diff-locales.py` clean throughout.
Full `go build`/`go vet`/`go test ./...` clean.

## [0.3.26] — 2026-08-01

### Fixed — Post-T-18 i18n follow-through: Alert Type, PropDef, and the
Asset dynamic-form schema

Continuation of T-18 Stage 9's own pattern, working through the
findings T-33 (filed this release) now tracks the remainder of.

**Alert Type, fully wired (masters_xolu_fields.go's write-side bridge
already existed and was already live -- this adds the read side):**
`AlertaMasterFormPage`'s dropdowns now submit the English canonical
values (`Maintenance`/`Security`/`Operational`,
`High`/`Medium`/`Low`) the bridge and `AlertaCreateSchema` already
expected, with labels via the existing, previously-unused
`alert_type.category.*`/`alert_type.criticality.*` keys.
`AlertaMasterEdit`'s pre-fill, `AlertasMasterList`/`loadAlertas`'s
fetch, and the list page's badge colours and delete-confirmation name
all now convert on read via two new helpers
(`AlertaTypeLabel`/`AlertaCriticalityLabel`). The downstream consumer
this rippled into -- AssetType's own "default alert" picker, which
composes a label from an Alerta's type/criticality -- is fixed too.

**PropDef, same shape:** `propTipoOptions` (a package-level var, unable
to call `t()` at init time) converted to `propTipoOptionsList(t)`,
called fresh per request; values now the English tokens
`PropDefCreateSchema` already validates, labels via the existing
`prop_definition.data_type.*` keys. `PropDefsListPage`'s badge,
`PropDefsList`'s fetch, and `PropDefEdit`'s pre-fill all convert on
read via a new `PropDefDataTypeLabel` helper.

**`asset.json`: zero Spanish left in the file, by explicit instruction
(the tech team: "asset.json should have no Spanish at all in it").** All 41
top-level field titles and all 9 document-link nested item titles were
already resolving correctly through existing
`assets.forms.asset.field.*`/`document_link_item.*` i18n keys -- the
schema's own literal `title`/`x-seam-help` text was dead, never
rendered, but still Spanish (or an English/Spanish mismatch) and
misleading to read as source. 50 literals replaced with their
resolving key's actual English value, including 3 nested
`service_configs` item titles that had no key at all yet (given plain
English: Measure/Threshold/Alert). Diffed against the pre-edit file to
confirm nothing else moved. One test needed updating as a direct,
correct consequence:
`TestAssetSchema_RequiredIndicatorMatchesRequiredArray` deliberately
renders with an empty `RenderContext{}` (no Translator) specifically
to exercise the schema's own literal fallback text -- its two
assertions now expect English. Its sibling test,
`TestAssetSchema_I18nContent_EndToEnd`, renders with a real translator
and locale=es and correctly expects `"Nombre *"` there -- verified
untouched and still passing, since it tests the live i18n system, not
the schema's literal.

**Two real, live bugs found during that same audit, both fixed:**

- `status`'s enum was silently half-broken: `active`/`lost` resolved,
  `inactive`/`maintenance` did not (no matching i18n keys existed).
  Added both. `connectivity_tier` was *entirely* unresolved -- all
  four values (`qr_only`/`gateway`/`lora_mesh`/`gps_cellular`). An
  orphaned `asset.connectivity.*` key set existed but under the wrong
  field name *and* wrong value spellings (`qr`/`gps`/`lora`), written
  against an earlier, since-corrected version of this schema and never
  updated. Renamed (not recreated) to `asset.connectivity_tier.*`,
  preserving the existing refined translations exactly, including a
  Japanese formatting nuance from an earlier review pass.
- `service_configs.measure` (nested inside the `repeatable` widget)
  was structurally unable to resolve any i18n key at all, for a
  different reason than a missing key: `repeatableRow` rewrites each
  item's `Field.Name` to the array-bracket submission form (e.g.
  `service_configs_measure[]`) before rendering, and
  `resolveEnumOptionLabel` was building its lookup key straight from
  that mangled name. Fixed at the engine level: added `Field.I18nName`
  to carry the item's true, original name separately from the
  submission-form `Name`; `repeatableRow` sets it before the rewrite;
  a new `i18nFieldName()` helper in `internal/formengine/i18n.go`
  prefers it when set. Zero behaviour change for every field outside a
  repeatable row (`I18nName` stays empty, falls back to `Name`
  exactly as before).

**One more real bug, functional rather than cosmetic, found while
tracing the above:** `service_configs`' item schema named its alert-
reference property `alerta_id` (Spanish) -- but the actual dynamic-form
POST parser (`asset_dynamic_post.go`) and the xolu persistence bridge
(`serviceConfigsItemToXolu`/`FromXolu`, `asset_xolu_fields.go`) both
already, correctly expected `alert_id` (English), matching this
package's own stated rule ("All 42 property names are English").
Because `repeatableRow` builds the submitted HTML field name straight
from the schema's own property name, the mismatch meant **a service
interval's linked Alert was silently dropped on every save through the
dynamic Asset form** -- not a display bug, a data-loss bug. Renamed the
schema property; the parser and bridge already expected the fix, so no
other code changed.

**Left open, filed as T-33:** `service_configs.measure`'s own enum
values (`Kms`/`Horas`/`Usos`) are still Spanish -- unlike every other
Asset enum field, there's no existing value-bridge for this one (only
a field-*name* bridge), and seed data already has `"medida": "Kms"`
persisted, so fixing the values needs a new asymmetric bridge built
from scratch, not just new keys. PropAssoc's own dropdowns, list,
fetch, and its downstream `propValueField` widget-type switch are the
same shape as Alerta/PropDef's fix but not yet done. AssetType's own
`connectivity_tiers` dropdown (`pages.go`) still hardcodes English
labels with no `t()` wrapping.

**Verified:** every fix built/vetted/tested individually, full
`go build`/`go vet`/`go test ./...` clean throughout. All four locales
end this release at 738 keys each (731 at 0.3.25), `diff-locales.py`
clean.

## [0.3.25] — 2026-08-01

### Fixed — T-18 Stage 9 (residual sweep): closes T-18

See `docs/RESOLVED.md`'s `T-18` entry for the full closure narrative.

Tree-wide sweep for hardcoded strings missed by Stages 1-8, per the
plan's own Stage 9 scope ("hardcoded Spanish string missed by Stages
1-8; empty-states and error messages; full page-by-page live check").
The sweep itself found the scope was larger than "residual" implied --
several self-contained areas (SeamPopover summary cards, the dynamic
Asset form's error paths, Login/Profile/Password Reset) had never
been named in any of Stages 1-8 and were still 100% untranslated.
Fixed all of it under T-18 rather than splitting into a new item,
since T-18's own charter ("wire i18n into the rest of the UI --
currently zero call sites outside formengine") already covers it.

**Within Stages 1-8's own named areas:**
- Dashboard's upcoming-expirations widget: `"Expiración"`/`"Fin
  garantía"` were stored as literal Spanish directly in
  `DashVencimientoRow.Campo`, bypassing the rest of the widget's
  correct i18n. Now stores English tokens (`"expiration"`/
  `"warranty_end"`), translated at render time.
- `RenderError`/`RenderPublicError`/`statusText` (the app-wide error
  page chrome): hardcoded `"Go Home"` button and `"Error"` fallback
  title, used by every error page regardless of locale. Now resolves
  `t` from `h.Config.Translator`/`Locale` directly.
- 40 call sites (`masters_routes.go`, `routes.go`) passed a literal
  English title (`"Error"`, `"Bad Request"`) alongside an already-
  translated message -- half-translated error pages. Unified via two
  new keys (`error.generic`, `error.bad_request`).

**Genuinely new ground, never named in Stages 1-8:**
- `summary_handlers.go` (SeamPopover cards for Custodio/Marca/Modelo/
  Ubicacion/CentroCosto/AssetType): 100% hardcoded Spanish, zero
  `t()` calls anywhere -- every label, the Activo/Inactivo value, the
  empty state, both action buttons. Full rewrite; renamed a `for i, t
  := range` loop variable to `tier` to avoid shadowing the translator,
  the same collision shape Stage 6 hit first.
- Dynamic Asset form POST handlers (`asset_dynamic_post.go`,
  `asset_dynamic_handlers.go`) and the static Asset routes'
  "not found"/list-title paths: entirely hardcoded English error
  paths and titles.
- Alert action handlers (`AlertAcknowledge`/`AlertResolve`) and the
  auth/permission error pages (`Error403`/`ErrorCSRF`): hardcoded
  English throughout.
- Modal-based entity forms (`masterPartialFormWrapper`, 11 call sites
  including Sensors): `"Cancelar"`/`"Guardar"` hardcoded with no `t`
  parameter at all, while the non-modal path was already correctly
  localized -- the same entity showed different languages depending
  on which edit path was used.
- `asset_detail.go` (the Asset detail/view page): zero `t()` calls.
  The "Editar" button opened a modal titled `'Editar Activo'`
  (Spanish) sitting next to an English "Delete"/"Confirm Delete" --
  mixed language on one row.
- `DocumentLinksField`: hardcoded Spanish placeholders and add-link
  label. Fixed for consistency, though this component turned out to
  be dead code (defined, never called from any live route) --
  flagged as such rather than implied to be a live fix.
- **Login, Profile, and Password Reset: three entire pages, zero
  `t()` calls each.** These render their own standalone `<html>`
  documents outside `BaseLayout` and were never touched by any of
  T-18's eight stages. Fixed page chrome, all form labels/
  placeholders/buttons, every error message (including `LoginSubmit`'s
  URL-redirect-encoded error messages), the `<html lang>` attribute
  (previously hardcoded `"en"` regardless of locale, now dynamic —
  same bug as the visible text, just in an attribute instead), and a
  genuine data bug found along the way: Profile's own "Last login"
  field was hardcoded to the literal string `"Today"` rather than
  reading `last_login_at`, unlike the Admin Users list's own correct
  `formatTimeAgo` usage two lines of precedent away.

**Filed rather than fixed:**
- T-31 (P3): `urlEncode()`'s naive escaping doesn't properly
  percent-encode non-ASCII bytes; translated error messages now flow
  through it in `LoginSubmit`, same as the pre-existing `lockMsg`
  usage, so this isn't a new gap, just a newly-relevant one worth
  tracking.
- T-32 (P4): `password_reset.go`'s standalone page titles hardcode
  "Seam AMS" instead of reading `h.config.AppName`, unlike
  `login.go`'s own title, which does. Cosmetic, no user report.

**Verified throughout:** every fix built/vetted/tested individually
(never batched untested), plus a final tree-wide sweep confirming zero
remaining `RenderError`/`Render` calls with hardcoded titles anywhere
in `internal/ui`, and the Spanish-diacritics sweep re-run clean (the
17 remaining hits are all previously-verified deliberate patterns:
the criticality/tenure_type/decommission_reason xolu bridge table,
PropDef/PropAssoc's documented Stage-4 enum-value scoping decision,
`ListActions`'s nil-safe fallback, the language endonym map, and the
diacritic-stripping slugify utility). All four locales end at 731
keys each (started this stage at 634), `scripts/diff-locales.py`
clean throughout. Full `go build`/`go vet`/`go test ./...` clean.

## [0.3.24] — 2026-08-01

### Fixed — 113 Spanish-language i18n key names renamed to English

Identifiers, not values: `internal/i18n`'s translation *keys* (not the
translated text) contained Spanish-language segments —
`masters.custodio.*`, `masters.marca.*`, `assets.forms.asset.section.
basico`, and 110 others — inherited from before T-18 and extended
without question across all eight of T-18's own stages. A real,
non-negotiable-law violation (`docs/KNOWN_ISSUES.md`'s "internal vs.
external" rule: identifiers must always be English, no exception) that
went unflagged for the entire session until raised directly.

`scripts/rename_spanish_keys.py` (kept in the repo, not a throwaway):
renamed all 113 keys across all four locale JSON files (values
untouched — confirmed these are pure identifiers before doing
anything), every Go call site (194 replacements across 4 files), and
`asset.json`'s own tab/section `label` fields, which store the i18n
key as a literal string value rather than a Go-constructed one —
confirmed by reading `formengine/i18n.go`'s `resolveLabel` first;
missing this would have silently broken section titles with no build
error to catch it. `masters.propdef.*`/`masters.propassoc.*` kept
as-is (already English abbreviations); only their Spanish field-level
segments were renamed. `region` (singular, identical spelling in both
languages) left alone; `regiones` (the plural, which differs from
English) renamed.

Filed T-30: `internal/formengine/schemas/masters/{region,moneda,
marca}.json` embed raw literal Spanish text directly rather than using
any i18n key — a related but structurally different violation (no key
exists to rename), found while scoping this fix. Confirmed via
`masters_pages_dynamic.go`'s own doc comment that this path isn't
wired into any live route (dev/test parity-proof only) — genuinely out
of user-facing scope today, but worth fixing before that path is ever
made live.

Verified: zero old-key references remain anywhere (locale JSON, Go
source, schema file); all four locales still at identical key sets
(634 each, `scripts/diff-locales.py`); full build/vet/test suite
clean; live-verified in Spanish across nav, Custodio, Brand, Alert
Types, and Asset form sections — all render exactly as before, since
only identifiers changed, not displayed text.

## [0.3.23] — 2026-07-31

### Fixed — Japanese locale review feedback (3 of 22 points had merit)

External review of `ja.json` raised 22 points. Most were either
confirmations that an existing choice was already correct, or based
on a misunderstanding of how the key/value i18n model works (flagging
Spanish-named *keys* like `masters.custodio` as a translation defect
— keys must stay identical across every locale for lookup to function
at all; not a Japanese-localization issue). Three points held up and
are fixed here:

**Custodian/Administrator collision** — both concepts translated to
the identical 管理者, a real ambiguity since they're genuinely
different things in this app (an asset's responsible party vs. a
system permission role). Found a real, pre-existing key collision
while fixing this: `field.name` was shared between object-context
forms (Sensors list, Rules, Asset Types) *and* the Users form's own
person-context Name field — couldn't fix Custodian/Administrator's
"名前 vs 名称" distinction without first splitting this, since users
explicitly need to keep 名前. Added `users.field.name` as its own key
(all four locales, same value as `field.name` had), repointed
`admin_users.go`'s own Name field to it, verified clean before
touching any translation. Custodian's 10 affected keys now use
保管責任者, disambiguated from Administrator's (correctly unchanged)
管理者.

**GPS セルラー → GPS／セルラー** — more natural compound-term wording.

**名前 → 名称 for object-context name fields** — applied to Asset,
Brand, Cost Center, Location (including its short-name field →
短縮名称), and Property (same) — not applied globally, only to the
specific fields discussed. Users' own Name field correctly stays 名前
via the new split key.

One related pre-existing gap noted, not fixed here (out of scope for
this specific task): `asset.connectivity.gps` is an orphaned key,
never actually called from any Go rendering path — the Asset Type
form's own connectivity checkboxes are hardcoded plain strings that
bypass i18n entirely. Candidate for T-18 Stage 9's residual sweep.

Verified live end-to-end in Japanese: nav, Custodio pages, Admin Users
(unaffected), Users' own form (name field correctly still 名前), Asset
form, and every affected Masters list/form (Brand, Cost Center,
Location, Property). Full build/vet/test suite clean, all four
locales confirmed still at identical key sets (634 each) via
`scripts/diff-locales.py`.

## [0.3.22] — 2026-07-31

### Added — Japanese locale (ja)

`internal/i18n/locales/ja.json`, all 633 keys translated (professional
business register, です/ます polite form, standard IT/asset-management
terminology). An experiment in the system's own claimed extensibility
more than a formal feature request — and it held up completely: adding
a fourth, previously-unanticipated locale required **zero** changes to
`Translator`, the Settings language dropdown, or the locale-validation/
persistence path. `New()` discovers locale files generically from the
embedded directory; `AvailableLanguages`/`ValidateLocale` are equally
generic downstream. Only one test needed updating —
`TestNew` asserted an exact count of 3 locales, which is a correct
assertion to have to update when a locale is genuinely, deliberately
added, not a bug it caught.

This also retroactively confirms the earlier "pt doesn't work"
diagnosis: the identical mechanism now demonstrably handles a locale
that was never specifically built for, which wouldn't be possible if
there were a real hidden defect — the earlier failures really were
test-harness mistakes, not product bugs.

`scripts/diff-locales.py` rewritten to auto-discover locale files
(`glob` the directory) rather than hardcoding `("en", "es", "pt")`, so
it stays useful as the locale set grows instead of needing a matching
edit every time. Confirms all four locales — en/es/pt/ja — have
exactly identical key sets (633 each).

Verified live end-to-end: 日本語 correctly offered in the Settings
dropdown, locale switch succeeds, and Dashboard/nav/Asset form all
render correctly in Japanese after switching. Full build/vet/test
suite clean.

## [0.3.21] — 2026-07-31

### Added — scripts/diff-locales.py; fixed pre-existing pt.json gap

`scripts/diff-locales.py`: compares the three `internal/i18n/locales/
*.json` key sets against each other, confirming any "missing" set is
consistent across references rather than a partial, inconsistent mix.
Key presence only, not translation quality.

Running it for the first time found `pt.json` short by 64 keys
relative to the (mutually identical) `en.json`/`es.json` — all
`assets.forms.asset.field/section/tab.*` (formengine's own dynamic
Asset-form keys) and `core.errors.*` (validation messages), predating
T-18 entirely, from the earlier T-22/T-23 formengine work. Confirmed
the missing set was identical whether compared against en or es (not
a partial mix), and that pt had zero keys absent from both (a clean
subset, not a divergent branch) before backfilling. Translated all 64
and merged them in, guarded by an assertion that the translated set
was exactly the missing set — no more, no less — before writing.
`diff-locales.py` now reports all three locales at 633/633/633,
identical.

Verified via a targeted `Translator` resolution test (the same
technique as Stage 2's Sprintf check) after seed-script defaults and
CSRF mechanics made a clean live Portuguese session impractical to
arrange quickly in this sandbox. Lower risk than a live check would
have needed to cover anyway — this adds values to an already-proven
lookup mechanism, not new Go code. Full build/test suite clean.

## [0.3.20] — 2026-07-31

### Added — T-18 Stage 8: Dashboard, Events, Map, Alerts, FSM Editor, QR Scan i18n

The broadest single stage — six genuinely distinct areas. Same
discovery as Stages 6-7 repeated: Map and Events were 100% hardcoded
English (same shape as Sensors/Rules); Dashboard and Alerts were 100%
hardcoded Spanish underneath. 79 new keys.

A third bug shape, distinct from the two found earlier: Dashboard's H1
literally hardcoded "Dashboard" in English while the nav (Stage 1)
correctly resolved to "Panel" for Spanish — not a missing translation
but a genuine mismatch between two pieces of chrome on the same
screen. Verified live: before the fix, a Spanish tenant saw nav
"Panel" sitting directly above content H1 "Dashboard"; after, both
correctly say "Panel". Alert Detail repeated Stage 6's
"mostly-English-with-several-leaks" shape (Fuente/Región/Custodio/✓
Reconocer/Volver a Alertas all mixed into an otherwise English page).

Two scoping decisions made explicit: FSM Editor's actual UI lives
entirely in an external Lit/JS web component, confirmed by reading the
Go rendering function itself (almost no text beyond one fallback
title) — translating the editor's internal UI would mean editing
vendored JavaScript, out of scope, same kind of call as the enum-value
decisions in Stages 4-5. Map's status values (active/alert/silent/
lost) were confirmed to be a genuinely different concept from
Dashboard's own status breakdown (active/inactive/maintenance/lost) —
sensor/connectivity state versus asset lifecycle state — verified by
checking actual stored values before reusing keys, not assumed from
the word "status" alone.

Verified live in both languages against seeded demo instances,
including QR Scan's public error paths confirming the locale mechanism
works correctly for unauthenticated requests. Alert Detail's specific
keys were confirmed via a temporary in-package i18n test after live
alert-instance creation proved impractical to arrange quickly — the
same verification technique used for Stage 2's Sprintf formatting.
Full build/vet/test suite clean.

Stage 8 of 9 in `docs/T18-I18N-UI-COMPLETION-PLAN.md`.

## [0.3.19] — 2026-07-31

### Added — T-18 Stage 7: Settings, Admin/Users i18n

Same shape as Stage 6 — Settings and Users weren't mostly-correct with
a few leaks, they were entirely hardcoded (Settings: 100% English;
Users: 100% English except one field). Verified live both ways:
Settings' section cards, field labels, and buttons now genuinely
render in Spanish for a Spanish tenant, not just English regardless of
locale.

One bug found here is a different shape than anything found so far:
the "Asset Map" feature toggle on the Settings page had its label and
description hardcoded in Spanish — meaning it showed Spanish **even on
the English demo**, not just missing translation for Spanish tenants.
Confirmed live before fixing. This is the mirror image of the
Sensors/Rules bug (hardcoded-English-showing-in-Spanish) — together
they confirm one underlying pattern (pages built without touching
i18n at all, regardless of which language happened to be hardcoded)
rather than two unrelated issues.

A second real, pre-existing bug: the Users list's own `ListActions`
call never set `T`, so its Edit/Delete row buttons were already
silently showing Spanish ("Editar"/"Eliminar") on an otherwise 100%
English page — a bug that predates this session, just never surfaced
until this stage's audit.

A third `t`-collision matching Stage 6's pattern: `formatTimeAgo`'s own
`time.Time` variable was named `t`. Renamed the variable, not the
translate-func parameter, consistent with how prior collisions were
resolved.

Explicit scoping decision: normal user-facing content (titles, field
labels, buttons, validation messages seen on correct form submission,
confirmations, relative-time strings) was translated in full.
Exceptional/malformed-request paths (`http.Error()` calls for an
unparseable user ID, an unauthenticated session, a generic form-parse
failure) were deliberately left as plain English — reached only
through direct API manipulation, not normal UI flow.

Verified live in both English and Spanish against seeded demo
instances: Settings' six section cards and all fields, the Users list
(including the newly-fixed row-action buttons), the Add/Edit User
forms, and relative-time formatting. Full build/vet/test suite clean.

Stage 7 of 9 in `docs/T18-I18N-UI-COMPLETION-PLAN.md`.

## [0.3.18] — 2026-07-31

### Added — T-18 Stage 6: Sensors, Rules, Asset Types i18n

Turned out substantially larger than planned. These three entities
weren't mostly-correct English with a few Spanish leaks — they were
100% hardcoded English with zero `t()` calls on anything beyond the
bare page title. For a Spanish-locale tenant this meant the nav
correctly said "Sensores"/"Reglas"/"Tipos de Activo" (Stage 1) while
every page underneath still showed "Sensors"/"Rules"/"Asset Types" —
the same mixed-language bug already caught twice this session, just
inverted and therefore invisible from an English-only perspective.
Verified live: before this stage a Spanish tenant's Sensors page
showed "Name"/"External ID"/"Type" in English; after, it correctly
shows "Nombre"/"ID Externo"/"Tipo".

Four genuine bugs also found and fixed along the way: a stray
`"Acciones"` column header in both the Sensors and Rules tables (the
one Spanish word in otherwise-English tables), a hardcoded
`'¿Eliminar sensor?'` confirmation on the Sensor detail page, and an
entire "Configuración de Alerta" section on the Asset Type form that
had never been translated.

~94 new keys — the largest stage yet. Two real naming collisions
surfaced (an `AssetTypeRow` loop variable and a `fieldTypes` loop
variable both happened to be named `t`) and were fixed by renaming
the colliding local variables rather than picking a different name for
the translate function, keeping `t` consistent throughout the
codebase. One JS-embedded string (the field-row "Required" checkbox
label, generated client-side inside a raw JS template) was handled via
Go string injection into the template, the same technique used
elsewhere for dynamic JS content.

One real mistake caught mid-work: initially collapsed Rules' "Delete
Rule" button to the generic `action.delete` key, silently losing its
entity-specific wording. Caught by comparing against the original
before moving on.

Verified live in both English and Spanish against seeded demo
instances: all three list pages, all three forms, the Sensor detail
page's confirm-dialog, the Asset Type form's full multi-section layout
including Alert Configuration. Full build/vet/test suite clean.

Stage 6 of 9 in `docs/T18-I18N-UI-COMPLETION-PLAN.md`.

## [0.3.17] — 2026-07-31

### Added — T-18 Stage 5: Masters (PropDef, PropAssoc, Ubicaciones) i18n

Completes the Masters module entirely. All three entities' page
titles, list headers, field labels, empty-states, and error messages
no longer hardcode Spanish. Largest single stage so far (40 new keys)
— PropDef carries its own uniqueness-violation messages, with genuinely
different wording for create vs. update ("a property" vs. "another
property") kept distinct rather than collapsed into one message.

Same enum-values-stay-untouched scoping decision applied again:
PropDef's data-type options and PropAssoc's target-entity/requirement
options remain raw Spanish, consistent with Stage 4's reasoning for
Custodio and Alerta.

One deliberate exclusion: `PropValuesSection` also renders "Propiedades
Personalizadas" but in a different context entirely — it displays
custom property *values* on the Asset form, not the PropDef admin
page. Confirmed by checking its actual callers before excluding it,
left for a later stage.

PropAssoc has no edit/update handler by original design (create/delete
only) — its form title needed no IsEdit branching, unlike every other
masters entity.

Verified live in both English and Spanish against seeded demo
instances: all three list pages, all three create forms, edit-modal
titles. Full build/vet/test suite clean.

Stage 5 of 9 in `docs/T18-I18N-UI-COMPLETION-PLAN.md`.

## [0.3.16] — 2026-07-31

### Added — T-18 Stage 4: Masters (Custodio, Alerta) i18n

Both entities' page titles, list headers, field labels, empty-states,
and error messages no longer hardcode Spanish. A third shared
component needed the same fallback-safety fix as Stage 2's two:
`masterFormSelect` (used by Custodio, Alerta, and PropAssoc's
not-yet-reached form) had its own hardcoded placeholder, fixed the
same way — literal Spanish text when `t` is nil, not a raw key.

A scoping boundary made explicit rather than assumed: enum option
*values* stay untouched. Custodio's document-type codes
(DNI/CI/RUT/RUC) and Alerta's type/criticality values
(Mantenimiento/Seguridad/Operativa, Alta/Media/Baja) remain stored as
raw Spanish, unlike Asset's own criticality/tenure_type/
decommission_reason, which T-22/T-23 migrated to English canonical
tokens earlier this session. Migrating these entities' stored values
the same way is a data-model change, not a UI-string one — out of
scope here. Only field labels and chrome were translated.

Reused `assets.forms.asset.field.criticality` for Alerta's own
Criticidad label rather than duplicating an already-established shared
key.

Verified live in both English and Spanish against seeded demo
instances. Full build/vet/test suite clean.

Stage 4 of 9 in `docs/T18-I18N-UI-COMPLETION-PLAN.md`.

## [0.3.15] — 2026-07-31

### Added — T-18 Stage 3: Masters (Marca, Modelo) i18n

Both entities' page titles, list column headers, field labels,
empty-states, and error messages no longer hardcode Spanish regardless
of locale. Reused the shared-component fixes from Stage 2
(`ListActions`, `masterFormButtons`) directly, with no further changes
needed there.

Modelo's Marca foreign-key dropdown (hand-built, not
formengine-driven) carries three distinct pieces of translatable text
in different contexts — the field label ("Marca *"), the empty-option
placeholder ("— Seleccionar Marca —"), and the list column header
("Marca", no asterisk) — kept as three separate keys rather than
forcing one shared key across genuinely different current strings.

Preserved a real distinction in the original Spanish rather than
flattening it for consistency: Marca's delete-guard message uses "is
in use by" while Modelo's uses "is assigned to" — different verbs in
the source text, kept different in English too.

Verified live in both English and Spanish against seeded demo
instances: list pages, create/edit forms (including all three pieces
of the Marca dropdown), edit-modal titles, and row-action buttons.
Full build/vet/test suite clean.

Stage 3 of 9 in `docs/T18-I18N-UI-COMPLETION-PLAN.md`.

## [0.3.14] — 2026-07-31

### Added — T-18 Stage 2: Masters (Region, Moneda, Centro de Costo) i18n

All three entities' page titles, list column headers, field labels,
empty-states, and — a deliberate scope expansion beyond the original
plan — their own per-entity error messages (invalid form data,
not-found, create/update failures, delete-blocked-by-reference) no
longer hardcode Spanish regardless of locale. Skipping the error
messages would have left the same "mixed language on failure" pattern
already caught twice this session; included them rather than
deferring to Stage 9.

Two genuinely shared components were fixed here, benefiting every
future stage for free: `ListActions` (the row-action Edit/Delete
buttons and delete-confirmation dialog — used by *every* entity's list
page, including Assets, whose own row buttons this retroactively
fixes) and `masterFormButtons` (Save/Cancel, used by every master
form). Both needed a real safety fix: since they're shared with
entities T-18 hasn't reached yet, falling back to a raw key string
when `t` is nil (this layer's usual convention) would have visibly
broken every untouched entity's UI. Fixed by falling back to the
*original literal Spanish text* instead, matching formengine's own
safer convention — confirmed this preserves every not-yet-updated
caller's exact prior behaviour.

One real inconsistency in the original Spanish text fixed as a side
effect: Centro de Costo's edit modal said "Editar Centro" while its
own form page said "Editar Centro de Costo" — unified to one
consistent translation everywhere.

Verified live in both English and Spanish against seeded demo
instances: list pages, create/edit forms, field labels, buttons, and
edit-modal titles. The one new key using Sprintf-style `%d` formatting
(the delete-blocked message) was verified via a temporary in-package
test exercising the same code path the live server calls, rather than
a live HTTP click — CSRF mechanics made that impractical to arrange
cleanly in this session. Full build/vet/test suite clean throughout.

Stage 2 of 9 in `docs/T18-I18N-UI-COMPLETION-PLAN.md`.

## [0.3.13] — 2026-07-31

### Added — T-18 Stage 1: nav sidebar i18n

The nav sidebar — visible on every single page — no longer hardcodes
Spanish (mostly) or English (a couple) regardless of locale.
`Config` gained a `Translator *i18n.Translator` field, set once in
`NewUIHandler`; `BaseLayout` resolves a translate function once per
request and threads it into `MainNav`. `cfg.Locale`'s correctness was
verified directly (not assumed) before relying on it — confirmed
`h.currentLocale()` and `h.page.Config.Locale` are written together
under the same lock everywhere the tenant's locale changes.

6 of the 24 needed keys already existed but were unused anywhere in
the code — reused rather than duplicated, picking up an already-better
Spanish translation for free (`nav.dashboard` = "Panel", not the
literal "Dashboard" the hardcoded nav actually showed before this).
18 new keys added across en/es/pt.

Verified live both ways: the English demo shows all 20 nav items and
4 section headers correctly in English; re-ran against the Spanish
demo and confirmed it renders identically to before, plus the
Dashboard-label improvement.

Stage 1 of 9 in `docs/T18-I18N-UI-COMPLETION-PLAN.md`.

## [0.3.12] — 2026-07-31

### Added — T-18: staged i18n UI completion plan

`docs/T18-I18N-UI-COMPLETION-PLAN.md` — a 9-stage plan for wiring i18n
into the rest of the UI, each stage sized to be completable in one
pass, following the same shape as the existing
`docs/RBAC-COMPLETION-PLAN.md` precedent.

Two of T-18's three original open questions are already resolved as
side effects of this session's own work, not re-opened here: the
per-request locale mechanism (`h.translateFunc()`/`h.currentLocale()`,
already methods on every `*UIHandler`, proven against real persisted
tenant locale by both the widget-chrome fix and the Assets list page
fix) and the naming convention (reusing exactly what those two fixes
already established by necessity).

Real, counted scope, not the original filing's estimate: 73 distinct
page-title strings, 24 nav labels, 19 master-form field labels, 5
select labels, from the files checked directly. Masters confirmed as
the dominant remaining body of hardcoded Spanish; Sensors, Rules,
Asset Types, Settings, and Admin already have English page titles
(their field-level content is unverified, not assumed clean — folded
into the plan's own Stages 6–7).

## [0.3.11] — 2026-07-31

### Removed — T-01/FE-24: static Asset create/edit form retired

The session's actual milestone: we gave direct, hands-on
production validation ("It opens, modifies, saves correctly... Assets
is good enough to give it the greenlight") — the real bar
`FORM_ENGINE_SPEC.md` §13.3 sets for retiring the static form, not a
scripted walkthrough. Two genuine i18n gaps surfaced during that real
use and were fixed in the same session (document-links/repeatable
widget chrome; the Assets list page's own hand-coded rendering path).

Retired per the tech team's explicit instruction: back the static form up to
`/attic/static-asset-form/` first, as a reference of what the codebase
looked like before, then remove it carefully — not an outright
deletion. Every function, type, and route the static form touched was
checked for external call sites across the whole `internal/ui`
package before anything was removed, not assumed safe from name or
file location. Two genuinely shared dependencies were found and kept:
`assetDynamicFields`/`assetDynamicField` and the `GET
/assets/type-fields` endpoint (`AssetTypeFields`) — the *dynamic*
form's own `type_id` select widget targets this exact same route to
reload identity/state fields on change; removing it would have broken
the dynamic form, not just retired the static one. Also kept:
`conditionalButtonText` (shared with the dynamic form's own wrapper
and the Rules page), `AssetTypeOption` (used broadly across the list
page, dashboard, and unrelated forms), and
`validation.AssetCreateSchema`/`AssetUpdateSchema` themselves (the
REST API validates against these directly, independent of the UI
layer entirely).

Removed: 4 routes, 5 handlers, `AssetFormPage` and 15 widget helpers
exclusive to it, 2 local schema aliases, `AssetEnumValueFromXoluValue`
(its underlying translation table stays — the dynamic form's own read
path uses it independently), and 4 test functions. One real fix needed
mid-removal: `AssetDetail`'s `ServiceConfigs` field became orphaned
once its type went with the form — confirmed nothing on the read-only
view page populated or read it, removed the field rather than
resurrecting the type for no reason.

Verified live afterward: old static routes correctly 404; the
read-only Asset detail/view page — never in scope, untouched by
design — still renders correctly; the dynamic create/edit form still
fully works; the Assets list page still works; the REST API still
responds correctly. Full build/vet/test suite clean throughout, zero
regressions. See `docs/formengine/RESOLVED.md`'s `FE-24` entry and
`/attic/static-asset-form/README.md` for the complete narrative.

**T-01 (implement the dynamic form engine) closes as a direct
consequence** — its own closure gate was exactly this.

## [0.3.10] — 2026-07-31

### Fixed — Assets list page mixed languages regardless of locale

Found by the tech team directly, again hands-on: "The only mixed language I
saw remaining was in the Assets listing." A genuinely different
rendering path from everything fixed so far this session — the list
page (`internal/ui/assets.go`) is hand-coded HTML, not a formengine
schema-driven form, so none of the T-22/T-23 or document-links work
touched it at all. Confirmed directly: the page title and empty-state
text were already English (some earlier i18n coverage existed), but
every column header ("Activo", "Tipo", "Estado", "Criticidad",
"Custodio", "Acciones") and the New/Edit Asset labels were hardcoded
Spanish unconditionally — a visibly jarring mix on a single page.

Threaded a translate function through `AssetsListPage` (`t
func(key string, args ...any) string`, `h.translateFunc()` from the
caller), reusing the exact same pattern `asset_dynamic_form_wrapper.go`
already established for its own hand-coded chrome. Column headers
reuse the dynamic form's own existing `assets.forms.asset.field.*`
keys directly — "Criticality" means the same thing whether it labels a
form field or a list column, so reusing avoids maintaining the
translation twice. `asset.new`/`asset.edit` already existed with the
right values; only `asset.list.actions` ("Actions") was genuinely new.

Verified live both ways: the English Fairborne demo now shows
Name/Asset Type/Status/Criticality/Custodian/Actions and a "New Asset"
button; re-ran against the original Spanish Constructec demo and
confirmed it renders identically to before (Nombre/Tipo de
Activo/Estado/Criticidad/Custodio/Acciones, "Nuevo Activo") — same
Spanish text, now resolved through i18n instead of hardcoded.

## [0.3.9] — 2026-07-31

### Fixed — Asset form still mixed languages in a few spots the T-22/T-23 pass didn't reach

Found by the tech team directly, hands-on, testing the Mac demo: "a few
elements in the Assets form are still mixing languages." Confirmed
live rather than guessed at — grepped `widgets.go`/`repeatable.go` for
hardcoded non-English strings and found four real, concrete spots, all
in the same widget-code family T-22/T-23 touched for enum selects but
never extended to these other widget types:

- Document-links row placeholders (`document_links`, `lifecycle_links`,
  `loan_links`) always read "Nombre" / "URL" / "Tipo (opcional)",
  regardless of locale — `item.Title` was read straight from the
  schema, bypassing i18n resolution entirely.
- The document-links "Add" button always read "+ Agregar enlace".
- The generic repeatable-row "Add" button (used by `service_configs`)
  always read "+ Agregar".
- The `parent_id` field's asset-picker MVP stopgap note was entirely
  hardcoded Spanish prose.

New `resolveDocumentLinkItemLabel` and `resolveWidgetChrome`
(`internal/formengine/i18n.go`) close this the same way every other
resolver in that file does — graceful literal fallback, no behaviour
change for anyone not using these specific translation keys. Six new
en/es/pt keys added. Re-verified live against both demos: the English
Fairborne seed now shows "Name"/"URL"/"Type (optional)" and "+ Add
link"/"+ Add"; the original Spanish Constructec seed is unchanged
("Nombre"/"URL"/"Tipo (opcional)", "+ Agregar enlace"/"+ Agregar") —
confirming this closes the gap without touching what already worked.

## [0.3.8] — 2026-07-31

### Fixed — scripts/build-demo.sh used sed, against this project's own absolute rule

0.3.7's fix for the hardcoded-credentials bug (above) used `sed -i` for
the file substitutions and `sed -n` for line extraction. We caught
it directly: "Never use sed" is an unconditional rule here, not one
with an carve-out for sed embedded inside an authored script rather
than run ad hoc — the failure modes the rule exists to avoid (multiline
corruption, platform-dependent Unicode handling) are properties of sed
itself, regardless of who invokes it or when. Replaced both `sed -i`
substitution calls and the `sed -n` line-extraction with plain Python,
matching the guarded-substitution pattern this project already uses
everywhere else. Rebuilt and re-verified the seed-05 demo package end
to end afterward — identical correct output (`README.txt`/`START.sh`
show the right Fairborne credentials), confirming the rewrite changed
nothing but the implementation. Every other `sed` call in
build-demo.sh predates this session and was left untouched — rewriting
pre-existing, unrelated code wasn't asked for and would have been its
own scope creep.

## [0.3.7] — 2026-07-31

### Added — seed-05-fairborne.py: English-content demo seed

New seed (Fairborne Equipment LLC, a fictional Pacific Northwest
construction-equipment company) built for the tech team's direct request:
"give me a demo for Mac I can use in English". Same proven structure
as seed-01-constructec.py — 16 assets, 10 sensors, 5 rules, 2 FSM
machines, full master data — every user-visible string translated to
English. Enum-constrained storage values (`criticidad`,
`calidad_tenencia`, `tipo_sujeto`, etc.) deliberately keep their exact
Spanish tokens, matching xolu's real storage format — translating
those would just break validation, not make anything more English.
`tenant_settings.locale` set to `en`. Admin accounts use neutral
placeholder credentials
(`admin@fairborne-equipment.example`/`ops@fairborne-equipment.example`),
not any real person's name or email.

`tipo_sujeto` (Custodian's tax-ID-type field) is still DNI/CI/RUT/RUC
only (T-28) — RUT/CI used here as the closest structural analogue for
a US company, not a genuine fit. Confirmed live: real seeded data
renders correctly through the dynamic Asset form's translation layer
(`value="High"`, label `"High"`, etc.) — the same mechanism T-22/T-23
built, now proven against a full realistic dataset, not just a
hand-built smoke test.

### Fixed — scripts/build-demo.sh: hardcoded login credentials regardless of which seed was selected

`START.cmd`, `START.sh`, and `README.txt` all hardcoded
`hl@example.com` / `holaquetal` unconditionally — harmless
only because every existing seed (01–04) happened to use those same
credentials. Building a demo from `seed-05-fairborne.py` (genuinely
different admin accounts) surfaced it directly: the generated
`README.txt` would have told the person to log in with credentials
that don't exist in that demo's own database. Fixed by extracting the
actual credentials from whichever seed was selected — a Python script
(not sed, per this project's own editing conventions — an earlier pass
of this same fix used sed and was corrected) scans forward from that
seed's own `hash_password(...)` call for the `"email"` values it
actually creates, rather than assuming one fixed pair, then patches
`START.cmd`/`START.sh` the same way. Verified against both seed-01
(unchanged: still extracts `hl@example.com`/`holaquetal`) and
seed-05 (correctly extracts its own Fairborne accounts) — no
regression for existing seeds.

## [0.3.6] — 2026-07-31

### Fixed — FE-24: FK-sourced dropdown fields crash the dynamic Asset form on a genuinely fresh tenant

The one confirmed blocker from a full FE-24 evidence-gathering pass
(field-coverage, required-field, enum-value, and MaxLength parity
between the static and dynamic Asset forms — all found to match
exactly, no other gaps). Re-verified live against xolu 0.19.3 before
fixing: on a brand-new, unseeded tenant, every FK-sourced field
(`type_id`, `brand_id`, `model_id`, `location_id`, `custodian_id`,
`currency_id`, `cost_center_id`) rendered `XOLU-QL004: entity does
not exist` in place of the widget — including `type_id`, which is
required, making the form genuinely unsubmittable on the very first
session of a new deployment.

New `entityNotYetCreated` (`internal/ui/asset_source_resolver.go`)
checks both the xolu error code and the specific `entity '%s' does
not exist` message xolu generates from exactly one place internally —
deliberately narrower than the code alone, since `XOLU-QL004` is a
general "query failed" bucket that covers more than just this one
case. A same-code failure for any other reason still surfaces as a
real, visible error, rather than degrading to a silently-empty
dropdown the way the static form's own blanket error-swallowing does.

Re-verified live both ways: the original fresh-instance repro now
renders every FK field as a real, functional, empty `<select>`
(`type_id` genuinely submittable); a fully seeded instance still
populates every dropdown correctly, no regression.

See `docs/formengine/TRACKING.md`'s `FE-24` entry for the full
evidence-gathering narrative and the fix's own detail. With this in,
FE-24's only remaining step is the tech team's own production validation —
nothing else identified as blocking it.

### Fixed — T-29: tracking-document version headers were stuck at 0.2.54 since before this session

Found while adding the FE-24 note above: `docs/TRACKING.md`,
`docs/KNOWN_ISSUES.md`, and `docs/formengine/TRACKING.md` all still
read `Version: 0.2.54 / Last reviewed: 2026-07-28`, unchanged across
every one of this session's six releases. `.repoman.json` declares
these three as version-sync targets, but this repo's own `syncver.sh`
never actually reads that manifest — it only updates `VERSION`,
`CHANGELOG.md`, `internal/version/version.go`, and `README.md`.
Corrected by hand for this release; the tooling gap itself (`syncver.sh`
doesn't consume `.repoman.json`) stays open as `T-29`.

## [0.3.5] — 2026-07-31

### Added — T-23/T-22: enum value/label separation, live-verified in three locales

The last piece of the internal/external language law work this
session, and the hardest: `asset.json`'s own duplicate
`criticality`/`tenure_type`/`decommission_reason` enums are now
English (matching T-24's tokens exactly), with a real display-boundary
mechanism so the *label* still varies by locale.

**Design decision** (the tech team): `widgetRenderer`'s signature extended
with `rc RenderContext`, matching `resolveTitle`/`resolveHelp`'s
existing shape, rather than threading locale via `context.Context`.
Touches all 17 registered widgets and both dispatch call sites
(`renderer.go`, `repeatable.go`'s nested dispatch) — mechanical,
`go build`-verified.

New `resolveEnumOptionLabel` resolves a select option's label via
`"<form>.<field>.<value>"` (e.g. `asset.criticality.high`), reusing
the exact key convention already shipped in Part 1. The submitted
*value* never changes by locale — only the label does. New
`resolveSelectPlaceholder` does the same for the two hardcoded
empty-option strings (T-22), with real `en`/`es`/`pt` translations
added, not just the mechanism.

`AssetValuesFromXolu`'s enum-value translation — deliberately
write-only since T-24, because `asset.json` was still Spanish then —
is now bidirectional, closing exactly the gap T-24 left open. Checked
every value the four seed scripts actually write: all covered by the
existing translation table, so **zero seed script changes** and no
data migration were needed — seed-created assets display correctly
translated the moment they're opened for editing.

**Live-verified beyond unit tests**: three real running `seam`
instances (`en`/`es`/`pt`), same `xolu`, actual rendered `<option>`
tags fetched from the live new-asset form. `value="High"` in every
locale; label `High`/`Alta`/`Alta`, `Medium`/`Media`/`Média`,
`Low`/`Baja`/`Baixa`.

See `docs/RESOLVED.md`'s `T-22`/`T-23` entries for the full narrative.
This closes the internal/external language law's original violation
in full — nothing from T-22/T-23/T-24/T-26/T-27 remains open.

## [0.3.4] — 2026-07-31

### Fixed — T-27: Region/Moneda/CentroCosto/Marca/Modelo/Ubicacion schema field names are English (internal/external language law, Part 3 — final)

Closes out the full `internal/validation/schemas.go` sweep T-24
started. Six more schemas, field names only — none of these have enum
values. `RegionCreateSchema`/`UpdateSchema` (`descripcion`),
`MonedaCreateSchema`/`UpdateSchema` (`codigo_iso`, `simbolo`),
`CentroCostoCreateSchema`/`UpdateSchema` (`codigo_contable`, `nombre`),
`MarcaCreateSchema`/`UpdateSchema` (`codigo`, `nombre`),
`ModeloCreateSchema` (`marca_id`, `nombre`; `UpdateSchema` is a pure
alias), `UbicacionCreateSchema` (`nombre_corto`, `descripcion`;
`UpdateSchema` is a pure alias) — all now English.

`internal/ui/masters_xolu_fields.go` extended with six more per-entity
translation tables. `Modelo`'s `brand_id` deliberately reuses the same
name `asset_xolu_fields.go` already uses for the equivalent Asset
field — one name per concept across the app, not two. All twelve
`masters_routes.go` handlers wired the same way as T-24/T-26: English
map keys, unchanged HTML forms, translated back to Spanish immediately
before every `xoluClient.Create`/`Update` call. Read-only paths
(list-page OQL queries, reference-count checks) query xolu's real,
unchanged storage directly and were left untouched.

A final full-file sweep of `internal/validation/schemas.go` confirms
every remaining `Field(...)`/`Enum(...)` string is either already
English, `GET`/`POST`/`PUT`, or `Custodio`'s deliberately-untranslated
`DNI`/`CI`/`RUT`/`RUC` codes — **the file is now fully English**,
closing the internal/external language law violation T-24 first found
in this file.

Caught and fixed during this session, worth recording: `register.py
add`'s automatic next-free-ID picked `T-26` again, colliding with the
`T-26` already closed in `docs/RESOLVED.md` — it only scans
`TRACKING.md`'s open items, not `RESOLVED.md`, a known, previously-
documented limitation. Caught before it propagated by checking
`RESOLVED.md` directly rather than trusting the tool's own count; the
wrongly-numbered entry was removed and this item re-filed as `T-27`
with an explicit ID.

See `docs/RESOLVED.md`'s `T-27` entry for the full narrative. T-23's
own remaining scope (`asset.json`'s duplicate enum, the four seed
scripts, persisted-data display, the widget-dispatch-locale fork) is
the only part of this whole area still open.

## [0.3.3] — 2026-07-31

### Fixed — T-26: Custodio/Alerta/PropDefinition/PropAssoc schema field names and enum values are English (internal/external language law, Part 2)

Same law as T-24, applied to the next slice of
`internal/validation/schemas.go`: `CustodioCreateSchema`/`UpdateSchema`,
`AlertaCreateSchema`/`UpdateSchema`, `PropDefinitionCreateSchema`/
`UpdateSchema`, and `PropAssocCreateSchema` (`PropAssocUpdateSchema` is
a pure alias) were all Spanish, field names and — for Alerta,
PropDefinition, and PropAssoc — enum values too.

Confirmed before touching anything: only the four `*CreateSchema`
variants are ever actually live — `validate.go` exposes one schema var
per entity, used by both the Create *and* Update HTTP handlers in
`masters_routes.go`. The separate `*UpdateSchema` declarations were
never referenced anywhere; fixed anyway for consistency, but the real
risk surface was the four live schemas and their eight handler
functions.

New `internal/ui/masters_xolu_fields.go`, mirroring T-24's
`asset_xolu_fields.go` pattern but simpler — none of these four
entities have array-typed fields, so flat key/value translation
covers all of them. `Custodio`'s `subject_type` values (`DNI`/`CI`/
`RUT`/`RUC`) are deliberately **not** translated — regional legal
document-type codes, not translatable prose. All eight
`masters_routes.go` handlers wired the same way as T-24: English map
keys read from the same, unchanged HTML form field names, enum values
translated on the way in, the whole map translated back to Spanish
immediately before every `xoluClient.Create`/`Update` call.

No test file needed updating — extensive pre-existing coverage
already submits via the unchanged Spanish form field names and passed
without modification, including a test that captures the actual PUT
body sent to xolu and asserts it's still Spanish-keyed with the exact
submitted values, direct proof the translation is correct.

i18n groups added, `en`/`es`/`pt`: `alerta.type.*`, `alerta.criticality.*`
(reusing T-24's High/Medium/Low tokens), `prop_definition.data_type.*`
(ten values), `prop_association.requirement.*`.

See `docs/RESOLVED.md`'s `T-26` entry for the full narrative. Part 3
(`Region`, `Moneda`, `CentroCosto`, `Marca`, `Modelo`, `Ubicacion` —
field names only, no enum violations) remains as the last slice of
this file.

## [0.3.2] — 2026-07-31

### Changed — xolu bumped to 0.19.3

Local `replace` directive to a fresh xolu checkout (was `v0.16.16`).
Confirmed safe before touching code, not assumed: `pkg/client` — the
only xolu surface Seam imports — was deliberately left untouched by
xolu's intervening TenantID conversion, per xolu's own changelog.
Build, `go vet`, full test suite, and a genuine live end-to-end pass
(real login, real dynamic-form fetch, `tenure_type`'s visibility
condition round-tripping correctly) all clean against the new version.

### Fixed — T-24: Asset schema field names and enum values are English (internal/external language law)

`internal/validation/schemas.go`'s `AssetCreateSchema`/
`AssetUpdateSchema` were Spanish throughout, not just the three enum
values originally filed — investigating the as-filed scope surfaced
~28 Spanish field names in the same two schemas, never touched by
FE-23's earlier field-name fix (which reached `asset.json` only). All
renamed to English, reusing the exact names FE-23's own
`asset_xolu_fields.go` translation table already used for the
equivalent `asset.json` fields, not invented independently.

Also corrects an assumption in T-24 as originally filed: the "REST
API path, confirmed unused" framing was accurate for
`internal/handlers/api/assets.go` specifically, but the same two
schemas are also validated by `internal/ui/routes.go`'s **static**
Asset form — the path actually reachable through normal navigation,
unlike the dynamic form. Missed on first pass; found by tracing two
test failures back to root cause after the schema change, not by
assuming the change was safe once one caller's tests passed. One of
the two, a separate `preCheck` fast-fail map in `AssetCreate`
duplicating a subset of the main data map, was the actual cause and is
easy to miss on any future schema change to this file.

xolu's actual `assets` storage remains Spanish-keyed/valued, matching
all existing data — confirmed via seed-01 through seed-04, which write
Spanish keys directly. `AssetValuesToXolu` (already used by the
dynamic form) now also translates the three enum fields' values,
write-direction only, and both live write paths translate through it
immediately before persistence. No data migration, no seed rewrite —
only the validation-layer identifiers themselves needed to change.

### Added — i18n group for the three Asset enum fields' display labels

`asset.criticality.{high,medium,low}`,
`asset.tenure_type.{owned,loaned,borrowed,rented}`,
`asset.decommission_reason.{sale,scrap,theft,donation,other}` added to
`en.json`/`es.json`/`pt.json`, following the existing
`asset.status.*`/`asset.connectivity.*` convention. Not yet consumed
by any renderer (that's T-22/T-23's own widget-dispatch-locale design
fork) but now centrally defined rather than left for whoever solves
that fork to invent ad hoc.

See `docs/RESOLVED.md`'s `T-24` entry for the full narrative. T-23's
own remaining scope (`asset.json`'s duplicate enum, the four seed
scripts, persisted-data display, the widget-dispatch-locale fork) is
unchanged by this release.

### Added — T-25 filed: `internal/ui/workflow` dormant guard undocumented and stale

Found during the xolu 0.19.3 bump: a legitimate dormant guard (own
`go.mod`, `make test-workflow` only, never `go test ./...`) that was
never added to `docs/KNOWN_ISSUES.md`'s dormant-guards table, and
whose own pins (`xolu v0.9.7-patched74`, `minty v0.1.0`,
`queryfy v0.3.1`, Go `1.22.2`) are roughly four months stale against
the rest of the repo. Added to the dormant-guards table with an
honest "not run this session" rather than a false pass; bringing its
pins current is real, separate work given the size of the xolu
internal-API gap specifically.

## [0.3.1] — 2026-07-31

### Added — Demo runtime language selection

`scripts/launch.sh` and `scripts/build-demo.sh`'s generated `START.sh`/
`START.cmd` templates now prompt for a UI language (English, Español,
or Português) at startup, before Seam starts, setting
`SEAM_FORCE_LANGUAGE` accordingly. `launch.sh` also gained a `--lang
CODE` flag and honors a `SEAM_FORCE_LANGUAGE` already set in the
environment, skipping the prompt when either is supplied — matching
the script's own existing "environment overrides take precedence"
convention rather than introducing a new one.

Defaults to English throughout, matching `ui.DefaultConfig()`'s own
default — an earlier draft of this defaulted to Español, incorrectly
inferring a UI-language default from the seed *data* being
Spanish-oriented; corrected before release, not shipped and fixed
later. Verified live in both directions: piping each menu choice
through a real running instance and confirming the actual rendered
Asset form (buttons, field labels) comes back in the selected
language, not just that the shell variable resolved correctly in
isolation.

### Added — Non-negotiable internal/external language invariant, documented

`docs/KNOWN_ISSUES.md` gained a new "Non-negotiable invariants"
section: internal identifiers, invariants, constants, and stored state
must always be English, with no exception; only external, user-facing
content depends on locale. States explicitly that this covers enum
*values* used as validation constants or stored state, not just field
*names* — a field named in English still violates the law if its own
valid values are hardcoded in one language at the validation layer.

Applied immediately against the codebase, not left abstract: found and
filed a real, concrete violation (`criticidad`, `calidad_tenencia`,
`motivo_baja` in `internal/validation/schemas.go` and, independently,
`asset.json`'s own duplicate copy of the same values) as `T-23`.
Investigated further and split into two register items once the real
shape became clear: `T-24` carries the narrow, self-contained slice
(`internal/validation/schemas.go`'s three enums, confirmed unused by
anything today — the REST API path nothing currently exercises); the
reduced `T-23` keeps the larger, harder remainder — `asset.json`'s own
independently-hardcoded copy (what the live UI actually uses), the
missing mechanism to display a stored value under a different,
locale-appropriate label (doesn't exist anywhere in the widget
renderer today), what happens to already-persisted data, and
reconciling all four existing seeds' Spanish literals. Neither T-23
nor T-24 is fixed by this release — this release documents the law
and files precise, honestly-scoped work against it.

### Added — `internal/condition`, a single source of truth for value-comparison and composite (AND/OR) condition evaluation

Extracted from `formengine`'s own visibility system after finding the
same shape of problem — independently re-implemented, with real,
undocumented semantic drift between copies — in multiple places across
the codebase. Deliberately *not* built on or synced with `mintydyn`
(`github.com/ha1tch/minty`), investigated as a shared dependency and
found already diverged in ways that mattered for this codebase's own
rules (its `empty` treats `false` as empty; Seam's own rule explicitly
doesn't; `gte`/`lte` are missing from `mintydyn` entirely).

`formengine/visibility.go` is now a 40-line adapter over
`internal/condition` (a type alias plus three forwarding functions) —
`schema.go`, `renderer.go`, and `hidden_field_policy.go` needed zero
changes, confirmed by a clean build before touching anything else, not
assumed. 14 new tests, including a permanent regression test for a
mistake caught during the port itself: an unnecessary boolean-coercion
guard was initially added to `looseEqual`, reasoning from the
client-side JS mirror's genuine need for one without checking whether
Go needed the same treatment — it didn't (Go's own `asFloat` and
type-strict interface equality already handle it), and the guard was
removed rather than shipped with an inaccurate comment.

Scope explicitly limited to `formengine`'s condition evaluator — two
other, unrelated "evaluate a condition" implementations found in the
same search (`internal/rules/engine.go`, `internal/fsm/evaluator.go`)
are genuinely separate domains (server-side sensor/threshold rules, no
JS or `minty` relationship) and were deliberately excluded.

Filed for later, not forgotten: `T-20` (whether this should eventually
become its own repository, matching `minty`/`xolu`/etc. — no deadline,
an open question) and `T-21` (the client-side JS mirror,
`visibilityScript` in `renderer.go`, is still hand-written and
manually kept in sync, not yet generated from `internal/condition`'s
own definitions the way `mintydyn` generates its JS — no known
divergence today, both sides were audited together, but preventive
work worth doing before either side is touched independently again).

### Changed — `minty` dependency upgraded 0.1.0 → 0.2.1

Two upgrade steps, each verified against the real, published module
via the actual Go proxy (not a local `replace` directive) before
adoption: full build, `vet`, and test suite clean, 10x stress runs on
`formengine` and `ui` specifically (the heaviest `minty` consumers)
after each. `v0.2.1` adds `in`/`notIn` list-membership operators to
`mintydyn`'s conditions — found to be a real gap while scoping (and
ultimately not pursuing) a `mintydyn`-based migration for the
condition evaluator described above.

### Fixed — Asset dynamic form's modal chrome now respects the active locale

`asset_dynamic_form_wrapper.go`'s buttons (Cancel/Create/Save) and
delete-confirmation text were hardcoded Spanish regardless of the
configured locale — found and confirmed by an actual live render with
`SEAM_FORCE_LANGUAGE=en` set, not by reading the code. Now wired
through the existing `action.cancel`/`action.create`/`action.save`/
`action.delete`/`confirm.delete` i18n keys, all of which already
existed in all three locale files. Verified live in both `es` and
`en`: buttons correctly translate, field labels and the
`x-seam-visible-when` condition rendering both remained intact
throughout.

Deliberately scoped, not a general i18n sweep: `AssetFormPage` (the
static Asset form this dynamic one is intended to eventually replace,
`T-01`/`FE-24`) has its own, much more extensive hardcoded-Spanish
field labels, out of scope here since investing in that code's i18n
isn't worthwhile ahead of its own retirement. The shared
`conditionalDeleteButton`/`conditionalButtonText` helpers
(`asset_detail.go`), used by that static form, were left completely
unchanged rather than given translator plumbing that form never had —
a small, separate `dynamicDeleteButton` handles the dynamic form's own
delete button instead.

One related gap found in the same sweep and deliberately deferred, not
silently dropped: the enum/FK-select empty-option placeholders
(`formengine/widgets.go`: `"— Seleccionar —"`, `"— Sin asignar —"`)
are still hardcoded. Filed as `T-22` — fixing it properly means
choosing between two real designs (extending the shared widget
dispatch table's signature, versus a second, `context.Context`-based
locale-threading mechanism alongside the one `formengine/i18n.go`
already uses), a decision worth making deliberately rather than
defaulting into while patching one string.

Full `go build`/`go vet`/`go test ./...` clean (16 packages, including
the new `internal/condition`), confirmed against a real, live instance
end to end — real login, real Asset-form render, real visibility
conditions — not unit tests alone.



### Changed — removed AI-authorship references from the repository

Exhaustive repo-wide search (case-insensitive, every file, `.git`
excluded) found four real occurrences, each handled on its own merits
rather than blindly stripped:

- `scripts/olu_to_xolu_docs.py`: dropped a parenthetical aside about a
  hypothetical future AI session; "whoever implements those modules
  later" already covers this without naming anything specifically.
- `docs/RESOLVED.md`: removed a sandbox-specific file path that
  happened to contain an AI-tool name as part of a local directory
  name, not an attribution — the substantive record (a local clone
  exists, pending a publishing decision) is unchanged. A narrow,
  non-substantive exception to this file's own append-only
  convention, made because the edit doesn't rewrite any decision or
  finding, only an irrelevant path fragment.
- `docs/proposals/mintydyn-composite-conditions.md`: removed an
  AI-authorship line (this is the only proposal file in the directory
  and the only one that ever had an Author field at all — no
  convention was broken by dropping it, and substituting a different
  name would have been factually wrong) and rewrote the "Access note"
  section to keep its substantive point (staged locally, no push
  access, handed to the tech team to decide) without the AI-assistant
  framing.
- Verified fully clean afterward with a second, identical exhaustive
  search — zero remaining matches anywhere in the repository.

New standing memory instruction recorded, per explicit request: never
add authorship/co-authorship claims, AI-usage credit, or AI-related
disclaimers into any project artifact (code, comments, commits, docs,
changelog) going forward.

Full repo `go build`/`go test ./...` clean (15 packages, excluding the
known `T-09` flakiness).

## [0.2.53] — 2026-07-29

### Changed — T-19's picker design substantially clarified

No code changed — register only. We resolved both open questions
from `T-19` directly rather than leaving them open.

- **Listings**: confirmed technically real, not assumed —
  autoComplete.js's own release notes document a specific bug fix for
  `resultsList` being disabled, meaning "the query engine runs, no
  dropdown renders" is a first-class, tested configuration, not a
  hack. `SearchInput` becomes: type, the actual table filters live via
  the library's own result event (most naturally through HTMX), no
  separate suggestion UI — a third option, better than either
  previously guessed (jump-to-record / typing-assistant).
- **Asset/Sensor picking**: confirmed multi-field result display is
  directly supported (`resultItem.element`/`resultsList.element`
  hooks hand back raw result data for arbitrary per-row rendering),
  not something requiring a workaround.
- **Architecture resolved**: one generic, schema-configured picker,
  not an `AssetPicker`. Extends the existing, already-proven
  `x-seam-source: entity:<name>` convention with two new attributes
  (`x-seam-search-fields`, `x-seam-display-fields`) — one widget, one
  generalized backend endpoint, zero new Go code per future entity
  type (Sensors, etc.).
- Full repo `go build`/`go test ./...` clean (15 packages).

## [0.2.52] — 2026-07-29

### Changed — T-19 scope extended to the listing search bar

No code changed — register only. We asked whether the same
vendored autoComplete.js could also power an optional search bar in
Seam's shared list-page components ("the listing invariants").

Investigated rather than assumed: confirmed `internal/ui/listing.go`'s
`ListFilterBar` and `forms.go`'s `SearchInput` are real, shared
components — `SearchInput` specifically is the true invariant, used
directly by the assets list and wrapped by `ListFilterBar` (sensors
list only, so far). Confirmed technically reusable — autoComplete.js
supports multiple independently-configured instances per page.

Flagged a real, undecided design question rather than picking
silently: the existing search submits a plain GET form (filters the
whole list server-side); autoComplete.js's native behaviour is
per-record live suggestions, a different granularity. Two legitimate
integration choices — jump-to-record (navigate straight to the
selected asset) vs. typing-assistant (autocomplete helps fill the
existing search box, current filtering behaviour unchanged) —
recorded in `T-19` rather than defaulted.

Full repo `go build`/`go test ./...` clean (15 packages).

## [0.2.51] — 2026-07-29

### Added — autoComplete.js vendored and re-themed for the asset-picker widget

Completes the version/changelog bookkeeping left unfinished at the end of
the previous turn (verified clean: build, vet, full test suite, both
register checks).

- `web/static/vendor/autocomplete@10.2.10.min.js` + `.css` vendored,
  verified byte-identical to the real npm package via direct `npm
  install` + `diff`, not just downloaded from a CDN mirror and trusted.
  Documented as Asset 6 in `VENDOR.md`, with `TestVendorFilesPresent`
  coverage for both files.
- New `web/static/css/autocomplete-seam.css`: re-themes the vendored
  default to match Seam's light/dark palette (via `var(--color-primary)`,
  already theme-aware — no JS wrapper needed, confirmed `minty`'s
  `DarkModeTailwind` only toggles `class="dark"` on `<html>`, no custom
  event fires) and font stack. Fixes a real, confirmed upstream
  limitation: the default highlight colour (`#ff7a7a`, coral-red) is
  applied directly to `mark` with high specificity — a known, open,
  unfixed upstream bug (GitHub issue #264). Matches it with the
  identical selector chain to win by load order, no `!important`
  needed. Dropdown styling matches Seam's real, existing dropdown-menu
  convention (`layout.go`'s user-menu popover), not guessed.
- `T-19` filed to track the real remaining work: a backend search
  endpoint (confirmed empirically that xolu's OQL already supports the
  exact `id = X OR name LIKE '%Y%'` query shape needed), formengine
  wiring to replace the current number-input MVP fallback, and a
  genuine, undecided scope question (the spec's own vision for this
  widget goes considerably further than a simple search-and-pick box).
- Full repo `go build`/`go vet`/`go test ./...` clean (15 packages,
  excluding the known `T-09` flakiness).

## [0.2.50] — 2026-07-29

### Changed — x-seam-relation values translated to English

Extends the existing "all schema identifiers stay English" principle
(established for the 42 Asset field names) to enum-like values too,
per the tech team: "contiene"/"usa" → "contains"/"uses" in `asset.json`'s
`parent_id` field and throughout `FORM_ENGINE_SPEC.md` §8; `capacidad`
→ `capacity` in the spec's third edge-type mention, for consistency
(not itself referenced anywhere in Go code).

- Confirmed no Go code switches on these string values before
  touching anything — `schema.go`'s only mention is a doc comment,
  now also updated.
- Spec fixes applied precisely: 5 distinct occurrences (the
  reference-table description, two identical JSON examples, the
  §8.1 value table, the §8.2 bullet list and prose sentence), each
  verified for an exact single match before writing. Confirmed the
  remaining 8 "contiene" matches in the spec are all ordinary Spanish
  prose (the verb "contains") or `x-seam-help` display text —
  correctly left untouched, not swept up by a looser match.
- New standing memory instruction recorded, per explicit request:
  internal/code-level identifiers stay English throughout; Spanish is
  reserved for user-facing content only.
- Full repo `go build`/`go vet`/`go test ./...` clean (15 packages,
  excluding the known `T-09` flakiness).

## [0.2.49] — 2026-07-29

### Fixed — seed scripts confirmed compatible, cosmetic naming cleaned up

We pushed back correctly on last turn's claim: confirming a
`<form>` tag exists isn't confirming the form actually renders. Full
field-by-field inspection of the real rendered HTML found something
genuine — on a fresh, unseeded xolu instance (exactly what
`launch.sh` produces on its own), every FK-sourced field (`type_id`,
`brand_id`, `model_id`, etc.) failed to render at all, showing a raw
`XOLU-QL004: entity 'asset_types' does not exist` error instead of a
widget. `type_id` is required, so the form was genuinely unsubmittable
in that state. The static form's own `loadMarcas`/`asset_types`
queries explicitly discard errors and fall back to an empty list;
`AssetSourceResolver.Resolve` deliberately propagates them instead —
a real difference, not investigated further this turn per instruction.

Instead: ran `seed-01-constructec.py` against a live instance to
check whether the seed scripts need updating for xolu's current
shape. They don't — it ran cleanly, creating a full, coherent
dataset (7 asset types, 8 brands, 15 models, 16 assets, etc.) with
zero code changes needed. The only failure on the first attempt was a
missing local Python dependency (`bcrypt`), not a script bug. Once
seeded, every previously-broken FK-select field rendered correctly
with real, populated options — confirmed field by field, not just by
page size.

Cosmetic cleanup applied to all 4 seed scripts (help text, docstrings,
print messages) to match `launch.sh`/`build-demo.sh`'s already-done
olu→xolu rename: word-boundary-safe regex, verified per-file counts
before writing, careful not to touch substrings like "volumen" that
contain "olu" incidentally. Re-ran the fixed script afterward to
confirm the cosmetic change didn't break anything.

Full repo `go build`/`go test ./...` clean (15 packages, excluding
the known `T-09` flakiness). All 4 seed scripts syntax-checked.

## [0.2.48] — 2026-07-29

### Fixed — two real bugs in build-demo.sh/launch.sh, found by actually running them

We asked whether the demo scripts fixed earlier this session
actually work. Answered by running them for real — syntax-checking
alone had already passed and would have missed both of these:

- **`build-demo.sh` had `XXOLU_DIR`/`XXOLU_REPO`** (double-X), 14
  occurrences — a genuine bug from this session's own earlier
  "OLU_→XOLU_" blanket substring fix, applied to text that already
  read "XOLU_" in this one file (unlike `launch.sh`, where the
  original names were "OLU_"). Fixed.
- **Both scripts cloned xolu over SSH**
  (`git@github.com:ha1tch/xolu.git`), which fails outright in any
  environment without SSH keys configured — confirmed by watching it
  fail live (`error: cannot run ssh`), then confirmed the fix
  (`https://github.com/ha1tch/xolu.git`) clones cleanly with no
  authentication needed, since xolu is public.
- **Full end-to-end verification of `launch.sh`**, not just its build
  steps: cloned xolu, built all three binaries (`xolu`, `seam`,
  `seam-admin`), started both services with the exact env vars the
  script sets, confirmed both health checks, created a real admin
  user via `seam-admin`, logged in for a real session, and fetched
  `/assets/dynamic/new` with that session — confirmed it renders a
  real `<form>` tag with the correct `hx-post` targets, proving last
  turn's form-tag fix works against a real running instance, not just
  fake test servers.
- `build-demo.sh`'s specific clone fix independently re-verified by
  direct execution, not just re-reading the diff.
- Full repo `go build`/`go vet`/`go test ./...` clean (15 packages,
  excluding the known, pre-existing `T-09` flakiness). Both scripts
  syntax-checked and confirmed free of any remaining `XX`-prefixed or
  SSH-URL patterns.

## [0.2.47] — 2026-07-29

### Fixed — the dynamic Asset form is now genuinely usable end to end

Itemised and executed against the tech team's own gate: "I can perform
production validation as soon as the new dynamic form system is
feature-complete, and a full data round trip can be verified."

- **A real, structural gap found by direct investigation, not
  assumed**: the dynamic Asset form rendered zero `<form>` tags at
  all — confirmed by an actual render. No amount of backend/logic
  testing would ever catch this. `renderForm`'s own comment confirmed
  this was always the caller's job by design, not an oversight in
  formengine — the gap was that no caller had done it yet. New
  `wrapAssetDynamicForm` mirrors the static form's exact existing
  pattern (correct modal system, not the masters module's different
  one; a subtle duplicate-HTMX-attributes detail on the submit
  button, needed to avoid a race with the native form-submit event).
  Wired into all four handlers.
- **A second self-correction along the way**: an earlier claim this
  session that "no delete route exists for assets at all" was wrong
  — one exists via the API layer, confirmed from source, and is now
  wired into the wrapper too, reusing the existing button rather than
  duplicating it.
- **Navigation was entirely disconnected from the dynamic form** —
  confirmed directly: every real entry point (list, detail, create)
  pointed at the static routes exclusively. All 5 repointed; a full
  sweep confirmed the remainder are correctly unrelated (the static
  form's own submit target, or detail/view links, which stay
  static-only by design).
- **Round-trip proof built**: the first test to chain create-side and
  read-side translation together, rather than each independently.
- **Two real test-quality findings, neither a production bug**: a
  mutation test caught a bare `"500"` substring check as a false
  positive (Tailwind's own CSS classes contain that number
  regardless of correctness); a 30-iteration stress run caught the
  round-trip test itself intermittently failing on an attribute-order
  assumption that isn't actually guaranteed — the underlying data was
  correct on every iteration, including failing ones. Both fixed and
  re-verified with 50x/15x stress runs.
- **A correction to the previous entry**: the date-field validation
  gap claimed unfixed in 0.2.46 was already fixed by the earlier
  18-field maxLength pass — a genuine record-keeping mistake, now
  corrected.
- Full repo `go build`/`go vet`/`go test ./...` clean (15 packages,
  excluding the known, pre-existing `T-09` flakiness).
- Production validation is now genuinely startable per the tech team's own
  stated gate.

## [0.2.46] — 2026-07-29

### Fixed — FE-24 regression testing: schema drift found and fixed, create/update split added

Started with the riskiest, most uncertain part flagged earlier this
session: systematic, programmatic comparison of the static form's
`AssetCreateSchema`/`AssetUpdateSchema` against `asset.json`'s
compiled schema — not transcribed by hand.

- **`asset.json` had zero `minLength`/`maxLength` constraints
  anywhere**, while the static schema enforces `MaxLength` on 18
  fields. All 18 added, extraction and mapping fully scripted and
  verified. `name` specifically raised from the static form's 200 to
  500, reasoned as generous enough for descriptive equipment names
  while still bounded.
- **A real, currently-unfixed gap found and flagged, not silently
  bundled into the same fix**: the 8 date fields use `"format":
  "date"`, which — confirmed empirically — queryfy's
  `jsonschema.FromJSON` silently ignores entirely. They have *no*
  validation today, worse than the static form's crude 10-char cap.
- **A structural gap**: the dynamic form used one schema for both
  create and update, requiring `name`/`type_id`/`criticality` on
  update too — the static form's own `AssetUpdateSchema` makes all
  three optional there. New `asset.SchemaForUpdate()` returns the
  schema with its `required` array emptied, used only for
  `AssetUpdateDynamic`'s validation — rendering is unaffected, so the
  visual `*` stays identical on create and edit either way.
- A real doc/code mismatch noted in passing: `AssetUpdateSchema`'s own
  comment says "all optional except name," but its code makes `name`
  optional too.
- Tests: one existing test updated (its old failure trigger no longer
  applies under the new required-ness), two new tests added — direct
  proof of the create/update split, and direct proof `name`'s raised
  cap is genuinely enforced at the exact boundary (500 passes, 501
  fails). Stress-run 10x clean.
- Full repo `go build`/`go vet`/`go test ./...` clean (15 packages,
  excluding the known, pre-existing `T-09` flakiness).
- `FE-24` remaining: field-coverage comparison, persistence parity
  (diffing actual xolu documents from both paths), and the date-field
  validation gap noted above.

## [0.2.45] — 2026-07-29

### Added — T-17 closed: real en/es content for the Asset form

See `docs/RESOLVED.md` for T-17's full closure record.

- 64 new i18n keys, identical across `en.json`/`es.json`, zero
  collisions with the existing 120 (verified programmatically before
  writing). 4 tab labels, 9 section labels, 42 field titles, 1 field
  help text (`parent_id`, the only field with `x-seam-help`), 8
  `core.errors.*` keys shared app-wide.
- **A real prerequisite gap found and fixed first**: `asset.json`'s
  `x-seam-form` had no `module`/`name` keys at all — every render's
  `rc.Module`/`rc.Form` were empty, so `resolveTitle`/`resolveHelp`
  were silently skipping the `assets.forms.asset.field.*` step
  entirely. Added `module: "assets"`, `name: "asset"`, matching
  §3.2's own documented example exactly.
- Also fixed while there: §3.2's prose named the override fields
  `RenderContext.I18nModule`/`I18nForm`, which don't exist — the real
  fields are `Module`/`Form`.
- **A real, visible bug confirmed empirically, not assumed**: with no
  content authored, tab labels rendered as their own raw i18n key
  text (`"assets.forms.asset.tab.principal"` literally visible in the
  HTML) — live and broken in the dynamic form every time it
  rendered, regardless of locale, since `resolveLabel`'s fallback is
  the label value itself, and the schema authors wrote that value as
  a key rather than literal text.
- Two new end-to-end tests using the real, shipped `i18n.New`
  translator, not mocks: tabs/sections/field titles/help all resolve
  correctly in both languages and the raw-key bug is genuinely gone;
  all 8 `core.errors.*` rules resolve in both locales against the
  actual authored content. Stress-run 10x clean.
- Full repo `go build`/`go vet`/`go test ./...` clean (15 packages,
  excluding the known, pre-existing `T-09` flakiness, confirmed
  unrelated).
- **`T-17` is now fully closed** — all three gaps (locale mechanism,
  handler wiring, content) resolved. `FE-24`'s retirement step is
  unblocked on the mechanism/content side; production-proven parity
  (§13.3) remains the tech team's own deployment process.

## [0.2.44] — 2026-07-29

### Fixed — T-17 gap 2: dynamic Asset form handlers now honour the current locale

- All four handlers (`AssetNewDynamic`, `AssetEditDynamic`,
  `AssetCreateDynamic`, `AssetUpdateDynamic`) now wire both
  `Translator` and `RenderContext.Locale`.
- New `h.currentLocale()` helper: reads `h.config.Locale` under its
  guarding lock — the same value Settings/Profile already read and
  write, and `SEAM_FORCE_LANGUAGE` seeds at startup.
- **Found while wiring this**: even the two POST handlers, which
  already had `Translator` wired since FE-23, never set `rc.Locale`
  at all — resolution was falling through to literal text there too,
  despite the translator being present.
- New `i18n.NewFromMessages` (exported test-support constructor,
  `internal/i18n`): needed since the real embedded locales have no
  Asset-form keys yet (gap 3, still open), so cross-package tests
  couldn't otherwise verify resolution against known content.
- New end-to-end test proves this is genuinely functional, not just
  wiring that compiles: confirms a translated label renders with a
  matching locale, and confirms fallback to the schema's literal
  title with a locale that has no match anywhere in its own fallback
  chain either — caught a test-construction mistake along the way
  (an early version still resolved through the translator's own
  fallback locale even when the *requested* locale had no key).
  Stress-run 10x clean.
- Full repo `go build`/`go vet`/`go test ./...` clean (15 packages).
- `T-17`'s only remaining piece is gap (3): authoring real en/es
  content for the 42 Asset fields.

## [0.2.43] — 2026-07-29

### Changed — T-18 filed: app-wide i18n wiring

The tech team, following up on whether i18n covers the whole app or only
formengine: "We need it. If it's not a task it should be one."

No code changed — register only. Investigated before filing:
`h.translator` has exactly 3 call sites in `internal/ui`
(`AvailableLanguages`, `ValidateLocale`, `formEngineTranslator`), zero
of which are `.T(...)` calls. Every nav label, page title, and
on-screen message outside the form engine is a hardcoded Go string
literal. Concrete evidence beyond "not yet translated": 100 separate
`h.page.Render` call sites across 26 files, hardcoded titles not even
consistently one language today (`masters_routes.go`'s Spanish
"Regiones"/"Nueva Región" alongside `asset_dynamic_post.go`'s English
"New Asset"/"Edit Asset", in the same running app).

Deliberately not scoped further than filing — 100+ page titles alone,
before nav/buttons/messages, is unlikely to be single-session work.
Shares its per-request locale mechanism with `T-17`'s gap (2); does
not itself block `FE-24`.

## [0.2.42] — 2026-07-29

### Fixed — confirmed sibling-region fallback, no code change needed

We asked directly whether `es_es` requested / no `es_es` / no
generic `es` / `es_uy` available as the closest alternative was
handled. Traced it through the real code rather than reasoning about
it in the abstract, then ran it as an actual test before answering:
already correct, via the existing candidates fallback in
`ResolveLocale` (any locale sharing the requested code's base
matches, not just the specific sibling already covered by
`TestResolveLocale_RegionRequestedDifferentRegionLoaded`, which
covers the identical logical case with the variants swapped).

Added `TestResolveLocale_RegionRequestedDifferentRegionLoaded_MirrorCase`
using the exact scenario as asked, so it's explicitly on record rather
than relying on a reader to verify the symmetry themselves. No
production code changed — 9/9 `ResolveLocale` tests pass.

## [0.2.41] — 2026-07-29

### Added — Translator.ResolveLocale, region-aware locale matching

- `internal/i18n/i18n.go`: new `ResolveLocale(requested string) (locale
  string, exact bool)`, supporting both directions of base/region
  matching — a requested base code (`"en"`) matches a loaded
  region-qualified variant (`"en_gb"`) if no plain `"en"` is loaded,
  and a requested region code (`"es_uy"`) matches a loaded base
  (`"es"`) or a sibling region (`"es_es"`) if no exact match exists.
  Deterministic when multiple region candidates exist and neither the
  exact code nor the base is loaded — alphabetically first, not map
  iteration order.
- `SEAM_FORCE_LANGUAGE` now resolves through this instead of a plain
  `HasLocale` check — loads whatever's available in the requested
  language family with a warning naming both the requested and loaded
  values, and only drops the setting entirely when nothing in that
  family is loaded at all.
- Confirmed `es.json`'s actual content before assuming anything about
  it: already written in the tuteante rioplatense style (`"tu
  portafolio de activos"`, etc.), not generic Spanish — `es` already
  means `es_uy` in substance today, region-qualified files just don't
  exist as separate artifacts yet.
- 8 new tests on `ResolveLocale`, including the exact scenario as
  specified ("we have en_gb in store, but we specify en only").
- Full repo `go build`/`go vet`/`go test ./...` clean (15 packages).

## [0.2.40] — 2026-07-29

### Added — SEAM_FORCE_LANGUAGE

- New env var, checked early at startup: sets the initial UI locale
  before `ui.NewRouter` is constructed. Deliberately narrow — only
  supplies a starting default; normal bootup (should it ever gain the
  ability to load a saved Settings locale at startup) is free to
  overwrite it afterward. Invalid values are logged and dropped, not
  silently kept, matching `SettingsSaveGeneral`'s existing
  fallback-to-`en` behaviour.
- `internal/config/config.go`: `FeatureConfig.ForceLanguage` +
  `SEAM_FORCE_LANGUAGE` parsing, tested.
- **A correction to T-17's earlier filing**: its gap (1) claimed no
  locale-selection mechanism existed anywhere in the app. Wrong —
  `/settings` and `/profile` both already render a real, working
  language selector, and `POST /settings/general` already persists a
  tenant-wide locale to xolu. Missed originally by not searching
  broadly enough before filing. T-17's real remaining gap is narrower
  than first filed: the dynamic Asset form handlers simply never read
  the mechanism that already exists.
- **A separate, real gap found and deliberately left unfixed**: xolu's
  saved Settings locale doesn't survive a server restart today —
  `server.go` builds the UI config from `ui.DefaultConfig()`'s
  hardcoded default with no startup-time load from `tenant_settings`
  at all. Out of scope for this task per explicit instruction; flagged
  in T-17 for whenever that's addressed.
- Full repo `go build`/`go vet`/`go test ./...` clean (15 packages).

## [0.2.39] — 2026-07-28

### Fixed — scripts/launch.sh and scripts/build-demo.sh, broken by the olu→xolu rename

Neither script was touched by the earlier olu→xolu cleanup this session,
since that work never looked at `scripts/*.sh`. Investigated whether xolu's
shape had changed beyond naming before fixing anything — it had, in one
real way, found by reading xolu's actual current source rather than
assuming a rename covers everything:

- **`./cmd/olu` doesn't exist** — confirmed it's `./cmd/xolu` now. This
  failed outright, not just looked stale, in both scripts (3 build
  invocations total).
- **`OLU_DB_PATH` is gone from xolu entirely**, not renamed — confirmed
  by checking xolu's config source directly. xolu now derives the SQLite
  path internally from `XOLU_BASE_DIR` via a per-tenant layout
  (`<BaseDir>/t0000/store/xolu.db`), confirmed from `pkg/storage`'s own
  comments. Removed the 4 occurrences (one in `launch.sh`, three in
  `build-demo.sh` including both generated Windows/Unix start scripts)
  rather than renaming them to a variable xolu no longer reads.
- **Every other `OLU_*` env var verified individually** against xolu's
  current `pkg/config/config.go` before renaming — all 10 remaining
  ones (`XOLU_TENANT_MODE`, `XOLU_GRAPH_MODE`, `XOLU_FULLTEXT_ENABLED`,
  etc.) confirmed to still exist under the `XOLU_` prefix, checked one
  by one rather than assumed as a group.
- **Confirmed `iolu`/`xotogen` aren't needed for this flow** — checked
  directly whether xolu's tenant auto-registration and default no-auth
  behaviour still work the way both scripts assume: `TenantAutoRegister`
  still triggers real, functional auto-registration on first request
  (`GetOrRegister` in `pkg/server/server.go`), and `AuthType` still
  defaults to `"none"`. The "start xolu, no separate bootstrap step"
  design the scripts were built around is still exactly how xolu works.
- **A real functional bug found along the way, not just a rename**:
  `build-demo.sh`'s GitHub-release download fallback searched for an
  asset named `olu-${os}-${arch}` — xolu's actual release workflow
  names assets `xolu-${goos}-${goarch}` (confirmed from
  `.github/workflows/release.yml`). The download path would have always
  missed and silently fallen back to the slower build-from-source path
  every time.
- Binary output filenames, local bash variable names (`olu_bin`,
  `NATIVE_OLU`, `OLU_BIN`, etc.), directory names (`data/olu`,
  `$SEED_TMPDIR/olu`), Windows batch labels (`:wait_olu`), and all
  banner/log text renamed for consistency — confirmed zero remaining
  bare `olu`/`OLU` references in either file by exhaustive grep after
  the fact, not assumed complete after the targeted replacements.
- `bash -n` syntax-checked clean on both files throughout.
- Left untouched, deliberately: `build-demo.sh`'s hardcoded demo login
  credentials in the generated README — pre-existing content, not
  something this fix should touch.

## [0.2.38] — 2026-07-28

### Fixed — required-field asterisk now driven by a single source of truth

- Started FE-24 by resolving the design question T-13 carried
  forward: the visual `*` next to a required field's label is now
  driven entirely by `f.Required` (`fkPopoverAwareLabel`), never
  hand-typed into the schema's literal title text. Same
  structured-over-embedded-in-prose reasoning as the queryfy
  structured-errors proposal filed this session — applied here
  first, cited there as independent confirmation the pattern
  generalizes rather than being a one-off preference.
- 4 fields had it hand-typed in `asset.json`: `name`, `type_id`,
  `criticality` (genuinely required) and `region_id` (which was not
  — the known static-form inconsistency, FE-22). All 4 stripped,
  confirmed as the complete set by checking every field's title
  against its actual `required` array membership, not assumed to be
  only the 3 already known about.
- `region_id`'s inconsistency resolves as a side effect of having one
  source of truth, not a separate decision about that one field.
- New test checks the label's actual rendered text — an earlier
  version wrongly checked the input/select element's own attributes
  instead, caught by the test failing on first run.
- `FORM_ENGINE_SPEC.md` §12 documented.
- Full repo `go build`/`go vet`/`go test ./...` clean (15 packages).
- One of FE-24's two prerequisites resolved. `T-17` (locale selection
  + bilingual content) still gates the actual "retire the static
  form" step — per §13.3, that requires production-proven parity, a
  bar this session can build toward but not clear itself. FE-24's
  regression/parity testing work can proceed regardless.

## [0.2.37] — 2026-07-28

### Added — FE-23 closed: POST handling, plus a new additive error-status mechanism

See `docs/formengine/RESOLVED.md` for FE-23's full closure record.

- `internal/ui/asset_dynamic_post.go`: `AssetCreateDynamic` +
  `AssetUpdateDynamic`, routed `POST /assets/dynamic` and
  `POST /assets/dynamic/{id}`.
- Type coercion resolved by checking real code, not inferring: 9
  fields are genuine integer FKs needing `int64` conversion before
  persistence; the 6 fields `asset.json` itself declares `"number"`
  are persisted **and read back** as plain strings by every real
  handler, contradicting the schema's own stated type. Matched the
  real, coexisting convention.
- Verified queryfy's transform system directly before assuming
  anything about it: `jsonschema.FromJSON` never attaches a
  `Transform` to number/integer schemas, so none of this pipeline
  uses transforms. The "strings validate fine" behaviour is
  `NumberSchema.Validate`'s own Loose-mode coercion *check* — it
  doesn't mutate the value, so manual coercion for persistence is
  necessary, not a workaround for an unused library feature.
- **A real crash caught by the test suite itself**: `h.translator`
  (nil in test setup) assigned directly into the `Translator`
  interface field produced a "typed nil" — formengine's own
  `Translator == nil` checks silently evaluate false, and the next
  call panics on the nil receiver. Fixed with a centralised
  `h.formEngineTranslator()` helper every call site now uses, plus a
  warning on `FormEngine.Translator`'s own doc comment.
- **A second, more fundamental bug**: queryfy's error path for a
  nested array-item constraint (`document_links[0].url`) is dotted
  and bracketed, not a flat field name — the existing
  `errs map[string]string` mechanism would have silently dropped
  every such error. Fixed jointly with the tech team: `RenderContext`
  gained an additive `Statuses []FieldStatus` field (element/status/
  message, `ERR`/`WRN`/`INF`) alongside the existing map, never
  replacing it — every pre-existing caller and test keeps working
  unchanged with zero modifications. `resolveFieldError`
  (`formengine/status.go`) merges both sources, with matching precise
  enough to reject false-positive prefix collisions. Only `ERR`
  produces a blocking error today; `WRN`/`INF` are recorded and
  matchable but have no distinct visual treatment yet — no real
  caller for that yet, so not built speculatively.
- Direct proof test: a malformed URL inside a `document-links` row
  now correctly blocks submission and shows an error — confirmed
  this was silently broken before the fix, not just theoretically so.
- 10 new tests (9 handler + `FieldStatus`/`resolveFieldError`
  coverage), stress-run 10x clean.
- Full repo `go build`/`go vet`/`go test ./...` clean (15 packages),
  gofmt clean.
- **FE-23 is now fully closed** — schema rename, translation table,
  SourceResolver, GET path, and POST handling all built and tested.
  `FE-24` (parity testing) is unblocked.

## [0.2.36] — 2026-07-28

### Changed — formengine.ParseSchema exported

- `parseSchema` (package-private) → `ParseSchema`, exported. `Form`
  and `Form.Compiled` were already exported types/fields, but nothing
  outside `formengine` could actually construct a `*Form` to reach
  them — needed for the dynamic Asset form's upcoming POST handler to
  validate against the same compiled schema `Render` already builds,
  rather than inventing a parallel, `formengine`-owned validation API
  that would duplicate queryfy's own interface for no reason.
- Fixed a stale doc comment on the same function referencing "until
  stage 3 adds tab handling" — stage 3 has been done for a long time.
- Confirmed the array-field submission convention directly from
  `repeatable.go`'s own doc comment (written during stage 4/5,
  explicitly anticipating this moment): `{field.Name}_{item.Name}[]`,
  identical for `document-links` and `repeatable`.
- Full repo `go build`/`go vet`/`go test ./...` clean (15 packages).
- `docs/formengine/TRACKING.md`: FE-23 updated. Remaining before the
  handler itself: confirming which of the 42 fields are
  integer-typed in `asset.json`, since `r.FormValue` always returns
  strings but the compiled schema expects properly-typed values for
  those — the static form does this coercion before validation, and
  the dynamic handler needs to match it, not guess.

## [0.2.35] — 2026-07-28

### Changed — i18n hard requirement recorded, gap filed as T-17

- The tech team's requirement: during development, English and Spanish
  translation content stay in sync for every user-facing feature — a
  content-authoring discipline, not a runtime one. A simple
  server-side, per-request locale choice is sufficient; live
  client-side switching without a reload is explicitly not required,
  matching minty's server-rendered architecture rather than a
  ReactJS-style SPA.
- Investigated before filing, not assumed: confirmed no locale-
  selection mechanism exists anywhere in the app,
  `AssetNewDynamic`/`AssetEditDynamic` never wire `Translator` or set
  `RenderContext.Locale`, and `en.json`/`es.json` have zero Asset-form
  keys today — only nav-chrome content exists.
- Filed as T-17, gated the same way T-13 was (must land before FE-24,
  not before FE-23) — the dynamic Asset form isn't live to real users
  yet, so FE-24 is the real deadline, not FE-23's remaining piece.
  Deliberately not started now: FE-23's POST handler investigation was
  complete and implementation-ready, and pausing again would have been
  a third interruption on the same task.
- No code changes this entry — register and memory only.

## [0.2.34] — 2026-07-28

### Fixed — a real, silent data-loss bug in the persistence translation table

- `internal/ui/asset_xolu_fields.go`: `service_configs`' threshold
  sub-field mapped to `"umbral"`, matching the submitted
  `svc_umbral[]` form field name — but both real handlers
  (`AssetCreate` and `AssetUpdate`, confirmed identical) actually
  persist it under `"umbral_alarma"`. Left unfixed, the dynamic form
  would have silently lost every asset's threshold value the first
  time it was edited through either form after being touched by the
  other — exactly the coexistence hazard this table exists to
  prevent. Caught by reading the real POST handler before resuming
  FE-23's own POST work, not discovered after something shipped
  broken.
- Also confirmed, not fixed (out of scope — touching the static form
  contradicts the earlier "don't touch it, it's being retired"
  decision): the static `AssetEdit` handler never populates
  `ServiceConfigs` on load at all. Not a convention to match — there
  isn't one. Means the dynamic form will show more complete data here
  than the static one currently does once POST handling lands, which
  is a real, known difference for FE-24's parity comparison to expect,
  not a regression.
- Full repo `go build`/`go vet`/`go test ./...` clean (15 packages).
- `docs/formengine/TRACKING.md`: FE-23 updated. POST handling
  investigation resumed.

## [0.2.33] — 2026-07-28

### Changed — queryfy bumped to the real, published v0.3.2

- `github.com/ha1tch/queryfy` v0.3.1 -> v0.3.2, now that v0.3.2 is
  actually published (see RESOLVED.md's T-13 entry and 0.2.32 above
  for the fix itself — both left as written, since they were accurate
  at the time; this entry is the update, not a correction).
- Checked whether this closes out any workaround or spec update on
  Seam's side: it doesn't. Seam's own mapping code
  (`internal/formengine/i18n_errors.go`) was never built around the
  bug — it always used the correct wording for the code path Seam
  actually uses, since `formengine` never calls queryfy's `Compile()`.
  `I18N_SPEC.md` §4.4 was never wrong either. This bump is dependency
  hygiene, not a fix landing.
- `go.sum` updated to match (fetched via the real proxy, since v0.3.2
  wasn't cached from before this session).
- Full repo `go build`/`go vet`/`go test ./...` clean (15 packages)
  with `GOPROXY=off`, confirming everything needed is now properly
  cached, not just resolved during the one `go mod tidy` call.

## [0.2.32] — 2026-07-28

### Added — T-13 closed: §4.4 queryfy error mapping, plus a real upstream fix

See `docs/RESOLVED.md` for T-13's full closure record (§4.1, §4.2,
§4.4 — the complete filed scope).

- `internal/formengine/i18n_errors.go`: `ResolveQueryfyError`, matched
  against queryfy's actual message wording for the `jsonschema.FromJSON`
  path specifically — the one `formengine`'s own `parseSchema` uses —
  verified with an integration test that runs real validation through
  a real schema, not hand-typed fixtures guessing at the library's
  wording.
- **That verification surfaced a genuine bug in `queryfy` itself, not
  a spec problem** — confirmed by checking both possibilities before
  fixing either, per instruction: fix bugs at the source, don't work
  around them downstream. `compile.go`'s `Compile()` (an optional
  pre-compilation step `formengine` never calls) re-implemented
  numeric min/max validation with different wording
  (`"must be at least/at most %v"`) than `builders/number.go`'s
  original (`"must be >= /<= %v"`, what `jsonschema.FromJSON` actually
  produces) — same schema, same violation, two different messages
  depending on whether a caller compiled the schema first.
- Fixed at the source: cloned `github.com/ha1tch/queryfy` fresh,
  corrected `compile.go`'s `compileNumber` to match
  `builders/number.go` exactly (a two-line diff), confirmed via the
  library's own test suite (`go test .`, `./builders/...` all clean).
  Not yet pushed — lives in a local clone pending a decision on
  publishing, since that's a decision about a separate repository and
  its release process. Dormant for Seam today either way, since
  nothing here calls `Compile()`.
- I18N_SPEC.md's original §4.4 wording (`must be >= N`) was correct
  all along for this code path; an earlier session turn had wrongly
  "corrected" it based on `compile.go` alone without checking which
  code path `jsonschema.FromJSON` actually uses — reverted.
- 7 new tests, including one running real `queryfy.Validate()` against
  a real imported schema and asserting on the genuine `FieldError`
  messages produced. Also fixed `fakeTranslator` (this package's test
  double), which didn't perform `fmt.Sprintf` interpolation like the
  real `Translator` does — caught because the interpolation tests
  failed with a literal, unsubstituted `%s` in the output.
- Full repo `go build`/`go vet`/`go test ./...` clean (15 packages),
  gofmt clean; stress-run 10x on the real-validation-based tests.

## [0.2.31] — 2026-07-28

### Added — T-13: i18n resolution wired into formengine (§4.1/§4.2)

- Two design questions resolved before writing any code, not guessed
  at. **A real spec inconsistency**: §4.1's own resolution chain said
  `{modulo}.form.field...` (singular, no form-name segment), while
  §3.4's canonical naming convention — already in production, matching
  the real Asset schema's tab/section keys — says
  `{modulo}.forms.{nombre_formulario}.field...`. Fixed §4.1/§4.2 to
  match §3.4. **A missing mechanism**: neither `{modulo}` nor
  `{nombre_formulario}` existed anywhere — not a `Render()` parameter,
  not in `x-seam-form`. Resolved with two independently-overridable
  levels: `x-seam-form.module`/`name` as the schema's own default,
  `RenderContext.Module`/`Form` (present in the engine since stage 1,
  unwired until now) overriding per call when non-empty — motivated
  by a reusable-schema scenario (a card instantiated multiple times
  under different namespaces), not decided in the abstract.
- `internal/formengine/i18n.go`: `resolveTitle` (§4.1's 4-step chain),
  `resolveHelp` (§4.2's 3-step chain), `resolveLabel` (single-key, for
  tab/section/detail/embed labels — already explicit keys per §3.4,
  unlike title's literal-or-implicit-key hybrid).
- **Every path degrades gracefully to literal schema text** when
  `Translator` is nil or nothing resolves — confirmed empirically, not
  just by design intent: every formengine test written before
  `Translator` existed still passes unchanged, since none of them
  configure one.
- Wired into `renderFieldRow`, `fkPopoverAwareLabel`, `tabButton`,
  `renderSection`, `renderDetailPanel`, `renderEmbedTrigger` (the last
  two needed `*FormEngine` added to their signatures — small blast
  radius, 3 call sites, all within `tabPanel` which already had `e` in
  scope).
- 12 new tests, including an end-to-end proof through the real
  `Render()` pipeline — not just `resolveTitle` called in isolation —
  confirming `rc.Module`/`rc.Form` actually reach the rendered HTML.
- Full repo `go build`/`go vet`/`go test ./...` clean (15 packages),
  gofmt clean.
- `docs/TRACKING.md`: T-13 updated. Remaining: §4.4 (queryfy error
  mapping) — the piece FE-23's POST work is specifically waiting on.

## [0.2.30] — 2026-07-28

### Fixed — corrected a mistake from the previous entry

- **T-04 (Documents module) is not postponed** — the previous entry
  wrongly bundled it with FE-25's postponement. It's active, planned
  work, reverted to its original filed state (gated only on external
  seam-filer/fijuggler/thumble service readiness).
- What's actually postponed is specifically **FE-25's own mechanism**:
  a per-Asset-instance attachment tray. The real reason: whether
  documents should ever attach directly to an Asset instance at all,
  versus attaching to the chain of categories/types an asset belongs
  to, is an open architectural question — a different mechanism
  entirely, likely not a per-instance tray widget. Explicitly a
  separate discussion, not decided or designed further here. FE-25
  stays filed as a placeholder for whatever that turns out to be, not
  a commitment to build it as originally scoped.
- `docs/TRACKING.md` and `docs/formengine/TRACKING.md` corrected.

## [0.2.29] — 2026-07-28

### Changed — two scope decisions, no code

- **FE-25 (attachment tray) and T-04 (Documents module) postponed
  indefinitely** (the tech team). The assessment this was always waiting on
  — whether the tray is worth building before a real Documents module
  exists — could finally be made with formengine's actual capabilities
  in hand, rather than guessed at in the abstract.
- **FE-23's remaining POST/submission work paused in favor of T-13
  (i18n wiring) going first.** The earlier plan (raw queryfy error
  messages now, T-13 wraps them later) understated what T-13 actually
  needs: §4.4's error mapping requires real parameterisation (a
  `{min}`/`{max}` value, not just a translated label), meaning errors
  need to carry structured data through the handler, not a flat
  string a later step re-translates. Error messages also typically
  name the field, and the field's own display name is itself an
  i18n-resolved value — another reason text built now wouldn't match
  what T-13 resolves later. Nothing had been implemented for FE-23's
  POST path yet, so nothing needs unwinding.
- `docs/TRACKING.md` and `docs/formengine/TRACKING.md` updated with
  both decisions and the reasoning behind them.

## [0.2.28] — 2026-07-28

### Fixed — stale Go version requirement in DEPLOY.md/README.md

- Both said "Go 1.22 or later" -- understated what `go.mod` actually
  requires (`go 1.26.5`, matching the sandbox toolchain exactly).
  Someone following the docs with Go 1.22 installed would hit a build
  failure the docs never warned them about. Updated both to "Go
  1.26.5 or later", matching `go.mod`'s directive precisely rather
  than a rounded-down approximation.
- Checked for other stale Go version pins across build/CI config
  (Dockerfile, GitHub Actions workflows) -- neither exists in this
  repo, so the two doc references were the only ones.
- Full repo `go build`/`go vet`/`go test ./...` clean (15 packages).

## [0.2.27] — 2026-07-28

### Added — internal/ui/asset_dynamic_handlers.go: the dynamic Asset form's GET path

- `AssetNewDynamic`/`AssetEditDynamic`, routed at `/assets/dynamic/new`
  and `/assets/dynamic/{id}/edit` — a genuinely parallel path per
  §13.3's coexistence model, not a replacement. `AssetNew`/`AssetEdit`
  are untouched; FE-24 needs both a static and dynamic rendering of
  the same real asset to actually compare.
- **Verified, not assumed, that the new routes don't collide with the
  existing wildcard patterns** (`/assets/{id}`, `/assets/{id}/edit`):
  Go 1.22's `ServeMux` panics at registration time on genuinely
  ambiguous overlapping patterns, so a throwaway test directly
  constructing the real router confirmed no panic — direct evidence,
  not inferred from other tests happening to pass.
- `AssetEditDynamic` loads the real entity via `xoluClient.Get`,
  translates it through `AssetValuesFromXolu`, and renders via
  `FormEngine.Render` with `AssetSourceResolver` supplying dropdown
  options — the full chain, end to end, for the first time.
- **A live cross-check that paid off**: reading `AssetEdit`'s own
  `entity.Data["criticidad"]`/`entity.Data["ubicacion_fisica_id"]`/etc.
  reads line by line confirmed all 42 of `asset_xolu_fields.go`'s
  Spanish-side keys are exactly right — independent confirmation
  against the actual handler reading this data today, not just the
  earlier `mi.Name(...)` audit repeated.
- Tested against a combined `httptest.Server` simulating both
  endpoints the edit handler needs (OQL and `GET /api/v1/assets/{id}`,
  confirmed as the real path+shape from `pkg/client`'s `Get` method
  directly). 5 new tests: HTMX render, non-HTMX redirect, real-asset
  load with the Spanish->English translation verified in the actual
  rendered output, unknown ID, malformed ID.
- Full repo `go build`/`go vet`/`go test ./...` clean (15 packages),
  gofmt clean.
- `docs/formengine/TRACKING.md`: FE-23 updated. Remaining: POST/
  submission handling.

## [0.2.26] — 2026-07-28

### Added — internal/ui/asset_source_resolver.go: the xolu-backed SourceResolver

- `AssetSourceResolver` implements `formengine.SourceResolver` for all
  9 `x-seam-source` values the Asset schema uses (marcas, modelos,
  regiones, ubicaciones, custodios, monedas, centros-costo, alertas,
  asset-types). Every OQL query and label format read directly out of
  `masters_routes.go`/`asset_detail.go` before being copied — not
  reconstructed from `FORM_ENGINE_SPEC.md` §5's table alone, which
  turned out accurate for all nine sources this time (confirmed
  individually, not assumed given its two prior known inaccuracies
  elsewhere in the spec).
- Preserved `custodios`' `WHERE estado = true` filter — load-bearing,
  not incidental: the old form only ever offered active custodians,
  and a new form silently offering inactive ones would be a real,
  quiet behavior change. Confirmed `entity:centros-costo`'s actual
  xolu table is `centros_costo` (underscore), distinct from the
  hyphenated source key and URL path.
- **Real bug caught during implementation, not by a test discovering
  it first**: `toString()` — an existing helper already used
  throughout `internal/ui` — only handles the string case and
  silently returns `""` for anything else, including the `float64`
  xolu sends for numeric ids. Read its actual source before trusting
  it for `row["id"]`; fixed to route through `toInt64` +
  `fmt.Sprintf` instead. Kept as a standing regression test
  regardless of having been caught before shipping.
- Tested against a real `httptest.Server` simulating xolu's actual
  request shape — including catching that the real path is
  `/api/v1/oql/query`, not `/oql/query` as `pkg/client`'s `OQL`
  method's own path constant alone would suggest (`do()` prepends
  `/api/v1`; caught by the first test run failing against the wrong
  assumed path, not read from source ahead of time).
- 21 new tests, stress-run 10x clean.
- Full repo `go build`/`go vet`/`go test ./...` clean (15 packages),
  gofmt clean.
- `docs/formengine/TRACKING.md`: FE-23 updated. Remaining: the GET
  rendering path and POST/submission handling.

## [0.2.25] — 2026-07-28

### Added — internal/ui/asset_xolu_fields.go: the English<->Spanish persistence translation table

- `AssetValuesToXolu`/`AssetValuesFromXolu`: translates between the
  Asset schema's English field names and the Spanish attribute keys
  xolu already stores and the coexisting static form still reads/
  writes. The Spanish side isn't invented — it's the exact field list
  from the `mi.Name(...)` audit that caught the `name`/`type_id` bug
  (FE-22), so the two paths agree on real, confirmed data, not a
  guess.
- One-to-one invariant enforced structurally: the reverse map's own
  `init()` panics on any collision (a bad edit fails at binary
  startup, not silently at runtime), and re-asserted by a standing
  test rather than checked once and trusted forever.
- Handles the three `document-links` arrays (`loan_links`,
  `lifecycle_links`, `document_links`; `nombre`/`url`/`tipo` <->
  `name`/`url`/`type`) and `service_configs`
  (`medida`/`umbral`/`alerta_id` <-> `measure`/`threshold`/
  `alert_id`) recursively — confirmed by explicit round-trip tests on
  the array fields, not just one-directional scalar checks.
- Unrecognised fields pass through rather than get silently dropped,
  so extending the schema later doesn't lose data before this table
  catches up.
- 13 new tests, all passing on first run.

### Analysed, not built — the hyphen<->identifier conversion question

- Checked whether reversing the proposed convention (double
  underscore -> single underscore, single underscore -> hyphen) fit
  the existing data better. It doesn't: every hyphenated widget-type/
  source-entity string has zero underscores, every underscored schema
  field name has zero hyphens, and the codebase's own established
  pattern for deriving a Go identifier from a hyphenated string is
  hand-written CamelCase with the hyphen dropped (`loadCentrosCosto`),
  not underscore-encoding at all.
- No current caller needs a lossless round-trip conversion anywhere
  in the codebase — the function itself wasn't built, on the same
  "no speculative code without a real caller" grounds as elsewhere
  this session. Documented as a convention to apply if and when a
  concrete need appears, with its invariant stated precisely: no
  anchor string may ever contain a literal double underscore.
- Full repo `go build`/`go vet`/`go test ./...` clean (15 packages),
  gofmt clean.
- `docs/formengine/TRACKING.md`: FE-23 updated. Next: the xolu-backed
  `SourceResolver`.

## [0.2.24] — 2026-07-28

### Changed — Asset schema field names renamed to English

- All 42 top-level properties, plus the nested `document-links`/
  `repeatable` item fields, renamed from Spanish to English:
  `criticidad`->`criticality`, `calidad_tenencia`->`tenure_type`,
  `custodio_id`->`custodian_id`, `vto_comodato`->`loan_expiry`,
  `marca_id`->`brand_id`, and 27 more. Code identifiers — including
  schema field names — now stay in English throughout; bilingual
  (Spanish/English) UI support is entirely the i18n layer's
  responsibility (§4, T-13), not something field names should carry.
- Applied via structured JSON manipulation (parse, rename, rewrite),
  not text regex — the earlier `name`/`type_id` fix had already shown
  a blind substring replace can break nested fields sharing a name
  with an unrelated top-level one (it broke `document-links`' own
  `nombre` sub-field the first time).
- **Two real bugs the rename itself introduced, caught before they
  shipped**: `x-seam-visible-when`'s `field` reference
  (`calidad_tenencia` -> `tenure_type`) is a string *value*, not a
  JSON key, so the structural rename didn't touch it — left unfixed,
  the conditional visibility would have silently never matched. An
  i18n key path (`assets.forms.asset.field.vto_comodato`) also
  embedded the old field name despite not being a schema key at all.
  Both found by explicit post-rename verification, not assumed clean.
- `FORM_ENGINE_SPEC.md` kept in sync: §12's fragment, §3b's
  `visible_when` worked example, §7.3's fk-popover field table, and
  one stray prose mention — all updated. One reference deliberately
  left alone: §5's `entity:alertas` label template uses `tipo`/
  `criticidad` as *that* master entity's own fields, unrelated to
  Asset's renamed ones — confirmed by reading the surrounding table
  row before assuming, not renamed on pattern-match alone.
- Integration tests updated to match; full repo `go build`/`go vet`/
  `go test ./...` clean (15 packages), gofmt clean.
- **Not yet built**: the translation table in FE-23's persistence
  code mapping these English schema names to the Spanish attribute
  keys xolu already stores (and the coexisting static form still
  reads/writes) — confirmed necessary and scoped, next concrete step.

## [0.2.23] — 2026-07-28

### Added — systematic olu->xolu nomenclature check for the living spec docs

- `scripts/olu_to_xolu_docs.py`: a reusable `--check`/`--apply` tool,
  not a one-off pass, built specifically to address a real risk —
  spec documents describing not-yet-built modules (Documents, Media)
  still said `olu:"field_name"` in their example Go structs. Left
  alone, whoever implements those modules later (including a future
  session) would copy that straight into real code, silently
  reintroducing exactly what the rest of this rename was meant to
  retire.
- Six named, narrow rules — struct tags, method-call-style prose
  (`olu.Commit`), config-path-style prose (`olu.required`), INI
  section headers (`[olu]`), bold labels, generic prose — applied to
  7 hand-picked living-spec files (`FORM_ENGINE_SPEC.md`,
  `ADMIN_MODULE_SPEC.md`, `I18N_SPEC.md`, `SEAM_MEDIA_SPEC.md`,
  `DOCUMENTS_MODULE_SPEC.md`, `FSM_DESIGN.md`,
  `SEEDS-CATALOGO.md`): 148 changes total.
- Anything not matching one of the six rules is surfaced as
  "unclassified" rather than guessed at. Caught one genuine edge case
  this way on the first dry run: a filename reference to
  `docs/OLU_CLIENT_REMEDIATION.md` in `FSM_DESIGN.md`'s bibliography —
  that file stays named that on purpose (historical record), so
  renaming the *reference* without renaming the file would have
  produced a dead link. Added as a named, documented exclusion rather
  than silently handled.
- Historical documents — `CHANGELOG.md`, `docs/RESOLVED.md`,
  `docs/OLU_CLIENT_REMEDIATION.md` itself, `ROADMAP.md` (stale against
  a 0.1.0-patched243 snapshot), `docs/RBAC-COMPLETION-PLAN.md`,
  `docs/TRACKING.md`/`docs/KNOWN_ISSUES.md` (mixed live/historical
  content in the same document) — deliberately excluded from the file
  scope, each for a checked, stated reason.
- Registered as a dormant guard in `docs/KNOWN_ISSUES.md`'s existing
  table, alongside the formengine integration test — meant to be
  re-run periodically, not just once.
- One now-stale reference to the deleted `internal/olu` package found
  along the way and fixed, in `docs/KNOWN_ISSUES.md` itself.
- `docs/TRACKING.md`: T-07 updated with this completion.

## [0.2.22] — 2026-07-28

### Changed — Tier 3: external-facing names, hard cutover

- `OLU_URL` -> `XOLU_URL`, `SEAM_OLU_API_KEY` -> `SEAM_XOLU_API_KEY`,
  `SEAM_OLU_URL` -> `SEAM_XOLU_URL` (seam-admin), the `-olu-url` CLI
  flag -> `-xolu-url`, and the `json:"olu"` config tag -> `json:"xolu"`.
  Updated across `config.go`, `config_test.go`, `cmd/seam/main.go`,
  `cmd/seam-admin/main.go`, `config.example.json`, `DEPLOY.md`,
  `README.md`.
- Verified against xolu's own source before renaming anything, not
  assumed: these were never xolu-defined names to preserve
  compatibility with — confirmed xolu's own current convention is
  unambiguously `XOLU_`-prefixed (`XOLU_AUTH_TYPE`, `XOLU_API_KEYS`,
  `XOLU_PORT`, `XOLU_BASE_DIR`, read directly from its
  `pkg/config/config.go`), and its own `CHANGELOG.md` shows it went
  through this exact same `OLU_` -> `XOLU_` rename in its own history.
- **Real functional bug found and fixed, not just renamed**:
  `docker-compose.yml`'s xolu-container environment (`OLU_DATA_DIR`,
  `OLU_PORT`) didn't match anything xolu's current server actually
  reads — confirmed against its source, which expects `XOLU_BASE_DIR`
  and `XOLU_PORT`. The old names were silent no-ops; xolu would have
  fallen back to its own defaults regardless of what the compose file
  set. Fixed to the real names; also renamed the service/container/
  volume (`olu` -> `xolu`, `seam-olu` -> `seam-xolu`, `olu-data` ->
  `xolu-data`) for consistency.
- **Left deliberately unverified rather than guessed**: whether
  `ghcr.io/ha1tch/xolu` exists as a published image (vs. the current
  `ghcr.io/ha1tch/olu:latest`) — flagged with a TODO in the compose
  file itself. Same reasoning applied to
  `internal/handlers/api/handler.go`'s `"olu_error"` API error-code
  string: nothing in this codebase checks for it, but an external API
  consumer outside this repo might, so it's left alone rather than
  renamed on a guess.
- Full repo `go build`/`go vet`/`go test ./...` clean (15 packages),
  gofmt clean.
- `docs/TRACKING.md`: T-07 updated with Tier 3's completion. Also
  surfaces a much larger, separate scope: the `docs/*.md` spec corpus
  (`FORM_ENGINE_SPEC.md`, `SEAM_MEDIA_SPEC.md`,
  `DOCUMENTS_MODULE_SPEC.md`, and others) has hundreds of "olu"
  references, including an `olu:"field_name"` struct-tag convention
  proposed for not-yet-built modules — a real design question, not a
  mechanical rename, deliberately not touched in this pass.

## [0.2.21] — 2026-07-28

### Changed — completed the olu-to-xolu identifier rename (Tier 1 + Tier 2)

- Every remaining Go code identifier containing "olu" renamed to its
  xolu equivalent: struct fields (`h.olu`/`s.olu` -> `xoluClient`,
  exported `Handler.OLU` -> `XOLU`), local variables (`oluClient`,
  `oluOpts`, `oluErr`, `oluURL`, `oluClientAdapter`), and every
  type/function name (`OLUClient`, `OLUEntity`, `OLUAuditStore`,
  `NewOLUAuditStore`, `OLUUserStore`, `NewOLUUserStore`,
  `WriteOLUError`, `OLUConfig`, `CLIOLUClient`).
  `internal/auth/store_olu.go` renamed to `store_xolu.go`.
- Comment prose referencing "olu" as the backend name updated across
  26 files; stale test-assertion message strings ("olu.Create was not
  called" etc.) corrected for accuracy.
- **Real miss caught only by a final exhaustive grep, not the earlier
  targeted passes**: `OLUWithTenant` is a *method*, and its receiver
  clause (`func (h *Handler) OLUWithTenant(...)`) sits between `func`
  and the name — defeating the original declaration-detection regex,
  which anchored on `func` immediately followed by the identifier.
  Used roughly 50 times across `internal/handlers/api`; found and
  fixed once the exhaustive check ran.
- **Tier 3 deliberately untouched**: `OLU_URL`/`SEAM_OLU_API_KEY`
  environment variables, the `-olu-url` CLI flag, and the
  `json:"olu"` config tag all remain exactly as they were — external
  deployment contracts, not internal naming, referenced in `DEPLOY.md`,
  `docker-compose.yml`, `config.example.json`, and `README.md`. These
  stay open pending a decision among hard-cutover, backward-compatible
  dual-read, or leave-alone — not a call to make silently inside a
  mechanical rename pass.
- Verified precisely at each step, not assumed: for `Config.OLU`
  specifically, the Go field name changed to `XOLU` while its
  `json:"olu"` tag and the `fmt.Errorf("OLU URL is required")` message
  were deliberately left untouched, since both describe the
  still-unchanged external `OLU_URL` contract.
- Full repo `go build`/`go vet`/`go test ./...` clean (15 packages),
  gofmt clean.
- `docs/TRACKING.md`: T-07 updated with the completed rename; stays
  open for the still-untouched Tier 3 decision and the separate T-02
  Phase 1 tenant-JWT credential swap.

## [0.2.20] — 2026-07-28

Patch release, no functional changes — checkpoint marking the
olu-to-xolu client migration (0.2.19) as its own release point.

## [0.2.19] — 2026-07-28

### Changed — migrated from internal/olu to xolu's official pkg/client

- `internal/olu` (Seam's hand-rolled xolu client) removed entirely.
  Every caller now uses `github.com/ha1tch/xolu/pkg/client` directly,
  imported as `xolu`.
- 24 files migrated across `internal/server`, `internal/ui`,
  `internal/auth`, `internal/handlers/api`, `internal/reports`,
  `internal/alerts`, and their test files.
- Verified against two xolu snapshots before touching anything —
  v0.16.16 (latest published tag) and a v0.16.27 checkpoint provided
  directly — confirmed `pkg/client` is byte-for-byte identical between
  them, so the migration isn't pinned to a stale read. `pkg/client`
  itself is pure standard library, zero third-party imports, confirmed
  by reading every import block directly rather than assumed from the
  module's own (much larger) server-side dependency tree.
- Empirically confirmed before migrating (not assumed): Seam calls
  exactly 7 methods across the whole codebase — `OQL`, `Get`, `Create`,
  `Update`, `Delete`, `List`, `Search` — all with identical signatures
  in `pkg/client`, plus `Entity`/`ListParams`/`SearchParams`/
  `ListResult`/`ClientOption`/`Error`, all structurally identical.
  `Error`'s field-name difference (`StatusCode`/`Details` vs
  `HTTPStatus`/`Detail`) turned out moot — `pkg/client` keeps the old
  names as documented backward-compatible deprecated aliases.
- Struct field and local variable names (`h.olu`, `s.olu`, `oluClient`,
  etc.) deliberately left as-is — only the package-qualified type
  references changed. Smaller, lower-risk diff; renaming those is a
  separable cosmetic decision, not bundled into this migration.
- **Self-inflicted bug caught immediately by the first build attempt,
  not by review**: a first-pass regex renaming `olu.` to `xolu.`
  correctly caught package-qualified references (`olu.Client` ->
  `xolu.Client`) but also incorrectly caught field-access chains
  (`s.olu.OQL(...)` -> `s.xolu.OQL(...)`, where `olu` is a struct field
  name, not a package qualifier). The compiler pointed at exactly where
  the two needed to be told apart (`s.xolu undefined`); fixed with a
  negative-lookbehind pattern (`.xolu.` can never legitimately occur,
  since a package qualifier is never itself accessed as a field) and
  reapplied cleanly to the remaining files from the start.
- Full repo `go build`/`go vet`/`go test ./...` clean (15 packages),
  gofmt clean.
- `docs/TRACKING.md`: T-07 updated, not closed — the client migration
  itself is done, but the tenant-JWT credential swap it also covers
  (T-02 Phase 1: per-tenant `xolu.WithJWT(...)` instead of one shared
  `xolu.WithAPIKey(...)`) is real, separate work, untouched by this
  pass, and stays open under T-07 until done.

## [0.2.18] — 2026-07-28

### Fixed — a serious field-naming bug caught before FE-23's handler work began

- Starting FE-23 (convert the Asset form handlers to the dynamic
  engine) required cross-checking every field name the real POST
  handlers expect. That surfaced a real problem: the schema (extracted
  from FORM_ENGINE_SPEC.md §12) used `"nombre"` and `"tipo_id"` for the
  asset's name and type, but the real static form's actual HTML field
  names for exactly those two fields are `"name"` and `"type_id"`
  (English) — confirmed against every `mi.Name(...)` call in
  `asset_detail.go`, not assumed from the otherwise-consistent Spanish
  naming the rest of the form uses.
- **Left uncaught, this would have silently broken every asset
  submission through the dynamic form**: the real `AssetCreate` handler
  reads `r.FormValue("name")`; a schema-driven POST sending `nombre`
  instead would never populate it, and the asset's name would save as
  empty on every single create or edit — no error, no warning, just
  silently wrong data.
- Fixed in three places: `asset.json` (with a first, overly-broad
  attempt caught before it shipped — a blind text replace of `"nombre"`
  also renamed the *legitimate* `nombre` sub-field inside the three
  `document-links` item schemas, which needed to stay `nombre`; caught
  by an explicit post-write verification, not assumed correct from the
  edit script's own console output), `FORM_ENGINE_SPEC.md` §12 (dated
  correction note added, matching the `HiddenRows` precedent), and the
  integration tests.
- Confirmed as a side effect: the existing `parseDocumentLinksToAny`/
  `parseDocumentLinks` handler helpers already expect exactly this
  schema's `{field}_nombre[]` field-naming convention for document
  links — no changes needed there when FE-23's handler work resumes.
- Also caught and fixed a tooling mistake in the process: a Python
  script computed a set of replacements correctly (confirmed by its own
  printed occurrence counts) but crashed on a later assertion *before*
  reaching the file write — meaning the earlier, valid replacements
  never actually reached disk, despite the console output implying
  success. Only caught by verifying the actual file content afterward,
  not by trusting the script's report. Redone correctly with an
  explicit post-write `grep` check.
- Full repo `go build`/`go vet`/`go test ./...` clean (16 packages),
  gofmt clean.
- `docs/formengine/TRACKING.md`: FE-23 scoped for the actual handler
  conversion work (SourceResolver, GET rendering path, POST handling)
  — not yet started.

## [0.2.17] — 2026-07-28

### Added — FE-22 closed: the real Asset schema, authored and verified

- `internal/formengine/schemas/asset/asset.json` + `asset.go`: the full
  4-tab, 42-property Asset schema, extracted from FORM_ENGINE_SPEC.md
  §12's example fragment and then verified field by field against the
  real static form (`internal/ui/asset_detail.go`, `routes.go`) rather
  than trusted as written.
- **Two real inaccuracies found and fixed, not just carried forward**:
  §12's `status` enum (`active`/`silent`/`alert`/`lost`) and
  `connectivity_tier` enum (`qr`/`gateway`/`lora`/`gps`) don't match
  what `routes.go` actually hardcodes and persists
  (`active`/`inactive`/`maintenance`/`lost` and
  `qr_only`/`gateway`/`lora_mesh`/`gps_cellular`, confirmed identical
  across all 5 occurrences in the handler). Using the spec's guessed
  values verbatim would have silently corrupted status/connectivity for
  any real asset whose value didn't happen to collide with the spec's
  vocabulary — existing data would render unselected, and submitting
  the form would write the wrong value. `vto_comodato`'s
  `x-seam-visible-when` (§3b.2's own worked example, missing from
  §12's fragment) was added.
- **One discrepancy deliberately preserved, not silently fixed**: the
  real form's `region_id` label reads "Región *" but `assetFKSelect`
  never actually enforces it as required (no `required` attribute,
  empty option always present) — an apparent pre-existing cosmetic bug
  in the static form. The schema reproduces this exactly (title carries
  "*", `region_id` absent from `required`) for faithful parity, since
  FE-24's regression comparison depends on the schema matching current
  *behaviour*, not a corrected version of it. Worth a deliberate
  decision later, not a schema-authoring judgement call now.
- Proven end to end, not just built-clean: the actual embedded schema
  rendered through the actual `FormEngine.Render()`, with real
  `SourceResolver` data for every `x-seam-source` field, checked for
  the presence of a real error node (`text-red-600` styling — distinct
  from `asset-picker`'s deliberate `text-amber-600` "not implemented in
  the MVP" note, which a naive substring check on the shared
  `formengine:` prefix would have false-flagged), and specifically
  verified `vto_comodato`'s visibility toggling and the
  required/not-required split across real fields.
- 3 new tests (118 total in the package).
- Full repo `go build`/`go vet`/`go test ./...` clean (16 packages),
  gofmt clean.
- `docs/formengine/TRACKING.md`: FE-22 closed. FE-23 (convert the Asset
  form handlers to the dynamic engine) and FE-24 (parity test, then
  retire the static form) remain — T-01's actual closure gate.

## [0.2.16] — 2026-07-28

### Added — internal/formengine: repeatable and asset-type-select widgets

- `repeatable` (§4.2's generalisation of document-links) implemented:
  each row can hold any combination of standard widgets — select,
  number, source-backed select — dispatched through the normal
  widgetRegistry lookup rather than a hardcoded subset, matching §4.2's
  own framing ("cualquier combinación de widgets"). Field naming follows
  the same {field}_{item}[] convention as document-links; the current
  hand-coded serviceConfigsField uses an unrelated "svc_" abbreviation
  instead (checked directly against its source), so FE-23's handler
  conversion will need a small, expected update — in scope for that
  task, not a silent regression here.
- Real Go gotcha hit and fixed while wiring repeatable into the widget
  registry: `renderRepeatable`'s body refers back to `widgetRegistry`
  (to dispatch each row's items), and Go's initialization-cycle detector
  flags that even though it isn't a real cycle at runtime. Fixed by
  moving the map's population into `init()`.
- `asset-type-select` (§12's htmx-triggered widget) implemented on a
  refactored `renderSelect` core (`renderSelectWithExtraAttrs`), so the
  option-resolution and required/fk-popover logic can never silently
  drift between the two. The four htmx attributes were matched against
  `assetTypeSelectWithHTMX`'s real implementation before writing this,
  not reconstructed from the spec table entry alone.
- Known, documented limitation: repeatable rows reuse the standard
  per-field `id="field-..."` from each widget renderer, which collides
  across multiple rows of the same item (invalid, if harmless, HTML —
  nothing in this package's JS does a global id lookup on a repeatable
  row's fields). The original hand-coded serviceConfigsField sidesteps
  this by never setting an id at all. Left as a follow-up rather than
  solved by threading an id-override through every widget's signature
  with no second caller yet to validate the fix against.

### Fixed — a real, proven test-flakiness source: minty's attribute order

- Chasing an intermittently-failing test surfaced that minty's
  `Element.Attributes` is `map[string]string` internally (confirmed by
  reading `builder.go`/`minty.go` directly), and Go deliberately
  randomizes map iteration order per process — so the same field,
  rendered twice from byte-identical input, can legally emit its HTML
  attributes in a different order each time. Proved empirically: a
  throwaway test rendering the same schema 50 times in a loop failed on
  the very first repeat.
- This was a latent bug in 6 test call sites across 4 files, all using
  fixed-byte-window slicing to check whether two attributes appeared on
  the same element — one of them (`TestMultiTab_ActiveTabSkipsToFirstVisibleWhenFirstIsHidden`)
  had already surfaced as genuine, intermittent failure before this was
  traced to its cause. All 6 fixed; added a shared `extractTag(html,
  idx)` test helper (finds the real enclosing `<tag...>` boundaries
  instead of guessing a byte count) so this doesn't get reintroduced.
- Not a Go-version issue: map iteration has been intentionally
  randomized since essentially Go 1.0, in every version since,
  deliberately, specifically so code can't come to depend on it.
- Verified with repeated stress runs, not just a single green pass:
  the full package clean across 25 consecutive full runs, and the
  specifically-affected tests clean across 10 additional isolated runs.
- 5 new tests for asset-type-select (115 total in the package).
- Full repo `go build`/`go vet`/`go test ./...` clean (16 packages),
  gofmt clean.

## [0.2.15] — 2026-07-28

### Added — internal/formengine, stage 7 continued: document-links widget

- `document-links` (§4.2) implemented and registered: a dynamic
  add/remove-row list of `{nombre, url, tipo}` links. Field naming
  deliberately matches the existing hand-coded `DocumentLinksField`'s
  parallel-array convention (`name_nombre[]`, `name_url[]`,
  `name_tipo[]`) rather than a nested-object scheme, so existing POST
  handlers don't need to change to accept schema-driven output —
  §13.3's migration philosophy. Generalised over the field's `Items`
  (new: `Field.Items []Field`, parsed from JSON Schema `items.properties`
  with the same document-order-preserving walk the top-level properties
  already use), not hardcoded to nombre/url/tipo.
- Real gap caught immediately by tests, not review: the widget was
  implemented but never added to `widgetRegistry` — caught by every
  document-links test failing with the "not implemented" fallback
  message, fixed in one line.
- Two more instances of the session's recurring test-writing pitfalls,
  both caught and fixed: a naive `strings.Count` picked up the target
  substring inside the widget's own embedded JS (not just the intended
  HTML), and a literal-quote check didn't account for minty's
  HTML-entity-encoding of quotes in attribute values.
- A real tooling-discipline lapse this session, recorded here since it
  affected working method, not just code: repeated use of `str_replace`
  for Go source edits despite `ed.py` already being set up and in active
  use in this same repository — named directly in memory as the
  failure mode to avoid, and fallen into anyway. Switching to `ed.py`
  then produced a *different* mistake (a `--with` value reconstructing
  a full line when the matched span covers only the search term,
  corrupting a test file), repaired with guarded Python and an explicit
  occurrence-count check before writing. Memory tightened to remove the
  str_replace-for-trivial-edits exception entirely.
- 10 new tests (104 total in the package).
- Full repo `go build`/`go vet`/`go test ./...` clean (16 packages),
  gofmt clean.

## [0.2.14] — 2026-07-28

### Added — internal/formengine, stage 7 begins (Asset form parity)

- FE-21 closed: `ModalTriggerAttrs(schema, url, label) []mi.Attribute`
  implemented per §15.4's confirmed decision — `x-seam-form.width:
  "fullscreen"` triggers `SeamModal` (via `hx-get`/`hx-target`/
  `hx-swap`), anything else triggers `SeamModal2` (via a single
  `onclick`), with §3.9's width vocabulary (`sm`/`md`/`lg`/`xl`/`2xl`)
  mapped to the literal CSS pixel values `SeamModal2.open`'s
  `opts.maxWidth` actually expects — verified against its real
  implementation in `app.js` before writing the mapping, not assumed
  from the width table's Tailwind-class framing. The deprecated
  `modal_width` alias is honoured with the same width-wins precedence
  `parseSchema` itself applies.
- `x-seam-fk-popover` (§7) implemented: an info-icon button beside an
  FK select's label that fires `SeamPopover.show(this, fieldName)`,
  hidden unless the current value is a positive ID. Verified against
  `assetFKSelectWithJump`'s actual markup line by line before
  implementing, not reconstructed from spec prose alone.
- `asset-picker` (§8.3's explicit MVP stub) implemented: renders as a
  number input with a visual note, exactly as the spec calls for —
  the full search-and-navigate asset-graph picker is explicitly out of
  scope for the form engine MVP.
- Major finding reframing the rest of stage 7: §12 of
  FORM_ENGINE_SPEC.md already contains an essentially complete draft of
  the full 4-tab Asset schema — every tab, section, and field-level
  `x-seam-*` extension — matching what a field-by-field audit of the
  real static form (`internal/ui/asset_detail.go`) found independently.
  Turns FE-22 from "author a schema from scratch" into "extract, verify,
  and correct this against the real form."
- Three real gaps confirmed between the static form and formengine's
  widget set — `document-links`, `repeatable`, and the FK-jump button
  — were found to already be fully spec'd (§4.2, §7) rather than
  needing new design, once read carefully. Decision: build the missing
  primitives now for true parity rather than scope the schema down to
  what already exists.
- 6 new tests.

## [0.2.13] — 2026-07-28

### Fixed — FORM_ENGINE_SPEC.md's own HiddenRows inconsistency, at the source

- Stage 6 (v0.2.12) found and worked around a real inconsistency in
  the spec itself: §3c.2 declared `RenderContext.HiddenRows` as
  `[]int`, while §3c.4 declared the identical data
  (`ResolvedPerms.HiddenRows`, which exists to populate that exact
  field) as `map[string][]int`. The code was already fixed to the map
  shape in 0.2.12; this release corrects the spec document itself at
  §3c.2, so the two sections agree, and adds a dated correction note
  at the point of the fix explaining why the map shape is the
  authoritative one — a flat list can't express "hide row 2 in tab A
  but not tab B", and the spec's own first example schema is
  multi-tab.
- No code changes — `internal/formengine` already matched the
  corrected shape as of 0.2.12.

## [0.2.12] — 2026-07-28

### Added — internal/formengine, stage 6 (field permissions)

- `permissions.go`: `FieldPermission` (§3c.3), `PermService` interface
  and `ResolvedPerms` (§3c.4), and `ResolvePermissions` — the pure
  role-priority resolution algorithm (hidden > readonly > editable,
  restrictive-wins across conflicting rules whether from one role or
  several). Same division of responsibility as `SourceResolver`:
  formengine owns the types and the algorithm, not olu-backed storage —
  there's no real caller yet (Administration module, T-02, doesn't
  exist), so the concrete implementation isn't built speculatively.
- Most of §3c's actual rendering-side application turned out to already
  be correct from earlier stages — `RenderContext` already had every
  field §3c.2 describes since stage 1/3, and every §3c.6 edge case
  (all-fields-individually-hidden tab omission, active-tab skip when
  the default is hidden, partial section hiding, embeds suppressed but
  details still showing on a read-only tab, `visible_when` evaluating
  against a permission-hidden field's persisted value) turned out
  already correct by construction. Verified with new tests rather than
  assumed — several of those edge cases had no test proving it before
  this stage.
- **Real spec inconsistency found and fixed**: §3c.2 declares
  `RenderContext.HiddenRows` as `[]int`; §3c.4 declares the identical
  data (`ResolvedPerms.HiddenRows`, which exists specifically to
  populate that field) as `map[string][]int`. A flat list can't
  express "hide row 2 in tab A but not tab B" — and the spec's own
  first example schema is multi-tab. Fixed `RenderContext.HiddenRows`
  to the map shape (§3c.4's, the only one well-defined for more than
  one tab) before any real caller could ship against the broken flat
  version — small blast radius, caught while building the resolver
  that's supposed to populate it correctly.
- §3c.6's required-field exemption: `RequiredFieldsExemptByPermission`
  — distinct from stage 5's `CheckHiddenFieldSubmission` (that's about
  `visible_when`-driven hiding, §15.2; this is permission-driven
  hiding via `RenderContext.HiddenFields`, §3c — deliberately separate
  mechanisms per §3c.1's own table, so deliberately separate functions).
- §3c.7 (Admin module permission-management UI) and §3c.8 (attachment
  tray permissions) are out of scope here — Administration module
  territory and stage 8 (already deferred, FE-25) respectively.
- 24 new tests (85 total in the package).
- Full repo `go build`/`go vet`/`go test ./...` clean (16 packages),
  gofmt clean.
- `docs/formengine/TRACKING.md`: FE-18 through FE-20 closed. Stage 7
  (Asset form parity — T-01's actual closure gate) is next.

## [0.2.11] — 2026-07-28

### Added — internal/formengine, stage 5 (conditional visibility)

- `visibility.go`: `VisibleWhen` condition tree and `EvaluateVisibleWhen`
  — all ten §3b.4 operators (`eq`/`neq`/`in`/`not_in`/`empty`/
  `not_empty`/`gt`/`gte`/`lt`/`lte`), nested `all_of`/`any_of`
  composition (§3b.5). Unknown operators and references to fields
  missing from the values map both fail closed (condition not met),
  matching §3b.8. Fully unit-tested in isolation before anything used
  it.
- `x-seam-visible-when` (fields) and `visible_when` (sections, rows,
  tabs) now parse throughout `schema.go`.
- `renderer.go`: field/section/row elements with a condition stay in
  the DOM and get `display:none` toggled — deliberately different from
  the existing permission-based hiding (`HiddenFields` etc.), which
  omits entirely (§3b.6). Tab-level `visible_when` is evaluated once at
  render time and omits like `HiddenTabs` — a documented scope choice,
  not an oversight: no spec example needs a reactively-visible tab, and
  reconciling that with the tab-switch script is real scope with
  nothing concrete driving it yet.
- `§3b.7` (required only enforced while the condition holds) needed a
  real fix mid-stage: the client evaluator was restoring `required` from
  a DOM-transition-history marker, which is never set for a field that
  starts hidden — so it could never become required on first reveal.
  Caught by an actual jsdom run, not by reading the code. Fixed by
  sourcing required-ness from a schema-level `data-schema-required`
  attribute instead of DOM history. A second bug in the same pass: a
  literal backtick inside a JS comment prematurely terminated the
  enclosing Go raw-string literal — also only surfaced by actually
  building, not by review.
- Client-side evaluator (`visibilityScript`) mirrors the Go evaluator's
  semantics exactly, verified with real jsdom runs (not just
  `node --check`) including the composite `all_of` case across two
  independent trigger fields.
- §15.2 (FE-17): `CheckHiddenFieldSubmission` — the dual policy for a
  submitted value on a field whose own condition is false at submit
  time: silently discarded by default, or rejected if the field sets
  `x-seam-hidden-reject: true`. Exported for a caller's handler to use
  after parsing POST values — formengine renders, it doesn't parse
  requests or persist data, same division of responsibility as the
  CSRF token.
- 18 new tests (67 total in the package), including three that caught
  their own false positives before being trusted: two hit the recurring
  naive-substring-match pitfall (matching text inside the script's own
  JS rather than actual HTML attributes) and one made a wrong
  assumption about what "one copy of the script" looks like when a
  function name legitimately appears twice (definition + call site)
  within a single copy.
- Full repo `go build`/`go vet`/`go test ./...` clean (16 packages),
  `node --check` clean, gofmt clean.
- `docs/formengine/TRACKING.md`: FE-15 through FE-17 closed. Stage 6
  (field permissions) is next.

## [0.2.10] — 2026-07-28

### Added — mintydyn upstream proposal, filed in-repo

- `docs/proposals/mintydyn-composite-conditions.md`: the composite
  `all_of`/`any_of` condition proposal for `github.com/ha1tch/minty`'s
  `mintydyn` subpackage, moved from an ephemeral local clone into this
  repository's own `docs/proposals/` (first use of that document kind
  here) so it's durably reachable rather than only living in `/tmp`.
  Scope-noted clearly: targets an external dependency, not Seam's own
  architecture — kept here because T-16 depends on its outcome.
- `docs/TRACKING.md`: T-16 updated to point at the in-repo path.

## [0.2.9] — 2026-07-28

### Fixed — real bug found during a requested JS/TS audit

- `web/static/js/app.js` defined `window.SeamConfirm` **twice**. The
  second definition silently overwrote the first at load time, making
  155 lines of dead code — and, critically, the surviving (second)
  definition was missing `dangerName` interpolation entirely. Every
  real call site passing `{dangerName: '...'}` (asset deletion, sensor
  deletion, asset-type deletion — `internal/ui/asset_detail.go`,
  `listing.go`, `pages.go`) was silently getting a generic, incomplete
  confirmation message instead of the intended "…the entry for
  **Name**?" wording. Fixed by removing the incomplete duplicate;
  proven with a real jsdom behavioral test, not just code reading —
  confirmed `dangerName` now interpolates correctly, then a full
  smoke test that every other `window.Seam*` global still loads clean.

### Changed — consolidated verified-duplicate tab-switching JS

- `internal/formengine/renderer.go`'s `tabSwitchScript` and
  `internal/ui/asset_detail.go`'s tab-switch script were byte-for-byte
  identical (confirmed by direct diff), just under different class
  prefixes (`fe-tab-*` vs `asset-tab-*`) — a known gap flagged in code
  comments back at stage 3. Extracted into one shared
  `window.SeamTabs.wire(panelSelector, btnSelector)` in `app.js`; both
  call sites now call it instead of embedding the full script. Proven
  with a jsdom behavioral test (real click, real tab switch, real
  active-class toggle), not just a syntax check.

### Investigated — TypeScript verdict: not needed

- Direct request to assess whether the client-side JS footprint
  justifies TypeScript. Verdict: no. The JS here is small, generated
  server-side from Go (Go is the source of truth), and the one place
  with real logic to get wrong (`visibility.go`'s evaluator) already
  has its JS mirror written to match the Go semantics exactly and
  tested for syntax + real DOM behavior via jsdom. TypeScript would
  add npm/node/a transpile step to the release pipeline for a JS
  surface that doesn't currently justify it. Revisit only if the JS
  surface grows substantially from here.

### Investigated — minty's own dynamic-component support (mintydyn)

- Read `mintydyn`'s full source (not just its README) — it's part of
  the already-vendored `github.com/ha1tch/minty` module, zero new
  dependency. Confirmed `go.mod` is already pinned to the latest
  available version (`v0.1.0`, verified against the module proxy's
  `@latest`, no newer tag exists).
- `mdy.Tabs()` is a real candidate to replace hand-rolled tab
  rendering, but not a clear-cut win: it generates a full
  `DynamicComponent_<id>` class **per instance**, not shared across
  components on a page (confirmed in `javascript.go`), so adopting it
  broadly could mean *more* total JS than the current shared-function
  approach on a page with multiple dynamic components. Filed as T-16
  rather than acted on speculatively.
- `mdy.Form()`/`ShowWhen()`/`DependencyRule` does **not** support
  `all_of`/`any_of` composite conditions — confirmed by a full-text
  search of the source, not assumed. `FORM_ENGINE_SPEC.md` §3b.5
  requires nested composites and `internal/formengine/visibility.go`
  already correctly implements and tests them. The custom evaluator
  stays for this reason.
- Full repo `go build`/`go vet`/`go test ./...` clean (16 packages),
  `node --check` clean on `app.js`, gofmt clean.

## [0.2.8] — 2026-07-28

### Added — internal/formengine, stage 4 (details panels, embeds)

- `schema.go`: `DetailPanel` (§3.5) and `EmbedForm` (§3.6) types with
  explicit JSON tags (snake_case wire keys like `trigger_label` and
  `refresh_detail` don't bridge to PascalCase Go fields via Go's
  case-insensitive fallback alone — caught before it became a silent
  parse bug, not after).
- `renderer.go`: details/embeds render only when `RenderContext.IsEdit`
  is true (§3.7); embeds additionally never render on a tab in
  `ReadOnlyTabs` (§10.1 step v — they have their own submit cycle).
  `{id}` substitution in detail sources and embed endpoints. Detail
  panels render as htmx-loaded placeholders (`hx-trigger="revealed"`,
  matching §3.5's own example); `position:"right"` panels split into a
  side column, `"bottom"`/`"inline"` stack normally (`"inline"`
  positioning degrades to `"bottom"` — no schema this stage targets
  uses it, and true inline placement needs more schema vocabulary than
  a tab-level array currently carries).
- Embed triggers: `mode:"modal2"` uses the established
  `SeamModal2.open(title, url)` convention (matched against actual
  existing call sites in `internal/ui/listing.go` before use, not
  invented); `mode:"inline"` reveals an htmx-loaded container.
  `mode:"drawer"` — see below.
- **`mode:"drawer"` — real open question, not guessed at.** Asked
  directly whether it means "open fijuggler." Confirmed fijuggler isn't
  vendored anywhere in this repo; `SEAM_MEDIA_SPEC.md` §2 documents its
  actual integration as `SeamModal.open({width:"fullscreen"})`,
  explicitly not an embedded drawer. Filed as T-14 — not blocking,
  since neither of this stage's two target embeds (`sensor_binding`,
  `qr_assign`) use `"drawer"`.
- `refresh_detail` (§3.7): emits a discoverable
  `data-refresh-detail="detail-<id>"` hook on the triggering embed.
  Reloading the named panel after a successful submit needs a small JS
  listener this package can't provide (it renders the trigger, not the
  embed's own loaded sub-form) — filed as T-15, deferred to when a real
  embed gets wired end to end.
- 10 new tests (44 total in the package).
- Full repo `go build`/`go vet`/`go test ./...` clean (16 packages),
  gofmt clean.
- `docs/formengine/TRACKING.md`: FE-12 through FE-14 closed. Stage 5
  (conditional visibility) is next.

## [0.2.7] — 2026-07-28

### Added — internal/formengine, stage 3 (tabs, sections, row layout)

- `schema.go`: `Tab`/`Row`/`Section` types; `x-seam-form.tabs` parsing
  handling both wire shapes — plain `sections` (array of full section
  objects, §3.8 "compatibilidad con sections directo", becomes one
  full-width row per section) and `rows` + `section_defs` (row.sections
  are string IDs resolved against section_defs, for horizontal layout
  control). §15.1 resolved decision implemented: a `section_defs` entry
  not referenced by any row rejects the schema at load time; a row
  referencing an undefined `section_defs` ID also rejects.
- `renderer.go`: tab bar + tab panels, same interaction pattern as the
  existing static Asset form (`internal/ui/asset_detail.go`'s
  `asset-tab-btn`/`asset-tab-panel`), reimplemented under an `fe-tab-*`
  namespace so the two never collide. 12-column row grid with §3.8's
  width vocabulary (`full`/`1/2`/`1/3`/`2/3`/`1/4`/`3/4`); missing
  widths auto-split the remaining budget evenly. Hidden/read-only tabs,
  sections, and rows (by index) all apply; empty sections and rows with
  no visible content collapse automatically rather than rendering blank
  chrome.
- 16 new tests (33 total in the package), including the §15.1 rejection
  cases and a realistic two-tab schema exercising hidden/read-only
  tabs, sections, and row widths together. One test-counting bug caught
  and fixed along the way — a naive substring count matched the tab-
  switch script's own `querySelectorAll('.fe-tab-btn')` text, not just
  actual button elements; not a renderer bug, a test-precision one.
- Full repo `go build`/`go vet`/`go test ./...` clean (16 packages),
  gofmt clean.
- `docs/formengine/TRACKING.md`: FE-09 through FE-11 closed. Stage 4
  (details, embeds) is next.

## [0.2.6] — 2026-07-28

### Changed — favicon.svg rebuilt from a vectorised trace of the real S mark

- Replaced the old shelf-pictogram `favicon.svg` with a proper vector
  trace of `icon-192.png`'s actual S mark.
- OCR run first as a diagnostic (per instruction): single-character-mode
  tesseract reads the raw mark as `"="` at 96% confidence, not `"S"` —
  recorded honestly rather than forced. Looser layout-mode OCR gives
  `"Ss"` at only 58% confidence.
- `icon-192.png` has no alpha channel (fully opaque, white background) —
  isolated the S via colour-distance thresholding, not alpha.
- Morphological closing removed the tiny holes the internal circuit-
  trace overlay left in the mask, keeping just the S silhouette.
- Traced with `potrace`. First attempt (`--tight`) scored only 0.67 IoU
  against the source raster — an aspect-ratio distortion caught by
  measuring, not by eyeballing. Second attempt: 0.9961 IoU.
- Gradient sampled from the actual corner pixels of the shape (`#1f1f47`
  dark navy to `#1da4bb` cyan, diagonal), not guessed.
- Checked legible from 16px to 256px before writing to the real path.
- `favicon.ico` (separate raster file) still has the old design and is
  now inconsistent with `favicon.svg` — flagged, not touched; wasn't
  asked for and is a different kind of task (multi-resolution raster
  regeneration vs. vectorisation).

## [0.2.5] — 2026-07-28

### Changed — "Shelf AMS" baked-in wordmark text replaced with "SEAM"

- 8 pixmaps had "Shelf AMS" literally rendered into them as pixels:
  `logo-160.png`, `logo-160-dark.png`, `logo-320.png`, `logo-320-dark.png`,
  `logo-landscape.png`, `logo-landscape-dark.png`,
  `logo-landscape-sm.png`, `logo-landscape-sm-dark.png`.
- Measured, not eyeballed: numpy row-band/column-gap analysis on the
  alpha channel located the exact "Shelf" bounding box in each file —
  handles both layouts present in this asset set (icon-above-text for
  the square logos, icon-beside-text for the landscape ones), gradient
  colours sampled per file from actual opaque pixels rather than
  assumed (light and dark variants confirmed genuinely different).
- Replacement set in Zalando Sans Expanded (Google Fonts), height-
  matched to the original glyphs then horizontally scaled to exactly
  the measured width, so it drops into the space "Shelf" occupied
  without disturbing "AMS" or the icon mark.
- Verified per file, not assumed: everything outside the measured box
  is numpy-array-identical to the original.
- `favicon.svg` confirmed as a separate, minor, low-priority item — a
  literal shelf pictogram (not text), needs a redesign or an easy swap
  to reuse the existing S mark, not a text substitution.

## [0.2.4] — 2026-07-28

### Changed — QR prefix resolved: SEAM

- `SHF` → `SEAM` everywhere: `qrcode.go`'s `DefaultConfig`, `internal/config/config.go`'s `QRPrefix` default, `config.example.json`, and every corresponding test (`qrcode_test.go`, `qr_regression_test.go`). Verified via `Validate()`'s `expectedLen := len(prefix) + Length` that prefix length is additive, not carved out of a fixed budget — nothing in this codebase actually needs the 3-character `SEA` fallback.
- Clarified the icon question: `favicon.svg` (browser-tab icon only) is confirmed, from direct read of its SVG source, to be a literal shelf pictogram — three horizontal bars, two small boxes — unrelated to the main "S" logo/wordmark used throughout the app (`logo-160.png`, `logo-landscape.png`, etc.), which is untouched and correct as-is. Only the small tab icon needs attention, and it's a design swap, not a text change.
- Module path (`github.com/ha1tch/seam-ui`) treated as confirmed after no correction across repeated flagging.
- Still open, low urgency: `qr_base_url` domain placeholder; `favicon.svg` redesign/swap.

## [0.2.3] — 2026-07-28

### Changed — Shelf → Seam rename, Pass 2 (texts and branding) + cleanup

- All markdown docs (same historical exclusions as Pass 1), seed scripts,
  the PWA manifest, the three locale files, `api/openapi.yaml`,
  `docker-compose.yml`. `SEAM_MEDIA_SPEC.md` renamed from its old
  filename, content updated including its own sub-brand names
  (`seam-filer`, `seam-media`).
- `web/static/css/app.css` — this one mattered functionally, not just
  cosmetically: it still had the old transition/keyframe names
  (`--shelf-transition`, `shelf-main`, ...) while the Go-embedded JS Pass
  1 already touched was using the new ones. A real, live CSS/JS mismatch
  existed between the two passes until this was caught and fixed.
- Two verbatim quotes of things actually said earlier in this
  conversation, found in `docs/TRACKING.md`, were protected from the
  sweep and confirmed unchanged afterward.
- 31 files, 631 lines.
- **Cleanup, found via a full-tree residual grep after Pass 2**: five
  `.sh` scripts (`syncver.sh`, `release.sh`, `scripts/bundle.sh`,
  `scripts/launch.sh`, `scripts/build-demo.sh`) that neither pass's file-
  extension list had covered. 5 files, 102 lines, bash syntax verified
  clean after.
- **Real bug caught in that cleanup**: `syncver.sh`'s own Go-file-
  generation template had the old product name hardcoded in a heredoc —
  every version bump was silently re-introducing it into
  `internal/version/version.go`, undoing Pass 1 each time. Fixed at the
  template, not just the generated file.
- **Self-referential corruption found and fixed**: `docs/TRACKING.md`'s
  and `docs/KNOWN_ISSUES.md`'s own prose *describing* the rename (using
  the old name as the "from" side of "X → Seam") got swept by the same
  substitution that was supposed to only touch live branding. Both
  rewritten to describe the rename without being corrupted by it.
- Full `go build`/`go vet`/`go test ./...` clean (16 packages), JS syntax
  clean, gofmt clean, all JSON valid, full-tree residual sweep clean
  except the deliberately excluded historical files.
- Still open, all needing a decision: the module path suffix choice made
  unilaterally in Pass 1 (treating silence as acceptance); the
  `qr_base_url` domain; the `"SHF"` QR prefix; `favicon.svg`'s actual
  shelf-shaped icon design, which needs a redesign, not a text change.

## [0.2.2] — 2026-07-28

### Changed — Shelf → Seam rename, Pass 1 (packages and identifiers)

- Go module path: `github.com/ha1tch/shelf-ui` → `github.com/ha1tch/seam-ui`
  (go.mod + every import statement). **Not explicitly confirmed** — a
  decision made during execution, flagged in T-05 for correction if wrong.
- Go/JS identifiers: `ShelfModal`/`ShelfModal2`/`ShelfPopover`/`ShelfNav`/
  `ShelfUserMenu`/`ShelfConfirm`/`ShelfAPIURL`/`ShelfUI` → `Seam*`.
- 26 `SHELF_*` environment variables → `SEAM_*`.
- `x-shelf-*` JSON Schema extension keys → `x-seam-*` (schema.go's `Field`
  struct tags, widgets.go, all schema JSON files, spec docs referencing
  the convention).
- `cmd/shelf` → `cmd/seam`, `cmd/shelf-admin` → `cmd/seam-admin`;
  `web/static/js/shelf-fsm-editor.js` → `seam-fsm-editor.js`.
- Makefile, package.json, package-lock.json updated.
- 83 files, 580 lines changed via a dry-run-then-apply Python script
  (never sed/awk), explicit UTF-8 encoding, reviewed before applying.
  Historical record never touched: CHANGELOG.md, docs/RESOLVED.md,
  docs/formengine/RESOLVED.md, .ed-journal.json.
- gofmt normalized repo-wide afterward — identifier length changes
  (Shelf→Seam is one character shorter) shifted some pre-existing
  alignment.
- Full `go build`/`go vet`/`go test ./...` clean (16 packages), JS syntax
  clean, gofmt clean.
- **Flagged, not auto-swept**: `qr_base_url` domain
  (`https://shelf.app/s/`, no confirmed Seam domain), QR prefix `"SHF"`
  (physical-label implications).
- Pass 2 (text/branding) not started.

## [0.2.1] — 2026-07-28

### Changed — version scheme switched to plain semver

- Versioning moves from `0.1.0-patchedNNN` to plain `X.Y.Z` semver,
  starting at `0.2.1`. Increments continue from this patch number going
  forward (`0.2.2`, `0.2.3`, ...). Historical `patched1`–`patched252`
  entries below are retained as-is — no renumbering of history.
- `repoman/` and `.repoman.json` added to `release.sh`'s checkpoint
  source list — both are genuinely part of the repo's committed state
  now (installed and used throughout the T-01 work) and were missing
  from the list only because it predates repoman's installation.
- This is the first checkpoint cut via `release.sh` rather than assembled
  by hand — full pipeline: version/CHANGELOG consistency, Tailwind CSS
  build, Go compile + test, binary/`.env`/DB guards, SHA-256 checksum.

## [0.1.0-patched252] — 2026-07-28

### Decided — xolu's native /fsm is the runtime FSM engine

- T-12 closed: xolu `/fsm` is the runtime for both Assets Management and
  the CMMS module's Workflow, no ambiguity. `fsm-toolkit` may still be
  imported separately for a graphical/TUI FSM designer (design-time
  tooling) — explicitly undecided and not urgent, and not a conflict with
  the above (same package used in two non-competing roles guarantees
  identical behaviour, not duplication).
- `docs/FSM_DESIGN.md` given a reconciliation banner: its §5
  storage-format-migration target (fsm-toolkit as runtime) is superseded;
  its domain examples and per-state metadata ideas are retained as
  possible input to whatever eventually authors FSM definitions.
- T-03 updated: the future ASSETS_MODULE_SPEC.md must specify migrating
  `internal/fsm`'s role onto xolu's native `/fsm`.
- `docs/ORDER_ENGINE_INTEGRATION_NOTES.md` §3.4 marked confirmed rather
  than merely recommended.
- `ROADMAP.md` Future table: fsm-toolkit-as-designer-tooling noted,
  explicitly not decided or urgent.

## [0.1.0-patched251] — 2026-07-28

### Added — full docs/ spec audit

- Read the six spec docs not yet touched this session: FSM_DESIGN.md,
  FSM_GUARD_EXPRESSION_COMPARISON.md, FSM_TOOLKIT_FEATURE_REQUEST.md,
  I18N_SPEC.md, OLU_CLIENT_REMEDIATION.md, SHELF_MEDIA_SPEC.md.
- T-12 filed: FSM_DESIGN.md targets a client-side fsm-toolkit migration
  decided before xolu's native /fsm stabilised; internal/fsm currently
  depends on neither. Two independently-decided FSM paradigms is a real
  risk if not reconciled before T-03 or T-11 commit to an approach.
- T-13 filed: formengine doesn't resolve i18n keys yet (I18N_SPEC.md §4.1
  — title is a translation key by default, not literal text). Works today
  only because the 3 stage-2 schemas use literal strings. internal/i18n
  already has a working Translator; this is wiring, not new
  infrastructure. Missing from FORMENGINE_DEVELOPMENT_PLAN.md's 8 stages
  entirely — must land before FE-24 (stage 7's completion gate).
- T-04 updated: SHELF_MEDIA_SPEC.md confirmed to fully specify the
  external services it's blocked on; internal/media/media.go already
  implements the contract with a working stub.
- T-08 updated: cross-referenced docs/I18N_SPEC.md by name for the
  i18n-manager third of its scope.
- Housekeeping note (not yet acted on): OLU_CLIENT_REMEDIATION.md and
  RBAC-COMPLETION-PLAN.md are both status "Complete" but sitting
  unarchived in docs/ root — second and third instances of this pattern.

## [0.1.0-patched250] — 2026-07-28

### Added — internal/formengine, stage 2 (flat master forms)

- Three flat JSON Schemas authored (`internal/formengine/schemas/masters/
  {region,moneda,marca}.json`), constraints verified against the actual
  server-side validation in `internal/validation/schemas.go` field by
  field, not assumed from the static form's HTML alone — caught two
  missing `maxLength` constraints (Región's `descripcion`, Moneda's
  `simbolo`) this way, and pulled a `pattern` constraint I'd added back
  out once I confirmed it wasn't part of current enforced behaviour
  (parity, not silent enhancement).
- Embedded via `internal/formengine/schemas/masters` (`//go:embed`) —
  explicitly a stage-2 bootstrap per its own package doc comment, not the
  intended long-term schema source (that's xolu, once the Settings module
  T-08 can author and persist them).
- `renderForm` now emits the CSRF hidden input from `RenderContext.CSRFToken`
  — a gap caught while wiring real handlers: without it, wired forms
  couldn't actually submit. Buttons and the full-page/modal envelope stay
  caller-composed by design, matching how the static forms already
  separate form content from form chrome.
- `RegionFormPageDynamic` / `MonedaFormPageDynamic` / `MarcaFormPageDynamic`
  (`internal/ui/masters_pages_dynamic.go`) — signature-compatible with
  their static counterparts, not yet wired into the live routes (§13.3
  coexistence; the live routes' button/CSRF/envelope composition around
  the dynamic output isn't built yet either).
- `TestMastersFormParity_*` (`internal/ui/masters_pages_dynamic_test.go`):
  automated structural equivalence between static and dynamic rendering
  for all three forms — same fields, same order, same required/value/error
  state. Deliberately not a byte-identical HTML comparison, which would be
  a brittle, meaningless bar.
- `docs/formengine/TRACKING.md`: FE-06 through FE-08 closed.
- Full repo `go build`, `go vet`, `go test ./...` clean.

## [0.1.0-patched249] — 2026-07-28

### Added — internal/formengine, stage 1 (core engine, flat forms)

- `internal/formengine` package created: `engine.go` (`FormEngine`,
  `RenderContext`, `Render`), `schema.go` (property-order-preserving JSON
  Schema parsing, queryfy compilation), `renderer.go` (field orchestration,
  label/help/error wrapper), `widgets.go` (registry + 13 stage-1 MVP
  widgets), `sources.go` (`SourceResolver`, `StaticSourceResolver`).
- Deliberate deviation from FORM_ENGINE_SPEC.md §13.2's exact prose
  signature: `Render` takes `context.Context` as its first parameter —
  resolving `x-shelf-source` is a network call to xolu and needs to be
  cancellable/timeout-able.
- 21 unit tests, all passing. Includes an explicit property-order
  regression test (Go maps don't preserve JSON key order; §3.10's
  "render in schema order" needed a token-stream walk, not
  `json.Unmarshal` into a map).
- New dormant guard `TestSchemaRoundTrip_XShelfExtensionsSurvive`
  (`internal/formengine/engine_integration_test.go`, build tag
  `integration`) — run for real against a local xolu v0.16.16 built from
  source, confirming x-shelf-* extensions (including nested
  `x-shelf-form.tabs`) survive `POST`/`GET /api/v1/schema/{entity}`.
  Registered in `docs/KNOWN_ISSUES.md`'s new dormant-guards table.
- `docs/formengine/TRACKING.md`: FE-01 through FE-05 closed, cross-
  referenced in `docs/formengine/RESOLVED.md`.
- Full repo `go build`, `go vet`, `go test ./...` clean, including with
  `GOPROXY=off`.

## [0.1.0-patched248] — 2026-07-28

### Added — staged development plan for the form engine

- `docs/FORMENGINE_DEVELOPMENT_PLAN.md` — frozen 8-stage build plan for T-01
  (7 core stages + stage 8 deferred, gated on the Documents module).
- `docs/formengine/TRACKING.md` — dedicated stage tracker with its own
  repoman register (ID prefix `FE`, own `.repoman.json`), 25 tasks filed
  across the 8 stages, verified not to interfere with the main register.
- Caught and fixed same-session: the initial 7-stage breakdown missed
  ROADMAP.md's own step 8 (attachment tray) — added as FE-25, explicitly
  deferred rather than silently dropped.
- T-01 updated to point at the plan/tracker pair instead of duplicating
  detail; closes when FE-24 (stage 7's parity task) closes.
- `ROADMAP.md`'s form-engine table annotated as no longer the source of
  truth — the plan doc is.

## [0.1.0-patched247] — 2026-07-28

### Added — Order Engine (CMMS) integration analysis

- `docs/ORDER_ENGINE_INTEGRATION_NOTES.md` — technical response to the
  "AMS Orders" v1.0 draft. Maps Order Engine's Workflow/Numbering/Actions/
  Business Rules onto xolu `/fsm`, `/gen/seq`, event definitions, and the
  `internal/rules` pattern; identifies genuinely new pieces (Blocks, Scope,
  Cuadrilla, Empresa/Proveedor); full field-type gap analysis against
  FORM_ENGINE_SPEC.md v0.3 §14.
- T-10 filed: form-engine widgets/extensions the Order Engine needs
  (signature, image, GPS, multi-select, decimal precision, barcodes, unit
  of measure, input mask, custom validation).
- T-11 filed: scope the fifth module (Order Engine / CMMS) and its five
  open design questions.
- `ROADMAP.md` Future table updated to reflect the first draft spec.

## [0.1.0-patched246] — 2026-07-28

### Changed — module boundaries corrected; xolu v0.16.16 noted

- Register corrected: the module list is Assets Management, Documents,
  Settings, Administration (four modules, not three). T-02 narrowed to
  Administration (users/roles/permissions); T-08 filed for Settings
  (schema lifecycle, form designer, i18n manager) — both carved out of
  `ADMIN_MODULE_SPEC.md` v0.1's combined scope. T-03 retitled to Assets
  Management to match.
- `ROADMAP.md` Future table: noted a CMMS-like module as the next step
  in the asset lifecycle after the four current modules land.
- T-07 filed: evaluate migrating `internal/olu` onto the official xolu
  client (`github.com/ha1tch/xolu/pkg/client`), now that xolu is at
  v0.16.16 with stable `/ts`, `/fsm`, `/cal`, `/bal` (`/dxp` still in
  progress upstream). See `docs/KNOWN_ISSUES.md`.

## [0.1.0-patched245] — 2026-07-28

### Changed — toolchain floor raised; repository tooling brought up to standard

- `go.mod`: `go 1.22.2` → `go 1.25`. `go build ./...` and `go vet ./...`
  clean against Go 1.26.5.
- `repoman` (github.com/ha1tch/repoman) installed at `repoman/`, selftest
  9/9 green. `docs/TRACKING.md`, `docs/RESOLVED.md`, `docs/KNOWN_ISSUES.md`
  established (T-01..T-06 filed) ahead of the Seam rename and module
  decomposition work. T-06 (this go.mod bump) closed same-session; see
  `docs/RESOLVED.md`.

## [0.1.0-patched244] — 2026-05-28

### Changed — §15 decisions resolved, ROADMAP rewritten

- `docs/FORM_ENGINE_SPEC.md` v0.3 — All six §15 pending decisions resolved:
  - §15.1: Option B — parser rejects schema at load if section_defs has IDs
    not referenced in any row
  - §15.2: Merge-conservative by default (hidden fields preserve existing DB
    value); `x-shelf-hidden-reject: true` for sensitive fields that must
    reject submission when hidden
  - §15.3: Role priority model adopted (admin=100, shelf_consultant=90,
    manager=50, operator=30, viewer=10); PermService.Resolve() takes
    highest-priority role's rule; ties broken by most-restrictive
  - §15.4: `ModalTriggerAttrs()` pattern confirmed — handlers delegate
    modal type selection to the engine based on schema.width
  - §15.5: Five-point schema versioning policy adopted — additive changes
    compatible, required-field changes semi-compatible, destructive changes
    bump major version; hard prerequisite before first production schema
  - §15.6: Boolean for MVP confirmed; upgrade path to object documented
    with non-breaking parser that accepts both forms simultaneously

- `ROADMAP.md` — Complete rewrite:
  - Updated to patched243
  - Architecture design section updated with all five spec documents and
    their current versions
  - olu client parity section (patched232–236) added
  - Media stub (patched241) added
  - Near-term reorganised into five implementation tracks with dependency
    ordering: form engine, documents module, i18n, admin module, module
    architecture
  - External projects section: shelf-filer, thumble extensions,
    shelf-indexer, shelf-media integration — each with step-by-step scope
  - RBAC updated with role priority table
  - Technical debt updated (fijuggler i18n added)
  - Future section updated: storage lifecycle, LLM pipeline, shelf-media
    consolidation, fijuggler S3 provider, slabbis olu persistence



### Changed — SHELF_MEDIA_SPEC v0.3: file_record, WAL, filesystem hierarchy

- `docs/SHELF_MEDIA_SPEC.md` v0.3 — Major structural updates:

  **No BoltDB:** removed entirely. file_record in xolu is the single source of
  truth for all file metadata, processing state, and storage locations.

  **file_record entity (§3.7):** FileRecord struct with FijugglerRef (stable
  across migrations), ContentHash, MimeType, SizeBytes, PageCount, Duration,
  ThumbleAssetID, ThumbState/IndexState/SummaryState (independent processing
  dimensions), and Locations []StorageLocation (extensible for cold storage
  migration). StorageLocation has Provider, Path, Tier (hot/warm/cold), State
  (active/migrating/archived/evicted), and VerifiedAt.

  **WAL (§3.7):** SQLite write-ahead log for xolu outage resilience. Upload
  writes bytes synchronously, then creates file_record in xolu. If xolu is
  unavailable, WALEntry written locally and synced in background. WAL is a
  delivery mechanism, not a parallel store — xolu remains authoritative.

  **Upload flow rewritten (§3.5):** split into synchronous phase (bytes to
  storage + file_record to xolu, completes before responding to fijuggler) and
  asynchronous phase (two goroutines: thumble registration + eager rendering,
  and index_job enqueue). fijuggler response is immediate; all enrichment is
  background.

  **Filesystem hierarchy (§3.9):** four new subsections:
  - §3.9.1: tenant root structure — well-known dirs Inbox/ and Trash/ created
    on provisioning; everything else is free-form user structure
  - §3.9.2: delete moves to Trash/ preserving original path for restore; purge
    permanently deletes (only from Trash/); optional auto-purge after N days
  - §3.9.3: S3 directory simulation — list uses prefix+delimiter, mkdir is
    no-op, rename/delete copy+delete all objects with prefix; preserve_empty_dirs
    option via .keep placeholder objects
  - §3.9.4: fijuggler_ref format — always relative to tenant root, no leading
    slash, no tenant_id; deterministic translation to physical path; stable
    across storage backend migrations

  **e2e flow corrected (§1.2):** renumbered, WAL and file_record steps added,
  background phases clearly marked, document linking decoupled from indexing.

  **TOML config updated (§3.6):** BoltDB meta_db removed; [olu] section added
  with url, api_key, wal_path.

  **§11.3 updated:** BoltDB single-writer risk replaced with WAL consistency
  risk and idempotency key mitigation (content_hash as xolu dedup key).

  **Component table:** shelf-indexer added as fourth component.



### Changed — Tags, composite search, and shelf-indexer

- `docs/DOCUMENTS_MODULE_SPEC.md` v0.5:
  - Added `Tags []string` to Document entity (free-form, tenant autocomplete)
  - Added note that `full_text` lives in `file_index` keyed by `fijuggler_ref`,
    not in the document entity — clean separation of file properties vs
    document metadata
  - Tags field added to detail view right panel
  - Tags filter added to list view filters
  - Tags i18n key added
  - New §5.6 — composite search scoring: tag match (40) + title match (30) +
    description match (20) + FTS match (10); handler joins document entity with
    file_index entity via fijuggler_ref; tag autocomplete endpoint documented

- `docs/SHELF_MEDIA_SPEC.md` v0.2:
  - New §12 — shelf-indexer service:
    - Queue-first design: index_job entity in xolu (job_type: "extract" |
      "summarise"), enabling future LLM pipeline with no redesign
    - file_index entity in xolu keyed by fijuggler_ref (full_text for FTS5,
      word_count, extracted_at)
    - Indexing triggered by shelf-filer immediately on upload alongside thumble
      registration — no dependency on document linking whatsoever
    - Simple polling worker loop (5s interval, batch 10, max 3 attempts)
    - Extractors: pdftotext for PDF, soffice for Office, direct read for
      txt/md/csv, pandoc for HTML, skip for unsupported formats
    - No new binary dependencies beyond what thumble already requires
    - Future summarise job type documented (adds ai_summary to file_index,
      fifth search signal)
    - Deployment: docker-compose addition, health endpoint, START.cmd entry
  - Updated §1.2 end-to-end flow: shelf-filer now enqueues index_job (step 9,
    fire-and-forget) and search step added (step 15)



### Added — Shelf Media specification and stub

- `docs/SHELF_MEDIA_SPEC.md` v0.1 — Complete file handling subsystem spec covering:
  - Three-component architecture: fijuggler (UI), shelf-filer (Go file server),
    thumble (preview generation)
  - End-to-end flow: upload → shelf-filer storage → thumble registration →
    eager rendering → document record linking → tray pill display
  - fijuggler: vendoring instructions, three-state panel pattern, i18n scope
    (~30 strings, estimated half-day, deferred to post-MVP)
  - shelf-filer: ten-endpoint fijuggler REST contract, integration endpoints
    (GET /api/info, /api/health, /api/capabilities), upload flow with thumble
    auto-registration, eager rendering criteria, TOML config, BoltDB meta store,
    multi-tenancy via X-Shelf-Tenant header, S3 support
  - thumble extensions required: page_count + duration on asset registration,
    batch job endpoint (POST /assets/{id}/jobs/batch + GET /batch-jobs/{id}),
    new job types (audioclip: 30s mono OGG with fade, frame: video frame at
    timestamp)
  - Video frame sampling algorithm (deterministic, canonical Go implementation)
  - Document rendering table: all formats, renderers, page count probe method
  - shelf-media consolidation plan (future repo, unified config, docker-compose)
  - shelf-filer ↔ thumble preview integration (future: thumb_url in file listing)
  - Shelf App integration: proxy handler skeleton, /files route, media.Client
    usage in Document Manager module
  - media.Client interface (§9): IsStub, FilerURL, APIKey, FileMetadata,
    ThumbleAssetID, FileContentURL
  - Deployment: env/config, demo packaging health-check order, air-gap mode
  - Four known risks: file modification consistency, eager rendering queue
    saturation, BoltDB single-writer limit, shelf-filer authentication

- `internal/media/media.go` — media.Client stub for Shelf App:
  - Client interface with IsStub, FilerURL, APIKey, FileMetadata,
    ThumbleAssetID, FileContentURL
  - NewStub() returns ErrNotConfigured for all operations — default when
    shelf-media is not deployed; Document Manager detects and operates in
    degraded mode
  - NewHTTPClient(filerURL, apiKey) scaffolded with TODO markers for
    implementation against shelf-filer GET /api/info

- `docs/DOCUMENTS_MODULE_SPEC.md` v0.4 — Minor corrections:
  - Description field clarified as short plain-text summary (one line,
    appears in listings and pills)
  - Notes field marked as markdown-enabled (widget: x-shelf-input:"markdown",
    renderer sanitises output before display)
  - Detail view right panel extended with created_by/created_at and
    updated_by/updated_at display



### Changed — Form engine spec v0.2

- `docs/FORM_ENGINE_SPEC.md` — Applied all 14 pending updates:
  1. `EntityType string` added to `RenderContext` (§3c.2) — required for tray
     attachment queries (`entity_type + entity_id`)
  2. `attachable: boolean` added to section property table (§3.4)
  3. Lifecycle table (§3.7) extended with `tray` row — no form data, no submit,
     lazy htmx, requires EntityType+EntityID+SectionID, edit mode only, controlled
     by TrayPermission not HiddenSections/ReadOnlySections
  4. Rendering algorithm (§10.1) step `vi` added — for each section with
     `attachable: true` in edit mode, emit tray htmx placeholder with
     TrayPermissions check, correct URL and access level query param
  5. Stale `RenderContext` struct in §13.2 replaced with pointer to §3c.2
  6. `modal_width`/`max_height` replaced with `"width": "2xl"` in §3.1 and
     §12 examples (deprecated property remains documented in §3.9)
  7. Literal Spanish labels in §12 and §3.1 examples replaced with i18n keys
  8. Duplicate `rows` key in §3.8 example fixed — split into Ejemplo A
     (equal-width) and Ejemplo B (explicit widths)
  9. §2.1 intro clarified: `attachable` is not an `x-shelf-*` extension
  10. New §3c.8 — TrayPermission model: four access levels (hidden/read/preview/
      full), `FieldPermission` with `Type: "tray"`, permService resolution logic,
      server-side gate on shelf-filer proxy for Download level
  11. `TrayPermissions map[string]string` added to `RenderContext` (§3c.2)
  12. §15.6 added — pending decision on tray config object (max_pills,
      document_types, entity_types) vs simple boolean; boolean retained for MVP
  13. `entity:document-types` added to §5.2 source table
  14. §10.4 `asset-type-select` generalised to `entity-type-select` — documented
      as a general motor capability, not an asset-specific hardcode



### Changed — Document Management module spec v0.3

- `docs/DOCUMENTS_MODULE_SPEC.md` — Added complete document viewer specification:
  - New §5.5 Visor de documentos covering all four file type categories:
    - Text/office documents (PDF, DOCX, ODT, XLSX, ODS, PPTX, MD, TXT): all pages
      pre-rendered as medium-quality JPEG by thumble on file link; page strip
      navigation with thumbnails, page jump, zoom controls
    - Images: full-size preview via thumble preview endpoint, gallery for
      multiple images; original file only served for Download permission
    - Audio: waveform image + 30-second mono OGG clip (64kbps, 0.5s fade-in,
      2s fade-out); minimal inline player over waveform
    - Video: adaptive frame sampling algorithm (target 20–30 frames; 30s interval
      for < 10min up to 5min interval for > 150min, 30 frame cap); horizontal
      frame strip with timestamps, keyboard navigation
    - Unsupported formats: MIME icon + metadata + download button
    - Tray pill click opens viewer in medium modal (width: "xl") without
      metadata panels; "Ver detalle completo" link to full detail view
  - Extended `document` entity model with viewer support fields: `page_count`,
    `video_frames` ([]VideoFrame with index/timestamp/thumb_url), `audio_duration`,
    `has_audio_clip`, `preview_type`
  - New `VideoFrame` struct
  - Updated §3 file linking flow: eager rendering steps expanded per file type —
    per-page JPEG jobs for documents, waveform+clip jobs for audio, N frame jobs
    for video (algorithm-driven), thumb+preview jobs for images
  - Step 6 updated to persist all new viewer fields to xolu on file link
  - Tray pill click action updated to reference §5.5 viewer
  - New API routes: `/api/v1/documents/{id}/viewer`, `/page/{n}`, `/frame/{n}`



### Changed — Architecture correction

- `docs/DOCUMENTS_MODULE_SPEC.md` v0.2 — Decoupled Document Management module
  from the `assets` entity. The module is now entity-agnostic and can manage
  documents attached to any entity in the system (assets, sensors, custodians,
  regions, or any future entity type). Changes:
  - `attachment` entity: replaced `asset_id int64` with `entity_type string`
    + `entity_id int64` (polymorphic reference)
  - Module dependency diagram: `documents` no longer depends on `assets`;
    it sits directly below `masters` and can be compiled with any combination
    of operational modules
  - OQL attachment queries now filter by `entity_type` + `entity_id`
  - All UI routes: `/documents/asset/{id}` → `/documents/{entity_type}/{entity_id}`
  - All API routes: `/api/v1/assets/{id}/attachments/*` →
    `/api/v1/attachments/{entity_type}/{entity_id}/*`
  - Tray htmx endpoint: `/api/v1/documents/tray/{asset_id}/{section_id}` →
    `/api/v1/documents/tray/{entity_type}/{entity_id}/{section_id}`
  - xolu graph Sulpher examples extended to show queries across asset, sensor,
    and custodio entity types
  - Detail view panel groups attached entities by type
  - i18n section key `"assets"` renamed to `"entities"`
  - Risk §10.4 updated to reference entities generically



### Added — Architecture Design

- `docs/DOCUMENTS_MODULE_SPEC.md` — Document Management module specification. Covers:
  - Module purpose: thin integration layer with business metadata in xolu,
    no file storage, no thumbnail generation
  - Clean separation of responsibilities: shelf-filer (storage), fijuggler
    (file UI), thumble (previews), documents module (metadata + linking)
  - Three data models: `document_type` (master table in masters module),
    `document` (metadata + fijuggler_ref + thumble_asset_id), `attachment`
    (document → asset + optional section_id)
  - Document status model: present / pending / expired (calculated)
  - xolu graph edges for Sulpher queries across documents and assets
  - File linking flow: shelf-filer metadata → thumble registration → job
    queue → thumb_url stored back on document entity in xolu
  - Three-state attachment tray in forms: empty / pills (up to 8) / modal
  - `attachable: true` property on form sections, loaded lazy via htmx
  - Document manager UI: list with filters, detail with preview panel and
    asset list, creation form with file picker
  - `/files` route mounting fijuggler with proxy to shelf-filer
  - Full route table: UI routes, API routes, file proxy
  - i18n keys: list, tray, document form, document type form
  - Module structure under `modules/documents/`
  - Degraded mode matrix: 4 combinations of shelf-filer/thumble availability
  - Four known risks: shelf-filer/xolu consistency on file deletion,
    stale thumb_url on thumble cache eviction, tray load on complex forms,
    document attached to many assets



### Changed — olu client test coverage

- `internal/olu/client_test.go` — Rewrote and substantially expanded mock-based
  test suite. Coverage: 82.9% → 97.4% (practical ceiling without network failure
  injection). Changes:
  - Fixed `TestErrorHandling` and `TestCommit` version conflict test — both used
    the old flat `{"error": "string"}` format; updated to xolu's nested
    `{"error": {"code": ..., "message": ..., "status": ...}}` envelope.
  - Added `xoluError()` and `xoluConflict()` test helpers emitting correct
    xolu error shapes.
  - Added tests for all 30 new methods added in patched233–235: OQL async,
    Sulpher async, all graph topology endpoints, schema create/get, export,
    and all 13 timeseries methods.
  - Added `TestErrorPaths` covering the `do()` error return branch of every
    single client method (35 subtests).
  - Added `TestSulpher` — the deprecated alias was at 0% coverage.
  - Added `TestCurrentVersionFloat64`, `TestCurrentVersionInt`,
    `TestCurrentVersionMissingKey`, `TestCurrentVersionUnknownType` covering
    all branches of the `CurrentVersion()` type switch.
  - Added `TestDoUnmarshalFailure` covering the response unmarshal error path.
  - Added `TestErrorDetailsPassthrough` covering the `errResp.Details` branch
    in `do()`.
  - Added `TestListOffsetNoLimit`, `TestListTimestampParsing`,
    `TestSearchWithPagination`, `TestSearchTimestampParsing`,
    `TestGraphNodeSearchWithLimit`, `TestGraphNodeSearchAllNodes`,
    `TestExportWithAuth`, `TestTSAppendEventWithPayload`,
    `TestTSQueryRangeWithOptions`, `TestTSLatestWithBounds` covering
    previously untested parameter combinations.
  - Remaining uncovered lines (2.6%): network-level failure paths in `do()`
    (`http.NewRequestWithContext`, `httpClient.Do`, `io.ReadAll`) that require
    transport-level injection and are not worth the infrastructure cost.

- `internal/olu/client_integration_test.go` — New file (build tag:
  `//go:build integration`). Tests require a running xolu instance.
  - Entity CRUD round-trip (create/get/update/patch/delete)
  - `Save` upsert semantics (created vs replaced)
  - `List` with pagination
  - `Commit` with optimistic concurrency — version increment verified,
    audit record written and retrieved, stale-version conflict verified
    with `current_version` extraction
  - OQL sync and async (with poll loop and result retrieval)
  - Schema create/get with validation enforcement (valid write passes,
    missing required field rejected)
  - Export — zip magic bytes verified
  - Graph topology suite (skipped if graph not enabled): neighbors,
    shortest path, path exists, node info, node degree, incoming/outgoing
    edges, node search, Sulpher query, common neighbors
  - Timeseries full flow (skipped if TS not enabled): provision, define/
    list/get/update timeline, single append, batch append (5 events),
    range query (6 events verified), latest N, scalar aggregate (avg),
    bucketed aggregate (max by 2s), retention policy, store stats,
    timeline stats with oldest/newest event timestamps
  - Two separate server URL env vars: `OLU_TEST_URL` (entity/graph/OQL)
    and `OLU_TS_TEST_URL` (timeseries, strict tenant mode)
  - Pebble files written to `OLU_TS_DATA_DIR` on the xolu server — not
    controlled by the test but documented in the file header



### Added — xolu timeseries client support

- `internal/olu/client.go` — Added complete Pebble timeseries API coverage:

  **Provisioning:** `TSProvision` (enable timeseries for tenant; idempotent).

  **Timeline management:** `TSDefineTimeline` (define or update; dims immutable
  after first write), `TSListTimelines`, `TSGetTimeline`, `TSUpdateTimeline`
  (name and retention_days only).

  **Write:** `TSAppendEvent` (single event), `TSBatchAppend` (atomic batch up
  to OLU_TS_MAX_BATCH_SIZE events).

  **Query:** `TSQueryRange` (time-range query via POST /ts/query/range),
  `TSLatest` (N most recent events with optional time bounds).

  **Aggregate:** `TSAggregate` — scalar (sum/avg/min/max/count over a range)
  or bucketed (time-bucketed results via Interval). Backed by source-confirmed
  request/response shapes from ts_handlers.go.

  **Diagnostics:** `TSGetRetention` (per-timeline retention policy),
  `TSStats` (timeline count + disk bytes), `TSTimelineStats` (event counts,
  oldest/newest timestamps, approximate flag).

  New types: `TSTimeline`, `TSEvent`, `TSRangeResult`, `TSBucket`,
  `TSAggregateResult`, `TSRetentionPolicy`, `TSStats`, `TSTimelineStats`,
  `TSAppendRequest`, `TSBatchResult`, `TSQueryRangeParams`, `TSLatestParams`,
  `TSAggregateParams`.

  The xolu client now covers the complete timeseries API surface.



### Added — xolu client feature parity

- `internal/olu/client.go` — Added all previously unsupported xolu endpoints:

  **OQL async:** `OQLAsync` (submit), `OQLJobStatus` (poll), `OQLJobResult`
  (retrieve). New types: `AsyncJobStatus`, `AsyncOQLResult`.

  **Sulpher async:** `GraphQueryAsync` (submit), `GraphQueryJobStatus` (poll),
  `GraphQueryJobResult` (retrieve). New type: `AsyncSulpherResult`.

  **Graph — new endpoints:** `GraphStats` (node/edge counts, cycle flag),
  `GraphPathExists` (boolean path check with length), `GraphCommonNeighbors`
  (shared outgoing neighbours of two nodes), `GraphNodeSearch` (list nodes
  by entity type), `GraphNodeInfo` (full node with all edges and degree),
  `GraphNodeDegree` (in/out/total degree counts), `GraphIncomingEdges`
  (all incoming edges as typed structs), `GraphOutgoingEdges` (all outgoing
  edges as typed structs). New types: `GraphStatsResult`, `GraphNodeInfoResult`,
  `GraphDegreeResult`, `GraphEdge`, `GraphEdgesResult`.

  **Schema:** `SchemaCreate` (register/update JSON Schema for entity validation),
  `SchemaGet` (retrieve registered schema).

  **Export:** `Export` (download full data snapshot as raw zip bytes).

  The xolu client now covers the complete API surface of xolu v0.9.7.



### Changed — xolu client compliance (non-breaking)

- `internal/olu/client.go` — Brought olu client into full compliance with
  xolu v0.9.7-patched78+ API:
  - Added `CommitTSEvent` struct matching xolu's `storage.CommitTSEvent`
    (Timeline, Dims, Time, Nums, Payload) with full documentation of the
    Pebble write-order atomicity protocol and OLU-CM015/CM016 failure modes.
  - Added `Timeseries []CommitTSEvent` field to `CommitRequest` with
    documentation of the at-least-one-of rule, the 5000-entry limit, and
    the SQLite-only constraint.
  - Added `TSAccepted int` field to `CommitResult`.
  - Updated package comment to reference xolu (not olu), target version
    v0.9.7-patched78+, and document the SQLite requirement for /commit.



### Fixed — xolu API compatibility

- `internal/handlers/api/assets.go` — Removed `GetRelated` handler. The
  endpoint was registered but never called by the UI, and its Sulpher query
  syntax (`FROM asset:%d TRAVERSE * DEPTH %d`) is invalid against xolu's
  Sulpher parser which requires `MATCH ... RETURN` syntax. Dead code with
  an active bug.

- `internal/handlers/api/router.go` — Removed `GET /api/v1/assets/{id}/related`
  route registration.

- `internal/ui/asset_detail.go` — Removed `RelatedAsset` struct,
  `RelatedAssets []RelatedAsset` field from `AssetDetailData`, the "Related
  Assets" card from `AssetDetailPage`, and the `assetRelatedContent` renderer.
  None of these were populated by any handler.

- `internal/olu/client.go` — Fixed error response parsing in `do()`. xolu
  returns `{"error": {"code": "OLU-ST001", "message": "...", "status": N}}`
  (nested object); the previous parser expected `{"error": "string"}` and
  silently discarded all structured error information. The parser now correctly
  extracts `Code`, `Message`, and `current_version` from conflict responses.
  Added `Code string` field to `Error` struct, updated `Error()` to include
  the code, and added `IsConflict()` and `CurrentVersion()` helper methods
  for callers handling 409 version conflicts.



### Added — Architecture Design

- `docs/ADMIN_MODULE_SPEC.md` — Complete specification for the Admin module. Covers:
  - Two user profiles: DIY operator and Shelf AMS consultant (`shelf_consultant` role),
    same tool with mode-switched capabilities
  - Module extraction map: routes leaving the main web module (`/settings/*`,
    `/admin/*`, `/fsm/*`, `/asset-types/*`, `/propiedades/*`) and their destinations
  - Module dependency order: core → masters → admin (independent of assets/monitoring)
  - Section 1 — User Management: extended User model (locale, expiry, notes),
    additional roles (`shelf_consultant`), session invalidation, user detail with
    audit trail, role priority ordering for multi-role conflict resolution
  - Section 2 — System Configuration: general (tenant, locale, timezone), module
    activation per role, security settings, notifications (email/Slack/webhook),
    feature toggles declared by each module, appearance preferences
  - Section 3 — Permission Manager: tree view of form elements with per-role access
    badges, direct vs inherited rule distinction, role view (inverse), diff view
    between two roles, inheritance editing with "modify parent?" prompt, consultant
    restrictions (cannot elevate own permissions or modify admin role)
  - Section 4 — i18n Manager: locale completeness view with schema-derived key
    coverage, two-column inline section editor with pending key highlighting, tenant
    vocabulary editor (`tenant.terms.*`), import/export with diff preview and partial
    import support
  - Section 5 — Schema Lifecycle: draft/published/archived state machine, SchemaVersion
    model, three compatibility levels (additive/semi/breaking) with affected record
    counts, JSON editor with real-time queryfy validation, visual diff against published
    schema, version history with restore
  - Section 6 — Form Designer: three-panel layout (block tree / configurator /
    live preview), per-block-type configurator with all x-shelf-* properties,
    visual condition builder for visible_when (no JSON editing for DIY),
    i18n key selector with inline creation, guided mode (admin) vs advanced mode
    (shelf_consultant) with bidirectional JSON↔tree sync, htmx preview endpoint
    (< 150ms target), locale and role selectors in preview panel, missing translation
    indicators, i18n gap resolution before publish
  - Section 9 — Admin navigation: dedicated sidebar layout separate from operational
    modules, "back to operational module" link
  - Section 10 — Audit log categories for all admin actions
  - Section 11 — Four known risks: bidirectional tree↔JSON sync, preview latency,
    concurrent draft editing (optimistic lock), broken schema in production
    (dry-run before publish)



### Added — Risk Documentation

- `docs/FORM_ENGINE_SPEC.md` §15 — Known Risks and Pending Decisions:
  - §15.1 `rows`/`section_defs` redundancy: silent field loss if a section is defined
    but not referenced in any row. Three resolution options documented (warn / reject /
    collapse), option B (schema rejection at load time) recommended.
  - §15.2 Hidden fields and POST submission: `display:none` fields are included in POST.
    Policy for "ignored" undefined — silent discard vs. active rejection. Flag
    `x-shelf-hidden-reject` proposed for fields requiring active rejection.
  - §15.3 Multi-role permission conflict resolution: most-restrictive-wins may be
    counter-intuitive. Three options documented (most restrictive / most permissive /
    explicit role priority). Option C (priority-ordered roles) recommended.
  - §15.4 `width: "fullscreen"` modal selection: non-obvious side effect of activating
    ShelfModal instead of ShelfModal2. `ModalTriggerAttrs()` helper proposed to
    encapsulate modal selection in the engine rather than each handler.
  - §15.5 Schema lifecycle and data/schema drift: no policy defined for incompatible
    schema changes. Five-point policy proposed covering additive, obligatory, and
    destructive changes with schema versioning and read-only migration mode.

- `docs/I18N_SPEC.md` §10 — Known Risks and Pending Decisions:
  - §10.1 i18n key / schema drift: dynamic schema changes produce untranslated labels
    in production without any visible signal. Admin completeness view must include
    schema-derived keys from olu alongside static code keys.
  - §10.2 Tenant locale namespace conflicts: unrestricted tenant overrides of `core.*`
    keys risk semantic drift and stale translations after system updates. `tenant.terms.*`
    namespace proposed as the correct scope for tenant vocabulary personalisation.



### Added — Architecture Design

- `docs/FORM_ENGINE_SPEC.md` — Complete specification for the dynamic form engine
  (shared core component). Covers:
  - Six `x-shelf-*` JSON Schema extensions: `x-shelf-form`, `x-shelf-input`,
    `x-shelf-source`, `x-shelf-help`, `x-shelf-fk-popover`, `x-shelf-relation`
  - Row-based layout model within tabs (`rows` + `section_defs`) with proportional
    widths mapped to Tailwind grid-cols-12
  - Form-level width control (`sm` → `fullscreen`)
  - Embedded read-only detail panels (`detail`) and action sub-forms (`embed`)
    with htmx loading, modal and inline modes, and post-action detail refresh
  - Conditional visibility (`x-shelf-visible-when`, `visible_when`) with full
    operator set, composable `all_of`/`any_of` conditions, dual client/server
    evaluation, and seven documented edge cases
  - Three-mechanism access control model: conditional visibility (schema),
    lifecycle read-only (handler/FSM), and role-based field permissions
    (`FieldPermission` in olu)
  - Extended `RenderContext` with `HiddenFields`, `HiddenTabs`, `HiddenSections`,
    `ReadOnlyFields`, `ReadOnlyTabs`, `ReadOnlySections`
  - Server-side HTML omission for hidden elements (not CSS display:none)
  - Twelve documented permission edge cases including: hidden tab activation
    fallback, implicit tab hiding when all fields are hidden, required field
    exemption for hidden fields, readonly field bypass on validation, embed
    suppression in readonly tabs, `visible_when` over permission-hidden fields
  - Permissive-by-default and deny-by-default tenant models
  - Full rendering algorithm covering pre-render permission resolution, row
    traversal, visible_when evaluation at each level, and readonly submit button
  - Two complete schema examples: Custodio (flat master form) and Asset
    (four-tab form with all widget types)
  - Widget table: 18 widget types, 16 implemented in MVP, 2 reserved for future

- `docs/I18N_SPEC.md` — Complete specification for the internationalisation system.
  Covers:
  - `LocalePackage` structure with `meta` + `modules` hierarchy
  - Full `LocaleMeta` schema: identity, direction/RTL, typography, layout,
    regional formats, SVG icon sets with per-key overrides, input/keyboard hints,
    CLDR pluralisation rules
  - Three-level namespace: `{module}.{section}.{key}`
  - `forms` subsection grouping all forms of a module by name
  - Form key taxonomy: `title`, `tab`, `section`, `field`, `field.help`,
    `field.placeholder`, `errors`, `actions`, `detail.{name}.*`, `embed.{name}.*`
  - Distinction between `detail` (read-only panels) and `embed` (action sub-forms)
    with full key coverage for columns, empty states, actions, and confirmations
  - Standard module inventory: `core`, `assets`, `assets.masters`, `admin`,
    `monitoring`, `reports`, `field` (global field namespace)
  - Complete form inventories for `assets`, `assets.masters`, and `admin` modules
  - i18n/form-engine integration: `title` as translation key, resolution priority
    chain, field label and help text resolution, queryfy error mapping table
  - `LocaleHelper` Go interface with `T()`, `TP()`, `Dir()`, `RTL()`, `Icon()`,
    `DateFmt()`, `NumberFmt()`
  - Hot-reload via `Translator.Reload()` without server restart
  - Dual storage: embedded `embed.FS` (base locales) + olu (tenant overrides)
  - Tenant-level locale personalisation (`{locale}-{tenantID}` packages)
  - Permissive resolution chain: tenant override → locale → fallback → key literal
  - Base locales: `en` (fallback), `es` (Rioplatense), `pt` (Brazilian)
  - Post-MVP locale table: 11 languages including Arabic (RTL), Hebrew (RTL),
    CJK scripts, Devanagari, Cyrillic
  - Admin UI spec: completeness tracking, two-column inline editor, role-aware
    permission audit view, import/export for offline translation

## [0.1.0-patched228] - 2026-04-16

### Fixed

- **START.cmd**: replaced `start /b` (background, no window, silent crash) with
  `start "Shelf DB"` and `start "Shelf AMS"` — each server now opens its own
  visible CMD window. If a binary crashes on launch, the user can see the error
  message instead of facing a frozen black screen.

- **START.cmd**: added 30-second retry limit to both `wait_olu` and
  `wait_shelf` health-check loops. Previously the script would spin forever if
  a server failed to start. Now it prints a clear error ("Check the Shelf DB
  window for error messages"), pauses for the user to read it, and then cleans up.

- **START.cmd**: changed `OLU_LOG_LEVEL` from `error` to `info` during startup
  so that the server windows show meaningful startup progress rather than
  silence.

- **START.cmd**: added `:stop_all` label so the cleanup block is shared
  between normal exit (press any key) and the timeout-error path.

## [0.1.0-patched227] - 2026-04-02

### Added

- **build-demo.sh**: after packaging, creates a distributable zip at
  `demo/zip/{demo-name}.zip`. Send the zip to the recipient, who unzips and
  double-clicks START.cmd.

## [0.1.0-patched226] - 2026-04-02

### Fixed

- **OLU_DB_PATH missing everywhere**: xolu's `DBPath` defaults to the bare
  filename `"olu.db"` resolved against the current working directory, NOT
  against `OLU_BASE_DIR`. This caused olu.db to land in the repo root (or
  in `/` on some systems) instead of `data/olu/`. Added explicit
  `OLU_DB_PATH` to all four places that start olu:
  - `scripts/build-demo.sh` (pre-seeding instance)
  - `scripts/build-demo.sh` (START.cmd and START.sh launcher templates)
  - `scripts/launch.sh` (development launcher)
  - `Makefile` (start-olu target)

  This is a known xolu quirk — `DBPath` should default to
  `filepath.Join(BaseDir, "olu.db")` but currently doesn't.

### Changed

- **build-demo.sh**: reverted CWD subshell workaround from patched225 in
  favour of the proper `OLU_DB_PATH` fix.

## [0.1.0-patched225] - 2026-04-02

### Fixed

- **build-demo.sh**: native olu binary for pre-seeding is now built
  separately WITHOUT `CGO_ENABLED=0`. The cross-compiled target binaries
  still use `CGO_ENABLED=0` (required for cross-compilation), but this
  likely caused xolu's SQLite storage to fall back to in-memory mode,
  producing an empty database directory. The native seeding binary is
  stored at `demo/build/native/olu` and is never packaged.

- **build-demo.sh**: broadened database file detection from `*.db` only
  to any file type, since olu may use different extensions. Lists found
  files after capture for diagnostic visibility.

## [0.1.0-patched224] - 2026-04-02

### Fixed

- **build-demo.sh**: added graceful shutdown with 1-second WAL checkpoint
  pause before copying database files. Verify `.db` files exist both after
  seeding and after copy into the demo package — fail with clear diagnostic
  if pre-seeding produced no database. Use `cp -a` (archive) to preserve
  `.db-wal` and `.db-shm` companion files.

## [0.1.0-patched223] - 2026-04-02

### Fixed

- **build-demo.sh**: native olu binary for pre-seeding now detected from
  the build machine's actual OS/architecture (`uname -s`/`uname -m`)
  instead of hardcoded `linux-amd64`. Fixes "cannot execute binary file"
  when building demos on macOS. If the build machine's platform is not
  one of the three standard cross-compilation targets, it is compiled
  automatically.

- **build-demo.sh**: ANSI colour variable definitions changed from
  single-quoted `'\033[...'` to `$'\033[...'` (ANSI-C quoting). Fixes
  raw escape codes appearing in the platform selection menu on macOS.

## [0.1.0-patched222] - 2026-04-02

### Added

- **`make demo` target** and `scripts/build-demo.sh`: self-contained demo
  builder for external reviewers. Cross-compiles olu + shelf for linux-amd64, darwin-arm64,
  and windows-amd64 (cached in `demo/build/`). Interactive seed and platform
  selection. Pre-seeds the database at build time — the target machine needs
  zero dependencies. Generates `START.cmd` (Windows) or `START.sh` (Linux/Mac)
  launchers that start both services, display login credentials, and copy the
  URL to clipboard. Output: `demo/shelf-demo-{platform}-seed{NN}-{seq}/`.

### Changed

- Makefile: added `demo` to `.PHONY`, top-level comment, help text.
  `make clean` now clears `demo/build/` cache but preserves packaged demos.

## [0.1.0-patched221] - 2026-04-02

### Added

- **seeds/SEEDS-CATALOGO.md**: Spanish-language catalogue documenting all
  four seed datasets (449 lines, 8 sections). Covers company profiles,
  asset types, brands, regulatory frameworks, edge cases, custody models,
  comparative matrix, and execution instructions.

- **seeds/SEEDS-CATALOGO.pdf**: Rendered PDF version (14 pages, A4).

- **seeds/md2pdf.py**: Markdown-to-PDF conversion script using markdown +
  weasyprint. Generates SEEDS-CATALOGO.pdf from the markdown source.

## [0.1.0-patched220] - 2026-04-02

### Added

- **Seed 04 — VendiBeria S.L.** (Spanish vending machine operator vertical):
  94 assets (32 coffee, 17 snacks, 16 cold drinks, 9 combo, 5 fresh food,
  12 payment terminals, 3 micromarkets) across 11 client sites in Madrid
  and Castilla-La Mancha. 7 asset types, 8 brands (Azkoyen, Jofemar, Necta,
  Bianchi, SandenVendo, Saeco, Nayax, Caslab), 2 FSM machines, 6 rules,
  8 sensors, 12 manufacture years (2014–2025). Companion document:
  `seeds/SEED-04-COMPANION.md` (399 lines, 11 sections).

- **Seed 03 — Extinfire SRL** (Uruguayan fire extinguisher maintenance vertical):
  113 assets across 8 client sites, 7 asset types, 7 brands (Fadesa, Drago,
  Melisam, Amerex, Kidde, Yamato, Grauser), 2 FSM machines, 7 rules.
  Regulatory framework: Decreto 372/023, UNIT 598/586/607/1221, IT-04 DNB.

### Changed

- **Seed company names** — resolved collisions with real businesses:
  - Seed 01: INTEC S.A. → **Constructec S.A.** (real INTEC S.A. exists in
    Canelones, Uruguay — plastics mould manufacturer since 1969)
  - Seed 03: ExtinFuego S.A. → **Extinfire SRL** (real Extinfuego SRL exists
    at Blandengues 1477, Montevideo — same city and industry sector)
  - Seed 02 (FrigoCold S.A.) and Seed 04 (VendiBeria S.L.) verified clean.

- Seed 03 filename: `seed-03-extinfuego.py` → `seed-03-extinfire.py`
- Seed 01 filename: `seed-01-intec.py` → `seed-01-constructec.py`

## [0.1.0-patched219] - 2026-04-01

### Fixed

**Issue 1 — Single-frame flash in sidebar and top bar during page transitions**
Root cause: the `:root` element implicitly participated in View Transitions,
creating `::view-transition-old(root)` and `::view-transition-new(root)`
pseudo-elements that screenshotted the sidebar and top bar on every navigation.
Two mechanisms produced the flash:
(a) The root screenshots were captured before `htmx:afterSwap` updated the
    active nav highlight, so the nav state in the pseudo-element differed from
    the live DOM underneath. When pseudo-elements were removed at the end of
    the shelf-main animation (500ms), the highlight jumped in one frame.
(b) Compositor frame gap: removing the root pseudo-element layer and revealing
    the live DOM layer required one compositor frame, during which the sidebar
    and top bar were briefly unpainted.

Fix: `view-transition-name: none` on `:root` opts the entire page chrome out
of View Transitions. The sidebar, top bar, and mobile header are never
screenshotted, never composited as pseudo-elements, and never removed/replaced.
They remain as live DOM throughout every transition.

Removed: `::view-transition-old(root)` and `::view-transition-new(root)` CSS
rules (dead code — those pseudo-elements no longer exist). Root references
removed from `prefers-reduced-motion` block. Comment block updated.

**Issue 2 — `make test-workflow` failed on macOS (BSD sed)**
Two `sed -i` calls — one in the Makefile (go.mod patching for XOLU_PATH),
one in syncver.sh (README version update) — used GNU sed syntax. BSD sed
on macOS requires an explicit extension argument, causing both to fail.
Replaced with Python per project conventions (never use sed).

**Issue 3 — `make test-workflow` relative XOLU_PATH not resolved**
`XOLU_PATH=./xolu` was written verbatim into the workflow go.mod replace
directive, but `go test` runs from `internal/ui/workflow/` — three levels
deep. Go resolved `./xolu` relative to the test directory, not the project
root. Fixed: the Makefile now resolves XOLU_PATH to an absolute path via
`cd "$path" && pwd` before patching go.mod.

**Issue 4 — Workflow test: TestWorkflow_InactiveCustodioExcludedFromDropdown**
`AssetNew` handler only renders the form as an HTMX partial — non-HTMX
requests redirect to `/assets?create=1`. The test was making a plain GET
without the `HX-Request: true` header, receiving a 303 instead of the form
HTML. Fixed: added `req.Header.Set("HX-Request", "true")`.

## [0.1.0-patched218] - 2026-04-01

### Fixed

**Issue 1 — Row hover highlighting restored**
ListRow: both even and odd row classes now include
  hover:bg-indigo-50 dark:hover:bg-indigo-900/20 transition-colors

**Issue 2 — First content column clickable in all master listings**
New ListCellLink(text, href string) mi.H function in listing.go renders
a <td> with an <a> tag styled as indigo underline-on-hover.
All 10 master listings updated: Regiones, Monedas, CentrosCosto, Marcas,
Custodios, Alertas, PropDefs, Modelos, Ubicaciones, PropAssocs.
The first meaningful (non-ID) cell in each row links to the detail/edit page.

**Issue 3 — Pagination**
Assets have pagination via ListPaginationBar. Masters pass nil — they show
all records with a count in the subtitle. No pagination was lost.

**Issue 4 — Dialog centering fixed (root cause identified and resolved)**
Root cause: anchorEl was passed as a 4th positional argument to
  ShelfModal2.open(title, url, opts)
which only accepts 3. The 4th argument was silently ignored, so anchorEl
never reached _build(), and every dialog centred on the full viewport.

Fix: anchorEl is now embedded INSIDE the opts object:
  ShelfModal2.open('title', '/url', {maxWidth:'600px', anchorEl: el})
Both create and edit onclick strings updated in listing.go.
ShelfModal2._build(anchorEl) now also sets min-height:'360px' on the anchor
container and min-height:'100%' on the backdrop so the modal is always
fully visible even when the listing is short.
configurable: ModalWidth on listing = anchored; no ModalWidth = viewport-centered.

**Issue 5/6 — Dialog form buttons moved to footer**
Added htmx:afterSwap listener in app.js. After HTMX populates
#shelf-modal2-body, any [data-modal-footer] element inside the body is
detected; its children are moved to #shelf-modal2-footer and the source
element is removed. masterPartialFormWrapper already renders
<div data-modal-footer> with Cancelar/Guardar. The buttons now appear in
the dedicated footer area rather than inside the scrollable form body.

**Issue 7/8/9 — Asset delete button: canonical style + ShelfConfirm + entity name**
conditionalDeleteButton() in asset_detail.go:
- Now accepts assetName string parameter
- Style updated to match canonical: inline-flex items-center gap-1 px-4 py-2
  border border-red-300 text-red-600 rounded-lg hover:bg-red-50
  dark:hover:bg-red-900/20 text-sm font-medium
- onclick uses ShelfConfirm.showDanger with dangerName so the message reads
  "Are you sure you want to delete the entry for <b>Camera XYZ</b>?"
- Both callers updated to pass data.Asset.Name

**Issue 10 — Custodians listing shows document type AND number**
TipoSujeto column replaced with combined ListCellMuted(c.TipoSujeto+": "+c.IdentificadorFiscal)
e.g. "RUT: 21-234567-8" instead of just "RUT"

## [0.1.0-patched217] - 2026-04-01

### Fixed

**Issue 1 — Remaining JS confirm() calls replaced with ShelfConfirm**
- asset_detail.go: inline confirm('Delete this asset?') → ShelfConfirm.showDanger()
- asset_detail.go: mi.HxConfirm("Are you sure...") removed (ShelfConfirm handles it)
- pages.go: asset-type delete form onsubmit confirm() → ShelfConfirm.showDanger()

**Issue 2 — Sensors: delete button on detail page**
- SensorDetailData: CSRFToken field added
- SensorDetailPage: Delete Sensor button added to header actions area, wired to
  POST /sensors/{id}/delete via ShelfConfirm.showForm with dangerName
- SensorDelete handler added (routes.go), route registered (router.go)
- SensorDetail handler now populates CSRFToken

**Issue 3 — Alerts listing restored**
- AlertsListData: CSRFToken field added
- AlertsListPage: Alert cell now shows Title + severity badge on first line,
  Message text on second line (was missing entirely)
- Severity column removed — severity badge now embedded in Alert cell
- Acknowledge button: POST form to /alerts/{id}/acknowledge with CSRF, hidden
  when alert is already acknowledged
- Resolve button: POST form to /alerts/{id}/resolve with CSRF
- Resolved alerts show "Resolved" text instead of action buttons
- AlertResolve handler added (routes.go), route POST /alerts/{id}/resolve
  registered (router.go)
- AlertsList handler now populates CSRFToken before render

**Issue 4 — Dialogs anchored to listing width**
- ListPageConfig: data-list-modal-anchor attribute added to outer div when
  ModalWidth is set
- listing.go ListPage create button onclick now passes
  anchorEl: this.closest('[data-list-modal-anchor]') to ShelfModal2.open()
- listing.go ListActions edit button onclick now passes same anchorEl
- ShelfModal2._build() updated to accept anchorEl parameter:
  - When anchorEl is provided (Element): backdrop is position:absolute within
    that element, z-index 100, flexStart with 5% paddingTop
  - When absent: existing fixed/viewport behaviour unchanged
- ShelfModal2.open() passes anchorEl from opts to _build()
- All 9 master listings: EditModalWidth added to ListActionsConfig, matching
  the ModalWidth set on the listing (600–700px depending on master)

**Issue 5 — ShelfConfirm redesign**
- Cancel is now the default (filled indigo, auto-focused on open)
- Delete/OK button starts as border-only in red/indigo, fills on hover/focus
- Message supports dangerName option: "Are you sure you want to delete the
  entry for <strong>Entity Name</strong>?"
- ListActionsConfig: DeleteName string field added
- listing.go: delete onclick conditionally passes {dangerName:'...'} when
  DeleteName is set
- All 9 master listings: DeleteName wired to the row's display field

**Issue 6 — User avatar dropdown**
- TopHeader: standalone logout icon removed; user avatar area replaced with
  a dropdown button (id="shelf-user-btn") showing initials + name + chevron
- Clicking the button calls ShelfUserMenu.toggle() to show/hide a panel with
  User Profile (/profile) and Log Out (/logout) items
- Outside-click and Escape close the menu
- ShelfUserMenu IIFE added to app.js
- MobileSidebar: Log Out link added at bottom (border-separated footer)
- userInitials() already returned correct initials; the ? fallback for empty
  name is unchanged

## [0.1.0-patched216] - 2026-04-01

### Fixed

**Issue 1 — JS confirm() dialogs replaced with ShelfConfirm**
listing.go ListActions delete button now calls
  event.preventDefault(); ShelfConfirm.showForm(this.form, '¿Confirmar eliminación?')
instead of return confirm(). The ShelfConfirm modal (added in patched215 to
app.js but not wired in listing.go) is now correctly bound.

**Issue 7 — Mobile sidebar nav chevrons now work**
renderNavGroup (layout.go) was using mi.ID("nav-group-{id}") and
mi.ID("nav-arrow-{id}"). Both desktop and mobile sidebars call navItems →
renderNavGroup, producing duplicate IDs in the DOM. ShelfNav.applyGroup
used getElementById which finds only the first match — always the desktop
sidebar element regardless of which sidebar triggered the toggle.

Fix: replaced IDs with data attributes:
  mi.ID("nav-group-"+id)  → mi.Attr("data-nav-group-id", id)
  mi.ID("nav-arrow-"+id)  → mi.Attr("data-nav-arrow-id", id)

app.js ShelfNav.applyGroup and ShelfNav.init now use
  document.querySelectorAll('[data-nav-group-id="X"]')
to update all matching elements in both sidebars simultaneously.

The head flash-prevention IIFE CSS now uses attribute selectors:
  [data-nav-group-id="X"]{max-height:0!important;opacity:0!important;}
  [data-nav-arrow-id="X"]{transform:rotate(-90deg)!important;}

### Added

**Issue 2 — ShelfModal2 auto-sizes to form content height**
Removed fixed height:'58vh'. Modal now uses maxHeight:'calc(100vh - 80px)'
and the body uses maxHeight:'calc(100vh - 200px)' with overflowY:'auto',
so the modal grows to fit its content and scrolls only when the form is
taller than the viewport.

**Issue 3 — Masters Create via ShelfModal2 dialog**
All 10 master listings now use CreateTargetModal2: true. The Create button
calls ShelfModal2.open(label, url, {maxWidth}) directly. The 4 master NEW
handlers that were missing IsPartial support (Regiones, Monedas, Alertas,
PropDefs) now detect HX-Request and call RenderPartial with IsPartial: true.
The 5 that already had it (CentrosCosto, Marcas, Custodios, Modelos,
Ubicaciones) needed only the listing config change.

**Issue 4 — Add Sensors via ShelfModal2 dialog**
SensorFormData: IsPartial bool added. SensorFormPage branches on IsPartial
to call sensorFormPartial (new helper, uses masterPartialFormWrapper targeting
#shelf-modal2-body). SensorNew handler detects HX-Request. Sensors listing
uses CreateTargetModal2: true, ModalWidth: "660px".

**Issue 5 — Dialog width configurable and tied to listing width**
ShelfModal2.open(title, url) now accepts an optional third argument opts:
  ShelfModal2.open('Title', '/url', {maxWidth: '600px'})
When maxWidth is provided, the modal width is constrained to that value.
ListPageConfig: ModalWidth string field added (passed to create dialog).
ListActionsConfig: EditModalWidth string field added (passed to edit dialog).
Each narrow master listing specifies its ModalWidth (600–700px) matching
its table's MaxWidth. Full-width masters (Custodios, PropAssocs) omit it.

**Issue 6 — Faster crossfade within the same section**
htmx:beforeSwap event handler added in layout.go:
  - Extracts the top-level path segment from both current URL and target URL
  - If they match (within-section navigation, e.g. /assets → /assets/123),
    sets --shelf-transition to 200ms for this swap
  - If different (cross-section navigation), uses the user's saved preference
htmx:afterSwap restores the saved preference after the transition completes.

## [0.1.0-patched215] - 2026-04-01

### Fixed

**Issue 1 — Sensors edit button yielded 404**
No /sensors/{id}/edit route exists. The sensors listing now shows only a view
(visibility) icon linking to /sensors/{id} (the detail page). The delete button
was also removed since no POST delete route is registered for sensors.

**Issue 5 — Users edit button yielded 404**
The route is GET /admin/users/{id} — no /edit suffix. The listing was incorrectly
generating /admin/users/{id}/edit. Corrected to /admin/users/{id}.

**Issue 4 — Master form action buttons left-aligned**
masterFormButtons now uses justify-end (was bare flex gap-3). The partial form
wrapper already had justify-end; the full-page path now matches.

### Added

**Issue 2 — ShelfConfirm modal messagebox**
New ShelfConfirm object in app.js replaces all window.confirm() calls in the
application. API:
  ShelfConfirm.show(message, onConfirm, opts)        — info variant
  ShelfConfirm.showDanger(message, onConfirm, opts)  — destructive (red OK)
  ShelfConfirm.showForm(form, message, opts)          — confirm then submit form
The opts object accepts title, okLabel, danger. Backdrop blur + scale animation
matches the ShelfModal visual language.

All ListActions delete buttons now call ShelfConfirm.showForm() instead of the
native confirm() dialog. (listing.go)

**Issue 3 — Masters edit via ShelfModal2 dialog (in-context)**
All 9 master listings with edit actions now open ShelfModal2 instead of
navigating to a separate full-page edit view.

listing.go: Added EditTargetModal2 bool to ListActionsConfig. When true,
the edit button calls ShelfModal2.open(title, url) directly — ShelfModal2
fetches the URL internally via htmx.ajax; no separate hx-get is needed.

IsPartial support added to the 4 masters that were missing it:
  Regiones, Monedas, AlertasMaster, PropDefs
  — RegionFormData, MonedaFormData, AlertaMasterFormData, PropDefFormData
    each gained IsPartial bool
  — Their FormPage functions now branch on IsPartial: when true they render
    masterPartialFormWrapper (which targets #shelf-modal2-body and provides
    its own footer buttons); when false they render the full standalone page
  — Their edit route handlers detect HX-Request and call RenderPartial

The 5 masters that already had IsPartial support (CentrosCosto, Marcas,
Custodios, Modelos, Ubicaciones) required only the listing update.

PropAssocs has no edit action (delete only) and was not changed.

## [0.1.0-patched214] - 2026-04-01

### Fixed — listing engine visual corrections (patched213 follow-up)

**Assets (assets.go)**
- Restored separate Estado column (was collapsed into the Activo name cell in
  patched213, causing status badges to appear below the name instead of in
  their own column).
- Criticidad now rendered as a colour-coded Badge pill (danger/warning/success)
  rather than coloured inline text.
- Fixed ListPageConfig.CreateURL being empty — the create button was rendered
  as a stray node outside ListPage; now wired through ListPageConfig.CreateHTMX
  so the header create button and the modal are correctly integrated.
- Removed stale standalone createAction node from the return value.
- Delete action removed from ListActions (AssetListData has no CSRFToken field;
  delete was handled via HTMX confirmation flow in the original, not inline).

**Sensors (pages.go)**
- Status field converted from coloured text span to Badge pill
  (active→success, inactive→default, pending→warning).

**Alerts operational (pages.go)**
- Severity badge variant now maps correctly: critical/high→danger (red),
  medium→warning (yellow), low→info (blue). Previously all fell through to
  default (gray) because severity strings are not Badge variant names.
- Action buttons restored to text-sm / py-1.5 sizing (were text-xs / py-1
  in patched213).

**Masters — pill consistency (masters_pages.go)**
- Custodios Estado: Badge("Activo","success") / Badge("Inactivo","default")
  replacing coloured text spans.
- AlertasMaster Criticidad: Badge with danger/warning/success variant mapping,
  replacing coloured text spans. Removed unused criticidadClass map and cls
  variable.
- PropDefs TipoDato: Badge("...", "info") for the human-readable type label.
  Note: PropDefRow has no long name field (only NombreCorto and TipoDato exist
  in the schema). The "long name" is a data model gap, not a rendering issue.

**Narrow listings — MaxWidth (listing.go + masters_pages.go)**
- Added MaxWidth string field to ListPageConfig. When non-empty, applies
  "max-w-X mx-auto" to the page's space-y-6 wrapper, constraining the full
  listing (header + table + pagination) to a centred column.
- Applied to 7 narrow masters:
    Regiones (2 cols)        → max-w-2xl
    Marcas   (2 cols)        → max-w-2xl
    Monedas  (3 cols)        → max-w-3xl
    Modelos  (3 cols)        → max-w-3xl
    Ubicaciones (3 cols)     → max-w-3xl
    CentrosCosto (3 cols)    → max-w-3xl
    PropDefs (3 cols)        → max-w-3xl

## [0.1.0-patched213] - 2026-04-01

### Added — unified listing engine (listing.go)

New file `internal/ui/listing.go` provides a single set of composable
primitives used by every table-style listing page in the application.

**ListPage** — outer shell rendering the page header (H1, optional subtitle,
optional primary create button as HTMX modal or href, optional secondary
action such as export), with slots for filter bar, content, and pagination.

**ListTable** — card-wrapped table with consistent thead, striped tbody, and
EmptyState component when the row slice is empty. SkipCard bool omits the
card wrapper for listings where tabs and table share one outer card (Alerts,
Users).

**ListRow / ListCell / ListCellMuted / ListCellRich / ListCellRight** — row
and cell helpers. ListRow applies even/odd striping. ListCell and ListCellMuted
produce plain-text <td> in primary or muted colour respectively. ListCellRich
wraps arbitrary mi.H (badges, coloured text, composite content). ListCellRight
is right-aligned for the actions column.

**ListActions** — standard icon-button actions cell (visibility / edit /
delete). Edit supports both href navigation and HTMX modal. Delete renders as
an inline POST form with CSRF and a confirm() guard. Replaces all four
previous action patterns: HTMX icon buttons (Assets), text links (masters),
non-functional delete button (Rules), and card-as-link (AssetTypes deferred).

**ListFilterBar** — GET form row with optional SearchInput and zero or more
auto-submit <select> filters. Replaces the three existing filter bar patterns
(Assets/Events style, Sensors labeled-card style, none).

**ListPaginationBar** — "Mostrando N de M label" count text paired with the
existing Pagination widget.

### Changed — all 18 table-style listings migrated

**10 masters** (masters_pages.go): Regiones, Monedas, CentrosCosto, Marcas,
Custodios, AlertasMaster, PropDefs, Modelos, Ubicaciones, PropAssocs.
All gain: consistent icon-button actions replacing text "Editar"/"Eliminar"
links; EmptyState component replacing inline italic text; consistent px-4 py-3
cell padding; striped rows. Delete buttons on all masters are now functional
POST forms (Rules had a broken non-functional delete button, now fixed).

**Events** (events.go): migrated to ListPage + ListTable + ListPaginationBar.
eventsFilterBar retained as custom mi.H (date inputs not expressible as
ListFilterField selects). eventsTable helper removed.

**Sensors** (pages.go): converted from card grid to table. sensorsGrid,
sensorCard, sensorTypeSelect helpers removed. Filter bar converted from
labeled-selects-with-explicit-button to ListFilterBar auto-submit style.

**Rules** (pages.go): migrated to ListPage + ListTable. rulesTable helper
removed. Delete button, previously wired to nothing, is now a functional
POST form via ListActions.

**Alerts** (pages.go): divide-y list rows converted to ListTable with
SkipCard=true. Tab navigation retained inside the shared card. alertsListContent
and alertListItem helpers removed.

**Users** (admin_users.go): migrated to ListPage + ListTable with SkipCard=true.
Role tab navigation retained. userCreateButton, usersTable, userTableRow
helpers removed. Permission-gated create button expressed via conditional
CreateURL in ListPageConfig.

**Assets** (assets.go): reference listing updated to use ListPage, ListTable,
ListActions, ListCellRich, ListCellMuted, ListPaginationBar. assetsTable
helper removed. HTMX modal create and auto-open behaviour preserved.

### Removed — obsolete master helpers
masterListHeader, masterTableWrap, masterTH, masterTD, masterDeleteButton
all deleted from masters_pages.go. All functionality now provided by the
listing engine.

### Note — self-check
extract_func Python helper treated {} inside interface{} type literals as brace
pairs, causing it to extract only the signature stub for masterTableWrap and
masterTD. The orphaned bodies were caught immediately at compile time and
removed. extract_func must not be used on functions whose signatures contain
{} in type literals without first stripping the signature.

## [0.1.0-patched212] - 2026-04-01

### Changed - crossfade duration 200ms → 500ms, configurable from Settings

**CSS (app.css)**
Transition duration moved from a hardcoded value into a CSS custom property:
  :root { --shelf-transition: 500ms; }
Both ::view-transition-old(shelf-main) and ::view-transition-new(shelf-main)
now reference var(--shelf-transition). JavaScript overrides the property on
:root at runtime without touching CSS files or requiring a page reload.

**layout.go — BaseLayout head**
A no-flash IIFE runs before first paint:
  (function(){
    var d = localStorage.getItem('shelf-transition');
    if (d) document.documentElement.style.setProperty('--shelf-transition', d);
  })()
This ensures the saved duration is applied before the browser renders the
first frame, preventing a brief flash of the default duration.

**layout.go — BaseLayout script**
ShelfUI namespace added alongside toggleMobileMenu:
  ShelfUI.setTransition(val)
    - Sets --shelf-transition on :root immediately (live effect)
    - Saves to localStorage('shelf-transition')
    - Syncs all .shelf-transition-select elements on the page

**pages.go — SettingsPage**
New "Appearance" card added between Features and Data Management:
  Page Transition Duration
  [Off | Fast (200ms) | Normal (500ms) | Slow (800ms)]
The select has class="shelf-transition-select" and onchange="ShelfUI.setTransition(this.value)".
A small inline IIFE reads localStorage on load and pre-selects the saved value.
No server round-trip: the preference is browser-local, appropriate for a
display preference that has no effect on server-rendered content.

## [0.1.0-patched211] - 2026-04-01

### Fixed - sequential fade replaced with true simultaneous crossfade

**What we were doing**

patched210 used htmx:afterSwap to restart the shelf-page-enter CSS animation
on #main-content after each HTMX swap. By the time afterSwap fires, the old
content is already gone (HTMX replaced innerHTML). The result: old content
disappears instantly, then new content fades in. Sequential, not crossfade.

**How a true crossfade works**

Both old and new states must exist simultaneously. HTMX 1.9.10 supports this
via hx-swap="innerHTML transition:true", which wraps the DOM mutation in
document.startViewTransition(). This is a same-document View Transition —
fully deterministic, no cross-document timing gaps, structurally identical to
a dialog animation. The CSS rules ::view-transition-old(shelf-main) and
::view-transition-new(shelf-main) (already present) animate old and new
simultaneously.

**Changes**

layout.go navItem(): hx-swap="innerHTML" → hx-swap="innerHTML transition:true"

layout.go htmx:afterSwap handler: animation restart now guarded by
  if (!document.startViewTransition) { ... }
so the shelf-page-enter fallback only fires in browsers without View
Transitions support (Firefox). In Chrome, the View Transition handles it.
Double-animating would otherwise fight the CSS crossfade.

app.css: shelf-main crossfade timings changed from asymmetric (160ms out /
220ms in) to symmetric 200ms for both. With matching durations and matching
easing (ease), combined opacity stays at 100% throughout — old fades out
while new simultaneously fades in with no visible gap or dimming.

## [0.1.0-patched210] - 2026-04-01

### Changed - HTMX partial navigation replaces cross-document view transitions

**Root cause of persistent top-bar flicker**

Cross-document View Transitions require the browser to:
  1. Snapshot the old page
  2. Load, parse, CSS-compute, and paint the new page in an offscreen buffer
  3. Composite both snapshots and begin the animation

Steps 2-3 are non-deterministic in timing. At 60fps a frame is 16ms. Any
layout phase that takes longer produces a dropped frame that shows as a flicker
regardless of what CSS animation is applied. Same-document DOM manipulations
(dialogs, modals) never exhibit this because both states exist simultaneously
in a fully-computed render tree. The comparison to the dialog animation
identified the architectural mismatch precisely.

**Solution — HTMX partial page swaps**

Nav link clicks now swap only #main-content via HTMX. The sidebar, topheader,
mobile header, and all chrome elements are never reloaded. Navigation becomes
a same-document XHR + innerHTML swap, structurally identical to how dialog
boxes are animated.

Changes:

handler.go — Render() and RenderWithConfig() now accept *http.Request and
auto-detect HX-Request: true. When detected, they call RenderPartial() and
set HX-Title so the browser updates <title>. The 14 manual IsHTMXRequest()
conditional blocks in routes.go are eliminated.

routes.go / masters_routes.go — All h.page.Render(w, ...) and
RenderWithConfig(w, cfg, ...) calls updated to pass r. Redundant
IsHTMXRequest conditionals collapsed.

layout.go navItem() — Each nav <a> receives:
  hx-get="/url"          — HTMX partial GET
  hx-target="#main-content" — swap target
  hx-swap="innerHTML"    — replace content only
  hx-push-url="true"     — update browser history
  data-nav-href="/url"   — URL for client-side active-state matching
  data-active-class="…"  — full active Tailwind class string
  data-inactive-class="…" — full inactive Tailwind class string

layout.go BaseLayout script — htmx:afterSwap event handler added:
  1. Restarts shelf-page-enter entrance animation on #main-content
     (style.animation = 'none' → offsetWidth reflow → style.animation = '')
  2. Updates nav active state in both desktop sidebar and mobile sidebar
     by matching window.location.pathname against data-nav-href and
     switching className to data-active-class or data-inactive-class
  3. Closes mobile sidebar overlay if open when navigation completes

app.css — @view-transition { navigation: auto } retained as progressive
enhancement for browsers where cross-document fallback is needed (e.g.
direct URL entry, back/forward, external links). The 60ms root crossfade
and shelf-main animation remain for those cases. For nav link clicks
(the overwhelming majority of in-app navigation), HTMX takes over and
View Transitions are bypassed entirely.

## [0.1.0-patched209] - 2026-04-01

### Fixed - crossfade artefact: tenant name and action buttons compressing toward centre

**Root cause**

view-transition-name: shelf-topbar (added in patched206) applies
contain: layout style paint to #top-header. The contain: layout property
can interfere with how a flex item resolves its width when it is inside a
flex-col parent column, causing the browser to briefly capture the element
at a reduced width. With justify-between, a narrower container compresses
all children toward the centre — left-aligned items move right, right-aligned
items move left, matching exactly the reported symptom.

Every attempt to fix this by adjusting named layers shifted the artefact
rather than eliminating it. The underlying issue is that assigning
view-transition-name to any chrome element that depends on parent flex
geometry for its width introduces contain: layout as a side-effect and
breaks the width resolution.

**Fix — remove all named layers from chrome; use fast root crossfade instead**

view-transition-name: shelf-topbar removed.
id="top-header" removed (no longer referenced by CSS).
#main-sidebar retains no view-transition-name (already removed in patched208).

The root pseudo-element (sidebar + topheader) now uses a 60ms linear
crossfade instead of animation: none:

  ::view-transition-old(root) { animation: 60ms linear both shelf-vt-out }
  ::view-transition-new(root) { animation: 60ms linear both shelf-vt-in  }

Because old-root and new-root are simultaneously visible during this overlap,
their combined opacity stays at 100% throughout — no element ever dims or
disappears. The topheader renders normally (fade-to-itself = invisible). The
sidebar nav item changes as a soft 60ms crossfade. The main content continues
its separate 160/220ms animation via shelf-main.

No contain: layout on any chrome element. No named-layer boundaries at x=256px.
No frame-gap hard-cuts. No flex width resolution artefacts.

## [0.1.0-patched208] - 2026-04-01

### Fixed - crossfade artefact: tenant name appearing to shift right

**Root cause analysis**

After patched206 promoted both #top-header (shelf-topbar) and #main-sidebar
(shelf-sidebar) to named view-transition layers, users reported a new symptom:
during page transitions the tenant name appeared to shift drastically to the
right, as if something was overflowing from the left.

Two compounding causes were identified:

1. Adjacent pseudo-element boundary at x=256px.
   The sidebar pseudo-element (shelf-sidebar) and the TopHeader pseudo-element
   (shelf-topbar) share a boundary at exactly the point where the sidebar ends
   and the main content begins. Rendering artefacts at this boundary could
   manifest as a visual shift in the TopHeader content.

2. Sidebar logo overflow during transition capture.
   The sidebar brand area contains two <img> elements (one per mode, toggled
   via dark:hidden / hidden dark:block). If both images briefly have display
   other than 'none' during the view-transition snapshot — before the CSS
   display rules are fully applied — they lay out side-by-side at 150px each
   (300px combined) inside a 256px sidebar that had no overflow: hidden. The
   44px overflow spills into the TopHeader's area, visually shifting the tenant
   name to the right in the pseudo-element capture.

**Fix 1 — Remove view-transition-name from sidebar (app.css)**

#main-sidebar is no longer promoted to its own layer. It remains in the root
capture with animation: none (hard cut). The sidebar nav item change is instant
and acceptable — the user just clicked it. Removing shelf-sidebar eliminates
the x=256px boundary artefact entirely.

**Fix 2 — Overflow-safe logo containers (layout.go)**

The sidebar brand div gains overflow-hidden. The <a> element wrapping the logo
pair is changed from flex items-center to block w-[150px] h-[36px] overflow-hidden.
With a fixed-dimension block container, even if both images briefly have display
other than none, the container is clamped to 150×36px and the overflow is clipped.
The logo never overflows under any circumstances.

Same treatment applied to MobileHeader: block w-[125px] h-[30px] overflow-hidden.

## [0.1.0-patched207] - 2026-04-01

### Added - dark mode icons and logos

Three source assets supplied in darkmode.zip (all RGBA with transparent
backgrounds, designed for legibility on dark surfaces):

  shelf_landscape_dark.png  500×120  landscape logo — lighter content (~159 brightness)
  icon_square_dark.png      230×230  square icon only — content ratio 0.80
  logo_square_dark.png      558×558  full square logo — content ratio 0.85

**Processing**

Landscape logos (direct resize, source fills canvas ~99%):
  logo-landscape-dark.png    300×72  (2×; displayed at 150×36 in desktop sidebar)
  logo-landscape-sm-dark.png 250×60  (2×; displayed at 125×30 in mobile header)

Square logos (direct resize; content ratio 0.85 matches existing light versions):
  logo-160-dark.png  160×160
  logo-320-dark.png  320×320

Icon files (cropped to content bbox before resize; existing light icons are
full-bleed, dark source has ~10% padding — crop preserves visual weight):
  icons/icon-32-dark.png    32×32
  icons/icon-48-dark.png    48×48
  icons/icon-180-dark.png  180×180
  icons/icon-192-dark.png  192×192
  icons/icon-512-dark.png  512×512

**Switching — Tailwind class-based (dark class on <html>)**

All visible logo locations use the dark:hidden / hidden dark:block pair:
  layout.go Sidebar           logo-landscape.png    ↔ logo-landscape-dark.png
  layout.go MobileHeader      logo-landscape-sm.png ↔ logo-landscape-sm-dark.png
  layout.go PublicLayout      logo-160.png          ↔ logo-160-dark.png
  login.go  login page        logo-160.png          ↔ logo-160-dark.png

**Favicon dark hints (media="(prefers-color-scheme: dark)")**

Applied in layout.go BaseLayout, PublicLayout, and login.go:
  icon-32-dark.png  — dark favicon for 32×32
  icon-180-dark.png — dark apple-touch-icon

Favicon switching uses OS preference (prefers-color-scheme) rather than
the app's class-based toggle. In the common case these are identical.

**Tests**

web/static_test.go TestVendorFilesPresent: eight new dark-mode file entries
covering landscape logos (×2), square logo, and favicon variants.

**Also includes patched206** (top-bar flicker fix) which was applied to the
broken shelf-ui tree but not yet in a clean checkpoint:
  layout.go: id="top-header" on TopHeader, id="main-sidebar" on Sidebar
  app.css: three-layer view-transition system (shelf-main, shelf-topbar, shelf-sidebar)

## [0.1.0-patched205] - 2026-04-01

### Fixed - dark/light mode toggle icon was invisible

**Root cause**

`DarkModeSVGIcons()` from minty renders two Heroicon SVGs with class="w-5 h-5".
Those classes are absent from the Tailwind-purged CSS: the minty module lives in
$GOPATH/pkg/mod and is not in the tailwind.config.js content scan path, so the
standalone .w-5 and .h-5 rules are never emitted. Without CSS dimensions the SVG
has no defined size, rendering invisibly in the flex container.

The functionality (toggle working, JS running) was unaffected — only the visual
icon was missing. The bug pre-dates patched203; it was present since the Tailwind
CLI build was introduced, and first noticed when reviewing the UI after patched203.

**Fix**

Replaced DarkModeSVGIcons() with DarkModeIcons() in the darkMode initialiser in
layout.go, passing Material Icons HTML strings that use the already-vendored font:

  LightIcon: `<span class="material-icons text-xl">dark_mode</span>`
             crescent moon — shown in light mode, click to go dark
  DarkIcon:  `<span class="material-icons text-xl">light_mode</span>`
             sun — shown in dark mode, click to go light

Both dark_mode and light_mode are confirmed present in the vendored WOFF2.
The Material Icons font handles rendering with no CSS dimension dependency.

**Test hardening**

TestIconNamesInFont in web/static_test.go now has two spot-check categories:
- mustFindInScan: icons that must be found by the double-quote regex AND be in
  the font (patched202 dashboard icons — they appear as "icon_name" literals)
- mustBeInFont: icons used inside backtick HTML strings whose content is not
  reachable by the double-quote regex scan. dark_mode and light_mode are in
  this category. Font presence is verified directly against the ligature table.

## [0.1.0-patched204] - 2026-03-31

### Changed - build discipline hardening (reviewer recommendations)

Addresses the highest-priority findings from an external build/release review.

**README version drift — root cause fixed**
syncver.sh set_version() was matching `**Current version:** v0.1.0-...` (with a
spurious `v` prefix) but README.md had no `v` prefix. The sed pattern never
matched, so README.md never updated. Fixed: removed the `v` from the pattern.
README.md updated to 0.1.0-patched204. Future `syncver.sh set` calls will keep
it current automatically.

**Shell script linting — shellcheck now wired into build and release**
All four bash scripts (release.sh, syncver.sh, scripts/launch.sh,
scripts/bundle.sh) now pass `shellcheck --format=gcc` with zero warnings.

Warnings fixed:
- release.sh: `[ ${PIPESTATUS[0]} -eq 0 ]` → quoted `"${PIPESTATUS[0]}"`
- syncver.sh: `sed "s/.*-patched//"` → parameter expansion `${current##*-patched}`
- launch.sh: two SC2015 `A && B || C` patterns refactored to explicit if/then/else
- launch.sh: unused BLUE variable and FIRST_RUN variable removed (SC2034)

shellcheck added to:
- `release.sh` as step 3a (gracefully skips if shellcheck not installed)
- `make check` preflight target

**package.json formalised**
Previously: minimal `{"devDependencies":{"tailwindcss":"^3.4.19"}}` — no name,
no version, no scripts, range pin (^) instead of exact pin.
Now: name, version, description, private:true, scripts (css, css:watch), exact
pin (3.4.19 without ^), engines (node>=18). package-lock.json regenerated.

release.sh updated from `npm install --save-dev tailwindcss@3.4.19` to
`npm ci --prefer-offline` — uses the lockfile for a reproducible, pinned tree.

**make check — single canonical preflight target**
Runs in order (non-destructive, safe to run any time):
1. Icon audit (make check-icons / scripts/check-icons.py)
2. Version consistency (syncver.sh check)
3. Shell linting (shellcheck --format=gcc on all bash scripts)
4. Go tests (go test ./...)
5. Go vet (go vet ./...)
6. JS syntax (node --check on all three JS files)

Added to Makefile .PHONY, header comment, and CSS/Icons section.

**Zip manifest — MANIFEST.txt added to checkpoint zip**
Each checkpoint zip now contains MANIFEST.txt at its root with:
- Version, build date (UTC), git commit (or "unavailable" if not in a repo)
- Go version, Node version
- SHA-256 of tailwind.css, MaterialIcons-Regular.woff2, htmx@1.9.10.min.js

Makes checkpoint zips auditable: an operator can unzip and read MANIFEST.txt
to know exactly what was on the machine when the checkpoint was cut.

## [0.1.0-patched204] - 2026-03-31

### Changed - build and release discipline improvements (reviewer feedback)

Addresses the actionable findings from the external reviewer's assessment.
Items deferred: CI/CD (acknowledged as out of scope), xolu version pinning
(separate concern), test-workflow tracked-file mutation (low risk, deferred).

**README version drift — fixed**
syncver.sh sed pattern had a spurious `v` prefix:
  s|**Current version:** v[^ ]*|...| — never matched (README has no `v`)
  s|**Current version:** [^ ]*|...|  — correct pattern
README.md version updated from 0.1.0-patched17 to current. Future `syncver.sh
set` calls will keep it in sync automatically.

**Shell scripts — all four now pass shellcheck --format=gcc with zero warnings**
Issues found and fixed:
- release.sh: ${PIPESTATUS[0]} → "${PIPESTATUS[0]}" (SC2086, unquoted)
- syncver.sh: sed "s/.*-patched//" → ${current##*-patched} (SC2001, style)
- launch.sh: two A && B || C patterns → proper if/then/else (SC2015)
- launch.sh: unused BLUE colour variable removed (SC2034)
- launch.sh: unused FIRST_RUN variable removed (SC2034)

**package.json — formalised**
Added name, description, private, scripts (css, css:watch), engines (>=18).
Pinned tailwindcss from "^3.4.19" (floating patch) to "3.4.19" (exact).
package-lock.json regenerated against the exact pin.

**release.sh — npm install → npm ci**
Replaced ad-hoc `npm install --save-dev tailwindcss@3.4.19` with `npm ci
--prefer-offline`. npm ci uses package-lock.json exclusively, fails fast if
the lockfile is absent or out of sync with package.json, and never modifies
the tree. This makes the CSS generation step fully reproducible.

**release.sh — shellcheck step added (step 3a)**
Runs `shellcheck --format=gcc` on all four bash scripts before the Tailwind
build. Gracefully skips with a warning if shellcheck is not installed.

**release.sh — MANIFEST.txt injected into checkpoint zip**
Every checkpoint zip now contains MANIFEST.txt at its root documenting:
  - Project version and build date (UTC)
  - Git commit short hash (or "unavailable" if outside a repo)
  - Go and Node version strings
  - SHA-256 of tailwind.css, MaterialIcons-Regular.woff2, htmx@1.9.10.min.js
Makes checkpoint artefacts auditable: a future operator can unzip MANIFEST.txt
alone to determine exactly what went into the build.

**Makefile — make check target added**
Single idempotent preflight command covering:
  1. Icon audit (make check-icons)
  2. Version consistency (syncver.sh check)
  3. Shell script linting (shellcheck)
  4. Go tests (go test ./...)
  5. Go vet (go vet ./...)
  6. JS syntax (node --check on all three static JS files)
No artefacts are cut, no files are mutated. Safe to run at any time.

## [0.1.0-patched203] - 2026-03-31

### Added - icon audit integrated into build and test pipeline

**scripts/check-icons.py**
Python script (fonttools + brotli, build-time only) that:
- Loads the vendored WOFF2 and extracts all 2,234 ligature names
- Scans internal/ui/**/*.go for string literals matching icon-name patterns
- Verifies every found icon name is present in the font
- Regenerates web/static/vendor/material-icons-ligatures.txt
- Exits non-zero if any icon is missing
- Accepts --check-only flag (audit without writing files, used by release.sh)

**web/static/vendor/material-icons-ligatures.txt**
All 2,234 ligature names extracted from MaterialIcons-Regular.woff2, sorted,
one per line. Committed to the repo so the Go test suite requires no Python
at test time. Regenerated by make check-icons or release.sh.

**web/static_test.go — TestIconNamesInFont**
Pure Go test that:
- Reads material-icons-ligatures.txt from the embedded FS
- Walks ../internal/ui/**/*.go (source tree; skips if not present)
- Finds all string literals that match a ligature name (same strategy as Python)
- Asserts the scan found a non-zero set (sanity check)
- Spot-checks the patched202 icons (crisis_alert, engineering, dangerous, inventory_2)
No Python required at test time. `go test ./...` is fully self-contained.

**Makefile — two new targets**
- make check-icons: runs scripts/check-icons.py (requires fonttools + brotli)
- make css: regenerates web/static/css/tailwind.css via the Tailwind CLI
Both targets and their descriptions added to the header comment and .PHONY list.

**release.sh — step 4c**
After Tailwind CSS build and before Go compile check: runs
scripts/check-icons.py --check-only. Gracefully skips with a warning if
python3 or fonttools/brotli are not installed (not a hard failure because
the Go test provides the same coverage without Python).

## [0.1.0-patched202] - 2026-03-31

### Changed - dashboard card improvements (previously agreed, not yet shipped)

These changes were analysed and approved during a prior session but were lost
to context compaction before implementation. Applied now.

**dashStatCard (dashboard.go)**

Old layout: tiny muted icon (text-gray-400, text-xl) in the top-right corner
alongside a label, then a large number below. Card height ~76px. Padding p-5.
The icon was decorative and added nothing to legibility.

New layout: p-6 padding. Left-anchored 48×48px rounded accent block containing
a 24px icon (text-2xl) in a coloured tint. Right of the block: label (text-sm
gray) above the large number. Urgent non-zero cards get red tint; all others
get indigo tint. Card height ~110px.

Icon choices updated:
- notifications_active → crisis_alert (siren ring: unambiguously urgent)
- build            → engineering  (hard-hat person: physically industrial)
- priority_high    → dangerous    (octagonal stop shape: genuine hazard weight)

**dashTotalCard (dashboard.go)**

Old: label + text-4xl number + link, padding p-5. ~90px content height, hollow
when CSS grid equalises it alongside the ~130px status breakdown.

New: same 48×48px accent block pattern as stat cards (inventory_2 icon, indigo
tint) alongside the label+number, followed by the "Ver listado →" link. Padding
p-6. Visual language now consistent across all metric cards on the dashboard.

The Alertas Recientes, Vencimientos Próximos, and Por Estado cards are unchanged
— they are data-driven and their height is correct as-is.

## [0.1.0-patched201] - 2026-03-31

### Changed - mobile sidebar overlay: second logo replaced with tenant name

MobileSidebar previously showed logo-landscape-sm.png in its header strip,
duplicating the logo already visible in the persistent MobileHeader behind it.

Replaced with:
- "workspace" label in text-xs text-gray-400 (contextual, single-tenant aware)
- cfg.TenantName in text-sm font-bold text-indigo-600 dark:text-indigo-400
  with truncate to handle long names and flex-shrink-0 on the close button
  so the name never pushes the close button off-screen

The close button gains flex-shrink-0 so long tenant names cannot displace it.
The tenant name column uses min-w-0 to allow truncation correctly in flex context.

Effect: on mobile, opening the sidebar now shows the tenant name at the top
rather than a redundant second copy of the app logo. The landscape logo remains
in the persistent top bar (MobileHeader) unchanged.

## [0.1.0-patched200] - 2026-03-31

### Added - page transition cross-fade on nav link clicks

Added two-layer page transition system to web/static/css/app.css.

**Layer 1 — entrance animation (all browsers)**
#main-content { animation: shelf-page-enter 220ms ease-out both }
Fades in with a 6px upward slide on every full page load. Fires regardless
of browser support for View Transitions.

**Layer 2 — View Transitions API (Chrome 126+)**
@view-transition { navigation: auto } enables automatic cross-document
transitions for all same-origin navigations (nav link clicks).

view-transition-name: shelf-main is assigned to #main-content only, so
the sidebar and top header remain visually stable during navigation — they
snap instantly to the new active state while only the content area animates.
The root transition is suppressed (animation: none) to prevent the sidebar
from flickering as the active nav item changes.

Cross-fade timing: 160ms fade-out, 220ms fade-in.

On browsers without View Transitions support the feature is a no-op and
layer 1 provides the entrance animation as fallback.

prefers-reduced-motion: all animations are suppressed.

## [0.1.0-patched199] - 2026-03-31

### Changed - desktop sidebar logo centred

Added justify-center to the h-16 flex container in Sidebar().
Mobile headers are already centred by the surrounding justify-between layout.

## [0.1.0-patched198] - 2026-03-31

### Changed - landscape logo replaces disjointed icon+text headers

The three Shelf AMS header locations (desktop sidebar, mobile top bar, mobile
sidebar overlay) previously showed a small 32×32px icon alongside a text span
reading cfg.AppName ("Shelf AMS") styled in Tailwind bold/indigo. The icon and
text were visually separate HTML elements, creating an inconsistent appearance
compared to professional software headers.

**New images** (generated from shelf-logo-landscape.png, transparent background
via flood-fill from edges, LANCZOS resampling, stored at 2× for retina sharpness):
- web/static/images/logo-landscape.png — 300×72px displayed as 150×36px
  Used in: desktop sidebar (h-16 header area)
- web/static/images/logo-landscape-sm.png — 250×60px displayed as 125×30px
  Used in: mobile top bar + mobile sidebar overlay (h-14 header area)

Both images have transparent backgrounds and embed the icon and "Shelf AMS"
brand name as a single composition. On dark mode backgrounds the transparent
PNG renders correctly since only the visible pixels (dark navy text, gradient
icon) are opaque.

**layout.go** — three brand blocks updated:
- Sidebar(): icon-32.png + text span → logo-landscape.png (150×36)
- MobileHeader(): icon-32.png + text span → logo-landscape-sm.png (125×30)
- MobileSidebar(): icon-32.png + text span → logo-landscape-sm.png (125×30)

The gap-2 flex gap is removed (no longer two separate elements).
The Span with cfg.AppName is removed (name is now part of the image).

**web/static_test.go**: two new entries in TestVendorFilesPresent covering
both logo-landscape.png and logo-landscape-sm.png.

## [0.1.0-patched197] - 2026-03-31

### Changed - vendor documentation and DEPLOY.md updated

**web/static/vendor/VENDOR.md — full rewrite**

Replaced the previous draft with a definitive inventory document covering:

- Exact upstream canonical URLs for every asset (direct file URL, npm tarball URL,
  npm homepage URL, GitHub release URL, and CDN mirrors where applicable).
- Exact re-vendoring commands with sha256sum steps for each asset.
- Separate SHA-256 + size for every vendored file including all 5 Leaflet PNG images.
- Complete sha256sum -c compatible checksums block for batch verification.
- Tailwind section updated: no longer "not yet downloaded" — fully generated by
  release.sh; SHA-256 and size current; canonical npm tarball URL added; note that
  cdn.tailwindcss.com is not on npm and only available from Cloudflare.
- Lit section updated: explains why npm package cannot be served directly (sub-path
  exports), documents the esbuild bundling step with exact flags.
- Material Icons section expanded: both npm and original Google Fonts URLs documented;
  explains the two-hop CDN (googleapis.com CSS → gstatic.com WOFF2) that is replaced.
- Wiring status table updated: all columns show complete (✓); references to the
  automated tests that enforce the invariant.
- OSM tile server section: table format with effort/notes per approach.
- Release date and Shelf version recorded at top of document.

**DEPLOY.md — two additions**

Prerequisites section: Node.js 18+ added as a build-time (not runtime) dependency.

New section "Self-contained binary and offline operation": explains that all UI
assets are embedded in the binary, that air-gapped deployments work except for map
tiles, and that go build works without Node using the committed tailwind.css.

## [0.1.0-patched196] - 2026-03-31

### Added - vendor and HTML integrity tests

**web/static_test.go** — 6 tests in the web package verifying the embedded FS:
- TestVendorFilesPresent: every expected vendor file exists, meets a minimum
  size bound, and contains/excludes specific content patterns. Covers all 15
  expected files: htmx, leaflet (JS/CSS/5 images), MaterialIcons WOFF2 and CSS,
  lit, tailwind.css, app.css, VENDOR.md.
- TestMaterialIconsFontPath: @font-face points to /static/vendor/fonts/, not
  to fonts.googleapis.com or fonts.gstatic.com.
- TestLeafletCSSImagePaths: relative image references present; no external
  url() declarations (comment-only URLs are ignored).
- TestTailwindCSSNotScript: tailwind.css is generated CSS, not the CDN play
  JS script (guards against accidentally committing the wrong file).
- TestVendorInventoryComplete: VENDOR.md mentions all five asset categories.

**internal/ui/vendor_html_test.go** — 9 tests in the ui package verifying
rendered HTML:
- TestBaseLayoutNoCDNRefs / TestPublicLayoutNoCDNRefs: rendered HTML contains
  none of the 6 CDN origins (cdn.tailwindcss.com, fonts.googleapis.com,
  fonts.gstatic.com, unpkg.com, esm.sh, cdnjs.cloudflare.com).
- TestBaseLayoutVendorPaths / TestPublicLayoutVendorPaths: the four required
  vendored asset paths appear in the rendered <head>.
- TestLitImportMapUsesVendor: LitImportMap emits /static/vendor/lit@3.js,
  not esm.sh.
- TestMapNavItemDefaultVisible: Mapa link present when EnableMap=true.
- TestMapNavItemHiddenWhenDisabled: Mapa link absent when EnableMap=false.
- TestMapViewRedirectsWhenDisabled: GET /map → 303 /assets when disabled.
- TestMapViewOKWhenEnabled: GET /map does not redirect when enabled.

Also fixed release.sh: die → fail (the script uses fail, not die).
tailwind.config.js added to repo root (was previously created only in a
temp directory and not committed to the project tree).

## [0.1.0-patched195] - 2026-03-31

### Added - Map feature toggle in Settings

The Map (Mapa) feature is now configurable from the Settings page rather than
always-on. This supports deployments where an OSM tile server is not available
(air-gapped, offline, or restricted-network environments).

**Config.EnableMap (layout.go)**
New bool field on Config, defaulting to true. Protected by the existing configMu
read-write mutex so the nav and map handler see a consistent value.

**MainNav gating (layout.go)**
MainNav now takes cfg Config as a second parameter. When EnableMap is false it
emits a zero-value NavItem{} placeholder. The Sidebar rendering loop gained a
guard that skips items with empty URL and Label, so the placeholder is silently
dropped from the nav without leaving a gap.

**Settings page — Features card (pages.go)**
New "Features" card between Notifications and Data Management. Contains a single
toggle: "Mapa de Activos", wired to POST /settings/features. The description text
explains the tile-server dependency so operators understand the implication before
enabling or disabling.

**SettingsSaveFeatures handler (routes.go)**
Saves {"key":"features","map_enabled":bool} to tenant_settings in OLU, then
immediately updates h.config.EnableMap and h.page.Config.EnableMap under lock.
The change takes effect on the next page render with no restart required.

**loadPersistedSettings (routes.go)**
Extended to also query tenant_settings WHERE key='features' and restore EnableMap
from the map_enabled field. Called at startup and every 30 seconds by the
background refresh goroutine, so the setting survives service restarts.

**MapView gate (routes.go)**
GET /map now checks EnableMap at request time. If disabled, redirects to /assets
with 303 SeeOther. The route remains registered so a direct URL never returns 404.

**POST /settings/features route (router.go)**
Registered alongside the existing /settings/general and /settings/notifications
routes.

## [0.1.0-patched194] - 2026-03-31

### Changed - full vendor wiring: zero external CDN references at runtime

Completed the transition from CDN-loaded assets to fully vendored assets embedded
in the binary. Node.js is now a build-time dependency only and is never required
at runtime.

**Tailwind CSS (CDN Play → CLI build)**
The 112 KB CDN Play script (a full JIT compiler running in the browser) has been
replaced by a 35 KB pre-generated CSS file produced by the Tailwind CLI at build
time. tailwind.config.js scans internal/**/*.go and web/static/js/*.js. The
generated web/static/css/tailwind.css is committed to the repo so the binary can
be built without running Node. release.sh regenerates it before every Go compile
check. The tailwind.config = {darkMode:'class'} inline script tag is also removed
since that configuration now lives in tailwind.config.js.

**HTMX, Material Icons, Leaflet, Lit**
All four remaining CDN URLs removed from layout.go, login.go, password_reset.go,
pages.go. All now served from /static/vendor/. The UseVendoredJS flag and jsURL()
helper removed — no longer needed.

**Leaflet image paths**
vendor/leaflet-images/ renamed to vendor/images/ so that relative URL references
in leaflet@1.9.4.css (url(images/marker-icon.png) etc) resolve correctly.

**material-icons.css simplified**
The .material-icons class rules were already present in app.css. The vendored CSS
now contains only the @font-face declaration to avoid duplication.

**VENDOR.md updated**
Wiring status table now shows all assets fully wired. Tailwind section updated with
CLI build details, SHA-256, and re-generation procedure.

## [0.1.0-patched193] - 2026-03-31

### Added - vendor directory with third-party frontend asset inventory

Created web/static/vendor/ containing all vendorable third-party frontend assets.
All files are embedded by the existing //go:embed in web/static.go.

Assets vendored:
- htmx@1.9.10.min.js — 47 KB, BSD 2-Clause, from htmx.org npm
- leaflet@1.9.4.js / .css / leaflet-images/ — 145+15 KB, BSD 2-Clause, from leaflet npm
- fonts/MaterialIcons-Regular.woff2 — 126 KB, SIL OFL 1.1, from @fontsource/material-icons
- material-icons.css — CSS with @font-face + .material-icons class rules, local font path
- lit@3.js — 16 KB ESM bundle, BSD 3-Clause, bundled from lit@3.2.1 npm via esbuild

Not yet vendored (requires manual download from internet-connected machine):
- tailwind-play@3.4.19.js — not on npm, only at cdn.tailwindcss.com (blocked in build env)

Wiring: vendor files exist but layout.go/login.go/password_reset.go still reference
CDN URLs. A follow-up commit will extend the UseVendoredJS mechanism to cover all
assets. The vendor/VENDOR.md inventory documents exact versions, SHA-256 checksums,
licenses, air-gap implications, and re-vendoring procedures for all assets.

## [0.1.0-patched192] - 2026-03-31

### Changed - ShelfModal adaptive sizing (1000px ceiling, 4K mode, narrow mode)

**Ceiling raised to 1000px**
maxHeight increased from 900px to 1000px. On any display with viewport
height >= 1096px (common desktops, 4K monitors) the modal now hits 1000px.

**4K display detection**
physW = screen.width * devicePixelRatio gives actual physical pixel count,
reliable across all OS scaling modes (100%, 150%, 200% HiDPI).
When physW >= 3840 (any 4K or 5K display, regardless of scaling):
  - Width cap: 1290px -> 1800px (maxWidth: calc(100vw - 96px))
  - Height cap: 1000px -> 1100px
At native 4K (no scaling): modal is 1800x1100 vs previous 1290x900 — 47% more area.
At 4K/200% HiDPI: physW is still 3840, so the wider/taller modal applies too;
the logical viewport is 1920x1080 and the modal fills it at 96%.

**Narrow viewport detection**
When vw < 1420 (1366x768 laptops, 13" MacBook Air at 1280px):
  - Body padding reduced from 20px/24px to 16px/16px
  - Reclaims 16px of usable width on each side
  - No other dimension changes; the 1290px cap combined with maxWidth:calc(100vw-32px)
    already ensures near-full-width usage on those screens

## [0.1.0-patched191] - 2026-03-31

### Changed - asset form modal fills available viewport height

Previously height was set to 56vh. On a 1920x1080 display that targets 604px
even though the maxHeight ceiling of 900px would have allowed the form to be
296px taller. The form appeared centred in a large empty space because 56vh
never reached the ceiling that would have capped it at something sensible.

Changed height to calc(100vh - 48px) so the modal tries to use the full
available viewport height (24px breathing room top and bottom). maxHeight:900px
caps it on large displays. The floor minHeight:500px is unchanged.

Resulting heights by viewport:
  <528px   → 500px  (floor)
  768px    → 720px
  900px    → 852px
  >=1024px → 900px  (ceiling)

## [0.1.0-patched190] - 2026-03-31

### Fixed - launch.sh ready-box alignment

Three URL lines (UI, API, olu) used printf %-25s for the port/path value.
With a 29-char prefix and 1-char suffix the total was 55, one short of the
56-char box width. Changed to %-26s.

The clipboard note line used hardcoded trailing spaces in an echo statement,
producing a 51-char line (5 short). Replaced with printf %-52s so the padding
is computed rather than counted by hand.

All ten box lines now render at exactly 56 characters regardless of version
string or port values.

## [0.1.0-patched189] - 2026-03-31

### Fixed - Create New Asset form size invariance

Two related bugs caused the Create New Asset form to appear at an incorrect size.

**Submit button race condition (asset_detail.go)**

The Guardar/Crear button in data-modal-footer used type="submit" with
form="asset-edit-form". When clicked, the browser fires a submit event on the
referenced form element. HTMX should intercept this event (the form has hx-post),
but when the button is outside the form the submit is triggered by the HTML form
owner mechanism rather than a direct click on the form's submit button. HTMX may
not intercept it before the browser's native form submission runs, sending a plain
POST without the HX-Request header. The server then renders a full-page form.

Fix: added hx-post, hx-include="#asset-edit-form", hx-target, and hx-swap attrs
directly on the button. HTMX intercepts the click directly on the button (which
does have hx-* attrs) before the browser's submit event fires. The form= attribute
is retained as a non-JS fallback only.

**Full-page render not size-constrained (asset_detail.go)**

AssetFormPage when IsPartial=false returned b.Div(Class("w-full"), Form(...))
with no width or height constraints. If the full-page path is reached (validation
error without HTMX, or JS disabled), the form stretched to full page width.

Fix: the full-page (non-partial) render now wraps the form in a centred container
(width:1290px, min-height:500px, max-height:900px, overflow-y:auto) matching the
modal dimensions. The form is visually identical whether shown in the modal or as
a standalone page.

## [0.1.0-patched188] - 2026-03-31

### Fixed - asset type summary card pointed to detail page instead of edit form

In summary_handlers.go, AssetTypeSummary used:
  editURL := fmt.Sprintf("/asset-types/%d", id)

All other entities correctly use /{entity}/{id}/edit, but asset type was missing
the /edit suffix. This meant ShelfModal2.open() loaded GET /asset-types/{id} which
routes to AssetTypeDetail — a read-only full-page view with nav and logo — instead
of AssetTypeEdit which returns an HTMX partial.

Fix: added /edit suffix to the Sprintf format string.

TestAssetTypeSummary_ContainsName extended to assert the body contains
/asset-types/{id}/edit so the same regression is immediately caught.

## [0.1.0-patched187] - 2026-03-31

### Fixed - asset modal size and close-button transition

**Width (app.js)**
ShelfModal width increased from 1170px to 1290px (+10.3%).

**Height bounds (app.js)**
ShelfModal previously had only an upper bound (calc(100vh - 32px)).
On small or short viewports the modal could shrink below any usable height.
Added minHeight: 500px and changed maxHeight to min(900px, calc(100vh-32px)).
The modal now flexes with the viewport (56vh) but never below 500px or above 900px.
At 1080p the working height is 604px; at 768p it is capped at the 500px floor.

**Close button transition (app.js)**
Both ShelfModal and ShelfModal2 close buttons were registered as:
  closeBtn.addEventListener('click', close)
When fired as an event handler the browser passes the MouseEvent as the first
argument. Inside close(immediate), any truthy value takes the instant-removal
path — so the x button always dismissed without animation.
The Cancel button calls ShelfModal.close() with no arguments so immediate is
undefined (falsy) and the animated path ran correctly.
Fix: wrapped both handlers in a no-argument lambda:
  closeBtn.addEventListener('click', function () { close(); })
The FSM editor dialog was not affected (it already uses a lambda).
Escape key and backdrop-click handlers were also already correct.

## [0.1.0-patched186] - 2026-03-31

### Fixed - ShelfModal2 partial rendering broken for Marca, CentroCosto, Custodio (all error paths), AssetType

Root cause audit: the patched183 implementation had four distinct failure modes.

**Marca** - Edit and all Update paths (validation error, DB error, success redirect)
were never saved. The first Python script modified them in memory then failed an
unrelated assertion before writing; the follow-up script skipped Marca entirely.
All three paths now have IsHTMXRequest branching: Edit returns RenderPartial,
Update validation/DB errors return RenderPartial, success returns HTMXTrigger.

**CentroCosto** - Edit was also never saved (same cause). The validation error
path in Update was saved but the DB error path and success redirect had no HTMX
check. All three paths now fixed.

**Custodio** - Edit and success redirect were saved correctly. The validation error
path fix landed in CustodioCreate instead of CustodioUpdate (near-identical string
patterns, wrong occurrence replaced). The DB error path was saved. Both remaining
paths now fixed.

**AssetType** - Edit and Update handlers were never touched at all. AssetTypeFormPage
had no IsPartial branch. Added IsPartial to AssetTypeFormData, added a limited
partial form (name, description, color; FSM and field template editors excluded
with a note directing to the full page), and added IsHTMXRequest branching to
both the Edit and Update handlers.

### Added - htmx_partial_test.go (47 tests)

Comprehensive test coverage for ShelfModal2 partial rendering across all 6 entities:
- Edit handler with/without HX-Request header (partial vs full page)
- Update handler with HX-Request + valid data: 200 + HX-Trigger:shelfModal2Saved
- Update handler with HX-Request + invalid data: partial with validation error
- Non-HTMX Update still redirects (regression guard)
- Data integrity: Custodio and Modelo Update bodies verified field-by-field
- Summary endpoints: 200 with key field content, 404 for ID=0

## [0.1.0-patched185] - 2026-03-31

### Fixed - asset modal footer alignment + ShelfPopover/ShelfModal2 missing

Two bugs introduced in patched183:

Footer alignment (asset_detail.go): The data-modal-footer div lacked flex
layout, so when JS moved it into the #shelf-modal-footer slot the two children
(delete button + cancel/save group) both floated left. Fixed by adding
flex items-center justify-between w-full to the div so the slot's own
justify-content:space-between applies to its two direct children.

ShelfPopover undefined (app.js): The Python insertion script for ShelfPopover
and ShelfModal2 failed its second assertion (afterSwap handler) before writing
the file, so the two module definitions were silently lost. Only the afterSwap
extension (from a follow-up script) was persisted. Fixed by inserting both
window.ShelfPopover and window.ShelfModal2 at the correct top-level position
(just before window.ShelfModal) with an immediate file write.

## [0.1.0-patched184] - 2026-03-31

### Changed - logo images: white background made transparent

logo-160.png and logo-320.png processed with a boundary flood-fill (BFS from
all four corners, fuzz=25) to make the white background transparent. The fill
only removes pixels reachable from the image edges, preserving any white areas
that are genuinely enclosed within the logo elements. Anti-aliased edge pixels
are handled by the fuzz tolerance.

The favicon icons (icon-*.png, favicon.ico) are not affected; they retain a
solid white background as required for browser tab display on platforms that
do not support alpha-channel favicons.

## [0.1.0-patched183] - 2026-03-31

### Added — FK jump icons: inline popover + stacked editing modal

Clicking a jump icon in the asset form now shows a compact summary popover
instead of navigating away. Two actions inside the popover:
- Nueva pestana: opens the entity edit page in a new browser tab
- Editar: opens ShelfModal2, a stacked dialog on top of the asset modal,
  for in-context editing without losing the current form state.

ShelfPopover (app.js): fetches /{entity}/{id}/summary and renders a card
anchored to the clicked button. One popover at a time; click outside to close.

ShelfModal2 (app.js): stacked modal above ShelfModal (z-index 10000).
Visually distinct: 2px indigo border, blue-tinted header, 760x58vh. Loads
entity edit partial via htmx.ajax into #shelf-modal2-body. Same footer
extraction pattern as ShelfModal. On save success, HX-Trigger:shelfModal2Saved
closes the inner modal and returns focus to the asset form.

Summary endpoints (summary_handlers.go): new GET /{entity}/{id}/summary routes
for custodios, marcas, modelos (marca resolved), ubicaciones (region resolved),
centros-costo, asset-types.

Partial edit (masters_routes.go + masters_pages.go): Custodio, Marca, Modelo,
Ubicacion, CentroCosto edit/update handlers now support IsHTMXRequest partial
rendering. New masterPartialFormWrapper helper in masters_pages.go.

Jump button change (asset_detail.go): buttons changed from anchor to button
with onclick=ShelfPopover.show; icon changed from open_in_new to info.

## [0.1.0-patched182] - 2026-03-31

### Fixed — asset form modal: action buttons in truly fixed footer

Replaces the CSS sticky approach from patched181 with a structural fix.

**Problem with sticky:** The buttons were still inside the scrollable
`#shelf-modal-body` div. CSS `position:sticky` is unreliable for this use
case — it collapses when content is shorter than the container, and behaves
inconsistently across mobile browsers.

**New structure:**

```
┌─ modal (flex-column, fixed height 56vh) ───────┐
│  header  (flex-shrink:0)  title + close        │
├────────────────────────────────────────────────┤
│  body    (flex:1, overflow-y:auto)             │
│  ← #shelf-modal-body, HTMX target             │
│  ← form fields only, no buttons               │
├────────────────────────────────────────────────┤
│  footer  (flex-shrink:0)                       │
│  ← #shelf-modal-footer, always visible        │
│  ← Delete / Cancel / Save                     │
└────────────────────────────────────────────────┘
```

**JS (ShelfModal in app.js):**
- `_build()` now creates a `_footer` div as a third flex child of the modal
  alongside header and body. Background and border match the modal.
- `_footer` reference cleared in `close()`.
- `htmx:afterSwap` handler now extracts `[data-modal-footer]` from the HTMX
  response body and moves it into `#shelf-modal-footer`. This runs on every
  swap, including validation-error re-renders, so the buttons are always fresh.

**Go (asset_detail.go):**
- When `IsPartial=true`:
  - Form gets `id=asset-edit-form`
  - The action buttons block inside formAttrs is replaced with `b.Span()`
  - A `<div data-modal-footer>` is emitted as a sibling of the form (not inside
    it); the submit button uses `form=asset-edit-form` (HTML form owner
    attribute) so it still submits the form from outside it
- When `IsPartial=false` (full-page render): buttons remain inside the form
  as a standard inline footer, unchanged.

## [0.1.0-patched181] - 2026-03-31

### Changed — asset form modal: sticky action footer, shorter height, FK jump links

**Fixed bottom bar for action buttons (asset_detail.go)**

The action buttons (Cancelar / Guardar Cambios / Crear Activo) were inside the
scrollable body of the modal. When form content exceeded the modal height the
buttons scrolled out of view and became unreachable.

Fix: the button container uses `position:sticky; bottom:0; z-index:10` so it
stays pinned to the bottom of the visible scroll area regardless of content
height. Negative side margins (`margin:8px -24px 0`) + matching padding extend
the bar to the full modal width so content scrolling underneath is hidden by
the opaque background (`bg-white dark:bg-slate-800`).

**20% shorter modal (app.js)**

`ShelfModal` height reduced from `70vh` to `56vh` (70 × 0.8).

**FK jump icons**

Small `open_in_new` icon added next to the label of FK select fields. Clicking
opens the referenced entity's edit/detail page in a new tab. The icon is hidden
when no value is selected and updates live as the user changes the select.

Fields with jump links and their target routes:

| Field | Jump target |
|-------|-------------|
| Tipo de Activo | `/asset-types/{id}` |
| Marca | `/marcas/{id}/edit` |
| Modelo | `/modelos/{id}/edit` |
| Ubicación | `/ubicaciones/{id}/edit` |
| Custodio | `/custodios/{id}/edit` |
| Centro de Costo | `/centros-costo/{id}/edit` |

Fields skipped (too simple to warrant a jump): Moneda, Región, enums.

New Go function `assetFKSelectWithJump()` wraps `assetFKSelect` adding the
icon and a `data-jump-base` attribute. New JS module `FKJumps` in `app.js`
reads `data-jump-base` on each select, binds a `change` listener, and calls
`init()` after every HTMX swap into `#shelf-modal-body`.

## [0.1.0-patched180] - 2026-03-31

### Fixed — login page using hardcoded SVG favicon and placeholder icon

`login.go` builds its own standalone HTML page outside of `BaseLayout` and
`PublicLayout`, so the favicon and logo changes in patched179 had no effect on it.

Three fixes:
1. `LoginPageData` gains a `StaticURL string` field; the handler now passes
   `h.config.StaticURL` so the page uses the same base path as the rest of the app.
2. Favicon in the login `<head>` changed from the hardcoded `/static/favicon.svg`
   to PNG + ICO links using `StaticURL` (same pattern as `BaseLayout`).
3. The logo area replaced the `inventory_2` Material Icons placeholder in a blue
   circle with the real `logo-160.png` image, matching the public layout.

## [0.1.0-patched179] - 2026-03-31

### Added — static images bundled into binary via go:embed

**Source images** (from shelf-images.zip):
- `icon_square.png` (230×230) — brand icon without text
- `logo_square.png` (558×558) — full logo with "Shelf AMS" wordmark

**Generated variants** (LANCZOS resampling, saved to `web/static/`):

| File | Source | Size | Use |
|------|--------|------|-----|
| `favicon.ico` | icon | 16/32/48 multi-size | Legacy browsers |
| `icons/icon-16.png` | icon | 16×16 | Favicon small |
| `icons/icon-32.png` | icon | 32×32 | Favicon standard, sidebar |
| `icons/icon-48.png` | icon | 48×48 | Windows taskbar |
| `icons/icon-180.png` | icon | 180×180 | Apple touch icon |
| `icons/icon-192.png` | icon | 192×192 | PWA manifest (Android) |
| `icons/icon-512.png` | icon | 512×512 | PWA splash screen |
| `images/logo-160.png` | logo | 160×160 | Login / public pages |
| `images/logo-320.png` | logo | 320×320 | og:image, larger display |

**Binary bundling:**
- `web/static.go` — new file; `//go:embed static` exposes `web.Static embed.FS`
- `internal/server/server.go` — switches from `http.Dir("web/static")` to
  `http.FS(fs.Sub(web.Static, "static"))`; the binary is now fully self-contained
- Dev override: set `SHELF_STATIC_DIR=web/static` to serve from disk instead
  (useful for live CSS/JS iteration without rebuilds)

**Layout updates:**
- `BaseLayout`: favicon links updated to PNG (32×32, 16×16, ICO fallback) and
  `apple-touch-icon` updated to `icon-180.png`
- `PublicLayout` (login/QR pages): head gets same favicon links; body header
  replaced with `logo-160.png` image (was plain CSS text)
- `MobileHeader`: icon-32.png added alongside app name text
- `MobileSidebar` header: icon-32.png added alongside app name text
- Desktop `Sidebar` brand: icon-32.png added alongside app name text
- `manifest.json`: updated to reference PNG icons (192, 512, 180) instead of SVG

## [0.1.0-patched178] - 2026-03-31

### Fixed — tenant name not appearing; sidebar collapse not persisting

**Tenant name (top bar):**

Two bugs combined to produce an empty top bar:

1. `loadPersistedSettings` runs once at `NewUIHandler` time. If the seed
   runs *after* the server starts, the in-memory config is never updated — the
   tenant name stays blank until the server restarts.
   Fix: `startSettingsRefresh()` starts a background goroutine that re-queries
   `tenant_settings` from olu every 30 seconds, so the name appears within
   one refresh cycle after seeding.

2. Concurrent writes to `h.config` and `h.page.Config` from the refresh
   goroutine and from `SettingsSaveGeneral` were unguarded.
   Fix: added `configMu sync.RWMutex` to `UIHandler`; all writes to
   `TenantName`/`Locale` now hold the lock.

**Sidebar collapse (state not persisting, buttons becoming unresponsive):**

The flash-prevention inline `<script>` in `<head>` injects a `<style>`
element using `!important` to hide collapsed groups before the sidebar
renders. `ShelfNav.init()` was then setting `items.style.maxHeight = '0'`
as a plain inline style — but the `!important` CSS won silently. When
`toggle()` tried to expand by setting `maxHeight = '600px'`, the
`!important` rule overrode it again, so groups could never be opened. The
button click fired and the state was written to `localStorage`, but the
DOM did not change — making buttons look stuck.

Fix — rewritten `ShelfNav` with a three-step init:
1. Remove the `<style>` element injected by the head script (identified by
   containing `nav-group-`), handing control over max-height to JS.
2. Set initial states with `transition: none` so there is no animation
   on page load; use a fixed `600px` ceiling for expansion rather than
   `scrollHeight` (which returns 0 when measured on a collapsed element).
3. Re-enable transitions after one `requestAnimationFrame` so subsequent
   toggle clicks animate correctly.

`sync` import added to routes.go.

## [0.1.0-patched177] - 2026-03-31

### Added — collapsible sidebar nav groups with persistent state

Sidebar menu groups (Operacional, Configuración, Maestros, Administración) are
now collapsible. State persists across sessions via `localStorage` under the
key `shelf-nav-groups`.

**Go (layout.go):**
- `navGroupID(label)` — converts a section label to a stable group ID using
  the same slug rules as JS (lowercase, accents stripped, spaces→hyphens).
- `navItems()` rewritten to group the flat `[]NavItem` list by Section
  headers, rendering each group as a collapsible `<button>` header + `<ul>`.
- `renderNavGroup()` emits the group container. The `<ul>` carries
  `data-nav-group-active` so JS always keeps the active group open.
- Inline `<script>` in `<head>` (synchronous, before sidebar renders)
  reads `localStorage` and injects a `<style>` block that pre-collapses
  groups — no visible flash on page load.
- `strings` import added to layout.go.
- Sidebar footer simplified: removed redundant tenant-name line (already in
  top bar), kept only the version string.

**JS (app.js):**
- `ShelfNav` module defined before the main IIFE so it is available when
  the IIFE's `init()` calls `ShelfNav.init()`.
- `ShelfNav.toggle(id)` — flips expanded/collapsed, saves to localStorage,
  animates via `max-height` transition (200 ms ease).
- `ShelfNav.init()` — on DOMContentLoaded, reads saved state and applies
  it without animation (transition temporarily suppressed then re-enabled via
  double `requestAnimationFrame`). Active groups are always expanded and their
  saved state is corrected so they stay open across navigations.

## [0.1.0-patched176] - 2026-03-31

### Fixed — tenant settings loaded at startup; SettingsSaveGeneral syncs both configs

**Root cause of TenantName not persisting across restarts:**
`SettingsSaveGeneral` stored the tenant name in olu (`tenant_settings`
key=general) and updated `h.config.TenantName` on the UIHandler. But
`Render()` reads from `h.page.Config` (PageHandler), which is a separate
value copy created at startup. So saved settings were never reflected in the
rendered top bar.

**Three fixes:**

1. `SettingsSaveGeneral` now updates both `h.config` and `h.page.Config`
   so the top bar reflects the new name immediately after saving.

2. `loadPersistedSettings(ctx)` — new method on UIHandler. On startup it
   queries `tenant_settings WHERE key = 'general'` from olu and updates
   both config copies. If olu is unreachable or has no settings yet, it
   no-ops gracefully.

3. `NewUIHandler` refactored from a one-liner return to a two-step
   construct-then-load, so `loadPersistedSettings` can be called before
   the handler is returned. Added `context` import to routes.go.

**Seeds:** both `seed-01-intec.py` and `seed-02-frigocold.py` now include a
`seed_tenant_settings()` section (section 15) that creates two
`tenant_settings` documents in olu:
- `key=general`: tenant_name (vertical-specific), locale=es
- `key=notifications`: email_alerts=True, slack_enabled=False,
  webhooks_enabled=True

## [0.1.0-patched175] - 2026-03-31

### Changed — top bar: tenant name left, search bar right

Tenant name (`cfg.TenantName`) is now displayed on the left of the top
header bar with a `domain` icon, replacing the search bar that was there.
The search bar moves into the right-side actions group (before notifications),
with a fixed `w-64` width instead of the previous `flex-1 max-w-md`.

`TenantName` was already persisted to `tenant_settings` in olu and loaded
into `Config` at startup — no new storage needed. Set it via
Settings → General → Tenant Name. Falls back to an empty string (no label)
when not configured.

## [0.1.0-patched174] - 2026-03-31

### Changed — modal resized; single edit path via ShelfModal

**Modal dimensions:** 1170px wide (+20%), 70vh tall (-20%).

**Edit consolidation:** The asset detail page previously had a standalone
"Edit" button that navigated to `/assets/{id}/edit` as a full page.
Replaced with an "Editar" button identical in behaviour to the list row
edit icon: `hx-get=/assets/{id}/edit` + `onclick=ShelfModal.open(...)`.
Both entry points (list row icon, detail page button) now open the same
ShelfModal form. The `/assets/{id}/edit` route is retained as a full-page
fallback for direct URL access.

## [0.1.0-patched173] - 2026-03-31

### Fixed — ShelfModal fixed height; no resize on tab switch

Changed `maxHeight: '88vh'` to `height: '88vh'` on the modal element.
The modal now occupies a fixed 88vh regardless of which tab is active
or how much content it contains. Tab switches no longer cause the window
to grow or shrink. `maxHeight: calc(100vh - 32px)` is retained as a
safety limit for very small viewports. The body div (flex: 1,
overflow-y: auto) absorbs all height variation: shorter tabs show
whitespace below their content, taller tabs scroll.

## [0.1.0-patched172] - 2026-03-31

### Added — expanded validation test coverage (48 tests total)

18 new tests added to `validation_handler_test.go`, covering handlers not
previously tested and adding `errorsToSummary` unit tests.

**New handler coverage:**
- `CentroCostoCreate`: missing codigo_contable -> form; valid -> 303
- `RegionUpdate`: empty descripcion -> form (Update schemas = Create schemas)
- `ModeloUpdate`: missing marca_id -> form (zero int64 stripped)
- `AssetTypeCreate`: missing name/slug -> non-redirect; valid -> 303
- `RuleCreate`: missing name -> non-redirect; invalid trigger_type -> non-redirect; valid -> 303
- `SensorCreate`: missing external_id -> non-redirect; valid -> 303
- `AdminUserUpdate`: invalid role -> non-redirect; valid -> 303
  Uses `req.SetPathValue("id", "1")` (handler uses r.PathValue, not URL parsing).
  Mock uses `PUT /api/v1` because olu.Update sends PUT, not POST.

**errorsToSummary unit tests:** nil/empty -> ""; single field; multiple
fields sorted; top-level `_` key appears first without field prefix.

## [0.1.0-patched171] - 2026-03-30

### Fixed — asset form now shows all validation errors in a top-level banner

**Root cause of the invisible error problem:** queryfy returns a full
`map[string]string` of field→message pairs. The asset form only surfaced errors
for fields whose individual widgets were wired to `data.Errors`. Fields like
`status` and `connectivity_tier` had unwired widgets, so their errors were
silently discarded — the re-rendered form looked unchanged.

**errorsToSummary** (validate.go): new helper that converts any
`map[string]string` error map into a sorted, multi-line human-readable string.
Top-level (`"_"`) errors appear first without a field prefix; field errors
follow sorted alphabetically as `field: message`. This is the fallback used
whenever individual field widgets are not wired.

**AssetFormData.Error string**: new field. When set, AssetFormPage renders it
as a red banner above the tab bar (`whitespace-pre-line` so the multi-line
summary formats correctly). Populated via `errorsToSummary(errs)` in all three
asset error paths: pre-check, post-check create, post-check update.

**errorBanner logic**: now checks `data.Error` first; if empty and
`len(data.Errors) > 0`, builds a summary automatically. This means any future
handler that populates `Errors` but forgets to set `Error` still shows a
visible banner — the field-wiring gap can no longer cause silent failures.

## [0.1.0-patched170] - 2026-03-30

### Fixed — 'Crear Activo' button appeared to do nothing (invisible validation errors)

**Root cause (patched167 regression):** The asset create form, when submitted
via the ShelfModal HTMX path, was sending `status=""` and
`connectivity_tier=""` (unselected optional selects). At patched167, before
the empty-string stripping introduced in patched168, these empty strings failed
their Enum constraints in queryfy. The handler re-rendered the form with errors
stored in `data.Errors["status"]` and `data.Errors["connectivity_tier"]` —
but `statusSelect` and `connectivitySelect` had no error message parameter,
so the errors were silently discarded. The re-rendered form looked identical to
the submitted form; the user saw no change and concluded the button did nothing.

**Fix 1 (patched168, already shipped):** Empty-string stripping in
`validateForm` removes the root cause — optional enum fields with unselected
blank values are now treated as absent and skip enum validation entirely.

**Fix 2 (this patch):** `statusSelect` and `connectivitySelect` now have
`*Err` variants that accept and display a field error message with red border
and red text, matching the pattern used by `assetEnumSelectErr`. The asset
form wires `fieldError(data.Errors, "status")` and
`fieldError(data.Errors, "connectivity_tier")` into these calls, so any future
validation failure on these fields is visible to the user rather than invisible.

All other asset form enum selects (criticidad, calidad_tenencia, motivo_baja)
were already wired in patched168.

## [0.1.0-patched169] - 2026-03-30

### Changed — asset modal wider; Detalles tab split into two

**ShelfModal width**: 780px → 975px (+25%).

**Asset form tabs** (4 tabs, up from 3):
- Principal — unchanged
- Ubicación y Custodia — unchanged
- Ciclo de Vida — lifecycle dates (6 date fields in 2×3 grid), motivo de baja,
  links ciclo de vida, custom prop values
- Finanzas y Service — precio compra/actual, moneda, centro de costo, fin de
  garantía, links documentos, service/IoT parameters and config rows

Custom property values moved from a bottom-of-tab-3 appendix into the right
column of Ciclo de Vida, where they sit naturally alongside lifecycle data.

## [0.1.0-patched168] - 2026-03-30

### Added — end-to-end validation tests; fixed empty/zero stripping in validateForm

**validation_handler_test.go** (37 new tests in `internal/ui/`):
Coverage: every handler wired with `validateForm`, proving the full path from
form submission through `validateForm`, queryfy schema evaluation, error map
population, and form re-render with inline error messages.

Test pattern per handler:
- *Missing required field* → 200 (form re-render), body contains
  `"field is required"`
- *Invalid enum value* → 200, body contains `"must be one of"`
- *Type constraint violation* → 200, body contains specific constraint message
- *Valid submission* → 303 redirect to list page, olu.Create was called

Handlers covered: RegionCreate, MonedaCreate, MarcaCreate, CustodioCreate,
AlertaMasterCreate, PropDefCreate, ModeloCreate, UbicacionCreate,
PropAssocCreate, AssetCreate (3 failure modes + valid path).

**validateForm — empty string and zero int64 stripping:**
HTML forms submit `` for all unselected inputs and selects.
`toInt64FromString()` returns `int64(0)` for FK selects with no selection.
Both are semantically "not provided" — treated as absent by queryfy to:
- Let Optional enum constraints (e.g. `calidad_tenencia`, `motivo_baja`) pass
  when the user makes no selection, rather than failing with "must be one of"
- Let Optional Positive FK fields (e.g. `marca_id=0`) pass when no FK is
  selected, rather than failing with "must be positive"
- Give Required fields the cleaner `"field is required"` error instead of
  `"length must be at least 1, got 0"`

**assetEnumSelectErr** — new variant of `assetEnumSelect` that accepts a
field error message and renders it as a red paragraph below the select, with
a red border on the control. Wired to `data.Errors` for `criticidad`,
`calidad_tenencia`, and `motivo_baja` in `AssetFormPage`.

## [0.1.0-patched167] - 2026-03-30

### Fixed — full schema/form/validation audit

**PropAssocCreateSchema.obligatoriedad** — typo: schema had `"requerido"`
but the form sends `"requerida"` (matching the UI label and the list page
renderer). One character difference; every PropAssoc save with obligatoriedad=
requerida would fail validation. Fixed in the schema.

**RuleCreateSchema / RuleUpdateSchema** — these were aliases for `RuleSchema`,
the API-level schema that requires `conditions[]` and `actions[]` arrays.
The UI rule form sends a flat representation (`trigger_type`, `condition_type`,
`threshold`, `action_type`, etc.). Every rule create/update would fail with
"conditions: field is required". Fixed by adding `RuleUISchema` — a separate
schema for the UI form — and pointing `schemaRuleCreate` at it.

**AssetTypeCreateSchema.identity_template** (Required) — verified safe: 
`parseFieldsFromForm` always returns a non-nil `map[string]any{}` even when
no identity fields are defined. queryfy accepts an empty map as satisfying
`Required()` for Object fields; only nil or absent keys fail.

**Audit findings (no further action needed):**
- Sensor required fields (external_id, type, driver): schema marks them
  required and handler has matching ad-hoc checks — redundant but consistent.
- PropDefinitionCreateSchema.validacion: optional, not sent by form, harmless
  with Loose mode.
- All other enum values verified to match between schemas and form renderers.

## [0.1.0-patched166] - 2026-03-30

### Fixed — 'unexpected field' error on Asset Type create/update

Two root causes:

1. **validateForm used Strict mode** (queryfy default). Strict rejects any field
   not declared in the schema. UI forms legitimately submit extra fields —
   hidden inputs, dynamic template fields, metadata like `default_alerta_id`.
   Fix: `validateForm` now calls `queryfy.ValidateWithMode(..., queryfy.Loose)`.
   Loose mode validates declared fields and silently ignores undeclared ones.
   Strict mode remains correct for API boundary validation; Loose is the right
   default for HTML form submissions.

2. **default_alerta_id missing from AssetTypeCreateSchema**. The AssetType form
   submits `default_alerta_id` (the alert to trigger by default for this type),
   but the schema had no declaration for it. Added as `Number().Optional()` to
   both `AssetTypeCreateSchema` and the Update schema.

## [0.1.0-patched165] - 2026-03-30

### Changed — asset form moved to ShelfModal; tabs consolidated to 3

**ShelfModal** (`web/static/js/app.js`):
- Same visual language as the FSM editor property dialog: blur backdrop,
  scale 0.72→1 animation, 12px border radius, 60px shadow, dark mode aware.
- 780px wide (vs 400px in the FSM dialog), body scrolls, max-height 88vh.
- Escape and backdrop click dismiss. `ShelfModal.open(title)` /
  `ShelfModal.close()`. HTMX afterSwap re-executes inline `<script>` blocks
  (needed for the tab-switching script in the form partial).

**AssetFormPage — 3 tabs** (down from 5):
- *Principal*: name, type, status, criticidad, connectivity; then GIAI/GRAI/
  barcode/tag; then marca/modelo/series/lote/dynamic fields. Everything an
  operator fills at intake, two columns on wide screens.
- *Ubicación y Custodia*: location + custody, two columns. Unchanged.
- *Detalles*: lifecycle dates (2×3 grid) + finanzas + service/IoT, two columns.
  Combined because all three are filled infrequently.

**HTMX modal wiring**:
- `AssetNew` / `AssetEdit`: return `RenderPartial` when `HX-Request` header
  is present; `IsPartial=true` on the form data sets `hx-post` on the form
  and a `ShelfModal.close()` Cancel button instead of page navigation.
- `AssetCreate` / `AssetUpdate`: on success, send `HX-Redirect: /assets`
  when HTMX request (triggers list reload). On validation error, re-render
  the partial so it swaps back into the modal body.
- Assets list *Nuevo Activo* buttons (header + empty state): `hx-get` to
  load the form partial, `onclick` to open the modal shell first.
- Assets list *Edit* row buttons: converted from `<a href>` to `<button>`
  with `hx-get=/assets/{id}/edit` and `onclick=ShelfModal.open(...)`.
- Full-page fallback: `/assets/new` and `/assets/{id}/edit` still render
  the full layout when accessed directly (no `HX-Request` header).

## [0.1.0-patched164] - 2026-03-30

### Added — full queryfy validation coverage across all form handlers

**New schemas** added to `internal/validation/schemas.go`:
- `PropAssocCreateSchema` / `PropAssocUpdateSchema`: prop_definition_id
  (Number required), entidad_destino (Enum: asset_type | asset), entidad_id
  (Number required), obligatoriedad (Enum: requerido | opcional)
- `ModeloCreateSchema` / `ModeloUpdateSchema`: marca_id (Number required),
  nombre (String required, maxlen 100)
- `UbicacionCreateSchema` / `UbicacionUpdateSchema`: nombre_corto (String
  required, maxlen 50), descripcion (String optional), region_id (Number required)
- `UserUpdateSchema`: name (String required), role (Enum: admin | manager |
  operator | viewer)

**Newly wired handlers** (all via `validateForm` before olu call):
- `PropAssocCreate` — first time; re-renders form with Errors on failure
- `SensorCreate` — replaces ad-hoc 3-field check with schema validation
- `AssetTypeCreate` / `AssetTypeUpdate` — had no validation at all previously
- `RuleCreate` / `RuleUpdate` — had no validation at all previously
- `AdminUserUpdate` — had no validation; now checks name + role before olu

**AdminUserCreate** deliberately kept with ad-hoc validation: password
confirmation (`password != passwordConfirm`) is cross-field logic that queryfy
cannot express. The existing `map[string]string` errors pattern already
produces good per-field messages and is correct.

**Complete coverage**: all 27 Create/Update/Save form handlers now run through
queryfy schemas before touching olu. The only exception is AdminUserCreate
(documented above).

## [0.1.0-patched163] - 2026-03-30

### Fixed — full validation type-safety audit across all form handlers

**Audit findings:**

All master table handlers (Región, Moneda, CentroCosto, Marca, Custodio, Alerta,
PropDef) were clean — no Number fields in their schemas, no type mismatches.
The one Bool field (CustodioCreateSchema.estado) was already correctly converted
via `r.FormValue("estado") == "on"`.

All 9 Number fields in AssetCreateSchema/AssetUpdateSchema (type_id, marca_id,
modelo_id, ubicacion_fisica_id, region_id, id_padre, custodio_id, moneda_id,
centro_costo_id) were already correctly converted to int64 in the data maps,
except type_id which was fixed in patched162.

**Modelo and Ubicación — wired for the first time:**

These two handlers were skipping queryfy entirely, going straight to olu without
any validation. This patch adds:
- `ModeloCreateSchema` / `ModeloUpdateSchema`: marca_id (Number, required),
  nombre (String, required, maxlen 100)
- `UbicacionCreateSchema` / `UbicacionUpdateSchema`: nombre_corto (String,
  required, maxlen 50), descripcion (String, optional, maxlen 200),
  region_id (Number, required, positive integer)
- Schema aliases added to `validate.go`
- validateForm wired into ModeloCreate, ModeloUpdate, UbicacionCreate,
  UbicacionUpdate before olu calls, re-rendering form with Errors on failure

**No other type mismatches found** across 18 Create/Update handlers.

## [0.1.0-patched162] - 2026-03-30

### Fixed — asset type selection always rejected with 'expected number, got string'

- `type_id` in both `AssetCreate` and `AssetUpdate` data maps was stored as
  the raw string from `r.FormValue("type_id")`. The queryfy schema declares
  `type_id` as `builders.Number().Required()`, which accepts `int64` and
  `float64` but not `string` — so every asset save failed regardless of the
  selected type.
- Fix: `"type_id": toInt64FromString(typeID)` in both handlers.
- Note: all other FK fields (marca_id, modelo_id, custodio_id, etc.) were
  already converted via `toInt64FromString` and were unaffected.

## [0.1.0-patched161] - 2026-03-30

### Added — queryfy validation wired into all UI form handlers

**validate.go** — new file in `internal/ui`:
- `validateForm(data, schema)` runs a queryfy schema and returns
  `map[string]string` keyed by field path, or nil if valid
- `firstError(errs)` returns the first message for the top-level form banner
- `fieldError(errs, field)` returns the per-field message for inline display
- Schema aliases expose `internal/validation` schemas without requiring
  handlers to import that package directly

**Master table form data structs** — all 10 now carry `Errors map[string]string`
alongside the existing `Error string` top-level banner field.

**masterFormInputErr** — new helper in `masters_pages.go` renders a red-bordered
input with an inline error paragraph when `errMsg != `. All 19
`masterFormInput` calls upgraded to `masterFormInputErr` passing
`fieldError(data.Errors, fieldname)`.

**Create handlers wired** (7 master tables + AssetCreate):
Región, Moneda, CentroCosto, Marca, Custodio, Alerta, PropDef, Asset.
Each runs `validateForm` before calling `h.olu.Create` and re-renders the
form with `Errors` populated on failure.

**Update handlers wired** (7 master tables + AssetUpdate):
Same entities. Failed updates re-render the edit form with field errors.

**AssetCreate / AssetUpdate** — two-pass validation:
1. Pre-check on scalar fields (name, type_id, criticidad) before assembling
   the full data map — fast path, avoids loading all reference data
2. Full schema pass on the assembled `assetData` map before calling olu

**AssetCreateSchema / AssetUpdateSchema rewritten** to match spec blocks A–G:
- Required: name, type_id, criticidad (create only)
- Block A: giai, grai, codigo_barras_rfid, tag_tecnico (optional, maxlen)
- Block B: marca_id, modelo_id, serie_1/2, lote, criticidad
- Blocks C–G: all optional fields with correct enum constraints
- Old schema referenced removed fields (identity, location, tags, notes)

**Tests updated**: `TestAssetCreateSchema` and `TestAssetHandler_Create` updated
to include `criticidad` (now required) and use correct enum values.

## [0.1.0-patched160] - 2026-03-30

### Fixed — Create Asset button was silently blocked

- `fecha_ingreso` had `required=true` (HTML attribute) but lives in the
  Ciclo de Vida tab, which starts hidden (`display:none`). Browsers that
  enforce native HTML5 form validation cannot focus a required field inside
  a hidden element and silently block submission — no error, nothing happens.
- Fix: removed `required` from `fecha_ingreso` in the form rendering.
  Server-side validation remains the authoritative enforcement path.
  The label still displays the field name without a star (it is not
  mandatory per the spec for the create operation).
- General rule now applied: HTML `required` attributes must only be placed
  on fields that are visible on the initially active tab. Fields on hidden
  tabs should rely on server-side validation only.

## [0.1.0-patched159] - 2026-03-30

### Fixed — all forms returned 403 on submit

- CSRF token field name mismatch: `ValidateToken` reads `r.FormValue("_csrf")`
  (the auth package constant `CSRFFormField`), but all master table forms
  and the alert acknowledge form embedded the token as `name="csrf_token"`.
  Fixed 12 occurrences across `masters_pages.go` and `pages.go`.
  The asset form was already correct (used `CSRFField()` component which
  reads from `auth.CSRFFormField`).

### Changed — asset form redesigned with tabs and two-column layout

- Replaced 8 stacked single-column cards with a 5-tab interface:
  Descripción, Ubicación y Custodia, Ciclo de Vida, Finanzas, Service/IoT.
  A sixth Propiedades tab appears when the asset type has custom properties.
- Each tab uses a two-column grid on lg screens where blocks pair naturally:
  - Descripción: left = Identificación + status fields; right = Caracterización
  - Ubicación: left = Ubicación Física + coords; right = Custodia + comodato
  - Ciclo de Vida: three-column date grid
  - Finanzas: left = precio/moneda; right = centro costo/garantía/docs
  - Service/IoT: full width (repeating configs benefit from horizontal space)
- Tab switching uses ~20 lines of inline vanilla JS — no HTMX, no framework.
  All panels remain in the DOM inside the same `<form>`, so the single
  submit fires all fields regardless of active tab.
- Submit buttons renamed to Guardar Cambios / Crear Activo / Cancelar.
- `assetTabBtn` helper renders individual tab buttons.

## [0.1.0-patched158] - 2026-03-30

### Added — regression test for EnableOperational/EnableAdmin defaults

- Added two assertions to `TestDefault` in `internal/config/config_test.go`:
  `EnableOperational` and `EnableAdmin` must be `true` in `Default()`.
- Each assertion carries an error message that names the concrete symptom:
  all operational routes return 404 / all admin routes return 404.
- Verified: reverting the patched157 fix causes both assertions to fail,
  confirming the tests would have caught the original regression.

## [0.1.0-patched157] - 2026-03-30

### Fixed — operational and admin routes were never registered on a clean launch

- `config.Default()` did not set `EnableOperational` or `EnableAdmin` — both
  bool fields defaulted to Go's zero value (false). The comments correctly
  stated "default: true" but the code did not reflect this.
- All routes inside the `if r.config.EnableOperational` and
  `if r.config.EnableAdmin` blocks were silently unregistered unless the
  operator explicitly set `SHELF_ENABLE_OPERATIONAL=true` or
  `SHELF_ENABLE_ADMIN=true` in the environment, or added the fields to their
  config.json. This caused every sidebar link to return 404 on a fresh launch.
- Fix: `Default()` now sets both flags to `true` alongside the other
  `EnableUI`, `EnableAPI` etc. defaults.

## [0.1.0-patched156] - 2026-03-30

### Added — workflow test suite (internal/ui/workflow/)

- Separate Go module at `internal/ui/workflow/go.mod` with its own
  dependency graph. `github.com/ha1tch/xolu` is imported here only —
  the main shelf-ui `go.mod` is untouched.
- `replace` directives: `github.com/ha1tch/shelf-ui => ../../../` and
  `github.com/ha1tch/xolu => /tmp/olu` (overridable via `XOLU_PATH`).
- `newWorkflowEnv(t)` helper spins up a real embedded xolu server:
  SQLite storage in `t.TempDir()`, `cache.NewMemoryCache`, `FlatGraph`,
  `noopValidator`, zerolog disabled. Each test gets a fresh isolated database.
- 6 end-to-end workflow tests in `workflow_test.go`:
  - `TestWorkflow_RegionInUseCannotBeDeleted`: create región → create
    ubicación referencing it → delete región fails (409) → delete
    ubicación → delete región succeeds
  - `TestWorkflow_MarcaInUseCannotBeDeleted`: same pattern for
    Marca/Modelo FK chain
  - `TestWorkflow_PropDefInUseCannotBeDeleted`: PropDef/PropAssoc chain
  - `TestWorkflow_AssetExpiryAppearsOnDashboard`: asset created with
    `fecha_expiracion` 10 days out; dashboard renders asset name in
    vencimientos panel
  - `TestWorkflow_AlertAcknowledgeReducesOpenCount`: alert created as
    open; AlertAcknowledge fires; OQL count of open alerts decreases
  - `TestWorkflow_InactiveCustodioExcludedFromDropdown`: active and
    inactive custodios created; AssetNew rendered HTML contains active
    name, does not contain inactive name
- `make test-workflow` target in Makefile: guards for missing xolu,
  supports `XOLU_PATH` override, runs with `-timeout 120s`
- `make test` unchanged — workflow tests run only via `make test-workflow`

## [0.1.0-patched155] - 2026-03-30

### Added — internal/ui test suite (29 tests across 4 files)

**helpers_test.go** (8 tests) — pure function coverage:
- `TestMasterID_ValidPaths` / `TestMasterID_InvalidPaths`: 12 path-parsing
  cases including zero ID, negative, non-numeric, missing suffix
- `TestParseDocumentLinks_Normal` / `_SkipBlankRows` / `_PartialRow`:
  multi-value form array assembly with blank-row skipping semantics
- `TestToInt64FromString`, `TestToFloatFromString`: 7-case each, edge cases
- `TestFormatNumber`: K/M thresholds, rounding, zero

**dashboard_test.go** (5 tests) — loadVencimientos logic via mock olu:
- Filters assets to 30-day window; rejects past dates; sorts ascending by
  urgency; checks both fecha_expiracion and fin_garantia per asset; caps at 10

**csrf_test.go** (5 tests) — CSRF middleware behaviour:
- Missing token → 403; wrong token → 403; matching token → handler executes;
  nil CSRFManager → POST proceeds; GET → never checked
- Uses `csrfProtect` wrapper that mirrors the CSRF branch inside requireAuth,
  isolating the validation logic from session management

**masters_handler_test.go** (6 tests) — master CRUD HTTP semantics:
- RegionCreate success → 303 + Location: /regiones
- RegionDelete ID=0 → 404
- RegionDelete conflict (count > 0) → 409
- RegionDelete success → 303, olu DELETE called
- MarcaDelete conflict → 409
- PropDefDelete conflict → 409

**alert_asset_handler_test.go** (5 tests) — alert, asset, dashboard handlers:
- AlertAcknowledge success → 303 + Location: /alerts/1, olu PUT called
- AlertAcknowledge ID=0 → 404
- AssetsList criticidad filter → OQL query contains `criticidad = 'Alta'`
- AssetsList status filter → OQL query contains `status = 'maintenance'`
- Dashboard zero state → 200, non-empty body, no panic

All tests use `mockOLU` / `httptest.Server` pattern consistent with
`internal/handlers/api`. No HTML string matching; all assertions on HTTP
status codes, redirect targets, olu call verification, and query contents.

## [0.1.0-patched154] - 2026-03-30

### Fixed — bugs identified during development review

- CSRF tokens on master table delete buttons: all ten list data structs now
  carry `CSRFToken`; list handlers populate it; `masterDeleteButton` threads
  it into the hidden form field. Deletes previously silently failed or 403'd
  when CSRF middleware was strict.
- `LinksCumodato` typo in `AssetFormData` corrected to `LinksComodato`.
  Edit mode now correctly pre-populates comodato document links.
- `criticidad` filter wired into `AssetsList`: `?criticidad=Alta` (and
  Media/Baja) now filters the OQL query. Dashboard link was already correct.
- Resolve/Dismiss dead-end buttons removed from alert detail actions bar.
  Replaced with a `Volver a Alertas` back link. Those workflows are deferred.

### Changed — asset list table

- Columns updated: Activo, Tipo, GIAI, Estado, Criticidad, Custodio, Acciones.
- Connectivity and Last Event columns removed (IoT pipeline data, not
  operational identity data).
- Criticidad rendered in colour (red/yellow/green).
- Custodio name resolved from `custodios` master table via a pre-query lookup.
- GIAI rendered in monospace for scanning legibility.
- Handler pre-fetches custodio names map before assembling rows.

## [0.1.0-patched153] - 2026-03-30

### Changed — Dashboard redesign (operational focus)

- Dashboard rebuilt around three questions: anything urgent? fleet state?
  anything expiring soon?
- Top row: three stat cards — Alertas Abiertas (red border when non-zero),
  En Mantenimiento, Criticidad Alta (red border when non-zero). All three
  are clickable links to filtered list views.
- Alertas Recientes panel: last 5 open alerts from the alerts entity with
  asset name, alerta tipo, criticidad badge (colour-coded), timestamp, and
  direct link to the alert detail page. Shows a green check when clear.
- Vencimientos Próximos panel: assets with fecha_expiracion or fin_garantia
  within 30 days, sorted ascending by days left. Days ≤7 shown in red with
  exclamation, ≤14 in yellow. Up to 10 rows. Asset names link to edit page.
- Below fold: total assets count + link to list, and status breakdown
  (Activo/Inactivo/Mantenimiento/Dado de baja) with colour dots.
- Removed: events today stat, recent events panel (IoT pipeline concern,
  not shelf-ui concern), alert status count (was always zero — wrong field).
- `loadVencimientos` helper: fetches assets with expiration fields set,
  parses dates in Go, filters to 30-day window, insertion-sorts by urgency.

## [0.1.0-patched152] - 2026-03-30

### Added — Wave 3 + navigation (P-19, P-22, P-23, nav)

- Navigation sidebar restructured into four labelled sections:
  Operacional, Configuración, Maestros (all 9 master tables), Administración.
  `NavItem.Section bool` field added; `navItem` renders section headers as
  non-link dividers. All master table paths now reachable from the sidebar.
- P-19 property value rendering: `LoadPropValues` loads prop_associations for
  an asset type, resolves current values from entity `prop_values` map, returns
  `[]PropValueEntry`. `PropValuesSection` + `propValueField` render each
  property with the correct input type (text, number, date, datetime-local, time,
  checkbox). Section rendered in `AssetFormPage` before the submit button.
  `AssetCreate` and `AssetUpdate` collect `prop_values[nombre_corto]` form
  fields and persist them as a JSON map on the asset entity.
- P-22 alert detail expanded: `AlertDetailData` extended with AssetGIAI,
  AssetRegion, AssetCustodio, AlertaConfig, AlertaTipo, AlertaCriticidad.
  `AlertDetail` handler resolves region name, custodio name, and alerta master
  config from olu. `AlertDetailPage` renders asset context grid and alerta
  config pill inline.
- P-23 alert acknowledgement: `POST /alerts/{id}/acknowledge` route +
  `AlertAcknowledge` handler updates alert status, appends `alert_history`
  entry with user email and optional notes. Acknowledge form rendered inline in
  `AlertDetailPage` when `status == "open"`. `nowISO()` helper added.

## [0.1.0-patched151] - 2026-03-30

### Added — Wave 2 asset form expansion (P-9 through P-15, P-20)

- `AssetDetail` struct expanded with all seven spec blocks: B (marca, modelo,
  serie, lote, criticidad), C (ubicacion_fisica, region_id, id_padre), D (custodio,
  calidad_tenencia, vto_comodato, links_comodato), E (lifecycle dates + links),
  F (temp range, refrigerante, voltaje, service_configs), G (precio, moneda,
  precio_actual, centro_costo, fin_garantia, links_documentos)
- `AssetFormData` expanded with dropdown datasets for all FK fields
- `AssetFormPage` extended with Cards for blocks A through G; each block uses
  the correct input type (date, number, select, DocumentLinksField)
- Block F service_configs rendered as repeating sub-form with inline JS add/remove
- `AssetCreate` and `AssetUpdate` handlers read and persist all new fields;
  service_configs assembled from multi-value form arrays; document link lists
  converted to JSON-compatible `[]any` for olu storage
- `assetFKSelect`, `assetEnumSelect`, `serviceConfigsField` — new form
  helpers in asset_detail.go
- Dropdown loaders: `loadModelos`, `loadUbicaciones`, `loadCustodiosSimple`,
  `loadMonedasSimple`, `loadCentrosCosto` added to masters_routes.go
- `parseDocumentLinksToAny`, `toFloatFromString` helpers
- Removed stale `location` field from AssetCreate (superseded by Block C)

## [0.1.0-patched150] - 2026-03-30

### Added — Wave 1 (P-5, P-6, P-13, P-16, P-18, P-21)

- P-5 Modelo — CRUD with Marca cascade dropdown; brand name resolved in list view
- P-6 Ubicación — CRUD with Región dropdown; region name resolved in list view
- P-16 DocumentLinksField — reusable add/remove document link component with
  inline JS; `parseDocumentLinks` helper for form handlers
- P-13 Block E — lifecycle date fields on `AssetDetail` struct and entity
  population; `links_ciclo_vida` JSON array parsed from olu
- P-18 prop_associations — CRUD (list, new, create, delete) with
  prop_definitions dropdown; `obligatoriedad` enum (requerida/opcional/por_regla)
- P-21 Alert config on Tipo de Activo — `default_alerta_id` field wired into
  asset type create/edit/update handlers and form page with Alertas dropdown
- `loadAlertas`, `loadMarcas`, `loadRegiones`, `loadPropDefs` — shared olu
  query helpers for dropdown population
- 24 additional routes registered in the admin block of router.go

## [0.1.0-patched149] - 2026-03-30

### Added — Wave 0 master tables (P-1, P-2, P-3, P-4, P-7, P-8, P-17)

- `internal/ui/masters_routes.go` — full CRUD handlers for all seven
  Wave 0 master entities: Región, Moneda, Centro de Costo, Marca,
  Custodio, Alerta, PropDefinition (~520 lines)
- `internal/ui/masters_pages.go` — minty page rendering for all list
  and create/edit forms; shared helpers (masterListHeader, masterFormInput,
  masterFormSelect, masterFormError, masterFormButtons, masterDeleteButton,
  masterTableWrap, masterTH, masterTD) (~640 lines)
- `internal/validation/schemas.go` — validation schemas for all seven
  master entities (Create and Update variants each)
- `internal/ui/router.go` — 42 new routes registered in the admin
  block under `PermSettingsUpdate`; list routes are `requireAuth` only
- Delete guards on all masters: referential integrity checked via OQL
  COUNT before deletion; conflicts return HTTP 409
- `prop_definitions` enforces `nombre_corto` uniqueness per tenant
  via OQL check at create and update time
- `api_externa` property type rendered as disabled in the form selector

## [0.1.0-patched148] - 2026-03-30

### Changed
- Module path renamed from `github.com/ha1tch/shelf` to `github.com/ha1tch/shelf-ui`
- Licence changed from MIT to Apache 2.0
- go.mod go directive: 1.22.2 (build environment; Dockerfile uses golang:1.23-alpine)

### Removed
- `internal/eventbus/` — dead in-process pub/sub bus, never instantiated
- `internal/webhook/adapter.go` — dead eventbus-to-webhook bridge
- `internal/tenant/` — SaaS billing dead code, never imported
- `internal/connectivity/connectivity.go` — never imported outside its own package
- `internal/notify/notify.go` — never imported outside its own package
- Pipeline execution hooks from `internal/handlers/api/events.go`:
  `processEvent`, `evaluateFSM`, `evaluateRules`, goroutine launch in `Ingest` and `IngestBatch`
- `FSMEvaluator` and `RuleEngine` fields from `EventHandler` and `RouterConfig`

### Added
- `SHELF_SESSION_SECRET` env var binding in `config.go`
- `SHELF_API_URL` env var binding in `config.go` (runtime state fetch from shelf-api)
- `SHELF_ENABLE_OPERATIONAL` and `SHELF_ENABLE_ADMIN` feature flags (default: true)
- `EnableOperational`, `EnableAdmin`, `ShelfAPIURL` fields on `ui.Config`
- Operational/admin route gating in `ui/router.go`
- `giai`, `grai`, `codigo_barras_rfid`, `tag_tecnico` fields on asset model, schemas, and `AssetDetail` struct
- `fetchRuntimeState` in `ui/routes.go`: fetches FSM state from shelf-api when GIAI is set
- `Dockerfile`: `golang:1.23-alpine` builder, `alpine:3.19` runtime
- `.github/workflows/ci.yml`: build + test on push/PR
- `.github/workflows/release.yml`: Docker image pushed to `ghcr.io/ha1tch/shelf-ui`

### Fixed
- `/oql/commit` URL corrected to `/commit` in `internal/olu/client.go`
- `client_test.go` updated to assert corrected URL
- `events_test.go` updated for simplified `NewEventHandler` signature
- `FSMHandler` and `RuleHandler` now instantiate their own evaluator/engine internally


---

## [0.1.0-patched147] - 2026-03-17

### Added — Add-linked-node gesture; node grow animation

**Add-linked-node (right-click halo)**
Right-clicking the halo ring on a node starts a new "add linked node" drag.
The temporary link behaves identically to the normal connect-link drag, except:
- A ghost node (indigo stroke, node-fill interior) is drawn at the arrowhead
  during the drag to preview where the new node will land.
- On drop over empty canvas: a new `Node` is created at the drop position,
  a `Link` from the origin node to the new node is committed, and the new
  node grows in with the animated entrance (see below).
- On drop over an existing node: falls back to normal link commit — no new
  node is created.

Left-click halo retains the original connect-link behaviour unchanged.

**Node grow animation**
`_startNodeGrow(node)` runs a 320ms cubic ease-out RAF loop. While active,
`Node.draw` scales the node radius from 0 to full size: `r * (1-(1-t)³)`.
Applied to:
- New nodes created by the add-linked-node gesture.
- New nodes created by double-click.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched146] - 2026-03-17

### Removed — Rete.js rules editor

**Deleted files:**
- `web/static/js/rules-editor.js` (880 lines, Rete.js node-graph editor)
- `web/static/js/fsm-graphview.js` (Rete custom node/connection factories)

**Removed Go code:**
- `RulesEditorNew`, `RulesEditorEdit` handlers (`routes.go`)
- `RulesEditorData`, `RulesEditorPage` (`pages.go`)
- `GET /rules/editor`, `GET /rules/editor/{id}` routes (`router.go`)
- `reteJSEntry`, `reteJSDeps`, `ReteImportMap` (`layout.go`)
- "Rules Editor" nav item (`layout.go`)

**Retained:**
- `/rules` CRUD pages (list, new, edit) — unaffected
- Lit import for the FSM editor shell — `ReteImportMap` replaced with
  `LitImportMap`, a minimal single-entry import map (`{"lit": "..."}`)

The importmap dependency chain drops from 8 CDN packages (rete, rete-area-
plugin, rete-connection-plugin, rete-render-utils, @retejs/lit-plugin,
rete-connection-path-plugin, d3-shape, lit) to 1 (lit only).

**Files changed:**
- `internal/ui/layout.go`
- `internal/ui/pages.go`
- `internal/ui/routes.go`
- `internal/ui/router.go`
- `web/static/js/shelf-fsm-editor.js` (stale comment removed)

---

## [0.1.0-patched145] - 2026-03-16

### Changed — Group halo pulse faster (0.0045 → 0.007)

Approximately 0.72 Hz → 1.1 Hz.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched144] - 2026-03-16

### Fixed — Group selection: right-drag moves group; stationary click deactivates

**Right-drag on a selected node now moves the group.**
The `e.button !== 2` guard on the group-move hit test was preventing right-
button mousedown on a selected node from triggering group movement — it fell
through to the rubber-band path instead. Guard removed; any button click on
a selected node starts the group drag.

**Stationary click deactivates group selection.**
On any mouseup where `!didDrag`, `_selectedNodes` is cleared and the halo
RAF loop stops. This covers left-click, right-click, and Shift+click on
anything that isn't a drag.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched143] - 2026-03-16

### Fixed — Dialog zoom scale 70%; animation fires on first open

Scale changed from 88% to 70% for a more pronounced zoom effect.

First-open animation fix: a single `requestAnimationFrame` can fire before
the browser has composited the initial `opacity:0, scale(0.70)` styles,
giving the transition nothing to animate from. Two nested `rAF` calls
guarantee the initial state is painted before the open transition fires.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched142] - 2026-03-16

### Added — Group selection with rubber band and pulsating halo

**Gesture:** Right-drag on empty canvas draws a rubber-band rectangle. On
mouseup, all nodes whose centres fall within the rectangle are added to
`_selectedNodes`. Right-click on a link still fires the properties dialog
(click vs drag distinguished by `_hasDragged`). Browser context menu
suppressed on the canvas.

**Visual feedback — rubber band:** Dashed indigo rectangle with 7% fill,
drawn in screen space above all other elements while dragging.

**Visual feedback — group halo:** Each selected node gets a pulsating indigo
ring (radius = node radius + 10px, 8px stroke, ~22% base opacity). The pulse
is a continuous sine wave at ~0.45Hz driven by `_startGroupHalo()` RAF loop.
The loop stops automatically when `_selectedNodes` is cleared.

**Group drag:** Mousedown on any node in `_selectedNodes` sets `movingObject`
true with `selectedObject = null`. The mousemove handler detects this and
calls `setAnchorPoint` on all selected nodes simultaneously. Links follow
their endpoint nodes geometrically as usual — no extra logic needed.

**Clearing the selection:** Any left-click that doesn't land on a selected
node clears `_selectedNodes`. Clearing a canvas (`clearCanvas`) also resets
group state.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched141] - 2026-03-16

### Changed — Dialog animation: zoom instead of slide

Modal opens with `scale(0.88) → scale(1)` and closes with `scale(1) → scale(0.88)`.
Backdrop opacity transition unchanged.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched140] - 2026-03-16

### Added — Smooth open/close animation for link properties dialog

`showFSMDialog` now animates in both directions:

**Open:** backdrop fades from `rgba(0,0,0,0)` → `rgba(0,0,0,0.45)` (220ms).
Modal slides up from `translateY(16px), opacity:0` → `translateY(0), opacity:1`
(200ms ease). Triggered via `requestAnimationFrame` after DOM insertion so
the browser has painted the initial invisible state first.

**Close:** `_dismiss(result)` reverses both transitions, then removes the
backdrop from the DOM after 220ms. All three dismiss paths (✕ button, Cancel,
Save, Escape key) use `_dismiss`.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched139] - 2026-03-16

### Fixed — LaTeX: above-U+00FF chars substituted directly; backspace deletes

**Unicode substitution fixed**
`_buildUnicodeDeclarations` and the non-Latin1 strip were fighting:
declarations for Δ (U+0394) were generated in the preamble, but the
character was replaced with `?` in the text body before reaching LaTeX.

Fix: the blanket `.replace(/[^\x00-\xFF]/g, '?')` is replaced with a
function that looks up `this._unicodeMap[codepoint]` and substitutes the
LaTeX equivalent directly in the text body. Unknown chars still become `?`.
`_buildUnicodeDeclarations` is now restricted to U+0080–U+00FF only (Latin-1
supplement, which inputenc handles natively from the UTF-8 bytes; declarations
needed for those). Above-U+00FF chars are now fully substituted in the body
and need no preamble declarations.

**Double quote → single quote** in label text.

**Backspace as Delete**
When not in label-edit mode, Backspace now deletes the selected node or link,
identical to the Delete key. In label-edit mode behaviour is unchanged
(delete character before caret).

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched138] - 2026-03-16

### Fixed — LaTeX: < and > rendered as wrong glyphs in text mode

In pdflatex text mode, `<` and `>` produce `¡` and `¿` respectively (they
are valid in math mode but not text mode). All label text now escapes these
as `\textless{}` and `\textgreater{}`.

Analysis of all preset label characters: °, ×, Δ, ≈ are all covered by the
Unicode map (patched136). No other non-ASCII characters appear in the
built-in presets.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched137] - 2026-03-16

### Fixed — PNG export CSRF failure: body consumed by FormValue

`requireAuth` validates CSRF by calling `r.csrfManager.ValidateToken(req)`,
which internally calls `r.FormValue(CSRFFormField)`. On a POST with
`Content-Type: image/svg+xml`, Go's `ParseForm` finds no form body, tries to
parse the raw body as a form anyway, and **discards it**. By the time
`FSMExportPNG` calls `io.ReadAll(r.Body)` the body is empty, and the SVG
never reaches `resvg`. Additionally, if `FormValue` drains the body before
checking the header, the CSRF token read from `X-CSRF-Token` may also fail
depending on Go version behaviour.

Fix: new `requireAuthRawBody` middleware that validates CSRF exclusively from
the `X-CSRF-Token` header using `subtle.ConstantTimeCompare` directly —
never calling `ParseForm` or `FormValue`. The PNG export route is now
registered with this wrapper instead of `requireAuth`.

**Files changed:**
- `internal/ui/router.go`

---

## [0.1.0-patched136] - 2026-03-16

### Added — LaTeX Unicode map extended: Greek, subscripts, superscripts, math

Added to `_unicodeMap`:

- **Greek uppercase** (U+0391–U+03A9): Α–Ω mapped to `\ensuremath` forms
- **Greek lowercase** (U+03B1–U+03C9): α–ω including variants (ε→\varepsilon, φ→\varphi, ς→\varsigma)
- **Unicode subscript digits** (U+2080–U+2089): ₀–₉ → `\textsubscript{N}`
- **Unicode superscript digits** (U+2070, U+2074–U+2079): ⁰,⁴–⁹ → `\textsuperscript{N}` (¹²³ were already in Latin-1)
- **Mathematical operators**: ∀ ∃ ∄ ∅ ∇ ∈ ∉ ∋ ∏ ∑ − ∓ ∕ ∗ √ ∛ ∞ ∂ ∠ ∡ ∥ ∦ ∧ ∨ ∩ ∪ ∫ ∬ ∶ ∷ ∼ ≃ ≅ ≈ ≍ ≐ ≠ ≡ ≢ ≤ ≥ ≪ ≫ ⊂ ⊃ ⊆ ⊇ ⊥ ⋅ ⋯
- **Arrows**: ← ↑ → ↓ ↔ ↕ ⇐ ⇑ ⇒ ⇓ ⇔ ↦ ↩
- **Miscellaneous technical**: Ω (ohm U+2126), Å (U+212B), ′ ″ …
- **Typography**: – — "" ''

Characters above U+00FF that are not in this map still fall back to `?`.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched135] - 2026-03-16

### Fixed — SVG/PNG export padding increased to avoid clipping

`getGeometryBounds` padding: 40px → 60px. Arrowheads and link labels
on peripheral nodes were extending beyond the export bounds. 60px
matches the padding used by `zoomToFit`.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched134] - 2026-03-16

### Changed — LaTeX line width 1.5pt → 1.2pt

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched133] - 2026-03-16

### Fixed — LaTeX Unicode map replaced with complete Latin-1 Supplement

The previous partial map (~80 entries covering Greek, arrows, and selected
math symbols) is replaced with a complete mapping of the Latin-1 Supplement
block (U+00A0–U+00FF, 96 entries). This is the ANSI subset — every
non-ASCII character a standard keyboard or typical technical label would
produce. Coverage is now exhaustive within that range rather than
aspirational.

The scan threshold is corrected from `cp > 0xFF` to `cp > 0x7F` so
characters in the U+0080–U+00FF range (which are non-ASCII) are actually
detected and declared. Previously the scan would miss them entirely.

Characters above U+00FF still fall back to `?`.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched132] - 2026-03-16

### Added — LaTeX: dynamic Unicode character declarations from label scan

`ExportAsLaTeX` now scans all node labels, link labels, guard, and action
fields before rendering. For each unique Unicode codepoint above U+00FF it
emits a `\DeclareUnicodeCharacter{XXXX}{...}` line in the preamble — only
for characters actually present in the diagram.

**Coverage table** (`_unicodeMap`):
- Greek uppercase and lowercase (Α–Ω, α–ω)
- Math operators: ±, ×, ÷, ≈, ≠, ≡, ≤, ≥, ∞, ∂, ∑, ∏, √, ∫, ∅, ∈, ∉, ∩, ∪, ∧, ∨, ¬
- Arrows: ←, →, ↑, ↓, ↔, ⇒, ⇐, ⇔
- Technical/unit: °, µ, Ω (ohm), Å, ², ³, ¹
- Typography: …, –, —, "", ''

Characters outside the map fall through to the existing `?` substitution.
A diagram using only ASCII generates no `\DeclareUnicodeCharacter` lines.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched131] - 2026-03-16

### Fixed — LaTeX: auto-scale to page width; Unicode fallback

**Page width fit**
`\resizebox{\textwidth}{!}{...}` wraps the `tikzpicture`. The diagram
scales to exactly the text width regardless of how large the canvas
coordinates are. The `scale=0.2` coordinate factor and `1.5pt` line width
stay unchanged — `\resizebox` scales the rendered output uniformly.

**Unicode fallback**
Non-Latin1 characters (codepoints > U+00FF) that survive the subscript/
superscript replacement are now replaced with `?` before LaTeX escaping.
This prevents pdflatex from aborting on stray Unicode like degree symbols,
arrows, or any other label character outside the inputenc Latin-1 range.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched130] - 2026-03-16

### Changed — Zoom inertia lasts 10% longer

Decay coefficient: `0.78` → `0.801`. Derived as `0.78^(1/1.1)` so the
velocity envelope decays over ~10% more frames before the threshold cutoff.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched129] - 2026-03-16

### Changed — LaTeX label font size reduced another 10%

6.65pt → 5.99pt (leading 8pt → 7.2pt).

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched128] - 2026-03-16

### Changed — LaTeX label font size reduced 5%

`\scriptsize` (7pt) replaced with `\fontsize{6.65}{8}\selectfont` (6.65pt).
No standard LaTeX size command falls between `\scriptsize` and `\tiny`,
so an explicit `\fontsize` is used for the precise reduction requested.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched127] - 2026-03-16

### Fixed — _tikzLineWidth regression: line width=undefined

The Python line-replacement patcher in patched126 consumed the `var lw` line
correctly but then wrote it back *twice* and dropped the `return` statement
entirely. `_tikzLineWidth()` returned `undefined`, producing
`line width=undefined` in every \draw command.

Replaced the broken body with a clean one-liner: `return '1.5pt';`.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched126] - 2026-03-16

### Fixed — LaTeX: line width fixed at 1.5pt; smaller font; Unicode sub/superscripts

**Line width**
`_tikzLineWidth()` now returns the fixed string `'1.5pt'`. The computed
formula (`lw * 0.0569 * 1.8`) from patched124 had not landed correctly and
was producing ~0.15pt strokes, thinner than TikZ's own default.

**Font size**
`\footnotesize` → `\scriptsize` (7pt at 10pt base — approximately 12%
smaller than `\footnotesize`'s 8pt, close to the requested 10%).

**Unicode subscripts/superscripts**
Unicode subscript digits ₀–₉ (U+2080–U+2089) and superscript digits ⁰–⁹
are now replaced with `\textsubscript{N}` and `\textsuperscript{N}` before
LaTeX escaping. These work in text mode without extra packages.
`\usepackage[utf8]{inputenc}` added to preamble for general Unicode coverage
in pdflatex.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched125] - 2026-03-16

### Fixed — sansmathfonts/fontenc actually removed from generated .tex

The patched124 fix for removing `sansmathfonts` and `fontenc` did not land
due to a quoting mismatch in the Python patcher. Applied directly in this
patch. The generated `.tex` preamble is now:

    \documentclass[10pt]{article}
    \usepackage{tikz}
    \renewcommand{\familydefault}{\sfdefault}

No packages beyond tikz required.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched124] - 2026-03-16

### Fixed — LaTeX line widths, font size, remove sansmathfonts dependency

- Line width: `_tikzLineWidth()` now returns the fixed string `'1.5pt'`
  instead of computing from `canvas.lineWidth`. Previous computed values
  (~0.15pt) were thinner than the pre-patch TikZ default; 1.5pt gives
  visually heavier strokes as intended.
- Font size: `\small` → `\footnotesize`. At 10pt base this gives ~8pt
  (~11% smaller than `\small`'s 9pt, close to the requested 7%).
- Removed `\usepackage{sansmathfonts}` and `\usepackage[T1]{fontenc}`.
  `\renewcommand{\familydefault}{\sfdefault}` is kept — it works without
  those packages and selects the system default sans-serif (typically
  Latin Modern Sans or Helvetica depending on the TeX installation).

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched123] - 2026-03-16

### Fixed — LaTeX rendering: line weights, label colour, font size, font style

**1. Line widths**
Added `_tikzLineWidth()` helper that converts `this.lineWidth` (canvas px)
to pt at the compound scale (×0.1 _scale × 0.2 tikzpicture × 1.8 boost).
All `\draw` path/arc/stroke commands now emit `line width=<N>pt` inline.
`\fill` (arrowheads) keeps no line width as it is solid fill.

**2. Label colour**
`advancedFillText` now always emits `\draw [black]` for node labels
regardless of monochrome mode. Previously it used `_tikzColor(strokeStyle)`
which passed the indigo/grey node stroke colour through to the text.

**3. Font size**
Document class changed from `[12pt]` to `[10pt]`. Label nodes wrapped in
`\small` which gives ~8.5pt — approximately 30% smaller than the previous
12pt math mode output.

**4. Font style**
Math mode (`$...$`) and `\mbox{ }` space hack removed. Labels now emit as:
`{\small\sffamily\upshape <text>}` — sans-serif, upright, no italic.
Special LaTeX characters in label text are now escaped (backslash, braces,
underscore, caret, ampersand, percent, dollar, hash, tilde).

**Packages added to preamble:**
`sansmathfonts` and `fontenc[T1]` with `\renewcommand{\familydefault}{\sfdefault}`
ensure the entire document uses sans-serif, so any math fragments or
fallback text also render consistently.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched122] - 2026-03-16

### Fixed/Added — LaTeX colour export; monochrome via Shift+click

**Broken `advancedFillText` closing braces fixed**
A previous failed patch left `advancedFillText` and `ExportAsLaTeX` without
their closing `};` and `}` delimiters. The entire block is now well-formed.

**Node text draw uses `_tikzColor`**
`advancedFillText` now emits `\draw [<colour>] ... node ...` instead of
a bare `\draw ... node ...`, so node labels carry the correct colour in
colour mode.

**Monochrome vs colour export**
- `ExportAsLaTeX(bounds, monochrome)` — boolean second argument.
  When `true`, `_tikzColor` always returns `'black'`.
  When `false` (default), CSS hex is converted to TikZ inline RGB.
- `saveAsLaTeX(monochrome)` passes the flag through; downloads as
  `fsm-diagram-mono.tex` or `fsm-diagram.tex` accordingly.
- Lit shell `_exportLaTeX(e)` passes `e.shiftKey` to `saveAsLaTeX`.
  Normal click → colour; Shift+click → monochrome.
- Button tooltip updated: "Export as LaTeX / TikZ (Shift for monochrome)".

**Palette coverage**
All live `strokeStyle` values are `#rrggbb` hex and are handled by the
existing `_tikzColor` regex. This includes: node indigo (#6366f1/#818cf8),
link grey (#64748b/#94a3b8), accept ring Irish green (#169B62), and the
link cycle colours aubergine (#4b0082) and maroon (#8b0000).

**Files changed:**
- `web/static/js/shelf-fsm-editor.js`
- `web/static/js/fsm-editor.js`

---

## [0.1.0-patched121] - 2026-03-16

### Fixed — LaTeX export: CSS hex colours converted to TikZ RGB syntax

All `\draw` and `\fill` commands in `ExportAsLaTeX` were emitting
`this.strokeStyle` verbatim — e.g. `\draw [#6366f1]`. TikZ interprets `#`
as a LaTeX parameter character, causing cascading `Illegal parameter number`
errors on every path.

Added `_tikzColor(css)` helper method inside `ExportAsLaTeX`:
- Named colours (`'black'`, `'white'`) pass through unchanged
- CSS hex `#rrggbb` is converted to TikZ inline RGB:
  `color={rgb,255:red,R;green,G;blue,B}` — works without any extra packages
- Anything else falls back to `black`

All four draw commands (`arc`, `stroke`, `fill`, and the `advancedFillText`
node draw) now call `this._tikzColor(this.strokeStyle)` instead of
using `this.strokeStyle` directly.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched120] - 2026-03-16

### Fixed — Accept ring above edit box (for real this time)

Two-part fix:

1. In `Node.draw` (main pass): the accept ring is now skipped when
   `_suppressEditBox` is true and the node is selected for editing. Previously
   it drew after `drawText` returned early, so the ring appeared in the main
   pass regardless.

2. In the edit overlay pass: `selectedObject.draw(c)` is replaced with a
   direct `drawText` call followed by an explicit accept ring arc. Draw order
   is: edit box rect → text → caret → ring. The ring is above the edit box
   border but the edit box background (97% opaque) covers the portion of the
   ring that falls within the text area, and the ring extends visibly beyond
   it. No node circle or other geometry is redrawn.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched119] - 2026-03-16

### Fixed — Link colour immediate; LaTeX scale error

**Link colour not immediately visible**
After Shift+click the link remained `selectedObject`, so `drawUsing` drew it
in `'blue'` (selected colour), masking the custom colour until the next
deselect. Added `selectedObject = null` before `draw()` in the colour cycle
block — colour is now visible immediately.

**ExportAsLaTeX `c.scale is not a function`**
`this.scale` was still missing from `ExportAsLaTeX`'s no-op stub list
(the patched117 fix silently failed). Added alongside the other stubs.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched118] - 2026-03-16

### Fixed — Complete missing patches from patched117; aubergine replaces prune

Several patches from patched117 silently failed (the `str_replace` targets
had already been partially modified by earlier patches in the same run,
making the exact strings unfindable). Applied cleanly in this pass:

- `linkColors` Map declaration added
- `clearCanvas` clears `linkColors`
- `drawUsing` link loop reads `linkColors` for custom colour
- `drawUsing` currentLink draws in accent when latched, pulsating grey when not
- `mouseup` Shift+click on link cycles colour; Shift prevents label edit

Colour cycle: Irish green → **aubergine** (`#4b0082`) → maroon (`#8b0000`) → default.
Prune (`#6b2737`) replaced with aubergine per request.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched117] - 2026-03-16

### Fixed/Added — Four issues in one pass

**ExportAsLaTeX missing `scale`**
`this.scale` stub added to `ExportAsLaTeX` alongside the existing no-op
stubs. `drawUsing` calls `c.scale(k, k)` and threw at the LaTeX export path.

**Green accept-state ring above edit box**
The overlay pass was calling `selectedObject.draw(c)` which redraws the
full node including the accept-state ring on top of everything. Changed to
call `drawText(c, selectedObject.text, x, y, null, true)` directly — only
the label, edit rect, and caret are drawn in the overlay.

**Latched link shown in accent colour**
In `drawUsing`, when `currentLink` is not a `TemporaryLink` (i.e. it has
snapped onto a target node and become a `Link`, `SelfLink`, or `StartLink`),
it is now drawn in `shelfAccentColor` (Irish green). When the mouse moves
off the target and reverts to `TemporaryLink`, the pulsating grey style
resumes. On mouseup commit the link becomes a permanent link at default colour.

**Shift+click on link cycles colour**
`linkColors` Map (keyed by `link._id`) stores per-link custom colours.
Shift+click on a committed link cycles through:
Irish green (`shelfAccentColor`) → prune (`#6b2737`) → maroon (`#8b0000`) → default (null).
`null` removes the entry so the link returns to `shelfLinkStrokeColor`.
Shift+click on a link no longer enters label edit mode.
Colours are persisted in `getBackupData`/`restoreFromBackupData` as a
`color` field alongside `guard` and `action`. `clearCanvas` clears the map.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched116] - 2026-03-16

### Added — JS syntax check in release.sh

`release.sh` now runs `node --check` on `shelf-fsm-editor.js` and
`fsm-editor.js` after the Go build step. A syntax error in either file
fails the release immediately with a clear message. Node.js v22 is
permanently available in the build container at `/usr/bin/node`.

**Files changed:** `release.sh`

---

## [0.1.0-patched115] - 2026-03-16

### Fixed — Missing `async _exportPNG` declaration (syntax error)

When `_exportLaTeX` was inserted before `_exportPNG`, the `async _exportPNG() {`
method declaration line was accidentally dropped, leaving the body of `_exportPNG`
as bare statements outside any method and causing an `Unexpected token 'typeof'`
syntax error at parse time.

Going forward, both JS files are checked with `node --check` before release.

**Files changed:** `web/static/js/fsm-editor.js`

---

## [0.1.0-patched114] - 2026-03-16

### Fixed — Remove dblclick accept-state toggle; edit box always on top

**Double-click accept-state toggle removed**
The `ondblclick` handler's `isAcceptState` toggle is gone. Double-clicking
a node now only creates a new node (when canvas is empty at that point) or
does nothing (when clicking an existing node). Accept state is toggled
exclusively via Shift+click.

**Edit box drawn above all other elements**
Previously the edit rect and caret were drawn inside `Node.draw`, which runs
during the main node loop — before links, halo, midpoint dots, and snap-back
animations. On cluttered diagrams the edit box was obscured by overlapping
elements drawn later.

Fix: `_suppressEditBox` global (default false). During `drawUsing` the main
node loop sets it to true, which causes `_drawTextMultiline` to return early
for the selected editing node without drawing the rect, text, or caret. After
`c.restore()`, a second pass draws only the selected editing node again with
`_suppressEditBox = false` and a fresh `c.save/translate/scale/restore` —
placing the edit box and text above every other element in the scene.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched113] - 2026-03-16

### Added — LaTeX export button; saveAsLaTeX now downloads a file

Toolbar gets a small button with a serifed capital **L** (Georgia, 15px bold)
and title "Export as LaTeX / TikZ". Calls `saveAsLaTeX()` via `_exportLaTeX`.

`saveAsLaTeX` fixed: previously called `output()` which wrote to `#output`,
a DOM element that doesn't exist in the Lit shell. Now builds a Blob and
downloads `fsm-diagram.tex` directly. Also clears halo/dot opacity before
drawing to the exporter (same discipline as `getSVGString`), and calls
`draw()` afterward to restore the visual.

**Files changed:**
- `web/static/js/shelf-fsm-editor.js`
- `web/static/js/fsm-editor.js`

---

## [0.1.0-patched112] - 2026-03-16

### Removed — Dead non-LaTeX code (116 lines)

Functions removed:
- `saveAsPNG` — Wallace's original PNG-via-data-URI; replaced by Go/resvg path
- `getBoundingRect` — pixel-scan bounding box; replaced by `getGeometryBounds`
- `downloadFile` — base64 data-URI download helper; no longer called
- `downloadSVGFile` — wrapper around `downloadFile`; no longer called
- `saveAsSVG` — old entry point calling `downloadSVGFile`; Lit shell uses
  `getSVGString` directly
- `saveAsJSON` — called `downloadFile`; Lit shell builds blob directly
- `jsonUploaded` — read from `#jsonUpload` DOM element; no such element
- `uploadJSON` — clicked `#jsonUpload`; no such element
- `restoreBackup` — read from `localStorage['fsm']`; was called from
  `window.onload` which was removed when we embedded in the Lit shell
- `saveBackup` — wrote to `localStorage['fsm']`; call was removed from
  `draw()` in early patching

`ExportAsLaTeX`, `saveAsLaTeX`, and `output()` retained — LaTeX export is
kept for potential use in other projects. `saveAsLaTeX` updated to call
`getGeometryBounds()` instead of the now-deleted `getBoundingRect()`.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched111] - 2026-03-16

### Fixed — JSON export crash: `lp.guard` on undefined

`var lp` was declared inside the `if(link instanceof SelfLink)` branch,
making it invisible to the `StartLink` and `Link` branches. Reading
`lp.guard` in those branches threw `TypeError: Cannot read properties of
undefined`. Hoisted `var lp` to before the if/else chain so all three
branches share the same declaration.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched110] - 2026-03-16

### Fixed — SVG/PNG export clips graph when zoomed in or nodes extend beyond canvas

`getSVGString` was calling `getBoundingRect()` which scans the canvas pixel
buffer for non-empty pixels. The canvas is a fixed physical size; when the
graph is larger than the canvas (nodes panned off-screen, or zoomed in so
nodes are clipped), those nodes produce no pixels and are excluded from the
bounds. The SVG/PNG then only contained what was visible in the canvas.

Fix: `getGeometryBounds()` computes bounds directly from `node.x/y` and
`node._r` (with a 40px pad for arrowheads and labels). This is independent of
canvas size, zoom, and pan — the full graph is always captured. The pixel-scan
`drawUsing(canvas.getContext('2d'))` call before export is also removed
(saving a redundant full redraw).

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched109] - 2026-03-16

### Changed — Link stroke colour changed to mid dark grey

`shelfLinkStrokeColor`: dark `#94a3b8` (slate-400), light `#64748b` (slate-500).

**Files changed:** `web/static/js/fsm-editor.js`

---

## [0.1.0-patched108] - 2026-03-16

### Fixed — JSON and SVG export clicks silently ignored

`el.click()` on a detached anchor element is silently dropped by some
browsers. All three export anchors now follow the correct pattern:
`appendChild → click → removeChild` before revoking any blob URL.

**Files changed:** `web/static/js/fsm-editor.js`

---

## [0.1.0-patched107] - 2026-03-16

### Added — SVG export button

New **SVG** toolbar button between JSON and PNG. Calls `getSVGString()`
(viewport reset, halo/dot cleared) and triggers a browser download of
`<fsmName>.svg`. No server round-trip required.

**Files changed:** `web/static/js/fsm-editor.js`

---

## [0.1.0-patched106] - 2026-03-16

### Fixed — PNG export CSRF rejection

`POST /fsm/export/png` was receiving no CSRF token, so the CSRF middleware
rejected the request and returned the full HTML error page, which the browser
then displayed as an alert.

Fix:
- `FSMEditorPage` now passes `csrf-token="%s"` as an attribute on the
  `<shelf-fsm-editor>` element (alongside `fsm-id` and `fsm-name`)
- `csrfToken` added as a Lit property with `attribute: 'csrf-token'`
- `_exportPNG` includes `'X-CSRF-Token': this.csrfToken` in the fetch headers,
  matching the `X-CSRF-Token` header name expected by the CSRF middleware

**Files changed:**
- `internal/ui/pages.go`
- `web/static/js/fsm-editor.js`

---

## [0.1.0-patched105] - 2026-03-16

### Changed — Shift+click rebind: toggle accept state; link creation via halo only

**Removed:** Shift+click on node → SelfLink creation (Wallace's original
link-start gesture). Shift+click on empty canvas → TemporaryLink creation.
Both are superseded by the halo affordance (mouse exits node boundary).

**Added:** Shift+click on a node toggles `isAcceptState`. Node deselects
immediately after toggle so no label edit follows. No animation delay.

**Removed:** `var shift = false` global and the `key == 16` (Shift key)
tracking in keydown/keyup handlers — no longer needed anywhere.

**Updated:** Toolbar hint text now reads "Shift-click: toggle accept state"
instead of "Shift-drag: add transition".

**Files changed:**
- `web/static/js/shelf-fsm-editor.js`
- `web/static/js/fsm-editor.js`

---

## [0.1.0-patched104] - 2026-03-16

### Changed — Link stroke colour shifted toward cyan

`shelfLinkStrokeColor`: dark `#6366f1` → `#4d8ef5`, light `#818cf8` → `#6aabf7`.
A modest hue shift (~25°) from indigo toward cyan-blue, keeping similar
saturation and lightness. Node stroke and selection colours unchanged.

**Files changed:** `web/static/js/fsm-editor.js`

---

## [0.1.0-patched103] - 2026-03-16

### Fixed — Orphaned duplicate preset code causing SyntaxError

The str_replace in patched102 replaced only the opening marker of the PRESETS
block, leaving the original five presets as orphaned unreachable code after
the new closing `};`. The parser hit `nodeRadius:` outside any expression
context and threw `Unexpected token ':'` at line 249.

Fixed by slicing the 87 orphaned lines out of fsm-editor.js.

**Files changed:** `web/static/js/fsm-editor.js`

---

## [0.1.0-patched102] - 2026-03-16

### Added — Three electronics-oriented demo presets

**6 · Battery Management System (bms)**
8 states: Power Off → Self-Test → Standby ↔ Charging ↔ Balancing,
Standby ↔ Discharging, Over-Temp (from Charging/Discharging), Fault Latch.
Guards on temperature (T > 45°C / T < 40°C), voltage thresholds (V_max,
V_min, 2.5V), SOC, and cell delta-voltage (ΔV 20mV/5mV). Watchdog self-link
on Standby. Hardware-reset arc from Fault back to Power Off.

**7 · I2C Master Controller (i2c)**
9 states: Bus Idle → Start Sent → Addr TX → Addr ACK Wait → Data Write /
Data Read → Stop Sent → Bus Idle. Error paths: Arb Lost (SDA mismatch during
Start), NACK Error (from Addr TX or Data Write). Self-link on Data Read for
multi-byte receive with ACK. Covers the complete I2C master transaction FSM.

**8 · PWM Brushless Motor Controller (pwm_motor)**
9 states: Fault Reset → Initialise → Stopped → Ramp Up → Running → Ramp
Down → Stopped. Side paths: Current Limit (recoverable, bidirectional with
Running), Stall Detect (from Running and Ramp Up), Fault (from Stall timeout
or critical overcurrent). Self-link on Running for PID speed adjustment tick.
Guards use electrical quantities (rpm, I_lim, V_bus).

All three presets use multi-line labels (\n) for guard conditions, making
use of the multi-line label editing introduced in earlier patches. A disabled
`<option>` separator divides the original five demos from the three new ones.

**Files changed:** `web/static/js/fsm-editor.js`

---

## [0.1.0-patched101] - 2026-03-16

### Removed — Slider dead code; getBoundingRect uses _r

`makeNodeTextOnly()` and `radiusChanged()` removed — both were dead code
targeting a `#rangeSlider` DOM element that does not exist in the embedded
Lit shell layout. Node sizing is controlled by default radius (30px) and the
auto-size toolbar toggle only.

`clearCanvas` no longer attempts to reset a non-existent slider.

`getBoundingRect`/`zoomToFit` now uses `node._r ?? nodeRadius` so
auto-sized nodes are properly included in the bounding box used for fit-to-
screen and SVG/PNG export bounds.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched100] - 2026-03-16

### Added/Fixed — SVG exporter overhaul + resvg PNG export

**SVG exporter fixes**

`ExportAsSVG.scale()` added as no-op — previously `c.scale(viewport.k, k)`
threw "not a function" on any SVG/PNG export attempt.

`ExportAsSVG.advancedFillText()` added — previous SVG export fell through to
`fillText()` which emitted "Times New Roman" at size 20. Now emits SVG
`<text>` elements with:
- `font-size` parsed from `shelfFont` (e.g. 12 from "12px system-ui, ...")
- `font-family="sans-serif"` (SVG-safe approximation)
- `text-anchor="middle"` (labels are centred in both axes)
- `fill` from `shelfActiveLabelColor` (set by `drawUsing` before each `drawText`)

This fixes multi-line labels and auto-sized node labels in SVG/PNG output.

**`getSVGString()` — viewport-aware SVG export**

New function. Before rendering:
- Saves and resets `viewport` to `{x:0, y:0, k:1}` so world coordinates
  are used (previously, pan/zoom affected which part of the diagram appeared)
- Saves and zeroes `_haloOpacity` and `_dotOpacity` (transient UI elements
  must not appear in exports)
- Draws to the real canvas to get correct `getBoundingRect()` pixel scan
- Renders to `ExportAsSVG`, retrieves SVG string
- Restores all state and calls `draw()`

`saveAsSVG()` now calls `getSVGString()`.

**PNG export via resvg — `POST /fsm/export/png`**

New Go handler `FSMExportPNG` in `internal/ui/export.go`:
- Reads SVG body (10 MB limit)
- Locates `resvg` via `exec.LookPath`; returns 503 with install instructions
  if not found
- Writes SVG to temp dir, runs `resvg --background white`, reads PNG back
- Returns `image/png` with `Content-Disposition: attachment`
- Temp dir cleaned up via `defer os.RemoveAll`

Lit shell `_exportPNG()` method: calls `getSVGString()`, POSTs to
`/fsm/export/png`, triggers browser download on success, shows `alert()` on
error. Toolbar gets a new **PNG** button (image icon) alongside the JSON
export button.

Route registered: `POST /fsm/export/png` (requireAuth).

**Known limitation**: node interiors are not filled in SVG output (the canvas
`c.fill()` call after `c.arc()` is a no-op in the SVG exporter because arc
emits inline, not via `_points`). `resvg --background white` mitigates this
for PNG — nodes appear white. A future pass can add per-arc fill tracking.

**Files changed:**
- `web/static/js/shelf-fsm-editor.js`
- `web/static/js/fsm-editor.js`
- `internal/ui/export.go` (new)
- `internal/ui/router.go`

---

## [0.1.0-patched99] - 2026-03-16

### Fixed — Label side-flip: X-axis (cos) also continuous

`cornerPointX = (maxWidth/2 + 5) * (cos > 0 ? 1 : -1)` had the same
sign-test problem as cornerPointY had in patched98.

Added `_drawTextCosOverride` and `_smoothLabelCos` using identical logic
to the sin axis, with per-link `_labelCosS/From/To/T0`. `_labelFlipFrame`
now advances both axes per link. `Link.draw` sets both overrides before
calling `drawText` and clears both after. `cornerPointX` now reads
`cosForSide` directly as a multiplier.

Both axes now tween with 450ms cubic ease-in-out. Labels travel on a smooth
diagonal path across the arc when both sin and cos flip simultaneously
(near-45° arcs), or along whichever axis is changing when the arc is
near-horizontal or near-vertical.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched98] - 2026-03-16

### Fixed — Label side-flip: continuous value was being sign-tested away

The animation in patched97 was running correctly but `cornerPointY` was
computed as `(totalHeight/2 + 5) * (sinForSide > 0 ? 1 : -1)` — a sign
test that discards the fractional value. Whether `sinForSide` was 0.9 or
0.1, the sign test produced the same ±1, so the label stayed in its current
position until `sinForSide` crossed zero, at which point it jumped.

Fix: `cornerPointY = (totalHeight/2 + 5) * sinForSide` — the smoothed float
is used directly as the multiplier. When `sinForSide` is 0.4 the label sits
40% of the way toward the opposite side. The transition is now genuinely
continuous.

The non-animated path also changed from `sin` to `sin >= 0 ? 1 : -1` so
unanimated labels snap cleanly to ±1 rather than wobbling with the arc angle.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched97] - 2026-03-16

### Improved — Label side-flip: time-based ease-in-out, fully autonomous RAF

Replaced the per-frame lerp (driven by draw() calls, dependent on mouse
movement rate) with a proper time-based animation.

Each link now carries `_labelFrom`, `_labelTo`, `_labelT0` alongside
`_labelSide`. When `target` flips sign, `_smoothLabelSide` records the
current `_labelSide` as `_labelFrom`, sets `_labelT0 = performance.now()`,
and schedules the RAF loop.

`_labelFlipFrame` runs autonomously at 60fps regardless of mouse activity.
Per frame: `t = (now - _labelT0) / 450ms`; `ease = t² × (3 − 2t)` (cubic
ease-in-out); `_labelSide = from + (to − from) × ease`. Loop continues until
all links have settled.

Mid-flip reversal: `_labelFrom = current _labelSide` (not the original ±1),
so the return journey starts from wherever the label currently sits — no
discontinuity, no snap.

Duration 450ms. Easing means the label accelerates from rest, crosses the
arc at peak speed, then decelerates into position.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched96] - 2026-03-16

### Added — Smooth label side-flip animation when curving a link

**Problem:** `cornerPointY = (totalHeight/2 + 5) * (sin > 0 ? 1 : -1)` flips
sign instantly when the arc angle passes through horizontal, snapping the
label across the arc.

**Fix:** `_smoothLabelSide(link, targetSin)` stores `link._labelSide` (a
float ±1 per link object, lazily initialised to the natural sign on first
draw). Each draw it lerps toward the target sign at factor 0.12 per frame
(~20 frames / ~330ms at 60fps to fully cross). After dragging stops, a single
`requestAnimationFrame` via `_labelFlipRafId` calls `draw()` again to
continue the animation until it settles.

`_drawTextSinOverride` (null or float) is set by `Link.draw` before calling
`drawText`, read in the link branch as `sinForSide`, then cleared. This
threads the smoothed value into `drawText` without changing its signature.

Labels that have never been near the flip point initialise at their natural
side with no animation. The lerp only activates when the sign is actually in
transition.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched95] - 2026-03-16

### Fixed — Dot proximity check runs unconditionally

The dot check was inside the `else` branch of the hover logic, which only
runs when `_haloNode === null`. Since `_haloNode` is primed whenever the
mouse has been over a node (and stays set until 20px outside), the else
branch almost never ran in practice — hence the dot only appeared after
clicking the line directly.

Fix: moved the dot proximity check after the if/else-if/else branching so
it runs on every mousemove idle frame regardless of `_haloNode` state.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched95] - 2026-03-16

### Fixed — Dot and halo inactive unless clicking line first

Both `_hoveredLink` (halo) and `_dotLink` (dot) checks were inside the
final `else` branch of the halo zone if/else chain. If `_haloNode` was set
(user had previously hovered a node), the code took the `else if(_haloNode
!= null)` path instead, and the dot/halo checks were never reached.

Fix: moved `_hoveredLink` update to an unconditional block at the bottom of
the idle section, alongside the existing dot check. Both now fire every
mousemove regardless of `_haloNode` state.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched94] - 2026-03-16

### Changed — Dot hover radius 10px → 30px

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched93] - 2026-03-16

### Fixed/Changed — No edit mode on link creation; animated dot with 10px trigger

**No edit mode after link snap**
`didDrag` is false during link creation (only `movingObject` sets it, which
is false while dragging a link). The edit mode check now additionally requires
`currentLink == null` so it cannot fire while a link is being committed.

**Dot: animated, 10px trigger, never at rest**
`_dotLink`, `_dotOpacity`, `_dotTarget`, `_dotRafId` — same structure as
the halo animation. `_startDotAnim()` fades in at step 0.15/frame (~7 frames)
and out at 0.09/frame (~11 frames).

The dot trigger uses a separate 10px proximity check in the `mousemove`
idle path: temporarily raises `hitTargetPadding` to 10, calls `selectObject`,
then restores. This is independent of `_hoveredLink` (which keeps its 6px
threshold for selection/halo purposes).

Dots no longer appear at rest on unlabelled links. All dots — labelled and
unlabelled — only appear when the mouse is within 10px of the arc, and
animate in/out. Halo behaviour (appears at 6px, same as selection) is
unchanged.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched92] - 2026-03-16

### Fixed — Link dot and halo now actually rendered (patched91 code was lost)

patched91 applied patches across two Python runs; the second run re-read the
original file content, clobbering everything the first run wrote. All code
that should have shipped in patched91 is now correctly applied.

getMidpoint() and drawArcOnly() on Link, SelfLink, StartLink; drawUsing halo
and dot block; _hoveredLink global; mousemove setter; node-hover clear.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched91] - 2026-03-16

### Added — Link midpoint dot and hover halo

**Midpoint dot**
A small filled circle (radius 3px) drawn at each link's midpoint:
- Unlabelled links: always visible at 35% indigo opacity — gives a persistent
  click target where there's no other visual anchor
- Labelled links: appears at 70% indigo opacity only when the mouse is
  hovering near the line (uses existing `selectObject` hit test)
- Not drawn on the selected link (the selection colour already provides feedback)

`getMidpoint()` added to `Link`, `SelfLink`, `StartLink`:
- `Link`: arc midpoint angle `(startAngle + endAngle) / 2` → point on circle;
  or segment midpoint for straight links
- `SelfLink`: point on the loop at `anchorAngle` (same as text position)
- `StartLink`: midpoint between tail and node

**Hover halo**
A 2px-effective semi-transparent wide stroke drawn along the arc before the
normal link draw, visible when `_hoveredLink === link && link !== selectedObject`.
Implemented via `drawArcOnly(c)` on each link type — strokes the path only,
no arrowhead, no text — called with `lineWidth = shelfLinkStroke + 10` and
`strokeStyle = rgba(99,102,241,0.14)`.

**`_hoveredLink` state**
Set in `mousemove` idle path (nothing dragged, nothing panning) from
`selectObject` when result is a link type. Cleared when entering a node,
entering halo zone, on `mouseleave`. Triggers `draw()` only when the
value changes, avoiding unnecessary redraws.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched90] - 2026-03-16

### Fixed — Link label edit rect is axis-aligned, not rotated

The edit rect was drawn with `c.rotate(angleOrNull)` matching the arc angle.
Removed — rect is now axis-aligned using `cx - bw/2, cy - bh/2` directly.
Labels and their edit boxes always face the user regardless of arc angle.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched89] - 2026-03-16

### Added — Multi-line link labels (Alt+Enter, Up/Down)

Link labels now support multiple lines, consistent with node labels.

**`drawText` link branch rewritten:**
- Splits `rawText` on `\n`, measures all lines, derives `maxWidth` and
  `totalHeight`
- Angle-offset calculation updated: `cornerPointY` now uses
  `(totalHeight / 2 + 5)` instead of the hardcoded `10 + 5`. This keeps
  multi-line blocks the same distance from the arc regardless of how many
  lines are present
- `cx/cy` is the block centre in canvas space; lines are rendered centred
  around `cx` with `shelfLineHeight` spacing
- Edit rect is drawn in the rotated coordinate frame (`c.save/translate/
  rotate/rect/restore`) so it aligns with the arc angle at any orientation
- Caret is positioned per-line using `caretToLineCol`/`lineColToPos` (same
  helpers used for node labels); caret x is computed in the line's local
  coordinate frame and mapped to canvas coordinates
- Export path (`advancedFillText`) loops over lines with y-offset so LaTeX
  and SVG exports receive each line separately

**Alt+Enter:** `instanceof Node` guard removed — Alt+Enter now inserts `\n`
into both node and link labels during editing.

**Arrow Up/Down:** `instanceof Node` guards replaced with `selectedObject !=
null` — Up/Down navigate between lines in both node and link labels. Visual
direction may not match keyboard direction on steep arcs; this matches the
behaviour of rotated text frames in other tools (Illustrator, Keynote).

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched88] - 2026-03-16

### Changed — Snap-back duration 280ms → 520ms

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched87] - 2026-03-16

### Added — Pulsating TemporaryLink + snap-back animation

**Pulsating opacity during drag (unconnected)**
`_startLinkPulse()` runs a `requestAnimationFrame` loop updating
`_linkPulseAlpha = 0.35 + 0.65 * (sin(t * 0.006) * 0.5 + 0.5)` (~1 Hz).
`drawUsing` wraps the `TemporaryLink` draw in `c.save/globalAlpha/restore`.
The pulse runs only while `currentLink instanceof TemporaryLink`. Once the
user hovers over a target node (link becomes `Link`/`SelfLink`/`StartLink`),
`_stopLinkPulse()` is called and the link draws at full opacity.

`_startLinkPulse()` is called from: halo-click mousedown, shift+empty-canvas
mousedown, and the `mousemove` path when `currentLink` is (re)set to a
`TemporaryLink`. `_stopLinkPulse()` is called whenever `currentLink`
transitions to any non-Temporary type.

**Snap-back on cancel**
When `mouseup` fires on a `TemporaryLink` (user released without connecting)
or `mouseleave` cancels the drag, `_startSnapBack(link)` runs instead of
clearing immediately.

`_startSnapBack` freezes a copy of `{from, to}` into `_snapBackLink`, sets
`_snapBackActive = true`, clears `currentLink` (so no further mouse tracking),
and runs a 280ms ease-out cubic RAF loop animating `_snapBackLink.to` from
its current position back to `_snapBackLink.from`. Opacity fades from 1 →
0.15 over the same duration, then the link disappears.

`drawUsing` draws `_snapBackLink` as a plain line+arrow when
`_snapBackActive`, respecting `_linkPulseAlpha` for opacity. `mousemove`
returns early during `_snapBackActive`.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched86] - 2026-03-16

### Added — Halo opacity animation (fast in, slow out)

`_haloOpacity` (0–1 float) replaces the binary `_haloVisible` flag in the
draw path. `_haloTarget` (0 or 1) is the destination; `_startHaloAnim()`
runs a `requestAnimationFrame` loop that steps toward it:

- Appearing (target 1): step 0.18/frame — ~6 frames at 60fps (~100ms)
- Disappearing (target 0): step 0.07/frame — ~14 frames at 60fps (~230ms)

When the loop reaches `_haloTarget` within 0.005, it snaps and stops. When
fading to 0, `_haloNode` is cleared at the end of the fade so the ring
completes its exit before releasing state.

`drawUsing` uses `rgba(99,102,241, 0.28 * _haloOpacity)` so the ring fades
smoothly through the same hue. `_haloVisible` is kept as the logical
"should be visible" bool; it and `_haloTarget`/`_haloOpacity` are all
cleared together on mousedown (halo click), shift-drag, and mouseleave.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched85] - 2026-03-16

### Changed — Halo doubled in width

Stroke width 7 → 14px. Arc centre offset 9 → 14px (keeps ring visually
centred at the same boundary distance). Exit-band threshold 10 → 20px.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched84] - 2026-03-16

### Added — Auto-size nodes to label (toolbar toggle)

Toolbar button (format_size icon, highlighted when active) toggles
`shelfAutoSize`. When on, each node's effective radius adapts to its label
text.

**`computeNodeRadius(node, c)`** — called in `drawUsing` before drawing each
node. Splits label on `\n`, measures each line with `c.measureText`, takes
`max(maxLineWidth/2, totalHeight/2) + shelfAutoSizePad` (14px). Never smaller
than `nodeRadius` (default 30). Result stored as `node._r`.

**All geometry reads `node._r`** (falling back to `nodeRadius` when null):
- `Node.draw` — arc radius and accept-state ring
- `Node.containsPoint` — hit testing
- `Node.closestPointOnCircle` — arc endpoint attachment
- `SelfLink.getEndPointsAndCircle` — self-loop geometry
- `Link.getEndPointsAndCircle` — arc offset from node boundaries
- Halo ring radius and exit-band distance check

Off by default. Toggling redraws immediately. `_autoSize` is a Lit reactive
state so the button highlight updates without a full re-render cycle.

**Files changed:** `web/static/js/shelf-fsm-editor.js`, `web/static/js/fsm-editor.js`

---

## [0.1.0-patched83] - 2026-03-16

### Fixed — Halo link-start only fires when halo ring is visible

`mousedown` was checking `_haloNode != null` to intercept clicks and start
link mode. `_haloNode` is set whenever the cursor has been inside a node, so
clicking inside a node was triggering link mode. The condition now requires
`_haloVisible`, which is only true while the cursor is in the 0–10px band
outside the node boundary and the ring is actually drawn.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched82] - 2026-03-16

### Fixed — Halo only appears after leaving node, not while inside

`_haloNode` was set while the mouse was still inside the node (priming for
the exit), and `drawUsing` drew the halo whenever `_haloNode != null`. This
caused the halo to appear immediately on hover, before the mouse had left.

Fix: added `_haloVisible` boolean (default false). The draw in `drawUsing`
now gates on `_haloVisible` instead of `_haloNode`. `_haloVisible` is set
true only in the `mousemove` path when the mouse is 0–10px outside the node
boundary. While the mouse is inside the node, `_haloNode` is primed but
`_haloVisible` stays false. Cursor and halo ring now only appear after exit.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched81] - 2026-03-16

### Added — Halo link-start affordance

Alternative to Shift+drag for adding transitions. When the mouse exits a
node, a semi-transparent indigo ring appears just outside the node boundary.
Clicking the ring starts a link from that node.

**State machine:**
`_haloNode` (null or Node) — set to the node under the cursor while the mouse
is inside it (priming). When the mouse exits, `_haloNode` remains set and the
halo is drawn while the mouse is 0–10px outside the node boundary (world
units, correct at any zoom level). Beyond 10px the halo disappears and
`_haloNode` is cleared.

**Drawing:**
Drawn in `drawUsing` inside the viewport transform (world-space), after all
nodes, before links. Rendered as a thick (7px) semi-transparent indigo stroke
circle at `nodeRadius + 9`. Inherits zoom/pan naturally.

**mousedown interception (halo click):**
Checked before the normal selection logic. If `_haloNode != null` and no
shift key: sets `selectedObject = _haloNode`, creates a `TemporaryLink` from
the node boundary toward the cursor, clears `_haloNode`, and returns. The
mousemove link-tracking logic then takes over identically to shift+drag.

**Cursor:** `cell` (standard "add" crosshair) while halo is visible.

**Cleared by:** pan start, shift+drag start, `mouseleave`.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched80] - 2026-03-16

### Fixed — Link label editing restored; gated on stationary mouseup only

patched79 was too restrictive: links should enter inline edit mode on a
stationary click (no drag), just not before mouseup. The `_hasDragged` guard
in `mouseup` already covers the drag case correctly. Reverted the
`instanceof Node` restriction back to `selectedObject != null`.

Edit mode entry rules are now:
- Drag node or link: no edit mode (hasDragged = true)
- Stationary click on node: edit mode on mouseup
- Stationary click on link: edit mode on mouseup (inline label editing)
- Right-click on link: transition properties dialog (guard + action)

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched79] - 2026-03-16

### Fixed — Link clicks never enter label edit mode

Stationary-click edit mode is now restricted to `Node` objects only.
Clicking a link line selects it; label editing on links goes through the
right-click dialog exclusively.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched78] - 2026-03-16

### Fixed/Added — Click-to-edit deferred to stationary mouseup; right-click dialog

**Click-to-edit deferred to stationary mouseup**

Previously `selectedObject` was set on `mousedown`, and `drawText`/
`_drawTextMultiline` showed the edit rect and caret immediately whenever
`isSelected` was true. This made every node drag appear to enter label edit
mode, because the edit rect appeared as soon as the button went down.

Fix: added `editingLabel` bool (default false) and `_hasDragged` flag.

- `mousedown`: clears `editingLabel = false` and `_hasDragged = false`;
  does NOT call `resetCaret`
- `mousemove` (movingObject path): sets `_hasDragged = true`
- `mouseup`: if `!_hasDragged && selectedObject has text`: sets
  `editingLabel = true` and calls `resetCaret`. Otherwise dragging happened
  and edit mode is not entered.
- `drawText` / `_drawTextMultiline`: `editing` condition now requires
  `editingLabel` in addition to `isSelected && canvasHasFocus()`
- All keyboard handlers (backspace, arrows, home/end, up/down, keypress)
  guard on `editingLabel` — keys are ignored while selected but not editing
- Enter/Escape/Delete clear `editingLabel` on deselect

**Right-click transition properties dialog**

`canvas.oncontextmenu` prevents default and opens `showFSMDialog` when the
right-click target is a Link, SelfLink, or StartLink. Fields: Label (writes
to `link.text`), Guard, Action. Guard and Action are stored in `linkProperties`
Map keyed by `link._id` (lazily assigned integer). Cancel discards changes.
Enter in any field submits.

`showFSMDialog` is a full port of `showDialog` from `rules-editor.js`:
dark/light aware, backdrop blur, Esc to cancel, indigo focus ring.

`ensureLinkId(link)` stamps a `_id` on a link object if absent.

`linkProperties` is cleared in `clearCanvas`. Guard/action are included in
`getBackupData` output and restored by `restoreFromBackupData`, so they
survive export/import round-trips.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched77] - 2026-03-16

### Fixed — Root cause of dangling links: wrong canvas coordinates

**Root cause**

All hit testing, node dragging, link drawing, and coordinate input relied on
`crossBrowserRawMousePos`, which computed canvas-relative coordinates as
`pageX - sum(offsetParent.offsetLeft)`. This is Wallace's 2010 cross-browser
fallback designed for a standalone page where the canvas is a direct child of
`<body>`.

In the Shelf AMS embedded layout (canvas inside `shelf-fsm-editor` →
`.fsm-canvas-wrap` → app shell with sidebar, header, and Lit components), the
`offsetParent` traversal produces a wrong element offset. The canvas position
was systematically miscalculated by a constant amount equal to the
sidebar/header offset.

**Consequence**

Every mouse event had wrong canvas-relative coordinates. `selectObject` was
called with a shifted position. Near Node2, the shifted position missed Node2's
`containsPoint` hit area (30px radius) but fell within the link arc's
`containsPoint` hit area (6px from arc). So `selectObject` returned the Link
instead of Node2. Then `movingObject = true` with `selectedObject = Link`, and
every `mousemove` called `Link.setAnchorPoint` — bending the arc — rather than
`Node.setAnchorPoint` — moving the node.

This was invariable because the offset error is constant for a given layout.

**Fix**

`crossBrowserRawMousePos` now returns `{ x: e.offsetX, y: e.offsetY }`.
`e.offsetX/Y` gives coordinates relative to `e.target` (the canvas) directly,
without any traversal, reliably in all modern browsers regardless of layout
complexity. `crossBrowserElementPos` and `crossBrowserMousePos` are removed.

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched76] - 2026-03-16

### Fixed — Accidental self-loops, dangling links, cursor feedback

**Bug: accidental self-loops on node click**
`mousedown` was checking the `shift` global variable instead of `e.shiftKey`.
The global can get stuck `true` if the user releases Shift while a mouse
button is held (keyup fires only when the window has focus). This caused
every subsequent click on a node to begin a self-link rather than selecting
the node for editing.

Fix: both shift-checks in `mousedown` (`selectedObject instanceof Node` and
`else if(shift)`) now read `e.shiftKey` directly from the event. The `shift`
global is kept for keydown/keyup bookkeeping but is no longer used in mouse
handlers.

**Bug: dangling links after Delete during drag**
`currentLink` holds a preview `Link(nodeA, nodeB)` that is not yet in
`links[]`. If Delete is pressed between the last `mousemove` (which set the
preview) and `mouseup`, the delete handler removes the node from `nodes[]`
but cannot clean up `currentLink`. Then `mouseup` commits the link with a
stale node reference.

Fix: `mouseup` now validates all node references in `currentLink` via
`nodes.indexOf(...)` before pushing to `links[]`. An invalid link is silently
discarded. Additionally, `drawUsing` scans `links[]` on each draw and removes
any link whose node references are no longer in `nodes[]` (defensive guard for
any other path that could leave stale references).

**Cursor feedback**
- `mousedown` on node (drag): cursor → `grabbing`
- `mousedown` for pan: cursor → `grabbing` (unchanged)
- `mousedown` for link creation (shift): cursor → `crosshair`
- `mouseup`: cursor resets immediately based on what is under the mouse
  (`grab` if hovering a node, `crosshair` otherwise)
- `mousemove` (nothing held): cursor = `grab` over nodes, `crosshair` elsewhere
- `mouseleave`: cancels any in-progress TemporaryLink or pan, resets cursor

**Files changed:** `web/static/js/shelf-fsm-editor.js`

---

## [0.1.0-patched75] - 2026-03-16

### Added/Changed — Distinct colours, smaller font, multi-line node labels

**Distinct colours per element type**
Four new globals, set by `_applyTheme` in the Lit shell:
- `shelfNodeStrokeColor` — node circle outline: indigo-500/indigo-400
- `shelfNodeLabelColor` — text inside nodes: foreground
- `shelfLinkStrokeColor` — transition arcs: indigo-400/indigo-600
- `shelfLinkLabelColor` — text on transitions: slate-500/slate-400 (muted)

`shelfActiveLabelColor` is a transient global set by `drawUsing` before each
draw call (selected: 'blue' → accent; unselected: appropriate label colour).
`drawText` and `_drawTextMultiline` read `shelfActiveLabelColor` for text,
decoupling text colour from stroke/fill used for circles and arrowheads.

`Node.draw` bug fixed: `c.fillStyle = savedStroke` was overwriting the label
colour with the circle stroke colour before calling `drawText`. Removed.

**Font size: 14px → 12px**
`shelfFont` and `shelfLineHeight` (new global, 16px) updated in both engine
and Lit shell.

**Multi-line node labels**
`drawText` branches on `angleOrNull === null`:
- Node labels → `_drawTextMultiline`: splits on `\n`, centres each line
  independently around the node centre, scales edit rect to total block
  height, positions caret by (lineIndex, col)
- Link labels → single-line Wallace path, unchanged

`caretToLineCol(text, pos)` and `lineColToPos(text, line, col)` helpers
convert between flat caret position and (line, col) pairs.

New keyboard actions (nodes only):
- Alt+Enter: insert `\n` at caret, increment position
- Arrow Up (38): move caret to same column on previous line
- Arrow Down (40): move caret to same column on next line

Link labels remain single-line; no multi-line for transitions.

**Files changed:** `web/static/js/shelf-fsm-editor.js`, `web/static/js/fsm-editor.js`

---

## [0.1.0-patched74] - 2026-03-16

### Improved — Label editor: caret positioning, arrow keys, edit rect

**Caret position tracking**
`caretPosition` (integer) tracks where in the text string the caret sits.
Initialised to `text.length` when an object is selected (caret starts at end,
ready for appending). Reset to 0 on deselect.

**Arrow keys**
- Left (37): move caret left one character
- Right (39): move caret right one character
- Home (36): jump to start of label
- End (35): jump to end of label
All prevent default to suppress page scrolling.

**Backspace**
Now deletes the character at `caretPosition - 1` (not always the last char)
and decrements `caretPosition`. Mid-string edits now work correctly.

**Character insertion**
Printable characters are spliced into `text` at `caretPosition` (not
appended). Caret advances by one after each insertion.

**Escape key**
Added as a second way to commit/deselect (key 27), alongside Enter (key 13).

**Edit-mode background rect**
When an object is selected and the canvas has focus, `drawText` draws a
filled rounded-rect (radius 4px, 7px padding) behind the label text using
`shelfEditBg` (semi-transparent canvas colour) and `shelfEditBorder` (indigo,
theme-aware opacity). The rect has a minimum width of 20px so it is visible
even on empty labels. Uses `c.roundRect` with `c.rect` fallback.

The rect and caret both disappear as soon as the object is deselected (Enter,
Escape, or click elsewhere). No visual change when the canvas does not have
focus.

**New globals** (set by Lit shell `_applyTheme`):
- `shelfEditBg` — dark: `rgba(30,41,59,0.97)`, light: `rgba(248,250,252,0.97)`
- `shelfEditBorder` — dark: `rgba(129,140,248,0.7)`, light: `rgba(99,102,241,0.5)`

**Files changed:** `web/static/js/shelf-fsm-editor.js`, `web/static/js/fsm-editor.js`

---

## [0.1.0-patched73] - 2026-03-16

### Changed — Launcher: version display and clipboard

**Version display**
`SHELF_VERSION` read from `VERSION` file at startup. Shown in two places:

- Startup banner: `│  version <version>  │` line below the title
- Ready box: `│  Shelf AMS <version>  │` replaces the plain "Shelf AMS is
  running" line

**Clipboard**
`copy_to_clipboard()` helper tries in order: `pbcopy` (macOS), `wl-copy`
(Wayland), `xclip` (X11), `xsel` (X11 fallback). Copies
`http://localhost:<SHELF_PORT>` once the ready box is about to print. If a
clipboard tool is found, adds `│  (URL copied to clipboard)  │` to the box.
If none is available, silently omits the line.

**Files changed:** `scripts/launch.sh`

---

## [0.1.0-patched72] - 2026-03-16

### Fixed — Link stroke width increased to 2px

`shelfLinkStroke` raised from 1.5 → 2.0 in both the engine default and the
Lit shell `_applyTheme`. Arrowhead geometry unchanged.

**Files changed:** `web/static/js/shelf-fsm-editor.js`, `web/static/js/fsm-editor.js`

---

## [0.1.0-patched71] - 2026-03-16

### Added/Fixed — Zoom inertia, visual polish, thicker arrows, Enter key

**Zoom inertia (shelf-fsm-editor.js)**
Wheel events accumulate into `_zoomVelocity` (log-space). A
`requestAnimationFrame` loop applies `exp(-velocity * dt * 0.05)` per frame,
decaying by 0.78 each frame (~60fps → ~10 frames of glide). Working in
log-space means the feel is uniform at any zoom level. Cursor position is
captured per-event so fast flicks track correctly. RAF loop cancels when
|step| < 0.0001. dt is capped at 64ms to prevent jumps after tab switches.

**Visual polish**
- `shelfNodeFill` global: nodes are filled with the canvas background colour
  before stroking, so arcs passing behind a node body are hidden
- `shelfAccentColor` (#169B62, Irish green): accept-state inner circle draws
  in accent colour instead of the default foreground
- `shelfNodeStroke` / `shelfLinkStroke` (both 1.5px): uniform slightly heavier
  strokes for nodes and arcs
- All style globals initialised in `shelf-fsm-editor.js`, set on each theme
  change by `_applyTheme` in the Lit shell — same bridge pattern as `shelfFont`

**Thicker arrowheads**
`drawArrow` triangle: 8×10px → 10×12px. Visibly more prominent at all zoom
levels.

**Enter key ends label editing**
Added `key == 13` branch to the `keydown` listener: sets `selectedObject =
null` and redraws. Consistent with common text editor conventions.

**Files changed:** `web/static/js/shelf-fsm-editor.js`, `web/static/js/fsm-editor.js`

---

## [0.1.0-patched70] - 2026-03-15

### Added — Virtual canvas, pan, zoom, zoomToFit, font variable, zoom buttons

**Renamed: fsm-evan.js → shelf-fsm-editor.js**
Attribution header trimmed to: original author, website, repo, licence.
No inline changelog in the vendored file.

**Virtual canvas (shelf-fsm-editor.js)**
- `viewport {x, y, k}` global; reset to `{0,0,1}` in `clearCanvas()`
- `drawUsing`: `c.translate(viewport.x + 0.5, viewport.y + 0.5)` +
  `c.scale(viewport.k, viewport.k)` — all node/link drawing goes through
  the viewport transform; no changes to geometry code
- `crossBrowserRawMousePos` (canvas-space) and updated
  `crossBrowserRelativeMousePos` (world-space = raw unprojected through
  viewport) — single change point, all hit testing works unchanged
- Pan: background drag sets `panningCanvas` flag, updates `viewport.x/y`,
  cursor changes to `grabbing` during pan
- Wheel zoom toward cursor: `onwheel` in `initFSM`, clamped to `[0.15, 5]`
- `zoomToFit()`: computes bounding box of all nodes, scales and centres
  viewport to fit with 60px padding; called after preset load and data restore
- `zoomBy(factor)`: zoom toward canvas centre; used by toolbar buttons
- `shelfFont` global variable (default `14px system-ui, …`); replaces all
  three hardcoded font literals in `drawText` and both export `measureText`
  stubs — consistent font across canvas draw and SVG/LaTeX export

**Lit shell (fsm-editor.js)**
- `draw()` wrapped: calls `origDraw()` then `this._syncGrid(canvas)` so grid
  tracks viewport on every redraw
- `_syncGrid`: `background-size = size*k`, `background-position = vp offset %
  scaled` — grid tiles follow pan and scale
- `_applyTheme`: sets `window.shelfFont` (bridge from Lit theme state to
  canvas draw loop); both themes use same face currently, variable exists
  for per-theme customisation without touching the engine
- Zoom toolbar buttons: `zoom_out`, `fit_screen` (→ `zoomToFit`), `zoom_in`
  via `zoomBy(1.25)` / `zoomBy(1/1.25)`
- Hint bar updated: "Drag background: pan · Scroll: zoom"
- `zoomToFit()` called after preset load and saved-data restore

**Files changed:** `web/static/js/shelf-fsm-editor.js` (renamed + patched),
`web/static/js/fsm-editor.js`, `internal/ui/pages.go`

---

## [0.1.0-patched69] - 2026-03-15

### Added/Changed — Grid, attribution, Shelf FSM Editor rename

**Grid on canvas**
CSS `background-image` linear-gradient grid (24px, indigo-tinted) applied to
the Wallace canvas via `_applyTheme()` in the Lit shell. Updates immediately
on dark mode toggle. Survives canvas resize (CSS background is not cleared
when `canvas.width`/`height` are reassigned). No changes to Wallace's drawing
pipeline — the grid is purely a CSS background on the canvas element.

**Attribution in fsm-evan.js**
Extended file header to credit Evan Wallace explicitly with links to his
website (https://madebyevan.com/) and source repository
(https://github.com/evanw/fsm), and documents all patches applied by
Shelf AMS.

**Rename: Shelf FSM Editor**
- Nav label: "Shelf FSM"
- Page title: "Shelf FSM Editor" (new FSM) or "{name} — Shelf FSM Editor" (edit)
- Default page title: "Shelf FSM Editor" (was "New FSM")

**Files changed:** `web/static/js/fsm-editor.js`, `web/static/js/fsm-evan.js`,
`internal/ui/layout.go`, `internal/ui/routes.go`, `internal/ui/pages.go`

---

## [0.1.0-patched68] - 2026-03-15

### Fixed — Guard all standalone-only getElementById calls in fsm-evan.js

`radiusChanged`, `output`, `jsonUploaded`, and `uploadJSON` all reference
DOM elements (`#rangeSlider`, `#output`, `#jsonUpload`) that only exist in
Wallace's original standalone page. Added null guards to all four so they
fail silently rather than throwing when called in the embedded context.

**Files changed:** `web/static/js/fsm-evan.js`

---

## [0.1.0-patched67] - 2026-03-15

### Fixed — clearCanvas: rangeSlider not present in embedded context

Wallace's `clearCanvas()` called `document.getElementById("rangeSlider").value = 30`
unconditionally. The slider only exists in his standalone page, not in the
Shelf embedded canvas. Guarded with a null check.

**Files changed:** `web/static/js/fsm-evan.js`

---

## [0.1.0-patched66] - 2026-03-15

### Fixed + Added — FSM editor toolbar theme; demo preset selector

**Theme fix**
The toolbar colours are computed inside `render()` so Lit must re-render
to update them. The MutationObserver previously only called `_applyTheme`
and `draw()` — neither of which triggers a Lit update cycle.

Fix: added `_dark` as a reactive Lit state property. The observer now sets
`this._dark = document.documentElement.classList.contains('dark')`, which
automatically calls `requestUpdate()` and re-renders the toolbar with the
correct palette.

**Demo preset selector**
Five FSMs from the Shelf AMS domain, in Wallace's native backup format
(`{nodes, links, nodeRadius}`), loaded via `clearCanvas()` +
`restoreFromBackupData()` + `draw()`. The select resets to "Load demo…"
after loading (same UX pattern as the Rules Editor).

Presets use Wallace's `Link` (directed arc with curve control via
`parallelPart`/`perpendicularPart`), `StartLink` (entry arrow), and
`SelfLink` (self-loop with `anchorAngle`) types natively — no adaptation
layer needed.

1. Lock / Unlock — bidirectional arc pair
2. Asset Lifecycle — diamond flow with maintenance loop
3. Extinguisher Inspection — branching pass/fail
4. Cold Chain Monitor — escalating alert loop
5. Work Order — six states with hold branch and self-loop on In Progress

**Files changed:** `web/static/js/fsm-editor.js`

---

## [0.1.0-patched65] - 2026-03-15

### Fixed — FSM/Rules editor interference, missing Rete JS, double script load

**Root causes found in full audit:**

1. `RulesEditorPage` was loading `fsm-editor.js` (the Wallace Lit shell)
   instead of the Rete editor module. The Rete JS had been overwritten by
   patched61 and was not preserved under a new name.
   Fix: restored Rete editor JS from patched60 checkpoint as `rules-editor.js`.
   `RulesEditorPage` now loads `rules-editor.js`.

2. `FSMEditorPage` loaded `fsm-evan.js` twice: once as a plain `<script>`
   and once via `import '/static/js/fsm-evan.js'` inside the module.
   Plain scripts and ES module imports are separate loader paths so this
   caused the globals to be set twice, with unpredictable ordering.
   Fix: removed the module import from `fsm-editor.js`; the page loads
   `fsm-evan.js` as a plain `<script>` before the module, which is the
   correct pattern for globals-based libraries.

3. Wallace's `document.addEventListener('keydown/up/press')` fired on every
   page since `fsm-evan.js` is loaded globally. Guards added: each listener
   now returns immediately if `canvas` is null (i.e. `initFSM` not called).

**Separation matrix (verified):**

| File | Rules Editor | FSM Editor |
|---|---|---|
| `rules-editor.js` | ✓ loaded | ✗ not loaded |
| `fsm-graphview.js` | ✓ imported by rules-editor | ✗ not loaded |
| `fsm-evan.js` | ✗ not loaded | ✓ plain script |
| `fsm-editor.js` | ✗ not loaded | ✓ module |
| Rete importmap | ✓ ExtraHead | ✓ ExtraHead (for lit only) |

**Files changed:** `internal/ui/pages.go`, `web/static/js/fsm-editor.js`
(removed import), `web/static/js/fsm-evan.js` (keyboard guards),
`web/static/js/rules-editor.js` (new — restored from patched60)

---

## [0.1.0-patched64] - 2026-03-15

### Fixed — RulesEditorEdit: use rule_graphs collection, not fsm_machines

Rule graphs and FSM machines are distinct domain entities and stored
separately in olu. `RulesEditorEdit` now fetches from `rule_graphs`;
`FSMEditorEdit` continues to use `fsm_machines`. The TODO comment is removed.

Verified no other cross-collection contamination: all remaining
`fsm_machines` references in `internal/handlers/api/fsm.go` are correct
FSM API handlers.

**Files changed:** `internal/ui/routes.go`

---

## [0.1.0-patched63] - 2026-03-15

### Fixed — Handler audit: field name contamination, wrong error messages

Full audit of the four editor handlers after the automated rename:

**`RulesEditorEdit`** error messages said "FSM not found" — corrected to
"Rule graph not found". olu collection remains `fsm_machines` pending a
data-model decision (marked with TODO comment).

**`RulesEditorData`** field names were FSM-prefixed (`FSMID`, `FSMName`,
`FSMJSON`) — renamed to `RuleID`, `RuleName`, `RuleJSON`. All usages in
`RulesEditorPage` and `RulesEditorEdit` updated accordingly.

**`RulesEditorPage`** default title was "New FSM" — corrected to
"New Rule Graph".

**`FSMEditorPage`** had acquired `data.RuleJSON` from the replace script
hitting the wrong occurrence — corrected back to `data.FSMJSON`.

**Files changed:** `internal/ui/pages.go`, `internal/ui/routes.go`

---

## [0.1.0-patched62] - 2026-03-15

### Fixed — Rules Editor redirect; FSM editor infinite recursion

**Bug 1: Rules Editor redirecting to FSM Editor**
`RulesEditorNew` was still rendering `FSMEditorPage(FSMEditorData{...})` at
`/fsm/editor` — the Python rename script updated the function signature but
not the body. Fixed to use `RulesEditorData` and `RulesEditorPage` at
`/rules/editor`.

**Bug 2: Maximum call stack size exceeded in fsm-editor.js**
`themedContext(canvas)` called `canvas.getContext('2d')` internally, but by
the time it ran, `canvas.getContext` had already been replaced with the
patching wrapper that calls `themedContext` — infinite mutual recursion.
Fixed by passing the raw context object directly to `themedContext(rawCtx)`
instead of the canvas element. The patching wrapper now calls
`origGetContext()` once and passes the result.

**Files changed:** `internal/ui/routes.go`, `web/static/js/fsm-editor.js`

---

## [0.1.0-patched61] - 2026-03-15

### Added — FSM Editor (Evan Wallace canvas); Rete renamed to Rules Editor

**Rules Editor (renamed)**
The Rete.js node-graph editor moves from `/fsm/editor` to `/rules/editor`
and is relabelled "Rules Editor" throughout: nav, page title, handlers, data
types (`RulesEditorData`, `RulesEditorPage`, `RulesEditorNew`, `RulesEditorEdit`).

**FSM Editor (new) — `GET /fsm/editor`**
A canvas-based FSM designer using Evan Wallace's MIT-licensed engine
(http://madebyevan.com/fsm/), wrapped in a Lit shell.

`web/static/js/fsm-evan.js` — Wallace's engine, minimally patched:
- `window.onload` → `function initFSM(canvasEl)` (takes element directly)
- `saveBackup()` removed from `draw()` (save handled explicitly)
- `document.onkeydown/up/press` → `addEventListener` (non-destructive)

`web/static/js/fsm-editor.js` — Lit shell (`<shelf-fsm-editor>`):
- Light DOM, full-height canvas
- Canvas context proxy remaps Wallace's `'black'`/`'blue'` to palette colours
- MutationObserver re-applies theme and redraws on dark mode toggle
- ResizeObserver keeps canvas sized to its container
- Toolbar: hint text, Clear, Export JSON, Save (when fsm-id present)
- Restores saved graph from `<script id="fsm-data">` on load

Interaction (Wallace's native UX, unchanged):
- Double-click canvas: add state
- Shift-drag: draw transition
- Double-click state: toggle accept state
- Click + type: label selected object
- Delete: remove selected

**Files changed:** `web/static/js/fsm-evan.js` (new),
`web/static/js/fsm-editor.js` (rewritten),
`web/static/js/fsm-graphview.js` (kept, now only used by Rules Editor),
`internal/ui/layout.go`, `internal/ui/pages.go`,
`internal/ui/routes.go`, `internal/ui/router.go`

---

## [0.1.0-patched60] - 2026-03-15

### Fixed — Identifier 'NODE_W' already declared

`NODE_W` and `NODE_H` were both imported from `fsm-graphview.js` and declared
as `const` locally in `fsm-editor.js`. Removed from the import.
The local values (190×80) are intentionally different from the graphview
ellipse values (104×72) and are used for transition label positioning.

**Files changed:** `web/static/js/fsm-editor.js`

---

## [0.1.0-patched59] - 2026-03-15

### Fixed — FSM graph view: replaced custom elements with plain Lit templates

The `LitElement`-based `ShelfFsmNodeElement` and `ShelfFsmConnectionElement`
rendered asynchronously (Lit batches updates to microtasks). This meant:
- The SVG ellipse wasn't in the DOM when Rete measured node dimensions
- `<rete-ref>` elements weren't discoverable via `document.elementsFromPoint`
  during the same tick, so socket positions were never registered

Fix: removed the custom elements entirely. `graphNodeFactory` and
`graphConnectionFactory` now return plain Lit template factory functions
directly — a div with inline SVG for nodes, an SVG with arrowhead marker for
connections. Lit renders these synchronously into the Rete node container.
Rete sees the DOM immediately and connection routing works.

`fsm-graphview.js` is now ~150 lines with no LitElement, no shadow DOM, no
`customElements.define`. The `ShelfFsmNodeElement` and
`ShelfFsmConnectionElement` classes are removed.

**Files changed:** `web/static/js/fsm-editor.js`, `web/static/js/fsm-graphview.js`

---

## [0.1.0-patched58] - 2026-03-15

### Fixed — fsm-graphview.js: css is not defined

`ShelfFsmConnectionElement` still had `static styles = css\`...\`` after the
`css` import was removed. Converted to light DOM (`createRenderRoot` returns
`this`) with inline `<style>` tag, matching `ShelfFsmNodeElement`.

**Files changed:** `web/static/js/fsm-graphview.js`

---

## [0.1.0-patched57] - 2026-03-15

### Fixed — FSM editor graph view: nodes not rendering, toggle not swapping

**Bug 1: toggle does not visually change nodes**
`area.update('node', id)` updates the component's data in place but does not
swap the component type — the Lit renderer reuses the existing template
instance. Fixed by a remove-and-re-add cycle in the toggle handler: node
instances and positions are saved, all connections/nodes removed, then
re-added. Fresh `addNode` events invoke the `customize.node` factory with
the current `graphMode`, producing the correct component type.

**Bug 2: loading a preset in graph mode renders transitions but not nodes**
Two root causes:
1. `<shelf-fsm-node>` used shadow DOM (`LitElement` default). Shadow DOM
   delays `clientWidth/clientHeight` availability, so `AreaExtensions.zoomAt`
   measured 0 dimensions and placed all nodes off-screen. Fixed by
   `createRenderRoot() { return this; }` (light DOM rendering).
2. `<rete-ref>` inside shadow DOM is not returned by
   `document.elementsFromPoint`, so drag-to-connect could not find sockets.
   Fixed by light DOM rendering, which puts `<rete-ref>` in the regular DOM.

**Also:**
- `static styles` removed from `ShelfFsmNodeElement`; styles are now inlined
  as a `<style>` tag in `render()` (compatible with light DOM).
- `StateNode` now declares explicit `width = 200; height = 120` so the area
  plugin has correct dimensions before the component renders.
- `zoomAt` and theme calls in the preset-change handler are deferred 150 ms to
  allow `<shelf-fsm-node>` to complete its Lit update cycle first.

**Files changed:** `web/static/js/fsm-editor.js`, `web/static/js/fsm-graphview.js`

---

## [0.1.0-patched56] - 2026-03-15

### Fixed — FSM editor: customize factories must return explicit templates, not undefined

When `customize.node` returns `undefined`, the lit-plugin tries to call
`undefined(...)` and throws `TypeError: r(...) is not a function`. The
preset has no "return undefined = use default" fallback — the factory must
always return a callable template.

Fixed by returning explicit default templates in block mode:
- node:       `({ emit }) => html\`<rete-node .data=... .emit=...>\``
- connection: `({ path }) => html\`<rete-connection .path=...>\``

Also removed the custom `socketPositionWatcher` override — the default
`getDOMSocketPosition` works correctly for both modes since the `<rete-ref>`
elements in the graph-mode node are centred on the node bounding box.

**Files changed:** `web/static/js/fsm-editor.js`

---

## [0.1.0-patched55] - 2026-03-15

### Added — FSM editor: graph theory view using Rete customize API

A toolbar toggle switches between block view (Rete card nodes) and graph
theory view (automaton-style ellipses + directed arcs) without any parallel
renderer. Both modes are driven by Rete's own layout and interaction system.

**How it works**
`Presets.classic.setup()` accepts `socketPositionWatcher` and `customize`.
The `customize.node` and `customize.connection` factories check `graphMode`
at render time and return either `undefined` (use built-in component) or a
Lit template rendering the custom element. On toggle, `area.update('node',
id)` / `area.update('connection', id)` is called for every element, forcing
Rete to re-invoke the factory with the new mode.

**`web/static/js/fsm-graphview.js`** (new)
- `ShelfFsmNodeElement` — Lit `LitElement` rendering an SVG ellipse. Normal
  states get an indigo-stroked ellipse; Start gets a filled Irish green disc;
  End gets a double-stroke Irish green ellipse. A transparent `<rete-ref>`
  overlay at input/output ports keeps Rete's connection system working.
- `ShelfFsmConnectionElement` — Lit element rendering a straight `<path>`
  with an arrowhead marker (via SVG `<marker>`).
- `makeEllipseSocketPosition()` — wraps `getDOMSocketPosition()` and
  overrides `calculatePosition` to return the ellipse boundary point at
  left-centre (input) or right-centre (output).
- `makeConnectionPathPlugin()` — returns a `ConnectionPathPlugin` configured
  for linear (straight) paths using `curveLinear` from d3-shape.

**`internal/ui/layout.go`**
- `d3-shape@3.2.0` and `rete-connection-path-plugin@2.0.4` added to
  importmap with correct `?deps=` entries.

**`internal/ui/pages.go`**
- "Graph view / Block view" toggle button added to FSM editor toolbar.

**Files changed:** `web/static/js/fsm-editor.js`, `web/static/js/fsm-graphview.js`,
`internal/ui/layout.go`, `internal/ui/pages.go`

---

## [0.1.0-patched54] - 2026-03-15

### Fixed — FSM editor: transition labels track node drag

`nodetranslated` events were not triggering a label redraw. Labels are
now redrawn on `nodetranslated` in addition to `translated`/`zoomed`.

**Files changed:** `web/static/js/fsm-editor.js`

---

## [0.1.0-patched53] - 2026-03-15

### Fixed — FSM editor: WeakMap is not iterable

`elemToNodeId` was declared as a `WeakMap` but iterated in `applyTheme`
with `for...of`. WeakMaps are not iterable by design. Changed to `Map`.

**Files changed:** `web/static/js/fsm-editor.js`

---

## [0.1.0-patched52] - 2026-03-15

### Fixed — FSM editor: Start/End nodes, transition labels, delete, import keying

**Start/End nodes in all presets**
All five demo FSMs now have explicit Start and End nodes with `stateType:
'start'/'end'` (Irish green border). Each preset connects them as the graph
entry and exit points.

**Transition labels now visible**
Labels appended to `area.area.content.holder` (the transformed inner div) in
graph-space coordinates so they pan and zoom automatically with no position
recomputation. Source/target positions looked up via `_area.nodeViews` and
`_editor.getConnections()` — no DOM path geometry needed.

**Irish green border reliable**
Replaced the `closest('[data-id]')` DOM heuristic with a `WeakMap<rete-node
element → nodeId>` (`elemToNodeId`) populated in the area render pipe when
`ctx.data.type === 'node'`. `injectShadowStyle(el, nodeId)` now receives the
nodeId directly.

**importGraph node-map keying**
`presetId = n.id` is captured before `editor.addNode()` so the connection
source/target lookup always uses the preset's string id, not Rete's UUID.

**Delete key**
`Delete` or `Backspace` (when focus is not in an input/textarea/select)
removes all selected nodes and their connected transitions.

**Files changed:** `web/static/js/fsm-editor.js`

---

## [0.1.0-patched51] - 2026-03-15

### Added — FSM editor: multi-transition, property dialogs, labels, demo presets

**Multiple output transitions**
- `StateNode` output changed to `multipleConnections: true` — any number of
  transitions can leave a state

**Property editors (Tailwind-styled dialogs, no framework)**
- Double-click any node → State Properties dialog (name, type, description)
- Click any connection line → Transition Properties dialog (label, guard, action)
- `showDialog()` renders a modal with dark/light mode support, keyboard dismiss
  (Esc), focus management, and indigo focus rings matching the app

**Transition labels**
- Label + guard text overlaid at the midpoint of each connection path
- Uses `path.getTotalLength()` / `getPointAtLength()` + `getBoundingClientRect()`
  to compute screen-space midpoint relative to canvas
- Re-positioned on every `translated`/`zoomed` event (same pipe as grid)

**Node type via data model**
- `nodeData` Map (nodeId → {description, stateType}) replaces title-text heuristic
  for Irish green border; `stateType: 'start'|'end'` triggers green stroke
- Preset graphs populate nodeData correctly on load

**Add State node placement fix**
- New nodes placed at the centre of the current viewport, accounting for
  pan offset and zoom scale

**Demo FSM preset selector**
- `<select id="fsm-preset-select">` in toolbar; resets and reloads the graph
  and resets to "Load demo…" after loading (non-destructive UX)
- Five presets, Shelf AMS domain, increasing complexity:
  1. Lock / Unlock (2 states, 2 transitions)
  2. Asset Lifecycle (4 states, 4 transitions)
  3. Extinguisher Inspection (5 states, branching pass/fail)
  4. Cold Chain Monitor (5 states, escalating alert loop)
  5. Work Order (6 states, rejection loop, on-hold branch)
- Each preset includes guard conditions and action strings

**Export extended**
- `exportGraph()` now includes `stateType`, `description`, `label`, `guard`,
  `action` in the JSON output

**Files changed:** `web/static/js/fsm-editor.js`, `internal/ui/pages.go`

---

## [0.1.0-patched50] - 2026-03-15

### Added — FSM editor: scrolling grid that tracks canvas pan and zoom

The grid is rendered via CSS `background-image` (two `linear-gradient` passes
for horizontal and vertical lines) on the canvas container. It stays in sync
with the viewport by:

- `area.addPipe` listening for `translated` and `zoomed` context types and
  calling `updateGrid(container, area.area.transform)` on every event
- `background-size` scales with `transform.k` (zoom level × GRID_SIZE)
- `background-position` is set to `(x mod size, y mod size)` so the grid
  tiles seamlessly as the user pans

Grid line colour is a subtle indigo tint (rgba(99,102,241,0.10) dark,
rgba(99,102,241,0.12) light) matching the application palette. Grid size is
24px at zoom=1.

**Files changed:** `web/static/js/fsm-editor.js`

---

## [0.1.0-patched49] - 2026-03-15

### Changed — FSM editor: ochre sockets; Irish green stroke on Start/End nodes

- Socket (connection point) colour changed from indigo to ochre (#D4871A)
  in both light and dark themes
- Start and End nodes get an Irish green (#169B62) border stroke instead of
  the default indigo, applied by reading the title text from the shadow root
  after render and overriding border-color in the injected style

**Files changed:** `web/static/js/fsm-editor.js`

---

## [0.1.0-patched48] - 2026-03-15

### Changed — FSM editor node theming via shadow DOM style injection

Rete's Lit plugin renders nodes as Web Components. CSS custom properties
inherit through shadow DOM boundaries (used for background/socket colours),
but hardcoded values like the node border colour (#4e58bf) cannot be
overridden from outside. The plugin exposes no CSS parts.

Fix: a MutationObserver on the canvas detects each `<rete-node>` element as
it is added and injects a `<style id="shelf-theme">` tag directly into its
shadow root. A second MutationObserver on `<html>` re-applies styles on dark
mode toggle. Both observers run whenever `applyTheme()` is called.

Palette (light / dark):
- Node background: white / slate-800
- Node border: indigo-300 / indigo-700
- Node title: indigo-50 bg / indigo-900 bg with matching text
- Socket: indigo-500 / indigo-400
- Connection: indigo-500 / indigo-400
- Selected ring: indigo glow

**Files changed:** `web/static/js/fsm-editor.js`

---

## [0.1.0-patched47] - 2026-03-15

### Fixed — Complete esm.sh ?deps= graph for all Rete packages

Analysed all five ESM files directly to map the full inter-package dependency
graph, then set ?deps= correctly for every package in one pass:

  rete                   — root, no deps
  rete-area-plugin       — ?deps=rete@2.0.4
  rete-connection-plugin — ?deps=rete@2.0.4,rete-area-plugin@2.0.3
  rete-render-utils      — ?deps=rete@2.0.4,rete-area-plugin@2.0.3
  @retejs/lit-plugin     — ?deps=rete@2.0.4,rete-area-plugin@2.0.3,
                                 rete-render-utils@2.0.2,lit@3

Previously rete-render-utils was missing rete from its deps, and
@retejs/lit-plugin was missing rete-render-utils.

**Files changed:** `internal/ui/layout.go`

---

## [0.1.0-patched46] - 2026-03-15

### Fixed — FSM editor: "actual parent is not instance of type"

`rete-connection-plugin` and `rete-render-utils` both import `BaseAreaPlugin`
from `rete-area-plugin`. The previous fix only pinned `rete` in their `?deps=`
parameter, leaving them to bundle their own private copy of `rete-area-plugin`.
The `instanceof BaseAreaPlugin` check in `parentScope()` then failed across
the two different class objects.

Fixed by adding `rete-area-plugin@2.0.3` to the deps of both packages:
- `rete-connection-plugin@2.0.3?deps=rete@2.0.4,rete-area-plugin@2.0.3`
- `rete-render-utils@2.0.2?deps=rete@2.0.4,rete-area-plugin@2.0.3`

**Files changed:** `internal/ui/layout.go`

---

## [0.1.0-patched45] - 2026-03-15

### Fixed — FSM editor: "cannot use non-Scope instance" due to duplicate rete bundles

Each package fetched from esm.sh without coordination bundles its own copy
of its peer dependencies. `rete-area-plugin` and `@retejs/lit-plugin` both
bundle `rete` internally, giving them a different `Scope` class than the one
imported directly. `NodeEditor.use()` does an `instanceof Scope` check which
fails across two different class objects even if they are structurally identical.

Fix: add `?deps=` to the esm.sh URLs for all plugin packages. This tells
esm.sh to rewrite those import specifiers to point at the pinned shared
instances rather than bundling private copies.

- `rete-area-plugin@2.0.3?deps=rete@2.0.4`
- `rete-connection-plugin@2.0.3?deps=rete@2.0.4`
- `rete-render-utils@2.0.2?deps=rete@2.0.4`
- `@retejs/lit-plugin@2.0.7?deps=rete@2.0.4,rete-area-plugin@2.0.3,lit@3`

**Files changed:** `internal/ui/layout.go`

---

## [0.1.0-patched44] - 2026-03-15

### Fixed — fsm-editor.js: LitArea2D is a TypeScript type, not a runtime export

`@retejs/lit-plugin@2.0.7` ESM file exports only `LitPlugin` and `Presets`.
`LitArea2D` is a TypeScript type annotation that evaporates at compile time
and is not present in the published ESM bundle. Importing it caused:

  Uncaught SyntaxError: The requested module '@retejs/lit-plugin'
  does not provide an export named 'LitArea2D'

All four imports are now verified against the actual ESM export lists:
- `rete@2.0.4`:                NodeEditor, ClassicPreset
- `rete-area-plugin@2.0.3`:   AreaPlugin, AreaExtensions
- `rete-connection-plugin@2.0.3`: ConnectionPlugin, Presets
- `@retejs/lit-plugin@2.0.7`: LitPlugin, Presets

**Files changed:** `web/static/js/fsm-editor.js`

---

## [0.1.0-patched43] - 2026-03-14

### Added — Minimal FSM visual editor (Rete.js + Lit, no React/Node/npm)

A placeholder FSM node-graph editor accessible at `/fsm/editor`.

**Editor features (minimal placeholder):**
- Two default nodes (Start → End) connected by an edge
- "Add State" button — prompts for a name, adds a draggable node
- "Export JSON" — serialises current graph to JSON in a textarea
- Loads saved graph from `fsm_machines.graph_json` when editing an existing FSM

**Theme integration:**
- `fsm-editor.js` applies CSS custom properties to the editor container on
  load, matching the current Tailwind dark/light mode
- A `MutationObserver` on `<html>` keeps colours in sync when the user
  toggles dark mode while the editor is open
- Colours use Tailwind's indigo palette (indigo-500/400 for sockets and
  connections, gray-50/gray-900 backgrounds)

**Infrastructure:**
- `web/static/js/fsm-editor.js` — vanilla ES module, no build step
- `web/static/css/app.css` — CSS variable overrides for Rete node colours
- `internal/ui/layout.go`: `ExtraHead mi.H` added to `Config`; `FullHeight bool`
  added; `extraHead()` helper; `mainContentClass()` for zero-padding full-height
  pages; `ReteImportMap` now injectable via `ExtraHead`
- `internal/ui/handler.go`: `RenderWithConfig` allows per-page Config overrides
- `internal/ui/pages.go`: `FSMEditorData`, `FSMEditorPage`
- `internal/ui/routes.go`: `FSMEditorNew`, `FSMEditorEdit` handlers
- `internal/ui/router.go`: `GET /fsm/editor` and `GET /fsm/editor/{id}` routes
- `internal/ui/layout.go` `MainNav`: "FSM Editor" entry with `account_tree` icon

**Files changed:** `internal/ui/layout.go`, `internal/ui/handler.go`,
`internal/ui/pages.go`, `internal/ui/routes.go`, `internal/ui/router.go`,
`web/static/js/fsm-editor.js`, `web/static/css/app.css`

---

## [0.1.0-patched42] - 2026-03-14

### Added — Rete.js + Lit import map, configurable CDN vs vendored

Groundwork for the FSM rule editor using Rete.js with the Lit render plugin
(no React, no Node, no npm).

**`internal/ui/layout.go`**
- `UseVendoredJS bool` added to UI `Config`
- `jsURL(cdnURL, vendorFile)` helper on `Config` — returns CDN URL or
  `/static/vendor/<file>` depending on the flag
- `ReteImportMap(cfg Config) mi.H` — renders a `<script type="importmap">`
  for Rete.js + Lit; inject into the `<head>` of any page using the FSM
  editor. Keys are in a stable fixed order (no Go map non-determinism).
- Pinned deps: rete@2.0.4, rete-area-plugin@2.0.3,
  rete-connection-plugin@2.0.3, rete-render-utils@2.0.2,
  @retejs/lit-plugin@2.0.7, lit@3

**`internal/config/config.go`**
- `VendorJS bool` added to `FeatureConfig` (`"vendor_js"` in JSON)

**`internal/server/server.go`**
- `cfg.Features.VendorJS` wired through to `uiCfg.UseVendoredJS`

**`Makefile`**
- `make vendor-js` — downloads all six pinned ESM files from esm.sh to
  `web/static/vendor/` for offline / air-gap deployments
- `.PHONY` and `make help` updated

**Usage:**
- Default (CDN): nothing to do — importmap points at esm.sh
- Vendored: run `make vendor-js` once, then set `"vendor_js": true` in
  `data/shelf/config.json` (or restart with `SHELF_VENDOR_JS=true`)
- Switch back to CDN: set `"vendor_js": false` and restart

**Files changed:** `internal/ui/layout.go`, `internal/config/config.go`,
`internal/server/server.go`, `Makefile`

---

## [0.1.0-patched41] - 2026-03-14

### Fixed — First-run admin prompt not appearing after patched32

The admin creation prompt was gated on `FIRST_RUN == "true"`, which is only
set when `.secrets` doesn't exist. On any run after secrets were first
generated — even if `.admin_created` was never written — the prompt was
silently skipped, leaving the system with no admin account.

Fixed by removing the `FIRST_RUN` dependency. The condition is now simply
`[[ ! -f "$ADMIN_MARKER" ]]` — the prompt appears on every launch until
an admin is successfully created and the marker file written.

**Files changed:** `scripts/launch.sh`

---

## [0.1.0-patched40] - 2026-03-14

### Fixed — make launch was missing OLU_STORAGE_TYPE and other feature flags

`scripts/launch.sh` was starting olu without `OLU_STORAGE_TYPE=sqlite`,
so olu defaulted to the `jsonfile` backend. Also missing compared to
`make start-olu`: timeseries, strict commit, cascading delete, metrics,
log level. All feature flags are now consistent between `make launch`
and `make start-olu`.

**Files changed:** `scripts/launch.sh`

---

## [0.1.0-patched39] - 2026-03-14

### Fixed — Asset identity/state fields not stored or displayed on detail page

Three bugs:

1. **Wrong read key on detail page**: `AssetDetail` handler was reading
   `entity.Data["identity"]` but `AssetCreate`/`AssetUpdate` write to
   `entity.Data["identity_fields"]`. Fixed to read `identity_fields` and
   `state_fields`.

2. **State fields never shown**: `AssetDetail` struct had no `State` field
   and the detail page had no state fields card. Added `State map[string]interface{}`
   to `AssetDetail` and a "State Fields" card to `AssetDetailPage`.

3. **`assetStateContent` renderer missing**: Added alongside the existing
   `assetIdentityContent`.

**Files changed:** `internal/ui/asset_detail.go`, `internal/ui/routes.go`

---

## [0.1.0-patched38] - 2026-03-14

### Fixed — Platform asset mapping corrected

`darwin/amd64` does not have a release asset (all modern Macs are arm64).
`windows/amd64` is supported. Platform detection updated in both
`scripts/launch.sh` and `Makefile`:

Supported assets: `olu-linux-amd64`, `olu-darwin-arm64`, `olu-windows-amd64`

Windows is detected via `uname -s` patterns `MINGW*`, `CYGWIN*`, `MSYS*`
(Git Bash / Cygwin / MSYS2). `darwin/amd64` combination is explicitly
cleared so it falls through to build-from-source.

**Files changed:** `scripts/launch.sh`, `Makefile`

---

## [0.1.0-patched37] - 2026-03-14

### Changed — make olu and make launch try to download olu binary before building

Both `make olu` and `scripts/launch.sh` now attempt to download the latest
release binary from `ha1tch/xolu` before falling back to clone-and-build.

**How it works:**
- If `GH_TOKEN` or `GITHUB_TOKEN` is set, the GitHub API is queried for the
  latest release of `ha1tch/xolu`
- The host platform is detected (`uname -s` / `uname -m`) and the matching
  asset name is constructed (e.g. `olu-darwin-arm64`, `olu-linux-amd64`)
- The binary is downloaded directly using the asset API with Bearer auth
- On any failure (no token, unknown arch, asset not found, download error)
  the scripts fall through to the existing clone-and-build path without
  interrupting the workflow
- `--skip-build` / `SKIP_BUILD=true` still short-circuits everything if the
  binary already exists

**Auth:** set `GH_TOKEN` (or `GITHUB_TOKEN`) to a personal access token with
`repo` scope (read-only is sufficient for downloading release assets).

**Files changed:** `scripts/launch.sh`, `Makefile`

---

## [0.1.0-patched36] - 2026-03-14

### Fixed — Add Sensor returns 404; missing routes and handlers

`GET /sensors/new` was not registered. The router only had
`GET /sensors/{id}`, which matched `/sensors/new` with "new" as the ID,
failed to parse it as an integer (returned 0), and the handler returned
"Sensor not found".

Fix:
- `router.go`: registered `GET /sensors/new` and `POST /sensors` before
  `GET /sensors/{id}` so the literal route takes precedence
- `routes.go`: added `SensorNew` (GET) and `SensorCreate` (POST) handlers
- `pages.go`: added `SensorFormData`, `SensorFormPage`, and
  `sensorFormField` helper — form fields: external_id, type, driver, name

**Files changed:** `internal/ui/router.go`, `internal/ui/routes.go`,
`internal/ui/pages.go`

---

## [0.1.0-patched36] - 2026-03-14

### Fixed — Multiple missing routes and handlers; asset type dynamic fields on asset form

**Sensors:**
`GET /sensors/new` was not registered. Router only had `GET /sensors/{id}`
which caught `/sensors/new`, treated "new" as an ID, and returned 404.
Added `SensorNew` (GET) and `SensorCreate` (POST) handlers and registered
them before `GET /sensors/{id}`. Form fields: external_id, type, driver, name.

**Rules:**
`POST /rules` was not registered so the create form posted to the current
URL (`/rules/new`) which doesn't accept POST. Added `RuleCreate` (POST /rules)
and `RuleUpdate` (POST /rules/{id}) handlers. Fixed rule form to include
explicit `mi.Action` so it posts to `/rules` (create) or `/rules/{id}` (edit).

**Asset type dynamic fields on asset form:**
When creating or editing an asset, the user-defined identity and state fields
from the asset type were never shown. Fixed by:
- Added `IdentityFields`, `StateFields []FieldRow` and `FieldValues map[string]string`
  to `AssetFormData`
- Asset type selector now triggers HTMX `GET /assets/type-fields?type_id=N`
  which returns a partial with the dynamic fields for the selected type
- `AssetEdit` loads existing field values from `identity_fields` and
  `state_fields` on the asset document
- `AssetCreate` and `AssetUpdate` collect `identity.*` and `state.*` form
  values and persist them as nested maps in the asset document
- Added `AssetTypeFields` partial handler and registered `GET /assets/type-fields`

**Supporting changes:**
- Added `mi` (minty) import to `routes.go`
- Added `toIntFromString` and `toFloatFromString` helpers
- Scanned all `GET /{resource}/{id}` routes — no other missing `/new` cases

**Files changed:** `internal/ui/routes.go`, `internal/ui/router.go`,
`internal/ui/asset_detail.go`, `internal/ui/pages.go`

---

## [0.1.0-patched35] - 2026-03-14

### Fixed — Asset create/edit form posting to API instead of UI routes

The asset form was submitting to `/api/v1/assets` (the REST API), which
requires a Bearer JWT token. The browser only sends the session cookie, so
every submission returned 401 Unauthorized.

Asset type and user forms correctly post to `/asset-types` and `/admin/users`
(UI routes that use session auth). The asset form was the only one pointing
at the API directly.

Fix:
- `asset_detail.go`: form action changed from `/api/v1/assets` to `/assets`
  (create) and `/assets/{id}` (edit); method is POST for both
- `routes.go`: added `AssetCreate` (POST /assets) and `AssetUpdate`
  (POST /assets/{id}) UI handlers, using `h.olu.Create/Update` directly
- `router.go`: registered the two new routes with `requirePerm`
- `routes.go`: added missing `fmt` import

**Files changed:** `internal/ui/asset_detail.go`, `internal/ui/routes.go`,
`internal/ui/router.go`

---

## [0.1.0-patched34] - 2026-03-14

### Fixed — CSRF tokens missing from multiple POST forms

Systematic audit of all hand-rolled `b.Form(... post ...)` calls found
five page data structs missing a `CSRFToken` field, causing 403 on every
POST to those forms:

- `UserFormData` (`admin_users.go`) — user create/edit — **originally reported**
- `AdminPasswordResetData` (`admin_users.go`) — admin password reset
- `AssetFormData` (`asset_detail.go`) — asset create/edit
- `RuleFormData` (`pages.go`) — rule create/edit
- `SettingsData` (`pages.go`) — settings/general and settings/notifications

Fix applied uniformly to all five:
1. Added `CSRFToken string` field to each struct
2. Added `CSRFField(data.CSRFToken)(b)` to each form template
3. Added `CSRFToken: h.GetCSRFToken(w, r)` to every handler callsite
   (6 callsites total across the 5 structs)

`CSRFField()` already existed in `components.go`. The root cause was
hand-rolled `b.Form(...)` calls bypassing the `Form()` helper in
`forms.go` which injects the CSRF field automatically. Convention going
forward: every POST form data struct must have `CSRFToken string`; every
`b.Form(... post ...)` must open with `CSRFField(data.CSRFToken)(b)`.

**Files changed:** `internal/ui/admin_users.go`, `internal/ui/asset_detail.go`,
`internal/ui/pages.go`, `internal/ui/routes.go`

---

## [0.1.0-patched33] - 2026-03-14

### Fixed — shelf-admin user create prompts for missing email and name

`make admin-create` passes only `--role=admin`, not `--email` or `--name`,
so `shelf-admin user create` was failing immediately with "email is required".

Fixed in `userCreate`: if `--email` or `--name` are absent, prompt
interactively using the shared `stdinReader` rather than exiting with an
error. The command is now fully interactive when called without flags.

`make user-create` (no flags at all) and `make admin-create` (role flag
only) both work correctly now.

**Files changed:** `cmd/shelf-admin/main.go`

---

## [0.1.0-patched32] - 2026-03-14

### Fixed — Admin creation at first run (EOF on password prompt)

`cmd/shelf-admin/main.go`: `promptPassword` was creating a new
`bufio.NewReader(os.Stdin)` on every call. The first call buffered ahead
on the pipe and consumed both lines; the second call created a fresh reader
on an exhausted stdin and got EOF. Fixed by using a single package-level
`stdinReader`. The delete confirmation reader was the same bug — also fixed.

`scripts/launch.sh`: removed the password collection and piping from the
launch script entirely. The script now collects only email and name, then
calls `shelf-admin user create` directly, letting it prompt for the
password from the terminal. This is simpler and avoids the pipe problem.

**Files changed:** `cmd/shelf-admin/main.go`, `scripts/launch.sh`

---

## [0.1.0-patched31] - 2026-03-14

### Fixed — make olu and make start-olu restored after help rewrite

The Python script that rewrote the help target replaced everything from
`# ── Help ──` to end of file. The olu and start-olu targets happened to
live after the help target, so they were silently deleted. Restored both
targets and their variable definitions (OLU_REPO, OLU_DIR, OLU_DATA,
OLU_BIN).

**Files changed:** `Makefile`

---

## [0.1.0-patched30] - 2026-03-14

### Changed — make help redesigned as two-column layout

Help output reformatted from a single vertical list to a two-column layout
at 100 characters wide (48-char left column, separator, 49-char right column).

Left column: Launch & services, olu standalone, Data & housekeeping.
Right column: User management, Development, Distribution, Custom ports.

All content preserved; nothing dropped.

**Files changed:** `Makefile`

---

## [0.1.0-patched29] - 2026-03-14

### Fixed — shelf-admin OQL endpoint and response envelope

`cmd/shelf-admin/client.go`: OQL calls were hitting `/api/v1/oql` (wrong)
instead of `/api/v1/oql/query`. olu was treating the POST as an entity
create on a type named "oql" and returning 201.

Response envelope was also wrong: olu returns `{"data": [...], "stats": {}}`
but the client was decoding `{"rows": [...]}`. Fixed both.

**Files changed:** `cmd/shelf-admin/client.go`

---

## [0.1.0-patched28] - 2026-03-14

### Fixed — olu binary path corrected

xolu's Makefile places the binary at `xolu/olu`, not `xolu/bin/olu`.
`OLU_BIN` in Makefile and `OLU_BIN` in `scripts/launch.sh` both corrected.
Fallback `go build` output path in `launch.sh` also corrected.

**Files changed:** `Makefile`, `scripts/launch.sh`

---

## [0.1.0-patched27] - 2026-03-14

### Changed — make olu clones/builds only; make start-olu runs

`make olu` now clones (or updates) xolu via git and builds the binary.
It does not start olu.

`make start-olu` starts a previously built olu with all features enabled.
Fails with a clear message if the binary doesn't exist yet.

**Files changed:** `Makefile`

---

## [0.1.0-patched26] - 2026-03-14

### Fixed — make olu is self-contained; git protocol throughout

`make olu` / `make start-olu` now clone, build, and run olu entirely on
their own — no dependency on `make launch`. They clone xolu using the git
protocol (`git@github.com:ha1tch/xolu.git`), build with `make build` or
`go build` as available, then start olu with all features enabled.

`scripts/launch.sh` also updated to clone via git protocol instead of https.

**Files changed:** `Makefile`, `scripts/launch.sh`

---

## [0.1.0-patched25] - 2026-03-14

### Added — make olu / make start-olu targets

`Makefile` updated with `olu` and `start-olu` targets (aliases of each
other) that start olu standalone with all features enabled:

- `OLU_STORAGE_TYPE=sqlite` — SQLite backend (not the default jsonfile)
- `OLU_GRAPH_MODE=flat` — graph enabled
- `OLU_FULLTEXT_ENABLED=true` — full-text search
- `OLU_TIMESERIES_ENABLED=true` — Pebble-backed timeseries
- `OLU_TS_RETENTION_ENABLED=true` — background retention sweep
- `OLU_TS_COMPRESSION=zstd` — timeseries compression
- `OLU_TENANT_MODE=path`, `OLU_TENANT_AUTO_REGISTER=true`
- `OLU_STRICT_COMMIT=true`, `OLU_CASCADING_DELETE=true`
- `OLU_METRICS_ENABLED=true`

Overridable: `OLU_DIR`, `OLU_DATA`, `OLU_PORT`. Requires xolu already
built (`make launch` handles this). `make help` updated with olu section.

**Files changed:** `Makefile`

---

## [0.1.0-patched24] - 2026-03-14

### Fixed — Module path corrected to match actual repository location

`go.mod` declared `module github.com/ha1tch/shelf` but the project lives
inside `github.com/ha1tch/sabremania/shelf/app`. All internal imports
used the wrong module path, causing build failures when building outside
the container.

Updated `go.mod` and all 34 Go source files to use the correct module path:
`github.com/ha1tch/sabremania/shelf/app`.

Build and full test suite confirmed passing with corrected module path.

**Files changed:** `go.mod`, 34 `.go` source files

---

## [0.1.0-patched24] - 2026-03-14

### Fixed — Wrong module path throughout codebase

All 34 Go source files and `go.mod` were using
`github.com/ha1tch/sabremania/shelf/app` as the module path instead of
`github.com/ha1tch/shelf`. This caused shelf to be tangled with the
sabremania project when resolved from a shared parent directory, and
caused `go mod tidy` run from scratch to pull in
`golang.org/x/crypto v0.49.0` (requires Go 1.25) instead of the pinned
v0.21.0.

Fixed with global replacement across all affected files. Module path is
now consistently `github.com/ha1tch/shelf` everywhere.

`release.sh` updated to include `go.mod` and `go.sum` in checkpoint zips
so the correct pinned dependencies ship with the source.

**Files changed:** all 34 `.go` files with internal imports, `go.mod`,
`release.sh`

---

## [0.1.0-patched23] - 2026-03-14

### Fixed — Root cause found: release.sh was deleting Makefile

`release.sh` runs `find . -maxdepth 1 -type f` after `go build` to remove
any stray binaries. The exclusion list covered `*.sh`, `*.md`, `*.json`,
etc., but not extensionless files — so `Makefile` was being silently deleted
on every release. Fixed by adding `! -name "Makefile"` to the exclusion
list.

This explains why the Makefile was absent from every checkpoint zip since
it was first added: it was written correctly, survived the build and test
steps, then deleted by the cleanup find before the zip was created.

**Files changed:** `release.sh`, `Makefile` (restored)

---

## [0.1.0-patched22] - 2026-03-14

### Fixed — Makefile restored and confirmed in release zip

Makefile was absent from the last several checkpoint zips despite being
listed in release.sh ZIP_SOURCES, because the [ -e "$src" ] filter was
silently omitting it when the file was missing. Restored from screen
capture and verified present before release. Build, tests, and make help
all confirmed working.

**Files changed:** `Makefile`

---

## [0.1.0-patched21] - 2026-03-14

### Fixed — Makefile now reliably included in release zip

Root cause identified: the Makefile was being written via shell heredoc
(`cat > Makefile << 'EOF'`) which does not persist between tool calls in
this environment. All other files (scripts, Go source, zips) were written
via the `create_file` tool which writes to persistent storage. Makefile
recreated with `create_file`. Confirmed present on disk before cutting
this release. Zip verified to contain `Makefile` and `scripts/`.

**Files changed:** `Makefile` (recreated with persistent write)

---

## [0.1.0-patched20] - 2026-03-14

### Fixed — Makefile now actually included in release zip

`Makefile` was listed in `release.sh`'s `ZIP_SOURCES` but the container
was dropping it between patches. Recreated and confirmed present before
cutting this release. Verified zip contains `Makefile` and `scripts/`.

**Files changed:** `Makefile` (recreated)

---

## [0.1.0-patched19] - 2026-03-14

### Added — Full user management via make subcommands

`Makefile` updated with complete user management targets:

  make admin-create                               — create admin (shorthand)
  make user-create                                — create any role
  make user-list                                  — list all users
  make user-disable        EMAIL=x@example.com
  make user-enable         EMAIL=x@example.com
  make user-set-role       EMAIL=x@example.com ROLE=manager
  make user-set-password   EMAIL=x@example.com
  make user-delete         EMAIL=x@example.com

All targets delegate to `bin/shelf-admin`, auto-build it if absent, and
accept `OLU_URL=http://host:port` override. Error messages include correct
usage when EMAIL or ROLE are missing.

`make help` updated to document all user management targets with role list
and OLU_URL override note.

**Files changed:** `Makefile`

---

## [0.1.0-patched18] - 2026-03-14

### Added — make help, admin-create, first-run admin prompt

`Makefile` expanded with a proper help target covering all workflows
(getting started, user management, development, distribution, custom ports,
data layout). Added `admin-create` target which invokes `bin/shelf-admin
user create --role=admin` interactively.

`make build` now builds both `bin/shelf` and `bin/shelf-admin`.

`scripts/launch.sh` updated: on first run (before `data/.secrets` exists),
after olu is up and healthy, prompts interactively for admin email, name,
and password, then calls `bin/shelf-admin user create --role=admin`.
Skipped on subsequent runs (presence of `data/.admin_created` marker).
`--no-admin` flag suppresses the prompt. Gracefully warns and continues
if admin creation fails rather than aborting the launch.

`cmd/shelf-admin` was already implemented with full user management
(`create`, `list`, `get`, `disable`, `enable`, `set-role`, `set-password`,
`delete`) — it just wasn't wired into the launcher or Makefile until now.

**Files changed:** `Makefile`, `scripts/launch.sh`

---

## [0.1.0-patched17] - 2026-03-14

### Fixed — Makefile and bundle script properly integrated

`Makefile` and `scripts/bundle.sh` added (Makefile was lost to the
container between patches; `scripts/bundle.sh` extracted from the Makefile
to avoid Python one-liner escaping issues with make line continuations).

`scripts/` added to `release.sh` zip sources so `scripts/launch.sh` and
`scripts/bundle.sh` are included in checkpoint zips going forward.

`make bundle` now delegates cleanly to `scripts/bundle.sh`. Verified zip
structure: `launch.sh` at root, `scripts/launch.sh` and `scripts/bundle.sh`
inside, no binaries or data.

**Files added/fixed:** `Makefile`, `scripts/bundle.sh`, `release.sh`

---

## [0.1.0-patched17] - 2026-03-14

### Fixed — Documentation review and sync

`README.md` rewritten: corrected olu port (`9000` → `9090`), replaced
non-existent `make dev` with `make launch`/`make launch-dev`/`make bundle`,
corrected env var table (`SHELF_OLU_URL` → `OLU_URL`), added FSM and
asset-type endpoints, updated copyright year to 2026, removed inaccurate
claims.

`DEPLOY.md` rewritten: corrected env var table (`OLU_URL`, removed
non-existent `SHELF_TENANT_ID`), removed non-existent `/health/ready`
endpoint (only `/health` exists), removed unimplemented metrics section
(replaced with honest "planned but not yet implemented" note), removed
non-existent `"tenant"` config block, updated olu version to `v0.9.7+`,
added `make launch` section, removed `make dev` reference.

`ROADMAP.md` updated: version corrected from patched7 to patched17, date
updated to March 2026, patched8–patched17 history added, FSM work and
scripts added to completed section, pending items updated to reflect
current state including OQL-as-action-executor and ual compute blocks.

**Files changed:** `README.md`, `DEPLOY.md`, `ROADMAP.md`

---

## [0.1.0-patched16] - 2026-03-14

### Added — Makefile and launch script

`Makefile` added at project root with targets: `build`, `test`, `vet`,
`launch`, `launch-dev`, `launch-fast`, `bundle`, `release`, `clean`, `help`.

`scripts/launch.sh` added: clones/updates `github.com/ha1tch/xolu` from
GitHub, builds both olu and Shelf AMS, generates secrets and config on
first run, starts both services with health-check polling, and stops both
cleanly on Ctrl-C. Configurable via flags or environment variables.

`make bundle` produces `shelf-ams-VERSION-bundle.zip` containing the full
shelf source tree plus a top-level `launch.sh` wrapper. The recipient
unzips and runs `./launch.sh` — no olu source required in the zip.

`make clean` removes `bin/`, coverage output, and all `*-bundle.zip` and
`*-checkpoint.zip` artefacts.

Removed stray `shelf-ams-launch.zip` left in outputs from earlier session.

**Files added:** `Makefile`, `scripts/launch.sh`

---

## [0.1.0-patched15] - 2026-03-14

### Added — Provisional FSM architecture simplification (24h review)

`docs/FSM_DESIGN.md` updated to v1.4.

Added section 5.6.1a (provisional, 24h): OQL-as-action-executor.

Key insight: olu's OQL layer is already tsqlparser under the hood. This
means the executor needs no action parser or evaluator of its own:

- Guards: evaluated locally via `SELECT @FSM WHERE <expr>` (tsqlparser,
  no olu round-trip, fast, boolean result only)
- Actions: OQL statements (`UPDATE assets SET ...`, `INSERT INTO ...`)
  with context variables interpolated by the executor, then wrapped into
  a single `olu.Commit` call

Eliminates the custom action vocabulary (`asset.set`, `asset.increment`,
etc.) and any action expression evaluator from the executor entirely. Full
OQL expression power — strings, dates, arithmetic, subqueries — is already
implemented and tested in olu at 96% coverage.

Also documented: the two architectures (OQL-as-action-executor and compiled
ual closure fast path for numeric guards) are not mutually exclusive.

Added open question 7.9: confirm olu Commit accepts OQL-derived mutations
or requires structured field maps; pending a response on ual IN
support and pkg/compute extraction.

Marked provisional pending a response within 24 hours.

**Files changed:** `docs/FSM_DESIGN.md`

---

## [0.1.0-patched14] - 2026-03-13

### Added — tsqlparser decision documented in FSM design

`docs/FSM_DESIGN.md` updated to v1.3:

Section 5.3 now opens with an explicit decision callout: guard expression
language is T-SQL via tsqlparser (settled). Includes a concise comparison
table of CEL vs T-SQL across all guard patterns, with the rationale:
T-SQL is more readable for date/duration/calendar patterns that dominate
the target domains; CEL's collection aggregate advantage is covered by
EXISTS subquery; CEL adds a Go 1.23+ requirement and a new dependency for
no substantive gain.

Formal language definition added: any expression that parses without errors
via `SELECT @FSM WHERE <expr>` in tsqlparser v0.5.2.

Section 4.2 updated to note that from fsm-toolkit's perspective `guard` is
simply an opaque string — the toolkit has no involvement in the expression
language.

Open questions 7.1 and 7.3 both closed.

**Assessment:** the fsm-toolkit team does not need to know about the T-SQL
decision. The feature request already states the `guard` field is an opaque
string, and disclosing the expression language would only invite scope creep
in their implementation. The decision lives in `FSM_DESIGN.md` only.

**Files changed:** `docs/FSM_DESIGN.md`

---

## [0.1.0-patched13] - 2026-03-13

### Added — Guard expression language resolved: tsqlparser + SELECT @FSM WHERE

`docs/FSM_DESIGN.md` updated to v1.2:

Closed open question 7.1. Guard expressions are T-SQL-compatible boolean
expressions parsed via `github.com/ha1tch/tsqlparser` using the sentinel
wrapper `SELECT @FSM WHERE <expr>`. The WHERE clause expression (`sel.Where`)
is extracted and evaluated against the `EvaluationContext` by an AST walker.

The `@FSM` sentinel is named rather than a bare literal so that log output
and debug traces are self-identifying and the parse site is grep-able.

Added section 5.3.2: full `ParseGuardExpression` function, rationale for
the sentinel name, and an AST dispatch table covering all node types needed
for the guard taxonomy (`InfixExpression`, `PrefixExpression`, `InExpression`,
`BetweenExpression`, `IsNullExpression`, `ExistsExpression`,
`QualifiedIdentifier`, `FunctionCall`, literals). Collection aggregate
pattern resolved: use `EXISTS (SELECT 1 FROM value.array alias WHERE ...)`.

Every pattern in the guard taxonomy (section 5.3.1) confirmed to parse
correctly via `SELECT @FSM WHERE` — tested against tsqlparser v0.5.2.

`docs/FSM_TOOLKIT_FEATURE_REQUEST.md` updated to note that the guard
expression language is settled on the Shelf side, confirming to the toolkit
team that the `guard` string is fully opaque to them.

**Files changed:** `docs/FSM_DESIGN.md`, `docs/FSM_TOOLKIT_FEATURE_REQUEST.md`

---

## [0.1.0-patched12] - 2026-03-13

### Added — FSM documents updated and feature request written

**`docs/FSM_DESIGN.md`** updated to v1.1:

- Section 2 domain table extended with soda vending machine, adding example
  guard and action columns throughout
- Section 3.2 execution model updated to describe the Commit-as-executor-primitive
  pattern (accumulate all action contributions into a single `olu.Commit`)
- Section 3.5 gap list extended with item 7: no transition actions
- Section 4.2 normalised JSON notes added re pending `guard`/`actions` fields
- Section 4.4 `ShelfValidate` extended with action vocabulary check
- Section 5.3.1 guard taxonomy extended with inventory check pattern
- Section 5.4 bridge spec updated to mention `Transition.Actions []string`
- Section 5.6 added: Transition actions — Commit-as-executor-primitive model,
  action vocabulary (7 verbs), action expression context, outbox pattern for
  notifications, vending machine action example
- Section 5.7 added: Parallel machines — bundle as packaging convention,
  parallel execution in the Shelf executor, cross-machine interaction model
- Section 6 domain examples table extended with vending machine bundle entry
- Section 7 open questions updated: 7.4 now mentions both `guard` and `actions`;
  7.6 resolved; 7.7 and 7.8 added (Commit limit, outbox worker design)

**`docs/FSM_TOOLKIT_FEATURE_REQUEST.md`** created:

Abstract feature request to the fsm-toolkit team with no Shelf domain
specifics. Three items:

1. `Guard string` on `Transition` — opaque boolean predicate; new
   `mixed_guards` warning in `Analyse()`; vending machine as canonical
   motivating example
2. `Actions []string` on `Transition` — ordered list of opaque effect
   expressions; rationale for list vs single string; batch execution model
3. Clarification question on whether unlinked machines in a bundle are
   an intended and supported use of the format

**Files changed:**
`docs/FSM_DESIGN.md`, `docs/FSM_TOOLKIT_FEATURE_REQUEST.md` (new)

---

## [0.1.0-patched11] - 2026-03-13

### Added — FSM design document

`docs/FSM_DESIGN.md` — authoritative design document covering the full
FSM handling strategy for Shelf AMS. Sections:

1. Purpose and scope
2. What FSMs model — four asset domains with example conditions
3. Current implementation — storage format, execution pipeline,
   condition types, and documented gaps
4. fsm-toolkit integration — validation pipeline, `Validate()`/`Analyse()`
   gates, normalised JSON format, Shelf-specific validation
5. Target design — storage format migration, `shelf_state` class schema
   for per-state domain metadata, transition guard design, bridge
   function spec, `.fsm` upload endpoint
6. Domain examples — reference machines for fridge, fire extinguisher,
   cattle, and HGV
7. Open questions — expression language, collection aggregates, upstream
   feature request, bundle support
8. Related documents

This document is the upstream source for the forthcoming
`docs/FSM_TOOLKIT_FEATURE_REQUEST.md`.

**Files changed:** `docs/FSM_DESIGN.md` (new)

---

## [0.1.0-patched10] - 2026-01-11

### Added — Asset Type Editor

Implements the full create/view/edit/delete UI for asset types. The list
page already existed; everything else was missing.

**New pages:**

`AssetTypeFormPage` — a single form used for both Create and Edit. Fields:
name, slug (create only), description, colour picker, connectivity tier
checkboxes (qr/gateway/lora/gps), and two dynamic field sections
(Identity Fields and State Fields). Each field section supports adding and
removing rows via JavaScript with key, label, type selector, and required
checkbox. The field type set matches the validation schema: text, number,
boolean, date, datetime, select, ref.

`AssetTypeDetailPage` — shows name, slug, description, asset count; links
to Edit and provides an inline Delete form with confirmation dialog. Delete
is refused server-side if any assets use the type (409 response).

**New handlers (7):**

- `AssetTypeNew` — `GET /asset-types/new`
- `AssetTypeCreate` — `POST /asset-types`
- `AssetTypeDetail` — `GET /asset-types/{id}`
- `AssetTypeEdit` — `GET /asset-types/{id}/edit`
- `AssetTypeUpdate` — `POST /asset-types/{id}`
- `AssetTypeDelete` — `POST /asset-types/{id}/delete`

**New helpers:**

`parseFieldsFromForm` — converts multi-value form arrays into the
`identity_template` / `state_template` map structure expected by olu.

`extractFieldRows` — converts a template `map[string]any` back into
`[]FieldRow` for pre-populating the edit form.

All routes registered in `router.go` under `requireAuth`.

**Files changed:**
`internal/ui/pages.go`, `internal/ui/routes.go`, `internal/ui/router.go`

---

## [0.1.0-patched9] - 2026-01-10

### Fixed — Login/Logout UI: CSRF tokens missing from auth forms

The Login/Logout UI was substantially complete — `LoginPage`, `LoginSubmit`,
`Logout`, `ForgotPasswordPage`, `ForgotPasswordSubmit`, `ResetPasswordPage`,
and `ResetPasswordSubmit` are all implemented and wired in the server, with
rate limiting, audit logging, session creation, and CSRF validation active.

**The gap:** the three auth forms did not inject the `_csrf` hidden field.
The CSRF manager is configured and validates `_csrf` on every POST request,
so login, forgot-password, and reset-password would all fail CSRF validation
in production when `csrf_enabled: true`.

**Fixed — CSRF token injection in `LoginPageData`**

Added `CSRFToken string` field. `LoginPage` handler now calls
`h.GetCSRFToken(w, r)` and passes the token. `LoginPage` function injects
`<input type="hidden" name="_csrf" value="…">` into the form.

**Fixed — CSRF token injection in `ForgotPasswordData`**

Same fix. `ForgotPasswordPage` handler passes the token. `forgotPasswordForm`
injects the hidden field as the first child of the form.

**Fixed — CSRF token injection in `ResetPasswordData`**

Same fix. `ResetPasswordPage` handler passes the token (only when a valid
token is found; invalid/expired paths render without a form). Hidden `_csrf`
field injected after the existing `token` field.

**Fixed — hardcoded version in `server.go`**

`uiCfg.Version` was hardcoded as `"0.1.0"`. Replaced with
`appversion.Version` from `internal/version`.

**Files changed:**
`internal/ui/login.go`, `internal/ui/password_reset.go`,
`internal/ui/routes.go`, `internal/server/server.go`

---

## [0.1.0-patched8] - 2026-01-09

### Fixed — event ingestion: nil Data panic and non-atomic FSM transitions

**Fixed — nil Data panic in `processEvent` (introduced in patched1)**

`Ingest` and `IngestBatch` were passing the `*Entity` returned by
`client.Create` directly to `processEvent`, which reads `event.Data["asset_id"]`
to look up the associated asset. Since patched1, `Create` returns an entity with
nil `Data` (olu does not echo the document on creation). This caused a nil map
dereference in `processEvent` — a silent no-op at best, a goroutine panic at worst.

Fix: both handlers now re-fetch the event document via `client.Get` after `Create`
before passing it to `processEvent`. `Ingest` responds with `{"id":N}` and skips
processing if the re-fetch fails (non-fatal). `IngestBatch` skips the goroutine
for that entry silently.

**Changed — `evaluateFSM` now uses `Commit` for atomic state transitions**

`evaluateFSM` previously called `client.Patch` to update `assets.fsm_state` as a
standalone write. A failure on either side of the state-transition + audit-trail
pair left the system permanently inconsistent: state advanced with no history entry,
or a history entry existed with no corresponding state change.

Replaced with `client.Commit`, which executes the upsert of the asset FSM state
and the insert of the `fsm_transitions` audit record in a single storage
transaction. The audit record captures `asset_id`, `from_state`, `to_state`,
`event_id`, and `timestamp`.

**Files changed:** `internal/handlers/api/events.go`

---

## [0.1.0-patched7] - 2026-01-08

### Added — regression tests for patched1 re-fetch fixes

Three regression test files guard against re-introduction of the patched1
silent data-loss bugs, where service methods called entity-mapping functions
on Create/Update return values that have nil Data.

**`internal/alerts/regression_test.go`**

`TestRegression_Create_RefetchPopulatesAlert`: creates an alert against a
mock olu server and asserts that the returned `*Alert` has populated `ID`,
`Title`, `Severity`, and `Status`. Before patched1 all would be zero/empty
because `entityToAlert` was called on the `Create` response (nil Data). The
mock serves both the Create (`POST`) and the re-fetch (`GET`) endpoints, so
removing the `Get` call causes a 404 and the test fails.

**`internal/connectivity/regression_test.go`**

`TestRegression_RegisterSensor_RefetchPopulatesSensor`: same pattern for
`RegisterSensor`. Asserts `ID`, `ExternalID`, `Name`, and `Type` are
populated. Before patched1 all would be zero/empty.

**`internal/handlers/api/qr_regression_test.go`**

`TestRegression_QRHandler_DisableRefetchesDocument`: calls `Disable` against
a mock olu server that serves OQL lookup, Update, and Get endpoints. Asserts
that the response body contains a non-empty `url` field and `status=disabled`.
Before patched1 the handler read `entity.Data["url"]` from the Update
response (nil Data), producing an empty URL or a nil-map panic.

**Files changed:**
`internal/alerts/regression_test.go` (new),
`internal/connectivity/regression_test.go` (new),
`internal/handlers/api/qr_regression_test.go` (new)

---

## [0.1.0-patched6] - 2026-01-07

### Fixed — Stage 6: test suite repair and coverage

Completes the olu client remediation plan
(`docs/OLU_CLIENT_REMEDIATION.md`). All six stages are now implemented.

**Added — `TestPatch`**

Verifies PATCH method, correct path, and that `Entity.Data` is nil after
a patch (consistent with Create and Update behaviour).

**Added — `TestError`**

Verifies the `Error.Error()` string format for both the message-present
and message-absent cases.

**Coverage result: `internal/olu` at 85.1%** (target: ≥ 85%)

The `Sulpher` deprecated alias remains at 0% — intentionally excluded as
it is a one-line wrapper of `GraphQuery` with a deprecation notice.

**Updated — `docs/OLU_CLIENT_REMEDIATION.md`**

Status updated to Complete. Stage table updated with actual version
numbers. Outcomes section added noting the queryfy upgrade required no
source changes and the final coverage result.

**Final coverage summary across all packages:**

| Package | Coverage |
|---|---|
| `internal/olu` | 85.1% ✓ (was 74.2%) |
| `internal/handlers/middleware` | 95.7% |
| `internal/eventbus` | 93.8% |
| `internal/qrcode` | 91.5% |
| `internal/i18n` | 86.4% |
| `internal/config` | 82.3% |
| `internal/fsm` | 79.2% |
| `internal/notify` | 75.3% |
| `internal/webhook` | 74.1% |
| `internal/rules` | 71.0% |
| `internal/validation` | 68.2% |
| `internal/auth` | 56.0% |
| `internal/tenant` | 36.2% |
| `internal/reports` | 31.5% |
| `internal/connectivity` | 26.8% |
| `internal/alerts` | 27.9% |
| `internal/handlers/api` | 16.5% |
| `internal/server` | 0.0% (no tests — tracked in ROADMAP) |
| `internal/ui` | 0.0% (no tests — tracked in ROADMAP) |

**Files changed:**
`internal/olu/client_test.go`, `docs/OLU_CLIENT_REMEDIATION.md`

---

## [0.1.0-patched5] - 2026-01-06

### Added — Stage 5: new capabilities

**Added — `Save` (idempotent upsert by caller-specified ID)**

`Save(ctx, entity, id, data) (created bool, err error)`

Calls `POST /{entity}/save/{id}`. Returns `created=true` when a new
record was inserted, `false` when an existing record was replaced.
Use case: idempotent device registration where the caller controls the
ID (e.g. hardware serial number as the entity ID).

**Added — `Commit` (atomic upsert + append)**

`Commit(ctx, CommitRequest) (*CommitResult, error)`

Calls `POST /oql/commit`. Executes one conditional upsert (`Update`) and
one or more unconditional inserts (`Append`) in a single storage
transaction. If any operation fails the entire commit is rolled back.

New types: `CommitRequest`, `CommitUpdate`, `CommitAppend`, `CommitResult`.

`CommitUpdate.Version *int` enables optimistic concurrency — the write
proceeds only if the stored `_version` matches. A mismatch returns a
wrapped `*Error` with `StatusCode 409`. `nil` version = unconditional.

`CommitAppend.ID *int64` is optional; `nil` = auto-assign. An explicit
ID that already exists returns an error and rolls back the commit.

Maximum 25 entries in `Append` — enforced server-side (400 if exceeded).

Primary use case: atomic FSM state-transition write + audit-trail append.
The event-ingestion handler currently makes two independent HTTP calls;
it can now use `Commit` to ensure both land together or not at all.
Refactoring the handler itself is deferred to a subsequent feature branch.

**Updated — test suite**

`TestSave`: two subtests — `created` (new record) and `replaced` (existing
record); verifies POST method, `/save/{id}` path, and `created` bool.

`TestCommit`: two subtests — `success` (verifies full round-trip including
`Update.Version` and `Appended` slice) and `version_conflict` (verifies
409 response surfaces as `*Error` with correct `StatusCode`).

**Files changed:**
`internal/olu/client.go`, `internal/olu/client_test.go`

---

## [0.1.0-patched4] - 2026-01-05

### Changed — Stage 4: dependency upgrade

Upgraded `queryfy` from v0.2.0 to v0.3.1. No source changes required —
the APIs used by `internal/validation/schemas.go` (`Validate`,
`ValidateWithMode`, `Loose`, `Schema`) are unchanged. The upgrade brings
JSON Schema interoperability, schema compilation (25–46% performance
improvement on repeated validation), wildcard queries, and async
validation support for future use.

**Files changed:** `go.mod`, `go.sum`

---

## [0.1.0-patched3] - 2026-01-04

### Fixed — Stage 3: API correctness

**Fixed — graph endpoints (all three were broken)**

All graph methods were calling the wrong HTTP method, wrong URL path, or
both. Every call would have received a 404 or 405 from current olu.

`GraphNeighbors`: changed from `GET /graph/neighbors/{id}?depth=N` to
`POST /graph/neighbors` with JSON body `{"node_id":"…","direction":"out|in|both"}`.
Signature updated to accept `direction string` instead of `depth int`.
Returns `*NeighborResult` with `Outgoing` and `Incoming` maps
(node ID → relationship label) instead of a flat `[]string`.

`GraphShortestPath`: changed from `GET /graph/shortest-path?from=…&to=…`
to `POST /graph/shortestPath` with JSON body. Signature gains `maxDepth int`
(0 = server default). Returns `*PathResult` with `Exists`, `Path`, and
`Length` fields instead of bare `[]string`; callers can distinguish
"no path" from "error".

`GraphQuery` (formerly `Sulpher`): renamed from `Sulpher` to `GraphQuery`.
Endpoint changed from `POST /graph/sulpher` to `POST /graph/query`. Request
body gains optional `max_depth` field. Response key corrected from `"data"`
to `"result"` — the olu graph/query response uses `"result"`, not `"data"`.
`Sulpher()` is retained as a zero-depth wrapper alias; marked deprecated.

**Added — result types**

`PathResult` — returned by `GraphShortestPath`.
`NeighborResult` — returned by `GraphNeighbors`.
`GraphQueryResult` — returned by `GraphQuery` (replaces `SulpherResult`).
`GraphQueryStats` — replaces `SulpherStats`.
`SulpherResult` and `SulpherStats` are now type aliases pointing to the
new types for backward compatibility.

**Fixed — tenantID hex formatting**

Added `WithTenantID(id uint16) ClientOption` which formats the ID as
4-digit uppercase hex (e.g. `1` → `"0001"`, `255` → `"00FF"`), matching
olu's internal `XXXX@` node-ID prefix format. `WithTenant(string)` is
unchanged for callers that supply a pre-formatted string.

**Updated — call site in assets.go**

`AssetHandler.GetRelated` updated from `client.Sulpher()` to
`client.GraphQuery()` with explicit depth, and from `result.Data` to
`result.Result`.

**Updated — test suite**

`TestSulpher` renamed to `TestGraphQuery`; mock updated to
`/graph/query` path; response uses `"result"` key; `max_depth` body
field assertion added.

`TestGraphNeighbors` added: verifies POST method, `/graph/neighbors`
path, `node_id`/`direction` body fields, and `Outgoing`/`Incoming` map
parsing.

`TestGraphShortestPath` added: two subtests — path found (verifies
`Exists`, `Length`, `Path`) and no path (verifies `Exists == false`).

`TestWithTenantID` added: verifies `uint16(1)` formats as `"0001"` and
`uint16(255)` formats as `"00FF"` in the tenant URL segment.

**Files changed:**
`internal/olu/client.go`, `internal/olu/client_test.go`,
`internal/handlers/api/assets.go`

---

## [0.1.0-patched2] - 2026-01-03

### Fixed — Stage 2: auth and connectivity

**Fixed — olu authentication support**

Shelf had no way to authenticate with olu. Any olu instance configured
with `OLU_AUTH_TYPE=bearertoken`, `apikey`, or `jwt` would reject every
request from Shelf with 401.

- `OLUConfig` gains `APIKey string` (`json:"api_key"`).
- `Client` gains `apiKey` field, `WithAPIKey(key string)` option.
- `do()` sets `Authorization: Bearer <key>` when a key is configured.
- `WithTenantContext` propagates `apiKey` to the derived client.
- `server.go` passes the key from config to the client constructor.
- `config.example.json` documents the `olu.api_key` field.
- Set via `SHELF_OLU_API_KEY` environment variable.

**Fixed — full-text search endpoint**

`Search()` was calling `GET /{entity}/search?q=…` — a path that does not
exist in olu. The correct endpoint is `GET /search?q=…&entity={optional}`.

- Endpoint changed to `/search`.
- `SearchParams` gains `Entity string` field; the positional `entity`
  argument is preserved for compatibility but `SearchParams.Entity` takes
  precedence when both are set.
- Response unmarshal updated: olu returns
  `{"query":"…","entity":"…","count":N,"results":[flat docs…]}`.
- `AssetHandler.Search` updated to pass `Entity: "assets"` in `SearchParams`.

**Updated — test suite**

`TestSearch` in `client_test.go`: mock updated to `/search` path with
entity query param assertion; response updated to search envelope shape;
entity and name assertions added.

`TestAuthHeader` added to `client_test.go`: verifies `Authorization`
header is absent without a key and set to `Bearer <key>` with one.

`TestAssetHandler_Search` in `assets_test.go`: mock updated from
`GET /api/v1/assets/search` to `GET /api/v1/search` with entity param
assertion and search envelope response.

**Files changed:**
`internal/olu/client.go`, `internal/olu/client_test.go`,
`internal/config/config.go`, `internal/server/server.go`,
`internal/handlers/api/assets.go`,
`internal/handlers/api/assets_test.go`, `config.example.json`

---

## [0.1.0-patched1] - 2026-01-02

### Fixed — Stage 1: olu client foundation

Addresses the most severe class of defects in the olu client: all entity
responses were silently returning empty data due to incorrect response
unmarshalling, and the `OQLResult` type was duplicated across two packages
with different field names, requiring a manual bridge.

**Fixed — entity response unmarshalling (critical, data-loss)**

All five mutating operations were unmarshalling olu responses into structs
expecting a `{"data":{…}}` wrapper that olu does not produce.

- `Create`: olu returns `{"message":"…","id":N}`. The document is not
  echoed. `Entity.Data` is now documented as nil after creation; callers
  that need the document must call `Get` with the returned ID.
- `Get`: olu returns the flat document directly. The full document is now
  stored in `Entity.Data`; timestamps are extracted from the document fields.
- `Update` / `Patch`: olu returns `{"message":"…"}`. `Entity.Data` is nil;
  callers that need the updated document must re-fetch.
- `List`: olu returns a `PagedResponse` envelope
  `{"data":[…],"pagination":{…}}`. Introduced `ListResult` type carrying
  both the entity slice and pagination metadata (`Page`, `PerPage`,
  `TotalItems`, `TotalPages`). Pagination parameters remapped: `Limit` →
  `per_page`, `Offset` converted to `page` number.

**Fixed — re-fetch after Create at three call sites**

`alerts.Create`, `connectivity.CreateSensor`, and `connectivity.CreateBinding`
all called `entityToAlert`/`entityToSensor`/`entityToBinding` on the entity
returned by `Create`, which had nil `Data`. Each now re-fetches via `Get`
using the returned ID before mapping to the domain type.

**Fixed — re-fetch after Update in `qr.go`**

`QRHandler` was reading `entity.Data["url"]` after an `Update` call. Now
re-fetches the document before augmenting it with the generated URL.

**Fixed — `OQLResult` type unification**

Deleted the duplicate `auth.OQLResult{Rows}` type. Updated `OLUClient`
interface in `internal/auth/store_olu.go` to use `*olu.OQLResult` directly.
Removed the manual `Rows: result.Data` bridge in `internal/server/server.go`.
All `.Rows` references in `store_olu.go` updated to `.Data`.

**Updated — all 5 handler `List` call sites**

`asset_types`, `assets`, `fsm`, `rules`, `sensors` handlers updated to
consume `*ListResult` and pass `result.TotalItems` (server-side total) to
`NewListResponse` rather than `len(result)` (page count only).

**Updated — audit store `List` call site**

`internal/auth/audit_store.go` updated to range over `result.Entities`.

**Updated — test suite**

`internal/olu/client_test.go`: `TestCreate`, `TestGet`, `TestUpdate`,
`TestList` rewritten to reflect real olu response shapes. `TestCreate` and
`TestUpdate` now assert `Data == nil`. `TestGet` mock returns a flat
document. `TestList` mock returns a `PagedResponse` envelope with pagination
metadata; test verifies both entity count and `TotalItems`.

`internal/handlers/api/router_test.go`: four mock olu servers returning
bare `[]map[string]any{}` updated to return `PagedResponse` envelopes.

`internal/handlers/api/assets_test.go`: `TestAssetHandler_List` mock
updated to return `PagedResponse` envelope.

**Files changed:**
`internal/olu/client.go`, `internal/olu/client_test.go`,
`internal/auth/store_olu.go`, `internal/server/server.go`,
`internal/alerts/alerts.go`, `internal/connectivity/connectivity.go`,
`internal/handlers/api/qr.go`, `internal/handlers/api/asset_types.go`,
`internal/handlers/api/assets.go`, `internal/handlers/api/assets_test.go`,
`internal/handlers/api/fsm.go`, `internal/handlers/api/rules.go`,
`internal/handlers/api/sensors.go`, `internal/handlers/api/router_test.go`,
`internal/auth/audit_store.go`

---

## Upcoming

The following stages are planned and tracked in
[`docs/OLU_CLIENT_REMEDIATION.md`](docs/OLU_CLIENT_REMEDIATION.md).
Entries will be moved here as each stage is completed.

### ~~v0.1.1 — Client foundation~~ — complete in v0.1.0-patched1

### ~~v0.1.2 — Auth and connectivity~~ — complete in v0.1.0-patched2

### ~~v0.1.3 — API correctness~~ — complete in v0.1.0-patched3

### ~~v0.1.4 — Dependency upgrade~~ — complete in v0.1.0-patched4

### ~~v0.1.5 — New capabilities~~ — complete in v0.1.0-patched5

### ~~v0.1.6 — Test suite repair~~ — complete in v0.1.0-patched6

## [0.1.0] - 2026-01-01

### Release hygiene baseline

This release establishes the release infrastructure that all subsequent
versions will depend on. No functional changes to application behaviour.

Prior to this release the project had no `VERSION` file, no `CHANGELOG`,
no `release.sh`, and no canonical version package. The version string
`"0.1.0"` was hardcoded in four separate locations that had already begun
to drift (the distributed archive was labelled `v0.0.9` while all
in-source strings read `0.1.0`). This release resolves that drift and
prevents recurrence.

**Added:**

- `VERSION` — canonical single source of truth for the release version.
  All derived version strings are generated from this file; it must never
  be edited manually outside of `syncver.sh` or `release.sh`.
- `internal/version/version.go` — Go package exposing `version.Version`
  as a compiled constant. Imported by all in-process code that needs the
  version string.
- `CHANGELOG.md` — this file. All releases from v0.1.0 forward must have
  a corresponding entry before `release.sh` will proceed.
- `syncver.sh` — propagates the version from `VERSION` to
  `internal/version/version.go`. Also checks that a matching entry exists
  in `CHANGELOG.md` and warns if absent. Called automatically by
  `release.sh`.
- `release.sh` — the only correct release path. Validates version format,
  checks CHANGELOG, calls `syncver.sh`, runs the test suite, builds
  cross-platform binaries (linux/amd64, darwin/arm64, windows/amd64 for
  both `shelf` and `shelf-admin`), produces a release zip, and prints
  the git steps to complete the release. Never invoke `zip`, `git tag`, or
  edit `internal/version/version.go` by hand.
- `docs/OLU_CLIENT_REMEDIATION.md` — design document for the six-stage
  olu client remediation plan covering all API divergence from current olu.

**Changed:**

- `cmd/shelf/main.go` — `version` variable now derives from
  `internal/version.Version` instead of a hardcoded string literal.
  Build-time injection of `commit` and `buildDate` via `-ldflags` is
  retained; `-X main.version` injection is removed as unnecessary.
- `cmd/shelf-admin/main.go` — same change as above.
- `internal/handlers/api/router.go` — health endpoint `"version"` field
  now uses `appversion.Version` from `internal/version`.
- `internal/ui/login.go` — version fallback now uses `appversion.Version`
  from `internal/version`.
- `Makefile` — `VERSION` is now read from the `VERSION` file rather than
  hardcoded. The `-X main.version` ldflag injection is removed. `release`
  target delegates to `release.sh`. New `syncver` target added.
- `README.md` — corrected olu repository URL from `github.com/ha1tch/olu`
  to `github.com/ha1tch/xolu`.
- `ROADMAP.md` — replaced the informal checkbox list with a structured
  document; remediation plan placed front and centre as the immediate
  priority; feature work marked as blocked on remediation.

**Removed:**

- `CONTRIBUTING.md` — removed per project documentation standards.

---
