package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestPickerSearch_UnknownEntity_Returns400 confirms an unrecognised
// entity key is rejected explicitly, not silently ignored or defaulted
// to some other config.
func TestPickerSearch_UnknownEntity_Returns400(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{})
	defer server.Close()
	h := newHandler(t, server)

	req := httptest.NewRequest("GET", "/picker/search?entity=entity:nonexistent&q=test", nil)
	rec := httptest.NewRecorder()
	h.PickerSearch(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("unknown entity: got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// TestPickerSearch_QueryIsEscaped is the adversarial one, guarding the
// exact lesson T-63 (this session) established: search terms flow into
// an OQL LIKE clause built by string concatenation
// (runPickerSearch) -- an unescaped single quote in the query breaks
// out of the intended string literal into the WHERE clause itself.
// Captures the actual OQL query sent to xolu and confirms an
// injection-shaped search term is correctly escaped (doubled quote),
// not passed through raw.
func TestPickerSearch_QueryIsEscaped(t *testing.T) {
	var capturedQuery string
	server := mockOLU(t, map[string]http.HandlerFunc{
		"POST /api/v1/oql/query": func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Query string `json:"query"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			capturedQuery = body.Query
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"status": "completed", "data": []map[string]any{}, "stats": map[string]any{}})
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	qv := url.Values{"entity": {"entity:assets"}, "q": {`x' OR '1'='1`}}
	req := httptest.NewRequest("GET", "/picker/search?"+qv.Encode(), nil)
	rec := httptest.NewRecorder()
	h.PickerSearch(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("PickerSearch: got status %d, want 200", rec.Code)
	}
	if strings.Contains(capturedQuery, "1'='1") {
		t.Errorf("query contains the raw, unescaped injection payload -- search terms are not being escaped:\n%s", capturedQuery)
	}
	if !strings.Contains(capturedQuery, `x'' OR ''1''=''1`) {
		t.Errorf("query doesn't show the correctly-escaped (doubled-quote) payload:\n%s", capturedQuery)
	}
}

// TestPickerSearch_ExcludeIsValidatedNumeric confirms a malformed
// exclude parameter is silently dropped from the query rather than
// either erroring or (worse) being concatenated unescaped -- exclude
// is meant to carry a numeric ID (parent_id's own self-reference
// guard), and anything that doesn't parse as one is simply not a valid
// exclusion, not a request to reject outright.
func TestPickerSearch_ExcludeIsValidatedNumeric(t *testing.T) {
	var capturedQuery string
	server := mockOLU(t, map[string]http.HandlerFunc{
		"POST /api/v1/oql/query": func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Query string `json:"query"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			capturedQuery = body.Query
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"status": "completed", "data": []map[string]any{}, "stats": map[string]any{}})
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	qv := url.Values{"entity": {"entity:assets"}, "q": {"test"}, "exclude": {"5 OR 1=1"}}
	req := httptest.NewRequest("GET", "/picker/search?"+qv.Encode(), nil)
	rec := httptest.NewRecorder()
	h.PickerSearch(rec, req)

	if strings.Contains(capturedQuery, "OR 1=1") {
		t.Errorf("query contains an injected OR clause via the malformed exclude parameter:\n%s", capturedQuery)
	}
}

// TestPickerSearch_IDLookup_ReturnsExactMatchOnly confirms the id=
// lookup mode is a genuine exact match, not the same substring-LIKE
// search q= uses -- an id lookup for "5" must not also match an
// unrelated asset whose name happens to contain "5".
func TestPickerSearch_IDLookup_ReturnsExactMatchOnly(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{
		"GET /api/v1/assets/5": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"id": float64(5), "name": "Conveyor Belt Motor", "status": "active"})
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	req := httptest.NewRequest("GET", "/picker/search?entity=entity:assets&id=5", nil)
	rec := httptest.NewRecorder()
	h.PickerSearch(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("PickerSearch id lookup: got status %d, want 200", rec.Code)
	}
	var results []pickerSearchResult
	json.NewDecoder(rec.Body).Decode(&results)
	if len(results) != 1 {
		t.Fatalf("expected exactly 1 result for an id lookup, got %d", len(results))
	}
	if results[0].ID != 5 || !strings.Contains(results[0].Label, "Conveyor Belt Motor") {
		t.Errorf("unexpected result: %+v", results[0])
	}
}

// TestPickerSearch_LabelIsJSONEncoded_NotVulnerableToBreakout confirms
// a malicious asset name (real, user-editable data -- the exact same
// risk class as the Map page's asset names, T-63) reaches the client
// as safely JSON-encoded data, not raw text that could break out of
// the JSON response structure itself.
func TestPickerSearch_LabelIsJSONEncoded_NotVulnerableToBreakout(t *testing.T) {
	malicious := `Motor"},{"id":999,"label":"injected`
	server := mockOLU(t, map[string]http.HandlerFunc{
		"POST /api/v1/oql/query": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"status": "completed", "data": []map[string]any{
				{"id": float64(1), "name": malicious, "status": "active"},
			}, "stats": map[string]any{}})
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	req := httptest.NewRequest("GET", "/picker/search?entity=entity:assets&q=motor", nil)
	rec := httptest.NewRecorder()
	h.PickerSearch(rec, req)

	var results []pickerSearchResult
	if err := json.NewDecoder(rec.Body).Decode(&results); err != nil {
		t.Fatalf("response body isn't valid JSON -- the malicious name broke the response structure: %v\nbody: %s", err, rec.Body.String())
	}
	if len(results) != 1 {
		t.Fatalf("expected exactly 1 result (the malicious name must not have injected a second, fake result), got %d: %+v", len(results), results)
	}
	if results[0].ID != 1 {
		t.Errorf("expected the real result's id (1), got %d -- the injected id=999 may have been parsed instead", results[0].ID)
	}
}
