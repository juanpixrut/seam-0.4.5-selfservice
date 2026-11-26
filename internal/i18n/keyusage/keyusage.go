// Package keyusage finds translation keys nothing in a source tree refers to
// (campaign DEV-19). It reads source, so it is for a development checkout: a
// deployed binary has no source beside it.
//
// A key is taken to be in use when:
//
//   - it appears whole in a string of the source (Go, the form definitions,
//     the scripts), which is how t("some.key") and a definition's label key
//     show up; or
//   - a string of the source that ends with a dot is the start of it, which is
//     how keys built at run time show up ("developer.scope." + scope); or
//   - it is one of the form engine's conventions, which no source spells out:
//     "<module>.forms.<form>.…" and "field.<name>…"; or
//   - each of its parts is itself a string of the source, which is how the
//     labels of a field's values ("asset.status.active": form, field, value)
//     are built.
//
// What is left is a list of candidates, not a verdict: a key assembled in a
// way none of these rules sees is reported although it is used. That is why
// the Developer module calls them "not referenced", and nothing removes them.
package keyusage

import (
	"encoding/json"
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Scan is what a source tree says about translation keys.
type Scan struct {
	literals map[string]bool // every string found, as written
	lower    map[string]bool // the same, lower-cased, for the parts rule
	prefixes []string        // strings ending in a dot, longest first
	Files    int             // how many files were read

	root  string
	where map[string][]string // for strings shaped like a key or its start: "file:line", first few
}

// maxWhere is how many places are kept for one string.
const maxWhere = 5

var keyShaped = regexp.MustCompile(`^[a-z0-9_-]+(\.[a-z0-9_-]+)+\.?$`)

// skipDirs are never read: not source, or not this application's.
var skipDirs = map[string]bool{".git": true, "node_modules": true, "vendor": true, "data": true, "locales": true, "testdata": true}

var jsString = regexp.MustCompile(`'((?:[^'\\\n]|\\.)*)'|"((?:[^"\\\n]|\\.)*)"`)

// ScanDir reads the Go files (tests excluded), the JSON files and the scripts
// under root. The locale files themselves are not read: they define the keys.
func ScanDir(root string) (*Scan, error) {
	s := &Scan{literals: map[string]bool{}, lower: map[string]bool{}, root: root, where: map[string][]string{}}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && (skipDirs[name] || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		switch {
		case strings.HasSuffix(name, "_test.go"):
			return nil
		case strings.HasSuffix(name, ".go"):
			return s.readGo(path)
		case strings.HasSuffix(name, ".json"):
			return s.readJSON(path)
		case strings.HasSuffix(name, ".js") && !strings.HasSuffix(name, ".min.js"):
			return s.readJS(path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for lit := range s.literals {
		if strings.HasSuffix(lit, ".") && len(lit) >= 3 && !seen[lit] && looksLikeKeyStart(lit) {
			seen[lit] = true
			s.prefixes = append(s.prefixes, lit)
		}
	}
	sort.Slice(s.prefixes, func(i, j int) bool { return len(s.prefixes[i]) > len(s.prefixes[j]) })
	return s, nil
}

var keyStart = regexp.MustCompile(`^[a-z0-9_-]+(\.[a-z0-9_-]+)*\.$`)

func looksLikeKeyStart(s string) bool { return keyStart.MatchString(s) }

func (s *Scan) add(lit, path string, line int) {
	if lit == "" || len(lit) > 300 {
		return
	}
	s.literals[lit] = true
	s.lower[strings.ToLower(lit)] = true
	if keyShaped.MatchString(lit) && len(s.where[lit]) < maxWhere {
		rel, err := filepath.Rel(s.root, path)
		if err != nil {
			rel = path
		}
		at := filepath.ToSlash(rel)
		if line > 0 {
			at += ":" + strconv.Itoa(line)
		}
		for _, have := range s.where[lit] {
			if have == at {
				return
			}
		}
		s.where[lit] = append(s.where[lit], at)
	}
}

func (s *Scan) readGo(path string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	s.Files++
	fset := token.NewFileSet()
	var sc scanner.Scanner
	sc.Init(fset.AddFile(path, fset.Base(), len(src)), src, nil, 0)
	for {
		pos, tok, lit := sc.Scan()
		if tok == token.EOF {
			return nil
		}
		if tok == token.STRING {
			if v, err := strconv.Unquote(lit); err == nil {
				s.add(v, path, fset.Position(pos).Line)
			}
		}
	}
}

func (s *Scan) readJSON(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	s.Files++
	dec := json.NewDecoder(f)
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil // the end, or not JSON after all: what was read stands
		}
		if v, ok := tok.(string); ok {
			s.add(v, path, 0)
		}
	}
}

func (s *Scan) readJS(path string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	s.Files++
	line, last := 1, 0
	for _, loc := range jsString.FindAllSubmatchIndex(src, -1) {
		line += strings.Count(string(src[last:loc[0]]), "\n")
		last = loc[0]
		switch {
		case loc[2] >= 0 && loc[3] > loc[2]:
			s.add(string(src[loc[2]:loc[3]]), path, line)
		case loc[4] >= 0:
			s.add(string(src[loc[4]:loc[5]]), path, line)
		}
	}
	return nil
}

// Used reports whether the source refers to key, by the package's rules.
func (s *Scan) Used(key string) bool {
	if s.literals[key] {
		return true
	}
	if strings.HasPrefix(key, "field.") || strings.Contains(key, ".forms.") {
		return true
	}
	for _, p := range s.prefixes {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	parts := strings.Split(key, ".")
	if len(parts) < 2 {
		return false
	}
	for _, p := range parts {
		if !s.lower[p] {
			return false
		}
	}
	return true
}

// Unreferenced returns the keys the source does not refer to, sorted.
func (s *Scan) Unreferenced(keys []string) []string {
	var out []string
	for _, k := range keys {
		if !s.Used(k) {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// The ways a key can be in use, as Why reports them.
const (
	RuleLiteral    = "literal"    // the key appears whole in the source
	RulePrefix     = "prefix"     // a string ending in a dot is its start
	RuleConvention = "convention" // a form-engine convention builds it
	RuleParts      = "parts"      // each of its parts is in the source
	RuleNone       = ""           // nothing accounts for it
)

// Why says by which rule key is in use and, for the first two, where: the
// places the key, or the string it starts with, is written ("file:line",
// relative to the tree, a few at most). detail is the starting string for
// RulePrefix. It is what makes a candidate checkable instead of believed.
func (s *Scan) Why(key string) (rule, detail string, where []string) {
	if s.literals[key] {
		return RuleLiteral, "", append([]string(nil), s.where[key]...)
	}
	if strings.HasPrefix(key, "field.") || strings.Contains(key, ".forms.") {
		return RuleConvention, "", nil
	}
	for _, p := range s.prefixes {
		if strings.HasPrefix(key, p) {
			return RulePrefix, p, append([]string(nil), s.where[p]...)
		}
	}
	if s.Used(key) {
		return RuleParts, "", nil
	}
	return RuleNone, "", nil
}
