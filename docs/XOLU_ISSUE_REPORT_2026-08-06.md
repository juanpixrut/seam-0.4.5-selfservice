# Status update and one small finding — xolu team

Date: 2026-08-06
From: Seam AMS

## Closing the loop on a previous report

`docs/oql-orderby-diagnostic-report.md` (the `ORDER BY DESC` diagnostic
shared earlier) — confirmed fixed. Re-tested directly against a live
v0.28.0 instance today: full, correct descending order across the
entire result set, not just the first row. Your own CHANGELOG entry
(T-144, v0.24.7) traces this to `fmt.Sscanf`'s partial-match behavior
on timestamp strings, found the same pattern in three separate call
sites via a full-tree sweep, and added a regression test that
specifically checks the full order rather than just the extremum — the
exact gap that let the original bug ship unnoticed. No further action
needed on our side; this is just confirmation it's genuinely resolved,
not assumed.

## One small, still-open item

`WithAPIKey`'s own doc comment (`pkg/client/client.go`) still reads:

> sent as `"Authorization: Bearer <key>"`

The actual implementation (`authHeader()`) has sent `"ApiKey <key>"`
since the T-160 fix landed. Not a functional issue — the code is
correct, only the comment describing it is stale. Worth a one-line fix
whenever it's convenient; not raising this as urgent, since it doesn't
affect anyone's actual integration, only a reader of the source
comment itself.

That's the complete list — a systematic search turned up nothing else
currently outstanding.
