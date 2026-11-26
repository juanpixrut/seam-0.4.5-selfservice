package accounting

import (
	"errors"
	"testing"
)

func expenseEntry(date string) Entry {
	return Entry{Date: date, Description: "Hosting October", Movements: []Movement{
		{AccountCode: "5.3", Side: Debit, Amount: 12500},
		{AccountCode: CodeBank, Side: Credit, Amount: 12500},
	}}
}

func TestEntryKeepsItsSupportingDocument(t *testing.T) {
	e := newEnv(t)
	in := DocumentInput{Type: DocReceipt, Number: "R-77", Filename: "recibo.pdf", Data: pdfBytes,
		Currency: "eur", ForeignTotal: "115.00", FxRate: "1.087", FxDate: "2026-10-10"}
	got, err := e.svc.PostEntryWithDocument(e.ctx, expenseEntry("2026-10-10"), &in, 5)
	must(t, err)
	if got.DocID == 0 {
		t.Fatal("entry has no document")
	}
	back, err := e.svc.GetEntry(e.ctx, got.ID)
	must(t, err)
	if back.DocID != got.DocID {
		t.Fatalf("stored DocID = %d, want %d", back.DocID, got.DocID)
	}
	doc, data, err := e.svc.DocumentContent(e.ctx, back.DocID)
	must(t, err)
	if string(data) != string(pdfBytes) {
		t.Error("stored bytes differ")
	}
	if !doc.ForEntry {
		t.Error("document is not marked as an entry document")
	}
	if doc.AmountUSD != 12500 {
		t.Errorf("document amount = %d, want the entry total 12500", doc.AmountUSD)
	}
	if doc.Date != "2026-10-10" {
		t.Errorf("document date = %q, want the entry date", doc.Date)
	}
	// Foreign data is kept as passive text (D-33), upper-cased currency.
	if doc.Currency != "EUR" || doc.ForeignTotal != "115.00" || doc.FxRate != "1.087" || doc.FxDate != "2026-10-10" {
		t.Errorf("foreign metadata lost: %+v", doc)
	}
	// The books do not use it: balances are the entry's USD only.
	_, bal, err := e.svc.Balances(e.ctx, "")
	must(t, err)
	if bal["5.3"] != 12500 {
		t.Errorf("5.3 balance = %d", bal["5.3"])
	}
}

func TestEntryWithoutDocumentIsPlain(t *testing.T) {
	e := newEnv(t)
	got, err := e.svc.PostEntryWithDocument(e.ctx, expenseEntry("2026-10-10"), nil, 5)
	must(t, err)
	if got.DocID != 0 {
		t.Error("a document appeared from nowhere")
	}
}

func TestRejectedEntryLeavesNoDocument(t *testing.T) {
	e := newEnv(t)
	blobs := e.svc.blobs.(*MemBlobs)
	before := len(blobs.m)
	bad := expenseEntry("2026-10-10")
	bad.Movements[1].Amount = 1 // unbalanced
	in := DocumentInput{Type: DocReceipt, Filename: "r.pdf", Data: pdfBytes}
	if _, err := e.svc.PostEntryWithDocument(e.ctx, bad, &in, 5); !errors.Is(err, ErrUnbalanced) {
		t.Fatalf("err = %v, want ErrUnbalanced", err)
	}
	if len(blobs.m) != before {
		t.Error("a file was stored for a refused entry")
	}
	if docs, _ := e.svc.find(e.ctx, Query{Entity: EntDocument, Limit: 10}); len(docs) != 0 {
		t.Errorf("%d document record(s) left behind", len(docs))
	}
}

func TestBadDocumentRefusesTheEntry(t *testing.T) {
	e := newEnv(t)
	in := DocumentInput{Type: DocReceipt, Filename: "r.exe", Data: []byte("MZ not a pdf")}
	if _, err := e.svc.PostEntryWithDocument(e.ctx, expenseEntry("2026-10-10"), &in, 5); !errors.Is(err, ErrBadDocument) {
		t.Fatalf("err = %v, want ErrBadDocument", err)
	}
	if es, _ := e.svc.ListEntries(e.ctx, EntryFilter{}); len(es) != 0 {
		t.Error("the entry was posted although its document was refused")
	}
}

func TestClosedMonthRefusesBeforeStoringFile(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.CloseMonth(e.ctx, "2026-09", 1)
	must(t, err)
	blobs := e.svc.blobs.(*MemBlobs)
	before := len(blobs.m)
	in := DocumentInput{Type: DocReceipt, Filename: "r.pdf", Data: pdfBytes}
	if _, err := e.svc.PostEntryWithDocument(e.ctx, expenseEntry("2026-09-15"), &in, 5); !errors.Is(err, ErrPeriodClosed) {
		t.Fatalf("err = %v, want ErrPeriodClosed", err)
	}
	if len(blobs.m) != before {
		t.Error("a file was stored for an entry in a closed month")
	}
}

func TestReversalDoesNotCarryTheDocument(t *testing.T) {
	e := newEnv(t)
	in := DocumentInput{Type: DocReceipt, Filename: "r.pdf", Data: pdfBytes}
	got, err := e.svc.PostEntryWithDocument(e.ctx, expenseEntry("2026-10-10"), &in, 5)
	must(t, err)
	rev, err := e.svc.ReverseEntry(e.ctx, got.ID, "", 5)
	must(t, err)
	if rev.DocID != 0 {
		t.Error("the reversal points at the original's document")
	}
}
