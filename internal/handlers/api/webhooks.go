package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ha1tch/seam-ui/internal/webhook"
)

// WebhookHandler handles webhook management endpoints.
type WebhookHandler struct {
	service *webhook.Service
}

// NewWebhookHandler creates a new webhook handler.
func NewWebhookHandler(service *webhook.Service) *WebhookHandler {
	return &WebhookHandler{service: service}
}

// WebhookRequest represents a webhook creation/update request.
type WebhookRequest struct {
	Name       string              `json:"name"`
	URL        string              `json:"url"`
	Secret     string              `json:"secret,omitempty"`
	Events     []webhook.EventType `json:"events"`
	Active     bool                `json:"active"`
	Headers    map[string]string   `json:"headers,omitempty"`
	RetryCount int                 `json:"retry_count,omitempty"`
	Timeout    int                 `json:"timeout_seconds,omitempty"`
}

// List handles GET /api/v1/webhooks
func (h *WebhookHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID := getTenantID(r)
	if tenantID == 0 {
		WriteError(w, http.StatusBadRequest, "bad_request", "tenant context required")
		return
	}

	webhooks := h.service.GetWebhooks(tenantID)

	// Remove secrets from response
	response := make([]map[string]interface{}, len(webhooks))
	for i, wh := range webhooks {
		response[i] = webhookToMap(wh, false)
	}

	WriteJSON(w, http.StatusOK, NewListResponse(response, len(webhooks), len(webhooks), 0))
}

// Create handles POST /api/v1/webhooks
func (h *WebhookHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID := getTenantID(r)
	if tenantID == 0 {
		WriteError(w, http.StatusBadRequest, "bad_request", "tenant context required")
		return
	}

	var req WebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	// Validate
	if req.Name == "" {
		WriteError(w, http.StatusBadRequest, "validation_error", "name is required")
		return
	}
	if req.URL == "" {
		WriteError(w, http.StatusBadRequest, "validation_error", "url is required")
		return
	}
	if len(req.Events) == 0 {
		WriteError(w, http.StatusBadRequest, "validation_error", "at least one event is required")
		return
	}

	// Create webhook
	wh := &webhook.Webhook{
		ID:         time.Now().UnixNano(), // Simple ID generation
		TenantID:   tenantID,
		Name:       req.Name,
		URL:        req.URL,
		Secret:     req.Secret,
		Events:     req.Events,
		Active:     req.Active,
		Headers:    req.Headers,
		RetryCount: req.RetryCount,
		Timeout:    req.Timeout,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	h.service.Register(wh)

	WriteJSON(w, http.StatusCreated, webhookToMap(wh, false))
}

// Get handles GET /api/v1/webhooks/{id}
func (h *WebhookHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := getWebhookID(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", "invalid webhook ID")
		return
	}

	wh, err := h.service.GetWebhook(id)
	if err != nil {
		WriteError(w, http.StatusNotFound, "not_found", "webhook not found")
		return
	}

	// Verify tenant
	tenantID := getTenantID(r)
	if wh.TenantID != tenantID {
		WriteError(w, http.StatusNotFound, "not_found", "webhook not found")
		return
	}

	WriteJSON(w, http.StatusOK, webhookToMap(wh, false))
}

// Update handles PUT /api/v1/webhooks/{id}
func (h *WebhookHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := getWebhookID(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", "invalid webhook ID")
		return
	}

	wh, err := h.service.GetWebhook(id)
	if err != nil {
		WriteError(w, http.StatusNotFound, "not_found", "webhook not found")
		return
	}

	// Verify tenant
	tenantID := getTenantID(r)
	if wh.TenantID != tenantID {
		WriteError(w, http.StatusNotFound, "not_found", "webhook not found")
		return
	}

	var req WebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	// Update fields
	if req.Name != "" {
		wh.Name = req.Name
	}
	if req.URL != "" {
		wh.URL = req.URL
	}
	if req.Secret != "" {
		wh.Secret = req.Secret
	}
	if len(req.Events) > 0 {
		wh.Events = req.Events
	}
	wh.Active = req.Active
	if req.Headers != nil {
		wh.Headers = req.Headers
	}
	if req.RetryCount > 0 {
		wh.RetryCount = req.RetryCount
	}
	if req.Timeout > 0 {
		wh.Timeout = req.Timeout
	}
	wh.UpdatedAt = time.Now()

	// Re-register (update in memory)
	h.service.Register(wh)

	WriteJSON(w, http.StatusOK, webhookToMap(wh, false))
}

// Delete handles DELETE /api/v1/webhooks/{id}
func (h *WebhookHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := getWebhookID(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", "invalid webhook ID")
		return
	}

	wh, err := h.service.GetWebhook(id)
	if err != nil {
		WriteError(w, http.StatusNotFound, "not_found", "webhook not found")
		return
	}

	// Verify tenant
	tenantID := getTenantID(r)
	if wh.TenantID != tenantID {
		WriteError(w, http.StatusNotFound, "not_found", "webhook not found")
		return
	}

	h.service.Unregister(id)

	w.WriteHeader(http.StatusNoContent)
}

// Enable handles POST /api/v1/webhooks/{id}/enable
func (h *WebhookHandler) Enable(w http.ResponseWriter, r *http.Request) {
	h.setActive(w, r, true)
}

// Disable handles POST /api/v1/webhooks/{id}/disable
func (h *WebhookHandler) Disable(w http.ResponseWriter, r *http.Request) {
	h.setActive(w, r, false)
}

func (h *WebhookHandler) setActive(w http.ResponseWriter, r *http.Request, active bool) {
	id, err := getWebhookID(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", "invalid webhook ID")
		return
	}

	wh, err := h.service.GetWebhook(id)
	if err != nil {
		WriteError(w, http.StatusNotFound, "not_found", "webhook not found")
		return
	}

	// Verify tenant
	tenantID := getTenantID(r)
	if wh.TenantID != tenantID {
		WriteError(w, http.StatusNotFound, "not_found", "webhook not found")
		return
	}

	wh.Active = active
	wh.UpdatedAt = time.Now()
	h.service.Register(wh)

	WriteJSON(w, http.StatusOK, webhookToMap(wh, false))
}

// Test handles POST /api/v1/webhooks/{id}/test
func (h *WebhookHandler) Test(w http.ResponseWriter, r *http.Request) {
	id, err := getWebhookID(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", "invalid webhook ID")
		return
	}

	wh, err := h.service.GetWebhook(id)
	if err != nil {
		WriteError(w, http.StatusNotFound, "not_found", "webhook not found")
		return
	}

	// Verify tenant
	tenantID := getTenantID(r)
	if wh.TenantID != tenantID {
		WriteError(w, http.StatusNotFound, "not_found", "webhook not found")
		return
	}

	// Trigger test event
	testData := map[string]interface{}{
		"test":      true,
		"timestamp": time.Now(),
		"message":   "This is a test webhook delivery",
	}

	// Temporarily enable if disabled for test
	wasActive := wh.Active
	wh.Active = true
	h.service.Register(wh)

	err = h.service.Trigger(r.Context(), "webhook.test", tenantID, testData)

	// Restore original state
	wh.Active = wasActive
	h.service.Register(wh)

	if err != nil {
		WriteError(w, http.StatusInternalServerError, "delivery_failed", err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Test webhook queued for delivery",
		"data":    testData,
	})
}

// Helper functions

func getWebhookID(r *http.Request) (int64, error) {
	path := r.URL.Path
	parts := strings.Split(strings.Trim(path, "/"), "/")

	// Find ID after "webhooks"
	for i, part := range parts {
		if part == "webhooks" && i+1 < len(parts) {
			return strconv.ParseInt(parts[i+1], 10, 64)
		}
	}
	return 0, nil
}

func getTenantID(r *http.Request) int64 {
	// From middleware context
	tenantStr := TenantID(r)
	if tenantStr == "" {
		return 0
	}
	id, _ := strconv.ParseInt(tenantStr, 10, 64)
	return id
}

func webhookToMap(wh *webhook.Webhook, includeSecret bool) map[string]interface{} {
	m := map[string]interface{}{
		"id":              wh.ID,
		"tenant_id":       wh.TenantID,
		"name":            wh.Name,
		"url":             wh.URL,
		"events":          wh.Events,
		"active":          wh.Active,
		"headers":         wh.Headers,
		"retry_count":     wh.RetryCount,
		"timeout_seconds": wh.Timeout,
		"created_at":      wh.CreatedAt,
		"updated_at":      wh.UpdatedAt,
	}

	if includeSecret && wh.Secret != "" {
		m["secret"] = wh.Secret
	} else if wh.Secret != "" {
		m["has_secret"] = true
	}

	return m
}
