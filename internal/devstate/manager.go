// Package devstate keeps the Developer module's runtime flag state (campaign
// DEV-8): the values an operator sets from the Developer control panel,
// persisted per tenant so they survive logging out and back in as a different
// role.
//
// The rules (DEC-6, DEC-7, DEC-9):
//
//   - State is per tenant, never per session or per user.
//   - Only runtime flags can be changed (see internal/features); static flags
//     are fixed at startup.
//   - A flag pinned at startup by config, environment or SEAM_FEATURES cannot
//     be changed; the panel shows it locked.
//   - While the Developer module is off, stored state is inert: Effective
//     returns the flag's default whatever is stored.
//   - Changing state is only possible for a real tenant (tenant > 0).
package devstate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ha1tch/seam-ui/internal/features"
)

// Errors returned by Manager.Change and Manager.Reset. They are wrapped with
// the flag key, so match them with errors.Is.
var (
	// ErrUnknownFlag: the key is not in the flag catalogue.
	ErrUnknownFlag = errors.New("unknown feature flag")
	// ErrNotRuntime: the flag is fixed at startup and cannot be changed here.
	ErrNotRuntime = errors.New("flag is fixed at startup and cannot be changed at runtime")
	// ErrPinned: startup configuration pins the flag's value.
	ErrPinned = errors.New("flag is pinned by startup configuration")
	// ErrInactive: a prerequisite flag (the Developer module) is off.
	ErrInactive = errors.New("flag is inert while its prerequisite is off")
	// ErrNoTenant: runtime state belongs to a tenant, which must be positive.
	ErrNoTenant = errors.New("a positive tenant ID is required")
)

// Entry is one persisted panel value.
type Entry struct {
	Tenant    int64
	Key       string
	Value     bool
	ChangedBy int64
	ChangedAt time.Time
}

// Store persists panel values.
type Store interface {
	// ListAll returns every tenant's entries.
	ListAll(ctx context.Context) ([]Entry, error)
	// Set creates or replaces the entry for (Tenant, Key).
	Set(ctx context.Context, e Entry) error
	// Delete removes the entry for (tenant, key); absent is not an error.
	Delete(ctx context.Context, tenant int64, key string) error
}

// Manager combines the resolved startup flags with the persisted panel
// values. It is safe for concurrent use.
type Manager struct {
	set   *features.Set
	store Store

	mu    sync.RWMutex
	panel map[int64]map[string]Entry
	subs  []func(tenant int64)

	// lastFP identifies the stored state that panel was last built from, so Sync
	// can tell whether another instance has changed it since.
	lastFP string
}

// NewManager returns a Manager. Call Load before use to read persisted state.
func NewManager(set *features.Set, store Store) *Manager {
	return &Manager{set: set, store: store, panel: make(map[int64]map[string]Entry)}
}

// OnChange registers fn to be called, with the tenant ID, after that tenant's
// effective state may have changed: once per tenant at Load, and after each
// successful Change or Reset. fn runs on the caller's goroutine, outside the
// Manager's lock, so it may call back into the Manager.
func (m *Manager) OnChange(fn func(tenant int64)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subs = append(m.subs, fn)
}

func (m *Manager) notify(tenants ...int64) {
	m.mu.RLock()
	subs := make([]func(int64), len(m.subs))
	copy(subs, m.subs)
	m.mu.RUnlock()
	for _, t := range tenants {
		for _, fn := range subs {
			fn(t)
		}
	}
}

// Load replaces the in-memory state with what the store holds and notifies
// observers of every tenant that has stored state, and of every tenant that had
// state and no longer does, so a switch removed elsewhere is undone here too.
// Entries for flags that are not (or no longer) runtime flags are ignored.
func (m *Manager) Load(ctx context.Context) error {
	entries, err := m.store.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("devstate: load: %w", err)
	}
	m.apply(entries)
	return nil
}

// Sync is Load for a process that is not the only one: it reads the store and
// applies it only when it differs from what this process last applied, which is
// how a change made by another instance is noticed (campaign DEV-28). It
// reports whether it applied anything. A change this process made itself shows
// up as a difference too, and is applied again, which is harmless.
func (m *Manager) Sync(ctx context.Context) (bool, error) {
	entries, err := m.store.ListAll(ctx)
	if err != nil {
		return false, fmt.Errorf("devstate: sync: %w", err)
	}
	m.mu.RLock()
	same := m.lastFP != "" && m.lastFP == fingerprint(entries)
	m.mu.RUnlock()
	if same {
		return false, nil
	}
	m.apply(entries)
	return true, nil
}

// fingerprint identifies a set of stored entries: it changes when any entry is
// added, removed or changed, and not otherwise.
func fingerprint(entries []Entry) string {
	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		lines = append(lines, fmt.Sprintf("%d|%s|%t|%d|%s", e.Tenant, e.Key, e.Value, e.ChangedBy, e.ChangedAt.UTC().Format(time.RFC3339Nano)))
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])
}

func (m *Manager) apply(entries []Entry) {
	next := make(map[int64]map[string]Entry)
	var tenants []int64
	for _, e := range entries {
		f, ok := m.lookup(e.Key)
		if !ok || f.Mode != features.ModeRuntime || e.Tenant <= 0 {
			continue
		}
		if next[e.Tenant] == nil {
			next[e.Tenant] = make(map[string]Entry)
			tenants = append(tenants, e.Tenant)
		}
		next[e.Tenant][e.Key] = e
	}
	m.mu.Lock()
	for t := range m.panel {
		if _, still := next[t]; !still {
			tenants = append(tenants, t) // its state is gone: it must hear of that
		}
	}
	m.panel = next
	m.lastFP = fingerprint(entries)
	m.mu.Unlock()
	sort.Slice(tenants, func(i, j int) bool { return tenants[i] < tenants[j] })
	m.notify(tenants...)
}

func (m *Manager) lookup(key string) (features.Flag, bool) {
	for _, f := range m.set.States() {
		if f.Flag.Key == key {
			return f.Flag, true
		}
	}
	return features.Flag{}, false
}

// Effective returns a runtime flag's value for the tenant and which layer
// decided it (see features.Set.Runtime). It panics for an unknown or static
// key, as that is a programming error.
func (m *Manager) Effective(tenant int64, key string) (bool, features.Source) {
	m.mu.RLock()
	e, ok := m.panel[tenant][key]
	m.mu.RUnlock()
	if ok {
		v := e.Value
		return m.set.Runtime(key, &v)
	}
	return m.set.Runtime(key, nil)
}

// Values is every runtime developer flag for one tenant.
type Values struct {
	HotloadI18N       bool
	HotloadSchemas    bool
	OverlayFilesystem bool
	OverlayXolu       bool
	ShowI18NKeys      bool
}

// Values returns the effective runtime flags for the tenant.
func (m *Manager) Values(tenant int64) Values {
	get := func(k string) bool { v, _ := m.Effective(tenant, k); return v }
	return Values{
		HotloadI18N:       get(features.HotloadI18N),
		HotloadSchemas:    get(features.HotloadSchemas),
		OverlayFilesystem: get(features.OverlayFilesystem),
		OverlayXolu:       get(features.OverlayXolu),
		ShowI18NKeys:      get(features.ShowI18NKeys),
	}
}

// AnyOn reports whether the runtime flag is effectively on for the process's
// default state or for any tenant with stored state. Background pollers use
// it: the filesystem layer is shared, so it is watched while anyone wants it.
func (m *Manager) AnyOn(key string) bool {
	if v, _ := m.Effective(0, key); v {
		return true
	}
	m.mu.RLock()
	tenants := make([]int64, 0, len(m.panel))
	for t := range m.panel {
		tenants = append(tenants, t)
	}
	m.mu.RUnlock()
	for _, t := range tenants {
		if v, _ := m.Effective(t, key); v {
			return true
		}
	}
	return false
}

// check validates that key may be changed for tenant.
func (m *Manager) check(tenant int64, key string) error {
	if tenant <= 0 {
		return fmt.Errorf("%s: %w", key, ErrNoTenant)
	}
	f, ok := m.lookup(key)
	if !ok {
		return fmt.Errorf("%s: %w", key, ErrUnknownFlag)
	}
	if f.Mode != features.ModeRuntime {
		return fmt.Errorf("%s: %w", key, ErrNotRuntime)
	}
	for _, req := range f.Requires {
		if !m.set.On(req) {
			return fmt.Errorf("%s: %w (%s is off)", key, ErrInactive, req)
		}
	}
	if m.set.Pinned(key) {
		return fmt.Errorf("%s: %w (%s)", key, ErrPinned, m.set.Source(key))
	}
	return nil
}

// Change sets a runtime flag for a tenant and persists it. actor is the user
// making the change (recorded in the entry). The store is written first; if
// that fails nothing changes.
func (m *Manager) Change(ctx context.Context, tenant int64, key string, value bool, actor int64) error {
	if err := m.check(tenant, key); err != nil {
		return err
	}
	e := Entry{Tenant: tenant, Key: key, Value: value, ChangedBy: actor, ChangedAt: time.Now().UTC()}
	if err := m.store.Set(ctx, e); err != nil {
		return fmt.Errorf("devstate: save %s: %w", key, err)
	}
	m.mu.Lock()
	if m.panel[tenant] == nil {
		m.panel[tenant] = make(map[string]Entry)
	}
	m.panel[tenant][key] = e
	m.mu.Unlock()
	m.notify(tenant)
	return nil
}

// Reset removes the tenant's stored value, returning the flag to its startup
// value.
func (m *Manager) Reset(ctx context.Context, tenant int64, key string) error {
	if err := m.check(tenant, key); err != nil {
		return err
	}
	if err := m.store.Delete(ctx, tenant, key); err != nil {
		return fmt.Errorf("devstate: reset %s: %w", key, err)
	}
	m.mu.Lock()
	delete(m.panel[tenant], key)
	if len(m.panel[tenant]) == 0 {
		delete(m.panel, tenant)
	}
	m.mu.Unlock()
	m.notify(tenant)
	return nil
}

// FlagView is one runtime flag as the control panel shows it.
type FlagView struct {
	Flag      features.Flag
	Value     bool            // effective value for the tenant
	Source    features.Source // what decided Value
	Locked    bool            // cannot be changed: pinned, or its prerequisite is off
	Pinned    bool
	Inactive  bool // prerequisite off
	ChangedBy int64
	ChangedAt time.Time // zero if never set from the panel
}

// Panel returns every runtime flag, in catalogue order, for the tenant.
func (m *Manager) Panel(tenant int64) []FlagView {
	var out []FlagView
	for _, st := range m.set.States() {
		if st.Flag.Mode != features.ModeRuntime {
			continue
		}
		v, src := m.Effective(tenant, st.Flag.Key)
		fv := FlagView{Flag: st.Flag, Value: v, Source: src, Pinned: st.Pinned}
		for _, req := range st.Flag.Requires {
			if !m.set.On(req) {
				fv.Inactive = true
			}
		}
		fv.Locked = fv.Pinned || fv.Inactive
		m.mu.RLock()
		if e, ok := m.panel[tenant][st.Flag.Key]; ok {
			fv.ChangedBy, fv.ChangedAt = e.ChangedBy, e.ChangedAt
		}
		m.mu.RUnlock()
		out = append(out, fv)
	}
	return out
}
