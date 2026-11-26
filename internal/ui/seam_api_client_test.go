package ui

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestSeamAPIClient_SuccessfulFetch confirms the basic happy path still
// works exactly as fetchRuntimeState's own original implementation did
// -- this refactor changed how the request gets made, not what a
// successful response looks like.
func TestSeamAPIClient_SuccessfulFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/assets/urn:giai:test123" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"fsm_state": "in_service"}`))
	}))
	defer server.Close()

	c := &seamAPIClient{httpClient: &http.Client{Timeout: 5 * time.Second}}
	state, ok := c.FetchRuntimeState(server.URL, "urn:giai:test123")
	if !ok {
		t.Fatal("expected success")
	}
	if state != "in_service" {
		t.Errorf("expected state 'in_service', got %q", state)
	}
}

// TestSeamAPIClient_EmptyStateIsNotSuccess confirms an empty fsm_state
// value is treated as "not found," not a successful-but-blank result --
// matches the original implementation's own return (result.FSMState,
// result.FSMState != "").
func TestSeamAPIClient_EmptyStateIsNotSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"fsm_state": ""}`))
	}))
	defer server.Close()

	c := &seamAPIClient{httpClient: &http.Client{Timeout: 5 * time.Second}}
	_, ok := c.FetchRuntimeState(server.URL, "urn:giai:test123")
	if ok {
		t.Error("expected an empty fsm_state to report false, not success")
	}
}

// TestSeamAPIClient_NonOKStatusFails confirms a non-200 response (asset
// not found, server error, etc.) is treated as a clean failure, not a
// panic or a parsed-garbage result.
func TestSeamAPIClient_NonOKStatusFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c := &seamAPIClient{httpClient: &http.Client{Timeout: 5 * time.Second}}
	_, ok := c.FetchRuntimeState(server.URL, "urn:giai:test123")
	if ok {
		t.Error("expected a 404 response to report false")
	}
}

// TestSeamAPIClient_MalformedJSONFails confirms a non-JSON or malformed
// response body degrades to a clean failure, not a panic.
func TestSeamAPIClient_MalformedJSONFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json at all`))
	}))
	defer server.Close()

	c := &seamAPIClient{httpClient: &http.Client{Timeout: 5 * time.Second}}
	_, ok := c.FetchRuntimeState(server.URL, "urn:giai:test123")
	if ok {
		t.Error("expected malformed JSON to report false, not succeed")
	}
}

// TestSeamAPIClient_TimeoutIsEnforced is the actual point of this
// refactor, proven directly rather than just asserted in a comment: the
// original implementation used a bare http.Get with no timeout at all,
// meaning a slow or hung seam-api would hang the calling page load
// indefinitely. This test uses a real, slow server and a short client
// timeout, and confirms the call returns promptly (well under the
// server's own delay) rather than blocking for it.
func TestSeamAPIClient_TimeoutIsEnforced(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.Write([]byte(`{"fsm_state": "too_late"}`))
	}))
	defer server.Close()

	c := &seamAPIClient{httpClient: &http.Client{Timeout: 100 * time.Millisecond}}

	start := time.Now()
	_, ok := c.FetchRuntimeState(server.URL, "urn:giai:test123")
	elapsed := time.Since(start)

	if ok {
		t.Error("expected the call to fail due to timeout, not succeed with the slow server's response")
	}
	if elapsed >= 2*time.Second {
		t.Errorf("call took %v -- the timeout doesn't appear to have been enforced at all, it waited for the slow server", elapsed)
	}
	if elapsed >= 500*time.Millisecond {
		t.Errorf("call took %v, expected it to return close to the 100ms timeout, not linger", elapsed)
	}
}

// TestFetchRuntimeState_DelegatesToDefaultClient confirms the existing
// call sites' own entry point (fetchRuntimeState, unchanged signature)
// still works end to end through the new seamAPIClient underneath it --
// the refactor's whole point was preserving this shape while fixing
// what's behind it.
func TestFetchRuntimeState_DelegatesToDefaultClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"fsm_state": "active"}`))
	}))
	defer server.Close()

	state, ok := fetchRuntimeState(server.URL, "urn:giai:test123")
	if !ok || state != "active" {
		t.Errorf("expected (active, true) via the package-level function, got (%q, %v)", state, ok)
	}
}
