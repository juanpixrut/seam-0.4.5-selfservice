# Left sidebar / module synchronization — plan

Date: 2026-08-06
Resolves: T-74's two open design questions
Status: not previously planned — T-74 identified the problem and raised two questions, neither answered. This is the answer.

---

## The two questions, resolved

### 1. Does each module own its own sidebar items, or is there one central lookup table?

**Each module owns its own.** This isn't a new mechanism — it's the same pattern already used for `MountRoutes`, `Label`, `Icon`, and `Path`: a module declares what it needs about itself, once, at registration, in `internal/server/server.go`. A central lookup table would be a second place carrying knowledge the module's own registration already carries, and the two could drift the way `MainNav`'s own item list has already drifted from the module system it predates.

**One real technical constraint this resolves, not just a preference**: `internal/modules`'s own package doc comment states its design directly — *"deliberately dependency-free with respect to any one module's own implementation — it doesn't import internal/ui."* And `internal/ui` already imports `internal/modules` in six files. So the new field can't be typed as `ui.NavItem` — that would either force a new import `internal/modules` explicitly avoids, or create an actual circular import, since `ui` already depends on `modules`.

The fix: a new, minimal, dependency-free type declared *in* `internal/modules`, matching exactly how `Label`/`Icon`/`Path` are already plain strings for the same reason:

```go
// SidebarItem is one entry a module can declare for the left sidebar.
// Deliberately as dependency-free as Module's own Label/Icon/Path
// fields -- no minty/mi.H types, matching this package's own
// "no dependency on internal/ui" principle stated in its doc comment.
type SidebarItem struct {
	Label string // i18n key, matching Module.Label's own convention
	URL   string
	Icon  string
	// RequiredPerm, optional -- see "permission filtering" below.
	RequiredPerm string
}
```

Added to `Module` as `SidebarItems []SidebarItem`. `internal/ui`'s own `Sidebar`/`MainNav` code converts each into a `ui.NavItem` at render time — that conversion is fine, since `ui` importing from `modules` is the existing, correct direction.

### 2. Do modules with only a few routes need a populated sidebar at all?

**No — it's optional, and zero is a fully valid answer.** A module that declares no `SidebarItems` simply gets no sidebar section when it's active; the user sees that module's own page with no extra clutter. This isn't a special case to code around — it falls out naturally from the same field being empty. Documents (one route), and now Work Orders and Issues (list/detail/create, no natural sub-areas), would all declare zero items and correctly show nothing. Assets, with 17 genuinely distinct sub-areas, declares 17. Nothing in between needs deciding case by case — each module answers this for itself, honestly, based on whether it has real sub-navigation or not.

## How "which module is active" gets determined

Using data already computed, not new plumbing: `cfg.VisibleModules` (`[]modules.Module`, already permission-filtered per request, populated by `PageHandler.viewerConfig` on every render) is walked to find whichever module's own `Path` is the longest matching prefix of `activePath`. Assets' own `Path` is `"/"` — the universal fallback, since every path matches it, but at length 1 it only wins when nothing more specific does. A request to `/issues/42` matches both `/` (Assets, length 1) and `/issues` (Issues, length 7) as prefixes; longest wins, correctly resolving to Issues. This generalizes correctly to every module without special-casing any one of them.

## Permission filtering within a module's own items — a related, smaller gap worth closing in the same pass

T-74's own investigation found a second gap: the sidebar shows every item to every role today, regardless of whether that role can actually reach it (a Viewer sees Rules and FSM Editor, which 403 at the route level). Since `SidebarItem` is now declared per-module anyway, adding an optional `RequiredPerm string` field lets a module gate its own items the same way `Module.HasAccess` already gates the module itself — checked against `cfg.UserRole` (also already populated, same place `VisibleModules` is). An empty `RequiredPerm` means always-visible-within-the-module, the same "empty means no gate" convention already used elsewhere (`HideFromSelector`'s own default, for instance).

## What this does *not* attempt

Doesn't touch `NavItem.Children` (still unused after this) — repurposing it wasn't necessary once `SidebarItem` lives at the module level rather than needing nesting inside the existing flat list. Doesn't change `Sidebar`/`MobileSidebar`'s own rendering mechanics (striping, collapse/expand, the mobile active-state JS) — those already work correctly on whatever list of items they're handed; the fix is entirely in *which* items get handed to them, not how they're rendered once received.

## Migration shape, for whoever implements this

Not a rewrite of `MainNav`'s current 19 items — a redistribution. Each item already belongs to exactly one module (17 of 19 to Assets, 2 to Administration/Settings respectively, confirmed in T-74's own original investigation); each becomes that module's own `SidebarItems` entry, declared at its `registry.Register(modules.Module{...})` call site in `server.go`, not invented fresh. `MainNav` itself is deleted once every module's registration carries its own items — there's no ongoing use for a centralized item list once each module owns its own.
