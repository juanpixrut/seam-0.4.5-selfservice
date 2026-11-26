// regression_test.go — guards against previously-fixed bugs in the alerts package.
//
// patched1: alerts.Service.Create was returning an Alert with all zero/empty fields
// because it called entityToAlert(entity) on the entity returned by xolu.Create,
// which has nil Data (xolu does not echo the document on creation). The fix was to
// re-fetch via xolu.Get after Create. These tests guard that regression.
package alerts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	xolu "github.com/ha1tch/xolu/pkg/client"
)

// mockOLUForAlerts creates a test xolu server that handles Create (POST /alerts)
// and Get (GET /alerts/{id}) in sequence, simulating the re-fetch pattern.
func mockOLUForAlerts(t *testing.T, id int64, doc map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost:
			// Create — return id only, no document
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{
				"message": "created",
				"id":      float64(id),
			})
		case r.Method == http.MethodGet:
			// Re-fetch — return the full document
			json.NewEncoder(w).Encode(doc)
		default:
			t.Errorf("unexpected method %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
}

// TestRegression_Create_RefetchPopulatesAlert guards against the patched1 regression
// where Create returned an Alert with empty fields because entity.Data was nil.
// The service must issue a Get after Create to populate the returned Alert.
func TestRegression_Create_RefetchPopulatesAlert(t *testing.T) {
	const alertID int64 = 42
	doc := map[string]any{
		"id":        float64(alertID),
		"tenant_id": float64(1),
		"title":     "High temperature detected",
		"message":   "Sensor reading exceeded threshold",
		"severity":  SeverityHigh,
		"status":    StatusOpen,
		"source":    "rules_engine",
	}

	server := mockOLUForAlerts(t, alertID, doc)
	defer server.Close()

	svc := NewService(xolu.New(server.URL))
	alert, err := svc.Create(context.Background(), CreateAlertRequest{
		TenantID: 1,
		Title:    "High temperature detected",
		Message:  "Sensor reading exceeded threshold",
		Severity: SeverityHigh,
		Source:   "rules_engine",
	})

	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Before the patched1 fix, all these would be zero/empty because
	// entityToAlert was called on an entity with nil Data.
	if alert.ID != alertID {
		t.Errorf("expected ID %d, got %d — re-fetch may be missing", alertID, alert.ID)
	}
	if alert.Title == "" {
		t.Errorf("expected Title to be populated — re-fetch after Create may be missing")
	}
	if alert.Severity == "" {
		t.Errorf("expected Severity to be populated — re-fetch after Create may be missing")
	}
	if alert.Status == "" {
		t.Errorf("expected Status to be populated — re-fetch after Create may be missing")
	}
}
