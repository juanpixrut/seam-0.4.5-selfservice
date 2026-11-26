package auth

import (
	"context"
	"testing"
)

// mergingClient records which of the two ways the store wrote a user.
type mergingClient struct {
	listUsersFakeClient
	patched, replaced int
	data              map[string]interface{}
}

func (f *mergingClient) Patch(ctx context.Context, entityType string, id int64, data map[string]interface{}) (*XOLUEntity, error) {
	f.patched++
	f.data = data
	return &XOLUEntity{ID: id, Data: data}, nil
}

func (f *mergingClient) Update(ctx context.Context, entityType string, id int64, data map[string]interface{}) (*XOLUEntity, error) {
	f.replaced++
	return &XOLUEntity{ID: id, Data: data}, nil
}

// replacingClient cannot merge.
type replacingClient struct {
	listUsersFakeClient
	replaced int
}

func (f *replacingClient) Update(ctx context.Context, entityType string, id int64, data map[string]interface{}) (*XOLUEntity, error) {
	f.replaced++
	return &XOLUEntity{ID: id, Data: data}, nil
}

// A user record carries fields the store does not know; an update (every
// sign-in makes one) must merge, so they stay.
func TestUpdateUser_MergesWhereTheClientCan(t *testing.T) {
	c := &mergingClient{}
	if err := NewXOLUUserStore(c).UpdateUser(context.Background(), &User{ID: 7, Email: "a@example.com", Name: "A"}); err != nil {
		t.Fatal(err)
	}
	if c.patched != 1 || c.replaced != 0 {
		t.Fatalf("patched %d, replaced %d; want a merge and no replacement", c.patched, c.replaced)
	}
	if c.data["email"] != "a@example.com" {
		t.Errorf("merged fields = %v", c.data)
	}
}

func TestUpdateUser_ReplacesWhereItCannot(t *testing.T) {
	c := &replacingClient{}
	if err := NewXOLUUserStore(c).UpdateUser(context.Background(), &User{ID: 7, Email: "a@example.com"}); err != nil {
		t.Fatal(err)
	}
	if c.replaced != 1 {
		t.Fatalf("replaced %d times, want 1", c.replaced)
	}
}
