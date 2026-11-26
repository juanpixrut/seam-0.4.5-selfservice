# Proposal: composite conditions for mintydyn's dependency system, and a note on an internal duplication

> **Scope note:** this proposal targets `github.com/ha1tch/minty`
> (specifically its `mintydyn` subpackage) — an external dependency of
> this repository, not Seam's own architecture. It's kept here per
> `docs/TRACKING.md`'s document taxonomy (Proposal → `docs/proposals/`)
> because it was drafted during Seam work and T-16 depends on its
> outcome, but nothing in it describes work to be done *in this repo*.
> The actual working copy, staged for filing as a GitHub issue against
> minty, lives in a local clone outside this repository (no push access
> from this environment — see the Access note at the end). This file is
> that same content, kept here so it isn't only reachable via an
> ephemeral `/tmp` clone.

Status: proposal, not started
Discovered: 2026-07-28, while auditing a mintydyn-adjacent consumer (Seam's
`internal/formengine`) for JS/TS footprint and redundancy

## Context

Seam's `internal/formengine` package needed conditional visibility for
form fields/sections/rows/tabs, driven by a condition language that
supports `all_of`/`any_of` composition (see FORM_ENGINE_SPEC.md §3b.5 in
that repo — not reproduced here, but the shape is: a leaf is
`{field, op, value}`, a composite is `{all_of: [...]}` or `{any_of: [...]}`,
and composites nest arbitrarily). mintydyn was evaluated as the natural
place for this — it already solves the general "dynamic UI driven by
form field state" problem — but its `DependencyRule`/`TriggerCondition`
system doesn't support composite conditions: each rule watches exactly
one trigger component's one condition (`javascript.go`,
`evaluateRules(triggerId, value)` / `evaluateTriggerCondition(trigger, value)`).
Confirmed by a full-text search of the package source for
`allOf`/`anyOf`/composite-related terms — nothing found. Seam ended up
building its own evaluator (`internal/formengine/visibility.go` +
a hand-written JS mirror) rather than force a composite requirement
through a system that can't express it.

This proposal is that finding, written up properly, in case composite
conditions are worth adding to mintydyn itself rather than staying
Seam-specific. If they land here, Seam can very likely delete its own
evaluator and depend on mintydyn's version instead — one less thing
Seam maintains, and every other mintydyn consumer gets composite
conditions too.

## Part 1: composite conditions — lower cost than it looks

The primitive this needs already exists and is already in production
inside mintydyn, just not reused where it's needed:

```js
// javascript.go, line ~526 — used today to gate ComponentState.Condition
// (i.e. whether a tab/state itself is visible)
evaluateCondition(condition) {
    const element = document.getElementById(condition.component || condition.field);
    if (!element) return false;
    const value = this.component.getInputValue(element);
    switch (condition.operator) {
        case 'equals': return value == condition.value;
        // ...
    }
}
```

`getInputValue(element)` (line ~298) is a general "read the current
value of any form control by element reference" helper. That's the one
thing a composite evaluator actually needs that a single-trigger one
doesn't: the ability to read a field's value *without* that field being
the one that just fired the change event. mintydyn already has it.

### Suggested Go-side change

Extend `TriggerCondition` (and, symmetrically, `StateCondition`, since
composites are equally useful for "is this tab/state visible") with two
optional fields:

```go
type TriggerCondition struct {
    ComponentID string      `json:"componentId,omitempty"`
    Event       string      `json:"event,omitempty"`
    Condition   string      `json:"condition,omitempty"`
    Value       interface{} `json:"value,omitempty"`
    Debounce    int         `json:"debounce,omitempty"`

    AllOf []TriggerCondition `json:"allOf,omitempty"`
    AnyOf []TriggerCondition `json:"anyOf,omitempty"`
}
```

Purely additive — every existing `TriggerCondition{ComponentID: ...}`
value is a valid leaf under the new shape, so this doesn't break any
current caller or serialized config.

### Suggested JS-side change

1. Generalize `evaluateCondition`/`evaluateTriggerCondition` into one
   recursive evaluator (they currently duplicate five of their nine-ish
   operators verbatim — see Part 2) that handles `allOf`/`anyOf` by
   recursing and reading each leaf's value via the existing
   `getInputValue`-by-id path, exactly like Seam's
   `internal/formengine`'s hand-written JS mirror does today (that code
   is a working reference implementation of this exact recursion, MIT/
   Apache-licensed same as this project, available on request if useful
   as a starting point — it's ~50 lines).
2. Rule registration (`this.activeRules`, keyed by a single
   `triggerId` today) needs to register a composite-conditioned rule
   against *every* component ID referenced anywhere in its condition
   tree, not just one, and re-evaluate the whole tree (not just the
   fired leaf) on any of their changes. Same shape as Seam's
   `visibilityScript`'s `fieldsOf(cond, out)` walk.

### Estimate

Given the primitive already exists, this reads like a self-contained,
few-hundred-line change concentrated in `javascript.go` plus a small
additive change to `mintydyn.go`'s types — not a redesign. Worth a
proper look rather than the rough sketch above, but the sketch above is
enough to judge whether it's worth scheduling.

## Part 2: found along the way — `evaluateCondition` vs. `evaluateTriggerCondition`

Not part of the ask, but found while reading the code closely enough to
write Part 1, and seems worth a line here rather than silently dropped.

`javascript.go` has two separate switch statements evaluating a
field-condition-value triple:

- `evaluateCondition(condition)` (~line 526) — used for
  `ComponentState.Condition` (tab/state visibility). Operators: `equals`,
  `notEquals`, `contains`, `greaterThan`, `lessThan`.
- `evaluateTriggerCondition(trigger, value)` (~line 1004) — used for
  `DependencyRule.Trigger`. Operators: the same five, plus `checked`,
  `unchecked`, `empty`, `notEmpty`.

The second is a strict superset of the first's operators, and the first
five cases are verbatim-identical text in both (confirmed by diffing the
two blocks directly). If Part 1 happens, this is a natural moment to
unify them into one evaluator function reused by both `StateCondition`
and `TriggerCondition` — composite support and de-duplication solve
together, since both need the same "recursively evaluate, reading
arbitrary field values" shape.

## Part 3: a lighter-weight tabs mode (lower priority, separate concern)

Unrelated to composites, but from the same audit: `mdy.Tabs()` generates
a full `class DynamicComponent_<id>` per instance (confirmed in
`javascript.go`) — no shared runtime across multiple component
instances on one page. That's the right tradeoff when a component
actually needs the full machinery (hooks, external script integration,
data filtering, rules), but for the common "just some tabs, nothing
fancy" case it means N instances on a page ship N copies of the whole
class body.

Seam ended up writing a ~25-line shared `window.SeamTabs.wire(panelSel,
btnSel)` function for exactly this case (used from two different call
sites, was previously hand-duplicated between them, now just calls the
one shared function — see Seam's `CHANGELOG.md` 0.2.9 entry if useful
context). Worth considering whether mintydyn wants an equivalent "lite"
pattern-detection tier — something like `PatternSimpleTabs` alongside
the existing `PatternPreRenderedStates` — that emits a call into one
shared page-level runtime instead of a full per-instance class, for
components with no hooks, no external scripts, and no data/rules. Lower
priority than Part 1 — it's a nice-to-have efficiency question, not a
capability gap — flagging it here since it came out of the same audit
rather than opening two separate write-ups.

## Access note

No push access from this environment to the `minty` repository -- this
file is staged locally in a clone, not pushed, and handed to ha1tch to
file as a GitHub issue (or discard) at his discretion.
