package ui

// Accounting & Admin: Account Executive portal, AEs, collections, commissions
// and settlements.

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	mi "github.com/ha1tch/minty"

	"github.com/ha1tch/seam-ui/internal/accounting"
	"github.com/ha1tch/seam-ui/internal/auth"
)

// ---------------------------------------------------------------------------
// shared helpers
// ---------------------------------------------------------------------------

func (q *acctReq) csrf() string { return q.h.GetCSRFToken(q.w, q.r) }

func tx(s string) mi.H { return func(b *mi.Builder) mi.Node { return b.Text(s) } }

const cellClass = "px-4 py-3 text-sm text-gray-900 dark:text-white"

func cellH(c mi.H) mi.H {
	return func(b *mi.Builder) mi.Node { return b.Td(mi.Class(cellClass), c(b)) }
}

func cellT(s string) mi.H { return cellH(tx(s)) }

// cellRight is a table cell holding a button, aligned to the right.
func cellRight(c mi.H) mi.H {
	return func(b *mi.Builder) mi.Node {
		return b.Td(mi.Class("px-4 py-3 text-sm text-right"), c(b))
	}
}

// cellN is a cell whose text never wraps (dates, periods, codes).
func cellN(s string) mi.H {
	return func(b *mi.Builder) mi.Node { return b.Td(mi.Class(cellClass+" whitespace-nowrap"), s) }
}

// rightMark in front of a column title right-aligns that column's header, for
// columns of numbers (rh adds it).
const rightMark = "\x00r"

func rh(s string) string { return rightMark + s }

// acctTable draws a table: header labels and rows of ready-made cells. With no
// rows it shows a quiet empty state instead.
func acctTable(head []string, rows [][]mi.H, empty string) mi.H {
	return func(b *mi.Builder) mi.Node {
		if len(rows) == 0 {
			return b.Div(mi.Class("text-center py-10 text-gray-500 dark:text-gray-400"),
				b.Div(b.Span(mi.Class("material-icons text-4xl text-gray-300 dark:text-gray-600"), "inbox")),
				b.P(mi.Class("mt-2 text-sm"), empty))
		}
		ths := []interface{}{}
		for _, h := range head {
			align := "text-left"
			if strings.HasPrefix(h, rightMark) {
				h, align = strings.TrimPrefix(h, rightMark), "text-right"
			}
			ths = append(ths, b.Th(mi.Scope("col"), mi.Class("px-4 py-3 "+align+" text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider whitespace-nowrap"), h))
		}
		trs := []interface{}{}
		for _, r := range rows {
			tds := []interface{}{}
			for _, c := range r {
				tds = append(tds, c(b))
			}
			trs = append(trs, b.Tr(append([]interface{}{mi.Class("border-t border-gray-100 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700")}, tds...)...))
		}
		return b.Div(mi.Class("overflow-x-auto"),
			b.Table(mi.Class("min-w-full"),
				b.Thead(b.Tr(ths...)),
				b.Tbody(trs...)))
	}
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

func isAE(u *auth.User) bool { return u.Role == auth.RoleAccountExecutive }

// acctAudit records a state-changing action in the audit log when one is set.
func (q *acctReq) acctAudit(action, detail string) {
	q.h.logger.Info("accounting action", "action", action, "detail", detail, "user_id", q.user.ID)
}

// portalAE returns the AE of the logged-in user, registering it the first time
// an account_executive user arrives (a real AE signs up, then Seam authorizes).
func (q *acctReq) portalAE() (accounting.AE, error) {
	ae, ok, err := q.ownAE()
	if err != nil {
		return accounting.AE{}, err
	}
	if ok {
		return ae, nil
	}
	if !isAE(q.user) || q.h.acct.opts.DisableAESelfRegister {
		return accounting.AE{}, accounting.ErrNotRealAE
	}
	name := strings.TrimSpace(q.user.Name)
	if name == "" {
		name = q.user.Email
	}
	if name == "" {
		name = "AE " + itoa(q.user.ID)
	}
	return q.svc.RegisterAE(q.ctx(), q.user.ID, name, q.user.Email)
}

// readDocument reads the uploaded file and its data from a multipart form.
func (q *acctReq) readDocument(typ accounting.DocType) (accounting.DocumentInput, error) {
	return q.readDocumentAs(typ, "")
}

// readDocumentAs is readDocument for a form whose document fields carry a
// name prefix ("doc_file", "doc_number"...), so they can sit beside the form's
// own fields. An absent file is accounting.ErrBadDocument, as always.
func (q *acctReq) readDocumentAs(typ accounting.DocType, prefix string) (accounting.DocumentInput, error) {
	if err := q.r.ParseMultipartForm(accounting.MaxDocumentBytes); err != nil {
		return accounting.DocumentInput{}, fmt.Errorf("%w: %v", accounting.ErrBadDocument, err)
	}
	f, hdr, err := q.r.FormFile(prefix + "file")
	if err != nil {
		return accounting.DocumentInput{}, fmt.Errorf("%w: no file", accounting.ErrBadDocument)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, accounting.MaxDocumentBytes+1))
	if err != nil {
		return accounting.DocumentInput{}, fmt.Errorf("%w: %v", accounting.ErrBadDocument, err)
	}
	val := func(name string) string { return strings.TrimSpace(q.r.FormValue(prefix + name)) }
	in := accounting.DocumentInput{
		Type:         typ,
		Number:       val("number"),
		Date:         val("date"),
		Filename:     hdr.Filename,
		Data:         data,
		Currency:     strings.ToUpper(val("currency")),
		ForeignTotal: val("foreign_total"),
		FxRate:       val("fx_rate"),
		FxDate:       val("fx_date"),
	}
	if a := val("amount"); a != "" {
		c, err := accounting.ParseUSD(a)
		if err != nil {
			return in, err
		}
		in.AmountUSD = c
	}
	return in, nil
}

// documentFields are the form fields of a document upload: the file, its
// number, date and USD amount, then the optional foreign-currency block.
func (q *acctReq) documentFields(amountLabel string, amount string) []mi.H {
	return q.docFields("", true, true, amountLabel, amount)
}

// docFields builds the document block. prefix is put in front of every field
// name; required makes the file mandatory; withAmount adds the USD amount (a
// ledger entry has its own total, so its block leaves it out).
func (q *acctReq) docFields(prefix string, required, withAmount bool, amountLabel, amount string) []mi.H {
	t := q.t
	fileOpts := []any{mi.Attr("accept", "application/pdf,image/png,image/jpeg")}
	if required {
		fileOpts = append(fileOpts, mi.Required())
	}
	dateDefault := today()
	if !required {
		dateDefault = "" // an entry's document takes the entry's date unless told otherwise
	}
	return []mi.H{
		acctField(t("accounting.doc.file"), prefix+"file", "file", "", fileOpts...),
		func(b *mi.Builder) mi.Node {
			cells := []interface{}{mi.Class("grid grid-cols-1 md:grid-cols-3 gap-4"),
				acctField(t("accounting.doc.number"), prefix+"number", "text", "")(b),
				acctField(t("accounting.doc.date"), prefix+"date", "date", dateDefault)(b)}
			if withAmount {
				cells = append(cells, acctField(amountLabel, prefix+"amount", "text", amount, mi.Required(), mi.Attr("inputmode", "decimal"), mi.Placeholder("0.00"))(b))
			}
			return b.Div(cells...)
		},
		func(b *mi.Builder) mi.Node {
			return b.Div(mi.Class("rounded-lg border border-gray-200 dark:border-gray-700 p-4 space-y-4"),
				b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400"), t("accounting.doc.foreign_hint")),
				b.Div(mi.Class("grid grid-cols-1 md:grid-cols-2 gap-4"),
					acctField(t("accounting.doc.currency"), prefix+"currency", "text", "", mi.Placeholder("EUR"), mi.Attr("maxlength", "3"))(b),
					acctField(t("accounting.doc.foreign_total"), prefix+"foreign_total", "text", "")(b),
					acctField(t("accounting.doc.fx_rate"), prefix+"fx_rate", "text", "")(b),
					acctField(t("accounting.doc.fx_date"), prefix+"fx_date", "date", "")(b)))
		},
	}
}

// payoutLabel is the display name of a payout method.
func payoutLabel(t func(string, ...any) string, m accounting.PayoutMethod) string {
	switch m {
	case accounting.PayoutTransfer:
		return t("accounting.payout.transfer")
	case accounting.PayoutPayPal:
		return t("accounting.payout.paypal")
	}
	return "—"
}

// dateOnly cuts a timestamp ("2026-10-31T02:21:45Z") down to its date.
func dateOnly(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return orDash(s)
}

// nameLookup returns the display name of a record by id, or "" when unknown.
type nameLookup func(id int64) string

// acctNames caches the names shown in lists, so a page of 50 rows does not
// read every tenant. Misses are fetched one by one, so a tenant beyond any
// cap still gets its name. The cache lives 30 seconds and is cleared by
// anything that changes names (a pull, an AE edit).
type acctNames struct {
	mu      sync.Mutex
	at      time.Time
	tenants map[int64]string
	aes     map[int64]string
}

const acctNamesTTL = 30 * time.Second

func (n *acctNames) reset() {
	n.mu.Lock()
	n.tenants, n.aes = nil, nil
	n.mu.Unlock()
}

func (n *acctNames) fresh() {
	if n.tenants == nil || time.Since(n.at) > acctNamesTTL {
		n.tenants, n.aes, n.at = map[int64]string{}, map[int64]string{}, time.Now()
	}
}

func (q *acctReq) aeNames() nameLookup {
	n := &q.h.acct.names
	return func(id int64) string {
		n.mu.Lock()
		defer n.mu.Unlock()
		n.fresh()
		if v, ok := n.aes[id]; ok {
			return v
		}
		name := ""
		if ae, err := q.svc.AEByID(q.ctx(), id); err == nil {
			name = ae.Name
		}
		n.aes[id] = name
		return name
	}
}

func (q *acctReq) tenantNames() nameLookup {
	n := &q.h.acct.names
	return func(id int64) string {
		n.mu.Lock()
		defer n.mu.Unlock()
		n.fresh()
		if v, ok := n.tenants[id]; ok {
			return v
		}
		name := ""
		if t, err := q.svc.TenantByID(q.ctx(), id); err == nil {
			name = t.Name
		}
		n.tenants[id] = name
		return name
	}
}

func (q *acctReq) itemRows(items []accounting.Item, withAE bool) [][]mi.H {
	tn := q.tenantNames()
	an := q.aeNames()
	var rows [][]mi.H
	for _, it := range items {
		r := []mi.H{cellT(itoa(it.ID)), cellN(it.Period), cellT(orDash(tn(it.TenantID)))}
		if withAE {
			r = append(r, cellT(orDash(an(it.AEID))))
		}
		r = append(r,
			cellT(q.t("accounting.concept."+string(it.Concept))),
			moneyCell(it.Amount),
			cellH(stateBadge(q.t, "item", string(it.State))),
		)
		rows = append(rows, r)
	}
	return rows
}

// ---------------------------------------------------------------------------
// home and portal
// ---------------------------------------------------------------------------

// AcctHome is the module's front door: AEs go to their portal; staff see the
// summary (unless disabled), or are sent to the first screen they may use.
func (h *UIHandler) AcctHome(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	if isAE(q.user) {
		http.Redirect(w, r, "/accounting/portal", http.StatusSeeOther)
		return
	}
	staff := []struct {
		p    auth.Permission
		path string
	}{
		{auth.PermAccountingCollectionView, "/accounting/collections"},
		{auth.PermAccountingLedgerView, "/accounting/ledger"},
		{auth.PermAccountingSettlementView, "/accounting/settlements"},
	}
	if !h.acct.opts.DisableDashboard {
		for _, d := range staff {
			if q.can(d.p) {
				q.dashboard()
				return
			}
		}
	}
	for _, d := range staff {
		if q.can(d.p) {
			http.Redirect(w, r, d.path, http.StatusSeeOther)
			return
		}
	}
	q.forbidden()
}

// stateTotals counts and adds up the items in state (looking at most
// acctMaxWindow of them).
func (q *acctReq) stateTotals(state accounting.ItemState) (n int, sum accounting.Cents, capped bool, err error) {
	items, err := q.svc.ListItems(q.ctx(), accounting.ItemFilter{State: state, Limit: acctMaxWindow})
	for _, it := range items {
		sum += it.Amount
	}
	return len(items), sum, len(items) >= acctMaxWindow, err
}

func acctTile(label, value, note, href string) mi.H {
	return func(b *mi.Builder) mi.Node {
		inner := []interface{}{
			b.Div(mi.Class("text-sm text-gray-500 dark:text-gray-400"), label),
			b.Div(mi.Class("mt-1 text-2xl font-bold text-gray-900 dark:text-white tabular-nums"), value),
		}
		if note != "" {
			inner = append(inner, b.Div(mi.Class("mt-1 text-xs text-gray-500 dark:text-gray-400"), note))
		}
		box := "block p-4 bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700"
		if href != "" {
			return b.A(append([]interface{}{mi.Href(href), mi.Class(box + " hover:border-indigo-400")}, inner...)...)
		}
		return b.Div(append([]interface{}{mi.Class(box)}, inner...)...)
	}
}

// dashboard draws the summary. Each figure shows only to a user who may see
// its source.
func (q *acctReq) dashboard() {
	t := q.t
	var tiles []mi.H
	more := func(capped bool) string {
		if capped {
			return t("accounting.dash.capped", fmtInt(acctMaxWindow))
		}
		return ""
	}
	if q.can(auth.PermAccountingCollectionView) {
		for _, st := range []accounting.ItemState{accounting.ItemPending, accounting.ItemInProgress} {
			n, sum, capped, err := q.stateTotals(st)
			if err != nil {
				q.fail(err)
				return
			}
			tiles = append(tiles, acctTile(t("accounting.state.item."+string(st)), money(sum),
				strings.TrimSpace(t("accounting.dash.items", fmtInt(n))+" "+more(capped)), "/accounting/collections?state="+string(st)))
		}
	}
	if q.can(auth.PermAccountingCommissionView) {
		rows, err := q.svc.ListCommission(q.ctx(), accounting.CommissionFilter{Limit: acctMaxWindow})
		if err != nil {
			q.fail(err)
			return
		}
		var sum accounting.Cents
		n := 0
		for _, c := range rows {
			if c.SettlementID == 0 {
				sum += c.Amount
				n++
			}
		}
		tiles = append(tiles, acctTile(t("accounting.dash.unsettled"), money(sum),
			strings.TrimSpace(t("accounting.dash.lines", fmtInt(n))+" "+more(len(rows) >= acctMaxWindow)), "/accounting/commissions"))
	}
	if q.can(auth.PermAccountingSettlementView) {
		sets, err := q.svc.ListSettlements(q.ctx(), accounting.SettlementFilter{State: accounting.SettlementInvoiceLoaded, Limit: acctMaxWindow})
		if err != nil {
			q.fail(err)
			return
		}
		var sum accounting.Cents
		for _, s := range sets {
			sum += s.Total
		}
		tiles = append(tiles, acctTile(t("accounting.dash.to_pay"), money(sum),
			t("accounting.dash.settlements", fmtInt(len(sets))), "/accounting/settlements"))
	}
	if q.can(auth.PermAccountingLedgerView) {
		_, bal, err := q.svc.Balances(q.ctx(), "")
		if err != nil {
			q.fail(err)
			return
		}
		tiles = append(tiles,
			acctTile(t("accounting.dash.cash"), money(bal[accounting.CodeCashGroup]), "", "/accounting/accounts"),
			acctTile(t("accounting.dash.payable"), money(bal[accounting.CodeAEPayables]), "", "/accounting/accounts"))
	}
	if !q.h.acct.opts.HideSyncProblems && q.can(auth.PermAccountingCollectionView) {
		if last := q.h.lastSync(); last != nil && len(last.Res.Problems) > 0 {
			tiles = append(tiles, acctTile(t("accounting.dash.sync_problems"), fmtInt(len(last.Res.Problems)),
				last.At.UTC().Format("2006-01-02 15:04 UTC"), "/accounting/collections"))
		}
	}
	grid := func(b *mi.Builder) mi.Node {
		args := []interface{}{mi.Class("grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4")}
		for _, tl := range tiles {
			args = append(args, tl(b))
		}
		return b.Div(args...)
	}
	q.render(t("accounting.dash.title"), "/accounting", acctPage(acctTitle(t, "accounting.dash.title"), q.banner(), grid))
}

func (h *UIHandler) lastSync() *acctSyncInfo {
	h.acct.mu.Lock()
	defer h.acct.mu.Unlock()
	return h.acct.last
}

// AcctPortal is the Account Executive's own page: status, balance, items,
// commission and settlements. Everything is scoped to the session's AE.
func (h *UIHandler) AcctPortal(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	ae, err := q.portalAE()
	if err != nil {
		if errors.Is(err, accounting.ErrNotRealAE) {
			q.forbidden()
			return
		}
		q.fail(err)
		return
	}
	t := q.t
	status := t("accounting.state.ae.registered")
	if ae.CanOperate() {
		status = t("accounting.state.ae.authorized")
	}
	var unsettled accounting.Cents
	var lines []accounting.CommissionLine
	var comm []accounting.CommissionRow
	var book accounting.Ledger
	var tenants []accounting.Tenant
	var items []accounting.Item
	var sets []accounting.Settlement
	if ae.CanOperate() {
		if lines, err = q.svc.UnsettledLines(q.ctx(), ae.ID); err != nil {
			q.fail(err)
			return
		}
		for _, l := range lines {
			unsettled += l.Amount
		}
		// Only this AE's own lines and own account: what the AE sees is
		// what it is due to receive, never what the customer pays (§6.4).
		if comm, err = q.svc.ListCommission(q.ctx(), accounting.CommissionFilter{AEID: ae.ID, Limit: 200}); err != nil {
			q.fail(err)
			return
		}
		// ListTenants(0) means "every tenant"; an AE without an id must never reach it.
		if ae.ID != 0 {
			if tenants, err = q.svc.ListTenants(q.ctx(), ae.ID); err != nil {
				q.fail(err)
				return
			}
		}
		if ae.AccountCode != "" {
			if book, err = q.svc.Ledger(q.ctx(), ae.AccountCode, "", ""); err != nil {
				q.fail(err)
				return
			}
		}
		if items, err = q.svc.ListItems(q.ctx(), accounting.ItemFilter{AEID: ae.ID, Limit: 100}); err != nil {
			q.fail(err)
			return
		}
		if sets, err = q.svc.ListSettlements(q.ctx(), accounting.SettlementFilter{AEID: ae.ID, Limit: 50}); err != nil {
			q.fail(err)
			return
		}
	}
	var srows [][]mi.H
	for _, s := range sets {
		srows = append(srows, []mi.H{
			cellH(func(b *mi.Builder) mi.Node {
				return b.A(mi.Href("/accounting/settlements/"+itoa(s.ID)), mi.Class("text-indigo-600 hover:underline"), "#"+itoa(s.ID))
			}),
			cellT(s.CreatedAt), moneyCell(s.Total), cellH(stateBadge(t, "settlement", string(s.State))),
		})
	}
	acts := []mi.H{acctLink("/accounting/portal/profile", t("accounting.portal.edit_profile"))}
	if ae.CanOperate() && q.can(auth.PermAccountingSettlementCreate) {
		acts = append(acts, acctLink("/accounting/portal/settle", t("accounting.portal.new_settlement")))
	}
	parts := []mi.H{
		acctTitle(t, "accounting.portal.title", acts...),
		q.banner(),
		Card(t("accounting.portal.status"), acctDefList([][2]string{
			{t("accounting.ae.name"), ae.Name},
			{t("accounting.ae.status"), status},
			{t("accounting.ae.payout_method"), payoutLabel(t, ae.PayoutMethod)},
			{t("accounting.portal.unsettled"), money(unsettled)},
		})),
	}
	if !ae.CanOperate() {
		parts = append(parts, Alert(t("accounting.portal.waiting"), "info", false))
	} else {
		parts = append(parts,
			acctPanel(t("accounting.portal.account"), q.portalAccount(book)),
			acctPanel(t("accounting.portal.commissions"), q.portalCommissions(comm)),
			acctPanel(t("accounting.portal.tenants"), q.portalTenants(tenants)),
			acctPanel(t("accounting.portal.items"), acctTable(
				[]string{"#", t("accounting.col.period"), t("accounting.col.tenant"), t("accounting.col.concept"), t("accounting.col.state")},
				q.portalItemRows(items), t("accounting.empty"))),
			acctPanel(t("accounting.portal.settlements"), acctTable(
				[]string{"#", t("accounting.col.date"), rh(t("accounting.col.amount")), t("accounting.col.state")}, srows, t("accounting.empty"))),
		)
	}
	q.render(t("accounting.portal.title"), "/accounting", acctPage(parts...))
}

// portalItemRows lists the AE's items by state only: the amount the customer
// pays is not the AE's to see; what it earns is in the commission table.
func (q *acctReq) portalItemRows(items []accounting.Item) [][]mi.H {
	tn := q.tenantNames()
	var rows [][]mi.H
	for _, it := range items {
		rows = append(rows, []mi.H{
			cellT(itoa(it.ID)), cellN(it.Period), cellT(orDash(tn(it.TenantID))),
			cellT(q.t("accounting.concept." + string(it.Concept))),
			cellH(stateBadge(q.t, "item", string(it.State))),
		})
	}
	return rows
}

// portalTenants lists the tenants this AE created or was assigned. Licences
// and renewal dates are not here: they come from the platform (section 7.4).
func (q *acctReq) portalTenants(tenants []accounting.Tenant) mi.H {
	t := q.t
	var rows [][]mi.H
	for _, tn := range tenants {
		rows = append(rows, []mi.H{
			cellT(tn.Name), cellN(orDash(tn.Country)), cellT(orDash(tn.Account)), cellT(orDash(tn.Registration)),
		})
	}
	return acctTable([]string{t("accounting.col.tenant"), t("accounting.col.country"), t("accounting.col.customer_account"),
		t("accounting.col.registration")}, rows, t("accounting.empty"))
}

// portalCommissions is the AE's commission broken down by month, tenant,
// concept and detail (§6.4), with the settlement that paid it.
func (q *acctReq) portalCommissions(comm []accounting.CommissionRow) mi.H {
	t := q.t
	tn := q.tenantNames()
	var rows [][]mi.H
	for _, c := range comm {
		set := "—"
		if c.SettlementID != 0 {
			set = "#" + itoa(c.SettlementID)
		}
		rows = append(rows, []mi.H{
			cellN(c.Period), cellT(orDash(tn(c.TenantID))), cellT(t("accounting.concept." + string(c.Concept))),
			cellT(c.Detail), moneyCell(c.Amount), cellT(set),
		})
	}
	return acctTable([]string{t("accounting.col.period"), t("accounting.col.tenant"), t("accounting.col.concept"),
		t("accounting.col.detail"), rh(t("accounting.portal.commission")), t("accounting.col.settlement")}, rows, t("accounting.empty"))
}

// portalAccount is the AE's own account (2.1.x): balance owed to it and the
// movements behind it, each labelled with its rubro. Newest first.
func (q *acctReq) portalAccount(l accounting.Ledger) mi.H {
	t := q.t
	const maxLines = 100
	var rows [][]mi.H
	for i := len(l.Lines) - 1; i >= 0 && len(rows) < maxLines; i-- {
		ln := l.Lines[i]
		var dr, cr accounting.Cents
		if ln.Side == accounting.Debit {
			dr = ln.Amount
		} else {
			cr = ln.Amount
		}
		rows = append(rows, []mi.H{
			cellN(ln.Date), cellH(sourceBadge(t, string(ln.Source))), cellT(ln.Description),
			moneyCell(cr), moneyCell(dr), moneyCell(ln.Balance),
		})
	}
	return func(b *mi.Builder) mi.Node {
		return b.Div(
			acctBody(acctDefList([][2]string{{t("accounting.portal.balance"), money(l.Closing)}}))(b),
			acctTable([]string{t("accounting.col.date"), t("accounting.col.rubro"), t("accounting.col.description"),
				rh(t("accounting.portal.earned")), rh(t("accounting.portal.paid")), rh(t("accounting.col.balance"))}, rows, t("accounting.empty"))(b))
	}
}

// profileForm draws the AE profile form posting to action.
func (q *acctReq) profileForm(ae accounting.AE, action, titleKey string) {
	t := q.t
	form := acctForm(action, q.csrf(), false, "space-y-4 max-w-xl",
		acctField(t("accounting.ae.name"), "name", "text", ae.Name, mi.Required()),
		acctField(t("accounting.ae.email"), "email", "email", ae.Email),
		acctField(t("accounting.ae.countries"), "countries", "text", strings.Join(ae.Countries, ", ")),
		acctField(t("accounting.ae.payout_method"), "payout_method", "select", string(ae.PayoutMethod), []acctOpt{
			{"", "—"},
			{string(accounting.PayoutTransfer), t("accounting.payout.transfer")},
			{string(accounting.PayoutPayPal), t("accounting.payout.paypal")},
		}),
		acctField(t("accounting.ae.payout_details"), "payout_details", "textarea", ae.PayoutDetails),
		acctSubmit(t("accounting.save")),
	)
	q.render(t(titleKey), "/accounting", acctPage(acctTitle(t, titleKey), q.banner(), Card(ae.Name, form)))
}

// readProfile reads the profile fields of the form.
func readProfile(r *http.Request) (accounting.AEProfile, error) {
	if err := r.ParseForm(); err != nil {
		return accounting.AEProfile{}, err
	}
	var countries []string
	for _, c := range strings.Split(r.FormValue("countries"), ",") {
		if c = strings.ToUpper(strings.TrimSpace(c)); c != "" {
			countries = append(countries, c)
		}
	}
	return accounting.AEProfile{
		Name:          strings.TrimSpace(r.FormValue("name")),
		Email:         strings.TrimSpace(r.FormValue("email")),
		Countries:     countries,
		PayoutMethod:  accounting.PayoutMethod(r.FormValue("payout_method")),
		PayoutDetails: strings.TrimSpace(r.FormValue("payout_details")),
	}, nil
}

func (h *UIHandler) AcctProfileForm(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	ae, err := q.portalAE()
	if err != nil {
		q.forbidden()
		return
	}
	q.profileForm(ae, "/accounting/portal/profile", "accounting.portal.edit_profile")
}

func (h *UIHandler) AcctProfileSave(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	ae, err := q.portalAE()
	if err != nil {
		q.forbidden()
		return
	}
	p, err := readProfile(r)
	if err != nil {
		q.redirectErr("/accounting/portal/profile", err)
		return
	}
	if _, err = q.svc.SaveAEProfile(q.ctx(), ae.ID, p); err != nil {
		q.redirectErr("/accounting/portal/profile", err)
		return
	}
	q.acctAudit("ae.profile", itoa(ae.ID)+payoutChange(ae, p))
	q.redirectMsg("/accounting/portal", "ok", "profile_saved")
}

// payoutChange notes in the audit detail that where the money goes changed.
func payoutChange(old accounting.AE, p accounting.AEProfile) string {
	if old.PayoutMethod != p.PayoutMethod || old.PayoutDetails != p.PayoutDetails {
		return " payout_changed"
	}
	return ""
}

// AcctAEEditForm and AcctAEEditSave let staff edit a real AE's profile. They
// are routed only for users holding accounting.ae.edit: the admin role, and
// admin_staff when staff_edit_ae_profiles is on.
func (h *UIHandler) AcctAEEditForm(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	id, ok := pathID(r)
	if !ok {
		q.notFound()
		return
	}
	ae, err := q.svc.AEByID(q.ctx(), id)
	if err != nil || ae.Kind != accounting.AEReal {
		q.notFound()
		return
	}
	q.profileForm(ae, "/accounting/aes/"+itoa(id)+"/edit", "accounting.ae.edit")
}

func (h *UIHandler) AcctAEEditSave(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	id, ok := pathID(r)
	if !ok {
		q.notFound()
		return
	}
	ae, err := q.svc.AEByID(q.ctx(), id)
	if err != nil || ae.Kind != accounting.AEReal {
		q.notFound()
		return
	}
	back := "/accounting/aes/" + itoa(id) + "/edit"
	p, err := readProfile(r)
	if err != nil {
		q.redirectErr(back, err)
		return
	}
	if _, err = q.svc.SaveAEProfile(q.ctx(), id, p); err != nil {
		q.redirectErr(back, err)
		return
	}
	q.acctAudit("ae.profile.staff", itoa(id)+payoutChange(ae, p))
	q.redirectMsg("/accounting/aes", "ok", "profile_saved")
}

// ---------------------------------------------------------------------------
// settlement creation (AE)
// ---------------------------------------------------------------------------

func (h *UIHandler) AcctSettleForm(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	ae, err := q.portalAE()
	if err != nil || !ae.CanOperate() {
		q.forbidden()
		return
	}
	t := q.t
	lines, err := q.svc.UnsettledLines(q.ctx(), ae.ID)
	if err != nil {
		q.fail(err)
		return
	}
	co, _ := q.svc.Company(q.ctx())
	tn := q.tenantNames()
	var rows [][]mi.H
	for _, l := range lines {
		id := itoa(l.ID)
		cents := strconv.FormatInt(int64(l.Amount), 10)
		desc := orDash(tn(l.TenantID)) + " " + l.Period
		rows = append(rows, []mi.H{
			cellH(func(b *mi.Builder) mi.Node {
				return b.Input(mi.Type("checkbox"), mi.Name("line"), mi.Value(id), mi.ID("l"+id),
					mi.Attr("data-cents", cents), mi.Attr("aria-label", desc))
			}),
			cellN(l.Period), cellT(orDash(tn(l.TenantID))),
			cellT(t("accounting.concept." + string(l.Concept))), moneyCell(l.Amount),
		})
	}
	billed := t("accounting.settle.billed_to", orDash(co.LegalName)) + " " + t("accounting.settle.max_lines", fmtInt(accounting.MaxSettlementLines))
	table := acctTable(
		[]string{"", t("accounting.col.period"), t("accounting.col.tenant"), t("accounting.col.concept"), rh(t("accounting.col.amount"))},
		rows, t("accounting.settle.no_lines"))
	if len(rows) == 0 {
		q.render(t("accounting.portal.new_settlement"), "/accounting", acctPage(
			acctTitle(t, "accounting.portal.new_settlement"), q.banner(), acctPanel(t("accounting.settle.pick"), table)))
		return
	}
	total := func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("flex items-center justify-between rounded-lg bg-gray-50 dark:bg-gray-900 px-4 py-3 text-sm"),
			b.Div(mi.Class("text-gray-600 dark:text-gray-300"), t("accounting.settle.selected"), " ",
				b.Span(mi.ID("sel-count"), mi.Class("font-semibold"), "0")),
			b.Div(mi.Class("text-gray-600 dark:text-gray-300"), t("accounting.settle.must_equal"), " ",
				b.Span(mi.ID("sel-total"), mi.Class("font-semibold text-gray-900 dark:text-white"), "USD 0.00")))
	}
	script := func(b *mi.Builder) mi.Node {
		return b.Script(mi.Raw(settleScript))
	}
	pick := acctPanel(t("accounting.settle.pick"), func(b *mi.Builder) mi.Node {
		return b.Div(table(b), b.Div(mi.Class("px-6 pb-4 pt-2"),
			b.Label(mi.Class("inline-flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300"),
				b.Input(mi.Type("checkbox"), mi.ID("sel-all")), t("accounting.settle.select_all"))))
	})
	fields := []mi.H{
		func(b *mi.Builder) mi.Node {
			return b.P(mi.Class("text-sm text-gray-600 dark:text-gray-300"), billed)
		}, total}
	fields = append(fields, q.documentFields(t("accounting.settle.invoice_total"), "")...)
	fields = append(fields, acctSubmit(t("accounting.settle.create")))
	invoice := acctPanel(t("accounting.settle.invoice"), acctBody(fields...))
	form := acctConfirmForm(acctConfirm{Message: t("accounting.settle.confirm"), Title: t("accounting.settle.create"),
		OK: t("accounting.settle.create"), ID: "settle-form"},
		"/accounting/portal/settle", q.csrf(), true, "space-y-6", pick, invoice, script)
	q.render(t("accounting.portal.new_settlement"), "/accounting", acctPage(
		acctTitle(t, "accounting.portal.new_settlement", acctLink("/accounting/portal", t("accounting.back"))), q.banner(), form))
}

// settleScript keeps the running count and total of the selected lines on the
// settlement form and fills the invoice total with it until the AE types one.
const settleScript = `(function(){
var f=document.getElementById('settle-form');if(!f)return;
var boxes=[].slice.call(f.querySelectorAll('input[name=line]'));
var amount=f.querySelector('input[name=amount]');var dirty=false;
if(amount)amount.addEventListener('input',function(){dirty=true;});
function fmt(c){var s=String(Math.abs(c));while(s.length<3)s='0'+s;return s.slice(0,-2).replace(/\B(?=(\d{3})+(?!\d))/g,',')+'.'+s.slice(-2);}
function update(){var n=0,c=0;boxes.forEach(function(b){if(b.checked){n++;c+=parseInt(b.getAttribute('data-cents'),10)||0;}});
document.getElementById('sel-count').textContent=n;document.getElementById('sel-total').textContent='USD '+fmt(c);
if(amount&&!dirty)amount.value=n?fmt(c).replace(/,/g,''):'';}
boxes.forEach(function(b){b.addEventListener('change',update);});
var all=document.getElementById('sel-all');if(all)all.addEventListener('change',function(){boxes.forEach(function(b){b.checked=all.checked;});update();});
update();})();`

func (h *UIHandler) AcctSettleCreate(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	ae, err := q.portalAE()
	if err != nil || !ae.CanOperate() {
		q.forbidden()
		return
	}
	in, err := q.readDocument(accounting.DocInvoice)
	if err != nil {
		q.redirectErr("/accounting/portal/settle", err)
		return
	}
	var ids []int64
	for _, v := range r.MultipartForm.Value["line"] {
		var id int64
		if _, e := fmt.Sscan(v, &id); e == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	s, err := q.svc.CreateSettlement(q.ctx(), ae.ID, ids, in, q.user.ID)
	if err != nil {
		q.redirectErr("/accounting/portal/settle", err)
		return
	}
	q.acctAudit("settlement.create", itoa(s.ID))
	q.redirectMsg("/accounting/settlements/"+itoa(s.ID), "ok", "settlement_created")
}

// ---------------------------------------------------------------------------
// AEs
// ---------------------------------------------------------------------------

func (h *UIHandler) AcctAEs(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	aes, err := q.svc.ListAEs(q.ctx())
	if err != nil {
		q.fail(err)
		return
	}
	t := q.t
	canAuth := q.can(auth.PermAccountingAEAuthorize)
	canEdit := q.can(auth.PermAccountingAEEdit)
	csrf := q.csrf()
	var rows [][]mi.H
	for _, a := range aes {
		st := string(a.Status)
		if a.Kind == accounting.AEVirtual {
			st = "authorized"
		}
		var btns []mi.H
		if canAuth && a.Kind == accounting.AEReal && a.Status == accounting.AERegistered {
			btns = append(btns, acctConfirmForm(acctConfirm{
				Message: t("accounting.ae.authorize_confirm", a.Name), Title: t("accounting.ae.authorize"), OK: t("accounting.ae.authorize"),
			}, "/accounting/aes/"+itoa(a.ID)+"/authorize", csrf, false, "", acctSubmit(t("accounting.ae.authorize"))))
		}
		if canEdit && a.Kind == accounting.AEReal {
			btns = append(btns, acctLink("/accounting/aes/"+itoa(a.ID)+"/edit", t("accounting.ae.edit")))
		}
		act := cellRight(func(b *mi.Builder) mi.Node {
			inner := []interface{}{mi.Class("flex justify-end gap-2")}
			for _, x := range btns {
				inner = append(inner, x(b))
			}
			return b.Div(inner...)
		})
		rows = append(rows, []mi.H{
			cellN(a.Name), cellT(t("accounting.ae.kind." + string(a.Kind))), cellT(orDash(a.Email)),
			cellT(orDash(strings.Join(a.Countries, ", "))), cellT(payoutLabel(t, a.PayoutMethod)),
			cellN(a.AccountCode), cellH(stateBadge(t, "ae", st)), act,
		})
	}
	q.render(t("accounting.aes.title"), "/accounting/aes", acctPage(
		acctTitle(t, "accounting.aes.title"), q.banner(),
		acctPanel("", acctTable(
			[]string{t("accounting.ae.name"), t("accounting.ae.kind"), t("accounting.ae.email"), t("accounting.col.countries"),
				t("accounting.col.payout"), t("accounting.ae.account"), t("accounting.col.state"), ""}, rows, t("accounting.empty")))))
}

func (h *UIHandler) AcctAEAuthorize(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	id, ok := pathID(r)
	if !ok {
		q.notFound()
		return
	}
	if _, err := q.svc.AuthorizeAE(q.ctx(), id, q.user.ID); err != nil {
		q.redirectErr("/accounting/aes", err)
		return
	}
	q.acctAudit("ae.authorize", itoa(id))
	q.redirectMsg("/accounting/aes", "ok", "ae_authorized")
}

// ---------------------------------------------------------------------------
// collections
// ---------------------------------------------------------------------------

func (h *UIHandler) AcctCollections(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	t := q.t
	state := accounting.ItemState(r.URL.Query().Get("state"))
	if state != "" && !state.Valid() {
		state = ""
	}
	period := q.queryText("period")
	page, size := q.pageParams()
	all, err := q.svc.ListItems(q.ctx(), accounting.ItemFilter{State: state, Period: period, Limit: windowFor(page, size)})
	if err != nil {
		q.fail(err)
		return
	}
	items, hasNext := paged(all, page, size)
	canCollect := q.can(auth.PermAccountingCollectionCollect)
	csrf := q.csrf()
	tn, an := q.tenantNames(), q.aeNames()
	var rows [][]mi.H
	for _, it := range items {
		var act mi.H = cellT("")
		id := itoa(it.ID)
		switch {
		case canCollect && it.State != accounting.ItemCollected:
			act = cellRight(acctLink("/accounting/items/"+id+"/collect", t("accounting.collect.mark")))
		case canCollect && q.svc.NotifiesOnCollect() && it.State == accounting.ItemCollected && !it.InstanceNotified:
			act = cellRight(acctForm("/accounting/items/"+id+"/notify", csrf, false, "", acctSubmit(t("accounting.collect.notify"))))
		}
		rows = append(rows, []mi.H{
			cellN(id), cellN(it.Period), cellT(orDash(tn(it.TenantID))), cellT(orDash(an(it.AEID))),
			cellT(t("accounting.concept." + string(it.Concept))), moneyCell(it.Amount),
			cellH(stateBadge(t, "item", string(it.State))), act,
		})
	}
	var acts []mi.H
	if q.can(auth.PermAccountingCollectionSync) {
		acts = append(acts, acctForm("/accounting/collections/sync", csrf, false, "", acctSubmit(t("accounting.collect.sync"))))
	}
	stateOpts := []acctOpt{{"", t("accounting.filter.all")}}
	for _, s := range []accounting.ItemState{accounting.ItemPending, accounting.ItemInProgress, accounting.ItemCollected} {
		stateOpts = append(stateOpts, acctOpt{string(s), t("accounting.state.item." + string(s))})
	}
	filter := q.acctFilter(
		acctField(t("accounting.col.state"), "state", "select", string(state), stateOpts),
		acctField(t("accounting.col.period"), "period", "text", period, mi.Placeholder("YYYY-MM")))
	q.render(t("accounting.collections.title"), "/accounting/collections", acctPage(
		acctTitle(t, "accounting.collections.title", acts...), q.banner(), q.syncPanel(), filter,
		q.pager(page, hasNext),
		acctPanel("", acctTable(
			[]string{"#", t("accounting.col.period"), t("accounting.col.tenant"), t("accounting.col.ae"), t("accounting.col.concept"),
				rh(t("accounting.col.amount")), t("accounting.col.state"), ""}, rows, t("accounting.empty"))),
		q.pager(page, hasNext)))
}

// syncPanel shows the outcome of the last pull, when the setting is on.
func (q *acctReq) syncPanel() mi.H {
	if q.h.acct.opts.HideSyncProblems {
		return nil
	}
	last := q.h.lastSync()
	if last == nil {
		return nil
	}
	t := q.t
	r := last.Res
	summary := t("accounting.sync.summary", last.At.UTC().Format("2006-01-02 15:04 UTC"),
		fmtInt(r.ItemsNew), fmtInt(r.ItemsUpdated), fmtInt(r.TenantsNew), fmtInt(r.ExternallyPaid), fmtInt(r.ItemsSkippedClose))
	if len(r.Problems) == 0 {
		return func(b *mi.Builder) mi.Node {
			return b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400"), summary)
		}
	}
	const show = 20
	var lines []mi.H
	for i, p := range r.Problems {
		if i == show {
			lines = append(lines, tx(t("accounting.sync.more", fmtInt(len(r.Problems)-show))))
			break
		}
		lines = append(lines, tx(p))
	}
	return Card(t("accounting.sync.problems", fmtInt(len(r.Problems))), func(b *mi.Builder) mi.Node {
		args := []interface{}{mi.Class("text-sm text-gray-700 dark:text-gray-300 space-y-1")}
		args = append(args, b.P(mi.Class("text-gray-500 dark:text-gray-400"), summary))
		for _, l := range lines {
			args = append(args, b.Div(l(b)))
		}
		return b.Div(args...)
	})
}

func (h *UIHandler) AcctSync(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	res, err := q.svc.Sync(q.ctx())
	if err != nil {
		q.redirectErr("/accounting/collections", err)
		return
	}
	for _, p := range res.Problems {
		q.h.logger.Warn("accounting sync problem", "problem", p)
	}
	q.h.acct.mu.Lock()
	q.h.acct.last = &acctSyncInfo{At: time.Now(), By: q.user.ID, Res: res}
	q.h.acct.mu.Unlock()
	q.acctAudit("sync", fmt.Sprintf("items+%d tenants+%d problems=%d", res.ItemsNew, res.TenantsNew, len(res.Problems)))
	key := "synced"
	if len(res.Problems) > 0 {
		key = "synced_problems"
	}
	q.redirectMsg("/accounting/collections", "ok", key)
}

// AcctCollectForm asks for the cash account and date of a collection, shows
// what is being collected and confirms before sending: marking an item
// collected creates the commission and cannot be taken back.
func (h *UIHandler) AcctCollectForm(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	id, ok := pathID(r)
	if !ok {
		q.notFound()
		return
	}
	it, err := q.svc.GetItem(q.ctx(), id)
	if err != nil {
		q.fail(err)
		return
	}
	t := q.t
	if it.State == accounting.ItemCollected {
		q.redirectErr("/accounting/collections", accounting.ErrAlreadyCollected)
		return
	}
	cash := []acctOpt{
		{accounting.CodeBank, t("accounting.cash.bank")},
		{accounting.CodePayPal, t("accounting.cash.paypal")},
		{accounting.CodeDLocalGo, t("accounting.cash.dlocalgo")},
	}
	summary := acctDefList([][2]string{
		{t("accounting.col.tenant"), orDash(q.tenantNames()(it.TenantID))},
		{t("accounting.col.ae"), orDash(q.aeNames()(it.AEID))},
		{t("accounting.col.period"), it.Period},
		{t("accounting.col.concept"), t("accounting.concept." + string(it.Concept))},
		{t("accounting.col.amount"), money(it.Amount)},
		{t("accounting.col.state"), t("accounting.state.item." + string(it.State))},
	})
	form := acctConfirmForm(acctConfirm{
		Message: t("accounting.collect.confirm", money(it.Amount)), Title: t("accounting.collect.mark"), OK: t("accounting.collect.mark"),
	}, "/accounting/items/"+itoa(id)+"/collect", q.csrf(), false, "space-y-4",
		func(b *mi.Builder) mi.Node {
			return b.Div(mi.Class("grid grid-cols-1 md:grid-cols-2 gap-4"),
				acctField(t("accounting.collect.cash"), "cash", "select", accounting.CodeBank, cash)(b),
				acctField(t("accounting.collect.date"), "date", "date", today(), mi.Required())(b))
		},
		acctSubmit(t("accounting.collect.mark")))
	q.render(t("accounting.collect.mark"), "/accounting/collections", acctPage(
		acctTitle(t, "accounting.collect.mark", acctLink("/accounting/collections", t("accounting.back"))), q.banner(),
		acctPanel("", acctBody(summary, form))))
}

func (h *UIHandler) AcctCollect(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	id, ok := pathID(r)
	if !ok {
		q.notFound()
		return
	}
	if err := r.ParseForm(); err != nil {
		q.redirectErr("/accounting/collections", err)
		return
	}
	res, err := q.svc.MarkCollected(q.ctx(), id, r.FormValue("cash"), strings.TrimSpace(r.FormValue("date")), q.user.ID)
	if err != nil {
		q.redirectErr("/accounting/collections", err)
		return
	}
	q.acctAudit("item.collect", itoa(id))
	key := "collected"
	if res.NotifyErr != nil {
		key = "collected_not_notified"
	}
	q.redirectMsg("/accounting/collections", "ok", key)
}

func (h *UIHandler) AcctNotify(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	id, ok := pathID(r)
	if !ok {
		q.notFound()
		return
	}
	if err := q.svc.NotifyInstance(q.ctx(), id); err != nil {
		q.redirectErr("/accounting/collections", err)
		return
	}
	q.redirectMsg("/accounting/collections", "ok", "notified")
}

// ---------------------------------------------------------------------------
// commissions
// ---------------------------------------------------------------------------

func (h *UIHandler) AcctCommissions(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	t := q.t
	period := q.queryText("period")
	page, size := q.pageParams()
	all, err := q.svc.ListCommission(q.ctx(), accounting.CommissionFilter{Period: period, Limit: windowFor(page, size)})
	if err != nil {
		q.fail(err)
		return
	}
	list, hasNext := paged(all, page, size)
	tn, an := q.tenantNames(), q.aeNames()
	var total accounting.Cents
	var rows [][]mi.H
	for _, c := range list {
		total += c.Amount
		set := "—"
		if c.SettlementID != 0 {
			set = "#" + itoa(c.SettlementID)
		}
		rows = append(rows, []mi.H{
			cellN(c.Period), cellT(orDash(an(c.AEID))), cellT(orDash(tn(c.TenantID))),
			cellT(t("accounting.concept." + string(c.Concept))), cellT(c.Detail), moneyCell(c.Amount), cellT(set),
		})
	}
	filter := q.acctFilter(acctField(t("accounting.col.period"), "period", "text", period, mi.Placeholder("YYYY-MM")))
	q.render(t("accounting.commissions.title"), "/accounting/commissions", acctPage(
		acctTitle(t, "accounting.commissions.title"), q.banner(), filter, q.pager(page, hasNext),
		Card(t("accounting.commissions.total", money(total)), acctTable(
			[]string{t("accounting.col.period"), t("accounting.col.ae"), t("accounting.col.tenant"), t("accounting.col.concept"),
				t("accounting.col.detail"), rh(t("accounting.col.amount")), t("accounting.col.settlement")}, rows, t("accounting.empty"))),
		q.pager(page, hasNext)))
}

// ---------------------------------------------------------------------------
// settlements
// ---------------------------------------------------------------------------

// canSeeSettlement is the row-level rule: staff with the view permission see
// every settlement; an AE sees only its own.
func (q *acctReq) canSeeSettlement(s accounting.Settlement) bool {
	if q.can(auth.PermAccountingSettlementView) {
		return true
	}
	ae, ok, err := q.ownAE()
	return err == nil && ok && ae.ID == s.AEID
}

func (h *UIHandler) AcctSettlements(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	t := q.t
	state := accounting.SettlementState(q.queryText("state"))
	if state != accounting.SettlementInvoiceLoaded && state != accounting.SettlementPaid {
		state = ""
	}
	page, size := q.pageParams()
	all, err := q.svc.ListSettlements(q.ctx(), accounting.SettlementFilter{State: state, Limit: windowFor(page, size)})
	if err != nil {
		q.fail(err)
		return
	}
	sets, hasNext := paged(all, page, size)
	an := q.aeNames()
	var rows [][]mi.H
	for _, s := range sets {
		rows = append(rows, []mi.H{
			cellH(func(b *mi.Builder) mi.Node {
				return b.A(mi.Href("/accounting/settlements/"+itoa(s.ID)), mi.Class("text-indigo-600 hover:underline"), "#"+itoa(s.ID))
			}),
			cellT(orDash(an(s.AEID))), cellT(s.CreatedAt), moneyCell(s.Total), cellT(string(s.Method)),
			cellH(stateBadge(t, "settlement", string(s.State))),
		})
	}
	filter := q.acctFilter(acctField(t("accounting.col.state"), "state", "select", string(state), []acctOpt{
		{"", t("accounting.filter.all")},
		{string(accounting.SettlementInvoiceLoaded), t("accounting.state.settlement.invoice_loaded")},
		{string(accounting.SettlementPaid), t("accounting.state.settlement.paid")},
	}))
	q.render(t("accounting.settlements.title"), "/accounting/settlements", acctPage(
		acctTitle(t, "accounting.settlements.title"), q.banner(), filter,
		Card(t("accounting.settlements.title"), acctTable(
			[]string{"#", t("accounting.col.ae"), t("accounting.col.date"), rh(t("accounting.col.amount")), t("accounting.col.method"), t("accounting.col.state")},
			rows, t("accounting.empty"))),
		q.pager(page, hasNext)))
}

func (h *UIHandler) AcctSettlementDetail(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	id, ok := pathID(r)
	if !ok {
		q.notFound()
		return
	}
	s, err := q.svc.GetSettlement(q.ctx(), id)
	if err != nil || s.State == accounting.SettlementDraft || !q.canSeeSettlement(s) {
		// Not found rather than forbidden: an AE must not learn that another's exists.
		q.notFound()
		return
	}
	t := q.t
	an, tn := q.aeNames(), q.tenantNames()
	lines, err := q.svc.SettlementLines(q.ctx(), s)
	if err != nil {
		q.fail(err)
		return
	}
	rows := [][2]string{
		{t("accounting.col.ae"), orDash(an(s.AEID))},
		{t("accounting.col.amount"), money(s.Total)},
		{t("accounting.col.method"), payoutLabel(t, s.Method)},
		{t("accounting.settle.billed_to_label"), orDash(s.BilledTo)},
		{t("accounting.col.date"), dateOnly(s.CreatedAt)},
		{t("accounting.col.paid_at"), dateOnly(s.PaidAt)},
	}
	var docs []mi.H
	for _, d := range []struct {
		id    int64
		label string
	}{{s.InvoiceDocID, t("accounting.doc.invoice")}, {s.PaymentDocID, t("accounting.doc.proof")}} {
		if d.id == 0 {
			continue
		}
		href, label := "/accounting/documents/"+itoa(d.id), d.label
		docs = append(docs, func(b *mi.Builder) mi.Node {
			return b.A(mi.Href(href), mi.Class(acctBtnGrey), mi.Attr("download", ""),
				b.Span(mi.Class("material-icons text-base align-middle mr-1"), "description"), label)
		})
	}
	var lrows [][]mi.H
	for _, l := range lines {
		lrows = append(lrows, []mi.H{
			cellN(l.Period), cellT(orDash(tn(l.TenantID))), cellT(t("accounting.concept." + string(l.Concept))), moneyCell(l.Amount),
		})
	}
	head := []mi.H{
		func(b *mi.Builder) mi.Node {
			return b.Div(mi.Class("flex items-center gap-3"), stateBadge(t, "settlement", string(s.State))(b))
		},
		acctDefList(rows),
	}
	if len(docs) > 0 {
		head = append(head, func(b *mi.Builder) mi.Node {
			args := []interface{}{mi.Class("flex flex-wrap gap-2")}
			for _, d := range docs {
				args = append(args, d(b))
			}
			return b.Div(args...)
		})
	}
	parts := []mi.H{
		acctTitle(t, "accounting.settlements.detail", acctLink("/accounting/settlements", t("accounting.back"))), q.banner(),
		acctPanel("#"+itoa(s.ID), acctBody(head...)),
		acctPanel(t("accounting.settlements.lines"), acctTable(
			[]string{t("accounting.col.period"), t("accounting.col.tenant"), t("accounting.col.concept"), rh(t("accounting.col.amount"))},
			lrows, t("accounting.empty"))),
	}
	if s.State == accounting.SettlementInvoiceLoaded && q.can(auth.PermAccountingSettlementPay) {
		fields := []mi.H{acctField(t("accounting.pay.date"), "paid_date", "date", today(), mi.Required())}
		fields = append(fields, q.documentFields(t("accounting.pay.amount"), s.Total.Display())...)
		fields = append(fields, acctSubmit(t("accounting.pay.mark_paid")))
		parts = append(parts, acctPanel(t("accounting.pay.title"), acctBody(
			acctConfirmForm(acctConfirm{
				Message: t("accounting.pay.confirm", money(s.Total), orDash(an(s.AEID))), Title: t("accounting.pay.mark_paid"), OK: t("accounting.pay.mark_paid"),
			}, "/accounting/settlements/"+itoa(s.ID)+"/pay", q.csrf(), true, "space-y-4", fields...))))
	}
	q.render(t("accounting.settlements.detail"), "/accounting/settlements", acctPage(parts...))
}

func (h *UIHandler) AcctPay(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	id, ok := pathID(r)
	if !ok {
		q.notFound()
		return
	}
	back := "/accounting/settlements/" + itoa(id)
	in, err := q.readDocument(accounting.DocPaymentProof)
	if err != nil {
		q.redirectErr(back, err)
		return
	}
	date := strings.TrimSpace(r.FormValue("paid_date"))
	if _, _, err := q.svc.PaySettlement(q.ctx(), id, in, date, q.user.ID); err != nil {
		q.redirectErr(back, err)
		return
	}
	q.acctAudit("settlement.pay", itoa(id))
	q.redirectMsg(back, "ok", "settlement_paid")
}

// AcctDocument serves a stored document as a download. Staff with the
// settlement view permission may fetch any; an AE only the documents of its own
// settlements.
func (h *UIHandler) AcctDocument(w http.ResponseWriter, r *http.Request) {
	q := h.acctBegin(w, r)
	if q == nil {
		return
	}
	id, ok := pathID(r)
	if !ok {
		q.notFound()
		return
	}
	// A document that backs a manual ledger entry is for ledger readers; a
	// settlement document is for settlement readers and for its own AE.
	rec, err := q.svc.GetDocument(q.ctx(), id)
	if err != nil {
		if errors.Is(err, accounting.ErrNotFound) {
			q.notFound()
			return
		}
		q.fail(err)
		return
	}
	allowed := q.can(auth.PermAccountingSettlementView)
	if rec.ForEntry {
		allowed = q.can(auth.PermAccountingLedgerView)
	}
	if !allowed && !rec.ForEntry {
		if ae, found, err := q.ownAE(); err == nil && found {
			sets, _ := q.svc.ListSettlements(q.ctx(), accounting.SettlementFilter{AEID: ae.ID, Limit: 500})
			for _, s := range sets {
				if s.InvoiceDocID == id || s.PaymentDocID == id {
					allowed = true
				}
			}
		}
	}
	if !allowed {
		q.notFound()
		return
	}
	doc, data, err := q.svc.DocumentContent(q.ctx(), id)
	if err != nil {
		if errors.Is(err, accounting.ErrNotFound) {
			q.notFound()
			return
		}
		q.fail(err)
		return
	}
	ct := doc.ContentType
	if ct == "" {
		ct = "application/octet-stream"
	}
	name := strings.NewReplacer(`"`, "", "\r", "", "\n", "", "\\", "").Replace(doc.Filename)
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox")
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = w.Write(data)
}
