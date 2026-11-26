package formengine

import (
	"context"
	"net/url"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"
)

func boundsField(t *testing.T, extra string) Field {
	t.Helper()
	return reorderField(t, extra) // the shared one-repeatable schema; extra is spliced into its property
}

func disabledCount(html string) int { return strings.Count(html, `disabled="disabled"`) }
func rowCount(html string) int      { return strings.Count(html, `repeatable-row"`) }

func TestBounds_ParseAcceptsAndRefuses(t *testing.T) {
	ok := map[string][2]int{
		`"minItems": 1,`: {1, 0}, `"maxItems": 3,`: {0, 3}, `"minItems": 2, "maxItems": 2,`: {2, 2},
		`"minItems": 0,`: {0, 0}, `"maxItems": null,`: {0, 0}, `"minItems": 1000, "maxItems": 1000,`: {1000, 1000},
	}
	for extra, want := range ok {
		f := boundsField(t, extra)
		if f.MinItems != want[0] || f.MaxItems != want[1] {
			t.Errorf("%s -> min %d max %d, want %v", extra, f.MinItems, f.MaxItems, want)
		}
	}
	bad := []string{
		`"minItems": -1,`, `"minItems": 1001,`, `"maxItems": 0,`, `"maxItems": -3,`, `"maxItems": 1001,`,
		`"minItems": 3, "maxItems": 2,`, `"minItems": 2.5,`, `"minItems": "2",`, `"maxItems": true,`, `"minItems": 9999999999999,`,
	}
	for _, extra := range bad {
		_, err := ParseSchema([]byte(`{"type":"object","properties":{"steps":{"type":"array","x-seam-input":"repeatable",` + extra +
			`"items":{"type":"object","properties":{"step":{"type":"string","x-seam-input":"text"}}}}}}`))
		if err == nil {
			t.Errorf("%s must be refused when the schema is parsed", extra)
		}
	}
}

// With no bound the output is exactly what it was.
func TestBounds_UnconfiguredOutputIsUnchanged(t *testing.T) {
	rows := []interface{}{map[string]interface{}{"step": "a"}}
	plain := renderRepeatableHTML(boundsField(t, ""), rows)
	zero := renderRepeatableHTML(boundsField(t, `"minItems": 0,`), rows)
	if plain != zero {
		t.Errorf("minItems 0 is the default and must not change the output")
	}
	for _, s := range []string{"data-min", "data-max", "SeamRepeatable.", `disabled="disabled"`} {
		if strings.Contains(plain, s) {
			t.Errorf("an unbounded repeatable must not output %q", s)
		}
	}
}

func TestBounds_FlatPadsToTheMinimumAndLocksRemove(t *testing.T) {
	f := boundsField(t, `"minItems": 3,`)
	html := renderRepeatableHTML(f, []interface{}{map[string]interface{}{"step": "a"}})
	if rowCount(html) != 3 {
		t.Errorf("one row of data and minItems 3 must show 3 rows, got %d", rowCount(html))
	}
	if disabledCount(html) != 3 || !strings.Contains(html, `data-min="3"`) || strings.Contains(html, "data-max") {
		t.Errorf("at the minimum every remove button is disabled (3), the container says data-min only:\n%s", html)
	}
	four := renderRepeatableHTML(f, []interface{}{map[string]interface{}{"step": "a"}, map[string]interface{}{"step": "b"},
		map[string]interface{}{"step": "c"}, map[string]interface{}{"step": "d"}})
	if rowCount(four) != 4 || disabledCount(four) != 0 {
		t.Errorf("above the minimum nothing is locked (rows %d, disabled %d)", rowCount(four), disabledCount(four))
	}
}

func TestBounds_FlatLocksAddAtTheMaximumAndKeepsExistingData(t *testing.T) {
	f := boundsField(t, `"maxItems": 3,`)
	rows := func(n int) []interface{} {
		var out []interface{}
		for i := 0; i < n; i++ {
			out = append(out, map[string]interface{}{"step": string(rune('a' + i))})
		}
		return out
	}
	if h := renderRepeatableHTML(f, rows(2)); disabledCount(h) != 0 {
		t.Errorf("below the maximum add stays enabled")
	}
	if h := renderRepeatableHTML(f, rows(3)); disabledCount(h) != 1 {
		t.Errorf("at the maximum only the add button is disabled, got %d", disabledCount(h))
	}
	over := renderRepeatableHTML(f, rows(5))
	if rowCount(over) != 5 || disabledCount(over) != 1 {
		t.Errorf("existing data above the maximum is never hidden: rows %d, disabled %d", rowCount(over), disabledCount(over))
	}
}

func TestBounds_HandlersOnlyEnforceWhenBounded(t *testing.T) {
	bounded := renderRepeatableHTML(boundsField(t, `"minItems": 1, "maxItems": 4,`), nil)
	for _, s := range []string{"SeamRepeatable.canAdd(c)", "SeamRepeatable.canRemove(c)", "SeamRepeatable.sync(c)"} {
		if !strings.Contains(bounded, s) {
			t.Errorf("a bounded repeatable's handlers must call %s", s)
		}
	}
	// the original handler logic is still there, untouched
	for _, s := range []string{"c._rowTemplate||c.querySelector(&#39;.repeatable-row&#39;)", "closest(&#39;.repeatable-row&#39;)"} {
		if !strings.Contains(bounded, s) {
			t.Errorf("the clone and stash logic must survive: missing %s", s)
		}
	}
}

func TestBounds_IndexedPadsLocksAndKeepsTheTemplateUnlocked(t *testing.T) {
	f := boundsField(t, `"x-seam-naming": "indexed", "minItems": 2, "maxItems": 3,`)
	html := renderRepeatableHTML(f, nil)
	if rowCount(html)-1 != 2 { // the template holds one more
		t.Errorf("an empty indexed list with minItems 2 shows 2 rows (plus the template), counted %d", rowCount(html))
	}
	if !strings.Contains(html, `data-min="2"`) || !strings.Contains(html, `data-max="3"`) || !strings.Contains(html, `data-next="2"`) {
		t.Errorf("the container carries both bounds and the next index follows the padding:\n%s", html)
	}
	a := strings.Index(html, "<template>")
	b := strings.Index(html, "</template>")
	if a < 0 || b < a {
		t.Fatal("no template")
	}
	if strings.Contains(html[a:b], "disabled") {
		t.Errorf("the template row is what an added row is made from: it must not be locked")
	}
	if got := disabledCount(html[:a]); got != 2 {
		t.Errorf("the two rows at the minimum have disabled remove buttons, got %d", got)
	}
}

func TestBounds_NestedRepeatablesKeepTheirOwn(t *testing.T) {
	form, err := ParseSchema([]byte(`{"type":"object","properties":{
	  "stages":{"type":"array","x-seam-input":"repeatable","maxItems":2,
	    "items":{"type":"object","properties":{
	      "name":{"type":"string","x-seam-input":"text"},
	      "tasks":{"type":"array","x-seam-input":"repeatable","minItems":1,
	        "items":{"type":"object","properties":{"task":{"type":"string","x-seam-input":"text"}}}}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	html := renderRepeatableHTML(form.Fields[0], nil)
	if strings.Count(html, `data-max="2"`) != 1 {
		t.Errorf("the outer list has its own maximum")
	}
	if strings.Count(html, `data-min="1"`) < 1 {
		t.Errorf("the nested list has its own minimum")
	}
	if form.Fields[0].Items[1].MinItems != 1 || form.Fields[0].MinItems != 0 {
		t.Errorf("bounds belong to the field that declares them")
	}
}

func TestBounds_ReorderAndBoundsCoexist(t *testing.T) {
	html := renderRepeatableHTML(boundsField(t, `"x-seam-reorder": true, "minItems": 1, "maxItems": 2,`), nil)
	for _, s := range []string{`data-reorder="1"`, `data-min="1"`, `data-max="2"`} {
		if !strings.Contains(html, s) {
			t.Errorf("missing %s", s)
		}
	}
}

func TestBounds_ServerSideCheckCountsBothNamings(t *testing.T) {
	indexed := boundsField(t, `"x-seam-naming": "indexed", "minItems": 2, "maxItems": 3,`)
	flat := boundsField(t, `"minItems": 2, "maxItems": 3,`)
	e, rc := &FormEngine{}, RenderContext{}
	idx := func(names ...string) url.Values {
		v := url.Values{}
		for i, n := range names {
			v.Set("steps["+string(rune('0'+i))+"][step]", n)
		}
		return v
	}
	flatPost := func(names ...string) url.Values { return url.Values{"steps_step[]": names} }
	cases := []struct {
		name  string
		field Field
		post  url.Values
		want  string
	}{
		{"indexed within", indexed, idx("a", "b"), ""},
		{"indexed at the maximum", indexed, idx("a", "b", "c"), ""},
		{"indexed too few", indexed, idx("a"), "At least 2 items are required"},
		{"indexed too many", indexed, idx("a", "b", "c", "d"), "At most 3 items are allowed"},
		{"indexed nothing posted", indexed, url.Values{}, "At least 2 items are required"},
		{"indexed blank rows do not count", indexed, idx("a", "", ""), "At least 2 items are required"},
		{"flat within", flat, flatPost("a", "b"), ""},
		{"flat too few", flat, flatPost("a"), "At least 2 items are required"},
		{"flat too many", flat, flatPost("a", "b", "c", "d"), "At most 3 items are allowed"},
		{"flat blank rows do not count", flat, flatPost("a", " ", ""), "At least 2 items are required"},
		{"flat nothing posted", flat, url.Values{}, "At least 2 items are required"},
	}
	for _, c := range cases {
		errs := ValidatePostedBounds(e, rc, &Form{Fields: []Field{c.field}}, c.post)
		if got := errs["steps"]; got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
	if ValidatePostedBounds(e, rc, nil, nil) != nil {
		t.Errorf("a nil form has nothing to check")
	}
	if errs := ValidatePostedBounds(e, rc, &Form{Fields: []Field{boundsField(t, "")}}, url.Values{}); errs != nil {
		t.Errorf("an unbounded repeatable is never reported: %v", errs)
	}
}

// Adversarial: hostile posted values only ever count as rows.
func TestBounds_ServerSideCheckSurvivesHostilePosts(t *testing.T) {
	f := boundsField(t, `"x-seam-naming": "indexed", "minItems": 1, "maxItems": 2,`)
	post := url.Values{"steps[0][step]": {"a"}, "steps[9999999999][step]": {"b"}, "steps[-1][step]": {"c"}, "steps[x][y][z]": {"d"},
		"steps[1][step]": {strings.Repeat("x", 1<<16)}, "steps": {"scalar"}, "steps[][step]": {"e"}}
	_ = ValidatePostedBounds(&FormEngine{}, RenderContext{}, &Form{Fields: []Field{f}}, post) // must not panic
}

type boundsTranslator struct{}

func (boundsTranslator) T(locale, key string, args ...any) string {
	switch key {
	case "field.repeatable.min_items":
		return "Mínimo %d filas"
	case "field.repeatable.max_items":
		return "Máximo %d filas"
	}
	return key
}

func TestBounds_MessagesAreTranslatable(t *testing.T) {
	f := boundsField(t, `"minItems": 2, "maxItems": 3,`)
	e := &FormEngine{Translator: boundsTranslator{}}
	if got := RepeatableBoundsMessage(e, RenderContext{Locale: "es"}, f, 1); got != "Mínimo 2 filas" {
		t.Errorf("got %q", got)
	}
	if got := RepeatableBoundsMessage(e, RenderContext{Locale: "es"}, f, 9); got != "Máximo 3 filas" {
		t.Errorf("got %q", got)
	}
	if got := RepeatableBoundsMessage(e, RenderContext{}, f, 2); got != "" {
		t.Errorf("within bounds there is no message, got %q", got)
	}
}

// Adversarial: a schema author asking for a thousand rows gets a thousand rows
// rendered promptly, not a hang or a panic.
func TestBounds_ALargeMinimumRendersPromptly(t *testing.T) {
	f := boundsField(t, `"minItems": 1000, "maxItems": 1000,`)
	html := mi.RenderToString(renderRepeatable(context.Background(), &FormEngine{}, f, nil, false, RenderContext{}))
	if rowCount(html) != 1000 {
		t.Errorf("want 1000 rows, got %d", rowCount(html))
	}
	if disabledCount(html) != 1001 { // 1000 removes and the add button
		t.Errorf("at min == max every control is locked, got %d", disabledCount(html))
	}
}
