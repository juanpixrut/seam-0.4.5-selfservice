package overrides

import (
	"context"
	"sort"
	"strconv"
	"sync"
)

// MemoryStore keeps overrides in memory (tests, and runs without xolu).
type MemoryStore struct {
	mu      sync.Mutex
	rev     uint64 // counts changes, for Fingerprint
	texts   map[trKey]Translation
	history map[trKey][]TranslationVersion // oldest first
	schemas map[scKey][]SchemaVersion      // oldest first
}

type trKey struct {
	tenant      int64
	locale, key string
}

type scKey struct {
	tenant       int64
	module, name string
}

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{texts: map[trKey]Translation{}, history: map[trKey][]TranslationVersion{}, schemas: map[scKey][]SchemaVersion{}}
}

// positive refuses a tenant no override can be stored under; GlobalTenant (0)
// is one, and whether it may be used is the Service's decision.
func positive(tenant int64) error {
	if tenant < 0 {
		return ErrNoTenant
	}
	return nil
}

// TenantsWithOverrides implements Store.
func (s *MemoryStore) TenantsWithOverrides(ctx context.Context) ([]int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := map[int64]bool{}
	for k := range s.texts {
		seen[k.tenant] = true
	}
	for k := range s.schemas {
		seen[k.tenant] = true
	}
	return sortedTenants(seen), nil
}

func sortedTenants(seen map[int64]bool) []int64 {
	out := make([]int64, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Translations implements Store.
func (s *MemoryStore) Translations(ctx context.Context, tenant int64) ([]Translation, error) {
	if err := positive(tenant); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Translation
	for k, t := range s.texts {
		if k.tenant == tenant {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Locale != out[j].Locale {
			return out[i].Locale < out[j].Locale
		}
		return out[i].Key < out[j].Key
	})
	return out, nil
}

// SetTranslation implements Store.
func (s *MemoryStore) SetTranslation(ctx context.Context, t Translation) error {
	if err := positive(t.Tenant); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.texts[trKey{t.Tenant, t.Locale, t.Key}] = t
	s.rev++
	return nil
}

// RemoveTranslation implements Store.
func (s *MemoryStore) RemoveTranslation(ctx context.Context, tenant int64, locale, key string) error {
	if err := positive(tenant); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.texts[trKey{tenant, locale, key}]; ok {
		delete(s.texts, trKey{tenant, locale, key})
		s.rev++
	}
	return nil
}

// AppendTranslationVersion implements Store.
func (s *MemoryStore) AppendTranslationVersion(ctx context.Context, v TranslationVersion) (TranslationVersion, error) {
	if err := positive(v.Tenant); err != nil {
		return TranslationVersion{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	k := trKey{v.Tenant, v.Locale, v.Key}
	v.Version = 1
	if h := s.history[k]; len(h) > 0 {
		v.Version = h[len(h)-1].Version + 1
	}
	s.history[k] = append(s.history[k], v)
	return v, nil
}

// TranslationHistory implements Store.
func (s *MemoryStore) TranslationHistory(ctx context.Context, tenant int64, locale, key string) ([]TranslationVersion, error) {
	if err := positive(tenant); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	h := s.history[trKey{tenant, locale, key}]
	out := make([]TranslationVersion, 0, len(h))
	for i := len(h) - 1; i >= 0; i-- {
		out = append(out, h[i])
	}
	return out, nil
}

// CurrentSchemas implements Store.
func (s *MemoryStore) CurrentSchemas(ctx context.Context, tenant int64) ([]SchemaVersion, error) {
	if err := positive(tenant); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []SchemaVersion
	for k, vs := range s.schemas {
		if k.tenant != tenant || len(vs) == 0 {
			continue
		}
		if v := vs[len(vs)-1]; !v.Removed {
			out = append(out, copyVersion(v))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Module != out[j].Module {
			return out[i].Module < out[j].Module
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func copyVersion(v SchemaVersion) SchemaVersion {
	v.Doc = append([]byte(nil), v.Doc...)
	return v
}

// AppendSchema implements Store.
func (s *MemoryStore) AppendSchema(ctx context.Context, v SchemaVersion) (SchemaVersion, error) {
	if err := positive(v.Tenant); err != nil {
		return SchemaVersion{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	k := scKey{v.Tenant, v.Module, v.Name}
	v.Version = 1
	if h := s.schemas[k]; len(h) > 0 {
		v.Version = h[len(h)-1].Version + 1
	}
	v = copyVersion(v)
	s.schemas[k] = append(s.schemas[k], v)
	s.rev++
	return copyVersion(v), nil
}

// SchemaHistory implements Store.
func (s *MemoryStore) SchemaHistory(ctx context.Context, tenant int64, module, name string) ([]SchemaVersion, error) {
	if err := positive(tenant); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	h := s.schemas[scKey{tenant, module, name}]
	out := make([]SchemaVersion, 0, len(h))
	for i := len(h) - 1; i >= 0; i-- {
		out = append(out, copyVersion(h[i]))
	}
	return out, nil
}

// Fingerprint implements Store.
func (s *MemoryStore) Fingerprint(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strconv.FormatUint(s.rev, 10), nil
}
