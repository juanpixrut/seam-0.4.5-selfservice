package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ha1tch/queryfy"
	"github.com/ha1tch/seam-ui/internal/validation"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

// EventHandler handles event-related API endpoints.
type EventHandler struct {
	*Handler
}

// NewEventHandler creates a new event handler.
func NewEventHandler(h *Handler) *EventHandler {
	return &EventHandler{
		Handler: h,
	}
}

// List handles GET /api/v1/events
func (h *EventHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	params := GetPaginationParams(r)

	// Filters
	assetID := GetQueryInt(r, "asset_id", 0)
	triggerType := GetQueryString(r, "trigger_type", "")
	from := GetQueryString(r, "from", "")
	to := GetQueryString(r, "to", "")

	// Build query (OQL uses TOP n, not LIMIT/OFFSET)
	fetchCount := params.Limit + params.Offset
	query := fmt.Sprintf("SELECT TOP %d * FROM events WHERE 1=1", fetchCount)
	if assetID > 0 {
		query += fmt.Sprintf(" AND asset_id = %d", assetID)
	}
	if triggerType != "" {
		query += fmt.Sprintf(" AND trigger_type = '%s'", triggerType)
	}
	if from != "" {
		query += fmt.Sprintf(" AND timestamp >= '%s'", from)
	}
	if to != "" {
		query += fmt.Sprintf(" AND timestamp <= '%s'", to)
	}
	query += " ORDER BY timestamp DESC"

	result, err := client.OQL(ctx, query)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	// Apply offset by slicing
	data := result.Data
	if params.Offset > 0 && params.Offset < len(data) {
		data = data[params.Offset:]
	} else if params.Offset >= len(data) {
		data = []map[string]any{}
	}
	if len(data) > params.Limit {
		data = data[:params.Limit]
	}

	WriteJSON(w, http.StatusOK, NewListResponse(data, len(data), params.Limit, params.Offset))
}

// Get handles GET /api/v1/events/{id}
func (h *EventHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid event ID")
		return
	}

	event, err := client.Get(ctx, "events", id)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, event)
}

// Ingest handles POST /api/v1/events
// This is the main endpoint for ingesting sensor data and other events.
func (h *EventHandler) Ingest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	var data map[string]any
	if err := ReadJSON(r, &data); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	// Validate event
	if err := queryfy.ValidateWithMode(data, validation.EventIngestSchema, queryfy.Loose); err != nil {
		WriteValidationError(w, err)
		return
	}

	// Resolve sensor_id to asset_id if needed
	if sensorID, ok := data["sensor_id"].(string); ok && sensorID != "" {
		assetID, err := h.resolveSensorToAsset(ctx, client, sensorID)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "sensor_not_bound", "Sensor is not bound to an asset")
			return
		}
		data["asset_id"] = assetID
	}

	// Set timestamp if not provided
	if _, ok := data["timestamp"]; !ok {
		data["timestamp"] = time.Now().UTC().Format(time.RFC3339)
	}

	// Create event in xolu
	created, err := client.Create(ctx, "events", data)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	// Re-fetch the event document — xolu does not echo on Create.
	event, err := client.Get(ctx, "events", created.ID)
	if err != nil {
		// Non-fatal: respond with the ID only and skip processing.
		WriteJSON(w, http.StatusCreated, map[string]any{"id": created.ID})
		return
	}

	WriteJSON(w, http.StatusCreated, event.Data)
}

// IngestBatch handles POST /api/v1/events/batch
func (h *EventHandler) IngestBatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	var batch struct {
		Events []map[string]any `json:"events"`
	}

	if err := ReadJSON(r, &batch); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	if len(batch.Events) == 0 {
		WriteError(w, http.StatusBadRequest, "empty_batch", "At least one event is required")
		return
	}

	if len(batch.Events) > 1000 {
		WriteError(w, http.StatusBadRequest, "batch_too_large", "Maximum 1000 events per batch")
		return
	}

	results := make([]map[string]any, 0, len(batch.Events))
	successCount := 0

	for i, eventData := range batch.Events {
		// Validate each event
		if err := queryfy.ValidateWithMode(eventData, validation.EventIngestSchema, queryfy.Loose); err != nil {
			results = append(results, map[string]any{
				"index":   i,
				"success": false,
				"error":   err.Error(),
			})
			continue
		}

		// Set timestamp if not provided
		if _, ok := eventData["timestamp"]; !ok {
			eventData["timestamp"] = time.Now().UTC().Format(time.RFC3339)
		}

		// Create event
		event, err := client.Create(ctx, "events", eventData)
		if err != nil {
			results = append(results, map[string]any{
				"index":   i,
				"success": false,
				"error":   err.Error(),
			})
			continue
		}

		results = append(results, map[string]any{
			"index":   i,
			"success": true,
			"id":      event.ID,
		})
		successCount++

	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"total":   len(batch.Events),
		"success": successCount,
		"failed":  len(batch.Events) - successCount,
		"results": results,
	})
}

// resolveSensorToAsset looks up the asset ID bound to a sensor.
func (h *EventHandler) resolveSensorToAsset(ctx context.Context, client *xolu.Client, sensorID string) (int64, error) {
	query := fmt.Sprintf("SELECT TOP 1 asset_id FROM sensor_bindings WHERE sensor_external_id = '%s' AND active = true", sensorID)
	result, err := client.OQL(ctx, query)
	if err != nil {
		return 0, err
	}
	if len(result.Data) == 0 {
		return 0, fmt.Errorf("sensor not bound")
	}
	row := result.Data[0]
	if assetID, ok := row["asset_id"].(float64); ok {
		return int64(assetID), nil
	}
	return 0, fmt.Errorf("invalid asset_id")
}

// GetStats handles GET /api/v1/events/stats
func (h *EventHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	// Time range
	from := GetQueryString(r, "from", time.Now().AddDate(0, 0, -7).Format(time.RFC3339))
	to := GetQueryString(r, "to", time.Now().Format(time.RFC3339))
	groupBy := GetQueryString(r, "group_by", "day") // day, hour, trigger_type

	var query string
	switch groupBy {
	case "trigger_type":
		query = fmt.Sprintf(`
			SELECT trigger_type, COUNT(*) as count 
			FROM events 
			WHERE timestamp >= '%s' AND timestamp <= '%s'
			GROUP BY trigger_type
		`, from, to)
	case "hour":
		query = fmt.Sprintf(`
			SELECT DATE_TRUNC('hour', timestamp) as period, COUNT(*) as count 
			FROM events 
			WHERE timestamp >= '%s' AND timestamp <= '%s'
			GROUP BY period
			ORDER BY period
		`, from, to)
	default: // day
		query = fmt.Sprintf(`
			SELECT DATE_TRUNC('day', timestamp) as period, COUNT(*) as count 
			FROM events 
			WHERE timestamp >= '%s' AND timestamp <= '%s'
			GROUP BY period
			ORDER BY period
		`, from, to)
	}

	result, err := client.OQL(ctx, query)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"from":     from,
		"to":       to,
		"group_by": groupBy,
		"data":     result.Data,
	})
}
