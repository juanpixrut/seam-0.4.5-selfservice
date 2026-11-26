package ui

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/i18n"
)

// T-96 Selector tests. Mirrors asset_picker_search_test.go; the Users source
// is exercised through a fake auth.UserStore, so what is asserted is the
// auth.UserFilter the source builds (the OQL itself is covered by
// internal/auth's ListUsers tests).

type selectorFakeUserStore struct {
	users []*auth.User
	calls []auth.UserFilter
}

func (f *selectorFakeUserStore) GetUserByID(ctx context.Context, id int64) (*auth.User, error) {
	for _, u := range f.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, auth.ErrUserNotFound
}
func (f *selectorFakeUserStore) GetUserByEmail(ctx context.Context, email string) (*auth.User, error) {
	return nil, errors.New("not used")
}
func (f *selectorFakeUserStore) CreateUser(ctx context.Context, u *auth.User) error {
	return errors.New("not used")
}
func (f *selectorFakeUserStore) UpdateUser(ctx context.Context, u *auth.User) error {
	return errors.New("not used")
}
func (f *selectorFakeUserStore) DeleteUser(ctx context.Context, id int64) error {
	return errors.New("not used")
}
func (f *selectorFakeUserStore) ListUsers(ctx context.Context, filter auth.UserFilter) ([]*auth.User, error) {
	f.calls = append(f.calls, filter)
	return f.users, nil
}

func selUser(id int64, name, email, role string) *auth.User {
	return &auth.User{ID: id, TenantID: 5, Name: name, Email: email, Role: role, Active: true}
}

func newSelectorTestHandler(users []*auth.User) (*UIHandler, *selectorFakeUserStore) {
	store := &selectorFakeUserStore{users: users}
	h := &UIHandler{
		page:        NewPageHandler(Config{AppName: "Test"}),
		logger:      slog.Default(),
		userService: auth.NewUserService(store, auth.DefaultConfig()),
	}
	return h, store
}

// selectorGet calls the handler directly (no mux), so the path value that
// the mux would set is set by hand. claims nil means no session claims.
func selectorGet(h *UIHandler, key, rawQuery string, claims *auth.Claims) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/selector/"+key+"?"+rawQuery, nil)
	req.SetPathValue("key", key)
	if claims != nil {
		req = req.WithContext(auth.ContextWithClaims(req.Context(), claims))
	}
	rec := httptest.NewRecorder()
	h.Selector(rec, req)
	return rec
}

func operatorClaims() *auth.Claims {
	return &auth.Claims{UserID: 1, TenantID: 5, Role: auth.RoleOperator}
}

func TestSelector_UnknownKey_Returns400(t *testing.T) {
	h, store := newSelectorTestHandler(nil)
	// "assets" is a real selector key since the selector entities landed
	// (selector_entities.go); these are the ones that must stay unknown.
	for _, key := range []string{"nope", "asset", "users;drop", "USERS", "", "../users", "users/../assets"} {
		rec := selectorGet(h, key, "", operatorClaims())
		if rec.Code != http.StatusBadRequest {
			t.Errorf("key %q: got status %d, want 400", key, rec.Code)
		}
	}
	if len(store.calls) != 0 {
		t.Errorf("an unknown key must not reach any source")
	}
}

func TestSelector_UsersRowsCarryIDAndLabel(t *testing.T) {
	h, _ := newSelectorTestHandler([]*auth.User{
		selUser(7, "Ana Perez", "ana@x.test", auth.RoleViewer),
		selUser(9, "", "nameless@x.test", auth.RoleOperator),
	})
	rec := selectorGet(h, "users", "target=owner_id", operatorClaims())
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	html := rec.Body.String()
	for _, want := range []string{
		`data-id="7"`,
		`data-label="Ana Perez (ana@x.test)"`,
		`data-id="9"`,
		`data-label="nameless@x.test"`, // no name: the label is the email alone
		`data-selector-key="users"`,
		`data-selector-target="owner_id"`,
		`onclick="SeamSelector.choose(this)"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("response missing %s", want)
		}
	}
}

// The Users source goes through auth.UserService; nothing about a password
// hash may reach the page.
func TestSelector_PasswordHashNeverRendered(t *testing.T) {
	u := selUser(7, "Ana Perez", "ana@x.test", auth.RoleViewer)
	u.SetPasswordHash("$2a$10$SECRETHASHVALUE")
	h, _ := newSelectorTestHandler([]*auth.User{u})
	html := selectorGet(h, "users", "", operatorClaims()).Body.String()
	if strings.Contains(html, "SECRETHASH") || strings.Contains(html, "$2a$") {
		t.Errorf("password hash leaked into the selector page")
	}
}

func TestSelector_HostileLabelCannotBreakMarkup(t *testing.T) {
	hostile := `"><script>alert(1)</script>`
	h, _ := newSelectorTestHandler([]*auth.User{selUser(7, hostile, hostile+"@x.test", auth.RoleViewer)})
	html := selectorGet(h, "users", "", operatorClaims()).Body.String()
	if strings.Contains(html, "<script>") {
		t.Errorf("record label broke out of the markup: %s", html)
	}
	if !strings.Contains(html, `onclick="SeamSelector.choose(this)"`) {
		t.Errorf("row onclick must be the fixed handler call")
	}
}

func TestSelector_UsersFilterByRoleHierarchy(t *testing.T) {
	cases := []struct {
		role string
		want []string
	}{
		{auth.RoleAdmin, []string{"admin", "manager", "operator", "viewer", "admin_staff", "accountant", "account_executive"}},
		{auth.RoleManager, []string{"manager", "operator", "viewer"}},
		{auth.RoleOperator, []string{"operator", "viewer"}},
		{auth.RoleViewer, []string{"viewer"}},
	}
	for _, c := range cases {
		h, store := newSelectorTestHandler(nil)
		rec := selectorGet(h, "users", "", &auth.Claims{UserID: 1, TenantID: 5, Role: c.role})
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d", c.role, rec.Code)
		}
		if len(store.calls) != 1 {
			t.Fatalf("%s: want 1 ListUsers call, got %d", c.role, len(store.calls))
		}
		f := store.calls[0]
		if !reflect.DeepEqual(f.Roles, c.want) {
			t.Errorf("%s: Roles = %v, want %v", c.role, f.Roles, c.want)
		}
		if f.TenantID != 5 {
			t.Errorf("%s: tenant condition missing: %d", c.role, f.TenantID)
		}
		if f.Active == nil || !*f.Active {
			t.Errorf("%s: only active users may be listed", c.role)
		}
	}
}

// An unknown or empty caller role lists nobody, and does not even query.
func TestSelector_UnknownCallerRoleListsNobody(t *testing.T) {
	for _, role := range []string{"", "root", "ADMIN"} {
		h, store := newSelectorTestHandler([]*auth.User{selUser(7, "Ana", "ana@x.test", auth.RoleViewer)})
		rec := selectorGet(h, "users", "", &auth.Claims{UserID: 1, TenantID: 5, Role: role})
		html := rec.Body.String()
		if len(store.calls) != 0 {
			t.Errorf("role %q: store must not be queried", role)
		}
		if strings.Contains(html, "data-id=") {
			t.Errorf("role %q: no rows may be rendered", role)
		}
		if !strings.Contains(html, "selector.empty_title") {
			t.Errorf("role %q: empty state expected", role)
		}
	}
}

// No session claims, or a tenant id that is not positive, lists nobody:
// the tenant condition is never dropped.
func TestSelector_NoClaimsOrNoTenantListsNobody(t *testing.T) {
	for name, claims := range map[string]*auth.Claims{
		"no claims": nil,
		"tenant 0":  {UserID: 1, TenantID: 0, Role: auth.RoleAdmin},
		"tenant -1": {UserID: 1, TenantID: -1, Role: auth.RoleAdmin},
	} {
		h, store := newSelectorTestHandler([]*auth.User{selUser(7, "Ana", "ana@x.test", auth.RoleViewer)})
		html := selectorGet(h, "users", "", claims).Body.String()
		if len(store.calls) != 0 || strings.Contains(html, "data-id=") {
			t.Errorf("%s: must list nobody and not query", name)
		}
	}
}

func TestSelector_TenantScopeDropsRoleRestrictionOnly(t *testing.T) {
	h, store := newSelectorTestHandler(nil)
	if !h.SetSelectorUsersScope(SelectorUsersScopeTenant) {
		t.Fatalf("tenant scope must be accepted")
	}
	selectorGet(h, "users", "", &auth.Claims{UserID: 1, TenantID: 5, Role: auth.RoleViewer})
	f := store.calls[0]
	if f.Roles != nil {
		t.Errorf("tenant scope: Roles must be nil, got %v", f.Roles)
	}
	if f.TenantID != 5 || f.Active == nil || !*f.Active {
		t.Errorf("tenant scope must keep the tenant and active conditions: %+v", f)
	}

	// Even in tenant scope a missing session lists nobody.
	store.calls = nil
	selectorGet(h, "users", "", nil)
	if len(store.calls) != 0 {
		t.Errorf("tenant scope must still require session claims")
	}
}

func TestSetSelectorUsersScope_RejectsUnknownValue(t *testing.T) {
	h, _ := newSelectorTestHandler(nil)
	if h.SetSelectorUsersScope("everyone") {
		t.Errorf("unknown scope must be rejected")
	}
	if h.selectorUsersScope != "" {
		t.Errorf("a rejected scope must not change the setting")
	}
	// The default (unset) behaves as hierarchy.
	_, store := newSelectorTestHandler(nil)
	h2 := &UIHandler{page: NewPageHandler(Config{}), logger: slog.Default(),
		userService: auth.NewUserService(store, auth.DefaultConfig())}
	selectorGet(h2, "users", "", &auth.Claims{UserID: 1, TenantID: 5, Role: auth.RoleViewer})
	if !reflect.DeepEqual(store.calls[0].Roles, []string{"viewer"}) {
		t.Errorf("default scope must be hierarchy, got Roles %v", store.calls[0].Roles)
	}
}

func TestSelector_SearchTermIsEscapedInPage(t *testing.T) {
	h, _ := newSelectorTestHandler(nil)
	q := `  x" onfocus="alert(1)<script>  `
	html := selectorGet(h, "users", "q="+url.QueryEscape(q), operatorClaims()).Body.String()
	if strings.Contains(html, `onfocus="alert`) || strings.Contains(html, "<script>") {
		t.Errorf("search term broke out of the input attribute: %s", html)
	}
}

// The search runs in Go, ignoring case, over name and email.
func TestSelector_SearchIsCaseInsensitiveOverNameAndEmail(t *testing.T) {
	h, _ := newSelectorTestHandler([]*auth.User{
		selUser(7, "Ana Perez", "ana@x.test", auth.RoleViewer),
		selUser(8, "Bo Diaz", "BO@corp.test", auth.RoleViewer),
	})
	for q, want := range map[string][]string{
		"ana": {`data-id="7"`}, "ANA": {`data-id="7"`}, "perez": {`data-id="7"`},
		"corp": {`data-id="8"`}, "bo@": {`data-id="8"`}, "  ": {`data-id="7"`, `data-id="8"`},
	} {
		html := selectorGet(h, "users", "q="+url.QueryEscape(q), operatorClaims()).Body.String()
		if got := strings.Count(html, "data-id="); got != len(want) {
			t.Errorf("q=%q: %d rows, want %d", q, got, len(want))
		}
		for _, w := range want {
			if !strings.Contains(html, w) {
				t.Errorf("q=%q: missing %s", q, w)
			}
		}
	}
}

func TestSelector_OverlongSearchIsTruncated(t *testing.T) {
	h, _ := newSelectorTestHandler(nil)
	html := selectorGet(h, "users", "q="+strings.Repeat("a", 500), operatorClaims()).Body.String()
	if !strings.Contains(html, `value="`+strings.Repeat("a", selectorMaxQueryRunes)+`"`) ||
		strings.Contains(html, strings.Repeat("a", selectorMaxQueryRunes+1)) {
		t.Errorf("the search term must be cut to %d characters", selectorMaxQueryRunes)
	}
}

func TestSelector_TargetAllowlist(t *testing.T) {
	h, _ := newSelectorTestHandler(nil)
	bad := selectorGet(h, "users", "target="+url.QueryEscape(`x"><script>`), operatorClaims()).Body.String()
	if strings.Contains(bad, "<script>") || strings.Contains(bad, `data-selector-target="x`) {
		t.Errorf("hostile target must be dropped: %s", bad)
	}
	if !strings.Contains(bad, `data-selector-target=""`) {
		t.Errorf("a rejected target renders as empty")
	}
	good := selectorGet(h, "users", "target=asset_form.owner-1", operatorClaims()).Body.String()
	if !strings.Contains(good, `data-selector-target="asset_form.owner-1"`) {
		t.Errorf("a plain id must be echoed")
	}
}

func TestSelector_PaginationUsesHasMore(t *testing.T) {
	// 11 rows come back for a page size of 10: one extra tells there is a next page.
	users := make([]*auth.User, 0, 11)
	for i := int64(1); i <= 11; i++ {
		users = append(users, selUser(i, "User", "u@x.test", auth.RoleViewer))
	}
	h, store := newSelectorTestHandler(users)
	html := selectorGet(h, "users", "target=owner", operatorClaims()).Body.String()

	_ = store
	if got := strings.Count(html, "data-id="); got != selectorDefaultPageSize {
		t.Errorf("rendered %d rows, want %d (the extra row is only a probe)", got, selectorDefaultPageSize)
	}
	if !strings.Contains(html, "page=2") {
		t.Errorf("a Next control to page 2 expected")
	}
	if !strings.Contains(html, "disabled") {
		t.Errorf("Previous must be disabled on page 1")
	}

	// Page 2 (paging happens in Go): the 11th user is the only row left, so
	// Previous is enabled and Next is disabled.
	h2, _ := newSelectorTestHandler(users)
	html2 := selectorGet(h2, "users", "page=2&target=owner", operatorClaims()).Body.String()
	if strings.Count(html2, "data-id=") != 1 || !strings.Contains(html2, `data-id="11"`) {
		t.Errorf("page 2 must hold only the 11th user")
	}
	if strings.Contains(html2, "page=3") {
		t.Errorf("no page 3 expected")
	}
	if strings.Count(html2, `hx-get="/selector/users?target=owner"`) != 2 {
		t.Errorf("search box and Previous (to page 1, which omits page) should both point at the page-1 URL")
	}
}

func TestSelector_PageParamIsClamped(t *testing.T) {
	users := make([]*auth.User, 0, 11)
	for i := int64(1); i <= 11; i++ {
		users = append(users, selUser(i, "User", "u@x.test", auth.RoleViewer))
	}
	for raw, wantRows := range map[string]int{
		"page=0": selectorDefaultPageSize, "page=-3": selectorDefaultPageSize,
		"page=abc": selectorDefaultPageSize, "page=1": selectorDefaultPageSize,
		"page=999999": 0, // clamped to the last allowed page, far past the data
	} {
		h, _ := newSelectorTestHandler(users)
		rec := selectorGet(h, "users", raw, operatorClaims())
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d", raw, rec.Code)
		}
		if got := strings.Count(rec.Body.String(), "data-id="); got != wantRows {
			t.Errorf("%s: %d rows, want %d", raw, got, wantRows)
		}
	}
}

func TestSelector_EmptyState(t *testing.T) {
	h, _ := newSelectorTestHandler(nil)
	html := selectorGet(h, "users", "q=zzz", operatorClaims()).Body.String()
	for _, want := range []string{"selector.empty_title", "selector.empty_message"} {
		if !strings.Contains(html, want) {
			t.Errorf("empty state missing %s", want)
		}
	}
	if strings.Contains(html, "data-id=") {
		t.Errorf("no rows expected")
	}
}

func TestSelector_NoUserService_Returns500(t *testing.T) {
	h := &UIHandler{page: NewPageHandler(Config{}), logger: slog.Default()}
	rec := selectorGet(h, "users", "", operatorClaims())
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("got status %d, want 500", rec.Code)
	}
}

// The routing-level checks: the route is registered under requireAuth, and
// has no permission gate (any authenticated role may open a selector).
func TestSelectorRoute_UnauthenticatedRedirectsToLogin(t *testing.T) {
	router, _ := buildRoutingTestRouter(t, true, true)
	req := httptest.NewRequest("GET", "/selector/users", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("got status %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "/login") {
		t.Errorf("Location = %q, want /login...", loc)
	}
}

func TestSelectorRoute_AnyAuthenticatedRoleReachesHandler(t *testing.T) {
	router, sessionMgr := buildRoutingTestRouter(t, true, true)
	for _, role := range auth.ValidRoles() {
		// An unknown key is answered by the handler itself with a 400, which
		// proves the request passed requireAuth and reached Selector.
		req := httptest.NewRequest("GET", "/selector/nope", nil)
		req.AddCookie(sessionCookie(t, sessionMgr, role))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("role %s: got status %d, want 400 from the handler", role, rec.Code)
		}
	}
}

// Every i18n key the Users selector uses exists in all four locales
// (T.* returns the key itself when it is missing).
func TestSelector_UsersI18nKeysExistInAllLocales(t *testing.T) {
	tr, err := i18n.New("en")
	if err != nil {
		t.Fatalf("i18n.New: %v", err)
	}
	keys := []string{
		usersSelectorSource.TitleKey, usersSelectorSource.EmptyTitleKey, usersSelectorSource.EmptyMessageKey,
		"selector.choose", "selector.page", "selector.search.placeholder", "action.previous", "action.next",
	}
	for _, c := range usersSelectorSource.Columns {
		keys = append(keys, c.LabelKey)
	}
	for _, locale := range []string{"en", "es", "pt", "ja"} {
		if !tr.HasLocale(locale) {
			t.Errorf("locale %s not loaded", locale)
			continue
		}
		for _, k := range keys {
			if tr.T(locale, k) == k {
				t.Errorf("locale %s: missing key %s", locale, k)
			}
		}
	}
}
