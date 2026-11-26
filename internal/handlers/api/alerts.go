package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/ha1tch/seam-ui/internal/alerts"
)

// AlertsHandler handles alert management endpoints.
type AlertsHandler struct {
	*Handler
	alertsSvc *alerts.Service
}

// NewAlertsHandler creates a new alerts handler.
func NewAlertsHandler(h *Handler, svc *alerts.Service) *AlertsHandler {
	return &AlertsHandler{
		Handler:   h,
		alertsSvc: svc,
	}
}

// List handles GET /api/v1/alerts
func (h *AlertsHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := getTenantID(r)

	params := GetPaginationParams(r)
	status := GetQueryString(r, "status", "")
	severity := GetQueryString(r, "severity", "")
	assetID := GetQueryInt(r, "asset_id", 0)

	alertsList, total, err := h.alertsSvc.List(ctx, tenantID, status, severity, int64(assetID), params.Limit, params.Offset)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "list_error", err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, NewListResponse(alertsList, total, params.Limit, params.Offset))
}

// Get handles GET /api/v1/alerts/{id}
func (h *AlertsHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := getAlertID(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", "invalid alert ID")
		return
	}

	alert, err := h.alertsSvc.Get(ctx, id)
	if err != nil {
		WriteError(w, http.StatusNotFound, "not_found", "alert not found")
		return
	}

	// Verify tenant
	tenantID := getTenantID(r)
	if alert.TenantID != tenantID {
		WriteError(w, http.StatusNotFound, "not_found", "alert not found")
		return
	}

	WriteJSON(w, http.StatusOK, alert)
}

// Create handles POST /api/v1/alerts
func (h *AlertsHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := getTenantID(r)

	var req struct {
		AssetID  int64                  `json:"asset_id,omitempty"`
		Title    string                 `json:"title"`
		Message  string                 `json:"message"`
		Severity string                 `json:"severity"`
		Source   string                 `json:"source"`
		Data     map[string]interface{} `json:"data,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	if req.Title == "" {
		WriteError(w, http.StatusBadRequest, "validation_error", "title is required")
		return
	}

	alert, err := h.alertsSvc.Create(ctx, alerts.CreateAlertRequest{
		TenantID: tenantID,
		AssetID:  req.AssetID,
		Title:    req.Title,
		Message:  req.Message,
		Severity: req.Severity,
		Source:   req.Source,
		Data:     req.Data,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "create_error", err.Error())
		return
	}

	WriteJSON(w, http.StatusCreated, alert)
}

// Acknowledge handles POST /api/v1/alerts/{id}/acknowledge
func (h *AlertsHandler) Acknowledge(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := getAlertID(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", "invalid alert ID")
		return
	}

	// Get user ID from auth context (simplified)
	userID := int64(1) // In production, get from auth middleware

	alert, err := h.alertsSvc.Acknowledge(ctx, id, userID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "acknowledge_error", err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, alert)
}

// Resolve handles POST /api/v1/alerts/{id}/resolve
func (h *AlertsHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := getAlertID(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", "invalid alert ID")
		return
	}

	// Get user ID from auth context (simplified)
	userID := int64(1) // In production, get from auth middleware

	alert, err := h.alertsSvc.Resolve(ctx, id, userID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "resolve_error", err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, alert)
}

// Dismiss handles POST /api/v1/alerts/{id}/dismiss
func (h *AlertsHandler) Dismiss(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := getAlertID(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", "invalid alert ID")
		return
	}

	alert, err := h.alertsSvc.Dismiss(ctx, id)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "dismiss_error", err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, alert)
}

// Summary handles GET /api/v1/alerts/summary
func (h *AlertsHandler) Summary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := getTenantID(r)

	openCount, err := h.alertsSvc.GetOpenCount(ctx, tenantID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "summary_error", err.Error())
		return
	}

	bySeverity, err := h.alertsSvc.GetBySeverity(ctx, tenantID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "summary_error", err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"open_count":  openCount,
		"by_severity": bySeverity,
	})
}

// Helper functions

func getAlertID(r *http.Request) (int64, error) {
	path := r.URL.Path
	parts := strings.Split(strings.Trim(path, "/"), "/")

	// Find ID after "alerts"
	for i, part := range parts {
		if part == "alerts" && i+1 < len(parts) {
			return strconv.ParseInt(parts[i+1], 10, 64)
		}
	}
	return 0, nil
}
