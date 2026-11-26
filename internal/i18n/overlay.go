package i18n

import "fmt"

// This file adds layered overrides to the Translator (campaign DEV-11).
//
// A message is looked up through up to four layers, highest precedence
// first:
//
//	filesystem   a developer's working copy; applies to every tenant
//	xolu-tenant  an override stored for one tenant
//	xolu-global  an override stored for all tenants
//	embedded     the locale files compiled into the binary
//
// Overrides apply per key: a layer holds only the keys it changes, so a
// key added by a new release still resolves from the embedded base. The
// requested locale is resolved through every layer before the fallback
// locale is consulted, which preserves the existing rule that a missing
// key in the requested locale falls back to the default locale.
//
// With no overlays loaded and zero Options, behaviour is identical to the
// original Translator.

// Layer names one source of messages.
type Layer string

const (
	// LayerFilesystem holds overrides read from a directory on disk.
	LayerFilesystem Layer = "filesystem"
	// LayerXoluTenant holds overrides stored for a single tenant.
	LayerXoluTenant Layer = "xolu-tenant"
	// LayerXoluGlobal holds overrides stored for every tenant.
	LayerXoluGlobal Layer = "xolu-global"
	// LayerEmbedded is the base compiled into the binary. It cannot be
	// reloaded.
	LayerEmbedded Layer = "embedded"
)

// layerOrder is the lookup order, highest precedence first.
var layerOrder = []Layer{LayerFilesystem, LayerXoluTenant, LayerXoluGlobal, LayerEmbedded}

// Options are per-tenant switches that change how lookups behave. They
// exist for the Developer control panel; the zero value is normal
// behaviour.
type Options struct {
	// DisableFilesystem ignores the filesystem layer.
	DisableFilesystem bool
	// DisableXolu ignores both xolu layers.
	DisableXolu bool
	// ShowKeys makes T return the key itself instead of the message.
	ShowKeys bool
}

type overlayKey struct {
	layer  Layer
	tenant int64 // only meaningful for LayerXoluTenant
}

// ReloadLayer atomically replaces everything a layer holds, for all
// locales at once, so readers never observe a half-loaded layer.
// byLocale maps locale -> key -> message. The input is copied.
//
// For LayerXoluTenant, tenant must be positive. For LayerFilesystem and
// LayerXoluGlobal it must be zero. LayerEmbedded cannot be reloaded.
// A nil or empty byLocale clears the layer.
func (t *Translator) ReloadLayer(layer Layer, tenant int64, byLocale map[string]map[string]string) error {
	switch layer {
	case LayerFilesystem, LayerXoluGlobal:
		if tenant != 0 {
			return fmt.Errorf("i18n: layer %q is not tenant-specific (tenant must be 0, got %d)", layer, tenant)
		}
	case LayerXoluTenant:
		if tenant <= 0 {
			return fmt.Errorf("i18n: layer %q needs a positive tenant ID, got %d", layer, tenant)
		}
	case LayerEmbedded:
		return fmt.Errorf("i18n: layer %q cannot be reloaded", layer)
	default:
		return fmt.Errorf("i18n: unknown layer %q", layer)
	}

	cp := make(map[string]map[string]string, len(byLocale))
	for loc, msgs := range byLocale {
		if len(msgs) == 0 {
			continue
		}
		m := make(map[string]string, len(msgs))
		for k, v := range msgs {
			m[k] = v
		}
		cp[loc] = m
	}

	k := overlayKey{layer: layer, tenant: tenant}
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(cp) == 0 {
		delete(t.overlays, k)
		return nil
	}
	if t.overlays == nil {
		t.overlays = make(map[overlayKey]map[string]map[string]string)
	}
	t.overlays[k] = cp
	return nil
}

// SetOptions sets the per-tenant lookup options. The zero Options removes
// the tenant's entry.
func (t *Translator) SetOptions(tenant int64, o Options) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if o == (Options{}) {
		delete(t.options, tenant)
		return
	}
	if t.options == nil {
		t.options = make(map[int64]Options)
	}
	t.options[tenant] = o
}

// Options returns the tenant's lookup options.
func (t *Translator) Options(tenant int64) Options {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.options[tenant]
}

// TenantView is a Translator bound to one tenant. It is cheap to create
// and safe for concurrent use.
type TenantView struct {
	t      *Translator
	tenant int64
}

// ForTenant returns a view that applies the tenant's overrides and
// options. Tenant 0 is the global view, which is what Translator.T uses.
func (t *Translator) ForTenant(tenant int64) *TenantView {
	return &TenantView{t: t, tenant: tenant}
}

// T translates like Translator.T, through this tenant's layers.
func (v *TenantView) T(locale, key string, args ...any) string {
	return v.t.tenantT(v.tenant, locale, key, args...)
}

// ForLocale returns a translation function bound to a locale.
func (v *TenantView) ForLocale(locale string) func(key string, args ...any) string {
	return func(key string, args ...any) string {
		return v.t.tenantT(v.tenant, locale, key, args...)
	}
}

func (t *Translator) tenantT(tenant int64, locale, key string, args ...any) string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	opts := t.options[tenant]
	if opts.ShowKeys {
		return key
	}
	msg, _, ok := t.lookupLocked(tenant, opts, locale, key)
	if !ok {
		return key // last resort, as before
	}
	if len(args) > 0 {
		return fmt.Sprintf(msg, args...)
	}
	return msg
}

// layerGetLocked reads one layer. It reports false for a disabled layer.
func (t *Translator) layerGetLocked(l Layer, tenant int64, opts Options, locale, key string) (string, bool) {
	switch l {
	case LayerEmbedded:
		m, ok := t.messages[locale][key]
		return m, ok
	case LayerFilesystem:
		if opts.DisableFilesystem {
			return "", false
		}
		m, ok := t.overlays[overlayKey{layer: l}][locale][key]
		return m, ok
	case LayerXoluGlobal:
		if opts.DisableXolu {
			return "", false
		}
		m, ok := t.overlays[overlayKey{layer: l}][locale][key]
		return m, ok
	case LayerXoluTenant:
		if opts.DisableXolu || tenant <= 0 {
			return "", false
		}
		m, ok := t.overlays[overlayKey{layer: l, tenant: tenant}][locale][key]
		return m, ok
	}
	return "", false
}

// lookupLocked resolves key: the requested locale through every layer,
// then the fallback locale through every layer.
func (t *Translator) lookupLocked(tenant int64, opts Options, locale, key string) (string, Layer, bool) {
	locales := []string{locale}
	if t.fallback != locale {
		locales = append(locales, t.fallback)
	}
	for _, loc := range locales {
		for _, l := range layerOrder {
			if msg, ok := t.layerGetLocked(l, tenant, opts, loc, key); ok {
				return msg, l, true
			}
		}
	}
	return "", "", false
}

// Candidate is one layer's value for a key, for editors that need to show
// where a message comes from and what it shadows.
type Candidate struct {
	Layer    Layer
	Locale   string
	Message  string
	Disabled bool // the layer is switched off by Options
	// Effective marks the value T would return.
	Effective bool
}

// Explain lists every layer that holds key for locale, highest precedence
// first, marking the one that wins. If no layer holds it for locale, the
// fallback locale's candidates are listed instead. Layers switched off by
// the tenant's Options are listed but never effective.
func (t *Translator) Explain(tenant int64, locale, key string) []Candidate {
	t.mu.RLock()
	defer t.mu.RUnlock()

	opts := t.options[tenant]
	collect := func(loc string) []Candidate {
		var out []Candidate
		// Ignore Options when probing, then flag what Options disables.
		for _, l := range layerOrder {
			msg, ok := t.layerGetLocked(l, tenant, Options{}, loc, key)
			if !ok {
				continue
			}
			disabled := false
			switch l {
			case LayerFilesystem:
				disabled = opts.DisableFilesystem
			case LayerXoluGlobal, LayerXoluTenant:
				disabled = opts.DisableXolu
			}
			out = append(out, Candidate{Layer: l, Locale: loc, Message: msg, Disabled: disabled})
		}
		return out
	}
	mark := func(c []Candidate) []Candidate {
		for i := range c {
			if !c[i].Disabled {
				c[i].Effective = true
				break
			}
		}
		return c
	}

	if c := mark(collect(locale)); anyEffective(c) {
		return c
	}
	if t.fallback != locale {
		if f := mark(collect(t.fallback)); anyEffective(f) {
			// Keep disabled candidates of the requested locale visible
			// ahead of the fallback ones.
			return append(collect(locale), f...)
		}
	}
	return collect(locale)
}

func anyEffective(c []Candidate) bool {
	for _, x := range c {
		if x.Effective {
			return true
		}
	}
	return false
}

// Effective returns a copy of every message for one locale as the tenant
// sees it, merged across layers (the fallback locale is not merged in).
func (t *Translator) Effective(tenant int64, locale string) map[string]string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	opts := t.options[tenant]
	out := make(map[string]string, len(t.messages[locale]))
	// Lowest precedence first, so higher layers overwrite.
	for i := len(layerOrder) - 1; i >= 0; i-- {
		l := layerOrder[i]
		var src map[string]string
		switch l {
		case LayerEmbedded:
			src = t.messages[locale]
		case LayerFilesystem:
			if !opts.DisableFilesystem {
				src = t.overlays[overlayKey{layer: l}][locale]
			}
		case LayerXoluGlobal:
			if !opts.DisableXolu {
				src = t.overlays[overlayKey{layer: l}][locale]
			}
		case LayerXoluTenant:
			if !opts.DisableXolu && tenant > 0 {
				src = t.overlays[overlayKey{layer: l, tenant: tenant}][locale]
			}
		}
		for k, v := range src {
			out[k] = v
		}
	}
	return out
}

// localeSetLocked returns every locale that exists in the embedded base or
// in a global overlay (filesystem or xolu-global). Tenant-specific layers
// do not introduce locales.
func (t *Translator) localeSetLocked() map[string]struct{} {
	set := make(map[string]struct{}, len(t.messages))
	for loc := range t.messages {
		set[loc] = struct{}{}
	}
	for _, l := range []Layer{LayerFilesystem, LayerXoluGlobal} {
		for loc := range t.overlays[overlayKey{layer: l}] {
			set[loc] = struct{}{}
		}
	}
	return set
}
