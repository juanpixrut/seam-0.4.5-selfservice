# Status -- Seam AMS / seam-ui -- 2026-10-07, second session

Read after `docs/HANDOVER_2026-10-07.md`. Where they differ, this file is newer.
No existing file was edited with repoman in this session, so `docs/TRACKING.md`
still shows the old states; the real states are below. VERSION is still 0.4.0,
no release was cut, CHANGELOG has no entry.

## Environment this session

Network was open. Go 1.27.1 installed (go.mod requires 1.26.5 or newer).
gorepoman v0.16.0, checksum verified, `selftest` exit 0. xolu v0.30.23 cloned
at the path the `replace` directive names (not included in this zip). Local
badcode config recreated outside the repo; `badcode check` was NOT run.

## Baseline (after one forced fix, see T-94 below)

| Command | Result |
|---|---|
| `go build ./...` | exit 0 |
| `go vet ./...` | exit 0 |
| `go test ./... -count=1` | exit 0, 25 packages ok |

`internal/ui/workflow` is a separate module and was not covered (T-86).
The tree as received did NOT build.

## Register items

| ID | Real state |
|---|---|
| T-94 | Compiles; the 4 tests (2 changed, 2 new) pass. Needed one fix: `internal/ui/doclink_js.go` renamed to `internal/ui/doclink_script.go`, because a `_js.go` suffix is a GOOS=js build constraint and the file was silently excluded on Linux. Browser check still outstanding. Stale mentions of the old file name remain in `docs/TRACKING.md` (T-94 text) and in the handover. |
| T-95 | Compiles. New tests in `internal/auth/store_xolu_listusers_test.go` (3, pass, vet clean). The `LIMIT`/`OFFSET` sweep over `*.go` and `*.md` shows only expected leftovers. Not done: sweep of seeds/JS/JSON, live-xolu check that `TOP` applies after `ORDER BY`, Sulpher grammar check. |
| T-96 | Not started; design decisions below. |
| T-97 | Not started. |
| T-98 | Answered: all users belong to the current tenant. The Users source keeps the session tenant condition; the omission of a cross-tenant path is deliberate. |
| T-99 | Untouched. |

## Decisions taken from the user's answers

* Repeatable at zero rows: confirmed behaviour is "no blank row remains, the
  add button creates a blank editable row". First render of an empty list shows
  one blank row. The server drops a row whose first field is empty. Known edge,
  not changed: a row with an empty first field but other fields filled is
  dropped silently on save.
* Users selector: open to every authenticated role, but lists only users whose
  role rank is equal or lower than the caller (admin > manager > operator >
  viewer). An internal switch (config field on the UI handler, default
  `hierarchy`, alternative `tenant`) allows listing every user of the tenant.
  Unknown or empty caller role lists nobody. The picker's id lookup must apply
  the same rule.

## Plan for the next session (acknowledged)

1. Role rank and "roles at or below" helpers in `internal/auth`, with tests.
2. `UserFilter` gains `Roles []string` and `Search string` (OQL: parenthesised
   `OR` of `role = '...'`, `LIKE` with `escapeOQL`); `XOLUUserStore.ListUsers`
   uses them; tests.
3. `ListRowWithAttrs` in `internal/ui/listing.go`; `ListRow` delegates to it.
4. T-96: `internal/ui/selector.go` with a server-side registry (key, columns,
   fetch function), route `GET /selector/{key}` under `requireAuth`, htmx
   search and Prev/Next paging inside the modal, choose helper in
   `web/static/js/app.js` dispatching `seam:selector-chosen`, i18n keys in all
   four locale files, tests per handover 6.6 plus the role-range rule.
5. T-97: register `any-picker` (keep `asset-picker` as alias), grow
   `pickerSearchConfigs` with `entity:users`, optional browse button opening
   the Selector, docs.
6. Fix the stale file-name mentions, then the sweep (7.8), `register check`,
   and the browser check for T-94.

## Commands run this session, in order

curl probe of go.dev, dl.google.com, proxy.golang.org, example.com, github.com:
all 2xx/3xx. gorepoman download, checksum compare, `repoman selftest`: exit 0.
Go install from go.dev. `git clone --branch v0.30.23` of xolu. `go build ./...`:
failed at `internal/ui/masters_pages.go:941` (undefined symbol); file renamed;
`go build`, `go vet`, `go test ./... -count=1`: all exit 0. T-94 tests run by
name: 4 pass. `go test ./internal/auth/ -run TestListUsers`: 3 pass; `go vet
./internal/auth/`: exit 0.
