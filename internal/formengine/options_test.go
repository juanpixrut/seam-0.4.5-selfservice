package formengine

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"testing"

	mi "github.com/ha1tch/minty"
)

// withOptions sets the process-wide options for one test and restores the
// defaults afterwards (the options are global, so tests that set them must not
// run in parallel).
func withOptions(t *testing.T, o Options) {
	t.Helper()
	SetOptions(o)
	t.Cleanup(func() { SetOptions(DefaultOptions()) })
}

func selectHTML(t *testing.T) string {
	t.Helper()
	f := Field{Name: "unit", Title: "Unit", JSONType: "string", XSeamInput: "select", Enum: []string{"a", "b"}}
	return mi.RenderToString(renderSelect(context.Background(), &FormEngine{}, f, "", false, RenderContext{}))
}

func TestOptions_DefaultsAreTheShippedChoices(t *testing.T) {
	d := DefaultOptions()
	if !d.SearchableSelect || d.SelectSearchMin != 8 || !d.ReorderRows || d.DocumentLinksLegacy {
		t.Errorf("unexpected defaults: %+v", d)
	}
}

// Adversarial: a threshold outside 1..10000 must never turn into "always show
// the search box" or "never" by accident; it falls back to the default.
func TestOptions_SearchMinOutOfRangeFallsBackToDefault(t *testing.T) {
	for _, c := range []struct{ in, want int }{
		{0, 8}, {-1, 8}, {-1 << 31, 8}, {1, 1}, {8, 8}, {10000, 10000}, {10001, 8}, {1 << 62, 8},
	} {
		SetOptions(Options{SelectSearchMin: c.in})
		if got := CurrentOptions().SelectSearchMin; got != c.want {
			t.Errorf("SelectSearchMin %d -> %d, want %d", c.in, got, c.want)
		}
	}
	SetOptions(DefaultOptions())
}

func TestSelect_FlagOffLeavesTheNativeSelectAlone(t *testing.T) {
	withOptions(t, Options{SearchableSelect: false, SelectSearchMin: 8, ReorderRows: true})
	html := selectHTML(t)
	for _, s := range []string{"data-seam-select", "data-ss-search", "data-ss-empty", "data-ss-min"} {
		if strings.Contains(html, s) {
			t.Errorf("searchable_select off must not output %q:\n%s", s, html)
		}
	}
	if !strings.Contains(html, "<select") || !strings.Contains(html, "<option") {
		t.Errorf("the native select must still render:\n%s", html)
	}
}

func TestSelect_ThresholdIsHandedToTheBrowser(t *testing.T) {
	withOptions(t, Options{SearchableSelect: true, SelectSearchMin: 3, ReorderRows: true})
	if html := selectHTML(t); !strings.Contains(html, `data-ss-min="3"`) {
		t.Errorf("want data-ss-min=\"3\":\n%s", html)
	}
	withOptions(t, Options{SearchableSelect: true, SelectSearchMin: -5, ReorderRows: true})
	if html := selectHTML(t); !strings.Contains(html, `data-ss-min="8"`) {
		t.Errorf("a negative threshold must reach the browser as the default 8:\n%s", html)
	}
}

// With the global switch off, a schema that asks for reordering renders
// byte-for-byte like one that does not: no handle, no marker, no position input.
func TestReorder_KillSwitchMakesTheSchemaPropertyInert(t *testing.T) {
	withOptions(t, Options{SearchableSelect: true, SelectSearchMin: 8, ReorderRows: false})
	for _, naming := range []string{``, `"x-seam-naming": "indexed",`} {
		rows := []interface{}{map[string]interface{}{"step": "a"}, map[string]interface{}{"step": "b"}}
		on := renderRepeatableHTML(reorderField(t, naming+`"x-seam-reorder": true,`), rows)
		off := renderRepeatableHTML(reorderField(t, naming), rows)
		if on != off {
			t.Errorf("naming %q: reorder_rows=false must make x-seam-reorder inert\n--- on\n%s\n--- off\n%s", naming, on, off)
		}
		for _, s := range []string{"fe-drag", "data-reorder", "_pos"} {
			if strings.Contains(on, s) {
				t.Errorf("naming %q: found %q with the kill switch on", naming, s)
			}
		}
	}
}

// Adversarial: the client controls _pos. Whatever it posts, parsing must not
// panic, must never leak _pos into the rows, and must fall back to index order
// unless EVERY row has a plain, in-range, non-negative number.
type pair struct{ k, v string }

func cat(parts ...[]pair) []pair {
	var out []pair
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func TestReorder_ForgedPositionsNeverBreakParsing(t *testing.T) {
	f := reorderField(t, `"x-seam-reorder": true, "x-seam-naming": "indexed",`)
	row := func(i, name string, extra ...pair) []pair {
		return append([]pair{{"steps[" + i + "][step]", name}}, extra...)
	}
	pos := func(i, v string) pair { return pair{"steps[" + i + "][_pos]", v} }
	cases := []struct {
		name string
		in   []pair
		want string
	}{
		{"valid reversal", cat(row("0", "a", pos("0", "2")), row("1", "b", pos("1", "1")), row("2", "c", pos("2", "0"))), "cba"},
		{"eleven digits is out of range", cat(row("0", "a", pos("0", "99999999999")), row("1", "b", pos("1", "0")), row("2", "c", pos("2", "1"))), "abc"},
		{"negative", cat(row("0", "a", pos("0", "-1")), row("1", "b", pos("1", "1"))), "ab"},
		{"not a number", cat(row("0", "a", pos("0", "abc")), row("1", "b", pos("1", "0"))), "ab"},
		{"empty", cat(row("0", "a", pos("0", "")), row("1", "b", pos("1", "0"))), "ab"},
		{"whitespace and sign", cat(row("0", "a", pos("0", " 1")), row("1", "b", pos("1", "+0"))), "ab"},
		{"all equal", cat(row("0", "a", pos("0", "5")), row("1", "b", pos("1", "5")), row("2", "c", pos("2", "5"))), "abc"},
		{"one row without a position", cat(row("0", "a", pos("0", "1")), row("1", "b"), row("2", "c", pos("2", "0"))), "abc"},
		{"position is a branch, not a value", cat(row("0", "a", pair{"steps[0][_pos][x]", "0"}), row("1", "b", pos("1", "0"))), "ab"},
		{"duplicate keys: first value wins", cat(row("0", "a", pos("0", "1"), pos("0", "0")), row("1", "b", pos("1", "0"))), "ba"},
	}
	for _, c := range cases {
		v := url.Values{}
		for _, p := range c.in {
			v.Add(p.k, p.v)
		}
		var got []interface{}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("%s: panic: %v", c.name, r)
				}
			}()
			got = ParseIndexedRepeatable(v, f)
		}()
		order := ""
		for _, r := range got {
			m := r.(map[string]interface{})
			if _, leaked := m["_pos"]; leaked {
				t.Errorf("%s: _pos leaked into a row: %#v", c.name, m)
			}
			order += m["step"].(string)
		}
		if order != c.want {
			t.Errorf("%s: order %q, want %q", c.name, order, c.want)
		}
	}
}

// A forged _pos on a repeatable that never asked for reordering is stripped
// from the result like any other, whatever it does to the order.
func TestReorder_PositionIsStrippedEvenWhenTheFieldDidNotAskForIt(t *testing.T) {
	f := reorderField(t, `"x-seam-naming": "indexed",`)
	v := url.Values{}
	v.Set("steps[0][step]", "a")
	v.Set("steps[0][_pos]", "1")
	v.Set("steps[1][step]", "b")
	v.Set("steps[1][_pos]", "0")
	for _, r := range ParseIndexedRepeatable(v, f) {
		if _, leaked := r.(map[string]interface{})["_pos"]; leaked {
			t.Errorf("_pos leaked: %#v", r)
		}
	}
}

func TestDocumentLinks_LegacyFlagSwitchesTheLook(t *testing.T) {
	withOptions(t, Options{SearchableSelect: true, SelectSearchMin: 8, ReorderRows: true, DocumentLinksLegacy: true})
	old := renderLinksRow(t, nil)
	for _, want := range []string{"doc-link-row", "\u2715", `name="links_comodato_url[]"`} {
		if !strings.Contains(old, want) {
			t.Errorf("legacy look missing %q:\n%s", want, old)
		}
	}
	if strings.Contains(old, "fe-remove") {
		t.Errorf("legacy look must not use the repeatable's remove button:\n%s", old)
	}
	SetOptions(DefaultOptions())
	cur := renderLinksRow(t, nil)
	if strings.Contains(cur, "doc-link-row") || !strings.Contains(cur, "fe-remove") {
		t.Errorf("default look must be the repeatable's:\n%s", cur)
	}
	// both modes post the same field names, so no handler cares which is on
	for _, name := range []string{`name="links_comodato_nombre[]"`, `name="links_comodato_url[]"`, `name="links_comodato_tipo[]"`} {
		if !strings.Contains(old, name) || !strings.Contains(cur, name) {
			t.Errorf("field name %s must be identical in both looks", name)
		}
	}
}

// Adversarial: the options are flipped while forms render (run with -race).
func TestOptions_FlippedWhileRenderingIsSafe(t *testing.T) {
	t.Cleanup(func() { SetOptions(DefaultOptions()) })
	f := reorderField(t, `"x-seam-reorder": true, "x-seam-naming": "indexed",`)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(2)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				SetOptions(Options{SearchableSelect: i%2 == 0, SelectSearchMin: i % 12, ReorderRows: (i+g)%3 != 0, DocumentLinksLegacy: i%5 == 0})
			}
		}(g)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				if renderRepeatableHTML(f, nil) == "" || selectHTML(t) == "" || renderLinksRow(t, nil) == "" {
					t.Error("empty render")
					return
				}
			}
		}()
	}
	wg.Wait()
}
