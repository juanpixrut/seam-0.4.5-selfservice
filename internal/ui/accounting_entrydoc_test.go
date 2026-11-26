package ui

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ha1tch/seam-ui/internal/accounting"
	"github.com/ha1tch/seam-ui/internal/auth"
)

var entryPDF = []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\ntrailer\n<<>>\n%%EOF\n")

// postEntry sends the manual entry form (multipart, as the page does) as an
// accountant. withFile adds the optional supporting document.
func (e *acctEnv) postEntry(t *testing.T, withFile bool, extra map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	f := map[string]string{
		"date": "2026-10-10", "description": "Hosting October",
		"account_0": accounting.CodeOperatingEx, "debit_0": "125.00",
		"account_1": accounting.CodeBank, "credit_1": "125.00",
	}
	for k, v := range extra {
		f[k] = v
	}
	for k, v := range f {
		mw.WriteField(k, v)
	}
	if withFile {
		fw, _ := mw.CreateFormFile("doc_file", "recibo.pdf")
		fw.Write(entryPDF)
	}
	mw.Close()
	req := httptest.NewRequest("POST", "/accounting/entries", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req = req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{UserID: 5, TenantID: 1, Role: auth.RoleAccountant}))
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req)
	return rec
}

func (e *acctEnv) lastEntry(t *testing.T) accounting.Entry {
	t.Helper()
	es, err := e.svc.ListEntries(context.Background(), accounting.EntryFilter{Limit: 1})
	if err != nil || len(es) == 0 {
		t.Fatalf("no entry: %v", err)
	}
	return es[0]
}

func TestEntryFormTakesAnOptionalDocument(t *testing.T) {
	e := newAcctEnv(t)
	// The form is multipart and offers the optional document block.
	body := e.get(auth.RoleAccountant, 5, "/accounting/entries/new").Body.String()
	for _, want := range []string{`enctype="multipart/form-data"`, `name="doc_file"`, `name="doc_type"`, `name="doc_currency"`, "accounting.entry.doc_title"} {
		if !strings.Contains(body, want) {
			t.Errorf("entry form lacks %s", want)
		}
	}
	if strings.Contains(body, `name="doc_amount"`) {
		t.Error("the entry form asks for a document amount; the entry total is used")
	}
	if strings.Contains(body, `name="doc_file" required`) || strings.Contains(body, `required="required" name="doc_file"`) {
		t.Error("the document must be optional")
	}
}

func TestEntryPostedWithDocumentShowsAndServesIt(t *testing.T) {
	e := newAcctEnv(t)
	rec := e.postEntry(t, true, map[string]string{"doc_type": "receipt", "doc_currency": "eur", "doc_foreign_total": "115.00"})
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "ok=") {
		t.Fatalf("post = %d %s", rec.Code, rec.Header().Get("Location"))
	}
	en := e.lastEntry(t)
	if en.DocID == 0 {
		t.Fatal("document not attached")
	}
	doc, err := e.svc.GetDocument(context.Background(), en.DocID)
	if err != nil || !doc.ForEntry || doc.Currency != "EUR" || doc.AmountUSD != 12500 {
		t.Fatalf("document = %+v, %v", doc, err)
	}
	// The ledger links to it; the accountant can download it.
	if body := e.get(auth.RoleAccountant, 5, "/accounting/ledger").Body.String(); !strings.Contains(body, "/accounting/documents/"+itoa(en.DocID)) {
		t.Error("ledger has no link to the document")
	}
	if rec := e.get(auth.RoleAccountant, 5, "/accounting/documents/"+itoa(en.DocID)); rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), entryPDF) {
		t.Errorf("accountant download = %d", rec.Code)
	}
}

func TestEntryDocumentIsNotForAEs(t *testing.T) {
	e := newAcctEnv(t)
	e.postEntry(t, true, map[string]string{"doc_type": "receipt"})
	id := itoa(e.lastEntry(t).DocID)
	for _, uid := range []int64{acctUserA, acctUserB} {
		if rec := e.get(auth.RoleAccountExecutive, uid, "/accounting/documents/"+id); rec.Code != 404 {
			t.Errorf("AE %d opened an entry document: %d", uid, rec.Code)
		}
	}
	// A manager has no ledger access either.
	if rec := e.get(auth.RoleManager, 9, "/accounting/documents/"+id); rec.Code == 200 {
		t.Error("a user without ledger access opened an entry document")
	}
	// Settlement documents keep working for their own AE.
	if rec := e.get(auth.RoleAccountExecutive, acctUserA, "/accounting/documents/"+itoa(e.invoiceDocID)); rec.Code != 200 {
		t.Errorf("owner lost access to its own invoice: %d", rec.Code)
	}
}

func TestEntryWithoutFileStaysPlainAndBadFileIsRefused(t *testing.T) {
	e := newAcctEnv(t)
	if rec := e.postEntry(t, false, nil); rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "ok=") {
		t.Fatalf("plain post = %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if e.lastEntry(t).DocID != 0 {
		t.Error("a document appeared")
	}
	n := len(mustEntries(t, e))
	// A file that is not a PDF/PNG/JPEG refuses the whole entry.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range map[string]string{"date": "2026-10-11", "description": "x", "account_0": "5.3", "debit_0": "1.00", "account_1": "1.1.1", "credit_1": "1.00", "doc_type": "receipt"} {
		mw.WriteField(k, v)
	}
	fw, _ := mw.CreateFormFile("doc_file", "evil.exe")
	fw.Write([]byte("MZ"))
	mw.Close()
	req := httptest.NewRequest("POST", "/accounting/entries", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req = req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{UserID: 5, TenantID: 1, Role: auth.RoleAccountant}))
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req)
	if !strings.Contains(rec.Header().Get("Location"), "err=") {
		t.Errorf("bad file not refused: %s", rec.Header().Get("Location"))
	}
	if len(mustEntries(t, e)) != n {
		t.Error("the entry was posted with a refused document")
	}
	// An unknown document type is refused, not stored.
	if rec := e.postEntry(t, true, map[string]string{"doc_type": "payment_proof"}); !strings.Contains(rec.Header().Get("Location"), "err=") {
		t.Errorf("unexpected type accepted: %s", rec.Header().Get("Location"))
	}
}

func mustEntries(t *testing.T, e *acctEnv) []accounting.Entry {
	t.Helper()
	es, err := e.svc.ListEntries(context.Background(), accounting.EntryFilter{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	return es
}
