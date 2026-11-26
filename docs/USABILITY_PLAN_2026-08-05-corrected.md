# Seam AMS — corrected plan, after examining the real ecosystem

Date: 2026-08-05
Supersedes: `USABILITY_PLAN_2026-08-05.md` (same day, wrong premise on the monitoring half)

---

## What I got wrong, and why

My first pass concluded the "monitoring loop" — sensor reports something, a rule fires, an alert appears — didn't exist anywhere. That was wrong. It exists, and it's substantially more sophisticated than anything I was about to propose building: a real MQTT/HTTP gateway, a NATS message bus, a normalization stage, a dedicated `fsm-executor` service doing atomic, CAS-based state transitions against xolu's own `/commit`, and a thin read API (`shelf-api`) for consuming the result. I found this by reading `internal/rules/engine.go` in seam-ui, saw it was never called from anywhere live, and concluded the whole pipeline was missing — when what I'd actually found was a leftover, superseded piece of *seam-ui's own* codebase, not evidence the capability itself didn't exist elsewhere.

That's worth sitting with for a second: I was about to recommend building a second, worse version of a system that already exists and is already better designed than what I'd have produced. The correction changes the plan substantially, not just the wording.

## The corrected map of who owns what

| Concern | Owner | Notes |
|---|---|---|
| Device connection, auth, envelope construction | `mosquitto` + `gateway` (shelf-compose) | Not seam-ui's concern at all |
| Ingestion validation, normalization, batching | `shelf-ingest` | Standalone service, real auth, real batch processing |
| Event routing, GIAI validation, dead-lettering | `norm-route` (shelf-compose) | Not seam-ui's concern |
| Rule matching, FSM state transitions | `fsm-executor` (shelf-compose) | Already atomic, already CAS-safe, already production-shaped |
| Rule *definition* (operator UI) | `shelf-ams` | Already has its own web UI (`/rules`, `/devices`, `/assets/types`) built with minty — the same renderer seam-ui uses |
| Reading current/historical operational state | `shelf-api` | Simple proxy: `GET /v1/assets/{giai}` and `/v1/assets/{giai}/history`, reading from a separate `olu-ops` xolu instance |
| Device simulation for testing | `shelf-vds` | Answers the "how does a sensor submit a reading" question I'd asked — already solved |
| Raw xolu data browsing/admin | `xoluman` | Separate operator tool, phpMyAdmin-shaped |
| Asset registry, masters data, people/access | **seam-ui** | Genuinely solid, confirmed in the first pass |
| Physical sensor-to-asset ownership (which sensor is bolted to which asset) | **seam-ui** | Registry data, not telemetry — this is correctly seam-ui's job, and the binding work from earlier today stands |
| Maintenance/work | **seam-ui** (Work Orders) | Genuinely seam-ui's own domain |
| Documents | **seam-ui** (mostly unbuilt) | Unchanged from the first pass |

The dividing line, once I actually looked, is clean: **seam-ui owns the registry — what exists, who's responsible for it, what work has been done to it. The pipeline owns telemetry — what's happening to it right now.** My first plan blurred that line because I found a disconnected piece of seam-ui's own code and assumed the capability was missing entirely, rather than checking whether it existed properly somewhere else first.

## What this means for `internal/rules/engine.go` and `internal/webhook`

Both are real, competently-built code. Both appear to predate — or duplicate — what `fsm-executor` and the NATS pipeline now do properly, atomically, and as a separately-scaled service. I don't think either should be "wired up," which is what I proposed in the first pass. I think they're candidates for removal, but **I'm not deciding that here** — it's possible one of them serves a purpose I haven't found yet (a fallback mode, a different tenant tier, something not yet connected to this ecosystem). Worth a direct answer from you before I touch either.

## The corrected critical path

Same walkthrough as before, re-scored against what's actually true:

1. Register an asset — **works**
2. Bind a sensor to it — **works** (registry-level, correctly seam-ui's job)
3. The sensor reports something — **works, elsewhere.** Not seam-ui's concern, and shouldn't be.
4. A rule evaluates it, an alert-worthy transition happens — **works, elsewhere**, via `fsm-executor`.
5. Someone *managing assets* in seam-ui sees that something happened — **this is the actual gap.** Seam-ui has no connection to `shelf-api` at all. An asset's real operational state (GIAI-keyed, already a field on every asset) is invisible from inside seam-ui.
6. Someone acknowledges/resolves — seam-ui's own alerts pages work, but only against seam-ui's own disconnected `alerts` table, which has no relationship to what `fsm-executor` actually produces.
7. Someone creates a work order to fix it — unchanged from the first pass: exists, gated off, no link from an alert.
8. The asset's history reflects the fix — unchanged: doesn't propagate.
9. Eventually retire the asset — unchanged: possible, not a dedicated action.

The real gap isn't "build monitoring." It's "seam-ui's asset registry has no window into the monitoring system that already exists, even though the join key (`giai`) has been sitting right there on every asset the whole time."

## Revised plan — small, compounding, in corrected order

### 1. Show real operational state on the asset detail page

**Correction, 2026-08-05, found during an unrelated audit**: this already exists. `fetchRuntimeState` (`internal/ui/routes.go`, tracked as `S-11`) already calls `seam-api`'s `GET /api/v1/assets/{giai}` and shows the result on the asset detail page, gated behind `SeamAPIURL` defaulting to empty (disabled). I should have found this before proposing to build it fresh — I didn't check for an existing integration before recommending one. The gap wasn't "this needs building," it's that `SeamAPIURL` is unset in every deployment checked this session, so the feature has been present but dormant the whole time. Fixed separately (see `docs/EXECUTION_PLAN_2026-08-05.md`'s own T-75 entry): the existing implementation had no request timeout and no named client boundary, both now corrected. What's left here isn't building it — it's deciding whether `SeamAPIURL` should be configured by default in more deployments, and whether `/v1/assets/{giai}/history` (the recent-transitions endpoint, not just current state) is worth surfacing too, neither of which this document resolves.

### 2. Decide the fate of seam-ui's own sensors/events/alerts pages
Not something to decide unilaterally. Three real options, each defensible: (a) retire them, since `shelf-api`/`shelf-ams` now cover this properly; (b) keep them as a simpler, seam-ui-local fallback for tenants not running the full pipeline; (c) reconcile them — e.g., seam-ui's `alerts` table becomes a record of alerts *acted on*, populated by reading from the real pipeline rather than existing as its own disconnected source. I have a mild lean toward (c), since the acknowledge/resolve workflow you already have is genuinely useful and shouldn't be thrown away — but this is a real product call, not a technical one.

### 3. Work Orders → real, scoped feature
Unchanged from the first pass, and worth restating why it's still right: it already uses xolu's own `/fsm` primitives directly — form submission drives a guarded transition, xolu commits it, no hand-rolled Go state machine involved. That's exactly the pattern this correction reinforces should be the standard, not the exception. What's left: flip `EnableWorkOrders` on, give it real permissions, link it from an alert (once item 1 makes alerts real), and have completion write back to the asset.

### 4. A dedicated "retire this asset" action
Unchanged from the first pass.

## Explicitly still not building

Same as the first pass, unchanged by this correction: the full Documents module (still depends on external services), data export/reporting (still genuinely absent, still a layer on top rather than a precondition), and the fuller CMMS vision beyond a scoped Work Orders.

## What I need from you now

1. **Confirm or correct the corrected map above** — particularly whether `shelf-api` is really the intended read path for seam-ui, since I inferred that from its shape rather than being told directly.
2. **A decision on `internal/rules/engine.go`/`internal/webhook`** — remove, or is there a use I haven't found?
3. **A decision on item 2 above** — retire, keep as fallback, or reconcile seam-ui's own sensors/events/alerts pages.

Item 1 (real operational state on the asset detail page) I'm confident enough to start on once you confirm `shelf-api` is the right target — everything else genuinely needs your call first, since these are product decisions about which of two working systems to trust, not technical ones I should make alone.
