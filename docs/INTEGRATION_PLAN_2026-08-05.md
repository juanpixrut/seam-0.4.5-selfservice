# Seam — customer integration API, webhooks, and shelf/Seam consolidation

Date: 2026-08-05
Builds on: `USABILITY_PLAN_2026-08-05-corrected.md`

---

## The headline finding — there's more already built than either of us realized

Before writing this, I went looking for what seam-ui already has toward a customer-facing API. I expected to find `internal/webhook` and not much else. Instead I found a second, substantial, already-documented REST API I hadn't examined all session: `internal/handlers/api`, with a real OpenAPI 3.0 spec (`api/openapi.yaml`, titled "Seam AMS API"). It covers assets, sensors, asset-types, rules, FSM, webhooks, QR codes, reports (including CSV export — the thing I said was "entirely absent" in the first plan; it isn't, it's just not wired into the UI I'd been looking at), and alerts. It's enabled by default. It talks to the same xolu registry the UI does — not a parallel data model.

It also explains something from the corrected plan: `internal/rules/engine.go`, which I'd flagged as dead code because nothing in `internal/ui` calls it, is in fact wired up — from here, not from the UI. Seam-ui has its own complete, working, in-process event-ingestion-and-rule-evaluation pipeline, entirely separate from the one in `shelf-compose`. The tech team's call on what to do with it is in Recommendation 3 below — not discarded, kept deliberately as a lighter-weight tier, made protocol-compatible with the real pipeline instead.

**One finding matters more than the others: API-key authentication is stubbed across the entire API.** Every protected endpoint — assets, sensors, events, webhooks, everything — accepts a bearer JWT (human session tokens) or nothing. `X-API-Key` returns a literal `"not yet implemented"` error, with the TODO already sketching the real shape (key storage, hash validation, expiry, scopes). Right now, no external system — no ERP, no WMS, nothing — can authenticate against this API at all, regardless of how complete the rest of it is.

## What already exists, mapped against what you're asking for

| You asked about | What's actually there |
|---|---|
| "An API for customers to integrate with" | Already built, comprehensive, documented, enabled by default. Blocked entirely on one missing piece: API-key auth. |
| "Webhooks... a simplified version customers may appreciate" | Also already built: full delivery engine (HMAC signing, retry, worker pool) plus a full CRUD+test management API. Two real gaps: registrations are in-memory only (lost on restart), and nothing ever calls `.Trigger()` for a real event — only the "Test" button does. |
| "The simple ingest API the virtual devices use" | That's `shelf-ingest`, genuinely separate, genuinely sophisticated, not seam-ui's concern. Seam-ui has its *own*, smaller, in-process ingest+rules pipeline (`/api/v1/events`, `internal/rules`) that appears to duplicate it. |
| Old shelf code working with Seam | Both systems currently exist independently, unaware of each other. |

## Recommendation 1 — finish API-key auth first; nearly everything else follows from it

This is the one item on this whole plan I'd call unambiguous. The TODO comment already in the code names the real shape:

1. Key storage — persisted in xolu, not in-memory, scoped per tenant.
2. Hash validation — store a hash, not the raw key, matching how session secrets are already handled elsewhere in this codebase.
3. Expiry and scope — a key should be revocable and limitable (read-only vs read-write, which entities it can touch), not all-or-nothing.
4. Claims derivation — an API key needs to produce the same `Claims` shape session auth does, so the existing `RequirePermission` machinery on every route keeps working unchanged.

Once this exists, the entire already-built API surface — assets, sensors, webhooks, reports, QR — becomes usable by an external ERP or WMS with no further work on those individual endpoints. This is the highest-leverage single item in this whole plan, for the same reason "wire the rule engine" was in the first draft of the other plan: the hard part is already built; what's missing is the one connector.

## Recommendation 2 — webhooks: persist registrations, then wire real triggers

Two separate, small pieces:

**Persistence.** `webhook.Service` holds registrations in a Go map. Move this to xolu — a `webhooks` entity, matching the shape already defined in `webhook.Webhook`. Not a redesign, a storage swap.

**Real triggers.** Right now `.Trigger()` is only ever called by the webhook's own "Test" button. The event types already defined (`asset.created`, `asset.updated`, `asset.deleted`, `asset.status_changed`, `alert.created`) are all things that happen inside seam-ui's own code, in `internal/handlers/api`'s asset/alert handlers — wiring these is calling one existing method from a few existing handlers, not new infrastructure. I'd deliberately leave `event.ingested` and `rule.triggered` unwired for now, since those describe telemetry concerns that — per the corrected plan — aren't seam-ui's to own; wiring them properly means a bridge from the real pipeline, which is real, separate work, not something to fold in here.

## Recommendation 3 — keep seam-ui's own ingest/rules pipeline, make it protocol-compatible with the real one

The tech team's call, and the reasoning is sound: a minimal, self-contained system matters for demos, entry-level users, and onboarding — someone shouldn't need to stand up mosquitto, NATS, `norm-route`, and `fsm-executor` just to see Seam work. But a self-contained tier is only useful if the same virtual devices (and real ones, eventually) can point at it — meaning it needs to speak the same ingest contract `shelf-ingest` does, not a different one.

I checked how close the two actually are, rather than assuming. Closer than expected, in one important way: **the `trigger_type` vocabulary is already identical between the two systems** — `sensor_driver`, `gateway_poll`, `lora_notification`, `qr_scan`, `custom_driver`, `api_call`, `mobile_app` appear verbatim in both `shelf-ingest`'s `triggerTypeAliases` map and seam-ui's own `EventIngestSchema` enum. That's not a coincidence; it means the vocabulary work is already done and doesn't need reconciling.

What's actually different, checked directly against both codebases:

| | `shelf-ingest` (`POST /ingest/v1/reading`) | seam-ui (`POST /api/v1/events`) |
|---|---|---|
| `trigger_type` | Optional — defaults to `telemetry` if absent | **Required** — rejects the request without it |
| `value` | `json.RawMessage` — any JSON shape (scalar, object, array) | **Must be a JSON object** — rejects a plain number |
| Route path | `/ingest/v1/reading`, `/ingest/v1/readings` (batch) | `/api/v1/events`, `/api/v1/events/batch` |
| Auth | `Authorization: Bearer <key>`, resolved to tenant via `SHELF_INGEST_API_KEYS` (`key:tenantID` pairs) | Bearer JWT only today; API-key auth is the Recommendation 1 gap |

`shelf-vds`'s actual payload — `{"sensor_id", "unit", "value": <number>, "timestamp"}`, no `trigger_type` at all — would be rejected by seam-ui's endpoint today on two separate counts (missing required field, wrong type for `value`), not because the systems disagree on what a reading *means*, but because seam-ui's schema is stricter than it needs to be.

The fix is narrow, not a redesign: relax `trigger_type` to optional with the same `telemetry` default `shelf-ingest` uses, accept `value` as either a scalar or an object, and mount the existing ingest logic at a compatible path alongside (not instead of) `/api/v1/events`. Auth compatibility falls out of Recommendation 1 for free if seam-ui's own API-key resolution follows the same bearer-token-to-tenant shape `shelf-ingest` already uses — same credential type, independently issued keys, no shared secret store required.

This also changes what "duplication to discard" actually means here: it's not the pipeline itself, it's the assumption that a stricter, incompatible schema was ever a deliberate choice rather than the two systems having simply grown up separately. Nothing about seam-ui's own rule engine or event storage needs to change for this — only the ingest contract's edges need loosening to match.

## Recommendation 4 — "may be a separate service": not urgent to decide now

`internal/handlers/api` is already a clean, separate Go package from `internal/ui`, sharing only the xolu client and auth machinery. Extracting it into its own deployable service later — matching `shelf-api`'s shape — is a real option, but doesn't block anything in Recommendations 1–2. I'd finish API-key auth and webhook triggering first, inside seam-ui as it exists today, and revisit extraction once there's a real reason to (scaling independently from the UI, a customer requirement for a separate SLA, etc.) rather than doing it speculatively now.

## Recommendation 5 — the shelf → Seam rebrand: search and replace, fix what breaks

Simpler than I first scoped it, and the tech team's call on the approach: a direct search-and-replace of `shelf`/`Shelf`/`SHELF` → `seam`/`Seam`/`SEAM` across all four packages, then build and test each one and fix whatever the rename surfaces. Covers module paths, imports, NATS subject strings, environment variable names, docker-compose service names, and directory/file names in one pass, rather than the phased, risk-sequenced rollout I originally proposed.

Rough scope, checked directly rather than guessed: ~61 files (Go/YAML/Markdown/shell/Python/env/JSON) contain a "shelf" reference case-insensitively, plus 8 files/directories with "shelf" in the name itself, across `seam-ingest`, `seam-compose` (11 separate Go modules inside it), and `seam-vds`. `xoluman` has none — untouched by this rebrand.

Not scheduled as part of this planning pass — this note records the agreed approach; the actual rename is separate, later work.

## What I need from you before starting

1. **Green light on API-key auth as the first, concrete piece of work** — I'm confident enough in this one to just start, if you'd like me to.
2. **Green light on Recommendation 3's compatibility fix** (relax `trigger_type`/`value`, mount a compatible route) — also confident enough to just start, now that the actual gap is precisely scoped.
3. **Rough priority between this plan and the corrected usability plan's items** — they're complementary, not competing, but worth knowing which you'd like worked first.
