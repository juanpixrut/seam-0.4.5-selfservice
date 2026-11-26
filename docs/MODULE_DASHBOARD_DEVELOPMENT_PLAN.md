# Module System + Dashboard Invariant — Development Plan

Updated: 2026-08-02

Design intent for the T-44 build (main register, `docs/TRACKING.md`). Frozen
once execution starts — deviations get recorded in `docs/modules/TRACKING.md`,
not silently merged back into this document. Task-level tracking, status, and
progress live in `docs/modules/TRACKING.md` (a separate repoman-managed
register, ID prefix `MOD`, its own `.repoman.json`) — this document is the
stable plan those tasks point back to. Same structure T-01/formengine already
established (`docs/FORMENGINE_DEVELOPMENT_PLAN.md`) — followed deliberately,
not reinvented.

Every citation below is to `MODULE_SPEC.md` v0.2 or `DASHBOARD_SPEC.md` v0.3
unless stated otherwise.

---

## Why this order

Each stage is independently testable and buildable on the last, same
discipline as the formengine plan — no stage requires a later stage's work to
compile or run:

1. **Permission reshape** has to exist before anything can check module
   entitlement — it's the foundation everything else derives access from
   (MODULE_SPEC §2.4).
2. **Dashboard engine core** doesn't depend on the module system at all — it's
   proven against the Assets dashboard migration, a real already-working
   page, before anything module-specific touches it. Same reasoning
   formengine's own plan used for choosing flat master forms over the Asset
   form as the first real proof.
3. **Audit log source + permission-based visibility** needs stage 1's new
   permission format to exist, and needs the actual audit-log wiring
   (DASHBOARD_SPEC §9.3) done as part of the same stage — a data source
   with nothing behind it proves nothing.
4. **Module registry + selector** needs both a real dashboard engine (stage
   2) and the new permission format (stage 1) — a `Module.DashboardID` has
   to point at something real, and `HasAccess` has to derive from something
   real.
5. **System module** is built on the existing Settings/Profile/Administration
   handlers as they are today, routed through the new registry — not
   rebuilt on formengine yet (see stage 8). Proves the registry against the
   most novel entitlement case (unconditional module access, per-view
   gating, one always-open view) before the easier cases.
6. **Assets module** is the thin-wrapper case — lower risk than System since
   it's re-registering ~3,000 already-working lines, not new logic.
7. **Documents module** is the genuinely-empty case, needs stage 2's
   empty-dashboard rendering (DASHBOARD_SPEC §2.2) and stage 1's permission
   format for its placeholder grant.

Stages 8 and 9 are real, deliberately deferred work, not forgotten scope —
same pattern as formengine's own Stage 8 (attachment tray).

---

## Stage 1 — Permission reshape

MODULE_SPEC §2. New `module.view.action` format, wildcard matching
(§2.2–2.3), module-access derivation (§2.4). Migrate every existing
`Permission` constant per §2.5's table, including the two real findings that
migration surfaced: `PermDataImport` is dead code (dropped, not migrated),
and every master-data route's reused `PermSettingsUpdate` gets its own real
permission (`assets.master.*`) instead of carrying the conflation forward.
Every `requirePerm(auth.PermXxx, ...)` call site in `router.go` gets its
constant updated — mechanical, but real, and the actual bulk of this stage's
work (§8.2).

**Load-bearing decision:** `HasPermission`'s external signature doesn't
change — only `rolePermissions`' values and the matching algorithm
underneath. Every call site changes its *constant*, not its own code. This
keeps the migration a pure find-and-replace-with-verification exercise
(matching this project's own established discipline for mass substitution:
classify every occurrence's syntactic role before touching anything, never a
blind pass), not a refactor of calling code.

## Stage 2 — Dashboard engine core

DASHBOARD_SPEC §2–6, §10. The `dashboardengine` package. The five
`dashboard.go`-derived widget types (§4.1–4.5) and the `entity` data source
(§5.1) only — not yet `audit_log` (stage 3) or `line_chart` (stage 9).
Proven against §8's worked example: the existing Assets dashboard, migrated
to a JSON definition, byte-for-byte equivalent behaviour. `dashboard.go`
itself is retired only once this parity is proven, same "coexist until
proven, then retire" discipline the static Asset form went through under
T-01/FE-24.

## Stage 3 — Audit log wiring + `audit_log` source

DASHBOARD_SPEC §5.2, §9.3. Two parts, done together because the second is
meaningless without the first:

1. Add the missing `auditLogger.Log(...)` calls to `AdminUserCreate`/
   `Update`/`PasswordResetSave`, the role-change path, `SettingsSaveGeneral`/
   `Notifications`/`Features`, and `ProfilePreferencesSave` — the last two
   need new `AuditEventType` constants (`AuditSettingsChanged`,
   `AuditPreferencesChanged`) that don't exist yet. Same pass fixes
   `/settings`'s missing permission gate (`requireAuth` only today), using
   stage 1's new `system.settings.edit` permission — one fix, not two trips
   through the same code, per DASHBOARD_SPEC §9.3's own note.
2. `dashboardengine`'s `audit_log` source type (§5.2) and `visible_when:
   {"permission": ...}` (§7.2, using stage 1's new format).

## Stage 4 — Module registry + selector

MODULE_SPEC §3–4. The `Module` struct and registry, wired into
`internal/server/server.go`'s bootstrap in place of today's single direct
call into `internal/ui`. The selector UI — placement confirmed directly:
top-right header, left of the user-profile menu, next to the dark-mode
toggle.

## Stage 5 — System module

MODULE_SPEC §5. `internal/modules/system/`. Three views (Settings, Profile,
Administration) routed through the registry using the *existing* handlers —
not yet rebuilt on formengine (stage 8). System's dashboard (DASHBOARD_
SPEC §9) proven against stage 3's real audit-log data, not empty
placeholders.

## Stage 6 — Assets module

MODULE_SPEC §6. `internal/modules/assets/` as a thin wrapper — registers
`internal/ui`'s existing routes as-is, per §6.1's explicit assessment
against this project's own `/loc` lesson (don't fork off a pattern before
proving the critical integration end-to-end). Assets' dashboard is stage
2's own worked example, already proven — this stage is the registry
wrapper only.

## Stage 7 — Documents module

MODULE_SPEC §7. `internal/modules/documents/`. The `documents.document.view`
placeholder permission (§4.4) added to all four roles. Lands on the
empty-dashboard case (DASHBOARD_SPEC §2.2) — the last real gap this plan
depended on, closed in stage 2 rather than left for this stage to solve
ad hoc.

---

## Stage 8 — formengine rebuild of Account/Settings/Administration (deferred)

Not part of T-44's closure gate, no active timeline. The original question
this whole design conversation started from — "assess whether we can use
formengine to build these views" — was answered (yes, for the form-shaped
parts: profile, preferences, password, general/notifications/features
settings, user create/edit; no, for user *listing* or a permissions/
entitlement matrix, neither of which formengine or any existing pattern
covers). Stage 5 deliberately ships System on the existing hand-coded
handlers so the module system itself doesn't wait on this larger, separate
rebuild. Picked up when there's a real reason to prioritise it, not on a
default schedule.

## Stage 9 — Chart.js + `line_chart` widget (deferred)

DASHBOARD_SPEC §4.6, §1.2's vendoring note. Purely additive — no other
stage depends on it, `internal/reports`' dormant `TimeSeriesPoint` data
stays dormant a while longer with no regression to anything shipped in
stages 1–7. Vendoring steps already concrete (§10): download the UMD build
to `web/static/vendor/`, add it to `VENDOR.md`, wire `//go:embed`.

---

## Deferred/open items carried from the specs, not re-litigated here

- `assets.master.*` per-entity granularity, if it ever turns out too coarse
  (MODULE_SPEC §6.2) — ship coarse first, split later only if a real case
  needs it, same discipline as everything else in this plan.
- A fifth module (CMMS) is explicitly out of scope for this plan (MODULE_
  SPEC §9) — the registry pattern proven in stages 4–7 is what a future
  module follows, not something to design speculatively now.
