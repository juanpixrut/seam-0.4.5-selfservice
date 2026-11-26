package accounting

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Group is one of the five top-level groups of the chart of accounts
// (Plan de cuentas, section 12.2). The group is the first digit of an
// account code.
type Group int

const (
	GroupAsset     Group = 1 // Activo      -- what Seam has
	GroupLiability Group = 2 // Pasivo      -- what Seam owes
	GroupEquity    Group = 3 // Patrimonio  -- what belongs to the partners
	GroupIncome    Group = 4 // Ingresos    -- what Seam earns
	GroupExpense   Group = 5 // Egresos     -- what Seam spends
)

// Name returns the group's name as it appears in the plan (Spanish, since the
// plan follows the Uruguayan convention).
func (g Group) Name() string {
	switch g {
	case GroupAsset:
		return "Activo"
	case GroupLiability:
		return "Pasivo"
	case GroupEquity:
		return "Patrimonio"
	case GroupIncome:
		return "Ingresos"
	case GroupExpense:
		return "Egresos"
	}
	return ""
}

// DebitNormal reports whether accounts of the group grow on the debit side
// (section 12.4: Activo and Egresos grow by debe; Pasivo, Patrimonio and
// Ingresos grow by haber).
func (g Group) DebitNormal() bool {
	return g == GroupAsset || g == GroupExpense
}

// Account is one account of the chart. Movements are only ever posted to
// accounts with Postable set (the last level of the plan, D-38); the balance
// of a higher-level account is the sum of the accounts below it.
type Account struct {
	ID         int64  `json:"id,omitempty"`
	Code       string `json:"code"`        // "1.1.1"
	Name       string `json:"name"`        // "Banco"
	ParentCode string `json:"parent_code"` // "1.1"; empty for the five groups
	Postable   bool   `json:"postable"`
	AEID       int64  `json:"ae_id,omitempty"` // owner, for accounts under 2.1
	Note       string `json:"note,omitempty"`
}

// Group returns the group the account belongs to, deduced from its code.
func (a Account) Group() Group { return GroupOfCode(a.Code) }

// GroupOfCode returns the group of an account code ("2.1.7" -> Liability),
// or 0 for a malformed code.
func GroupOfCode(code string) Group {
	first, _, _ := strings.Cut(code, ".")
	n, err := strconv.Atoi(first)
	if err != nil || n < 1 || n > 5 {
		return 0
	}
	return Group(n)
}

// ParentOfCode returns the code one level up ("1.1.2" -> "1.1"); empty for a
// top-level code.
func ParentOfCode(code string) string {
	i := strings.LastIndexByte(code, '.')
	if i < 0 {
		return ""
	}
	return code[:i]
}

// Well-known codes of the initial chart (section 12.3).
const (
	CodeBank        = "1.1.1"
	CodePayPal      = "1.1.2"
	CodeDLocalGo    = "1.1.3"
	CodeCashGroup   = "1.1" // Disponibilidades
	CodeAEPayables  = "2.1" // Comisiones a pagar a AE
	CodePartners    = "3.1" // Aportes y retiros de socios
	CodeServiceInc  = "4.1" // Ingresos por servicios
	CodeOtherInc    = "4.2" // Otros ingresos
	CodeAECommExp   = "5.1" // Comisiones de AE
	CodeCollectExp  = "5.2" // Gastos de cobranza
	CodeOperatingEx = "5.3" // Gastos de operación
	CodeTaxesExp    = "5.4" // Impuestos y tasas
	CodeOtherExp    = "5.5" // Otros egresos y ajustes
	CodeSeamAE      = "2.1.1"
	CodeSeamWeb     = "2.1.2"
)

// DefaultChart returns the initial chart of accounts exactly as defined in
// section 12.3 of the plan. Accounts of real AEs (2.1.n, n >= 3) are not in
// it: each is created when its AE is authorized.
func DefaultChart() []Account {
	return []Account{
		{Code: "1", Name: "Activo"},
		{Code: "1.1", Name: "Disponibilidades", ParentCode: "1"},
		{Code: CodeBank, Name: "Banco", ParentCode: "1.1", Postable: true},
		{Code: CodePayPal, Name: "PayPal", ParentCode: "1.1", Postable: true},
		{Code: CodeDLocalGo, Name: "dLocal Go", ParentCode: "1.1", Postable: true},

		{Code: "2", Name: "Pasivo"},
		{Code: CodeAEPayables, Name: "Comisiones a pagar a AE", ParentCode: "2", Note: "Una cuenta por cada AE"},
		{Code: CodeSeamAE, Name: "Seam AE", ParentCode: CodeAEPayables, Postable: true, Note: "AE virtual"},
		{Code: CodeSeamWeb, Name: "Seam Web", ParentCode: CodeAEPayables, Postable: true, Note: "AE virtual"},
		{Code: "2.2", Name: "Acreedores y proveedores", ParentCode: "2", Note: "Se incorpora más adelante"},

		{Code: "3", Name: "Patrimonio"},
		{Code: CodePartners, Name: "Aportes y retiros de socios", ParentCode: "3"},
		{Code: "3.1.1", Name: "Shareholder1", ParentCode: CodePartners, Postable: true},
		{Code: "3.1.2", Name: "Shareholder2", ParentCode: CodePartners, Postable: true, Note: "Más adelante, otros socios e inversores"},

		{Code: "4", Name: "Ingresos"},
		{Code: CodeServiceInc, Name: "Ingresos por servicios", ParentCode: "4", Postable: true, Note: "Suscripción, alta o startup, add-on y servicios"},
		{Code: CodeOtherInc, Name: "Otros ingresos", ParentCode: "4", Postable: true},

		{Code: "5", Name: "Egresos"},
		{Code: CodeAECommExp, Name: "Comisiones de AE", ParentCode: "5", Postable: true},
		{Code: CodeCollectExp, Name: "Gastos de cobranza", ParentCode: "5", Postable: true, Note: "Comisiones de pasarela y bancarias"},
		{Code: CodeOperatingEx, Name: "Gastos de operación", ParentCode: "5", Postable: true, Note: "Infraestructura y alojamiento, software, servicios profesionales y marketing"},
		{Code: CodeTaxesExp, Name: "Impuestos y tasas", ParentCode: "5", Postable: true},
		{Code: CodeOtherExp, Name: "Otros egresos y ajustes", ParentCode: "5", Postable: true},
	}
}

// Chart is an in-memory view of the chart of accounts used to validate
// entries and to roll balances up the hierarchy.
type Chart struct {
	byCode map[string]Account
}

// NewChart indexes accounts by code. It rejects duplicate or malformed codes,
// a code whose group digit is wrong for its parent, a missing parent, and a
// postable account that has children (movements may only be posted at the
// last level, D-38).
func NewChart(accounts []Account) (*Chart, error) {
	c := &Chart{byCode: make(map[string]Account, len(accounts))}
	for _, a := range accounts {
		if a.Group() == 0 {
			return nil, fmt.Errorf("accounting: malformed account code %q", a.Code)
		}
		if _, dup := c.byCode[a.Code]; dup {
			return nil, fmt.Errorf("accounting: duplicate account code %q", a.Code)
		}
		c.byCode[a.Code] = a
	}
	children := map[string]int{}
	for _, a := range c.byCode {
		if p := ParentOfCode(a.Code); p != "" {
			if a.ParentCode != p {
				return nil, fmt.Errorf("accounting: account %q has parent %q, want %q", a.Code, a.ParentCode, p)
			}
			if _, ok := c.byCode[p]; !ok {
				return nil, fmt.Errorf("accounting: account %q has no parent %q in the chart", a.Code, p)
			}
			children[p]++
		} else if a.ParentCode != "" {
			return nil, fmt.Errorf("accounting: top-level account %q must not have a parent", a.Code)
		}
	}
	for code, n := range children {
		if n > 0 && c.byCode[code].Postable {
			return nil, fmt.Errorf("accounting: account %q admits movements but has %d sub-accounts", code, n)
		}
	}
	return c, nil
}

// Get returns the account with the given code.
func (c *Chart) Get(code string) (Account, bool) {
	a, ok := c.byCode[code]
	return a, ok
}

// Accounts returns every account sorted by code in numeric order
// ("1.1.2" before "1.1.10").
func (c *Chart) Accounts() []Account {
	out := make([]Account, 0, len(c.byCode))
	for _, a := range c.byCode {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return CompareCodes(out[i].Code, out[j].Code) < 0 })
	return out
}

// Under reports whether code is the same as, or a descendant of, ancestor.
func Under(code, ancestor string) bool {
	return code == ancestor || strings.HasPrefix(code, ancestor+".")
}

// CompareCodes orders account codes segment by segment, numerically.
func CompareCodes(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		x, _ := strconv.Atoi(as[i])
		y, _ := strconv.Atoi(bs[i])
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	switch {
	case len(as) < len(bs):
		return -1
	case len(as) > len(bs):
		return 1
	}
	return 0
}

// NextAEAccountCode returns the next free 2.1.n code given the codes already
// in use (2.1.1 and 2.1.2 belong to the two virtual AEs, so the first real AE
// gets 2.1.3).
func NextAEAccountCode(existing []string) string {
	max := 2
	for _, code := range existing {
		if ParentOfCode(code) != CodeAEPayables {
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(code, CodeAEPayables+"."))
		if err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("%s.%d", CodeAEPayables, max+1)
}

// Balance converts a debit total and a credit total into the account's
// natural-sign balance (section 12.4): debit minus credit for Activo and
// Egresos, credit minus debit for the other three groups.
func Balance(g Group, debit, credit Cents) Cents {
	if g.DebitNormal() {
		return debit - credit
	}
	return credit - debit
}

// Totals are the debit and credit sums posted to one account.
type Totals struct {
	Debit  Cents `json:"debit"`
	Credit Cents `json:"credit"`
}

// Balances returns, for every account in the chart, its natural-sign balance
// given the posted totals of the postable accounts. A higher-level account's
// balance is the sum of the balances of its children (section 12.2).
//
// Adding natural-sign balances of siblings is valid because accounts in the
// same subtree always share a group, and therefore a nature.
func (c *Chart) Balances(posted map[string]Totals) map[string]Cents {
	out := make(map[string]Cents, len(c.byCode))
	for code, a := range c.byCode {
		if !a.Postable {
			continue
		}
		t := posted[code]
		out[code] = Balance(a.Group(), t.Debit, t.Credit)
	}
	// Roll up: deepest codes first so each parent sees finished children.
	codes := make([]string, 0, len(c.byCode))
	for code, a := range c.byCode {
		if !a.Postable {
			codes = append(codes, code)
		}
	}
	sort.Slice(codes, func(i, j int) bool {
		di, dj := strings.Count(codes[i], "."), strings.Count(codes[j], ".")
		if di != dj {
			return di > dj
		}
		return codes[i] < codes[j]
	})
	for _, code := range codes {
		var sum Cents
		for child, ca := range c.byCode {
			if ca.ParentCode == code {
				sum += out[child]
			}
		}
		out[code] = sum
	}
	return out
}
