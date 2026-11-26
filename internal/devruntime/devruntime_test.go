package devruntime

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ha1tch/seam-ui/internal/devstate"
	"github.com/ha1tch/seam-ui/internal/features"
	"github.com/ha1tch/seam-ui/internal/i18n"
	"github.com/ha1tch/seam-ui/internal/schemaoverlay"
)

func flags(t *testing.T, env map[string]string) *features.Set {
	t.Helper()
	set, err := features.Catalog().Resolve(features.Inputs{
		AuthEnabled: true,
		LookupEnv:   func(n string) (string, bool) { v, ok := env[n]; return v, ok },
	})
	if err != nil {
		t.Fatal(err)
	}
	return set
}

var devOn = map[string]string{"SEAM_ENABLE_DEVELOPER": "1"}

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func translator() *i18n.Translator {
	return i18n.NewFromMessages("en", map[string]map[string]string{"en": {"greet": "Hello"}})
}

func write(t *testing.T, path, content string, hoursAhead int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	mt := time.Now().Add(time.Duration(hoursAhead) * time.Hour)
	if err := os.Chtimes(path, mt, mt); err != nil {
		t.Fatal(err)
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestStartIsANoOpWhenDeveloperIsOff(t *testing.T) {
	rt, err := Start(context.Background(), Options{Flags: flags(t, nil), Translator: translator(), Store: devstate.NewMemoryStore()})
	if err != nil || rt != nil {
		t.Errorf("Start = %v, %v; want nil, nil", rt, err)
	}
	// And the nil *Resolver that results is harmless.
	var res *schemaoverlay.Resolver
	if got := string(res.Resolve(1, "m", "n", []byte("base"))); got != "base" {
		t.Errorf("nil Resolver.Resolve = %q", got)
	}
	if res.EffectiveLayer(1, "m", "n") != schemaoverlay.LayerEmbedded {
		t.Error("nil Resolver.EffectiveLayer")
	}
}

func TestStartRejectsMissingPieces(t *testing.T) {
	ctx := context.Background()
	if _, err := Start(ctx, Options{}); err == nil {
		t.Error("missing Flags")
	}
	if _, err := Start(ctx, Options{Flags: flags(t, devOn), Store: devstate.NewMemoryStore()}); err == nil {
		t.Error("missing Translator")
	}
	if _, err := Start(ctx, Options{Flags: flags(t, devOn), Translator: translator()}); err == nil {
		t.Error("missing Client and Store")
	}
}

func TestStartLoadsFilesAtOnceAndPollsOnlyWhileHotloadIsOn(t *testing.T) {
	root := t.TempDir()
	locales, schemas := filepath.Join(root, "locales"), filepath.Join(root, "schemas")
	write(t, filepath.Join(locales, "en.json"), `{"greet": "Hey"}`, 1)
	write(t, filepath.Join(schemas, "system", "user_edit.json"), `{"type":"object"}`, 1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tr := translator()
	rt, err := Start(ctx, Options{
		Flags: flags(t, devOn), Translator: tr, Store: devstate.NewMemoryStore(),
		LocalesDir: locales, SchemasDir: schemas, PollInterval: 5 * time.Millisecond, Logger: quiet(),
	})
	if err != nil || rt == nil {
		t.Fatalf("Start: %v, %v", rt, err)
	}

	// Initial load happens regardless of the hot-load switches.
	if got := tr.T("en", "greet"); got != "Hey" {
		t.Errorf("initial translation load: %q", got)
	}
	if got := string(rt.Resolver.Resolve(1, "system", "user_edit", []byte("base"))); got != `{"type":"object"}` {
		t.Errorf("initial schema load: %q", got)
	}

	// Hot loading is off: later edits are NOT picked up.
	write(t, filepath.Join(locales, "en.json"), `{"greet": "Hot"}`, 2)
	write(t, filepath.Join(schemas, "system", "user_edit.json"), `{"type":"array"}`, 2)
	time.Sleep(80 * time.Millisecond)
	if got := tr.T("en", "greet"); got != "Hey" {
		t.Errorf("translations reloaded with hot loading off: %q", got)
	}

	// Switch on hot-load i18n for tenant 1 from the panel: translations follow,
	// schemas (a separate switch) still do not.
	if err := rt.Manager.Change(ctx, 1, features.HotloadI18N, true, 1); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "translations to hot-load", func() bool { return tr.T("en", "greet") == "Hot" })
	if got := string(rt.Resolver.Resolve(1, "system", "user_edit", []byte("base"))); got != `{"type":"object"}` {
		t.Errorf("schemas reloaded although hot-load schemas is off: %q", got)
	}

	if err := rt.Manager.Change(ctx, 1, features.HotloadSchemas, true, 1); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "schemas to hot-load", func() bool {
		return string(rt.Resolver.Resolve(1, "system", "user_edit", []byte("base"))) == `{"type":"array"}`
	})
	if st := rt.Locales.Status(); st.LastError != "" || st.Keys != 1 {
		t.Errorf("locale layer status = %+v", st)
	}
}

func TestStartKeepsGoingWhenAnInitialLayerIsBroken(t *testing.T) {
	root := t.TempDir()
	locales := filepath.Join(root, "locales")
	write(t, filepath.Join(locales, "en.json"), `{broken`, 1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tr := translator()
	rt, err := Start(ctx, Options{
		Flags: flags(t, devOn), Translator: tr, Store: devstate.NewMemoryStore(),
		LocalesDir: locales, SchemasDir: filepath.Join(root, "does-not-exist"), Logger: quiet(),
	})
	if err != nil || rt == nil {
		t.Fatalf("a broken override must not stop startup: %v, %v", rt, err)
	}
	if got := tr.T("en", "greet"); got != "Hello" {
		t.Errorf("base translations must be untouched: %q", got)
	}
	if st := rt.Locales.Status(); len(st.Refused) != 1 || !strings.Contains(st.Refused[0].Reason, "en.json") {
		t.Errorf("status should name the broken file: %+v", st)
	}
	if rt.Schemas.Status().LastError == "" {
		t.Error("a missing schema directory should be reported in status")
	}
}

func TestWiringAppliesPersistedOptionsAtStartup(t *testing.T) {
	store := devstate.NewMemoryStore()
	_ = store.Set(context.Background(), devstate.Entry{Tenant: 3, Key: features.ShowI18NKeys, Value: true})
	tr := translator()
	if _, err := Start(context.Background(), Options{Flags: flags(t, devOn), Translator: tr, Store: store, Logger: quiet()}); err != nil {
		t.Fatal(err)
	}
	if got := tr.ForTenant(3).T("en", "greet"); got != "greet" {
		t.Errorf("persisted show-keys setting not applied at startup: %q", got)
	}
	if got := tr.ForTenant(4).T("en", "greet"); got != "Hello" {
		t.Errorf("another tenant affected: %q", got)
	}
}
