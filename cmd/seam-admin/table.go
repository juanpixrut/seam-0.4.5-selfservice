package main

import (
	"fmt"
	"io"
	"strings"
	"unicode"
)

// A table for the terminal whose columns line up whatever the text: every
// cell is measured by the screen columns it takes, not by its bytes or its
// runes. A Japanese name takes two columns per character, an accent none of
// its own. Nothing is cut: a column is as wide as its widest cell, so an
// email can always be copied whole into the next command. The same rule the
// shell scripts follow through scripts/lib/ui.py.

// displayWidth returns the number of terminal columns s takes.
func displayWidth(s string) int {
	n := 0
	for _, r := range s {
		switch {
		case r == 0 || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Cf, r) || unicode.IsControl(r):
			// combining marks, format characters and controls take no column
		case isWide(r):
			n += 2
		default:
			n++
		}
	}
	return n
}

// isWide reports whether r is drawn two columns wide: the East Asian Wide and
// Fullwidth ranges of Unicode (UAX #11), which cover CJK, kana, hangul and the
// fullwidth forms.
func isWide(r rune) bool {
	return (r >= 0x1100 && r <= 0x115F) || // hangul jamo
		(r >= 0x2E80 && r <= 0x303E) || // CJK radicals, punctuation
		(r >= 0x3041 && r <= 0x33FF) || // kana, CJK symbols
		(r >= 0x3400 && r <= 0x4DBF) || // CJK extension A
		(r >= 0x4E00 && r <= 0x9FFF) || // CJK unified ideographs
		(r >= 0xA000 && r <= 0xA4CF) || // Yi
		(r >= 0xAC00 && r <= 0xD7A3) || // hangul syllables
		(r >= 0xF900 && r <= 0xFAFF) || // CJK compatibility ideographs
		(r >= 0xFE30 && r <= 0xFE4F) || // CJK compatibility forms
		(r >= 0xFF00 && r <= 0xFF60) || // fullwidth forms
		(r >= 0xFFE0 && r <= 0xFFE6) ||
		(r >= 0x1F300 && r <= 0x1F64F) || // pictographs
		(r >= 0x20000 && r <= 0x3FFFD) // CJK extensions B and later
}

// padRight returns s followed by spaces up to width columns.
func padRight(s string, width int) string {
	if gap := width - displayWidth(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

// tableLines returns the header, a rule and the rows, every line the same
// display width.
func tableLines(header []string, rows [][]string) []string {
	widths := make([]int, len(header))
	for i, h := range header {
		widths[i] = displayWidth(h)
	}
	for _, row := range rows {
		for i := range header {
			if i < len(row) && displayWidth(row[i]) > widths[i] {
				widths[i] = displayWidth(row[i])
			}
		}
	}
	line := func(cells []string) string {
		parts := make([]string, len(header))
		for i := range header {
			cell := ""
			if i < len(cells) {
				cell = cells[i]
			}
			parts[i] = padRight(cell, widths[i])
		}
		return strings.Join(parts, "  ")
	}
	rule := make([]string, len(header))
	for i, w := range widths {
		rule[i] = strings.Repeat("-", w)
	}
	out := []string{line(header), strings.Join(rule, "  ")}
	for _, row := range rows {
		out = append(out, line(row))
	}
	return out
}

func printTable(w io.Writer, header []string, rows [][]string) {
	for _, l := range tableLines(header, rows) {
		fmt.Fprintln(w, l)
	}
}
