package accounting

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ha1tch/seam-ui/internal/oqlguard"
)

// OQL formulas
//
// A rule of method "oql" holds one to MaxOQLQueries queries. They run in order,
// each against the store the module itself uses; the last must produce a
// single amount, and that amount, in USD cents like every figure the module
// stores, is the rule's figure. Earlier queries are for working things out:
// each one's single value is available to the next as :prev and, by position,
// as :q1, :q2, ...
//
// Because the text is written by a person and run against a database that can
// also change data, it is held to a short list of rules, checked when the rule
// is saved and again, as part of building the query, every time it runs:
//
//   - it is a single SELECT: no ";", no comments, none of INSERT, UPDATE,
//     DELETE, DROP, CREATE, ALTER, TRUNCATE, EXEC, no sequence calls (@ or
//     NEXT VALUE) that change state as a side effect of reading;
//   - every table it reads (after FROM or JOIN) is on a short list of this
//     module's business tables (OQLTables): not users, passwords or other
//     modules' data, and not the module's own people and files (AE payout
//     details, documents, company data); tables are written plainly, never
//     quoted or bracketed, and FROM lists no commas (use JOIN), so every table
//     can be seen and checked;
//   - values reach it only as named parameters, written :name, which are filled
//     in as escaped literals, never by pasting text, and only outside quotes.

// MaxOQLQueries and the other limits of an OQL formula.
const (
	MaxOQLQueries    = 5
	MaxOQLQueryChars = 2000
	oqlTimeout       = 5 * time.Second
)

// QueryRunner runs one OQL query and returns its rows.
type QueryRunner interface {
	Query(ctx context.Context, oql string) ([]map[string]any, error)
}

// WithQueryRunner sets the runner OQL formulas use. A store that can run OQL
// (XoluStore) is picked up automatically.
func WithQueryRunner(r QueryRunner) Option { return func(s *Service) { s.runner = r } }

// WithoutOQLFormulas switches the OQL method off: rules cannot be saved or
// enabled with it, and a stored one is not run.
func WithoutOQLFormulas() Option { return func(s *Service) { s.noOQL = true } }

// OQLFormulasEnabled reports whether the OQL method may be used.
func (s *Service) OQLFormulasEnabled() bool { return !s.noOQL }

// oqlParams are the names a query may use, per situation. q1..qN and prev are
// added by position.
func oqlParams(sit Situation) map[string]bool {
	p := map[string]bool{"amount": true, "running": true, "country": true, "concept": true}
	if sit == SituationCommission {
		for _, n := range []string{"period", "tenant_id", "ae_id", "ae_kind", "item_id"} {
			p[n] = true
		}
	}
	return p
}

// OQLParamNames lists the parameter names available to a situation, for the
// panel's help text.
func OQLParamNames(sit Situation) []string {
	names := []string{"amount", "running", "country", "concept"}
	if sit == SituationCommission {
		names = append(names, "period", "tenant_id", "ae_id", "ae_kind", "item_id")
	}
	return names
}

// OQLTables are the tables an OQL formula may read.
var OQLTables = []string{EntItem, EntTenant, EntCommission, EntSettlement, EntClose, EntEntry, EntMovement, EntAccount, EntClaim}

func oqlTableAllowed(name string) bool {
	for _, t := range OQLTables {
		if strings.EqualFold(t, name) {
			return true
		}
	}
	return false
}

// The checks that are not specific to accounting live in internal/oqlguard,
// which the form engine's query-picker shares.
var (
	oqlParamRE     = oqlguard.ParamRE
	oqlQueryRefRE  = regexp.MustCompile(`^q([0-9]+)$`)
	oqlForbidRE    = oqlguard.ForbidRE
	oqlFromRE      = oqlguard.FromRE
	oqlFromParenRE = oqlguard.FromParenRE
	oqlSelectRE    = oqlguard.SelectRE
)

func fromListHasComma(m string, i int) bool { return oqlguard.FromListHasComma(m, i) }

func maskStrings(q string) (string, bool) { return oqlguard.MaskStrings(q) }

// ValidateOQLFormula checks the queries of an OQL rule without running them.
func ValidateOQLFormula(sit Situation, queries []string) error {
	if len(queries) == 0 {
		return problem("oql_empty", "")
	}
	if len(queries) > MaxOQLQueries {
		return problem("oql_count", "")
	}
	allowed := oqlParams(sit)
	for i, q := range queries {
		if len(q) > MaxOQLQueryChars {
			return problem("oql_length", "")
		}
		m, closed := maskStrings(q)
		if !closed || strings.ContainsAny(m, ";") || strings.Contains(m, "--") || strings.Contains(m, "/*") {
			return problem("oql_syntax", "")
		}
		if !oqlSelectRE.MatchString(strings.TrimSpace(m)) {
			return problem("oql_select", "")
		}
		if strings.Contains(m, "@") || oqlForbidRE.MatchString(m) {
			return problem("oql_forbidden", "")
		}
		if strings.ContainsAny(m, "[]\"`") {
			return problem("oql_syntax", "")
		}
		for _, loc := range oqlFromParenRE.FindAllStringIndex(m, -1) {
			if fromListHasComma(m, loc[1]-1) { // FROM (subquery) t, other
				return problem("oql_table", ",")
			}
		}
		for _, loc := range oqlFromRE.FindAllStringSubmatchIndex(m, -1) {
			if name := m[loc[2]:loc[3]]; !oqlTableAllowed(name) {
				return problem("oql_table", name)
			}
			if fromListHasComma(m, loc[3]) {
				return problem("oql_table", ",")
			}
		}
		for _, p := range oqlParamRE.FindAllStringSubmatch(m, -1) {
			name := p[1]
			if allowed[name] {
				continue
			}
			if name == "prev" && i > 0 {
				continue
			}
			if r := oqlQueryRefRE.FindStringSubmatch(name); r != nil {
				if n, _ := strconv.Atoi(r[1]); n >= 1 && n <= i {
					continue
				}
			}
			return problem("oql_param", name)
		}
	}
	return nil
}

// oqlLiteralFor renders a parameter value as an OQL literal: numbers from their
// Go value, text quoted with quotes doubled.
func oqlLiteralFor(v any) string {
	switch t := v.(type) {
	case int64:
		return strconv.FormatInt(t, 10)
	case Cents:
		return strconv.FormatInt(int64(t), 10)
	case int:
		return strconv.Itoa(t)
	case string:
		return "'" + strings.ReplaceAll(t, "'", "''") + "'"
	}
	return "NULL"
}

// bindOQL replaces the :name parameters of q, outside string literals.
func bindOQL(q string, params map[string]any) (string, error) {
	masked, _ := maskStrings(q)
	var b strings.Builder
	last := 0
	for _, loc := range oqlParamRE.FindAllStringSubmatchIndex(masked, -1) {
		name := masked[loc[2]:loc[3]]
		v, ok := params[name]
		if !ok {
			return "", fmt.Errorf("unknown parameter :%s", name)
		}
		b.WriteString(q[last:loc[0]])
		b.WriteString(oqlLiteralFor(v))
		last = loc[1]
	}
	b.WriteString(q[last:])
	return b.String(), nil
}

// systemColumns are returned by the store next to the asked-for columns and
// are not part of an answer.
var systemColumns = map[string]bool{"id": true, "_version": true, "created_at": true, "updated_at": true, "tenant_id": true}

// scalar reduces a result to one value: no rows is nothing (zero), more than
// one row or column is an error.
func scalar(rows []map[string]any) (v any, none bool, err error) {
	if len(rows) == 0 {
		return nil, true, nil
	}
	if len(rows) > 1 {
		return nil, false, fmt.Errorf("the query returned %d rows; it must return one", len(rows))
	}
	var keys []string
	for k := range rows[0] {
		if !systemColumns[k] {
			keys = append(keys, k)
		}
	}
	if len(keys) != 1 {
		return nil, false, fmt.Errorf("the query returned %d columns; it must return one", len(keys))
	}
	return rows[0][keys[0]], false, nil
}

// asCents reads an amount, in cents, from what the store returned.
func asCents(v any) (Cents, error) {
	switch n := v.(type) {
	case nil:
		return 0, nil
	case float64:
		if math.IsNaN(n) || math.IsInf(n, 0) || math.Abs(n) > float64(maxFormulaCents) {
			return 0, fmt.Errorf("the amount is out of range")
		}
		return Cents(math.Round(n)), nil
	case int64:
		return Cents(n), nil
	case int:
		return Cents(n), nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		if err != nil {
			// The text itself is not echoed: it came from the database and
			// the message is shown to whoever edits formulas.
			return 0, fmt.Errorf("a text value is not an amount")
		}
		return asCents(f)
	}
	return 0, fmt.Errorf("a value of type %T is not an amount", v)
}

// runOQLFormula runs the queries of f and returns its figure.
func (s *Service) runOQLFormula(ctx context.Context, sit Situation, f Formula, in FormulaInput, running Cents) (Cents, string, error) {
	if s.noOQL {
		return 0, "", ErrOQLDisabled
	}
	if s.runner == nil {
		return 0, "", ErrNoOQL
	}
	// Saved rules were validated, but stored data can be edited by other
	// means; the same checks run again before anything is sent.
	if err := ValidateOQLFormula(sit, f.Queries); err != nil {
		return 0, "", err
	}
	params := map[string]any{
		"amount": in.Amount, "running": running, "country": in.Country, "concept": string(in.Concept),
		"period": in.Period, "tenant_id": in.TenantID, "ae_id": in.AEID, "ae_kind": in.AEKind, "item_id": in.ItemID,
	}
	var last Cents
	note := fmt.Sprintf("%d OQL", len(f.Queries))
	for i, q := range f.Queries {
		bound, err := bindOQL(q, params)
		if err != nil {
			return 0, "", err
		}
		qctx, cancel := context.WithTimeout(ctx, oqlTimeout)
		rows, err := s.runner.Query(qctx, bound)
		cancel()
		if err != nil {
			return 0, "", fmt.Errorf("query %d: %w", i+1, err)
		}
		v, none, err := scalar(rows)
		if err != nil {
			return 0, "", fmt.Errorf("query %d: %w", i+1, err)
		}
		amt, err := asCents(v)
		if err != nil {
			return 0, "", fmt.Errorf("query %d: %w", i+1, err)
		}
		if none && i == len(f.Queries)-1 {
			note += ", no rows = 0"
		}
		last = amt
		params["prev"] = amt
		params["q"+strconv.Itoa(i+1)] = amt
	}
	if last < 0 {
		return 0, "", fmt.Errorf("the result is negative (%d); use the discount effect instead", last)
	}
	if last > maxFormulaCents {
		return 0, "", fmt.Errorf("the result is out of range")
	}
	return last, note, nil
}
