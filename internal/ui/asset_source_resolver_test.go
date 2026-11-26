package ui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	xolu "github.com/ha1tch/xolu/pkg/client"
)

// newTestOQLServer returns an httptest.Server simulating xolu's
// POST /oql/query endpoint: it echoes back rows from a rows-by-query
// map, keyed by the exact OQL string submitted. Faithful to the real
// request/response shape confirmed by reading pkg/client's OQL method
// directly, not guessed at.
func newTestOQLServer(t *testing.T, rowsByQuery map[string][]map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The real path is /api/v1/oql/query -- pkg/client's do() method
		// prepends /api/v1 to whatever path OQL passes it. Confirmed by
		// the first test run against this server failing with the raw
		// /oql/query assumption, not read from source beforehand.
		if r.URL.Path != "/api/v1/oql/query" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		var body struct {
			Query string `json:"query"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		rows, ok := rowsByQuery[body.Query]
		if !ok {
			t.Errorf("unexpected query: %q", body.Query)
			rows = nil
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"data":   rows,
			"stats":  map[string]any{},
		})
	}))
}

// newTestOQLErrorServer returns an httptest.Server that responds to
// every OQL query with the given xolu-style structured error, in the
// exact wire shape pkg/client's error decoder expects
// ({"error":{"code":...,"message":...,"status":...}}) -- confirmed
// against client.go's own decoder directly, not guessed at.
func newTestOQLErrorServer(t *testing.T, httpStatus int, code, message string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpStatus)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"code":    code,
				"message": message,
				"status":  httpStatus,
			},
		})
	}))
}

func TestAssetSourceResolver_EntityNotYetCreated_ReturnsEmptyNotError(t *testing.T) {
	// The actual bug this exists to fix (T-28 evidence work, confirmed
	// live against a genuinely fresh xolu tenant): every FK-sourced
	// field failed outright on a brand-new deployment before any
	// master data existed, including type_id, a required field --
	// making the form genuinely unsubmittable in that state.
	srv := newTestOQLErrorServer(t, 400, "XOLU-QL004", "entity 'marcas' does not exist")
	defer srv.Close()
	r := &AssetSourceResolver{Client: xolu.New(srv.URL)}

	got, err := r.Resolve(context.Background(), "entity:brands")
	if err != nil {
		t.Fatalf("Resolve: expected nil error for a not-yet-created entity, got %v", err)
	}
	if got == nil {
		t.Error("expected an empty slice, got nil")
	}
	if len(got) != 0 {
		t.Errorf("expected 0 options, got %d", len(got))
	}
}

func TestAssetSourceResolver_OtherQL004Error_StillPropagates(t *testing.T) {
	// XOLU-QL004 (ErrQueryFailed) is a general "query failed" bucket in
	// xolu (pkg/errors/errors.go), not exclusively "entity doesn't
	// exist" -- the fix must not treat every QL004 as safe to swallow,
	// only the specific "does not exist" message. A same-code error
	// with a different message (a stand-in for "some other reason this
	// query failed") must still surface as a real error.
	srv := newTestOQLErrorServer(t, 400, "XOLU-QL004", "query execution failed: syntax error near ORDER")
	defer srv.Close()
	r := &AssetSourceResolver{Client: xolu.New(srv.URL)}

	_, err := r.Resolve(context.Background(), "entity:brands")
	if err == nil {
		t.Fatal("expected an error to propagate for a QL004 failure that isn't \"entity does not exist\", got nil")
	}
}

func TestAssetSourceResolver_UnrelatedError_StillPropagates(t *testing.T) {
	// A completely different code/status (a stand-in for a genuine
	// server-side failure, e.g. XOLU-ST006 storage failed) must never
	// be swallowed -- only the one specific, narrow case is safe to
	// treat as "no data yet".
	srv := newTestOQLErrorServer(t, 500, "XOLU-ST006", "storage failed")
	defer srv.Close()
	r := &AssetSourceResolver{Client: xolu.New(srv.URL)}

	_, err := r.Resolve(context.Background(), "entity:brands")
	if err == nil {
		t.Fatal("expected a genuine server error to propagate, got nil")
	}
}

func TestAssetSourceResolver_Marcas(t *testing.T) {
	srv := newTestOQLServer(t, map[string][]map[string]any{
		"SELECT * FROM brands ORDER BY name": {
			{"id": float64(1), "code": "ACM", "name": "Acme"},
			{"id": float64(2), "code": "GLB", "name": "Globex"},
		},
	})
	defer srv.Close()

	r := &AssetSourceResolver{Client: xolu.New(srv.URL)}
	got, err := r.Resolve(context.Background(), "entity:brands")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := []struct{ Value, Label string }{
		{"1", "ACM — Acme"},
		{"2", "GLB — Globex"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d options, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Value != w.Value || got[i].Label != w.Label {
			t.Errorf("option %d = {%q, %q}, want {%q, %q}", i, got[i].Value, got[i].Label, w.Value, w.Label)
		}
	}
}

func TestAssetSourceResolver_AllNineSourcesQueryCorrectly(t *testing.T) {
	// One row per source, enough to confirm each source hits the exact
	// query string real code uses and produces a sane label -- not
	// re-testing every label format in detail (Marcas above already
	// does that), just confirming none of the nine are wired wrong.
	rows := map[string][]map[string]any{
		"SELECT * FROM brands ORDER BY name":                                                          {{"id": float64(1), "code": "A", "name": "B"}},
		"SELECT id, brand_id, name FROM models ORDER BY name":                                         {{"id": float64(1), "name": "X100"}},
		"SELECT * FROM regions ORDER BY description":                                                  {{"id": float64(1), "description": "Montevideo"}},
		"SELECT id, short_name, region_id FROM locations ORDER BY short_name":                         {{"id": float64(1), "short_name": "Planta A"}},
		"SELECT id, legal_name, subject_type FROM custodians WHERE active = true ORDER BY legal_name": {{"id": float64(1), "legal_name": "Juan Perez"}},
		"SELECT id, iso_code, symbol FROM currencies ORDER BY iso_code":                               {{"id": float64(1), "iso_code": "USD", "symbol": "$"}},
		"SELECT id, accounting_code, name FROM cost_centers ORDER BY name":                            {{"id": float64(1), "accounting_code": "CC-01", "name": "Planta"}},
		"SELECT * FROM alert_types ORDER BY type, criticality":                                        {{"id": float64(1), "type": "Temp", "criticality": "Alta"}},
		"SELECT id, name FROM asset_types ORDER BY name":                                              {{"id": float64(1), "name": "Compresor"}},
	}
	srv := newTestOQLServer(t, rows)
	defer srv.Close()
	r := &AssetSourceResolver{Client: xolu.New(srv.URL)}

	for source := range assetSourceQueries {
		t.Run(source, func(t *testing.T) {
			got, err := r.Resolve(context.Background(), source)
			if err != nil {
				t.Fatalf("Resolve(%q): %v", source, err)
			}
			if len(got) != 1 {
				t.Fatalf("Resolve(%q) returned %d options, want 1", source, len(got))
			}
			if got[0].Value != "1" {
				t.Errorf("Resolve(%q)[0].Value = %q, want \"1\"", source, got[0].Value)
			}
			if got[0].Label == "" {
				t.Errorf("Resolve(%q)[0].Label is empty", source)
			}
		})
	}
}

func TestAssetSourceResolver_UnrecognisedSource(t *testing.T) {
	srv := newTestOQLServer(t, nil)
	defer srv.Close()
	r := &AssetSourceResolver{Client: xolu.New(srv.URL)}

	_, err := r.Resolve(context.Background(), "entity:does-not-exist")
	if err == nil {
		t.Fatal("expected an error for an unrecognised source, got nil")
	}
}

func TestAssetSourceResolver_EmptyResultIsEmptySliceNotNil(t *testing.T) {
	srv := newTestOQLServer(t, map[string][]map[string]any{
		"SELECT * FROM brands ORDER BY name": {},
	})
	defer srv.Close()
	r := &AssetSourceResolver{Client: xolu.New(srv.URL)}

	got, err := r.Resolve(context.Background(), "entity:brands")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got == nil {
		t.Error("expected an empty slice, got nil -- formengine's renderSelect treats a nil vs empty slice the same for rendering, but nil is the wrong zero value for a resolver contract to hand back")
	}
	if len(got) != 0 {
		t.Errorf("expected 0 options, got %d", len(got))
	}
}

func TestAssetSourceResolver_IDConvertedFromFloat64Correctly(t *testing.T) {
	// The bug this test exists to catch: toString() only handles the
	// string case and silently returns "" for anything else, including
	// the float64 xolu actually sends for numeric ids. Caught during
	// implementation by reading toString's source before trusting it,
	// not by this test discovering it first -- kept as a standing
	// regression guard regardless.
	srv := newTestOQLServer(t, map[string][]map[string]any{
		"SELECT * FROM brands ORDER BY name": {
			{"id": float64(42), "code": "X", "name": "Y"},
		},
	})
	defer srv.Close()
	r := &AssetSourceResolver{Client: xolu.New(srv.URL)}

	got, err := r.Resolve(context.Background(), "entity:brands")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got[0].Value != "42" {
		t.Errorf("Value = %q, want \"42\" (not empty, not \"42.000000\")", got[0].Value)
	}
}
