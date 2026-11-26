package querypicker

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Cell is one formatted cell of a result row.
type Cell struct {
	V string `json:"v"`           // the text to show
	B string `json:"b,omitempty"` // badge variant, for a badge column
}

// Row is one result, ready for the browser.
type Row struct {
	Value string `json:"value"` // what the field stores
	Label string `json:"label"` // what the box shows once chosen
	Cells []Cell `json:"cells"` // one per column, in column order
}

// Text renders a value the store returned as plain text.
func Text(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case json.Number:
		return t.String()
	}
	return fmt.Sprint(v)
}

// Row formats one raw result row. lookup resolves a lookup column's id to its
// text; it may be nil, and a miss shows the id.
func (s *Spec) Row(raw map[string]any, lookup func(c Column, id string) (string, bool)) Row {
	show := map[string]string{}
	cells := make([]Cell, len(s.Columns))
	for i, c := range s.Columns {
		cell := Cell{V: Text(raw[c.Field])}
		if c.Lookup != "" && lookup != nil && cell.V != "" {
			if text, ok := lookup(c, cell.V); ok {
				cell.V = text
			}
		}
		switch c.Type {
		case "money":
			if cell.V != "" {
				if f, err := strconv.ParseFloat(cell.V, 64); err == nil {
					cell.V = strconv.FormatFloat(f, 'f', 2, 64)
				}
				cur := c.Currency
				if cur != "" && !currencyRE.MatchString(cur) {
					cur = Text(raw[cur])
				}
				if cur != "" {
					cell.V = cur + " " + cell.V
				}
			}
		case "date":
			if len(cell.V) >= 10 {
				cell.V = cell.V[:10]
			}
		case "datetime":
			if len(cell.V) >= 16 {
				cell.V = strings.Replace(cell.V[:16], "T", " ", 1)
			}
		case "badge":
			if cell.V != "" {
				cell.B = c.Variants[cell.V]
				if cell.B == "" {
					cell.B = "neutral"
				}
			}
		}
		cells[i] = cell
		show[c.Field] = cell.V
	}
	parts := make([]string, 0, len(s.LabelFields))
	for _, f := range s.LabelFields {
		text, ok := show[f]
		if !ok {
			text = Text(raw[f])
		}
		if text != "" {
			parts = append(parts, text)
		}
	}
	return Row{Value: Text(raw[s.ValueField]), Label: strings.Join(parts, " · "), Cells: cells}
}
