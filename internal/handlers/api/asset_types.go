package api

import (
	"fmt"
	"net/http"

	"github.com/ha1tch/queryfy"
	"github.com/ha1tch/seam-ui/internal/validation"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

// AssetTypeHandler handles asset type API endpoints.
type AssetTypeHandler struct {
	*Handler
}

// NewAssetTypeHandler creates a new asset type handler.
func NewAssetTypeHandler(h *Handler) *AssetTypeHandler {
	return &AssetTypeHandler{Handler: h}
}

// List handles GET /api/v1/asset-types
func (h *AssetTypeHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	params := GetPaginationParams(r)

	typesResult, err := client.List(ctx, "asset_types", &xolu.ListParams{
		Limit:  params.Limit,
		Offset: params.Offset,
		Sort:   params.Sort,
	})
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, NewListResponse(typesResult.Entities, typesResult.TotalItems, params.Limit, params.Offset))
}

// Get handles GET /api/v1/asset-types/{id}
func (h *AssetTypeHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid asset type ID")
		return
	}

	assetType, err := client.Get(ctx, "asset_types", id)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, assetType)
}

// Create handles POST /api/v1/asset-types
func (h *AssetTypeHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	var data map[string]any
	if err := ReadJSON(r, &data); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	if err := queryfy.Validate(data, validation.AssetTypeCreateSchema); err != nil {
		WriteValidationError(w, err)
		return
	}

	assetType, err := client.Create(ctx, "asset_types", data)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	WriteJSON(w, http.StatusCreated, assetType)
}

// Update handles PUT /api/v1/asset-types/{id}
func (h *AssetTypeHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid asset type ID")
		return
	}

	var data map[string]any
	if err := ReadJSON(r, &data); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	if err := queryfy.Validate(data, validation.AssetTypeUpdateSchema); err != nil {
		WriteValidationError(w, err)
		return
	}

	assetType, err := client.Update(ctx, "asset_types", id, data)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, assetType)
}

// Delete handles DELETE /api/v1/asset-types/{id}
func (h *AssetTypeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid asset type ID")
		return
	}

	// Check if any assets use this type
	query := fmt.Sprintf("SELECT COUNT(*) as count FROM assets WHERE type_id = %d", id)
	result, err := client.OQL(ctx, query)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	if len(result.Data) > 0 {
		if count, ok := result.Data[0]["count"].(float64); ok && count > 0 {
			WriteError(w, http.StatusConflict, "type_in_use",
				fmt.Sprintf("Cannot delete asset type: %d assets are using it", int(count)))
			return
		}
	}

	if err := client.Delete(ctx, "asset_types", id); err != nil {
		WriteXOLUError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetAssets handles GET /api/v1/asset-types/{id}/assets
// Returns all assets of this type.
func (h *AssetTypeHandler) GetAssets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid asset type ID")
		return
	}

	params := GetPaginationParams(r)

	fetchCount := params.Limit + params.Offset
	query := fmt.Sprintf("SELECT TOP %d * FROM assets WHERE type_id = %d",
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

// GetIdentityTemplate handles GET /api/v1/asset-types/{id}/identity-template
// Returns the identity field template for this asset type.
func (h *AssetTypeHandler) GetIdentityTemplate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid asset type ID")
		return
	}

	assetType, err := client.Get(ctx, "asset_types", id)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	template, ok := assetType.Data["identity_template"]
	if !ok {
		WriteJSON(w, http.StatusOK, map[string]any{"fields": []any{}})
		return
	}

	WriteJSON(w, http.StatusOK, template)
}

// UpdateIdentityTemplate handles PUT /api/v1/asset-types/{id}/identity-template
// Updates the identity field template for this asset type.
func (h *AssetTypeHandler) UpdateIdentityTemplate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid asset type ID")
		return
	}

	var template map[string]any
	if err := ReadJSON(r, &template); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	// Validate template structure
	if err := queryfy.Validate(template, validation.IdentityTemplateSchema); err != nil {
		WriteValidationError(w, err)
		return
	}

	_, err = client.Patch(ctx, "asset_types", id, map[string]any{
		"identity_template": template,
	})
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, template)
}
