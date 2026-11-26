package devstate

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ha1tch/seam-ui/internal/features"
)

func syncManager(t *testing.T) (*Manager, *MemoryStore, *[]int64) {
	t.Helper()
	set, err := features.Catalog().Resolve(features.Inputs{AuthEnabled: true,
		LookupEnv: func(n string) (string, bool) { return "1", n == "SEAM_ENABLE_DEVELOPER" }})
	if err != nil {
		t.Fatal(err)
	}
	store := NewMemoryStore()
	m := NewManager(set, store)
	var mu sync.Mutex
	var heard []int64
	m.OnChange(func(tenant int64) { mu.Lock(); heard = append(heard, tenant); mu.Unlock() })
	return m, store, &heard
}

func TestManagerSync_NoticesAnotherInstancesChangesAndRemovals(t *testing.T) {
	ctx := context.Background()
	m, store, heard := syncManager(t)
	if err := m.Load(ctx); err != nil {
		t.Fatal(err)
	}
	if changed, err := m.Sync(ctx); changed || err != nil {
		t.Fatalf("nothing changed: %v %v", changed, err)
	}
	// another instance turns a switch on for tenant 4
	_ = store.Set(ctx, Entry{Tenant: 4, Key: features.ShowI18NKeys, Value: true, ChangedBy: 9, ChangedAt: time.Now()})
	*heard = nil
	if changed, err := m.Sync(ctx); !changed || err != nil {
		t.Fatalf("must notice: %v %v", changed, err)
	}
	if on, _ := m.Effective(4, features.ShowI18NKeys); !on || len(*heard) != 1 || (*heard)[0] != 4 {
		t.Errorf("applied and observers told about tenant 4: on=%v heard=%v", on, *heard)
	}
	if changed, _ := m.Sync(ctx); changed {
		t.Errorf("the same state is not applied twice")
	}
	// the other instance resets it: the tenant now has no stored state at all,
	// and observers must still hear of it, or its overlay switches go stale
	_ = store.Delete(ctx, 4, features.ShowI18NKeys)
	*heard = nil
	if changed, err := m.Sync(ctx); !changed || err != nil {
		t.Fatalf("%v %v", changed, err)
	}
	if on, _ := m.Effective(4, features.ShowI18NKeys); on || len(*heard) != 1 || (*heard)[0] != 4 {
		t.Errorf("a tenant that lost its state must be told: on=%v heard=%v", on, *heard)
	}
}

func TestManagerLoad_NotifiesTenantsThatLostTheirState(t *testing.T) {
	ctx := context.Background()
	m, store, heard := syncManager(t)
	_ = store.Set(ctx, Entry{Tenant: 2, Key: features.ShowI18NKeys, Value: true, ChangedAt: time.Now()})
	_ = m.Load(ctx)
	_ = store.Delete(ctx, 2, features.ShowI18NKeys)
	*heard = nil
	_ = m.Load(ctx)
	if len(*heard) != 1 || (*heard)[0] != 2 {
		t.Errorf("Load must tell a tenant whose state is gone: %v", *heard)
	}
}

type listFails struct{ Store }

func (listFails) ListAll(context.Context) ([]Entry, error) { return nil, errors.New("xolu down") }

func TestManagerSync_StoreErrorIsReported(t *testing.T) {
	set, _ := features.Catalog().Resolve(features.Inputs{AuthEnabled: true})
	if _, err := NewManager(set, listFails{NewMemoryStore()}).Sync(context.Background()); err == nil {
		t.Errorf("an unreadable store must be an error")
	}
}

func TestFingerprint_IgnoresOrderAndSeesEveryField(t *testing.T) {
	at := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	a := Entry{Tenant: 1, Key: "x", Value: true, ChangedBy: 1, ChangedAt: at}
	b := Entry{Tenant: 2, Key: "y", Value: false, ChangedBy: 2, ChangedAt: at}
	if fingerprint([]Entry{a, b}) != fingerprint([]Entry{b, a}) {
		t.Errorf("order must not matter")
	}
	base := fingerprint([]Entry{a})
	for name, e := range map[string]Entry{
		"tenant": {Tenant: 9, Key: "x", Value: true, ChangedBy: 1, ChangedAt: at}, "key": {Tenant: 1, Key: "z", Value: true, ChangedBy: 1, ChangedAt: at},
		"value": {Tenant: 1, Key: "x", Value: false, ChangedBy: 1, ChangedAt: at}, "by": {Tenant: 1, Key: "x", Value: true, ChangedBy: 7, ChangedAt: at},
		"time": {Tenant: 1, Key: "x", Value: true, ChangedBy: 1, ChangedAt: at.Add(time.Nanosecond)},
	} {
		if fingerprint([]Entry{e}) == base {
			t.Errorf("a change of %s must change the fingerprint", name)
		}
	}
	if fingerprint(nil) == base {
		t.Errorf("empty differs from non-empty")
	}
}
