# fsm-toolkit — Feature Request

**From:** haitch <h@ual.fi>  
**Date:** March 2026  
**Version:** 1.0  
**Affects:** `pkg/fsm`, `pkg/fsmfile`, normalised JSON format, `.fsm` file format

---

## Background

We are building an executor that drives physical assets through state
machines based on incoming event streams. We use fsm-toolkit as the
interchange and validation layer. During integration we identified two
capabilities that are standard in the Extended Finite State Machine (EFSM)
literature but are currently absent from the toolkit: **guards** and
**actions** on transitions.

On our side, we have resolved the guard expression language question: guard
strings are T-SQL-compatible boolean expressions parsed via our own
`tsqlparser` library. This is mentioned only to confirm that the expression
language is entirely opaque to fsm-toolkit — the toolkit has no need to
understand or constrain the syntax of guard strings. Any non-empty string
is a valid guard as far as the toolkit is concerned.

We also have a question about the intended semantics of bundles containing
unlinked machines.

---

## Request 1 — Guard on `Transition`

### What it is

A **guard** is a boolean predicate that must evaluate to true, in addition
to the input symbol matching, for a transition to fire. This is the
canonical definition from Guarded Transition Systems and Extended FSM
theory. The guard is evaluated against a runtime context (event data,
machine variables, external state) that is entirely outside the scope of
the toolkit.

Without a guard field, a machine that dispatches on the *same input symbol*
to *different target states* depending on runtime conditions cannot be
represented cleanly. The workaround — encoding the condition in the symbol
name — conflates the abstract alphabet with the concrete expression
language, breaks vocabulary hygiene, and makes `Analyse()` results
misleading (the alphabet appears larger than it is).

### Proposed addition to `Transition`

```go
type Transition struct {
    From   string
    Input  *string
    To     []string
    Output *string  // Mealy only — existing field

    // Guard is an optional boolean predicate expression evaluated by the
    // executor before firing this transition. The toolkit stores and
    // round-trips it as an opaque string; it does not parse, interpret,
    // or evaluate the expression. The expression language is defined by
    // the host application.
    //
    // When Guard is non-empty and the executor evaluates it to false,
    // the transition is not fired even if the input symbol matches.
    // When Guard is empty, the transition fires on input match alone
    // (classical FSM behaviour, backward compatible).
    Guard string `json:"guard,omitempty"`
}
```

### Format additions

**Normalised JSON** (`ToJSON` / `ParseJSON`): emit `guard` when non-empty;
parse it when present. No other changes.

**`.fsm` format**: `guard` expressions are stored in `labels.toml` under
a new `[guards]` section, keyed by a transition identifier. The transition
identifier is `from_state:input_index:to_state` (deterministic and compact).
This keeps `machine.hex` unchanged and limits the format change to the
labels file.

Alternatively the `guard` could be stored in `classes.json` as a per-transition
property if a transition class system is added — but we leave that to the
team's discretion.

### Effect on `Validate()`

No change. A machine with guards is a valid EFSM. Structural validity
(state references, alphabet membership) is independent of guard expressions.

### Effect on `Analyse()`

Add one new warning type:

| Warning type | Condition | Meaning |
|---|---|---|
| `mixed_guards` | A state has transitions on the same input where some have guards and some do not | The unguarded transition will always fire before guarded ones on that input, making the guarded transitions unreachable unless priority ordering is defined |

This is a design smell, not an error. The warning should list the affected
state and input symbol.

### Canonical example: vending machine

The classical vending machine example from automata theory demonstrates
the need directly. A vending machine with a finite can inventory has a
`dispense` input that leads to `stocked` when cans remain and to `empty`
when the last can is dispensed. These are the same input symbol, same
source state, different targets depending on a runtime variable
(`can_count`). Without a guard, this requires either:

- Encoding the count in the state topology (a separate state per count
  value — correct but unscalable), or
- Splitting the input symbol into `dispense_has_remaining` and
  `dispense_last_one` (smuggles the condition into the alphabet)

With guards:

```json
{"from": "stocked", "input": "dispense", "to": "stocked", "guard": "can_count > 1"},
{"from": "stocked", "input": "dispense", "to": "empty",   "guard": "can_count == 1"}
```

This is a canonical EFSM. The expression language (`can_count > 1`) is
opaque to the toolkit and defined by the executor.

---

## Request 2 — Actions on `Transition`

### What it is

An **action** (also called an **effect** in some formalisms) is a side
effect that executes when a transition fires. In classical Mealy machines
the only action is emitting an output symbol. In EFSMs the action set is
richer: updating machine variables, writing to external storage, emitting
events, scheduling future inputs.

The toolkit currently models outputs as a single optional string on
`Transition` (Mealy model). This is insufficient when a transition must
produce multiple effects, or when effects are not output symbols but
instructions to the executor.

### Proposed addition to `Transition`

```go
type Transition struct {
    From   string
    Input  *string
    To     []string
    Output *string  // Mealy only — existing field, unchanged
    Guard  string   // from Request 1

    // Actions is an ordered list of opaque effect expressions executed
    // by the executor when this transition fires. The toolkit stores and
    // round-trips them as strings; it does not parse, interpret, or
    // execute them. The expression language is defined by the host
    // application.
    //
    // Actions are intended to be evaluated as a batch by the executor,
    // not as independent sequential side effects. Ordering within the
    // list is significant when two actions affect the same target.
    //
    // The distinction between Output (single Mealy symbol) and Actions
    // (executor-defined effect list) is intentional: Output remains for
    // Mealy semantics; Actions serves EFSM semantics.
    Actions []string `json:"actions,omitempty"`
}
```

### Format additions

Same approach as `guard`: emit `actions` when non-empty in normalised JSON;
store in `labels.toml` under `[actions]` section keyed by transition
identifier in the `.fsm` format.

### Effect on `Validate()`

No change. An action list is not a structural property of the FSM.

### Effect on `Analyse()`

No new warnings required. The toolkit has no visibility into what actions
do; it cannot reason about their correctness.

### Why actions as a list rather than a single string

The single `Output` field on `Transition` works for Mealy machines because
a Mealy machine emits exactly one output per transition. An EFSM transition
may need to update multiple independent variables or write to multiple
targets. An ordered list allows the executor to accumulate all effects into
a single atomic operation, which is the correct execution model for
state-changing systems where partial writes are unacceptable.

If the list contains one entry, it is semantically equivalent to a single
action string. The list generalises cleanly without breaking the simple case.

### Example

```json
{
  "from": "stocked",
  "input": "dispense",
  "to": "empty",
  "guard": "can_count == 1",
  "actions": [
    "asset.set(can_count, 0)",
    "append(dispense_log, {dispensed_at: event.timestamp})",
    "outbox(notify_restock, {location: asset.location_id})"
  ]
}
```

The executor interprets these three strings and builds a single atomic
write. The toolkit carries them without interpretation.

---

## Question 3 — Unlinked machines in a bundle

### Context

We confirmed that `CreateBundle` places multiple machines in a single zip
without requiring links between them, and that `ValidateBundleLinks`
silently passes machines with no `LinkedMachines` entries. This means the
file format already supports co-packaging independent machines.

We use this as a packaging convention: machines governing orthogonal
concerns of the same physical entity are bundled together for transport
and version management, but execute independently.

### The question

Is a bundle containing multiple unlinked machines an **intended and
supported** use of the format, or a side effect of the implementation
that might be removed or restricted in a future version?

If it is intended, we would like it documented explicitly in the format
specification so we can rely on it.

### Why this matters

An entity with orthogonal concerns (e.g. inventory state and temperature
state) should not be modelled as a single machine with a cross-producted
state space. The correct model is two independent machines running
concurrently over the same entity. The bundle is a natural unit of
packaging for this pair without implying any coupling between them.

If bundles are intended only for linked (delegating) machines, we will
use two separate `fsm_machines` entities and manage the grouping ourselves.
Either answer is workable; we just need to know which one is correct.

---

## Summary

| # | Request | `Transition` change | Format change | `Validate()` | `Analyse()` |
|---|---|---|---|---|---|
| 1 | Guard | Add `Guard string` | JSON field + labels.toml section | No change | New `mixed_guards` warning |
| 2 | Actions | Add `Actions []string` | JSON field + labels.toml section | No change | No change |
| 3 | Unlinked bundles | None | Clarify specification | No change | No change |

Both new fields are opaque to the toolkit. The toolkit stores, round-trips,
and (for `guard`) optionally warns about them. It never evaluates them.
The expression language in both cases is entirely the responsibility of the
host application executor.

Backward compatibility is preserved in all cases: machines without `guard`
or `actions` behave identically to today.
