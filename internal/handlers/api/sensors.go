package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/ha1tch/queryfy"
	"github.com/ha1tch/seam-ui/internal/validation"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

// SensorHandler handles sensor-related API endpoints.
type SensorHandler struct {
	*Handler
}

// NewSensorHandler creates a new sensor handler.
func NewSensorHandler(h *Handler) *SensorHandler {
	return &SensorHandler{Handler: h}
}

// List handles GET /api/v1/sensors
func (h *SensorHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	params := GetPaginationParams(r)

	// Filters
	sensorType := GetQueryString(r, "type", "")
	status := GetQueryString(r, "status", "")

	if sensorType != "" || status != "" {
		fetchCount := params.Limit + params.Offset
		query := fmt.Sprintf("SELECT TOP %d * FROM sensors WHERE 1=1", fetchCount)
		if sensorType != "" {
			query += fmt.Sprintf(" AND type = '%s'", sensorType)
		}
		if status != "" {
			query += fmt.Sprintf(" AND status = '%s'", status)
		}

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
		return
	}

	sensorsResult, err := client.List(ctx, "sensors", &xolu.ListParams{
		Limit:  params.Limit,
		Offset: params.Offset,
		Sort:   params.Sort,
	})
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, NewListResponse(sensorsResult.Entities, sensorsResult.TotalItems, params.Limit, params.Offset))
}

// Get handles GET /api/v1/sensors/{id}
func (h *SensorHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid sensor ID")
		return
	}

	sensor, err := client.Get(ctx, "sensors", id)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, sensor)
}

// Create handles POST /api/v1/sensors
func (h *SensorHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	var data map[string]any
	if err := ReadJSON(r, &data); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	if err := queryfy.ValidateWithMode(data, validation.SensorCreateSchema, queryfy.Loose); err != nil {
		WriteValidationError(w, err)
		return
	}

	sensor, err := client.Create(ctx, "sensors", data)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	WriteJSON(w, http.StatusCreated, sensor)
}

// Update handles PUT /api/v1/sensors/{id}
func (h *SensorHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid sensor ID")
		return
	}

	var data map[string]any
	if err := ReadJSON(r, &data); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	if err := queryfy.ValidateWithMode(data, validation.SensorUpdateSchema, queryfy.Loose); err != nil {
		WriteValidationError(w, err)
		return
	}

	sensor, err := client.Update(ctx, "sensors", id, data)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, sensor)
}

// Delete handles DELETE /api/v1/sensors/{id}
func (h *SensorHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid sensor ID")
		return
	}

	if err := client.Delete(ctx, "sensors", id); err != nil {
		WriteXOLUError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Bind handles POST /api/v1/sensors/{id}/bind
// Binds a sensor to an asset.
func (h *SensorHandler) Bind(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid sensor ID")
		return
	}

	var data struct {
		AssetID int64 `json:"asset_id"`
	}
	if err := ReadJSON(r, &data); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	if data.AssetID == 0 {
		WriteError(w, http.StatusBadRequest, "missing_asset_id", "asset_id is required")
		return
	}

	// Get sensor to get external_id
	sensor, err := client.Get(ctx, "sensors", id)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	externalID, _ := sensor.Data["external_id"].(string)

	// Create binding record
	binding := map[string]any{
		"sensor_id":          id,
		"sensor_external_id": externalID,
		"asset_id":           data.AssetID,
		"bound_at":           time.Now().UTC().Format(time.RFC3339),
		"active":             true,
	}

	_, err = client.Create(ctx, "sensor_bindings", binding)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	// Update sensor with current asset binding
	_, err = client.Patch(ctx, "sensors", id, map[string]any{
		"current_asset_id": data.AssetID,
		"status":           "bound",
	})
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"sensor_id": id,
		"asset_id":  data.AssetID,
		"bound_at":  binding["bound_at"],
	})
}

// Unbind handles POST /api/v1/sensors/{id}/unbind
// Unbinds a sensor from its current asset.
func (h *SensorHandler) Unbind(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid sensor ID")
		return
	}

	// Deactivate current binding
	query := fmt.Sprintf("UPDATE sensor_bindings SET active = false, unbound_at = '%s' WHERE sensor_id = %d AND active = true",
		time.Now().UTC().Format(time.RFC3339), id)
	_, err = client.OQL(ctx, query)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	// Update sensor status
	_, err = client.Patch(ctx, "sensors", id, map[string]any{
		"current_asset_id": nil,
		"status":           "unbound",
	})
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"sensor_id": id,
		"unbound":   true,
	})
}

// GetBindings handles GET /api/v1/sensors/{id}/bindings
// Returns binding history for a sensor.
func (h *SensorHandler) GetBindings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid sensor ID")
		return
	}

	params := GetPaginationParams(r)

	fetchCount := params.Limit + params.Offset
	query := fmt.Sprintf("SELECT TOP %d * FROM sensor_bindings WHERE sensor_id = %d ORDER BY bound_at DESC",
		fetchCount, id)

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
