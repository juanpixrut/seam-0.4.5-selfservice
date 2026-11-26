# Multi-instance xolu provisioning — architecture plan

Date: 2026-08-05
Trigger: clarifying what "xolu tokens" meant in the Administration screen-split discussion — the real answer is bigger than a token type, it's a standing architectural gap in how seam-ui finds its own data.

---

## What this actually is

Every line of seam-ui code written this entire session assumes one thing that turns out not to be true going forward: that there is exactly one xolu instance, configured once at startup (`cfg.XOLU.URL`, `cfg.XOLU.APIKey` → one `xolu.Client`, built once in `internal/server/server.go`, handed to every handler for the process's whole lifetime). That's the actual shape of `xolu.New(cfg.XOLU.URL, xoluOpts...)` today — one URL, one credential, no per-tenant resolution at all.

The real target: many xolu instances, some hosting multiple tenants each, possibly more than one instance per physical server. Seam needs to determine, per logged-in account, *which* instance and *which* tenant on it holds that account's data — and that resolution needs to come from a provisioning layer coordinated with `nolu`, not be assumed fixed at deploy time.

This is what "xolu tokens" actually named in the earlier conversation: not a customer-facing token type sitting next to API/ERP tokens, but the credentials for a growing *fleet* of xolu instances, and the standing question of which one a given account's request should even be routed to.

## Why `nolu` is already the right shape for this — checked directly, not assumed

`nolu`'s own model, read directly from `pkg/identity`:

```go
type LocalRef struct {
    InstanceURL string  // "https://xolu.acme.com"
    TenantID    uint16
    EntityType  string
    LocalID     int
    TenantName  string
}

type OwnershipRecord struct {
    GlobalID GlobalID
    Current  LocalRef
    History  []Transfer
}
```

This is documented as solving "which xolu instance currently holds *this entity*," for physical assets changing hands between organizations. But structurally, it's identical to "which xolu instance currently holds *this account's* data" — mint a GlobalID per Seam account/tenant instead of per physical asset, and `OwnershipRecord.Current.LocalRef` gives exactly `{InstanceURL, TenantID}`, exactly what a request needs to know which client to use. An account migrating from one xolu instance to another — a real operational event as the fleet grows — becomes a `Transfer`, with the same negotiated lifecycle and audit history `nolu` already provides for asset transfers. Nothing about `nolu`'s design needs to change to serve this purpose; it's the same mechanism, a different kind of entity being tracked.

## Why this can't be built yet — and why "prepare it now anyway" turned out to be the wrong instinct

`nolu` is explicitly "v0.1.0 — skeleton. All interfaces defined; concrete implementations pending" — no HTTP API, no `MemoryRegistry`, no `XoluRegistry`, nothing runnable. Seam-ui cannot query a real `nolu` registry today because there is no real `nolu` registry to query yet.

The shape the eventual solution should take is still worth recording, even though — corrected below — it's not something to start building now:

```go
type InstanceResolver interface {
    Resolve(ctx context.Context, accountID string) (*xolu.Client, error)
}
```

Today's behavior would become the trivial implementation — a `StaticResolver` always returning the one configured client. When `nolu` is ready, a `NoluResolver` implementing the same interface drops in, resolving `accountID → LocalRef → xolu.Client`.

**Correction, checked directly rather than assumed**: I'd originally claimed introducing this now would mean "none of the hundreds of existing call sites need to change." That's wrong, and I found out by actually counting rather than continuing to assert it — `xolu.Client` has 89 public methods, too many to transparently wrap without an ongoing maintenance burden, and there are already 229 call sites across `internal/ui` and `internal/handlers/api` that touch the client directly. Every one of those needs to change to route through a resolver, whether that happens now or later — introducing the interface today doesn't avoid that migration, it just moves it earlier, against a `nolu` API that isn't finalized yet. See the revised recommendation at the end of this document.

This still mirrors a pattern already proven twice in this same planning arc — `slabbis`'s own `Cache` interface, `formengine`'s `SourceResolver` — the *shape* is right. The timing claim was the part that didn't survive checking.

## Caching the resolution — same tool, same reason as Phase 1

Resolving "which instance does this account live on" on every single request would mean hitting `nolu`'s registry constantly for something that changes rarely — an account's home instance is stable until a deliberate migration. This is the identical shape of problem Phase 1 solved for API-key validation: `slabbis`, in-process, `"account:"+accountID → LocalRef`, a TTL long enough to matter (minutes, not seconds — this changes far less often than an API key's validity) with explicit invalidation the moment a `Transfer` completes, not left to expire on its own. A migration mid-flight is a real event worth invalidating immediately, the same reasoning that applied to key revocation in Phase 1.

## The "xolu tokens" admin screen — a third category, not a row type

Given the above, this is now clearly not a peer to the API/ERP tokens on the Automation screen from the earlier discussion — it's operator-facing fleet visibility, not customer self-service. And per xolu's own fleet document (§3.5, already established in the earlier pass of this plan): the actual credentials for connecting to each xolu instance belong in secrets infrastructure, not a database record next to customer tokens. What a screen *can* usefully show, without violating that principle: which instances exist, which accounts/tenants are currently resolved to each (read from `nolu`'s own registry once it exists), instance health, and — critically — never the raw credential itself, the same way a cloud console shows "a secret exists and was rotated on this date" without ever displaying its value.

## What I'd actually do, in order — revised after checking the real cost

**Correction to an earlier claim in this document**: I originally called introducing `InstanceResolver` now "cheap" and "the seam built before it's needed." I checked that more carefully after we asked directly whether simple mode now, extended later, was viable — and the "cheap" framing doesn't hold up. `xolu.Client` has 89 public methods; wrapping it transparently so existing call sites keep working unchanged means hand-maintaining an 89-method interface against everything xolu ever adds, for no benefit until `nolu` exists. There are already 229 call sites touching the client directly across `internal/ui` and `internal/handlers/api`. That number needs to change regardless of when this happens — doing it now doesn't reduce the eventual work, it just moves it earlier, against a `nolu` interface that isn't finalized yet.

1. **Run simple mode now, as-is.** Not a compromise — everything built and verified this session runs on exactly this architecture, and it's real.
2. **Don't start the resolver work yet.** There's no genuinely free way to prepare for it that doesn't risk designing against a guess rather than `nolu`'s real, eventual API.
3. **When `nolu` has a real HTTP API and a concrete registry implementation** — not scheduled by this plan, gated on `nolu`'s own progress — do the `InstanceResolver` introduction and the full call-site migration together, in one pass, informed by what `nolu` actually turned out to need. The `slabbis` caching layer described above still applies at that point.
4. **The fleet-visibility admin screen**: depends on step 3 existing first — nothing meaningful to show an operator before `nolu`'s registry has real data.

Nothing here is on a timeline seam-ui's own team controls until `nolu` is concretely closer to ready. That's a real answer, not a placeholder — the honest cost of the growing call-site count between now and then is mechanical (find-and-replace shaped) rather than subtle or risky, which is exactly why it's safe to defer rather than guess at early.
