package accounting

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Cents is an amount of money in USD cents.
//
// Seam Admin keeps its books in USD only (DESIGN.md P3, D-32): there is no
// currency field on any accounting record and no conversion anywhere. Money
// is held as an integer number of cents so that sums, the debit = credit
// check and balances are exact; floats never enter the ledger.
type Cents int64

// ErrBadAmount is returned by ParseUSD for text that is not a USD amount.
var ErrBadAmount = errors.New("accounting: invalid amount")

// ParseUSD parses a decimal amount typed by a person: "1234", "1234.5" or
// "1234.56", optionally with a leading "-" and optionally with "," thousands
// separators in the integer part ("1,234.56"). More than two decimals is an
// error rather than a silent rounding: a ledger must never change the amount
// the operator typed.
func ParseUSD(s string) (Cents, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, ErrBadAmount
	}
	neg := false
	if s[0] == '-' {
		neg = true
		s = strings.TrimSpace(s[1:])
	}
	intPart, fracPart, hasFrac := strings.Cut(s, ".")
	if intPart == "" && !hasFrac {
		return 0, ErrBadAmount
	}
	if strings.Contains(intPart, ",") {
		// Thousands separators are only accepted in well-formed groups.
		groups := strings.Split(intPart, ",")
		if len(groups[0]) == 0 || len(groups[0]) > 3 {
			return 0, ErrBadAmount
		}
		for _, g := range groups[1:] {
			if len(g) != 3 {
				return 0, ErrBadAmount
			}
		}
		intPart = strings.Join(groups, "")
	}
	if intPart == "" {
		intPart = "0"
	}
	if !allDigits(intPart) || (hasFrac && (fracPart == "" || !allDigits(fracPart))) {
		return 0, ErrBadAmount
	}
	if len(fracPart) > 2 {
		return 0, ErrBadAmount
	}
	for len(fracPart) < 2 {
		fracPart += "0"
	}
	whole, err := strconv.ParseInt(intPart, 10, 64)
	if err != nil {
		return 0, ErrBadAmount
	}
	frac, _ := strconv.ParseInt(fracPart, 10, 64)
	const maxWhole = (1<<63 - 1) / 100
	if whole > maxWhole-1 {
		return 0, ErrBadAmount
	}
	c := Cents(whole*100 + frac)
	if neg {
		c = -c
	}
	return c, nil
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// String renders the amount as "1234.56" (no currency symbol, no grouping).
func (c Cents) String() string {
	sign := ""
	v := int64(c)
	if v < 0 {
		sign = "-"
		v = -v
	}
	return fmt.Sprintf("%s%d.%02d", sign, v/100, v%100)
}

// Display renders the amount with thousands separators: "1,234.56".
func (c Cents) Display() string {
	s := c.String()
	sign := ""
	if strings.HasPrefix(s, "-") {
		sign, s = "-", s[1:]
	}
	whole, frac, _ := strings.Cut(s, ".")
	var out []byte
	for i := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, whole[i])
	}
	return sign + string(out) + "." + frac
}
