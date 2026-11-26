package i18n

import (
	"sort"
	"strings"
	"sync"
	"testing"
)

func newTr() *Translator {
	return NewFromMessages("en", map[string]map[string]string{
		"en": {"greet": "Hello", "bye": "Goodbye", "count": "%d items", "only_en": "English only"},
		"es": {"greet": "Hola", "bye": "Adiós"},
	})
}

func layerMsgs(locale string, kv ...string) map[string]map[string]string {
	m := map[string]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	return map[string]map[string]string{locale: m}
}

func mustReload(t *testing.T, tr *Translator, l Layer, tenant int64, data map[string]map[string]string) {
	t.Helper()
	if err := tr.ReloadLayer(l, tenant, data); err != nil {
		t.Fatalf("ReloadLayer(%s, %d): %v", l, tenant, err)
	}
}

func TestNoOverlaysBehavesAsBefore(t *testing.T) {
	tr := newTr()
	cases := []struct{ loc, key, want string }{
		{"en", "greet", "Hello"},
		{"es", "greet", "Hola"},
		{"es", "only_en", "English only"}, // falls back to en
		{"fr", "greet", "Hello"},          // unknown locale falls back
		{"en", "missing", "missing"},      // last resort: the key
	}
	for _, c := range cases {
		if got := tr.T(c.loc, c.key); got != c.want {
			t.Errorf("T(%q,%q) = %q, want %q", c.loc, c.key, got, c.want)
		}
	}
	if got := tr.T("en", "count", 3); got != "3 items" {
		t.Errorf("args: got %q", got)
	}
}

func TestLayerPrecedence(t *testing.T) {
	tr := newTr()
	v := tr.ForTenant(7)

	mustReload(t, tr, LayerXoluGlobal, 0, layerMsgs("en", "greet", "Hi (global)"))
	if got := v.T("en", "greet"); got != "Hi (global)" {
		t.Errorf("xolu-global over embedded: got %q", got)
	}
	mustReload(t, tr, LayerXoluTenant, 7, layerMsgs("en", "greet", "Hi (tenant)"))
	if got := v.T("en", "greet"); got != "Hi (tenant)" {
		t.Errorf("xolu-tenant over xolu-global: got %q", got)
	}
	mustReload(t, tr, LayerFilesystem, 0, layerMsgs("en", "greet", "Hi (file)"))
	if got := v.T("en", "greet"); got != "Hi (file)" {
		t.Errorf("filesystem over xolu-tenant: got %q", got)
	}
}

func TestOverridesApplyPerKey(t *testing.T) {
	tr := newTr()
	mustReload(t, tr, LayerFilesystem, 0, layerMsgs("en", "greet", "Hey"))
	if got := tr.T("en", "bye"); got != "Goodbye" {
		t.Errorf("untouched key must still resolve from the base, got %q", got)
	}
	// a key that exists only in an overlay is reachable too
	mustReload(t, tr, LayerFilesystem, 0, layerMsgs("en", "brand_new", "New!"))
	if got := tr.T("en", "brand_new"); got != "New!" {
		t.Errorf("overlay-only key: got %q", got)
	}
}

func TestRequestedLocaleBeatsFallbackLocaleAcrossLayers(t *testing.T) {
	tr := newTr()
	// An English override must not shadow a Spanish base string: the
	// requested locale is resolved through every layer first.
	mustReload(t, tr, LayerFilesystem, 0, layerMsgs("en", "greet", "Hey"))
	if got := tr.T("es", "greet"); got != "Hola" {
		t.Errorf("es greet = %q, want the Spanish base", got)
	}
	// When Spanish lacks the key everywhere, the fallback locale's
	// layers apply, overrides included.
	mustReload(t, tr, LayerFilesystem, 0, layerMsgs("en", "only_en", "English override"))
	if got := tr.T("es", "only_en"); got != "English override" {
		t.Errorf("fallback through layers: got %q", got)
	}
}

func TestTenantIsolation(t *testing.T) {
	tr := newTr()
	mustReload(t, tr, LayerXoluTenant, 1, layerMsgs("en", "greet", "Hi tenant 1"))
	if got := tr.ForTenant(1).T("en", "greet"); got != "Hi tenant 1" {
		t.Errorf("tenant 1: %q", got)
	}
	if got := tr.ForTenant(2).T("en", "greet"); got != "Hello" {
		t.Errorf("tenant 2 must not see tenant 1's override: %q", got)
	}
	if got := tr.T("en", "greet"); got != "Hello" {
		t.Errorf("global T must not see a tenant override: %q", got)
	}
	// but global layers reach every tenant and the global view
	mustReload(t, tr, LayerXoluGlobal, 0, layerMsgs("en", "bye", "Bye all"))
	for _, got := range []string{tr.T("en", "bye"), tr.ForTenant(1).T("en", "bye"), tr.ForTenant(2).T("en", "bye")} {
		if got != "Bye all" {
			t.Errorf("global override missing: %q", got)
		}
	}
}

func TestReloadLayerReplacesAtomicallyAndClears(t *testing.T) {
	tr := newTr()
	mustReload(t, tr, LayerFilesystem, 0, map[string]map[string]string{
		"en": {"greet": "A", "bye": "A"},
		"es": {"greet": "A-es"},
	})
	// A second reload replaces everything: keys and locales that vanished
	// from the source vanish from the layer.
	mustReload(t, tr, LayerFilesystem, 0, layerMsgs("en", "greet", "B"))
	if got := tr.T("en", "bye"); got != "Goodbye" {
		t.Errorf("removed key still overridden: %q", got)
	}
	if got := tr.T("es", "greet"); got != "Hola" {
		t.Errorf("removed locale still overridden: %q", got)
	}
	mustReload(t, tr, LayerFilesystem, 0, nil)
	if got := tr.T("en", "greet"); got != "Hello" {
		t.Errorf("cleared layer still overriding: %q", got)
	}
}

func TestReloadLayerCopiesItsInput(t *testing.T) {
	tr := newTr()
	in := layerMsgs("en", "greet", "Hey")
	mustReload(t, tr, LayerFilesystem, 0, in)
	in["en"]["greet"] = "MUTATED"
	if got := tr.T("en", "greet"); got != "Hey" {
		t.Errorf("caller mutation leaked into the translator: %q", got)
	}
}

func TestReloadLayerValidation(t *testing.T) {
	tr := newTr()
	bad := []struct {
		name   string
		layer  Layer
		tenant int64
	}{
		{"embedded", LayerEmbedded, 0},
		{"unknown layer", Layer("nope"), 0},
		{"tenant layer without tenant", LayerXoluTenant, 0},
		{"tenant layer negative", LayerXoluTenant, -1},
		{"filesystem with tenant", LayerFilesystem, 3},
		{"global with tenant", LayerXoluGlobal, 3},
	}
	for _, c := range bad {
		if err := tr.ReloadLayer(c.layer, c.tenant, layerMsgs("en", "greet", "x")); err == nil {
			t.Errorf("%s: expected an error", c.name)
		}
	}
	if got := tr.T("en", "greet"); got != "Hello" {
		t.Errorf("a rejected reload changed state: %q", got)
	}
}

func TestOptions(t *testing.T) {
	tr := newTr()
	mustReload(t, tr, LayerFilesystem, 0, layerMsgs("en", "greet", "file"))
	mustReload(t, tr, LayerXoluGlobal, 0, layerMsgs("en", "bye", "xolu-global"))
	mustReload(t, tr, LayerXoluTenant, 5, layerMsgs("en", "bye", "xolu-tenant"))
	v := tr.ForTenant(5)

	tr.SetOptions(5, Options{DisableFilesystem: true})
	if got := v.T("en", "greet"); got != "Hello" {
		t.Errorf("filesystem should be disabled for tenant 5: %q", got)
	}
	if got := tr.ForTenant(6).T("en", "greet"); got != "file" {
		t.Errorf("options must be per tenant; tenant 6 got %q", got)
	}

	tr.SetOptions(5, Options{DisableXolu: true})
	if got := v.T("en", "bye"); got != "Goodbye" {
		t.Errorf("both xolu layers should be disabled: %q", got)
	}
	if got := v.T("en", "greet"); got != "file" {
		t.Errorf("filesystem should be back: %q", got)
	}

	tr.SetOptions(5, Options{ShowKeys: true})
	if got := v.T("en", "greet"); got != "greet" {
		t.Errorf("ShowKeys: got %q", got)
	}
	if got := v.T("en", "count", 3); got != "count" {
		t.Errorf("ShowKeys must not format: %q", got)
	}

	if (tr.Options(5) != Options{ShowKeys: true}) {
		t.Errorf("Options getter: %+v", tr.Options(5))
	}
	tr.SetOptions(5, Options{})
	if (tr.Options(5) != Options{}) {
		t.Error("zero Options should clear")
	}
	if got := v.T("en", "bye"); got != "xolu-tenant" {
		t.Errorf("after clearing options: %q", got)
	}
}

func TestExplain(t *testing.T) {
	tr := newTr()
	mustReload(t, tr, LayerFilesystem, 0, layerMsgs("en", "greet", "file"))
	mustReload(t, tr, LayerXoluGlobal, 0, layerMsgs("en", "greet", "global"))

	c := tr.Explain(3, "en", "greet")
	if len(c) != 3 {
		t.Fatalf("want 3 candidates, got %+v", c)
	}
	wantLayers := []Layer{LayerFilesystem, LayerXoluGlobal, LayerEmbedded}
	for i, l := range wantLayers {
		if c[i].Layer != l {
			t.Errorf("candidate %d layer = %s, want %s", i, c[i].Layer, l)
		}
	}
	if !c[0].Effective || c[1].Effective || c[2].Effective {
		t.Errorf("only the top layer should be effective: %+v", c)
	}

	// With the filesystem layer switched off it is listed but not effective.
	tr.SetOptions(3, Options{DisableFilesystem: true})
	c = tr.Explain(3, "en", "greet")
	if !c[0].Disabled || c[0].Effective || !c[1].Effective {
		t.Errorf("disabled handling: %+v", c)
	}

	// Key only in the fallback locale.
	c = tr.Explain(3, "es", "only_en")
	if len(c) != 1 || c[0].Locale != "en" || !c[0].Effective {
		t.Errorf("fallback explain: %+v", c)
	}

	// Unknown key: nothing.
	if c := tr.Explain(3, "en", "nope"); len(c) != 0 {
		t.Errorf("unknown key should have no candidates: %+v", c)
	}
}

func TestExplainAgreesWithT(t *testing.T) {
	tr := newTr()
	mustReload(t, tr, LayerFilesystem, 0, layerMsgs("en", "greet", "file"))
	mustReload(t, tr, LayerXoluTenant, 3, layerMsgs("en", "bye", "tenant-bye"))
	for _, tenant := range []int64{0, 3} {
		for _, loc := range []string{"en", "es", "fr"} {
			for _, key := range []string{"greet", "bye", "only_en", "missing"} {
				got := tr.ForTenant(tenant).T(loc, key)
				want := key
				for _, c := range tr.Explain(tenant, loc, key) {
					if c.Effective {
						want = c.Message
					}
				}
				if got != want {
					t.Errorf("tenant %d %s/%s: T=%q Explain=%q", tenant, loc, key, got, want)
				}
			}
		}
	}
}

func TestEffective(t *testing.T) {
	tr := newTr()
	mustReload(t, tr, LayerXoluGlobal, 0, layerMsgs("en", "greet", "global"))
	mustReload(t, tr, LayerXoluTenant, 2, layerMsgs("en", "greet", "tenant", "extra", "x"))
	got := tr.Effective(2, "en")
	if got["greet"] != "tenant" || got["extra"] != "x" || got["bye"] != "Goodbye" {
		t.Errorf("Effective(2,en) = %v", got)
	}
	got["bye"] = "MUTATED"
	if tr.Effective(2, "en")["bye"] != "Goodbye" {
		t.Error("Effective must return a copy")
	}
	if g := tr.Effective(0, "en"); g["greet"] != "global" || g["extra"] != "" {
		t.Errorf("Effective(0,en) = %v", g)
	}
	// The fallback locale is not merged in.
	if _, ok := tr.Effective(0, "es")["only_en"]; ok {
		t.Error("Effective must not merge the fallback locale")
	}
}

func TestNewLocaleFromGlobalOverlay(t *testing.T) {
	tr := newTr()
	if tr.HasLocale("fr") {
		t.Fatal("fr should not exist yet")
	}
	n := tr.LocaleCount()
	mustReload(t, tr, LayerFilesystem, 0, layerMsgs("fr", "greet", "Bonjour"))
	if !tr.HasLocale("fr") || tr.LocaleCount() != n+1 {
		t.Errorf("filesystem overlay should introduce locale fr (count %d -> %d)", n, tr.LocaleCount())
	}
	if got := tr.T("fr", "greet"); got != "Bonjour" {
		t.Errorf("fr greet = %q", got)
	}
	if got := tr.T("fr", "bye"); got != "Goodbye" {
		t.Errorf("fr falls back to en for missing keys: %q", got)
	}
	if loc, exact := tr.ResolveLocale("fr_ca"); loc != "fr" || exact {
		t.Errorf("ResolveLocale(fr_ca) = %q, %v", loc, exact)
	}
	found := false
	for _, l := range tr.Locales() {
		if l == "fr" {
			found = true
		}
	}
	if !found {
		t.Error("Locales() should list fr")
	}

	// Tenant layers do not introduce locales.
	mustReload(t, tr, LayerXoluTenant, 9, layerMsgs("de", "greet", "Hallo"))
	if tr.HasLocale("de") {
		t.Error("a tenant-only locale must not appear globally")
	}
}

func TestResolveLocaleDeterministicWithOverlayLocales(t *testing.T) {
	tr := newTr()
	mustReload(t, tr, LayerXoluGlobal, 0, map[string]map[string]string{
		"pt_br": {"greet": "Olá"}, "pt_pt": {"greet": "Olá!"},
	})
	for i := 0; i < 20; i++ {
		if loc, exact := tr.ResolveLocale("pt"); loc != "pt_br" || exact {
			t.Fatalf("ResolveLocale(pt) = %q, %v; want pt_br, false", loc, exact)
		}
	}
}

func TestConcurrentReloadAndRead(t *testing.T) {
	tr := newTr()
	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Writer alternates between two complete generations.
	wg.Add(1)
	go func() {
		defer wg.Done()
		gens := []map[string]map[string]string{
			layerMsgs("en", "a", "1", "b", "1"),
			layerMsgs("en", "a", "2", "b", "2"),
		}
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			_ = tr.ReloadLayer(LayerFilesystem, 0, gens[i%2])
			tr.SetOptions(1, Options{DisableXolu: i%2 == 0})
		}
	}()

	var bad int
	var mu sync.Mutex
	var rwg sync.WaitGroup
	for r := 0; r < 4; r++ {
		rwg.Add(1)
		go func() {
			defer rwg.Done()
			for i := 0; i < 2000; i++ {
				snap := tr.Effective(0, "en")
				a, b := snap["a"], snap["b"]
				if a != b { // both present from the same generation, or both absent
					mu.Lock()
					bad++
					mu.Unlock()
				}
				_ = tr.ForTenant(1).T("en", "greet")
			}
		}()
	}
	rwg.Wait()
	close(stop)
	wg.Wait()
	if bad != 0 {
		t.Errorf("%d snapshots mixed two reload generations; ReloadLayer is not atomic", bad)
	}
}

func TestEmbeddedLocalesStillLoad(t *testing.T) {
	tr, err := New("en")
	if err != nil {
		t.Fatal(err)
	}
	locs := tr.Locales()
	sort.Strings(locs)
	if got := strings.Join(locs, ","); got != "en,es,ja,pt" {
		t.Errorf("embedded locales = %s", got)
	}
	if tr.T("en", "definitely.not.a.key") != "definitely.not.a.key" {
		t.Error("unknown key should return itself")
	}
}
