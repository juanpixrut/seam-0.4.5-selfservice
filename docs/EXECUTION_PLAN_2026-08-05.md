# Execution plan — API-key authentication + ingest compatibility

Date: 2026-08-05
Approved: all three points from `INTEGRATION_PLAN_2026-08-05.md`
Scope: the concrete design and sequencing for the two pieces of real work approved — API-key auth, and Recommendation 3's compatibility fix. Not yet code; the next step after this is implementation.

---

## Sequencing

API-key auth first, ingest compatibility second — not independent, dependent. A compatible ingest endpoint is useless without a way for a non-human caller to authenticate against it, and the compatibility fix's own auth story (Recommendation 3's table) explicitly leans on API-key auth existing first. Building them in the other order would mean either a throwaway auth stub for the ingest endpoint or leaving it genuinely open, neither of which is worth doing twice.

---

## The keys situation, across every project — checked directly, not assumed

Before finalizing Phase 1's design, I went looking at how every other service in this ecosystem actually handles credentials, including `nolu` (cloned directly from `github.com/ha1tch/nolu` once I had the right instruction to just go get it). The picture is more consistent than my original design accounted for, and it changed one thing worth being direct about.

| Project | Mechanism | Format |
|---|---|---|
| xolu itself | `XOLU_API_KEYS` env var | Flat comma-separated list, no tenant embedded — tenant comes from elsewhere (header/JWT) |
| `shelf-ingest` | `SHELF_INGEST_API_KEYS` env var | `key:tenantID` pairs |
| `auth-service` (shelf-compose) | `AUTH_API_KEYS` env var / docker secret | `token:tenant_id[:role]` triples, role defaults to `reader` |
| `slabbis-server` | N/A — device credentials, not API keys | Pulled from `olu-registry`, a separate concern entirely (device identity, not customer/API access) |
| nolu (for its own xolu storage backend) | `OLU_API_KEYS` env var | Same flat comma-separated list as xolu itself |

Every single one of these is static, environment-configured, and — per xolu's own `docs/FLEET_ARCHITECTURE.md` §3.5 — deliberately so: *"The registry stores instance endpoints but never stores credentials... Credentials are resolved at runtime from the deployment's secrets infrastructure."* That's not an oversight anywhere; it's the same principle applied consistently four times over.

**I don't think this principle applies to what Phase 1 is building, and I want to say precisely why rather than wave past the apparent contradiction.** The fleet document's rule is about *infrastructure* credentials — how a hot instance, the admin binary, and the registry authenticate to each other, provisioned alongside the deployment itself, often by an operator who already has secrets-manager access. Its own privilege ladder (§8.2) names this explicitly: infrastructure credentials sit at levels 2–4 (hot instance, admin binary, operator). A customer's own API key for calling Seam's REST API is level 0 — "Client (tenant-scoped): CRUD on own tenant's data" — a different layer the fleet document doesn't actually govern.

The distinction that matters isn't "database vs. env var" in the abstract, it's *who provisions the credential and how often*. Infrastructure credentials are provisioned once, by an operator, alongside standing up a service — an env var is exactly right for that. A customer's integration key is provisioned by the customer, on their own schedule, potentially many times, without needing an operator to edit a file and restart a service. `shelf-ingest`'s own README names this precisely as a gap: *"Actualmente las claves se cargan desde variables de entorno. Se debe integrar la consulta a olu-ams para validar tokens... permitiendo la revocación y creación de llaves sin reiniciar el servicio"* — dynamic key creation and revocation without a restart is explicitly the direction `shelf-ingest` itself says it still needs to go. Phase 1's design isn't diverging from the ecosystem's pattern; it's building the piece the ecosystem's own most-relevant service already says is missing.

What I'm changing based on this: the plan below keeps the dynamic, self-service, database-backed design for customer-facing keys, but now explicitly frames it that way rather than leaving it looking like an unexplained one-off next to four static-env-var systems.

## nolu changes the webhook picture — not for this round, but worth flagging now

`nolu`'s own subscription model (`docs/API.md`, `POST /subscriptions`) already treats `webhook` as one of four delivery endpoint kinds, alongside `xolu`, `nats`, and `activitypub` — scoped by GlobalID or entity type, with an `auth_token` stored encrypted and never returned in reads. That's architecturally the same problem Recommendation 2 (webhook persistence + real triggers, not part of this round's approvals) is solving, and nolu's version does it with cross-instance federation seam-ui's own bespoke engine never will.

I'm not proposing routing through nolu now — its own status is explicit: *"v0.1.0 — skeleton. All interfaces defined; concrete implementations pending."* No HTTP API, no concrete registry, no NATS bus — nothing runnable yet. But it's worth shaping seam-ui's own webhook work, whenever it happens, to *not* diverge unnecessarily from nolu's own event/subscription shape (similar payload envelope, similar endpoint-kind concept) so that migrating to nolu later is a swap, not a rewrite. Noted here so it's on record before that work starts, not discovered partway through it.

### Data model

A new xolu entity, `api_keys`:

| Field | Type | Notes |
|---|---|---|
| `tenant_id` | integer | Which tenant this key acts as |
| `name` | string | Human label — "Acme ERP integration", not the key itself |
| `role` | string | Reuses the existing role vocabulary (`viewer`, `operator`, `admin`, etc.) — deliberately not a new, separate scopes system, see below |
| `key_prefix` | string | First ~8 chars of the key, stored unhashed, indexed — enables lookup before the expensive hash comparison |
| `key_hash` | string | SHA-256 of the full key — API keys are already high-entropy random values, not human-chosen passwords, so a fast hash is the right tool here, not bcrypt |
| `created_at` | timestamp | |
| `expires_at` | timestamp, nullable | Optional — a key can be issued without an expiry |
| `revoked_at` | timestamp, nullable | Set on revocation; presence means the key is dead regardless of `expires_at` |
| `last_used_at` | timestamp, nullable | Updated on each successful auth — gives an operator a real signal for "is this key still in use" before revoking it |

**Why role, not a new scopes system:** `Claims.Role` already exists, `RequirePermission` already checks it against the established `rolePermissions` table, and every protected route in the API is already gated this way. An API key that resolves to `Claims{Role: "operator", ...}` gets exactly the same enforcement every human operator session already gets, for free. Inventing a parallel per-key capability system would duplicate machinery that already works correctly — the "reuse, don't duplicate" principle applies here as much as it did to the ingest schema question.

### Key format

`seam_<8-char-prefix>_<32-char-secret>`, generated with `crypto/rand`. The prefix is not a security boundary — it exists purely so lookup doesn't require hashing and comparing against every stored key on every request. Shown to the operator exactly once, at creation. Never displayed or logged again, matching how session secrets are already handled elsewhere in this codebase.

### Validation flow

Extends `internal/auth/middleware.go`'s already-stubbed path, not a new file:

1. `Authorization: X-API-Key <key>` (or the header `apiKeyHeader` is already configured for) arrives.
2. Split on the second `_` to extract the prefix; reject malformed keys immediately (wrong shape, wrong length) without touching storage.
3. **Check the cache first** (see below) before touching xolu at all.
4. On a cache miss: look up `api_keys` by `key_prefix` in xolu.
5. If found: hash the full presented key, compare against `key_hash` (constant-time comparison — timing attacks on a hash comparison are a real, well-known class of bug, worth doing correctly from the start).
6. Check `revoked_at` (must be null) and `expires_at` (must be null or in the future).
7. On success: build `Claims{TokenType: TokenTypeAPIKey, TenantID: <from key>, Role: <from key>, IssuedAt: now, ExpiresAt: <key's own expiry, or a short rolling window if the key itself has none>}` — `TokenTypeAPIKey` already exists as a constant, unused until now; this is what it was for.
8. Populate the cache with the validation result before returning.
9. Update `last_used_at` — best-effort, not on the request's critical path (a failed update shouldn't fail the request).

### Caching layer — reusing `slabbis`, not inventing a new one

The tech team's own reason for choosing `slabbis` for `shelf-compose`'s device-auth path (`slabbis-server`, caching device credentials in front of `olu-registry`) is that it's genuinely fast — and validating an API key on every single request, by hitting xolu each time, is exactly the same shape of problem `slabbis-server` already solves for device auth. Reusing it here isn't a new dependency for a new purpose; it's the same tool for the same kind of lookup.

`slabbis` is available as an in-process Go library (`github.com/ha1tch/slabbis`, `slabbis.NewDefault()`) with no extra service to deploy — `cache.Set(key, value, ttl)`, `cache.GetCopy(key)`, `cache.Del(key)`. Concretely:

- Cache key: `"apikey:" + key_prefix`. Cache value: the resolved `{tenant_id, role, key_hash, revoked, expires_at}` needed to finish validation without a second xolu round-trip.
- TTL short — 60 seconds is a reasonable starting point — so a revoked key can't remain silently valid for long even without explicit invalidation.
- **Explicit invalidation on revocation**: the moment a key is revoked through the management API, `cache.Del("apikey:"+prefix)` immediately, rather than waiting out the TTL. Revocation should be instant, not eventually-consistent, for a security-relevant credential.
- **Worth considering, not committed to yet**: caching negative results too (a short TTL on "this prefix doesn't exist") — protects xolu from being hammered by a scan of invalid keys, at the cost of a slightly slower first-response for a newly-created key if its prefix happened to be probed just before creation. A real trade-off, small enough to decide during implementation rather than needing a decision now.
- **Worth flagging for later, not solving now**: `slabbis`'s own `Cache` interface is explicitly swappable — in-process for a single node, a Redis-backed implementation for multi-node deployments where each seam-ui instance needs to see the same revocations immediately rather than per-node. In-process is the right choice for a single-node deployment (which matches the "self-contained tier" framing this whole plan started from); the swap point exists already if a multi-node deployment ever needs it, and doesn't need designing now.

### Management surface — two screens, one module, not one form with a role picker

The tech team's own correction to my first pass, and a stronger fix than what I'd proposed: rather than softening the *wording* on a shared form (a coarser "Read-only/Read-write" choice bolted onto the same screen humans use), split Administration into two genuinely separate screens — **Users** (unchanged, human accounts, existing roles, existing login) and a new sibling, **Automation** (or **Integrations** — naming worth settling during implementation, not here), covering every non-human credential as its own category rather than a variant of the human one.

This removes the role-mapping confusion entirely rather than just softening it: a person on the Automation screen is never looking at `viewer`/`operator`/`admin` at all, because that vocabulary belongs to the other screen. Read-only/Read-write (or whatever the right coarse split turns out to be) is the *only* vocabulary this screen needs, with no human-role concept sitting behind it to accidentally leak through in a label, an error message, or a future feature that assumes "role" always means the same thing.

Within Automation, rows are typed — `api` (general-purpose Seam API access, Phase 1's own design) and `erp` (a distinct type, worth scoping as more narrowly permissioned than general API tokens by default, matching what an ERP/WMS integration actually needs rather than everything the general API exposes) are both clearly in scope and share the same underlying `api_keys` table and validation path — a `type` field distinguishing them, not a separate schema. Same CRUD, same Roll action, same cache, same error-message discipline from the first pass — the two-screen split is an information-architecture change, not a second implementation.

**One real open question, not resolved here**: whether "xolu tokens" means a third row type on this same screen (a sophisticated integrator's more direct xolu-shaped access — fits cleanly, no complication) or something structurally different — the credential seam-ui itself uses to authenticate *to* xolu, which today is a single config-level value, not a per-tenant database record at all. If it's the second one, it doesn't belong on this screen, or in this database, at all — xolu's own fleet document is explicit that infrastructure credentials belong in secrets infrastructure, not application data next to customer tokens. Worth a direct answer before this gets built, since the two possibilities aren't a matter of degree — they're different features with different rules.

Three refinements within the Automation screen itself, carried over from the first pass and still worth keeping regardless of how the screen split lands:

**Read-only/Read-write as the only vocabulary this screen uses.** Internally still resolves to a role (or role-shaped permission set) on the same enforcement path every other credential already uses — the simplification is entirely in what's asked, not a new mechanism underneath it. Worth deciding during implementation exactly which existing role "Read-write" maps to, rather than inventing a third permission tier just for this.

**A "Roll this key" action, not just revoke-then-create.** Same key record, same name, same role, a freshly generated secret, the old secret dead the instant the new one's issued. This removes the actual failure mode that matters in practice — a live window with either two valid keys or a broken integration while someone copies the new value into their ERP. Plain revoke and plain create both still exist for when someone genuinely wants to retire an integration rather than rotate its credential.

**Error responses that help without leaking.** A malformed `Authorization` header (wrong scheme, wrong shape) gets a specific, actionable message — that's the caller's own mistake, safe to describe precisely. A key that's simply invalid — wrong secret, unknown prefix, expired, revoked — gets one generic "invalid or expired API key" response across all four cases, deliberately not distinguishing them: telling an unauthenticated caller *which* of those four is true is a small but real signal toward enumerating valid prefixes, and the specific reason isn't something a legitimate integrator needs to fix their own mistake (they need to re-copy the key or ask an admin, not learn exactly why the old one stopped working).

### Testing plan

- Unit tests: valid key succeeds, wrong secret with a valid prefix fails, unknown prefix fails, revoked key fails, expired key fails, malformed header fails gracefully (not a panic).
- Cache-specific: a second validation of the same key doesn't hit xolu again (confirm via a call-count assertion on the mock, not just "the test passed" — the whole point is avoiding the round-trip, worth proving directly); a revoked key stops validating *immediately* after revocation, not after the TTL lapses — this is the one that actually matters for security, and deserves its own explicit test rather than trusting the TTL math.
- Roll action: the old secret stops working and the new one works, in the same test, confirming there's no window where both or neither are valid.
- Error-message specificity: a malformed header gets a distinct, specific message; all four "the key itself is bad" cases (wrong secret, unknown prefix, expired, revoked) return the *same* generic message — worth a table-driven test confirming this deliberately, since it's easy for a future change to accidentally leak a distinguishing detail back into one of them.
- Adversarial: confirm the comparison is genuinely constant-time (or accept a documented, deliberate trade-off if not — worth being explicit rather than silent either way), confirm a key's raw value is never logged, confirm the key is never returned by any subsequent GET (only at creation).
- Live verification: create a real key through the real API, use it to call a real protected endpoint (`GET /api/v1/assets`) end to end, confirm a revoked key is rejected immediately afterward.

**Not code, but part of finishing this properly**: a short, copy-paste-ready customer-facing doc — "connect your ERP" in a few lines, matching what Stripe/Shopify/GitHub all lead with on their own equivalent pages. The best-designed key-creation flow still needs one page telling a non-technical person what to actually do with the value they're shown.

---

## Phase 2 — ingest compatibility

Builds on Phase 1. Three changes, all narrow:

### 1. Schema relaxation (`internal/validation/schemas.go`)

- `trigger_type`: `Required()` → `Optional()`. When absent, default to `"telemetry"` before validation runs — matching `shelf-ingest`'s own `resolveReadingType` default exactly, not inventing a different one.
- `value`: currently `builders.Object().Required()`. Needs to accept a scalar (number, string, bool) or an object — matching `shelf-ingest`'s `json.RawMessage` flexibility. The validation library's exact mechanism for "accept multiple shapes" needs checking against how `queryfy`/`builders` actually expresses that (a union type, an `Any()` builder, or hand-rolled type-switch validation) before committing to one approach — this is the one piece of Phase 2 I'd want to confirm against the real library API before writing it, not assume.

### 2. Compatible routes

Mount the same underlying `Ingest`/`IngestBatch` logic at `/ingest/v1/reading` and `/ingest/v1/readings`, alongside — not instead of — the existing `/api/v1/events`/`/api/v1/events/batch`. Two real differences to handle at this layer, not the schema layer:

- **Batch wrapper key**: `shelf-ingest` sends `{"readings": [...]}`; seam-ui's own batch handler reads `{"events": [...]}`. The compatible route needs its own thin decode step accepting `readings`, translating to the same internal shape before handing off to the shared ingest logic — not a schema change, a route-level adapter.
- **Response shape**: `shelf-ingest`'s own README documents an HTTP 207 Multi-Status response for partial batch success/failure. Confirm whether seam-ui's existing `IngestBatch` already does per-item success/failure reporting or returns an all-or-nothing result — if the latter, matching 207 behavior is itself real, if small, work, not just a route alias.

### 3. Auth

No new mechanism — this is what Phase 1 was for. A device (real or `shelf-vds`) is issued a seam-ui API key scoped to a role with event-ingest permission, and authenticates with `Authorization: Bearer <key>` (or the configured header) exactly like any other API caller. This is *protocol* compatibility with `shelf-ingest`, not a shared credential store — the two systems remain separately deployed with separately issued keys, `shelf-vds` just needs a URL and a key for whichever one it's pointed at, with no code change either way.

### Testing plan

- Reuse existing `Ingest`/`IngestBatch` tests as the base; add cases sending exactly the payload shape `shelf-vds` actually sends (`sensor_id`/`unit`/`value`-as-scalar/`timestamp`, no `trigger_type`) and confirm it's accepted and defaults correctly.
- A batch test using `{"readings": [...]}` specifically, confirming the wrapper-key adapter works.
- Live verification: point the real `shelf-vds` binary at seam-ui's new compatible endpoint (swap its `SHELF_INGEST_URL` env var to seam-ui's own address, and its API key to a real, freshly-issued one) and confirm an actual simulated sensor reading round-trips into a real xolu `events` row — the strongest possible confirmation, since it's the real client, not a synthetic test payload.

---

## What's deliberately not in this plan

Recommendation 2 (webhook persistence and real triggers) wasn't part of this round's three approvals — not folded in here, stays its own separate piece of work, though its eventual design should account for nolu's own subscription shape per the note above. Same for the shelf → Seam rebrand (recorded as an agreed approach, not scheduled), any integration with nolu itself (genuinely not usable yet — skeleton status, nothing concrete implemented), and the corrected usability plan's own items.
