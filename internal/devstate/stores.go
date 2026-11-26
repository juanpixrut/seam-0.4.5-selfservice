package devstate

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ha1tch/seam-ui/internal/auth"
)

// Entity is the xolu entity that holds the panel values.
const Entity = "feature_state"

// ---- In-memory store (tests, and runs without xolu) ----

// MemoryStore keeps entries in memory.
type MemoryStore struct {
	mu sync.Mutex
	m  map[string]Entry
}

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore() *MemoryStore { return &MemoryStore{m: make(map[string]Entry)} }

func memKey(tenant int64, key string) string { return strconv.FormatInt(tenant, 10) + "/" + key }

// ListAll implements Store.
func (s *MemoryStore) ListAll(ctx context.Context) ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Entry, 0, len(s.m))
	for _, e := range s.m {
		out = append(out, e)
	}
	return out, nil
}

// Set implements Store.
func (s *MemoryStore) Set(ctx context.Context, e Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[memKey(e.Tenant, e.Key)] = e
	return nil
}

// Delete implements Store.
func (s *MemoryStore) Delete(ctx context.Context, tenant int64, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, memKey(tenant, key))
	return nil
}

// ---- xolu-backed store ----

// XoluStore persists entries in the feature_state entity, one row per
// (tenant_id, key): {tenant_id, key, value, changed_by, changed_at}. It uses
// the same client interface as the user store, which server.go already
// adapts from the real xolu client.
type XoluStore struct {
	client auth.XOLUClient
}

// NewXoluStore returns an XoluStore.
func NewXoluStore(client auth.XOLUClient) *XoluStore { return &XoluStore{client: client} }

func oqlQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func (s *XoluStore) find(ctx context.Context, tenant int64, key string) (int64, bool, error) {
	res, err := s.client.OQL(ctx, fmt.Sprintf(
		"SELECT TOP 1 * FROM %s WHERE tenant_id = %d AND key = %s", Entity, tenant, oqlQuote(key)))
	if auth.EntityNotYetCreated(err) {
		// No switch has ever been saved on this xolu: the entity appears
		// with its first row, so "does not exist" means "not found".
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if res == nil || len(res.Data) == 0 {
		return 0, false, nil
	}
	return asInt64(res.Data[0]["id"]), true, nil
}

// ListAll implements Store.
func (s *XoluStore) ListAll(ctx context.Context) ([]Entry, error) {
	res, err := s.client.OQL(ctx, "SELECT * FROM "+Entity)
	if auth.EntityNotYetCreated(err) {
		return nil, nil // nothing saved yet
	}
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}
	out := make([]Entry, 0, len(res.Data))
	for _, row := range res.Data {
		key, _ := row["key"].(string)
		if key == "" {
			continue
		}
		e := Entry{
			Tenant:    asInt64(row["tenant_id"]),
			Key:       key,
			Value:     asBool(row["value"]),
			ChangedBy: asInt64(row["changed_by"]),
		}
		if ts, _ := row["changed_at"].(string); ts != "" {
			e.ChangedAt, _ = time.Parse(time.RFC3339, ts)
		}
		out = append(out, e)
	}
	return out, nil
}

// Set implements Store: update the row if one exists, otherwise create it.
func (s *XoluStore) Set(ctx context.Context, e Entry) error {
	data := map[string]interface{}{
		"tenant_id":  e.Tenant,
		"key":        e.Key,
		"value":      e.Value,
		"changed_by": e.ChangedBy,
		"changed_at": e.ChangedAt.UTC().Format(time.RFC3339),
	}
	id, found, err := s.find(ctx, e.Tenant, e.Key)
	if err != nil {
		return err
	}
	if found {
		_, err = s.client.Update(ctx, Entity, id, data)
		return err
	}
	_, err = s.client.Create(ctx, Entity, data)
	return err
}

// Delete implements Store.
func (s *XoluStore) Delete(ctx context.Context, tenant int64, key string) error {
	id, found, err := s.find(ctx, tenant, key)
	if err != nil || !found {
		return err
	}
	return s.client.Delete(ctx, Entity, id)
}

func asInt64(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	case string:
		i, _ := strconv.ParseInt(n, 10, 64)
		return i
	}
	return 0
}

func asBool(v interface{}) bool {
	switch b := v.(type) {
	case bool:
		return b
	case string:
		return b == "true" || b == "1"
	case float64:
		return b != 0
	case int:
		return b != 0
	case int64:
		return b != 0
	}
	return false
}
