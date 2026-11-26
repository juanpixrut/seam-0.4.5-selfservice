package ui

import (
	"strconv"
	"strings"
)

// A line diff for the form-definition editor (campaign DEV-20): what an
// override changes against the built-in definition, shown before it is saved
// and again whenever the override is opened. It is a plain longest-common-
// subsequence diff over lines, bounded in size (the table is quadratic), with
// unchanged stretches collapsed to a separator and a few lines of context kept
// around every change. The lines are data; the page escapes them.

const (
	developerDiffMaxLines = 2000
	developerDiffContext  = 3
)

// diffLine is one line of a diff: Op is '+' (only in the new text), '-' (only in
// the old), ' ' (unchanged context) or '~' (a collapsed run of unchanged lines,
// Text then holds how many).
type diffLine struct {
	Op   byte
	Text string
}

func diffSplit(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// developerLineDiff compares old with new. It returns no lines when they are
// equal, and tooLarge when either has more than developerDiffMaxLines lines.
func developerLineDiff(oldText, newText string) (lines []diffLine, tooLarge bool) {
	a, b := diffSplit(oldText), diffSplit(newText)
	if len(a) == len(b) {
		same := true
		for i := range a {
			if a[i] != b[i] {
				same = false
				break
			}
		}
		if same { // the same lines, whatever the line endings or a final newline
			return nil, false
		}
	}
	if len(a) > developerDiffMaxLines || len(b) > developerDiffMaxLines {
		return nil, true
	}
	n, m := len(a), len(b)
	// lcs[i][j]: length of the longest common subsequence of a[i:] and b[j:]
	lcs := make([][]uint16, n+1)
	for i := range lcs {
		lcs[i] = make([]uint16, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}
	var full []diffLine
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			full = append(full, diffLine{' ', a[i]})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			full = append(full, diffLine{'-', a[i]})
			i++
		default:
			full = append(full, diffLine{'+', b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		full = append(full, diffLine{'-', a[i]})
	}
	for ; j < m; j++ {
		full = append(full, diffLine{'+', b[j]})
	}
	return collapseDiff(full), false
}

// collapseDiff keeps every change and developerDiffContext lines around it,
// replacing each longer unchanged run by one '~' line carrying its length.
func collapseDiff(full []diffLine) []diffLine {
	keep := make([]bool, len(full))
	for i, l := range full {
		if l.Op == ' ' {
			continue
		}
		for k := i - developerDiffContext; k <= i+developerDiffContext; k++ {
			if k >= 0 && k < len(full) {
				keep[k] = true
			}
		}
	}
	var out []diffLine
	skipped := 0
	flush := func() {
		if skipped > 0 {
			out = append(out, diffLine{'~', strconv.Itoa(skipped)})
			skipped = 0
		}
	}
	for i, l := range full {
		if keep[i] {
			flush()
			out = append(out, l)
		} else {
			skipped++
		}
	}
	flush()
	return out
}
