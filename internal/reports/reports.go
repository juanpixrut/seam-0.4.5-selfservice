// Package reports provides reporting and analytics for Seam AMS.
package reports

import (
	"context"
	"fmt"
	"time"

	xolu "github.com/ha1tch/xolu/pkg/client"
)

// TimeRange represents a time range for reports.
type TimeRange struct {
	From time.Time
	To   time.Time
}

// Preset time ranges
func Today() TimeRange {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return TimeRange{From: start, To: now}
}

func Yesterday() TimeRange {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, now.Location())
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return TimeRange{From: start, To: end}
}

func ThisWeek() TimeRange {
	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	start := time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
	return TimeRange{From: start, To: now}
}

func ThisMonth() TimeRange {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	return TimeRange{From: start, To: now}
}

func Last30Days() TimeRange {
	now := time.Now()
	start := now.AddDate(0, 0, -30)
	return TimeRange{From: start, To: now}
}

func Last90Days() TimeRange {
	now := time.Now()
	start := now.AddDate(0, 0, -90)
	return TimeRange{From: start, To: now}
}

// DashboardStats represents dashboard statistics.
type DashboardStats struct {
	TotalAssets    int            `json:"total_assets"`
	ActiveAssets   int            `json:"active_assets"`
	TotalEvents    int            `json:"total_events"`
	EventsToday    int            `json:"events_today"`
	AlertsOpen     int            `json:"alerts_open"`
	RulesTriggered int            `json:"rules_triggered"`
	AssetsByStatus map[string]int `json:"assets_by_status"`
	AssetsByType   map[string]int `json:"assets_by_type"`
	RecentActivity []Activity     `json:"recent_activity"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// Activity represents a recent activity item.
type Activity struct {
	Type        string    `json:"type"`
	Description string    `json:"description"`
	AssetID     int64     `json:"asset_id,omitempty"`
	AssetName   string    `json:"asset_name,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// AssetReport represents an asset report.
type AssetReport struct {
	TotalAssets     int               `json:"total_assets"`
	ByStatus        map[string]int    `json:"by_status"`
	ByType          map[string]int    `json:"by_type"`
	ByLocation      map[string]int    `json:"by_location"`
	CreatedOverTime []TimeSeriesPoint `json:"created_over_time"`
	StatusChanges   []StatusChange    `json:"status_changes"`
	TimeRange       TimeRange         `json:"time_range"`
	GeneratedAt     time.Time         `json:"generated_at"`
}

// EventReport represents an event report.
type EventReport struct {
	TotalEvents    int               `json:"total_events"`
	ByTriggerType  map[string]int    `json:"by_trigger_type"`
	ByAsset        map[int64]int     `json:"by_asset"`
	EventsOverTime []TimeSeriesPoint `json:"events_over_time"`
	PeakHours      map[int]int       `json:"peak_hours"`
	AveragePerDay  float64           `json:"average_per_day"`
	TimeRange      TimeRange         `json:"time_range"`
	GeneratedAt    time.Time         `json:"generated_at"`
}

// TimeSeriesPoint represents a point in a time series.
type TimeSeriesPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     int       `json:"value"`
	Label     string    `json:"label,omitempty"`
}

// StatusChange represents a status change event.
type StatusChange struct {
	AssetID    int64     `json:"asset_id"`
	AssetName  string    `json:"asset_name"`
	FromStatus string    `json:"from_status"`
	ToStatus   string    `json:"to_status"`
	Timestamp  time.Time `json:"timestamp"`
}

// Service provides reporting functionality.
type Service struct {
	xoluClient *xolu.Client
}

// NewService creates a new reports service.
func NewService(client *xolu.Client) *Service {
	return &Service{xoluClient: client}
}

// GetDashboardStats retrieves dashboard statistics.
func (s *Service) GetDashboardStats(ctx context.Context, tenantID int64) (*DashboardStats, error) {
	stats := &DashboardStats{
		AssetsByStatus: make(map[string]int),
		AssetsByType:   make(map[string]int),
		UpdatedAt:      time.Now(),
	}

	// Total assets
	result, err := s.xoluClient.OQL(ctx, fmt.Sprintf("SELECT COUNT(*) as total FROM assets WHERE tenant_id = %d", tenantID))
	if err == nil && len(result.Data) > 0 {
		if total, ok := result.Data[0]["total"].(float64); ok {
			stats.TotalAssets = int(total)
		}
	}

	// Active assets
	result, err = s.xoluClient.OQL(ctx, fmt.Sprintf("SELECT COUNT(*) as total FROM assets WHERE tenant_id = %d AND status = 'active'", tenantID))
	if err == nil && len(result.Data) > 0 {
		if total, ok := result.Data[0]["total"].(float64); ok {
			stats.ActiveAssets = int(total)
		}
	}

	// Events today
	today := time.Now().Format("2006-01-02")
	result, err = s.xoluClient.OQL(ctx, fmt.Sprintf("SELECT COUNT(*) as total FROM events WHERE tenant_id = %d AND timestamp >= '%s'", tenantID, today))
	if err == nil && len(result.Data) > 0 {
		if total, ok := result.Data[0]["total"].(float64); ok {
			stats.EventsToday = int(total)
		}
	}

	// Total events
	result, err = s.xoluClient.OQL(ctx, fmt.Sprintf("SELECT COUNT(*) as total FROM events WHERE tenant_id = %d", tenantID))
	if err == nil && len(result.Data) > 0 {
		if total, ok := result.Data[0]["total"].(float64); ok {
			stats.TotalEvents = int(total)
		}
	}

	// Open alerts
	result, err = s.xoluClient.OQL(ctx, fmt.Sprintf("SELECT COUNT(*) as total FROM alerts WHERE tenant_id = %d AND status = 'open'", tenantID))
	if err == nil && len(result.Data) > 0 {
		if total, ok := result.Data[0]["total"].(float64); ok {
			stats.AlertsOpen = int(total)
		}
	}

	// Assets by status
	result, err = s.xoluClient.OQL(ctx, fmt.Sprintf("SELECT status, COUNT(*) as count FROM assets WHERE tenant_id = %d GROUP BY status", tenantID))
	if err == nil {
		for _, row := range result.Data {
			if status, ok := row["status"].(string); ok {
				if count, ok := row["count"].(float64); ok {
					stats.AssetsByStatus[status] = int(count)
				}
			}
		}
	}

	// Assets by type
	result, err = s.xoluClient.OQL(ctx, fmt.Sprintf("SELECT asset_type, COUNT(*) as count FROM assets WHERE tenant_id = %d GROUP BY asset_type", tenantID))
	if err == nil {
		for _, row := range result.Data {
			if assetType, ok := row["asset_type"].(string); ok {
				if count, ok := row["count"].(float64); ok {
					stats.AssetsByType[assetType] = int(count)
				}
			}
		}
	}

	// Recent activity
	result, err = s.xoluClient.OQL(ctx, fmt.Sprintf("SELECT TOP 10 * FROM events WHERE tenant_id = %d ORDER BY timestamp DESC", tenantID))
	if err == nil {
		for _, row := range result.Data {
			activity := Activity{
				Type: "event",
			}
			if triggerType, ok := row["trigger_type"].(string); ok {
				activity.Description = triggerType + " event"
			}
			if assetID, ok := row["asset_id"].(float64); ok {
				activity.AssetID = int64(assetID)
			}
			if ts, ok := row["timestamp"].(string); ok {
				activity.Timestamp, _ = time.Parse(time.RFC3339, ts)
			}
			stats.RecentActivity = append(stats.RecentActivity, activity)
		}
	}

	return stats, nil
}

// GetAssetReport generates an asset report.
func (s *Service) GetAssetReport(ctx context.Context, tenantID int64, tr TimeRange) (*AssetReport, error) {
	report := &AssetReport{
		ByStatus:    make(map[string]int),
		ByType:      make(map[string]int),
		ByLocation:  make(map[string]int),
		TimeRange:   tr,
		GeneratedAt: time.Now(),
	}

	// Total assets
	result, err := s.xoluClient.OQL(ctx, fmt.Sprintf("SELECT COUNT(*) as total FROM assets WHERE tenant_id = %d", tenantID))
	if err == nil && len(result.Data) > 0 {
		if total, ok := result.Data[0]["total"].(float64); ok {
			report.TotalAssets = int(total)
		}
	}

	// By status
	result, err = s.xoluClient.OQL(ctx, fmt.Sprintf("SELECT status, COUNT(*) as count FROM assets WHERE tenant_id = %d GROUP BY status", tenantID))
	if err == nil {
		for _, row := range result.Data {
			if status, ok := row["status"].(string); ok {
				if count, ok := row["count"].(float64); ok {
					report.ByStatus[status] = int(count)
				}
			}
		}
	}

	// By type
	result, err = s.xoluClient.OQL(ctx, fmt.Sprintf("SELECT asset_type, COUNT(*) as count FROM assets WHERE tenant_id = %d GROUP BY asset_type", tenantID))
	if err == nil {
		for _, row := range result.Data {
			if assetType, ok := row["asset_type"].(string); ok {
				if count, ok := row["count"].(float64); ok {
					report.ByType[assetType] = int(count)
				}
			}
		}
	}

	// Assets created over time (daily for last 30 days)
	report.CreatedOverTime = s.getAssetsCreatedOverTime(ctx, tenantID, tr)

	return report, nil
}

// GetEventReport generates an event report.
func (s *Service) GetEventReport(ctx context.Context, tenantID int64, tr TimeRange) (*EventReport, error) {
	report := &EventReport{
		ByTriggerType: make(map[string]int),
		ByAsset:       make(map[int64]int),
		PeakHours:     make(map[int]int),
		TimeRange:     tr,
		GeneratedAt:   time.Now(),
	}

	fromStr := tr.From.Format(time.RFC3339)
	toStr := tr.To.Format(time.RFC3339)

	// Total events in range
	query := fmt.Sprintf("SELECT COUNT(*) as total FROM events WHERE tenant_id = %d AND timestamp >= '%s' AND timestamp <= '%s'",
		tenantID, fromStr, toStr)
	result, err := s.xoluClient.OQL(ctx, query)
	if err == nil && len(result.Data) > 0 {
		if total, ok := result.Data[0]["total"].(float64); ok {
			report.TotalEvents = int(total)
		}
	}

	// By trigger type
	query = fmt.Sprintf("SELECT trigger_type, COUNT(*) as count FROM events WHERE tenant_id = %d AND timestamp >= '%s' AND timestamp <= '%s' GROUP BY trigger_type",
		tenantID, fromStr, toStr)
	result, err = s.xoluClient.OQL(ctx, query)
	if err == nil {
		for _, row := range result.Data {
			if triggerType, ok := row["trigger_type"].(string); ok {
				if count, ok := row["count"].(float64); ok {
					report.ByTriggerType[triggerType] = int(count)
				}
			}
		}
	}

	// Events over time
	report.EventsOverTime = s.getEventsOverTime(ctx, tenantID, tr)

	// Calculate average per day
	days := tr.To.Sub(tr.From).Hours() / 24
	if days > 0 {
		report.AveragePerDay = float64(report.TotalEvents) / days
	}

	return report, nil
}

// Helper functions

func (s *Service) getAssetsCreatedOverTime(ctx context.Context, tenantID int64, tr TimeRange) []TimeSeriesPoint {
	var points []TimeSeriesPoint

	// Generate daily points
	current := tr.From
	for current.Before(tr.To) {
		next := current.AddDate(0, 0, 1)

		query := fmt.Sprintf("SELECT COUNT(*) as count FROM assets WHERE tenant_id = %d AND created_at >= '%s' AND created_at < '%s'",
			tenantID, current.Format(time.RFC3339), next.Format(time.RFC3339))

		result, err := s.xoluClient.OQL(ctx, query)
		count := 0
		if err == nil && len(result.Data) > 0 {
			if c, ok := result.Data[0]["count"].(float64); ok {
				count = int(c)
			}
		}

		points = append(points, TimeSeriesPoint{
			Timestamp: current,
			Value:     count,
			Label:     current.Format("2006-01-02"),
		})

		current = next
	}

	return points
}

func (s *Service) getEventsOverTime(ctx context.Context, tenantID int64, tr TimeRange) []TimeSeriesPoint {
	var points []TimeSeriesPoint

	// Determine granularity based on range
	duration := tr.To.Sub(tr.From)
	var interval time.Duration
	if duration <= 24*time.Hour {
		interval = time.Hour
	} else if duration <= 7*24*time.Hour {
		interval = 6 * time.Hour
	} else {
		interval = 24 * time.Hour
	}

	current := tr.From
	for current.Before(tr.To) {
		next := current.Add(interval)

		query := fmt.Sprintf("SELECT COUNT(*) as count FROM events WHERE tenant_id = %d AND timestamp >= '%s' AND timestamp < '%s'",
			tenantID, current.Format(time.RFC3339), next.Format(time.RFC3339))

		result, err := s.xoluClient.OQL(ctx, query)
		count := 0
		if err == nil && len(result.Data) > 0 {
			if c, ok := result.Data[0]["count"].(float64); ok {
				count = int(c)
			}
		}

		points = append(points, TimeSeriesPoint{
			Timestamp: current,
			Value:     count,
		})

		current = next
	}

	return points
}

// ExportFormat represents export format options.
type ExportFormat string

const (
	FormatJSON ExportFormat = "json"
	FormatCSV  ExportFormat = "csv"
)

// ExportAssets exports assets in the specified format.
func (s *Service) ExportAssets(ctx context.Context, tenantID int64, format ExportFormat) ([]byte, error) {
	query := fmt.Sprintf("SELECT * FROM assets WHERE tenant_id = %d ORDER BY name", tenantID)
	result, err := s.xoluClient.OQL(ctx, query)
	if err != nil {
		return nil, err
	}

	switch format {
	case FormatCSV:
		return s.toCSV(result.Data, []string{"id", "name", "asset_type", "status", "location", "created_at"})
	default:
		return s.toJSON(result.Data)
	}
}

// ExportEvents exports events in the specified format.
func (s *Service) ExportEvents(ctx context.Context, tenantID int64, tr TimeRange, format ExportFormat) ([]byte, error) {
	query := fmt.Sprintf("SELECT TOP 10 * FROM events WHERE tenant_id = %d AND timestamp >= '%s' AND timestamp <= '%s' ORDER BY timestamp DESC",
		tenantID, tr.From.Format(time.RFC3339), tr.To.Format(time.RFC3339))
	result, err := s.xoluClient.OQL(ctx, query)
	if err != nil {
		return nil, err
	}

	switch format {
	case FormatCSV:
		return s.toCSV(result.Data, []string{"id", "asset_id", "trigger_type", "timestamp"})
	default:
		return s.toJSON(result.Data)
	}
}

func (s *Service) toJSON(data []map[string]interface{}) ([]byte, error) {
	// Simple JSON encoding
	result := "["
	for i, row := range data {
		if i > 0 {
			result += ","
		}
		result += "{"
		first := true
		for k, v := range row {
			if !first {
				result += ","
			}
			first = false
			result += fmt.Sprintf(`"%s":`, k)
			switch val := v.(type) {
			case string:
				result += fmt.Sprintf(`"%s"`, val)
			case float64:
				result += fmt.Sprintf("%v", val)
			case bool:
				result += fmt.Sprintf("%v", val)
			default:
				result += "null"
			}
		}
		result += "}"
	}
	result += "]"
	return []byte(result), nil
}

func (s *Service) toCSV(data []map[string]interface{}, columns []string) ([]byte, error) {
	// Header
	result := ""
	for i, col := range columns {
		if i > 0 {
			result += ","
		}
		result += col
	}
	result += "\n"

	// Rows
	for _, row := range data {
		for i, col := range columns {
			if i > 0 {
				result += ","
			}
			if val, ok := row[col]; ok {
				switch v := val.(type) {
				case string:
					// Escape quotes and wrap in quotes if contains comma
					if containsSpecial(v) {
						result += `"` + escapeCSV(v) + `"`
					} else {
						result += v
					}
				case float64:
					result += fmt.Sprintf("%v", v)
				case bool:
					result += fmt.Sprintf("%v", v)
				}
			}
		}
		result += "\n"
	}

	return []byte(result), nil
}

func containsSpecial(s string) bool {
	for _, c := range s {
		if c == ',' || c == '"' || c == '\n' {
			return true
		}
	}
	return false
}

func escapeCSV(s string) string {
	result := ""
	for _, c := range s {
		if c == '"' {
			result += `""`
		} else {
			result += string(c)
		}
	}
	return result
}
