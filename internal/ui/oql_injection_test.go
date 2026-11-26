package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ha1tch/seam-ui/internal/auth"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

// This file guards a real, serious vulnerability found 2026-08-04 via
// adversarial testing: every list-with-filters handler in routes.go
// built its OQL query by directly string-concatenating URL query
// parameters into the WHERE clause, completely unescaped --
//
//	query += " AND status = '" + statusFilter + "'"
//
// with statusFilter read straight from r.URL.Query().Get("status"). A
// single quote in any filter value breaks out of the intended string
// literal into the WHERE clause itself -- classic SQL/OQL injection,
// reachable by anyone who can construct a URL, no special privilege
// needed beyond viewing the page at all. internal/auth already had an
// escapeOQL helper for its own OQL construction; this package simply
// never had an equivalent, despite building OQL queries from user
// input in six separate handlers: AssetsList (4 filters), EventsList
// (4), SensorsList (2), AlertsList (2), MapView (2), AdminUsersList
// (1) -- 15 injection points in total.
//
// Two of those (AssetsList's type_id, EventsList's asset_id) were
// unquoted numeric comparisons -- escaping doesn't help there at all
// (there's no quote to escape out of), so those use a numeric
// validation check (validOQLIntFilter) instead of escapeOQL.
//
// Fixed by adding escapeOQL (routes.go, same quote-doubling logic as
// internal/auth's own) and applying it -- or validOQLIntFilter for the
// two numeric cases -- to every one of the 15 points.
//
// Each test below captures the actual OQL query string sent to xolu
// (via the mock server's own request body) and asserts the injection
// payload's quote was doubled (escaped), not passed through raw.

// captureOQLQuery returns a mockOLU handler that records the "query"
// field of every OQL POST body it receives, in order, and a pointer to
// the slice of captured values -- some handlers (AssetsList, at least)
// make more than one OQL call per request (e.g. a filter-dropdown
// options query alongside the actual filtered list), so a single
// last-write-wins capture would silently only ever see the final one.
func captureOQLQuery() (*[]string, http.HandlerFunc) {
	captured := new([]string)
	return captured, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Query string `json:"query"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		*captured = append(*captured, body.Query)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"status": "completed", "data": []map[string]any{}, "stats": map[string]any{}})
	}
}

// anyContains reports whether any captured query contains substr.
func anyContains(queries []string, substr string) bool {
	for _, q := range queries {
		if strings.Contains(q, substr) {
			return true
		}
	}
	return false
}

// allQueries joins captured queries for a diagnostic message.
func allQueries(queries []string) string {
	return strings.Join(queries, "\n---\n")
}

const oqlInjectionPayload = `x' OR '1'='1`

func TestAssetsList_FiltersAreEscaped(t *testing.T) {
	captured, oqlHandler := captureOQLQuery()
	server := mockOLU(t, map[string]http.HandlerFunc{"POST /api/v1/oql/query": oqlHandler})
	defer server.Close()
	h := newHandler(t, server)

	qv := url.Values{"q": {oqlInjectionPayload}, "status": {oqlInjectionPayload}, "criticidad": {oqlInjectionPayload}}
	req := httptest.NewRequest("GET", "/assets?"+qv.Encode(), nil)
	rec := httptest.NewRecorder()
	h.AssetsList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("AssetsList: got status %d, want 200", rec.Code)
	}
	if anyContains(*captured, "1'='1") {
		t.Errorf("a captured query still contains the raw, unescaped injection payload -- filters are not being escaped:\n%s", allQueries(*captured))
	}
	if !anyContains(*captured, `x'' OR ''1''=''1`) {
		t.Errorf("no captured query shows the correctly-escaped (doubled-quote) payload -- expected escaping didn't happen as intended:\n%s", allQueries(*captured))
	}
}

func TestAssetsList_TypeFilterRejectsNonNumeric(t *testing.T) {
	// type_id is embedded unquoted ("type_id = "+typeFilter) -- escaping
	// quotes does nothing here, since there's no quote to escape out of.
	// The fix is a numeric check that silently drops the condition
	// (degrading to "no type filter," not an error) when the value
	// isn't a plain integer.
	captured, oqlHandler := captureOQLQuery()
	server := mockOLU(t, map[string]http.HandlerFunc{"POST /api/v1/oql/query": oqlHandler})
	defer server.Close()
	h := newHandler(t, server)

	qv := url.Values{"type": {"1 OR 1=1"}}
	req := httptest.NewRequest("GET", "/assets?"+qv.Encode(), nil)
	rec := httptest.NewRecorder()
	h.AssetsList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("AssetsList: got status %d, want 200", rec.Code)
	}
	if anyContains(*captured, "OR 1=1") || anyContains(*captured, "OR 1 = 1") {
		t.Errorf("a captured query contains an injected OR clause via the numeric type_id filter -- validOQLIntFilter isn't rejecting it:\n%s", allQueries(*captured))
	}
}

func TestEventsList_FiltersAreEscaped(t *testing.T) {
	captured, oqlHandler := captureOQLQuery()
	server := mockOLU(t, map[string]http.HandlerFunc{"POST /api/v1/oql/query": oqlHandler})
	defer server.Close()
	h := newHandler(t, server)

	qv := url.Values{"type": {oqlInjectionPayload}, "from": {oqlInjectionPayload}}
	req := httptest.NewRequest("GET", "/events?"+qv.Encode(), nil)
	rec := httptest.NewRecorder()
	h.EventsList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("EventsList: got status %d, want 200", rec.Code)
	}
	if anyContains(*captured, "1'='1") {
		t.Errorf("a captured query still contains the raw, unescaped injection payload:\n%s", allQueries(*captured))
	}
}

func TestSensorsList_FiltersAreEscaped(t *testing.T) {
	captured, oqlHandler := captureOQLQuery()
	server := mockOLU(t, map[string]http.HandlerFunc{"POST /api/v1/oql/query": oqlHandler})
	defer server.Close()
	h := newHandler(t, server)

	qv := url.Values{"type": {oqlInjectionPayload}, "status": {oqlInjectionPayload}}
	req := httptest.NewRequest("GET", "/sensors?"+qv.Encode(), nil)
	rec := httptest.NewRecorder()
	h.SensorsList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("SensorsList: got status %d, want 200", rec.Code)
	}
	if anyContains(*captured, "1'='1") {
		t.Errorf("a captured query still contains the raw, unescaped injection payload:\n%s", allQueries(*captured))
	}
}

func TestAlertsList_FiltersAreEscaped(t *testing.T) {
	captured, oqlHandler := captureOQLQuery()
	server := mockOLU(t, map[string]http.HandlerFunc{"POST /api/v1/oql/query": oqlHandler})
	defer server.Close()
	h := newHandler(t, server)

	qv := url.Values{"status": {oqlInjectionPayload}, "severity": {oqlInjectionPayload}}
	req := httptest.NewRequest("GET", "/alerts?"+qv.Encode(), nil)
	rec := httptest.NewRecorder()
	h.AlertsList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("AlertsList: got status %d, want 200", rec.Code)
	}
	if anyContains(*captured, "1'='1") {
		t.Errorf("a captured query still contains the raw, unescaped injection payload:\n%s", allQueries(*captured))
	}
}

func TestMapView_FiltersAreEscaped(t *testing.T) {
	captured, oqlHandler := captureOQLQuery()
	server := mockOLU(t, map[string]http.HandlerFunc{"POST /api/v1/oql/query": oqlHandler})
	defer server.Close()
	client := xolu.New(server.URL)
	cfg := Config{AppName: "Test", Version: "0.0.0", EnableMap: true} // MapView redirects to /assets when EnableMap is false
	h := NewUIHandler(client, cfg, nil, nil, nil, nil, nil, nil, nil)

	qv := url.Values{"status": {oqlInjectionPayload}, "type": {oqlInjectionPayload}}
	req := httptest.NewRequest("GET", "/map?"+qv.Encode(), nil)
	rec := httptest.NewRecorder()
	h.MapView(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("MapView: got status %d, want 200", rec.Code)
	}
	if anyContains(*captured, "1'='1") {
		t.Errorf("a captured query still contains the raw, unescaped injection payload:\n%s", allQueries(*captured))
	}
}

func TestAdminUsersList_RoleFilterIsEscaped(t *testing.T) {
	captured, oqlHandler := captureOQLQuery()
	server := mockOLU(t, map[string]http.HandlerFunc{"POST /api/v1/oql/query": oqlHandler})
	defer server.Close()
	h := newHandler(t, server)

	qv := url.Values{"role": {oqlInjectionPayload}}
	req := httptest.NewRequest("GET", "/admin/users?"+qv.Encode(), nil)
	req = req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{UserID: 1, TenantID: 1, Role: "admin"}))
	rec := httptest.NewRecorder()
	h.AdminUsersList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("AdminUsersList: got status %d, want 200", rec.Code)
	}
	if anyContains(*captured, "1'='1") {
		t.Errorf("a captured query still contains the raw, unescaped injection payload:\n%s", allQueries(*captured))
	}
}

// TestAssetsList_LegitimateFiltersStillWork is the inverse check: the
// escaping fix must not corrupt ordinary, non-malicious filter values
// -- a fix that breaks the happy path is its own kind of regression.
func TestAssetsList_LegitimateFiltersStillWork(t *testing.T) {
	captured, oqlHandler := captureOQLQuery()
	server := mockOLU(t, map[string]http.HandlerFunc{"POST /api/v1/oql/query": oqlHandler})
	defer server.Close()
	h := newHandler(t, server)

	req := httptest.NewRequest("GET", "/assets?status=active&type=3&criticidad=Alta", nil)
	rec := httptest.NewRecorder()
	h.AssetsList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("AssetsList: got status %d, want 200", rec.Code)
	}
	for _, want := range []string{"status = 'active'", "type_id = 3", "criticality = 'Alta'"} {
		if !anyContains(*captured, want) {
			t.Errorf("no captured query has expected clean condition %q -- escaping fix may have broken ordinary filters:\n%s", want, allQueries(*captured))
		}
	}
}
