package alerts

import (
	"testing"
	"time"
)

func TestSeverityConstants(t *testing.T) {
	severities := []string{
		SeverityLow,
		SeverityMedium,
		SeverityHigh,
		SeverityCritical,
	}

	for _, sev := range severities {
		if sev == "" {
			t.Error("Severity constant should not be empty")
		}
	}

	if SeverityLow != "low" {
		t.Errorf("SeverityLow = %q, want %q", SeverityLow, "low")
	}
	if SeverityCritical != "critical" {
		t.Errorf("SeverityCritical = %q, want %q", SeverityCritical, "critical")
	}
}

func TestStatusConstants(t *testing.T) {
	statuses := []string{
		StatusOpen,
		StatusAcknowledged,
		StatusResolved,
		StatusDismissed,
	}

	for _, status := range statuses {
		if status == "" {
			t.Error("Status constant should not be empty")
		}
	}

	if StatusOpen != "open" {
		t.Errorf("StatusOpen = %q, want %q", StatusOpen, "open")
	}
	if StatusResolved != "resolved" {
		t.Errorf("StatusResolved = %q, want %q", StatusResolved, "resolved")
	}
}

func TestIsValidSeverity(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{SeverityLow, true},
		{SeverityMedium, true},
		{SeverityHigh, true},
		{SeverityCritical, true},
		{"invalid", false},
		{"", false},
		{"LOW", false}, // Case sensitive
	}

	for _, tt := range tests {
		got := isValidSeverity(tt.input)
		if got != tt.want {
			t.Errorf("isValidSeverity(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestMapToAlert(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	data := map[string]interface{}{
		"id":              float64(123),
		"tenant_id":       float64(456),
		"asset_id":        float64(789),
		"rule_id":         float64(101),
		"event_id":        float64(202),
		"title":           "Test Alert",
		"message":         "This is a test alert",
		"severity":        SeverityHigh,
		"status":          StatusOpen,
		"source":          "rule_engine",
		"acknowledged_by": float64(999),
		"created_at":      now.Format(time.RFC3339),
		"updated_at":      now.Format(time.RFC3339),
		"data": map[string]interface{}{
			"threshold": float64(100),
		},
	}

	alert := mapToAlert(data)

	if alert.ID != 123 {
		t.Errorf("ID = %d, want 123", alert.ID)
	}
	if alert.TenantID != 456 {
		t.Errorf("TenantID = %d, want 456", alert.TenantID)
	}
	if alert.AssetID != 789 {
		t.Errorf("AssetID = %d, want 789", alert.AssetID)
	}
	if alert.RuleID != 101 {
		t.Errorf("RuleID = %d, want 101", alert.RuleID)
	}
	if alert.EventID != 202 {
		t.Errorf("EventID = %d, want 202", alert.EventID)
	}
	if alert.Title != "Test Alert" {
		t.Errorf("Title = %q, want %q", alert.Title, "Test Alert")
	}
	if alert.Message != "This is a test alert" {
		t.Errorf("Message = %q, want %q", alert.Message, "This is a test alert")
	}
	if alert.Severity != SeverityHigh {
		t.Errorf("Severity = %q, want %q", alert.Severity, SeverityHigh)
	}
	if alert.Status != StatusOpen {
		t.Errorf("Status = %q, want %q", alert.Status, StatusOpen)
	}
	if alert.Source != "rule_engine" {
		t.Errorf("Source = %q, want %q", alert.Source, "rule_engine")
	}
	if alert.AcknowledgedBy != 999 {
		t.Errorf("AcknowledgedBy = %d, want 999", alert.AcknowledgedBy)
	}
	if !alert.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt = %v, want %v", alert.CreatedAt, now)
	}
}

func TestMapToAlertEmptyData(t *testing.T) {
	data := map[string]interface{}{
		"id": float64(1),
	}

	alert := mapToAlert(data)

	if alert.ID != 1 {
		t.Errorf("ID = %d, want 1", alert.ID)
	}
	if alert.Data == nil {
		t.Error("Data should not be nil")
	}
}

func TestAlertStructure(t *testing.T) {
	alert := &Alert{
		ID:       1,
		TenantID: 2,
		AssetID:  3,
		Title:    "Test",
		Severity: SeverityCritical,
		Status:   StatusOpen,
	}

	if alert.ID != 1 {
		t.Errorf("ID = %d, want 1", alert.ID)
	}
	if alert.Severity != SeverityCritical {
		t.Errorf("Severity = %q, want %q", alert.Severity, SeverityCritical)
	}
}

func TestCreateAlertRequest(t *testing.T) {
	req := CreateAlertRequest{
		TenantID: 1,
		AssetID:  2,
		RuleID:   3,
		Title:    "Test Alert",
		Message:  "Test message",
		Severity: SeverityHigh,
		Source:   "test",
		Data: map[string]interface{}{
			"key": "value",
		},
	}

	if req.TenantID != 1 {
		t.Errorf("TenantID = %d, want 1", req.TenantID)
	}
	if req.Title != "Test Alert" {
		t.Errorf("Title = %q, want %q", req.Title, "Test Alert")
	}
	if req.Data["key"] != "value" {
		t.Errorf("Data[key] = %v, want %q", req.Data["key"], "value")
	}
}
