// Package querypicker is the model behind the form engine's "query-picker"
// widget: a field whose options are the rows of an OQL query, shown with
// several typed columns (docs/QUERY_PICKER.md).
//
// It holds what does not depend on the form engine or on HTTP: the field's
// Spec, the rules its query must follow, the queries built from it (search,
// and lookup of one value), the sealing of a Spec into the opaque token the
// browser sends back, and the formatting of result rows.
package querypicker

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/ha1tch/seam-ui/internal/oqlguard"
)

// Limits of a query-picker field.
const (
	MaxQueryChars = 2000
	MaxColumns    = 12
	DefaultLimit  = 8
	MaxLimit      = 50
)

// Column is one column of the dropdown.
type Column struct {
	Field    string            `json:"field"`
	Type     string            `json:"type,omitempty"`     // text (default), number, money, date, datetime, bool, badge, icon
	Title    string            `json:"title,omitempty"`    // literal heading, used when no translation exists
	Key      bool              `json:"key,omitempty"`      // the main column
	Muted    bool              `json:"muted,omitempty"`    // secondary colour
	Lookup   string            `json:"lookup,omitempty"`   // "<entity>.<column>": show that column of the referenced record
	Currency string            `json:"currency,omitempty"` // money: an ISO code, or a result column holding one
	Variants map[string]string `json:"variants,omitempty"` // badge: value -> success | warning | danger | info | neutral
}

// Spec is everything a query-picker field needs to run. It is what Seal
// seals, so its JSON names are short.
type Spec struct {
	Query        string   `json:"q"`
	ValueField   string   `json:"v"`
	ValueNumeric bool     `json:"n,omitempty"` // the field's JSON type is integer or number
	LabelFields  []string `json:"l,omitempty"`
	Columns      []Column `json:"c"`
	Limit        int      `json:"t,omitempty"`

	an *analysis // set by Validate
}

var (
	identRE    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	lookupRE   = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)$`)
	currencyRE = regexp.MustCompile(`^[A-Z]{3}$`)
)

var columnTypes = map[string]bool{"text": true, "number": true, "money": true, "date": true, "datetime": true, "bool": true, "badge": true, "icon": true}

var badgeVariants = map[string]bool{"success": true, "warning": true, "danger": true, "info": true, "neutral": true}

// The entities a query-picker may read: business records a signed-in user can
// already list elsewhere. Users, credentials, settings, audit data and the
// accounting module's tables are not here, and a query naming them is refused.
var (
	tablesMu sync.RWMutex
	tables   = map[string]bool{
		"assets": true, "asset_types": true, "brands": true, "models": true,
		"locations": true, "regions": true, "cost_centers": true, "currencies": true,
		"custodians": true, "sensors": true, "alert_types": true, "alerts": true,
		"issues": true, "issue_templates": true, "work_orders": true,
		"work_order_templates": true, "wo_actions": true, "warehouse": true,
		"unit": true, "consumable": true, "consumable_types": true, "crews": true,
		"outsourced": true, "prop_definitions": true,
	}
)

// AllowTable adds an entity a query-picker may read. A module that owns an
// entity calls it at start-up.
func AllowTable(name string) {
	tablesMu.Lock()
	tables[strings.ToLower(name)] = true
	tablesMu.Unlock()
}

// AllowedTables lists the entities a query-picker may read, sorted.
func AllowedTables() []string {
	tablesMu.RLock()
	defer tablesMu.RUnlock()
	out := make([]string, 0, len(tables))
	for t := range tables {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

func tableAllowed(name string) bool {
	tablesMu.RLock()
	defer tablesMu.RUnlock()
	return tables[strings.ToLower(name)]
}

// Validate checks the Spec and its query against the rules of
// docs/QUERY_PICKER.md and fills in the defaults. It runs when a schema is
// parsed and again whenever a sealed Spec is opened.
func (s *Spec) Validate() error {
	s.Query = strings.TrimSpace(s.Query)
	an, err := analyse(s.Query)
	if err != nil {
		return err
	}
	if len(s.Columns) == 0 {
		return fmt.Errorf("x-seam-columns: at least one column is required")
	}
	if len(s.Columns) > MaxColumns {
		return fmt.Errorf("x-seam-columns: at most %d columns", MaxColumns)
	}
	keys := 0
	for i := range s.Columns {
		c := &s.Columns[i]
		if !identRE.MatchString(c.Field) {
			return fmt.Errorf("x-seam-columns[%d]: field %q is not a column name", i, c.Field)
		}
		if _, ok := an.outputs[c.Field]; !ok {
			return fmt.Errorf("x-seam-columns[%d]: the query does not return a column named %q", i, c.Field)
		}
		if c.Type == "" {
			c.Type = "text"
		}
		if !columnTypes[c.Type] {
			return fmt.Errorf("x-seam-columns[%d]: unknown type %q", i, c.Type)
		}
		if c.Key {
			keys++
		}
		if c.Lookup != "" {
			m := lookupRE.FindStringSubmatch(c.Lookup)
			if m == nil {
				return fmt.Errorf("x-seam-columns[%d]: lookup must be \"<entity>.<column>\", got %q", i, c.Lookup)
			}
			if !tableAllowed(m[1]) {
				return fmt.Errorf("x-seam-columns[%d]: lookup entity %q is not one a query-picker may read", i, m[1])
			}
		}
		if c.Currency != "" && !currencyRE.MatchString(c.Currency) {
			if _, ok := an.outputs[c.Currency]; !ok {
				return fmt.Errorf("x-seam-columns[%d]: currency %q is neither an ISO code nor a column the query returns", i, c.Currency)
			}
		}
		for v, variant := range c.Variants {
			if !badgeVariants[variant] {
				return fmt.Errorf("x-seam-columns[%d]: unknown badge variant %q for value %q", i, variant, v)
			}
		}
	}
	if keys > 1 {
		return fmt.Errorf("x-seam-columns: only one column may be the key")
	}
	if s.ValueField == "" {
		s.ValueField = "id"
	}
	if _, ok := an.outputs[s.ValueField]; !ok {
		return fmt.Errorf("x-seam-value-field: the query does not return a column named %q", s.ValueField)
	}
	if len(s.LabelFields) == 0 {
		s.LabelFields = []string{s.Columns[0].Field}
		for _, c := range s.Columns {
			if c.Key {
				s.LabelFields = []string{c.Field}
			}
		}
	}
	for _, f := range s.LabelFields {
		if _, ok := an.outputs[f]; !ok {
			return fmt.Errorf("x-seam-label-fields: the query does not return a column named %q", f)
		}
	}
	switch {
	case s.Limit == 0:
		s.Limit = DefaultLimit
	case s.Limit < 1 || s.Limit > MaxLimit:
		return fmt.Errorf("x-seam-limit: must be between 1 and %d", MaxLimit)
	}
	s.an = an
	return nil
}

// analysis is what Validate learns about the query and the builders reuse.
// Offsets are into Spec.Query.
type analysis struct {
	outputs  map[string]string // output column -> the expression that produces it
	wherePos int               // offset of the top-level WHERE, or -1
	orderPos int               // offset of the top-level ORDER BY
}

var allowedParams = map[string]bool{"q": true, "q_raw": true, "tenant_id": true, "user_id": true}

var (
	itemAliasRE = regexp.MustCompile(`(?is)^(.+?)\s+as\s+([A-Za-z_][A-Za-z0-9_]*)$`)
	itemPlainRE = regexp.MustCompile(`^(?:[A-Za-z_][A-Za-z0-9_]*\.)?([A-Za-z_][A-Za-z0-9_]*)$`)
)

// explain turns a guard problem into the sentence a schema author reads.
func explain(code, detail string) error {
	text := map[string]string{
		"length":    fmt.Sprintf("the query is longer than %d characters", MaxQueryChars),
		"syntax":    "the query must be one statement, with no \";\", no comments, closed quotes and no quoted or bracketed identifiers",
		"select":    "the query must start with SELECT",
		"forbidden": "the query may only read: %s is not allowed",
		"single":    "the query must be a single SELECT: no subquery, UNION, INTERSECT or EXCEPT",
		"group":     "GROUP BY and HAVING are not supported in a query-picker",
		"star":      "name the columns: \"*\" is not allowed in the SELECT list (xolu cannot push it down and reads the whole entity)",
		"top":       "do not write %s: the server limits the rows from x-seam-limit",
		"from":      "the query has no FROM",
		"order":     "the query needs an ORDER BY, so the rows shown are the first ones and not arbitrary ones",
		"column":    "SELECT item %q needs \"AS <name>\": only a plain column can do without one",
		"duplicate": "the query returns two columns named %q",
		"like":      "every LIKE needs its own parentheses, as in ((a LIKE :q) OR (b LIKE :q)): in xolu a LIKE followed by AND or OR without them matches nothing",
		"join":      "at most one JOIN; use a lookup column for a second reference",
		"table":     "entity %q is not one a query-picker may read",
		"param":     "unknown parameter :%s (available: :q, :q_raw, :tenant_id, :user_id)",
	}[code]
	if text == "" {
		text = code
	}
	if strings.Contains(text, "%") {
		text = fmt.Sprintf(text, detail)
	}
	return fmt.Errorf("x-seam-query: %s", text)
}

// analyse applies the query rules.
func analyse(q string) (*analysis, error) {
	if len(q) > MaxQueryChars {
		return nil, explain("length", "")
	}
	m, err := oqlguard.ReadOnlySelect(q)
	if err != nil {
		if p, ok := err.(*oqlguard.Problem); ok {
			return nil, explain(p.Code, p.Detail)
		}
		return nil, err
	}

	// One pass over the words of the masked text, tracking parentheses. Each
	// parenthesis group remembers whether it holds a LIKE and whether it holds
	// an AND or OR at its own level; group 0 is the text outside any.
	type group struct{ like, logic bool }
	groups := []group{{}}
	stack := []int{0}
	an := &analysis{wherePos: -1, orderPos: -1}
	fromPos, selects, joins := -1, 0, 0
	for i := 0; i < len(m); {
		c := m[i]
		switch {
		case c == '(':
			groups = append(groups, group{})
			stack = append(stack, len(groups)-1)
			i++
		case c == ')':
			if len(stack) == 1 {
				return nil, explain("syntax", "")
			}
			stack = stack[:len(stack)-1]
			i++
		case oqlguard.IsWordByte(c) && (i == 0 || !oqlguard.IsWordByte(m[i-1])):
			j := i
			for j < len(m) && oqlguard.IsWordByte(m[j]) {
				j++
			}
			w := strings.ToLower(m[i:j])
			top := len(stack) == 1
			g := &groups[stack[len(stack)-1]]
			switch w {
			case "select":
				selects++
			case "union", "intersect", "except":
				return nil, explain("single", "")
			case "group", "having":
				return nil, explain("group", "")
			case "top", "limit", "offset":
				return nil, explain("top", strings.ToUpper(w))
			case "join":
				joins++
			case "from":
				if top && fromPos < 0 {
					fromPos = i
				}
			case "where":
				if top && an.wherePos < 0 {
					an.wherePos = i
				}
			case "order":
				if top && an.orderPos < 0 {
					an.orderPos = i
				}
			case "like":
				g.like = true
			case "and", "or":
				g.logic = true
			}
			i = j
		default:
			i++
		}
	}
	if len(stack) != 1 {
		return nil, explain("syntax", "")
	}
	if selects != 1 {
		return nil, explain("single", "")
	}
	if fromPos < 0 {
		return nil, explain("from", "")
	}
	if an.orderPos < 0 {
		return nil, explain("order", "")
	}
	if an.wherePos > an.orderPos {
		return nil, explain("syntax", "")
	}
	for _, g := range groups {
		if g.like && g.logic {
			return nil, explain("like", "")
		}
	}
	if groups[0].like {
		return nil, explain("like", "")
	}
	if joins > 1 {
		return nil, explain("join", "")
	}

	// The SELECT list: no "*", and every item has a name.
	selEnd := len("select")
	list := m[selEnd:fromPos]
	if strings.Contains(list, "*") {
		return nil, explain("star", "")
	}
	an.outputs = map[string]string{}
	depth, start := 0, selEnd
	flush := func(end int) error {
		item := strings.TrimSpace(q[start:end])
		name, expr := "", item
		if a := itemAliasRE.FindStringSubmatch(item); a != nil {
			expr, name = strings.TrimSpace(a[1]), a[2]
		} else if p := itemPlainRE.FindStringSubmatch(item); p != nil {
			name = p[1]
		} else {
			return explain("column", item)
		}
		if _, dup := an.outputs[name]; dup {
			return explain("duplicate", name)
		}
		an.outputs[name] = expr
		return nil
	}
	for i := selEnd; i < fromPos; i++ {
		switch m[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				if err := flush(i); err != nil {
					return nil, err
				}
				start = i + 1
			}
		}
	}
	if err := flush(fromPos); err != nil {
		return nil, err
	}

	names, err := oqlguard.Tables(m)
	if err != nil {
		return nil, explain("syntax", "")
	}
	for _, n := range names {
		if !tableAllowed(n) {
			return nil, explain("table", n)
		}
	}
	for _, p := range oqlguard.ParamRE.FindAllStringSubmatch(m, -1) {
		if !allowedParams[p[1]] {
			return nil, explain("param", p[1])
		}
	}
	return an, nil
}
