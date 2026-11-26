package overrides

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ha1tch/seam-ui/internal/formengine"
	"github.com/ha1tch/seam-ui/internal/i18n"
	"github.com/ha1tch/seam-ui/internal/schemaoverlay"
)

const (
	baseDoc = `{"type":"object","properties":{"base":{"type":"string","title":"Base","x-seam-input":"text"}}}`
	docA    = `{"type":"object","properties":{"name":{"type":"string","title":"Name","x-seam-input":"text"}}}`
	docB    = `{"type":"object","properties":{"code":{"type":"string","title":"Code","x-seam-input":"text"}}}`
)

type fixture struct {
	svc   *Service
	store Store
	tr    *i18n.Translator
	res   *schemaoverlay.Resolver
	valid *atomic.Bool // the resolver's validator accepts documents while true
}

func newFixture(t *testing.T, store Store) *fixture {
	t.Helper()
	if store == nil {
		store = NewMemoryStore()
	}
	valid := &atomic.Bool{}
	valid.Store(true)
	tr := i18n.NewFromMessages("en", map[string]map[string]string{
		"en": {"greet": "Hello %s", "plain": "Plain"}, "es": {"plain": "Llano"},
	})
	res := schemaoverlay.New(func(b []byte) error {
		if !valid.Load() {
			return errors.New("the engine no longer accepts this")
		}
		_, err := formengine.ParseSchema(b)
		return err
	})
	return &fixture{svc: NewService(store, tr, res), store: store, tr: tr, res: res, valid: valid}
}

func (f *fixture) text(tenant int64, locale, key string, args ...any) string {
	return f.tr.ForTenant(tenant).ForLocale(locale)(key, args...)
}

func sameJSON(a, b []byte) bool {
	var x, y interface{}
	return json.Unmarshal(a, &x) == nil && json.Unmarshal(b, &y) == nil && reflect.DeepEqual(x, y)
}

func (f *fixture) resolved(tenant int64, module, name string) []byte {
	return f.res.Resolve(tenant, module, name, []byte(baseDoc))
}

func TestService_TranslationReachesOnlyItsTenant(t *testing.T) {
	f := newFixture(t, nil)
	must(t, f.svc.SetTranslation(ctx, 3, "en", "greet", "Hi %s", "Hello %s", 9))
	if got := f.text(3, "en", "greet", "Bob"); got != "Hi Bob" {
		t.Errorf("tenant 3 sees its override, got %q", got)
	}
	if got := f.text(4, "en", "greet", "Bob"); got != "Hello Bob" {
		t.Errorf("another tenant is unaffected, got %q", got)
	}
	if got := f.tr.ForLocale("en")("greet", "Bob"); got != "Hello Bob" {
		t.Errorf("the global lookup is unaffected, got %q", got)
	}
	rows, _ := f.svc.Translations(ctx, 3)
	if len(rows) != 1 || rows[0].ChangedBy != 9 || rows[0].ChangedAt.IsZero() {
		t.Errorf("the change is stored with who and when: %+v", rows)
	}
	must(t, f.svc.RemoveTranslation(ctx, 3, "en", "greet", 9))
	if got := f.text(3, "en", "greet", "Bob"); got != "Hello Bob" {
		t.Errorf("removing the override restores the original, got %q", got)
	}
}

func TestService_TranslationValidationStoresNothing(t *testing.T) {
	f := newFixture(t, nil)
	cases := []struct {
		name                    string
		tenant                  int64
		locale, key, value, ref string
		want                    error
	}{
		{"no tenant", 0, "en", "plain", "x", "", ErrNoTenant},
		{"negative tenant", -3, "en", "plain", "x", "", ErrNoTenant},
		{"bad locale", 3, "EN", "plain", "x", "", i18n.ErrInvalid},
		{"locale with a path", 3, "../en", "plain", "x", "", i18n.ErrInvalid},
		{"bad key", 3, "en", "no spaces allowed", "x", "", i18n.ErrInvalid},
		{"empty text", 3, "en", "plain", "   ", "", i18n.ErrInvalid},
		{"control character", 3, "en", "plain", "a\x00b", "", i18n.ErrInvalid},
		{"too long", 3, "en", "plain", strings.Repeat("x", i18n.MaxOverrideLen+1), "", i18n.ErrInvalid},
		{"placeholder missing", 3, "en", "greet", "Hi", "Hello %s", i18n.ErrVerbs},
		{"placeholder added", 3, "en", "greet", "Hi %s %s", "Hello %s", i18n.ErrVerbs},
	}
	for _, c := range cases {
		err := f.svc.SetTranslation(ctx, c.tenant, c.locale, c.key, c.value, c.ref, 1)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, err, c.want)
		}
	}
	if rows, _ := f.store.Translations(ctx, 3); len(rows) != 0 {
		t.Errorf("a refused text must never reach the store: %+v", rows)
	}
	if err := f.svc.RemoveTranslation(ctx, 3, "../x", "plain", 9); !errors.Is(err, i18n.ErrInvalid) {
		t.Errorf("removal validates its names too: %v", err)
	}
	if err := f.svc.RemoveTranslation(ctx, 0, "en", "plain", 9); !errors.Is(err, ErrNoTenant) {
		t.Errorf("and the tenant: %v", err)
	}
}

func TestService_SaveSchemaAppliesAndVersions(t *testing.T) {
	f := newFixture(t, nil)
	v1, _, err := f.svc.SaveSchema(ctx, 3, "asset", "create", []byte(docA), []byte(baseDoc), 9, "first try")
	must(t, err)
	if v1.Version != 1 || v1.Note != "first try" || v1.ChangedBy != 9 {
		t.Errorf("version 1 as saved: %+v", v1)
	}
	if !sameJSON(f.resolved(3, "asset", "create"), []byte(docA)) {
		t.Errorf("tenant 3 resolves its override")
	}
	if !sameJSON(f.resolved(4, "asset", "create"), []byte(baseDoc)) || !sameJSON(f.resolved(3, "asset", "update"), []byte(baseDoc)) {
		t.Errorf("other tenants and other forms resolve the built-in definition")
	}
	v2, _, _ := f.svc.SaveSchema(ctx, 3, "asset", "create", []byte(docB), []byte(baseDoc), 9, "")
	if v2.Version != 2 || !sameJSON(f.resolved(3, "asset", "create"), []byte(docB)) {
		t.Errorf("a second save is version 2 and replaces the first: %+v", v2)
	}
	if !bytes.HasSuffix(v2.Doc, []byte("}\n")) || !bytes.Contains(v2.Doc, []byte("\n  ")) {
		t.Errorf("the stored document is indented for reading: %q", v2.Doc)
	}
	cur, ok, _ := f.svc.Current(ctx, 3, "asset", "create")
	if !ok || cur.Version != 2 {
		t.Errorf("Current is the newest: %+v %v", cur, ok)
	}
	if _, ok, _ := f.svc.Current(ctx, 4, "asset", "create"); ok {
		t.Errorf("another tenant has no override")
	}
}

func TestService_RefusedDocumentsStoreNothing(t *testing.T) {
	f := newFixture(t, nil)
	huge := append([]byte(`{"x":"`), append(bytes.Repeat([]byte("a"), 1<<20), []byte(`"}`)...)...)
	cases := []struct {
		name          string
		tenant        int64
		module, name2 string
		doc           []byte
		want          error
	}{
		{"not json", 3, "asset", "create", []byte("not json"), schemaoverlay.ErrRejected},
		{"empty", 3, "asset", "create", nil, schemaoverlay.ErrRejected},
		{"not a form schema", 3, "asset", "create", []byte(`{"type":"object","properties":{"a":{"type":"nonsense"}}}`), schemaoverlay.ErrRejected},
		{"over a megabyte", 3, "asset", "create", huge, schemaoverlay.ErrRejected},
		{"a module that is a path", 3, "../asset", "create", []byte(docA), schemaoverlay.ErrInvalidName},
		{"a name with a quote", 3, "asset", "cr'eate", []byte(docA), schemaoverlay.ErrInvalidName},
		{"an upper-case name", 3, "Asset", "create", []byte(docA), schemaoverlay.ErrInvalidName},
		{"no tenant", 0, "asset", "create", []byte(docA), ErrNoTenant},
	}
	for _, c := range cases {
		if _, _, err := f.svc.SaveSchema(ctx, c.tenant, c.module, c.name2, c.doc, []byte(baseDoc), 1, ""); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, err, c.want)
		}
	}
	if ts, _ := f.store.TenantsWithOverrides(ctx); len(ts) != 0 {
		t.Errorf("a refused document must never reach the store: tenants %v", ts)
	}
}

func TestService_RemoveAndRestore(t *testing.T) {
	f := newFixture(t, nil)
	f.svc.SaveSchema(ctx, 3, "asset", "create", []byte(docA), nil, 1, "A")
	f.svc.SaveSchema(ctx, 3, "asset", "create", []byte(docB), nil, 1, "B")
	must(t, f.svc.RemoveSchema(ctx, 3, "asset", "create", 1))
	if !sameJSON(f.resolved(3, "asset", "create"), []byte(baseDoc)) {
		t.Errorf("after a removal the built-in definition is back")
	}
	must(t, f.svc.RemoveSchema(ctx, 3, "asset", "create", 1)) // nothing to remove
	must(t, f.svc.RemoveSchema(ctx, 3, "asset", "never_overridden", 1))
	if h, _ := f.svc.History(ctx, 3, "asset", "create"); len(h) != 3 || !h[0].Removed {
		t.Errorf("removing twice must not add a version (history %d, newest removed %v)", len(h), h[0].Removed)
	}
	if h, _ := f.svc.History(ctx, 3, "asset", "never_overridden"); len(h) != 0 {
		t.Errorf("removing what was never overridden writes nothing")
	}
	v, err := f.svc.RestoreSchema(ctx, 3, "asset", "create", 1, 5)
	must(t, err)
	if v.Version != 4 || v.Note != "restored from version 1" || v.ChangedBy != 5 {
		t.Errorf("a restore is a new version that says where it came from: %+v", v)
	}
	if !sameJSON(f.resolved(3, "asset", "create"), []byte(docA)) {
		t.Errorf("version 1 is live again")
	}
	if _, err := f.svc.RestoreSchema(ctx, 3, "asset", "create", 3, 5); !errors.Is(err, ErrNotRestorable) {
		t.Errorf("a removal has nothing to restore: %v", err)
	}
	if _, err := f.svc.RestoreSchema(ctx, 3, "asset", "create", 99, 5); !errors.Is(err, ErrUnknownVersion) {
		t.Errorf("an unknown version: %v", err)
	}
	if _, err := f.svc.RestoreSchema(ctx, 4, "asset", "create", 1, 5); !errors.Is(err, ErrUnknownVersion) {
		t.Errorf("another tenant's history is not visible: %v", err)
	}
	if _, err := f.svc.RestoreSchema(ctx, 0, "asset", "create", 1, 5); !errors.Is(err, ErrNoTenant) {
		t.Errorf("no tenant: %v", err)
	}
	// the engine can change under an old definition: it is checked again
	f.valid.Store(false)
	before, _ := f.svc.History(ctx, 3, "asset", "create")
	if _, err := f.svc.RestoreSchema(ctx, 3, "asset", "create", 2, 5); !errors.Is(err, schemaoverlay.ErrRejected) {
		t.Errorf("a definition the engine now rejects must not come back: %v", err)
	}
	if after, _ := f.svc.History(ctx, 3, "asset", "create"); len(after) != len(before) {
		t.Errorf("a refused restore adds no version")
	}
}

func TestService_LoadAppliesStoredOverridesAndSurvivesABadTenant(t *testing.T) {
	store := NewMemoryStore()
	a := newFixture(t, store)
	must(t, a.svc.SetTranslation(ctx, 3, "en", "plain", "Plain 3", "", 1))
	must(t, a.svc.SetTranslation(ctx, 5, "en", "plain", "Plain 5", "", 1))
	_, _, err := a.svc.SaveSchema(ctx, 3, "asset", "create", []byte(docA), nil, 1, "")
	must(t, err)
	// tenant 7 has a stored document the engine does not accept (an old row, say)
	_, _ = store.AppendSchema(ctx, SchemaVersion{Tenant: 7, Module: "asset", Name: "create", Doc: []byte(`{"type":"object","properties":{"a":{"type":"nonsense"}}}`)})

	b := newFixture(t, store) // a fresh process over the same store
	err = b.svc.Load(ctx)
	if err == nil || !strings.Contains(err.Error(), "tenant 7") {
		t.Errorf("the bad tenant is reported by number: %v", err)
	}
	if b.text(3, "en", "plain") != "Plain 3" || b.text(5, "en", "plain") != "Plain 5" {
		t.Errorf("translations are live after start")
	}
	if !sameJSON(b.resolved(3, "asset", "create"), []byte(docA)) {
		t.Errorf("form definitions are live after start, in spite of tenant 7")
	}
	if !sameJSON(b.resolved(7, "asset", "create"), []byte(baseDoc)) {
		t.Errorf("the bad tenant falls back to the built-in definition")
	}
}

func TestService_ConcurrentSavesNumberEveryVersionOnceAndEndConsistent(t *testing.T) {
	f := newFixture(t, nil)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			doc := docA
			if i%2 == 1 {
				doc = docB
			}
			if _, _, err := f.svc.SaveSchema(ctx, 3, "asset", "create", []byte(doc), nil, int64(i), ""); err != nil {
				t.Errorf("save %d: %v", i, err)
			}
			_ = f.svc.SetTranslation(ctx, 3, "en", "plain", "P"+strings.Repeat("x", i), "", 1)
		}(i)
	}
	wg.Wait()
	h, _ := f.svc.History(ctx, 3, "asset", "create")
	seen := map[int]bool{}
	for _, v := range h {
		seen[v.Version] = true
	}
	if len(h) != 20 || len(seen) != 20 || h[0].Version != 20 {
		t.Errorf("20 saves, versions 1..20 once each: %d rows, %d distinct", len(h), len(seen))
	}
	if !sameJSON(f.resolved(3, "asset", "create"), h[0].Doc) {
		t.Errorf("what is live is the newest stored version")
	}
	rows, _ := f.svc.Translations(ctx, 3)
	if len(rows) != 1 || f.text(3, "en", "plain") != rows[0].Value {
		t.Errorf("what is live is what is stored")
	}
}

type context_ = context.Context

type failingStore struct {
	Store
	failSet, failAppend bool
}

func (s failingStore) SetTranslation(c context_, t Translation) error {
	if s.failSet {
		return errors.New("disk on fire")
	}
	return s.Store.SetTranslation(c, t)
}

func (s failingStore) AppendSchema(c context_, v SchemaVersion) (SchemaVersion, error) {
	if s.failAppend {
		return SchemaVersion{}, errors.New("disk on fire")
	}
	return s.Store.AppendSchema(c, v)
}

func TestService_StoreFailureLeavesTheLiveLayersAlone(t *testing.T) {
	f := newFixture(t, failingStore{Store: NewMemoryStore(), failSet: true, failAppend: true})
	if err := f.svc.SetTranslation(ctx, 3, "en", "plain", "Changed", "", 1); err == nil || !strings.Contains(err.Error(), "disk") {
		t.Errorf("the store's error reaches the caller: %v", err)
	}
	if got := f.text(3, "en", "plain"); got != "Plain" {
		t.Errorf("nothing is applied that was not stored, got %q", got)
	}
	if _, _, err := f.svc.SaveSchema(ctx, 3, "asset", "create", []byte(docA), nil, 1, ""); err == nil {
		t.Errorf("a failed append is a failed save")
	}
	if !sameJSON(f.resolved(3, "asset", "create"), []byte(baseDoc)) {
		t.Errorf("a failed save leaves the built-in definition")
	}
}

func TestService_NotesAreCleaned(t *testing.T) {
	f := newFixture(t, nil)
	v, _, err := f.svc.SaveSchema(ctx, 3, "asset", "create", []byte(docA), nil, 1, "  line\x00 one\r\n\x1b[31mred  ")
	must(t, err)
	if strings.ContainsAny(v.Note, "\x00\r\n\x1b") || strings.TrimSpace(v.Note) != v.Note {
		t.Errorf("control characters and padding are removed from a note: %q", v.Note)
	}
	long, _, _ := f.svc.SaveSchema(ctx, 3, "asset", "create", []byte(docA), nil, 1, strings.Repeat("é", 500))
	if n := len([]rune(long.Note)); n != MaxNoteLen {
		t.Errorf("a note is cut to %d characters, got %d", MaxNoteLen, n)
	}
}

func TestService_HistoryIsTrimmedNewestFirst(t *testing.T) {
	f := newFixture(t, nil)
	for i := 0; i < MaxHistory+10; i++ {
		_, _, _ = f.svc.SaveSchema(ctx, 3, "asset", "create", []byte(docA), nil, 1, "")
	}
	h, _ := f.svc.History(ctx, 3, "asset", "create")
	if len(h) != MaxHistory || h[0].Version != MaxHistory+10 {
		t.Errorf("the newest %d: got %d rows, newest %d", MaxHistory, len(h), h[0].Version)
	}
}

func TestService_WithoutResolverOrTranslatorItStillStoresSafely(t *testing.T) {
	s := NewService(NewMemoryStore(), nil, nil)
	must(t, s.SetTranslation(ctx, 3, "en", "plain", "x", "", 1))
	if _, _, err := s.SaveSchema(ctx, 3, "a", "b", []byte(docA), nil, 1, ""); err == nil {
		t.Errorf("a schema cannot be validated without a resolver, so it is refused")
	}
	must(t, s.Load(ctx))
}

// Two services over one store are two instances: one's change reaches the other
// at its next Sync, and only then.
func TestService_SyncAppliesAnotherInstancesChanges(t *testing.T) {
	store := NewMemoryStore()
	a, b := newFixture(t, store), newFixture(t, store)
	must(t, b.svc.Load(ctx))
	if changed, err := b.svc.Sync(ctx); changed || err != nil {
		t.Fatalf("nothing changed yet: %v %v", changed, err)
	}
	must(t, a.svc.SetTranslation(ctx, 3, "en", "plain", "From A", "", 1))
	_, _, err := a.svc.SaveSchema(ctx, 3, "asset", "create", []byte(docA), nil, 1, "")
	must(t, err)
	if b.text(3, "en", "plain") != "Plain" {
		t.Fatalf("B must not see it before it syncs")
	}
	if changed, err := b.svc.Sync(ctx); !changed || err != nil {
		t.Fatalf("B must notice A's changes: %v %v", changed, err)
	}
	if b.text(3, "en", "plain") != "From A" || !sameJSON(b.resolved(3, "asset", "create"), []byte(docA)) {
		t.Errorf("B must now apply them")
	}
	if changed, _ := b.svc.Sync(ctx); changed {
		t.Errorf("and a second Sync with nothing new does nothing")
	}
	// A removes tenant 3's last overrides: B must clear the tenant, not keep it
	must(t, a.svc.RemoveTranslation(ctx, 3, "en", "plain", 9))
	must(t, a.svc.RemoveSchema(ctx, 3, "asset", "create", 1))
	if changed, err := b.svc.Sync(ctx); !changed || err != nil {
		t.Fatalf("%v %v", changed, err)
	}
	if b.text(3, "en", "plain") != "Plain" || !sameJSON(b.resolved(3, "asset", "create"), []byte(baseDoc)) {
		t.Errorf("a tenant whose overrides are all gone must be cleared on the other instance too")
	}
}

type fingerprintFails struct{ Store }

func (fingerprintFails) Fingerprint(context_) (string, error) { return "", errors.New("xolu down") }

func TestService_SyncErrorsAndRepeatedFailures(t *testing.T) {
	f := newFixture(t, fingerprintFails{NewMemoryStore()})
	if _, err := f.svc.Sync(ctx); err == nil {
		t.Errorf("a store that cannot be read is an error")
	}
	// a stored document the engine rejects fails the same way every time, so
	// Sync reports it once and then leaves it alone
	store := NewMemoryStore()
	_, _ = store.AppendSchema(ctx, SchemaVersion{Tenant: 7, Module: "asset", Name: "create", Doc: []byte(`{"type":"object","properties":{"a":{"type":"nonsense"}}}`)})
	g := newFixture(t, store)
	if changed, err := g.svc.Sync(ctx); !changed || err == nil {
		t.Errorf("the first sync reports the bad document: %v %v", changed, err)
	}
	if changed, err := g.svc.Sync(ctx); changed || err != nil {
		t.Errorf("the second must not repeat it: %v %v", changed, err)
	}
}
