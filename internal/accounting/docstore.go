package accounting

import (
	"context"
	"errors"
)

// The accounting module keeps everything in xolu entities. DocStore is the
// small slice of xolu the module needs, so the business rules in Service can
// be exercised against an in-memory implementation (MemStore) and run against
// the real one (XoluStore) in production and in the integration test.

// Entity names. They are prefixed so that they can never collide with the
// entities of the Assets module that shares the same xolu instance.
const (
	EntAE         = "adm_ae"
	EntAccount    = "adm_account"
	EntEntry      = "adm_entry"
	EntMovement   = "adm_movement"
	EntInstance   = "adm_instance"
	EntTenant     = "adm_tenant"
	EntItem       = "adm_item"
	EntCommission = "adm_commission"
	EntClaim      = "adm_line_claim"
	EntSettlement = "adm_settlement"
	EntDocument   = "adm_document"
	EntClose      = "adm_close"
	EntCompany    = "adm_company"
	EntFormula    = "adm_formula" // formula rules (discounts, surcharges, commission and price rules)
	EntHead       = "adm_head"    // one-row anchor for entries that change no other record
)

// Errors a DocStore returns.
var (
	ErrNotFound = errors.New("accounting: record not found")
	ErrConflict = errors.New("accounting: the record was changed by someone else")
	ErrExists   = errors.New("accounting: a record with that id already exists")
)

// Doc is one stored record: the entity it belongs to, its id, its version
// (bumped on every write, used for optimistic concurrency) and its fields.
type Doc struct {
	Entity  string
	ID      int64
	Version int
	Data    map[string]any
}

// Cond is one equality or range condition of a Query.
type Cond struct {
	Field string
	Op    string // "=", "!=", "<", "<=", ">", ">="
	Value any    // string, int, int64, or bool
}

// Query selects records of one entity.
type Query struct {
	Entity  string
	Where   []Cond
	OrderBy string // field name; "" means id
	Desc    bool
	Limit   int // 0 means the store's default cap
}

// DefaultLimit caps a Query that sets no Limit.
const DefaultLimit = 1000

// Append is a record inserted as part of a Batch. ID 0 asks the store to
// assign one; a non-zero ID that already exists fails the whole Batch with
// ErrExists. Line claims use their commission line's id as primary key, which
// is what makes "this line is in at most one settlement" a storage guarantee.
type Append struct {
	Entity string
	ID     int64
	Data   map[string]any
}

// Batch is one atomic write: an optional guarded update of one record plus any
// number of inserts (xolu allows 25). Either everything lands or nothing does.
type Batch struct {
	// Update replaces the data of one existing record. If ExpectVersion is
	// non-zero the update only applies when the stored version matches,
	// otherwise ErrConflict is returned and nothing is written.
	Update        *Doc
	ExpectVersion int
	Appends       []Append
}

// MaxBatchAppends is xolu's limit on the append list of one commit.
const MaxBatchAppends = 25

// DocStore is what the module needs from its storage.
type DocStore interface {
	// Create inserts a record and returns its id.
	Create(ctx context.Context, entity string, data map[string]any) (int64, error)
	// Get returns one record, or ErrNotFound.
	Get(ctx context.Context, entity string, id int64) (Doc, error)
	// Put replaces a record. expectVersion != 0 makes it conditional.
	Put(ctx context.Context, entity string, id int64, data map[string]any, expectVersion int) error
	// Delete removes a record. It exists only to discard what a failed
	// operation left behind (a draft settlement, an unused document); the
	// books are never deleted from: entries are append-only.
	Delete(ctx context.Context, entity string, id int64) error
	// Find returns the records matching q. An entity nothing has ever been
	// written to is an empty result, not an error.
	Find(ctx context.Context, q Query) ([]Doc, error)
	// Sum groups the records of entity by groupBy and sums the given numeric
	// fields. Each result row has the group value under groupBy and the totals
	// under the field names.
	Sum(ctx context.Context, entity, groupBy string, fields []string, where []Cond) ([]map[string]any, error)
	// Commit applies a Batch atomically and returns the ids assigned to the
	// appended records, in order.
	Commit(ctx context.Context, b Batch) ([]int64, error)
}
