package auth

import (
	"context"
	"strings"
	"testing"
)

// T-96: the Users selector lists only users whose role is at or below the
// caller's, and supports a name/email search. Both are expressed as OQL
// conditions built by XOLUUserStore.ListUsers.

func TestListUsers_RolesBecomeParenthesisedOr(t *testing.T) {
	fake := &listUsersFakeClient{rows: fiveUserRows()}
	store := NewXOLUUserStore(fake)

	_, err := store.ListUsers(context.Background(), UserFilter{
		TenantID: 3,
		Roles:    []string{RoleOperator, RoleViewer},
	})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	q := fake.queries[0]
	if !strings.Contains(q, "(role = 'operator' OR role = 'viewer')") {
		t.Errorf("want parenthesised OR of roles, got: %s", q)
	}
	if !strings.Contains(q, "tenant_id = 3 AND (role = 'operator' OR role = 'viewer')") {
		t.Errorf("roles condition must be ANDed with the tenant condition: %s", q)
	}
}

// A non-nil empty Roles means "match nothing": no query is sent at all.
func TestListUsers_EmptyNonNilRolesReturnsNothingWithoutQuery(t *testing.T) {
	fake := &listUsersFakeClient{rows: fiveUserRows()}
	store := NewXOLUUserStore(fake)

	got, err := store.ListUsers(context.Background(), UserFilter{TenantID: 3, Roles: RolesAtOrBelow("")})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want no users, got %d", len(got))
	}
	if len(fake.queries) != 0 {
		t.Errorf("no query should be sent for an empty role set, got: %v", fake.queries)
	}
}

// nil Roles is "no role restriction" (unchanged behaviour for existing
// callers).
func TestListUsers_NilRolesHasNoRoleCondition(t *testing.T) {
	fake := &listUsersFakeClient{rows: fiveUserRows()}
	store := NewXOLUUserStore(fake)

	if _, err := store.ListUsers(context.Background(), UserFilter{TenantID: 3}); err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if strings.Contains(fake.queries[0], "role") {
		t.Errorf("no Roles set: query should have no role condition: %s", fake.queries[0])
	}
}

func TestListUsers_RolesAreEscaped(t *testing.T) {
	fake := &listUsersFakeClient{rows: fiveUserRows()}
	store := NewXOLUUserStore(fake)

	if _, err := store.ListUsers(context.Background(), UserFilter{Roles: []string{"x' OR '1'='1"}}); err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if !strings.Contains(fake.queries[0], "role = 'x'' OR ''1''=''1'") {
		t.Errorf("role value must have its quotes doubled: %s", fake.queries[0])
	}
}

func TestListUsers_SearchMatchesNameOrEmail(t *testing.T) {
	fake := &listUsersFakeClient{rows: fiveUserRows()}
	store := NewXOLUUserStore(fake)

	if _, err := store.ListUsers(context.Background(), UserFilter{TenantID: 3, Search: "  ana "}); err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	q := fake.queries[0]
	if !strings.Contains(q, "(name LIKE '%ana%' OR email LIKE '%ana%')") {
		t.Errorf("want name/email LIKE with the trimmed term, got: %s", q)
	}
}

func TestListUsers_SearchIsEscaped(t *testing.T) {
	fake := &listUsersFakeClient{rows: fiveUserRows()}
	store := NewXOLUUserStore(fake)

	if _, err := store.ListUsers(context.Background(), UserFilter{Search: "x' OR '1'='1"}); err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	q := fake.queries[0]
	if !strings.Contains(q, "name LIKE '%x'' OR ''1''=''1%'") {
		t.Errorf("search term must have its quotes doubled: %s", q)
	}
	if strings.Contains(q, "'x' OR") {
		t.Errorf("search term broke out of its string literal: %s", q)
	}
}

func TestListUsers_BlankSearchHasNoLike(t *testing.T) {
	fake := &listUsersFakeClient{rows: fiveUserRows()}
	store := NewXOLUUserStore(fake)

	if _, err := store.ListUsers(context.Background(), UserFilter{Search: "   "}); err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if strings.Contains(fake.queries[0], "LIKE") {
		t.Errorf("blank search must add no LIKE condition: %s", fake.queries[0])
	}
}

// Roles, search, active, TOP and ORDER BY compose into one valid shape.
func TestListUsers_AllConditionsCompose(t *testing.T) {
	fake := &listUsersFakeClient{rows: fiveUserRows()}
	store := NewXOLUUserStore(fake)
	active := true

	_, err := store.ListUsers(context.Background(), UserFilter{
		TenantID: 9, Roles: []string{RoleViewer}, Search: "q", Active: &active, Limit: 11, Offset: 10,
	})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	want := "SELECT TOP 21 * FROM users WHERE tenant_id = 9 AND (role = 'viewer') AND (name LIKE '%q%' OR email LIKE '%q%') AND active = true ORDER BY name ASC"
	if fake.queries[0] != want {
		t.Errorf("query shape:\n got: %s\nwant: %s", fake.queries[0], want)
	}
}
