package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	xolu "github.com/ha1tch/xolu/pkg/client"
)

// testHandler creates a Handler with a mock xolu client pointing to the test server.
func testHandler(t *testing.T, server *httptest.Server) *Handler {
	client := xolu.New(server.URL)
	return NewHandler(client)
}

// mockOLUServer creates a test server that simulates xolu responses.
func mockOLUServer(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Find matching handler by method + path prefix
		for pattern, handler := range handlers {
			if matchRoute(r.Method, r.URL.Path, pattern) {
				handler(w, r)
				return
			}
		}

		// Default 404
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	}))
}

// matchRoute checks if method+path matches a pattern like "GET /api/v1/assets"
func matchRoute(method, path, pattern string) bool {
	// Simple prefix matching - pattern format: "METHOD /path"
	parts := bytes.SplitN([]byte(pattern), []byte(" "), 2)
	if len(parts) != 2 {
		return false
	}

	patternMethod := string(parts[0])
	patternPath := string(parts[1])

	if method != patternMethod {
		return false
	}

	// Check if path starts with pattern path
	return len(path) >= len(patternPath) && path[:len(patternPath)] == patternPath
}

// jsonResponse writes a JSON response.
func jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// readJSONBody reads and parses JSON from request body.
func readJSONBody(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// =============================================================================
// ASSET HANDLER TESTS
// =============================================================================

func TestAssetHandler_List(t *testing.T) {
	server := mockOLUServer(t, map[string]http.HandlerFunc{
		"GET /api/v1/assets": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, http.StatusOK, map[string]any{
				"data": []map[string]any{
					{"id": float64(1), "name": "Asset 1", "status": "active"},
					{"id": float64(2), "name": "Asset 2", "status": "active"},
				},
				"pagination": map[string]any{
					"page": 1, "per_page": 10,
					"total_items": 2, "total_pages": 1,
				},
			})
		},
	})
	defer server.Close()

	h := NewAssetHandler(testHandler(t, server))

	req := httptest.NewRequest("GET", "/api/v1/assets?limit=10", nil)
	rec := httptest.NewRecorder()

	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp ListResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Meta.Limit != 10 {
		t.Errorf("expected limit 10, got %d", resp.Meta.Limit)
	}
}

func TestAssetHandler_Get(t *testing.T) {
	server := mockOLUServer(t, map[string]http.HandlerFunc{
		"GET /api/v1/assets/": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, http.StatusOK, map[string]any{
				"id":   1,
				"data": map[string]any{"name": "Test Asset", "status": "active"},
			})
		},
	})
	defer server.Close()

	h := NewAssetHandler(testHandler(t, server))

	req := httptest.NewRequest("GET", "/api/v1/assets/1", nil)
	rec := httptest.NewRecorder()

	h.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestAssetHandler_Create(t *testing.T) {
	var received map[string]any
	server := mockOLUServer(t, map[string]http.HandlerFunc{
		"POST /api/v1/assets": func(w http.ResponseWriter, r *http.Request) {
			readJSONBody(r, &received)

			jsonResponse(w, http.StatusCreated, map[string]any{
				"id":   1,
				"data": received,
			})
		},
	})
	defer server.Close()

	h := NewAssetHandler(testHandler(t, server))

	// English field name/value (AssetCreateSchema, T-24). xolu's own
	// storage column is now "criticality" too (T-40, this session's own
	// Asset rename, Block B) -- only the enum *value* still translates
	// (Medium->Media), asserted below.
	body := `{
		"name": "New Asset",
		"type_id": 1,
		"status": "active",
		"criticality": "Medium",
		"identity": {"serial": "SN001"},
		"connectivity_tier": "gateway"
	}`
	req := httptest.NewRequest("POST", "/api/v1/assets", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := received["criticality"]; got != "Media" {
		t.Errorf("expected xolu to receive criticality=Media (English key, translated value), got criticality=%v (full body: %v)", got, received)
	}
}

func TestAssetHandler_Create_ValidationError(t *testing.T) {
	server := mockOLUServer(t, map[string]http.HandlerFunc{})
	defer server.Close()

	h := NewAssetHandler(testHandler(t, server))

	// Missing required fields
	body := `{"name": "A"}`
	req := httptest.NewRequest("POST", "/api/v1/assets", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rec.Code)
	}
}

func TestAssetHandler_Update(t *testing.T) {
	server := mockOLUServer(t, map[string]http.HandlerFunc{
		"PUT /api/v1/assets/": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			readJSONBody(r, &body)

			jsonResponse(w, http.StatusOK, map[string]any{
				"id":   1,
				"data": body,
			})
		},
	})
	defer server.Close()

	h := NewAssetHandler(testHandler(t, server))

	body := `{"name": "Updated Asset", "status": "active"}`
	req := httptest.NewRequest("PUT", "/api/v1/assets/1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAssetHandler_Delete(t *testing.T) {
	server := mockOLUServer(t, map[string]http.HandlerFunc{
		"DELETE /api/v1/assets/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer server.Close()

	h := NewAssetHandler(testHandler(t, server))

	req := httptest.NewRequest("DELETE", "/api/v1/assets/1", nil)
	rec := httptest.NewRecorder()

	h.Delete(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rec.Code)
	}
}

func TestAssetHandler_Search(t *testing.T) {
	server := mockOLUServer(t, map[string]http.HandlerFunc{
		// xolu search is at /search, not /{entity}/search
		"GET /api/v1/search": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("entity") != "assets" {
				t.Errorf("expected entity=assets, got %q", r.URL.Query().Get("entity"))
			}
			jsonResponse(w, http.StatusOK, map[string]any{
				"query":  "test",
				"entity": "assets",
				"count":  1,
				"results": []map[string]any{
					{"id": float64(1), "name": "Found Asset"},
				},
			})
		},
	})
	defer server.Close()

	h := NewAssetHandler(testHandler(t, server))

	req := httptest.NewRequest("GET", "/api/v1/assets/search?q=test", nil)
	rec := httptest.NewRecorder()

	h.Search(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestAssetHandler_Search_MissingQuery(t *testing.T) {
	server := mockOLUServer(t, map[string]http.HandlerFunc{})
	defer server.Close()

	h := NewAssetHandler(testHandler(t, server))

	req := httptest.NewRequest("GET", "/api/v1/assets/search", nil)
	rec := httptest.NewRecorder()

	h.Search(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestAssetHandler_UpdateStatus(t *testing.T) {
	server := mockOLUServer(t, map[string]http.HandlerFunc{
		"PATCH /api/v1/assets/": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			readJSONBody(r, &body)

			jsonResponse(w, http.StatusOK, map[string]any{
				"id":   1,
				"data": map[string]any{"status": body["status"]},
			})
		},
	})
	defer server.Close()

	h := NewAssetHandler(testHandler(t, server))

	body := `{"status": "alert"}`
	req := httptest.NewRequest("PATCH", "/api/v1/assets/1/status", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.UpdateStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAssetHandler_UpdateStatus_Invalid(t *testing.T) {
	server := mockOLUServer(t, map[string]http.HandlerFunc{})
	defer server.Close()

	h := NewAssetHandler(testHandler(t, server))

	body := `{"status": "invalid_status"}`
	req := httptest.NewRequest("PATCH", "/api/v1/assets/1/status", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.UpdateStatus(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestAssetHandler_BatchUpdate(t *testing.T) {
	patchCount := 0
	server := mockOLUServer(t, map[string]http.HandlerFunc{
		"PATCH /api/v1/assets/": func(w http.ResponseWriter, r *http.Request) {
			patchCount++
			jsonResponse(w, http.StatusOK, map[string]any{
				"id":   patchCount,
				"data": map[string]any{"status": "updated"},
			})
		},
	})
	defer server.Close()

	h := NewAssetHandler(testHandler(t, server))

	body := `{"ids": [1, 2, 3], "update": {"status": "silent"}}`
	req := httptest.NewRequest("POST", "/api/v1/assets/batch", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.BatchUpdate(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	results, ok := resp["results"].([]any)
	if !ok || len(results) != 3 {
		t.Errorf("expected 3 results, got %v", resp["results"])
	}
}

// Ensure context import is used
var _ context.Context
