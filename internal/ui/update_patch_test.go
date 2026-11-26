package ui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// This file guards a real, serious bug found 2026-08-03 (reported by
// The tech team: saving Settings/an edited user "destroys or overwrites the
// user's profile"). Root cause: xolu.Client.Update() sends HTTP PUT,
// which is a full-document replace on the server -- confirmed directly
// against a live instance (PATCH with one field genuinely merges;
// PUT with the same field wipes everything else). Every handler that
// called Update with a hand-picked subset of an entity's real fields
// was silently destroying the rest on every edit. Fixed by switching
// every entity-update call site (17 total) from Update to Patch.
//
// Most of those 17 already had existing tests exercising a successful
// update, and switching their mock's registered route from "PUT" to
// "PATCH" was enough to make those tests fail if the bug ever came
// back (a route registered only for PATCH 404s a PUT request). But
// four entities -- Region, Currency, PropDefinition, and Rule -- had
// no such coverage at all: their only existing tests short-circuit on
// validation before ever reaching xolu. These four tests close that
// gap.

func TestRegionUpdate_UsesPatch(t *testing.T) {
	patched := false
	server := mockOLU(t, map[string]http.HandlerFunc{
		"PATCH /api/v1/regions/": func(w http.ResponseWriter, r *http.Request) {
			patched = true
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"message": "updated"})
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.RegionUpdate, "/regions/1", url.Values{
		"description": {"Patagonia"},
	})

	assertRedirectsTo(t, rec, "/regions")
	if !patched {
		t.Error("xolu.Patch was not called -- if this used Update (PUT) instead, every other field on the region would be wiped")
	}
}

func TestMonedaUpdate_UsesPatch(t *testing.T) {
	patched := false
	server := mockOLU(t, map[string]http.HandlerFunc{
		"PATCH /api/v1/currencies/": func(w http.ResponseWriter, r *http.Request) {
			patched = true
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"message": "updated"})
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.MonedaUpdate, "/currencies/1", url.Values{
		"iso_code": {"GBP"},
		"symbol":   {"£"},
	})

	assertRedirectsTo(t, rec, "/currencies")
	if !patched {
		t.Error("xolu.Patch was not called")
	}
}

func TestPropDefUpdate_UsesPatch(t *testing.T) {
	patched := false
	server := mockOLU(t, map[string]http.HandlerFunc{
		// Uniqueness pre-check
		"POST /api/v1/oql/query": func(w http.ResponseWriter, r *http.Request) {
			oqlEmpty(w)
		},
		"PATCH /api/v1/prop_definitions/": func(w http.ResponseWriter, r *http.Request) {
			patched = true
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"message": "updated"})
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.PropDefUpdate, "/properties/1", url.Values{
		"nombre_corto":  {"color"},
		"tipo_dato":     {"alphanumeric"},
		"valor_defecto": {""},
	})

	assertRedirectsTo(t, rec, "/properties")
	if !patched {
		t.Error("xolu.Patch was not called")
	}
}

// TestPropDefUpdate_DataIntegrity is a real data-integrity check,
// added 2026-08-11 alongside the short_name/data_type/default_value
// rename -- none of the 9 pre-existing PropDef/PropAssoc tests
// actually inspected the payload sent to xolu (only whether Patch/
// Create was called, or whether validation failed), so none of them
// would have caught a rename mistake. This one does: asserts the
// actual PATCH body uses the new English keys, not the old Spanish
// ones.
func TestPropDefUpdate_DataIntegrity(t *testing.T) {
	var capturedBody []byte
	server := mockOLU(t, map[string]http.HandlerFunc{
		"POST /api/v1/oql/query": func(w http.ResponseWriter, r *http.Request) { oqlEmpty(w) },
		"PATCH /api/v1/prop_definitions/": func(w http.ResponseWriter, r *http.Request) {
			capturedBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	htmxPost(t, h.PropDefUpdate, "/properties/1",
		url.Values{"nombre_corto": {"Temperatura Max"}, "tipo_dato": {"numeric"}, "valor_defecto": {"100"}})

	if len(capturedBody) == 0 {
		t.Fatal("xolu PATCH was never called")
	}
	var sent map[string]any
	json.Unmarshal(capturedBody, &sent)
	if sent["short_name"] != "Temperatura Max" {
		t.Errorf("short_name: got %v, want Temperatura Max", sent["short_name"])
	}
	if sent["default_value"] != "100" {
		t.Errorf("default_value: got %v, want 100", sent["default_value"])
	}
	// data_type's own VALUE must still translate English->Spanish for
	// storage (numeric->numerico) even though its KEY no longer needs
	// translating -- these are two separate translations sharing one
	// bridge map, and emptying the map to silence the key translation
	// silently silenced the value translation too (found live: a real
	// created record stored "numeric" instead of "numerico" after the
	// bridge was wrongly emptied to nothing rather than an explicit
	// identity map). This assertion is what would have caught it here
	// instead of only in live verification.
	if sent["data_type"] != "numerico" {
		t.Errorf("data_type: got %v, want numerico (translated, not passed through as English)", sent["data_type"])
	}
}

func TestRuleUpdate_UsesPatch(t *testing.T) {
	patched := false
	server := mockOLU(t, map[string]http.HandlerFunc{
		"PATCH /api/v1/rules/": func(w http.ResponseWriter, r *http.Request) {
			patched = true
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"message": "updated"})
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.RuleUpdate, "/rules/1", url.Values{
		"name":         {"Alerta Temperatura"},
		"trigger_type": {"threshold"},
		"action_type":  {"notify"},
	})

	assertRedirectsTo(t, rec, "/rules")
	if !patched {
		t.Error("xolu.Patch was not called -- ruleData omits created_at, which Update (PUT) would silently wipe on every edit")
	}
}

// TestAssetTypeUpdate_UsesPatch guards a real instance of the same bug:
// the handler's data map is missing 2 of a real asset_type's 9 fields
// (slug and fsm_id), confirmed directly against a real stored
// asset_type record -- Update would have wiped both on every edit.
func TestAssetTypeUpdate_UsesPatch(t *testing.T) {
	patched := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "PATCH":
			patched = true
			json.NewEncoder(w).Encode(map[string]any{"message": "updated"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.AssetTypeUpdate, "/asset-types/1", url.Values{
		"name":               {"Maquinaria Pesada"},
		"description":        {"Heavy machinery"},
		"color":              {"#f59e0b"},
		"connectivity_tiers": {"gps"},
		"default_alerta_id":  {"1"},
	})

	assertRedirectsTo(t, rec, "/asset-types/1")
	if !patched {
		t.Error("xolu.Patch was not called")
	}
}

// TestAlertaMasterUpdate_UsesPatch closes a real, previously-unfilled
// gap found 2026-08-04 while auditing whether all 17 of this session's
// Update-to-Patch fixes actually have adversarial test coverage: this
// entity had none at all, not even the incidental protection a shared
// mock helper gives several of its siblings (brands/cost_centers/
// custodians, via mockOLUForEntity in htmx_partial_test.go) -- if
// AlertaMasterUpdate had regressed back to Update (PUT), nothing would
// have caught it.
func TestAlertaMasterUpdate_UsesPatch(t *testing.T) {
	patched := false
	server := mockOLU(t, map[string]http.HandlerFunc{
		"PATCH /api/v1/alert_types/": func(w http.ResponseWriter, r *http.Request) {
			patched = true
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"message": "updated"})
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.AlertaMasterUpdate, "/alert-types/1", url.Values{
		"type":        {"Maintenance"},
		"description": {"Scheduled maintenance alert"},
		"criticality": {"High"},
	})

	assertRedirectsTo(t, rec, "/alert-types")
	if !patched {
		t.Error("xolu.Patch was not called")
	}
}
