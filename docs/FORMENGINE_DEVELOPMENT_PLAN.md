# internal/formengine — Development Plan

Updated: 2026-07-28

Design intent for the T-01 build (main register, `docs/TRACKING.md`). Frozen
once execution starts — deviations get recorded in
`docs/formengine/TRACKING.md`, not silently merged back into this document.
Task-level tracking, status, and progress live in `docs/formengine/TRACKING.md`
(a separate repoman-managed register, ID prefix `FE`, its own
`.repoman.json`) — this document is the stable plan those tasks point back to.

Stage order matches `ROADMAP.md`'s existing 7-step outline for the form
engine, expanded into concrete tasks. Every citation below is to
`FORM_ENGINE_SPEC.md` v0.3 unless stated otherwise.

---

## Why this order

Each stage is independently testable and buildable on the last, and the tree
stays green throughout — no stage requires a later stage's work to compile or
run:

1. **Core engine** has to exist before anything can render.
2. **Flat master forms** are the smallest real proof the engine works, chosen
   deliberately over the Asset form (Region/Currency/Brand have no tabs,
   details, embeds, visibility rules, or field permissions to get wrong).
3. **Tabs and row layout** are the first piece the Asset form actually needs
   that the flat masters didn't exercise.
4. **Details and embeds** are Asset-specific (sensor binding, custody
   history) and depend on tabs existing.
5. **Conditional visibility** is orthogonal to structure but depends on having
   real fields and sections to hide.
6. **Field permissions** depend on `RenderContext` (stage 1) and are easiest
   to reason about once the form's shape is stable.
7. **Asset form parity** is the actual deliverable everything else was
   building toward — full conversion, full regression test, and only then is
   the static Asset form retired.

Static and dynamic forms coexist throughout (§13.3) — nothing is deleted
until parity is proven at the end of stage 7.

---

## Stage 1 — Core engine (flat forms, no tabs)

Package layout per §13.1: `engine.go`, `renderer.go`, `widgets.go`,
`schema.go`, `sources.go`. `RenderContext` per §3c.2. `SchemaToForm` entrypoint
and queryfy validation wiring per §13.2 and §9.

**Load-bearing decision:** the widget dispatch mechanism must be a registry
(name → renderer), not a hardcoded switch. T-10 already lists five widget
types the CMMS work needs that aren't in the v0.3 MVP (signature, image, GPS,
multi-select, decimal precision) — those need to slot in later without
restructuring this stage's code.

## Stage 2 — Flat master forms

Región, Moneda, Marca — no tabs, smallest possible real schemas. First
side-by-side parity check between dynamic and static rendering.

## Stage 3 — Tabs, sections, row layout

§3.1–§3.8. Includes the §15.1 resolved decision (reject a schema at load if
`section_defs` has IDs not referenced in any row). Required for the Asset
form, which is four tabs.

## Stage 4 — Detail panels and embedded sub-forms

§3.5 (`details`), §3.6 (`embeds`, `SeamModal2`), §3.7 (lifecycle
interaction). Needed for the Asset form's `sensores_vinculados` /
`custodio_history` panels and `sensor_binding` embed.

## Stage 5 — Conditional visibility

§3b, client-side JS evaluator plus server-side queryfy re-evaluation (§3b.6 —
client evaluation is never trusted alone). §15.2 resolved decision: merge-
conservative hidden-field policy by default, `x-seam-hidden-reject: true`
opt-in for fields with external effects.

## Stage 6 — Field permissions

§3c.3 (`FieldPermission`), §15.3 resolved decision (explicit role priority:
admin=100, seam_consultant=90, manager=50, operator=30, viewer=10;
most-restrictive wins on ties), §3c.5 (applying permissions in the renderer).
This stage builds the interface/hook the eventual Administration module (T-02)
manages permissions through — not that module's UI itself.

## Stage 7 — ModalTriggerAttrs and full Asset form parity

§15.4 resolved decision: `ModalTriggerAttrs(schema, url, label)` — handlers
never choose `SeamModal` vs `SeamModal2` directly, the engine decides from
`x-seam-form.width`. Then: author the full 4-tab Asset schema, convert the
handlers, run full regression against the existing static form, and only then
retire the static implementation.

**This stage is the completion gate for T-01** in the main register — T-01
doesn't close until stage 7's parity task closes.

## Stage 8 — Attachment tray (deferred, not part of T-01's closure gate)

`attachable: true` on sections (§15.6, MVP: boolean), `TrayPermissions`,
four-level access control per `DOCUMENTS_MODULE_SPEC.md`. This is
`ROADMAP.md`'s own step 8 for the form engine — hard-dependent on the
Documents module (T-04) existing, which is itself blocked on external
services with no ETA. Filed as its own tracker item (FE-25) rather than
folded into stage 7, and T-01 closes without waiting on it: gating T-01 on
an ETA-less external dependency would leave it open indefinitely for a
component outside formengine's own logic. Picked up when T-04 unblocks.

---

*Paired document: `docs/formengine/TRACKING.md` (task-level status, its own
FE-nn register).*
