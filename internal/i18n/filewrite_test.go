package i18n

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func writerFixture(t *testing.T) (*FileLayer, *Translator, string) {
	t.Helper()
	dir := t.TempDir()
	tr := NewFromMessages("en", map[string]map[string]string{
		"en": {"greet": "Hello %s", "bye": "Bye", "count": "%d items"},
		"es": {"greet": "Hola %s", "bye": "Adios", "count": "%d elementos"},
	})
	return NewFileLayer(tr, dir), tr, dir
}

func dirEntries(t *testing.T, dir string) []string {
	t.Helper()
	es, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range es {
		if e.Name() == historyDirName { // the layer's own record of its changes, not a locale file
			continue
		}
		out = append(out, e.Name())
	}
	return out
}

func TestSetOverride_WritesReloadsAndKeepsTheOtherKeys(t *testing.T) {
	l, tr, dir := writerFixture(t)
	if err := l.SetOverride("en", "greet", "Hi %s", "Hello %s"); err != nil {
		t.Fatal(err)
	}
	if err := l.SetOverride("en", "bye", "See you", "Bye"); err != nil {
		t.Fatal(err)
	}
	if got := tr.T("en", "greet", "Ana"); got != "Hi Ana" {
		t.Errorf("the override must be live at once, got %q", got)
	}
	if got := tr.T("es", "greet", "Ana"); got != "Hola Ana" {
		t.Errorf("another locale is untouched, got %q", got)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "en.json"))
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil || len(m) != 2 {
		t.Fatalf("en.json must hold both keys as valid JSON: %v %s", err, data)
	}
	if names := dirEntries(t, dir); len(names) != 1 || names[0] != "en.json" {
		t.Errorf("no temporary files may be left behind: %v", names)
	}
}

// Adversarial: nothing hostile reaches the disk.
func TestSetOverride_RefusesHostileInput(t *testing.T) {
	l, _, dir := writerFixture(t)
	long := strings.Repeat("x", MaxOverrideLen+1)
	cases := []struct{ name, locale, key, value, ref string }{
		{"traversal in the locale", "../etc/passwd", "greet", "Hi", ""},
		{"slash in the locale", "en/../x", "greet", "Hi", ""},
		{"upper-case locale", "EN", "greet", "Hi", ""},
		{"empty locale", "", "greet", "Hi", ""},
		{"space in the key", "en", "a b", "Hi", ""},
		{"newline in the key", "en", "a\nb", "Hi", ""},
		{"quote in the key", "en", `a"b`, "Hi", ""},
		{"empty key", "en", "", "Hi", ""},
		{"overlong key", "en", strings.Repeat("k", 201), "Hi", ""},
		{"NUL in the value", "en", "greet", "Hi\x00", ""},
		{"control character in the value", "en", "greet", "Hi\x1b[31m", ""},
		{"empty value", "en", "greet", "   ", ""},
		{"overlong value", "en", "greet", long, ""},
		{"verbs differ", "en", "count", "%s items", "%d items"},
		{"verb missing", "en", "greet", "Hi", "Hello %s"},
		{"verb added", "en", "bye", "Bye %s", "Bye"},
	}
	for _, c := range cases {
		if err := l.SetOverride(c.locale, c.key, c.value, c.ref); err == nil {
			t.Errorf("%s: must be refused", c.name)
		}
	}
	if names := dirEntries(t, dir); len(names) != 0 {
		t.Errorf("a refused write must leave the directory empty, found %v", names)
	}
	// legitimate texts that merely look odd are fine
	for _, v := range []string{"Line one\nLine two", "Tab\there", "100%% sure", "日本語 ✓"} {
		if err := l.SetOverride("en", "bye", v, ""); err != nil {
			t.Errorf("%q must be accepted: %v", v, err)
		}
	}
}

// The file is the developer's own work: a broken one is never replaced.
func TestSetOverride_NeverOverwritesABrokenFile(t *testing.T) {
	l, _, dir := writerFixture(t)
	broken := []byte(`{"greet": "half saved`)
	p := filepath.Join(dir, "en.json")
	if err := os.WriteFile(p, broken, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := l.SetOverride("en", "bye", "See you", ""); err == nil {
		t.Fatal("a write beside a broken file must be refused")
	}
	if err := l.RemoveOverride("en", "greet"); err == nil {
		t.Fatal("a removal from a broken file must be refused")
	}
	if got, _ := os.ReadFile(p); string(got) != string(broken) {
		t.Errorf("the broken file must be left exactly as it was, got %q", got)
	}
}

func TestRemoveOverride(t *testing.T) {
	l, tr, dir := writerFixture(t)
	_ = l.SetOverride("en", "greet", "Hi %s", "")
	_ = l.SetOverride("en", "bye", "See you", "")
	if err := l.RemoveOverride("en", "greet"); err != nil {
		t.Fatal(err)
	}
	if got := tr.T("en", "greet", "Ana"); got != "Hello Ana" {
		t.Errorf("removing an override restores the embedded text, got %q", got)
	}
	if got := tr.T("en", "bye"); got != "See you" {
		t.Errorf("the other override stays, got %q", got)
	}
	if err := l.RemoveOverride("en", "no.such.key"); err != nil {
		t.Errorf("removing an absent key is not an error: %v", err)
	}
	if err := l.RemoveOverride("en", "bye"); err != nil {
		t.Fatal(err)
	}
	if names := dirEntries(t, dir); len(names) != 0 {
		t.Errorf("removing the last key deletes the file, found %v", names)
	}
	if got := tr.T("en", "bye"); got != "Bye" {
		t.Errorf("got %q", got)
	}
}

// Adversarial: concurrent editors must leave one valid file holding every key
// (run with -race), and a reload that meets another writer's temporary file
// must not be derailed by it.
func TestOverrides_ConcurrentWritersLeaveAValidFile(t *testing.T) {
	l, _, dir := writerFixture(t)
	var wg sync.WaitGroup
	const n = 16
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := "k" + string(rune('a'+i))
			for j := 0; j < 10; j++ {
				if err := l.SetOverride("en", key, "value "+key, ""); err != nil {
					t.Errorf("writer %d: %v", i, err)
					return
				}
			}
		}(i)
	}
	wg.Wait()
	m, err := l.Overrides("en")
	if err != nil || len(m) != n {
		t.Fatalf("want %d keys in a valid file, got %d (%v)", n, len(m), err)
	}
	for _, name := range dirEntries(t, dir) {
		if strings.HasPrefix(name, ".tmp-") {
			t.Errorf("temporary file left behind: %s", name)
		}
	}
}

func TestSameVerbs_IsExported(t *testing.T) {
	if !SameVerbs("%s has %d", "%s hat %d") || SameVerbs("%s", "%d") || SameVerbs("%s %d", "%d %s") {
		t.Errorf("SameVerbs must agree with the comparison used for the reports")
	}
}
