package i18n

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// This file is the language-file comparison behind the Developer module's
// sync and completeness view (campaign DEV-19). It is pure: it works on
// locale -> key -> message maps, so it can check the embedded files, an
// overlay, or what a tenant actually sees (Translator.Compare).

// LocaleDiff is how one locale differs from the reference locale. Every
// slice holds keys, sorted.
type LocaleDiff struct {
	Locale string
	// Missing: in the reference but not in this locale (it falls back to
	// the reference at runtime).
	Missing []string
	// Extra: in this locale but not in the reference.
	Extra []string
	// Untranslated: present, but word-for-word identical to the reference,
	// which usually means nobody translated it.
	Untranslated []string
	// FormatMismatch: the printf-style verbs (%s, %d, ...) differ from the
	// reference's. T passes these messages to Sprintf, so a mismatch prints
	// something like "%!d(MISSING)" or "%!(EXTRA string=...)" to users.
	FormatMismatch []string
}

// Clean reports whether the locale has no differences of any kind.
func (d LocaleDiff) Clean() bool {
	return len(d.Missing) == 0 && len(d.Extra) == 0 &&
		len(d.Untranslated) == 0 && len(d.FormatMismatch) == 0
}

// Report compares every locale with a reference.
type Report struct {
	Ref     string
	RefKeys int
	Diffs   []LocaleDiff // every locale but the reference, sorted by locale
}

// Clean reports whether every locale is clean.
func (r Report) Clean() bool {
	for _, d := range r.Diffs {
		if !d.Clean() {
			return false
		}
	}
	return true
}

// CompareOptions tunes Compare.
type CompareOptions struct {
	// IgnoreIdentical, if set, is asked about each message that equals the
	// reference's and returns true if that is fine (a brand name, say).
	IgnoreIdentical func(locale, key, message string) bool
}

// verbPattern matches one printf verb, including flags, width and
// precision. "%%" (a literal percent sign) is not a verb and is removed
// before matching.
var verbPattern = regexp.MustCompile(`%[-+# 0]*\d*(?:\.\d+)?[a-zA-Z]`)

func formatVerbs(msg string) []string {
	msg = strings.ReplaceAll(msg, "%%", "")
	verbs := verbPattern.FindAllString(msg, -1)
	sort.Strings(verbs)
	return verbs
}

func sameVerbs(a, b string) bool {
	va, vb := formatVerbs(a), formatVerbs(b)
	if len(va) != len(vb) {
		return false
	}
	for i := range va {
		if va[i] != vb[i] {
			return false
		}
	}
	return true
}

// worthTranslating reports whether a message has enough letters that being
// identical to the reference is suspicious. Numbers, symbols and very short
// strings ("OK", "%d") are the same in every language.
func worthTranslating(msg string) bool {
	letters := 0
	for _, r := range msg {
		if unicode.IsLetter(r) {
			letters++
			if letters >= 3 {
				return true
			}
		}
	}
	return false
}

// Compare compares each locale in msgs with the reference locale ref. It
// returns an error only if ref is not in msgs.
func Compare(msgs map[string]map[string]string, ref string, opts ...CompareOptions) (Report, error) {
	base, ok := msgs[ref]
	if !ok {
		return Report{}, fmt.Errorf("i18n: reference locale %q not found", ref)
	}
	var opt CompareOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	rep := Report{Ref: ref, RefKeys: len(base)}
	locales := make([]string, 0, len(msgs))
	for loc := range msgs {
		if loc != ref {
			locales = append(locales, loc)
		}
	}
	sort.Strings(locales)

	for _, loc := range locales {
		m := msgs[loc]
		d := LocaleDiff{Locale: loc}
		for k, refMsg := range base {
			got, present := m[k]
			if !present {
				d.Missing = append(d.Missing, k)
				continue
			}
			if got == refMsg && worthTranslating(got) &&
				(opt.IgnoreIdentical == nil || !opt.IgnoreIdentical(loc, k, got)) {
				d.Untranslated = append(d.Untranslated, k)
			}
			if !sameVerbs(refMsg, got) {
				d.FormatMismatch = append(d.FormatMismatch, k)
			}
		}
		for k := range m {
			if _, inRef := base[k]; !inRef {
				d.Extra = append(d.Extra, k)
			}
		}
		sort.Strings(d.Missing)
		sort.Strings(d.Extra)
		sort.Strings(d.Untranslated)
		sort.Strings(d.FormatMismatch)
		rep.Diffs = append(rep.Diffs, d)
	}
	return rep, nil
}

// Compare compares every locale as the tenant sees it (embedded base plus
// whichever overlay layers apply) with the reference locale. Tenant 0 is the
// global view.
func (t *Translator) Compare(tenant int64, ref string, opts ...CompareOptions) (Report, error) {
	t.mu.RLock()
	locales := t.localeSetLocked()
	t.mu.RUnlock()

	msgs := make(map[string]map[string]string, len(locales))
	for loc := range locales {
		msgs[loc] = t.Effective(tenant, loc) // takes its own lock
	}
	return Compare(msgs, ref, opts...)
}
