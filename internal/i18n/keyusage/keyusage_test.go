package keyusage

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScan_FindsTheKeysNothingRefersTo(t *testing.T) {
	root := t.TempDir()
	write(t, root, "internal/ui/page.go", "package ui\nfunc f(t func(string) string, scope string) {\n\t_ = t(\"orders.list.title\")\n\t_ = t(\"developer.scope.\" + scope)\n\t_ = `raw.key.here`\n\t_ = []string{\"asset\", \"status\"}\n}\n")
	write(t, root, "internal/ui/page_test.go", "package ui\nvar _ = \"only.in.a.test\"\n")
	write(t, root, "internal/formengine/schemas/asset/asset.json", `{"properties":{"status":{"enum":["Active","Retired"],"title":"asset.tab.principal"}}}`)
	write(t, root, "web/static/js/app.js", "const a = t('js.key.one'); const b = \"js.key.two\";\n")
	write(t, root, "web/static/js/lib.min.js", "x('minified.key')")
	write(t, root, "internal/i18n/locales/en.json", `{"defined.only.here": "x"}`)
	write(t, root, "node_modules/pkg/index.js", "t('dependency.key')")

	s, err := ScanDir(root)
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{
		"orders.list.title",        // a literal in Go
		"raw.key.here",             // a raw string
		"developer.scope.tenant",   // built from a prefix
		"asset.tab.principal",      // a label key in a definition
		"js.key.one", "js.key.two", // scripts
		"assets.forms.asset.field.name",   // the engine's convention
		"field.email", "field.email.help", // the general field keys
		"asset.status.active",            // form, field and value are all in the source
		"only.in.a.test",                 // tests do not count
		"minified.key", "dependency.key", // not this application's source
		"defined.only.here", // the locale files define, they do not use
		"orders.list.gone", "nowhere.at.all",
	}
	want := []string{"defined.only.here", "dependency.key", "minified.key", "nowhere.at.all", "only.in.a.test", "orders.list.gone"}
	if got := s.Unreferenced(keys); !reflect.DeepEqual(got, want) {
		t.Errorf("unreferenced:\n got %v\nwant %v", got, want)
	}
	if rule, _, where := s.Why("orders.list.title"); rule != RuleLiteral || len(where) != 1 || where[0] != "internal/ui/page.go:3" {
		t.Errorf("a literal says where it is written: %q %v", rule, where)
	}
	if rule, detail, where := s.Why("developer.scope.tenant"); rule != RulePrefix || detail != "developer.scope." || len(where) != 1 || where[0] != "internal/ui/page.go:4" {
		t.Errorf("a key built from a prefix names the prefix and its place: %q %q %v", rule, detail, where)
	}
	if rule, _, where := s.Why("js.key.two"); rule != RuleLiteral || len(where) != 1 || where[0] != "web/static/js/app.js:1" {
		t.Errorf("a script literal has its line: %q %v", rule, where)
	}
	for key, want := range map[string]string{"field.email": RuleConvention, "asset.status.active": RuleParts, "nowhere.at.all": RuleNone} {
		if rule, _, _ := s.Why(key); rule != want {
			t.Errorf("Why(%s) = %q, want %q", key, rule, want)
		}
	}
	if s.Files != 3 {
		t.Errorf("three files are source, read %d", s.Files)
	}
}

// Run against this repository: the scan must work on the real tree and leave
// the bulk of the keys in use.
func TestScan_OnThisRepository(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Skip("not in a checkout")
	}
	s, err := ScanDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if s.Files < 100 {
		t.Fatalf("the scan read only %d files", s.Files)
	}
	for _, k := range []string{"action.cancel", "developer.nav.forms", "developer.scope.tenant"} {
		if !s.Used(k) {
			t.Errorf("%s is in use", k)
		}
	}
	if s.Used("this.key.is.not.in.the.tree") {
		t.Errorf("an invented key is not in use")
	}
}
