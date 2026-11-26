// Package overrides stores the Developer module's tenant-scope overrides
// (campaign DEV-13, DEV-16): translation overrides and form-definition
// overrides kept in xolu, per tenant, and applied to the translator's and the
// resolver's xolu-tenant layers.
//
// Three entities hold them:
//
//	i18n_override    one row per (tenant_id, locale, key): value, changed_by, changed_at
//	i18n_override_history  an append-only history of the texts: one row per
//	                 version of a (tenant_id, locale, key), with the value or a
//	                 removed flag, changed_by and changed_at
//	schema_override  an append-only history: one row per version of a (tenant_id,
//	                 module, name), with the document (doc), a removed flag, a note,
//	                 changed_by and changed_at
//
// A form definition is never overwritten: saving appends a version, removing
// appends a version marked removed, and restoring appends a copy of an older
// one. The current definition is the newest version that is not removed, so the
// history and rollback cost nothing extra. A text keeps its current row (what
// every instance loads) and, beside it, a version in the history for each save
// and each removal, so an earlier wording can be restored. Texts saved before the
// history existed have none until they are next changed.
//
// Precedence between layers is unchanged (DEC-4): a filesystem override still
// outranks a tenant's database override, so the editors say so when one shadows
// the other. Every operation takes the tenant from the caller's session, never
// from a request field, and refuses a tenant that is not positive.
//
// Limits worth knowing: the version number is the highest stored plus one, which
// two instances saving at the same instant could duplicate (the history then
// sorts by version and time, and the later one wins); instances load overrides
// at start and reload a tenant's layer after each of their own saves, so a
// second instance sees another's change at its next restart (DEV-28); and
// whether xolu needs the entities declared before first use is unverified, so a
// failure to read them at start is logged, not fatal.
package overrides

import (
	"context"
	"errors"
	"time"
)

// The xolu entities.
const (
	EntityTranslation        = "i18n_override"
	EntityTranslationHistory = "i18n_override_history"
	EntitySchema             = "schema_override"
)

// MaxHistory is how many versions History returns, newest first.
const MaxHistory = 50

// ErrNoTenant is returned for a tenant that is not positive.
var ErrNoTenant = errors.New("overrides: a positive tenant is required")

// Translation is one overridden text of one tenant.
type Translation struct {
	Tenant    int64
	Locale    string
	Key       string
	Value     string
	ChangedBy int64
	ChangedAt time.Time
}

// TranslationVersion is one version of a tenant's override of one text. A
// removal is a version with Removed set and no value.
type TranslationVersion struct {
	Tenant    int64
	Locale    string
	Key       string
	Version   int
	Value     string
	Removed   bool
	ChangedBy int64
	ChangedAt time.Time
}

// SchemaVersion is one version of a tenant's override of module/name. A removal
// is a version with Removed set and no document.
type SchemaVersion struct {
	Tenant    int64
	Module    string
	Name      string
	Version   int
	Doc       []byte
	Removed   bool
	Note      string
	ChangedBy int64
	ChangedAt time.Time
}

// Store persists the overrides.
type Store interface {
	// TenantsWithOverrides lists the tenants that have any stored row.
	TenantsWithOverrides(ctx context.Context) ([]int64, error)

	// Translations returns the tenant's overridden texts.
	Translations(ctx context.Context, tenant int64) ([]Translation, error)
	// SetTranslation creates or replaces one text.
	SetTranslation(ctx context.Context, t Translation) error
	// RemoveTranslation deletes one text; removing an absent one is not an error.
	RemoveTranslation(ctx context.Context, tenant int64, locale, key string) error
	// AppendTranslationVersion records a version of one text in its history,
	// numbering it one more than the highest stored, and returns it as stored.
	// It does not change the current text.
	AppendTranslationVersion(ctx context.Context, v TranslationVersion) (TranslationVersion, error)
	// TranslationHistory returns every recorded version of one text, newest first.
	TranslationHistory(ctx context.Context, tenant int64, locale, key string) ([]TranslationVersion, error)

	// CurrentSchemas returns, for each form the tenant overrides, its newest
	// version, leaving out forms whose newest version is a removal.
	CurrentSchemas(ctx context.Context, tenant int64) ([]SchemaVersion, error)
	// AppendSchema stores a new version of module/name, numbering it one more
	// than the highest stored, and returns it as stored.
	AppendSchema(ctx context.Context, v SchemaVersion) (SchemaVersion, error)
	// SchemaHistory returns every version of module/name, newest first.
	SchemaHistory(ctx context.Context, tenant int64, module, name string) ([]SchemaVersion, error)

	// Fingerprint identifies everything stored: it changes whenever a text or a
	// form version is added, replaced or removed, and not otherwise. It is how an
	// instance notices a change another instance made (DEV-28).
	Fingerprint(ctx context.Context) (string, error)
}
