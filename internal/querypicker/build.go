package querypicker

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ha1tch/seam-ui/internal/oqlguard"
)

// Params are the values a query's named parameters take.
type Params struct {
	Q        string // the search text, as typed
	TenantID int64
	UserID   int64
}

var numberRE = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)

func (s *Spec) ready() error {
	if s.an == nil {
		return s.Validate()
	}
	return nil
}

func (s *Spec) bind(q string, p Params) (string, error) {
	return oqlguard.Bind(q, func(name string) (string, bool) {
		switch name {
		case "q":
			return oqlguard.Quote("%" + p.Q + "%"), true
		case "q_raw":
			return oqlguard.Quote(p.Q), true
		case "tenant_id":
			return strconv.FormatInt(p.TenantID, 10), true
		case "user_id":
			return strconv.FormatInt(p.UserID, 10), true
		}
		return "", false
	})
}

// withTop adds TOP n after the leading SELECT.
func withTop(q string, n int) string {
	return "SELECT TOP " + strconv.Itoa(n) + q[len("select"):]
}

// SearchQuery is the query that lists the first top rows matching p.Q.
func (s *Spec) SearchQuery(p Params, top int) (string, error) {
	if err := s.ready(); err != nil {
		return "", err
	}
	if top < 1 {
		top = 1
	}
	return s.bind(withTop(s.Query, top), p)
}

// LookupQuery is the query restricted to one value of the value column: the
// row the query would return for it, or none. It is how a stored value gets
// its label and how a submitted value is checked.
func (s *Spec) LookupQuery(p Params, value string) (string, error) {
	if err := s.ready(); err != nil {
		return "", err
	}
	lit := oqlguard.Quote(value)
	if s.ValueNumeric {
		if !numberRE.MatchString(value) {
			return "", fmt.Errorf("querypicker: %q is not a number", value)
		}
		lit = value
	}
	cond := "(" + s.an.outputs[s.ValueField] + " = " + lit + ")"
	q, w, o := s.Query, s.an.wherePos, s.an.orderPos
	var b strings.Builder
	if w >= 0 {
		b.WriteString(q[:w+len("where")])
		b.WriteString(" (")
		b.WriteString(strings.TrimSpace(q[w+len("where") : o]))
		b.WriteString(") AND ")
	} else {
		b.WriteString(q[:o])
		b.WriteString("WHERE ")
	}
	b.WriteString(cond)
	b.WriteString(" ")
	b.WriteString(q[o:])
	p.Q = "" // :q becomes '%%', which matches every row
	return s.bind(withTop(b.String(), 1), p)
}
