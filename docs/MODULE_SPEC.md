# Seam AMS — Module System
## Design Specification

**Version:** 0.5
**Date:** August 2026
**Status:** Internal draft
**Repository:** github.com/ha1tch/seam-ui (core shared across modules)

---

## 1. Purpose and Context

Today's application is one monolithic package (`internal/ui`) with a flat,
non-module-scoped permission model (`internal/auth`'s `Permission` type and
`rolePermissions` table) and no concept of a user having access to some parts
of the application but not others beyond individual action-level permissions.
This specification defines: a reshaped, hierarchical permission format that
carries module identity in the permission string itself; a `Module` as a
first-class Go concept with its own package, routes, and entitlement; a
central registry every other part of the system (router, selector, permission
checks) reads from instead of each maintaining its own copy of "what modules
exist"; and the module selector itself.

### 1.1 Terminology

Precise terms, since imprecision here caused real back-and-forth while
designing this:

- **Module** — a switchable top-level area of the application (Assets,
  Documents, System, future CMMS). Appears as one entry in the module
  selector. Internally, a Go package under `internal/modules/<name>/`.
- **View** — a named area of functionality *within* a module (System has
  three: Settings, Profile, Administration; Assets has several — assets
  themselves, sensors, rules, master data — see §6.2). A view is not
  necessarily a single page; it's the second segment of a permission string
  and the unit at which access is independently gated.
- **Action** — the specific operation within a view: `view`, `edit`,
  `create`, `delete` today; not a closed set, extended per view as needed
  (§2.1).
- **Permission** — a concrete, fully-qualified `module.view.action` string,
  or a pattern containing `*` at the view or action position, granted to a
  role (§2).

### 1.2 Motivation

Two needs converged on this design, not one:

1. Real per-user module entitlement — confirmed directly (this
   specification's own design conversation): a user can have access to
   Assets and not Documents, something today's flat, module-blind
   `Permission` type has no way to express at all.
2. The permission model itself was already found to be conflating unrelated
   concerns before this design started — `PermSettingsUpdate` is reused
   today for both actual tenant settings *and* master-data CRUD (regions,
   brands, custodians, etc.), because nothing more specific existed. A
   reshape that carries module and view identity in the string fixes this
   as a side effect, not a separate piece of work.

### 1.3 MVP Scope

Covers: the permission format and its matching/derivation rules (§2); the
`Module` interface and registry (§3); the selector (§4); System as a fully
worked example, since its rules (unconditional module access, per-view
gating, one always-open view) are the most novel (§5); Assets as a worked
example of the thin-wrapper migration path (§6); Documents as a worked
example of a genuinely minimal module (§7).

Not in MVP scope:

- Runtime/dynamic module loading (`.so` plugins, hot registration). "Registry
  driven" here means one static, compile-time source of truth that the
  selector/router/permission checks all read from — not a plugin system.
  Adding a module still means writing a Go package and registering it.
- Per-user permission overrides distinct from role (today's model is
  role-grants-permissions only; nothing in this design conversation asked
  for per-user exceptions on top of role, so none are added speculatively).
- Migrating `internal/ui`'s existing ~3,000 lines into
  `internal/modules/assets/` physically. Assessed and deliberately deferred
  (§6.1) — proving the registry pattern doesn't require moving working code
  first.

---

## 2. Permission Model

### 2.1 Format

```
module.view.action
```

Three segments, dot-separated. Examples, all confirmed directly:

| Pattern | Meaning |
|---------|---------|
| `assets.asset.view` | View (read) permission on the Asset view, Assets module |
| `assets.asset.edit` | Edit permission on the Asset view, Assets module |
| `assets.asset.*` | Every action on the Asset view |
| `assets.*` | Every view and every action within the Assets module |

`action` is not a fixed enum — `view`/`edit`/`create`/`delete` cover
everything migrated from the existing flat model (§2.5), and a view can
define additional actions if a real case needs one (e.g. a future
`assets.asset.export`), following the same discipline as every other spec
this project has written: specify the concrete case when it's real, not a
speculative closed or open action set up front.

### 2.2 Wildcard Rules

A granted pattern is either fully concrete (all three segments literal) or
ends in an explicit `*` at the view or action position:

- `module.view.action` — exact grant.
- `module.view.*` — every action on that view.
- `module.*` — every view and action in that module.

No implicit trailing wildcard: `assets.asset` (two segments, no `*`) is not
a valid grant — it must be written `assets.asset.*` if that's what's meant.
This is deliberate: a grant's scope should be readable from the string
itself without a special-cased parsing rule for "what happens when it's
short."

### 2.3 Matching Algorithm

To check whether a role holds permission `P` (always a concrete
`module.view.action` string, since that's what code checks against), test
each of the role's granted patterns `G`:

```
match(G, P):
    g_segments = split(G, ".")
    p_segments = split(P, ".")
    for i in range(len(g_segments)):
        if g_segments[i] == "*":
            return true   # rest of P is covered
        if g_segments[i] != p_segments[i]:
            return false
    return true  # G fully consumed and matched — only reachable if G had 3 concrete segments
```

`HasPermission(role, perm)` becomes: does any granted pattern for `role`
match `perm`. Same external shape as today's `auth.HasPermission(role,
Permission)`, different internal implementation.

### 2.4 Module-Access Derivation

**For Apps-group modules (Assets, Documents, future CMMS):** a role has
access to module `M` if and only if it holds at least one granted pattern
whose first segment is `M` (equivalently: `match("M.*", "M.anything.anything")`
is true for at least one granted pattern, by definition, if the role holds
`M.*` or any `M.view.*` or `M.view.action`). No separate stored entitlement
— the permission set *is* the entitlement. This is what makes "user has
Assets access but not Documents access" (confirmed directly, §1.2) true
automatically: a role with zero `documents.*` grants simply never matches
the derivation check, and Documents doesn't appear in that user's selector.

**For System specifically, this derivation does not apply.** Confirmed
directly: every authenticated user always has System access and always has
its Profile view, regardless of role or permissions. System's module-level
access is unconditional — not derived from any permission check at all.
Within System, each *view* is independently gated by the normal mechanism
(§2.3): Profile has no gate (always visible), Settings requires
`system.settings.*`, Administration requires `system.user.*`. This is a
real asymmetry between System and every other module, not an oversight —
stated as its own rule rather than forced through the Apps-group derivation
by treating Profile as some implicit always-granted permission, which would
just be the same rule wearing a disguise.

### 2.5 Migration from the Existing Flat Model

`internal/auth/auth.go`'s current `Permission` constants, mapped to the new
format. Action-segment vocabulary follows this design's own stated examples
(`view`/`edit`, not `read`/`update`) rather than the old constants' naming:

| Old | New |
|---|---|
| `PermAssetCreate` (`asset:create`) | `assets.asset.create` |
| `PermAssetRead` (`asset:read`) | `assets.asset.view` |
| `PermAssetUpdate` (`asset:update`) | `assets.asset.edit` |
| `PermAssetDelete` (`asset:delete`) | `assets.asset.delete` |
| `PermRuleCreate/Read/Update/Delete` | `assets.rule.create/view/edit/delete` (rules live in Assets per the existing module-scoping decision — T-03's "bundle monitoring (sensors/alerts/fsm/rules)... into this module") |
| `PermUserCreate/Read/Update/Delete` | `system.user.create/view/edit/delete` (Administration view) |
| `PermSettingsRead/Update` | `system.settings.view/edit` |
| `PermDataExport` (`data:export`) | `assets.report.export` — confirmed real: gates `/api/v1/reports/export/assets` and `/api/v1/reports/export/events` (`internal/handlers/api/router.go`), Seam's own JSON/CSV export built on `internal/reports`, not a direct xolu passthrough. |
| `PermDataImport` (`data:import`) | **Not migrated.** Confirmed dead: zero usages anywhere outside its own definition and the `rolePermissions` lists — no route, no handler. Removed as part of this migration rather than carried forward for something that doesn't exist. |

**Real gap this migration surfaces, not introduced by it:** every master-data
route (`/regions`, `/currencies`, `/cost-centers`, `/brands`, `/custodians`,
`/alert-types`, `/properties`, `/models`, `/locations` — confirmed directly
in `router.go`) is currently gated by the reused, unrelated
`PermSettingsUpdate`. This reshape is the natural place to give master data
its own real permission — `assets.master.edit` (or per-entity, `assets.
region.edit`/`assets.brand.edit`/etc., if that granularity turns out to
matter) — rather than carrying the conflation forward into the new format
under a different name.

### 2.6 Go Representation

`Permission` stays a `string` type (matching today), just holding the new
three-segment format instead of the old two-segment one. `rolePermissions
map[string][]Permission` stays structurally the same — a role maps to a list
of granted patterns. `HasPermission` is reimplemented per §2.3's algorithm;
its signature doesn't change, so every existing `requirePerm(auth.PermXxx,
...)` call site needs its *constant* updated to the new value, not its own
code touched — a real, large-count but mechanical migration (every
`router.go` route gate), left as its own pass (§8.3), not bundled into this
specification's own scope.

---

## 3. Module Registry

### 3.1 The `Module` Interface

```go
type Module struct {
    ID          string                 // "assets", "documents", "system"
    Label       string                 // i18n key for the selector
    Icon        string                 // Material Icons name
    Group       string                 // "apps" | "system" — which selector group
    DashboardID string                 // x-seam-dashboard id (DASHBOARD_SPEC.md) -- an internal schema identifier, not a URL
    Path        string                 // the URL the selector navigates to (§4.3) -- added in v0.3, see below
    MountRoutes func(mux *http.ServeMux) // registers this module's own routes onto the shared top-level mux -- corrected in v0.4, see below
    HasAccess   func(role string) bool // module-level entitlement (§2.4)
}
```

`DashboardID` and `Path` are deliberately separate fields, not one —
found necessary during MOD-4's actual implementation, not designed this
way from the start: an earlier draft of this struct had only
`DashboardID`, and the selector's own href construction conflated it
with the module's URL (`"/" + DashboardID`). That's wrong for Assets
specifically — its dashboard lives at `/` (the existing root route,
`internal/ui`'s current `Dashboard` handler), not at `/assets-dashboard`.
`Path` is what the selector links to; `DashboardID` stays purely an
internal identifier a module's own dashboard handler uses to pick which
JSON definition to render once a request actually reaches that path.

`MountRoutes`'s signature is corrected in v0.4: this section's own
snippet showed `func(r *Router)` through v0.3, drifted from what MOD-4
actually shipped (`func(mux *http.ServeMux)`, confirmed directly against
`internal/modules/registry.go`) and never caught until MOD-5 needed to
implement a real, non-nil `MountRoutes` for System and had to resolve
what the parameter actually was. `Router` isn't even this package's
type — `internal/modules` is deliberately dependency-free with respect
to `internal/ui`, so the snippet couldn't have been correct as written.
See §5.4 for where MOD-5 resolved the mechanism this implies:
a module whose routes already live in `internal/ui` (behind `internal/
ui.Router`'s private `requireAuth`/`requirePerm` middleware) gets a real
`MountRoutes` via an exported method on `*ui.Router` itself (e.g.
`MountSystemRoutes`) that a thin `internal/modules/<name>` package wraps
in a closure — the middleware and handler logic never move out of
`internal/ui`; only the registration call site does, per §6.1's "thin
wrapper" reasoning, applied here to more than just Assets.

`HasAccess` for an Apps-group module is a thin wrapper around §2.4's
derivation check against that module's ID. For System, it's a function that
always returns `true` — the unconditional rule from §2.4, expressed as code
rather than special-cased inside the derivation function itself, so the
derivation function stays uniform and the exception lives visibly on
System's own registry entry.

### 3.2 Registration

Modules register themselves into a package-level registry at `internal/
server/server.go`'s bootstrap (the same place `internal/ui`, `internal/
alerts`, `internal/reports`, etc. are already wired in today — confirmed
directly by reading `Server.New()`). A `Registry` type holds the ordered
list of registered `Module`s; `Server.New()` calls each module's
`MountRoutes` in turn instead of (today's) a single flat call into `internal/
ui`'s router setup.

### 3.3 Route Mounting

Each module owns registering its own routes onto the shared `*http.
ServeMux` (or whatever router abstraction `Router` wraps today —
`internal/ui/router.go`'s existing `HandleFunc` calls become, for Assets,
the *same calls*, just invoked from `internal/modules/assets/routes.go`
instead of inline in `internal/ui/router.go` — see §6.1 for why Assets
specifically doesn't need its handler *logic* to move, only the
registration call site). §5.4 is this pattern made concrete and shipped,
for System, once MOD-5 needed a real answer rather than a description.

### 3.4 Package Layout

```
internal/modules/
  assets/       — thin wrapper (§6)
  documents/    — minimal module (§7)
  system/       — Settings + Profile + Administration (§5)
```

Confirmed placement: `internal/modules/`, not a top-level `/modules/` —
`internal/` is compiler-enforced privacy, and this project ships as one
binary, not a library other code imports (this design conversation's own
prior turn, agreed directly).

---

## 4. Module Selector

### 4.1 Structure

```
[Apps ▾]
  Apps
    Assets
    Documents
  System
```

One entry for System (not a group header with children) — its three views
live inside System's own UI once selected, not as separate switcher
entries. This was corrected explicitly during this design conversation
after an initial wrong assumption; recorded here so the correction doesn't
get silently lost.

**As built now (2026-11-18).** The menu is titled "Modules", the word the rest of the product
uses, and its button says so beside the grid icon. It opens a panel, not a list:

```
[▦ Modules ▾]
  Modules
  [icon] Assets         [icon] Documents
         one line              one line
  [icon] Work Orders    [icon] Issues
  ...
  ─────────────────────────────────────
  ⚙ Settings   Administration
```

Application modules (`Group: "apps"`) are tiles in two columns, each with its icon, its name
and a line saying what it is for (`<Label>.desc` in the locale files; a module without one shows
its name alone). System modules are links in the row under them. The module the page belongs to
is marked. With a single application the panel is one narrow column. Escape and a click outside
close it. Markup: `internal/ui/module_selector.go`; look: `.seam-mod-*` in `app.css`.

### 4.2 Data Source

The selector reads the `Registry` (§3.2) directly: for each registered
`Module`, call `HasAccess(currentUser.Role)`; if true, render an entry
under `Group`. No separate selector-specific data model — the registry
already has everything (`Label`, `Icon`, `Group`), so there's nothing to
keep in sync between "what modules exist" and "what the selector shows."

### 4.3 Landing Behaviour

Selecting a module navigates to its `DashboardID` (DASHBOARD_SPEC.md).
Placement: top-right header, left of the user-profile menu, next to the
dark-mode toggle (`internal/ui/layout.go`'s existing header row — confirmed
directly, that's where the dark-mode button and user menu already live).

### 4.4 Documents as a Minimal Module

Confirmed directly: Documents appears in the selector normally, lands on a
default dashboard, and — since it has no other views yet (`DOCUMENTS_
MODULE_SPEC.md` exists, T-04 to build it is still open) — its own
application menu is empty. For this to work under §2.4's derivation rule
(module access requires holding *some* `documents.*` permission), Documents
needs at least one real granted permission to exist, or it satisfies the
rule vacuously for nobody. **Confirmed:** one nominal permission,
`documents.document.view`, granted to all four existing roles by default —
enough to make the module real and visible under the same rule every other
module follows, without requiring any actual Documents functionality to
exist yet.

---

## 5. System Module (Worked Example)

### 5.1 Views

| View | Permission | Gated? |
|---|---|---|
| Profile | *(none)* | Never — every authenticated user |
| Settings | `system.settings.*` | Yes |
| Administration | `system.user.*` | Yes |

### 5.2 Relationship to DASHBOARD_SPEC.md

System's dashboard (DASHBOARD_SPEC.md §9) already uses this exact model —
`visible_when: { "permission": "system.settings.update" }`-shaped conditions
per section, one section per view, Profile's section carrying no
`visible_when` at all. That spec was written before this permission format
was finalized; its examples use the *old* flat permission strings
(`"settings:update"`, `"user:read"`) and need updating to the new
`module.view.action` format as part of implementing this specification —
noted here so it isn't missed, not treated as a new decision (DASHBOARD_
SPEC.md's own design — three sections, one per view, permission-gated — is
unchanged; only the permission string format changes).

### 5.3 Registry Entry

```go
Module{
    ID:          "system",
    Label:       "nav.module.system",
    Icon:        "settings",
    Group:       "system",
    DashboardID: "system-dashboard",
    Path:        "/system",
    HasAccess:   func(role string) bool { return true },
    MountRoutes: system.MountRoutes(uiRouter), // corrected in v0.4 -- system.MountRoutes takes *ui.Router and returns the func(mux) closure; see §5.4
}
```

### 5.4 Route Registration Mechanism (v0.4, MOD-5)

New in v0.4 — resolves §3.1's own corrected note for a concrete case.
System's Profile/Settings/Administration views, and the routes behind
them, already exist in `internal/ui` (`registerRoutes()`, behind
`Router`'s own private `requireAuth`/`requirePerm` middleware). MOD-5's
job wasn't to move that logic — it was to figure out how a real,
non-nil `MountRoutes` could register those same routes onto the shared
top-level mux (`modules.Registry`'s own documented contract) without
either bypassing that middleware or leaving the original registration in
place as dead, shadowed code.

Resolved as: `internal/ui.Router` gained one new exported method,
`MountSystemRoutes(mux *http.ServeMux)`, which registers Profile/
Settings/Administration/the System dashboard using exactly the same
`r.requireAuth`/`r.requirePerm`/`r.handler.*` calls `registerRoutes()`
used to make inline — same middleware, same handlers, same
`r.config.EnableAdmin` gate on Settings/Administration, same everything,
just called from a different site. `registerRoutes()` no longer
registers any of these paths on `r.mux` (`Router`'s own private mux),
so each route is registered exactly once — on the mux `internal/server/
server.go` passes to `registry.MountAll`, the same one `Router` itself
is mounted on via `mux.Handle("/", uiRouter)`.

`internal/modules/system` stays a thin, two-line wrapper
(`MountRoutes(router *ui.Router) func(mux *http.ServeMux)`) purely so
`internal/server/server.go` has a `modules.Module`-shaped value to
register — the substantive logic lives in `internal/ui`, where the
private state it needs already is. This is the general pattern §6.1
gestures at ("only the registration call site moves") made concrete
enough to actually implement; MOD-6 (Assets) followed the same shape
-- see §6.4.

---

## 6. Assets Module (Worked Example — Thin Wrapper)

### 6.1 Why Thin, Not Relocated

`internal/ui` today is, functionally, almost the entire Assets module
already — assets, sensors, rules, alerts, masters, ~3,000+ lines across
`routes.go`/`assets.go`/`dashboard.go`/`masters_*.go`/etc. Physically moving
all of it into `internal/modules/assets/` as a prerequisite for this
specification would be a large, mechanical, risky move disconnected from
proving the actually-new part (registry, entitlement, selector) works —
assessed directly against this project's own recorded lesson (the xolu
`/loc` note: don't fork off a default pattern before the critical
integration is proven end-to-end) and agreed. `internal/modules/assets/`
registers `internal/ui`'s existing handlers as-is; the physical relocation,
if ever done, is separate, later, optional cleanup.

### 6.2 Illustrative View Mapping

Not exhaustive — enough to prove the registry/permission pattern against a
real, large, already-working module, following the same "worked example,
not full enumeration" precedent DASHBOARD_SPEC.md §8 set for the Assets
dashboard:

| View | Old permission | New permission |
|---|---|---|
| `asset` | `PermAsset*` | `assets.asset.*` |
| `sensor` | *(none today)* | `assets.sensor.*` (new — sensors have no dedicated permission today; assigning one here, not carrying forward an absence) |
| `rule` | `PermRule*` | `assets.rule.*` |
| `master` | `PermSettingsUpdate` (reused, see §2.5's flagged gap) | `assets.master.*` |

### 6.3 Registry Entry

```go
Module{
    ID:          "assets",
    Label:       "nav.module.assets",
    Icon:        "inventory_2",
    Group:       "apps",
    DashboardID: "assets-dashboard",
    Path:        "/",
    HasAccess:   func(role string) bool { return hasModuleAccess(role, "assets") },
    MountRoutes: assets.MountRoutes(uiRouter),  // corrected in v0.5 -- see §5.4/§6.4, the same closure-over-router shape §5.4 resolved for System
}
```

### 6.4 Resolution (v0.5, MOD-6)

Followed §5.4's pattern exactly, applied to a far larger surface: 98
routes (dashboard, assets, events, sensors, alerts, map, FSM editor,
rules, asset types, and all 9 master tables) versus System's 13 --
confirmed directly against this section's own §6.1 description
("assets, sensors, rules, alerts, masters... ~3,000+ lines"), not a
narrower reading. `internal/ui.Router` gained one new exported method,
`MountAssetRoutes(mux *http.ServeMux)`, extracted mechanically from its
original location in `registerRoutes()` (not retyped, to eliminate
transcription risk at this size) with the exact `EnableOperational`/
`EnableAdmin` gating each route already had preserved unchanged.
`internal/modules/assets` is the same two-line closure wrapper pattern
as `internal/modules/system`.

By MOD-6, `registerRoutes()` has nothing `requireAuth`-gated left inline
at all -- only genuinely public routes (login, password reset, QR scan,
static files) remain.

---

## 7. Documents Module (Worked Example — Minimal)

### 7.1 What Exists Today

Nothing. `DOCUMENTS_MODULE_SPEC.md` (v0.5) is a design document; T-04
("Build Documents module per DOCUMENTS_MODULE_SPEC.md v0.5") is still open
in `docs/TRACKING.md`. No code, no routes, no permissions.

### 7.2 Minimal Registration

Confirmed directly: Documents shows in the selector, lands on a default
(empty-state) dashboard, and its application menu is empty since it has no
other views. `documents-dashboard` is defined as `"sections": []` — the
empty-dashboard case DASHBOARD_SPEC.md §2.2 defines: centred module name +
"No data to display in this module at this time." Registry entry:

```go
Module{
    ID:          "documents",
    Label:       "nav.module.documents",
    Icon:        "description",
    Group:       "apps",
    DashboardID: "documents-dashboard",  // "sections": [] — DASHBOARD_SPEC.md §2.2
    Path:        "/documents",
    HasAccess:   func(role string) bool { return hasModuleAccess(role, "documents") },
    MountRoutes: documents.MountRoutes,  // registers only the dashboard landing route
}
```

### 7.3 Why This Case Matters

Documents proves the registry pattern works for a module that is genuinely
almost nothing — the opposite end of the spectrum from Assets (§6). If the
pattern only worked for a module wrapping 3,000 existing lines, that
wouldn't be evidence it's the right abstraction; proving it against both
ends is why this worked example is here rather than skipped as "obviously
fine."

---

## 8. Implementation Considerations

### 8.1 Bootstrap Order

`internal/server/server.go`'s `New()` constructs the `Registry`, registers
`system`, `assets`, `documents` (order matters only for selector display
order, not correctness), then mounts routes from each in turn — replacing
today's single direct call into `internal/ui`'s router setup.

### 8.2 The `requirePerm` Migration

Every existing `r.mux.HandleFunc(..., r.requirePerm(auth.PermXxx, ...))`
call site needs its constant updated to the new format (§2.5's table).
Mechanical, but real — dozens of call sites in `router.go` alone (confirmed
directly earlier in this design conversation). Its own pass, not bundled
into this specification; §2.6 already says this.

### 8.3 Intersects With an Already-Filed Gap

DASHBOARD_SPEC.md §9.3 already identified that `/settings`'s save handlers
have no permission gate at all today (`requireAuth` only). Fixing that gate
now naturally uses this specification's new `system.settings.edit`
permission rather than the old flat one — one fix, not two passes through
the same code, as already noted there.

### 8.4 Decisions Confirmed During Design (2026-08-02)

All four items this document originally flagged as open are now settled,
recorded here rather than silently dropped:

- `PermDataExport` → `assets.report.export` (real, confirmed against
  `router.go`); `PermDataImport` → not migrated, confirmed dead code (§2.5).
- `assets.master.*` is one permission, not per-entity (§6.2).
- Documents' placeholder permission is `documents.document.view`, granted
  to all roles by default (§4.4).
- A zero-widget dashboard's rendering is defined in DASHBOARD_SPEC.md §2.2
  (added there, not here, since it's dashboard-rendering behaviour, not a
  module concern).

### 8.5 i18n

Confirmed as a governing principle, not specific to any one piece: every
piece of text this specification's own examples show as a literal string
for readability — module labels, the empty-dashboard message, view names —
is an i18n key in the real implementation, never a hardcoded string. This
document's own Go snippets (`Label: "nav.module.assets"`) already follow
that convention; stated explicitly here so it isn't assumed only where it
happened to already be written that way.

---

## 9. Out of Scope

- Dynamic/plugin module loading — the registry is a static, compile-time
  list (§1.3)
- Per-user permission overrides on top of role
- A fifth or later module (CMMS) — this spec's job is the mechanism,
  proven against System/Assets/Documents; a new module is a new registry
  entry following §5–§7's pattern, not a reason to speculate about its
  shape now
