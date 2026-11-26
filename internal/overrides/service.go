package overrides

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/ha1tch/seam-ui/internal/i18n"
	"github.com/ha1tch/seam-ui/internal/schemaoverlay"
)

// MaxNoteLen is the longest note kept with a form-definition version.
const MaxNoteLen = 200

// Errors of the version operations.
var (
	ErrUnknownVersion = errors.New("overrides: no such version")
	ErrNotRestorable  = errors.New("overrides: that version is a removal and has no definition to restore")
)

// Service applies the overrides in a Store to a translator's and a resolver's
// xolu-tenant layers. Every change is validated with the same rules as the
// filesystem layers (i18n.CheckOverride, schemaoverlay.Resolver.CheckDocument),
// stored, and then the tenant's layer is rebuilt from the store, so what is live
// is always what is stored. The tenant is always the caller's, never a request's.
type Service struct {
	store Store
	tr    *i18n.Translator
	res   *schemaoverlay.Resolver

	mu  sync.Mutex // one change at a time: store write and reload are a unit
	now func() time.Time

	// loaded are the tenants whose layers hold stored overrides, so a tenant
	// that has just lost its last one elsewhere is cleared here too (Sync);
	// lastFP is the store's fingerprint when everything was last applied.
	loaded map[int64]bool
	lastFP string
	synced bool

	// global says the global scope is in use (AllowGlobal): overrides stored
	// under GlobalTenant are applied to every tenant and may be written.
	global bool
}

// GlobalTenant is the tenant ID the overrides that apply to every tenant are
// stored under (campaign DEV-26). They sit below a tenant's own overrides and
// above the built-in texts and forms (DEC-4).
const GlobalTenant int64 = 0

// AllowGlobal switches the global scope on: from then on the overrides stored
// under GlobalTenant are loaded and applied, and may be written through the
// same methods with GlobalTenant as the tenant. Without it they are neither
// read nor written, so rows left by a deployment that used the scope are
// inert. Call it before Load. Who may write them is the caller's business
// (the Developer module asks the configured home tenant).
func (s *Service) AllowGlobal() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.global = true
}

// allowed reports whether overrides may be written for tenant.
func (s *Service) allowed(tenant int64) bool {
	return tenant > 0 || (tenant == GlobalTenant && s.global)
}

// NewService returns a Service. Either of tr and res may be nil, in which case
// that kind of override is stored but not applied.
func NewService(store Store, tr *i18n.Translator, res *schemaoverlay.Resolver) *Service {
	return &Service{store: store, tr: tr, res: res, now: time.Now, loaded: map[int64]bool{}}
}

// Load applies every tenant's stored overrides; it is run once at start. It goes
// on past a tenant that fails and returns every failure.
func (s *Service) Load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked(ctx)
}

// Sync applies the stored overrides again, but only if they differ from what
// this process last applied: it is how a change another instance made is noticed
// (campaign DEV-28). It reports whether it applied anything. A change made by
// this process itself also shows as a difference, and is applied again, which is
// harmless.
func (s *Service) Sync(ctx context.Context) (bool, error) {
	fp, err := s.store.Fingerprint(ctx)
	if err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.synced && fp == s.lastFP {
		return false, nil
	}
	return true, s.loadLocked(ctx)
}

func (s *Service) loadLocked(ctx context.Context) error {
	// The fingerprint is read first: a change that lands while this runs makes
	// the next Sync apply again, so it can be applied twice but never missed.
	fp, err := s.store.Fingerprint(ctx)
	if err != nil {
		return err
	}
	tenants, err := s.store.TenantsWithOverrides(ctx)
	if err != nil {
		return err
	}
	inStore := map[int64]bool{}
	todo := map[int64]bool{}
	for _, t := range tenants {
		if t <= 0 {
			continue // the global scope's rows are for AllowGlobal alone, below
		}
		inStore[t], todo[t] = true, true
	}
	for t := range s.loaded {
		todo[t] = true // one that has just lost its last override is cleared
	}
	if s.global {
		inStore[GlobalTenant], todo[GlobalTenant] = true, true
	}
	ordered := make([]int64, 0, len(todo))
	for t := range todo {
		ordered = append(ordered, t)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	var errs []error
	for _, t := range ordered {
		if err := s.reloadTranslations(ctx, t); err != nil {
			errs = append(errs, fmt.Errorf("tenant %d translations: %w", t, err))
		}
		if err := s.reloadSchemas(ctx, t); err != nil {
			errs = append(errs, fmt.Errorf("tenant %d form definitions: %w", t, err))
		}
		if !inStore[t] {
			delete(s.loaded, t)
		}
	}
	// recorded even when something failed: the same stored state would fail the
	// same way, and Sync must not repeat it every tick
	s.lastFP, s.synced = fp, true
	return errors.Join(errs...)
}

func (s *Service) reloadTranslations(ctx context.Context, tenant int64) error {
	s.loaded[tenant] = true
	if s.tr == nil {
		return nil
	}
	rows, err := s.store.Translations(ctx, tenant)
	if err != nil {
		return err
	}
	byLocale := map[string]map[string]string{}
	for _, r := range rows {
		if byLocale[r.Locale] == nil {
			byLocale[r.Locale] = map[string]string{}
		}
		byLocale[r.Locale][r.Key] = r.Value
	}
	if tenant == GlobalTenant {
		return s.tr.ReloadLayer(i18n.LayerXoluGlobal, 0, byLocale)
	}
	return s.tr.ReloadLayer(i18n.LayerXoluTenant, tenant, byLocale)
}

func (s *Service) reloadSchemas(ctx context.Context, tenant int64) error {
	if s.res == nil {
		return nil
	}
	cur, err := s.store.CurrentSchemas(ctx, tenant)
	if err != nil {
		return err
	}
	m := map[schemaoverlay.Key][]byte{}
	for _, v := range cur {
		m[schemaoverlay.Key{Module: v.Module, Name: v.Name}] = v.Doc
	}
	if tenant == GlobalTenant {
		return s.res.ReloadXoluGlobal(m)
	}
	return s.res.ReloadXoluTenant(tenant, m)
}

// ---- translations ----

// Translations returns the tenant's overridden texts.
func (s *Service) Translations(ctx context.Context, tenant int64) ([]Translation, error) {
	return s.store.Translations(ctx, tenant)
}

// SetTranslation validates and stores one text for the tenant and applies it.
// reference is the text it replaces, for the placeholder check ("" skips it).
func (s *Service) SetTranslation(ctx context.Context, tenant int64, locale, key, value, reference string, by int64) error {
	if !s.allowed(tenant) {
		return ErrNoTenant
	}
	value, err := i18n.CheckOverride(locale, key, value, reference)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.setTranslationLocked(ctx, tenant, locale, key, value, by)
}

// setTranslationLocked stores the text, applies it and records the version.
// The history is written last and its failure is returned: the text is then
// already live, and the caller is told its history is incomplete.
func (s *Service) setTranslationLocked(ctx context.Context, tenant int64, locale, key, value string, by int64) error {
	now := s.now()
	if err := s.backfillTranslationLocked(ctx, tenant, locale, key); err != nil {
		return err
	}
	if err := s.store.SetTranslation(ctx, Translation{Tenant: tenant, Locale: locale, Key: key, Value: value, ChangedBy: by, ChangedAt: now}); err != nil {
		return err
	}
	if err := s.reloadTranslations(ctx, tenant); err != nil {
		return err
	}
	_, err := s.store.AppendTranslationVersion(ctx, TranslationVersion{Tenant: tenant, Locale: locale, Key: key, Value: value, ChangedBy: by, ChangedAt: now})
	return err
}

// backfillTranslationLocked records, as the first version of a text that has
// no history yet, the override that is there now: one saved before the history
// existed. It is run before a change, so the wording about to be replaced or
// removed can still be restored.
func (s *Service) backfillTranslationLocked(ctx context.Context, tenant int64, locale, key string) error {
	history, err := s.store.TranslationHistory(ctx, tenant, locale, key)
	if err != nil || len(history) > 0 {
		return err
	}
	rows, err := s.store.Translations(ctx, tenant)
	if err != nil {
		return err
	}
	for _, r := range rows {
		if r.Locale == locale && r.Key == key {
			_, err = s.store.AppendTranslationVersion(ctx, TranslationVersion{Tenant: tenant, Locale: locale, Key: key, Value: r.Value,
				ChangedBy: r.ChangedBy, ChangedAt: r.ChangedAt})
			return err
		}
	}
	return nil
}

// TranslationHistory returns up to MaxHistory versions of one of the tenant's
// texts, newest first. A text overridden before the history existed, and not
// changed since, shows its current wording as its one version.
func (s *Service) TranslationHistory(ctx context.Context, tenant int64, locale, key string) ([]TranslationVersion, error) {
	if !s.allowed(tenant) {
		return nil, ErrNoTenant
	}
	h, err := s.store.TranslationHistory(ctx, tenant, locale, key)
	if err == nil && len(h) == 0 {
		rows, rerr := s.store.Translations(ctx, tenant)
		if rerr != nil {
			return nil, rerr
		}
		for _, r := range rows {
			if r.Locale == locale && r.Key == key {
				h = []TranslationVersion{{Tenant: tenant, Locale: locale, Key: key, Version: 1, Value: r.Value, ChangedBy: r.ChangedBy, ChangedAt: r.ChangedAt}}
			}
		}
	}
	if len(h) > MaxHistory {
		h = h[:MaxHistory]
	}
	return h, err
}

// RestoreTranslation makes an older version of a text current again, as a new
// version. reference is the text it replaces, for the placeholder check: the
// built-in text may have changed since the version was saved.
func (s *Service) RestoreTranslation(ctx context.Context, tenant int64, locale, key string, version int, reference string, by int64) (TranslationVersion, error) {
	if !s.allowed(tenant) {
		return TranslationVersion{}, ErrNoTenant
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	history, err := s.store.TranslationHistory(ctx, tenant, locale, key)
	if err != nil {
		return TranslationVersion{}, err
	}
	var old *TranslationVersion
	for i := range history {
		if history[i].Version == version {
			old = &history[i]
			break
		}
	}
	if old == nil {
		return TranslationVersion{}, ErrUnknownVersion
	}
	if old.Removed {
		return TranslationVersion{}, ErrNotRestorable
	}
	value, err := i18n.CheckOverride(locale, key, old.Value, reference)
	if err != nil {
		return TranslationVersion{}, err
	}
	if err := s.setTranslationLocked(ctx, tenant, locale, key, value, by); err != nil {
		return TranslationVersion{}, err
	}
	history, err = s.store.TranslationHistory(ctx, tenant, locale, key)
	if err != nil || len(history) == 0 {
		return TranslationVersion{}, err
	}
	return history[0], nil
}

// RemoveTranslation deletes one of the tenant's texts and applies that. A
// removal of a text that was there is recorded in its history; by is who.
func (s *Service) RemoveTranslation(ctx context.Context, tenant int64, locale, key string, by int64) error {
	if !s.allowed(tenant) {
		return ErrNoTenant
	}
	if _, err := i18n.CheckOverride(locale, key, "x", ""); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.backfillTranslationLocked(ctx, tenant, locale, key); err != nil {
		return err
	}
	rows, err := s.store.Translations(ctx, tenant)
	if err != nil {
		return err
	}
	was := false
	for _, r := range rows {
		if r.Locale == locale && r.Key == key {
			was = true
		}
	}
	if err := s.store.RemoveTranslation(ctx, tenant, locale, key); err != nil {
		return err
	}
	if err := s.reloadTranslations(ctx, tenant); err != nil || !was {
		return err
	}
	_, err = s.store.AppendTranslationVersion(ctx, TranslationVersion{Tenant: tenant, Locale: locale, Key: key, Removed: true, ChangedBy: by, ChangedAt: s.now()})
	return err
}

// ---- form definitions ----

func cleanNote(note string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(note) {
		if r >= 0x20 && r != 0x7f {
			b.WriteRune(r)
		}
	}
	n := b.String()
	if utf8.RuneCountInString(n) > MaxNoteLen {
		n = string([]rune(n)[:MaxNoteLen])
	}
	return n
}

func indent(data []byte) []byte {
	var out bytes.Buffer
	if err := json.Indent(&out, data, "", "  "); err != nil {
		return data
	}
	out.WriteByte('\n')
	return out.Bytes()
}

// SaveSchema validates doc as the tenant's override of module/name, stores it as
// a new version and applies it. base is the built-in definition, for the
// advisory compatibility notes that are returned beside the new version.
func (s *Service) SaveSchema(ctx context.Context, tenant int64, module, name string, doc, base []byte, by int64, note string) (SchemaVersion, []string, error) {
	if !s.allowed(tenant) {
		return SchemaVersion{}, nil, ErrNoTenant
	}
	if s.res == nil {
		return SchemaVersion{}, nil, errors.New("overrides: no schema resolver")
	}
	if err := s.res.CheckDocument(module, name, doc); err != nil {
		return SchemaVersion{}, nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.store.AppendSchema(ctx, SchemaVersion{Tenant: tenant, Module: module, Name: name, Doc: indent(doc),
		Note: cleanNote(note), ChangedBy: by, ChangedAt: s.now()})
	if err != nil {
		return SchemaVersion{}, nil, err
	}
	if err := s.reloadSchemas(ctx, tenant); err != nil {
		return v, nil, err
	}
	return v, schemaoverlay.Compat(base, doc), nil
}

// RemoveSchema removes the tenant's override of module/name by appending a
// removal to its history. Removing one that is not there changes nothing.
func (s *Service) RemoveSchema(ctx context.Context, tenant int64, module, name string, by int64) error {
	if !s.allowed(tenant) {
		return ErrNoTenant
	}
	if s.res != nil {
		if err := s.res.CheckDocument(module, name, []byte("{}")); err != nil && errors.Is(err, schemaoverlay.ErrInvalidName) {
			return err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, err := s.currentLocked(ctx, tenant, module, name)
	if err != nil || cur == nil {
		return err
	}
	if _, err := s.store.AppendSchema(ctx, SchemaVersion{Tenant: tenant, Module: module, Name: name, Removed: true,
		Note: "removed", ChangedBy: by, ChangedAt: s.now()}); err != nil {
		return err
	}
	return s.reloadSchemas(ctx, tenant)
}

// RestoreSchema makes an older version current again by appending a copy of it.
func (s *Service) RestoreSchema(ctx context.Context, tenant int64, module, name string, version int, by int64) (SchemaVersion, error) {
	if !s.allowed(tenant) {
		return SchemaVersion{}, ErrNoTenant
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	history, err := s.store.SchemaHistory(ctx, tenant, module, name)
	if err != nil {
		return SchemaVersion{}, err
	}
	var old *SchemaVersion
	for i := range history {
		if history[i].Version == version {
			old = &history[i]
			break
		}
	}
	if old == nil {
		return SchemaVersion{}, ErrUnknownVersion
	}
	if old.Removed {
		return SchemaVersion{}, ErrNotRestorable
	}
	// the engine may have changed since: an old definition is checked again
	if s.res != nil {
		if err := s.res.CheckDocument(module, name, old.Doc); err != nil {
			return SchemaVersion{}, err
		}
	}
	v, err := s.store.AppendSchema(ctx, SchemaVersion{Tenant: tenant, Module: module, Name: name, Doc: old.Doc,
		Note: fmt.Sprintf("restored from version %d", version), ChangedBy: by, ChangedAt: s.now()})
	if err != nil {
		return SchemaVersion{}, err
	}
	return v, s.reloadSchemas(ctx, tenant)
}

// History returns up to MaxHistory versions of the tenant's override of
// module/name, newest first.
func (s *Service) History(ctx context.Context, tenant int64, module, name string) ([]SchemaVersion, error) {
	h, err := s.store.SchemaHistory(ctx, tenant, module, name)
	if len(h) > MaxHistory {
		h = h[:MaxHistory]
	}
	return h, err
}

// Current returns the tenant's current override of module/name, if it has one.
func (s *Service) Current(ctx context.Context, tenant int64, module, name string) (SchemaVersion, bool, error) {
	v, err := s.currentLocked(ctx, tenant, module, name)
	if err != nil || v == nil {
		return SchemaVersion{}, false, err
	}
	return *v, true, nil
}

func (s *Service) currentLocked(ctx context.Context, tenant int64, module, name string) (*SchemaVersion, error) {
	cur, err := s.store.CurrentSchemas(ctx, tenant)
	if err != nil {
		return nil, err
	}
	for i := range cur {
		if cur[i].Module == module && cur[i].Name == name {
			return &cur[i], nil
		}
	}
	return nil, nil
}

// Overrides lists the tenant's current form-definition overrides.
func (s *Service) Overrides(ctx context.Context, tenant int64) ([]SchemaVersion, error) {
	return s.store.CurrentSchemas(ctx, tenant)
}
