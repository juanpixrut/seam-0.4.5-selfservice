package devruntime

import (
	"context"
	"testing"
	"time"

	"github.com/ha1tch/seam-ui/internal/devstate"
	"github.com/ha1tch/seam-ui/internal/features"
	"github.com/ha1tch/seam-ui/internal/i18n"
	"github.com/ha1tch/seam-ui/internal/overrides"
)

// Two runtimes over the same stores are two instances of Seam on one database.
func twoInstances(t *testing.T, interval time.Duration, env ...map[string]string) (a, b *Runtime, ta, tb *i18n.Translator, ctx context.Context) {
	t.Helper()
	cctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	envMap := devOverridesOn
	if len(env) > 0 {
		envMap = env[0]
	}
	state, ostore := devstate.NewMemoryStore(), overrides.NewMemoryStore()
	start := func() (*Runtime, *i18n.Translator) {
		tr := translator()
		rt, err := Start(cctx, Options{Flags: flags(t, envMap), Translator: tr, Store: state, Overrides: ostore, SyncInterval: interval, Logger: quiet()})
		if err != nil || rt == nil {
			t.Fatalf("Start: %v %v", rt, err)
		}
		return rt, tr
	}
	a, ta = start()
	b, tb = start()
	return a, b, ta, tb, cctx
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for: %s", what)
}

func TestSync_AnotherInstancesOverridesArriveAndAreCleared(t *testing.T) {
	a, _, _, tb, ctx := twoInstances(t, 15*time.Millisecond)
	if err := a.Overrides.SetTranslation(ctx, 3, "en", "greet", "Hi from A", "", 1); err != nil {
		t.Fatal(err)
	}
	eventually(t, "B shows A's text", func() bool { return tb.ForTenant(3).ForLocale("en")("greet") == "Hi from A" })
	if tb.ForTenant(4).ForLocale("en")("greet") != "Hello" {
		t.Errorf("another tenant on B is unaffected")
	}
	if err := a.Overrides.RemoveTranslation(ctx, 3, "en", "greet", 1); err != nil {
		t.Fatal(err)
	}
	eventually(t, "B drops it when A removes the tenant's last text", func() bool { return tb.ForTenant(3).ForLocale("en")("greet") == "Hello" })
}

func TestSync_AnotherInstancesFormDefinitionsArriveAndAreCleared(t *testing.T) {
	a, b, _, _, ctx := twoInstances(t, 15*time.Millisecond)
	if _, _, err := a.Overrides.SaveSchema(ctx, 3, "asset", "create", []byte(okDoc), nil, 1, ""); err != nil {
		t.Fatal(err)
	}
	has := func() bool {
		return len(b.Resolver.Resolve(3, "asset", "create", []byte(`{"base":true}`))) > 0 && !jsonHas(b.Resolver.Resolve(3, "asset", "create", []byte(`{"base":true}`)), `"base"`)
	}
	eventually(t, "B resolves A's form definition", has)
	if err := a.Overrides.RemoveSchema(ctx, 3, "asset", "create", 1); err != nil {
		t.Fatal(err)
	}
	eventually(t, "B goes back to the built-in definition", func() bool {
		return jsonHas(b.Resolver.Resolve(3, "asset", "create", []byte(`{"base":true}`)), `"base"`)
	})
}

func jsonHas(b []byte, s string) bool { return len(b) > 0 && contains(string(b), s) }

func contains(h, n string) bool {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return true
		}
	}
	return false
}

func TestSync_AnotherInstancesSwitchesArriveAndTheirRemovalReachesTheTranslator(t *testing.T) {
	a, b, _, tb, ctx := twoInstances(t, 15*time.Millisecond)
	if err := a.Manager.Change(ctx, 3, features.ShowI18NKeys, true, 1); err != nil {
		t.Fatal(err)
	}
	eventually(t, "B's manager sees the switch", func() bool { on, _ := b.Manager.Effective(3, features.ShowI18NKeys); return on })
	eventually(t, "and B's translator shows keys for that tenant", func() bool { return tb.ForTenant(3).ForLocale("en")("greet") == "greet" })
	if err := a.Manager.Reset(ctx, 3, features.ShowI18NKeys); err != nil {
		t.Fatal(err)
	}
	eventually(t, "resetting it on A is undone on B, translator included", func() bool {
		on, _ := b.Manager.Effective(3, features.ShowI18NKeys)
		return !on && tb.ForTenant(3).ForLocale("en")("greet") == "Hello"
	})
}

func TestSync_NegativeIntervalNeverChecks(t *testing.T) {
	a, _, _, tb, ctx := twoInstances(t, -1)
	_ = a.Overrides.SetTranslation(ctx, 3, "en", "greet", "Hi from A", "", 1)
	time.Sleep(150 * time.Millisecond)
	if tb.ForTenant(3).ForLocale("en")("greet") != "Hello" {
		t.Errorf("with checking switched off another instance's change must not arrive")
	}
}

func TestSync_StopsWithItsContext(t *testing.T) {
	cctx, cancel := context.WithCancel(context.Background())
	rt, err := Start(cctx, Options{Flags: flags(t, devOverridesOn), Translator: translator(), Store: devstate.NewMemoryStore(), Overrides: overrides.NewMemoryStore(), SyncInterval: 5 * time.Millisecond, Logger: quiet()})
	if err != nil || rt == nil {
		t.Fatal(err)
	}
	cancel()
	time.Sleep(40 * time.Millisecond) // must not panic or race when the context is gone
}

func TestSync_DefaultIntervalIsGentle(t *testing.T) {
	if DefaultSyncInterval < 5*time.Second {
		t.Errorf("every check reads the stored rows: %v is too often", DefaultSyncInterval)
	}
}
