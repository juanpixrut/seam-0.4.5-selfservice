package overrides

import (
	"context"
	"regexp"
	"sync"
	"testing"

	xolu "github.com/ha1tch/xolu/pkg/client"

	"github.com/ha1tch/seam-ui/internal/auth"
)

// freshXolu answers a SELECT on an entity with no rows the way a real xolu
// does (XOLU-QL004 "entity '<name>' does not exist"); the in-memory fake
// answers it with no rows. See internal/devstate/fresh_xolu_test.go.
type freshXolu struct {
	auth.XOLUClient
	mu      sync.Mutex
	created map[string]bool
}

var freshFromRE = regexp.MustCompile(`(?i)\bFROM\s+([a-z_0-9]+)`)

func (f *freshXolu) OQL(ctx context.Context, q string) (*auth.OQLResult, error) {
	m := freshFromRE.FindStringSubmatch(q)
	f.mu.Lock()
	known := m != nil && f.created[m[1]]
	f.mu.Unlock()
	if !known {
		name := "?"
		if m != nil {
			name = m[1]
		}
		return nil, &xolu.Error{Code: "XOLU-QL004", HTTPStatus: 400, Message: "entity '" + name + "' does not exist"}
	}
	return f.XOLUClient.OQL(ctx, q)
}

func (f *freshXolu) Create(ctx context.Context, et string, d map[string]interface{}) (*auth.XOLUEntity, error) {
	f.mu.Lock()
	f.created[et] = true
	f.mu.Unlock()
	return f.XOLUClient.Create(ctx, et, d)
}

func TestXoluStore_FirstOverridesOnAFreshXolu(t *testing.T) {
	ctx := context.Background()
	s := NewXoluStore(&freshXolu{XOLUClient: newFakeXolu(), created: map[string]bool{}})

	if tenants, err := s.TenantsWithOverrides(ctx); err != nil || len(tenants) != 0 {
		t.Fatalf("TenantsWithOverrides on a fresh xolu = %v, %v", tenants, err)
	}
	if tr, err := s.Translations(ctx, 5); err != nil || len(tr) != 0 {
		t.Fatalf("Translations on a fresh xolu = %v, %v", tr, err)
	}
	if cur, err := s.CurrentSchemas(ctx, 5); err != nil || len(cur) != 0 {
		t.Fatalf("CurrentSchemas on a fresh xolu = %v, %v", cur, err)
	}
	if _, err := s.Fingerprint(ctx); err != nil {
		t.Fatalf("Fingerprint on a fresh xolu: %v", err)
	}
	if err := s.RemoveTranslation(ctx, 5, "en", "action.save"); err != nil {
		t.Errorf("RemoveTranslation of a text never saved = %v", err)
	}
	if err := s.SetTranslation(ctx, Translation{Tenant: 5, Locale: "en", Key: "action.save", Value: "Store"}); err != nil {
		t.Fatalf("the first SetTranslation on a fresh xolu failed: %v", err)
	}
	if tr, err := s.Translations(ctx, 5); err != nil || len(tr) != 1 || tr[0].Value != "Store" {
		t.Errorf("after the first SetTranslation: %+v, %v", tr, err)
	}
	v, err := s.AppendSchema(ctx, SchemaVersion{Tenant: 5, Module: "system", Name: "user_edit", Doc: []byte(`{"type":"object"}`)})
	if err != nil || v.Version != 1 {
		t.Fatalf("the first AppendSchema on a fresh xolu = %+v, %v", v, err)
	}
	if cur, err := s.CurrentSchemas(ctx, 5); err != nil || len(cur) != 1 {
		t.Errorf("after the first AppendSchema: %+v, %v", cur, err)
	}
}
