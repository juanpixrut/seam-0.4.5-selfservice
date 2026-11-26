package accounting

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// WithBlobs sets where uploaded documents are kept.
func WithBlobs(b BlobStore) Option { return func(s *Service) { s.blobs = b } }

// MaxDocumentBytes caps an uploaded document.
const MaxDocumentBytes = 10 << 20

// allowedContent lists what may be uploaded. The type is sniffed from the
// bytes, never taken from what the client says.
var allowedContent = map[string]string{
	"application/pdf": ".pdf",
	"image/png":       ".png",
	"image/jpeg":      ".jpg",
}

// DocumentInput is a document being taken in.
type DocumentInput struct {
	Type      DocType
	Number    string
	Date      string // "" means today
	Filename  string
	Data      []byte
	AmountUSD Cents // the amount translated to USD, the one the books use
	ForEntry  bool  // the document backs a manual ledger entry

	// Informational only (D-33): the same document in its own currency.
	Currency     string
	ForeignTotal string
	FxRate       string
	FxDate       string
}

// SaveDocument stores the file and its record, and returns the record. The
// currency metadata is kept only when the document is in another currency
// than USD (D-33); it is text and nothing ever reads it to compute anything.
func (s *Service) SaveDocument(ctx context.Context, in DocumentInput, actor int64) (Document, error) {
	if s.blobs == nil {
		return Document{}, ErrNoBlobs
	}
	if len(in.Data) == 0 {
		return Document{}, fmt.Errorf("%w: the file is empty", ErrBadDocument)
	}
	if len(in.Data) > MaxDocumentBytes {
		return Document{}, fmt.Errorf("%w: the file is larger than %d MB", ErrBadDocument, MaxDocumentBytes>>20)
	}
	ct := http.DetectContentType(in.Data)
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	if _, ok := allowedContent[ct]; !ok {
		return Document{}, fmt.Errorf("%w: only PDF, PNG and JPEG files are accepted", ErrBadDocument)
	}
	switch in.Type {
	case DocInvoice, DocReceipt, DocPaymentProof, DocOther:
	default:
		return Document{}, fmt.Errorf("%w: unknown document type %q", ErrBadDocument, in.Type)
	}
	if in.Date != "" {
		if _, err := time.Parse(DateLayout, in.Date); err != nil {
			return Document{}, fmt.Errorf("%w: date %q", ErrBadDocument, in.Date)
		}
	}
	date := in.Date
	if date == "" {
		date = s.today()
	}
	doc := Document{
		Type: in.Type, Number: trim(in.Number), Date: date,
		Filename:    safeFilename(in.Filename, allowedContent[ct]),
		ContentType: ct, Size: int64(len(in.Data)), BlobKey: newUID(),
		AmountUSD: in.AmountUSD, ForEntry: in.ForEntry, UploadedBy: actor, UploadedAt: s.stamp(),
	}
	if cur := strings.ToUpper(trim(in.Currency)); cur != "" && cur != "USD" {
		doc.Currency, doc.ForeignTotal = cur, trim(in.ForeignTotal)
		doc.FxRate, doc.FxDate = trim(in.FxRate), trim(in.FxDate)
	}
	if err := s.blobs.Put(ctx, doc.BlobKey, in.Data); err != nil {
		return Document{}, err
	}
	id, err := s.store.Create(ctx, EntDocument, encode(doc))
	if err != nil {
		return Document{}, err
	}
	doc.ID = id
	return doc, nil
}

// discardDocument removes a document a failed operation stored. Best effort: a
// leftover file is harmless (nothing refers to it), so errors are ignored.
func (s *Service) discardDocument(ctx context.Context, doc Document) {
	if doc.ID != 0 {
		_ = s.store.Delete(ctx, EntDocument, doc.ID)
	}
	if s.blobs != nil && doc.BlobKey != "" {
		_ = s.blobs.Delete(ctx, doc.BlobKey)
	}
}

// safeFilename keeps only a plain base name with a harmless character set and
// the extension of the sniffed type.
func safeFilename(name, ext string) string {
	base := filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	base = strings.TrimSuffix(base, filepath.Ext(base))
	var b strings.Builder
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == ' ':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
		if b.Len() >= 80 {
			break
		}
	}
	out := strings.Trim(b.String(), " ._")
	if out == "" {
		out = "document"
	}
	return out + ext
}

// GetDocument returns a document record.
func (s *Service) GetDocument(ctx context.Context, id int64) (Document, error) {
	d, err := s.store.Get(ctx, EntDocument, id)
	if err != nil {
		return Document{}, err
	}
	return decode[Document](d)
}

// DocumentContent returns a document's record and bytes.
func (s *Service) DocumentContent(ctx context.Context, id int64) (Document, []byte, error) {
	if s.blobs == nil {
		return Document{}, nil, ErrNoBlobs
	}
	doc, err := s.GetDocument(ctx, id)
	if err != nil {
		return Document{}, nil, err
	}
	data, err := s.blobs.Get(ctx, doc.BlobKey)
	return doc, data, err
}

// ----------------------------------------------------------------------------
// settlements (section 6.5)
// ----------------------------------------------------------------------------

// SettlementLines returns the commission lines a settlement includes, in the
// order they were selected.
func (s *Service) SettlementLines(ctx context.Context, st Settlement) ([]CommissionLine, error) {
	out := make([]CommissionLine, 0, len(st.LineIDs))
	for _, id := range st.LineIDs {
		d, err := s.store.Get(ctx, EntCommission, id)
		if err != nil {
			return nil, err
		}
		l, err := decode[CommissionLine](d)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, nil
}

// MaxSettlementLines is how many commission lines one settlement can hold: its
// claims travel in one atomic write, which xolu limits to 25 appends.
const MaxSettlementLines = MaxBatchAppends

// UnsettledLines returns the commission lines of an AE that no settlement has
// claimed. Lines exist only for Collected items (D-21).
func (s *Service) UnsettledLines(ctx context.Context, aeID int64) ([]CommissionLine, error) {
	rows, err := s.ListCommission(ctx, CommissionFilter{AEID: aeID, Limit: 5000})
	if err != nil {
		return nil, err
	}
	var out []CommissionLine
	for _, r := range rows {
		if r.SettlementID == 0 {
			out = append(out, r.CommissionLine)
		}
	}
	return out, nil
}

// CreateSettlement is the AE's step: it picks commission lines, and uploads an
// invoice, made out to Seam's current legal name, for exactly their total.
// The settlement is born in the "invoice loaded" state.
//
// The rules that protect the money live here: the AE must be operational and
// the lines its own; a line can be claimed once (the claim's primary key is
// the line id, so even two concurrent requests cannot both win); the invoice
// must equal the total to the cent.
func (s *Service) CreateSettlement(ctx context.Context, aeID int64, lineIDs []int64, invoice DocumentInput, actor int64) (Settlement, error) {
	ids := uniqueIDs(lineIDs)
	if len(ids) == 0 {
		return Settlement{}, ErrNothingSelected
	}
	if len(ids) > MaxSettlementLines {
		return Settlement{}, ErrTooManyLines
	}
	ae, err := s.AEByID(ctx, aeID)
	if err != nil {
		return Settlement{}, err
	}
	if !ae.CanOperate() {
		return Settlement{}, ErrAENotOperational
	}
	if !ae.PayoutMethod.Valid() {
		return Settlement{}, ErrNoPayoutMethod
	}
	company, err := s.Company(ctx)
	if err != nil {
		return Settlement{}, err
	}
	if company.LegalName == "" {
		return Settlement{}, ErrNoCompany
	}

	var total Cents
	for _, id := range ids {
		ld, err := s.store.Get(ctx, EntCommission, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return Settlement{}, ErrLineUnavailable
			}
			return Settlement{}, err
		}
		line, err := decode[CommissionLine](ld)
		if err != nil {
			return Settlement{}, err
		}
		if line.AEID != ae.ID {
			return Settlement{}, ErrLineUnavailable // somebody else's line
		}
		if _, err := s.store.Get(ctx, EntClaim, id); err == nil {
			return Settlement{}, ErrLineUnavailable // already in a settlement
		} else if !errors.Is(err, ErrNotFound) {
			return Settlement{}, err
		}
		idoc, err := s.store.Get(ctx, EntItem, line.ItemID)
		if err != nil {
			return Settlement{}, ErrLineUnavailable
		}
		if st, _ := idoc.Data["state"].(string); ItemState(st) != ItemCollected {
			return Settlement{}, ErrLineUnavailable // only Collected items (D-21)
		}
		total += line.Amount
	}
	if total <= 0 {
		return Settlement{}, ErrNothingSelected
	}
	if invoice.AmountUSD != total {
		return Settlement{}, fmt.Errorf("%w: the lines add up to %s and the invoice says %s", ErrInvoiceMismatch, total.Display(), invoice.AmountUSD.Display())
	}
	invoice.Type = DocInvoice
	doc, err := s.SaveDocument(ctx, invoice, actor)
	if err != nil {
		return Settlement{}, err
	}

	st := Settlement{AEID: ae.ID, LineIDs: ids, Total: total, State: SettlementDraft,
		BilledTo: company.LegalName, Method: ae.PayoutMethod, CreatedAt: s.stamp()}
	sid, err := s.store.Create(ctx, EntSettlement, encode(st))
	if err != nil {
		s.discardDocument(ctx, doc)
		return Settlement{}, err
	}
	st.ID = sid
	// From here on a failure must not leave a draft or an unused document.
	discard := func() {
		_ = s.store.Delete(ctx, EntSettlement, sid)
		s.discardDocument(ctx, doc)
	}
	d, err := s.store.Get(ctx, EntSettlement, sid)
	if err != nil {
		discard()
		return Settlement{}, err
	}
	st.State, st.InvoiceDocID = SettlementInvoiceLoaded, doc.ID
	appends := make([]Append, 0, len(ids))
	for _, id := range ids {
		appends = append(appends, Append{Entity: EntClaim, ID: id, Data: encode(Claim{LineID: id, SettlementID: sid, AEID: ae.ID})})
	}
	if _, err := s.store.Commit(ctx, Batch{
		Update:        &Doc{Entity: EntSettlement, ID: sid, Data: encode(st)},
		ExpectVersion: d.Version,
		Appends:       appends,
	}); err != nil {
		discard()
		if errors.Is(err, ErrExists) || errors.Is(err, ErrConflict) {
			return Settlement{}, ErrLineUnavailable
		}
		return Settlement{}, err
	}
	return st, nil
}

func uniqueIDs(in []int64) []int64 {
	seen := map[int64]bool{}
	var out []int64
	for _, id := range in {
		if id > 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// GetSettlement returns a settlement. A draft is reported as not found.
func (s *Service) GetSettlement(ctx context.Context, id int64) (Settlement, error) {
	d, err := s.store.Get(ctx, EntSettlement, id)
	if err != nil {
		return Settlement{}, err
	}
	st, err := decode[Settlement](d)
	if err != nil {
		return Settlement{}, err
	}
	if st.State == SettlementDraft {
		return Settlement{}, ErrNotFound
	}
	return st, nil
}

// SettlementFilter narrows ListSettlements.
type SettlementFilter struct {
	AEID  int64
	State SettlementState
	Limit int
}

// ListSettlements returns settlements newest first. Drafts never appear.
func (s *Service) ListSettlements(ctx context.Context, f SettlementFilter) ([]Settlement, error) {
	var where []Cond
	if f.AEID != 0 {
		where = append(where, eq("ae_id", f.AEID))
	}
	if f.State != "" {
		where = append(where, eq("state", string(f.State)))
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 200
	}
	docs, err := s.find(ctx, Query{Entity: EntSettlement, Where: where, Desc: true, Limit: limit + 50})
	if err != nil {
		return nil, err
	}
	all, err := decodeAll[Settlement](docs)
	if err != nil {
		return nil, err
	}
	out := make([]Settlement, 0, len(all))
	for _, st := range all {
		if st.State != SettlementDraft && len(out) < limit {
			out = append(out, st)
		}
	}
	return out, nil
}

// PaySettlement is Seam's step: it loads the payment proof, and the
// settlement becomes Paid. In one atomic write it also posts the payment to the
// books (debit the AE's 2.1.x, credit the cash account of the payout method:
// PayPal for PayPal, the bank for a transfer).
func (s *Service) PaySettlement(ctx context.Context, id int64, proof DocumentInput, date string, actor int64) (Settlement, Entry, error) {
	if date == "" {
		date = s.today()
	}
	d, err := s.store.Get(ctx, EntSettlement, id)
	if err != nil {
		return Settlement{}, Entry{}, err
	}
	st, err := decode[Settlement](d)
	if err != nil {
		return Settlement{}, Entry{}, err
	}
	if st.State != SettlementInvoiceLoaded {
		return Settlement{}, Entry{}, ErrNotPayable
	}
	ae, err := s.AEByID(ctx, st.AEID)
	if err != nil {
		return Settlement{}, Entry{}, err
	}
	if ae.AccountCode == "" {
		return Settlement{}, Entry{}, ErrAENoAccount
	}
	chart, err := s.Chart(ctx)
	if err != nil {
		return Settlement{}, Entry{}, err
	}
	entry, err := SettlementPayment(chart, date,
		fmt.Sprintf("Settlement #%d paid to %s", st.ID, ae.Name), fmt.Sprintf("settlement:%d", st.ID),
		ae.AccountCode, st.Method.CashAccount(), st.Total)
	if err != nil {
		return Settlement{}, Entry{}, err
	}
	entry, err = s.prepare(ctx, chart, entry, actor)
	if err != nil {
		return Settlement{}, Entry{}, err
	}
	proof.Type = DocPaymentProof
	doc, err := s.SaveDocument(ctx, proof, actor)
	if err != nil {
		return Settlement{}, Entry{}, err
	}
	st.State, st.PaidAt, st.PaidBy = SettlementPaid, s.stamp(), actor
	st.PaymentDocID, st.EntryUID = doc.ID, entry.UID
	ids, err := s.store.Commit(ctx, Batch{
		Update:        &Doc{Entity: EntSettlement, ID: st.ID, Data: encode(st)},
		ExpectVersion: d.Version,
		Appends:       entryAppends(entry),
	})
	if err != nil {
		s.discardDocument(ctx, doc)
		if errors.Is(err, ErrConflict) {
			return Settlement{}, Entry{}, ErrNotPayable // someone paid it first
		}
		return Settlement{}, Entry{}, err
	}
	entry.ID = ids[0]
	return st, entry, nil
}

// ----------------------------------------------------------------------------
// monthly close (section 6.6)
// ----------------------------------------------------------------------------

// ErrCloseOrder is returned when a month is closed out of order.
var ErrCloseOrder = errors.New("accounting: months are closed in order; this month is not after the last closed one")

// CloseMonth fixes a finished month (D-35): the consolidated figures and the
// balance of every account at month end are kept, the month and every month
// before it become read-only, and its items are never pulled again.
func (s *Service) CloseMonth(ctx context.Context, period string, actor int64) (Close, error) {
	if !validPeriod(period) {
		return Close{}, ErrBadPeriod
	}
	start, _ := time.Parse("2006-01", period)
	end := start.AddDate(0, 1, -1)
	if !s.now().UTC().After(end.Add(24*time.Hour - time.Nanosecond)) {
		return Close{}, ErrPeriodNotFinished
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	closed, err := s.ClosedPeriods(ctx)
	if err != nil {
		return Close{}, err
	}
	if closed[period] {
		return Close{}, ErrAlreadyClosed
	}
	for p := range closed {
		if p > period {
			return Close{}, ErrCloseOrder
		}
	}
	_, balances, err := s.Balances(ctx, end.Format(DateLayout))
	if err != nil {
		return Close{}, err
	}
	c := Close{Period: period, ClosedAt: s.stamp(), ClosedBy: actor, Balances: balances}

	idocs, err := s.find(ctx, Query{Entity: EntItem, Where: []Cond{eq("period", period)}, Limit: 50000})
	if err != nil {
		return Close{}, err
	}
	items, err := decodeAll[Item](idocs)
	if err != nil {
		return Close{}, err
	}
	for _, it := range items {
		c.Items++
		c.BilledTotal += it.Amount
		if it.State == ItemCollected {
			c.ItemsCollected++
			c.CollectedTotal += it.Amount
		}
	}
	cdocs, err := s.find(ctx, Query{Entity: EntCommission, Where: []Cond{eq("period", period)}, Limit: 50000})
	if err != nil {
		return Close{}, err
	}
	lines, err := decodeAll[CommissionLine](cdocs)
	if err != nil {
		return Close{}, err
	}
	for _, l := range lines {
		c.CommissionTotal += l.Amount
	}
	id, err := s.store.Create(ctx, EntClose, encode(c))
	if err != nil {
		return Close{}, err
	}
	c.ID = id
	return c, nil
}

// ListCloses returns the closed months, newest first.
func (s *Service) ListCloses(ctx context.Context) ([]Close, error) {
	docs, err := s.find(ctx, Query{Entity: EntClose, OrderBy: "period", Desc: true, Limit: 1000})
	if err != nil {
		return nil, err
	}
	return decodeAll[Close](docs)
}
