package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ha1tch/seam-ui/internal/qrcode"
)

// QRHandler handles QR code management endpoints.
type QRHandler struct {
	*Handler
	generator *qrcode.Generator
}

// NewQRHandler creates a new QR code handler.
func NewQRHandler(h *Handler, gen *qrcode.Generator) *QRHandler {
	return &QRHandler{
		Handler:   h,
		generator: gen,
	}
}

// GenerateRequest represents a QR code generation request.
type GenerateRequest struct {
	Count       int    `json:"count"`
	AssetTypeID int64  `json:"asset_type_id,omitempty"`
	BatchID     string `json:"batch_id,omitempty"`
}

// AssignRequest represents a QR code assignment request.
type AssignRequest struct {
	AssetID int64 `json:"asset_id"`
}

// Generate handles POST /api/v1/qr/generate
func (h *QRHandler) Generate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)
	tenantID := getTenantID(r)

	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	if req.Count <= 0 {
		req.Count = 1
	}
	if req.Count > 1000 {
		WriteError(w, http.StatusBadRequest, "bad_request", "maximum 1000 codes per request")
		return
	}

	// Generate batch ID if not provided
	batchID := req.BatchID
	if batchID == "" {
		batchID = time.Now().Format("20060102-150405")
	}

	// Generate codes
	codes, err := h.generator.GenerateBatch(req.Count)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "generation_failed", err.Error())
		return
	}

	// Store codes in xolu
	results := make([]map[string]interface{}, 0, len(codes))
	for _, code := range codes {
		data := map[string]interface{}{
			"tenant_id":  tenantID,
			"code":       code,
			"status":     qrcode.StatusAvailable,
			"batch_id":   batchID,
			"created_at": time.Now(),
		}

		if req.AssetTypeID > 0 {
			data["asset_type_id"] = req.AssetTypeID
		}

		entity, err := client.Create(ctx, "qr_codes", data)
		if err != nil {
			continue // Skip failed ones
		}

		results = append(results, map[string]interface{}{
			"id":   entity.ID,
			"code": code,
			"url":  h.generator.URL(code),
		})
	}

	WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"batch_id":  batchID,
		"count":     len(results),
		"requested": req.Count,
		"codes":     results,
	})
}

// List handles GET /api/v1/qr
func (h *QRHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	params := GetPaginationParams(r)
	status := GetQueryString(r, "status", "")
	batchID := GetQueryString(r, "batch_id", "")

	// OQL uses TOP n, not LIMIT/OFFSET
	fetchCount := params.Limit + params.Offset
	query := fmt.Sprintf("SELECT TOP %d * FROM qr_codes WHERE 1=1", fetchCount)
	if status != "" {
		query += " AND status = '" + status + "'"
	}
	if batchID != "" {
		query += " AND batch_id = '" + batchID + "'"
	}
	query += " ORDER BY created_at DESC"

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

	// Add URLs to response
	for _, row := range data {
		if code, ok := row["code"].(string); ok {
			row["url"] = h.generator.URL(code)
		}
	}

	WriteJSON(w, http.StatusOK, NewListResponse(data, len(data), params.Limit, params.Offset))
}

// Get handles GET /api/v1/qr/{code}
func (h *QRHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	code := getQRCode(r)
	if code == "" {
		WriteError(w, http.StatusBadRequest, "bad_request", "code is required")
		return
	}

	// Validate code format
	if err := h.generator.Validate(code); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_code", err.Error())
		return
	}

	// Look up code
	query := "SELECT TOP 1 * FROM qr_codes WHERE code = '" + strings.ToUpper(code) + "'"
	result, err := client.OQL(ctx, query)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	if len(result.Data) == 0 {
		WriteError(w, http.StatusNotFound, "not_found", "QR code not found")
		return
	}

	row := result.Data[0]
	row["url"] = h.generator.URL(code)

	WriteJSON(w, http.StatusOK, row)
}

// Assign handles POST /api/v1/qr/{code}/assign
func (h *QRHandler) Assign(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	code := getQRCode(r)
	if code == "" {
		WriteError(w, http.StatusBadRequest, "bad_request", "code is required")
		return
	}

	var req AssignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	if req.AssetID <= 0 {
		WriteError(w, http.StatusBadRequest, "bad_request", "asset_id is required")
		return
	}

	// Look up code
	query := "SELECT TOP 1 * FROM qr_codes WHERE code = '" + strings.ToUpper(code) + "'"
	result, err := client.OQL(ctx, query)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	if len(result.Data) == 0 {
		WriteError(w, http.StatusNotFound, "not_found", "QR code not found")
		return
	}

	row := result.Data[0]
	qrID := int64(row["id"].(float64))
	status := row["status"].(string)

	if status == qrcode.StatusAssigned {
		WriteError(w, http.StatusConflict, "already_assigned", "QR code is already assigned")
		return
	}

	if status == qrcode.StatusDisabled {
		WriteError(w, http.StatusConflict, "disabled", "QR code is disabled")
		return
	}

	// Verify asset exists
	_, err = client.Get(ctx, "assets", req.AssetID)
	if err != nil {
		WriteError(w, http.StatusNotFound, "asset_not_found", "asset not found")
		return
	}

	// Update QR code
	updates := map[string]interface{}{
		"asset_id":    req.AssetID,
		"status":      qrcode.StatusAssigned,
		"assigned_at": time.Now(),
	}

	entity, err := client.Update(ctx, "qr_codes", qrID, updates)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	// Also update asset with QR code
	client.Update(ctx, "assets", req.AssetID, map[string]interface{}{
		"qr_code": code,
	})

	entity.Data["url"] = h.generator.URL(code)

	WriteJSON(w, http.StatusOK, entity.Data)
}

// Unassign handles POST /api/v1/qr/{code}/unassign
func (h *QRHandler) Unassign(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	code := getQRCode(r)
	if code == "" {
		WriteError(w, http.StatusBadRequest, "bad_request", "code is required")
		return
	}

	// Look up code
	query := "SELECT TOP 1 * FROM qr_codes WHERE code = '" + strings.ToUpper(code) + "'"
	result, err := client.OQL(ctx, query)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	if len(result.Data) == 0 {
		WriteError(w, http.StatusNotFound, "not_found", "QR code not found")
		return
	}

	row := result.Data[0]
	qrID := int64(row["id"].(float64))

	// Get asset ID before clearing
	var assetID int64
	if aid, ok := row["asset_id"].(float64); ok {
		assetID = int64(aid)
	}

	// Update QR code
	updates := map[string]interface{}{
		"asset_id":    nil,
		"status":      qrcode.StatusAvailable,
		"assigned_at": nil,
	}

	entity, err := client.Update(ctx, "qr_codes", qrID, updates)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	// Clear QR code from asset
	if assetID > 0 {
		client.Update(ctx, "assets", assetID, map[string]interface{}{
			"qr_code": nil,
		})
	}

	entity.Data["url"] = h.generator.URL(code)

	WriteJSON(w, http.StatusOK, entity.Data)
}

// Disable handles POST /api/v1/qr/{code}/disable
func (h *QRHandler) Disable(w http.ResponseWriter, r *http.Request) {
	h.setStatus(w, r, qrcode.StatusDisabled)
}

// Enable handles POST /api/v1/qr/{code}/enable
func (h *QRHandler) Enable(w http.ResponseWriter, r *http.Request) {
	h.setStatus(w, r, qrcode.StatusAvailable)
}

func (h *QRHandler) setStatus(w http.ResponseWriter, r *http.Request, status string) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	code := getQRCode(r)
	if code == "" {
		WriteError(w, http.StatusBadRequest, "bad_request", "code is required")
		return
	}

	// Look up code
	query := "SELECT TOP 1 id FROM qr_codes WHERE code = '" + strings.ToUpper(code) + "'"
	result, err := client.OQL(ctx, query)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	if len(result.Data) == 0 {
		WriteError(w, http.StatusNotFound, "not_found", "QR code not found")
		return
	}

	qrID := int64(result.Data[0]["id"].(float64))

	_, err = client.Update(ctx, "qr_codes", qrID, map[string]interface{}{
		"status": status,
	})
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	// Re-fetch after update — xolu does not echo the document on writes.
	updated, err := client.Get(ctx, "qr_codes", qrID)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	updated.Data["url"] = h.generator.URL(code)

	WriteJSON(w, http.StatusOK, updated.Data)
}

// Scan handles GET /api/v1/qr/{code}/scan
// This is the public endpoint hit when a QR code is scanned.
func (h *QRHandler) Scan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	code := getQRCode(r)
	if code == "" {
		WriteError(w, http.StatusBadRequest, "bad_request", "code is required")
		return
	}

	// Validate code format
	if err := h.generator.Validate(code); err != nil {
		WriteJSON(w, http.StatusOK, qrcode.ScanResult{
			Code:      code,
			Valid:     false,
			Error:     "invalid QR code format",
			ScannedAt: time.Now(),
		})
		return
	}

	// Look up code (without tenant filter for public scan)
	query := "SELECT TOP 1 * FROM qr_codes WHERE code = '" + strings.ToUpper(code) + "'"
	result, err := h.XOLU.OQL(ctx, query)
	if err != nil || len(result.Data) == 0 {
		WriteJSON(w, http.StatusOK, qrcode.ScanResult{
			Code:      code,
			Valid:     false,
			Error:     "QR code not found",
			ScannedAt: time.Now(),
		})
		return
	}

	row := result.Data[0]
	status := row["status"].(string)

	if status == qrcode.StatusDisabled {
		WriteJSON(w, http.StatusOK, qrcode.ScanResult{
			Code:      code,
			Valid:     false,
			Error:     "QR code is disabled",
			ScannedAt: time.Now(),
		})
		return
	}

	// Get asset if assigned
	var assetData map[string]interface{}
	var assetID int64
	if aid, ok := row["asset_id"].(float64); ok && aid > 0 {
		assetID = int64(aid)
		asset, err := h.XOLU.Get(ctx, "assets", assetID)
		if err == nil {
			assetData = asset.Data
		}
	}

	// Record scan event
	h.recordScan(ctx, code, assetID, r)

	WriteJSON(w, http.StatusOK, qrcode.ScanResult{
		Code:      code,
		Valid:     true,
		AssetID:   assetID,
		Asset:     assetData,
		ScannedAt: time.Now(),
	})
}

// Labels handles GET /api/v1/qr/labels
// Returns label data for printing.
func (h *QRHandler) Labels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	// Get codes from query (comma-separated) or batch_id
	codesParam := GetQueryString(r, "codes", "")
	batchID := GetQueryString(r, "batch_id", "")

	var query string
	if codesParam != "" {
		codes := strings.Split(codesParam, ",")
		quotedCodes := make([]string, len(codes))
		for i, c := range codes {
			quotedCodes[i] = "'" + strings.ToUpper(strings.TrimSpace(c)) + "'"
		}
		query = "SELECT * FROM qr_codes WHERE code IN (" + strings.Join(quotedCodes, ",") + ")"
	} else if batchID != "" {
		query = "SELECT * FROM qr_codes WHERE batch_id = '" + batchID + "'"
	} else {
		WriteError(w, http.StatusBadRequest, "bad_request", "codes or batch_id required")
		return
	}

	result, err := client.OQL(ctx, query)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	labels := make([]*qrcode.Label, 0, len(result.Data))
	for _, row := range result.Data {
		code := row["code"].(string)

		// Get asset info if assigned
		var assetName, assetType string
		if aid, ok := row["asset_id"].(float64); ok && aid > 0 {
			asset, err := client.Get(ctx, "assets", int64(aid))
			if err == nil {
				if name, ok := asset.Data["name"].(string); ok {
					assetName = name
				}
				if typ, ok := asset.Data["asset_type"].(string); ok {
					assetType = typ
				}
			}
		}

		label := h.generator.GenerateLabel(code, assetName, assetType, "")
		labels = append(labels, label)
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"count":  len(labels),
		"labels": labels,
	})
}

// recordScan records a QR code scan event.
func (h *QRHandler) recordScan(ctx context.Context, code string, assetID int64, r *http.Request) {
	record := map[string]interface{}{
		"code":       code,
		"asset_id":   assetID,
		"user_agent": r.UserAgent(),
		"ip_address": getClientIP(r),
		"scanned_at": time.Now(),
	}

	// Try to get location from query params
	if lat := r.URL.Query().Get("lat"); lat != "" {
		if lng := r.URL.Query().Get("lng"); lng != "" {
			latF, _ := strconv.ParseFloat(lat, 64)
			lngF, _ := strconv.ParseFloat(lng, 64)
			record["location"] = map[string]interface{}{
				"latitude":  latF,
				"longitude": lngF,
			}
		}
	}

	h.XOLU.Create(ctx, "qr_scans", record)
}

// Helper functions

func getQRCode(r *http.Request) string {
	path := r.URL.Path
	parts := strings.Split(strings.Trim(path, "/"), "/")

	// Find code after "qr"
	for i, part := range parts {
		if part == "qr" && i+1 < len(parts) {
			return strings.ToUpper(parts[i+1])
		}
	}
	return ""
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	return strings.Split(r.RemoteAddr, ":")[0]
}

// Note: OQL uses T-SQL syntax with TOP n, not LIMIT/OFFSET
// For pagination, results should be fetched and sliced in Go
