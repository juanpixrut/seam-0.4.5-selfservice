package overrides

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

var ctx = context.Background()

func stores() map[string]func() Store {
	return map[string]func() Store{
		"memory": func() Store { return NewMemoryStore() },
		"xolu":   func() Store { return NewXoluStore(newFakeXolu()) },
	}
}

func at(h int) time.Time { return time.Date(2026, 10, 13, h, 0, 0, 0, time.UTC) }

// Both stores must behave identically; this is the contract.
func TestStores_Conformance(t *testing.T) {
	for name, mk := range stores() {
		t.Run(name, func(t *testing.T) {
			// 0 is the global scope's tenant: the stores keep it, the Service decides who may use it
			t.Run("a negative tenant is refused everywhere", func(t *testing.T) {
				s := mk()
				for _, tenant := range []int64{-1, -9223372036854775808} {
					if _, err := s.Translations(ctx, tenant); !errors.Is(err, ErrNoTenant) {
						t.Errorf("Translations(%d): %v", tenant, err)
					}
					if err := s.SetTranslation(ctx, Translation{Tenant: tenant, Locale: "en", Key: "k", Value: "v"}); !errors.Is(err, ErrNoTenant) {
						t.Errorf("SetTranslation(%d): %v", tenant, err)
					}
					if err := s.RemoveTranslation(ctx, tenant, "en", "k"); !errors.Is(err, ErrNoTenant) {
						t.Errorf("RemoveTranslation(%d): %v", tenant, err)
					}
					if _, err := s.CurrentSchemas(ctx, tenant); !errors.Is(err, ErrNoTenant) {
						t.Errorf("CurrentSchemas(%d): %v", tenant, err)
					}
					if _, err := s.AppendSchema(ctx, SchemaVersion{Tenant: tenant, Module: "m", Name: "n"}); !errors.Is(err, ErrNoTenant) {
						t.Errorf("AppendSchema(%d): %v", tenant, err)
					}
					if _, err := s.SchemaHistory(ctx, tenant, "m", "n"); !errors.Is(err, ErrNoTenant) {
						t.Errorf("SchemaHistory(%d): %v", tenant, err)
					}
				}
			})

			t.Run("translations: set, replace, isolate, remove", func(t *testing.T) {
				s := mk()
				must(t, s.SetTranslation(ctx, Translation{Tenant: 1, Locale: "en", Key: "a", Value: "one", ChangedBy: 7, ChangedAt: at(9)}))
				must(t, s.SetTranslation(ctx, Translation{Tenant: 1, Locale: "es", Key: "a", Value: "uno", ChangedAt: at(9)}))
				must(t, s.SetTranslation(ctx, Translation{Tenant: 2, Locale: "en", Key: "a", Value: "other tenant", ChangedAt: at(9)}))
				must(t, s.SetTranslation(ctx, Translation{Tenant: 1, Locale: "en", Key: "a", Value: "ONE", ChangedBy: 8, ChangedAt: at(10)}))
				got, err := s.Translations(ctx, 1)
				must(t, err)
				if len(got) != 2 {
					t.Fatalf("replacing a text must not add a row: %+v", got)
				}
				byLoc := map[string]Translation{}
				for _, g := range got {
					byLoc[g.Locale] = g
				}
				if byLoc["en"].Value != "ONE" || byLoc["en"].ChangedBy != 8 || !byLoc["en"].ChangedAt.Equal(at(10)) || byLoc["es"].Value != "uno" {
					t.Errorf("wrong content: %+v", got)
				}
				other, _ := s.Translations(ctx, 2)
				if len(other) != 1 || other[0].Value != "other tenant" {
					t.Errorf("tenant 2 must see only its own text: %+v", other)
				}
				must(t, s.RemoveTranslation(ctx, 1, "en", "a"))
				must(t, s.RemoveTranslation(ctx, 1, "en", "never-there"))
				after, _ := s.Translations(ctx, 1)
				if len(after) != 1 || after[0].Locale != "es" {
					t.Errorf("only the removed text goes: %+v", after)
				}
				if o, _ := s.Translations(ctx, 2); len(o) != 1 {
					t.Errorf("removing from tenant 1 touched tenant 2")
				}
			})

			t.Run("schemas: versions, current, removal, history", func(t *testing.T) {
				s := mk()
				v1, err := s.AppendSchema(ctx, SchemaVersion{Tenant: 1, Module: "asset", Name: "create", Doc: []byte(`{"v":1}`), Note: "first", ChangedBy: 3, ChangedAt: at(9)})
				must(t, err)
				v2, _ := s.AppendSchema(ctx, SchemaVersion{Tenant: 1, Module: "asset", Name: "create", Doc: []byte(`{"v":2}`), ChangedAt: at(10)})
				other, _ := s.AppendSchema(ctx, SchemaVersion{Tenant: 1, Module: "asset", Name: "update", Doc: []byte(`{"u":1}`), ChangedAt: at(10)})
				foreign, _ := s.AppendSchema(ctx, SchemaVersion{Tenant: 2, Module: "asset", Name: "create", Doc: []byte(`{"t":2}`), ChangedAt: at(10)})
				if v1.Version != 1 || v2.Version != 2 || other.Version != 1 || foreign.Version != 1 {
					t.Errorf("versions count per tenant and form: %d %d %d %d", v1.Version, v2.Version, other.Version, foreign.Version)
				}
				cur, _ := s.CurrentSchemas(ctx, 1)
				if len(cur) != 2 || cur[0].Name != "create" || string(cur[0].Doc) != `{"v":2}` || cur[1].Name != "update" {
					t.Errorf("current is the newest version of each form, in order: %+v", cur)
				}
				v3, _ := s.AppendSchema(ctx, SchemaVersion{Tenant: 1, Module: "asset", Name: "create", Removed: true, Note: "removed", ChangedAt: at(11)})
				if cur, _ = s.CurrentSchemas(ctx, 1); len(cur) != 1 || cur[0].Name != "update" {
					t.Errorf("a removal hides the form: %+v", cur)
				}
				v4, _ := s.AppendSchema(ctx, SchemaVersion{Tenant: 1, Module: "asset", Name: "create", Doc: []byte(`{"v":4}`), ChangedAt: at(12)})
				if v3.Version != 3 || v4.Version != 4 {
					t.Errorf("a removal is a version, and the history goes on after it: %d %d", v3.Version, v4.Version)
				}
				if cur, _ = s.CurrentSchemas(ctx, 1); len(cur) != 2 || string(cur[0].Doc) != `{"v":4}` {
					t.Errorf("a form comes back after a removal: %+v", cur)
				}
				h, _ := s.SchemaHistory(ctx, 1, "asset", "create")
				if len(h) != 4 || h[0].Version != 4 || h[3].Version != 1 || !h[1].Removed || h[3].Note != "first" || h[3].ChangedBy != 3 || string(h[3].Doc) != `{"v":1}` {
					t.Errorf("history is newest first with everything kept: %+v", h)
				}
				if fh, _ := s.SchemaHistory(ctx, 2, "asset", "create"); len(fh) != 1 || string(fh[0].Doc) != `{"t":2}` {
					t.Errorf("tenant 2's history is its own: %+v", fh)
				}
				if e, _ := s.SchemaHistory(ctx, 1, "asset", "nothing"); len(e) != 0 {
					t.Errorf("an unknown form has no history")
				}
			})

			t.Run("tenants with overrides", func(t *testing.T) {
				s := mk()
				if ts, _ := s.TenantsWithOverrides(ctx); len(ts) != 0 {
					t.Errorf("none yet: %v", ts)
				}
				must(t, s.SetTranslation(ctx, Translation{Tenant: 5, Locale: "en", Key: "k", Value: "v"}))
				_, _ = s.AppendSchema(ctx, SchemaVersion{Tenant: 2, Module: "m", Name: "n", Doc: []byte("{}")})
				_, _ = s.AppendSchema(ctx, SchemaVersion{Tenant: 5, Module: "m", Name: "n", Doc: []byte("{}")})
				if ts, _ := s.TenantsWithOverrides(ctx); !reflect.DeepEqual(ts, []int64{2, 5}) {
					t.Errorf("each tenant once, in order: %v", ts)
				}
			})

			// Adversarial: names and texts are data. They must round-trip exactly and
			// must not reach beyond their own row (the fake refuses a query whose
			// WHERE it cannot fully account for).
			t.Run("hostile strings round-trip and stay inside their row", func(t *testing.T) {
				s := mk()
				must(t, s.SetTranslation(ctx, Translation{Tenant: 1, Locale: "en", Key: "innocent", Value: "keep me"}))
				hostile := []string{"en' OR '1'='1", "x'; DELETE FROM i18n_override; --", `back\slash`, "emoji 😀 and ' quote", "line1\nline2", "''", "' AND tenant_id = 2 AND key = '"}
				for _, h := range hostile {
					must(t, s.SetTranslation(ctx, Translation{Tenant: 1, Locale: h, Key: h, Value: h}))
					must(t, s.SetTranslation(ctx, Translation{Tenant: 1, Locale: h, Key: h, Value: h + "!"})) // replace goes through find()
					v, err := s.AppendSchema(ctx, SchemaVersion{Tenant: 1, Module: h, Name: h, Doc: []byte(`{"d":"` + "x" + `"}`), Note: h})
					must(t, err)
					v2, err := s.AppendSchema(ctx, SchemaVersion{Tenant: 1, Module: h, Name: h, Doc: []byte(`{}`)})
					must(t, err)
					if v.Version != 1 || v2.Version != 2 {
						t.Errorf("%q: versions %d %d, a hostile name must still be its own form", h, v.Version, v2.Version)
					}
					if hist, _ := s.SchemaHistory(ctx, 1, h, h); len(hist) != 2 || hist[1].Note != h {
						t.Errorf("%q: history %+v", h, hist)
					}
				}
				got, err := s.Translations(ctx, 1)
				must(t, err)
				if len(got) != len(hostile)+1 {
					t.Fatalf("every text is its own row, and the innocent one survives: %d rows", len(got))
				}
				for _, g := range got {
					if g.Key == "innocent" && g.Value != "keep me" {
						t.Errorf("an injected string changed another row")
					}
				}
				must(t, s.RemoveTranslation(ctx, 1, hostile[1], hostile[1]))
				if got, _ = s.Translations(ctx, 1); len(got) != len(hostile) {
					t.Errorf("removing a hostile key removes exactly that row: %d", len(got))
				}
			})

			t.Run("a large document round-trips", func(t *testing.T) {
				s := mk()
				doc := append([]byte(`{"pad":"`), append(bytes.Repeat([]byte("x"), 900<<10), []byte(`"}`)...)...)
				_, err := s.AppendSchema(ctx, SchemaVersion{Tenant: 1, Module: "m", Name: "n", Doc: doc})
				must(t, err)
				cur, _ := s.CurrentSchemas(ctx, 1)
				if len(cur) != 1 || !bytes.Equal(cur[0].Doc, doc) {
					t.Errorf("document changed in storage")
				}
			})
		})
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestMemoryStore_ConcurrentAppendsNumberEveryVersionOnce(t *testing.T) {
	s := NewMemoryStore()
	var wg sync.WaitGroup
	for i := 0; i < 60; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.AppendSchema(ctx, SchemaVersion{Tenant: 1, Module: "m", Name: "n", Doc: []byte("{}")})
		}()
	}
	wg.Wait()
	h, _ := s.SchemaHistory(ctx, 1, "m", "n")
	seen := map[int]bool{}
	for _, v := range h {
		seen[v.Version] = true
	}
	if len(h) != 60 || len(seen) != 60 || h[0].Version != 60 {
		t.Errorf("60 appends must give versions 1..60 once each: %d rows, %d distinct", len(h), len(seen))
	}
}

// The documents a store hands back are copies: changing one cannot change the store.
func TestMemoryStore_ReturnsCopies(t *testing.T) {
	s := NewMemoryStore()
	_, _ = s.AppendSchema(ctx, SchemaVersion{Tenant: 1, Module: "m", Name: "n", Doc: []byte("abc")})
	cur, _ := s.CurrentSchemas(ctx, 1)
	cur[0].Doc[0] = 'X'
	again, _ := s.CurrentSchemas(ctx, 1)
	if string(again[0].Doc) != "abc" {
		t.Errorf("the store's own document was changed through a returned copy")
	}
}

func TestXoluStore_PropagatesClientErrors(t *testing.T) {
	f := newFakeXolu()
	s := NewXoluStore(f)
	f.tables = nil // any create now panics on nil map write? no: guard with a failing client instead
	_ = s
	bad := NewXoluStore(errClient{})
	if _, err := bad.Translations(ctx, 1); err == nil || !strings.Contains(err.Error(), "down") {
		t.Errorf("a failing client's error must reach the caller: %v", err)
	}
	if err := bad.SetTranslation(ctx, Translation{Tenant: 1, Locale: "en", Key: "k", Value: "v"}); err == nil {
		t.Errorf("a failing client must fail the write")
	}
	if _, err := bad.AppendSchema(ctx, SchemaVersion{Tenant: 1, Module: "m", Name: "n"}); err == nil {
		t.Errorf("a failing client must fail the append")
	}
	if _, err := bad.TenantsWithOverrides(ctx); err == nil {
		t.Errorf("a failing client must fail the listing")
	}
	if _, err := bad.Fingerprint(ctx); err == nil {
		t.Errorf("a failing client must fail the fingerprint")
	}
}

// The fingerprint is how an instance notices another's change: it must move on
// every kind of change and stay put when nothing changed.
func TestStores_FingerprintMovesExactlyWhenSomethingChanges(t *testing.T) {
	for name, mk := range stores() {
		t.Run(name, func(t *testing.T) {
			s := mk()
			fp := func() string {
				v, err := s.Fingerprint(ctx)
				must(t, err)
				return v
			}
			f0 := fp()
			if fp() != f0 {
				t.Fatalf("reading twice must give the same fingerprint")
			}
			steps := []struct {
				name string
				do   func()
			}{
				{"a text is added", func() {
					must(t, s.SetTranslation(ctx, Translation{Tenant: 1, Locale: "en", Key: "k", Value: "v", ChangedAt: at(9)}))
				}},
				{"a text is replaced", func() {
					must(t, s.SetTranslation(ctx, Translation{Tenant: 1, Locale: "en", Key: "k", Value: "w", ChangedAt: at(10)}))
				}},
				{"a form version is added", func() {
					_, err := s.AppendSchema(ctx, SchemaVersion{Tenant: 1, Module: "m", Name: "n", Doc: []byte("{}"), ChangedAt: at(11)})
					must(t, err)
				}},
				{"a removal is appended", func() {
					_, err := s.AppendSchema(ctx, SchemaVersion{Tenant: 1, Module: "m", Name: "n", Removed: true, ChangedAt: at(12)})
					must(t, err)
				}},
				{"a text is removed", func() { must(t, s.RemoveTranslation(ctx, 1, "en", "k")) }},
			}
			prev := f0
			for _, st := range steps {
				st.do()
				if cur := fp(); cur == prev {
					t.Errorf("%s: the fingerprint did not move", st.name)
				} else {
					prev = cur
				}
			}
			must(t, s.RemoveTranslation(ctx, 1, "en", "never-there"))
			_, _ = s.Translations(ctx, 1)
			_, _ = s.CurrentSchemas(ctx, 1)
			_, _ = s.SchemaHistory(ctx, 1, "m", "n")
			if fp() != prev {
				t.Errorf("removing what is not there, and reading, must not move it")
			}
		})
	}
}
