package i18n

import (
	"testing"
)

func TestNew(t *testing.T) {
	translator, err := New("en")
	if err != nil {
		t.Fatalf("failed to create translator: %v", err)
	}

	if translator.LocaleCount() != 4 {
		t.Errorf("expected 4 locales, got %d", translator.LocaleCount())
	}

	locales := translator.Locales()
	expected := map[string]bool{"en": true, "es": true, "pt": true, "ja": true}
	for _, loc := range locales {
		if !expected[loc] {
			t.Errorf("unexpected locale: %s", loc)
		}
	}
}

func TestT(t *testing.T) {
	translator, _ := New("en")

	tests := []struct {
		locale   string
		key      string
		expected string
	}{
		{"en", "nav.dashboard", "Dashboard"},
		{"es", "nav.dashboard", "Panel"},
		{"pt", "nav.dashboard", "Painel"},
		{"en", "action.save", "Save"},
		{"es", "action.save", "Guardar"},
		{"pt", "action.save", "Salvar"},
	}

	for _, tt := range tests {
		got := translator.T(tt.locale, tt.key)
		if got != tt.expected {
			t.Errorf("T(%q, %q) = %q, want %q", tt.locale, tt.key, got, tt.expected)
		}
	}
}

func TestTWithArgs(t *testing.T) {
	translator, _ := New("en")

	tests := []struct {
		locale   string
		key      string
		args     []any
		expected string
	}{
		{"en", "time.minutes_ago", []any{5}, "5 minutes ago"},
		{"es", "time.minutes_ago", []any{5}, "Hace 5 minutos"},
		{"pt", "time.minutes_ago", []any{5}, "Há 5 minutos"},
	}

	for _, tt := range tests {
		got := translator.T(tt.locale, tt.key, tt.args...)
		if got != tt.expected {
			t.Errorf("T(%q, %q, %v) = %q, want %q", tt.locale, tt.key, tt.args, got, tt.expected)
		}
	}
}

func TestFallback(t *testing.T) {
	translator, _ := New("en")

	// Test that missing key in locale falls back to English
	// (We'd need to add a key only in en.json to test this properly)

	// Test that missing key everywhere returns the key itself
	got := translator.T("en", "nonexistent.key")
	if got != "nonexistent.key" {
		t.Errorf("expected key to be returned for missing translation, got %q", got)
	}
}

func TestForLocale(t *testing.T) {
	translator, _ := New("en")

	T := translator.ForLocale("es")

	if got := T("nav.dashboard"); got != "Panel" {
		t.Errorf("ForLocale(es)(nav.dashboard) = %q, want %q", got, "Panel")
	}

	if got := T("time.hours_ago", 3); got != "Hace 3 horas" {
		t.Errorf("ForLocale(es)(time.hours_ago, 3) = %q, want %q", got, "Hace 3 horas")
	}
}

func TestHasLocale(t *testing.T) {
	translator, _ := New("en")

	if !translator.HasLocale("en") {
		t.Error("expected HasLocale(en) to be true")
	}
	if !translator.HasLocale("es") {
		t.Error("expected HasLocale(es) to be true")
	}
	if !translator.HasLocale("pt") {
		t.Error("expected HasLocale(pt) to be true")
	}
	if translator.HasLocale("fr") {
		t.Error("expected HasLocale(fr) to be false")
	}
}

// newTestTranslatorForResolve builds a *Translator directly with an
// arbitrary set of loaded locales, bypassing New()'s embedded-FS
// loading -- needed since the real embedded locales are only
// en/es/pt, and ResolveLocale's region-matching behaviour needs
// region-qualified locales (en_gb, es_uy, etc.) to test against.
func newTestTranslatorForResolve(locales ...string) *Translator {
	tr := &Translator{
		messages: make(map[string]map[string]string),
		fallback: locales[0],
	}
	for _, l := range locales {
		tr.messages[l] = map[string]string{}
	}
	return tr
}

func TestResolveLocale_ExactMatch(t *testing.T) {
	tr := newTestTranslatorForResolve("en", "es")
	locale, exact := tr.ResolveLocale("es")
	if locale != "es" || !exact {
		t.Errorf("got (%q, %v), want (\"es\", true)", locale, exact)
	}
}

func TestResolveLocale_ExactRegionMatchWinsOverBase(t *testing.T) {
	tr := newTestTranslatorForResolve("es", "es_uy")
	locale, exact := tr.ResolveLocale("es_uy")
	if locale != "es_uy" || !exact {
		t.Errorf("got (%q, %v), want (\"es_uy\", true) -- exact match must win even when the base is also loaded", locale, exact)
	}
}

func TestResolveLocale_BaseRequestedOnlyRegionLoaded(t *testing.T) {
	// The exact scenario we described: "we have en_gb in store,
	// but we specify en only, thus whatever english locale is
	// available should be loaded".
	tr := newTestTranslatorForResolve("en_gb", "es")
	locale, exact := tr.ResolveLocale("en")
	if locale != "en_gb" || exact {
		t.Errorf("got (%q, %v), want (\"en_gb\", false)", locale, exact)
	}
}

func TestResolveLocale_RegionRequestedOnlyBaseLoaded(t *testing.T) {
	tr := newTestTranslatorForResolve("en", "es")
	locale, exact := tr.ResolveLocale("es_uy")
	if locale != "es" || exact {
		t.Errorf("got (%q, %v), want (\"es\", false)", locale, exact)
	}
}

func TestResolveLocale_RegionRequestedDifferentRegionLoaded(t *testing.T) {
	tr := newTestTranslatorForResolve("en", "es_es")
	locale, exact := tr.ResolveLocale("es_uy")
	if locale != "es_es" || exact {
		t.Errorf("got (%q, %v), want (\"es_es\", false) -- no exact match and no base, but a sibling region exists", locale, exact)
	}
}

func TestResolveLocale_RegionRequestedDifferentRegionLoaded_MirrorCase(t *testing.T) {
	// The same logical case as the test above, with the specific
	// variants swapped -- the tech team's exact scenario: es_es requested,
	// es_es doesn't exist, generic es doesn't exist, es_uy exists as
	// the closest alternative and matches. Checked directly against a
	// running test before assuming the general test above already
	// covered it symmetrically -- it does, but this makes that
	// explicit rather than relying on the reader to verify symmetry
	// themselves.
	tr := newTestTranslatorForResolve("en", "es_uy")
	locale, exact := tr.ResolveLocale("es_es")
	if locale != "es_uy" || exact {
		t.Errorf("got (%q, %v), want (\"es_uy\", false)", locale, exact)
	}
}

func TestResolveLocale_MultipleRegionCandidatesDeterministic(t *testing.T) {
	tr := newTestTranslatorForResolve("en_us", "en_gb")
	locale, exact := tr.ResolveLocale("en")
	if locale != "en_gb" || exact {
		t.Errorf("got (%q, %v), want (\"en_gb\", false) -- alphabetically first, not map iteration order", locale, exact)
	}
}

func TestResolveLocale_NoMatchAtAll(t *testing.T) {
	tr := newTestTranslatorForResolve("en", "es")
	locale, exact := tr.ResolveLocale("fr")
	if locale != "" || exact {
		t.Errorf("got (%q, %v), want (\"\", false)", locale, exact)
	}
}

func TestResolveLocale_EmptyRequestedResolvesToNothing(t *testing.T) {
	tr := newTestTranslatorForResolve("en", "es")
	locale, exact := tr.ResolveLocale("")
	if locale != "" || exact {
		t.Errorf("got (%q, %v), want (\"\", false)", locale, exact)
	}
}
