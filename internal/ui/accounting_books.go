package ui

// Accounting & Admin: the books (ledger, chart, manual entries), monthly close
// and Seam's own company data.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	mi "github.com/ha1tch/minty"

	"github.com/ha1tch/seam-ui/internal/accounting"
	"github.com/ha1tch/seam-ui/internal/auth"
)

const entryRows = 6

func (h *UIHandler) AcctLedger(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	t := q.t
	chart, err := q.svc.Chart(q.ctx())
	if err != nil {
		q.fail(err)
		return
	}
	from, to := q.queryText("from"), q.queryText("to")
	page, size := q.pageParams()
	all, err := q.svc.ListEntries(q.ctx(), accounting.EntryFilter{From: from, To: to, Limit: windowFor(page, size)})
	if err != nil {
		q.fail(err)
		return
	}
	entries, hasNext := paged(all, page, size)
	canRev := q.can(auth.PermAccountingEntryReverse)
	csrf := q.csrf()
	var rows [][]mi.H
	for _, e := range entries {
		d, _ := e.Totals()
		mv := e.Movements
		moves := cellH(func(b *mi.Builder) mi.Node {
			args := []interface{}{mi.Class("space-y-1 min-w-0")}
			for _, m := range mv {
				name := m.AccountCode
				if a, ok := chart.Get(m.AccountCode); ok {
					name += " " + a.Name
				}
				args = append(args, b.Div(mi.Class("flex items-center gap-2 text-xs text-gray-600 dark:text-gray-300"),
					b.Span(mi.Class("w-10 shrink-0 font-semibold uppercase text-gray-500 dark:text-gray-400"), t("accounting.side."+string(m.Side))),
					b.Span(mi.Class("flex-1 min-w-0 truncate"), mi.Attr("title", name), name),
					b.Span(mi.Class("tabular-nums whitespace-nowrap"), m.Amount.Display())))
			}
			return b.Div(args...)
		})
		var act mi.H = cellT("")
		if canRev && e.Source != accounting.SourceReversal {
			act = cellRight(acctConfirmForm(acctConfirm{
				Message: t("accounting.entry.reverse_confirm", itoa(e.ID)), Title: t("accounting.entry.reverse"),
				OK: t("accounting.entry.reverse"), Danger: true,
			}, "/accounting/entries/"+itoa(e.ID)+"/reverse", csrf, false, "", acctBtnDangerSubmit(t("accounting.entry.reverse"))))
		}
		rows = append(rows, []mi.H{
			cellN("#" + itoa(e.ID)), cellN(e.Date), entryDescription(t, e),
			cellH(sourceBadge(t, string(e.Source))), moves, moneyCell(d), act,
		})
	}
	var acts []mi.H
	acts = append(acts, acctLink("/accounting/accounts", t("accounting.accounts.title")))
	if q.can(auth.PermAccountingEntryCreate) {
		acts = append(acts, acctLink("/accounting/entries/new", t("accounting.entry.new")))
	}
	q.render(t("accounting.ledger.title"), "/accounting/ledger", acctPage(
		acctTitle(t, "accounting.ledger.title", acts...), q.banner(), q.dateFilter(from, to), q.pager(page, hasNext),
		acctPanel("", acctTable(
			[]string{"#", t("accounting.col.date"), t("accounting.col.description"), t("accounting.col.source"),
				t("accounting.col.movements"), rh(t("accounting.col.amount")), ""}, rows, t("accounting.empty"))),
		q.pager(page, hasNext)))
}

// entryDescription is the description cell of an entry, with a link to its
// supporting document when it has one.
func entryDescription(t func(string, ...any) string, e accounting.Entry) mi.H {
	if e.DocID == 0 {
		return cellT(e.Description)
	}
	return cellH(func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("space-y-1"),
			b.Div(e.Description),
			b.A(mi.Href("/accounting/documents/"+itoa(e.DocID)), mi.Class("inline-flex items-center gap-1 text-xs text-indigo-600 hover:underline"),
				b.Span(mi.Class("material-icons text-sm"), mi.Attr("aria-hidden", "true"), "attach_file"), t("accounting.entry.document")))
	})
}

// dateFilter is the from/to filter of the ledger and the account books.
func (q *acctReq) dateFilter(from, to string) mi.H {
	return q.acctFilter(
		acctField(q.t("accounting.filter.from"), "from", "date", from),
		acctField(q.t("accounting.filter.to"), "to", "date", to))
}

func (h *UIHandler) AcctAccounts(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	t := q.t
	chart, bal, err := q.svc.Balances(q.ctx(), "")
	if err != nil {
		q.fail(err)
		return
	}
	var rows [][]mi.H
	for _, a := range chart.Accounts() {
		depth := strings.Count(a.Code, ".")
		code, name, post := a.Code, a.Name, a.Postable
		pad := fmt.Sprintf("padding-left:%drem", depth)
		nameCell := cellH(func(b *mi.Builder) mi.Node {
			if post {
				return b.A(mi.Href("/accounting/accounts/"+code), mi.Class("text-indigo-600 hover:underline"), mi.Attr("style", pad), name)
			}
			return b.Span(mi.Class("font-semibold"), mi.Attr("style", pad), name)
		})
		rows = append(rows, []mi.H{cellN(code), nameCell, moneyCell(bal[code])})
	}
	q.render(t("accounting.accounts.title"), "/accounting/accounts", acctPage(
		acctTitle(t, "accounting.accounts.title"), q.banner(),
		acctPanel("", acctTable(
			[]string{t("accounting.col.code"), t("accounting.col.account"), rh(t("accounting.col.balance"))}, rows, t("accounting.empty")))))
}

func (h *UIHandler) AcctAccountBook(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	t := q.t
	code := r.PathValue("code")
	from, to := q.queryText("from"), q.queryText("to")
	l, err := q.svc.Ledger(q.ctx(), code, from, to)
	if err != nil {
		if acctErrKey(err) == "accounting.err.generic" {
			q.fail(err)
		} else {
			q.notFound()
		}
		return
	}
	page, size := q.pageParams()
	pageLines, hasNext := paged(l.Lines, page, size)
	var rows [][]mi.H
	for _, ln := range pageLines {
		var dr, cr accounting.Cents
		if ln.Side == accounting.Debit {
			dr = ln.Amount
		} else {
			cr = ln.Amount
		}
		rows = append(rows, []mi.H{
			cellN("#" + itoa(ln.EntryID)), cellN(ln.Date), cellT(ln.Description), moneyCell(dr), moneyCell(cr), moneyCell(ln.Balance)})
	}
	q.render(l.Account.Code+" "+l.Account.Name, "/accounting/accounts", acctPage(
		acctTitle(t, "accounting.book.title", acctLink("/accounting/accounts", t("accounting.accounts.title"))),
		q.dateFilter(from, to), q.pager(page, hasNext),
		acctPanel(l.Account.Code+" "+l.Account.Name, func(b *mi.Builder) mi.Node {
			return b.Div(acctBody(acctDefList([][2]string{{t("accounting.book.opening"), money(l.Opening)}, {t("accounting.book.closing"), money(l.Closing)}}))(b),
				acctTable([]string{"#", t("accounting.col.date"), t("accounting.col.description"), rh(t("accounting.side.debit")),
					rh(t("accounting.side.credit")), rh(t("accounting.col.balance"))}, rows, t("accounting.empty"))(b))
		})))
}

func (h *UIHandler) AcctEntryForm(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	t := q.t
	chart, err := q.svc.Chart(q.ctx())
	if err != nil {
		q.fail(err)
		return
	}
	opts := []acctOpt{{"", "—"}}
	for _, a := range chart.Accounts() {
		if a.Postable {
			opts = append(opts, acctOpt{a.Code, a.Code + " " + a.Name})
		}
	}
	head := func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("grid grid-cols-1 md:grid-cols-3 gap-4"),
			acctField(t("accounting.col.date"), "date", "date", today(), mi.Required())(b),
			b.Div(mi.Class("md:col-span-2"),
				acctField(t("accounting.col.description"), "description", "text", "", mi.Required(), mi.Attr("maxlength", "300"))(b)))
	}
	grid := func(b *mi.Builder) mi.Node {
		var trs []interface{}
		for i := 0; i < entryRows; i++ {
			n := fmt.Sprint(i)
			trs = append(trs, b.Tr(mi.Class("border-t border-gray-100 dark:border-gray-700"),
				b.Td(mi.Class("px-2 py-2"), acctField(t("accounting.col.account")+" "+fmt.Sprint(i+1), "account_"+n, "select", "", opts, acctSrOnlyLabel{})(b)),
				b.Td(mi.Class("px-2 py-2 w-40"), acctField(t("accounting.side.debit")+" "+fmt.Sprint(i+1), "debit_"+n, "text", "",
					acctSrOnlyLabel{}, mi.Attr("inputmode", "decimal"), mi.Placeholder("0.00"), acctClass("text-right"))(b)),
				b.Td(mi.Class("px-2 py-2 w-40"), acctField(t("accounting.side.credit")+" "+fmt.Sprint(i+1), "credit_"+n, "text", "",
					acctSrOnlyLabel{}, mi.Attr("inputmode", "decimal"), mi.Placeholder("0.00"), acctClass("text-right"))(b))))
		}
		th := func(txt, cls string) interface{} {
			return b.Th(mi.Scope("col"), mi.Class("px-2 py-2 text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400 "+cls), txt)
		}
		foot := b.Tr(mi.Class("border-t border-gray-200 dark:border-gray-600 font-semibold text-sm text-gray-900 dark:text-white"),
			b.Td(mi.Class("px-2 py-2"), b.Span(mi.ID("entry-state"), mi.Class("text-gray-500 dark:text-gray-400"), t("accounting.entry.totals"))),
			b.Td(mi.Class("px-2 py-2 text-right tabular-nums"), b.Span(mi.ID("entry-debit"), "0.00")),
			b.Td(mi.Class("px-2 py-2 text-right tabular-nums"), b.Span(mi.ID("entry-credit"), "0.00")))
		return b.Div(mi.Class("overflow-x-auto"), b.Table(mi.Class("w-full"),
			b.Thead(b.Tr(th(t("accounting.col.account"), "text-left"), th(t("accounting.side.debit"), "text-right"), th(t("accounting.side.credit"), "text-right"))),
			b.Tbody(append(trs, foot)...)))
	}
	hint := func(b *mi.Builder) mi.Node {
		return b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400"), t("accounting.entry.hint"))
	}
	docBlock := func(b *mi.Builder) mi.Node {
		kids := []interface{}{mi.Class("rounded-lg border border-gray-200 dark:border-gray-700 p-4 space-y-4"),
			b.H3(mi.Class("text-sm font-semibold text-gray-900 dark:text-white"), t("accounting.entry.doc_title")),
			b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400"), t("accounting.entry.doc_hint")),
			acctField(t("accounting.doc.type"), "doc_type", "select", string(accounting.DocReceipt), []acctOpt{
				{string(accounting.DocReceipt), t("accounting.doctype.receipt")},
				{string(accounting.DocInvoice), t("accounting.doctype.invoice")},
				{string(accounting.DocOther), t("accounting.doctype.other")},
			})(b)}
		for _, f := range q.docFields("doc_", false, false, "", "") {
			kids = append(kids, f(b))
		}
		return b.Div(kids...)
	}
	form := acctConfirmForm(acctConfirm{Message: t("accounting.entry.confirm"), Title: t("accounting.entry.post"), OK: t("accounting.entry.post"), ID: "entry-form"},
		"/accounting/entries", q.csrf(), true, "space-y-4", head, hint, grid, docBlock,
		func(b *mi.Builder) mi.Node {
			return b.Div(mi.Class("flex items-center justify-between gap-4"),
				b.Div(mi.ID("entry-diff"), mi.Class("text-sm"), mi.Attr("aria-live", "polite")),
				acctSubmit(t("accounting.entry.post"))(b))
		},
		func(b *mi.Builder) mi.Node {
			return b.Script(mi.Raw(entryScript(t("accounting.entry.balanced"), t("accounting.entry.difference"))))
		})
	q.render(t("accounting.entry.new"), "/accounting/ledger", acctPage(
		acctTitle(t, "accounting.entry.new", acctLink("/accounting/ledger", t("accounting.back"))), q.banner(),
		acctPanel("", acctBody(form))))
}

// entryScript keeps running debit and credit totals on the manual entry form
// and says whether it balances. The server checks again; this only helps.
func entryScript(balanced, difference string) string {
	q := func(s string) string { j, _ := json.Marshal(s); return string(j) }
	return `(function(){
var f=document.getElementById('entry-form');if(!f)return;
var B=` + q(balanced) + `,D=` + q(difference) + `;
function cents(v){v=(v||'').replace(/[\s,]/g,'');if(!/^\d+(\.\d{1,2})?$/.test(v))return 0;var p=v.split('.');return parseInt(p[0],10)*100+parseInt((p[1]||'0').padEnd(2,'0'),10);}
function fmt(c){var s=String(Math.abs(c));while(s.length<3)s='0'+s;return s.slice(0,-2).replace(/\B(?=(\d{3})+(?!\d))/g,',')+'.'+s.slice(-2);}
function update(){var d=0,c=0;for(var i=0;i<` + fmt.Sprint(entryRows) + `;i++){d+=cents(f.elements['debit_'+i].value);c+=cents(f.elements['credit_'+i].value);}
document.getElementById('entry-debit').textContent=fmt(d);document.getElementById('entry-credit').textContent=fmt(c);
var out=document.getElementById('entry-diff');
if(d===0&&c===0){out.textContent='';}
else if(d===c){out.textContent=B;out.className='text-sm text-green-700 dark:text-green-400';}
else{out.textContent=D+' '+fmt(Math.abs(d-c));out.className='text-sm text-red-600 dark:text-red-400';}}
f.addEventListener('input',update);update();})();`
}

func (h *UIHandler) AcctEntryCreate(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	back := "/accounting/entries/new"
	if err := r.ParseMultipartForm(accounting.MaxDocumentBytes); err != nil && !errors.Is(err, http.ErrNotMultipart) {
		q.redirectErr(back, fmt.Errorf("%w: %v", accounting.ErrBadDocument, err))
		return
	}
	e := accounting.Entry{
		Date:        strings.TrimSpace(r.FormValue("date")),
		Description: strings.TrimSpace(r.FormValue("description")),
		Source:      accounting.SourceManual,
	}
	for i := 0; i < entryRows; i++ {
		n := fmt.Sprint(i)
		code := strings.TrimSpace(r.FormValue("account_" + n))
		d, c := strings.TrimSpace(r.FormValue("debit_"+n)), strings.TrimSpace(r.FormValue("credit_"+n))
		if code == "" && d == "" && c == "" {
			continue
		}
		if (d != "") == (c != "") { // exactly one side per row
			q.redirectErr(back, accounting.ErrBadMovement)
			return
		}
		side, txt := accounting.Debit, d
		if c != "" {
			side, txt = accounting.Credit, c
		}
		amt, err := accounting.ParseUSD(txt)
		if err != nil {
			q.redirectErr(back, err)
			return
		}
		e.Movements = append(e.Movements, accounting.Movement{AccountCode: code, Side: side, Amount: amt})
	}
	var doc *accounting.DocumentInput
	if _, hdr, err := r.FormFile("doc_file"); err == nil && hdr != nil {
		typ := accounting.DocType(strings.TrimSpace(r.FormValue("doc_type")))
		switch typ {
		case accounting.DocReceipt, accounting.DocInvoice, accounting.DocOther:
		default:
			q.redirectErr(back, accounting.ErrBadDocument)
			return
		}
		in, err := q.readDocumentAs(typ, "doc_")
		if err != nil {
			q.redirectErr(back, err)
			return
		}
		doc = &in
	}
	saved, err := q.svc.PostEntryWithDocument(q.ctx(), e, doc, q.user.ID)
	if err != nil {
		q.redirectErr(back, err)
		return
	}
	q.acctAudit("entry.post", itoa(saved.ID))
	q.redirectMsg("/accounting/ledger", "ok", "entry_posted")
}

func (h *UIHandler) AcctEntryReverse(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	id, ok := pathID(r)
	if !ok {
		q.notFound()
		return
	}
	if _, err := q.svc.ReverseEntry(q.ctx(), id, "", q.user.ID); err != nil {
		q.redirectErr("/accounting/ledger", err)
		return
	}
	q.acctAudit("entry.reverse", itoa(id))
	q.redirectMsg("/accounting/ledger", "ok", "entry_reversed")
}

func (h *UIHandler) AcctCloses(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	t := q.t
	closes, err := q.svc.ListCloses(q.ctx())
	if err != nil {
		q.fail(err)
		return
	}
	var rows [][]mi.H
	for _, c := range closes {
		rows = append(rows, []mi.H{cellN(c.Period), cellN(dateOnly(c.ClosedAt)), cellN(fmt.Sprintf("%d / %d", c.ItemsCollected, c.Items)),
			moneyCell(c.BilledTotal), moneyCell(c.CollectedTotal), moneyCell(c.CommissionTotal)})
	}
	parts := []mi.H{acctTitle(t, "accounting.closes.title"), q.banner()}
	if q.can(auth.PermAccountingCloseCreate) {
		parts = append(parts, acctPanel(t("accounting.closes.new"), acctBody(
			func(b *mi.Builder) mi.Node {
				return b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400"), t("accounting.closes.hint"))
			},
			acctConfirmForm(acctConfirm{Message: t("accounting.closes.confirm"), Title: t("accounting.closes.close"), OK: t("accounting.closes.close"), Danger: true},
				"/accounting/closes", q.csrf(), false, "flex items-end gap-2",
				acctField(t("accounting.col.period"), "period", "text", "", mi.Placeholder("YYYY-MM"), mi.Required(), mi.Attr("maxlength", "7")),
				acctSubmit(t("accounting.closes.close"))))))
	}
	parts = append(parts, acctPanel(t("accounting.closes.title"), acctTable(
		[]string{t("accounting.col.period"), t("accounting.closes.closed_at"), t("accounting.closes.items"),
			rh(t("accounting.closes.billed")), rh(t("accounting.closes.collected")), rh(t("accounting.closes.commission"))}, rows, t("accounting.empty"))))
	q.render(t("accounting.closes.title"), "/accounting/closes", acctPage(parts...))
}

func (h *UIHandler) AcctCloseCreate(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	if err := r.ParseForm(); err != nil {
		q.redirectErr("/accounting/closes", err)
		return
	}
	period := strings.TrimSpace(r.FormValue("period"))
	if _, err := q.svc.CloseMonth(q.ctx(), period, q.user.ID); err != nil {
		q.redirectErr("/accounting/closes", err)
		return
	}
	q.acctAudit("close", period)
	q.redirectMsg("/accounting/closes", "ok", "month_closed")
}

func (h *UIHandler) AcctCompany(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	t := q.t
	c, err := q.svc.Company(q.ctx())
	if err != nil {
		q.fail(err)
		return
	}
	ro := []mi.Attribute{}
	if !q.can(auth.PermAccountingCompanyEdit) {
		ro = append(ro, mi.Attr("readonly", "readonly"))
	}
	f := func(label, name, kind, val string) mi.H {
		a := []any{}
		for _, x := range ro {
			a = append(a, x)
		}
		return acctField(label, name, kind, val, a...)
	}
	fields := []mi.H{
		f(t("accounting.company.legal_name"), "legal_name", "text", c.LegalName),
		f(t("accounting.company.tax_id"), "tax_id", "text", c.TaxID),
		f(t("accounting.company.address"), "address", "text", c.Address),
		f(t("accounting.company.country"), "country", "text", c.Country),
		f(t("accounting.company.billing_email"), "billing_email", "email", c.BillingEmail),
	}
	if q.can(auth.PermAccountingCompanyEdit) {
		fields = append(fields, acctSubmit(t("accounting.save")))
	}
	q.render(t("accounting.company.title"), "/accounting/company", acctPage(
		acctTitle(t, "accounting.company.title"), q.banner(),
		acctPanel("", acctBody(acctForm("/accounting/company", q.csrf(), false, "space-y-4 max-w-xl", fields...)))))
}

func (h *UIHandler) AcctCompanySave(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	if err := r.ParseForm(); err != nil {
		q.redirectErr("/accounting/company", err)
		return
	}
	err := q.svc.SaveCompany(q.ctx(), accounting.Company{
		LegalName:    strings.TrimSpace(r.FormValue("legal_name")),
		TaxID:        strings.TrimSpace(r.FormValue("tax_id")),
		Address:      strings.TrimSpace(r.FormValue("address")),
		Country:      strings.TrimSpace(r.FormValue("country")),
		BillingEmail: strings.TrimSpace(r.FormValue("billing_email")),
	})
	if err != nil {
		q.redirectErr("/accounting/company", err)
		return
	}
	q.acctAudit("company.save", "")
	q.redirectMsg("/accounting/company", "ok", "company_saved")
}
