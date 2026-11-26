# formengine — Stage Tracker

Version: 0.4.5
Last reviewed: 2026-11-18

Stage-status tracker for the T-01 build (internal/formengine, per
FORM_ENGINE_SPEC.md v0.3 and the 7-step order in ROADMAP.md). Paired with
FORMENGINE_DEVELOPMENT_PLAN.md (frozen design intent) one level up. This
file is edited freely as work proceeds; the plan doc is not.

## Status table

| ID | Summary | Theme | Priority | Status | Blocks/after |
|----|---------|-------|----------|--------|---------------|
| FE-25 | Attachment tray -- attachable:true, TrayPermissions, four-level access control | stage-8-attachment-tray | P2 | ☐ | Requires Documents module (T-04); deferred, not part of T-01's closure gate |

---

## stage-8-attachment-tray

### FE-25. Attachment tray -- attachable:true, TrayPermissions, four-level access control

Theme: stage-8-attachment-tray · Priority: P2 · Status: ☐ · Blocks/after: Postponed indefinitely, possibly deprecated (the tech team, 2026-07-28) -- not part of T-01's closure gate, no active timeline

Trigger: ROADMAP.md's own step 8 for internal/formengine, missed when this tracker was first set up 2026-07-28 -- caught rereading ROADMAP while adding a pointer note there, fixed same session rather than left silently dropped.

Postponed indefinitely, possibly deprecated, 2026-07-28: correction to an earlier note that wrongly said this depends on T-04 being postponed too -- T-04 (Documents module) is NOT postponed, it's active planned work. What's actually postponed here is specifically this item's own mechanism: a per-Asset-instance attachment tray (attachable:true on Asset fields, TrayPermissions). The real reason, per the tech team: it's an open question whether documents should ever attach directly to an Asset instance at all, versus attaching to the chain of categories/types the asset belongs to -- a different architecture entirely, likely not a per-instance form-embedded tray widget. Explicitly flagged as a matter for a separate discussion, not decided here and not to be designed further until that discussion happens. FE-25 stays filed as a placeholder for "however document-to-asset attachment actually ends up working," not as a commitment to build it as originally scoped.
Scope: attachable: true on sections (FORM_ENGINE_SPEC.md $15.6, MVP: boolean), TrayPermissions, four-level access control per DOCUMENTS_MODULE_SPEC.md. Hard-depends on the Documents module existing, which is blocked on external seam-filer/fijuggler/thumble services (T-04) with no ETA. Judgment call: T-01 closes at FE-24 (core engine + Asset form parity) without waiting on this -- gating T-01 on an external, ETA-less dependency would leave it open indefinitely for a component outside formengine's own logic. FE-25 stays open as its own deferred item, picked up when T-04 unblocks.


---
