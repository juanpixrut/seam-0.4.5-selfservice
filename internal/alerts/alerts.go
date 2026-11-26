// Package alerts provides alert management for Seam AMS.
package alerts

import (
	"context"
	"fmt"
	"time"

	xolu "github.com/ha1tch/xolu/pkg/client"
)

// Severity levels
const (
	SeverityLow      = "low"
	SeverityMedium   = "medium"
	SeverityHigh     = "high"
	SeverityCritical = "critical"
)

// Status values
const (
	StatusOpen         = "open"
	StatusAcknowledged = "acknowledged"
	StatusResolved     = "resolved"
	StatusDismissed    = "dismissed"
)

// Alert represents an alert triggered by rules or system events.
type Alert struct {
	ID             int64                  `json:"id"`
	TenantID       int64                  `json:"tenant_id"`
	AssetID        int64                  `json:"asset_id,omitempty"`
	RuleID         int64                  `json:"rule_id,omitempty"`
	EventID        int64                  `json:"event_id,omitempty"`
	Title          string                 `json:"title"`
	Message        string                 `json:"message"`
	Severity       string                 `json:"severity"`
	Status         string                 `json:"status"`
	Source         string                 `json:"source"`
	Data           map[string]interface{} `json:"data,omitempty"`
	AcknowledgedBy int64                  `json:"acknowledged_by,omitempty"`
	AcknowledgedAt time.Time              `json:"acknowledged_at,omitempty"`
	ResolvedBy     int64                  `json:"resolved_by,omitempty"`
	ResolvedAt     time.Time              `json:"resolved_at,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

// CreateAlertRequest represents a request to create an alert.
type CreateAlertRequest struct {
	TenantID int64
	AssetID  int64
	RuleID   int64
	EventID  int64
	Title    string
	Message  string
	Severity string
	Source   string
	Data     map[string]interface{}
}

// Service provides alert management.
type Service struct {
	xoluClient *xolu.Client
}

// NewService creates a new alerts service.
func NewService(client *xolu.Client) *Service {
	return &Service{xoluClient: client}
}

// Create creates a new alert.
func (s *Service) Create(ctx context.Context, req CreateAlertRequest) (*Alert, error) {
	// Validate severity
	if !isValidSeverity(req.Severity) {
		req.Severity = SeverityMedium
	}

	data := map[string]interface{}{
		"tenant_id":  req.TenantID,
		"title":      req.Title,
		"message":    req.Message,
		"severity":   req.Severity,
		"status":     StatusOpen,
		"source":     req.Source,
		"data":       req.Data,
		"created_at": time.Now(),
		"updated_at": time.Now(),
	}

	if req.AssetID > 0 {
		data["asset_id"] = req.AssetID
	}
	if req.RuleID > 0 {
		data["rule_id"] = req.RuleID
	}
	if req.EventID > 0 {
		data["event_id"] = req.EventID
	}

	created, err := s.xoluClient.Create(ctx, "alerts", data)
	if err != nil {
		return nil, fmt.Errorf("creating alert: %w", err)
	}

	// Re-fetch — xolu does not echo the document on creation.
	entity, err := s.xoluClient.Get(ctx, "alerts", created.ID)
	if err != nil {
		return nil, fmt.Errorf("fetching created alert: %w", err)
	}

	return entityToAlert(entity), nil
}

// Get retrieves an alert by ID.
func (s *Service) Get(ctx context.Context, id int64) (*Alert, error) {
	entity, err := s.xoluClient.Get(ctx, "alerts", id)
	if err != nil {
		return nil, fmt.Errorf("getting alert: %w", err)
	}
	return entityToAlert(entity), nil
}

// List retrieves alerts with optional filtering.
func (s *Service) List(ctx context.Context, tenantID int64, status, severity string, assetID int64, limit, offset int) ([]*Alert, int, error) {
	// OQL uses TOP n, not LIMIT/OFFSET
	fetchCount := limit + offset
	query := fmt.Sprintf("SELECT TOP %d * FROM alerts WHERE tenant_id = %d", fetchCount, tenantID)

	if status != "" {
		query += fmt.Sprintf(" AND status = '%s'", status)
	}
	if severity != "" {
		query += fmt.Sprintf(" AND severity = '%s'", severity)
	}
	if assetID > 0 {
		query += fmt.Sprintf(" AND asset_id = %d", assetID)
	}

	query += " ORDER BY created_at DESC"

	result, err := s.xoluClient.OQL(ctx, query)
	if err != nil {
		return nil, 0, fmt.Errorf("listing alerts: %w", err)
	}

	// Apply offset by slicing
	data := result.Data
	if offset > 0 && offset < len(data) {
		data = data[offset:]
	} else if offset >= len(data) {
		data = []map[string]any{}
	}
	if len(data) > limit {
		data = data[:limit]
	}

	alerts := make([]*Alert, len(data))
	for i, row := range data {
		alerts[i] = mapToAlert(row)
	}

	// Get total count
	countQuery := fmt.Sprintf("SELECT COUNT(*) as total FROM alerts WHERE tenant_id = %d", tenantID)
	if status != "" {
		countQuery += fmt.Sprintf(" AND status = '%s'", status)
	}
	if severity != "" {
		countQuery += fmt.Sprintf(" AND severity = '%s'", severity)
	}
	if assetID > 0 {
		countQuery += fmt.Sprintf(" AND asset_id = %d", assetID)
	}

	countResult, err := s.xoluClient.OQL(ctx, countQuery)
	total := len(alerts)
	if err == nil && len(countResult.Data) > 0 {
		if t, ok := countResult.Data[0]["total"].(float64); ok {
			total = int(t)
		}
	}

	return alerts, total, nil
}

// Acknowledge acknowledges an alert.
func (s *Service) Acknowledge(ctx context.Context, id, userID int64) (*Alert, error) {
	updates := map[string]interface{}{
		"status":          StatusAcknowledged,
		"acknowledged_by": userID,
		"acknowledged_at": time.Now(),
		"updated_at":      time.Now(),
	}

	entity, err := s.xoluClient.Update(ctx, "alerts", id, updates)
	if err != nil {
		return nil, fmt.Errorf("acknowledging alert: %w", err)
	}

	return entityToAlert(entity), nil
}

// Resolve resolves an alert.
func (s *Service) Resolve(ctx context.Context, id, userID int64) (*Alert, error) {
	updates := map[string]interface{}{
		"status":      StatusResolved,
		"resolved_by": userID,
		"resolved_at": time.Now(),
		"updated_at":  time.Now(),
	}

	entity, err := s.xoluClient.Update(ctx, "alerts", id, updates)
	if err != nil {
		return nil, fmt.Errorf("resolving alert: %w", err)
	}

	return entityToAlert(entity), nil
}

// Dismiss dismisses an alert.
func (s *Service) Dismiss(ctx context.Context, id int64) (*Alert, error) {
	updates := map[string]interface{}{
		"status":     StatusDismissed,
		"updated_at": time.Now(),
	}

	entity, err := s.xoluClient.Update(ctx, "alerts", id, updates)
	if err != nil {
		return nil, fmt.Errorf("dismissing alert: %w", err)
	}

	return entityToAlert(entity), nil
}

// GetOpenCount returns the count of open alerts for a tenant.
func (s *Service) GetOpenCount(ctx context.Context, tenantID int64) (int, error) {
	query := fmt.Sprintf("SELECT COUNT(*) as total FROM alerts WHERE tenant_id = %d AND status = 'open'", tenantID)

	result, err := s.xoluClient.OQL(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("counting alerts: %w", err)
	}

	if len(result.Data) > 0 {
		if total, ok := result.Data[0]["total"].(float64); ok {
			return int(total), nil
		}
	}

	return 0, nil
}

// GetBySeverity returns counts by severity for a tenant.
func (s *Service) GetBySeverity(ctx context.Context, tenantID int64) (map[string]int, error) {
	query := fmt.Sprintf("SELECT severity, COUNT(*) as count FROM alerts WHERE tenant_id = %d AND status = 'open' GROUP BY severity", tenantID)

	result, err := s.xoluClient.OQL(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("counting alerts by severity: %w", err)
	}

	counts := make(map[string]int)
	for _, row := range result.Data {
		if sev, ok := row["severity"].(string); ok {
			if count, ok := row["count"].(float64); ok {
				counts[sev] = int(count)
			}
		}
	}

	return counts, nil
}

// Helper functions

func isValidSeverity(s string) bool {
	switch s {
	case SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical:
		return true
	}
	return false
}

func entityToAlert(entity *xolu.Entity) *Alert {
	return mapToAlert(entity.Data)
}

func mapToAlert(data map[string]interface{}) *Alert {
	alert := &Alert{
		Data: make(map[string]interface{}),
	}

	if v, ok := data["id"].(float64); ok {
		alert.ID = int64(v)
	}
	if v, ok := data["tenant_id"].(float64); ok {
		alert.TenantID = int64(v)
	}
	if v, ok := data["asset_id"].(float64); ok {
		alert.AssetID = int64(v)
	}
	if v, ok := data["rule_id"].(float64); ok {
		alert.RuleID = int64(v)
	}
	if v, ok := data["event_id"].(float64); ok {
		alert.EventID = int64(v)
	}
	if v, ok := data["title"].(string); ok {
		alert.Title = v
	}
	if v, ok := data["message"].(string); ok {
		alert.Message = v
	}
	if v, ok := data["severity"].(string); ok {
		alert.Severity = v
	}
	if v, ok := data["status"].(string); ok {
		alert.Status = v
	}
	if v, ok := data["source"].(string); ok {
		alert.Source = v
	}
	if v, ok := data["data"].(map[string]interface{}); ok {
		alert.Data = v
	}
	if v, ok := data["acknowledged_by"].(float64); ok {
		alert.AcknowledgedBy = int64(v)
	}
	if v, ok := data["resolved_by"].(float64); ok {
		alert.ResolvedBy = int64(v)
	}
	if v, ok := data["acknowledged_at"].(string); ok {
		alert.AcknowledgedAt, _ = time.Parse(time.RFC3339, v)
	}
	if v, ok := data["resolved_at"].(string); ok {
		alert.ResolvedAt, _ = time.Parse(time.RFC3339, v)
	}
	if v, ok := data["created_at"].(string); ok {
		alert.CreatedAt, _ = time.Parse(time.RFC3339, v)
	}
	if v, ok := data["updated_at"].(string); ok {
		alert.UpdatedAt, _ = time.Parse(time.RFC3339, v)
	}

	return alert
}
