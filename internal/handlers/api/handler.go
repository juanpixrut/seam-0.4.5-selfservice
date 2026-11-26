// Package api provides HTTP handlers for the Seam AMS REST API.
//
// The API follows RESTful conventions:
//   - GET /api/v1/{resource} - List resources
//   - POST /api/v1/{resource} - Create resource
//   - GET /api/v1/{resource}/{id} - Get resource
//   - PUT /api/v1/{resource}/{id} - Update resource (full)
//   - PATCH /api/v1/{resource}/{id} - Update resource (partial)
//   - DELETE /api/v1/{resource}/{id} - Delete resource
//
// All responses are JSON. Errors follow the ErrorResponse format.
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/ha1tch/seam-ui/internal/handlers/middleware"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

// Handler is the base handler with shared dependencies.
type Handler struct {
	XOLU *xolu.Client
}

// NewHandler creates a new base handler.
func NewHandler(xoluClient *xolu.Client) *Handler {
	return &Handler{XOLU: xoluClient}
}

// Response helpers

// WriteJSON writes a JSON response.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	middleware.WriteJSON(w, status, data)
}

// WriteError writes a JSON error response.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	middleware.WriteError(w, status, code, message)
}

// WriteErrorWithDetails writes a JSON error response with details.
func WriteErrorWithDetails(w http.ResponseWriter, status int, code, message string, details any) {
	middleware.WriteErrorWithDetails(w, status, code, message, details)
}

// WriteValidationError writes a validation error response.
func WriteValidationError(w http.ResponseWriter, err error) {
	middleware.WriteErrorWithDetails(w, http.StatusUnprocessableEntity, "validation_error", "Validation failed", err.Error())
}

// WriteXOLUError writes an error response from an xolu error.
func WriteXOLUError(w http.ResponseWriter, err error) {
	if xoluErr, ok := err.(*xolu.Error); ok {
		status := xoluErr.StatusCode
		if status == 0 {
			status = http.StatusInternalServerError
		}
		WriteError(w, status, "olu_error", xoluErr.Message)
		return
	}
	WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
}

// Request helpers

// ReadJSON reads and decodes JSON from the request body.
func ReadJSON(r *http.Request, v any) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
	if err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}
	defer r.Body.Close()

	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	return nil
}

// ParseID extracts and parses an integer ID from a URL path parameter.
func ParseID(r *http.Request, paramName string) (int64, error) {
	// Chi stores path params in the URL path pattern
	// For chi router, use chi.URLParam(r, paramName)
	// For simple extraction from path, we parse manually
	path := r.URL.Path
	parts := strings.Split(strings.Trim(path, "/"), "/")

	// Look for the parameter by position or name
	for i, part := range parts {
		// If previous part matches the resource name (e.g., "assets"), current is the ID
		if i > 0 && !strings.Contains(parts[i-1], "{") {
			if _, err := strconv.ParseInt(part, 10, 64); err == nil {
				return strconv.ParseInt(part, 10, 64)
			}
		}
	}

	return 0, fmt.Errorf("ID parameter not found")
}

// GetIDFromPath extracts an ID from a specific position in the URL path.
// Position is 0-indexed from the path segments.
func GetIDFromPath(r *http.Request, position int) (int64, error) {
	path := r.URL.Path
	parts := strings.Split(strings.Trim(path, "/"), "/")

	if position < 0 || position >= len(parts) {
		return 0, fmt.Errorf("invalid path position")
	}

	id, err := strconv.ParseInt(parts[position], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid ID: %w", err)
	}

	return id, nil
}

// GetStringFromPath extracts a string from a specific position in the URL path.
func GetStringFromPath(r *http.Request, position int) (string, error) {
	path := r.URL.Path
	parts := strings.Split(strings.Trim(path, "/"), "/")

	if position < 0 || position >= len(parts) {
		return "", fmt.Errorf("invalid path position")
	}

	return parts[position], nil
}

// Pagination helpers

// PaginationParams holds pagination parameters.
type PaginationParams struct {
	Limit  int
	Offset int
	Sort   string
}

// DefaultLimit is the default number of items per page.
const DefaultLimit = 20

// MaxLimit is the maximum number of items per page.
const MaxLimit = 100

// GetPaginationParams extracts pagination parameters from the request.
func GetPaginationParams(r *http.Request) PaginationParams {
	params := PaginationParams{
		Limit: DefaultLimit,
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			if limit > MaxLimit {
				limit = MaxLimit
			}
			params.Limit = limit
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			params.Offset = offset
		}
	}

	if sort := r.URL.Query().Get("sort"); sort != "" {
		params.Sort = sort
	}

	return params
}

// ListResponse wraps a list of items with pagination metadata.
type ListResponse struct {
	Data any      `json:"data"`
	Meta ListMeta `json:"meta"`
}

// ListMeta contains pagination metadata.
type ListMeta struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// NewListResponse creates a list response with metadata.
func NewListResponse(data any, total, limit, offset int) ListResponse {
	return ListResponse{
		Data: data,
		Meta: ListMeta{
			Total:  total,
			Limit:  limit,
			Offset: offset,
		},
	}
}

// Query string helpers

// GetQueryString gets a query string parameter with a default value.
func GetQueryString(r *http.Request, key, defaultValue string) string {
	if v := r.URL.Query().Get(key); v != "" {
		return v
	}
	return defaultValue
}

// GetQueryInt gets a query integer parameter with a default value.
func GetQueryInt(r *http.Request, key string, defaultValue int) int {
	if v := r.URL.Query().Get(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultValue
}

// GetQueryBool gets a query boolean parameter with a default value.
func GetQueryBool(r *http.Request, key string, defaultValue bool) bool {
	v := r.URL.Query().Get(key)
	if v == "" {
		return defaultValue
	}
	return v == "true" || v == "1" || v == "yes"
}

// GetQueryStrings gets all values for a query parameter (for arrays).
func GetQueryStrings(r *http.Request, key string) []string {
	return r.URL.Query()[key]
}

// Context helpers

// TenantID returns the tenant ID from the request context.
func TenantID(r *http.Request) string {
	return middleware.TenantFromContext(r.Context())
}

// Locale returns the locale from the request context.
func Locale(r *http.Request) string {
	return middleware.LocaleFromContext(r.Context())
}

// XOLU client helpers

// XOLUWithTenant returns an xolu client configured for the request's tenant.
func (h *Handler) XOLUWithTenant(r *http.Request) *xolu.Client {
	tenantID := TenantID(r)
	if tenantID == "" {
		return h.XOLU
	}
	return h.XOLU.WithTenantContext(tenantID)
}
