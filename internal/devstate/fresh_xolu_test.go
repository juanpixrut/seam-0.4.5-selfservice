package devstate

import (
	"context"
	"errors"
	"regexp"
	"sync"
	"testing"
	"time"

	xolu "github.com/ha1tch/xolu/pkg/client"

	"github.com/ha1tch/seam-ui/internal/auth"
)

// A real xolu has no "create entity" step: an entity exists from its first
// row, and a SELECT on one with no rows fails with XOLU-QL004 "entity '<name>'
// does not exist". The in-memory fake answers such a query with no rows, which
// is why the first save of a runtime switch on a fresh instance failed in the
// field and passed here (issue report of 2026-10-15). freshXolu answers the way
// xolu does.
type freshXolu struct {
	auth.XOLUClient
	mu      sync.Mutex
	created map[string]bool
}

var fromRE = regexp.MustCompile(`(?i)\bFROM\s+([a-z_]+)`)

func (f *freshXolu) OQL(ctx context.Context, q string) (*auth.OQLResult, error) {
	m := fromRE.FindStringSubmatch(q)
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

func TestXoluStore_FirstSaveOnAFreshXolu(t *testing.T) {
	ctx := context.Background()
	s := NewXoluStore(&freshXolu{XOLUClient: newFakeXolu(), created: map[string]bool{}})

	all, err := s.ListAll(ctx)
	if err != nil || len(all) != 0 {
		t.Fatalf("ListAll on a fresh xolu = %v, %v; want nothing and no error", all, err)
	}
	if err := s.Delete(ctx, 5, "hotload_i18n"); err != nil {
		t.Errorf("Delete of a switch never saved = %v, want nil", err)
	}
	when := time.Date(2026, 10, 15, 15, 57, 51, 0, time.UTC)
	if err := s.Set(ctx, Entry{Tenant: 5, Key: "hotload_i18n", Value: true, ChangedBy: 9, ChangedAt: when}); err != nil {
		t.Fatalf("the first Set on a fresh xolu failed: %v", err)
	}
	if err := s.Set(ctx, Entry{Tenant: 5, Key: "hotload_i18n", Value: false, ChangedBy: 9, ChangedAt: when}); err != nil {
		t.Fatalf("the second Set failed: %v", err)
	}
	all, err = s.ListAll(ctx)
	if err != nil || len(all) != 1 || all[0].Key != "hotload_i18n" || all[0].Value {
		t.Errorf("after two Sets: %+v, %v; want one row, off", all, err)
	}
}

// Only "the entity has no rows yet" is treated as empty: any other failure
// under the same error code still stops the save.
type brokenXolu struct{ auth.XOLUClient }

func (brokenXolu) OQL(ctx context.Context, q string) (*auth.OQLResult, error) {
	return nil, &xolu.Error{Code: "XOLU-QL004", HTTPStatus: 400, Message: "unexpected token near 'key'"}
}

func TestXoluStore_OtherQueryFailuresStillFail(t *testing.T) {
	ctx := context.Background()
	s := NewXoluStore(brokenXolu{newFakeXolu()})
	if _, err := s.ListAll(ctx); err == nil {
		t.Error("ListAll swallowed a query failure")
	}
	err := s.Set(ctx, Entry{Tenant: 5, Key: "hotload_i18n", Value: true})
	var xerr *xolu.Error
	if err == nil || !errors.As(err, &xerr) {
		t.Errorf("Set = %v, want the query failure", err)
	}
}
