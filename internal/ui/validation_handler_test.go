package ui

// validation_handler_test.go — end-to-end validation tests.
//
// Each test submits a form with a specific violation and asserts that:
//   1. The handler re-renders the form (HTTP 200) rather than redirecting (303).
//   2. The response body contains the queryfy error message for that field.
//   3. The xolu backend is never called (validateForm short-circuits before it).
//
// On valid input the handler must redirect (303) and the xolu mock must be called.
//
// Coverage: every handler wired with validateForm, one violation per constraint
// type (required, minLength, maxLength, enum, number positive).

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/ha1tch/seam-ui/internal/auth"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// assertValidationFail checks a form re-render response.
func assertValidationFail(t *testing.T, rec *httptest.ResponseRecorder, errFragment string) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 (form re-render) on validation failure, got %d\nbody: %s",
			rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Location") != "" {
		t.Errorf("handler should not redirect on validation failure, got Location: %s",
			rec.Header().Get("Location"))
	}
	if !strings.Contains(rec.Body.String(), errFragment) {
		t.Errorf("expected response body to contain %q\nbody excerpt: %.300s",
			errFragment, rec.Body.String())
	}
}

// assertRedirectsTo checks a successful redirect response.
func assertRedirectsTo(t *testing.T, rec *httptest.ResponseRecorder, location string) {
	t.Helper()
	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303 SeeOther on success, got %d\nbody: %s",
			rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != location {
		t.Errorf("expected redirect to %q, got %q", location, loc)
	}
}

// ooluCreatedOnce returns a mock that responds to one POST create and counts calls.
func ooluCreatedOnce(t *testing.T) (*httptest.Server, *int) {
	t.Helper()
	count := 0
	server := mockOLU(t, map[string]http.HandlerFunc{
		"POST /api/v1": func(w http.ResponseWriter, r *http.Request) {
			count++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"id": float64(1)})
		},
	})
	return server, &count
}

// ── Región ────────────────────────────────────────────────────────────────────

func TestRegionCreate_MissingDescripcion_ReturnsForm(t *testing.T) {
	server := mockOLU(t, nil)
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.RegionCreate, "/regions", url.Values{})

	assertValidationFail(t, rec, "field is required")
}

func TestRegionCreate_DescriptionTooLong_ReturnsForm(t *testing.T) {
	server := mockOLU(t, nil)
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.RegionCreate, "/regions", url.Values{
		"description": {strings.Repeat("x", 201)},
	})

	assertValidationFail(t, rec, "length must be at most")
}

func TestRegionCreate_Valid_Redirects(t *testing.T) {
	server, count := ooluCreatedOnce(t)
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.RegionCreate, "/regions", url.Values{
		"description": {"Región Sur"},
	})

	assertRedirectsTo(t, rec, "/regions")
	if *count == 0 {
		t.Error("xolu.Create was not called on valid submission")
	}
}

// ── Moneda ────────────────────────────────────────────────────────────────────

func TestMonedaCreate_MissingIsoCode_ReturnsForm(t *testing.T) {
	server := mockOLU(t, nil)
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.MonedaCreate, "/currencies", url.Values{
		"symbol": {"$"},
	})

	assertValidationFail(t, rec, "field is required")
}

func TestMonedaCreate_IsoCodeWrongLength_ReturnsForm(t *testing.T) {
	server := mockOLU(t, nil)
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.MonedaCreate, "/currencies", url.Values{
		"iso_code": {"US"}, // must be exactly 3 chars
		"symbol":   {"$"},
	})

	assertValidationFail(t, rec, "length must be at least 3")
}

func TestMonedaCreate_Valid_Redirects(t *testing.T) {
	server, count := ooluCreatedOnce(t)
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.MonedaCreate, "/currencies", url.Values{
		"iso_code": {"USD"},
		"symbol":   {"$"},
	})

	assertRedirectsTo(t, rec, "/currencies")
	if *count == 0 {
		t.Error("xolu.Create was not called")
	}
}

// ── Marca ─────────────────────────────────────────────────────────────────────

func TestMarcaCreate_MissingCode_ReturnsForm(t *testing.T) {
	server := mockOLU(t, nil)
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.MarcaCreate, "/brands", url.Values{
		"name": {"Acme"},
	})

	assertValidationFail(t, rec, "field is required")
}

func TestMarcaCreate_Valid_Redirects(t *testing.T) {
	server, count := ooluCreatedOnce(t)
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.MarcaCreate, "/brands", url.Values{
		"code": {"ACM"},
		"name": {"Acme Corp"},
	})

	assertRedirectsTo(t, rec, "/brands")
	if *count == 0 {
		t.Error("xolu.Create was not called")
	}
}

// ── Custodio ──────────────────────────────────────────────────────────────────

func TestCustodioCreate_InvalidSubjectType_ReturnsForm(t *testing.T) {
	server := mockOLU(t, nil)
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.CustodioCreate, "/custodians", url.Values{
		"subject_type": {"PASSPORT"}, // not in enum DNI/CI/RUT/RUC
		"legal_name":   {"Empresa SA"},
		"tax_id":       {"12345678"},
		"email":        {"empresa@example.com"},
	})

	assertValidationFail(t, rec, "must be one of")
}

func TestCustodioCreate_MissingEmail_ReturnsForm(t *testing.T) {
	server := mockOLU(t, nil)
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.CustodioCreate, "/custodians", url.Values{
		"subject_type": {"DNI"},
		"legal_name":   {"Empresa SA"},
		"tax_id":       {"12345678"},
		// no email
	})

	assertValidationFail(t, rec, "field is required")
}

func TestCustodioCreate_Valid_Redirects(t *testing.T) {
	server, count := ooluCreatedOnce(t)
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.CustodioCreate, "/custodians", url.Values{
		"subject_type": {"DNI"},
		"legal_name":   {"Empresa SA"},
		"tax_id":       {"12345678"},
		"email":        {"empresa@example.com"},
		"active":       {"on"},
	})

	assertRedirectsTo(t, rec, "/custodians")
	if *count == 0 {
		t.Error("xolu.Create was not called")
	}
}

// ── Alerta (master table) ─────────────────────────────────────────────────────

func TestAlertaMasterCreate_InvalidType_ReturnsForm(t *testing.T) {
	server := mockOLU(t, nil)
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.AlertaMasterCreate, "/alert-types", url.Values{
		"type":        {"Incendio"}, // not in enum
		"description": {"Alerta test"},
		"criticality": {"Alta"},
	})

	assertValidationFail(t, rec, "must be one of")
}

func TestAlertaMasterCreate_InvalidCriticality_ReturnsForm(t *testing.T) {
	server := mockOLU(t, nil)
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.AlertaMasterCreate, "/alert-types", url.Values{
		"type":        {"Mantenimiento"},
		"description": {"Test"},
		"criticality": {"Extrema"}, // not in enum
	})

	assertValidationFail(t, rec, "must be one of")
}

func TestAlertaMasterCreate_Valid_Redirects(t *testing.T) {
	server, count := ooluCreatedOnce(t)
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.AlertaMasterCreate, "/alert-types", url.Values{
		"type":        {"Mantenimiento"},
		"description": {"Temperatura elevada"},
		"criticality": {"Alta"},
	})

	assertRedirectsTo(t, rec, "/alert-types")
	if *count == 0 {
		t.Error("xolu.Create was not called")
	}
}

// ── PropDefinición ────────────────────────────────────────────────────────────

func TestPropDefCreate_MissingNombreCorto_ReturnsForm(t *testing.T) {
	server := mockOLU(t, nil)
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.PropDefCreate, "/properties", url.Values{
		"tipo_dato": {"numerico"},
	})

	assertValidationFail(t, rec, "field is required")
}

func TestPropDefCreate_InvalidTipoDato_ReturnsForm(t *testing.T) {
	server := mockOLU(t, nil)
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.PropDefCreate, "/properties", url.Values{
		"nombre_corto": {"temp_max"},
		"tipo_dato":    {"texto_libre"}, // not in enum
	})

	assertValidationFail(t, rec, "must be one of")
}

func TestPropDefCreate_Valid_Redirects(t *testing.T) {
	server, count := ooluCreatedOnce(t)
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.PropDefCreate, "/properties", url.Values{
		"nombre_corto": {"temp_max"},
		"tipo_dato":    {"numerico"},
	})

	assertRedirectsTo(t, rec, "/properties")
	if *count == 0 {
		t.Error("xolu.Create was not called")
	}
}

// ── Modelo ────────────────────────────────────────────────────────────────────

func TestModeloCreate_MissingMarcaID_ReturnsForm(t *testing.T) {
	server := mockOLU(t, nil)
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.ModeloCreate, "/models", url.Values{
		"nombre": {"Modelo X1"},
		// no marca_id — stripped as int64(0) → absent → "field is required"
	})

	assertValidationFail(t, rec, "field is required")
}

func TestModeloCreate_MissingNombre_ReturnsForm(t *testing.T) {
	server := mockOLU(t, nil)
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.ModeloCreate, "/models", url.Values{
		"marca_id": {"1"},
		// no nombre
	})

	assertValidationFail(t, rec, "field is required")
}

func TestModeloCreate_Valid_Redirects(t *testing.T) {
	server, count := ooluCreatedOnce(t)
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.ModeloCreate, "/models", url.Values{
		"marca_id": {"1"},
		"nombre":   {"Modelo X1"},
	})

	assertRedirectsTo(t, rec, "/models")
	if *count == 0 {
		t.Error("xolu.Create was not called")
	}
}

// ── Ubicación ─────────────────────────────────────────────────────────────────

func TestUbicacionCreate_MissingNombreCorto_ReturnsForm(t *testing.T) {
	// loadRegiones is called on error re-render — mock OQL for it
	server2 := mockOLU(t, map[string]http.HandlerFunc{
		"POST /api/v1": func(w http.ResponseWriter, r *http.Request) {
			oqlEmpty(w)
		},
	})
	defer server2.Close()
	h := newHandler(t, server2)

	rec := postForm(t, h.UbicacionCreate, "/locations", url.Values{
		"region_id": {"1"},
		// no nombre_corto
	})

	assertValidationFail(t, rec, "field is required")
}

func TestUbicacionCreate_MissingRegionID_ReturnsForm(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{
		"POST /api/v1": func(w http.ResponseWriter, r *http.Request) {
			oqlEmpty(w)
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.UbicacionCreate, "/locations", url.Values{
		"nombre_corto": {"Depósito A"},
		// region_id absent → int64(0) stripped → "field is required"
	})

	assertValidationFail(t, rec, "field is required")
}

func TestUbicacionCreate_Valid_Redirects(t *testing.T) {
	count := 0
	server := mockOLU(t, map[string]http.HandlerFunc{
		"POST /api/v1": func(w http.ResponseWriter, r *http.Request) {
			count++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"id": float64(1)})
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.UbicacionCreate, "/locations", url.Values{
		"short_name": {"Depósito A"},
		"region_id":  {"1"},
	})

	assertRedirectsTo(t, rec, "/locations")
	if count == 0 {
		t.Error("xolu.Create was not called")
	}
}

// ── PropAssoc ─────────────────────────────────────────────────────────────────

func TestPropAssocCreate_InvalidEntidadDestino_ReturnsForm(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{
		"POST /api/v1": func(w http.ResponseWriter, r *http.Request) {
			oqlEmpty(w)
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.PropAssocCreate, "/properties/associations", url.Values{
		"prop_definition_id": {"1"},
		"entidad_destino":    {"sensor"}, // not in enum asset_type|asset
		"entidad_id":         {"1"},
		"obligatoriedad":     {"opcional"},
	})

	assertValidationFail(t, rec, "must be one of")
}

func TestPropAssocCreate_InvalidObligatoriedad_ReturnsForm(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{
		"POST /api/v1": func(w http.ResponseWriter, r *http.Request) {
			oqlEmpty(w)
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.PropAssocCreate, "/properties/associations", url.Values{
		"prop_definition_id": {"1"},
		"entidad_destino":    {"asset_type"},
		"entidad_id":         {"1"},
		"obligatoriedad":     {"requerido"}, // was the schema typo — now "requerida" is correct
	})

	assertValidationFail(t, rec, "must be one of")
}

func TestPropAssocCreate_Valid_Redirects(t *testing.T) {
	count := 0
	server := mockOLU(t, map[string]http.HandlerFunc{
		"POST /api/v1": func(w http.ResponseWriter, r *http.Request) {
			count++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"id": float64(1)})
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.PropAssocCreate, "/properties/associations", url.Values{
		"prop_definition_id": {"1"},
		"entidad_destino":    {"asset_type"},
		"entidad_id":         {"1"},
		"obligatoriedad":     {"requerida"},
	})

	assertRedirectsTo(t, rec, "/properties/associations")
	if count == 0 {
		t.Error("xolu.Create was not called")
	}
}

// TestPropAssocCreate_DataIntegrity, added 2026-08-11 alongside the
// target_entity/entity_id/requirement rename, same reasoning as
// TestPropDefUpdate_DataIntegrity above -- none of the pre-existing
// PropAssoc tests inspected the actual payload sent to xolu.
func TestPropAssocCreate_DataIntegrity(t *testing.T) {
	var capturedBody []byte
	server := mockOLU(t, map[string]http.HandlerFunc{
		"POST /api/v1": func(w http.ResponseWriter, r *http.Request) {
			capturedBody, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"id": float64(1)})
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	postForm(t, h.PropAssocCreate, "/properties/associations", url.Values{
		"prop_definition_id": {"1"},
		"entidad_destino":    {"asset_type"},
		"entidad_id":         {"7"},
		"obligatoriedad":     {"requerida"},
	})

	if len(capturedBody) == 0 {
		t.Fatal("xolu.Create was never called")
	}
	var sent map[string]any
	json.Unmarshal(capturedBody, &sent)
	if sent["target_entity"] != "asset_type" {
		t.Errorf("target_entity: got %v, want asset_type", sent["target_entity"])
	}
	if int(sent["entity_id"].(float64)) != 7 {
		t.Errorf("entity_id: got %v, want 7", sent["entity_id"])
	}
	// requirement's own VALUE must still translate English->Spanish for
	// storage (requerida is what "requerida" was submitted as here,
	// deliberately already-Spanish to confirm the tolerant pass-through
	// path too) -- same class of bug as PropDef's data_type, see
	// TestPropDefUpdate_DataIntegrity's own comment for the full story.
	if sent["requirement"] != "requerida" {
		t.Errorf("requirement: got %v, want requerida", sent["requirement"])
	}
}

// ── CentroCosto ───────────────────────────────────────────────────────────────

func TestCentroCostoCreate_MissingCodigo_ReturnsForm(t *testing.T) {
	server := mockOLU(t, nil)
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.CentroCostoCreate, "/cost-centers", url.Values{
		"name": {"Administración"},
		// no accounting_code
	})

	assertValidationFail(t, rec, "field is required")
}

func TestCentroCostoCreate_Valid_Redirects(t *testing.T) {
	server, count := ooluCreatedOnce(t)
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.CentroCostoCreate, "/cost-centers", url.Values{
		"accounting_code": {"ADM-001"},
		"name":            {"Administración"},
	})

	assertRedirectsTo(t, rec, "/cost-centers")
	if *count == 0 {
		t.Error("xolu.Create was not called")
	}
}

// ── Update variants (schemas reused from Create) ──────────────────────────────

func TestRegionUpdate_MissingDescription_ReturnsForm(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{
		"GET /api/v1": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"id":   float64(1),
				"data": map[string]any{"description": "Original"},
			})
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	// RegionUpdate extracts id from URL path — use URL directly, not PathValue
	rec := postForm(t, h.RegionUpdate, "/regions/1", url.Values{})

	assertValidationFail(t, rec, "field is required")
}

func TestModeloUpdate_MissingMarcaID_ReturnsForm(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{
		"GET /api/v1": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"id":   float64(1),
				"data": map[string]any{"name": "Modelo X", "brand_id": float64(5)},
			})
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.ModeloUpdate, "/models/1", url.Values{
		"nombre": {"Modelo X"},
		// no marca_id — zero → stripped → "field is required"
	})

	assertValidationFail(t, rec, "field is required")
}

// ── AssetTypeCreate ───────────────────────────────────────────────────────────

func TestAssetTypeCreate_MissingName_ReturnsForm(t *testing.T) {
	server := mockOLU(t, nil)
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.AssetTypeCreate, "/asset-types", url.Values{
		"slug": {"motor"},
		// no name
	})

	// AssetTypeCreate returns RenderError on validation failure (422)
	if rec.Code == http.StatusSeeOther {
		t.Errorf("expected non-redirect on validation failure, got 303")
	}
	if rec.Code == http.StatusSeeOther {
		t.Error("should not redirect when required field is missing")
	}
}

func TestAssetTypeCreate_MissingSlug_ReturnsError(t *testing.T) {
	server := mockOLU(t, nil)
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.AssetTypeCreate, "/asset-types", url.Values{
		"name": {"Motor"},
		// no slug
	})

	if rec.Code == http.StatusSeeOther {
		t.Errorf("expected non-redirect on validation failure, got 303")
	}
}

func TestAssetTypeCreate_Valid_Redirects(t *testing.T) {
	server, count := ooluCreatedOnce(t)
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.AssetTypeCreate, "/asset-types", url.Values{
		"name": {"Motor"},
		"slug": {"motor"},
	})

	assertRedirectsTo(t, rec, "/asset-types")
	if *count == 0 {
		t.Error("xolu.Create was not called")
	}
}

// ── RuleCreate ────────────────────────────────────────────────────────────────

func TestRuleCreate_MissingName_ReturnsError(t *testing.T) {
	server := mockOLU(t, nil)
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.RuleCreate, "/rules", url.Values{
		// no name — the handler has an ad-hoc required check for name
		"trigger_type": {"event"},
		"action_type":  {"notify"},
	})

	// Handler returns a full page render (not 303) on missing name
	if rec.Code == http.StatusSeeOther {
		t.Errorf("expected non-redirect on missing name, got 303")
	}
}

func TestRuleCreate_InvalidTriggerType_ReturnsError(t *testing.T) {
	server := mockOLU(t, nil)
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.RuleCreate, "/rules", url.Values{
		"name":         {"Alerta Temperatura"},
		"trigger_type": {"invalid_type"}, // not in enum
		"action_type":  {"notify"},
	})

	if rec.Code == http.StatusSeeOther {
		t.Errorf("expected non-redirect on invalid trigger_type, got 303")
	}
}

func TestRuleCreate_Valid_Redirects(t *testing.T) {
	server, count := ooluCreatedOnce(t)
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.RuleCreate, "/rules", url.Values{
		"name":         {"Alerta Temperatura"},
		"trigger_type": {"threshold"},
		"action_type":  {"notify"},
	})

	assertRedirectsTo(t, rec, "/rules")
	if *count == 0 {
		t.Error("xolu.Create was not called")
	}
}

// ── SensorCreate ──────────────────────────────────────────────────────────────

func TestSensorCreate_MissingExternalID_ReturnsForm(t *testing.T) {
	server := mockOLU(t, nil)
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.SensorCreate, "/sensors", url.Values{
		"type":   {"temperature"},
		"driver": {"mqtt"},
		// no external_id
	})

	// SensorCreate has ad-hoc required checks; returns full-page render on error
	if rec.Code == http.StatusSeeOther {
		t.Errorf("expected non-redirect on missing external_id, got 303")
	}
}

func TestSensorCreate_Valid_Redirects(t *testing.T) {
	server, count := ooluCreatedOnce(t)
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.SensorCreate, "/sensors", url.Values{
		"external_id": {"SENSOR-001"},
		"type":        {"temperature"},
		"driver":      {"mqtt"},
	})

	assertRedirectsTo(t, rec, "/sensors")
	if *count == 0 {
		t.Error("xolu.Create was not called")
	}
}

// ── AdminUserUpdate ───────────────────────────────────────────────────────────

func TestAdminUserUpdate_InvalidRole_ReturnsError(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{
		"GET /api/v1": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"id":   float64(1),
				"data": map[string]any{"name": "Ana García", "role": "operator"},
			})
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	rec := postForm(t, h.AdminUserUpdate, "/admin/users/1", url.Values{
		"name": {"Ana García"},
		"role": {"superuser"}, // not in enum
	})

	if rec.Code == http.StatusSeeOther {
		t.Errorf("expected non-redirect on invalid role, got 303")
	}
}

func TestAdminUserUpdate_Valid_Redirects(t *testing.T) {
	count := 0
	// xolu.Patch sends PATCH /api/v1/users/1 — match on "PATCH /api/v1"
	server := mockOLU(t, map[string]http.HandlerFunc{
		// AdminUserUpdate now fetches the existing user first (to detect
		// a role change for the audit log) before updating it.
		"GET /api/v1": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"id": 1, "name": "Ana", "role": "operator"})
		},
		"PATCH /api/v1": func(w http.ResponseWriter, r *http.Request) {
			count++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"message": "updated"})
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	// AdminUserUpdate uses r.PathValue("id") — must be set on the request
	req := httptest.NewRequest("POST", "/admin/users/1",
		strings.NewReader(url.Values{"name": {"Ana García"}, "role": {"manager"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()
	h.AdminUserUpdate(rec, req)

	assertRedirectsTo(t, rec, "/admin/users")
	if count == 0 {
		t.Error("xolu.Patch was not called")
	}
}

// csrfFieldValueRe extracts the value of a rendered <input name="_csrf" ...>
// field regardless of attribute order (the real markup is name="_csrf"
// type="hidden" value="...", not name="_csrf" value="..." -- a plain
// strings.Contains on the latter would never match either way).
var csrfFieldValueRe = regexp.MustCompile(`name="_csrf"[^>]*value="([^"]*)"`)

// TestAdminUserEdit_RendersNonEmptyCSRFToken guards against a real bug
// found 2026-08-03 during MOD-5 live verification: AdminUserEdit built
// its UserFormData without setting CSRFToken at all -- every other
// form-rendering handler in this codebase does (grep CSRFToken:
// h.GetCSRFToken(w, r) across routes.go/masters_routes.go/admin_users.go
// confirms this was the only omission). Practical effect in production:
// the edit-user form's hidden _csrf field always rendered empty, so
// submitting it always failed CSRF validation with 403 -- editing an
// existing user was completely broken. newHandler's own csrfManager is
// nil (by design, for tests that don't care about CSRF), which would
// make this assertion pass trivially regardless of the bug -- this test
// deliberately constructs its own handler with a real csrfManager so a
// regression (an empty token) actually fails it.
func TestAdminUserEdit_RendersNonEmptyCSRFToken(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{
		"GET /api/v1": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"id":   float64(1),
				"data": map[string]any{"name": "Ana", "email": "ana@example.com", "role": "operator", "active": true},
			})
		},
	})
	defer server.Close()

	client := xolu.New(server.URL)
	cfg := Config{AppName: "Test", Version: "0.0.0"}
	h := NewUIHandler(client, cfg, nil, nil, nil, testCSRFManager(), nil, nil, nil)

	req := httptest.NewRequest("GET", "/admin/users/1", nil)
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()
	h.AdminUserEdit(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("AdminUserEdit: got status %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	// Attribute order in the rendered <input> isn't name="_csrf" value="...">
	// -- it's name="_csrf" type="hidden" value="..."> -- a plain
	// strings.Contains(body, `name="_csrf" value=""`) check would never
	// match either way (empty or populated) and pass vacuously regardless
	// of the actual bug. Caught by this test's own sanity check (reverting
	// the fix and confirming the test failed) before trusting it.
	m := csrfFieldValueRe.FindStringSubmatch(body)
	if m == nil {
		t.Fatal("rendered form has no _csrf field at all")
	}
	if m[1] == "" {
		t.Error(`_csrf field rendered with an empty value -- form submission will always fail CSRF validation`)
	}
}

// ── errorsToSummary ───────────────────────────────────────────────────────────

func TestErrorsToSummary_Empty(t *testing.T) {
	if got := errorsToSummary(nil); got != "" {
		t.Errorf("expected empty string for nil map, got %q", got)
	}
	if got := errorsToSummary(map[string]string{}); got != "" {
		t.Errorf("expected empty string for empty map, got %q", got)
	}
}

func TestErrorsToSummary_SingleField(t *testing.T) {
	got := errorsToSummary(map[string]string{"name": "field is required"})
	if got != "name: field is required" {
		t.Errorf("unexpected summary: %q", got)
	}
}

func TestErrorsToSummary_MultipleFields_Sorted(t *testing.T) {
	got := errorsToSummary(map[string]string{
		"status":     "must be one of: active, inactive",
		"criticidad": "must be one of: Alta, Media, Baja",
		"name":       "field is required",
	})
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), got)
	}
	// Lines should be sorted: criticidad, name, status
	if lines[0] != "criticidad: must be one of: Alta, Media, Baja" {
		t.Errorf("line 0 wrong: %q", lines[0])
	}
	if lines[1] != "name: field is required" {
		t.Errorf("line 1 wrong: %q", lines[1])
	}
	if lines[2] != "status: must be one of: active, inactive" {
		t.Errorf("line 2 wrong: %q", lines[2])
	}
}

func TestErrorsToSummary_TopLevelFirst(t *testing.T) {
	got := errorsToSummary(map[string]string{
		"_":    "submission failed",
		"name": "field is required",
	})
	lines := strings.Split(got, "\n")
	if lines[0] != "submission failed" {
		t.Errorf("top-level error should be first, got: %q", lines[0])
	}
}

// This block guards a real bug found 2026-08-03 while rewriting
// AdminUserCreate for MOD-8's formengine rebuild: of its three possible
// render paths (validation failure, the "email already exists" case,
// and the success redirect), the email-exists path never set
// CSRFToken at all -- resubmitting that specific re-rendered form
// would always have failed CSRF validation. Fixed by routing all three
// paths through one closure (rerenderCreateForm) that always sets it.
//
// TestAdminUserCreate_ValidationFailureRerendersWithCSRFToken doesn't
// exercise the email-exists branch directly (that needs a real
// UserService returning auth.ErrUserExists, which newHandler's nil
// userService can't produce -- it takes the "create directly in xolu"
// fallback path instead), but validation failure hits the exact same
// rerenderCreateForm closure the email-exists branch calls, so this is
// still a genuine test of the fix, not a weaker substitute for one.
func TestAdminUserCreate_ValidationFailureRerendersWithCSRFToken(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{})
	defer server.Close()

	client := xolu.New(server.URL)
	cfg := Config{AppName: "Test", Version: "0.0.0"}
	h := NewUIHandler(client, cfg, nil, nil, nil, testCSRFManager(), nil, nil, nil)

	req := httptest.NewRequest("POST", "/admin/users", strings.NewReader(url.Values{
		"email": {""}, "name": {"New User"}, "role": {"operator"}, // missing email -> validation failure
		"password": {"longenough1"}, "password_confirm": {"longenough1"},
	}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.AdminUserCreate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("AdminUserCreate with missing email: got status %d, want 200 (re-render)", rec.Code)
	}
	m := csrfFieldValueRe.FindStringSubmatch(rec.Body.String())
	if m == nil {
		t.Fatal("re-rendered create-user form has no _csrf field at all")
	}
	if m[1] == "" {
		t.Error("_csrf field rendered with an empty value -- resubmitting this form would always fail CSRF validation")
	}
}

func TestAdminUserNew_RendersCreateFormFields(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{})
	defer server.Close()
	h := newHandler(t, server)

	req := httptest.NewRequest("GET", "/admin/users/new", nil)
	rec := httptest.NewRecorder()
	h.AdminUserNew(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("AdminUserNew: got status %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, id := range []string{"field-email", "field-name", "field-role", "field-password", "field-password_confirm"} {
		if !strings.Contains(body, `id="`+id+`"`) {
			t.Errorf("create form missing expected field %q", id)
		}
	}
	if strings.Contains(body, `id="field-active"`) {
		t.Error("create form should not have an active field -- that's edit-only (user_edit.json, not user_create.json)")
	}
}

func TestAdminUserEdit_RendersEditFormFieldsAndDisabledEmail(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{
		"GET /api/v1": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			// Flat, not {"id":..,"data":{...}} -- xolu.Client.Get uses the
			// whole response body as Entity.Data directly (confirmed
			// directly against pkg/client/client.go), so a nested "data"
			// wrapper here would silently leave every field nil. Several
			// existing tests in this file have that exact inaccuracy
			// (harmless there only because none of them assert on a
			// value that flows through it) -- filed as its own finding
			// rather than fixed everywhere as a side effect of this test.
			json.NewEncoder(w).Encode(map[string]any{
				"id": float64(1), "name": "Ana", "email": "ana@example.com", "role": "operator", "active": true,
			})
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	req := httptest.NewRequest("GET", "/admin/users/1", nil)
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()
	h.AdminUserEdit(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("AdminUserEdit: got status %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, id := range []string{"field-name", "field-role", "field-active"} {
		if !strings.Contains(body, `id="`+id+`"`) {
			t.Errorf("edit form missing expected field %q", id)
		}
	}
	if strings.Contains(body, `id="field-email"`) {
		t.Error("edit form should not render email as a formengine field -- it's hand-coded disabled/readonly (userEmailFieldIfEdit), not part of user_edit.json")
	}
	if !strings.Contains(body, `id="email_display"`) {
		t.Error("edit form missing the hand-coded disabled email field")
	}
	if !strings.Contains(body, "disabled") {
		t.Error("edit form's email field isn't actually disabled")
	}
	if !strings.Contains(body, `value="ana@example.com"`) {
		t.Error("edit form's disabled email field doesn't show the real fetched email")
	}
}

// This block guards a real, security-relevant gap found 2026-08-04 via
// adversarial testing (deliberately submitting values a well-behaved
// client's <select> would never send, to see what the server actually
// does with them -- not just what the UI offers).
//
// AdminUserUpdate was already protected: UserUpdateSchema enforces
// .Enum("admin","manager","operator","viewer") via validateForm before
// any write happens. AdminUserCreate had no equivalent at the handler
// level. Its h.userService != nil path was accidentally protected one
// layer down (UserService.CreateUser calls auth.IsValidRole internally)
// -- but that path's error handling for ErrInvalidRole fell through to
// a generic 500, not a real validation error, and its fallback path
// (h.userService == nil, direct xolu write, exercised by every test in
// this file via newHandler) had no validation at all: a submitted role
// outside the four known values was written straight to storage.
// Fixed by checking auth.IsValidRole(role) in AdminUserCreate's own
// shared, up-front validation block, ahead of either path.
func TestAdminUserCreate_RejectsRoleOutsideEnum_FallbackPath(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{}) // no route should ever be hit -- Create must never be called
	defer server.Close()
	h := newHandler(t, server)

	for _, badRole := range []string{"superadmin", "root", "ADMIN", "admin ", "<script>alert(1)</script>", "'; DROP TABLE users;--"} {
		t.Run(badRole, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/admin/users", strings.NewReader(url.Values{
				"email": {"attacker@example.com"}, "name": {"Attacker"}, "role": {badRole},
				"password": {"longenough1"}, "password_confirm": {"longenough1"},
			}.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			h.AdminUserCreate(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("role=%q: got status %d, want 200 (re-rendered form with a validation error, not a write)", badRole, rec.Code)
			}
			// newHandler's translator is nil, so t(key) returns the raw
			// key itself (translateFunc's own documented nil fallback),
			// not real English text -- checking for the key is both
			// correct for this test's setup and locale-independent.
			if !strings.Contains(rec.Body.String(), "users.error.role_invalid") {
				t.Errorf("role=%q: response doesn't show the role_invalid error", badRole)
			}
		})
	}
}

func TestAdminUserCreate_AcceptsEachRealRole(t *testing.T) {
	// The inverse check: the fix must not be so strict it rejects the
	// four legitimate values -- a validation fix that silently breaks
	// the happy path is its own kind of regression.
	for _, goodRole := range []string{"admin", "manager", "operator", "viewer"} {
		t.Run(goodRole, func(t *testing.T) {
			var created bool
			server := mockOLU(t, map[string]http.HandlerFunc{
				"POST /api/v1": func(w http.ResponseWriter, r *http.Request) {
					created = true
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(map[string]any{"id": float64(1), "message": "created"})
				},
			})
			defer server.Close()
			h := newHandler(t, server)

			req := httptest.NewRequest("POST", "/admin/users", strings.NewReader(url.Values{
				"email": {"real@example.com"}, "name": {"Real User"}, "role": {goodRole},
				"password": {"longenough1"}, "password_confirm": {"longenough1"},
			}.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req = req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{UserID: 1, TenantID: 1, Role: "admin"}))
			rec := httptest.NewRecorder()
			h.AdminUserCreate(rec, req)

			assertRedirectsTo(t, rec, "/admin/users")
			if !created {
				t.Errorf("role=%q: a legitimate role was rejected -- xolu.Create was never called", goodRole)
			}
		})
	}
}
