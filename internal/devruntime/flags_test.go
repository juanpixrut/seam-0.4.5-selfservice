package devruntime

import (
	"context"
	"testing"
	"time"

	"github.com/ha1tch/seam-ui/internal/devstate"
	"github.com/ha1tch/seam-ui/internal/overrides"
)

// With the tenant-overrides decision not taken, nothing is read, written or
// polled, whatever store the caller supplied.
func TestStartIgnoresAnOverrideStoreWhileTheDecisionIsNotTaken(t *testing.T) {
	store := overrides.NewMemoryStore()
	_ = store.SetTranslation(context.Background(), overrides.Translation{Tenant: 3, Locale: "en", Key: "greet", Value: "Hi from tenant 3"})
	tr := translator()
	rt, err := Start(context.Background(), Options{Flags: flags(t, devOn), Translator: tr, Store: devstate.NewMemoryStore(), Overrides: store, Logger: quiet()})
	if err != nil || rt == nil {
		t.Fatal(err)
	}
	if rt.Overrides != nil {
		t.Errorf("the service must not exist")
	}
	if got := tr.ForTenant(3).ForLocale("en")("greet"); got != "Hello" {
		t.Errorf("and the stored text must not be applied, got %q", got)
	}
}

func TestSyncIsOffWhenTheDecisionSaysSo(t *testing.T) {
	env := map[string]string{"SEAM_ENABLE_DEVELOPER": "1", "SEAM_DEVELOPER_TENANT_OVERRIDES": "1", "SEAM_DEVELOPER_INSTANCE_SYNC": "0"}
	a, _, _, tb, ctx := twoInstances(t, 10*time.Millisecond, env)
	_ = a.Overrides.SetTranslation(ctx, 3, "en", "greet", "Hi from A", "", 1)
	time.Sleep(150 * time.Millisecond)
	if tb.ForTenant(3).ForLocale("en")("greet") != "Hello" {
		t.Errorf("with instance sync off another instance's change must not arrive")
	}
}
