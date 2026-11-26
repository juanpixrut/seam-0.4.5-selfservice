# Seam AMS Roadmap

**Last updated:** May 2026 (metadata corrected 2026-08-04; content below not yet refreshed — see notice)
**Current version at last content update:** 0.1.0-patched243

---

> **This document is a historical planning snapshot, not current status.**
> Everything below this notice reflects planning as of May 2026
> (0.1.0-patched243). Seam AMS is now at v0.3.55 (2026-08-04) — three
> months and dozens of releases later, most of which aren't reflected
> here. Most of "Near-term — Implementation" below (the form engine,
> the module architecture, i18n, Documents) has since been substantially
> built, in some cases quite differently than planned here (e.g. the
> "Admin module" section below describes one combined module; what
> actually shipped is three separate modules — Account, Settings,
> Administration — see T-02 in `docs/RESOLVED.md`).
>
> For accurate, current information, use these instead:
> - **`CHANGELOG.md`** — what has actually shipped, release by release
> - **`docs/TRACKING.md`** — open work, correctly prioritised
> - **`docs/RESOLVED.md`** — closed work, with the real reasoning behind each decision
>
> A full regeneration of this document against current reality is real,
> substantial work of its own (cross-referencing every line below
> against ~65 resolved tracking items and dozens of releases) — not
> attempted as a quick fix alongside noticing the staleness, since a
> rushed rewrite risks introducing fresh inaccuracies of exactly the
> same kind. Tracked as T-66.

---

## Completed

### olu client remediation (patched1–patched7)

| Version | Summary |
|---|---|
| patched1 | Fixed silent data-loss in all CRUD responses; unified `OQLResult` |
| patched2 | `WithAPIKey`, `Authorization` header; search endpoint corrected |
| patched3 | All 3 graph endpoints fixed; `WithTenantID`; `Sulpher` → `GraphQuery` |
| patched4 | `queryfy` v0.2.0 → v0.3.1 |
| patched5 | `Save` and `Commit` added to client |
| patched6 | `internal/olu` at 85.1% coverage; all client tests rewritten |
| patched7 | Regression tests: alerts, connectivity, QR handler |

### Feature and hardening work (patched8–patched228)

| Version | Summary |
|---|---|
| patched8 | Fixed nil panic in `processEvent`; `evaluateFSM` uses `Commit` atomically |
| patched9 | Login/Logout CSRF fix; CSRF token propagated to all auth pages |
| patched10 | Asset Type Editor: 7 new handlers, pages, routes |
| patched11 | `docs/FSM_DESIGN.md` v1.0 |
| patched12 | FSM design updated (actions, parallel machines, vending machine example) |
| patched13 | Guard expression language: `tsqlparser` via `SELECT @FSM WHERE` wrapper |
| patched14 | `tsqlparser` decision documented; CEL comparison table |
| patched15 | OQL-as-action-executor architecture documented |
| patched16 | `Makefile` and `scripts/launch.sh`/`scripts/bundle.sh` added |
| patched17 | Docs review and sync: README, DEPLOY, ROADMAP |
| patched17–patched228 | Wave 2 asset form, masters suite, sensors, events, alerts, QR, map, SeamModal2, listing engine, i18n foundation, RBAC infrastructure |

### Architecture design (patched229–patched243)

| Document | Version | Description |
|---|---|---|
| `docs/FORM_ENGINE_SPEC.md` | 0.3 | Dynamic form engine — JSON Schema extensions, row layout, conditional visibility, access control, tray permissions (four levels), widget catalogue. All six §15 pending decisions resolved. |
| `docs/I18N_SPEC.md` | 0.1 | Internationalisation system — locale packages, RTL, forms nomenclature, hot reload |
| `docs/ADMIN_MODULE_SPEC.md` | 0.1 | Admin module — user management, permissions, i18n manager, schema lifecycle, form designer |
| `docs/DOCUMENTS_MODULE_SPEC.md` | 0.5 | Document Management module — entity-agnostic attachments, four-level tray permissions, file viewer (all types), composite search scoring, tags |
| `docs/SEAM_MEDIA_SPEC.md` | 0.3 | Seam Media subsystem — fijuggler, seam-filer, thumble, seam-indexer, file_record lifecycle, WAL, filesystem hierarchy, S3 directory simulation, storage migration model |

### olu client — xolu API parity (patched232–patched236)

| Version | Summary |
|---|---|
| patched232 | Fixed error response parsing (nested object format); removed dead `GetRelated`/`RelatedAsset` code; `Error.Code`, `IsConflict()`, `CurrentVersion()` |
| patched233 | `CommitTSEvent`, `Timeseries` field in `CommitRequest`, `TSAccepted` in `CommitResult` |
| patched234 | All previously unsupported endpoints: OQL async, Sulpher async, graph topology (12 methods), Schema, Export — 35 methods total |
| patched235 | Complete Pebble timeseries API (13 methods) — 48 methods total, full xolu v0.9.7 coverage |
| patched236 | Test coverage 82.9% → 97.4%; `client_test.go` rewritten (63 → 110+ tests); `client_integration_test.go` added (build tag: integration) |

### Seam Media stub (patched241)

| Item | Description |
|---|---|
| `internal/media/media.go` | `media.Client` interface + `NewStub()` + `NewHTTPClient()` scaffold. Seam App compiles and runs without seam-media deployed. |

---

## Near-term — Implementation

### 1. Dynamic form engine (`internal/formengine`)

Spec: `docs/FORM_ENGINE_SPEC.md` v0.3. All §15 decisions resolved — implementation can begin.

Staged build plan: `docs/FORMENGINE_DEVELOPMENT_PLAN.md` (frozen, 8 stages,
stage 8 deferred). Task-level tracking: `docs/formengine/TRACKING.md` (own
repoman register, ID prefix `FE`). The table below is retained for a quick
scan but is no longer the source of truth — the plan doc is.

| Step | Scope | Notes |
|------|-------|-------|
| 1 | `FormEngine` struct, `RenderContext`, widget renderer | Start with flat forms, no tabs |
| 2 | Flat master forms (Región, Moneda, Marca) | First forms converted to schema-driven |
| 3 | Tab support + row layout; §15.1 schema validation (reject undefined section_defs) | Required for Asset form |
| 4 | `detail` panels and `embed` sub-forms | Sensor binding, custody history |
| 5 | `x-seam-visible-when` + §15.2 merge-conservative hidden field policy | Client JS evaluator + server queryfy |
| 6 | `FieldPermission` + `PermService` + §15.3 role-priority resolution | RBAC on form fields |
| 7 | `ModalTriggerAttrs` per §15.4; Asset form fully converted | Parity with current static implementation — completion gate for T-01 |
| 8 | Attachment tray — `attachable: true`, `TrayPermissions`, four-level access control | Requires Document Manager module — deferred, not part of T-01's closure |

### 2. Document Management module (`modules/documents`)

Spec: `docs/DOCUMENTS_MODULE_SPEC.md` v0.5. Depends on form engine step 8.

| Step | Scope |
|------|-------|
| 1 | `document_type` master table in `masters` module |
| 2 | `document` entity CRUD — title, description, tags, dates, notes (Markdown), status |
| 3 | `attachment` entity — entity-agnostic (entity_type + entity_id + section_id) |
| 4 | Document list view with composite search scoring (§5.6) + tag autocomplete |
| 5 | Document detail view with file viewer (§5.5) — all four media type branches |
| 6 | Attachment tray htmx endpoint + pill rendering |
| 7 | Integration with `media.Client` — file metadata, thumble_asset_id resolution |

### 3. Internationalisation (`internal/i18n` refactor)

Spec: `docs/I18N_SPEC.md`.

| Step | Scope |
|------|-------|
| 1 | `LocalePackage` + `LocaleMeta` structs, hierarchical `Translator` |
| 2 | Convert `en.json`, `es.json`, `pt.json` to nested format |
| 3 | `forms` subsection for all existing forms |
| 4 | Hot-reload via olu |
| 5 | Admin UI — locale editor with completeness tracking |

The Developer module (`docs/DEVELOPER_MODULE_CAMPAIGN.md`) already provides, for developers, a text editor with per-tenant, global and file overrides, version history, hot loading, a completeness view per language and a list of texts the source does not refer to (campaign DEV-11 to DEV-13, DEV-18, DEV-19). Steps 4 and 5 here are the same capabilities for administrators, and should build on that code rather than beside it.

### 4. Admin module (`modules/admin`)

Spec: `docs/ADMIN_MODULE_SPEC.md`. §15.5 schema lifecycle policy is a hard prerequisite before first production schema.

| Step | Scope | Depends on |
|------|-------|------------|
| 1 | Extract `/admin/users/*`, `/admin/settings` to `admin` module | — |
| 2 | Permission Manager UI — `FieldPermission` editor, role priority view | formengine §3c |
| 3 | i18n Manager UI | i18n refactor |
| 4 | Schema Lifecycle — draft/publish/rollback, §15.5 compatibility checker | queryfy, olu |
| 5 | Form Designer — block tree, configurator, live preview | All above |
| 6 | Extract FSM, asset-types, propiedades to `admin` | Step 5 |

The Developer module has a form designer (fields and the block tree: palette, inspector, tabs, sections, rows, drag and drop, duplication, live preview in any language, save into any override scope; campaign DEV-21, DEC-29, DEC-37, DEC-39) and the override layers, validation gate and version history a schema lifecycle needs (DEV-14 to DEV-17, DEC-32 to DEC-34). Step 5's block tree exists there, with details and embeds edited as JSON lists; steps 4 and 5 should start from it.

### 5. Module architecture

| Step | Scope |
|------|-------|
| 1 | Module registration interface in `core` |
| 2 | Extract master routes to `masters` module |
| 3 | Extract admin routes to `admin` module |
| 4 | Launcher — module-aware nav, role-based module visibility |
| 5 | Menu merging across modules (design TBD) |

---

## Near-term — External projects (parallel development)

These are developed independently and integrate with Seam App via `media.Client`.

### seam-filer (new repo: github.com/ha1tch/seam-filer)

Spec: `docs/SEAM_MEDIA_SPEC.md` §3.

| Step | Scope |
|------|-------|
| 1 | Ten-endpoint fijuggler REST contract — local FS backend |
| 2 | `file_record` entity in xolu — creation on upload, WAL for resilience |
| 3 | Thumble integration — auto-registration, eager rendering (§3.5) |
| 4 | seam-indexer job enqueue on upload |
| 5 | Tenant routing via `X-Seam-Tenant` header; well-known dirs (Inbox/, Trash/) |
| 6 | S3 backend — directory simulation, object prefix operations |
| 7 | `GET /api/info` — reads file_record from xolu |
| 8 | Storage location migration protocol (hot → cold) |

### thumble extensions (github.com/ha1tch/thumble)

Spec: `docs/SEAM_MEDIA_SPEC.md` §4.2.

| Extension | Description |
|---|---|
| `page_count` + `duration` on registration | pdfinfo / ffprobe on POST /assets |
| Batch job endpoint | POST /assets/{id}/jobs/batch + GET /batch-jobs/{id} |
| `audioclip` job type | 30s mono OGG, fade in/out, via ffmpeg |
| `frame` job type | Video frame at timestamp, via ffmpeg |
| Eager config | `eager_max_pages`, `eager_max_bytes`, `eager_max_duration` |

### seam-indexer (new repo: github.com/ha1tch/seam-indexer)

Spec: `docs/SEAM_MEDIA_SPEC.md` §12.

| Step | Scope |
|------|-------|
| 1 | `index_job` polling loop — consume jobs from xolu |
| 2 | Text extractors — pdftotext, soffice, pandoc, direct read |
| 3 | `file_index` entity in xolu — write full_text per fijuggler_ref |
| 4 | Update `file_record.IndexState` on completion |
| 5 | `summarise` job type stub (queue infrastructure ready, worker deferred) |

### seam-media integration into Seam App

When seam-filer reaches step 7:

| Step | Scope |
|------|-------|
| 1 | Implement `media.NewHTTPClient()` against seam-filer GET /api/info |
| 2 | Add `/files` route and `/api/files/*` proxy |
| 3 | Vendor fijuggler.min.js + fj-tailwind-dark.css |
| 4 | Wire `media.Client` into Document Manager link flow |

---

## Existing near-term items

| Feature | Priority |
|---|---|
| FSM guard evaluator (tsqlparser AST walker) | High |
| FSM action executor (OQL-as-executor) | High |
| Rule Builder UI | Medium |
| Permission Guards — enforce RBAC on UI routes | Medium |
| `fsm-toolkit` `guard`/`actions` fields on `Transition` | Blocked on upstream |

---

## RBAC

Foundation in place. Role priority model resolved (§15.3): admin=100, seam_consultant=90, manager=50, operator=30, viewer=10. Form-level permissions fully specified in `docs/FORM_ENGINE_SPEC.md` §3c including four-level tray permissions.

| Role | Priority | Description |
|---|---|---|
| `admin` | 100 | Full system access |
| `seam_consultant` | 90 | Cross-tenant read, demo and onboarding access |
| `manager` | 50 | Tenant management — assets, rules, settings, export |
| `operator` | 30 | Day-to-day — create/read/update assets, read rules |
| `viewer` | 10 | Read-only |

---

## Technical debt

| Item | Notes |
|---|---|
| `internal/server` — no tests | Deferred |
| `internal/ui` — no tests | Reduces as static forms migrate to `formengine` |
| `internal/auth` — 56% coverage | Auth hardening pass deferred |
| CSRF enforcement | Foundation in place; enforcement incomplete |
| Structured logging | Currently ad-hoc |
| Prometheus metrics | `/metrics` not implemented |
| FSM `guard`/`actions` — `::` stopgap | Replace once fsm-toolkit adds `guard` field |
| Static forms in `internal/ui` | Migrate progressively to `formengine` |
| Flat i18n keys in `internal/i18n/locales/` | Convert to nested module/forms format |
| fijuggler i18n | ~30 hardcoded English strings; `prompt()`/`confirm()` replacements needed; estimated half-day; deferred post-MVP |

---

## Future

| Area | Items |
|---|---|
| Storage lifecycle | Cold storage migration — mark file groups for archival; StorageLocation tier transitions; automated retention policies per tenant |
| seam-indexer LLM pipeline | `summarise` job type; `ai_summary` field on `file_index`; fifth search signal in composite scoring |
| seam-media consolidation | `seam-media` repo unifying seam-filer + fijuggler vendor + unified TOML + docker-compose stack |
| IoT integrations | MQTT broker, LoRaWAN network server |
| Notifications | Slack, Microsoft Teams, outbox worker |
| Advanced features | Scheduled reports, custom dashboards, multi-tenant admin, API key management UI, webhook signature verification |
| Batch operations | Bulk asset import/export, CSV upload wizard |
| `asset-picker` widget | Hierarchical asset selector for `id_padre` and `usa` relations |
| `SeamTooltip` | Floating tooltip component for `x-seam-help-style: tooltip` |
| Post-MVP locales | Arabic (RTL), Hebrew (RTL), French, German, Chinese, Japanese, Turkish, Russian, Hindi |
| Tenant locale personalisation | `{locale}-{tenantID}` override packages |
| Schema versioning | Store schema version with record; render historic forms against their original schema (§15.5 policy adopted) |
| fijuggler S3 provider | Currently local FS only; S3 provider in fijuggler would allow direct browser-to-S3 uploads bypassing seam-filer for large files |
| slabbis native olu persistence | Load from olu on startup, dump on change, periodic sync; write-through vs write-behind as config option. Motivation: Seam AMS device registry and RULES distribution. |
| CMMS-like module | First draft received 2026-07-28 (the "AMS Orders" v1.0 — Order Engine: OrderType/OrderModel/Order, configurable Header/Blocks, Workflow, Numbering, Business Rules, Actions). Integration analysis in `docs/ORDER_ENGINE_INTEGRATION_NOTES.md`; scoping tracked as T-11. Likely candidate for xolu's `/cal` primitive (stable, v0.16.16) for maintenance scheduling, and `/fsm`/`/gen`/event-defs for Workflow/Numbering/Actions. |
| fsm-toolkit as design-time tooling | Possible future import of `github.com/ha1tch/fsm-toolkit` for its graphical/TUI FSM designer — not as a runtime (xolu's native `/fsm` is the runtime, decided 2026-07-28, see T-12 in `docs/RESOLVED.md`). Explicitly not decided or urgent. |
