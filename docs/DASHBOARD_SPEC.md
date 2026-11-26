# Seam AMS — Declarative Dashboard
## Design Specification

**Version:** 0.6
**Date:** August 2026
**Status:** Internal draft
**Repository:** github.com/ha1tch/seam-ui (core shared across modules)

---

## 1. Purpose and Context

The Dashboard is the landing invariant for every module in the selector (Apps:
Assets, Documents, future CMMS; System, with its own variant — see §9). This
specification defines a declarative JSON format for dashboards, in the same spirit
as `x-seam-form` (FORM_ENGINE_SPEC.md) did for entity forms: replace hardcoded
Go composition with a stored, versionable definition rendered by a generic engine.

### 1.1 Motivation

The dashboard that exists today (`internal/ui/dashboard.go`) — the Assets one — is
entirely hardcoded: every widget is a Go function, every data source is an OQL
query written directly into the handler (`routes.go`'s `Dashboard`), and none of
it is reusable for another module. With the module selector (Assets, Documents,
System, future CMMS) each module needs its own dashboard, and System needs one
whose composition depends on the permissions of whoever is looking at it —
hand-building each one in Go would multiply exactly the problem `x-seam-form`
already solved for forms.

### 1.2 Supporting Infrastructure

| Component | Role |
|------------|-----|
| **xolu OQL** (`pkg/client`) | Data source for `entity`-type widgets — `SELECT`/`COUNT`/`GROUP BY` queries already used today in `dashboard.go`/`routes.go`, now generated from the declarative definition instead of hand-written. |
| **`internal/auth` audit log** (`AuditEvent`/`AuditStore`/`XOLUAuditStore`, xolu entity `audit_logs`) | Data source for `audit_log`-type widgets — already exists and is persisted, but today is only wired into four authentication handlers (`LoginSubmit`, `Logout`, `ForgotPasswordSubmit`, `ResetPasswordSubmit`). The event types for administration (`AuditUserCreated`, `AuditUserUpdated`, `AuditRoleChanged`, etc.) are already defined in the enum but **not connected** to the `AdminUser*` handlers; there's likewise no connection for Settings changes or user preference changes. This is a real prerequisite for the System sections to have data, not an optional enhancement — see §9.3. |
| **`internal/condition`** | Condition evaluator already used by `x-seam-visible-when` in forms. Reused for conditional widget visibility (§7), extended with a new permission-based condition type (§7.2), not just field values. |
| **minty** | HTML rendering of the dashboard from the compiled definition — same mechanism as the form engine. |
| **Chart.js** (to be vendored) | Not currently in `web/static/vendor/` — needs adding, same no-CDN policy as every other frontend dependency (see `VENDOR.md`: htmx, leaflet, lit, autoComplete.js are all vendored and `//go:embed`-ed today, zero runtime CDN requests). Required for `line_chart` (§4.6) — see §1.3 for why this is now evidenced, not speculative. |

### 1.3 MVP Scope

The MVP covers the five widget types `dashboard.go` already implements today
(catalogued empirically by reading the existing code, not designed in the
abstract — see §4.1–4.5), plus the `audit_log` data-source type System needs,
plus one chart type — `line_chart` (§4.6) — added in this revision on real
evidence, not speculatively: `internal/reports/reports.go` already computes
`[]TimeSeriesPoint` data (`getAssetsCreatedOverTime`, `getEventsOverTime`) and
exposes it via `internal/handlers/api/reports.go`, but **no UI anywhere renders
it** — it's fully computed, API-accessible, and unvisualized. That's a real,
already-built need, not a hypothetical one. The Assets dashboard migrates to
this format as proof that the spec covers a real, complete case (§8); the
System dashboard is built new on top of the same engine (§9).

Out of MVP scope:

- Bar, pie, doughnut, and gauge charts — no existing code produces data shaped
  for any of these today. `line_chart` is in scope because `TimeSeriesPoint`
  data already exists; the others would be speculative additions until a
  real case appears (see §11's own stated discipline on this point — it
  applies here too, not just to entirely new widget types).
- Per-user dashboard customization (reordering/hiding individual widgets)
- Third-party or externally embedded widgets
- Real-time refresh (websockets/SSE) — the dashboard renders on each page load,
  same as today, and same as `reports.go`'s own request-time computation

---

## 2. General Structure

A dashboard is defined as a JSON document with a root `x-seam-dashboard` property,
the same way a form is defined under `x-seam-form`.

```json
{
  "x-seam-dashboard": {
    "id": "assets-dashboard",
    "title": "nav.dashboard",
    "layout": "stack",
    "sections": [ /* see §3 */ ]
  }
}
```

### 2.1 `x-seam-dashboard` Properties

| Property | Type | Required | Description |
|-----------|------|:---:|-------------|
| `id` | string | yes | Unique dashboard identifier (one per module today: `assets-dashboard`, `system-dashboard`; future `documents-dashboard`). |
| `title` | string | yes | i18n key for the page title (`t(title)`). |
| `layout` | string | yes | `"stack"` (sections stacked vertically, each full-width or in its own internal grid — the only layout `dashboard.go` uses today). Other values are reserved for when a real case needs them, not speculatively specified. |
| `sections` | array | yes | Ordered list of sections (§3). Rendered in the given order, subject to `visible_when` (§7). May be an empty array — see §2.2. |

### 2.2 Empty Dashboards

A dashboard with zero sections (`"sections": []`) — Documents' dashboard
today, since it has no widgets defined at all (MODULE_SPEC.md §7) — renders
neither the stack layout nor any widget machinery. Instead: centred text,
two elements, both i18n keys (per this project's own governing rule — see
`docs/KNOWN_ISSUES.md`'s internal/external language law — nothing rendered
to a user is ever a literal string):

1. The module's own display name — `t(module.Label)`, the same i18n key
   the selector itself uses (MODULE_SPEC.md §3.1's `Label` field), not a
   separate key invented for this case.
2. A fixed message, `t("dashboard.empty_message")` — "No data to display in
   this module at this time."

This is distinct from an individual *widget's* empty state (§4.2's
`empty_icon`/`empty_message`, e.g. "no open alerts") — that's a normal
widget with no matching rows; this is a dashboard with no widgets defined
at all. A dashboard can have all its sections present but every widget
individually empty (normal, handled per-widget); it can also have zero
sections at all (this case). The two are not conflated: `sections: []` is
the only trigger for §2.2's behaviour, not "every section under `visible_
when` happened to evaluate false for this viewer" (which, for System in
particular, can never actually happen — Profile's section has no `visible_
when` at all, so System's dashboard is never empty in this sense for any
authenticated user).

---

## 3. Sections

A **section** is a dashboard row: one or more widgets arranged in a grid.
Corresponds exactly to the blocks `DashboardPage` already composes today — the
row of three stat cards, the recent-alerts panel, the upcoming-expirations panel,
the total+breakdown row.

```json
{
  "id": "top-stats",
  "grid": "3col",
  "widgets": [ /* see §4 */ ]
}
```

| Property | Type | Required | Description |
|-----------|------|:---:|-------------|
| `id` | string | yes | Unique identifier within the dashboard. |
| `grid` | string | yes | `"1col"` \| `"2col"` \| `"3col"` — number of columns in the section's CSS grid. `dashboard.go` uses `3col` for the stat cards and `2col` for the total+breakdown row; the list panels (recent alerts, upcoming expirations) are single-widget `1col` sections. |
| `widgets` | array | yes | List of widgets (§4) filling the grid, in order. |
| `visible_when` | object \| null | no | Visibility condition for the whole section (§7). If `null` or absent, the section always shows (subject to its individual widgets' own empty states). |

---

## 4. Widget Types

Six widget types: five catalogued directly from `dashboard.go` — none
speculative, each corresponding to an existing Go function today
(`dashStatCard`, `dashRecentAlerts`, `dashVencimientos`, `dashTotalCard`,
`dashStatusBreakdown`) — plus one (§4.6) catalogued from
`internal/reports/reports.go`'s existing, currently-unvisualized time-series
data.

### 4.1 `stat_card`

Corresponds to `dashStatCard`. A large number with an icon, conditionally
recolored when it's "urgent" and non-zero.

```json
{
  "type": "stat_card",
  "label": "dashboard.open_alerts",
  "icon": "crisis_alert",
  "href": "/alerts?status=open",
  "urgent": true,
  "source": { "type": "entity", "entity": "alerts", "aggregate": "count", "filter": "status = 'open'" }
}
```

| Property | Type | Description |
|-----------|------|-------------|
| `label` | string | i18n key. |
| `icon` | string | Material Icons name. |
| `href` | string | Destination when the whole card is clicked. |
| `urgent` | bool | If `true`, when the resolved value is > 0 the card recolors to red (border, icon, number) — same behaviour as `dashStatCard`'s `urgent` parameter. |
| `source` | object | Data source (§5). Must resolve to a single number (`aggregate: "count"` or another scalar). |

### 4.2 `recent_list`

Corresponds to `dashRecentAlerts`, generalized. Panel with a title, an optional
"view all" link, and a list of rows with icon + primary text + secondary text +
colored badge + trailing timestamp. This is the widget System reuses for its
three "recent changes" sections (§9), with `source.type: "audit_log"` instead of
`"entity"` — see §5.2.

```json
{
  "type": "recent_list",
  "title": "dashboard.recent_alerts",
  "view_all_href": "/alerts?status=open",
  "item_href_prefix": "/alerts",
  "empty_icon": "check_circle",
  "empty_message": "dashboard.no_open_alerts",
  "item_icon": "warning",
  "primary_field": "asset_name",
  "secondary_field": "title",
  "secondary_fallback_field": "alert_type",
  "timestamp_field": "created_at",
  "badge_field": "criticality",
  "badge_colors": { "High": "danger", "Medium": "warning", "Low": "success" },
  "source": { "type": "entity", "entity": "alerts", "filter": "status = 'open'", "sort": "-created_at", "limit": 5 }
}
```

| Property | Type | Description |
|-----------|------|-------------|
| `title` | string | i18n key for the header. |
| `view_all_href` | string \| null | If present, shows the "view all" link next to the title. |
| `item_href_prefix` | string | Each row's own link is `item_href_prefix + "/" + id` — deliberately **not** `view_all_href`, added in v0.5 after catching a real bug: `dashRecentAlerts`'s header links to `/alerts?status=open` but each row links to `/alerts/{id}`, two different base paths. An early implementation pass reused `view_all_href` for both, which produces `/alerts?status=open/5` — caught by inspection before it shipped, not found later. |
| `empty_icon` / `empty_message` | string | Icon and i18n key shown when `source` returns no rows — same pattern as the current empty state (`check_circle` + "no open alerts"). |
| `item_icon` | string | Fixed icon for every row (`dashboard.go` uses one fixed icon per widget today, not per row — there's no real per-row conditional icon to migrate). |
| `primary_field` | string | Row field shown as each item's primary (bold) text. Added in v0.4, found necessary implementing this widget generically: the pre-migration code hardcoded `AssetName` here, which doesn't generalise to System's audit-log rows (§9), which have no such field. |
| `secondary_field` / `secondary_fallback_field` | string | Row field shown as secondary text; if empty and `secondary_fallback_field` is set, that field's value is used instead. Reproduces the pre-migration code's own `if title == "" { title = a.AlertaTipo }` fallback exactly, now declarative. `secondary_fallback_field` is optional — omit it for a widget with no fallback case. |
| `timestamp_field` | string | Row field shown as the trailing timestamp. |
| `badge_field` | string | Row field, resolved by `source`, whose value colors the badge. |
| `badge_colors` | object | Map of `value → color class` (`danger`/`warning`/`success`/`default`), same map as today's `criticidadClass`. |
| `source` | object | Data source (§5), must resolve to a list of rows. |

### 4.3 `urgency_list`

Corresponds to `dashVencimientos`. Similar to `recent_list`, but each row's
trailing label isn't a fixed color badge — it's computed from days-remaining
thresholds.

```json
{
  "type": "urgency_list",
  "title": "dashboard.upcoming_expirations",
  "subtitle": "dashboard.next_30_days",
  "empty_message": "dashboard.no_upcoming_expirations",
  "item_icon": "event",
  "item_href_prefix": "/assets",
  "thresholds": [
    { "max_days": 7,  "class": "urgent", "label": "dashboard.days_urgent" },
    { "max_days": 14, "class": "warning", "label": "dashboard.days" },
    { "max_days": null, "class": "normal", "label": "dashboard.days" }
  ],
  "source": { "type": "entity", "entity": "assets", "date_fields": ["expiration_date", "warranty_end"],
    "date_field_labels": { "expiration_date": "expiration", "warranty_end": "warranty_end" },
    "within_days": 30, "limit": 10 }
}
```

`thresholds` is evaluated in order; the first entry whose `max_days` is `null` or
greater than or equal to the row's remaining days wins. This reproduces exactly
today's hardcoded rule (`≤7 days` red/bold, `≤14 days` yellow, everything else
default). `date_fields` is two fields, not three — corrected in v0.5:
`loadVencimientos` (the code this widget migrates) only ever checks
`fecha_expiracion`/`expiration_date` and `fin_garantia`/`warranty_end`; an
earlier draft of this example listed a third (`loan_expiry`) that was never
actually part of it, caught cross-referencing the real function rather than
trusted from an earlier pass. `item_href_prefix` (§4.2) applies here too —
each row links to `item_href_prefix + "/" + entity_id`, matching
`dashVencimientos`'s own hardcoded `/assets/%d`. Row field names (`date`,
`field_name`, `entity_id`, `entity_name`) are fixed by `UrgencyRow`'s own
shape, not configurable per widget — an earlier draft of this spec listed
`date_field`/`label_field` as widget properties for this; removed in v0.5,
unused by any actual rendering code once implemented.

### 4.4 `total_card`

Corresponds to `dashTotalCard`. A large single-number card with a link, no
urgency logic (unlike `stat_card`).

```json
{
  "type": "total_card",
  "label": "dashboard.total_assets",
  "icon": "inventory_2",
  "href": "/assets",
  "view_link_label": "dashboard.view_list",
  "source": { "type": "entity", "entity": "assets", "aggregate": "count" }
}
```

### 4.5 `breakdown_list`

Corresponds to `dashStatusBreakdown`. A list of colored-dot + label + count,
grouped by a field.

```json
{
  "type": "breakdown_list",
  "title": "dashboard.by_status",
  "empty_message": "dashboard.no_status_data",
  "labels": { "active": "dashboard.status.active", "inactive": "dashboard.status.inactive", "maintenance": "dashboard.status.maintenance", "lost": "dashboard.status.lost" },
  "colors": { "active": "green-500", "inactive": "gray-400", "maintenance": "yellow-500", "lost": "red-500" },
  "source": { "type": "entity", "entity": "assets", "aggregate": "group_count", "group_by": "status" }
}
```

`colors`' values are the exact Tailwind `bg-{value}` suffix to use (`"green-500"`, not a bare `"green"`) — preserved directly from `dashStatusBreakdown`'s own hardcoded map, including its one inconsistency (`gray-400`, not `gray-500`, for `inactive`), rather than normalised away during migration.

### 4.6 `line_chart`

New in this revision — no direct `dashboard.go` equivalent, but a direct
declarative equivalent of `internal/reports/reports.go`'s
`getAssetsCreatedOverTime`/`getEventsOverTime`, which compute this exact shape
today and expose it via `internal/handlers/api/reports.go` with no UI consumer.
Rendered with Chart.js (vendored — see §1.2).

```json
{
  "type": "line_chart",
  "title": "dashboard.assets_created_over_time",
  "color": "indigo",
  "source": { "type": "entity", "entity": "assets", "aggregate": "time_series", "date_field": "created_at", "bucket": "day", "within_days": 30 }
}
```

| Property | Type | Description |
|-----------|------|-------------|
| `title` | string | i18n key for the chart's heading. |
| `color` | string | Named color for the line (`indigo`/`green`/`red`/etc. — same palette convention as `breakdown_list`'s `colors`, not raw hex, so the chart stays theme-consistent). |
| `source` | object | Data source (§5.1) with `aggregate: "time_series"`. Must resolve to a list of `{ timestamp, value, label? }` points — the same shape `TimeSeriesPoint` already has, deliberately, so a future Go implementation can serialize `reports.Service`'s existing return types directly without a translation layer. |

One deliberate simplification versus the existing code: `getEventsOverTime`
picks its bucket granularity adaptively (hourly for a same-day range, 6-hourly
for a week, daily beyond that). The declarative form takes an explicit
`bucket` instead (§5.1) — a widget's granularity should be readable from its
definition without running it, and no evidenced case today needs the adaptive
behaviour in a dashboard context specifically (`reports.go`'s adaptive logic
serves its own report-generation use, not this dashboard widget).

---

## 5. Data Source (`source`)

Every widget resolves its data from exactly one `source`. Two source types cover
every real case identified: `entity` (any xolu entity via OQL) and `audit_log`
(the `internal/auth` audit store, with its own fixed shape, not a generic
business entity).

### 5.1 `source.type: "entity"`

| Property | Required | Description |
|-----------|:---:|-------------|
| `entity` | yes | xolu entity name (`assets`, `alerts`, etc. — already English on every entity migrated by T-40). |
| `aggregate` | no | `"count"` \| `"group_count"` \| `"time_series"`. Absent = return rows (for `recent_list`/`urgency_list`). |
| `group_by` | conditional | Required if `aggregate: "group_count"`. Field to group by. |
| `date_field` | conditional | Required if `aggregate: "time_series"`. The entity's date/timestamp field to bucket by (e.g. `created_at`) — same field `getAssetsCreatedOverTime`/`getEventsOverTime` already query against. |
| `bucket` | conditional | Required if `aggregate: "time_series"`. `"hour"` \| `"day"` \| `"week"` \| `"month"` — fixed granularity, explicit in the definition rather than adaptive (see §4.6's own note on why). |
| `filter` | no | `WHERE` clause in literal OQL — same pattern as today's hardcoded queries (`"status = 'open'"`). No structured filter DSL is introduced in the MVP: there's no evidence today that filters need to be more than a direct OQL clause, and the form engine already established the precedent of accepting literal OQL in `x-seam-source` for the analogous case. |
| `sort` | no | Sort field, `-` prefix for descending. |
| `limit` | no | Maximum row count. |
| `date_fields` / `within_days` | conditional | Exclusive to `urgency_list`: list of date fields to evaluate and the forward-looking day window. `within_days` is also used by `aggregate: "time_series"` to bound the bucketed range (singular `date_field`, not the plural `date_fields` — `urgency_list` checks several date fields per row for whichever is soonest; a time-series chart buckets by exactly one). |
| `date_field_labels` | no | Exclusive to `urgency_list`, added in v0.6. Maps a `date_fields` entry to a stable semantic label used for its i18n key (`"dashboard.field."+label`), decoupled from the entity's own raw field name. Found necessary during MOD-2's live verification, not designed upfront: the pre-migration code labelled its two date checks `"expiration"`/`"warranty_end"` — already translated in every locale file — never the raw xolu column names (`fecha_expiracion`/`fin_garantia`) `assets-dashboard.json` actually queries today. Using the raw field name directly as the i18n key would work today but churns again the moment T-40's still-paused Asset rename lands and those columns become English; this mapping is what avoids that. A date field absent from this map falls back to its own raw name as the label. |
| `enrich` | no | List of secondary lookups merged into each row after the main query resolves — see below. Only meaningful when `aggregate` is absent (rows mode). |

**`enrich`, added in this revision, found necessary during MOD-2's actual implementation, not designed speculatively:** the Assets dashboard's own "recent alerts" widget (§8) needs data from three entities — `alerts` (the main query: filtered, sorted, limited), `alert_types` (each alert's type/criticality, via `alerta_id`), and `assets` (each alert's asset name, via `asset_id`). xolu's OQL supports two-table joins only (confirmed directly against `OQL_API.md`); this widget needs three. `enrich` reproduces exactly what the pre-migration code already did — fetch each lookup entity's rows once, build an in-memory map keyed by `id`, merge named fields into each main row by a foreign-key field — as a declarative list instead of hand-written Go:

```json
"enrich": [
  { "on": "asset_id", "from": "assets", "fields": { "name": "asset_name" } },
  { "on": "alerta_id", "from": "alert_types", "fields": { "type": "alert_type", "criticality": "criticality" } }
]
```

| Property | Description |
|---|---|
| `on` | The main row's own field holding the foreign key (e.g. `asset_id`). |
| `from` | The entity to look up (fetched in full, once per widget resolution — not filtered, matching the pre-migration code's own behaviour of fetching every asset/alert-type unconditionally). |
| `fields` | Map of `source field on the looked-up entity → name to merge onto the main row`. |

Each `enrich` entry is its own independent lookup and in-memory merge — never a generated OQL `JOIN`, deliberately, since that would immediately hit the two-table limit the moment a widget needs more than one enrichment (as this one does). Two enrichments here, two entities, zero joins.

### 5.2 `source.type: "audit_log"`

```json
{ "type": "audit_log", "resource": "settings", "limit": 5 }
```

| Property | Required | Description |
|-----------|:---:|-------------|
| `resource` | no | Filters on `AuditEvent`'s `Resource` field (e.g. `"settings"`, `"user"`, `"profile"`). If absent, brings all events for the user/tenant per context (see §9.2). |
| `event_types` | no | List of `AuditEventType`s to include (e.g. `["user_created", "user_updated", "role_changed"]`). If absent, includes every type matching `resource`. |
| `scope` | yes | `"tenant"` (all tenant events — for the Administration section) or `"self"` (only the authenticated user's own events — for the User Profile section). |
| `limit` | no | Maximum event count. |

A widget with `source.type: "audit_log"` is always `type: "recent_list"` — there's
no real case today combining an audit log with `stat_card`/`breakdown_list`/etc.,
so that combination is left unspecified in the MVP rather than designed
speculatively.

---

## 6. Row Field Mapping

`recent_list` references row fields returned by `source` (`primary_field`,
`secondary_field`, `badge_field`, `timestamp_field`) by name — the same name
the entity exposes (already English, post-T-40) or, for `audit_log`, one of
`AuditEvent`'s fixed fields (`event_type`, `resource`, `action`, `email`,
`timestamp`, `details`). `urgency_list`'s row fields are fixed by
`UrgencyRow`'s own shape (§4.3), not configurable per widget. The engine
needs no additional mapping table beyond this: the field name in `source`
and the name referenced in the widget are the same.

---

## 7. Conditional Visibility

### 7.1 Field-value visibility

Reuses `internal/condition` (already used by `x-seam-visible-when`,
FORM_ENGINE_SPEC.md §3b) unchanged, for the case — not identified as needed
today but kept for consistency with the form engine — of a widget whose
visibility depends on tenant data (e.g. hiding a map widget if
`tenant_settings.map_enabled` is `false`).

### 7.2 Permission-based visibility — new extension

`x-seam-form` didn't need this because a whole form is already behind a
permissioned route; a dashboard with sections for *different* permission levels
on the *same page* does — this is exactly the System case (§9).

```json
{ "visible_when": { "permission": "user:read" } }
```

A `visible_when` object with the `permission` key (instead of
`field`/`operator`/`value`) is evaluated against `auth.HasPermission(role, perm)`
for the authenticated user, not against row data. The two forms are mutually
exclusive within one `visible_when` object — field-condition and
permission-condition aren't combined in a single object; if a future case needs
that, it's handled with `visible_when: { "all": [...] }` (the same composite
operator FORM_ENGINE_SPEC.md §3b.5 already defines) containing both forms.

---

## 8. Full Example: Assets Dashboard

Direct migration of `dashboard.go` to the declarative format — same content,
same layout, same color/urgency rules, zero functional loss.

```json
{
  "x-seam-dashboard": {
    "id": "assets-dashboard",
    "title": "nav.dashboard",
    "layout": "stack",
    "sections": [
      {
        "id": "top-stats", "grid": "3col",
        "widgets": [
          { "type": "stat_card", "label": "dashboard.open_alerts", "icon": "crisis_alert", "href": "/alerts?status=open", "urgent": true,
            "source": { "type": "entity", "entity": "alerts", "aggregate": "count", "filter": "status = 'open'" } },
          { "type": "stat_card", "label": "dashboard.maintenance", "icon": "engineering", "href": "/assets?status=maintenance", "urgent": false,
            "source": { "type": "entity", "entity": "assets", "aggregate": "count", "filter": "status = 'maintenance'" } },
          { "type": "stat_card", "label": "dashboard.high_criticality", "icon": "dangerous", "href": "/assets?criticality=High", "urgent": true,
            "source": { "type": "entity", "entity": "assets", "aggregate": "count", "filter": "criticality = 'High'" } }
        ]
      },
      {
        "id": "recent-alerts", "grid": "1col",
        "widgets": [
          { "type": "recent_list", "title": "dashboard.recent_alerts", "view_all_href": "/alerts?status=open", "item_href_prefix": "/alerts",
            "empty_icon": "check_circle", "empty_message": "dashboard.no_open_alerts", "item_icon": "warning",
            "primary_field": "asset_name", "secondary_field": "title", "secondary_fallback_field": "alert_type", "timestamp_field": "created_at",
            "badge_field": "criticality", "badge_colors": { "High": "danger", "Medium": "warning", "Low": "success" },
            "source": { "type": "entity", "entity": "alerts", "filter": "status = 'open'", "sort": "-created_at", "limit": 5,
              "enrich": [
                { "on": "asset_id", "from": "assets", "fields": { "name": "asset_name" } },
                { "on": "alerta_id", "from": "alert_types", "fields": { "type": "alert_type", "criticality": "criticality" } }
              ] } }
        ]
      },
      {
        "id": "upcoming-expirations", "grid": "1col",
        "widgets": [
          { "type": "urgency_list", "title": "dashboard.upcoming_expirations", "subtitle": "dashboard.next_30_days",
            "empty_message": "dashboard.no_upcoming_expirations", "item_icon": "event", "item_href_prefix": "/assets",
            "thresholds": [
              { "max_days": 7, "class": "urgent", "label": "dashboard.days_urgent" },
              { "max_days": 14, "class": "warning", "label": "dashboard.days" },
              { "max_days": null, "class": "normal", "label": "dashboard.days" }
            ],
            "source": { "type": "entity", "entity": "assets", "date_fields": ["expiration_date", "warranty_end"],
              "date_field_labels": { "expiration_date": "expiration", "warranty_end": "warranty_end" },
              "within_days": 30, "limit": 10 } }
        ]
      },
      {
        "id": "totals", "grid": "2col",
        "widgets": [
          { "type": "total_card", "label": "dashboard.total_assets", "icon": "inventory_2", "href": "/assets", "view_link_label": "dashboard.view_list",
            "source": { "type": "entity", "entity": "assets", "aggregate": "count" } },
          { "type": "breakdown_list", "title": "dashboard.by_status", "empty_message": "dashboard.no_status_data",
            "labels": { "active": "dashboard.status.active", "inactive": "dashboard.status.inactive", "maintenance": "dashboard.status.maintenance", "lost": "dashboard.status.lost" },
            "colors": { "active": "green-500", "inactive": "gray-400", "maintenance": "yellow-500", "lost": "red-500" },
            "source": { "type": "entity", "entity": "assets", "aggregate": "group_count", "group_by": "status" } }
        ]
      }
    ]
  }
}
```

Note: `criticality = 'High'` — this example already assumes T-40's Asset pass is
complete (`criticality` as the English field name); while that migration stays
paused (see TRACKING.md, T-40), the real implementation of this dashboard must
keep filtering on `criticidad = 'Alta'` until it's done. This spec describes the
target state, it doesn't require reordering T-40's own work.

---

## 9. Full Example: System Dashboard

Unlike Assets, System has no business-widget dashboard — its three sections (one
per view: Settings, User Profile, Administration) show the latest changes made
in that view, and each section only renders if the authenticated user has
permission for that view.

### 9.1 Definition

```json
{
  "x-seam-dashboard": {
    "id": "system-dashboard",
    "title": "system.dashboard.title",
    "layout": "stack",
    "sections": [
      {
        "id": "settings-activity", "grid": "1col",
        "visible_when": { "permission": "settings:update" },
        "widgets": [
          { "type": "recent_list", "title": "system.dashboard.settings_activity",
            "view_all_href": null, "empty_icon": "settings", "empty_message": "system.dashboard.no_recent_settings_changes",
            "item_icon": "tune", "primary_field": "email", "secondary_field": "action", "timestamp_field": "timestamp",
            "badge_field": "action", "badge_colors": {},
            "source": { "type": "audit_log", "resource": "settings", "scope": "tenant", "limit": 5 } }
        ]
      },
      {
        "id": "profile-activity", "grid": "1col",
        "visible_when": null,
        "widgets": [
          { "type": "recent_list", "title": "system.dashboard.profile_activity",
            "view_all_href": null, "empty_icon": "person", "empty_message": "system.dashboard.no_recent_profile_changes",
            "item_icon": "badge", "primary_field": "event_type", "secondary_field": "details", "timestamp_field": "timestamp",
            "badge_field": "event_type", "badge_colors": {},
            "source": { "type": "audit_log", "resource": "profile", "scope": "self", "limit": 5 } }
        ]
      },
      {
        "id": "admin-activity", "grid": "1col",
        "visible_when": { "permission": "user:read" },
        "widgets": [
          { "type": "recent_list", "title": "system.dashboard.admin_activity",
            "view_all_href": null, "empty_icon": "admin_panel_settings", "empty_message": "system.dashboard.no_recent_admin_changes",
            "item_icon": "manage_accounts", "primary_field": "email", "secondary_field": "event_type", "timestamp_field": "timestamp",
            "badge_field": "event_type",
            "badge_colors": { "user_created": "success", "user_deleted": "danger", "role_changed": "warning" },
            "source": { "type": "audit_log", "resource": "user", "scope": "tenant", "limit": 5 } }
        ]
      }
    ]
  }
}
```

`visible_when: null` on the profile section is explicit, not an omission — every
authenticated user has the profile view (agreed explicitly, see
TRACKING.md/this design conversation's own history), so that section carries no
permission condition.

### 9.2 `scope: "self"` vs `"tenant"`

- `"self"` — the engine filters `AuditEvent.UserID` against the request's
  authenticated user. Used by the profile section: each user sees their own
  changes, not anyone else's.
- `"tenant"` — the engine filters `AuditEvent.TenantID` against the authenticated
  user's tenant, without filtering by `UserID`. Used by Settings and
  Administration: changes made by any admin in the tenant are relevant to
  whoever is looking at that section.

### 9.3 Real prerequisite: wiring up the audit log

This specification assumes the following call sites exist; they don't today (see
§1.2) and need adding before the Settings and Administration sections have any
real data to show (the profile section already has partial data via
`AuditPasswordChanged`, but not preference changes):

| Handler | Event to log | Status today |
|---------|-------------------|------------|
| `AdminUserCreate` | `AuditUserCreated` | Event type exists, not wired up |
| `AdminUserUpdate` | `AuditUserUpdated` | Event type exists, not wired up |
| `AdminUserPasswordResetSave` (admin resetting another user) | `AuditPasswordReset` | Event type exists, not wired up |
| Role change (inside `AdminUserUpdate`, if the role changed) | `AuditRoleChanged` | Event type exists, not wired up |
| `SettingsSaveGeneral`/`SettingsSaveNotifications`/`SettingsSaveFeatures` | *(new event type, doesn't exist yet — e.g. `AuditSettingsChanged`)* | No event type exists for Settings at all |
| `ProfilePreferencesSave` | *(new event type — e.g. `AuditPreferencesChanged`)* | Doesn't exist |

This is also the natural occasion to fix the finding from the architecture
discussion: `SettingsSaveGeneral`/`Notifications`/`Features` currently have no
permission gate at all (`requireAuth` only) — wiring up the audit log on these
handlers and fixing their permission gate are changes to the same code, worth
doing together, not as two separate passes.

---

## 10. Implementation Considerations (Go)

Built in MOD-2, following the same pattern as `internal/formengine`:

- `internal/dashboardengine` — resolve/compile/render as three explicit
  phases, not prescribed in earlier drafts of this document but settled
  during implementation:
  - **Resolve** (`source.go`'s `EntityResolver`, implemented against real
    xolu in `internal/ui/dashboard_resolver.go`) — same decoupling
    `formengine.SourceResolver` already established; this package has no
    xolu dependency itself.
  - **Compile** (`compile.go`) — resolved data (Go-typed: `Row`,
    `GroupCountRow`, `UrgencyRow`, or a bare count) is rendered through a
    fixed, per-widget-type template using
    `github.com/ha1tch/seam-ui/pkg/nplate` (§10.1) into a canonical
    `map[string]any` shape. Not part of the original plan for this
    document — added after a design conversation about whether jsonplate
    (which `nplate` extends) was the right fit; see `pkg/nplate`'s own
    doc comment for the reasoning.
  - **Render** (`widgets.go`) — the five widget functions, consuming only
    compiled data, never a resolver's typed result directly.
- Definition files (`assets-dashboard.json`, `system-dashboard.json`) live in
  `internal/dashboardengine/schemas/`, mirroring
  `internal/formengine/schemas/`.
- Caching (`cache.go`): `DashboardEngine.CacheTTL`, applied at the compiled-
  data boundary, keyed by widget type + the widget's own `EntitySource`.
  Zero (the default) disables caching entirely — every render resolves
  fresh, matching this section's own earlier "no real-time refresh
  requirement, so no need to cache aggressively" reasoning. A short TTL
  (a few seconds) is available for widgets whose resolution is genuinely
  heavy (`recent_list`'s `enrich`, `urgency_list`'s full-entity scan)
  without touching `EntityResolver` or the render step at all.
- Chart.js vendoring: a real, concrete pre-step before `line_chart` can ship,
  not yet done. Download the UMD build to `web/static/vendor/`, add it to
  `VENDOR.md`'s asset list (matching htmx/leaflet/lit/autoComplete.js's
  existing entries), and wire it into `web/static.go`'s `//go:embed` — same
  three steps every other vendored dependency in this project already went
  through. No CDN reference, per this project's stated no-CDN policy
  (`VENDOR.md`'s own rationale section).

### 10.1 Why `nplate`, not hand-written Go per widget type

`jsonplate` (`github.com/ha1tch/xolu/pkg/jsonplate`) substitutes `{"$ref":
"path"}` leaves against a data context but has no iteration construct —
deliberately, by its own documented design (event/webhook payload
templates, safe by construction because there's nothing to loop or
branch). Extending it to add iteration was considered and rejected: it's
a shared package with real existing consumers elsewhere in xolu
(`event_dispatch.go`, `v2_dxp_def_handlers.go`, `pkg/storage/sqlite.go`),
and weakening its own stated safety guarantee to serve this package's
needs would be xolu's call, not this project's to make as a side effect.

`pkg/nplate` (this repository, `github.com/ha1tch/seam-ui/pkg/nplate`)
adds exactly one construct, `{"$each": "path", "as": "name", "template":
{...}}`, built entirely on two already-independent, already-tested
packages' public APIs — `queryfy.Collect` drives the iteration (path
resolution, wildcard expansion), `jsonplate.Render` does every actual
value substitution, both inside a loop body and for everything outside
one. `nplate` re-implements neither. See `pkg/nplate/nplate.go`'s own doc
comment for the full design and a worked example.

---

## 11. Out of Scope (revisit if a real case appears)

- Bar, pie, doughnut, and gauge charts — see §1.3 for why `line_chart`
  specifically is now in scope while these remain out
- User-configurable widgets (add/remove/reorder)
- A seventh generic "custom HTML" widget type — if a real need appears that the
  six existing types don't cover, the specific type needed gets specified, not
  a generic escape hatch that reintroduces the problem this spec solves
