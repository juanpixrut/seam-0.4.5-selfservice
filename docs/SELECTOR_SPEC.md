# Selector dialog (T-96)

Status: implemented in the 2026-10-07 session 3 changes, not yet compiled or
exercised in a browser. Written in English per the repository's documentation
convention.

## What it is

A common modal dialog for choosing ONE record from a list. The list is laid out
with the List primitives (the same ones the Assets and Users list pages use), with
a search box, Previous/Next paging and a Choose action per row. Which entity it
lists is decided by a server-side registry; the client only sends a registry key.

Code: `internal/ui/selector.go` (registry, handler, Users source),
`web/static/js/app.js` (`SeamSelector.choose`), route in `internal/ui/router.go`.

## Request

    GET /selector/{key}?q=<text>&page=<n>&target=<id>

Under `requireAuth`, with no permission gate: every authenticated role may open a
selector. What a source lists is restricted inside the source itself.

* `key` must be in `selectorSources`; otherwise 400.
* `q`: trimmed, at most 100 runes.
* `page`: 1-based, clamped to 1..200 (paging fetches TOP (page*size+1) rows because
  OQL has no OFFSET; the extra row tells whether a next page exists).
* `target`: the id of the opener's hidden input. Only `[A-Za-z0-9_.:-]{1,64}` is
  accepted; anything else is dropped (rendered as empty).

The response is always an HTML fragment (the modal body, or, for htmx search and
paging requests, the results block that `hx-select` extracts from it).

## Opening and choosing

    SeamModal2.open(title, '/selector/users?target=<hidden-input-id>', {maxWidth:'720px'})

Each row carries `data-id` and `data-label`. Clicking a row, pressing Enter on it,
or pressing its Choose button calls `SeamSelector.choose(row)`, which reads the two
attributes with `dataset` (never innerHTML), dispatches

    document: CustomEvent 'seam:selector-chosen'
    detail: { key, id, label, target }     (all strings)

and closes the modal. The opener listens for the event and matches `detail.target`
to its own input. The `any-picker` widget does this (see FORM_ENGINE_SPEC.md).
Record labels are never interpolated into inline script.

## Registry

    type SelectorSource struct {
        Key, TitleKey, EmptyIcon, EmptyTitleKey, EmptyMessageKey string
        Columns  []SelectorColumn   // LabelKey (i18n), Right, Cell(row, t)
        PageSize int                // default 10
        Fetch    func(ctx, h, q, page, pageSize) (rows []SelectorRow, hasMore bool, err error)
    }

`Fetch` is a function, not a table/column pair, so a source can go through a service.
To make another entity selectable, add an entry to `selectorSources`, add its i18n keys
to all four locale files, and add tests mirroring `selector_test.go`.

## Users source

Goes through `auth.UserService.ListUsers`; password hashes never reach the page.
Always applied (shared with the `entity:users` picker source, see
`visibleUsersFilter` and `userVisibleToCaller` in `selector.go`):

* the session tenant (T-98: all users belong to the current tenant; there is
  deliberately no cross-tenant path). No session claims, or a tenant id <= 0, lists
  nobody; the tenant condition is never dropped.
* active users only.
* by default (scope `hierarchy`) only users whose role rank is equal to or lower than
  the caller's (admin > manager > operator > viewer, `auth.RoleRank`). An unknown or
  empty caller role lists nobody; a user with an unknown role is never listed.

`UIHandler.SetSelectorUsersScope("tenant")` lifts only the role restriction (every
active user of the tenant). The default is `hierarchy`. Nothing wires this to a
config file or environment variable yet; call it at startup if needed.

Search is a case- and accent-insensitive substring match on name or email, done in Go
(`listVisibleUsers`): OQL's LIKE is case-sensitive, so the Users source loads the
visible users (first 1000 by name) and filters and pages them in memory.

Label shown for a chosen user: `Name (email)`, or the email alone when there is no name.

## Not done

Multi-select; selectors for entities other than users; wiring the scope switch to
configuration; a total count (OQL `COUNT(*)` is unverified, so paging is Prev/Next).
