# Feature request: xolu Go client — `CreateMachineDef`

Version: 0.1
Date: 2026-08-04
Status: Request — describes a local, provisional patch on the Seam AMS side,
not a specification the xolu team is bound to. Please review and shape this
however fits xolu's own conventions best; the shapes below are "what worked
for us," not a mandate.
From: Seam AMS (github.com/ha1tch/seam-ui)
Concerns: `github.com/ha1tch/xolu/pkg/client`

---

## 1. Context

Seam AMS just built and shipped a real, working "Work Orders" feature —
create a work order, drive it through a small lifecycle (`draft` →
`in_progress` → `completed`), gate the final transition on a technician's
signature, all backed by xolu's `/api/v2/fsm` primitive. This is the first
time Seam AMS has composed a form submission with an FSM transition and a
durable xolu commit, and the FSM subsystem itself worked exactly as
documented — guard evaluation, the reachability/determinism analysis on
definition creation, the audit trail, all correct and pleasant to build
against once we had the right client-side pieces.

One piece was missing.

## 2. The gap

`pkg/client` has a complete, well-designed surface for *operating* on FSM
machines — `CreateMachine`, `WalkMachine`, `GetMachine`, `GetMachineState`,
`GetMachineTransitions`, `GetMachineHistory`, `PatchMachine`,
`DeleteMachine`, `ListMachines` — and for *reading* definitions
(`ListMachineDefs`, `GetMachineDef`). There is no client method to *create*
a definition. A caller who wants to instantiate a machine has to already
have a `fsm_def_id` from somewhere, and the only way to get one through the
client library is to already have created it some other way.

We confirmed this directly, not from stale docs: neither the version we had
pinned (v0.23.0) nor the newer checkpoint (v0.25.0) has this method. Every
other definition-adjacent read operation exists; this one create operation
doesn't.

## 3. What we did as a stopgap

We added `CreateMachineDef` to our local sibling checkout of `pkg/client`,
matching `CreateMachine`'s own shape as closely as made sense. This is what
we're asking the xolu team to review, reshape as needed, and land properly
upstream — not what we're asking you to adopt verbatim.

```go
// CreateMachineDefRequest is the body of Client.CreateMachineDef -- we
// reused MachineSpec directly (the same struct GetMachineDef returns
// inside MachineDef.Spec) rather than inventing a parallel type, since
// the def-create endpoint accepts exactly the spec body with no
// separate wrapper.
type CreateMachineDefRequest = MachineSpec

// CreateMachineDefResult is the response shape we actually observed
// from a real running v0.25.0 server -- {id, name, created_at,
// analysis}, not the full spec body (the caller already has the spec
// it just submitted, so echoing it back seemed redundant, but that's
// exactly the kind of call we'd want your team's own judgment on).
type CreateMachineDefResult struct {
	ID        int64           `json:"id"`
	Name      string          `json:"name"`
	CreatedAt string          `json:"created_at"`
	Analysis  json.RawMessage `json:"analysis,omitempty"`
}

func (c *Client) CreateMachineDef(ctx context.Context, req CreateMachineDefRequest) (*CreateMachineDefResult, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if req.Initial == "" {
		return nil, fmt.Errorf("initial is required")
	}
	var result CreateMachineDefResult
	if err := c.doURL(ctx, http.MethodPost, c.buildURLv2("/fsm/def"), req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
```

Hits `POST /api/v2/fsm/def`, verified end-to-end against a real running
server — including the real `XOLU-FSM006` ("definition declares no states")
and `XOLU-FSM004` ("transition guard rejected: ...") errors surfacing
correctly as structured `*client.Error` values through this method, the
same way they do through every other machine-operation method.

We wrote tests for it (happy path, both empty-required-field rejections, and
a structured-error case) — happy to share those directly if useful as a
starting point, though we'd expect your team to want your own coverage
regardless.

## 4. A smaller thing, worth mentioning while we're here

Not a missing feature — a real papercut we hit directly and lost real time
to. A guard expression written with a double-quoted empty string,

```json
"guard": "payload.technician_signature != \"\""
```

silently matches nothing, ever — not a parse error, not a validation
warning, just a guard that always evaluates false regardless of what's in
the payload. The fix was switching to single-quoted string literals
(`!= ''`), which is what every real example in the xolu docs already uses —
we just didn't catch that we'd deviated from it until a working guard
mysteriously rejected a perfectly valid payload.

We don't have a strong opinion on the right fix — better emphasis in
`docs/FSM.md`'s examples, a lint-style warning at definition-creation time
when a guard contains a double-quoted string literal (which is almost
certainly a T-SQL-identifier/string-literal mix-up, not intentional), or
just leaving it as documented T-SQL behavior we now know to watch for. Only
flagging it because it cost us real debugging time and might cost someone
else the same.

## 5. Ask

1. Review `CreateMachineDef` (or your own equivalent shape) and land it in
   `pkg/client` — this is the one genuinely blocking gap.
2. Consider the guard-syntax footgun in section 4 — no specific fix
   requested, just flagging it.

## 6. What happens after

Once this ships upstream, Seam AMS will drop the local patch on our sibling
checkout and pull the real, official client method instead — this document
describes temporary, provisional code on our side, not a fork we intend to
maintain.
