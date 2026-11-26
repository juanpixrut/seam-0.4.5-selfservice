# Module System + Dashboard Invariant — Resolved

Closed tracking items, full text as at closure, newest first. Append-only.
Cross-references the changelog entry that shipped the fix; does not
duplicate it.

## [0.3.40] MOD-4 — Module registry and selector UI (v0.3.40, 2026-08-02)

Theme: module-registry · closed 0.3.40 · 2026-08-02

Stage 4 of MODULE_DASHBOARD_DEVELOPMENT_PLAN.md. MODULE_SPEC.md sections 3-4.

One scope refinement from the filed description, recorded rather than silently redefined: "replacing today's single direct call into internal/ui" turned out to be MOD-6's own dependency, not something MOD-4 could safely do itself. internal/server/server.go's uiRouter := ui.NewRouter(...) call still mounts Assets' ~3,000 lines of existing routes directly, unchanged -- replacing it now, before Assets is actually re-registered through the new mechanism, would have broken the entire application until MOD-6 landed. MOD-4 instead introduces the registry and selector as a real, tested, live-verified mechanism that MOD-6 will complete the migration onto, exactly the dependency order MODULE_DASHBOARD_DEVELOPMENT_PLAN.md's own "why this order" section already stated (MOD-4 blocks MOD-6, not the reverse).

internal/modules (new package): Module and Registry types, deliberately dependency-free with respect to any one module's own implementation (no import of internal/ui or any future internal/modules/{assets,documents,system} subpackage) -- MountRoutes takes *http.ServeMux, the same top-level mux internal/server already builds everything else on. Registry.Register panics on a duplicate ID (a startup-time programming error, not a runtime condition); VisibleTo fails closed on a nil HasAccess, matching dashboardengine.PermissionChecker's own nil-case reasoning from MOD-3. 7 tests.

Selector UI (internal/ui/module_selector.go): renders MODULE_SPEC.md §4.1's structure exactly -- an Apps group header with its members, then System as one flat entry with no group wrapper of its own (a detail the spec's own §4.1 already flags as corrected once during design, not repeated here). Wired into TopHeader between the dark-mode toggle and the user menu (§4.3's placement), with a SeamModuleSelector JS toggle mirroring the existing SeamUserMenu's own open/close-on-outside-click behaviour exactly. Config gained VisibleModules, computed fresh per request by a new PageHandler.viewerConfig(r) from auth.Claims and PageHandler.ModuleRegistry -- never written back onto the shared Config, since (unlike every other Config field) this one varies by viewer, not by tenant. Config's own pre-existing UserRole field, defined but never populated anywhere in the codebase before this, is now populated by the same mechanism.

Two real bugs caught and fixed before they shipped:
- A Go language limitation, not a logic error: b.Div(a, b, c, items...) doesn't compile when mixing explicit arguments with a slice spread in the same call. Fixed the same way already used in dashboardengine/widgets.go -- append into one combined slice first, then spread that.
- A real spec bug: MODULE_SPEC.md's own Module struct had only DashboardID, and the selector's first draft built hrefs as "/" + DashboardID -- wrong for Assets specifically, whose dashboard lives at the existing root route "/", not at "/assets-dashboard" (DashboardID is an internal schema identifier, not a URL). Added a distinct Path field and corrected MODULE_SPEC.md itself (now v0.3), including all three worked-example registry entries (System, Assets, Documents), not just the one that happened to break first.

12 new tests across two files (5 for ModuleSelector's own rendering -- empty state, Apps grouping, System's flat entry, mixed groups preserving registration order, an unrecognised Group value rendering nothing rather than guessing where to put it; 5 for PageHandler.viewerConfig -- no-claims passthrough, UserRole population, nil-registry passthrough, real registry filtering by role, and a dedicated test proving the shared Config is never mutated, only copied).

Live-verified against a real xolu instance and a real authenticated session (not a mock): the selector rendered correctly with Spanish translations from the tenant's own locale ("Cambiar módulo", "Aplicaciones", "Activos"), the correct inventory_2 icon, and href="/" -- confirming the Path fix specifically, not just that something rendered. Only Assets registered for this stage, deliberately -- System and Documents don't have real routes to land on yet (MOD-5, MOD-7), and shipping a selector entry whose Path 404s would be a broken feature, not a working one. Zero errors in server logs across the verification session.

Cross-ref: CHANGELOG 0.3.40.

## [0.3.39] MOD-3 — Audit log wiring plus the audit_log dashboard data source and permission-based widget visibility (v0.3.39, 2026-08-02)

Theme: audit-log · closed 0.3.39 · 2026-08-02

Stage 3 of MODULE_DASHBOARD_DEVELOPMENT_PLAN.md. DASHBOARD_SPEC.md sections 5.2, 9.3. Two parts, shipped as two checkpoints (0.3.38, part 1; this one, part 2) rather than one, since they turned out separable enough to verify independently.

Part 1 (v0.3.38): auditLogger.Log calls added to AdminUserCreate/Update/PasswordResetSave (using LogUserCreated/LogUserUpdated/LogRoleChanged/LogPasswordChanged -- convenience methods that already existed in internal/auth/audit.go but were never called by anything before this), SettingsSaveGeneral/Notifications/Features (new AuditSettingsChanged event type and LogSettingsChanged method, neither existed before), and ProfilePreferencesSave (new AuditPreferencesChanged event type and LogPreferencesChanged method). Same pass fixed /settings/{general,notifications,features}'s missing permission gate (requireAuth only until now) to requirePerm(auth.PermSystemSettingsEdit, ...) -- GET /settings deliberately left alone, every current role already holds PermSystemSettingsView, so gating it would have been a no-op dressed up as a fix, not in this pass's actual scope.

Two real findings in part 1, not assumed clean: a genuine regression (TestAdminUserUpdate_Valid_Redirects' mock server didn't handle the new Get() call AdminUserUpdate now makes to detect a role change -- fixed the test's mock, not the production code) and a latent nil-receiver bug in AuditLogger.Log itself, found while investigating why a first pass of new tests passed without any audit call actually executing (no auth.Claims in bare test requests, so the calling code's own guard silently skipped them -- not evidence the wiring worked). Added if a == nil { return } to Log; wrote 5 real end-to-end tests (internal/ui/audit_wiring_test.go) injecting real auth.Claims and asserting against a recording auth.AuditStore, proving each event fires with correct data.

Part 2 (this closure): dashboardengine's audit_log source type (§5.2) and permission-based visible_when (§7.2, field-value visibility from §7.1 stays unimplemented -- the spec itself notes it's "not identified as needed today").

Real design work needed before any of this could be built, not mechanical: AuditFilter had no way to filter by Resource or multiple EventTypes, and there was no exported way to query audit events from outside the auth package at all (ListAuditEvents existed only as an AuditStore interface method, never called anywhere in production). Extended AuditFilter with Resource and EventTypes (kept alongside the existing singular EventType rather than replacing it, since nothing set that field in production either); added AuditLogger.List (nil-safe, degrading to an empty result rather than a panic or error, matching Log's own reasoning: "no audit data available" is a legitimate empty-state for a widget, not a fatal one); extended XOLUAuditStore's filter loop to honor both new fields, in the same fetch-broadly-filter-in-Go style its existing UserID/TenantID/EventType checks already used.

dashboardengine's own extensions: EntitySource gained Resource/EventTypes/Scope (§5.2) plus an internal, non-JSON CurrentUserID field populated by the engine from a new RenderContext.UserID right before any source resolves (harmless no-op for entity sources, required for audit_log's scope: "self"). PermissionChecker is a new interface (same decoupling pattern as EntityResolver/Translator -- no auth dependency in this package itself) and Section gained VisibleWhen.

Two real bugs caught and fixed before they shipped, not after, both found by questioning my own first draft rather than trusting it compiled:
- Wrote PermissionChecker's nil-fallback as "fail open" (show the section) at first, matching Translator's own nil fallback -- then recognised mid-edit that a permission gate failing open is a security anti-pattern, unlike Translator's cosmetic worst case (an untranslated key). Corrected to fail closed before any code called it.
- sourceCacheKey builds its key via json.Marshal(EntitySource), and CurrentUserID deliberately carries json:"-" (it's not part of the schema) -- meaning the marshal alone would never vary the cache key by user, so two different users' scope: "self" results would have collided in the cache, one user seeing another's audit trail. Fixed by explicitly appending CurrentUserID to the key string outside the marshal, with a dedicated test (TestSourceCacheKey_VariesByCurrentUserID) proving two different users now produce different keys.

xoluDashboardResolver.auditLogRows resolves the source via the new AuditLogger.List, converting AuditEvent to Row with the field names DASHBOARD_SPEC.md §9's own audit_log widget examples already referenced (email, action, event_type, details, timestamp) -- scope: "tenant" relies entirely on h.xoluClient already being tenant-scoped (confirmed directly: no handler anywhere in this codebase manually filters by tenant_id, the xolu client itself does it), only scope: "self" adds an explicit UserID filter on top of that. authPermissionChecker adapts auth.HasPermission to the new interface -- the whole adapter is one method, since HasPermission's own signature already matches modulo the auth.Permission wrapper type.

9 new tests across the two packages (6 in dashboardengine covering visible_when's four real cases plus the cache-key fix; 4 in internal/ui covering auditLogRows' scope filtering, field mapping, and EventTypes filtering) -- one test bug caught in the same pass as the security-relevant implementation bugs: an early visible_when test never set RenderContext.Role, so it "passed" by coincidence (empty role correctly failed the permission check) rather than proving anything; added renderToStringWithContext and fixed the three tests that actually needed a specific role.

Cross-ref: CHANGELOG 0.3.38 (part 1), 0.3.39 (part 2, this entry).

## [0.3.37] MOD-2 — Dashboard engine core: dashboardengine package, five widget types, entity data source, proven against the Assets dashboard migration (v0.3.37, 2026-08-02)

Theme: dashboard-engine · closed 0.3.37 · 2026-08-02

Stage 2 of MODULE_DASHBOARD_DEVELOPMENT_PLAN.md. Scope: DASHBOARD_SPEC.md sections 2-6 and 10 -- the dashboardengine package, the five dashboard.go-derived widget types, and the entity data source. Proven via section 8's worked example: the existing Assets dashboard migrated to a JSON definition. dashboard.go retired only once parity was proven live, same coexist-then-retire discipline as T-01/FE-24's static Asset form.

Built as three explicit phases, not specified this precisely at filing time: resolve (source.go's EntityResolver, real implementation in internal/ui/dashboard_resolver.go, no xolu dependency in the package itself) -> compile (compile.go, resolved data rendered through a fixed per-widget-type template using pkg/nplate into canonical JSON -- see pkg/nplate/nplate.go's own doc comment and CHANGELOG 0.3.36 for why nplate exists, not tracked as its own MOD item since it emerged mid-stage rather than being separately scoped) -> render (widgets.go, consuming only compiled data). cache.go adds an optional TTL cache at the compiled-data boundary, zero by default.

19 dashboardengine tests, including a call-counting fake resolver proving cache hits skip EntityResolver entirely, and full end-to-end renders through the real Render pipeline rather than unit tests of individual functions in isolation.

Real findings during implementation, not assumed going in:
- entity sources couldn't express the recent-alerts widget's real need (alerts -> alert_types -> assets, three entities) -- confirmed directly that xolu's OQL supports only two-table joins. Added enrich to the source format (DASHBOARD_SPEC.md section 5.1, v0.4).
- recent_list had no way to name which row field is primary/secondary/timestamp text -- the pre-migration code hardcoded AssetName/Title-with-AlertaTipo-fallback/CreatedAt, which doesn't generalise to System's future audit-log rows. Added primary_field/secondary_field/secondary_fallback_field/timestamp_field.
- A real bug caught before shipping: a row's own link and the header's "view all" link were conflated (both reading the same property) -- would have produced a broken /alerts?status=open/5 href. Fixed with a distinct item_href_prefix property (v0.5).
- date_field/label_field/group_field (speculative widget properties from early drafts) turned out unused by any actual rendering code once built, since the compiled row shape is fixed by the resolver's own types, not configurable per widget instance -- removed rather than left as dead schema surface.
- An earlier spec draft's urgency_list example listed three date fields where the real loadVencimientos only ever checked two (loan_expiry was never part of it).

Live verification (real xolu, real seed data -- 16 assets, 4 runtime alerts -- real authenticated session, not a mock) caught two more real bugs, both confirmed against the live database before fixing:
- badge_colors used English keys (High/Medium/Low) against actual stored Spanish criticality values (Alta/Media/Baja) -- consistent with T-40's own note that Alerta's field values, not just field names, were left untouched, a separate scope.
- date_fields' raw xolu column names (fecha_expiracion/fin_garantia) don't match the pre-existing dashboard.field.expiration/warranty_end i18n keys. Added date_field_labels to EntitySource (DASHBOARD_SPEC.md v0.6) to decouple the i18n key from the raw field name -- avoids the same churn recurring when T-40's still-paused Asset rename eventually lands.

internal/dashboardengine/schemas/assets-dashboard.json is the real, shipped definition, using current field names throughout (criticidad, fecha_expiracion, fin_garantia), not the spec's own aspirational English worked example.

dashboard.go retired to attic/old-dashboard/ after live parity was proven, confirmed by re-verifying live a second time immediately after retirement (identical stat counts, badges, and translated labels both times). Whole-file/whole-function .pre-retirement copies, matching the static-asset-form precedent (FE-24): dashboard.go, dashboard_test.go, and loadVencimientos (extracted separately from masters_routes.go, which stays). Two real exceptions found by a tree-wide reference sweep before retiring, not assumed: EventSummary (genuinely used outside the old dashboard) moved to asset_detail.go rather than retired; three shared test helpers (contains, containsHelper, newTestHandler) moved to helpers_test.go. TestFormatNumber ported to dashboardengine's own tests, since it now tests an unexported function in a different package. web/static_test.go's icon-font scan extended to also walk internal/dashboardengine/schemas/*.json -- icons that used to be Go string literals now live in JSON, invisible to the existing Go-source-only scanner.

Cross-ref: CHANGELOG 0.3.37.

## [0.3.35] MOD-1 — Permission reshape: module.view.action format, wildcard matching, module-access derivation, migrate every Permission constant and requirePerm call site (v0.3.35, 2026-08-02)

Theme: permission-reshape · closed 0.3.35 · 2026-08-02

Stage 1 of MODULE_DASHBOARD_DEVELOPMENT_PLAN.md. Scope: MODULE_SPEC.md §2 in full -- the new module.view.action permission format, wildcard matching (§2.2-2.3), module-access derivation (§2.4), migrating every existing Permission constant per §2.5's table, and updating every requirePerm(auth.PermXxx, ...) call site in router.go to the new constants.

Two real findings from §2.5 confirmed exactly as filed: PermDataImport was dead code (zero usages anywhere outside its own declaration) -- dropped, not migrated. Every master-data route's reused PermSettingsUpdate (48 sites, all in internal/ui/router.go, all write actions -- confirmed none of the 8 master entities' GET/list routes were ever permission-gated at all, requireAuth only, unchanged) got its own real assets.master.edit permission instead of carrying the conflation forward.

**A third conflation, not anticipated by MODULE_SPEC.md's own §2.5, found during implementation, not assumed:** internal/handlers/api/router.go's webhook routes (8 sites: 2 read + 6 write) also reused PermSettingsRead/Update -- a second, entirely separate misuse of the same old permission, disjoint from the master-data one (confirmed zero overlap: master data only ever appears in the UI router, webhooks only in the API router). Given its own real permission pair, assets.webhook.view/edit, rather than folded into assets.master.* where it doesn't belong.

**A deliberate deviation from MODULE_SPEC.md §6.2's own illustrative (explicitly "not exhaustive") view mapping:** the spec sketched assets.sensor.* as a new, more granular split for sensors. Implementation preserved current behaviour exactly instead -- sensors stay under assets.asset.* (the API router's sensor routes already reused PermAsset* before this reshape, same as asset-types/QR/alerts/events). Introducing a genuinely new permission split changes who-can-do-what and wasn't this stage's job (a pure format reshape); MODULE_SPEC.md's illustrative mapping remains a legitimate future direction if a real case ever needs the finer split, not something to force into a migration whose whole point was behaviour preservation.

**Two more dead-code findings, confirmed by zero call sites anywhere in the tree:** User.CanAccess (constructed the old two-segment "resource:action" format directly -- would have been actively wrong left as-is, removed rather than migrated) and Middleware.RequireAnyPermission (a generic multi-permission check, structurally unaffected by the format change since it delegates to HasPermission, kept as live infrastructure since it's a reusable primitive rather than a single obsolete permission, just currently unused).

**Real compile errors caught by go build/go vet, not assumed clean:** three direct auth.PermUserCreate/Update/Delete references in internal/ui/routes.go's Admin users list handler (inline HasPermission calls, not route registrations -- missed by the router-only sweep, caught immediately by go build). Test-file breakage in internal/auth/auth_test.go went beyond a rename: TestUser_CanAccess tested a now-removed method (replaced with TestHasModuleAccess, real new coverage for §2.4's derivation rule, not a like-for-like swap) and TestHasPermission's two "import data" cases were removed rather than renamed, since PermDataImport no longer exists. New test coverage added, not just preserved: TestPermissionMatches, covering §2.3's wildcard algorithm directly (exact match, view-level wildcard, module-level wildcard, wildcard non-crossing between views/modules, no implicit partial-segment match) -- behaviour this migration introduces that had zero prior coverage under the old exact-match-only model.

Role grants preserved exactly across every new constant -- this migration reshapes the permission string format, it does not change who can do what. rolePermissions' new shape verified directly against the old one construct-by-construct before writing any code, not inferred after the fact.

system.settings.view/edit are defined in this migration but not yet wired to any route -- /settings' own handlers stay requireAuth-only, deliberately, per DASHBOARD_SPEC.md §9.3 and MOD-3's own scope (fixing that gate and adding its audit-log call happen together, one pass through that code, not two).

Verification: gofmt clean across all six touched files (internal/auth/auth.go, internal/auth/auth_test.go, internal/ui/router.go, internal/ui/routes.go, internal/handlers/api/router.go, internal/handlers/api/router_test.go, internal/handlers/api/router_security_test.go), full go build/go vet/go test ./... clean, tree-wide completion sweep for every old constant name and .CanAccess( confirmed zero remaining references outside auth.go's own explanatory "// was PermXxx" comments.

Cross-ref: CHANGELOG 0.3.35.

