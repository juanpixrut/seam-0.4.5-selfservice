# Developer Module Campaign

Started: 2026-10-10
Base: `seam-v0.4.1-pre2-checkpoint` (VERSION 0.4.1-pre2)
Current: VERSION 0.4.1-pre-devtools1 (2026-10-11)
Merged: 2026-10-12, onto the line that carries the visual-controls work (0.4.1-pre8 and
later); see the CHANGELOG, [Unreleased]. The merged tree has not been run against a browser or
a live xolu.
Status of this document: living tracker. Items are referred to by number
(`DEV-n`, `DEC-n`, `Q-n`). Numbers are never reused or renumbered; a dropped
item is marked dropped, not deleted.

Status markers: ☐ not started · ◐ in progress · ☑ done · ✗ dropped

---

## 1. Purpose and scope

Add a **Developer** module for development teams, enabled by an environment
variable in the same way the Demo module is enabled by `SEAM_ENABLE_DEMO`.
It provides:

1. Editing, syncing and hot loading of i18n language files.
2. Editing and hot loading of form definitions (JSON Schemas).
3. A form designer.
4. A control panel for runtime and feature flags, including the hot-loading
   switch.
5. Later: a "view as role" facility for testing under other roles.

Related specifications: `docs/I18N_SPEC.md` (§5.2 `Reload`, §5.3 xolu
structure), `docs/ADMIN_MODULE_SPEC.md` (§8 Form Designer, §15.5 schema
lifecycle policy), `ROADMAP.md` (i18n steps 4-5, Admin steps 3-5).

---

## 2. Status snapshot (2026-10-10)

* **Nothing in this campaign has been written yet.** Study only.
* **The tree has no verified baseline.** The assistant sandbox cannot build
  Seam: `go.mod` requires Go 1.26.5 (the Ubuntu LTS toolchain is 1.22.2), the
  Go module proxy is unreachable from the sandbox, and `go.mod` carries a
  `replace` directive for xolu pointing at an absolute path on one machine, which
  does not exist there. See DEV-1.
* The 2026-10-07 handover records T-94, T-95 and T-96 as never compiled. The
  Demo module showcases T-96's Selector, so it inherits that uncertainty.
* `STATUS_2026-10-07_session3.md` has not been read; the latest state of those
  items may differ.

---

## 3. Decisions

| ID | Decision | Notes |
|----|----------|-------|
| DEC-1 | **Developer is a role**: an admin who additionally has access to the Developer module. | Implemented as admin's permissions plus `developer.access`. Proposed rank 5, above admin (verify, Q-1). |
| DEC-2 | The module is enabled by `SEAM_ENABLE_DEVELOPER`. | Follows the existing `SEAM_ENABLE_*` convention. The existing Demo variable is `SEAM_ENABLE_DEMO`. |
| DEC-3 | **Both filesystem-based and xolu-based overrides** are supported, for i18n files and for form definitions/schemas. | |
| DEC-4 | Layer order, highest first: filesystem, tenant xolu, global xolu, embedded. | Assumed, not yet explicitly confirmed. The panel must show the effective layer and mark shadowed entries. Revisit if xolu should outrank the filesystem. |
| DEC-5 | i18n overrides apply **per key** (merged over the base). Form schemas are replaced **whole**. | "Form definition" and "schema" are treated as one document, since layout metadata lives in `x-seam-form` inside the schema. |
| DEC-6 | Hot loading and other runtime developer settings are changed **only from the Developer control panel**, and persist in xolu. They survive logout and login and apply whichever role is testing. | State is stored per tenant, not per session or per user. |
| DEC-7 | Flags fall into two classes: **startup-only** (shown read-only in the panel with their source) and **runtime** (editable in the panel). An env or config pin locks a runtime flag. | Resolution order: pin, then persisted panel value, then default. |
| DEC-8 | Tenancy is supported from the first version. One xolu instance may hold several tenants; some deployments are single-user instances (to avoid noisy-neighbour problems). Code must be correct under both. | Answer to T-98. |
| DEC-9 | **First version edits tenant scope only.** Global-scope editing is deferred (DEV-26). | |
| DEC-10 | **View-as role is built after the control panel exists.** Until then, test under other roles with separate test accounts in a second browser session. | The persisted per-tenant state means both sessions see the same overrides. |
| DEC-11 | Developer-class features refuse to start when auth is disabled, and log a loud warning like the existing DevMode one. | |
| DEC-12 | **One page per section, each with its own left-menu entry**: Overview (the runtime switches), Translations, Form definitions, Startup configuration. The Overview is served at `/developer` and `/developer/overview`, and renders with the latter as its active path. | The sidebar marks an item active by URL prefix, so an item at `/developer` would stay lit on every other page. `TestDeveloperSidebar_PointsAtRoutesThatExist` fails if two items are prefixes of one another or an item has no route. |
| DEC-13 | **Every input form of the tools is a form-engine schema** (`internal/formengine/schemas/developer`: `translation_filter`, `translation_edit`, `schema_edit`), rendered by the engine and resolved through `resolveSchema`, so the tools can be edited with themselves. Hand-built minty is kept for what is not a form field: status cards, the segmented Enable / Disable switch, tables, links. | The engine prefers a translation key to a schema's literal title, so a label is changed in the locale files, not in an override's `title`. Fields the editors hide (locale, key, schema) are posted as hidden inputs by the wrapper form, not by the engine. |
| DEC-14 | **The editors write only through the validated, atomic writers** (`i18n.FileLayer.SetOverride`/`RemoveOverride`, `schemaoverlay.FileLayer.Save`/`Remove`): shapes and sizes checked, written to a temporary file and renamed, then reloaded. A file that is not valid JSON is never overwritten. Only texts that exist in the reference locale (or in the locale itself) and forms in the catalogue can be edited, so a name can never be a path or invent a key. | A text's printf verbs must match the original's **in the same order**, stricter than the completeness report (which compares them as a set), because `Sprintf` consumes its arguments in order. A browser's CRLF newlines are normalised before validation. Failures carry sentinel errors (`ErrInvalid`, `ErrVerbs`, `ErrInvalidName`, `ErrRejected`) so the page can say which. |
| DEC-15 | **Reloads after a write run under the write lock; loaders ignore dot-files and dot-directories and treat a file that vanishes mid-scan as absent.** | Found by the concurrent-writer tests: a reload meeting another writer's temporary file failed with "not a valid locale file name", which the panel would have shown as a spurious load error. |
| DEC-16 | **A form-definition override replaces the whole document on the production pages** (DEC-5) and is not applied to the Demo's examples, which are fixed. Asset update validation still parses the embedded `SchemaForUpdate`. | An override that adds fields to the asset form is rendered but not validated on update. The catalogue offers no demo forms, so the browser never offers an override that would do nothing. |
| DEC-17 | **The editors edit the filesystem layer only**, which is global (DEC-9 stands): a saved text or form changes for every tenant. The xolu layers (DEV-13, DEV-16) and tenant-scope editing come later. | The pages say so, and the editors only exist when `SEAM_LOCALES_DIR` / `SEAM_SCHEMAS_DIR` is set. |
| DEC-18 | **The layout chrome learns the viewer's tenant from the request**: `PageHandler.viewerConfig` copies it into `Config.ViewerTenantID` (render-time only, never configuration), and `chromeTranslate` is the only way the layout reaches the translator. | Without it the navigation ignored the tenant's switches while the page body followed them (DEV-31). `TestLayoutChrome_UsesTheTenantAwareTranslator` fails if a new call site bypasses the helper. The error page has no request and keeps the global lookup. |
| DEC-19 | **A tenant's page says so while a switch that changes what its users see is on**: show translation keys, or a filesystem or database layer switched off. Every role sees the notice; only a developer is offered the way to the switch. The hot-load switches are not announced, and with nothing active the layout gains no markup at all. | Answers DEV-10. The notice reads the same persisted per-tenant state as the panel (DEC-6), so it cannot disagree with it. |
| DEC-20 | **Tenant-scope overrides live in two xolu entities and are applied to the xolu-tenant layers**: `i18n_override` (one row per tenant, locale and key) and `schema_override` (append-only: one row per version of a tenant's form). `internal/overrides.Service` validates, stores, then rebuilds that tenant's layer from the store, so what is live is always what is stored; at start it loads every tenant's. | The layers already existed (DEC-4). Append-only gives history and rollback for free: a removal is a version, a restore appends a copy of an old one, nothing is rewritten. The Store interface has a memory and a xolu implementation tested by one conformance suite, including hostile names. |
| DEC-21 | **File and database overrides share their validation**: `i18n.CheckOverride` and `schemaoverlay.Resolver.CheckDocument` are the one definition of what an override may be (shapes, size, control characters, placeholders in order, form accepted by the engine). | They were extracted from the file writers, so the two kinds cannot drift. |
| DEC-22 | **The tenant of an override is always the session's.** Every store method refuses a tenant that is not positive; a hostile `tenant` or `tenant_id` request field is ignored (tested end to end), and one tenant never sees another's history. | |
| DEC-23 | **The editors offer the scopes as tabs, tenant first**: "This tenant" (database) and "All tenants" (files), only those configured. A post with no scope means the first available (existing callers work), an unknown one is refused. A file override outranks a tenant's (DEC-4) and each editor says so in a sentence, in both directions. A note belongs to a tenant version; Restore adds a version that says where it came from. | |
| DEC-24 | **Known limits, recorded rather than hidden**: no cross-instance reload (a second instance sees another's change at its next restart; DEV-28); version numbers are highest-plus-one, which two instances saving at the same instant could duplicate (the history sorts by version then time); whether xolu needs the two entities declared before first use is unverified (T-106), so a failure to read them at start is logged, not fatal. | |
| DEC-25 | **View-as** (DEV-22): the role a developer views the application as rides in the signed session (`va`), is set only for a developer session and only to another valid role (`auth.CanViewAs`), and is ignored entirely unless the Developer module is on. `Router.contextFor` rewrites the request's claims to that role, so every permission check and module gate sees the lower role unchanged; the real role sits under its own context key. The Developer routes check the real role (`requireRealPerm`), the Developer module is kept in the selector, and a notice on every page names the role and links to Exit. User, tenant and account stay the developer's. Exit is a GET, like Logout, and harmless as one: it can only give the caller their own role back. | Choosing the session's own role (developer) is leaving, not a view. Changing the role does not extend the session. Set and exit are audited with the real role; other audited actions of a viewing developer record their user ID and the effective role, not both. |
| DEC-26 | **The role selects offer the developer role only to a caller who may assign it** (DEV-30): one byte-level insertion into the schema text (`withEnumValue`), because decoding and encoding would reorder the keys. The rule that kept a developer's role is replaced by one refusal: a developer cannot remove their own developer role. | Without the self-demotion rule a lone developer could lock the module out by saving their own account. Another developer can still change it. |
| DEC-27 | **Instances stay in step by comparing fingerprints** (DEV-28): `Manager.Sync` and `Service.Sync` re-read the stored rows on a timer and re-apply them only when a fingerprint of them changed, including clearing a tenant whose rows were all deleted elsewhere. Default 15 seconds when the state is in xolu (`devruntime.Options.SyncInterval`; negative never checks); no configuration key yet. | Found while building it: `Manager.Load` never notified a tenant that had lost all its state, so its overlay switches stayed stale. Fixed in `Load` itself. |
| DEC-28 | **The Developer module's decisions are flags, each with a reasoned default** (`developer_view_as`, `developer_tenant_overrides`, `developer_instance_sync`, `developer_change_notice`, `developer_protect_own_role`, `developer_form_designer`, `designer_live_preview`), plus the setting `developer_sync_seconds`. They need the module on, are developer-class (production refuses them), and the reasons are in `internal/features/catalog.go` and DEPLOY.md. Tenant overrides and the designer are off by default and on in the `development` profile. The tenant of an override, the precedence of file over tenant overrides, and the shared validation are invariants, not flags. | Tenant overrides are off by default because they change what real users see, need two entities not yet checked against a live xolu (T-106), and are read by every instance; the default can flip once that is verified. `Flag.Requires` is enforced only for switching a flag on, so a standing `=0` survives the module being turned off. |
| DEC-29 | **The form designer starts at the field level and is built on its own vocabulary.** `internal/formengine/designer` holds a catalogue (21 widgets, ~38 properties with kinds, allowed values and applicability) held to the engine by tests in both directions, and a lossless ordered document model. The page is one stateless form: the document travels in the hidden `json` field, every action is applied, re-parsed by the real engine and refused if the engine would not accept the result; Save is the existing save route. The block tree (`x-seam-form`: tabs, rows, sections), drag-and-drop and translated widget titles are not done. | The inspector draws its own controls instead of rendering a generated form-engine schema (an exception to DEC-13): its property keys contain hyphens and must post under stable names; moving it onto the engine is part of the next step. Widget and property titles are English, looked up as `designer.widget.<name>.title` and `designer.prop.<key>.title` first. |
| DEC-30 | **The module's pages are built from the application's own components and listing primitives** (`ListTable`, `ListRow`, `ListFilterBar`, `Badge`, `IconButton`, `Alert`, `EmptyState`, the card and the `SeamTabs` strip; `internal/ui/developer_ui.go` holds the few pieces with no shared counterpart: the page frame, the titled panel, the summary tile, the segmented choice, the tabs and a pager whose links keep the filters). The Overview opens with four tiles that are the way into each section; the Translations editor sits above the list; a form opens on a page of its own with Definition, Preview, Changes and Versions as tabs; the designer's controls belong to its form by name (`form=`), so the live preview is outside it. **Departure from DEC-13 (2026-11-17):** the translations list is filtered with the listing filter bar (search, language, and a new "show" filter: overridden, not translated, missing, extra, different formats), not with a form-engine schema; `translation_filter` is removed from `schemas/developer`, so three of the tools' forms remain schemas (`translation_edit`, `schema_edit`, `view_as`). | The filter is no longer overridable with the tools. A form-engine form cannot lay two fields and a button on one row without tabs and sections, and its enum options have no translated labels; the listing filter bar is what every other list uses. The classes used were checked against the compiled `tailwind.css` (no rebuild was needed); the module's own rules are the `.dev-*` block of `app.css`. |
| DEC-31 | **The tools can create, keep a history of and export what they override (2026-11-17).** *Create:* a text under a new key (`POST /developer/translations/create`, in the reference locale, in either scope) and a form under a new module/name (`POST /developer/forms/create`, starting from one field; it opens in the designer). A form is then either built in or custom (`developerForm`): a custom one exists only as an override, has no diff and no compatibility notes, and is known while an override of it is visible or the tenant's history still holds a version. Their forms are schemas (`translation_new`, `form_new`), as DEC-13 asks. This **departs from DEC-14**, which let the editors change only texts and forms that exist so that a typo could not invent a key: a new key is now created on purpose, through its own form, with a narrower shape than an override's (`i18n.ValidNewKey`: lower-case words joined by dots), and never over a key or a form that exists; the editors themselves still open only what exists. *History:* a tenant's override of a text keeps its versions in a third entity, `i18n_override_history` (append-only: every save, every real removal), beside the current row; an older wording is restored as a new version, checked again against the text it replaces. File overrides have none, as with forms. *Export:* `GET /developer/export` is the tenant's overrides as a zip in two layouts: `overrides/` (what the override directories read) and `source/` (the built-in locale files with the overrides applied, keys in their original order and new keys at the end, and each definition under its package). | The history does not move the stores' fingerprint: it adds nothing to what is live. Where a file and a tenant override of the same thing exist the file's is exported, since it is the one that applies (DEC-4); the runtime switches are ignored. A new key exists in the locales it was written in until the others are added; a new form is used by nothing until code asks for it with `resolveSchema` and a package embeds it. With no overrides the exported locale file is the built-in one byte for byte (tested). |
| DEC-32 | **A definition is checked for renderability when the tools write it, and the file scope keeps a history (2026-11-17).** *Gate:* `formengine.CheckRenderable(schema, sourceOK)` refuses a definition with no fields, a widget the engine does not have, or a select or multi-select whose source is not usable for that form; it runs on save, preview, every designer action that writes, and restore, in both scopes, and the answer is 422 with the reason. A source is usable when the form's built-in definition already names it, or when the form's module has a source resolver (`developerResolverModules`: asset); a custom form may name none outside those modules. Removing a field the built-in form requires stays a compatibility note, not a refusal. *History in files:* each override directory keeps an append-only `.history/` file (`texts.jsonl`, `forms.jsonl`) with version, time, user, note and content for every save and removal the tools make; restoring writes a new version. *Tenant texts:* an override older than the history is recorded as version 1 before its first change, and shown as version 1 until then. *Label keys:* the editors name the keys that supply a field's label by looking for `<module>.forms.<form>.field.<field>` and then `field.<field>` in the texts the tenant sees. | The loaders are not gated: a file put in the directory by hand is loaded as before and the list marks it, because refusing at load would take a form away from a running page for a mistake the page can still partly show. The file history records what the tools did, not edits made in an editor. The label-key list is a lookup of the two conventions, not a trace of the render, so a page that passes its own key prefix is not covered. Definitions that code generates per record (issue templates, work-order actions and template steps, the asset update form) have no file to override and stay outside the mechanism. |
| DEC-33 | **The directory is gated per file, outside changes are versions, and the label keys come from the page (2026-11-17).** This **replaces three points of DEC-32**. *Directory gate:* the schemas directory runs the editors' check on every well-formed file when it loads (`FileLayer.SetCheck`, wired through `devruntime.Options.SchemaCheck` to `ui.DeveloperFileCheck`). A file that fails is set aside alone: the definition loaded before it stays in force (none, if there was none), the rest loads, and `Status().Refused` carries the file and the reason to the Overview and the forms list; the file can be opened in the editor and mended. DEC-32 left the loaders ungated because the load was all-or-nothing; per-file refusal removes that reason. *Outside changes:* each load compares the directory with the history (or, for something with no history, with what the layer held before) and records what changed without the tools as a version by `ChangedOutside`; the first load of a directory records nothing. *Label keys:* `resolveSchema` notes which definition it handed out and the engine reports each render with the i18n identity it used (`fe.SetRenderObserver`); the two are matched by the definition's bytes, and from then on the editors use the engine's own key chain with the page's identity. The tools' own renders (previews, the designer, the try page) are excluded. *Try page:* `GET/POST /developer/forms/try` renders the definition in force with the asset source resolver and shows the parsed submission, required fields, repeatable bounds and query-picker checks; it stores nothing. *New text:* the `translation_new` definition carries a template field (`value_locale`) that the form repeats per language; each language written is an override of the new key, checked against the reference text. | The tenant layer is loaded as stored: only the tools write it and they are gated. A malformed file still stops the whole reload (unchanged). A file set aside was never in force and is not a version. The page identity is learnt per process and lost at restart, and is per form, not per tenant; until the page is opened the convention is used and the editor says so. On the try page a list filled by a resolver other than the asset one may be empty, and the page says which. A language whose text is refused (different placeholders) leaves the key created and opens its editor in that language. |
| DEC-34 | **The directory loads file by file, and what the tools learn and record does not depend on hot loading or on a restart (2026-11-17).** *File by file:* the schemas directory sets aside, alone, any file it cannot use (unreadable, not JSON, refused by the validator, a name no form can have, or not renderable), with the definition loaded before it left in force; only a directory that cannot be read at all leaves the layer as it was. This **replaces** the all-or-nothing reload of DEV-15 and the last exception DEC-33 kept; `LoadDir` itself stays strict for its callers. *Observing:* while hot loading is off the poller calls `Observe` on both file layers, which records changes made in the files and loads nothing, so the history has the poll interval's resolution whatever the switch says. *Identities:* `SetDeveloperTraceFile` keeps the page identities in `.history/pages.json` under the schemas directory (or the locales directory when there is no other), read at start and rewritten when one is learnt or changes. *Try page lists:* application sources as the asset pages resolve them; any other `entity:<name>` by reading that entity for the tenant (first 200 rows, labelled by the first naming field present). *Export:* for a new form in a module the tree does not have, `source/` includes the package file (`//go:embed *.json`, `Schema`, `Names`); the README lists, per new form, the embed or catalogue step and the render call. | The translations directory keeps its all-or-nothing reload: one broken locale file leaves the previous texts in force (unchanged, DEV-12). Two changes to a file inside one poll interval (two seconds by default) are one version: the directory is polled, not watched. With no override directory the identities are per process. The try page's lists have no page-specific filter (enabled only, all but the current record), and any-picker fields search through their own endpoint as on a real page. A new form still needs a page written for it; the export shortens the way there. The generic list read was exercised on a live stack only with empty entities (the seed has no warehouses or consumables); the asset sources were seen filled. |
| DEC-35 | **Both directories load file by file; the build carries the page identities; the history is looked at more often than the layers are loaded (2026-11-17).** *Translations directory:* the rule DEC-34 gave the schemas directory now holds here too (`i18n.FileLayer`, `FileStatus.Refused`); this **replaces** the all-or-nothing reload of DEV-12, the exception DEC-34 kept. A locale whose file is set aside keeps the texts it had, and is not recorded as having lost them. *Identities:* `developer_pages_seed.json` is embedded: the identities seen by opening every form page on a live stack, plus the ones the source states as literals; a test fails when a literal site and the table disagree, or when the table names a form that is not built in. What an instance learns is still kept in `.history/pages.json` and wins over the table. A form can have several identities (the two steps of a work-order template render one definition under two); the editors list the keys of each. *History:* `Poll` looks at its directory every `HistoryInterval` (250 ms) for the history only (`Observe`) and loads at its own interval. *Try lists:* a row with `enabled`, `active` or `is_active` stored as false is left out, which is the filter the pages that fill these lists share. | Seven built-in forms are not in the table (their pages need records the seed data lacks, or build their identity where the test cannot read it): they are named by the convention until their page is opened, and the editor says so. The table is correct for a build only as far as the literal sites are checked; a page that changes its identity elsewhere corrects itself when rendered. Two changes to a file within 250 ms are one version: closing that needs file-system events, which is a new dependency (fsnotify) and has not been added. A try list cannot apply a filter that depends on the record being edited. A new form still needs a page written for it (unchanged; see DEC-31). |
| DEC-36 | **The decisions taken to finish the campaign's open items, each configurable (2026-11-17).** *Global scope (DEV-26):* stored as the tenant scope is, under `overrides.GlobalTenant` (0), applied through the layers DEC-4 already ordered (built-in, global, tenant, file). The rule for who may edit it is a configured home tenant, `developer_global_tenant`: its developers are offered a third scope in the editors and nobody else can post to it. Default 0, nobody: what the scope changes is seen by tenants its author does not belong to, so a deployment names who may. The export stays the tenant's own overrides. *Module switches (DEV-27):* `developer_module_switches` (on) mounts the Demo and Work Orders routes even when their own flag is off and asks each request whether the module is on for its tenant (`module_demo`, `module_work_orders`, runtime, off); the module registry asks the same per tenant (`HasAccessFor`). On at startup stays on for everyone and shows no switch. Default on because nothing changes until a developer uses a switch; off restores unmounted routes. *History interval:* file-system events were weighed against polling and polling kept: events are exact but add a dependency and behave differently on Linux, macOS and network file systems. `developer_history_ms` (250; -1 for only at load) and `developer_poll_ms` (2000) make the resolution a setting instead. *Unreferenced texts (DEV-19):* found by reading a source checkout (`developer_source_dir`), not shipped with the binary, so the list is always about the tree in front of the developer; default off, set by `make dev`. *Preferences (DEV-29):* kept in a cookie limited to the module's pages rather than in the user record or with the tenant's state: they affect nobody else and need no storage or sync. *Tenant overrides default:* `developer_tenant_overrides` stays off outside the development profile although T-106's reason is gone (verified live): it changes what a tenant's users see, so a deployment opts in. *DEV-4:* the optional switch of `server.go` and `ui` from the feature booleans to `cfg.Flags()` is not done and not planned: the booleans are the config-file layer and every consumer reads them correctly. | The global scope has been run live on a single-tenant seed (two instances): that another tenant sees a global override is covered by tests, not by a live run. A module switched on at runtime shows for the tenant's every user who has access to that module, not only for developers. Unreferenced texts are candidates (`internal/i18n/keyusage` lists the rules): 80 of 2,572 on this tree. Preferences are per browser, not per account. The form designer's block tree (DEV-21) is the one campaign item left open. |
| DEC-37 | **What was missing for daily use, and the decisions it took, each configurable (2026-11-17).** *Block tree (DEV-21):* the designer edits the layout of a form (`x-seam-form`) as a tree: tabs, their sections, the sections side by side in a row, and the fields in each. The model is `designer/layout.go`; it writes the two spellings the engine already reads (plain `sections`, or `rows` with `section_defs` once two sections share a row), keeps every key it does not know, and each change is checked by the engine before it is kept. A field is placed by dragging it onto a section or with the buttons; both post the same actions, so the page works without scripts. Details and embeds of a tab are edited as the JSON lists the engine reads, not through controls of their own: they are few, their shape varies per source, and a wrong one is refused with the engine's message. Adding, renaming and removing a field keeps the layout in step; a field the layout places can now be removed. *Search by use:* the forms list also matches what a definition uses (a widget, a source, a field name) and says which. *Identity table:* all 37 built-in forms are in the seed table, and a test fails when one is not or when the table disagrees with a page that renders. *Following the directories (`developer_watch`):* `auto` (default) uses file-system events where the platform has them (inotify on Linux, through the standard library, no new dependency) and polling elsewhere (macOS included); `events` and `poll` force one. Polling stays on beside events as the safety net, so a missed event costs at most one interval. Two saves 120 ms apart are two versions on Linux. *Module audience (`developer_module_audience`):* a module switched on from the panel is on for the developers of the tenant (`developers`, default) or for every user of it (`tenant`). The default is the narrow one because the switch exists to look at a module, and widening is one setting away. *Preferences (`developer_preferences`):* kept with the account (`account`, default), with the cookie as the browser's copy, or in the cookie only (`browser`). For this the user store now merges its fields into the user record instead of replacing it: every sign-in replaced the record and removed whatever else was kept there. *Custom form pages (`developer_custom_pages`, off):* with it on, a form that exists only as an override has a page, `/forms/{module}/{name}`, for any user of the tenant, and what it receives is stored as `form_submission` rows the form editor lists. Off by default because it is a page the tenant's users can reach; a built-in form never has one. *Export:* includes the global-scope overrides, below the tenant's own and the files. *Forms generated per record:* the asset update form now follows the override of the asset definition. Issue templates, work-order actions and template steps are defined by their own records, which is where they are edited; they stay outside the override mechanism by design. |
| DEC-38 | **Five items taken off the open list (2026-11-17).** *Quick changes to a file on macOS:* accepted. A Mac is a development machine and servers are Linux, where the directories are watched; on macOS they are polled, and two hand edits of one file inside `developer_history_ms` are one version. No kqueue or FSEvents watcher is planned. *Details and embeds in the designer:* JSON lists per tab, checked by the engine, are the form they keep; controls of their own are dropped until something asks for them. *Texts not referenced in the source:* a documented limit, not a defect. A scan of the source cannot see a key assembled at run time in a way its rules do not model, so the list is of candidates and the editor shows the rule that accounts for each text. *Preferences:* the gap is closed. With `developer_preferences` on `account` the account is read on each page that offers the choices (one read of the user record) and the cookie follows it, so a choice made in one browser is what any other opens with; the cookie alone is used when the account cannot be read, and an account that has none takes the cookie's. *Custom form pages:* storing a submission and listing it in the form's editor is the whole of the feature. The page exists to try a form with real users before its code is written; what a submission then becomes is for that code to decide. |
| DEC-39 | **The designer's last three items (2026-11-17).** *Duplicate, done:* a field, a section and a tab can be duplicated (`designer/duplicate.go`). A field name is unique in a form and a block ID in its layout, so a copy takes the next free name (`phone_2`, `contact-2`); the copy of a field follows its original in the list and in its section; the copy of a section or a tab holds copies of its fields, since a field has one place, and a tab's copy keeps its rows, widths, details and embeds. Labels are copied as they are. *Preview language, done:* the preview has a language select; it is a control of the designer's form, so the choice travels with every action and needs no state of its own. *Preview role, dropped:* the engine does not consult roles (it hides what `RenderContext` lists) and no page of the application resolves per-role form permissions today, so a role selector would change nothing in the preview. It belongs to whatever work introduces those permissions; view-as (DEV-22) already covers the pages. *Inspector rendered by the engine, dropped:* it was a preference of the original plan, the designer built with its own engine. The designer's inspector handles typed values and per-widget properties, is translated in four locales, and a rewrite would change nothing a developer sees. DEV-21 is done with this. |

---

## 4. Open questions and findings

Findings from DEV-2 (2026-10-10, read-only, nothing compiled).

| ID | Question | Finding | State |
|----|----------|---------|-------|
| Q-1 | Is role assignment rank-checked? | **No.** Creation and update only call `IsValidRole` (`internal/auth/users.go` lines 69 and 182; `internal/ui/routes.go` line 2501). Any admin can already assign any valid role. Adding `developer` to the valid set would let admins assign it unless a rule is added (DEV-6). The `seam-admin` CLI has its own hard-coded role maps (`cmd/seam-admin/main.go` lines 222 and 415) and is the natural bootstrap path for the first developer. | Answered |
| Q-2 | What happens when auth is disabled? | `requireAuth` passes through with **no claims in the context** (`internal/ui/router.go` lines 65-71). Demo relies on `requireAuth` alone. Later reading showed `requirePerm` (`router.go` lines 155-170) already fails closed: it redirects to login when claims are absent and audits and returns 403 when the permission is missing. The Developer routes should therefore use `requirePerm(auth.PermDeveloperAccess, ...)` rather than a new wrapper, in addition to the startup guard (DEC-11). | Answered |
| Q-3 | Does the audit store fit? | `AuditLogger.Log(ctx, *AuditEvent)` is generic, with typed convenience methods. New event types can be added without new plumbing. Field-level fit is checked when DEV-8 is written. | Answered, to confirm at DEV-8 |
| Q-4 | Overlap with `settings_features.json`? | None of substance: it holds a single tenant-level field, `map_enabled`. Keep deployment flags (Developer panel) and tenant settings (Settings) separate. The panel may link to Settings. | Answered |
| Q-5 | Is any schema cached? | **No.** There is no `sync.Once` or `sync.Map` in `formengine` or `ui`, and `ParseSchema` runs on each render. About 27 call sites in `internal/ui` pass `xschemas.Schema("name")` straight to `Render(ctx, ...)` (routes, workorders, issues, demo, masters). The `asset` package differs: `Schema()` and `SchemaForUpdate()` take no name. Masters renders with `context.Background()` and needs the request context. | Answered |
| Q-6 | Where is `ValidRoles` consumed? | See DEV-6 for the full list of hard-coded role sites. | Answered |
| Q-7 | Does any handler bypass context claims? | Role is read from claims in the context almost everywhere (`handler.go`, `router.go` permission checks, `selector.go`). Direct `ValidateSession` calls exist in `router.go` (the two `requireAuth` variants, which build the claims) and at `routes.go` lines 2904 and 3040 (not yet inspected). User administration reads `currentUser.Role` from the store, so under view-as those pages would still use the real role. A context rewrite at the single point where `requireAuth` calls `ContextWithClaims` (and its raw-body twin) is feasible. | Answered, two sites to inspect at DEV-22 |
| Q-8 | The Users list has no tenant condition. A live exposure now that shared instances are confirmed? | Confirmed worse while working on DEV-6: `AdminUsersList` runs `SELECT TOP 100 * FROM users` with no tenant condition, and `AdminUserEdit`, `AdminUserUpdate`, `AdminUserPasswordReset` and `AdminUserPasswordResetSave` load the target with `xoluClient.Get(ctx, "users", id)` and never compare its tenant with the caller's. On a shared instance an admin of one tenant can list, edit and reset passwords of another tenant's users. Not changed here (outside this campaign); needs its own item and decision. | Open (T-98) |
| Q-9 | Where does a single-user instance get its tenant ID? | There is no single-user mode in the code. `seam-admin` defaults the tenant to 1, and tenant-scoped code requires `TenantID > 0` in claims. A single-user instance is just tenant 1 (or whichever ID was created). Tenant 0 is therefore free to mean "global" (DEV-26). Code must not assume how many tenants exist. | Answered |

---

## 5. Work items

Index (details follow).

| ID | Title | Phase | Status |
|----|-------|-------|--------|
| DEV-1 | Establish a verified build and test baseline | 0 Preconditions | ☑ |
| DEV-2 | Settle open questions Q-1 to Q-7 and Q-9 by reading code | 0 Preconditions | ☑ |
| DEV-3 | `features` package: flag table, strict parsing, layering | A Foundations | ☑ |
| DEV-4 | Migrate existing flags onto the `features` package | A Foundations | ☑ |
| DEV-5 | `SEAM_ENABLE_DEVELOPER` flag and startup guard | A Foundations | ☑ |
| DEV-6 | `developer` role | A Foundations | ☑ |
| DEV-7 | Developer module skeleton | A Foundations | ☑ |
| DEV-8 | `feature_state` xolu entity and store | B Runtime state | ☑ |
| DEV-9 | Control panel | B Runtime state | ☑ |
| DEV-10 | Visible indicator when runtime developer flags are active | B Runtime state | ☑ |
| DEV-11 | `Translator.Reload` and per-tenant view | C Overrides | ☑ |
| DEV-12 | i18n filesystem layer | C Overrides | ☑ |
| DEV-13 | i18n xolu layer | C Overrides | ☑ |
| DEV-14 | Tenant-aware schema resolver | C Overrides | ☑ |
| DEV-15 | Schema filesystem layer | C Overrides | ☑ |
| DEV-16 | Schema xolu layer with version history | C Overrides | ☑ |
| DEV-17 | Schema validation gate | C Overrides | ☑ |
| DEV-18 | i18n editor | D Developer tools | ☑ |
| DEV-19 | i18n sync and completeness view | D Developer tools | ☑ |
| DEV-20 | Schema browser, raw JSON editor and live preview | D Developer tools | ☑ |
| DEV-21 | Form designer (block tree) | D Developer tools | ☑ |
| DEV-22 | View-as role | E Later | ☑ |
| DEV-23 | Seeds update | Cross-cutting | ☑ |
| DEV-24 | Documentation and configuration updates | Cross-cutting | ☑ |
| DEV-25 | Flag consistency test | Cross-cutting | ☑ |
| DEV-26 | Global-scope editing of overrides | Deferred | ☑ |
| DEV-27 | Runtime toggling of existing modules (Demo, Work Orders) | Deferred | ☑ |
| DEV-28 | Multi-instance reload | Deferred | ☑ |
| DEV-29 | Per-developer preferences | Deferred | ☑ |
| DEV-30 | Developer option in the user role selects | Deferred | ☑ |
| DEV-31 | Tenant-aware translation in the UI render path | Deferred | ☑ |
| DEV-32 | Migrate schema call sites to the resolver | C Overrides | ☑ |

### Phase 0: Preconditions

### DEV-1. Establish a verified build and test baseline
Status: ☑ (2026-10-12) · Blocks: everything that writes Go.
Done: the tree builds, vets and runs its tests (31 packages pass; the eight failing `internal/ui` tests are T-102 and predate this campaign), and the Developer module has been run by a person. `go.mod` says `go 1.26.5`; the sandbox builds it with Go 1.27.2.
Run `go build ./... && go vet ./... && go test ./...` where the toolchain and
the xolu checkout exist, and record what is red. `internal/ui/workflow` is
known stale (T-86). Confirm the effect of T-94, T-95 and T-96 being
uncompiled. Alternative: open the sandbox's network settings for the Go hosts
and supply the xolu checkout. Also note that `go.mod` says `go 1.26.5`, which
sits outside the Ubuntu LTS baseline policy of 1.22; decide whether that is
intended.

### DEV-2. Settle open questions Q-1 to Q-7 and Q-9 by reading code
Status: ☑ (2026-10-10) · Depends on: none (read-only). Findings in section 4.
Answer each question in section 4 and record the finding beside it. Q-8 is
tracked separately as a defect candidate (see T-98).

### Phase A: Foundations

### DEV-3. `features` package: flag table, strict parsing, layering
Status: ☑ (2026-10-10) · Depends on: none in practice (stdlib only).
Done: `internal/features` (`features.go`, `resolve.go`, `catalog.go`, tests). Compiled, vetted and tested in an isolated go 1.22 scratch module (92.9% statement coverage); not yet built under the repo's own `go.mod` (DEV-1). Notes: `SEAM_AUTH_ENABLED` and similar non-flag booleans still use the old `"true" or "1"` parsing; the `development` profile enables `developer`, so it fails at startup without auth, by design; profile bundles are proposals, adjust freely; poll interval and other non-boolean panel settings are outside this package.
Declarative flag table: key, env var, default, description, class
(`stable`, `preview`, `dev`), static or runtime, optional "requires auth".
One shared boolean parser accepting true/1/yes/on and rejecting anything else
at startup (today `TRUE` or `yes` silently mean false). Precedence: defaults,
`config.json`, individual env vars, then an optional `SEAM_FEATURES` list such
as `developer,demo,-work_orders`. Resolved once into an immutable set. Profiles
(`SEAM_PROFILE=production|development|demo`) that refuse `dev`-class flags in
production unless explicitly overridden. Startup log of effective flags and the
source of each. Table-driven tests.

### DEV-4. Migrate existing flags onto the `features` package
Status: ☑ (2026-11-17) · Depends on: DEV-3.
Closed (2026-11-17): the optional switch of the consumers to `cfg.Flags()` is not planned (DEC-36); the behaviour change is in the CHANGELOG.
Done: the six hand-written env blocks (UI, API, operational, admin, work orders, demo) are replaced by `Config.resolveFeatures` in `internal/config/features.go`, which writes results back into the `FeatureConfig` booleans so no consumer changed. The struct acts as the config-file layer (a field differing from its default counts as set in config), which also preserves hand-built configs in tests. Bad values now fail `Validate` instead of silently meaning false (a deliberate behaviour change). `config.Features.Flags` (JSON `flags`) sets any flag by key, including runtime flags, for pinning. Remaining: switch `server.go`/`ui` to read `cfg.Flags()` instead of the booleans (optional, later), and document the behaviour change in the CHANGELOG (DEV-24). Tested in the scratch module with `config` and `features` together.
Replace the roughly ten duplicated env blocks in `internal/config/config.go`
and the copying of flags into the UI config. Keep existing variable names
working. Staged: new flags first, old flags migrated one at a time with tests.

### DEV-5. `SEAM_ENABLE_DEVELOPER` flag and startup guard
Status: ☑ (2026-10-10) · Depends on: DEV-3.
Done: `FeatureConfig.EnableDeveloper`, `SEAM_ENABLE_DEVELOPER`, the `RequiresAuth` rule in `features`, and a second, independent check in `Config.Validate`. Also done: `server.New` logs the resolved flags once at startup with a warning per development-class flag that is on (`Set.Log`), and `devruntime.Start` logs a further warning when the module is active. Routes are guarded by `requirePerm` (DEV-7).
The master switch. Refuse to start when set with auth disabled (DEC-11). Loud
startup warning. When off, persisted runtime developer state is inert.

### DEV-6. `developer` role
Status: ☑ (2026-10-13)  · Depends on: DEV-2 (done).
Updated (2026-10-13): the guards written without a compiler are now compiled and tested (`developer_guard_test.go`), and the behaviour decision that the web forms never offer the role was replaced by DEV-30 (a developer is offered it; nobody else is).
Done and tested in the scratch build (`internal/auth`): `RoleDeveloper`, rank 5, in `ValidRoles` and `IsValidRole`; permissions derived in `init()` as admin's list plus `PermDeveloperAccess` (`developer.access`), so they cannot drift; `Claims.HasRole` made rank-based (the developer passes every check admin does); `CanAssignRole` and `CanManageUser` (only developers may grant or manage the developer role). Also done: `seam-admin` accepts the role (the way to create the first developer); `user.role.developer` and `users.role.developer` in all four locales (key sets verified identical, 874 each); `Enum` in `UserUpdateSchema` extended. Written but NOT compiled or run (no compile loop for `internal/ui`, see DEV-1): `internal/ui/developer_guard.go` and guards in `AdminUsersList` (developer rows hidden from non-developers), `AdminUserCreate`, `AdminUserEdit`, `AdminUserUpdate`, `AdminUserPasswordReset` and `AdminUserPasswordResetSave` (which now refuses with 404 if the target cannot be read), plus `internal/ui/developer_guard_test.go`. The diff to `routes.go` is 33 added lines and parses. Behaviour decision: the UI never offers the role and never demotes a developer (a developer's submitted role is kept); roles are managed with `seam-admin` until DEV-30. Remaining: run the UI tests (DEV-1), the dev-mode fallbacks in `routes.go`, the role label switches in `admin_users.go` and `profile.go` (a developer would show an unlabelled role), and the "cannot delete admins" rule in `admin_users.go` (also applies to developers).
`RoleDeveloper`: admin's permissions plus `developer.access`; rank above admin
in `internal/auth/roles.go`; add to `ValidRoles`. The user create and edit
selectors offer it only when the flag is on, and only to developers. Admins
cannot assign it. Tests for rank, visibility (T-96) and assignment.
Hard-coded role sites found by DEV-2 that must all be handled:
`Claims.HasRole` in `internal/auth/auth.go` (only the literal `"admin"` is a
superuser, so a developer would be refused; make it rank-based);
`IsValidRole` and `ValidRoles` in `internal/auth/users.go`; the role enums in
`schemas/system/user_create.json` and `user_edit.json` (the selector must not
offer `developer` unless the flag is on and the caller is a developer); the
`Enum("admin", ...)` list in `internal/validation/schemas.go` line 841; the
role tabs, label switch and the "cannot delete admins" rule in
`internal/ui/admin_users.go`; the label switch in `internal/ui/profile.go`;
`user.role.*` and `users.role.*` i18n keys in all four locales; the dev-mode
fallbacks in `internal/ui/routes.go`; and the two hard-coded role maps in
`cmd/seam-admin/main.go`. Developer accounts must not be deletable from the
Users list, as with admins.

### DEV-7. Developer module skeleton
Status: ☑ (2026-11-17)   · Depends on: DEV-5, DEV-6 (both substantially done).
Closed (2026-11-17): the skeleton is complete; what is left of the designer is DEV-21's own.
Updated (2026-10-13): view-as (DEV-22) lives on the Overview. What remains is DEV-21, the form designer.
Updated (2026-10-12): the module is four sections with their own left-menu entries (DEC-12). The sections beyond Overview, Translations and Form definitions arrive with DEV-21 and DEV-22.
Written, mirroring the Demo module: `internal/modules/developer/{routes.go,routes_test.go}`; `Router.MountDeveloperRoutes` (`GET /developer`, `requirePerm(PermDeveloperAccess)`, no-op unless `EnableDeveloper`); `ui.Config.EnableDeveloper` and `ui.Config.Flags`; `internal/ui/developer.go` (overview page: planned tools plus every flag as resolved at startup, read-only, with source, class, mode and pinned marker); registry entry and `uiCfg` wiring in `server.go`; 15 new i18n keys in all four locales (889 each, identical sets). Verification: every file parses; `developer.go` and `developer_guard.go` compile and vet against a stand-in for minty's API (assumed signatures taken from existing call sites), not the real library. NOT compiled or run for real: the router, server and module-test edits (DEV-1). Remaining: sections beyond the overview arrive with DEV-9, 18-21.
New `internal/modules/developer` package, `MountDeveloperRoutes` on the router
(no-op without the flag), registry entry in `internal/server/server.go` with
`HasAccess` = flag and `HasModuleAccess(role, "developer")`. Every route uses
`requirePerm(auth.PermDeveloperAccess, ...)`, which already fails closed when
claims are absent or lack the permission (Q-2). Navigation and
sidebar i18n keys in all four locales (en, es, ja, pt), keeping key sets
identical (`scripts/diff-locales.py`).

### Phase B: Runtime state

### DEV-8. `feature_state` xolu entity and store
Status: ☑ (2026-10-13)  · Depends on: DEV-3; Q-3 and Q-9 answered.
Updated (2026-10-13): implemented and tested; the one thing not verified, the `feature_state` entity against a real xolu, is tracked as T-106 together with the override entities.
Done and tested in the scratch build: `internal/devstate`. `Manager` enforces every rule in DEC-6, DEC-7 and DEC-9: per-tenant state; only runtime flags change; pinned flags are refused; stored state is inert while the module is off (a default-on flag stays on, so stale rows cannot disable overrides in a deployment that has the module off); tenant must be positive; the store is written before memory and observers are notified only on success. `Panel(tenant)` supplies the control panel's view. `MemoryStore`, and `XoluStore` over the existing `auth.XOLUClient` interface (one `feature_state` row per tenant and key: `tenant_id`, `key`, `value`, `changed_by`, `changed_at`; upsert via query then update or create; keys quoted in OQL), tested against a fake client including persistence across a simulated restart. `Wire` pushes each tenant's overlay switches into the translator and schema resolver. The audit event `AuditDeveloperChanged` and `AuditLogger.LogDeveloperChange` were added to `internal/auth` (Q-3 closed; tested). Not verified: the `feature_state` entity against a real xolu (whether xolu needs the entity declared first is unknown), and the real `xoluClientAdapter` path. Race detector clean.
Per-tenant rows: key, value, changed by, changed at. Never tied to a session.
Every change audited. Correct under shared multi-tenant instances and under
single-user instances (DEC-8).

### DEV-9. Control panel
Status: ☑ (2026-11-17)  · Depends on: DEV-7, DEV-8; Q-4 answered.
Done (2026-11-17): the poll interval is a setting (`developer_poll_ms`), with `developer_history_ms`; both are startup settings, not panel switches, because the pollers are the instance's and the panel's state is a tenant's (DEC-6, DEC-36). The Startup page lists them.
Updated (2026-10-13): the handler tests exist (`developer_e2e_test.go`). Still open: the poll interval as an editable setting (the directory pollers use two seconds; the instance sync fifteen, DEC-27).
Page reworked (2026-10-12): each runtime switch is a card on a grid with one Enable | Disable control; overrides, translation completeness and the startup flags are separate sections. Still open: the poll interval as an editable setting, and handler tests for the two POST handlers.
Written: `internal/ui/developer.go` (page, `DeveloperFlagSet`, `DeveloperReload`), routes `POST /developer/flags` and `POST /developer/reload` (both `requirePerm(PermDeveloperAccess)`; CSRF is validated centrally by `requireAuth` for state-changing requests), and 29 further i18n keys (918 per locale, identical sets). The page shows each runtime flag with On, Off and Reset controls or the reason it is locked; each filesystem layer's status, last error and a reload button; and the startup flags read-only. Messages shown come only from a fixed code table, so nothing from the query string is echoed. Every change is audited. `devruntime.Start` and the server wiring (`devRuntime`, `devCancel`, `uiCfg.DevRuntime`) are in `server.go`. Verification: `developer.go` compiles and vets against stand-ins for minty's API; the logic it calls is tested (devstate, devruntime). NOT verified: the real minty calls, the router, and the server wiring (DEV-1). Not yet done: the poll interval as an editable setting (non-boolean; `devruntime.Options.PollInterval` is fixed at two seconds), and handler tests for the two POST handlers. Note the per-tenant `overlay_*` and `show_i18n_keys` switches do not yet change page text (DEV-31).
Shows every flag with its class and source. Startup-only flags are read-only.
Runtime flags are editable unless pinned, in which case the panel shows the
lock and its reason. First runtime flags: hot-load i18n, hot-load schemas, poll
interval, filesystem layer on/off, xolu layer on/off, and optionally a
"show i18n keys instead of text" mode.

### DEV-10. Visible indicator when runtime developer flags are active
Status: ☑ (2026-10-13) · Depends on: DEV-9.
Done (2026-10-13): every page of a tenant shows a notice while a behaviour-changing runtime switch is on (DEC-19); `developerActiveFlags`, `devBannerNodes`, tested end to end across roles.
A small badge in the layout for all users of the tenant, so a tester under a
different role can see that hot loading is on. Optional auto-expiry after N
hours.

### Phase C: Overrides

### DEV-11. `Translator.Reload` and per-tenant view
Status: ☑ (2026-10-12) · Depends on: none (pure; flag gating comes from DEV-9).
Done: the translator's tenant view now reaches rendered page bodies (`UIHandler.translateFunc(r)`; see DEV-31 for the navigation chrome).
Done: `internal/i18n/overlay.go` and tests. `ReloadLayer(layer, tenant, byLocale)` replaces a layer atomically; layers are filesystem, xolu-tenant, xolu-global, embedded; `ForTenant(id)` gives a tenant view (a per-tenant cache proved unnecessary: lookup is a few map reads); per-tenant `Options` (disable filesystem, disable xolu, show keys) carry the panel toggles; `Explain` reports every layer's value and which is effective, and `Effective` returns a merged copy, both for the editor (DEV-18) and sync view (DEV-19). `T` is unchanged for callers and applies global layers. The atomicity test was mutation-checked (a non-atomic reload fails it) and the package passes the race detector. Remaining: switch UI call sites that should honour tenant overrides from `t.ForLocale(...)` to a tenant view, once DEV-1 gives a compile loop for `internal/ui`.
Atomic reload per locale (I18N_SPEC §5.2). `ForTenant(id)` returns a view
derived from the base and cached, rebuilt on reload. The existing `T(locale,
key)` remains the global path. Builds on the current flat 872-key format; it
does not wait for the nested `LocalePackage` refactor.

### DEV-12. i18n filesystem layer
Status: ☑ (2026-10-12) · Depends on: DEV-11.
Done: constructed from `SEAM_LOCALES_DIR` by `devruntime.Start`, polling gated on the hot-load flag, status and a reload button on the page.
Done: `internal/i18n/filelayer.go` and tests. `LoadDir` reads `<locale>.json` files (flat key to message, the embedded shape); all problems are reported together, naming each file. `FileLayer.Reload` is all-or-nothing: any invalid file leaves the previous layer in force, so a half-saved edit never blanks translations. `ReloadIfChanged` compares a name/size/mtime signature and reports a broken file once, not on every poll. `Poll(ctx, interval, active)` blocks until cancelled and skips checks while `active` is false (the hot-load flag). `Status` gives loaded time, locales, key count, last error, and keys the base does not define (usually typos). Race detector clean. Remaining: construct it in `server.go` from `SEAM_LOCALES_DIR`, start `Poll` with the hot-load flag as `active`, and expose `Status` and a manual reload in the panel (DEV-9). Note that the filesystem layer is global (not per tenant); tenant switches only turn it on or off via `Options`.
`SEAM_LOCALES_DIR` (unset means the layer is off). Per-key overlay files.
Modification-time polling (no new dependency) plus a manual reload button.

### DEV-13. i18n xolu layer
Status: ☑ (2026-10-13) · Depends on: DEV-8, DEV-11.
Done (2026-10-13): translation overrides stored per tenant in `i18n_override`, loaded at start, rebuilt after each change (`overrides.Service`, DEC-20), edited from the Translations section in the tenant scope. Not yet run against a live xolu (T-106); no cross-instance reload (DEV-28).
`locale_package` documents per locale and per tenant (I18N_SPEC §5.3), loaded
over the embedded files at startup and reloaded in-process on save.

### DEV-14. Tenant-aware schema resolver
Status: ☑ (2026-10-12) · Depends on: none (Q-5 answered).
Done: consumed by 25 production call sites through `resolveSchema` (DEV-32).
Done and tested (race clean): `internal/schemaoverlay`. `Resolver.Resolve(tenant, module, name, base)` returns the highest-precedence override (filesystem, xolu-tenant, xolu-global) or `base`; a nil `*Resolver` returns `base`, so a deployment with the module off pays nothing. Also `EffectiveLayer`, `Explain`, `Overridden` and per-tenant `Options`. Not done: switching the call sites (DEV-32).
Revised design: leave the seven schema packages untouched. Add a resolver with
`Resolve(ctx, module, name string, base []byte) []byte` that returns the
effective override for the request's tenant, or `base` unchanged. About 27 call
sites in `internal/ui` change from `xschemas.Schema("n")` to
`h.schemas.Resolve(ctx, "x", "n", xschemas.Schema("n"))`. The `asset` package
(`Schema()`, `SchemaForUpdate()`) and the masters pages (which use
`context.Background()`) need individual handling. Migrate incrementally.

### DEV-15. Schema filesystem layer
Status: ☑ (2026-10-12) · Depends on: DEV-14.
Done: takes effect in the UI now that the call sites use the resolver (DEV-32).
Done and tested: `schemaoverlay.LoadDir` (layout `<dir>/<module>/<name>.json`), `FileLayer` with all-or-nothing validating reload, change detection, `Poll` gated on the hot-load flag, and `Status`. Started from `devruntime.Start` with `SEAM_SCHEMAS_DIR` (config `schemas_dir`). It takes effect in the UI only once the call sites use the resolver (DEV-32).
`SEAM_SCHEMAS_DIR`. Whole-document replacement. No lifecycle: a developer's
working copy. Mtime polling plus manual reload.

### DEV-16. Schema xolu layer with version history
Status: ☑ (2026-10-13)  · Depends on: DEV-8, DEV-14.
Done (2026-10-13): form-definition overrides per tenant in `schema_override`, append-only, with version history, notes, removal as a version and Restore (DEC-20, DEC-23), edited from the Form definitions section. Same limits as DEV-13.
Done and tested: `schemaoverlay.Compat(base, override)` returns warnings for a removed property, a changed type and a newly required property (advisory, not a gate), and `Resolver.ReloadXoluGlobal` and `ReloadXoluTenant`. Not done: the `schema_override` xolu entity (proposed: one row per save with `tenant_id`, `module`, `name`, `version`, `schema_json`, `changed_by`, `changed_at`; the effective schema is the highest version; rollback saves an older version as a new row, so history is never lost), its store, and loading it at startup.
`schema_override` entity, per tenant. Version history and rollback from the
start. Warn when an override removes or renames fields present in the base. The
full §15.5 compatibility checker stays with the Admin roadmap.

### DEV-17. Schema validation gate
Status: ☑ (2026-10-10) · Depends on: none for the gate itself; wired in by DEV-15 and DEV-16.
Done: every override passes a JSON syntax check and the injected validator (in `server.go`, a wrapper around `formengine.ParseSchema`) before it can take effect; `Resolver.Validate` is available to the editors. Reloads are all-or-nothing and report every problem, naming the schema.
Every save passes `ParseSchema`; on failure the previous good version stays in
force and the error is shown. Applies to both layers.

### Phase D: Developer tools

### DEV-18. i18n editor
Status: ☑ (2026-10-13)  · Depends on: DEV-9, DEV-11.
Updated (2026-10-13): tenant-scope editing and the shadow notices (a file override hides a tenant's, and the reverse) are in (DEC-23).
Done (2026-10-12): the Translations section (`/developer/translations`). A filterable, paged list of every text with the layer that supplies it; an editor for one text showing the reference text, the built-in text and the current override; Save and Remove through `FileLayer.SetOverride`/`RemoveOverride` (DEC-14), live at once; only when `SEAM_LOCALES_DIR` is set. Still open: marking shadowed entries across layers (DEC-4), which needs the xolu layers (DEV-13), and tenant-scope editing (DEC-9, DEC-17).
Per-key editing; shows the effective layer for each key and marks shadowed
entries (DEC-4); writes to the chosen layer.

### DEV-19. i18n sync and completeness view
Status: ☑ (2026-11-17) · Depends on: none for the pure comparison logic; UI after DEV-9.
Done (2026-11-17): unused keys, as texts not referenced in a source checkout (`internal/i18n/keyusage`, the "not referenced" filter; DEC-36).
UI done (2026-10-12): the page shows, per locale, missing, extra, untranslated and format-mismatch counts against `en` as the tenant sees it. Still open: unused-key detection.
Done: `internal/i18n/compare.go` and tests: `Compare` (and `Translator.Compare(tenant, ref)`, which sees overlays) reports per locale the missing keys, extra keys, untranslated values (identical to the reference and with at least three letters, with an optional `IgnoreIdentical` hook) and printf-verb mismatches, which matter because `T` passes messages to `Sprintf`. Run against the real embedded files: 889 keys in en, es, ja, pt with no missing, extra or mismatched keys; 24 (es), 8 (ja) and 25 (pt) untranslated candidates, mostly legitimate cognates and acronyms. Not done: unused keys (needs scanning source for `t("...")` uses) and the UI.
Port the logic of `scripts/diff-locales.py` into Go: missing keys per locale.
Extend it with values identical to English (probably untranslated) and unused
keys.

### DEV-20. Schema browser, raw JSON editor and live preview
Status: ☑ (2026-10-13)   · Depends on: DEV-9, DEV-14, DEV-17.
Updated (2026-10-13): the xolu layer and the version history are in (DEV-16); with the diff, the editor is complete for this campaign.
Updated (2026-10-13): the editor now shows a line diff against the built-in definition for the text in the box, both for a saved override and for a preview before saving (bounded at 2000 lines, unchanged runs collapsed). Still open: the xolu layer and version history (DEV-16).
Done (2026-10-12): the Form definitions section (`/developer/forms`). The catalogue of every form with its effective layer (and overrides no page uses, flagged); a raw JSON editor; Preview (validates with the form engine and renders it, writing nothing); Save (validated before writing, refused definitions keep what was typed and say why, compatibility notes from `schemaoverlay.Compat`); Remove. Still open: the xolu layer and version history (DEV-16), and a diff against the built-in definition.
List schemas with their effective layer; edit raw JSON; preview through the
real form engine, reusing what the Demo module already does.

### DEV-21. Form designer (block tree)
Status: ☑ (2026-11-17) · Depends on: DEV-20.
Done (2026-11-17): duplicating a field, a section and a tab, and the preview's language select (DEC-39).
Done (2026-11-17): the block tree (DEC-37): tabs, sections, sections side by side in a row, fields placed in sections by drag and drop or by buttons, details and embeds as JSON lists per tab; widget and property titles translated; the page run in a browser on a live stack.
Done (2026-10-13): the field level, behind `developer_form_designer` (DEC-29). The vocabulary (`designer.Widgets`, `designer.Props`, `WidgetOf`, held to the parser and to every embedded schema by tests), a lossless ordered document model (`designer.Doc`: add, remove, move, rename, required, properties, with typed values from the inspector), and the page `/developer/designer`: palette (a select with an icon per widget), field list, inspector, live preview, Save through the existing route into either scope, a link from the form editor, a conditional menu entry. End to end tested, with the engine vetoing a change it would not accept.
Remaining: nothing. Dropped: controls of their own for details and embeds (DEC-38); the preview's role selector and the inspector rendered by the engine (DEC-39).
Block tree, configurator and live preview, per ADMIN_MODULE_SPEC §8 and
ROADMAP step 5. Starts file-based and independent of the Admin schema
lifecycle; may later graduate into Admin. The largest item in the campaign.

### Phase E: Later

### DEV-22. View-as role
Status: ☑ (2026-10-13) · Depends on: DEV-9 (DEC-10).
Done (2026-10-13): see DEC-25. Chosen from a section of the Overview (a form-engine schema, `view_as`); end to end tested through the real router (a viewer is refused the user administration yet keeps the module, can save a form definition, and leaves cleanly), with the hostile-input, non-developer, module-off and forged-role cases.
The signed session stores the real role plus an optional `view_as` role. A
middleware rewrites the role in the request context to the effective one, so
existing permission checks and `HasAccess` see the lower role unchanged; the
real role sits under a separate context key. Only the Developer module's access
check and a persistent layout banner ("Viewing as <role>. Exit.") read the real
role, so the module never disappears. `view_as` may only select roles at or
below the developer's own, is validated server-side, and is unavailable when
the flag is off. Audit entries record both the real user and the viewed-as role.
User ID stays the developer's.

### Cross-cutting

### DEV-23. Seeds update
Status: ☑ (2026-11-17) · Depends on: DEV-6.
Done (2026-11-17): a manager and a viewer account in each seed (operators were there), all six loaded into a fresh xolu and checked (five roles, one tenant, ID 1); the catalogue describes seeds 05 and 06 and lists the accounts. Each seed loads one company into an empty instance, so there is one tenant per seed by design.
Done (2026-10-13): `dev@ual.li` (developer role) in all six seeds, listed in the catalogue; `make dev` (the Developer and Demo modules on, the override directories under `data/dev/`). The seeds, the Makefile and the demo builder's login parsing were checked against a real xolu: all six load, the seeded developer logs in, and the Developer module leaves every page usable. Remaining: a test account per role per tenant, tenant ID and single-user checks, and the catalogue still describes four datasets (seeds 05 and 06 are not in it).
Review `seeds/seed-01` to `seed-06` and the catalogue. Add a developer-role
user, and a test account per role per tenant (the no-code route in DEC-10).
Check tenant IDs and behaviour on single-user instances.

### DEV-24. Documentation and configuration updates
Status: ☑ (2026-11-17)  · Depends on: each feature as it lands.
Done (2026-11-17): the cross-references in `I18N_SPEC.md`, `ADMIN_MODULE_SPEC.md` and `ROADMAP.md`; `DEPLOY.md` and `config.example.json` carry the new flag and settings.
Updated (2026-10-13): `README.md`, `DEPLOY.md` and `docker-compose.yml` now document the flags, the Developer module, tenant overrides and multi-instance sync. Still open: the cross-references in `I18N_SPEC.md`, `ADMIN_MODULE_SPEC.md` and `ROADMAP.md`.
Done: `config.example.json` now shows every flag (it was missing five), `flags`, `locales_dir` and `schemas_dir`. Remaining: `README.md`, `DEPLOY.md` and `docker-compose.yml` document no flags or `SEAM_*` variables at all today, so a short "Feature flags and the Developer module" section is needed (`SEAM_ENABLE_DEVELOPER`, `SEAM_FEATURES`, `SEAM_PROFILE`, `SEAM_ALLOW_DEV_IN_PRODUCTION`, `SEAM_LOCALES_DIR`, `SEAM_SCHEMAS_DIR`), and cross-references in the specs and `docs/TRACKING.md`. The CHANGELOG entry for 0.4.1-pre-devtools1 is written (it records the strict-parsing behaviour change and the `seam-admin` change).
`config.example.json`, `README.md`, `DEPLOY.md`, `docker-compose.yml`,
`CHANGELOG.md`, cross-references in `I18N_SPEC.md`, `ADMIN_MODULE_SPEC.md` and
`ROADMAP.md`, and a register entry in `docs/TRACKING.md`. Per the handover,
keep personal names and the assistant's name out of all added text.

### DEV-25. Flag consistency test
Status: ☑ (2026-10-10) · Depends on: DEV-3.
Done: `TestExampleConfigDocumentsEveryFlag` in `internal/config` (it failed against the old example and passes now). The README half is not enforced because the README documents no flags (see DEV-24).
A test that every flag in the table appears in `config.example.json` and in the
README.

### Deferred (recorded so they are not forgotten)

### DEV-26. Global-scope editing of overrides
Status: ☑ (2026-11-17) · Depends on: DEV-9, DEV-13, DEV-16.
Done (2026-11-17): see DEC-36. Overrides only; the runtime switches stay per tenant.
Run live with two tenants (2026-11-17): a text and a form saved in the global scope by the home tenant were in force for a second tenant; that tenant's developer was not offered the scope and a post to it changed nothing; its own override won over the global one, for it alone.
Overrides and flags with `tenant_id = 0` that apply to every tenant. Needs a
rule for who may edit them (for example a platform capability or a configured
home tenant) and the layer order already assumed in DEC-4. **Do not drop:** the
first version deliberately cannot do this.

### DEV-27. Runtime toggling of existing modules (Demo, Work Orders)
Status: ☑ (2026-11-17) · Depends on: DEV-9.
Done (2026-11-17): see DEC-36. Run live on two instances.
Routes are registered at mount time and `http.ServeMux` cannot unregister them.
The approach would be to always mount and wrap in a per-request flag check
returning 404; `HasAccess` already reads the flag at call time. Changes existing
modules, so only on request.

### DEV-28. Multi-instance reload
Status: ☑ (2026-10-13) · Depends on: DEV-8, DEV-13, DEV-16.
Done (2026-10-13): see DEC-27. Tested with two whole runtimes over one store acting as two instances (overrides, form definitions and switches arrive and are cleared), and at the store, manager and service levels. Not run against a live xolu (T-106).
The first version assumes one Seam process. For several, poll an `updated_at`
stamp on `feature_state` and the override entities.

### DEV-29. Per-developer preferences
Status: ☑ (2026-11-17) · Depends on: DEV-9.
Done (2026-11-17): see DEC-36.
User-level settings such as a preferred panel tab, kept separate from the
tenant-wide runtime state in DEC-6.

### DEV-30. Developer option in the user role selects
Status: ☑ (2026-10-13) · Depends on: DEV-6.
Done (2026-10-13): see DEC-26.
The create and edit forms (`user_create.json`, `user_edit.json`) list only four roles. Add `developer` to the enums and filter it out of the rendered schema for callers who may not assign it (`auth.CanAssignRole`), preserving key order in the schema JSON (a plain unmarshal and marshal would reorder it). Then drop the "keep a developer's role" rule in `AdminUserUpdate`.

### DEV-31. Tenant-aware translation in the UI render path
Status: ☑ (2026-10-13)  · Depends on: DEV-1.
Done (2026-10-13): page bodies use the tenant's view (`translateFunc(r)`), and the navigation and layout chrome now do too (`Config.ViewerTenantID`, `chromeTranslate`, DEC-18). The error page, which has no request, keeps the global lookup.
Done for page bodies (2026-10-12): `translateFunc(r)` uses the tenant's view when the Developer module is on and the request has a tenant, so the per-tenant switches (filesystem and database overrides, show keys) and tenant overrides reach the page body; with the module off it is the global lookup, as before. Still open: the navigation and layout chrome (`layout.go`, `handler.go`) use the global lookup, so under show-keys the nav keeps its text.
`UIHandler.translateFunc()` takes no request and is called about 140 times; the UI's locale is deployment-global. The Translator's tenant layers and per-tenant options (DEV-11) therefore do not reach rendered pages yet. Global overrides (filesystem, xolu-global) do work, because `T` applies them, but tenant overrides, and the panel's per-tenant `overlay_*` and `show_i18n_keys` switches, only affect code that calls `ForTenant`. Until this is done the panel shows those switches but they do not change page text. The fix is `translateFor(r)` using the tenant in the request's claims, replacing `translateFunc()` call by call; it wants a compile loop.

### DEV-32. Migrate schema call sites to the resolver
Status: ☑ (2026-10-12) · Depends on: DEV-1, DEV-14.
Done (2026-10-12): the 25 production render and parse sites in `internal/ui` (system, masters, issues, sensors, workorders and asset) go through `resolveSchema(ctx, module, name, embedded)`, installed by the server only when the module is on. The Demo module's fixed examples are left alone on purpose, and asset update validation still parses the embedded `SchemaForUpdate`, so an override that adds fields to the asset form is rendered but not validated on update.
About 27 call sites in `internal/ui` change from `xschemas.Schema("n")` to `h.config.DevRuntime.Resolver.Resolve(tenant, "x", "n", xschemas.Schema("n"))` (nil-safe when the module is off). Until then, form-definition overrides load and validate but do not change rendered forms. The `asset` package's `Schema()` and `SchemaForUpdate()` and the masters pages (which use `context.Background()`) need individual handling.

---

## 6. Document history

| Date | Change |
|------|--------|
| 2026-10-10 | Created from the planning discussion: DEC-1 to DEC-11, Q-1 to Q-9, DEV-1 to DEV-29. |
| 2026-10-11 | Version set to 0.4.1-pre-devtools1 with `syncver.sh set`; CHANGELOG entry written; the four docs' `Version:` headers synced by hand (`gorepoman` not installed here); `syncver.sh` and `release.sh` now accept hyphens in the version suffix. |
| 2026-10-10 | DEV-8, DEV-9, DEV-14 to DEV-17, DEV-24, DEV-25 progressed (see items); `devruntime`, `devstate` and `schemaoverlay` added; server wiring written. Found: the translation render path is not tenant-aware (DEV-31); schema call sites are not yet on the resolver (DEV-32). Added DEV-31 and DEV-32. |
| 2026-10-10 | DEV-12 (loader and poller) and DEV-19 (comparison logic) written and tested; real locale files checked clean. |
| 2026-10-10 | DEV-7 written (parse-checked; developer.go compile-checked against a stub). |
| 2026-10-10 | DEV-6 mostly done (auth level verified in the scratch build; UI guards and tests written but uncompiled). Q-2 refined (use `requirePerm`); Q-8 confirmed as a live cross-tenant exposure; DEV-30 added. |
| 2026-10-10 | DEV-3 done; DEV-4, DEV-5 and DEV-11 partly done (see items). Method: pure packages are compiled and tested in an isolated go 1.22 scratch module using the real module path; this does not replace DEV-1. |
| 2026-10-10 | DEV-2 done: Q-1 to Q-7 and Q-9 answered (section 4). Dependencies of DEV-5, 6, 11, 14, 17, 19, 22 relaxed. DEV-6 and DEV-7 expanded with the hard-coded role sites and `requireDeveloper`. DEV-14 redesigned as a resolver taking the base bytes. |
