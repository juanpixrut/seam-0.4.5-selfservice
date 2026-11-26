package ui

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func iconSearch(t *testing.T, q string) []iconResult {
	t.Helper()
	h := &UIHandler{logger: slog.Default()}
	rec := httptest.NewRecorder()
	h.PickerIcons(rec, httptest.NewRequest("GET", "/picker/icons?q="+url.QueryEscape(q), nil))
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("status %d, content type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	var res []iconResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	if res == nil {
		t.Fatalf("the result must be an array, not null: %q", rec.Body.String())
	}
	return res
}

func TestIconNames_TheEmbeddedListIsLoaded(t *testing.T) {
	if n := len(loadIconNames()); n < 1000 {
		t.Fatalf("expected the vendored ligature list, got %d names", n)
	}
	for _, n := range loadIconNames() {
		if n == "" || strings.HasPrefix(n, "#") {
			t.Fatalf("comment or blank line leaked into the list: %q", n)
		}
	}
}

func TestPickerIcons_SearchIsCaseInsensitivePrefixFirstAndCapped(t *testing.T) {
	res := iconSearch(t, "HoMe")
	if len(res) == 0 || len(res) > iconPickerMaxResults {
		t.Fatalf("got %d results", len(res))
	}
	seenContains := false
	for _, r := range res {
		if r.ID != r.Label || !strings.Contains(r.ID, "home") {
			t.Errorf("unexpected result %+v", r)
		}
		if strings.HasPrefix(r.ID, "home") && seenContains {
			t.Errorf("prefix matches must come before names that only contain the text: %+v", res)
		}
		if !strings.HasPrefix(r.ID, "home") {
			seenContains = true
		}
	}
	if res[0].ID != "home" && !strings.HasPrefix(res[0].ID, "home") {
		t.Errorf("the first result must start with the text: %+v", res[0])
	}
	if len(iconSearch(t, "e")) != iconPickerMaxResults {
		t.Errorf("a broad search must be capped at %d", iconPickerMaxResults)
	}
}

func TestPickerIcons_AllReturnsEveryNameAsPlainStrings(t *testing.T) {
	h := &UIHandler{logger: slog.Default()}
	rec := httptest.NewRecorder()
	h.PickerIcons(rec, httptest.NewRequest("GET", "/picker/icons?all=1", nil))
	var names []string
	if err := json.Unmarshal(rec.Body.Bytes(), &names); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(names) != len(loadIconNames()) || len(names) < 1000 {
		t.Errorf("got %d names, want %d", len(names), len(loadIconNames()))
	}
	// Mutating the response must not touch the cached list.
	names[0] = "changed"
	if loadIconNames()[0] == "changed" {
		t.Errorf("the handler must return a copy of the cached list")
	}
}

func TestPickerIcons_EmptyBlankAndUnknownReturnEmptyArray(t *testing.T) {
	for _, q := range []string{"", "   ", "zzzzzzzzzz-not-an-icon"} {
		if res := iconSearch(t, q); len(res) != 0 {
			t.Errorf("q=%q: want no results, got %+v", q, res)
		}
	}
}

func TestPickerIcons_RouteNeedsAuthWhenMounted(t *testing.T) {
	router, _ := buildRoutingTestRouter(t, true, true)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest("GET", "/picker/icons?q=home", nil))
	if rec.Code != http.StatusSeeOther {
		t.Errorf("unauthenticated: got %d, want 303 to login", rec.Code)
	}
}
