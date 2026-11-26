# Seam AMS — olu Client Remediation Plan

**Version:** 1.0  
**Author:** haitch <h@ual.fi>  
**Date:** January 2026  
**Status:** Complete — all six stages implemented (v0.1.0-patched1 through v0.1.0-patched6)  

---

## 1. Background

Seam AMS uses olu as its sole data store. All entity persistence, graph
relationships, OQL queries, and full-text search pass through
`internal/olu/client.go`, a thin HTTP client written against an early
version of olu's REST API.

Over the past month olu underwent significant hardening: response envelopes
were standardised, graph endpoints changed method and path, the search
surface was consolidated, new write primitives (`Save`, `Commit`) were
added, and authentication was made mandatory for production deployments.
Seam's client was not updated to match. As a result, several categories
of request silently fail or produce corrupted data when issued against
current olu (`v0.9.7-patched74`).

This document catalogues every divergence, describes the correct target
behaviour derived from olu's source, and defines a six-stage remediation
plan with explicit ordering constraints.

---

## 2. Divergence Catalogue

### 2.1 Silent data loss — entity response unmarshalling

**Severity: critical.**

All five mutating operations (`Create`, `Get`, `Update`, `Patch`, `List`)
unmarshal responses into structs with a `Data map[string]any` field,
expecting olu to return a `{"data": {…}}` wrapper. olu does not wrap
entity data.

Actual olu response shapes:

| Operation | olu response | Client expects | Effect |
|---|---|---|---|
| `Create` | `{"message":"…","id":N}` | `{"id":…,"data":{…}}` | `Entity.Data` always nil |
| `Get` | flat document `{"id":1,"name":"…"}` | `{"id":…,"data":{…}}` | `Entity.Data` always nil |
| `Update` | `{"message":"…"}` | `{"id":…,"data":{…}}` | `Entity.Data` always nil |
| `Patch` | `{"message":"…"}` | `{"id":…,"data":{…}}` | `Entity.Data` always nil |
| `List` | `{"data":[…],"pagination":{…}}` | `[{…}]` (bare array) | unmarshal fails silently |

Every read path in Seam AMS is returning empty entity data. This is the
most severe defect in the codebase.

### 2.2 Duplicated and inconsistent `OQLResult` type

**Severity: high.**

`internal/olu/client.go` defines `OQLResult.Data []map[string]any`.
`internal/auth/store_olu.go` defines a separate `OQLResult.Rows
[]map[string]interface{}` with an identical purpose but different field
name. `internal/server/server.go` bridges the two with
`Rows: result.Data`, which is correct only by luck and will silently break
if either definition changes.

olu's actual OQL response is `{"status":"completed","data":[…],"stats":{…}}`,
which matches the client's `Data` field name. The auth package's `Rows`
field and the manual bridge are therefore unnecessary.

### 2.3 No authentication support

**Severity: high in production.**

`OLUConfig` has no field for an olu credential. The client's `do()` method
never sets an `Authorization` header. Any olu instance configured with
`OLU_AUTH_TYPE=jwt`, `OLU_AUTH_TYPE=apikey`, or `OLU_AUTH_TYPE=bearertoken`
will reject every request from Seam with 401. There is no way to configure
olu credentials in Seam at all.

### 2.4 Wrong search endpoint

**Severity: high.**

`Search()` calls `GET /{entity}/search?q=…`.  
olu exposes `GET /api/v1/search?q=…&entity={optional}`.  

The entity is a query parameter, not a path segment. The entity-scoped
path does not exist; every search call returns 404.

### 2.5 Graph endpoints — wrong method and path

**Severity: high.**

All three graph methods call the wrong endpoint or use the wrong HTTP
method:

| Client call | olu endpoint | Problem |
|---|---|---|
| `GET /graph/neighbors/{id}?depth=N` | `POST /graph/neighbors` (JSON body) | wrong method; wrong URL structure |
| `GET /graph/shortest-path?from=…&to=…` | `POST /graph/shortestPath` (JSON body) | wrong method; camelCase path; body not query params |
| `POST /graph/sulpher` | `POST /graph/query` | endpoint renamed |

The graph request and response shapes have also changed:

**Neighbors:** request body `{"node_id":"…","direction":"out|in|both"}`;
response `{"neighbors":{"outgoing":{node:label,…},"incoming":{…}}}`.

**ShortestPath:** request body `{"from":"…","to":"…","max_depth":N}`;
response `{"from":…,"to":…,"exists":bool,"path":[…],"length":N}`.

**GraphQuery (formerly Sulpher):** request body `{"query":"…","max_depth":N}`;
response `{"status":"completed","result":[…],"stats":{…}}` — note `"result"`,
not `"data"`, unlike the OQL response.

### 2.6 `tenantID` type mismatch

**Severity: medium.**

The client stores the tenant identifier as a bare `string` with no
validation. olu requires node IDs to use the `XXXX@entity:id` format where
`XXXX` is 4-digit uppercase hex. `buildURL` inserts the tenant string
directly into the path as `/api/v1/tenant/{tenantID}/…`. If the caller
passes a decimal integer (`"1"`, `"42"`) rather than a formatted hex
prefix (`"0001"`, `"002A"`), olu will reject or misroute the request.

### 2.7 Missing `Save` and `Commit` operations

**Severity: medium.**

Two write primitives added to olu are not exposed in the client:

**`Save`** — upsert by caller-specified ID (`POST /{entity}/save/{id}`).
Returns `{"created": bool, "id": N}`. Useful for idempotent device
registration where the caller controls the ID.

**`Commit`** — atomic upsert + one or more inserts in a single transaction
(`POST /oql/commit`). Request mirrors `storage.CommitRequest`:

```json
{
  "update": {"entity":"…","id":N,"version":V,"data":{…}},
  "append": [{"entity":"…","data":{…}}, …]
}
```

Response mirrors `storage.CommitResult`. Maximum 25 append entries per
commit. Returns `ErrConflict` (409) on version mismatch; rolls back
entirely on any append failure.

Without `Commit`, the event-ingestion pipeline cannot atomically write a
state-transition entity and its audit trail entry. It currently makes two
independent HTTP calls with no rollback on partial failure.

### 2.8 `queryfy` at v0.2.0, olu dependency at v0.3.1

**Severity: medium.**

`go.mod` pins `queryfy` at `v0.2.0`. olu uses `v0.3.1`. The public API
of `queryfy.Validate`, `queryfy.ValidateWithMode`, and `queryfy.Schema`
must be verified against the new version. The schema compilation change
introduced in v0.3.1 (25–46% performance improvement) is the most likely
source of a breaking change to `internal/validation/schemas.go`.

---

## 3. Remediation Stages

### Stage overview

| Stage | Name | Prerequisite | Completed in |
|---|---|---|---|
| 1 | Client foundation | — | v0.1.0-patched1 |
| 2 | Auth and connectivity | Stage 1 | v0.1.0-patched2 |
| 3 | API correctness | Stage 2 | v0.1.0-patched3 |
| 4 | Dependency upgrade | Stage 1 | v0.1.0-patched4 |
| 5 | New capabilities | Stage 3 | v0.1.0-patched5 |
| 6 | Test suite repair | Stages 3, 5 | v0.1.0-patched6 |

Stage 4 may be executed in parallel with Stage 3 if a separate branch is
used. All other ordering constraints are strict: downstream stages depend
on the client shape established by their prerequisites.

---

### Stage 1 — Client foundation

**Addresses:** §2.1, §2.2  
**Version:** v0.1.1

#### 1a. Fix entity response unmarshalling

Rewrite the response-parsing logic in all five CRUD methods.

`Create` response: unmarshal `{"message":"…","id":N}`. Return an `Entity`
with only `ID` populated; `Data` is not available after create (olu does
not echo the document back). Callers that need the document must issue a
subsequent `Get`.

`Get` response: unmarshal the flat document directly into `Entity.Data`.
The `"id"` field is present in the document; extract it for `Entity.ID`.

`Update` / `Patch` response: unmarshal `{"message":"…"}`. Return the same
`Entity` passed in (ID known from the call; Data unchanged) or a minimal
entity with only ID. Callers that need the updated document must re-fetch.

`List` response: unmarshal the `PagedResponse` envelope
`{"data":[…],"pagination":{"page":N,"per_page":N,"total_items":N,"total_pages":N}}`.
The `ListParams` struct should be extended to expose the pagination
metadata to callers. Add `ListResult` as the return type, holding both
the entity slice and the pagination fields.

#### 1b. Unify `OQLResult`

- Delete `auth.OQLResult` from `internal/auth/store_olu.go`.
- Update the `OLUClient` interface in `internal/auth/store_olu.go` to
  return `*olu.OQLResult`.
- Remove the `Rows: result.Data` bridge from `internal/server/server.go`.
- Update all callers in `internal/auth/store_olu.go` that reference
  `.Rows` to use `.Data`.

---

### Stage 2 — Auth and connectivity

**Addresses:** §2.3, §2.4  
**Prerequisite:** Stage 1  
**Version:** v0.1.2

#### 2a. Add olu credential to `OLUConfig`

Add `APIKey string` to `internal/config/config.go` `OLUConfig`. Expose a
`WithAPIKey(key string)` option on `New()`. In `do()`, set
`Authorization: Bearer <key>` when a key is present. The config key in
`config.json` is `olu.api_key`; the environment variable is
`SEAM_OLU_API_KEY`.

Update `config.example.json` to document the field.

#### 2b. Fix search endpoint

Change `Search()` from `GET /{entity}/search?q=…` to
`GET /search?q=…&entity={entity}`. The entity parameter is now optional
— pass it when the caller supplies one, omit it otherwise. Update the
`SearchParams` struct to reflect this: `Entity string` replaces the
positional `entity` argument.

---

### Stage 3 — API correctness

**Addresses:** §2.5, §2.6  
**Prerequisite:** Stage 2  
**Version:** v0.1.3

#### 3a. Fix `GraphNeighbors`

Replace `GET /graph/neighbors/{id}?depth=N` with `POST /graph/neighbors`
and a JSON body:

```json
{"node_id": "assets:42", "direction": "out"}
```

`direction` is `"out"`, `"in"`, or `"both"`. Default `"out"`.

Response:

```json
{
  "neighbors": {
    "outgoing": {"locations:5": "location_ref"},
    "incoming": {"events:12": "asset_ref"}
  }
}
```

Update the method signature to accept `direction string` and return
`map[string]map[string]string` (direction → neighbour → label).

#### 3b. Fix `GraphShortestPath`

Replace `GET /graph/shortest-path?from=…&to=…` with
`POST /graph/shortestPath` and a JSON body:

```json
{"from": "assets:1", "to": "locations:5", "max_depth": 10}
```

Response:

```json
{
  "from": "assets:1",
  "to": "locations:5",
  "exists": true,
  "path": ["assets:1", "locations:5"],
  "length": 1
}
```

Return a `PathResult` struct instead of `[]string`, exposing `Exists`,
`Path`, and `Length`.

#### 3c. Rename `Sulpher` to `GraphQuery`, fix endpoint

Rename the method from `Sulpher` to `GraphQuery`. Change endpoint from
`POST /graph/sulpher` to `POST /graph/query`. Request body:

```json
{"query": "FIND PATHS FROM assets:1 TO locations:5", "max_depth": 5}
```

Response uses `"result"` (not `"data"`):

```json
{
  "status": "completed",
  "result": […],
  "stats": {"nodes_traversed": 12, "paths_found": 2, "execution_time_ms": 3}
}
```

Update `SulpherResult` → `GraphQueryResult`, `SulpherStats` →
`GraphQueryStats` accordingly.

#### 3d. Fix `tenantID` type handling

Add `WithTenantID(id uint16) ClientOption` that formats the tenant
identifier as 4-digit uppercase hex before storing it:

```go
func WithTenantID(id uint16) ClientOption {
    return WithTenant(fmt.Sprintf("%04X", id))
}
```

Deprecate the existing `WithTenant(string)` option with a comment
requiring callers to supply a pre-formatted `XXXX` string. Add a
`validateTenantID(s string) error` helper that enforces the format in
`buildURL` (or panics on construction — to be decided).

---

### Stage 4 — Dependency upgrade

**Addresses:** §2.8  
**Prerequisite:** Stage 1 (for a stable build baseline)  
**Version:** v0.1.4 (may be cut concurrently with Stage 3)

Update `go.mod` to `queryfy v0.3.1`. Run `go mod tidy`. Build.

If `DynamicFieldSchema` in `internal/validation/schemas.go` no longer
compiles, the schema compilation API has changed. Inspect the queryfy
v0.3.1 release notes and update accordingly. The most likely change is
that `queryfy.Schema` is now a compiled type requiring a compile-or-build
step rather than a plain map-like value; if so, `DynamicFieldSchema` must
call the new compile function.

Run the full test suite. The validation tests in
`internal/validation/schemas_test.go` should be the first to surface any
API incompatibility.

---

### Stage 5 — New capabilities

**Addresses:** §2.7  
**Prerequisite:** Stage 3  
**Version:** v0.1.5

#### 5a. Add `Save`

```go
func (c *Client) Save(ctx context.Context, entity string, id int64,
    data map[string]any) (created bool, err error)
```

Calls `POST /{entity}/save/{id}` with `data` as the body. olu returns
`{"created": bool, "id": N}`. Surface the `created` flag to the caller.

Use case: idempotent device registration — the caller supplies the hardware
ID; olu creates on first call, replaces on subsequent calls.

#### 5b. Add `Commit`

```go
type CommitUpdate struct {
    Entity  string         `json:"entity"`
    ID      int64          `json:"id"`
    Version *int           `json:"version,omitempty"` // nil = unconditional
    Data    map[string]any `json:"data"`
}

type CommitAppend struct {
    Entity string         `json:"entity"`
    ID     *int64         `json:"id,omitempty"` // nil = auto-assign
    Data   map[string]any `json:"data"`
}

type CommitRequest struct {
    Update CommitUpdate   `json:"update"`
    Append []CommitAppend `json:"append"`
}

type CommitResult struct {
    Update struct {
        Entity  string `json:"entity"`
        ID      int64  `json:"id"`
        Created bool   `json:"created"`
        Version int    `json:"version"`
    } `json:"update"`
    Appended []struct {
        Entity string `json:"entity"`
        ID     int64  `json:"id"`
    } `json:"appended"`
}

func (c *Client) Commit(ctx context.Context, req CommitRequest) (*CommitResult, error)
```

Calls `POST /oql/commit`. Returns `ErrConflict` (wrapping the 409
response) when the optimistic version check fails. Returns
`ErrAlreadyExists` (wrapping 409 with `already_exists` code) when an
append entry uses an explicit ID that already exists.

Maximum 25 entries in `Append` — this limit is enforced by olu and
surfaces as a 400.

---

### Stage 6 — Test suite repair

**Addresses:** all of the above  
**Prerequisite:** Stages 3, 5  
**Version:** v0.1.6

The existing `internal/olu/client_test.go` tests pass against mock
servers that reflect the *old* API shapes. After Stages 1–3 they will
fail by design. Rewrite the full test file with mock servers that reflect
current olu response shapes.

Required changes per method:

| Method | Mock currently returns | Must return after fix |
|---|---|---|
| `TestCreate` | `{"id":1,"data":{…}}` | `{"message":"…","id":1}` |
| `TestGet` | `{"id":1,"data":{…}}` | flat doc `{"id":1,"name":"…"}` |
| `TestUpdate` | `{"id":1,"data":{…}}` | `{"message":"…"}` |
| `TestPatch` | `{"id":1,"data":{…}}` | `{"message":"…"}` |
| `TestList` | bare array `[{…}]` | `PagedResponse` envelope |
| `TestSearch` | bare array `[{…}]` | `{"query":…,"count":N,"results":[…]}` |
| `TestOQL` | `{"status":"…","data":[…]}` | verify stats fields present |
| `TestSulpher` | `{"data":[…]}` | rename to `TestGraphQuery`; use `{"status":"completed","result":[…],"stats":{…}}` |

New tests to add:

- `TestSave` — mock `POST /{entity}/save/{id}`, verify `created` bool
- `TestCommit` — mock `POST /oql/commit`, verify result and conflict path
- `TestGraphNeighbors` — mock `POST /graph/neighbors`, verify direction
  parameter and response parsing
- `TestGraphShortestPath` — mock `POST /graph/shortestPath`, verify both
  found and not-found paths
- `TestAuthHeader` — verify `do()` sets `Authorization: Bearer` when
  `APIKey` is configured; verify it is absent when not configured
- `TestWithTenantID` — verify `%04X` formatting of uint16 values

Coverage target for `internal/olu`: ≥ 85% (currently 74.2%).

---

## 4. Files affected

| File | Stages |
|---|---|
| `internal/olu/client.go` | 1, 2, 3, 5 |
| `internal/olu/client_test.go` | 6 |
| `internal/auth/store_olu.go` | 1 |
| `internal/server/server.go` | 1 |
| `internal/config/config.go` | 2 |
| `config.example.json` | 2 |
| `go.mod` | 4 |
| `internal/validation/schemas.go` | 4 (conditional) |
| `CHANGELOG.md` | each stage |

---

## 5. Actual outcomes vs plan

All six stages were implemented as designed with the following notes:

**Stage 4 (queryfy upgrade):** No source changes were required. The APIs
used by `internal/validation/schemas.go` are unchanged between v0.2.0 and
v0.3.1. The upgrade was a `go.mod` change only.

**Stage 6 coverage:** `internal/olu` reached 85.1% coverage (target ≥ 85%).
The `Sulpher` deprecated alias is excluded from coverage; all substantive
methods are tested. The 0% packages noted at the start of the project
(`internal/server`, `internal/ui`) remain untested — this is expected and
tracked in ROADMAP.md under Technical Debt.

---

## 6. Out of scope

The following issues are known but are not addressed by this plan:

- `internal/server` has no tests. A separate testing initiative is
  warranted once the client is correct.
- `internal/ui` has no tests. UI testing strategy is deferred.
- `internal/auth` coverage is 56%. Improved coverage for the audit, reset,
  and CSRF paths is deferred to a dedicated auth hardening pass.
- The `Commit` operation (Stage 5b) enables atomic event ingestion.
  Refactoring the existing event-ingestion handler to use it is deferred
  to a subsequent feature branch, not bundled into this remediation.
