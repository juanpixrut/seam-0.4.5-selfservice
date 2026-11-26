package ui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ha1tch/seam-ui/internal/auth"
)

// Tests for the developer-account guards (campaign DEV-6). They follow the
// scaffolding of audit_wiring_test.go and validation_handler_test.go: real
// handlers, a mock xolu server, claims injected into the request context.

func asRole(req *http.Request, role string) *http.Request {
	return req.WithContext(auth.ContextWithClaims(req.Context(),
		&auth.Claims{UserID: 1, TenantID: 1, Role: role}))
}

// guardServer answers every GET with a user whose role is targetRole, counts
// PATCH calls, and records the last PATCH body.
func guardServer(t *testing.T, targetRole string, patches *atomic.Int32, lastPatch *atomic.Value) *httptest.Server {
	t.Helper()
	return mockOLU(t, map[string]http.HandlerFunc{
		"GET /api/v1": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"id": 5, "name": "Target", "email": "target@example.com",
				"role": targetRole, "active": true,
			})
		},
		"PATCH /api/v1": func(w http.ResponseWriter, r *http.Request) {
			patches.Add(1)
			b, _ := io.ReadAll(r.Body)
			lastPatch.Store(string(b))
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"message": "updated"})
		},
	})
}

func updateRequest(caller string, form url.Values) *http.Request {
	req := httptest.NewRequest("POST", "/admin/users/5", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "5")
	return asRole(req, caller)
}

func TestAdminUserUpdate_AdminCannotManageADeveloper(t *testing.T) {
	var patches atomic.Int32
	var last atomic.Value
	server := guardServer(t, auth.RoleDeveloper, &patches, &last)
	defer server.Close()
	h := newHandler(t, server)

	rec := httptest.NewRecorder()
	h.AdminUserUpdate(rec, updateRequest(auth.RoleAdmin, url.Values{"name": {"Hacked"}, "role": {"admin"}}))

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
	if patches.Load() != 0 {
		t.Error("an admin's update of a developer reached storage")
	}
}

func asUser(req *http.Request, id int64, role string) *http.Request {
	return req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{UserID: id, TenantID: 1, Role: role}))
}

// The role select offers Developer to a developer, so what a developer submits
// for another developer is what they chose: keeping it, or changing it.
func TestAdminUserUpdate_DeveloperChoosesAnotherDevelopersRole(t *testing.T) {
	for _, c := range []struct{ posted, want string }{{"developer", "developer"}, {"admin", "admin"}} {
		var patches atomic.Int32
		var last atomic.Value
		server := guardServer(t, auth.RoleDeveloper, &patches, &last)
		h := newHandler(t, server)
		rec := httptest.NewRecorder()
		h.AdminUserUpdate(rec, updateRequest(auth.RoleDeveloper, url.Values{"name": {"Renamed"}, "role": {c.posted}}))
		server.Close()
		assertRedirectsTo(t, rec, "/admin/users")
		if body, _ := last.Load().(string); !strings.Contains(body, `"role":"`+c.want+`"`) {
			t.Errorf("posted %q: PATCH body = %s", c.posted, body)
		}
	}
}

func TestAdminUserUpdate_DeveloperCannotRemoveTheirOwnDeveloperRole(t *testing.T) {
	var patches atomic.Int32
	var last atomic.Value
	server := guardServer(t, auth.RoleDeveloper, &patches, &last) // the target is user 5
	defer server.Close()
	h := newHandler(t, server)
	req := httptest.NewRequest("POST", "/admin/users/5", strings.NewReader(url.Values{"name": {"Me"}, "role": {"admin"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "5")
	rec := httptest.NewRecorder()
	h.AdminUserUpdate(rec, asUser(req, 5, auth.RoleDeveloper)) // the caller is user 5
	if rec.Code != http.StatusUnprocessableEntity || patches.Load() != 0 {
		t.Errorf("self-demotion must be refused and write nothing: %d, %d patches", rec.Code, patches.Load())
	}
	// keeping the role while saving other changes is fine
	req = httptest.NewRequest("POST", "/admin/users/5", strings.NewReader(url.Values{"name": {"Me, renamed"}, "role": {"developer"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "5")
	rec = httptest.NewRecorder()
	h.AdminUserUpdate(rec, asUser(req, 5, auth.RoleDeveloper))
	assertRedirectsTo(t, rec, "/admin/users")
}

func TestUserForms_RoleSelectOffersDeveloperOnlyToThoseWhoMayAssignIt(t *testing.T) {
	var patches atomic.Int32
	var last atomic.Value
	server := guardServer(t, auth.RoleOperator, &patches, &last)
	defer server.Close()
	h := newHandler(t, server)
	for _, c := range []struct {
		caller string
		want   bool
	}{{auth.RoleDeveloper, true}, {auth.RoleAdmin, false}} {
		rec := httptest.NewRecorder()
		h.AdminUserNew(rec, asRole(httptest.NewRequest("GET", "/admin/users/new", nil), c.caller))
		if got := strings.Contains(rec.Body.String(), `value="developer"`); got != c.want {
			t.Errorf("create form as %s: developer option %v, want %v", c.caller, got, c.want)
		}
		req := httptest.NewRequest("GET", "/admin/users/5/edit", nil)
		req.SetPathValue("id", "5")
		rec = httptest.NewRecorder()
		h.AdminUserEdit(rec, asRole(req, c.caller))
		if got := strings.Contains(rec.Body.String(), `value="developer"`); got != c.want {
			t.Errorf("edit form as %s: developer option %v, want %v", c.caller, got, c.want)
		}
	}
}

func TestWithEnumValue(t *testing.T) {
	const doc = `{
  "properties": {
    "name": {"type": "string", "enum": ["x"]},
    "role": { "type": "string", "enum": [ "admin", "viewer" ], "title": "Role" },
    "z": {"enum": ["role"]}
  },
  "required": ["role"]
}`
	got := string(withEnumValue([]byte(doc), "role", "developer"))
	want := strings.Replace(doc, `"viewer" ]`, `"viewer" ,"developer"]`, 1)
	if got != want {
		t.Errorf("exactly one insertion, everything else byte for byte:\n%s", got)
	}
	if string(withEnumValue([]byte(got), "role", "developer")) != got {
		t.Errorf("adding twice must change nothing")
	}
	for name, d := range map[string]string{
		"no such property": `{"properties":{"a":{"enum":["x"]}}}`,
		"no enum":          `{"properties":{"role":{"type":"string"}}}`,
		"enum elsewhere":   `{"definitions":{"properties":{"role":{"enum":["x"]}}}}`,
		"not json":         `{"properties":`,
		"empty":            ``,
		"a scalar":         `42`,
	} {
		if string(withEnumValue([]byte(d), "role", "developer")) != d {
			t.Errorf("%s: must come back unchanged", name)
		}
	}
	if got := string(withEnumValue([]byte(`{"properties":{"role":{"enum":[]}}}`), "role", `de"v`)); got != `{"properties":{"role":{"enum":["de\"v"]}}}` {
		t.Errorf("an empty enum takes the value, quoted safely: %s", got)
	}
	if got := string(withEnumValue([]byte(`{"properties":{"role":{"enum":["a"],"nested":{"enum":["b"]}}}}`), "role", "c")); got != `{"properties":{"role":{"enum":["a","c"],"nested":{"enum":["b"]}}}}` {
		t.Errorf("only the property's own enum is touched: %s", got)
	}
}

func TestAdminUserUpdate_AdminCannotGrantDeveloper(t *testing.T) {
	var patches atomic.Int32
	var last atomic.Value
	server := guardServer(t, auth.RoleOperator, &patches, &last)
	defer server.Close()
	h := newHandler(t, server)

	rec := httptest.NewRecorder()
	h.AdminUserUpdate(rec, updateRequest(auth.RoleAdmin, url.Values{"name": {"Ana"}, "role": {"developer"}}))

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
	if patches.Load() != 0 {
		t.Error("an admin granted the developer role")
	}
}

func TestAdminUserUpdate_DeveloperCanGrantDeveloper(t *testing.T) {
	var patches atomic.Int32
	var last atomic.Value
	server := guardServer(t, auth.RoleOperator, &patches, &last)
	defer server.Close()
	h := newHandler(t, server)

	rec := httptest.NewRecorder()
	h.AdminUserUpdate(rec, updateRequest(auth.RoleDeveloper, url.Values{"name": {"Ana"}, "role": {"developer"}}))

	assertRedirectsTo(t, rec, "/admin/users")
	if body, _ := last.Load().(string); !strings.Contains(body, `"role":"developer"`) {
		t.Errorf("PATCH body = %s", body)
	}
}

func TestAdminUserUpdate_OrdinaryUpdatesByAdminAreUnchanged(t *testing.T) {
	var patches atomic.Int32
	var last atomic.Value
	server := guardServer(t, auth.RoleOperator, &patches, &last)
	defer server.Close()
	h := newHandler(t, server)

	rec := httptest.NewRecorder()
	h.AdminUserUpdate(rec, updateRequest(auth.RoleAdmin, url.Values{"name": {"Ana"}, "role": {"manager"}}))

	assertRedirectsTo(t, rec, "/admin/users")
	if body, _ := last.Load().(string); !strings.Contains(body, `"role":"manager"`) {
		t.Errorf("PATCH body = %s", body)
	}
}

func TestAdminUserEdit_AdminCannotOpenADeveloper(t *testing.T) {
	var patches atomic.Int32
	var last atomic.Value
	server := guardServer(t, auth.RoleDeveloper, &patches, &last)
	defer server.Close()
	h := newHandler(t, server)

	req := asRole(httptest.NewRequest("GET", "/admin/users/5", nil), auth.RoleAdmin)
	req.SetPathValue("id", "5")
	rec := httptest.NewRecorder()
	h.AdminUserEdit(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestAdminUserPasswordReset_AdminCannotOpenDeveloperForm(t *testing.T) {
	var patches atomic.Int32
	var last atomic.Value
	server := guardServer(t, auth.RoleDeveloper, &patches, &last)
	defer server.Close()
	h := newHandler(t, server)

	req := asRole(httptest.NewRequest("GET", "/admin/users/5/password", nil), auth.RoleAdmin)
	req.SetPathValue("id", "5")
	rec := httptest.NewRecorder()
	h.AdminUserPasswordReset(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func resetSaveRequest(caller string) *http.Request {
	req := httptest.NewRequest("POST", "/admin/users/5/password", strings.NewReader(url.Values{
		"new_password": {"longenough1"}, "confirm_password": {"longenough1"},
	}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "5")
	return asRole(req, caller)
}

func TestAdminUserPasswordResetSave_Guards(t *testing.T) {
	cases := []struct {
		name, caller, target string
		wantCode             int
	}{
		{"admin cannot reset a developer", auth.RoleAdmin, auth.RoleDeveloper, http.StatusForbidden},
		{"manager cannot reset a developer", auth.RoleManager, auth.RoleDeveloper, http.StatusForbidden},
		{"developer can reset a developer", auth.RoleDeveloper, auth.RoleDeveloper, http.StatusSeeOther},
		{"admin can still reset an operator", auth.RoleAdmin, auth.RoleOperator, http.StatusSeeOther},
	}
	for _, c := range cases {
		var patches atomic.Int32
		var last atomic.Value
		server := guardServer(t, c.target, &patches, &last)
		h := newHandler(t, server)
		rec := httptest.NewRecorder()
		h.AdminUserPasswordResetSave(rec, resetSaveRequest(c.caller))
		server.Close()
		if rec.Code != c.wantCode {
			t.Errorf("%s: status = %d, want %d", c.name, rec.Code, c.wantCode)
		}
	}
}

func TestAdminUserPasswordResetSave_UnreadableTargetIsRefused(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{
		"GET /api/v1": func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	rec := httptest.NewRecorder()
	h.AdminUserPasswordResetSave(rec, resetSaveRequest(auth.RoleAdmin))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404: with the target's role unknown the reset must be refused", rec.Code)
	}
}

func createRequest(caller, role string) *http.Request {
	req := httptest.NewRequest("POST", "/admin/users", strings.NewReader(url.Values{
		"email": {"new@example.com"}, "name": {"New User"}, "role": {role},
		"password": {"longenough1"}, "password_confirm": {"longenough1"},
	}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return asRole(req, caller)
}

func TestAdminUserCreate_OnlyDevelopersCreateDevelopers(t *testing.T) {
	newServer := func() *httptest.Server {
		return mockOLU(t, map[string]http.HandlerFunc{
			"POST /api/v1": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]any{"id": float64(42), "message": "created"})
			},
		})
	}

	server := newServer()
	h := newHandler(t, server)
	rec := httptest.NewRecorder()
	h.AdminUserCreate(rec, createRequest(auth.RoleAdmin, auth.RoleDeveloper))
	server.Close()
	if rec.Code == http.StatusSeeOther {
		t.Error("an admin created a developer account")
	}

	server = newServer()
	h = newHandler(t, server)
	rec = httptest.NewRecorder()
	h.AdminUserCreate(rec, createRequest(auth.RoleDeveloper, auth.RoleDeveloper))
	server.Close()
	assertRedirectsTo(t, rec, "/admin/users")

	// unchanged for ordinary roles
	server = newServer()
	h = newHandler(t, server)
	rec = httptest.NewRecorder()
	h.AdminUserCreate(rec, createRequest(auth.RoleAdmin, auth.RoleOperator))
	server.Close()
	assertRedirectsTo(t, rec, "/admin/users")
}

func TestAdminUsersList_DeveloperAccountsAreHiddenFromNonDevelopers(t *testing.T) {
	rows := []map[string]any{
		{"id": 1, "email": "dev-person@example.com", "name": "Dev", "role": "developer", "active": true},
		{"id": 2, "email": "op-person@example.com", "name": "Op", "role": "operator", "active": true},
	}
	newServer := func() *httptest.Server {
		return mockOLU(t, map[string]http.HandlerFunc{
			"POST /api/v1/oql/query": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]any{
					"status": "completed", "data": rows,
					"stats": map[string]any{"rows_returned": len(rows)},
				})
			},
		})
	}
	listAs := func(role string) string {
		server := newServer()
		defer server.Close()
		h := newHandler(t, server)
		rec := httptest.NewRecorder()
		h.AdminUsersList(rec, asRole(httptest.NewRequest("GET", "/admin/users", nil), role))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d", role, rec.Code)
		}
		return rec.Body.String()
	}

	adminView := listAs(auth.RoleAdmin)
	if strings.Contains(adminView, "dev-person@example.com") {
		t.Error("an admin can see a developer account in the Users list")
	}
	if !strings.Contains(adminView, "op-person@example.com") {
		t.Error("ordinary users disappeared from the list")
	}
	if !strings.Contains(listAs(auth.RoleDeveloper), "dev-person@example.com") {
		t.Error("a developer cannot see developer accounts")
	}
}
