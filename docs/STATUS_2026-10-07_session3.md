# Status -- Seam AMS / seam-ui -- 2026-10-07, third session

Read after `docs/STATUS_2026-10-07_session2.md`. Where they differ, this file is
newer. Everything below was written WITHOUT a Go toolchain (none in the session
sandbox): not compiled, vetted or tested. First action next time: `go build ./...`,
`go vet ./...`, `go test ./... -count=1`. As in session 2, no register entry was
edited with repoman, so `docs/TRACKING.md` still shows the old states for T-95..T-97.

## Plan items from session 2, in order

1. Role rank helpers: new `internal/auth/roles.go` (`RoleRank`, `RolesAtOrBelow`,
   `RoleAtOrBelow`) and `roles_test.go`.
2. `UserFilter` gains `Roles []string` and `Search string` (`internal/auth/users.go`);
   `XOLUUserStore.ListUsers` builds `(role = 'a' OR role = 'b')` and
   `(name LIKE '%q%' OR email LIKE '%q%')` with `escapeOQL`. nil `Roles` = no role
   filter; a non-nil EMPTY `Roles` = match nothing, no query sent. Tests in new
   `store_xolu_listusers_filter_test.go` (the session 2 test file is untouched).
3. `ListRowWithAttrs` in `internal/ui/listing.go`; `ListRow` delegates to it. Test in
   new `listing_rowattrs_test.go`.
4. T-96: new `internal/ui/selector.go` and `selector_test.go`; route
   `GET /selector/{key}` in `router.go`; `UIHandler.selectorUsersScope` field in
   `routes.go`; `SeamSelector.choose` in `web/static/js/app.js` (logic run in node
   against a stub DOM: dispatches the event, ignores a missing id or container);
   six `selector.*` keys in all four locale files. Spec: `docs/SELECTOR_SPEC.md`.
5. T-97: `any-picker` registered next to `asset-picker` (`renderAnyPicker`, new
   `x-seam-selector` schema key, `field.any_picker.browse` key in four locales);
   `pickerSearchConfig` gained optional `search`/`lookup` functions; `entity:users`
   source in new `internal/ui/picker_users.go` with `picker_users_test.go`; formengine
   tests in `any_picker_test.go`. The id lookup applies the same visibility rule as the
   list. Docs: FORM_ENGINE_SPEC.md (any-picker section, property/source/widget tables).
6. Stale `doclink_js.go` mentions fixed in `docs/TRACKING.md` and the handover.

## Not done

* The sweep (handover 7.8), `register check`, `badcode check`, the T-94 browser check,
  the T-95 live-xolu `TOP`/`ORDER BY` check and Sulpher grammar check.
* Browser check of the Selector: htmx search (`hx-select` + `outerHTML` on
  `#seam-selector-results`, input keeps focus) and Prev/Next inside the modal.
* Wiring `SetSelectorUsersScope` to configuration (default `hierarchy` applies).
* No schema uses `any-picker` or `x-seam-selector` yet.
* Register: T-95/T-96/T-97 states need updating with repoman after the build is green.

## Points to verify first when building

* `ListRowWithAttrs` takes `[]interface{}` of `mi.Attr(...)` values; `mi.Attr` is
  assumed to return a value `b.Tr` accepts (same as in the other attribute slices).
* Tests assume minty HTML-escapes attribute values and writes `data-id="7"` style
  attributes; `selector_test.go` also asserts exact `hx-get="/selector/users?target=owner"`.
* `Selector` reads `r.PathValue("key")`; tests set it by hand with `SetPathValue`.
* Selector requires a tenant id > 0 in the session claims (otherwise it lists nobody).

## Demo module (added later in session 3)

New opt-in module at `/demo` (flag `features.enable_demo`, env `SEAM_ENABLE_DEMO=1`,
default false). Two tabs, each with a short text: "New components" (Selector and
any-picker, real form engine) and "Repeatable fix" (the reported schema as text next
to the corrected one rendered by the engine). Each form posts to `POST /demo/echo`,
which shows what the server received and saves nothing. Any valid role may open it
(`requireAuth` only). Files: `internal/ui/demo.go`, `demo_test.go`,
`internal/modules/demo/`, `internal/formengine/schemas/demo/`, route
`MountDemoRoutes` in `router.go`, registration in `server.go`, `EnableDemo` in
`config.go` and `layout.go`, `demo.*` keys in the four locale files.

Engine change made for it (`internal/formengine/repeatable.go`): the repeatable add
button now finds its rows container as its own previous sibling instead of by
`getElementById` (nested repeatables repeat the container id per outer row, so the
second block's "+ Add" grew the first block's lines), and a cloned outer row keeps
only the first row of each nested container. The T-94 stash/remove handler and its
tests are unchanged. Tests: `repeatable_nested_test.go`.

Follow-up (same session): the three limits found while building the Demo are now
addressed, still uncompiled and untested in a browser.

* Nested values: repeatables that hold a nested repeatable or an any-picker (or set
  `x-seam-naming: "indexed"`) post indexed names (`blocks[0][lines][1][name]`);
  `fe.ParseIndexedRepeatable` / `fe.ParseIndexedForm` rebuild the structure server-side
  (`internal/formengine/repeatable_indexed.go`, tests in `repeatable_indexed_test.go`).
  Plain repeatables keep the flat names, so Asset service configurations and their
  handlers are untouched. No production handler reads indexed names yet: the Work Orders
  template handler must call `ParseIndexedForm` on its schema's fields.
* Pickers in rows: any-picker no longer delegates to `renderAssetPicker`; its markup is
  inert and `SeamPicker` (`web/static/js/app.js`) initialises it, including rows added
  later. asset-picker is unchanged and still not usable inside repeatable rows.
* Emptied nested container: indexed repeatables add from a `<template>`, so adding after
  removing every row works at any depth. The flat repeatable's stash behaviour (T-94) is
  unchanged.

Checked in node against a stub DOM: SeamRepeatable.add substitutes the index at both
levels and bumps the counter, remove works, app.js parses. NOT checked: the browser
(autoComplete.js behaviour in added rows, htmx swaps, the Selector round trip inside a
row). Assumptions to verify first when building: `mi.Raw` is accepted as a child of a
`div` (used here to embed the `<template>`); `mi.RenderToString` accepts a func literal;
the vendored autoComplete.js accepts `selector: '#<id>'` with the generated ids.

## Demo records (persistence for the Repeatable fix tab)

The Repeatable tab can now save what its form posts, to show that nested values
(blocks -> lines) round trip through `fe.ParseIndexedForm`. It is kept apart from real
data by construction:

* one dedicated xolu entity, `demo_templates` (`demoEntity` in
  `internal/ui/demo_templates.go`). No other module reads or writes it, and the demo
  never touches any other entity: `demo_templates_test.go` records every request the
  handlers make and fails if any path or OQL query names something else.
* every record carries `demo: true` and its `tenant_id`. Listing, delete and reset are
  filtered by the session tenant; delete and reset refuse any record that is not a
  boolean `demo: true` of that tenant. No session tenant (or tenant <= 0) means nothing
  is listed, saved or deleted.
* the module itself is opt-in (`EnableDemo`).

Record shape: `{tenant_id, demo, name, blocks: [{name, responsible_id, lines: [{name,
type}]}], workflow_states: [{name}], created_by, created_at}`. `blocks` comes from
`ParseIndexedForm` (indexed names); `workflow_states` from the flat
`workflow_states_name[]` array, deliberately left flat to show both conventions.

Routes (all `requireAuth`, only mounted with the flag): `POST /demo/templates` (save),
`POST /demo/templates/{id}/delete`, `POST /demo/templates/reset` (up to 200 records of
the tenant). After each, redirect to `/demo?tab=repeatable`, which re-opens that tab;
only that exact value produces any script. The page lists the tenant's latest 20 records
with their structure as indented JSON.

To remove the demo data completely: use "Delete all demo records" on the tab (per
tenant), or drop the `demo_templates` entity in xolu. Nothing else depends on it.

Not checked, as everything else this session: compilation, browser behaviour, and the
xolu client's exact wire shapes for Create/Get/Delete (the tests' fake server assumes
`POST /api/v1/<entity>`, `GET|DELETE /api/v1/<entity>/<id>`, and `POST /api/v1/oql/query`
with a `{"query": ...}` body, copied from the existing tests).

## Fixes after the developers' first run (0.4.1-pre-release)

* Demo tabs did nothing: `app.js` is loaded with `defer`, so on a full page load the
  inline `SeamTabs.wire(...)` ran before `SeamTabs` existed. The Demo page and the
  form engine's own tab renderer now wait for DOMContentLoaded when it is not defined yet.
* Any-picker (and Selector) user search found nobody: the search is now done in Go,
  case-insensitive, over name and email of the visible users (`listVisibleUsers`), not
  with an OQL `LIKE ... OR LIKE ...` condition. The cause was not confirmed against a
  running xolu (candidates: case-sensitive LIKE, or the two-LIKE OR condition); the new
  path avoids both. `UserFilter.Search` and its store support remain, unused by the UI.

## Second round of developer reports (0.4.1-pre3)

* Nested repeatables hard to tell apart, cells without placeholders/names, rows unlike the
  Identity Fields of the Asset Type form: rows are tinted cards with per-cell placeholders
  and aria-labels, `x-seam-placeholder`, optional i18n keys `repeatable_item.*`, nested
  boxes indented under a rule with a heading (`repeatable.go`, `repeatable_indexed.go`,
  `repeatable_labels_test.go`).
* Icon search: `icon-picker` + `GET /picker/icons` (`internal/ui/icon_picker.go`,
  `SeamPicker` endpoint/preview support in `app.js`).
* i18n workload: `scripts/i18n-sync.py`, `make i18n-check`, `make i18n-todo`.
* Seeds 03 and 04 users get `tenant_id: 1`.

## 0.4.1-pre4
* icon-picker Browse grid (scrollable, filterable, lazy chunks) and a larger preview of the
  chosen icon (`SeamPicker.toggleGrid` in `app.js`, `?all=1` in `icon_picker.go`).

## 0.4.1-pre5
* Demo shows the icon-picker, `x-seam-placeholder`, the row visuals and an icon-picker inside
  a repeatable; texts updated in four languages.

## 0.4.1-pre6
* `x-seam-layout`, `x-seam-browse`, `x-seam-highlight`; icon buttons for repeatable add/remove;
  dark-mode text colours (inputClass, picker CSS, app.css theme variables); dropdown/grid
  exclusivity; `SeamHighlight`; list row click (`SeamList`, `ListCellLink`, docs/LIST_SELECTION.md).
