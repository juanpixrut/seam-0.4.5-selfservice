package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// listUsersFakeClient captures the OQL text sent by XOLUUserStore.ListUsers
// and replies with canned rows. Only OQL is exercised; the other methods
// exist to satisfy XOLUClient.
type listUsersFakeClient struct {
	queries []string
	rows    []map[string]interface{}
}

func (f *listUsersFakeClient) Get(ctx context.Context, entityType string, id int64) (*XOLUEntity, error) {
	return nil, errors.New("not used")
}
func (f *listUsersFakeClient) Create(ctx context.Context, entityType string, data map[string]interface{}) (*XOLUEntity, error) {
	return nil, errors.New("not used")
}
func (f *listUsersFakeClient) Update(ctx context.Context, entityType string, id int64, data map[string]interface{}) (*XOLUEntity, error) {
	return nil, errors.New("not used")
}
func (f *listUsersFakeClient) Delete(ctx context.Context, entityType string, id int64) error {
	return errors.New("not used")
}
func (f *listUsersFakeClient) OQL(ctx context.Context, query string) (*OQLResult, error) {
	f.queries = append(f.queries, query)
	return &OQLResult{Data: f.rows}, nil
}

func fiveUserRows() []map[string]interface{} {
	rows := make([]map[string]interface{}, 0, 5)
	for _, e := range []string{"a@x.test", "b@x.test", "c@x.test", "d@x.test", "e@x.test"} {
		rows = append(rows, map[string]interface{}{"email": e, "name": e, "role": "operator", "active": true})
	}
	return rows
}

// T-95: OQL parses T-SQL. A row cap is "SELECT TOP n", and there is no
// OFFSET keyword. ListUsers must fetch TOP (limit+offset) and drop the
// first Offset rows itself.
func TestListUsers_LimitAndOffsetUseTopAndSlice(t *testing.T) {
	fake := &listUsersFakeClient{rows: fiveUserRows()}
	store := NewXOLUUserStore(fake)

	got, err := store.ListUsers(context.Background(), UserFilter{TenantID: 7, Limit: 3, Offset: 2})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(fake.queries) != 1 {
		t.Fatalf("want 1 query, got %d", len(fake.queries))
	}
	q := fake.queries[0]
	if !strings.Contains(q, "SELECT TOP 5 * FROM users") {
		t.Errorf("want TOP (limit+offset) = 5 in the query, got: %s", q)
	}
	for _, bad := range []string{"LIMIT", "OFFSET", "limit ", "offset "} {
		if strings.Contains(q, bad) {
			t.Errorf("query must not contain %q (not valid OQL): %s", bad, q)
		}
	}
	if !strings.Contains(q, "tenant_id = 7") {
		t.Errorf("tenant condition missing: %s", q)
	}
	// Five rows came back; the first two are dropped in Go.
	if len(got) != 3 || got[0].Email != "c@x.test" || got[2].Email != "e@x.test" {
		t.Errorf("offset slicing wrong: got %d rows", len(got))
	}
}

func TestListUsers_NoLimitHasNoTop(t *testing.T) {
	fake := &listUsersFakeClient{rows: fiveUserRows()}
	store := NewXOLUUserStore(fake)

	got, err := store.ListUsers(context.Background(), UserFilter{})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if strings.Contains(fake.queries[0], "TOP") {
		t.Errorf("no Limit set: query should have no TOP: %s", fake.queries[0])
	}
	if len(got) != 5 {
		t.Errorf("want 5 rows, got %d", len(got))
	}
}

func TestListUsers_OffsetBeyondResultIsEmpty(t *testing.T) {
	fake := &listUsersFakeClient{rows: fiveUserRows()}
	store := NewXOLUUserStore(fake)

	got, err := store.ListUsers(context.Background(), UserFilter{Limit: 10, Offset: 99})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("offset past the end must give no rows, got %d", len(got))
	}
}
