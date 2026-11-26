package accounting

import "context"

// InstanceClient is the boundary between Seam Admin and a platform instance
// (DESIGN.md 4.2, 4.3). The platform stays the source of truth for licenses,
// plans, renewals, the payment gateway and the payments themselves (P1):
// Admin pulls what it needs, and calls back only to confirm that Seam saw the
// money arrive.
//
// The platform side of this API is not specified in the design: what an
// instance must expose is the contents of InstanceSnapshot and one call to
// mark an item collected. Until an implementation is wired in, Admin runs
// with no instance (see NoInstance) and the pull reports ErrNoInstance.
type InstanceClient interface {
	// Pull returns the tenants and billable items of the instance.
	Pull(ctx context.Context) (InstanceSnapshot, error)
	// MarkCollected tells the instance that Seam verified the money for the
	// item identified by its instance-side reference. It must be idempotent.
	MarkCollected(ctx context.Context, itemExternalRef string) error
}

// InstanceSnapshot is what one pull returns.
type InstanceSnapshot struct {
	Tenants []TenantSnapshot
	Items   []ItemSnapshot
}

// TenantSnapshot is a tenant as the instance reports it. AERef names the AE
// the tenant belongs to: the id of a real AE in Admin ("17"), or the slug of
// a virtual one ("seam_ae", "seam_web"). Every tenant has one (P2).
type TenantSnapshot struct {
	ExternalRef  string
	Name         string
	Country      string
	AERef        string
	Account      string // customer account grouping the tenants of one customer
	Registration string
}

// ItemSnapshot is a billable item as the instance reports it. State is the
// instance's view: pending or collection in progress. Collected is decided in
// Admin, never imported.
type ItemSnapshot struct {
	ExternalRef string
	TenantRef   string
	Period      string // "YYYY-MM"
	Concept     Concept
	Description string
	Amount      Cents
	State       ItemState
}

// NoInstance is the InstanceClient used when no platform is connected.
type NoInstance struct{}

// Pull always fails with ErrNoInstance.
func (NoInstance) Pull(context.Context) (InstanceSnapshot, error) {
	return InstanceSnapshot{}, ErrNoInstance
}

// MarkCollected always fails with ErrNoInstance.
func (NoInstance) MarkCollected(context.Context, string) error { return ErrNoInstance }

// CommissionResult is one commission line produced by the formula.
type CommissionResult struct {
	Detail string
	Amount Cents
}

// CommissionCalculator turns a collected item into commission lines. The
// formula is outside the scope of the schema (P6, section 11): the schema
// stores only its results, in USD, and the same call serves real and virtual
// AEs alike. The formula decides what each AE receives, including the two
// virtual ones.
type CommissionCalculator interface {
	Calculate(ctx context.Context, item Item, tenant Tenant, ae AE) ([]CommissionResult, error)
}

// NoFormula is the default calculator: it returns no commission. With it an
// item can be collected and booked as income, but no AE accrues anything.
type NoFormula struct{}

// Calculate returns no lines.
func (NoFormula) Calculate(context.Context, Item, Tenant, AE) ([]CommissionResult, error) {
	return nil, nil
}

// CalculatorFunc adapts a function to CommissionCalculator.
type CalculatorFunc func(ctx context.Context, item Item, tenant Tenant, ae AE) ([]CommissionResult, error)

// Calculate calls f.
func (f CalculatorFunc) Calculate(ctx context.Context, item Item, tenant Tenant, ae AE) ([]CommissionResult, error) {
	return f(ctx, item, tenant, ae)
}
