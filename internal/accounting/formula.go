package accounting

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Formulas
//
// A formula rule turns a reference amount into a new one: a percentage of it, a
// fixed amount, or whatever a short sequence of OQL queries returns. Rules are
// grouped by the situation they serve (the commission of a collected item, the
// price of an item in a market) and are evaluated in order over a running
// amount, so that a base rule and any number of discounts and surcharges can
// follow each other. The panel at /accounting/formulas manages them; the
// schema stores only the rules and, as before, only the results of using them.

// Situation says where a set of rules is used.
type Situation string

const (
	// SituationCommission: what an AE earns on a collected item. The running
	// amount starts at zero and the reference is what the customer paid.
	SituationCommission Situation = "commission"
	// SituationPrice: the price of an item in a market. The running amount
	// starts at the base (master list) price, which is also the reference.
	SituationPrice Situation = "price"
)

// Situations lists them in display order.
func Situations() []Situation { return []Situation{SituationCommission, SituationPrice} }

// Valid reports whether s is a known situation.
func (s Situation) Valid() bool { return s == SituationCommission || s == SituationPrice }

// FormulaEffect is what a rule does to the running amount.
type FormulaEffect string

const (
	EffectSet       FormulaEffect = "set"       // replace it
	EffectSurcharge FormulaEffect = "surcharge" // add to it
	EffectDiscount  FormulaEffect = "discount"  // subtract from it (never below zero)
)

// Valid reports whether e is a known effect.
func (e FormulaEffect) Valid() bool {
	return e == EffectSet || e == EffectSurcharge || e == EffectDiscount
}

// FormulaMethod is how a rule gets its figure.
type FormulaMethod string

const (
	MethodPercent FormulaMethod = "percent" // a percentage
	MethodFixed   FormulaMethod = "fixed"   // a fixed, ad-hoc amount
	MethodOQL     FormulaMethod = "oql"     // one or more OQL queries ending in one amount
)

// Valid reports whether m is a known method.
func (m FormulaMethod) Valid() bool {
	return m == MethodPercent || m == MethodFixed || m == MethodOQL
}

// Limits of a rule.
const (
	MaxFormulasPerSituation       = 100
	maxFormulaName                = 80
	maxPercentBPS                 = 100000            // 1000%
	maxFormulaCents         Cents = 1_000_000_000_000 // USD 10 billion: a guard against nonsense, not a business limit
)

// Formula is one rule.
type Formula struct {
	ID        int64         `json:"id,omitempty"`
	Situation Situation     `json:"situation"`
	Name      string        `json:"name"`
	Position  int           `json:"position"` // evaluation order, ascending
	Enabled   bool          `json:"enabled"`
	Effect    FormulaEffect `json:"effect"`
	Method    FormulaMethod `json:"method"`

	BPS     int      `json:"bps,omitempty"`     // percent: basis points, 100 = 1%
	Amount  Cents    `json:"amount,omitempty"`  // fixed: USD cents
	Queries []string `json:"queries,omitempty"` // oql: run in order; the last one gives the amount

	// Conditions; empty means "any". All that are set must match.
	AEKind  string  `json:"ae_kind,omitempty"` // "real", "seam_ae" or "seam_web" (commission only)
	Country string  `json:"country,omitempty"` // two letters
	Concept Concept `json:"concept,omitempty"`

	Stop bool `json:"stop,omitempty"` // when the rule applies, no later rule is evaluated

	UpdatedAt string `json:"updated_at,omitempty"`
	UpdatedBy int64  `json:"updated_by,omitempty"`
}

// Errors of formulas. Test with errors.Is.
var (
	ErrBadFormula    = errors.New("accounting: invalid formula")
	ErrFormulaFailed = errors.New("accounting: a formula could not be evaluated")
	ErrNoOQL         = errors.New("accounting: the store cannot run OQL queries")
	ErrOQLDisabled   = errors.New("accounting: OQL formulas are switched off")
)

// FormulaProblem is a validation failure of a rule. Code is stable and is what
// the interface turns into a message.
type FormulaProblem struct {
	Code   string
	Detail string
}

func (p *FormulaProblem) Error() string {
	if p.Detail == "" {
		return "accounting: invalid formula: " + p.Code
	}
	return "accounting: invalid formula: " + p.Code + ": " + p.Detail
}

// Is makes errors.Is(err, ErrBadFormula) true for every problem.
func (p *FormulaProblem) Is(target error) bool { return target == ErrBadFormula }

func problem(code, detail string) error { return &FormulaProblem{Code: code, Detail: detail} }

// FormulaInput is what a rule set is evaluated for.
type FormulaInput struct {
	Amount  Cents // the reference: what the customer paid, or the base price
	Country string
	Concept Concept

	// Commission only.
	AEKind   string // see AEKey
	AEID     int64
	TenantID int64
	ItemID   int64
	Period   string
}

// AEKey is the value a rule's AEKind condition is compared with: "real" for a
// registered AE, the slug for a virtual one.
func AEKey(ae AE) string {
	if ae.Kind == AEVirtual {
		return ae.Slug
	}
	return string(AEReal)
}

// FormulaStep records what one rule did.
type FormulaStep struct {
	Formula Formula
	Before  Cents
	After   Cents
	Note    string // what produced the figure, e.g. "10%" or "2 OQL queries"
}

// FormulaResult is the outcome of evaluating a rule set.
type FormulaResult struct {
	Amount Cents
	Steps  []FormulaStep
}

// Detail is a short text for the commission line: the rules that applied.
func (r FormulaResult) Detail() string {
	names := make([]string, 0, len(r.Steps))
	for _, s := range r.Steps {
		names = append(names, s.Formula.Name)
	}
	d := strings.Join(names, " → ")
	if len(d) > 200 {
		d = d[:197] + "..."
	}
	return d
}

// Figure is the rule's value as text: "12.5%" or "USD 5.00"; an OQL rule has no
// single figure and gives "".
func (f Formula) Figure() string {
	switch f.Method {
	case MethodPercent:
		return bpsLabel(f.BPS)
	case MethodFixed:
		return money(f.Amount)
	}
	return ""
}

// matches reports whether a rule's conditions hold for in.
func (f Formula) matches(sit Situation, in FormulaInput) bool {
	if f.AEKind != "" && sit == SituationCommission && f.AEKind != in.AEKind {
		return false
	}
	if f.Country != "" && !strings.EqualFold(f.Country, in.Country) {
		return false
	}
	if f.Concept != "" && f.Concept != in.Concept {
		return false
	}
	return true
}

func startAmount(sit Situation, in FormulaInput) Cents {
	if sit == SituationPrice {
		return in.Amount
	}
	return 0
}

// percentOf is amount * bps / 10000, rounded half up to the cent.
func percentOf(amount Cents, bps int) Cents {
	return (amount*Cents(bps) + 5000) / 10000
}

// EvaluateFormulas runs rules (already in order, enabled ones only are used)
// over in. A rule that cannot be evaluated stops the whole evaluation with
// ErrFormulaFailed: guessing would book money that may not be owed.
func (s *Service) EvaluateFormulas(ctx context.Context, sit Situation, rules []Formula, in FormulaInput) (FormulaResult, error) {
	running := startAmount(sit, in)
	res := FormulaResult{}
	for _, f := range rules {
		if !f.Enabled || !f.matches(sit, in) {
			continue
		}
		before := running
		var fig Cents
		var note string
		switch f.Method {
		case MethodPercent:
			basis := running
			if f.Effect == EffectSet {
				basis = in.Amount
			}
			fig, note = percentOf(basis, f.BPS), bpsLabel(f.BPS)
		case MethodFixed:
			fig, note = f.Amount, money(f.Amount)
		case MethodOQL:
			var err error
			fig, note, err = s.runOQLFormula(ctx, sit, f, in, running)
			if err != nil {
				return res, fmt.Errorf("%w: %q: %w", ErrFormulaFailed, f.Name, err)
			}
		default:
			return res, fmt.Errorf("%w: %q: unknown method %q", ErrFormulaFailed, f.Name, f.Method)
		}
		switch f.Effect {
		case EffectSet:
			running = fig
		case EffectSurcharge:
			running += fig
		case EffectDiscount:
			running -= fig
		}
		if running < 0 {
			running = 0
		}
		if running > maxFormulaCents {
			return res, fmt.Errorf("%w: %q: the result is out of range", ErrFormulaFailed, f.Name)
		}
		res.Steps = append(res.Steps, FormulaStep{Formula: f, Before: before, After: running, Note: note})
		if f.Stop {
			break
		}
	}
	res.Amount = running
	return res, nil
}

func money(c Cents) string { return "USD " + c.Display() }

// AdjustPrice applies the price rules to a base price for a market. With no
// price rules the base price is returned unchanged.
func (s *Service) AdjustPrice(ctx context.Context, base Cents, country string, concept Concept) (FormulaResult, error) {
	rules, err := s.Formulas(ctx, SituationPrice)
	if err != nil {
		return FormulaResult{}, err
	}
	return s.EvaluateFormulas(ctx, SituationPrice, rules, FormulaInput{Amount: base, Country: country, Concept: concept})
}

// commissionResults is what MarkCollected calls: the commission rules when
// there are any enabled, otherwise the configured calculator (the legacy
// rates, or none).
func (s *Service) commissionResults(ctx context.Context, item Item, tenant Tenant, ae AE) ([]CommissionResult, error) {
	rules, err := s.Formulas(ctx, SituationCommission)
	if err != nil {
		return nil, err
	}
	active := false
	for _, r := range rules {
		if r.Enabled {
			active = true
			break
		}
	}
	if !active {
		return s.calc.Calculate(ctx, item, tenant, ae)
	}
	res, err := s.EvaluateFormulas(ctx, SituationCommission, rules, FormulaInput{
		Amount: item.Amount, Country: tenant.Country, Concept: item.Concept,
		AEKind: AEKey(ae), AEID: ae.ID, TenantID: tenant.ID, ItemID: item.ID, Period: item.Period,
	})
	if err != nil {
		return nil, err
	}
	if res.Amount <= 0 {
		return nil, nil
	}
	return []CommissionResult{{Detail: res.Detail(), Amount: res.Amount}}, nil
}

// ---------------------------------------------------------------------------
// storage and management
// ---------------------------------------------------------------------------

// Formulas returns the rules of a situation in evaluation order.
func (s *Service) Formulas(ctx context.Context, sit Situation) ([]Formula, error) {
	docs, err := s.find(ctx, Query{Entity: EntFormula, Where: []Cond{eq("situation", string(sit))}, Limit: MaxFormulasPerSituation + 50})
	if err != nil {
		return nil, err
	}
	out, err := decodeAll[Formula](docs)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Position != out[j].Position {
			return out[i].Position < out[j].Position
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// GetFormula returns one rule, or ErrNotFound.
func (s *Service) GetFormula(ctx context.Context, id int64) (Formula, error) {
	d, err := s.store.Get(ctx, EntFormula, id)
	if err != nil {
		return Formula{}, err
	}
	return decode[Formula](d)
}

// normalizeFormula trims, upper-cases and drops what the method does not use,
// then validates. The result is what gets stored.
func (s *Service) normalizeFormula(f Formula) (Formula, error) {
	f.Name = trim(f.Name)
	f.Country = strings.ToUpper(trim(f.Country))
	if !f.Situation.Valid() {
		return f, problem("situation", "")
	}
	if f.Name == "" || len(f.Name) > maxFormulaName {
		return f, problem("name", "")
	}
	if !f.Effect.Valid() {
		return f, problem("effect", "")
	}
	if !f.Method.Valid() {
		return f, problem("method", "")
	}
	switch f.Method {
	case MethodPercent:
		if f.BPS <= 0 || f.BPS > maxPercentBPS || (f.Effect == EffectDiscount && f.BPS > 10000) {
			return f, problem("percent", "")
		}
		f.Amount, f.Queries = 0, nil
	case MethodFixed:
		if f.Amount < 0 || f.Amount > maxFormulaCents || (f.Amount == 0 && f.Effect != EffectSet) {
			return f, problem("amount", "")
		}
		f.BPS, f.Queries = 0, nil
	case MethodOQL:
		if s.noOQL {
			return f, ErrOQLDisabled
		}
		qs := make([]string, 0, len(f.Queries))
		for _, q := range f.Queries {
			if q = trim(q); q != "" {
				qs = append(qs, q)
			}
		}
		if err := ValidateOQLFormula(f.Situation, qs); err != nil {
			return f, err
		}
		f.Queries, f.BPS, f.Amount = qs, 0, 0
	}
	if f.Country != "" && !countryRE.MatchString(f.Country) {
		return f, problem("country", "")
	}
	switch f.Concept {
	case "", ConceptSubscription, ConceptStartup, ConceptAddon, ConceptServices, ConceptOther:
	default:
		return f, problem("concept", "")
	}
	switch f.AEKind {
	case "":
	case string(AEReal), SlugSeamAE, SlugSeamWeb:
		if f.Situation != SituationCommission {
			return f, problem("ae_kind", "")
		}
	default:
		return f, problem("ae_kind", "")
	}
	return f, nil
}

// SaveFormula creates a rule (ID 0, appended after the others) or replaces one.
// The situation of an existing rule cannot change.
func (s *Service) SaveFormula(ctx context.Context, f Formula, actor int64) (Formula, error) {
	f, err := s.normalizeFormula(f)
	if err != nil {
		return f, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f.UpdatedAt, f.UpdatedBy = s.now().UTC().Format("2006-01-02T15:04:05Z"), actor
	if f.ID != 0 {
		old, err := s.GetFormula(ctx, f.ID)
		if err != nil {
			return f, err
		}
		f.Situation, f.Position = old.Situation, old.Position
		return f, s.store.Put(ctx, EntFormula, f.ID, encode(f), 0)
	}
	rules, err := s.Formulas(ctx, f.Situation)
	if err != nil {
		return f, err
	}
	if len(rules) >= MaxFormulasPerSituation {
		return f, problem("too_many", "")
	}
	f.Position = 10
	if n := len(rules); n > 0 {
		f.Position = rules[n-1].Position + 10
	}
	id, err := s.store.Create(ctx, EntFormula, encode(f))
	f.ID = id
	return f, err
}

// SetFormulaEnabled switches a rule on or off. Switching an OQL rule on while
// OQL formulas are off is refused.
func (s *Service) SetFormulaEnabled(ctx context.Context, id int64, enabled bool, actor int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.GetFormula(ctx, id)
	if err != nil {
		return err
	}
	if enabled && f.Method == MethodOQL && s.noOQL {
		return ErrOQLDisabled
	}
	f.Enabled = enabled
	f.UpdatedAt, f.UpdatedBy = s.now().UTC().Format("2006-01-02T15:04:05Z"), actor
	return s.store.Put(ctx, EntFormula, id, encode(f), 0)
}

// MoveFormula moves a rule one place up (dir < 0) or down (dir > 0) within its
// situation. At either end it does nothing.
func (s *Service) MoveFormula(ctx context.Context, id int64, dir int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.GetFormula(ctx, id)
	if err != nil {
		return err
	}
	rules, err := s.Formulas(ctx, f.Situation)
	if err != nil {
		return err
	}
	i := -1
	for k, r := range rules {
		if r.ID == id {
			i = k
		}
	}
	j := i + 1
	if dir < 0 {
		j = i - 1
	}
	if i < 0 || j < 0 || j >= len(rules) {
		return nil
	}
	// Renumber 10, 20, 30... with the two swapped, so equal positions heal.
	rules[i], rules[j] = rules[j], rules[i]
	for k := range rules {
		want := (k + 1) * 10
		if rules[k].Position == want {
			continue
		}
		rules[k].Position = want
		if err := s.store.Put(ctx, EntFormula, rules[k].ID, encode(rules[k]), 0); err != nil {
			return err
		}
	}
	return nil
}

// DeleteFormula removes a rule. Commission lines already booked keep their
// amounts: only the rule is gone.
func (s *Service) DeleteFormula(ctx context.Context, id int64) error {
	return s.store.Delete(ctx, EntFormula, id)
}
