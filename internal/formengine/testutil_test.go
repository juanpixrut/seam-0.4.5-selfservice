package formengine

import "strings"

// extractTag returns the full enclosing HTML tag around byte offset idx
// — from the nearest preceding "<" to the nearest following ">". Use
// this instead of a fixed-size byte window (html[idx:idx+N]) when a
// test needs to check whether two attributes appear on the *same*
// element.
//
// Why this exists: minty's Element.Attributes is map[string]string
// internally (confirmed by reading builder.go/minty.go directly, not
// assumed), and Go deliberately randomizes map iteration order per
// process — so the same field, rendered twice with byte-for-byte
// identical input, can legally emit its attributes in a different
// order each time. Proved this empirically, not just from reading the
// source: a throwaway test rendering the same schema 50 times in a
// loop failed on the very first repeat. A fixed byte window assumes a
// stable attribute order that the library does not guarantee — several
// tests in this package had that exact latent bug (one had already
// surfaced as real, intermittent CI flakiness) before this helper
// replaced them. HTML attribute order itself doesn't matter to a
// browser or to form submission; it only matters to a test written as
// if it did.
func extractTag(html string, idx int) string {
	if idx < 0 || idx >= len(html) {
		return ""
	}
	start := strings.LastIndex(html[:idx], "<")
	if start == -1 {
		start = 0
	}
	end := strings.Index(html[idx:], ">")
	if end == -1 {
		return html[start:]
	}
	return html[start : idx+end+1]
}
