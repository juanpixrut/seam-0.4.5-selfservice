// Package schemaoverlay resolves form-definition schemas through layered
// overrides (campaign DEV-14, DEV-15, DEV-16, DEV-17).
//
// A form schema is a JSON document that the form engine renders. Seam ships
// them embedded in the binary; a developer or a tenant may override one.
// Overrides replace a schema whole (unlike translations, a JSON Schema is not
// meaningfully mergeable). Lookup order, highest precedence first:
//
//	filesystem   a developer's working copy; applies to every tenant
//	xolu-tenant  an override stored for one tenant
//	xolu-global  an override stored for all tenants
//	embedded     the schema compiled into the binary (supplied by the caller)
//
// Every override is validated before it can take effect: a candidate that is
// not valid JSON, or that the injected Validator rejects (in Seam,
// formengine.ParseSchema), is refused and whatever was in force stays in
// force. Reloads are all-or-nothing for the same reason.
//
// The package depends only on the standard library.
package schemaoverlay

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
)

// Layer names one source of schemas.
type Layer string

const (
	// LayerFilesystem holds overrides read from a directory on disk.
	LayerFilesystem Layer = "filesystem"
	// LayerXoluTenant holds overrides stored for a single tenant.
	LayerXoluTenant Layer = "xolu-tenant"
	// LayerXoluGlobal holds overrides stored for every tenant.
	LayerXoluGlobal Layer = "xolu-global"
	// LayerEmbedded is the base compiled into the binary.
	LayerEmbedded Layer = "embedded"
)

// Key identifies a schema: the module package it belongs to and its name,
// for example {"system", "user_edit"}.
type Key struct {
	Module string
	Name   string
}

// String returns "module/name".
func (k Key) String() string { return k.Module + "/" + k.Name }

func lessKey(a, b Key) bool {
	if a.Module != b.Module {
		return a.Module < b.Module
	}
	return a.Name < b.Name
}

// Validator decides whether a candidate schema is acceptable. It is called
// after the JSON syntax check. A nil Validator checks syntax only.
type Validator func(schema []byte) error

// Options are per-tenant switches for testing against fewer layers. The zero
// value is normal behaviour.
type Options struct {
	// DisableFilesystem ignores the filesystem layer.
	DisableFilesystem bool
	// DisableXolu ignores both xolu layers.
	DisableXolu bool
}

// Resolver holds the override layers. It is safe for concurrent use.
type Resolver struct {
	validate Validator

	mu     sync.RWMutex
	fs     map[Key][]byte
	global map[Key][]byte
	tenant map[int64]map[Key][]byte
	opts   map[int64]Options
}

// New returns an empty Resolver. validate may be nil.
func New(validate Validator) *Resolver {
	return &Resolver{validate: validate}
}

// Validate checks a candidate schema without storing it. The Developer
// editor calls this before saving.
func (r *Resolver) Validate(schema []byte) error {
	if !json.Valid(schema) {
		return errors.New("not valid JSON")
	}
	if r.validate != nil {
		return r.validate(schema)
	}
	return nil
}

// validateAll checks every schema, reporting all problems together, and
// returns a defensive copy of the map when they all pass.
func (r *Resolver) validateAll(m map[Key][]byte) (map[Key][]byte, error) {
	keys := make([]Key, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return lessKey(keys[i], keys[j]) })

	var errs []error
	cp := make(map[Key][]byte, len(m))
	for _, k := range keys {
		if err := r.Validate(m[k]); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", k, err))
			continue
		}
		cp[k] = append([]byte(nil), m[k]...)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return cp, nil
}

// ReloadFilesystem atomically replaces the whole filesystem layer. If any
// schema is invalid nothing changes. A nil or empty map clears the layer.
func (r *Resolver) ReloadFilesystem(m map[Key][]byte) error {
	cp, err := r.validateAll(m)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(cp) == 0 {
		cp = nil
	}
	r.fs = cp
	return nil
}

// ReloadXoluGlobal atomically replaces the whole xolu-global layer.
func (r *Resolver) ReloadXoluGlobal(m map[Key][]byte) error {
	cp, err := r.validateAll(m)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(cp) == 0 {
		cp = nil
	}
	r.global = cp
	return nil
}

// ReloadXoluTenant atomically replaces one tenant's xolu layer. tenant must
// be positive.
func (r *Resolver) ReloadXoluTenant(tenant int64, m map[Key][]byte) error {
	if tenant <= 0 {
		return fmt.Errorf("schemaoverlay: tenant layer needs a positive tenant ID, got %d", tenant)
	}
	cp, err := r.validateAll(m)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(cp) == 0 {
		delete(r.tenant, tenant)
		return nil
	}
	if r.tenant == nil {
		r.tenant = make(map[int64]map[Key][]byte)
	}
	r.tenant[tenant] = cp
	return nil
}

// SetOptions sets the per-tenant options. The zero Options clears them.
func (r *Resolver) SetOptions(tenant int64, o Options) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if o == (Options{}) {
		delete(r.opts, tenant)
		return
	}
	if r.opts == nil {
		r.opts = make(map[int64]Options)
	}
	r.opts[tenant] = o
}

// Options returns the tenant's options.
func (r *Resolver) Options(tenant int64) Options {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.opts[tenant]
}

func (r *Resolver) lookupLocked(tenant int64, k Key) ([]byte, Layer, bool) {
	o := r.opts[tenant]
	if !o.DisableFilesystem {
		if b, ok := r.fs[k]; ok {
			return b, LayerFilesystem, true
		}
	}
	if !o.DisableXolu {
		if tenant > 0 {
			if b, ok := r.tenant[tenant][k]; ok {
				return b, LayerXoluTenant, true
			}
		}
		if b, ok := r.global[k]; ok {
			return b, LayerXoluGlobal, true
		}
	}
	return nil, LayerEmbedded, false
}

// Resolve returns the schema in force for the tenant: the highest-precedence
// override if there is one, otherwise base. The returned slice is shared and
// must not be modified. Tenant 0 means no tenant (global view).
//
// A nil *Resolver is valid and always returns base, so callers need no check
// when the Developer module is off.
func (r *Resolver) Resolve(tenant int64, module, name string, base []byte) []byte {
	if r == nil {
		return base
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if b, _, ok := r.lookupLocked(tenant, Key{module, name}); ok {
		return b
	}
	return base
}

// EffectiveLayer reports which layer supplies a schema for the tenant:
// an override layer, or LayerEmbedded when there is no override.
func (r *Resolver) EffectiveLayer(tenant int64, module, name string) Layer {
	if r == nil {
		return LayerEmbedded
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, l, _ := r.lookupLocked(tenant, Key{module, name})
	return l
}

// Candidate is one layer's version of a schema.
type Candidate struct {
	Layer     Layer
	Schema    []byte
	Disabled  bool // switched off by the tenant's Options
	Effective bool // the one Resolve would return
}

// Explain lists every override layer holding the schema, highest precedence
// first, marking the effective one. The embedded base is not included; the
// caller owns it.
func (r *Resolver) Explain(tenant int64, module, name string) []Candidate {
	r.mu.RLock()
	defer r.mu.RUnlock()
	k := Key{module, name}
	o := r.opts[tenant]

	var out []Candidate
	if b, ok := r.fs[k]; ok {
		out = append(out, Candidate{Layer: LayerFilesystem, Schema: b, Disabled: o.DisableFilesystem})
	}
	if tenant > 0 {
		if b, ok := r.tenant[tenant][k]; ok {
			out = append(out, Candidate{Layer: LayerXoluTenant, Schema: b, Disabled: o.DisableXolu})
		}
	}
	if b, ok := r.global[k]; ok {
		out = append(out, Candidate{Layer: LayerXoluGlobal, Schema: b, Disabled: o.DisableXolu})
	}
	for i := range out {
		if !out[i].Disabled {
			out[i].Effective = true
			break
		}
	}
	return out
}

// Overridden returns the keys that have at least one override layer visible
// to the tenant (ignoring Options), sorted.
func (r *Resolver) Overridden(tenant int64) []Key {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := map[Key]struct{}{}
	for k := range r.fs {
		seen[k] = struct{}{}
	}
	for k := range r.global {
		seen[k] = struct{}{}
	}
	if tenant > 0 {
		for k := range r.tenant[tenant] {
			seen[k] = struct{}{}
		}
	}
	out := make([]Key, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return lessKey(out[i], out[j]) })
	return out
}
