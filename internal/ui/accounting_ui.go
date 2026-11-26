package ui

// Accounting & Admin module (Seam Admin): the screens and handlers.
//
// The business rules live in internal/accounting (Service); this file only
// decides who may do what (permissions and row scoping), reads the form, calls
// the service and draws the result. Everything is behind
// features.enable_accounting, and the routes mount only when a Service was
// wired in (Router.SetAccounting).
//
// Two kinds of user meet here and must never see each other's data: staff
// (admin_staff, accountant, admin), who see everything their permissions
// allow, and Account Executives, whose every route is scoped to the AE linked
// to their login. The scoping is done in the handlers (acctOwnAE,
// acctCanSeeSettlement), not in the permission string.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	mi "github.com/ha1tch/minty"

	"github.com/ha1tch/seam-ui/internal/accounting"
	"github.com/ha1tch/seam-ui/internal/auth"
)

// acctState is the module's per-handler state.
type acctState struct {
	svc     *accounting.Service
	opts    AccountingOptions
	last    *acctSyncInfo // result of the last pull, kept in memory only
	names   acctNames
	mu      sync.Mutex
	started bool // Bootstrap succeeded
}

// AccountingOptions are the module's policy settings (see config.AccountingConfig).
type AccountingOptions struct {
	// TenantID, when not 0, restricts the module to the users of that tenant.
	TenantID int64
	// DisableAESelfRegister stops account_executive users from registering
	// themselves on first visit; only AEs already known to Admin can then use
	// the portal. The zero value keeps the design's "register, then authorize".
	DisableAESelfRegister bool
	// PageSize is the rows per page in lists; 0 means 50.
	PageSize int
	// HideSyncProblems stops the Collections page from showing the problems
	// found by the last pull.
	HideSyncProblems bool
	// DisableDashboard makes the module's home page redirect to the first
	// screen the user may use instead of showing summary figures.
	DisableDashboard bool
}

// acctSyncInfo is the last pull's outcome.
type acctSyncInfo struct {
	At  time.Time
	By  int64
	Res accounting.SyncResult
}

// SetAccountingOptions sets the policy options. Call before serving.
func (r *Router) SetAccountingOptions(o AccountingOptions) { r.handler.acct.opts = o }

// SetAccounting wires the Accounting & Admin service into the router. Without
// it the module's routes are not mounted.
func (r *Router) SetAccounting(svc *accounting.Service) {
	r.handler.acct.svc = svc
}

// acctReady runs Bootstrap once (and again after a failure, e.g. xolu not up
// yet at start-up), then reports whether the service is usable.
func (h *UIHandler) acctReady(ctx context.Context) error {
	st := &h.acct
	if st.svc == nil {
		return errors.New("accounting is not configured")
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.started {
		return nil
	}
	if err := st.svc.Bootstrap(ctx); err != nil {
		return err
	}
	st.started = true
	return nil
}

// acctLimit caps the body of the module's uploads (a document plus its form).
func acctLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, accounting.MaxDocumentBytes+(1<<20))
		next(w, r)
	}
}

// MountAccountingRoutes registers the Accounting & Admin routes. Each is gated
// by the permission of the area it belongs to (auth.PermAccounting*); the
// handlers add the row-level scoping for Account Executives.
func (r *Router) MountAccountingRoutes(mux *http.ServeMux) {
	if !r.config.EnableAccounting || r.handler.acct.svc == nil {
		return
	}
	h := r.handler
	p := r.requirePerm
	mux.HandleFunc("GET /accounting", r.requireAuth(h.AcctHome))

	// Account Executive portal (own data only).
	mux.HandleFunc("GET /accounting/portal", p(auth.PermAccountingPortalView, h.AcctPortal))
	mux.HandleFunc("GET /accounting/portal/profile", p(auth.PermAccountingProfileEdit, h.AcctProfileForm))
	mux.HandleFunc("POST /accounting/portal/profile", p(auth.PermAccountingProfileEdit, h.AcctProfileSave))
	mux.HandleFunc("GET /accounting/portal/settle", p(auth.PermAccountingSettlementCreate, h.AcctSettleForm))
	mux.HandleFunc("POST /accounting/portal/settle", acctLimit(p(auth.PermAccountingSettlementCreate, h.AcctSettleCreate)))

	// Staff.
	mux.HandleFunc("GET /accounting/aes", p(auth.PermAccountingAEView, h.AcctAEs))
	mux.HandleFunc("GET /accounting/aes/{id}/edit", p(auth.PermAccountingAEEdit, h.AcctAEEditForm))
	mux.HandleFunc("POST /accounting/aes/{id}/edit", p(auth.PermAccountingAEEdit, h.AcctAEEditSave))
	mux.HandleFunc("POST /accounting/aes/{id}/authorize", p(auth.PermAccountingAEAuthorize, h.AcctAEAuthorize))
	mux.HandleFunc("GET /accounting/collections", p(auth.PermAccountingCollectionView, h.AcctCollections))
	mux.HandleFunc("POST /accounting/collections/sync", p(auth.PermAccountingCollectionSync, h.AcctSync))
	mux.HandleFunc("GET /accounting/items/{id}/collect", p(auth.PermAccountingCollectionCollect, h.AcctCollectForm))
	mux.HandleFunc("POST /accounting/items/{id}/collect", p(auth.PermAccountingCollectionCollect, h.AcctCollect))
	mux.HandleFunc("POST /accounting/items/{id}/notify", p(auth.PermAccountingCollectionCollect, h.AcctNotify))
	mux.HandleFunc("GET /accounting/commissions", p(auth.PermAccountingCommissionView, h.AcctCommissions))
	mux.HandleFunc("GET /accounting/settlements", p(auth.PermAccountingSettlementView, h.AcctSettlements))
	mux.HandleFunc("POST /accounting/settlements/{id}/pay", acctLimit(p(auth.PermAccountingSettlementPay, h.AcctPay)))

	// Shared between staff and the owning AE: the handler checks ownership.
	mux.HandleFunc("GET /accounting/settlements/{id}", r.requireAuth(h.AcctSettlementDetail))
	mux.HandleFunc("GET /accounting/documents/{id}", r.requireAuth(h.AcctDocument))

	// Books.
	mux.HandleFunc("GET /accounting/ledger", p(auth.PermAccountingLedgerView, h.AcctLedger))
	mux.HandleFunc("GET /accounting/accounts", p(auth.PermAccountingLedgerView, h.AcctAccounts))
	mux.HandleFunc("GET /accounting/accounts/{code}", p(auth.PermAccountingLedgerView, h.AcctAccountBook))
	mux.HandleFunc("GET /accounting/entries/new", p(auth.PermAccountingEntryCreate, h.AcctEntryForm))
	mux.HandleFunc("POST /accounting/entries", p(auth.PermAccountingEntryCreate, h.AcctEntryCreate))
	mux.HandleFunc("POST /accounting/entries/{id}/reverse", p(auth.PermAccountingEntryReverse, h.AcctEntryReverse))
	mux.HandleFunc("GET /accounting/closes", p(auth.PermAccountingCloseView, h.AcctCloses))
	mux.HandleFunc("POST /accounting/closes", p(auth.PermAccountingCloseCreate, h.AcctCloseCreate))
	mux.HandleFunc("GET /accounting/formulas", p(auth.PermAccountingFormulaView, h.AcctFormulas))
	mux.HandleFunc("GET /accounting/formulas/new", p(auth.PermAccountingFormulaEdit, h.AcctFormulaNew))
	mux.HandleFunc("POST /accounting/formulas", p(auth.PermAccountingFormulaEdit, h.AcctFormulaSave))
	mux.HandleFunc("GET /accounting/formulas/{id}/edit", p(auth.PermAccountingFormulaEdit, h.AcctFormulaEdit))
	mux.HandleFunc("POST /accounting/formulas/{id}", p(auth.PermAccountingFormulaEdit, h.AcctFormulaSave))
	mux.HandleFunc("POST /accounting/formulas/{id}/enable", p(auth.PermAccountingFormulaEdit, h.AcctFormulaEnable))
	mux.HandleFunc("POST /accounting/formulas/{id}/move", p(auth.PermAccountingFormulaEdit, h.AcctFormulaMove))
	mux.HandleFunc("POST /accounting/formulas/{id}/delete", p(auth.PermAccountingFormulaEdit, h.AcctFormulaDelete))
	mux.HandleFunc("GET /accounting/company", p(auth.PermAccountingCompanyView, h.AcctCompany))
	mux.HandleFunc("POST /accounting/company", p(auth.PermAccountingCompanyEdit, h.AcctCompanySave))
}

// ---------------------------------------------------------------------------
// plumbing
// ---------------------------------------------------------------------------

type acctReq struct {
	h    *UIHandler
	w    http.ResponseWriter
	r    *http.Request
	t    func(key string, args ...any) string
	user *auth.User
	svc  *accounting.Service
}

// acctBegin prepares a request: the service must be up and a user known.
// It writes the error page itself and returns nil when it cannot go on.
func (h *UIHandler) acctBegin(w http.ResponseWriter, r *http.Request) *acctReq {
	t := h.translateFunc(r)
	if err := h.acctReady(r.Context()); err != nil {
		h.logger.Error("accounting is not available", "error", err)
		h.page.RenderError(w, http.StatusServiceUnavailable, t("accounting.err.unavailable_title"), t("accounting.err.unavailable"))
		return nil
	}
	u := h.getCurrentUser(r)
	if u == nil || u.ID == 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return nil
	}
	if t := h.acct.opts.TenantID; t != 0 && u.TenantID != t {
		h.Error403(w, r)
		return nil
	}
	return &acctReq{h: h, w: w, r: r, t: t, user: u, svc: h.acct.svc}
}

func (q *acctReq) ctx() context.Context { return q.r.Context() }
func (q *acctReq) can(p auth.Permission) bool {
	return auth.HasPermission(q.user.Role, p)
}

func (q *acctReq) render(title, active string, content mi.H) {
	q.h.page.Render(q.w, q.r, title, active, content)
}

func (q *acctReq) notFound() {
	q.h.page.RenderError(q.w, http.StatusNotFound, q.t("error.not_found"), q.t("error.not_found"))
}

func (q *acctReq) forbidden() { q.h.Error403(q.w, q.r) }

func (q *acctReq) fail(err error) {
	q.h.logger.Error("accounting request failed", "path", q.r.URL.Path, "error", err)
	q.h.page.RenderError(q.w, http.StatusInternalServerError, q.t("error.internal"), q.t(acctErrKey(err)))
}

// redirectMsg sends the user back to a page with a one-shot message. Only the
// message key travels, and only keys of this module's own namespaces are shown.
func (q *acctReq) redirectMsg(path, kind, key string) {
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	http.Redirect(q.w, q.r, path+sep+kind+"="+key, http.StatusSeeOther)
}

// redirectErr sends the user back with the message of err.
func (q *acctReq) redirectErr(path string, err error) {
	if acctErrKey(err) == "accounting.err.generic" {
		q.h.logger.Error("accounting operation failed", "path", q.r.URL.Path, "error", err)
	}
	q.redirectMsg(path, "err", strings.TrimPrefix(acctErrKey(err), "accounting.err."))
}

// banner returns the message carried by ?ok= / ?err=, if any. It also removes
// those parameters from the address bar, so that a refresh (or a copied link)
// does not show the message again.
func (q *acctReq) banner() mi.H {
	var alert mi.H
	if k := q.r.URL.Query().Get("ok"); acctSafeKey(k) {
		alert = Alert(q.t("accounting.msg."+k), "success", true)
	} else if k := q.r.URL.Query().Get("err"); acctSafeKey(k) {
		alert = Alert(q.t("accounting.err."+k), "error", true)
	}
	if alert == nil {
		return nil
	}
	return func(b *mi.Builder) mi.Node {
		return b.Div(alert(b), b.Script(mi.Raw(bannerScript)))
	}
}

const bannerScript = `(function(){try{var u=new URL(location.href);
if(u.searchParams.has('ok')||u.searchParams.has('err')){u.searchParams.delete('ok');u.searchParams.delete('err');
history.replaceState(history.state,'',u.pathname+u.search+u.hash);}}catch(e){}})();`

func acctSafeKey(k string) bool {
	if k == "" || len(k) > 40 {
		return false
	}
	for _, c := range k {
		if !(c >= 'a' && c <= 'z' || c == '_') {
			return false
		}
	}
	return true
}

// acctErrKey maps a domain error to its i18n key.
func acctErrKey(err error) string {
	table := []struct {
		err error
		key string
	}{
		{accounting.ErrAlreadyCollected, "already_collected"},
		{accounting.ErrAENotOperational, "ae_not_operational"},
		{accounting.ErrAENoAccount, "ae_no_account"},
		{accounting.ErrAEAlreadyAuthed, "already_authorized"},
		{accounting.ErrNotRealAE, "not_real_ae"},
		{accounting.ErrNoCompany, "no_company"},
		{accounting.ErrNoPayoutMethod, "no_payout_method"},
		{accounting.ErrInvoiceMismatch, "invoice_mismatch"},
		{accounting.ErrLineUnavailable, "line_unavailable"},
		{accounting.ErrNothingSelected, "nothing_selected"},
		{accounting.ErrNotPayable, "not_payable"},
		{accounting.ErrAlreadyReversed, "already_reversed"},
		{accounting.ErrNotReversible, "not_reversible"},
		{accounting.ErrAlreadyClosed, "already_closed"},
		{accounting.ErrPeriodNotFinished, "period_not_finished"},
		{accounting.ErrCloseOrder, "close_order"},
		{accounting.ErrNoInstance, "no_instance"},
		{accounting.ErrBadDocument, "bad_document"},
		{accounting.ErrNoBlobs, "no_blobs"},
		{accounting.ErrTooManyLines, "too_many_lines"},
		{accounting.ErrBadPeriod, "bad_period"},
		{accounting.ErrPeriodClosed, "period_closed"},
		{accounting.ErrBadDate, "bad_date"},
		{accounting.ErrBadAmount, "bad_amount"},
		{accounting.ErrBadMovement, "bad_movement"},
		{accounting.ErrUnbalanced, "unbalanced"},
		{accounting.ErrTooFewLines, "too_few_lines"},
		{accounting.ErrNoDescription, "no_description"},
		{accounting.ErrBadCompany, "bad_company"},
		{accounting.ErrOQLDisabled, "oql_disabled"},
		{accounting.ErrNoOQL, "no_oql"},
		{accounting.ErrFormulaFailed, "formula_failed"},
		{accounting.ErrUnknownAccount, "bad_account"},
		{accounting.ErrNotPostable, "bad_account"},
		{accounting.ErrNotFound, "not_found"},
		{accounting.ErrConflict, "conflict"},
	}
	var fp *accounting.FormulaProblem
	if errors.As(err, &fp) {
		return "accounting.err.formula_" + fp.Code
	}
	for _, e := range table {
		if errors.Is(err, e.err) {
			return "accounting.err." + e.key
		}
	}
	return "accounting.err.generic"
}

// ownAE returns the AE linked to the logged-in user, if any.
func (q *acctReq) ownAE() (accounting.AE, bool, error) {
	return q.svc.AEByUser(q.ctx(), q.user.ID)
}

func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

// ---------------------------------------------------------------------------
// small view helpers
// ---------------------------------------------------------------------------

func acctTitle(t func(string, ...any) string, key string, actions ...mi.H) mi.H {
	return func(b *mi.Builder) mi.Node {
		args := []interface{}{mi.Class("flex items-center justify-between gap-4")}
		args = append(args, b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), t(key)))
		var acts []interface{}
		acts = append(acts, mi.Class("flex items-center gap-2"))
		for _, a := range actions {
			if a != nil {
				acts = append(acts, a(b))
			}
		}
		args = append(args, b.Div(acts...))
		return b.Div(args...)
	}
}

func acctPage(parts ...mi.H) mi.H {
	return func(b *mi.Builder) mi.Node {
		args := []interface{}{mi.Class("space-y-6")}
		for _, p := range parts {
			if p != nil {
				args = append(args, p(b))
			}
		}
		return b.Div(args...)
	}
}

const (
	acctBtn     = "px-3 py-1.5 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 text-sm font-medium whitespace-nowrap"
	acctBtnGrey = "px-3 py-1.5 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg text-sm font-medium hover:bg-gray-50 dark:hover:bg-gray-700 whitespace-nowrap"
	acctInput   = "w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-white text-sm"
	acctLabel   = "block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"
	// acctBtnDanger is the outline button of an action that cannot be undone.
	acctBtnDanger = "px-3 py-1.5 border border-red-300 dark:border-red-700 text-red-700 dark:text-red-400 rounded-lg text-sm font-medium hover:bg-gray-50 dark:hover:bg-gray-700 whitespace-nowrap"
)

// acctForm wraps fields in a POST form with the CSRF token. multipart is for
// forms that upload a file.
func acctForm(action, csrf string, multipart bool, class string, children ...mi.H) mi.H {
	return func(b *mi.Builder) mi.Node {
		args := []interface{}{mi.Method("post"), mi.Action(action), mi.Class(class)}
		if multipart {
			args = append(args, mi.Enctype("multipart/form-data"))
		}
		args = append(args, CSRFField(csrf)(b))
		for _, c := range children {
			if c != nil {
				args = append(args, c(b))
			}
		}
		return b.Form(args...)
	}
}

// acctConfirm describes a confirmation asked before a form is sent, using the
// app's own dialog (SeamConfirm). Danger paints the OK button red.
type acctConfirm struct {
	Message, Title, OK string
	Danger             bool
	ID                 string // optional id of the form, for scripts
}

// attr returns the onsubmit attribute. The texts travel as JSON string
// literals, so quotes and angle brackets in them cannot break out of the
// script.
func (c acctConfirm) attr() mi.Attribute {
	q := func(s string) string { j, _ := json.Marshal(s); return string(j) }
	return mi.Attr("onsubmit", fmt.Sprintf("SeamConfirm.showForm(this,%s,{danger:%t,title:%s,okLabel:%s});return false;",
		q(c.Message), c.Danger, q(c.Title), q(c.OK)))
}

// acctConfirmForm is acctForm with a confirmation before sending.
func acctConfirmForm(c acctConfirm, action, csrf string, multipart bool, class string, children ...mi.H) mi.H {
	return func(b *mi.Builder) mi.Node {
		args := []interface{}{mi.Method("post"), mi.Action(action), mi.Class(class), c.attr()}
		if c.ID != "" {
			args = append(args, mi.ID(c.ID))
		}
		if multipart {
			args = append(args, mi.Enctype("multipart/form-data"))
		}
		args = append(args, CSRFField(csrf)(b))
		for _, ch := range children {
			if ch != nil {
				args = append(args, ch(b))
			}
		}
		return b.Form(args...)
	}
}

// acctPanel is a white box with an optional title and no inner padding, so a
// table can use the full width.
func acctPanel(title string, content mi.H) mi.H {
	return func(b *mi.Builder) mi.Node {
		args := []interface{}{mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700")}
		if title != "" {
			args = append(args, b.Div(mi.Class("px-6 py-4 border-b border-gray-200 dark:border-gray-700"),
				b.H2(mi.Class("text-lg font-semibold text-gray-900 dark:text-white"), title)))
		}
		return b.Div(append(args, content(b))...)
	}
}

// acctBody pads content inside a panel.
func acctBody(parts ...mi.H) mi.H {
	return func(b *mi.Builder) mi.Node {
		args := []interface{}{mi.Class("p-6 space-y-4")}
		for _, p := range parts {
			if p != nil {
				args = append(args, p(b))
			}
		}
		return b.Div(args...)
	}
}

func acctSubmit(label string) mi.H {
	return func(b *mi.Builder) mi.Node {
		return b.Button(mi.Type("submit"), mi.Class(acctBtn), label)
	}
}

func acctLink(href, label string) mi.H {
	return func(b *mi.Builder) mi.Node {
		return b.A(mi.Href(href), mi.Class(acctBtnGrey), label)
	}
}

type acctOpt struct{ Value, Label string }

// acctField is a labelled input; kind "select" takes opts.
//
// Options after value: a []acctOpt (select), mi.Attribute values, acctID to set
// the control's id (needed when the same field appears in several forms of one
// page: ids must be unique), and acctSrOnly to keep the label for screen
// readers but not show it.
func acctField(label, name, kind, value string, rest ...any) mi.H {
	var opts []acctOpt
	var attrs []mi.Attribute
	id, srOnly, extra := "f_"+name, false, ""
	for _, x := range rest {
		switch v := x.(type) {
		case []acctOpt:
			opts = v
		case mi.Attribute:
			attrs = append(attrs, v)
		case acctID:
			id = string(v)
		case acctSrOnlyLabel:
			srOnly = true
		case acctClass:
			extra += " " + string(v)
		}
	}
	return func(b *mi.Builder) mi.Node {
		var control mi.Node
		base := []interface{}{mi.Name(name), mi.ID(id), mi.Class(acctInput + extra)}
		for _, a := range attrs {
			base = append(base, a)
		}
		switch kind {
		case "select":
			for _, o := range opts {
				oa := []interface{}{mi.Value(o.Value)}
				if o.Value == value {
					oa = append(oa, mi.Selected())
				}
				oa = append(oa, o.Label)
				base = append(base, b.Option(oa...))
			}
			control = b.Select(base...)
		case "textarea":
			base = append(base, value)
			control = b.Textarea(base...)
		default:
			base = append(base, mi.Type(kind))
			if value != "" {
				base = append(base, mi.Value(value))
			}
			attrsOnly := make([]mi.Attribute, 0, len(base))
			for _, x := range base {
				if a, ok := x.(mi.Attribute); ok {
					attrsOnly = append(attrsOnly, a)
				}
			}
			control = b.Input(attrsOnly...)
		}
		lc := acctLabel
		if srOnly {
			lc = "sr-only"
		}
		return b.Div(b.Label(mi.For(id), mi.Class(lc), label), control)
	}
}

// acctID overrides a field's control id; acctSrOnlyLabel hides its label
// visually (it stays for screen readers).
type (
	acctID          string
	acctSrOnlyLabel struct{}
	acctClass       string // extra classes on the control
)

func money(c accounting.Cents) string { return "USD " + c.Display() }

func moneyCell(c accounting.Cents) mi.H {
	cls := "px-4 py-3 text-sm text-right tabular-nums text-gray-900 dark:text-white whitespace-nowrap"
	if c < 0 {
		cls = "px-4 py-3 text-sm text-right tabular-nums text-red-600 dark:text-red-400 whitespace-nowrap"
	}
	return func(b *mi.Builder) mi.Node { return b.Td(mi.Class(cls), money(c)) }
}

func acctDefList(rows [][2]string) mi.H {
	return func(b *mi.Builder) mi.Node {
		args := []interface{}{mi.Class("grid grid-cols-1 md:grid-cols-2 gap-4 text-sm")}
		for _, r := range rows {
			args = append(args, b.Div(
				b.Dt(mi.Class("text-gray-500 dark:text-gray-400"), r[0]),
				b.Dd(mi.Class("text-gray-900 dark:text-white font-medium"), r[1]),
			))
		}
		return b.Dl(args...)
	}
}

func today() string { return time.Now().UTC().Format(accounting.DateLayout) }

func stateBadge(t func(string, ...any) string, kind, state string) mi.H {
	variant := "draft"
	switch state {
	case "collected", "paid", "authorized":
		variant = "success"
	case "collection_in_progress", "invoice_loaded":
		variant = "info"
	case "registered", "pending":
		variant = "warning"
	}
	return Badge(t("accounting.state."+kind+"."+state), variant)
}

// sourceBadge labels where a ledger entry came from.
func sourceBadge(t func(string, ...any) string, source string) mi.H {
	variant := "draft"
	switch source {
	case "manual":
		variant = "info"
	case "reversal":
		variant = "warning"
	}
	return Badge(t("accounting.source."+source), variant)
}

// acctBtnDangerSubmit is a submit button for an irreversible action.
func acctBtnDangerSubmit(label string) mi.H {
	return func(b *mi.Builder) mi.Node {
		return b.Button(mi.Type("submit"), mi.Class(acctBtnDanger), label)
	}
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func itoa(i int64) string { return strconv.FormatInt(i, 10) }
func fmtInt(i int) string { return fmt.Sprint(i) }

// ---------------------------------------------------------------------------
// paging and filters
// ---------------------------------------------------------------------------

const acctMaxWindow = 2000 // most rows a list will look through

func (h *UIHandler) acctPageSize() int {
	if n := h.acct.opts.PageSize; n >= 10 && n <= 200 {
		return n
	}
	return 50
}

// pageParams returns the requested page (1-based) and the page size.
func (q *acctReq) pageParams() (page, size int) {
	size = q.h.acctPageSize()
	page, _ = strconv.Atoi(q.r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	if page*size > acctMaxWindow-1 {
		page = (acctMaxWindow - 1) / size
	}
	return page, size
}

// window is how many rows a list must fetch to fill the page and know whether
// there is a next one.
func windowFor(page, size int) int { return page*size + 1 }

// paged cuts the requested page out of all (the rows from the top of the list)
// and reports whether a next page exists.
func paged[T any](all []T, page, size int) ([]T, bool) {
	start := (page - 1) * size
	if start >= len(all) {
		return nil, false
	}
	end := start + size
	if end >= len(all) {
		return all[start:], false
	}
	return all[start:end], true
}

// pager draws previous/next links that keep the current filters.
func (q *acctReq) pager(page int, hasNext bool) mi.H {
	if page <= 1 && !hasNext {
		return nil
	}
	link := func(p int, label string) mi.H {
		v := q.r.URL.Query()
		v.Del("ok")
		v.Del("err")
		v.Set("page", strconv.Itoa(p))
		return acctLink(q.r.URL.Path+"?"+v.Encode(), label)
	}
	var parts []mi.H
	if page > 1 {
		parts = append(parts, link(page-1, q.t("accounting.page.prev")))
	}
	parts = append(parts, tx(q.t("accounting.page.n", strconv.Itoa(page))))
	if hasNext {
		parts = append(parts, link(page+1, q.t("accounting.page.next")))
	}
	return func(b *mi.Builder) mi.Node {
		args := []interface{}{mi.Class("flex items-center gap-3 justify-end text-sm text-gray-600 dark:text-gray-300")}
		for _, p := range parts {
			args = append(args, p(b))
		}
		return b.Div(args...)
	}
}

// acctFilter is a GET form of fields with an Apply button.
func (q *acctReq) acctFilter(fields ...mi.H) mi.H {
	return func(b *mi.Builder) mi.Node {
		args := []interface{}{mi.Method("get"), mi.Action(q.r.URL.Path), mi.Class("flex flex-wrap items-end gap-3")}
		for _, f := range fields {
			args = append(args, f(b))
		}
		args = append(args, b.Button(mi.Type("submit"), mi.Class(acctBtn), q.t("accounting.filter.apply")))
		return b.Form(args...)
	}
}

// queryText returns a trimmed, length-limited query value.
func (q *acctReq) queryText(name string) string {
	v := strings.TrimSpace(q.r.URL.Query().Get(name))
	if len(v) > 40 {
		return ""
	}
	return v
}
