package accounting

import (
	"errors"
	"fmt"
	"strings"
)

// =============================================================================
// Account Executives (DESIGN.md section 5)
// =============================================================================

// AEKind distinguishes the real salespeople from the two fixed virtual AEs.
type AEKind string

const (
	AEReal    AEKind = "real"
	AEVirtual AEKind = "virtual"
)

// AEStatus is a real AE's authorization state. It applies to real AEs only:
// virtual AEs exist from the start and never need authorizing (D-10, D-11).
type AEStatus string

const (
	AERegistered AEStatus = "registered" // signed up, may not operate yet
	AEAuthorized AEStatus = "authorized" // Seam gave the OK
)

// PayoutMethod is how Seam pays an AE (D-23). Other methods are out of scope
// for the first version (section 11).
type PayoutMethod string

const (
	PayoutTransfer PayoutMethod = "transfer"
	PayoutPayPal   PayoutMethod = "paypal"
)

// Valid reports whether m is one of the two supported methods.
func (m PayoutMethod) Valid() bool { return m == PayoutTransfer || m == PayoutPayPal }

// CashAccount returns the Seam cash account a payout by this method leaves
// from: PayPal pays from PayPal (cheaper, and avoids moving funds to the bank
// first, section 6.5), transfers pay from the bank.
func (m PayoutMethod) CashAccount() string {
	if m == PayoutPayPal {
		return CodePayPal
	}
	return CodeBank
}

// Slugs of the two virtual AEs (P2, D-11).
const (
	SlugSeamAE  = "seam_ae"  // direct sales by Seam HQ
	SlugSeamWeb = "seam_web" // self-managed sales (tenants the customer created)
)

// AE is a seller or sales channel to which tenants and commissions are
// attributed. Real AEs load their own data and are linked to a Seam user (the
// login with the account_executive role) by UserID.
type AE struct {
	ID            int64        `json:"id,omitempty"`
	Kind          AEKind       `json:"kind"`
	Slug          string       `json:"slug,omitempty"` // virtual AEs only
	Name          string       `json:"name"`
	Email         string       `json:"email,omitempty"`
	UserID        int64        `json:"user_id,omitempty"`
	Status        AEStatus     `json:"status,omitempty"`    // real AEs only
	Countries     []string     `json:"countries,omitempty"` // coverage; advisory only (D-13)
	PayoutMethod  PayoutMethod `json:"payout_method,omitempty"`
	PayoutDetails string       `json:"payout_details,omitempty"` // bank account or PayPal address, typed by the AE
	AccountCode   string       `json:"account_code,omitempty"`   // the AE's own 2.1.x account
	CreatedAt     string       `json:"created_at,omitempty"`
	AuthorizedAt  string       `json:"authorized_at,omitempty"`
	AuthorizedBy  int64        `json:"authorized_by,omitempty"`
}

// CanOperate reports whether the AE may create tenants, see its account and
// settle commissions: always for a virtual AE, only once authorized for a
// real one.
func (a AE) CanOperate() bool {
	return a.Kind == AEVirtual || a.Status == AEAuthorized
}

// CoversCountry reports whether code is among the AE's coverage countries.
// Coverage is advisory: callers must never use this to block anything (D-13).
func (a AE) CoversCountry(code string) bool {
	for _, c := range a.Countries {
		if strings.EqualFold(c, code) {
			return true
		}
	}
	return false
}

// ValidateProfile checks the data a real AE loads about itself.
func (a AE) ValidateProfile() error {
	if strings.TrimSpace(a.Name) == "" {
		return errors.New("accounting: AE name is required")
	}
	if a.PayoutMethod != "" && !a.PayoutMethod.Valid() {
		return fmt.Errorf("accounting: unsupported payout method %q", a.PayoutMethod)
	}
	return nil
}

// VirtualAEs returns the two fixed virtual AEs with their fixed accounts.
func VirtualAEs() []AE {
	return []AE{
		{Kind: AEVirtual, Slug: SlugSeamAE, Name: "Seam AE", AccountCode: CodeSeamAE},
		{Kind: AEVirtual, Slug: SlugSeamWeb, Name: "Seam Web", AccountCode: CodeSeamWeb},
	}
}

// =============================================================================
// Instances, tenants, billable items (sections 6.2, 6.3, 7)
// =============================================================================

// Instance identifies one installation of the platform that Admin pulls from.
// There is one today; every tenant and every pulled record carries its
// instance from the start so a second one needs no model change (D-04).
type Instance struct {
	ID   int64  `json:"id,omitempty"`
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// Tenant is a customer's service space on an instance. It always belongs to
// exactly one AE, fixed at creation, with no history of the link (D-12, D-15).
type Tenant struct {
	ID           int64  `json:"id,omitempty"`
	InstanceID   int64  `json:"instance_id"`
	ExternalRef  string `json:"external_ref"` // the tenant's id on the instance
	Name         string `json:"name"`
	Country      string `json:"country,omitempty"`
	AEID         int64  `json:"ae_id"`
	Account      string `json:"account,omitempty"` // the customer "Account" grouping tenants (D-14); text only
	Registration string `json:"registration,omitempty"`
}

// ItemState is the lifecycle of a billable item (section 6.3).
type ItemState string

const (
	ItemPending    ItemState = "pending"                // licenses loaded, awaiting payment
	ItemInProgress ItemState = "collection_in_progress" // payment method registered or paid via the platform
	ItemCollected  ItemState = "collected"              // Seam verified the money is in the bank or PayPal
)

// Valid reports whether s is one of the three states.
func (s ItemState) Valid() bool {
	return s == ItemPending || s == ItemInProgress || s == ItemCollected
}

// Concept says what an item bills for. These are the income kinds of the
// original rubro list (section 8.5 of the design); in the ledger they all land
// in 4.1, and the concept is kept on the item and on the commission line.
type Concept string

const (
	ConceptSubscription Concept = "subscription"
	ConceptStartup      Concept = "startup"
	ConceptAddon        Concept = "addon"
	ConceptServices     Concept = "services"
	ConceptOther        Concept = "other"
)

// Concepts lists the valid concepts in display order.
func Concepts() []Concept {
	return []Concept{ConceptSubscription, ConceptStartup, ConceptAddon, ConceptServices, ConceptOther}
}

// Valid reports whether c is a known concept.
func (c Concept) Valid() bool {
	for _, k := range Concepts() {
		if c == k {
			return true
		}
	}
	return false
}

// Item is a billable unit obtained from an instance: Admin never creates
// items, it only mirrors what the pull returns (D-05). Collected is the one
// state Admin sets itself, by hand (D-07).
type Item struct {
	ID          int64     `json:"id,omitempty"`
	InstanceID  int64     `json:"instance_id"`
	ExternalRef string    `json:"external_ref"` // the item's id on the instance; makes the pull idempotent
	TenantID    int64     `json:"tenant_id"`
	AEID        int64     `json:"ae_id"`  // copied from the tenant: fixed for life
	Period      string    `json:"period"` // "YYYY-MM"
	Concept     Concept   `json:"concept"`
	Description string    `json:"description,omitempty"`
	Amount      Cents     `json:"amount"` // what the customer pays, in USD
	State       ItemState `json:"state"`

	// Set when the item is marked Collected.
	CollectedAt      string `json:"collected_at,omitempty"`
	CollectedBy      int64  `json:"collected_by,omitempty"`
	CollectedAccount string `json:"collected_account,omitempty"` // 1.1.x where the money arrived
	EntryUID         string `json:"entry_uid,omitempty"`         // the collection entry
	InstanceNotified bool   `json:"instance_notified,omitempty"` // the instance API was told
}

// =============================================================================
// Commission, settlement, documents (sections 6.4, 6.5, 7.2)
// =============================================================================

// CommissionLine is the calculated commission for one item, in USD. The
// formula lives outside the schema (P6): Admin stores only its result.
type CommissionLine struct {
	ID       int64   `json:"id,omitempty"`
	AEID     int64   `json:"ae_id"`
	ItemID   int64   `json:"item_id"`
	TenantID int64   `json:"tenant_id"`
	Period   string  `json:"period"`
	Concept  Concept `json:"concept"`
	Detail   string  `json:"detail,omitempty"`
	Amount   Cents   `json:"amount"`
	EntryUID string  `json:"entry_uid,omitempty"` // the accrual entry
}

// Claim records that a commission line belongs to a settlement. Its primary
// key is the commission line's own id, so the store itself refuses a second
// claim on the same line: "a line is paid at most once" is a storage
// guarantee, not a check that two concurrent requests could both pass.
type Claim struct {
	LineID       int64 `json:"line_id"`
	SettlementID int64 `json:"settlement_id"`
	AEID         int64 `json:"ae_id"`
}

// SettlementState is the lifecycle of an AE settlement (section 6.5).
type SettlementState string

const (
	SettlementDraft         SettlementState = "draft"          // being created; never shown, never payable
	SettlementInvoiceLoaded SettlementState = "invoice_loaded" // the AE uploaded its invoice
	SettlementPaid          SettlementState = "paid"           // Seam paid and loaded the proof
)

// Settlement groups commission lines for payment to an AE. Only lines of
// Collected items can be included (D-21); the AE uploads an invoice made out
// to Seam's current legal name for exactly Total (D-22).
type Settlement struct {
	ID           int64           `json:"id,omitempty"`
	AEID         int64           `json:"ae_id"`
	LineIDs      []int64         `json:"line_ids"`
	Total        Cents           `json:"total"`
	State        SettlementState `json:"state"`
	BilledTo     string          `json:"billed_to"` // Seam's legal name when the invoice was due
	Method       PayoutMethod    `json:"method"`
	InvoiceDocID int64           `json:"invoice_doc_id,omitempty"`
	PaymentDocID int64           `json:"payment_doc_id,omitempty"`
	CreatedAt    string          `json:"created_at,omitempty"`
	PaidAt       string          `json:"paid_at,omitempty"`
	PaidBy       int64           `json:"paid_by,omitempty"`
	EntryUID     string          `json:"entry_uid,omitempty"` // the payment entry
}

// DocType says what a Document is.
type DocType string

const (
	DocInvoice      DocType = "invoice"
	DocReceipt      DocType = "receipt"
	DocPaymentProof DocType = "payment_proof"
	DocOther        DocType = "other"
)

// Document records any document taken into the system. A document in a
// currency other than USD may carry four informational facts (D-33). They are
// passive metadata: nothing ever reads them to compute anything (P4).
type Document struct {
	ID          int64   `json:"id,omitempty"`
	Type        DocType `json:"type"`
	Number      string  `json:"number,omitempty"` // the document's own number, as typed
	Date        string  `json:"date,omitempty"`
	Filename    string  `json:"filename,omitempty"`
	ContentType string  `json:"content_type,omitempty"`
	Size        int64   `json:"size,omitempty"`
	BlobKey     string  `json:"blob_key,omitempty"`
	AmountUSD   Cents   `json:"amount_usd,omitempty"` // the amount translated to USD, the one the books use
	ForEntry    bool    `json:"for_entry,omitempty"`  // backs a manual ledger entry: only ledger readers may open it
	UploadedBy  int64   `json:"uploaded_by,omitempty"`
	UploadedAt  string  `json:"uploaded_at,omitempty"`

	// Metadata for documents issued in another currency; all four empty for USD.
	Currency     string `json:"currency,omitempty"`
	ForeignTotal string `json:"foreign_total,omitempty"` // text, never parsed
	FxRate       string `json:"fx_rate,omitempty"`       // text, never parsed
	FxDate       string `json:"fx_date,omitempty"`
}

// =============================================================================
// Monthly close, company settings
// =============================================================================

// Close fixes a month. It keeps the consolidated figures of the month so they
// are never read from the instance again (D-35), and the balance of every
// account at the end of the month.
type Close struct {
	ID              int64            `json:"id,omitempty"`
	Period          string           `json:"period"` // "YYYY-MM"
	ClosedAt        string           `json:"closed_at"`
	ClosedBy        int64            `json:"closed_by"`
	Balances        map[string]Cents `json:"balances"` // account code -> natural-sign balance at month end
	Items           int              `json:"items"`
	ItemsCollected  int              `json:"items_collected"`
	BilledTotal     Cents            `json:"billed_total"`
	CollectedTotal  Cents            `json:"collected_total"`
	CommissionTotal Cents            `json:"commission_total"`
}

// Company is the data of Seam's own company (section 4.2). The legal name is a
// plain text, "Aberton SA" for instance, not an entity (D-24): AE invoices are
// made out to whatever it says when the settlement is created.
type Company struct {
	ID           int64  `json:"id,omitempty"`
	LegalName    string `json:"legal_name"`
	TaxID        string `json:"tax_id,omitempty"`
	Address      string `json:"address,omitempty"`
	Country      string `json:"country,omitempty"`
	BillingEmail string `json:"billing_email,omitempty"`
}
