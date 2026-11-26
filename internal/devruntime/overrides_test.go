package devruntime

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/ha1tch/seam-ui/internal/devstate"
	"github.com/ha1tch/seam-ui/internal/i18n"
	"github.com/ha1tch/seam-ui/internal/overrides"
)

// devOverridesOn is the Developer module with the tenant-overrides decision taken.
var devOverridesOn = map[string]string{"SEAM_ENABLE_DEVELOPER": "1", "SEAM_DEVELOPER_TENANT_OVERRIDES": "1"}

const okDoc = `{"type":"object","properties":{"name":{"type":"string","title":"Name","x-seam-input":"text"}}}`

func TestStartAppliesStoredTenantOverridesBeforeServing(t *testing.T) {
	store := overrides.NewMemoryStore()
	ctx := context.Background()
	_ = store.SetTranslation(ctx, overrides.Translation{Tenant: 3, Locale: "en", Key: "greet", Value: "Hi from tenant 3"})
	_, _ = store.AppendSchema(ctx, overrides.SchemaVersion{Tenant: 3, Module: "asset", Name: "create", Doc: []byte(okDoc)})
	tr := translator()
	rt, err := Start(ctx, Options{Flags: flags(t, devOverridesOn), Translator: tr, Store: devstate.NewMemoryStore(), Overrides: store, Logger: quiet()})
	if err != nil || rt == nil || rt.Overrides == nil {
		t.Fatalf("Start: %v %v", rt, err)
	}
	if got := tr.ForTenant(3).ForLocale("en")("greet"); got != "Hi from tenant 3" {
		t.Errorf("tenant 3's stored text is live from the first request, got %q", got)
	}
	if got := tr.ForTenant(4).ForLocale("en")("greet"); got != "Hello" {
		t.Errorf("tenant 4 is unaffected, got %q", got)
	}
	if got := rt.Resolver.Resolve(3, "asset", "create", []byte(`{"base":true}`)); !bytes.Contains(got, []byte(`"Name"`)) {
		t.Errorf("tenant 3's stored form definition is live: %s", got)
	}
	if got := rt.Resolver.Resolve(4, "asset", "create", []byte(`{"base":true}`)); !bytes.Contains(got, []byte(`"base"`)) {
		t.Errorf("tenant 4 keeps the built-in definition: %s", got)
	}
}

// Not every deployment has the entities: reading them failing is logged, and
// the module still starts, with the service ready to create them on first use.
func TestStartWithAnUnreadableOverrideStoreStillStarts(t *testing.T) {
	var logged bytes.Buffer
	log := slogTo(&logged)
	rt, err := Start(context.Background(), Options{Flags: flags(t, devOverridesOn), Translator: translator(), Store: devstate.NewMemoryStore(),
		Overrides: brokenStore{overrides.NewMemoryStore()}, Logger: log})
	if err != nil || rt == nil || rt.Overrides == nil {
		t.Fatalf("a broken override store must not stop the module: %v %v", rt, err)
	}
	if !strings.Contains(logged.String(), "tenant overrides not") {
		t.Errorf("and it must say so in the log: %q", logged.String())
	}
}

func TestStartWithoutAStoreOrClientHasNoTenantOverrides(t *testing.T) {
	rt, err := Start(context.Background(), Options{Flags: flags(t, devOn), Translator: translator(), Store: devstate.NewMemoryStore(), Logger: quiet()})
	if err != nil || rt == nil {
		t.Fatal(err)
	}
	if rt.Overrides != nil {
		t.Errorf("with nowhere to keep them there is no tenant-scope editing")
	}
}

func TestStartOverrideTextsStayWithinTheirPlaceholders(t *testing.T) {
	rt, _ := Start(context.Background(), Options{Flags: flags(t, devOverridesOn), Translator: translator(), Store: devstate.NewMemoryStore(), Overrides: overrides.NewMemoryStore(), Logger: quiet()})
	if err := rt.Overrides.SetTranslation(context.Background(), 3, "en", "greet", "Hi %s", "Hello", 1); !errors.Is(err, i18n.ErrVerbs) {
		t.Errorf("the runtime's service enforces the same placeholder rule: %v", err)
	}
}

type brokenStore struct{ *overrides.MemoryStore }

func (brokenStore) TenantsWithOverrides(context.Context) ([]int64, error) {
	return nil, errors.New("entity i18n_override does not exist")
}

func slogTo(w *bytes.Buffer) *slog.Logger { return slog.New(slog.NewTextHandler(w, nil)) }
