package devstate

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/features"
	"github.com/ha1tch/seam-ui/internal/i18n"
	"github.com/ha1tch/seam-ui/internal/schemaoverlay"
)

var ctx = context.Background()

func newSet(t *testing.T, env map[string]string) *features.Set {
	t.Helper()
	set, err := features.Catalog().Resolve(features.Inputs{
		AuthEnabled: true,
		LookupEnv:   func(n string) (string, bool) { v, ok := env[n]; return v, ok },
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	return set
}

var devOn = map[string]string{"SEAM_ENABLE_DEVELOPER": "1"}

func newManager(t *testing.T, env map[string]string, store Store) *Manager {
	t.Helper()
	m := NewManager(newSet(t, env), store)
	if err := m.Load(ctx); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestDefaultsWithDeveloperOn(t *testing.T) {
	m := newManager(t, devOn, NewMemoryStore())
	want := Values{OverlayFilesystem: true, OverlayXolu: true}
	if got := m.Values(1); got != want {
		t.Errorf("Values = %+v, want %+v", got, want)
	}
}

func TestChangePersistsAndSurvivesANewManager(t *testing.T) {
	store := NewMemoryStore()
	m1 := newManager(t, devOn, store)
	if err := m1.Change(ctx, 1, features.HotloadI18N, true, 42); err != nil {
		t.Fatal(err)
	}
	if v, src := m1.Effective(1, features.HotloadI18N); !v || src != features.SourcePanel {
		t.Errorf("after change: %v from %v", v, src)
	}

	// The requirement: a developer logs out, logs in as another role, and the
	// switch is still on. State lives in the store, not in a session; a fresh
	// Manager (a restart) reads it back.
	m2 := newManager(t, devOn, store)
	if v, src := m2.Effective(1, features.HotloadI18N); !v || src != features.SourcePanel {
		t.Errorf("after reload: %v from %v", v, src)
	}
	var fv FlagView
	for _, p := range m2.Panel(1) {
		if p.Flag.Key == features.HotloadI18N {
			fv = p
		}
	}
	if fv.ChangedBy != 42 || fv.ChangedAt.IsZero() {
		t.Errorf("who/when not recorded: %+v", fv)
	}
}

func TestStateIsPerTenant(t *testing.T) {
	m := newManager(t, devOn, NewMemoryStore())
	if err := m.Change(ctx, 1, features.ShowI18NKeys, true, 1); err != nil {
		t.Fatal(err)
	}
	if v, _ := m.Effective(2, features.ShowI18NKeys); v {
		t.Error("tenant 2 saw tenant 1's setting")
	}
	if v, _ := m.Effective(0, features.ShowI18NKeys); v {
		t.Error("the global view saw tenant 1's setting")
	}
}

func TestChangeRules(t *testing.T) {
	m := newManager(t, devOn, NewMemoryStore())
	cases := []struct {
		name   string
		tenant int64
		key    string
		want   error
	}{
		{"no tenant", 0, features.HotloadI18N, ErrNoTenant},
		{"negative tenant", -1, features.HotloadI18N, ErrNoTenant},
		{"unknown flag", 1, "nonsense", ErrUnknownFlag},
		{"static flag", 1, features.Demo, ErrNotRuntime},
		{"the master switch itself", 1, features.Developer, ErrNotRuntime},
	}
	for _, c := range cases {
		err := m.Change(ctx, c.tenant, c.key, true, 1)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}
}

func TestInertWhileDeveloperIsOff(t *testing.T) {
	// State stored while the module was on must do nothing once it is off.
	store := NewMemoryStore()
	on := newManager(t, devOn, store)
	_ = on.Change(ctx, 1, features.HotloadI18N, true, 1)
	_ = on.Change(ctx, 1, features.OverlayXolu, false, 1)

	off := newManager(t, nil, store) // developer off, same persisted rows
	if v, _ := off.Effective(1, features.HotloadI18N); v {
		t.Error("stale hot-load state is live while the module is off")
	}
	if v, _ := off.Effective(1, features.OverlayXolu); !v {
		t.Error("stale 'xolu off' state disabled overrides while the module is off")
	}
	if off.AnyOn(features.HotloadI18N) {
		t.Error("AnyOn must be false while the module is off")
	}
	if err := off.Change(ctx, 1, features.HotloadI18N, true, 1); !errors.Is(err, ErrInactive) {
		t.Errorf("change while off: %v, want ErrInactive", err)
	}
	for _, p := range off.Panel(1) {
		if !p.Inactive || !p.Locked {
			t.Errorf("%s should show as inactive and locked: %+v", p.Flag.Key, p)
		}
	}
}

func TestPinnedFlagsCannotBeChanged(t *testing.T) {
	env := map[string]string{"SEAM_ENABLE_DEVELOPER": "1", features.EnvList: "-" + features.OverlayXolu}
	m := newManager(t, env, NewMemoryStore())
	err := m.Change(ctx, 1, features.OverlayXolu, true, 1)
	if !errors.Is(err, ErrPinned) {
		t.Fatalf("err = %v, want ErrPinned", err)
	}
	if v, src := m.Effective(1, features.OverlayXolu); v || src != features.SourceList {
		t.Errorf("pinned value must hold: %v from %v", v, src)
	}
	for _, p := range m.Panel(1) {
		if p.Flag.Key == features.OverlayXolu && (!p.Pinned || !p.Locked) {
			t.Errorf("panel should show it pinned and locked: %+v", p)
		}
	}
}

func TestResetReturnsToTheStartupValue(t *testing.T) {
	store := NewMemoryStore()
	m := newManager(t, devOn, store)
	_ = m.Change(ctx, 1, features.OverlayXolu, false, 1)
	if v, _ := m.Effective(1, features.OverlayXolu); v {
		t.Fatal("precondition")
	}
	if err := m.Reset(ctx, 1, features.OverlayXolu); err != nil {
		t.Fatal(err)
	}
	if v, src := m.Effective(1, features.OverlayXolu); !v || src != features.SourceDefault {
		t.Errorf("after reset: %v from %v", v, src)
	}
	if left, _ := store.ListAll(ctx); len(left) != 0 {
		t.Errorf("reset should delete the stored row: %v", left)
	}
}

type failingStore struct{ *MemoryStore }

func (failingStore) Set(ctx context.Context, e Entry) error { return errors.New("disk on fire") }

func TestAFailedSaveChangesNothingAndNotifiesNobody(t *testing.T) {
	m := newManager(t, devOn, failingStore{NewMemoryStore()})
	notified := 0
	m.OnChange(func(int64) { notified++ })
	if err := m.Change(ctx, 1, features.HotloadI18N, true, 1); err == nil {
		t.Fatal("expected the store's error")
	}
	if v, _ := m.Effective(1, features.HotloadI18N); v {
		t.Error("the in-memory state changed although the save failed")
	}
	if notified != 0 {
		t.Error("observers were told about a change that did not happen")
	}
}

func TestObserversAreNotifiedAndMayCallBack(t *testing.T) {
	store := NewMemoryStore()
	_ = store.Set(ctx, Entry{Tenant: 3, Key: features.ShowI18NKeys, Value: true})
	_ = store.Set(ctx, Entry{Tenant: 1, Key: features.HotloadI18N, Value: true})

	m := NewManager(newSet(t, devOn), store)
	var seen []int64
	m.OnChange(func(tenant int64) {
		_ = m.Values(tenant) // calling back must not deadlock
		seen = append(seen, tenant)
	})
	if err := m.Load(ctx); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(seen) != "[1 3]" {
		t.Errorf("Load should notify each stored tenant once, in order: %v", seen)
	}
	seen = nil
	_ = m.Change(ctx, 2, features.OverlayXolu, false, 1)
	_ = m.Reset(ctx, 2, features.OverlayXolu)
	if fmt.Sprint(seen) != "[2 2]" {
		t.Errorf("Change and Reset should each notify: %v", seen)
	}
}

func TestLoadIgnoresRowsForNonRuntimeOrUnknownFlags(t *testing.T) {
	store := NewMemoryStore()
	_ = store.Set(ctx, Entry{Tenant: 1, Key: features.Demo, Value: true})  // static
	_ = store.Set(ctx, Entry{Tenant: 1, Key: "retired_flag", Value: true}) // gone from the catalogue
	_ = store.Set(ctx, Entry{Tenant: 0, Key: features.ShowI18NKeys, Value: true})
	m := newManager(t, devOn, store)
	if len(m.Panel(1)) != 8 {
		t.Fatalf("panel should list the 8 runtime flags, got %d", len(m.Panel(1)))
	}
	if m.AnyOn(features.ShowI18NKeys) {
		t.Error("a tenant-0 row must be ignored")
	}
}

func TestAnyOn(t *testing.T) {
	m := newManager(t, devOn, NewMemoryStore())
	if m.AnyOn(features.HotloadI18N) {
		t.Error("nothing is on yet")
	}
	_ = m.Change(ctx, 5, features.HotloadI18N, true, 1)
	if !m.AnyOn(features.HotloadI18N) {
		t.Error("one tenant has it on")
	}
	_ = m.Change(ctx, 5, features.HotloadI18N, false, 1)
	if m.AnyOn(features.HotloadI18N) {
		t.Error("switched back off")
	}
	if !m.AnyOn(features.OverlayXolu) {
		t.Error("a flag that is on by default is on")
	}
}

func TestWireDrivesTranslatorAndResolverOptions(t *testing.T) {
	tr := i18n.NewFromMessages("en", map[string]map[string]string{"en": {"greet": "Hello"}})
	res := schemaoverlay.New(nil)
	m := NewManager(newSet(t, devOn), NewMemoryStore())
	Wire(m, tr, res)
	if err := m.Load(ctx); err != nil {
		t.Fatal(err)
	}

	_ = m.Change(ctx, 7, features.OverlayFilesystem, false, 1)
	_ = m.Change(ctx, 7, features.ShowI18NKeys, true, 1)
	if got := tr.Options(7); !got.DisableFilesystem || got.DisableXolu || !got.ShowKeys {
		t.Errorf("translator options = %+v", got)
	}
	if got := res.Options(7); !got.DisableFilesystem || got.DisableXolu {
		t.Errorf("resolver options = %+v", got)
	}
	if got := tr.ForTenant(7).T("en", "greet"); got != "greet" {
		t.Errorf("show-keys not in effect: %q", got)
	}
	if got := tr.Options(8); got != (i18n.Options{}) {
		t.Errorf("another tenant picked up tenant 7's options: %+v", got)
	}

	_ = m.Reset(ctx, 7, features.OverlayFilesystem)
	_ = m.Reset(ctx, 7, features.ShowI18NKeys)
	if tr.Options(7) != (i18n.Options{}) || res.Options(7) != (schemaoverlay.Options{}) {
		t.Error("resetting everything should restore normal lookups")
	}
	Wire(m, nil, nil) // nil arguments are allowed
	_ = m.Change(ctx, 7, features.OverlayXolu, false, 1)
}

// ---- xolu-backed store, against a fake client ----

type fakeXolu struct {
	mu       sync.Mutex
	rows     map[int64]map[string]interface{}
	nextID   int64
	creates  int
	updates  int
	deletes  int
	queries  []string
	failNext error
}

func newFakeXolu() *fakeXolu { return &fakeXolu{rows: map[int64]map[string]interface{}{}, nextID: 100} }

var topOne = regexp.MustCompile(`tenant_id = (\d+) AND key = '((?:[^']|'')*)'`)

func (f *fakeXolu) OQL(ctx context.Context, q string) (*auth.OQLResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queries = append(f.queries, q)
	if f.failNext != nil {
		err := f.failNext
		f.failNext = nil
		return nil, err
	}
	var data []map[string]interface{}
	if m := topOne.FindStringSubmatch(q); m != nil {
		tenant, _ := strconv.ParseInt(m[1], 10, 64)
		key := strings.ReplaceAll(m[2], "''", "'")
		for id, row := range f.rows {
			if row["tenant_id"] == tenant && row["key"] == key {
				cp := map[string]interface{}{"id": float64(id)} // xolu returns JSON numbers as float64
				for k, v := range row {
					cp[k] = v
				}
				data = append(data, cp)
				break
			}
		}
	} else if strings.HasPrefix(q, "SELECT * FROM "+Entity) {
		for id, row := range f.rows {
			cp := map[string]interface{}{"id": float64(id)}
			for k, v := range row {
				switch x := v.(type) { // mimic JSON round-tripping
				case int64:
					cp[k] = float64(x)
				default:
					cp[k] = v
				}
			}
			data = append(data, cp)
		}
	}
	return &auth.OQLResult{Data: data}, nil
}

func (f *fakeXolu) Create(ctx context.Context, et string, d map[string]interface{}) (*auth.XOLUEntity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.creates++
	f.nextID++
	f.rows[f.nextID] = d
	return &auth.XOLUEntity{ID: f.nextID, Data: d}, nil
}
func (f *fakeXolu) Update(ctx context.Context, et string, id int64, d map[string]interface{}) (*auth.XOLUEntity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updates++
	f.rows[id] = d
	return &auth.XOLUEntity{ID: id, Data: d}, nil
}
func (f *fakeXolu) Delete(ctx context.Context, et string, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletes++
	delete(f.rows, id)
	return nil
}
func (f *fakeXolu) Get(ctx context.Context, et string, id int64) (*auth.XOLUEntity, error) {
	return nil, errors.New("not used")
}

func TestXoluStoreUpsertsOneRowPerTenantAndKey(t *testing.T) {
	fx := newFakeXolu()
	s := NewXoluStore(fx)

	e := Entry{Tenant: 1, Key: features.HotloadI18N, Value: true, ChangedBy: 9, ChangedAt: time.Now().UTC()}
	if err := s.Set(ctx, e); err != nil {
		t.Fatal(err)
	}
	e.Value = false
	if err := s.Set(ctx, e); err != nil { // same (tenant, key): must update, not duplicate
		t.Fatal(err)
	}
	if err := s.Set(ctx, Entry{Tenant: 2, Key: features.HotloadI18N, Value: true}); err != nil {
		t.Fatal(err)
	}
	if fx.creates != 2 || fx.updates != 1 || len(fx.rows) != 2 {
		t.Errorf("creates=%d updates=%d rows=%d, want 2/1/2", fx.creates, fx.updates, len(fx.rows))
	}

	all, err := s.ListAll(ctx)
	if err != nil || len(all) != 2 {
		t.Fatalf("ListAll = %v, %v", all, err)
	}
	byTenant := map[int64]Entry{}
	for _, x := range all {
		byTenant[x.Tenant] = x
	}
	if byTenant[1].Value || byTenant[1].ChangedBy != 9 || byTenant[1].ChangedAt.IsZero() || !byTenant[2].Value {
		t.Errorf("round trip lost data: %+v", byTenant)
	}

	if err := s.Delete(ctx, 1, features.HotloadI18N); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, 1, features.HotloadI18N); err != nil { // already gone: fine
		t.Fatal(err)
	}
	if fx.deletes != 1 || len(fx.rows) != 1 {
		t.Errorf("deletes=%d rows=%d", fx.deletes, len(fx.rows))
	}
}

func TestXoluStoreEscapesTheKeyInQueries(t *testing.T) {
	fx := newFakeXolu()
	s := NewXoluStore(fx)
	evil := "x' OR tenant_id > 0 --"
	_ = s.Set(ctx, Entry{Tenant: 1, Key: evil, Value: true})
	for _, q := range fx.queries {
		if strings.Contains(q, "key = 'x' OR") {
			t.Errorf("unescaped key reached the query: %s", q)
		}
	}
	if len(fx.rows) != 1 {
		t.Errorf("the quote must be stored literally, not interpreted: %v", fx.rows)
	}
}

func TestXoluStoreReportsClientErrors(t *testing.T) {
	fx := newFakeXolu()
	s := NewXoluStore(fx)
	fx.failNext = errors.New("xolu down")
	if err := s.Set(ctx, Entry{Tenant: 1, Key: "k"}); err == nil {
		t.Error("Set must surface a failed lookup")
	}
	fx.failNext = errors.New("xolu down")
	if _, err := s.ListAll(ctx); err == nil {
		t.Error("ListAll must surface a failed query")
	}
}

func TestManagerWithXoluStoreEndToEnd(t *testing.T) {
	fx := newFakeXolu()
	m1 := newManager(t, devOn, NewXoluStore(fx))
	if err := m1.Change(ctx, 4, features.HotloadSchemas, true, 7); err != nil {
		t.Fatal(err)
	}
	m2 := newManager(t, devOn, NewXoluStore(fx)) // "restart"
	if v, src := m2.Effective(4, features.HotloadSchemas); !v || src != features.SourcePanel {
		t.Errorf("state did not survive a restart through xolu: %v from %v", v, src)
	}
}

func TestAsHelpers(t *testing.T) {
	if asInt64(float64(7)) != 7 || asInt64("8") != 8 || asInt64(int(9)) != 9 || asInt64(nil) != 0 || asInt64(int64(10)) != 10 {
		t.Error("asInt64")
	}
	if !asBool(true) || !asBool("true") || !asBool("1") || !asBool(float64(1)) || asBool("no") || asBool(nil) || asBool(float64(0)) {
		t.Error("asBool")
	}
}
