package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/ha1tch/queryfy"
	"github.com/ha1tch/seam-ui/internal/ui"
	"github.com/ha1tch/seam-ui/internal/validation"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

// AssetHandler handles asset-related API endpoints.
type AssetHandler struct {
	*Handler
}

// NewAssetHandler creates a new asset handler.
func NewAssetHandler(h *Handler) *AssetHandler {
	return &AssetHandler{Handler: h}
}

// List handles GET /api/v1/assets
func (h *AssetHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	params := GetPaginationParams(r)

	// Additional filters
	typeID := GetQueryInt(r, "type_id", 0)
	status := GetQueryString(r, "status", "")
	tags := GetQueryStrings(r, "tag")

	// Build OQL query for complex filtering
	if typeID > 0 || status != "" || len(tags) > 0 {
		// OQL uses TOP n syntax; for pagination we fetch extra and slice
		fetchCount := params.Limit + params.Offset
		query := fmt.Sprintf("SELECT TOP %d * FROM assets WHERE 1=1", fetchCount)
		if typeID > 0 {
			query += fmt.Sprintf(" AND type_id = %d", typeID)
		}
		if status != "" {
			query += fmt.Sprintf(" AND status = '%s'", status)
		}
		// Note: Tag filtering would require more complex query

		result, err := client.OQL(ctx, query)
		if err != nil {
			WriteXOLUError(w, err)
			return
		}

		// Apply offset by slicing results
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

	// Simple list
	assetsResult, err := client.List(ctx, "assets", &xolu.ListParams{
		Limit:  params.Limit,
		Offset: params.Offset,
		Sort:   params.Sort,
	})
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, NewListResponse(assetsResult.Entities, assetsResult.TotalItems, params.Limit, params.Offset))
}

// Get handles GET /api/v1/assets/{id}
func (h *AssetHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	// Position 3: /api/v1/assets/{id}
	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid asset ID")
		return
	}

	asset, err := client.Get(ctx, "assets", id)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, asset)
}

// Create handles POST /api/v1/assets
func (h *AssetHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	var data map[string]any
	if err := ReadJSON(r, &data); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	// Validate with schema (loose mode for dynamic identity fields)
	if err := queryfy.ValidateWithMode(data, validation.AssetCreateSchema, queryfy.Loose); err != nil {
		WriteValidationError(w, err)
		return
	}

	// TODO: Validate identity fields against asset type template

	// AssetCreateSchema validates English field names/values (internal/
	// external law, T-24). xolu's actual "assets" storage is still
	// Spanish-keyed -- translate here, the same table the dynamic Asset
	// form already uses, so this path persists in the identical shape.
	data = ui.AssetValuesToXolu(data)

	asset, err := client.Create(ctx, "assets", data)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	WriteJSON(w, http.StatusCreated, asset)
}

// Update handles PUT /api/v1/assets/{id}
func (h *AssetHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid asset ID")
		return
	}

	var data map[string]any
	if err := ReadJSON(r, &data); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	// Validate
	if err := queryfy.ValidateWithMode(data, validation.AssetUpdateSchema, queryfy.Loose); err != nil {
		WriteValidationError(w, err)
		return
	}

	// See Create's comment: translate English-keyed validated data to
	// xolu's Spanish-keyed storage before persisting.
	data = ui.AssetValuesToXolu(data)

	asset, err := client.Update(ctx, "assets", id, data)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, asset)
}

// Patch handles PATCH /api/v1/assets/{id}
func (h *AssetHandler) Patch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid asset ID")
		return
	}

	var data map[string]any
	if err := ReadJSON(r, &data); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	// For PATCH, validation is optional for provided fields only
	// In production, we'd validate only the fields present in the request

	asset, err := client.Patch(ctx, "assets", id, data)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, asset)
}

// Delete handles DELETE /api/v1/assets/{id}
func (h *AssetHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid asset ID")
		return
	}

	if err := client.Delete(ctx, "assets", id); err != nil {
		WriteXOLUError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Search handles GET /api/v1/assets/search
func (h *AssetHandler) Search(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	query := GetQueryString(r, "q", "")
	if query == "" {
		WriteError(w, http.StatusBadRequest, "missing_query", "Search query 'q' is required")
		return
	}

	params := GetPaginationParams(r)

	assets, err := client.Search(ctx, "assets", xolu.SearchParams{
		Query:  query,
		Entity: "assets",
		Limit:  params.Limit,
		Offset: params.Offset,
	})
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, NewListResponse(assets, len(assets), params.Limit, params.Offset))
}

// GetByQR handles GET /api/v1/qr/{code}
func (h *AssetHandler) GetByQR(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	// Position 3: /api/v1/qr/{code}
	code, err := GetStringFromPath(r, 3)
	if err != nil || code == "" {
		WriteError(w, http.StatusBadRequest, "invalid_code", "Invalid QR code")
		return
	}

	// Query for asset by QR code
	query := fmt.Sprintf("SELECT TOP 1 * FROM assets WHERE qr_code = '%s'", code)
	result, err := client.OQL(ctx, query)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	if len(result.Data) == 0 {
		WriteError(w, http.StatusNotFound, "not_found", "Asset not found")
		return
	}

	WriteJSON(w, http.StatusOK, result.Data[0])
}

// UpdateStatus handles PATCH /api/v1/assets/{id}/status
func (h *AssetHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid asset ID")
		return
	}

	var data struct {
		Status string `json:"status"`
	}
	if err := ReadJSON(r, &data); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	validStatuses := map[string]bool{"active": true, "silent": true, "alert": true, "lost": true}
	if !validStatuses[data.Status] {
		WriteError(w, http.StatusBadRequest, "invalid_status", "Status must be one of: active, silent, alert, lost")
		return
	}

	asset, err := client.Patch(ctx, "assets", id, map[string]any{"status": data.Status})
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, asset)
}

// GetEvents handles GET /api/v1/assets/{id}/events
func (h *AssetHandler) GetEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid asset ID")
		return
	}

	params := GetPaginationParams(r)

	// Query events for this asset (OQL uses TOP n, no OFFSET)
	fetchCount := params.Limit + params.Offset
	query := fmt.Sprintf("SELECT TOP %d * FROM events WHERE asset_id = %d ORDER BY timestamp DESC",
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

// BatchUpdate handles POST /api/v1/assets/batch
func (h *AssetHandler) BatchUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	var batch struct {
		IDs    []int64        `json:"ids"`
		Update map[string]any `json:"update"`
	}

	if err := ReadJSON(r, &batch); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	if len(batch.IDs) == 0 {
		WriteError(w, http.StatusBadRequest, "missing_ids", "At least one asset ID is required")
		return
	}

	if len(batch.IDs) > 100 {
		WriteError(w, http.StatusBadRequest, "too_many_ids", "Maximum 100 assets per batch")
		return
	}

	// Update each asset
	results := make([]map[string]any, 0, len(batch.IDs))
	for _, id := range batch.IDs {
		asset, err := client.Patch(ctx, "assets", id, batch.Update)
		if err != nil {
			results = append(results, map[string]any{
				"id":      id,
				"success": false,
				"error":   err.Error(),
			})
		} else {
			results = append(results, map[string]any{
				"id":      id,
				"success": true,
				"data":    asset,
			})
		}
	}

	WriteJSON(w, http.StatusOK, map[string]any{"results": results})
}

// Placeholder for chi URLParam - in production use chi's URLParam
func urlParam(r *http.Request, key string) string {
	// This is a simplified version - chi router provides this
	return ""
}

// Ensure context is used (for linter)
var _ context.Context
