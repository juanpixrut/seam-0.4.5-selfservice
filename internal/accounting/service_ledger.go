package accounting

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

func trim(s string) string { return strings.TrimSpace(s) }

// Chart loads the chart of accounts.
func (s *Service) Chart(ctx context.Context) (*Chart, error) {
	docs, err := s.find(ctx, Query{Entity: EntAccount, Limit: 5000})
	if err != nil {
		return nil, err
	}
	accounts, err := decodeAll[Account](docs)
	if err != nil {
		return nil, err
	}
	return NewChart(accounts)
}

// ClosedPeriods returns the set of closed months ("2026-09" -> true).
func (s *Service) ClosedPeriods(ctx context.Context) (map[string]bool, error) {
	docs, err := s.find(ctx, Query{Entity: EntClose, Limit: 1000})
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, d := range docs {
		if p, _ := d.Data["period"].(string); p != "" {
			out[p] = true
		}
	}
	return out, nil
}

// FrozenThrough returns the latest closed month ("" when none is closed).
// Everything up to and including it is frozen: months are closed in order, so
// a closed month can never gain a late entry, and the balances saved with it
// stay true.
func (s *Service) FrozenThrough(ctx context.Context) (string, error) {
	closed, err := s.ClosedPeriods(ctx)
	if err != nil {
		return "", err
	}
	latest := ""
	for p := range closed {
		if p > latest {
			latest = p
		}
	}
	return latest, nil
}

// checkOpen returns ErrPeriodClosed when date falls in a closed month, or in
// any month before one. The ledger of a closed month is frozen so that its
// saved balances stay true; a correction goes in the open month as a reversal
// or an adjustment.
func (s *Service) checkOpen(ctx context.Context, date string) error {
	if len(date) < 7 {
		return fmt.Errorf("%w: %q", ErrBadDate, date)
	}
	frozen, err := s.FrozenThrough(ctx)
	if err != nil {
		return err
	}
	if frozen != "" && date[:7] <= frozen {
		return fmt.Errorf("%w (%s)", ErrPeriodClosed, date[:7])
	}
	return nil
}

// entryAppends builds the records of an entry: the entry itself first, then
// its movements. They always travel together in one Batch.
func entryAppends(e Entry) []Append {
	head := encode(e)
	delete(head, "movements")
	head["period"] = e.Period()
	out := []Append{{Entity: EntEntry, Data: head}}
	for _, m := range e.Movements {
		mv := map[string]any{
			"entry_uid":    e.UID,
			"date":         e.Date,
			"period":       e.Period(),
			"account_code": m.AccountCode,
			"side":         string(m.Side),
			"amount":       int64(m.Amount),
			"debit_cents":  int64(0),
			"credit_cents": int64(0),
			"source":       string(e.Source),
		}
		if m.Side == Debit {
			mv["debit_cents"] = int64(m.Amount)
		} else {
			mv["credit_cents"] = int64(m.Amount)
		}
		out = append(out, Append{Entity: EntMovement, Data: mv})
	}
	return out
}

// prepare validates e against the chart and the closed months, and stamps it.
func (s *Service) prepare(ctx context.Context, chart *Chart, e Entry, actor int64) (Entry, error) {
	if e.Source == "" {
		e.Source = SourceManual
	}
	e.UID = newUID()
	e.CreatedBy = actor
	e.CreatedAt = s.stamp()
	if err := e.Validate(chart); err != nil {
		return Entry{}, err
	}
	if err := s.checkOpen(ctx, e.Date); err != nil {
		return Entry{}, err
	}
	return e, nil
}

// PostEntry validates and stores an entry that changes nothing else (a manual
// movement, a reversal). The entry and all its movements land atomically.
func (s *Service) PostEntry(ctx context.Context, e Entry, actor int64) (Entry, error) {
	chart, err := s.Chart(ctx)
	if err != nil {
		return Entry{}, err
	}
	e, err = s.prepare(ctx, chart, e, actor)
	if err != nil {
		return Entry{}, err
	}
	head, err := s.headID(ctx)
	if err != nil {
		return Entry{}, err
	}
	ids, err := s.store.Commit(ctx, Batch{
		Update:  &Doc{Entity: EntHead, ID: head, Data: map[string]any{"touched_at": s.stamp()}},
		Appends: entryAppends(e),
	})
	if err != nil {
		return Entry{}, err
	}
	e.ID = ids[0]
	return e, nil
}

// movementsOf loads the movements of the entries with the given uids.
func (s *Service) movementsByUID(ctx context.Context, conds []Cond) (map[string][]Movement, error) {
	docs, err := s.find(ctx, Query{Entity: EntMovement, Where: conds, Limit: 20000})
	if err != nil {
		return nil, err
	}
	out := map[string][]Movement{}
	for _, d := range docs {
		uid, _ := d.Data["entry_uid"].(string)
		side, _ := d.Data["side"].(string)
		code, _ := d.Data["account_code"].(string)
		out[uid] = append(out[uid], Movement{AccountCode: code, Side: Side(side), Amount: toCents(d.Data["amount"])})
	}
	return out, nil
}

// GetEntry returns one entry with its movements.
func (s *Service) GetEntry(ctx context.Context, id int64) (Entry, error) {
	d, err := s.store.Get(ctx, EntEntry, id)
	if err != nil {
		return Entry{}, err
	}
	e, err := decode[Entry](d)
	if err != nil {
		return Entry{}, err
	}
	mv, err := s.movementsByUID(ctx, []Cond{eq("entry_uid", e.UID)})
	if err != nil {
		return Entry{}, err
	}
	e.Movements = mv[e.UID]
	return e, nil
}

// EntryFilter narrows ListEntries.
type EntryFilter struct {
	From, To string // inclusive dates; "" is open
	Limit    int
}

func dateConds(field, from, to string) []Cond {
	var c []Cond
	if from != "" {
		c = append(c, Cond{Field: field, Op: ">=", Value: from})
	}
	if to != "" {
		c = append(c, Cond{Field: field, Op: "<=", Value: to})
	}
	return c
}

// ListEntries returns entries, newest first, with their movements.
func (s *Service) ListEntries(ctx context.Context, f EntryFilter) ([]Entry, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	docs, err := s.find(ctx, Query{Entity: EntEntry, Where: dateConds("date", f.From, f.To), OrderBy: "date", Desc: true, Limit: limit})
	if err != nil {
		return nil, err
	}
	entries, err := decodeAll[Entry](docs)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return entries, nil
	}
	lo, hi := entries[0].Date, entries[0].Date
	for _, e := range entries {
		if e.Date < lo {
			lo = e.Date
		}
		if e.Date > hi {
			hi = e.Date
		}
	}
	mv, err := s.movementsByUID(ctx, dateConds("date", lo, hi))
	if err != nil {
		return nil, err
	}
	for i := range entries {
		entries[i].Movements = mv[entries[i].UID]
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Date != entries[j].Date {
			return entries[i].Date > entries[j].Date
		}
		return entries[i].ID > entries[j].ID
	})
	return entries, nil
}

// ReverseEntry posts the contra-entry of an existing entry (dated date, today
// if empty) -- the only way to correct the ledger, which is append-only.
func (s *Service) ReverseEntry(ctx context.Context, id int64, date string, actor int64) (Entry, error) {
	orig, err := s.GetEntry(ctx, id)
	if err != nil {
		return Entry{}, err
	}
	if orig.Source == SourceReversal {
		return Entry{}, fmt.Errorf("%w: it is itself a reversal", ErrNotReversible)
	}
	ref := fmt.Sprintf("entry:%d", id)
	prior, err := s.find(ctx, Query{Entity: EntEntry, Where: []Cond{eq("reference", ref), eq("source", string(SourceReversal))}, Limit: 1})
	if err != nil {
		return Entry{}, err
	}
	if len(prior) > 0 {
		return Entry{}, ErrAlreadyReversed
	}
	if date == "" {
		date = s.today()
	}
	return s.PostEntry(ctx, orig.Reversal(date, actor), actor)
}

// Posted returns the debit and credit totals posted to each account up to and
// including asOf ("" means no limit).
func (s *Service) Posted(ctx context.Context, asOf string) (map[string]Totals, error) {
	var where []Cond
	if asOf != "" {
		where = []Cond{{Field: "date", Op: "<=", Value: asOf}}
	}
	rows, err := s.store.Sum(ctx, EntMovement, "account_code", []string{"debit_cents", "credit_cents"}, where)
	if err != nil {
		return nil, err
	}
	out := map[string]Totals{}
	for _, r := range rows {
		code, _ := r["account_code"].(string)
		out[code] = Totals{Debit: toCents(r["debit_cents"]), Credit: toCents(r["credit_cents"])}
	}
	return out, nil
}

// Balances returns the natural-sign balance of every account in the chart as
// of the given date, higher-level accounts rolled up from their children.
func (s *Service) Balances(ctx context.Context, asOf string) (*Chart, map[string]Cents, error) {
	chart, err := s.Chart(ctx)
	if err != nil {
		return nil, nil, err
	}
	posted, err := s.Posted(ctx, asOf)
	if err != nil {
		return nil, nil, err
	}
	return chart, chart.Balances(posted), nil
}

// LedgerLine is one line of an account's book (libro de movimientos).
type LedgerLine struct {
	EntryID     int64  `json:"entry_id"`
	EntryUID    string `json:"entry_uid"`
	Date        string `json:"date"`
	Description string `json:"description"`
	Source      Source `json:"source"`
	Side        Side   `json:"side"`
	Amount      Cents  `json:"amount"`
	Balance     Cents  `json:"balance"` // running natural-sign balance after this line
}

// Ledger is an account's book for a date range.
type Ledger struct {
	Account Account      `json:"account"`
	From    string       `json:"from"`
	To      string       `json:"to"`
	Opening Cents        `json:"opening"`
	Lines   []LedgerLine `json:"lines"`
	Closing Cents        `json:"closing"`
}

// Ledger returns the book of one posting account between from and to
// (inclusive, "" is open) with a running balance.
func (s *Service) Ledger(ctx context.Context, code, from, to string) (Ledger, error) {
	chart, err := s.Chart(ctx)
	if err != nil {
		return Ledger{}, err
	}
	acct, ok := chart.Get(code)
	if !ok {
		return Ledger{}, fmt.Errorf("%w: %q", ErrUnknownAccount, code)
	}
	if !acct.Postable {
		return Ledger{}, fmt.Errorf("%w: %s %s", ErrNotPostable, acct.Code, acct.Name)
	}
	g := acct.Group()
	out := Ledger{Account: acct, From: from, To: to}

	if from != "" {
		rows, err := s.store.Sum(ctx, EntMovement, "account_code", []string{"debit_cents", "credit_cents"},
			[]Cond{eq("account_code", code), {Field: "date", Op: "<", Value: from}})
		if err != nil {
			return Ledger{}, err
		}
		for _, r := range rows {
			out.Opening += Balance(g, toCents(r["debit_cents"]), toCents(r["credit_cents"]))
		}
	}

	where := append([]Cond{eq("account_code", code)}, dateConds("date", from, to)...)
	mdocs, err := s.find(ctx, Query{Entity: EntMovement, Where: where, OrderBy: "date", Limit: 20000})
	if err != nil {
		return Ledger{}, err
	}
	type mv struct {
		uid, date, source, side string
		amount                  Cents
		id                      int64
	}
	var mvs []mv
	lo, hi := "", ""
	for _, d := range mdocs {
		m := mv{id: d.ID, amount: toCents(d.Data["amount"])}
		m.uid, _ = d.Data["entry_uid"].(string)
		m.date, _ = d.Data["date"].(string)
		m.source, _ = d.Data["source"].(string)
		m.side, _ = d.Data["side"].(string)
		mvs = append(mvs, m)
		if lo == "" || m.date < lo {
			lo = m.date
		}
		if m.date > hi {
			hi = m.date
		}
	}
	sort.SliceStable(mvs, func(i, j int) bool {
		if mvs[i].date != mvs[j].date {
			return mvs[i].date < mvs[j].date
		}
		return mvs[i].id < mvs[j].id
	})
	byUID := map[string]Entry{}
	if len(mvs) > 0 {
		edocs, err := s.find(ctx, Query{Entity: EntEntry, Where: dateConds("date", lo, hi), Limit: 20000})
		if err != nil {
			return Ledger{}, err
		}
		entries, err := decodeAll[Entry](edocs)
		if err != nil {
			return Ledger{}, err
		}
		for _, e := range entries {
			byUID[e.UID] = e
		}
	}
	bal := out.Opening
	for _, m := range mvs {
		if Side(m.side) == Debit {
			if g.DebitNormal() {
				bal += m.amount
			} else {
				bal -= m.amount
			}
		} else {
			if g.DebitNormal() {
				bal -= m.amount
			} else {
				bal += m.amount
			}
		}
		e := byUID[m.uid]
		out.Lines = append(out.Lines, LedgerLine{
			EntryID: e.ID, EntryUID: m.uid, Date: m.date, Description: e.Description,
			Source: Source(m.source), Side: Side(m.side), Amount: m.amount, Balance: bal,
		})
	}
	out.Closing = bal
	return out, nil
}

// PostEntryWithDocument posts a manual entry together with the document that
// supports it (a receipt, a supplier invoice), kept side by side. The document
// is stored first and removed again if the entry is refused, so a rejected
// entry leaves no file behind. A nil doc is the same as PostEntry. The
// document's USD amount is the entry's own total (the books' amount), and its
// date defaults to the entry's; its foreign-currency data stays passive (D-33).
func (s *Service) PostEntryWithDocument(ctx context.Context, e Entry, doc *DocumentInput, actor int64) (Entry, error) {
	if doc == nil {
		return s.PostEntry(ctx, e, actor)
	}
	chart, err := s.Chart(ctx)
	if err != nil {
		return Entry{}, err
	}
	// Refuse a bad entry before taking the file in.
	if err := e.Validate(chart); err != nil {
		return Entry{}, err
	}
	if err := s.checkOpen(ctx, e.Date); err != nil {
		return Entry{}, err
	}
	in := *doc
	in.ForEntry = true
	debit, _ := e.Totals()
	in.AmountUSD = debit
	if in.Date == "" {
		in.Date = e.Date
	}
	saved, err := s.SaveDocument(ctx, in, actor)
	if err != nil {
		return Entry{}, err
	}
	e.DocID = saved.ID
	posted, err := s.PostEntry(ctx, e, actor)
	if err != nil {
		s.discardDocument(ctx, saved)
		return Entry{}, err
	}
	return posted, nil
}
