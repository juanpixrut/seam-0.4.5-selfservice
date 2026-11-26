package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEventHandler_List(t *testing.T) {
	server := mockOLUServer(t, map[string]http.HandlerFunc{
		"POST /api/v1/oql": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, http.StatusOK, map[string]any{
				"status": "completed",
				"data": []map[string]any{
					{"id": 1, "asset_id": 1, "trigger_type": "temperature"},
					{"id": 2, "asset_id": 1, "trigger_type": "humidity"},
				},
				"stats": map[string]any{
					"rows_returned": 2,
				},
			})
		},
	})
	defer server.Close()

	h := NewEventHandler(testHandler(t, server))

	req := httptest.NewRequest("GET", "/api/v1/events?asset_id=1", nil)
	rec := httptest.NewRecorder()

	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestEventHandler_Get(t *testing.T) {
	server := mockOLUServer(t, map[string]http.HandlerFunc{
		"GET /api/v1/events/": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, http.StatusOK, map[string]any{
				"id": 1,
				"data": map[string]any{
					"asset_id":     1,
					"trigger_type": "temperature",
					"value":        map[string]any{"temp": 25.5},
				},
			})
		},
	})
	defer server.Close()

	h := NewEventHandler(testHandler(t, server))

	req := httptest.NewRequest("GET", "/api/v1/events/1", nil)
	rec := httptest.NewRecorder()

	h.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestEventHandler_Ingest(t *testing.T) {
	server := mockOLUServer(t, map[string]http.HandlerFunc{
		"POST /api/v1/events": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			readJSONBody(r, &body)

			jsonResponse(w, http.StatusCreated, map[string]any{
				"id":   1,
				"data": body,
			})
		},
	})
	defer server.Close()

	h := NewEventHandler(testHandler(t, server))

	body := `{
		"asset_id": 1,
		"trigger_type": "sensor_driver",
		"timestamp": "2025-01-07T12:00:00Z",
		"value": {"temp": 28.5}
	}`
	req := httptest.NewRequest("POST", "/api/v1/events", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Ingest(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestEventHandler_Ingest_ValidationError(t *testing.T) {
	server := mockOLUServer(t, map[string]http.HandlerFunc{})
	defer server.Close()

	h := NewEventHandler(testHandler(t, server))

	// Missing required fields
	body := `{"value": {}}`
	req := httptest.NewRequest("POST", "/api/v1/events", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Ingest(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestEventHandler_IngestBatch(t *testing.T) {
	createCount := 0
	server := mockOLUServer(t, map[string]http.HandlerFunc{
		"POST /api/v1/events": func(w http.ResponseWriter, r *http.Request) {
			createCount++
			var body map[string]any
			readJSONBody(r, &body)

			jsonResponse(w, http.StatusCreated, map[string]any{
				"id":   createCount,
				"data": body,
			})
		},
	})
	defer server.Close()

	h := NewEventHandler(testHandler(t, server))

	body := `{
		"events": [
			{"asset_id": 1, "trigger_type": "sensor_driver", "timestamp": "2025-01-07T12:00:00Z", "value": {"temp": 25}},
			{"asset_id": 1, "trigger_type": "sensor_driver", "timestamp": "2025-01-07T12:01:00Z", "value": {"humidity": 60}}
		]
	}`
	req := httptest.NewRequest("POST", "/api/v1/events/batch", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.IngestBatch(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["success"].(float64) != 2 {
		t.Errorf("expected 2 successful, got %v", resp["success"])
	}
}

func TestEventHandler_IngestBatch_Empty(t *testing.T) {
	server := mockOLUServer(t, map[string]http.HandlerFunc{})
	defer server.Close()

	h := NewEventHandler(testHandler(t, server))

	body := `{"events": []}`
	req := httptest.NewRequest("POST", "/api/v1/events/batch", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.IngestBatch(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestEventHandler_IngestBatch_TooLarge(t *testing.T) {
	server := mockOLUServer(t, map[string]http.HandlerFunc{})
	defer server.Close()

	h := NewEventHandler(testHandler(t, server))

	// Create batch with 1001 events
	events := make([]map[string]any, 1001)
	for i := range events {
		events[i] = map[string]any{"asset_id": 1, "trigger_type": "test", "value": map[string]any{}}
	}
	bodyBytes, _ := json.Marshal(map[string]any{"events": events})

	req := httptest.NewRequest("POST", "/api/v1/events/batch", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.IngestBatch(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestEventHandler_GetStats(t *testing.T) {
	server := mockOLUServer(t, map[string]http.HandlerFunc{
		"POST /api/v1/oql": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, http.StatusOK, map[string]any{
				"status": "completed",
				"data": []map[string]any{
					{"period": "2025-01-01", "count": 100},
					{"period": "2025-01-02", "count": 150},
				},
				"stats": map[string]any{
					"rows_returned": 2,
				},
			})
		},
	})
	defer server.Close()

	h := NewEventHandler(testHandler(t, server))

	req := httptest.NewRequest("GET", "/api/v1/events/stats?group_by=day", nil)
	rec := httptest.NewRecorder()

	h.GetStats(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["group_by"] != "day" {
		t.Errorf("expected group_by=day, got %v", resp["group_by"])
	}
}
