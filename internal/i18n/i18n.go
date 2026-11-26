// Package i18n provides internationalization support for Seam AMS.
//
// It loads locale files from embedded JSON files and provides
// a simple translation function that supports parameter substitution.
//
// Supported languages:
//   - en: English
//   - es: Spanish (Rioplatense, without voseo)
//   - pt: Portuguese (Brazilian)
//   - ja: Japanese
//
// New locales require nothing beyond dropping a same-shaped
// locales/<code>.json file in this package and rebuilding -- New()
// discovers every *.json file generically (no hardcoded locale list),
// and AvailableLanguages/ValidateLocale downstream are equally
// generic. Confirmed directly: adding ja.json required zero changes
// to the Translator, the Settings language dropdown, or the
// locale-validation/persistence path -- only one test asserting an
// exact locale count needed updating to reflect the new total.
//
// Locale codes may be region-qualified with an underscore (e.g.
// "es_uy"), though no region-specific locale files exist yet -- see
// Translator.ResolveLocale for how a requested code (base or
// region-qualified) matches against whatever's actually loaded.
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

//go:embed locales/*.json
var localeFS embed.FS

// Translator provides translation functions for the application.
type Translator struct {
	messages map[string]map[string]string // locale -> key -> message (the embedded base)
	fallback string
	mu       sync.RWMutex

	// Layered overrides (overlay.go). Both are nil until first used and
	// are guarded by mu.
	overlays map[overlayKey]map[string]map[string]string // layer -> locale -> key -> message
	options  map[int64]Options                           // tenant -> lookup options
}

// New creates a new Translator with the specified fallback locale.
// EmbeddedLocale returns the built-in file of a locale exactly as it is in the
// source tree, and false when there is none. The Developer module's export
// applies a tenant's overrides to it.
func EmbeddedLocale(locale string) ([]byte, bool) {
	if !localeNameRE.MatchString(locale) {
		return nil, false
	}
	data, err := localeFS.ReadFile("locales/" + locale + ".json")
	if err != nil {
		return nil, false
	}
	return data, true
}

func New(fallback string) (*Translator, error) {
	t := &Translator{
		messages: make(map[string]map[string]string),
		fallback: fallback,
	}

	entries, err := localeFS.ReadDir("locales")
	if err != nil {
		return nil, fmt.Errorf("failed to read locales directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := localeFS.ReadFile("locales/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("failed to read locale file %s: %w", entry.Name(), err)
		}

		locale := strings.TrimSuffix(entry.Name(), ".json")

		var messages map[string]string
		if err := json.Unmarshal(data, &messages); err != nil {
			return nil, fmt.Errorf("failed to parse locale file %s: %w", entry.Name(), err)
		}

		t.messages[locale] = messages
	}

	// Verify fallback locale exists
	if _, ok := t.messages[fallback]; !ok {
		return nil, fmt.Errorf("fallback locale %q not found", fallback)
	}

	return t, nil
}

// T translates a key to the specified locale.
// If the key is not found in the locale, it falls back to the default locale.
// If still not found, it returns the key itself.
// Arguments can be passed for fmt.Sprintf substitution.
//
// Global overrides (filesystem and xolu-global layers, see overlay.go) apply;
// tenant-specific ones need ForTenant.
func (t *Translator) T(locale, key string, args ...any) string {
	return t.tenantT(0, locale, key, args...)
}

// ForLocale returns a translation function bound to a specific locale.
// This is convenient for passing to templates or handlers.
func (t *Translator) ForLocale(locale string) func(key string, args ...any) string {
	return func(key string, args ...any) string {
		return t.T(locale, key, args...)
	}
}

// HasLocale returns true if the locale is loaded.
func (t *Translator) HasLocale(locale string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	_, ok := t.localeSetLocked()[locale]
	return ok
}

// LocaleCount returns the number of loaded locales.
func (t *Translator) LocaleCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.localeSetLocked())
}

// Locales returns a list of all loaded locale codes.
func (t *Translator) Locales() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	set := t.localeSetLocked()
	locales := make([]string, 0, len(set))
	for locale := range set {
		locales = append(locales, locale)
	}
	return locales
}

// Fallback returns the fallback locale code.
func (t *Translator) Fallback() string {
	return t.fallback
}

// NewFromMessages builds a *Translator directly from a caller-supplied
// message set, bypassing New()'s embedded-FS loading. Exported
// specifically for cross-package test support -- other packages'
// tests (internal/ui's, for the dynamic Asset form's locale wiring)
// need to verify end-to-end resolution against known content, and the
// real embedded locales don't yet have matching keys for what they
// need to test (T-17 gap 3, still open). Not for production use.
func NewFromMessages(fallback string, messages map[string]map[string]string) *Translator {
	return &Translator{messages: messages, fallback: fallback}
}

// ResolveLocale finds the best loaded locale for a requested code,
// supporting both directions of base/region matching -- a requested
// base code ("en") matches any loaded region-qualified variant
// sharing that base ("en_gb"), and a requested region-qualified code
// ("es_uy") matches a loaded base locale ("es") or a different region
// sharing the same base ("es_es") if the exact code isn't loaded.
// Region separator is "_" (matching the codes already in use
// elsewhere in this codebase), not BCP-47's hyphen.
//
// Returns the actual locale to load and whether it was an exact
// match. An empty locale means nothing in that language family is
// loaded at all -- callers should treat that as "unresolvable", not
// silently use the empty string as a locale.
//
// Deterministic when the exact code and the exact base aren't loaded
// but multiple region variants are: picks the alphabetically-first
// candidate, not map iteration order.
func (t *Translator) ResolveLocale(requested string) (locale string, exact bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	set := t.localeSetLocked()
	if _, found := set[requested]; found {
		return requested, true
	}

	base := requested
	if i := strings.Index(requested, "_"); i >= 0 {
		base = requested[:i]
	}

	if _, found := set[base]; found {
		return base, false
	}

	var candidates []string
	for candidate := range set {
		if strings.HasPrefix(candidate, base+"_") {
			candidates = append(candidates, candidate)
		}
	}
	if len(candidates) == 0 {
		return "", false
	}
	sort.Strings(candidates)
	return candidates[0], false
}
