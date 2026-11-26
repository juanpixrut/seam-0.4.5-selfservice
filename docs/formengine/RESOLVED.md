# formengine — Resolved

Closed stage-tracker items, newest first. Append-only.

## [0.3.11] FE-24 — Full regression/parity test vs. the existing static Asset form; retire the static form once parity holds (v0.3.11, 2026-07-31)

Theme: stage-7-asset-form-parity · closed 0.3.11 · 2026-07-31


Trigger: FORMENGINE_DEVELOPMENT_PLAN.md stage 7.
Scope: This is the completion gate for T-01 in the main register -- T-01 doesn't close until this item does. Per FORM_ENGINE_SPEC.md $13.3, the static Asset form is only removed after parity is proven, not before.

Carried forward from T-13 (closed 2026-07-28): the required-field "*" convention needs a decision before the parity comparison here can be fair. The current 42-field Asset schema has "*" hand-appended to literal titles for fields that look required in the static form (e.g. "Nombre *"), including region_id specifically, which is labelled "*" but never actually HTML-required in the real static form (FE-22's finding). Once titles resolve through real i18n keys, whether "required" is baked into the resolved string or driven separately by the schema's own `required` array needs one consistent design across all 42 fields -- and that's also the moment to decide whether region_id becomes genuinely required or the asterisk is simply dropped, rather than guessing at one field in isolation.

Resolved, 2026-07-28: driven entirely from f.Required, never baked into title text -- the same "structured over embedded-in-prose" reasoning as the queryfy structured-errors proposal filed the same session, applied here first and cited there as independent confirmation the pattern generalizes. fkPopoverAwareLabel (renderer.go) appends " *" when f.Required is true; the 4 fields that had it hand-typed (name, type_id, criticality -- genuinely required -- and region_id, which was not) had it stripped from asset.json's literal titles, confirmed as the complete set by checking every field's title against its actual required-array membership, not assumed to be only the 3 already known about. region_id's known inconsistency (labelled required, never enforced) is resolved as a side effect of using a single source of truth, not a separate decision made about that one field. New test (TestAssetSchema_RequiredIndicatorMatchesRequiredArray) checks the label's actual rendered text, after an early version of the test wrongly checked the input/select element's own attributes instead -- caught by the test failing on first run, not assumed correct. FORM_ENGINE_SPEC.md documented at §12.

One of FE-24's two carried-forward prerequisites resolved. T-17 (locale selection + bilingual content) remains open and still gates FE-24's actual "retire the static form" step -- per FORM_ENGINE_SPEC.md §13.3, that step requires the dynamic engine to have "demostrado paridad funcional completa en producción" (demonstrated complete functional parity in production), a bar this session can build automated tests toward but cannot itself clear -- production validation is the tech team's own deployment process, not something buildable here. FE-24's parity/regression testing work can proceed without T-17 first; the retirement step cannot.

Regression/parity testing started, 2026-07-29 -- began with the riskiest, most uncertain part flagged earlier this session: systematic field-by-field comparison of validation.AssetCreateSchema/AssetUpdateSchema (static, Spanish-keyed) against asset.json's compiled schema (dynamic, English-keyed), extracted programmatically from both real sources rather than transcribed by hand, cross-checked against the real English->Spanish mapping in asset_xolu_fields.go. Found three real things, not zero as an optimistic guess might have assumed:

1. AssetUpdateSchema's own doc comment ("all optional except name") doesn't match its code (name is Optional() there too) -- a real doc/code mismatch, noted but not itself fixed (out of this task's scope).
2. asset.json had zero minLength/maxLength constraints anywhere, while the static schema enforces MaxLength on 18 fields. Fixed: all 18 added, verified programmatically (field extraction, name mapping, and collision-checking all scripted, not hand-transcribed), with name specifically raised from the static form's 200 to 500 per the tech team's own reconsideration ("may be worth raising, not just copying") -- reasoned as generous enough for descriptive industrial-equipment names while still bounded, not left uncapped.
3. asset.json's 8 date fields use "format": "date" -- confirmed empirically (a standalone queryfy test) that jsonschema.FromJSON silently ignores "format" entirely; every value tested passed, including "not-a-date" and "2026-13-99". This is a real, currently-unfixed gap: the date fields have no validation at all in the dynamic form today, a regression versus the static form's crude-but-present MaxLength(10). Not fixed in this pass -- MaxLength(10) could be re-added as a stopgap, but that's a separate decision from the maxLength additions above, which were direct 1:1 matches to real static-form values; flagging for a follow-up, not silently bundling a new decision into this one.

A fourth, more structural finding: the dynamic form uses one schema (asset.json) for both create and update, so it required name/type_id/criticality on update too -- the static form's own AssetUpdateSchema makes all three optional there. Resolved per the tech team's explicit choice ("build a separate update-specific schema variant, mirroring the static form's own create/update split"): new asset.SchemaForUpdate() (internal/formengine/schemas/asset/asset.go) returns the schema with its required array emptied, used only for AssetUpdateDynamic's validation step -- rendering still uses the unmodified Schema(), so the visual "*" indicator is identical on create and edit regardless of which validation path applies, consistent with region_id's own long-standing "asterisk reflects importance, not literal enforcement" precedent in this same package.

Tests: existing TestAssetUpdateDynamic_ValidationFailure_DoesNotCallXolu updated (its old trigger, missing name/criticality, no longer produces a failure -- switched to an invalid enum value, which still fails regardless of required-ness). New TestAssetUpdateDynamic_NameAndCriticalityOptionalOnUpdate is the direct proof of the split. New TestAssetCreateDynamic_NameMaxLengthEnforced proves the raised cap is genuinely enforced, not just present in the schema file -- checked the exact boundary (500 passes, 501 fails), not just "some large value fails". Stress-run 10x clean.

Remaining for FE-24's regression/parity work: field-coverage comparison (does dynamic handle every field static does, and vice versa), persistence parity (same logical input through both paths, diff the resulting xolu documents), and the date-field validation gap noted above.

"Feature complete + usable by a human" itemised and executed, 2026-07-29, following the tech team's own framing: "I can perform production validation as soon as the new dynamic form system is feature-complete, and a full data round trip can be verified." Four items identified by direct investigation (not guessed at), all four now closed:

1. Date-field validation gap -- turned out to already be fixed by the earlier 18-field maxLength pass (the static schema's fecha_*/vto_comodato fields all had MaxLength(10), and those are these same 8 date fields). The 0.2.46 changelog entry calling this "unfixed" was a genuine mistake in this session's own record-keeping, corrected here. Added a direct test proving it.

2. AssetUpdateDynamic had no equivalent to TestAssetCreateDynamic_XoluError_Returns500 -- a real, asymmetric test gap. Added.

3. A real, previously-undiscovered, structural gap: rendering the dynamic Asset form produced zero <form> tags in the output at all -- confirmed by an actual render, not assumed. No amount of backend/logic testing (validation, persistence, translation, all already thoroughly covered) would ever have caught this, since it's purely about whether the HTML has a submit mechanism a browser can use. renderForm's own comment ("Buttons and the full-page vs. modal wrapper stay the caller's job") confirmed this was always a deliberate architectural choice, not an oversight in formengine -- the gap was that no caller (my own handlers) had actually done that job yet. New internal/ui/asset_dynamic_form_wrapper.go (wrapAssetDynamicForm) mirrors AssetFormPage's exact existing pattern (action/method/hx-post logic, the correct modal system -- SeamModal/#seam-modal-body, not SeamModal2, which the masters module's own wrapper targets and would have been the wrong thing to copy), including a genuinely subtle detail: the submit button, though outside the <form> and linked via form=, needs its own duplicate hx-post/hx-include/hx-target/hx-swap attributes to avoid a race between HTMX's click listener and the native form-submit event. Wired into all four handlers. Also corrected a second mistake made earlier this session while investigating this: an earlier claim that "no delete route exists for assets at all" was wrong -- one exists via the API layer (DELETE /api/v1/assets/{id}), confirmed from source, and conditionalDeleteButton (reused directly, not duplicated) is now wired into the wrapper too.

4. Navigation was entirely disconnected from the dynamic form -- confirmed directly: assets.go's list (EditURL, CreateURL, both the auto-open modal script and the empty-state create button) and asset_detail.go's own edit link all pointed at the static routes exclusively; /assets/dynamic/* was reachable only by typing the URL by hand. All 5 real entry points repointed. A full sweep of remaining "/assets/%d"-style references confirmed the rest are either the static form's own internal submit target (correctly unchanged, since the static form stays registered as a fallback) or links to the detail/view page specifically, which remains static-only by design (never in scope) -- not further gaps.

Round-trip proof built: TestAssetDynamic_FullRoundTrip_CreateThenReadBack, the first test to chain create-side and read-side translation together (every prior test exercised AssetValuesToXolu or AssetValuesFromXolu against its own independent fixture) -- submits via the dynamic form, captures what was actually persisted, serves that back as if xolu now has it, confirms AssetEditDynamic displays exactly what was submitted, across scalars, an FK-style id, and both array-field types.

Two real test-quality findings surfaced while building and stress-testing this, neither a production bug:
- A mutation test (deliberately submitting a different value, confirming the assertion would catch the mismatch) caught that a bare "500" substring check for service_configs' threshold was a false positive -- Tailwind classes like text-gray-500/focus:ring-indigo-500 make that number appear in the page regardless of whether the field round-tripped correctly. Fixed by checking the precise value="500" attribute pattern instead, then re-verified with the same mutation technique.
- A 30-iteration stress run caught the round-trip test itself intermittently failing on criticality's selected-option check specifically. Diagnosed with targeted logging rather than guessed at: the persisted value and the "selected" marking were correct on every single iteration, including failing ones -- the two-attribute value="Media" selected="selected" concatenation the assertion checked for is not rendered in a guaranteed order (confirmed both orderings appearing across different runs of the identical test). Fixed by checking both possible orderings explicitly. 50-iteration stress run clean after the fix, plus a 15x stress run of the complete dynamic Asset form suite together.

Remaining for FE-24: field-coverage comparison and persistence-parity work noted above (now substantially covered by the round-trip test, though a fuller field-by-field sweep across all 42 fields hasn't been done), plus the still-open date-field format:"date" no-op in queryfy (distinct from the maxLength stopgap, which is in place). Production-proven parity (§13.3) remains the tech team's own process, now genuinely startable per our own stated gate.

A real, genuine parity gap found and confirmed, 2026-07-29 -- the tech team correctly pushed back on an earlier claim in this same turn that only checked for a <form> tag's presence, not whether the form actually rendered. Full field-by-field inspection of a real running instance found it: on a fresh, unseeded xolu (exactly what launch.sh produces on its own, before any data exists), every FK-sourced field failed to render at all, showing a raw XOLU-QL004 "entity does not exist" error in place of the widget -- including type_id, which is required, making the form genuinely unsubmittable in that state. Root cause confirmed by direct comparison: the static form's own loadMarcas and every asset_types query explicitly discard the error and fall back to an empty list (dropdown still renders, just empty); AssetSourceResolver.Resolve deliberately wraps and propagates it instead. Confirmed the seed scripts themselves need no changes for xolu's current shape (ran seed-01-constructec.py live, zero code changes needed, only a missing local Python dependency) -- once seeded, every field renders correctly, which is why this specific gap only manifests on a genuinely fresh instance, not the seeded demo path build-demo.sh already produces.

Deliberately not fixed in the same turn as found, per instruction ("no need to do anything special" -- the ask was specifically to verify the seed scripts, not fix this) -- filed here so it isn't lost. Worth real design thought before fixing, not a blind copy of the static form's blanket swallow-every-error approach: that would also hide genuine failures (a real network error, a malformed query) behind a silently-empty dropdown with no indication anything's wrong, which is a different problem than "entity doesn't exist yet on a fresh instance" specifically.

Fixed, 2026-07-31, as the confirmed blocker from a full FE-24 evidence-gathering pass (the tech team's own request: "collect all the evidence needed to guarantee FE-24 is achievable"). Re-verified live before fixing, not assumed still true from the original 2026-07-29 finding -- reproduced identically against xolu 0.19.3 on a genuinely fresh tenant. New `entityNotYetCreated` (internal/ui/asset_source_resolver.go) checks both the xolu error code (XOLU-QL004, `errors.As` against `*xolu.Error`, not string-matching) AND the specific "entity '%s' does not exist" message pattern (confirmed generated from exactly one place, `pkg/oql/validator.go`, in xolu itself) -- deliberately narrower than the code alone, since XOLU-QL004 is a general "query failed" bucket (pkg/errors/errors.go) covering more than just this one case; a same-code failure for a different reason still propagates as a real error. Three new tests cover the boundary precisely: the fresh-tenant case (empty list, no error), a same-code-different-message failure (still errors), and a different-code failure (still errors). Re-verified live against the exact original repro (fresh, unseeded xolu 0.19.3 instance): every FK-sourced field, including the required type_id, now renders as a real, functional, empty `<select>` instead of an error block. Re-verified against a fully seeded instance too, confirming no regression -- real master data still populates every dropdown correctly.

The field-by-field/enum/maxLength structural parity work this same evidence pass did (field coverage 42/42 both sides, required fields identical, all 5 enum value sets identical, MaxLength identical except one already-documented deliberate difference) found no other gaps. With this fix in, FE-24's remaining step is the tech team's own production validation (§13.3) -- nothing else identified as blocking it.

Production validation given, 2026-07-31 (the tech team, hands-on): "It opens, modifies, saves correctly... Assets is good enough to give it the greenlight." Real usage, not a scripted check -- the same session surfaced two genuine i18n gaps neither of us had caught by inspection alone (formengine's document-links/repeatable widget chrome; internal/ui/assets.go's own list page, a completely different hand-coded rendering path formengine's own i18n work never touched), both found and fixed the same way: confirmed live, fixed, re-verified against both the English and Spanish demos with no regression. §13.3's own bar -- "demostrado paridad funcional completa en producción" -- is met by this: real create/edit/save use, not a walkthrough.

Static form retired, 2026-07-31 (the tech team's explicit instruction: back it up to /attic first, then remove carefully, not delete outright). Full audit before touching anything -- every function/type/route the static form touched was checked for external call sites across the whole internal/ui package, not assumed safe from name or file location alone. Two genuinely shared dependencies found and preserved: assetDynamicFields/assetDynamicField and the GET /assets/type-fields route (AssetTypeFields) -- the dynamic form's own type_id select widget (renderAssetTypeSelect, formengine/widgets.go) targets this exact same endpoint to reload identity/state fields on change; removing it would have broken the dynamic form, not just retired the static one. Also preserved: conditionalButtonText (shared with asset_dynamic_form_wrapper.go and pages.go's Rules form), AssetTypeOption (used broadly by the list page, dashboard, and several unrelated asset-type dropdowns), and validation.AssetCreateSchema/AssetUpdateSchema themselves (the REST API validates against these directly, independent of the UI layer).

/attic/static-asset-form/ holds full pre-removal snapshots of every touched file plus a README cataloguing exactly what moved and what deliberately stayed, with the verification reasoning for each -- not a bare dump. Removed: 4 routes (GET /assets/new, POST /assets, GET /assets/{id}/edit, POST /assets/{id}), 5 handlers (AssetNew, AssetEdit, AssetCreate, AssetUpdate, collectDynamicFieldValues), AssetFormPage and 15 widget helpers exclusive to it, the schemaAssetCreate/schemaAssetUpdate local aliases, AssetEnumValueFromXoluValue (its underlying assetEnumValueFromXolu table stays -- the dynamic form's own read path uses it independently), and 4 test functions. One real fix needed mid-removal, not just deletion: AssetDetail's own ServiceConfigs field became orphaned once ServiceConfig's type went with the form -- confirmed nothing populated or read it on the view page (dead weight inherited from the struct being shared with the form), removed the field rather than resurrecting the type.

Verified live afterward, not just via build+test: old static routes genuinely 404; the read-only detail/view page still renders correctly; the dynamic create/edit form still fully works (GET both, real data); the Assets list page still works; the REST API still responds correctly (confirming validation.AssetCreateSchema's wiring is intact, independent of the UI layer). Full build/vet/test suite clean throughout, zero regressions.

FE-24 closed, 2026-07-31 (v0.3.11). Both halves of this item's own scope are done: parity proven in production use, static form retired. T-01 unblocked.

## [0.2.37] FE-23 — Convert Asset form handlers to the dynamic engine (v0.2.37, 2026-07-29)

Theme: stage-7-asset-form-parity · closed 0.2.37 · 2026-07-29


Trigger: FORMENGINE_DEVELOPMENT_PLAN.md stage 7; direct request 2026-07-28.

Real bug caught starting this task, 2026-07-28: cross-checked every mi.Name(...) call in asset_detail.go against the schema (triggered by needing exact field names for the POST-handling work this task requires) and found the schema used "nombre"/"tipo_id" for the asset's name and type -- but the real form's actual HTML field names for exactly those two fields are "name"/"type_id" (English), inconsistent with the rest of the form's otherwise-consistent Spanish naming. Undetected, a dynamic-schema POST would have silently submitted every asset with an empty name against the real AssetCreate handler (r.FormValue("name")). Fixed in asset.json, FORM_ENGINE_SPEC.md §12 (dated correction note added), and the integration tests. Confirmed via the existing parseDocumentLinksToAny/parseDocumentLinks helpers that document-links field naming ({field}_nombre[] etc.) already matches this schema's convention exactly -- no changes needed there.

Scope for the actual handler conversion, not yet started: a xolu-backed SourceResolver wrapping the existing h.load* methods (loadMarcas, loadModelos, loadRegiones, loadUbicaciones, loadCustodiosSimple, loadMonedasSimple, loadCentrosCosto, loadAlertas) plus asset-types; a rendering path calling FormEngine.Render() for GET (new + edit); POST/submission handling, which needs care given the depth of the existing AssetCreate/AssetUpdate handlers' coupling to the static form's structure (validateForm against schemaAssetCreate, collectDynamicFieldValues, AssetFormData/AssetDetail) -- likely wants a new, parallel path per §13.3's coexistence model rather than a destructive rewrite, so FE-24 has something to compare against.

i18n scope decision, 2026-07-28: checked docs/I18N_SPEC.md $4 before starting POST/validation work, not assumed. $4.4 (queryfy-error-to-i18n-key mapping) is already explicitly part of T-13's own original filing, and T-13's dependency graph already sequences it to land before FE-24, not before FE-23 -- confirmed this wasn't a new scope call to make, just one to honour. Also checked queryfy's actual FieldError struct: flat Path/Message/Value, no error-type enum -- whoever builds $4.4's mapping has to pattern-match Message text regardless of when it's built, so there's no cheap "structure now, translate later" middle ground; building it inside FE-23 would mean duplicating T-13's already-scoped work in a narrower, Asset-only form ahead of its proper formengine-wide design. Decision: FE-23's POST handling surfaces queryfy's raw FieldError.Message as-is, with a visible marker at the exact hook point T-13 needs -- not silently incomplete, not pre-empting T-13's work either.

Schema field names renamed to English, 2026-07-28 (the tech team's explicit decision, confirmed): all 42 top-level properties plus nested document-links/repeatable item fields renamed from Spanish to English (criticidad->criticality, calidad_tenencia->tenure_type, custodio_id->custodian_id, vto_comodato->loan_expiry, and so on) -- code identifiers, including schema field names, stay English throughout; bilingual UI support is entirely §4/T-13's job, not something field names should carry. Applied via structured JSON manipulation (parse/rename/rewrite), not text regex, given the real risk of collateral damage on nested fields sharing names with unrelated top-level ones (confirmed real: a first attempt at the earlier name/type_id fix already broke document-links' own nombre sub-field once this session). Caught and fixed two real bugs the rename itself introduced: x-seam-visible-when's field reference (calidad_tenencia -> tenure_type) is a string VALUE, not a JSON key, so the structural rename didn't touch it and it needed a separate fix; an i18n key path (assets.forms.asset.field.vto_comodato) embedded the old field name even though it isn't a schema key at all. FORM_ENGINE_SPEC.md kept in sync: §12's fragment, §3b's visible_when worked example (automatically consistent since it referenced the same quoted field names), §7.3's fk-popover field table, and one stray prose mention, all updated -- with one deliberately-excluded reference at §5 caught and left alone (entity:alertas' own label template uses tipo/criticidad as ITS OWN master-entity fields, unrelated to Asset's renamed fields, confirmed by reading the surrounding table row before assuming).

Built, 2026-07-28: the translation table itself, in FE-23's persistence code, mapping these English schema names to the Spanish attribute keys xolu already stores and the coexisting static form still reads/writes -- needed before any GET/POST handler work can actually run against real data.

Also confirmed, 2026-07-28: a proposed lossless anchor<->identifier convention (hyphen -> double underscore) was analysed for whether the reverse mapping (double underscore -> single underscore, single underscore -> hyphen) fit the existing data better. It doesn't -- every widget-type/source-entity string with hyphens has zero underscores, every schema field name with underscores has zero hyphens, and the codebase's own established pattern for deriving a Go identifier from a hyphenated string is hand-written CamelCase with the hyphen dropped (loadCentrosCosto), not underscore-encoding at all. No current caller needs a lossless round-trip conversion anywhere in the codebase as it stands, so the function itself wasn't built -- speculative utility code with no real caller to validate against, avoided on the same grounds as elsewhere this session. Documented as a convention to apply if and when a concrete need appears: anchor's hyphen -> identifier's double underscore, anchor's own underscore stays a single underscore; invariant is that no anchor string may ever contain a literal double underscore, which holds naturally for both kebab-case and snake_case conventions.

internal/ui/asset_xolu_fields.go: AssetValuesToXolu / AssetValuesFromXolu, built and tested (13 tests). Table's Spanish side isn't invented -- it's the exact field list from the mi.Name(...) audit that caught the name/type_id bug (FE-22). One-to-one invariant enforced by the reverse map's own init() (panics on any collision, so a bad edit fails at binary startup, not silently at runtime) and re-asserted by a standing test rather than only checked once. Handles the three document-links arrays (loan_links, lifecycle_links, document_links; nombre/url/tipo <-> name/url/type) and service_configs (medida/umbral/alerta_id <-> measure/threshold/alert_id) recursively, confirmed by round-trip tests, not just one-directional ones. Unrecognised fields pass through rather than get dropped, so extending the schema later doesn't silently lose data before this table catches up.

AssetSourceResolver built and tested, 2026-07-28: internal/ui/asset_source_resolver.go, implementing formengine.SourceResolver for all 9 x-seam-source values the Asset schema uses (marcas, modelos, regiones, ubicaciones, custodios, monedas, centros-costo, alertas, asset-types). Every OQL query and label format read directly out of masters_routes.go/asset_detail.go before being copied, not reconstructed from FORM_ENGINE_SPEC.md §5's table alone -- which turned out accurate for all nine sources this time, confirmed individually, not assumed from its two prior known inaccuracies elsewhere in the spec. Preserved custodios' WHERE estado = true filter (load-bearing: the old form only ever offered active custodians) and confirmed entity:centros-costo's actual xolu table is centros_costo (underscore), distinct from the hyphenated source key and URL path.

Real bug caught during implementation, not by a test discovering it first: toString() -- an existing helper already used throughout internal/ui -- only handles the string case and silently returns "" for anything else, including the float64 xolu sends for numeric ids. Read its actual source before trusting it for row["id"]; fixed to route through toInt64 + fmt.Sprintf instead. Kept as a standing regression test regardless of having been caught before shipping.

Tested against a real httptest.Server simulating xolu's actual POST /api/v1/oql/query endpoint (not /oql/query as pkg/client's OQL method's own path constant alone would suggest -- do() prepends /api/v1, caught by the first test run against the wrong path, not read from source first). 21 new tests, stress-run 10x clean.

GET rendering path built and tested, 2026-07-28: internal/ui/asset_dynamic_handlers.go, AssetNewDynamic + AssetEditDynamic, routed at /assets/dynamic/new and /assets/dynamic/{id}/edit -- distinct paths per §13.3's coexistence model, verified not to collide with the existing wildcard routes by directly constructing the router in a throwaway check (Go 1.22's ServeMux panics at registration time on genuinely ambiguous overlapping patterns; confirmed no panic, not inferred from other tests passing). AssetEdit's own entity.Data field reads (name, criticidad, ubicacion_fisica_id, id_padre, custodio_id, calidad_tenencia, vto_comodato, and the rest, checked line by line) independently confirmed all 42 of asset_xolu_fields.go's Spanish-side keys are exactly right -- a live cross-check against the actual handler that reads this data today, not just the earlier mi.Name(...) audit.

Tested against a combined httptest.Server simulating both endpoints AssetEditDynamic needs (OQL for AssetSourceResolver, GET /api/v1/assets/{id} for the entity itself, confirmed as the real path+shape from pkg/client's Get method directly). 5 new tests: HTMX render, non-HTMX redirect, real-asset load with Spanish->English translation verified in the rendered output, unknown ID -> 404, malformed ID -> 404.

Remaining for FE-23: POST/submission handling -- paused, 2026-07-28 (the tech team's call): building POST error handling around raw queryfy FieldError.Message now, then having T-13 wrap it later, understated what T-13 actually needs to change. §4.4's mapping requires real parameterisation (must be >= N -> core.errors.min_value with an actual {min} value), meaning the error needs to carry structured data (which rule, which bound) through the handler, not just a flat string a later step could re-translate. A good error message also names the field, and the field's display name is itself an i18n-resolved value per §4.1 -- another reason error text built now wouldn't match what T-13 actually resolves. T-13 goes first; nothing had been implemented for FE-23's POST path yet, so there's nothing to unwind.

Resuming FE-23, 2026-07-28, T-13 now closed. Started by reading AssetCreate/AssetUpdate's real POST-parsing code directly (routes.go), not assumed from the earlier field-name audit -- and found a real bug in asset_xolu_fields.go before building anything on top of it. service_configs' threshold sub-field: the table said "umbral" (matching the submitted svc_umbral[] form field name), but both real handlers persist it under "umbral_alarma" instead -- confirmed identical in AssetCreate and AssetUpdate. Left unfixed, the dynamic form would have silently lost every asset's threshold value the first time it got edited through either form after being touched by the other -- exactly the coexistence hazard this table exists to prevent, caught by reading the real handler before depending on the table, not after shipping something broken. Fixed in asset_xolu_fields.go and its test.

Also confirmed while there: the static AssetEdit handler never populates ServiceConfigs on load at all -- ServiceConfigs is declared on AssetDetail but nothing in AssetEdit's body sets it. Not a static-form bug this session needs to fix (out of scope, and touching the static form contradicts the "don't touch it, it's being retired" decision from earlier) -- just confirms there's no existing read-path convention for this field to match. AssetValuesFromXolu reading entity.Data["service_configs"] directly means the dynamic form will actually show more complete data here than the static one currently does, which is a difference worth knowing about for FE-24's parity comparison, not a regression to fix now.

Investigation completed, 2026-07-28: confirmed the array-field submission convention directly from repeatable.go's own doc comment (written during stage 4/5, explicitly anticipating this exact moment) -- {field.Name}_{item.Name}[], the same for document-links and repeatable. For Asset: loan_links_name[]/loan_links_url[]/loan_links_type[], same pattern for lifecycle_links and document_links; service_configs_measure[]/service_configs_threshold[]/service_configs_alert_id[] for repeatable. Scalar fields confirmed as plain mi.Name(f.Name).

One real gap found and fixed before it could block anything: formengine.ParseSchema (was parseSchema, package-private) had no exported path -- Form and Form.Compiled are both exported types/fields, but nothing outside formengine could actually construct a *Form to reach them. Exported it (schema.go + engine.go + 4 test files updated, all internal call sites), since inventing a parallel, formengine-owned validation API to work around this would duplicate queryfy's own interface for no reason. Also fixed a stale doc comment on the same function referencing "until stage 3 adds tab handling" -- stage 3 has been done for a long time.

Remaining before the handler itself can be written: confirm which of the 42 fields are integer-typed in asset.json (type_id, brand_id, model_id, etc.), since r.FormValue always returns strings but the compiled schema expects properly-typed values for those fields -- the static form's own assetData map does this coercion via toInt64FromString before validation, and the dynamic handler needs the same, not a guess at which fields need it.

Type-coercion question resolved, 2026-07-28, by checking asset.json's real type declarations against the static form's actual code, not by inference: 9 fields are genuine integer FKs (type_id, brand_id, model_id, location_id, region_id, parent_id, custodian_id, currency_id, cost_center_id) and need int64 conversion before persistence. The 6 fields asset.json itself declares "number" (latitude, longitude, purchase_price, current_price, temp_min, temp_max) are persisted AND read back as plain strings by AssetCreate, AssetUpdate, and AssetEdit alike -- confirmed identical across all three, contradicting the schema's own stated type. The dynamic handler matches the real, coexisting convention, not the declared one.

Also confirmed before writing any handler code, per instruction to verify rather than assume: read queryfy's actual transform system (TransformSchema, ValidateAndTransform, the transformers package) directly. None of it is in play here -- jsonschema.FromJSON never attaches a Transform when building number/integer schemas from a plain JSON Schema document, confirmed by reading convertNumber directly. The "strings validate fine for number fields" behaviour comes from NumberSchema.Validate's own Loose-mode internal coercion check (toFloat64WithMode), not a transform pipeline, and critically doesn't mutate the value -- validation passing doesn't mean persistence gets a typed value for free. Manual coercion for the 9 FK fields is necessary and correct, not a workaround for a queryfy feature that should have been used instead.

POST handlers built and tested: internal/ui/asset_dynamic_post.go, AssetCreateDynamic + AssetUpdateDynamic, routed POST /assets/dynamic and POST /assets/dynamic/{id} (verified no mux conflict the same direct-construction way as the GET routes). Array field parsing confirmed against repeatable.go's own documented {field}_{item}[] convention. A real crash caught by the test suite itself, not by review: h.translator (a *i18n.Translator) is nil in test setup; assigning a nil concrete pointer straight into the Translator interface field produces a "typed nil" that formengine's own nil checks can't see, and the next call into it panics on the nil receiver. Fixed with a centralised h.formEngineTranslator() helper (routes.go) that every FormEngine-constructing call site now uses instead of touching h.translator directly, plus a warning on FormEngine.Translator's own doc comment for any future caller outside Seam.

A second, more fundamental bug surfaced while building error handling: queryfy's error path for a nested array-item constraint (e.g. document_links[0].url) is dotted and bracketed, not a flat field name -- confirmed empirically, not assumed -- so the existing errs map[string]string mechanism (an exact fieldName lookup) would have silently dropped every such error. Fixed jointly with the tech team: RenderContext gained an additive Statuses []FieldStatus field (element/status/message, status one of ERR/WRN/INF) alongside the existing errors map, never replacing it -- every pre-existing caller and test keeps working unchanged, confirmed by the full suite passing with zero test modifications needed for the addition itself. resolveFieldError (formengine/status.go) merges both sources, map wins if both apply, with matching precise enough to reject false-positive prefix collisions (a field named X must not match a status for a differently-named field that happens to start with X). Only ERR entries produce a blocking error today; WRN/INF are recorded and matchable but no widget gives them distinct visual treatment yet -- deliberately not built now, since there's no real caller asking for it yet, matching this session's standing rule against speculative code.

FE-23 complete: schema rename, translation table, SourceResolver, GET path, and now POST handling, all built and tested. Closing here.

Cross-ref: CHANGELOG 0.2.37.

## [0.2.17] FE-22 — Author the full 4-tab Asset schema matching the current static implementation (v0.2.17, 2026-07-28)

Theme: stage-7-asset-form-parity · closed 0.2.17 · 2026-07-28


Trigger: FORMENGINE_DEVELOPMENT_PLAN.md stage 7; direct request 2026-07-28 ("the existing Assets form needs to be converted to the new forms system/invariant").
Progress, 2026-07-28: widget primitives this task depends on are now built and tested -- document-links (§4.2), repeatable (§4.2's generalisation, over the same document-order-preserving Field.Items schema.go added), asset-type-select (§12's htmx-triggered widget, verified against assetTypeSelectWithHTMX's real attributes), x-seam-fk-popover (§7), and asset-picker (§8.3's MVP stub). What's left for this item specifically: extract §12's Asset schema fragment into a real schema file, verify it against the static form field-by-field, and add vto_comodato's visible_when (present in §3b.2's own worked example, absent from §12's fragment -- an inconsistency to resolve when the real file is written, not carried forward silently).

Real bug found and fixed along the way, not specific to this task but discovered while testing it: minty's Element.Attributes is map[string]string internally (confirmed by reading builder.go/minty.go directly), and Go deliberately randomizes map iteration order per process -- so the same field, rendered twice from byte-identical input, can legally emit its HTML attributes in a different order each time. Proved empirically: a throwaway test rendering the same schema 50 times in a loop failed on the very first repeat. This had already caused one genuinely intermittent test failure (TestMultiTab_ActiveTabSkipsToFirstVisibleWhenFirstIsHidden) and was a latent bug in 5 more tests that used fixed-byte-window slicing to check whether two attributes appeared on the same element -- all 6 fixed, and a shared extractTag(html, idx) test helper added (finds the real enclosing <tag...> boundaries instead of guessing a byte count) so future formengine tests don't reintroduce the same assumption. Not a Go-version issue and not fixable by one -- map iteration has been intentionally randomized since essentially Go 1.0, in every version since, deliberately, so code can't come to depend on it.

Cross-ref: CHANGELOG 0.2.17.

## [0.2.14] FE-21 — ModalTriggerAttrs(schema, url, label) (v0.2.14, 2026-07-28)

Theme: stage-7-asset-form-parity · closed 0.2.14 · 2026-07-28


Trigger: FORMENGINE_DEVELOPMENT_PLAN.md stage 7; ROADMAP.md step 7.
Scope: FORM_ENGINE_SPEC.md $15.4 (resolved: confirmed) -- handlers never choose SeamModal vs SeamModal2 directly, the engine decides from schema width.

Cross-ref: CHANGELOG 0.2.14.

## [0.2.12] FE-20 — Apply permissions in the renderer (v0.2.12, 2026-07-28)

Theme: stage-6-field-permissions · closed 0.2.12 · 2026-07-28


Trigger: FORMENGINE_DEVELOPMENT_PLAN.md stage 6.
Scope: FORM_ENGINE_SPEC.md $3c.5. Interface/hook only for the eventual Administration-module permission UI (T-02) -- not that UI itself.

Cross-ref: CHANGELOG 0.2.12.

## [0.2.12] FE-19 — PermService.Resolve() -- role-priority table (admin=100..viewer=10), most-restrictive tiebreak (v0.2.12, 2026-07-28)

Theme: stage-6-field-permissions · closed 0.2.12 · 2026-07-28


Trigger: FORMENGINE_DEVELOPMENT_PLAN.md stage 6; FORM_ENGINE_SPEC.md $15.3 (resolved: Option C, explicit role priority).

Cross-ref: CHANGELOG 0.2.12.

## [0.2.12] FE-18 — FieldPermission model (v0.2.12, 2026-07-28)

Theme: stage-6-field-permissions · closed 0.2.12 · 2026-07-28


Trigger: FORMENGINE_DEVELOPMENT_PLAN.md stage 6; ROADMAP.md step 6.
Scope: FORM_ENGINE_SPEC.md $3c.3.

Cross-ref: CHANGELOG 0.2.12.

## [0.2.11] FE-17 — Implement $15.2 merge-conservative hidden-field policy + x-seam-hidden-reject opt-in (v0.2.11, 2026-07-28)

Theme: stage-5-conditional-visibility · closed 0.2.11 · 2026-07-28


Trigger: FORMENGINE_DEVELOPMENT_PLAN.md stage 5; FORM_ENGINE_SPEC.md $15.2 (resolved: dual policy).

Cross-ref: CHANGELOG 0.2.11.

## [0.2.11] FE-16 — Server-side queryfy re-evaluation of visibility (v0.2.11, 2026-07-28)

Theme: stage-5-conditional-visibility · closed 0.2.11 · 2026-07-28


Trigger: FORMENGINE_DEVELOPMENT_PLAN.md stage 5.
Scope: FORM_ENGINE_SPEC.md $3b.6 -- client evaluation cannot be trusted alone.

Cross-ref: CHANGELOG 0.2.11.

## [0.2.11] FE-15 — x-seam-visible-when client-side JS evaluator (v0.2.11, 2026-07-28)

Theme: stage-5-conditional-visibility · closed 0.2.11 · 2026-07-28


Trigger: FORMENGINE_DEVELOPMENT_PLAN.md stage 5; ROADMAP.md step 5.
Scope: FORM_ENGINE_SPEC.md $3b.

Cross-ref: CHANGELOG 0.2.11.

## [0.2.8] FE-14 — Implement $3.7 detail/embed lifecycle interaction with parent form (v0.2.8, 2026-07-28)

Theme: stage-4-details-and-embeds · closed 0.2.8 · 2026-07-28


Trigger: FORMENGINE_DEVELOPMENT_PLAN.md stage 4.

Cross-ref: CHANGELOG 0.2.8.

## [0.2.8] FE-13 — Implement embeds sub-form rendering, SeamModal2 wiring (v0.2.8, 2026-07-28)

Theme: stage-4-details-and-embeds · closed 0.2.8 · 2026-07-28


Trigger: FORMENGINE_DEVELOPMENT_PLAN.md stage 4.
Scope: FORM_ENGINE_SPEC.md $3.6. Needed for the Asset form's sensor_binding embed.

Cross-ref: CHANGELOG 0.2.8.

## [0.2.8] FE-12 — Implement details panel rendering (v0.2.8, 2026-07-28)

Theme: stage-4-details-and-embeds · closed 0.2.8 · 2026-07-28


Trigger: FORMENGINE_DEVELOPMENT_PLAN.md stage 4; ROADMAP.md step 4.
Scope: FORM_ENGINE_SPEC.md $3.5. Needed for the Asset form's sensores_vinculados and custodio_history panels.

Cross-ref: CHANGELOG 0.2.8.

## [0.2.7] FE-11 — Multi-tab regression tests (v0.2.7, 2026-07-28)

Theme: stage-3-tabs-and-layout · closed 0.2.7 · 2026-07-28


Trigger: FORMENGINE_DEVELOPMENT_PLAN.md stage 3.

Cross-ref: CHANGELOG 0.2.7.

## [0.2.7] FE-10 — Implement $15.1 schema-load validation: reject section_defs IDs not referenced in any row (v0.2.7, 2026-07-28)

Theme: stage-3-tabs-and-layout · closed 0.2.7 · 2026-07-28


Trigger: FORMENGINE_DEVELOPMENT_PLAN.md stage 3; FORM_ENGINE_SPEC.md $15.1 (resolved: Option B).

Cross-ref: CHANGELOG 0.2.7.

## [0.2.7] FE-09 — Implement tabs/sections/row-layout rendering (v0.2.7, 2026-07-28)

Theme: stage-3-tabs-and-layout · closed 0.2.7 · 2026-07-28


Trigger: FORMENGINE_DEVELOPMENT_PLAN.md stage 3; ROADMAP.md step 3.
Scope: FORM_ENGINE_SPEC.md $3.1-$3.8. Required for the Asset form, which is 4 tabs.

Cross-ref: CHANGELOG 0.2.7.

## [0.1.0-patched250] FE-08 — Parity check: dynamic vs static rendering, these three forms, side by side (v0.1.0-patched250, 2026-07-28)

Theme: stage-2-flat-masters · closed 0.1.0-patched250 · 2026-07-28


Trigger: FORMENGINE_DEVELOPMENT_PLAN.md stage 2.
Scope: First proof the engine actually works end to end before building tab/detail/embed/visibility/permission complexity on top of it.

Cross-ref: CHANGELOG 0.1.0-patched250.

## [0.1.0-patched250] FE-07 — Wire masters handlers for these three to render via formengine; static forms elsewhere untouched (v0.1.0-patched250, 2026-07-28)

Theme: stage-2-flat-masters · closed 0.1.0-patched250 · 2026-07-28


Trigger: FORMENGINE_DEVELOPMENT_PLAN.md stage 2.
Scope: internal/ui/masters_pages.go / masters_routes.go for Region/Currency/Brand only. FORM_ENGINE_SPEC.md $13.3: static and dynamic forms coexist during migration -- do not remove any static form until parity is proven.

Cross-ref: CHANGELOG 0.1.0-patched250.

## [0.1.0-patched250] FE-06 — Author flat JSON Schemas for Region, Currency, Brand masters (v0.1.0-patched250, 2026-07-28)

Theme: stage-2-flat-masters · closed 0.1.0-patched250 · 2026-07-28


Trigger: FORMENGINE_DEVELOPMENT_PLAN.md stage 2; ROADMAP.md step 2.
Scope: No tabs -- flat field lists. First real schemas the engine renders, chosen for simplicity per ROADMAP's own ordering (Region, Moneda, Marca).

Cross-ref: CHANGELOG 0.1.0-patched250.

## [0.1.0-patched249] FE-05 — Unit tests per widget type + schema round-trip test against a live xolu instance (v0.1.0-patched249, 2026-07-28)

Theme: stage-1-core-engine · closed 0.1.0-patched249 · 2026-07-28


Trigger: FORMENGINE_DEVELOPMENT_PLAN.md stage 1.
Scope: Confirms in code, not just in KNOWN_ISSUES.md's verified-non-blocker note, that x-shelf-* extensions round-trip through POST/GET /api/v1/schema/{entity}. Closes the loop on that earlier desk-check.

Cross-ref: CHANGELOG 0.1.0-patched249.

## [0.1.0-patched249] FE-04 — Widget registry + MVP widget set (text/email/url/tel/date/datetime-local/time/number/select/checkbox/toggle/color/textarea) (v0.1.0-patched249, 2026-07-28)

Theme: stage-1-core-engine · closed 0.1.0-patched249 · 2026-07-28


Trigger: FORMENGINE_DEVELOPMENT_PLAN.md stage 1; FORM_ENGINE_SPEC.md $14.
Scope: Build as a registry (map[string]WidgetRenderer or equivalent), not a hardcoded switch -- T-10's future widgets (signature, image, GPS, multi-select, decimal precision) must slot in without restructuring this. This is a load-bearing design decision for the whole engine, not a stage-1-only concern.

Cross-ref: CHANGELOG 0.1.0-patched249.

## [0.1.0-patched249] FE-03 — Implement SchemaToForm entrypoint + queryfy schema compilation wiring (v0.1.0-patched249, 2026-07-28)

Theme: stage-1-core-engine · closed 0.1.0-patched249 · 2026-07-28


Trigger: FORMENGINE_DEVELOPMENT_PLAN.md stage 1.
Scope: Per FORM_ENGINE_SPEC.md §13.2 (public interface) and §9 (queryfy validation, standard JSON Schema properties recognised, x-shelf-* ignored by queryfy).

Cross-ref: CHANGELOG 0.1.0-patched249.

## [0.1.0-patched249] FE-02 — Define RenderContext struct (v0.1.0-patched249, 2026-07-28)

Theme: stage-1-core-engine · closed 0.1.0-patched249 · 2026-07-28


Trigger: FORMENGINE_DEVELOPMENT_PLAN.md stage 1.
Scope: Per FORM_ENGINE_SPEC.md §3c.2 -- the authoritative definition. Includes resolved permission fields, EntityType, EntityID, TrayPermissions, active locale.

Cross-ref: CHANGELOG 0.1.0-patched249.

## [0.1.0-patched249] FE-01 — Scaffold internal/formengine package (engine.go, renderer.go, widgets.go, schema.go, sources.go) (v0.1.0-patched249, 2026-07-28)

Theme: stage-1-core-engine · closed 0.1.0-patched249 · 2026-07-28


Trigger: T-01, stage 1 of FORMENGINE_DEVELOPMENT_PLAN.md.
Scope: Package layout per FORM_ENGINE_SPEC.md §13.1, exactly as specified. No logic yet — empty files with package declarations and doc comments, so subsequent stages have somewhere to land.

Cross-ref: CHANGELOG 0.1.0-patched249.

## — tracker established 2026-07-28 —

No items closed before this point.
