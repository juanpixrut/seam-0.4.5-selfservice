// qr_regression_test.go — guards against previously-fixed bugs in QRHandler.
//
// patched1: QRHandler.setStatus (used by Disable and Enable) was calling
// entity.Data["url"] on the entity returned by xolu.Update, which has nil Data.
// This caused a nil map panic or silently returned an empty url field.
// The fix was to re-fetch via xolu.Get after Update.
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ha1tch/seam-ui/internal/qrcode"
)

// TestRegression_QRHandler_DisableRefetchesDocument guards against the patched1
// regression where Disable returned {"url": ""} (or panicked on nil map access)
// because it read entity.Data["url"] from the Update response, which has nil Data.
// The handler must issue a Get after Update to retrieve the document.
func TestRegression_QRHandler_DisableRefetchesDocument(t *testing.T) {
	const qrID = float64(99)
	const qrCode = "ABC123"

	server := mockOLUServer(t, map[string]http.HandlerFunc{
		// OQL lookup returns the QR code ID
		"POST /api/v1/oql/query": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, http.StatusOK, map[string]any{
				"status": "completed",
				"data": []map[string]any{
					{"id": qrID},
				},
				"stats": map[string]any{},
			})
		},
		// Update — returns message only, no document (the xolu contract)
		"PUT /api/v1/qr_codes/": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, http.StatusOK, map[string]any{
				"message": "updated successfully",
			})
		},
		// Re-fetch — returns full document
		"GET /api/v1/qr_codes/": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, http.StatusOK, map[string]any{
				"id":     qrID,
				"code":   qrCode,
				"status": "disabled",
			})
		},
	})
	defer server.Close()

	gen := qrcode.NewGenerator(qrcode.Config{
		BaseURL: "https://seam.test/s/",
		Prefix:  "SEAM",
	})

	h := NewQRHandler(testHandler(t, server), gen)

	req := httptest.NewRequest("POST", "/api/v1/qr/"+qrCode+"/disable", nil)
	rec := httptest.NewRecorder()

	h.Disable(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Before the patched1 fix, url would be "" because entity.Data was nil.
	url, ok := resp["url"].(string)
	if !ok || url == "" {
		t.Errorf("expected url to be populated in response — re-fetch after Update may be missing (got %v)", resp["url"])
	}

	// The status field should also be present, from the re-fetched document.
	if resp["status"] != "disabled" {
		t.Errorf("expected status=disabled in response, got %v — re-fetch after Update may be missing", resp["status"])
	}
}
