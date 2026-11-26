package designer

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"

	fe "github.com/ha1tch/seam-ui/internal/formengine"
)

// the keys every property has whatever the widget; they are not catalogue
// properties but the shape of the field itself
var coreKeys = map[string]bool{"title": true, "type": true, "format": true, "enum": true, "x-seam-input": true, "items": true}

func embeddedSchemaFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	_ = filepath.WalkDir("..", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".json") && strings.Contains(filepath.ToSlash(p), "/schemas/") {
			files = append(files, p)
		}
		return nil
	})
	if len(files) < 20 {
		t.Fatalf("expected the embedded schemas, found %d files", len(files))
	}
	return files
}

func TestCatalogue_IsWellFormed(t *testing.T) {
	seenW, seenP := map[string]bool{}, map[string]bool{}
	for _, w := range widgets {
		if w.Name == "" || w.Title == "" || w.Icon == "" || w.Category == "" || w.Summary == "" || !json.Valid([]byte(w.Template)) || seenW[w.Name] {
			t.Errorf("widget %+v is incomplete or repeated", w)
		}
		seenW[w.Name] = true
	}
	valid := map[Kind]bool{KindBool: true, KindInt: true, KindString: true, KindEnum: true, KindColor: true, KindIcon: true, KindList: true, KindMap: true, KindJSON: true}
	for _, p := range props {
		if p.Key == "" || p.Title == "" || p.Help == "" || !valid[p.Kind] || seenP[p.Key] || coreKeys[p.Key] {
			t.Errorf("property %+v is incomplete, repeated or a core key", p)
		}
		seenP[p.Key] = true
		if (p.Kind == KindEnum || p.Kind == KindColor) && len(p.Enum) == 0 {
			t.Errorf("%s: a %s needs its values", p.Key, p.Kind)
		}
		for _, w := range p.Widgets {
			if !seenW[w] {
				t.Errorf("%s applies to %q, which is not a widget", p.Key, w)
			}
		}
	}
	for _, c := range Categories() {
		if c.Name == "" || len(c.Widgets) == 0 {
			t.Errorf("empty category %+v", c)
		}
	}
}

// Every template is a property the engine accepts and draws.
func TestCatalogue_EveryWidgetTemplateParsesAndRenders(t *testing.T) {
	for _, w := range widgets {
		schema := `{"type":"object","properties":{"probe_field":` + w.Template + `}}`
		form, err := fe.ParseSchema([]byte(schema))
		if err != nil || len(form.Fields) != 1 {
			t.Errorf("%s: %v", w.Name, err)
			continue
		}
		html := mi.RenderToString((&fe.FormEngine{}).Render(context.Background(), []byte(schema), nil, nil, fe.RenderContext{Locale: "en"}))
		if !strings.Contains(html, "probe_field") {
			t.Errorf("%s renders nothing for its field:\n%.300s", w.Name, html)
		}
		if got := WidgetOf([]byte(w.Template)); got != w.Name {
			t.Errorf("%s: its template is classified as %q", w.Name, got)
		}
		if got := fe.InferWidget(form.Fields[0]); got != w.Name {
			t.Errorf("%s: the engine infers %q for its template", w.Name, got)
		}
	}
}

// Held to the engine one way: nothing the schema parser reads is missing here.
func TestCatalogue_HasEveryPropertyTheParserReads(t *testing.T) {
	src, err := os.ReadFile("../schema.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	i := strings.Index(text, "var p struct {")
	if i < 0 {
		t.Fatal("cannot find the parser's property struct")
	}
	var block []string
	for _, line := range strings.Split(text[i:], "\n")[1:] {
		if strings.HasPrefix(line, "\t}") {
			break
		}
		block = append(block, line)
	}
	tag := regexp.MustCompile("`json:\"([^\"]+)\"`")
	parser := map[string]bool{}
	for _, line := range block {
		if m := tag.FindStringSubmatch(line); m != nil {
			parser[m[1]] = true
		}
	}
	if len(parser) < 40 {
		t.Fatalf("read only %d tags from the parser: the test has lost its footing", len(parser))
	}
	for k := range parser {
		if coreKeys[k] {
			continue
		}
		if _, ok := LookupProp(k); !ok {
			t.Errorf("the parser reads %q and the catalogue does not know it: add it to catalogue.go", k)
		}
	}
	// and the other way: nothing is offered that the parser would ignore
	for _, p := range props {
		if !parser[p.Key] && !p.Validation {
			t.Errorf("the catalogue offers %q and the parser does not read it", p.Key)
		}
	}
}

// Every widget the embedded schemas use is known, and the catalogue classifies
// every real property the way the engine does.
func TestCatalogue_AgreesWithEveryEmbeddedSchema(t *testing.T) {
	for _, file := range embeddedSchemaFiles(t) {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		form, err := fe.ParseSchema(data)
		if err != nil {
			continue // a fragment, not a form
		}
		var doc struct {
			Properties map[string]json.RawMessage `json:"properties"`
		}
		_ = json.Unmarshal(data, &doc)
		for _, f := range form.Fields {
			raw, ok := doc.Properties[f.Name]
			if !ok {
				continue
			}
			want := fe.InferWidget(f)
			if _, known := LookupWidget(want); !known {
				t.Errorf("%s: field %q is drawn as %q, which the catalogue does not know", file, f.Name, want)
			}
			if got := WidgetOf(raw); got != want {
				t.Errorf("%s: field %q: WidgetOf says %q, the engine says %q", file, f.Name, got, want)
			}
			var props map[string]json.RawMessage
			_ = json.Unmarshal(raw, &props)
			for key := range props {
				if coreKeys[key] {
					continue
				}
				if _, ok := LookupProp(key); !ok {
					t.Errorf("%s: field %q uses %q, which the catalogue does not know", file, f.Name, key)
				}
			}
		}
	}
}

func TestCatalogue_PropsForAWidgetAreTheUniversalOnesPlusItsOwn(t *testing.T) {
	has := func(widget, key string) bool {
		for _, p := range PropsFor(widget) {
			if p.Key == key {
				return true
			}
		}
		return false
	}
	for _, c := range []struct {
		widget, key string
		want        bool
	}{
		{"text", "x-seam-help", true}, {"repeatable", "x-seam-help", true}, {"select", "x-seam-option-icons", true},
		{"text", "x-seam-option-icons", false}, {"repeatable", "minItems", true}, {"select", "minItems", false},
		{"decimal", "decimalScale", true}, {"number", "decimalScale", false}, {"barcode", "x-seam-barcode-format", true},
		{"textarea", "x-seam-rows", true}, {"any-picker", "x-seam-selector", true}, {"nonsense", "x-seam-help", true}, {"nonsense", "minItems", false},
	} {
		if got := has(c.widget, c.key); got != c.want {
			t.Errorf("PropsFor(%q) has %q = %v, want %v", c.widget, c.key, got, c.want)
		}
	}
	if _, ok := LookupWidget("nonsense"); ok {
		t.Errorf("an unknown widget is not found")
	}
}
