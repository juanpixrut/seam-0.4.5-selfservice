# Seam AMS — Tracking

Version: 0.4.5
Last reviewed: 2026-11-18

Open actionable items only. Closed items move to `RESOLVED.md` under the
closure procedure — never marked done in place here.

## Status table

| ID | Summary | Theme | Priority | Status | Blocks/after |
|----|---------|-------|----------|--------|---------------|
| T-03 | Scope and extract Assets Management module | assets-management-module | P2 | ☐ | T-02's boundary question resolved (T-02 closed) -- pure scoping/spec work remains, no longer architecturally blocking |
| T-04 | Build Documents module per DOCUMENTS_MODULE_SPEC.md v0.5 | documents-module | P2 | ☐ | External seam-filer/fijuggler/thumble services |
| T-05 | Rename the repository's own branding from the old name to Seam | rename | P1 | ◐ | Both passes complete; three items need your decision |
| T-08 | Scope and extract Settings module (schema lifecycle, form designer, i18n manager) | settings-module | P2 | ☐ | After T-01 (closed). T-02's boundary question resolved (T-02 closed, Settings is now a real module) -- remaining scope is feature-build, not architecture |
| T-07 | Evaluate migrating internal/olu to the official xolu client (pkg/client) | xolu-integration | P2 | ◐ | Client migration done (see body); timeseries wrapper and per-tenant JWT credential still open |
| T-11 | Scope the Order Engine / CMMS module (fifth module) and resolve its open design questions | cmms-module | P2 | ☐ | Depends on T-01, T-03 (Scope=assets), T-04 (Document/Image/Signature routing), T-08 (Field Definitions) |
| T-14 | Resolve embed mode:"drawer" -- does it mean "open fijuggler", and if so how | form-engine | P2 | ☐ | Not urgent -- no current embed example uses drawer mode |
| T-15 | Wire refresh_detail: JS listener for embed-submit-success -> reload the named detail panel | form-engine | P2 | ☐ | Needed before a real embed with refresh_detail goes live (e.g. sensor_binding) |
| T-16 | Consider adopting mintydyn for tabs -- real capability, real trade-off, not a slam dunk | form-engine | P2 | ☐ | Not urgent -- current shared SeamTabs.wire() works and is tested |
| T-19 | Asset-picker widget -- built and shipped (T-72, T-76); self-reference exclusion and the separate listing-search-bar question remain genuinely open | formengine | P2 | ◐ | Not required for FE-24/production validation -- parent_id's numeric-entry fallback is a known, documented, working limitation (SPEC §8.3), not a blocker |
| T-20 | Reconsider extracting internal/condition to a standalone repo (e.g. github.com/ha1tch/condition), matching the ha1tch ecosystem's other sovereign libraries | formengine | P3 | ☐ | Nothing -- no deadline, explicitly deferred |
| T-21 | visibilityScript (renderer.go) still hand-written, manually duplicating internal/condition's semantics in JS -- not generated from it | formengine | P2 | ☐ | None currently -- deliberately deferred, not a blocker for anything in flight |
| T-25 | internal/ui/workflow's own dormant-guards table row already existed (predates this item); its pins are now current (T-85, 2026-08-13) but the module still doesn't compile -- real xolu internal-API breakage remains, tracked separately as T-86 | test-quality | P2 | ◐ | Nothing currently blocking -- see T-86 for the remaining compile-breakage repair |
| T-28 | Custodio's tipo_sujeto/subject_type field only accepts DNI/CI/RUT/RUC (Uruguayan-specific tax/identity document codes) -- no way for a non-Uruguayan customer to represent their own tax identifier | validation | P3 | ☐ | English-market seed/customer work, if that ever becomes real -- not blocking anything currently in flight; discovered exploring a hypothetical, not requested production work |
| T-32 | password_reset.go's standalone page <title> hardcodes the app name ("Seam AMS") instead of reading h.config.AppName, unlike login.go which does | i18n | P4 | ☐ | — |
| T-34 | START.sh/START.cmd's 'Select UI language' menu is cosmetic -- SEAM_FORCE_LANGUAGE can never actually take effect once a tenant_settings row exists, and every seed script writes one unconditionally at build time | i18n | P3 | ☐ | — |
| T-35 | formengine has no unit-of-measurement concept for scalar fields -- temp_min/temp_max bake "(°C)" directly into the translatable title text instead of a structured, convertible unit | formengine | P3 | ☐ | — |
| T-41 | formengine needs jurisdiction-conditional field visibility + a tenant jurisdiction setting distinct from tenant_settings.locale | formengine | P4 | ☐ | Nothing currently -- not scoped, not blocking anything in flight |
| T-43 | Existing Spanish-language specs (FORM_ENGINE_SPEC.md, ADMIN_MODULE_SPEC.md, DOCUMENTS_MODULE_SPEC.md, I18N_SPEC.md) need translation to English per the new internal-documentation-language invariant | i18n | P4 | ☐ | Nothing currently -- not blocking any in-flight work |
| T-57 | formengine's visibilityScript IIFE duplicates verbatim once per Render() call on a page -- was hypothetical-future, now current: Settings' 3 sections (built 2026-08-04) already trigger a 3-way duplication on one live page, though still no visible_when conditions exist anywhere to make it user-visible | formengine | P2 | ☐ | - |
| T-58 | Preferences form's timezone select shows raw IANA identifiers (America/Montevideo) instead of human-readable labels the old hand-coded version had -- 76 i18n keys needed for full parity, not added | i18n | P3 | ☐ | - |
| T-61 | Several test mocks for xolu.Client.Get responses wrap fields under a nested "data" key that Get() doesn't actually unwrap -- currently invisible since none of those tests assert on the affected fields | test-quality | P3 | ☐ | - |
| T-66 | ROADMAP.md needs a full content regeneration against current reality -- header/staleness notice fixed 2026-08-04, but ~200 lines of planning content still describe May 2026's state, not v0.3.55's | documentation | P3 | ☐ | - |
| T-70 | xolu v0.27.0's own CHANGELOG (T-160) states apikey auth mode has been silently, completely non-functional (server rejects the format the client actually sent) -- the fix verified working end to end (real server, real client, seam-ui's own config pattern); the one open question (was this ever configured in a real deployment) still needs the tech team's direct confirmation | xolu-integration | P1 | ◐ | Requires the tech team's direct confirmation -- not determinable from the repository alone |
| T-86 | internal/ui/workflow/ (six end-to-end, cross-handler workflow tests against a real embedded xolu) needs effort estimation for a full repair -- currently builds again (T-85 fixed its stale replace-directive path) but go vet surfaces real API staleness (removed config field, changed constructor signature, renamed handler method) unrelated to Spanish identifiers; scoping/estimating the fix is the next step, not performed here | testing | P3 | ☐ | - |
| T-92 | config.Server.MaxBodySize is parsed but never enforced -- dead config value | config | P3 | ☐ | - |
| T-93 | internal/qrcode's image rendering is a placeholder; seam-2dcodes is a real candidate fix, pending its own auth/blob-verification gaps | qrcode | P3 | ☐ | - |
| T-96 | Generic Selector dialog: modal chooser for any entity, list configured like the List component (first use: Users) | ui-components | P2 | ☐ | Go toolchain in the sandbox; tenancy-model question for the users entity |
| T-97 | any-picker formengine widget: generalise asset-picker to any entity, add a users source | formengine | P2 | ☐ | Extends T-19; Go toolchain; tenancy-model question |
| T-98 | Confirm the tenancy model for the users entity before any Users picker/selector ships (admin list and picker add no tenant condition; auth store does) | xolu-integration | P2 | ☐ | Blocks the Users source of the Selector and any-picker |
| T-99 | Housekeeping after the gorepoman migration and Wallace removal: historical mentions and the doctor Go-version note remain (repoman/ removed and badcode clean, 2026-11-17) | housekeeping | P3 | ◐ | None -- decisions only |
| T-100 | 0.4.1-pre8 written without a Go toolchain or a browser: the browser pass is outstanding (build, vet, gofmt, icon check and tests green at 0.4.3, 2026-11-17) | formengine | P1 | ◐ | A browser pass, light and dark |
| T-101 | Selector and any-picker over assets, cost centres, locations and custodians: confirm who may list them | xolu-integration | P2 | ☐ | T-98 (same tenancy question) |
| T-103 | Masters-page DocumentLinksField still has the old hand-coded look | formengine | P3 | ☐ | — |
| T-104 | Inline validation styling for form fields | formengine | P3 | ☐ | — |
| T-105 | Optional esbuild minification of app.js when esbuild is available | formengine | P3 | ☐ | — |
| T-107 | UPC-E barcodes cannot be drawn: the vendored JsBarcode bundle omits UPCE from its format table | formengine | P3 | ☐ | — |
| T-108 | number widget ignores minimum, maximum and step although the catalogue says it supports them | formengine | P3 | ☐ | — |
| T-109 | OQL LIKE followed by AND/OR without its own parentheses matches nothing in xolu 0.30.44: user search, asset search with a filter, selector search and issue search are affected | xolu-integration | P1 | ☐ | Blocks the query-picker widget's search; report to xolu |
| T-110 | List queries use SELECT *, which xolu cannot push down: 125-220 ms at 10,000 assets; named columns plus a registered entity schema take 2-3 ms | xolu-integration | P2 | ☐ | Blocks making the seed's --schemas flag the default |
| T-111 | query-picker formengine widget: choose a record from an OQL query with typed columns -- built and verified live; closes with the release, after the browser script is added as a guard | formengine | P2 | ◐ | Shipped in 0.4.5; the browser script as a repository guard; a first production form |
| T-118 | Developer module: what is missing for daily use (new keys, new forms, export, untranslated list, translation history, designer structure) | developer-module | P2 | ◐ | - |
| T-119 | Demo tabs in dark mode: the tab open at load keeps looking open after another is chosen | ui | P3 | ☐ | - |
| T-120 | Seeds: move from Python scripts to xolu's .seed format (review and proposals in docs/SEED_FORMAT_PROPOSAL.md) | xolu-integration | P2 | ◐ | The Go command in xolu, then xoluman; the files shipped in 0.4.5 |
| T-122 | Issue templates: a name with no Latin letters or digits is refused as empty (the key comes from slugify) | i18n | P3 | ☐ | - |
| T-124 | docker-compose.yml cannot be run: no Dockerfile for the seam service, and the xolu image name is unconfirmed | xolu-integration | P2 | ☐ | - |
| T-125 | Issues has no permissions of its own: a read-only viewer and the Accounting roles can act on issues | security | P1 | ☐ | - |
| T-126 | Issues: hold, resume and merge are in the lifecycle and no screen sends them | issues | P2 | ☐ | - |
| T-127 | Issues writes nothing to the audit log | issues | P2 | ☐ | - |
| T-128 | Seam does not check at start that xolu serves what its modules need (API v2 for Issues) | xolu-integration | P2 | ☐ | - |
| T-129 | Issues stores the asset category as the Spanish word critico | i18n | P3 | ☐ | - |
| T-130 | Issues has no dashboard and no REST API | issues | P3 | ☐ | - |
| T-132 | Accounting has the start-time flag but no switch for one tenant, unlike Work Orders and Issues | accounting | P3 | ☐ | — |
| T-133 | Interface text outside the locale files: page chrome, a confirmation, Spanish fallbacks and 101 error replies | i18n | P2 | ◐ | Shipped in 0.4.5; open: what the body lists as left |
| T-134 | Texts in the browser scripts do not go through the locale files | i18n | P3 | ◐ | Shipped in 0.4.5; open: what the body lists as left |
| T-137 | Values stored in Spanish are shown as stored: asset criticality, ownership and disposal, the chart of accounts | i18n | P3 | ◐ | Shipped in 0.4.5; open: what the body lists as left |
| T-138 | Dates and amounts are formatted the same for every language | i18n | P3 | ◐ | Shipped in 0.4.5; open: what the body lists as left |

---

## form-engine

### T-14. Resolve embed mode:"drawer" -- does it mean "open fijuggler", and if so how

Theme: form-engine · Priority: P2 · Status: ☐ · Blocks/after: Not urgent -- no current embed example uses drawer mode

Trigger: Discovered 2026-07-28 while implementing formengine stage 4 (embeds, FE-13). We asked directly whether embed mode:"drawer" refers to fijuggler; answer wasn't settled either way.
Scope: fijuggler is not vendored anywhere in this repo -- confirmed via full-tree search, only conceptual references exist (internal/media/media.go comments, docs/SEAM_MEDIA_SPEC.md). SEAM_MEDIA_SPEC.md $2 documents fijuggler's actual integration pattern as SeamModal.open({width:"fullscreen", url:"/files"}) -- explicitly NOT embedded inline as a drawer/side-panel. Whether FORM_ENGINE_SPEC.md's embed mode:"drawer" means "open fijuggler this way" or is a general side-panel pattern unrelated to file browsing is unresolved. Currently in internal/formengine/renderer.go: mode:"drawer" falls back to the modal2 (SeamModal2) trigger behavior, documented inline as an open question, not a silent guess. Not blocking -- neither of stage 4's two target embed examples (sensor_binding, qr_assign) use "drawer". Revisit when a real "drawer"-mode schema is authored, or when fijuggler integration work starts for real (Documents module, T-04, or CMMS).

### T-15. Wire refresh_detail: JS listener for embed-submit-success -> reload the named detail panel

Theme: form-engine · Priority: P2 · Status: ☐ · Blocks/after: Needed before a real embed with refresh_detail goes live (e.g. sensor_binding)

Trigger: FE-14 (stage 4, §3.7), 2026-07-28.
Scope: formengine renders only an embed's trigger, not its loaded sub-form content (separate request/render, same as detail panels) -- so it can emit the hook but not the behavior. Done: renderEmbedTrigger emits data-refresh-detail="detail-<id>" on the trigger when EmbedForm.RefreshDetail is set, discoverable and tested. Not done: no JS listens for it yet. Needed: a small addition (app.js or similar) that listens for the embed's submit-success (however that gets signalled -- htmx response event, custom event dispatch from the modal close, etc. -- not decided) and triggers an htmx reload of the named detail-<id> container. Deferred rather than invented speculatively, since the actual submit-success signalling mechanism depends on how a real embed's sub-form ends up wired (stage 7 territory, when the Asset form's real sensor_binding embed gets built).

### T-16. Consider adopting mintydyn for tabs -- real capability, real trade-off, not a slam dunk

Theme: form-engine · Priority: P2 · Status: ☐ · Blocks/after: Not urgent -- current shared SeamTabs.wire() works and is tested

Trigger: Direct request 2026-07-28 -- audit the JS/TS situation, check whether minty's own dynamic-component support (mintydyn) should be used instead of hand-rolled JS.
Scope: mintydyn (already vendored, zero new dependency -- part of github.com/ha1tch/minty v0.1.0, pure Go) provides mdy.Tabs()/mdy.TabsWithTheme() which generates both the HTML structure and the switching JS from Go, with a pluggable DynamicTheme interface (could match our existing indigo-border active-tab styling exactly via a custom theme, confirmed by reading theme.go -- no forced visual change). Read the full source (mintydyn.go, builder.go, javascript.go, convenience.go, theme.go), not just the README.
Real trade-off found, not just a win: mintydyn's class DynamicComponent_<id> is generated per component instance (confirmed in javascript.go), not shared across instances on one page. Switching to it would remove the hand-typed-twice duplication (already fixed a different way this session -- see RESOLVED.md, SeamTabs.wire()) but could mean MORE total JS bytes per page than the current one-shared-function approach if multiple dynamic components render together (e.g. a future page with both a formengine tab set and a filterable list). Needs an actual byte-size comparison on a real multi-component page before deciding, not assumed either direction.
Not applicable to conditional visibility (§3b): confirmed via full-text search of mintydyn's source that its DependencyRule/ShowWhen system has no all_of/any_of composite condition support -- each rule watches exactly one trigger field independently (javascript.go's evaluateRules/evaluateTriggerCondition). FORM_ENGINE_SPEC.md §3b.5 requires nested all_of/any_of and internal/formengine/visibility.go already implements and tests it correctly. The custom evaluator stays for this reason, not out of not-invented-here -- it does something mintydyn cannot.

Upstream proposal drafted 2026-07-28, not filed (no push access to github.com/ha1tch/minty from this environment -- confirmed by testing, not assumed): composite all_of/any_of support for TriggerCondition/StateCondition, judged lower-cost than expected since the needed "read an arbitrary field's value" primitive (getInputValue) already exists and is already used by evaluateCondition for state/tab visibility -- just not reused by the DependencyRule engine or composed into trees. Also found, unprompted: evaluateCondition and evaluateTriggerCondition are two separate, overlapping operator switches inside mintydyn itself (confirmed by diff), a real internal duplication independent of anything Seam does. Full write-up: docs/proposals/mintydyn-composite-conditions.md (this repository) -- handed to the tech team to file upstream (as a GitHub issue against github.com/ha1tch/minty) or discard. If this lands in mintydyn, it directly changes this item's calculus: Seam's own visibility.go evaluator could likely be retired in favour of mintydyn's, and this tabs question would be evaluated alongside it rather than separately.

## admin-module

## ams-module

### T-03. Scope and extract Assets Management module

Theme: assets-management-module · Priority: P2 · Status: ☐ · Blocks/after: T-02's boundary question resolved (T-02 closed) -- pure scoping/spec work remains, no longer architecturally blocking

Trigger: Direct request, 2026-07-28 — "make the asset management system module as well"; confirmed as one of four named modules 2026-07-28 (Assets Management, Documents, Settings, Administration), with a CMMS-like module planned as a future addition to the asset lifecycle.
Scope: No ASSETS_MODULE_SPEC.md exists yet. 2026-07-28 decision: bundle monitoring (sensors/alerts/fsm/rules) + reports into this module for now, split later if wanted. Proposed home: internal/modules/assets/. masters stays in internal/core/masters/ per ADMIN_MODULE_SPEC's own dependency diagram. Design should leave room for the future CMMS-like module without committing to its shape now.
FSM decision, 2026-07-28 (see RESOLVED.md T-12): ASSETS_MODULE_SPEC.md must specify migrating internal/fsm's role onto xolu's native /fsm, not the fsm-toolkit-as-runtime path FSM_DESIGN.md originally targeted (superseded, see that doc's reconciliation banner).

Discrepancy found 2026-08-03, not resolved here: internal/modules/assets/ (this item's own "proposed home") does now exist -- built 2026-08-03 as MOD-6 -- but as a two-line registry-registration wrapper (MODULE_SPEC.md §6.1's deliberate "thin wrapper, not relocated" design), not the richer module this item envisions (monitoring + reports bundled in, an ASSETS_MODULE_SPEC.md of its own). The package name and location match; the content and scope don't. Same "is this superseded, rescoped, or still separately real" question as T-02/T-08 -- flagged, not decided here.

Priority downgraded P1->P2, 2026-08-04, examining interdependencies between Settings/Administration/Account after T-02 closed. Same reasoning T-02 itself resolved by: the P1-worthy question was "does Assets have a real, working module boundary" (routes, permissions, registry entry) -- that's answered, yes, since MOD-6. What remains (ASSETS_MODULE_SPEC.md, deciding whether monitoring/reports formally bundle in) is scoping and documentation work, not an open architectural gap blocking anything else. "Blocks/after" also fixed -- "boundary reconciled with T-02" was written when T-02 was open and the Administration/Settings split was undecided; T-02 is now closed and the split is real, so there's no longer an open boundary question to reconcile against.


## documents-module

### T-04. Build Documents module per DOCUMENTS_MODULE_SPEC.md v0.5

Theme: documents-module · Priority: P2 · Status: ☐ · Blocks/after: External seam-filer/fijuggler/thumble services

Trigger: Direct request, 2026-07-28 — "we're also going to have a Document management module."

Correction, 2026-07-28: NOT postponed -- an earlier note wrongly bundled this with FE-25's postponement. The Documents module itself is active, planned work. What's actually postponed is specifically FE-25's widget-level mechanism (per-Asset-instance attachment tray) -- see FE-25's own entry for why. Still gated on external seam-filer/fijuggler/thumble services readiness, as originally filed.
Scope: Thin metadata layer over external file storage/preview services (seam-filer, fijuggler, thumble). Confirmed 2026-07-28: docs/SEAM_MEDIA_SPEC.md v0.3 fully specifies these (target repo github.com/ha1tch/seam-media explicitly "pendiente de creación" -- not built yet), and internal/media/media.go already implements the media.Client contract against it with a working NewStub() degradation path (Document viewer falls back to MIME icons, fijuggler_ref/thumble_asset_id stay empty). Better-understood blocker than previously recorded: not "nothing exists", it's "spec'd and contract-stubbed, service unbuilt."


## rename

### T-05. Rename the repository's own branding from the old name to Seam

Theme: rename · Priority: P1 · Status: ◐ · Blocks/after: Both passes complete; three items need your decision

Trigger: renaming decided in an earlier session; go-now-two-passes decided 2026-07-28 (packages/identifiers, then texts/branding; Python only, no sed/awk; restore from the 0.2.1 checkpoint if anything needs undoing rather than patching forward).
Scope: Blast radius at last count before starting: 70 Go files, 394 occurrences.

Pass 1 complete, 2026-07-28: Go module path (go.mod + every import); Go/JS identifiers (SeamModal/SeamModal2/SeamPopover/SeamNav/SeamUserMenu/SeamConfirm/SeamAPIURL/SeamUI); 26 environment variables; the x-seam-* JSON Schema extension key family; cmd/ directory names and the seam-fsm-editor.js filename; Makefile, package.json, package-lock.json. 83 files, 580 lines, reviewed dry-run-then-apply Python. Historical record never touched: CHANGELOG.md, docs/RESOLVED.md, docs/formengine/RESOLVED.md, .ed-journal.json. go build/vet/test clean (16 packages), JS syntax clean, gofmt clean.

Pass 2 complete, 2026-07-28: all markdown docs (same historical exclusions), seed scripts, the PWA manifest, the three locale files, app.css (had to match the transition-name identifiers Pass 1 already changed in Go-embedded JS -- found and fixed a real CSS/JS mismatch this way, not just cosmetic), api/openapi.yaml, docker-compose.yml. SEAM_MEDIA_SPEC.md renamed from its old filename, content updated throughout including its own sub-brand names. Two verbatim quotes of things actually said in this conversation, found in this register, were protected from the sweep and confirmed unchanged after. Cleanup pass also caught five .sh scripts (syncver.sh, release.sh, and three in scripts/) that neither pass's file-extension list originally covered -- found via a full-tree residual grep after Pass 2, not planned for up front. That cleanup also caught and fixed a real bug: syncver.sh's own Go-file-generation template had the old name hardcoded in a heredoc, so it was re-introducing the old name into internal/version/version.go every time the version got bumped, silently undoing Pass 1 each time. 31 files, 631 lines in Pass 2 proper; 5 files, 102 lines in the .sh cleanup. Full-tree residual sweep after everything: clean except the deliberately excluded historical files.

Resolved 2026-07-28: (c) QR prefix -- confirmed SEAM everywhere four characters fit (qrcode.go DefaultConfig, internal/config/config.go, config.example.json, all corresponding tests); verified via Validate()'s expectedLen := len(prefix) + Length that prefix length is additive, not carved from a fixed budget, so no context anywhere in this codebase actually needs the 3-char SEA fallback. (a) module path -- treated as confirmed (github.com/ha1tch/seam-ui) after no correction on repeated flagging.

Resolved 2026-07-28 (logo pixmaps): the 8 wordmark pixmaps that literally spelled out "Shelf AMS" as baked-in text (logo-160/320 + dark, logo-landscape/-sm + dark) had only their "Shelf" region replaced, measured and verified with numpy rather than eyeballed. Method: row-band and column-gap analysis on the alpha channel located the exact "Shelf" bounding box (handling both layouts in this asset set -- icon-above-text for the square logos, icon-beside-text for the landscape ones, auto-detected by row-band count); per-file gradient colour sampled from the actual opaque pixels (light variants: dark-navy-to-blue; dark variants: light-gray-to-blue-gray, confirmed genuinely different per file, not assumed); replacement text set in Zalando Sans Expanded (Google Fonts, downloaded from the google/fonts repo), height-matched then horizontally scaled to exactly the measured width. Verified per file post-write: region outside the measured box is byte-identical to the original (numpy array equality), not just visually similar. "AMS" and the icon mark are untouched in all 8 files -- confirmed, not assumed.

Resolved 2026-07-28 (favicon.svg): rebuilt from a vectorised trace of icon-192.png's actual S mark rather than the old shelf pictogram. Method: enlarged icon-192.png 3x (576px) for tracing precision; OCR was run on it first as a diagnostic per instruction, honest result recorded rather than forced -- single-character-mode tesseract reads the raw mark as "=" at 96% confidence, not "S"; looser layout-mode OCR gives "Ss" but only at 58% confidence, so the mark is not unambiguously OCR-legible as S on its own (worth knowing, not blocking -- brand marks lean on colour/context human readers have and OCR doesn't). Isolated the S silhouette from the white icon background via colour-distance thresholding (icon-192.png has no alpha channel -- fully opaque, white square background, not transparent) plus a small morphological closing to remove tiny holes from the circuit-trace overlay, keeping just "the large S" as asked. Traced with potrace; verified 0.9961 IoU against the source raster mask before trusting the path data (first attempt with potrace's --tight flag scored only 0.67 IoU from an aspect-ratio distortion -- caught by measuring, not by eyeballing, exactly the kind of thing that looks fine at a glance and is wrong). Gradient colour sampled from the actual corner pixels of the S shape (dark navy #1f1f47 bottom-left to cyan #1da4bb top-right, diagonal) and applied as the fill, not guessed. Checked legible from 16px to 256px before writing to the real path.

Still open: (b) qr_base_url domain (currently the old domain, harmless as a placeholder but should get a real one before anything prints it) -- low urgency, not blocking; (e) favicon.ico -- separate raster file, still the old shelf-pictogram design, now inconsistent with the new favicon.svg. Not touched -- wasn't asked for, and .ico regeneration (multi-resolution raster bundle) is a different kind of task than the SVG vectorisation just done.


## settings-module

### T-08. Scope and extract Settings module (schema lifecycle, form designer, i18n manager)

Theme: settings-module · Priority: P2 · Status: ☐ · Blocks/after: After T-01 (closed). T-02's boundary question resolved (T-02 closed, Settings is now a real module) -- remaining scope is feature-build, not architecture

Trigger: Module list clarified 2026-07-28 — Assets Management, Documents, Settings, Administration.
Scope: Carves the DIY-configuration half out of ADMIN_MODULE_SPEC.md v0.1 (schema lifecycle §3c.7/§15.5, form designer §13.4, i18n manager) as its own module, separate from Administration (T-02, which keeps users/roles/permissions). No SETTINGS_MODULE_SPEC.md exists yet — needed before extraction, same as T-03. The i18n manager third has its own full spec, docs/I18N_SPEC.md v0.1 (1230 lines) -- not previously cross-referenced here. See also T-13 (formengine's own i18n wiring, a T-01 prerequisite, separate from the Settings-module admin UI this item covers).

Discrepancy found 2026-08-03, not resolved here: what actually shipped under the name "Settings" (MOD-5, MODULE_SPEC.md §5) is one of three views inside the System module -- a simple tenant_settings form (name, locale, notification toggles, map feature flag). It shares nothing with this item's scope (schema lifecycle, form designer, i18n manager -- meta-configuration tooling, none of which exists yet). Two genuinely different things named "Settings." This item's real scope (T-08 as written) remains entirely unbuilt -- not superseded so much as untouched, unlike T-02/T-03 where the shipped design more directly overlaps the old one. Noted for the same reason: worth the tech team's eyes before assuming this is still accurately scoped.

Update 2026-08-04, factual only, decision unchanged: "Settings" is no longer a view inside System -- T-02's resolution (see RESOLVED.md) split the former System module into Account/Settings/Administration, so Settings is now its own registered modules.Module with its own MountSettingsRoutes, its own permission namespace (settings.*), and its own nav entry. This narrows, but doesn't close, the discrepancy above: Settings is now a real module boundary (the part of this item's own premise that concerned module *structure*), but its actual *content* is still just the same tenant_settings form -- schema lifecycle, form designer, and i18n manager remain entirely unbuilt. T-08 as written is still open work, now scoped as "add real functionality to an existing module" rather than "extract a module that doesn't exist yet."

Priority downgraded P1->P2, 2026-08-04, examining interdependencies between Settings/Administration/Account after T-02 closed: same reasoning as T-03's own downgrade. The P1-worthy question -- "is Settings a real module" -- is answered. What remains (schema lifecycle, form designer, i18n manager) is real, valuable feature work, but doesn't block anything else in the way an open architectural gap would.

Cross-reference, same review: T-41 (jurisdiction-conditional field visibility) names "no tenant-level jurisdiction/region setting exists at all" as its own foundational gap, alongside tenant_settings.locale. Settings' existing settings_general.json schema and SettingsSaveGeneral handler are now a concrete, real place such a setting could actually live -- before T-02, "the Settings module" wasn't a stable enough target to point T-41 at. Not scoped as part of T-08 (T-41 remains its own item, its own priority), just noted as a newly-answerable "where would this go" question.


## xolu-integration

### T-07. Evaluate migrating internal/olu to the official xolu client (pkg/client)

Theme: xolu-integration · Priority: P2 · Status: ◐ · Blocks/after: Timeseries wrapper and per-tenant JWT credential still open

Status corrected ☐->◐, 2026-08-04, noticed in passing while filing T-67 (unrelated xolu client-library version bump, v0.23.0->v0.25.0). This item's own body already describes substantial completed work ("Client migration done, 2026-07-28... internal/olu itself deleted afterward") that the status field never reflected. Not re-audited in full here -- just the field-line/body inconsistency corrected. The two remaining named gaps (timeseries client wrapper, per-tenant JWT credential) are unaffected by T-67's version bump, which only touched pkg/client's version, not internal/olu (already deleted) or the credential model.

Trigger: 2026-07-28 — xolu is now at v0.16.16 with an official client (github.com/ha1tch/xolu/pkg/client) and stable /ts, /fsm, /cal, /bal primitives (/fsm was early-stage, /cal and /bal did not exist, when internal/olu was last built against xolu ~v0.9.7). /dxp (distributed cross-substrate transactions) is still in progress upstream.
Scope: pkg/client currently covers Create/Get/Update/Patch/Delete/Save/Commit/List/Search/OQL/GraphQuery/Sulpher/Health/Ready/Graph traversal, plus dedicated cal.go and machine.go (fsm) wrappers. No dedicated timeseries client file yet — internal/olu's hand-rolled Pebble timeseries methods (13 methods, per ROADMAP) have no official equivalent to migrate onto yet; this is the gap that actually matters now, since Assets Management needs sensor timeseries. No bal.go wrapper either, but confirmed 2026-07-28: none of the four current modules need /bal — it's only a plausible fit for the future CMMS-like module's spare-parts/consumables inventory, which is unscoped. Any migration is partial until upstream adds the timeseries wrapper. Relevant to core layer design for T-02/T-03/T-08's module split. Also now covers Phase 1 of T-02's tenant-credential decision (2026-07-28): swap internal/olu's single wildcard bearer for one xolu-scoped JWT per Seam tenant. JWT minting needs no xolu-side registration (just the shared XOLU_JWT_SECRET and ~20 lines of stdlib HMAC signing, per cmd/xotogen/main.go), so this doesn't block on xolu config changes -- it blocks on internal/olu picking the right per-tenant credential per request instead of one shared apiKey, and on deciding where Seam stores those secrets.

Client migration done, 2026-07-28 (verified against both v0.16.16, the latest published tag, and a v0.16.27 checkpoint the tech team provided directly -- pkg/client is byte-for-byte identical between them, so nothing here is pinned to a stale snapshot). Confirmed empirically before touching anything: Seam only ever calls 7 methods across the whole codebase (OQL 95x, Get 41x, Create 23x, Update 22x, Delete 12x, List 5x, Search 1x -- List/Search were missed in the first pass since handlers/api/*.go calls them through a variable named "client", not "olu"), all with identical signatures in pkg/client, plus Entity/ListParams/SearchParams/ListResult/ClientOption/Error, all structurally identical (Error's field-name difference -- StatusCode/Details vs HTTPStatus/Detail -- turned out moot, since pkg/client keeps StatusCode/Details as documented backward-compatible deprecated aliases). pkg/client itself is pure stdlib, zero third-party imports, confirmed by reading every import block directly.

24 files migrated: import swapped to `xolu "github.com/ha1tch/xolu/pkg/client"`, every package-qualified reference (olu.Client, olu.Entity, etc.) renamed to xolu.*. Struct field and local variable names (h.olu, s.olu, oluClient, etc.) deliberately left as-is for this pass -- lower risk, smaller diff, and the rename is a separable cosmetic decision, not bundled in here. internal/olu itself deleted afterward once confirmed zero remaining importers. Full repo go build/go vet/go test clean throughout, including a self-inflicted bug caught immediately by the first build attempt: a first-pass regex renamed olu. to xolu. everywhere, which correctly caught package-qualified references (olu.Client -> xolu.Client) but incorrectly also caught field-access chains (s.olu.OQL(...) -> s.xolu.OQL(...), where "olu" is a struct field name, not a package qualifier) -- fixed with a negative-lookbehind pattern once the compiler pointed at exactly where the two cases needed to be told apart.

Still open, not touched by this pass: the tenant-JWT credential swap described above (T-02 Phase 1) -- construction in server.go still uses xolu.WithAPIKey(cfg.XOLU.APIKey), a single wildcard key, not per-tenant xolu.WithJWT(...). That's real, separate work, not a mechanical rename, and stays open under this item until done.

Identifier rename done, 2026-07-28 (Tier 1 + Tier 2 of the three-tier estimate given when asked): every remaining Go code identifier containing "olu" -- struct fields (h.olu, s.olu -> xoluClient), the exported Handler.OLU field -> XOLU, local variables (oluClient, oluOpts, oluErr, oluURL, oluClientAdapter), and every type/function name (OLUClient, OLUEntity, OLUAuditStore, NewOLUAuditStore, OLUUserStore, NewOLUUserStore, WriteOLUError, OLUConfig, CLIOLUClient) -- renamed to xolu/XOLU equivalents, plus internal/auth/store_olu.go renamed to store_xolu.go. Comment prose referencing "olu" as the backend name updated across 26 files, and stale test-assertion message strings ("olu.Create was not called" etc.) corrected for accuracy. One real miss caught only by a final exhaustive grep, not the earlier targeted passes: OLUWithTenant is a *method* (receiver clause between `func` and the name defeated the original declaration-detection regex, which anchored on `func` immediately followed by the name), used ~50 times across handlers/api -- found and fixed in the same pass.

Tier 3 deliberately untouched, exactly as scoped when the estimate was given: OLU_URL and SEAM_OLU_API_KEY environment variables, the "olu-url" CLI flag, and the `json:"olu"` config tag all remain -- these are external deployment contracts (referenced in DEPLOY.md, docker-compose.yml, config.example.json, README.md), not internal naming, and still need the tech team's choice among hard-cutover / backward-compatible dual-read / leave-alone before any of them change. Full repo go build/go vet/go test clean throughout (15 packages), gofmt clean.

Tier 3 done, 2026-07-28, hard cutover -- the tech team's call once the actual question was clarified: these were never xolu-defined names to preserve compatibility with (checked directly against xolu's own source first, not assumed) -- OLU_URL/SEAM_OLU_API_KEY were always Seam's own invented client-side config names, fully within Seam's own scope to rename. Confirmed xolu's own current convention is unambiguously XOLU_-prefixed (XOLU_AUTH_TYPE, XOLU_API_KEYS, XOLU_PORT, XOLU_BASE_DIR, etc., read directly from pkg/config/config.go), and xolu's own CHANGELOG.md shows it went through this exact same OLU_->XOLU_ rename in its own history. Renamed: OLU_URL->XOLU_URL, SEAM_OLU_API_KEY->SEAM_XOLU_API_KEY, SEAM_OLU_URL->SEAM_XOLU_URL (seam-admin), the -olu-url CLI flag -> -xolu-url, and the json:"olu" config tag -> json:"xolu", across config.go, config_test.go, cmd/seam/main.go, cmd/seam-admin/main.go, config.example.json, DEPLOY.md, README.md.

Real functional bug found and fixed along the way, not just renamed: docker-compose.yml's xolu-container environment (OLU_DATA_DIR, OLU_PORT) doesn't match anything xolu's current server actually reads -- confirmed against pkg/config/config.go, which expects XOLU_BASE_DIR and XOLU_PORT. The old names were silently no-ops; xolu would have fallen back to its own defaults regardless of what the compose file set. Fixed to the real names, plus renamed the service/container/volume (olu -> xolu, seam-olu -> seam-xolu, olu-data -> xolu-data) for consistency. Left genuinely unverified rather than guessed: whether ghcr.io/ha1tch/xolu exists as a published image (vs. the current ghcr.io/ha1tch/olu:latest) -- flagged with a TODO comment in the compose file itself rather than assumed either way.

One deliberately-flagged-not-renamed item: internal/handlers/api/handler.go's WriteXOLUError still emits "olu_error" as the API error-code string in JSON responses. Nothing in this codebase checks for that literal string, but an external API consumer outside this repo might -- left alone rather than guessed at, same reasoning as the container image.

Separate, much larger scope surfaced by the exhaustive check and deliberately NOT touched in this pass: the docs/*.md spec corpus (FORM_ENGINE_SPEC.md, ADMIN_MODULE_SPEC.md, I18N_SPEC.md, SEAM_MEDIA_SPEC.md, DOCUMENTS_MODULE_SPEC.md, FSM_DESIGN.md, RBAC-COMPLETION-PLAN.md, ROADMAP.md) has hundreds of "olu" references, including an olu:"field_name" struct-tag convention used throughout DOCUMENTS_MODULE_SPEC.md and SEAM_MEDIA_SPEC.md's example Go structs for the not-yet-built Documents/Media modules. This is a different, much bigger question than the four external contracts originally scoped as Tier 3 -- whether that proposed tag convention should also become xolu:"..." is a real design decision for modules that don't exist yet, not a mechanical rename, and wasn't part of what was estimated or asked. Flagged for a separate, explicit decision rather than folded in here.

Docs corpus done, 2026-07-28: scripts/olu_to_xolu_docs.py built specifically to address this -- not a one-off manual pass, a reusable, re-runnable --check/--apply tool (registered as a dormant guard in docs/KNOWN_ISSUES.md), motivated directly by the risk the tech team named: specs describing not-yet-built modules (Documents, Media) still saying olu:"field_name" would get faithfully copied into real Go code by whoever implements those modules later, silently reintroducing exactly what this whole rename was meant to retire. Six named, narrow rules (struct tags, method-call-style prose, config-path-style prose, INI section headers, bold labels, generic prose) applied to 7 living-spec files (148 changes total), with anything not matching a known rule surfaced as "unclassified" rather than guessed at -- caught one genuine edge case this way (a filename reference to docs/OLU_CLIENT_REMEDIATION.md, which stays named that on purpose, since renaming the reference without renaming the file would produce a dead link). Historical documents (CHANGELOG.md, RESOLVED.md, docs/OLU_CLIENT_REMEDIATION.md itself, ROADMAP.md, docs/RBAC-COMPLETION-PLAN.md) deliberately excluded from the file scope, each for a checked, stated reason, not a blanket assumption. Also fixed one now-stale reference to the deleted internal/olu package found along the way, in docs/KNOWN_ISSUES.md itself.


### T-70. xolu v0.27.0's own CHANGELOG (T-160) states apikey auth mode has been silently, completely non-functional (server rejects the format the client actually sent) -- the fix verified working end to end (real server, real client, seam-ui's own config pattern); the one open question (was this ever configured in a real deployment) still needs the tech team's direct confirmation

Theme: xolu-integration · Priority: P1 · Status: ◐ · Blocks/after: Requires the tech team's direct confirmation -- not determinable from the repository alone

Trigger: found 2026-08-04 while reviewing xolu v0.27.0's own CHANGELOG
for unrelated reasons (verifying T-69's CreateMachineDef delivery).
Their entry for T-160 states plainly: "the client's apikey auth mode
has been completely non-functional -- sends 'Bearer', the server never
accepts that for apikey auth... Every WithAPIKey-configured client has
been silently unauthenticated on every request, regardless of key
correctness."

This is NOT hypothetical for Seam AMS -- confirmed directly, not
assumed:

- SEAM_XOLU_API_KEY is a real, documented environment variable in both
  DEPLOY.md ("xolu API key (if xolu auth is enabled)") and README.md
  (identical wording).
- internal/server/server.go wires it through conditionally:
  `if cfg.XOLU.APIKey != "" { xoluOpts = append(xoluOpts,
  xolu.WithAPIKey(cfg.XOLU.APIKey)) }`.
- internal/config/config.go's own field comment for XOLU.APIKey reads
  "Bearer token; required when xolu auth is enabled" -- meaning the
  project's own documentation independently describes the mechanism
  the exact way the bug behaved, rather than catching the mismatch.
- docs/TRACKING.md (T-02's own notes) separately records that
  server.go's xolu construction "still uses xolu.WithAPIKey(cfg.XOLU.
  APIKey), a single wildcard key" as of this session -- confirming
  this isn't dead, superseded code either.

What's NOT known, and can't be determined from the code alone: whether
SEAM_XOLU_API_KEY has ever actually been SET to a non-empty value in
any real Seam AMS deployment (dev, staging, demo, or production). If
it has never been set, xolu auth was simply disabled throughout (the
`if cfg.XOLU.APIKey != ""` guard means no options were added at all,
so no bug was ever exercised). If it HAS been set anywhere xolu auth
was also enabled server-side, every request from that Seam AMS
instance would have been silently unauthenticated the entire time --
not merely broken, but silently insecure: requests likely still
succeeded (since xolu apparently doesn't hard-reject on a malformed
apikey attempt in a way that surfaced as visible errors, or this would
likely have been noticed already), meaning any access control xolu's
own auth was meant to provide was not actually being enforced.

xolu v0.27.0 fixes this at the client level (WithAPIKey now sends the
key in a format their server's pkg/authmw.validateAPIKey actually
accepts) -- already swapped in as part of T-69's delivery, so any
NEWLY-configured or newly-restarted-against-v0.27.0 deployment is
correct going forward. This item is about the PAST: whether this
credential path was ever live, and if so, what if anything needs
auditing (access logs, whether anything sensitive was exposed to
unauthenticated requests during that window) beyond just having the
fix now in place.

Requires the tech team's own direct confirmation -- not something we can
determine from the repository alone, and not something to guess at or
downplay given the severity if the answer is yes.

Verified as thoroughly as possible without that confirmation, 2026-08-05, after we asked to "fix" this alongside T-31. The code-fixable part is genuinely done; the actual open question -- was this credential path ever live in a real deployment -- is not something fixing resolves, and remains open below.

Confirmed the original bug's real severity empirically, not just from xolu's own changelog claim: started a real xolu v0.28.0 server with XOLU_AUTH_TYPE=apikey actually enabled, confirmed `Authorization: Bearer <key>` (the old, broken format) genuinely gets rejected with 401 against a real server, and `Authorization: ApiKey <key>` (the fixed format) succeeds with 200.

Confirmed the fix works at the level that actually matters to Seam AMS: a standalone Go program using the real xolu client, configured exactly the way internal/server/server.go configures it (`xolu.New(url, xolu.WithAPIKey(key))`), authenticated correctly against the same real server. Not inferred from reading the fix, not trusted from xolu's own changelog description alone -- run.

Found one small, unrelated loose end in xolu's own code while verifying this: `WithAPIKey`'s doc comment (pkg/client/client.go) still says the key is "sent as Authorization: Bearer <key>", even though the actual implementation (authHeader(), confirmed correct) now sends "ApiKey <key>". Their codebase, not patched here -- worth mentioning to them if it comes up again, not urgent enough to justify a dedicated follow-up letter on its own.

Searched the repository for any signal on whether this was ever actually configured, informative but not conclusive: every occurrence of SEAM_XOLU_API_KEY across DEPLOY.md, README.md, and CHANGELOG.md is documentation describing the option, never a filled-in real value. config.example.json shows `"api_key": ""`. Consistent with every server started this entire session (always with an empty key, the `if cfg.XOLU.APIKey != ""` guard never firing). This is real signal that the credential path was likely never turned on -- but it is not proof, since a private, non-repo-tracked deployment configuration is genuinely outside what any of this can see. The core question -- was this ever configured in a real deployment, and if so, does anything from that window need auditing -- remains exactly as open as before, still requiring the tech team's own answer, not narrowed by this verification pass.

### T-98. Confirm the tenancy model for the users entity before any Users picker/selector ships (admin list and picker add no tenant condition; auth store does)

Theme: xolu-integration · Priority: P2 · Status: ☐ · Blocks/after: Blocks the Users source of the Selector and any-picker

Trigger: found while reading code for the Users selector/picker (2026-10-07). The admin Users page lists users with `SELECT TOP 100 * FROM users ORDER BY name ASC` (internal/ui/routes.go:2381) with no tenant condition, while the auth store's ListUsers filters on tenant_id and /picker/search adds no tenant condition either. Whether that is safe depends on whether one xolu instance can hold users of several tenants (per-tenant instance isolation) or not -- NOT verified.

Needed before any Users picker or selector ships: confirm the tenancy model for the users entity. If instances are shared across tenants, the Users source must add `tenant_id = <auth.TenantIDFromContext(ctx)>` (requireAuth already puts claims in the context via auth.ContextWithClaims). If instances are per tenant, record that here and in the Selector design so the tenant condition is a deliberate omission rather than an accident. See docs/HANDOVER_2026-10-07.md section 6.4.

### T-101. Selector and any-picker over assets, cost centres, locations and custodians: confirm who may list them

Theme: xolu-integration · Priority: P2 · Status: ☐ · Blocks/after: T-98 (same tenancy question)

Trigger: the pre8 sources use `/selector` and `/picker/search`, which are open to every
signed-in user, with tenant scoping left to the xolu client as for the existing asset
picker. Confirm that is the intended visibility for these four entities (custodians carry
tax id, email and phone), or add a per-entity permission check to `xoluSelectorSpec`.


### T-109. OQL LIKE followed by AND/OR without its own parentheses matches nothing in xolu 0.30.44: user search, asset search with a filter, selector search and issue search are affected

Theme: xolu-integration · Priority: P1 · Status: ☐ · Blocks/after: Blocks the query-picker widget's search; report to xolu

Trigger: found 2026-11-17 while checking JOIN support for the proposed query-picker widget, by running queries against a real xolu 0.30.44 test server (3000 seeded rows).

In xolu 0.30.44's OQL, a LIKE that is followed by AND or OR without its own parentheses matches nothing: the rest of the condition is taken into the LIKE. Confirmed on a single table and through a JOIN:
- `description LIKE '%filter%'` returns 900 rows; `description LIKE '%filter%' OR code LIKE '%filter%'` returns 0; `description LIKE '%filter%' AND tenant_id = 1` returns 0.
- A LIKE that comes last works (`tenant_id = 1 AND description LIKE '%filter%'` returns 300), and so does one wrapped on its own: `(description LIKE '%filter%') OR (code LIKE '%filter%')` returns 900.
- Parentheses around the whole OR group do not help; each LIKE needs its own.
The query returns status 200 with an empty result, so nothing reports the fault.

Seam query builders with the affected shape (by reading the code; not yet run against a live xolu):
- internal/auth/store_xolu.go ListUsers: `(name LIKE ... OR email LIKE ...)`. A user search returns nobody; this is the search behind the Users selector and the users picker source.
- internal/ui/routes.go AssetsList: `name LIKE ...` is the first condition, so a search combined with a status, type or criticality filter returns nothing.
- internal/ui/selector_entities.go: search columns joined with OR; affected with two or more search columns, or when the text is a number (the `id =` branch follows the LIKE).
- internal/ui/issue_filters.go: `title LIKE ... OR <asset ids>`.
Not affected: internal/ui/asset_picker_search.go (the LIKE comes last inside its group).

Scope: wrap every generated LIKE term in its own parentheses, through one helper, in the four builders above, and add a test per builder that asserts the generated text. The existing tests use a mock that captures the query and never evaluate it, which is why this passed unnoticed. Report the parser fault to xolu; the Seam-side change is correct with or without an upstream fix.

### T-110. List queries use SELECT *, which xolu cannot push down: 125-220 ms at 10,000 assets; named columns plus a registered entity schema take 2-3 ms

Theme: xolu-integration · Priority: P2 · Status: ☐ · Blocks/after: Blocks making the seed's --schemas flag the default

Trigger: measured 2026-11-17 while writing an entity schema set for the Constructec seed (seeds/schemas/seed-01-constructec.schemas.json, opt-in with the seed's new --schemas flag) to see whether xolu's typed tables speed up Seam's queries. Real xolu 0.30.44 server with the launcher's settings, the Constructec seed plus generated assets, sequential requests on a 2-core sandbox; medians.

Finding: xolu does not push down a `SELECT *`. With a registered schema, every `SELECT TOP n * FROM assets ...` reads the whole entity before applying TOP; without a schema the same happens whenever the query has ORDER BY or no WHERE. Naming the columns removes it. At 10,000 assets:
- filter by status, `SELECT *`: 2.2 ms without a schema, 125 ms with one; with six named columns: 1.9 ms either way.
- sorted list page, `SELECT *`: 224 ms without a schema, 217 ms with one; with six named columns: 116 ms without a schema, 3.2 ms with one.
- search for a picker (named columns, sorted): 25.9 ms without a schema, 2.6 ms with one; the same through a JOIN to asset_types: 70 ms and 11 ms.
- count by status: 55 ms without a schema, 2.9 ms with one.
Rows returned were identical in every comparison, and the seeded data read back identical with and without schemas.

So registering schemas today would make most list pages slower, because Seam has 96 `SELECT *` queries outside tests (masters_routes.go 14, routes.go 13, and others). The gain needs both: a schema and named columns.

Also found:
- xolu refuses a schema that declares `created_at` or `updated_at` (its typed tables already have columns with those names: "duplicate column name"). Five seeded entities carry such fields (alerts, rules, sensor_bindings, tenant_settings, users). The schema file leaves them out; the values are still stored and read back correctly. Worth reporting to xolu.
- An `enum` on assets.status and assets.connectivity_tier (xolu indexes enum fields) made no measurable difference, so the file declares no enums.
- Once a schema is registered xolu validates every write against it. The types in the file are the ones the seed writes; whether every Seam form writes the same types (for example purchase_price and temp_min are strings in the seed) is not verified, which is why --schemas is opt-in.

Scope: replace `SELECT *` with named columns in the list and picker queries, starting with the Assets list and the masters; then run Seam's handlers against a xolu with the schemas registered (writes included) before making --schemas the default; extend the schema set to the other five seeds.

### T-120. Seeds: move from Python scripts to xolu's .seed format (review and proposals in docs/SEED_FORMAT_PROPOSAL.md)

Theme: xolu-integration · Priority: P2 · Status: ◐ · Blocks/after: The Go command in xolu, then xoluman; the files shipped in 0.4.5

Progress 2026-11-18 (in the tree, unreleased): the decision was to proceed. `seeds/seed_system/` is a loader for the format (Python standard library only) with the safety changes of the proposal: a tenant that holds data is refused, a load is recorded in the tenant (`seed_loads`), a second load leaves existing records alone, unknown keys and entity types are errors, a failed load can be continued or undone, and dates relative to the day of the load. `docs/SEED_FORMAT.md` specifies format version 2. The six seeds are `.seed` files generated from the Python scripts, with 44 to 78 clock-dependent values each turned into placeholders; `seeds/seed-NN-name.py` remain as the same commands and now load the files; `requests` and `bcrypt` are no longer needed. Verified: `seeds/conformance/run.py`, 24 cases on xolu 0.30.44, six of which load each file as of a fixed moment and compare it, record by record, with what the Python script wrote as of that moment (no differences apart from the password hashes, which a salt makes different on every run); 20 unit tests; the 141 browser checks over an instance seeded with the new command. Not done here: xolu's own `scripts/seed.py` reads version 1 only, so these files load with Seam's loader until xolu takes version 2; `xoseed` in Go and xoluman's normalization.

Progress 2026-11-18, second part (in the tree, unreleased): P-9 and P-12 are built as `match`, `find`, `addition` and `requires` (docs/SEED_FORMAT.md, sections 2, 3.1 and 6). The two work-order seeds are `.seed` files; each was compared with its Python script on an empty instance and over seeds 01 and 03, with no differences (16, 39 and 136 records). New seeds, as additions: Issues, one file for each dataset (`seeds/issues/`), and accounting (`seeds/accounting/`, written by `cmd/accounting-seedgen` from the module's own Service). Verified: 33 conformance cases, 24 unit tests; in a browser over seed 01, 15 checks of the Issues pages (list, detail with template values and history, the case and its issues, templates, QR codes and their public page, starting an issue, reporting a new one) and 22 of the accounting pages as back office, accountant and AE, with the seed loaded before and after the module made its chart.

Trigger: review of xolu's `.seed` loader (xolu 0.30.44, `scripts/seed.py`, format version 1) against Seam's six seeds, 2026-11-17, to decide whether xolu and Seam standardize on that format, with xoluman following.

Found: all six seeds convert to `.seed` files and load with xolu's loader with no differences (164 to 288 records each, compared record by record on two instances; 19 Seam pages identical over both; seed 01 with its entity schemas and with the work-order seeds as well). The format is not yet safe to adopt: a load over existing data is not detected, a re-run replaces every record, unknown keys and misspelt entity names are accepted, and nothing in a file can be relative to the day of the load. The review, its evidence and 24 proposals are in docs/SEED_FORMAT_PROPOSAL.md.

Scope: decide on the proposal (its section 8). If adopted: the changes of its section 5.1, P-8 and P-22 land in xolu; Seam's seeds become six `.seed` files and the Python seeds, `requests` and `bcrypt` go; `make dev`, scripts/build-demo.sh and the seeds catalog follow. Seam's seeds are unchanged until then.

### T-124. docker-compose.yml cannot be run: no Dockerfile for the seam service, and the xolu image name is unconfirmed

Theme: xolu-integration · Priority: P2 · Status: ☐ · Blocks/after: -

Trigger: 2026-11-18, while making the ways of starting a system agree with each other.

Found: the seam service says "build: ." and the repository has no Dockerfile. The xolu service names the image ghcr.io/ha1tch/olu:latest with a note that the name is unconfirmed. The file also gave Seam four variables it does not read (SEAM_SERVER_HOST, SEAM_SERVER_PORT, SEAM_XOLU_URL, SEAM_AUTH_JWT_SECRET), so the address of xolu and the JWT secret came from config.json whatever the environment said, and it started xolu without API v2, full text, time series and cascading delete.

Fixed in the tree (unreleased): the variable names are the ones Seam reads (SEAM_HOST, SEAM_PORT, XOLU_URL, SEAM_JWT_SECRET) and xolu's settings are the list of scripts/lib/xolu-env.sh; scripts/tests/test_makefile.py holds the file to both. DEPLOY.md says the compose path has not been run.

Scope: write the Dockerfile (a static seam binary; it embeds its assets), confirm or publish the xolu image, run docker-compose up on a clean machine, and have the seam service read its secrets from the environment only. Until then the supported ways are make launch and the demo packages.

### T-128. Seam does not check at start that xolu serves what its modules need (API v2 for Issues)

Theme: xolu-integration · Priority: P2 · Status: ☐ · Blocks/after: -

Trigger: 2026-11-18, T-121 and docs/ISSUES_MODULE.md.

Found: with xolu started without XOLU_API_V2_ENABLED=true, Seam starts, the list of issues opens, and reporting an issue answers 500 ("Failed to initialize issue workflow"). Nothing says why. The launch scripts now always set the variable; a deployment that starts xolu by its own means has no warning.

Scope: at start, ask xolu for one /api/v2 route and for the blob store, and log a clear error naming the setting when one is missing; show the same reason on the page instead of a bare 500.

## test-quality

### T-25. internal/ui/workflow's own dormant-guards table row already existed (predates this item); its pins are now current (T-85, 2026-08-13) but the module still doesn't compile -- real xolu internal-API breakage remains, tracked separately as T-86

Theme: test-quality · Priority: P2 · Status: ◐ · Blocks/after: Nothing currently blocking -- see T-86 for the remaining compile-breakage repair

Trigger: discovered 2026-07-31 while verifying seam-ui's build/test suite against xolu 0.19.3. go list ./... at the repo root doesn't include internal/ui/workflow -- it carries its own go.mod (module github.com/ha1tch/seam-ui/workflow-tests), invoked only via a separate 'make test-workflow' target, never via 'make test' or 'go test ./...'. That's a legitimate, deliberate dormant-guard design (see CHANGELOG [0.1.0-patched156]) -- the gap is that it was never added to docs/KNOWN_ISSUES.md's dormant-guards table (which currently lists only TestSchemaRoundTrip_XSeamExtensionsSurvive), so its own last-exercised date isn't tracked and its currency wasn't checked as part of any release gate.

Scope: (1) add internal/ui/workflow's six end-to-end tests as a second row in the dormant-guards table, with the make test-workflow XOLU_PATH=/path/to/xolu invocation. (2) Separately, and likely more work: the module's own go.mod pins xolu v0.9.7-patched74 (vs v0.19.3 now in the root go.mod -- a huge internal-API gap, since workflow_test.go imports pkg/cache, pkg/config, pkg/graph, pkg/server, pkg/storage directly, not just the stable pkg/client boundary), minty v0.1.0 (vs v0.2.1 root), queryfy v0.3.1 (vs v0.3.2 root, though MVS likely resolves this one up transitively via the seam-ui replace), tsqlparser v0.0.1, and go 1.22.2 (vs go 1.26.5 root). Given the size of the xolu internal-package gap specifically, actually running this against current xolu is likely to surface real compile breakage, not just a version-string bump -- needs its own pass, not folded into the routine xolu-bump work.

**Correction, 2026-08-13, prompted directly by the tech team asking whether any of the 27 open items had been unwittingly resolved by this session's own, unrelated work.** Checked rather than assumed: scope item (1) above (add to the dormant-guards table) was already done -- the table row exists, referencing this item by name -- but predates this item's own 2026-07-31 trigger, so it isn't this session's doing. Scope item (2)'s own prediction was directly confirmed correct by this session's separate T-40/T-42 work: T-85 (fixing internal/ui/workflow's own stale replace-directive path, a prerequisite this item's own scope didn't anticipate needing) ran `go mod tidy`, which genuinely updated every pin this item names -- confirmed directly against the current go.mod, not assumed: xolu v0.30.6, minty v0.2.1, queryfy v0.3.2, go 1.26.5. But exactly as this item's own scope (2) predicted, the pin bump surfaced real compile breakage in xolu's internal packages this module imports directly (pkg/config's own DBPath field removed, pkg/server.New's own constructor signature changed, a renamed UIHandler method) -- confirmed via go vet, not just theorized. That breakage is not fixed; it's tracked as its own item, T-86, deliberately scoped to effort-estimation rather than the fix itself. This item stays open, now ◐, until T-86 closes -- not closed outright, since "the pins are current" and "the suite passes" are different facts, and only the first is now true. The dormant-guards table's own "Last exercised" text (docs/KNOWN_ISSUES.md) corrected in the same pass to stop claiming stale pins that are no longer stale.

### T-61. Several test mocks for xolu.Client.Get responses wrap fields under a nested "data" key that Get() doesn't actually unwrap -- currently invisible since none of those tests assert on the affected fields

Theme: test-quality · Priority: P3 · Status: ☐ · Blocks/after: -

Trigger: found 2026-08-03 while writing a new test for AdminUserEdit's
formengine rebuild (MOD-8) -- the test's assertion on the fetched
email's value failed even though the handler code was correct, tracing
to the mock's response shape, not the handler.

xolu.Client.Get (pkg/client/client.go) uses the entire response body as
Entity.Data directly -- confirmed directly reading the client code, not
assumed: `var doc map[string]any; c.do(...); Entity{ID: id, Data: doc}`.
No "data" sub-object unwrapping happens. A mock responding with
`{"id": 1, "data": {"email": "...", ...}}` therefore produces an Entity
whose Data map has no top-level "email" key at all -- Data["email"] is
nil, and any code reading it (toString(existing.Data["email"]), etc.)
silently gets an empty string.

At least four existing test mocks in internal/ui/validation_handler_test.go
have this exact shape mistake for GET /api/v1 (Client.Get) responses:
TestRegionUpdate_MissingDescription_ReturnsForm, the Modelo update test,
TestAdminUserUpdate_InvalidRole_ReturnsError, and
TestAdminUserEdit_RendersNonEmptyCSRFToken (T-46, this session). None of
them currently fail, because none of them assert on a value that flows
through the broken field -- they check redirects, error messages, or
(for the CSRF test) an unrelated field entirely. The inaccuracy is
real but currently invisible.

By contrast: xolu.Client.Update's response body is ignored entirely
(Patch/Update return &Entity{ID: id} with Data: nil, confirmed directly),
and OQL responses genuinely do use {"status":..., "data": [...],
"stats": {...}} -- so this is specifically a Get()-response mock
mistake, not a codebase-wide convention error, and not something to
"fix" indiscriminately across every mock using the word "data".

Scope: fix the four existing mocks to the flat shape (confirmed correct
by TestAdminUserUpdate_LogsRoleChangeWhenRoleDiffers, this session's
own T-49 test, which already uses it correctly) -- mechanical,
low-risk, each mock's assertions stay the same, only the response shape
changes. Not fixed here to avoid scope creep mid a different task; the
one new test this session's work depended on
(TestAdminUserEdit_RendersEditFormFieldsAndDisabledEmail) was written
correctly from the start.

## cmms-module

### T-11. Scope the Order Engine / CMMS module (fifth module) and resolve its open design questions

Theme: cmms-module · Priority: P2 · Status: ☐ · Blocks/after: Depends on T-01, T-03 (Scope=assets), T-04 (Document/Image/Signature routing), T-08 (Field Definitions)

Trigger: "AMS Orders -- anotada.docx" v1.0, 2026-07-28. Full analysis in docs/ORDER_ENGINE_INTEGRATION_NOTES.md.
Scope: Most of Order Engine maps onto existing/planned primitives (Workflow->xolu /fsm, Numbering->xolu /gen/seq, Actions->xolu event defs not internal/webhook, Business Rules->same pattern as internal/rules but different scope). Genuinely new: Blocks (sub-units with independent FSM/responsible party), Scope (asset-selection constraint, no x-seam-form mechanism for this today), and two missing domain entities: Cuadrilla (crew) and Empresa/Proveedor (vendor) -- neither has an obvious home yet (Administration vs Settings vs core). Five open questions recorded in the integration notes doc $6 -- none answered yet. No module name settled ("Orders" vs "Maintenance" working titles).

Cross-reference, 2026-08-04, examining interdependencies between Settings/Administration/Account after T-02 closed: the "Administration vs Settings vs core" open question above is now more answerable than when filed. Administration and Settings have real, narrow, separately-permissioned scopes as of T-02 -- Administration is specifically user/role management (administration.user.*), Settings is specifically tenant configuration (settings.*). Neither is a natural fit for "a list of work crews" or "a list of vendors" -- both read as core masters data (matching regions/currencies/cost_centers/custodians' own existing home in internal/core/masters/), not an Administration or Settings concern. Not decided here -- T-11 is still unscoped -- but the boundary that made this question hard to answer before T-02 (one big System module, unclear where anything belonged) no longer exists.


## i18n

### T-32. password_reset.go's standalone page <title> hardcodes the app name ("Seam AMS") instead of reading h.config.AppName, unlike login.go which does

Theme: i18n · Priority: P4 · Status: ☐

Found 2026-08-01 during T-18 Stage 9. RenderForgotPasswordPage/RenderResetPasswordPage build their <title> as t(key)+"Seam AMS" -- a literal brand-name suffix -- while RenderLoginPage's own title correctly composes from data.AppName (falling back to "Seam AMS" only when unset). Not a translation bug (the brand name is a proper noun, correctly identical across locales), just a configurability inconsistency: a tenant with a custom AppName would see it on the login page but not on the two password-reset pages. Low priority, cosmetic, no user-facing report behind it.

### T-34. START.sh/START.cmd's 'Select UI language' menu is cosmetic -- SEAM_FORCE_LANGUAGE can never actually take effect once a tenant_settings row exists, and every seed script writes one unconditionally at build time

Theme: i18n · Priority: P3 · Status: ☐

Found 2026-08-01 while investigating a demo build's locale not matching the runtime language selection. server.go's own code comment states the design explicitly: SEAM_FORCE_LANGUAGE only supplies a starting default 'where none would otherwise exist' and is deliberately meant to be overwritten once the tenant's saved settings load -- loadPersistedSettings does exactly that, and re-reads every 30s via the background settings refresh besides. Every seed script (01-06) unconditionally writes a tenant_settings row with a hardcoded locale at build time, so that row always exists by the time START.sh runs -- meaning the language menu has never actually worked, for any seed, on any build; it always shows whichever locale that seed hardcoded regardless of what's picked. Restored the missing 4) Japanese (ja) option to both menu templates (a real, separate bug -- fixed) and worked around the deeper issue for this session's purposes by adding seed-06-asahikenki.py (locale=ja baked in at seed time, matching the only mechanism that actually works). The underlying gap remains: anyone picking a different seed and expecting the START.sh menu to change the demo's language will be surprised it doesn't. Three fix directions discussed with the tech team, none chosen yet: (1) parameterize --locale into each seed script and prompt for it at build time instead of runtime, dropping the now-fake START.sh menu; (2) make START.sh actually patch the already-seeded tenant_settings row via a curl call to xolu's own API before starting Seam (keeps today's UX, needs care: no jq/JSON-parsing guaranteed on the target machine per the script's own zero-dependency promise, and the logic would need writing twice -- bash and Windows batch); (3) drop the runtime menu entirely and document language as build-time-only, selected via which seed you pick.

### T-43. Existing Spanish-language specs (FORM_ENGINE_SPEC.md, ADMIN_MODULE_SPEC.md, DOCUMENTS_MODULE_SPEC.md, I18N_SPEC.md) need translation to English per the new internal-documentation-language invariant

Theme: i18n · Priority: P4 · Status: ☐ · Blocks/after: Nothing currently -- not blocking any in-flight work

Surfaced 2026-08-02 as a direct consequence of the new "Internal
documentation language" invariant (docs/KNOWN_ISSUES.md): all existing
specs -- FORM_ENGINE_SPEC.md, ADMIN_MODULE_SPEC.md,
DOCUMENTS_MODULE_SPEC.md, I18N_SPEC.md -- are written in Spanish
prose, and RBAC-COMPLETION-PLAN.md is already partly English. These
predate the decision and aren't retroactively translated by it alone.

Not scoped or estimated. Real considerations for whoever scopes it:
translation quality for technical/design prose (not a mechanical
find-replace -- these are genuine engineering documents with nuance),
whether to translate in one pass per document or incrementally as each
spec is next touched for substantive reasons, and whether any of these
docs are stale enough (see T-08's finding on RBAC-COMPLETION-PLAN.md
describing infrastructure as "missing" that's since been built) that
translation should wait until after a content reconciliation pass
rather than translating text that needs rewriting anyway.

### T-58. Preferences form's timezone select shows raw IANA identifiers (America/Montevideo) instead of human-readable labels the old hand-coded version had -- 76 i18n keys needed for full parity, not added

Theme: i18n · Priority: P3 · Status: ☐ · Blocks/after: -

Trigger: found 2026-08-03 while building MOD-8's Preferences form.
formengine's resolveEnumOptionLabel (i18n.go) is the only mechanism for
translated <select> option labels -- confirmed directly against
FORM_ENGINE_SPEC.md §4.2, which has no schema-level value+label pair
syntax, only bare enum arrays. It works via a per-value i18n key,
"<form>.<field>.<lowercased value>".

Added these keys for locale (4 values) and dark_mode (3 values) --
small sets, previously had real human-readable labels in the hand-coded
version being replaced (e.g. locale showed "English"/"Español", not raw
codes). Deliberately did NOT add them for timezone: 19 IANA zone
identifiers (America/Montevideo, Europe/London, etc.) across 4 locales
is 76 key/value pairs. The old hand-coded CommonTimezones() had nice
labels too ("Uruguay", "Eastern Time (US)"), so this is a real, visible
difference from what shipped before, not a non-issue -- flagging it as
a finding, not quietly accepting it as fine.

Scope: either (a) add all 76 entries, reusing CommonTimezones()'s
existing English labels as the source and translating the other three
locales, or (b) accept raw IANA identifiers as sufficiently self-
descriptive for a timezone selector specifically (unlike locale/
dark_mode, IANA zone names are a recognisable, semi-international
convention many products show unlocalized) and close this as
won't-fix. Not decided here -- a judgement call, not purely mechanical.

### T-122. Issue templates: a name with no Latin letters or digits is refused as empty (the key comes from slugify)

Theme: i18n · Priority: P3 · Status: ☐ · Blocks/after: -

Trigger: found 2026-11-18 while writing the Japanese Issues seed (T-120).

Found (from the code, not run): internal/ui/issue_templates.go makes a template's key with slugify(name), which keeps a-z and 0-9 only. A name written entirely in Japanese, or in any script without Latin letters, gives an empty key, and the create handler answers with the "name required" error. The seed writes its templates with keys made from the English names, so the seeded data is not affected.

Scope: make the key independent of the script of the name (a generated key, or one the user can type), keeping existing keys as they are; add a test with a Japanese name.

### T-129. Issues stores the asset category as the Spanish word critico

Theme: i18n · Priority: P3 · Status: ☐ · Blocks/after: -

Trigger: 2026-11-18, docs/ISSUES_MODULE.md. The rule of the project is that stored constants are English.

Found: issues.asset_category and issue_qr_codes.asset_category hold "critico" or "normal" (assetCategoryFromCriticality, issueCategoryFilterValues, the QR landing and report). The same class as T-23.

Scope: store "critical" and "normal"; migrate the stored values; the seven Issues seeds and the conformance expectations change with it.

### T-133. Interface text outside the locale files: page chrome, a confirmation, Spanish fallbacks and 101 error replies

Theme: i18n · Priority: P2 · Status: ◐ · Blocks/after: Shipped in 0.4.5; open: what the body lists as left

Trigger: 2026-11-18, an audit of the interface for text that does not go through the locale files (a scan of every string literal in the non-test Go code by the call it is an argument of, the browser scripts, the embedded form definitions and the four locale files).

Found: the locale files themselves are sound (2659 keys in each, no pending marks, 16 values equal in all four and all of them names or codes). What is outside them, in the pages:

- Chrome of every page, in English whatever the language: internal/ui/layout.go ("Skip to main content", "Open menu", "Notifications", "User Profile", "Log Out" twice), internal/ui/components.go (the table header "Actions", "Previous" and "Next" of the pager), aria-label "Tabs" in pages.go and admin_users.go, "Clear" on the signature widget (internal/formengine/widgets.go:277).
- internal/ui/admin_users.go: the confirmation "Are you sure you want to delete %s?" and the one list whose ListActions has no translator, so its buttons say Ver, Editar, Eliminar in every language.
- In Spanish whatever the language: the tooltip "Ver / editar" (internal/formengine/renderer.go:707), and the reply of internal/ui/consumable.go:384, which is also in voseo.
- Spanish literals kept as fallbacks from the staged rollout (T-18), now reached by no caller or only when a key is missing: listing.go:316, masters_pages.go:54, 99, 122 and 932, and four in the form engine (action.add, action.remove, action.reorder, action.add_link). The interface's base language is English; these should be English or go.
- Error replies written with http.Error: 101 in internal/ui, 46 distinct texts ("Invalid form data" 21 times, "not found" 13, "Failed to save ..." and so on), in English. Some reach the person through htmx; which ones do was not established.

Scope: keys for the chrome and the confirmation (about 15), the translator for the Users list, English for the fallbacks, and a decision on the error replies: one translated family (bad form, not found, could not save) in place of 46 literals. A test that fails on a literal passed to a minty text node in internal/ui would keep it from coming back.

Progress (2026-11-18, not released): the chrome of the page goes through the locale files (layout.skip_to_content, .open_menu, .notifications, .user_profile, .logout, .tabs; the pager takes action.previous and action.next; the signature's Clear and the tooltip of a linked field have keys). The Spanish fallbacks are English (listing.go, masters_pages.go, the form engine's add, remove, reorder, add link and the two empty options of a select). The Spanish reply of consumable.go is a key, in tuteo. 89 of the 101 error replies go through UIHandler.errText with a family of keys (error.invalid_form_data, .not_found, .server_error, .search_failed, .upload_failed, .missing_file, .file_too_large, .save_failed, .load_failed, .delete_failed, .invalid_id, .auth_required, .name_required, users.error.not_found, users.error.own_developer_role), the English literal kept as the fallback. The Users list already passed its translator: the audit was wrong about it.

Left: 12 replies that name a program fault for a developer stay in English by decision (unknown picker, unknown selector, invalid schema, missing key, the SVG to PNG conversion, the login page's own render failure); two pieces of dead code still hold English literals (Table's "Actions" header and userActions' confirmation in admin_users.go, neither called); a test that refuses a literal in a text node was not written.

### T-134. Texts in the browser scripts do not go through the locale files

Theme: i18n · Priority: P3 · Status: ◐ · Blocks/after: Shipped in 0.4.5; open: what the body lists as left

Trigger: 2026-11-18, the same audit.

Found: web/static/js/app.js has 7 texts of its own ("Copied to clipboard", "Failed to copy", "Confirm deletion", "Confirm", "Are you sure you want to delete", "Please select an item from the list", and "Error al cargar" in Spanish at line 448); file-upload.js has 4 ("Could not load existing images.", "Upload failed.", "Could not delete image.", "Add photos" as a fallback). The pickers already take their texts from data attributes the server fills (data-t-none and the like), and the cancel label comes from data-i18n-cancel on the page: the same way serves these. The FSM editor (fsm-editor.js, seam-fsm-editor.js) has about 50 more, its toolbar titles and the texts of its sample machines; it has a lookup of its own and was not gone through.

Scope: the 11 texts through data attributes or one object the layout writes; then the FSM editor, counted properly first.

Progress (2026-11-18, not released): the 11 texts of app.js and file-upload.js come from the page. The layout writes them in one attribute of <html>, data-i18n-texts, from the keys js.* (scriptTexts, internal/ui/layout.go) and the scripts ask SeamText(name, fallback). The delete question is one sentence with {name} in it, so each language places the name. "Error al cargar" is gone.

Left: the FSM editor (about 50 texts), not gone through; the login and password pages build their own <html> and do not carry the texts, so a script there shows its English.

### T-137. Values stored in Spanish are shown as stored: asset criticality, ownership and disposal, the chart of accounts

Theme: i18n · Priority: P3 · Status: ◐ · Blocks/after: Shipped in 0.4.5; open: what the body lists as left

Trigger: 2026-11-18, the same audit. Goes with T-129, which is one case of it.

Found: some values are stored in Spanish and shown as stored. An asset's criticality is "Alta", "Media" or "Baja" in the data, and the Assets list and the asset page print it in a badge as it is (internal/ui/assets.go:70, pages.go:1629), though asset.criticality.high, .medium and .low exist in the locale files. internal/ui/asset_xolu_fields.go and masters_xolu_fields.go translate between the Spanish stored words and English ones for ownership (Propiedad, Comodato, Préstamo, Alquiler), disposal (Venta, Desguace, Robo, Donación, Otros), alert type and level, in maps written in the code. Accounting makes its chart of accounts with Spanish names and notes (internal/accounting/chart.go: 23 names, "Activo", "Pasivo", "Ingresos por servicios"), and writes entry descriptions in English ("Settlement #%d paid to %s", "Reversal of entry #%d").

Scope: decide per case. For the asset fields: store a neutral code and show its translation, with a migration of what is stored; or keep the stored words and translate at display, which needs no migration. For the chart of accounts: the books have one language, the company's; make it a setting of the module, not of the viewer. Entry descriptions the same.

Progress (2026-11-18, not released): the criticality of an asset is shown in the viewer's language in the Assets list and in the alert line of the asset page (criticalityLabel), translated at display with no change to what is stored.

Left, each a decision: ownership, disposal reason and alert words, still stored in Spanish and mapped in code; the chart of accounts and the entry descriptions of Accounting; T-129.

### T-138. Dates and amounts are formatted the same for every language

Theme: i18n · Priority: P3 · Status: ◐ · Blocks/after: Shipped in 0.4.5; open: what the body lists as left

Trigger: 2026-11-18, the same audit.

Found: dates and amounts are formatted one way for every language. internal/ui/routes.go:2205 and 2908 write dates as "2 Jan 2006", with English month names; amounts are written "USD 1,160.00" by accounting.Cents.String for everyone, where Spanish and Portuguese write 1.160,00; a date input shows mm/dd/yyyy or dd/mm/yyyy by the browser's own language, not Seam's. T-58 (time zone names) is the same kind of gap.

Scope: one place that formats a date and a number for a locale, used by the pages; amounts in the books keep one format inside the module and take the viewer's at display.

Progress (2026-11-18, not released): dates written by the pages follow the language (formatDay; keys date.short and date.mon.1 to .12): "2 Nov 2026", "2 nov 2026", "2026年11月2日". The two places that wrote "2 Jan 2006" use it.

Left: amounts ("USD 1,160.00" for everyone), date inputs, which follow the browser, and T-58.

## formengine

### T-19. Asset-picker widget -- built and shipped (T-72, T-76); self-reference exclusion and the separate listing-search-bar question remain genuinely open

Theme: formengine · Priority: P2 · Status: ◐ · Blocks/after: Not required for FE-24/production validation -- parent_id's numeric-entry fallback is a known, documented, working limitation (SPEC §8.3), not a blocker

Trigger: the tech team, 2026-07-29 -- asked whether asset-picker could reuse an existing search widget rather than being built from scratch, then chose @tarekraafat/autocomplete.js and asked for Seam re-theming.

Done so far: autocomplete@10.2.10.min.js + autocomplete@10.2.10.css vendored (web/static/vendor/, documented as Asset 6 in VENDOR.md, verified byte-identical to the real npm package via direct npm install + diff, not just downloaded from a CDN mirror and trusted). New web/static/css/autocomplete-seam.css re-themes it to match Seam's light/dark palette (via var(--color-primary), already theme-aware, no JS wrapper needed -- confirmed minty's DarkModeTailwind only toggles class="dark" on <html>, no custom event fires, none needed) and fonts (system-ui stack, copied verbatim from app.css). Fixes a real, confirmed upstream limitation, not assumed: the default highlight colour (#ff7a7a, coral-red) is applied directly to  with high specificity (GitHub issue #264, a known, open, unfixed upstream bug) -- autocomplete-seam.css uses the identical selector chain to win by load order, no !important needed. Dropdown styling matches the real, existing dropdown-menu convention used elsewhere in Seam (layout.go's user-menu popover: bg-white dark:bg-gray-800, not guessed). New TestVendorFilesPresent entries for both vendored files.

Remaining, not started: (1) backend search endpoint -- confirmed empirically that xolu's OQL already supports the exact query shape needed (id = X OR name LIKE '%Y%', case-insensitive, verified against a real running instance with real test data), but no Go handler exists yet to expose this for the widget to call. (2) formengine wiring -- actually replacing the asset-picker widget's current number-input MVP fallback (schema.go/widgets.go) with the real, configured autoComplete.js instance, including how results get selected and written back into the hidden parent_id value. (3) A genuine scope decision, not yet made: the spec's own description of this widget is considerably more than a search box ("a selector with search and a preview of the containment tree"), and x-seam-relation supports both contains and uses, meaning the widget's meant to be reusable beyond just parent_id -- full vision vs a scoped-down search-and-pick version needs an explicit choice, not a default assumption. (4) Cycle/self-reference handling -- confirmed xolu has real cycle detection at the graph layer but in warn mode by default, not blocking; whether to configure that globally or filter self/descendants out at the application layer is undecided. (5) No image icon vendored deliberately -- the upstream CSS's background-image search icon was removed rather than kept, since adding an icon wasn't asked for; worth revisiting if a search icon is wanted later.

Not urgent: parent_id's current MVP fallback (a plain number input with a visible note) is a known, working, documented limitation, not a broken feature -- SPEC §8.3 already describes it as deliberate. This item tracks finishing the real widget, not fixing something broken.

**Status correction, 2026-08-06, found while grounding a "what's next" answer**: this item was never updated when the work it describes was actually completed, and sat at ☐ describing already-finished work as if it were unstarted -- a real register-accuracy gap, not a status we asked about directly. Of the five "remaining, not started" items above: (1) backend search endpoint -- done, T-72 (internal/ui/asset_picker_search.go). (2) formengine wiring -- done, T-72 (renderAssetPicker replacing the number-input stopgap). (3) the scope decision -- resolved, the tech team's own call: one generic, schema-configured picker (x-seam-source/x-seam-search-fields/x-seam-display-fields), not the fuller containment-tree vision, matching what shipped. (5) no search icon -- still true, still deliberate, unchanged. Genuinely still open: (4) cycle/self-reference exclusion -- the search endpoint's own exclude= parameter exists and is tested, but was deliberately never wired into the widget's JS (documented directly in renderAssetPicker's own comment: a naive always-exclude would be wrong for a cross-entity picker like Sensor->Asset, and building it correctly needs RenderContext to carry which entity the record being edited belongs to, which it doesn't yet). Also still open, not part of the original five: the separate "scope extended" question below about reusing this for SearchInput/ListFilterBar's own listing search bars -- never revisited after being raised. T-76 (Issue Tracker) since confirmed the widget generalizes to a third, independent consumer with zero widget code changes, direct evidence for whichever of these gets picked up next.

Scope extended, 2026-07-29 -- we asked whether the same vendored autoComplete.js could also power an optional search bar in "the listing invariants". Investigated rather than assumed: internal/ui/listing.go's ListPageConfig/ListFilterBar and forms.go's SearchInput are real, confirmed shared components -- SearchInput specifically is the true invariant, used directly by assets.go and wrapped by both ListFilterBar (sensors list only, so far) and a third, separate FilterBar helper. Confirmed technically reusable: autoComplete.js supports multiple, independently-configured instances per page, so the same vendored files/theme CSS apply with zero duplication regardless of which input they're attached to.

A real, undecided difference in kind, not just wiring: SearchInput today submits a plain GET form (type, submit, full server-side re-render filtering the whole list). autoComplete.js's native behaviour is per-record live suggestions, not list-level filtering. Two genuinely different, both legitimate integration choices, neither picked yet: (a) jump-to-record -- suggestions are individual assets, selecting one navigates straight to that asset's detail page, bypassing list filtering entirely; (b) typing-assistant -- suggestions help find the right search term, selecting one fills the box and auto-submits the existing GET form, leaving the current list-filtering behaviour untouched. Likely shares the same backend search endpoint needed for the picker itself (item 1 above), not a separate piece of backend work -- folded into this same item rather than filed separately for that reason.

Design substantially clarified, 2026-07-29, by the tech team directly rather than left as an open question for long:

Listings: not jump-to-record nor typing-assistant as separately guessed -- a third option, confirmed technically real rather than assumed: autoComplete.js's release notes document a specific bug fix for resultsList being disabled ("errors when resultsList is disabled due to the attachment of the keydown event"), confirming this is a real, tested, first-class configuration, not a hack fighting the library's own design. SearchInput becomes: the library's own debounced query/match engine keeps running against the input, but nothing renders as a dropdown -- its result/response event instead triggers live filtering of the actual table already on the page (most naturally via HTMX, matching how the rest of Seam already re-renders server-side), turning today's "type, press Enter, list filters" into "type, list filters live", same underlying query, no separate results UI.

Asset/Sensor picking: confirmed, not aspirational, that multi-field result display is directly supported -- resultItem.element/resultsList.element are documented customization hooks handing back the raw result data, letting arbitrary HTML render per row (e.g. name + serial + location together, to disambiguate similar assets), not a workaround.

Architecture: resolved, not left open -- build one generic, schema-configured picker widget, not an AssetPicker. This directly extends x-seam-source's existing entity:<name> convention (already proven reusable across entities/forms for the plain FK-select widgets: brand_id -> entity:marcas, region_id -> entity:regiones), adding two new schema attributes: x-seam-search-fields (which fields the backend matches against) and x-seam-display-fields (which fields render per result row). A future Sensor-picking field looks identical in shape, just pointing at entity:sensors with its own field lists -- one widget implementation, one generalized backend endpoint (parameterized OQL: id = X OR field1 LIKE Y OR field2 LIKE Y ..., extending the id/name pattern already confirmed working), zero new Go code per entity type. Matches an already-working pattern in this codebase, not a new one invented for this.


### T-20. Reconsider extracting internal/condition to a standalone repo (e.g. github.com/ha1tch/condition), matching the ha1tch ecosystem's other sovereign libraries

Theme: formengine · Priority: P3 · Status: ☐ · Blocks/after: Nothing -- no deadline, explicitly deferred

Trigger: the tech team, 2026-07-30 -- while consolidating four independently-evolved condition/operator evaluators found across Seam (formengine/visibility.go, its hand-mirrored client-side JS, rules/engine.go, fsm/evaluator.go) into a single package, the instinct was that this belongs as a proper, exported, general-use library, matching Seam's own established pattern (minty, xolu, queryfy, scrapex, tsqlparser, fsm-toolkit are all separate, sovereign repos, not internal subpackages).

Explicitly decided against doing that now: internal/ genuinely cannot be imported outside this module regardless of naming, so a real exported package means a new standalone repository, not just a rename -- real, additional scope (repo setup, versioning, its own release process, deciding whether the comparison semantics should be configurable rather than Seam-specific-and-hardcoded, so a different consumer could actually adopt it without forking) beyond what's needed to stop the immediate drift. Chose internal/condition instead, pragmatically, to make progress on the actual, current need (formengine's visibility system specifically -- not rules/engine.go or fsm/evaluator.go, which are separate domains with no JS/minty relationship and were explicitly excluded from this consolidation).

Revisit later, no deadline: is internal/condition a good enough shape, on its own merits (has it stayed clean, has anything else in Seam wanted to depend on it, would extraction still be worth the overhead) to justify the move to a standalone repo, or was the original 'general use' instinct better served by leaving it internal all along. Not a regret to preempt -- a genuine open question worth an honest look once there's more evidence either way.

### T-21. visibilityScript (renderer.go) still hand-written, manually duplicating internal/condition's semantics in JS -- not generated from it

Theme: formengine · Priority: P2 · Status: ☐ · Blocks/after: None currently -- deliberately deferred, not a blocker for anything in flight

Trigger: internal/condition consolidation (2026-07-30, see T-20). The Go-side evaluator (visibility.go's EvaluateVisibleWhen/evaluateOp) now delegates entirely to internal/condition -- a single source of truth for Seam's own comparison semantics (false is not empty, gte/lte included, etc). The client-side mirror in renderer.go's visibilityScript function is untouched: still a separate, hand-written JS implementation of the same logic, manually kept in sync by whoever edits either side remembering to update both.

This is exactly the kind of drift risk internal/condition's own consolidation was meant to close, and it isn't closed yet for the JS half. The natural fix, explicitly discussed and deferred rather than decided against: generate visibilityScript's logic from internal/condition's own Go definitions, the same pattern minty/mintydyn already uses for its own generated JS (though not by depending on mintydyn itself -- that relationship was explicitly ruled out, see T-20's body). Scope not yet estimated. No current known divergence between the two sides (both were audited together during T-20's investigation and found consistent), so this is preventive, not a live bug -- worth doing before the next time either side gets touched independently, not urgently before that.

### T-35. formengine has no unit-of-measurement concept for scalar fields -- temp_min/temp_max bake "(°C)" directly into the translatable title text instead of a structured, convertible unit

Theme: formengine · Priority: P3 · Status: ☐

Found 2026-08-01 while working through T-33's listbox fixes, prompted by the tech team noticing the mixed concerns. Checked: no x-seam-unit / unit_of_measure / UnitOfMeasure keyword exists anywhere in internal/formengine or docs/FORM_ENGINE_SPEC.md. asset.json's temp_min/temp_max are the only fields that bake a unit into the field title itself (assets.forms.asset.field.temp_min = 'Min. Temp. (°C)') -- meaning the unit is baked into translatable prose, not a structured, independently-addressable concept: no way to display Fahrenheit for a tenant that wants it, no conversion logic, and the unit can't be changed without editing the label text in every locale. Notably the codebase already has two better patterns for the same underlying problem, just not applied consistently: purchase_price/current_price's unit is a separate structured reference (currency_id, itself a real xolu entity with its own symbol/code), and service_configs.threshold's unit is determined dynamically by its sibling 'measure' field in the same repeatable row (Kilometers/Hours/Uses, T-33). temp_min/temp_max are the outlier, hardcoding a single unit directly in prose instead of using either pattern. Whether this deserves a proper x-seam-unit schema keyword (with an associated i18n-labeled unit table and optional conversion, similar in spirit to how connectivity_tier/criticality/etc. are handled) or is a narrow enough case to just fix locally (make temp_min/temp_max's unit a sibling field or schema attribute) is an open design question -- filed rather than decided unilaterally, since it could be scoped as either a two-field fix or a formengine capability extension depending on how many future fields end up needing it.

### T-41. formengine needs jurisdiction-conditional field visibility + a tenant jurisdiction setting distinct from tenant_settings.locale

Theme: formengine · Priority: P4 · Status: ☐ · Blocks/after: Nothing currently -- not scoped, not blocking anything in flight

Raised 2026-08-02 during Custodio's T-40 rename work: `subject_type`'s
DNI/CI/RUT/RUC options are Uruguay-specific (tracked narrowly as T-28 --
"widen the enum"), which surfaces the broader, unfiled question:
formengine has no way to say "this field only makes sense for
jurisdiction X" at all.

Two related pieces, since the second is a prerequisite for the first:

1. New schema condition type, keyed by jurisdiction/locale rather than
   another field's value. What exists today (`x-seam-visible-when`,
   `VisibleWhen`, `internal/formengine/schema.go` +
   `hidden_field_policy.go`) only evaluates conditions against the same
   record's other fields (e.g. Asset's `loan_expiry` visible only when
   `tenure_type == 'Loaned'`).
2. Data-preservation risk: the existing `CheckHiddenFieldSubmission`
   discard-on-submit policy was designed for same-record conditions,
   where the field genuinely no longer applies to *this record*.
   Jurisdiction-based hiding is different -- hidden because of *who's
   viewing it*, not anything about the record. Not yet checked directly
   against the current implementation whether "discard" is actually
   destructive here (depends on whether the calling handler's update
   mechanism does a true partial-patch or a full-replace at the xolu
   layer) -- verify before designing the fix, not after.
3. Foundational gap: no tenant-level jurisdiction/region setting exists
   at all -- only `tenant_settings.locale`, display language only. A
   system can run under US jurisdiction (US legal codes, US accounting
   practices, US measurement units) while a user views the interface in
   Spanish -- the business doesn't become Mexican because someone reads
   it in Spanish. Locale and jurisdiction are orthogonal axes today
   conflated into a single setting that doesn't actually capture the
   jurisdiction concept.

Not scoped or estimated -- deliberately not designed under the momentum
of the unrelated Custodio task. Related to T-28 in spirit (T-28 is the
narrow "the enum's too small" fix; this is the broader capability
question of whether the field should even be shown to every
jurisdiction at all) but filed separately, not conflated.

Cross-reference, 2026-08-04, examining interdependencies between
Settings/Administration/Account after T-02 closed: point 3's
foundational gap (no tenant-level jurisdiction setting) now has a
concrete, real place it could actually live -- Settings' own
settings_general.json schema and SettingsSaveGeneral handler, both real
since T-02. Before T-02, "the Settings module" wasn't a stable enough
target to design against. Not scoped here, not a decision to add it --
just noting the "where would this go" question T-08 itself would have
faced is now answerable, whenever this item is picked up.

### T-57. formengine's visibilityScript IIFE duplicates verbatim once per Render() call on a page -- was hypothetical-future, now current: Settings' 3 sections (built 2026-08-04) already trigger a 3-way duplication on one live page, though still no visible_when conditions exist anywhere to make it user-visible

Theme: formengine · Priority: P2 · Status: ☐ · Blocks/after: -

Trigger: found 2026-08-03 while building MOD-8's Profile/Preferences
rendering. formengine's renderForm (renderer.go) unconditionally emits
visibilityScript(b) once per call -- a self-contained ~80-line IIFE
implementing DASHBOARD_SPEC.md/FORM_ENGINE_SPEC.md §3b.6's client-side
visible_when evaluator. Every existing caller before this session
(AssetCreateDynamic/AssetUpdateDynamic, the three masters dynamic
pages) renders exactly one form per page, so the script only ever
appears once. Profile is the first page to call FormEngine.Render()
twice (profile + preferences, two independent <form> elements) --
confirmed directly by rendering and grepping the output: the identical
script block appears twice, verbatim.

Not a functional bug for the current profile/preferences schemas
specifically (neither declares any x-seam-visible-when condition, so
both IIFE invocations are genuine no-ops -- querySelectorAll('[data-
visible-when]') finds nothing either time). But it is wasted bytes sent
to the browser on every /profile request, and the underlying pattern
(one visibilityScript per Render() call, not deduplicated per page)
would become a real problem the moment any future multi-form page
(Settings' three sections, once built, are the next candidate) uses a
schema that actually has visible_when conditions -- duplicate event
listeners registering the same watchers twice isn't dangerous exactly,
but isn't right either.

Scope: not fixed here -- the fix touches renderForm/renderer.go, shared
by every formengine caller in the codebase, not something to change as
a side effect of building Profile. Needs its own consideration: either
(a) emit the script once per page regardless of how many Render() calls
happen (requires some form of caller-visible "already emitted" state,
which renderForm doesn't have today and FormEngine is currently
stateless/reusable across calls), or (b) have the caller (routes.go's
Profile handler, and any future multi-form page) render forms' field
content only and call visibilityScript once itself, outside the
per-form loop -- but renderForm doesn't expose fields-without-script as
a separate call today. Neither decided here.

Refined 2026-08-04, adversarial tracing (no code change, understanding
only): traced through what actually happens once a real visible_when
condition exists on a multi-form page, rather than leaving this at
"duplicate event listeners isn't dangerous exactly." The duplication is
worse than a predictable 2x: each IIFE's querySelectorAll('[data-
visible-when]') is genuinely global (not scoped to "this form's own
fields"), and each IIFE runs synchronously at the point its own
<script> tag is reached during sequential HTML parsing -- so an IIFE
emitted by an earlier form on the page finds only elements that already
exist in the DOM at that point, while a later form's IIFE, running
after more of the page has parsed, finds and re-wires EVERY qualifying
element seen so far, including ones an earlier IIFE already wired. Net
effect: an earlier form's visible_when field accumulates one extra
duplicate listener per later form on the same page; a later form's own
field gets none. Not a fixed 2x -- an unpredictable, DOM-position-
dependent count. Still not dangerous (apply() is idempotent, so the
final displayed/required state is correct regardless of how many times
it's set) -- multiple redundant evaluations per user keystroke on
affected fields, not incorrect ones. Priority/scope decision unchanged
by this refinement.

Priority bumped P3->P2, 2026-08-04, examining interdependencies between
Settings/Administration/Account after T-02 closed. This item's own
2026-08-03 text named "Settings' three sections, once built" as the
next candidate for this bug to stop being hypothetical -- those three
sections (settings_general.json/settings_notifications.json/
settings_features.json) were built the same day (MOD-8) and are live
today, meaning /settings now renders 3 independent visibilityScript
copies per page load, not 2. The risk surface didn't just grow, it
stopped being a "future page" question and became a "this page,
right now" one. Still not user-visible (no visible_when condition
exists anywhere in Account/Settings/Administration's schemas yet), so
not bumped to P1 -- but no longer purely speculative either.

### T-100. 0.4.1-pre8 was written without a Go toolchain or a browser: the browser pass is outstanding (build, vet, gofmt, icon check and tests green at 0.4.3, 2026-11-17)

Theme: formengine · Priority: P1 · Status: ◐ · Blocks/after: A browser pass, light and dark

Trigger: the pre8 session had no network and no Go; everything in the 0.4.1-pre8
changelog entry was reviewed by reading (bracket balance, call sites, helper
signatures) and has never been compiled or run.

Progress 2026-11-17 (v0.4.3, Go 1.26.5): steps 1 and 2 are done. go build, go vet ./... and go test ./... -count=1 are clean over all 35 test packages; gofmt -l . is empty after formatting 12 files; scripts/diff-locales.py reports identical key sets; scripts/check-icons.py confirms all 150 icon names. Steps 3 to 6 need a browser run; a headless Chrome is available in the sandbox and the repository's scripts/browser-test-*.js can drive it, not yet run.

To close, in this order:
1. `go build ./...`, `go vet ./internal/formengine/... ./internal/ui/...`, `gofmt -l .`
   (the three new `pickerSearchConfigs` lines are probably not aligned), then
   `go test ./internal/formengine/... ./internal/ui/...`.
2. `make check-icons` and `scripts/diff-locales.py` (the latter was run: identical key sets).
3. Browser, light and dark: the four new Demo tabs; the stacked-cells fix on the Demo
   "Lines" rows; Asset form > Service Configurations, which is now one line of three cells
   in a half-width column and may be too cramped (decide: leave, or give it `x-seam-layout: form`).
4. Open one Selector per entity and check the dialog title: nothing in the widget code
   reads `selector.title.<key>`, so where the title comes from is unverified.
5. Add and remove rows in every new tab, including rows added after load (the template
   rows carry the custom remove button and the accent).
6. Row effects (`SeamRowFx`, unreleased): rows grow in on add and shrink out on remove in every
   Demo tab, flat and indexed; removing the last row and adding again still works (the flat
   handler clones the row, which must be back to normal when it does); nothing animates
   outside the Demo page or under reduced motion. `node --check app.js` passed; nothing else ran.
Progress 2026-10-13: the eight Demo tabs were run in a browser (79 checks: T-94 scenario on every unbounded repeatable, no script errors, selects enhanced once). Still to run: Asset form > Service Configurations, the Selector dialog (htmx search, Prev/Next) and a select inside SeamModal.


### T-97. any-picker formengine widget: generalise asset-picker to any entity, add a users source

Theme: formengine · Priority: P2 · Status: ☐ · Blocks/after: Extends T-19; Go toolchain; tenancy-model question

Trigger: instruction 2026-10-07 -- a user picker formengine element, preferably one generic any-picker configurable for any entity instead of a second single-purpose widget.

Not started (blocked on the Go toolchain). The existing "asset-picker" widget (renderAssetPicker, internal/formengine/widgets.go) is already generic over `x-seam-source: "entity:<name>"` and calls the shared endpoint GET /picker/search (internal/ui/asset_picker_search.go); only the server registry pickerSearchConfigs (one entry, entity:assets) is asset-specific. Plan: register "any-picker" in widgetRegistry pointing at the same renderer (keep "asset-picker" as a backward-compatible alias), grow the registry (entity:users first), and generalise pickerSearchConfig from {table, one searchColumn, label func} to also allow a fetch function so the Users entry can go through auth.UserService instead of raw OQL. Optional browse button that opens the Selector dialog (separate item). The widget JS never interpolates record labels into script (T-63/T-64); keep that.

Existing tests to mirror: internal/ui/asset_picker_search_test.go (5 tests) and internal/formengine/fk_popover_test.go (4 AssetPicker tests). Extends T-19. Details: docs/HANDOVER_2026-10-07.md section 7.

### T-103. Masters-page DocumentLinksField still has the old hand-coded look

Theme: formengine · Priority: P3 · Status: ☐

Trigger: the Document Links review, 2026-10-12. The engine's `document-links` widget now renders as the flat repeatable (same rows, remove and add buttons); the hand-coded `DocumentLinksField` on the masters pages (`internal/ui/masters_pages.go`, script in `doclink_script.go`, parsing in `masters_routes.go`) still has the old look (red cross, bare inputs, text-link add button) and its own `.doc-link-row` handlers. Migrating it needs a decision on its field-name format first (`prefix` plus index versus the engine's parallel arrays). The engine-side old look stays available through `features.document_links_legacy`.

### T-104. Inline validation styling for form fields

Theme: formengine · Priority: P3 · Status: ☐

Trigger: item 4 of the visual-components review, 2026-10-12, not yet started. Fields have no inline validation state beyond the red border on a server-rendered error: no per-field message under the input, no live state on blur, no valid/invalid styling shared with the new combobox trigger (which already has an error style, `.fe-ss-error`).

### T-105. Optional esbuild minification of app.js when esbuild is available

Theme: formengine · Priority: P3 · Status: ☐

Trigger: decision of 2026-10-12. `web/static/js/app.js` ships unminified and the server has no minification; only gzip (`features.compress_static`) applies. Wanted: an OPTIONAL build step that minifies with esbuild when it is on the PATH and otherwise ships app.js unchanged, adding no dependency to the repository. Needs an output name (the embedded file must stay the readable source) and a way for the layout to pick it up. Not needed now.

### T-107. UPC-E barcodes cannot be drawn: the vendored JsBarcode bundle omits UPCE from its format table

Theme: formengine · Priority: P3 · Status: ☐

Trigger: found 2026-10-21 while building the Kitchen demo tabs. A UPCE barcode field was checked against `web/static/vendor/jsbarcode-ean-upc@3.12.3.min.js` under node before it went into the demo.

The vendored bundle defines a UPCE encoder, but its format table maps only `UPC`: there is no `UPCE` entry. JsBarcode therefore throws for `format: "UPCE"`, and `renderBarcode` catches the error and clears the SVG, so a field with `x-seam-barcode-format: "UPCE"` silently draws nothing. `schema.go`, the designer catalogue's enum and the widget catalogue all advertise UPCE. EAN13, EAN8 and UPC were verified valid with sample codes, and a wrong check digit is rejected as expected.

Scope: either re-vendor a bundle whose format table exports UPCE (and update VENDOR.md, Asset 7), or drop UPCE from `schema.go` and the designer enum. Either way, add a test that fails while an advertised format cannot be drawn. The Kitchen demo has no UPC-E field until this is fixed.

### T-108. number widget ignores minimum, maximum and step although the catalogue says it supports them

Theme: formengine · Priority: P3 · Status: ☐

Trigger: found 2026-10-21 while building the Kitchen demo tabs, comparing the widget catalogue (which says `number` supports `minimum`, `maximum` and `step`) with `renderNumber`.

`renderNumber` emits only type, name, id, value and required, and `Field` does not parse `minimum`, `maximum` or `multipleOf`, so a schema that sets them gets no bounds and no step in the browser. The implementation is behind the catalogue, and the catalogue states the intended behaviour.

Scope: parse `minimum`, `maximum` and `multipleOf` (rendered as `step`) in `schema.go`, emit `min`, `max` and `step` in `renderNumber`, and add the matching server-side check where the validator already reads the schema. Add the three keys to the designer catalogue for the `number` widget. The Kitchen demo makes no claim about number bounds until this is done.

### T-111. query-picker formengine widget: choose a record from an OQL query with typed columns -- built and verified live; closes with the release, after the browser script is added as a guard

Theme: formengine · Priority: P2 · Status: ◐ · Blocks/after: Shipped in 0.4.5; the browser script as a repository guard; a first production form

Trigger: instruction 2026-11-17 -- a form engine widget like any-picker whose options come from an OQL query that may return several columns of different types, with a dropdown that looks like a list.

Design agreed in outline and written up as docs/QUERY_PICKER.md (the mini manual; it is the specification until the widget exists). Mock-ups of the table and rows dropdowns, light and dark, were reviewed in the same session. Nothing is built.

Scope, in order: a shared read-only OQL guard (moved out of internal/accounting/formula_oql.go, not copied); the `query-picker` widget and its schema keys in internal/formengine; the search endpoint, which takes the form key, the field name and the search text and never a query; lookup columns with a per-tenant cache; the two dropdown layouts in app.js and app.css; the Selector dialog as the Browse target; the designer and widget catalogue entries; i18n keys in the four locales; tests, including one that runs the generated query against a real xolu. Then a Demo tab.

Progress 2026-11-17: built, everything in the scope above. New packages internal/oqlguard (the checks accounting's OQL formulas already had, moved there; accounting now uses them from it) and internal/querypicker (rules, query builders, sealing, row formatting); internal/formengine/query_picker.go; internal/ui/picker_query.go with GET /picker/query and GET /picker/query/browse; the dropdown as the "query-picker" kind of SeamWidgets; a "Query picker" Demo tab; 21 keys in each of the four locales; the designer catalogue entry and seven properties. docs/QUERY_PICKER.md is rewritten to describe what was built.

Departures from the first write-up, each recorded in the manual: (1) schemas have no global registry (many are built inline by handlers), so a request cannot name "form and field"; the field's specification reaches the page as a token encrypted and authenticated with a server-held key, and comes back with each search. (2) The footer says more rows match but gives no total: a count would be a second query per keystroke. (3) A stored value's label is fetched by the widget when the form opens, as any-picker does, not rendered by the server. (4) x-seam-selector takes the text "query" (the key is a string for any-picker). (5) Business entities are not filtered by tenant: the xolu client is tenant-scoped and assets has no tenant_id. (6) Dates and amounts are not formatted for the locale.

Verified: go build, go vet and go test ./... (36 packages); the query-picker tests under the race detector; the rule, cache and minimum-length tests fail when those are disabled. Live: xolu 0.30.44 seeded with the Constructec seed, Seam with the Demo module on, headless Chrome, 53 checks, light and dark, no page error and no server error.

Open before closing: (a) the browser script is ad hoc and not in the repository: add it under scripts/ and register it as a dormant guard; (b) no production form uses the widget yet, and a handler that saves one must call QueryPickerErrors; (c) FORM_ENGINE_SPEC.md's widget tables do not list query-picker yet; (d) not measured under concurrent load.

## What was built

The real "asset-picker" widget, replacing SPEC §8.3's MVP number-input
stopgap. Generic over entity via x-seam-source's existing "entity:
<name>" convention, two new schema attributes (x-seam-search-fields,
x-seam-display-fields), matching the architecture the tech team itself
resolved in an earlier session -- not invented fresh here.

Backend: a live search endpoint (GET /picker/search), deliberately
NOT trusting client-supplied field names for query construction --
a server-side, hardcoded config maps entity:<name> to its real OQL
shape, mirroring asset_source_resolver.go's existing
assetSourceQueries pattern, following T-63's lesson from earlier this
session directly. Two modes: live search (q=) and exact-ID lookup
(id=, resolving an existing value's label client-side on page load
without needing new render-time server plumbing).

Widget: autoComplete.js (already vendored, T-19's own earlier work),
current value's label resolved entirely client-side via fetch, never
interpolated into the widget's own inline script -- the T-63/T-64
lesson applied to a fourth mi.Raw site, this time from the start
rather than found and fixed after the fact.

Used for sensor-asset binding, the second half of the ask: found the
"Bind"/"Unbind" buttons already existed in SensorDetailPage's
boundAssetCard, completely non-functional -- no onclick, no form, no
href, a real confirmed dead-button scenario predating this session.
Built the actual feature: internal/formengine/schemas/sensors/
(bind.json, one field, the asset-picker widget -- proving it's
genuinely generic, not accidentally asset-specific, the first real
consumer beyond parent_id), three handlers (SensorBindForm/SensorBind/
SensorUnbind), routes, and the two dead buttons now do real things.

## Scope boundaries, deliberate

- sensor_bindings' create flow omits binding_metadata (role/protocol
  -- real fields the seed scripts populate, but enrichment data, not
  core to the binding itself). Noted, not silently dropped.
- The search endpoint's exclude= parameter (self-reference prevention
  for parent_id) is built and tested server-side but NOT wired into
  the widget's own JS -- a naive always-exclude would be actively
  wrong for a cross-entity picker (a Sensor's own ID excluded from an
  Asset search would hide an unrelated asset sharing that numeric ID).
  Matches T-19's own tracked note that cycle/self-reference handling
  remains undecided -- left unwired rather than rushed into a subtler
  bug. Documented directly in renderAssetPicker's own comment.

## Testing and verification

8 new unit tests (3 widget-level, 5 endpoint-level) -- including
adversarial ones guarding the exact T-63/T-64 lesson (OQL injection
via search terms, script-injection via the current value, JSON-
breakout via a malicious label). Both the widget's escaping and the
endpoint's OQL escaping verified with the full discipline: reverted,
confirmed genuine failures (one diagnostic showed a literal working
SQL injection), restored.

Live-verified end to end, twice over: the asset-picker on a real
asset's parent_id (search, select, submit, durable persistence
confirmed), and the full sensor bind/unbind cycle (bind via the real
form, confirmed durable in sensor_bindings, unbind via the real form,
confirmed removed). Zero server errors throughout either flow.

## A real methodology lesson from this session, worth recording honestly

Mid-session, a live test appeared to show parent_id silently failing
to persist despite every step of the pipeline looking correct --
spent real effort tracing it (debug logging, isolating each stage,
testing xolu directly). It was a false alarm: my own verification
query checked a column named "parent_id", which doesn't exist as a
raw xolu column -- the real, translated field name is "id_padre"
(assetFieldToXolu's own mapping, confirmed correct throughout). The
actual save path worked correctly the entire time; only my own
ad-hoc verification query was wrong. Worth recording plainly rather
than glossing over -- a reminder that live verification is only as
trustworthy as the verification query itself, and an English/Spanish
field-name mapping is exactly the kind of detail easy to get right in
the application code and wrong in a hand-typed debugging query.

## validation

### T-28. Custodio's tipo_sujeto/subject_type field only accepts DNI/CI/RUT/RUC (Uruguayan-specific tax/identity document codes) -- no way for a non-Uruguayan customer to represent their own tax identifier

Theme: validation · Priority: P3 · Status: ☐ · Blocks/after: English-market seed/customer work, if that ever becomes real -- not blocking anything currently in flight; discovered exploring a hypothetical, not requested production work

Trigger: 2026-07-31, testing what running Seam in English with English-market seed data actually looks like (the tech team's direct question, no English seed existed so one was built for this: seeds/seed-en-fairborne-minimal.py, a small Pacific-Northwest construction-equipment company mirroring the earlier-proposed Fairborne Equipment LLC). Already flagged as a debated edge case during T-26 (CustodioCreateSchema's field-name/enum-value rename) -- deliberately left the four DNI/CI/RUT/RUC values untranslated at the time, reasoning they're legal document-type codes, not translatable prose. This item is the concrete confirmation that decision has a real limit: the codes aren't just untranslated, they're regionally exhaustive -- Enum("DNI","CI","RUT","RUC") has no member that fits a US EIN, an EU VAT number, or any other jurisdiction's tax identifier at all.

Confirmed live, not assumed: created a real Custodian creation attempt through the actual UI form for a fictional Seattle-based company, submitting tipo_sujeto=EIN directly (the dropdown itself only ever offers the four Uruguayan codes, no way to even attempt this through the rendered UI). Server-side validation correctly rejected it: "must be one of: DNI, CI, RUT, RUC" -- confirming this isn't just a missing option in one dropdown, it's enforced at the schema layer (CustodioCreateSchema, internal/validation/schemas.go) and would need a real design decision, not a quick fix.

Scope, not decided here: this needs an actual design conversation, not a guess -- options include (a) adding more jurisdiction-specific codes to the enum as they come up (doesn't scale, and duplicates work already solved elsewhere for tax-ID formats), (b) a generic "other/international" catch-all value with the actual identifier type captured as free text elsewhere, (c) splitting subject_type (individual vs. company, genuinely useful business data) from the jurisdiction-specific document/tax-ID scheme (a separate concern that varies per country and probably shouldn't be a fixed enum at all). Not scoped further -- this item exists so the finding isn't lost, not as a commitment to any of these shapes.


## Part 1: OQL injection, six handlers, fifteen injection points

Every list-with-filters handler in internal/ui/routes.go built its OQL
WHERE clause by directly string-concatenating URL query parameters,
completely unescaped:

    query += " AND status = '" + statusFilter + "'"

with statusFilter read straight from r.URL.Query().Get("status"). A
single quote in any filter value breaks out of the intended string
literal into the WHERE clause itself -- reachable by constructing a
URL, no special privilege needed beyond viewing the page at all.
internal/auth already had an escapeOQL helper for its own OQL
construction (store_xolu.go); internal/ui simply never had an
equivalent, despite building OQL queries from user input in:

- AssetsList: search (LIKE clause), status, type_id (unquoted numeric),
  criticidad -- 4 points
- EventsList: asset_id (unquoted numeric), trigger_type, from/to dates
  -- 4 points
- SensorsList: type, status -- 2 points
- AlertsList: status, severity -- 2 points
- MapView: status, type_name -- 2 points
- AdminUsersList: role -- 1 point

Two (AssetsList's type_id, EventsList's asset_id) are unquoted numeric
comparisons -- escaping quotes does nothing there, since there's no
quote to escape out of in the first place.

Fixed: added escapeOQL (routes.go, same quote-doubling logic as
internal/auth's own -- duplicated rather than cross-package-imported
for something this small) and validOQLIntFilter (a strconv.Atoi check
for the two unquoted numeric cases) and applied one or the other to
every one of the 15 points.

## Part 2: stored XSS via asset name, on the Map page specifically

Layered on top of (1)'s injection risk on the same page: markersJSON
was hand-assembled via fmt.Sprintf with %s (no escaping at all, not
even json.Marshal), then embedded into a <script> tag, then the
client-side JS concatenated m.name/m.type directly into an HTML string
passed to Leaflet's bindPopup() (which renders its string argument as
HTML). m.name is a real asset's name -- genuinely user-editable via
the asset create/edit forms by any user with assets.asset.create/edit
(admin/manager/operator roles). An asset named
"<img src=x onerror=alert(document.cookie)>" would execute as live,
running HTML in the popup of every user who opens the map page --
classic stored XSS, session-hijacking-capable, with no special access
beyond editing one asset's name field.

Two independent layers needed fixing, not one:
- markersJSON now marshaled via encoding/json (MapMarker gained json
  tags matching the JS's existing field names) -- json.Marshal HTML-
  escapes </>/& by default, specifically for safe <script> embedding.
- Confirmed this alone is NOT sufficient: JSON.parse reverses that
  escaping client-side (\u003c becomes a literal "<" again once parsed
  into a JS string), so bindPopup()'s raw-HTML-string construction
  would still be exploitable even with (1)'s escaping in place. Fixed
  by rebuilding the popup via DOM APIs (createElement/textContent)
  instead of an HTML string at all -- the dynamic parts never pass
  through anything that gets interpreted as markup, escaped or not.
- A third, smaller bug found and fixed along the way: json.Marshal(nil
  slice) produces the literal "null", not "[]" -- the original fallback
  logic only guarded a marshal error, not the everyday "no assets have
  coordinates yet" case, which would have thrown a client-side
  TypeError on markers.forEach.

## Testing

internal/ui/oql_injection_test.go: one test per handler, each
submitting `x' OR '1'='1` (SQL-injection-shaped) via the relevant URL
query parameters and asserting the captured OQL query sent to xolu
never contains the raw payload. A dedicated test for the two unquoted-
numeric cases submits `1 OR 1=1` and confirms no injected OR clause
appears. An inverse test confirms ordinary, non-malicious filter values
still produce clean, correct queries -- the fix must not break the
happy path.

internal/ui/vendor_html_test.go: a script-tag-breakout payload in an
asset name, confirming the JSON-escaped form appears (not the raw
form, and not marker-silently-dropped); confirming the vulnerable
bindPopup(string) pattern is genuinely gone from rendered JS, replaced
by the DOM-construction pattern; confirming the empty-markers case
renders "[]" not "null".

Every fix in both parts verified with the full discipline: reverted,
confirmed the test failed (the AssetsList sanity check's diagnostic
output literally showed "WHERE type_id = 1 OR 1=1" -- a working
injection, executing), restored.

Full build/vet/test clean throughout, zero regressions.


## documentation

### T-66. ROADMAP.md needs a full content regeneration against current reality -- header/staleness notice fixed 2026-08-04, but ~200 lines of planning content still describe May 2026's state, not v0.3.55's

Theme: documentation · Priority: P3 · Status: ☐ · Blocks/after: -

Trigger: reviewer feedback, 2026-08-04, on the T-02 interdependency
pass -- flagged that ROADMAP.md still says "Last updated: May 2026,
Current version: 0.1.0-patched243" while the project is actually at
v0.3.55, and noted this is the same specific documentation-staleness
risk already flagged and fixed once in xoluman's own README, not yet
applied to the document a new team is actually likely to read first.

Confirmed directly: the header claim is real, and the staleness goes
well beyond the header. "Near-term -- Implementation" describes five
sub-plans (form engine, Documents module, i18n, Admin module, module
architecture) as numbered step lists still to be done -- most steps
across all five have since been substantially completed, several in a
materially different shape than planned (the "Admin module" section
describes one combined module; what shipped, T-02, is three separate
modules -- Account, Settings, Administration).

Scope: NOT attempted as an immediate fix alongside the header/notice
correction (done separately, same session) -- an accurate regeneration
means reconciling all ~200 lines of "Completed"/"Near-term"/"Technical
debt"/"Future" content against the real current state: ~65 resolved
tracking items (RESOLVED.md), the full CHANGELOG.md release history,
and whatever open work in TRACKING.md genuinely still matches this
doc's own "Future" section's intent. Doing this quickly risks
introducing fresh inaccuracies of exactly the kind being fixed --
worse than the current honestly-labelled staleness notice.

Not scoped or estimated further here. A real editorial/planning task,
not a coding one -- likely needs the tech team's own judgment on which
"Future" items are still genuinely future vs already reprioritised
away, not just a mechanical "what shipped" reconciliation.


## What was built

- xolu client extended (local sibling checkout): CreateMachineDef, the
  one missing piece needed to create FSM definitions through the
  library rather than raw HTTP. New tests, full xolu suite clean
  afterward. This is the change T-69's letter to the xolu team asks
  them to formally adopt upstream.
- EnableWorkOrders feature flag (internal/config.FeatureConfig),
  default false -- deliberately opt-in, wired through to
  internal/ui.Config and the module registry's HasAccess. A real bug
  caught before shipping: the first version of HasAccess returned true
  unconditionally, which would have made the module visible in the nav
  even when disabled -- exactly the "visible but unmountable" state
  this session's own adversarial registry tests documented as allowed
  by the type system, caught here as an actual near-miss rather than
  only a test case. Fixed to check the flag directly.
  SEAM_ENABLE_WORK_ORDERS env var added alongside the existing
  SEAM_ENABLE_ADMIN/SEAM_ENABLE_OPERATIONAL pattern.
- internal/formengine/schemas/workorders/ (create.json, complete.json
  -- the latter using the signature widget).
- internal/ui/workorders.go: an idempotent FSM-definition bootstrap
  helper (lists definitions, creates only if missing -- definitions are
  immutable, so no update path needed), the WorkOrder entity mapping,
  and all six handlers (List/New/Create/Detail/Start/Complete) plus
  their page-rendering functions. Deliberately simple, direct markup
  (not the fuller ListRow/ListCellRich/ListActions framework
  AdminUsersList uses) -- a playground the team will read and extend,
  not a polished production list view.
- internal/modules/workorders, registered as a fifth "apps"-group
  module in server.go, following the exact pattern established for
  Account/Settings/Administration (T-02) and Assets/Documents.
- 18 new i18n keys x 4 locales.
- A real scope decision made and recorded, not silently skipped: no
  audit logging for Work Order actions in this iteration. Adding new
  AuditEventType constants to internal/auth's shared enum for a
  playground feature would be scope creep ahead of T-11's own real
  event-taxonomy design.

## Testing and verification

Unit tests: xolu client (CreateMachineDef, 5 tests), the workorders
module's delegation + disabled-flag tests, and 5 handler-level tests
in internal/ui/workorders_test.go covering the bootstrap helper's
create/idempotent paths, the create flow's machine-then-entity
sequencing (embedding machine_id and initial state directly, not a
create-then-patch round trip), a guard-rejected completion attempt
re-rendering with a visible error rather than a raw 500, and a
successful completion correctly denormalizing both state and signature
onto the entity. The machine_id-embedding assertion verified with the
full discipline: reverted, confirmed the test failed, restored.

Live-verified end to end through the real running application, not
just unit tests: created a real work order through the real form ->
FSM definition auto-bootstrapped on first use (confirmed via a fresh
xolu query) -> machine created and linked (machine_id present on the
entity) -> Start -> detail page correctly shows "In Progress" with the
signature canvas -> attempted Complete with an empty signature ->
correctly rejected, error shown on the page, no 500 -> completed with
a real signature -> final state "completed", signature image rendered,
confirmed durable via a fresh xolu query in a separate request -> list
page correctly shows the completed work order. Zero server errors
throughout the entire flow.

Full seam-ui build/vet/test clean throughout (24 packages, including
the new internal/modules/workorders).


## navigation

## testing

### T-86. internal/ui/workflow/ (six end-to-end, cross-handler workflow tests against a real embedded xolu) needs effort estimation for a full repair -- currently builds again (T-85 fixed its stale replace-directive path) but go vet surfaces real API staleness (removed config field, changed constructor signature, renamed handler method) unrelated to Spanish identifiers; scoping/estimating the fix is the next step, not performed here

Theme: testing · Priority: P3 · Status: ☐ · Blocks/after: -

Trigger: split out of T-85's own discovery, at the tech team's direction --
"file an issue for the unearthed workflow package, with a focus on
effort estimation for bringing it up to standard. add a note
explaining what it used to be good for and why we may want it back."
This item is that issue; no repair or estimation is performed here,
deliberately -- both are scoped as the next step, not done as part of
filing this.

**What it is.** internal/ui/workflow/ (module
github.com/ha1tch/seam-ui/workflow-tests, its own go.mod, xolu a
test-only dependency never imported by the main seam-ui module) spins
up a real, embedded xolu server (SQLite, in-process -- not mocks) and
drives seam-ui's actual UIHandler methods through multi-step
sequences. Six workflows, each create-then-verify-elsewhere:

1. Región -> Ubicación -> delete guard (a Region can't be deleted
   while a Location still references it)
2. Marca -> Modelo -> delete guard (same shape, Brand/Model)
3. PropDef -> PropAssoc -> delete guard (same shape)
4. Asset with expiry date appears on the real dashboard's
   urgency-list widget
5. Alert acknowledge reduces the open-alerts count shown elsewhere
6. An inactive Custodio is correctly excluded from the asset
   dropdown's own picker

**What it used to be good for, and why we may want it back.** Every
one of these six tests exercises a *cross-handler* consequence --
data created through one handler, a fact verified through a
completely different one. This is exactly the class of bug that
per-handler unit tests with mocked xolu responses structurally cannot
catch, since each side of the interaction is tested in isolation
there. Workflow 4 in particular (asset expiry -> dashboard) tests
precisely the code path T-85 found genuinely broken in production for
three releases (Block E's own rename silently breaking the assets
dashboard's urgency-list widget, discovered only by a deliberate
manual dark-spot audit, not by any automated check). Had this suite
been running, it would very likely have caught that regression the
same day it was introduced rather than three releases and one
deliberate audit later. That's the concrete case for its value: not
hypothetical coverage, but a real regression this exact suite is
shaped to catch, that it did not catch, because it wasn't running.

**Current state.** Was completely unbuildable at all -- its own
replace directive pointed github.com/ha1tch/xolu at /tmp/olu, a path
confirmed not to exist in this container. T-85 repointed this to the
session's actual xolu working copy; the module then builds and
resolves its own dependency graph cleanly (go mod tidy succeeded,
confirming pkg/client's core surface really has stayed stable across
the v0.9.7-patched74 -> v0.30.6 gap, consistent with every other
compatibility check this session). T-85 also fixed the one directly
Asset-rename-relevant stale fixture (fecha_expiracion/criticidad in
Workflow 4's own createDirect call).

Beyond that, `go vet` surfaces real staleness unrelated to Spanish
identifiers: config.Config no longer has a DBPath field the test
constructs; server.New's own signature no longer matches (parameter
count/types changed); UIHandler no longer has an AssetNew method
(renamed AssetNewDynamic when the dynamic-schema-driven asset form was
built). `go vet` stops at the first error it hits per compile unit, so
these three are confirmed real but not confirmed exhaustive -- there
may be more once the first three are cleared. Given only one file
(workflow_test.go, ~460 lines) and one package are involved, the
repair surface is at least bounded, even though its exact size isn't
yet known.

**Next step, explicitly not done here:** scope and estimate the
effort to bring this module fully current -- likely requires reading
xolu's pkg/config.Config and pkg/server.New as they exist today,
reading UIHandler's current asset-handler surface, updating all three
(and whatever `go vet` surfaces next, iteratively), then actually
running the six workflows against current code and confirming they
pass for real reasons, not just compile. Whether that's a small,
contained task or larger depends on how much the six workflows' own
setup/assertions have drifted beyond the constructor-level signature
changes already found -- unknown until someone reads them properly
against current handler behavior, which this item deliberately did
not do.

## config

### T-92. config.Server.MaxBodySize is parsed but never enforced -- dead config value

Theme: config · Priority: P3 · Status: ☐ · Blocks/after: -

config.Server.MaxBodySize (internal/config/config.go, parsed from
server.max_body_size in config.example.json, default 10MB) is never
actually enforced anywhere -- no http.MaxBytesReader or equivalent
call references it (confirmed by a whole-tree grep). It's a dead
config value: setting it in a deployment's config.json currently has
zero effect on request size limits.

Found 2026-08-13 while building the "image" formengine widget's
gallery-upload endpoint (internal/ui/media_gallery.go, T-10), which
needed its own upload size ceiling and discovered there was no
platform-wide one to lean on. Not fixed here -- wiring MaxBodySize to
actual enforcement is server-setup-level work (affects every route,
not just the gallery endpoint) and deserves its own consideration and
testing, out of scope for what T-10 asked for. The gallery endpoint's
own 20MB ceiling (maxGalleryUploadBytes) is correct and sufficient on
its own regardless of this gap -- not blocked on it, just noting the
platform-wide value doesn't currently back it up.

## qrcode

### T-93. internal/qrcode's image rendering is a placeholder; seam-2dcodes is a real candidate fix, pending its own auth/blob-verification gaps

Theme: qrcode · Priority: P3 · Status: ☐ · Blocks/after: -

internal/qrcode's actual QR image rendering is a placeholder, not a
real implementation -- generateQRBase64() (qrcode.go) returns
base64("QR:"+data), not a scannable QR image. Only that one function
is fake; the code-string generation/checksum/validation/scan-record/
label-metadata logic around it is genuinely implemented and unaffected.

Found 2026-08-17 while comparing seam-2dcodes (a separate PoC project,
uploaded this session) against the barcode widget's own JsBarcode
choice. seam-2dcodes is a working, independently-scanner-verified
renderer covering qr/datamatrix/aztec/pdf417 (2D) and
ean13/ean8/upca/code128/code39/code93/codabar/itf (1D), PNG+SVG, with
GS1-recommended OCR-B human-readable text on 1D symbologies (real
vector glyph outlines, not rasterised). At v0.2.0-poc, its own
register lists two open P2s before it's safe to expose: auth model
undecided (open to any caller reaching the port), and the xolu /blob
output path has never round-tripped against a real xolu server.

Two separate decisions, not one:

1. Package move: internal/qrcode -> pkg/qrcode. We confirmed no
   objection -- straightforward, not blocked on anything below.

2. Where 2D-code rendering should actually live, still open:
   (a) as-built: a standalone seam-2dcodes microservice (new binary,
       new port, new network dependency; clean single-purpose
       separation; but S2D-1/S2D-2 need closing first)
   (b) folded into xolu as a plain utility endpoint (not a /-prefixed
       primitive -- every existing xolu primitive is a persistence/
       coordination concept with a lifecycle; a renderer is a pure,
       stateless transform, categorically different). This kills
       S2D-1 and S2D-2 by construction (local blob write, inherited
       xolu auth) at the cost of xolu's dependency footprint gaining
       boombuler/barcode + x/image/font/sfnt + the OCR-B font asset,
       and xolu's release cadence becoming coupled to an unrelated
       rendering library's changes.
   (c) imported directly into seam-ui (or wherever needs it) as a
       plain Go library, once un-internal'd -- no new service, no
       xolu coupling, but no shared reuse across other consumers
       either.
   Not decided here -- the tech team's call given how central xolu is
   across everything else built on it.

Not blocking anything currently in flight: the barcode widget (T-10)
deliberately kept its already-built, already-tested JsBarcode choice
rather than switching -- simpler, zero new deployment/auth surface,
better live-as-you-type UX, and seam-2dcodes isn't safe to expose yet
regardless. This item is about internal/qrcode's real gap, a separate
piece of work from T-10.

## ui-components

### T-96. Generic Selector dialog: modal chooser for any entity, list configured like the List component (first use: Users)

Theme: ui-components · Priority: P2 · Status: ☐ · Blocks/after: Go toolchain in the sandbox; tenancy-model question for the users entity

Trigger: instruction 2026-10-07 -- a common dialog to choose from a list of Users, generalised into a configurable Selector that can choose any entity, its list configured the way the List component (the Assets list screen) is.

Not started (blocked on the Go toolchain, see HANDOVER_2026-10-07.md section 2). Grounded design, from reading the code: the List component is composable primitives (ListTable/ListRow/ListCell*/ListFilterBar/ListPaginationBar in internal/ui/listing.go), not a data-driven column spec, and the Users admin page (internal/ui/admin_users.go) is a ready example of a Users table. The Selector should reuse those primitives. Entities come from a SERVER-SIDE registry keyed by name (extend the pattern of pickerSearchConfigs in internal/ui/asset_picker_search.go); the client sends a registry key, never an entity or field name (T-63 injection history). Rendered into the existing SeamModal2 (web/static/js/app.js); selection returns to the opener by a DOM event. Users source must go through auth.UserService (never expose password-hash fields), be tenant-scoped, and default to active users.

Full design, open decisions with recommended defaults, file list and test plan: docs/HANDOVER_2026-10-07.md section 6. Depends on the tenancy-model question filed alongside this item.

## housekeeping

### T-99. Housekeeping after the gorepoman migration and Wallace removal: historical mentions and the doctor Go-version note remain (repoman/ removed and badcode clean, 2026-11-17)

Theme: housekeeping · Priority: P3 · Status: ◐ · Blocks/after: None -- decisions only

Trigger: loose ends from the 2026-10-07 session.

1) Done 2026-11-17, on instruction: the repoman/ directory (the gorepoman shims, repoman/VERSION and the dead config.py) was removed. syncver.sh now calls the gorepoman binary directly (REPOMAN_BIN, else repoman on PATH) and release.sh no longer ships repoman/. The root .repoman.json, which was missing from the checkpoint, was recreated with version_targets for the four tracked documents.
2) Historical "Wallace" mentions remain in CHANGELOG.md, docs/RESOLVED.md, docs/formengine/RESOLVED.md, docs/HANDOVER_2026-08-13.md and attic/ (frozen files). The FSM engine's header, attribution and licence were removed on instruction (clean-room rewrite), and the code/comment mentions in pages.go, routes.go, router.go, file-upload.js and the two FSM JS files were cleaned; decide whether the history files stay as they are.
3) `repoman badcode check .` now reports no hits (3 patterns), done 2026-11-17. The last four were a test fixture user name in internal/ui/issues_test.go, since renamed; the 229 earlier hits (docs, comments, seeds, two sandbox go.mod replace paths) were already cleared. A release through gorepoman relcore would no longer be blocked by it.
4) `repoman doctor` exits 1 because the v0.16.0 release binary was built with Go 1.25.14 while go.mod requires 1.26.5; effect on repoman's own Go-syntax awareness not determined. Re-checked 2026-11-17 with the same 0.16.0 binary: still exits 1; selftest passes 236/236 and gofmt-backed validation is enabled through the sandbox's own gofmt.

## developer-module

### T-118. Developer module: what is missing for daily use (new keys, new forms, export, untranslated list, translation history, designer structure)

Theme: developer-module · Priority: P2 · Status: ◐ · Blocks/after: -

Trigger: review of the Developer module on a live stack, 2026-11-17: what it lacks for daily use, apart from the defects filed as T-112 to T-117. Each point was observed, not inferred from the campaign document.

Works today, verified: the five runtime switches; a translation override (tenant or file) shows on real pages at once and disappears when its layer is switched off; a text that drops a %d marker is refused; hot loading picks up a file changed in an editor within about two seconds, for translations and forms; a broken translation file is refused, reported and the previous one kept; form history with restore and remove; live preview and a diff against the embedded form; view-as-role; "show translation keys"; another role gets 403; changes are audited.

Missing:
1. A new translation key cannot be added ("Ese texto no existe"): a developer building a feature cannot use the tool for its texts.
2. A new form cannot be created; only the catalogued ones can be overridden.
3. Nothing takes an override back to the source tree: no export or download of a tenant's overrides as the locale and schema files the repository uses.
4. Translation completeness is four counts per language; there is no way to list the 47 untranslated texts of `es`, and no filter by origin (overridden, built in).
5. Translation overrides have no history; form overrides do.
6. Changing a field's `title` in a form has no visible effect when a translation key exists for it, and the editor does not say so.
7. "Recargar ahora" is on the Translations and Forms pages, not on the overview, where the hot-loading switches are.
8. The form designer handles a flat list of fields: no tabs, rows, sections, details or embeds, no drag and drop (DEV-21's remaining scope).
9. No search across form definitions (which forms use a widget or a source).
10. The topbar search placeholder is English on a Spanish page (not specific to this module).

Order suggested: 1, 3, 4 first (they decide whether a team can do a feature end to end with the tool), then 2 and 8.

Progress 2026-11-17 (in the tree, unreleased), with the rework of the module's pages (DEC-30 of docs/DEVELOPER_MODULE_CAMPAIGN.md): point 4 is done (each count of the completeness table links to its texts, and the list has a "show" filter: overridden, not translated, missing, extra, different formats); point 7 is done (both directories are reloadable from the Overview and return to it); the forms list filters by module and by part of a name. Also seen then: the designer's property titles and help texts (`designer.prop.*`) have no `es`, `pt` or `ja` texts and show in English.

Progress 2026-11-17, second pass (in the tree, unreleased; DEC-31): point 1 is done (a text under a new key is created from the Translations page, in the reference locale, and is then translated with the editor; a translation of it keeps its placeholders); point 2 is done (a form under a new module/name is created from the Form definitions page and opens in the designer; it is marked "Not built in" and no page uses it until code asks for it); point 3 is done (the Overview and both lists offer "Export overrides": a zip in the override directories' layout and in the source tree's); point 5 is done (a tenant's override of a text keeps its versions and an older wording can be restored). Open: points 6, 8, 9 and 10, the `designer.prop.*` texts, and the remaining campaign items.

Progress 2026-11-17, third pass (in the tree, unreleased; DEC-32): point 6 is done (the form editor and the designer name the fields whose label is a translation key and link to the text); the `designer.widget.*` and `designer.prop.*` texts exist in the four locales. Also added: history and restore for file-scope overrides of texts and forms; version 1 for tenant texts overridden before the history; the state of a text in every language, in its editor; the line of code that renders a custom form; the definition shown read-only when there is no schemas directory; `make dev` turns on per-tenant overrides and the designer. Open: points 8, 9 and 10, and the remaining campaign items.

Progress 2026-11-17, fourth pass (in the tree, unreleased; DEC-33): point 10 is done (`layout.search.placeholder`). Also added: the directory gate; versions for changes made in the files; label keys taken from the page once it has been rendered; the try page; a new text in every language at once. Verified by tests and on a live stack (132 browser checks). Open: points 8 and 9, and the remaining campaign items.

Progress 2026-11-17, eighth pass (in the tree, unreleased; DEC-37): point 8 is done (the designer edits tabs, sections, rows of sections and the place of each field, by drag and drop or by buttons; details and embeds as JSON lists per tab); point 9 is done (the forms list matches what a definition uses: a widget, a source, a field). Also added: file-system events for the override directories on Linux (`developer_watch`), the audience of a module switched on at runtime (`developer_module_audience`), preferences kept with the account (`developer_preferences`), pages for custom forms (`developer_custom_pages`), the global scope in the export, the try page for a given record, the rule that accounts for a text, all 37 built-in forms in the identity table. Verified by tests and on a live stack (182 browser checks in four scripts). All ten points are done; the item closes with the next release. DEV-21 is done (DEC-39): blocks can be duplicated and the preview has a language select; the preview's role selector, the inspector rendered by the engine and controls for details and embeds were dropped (DEC-38, DEC-39).

Progress 2026-11-17, seventh pass (in the tree, unreleased; DEC-36): the campaign's open items except the designer's block tree are done (DEV-4, 7, 9, 19, 23, 24, 26, 27, 29): global scope, module switches, texts not referenced in the source, preferences, the interval settings, the seeds' role accounts and the cross-references. Open: point 8 (DEV-21's block tree) and point 9.

Progress 2026-11-17, sixth pass (in the tree, unreleased; DEC-35): the translations directory loads file by file; the page identities ship with the build (30 forms) and a form can have several; the directories are looked at every 250 ms for the history; try lists leave out rows switched off. Verified by tests and on a live stack (141 browser checks). Open: points 8 and 9, and the remaining campaign items.

Progress 2026-11-17, fifth pass (in the tree, unreleased; DEC-34): the schemas directory loads file by file (a malformed file no longer stops the others); changes made in the files are recorded while hot loading is off; the page identities for label keys are kept across restarts; the try page fills the lists of any entity source; the export includes the package of a new form's module and the steps left. Verified by tests and on a live stack (137 browser checks); the generic list read was seen only with empty entities there. Open: points 8 and 9, and the remaining campaign items.

## ui

### T-119. Demo tabs in dark mode: the tab open at load keeps looking open after another is chosen

Theme: ui · Priority: P3 · Status: ☐ · Blocks/after: -

Trigger: rework of the Developer module's pages, 2026-11-17, when its tab strip was built on the same script (`SeamTabs.wire`, web/static/js/app.js).

`SeamTabs` marks the open tab by swapping four classes (`border-indigo-600 text-indigo-600` against `border-transparent text-gray-500`). The Demo module's tab buttons (internal/ui/demo.go: `demoTabActive`, `demoTabInactive`) also carry `dark:` variants (`dark:text-indigo-400 dark:border-indigo-400`, `dark:text-gray-400 dark:hover:text-gray-300`) that the swap never touches. In dark mode they win, so after the first click the tab that was open at load still looks open and the one just opened looks closed. Read from the code and from the classes the buttons end up with (seen in Chromium on the Developer module's strip before its own fix); the Demo page itself was not opened in dark mode for this.

The Developer module's strip avoids it by keeping only the swapped classes on its buttons and putting the dark colours in app.css (`.dev-tab-btn`). The form engine's own tabs and the Asset form use other class sets and were not checked.

Scope: either make `SeamTabs.wire` swap the dark variants too, or give the Demo's buttons the same treatment; check `fe-tab-btn` and the Asset form's tabs in dark mode while there.

## security

### T-125. Issues has no permissions of its own: a read-only viewer and the Accounting roles can act on issues

Theme: security · Priority: P1 · Status: ☐ · Blocks/after: -

Trigger: 2026-11-18, the decision to hold the Issues module to what the other application modules meet (docs/ISSUES_MODULE.md).

Found: the module is registered with HasAccess true for every role, and its issue routes ask for a signed-in user only. Who may act is decided in the handlers by role rank and ownership (canActOnIssue, canAssignIssues, canManageIssueGroups). Checked on a running instance over seed 01 with the Issues and accounting seeds: viewer@example.com (role viewer, read-only everywhere else) was offered resolve and reject on an issue nobody is responsible for, and its reject form answered 200; marta.rivas@example.com (role account_executive, a role of the Accounting module) started that issue. Both see the module in the navigation and can report. Templates and the list of QR codes borrow settings.edit.

Scope: an issues.* namespace in internal/auth (view, report, act, assign, group, template.manage, qr.manage, or what the design settles on), granted to roles; requirePerm on the routes; HasAccess as HasModuleAccess(role, "issues"); the ownership rule stays in the handlers on top of it. Decide the grant of each role, in particular whether a viewer reports and whether the Accounting roles see Issues at all. The Issue Tracking team's repository carries this code too: agree the change with them so it merges once.

## issues

### T-126. Issues: hold, resume and merge are in the lifecycle and no screen sends them

Theme: issues · Priority: P2 · Status: ☐ · Blocks/after: -

Trigger: 2026-11-18, docs/ISSUES_MODULE.md.

Found: the definition seam_issue_lifecycle has the inputs hold (to on_hold), resume (back to in_progress) and merge (to duplicate). MountIssuesRoutes has routes for start, resolve, close and reject only, so on_hold and duplicate cannot be reached from the application; the list's filters offer both states. Each Issues seed has one issue put on hold by the loader (ISS-06): from the application it can be rejected and not resumed.

Scope: either add the three actions (hold with a reason, resume, merge into another issue, with what a case does to its grouped issues) or take the two states out of the definition and the filters. If they go, ISS-06 changes state in the six seed files.

### T-127. Issues writes nothing to the audit log

Theme: issues · Priority: P2 · Status: ☐ · Blocks/after: -

Trigger: 2026-11-18, docs/ISSUES_MODULE.md.

Found: no file of the module calls the audit log; assets, settings and the Developer module do. The steps of an issue are in the history of its state machine. A change of responsible, grouping and ungrouping, creating, editing, assigning or deleting a template, and revoking a QR code are recorded nowhere.

Scope: audit entries for those actions and for the lifecycle steps, with the actor, in the form the other modules use.

### T-130. Issues has no dashboard and no REST API

Theme: issues · Priority: P3 · Status: ☐ · Blocks/after: -

Trigger: 2026-11-18, docs/ISSUES_MODULE.md.

Found: Assets and Documents register a dashboard (DashboardID) and assets are served under /api/v1; Issues has the list with filters and nothing else. molu and the MCP Hub reach Seam's data through xolu, so this is about Seam's own surface.

Scope: decide whether Issues gets a dashboard (open by state, deadlines missed, by asset) and API routes; build what is decided.

## accounting

### T-132. Accounting has the start-time flag but no switch for one tenant, unlike Work Orders and Issues

Theme: accounting · Priority: P3 · Status: ☐

Trigger: 2026-11-18, bringing Issues to the module lifecycle of Work Orders (T-131) and putting the three opt-in application modules in one list (scripts/lib/modules.sh).

Found: Accounting has the start-time flag (enable_accounting, SEAM_ENABLE_ACCOUNTING, off by default) and its registry entry asks it, but it has no switch for one tenant: there is no module_accounting in internal/features/catalog.go, its routes are mounted or not at startup, and the Developer panel does not list it. Work Orders and Issues have both halves. Its flag also carries no class, where the other two are class preview.

Scope: decide whether Accounting gets the runtime switch. It differs from the other two: it holds the books of the company that sells Seam, with tenants as its subject, so "on for one tenant" may not mean anything for it. Either add module_accounting with moduleGate, HasAccessFor and the panel line, as for Issues, or record in docs/KNOWN_ISSUES.md why it has the start-time flag alone. Give the flag its class either way.

Note (2026-11-18): Accounting can now run as the whole system, without the Assets module (make launch-accounting, make demo-accounting; enable_operational off). That is a start-time arrangement and does not change this item.

