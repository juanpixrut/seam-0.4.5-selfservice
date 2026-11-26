// Package workflowtests contains end-to-end workflow tests for seam-ui.
//
// These tests spin up a real embedded xolu server (SQLite, in-process) and
// exercise seam-ui UIHandler methods in multi-step sequences that verify
// referential integrity, state transitions, and cross-handler data flow.
//
// Run with:
//
//	cd internal/ui/workflow && go test ./... -v
//
// xolu is a test-only dependency of this module. It is NOT imported by the
// main seam-ui module (go.mod at the repo root).
package workflowtests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	uipkg "github.com/ha1tch/seam-ui/internal/ui"
	"github.com/ha1tch/xolu/pkg/cache"
	xolu "github.com/ha1tch/xolu/pkg/client"
	"github.com/ha1tch/xolu/pkg/config"
	"github.com/ha1tch/xolu/pkg/graph"
	"github.com/ha1tch/xolu/pkg/server"
	"github.com/ha1tch/xolu/pkg/storage"
	"github.com/rs/zerolog"
)

// ── noopValidator ─────────────────────────────────────────────────────────────
// xolu's Validator interface requires schema methods we don't need for tests.

type noopValidator struct{}

func (noopValidator) Validate(entity string, data map[string]interface{}) (bool, []string) {
	return true, nil
}
func (noopValidator) LoadSchema(entity string, schemaData map[string]interface{}) error { return nil }
func (noopValidator) SaveSchema(entity string, schemaData map[string]interface{}) error { return nil }
func (noopValidator) HasSchema(entity string) bool                                      { return false }
func (noopValidator) GetSchema(entity string) (map[string]interface{}, error) {
	return nil, nil
}

// ── workflowEnv ───────────────────────────────────────────────────────────────

type workflowEnv struct {
	xoluServer *httptest.Server
	handler    *uipkg.UIHandler
	t          *testing.T
}

// newWorkflowEnv spins up a real embedded xolu server with SQLite storage
// and returns a configured seam-ui UIHandler pointing at it.
func newWorkflowEnv(t *testing.T) *workflowEnv {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "seam-workflow-test.db")

	// Create SQLite store
	store, err := storage.NewStoreFromConfig(storage.StoreConfig{
		Type:   "sqlite",
		DBPath: dbPath,
	})
	if err != nil {
		t.Fatalf("failed to create SQLite store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	// Minimal config — no auth, no rate limiting, path tenant mode
	cfg := config.Default()
	cfg.StorageType = "sqlite"
	cfg.DBPath = dbPath
	cfg.GraphEnabled = false // not needed for master table workflows
	cfg.AuthType = "none"
	cfg.TenantMode = "path"
	cfg.TenantAutoRegister = true
	cfg.MaxEntitySize = 1 << 20
	cfg.DefaultPageSize = 100
	cfg.MaxEmbedDepth = 0
	cfg.RefEmbedDepth = 0
	cfg.QueryTimeout = 10
	cfg.RateLimitEnabled = false
	cfg.MetricsEnabled = false

	memCache := cache.NewMemoryCache(1000, 5*time.Minute)
	t.Cleanup(func() { memCache.Close() })

	g := graph.NewFlatGraph()
	logger := zerolog.New(os.Stdout).Level(zerolog.Disabled)

	srv := server.New(cfg, store, memCache, g, nil, noopValidator{}, logger)
	xoluSrv := httptest.NewServer(srv.Handler())
	t.Cleanup(xoluSrv.Close)

	// seam-ui UIHandler pointing at the embedded xolu
	client := xolu.New(xoluSrv.URL)
	uiCfg := uipkg.Config{AppName: "WorkflowTest", Version: "0.0.0"}
	h := uipkg.NewUIHandler(client, uiCfg, nil, nil, nil, nil, nil, nil, nil)

	return &workflowEnv{
		xoluServer: xoluSrv,
		handler:    h,
		t:          t,
	}
}

// ── Request helpers ───────────────────────────────────────────────────────────

func (e *workflowEnv) POST(handler http.HandlerFunc, target string, fields url.Values) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequest("POST", target, strings.NewReader(fields.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func (e *workflowEnv) GET(handler http.HandlerFunc, target string) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequest("GET", target, nil)
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func (e *workflowEnv) assertRedirect(rec *httptest.ResponseRecorder, location string) {
	e.t.Helper()
	if rec.Code != http.StatusSeeOther {
		e.t.Errorf("expected 303 SeeOther, got %d\nbody: %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != location {
		e.t.Errorf("expected redirect to %q, got %q", location, loc)
	}
}

func (e *workflowEnv) assertStatus(rec *httptest.ResponseRecorder, code int) {
	e.t.Helper()
	if rec.Code != code {
		e.t.Errorf("expected HTTP %d, got %d\nbody: %s", code, rec.Code, rec.Body.String())
	}
}

// lastCreatedID queries xolu directly to find the highest ID for an entity.
func (e *workflowEnv) lastCreatedID(entity string) int64 {
	e.t.Helper()
	body, _ := json.Marshal(map[string]string{
		"query": fmt.Sprintf("SELECT TOP 1 id FROM %s ORDER BY id DESC", entity),
	})
	resp, err := http.Post(
		e.xoluServer.URL+"/api/v1/oql/query",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil || resp.StatusCode != http.StatusOK {
		e.t.Fatalf("lastCreatedID OQL failed for %s: %v status=%d", entity, err, resp.StatusCode)
	}
	defer resp.Body.Close()
	var result struct {
		Data []map[string]interface{} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if len(result.Data) == 0 {
		return 0
	}
	switch v := result.Data[0]["id"].(type) {
	case float64:
		return int64(v)
	}
	return 0
}

// createDirect creates an entity in xolu directly (bypassing the UI handler).
// Used for setup: e.g. creating an asset for alert tests.
func (e *workflowEnv) createDirect(entity string, data map[string]interface{}) int64 {
	e.t.Helper()
	body, _ := json.Marshal(data)
	resp, err := http.Post(
		fmt.Sprintf("%s/api/v1/%s", e.xoluServer.URL, entity),
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil || resp.StatusCode != http.StatusCreated {
		e.t.Fatalf("createDirect %s failed: status=%d err=%v", entity, resp.StatusCode, err)
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if id, ok := result["id"].(float64); ok {
		return int64(id)
	}
	return 0
}

// ── Workflow 1: Región → Ubicación → Delete guard ────────────────────────────

func TestWorkflow_RegionInUseCannotBeDeleted(t *testing.T) {
	e := newWorkflowEnv(t)

	// Step 1: create a región via the UI handler
	rec := e.POST(e.handler.RegionCreate, "/regions",
		url.Values{"descripcion": {"Región Sur"}})
	e.assertRedirect(rec, "/regions")
	regionID := e.lastCreatedID("regions")
	if regionID == 0 {
		t.Fatal("region was not persisted")
	}

	// Step 2: create an ubicación referencing that región
	rec = e.POST(e.handler.UbicacionCreate, "/locations",
		url.Values{
			"nombre_corto": {"Depósito Sur"},
			"descripcion":  {"Depósito principal región sur"},
			"region_id":    {fmt.Sprintf("%d", regionID)},
		})
	e.assertRedirect(rec, "/locations")

	// Step 3: attempt to delete the región — must fail with 409
	req := httptest.NewRequest("POST",
		fmt.Sprintf("/regions/%d/delete", regionID), nil)
	rec2 := httptest.NewRecorder()
	e.handler.RegionDelete(rec2, req)
	e.assertStatus(rec2, http.StatusConflict)

	// Step 4: delete the ubicación first
	ubID := e.lastCreatedID("locations")
	req = httptest.NewRequest("POST",
		fmt.Sprintf("/locations/%d/delete", ubID), nil)
	rec3 := httptest.NewRecorder()
	e.handler.UbicacionDelete(rec3, req)
	e.assertRedirect(rec3, "/locations")

	// Step 5: now delete the región — must succeed
	req = httptest.NewRequest("POST",
		fmt.Sprintf("/regions/%d/delete", regionID), nil)
	rec4 := httptest.NewRecorder()
	e.handler.RegionDelete(rec4, req)
	e.assertRedirect(rec4, "/regions")
}

// ── Workflow 2: Marca → Modelo → Delete guard ─────────────────────────────────

func TestWorkflow_MarcaInUseCannotBeDeleted(t *testing.T) {
	e := newWorkflowEnv(t)

	// Create marca
	rec := e.POST(e.handler.MarcaCreate, "/brands",
		url.Values{"codigo": {"ACME"}, "nombre": {"Acme Corp"}})
	e.assertRedirect(rec, "/brands")
	marcaID := e.lastCreatedID("brands")

	// Create modelo referencing marca
	rec = e.POST(e.handler.ModeloCreate, "/models",
		url.Values{
			"marca_id": {fmt.Sprintf("%d", marcaID)},
			"nombre":   {"Modelo X1"},
		})
	e.assertRedirect(rec, "/models")

	// Delete marca → 409
	req := httptest.NewRequest("POST",
		fmt.Sprintf("/brands/%d/delete", marcaID), nil)
	rec2 := httptest.NewRecorder()
	e.handler.MarcaDelete(rec2, req)
	e.assertStatus(rec2, http.StatusConflict)

	// Delete modelo first
	modeloID := e.lastCreatedID("models")
	req = httptest.NewRequest("POST",
		fmt.Sprintf("/models/%d/delete", modeloID), nil)
	rec3 := httptest.NewRecorder()
	e.handler.ModeloDelete(rec3, req)
	e.assertRedirect(rec3, "/models")

	// Now marca deletion succeeds
	req = httptest.NewRequest("POST",
		fmt.Sprintf("/brands/%d/delete", marcaID), nil)
	rec4 := httptest.NewRecorder()
	e.handler.MarcaDelete(rec4, req)
	e.assertRedirect(rec4, "/brands")
}

// ── Workflow 3: PropDef → PropAssoc → Delete guard ────────────────────────────

func TestWorkflow_PropDefInUseCannotBeDeleted(t *testing.T) {
	e := newWorkflowEnv(t)

	// Create prop_definition
	rec := e.POST(e.handler.PropDefCreate, "/properties",
		url.Values{
			"nombre_corto":  {"numero_serie_ext"},
			"tipo_dato":     {"alfanumerico"},
			"valor_defecto": {""},
		})
	e.assertRedirect(rec, "/properties")
	propID := e.lastCreatedID("prop_definitions")

	// Create prop_association referencing it
	rec = e.POST(e.handler.PropAssocCreate, "/properties/associations",
		url.Values{
			"prop_definition_id": {fmt.Sprintf("%d", propID)},
			"entidad_destino":    {"asset_type"},
			"entidad_id":         {"1"},
			"obligatoriedad":     {"opcional"},
		})
	e.assertRedirect(rec, "/properties/associations")

	// Delete prop_def → 409
	req := httptest.NewRequest("POST",
		fmt.Sprintf("/properties/%d/delete", propID), nil)
	rec2 := httptest.NewRecorder()
	e.handler.PropDefDelete(rec2, req)
	e.assertStatus(rec2, http.StatusConflict)

	// Delete the association first
	assocID := e.lastCreatedID("prop_associations")
	req = httptest.NewRequest("POST",
		fmt.Sprintf("/properties/associations/%d/delete", assocID), nil)
	rec3 := httptest.NewRecorder()
	e.handler.PropAssocDelete(rec3, req)
	e.assertRedirect(rec3, "/properties/associations")

	// Now prop_def deletion succeeds
	req = httptest.NewRequest("POST",
		fmt.Sprintf("/properties/%d/delete", propID), nil)
	rec4 := httptest.NewRecorder()
	e.handler.PropDefDelete(rec4, req)
	e.assertRedirect(rec4, "/properties")
}

// ── Workflow 4: Asset with expiry date appears on dashboard ───────────────────

func TestWorkflow_AssetExpiryAppearsOnDashboard(t *testing.T) {
	e := newWorkflowEnv(t)

	// Create an asset with expiration_date 10 days from now
	expiry := time.Now().AddDate(0, 0, 10).Format("2006-01-02")
	_ = e.createDirect("assets", map[string]interface{}{
		"name":            "Activo de prueba",
		"status":          "active",
		"criticality":     "Alta",
		"expiration_date": expiry,
		"fin_garantia":    "",
	})

	// Fetch the dashboard
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	e.handler.Dashboard(rec, req)
	e.assertStatus(rec, http.StatusOK)

	// Verify the response contains the asset name — the dashboard renders
	// the vencimientos panel with the asset name as a link
	body := rec.Body.String()
	if !strings.Contains(body, "Activo de prueba") {
		t.Error("expected dashboard body to contain the expiring asset name")
	}
}

// ── Workflow 5: Alert acknowledge removes it from open count ──────────────────

func TestWorkflow_AlertAcknowledgeReducesOpenCount(t *testing.T) {
	e := newWorkflowEnv(t)

	// Create an open alert directly in xolu
	alertID := e.createDirect("alerts", map[string]interface{}{
		"title":    "Temperatura alta",
		"message":  "Sensor excedió umbral",
		"severity": "high",
		"status":   "open",
		"source":   "rule",
	})

	// Confirm it shows up in dashboard open alerts
	ctx := context.Background()
	client := xolu.New(e.xoluServer.URL)
	result, err := client.OQL(ctx, "SELECT COUNT(*) as c FROM alerts WHERE status = 'open'")
	if err != nil || len(result.Data) == 0 {
		t.Fatal("failed to count open alerts")
	}
	before := int64(result.Data[0]["c"].(float64))
	if before == 0 {
		t.Fatal("expected at least 1 open alert before acknowledge")
	}

	// Acknowledge via the UI handler
	form := url.Values{"notes": {"Revisado"}}
	req := httptest.NewRequest("POST",
		fmt.Sprintf("/alerts/%d/acknowledge", alertID),
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	e.handler.AlertAcknowledge(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 after acknowledge, got %d: %s", rec.Code, rec.Body.String())
	}

	// Count open alerts after — must have decreased
	result, err = client.OQL(ctx, "SELECT COUNT(*) as c FROM alerts WHERE status = 'open'")
	if err != nil || len(result.Data) == 0 {
		t.Fatal("failed to count open alerts after acknowledge")
	}
	after := int64(result.Data[0]["c"].(float64))
	if after >= before {
		t.Errorf("expected open alert count to decrease after acknowledge: before=%d after=%d",
			before, after)
	}
}

// ── Workflow 6: Custodio inactive excluded from asset dropdown ────────────────

func TestWorkflow_InactiveCustodioExcludedFromDropdown(t *testing.T) {
	e := newWorkflowEnv(t)

	// Create an active custodio
	activeID := e.createDirect("custodians", map[string]interface{}{
		"subject_type": "DNI",
		"legal_name":   "Empresa Activa SA",
		"tax_id":       "12345678",
		"email":        "activa@example.com",
		"active":       true,
	})

	// Create an inactive custodio
	_ = e.createDirect("custodians", map[string]interface{}{
		"subject_type": "DNI",
		"legal_name":   "Empresa Inactiva SRL",
		"tax_id":       "87654321",
		"email":        "inactiva@example.com",
		"active":       false,
	})

	// Render the asset new form — it populates custodio dropdown
	req := httptest.NewRequest("GET", "/assets/new", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	e.handler.AssetNew(rec, req)
	e.assertStatus(rec, http.StatusOK)

	body := rec.Body.String()

	// Active custodio must appear
	if !strings.Contains(body, "Empresa Activa SA") {
		t.Errorf("active custodio not found in asset form (id=%d)", activeID)
	}

	// Inactive custodio must NOT appear
	if strings.Contains(body, "Empresa Inactiva SRL") {
		t.Error("inactive custodio appeared in asset form dropdown — should be excluded")
	}
}
