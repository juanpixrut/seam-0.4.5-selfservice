package ui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ha1tch/seam-ui/internal/auth"
)

// accountPrefsServer is a user record that can be read and merged into.
func accountPrefsServer(t *testing.T, record map[string]any, patches *int) *httptest.Server {
	t.Helper()
	return mockOLU(t, map[string]http.HandlerFunc{
		"GET /api/v1/users/": func(w http.ResponseWriter, r *http.Request) { mockEntityGet(record)(w, r) },
		"PATCH /api/v1/users/": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			var in map[string]any
			_ = json.Unmarshal(body, &in)
			for k, v := range in {
				record[k] = v
			}
			*patches++
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"id": float64(1)})
		},
	})
}

func prefsRequest(path string, cookie *http.Cookie) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	return req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{UserID: 1, TenantID: 1, Role: "developer"}))
}

func prefsCookieOf(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == developerPrefsCookie {
			return c
		}
	}
	return nil
}

// The account's preferences are the ones in force on every page that offers
// the choices: a browser that holds an older copy opens with what was chosen
// elsewhere, and its copy is brought up to date.
func TestPreferences_TheAccountWinsOverAnOlderCopy(t *testing.T) {
	patches := 0
	record := map[string]any{"id": 1.0, "name": "Dev", developerPrefScopeField: "file"}
	srv := accountPrefsServer(t, record, &patches)
	defer srv.Close()
	h := newHandler(t, srv)

	old := &http.Cookie{Name: developerPrefsCookie, Value: "scope=tenant"}
	req, rec := prefsRequest("/developer/forms", old), httptest.NewRecorder()
	h.applyDeveloperPrefs(rec, req, false)
	if got := req.URL.Query().Get("scope"); got != "file" {
		t.Fatalf("the page opens in scope %q, want the account's (file)", got)
	}
	c := prefsCookieOf(rec)
	if c == nil || readDeveloperPrefs(&http.Request{Header: http.Header{"Cookie": {c.String()}}}).Scope != "file" {
		t.Errorf("the browser's copy is brought up to date: %+v", c)
	}
	if patches != 0 {
		t.Errorf("nothing was chosen, so nothing is written to the account (%d writes)", patches)
	}

	// a choice in the address wins and is written to both
	req, rec = prefsRequest("/developer/forms?scope=tenant", c), httptest.NewRecorder()
	h.applyDeveloperPrefs(rec, req, false)
	if record[developerPrefScopeField] != "tenant" || patches != 1 {
		t.Errorf("a choice is kept with the account: %v after %d writes", record[developerPrefScopeField], patches)
	}
}

// An account that has none yet takes what the browser holds.
func TestPreferences_AnEmptyAccountTakesTheBrowsersCopy(t *testing.T) {
	patches := 0
	record := map[string]any{"id": 1.0, "name": "Dev"}
	srv := accountPrefsServer(t, record, &patches)
	defer srv.Close()
	h := newHandler(t, srv)

	req, rec := prefsRequest("/developer/forms", &http.Cookie{Name: developerPrefsCookie, Value: "scope=file"}), httptest.NewRecorder()
	h.applyDeveloperPrefs(rec, req, false)
	if got := req.URL.Query().Get("scope"); got != "file" {
		t.Errorf("the page opens in scope %q, want the browser's (file)", got)
	}
	if record[developerPrefScopeField] != "file" || patches != 1 {
		t.Errorf("the account takes it: %v after %d writes", record[developerPrefScopeField], patches)
	}
}

// With the setting on "browser" the account is neither read nor written.
func TestPreferences_BrowserOnly(t *testing.T) {
	patches := 0
	record := map[string]any{"id": 1.0, developerPrefScopeField: "file"}
	srv := accountPrefsServer(t, record, &patches)
	defer srv.Close()
	h := newHandler(t, srv)
	h.config.DevSettings.Preferences = DeveloperPreferencesBrowser

	req, rec := prefsRequest("/developer/forms?scope=tenant", nil), httptest.NewRecorder()
	h.applyDeveloperPrefs(rec, req, false)
	if patches != 0 || record[developerPrefScopeField] != "file" {
		t.Errorf("the account was written: %v, %d writes", record[developerPrefScopeField], patches)
	}
	if c := prefsCookieOf(rec); c == nil {
		t.Errorf("the choice is kept in the cookie")
	}
}
