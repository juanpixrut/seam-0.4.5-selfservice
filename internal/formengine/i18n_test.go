package formengine

import (
	"context"
	"fmt"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"
)

// fakeTranslator is a minimal Translator for testing -- a flat
// map[locale][key]translation, matching internal/i18n.Translator's own
// T() contract exactly: returns the key itself when nothing resolves,
// never an error.
type fakeTranslator map[string]map[string]string

func (f fakeTranslator) T(locale, key string, args ...any) string {
	msg, ok := f[locale][key]
	if !ok {
		return key
	}
	// Matches internal/i18n.Translator.T's real behaviour exactly
	// (fmt.Sprintf(msg, args...) when args is non-empty) -- an earlier
	// version of this test double just returned msg verbatim, which
	// silently made every interpolation test pass a %s-shaped string
	// through unresolved instead of actually proving interpolation
	// works. Caught when the queryfy error-mapping tests failed with
	// literal "%s" in the output, not by re-reading this file first.
	if len(args) > 0 {
		return fmt.Sprintf(msg, args...)
	}
	return msg
}

func TestResolveTitle_FormScopedKeyWins(t *testing.T) {
	tr := fakeTranslator{"es": {"assets.forms.asset.field.name": "Nombre"}}
	e := &FormEngine{Translator: tr}
	rc := RenderContext{Locale: "es", Module: "assets", Form: "asset"}
	got := resolveTitle(e, rc, "name", "literal fallback")
	if got != "Nombre" {
		t.Errorf("got %q, want %q", got, "Nombre")
	}
}

func TestResolveTitle_LabelVariantFallback(t *testing.T) {
	tr := fakeTranslator{"es": {"assets.forms.asset.field.name.label": "Nombre (label variant)"}}
	e := &FormEngine{Translator: tr}
	rc := RenderContext{Locale: "es", Module: "assets", Form: "asset"}
	got := resolveTitle(e, rc, "name", "literal fallback")
	if got != "Nombre (label variant)" {
		t.Errorf("got %q, want the .label variant", got)
	}
}

func TestResolveTitle_GlobalFieldNamespaceFallback(t *testing.T) {
	tr := fakeTranslator{"es": {"field.name": "Nombre (global)"}}
	e := &FormEngine{Translator: tr}
	rc := RenderContext{Locale: "es", Module: "assets", Form: "asset"}
	got := resolveTitle(e, rc, "name", "literal fallback")
	if got != "Nombre (global)" {
		t.Errorf("got %q, want the global field.* fallback", got)
	}
}

func TestResolveTitle_LiteralFallbackWhenNothingResolves(t *testing.T) {
	e := &FormEngine{Translator: fakeTranslator{}}
	rc := RenderContext{Locale: "es", Module: "assets", Form: "asset"}
	got := resolveTitle(e, rc, "name", "literal fallback")
	if got != "literal fallback" {
		t.Errorf("got %q, want the literal schema title", got)
	}
}

func TestResolveTitle_NilTranslatorFallsBackToLiteral(t *testing.T) {
	e := &FormEngine{} // Translator is nil
	rc := RenderContext{Locale: "es", Module: "assets", Form: "asset"}
	got := resolveTitle(e, rc, "name", "literal fallback")
	if got != "literal fallback" {
		t.Errorf("a nil Translator should degrade to the literal title, got %q", got)
	}
}

func TestResolveTitle_NoModuleOrForm_SkipsFormScopedKeysEntirely(t *testing.T) {
	// Module/Form both empty (schema declared neither, RenderContext
	// didn't override) -- must not attempt "..forms..field.name" (a
	// malformed key), should go straight to the global field.* fallback.
	tr := fakeTranslator{"es": {"field.name": "Nombre (global only)"}}
	e := &FormEngine{Translator: tr}
	rc := RenderContext{Locale: "es"}
	got := resolveTitle(e, rc, "name", "literal fallback")
	if got != "Nombre (global only)" {
		t.Errorf("got %q, want the global fallback (form-scoped keys should be skipped)", got)
	}
}

func TestResolveHelp_ResolvesAndFallsBack(t *testing.T) {
	tr := fakeTranslator{"es": {"assets.forms.asset.field.name.help": "Ayuda resuelta"}}
	e := &FormEngine{Translator: tr}
	rc := RenderContext{Locale: "es", Module: "assets", Form: "asset"}
	if got := resolveHelp(e, rc, "name", "literal help"); got != "Ayuda resuelta" {
		t.Errorf("got %q, want the resolved help text", got)
	}
	// A field with no matching key falls back to its literal help.
	if got := resolveHelp(e, rc, "unmapped_field", "literal help"); got != "literal help" {
		t.Errorf("got %q, want the literal fallback", got)
	}
}

func TestResolveLabel_ResolvesAndFallsBack(t *testing.T) {
	tr := fakeTranslator{"es": {"assets.forms.asset.tab.principal": "Principal"}}
	e := &FormEngine{Translator: tr}
	rc := RenderContext{Locale: "es"}
	if got := resolveLabel(e, rc, "assets.forms.asset.tab.principal"); got != "Principal" {
		t.Errorf("got %q, want the resolved tab label", got)
	}
	// An unresolved key falls back to the key itself as literal text --
	// the same "show something, not nothing" principle as everywhere
	// else in this package.
	if got := resolveLabel(e, rc, "assets.forms.asset.tab.unmapped"); got != "assets.forms.asset.tab.unmapped" {
		t.Errorf("got %q, want the key itself as literal fallback", got)
	}
}

func TestResolveModuleForm_RenderContextOverridesSchemaDefault(t *testing.T) {
	form := &Form{Module: "assets", Name: "asset"}
	// No override -- schema defaults win.
	m, n := resolveModuleForm(form, RenderContext{})
	if m != "assets" || n != "asset" {
		t.Errorf("got (%q, %q), want (\"assets\", \"asset\")", m, n)
	}
	// RenderContext overrides both independently.
	m, n = resolveModuleForm(form, RenderContext{Module: "dashboard", Form: "kpi_card"})
	if m != "dashboard" || n != "kpi_card" {
		t.Errorf("got (%q, %q), want override values", m, n)
	}
	// Override just one axis -- the other keeps the schema default.
	m, n = resolveModuleForm(form, RenderContext{Form: "kpi_card_variant"})
	if m != "assets" || n != "kpi_card_variant" {
		t.Errorf("got (%q, %q), want module from schema, form from override", m, n)
	}
}

func TestRender_EndToEnd_TitleAndTabLabelResolveWithRealTranslator(t *testing.T) {
	// Proof case: the actual Render() pipeline, not resolveTitle called
	// in isolation -- confirms rc.Module/rc.Form get set from the
	// schema once at the top of Render() and actually reach the widget
	// row and tab button by the time HTML comes out the other end.
	schema := []byte(`{
		"type": "object",
		"x-seam-form": {
			"module": "assets",
			"name": "asset",
			"tabs": [{"id": "tab-a", "label": "assets.forms.asset.tab.a",
				"sections": [{"id": "s1", "fields": ["name"]}]}]
		},
		"properties": {"name": {"type": "string", "title": "Name (literal)"}}
	}`)
	tr := fakeTranslator{"es": {
		"assets.forms.asset.field.name": "Nombre (resuelto)",
		"assets.forms.asset.tab.a":      "Pestaña A",
	}}
	e := &FormEngine{Translator: tr}
	rc := RenderContext{Locale: "es"}
	html := mi.RenderToString(e.Render(context.Background(), schema, nil, nil, rc))

	if !strings.Contains(html, "Nombre (resuelto)") {
		t.Errorf("expected the resolved field title in the rendered output:\n%s", html)
	}
	if strings.Contains(html, "Name (literal)") {
		t.Errorf("literal title should not appear once the key resolves:\n%s", html)
	}
	if !strings.Contains(html, "Pestaña A") {
		t.Errorf("expected the resolved tab label:\n%s", html)
	}
}

func TestRender_EndToEnd_FallsBackToLiteralWithoutTranslator(t *testing.T) {
	// Same schema, no Translator -- confirms the whole pipeline still
	// renders using literal text, unchanged from every stage before T-13.
	schema := []byte(`{
		"type": "object",
		"x-seam-form": {"module": "assets", "name": "asset"},
		"properties": {"name": {"type": "string", "title": "Name (literal)"}}
	}`)
	e := &FormEngine{}
	html := mi.RenderToString(e.Render(context.Background(), schema, nil, nil, RenderContext{}))
	if !strings.Contains(html, "Name (literal)") {
		t.Errorf("expected the literal title with no Translator configured:\n%s", html)
	}
}

// ============================================================================
// Adversarial tests below.
// ============================================================================

// TestTryKeys_TranslationEqualToItsOwnKey_TreatedAsNotFound documents a
// real, if narrow, limitation the package's own comment on tryKeys
// already flags: "result != key" is the only available not-found
// signal (internal/i18n.Translator has no separate "does this key
// exist" method), so a locale file entry whose translated VALUE
// happens to equal its own KEY string (a real, if odd, possibility --
// a placeholder left in during translation work, or literally the
// same word in two languages) is indistinguishable from "not
// translated" and silently falls through to the next key in the chain
// or the literal fallback. Not fixed here (fixing it would need
// internal/i18n.Translator to grow a real "exists" method, a bigger,
// cross-package change) -- this test exists so the limitation is
// demonstrated and locked in as a known, deliberate trade-off, not
// something a future reader has to rediscover by reading a comment and
// trusting it.
func TestTryKeys_TranslationEqualToItsOwnKey_TreatedAsNotFound(t *testing.T) {
	tr := fakeTranslator{
		"en": {"field.odd": "field.odd"}, // a real entry, coincidentally equal to its key
	}
	result, ok := tryKeys(tr, "en", "field.odd")
	if ok {
		t.Error("expected tryKeys to report \"not found\" for a translation equal to its own key -- if this now passes, the underlying Translator contract changed and this comment/test pair should be revisited together")
	}
	if result != "" {
		t.Errorf("expected an empty result alongside ok=false, got %q", result)
	}
}

// TestResolveTitle_GarbageLocale_DegradesGracefully confirms a
// malformed or unexpected rc.Locale value (empty, garbage, injection-
// shaped) can't do anything worse than a lookup miss -- internal/
// i18n.Translator.T resolves via a plain map[locale][key] lookup
// (confirmed directly from its source), so any string is a safe map
// key in Go regardless of content; there's no file path, no query, no
// eval anywhere in this chain for a malformed locale to reach.
func TestResolveTitle_GarbageLocale_DegradesGracefully(t *testing.T) {
	tr := fakeTranslator{"en": {"field.name": "Name"}}
	e := &FormEngine{Translator: tr}
	cases := []struct {
		name   string
		locale string
	}{
		{"empty", ""},
		{"not-a-real-locale", "not-a-real-locale"},
		{"path-traversal-shaped", "../../etc/passwd"},
		{"embedded-null-byte", "en\x00es"},
		{"very-long", strings.Repeat("x", 10000)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			defer func() {
				if rec := recover(); rec != nil {
					t.Fatalf("resolveTitle panicked on locale %q: %v", c.name, rec)
				}
			}()
			got := resolveTitle(e, RenderContext{Locale: c.locale}, "name", "Name (literal)")
			if got != "Name (literal)" {
				t.Errorf("locale %q: expected literal fallback %q, got %q", c.name, "Name (literal)", got)
			}
		})
	}
}

// TestAllResolvers_NilTranslator_NeverPanic sweeps every resolver
// function in this file against a nil Translator and an otherwise
// zero-value RenderContext -- TestResolveTitle_NilTranslatorFallsBackToLiteral
// already covers resolveTitle specifically; this confirms the same
// property holds for every other resolver, not just the one that
// happened to get its own test first.
func TestAllResolvers_NilTranslator_NeverPanic(t *testing.T) {
	e := &FormEngine{}
	rc := RenderContext{}
	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("a resolver panicked with a nil Translator: %v", rec)
		}
	}()
	if got := resolveTitle(e, rc, "f", "literal"); got != "literal" {
		t.Errorf("resolveTitle: got %q, want literal fallback", got)
	}
	if got := resolveHelp(e, rc, "f", "help literal"); got != "help literal" {
		t.Errorf("resolveHelp: got %q, want literal fallback", got)
	}
	if got := resolveLabel(e, rc, "label literal"); got != "label literal" {
		t.Errorf("resolveLabel: got %q, want literal fallback", got)
	}
	if got := resolveEnumOptionLabel(e, rc, "f", "SomeValue"); got != "SomeValue" {
		t.Errorf("resolveEnumOptionLabel: got %q, want the literal value unchanged", got)
	}
	if got := resolveOptionGroupLabel(e, rc, "f", "category"); got != "category" {
		t.Errorf("resolveOptionGroupLabel: got %q, want the literal category unchanged", got)
	}
	if got := resolveSelectPlaceholder(e, rc, "field.select.empty", "— Select —"); got != "— Select —" {
		t.Errorf("resolveSelectPlaceholder: got %q, want literal fallback", got)
	}
	if got := resolveDocumentLinkItemLabel(e, rc, "name", "Name literal"); got != "Name literal" {
		t.Errorf("resolveDocumentLinkItemLabel: got %q, want literal fallback", got)
	}
	if got := resolveWidgetChrome(e, rc, "some.key", "chrome literal"); got != "chrome literal" {
		t.Errorf("resolveWidgetChrome: got %q, want literal fallback", got)
	}
}

// TestResolveEnumOptionLabel_EmptyFormOrValue_ReturnsValueUnchanged
// confirms the explicit early-return guards (rc.Form == "" ||
// value == "") actually fire -- without them, an empty rc.Form would
// build the key ".fieldname.value" (a real, if malformed-looking, key
// that could coincidentally collide with something), and an empty
// value would build "form.fieldname." (trailing dot) and attempt a
// real lookup for no reason.
func TestResolveEnumOptionLabel_EmptyFormOrValue_ReturnsValueUnchanged(t *testing.T) {
	tr := fakeTranslator{"en": {".status.": "SHOULD NEVER MATCH"}}
	e := &FormEngine{Translator: tr}

	if got := resolveEnumOptionLabel(e, RenderContext{Locale: "en", Form: ""}, "status", "Active"); got != "Active" {
		t.Errorf("empty Form: got %q, want the literal value \"Active\" unchanged", got)
	}
	if got := resolveEnumOptionLabel(e, RenderContext{Locale: "en", Form: "asset"}, "status", ""); got != "" {
		t.Errorf("empty value: got %q, want empty string unchanged", got)
	}
}

// TestResolveEnumOptionLabel_UnicodeValue_LowercasesWithoutPanicking
// is a light correctness check, not a security one: strings.ToLower
// uses simple Unicode case folding, not locale-aware casing, so a
// value like Turkish "İstanbul" won't lowercase the way a Turkish
// locale's own rules would -- worth confirming this doesn't panic or
// produce garbage, even though a mismatched key (falling back to the
// literal value) is a minor correctness gap, not a crash or security
// issue.
func TestResolveEnumOptionLabel_UnicodeValue_LowercasesWithoutPanicking(t *testing.T) {
	e := &FormEngine{Translator: fakeTranslator{}}
	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("resolveEnumOptionLabel panicked on a Unicode value: %v", rec)
		}
	}()
	got := resolveEnumOptionLabel(e, RenderContext{Locale: "en", Form: "asset"}, "city", "İstanbul")
	if got != "İstanbul" {
		t.Errorf("expected the literal value unchanged (no matching key), got %q", got)
	}
}
