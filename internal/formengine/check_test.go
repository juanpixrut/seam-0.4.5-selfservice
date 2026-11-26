package formengine

import (
	"errors"
	"strings"
	"testing"
)

func TestCheckRenderable(t *testing.T) {
	ok := `{"type":"object","properties":{"a":{"type":"string","x-seam-input":"text"},"s":{"type":"integer","x-seam-input":"select","x-seam-source":"entity:brands"},"p":{"type":"integer","x-seam-input":"any-picker","x-seam-source":"entity:users"}}}`
	known := func(s string) bool { return s == "entity:brands" }
	if err := CheckRenderable([]byte(ok), known); err != nil {
		t.Errorf("a renderable form: %v", err)
	}
	if err := CheckRenderable([]byte(ok), nil); err != nil {
		t.Errorf("with no source check: %v", err)
	}
	// a picker fetches its own records: its source is not the resolver's business
	if err := CheckRenderable([]byte(ok), func(string) bool { return false }); err == nil || strings.Contains(err.Error(), `"p"`) || !strings.Contains(err.Error(), `"s"`) {
		t.Errorf("only the select's source is checked: %v", err)
	}
	for _, c := range []struct{ name, doc, want string }{
		{"no fields", `{"type":"object","properties":{}}`, "no fields"},
		{"unknown widget", `{"type":"object","properties":{"a":{"type":"string","x-seam-input":"nope"}}}`, `no widget "nope"`},
		{"unknown widget in a repeatable", `{"type":"object","properties":{"l":{"type":"array","x-seam-input":"repeatable","items":{"type":"object","properties":{"z":{"type":"string","x-seam-input":"nope"}}}}}}`, `"l.z"`},
		{"unknown source", `{"type":"object","properties":{"s":{"type":"integer","x-seam-input":"select","x-seam-source":"entity:none"}}}`, `"entity:none"`},
	} {
		err := CheckRenderable([]byte(c.doc), known)
		if !errors.Is(err, ErrNotRenderable) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want it to name %s", c.name, err, c.want)
		}
	}
	// every problem is reported, not only the first
	two := `{"type":"object","properties":{"a":{"type":"string","x-seam-input":"nope"},"s":{"type":"integer","x-seam-input":"select","x-seam-source":"entity:none"}}}`
	if err := CheckRenderable([]byte(two), known); err == nil || !strings.Contains(err.Error(), "nope") || !strings.Contains(err.Error(), "entity:none") {
		t.Errorf("both problems: %v", err)
	}
	// what does not parse is the parser's error, not this one's
	if err := CheckRenderable([]byte(`[1]`), nil); err == nil || errors.Is(err, ErrNotRenderable) {
		t.Errorf("a document that does not parse: %v", err)
	}
	if got := SourcesOf([]byte(ok)); len(got) != 1 || got[0] != "entity:brands" {
		t.Errorf("SourcesOf lists the selects' sources only: %v", got)
	}
	if got := SourcesOf([]byte(`nope`)); got != nil {
		t.Errorf("a document that does not parse has none: %v", got)
	}
}

// Every widget the registry has can be named by a definition: the check and
// the renderer must agree on what exists.
func TestCheckRenderable_KnowsEveryRegisteredWidget(t *testing.T) {
	for name := range widgetRegistry {
		doc := `{"type":"object","properties":{"a":{"type":"string","x-seam-input":"` + name + `"}}}`
		if err := CheckRenderable([]byte(doc), nil); errors.Is(err, ErrNotRenderable) {
			t.Errorf("%s is registered but refused: %v", name, err)
		}
	}
}
