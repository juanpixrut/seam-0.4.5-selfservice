package ui

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/ha1tch/seam-ui/internal/auth"
)

// T-97: the "entity:users" source of the picker backend (GET /picker/search).
// Uses the fake auth.UserStore from selector_test.go. The point of these
// tests is that the id lookup obeys the SAME visibility rule as the list.

func pickerGet(h *UIHandler, rawQuery string, claims *auth.Claims) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/picker/search?"+rawQuery, nil)
	if claims != nil {
		req = req.WithContext(auth.ContextWithClaims(req.Context(), claims))
	}
	rec := httptest.NewRecorder()
	h.PickerSearch(rec, req)
	return rec
}

func decodePickerResults(t *testing.T, rec *httptest.ResponseRecorder) []pickerSearchResult {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var res []pickerSearchResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return res
}

func TestPickerUsers_SearchAppliesTenantActiveAndRoleRule(t *testing.T) {
	h, store := newSelectorTestHandler([]*auth.User{
		selUser(7, "Ana Perez", "ana@x.test", auth.RoleViewer),
		selUser(8, "Bo Diaz", "bo@x.test", auth.RoleOperator),
	})
	rec := pickerGet(h, "entity=entity:users&q=an", operatorClaims())
	res := decodePickerResults(t, rec)

	if len(store.calls) != 1 {
		t.Fatalf("want 1 ListUsers call, got %d", len(store.calls))
	}
	f := store.calls[0]
	if !reflect.DeepEqual(f.Roles, []string{"operator", "viewer"}) {
		t.Errorf("Roles = %v", f.Roles)
	}
	if f.TenantID != 5 || f.Active == nil || !*f.Active {
		t.Errorf("filter = %+v", f)
	}
	// The text search is done in Go, not sent to OQL.
	if f.Search != "" {
		t.Errorf("Search must not reach the store, got %q", f.Search)
	}
	want := []pickerSearchResult{{ID: 7, Label: "Ana Perez (ana@x.test)"}}
	if !reflect.DeepEqual(res, want) {
		t.Errorf("results = %+v, want %+v", res, want)
	}
}

// The search ignores case and matches name or email.
func TestPickerUsers_SearchIsCaseInsensitive(t *testing.T) {
	h, _ := newSelectorTestHandler([]*auth.User{
		selUser(7, "Ana Perez", "ana@x.test", auth.RoleViewer),
		selUser(8, "Bo Diaz", "BO@corp.test", auth.RoleViewer),
	})
	for q, wantID := range map[string]int64{"ANA": 7, "ana": 7, "PEREZ": 7, "corp": 8, "Bo@C": 8} {
		res := decodePickerResults(t, pickerGet(h, "entity=entity:users&q="+url.QueryEscape(q), operatorClaims()))
		if len(res) != 1 || res[0].ID != wantID {
			t.Errorf("q=%q: got %+v, want only id %d", q, res, wantID)
		}
	}
}

// Accents do not matter: "jose" finds "Jos\u00e9", "perez" finds "P\u00e9rez".
func TestPickerUsers_SearchIgnoresAccents(t *testing.T) {
	h, _ := newSelectorTestHandler([]*auth.User{
		selUser(7, "Jos\u00e9 P\u00e9rez", "jp@x.test", auth.RoleViewer),
		selUser(8, "Bo Diaz", "bo@x.test", auth.RoleViewer),
	})
	for _, q := range []string{"jose", "JOS\u00c9", "perez", "p\u00e9rez"} {
		res := decodePickerResults(t, pickerGet(h, "entity=entity:users&q="+url.QueryEscape(q), operatorClaims()))
		if len(res) != 1 || res[0].ID != 7 {
			t.Errorf("q=%q: got %+v, want only id 7", q, res)
		}
	}
}

func TestPickerUsers_SearchExcludeAndCap(t *testing.T) {
	users := make([]*auth.User, 0, 25)
	for i := int64(1); i <= 25; i++ {
		users = append(users, selUser(i, "User", "u@x.test", auth.RoleViewer))
	}
	h, _ := newSelectorTestHandler(users)

	if got := len(decodePickerResults(t, pickerGet(h, "entity=entity:users&q=u", operatorClaims()))); got != pickerUsersMaxResults {
		t.Errorf("got %d results, want the cap %d", got, pickerUsersMaxResults)
	}
	for _, r := range decodePickerResults(t, pickerGet(h, "entity=entity:users&q=u&exclude=3", operatorClaims())) {
		if r.ID == 3 {
			t.Errorf("excluded id 3 still present")
		}
	}
}

func TestPickerUsers_SearchWithoutVisibilityReturnsEmptyArray(t *testing.T) {
	for name, claims := range map[string]*auth.Claims{
		"no claims":    nil,
		"tenant 0":     {UserID: 1, TenantID: 0, Role: auth.RoleAdmin},
		"unknown role": {UserID: 1, TenantID: 5, Role: "root"},
		"empty role":   {UserID: 1, TenantID: 5, Role: ""},
	} {
		h, store := newSelectorTestHandler([]*auth.User{selUser(7, "Ana", "ana@x.test", auth.RoleViewer)})
		rec := pickerGet(h, "entity=entity:users&q=a", claims)
		if strings.TrimSpace(rec.Body.String()) != "[]" {
			t.Errorf("%s: body = %q, want []", name, rec.Body.String())
		}
		if len(store.calls) != 0 {
			t.Errorf("%s: store must not be queried", name)
		}
	}
}

func TestPickerUsers_IDLookupExactAndVisible(t *testing.T) {
	h, store := newSelectorTestHandler([]*auth.User{
		selUser(7, "Ana Perez", "ana@x.test", auth.RoleViewer),
		selUser(17, "Other", "o@x.test", auth.RoleViewer),
	})
	res := decodePickerResults(t, pickerGet(h, "entity=entity:users&id=7", operatorClaims()))
	if !reflect.DeepEqual(res, []pickerSearchResult{{ID: 7, Label: "Ana Perez (ana@x.test)"}}) {
		t.Errorf("results = %+v (id=7 must not also match 17)", res)
	}
	if len(store.calls) != 0 {
		t.Errorf("an id lookup must not run a list query")
	}
}

// The core T-97 rule: the lookup must not reveal a user the list would hide.
func TestPickerUsers_IDLookupFollowsRoleHierarchy(t *testing.T) {
	cases := []struct {
		caller, target string
		visible        bool
	}{
		{auth.RoleOperator, auth.RoleViewer, true},
		{auth.RoleOperator, auth.RoleOperator, true},
		{auth.RoleOperator, auth.RoleManager, false},
		{auth.RoleOperator, auth.RoleAdmin, false},
		{auth.RoleViewer, auth.RoleOperator, false},
		{auth.RoleAdmin, auth.RoleAdmin, true},
		{"root", auth.RoleViewer, false},
		{"", auth.RoleViewer, false},
	}
	for _, c := range cases {
		h, _ := newSelectorTestHandler([]*auth.User{selUser(7, "T", "t@x.test", c.target)})
		res := decodePickerResults(t, pickerGet(h, "entity=entity:users&id=7", &auth.Claims{UserID: 1, TenantID: 5, Role: c.caller}))
		if got := len(res) == 1; got != c.visible {
			t.Errorf("caller %q looking up %q: visible=%v, want %v", c.caller, c.target, got, c.visible)
		}
	}
}

func TestPickerUsers_IDLookupTenantAndActiveAndMissing(t *testing.T) {
	otherTenant := selUser(7, "A", "a@x.test", auth.RoleViewer)
	otherTenant.TenantID = 99
	inactive := selUser(8, "B", "b@x.test", auth.RoleViewer)
	inactive.Active = false
	h, _ := newSelectorTestHandler([]*auth.User{otherTenant, inactive})

	for _, id := range []string{"7", "8", "404", "abc", "-1", "7x"} {
		res := decodePickerResults(t, pickerGet(h, "entity=entity:users&id="+id, operatorClaims()))
		if len(res) != 0 {
			t.Errorf("id=%s: want nothing, got %+v", id, res)
		}
	}
	// No session claims: nothing, even for an id that exists in some tenant.
	if res := decodePickerResults(t, pickerGet(h, "entity=entity:users&id=7", nil)); len(res) != 0 {
		t.Errorf("no claims: want nothing, got %+v", res)
	}
}

func TestPickerUsers_TenantScopeLiftsOnlyTheRoleRule(t *testing.T) {
	manager := selUser(7, "M", "m@x.test", auth.RoleManager)
	elsewhere := selUser(8, "E", "e@x.test", auth.RoleViewer)
	elsewhere.TenantID = 99
	h, store := newSelectorTestHandler([]*auth.User{manager, elsewhere})
	h.SetSelectorUsersScope(SelectorUsersScopeTenant)

	if res := decodePickerResults(t, pickerGet(h, "entity=entity:users&id=7", operatorClaims())); len(res) != 1 {
		t.Errorf("tenant scope: an operator may resolve a manager of the same tenant, got %+v", res)
	}
	if res := decodePickerResults(t, pickerGet(h, "entity=entity:users&id=8", operatorClaims())); len(res) != 0 {
		t.Errorf("tenant scope must still hide other tenants, got %+v", res)
	}
	decodePickerResults(t, pickerGet(h, "entity=entity:users&q=m", operatorClaims()))
	if store.calls[0].Roles != nil || store.calls[0].TenantID != 5 {
		t.Errorf("tenant scope search filter = %+v", store.calls[0])
	}
}

func TestPickerUsers_NeverReturnsPasswordHash(t *testing.T) {
	u := selUser(7, "Ana", "ana@x.test", auth.RoleViewer)
	u.SetPasswordHash("$2a$10$SECRETHASHVALUE")
	h, _ := newSelectorTestHandler([]*auth.User{u})
	for _, q := range []string{"entity=entity:users&q=a", "entity=entity:users&id=7"} {
		if body := pickerGet(h, q, operatorClaims()).Body.String(); strings.Contains(body, "SECRET") || strings.Contains(body, "$2a$") {
			t.Errorf("%s: password hash leaked: %s", q, body)
		}
	}
}

func TestPickerUsers_NoUserService_Returns500(t *testing.T) {
	h := &UIHandler{page: NewPageHandler(Config{}), logger: slog.Default()}
	if rec := pickerGet(h, "entity=entity:users&q=a", operatorClaims()); rec.Code != http.StatusInternalServerError {
		t.Errorf("search: status %d, want 500", rec.Code)
	}
	if rec := pickerGet(h, "entity=entity:users&id=1", operatorClaims()); rec.Code != http.StatusInternalServerError {
		t.Errorf("lookup: status %d, want 500", rec.Code)
	}
}
