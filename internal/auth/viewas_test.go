package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCanViewAs(t *testing.T) {
	for _, c := range []struct {
		real, target string
		want         bool
	}{
		{RoleDeveloper, RoleAdmin, true}, {RoleDeveloper, RoleManager, true}, {RoleDeveloper, RoleOperator, true}, {RoleDeveloper, RoleViewer, true},
		{RoleDeveloper, RoleDeveloper, false}, // that is the real role, not viewing as anything
		{RoleDeveloper, "", false}, {RoleDeveloper, "root", false}, {RoleDeveloper, "ADMIN", false}, {RoleDeveloper, "admin ", false},
		{RoleAdmin, RoleViewer, false}, {RoleAdmin, RoleDeveloper, false}, {RoleViewer, RoleViewer, false}, {"", RoleViewer, false},
	} {
		if got := CanViewAs(c.real, c.target); got != c.want {
			t.Errorf("CanViewAs(%q, %q) = %v, want %v", c.real, c.target, got, c.want)
		}
	}
}

func TestRealRoleHelpers(t *testing.T) {
	claims := &Claims{UserID: 1, TenantID: 1, Role: RoleViewer}
	if got := RealRole(context.Background(), claims); got != RoleViewer {
		t.Errorf("with nothing recorded the claims' own role is the real one, got %q", got)
	}
	ctx := ContextWithRealRole(context.Background(), RoleDeveloper)
	if got := RealRole(ctx, claims); got != RoleDeveloper {
		t.Errorf("the recorded role wins, got %q", got)
	}
	if _, ok := RealRoleFromContext(context.Background()); ok {
		t.Errorf("nothing recorded, nothing reported")
	}
	if _, ok := RealRoleFromContext(ContextWithRealRole(context.Background(), "")); ok {
		t.Errorf("an empty recorded role is no role")
	}
	if got := RealRole(context.Background(), nil); got != "" {
		t.Errorf("no claims, no role: %q", got)
	}
}

func sessionFor(t *testing.T, m *SessionManager, role string) (*http.Cookie, *http.Request) {
	t.Helper()
	rec := httptest.NewRecorder()
	if err := m.CreateSession(rec, &User{ID: 1, TenantID: 1, Email: "d@example.com", Name: "D", Role: role}); err != nil {
		t.Fatal(err)
	}
	c := rec.Result().Cookies()[0]
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(c)
	return c, req
}

func TestSessionViewAs_RoundTripKeepsTheExpiryAndCanBeCleared(t *testing.T) {
	m := NewSessionManager(SessionConfig{Secret: "s3cret", Duration: time.Hour})
	_, req := sessionFor(t, m, RoleDeveloper)
	before, _ := m.ValidateSession(req)

	rec := httptest.NewRecorder()
	if err := m.SetViewAs(rec, req, RoleViewer); err != nil {
		t.Fatal(err)
	}
	c2 := rec.Result().Cookies()[0]
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.AddCookie(c2)
	after, err := m.ValidateSession(req2)
	if err != nil || after.ViewAs != RoleViewer || after.Role != RoleDeveloper {
		t.Fatalf("view-as stored beside the real role: %+v %v", after, err)
	}
	if !after.ExpiresAt.Equal(before.ExpiresAt) || after.UserID != before.UserID || after.TenantID != before.TenantID {
		t.Errorf("choosing a role must not extend the session or change who it is: %+v vs %+v", after, before)
	}
	if !c2.HttpOnly || c2.SameSite != http.SameSiteLaxMode || !c2.Expires.Equal(before.ExpiresAt.Truncate(time.Second)) && c2.Expires.IsZero() {
		t.Errorf("the cookie keeps its attributes: %+v", c2)
	}
	rec = httptest.NewRecorder()
	if err := m.SetViewAs(rec, req2, ""); err != nil {
		t.Fatal(err)
	}
	req3 := httptest.NewRequest("GET", "/", nil)
	req3.AddCookie(rec.Result().Cookies()[0])
	if d, _ := m.ValidateSession(req3); d.ViewAs != "" {
		t.Errorf("an empty role clears it: %+v", d)
	}
}

func TestSessionViewAs_RefusalsSetNoCookie(t *testing.T) {
	m := NewSessionManager(SessionConfig{Secret: "s3cret", Duration: time.Hour})
	for _, c := range []struct{ real, target string }{
		{RoleAdmin, RoleViewer}, {RoleViewer, RoleViewer}, {RoleDeveloper, "root"}, {RoleDeveloper, RoleDeveloper}, {RoleDeveloper, strings.Repeat("a", 10000)},
	} {
		_, req := sessionFor(t, m, c.real)
		rec := httptest.NewRecorder()
		if err := m.SetViewAs(rec, req, c.target); err != ErrViewAs {
			t.Errorf("%s -> %.10q: %v", c.real, c.target, err)
		}
		if len(rec.Result().Cookies()) != 0 {
			t.Errorf("a refusal must not send a cookie")
		}
	}
	rec := httptest.NewRecorder()
	if err := m.SetViewAs(rec, httptest.NewRequest("GET", "/", nil), RoleViewer); err != ErrNoSession {
		t.Errorf("no session: %v", err)
	}
}

// The cookie is signed: a viewed-as role cannot be written into it by hand.
func TestSessionViewAs_ForgedOrTamperedCookiesAreRejected(t *testing.T) {
	m := NewSessionManager(SessionConfig{Secret: "s3cret", Duration: time.Hour})
	c, _ := sessionFor(t, m, RoleAdmin)
	for name, v := range map[string]string{
		"a flipped character": c.Value[:len(c.Value)-3] + "AAA",
		"another secret": func() string {
			o := NewSessionManager(SessionConfig{Secret: "other", Duration: time.Hour})
			cc, _ := sessionFor(t, o, RoleDeveloper)
			return cc.Value
		}(),
		"garbage": "not-a-cookie", "empty": "",
	} {
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: c.Name, Value: v})
		if _, err := m.ValidateSession(req); err == nil {
			t.Errorf("%s must not validate", name)
		}
	}
}
