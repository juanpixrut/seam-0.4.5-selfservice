# Seam AMS — FSM Design and Integration

> **Reconciliation banner, 2026-07-28 (see docs/RESOLVED.md, T-12):**
> Decided — xolu's native `/fsm` primitive (server-side, atomically
> committed with data, stable as of xolu v0.16.16) is the runtime FSM
> engine for Seam AMS, superseding this document's §5 target design
> (migrating `internal/fsm` onto `fsm-toolkit` as the runtime). `fsm-toolkit`
> may still be imported separately for a graphical/TUI FSM designer
> (design-time tooling, not runtime) — undecided, not urgent, not a
> conflict with the above. Content below is retained for its domain
> examples and per-state metadata ideas (§5.2, §6), which may still inform
> whatever eventually authors FSM definitions for xolu's `/fsm`, but §5's
> storage-format-migration plan itself is superseded and should not be
> executed as written.

**Version:** 1.4  
**Author:** haitch <h@ual.fi>  
**Date:** March 2026  
**Status:** Active design document — implementation partially complete, extension pending

**Changelog:**
- 1.4 — Provisional (24h): OQL-as-action-executor and simplified architecture.
- 1.3 — Documented tsqlparser decision; closed Q7.1 and Q7.3.
- 1.2 — Closed open question 7.1: tsqlparser via `SELECT @FSM WHERE` wrapper
  as the guard expression parser (section 5.3.2).
- 1.1 — Added transition actions (section 5.6), vending machine domain
  example, parallel bundle model (section 5.7), Commit-as-executor-primitive
  execution model, updated open questions.
- 1.0 — Initial version.

---

## 1. Purpose

This document describes how Finite State Machines are used within Seam AMS:
what they model, how they are stored, validated, executed, and extended.
It is the authoritative reference for all FSM-related decisions and serves
as the upstream source of truth for any feature requests made to external
libraries (specifically fsm-toolkit).

---

## 2. What FSMs model in Seam AMS

Assets tracked by Seam AMS exist in operational states that change in
response to sensor events, scheduled inspections, location updates, and
manual interventions. The rules governing those changes — what conditions
trigger a state change, what the valid next states are, and what effects
must follow a transition — are domain-specific but structurally uniform
across asset types.

An FSM assigned to an asset type defines:

- The complete set of named operational states the asset can be in
- Which state is the initial state when the asset is first commissioned
- Which transitions are possible between states
- The **guard** that must be satisfied for each transition to fire
- The **actions** to execute atomically when a transition fires
- Per-state metadata (label, colour, alert behaviour) that downstream
  systems use without requiring domain knowledge of the asset type

Examples across domains that Seam AMS is designed to serve:

| Asset type | States | Example guard | Example action |
|---|---|---|---|
| Refrigerated unit | normal, temperature_warning, temperature_critical, door_open, power_loss, fault | `value.temperature > 5.0` | `outbox(notify_operator, "Temperature alert")` |
| Fire extinguisher | commissioned, due_inspection, overdue, in_inspection, passed, failed, condemned | `calendar.days_since(asset.last_inspection_date) >= 365` | `asset.set(next_inspection_due, calendar.add_days(today, 365))` |
| Cattle (individual) | on_farm, in_transit, at_market, quarantine, treatment, slaughter_eligible, deceased | `value.temperature_celsius > 39.5 \|\| value.respiration_rate > 40` | `append(quarantine_log, {asset_id, timestamp, reason})` |
| HGV (truck) | available, driving, rest, loading, unloading, maintenance, border_hold, infraction | `asset.tachograph_continuous_driving_minutes >= 270` | `outbox(notify_driver, "Mandatory rest required")` |
| Pressure vessel | nominal, inspection_due, pressure_anomaly, depressurising, decommissioned | `value.pressure_bar > asset.max_rated_bar * 1.05` | `outbox(notify_safety_officer, "Pressure anomaly")` |
| Soda vending machine | stocked, low, empty (inventory) + normal, temp_warning, temp_critical, fault (temperature) | `asset.can_count > 0` | `asset.increment(can_count, -1)` |

The soda vending machine row is significant: it demonstrates that a single
physical asset has **orthogonal concerns** — inventory state and temperature
state are independent and must not be cross-producted into a single machine.
See section 5.7 for the parallel machine model.

The key architectural property is that the FSM executor must be able to
evaluate guards and execute actions **without domain knowledge of the
asset type**. All domain context must travel with the machine definition.

---

## 3. Current implementation

### 3.1 Storage

FSM machine definitions are stored in xolu as `fsm_machines` entities.
The current storage format is Seam AMS's own JSON schema, described in
section 3.3. Each machine is assigned to an asset type via the `fsm_id`
field on the `asset_types` entity.

The current asset FSM state is stored in the `fsm_state` field on each
`assets` entity. All state transitions are recorded in the `fsm_transitions`
entity with `asset_id`, `from_state`, `to_state`, `event_id`, and
`timestamp` — written atomically with the state update via `xolu.Commit`.

### 3.2 Execution

The FSM evaluator lives in `internal/fsm/evaluator.go`. It is instantiated
at server startup and populated with machine definitions loaded from xolu.
Machines are loaded into the evaluator's in-memory registry when created
or updated via the API.

Evaluation is triggered from the event ingestion pipeline:

1. `POST /api/v1/events` receives an event for an asset
2. The event document is created in xolu and the full document re-fetched
3. `processEvent` extracts `asset_id` from the event, fetches the asset
4. If the asset has an `fsm_id`, `evaluateFSM` is called
5. The evaluator checks all transitions out of the current state in
   priority order (highest `priority` value first)
6. The first transition whose guard evaluates to true is fired
7. The executor builds a `CommitRequest` from the transition's action list
   (see section 5.6) and calls `xolu.Commit`, which atomically writes:
   - The new `fsm_state` on the asset
   - Any asset field mutations declared in the action list
   - The `fsm_transitions` audit record
   - Any additional entity appends declared in the action list
   - Any outbox entries for deferred notifications

Evaluation runs asynchronously in a goroutine. A failed evaluation
does not fail the event ingestion request.

**Note on actions in the current implementation:** The current evaluator
does not yet support transition actions. Step 7 currently writes only the
state change and the audit record. The full action model is described in
section 5.6 and is pending implementation.

### 3.3 Current machine storage format

The current JSON format stored in xolu is Seam AMS's own schema. It differs
from the fsm-toolkit normalised JSON format. The evaluator's `Machine`
struct is populated from this format by `loadMachineIntoEvaluator` in
`internal/handlers/api/fsm.go`.

```json
{
  "name": "Refrigerated Unit Monitor",
  "description": "Temperature, door, and power state machine for cold chain assets",
  "initial": "normal",
  "states": {
    "normal": {
      "name": "normal",
      "label": "Normal Operation",
      "color": "#22c55e",
      "is_initial": true,
      "is_final": false,
      "transitions": [
        {
          "to": "temperature_warning",
          "priority": 10,
          "condition": {
            "type": "value_gt",
            "field": "temperature",
            "threshold": 5.0
          }
        },
        {
          "to": "door_open",
          "priority": 5,
          "condition": {
            "type": "value_eq",
            "field": "door_state",
            "target": "open"
          }
        }
      ]
    },
    "temperature_warning": {
      "name": "temperature_warning",
      "label": "Temperature Warning",
      "color": "#f59e0b",
      "is_final": false,
      "transitions": [
        {
          "to": "normal",
          "priority": 10,
          "condition": {
            "type": "value_lt",
            "field": "temperature",
            "threshold": 4.5
          }
        },
        {
          "to": "temperature_critical",
          "priority": 20,
          "condition": {
            "type": "value_gt",
            "field": "temperature",
            "threshold": 8.0
          }
        }
      ]
    }
  }
}
```

### 3.4 Current condition types

The evaluator supports the following condition types, validated by
`FSMConditionSchema` in `internal/validation/schemas.go`:

| Type | Description | Required fields |
|---|---|---|
| `value_gt` | `event.value[field] > threshold` | `field`, `threshold` |
| `value_lt` | `event.value[field] < threshold` | `field`, `threshold` |
| `value_eq` | `event.value[field] == target` | `field`, `target` |
| `value_in` | `event.value[field]` is in list | `field`, `values` |
| `duration_exceeded` | time since last event in this state > duration | `duration` |
| `trigger_type` | `event.trigger_type == trigger` | `trigger` |
| `absent` | no event received for duration | `duration` |

Compound conditions (`and`/`or`) are defined in `internal/fsm/evaluator.go`
as `ConditionAnd` and `ConditionOr` but are not yet exposed in the validation
schema or API. Duration values are strings (`"5m"`, `"1h"`, `"30s"`).

### 3.5 Gaps and limitations of the current format

The current format has several limitations that motivate the migration
described in section 5:

1. **No interoperability.** The format is Seam AMS-specific. Machines
   cannot be authored, edited, or validated with external tooling.

2. **States as a map, not a list.** xolu stores the `states` field as a
   JSON object keyed by state name. Order is not preserved.

3. **Conditions do not travel cleanly.** The `Condition` struct couples
   evaluation logic tightly to the Seam AMS evaluator.

4. **No structural validation before storage.** Invalid machines can be
   stored. The queryfy schema check does not understand FSM semantics.

5. **No analysis.** No mechanism to detect unreachable states, dead
   states, or ambiguous transitions before a machine goes live.

6. **Compound conditions not wired.** `and`/`or` are implemented in the
   evaluator but not in the validation schema or API.

7. **No transition actions.** Asset field mutations, entity appends, and
   deferred notifications must be hardcoded in the evaluator.

---

## 4. fsm-toolkit

Seam AMS depends on `github.com/ha1tch/fsm-toolkit` for FSM interchange
and validation. The toolkit is not vendored; it is fetched from the module
proxy at build time.

### 4.1 What the toolkit provides

**`pkg/fsm`** — Core FSM types (`FSM`, `Transition`, `State`, etc.),
structural validation (`Validate()`), and structural analysis (`Analyse()`).

**`pkg/fsmfile`** — Read/write support for the `.fsm` binary format (zip
containing `machine.hex`, `labels.toml`, optional `layout.toml` and
`classes.json`) and the normalised JSON interchange format (`ParseJSON`,
`ToJSON`). The two formats are fully round-trip equivalent.

The `.fsm` binary format is the authoring format, used by the `fsmedit`
TUI editor. The normalised JSON format is the interchange format, used
for storage and API exchange.

### 4.2 Normalised JSON format

Key structural properties of the fsm-toolkit normalised JSON format:

- `states` is a **list**, not a map. Order is preserved.
- `transitions` is a flat list at the top level, each with `from`, `input`,
  and `to`. Transitions are not nested under states.
- `alphabet` is an explicit list of all input symbols.
- `initial` is the name of the initial state.
- `accepting` is a list of accepting state names.
- `state_properties` carries per-state typed metadata via the class system.

**Pending upstream:** `transitions` entries will also carry `guard` (opaque
string, optional) and `actions` ([]string, optional) once the fsm-toolkit
team implements them. See `docs/FSM_TOOLKIT_FEATURE_REQUEST.md`.

From fsm-toolkit's perspective, `guard` is simply an opaque string field —
the toolkit has no involvement in the expression language. On the Seam
side, guard strings are T-SQL WHERE clause expressions evaluated via
`tsqlparser` (see section 5.3).

### 4.3 Validation pipeline

Before any machine is stored in xolu, Seam AMS applies the following
validation pipeline:

```
raw input (JSON or .fsm bytes)
    ↓
fsmfile.ParseJSON / fsmfile.ReadFSMBytes
    ↓
fsm.FSM struct
    ↓
f.Validate()            — hard gate: structural correctness
                          error → reject with 422
    ↓
f.Analyse()             — soft gate: design quality warnings
                          []ValidationWarning → store as analysis_warnings,
                          return to caller in API response
    ↓
SeamValidate(f)        — Seam-specific gate: see section 4.4
                          error → reject with 422
    ↓
fsmfile.ToJSON(f, true) — normalised JSON
    ↓
xolu.Create / xolu.Update on fsm_machines entity
```

#### `f.Validate()` checks (hard gate)

- At least one state exists; initial state declared and present
- All accepting states present in the states list
- All transition `from`/`to` values reference declared states
- All transition `input` values exist in the alphabet
- No epsilon transitions in a DFA
- Output values exist in declared output alphabets (Moore/Mealy)

#### `f.Analyse()` warnings (soft gate)

| Warning type | Meaning |
|---|---|
| `unreachable` | States not reachable from the initial state |
| `dead` | Non-accepting states with no outgoing transitions |
| `nondeterministic` | DFA states with multiple transitions on same input |
| `incomplete` | DFA states missing transitions for some alphabet symbols |
| `unused_input` | Alphabet symbols used in no transition |
| `unused_output` | Output symbols used in no transition or state |

### 4.4 Seam-specific validation (`SeamValidate`)

- At least one transition has a guard expression
- Guard expressions are non-empty strings
- Action strings reference verbs from the registered executor vocabulary
  (unknown verbs generate a warning, not an error — forward compatibility)
- State properties conform to the `seam_state` class schema (section 5.2)

---

## 5. Target design (pending implementation)

### 5.1 Storage format migration

1. Add `go get github.com/ha1tch/fsm-toolkit` to `go.mod`
2. Implement the validation pipeline (section 4.3)
3. Implement `SeamMachineFromFSM` bridge (section 5.4)
4. Add `POST /api/v1/fsm/upload` endpoint for `.fsm` file upload
5. Migrate existing `fsm_machines` entities to the new format
6. Remove the old `loadMachineIntoEvaluator` parsing logic

### 5.2 Per-state domain metadata — `seam_state` class

Stored in `state_properties` using the fsm-toolkit class system. Enables
the executor to act on state entry without Seam domain knowledge.

| Property | Type | Description |
|---|---|---|
| `alert` | `bool` | Whether entering this state should raise an alert |
| `alert_severity` | `[40]string` | Severity: `low`, `medium`, `high`, `critical` |
| `alert_message` | `string` | Human-readable message for the alert |
| `operational` | `bool` | Whether the asset is operational in this state |
| `requires_action` | `bool` | Whether a human action is required to leave this state |
| `display_label` | `[40]string` | Short label for UI display |
| `display_color` | `[40]string` | Hex colour for UI display, e.g. `#ef4444` |

### 5.3 Transition guards

**Decision — guard expression language: T-SQL via tsqlparser (settled)**

Guard expressions are T-SQL WHERE clause expressions, parsed using
`github.com/ha1tch/tsqlparser`. This was evaluated against Google CEL
(`github.com/google/cel-go`). Summary of the comparison:

| Aspect | CEL | T-SQL (tsqlparser) |
|---|---|---|
| Simple comparisons, AND/OR, arithmetic | Identical | Identical |
| String equality | `==` double-quotes | `=` single-quotes |
| Null check | `!= null` | `IS NOT NULL` |
| Date arithmetic | `timestamp(x).timeSince() >= duration("8760h")` | `calendar.days_since(x) >= 365` |
| Duration | `duration_in_state > duration("30m")` | `duration_in_state > 1800` |
| Range | `x >= 2.0 && x <= 5.0` | `x BETWEEN 2.0 AND 5.0` |
| Collection aggregate | `arr.exists(w, w > threshold)` ✓ cleaner | `EXISTS (SELECT 1 FROM arr w WHERE w > threshold)` |
| Calendar day | `timestamp(now()).getDayOfWeek("UTC") != 0` | `calendar.day_of_week != 0` |
| Go version requirement | 1.23+ | 1.22 (current) |
| Already in project | No — new dependency | Yes — tsqlparser v0.5.2 |

T-SQL is materially more readable for the date, duration, and calendar
patterns that dominate the target domains (logistics, agriculture, fleet
management). CEL's only genuine advantage — collection aggregate syntax —
is adequately served by the SQL `EXISTS` subquery form. The toolchain
version bump and new dependency weight of CEL are not justified by the gain.

The formal definition of the guard language is: any expression that parses
without errors when wrapped as `SELECT @FSM WHERE <expr>` via tsqlparser.
This closes open questions 7.1 and 7.3.

The fsm-toolkit `Transition` struct currently has no guard field. Pending
the upstream addition, Seam AMS uses a stopgap convention: the `input`
symbol encodes both the event type and the guard expression separated by
`::`:

```
sensor_reading::value.temperature > 5.0
```

The executor splits on `::`, uses the left side for alphabet matching and
the right side as the guard expression. Once the toolkit adds a proper
`guard` field, this convention will be replaced.

Guard expressions are evaluated using `github.com/ha1tch/tsqlparser` via
the `SELECT @FSM WHERE` wrapper (see section 5.3.2). The `@FSM` sentinel
is discarded; only the WHERE clause expression is retained for evaluation.

Guard expressions are evaluated against an `EvaluationContext` containing:

- `value.*` — fields from `event.value`
- `asset.*` — fields from the current asset document in xolu
- `calendar.*` — date/time utilities (`today`, `days_since`, `day_of_week`)
- `duration_in_state` — seconds since entering the current state
- `event.consecutive_count` — consecutive matching events (debounce)

#### 5.3.1 Guard expression pattern taxonomy

| Pattern | Example | Notes |
|---|---|---|
| Simple numeric threshold | `value.temperature > 5.0` | |
| String equality | `value.door_state == "open"` | |
| Range AND | `value.temperature <= 8.0 && value.temperature > 5.0` | |
| Boolean OR | `value.pressure_bar < asset.min \|\| value.pressure_bar > asset.max` | |
| Null check | `value.next_due_date != null` | |
| Set membership (literal) | `value.damage_type in ["corrosion", "impact"]` | |
| Set membership (asset) | `value.geofence_id not in asset.permitted_geofence_ids` | Set from asset doc |
| Computed threshold | `value.weight_kg < asset.breed_min_weight_kg * 0.8` | Arithmetic on threshold |
| Date arithmetic | `calendar.days_since(asset.last_inspection_date) >= 365` | Function call |
| Date comparison | `value.cert_expiry > calendar.today` | |
| Duration in state | `duration_in_state > 1800` | Machine runtime variable |
| Duration from asset | `duration_in_state >= asset.treatment_withdrawal_period_seconds` | |
| Sequence counter | `event.consecutive_count >= 3` | Debounce |
| Asset-only | `asset.age_days >= asset.breed_min_slaughter_age_days` | No event field |
| Intra-event comparison | `value.gross_weight_kg >= value.manifest_declared_weight_kg * 0.95` | |
| Collection aggregate | `value.axle_weights.any(w => w > asset.max_axle_weight_kg)` | Lambda — tier 2 |
| Calendar day | `calendar.day_of_week != 0` | |
| Inventory check | `asset.can_count > 0` | Asset variable |

The collection aggregate pattern has no T-SQL lambda syntax equivalent.
Use `EXISTS (SELECT 1 FROM value.array_field alias WHERE alias > threshold)`
instead, which parses as `*ast.ExistsExpression` and allows the evaluator
to iterate the array field using the subquery's WHERE clause as the
per-element predicate. This is idiomatic T-SQL and unambiguous.

#### 5.3.2 Parsing guard expressions with tsqlparser

Guard expressions are parsed by wrapping them in a sentinel SELECT:

```go
func ParseGuardExpression(expr string) (ast.Expression, []string) {
    program, errors := tsqlparser.Parse("SELECT @FSM WHERE " + expr)
    if len(errors) > 0 {
        return nil, errors
    }
    sel := program.Statements[0].(*ast.SelectStatement)
    return sel.Where, nil
}
```

The `@FSM` sentinel is a T-SQL variable token. It is discarded after
parsing — `sel.Where` is the bare `ast.Expression` with no SELECT noise.
The sentinel is intentionally named `@FSM` rather than a literal (`1`,
`NULL`, etc.) so that:

- Any logging or debugging output is immediately identifiable as an FSM
  guard expression rather than an anonymous SQL fragment
- The parse site is grep-able: `SELECT @FSM WHERE` appears nowhere in
  normal T-SQL and can be found in one search
- Intent is self-documenting in the source

The `sel.Where` expression is then evaluated against the
`EvaluationContext` by an AST walker that dispatches on node type:

| AST node type | Evaluation |
|---|---|
| `*ast.InfixExpression` | Evaluate left and right, apply operator (`>`, `<`, `>=`, `<=`, `=`, `<>`, `AND`, `OR`, `+`, `-`, `*`, `/`, `%`) |
| `*ast.PrefixExpression` | Evaluate operand, apply `NOT` or unary `-` |
| `*ast.InExpression` | Evaluate left; check membership in list or subquery result |
| `*ast.BetweenExpression` | Evaluate subject; check lower ≤ subject ≤ upper |
| `*ast.IsNullExpression` | Evaluate expression; check null/not-null |
| `*ast.ExistsExpression` | Evaluate inner SELECT against the context; true if any row matches |
| `*ast.QualifiedIdentifier` | Resolve dotted path (`value.temperature`, `asset.can_count`, `calendar.today`) against context |
| `*ast.FunctionCall` | Dispatch to registered function handlers (`calendar.days_since`, etc.) |
| `*ast.IntegerLiteral`, `*ast.FloatLiteral` | Return numeric value |
| `*ast.StringLiteral` | Return string value |
| `*ast.NullLiteral` | Return nil |

### 5.4 Bridge: fsm-toolkit `FSM` → Seam evaluator `Machine`

`SeamMachineFromFSM(f *fsmfile.FSM) (*fsm.Machine, error)` converts a
validated `fsm.FSM` to the Seam evaluator's `Machine` struct:

- `f.Name`, `f.Description`, `f.Initial` → `Machine` fields directly
- Each state → `Machine.States[name]` with `state_properties` mapped to
  `State.Label`, `State.Color`, `State.IsFinal` (from `operational`)
- Each transition → `Transition` on source state, with guard string parsed
  by `ParseGuardExpression` into a `Condition` struct, and action list
  stored as `Transition.Actions []string`

Guard parsing: simple patterns map to existing `ConditionType` values;
compound `&&`/`||` patterns map to `ConditionAnd`/`ConditionOr`;
unrecognised patterns stored as `ConditionRaw` (pending full expression
evaluator).

### 5.5 Upload endpoint for `.fsm` files

`POST /api/v1/fsm/upload` accepts a multipart form with a `.fsm` file:

1. `fsmfile.ReadFSMBytes` — parse the zip
2. Full validation pipeline (section 4.3)
3. `fsmfile.ToJSON` — normalised JSON
4. Store as `fsm_machines` entity in xolu
5. Return entity with analysis warnings

### 5.6 Transition actions

When a transition fires, the executor must often do more than advance
the FSM state. The `actions` field on a `Transition` is an ordered list
of opaque strings that the executor interprets as instructions for
building a single `xolu.Commit` call.

#### 5.6.1 The Commit-as-executor-primitive model

Every side effect that must occur atomically with a state transition is
expressed as a contribution to a single `xolu.Commit` call. The executor
accumulates contributions as it evaluates the action list, then fires one
Commit that writes everything — or nothing — together.

```
Transition fires
    ↓
CommitRequest.update = {entity: "assets", id: N,
                        data: {fsm_state: new_state}}
    ↓
For each action string in Transition.actions:
    asset.set(field, expr)      → add field to update.data
    asset.increment(field, n)   → add computed field to update.data
    asset.append_list(field, v) → add to update.data list field
    asset.clear(field)          → add null field to update.data
    append(entity, data)        → add entry to update.append
    timeseries(metric, value)   → add timeseries append via xolu Pebble
    outbox(verb, payload)       → add entry to pending_notifications append
    ↓
xolu.Commit(CommitRequest)       — single atomic write
    ↓
pending_notifications worker delivers outbox entries asynchronously
```

The `fsm_transitions` audit record is always appended implicitly and is
not declared in the action list. If two actions write the same field, the
last one wins. The `xolu.Commit` 25-entry append limit applies; see open
question 7.7.

#### 5.6.1a Provisional alternative: OQL-as-action-executor (under evaluation)

> **Status:** Provisional — pending a response on ual `IN` support
> and `pkg/compute` extraction. Review within 24 hours of this version.

A simpler architecture is possible because xolu's OQL query layer is already
`tsqlparser` under the hood. This means:

- **Guards** are evaluated locally by the Seam executor using
  `SELECT @FSM WHERE <expr>` via tsqlparser — no xolu round-trip, fast,
  purely boolean
- **Actions** are OQL statements sent directly to xolu, with context
  variables interpolated by the executor before dispatch

Under this model, action strings in the machine definition are plain OQL:

```sql
UPDATE assets SET can_count = can_count - 1 WHERE id = @asset_id
INSERT INTO dispense_log (asset_id, dispensed_at) VALUES (@asset_id, @now)
UPDATE assets SET next_inspection_due = DATEADD(day, 365, @today) WHERE id = @asset_id
```

The executor's role for actions reduces to:
1. Interpolate context variables (`@asset_id`, `@now`, `@today`, etc.)
   into the action strings
2. Wrap the interpolated statements into a `CommitRequest` (one UPDATE
   becomes `update`, each INSERT becomes an `append`)
3. Fire `xolu.Commit` once

**What this eliminates:**

- The custom action vocabulary (`asset.set`, `asset.increment`, etc.)
- Any action expression parser in the executor
- Any action evaluator in the executor
- The separate string handling concern for actions

**What it gains:**

- Full OQL expression power for actions — dates, strings, arithmetic,
  subqueries, all already implemented and tested in xolu
- One language for both guard expressions (T-SQL WHERE subset) and action
  expressions (T-SQL UPDATE/INSERT subset) — consistent authoring experience
- The same parser, already deployed, already battle-tested at 96% coverage
  on 203 real-world stored procedures

**What requires confirmation:**

- Whether xolu's Commit endpoint accepts OQL-derived mutations or only
  structured `{entity, id, data}` objects — if the latter, the
  executor must translate OQL SET clauses to field maps (straightforward
  with tsqlparser's UpdateStatement AST)
- The answer on ual `IN` support and compute block extraction, which
  determines whether the hybrid tsqlparser+ual approach for high-frequency
  numeric guards is worth pursuing

The two architectures are not mutually exclusive: OQL-as-action-executor
and the compiled ual closure fast path for numeric guards can coexist.

#### 5.6.2 Action vocabulary

Actions are a closed vocabulary. Unknown verbs generate a warning at
validation time and are skipped at execution time (forward compatibility).

| Verb | Signature | Effect |
|---|---|---|
| `asset.set` | `asset.set(field, expr)` | Sets field on asset to evaluated expr |
| `asset.increment` | `asset.increment(field, n)` | Adds n (may be negative) to numeric field |
| `asset.append_list` | `asset.append_list(field, expr)` | Appends value to list field |
| `asset.clear` | `asset.clear(field)` | Sets field to null |
| `append` | `append(entity, {fields})` | Appends record to any xolu entity |
| `timeseries` | `timeseries(metric, value)` | Appends point to asset timeseries (xolu Pebble) |
| `outbox` | `outbox(verb, payload)` | Appends entry to `pending_notifications` for async delivery |

**Not supported:** arbitrary code execution or arbitrary HTTP calls as
inline actions. All external dispatch goes through the outbox, which
provides delivery guarantees and prevents injection.

#### 5.6.3 Action expression context

Action expressions share the guard evaluation context (section 5.3),
plus:

- `new_state` — name of the state being entered
- `old_state` — name of the state being exited
- `transition_id` — unique identifier for this transition firing event

#### 5.6.4 Outbox pattern for notifications

The `outbox` verb appends a `pending_notifications` record inside the
same `xolu.Commit` as the state transition. A background worker polls
`pending_notifications` and delivers entries via the notification and
webhook systems. This is the transactional outbox pattern:

- The notification *intent* is committed atomically with the transition
- Delivery is asynchronous and retryable
- A rolled-back Commit produces no outbox entry and no spurious notification
- A delivered notification that preceded a crash will not be re-delivered
  (the outbox entry was already committed)

#### 5.6.5 Vending machine action example

Transition from `stocked` to `stocked` on `dispense` (not last can):

```
guard:   "asset.can_count > 1"
actions: [
  "asset.increment(can_count, -1)",
  "append(dispense_log, {asset_id, dispensed_at: event.timestamp})"
]
```

Transition from `stocked` to `empty` on `dispense` (last can):

```
guard:   "asset.can_count == 1"
actions: [
  "asset.set(can_count, 0)",
  "append(dispense_log, {asset_id, dispensed_at: event.timestamp})",
  "outbox(notify_restock, {asset_id, location: asset.location_id})"
]
```

### 5.7 Parallel machines (orthogonal concerns)

A single physical asset may have multiple independent state concerns.
The soda vending machine illustrates this clearly: inventory state
(`stocked`, `low`, `empty`) and temperature state (`normal`,
`temperature_warning`, `temperature_critical`, `fault`) are entirely
independent. Encoding both in a single FSM would require cross-producting
every inventory state with every temperature state (3 × 4 = 12 states),
and the explosion worsens with each additional concern.

#### 5.7.1 Bundle as packaging convention

The fsm-toolkit `.fsm` file format supports bundling multiple machines
into a single zip without requiring linking. `ValidateBundleLinks`
silently passes machines with no linked states. `ReadMachineFromBundle`
reads each machine independently by name.

Seam AMS uses bundles as a **packaging and transport convention**:
related machines governing the same asset type are bundled into a single
`.fsm` file and stored as a single `fsm_machines` entity in xolu.

#### 5.7.2 Parallel execution in the Seam executor

The toolkit's `BundleRunner` is sequential — one machine active at a
time via a delegation stack. It does not support concurrent execution
of unlinked machines. Seam AMS manages parallel dispatch in the
evaluator itself:

1. The `fsm_machines` entity lists the machine names in the bundle
2. For each incoming event, `evaluateFSM` iterates over all machines
   for the asset type
3. Each machine is evaluated independently against the event
4. All triggered transitions across all machines are accumulated into
   a single `CommitRequest`
5. One `xolu.Commit` writes all state changes and actions atomically

The asset document carries a state field per machine:
`fsm_state_inventory`, `fsm_state_temperature`, etc., keyed by machine
name. The `fsm_transitions` audit record includes a `machine_name` field.

#### 5.7.3 Cross-machine interaction

Parallel machines are intentionally isolated. Cross-machine interactions
are expressed through shared asset fields: one machine's action sets a
field that another machine's guard reads on the next event cycle. The
one-event-cycle delay is deliberate — it makes causality explicit and
prevents evaluation loops.

---

## 6. Domain examples

Complete example machines are maintained in `docs/examples/fsm/`.

| File | Domain | Machines | States | Guards | Actions |
|---|---|---|---|---|---|
| `fridge_cold_chain.json` | Refrigerated unit | 1 | 6 | temperature, door, power, duration | outbox notify |
| `fire_extinguisher.json` | Fire extinguisher | 1 | 7 | date arithmetic, credentials, set membership | set next due date, outbox notify |
| `cattle_tracking.json` | Cattle (individual) | 1 | 7 | computed thresholds, geofences, withdrawal periods | append quarantine log, set withdrawal deadline |
| `truck_hgv.json` | HGV (truck) | 1 | 8 | tachograph hours, axle weights, border crossing | outbox notify, append infraction log |
| `vending_machine.fsm` | Soda vending machine | 2 (unlinked) | 3 + 4 | inventory count, temperature | increment/decrement, outbox restock |

The vending machine bundle exercises the parallel execution model and
serves as the primary test case for multi-machine Commit accumulation.
The examples also serve as the reference corpus for the guard expression
parser and for the feature request to the fsm-toolkit team.

---

## 7. Open questions

| # | Question | Status |
|---|---|---|
| 7.1 | Guard/action expression language: custom evaluator or existing CEL/expr library? | **Resolved** — use `tsqlparser` via `SELECT @FSM WHERE` wrapper; see section 5.3.2 |
| 7.2 | Collection aggregate guards (`any`, `all`): include in initial implementation or defer? | Open |
| 7.3 | Guard and action expression syntax: define formally before implementation | **Resolved** — T-SQL WHERE clause subset via `SELECT @FSM WHERE` wrapper; see section 5.3 |
| 7.4 | fsm-toolkit `guard` and `actions` fields on `Transition`: pending upstream | Open — see `FSM_TOOLKIT_FEATURE_REQUEST.md`. Note: if OQL-as-action-executor (§5.6.1a) is adopted, `actions` strings are OQL statements; fsm-toolkit still carries them opaquely |
| 7.5 | Migration of existing `fsm_machines` entities to normalised JSON format | Open |
| 7.6 | Parallel bundle execution: file format supports unlinked machines; Seam executor manages dispatch | **Resolved** — section 5.7 |
| 7.7 | `xolu.Commit` 25-entry append limit: sufficient for all action lists? | Open — to discuss |
| 7.8 | `pending_notifications` outbox: worker design, polling interval, retry policy, dead letter | Open |
| 7.9 | OQL-as-action-executor (§5.6.1a): confirm whether xolu Commit accepts OQL-derived mutations or requires structured field maps; confirm on ual `IN` and `pkg/compute` extraction | **Provisional — 24h review** |

---

## 8. Related documents

- `docs/OLU_CLIENT_REMEDIATION.md` — olu client API corrections
- `docs/FSM_TOOLKIT_FEATURE_REQUEST.md` — abstract feature request for
  fsm-toolkit (to be written)
- `internal/fsm/evaluator.go` — current evaluator implementation
- `internal/validation/schemas.go` — FSM validation schemas
- `internal/handlers/api/fsm.go` — FSM API handlers
