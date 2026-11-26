# Module System + Dashboard Invariant — Stage Tracker

Version: 0.4.5
Last reviewed: 2026-11-18

Stage-status tracker for the T-44 build (Module system, `internal/modules/`,
and the Dashboard invariant, `internal/dashboardengine/`, per MODULE_SPEC.md
v0.2 and DASHBOARD_SPEC.md v0.3). Paired with
MODULE_DASHBOARD_DEVELOPMENT_PLAN.md (frozen design intent) one level up.
This file is edited freely as work proceeds; the plan doc is not.

## Status table

| ID | Summary | Theme | Priority | Status | Blocks/after |
|----|---------|-------|----------|--------|---------------|
| MOD-5 | System module: Settings/Profile/Administration views routed through the registry using existing handlers, System dashboard proven against real audit-log data | system-module | P1 | ✓ | After MOD-3, MOD-4 |
| MOD-6 | Assets module: thin wrapper registering internal/ui's existing routes through the registry | assets-module | P2 | ✓ | After MOD-4 |
| MOD-7 | Documents module: minimal stub proving the registry pattern for a genuinely empty module | documents-module | P2 | ✓ | After MOD-2, MOD-4 |
| MOD-8 | formengine rebuild of Account/Settings/Administration's form-shaped views -- profile, preferences, settings, password, user create/edit all done | formengine-rebuild | P3 | ✓ | Complete, 2026-08-04 |
| MOD-9 | Deferred: Chart.js vendoring and the line_chart dashboard widget | chart-widget | P3 | ☐ | Not blocking anything -- purely additive, per the plan's own Stage 9 |
| MOD-10 | Adopt Sulpher for multi-entity dashboard widgets once xolu schemas exist -- real query-language improvement over the enrich mechanism, currently blocked on T-08 | dashboard-engine | P3 | ☐ | Blocked on T-08 (Settings module schema lifecycle) -- no xolu schema is registered for any entity in this deployment today |

---

## permission-reshape


## dashboard-engine


### MOD-10. Adopt Sulpher for multi-entity dashboard widgets once xolu schemas exist -- real query-language improvement over the enrich mechanism, currently blocked on T-08

Theme: dashboard-engine · Priority: P3 · Status: ☐ · Blocks/after: Blocked on T-08 (Settings module schema lifecycle) -- no xolu schema is registered for any entity in this deployment today

Surfaced 2026-08-02 during MOD-2 implementation: the "recent alerts"
widget needs three entities (alerts -> alert_types -> assets), which
xolu's OQL can't express in one query (two-table joins only, confirmed
directly against OQL_API.md). Shipped on the enrich mechanism instead
(DASHBOARD_SPEC.md v0.4 section 5.1) -- independent per-entity lookups
merged in Go, reproducing exactly what the pre-migration code already
hand-did.

Sulpher (xolu's Cypher-like graph query engine, github.com/ha1tch/
sulpher, exposed via /api/v1/graph/query) is architecturally the right
tool for this shape of query -- confirmed directly, a real Sulpher query
expresses the same three-entity lookup as one traversal:

  MATCH (a:alerts)-[:REF_alerta]->(t:alert_types), (a)-[:REF_asset]->(s:assets)
  WHERE a.status = 'open'
  RETURN a.id, a.title, a.created_at, s.name AS asset_name, t.type AS alert_type, t.criticality
  ORDER BY a.created_at DESC
  LIMIT 5

But it's not usable today. Graph edges only exist for fields declared
"REF" type in a registered xolu schema (POST /api/v1/schema/{entity}),
and confirmed directly: no entity in this deployment has a schema
registered at all -- everything is blob storage. The only mention of
schema registration anywhere in the codebase is a comment in masters.go
calling it "verified working end to end by a dormant guard," explicitly
not wired into production, gated on T-08 (Settings module's schema
lifecycle), which doesn't exist yet. Second cost on top of that: edges
are built at write time (create/update); nothing suggests retroactive
backfill on schema registration, so even after registering a schema,
every already-seeded alert/asset would need re-creating, not just
querying differently.

Real prerequisite, not a swap. Blocked on T-08. Not scoped or estimated
beyond that -- registering schemas for alerts/assets/alert_types with
correct REF fields, confirming edge-maintenance behavior against real
data, and learning Sulpher's actual query surface for this project's
specific cases are all real work once T-08 unblocks it, not assumed
trivial.

## audit-log


## module-registry


## system-module

### MOD-5. System module: Settings/Profile/Administration views routed through the registry using existing handlers, System dashboard proven against real audit-log data

Theme: system-module · Priority: P1 · Status: ✓ · Blocks/after: After MOD-3, MOD-4

Stage 5 of MODULE_DASHBOARD_DEVELOPMENT_PLAN.md. MODULE_SPEC.md section 5. internal/modules/system/. Uses the existing Settings/Profile/AdminUsers handlers as they are today -- not rebuilt on formengine yet, see MOD-8. Proves the registry against the most novel entitlement case (unconditional module access, per-view gating, one always-open view) before the easier Assets/Documents cases.

**Done, 2026-08-03.** Real, non-nil `MountRoutes` shipped for the first time (Assets' MOD-4 entry deliberately used `nil`, deferred to MOD-6). Resolved a genuine architecture gap MODULE_SPEC.md itself hadn't answered: `internal/ui.Router` keeps its own private mux and `requireAuth`/`requirePerm` middleware, so a naive `MountRoutes` registering directly onto the server's shared top-level mux would either bypass that middleware entirely or duplicate/shadow the existing registration. Resolved as a new exported `Router.MountSystemRoutes(mux)` method -- same private middleware and handlers, only the registration call site moves, per §6.1's "thin wrapper" reasoning made concrete; `internal/modules/system` stays a two-line closure wrapper. `MODULE_SPEC.md` bumped to v0.4 to fix its own stale `Module` struct snippet (still showed `func(r *Router)` through v0.3; the real shipped signature is `func(mux *http.ServeMux)`) and document the resolved mechanism (§5.4).

New `GET /system` route + `SystemDashboard` handler, mirroring the existing Assets `Dashboard` handler. `system-dashboard.json` written with corrected `module.view.action` permission strings (`system.settings.edit`, `system.user.view`) in place of `DASHBOARD_SPEC.md` §9's placeholder flat-format strings. 7 new i18n keys × 4 locales.

New `internal/ui/router_test.go` (8 tests) -- a class of test that didn't exist before: proves routes are reachable through the real mux with real middleware, `EnableAdmin` gating works, `requirePerm` genuinely blocks (not just `requireAuth`), and the pre-existing `/` catch-all still works. Verified by deliberately breaking a route and confirming the test caught it.

Live verification surfaced three real, unrelated bugs, all found and fixed in the same session, not deferred:
- **T-46**: `AdminUserEdit` never set `CSRFToken` on its form data -- editing an existing user was completely broken (CSRF always failed).
- **T-47**: `ListAuditEvents`' sort was broken two layers deep -- `xolu.Client.List()` never transmits its `Sort` parameter to the server at all, and OQL's own `ORDER BY DESC` has a real bug past the first row (reported upstream to the xolu team, full diagnostic transcript in `docs/oql-orderby-diagnostic-report.md`). Fixed by querying via OQL with `ORDER BY id DESC` instead, independently confirmed correct.
- **T-49**: `xolu.Client.Update()` is HTTP PUT (full-document replace) -- reported by the tech team after using the Settings form ("destroys or overwrites the user's profile"). Traced to a systemic pattern: 17 call sites across `internal/ui` calling `Update` with a partial field set, silently wiping the rest on every edit. Audited and fixed all 17, switching to `Patch` (a real partial merge).

Verified in both Spanish (tenant default) and English (switched via Settings) -- per the tech team's standing instruction that English, as the baseline, must always be checked alongside any other locale.

## assets-module

### MOD-6. Assets module: thin wrapper registering internal/ui's existing routes through the registry

Theme: assets-module · Priority: P2 · Status: ✓ · Blocks/after: After MOD-4

Stage 6 of MODULE_DASHBOARD_DEVELOPMENT_PLAN.md. MODULE_SPEC.md section 6. internal/modules/assets/. Deliberately not a physical relocation of internal/ui's ~3,000 lines -- assessed directly against this project's own /loc lesson (docs/KNOWN_ISSUES.md-adjacent behavioral lesson recorded 2026-08-01). Assets' dashboard is MOD-2's own worked example, already proven; this stage is the registry wrapper only.

**Done, 2026-08-03.** Applied MOD-5's resolved mechanism (§5.4) to a far larger surface, confirmed directly against §6.1's own description ("assets, sensors, rules, alerts, masters... ~3,000+ lines"), not a narrower reading: **98 routes** (dashboard, assets, events, sensors, alerts, map, FSM editor, rules, asset types, and all 9 master tables) versus System's 13. `Router.MountAssetRoutes(mux)` -- extracted mechanically from `registerRoutes()`'s original inline lines via a Python line-slice-and-transform script, not retyped, specifically to eliminate transcription risk at this size; the exact `EnableOperational`/`EnableAdmin` gating each route already had was preserved unchanged. `internal/modules/assets` is the same two-line closure wrapper as `internal/modules/system`. `MODULE_SPEC.md` bumped to v0.5 to fix its own stale registry-entry snippet (`assets.MountRoutes` without the `(uiRouter)` call) and document the resolved mechanism (§6.4).

By MOD-6, `registerRoutes()` has nothing `requireAuth`-gated left inline at all -- only genuinely public routes (login, password reset, QR scan, static files) remain. One existing test's premise broke as a direct, correctly-anticipated consequence: `TestMountSystemRoutes_PreexistingRouteStillReachableViaCatchAll` used `GET /assets` as its "still untouched by this stage" reference route, which MOD-6 obviously touches -- updated to `GET /login`, the only genuinely-still-inline case left.

New test coverage mirroring MOD-5's own pattern: 5 new tests in `internal/ui/router_test.go` covering dashboard reachability, `EnableOperational` gating, `EnableAdmin` gating (both a non-master route, rules, and a master-table route, regions, since they use different permission constants), and `requirePerm` genuinely blocking an under-privileged role (not just `requireAuth`) -- verified by deliberately breaking a route and confirming the test caught it. `mountedMux`'s shared test helper was extended to register both `MountSystemRoutes` and `MountAssetRoutes` together, matching real production wiring exactly and doubling as an integration check that the two don't collide.

Live-verified beyond unit tests: all 19 top-level routes across every category (dashboard, operational, admin-gated non-master, all 9 masters) confirmed reachable (200) as authenticated admin; module selector confirmed still rendering both entries correctly; a real region created through the real form (`POST /regions`, real CSRF, real `requirePerm` gate) confirmed landing correctly in xolu.

## documents-module

### MOD-7. Documents module: minimal stub proving the registry pattern for a genuinely empty module

Theme: documents-module · Priority: P2 · Status: ✓ · Blocks/after: After MOD-2, MOD-4

Stage 7 of MODULE_DASHBOARD_DEVELOPMENT_PLAN.md. MODULE_SPEC.md section 7. internal/modules/documents/. The documents.document.view placeholder permission added to all four roles. Lands on the empty-dashboard case (DASHBOARD_SPEC.md section 2.2).

**Done, 2026-08-03.** The permission (`documents.document.view`) and the selector's `nav.module.documents` i18n key already existed from earlier sessions' work, narrowing this stage's real scope to: `documents-dashboard.json` (`"sections": []`), `schemas.Documents()`, a `DocumentsDashboard` handler mirroring `SystemDashboard`, and `Router.MountDocumentsRoutes(mux)` -- one route, unconditional, applying MOD-5/MOD-6's same pattern at the opposite end of the size spectrum (1 route vs 13 or 98), per §7.3's own stated purpose for this worked example.

Found and fixed a real, previously-invisible gap along the way: `dashboardengine`'s empty-dashboard rendering path (DASHBOARD_SPEC.md §2.2, built in MOD-2) referenced an i18n key, `dashboard.empty_message`, that was never actually added to any locale file -- confirmed directly, not assumed, by grepping all four locale files before touching anything. Nothing had exercised `"sections": []` against a real schema before Documents, so the gap was invisible until now: without the key, the translator's own documented fallback returns the raw key string, meaning an authenticated user would have seen the literal text "dashboard.empty_message" instead of a real message. Added the key to all four locales.

New test coverage: 3 tests in internal/ui/router_test.go (reachability/auth-gating, plus one specifically guarding the missing-key gap -- verified by deliberately removing the key and confirming the test failed, then restoring it), plus internal/modules/documents' own delegation test matching System/Assets. mountedMux's shared test helper extended to register all three MountXRoutes methods together.

Live-verified beyond unit tests: `/documents` reachable (200) as authenticated user, module selector confirmed showing all three modules correctly, and the empty-dashboard rendering confirmed showing the real Spanish message ("No hay datos para mostrar en este módulo por el momento."), not the raw key -- direct confirmation the i18n gap actually mattered in practice, not just in theory.

## formengine-rebuild

### MOD-8. formengine rebuild of Account/Settings/Administration's form-shaped views -- profile, preferences, settings, password, user create/edit all done

Theme: formengine-rebuild · Priority: P3 · Status: ◐ · Blocks/after: No longer blocking-free by default -- actively in progress this session

Stage 8 of MODULE_DASHBOARD_DEVELOPMENT_PLAN.md (was deferred; picked back up 2026-08-03 -- the tech team: "Settings/Profile/Administration... are one and the same module with three separate views," confirming the fuller formengine-based separation these views are heading toward, not the hand-coded MOD-5 shape as a final state). The original design-conversation question -- can formengine build these views -- was answered yes for the form-shaped parts (profile, preferences, password, general/notifications/features settings, user create/edit) and no for user listing or a permissions/entitlement matrix, neither of which formengine or any existing pattern covers.

**Profile, Preferences, and all three Settings sections done and live-verified, 2026-08-03.** New schemas under internal/formengine/schemas/system/ (profile, preferences, settings_general, settings_notifications, settings_features). Handlers construct their FormEngine per-request with a real Translator (h.formEngineTranslator()) -- mirroring AssetCreateDynamic/AssetUpdateDynamic's proven pattern, not masters_pages_dynamic.go's package-level nil-Translator one, which is real, existing code in this repo but was never actually wired into a live route (confirmed directly from its own doc comment) and would have silently broken i18n resolution identically if copied.

Two real, previously-invisible bugs found and fixed along the way, both because this was the first time these code paths were ever exercised against a live route:
- **T-59**: formengine's renderFieldRow computed a translated field title but never applied it to self-labeled widgets (checkbox/toggle) -- every toggle in the whole system showed its raw English literal regardless of locale. Fixed at the engine level (renderer.go), one line, with a real-translator regression test verified by reverting the fix and confirming failure.
- Missing i18n keys, a separate gap from the engine bug: none of the affected fields (name, locale, timezone, dark_mode, email_alerts, compact_view, tenant_name, slack_enabled, webhook_enabled, map_enabled) had field-title translation keys authored at all. Added across all four locales, reusing the old hand-coded forms' exact existing wording rather than inventing new text. Also found, live (not assumed): resolveEnumOptionLabel's key format is form-scoped, so Preferences' locale labels didn't cover Settings-General's own "locale" field despite the identical enum values -- needed a second, separate general.locale.* key set.

Submit-side handlers (ProfileSave, ProfilePreferencesSave, SettingsSaveGeneral/Notifications/Features) needed zero new code -- their existing r.FormValue(...) parsing already matched the new forms' field names exactly, confirmed live: real saves, including the unchecked-checkbox-omission case, round-tripped correctly with every other field on the record surviving intact (T-49's PATCH fix from earlier this session).

Known, filed gaps, not blocking further progress: T-57 (formengine's visibilityScript duplicates once per Render() call on a page with multiple forms -- harmless today, no schema here has visible_when), T-58 (timezone's 19 IANA identifiers show unlocalized -- a scope call, not an oversight).

**Password change form done and live-verified, 2026-08-03 (T-60).** formengine had no "password" widget at all -- added one deliberately, not as a reuse of the value-echoing text widget (renderTextLike), which would have leaked the actual password into rendered HTML on any validation-error re-render. Also added a new x-seam-autocomplete schema extension, since current-password vs new-password meaningfully changes browser password-manager behaviour and a single hardcoded value would have been wrong for at least one of the three fields. Security-critical property (never echoes the value) verified with the full discipline: reverted the fix and confirmed the failure output visibly leaked the test's password value into rendered HTML, then restored it. Live-verified beyond unit tests: a real password change through the real form, confirmed the new password actually works via a real login attempt, then reverted and confirmed the original works too.

**User create/edit (Administration) done and live-verified, 2026-08-04 -- completes MOD-8's entire original scope.** Two schemas, not one: user_create.json (email, name, role, password, password_confirm) and user_edit.json (name, role, active) are genuinely different field sets, since formengine schemas are static and create/edit have never shared one form's worth of fields (password only on create, active only on edit). Email stays hand-coded, disabled, on edit -- same readonly-field gap as Profile's, formengine still has no such widget type.

Found and fixed a second real, pre-existing bug while rewriting this: AdminUserCreate has three possible render paths (validation failure, "email already exists", success), and the email-exists path never set CSRFToken at all -- resubmitting that specific re-rendered form would always have failed CSRF validation (T-62). Consolidated all three paths through one closure so this can't drift again.

Also found, while writing a new test, a real (if currently harmless) test-mock inaccuracy: xolu.Client.Get uses the whole response body as Entity.Data directly, no "data" sub-object unwrapping -- several existing test mocks, including one from this session's own T-46 work, wrap fields under a nested "data" key that Get() never reads. None of those tests currently fail (none assert on the affected field), but it's a real inaccuracy. Filed as T-61 rather than fixed everywhere mid-task.

Live-verified end-to-end, not just unit-tested: created a real user through the real form (all fields, correctly translated role labels), confirmed the new user could log in, edited the same user (name/role/active all changed correctly, everything else on the record survived via T-49's PATCH fix), confirmed the deactivated user is correctly blocked from login. No server errors throughout.

## chart-widget

### MOD-9. Deferred: Chart.js vendoring and the line_chart dashboard widget

Theme: chart-widget · Priority: P3 · Status: ☐ · Blocks/after: Not blocking anything -- purely additive, per the plan's own Stage 9

Stage 9 of MODULE_DASHBOARD_DEVELOPMENT_PLAN.md (deferred). DASHBOARD_SPEC.md section 4.6 and section 1.2's vendoring note. Vendoring steps: download the Chart.js UMD build to web/static/vendor/, add it to VENDOR.md, wire //go:embed in web/static.go -- same three steps every other vendored dependency already went through.

