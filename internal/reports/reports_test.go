package reports

import (
	"testing"
	"time"
)

func TestTimeRanges(t *testing.T) {
	// Test Today
	today := Today()
	if today.From.After(today.To) {
		t.Error("Today: From should be before To")
	}
	if today.From.Day() != time.Now().Day() {
		t.Error("Today: From should be today")
	}

	// Test Yesterday
	yesterday := Yesterday()
	if yesterday.From.After(yesterday.To) {
		t.Error("Yesterday: From should be before To")
	}
	expectedYesterday := time.Now().AddDate(0, 0, -1)
	if yesterday.From.Day() != expectedYesterday.Day() {
		t.Error("Yesterday: From should be yesterday")
	}

	// Test ThisWeek
	week := ThisWeek()
	if week.From.After(week.To) {
		t.Error("ThisWeek: From should be before To")
	}

	// Test ThisMonth
	month := ThisMonth()
	if month.From.After(month.To) {
		t.Error("ThisMonth: From should be before To")
	}
	if month.From.Day() != 1 {
		t.Error("ThisMonth: From should be first of month")
	}

	// Test Last30Days
	last30 := Last30Days()
	if last30.From.After(last30.To) {
		t.Error("Last30Days: From should be before To")
	}
	duration := last30.To.Sub(last30.From)
	if duration < 29*24*time.Hour || duration > 31*24*time.Hour {
		t.Errorf("Last30Days: Duration should be ~30 days, got %v", duration)
	}

	// Test Last90Days
	last90 := Last90Days()
	if last90.From.After(last90.To) {
		t.Error("Last90Days: From should be before To")
	}
	duration = last90.To.Sub(last90.From)
	if duration < 89*24*time.Hour || duration > 91*24*time.Hour {
		t.Errorf("Last90Days: Duration should be ~90 days, got %v", duration)
	}
}

func TestExportFormats(t *testing.T) {
	if FormatJSON != "json" {
		t.Errorf("FormatJSON = %q, want %q", FormatJSON, "json")
	}
	if FormatCSV != "csv" {
		t.Errorf("FormatCSV = %q, want %q", FormatCSV, "csv")
	}
}

func TestContainsSpecial(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"hello", false},
		{"hello,world", true},
		{`hello"world`, true},
		{"hello\nworld", true},
		{"plain text", false},
		{"", false},
	}

	for _, tt := range tests {
		got := containsSpecial(tt.input)
		if got != tt.want {
			t.Errorf("containsSpecial(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestEscapeCSV(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{`hello"world`, `hello""world`},
		{`a"b"c`, `a""b""c`},
		{"no quotes", "no quotes"},
	}

	for _, tt := range tests {
		got := escapeCSV(tt.input)
		if got != tt.want {
			t.Errorf("escapeCSV(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestToCSV(t *testing.T) {
	svc := &Service{}

	data := []map[string]interface{}{
		{"id": float64(1), "name": "Asset 1", "status": "active"},
		{"id": float64(2), "name": "Asset 2", "status": "inactive"},
	}

	columns := []string{"id", "name", "status"}

	result, err := svc.toCSV(data, columns)
	if err != nil {
		t.Fatalf("toCSV error: %v", err)
	}

	csv := string(result)

	// Check header
	if csv[:len("id,name,status")] != "id,name,status" {
		t.Errorf("CSV header incorrect, got: %s", csv[:20])
	}

	// Check rows present
	if len(csv) < 50 {
		t.Error("CSV output too short")
	}
}

func TestToJSON(t *testing.T) {
	svc := &Service{}

	data := []map[string]interface{}{
		{"id": float64(1), "name": "Test"},
	}

	result, err := svc.toJSON(data)
	if err != nil {
		t.Fatalf("toJSON error: %v", err)
	}

	json := string(result)

	// Should be valid JSON array
	if json[0] != '[' || json[len(json)-1] != ']' {
		t.Error("toJSON should return JSON array")
	}
}

func TestDashboardStatsStructure(t *testing.T) {
	stats := &DashboardStats{
		TotalAssets:    100,
		ActiveAssets:   80,
		TotalEvents:    1000,
		EventsToday:    50,
		AlertsOpen:     5,
		AssetsByStatus: map[string]int{"active": 80, "inactive": 20},
		AssetsByType:   map[string]int{"equipment": 60, "vehicle": 40},
		UpdatedAt:      time.Now(),
	}

	if stats.TotalAssets != 100 {
		t.Errorf("TotalAssets = %d, want 100", stats.TotalAssets)
	}
	if stats.AssetsByStatus["active"] != 80 {
		t.Errorf("AssetsByStatus[active] = %d, want 80", stats.AssetsByStatus["active"])
	}
}

func TestTimeSeriesPoint(t *testing.T) {
	point := TimeSeriesPoint{
		Timestamp: time.Now(),
		Value:     42,
		Label:     "2024-01-01",
	}

	if point.Value != 42 {
		t.Errorf("Value = %d, want 42", point.Value)
	}
	if point.Label != "2024-01-01" {
		t.Errorf("Label = %q, want %q", point.Label, "2024-01-01")
	}
}

func TestActivityStructure(t *testing.T) {
	activity := Activity{
		Type:        "event",
		Description: "scan event",
		AssetID:     123,
		AssetName:   "Test Asset",
		Timestamp:   time.Now(),
	}

	if activity.Type != "event" {
		t.Errorf("Type = %q, want %q", activity.Type, "event")
	}
	if activity.AssetID != 123 {
		t.Errorf("AssetID = %d, want 123", activity.AssetID)
	}
}
