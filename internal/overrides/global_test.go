package overrides

import (
	"errors"
	"strings"
	"testing"

	"github.com/ha1tch/seam-ui/internal/i18n"
	"github.com/ha1tch/seam-ui/internal/schemaoverlay"
)

// The global scope: overrides stored under GlobalTenant apply to every tenant,
// below a tenant's own, and only once the scope is switched on.
func TestService_GlobalScope(t *testing.T) {
	tr := i18n.NewFromMessages("en", map[string]map[string]string{"en": {"greet": "Hello %s", "bye": "Bye"}})
	res := schemaoverlay.New(nil)
	store := NewMemoryStore()
	doc := []byte(`{"type":"object","properties":{"a":{"type":"string"}}}`)

	off := NewService(store, tr, res)
	if err := off.SetTranslation(ctx, GlobalTenant, "en", "bye", "Ciao", "Bye", 1); !errors.Is(err, ErrNoTenant) {
		t.Fatalf("without the scope a global write is refused: %v", err)
	}

	s := NewService(store, tr, res)
	s.AllowGlobal()
	if err := s.SetTranslation(ctx, GlobalTenant, "en", "bye", "Ciao", "Bye", 1); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.SaveSchema(ctx, GlobalTenant, "system", "profile", doc, nil, 1, "for everyone"); err != nil {
		t.Fatal(err)
	}
	for _, tenant := range []int64{1, 2} {
		if got := tr.ForTenant(tenant).T("en", "bye"); got != "Ciao" {
			t.Errorf("tenant %d sees the global text, got %q", tenant, got)
		}
		if got := res.Resolve(tenant, "system", "profile", nil); !strings.Contains(string(got), `"a"`) {
			t.Errorf("tenant %d gets the global form", tenant)
		}
	}
	// a tenant's own override wins over the global one
	if err := s.SetTranslation(ctx, 2, "en", "bye", "Adieu", "Bye", 5); err != nil {
		t.Fatal(err)
	}
	if a, b := tr.ForTenant(1).T("en", "bye"), tr.ForTenant(2).T("en", "bye"); a != "Ciao" || b != "Adieu" {
		t.Errorf("tenant beats global: %q %q", a, b)
	}
	// history and restore work as for a tenant
	if err := s.SetTranslation(ctx, GlobalTenant, "en", "bye", "Tschuess", "Bye", 1); err != nil {
		t.Fatal(err)
	}
	if h, err := s.TranslationHistory(ctx, GlobalTenant, "en", "bye"); err != nil || len(h) != 2 {
		t.Fatalf("global history: %+v %v", h, err)
	}
	// another instance: a fresh service over the same store loads the global rows
	tr2 := i18n.NewFromMessages("en", map[string]map[string]string{"en": {"bye": "Bye"}})
	s2 := NewService(store, tr2, schemaoverlay.New(nil))
	s2.AllowGlobal()
	if err := s2.Load(ctx); err != nil {
		t.Fatal(err)
	}
	if got := tr2.ForTenant(7).T("en", "bye"); got != "Tschuess" {
		t.Errorf("a new instance applies the global text, got %q", got)
	}
	// and one without the scope leaves them inert
	tr3 := i18n.NewFromMessages("en", map[string]map[string]string{"en": {"bye": "Bye"}})
	s3 := NewService(store, tr3, schemaoverlay.New(nil))
	if err := s3.Load(ctx); err != nil {
		t.Fatal(err)
	}
	if got := tr3.ForTenant(1).T("en", "bye"); got != "Bye" {
		t.Errorf("with the scope off the global rows are inert, got %q", got)
	}
	// removal clears it everywhere
	if err := s.RemoveTranslation(ctx, GlobalTenant, "en", "bye", 1); err != nil {
		t.Fatal(err)
	}
	if got := tr.ForTenant(1).T("en", "bye"); got != "Bye" {
		t.Errorf("removed: %q", got)
	}
}
