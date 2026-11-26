package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The stylesheet is precompiled, so a utility class that was never used
// elsewhere does not exist and silently does nothing. This lists the classes
// the accounting screens put in their string literals and fails for any that is
// in neither tailwind.css nor app.css.
func TestAccountingClassesExist(t *testing.T) {
	var css strings.Builder
	for _, f := range []string{"tailwind.css", "app.css"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "web", "static", "css", f))
		if err != nil {
			t.Fatal(err)
		}
		css.Write(b)
	}
	have := map[string]bool{}
	selRE := regexp.MustCompile(`\.((?:\\.|[\w-])+)`)
	unesc := regexp.MustCompile(`\\(.)`)
	for _, m := range selRE.FindAllStringSubmatch(css.String(), -1) {
		have[unesc.ReplaceAllString(m[1], "$1")] = true
	}

	files, _ := filepath.Glob("accounting_*.go")
	litRE := regexp.MustCompile(`"((?:[^"\\\n]|\\.)*)"`)
	tokenRE := regexp.MustCompile(`^[a-z0-9:/\[\].\-()%#!]+$`)
	utilRE := regexp.MustCompile(`^((dark|hover|focus|sm|md|lg|disabled):)*((bg|text|border|px|py|p[trblxy]?|m[trblxy]?|gap|space|rounded|font|w|h|max|min|items|justify|divide|shadow|overflow|whitespace|tracking|align|sr|cursor|opacity|ring|col|leading|shrink|order)-|(flex|grid|inline|block|hidden|uppercase|truncate|relative|absolute|underline|tabular)(-|$))`)
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, _ := os.ReadFile(f)
		for _, m := range litRE.FindAllStringSubmatch(string(src), -1) {
			toks := strings.Fields(m[1])
			if len(toks) == 0 {
				continue
			}
			ok := true
			for _, c := range toks {
				if !tokenRE.MatchString(c) {
					ok = false
				}
			}
			if !ok || len(toks) == 1 && !strings.ContainsAny(toks[0], "-:") {
				continue
			}
			for _, c := range toks {
				if utilRE.MatchString(c) && !have[c] && !strings.Contains(c, "/") && !strings.Contains(c, "%") {
					t.Errorf("%s: class %q is in neither tailwind.css nor app.css", f, c)
				}
			}
		}
	}
}
