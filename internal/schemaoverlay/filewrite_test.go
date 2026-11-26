package schemaoverlay

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// objectValidator stands in for the form engine's parser: a JSON object with a
// "properties" object.
func objectValidator(b []byte) error {
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	if _, ok := m["properties"].(map[string]any); !ok {
		return errors.New("not a form definition")
	}
	return nil
}

const (
	baseDoc = `{"type":"object","required":["alpha"],"properties":{"alpha":{"type":"string"},"beta":{"type":"string"}}}`
	goodDoc = `{"type":"object","properties":{"alpha":{"type":"string","title":"Alpha OVERRIDE"}}}`
)

func schemaFixture(t *testing.T) (*FileLayer, *Resolver, string) {
	t.Helper()
	dir := t.TempDir()
	r := New(objectValidator)
	return NewFileLayer(r, dir), r, dir
}

func TestSave_WritesReloadsAndIndents(t *testing.T) {
	l, r, dir := schemaFixture(t)
	if _, err := l.Save("system", "user_edit", []byte(goodDoc), nil); err != nil {
		t.Fatal(err)
	}
	if got := r.Resolve(1, "system", "user_edit", []byte(baseDoc)); !strings.Contains(string(got), "Alpha OVERRIDE") {
		t.Errorf("the override must be live at once, got %s", got)
	}
	data, err := os.ReadFile(filepath.Join(dir, "system", "user_edit.json"))
	if err != nil || !strings.Contains(string(data), "\n  ") || !strings.HasSuffix(string(data), "}\n") {
		t.Errorf("the file is stored indented with a final newline: %q %v", data, err)
	}
	if doc, ok, _ := l.Read("system", "user_edit"); !ok || len(doc) == 0 {
		t.Errorf("Read must return what Save wrote")
	}
}

// A document that does not validate is never written; the last good one stays.
func TestSave_RefusesWhatDoesNotValidateAndKeepsTheLastGoodFile(t *testing.T) {
	l, r, dir := schemaFixture(t)
	if _, err := l.Save("system", "user_edit", []byte(goodDoc), nil); err != nil {
		t.Fatal(err)
	}
	for name, bad := range map[string]string{"not json": `{"type":`, "no properties": `{"type":"object"}`, "array": `[1]`, "empty": ``, "null": `null`} {
		if _, err := l.Save("system", "user_edit", []byte(bad), nil); err == nil {
			t.Errorf("%s: must be refused", name)
		}
		if got := r.Resolve(1, "system", "user_edit", []byte(baseDoc)); !strings.Contains(string(got), "Alpha OVERRIDE") {
			t.Errorf("%s: the last good override must stay in force", name)
		}
	}
	data, _ := os.ReadFile(filepath.Join(dir, "system", "user_edit.json"))
	if !strings.Contains(string(data), "Alpha OVERRIDE") {
		t.Errorf("the file on disk must still be the last good one")
	}
}

// Adversarial: names are identifiers, never paths.
func TestSave_RefusesPathsAndOddNames(t *testing.T) {
	l, _, dir := schemaFixture(t)
	parent := filepath.Dir(dir)
	before, _ := os.ReadDir(parent)
	for _, c := range [][2]string{
		{"../x", "form"}, {"a/b", "form"}, {"system", "../form"}, {"system", "a/b"}, {"System", "form"},
		{"system", "Form"}, {"system", "x.y"}, {"", "form"}, {"system", ""}, {"system", "form\x00"}, {"..", ".."}, {"sys tem", "form"},
	} {
		if _, err := l.Save(c[0], c[1], []byte(goodDoc), nil); err == nil {
			t.Errorf("%q/%q must be refused", c[0], c[1])
		}
		if err := l.Remove(c[0], c[1]); err == nil {
			t.Errorf("Remove(%q/%q) must be refused", c[0], c[1])
		}
	}
	after, _ := os.ReadDir(parent)
	if len(after) != len(before) {
		t.Errorf("nothing may be created outside the layer's directory")
	}
	if es, _ := os.ReadDir(dir); len(es) != 0 {
		t.Errorf("a refused save leaves the directory empty")
	}
}

func TestSave_RefusesAnOversizeDocument(t *testing.T) {
	l, _, _ := schemaFixture(t)
	big := `{"properties":{},"pad":"` + strings.Repeat("x", MaxSchemaBytes) + `"}`
	if _, err := l.Save("system", "big", []byte(big), nil); err == nil {
		t.Errorf("a document over %d bytes must be refused", MaxSchemaBytes)
	}
}

func TestSave_ReturnsCompatibilityWarnings(t *testing.T) {
	l, _, _ := schemaFixture(t)
	warnings, err := l.Save("system", "user_edit", []byte(goodDoc), []byte(baseDoc))
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) == 0 {
		t.Errorf("an override that removes a property must be reported (advice, not a gate)")
	}
}

func TestRemove_RestoresTheBaseAndIsIdempotent(t *testing.T) {
	l, r, dir := schemaFixture(t)
	_, _ = l.Save("system", "user_edit", []byte(goodDoc), nil)
	if err := l.Remove("system", "user_edit"); err != nil {
		t.Fatal(err)
	}
	if got := r.Resolve(1, "system", "user_edit", []byte(baseDoc)); string(got) != baseDoc {
		t.Errorf("removing the override restores the embedded document, got %s", got)
	}
	if err := l.Remove("system", "user_edit"); err != nil {
		t.Errorf("removing an absent override is not an error: %v", err)
	}
	if _, ok, _ := l.Read("system", "user_edit"); ok {
		t.Errorf("Read must report no override")
	}
	if _, err := os.Stat(filepath.Join(dir, "system", "user_edit.json")); !os.IsNotExist(err) {
		t.Errorf("the file must be gone")
	}
}

// Adversarial: concurrent saves and removals leave a consistent layer (run
// with -race) and a reload that meets another writer's temporary file is not
// derailed by it.
func TestSaveRemove_ConcurrentWritersStayConsistent(t *testing.T) {
	l, _, dir := schemaFixture(t)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := "form" + string(rune('a'+i%4))
			for j := 0; j < 8; j++ {
				if (i+j)%3 == 0 {
					if err := l.Remove("system", name); err != nil {
						t.Errorf("remove: %v", err)
						return
					}
				} else if _, err := l.Save("system", name, []byte(goodDoc), nil); err != nil {
					t.Errorf("save: %v", err)
					return
				}
			}
		}(i)
	}
	wg.Wait()
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() && info.Name() == historyDirName {
			return filepath.SkipDir // the layer's own record of its changes, not a definition
		}
		if err == nil && !info.IsDir() {
			if strings.HasPrefix(info.Name(), ".tmp-") {
				t.Errorf("temporary file left behind: %s", p)
			}
			if b, _ := os.ReadFile(p); objectValidator(b) != nil {
				t.Errorf("%s is not a valid document", p)
			}
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}
