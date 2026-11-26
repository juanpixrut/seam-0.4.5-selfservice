package overrides

import (
	"errors"
	"testing"

	"github.com/ha1tch/seam-ui/internal/i18n"
)

// Both stores keep the history of a text the same way: numbered from one per
// (tenant, locale, key), newest first, apart from the current text.
func TestStores_TranslationHistoryConformance(t *testing.T) {
	for name, mk := range stores() {
		t.Run(name, func(t *testing.T) {
			s := mk()
			// 0 is the global scope's tenant: the stores keep it, the Service decides who may use it
			for _, tenant := range []int64{-1, -7} {
				if _, err := s.AppendTranslationVersion(ctx, TranslationVersion{Tenant: tenant, Locale: "en", Key: "k"}); !errors.Is(err, ErrNoTenant) {
					t.Errorf("AppendTranslationVersion(%d): %v", tenant, err)
				}
				if _, err := s.TranslationHistory(ctx, tenant, "en", "k"); !errors.Is(err, ErrNoTenant) {
					t.Errorf("TranslationHistory(%d): %v", tenant, err)
				}
			}
			if h, err := s.TranslationHistory(ctx, 1, "en", "k"); err != nil || len(h) != 0 {
				t.Fatalf("a text with no history has none, not an error: %v %v", h, err)
			}
			v1, err := s.AppendTranslationVersion(ctx, TranslationVersion{Tenant: 1, Locale: "en", Key: "k", Value: "one", ChangedBy: 7, ChangedAt: at(9)})
			must(t, err)
			v2, err := s.AppendTranslationVersion(ctx, TranslationVersion{Tenant: 1, Locale: "en", Key: "k", Removed: true, ChangedBy: 8, ChangedAt: at(10)})
			must(t, err)
			v3, err := s.AppendTranslationVersion(ctx, TranslationVersion{Tenant: 1, Locale: "en", Key: "k", Value: "it's \"three\"\n%s", ChangedAt: at(11)})
			must(t, err)
			if v1.Version != 1 || v2.Version != 2 || v3.Version != 3 {
				t.Fatalf("versions are numbered from one: %d %d %d", v1.Version, v2.Version, v3.Version)
			}
			// other keys, locales and tenants number on their own and stay apart
			for _, o := range []TranslationVersion{{Tenant: 1, Locale: "es", Key: "k", Value: "uno"}, {Tenant: 1, Locale: "en", Key: "k2", Value: "x"}, {Tenant: 2, Locale: "en", Key: "k", Value: "other"}} {
				o.ChangedAt = at(12)
				got, err := s.AppendTranslationVersion(ctx, o)
				must(t, err)
				if got.Version != 1 {
					t.Errorf("%+v must start its own numbering, got %d", o, got.Version)
				}
			}
			h, err := s.TranslationHistory(ctx, 1, "en", "k")
			must(t, err)
			if len(h) != 3 || h[0].Version != 3 || h[1].Version != 2 || h[2].Version != 1 {
				t.Fatalf("newest first: %+v", h)
			}
			if h[0].Value != "it's \"three\"\n%s" || !h[1].Removed || h[1].Value != "" || h[2].Value != "one" || h[2].ChangedBy != 7 || !h[2].ChangedAt.Equal(at(9)) {
				t.Errorf("wrong content: %+v", h)
			}
			// the history is beside the current texts, not among them
			if cur, _ := s.Translations(ctx, 1); len(cur) != 0 {
				t.Errorf("recording a version must not create a current text: %+v", cur)
			}
			if ts, _ := s.TenantsWithOverrides(ctx); len(ts) != 0 {
				t.Errorf("a history alone is not an override: %v", ts)
			}
		})
	}
}

// The history must not move the fingerprint: it is what tells instances that
// the live overrides changed, and a version adds nothing to them.
func TestStores_TranslationHistoryLeavesTheFingerprintAlone(t *testing.T) {
	for name, mk := range stores() {
		t.Run(name, func(t *testing.T) {
			s := mk()
			before, err := s.Fingerprint(ctx)
			must(t, err)
			_, err = s.AppendTranslationVersion(ctx, TranslationVersion{Tenant: 1, Locale: "en", Key: "k", Value: "one", ChangedAt: at(9)})
			must(t, err)
			after, err := s.Fingerprint(ctx)
			must(t, err)
			if before != after {
				t.Errorf("the fingerprint moved: %q, then %q", before, after)
			}
		})
	}
}

func TestService_TranslationHistoryAndRestore(t *testing.T) {
	for name, mk := range stores() {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, mk())
			must(t, f.svc.SetTranslation(ctx, 3, "en", "greet", "Hi %s", "Hello %s", 9))
			must(t, f.svc.SetTranslation(ctx, 3, "en", "greet", "Hey %s", "Hello %s", 10))
			must(t, f.svc.RemoveTranslation(ctx, 3, "en", "greet", 11))
			must(t, f.svc.RemoveTranslation(ctx, 3, "en", "greet", 11)) // nothing there: no version
			h, err := f.svc.TranslationHistory(ctx, 3, "en", "greet")
			must(t, err)
			if len(h) != 3 || !h[0].Removed || h[0].ChangedBy != 11 || h[1].Value != "Hey %s" || h[2].Value != "Hi %s" || h[2].ChangedBy != 9 {
				t.Fatalf("every save and the one real removal are versions, newest first: %+v", h)
			}
			if got := f.text(3, "en", "greet", "Bob"); got != "Hello Bob" {
				t.Fatalf("after the removal the original shows, got %q", got)
			}
			// restoring an older wording makes it live again, as a new version
			v, err := f.svc.RestoreTranslation(ctx, 3, "en", "greet", 1, "Hello %s", 12)
			must(t, err)
			if v.Version != 4 || v.Value != "Hi %s" || v.ChangedBy != 12 {
				t.Errorf("the restored text is version 4: %+v", v)
			}
			if got := f.text(3, "en", "greet", "Bob"); got != "Hi Bob" {
				t.Errorf("the restored text is live, got %q", got)
			}
			if got := f.text(4, "en", "greet", "Bob"); got != "Hello Bob" {
				t.Errorf("another tenant is unaffected, got %q", got)
			}
			// refusals change nothing
			if _, err := f.svc.RestoreTranslation(ctx, 3, "en", "greet", 3, "Hello %s", 12); !errors.Is(err, ErrNotRestorable) {
				t.Errorf("a removal cannot be restored: %v", err)
			}
			if _, err := f.svc.RestoreTranslation(ctx, 3, "en", "greet", 99, "Hello %s", 12); !errors.Is(err, ErrUnknownVersion) {
				t.Errorf("an unknown version: %v", err)
			}
			if _, err := f.svc.RestoreTranslation(ctx, 0, "en", "greet", 1, "Hello %s", 12); !errors.Is(err, ErrNoTenant) {
				t.Errorf("no tenant: %v", err)
			}
			// the built-in text gained a placeholder since: the old wording no longer fits
			if _, err := f.svc.RestoreTranslation(ctx, 3, "en", "greet", 2, "Hello %s, you have %d", 12); !errors.Is(err, i18n.ErrVerbs) {
				t.Errorf("a version whose placeholders no longer match is refused: %v", err)
			}
			if h, _ := f.svc.TranslationHistory(ctx, 3, "en", "greet"); len(h) != 4 {
				t.Errorf("refusals must not add versions: %d", len(h))
			}
			// another tenant has no history of it, and cannot restore this one's
			if h, _ := f.svc.TranslationHistory(ctx, 4, "en", "greet"); len(h) != 0 {
				t.Errorf("tenant 4 sees tenant 3's history: %+v", h)
			}
			if _, err := f.svc.RestoreTranslation(ctx, 4, "en", "greet", 1, "Hello %s", 12); !errors.Is(err, ErrUnknownVersion) {
				t.Errorf("tenant 4 restored tenant 3's version: %v", err)
			}
		})
	}
}
