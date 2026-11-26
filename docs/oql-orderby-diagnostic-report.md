# OQL `ORDER BY` investigation — raw diagnostic output

Environment: xolu v0.23.0, sqlite3 3.45.1, single-tenant deployment
(`t0000_*` tables), db at `/tmp/xolu-data/shared/store/xolu.db`. All
commands below were run directly against this instance; output is
pasted verbatim except where marked `[...]` for length.

---

## 1. Raw stored JSON, direct from the database file

**Finding the table name first, rather than guessing.** `t0000_nodes`
is not a guess — it's `SQLiteDialect.nodesTable()`'s documented default
in xolu's own source (`pkg/oql/sqlgen.go:130-135`):

```go
// NodesTable is the tenant-scoped blob node store table name (e.g. t0000_nodes).
// Set by the OQL executor from store.NodesTable() when the store implements
// storage.TableNamer. Defaults to "t0000_nodes" if unset.
NodesTable string
```

Confirmed against the actual running database, not assumed:

```
$ sqlite3 /tmp/xolu-data/shared/store/xolu.db ".tables"
schema_version      t0000_efts_content  t0000_graph         t0000_nfts_docsize
t0000_e_sch         t0000_efts_data     t0000_nfts          t0000_nfts_idx
t0000_edges         t0000_efts_docsize  t0000_nfts_config   t0000_nodes
t0000_efts          t0000_efts_idx      t0000_nfts_content  t0000_nseq
t0000_efts_config   t0000_eseq          t0000_nfts_data     tenants
```

Schema of `t0000_nodes`:

```
$ sqlite3 /tmp/xolu-data/shared/store/xolu.db ".schema t0000_nodes"
CREATE TABLE t0000_nodes (
            entity_type TEXT NOT NULL,
            id          INTEGER NOT NULL,
            data        TEXT NOT NULL,
            _version    INTEGER NOT NULL DEFAULT 1,
            created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            PRIMARY KEY (entity_type, id)
        );
CREATE INDEX idx_t0000_nodes_etype ON t0000_nodes(entity_type);
CREATE INDEX idx_t0000_nodes_updated ON t0000_nodes(updated_at);
```

**This is itself relevant**: `created_at`/`updated_at` genuinely exist,
but as native SQL columns on `t0000_nodes` — separate from anything
inside the `data` blob, and not exposed by OQL's `SELECT *` (which only
returns the blob's own JSON keys). `xolu.Client.List()`'s original
`Sort: "-created_at"` wasn't referencing nothing — it was referencing a
real column that exists at the storage layer, just not one OQL (or the
`List` HTTP endpoint, per §1 of the original report) surfaces.

Raw stored JSON of one record, exactly as requested:

```
$ sqlite3 /tmp/xolu-data/shared/store/xolu.db "SELECT data FROM t0000_nodes WHERE entity_type='audit_logs' LIMIT 1"
{"action":"","details":"","email":"hl@example.com","event_type":"login_success","id":1,"ip_address":"127.0.0.1","metadata":"","resource":"","success":true,"tenant_id":0,"timestamp":"2026-08-03T02:04:33Z","user_agent":"curl/8.5.0","user_id":1}
```

**This settles it**: the key inside the JSON blob is `timestamp`, not
`created_at`. Our own query (`ListAuditEvents`, seam-ui side) already
asks for `timestamp` — that part of our query was correct. The bug is
on your side, in what `ORDER BY <field>` does with a blob-only field
value once it's asked to sort on it (see §4).

---

## 2. The "no schema registered" claim — retracted as stated, replaced with what's actually verified

I did not have a boot log line making this claim. I said it as an
inference from an empty schema directory, and reported it as if it were
confirmed — that was wrong of me to present that way. What I've now
actually verified, directly:

Boot log (`/tmp/xolu.log`, `/tmp/xolu3.log`) — the only two lines
mentioning "schema" in either boot, full grep output, nothing omitted:

```
$ grep -i "schema" /tmp/xolu.log
  Schema directory:  /tmp/xolu-data/schema

$ grep -i "schema" /tmp/xolu3.log
  Schema directory:  /tmp/xolu-data/schema
```

That's a startup banner printing the *configured* schema directory
path — it makes no claim about whether anything is registered in it.

Directory listing, which is the actual evidence:

```
$ ls -la /tmp/xolu-data/schema/
total 8
drwxr-xr-x 2 root root 4096 Aug  3 02:03 .
drwxr-xr-x 4 root root 4096 Aug  3 02:03 ..

$ find /tmp/xolu-data -type f
/tmp/xolu-data/shared/store/xolu.db
```

The schema directory exists and is empty; the only file anywhere under
the data root is the sqlite db itself. So: no schema *files* exist in
this deployment. Whether that's the same thing as "no schema
registered" internally, I can't confirm from outside — I only have the
filesystem evidence above, not a log line asserting it.

---

## 3. Same pattern against a schema-registered entity

**Could not run this.** Per §2, no schema file exists anywhere in this
deployment, so there is no schema-registered entity to test against.
Not attempted with a substitute or a guess.

---

## 4. Dataset where insertion order and timestamp order genuinely differ

This is the decisive one, and it overturns part of what I reported
before — worth being direct about that.

**Setup**: read `id=1`'s original timestamp, then directly rewrote its
JSON blob (via `json_set` on the raw column, not through the app or
OQL) so it holds the *latest* timestamp of all ten records, deliberately
breaking any correlation between `id` order and `timestamp` order:

```
$ sqlite3 /tmp/xolu-data/shared/store/xolu.db "SELECT json_extract(data,'\$.timestamp') FROM t0000_nodes WHERE entity_type='audit_logs' AND id=1"
2026-08-03T02:04:33Z

$ sqlite3 /tmp/xolu-data/shared/store/xolu.db "UPDATE t0000_nodes SET data = json_set(data, '\$.timestamp', '2099-01-01T00:00:00Z') WHERE entity_type='audit_logs' AND id=1"

$ sqlite3 /tmp/xolu-data/shared/store/xolu.db "SELECT json_extract(data,'\$.timestamp') FROM t0000_nodes WHERE entity_type='audit_logs' AND id=1"
2099-01-01T00:00:00Z
```

(xolu was restarted between the write and the queries below, due to an
unrelated environment interruption on my end — the write is a direct,
committed sqlite change, durable independent of the server process. Not
a factor in the result.)

**`ORDER BY timestamp ASC`** — raw response, complete:

```
$ curl -s -X POST http://localhost:9090/api/v1/oql/query -H "Content-Type: application/json" \
    -d '{"query":"SELECT id, timestamp FROM audit_logs ORDER BY timestamp ASC"}'
{"data":[{"id":2,"timestamp":"2026-08-03T02:05:17Z"},{"id":3,"timestamp":"2026-08-03T02:05:25Z"},{"id":4,"timestamp":"2026-08-03T02:05:37Z"},{"id":5,"timestamp":"2026-08-03T02:43:02Z"},{"id":6,"timestamp":"2026-08-03T02:43:18Z"},{"id":7,"timestamp":"2026-08-03T02:43:41Z"},{"id":8,"timestamp":"2026-08-03T02:57:41Z"},{"id":9,"timestamp":"2026-08-03T03:00:00Z"},{"id":10,"timestamp":"2026-08-03T03:00:00Z"},{"id":1,"timestamp":"2099-01-01T00:00:00Z"}],"stats":{"execution_time_ms":2,"rows_affected":0,"rows_returned":10,"rows_scanned":10},"status":"completed"}
```

`id=1` (the manipulated, latest-valued row) is **last** — correct
ascending order by actual value. This directly closes the gap I left
open in the original report ("I couldn't distinguish ASC actually
working from ASC also silently no-opping but coincidentally matching
insertion order"). It wasn't coincidence: **`ORDER BY <field> ASC`
genuinely sorts by the field's real value**, confirmed by breaking the
coincidence deliberately and watching it still sort correctly.

**`ORDER BY timestamp DESC`** — raw response, complete:

```
$ curl -s -X POST http://localhost:9090/api/v1/oql/query -H "Content-Type: application/json" \
    -d '{"query":"SELECT id, timestamp FROM audit_logs ORDER BY timestamp DESC"}'
{"data":[{"id":1,"timestamp":"2099-01-01T00:00:00Z"},{"id":2,"timestamp":"2026-08-03T02:05:17Z"},{"id":3,"timestamp":"2026-08-03T02:05:25Z"},{"id":4,"timestamp":"2026-08-03T02:05:37Z"},{"id":5,"timestamp":"2026-08-03T02:43:02Z"},{"id":6,"timestamp":"2026-08-03T02:43:18Z"},{"id":7,"timestamp":"2026-08-03T02:43:41Z"},{"id":8,"timestamp":"2026-08-03T02:57:41Z"},{"id":9,"timestamp":"2026-08-03T03:00:00Z"},{"id":10,"timestamp":"2026-08-03T03:00:00Z"}],"stats":{"execution_time_ms":0,"rows_returned":10,"rows_scanned":10,"rows_affected":0},"status":"completed"}
```

This is a more specific and stranger bug than what I originally
reported. It is **not** "DESC is silently ignored" — `id=1`, the
correct single maximum, is placed first, exactly where DESC should put
it. But everything *after* that first row is in **ascending** order
(`2, 3, 4, ... 10`), not descending. My original hypothesis
("`ORDER BY timestamp DESC` behaves as if unsorted, falls back to
insertion order") is wrong as stated — it reacts to real data and gets
the first element right. What's actually broken looks narrower: DESC
correctly finds/places the extremum, then the remainder of the ordering
falls back to ascending rather than continuing descending.

One caveat I can't rule out from this data alone: ISO-8601 timestamp
strings sort correctly under plain lexicographic string comparison too,
so this test doesn't distinguish "real chronological sort" from
"correct lexicographic string sort" — both agree on every case above.
If that distinction matters for the fix, it'd need a value that sorts
differently as a timestamp than as a string to separate them; I didn't
construct one.

Reverted the manipulation afterward:

```
$ sqlite3 /tmp/xolu-data/shared/store/xolu.db "UPDATE t0000_nodes SET data = json_set(data, '\$.timestamp', '2026-08-03T02:04:33Z') WHERE entity_type='audit_logs' AND id=1"
$ sqlite3 /tmp/xolu-data/shared/store/xolu.db "SELECT json_extract(data,'\$.timestamp') FROM t0000_nodes WHERE entity_type='audit_logs' AND id=1"
2026-08-03T02:04:33Z
```

---

## Summary of what changed from the original report

- §1 (real field is `timestamp`, not `created_at`): confirmed, as
  originally reported — but now also confirms *why* `created_at`
  seemed plausible in the first place (it's a real column, just not a
  blob field, and not one OQL or `List` currently expose).
- §2: retracted as a "boot log claims X" statement. What's actually
  verified is a directory listing, not a log assertion — flagging that
  I shouldn't have blurred the two.
- §3: not run, no substitute attempted. No schema-registered entity
  exists in this deployment.
- §4: `ASC` is confirmed genuinely correct, not coincidental. `DESC` is
  a narrower, stranger bug than reported — gets the single extremum
  right, then the rest of the order isn't descending, it's ascending.
