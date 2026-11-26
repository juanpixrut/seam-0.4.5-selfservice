# /attic/old-dashboard

Full snapshots of every file retired when the Assets dashboard moved from
hand-written Go rendering (`internal/ui/dashboard.go`) to the declarative
`internal/dashboardengine` (MOD-2, `MODULE_DASHBOARD_DEVELOPMENT_PLAN.md`),
taken immediately before removal, 2026-08-02 — kept as a reference of
what the codebase looked like before, same convention
`attic/static-asset-form/` already established (FE-24), not as a
reactivatable fallback.

## What this is

- `dashboard.go.pre-retirement` — the five `dash*` widget-rendering
  functions (`dashStatCard`, `dashRecentAlerts`, `dashVencimientos`,
  `dashTotalCard`, `dashStatusBreakdown`), `DashboardPage`, and the
  `DashboardData`/`DashAlertRow`/`DashStatusRow`/`DashVencimientoRow`/
  `AlertSummary` types. `EventSummary` (also originally in this file) is
  **not** retired — it's genuinely used outside the old dashboard
  (`asset_detail.go`, `pages.go`, `routes.go`) and moved to
  `asset_detail.go` instead.
- `dashboard_test.go.pre-retirement` — tests for `loadVencimientos`
  (`TestLoadVencimientos_*`, 5 cases). `contains`/`containsHelper`/
  `newTestHandler` (also originally in this file) are **not** retired —
  genuinely shared by other test files, moved to `helpers_test.go`.
  `dateFromNow`/`mockOLUForDashboard` stay retired, dashboard-specific,
  no other callers.
- `loadVencimientos.go.pre-retirement` — extracted separately from
  `internal/ui/masters_routes.go`, which stays (a large, mixed,
  still-active file); only this one function was removed from it.

## Why whole-file / whole-function copies, not surgical diffs

Same reasoning `static-asset-form/README.md` already gives: these
snapshots mix retired-only code with nothing that stays active (unlike
that case, these three pieces genuinely were dashboard-only, confirmed
by a tree-wide reference sweep before removal — `EventSummary`,
`contains`, `containsHelper`, and `newTestHandler` were the only
exceptions found, and they moved rather than retired). Kept as whole
units anyway, matching the existing convention, so anyone looking here
later sees the complete original context without needing to reassemble
it from a diff.

## What replaced this

`internal/dashboardengine` (`engine.go`, `schema.go`, `source.go`,
`compile.go`, `cache.go`, `widgets.go`, `renderer.go`) plus
`internal/ui/dashboard_resolver.go` (the real xolu-backed
`EntityResolver`) and `internal/dashboardengine/schemas/
assets-dashboard.json` (the declarative definition, using current field
names — `criticidad`, `fecha_expiracion`, `fin_garantia`, since T-40's
Asset rename is still paused).

Proven equivalent before removal, not assumed: live-verified twice
against a real xolu instance with seeded data (real stat counts, real
`enrich`-derived alert rows, real threshold coloring, real i18n) —
first before this retirement, then again immediately after, confirming
identical output. Two real bugs were found and fixed during that live
verification (`badge_colors` language mismatch against actual stored
values, missing `DateFieldLabels` mapping for the urgency-list i18n
keys) — see `CHANGELOG.md` [0.3.36] for the full account.
