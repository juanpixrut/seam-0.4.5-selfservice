// Package oqlguard holds the checks Seam applies to OQL text that a person
// wrote and the server will run: accounting's OQL formulas and the form
// engine's query-picker fields.
//
// The checks work on a masked copy of the query, in which the contents of
// string literals are blanked, so that rules about keywords, tables and
// parameters look only at code. They are deliberately textual: the aim is to
// refuse anything that is not plainly a single read-only SELECT over tables
// that can be seen and checked, not to parse OQL.
package oqlguard

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// ParamRE matches a named parameter, :name.
	ParamRE = regexp.MustCompile(`:([A-Za-z_][A-Za-z0-9_]*)`)
	// ForbidRE matches statements and calls that change data.
	ForbidRE = regexp.MustCompile(`(?i)\b(insert|update|delete|drop|create|alter|truncate|exec|execute|merge|grant|revoke)\b|\bnext\s+value\b`)
	// FromRE matches a table named after FROM or JOIN.
	FromRE = regexp.MustCompile(`(?i)\b(?:from|join)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	// FromParenRE matches a subquery after FROM or JOIN.
	FromParenRE = regexp.MustCompile(`(?i)\b(?:from|join)\s*\(`)
	// SelectRE matches a query that starts with SELECT.
	SelectRE = regexp.MustCompile(`(?i)^select\b`)

	clauseEndRE = regexp.MustCompile(`(?i)^(where|group|order|having|limit|offset|union|intersect|except|on)\b`)
)

// FromListHasComma reports whether the FROM clause that starts at i (just after
// the keyword) has a comma at its own level: an old-style join, which would let
// a second table in without a JOIN to check. Commas inside parentheses belong
// to function calls or to subqueries, whose own FROM is checked on its own.
func FromListHasComma(m string, i int) bool {
	depth := 0
	for ; i < len(m); i++ {
		c := m[i]
		switch {
		case c == '(':
			depth++
		case c == ')':
			if depth == 0 {
				return false
			}
			depth--
		case depth == 0 && c == ',':
			return true
		case depth == 0 && (i == 0 || !IsWordByte(m[i-1])) && clauseEndRE.MatchString(m[i:]):
			return false
		}
	}
	return false
}

// IsWordByte reports whether c can be part of an identifier.
func IsWordByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// MaskStrings blanks the contents of single-quoted literals (” is an escaped
// quote) so that rules about keywords and parameters look only at code. It
// reports false when a quote is left open. The result has the same length as
// q, so an offset in one is an offset in the other.
func MaskStrings(q string) (string, bool) {
	var b strings.Builder
	in := false
	for i := 0; i < len(q); i++ {
		c := q[i]
		switch {
		case c == '\'' && in && i+1 < len(q) && q[i+1] == '\'':
			b.WriteString("  ")
			i++
		case c == '\'':
			in = !in
			b.WriteByte('\'')
		case in:
			b.WriteByte(' ')
		default:
			b.WriteByte(c)
		}
	}
	return b.String(), !in
}

// Problem is a rule a query broke. Code is stable and meant for translation;
// Detail names the offending table, parameter or keyword when there is one.
type Problem struct {
	Code   string
	Detail string
}

func (p *Problem) Error() string {
	if p.Detail == "" {
		return "oql: " + p.Code
	}
	return fmt.Sprintf("oql: %s (%s)", p.Code, p.Detail)
}

// ReadOnlySelect applies the rules every guarded query shares and returns the
// masked text for the caller's own rules: quotes closed; no ";" and no
// comments; a single statement that starts with SELECT; nothing that changes
// data and no "@" sequence call; no quoted or bracketed identifiers, so every
// table can be seen and checked.
func ReadOnlySelect(q string) (string, error) {
	m, closed := MaskStrings(q)
	if !closed || strings.ContainsAny(m, ";") || strings.Contains(m, "--") || strings.Contains(m, "/*") {
		return "", &Problem{Code: "syntax"}
	}
	if !SelectRE.MatchString(strings.TrimSpace(m)) {
		return "", &Problem{Code: "select"}
	}
	if strings.Contains(m, "@") {
		return "", &Problem{Code: "forbidden", Detail: "@"}
	}
	if w := ForbidRE.FindString(m); w != "" {
		return "", &Problem{Code: "forbidden", Detail: strings.ToLower(w)}
	}
	if strings.ContainsAny(m, "[]\"`") {
		return "", &Problem{Code: "syntax"}
	}
	return m, nil
}

// Tables lists the tables a masked query reads, in order of appearance, and
// refuses a FROM list with a comma at its own level.
func Tables(m string) ([]string, error) {
	for _, loc := range FromParenRE.FindAllStringIndex(m, -1) {
		if FromListHasComma(m, loc[1]-1) { // FROM (subquery) t, other
			return nil, &Problem{Code: "table", Detail: ","}
		}
	}
	var out []string
	for _, loc := range FromRE.FindAllStringSubmatchIndex(m, -1) {
		out = append(out, m[loc[2]:loc[3]])
		if FromListHasComma(m, loc[3]) {
			return nil, &Problem{Code: "table", Detail: ","}
		}
	}
	return out, nil
}

// Quote renders text as an OQL string literal, with quotes doubled.
func Quote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// Bind replaces the :name parameters of q that stand outside string literals
// with the literal lit returns for them. A name lit does not know is an error.
func Bind(q string, lit func(name string) (string, bool)) (string, error) {
	masked, _ := MaskStrings(q)
	var b strings.Builder
	last := 0
	for _, loc := range ParamRE.FindAllStringSubmatchIndex(masked, -1) {
		name := masked[loc[2]:loc[3]]
		v, ok := lit(name)
		if !ok {
			return "", fmt.Errorf("unknown parameter :%s", name)
		}
		b.WriteString(q[last:loc[0]])
		b.WriteString(v)
		last = loc[1]
	}
	b.WriteString(q[last:])
	return b.String(), nil
}
