package formengine

import (
	"strings"
	"testing"
)

// customRepeatable parses a flat repeatable (one text cell, so it keeps flat
// names) carrying the given extra schema properties, e.g. `"x-seam-accent":"rose"`.
func customRepeatable(t *testing.T, extra string) Field {
	t.Helper()
	if extra != "" {
		extra = "," + extra
	}
	form, err := ParseSchema([]byte(`{"type":"object","properties":{"tags":{"type":"array","title":"Tags","x-seam-input":"repeatable"` + extra + `,
	  "items":{"type":"object","properties":{"tag":{"type":"string","title":"Tag","x-seam-input":"text"}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	return form.Fields[0]
}

func TestRepeatableCustom_DefaultsAreUnchanged(t *testing.T) {
	html := renderRepeatableHTML(customRepeatable(t, ""), nil)
	for _, want := range []string{`class="fe-btn fe-add"`, `class="fe-remove"`, ">add<", ">remove_circle_outline<"} {
		if !strings.Contains(html, want) {
			t.Errorf("default output missing %s:\n%s", want, html)
		}
	}
	for _, bad := range []string{"data-fe-accent", "fe-remove--text", "fe-btn--filled", "fe-btn--link", "fe-btn--icon", "--fe-"} {
		if strings.Contains(html, bad) {
			t.Errorf("default output must not contain %s:\n%s", bad, html)
		}
	}
}

func TestRepeatableCustom_SchemaPropertiesAreParsed(t *testing.T) {
	f := customRepeatable(t, `"x-seam-add-label":"Add tag","x-seam-add-icon":"label","x-seam-add-style":"link",`+
		`"x-seam-remove-icon":"close","x-seam-remove-label":"Delete","x-seam-remove-color":"rose","x-seam-accent":"#7c3aed"`)
	if f.XSeamAddLabel != "Add tag" || f.XSeamAddIcon != "label" || f.XSeamAddStyle != "link" ||
		f.XSeamRemoveIcon != "close" || f.XSeamRemoveLabel != "Delete" || f.XSeamRemoveColor != "rose" || f.XSeamAccent != "#7c3aed" {
		t.Errorf("properties not parsed: %+v", f)
	}
}

func TestRepeatableCustom_AddButtonVariants(t *testing.T) {
	cases := []struct {
		extra string
		want  []string
		not   []string
	}{
		{`"x-seam-add-style":"filled"`, []string{`class="fe-btn fe-add fe-btn--filled"`, ">add<"}, nil},
		{`"x-seam-add-style":"link","x-seam-add-label":"Add tag","x-seam-add-icon":"label"`,
			[]string{`class="fe-btn fe-add fe-btn--link"`, ">label<", ">Add tag<"}, nil},
		{`"x-seam-add-style":"icon","x-seam-add-label":"Attach","x-seam-add-icon":"attach_file"`,
			[]string{`class="fe-btn fe-add fe-btn--icon"`, `aria-label="Attach"`, `title="Attach"`, ">attach_file<"}, []string{">Attach<"}},
		{`"x-seam-add-style":"bogus"`, []string{`class="fe-btn fe-add"`}, []string{"fe-btn--"}},
		{`"x-seam-add-icon":"Bad Icon!"`, []string{">add<"}, []string{"Bad Icon"}},
	}
	for _, c := range cases {
		html := renderRepeatableHTML(customRepeatable(t, c.extra), nil)
		for _, w := range c.want {
			if !strings.Contains(html, w) {
				t.Errorf("%s: missing %s:\n%s", c.extra, w, html)
			}
		}
		for _, n := range c.not {
			if strings.Contains(html, n) {
				t.Errorf("%s: must not contain %s:\n%s", c.extra, n, html)
			}
		}
	}
}

func TestRepeatableCustom_RemoveButton(t *testing.T) {
	html := renderRepeatableHTML(customRepeatable(t,
		`"x-seam-remove-label":"Delete","x-seam-remove-icon":"delete_outline","x-seam-remove-color":"rose"`), nil)
	for _, want := range []string{
		`class="fe-remove fe-remove--text"`, `aria-label="Delete"`, "<span>Delete</span>", ">delete_outline<",
		`--fe-remove:#f43f5e;--fe-remove-hover:#f43f5e`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %s:\n%s", want, html)
		}
	}
	// A colour alone keeps the icon-only button.
	plain := renderRepeatableHTML(customRepeatable(t, `"x-seam-remove-color":"#abc"`), nil)
	if !strings.Contains(plain, `class="fe-remove"`) || strings.Contains(plain, "fe-remove--text") || !strings.Contains(plain, "--fe-remove:#abc") {
		t.Errorf("colour only must keep the icon button:\n%s", plain)
	}
}

func TestRepeatableCustom_AccentOnWrapperAndRejectsUnsafeValues(t *testing.T) {
	html := renderRepeatableHTML(customRepeatable(t, `"x-seam-accent":"Emerald"`), nil)
	if !strings.Contains(html, `data-fe-accent="1"`) || !strings.Contains(html, "--fe-accent:#10b981") {
		t.Errorf("accent missing:\n%s", html)
	}
	for _, bad := range []string{`red;background:url(x)`, `#12345`, `javascript:1`, `rgb(1,2,3)`, `</div>`} {
		out := renderRepeatableHTML(customRepeatable(t, `"x-seam-accent":"`+strings.ReplaceAll(bad, `"`, "")+`","x-seam-remove-color":"`+strings.ReplaceAll(bad, `"`, "")+`"`), nil)
		if strings.Contains(out, "data-fe-accent") || strings.Contains(out, "--fe-accent") || strings.Contains(out, "--fe-remove") || strings.Contains(out, "url(") {
			t.Errorf("unsafe colour %q was copied through:\n%s", bad, out)
		}
	}
}

func TestRepeatableCustom_IndexedAndNestedKeepTheirOwnSettings(t *testing.T) {
	form, err := ParseSchema([]byte(`{"type":"object","properties":{"blocks":{"type":"array","title":"Blocks","x-seam-input":"repeatable",
	  "x-seam-accent":"#7c3aed","x-seam-remove-label":"Remove block","x-seam-add-label":"Add block",
	  "items":{"type":"object","properties":{
	    "name":{"type":"string","title":"Name","x-seam-input":"text"},
	    "lines":{"type":"array","title":"Lines","x-seam-input":"repeatable","x-seam-accent":"teal","x-seam-add-style":"link",
	      "items":{"type":"object","properties":{"line":{"type":"string","title":"Line","x-seam-input":"text"}}}}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	html := renderRepeatableHTML(form.Fields[0], nil)
	if strings.Count(html, `data-fe-accent="1"`) < 2 {
		t.Errorf("outer and nested repeatables must each carry their accent:\n%s", html)
	}
	for _, want := range []string{"--fe-accent:#7c3aed", "--fe-accent:#14b8a6", ">Add block<", "fe-btn--link"} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %s:\n%s", want, html)
		}
	}
	// The row template used by the add button carries the custom remove button too.
	tmpl := html[strings.Index(html, "<template>"):]
	if !strings.Contains(tmpl, "fe-remove--text") || !strings.Contains(tmpl, "Remove block") {
		t.Errorf("the <template> row must carry the custom remove button:\n%s", tmpl)
	}
}

func TestRepColorAndRepIcon(t *testing.T) {
	for in, want := range map[string]string{
		"emerald": "#10b981", " ROSE ": "#f43f5e", "#ABC": "#abc", "#7C3AED": "#7c3aed",
		"": "", "#12345": "", "#gggggg": "", "red": "", "javascript:1": "", "#fff;x": "",
	} {
		if got := repColor(in); got != want {
			t.Errorf("repColor(%q) = %q, want %q", in, got, want)
		}
	}
	for _, c := range []struct{ in, def, want string }{
		{"delete_outline", "x", "delete_outline"}, {"", "add", "add"}, {"Bad Icon", "add", "add"},
		{"a;b", "add", "add"}, {strings.Repeat("a", 41), "add", "add"},
	} {
		if got := repIcon(c.in, c.def); got != c.want {
			t.Errorf("repIcon(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
