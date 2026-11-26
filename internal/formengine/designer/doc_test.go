package designer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"

	fe "github.com/ha1tch/seam-ui/internal/formengine"
)

func compact(t *testing.T, b []byte) string {
	t.Helper()
	var out bytes.Buffer
	if err := json.Compact(&out, b); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, b)
	}
	return out.String()
}

func mustParseDoc(t *testing.T, s string) *Doc {
	t.Helper()
	d, err := Parse([]byte(s))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func engineAccepts(t *testing.T, d *Doc) *fe.Form {
	t.Helper()
	f, err := fe.ParseSchema(d.Bytes())
	if err != nil {
		t.Fatalf("the engine refuses the result: %v\n%s", err, d.Bytes())
	}
	return f
}

func names(d *Doc) string {
	var n []string
	for _, f := range d.Fields() {
		n = append(n, f.Name)
	}
	return strings.Join(n, ",")
}

// What the designer does not understand it does not lose: every real form comes
// back out with its keys in their order and its content intact.
func TestDoc_IsLosslessOnEveryEmbeddedSchema(t *testing.T) {
	n := 0
	for _, file := range embeddedSchemaFiles(t) {
		data, _ := os.ReadFile(file)
		d, err := Parse(data)
		if err != nil {
			continue // not an object form
		}
		n++
		if compact(t, d.Bytes()) != compact(t, data) {
			t.Errorf("%s changed in a round trip", file)
		}
	}
	if n < 20 {
		t.Errorf("only %d documents were checked", n)
	}
}

func TestDoc_EveryWidgetCanBeAddedAndTheEngineAcceptsIt(t *testing.T) {
	d := mustParseDoc(t, `{"type":"object","properties":{}}`)
	for _, w := range widgets {
		if err := d.AddField("f_"+strings.ReplaceAll(w.Name, "-", "_"), w.Name); err != nil {
			t.Fatalf("%s: %v", w.Name, err)
		}
	}
	form := engineAccepts(t, d)
	if len(form.Fields) != len(widgets) || len(d.Fields()) != len(widgets) {
		t.Fatalf("%d fields, want %d", len(form.Fields), len(widgets))
	}
	for i, f := range d.Fields() {
		if f.Widget != widgets[i].Name {
			t.Errorf("field %d is a %s, want %s", i, f.Widget, widgets[i].Name)
		}
	}
	html := mi.RenderToString((&fe.FormEngine{}).Render(context.Background(), d.Bytes(), nil, nil, fe.RenderContext{Locale: "en"}))
	for _, f := range d.Fields() {
		if !strings.Contains(html, f.Name) {
			t.Errorf("%s is not drawn", f.Name)
		}
	}
}

func TestDoc_FieldOperations(t *testing.T) {
	d := mustParseDoc(t, `{"type":"object","title":"Kept","x-custom":{"a":1},"properties":{"a":{"type":"string","x-seam-input":"text"},"b":{"type":"string","x-seam-input":"text"},"c":{"type":"boolean"}},"required":["a","c"]}`)
	if names(d) != "a,b,c" {
		t.Fatal(names(d))
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(d.MoveField("c", -1))
	if names(d) != "a,c,b" {
		t.Errorf("up: %s", names(d))
	}
	must(d.MoveField("a", 5))
	if names(d) != "c,b,a" {
		t.Errorf("down past the end stops at the end: %s", names(d))
	}
	must(d.MoveField("c", -9))
	must(d.MoveField("c", 0))
	if names(d) != "c,b,a" {
		t.Errorf("up past the start stays at the start: %s", names(d))
	}
	must(d.SetRequired("b", true))
	must(d.SetRequired("b", true))
	must(d.SetRequired("a", false))
	req := map[string]bool{}
	for _, f := range d.Fields() {
		req[f.Name] = f.Required
	}
	if !req["b"] || req["a"] || !req["c"] {
		t.Errorf("required: %v", req)
	}
	must(d.Rename("b", "beta"))
	if names(d) != "c,beta,a" || !d.Fields()[1].Required {
		t.Errorf("a rename keeps the place and the required mark: %s %+v", names(d), d.Fields()[1])
	}
	must(d.RemoveField("c"))
	if names(d) != "beta,a" {
		t.Errorf("remove: %s", names(d))
	}
	for _, f := range d.Fields() {
		if f.Name == "c" {
			t.Errorf("c is gone")
		}
	}
	var root map[string]json.RawMessage
	_ = json.Unmarshal(d.Bytes(), &root)
	if string(root["x-custom"]) == "" || !strings.Contains(string(root["title"]), "Kept") {
		t.Errorf("the keys the designer does not edit are kept: %s", d.Bytes())
	}
	if strings.Contains(string(root["required"]), `"c"`) {
		t.Errorf("a removed field leaves required: %s", root["required"])
	}
	engineAccepts(t, d)
}

func TestDoc_RefusalsLeaveTheDocumentAlone(t *testing.T) {
	d := mustParseDoc(t, `{"type":"object","properties":{"a":{"type":"string","x-seam-input":"text"}},"x-seam-form":{"tabs":[{"visible_when":{"field":"a","eq":"x"},"sections":[{"fields":["a"]}]}]}}`)
	// "a" is in a section, which the designer keeps in step, and in a
	// visibility condition, which it does not edit: that still stops it
	before := string(d.Bytes())
	for name, err := range map[string]error{
		"add a bad name":     d.AddField("A b", "text"),
		"add a path":         d.AddField("../x", "text"),
		"add an empty name":  d.AddField("", "text"),
		"add a long name":    d.AddField(strings.Repeat("a", 65), "text"),
		"add a taken name":   d.AddField("a", "text"),
		"add an unknown one": d.AddField("b", "no-such-widget"),
		"remove a missing":   d.RemoveField("zzz"),
		"remove a used one":  d.RemoveField("a"),
		"rename a used one":  d.Rename("a", "b"),
		"rename to a bad":    d.Rename("a", "B"),
		"move a missing":     d.MoveField("zzz", 1),
		"require a missing":  d.SetRequired("zzz", true),
		"set on a missing":   d.SetProps("zzz", map[string]json.RawMessage{"title": []byte(`"x"`)}, nil),
		"set a foreign key":  d.SetProps("a", map[string]json.RawMessage{"x-seam-rows": []byte(`3`)}, nil),
		"set bad json":       d.SetProps("a", map[string]json.RawMessage{"title": []byte(`{bad`)}, nil),
		"unset a foreign":    d.SetProps("a", nil, []string{"minItems"}),
	} {
		if err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
	if !errors.Is(d.RemoveField("a"), ErrReferenced) {
		t.Errorf("a field the layout names says so")
	}
	if string(d.Bytes()) != before {
		t.Errorf("a refusal must change nothing")
	}
}

func TestDoc_SetPropsKeepsCustomKeysAndPlacesNewOnesInOrder(t *testing.T) {
	d := mustParseDoc(t, `{"properties":{"t":{"type":"string","x-seam-input":"text","title":"T","line_items":{"keep":true},"zz-custom":1}}}`)
	err := d.SetProps("t", map[string]json.RawMessage{"x-seam-help": []byte(`"Help"`), "title": []byte(`"New"`), "maxLength": []byte(`40`)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := d.Field("t")
	s := compact(t, raw)
	for _, want := range []string{`"line_items":{"keep":true}`, `"zz-custom":1`, `"title":"New"`, `"x-seam-help":"Help"`, `"maxLength":40`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s in %s", want, s)
		}
	}
	if !(strings.Index(s, `"title"`) < strings.Index(s, `"line_items"`) && strings.Index(s, `"zz-custom"`) < strings.Index(s, `"title":"New"`) == false) {
		// the existing title stays where it was; new keys come after what was there
	}
	if strings.Index(s, `"x-seam-help"`) < strings.Index(s, `"zz-custom"`) {
		t.Errorf("a new property goes after the existing ones: %s", s)
	}
	if err := d.SetProps("t", nil, []string{"x-seam-help", "maxLength"}); err != nil {
		t.Fatal(err)
	}
	raw, _ = d.Field("t")
	if strings.Contains(string(raw), "x-seam-help") || !strings.Contains(string(raw), "line_items") {
		t.Errorf("unset removes only what was asked: %s", raw)
	}
}

func TestDoc_ParseRefusals(t *testing.T) {
	for name, in := range map[string]string{"an array": `[1]`, "a scalar": `5`, "empty": ``, "a duplicate key": `{"a":1,"a":2}`,
		"trailing text": `{"a":1} x`, "unterminated": `{"a":`, "too large": `{"x":"` + strings.Repeat("a", MaxDocBytes) + `"}`} {
		if _, err := Parse([]byte(in)); !errors.Is(err, ErrDoc) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if d, err := Parse([]byte(`  {"type":"object"}  `)); err != nil || len(d.Fields()) != 0 {
		t.Errorf("a form with no properties is a form with no fields: %v", err)
	}
}

func TestValueFromText_EveryKindAndHostileInput(t *testing.T) {
	prop := func(k Kind, enum ...string) Prop { return Prop{Key: "k", Title: "K", Kind: k, Enum: enum} }
	type c struct {
		p     Prop
		in    string
		want  string
		unset bool
		bad   bool
	}
	for i, x := range []c{
		{prop(KindBool), "on", `true`, false, false}, {prop(KindBool), "off", "", true, false}, {prop(KindBool), "maybe", "", false, true}, {prop(KindBool), "", "", true, false},
		{prop(KindInt), "12", `12`, false, false}, {prop(KindInt), "0", `0`, false, false}, {prop(KindInt), "-1", "", false, true}, {prop(KindInt), "1.5", "", false, true}, {prop(KindInt), "99999999999", "", false, true}, {prop(KindInt), "1e3", "", false, true},
		{prop(KindString), `say "hi" <b>`, `"say \"hi\" \u003cb\u003e"`, false, false}, {prop(KindString), "a\x00b", "", false, true}, {prop(KindString), strings.Repeat("x", 4001), "", false, true},
		{prop(KindEnum, "a", "b"), "b", `"b"`, false, false}, {prop(KindEnum, "a", "b"), "c", "", false, true}, {prop(KindEnum, "a", "b"), "A", "", false, true},
		{prop(KindColor, "rose"), "rose", `"rose"`, false, false}, {prop(KindColor, "rose"), "#a1b", `"#a1b"`, false, false}, {prop(KindColor, "rose"), "#a1b2c3", `"#a1b2c3"`, false, false}, {prop(KindColor, "rose"), "javascript:1", "", false, true}, {prop(KindColor, "rose"), "#12", "", false, true},
		{prop(KindIcon), "check_circle", `"check_circle"`, false, false}, {prop(KindIcon), "Check", "", false, true}, {prop(KindIcon), `"><script>`, "", false, true},
		{prop(KindList), "a\n b \n\n c", `["a","b","c"]`, false, false}, {prop(KindList), "\n \n", "", true, false}, {prop(KindList), strings.Repeat("x", 201), "", false, true},
		{prop(KindMap), "low=arrow_down\nhigh = priority_high", `{"low":"arrow_down","high":"priority_high"}`, false, false}, {prop(KindMap), "low", "", false, true}, {prop(KindMap), "low=Bad Icon", "", false, true}, {prop(KindMap), "a=x\na=y", "", false, true}, {prop(KindMap), "=x", "", false, true},
		{prop(KindJSON), `{"field":"a",  "equals":"b"}`, `{"field":"a","equals":"b"}`, false, false}, {prop(KindJSON), `{bad`, "", false, true},
	} {
		raw, unset, err := ValueFromText(x.p, x.in)
		switch {
		case x.bad && err == nil:
			t.Errorf("#%d %s %q must be refused, gave %s", i, x.p.Kind, x.in, raw)
		case !x.bad && err != nil:
			t.Errorf("#%d %s %q: %v", i, x.p.Kind, x.in, err)
		case !x.bad && unset != x.unset:
			t.Errorf("#%d %s %q: unset=%v want %v", i, x.p.Kind, x.in, unset, x.unset)
		case !x.bad && !x.unset && string(raw) != x.want:
			t.Errorf("#%d %s %q: %s, want %s", i, x.p.Kind, x.in, raw, x.want)
		}
		if !x.bad && !x.unset && err == nil && !json.Valid(raw) {
			t.Errorf("#%d produced invalid JSON: %s", i, raw)
		}
	}
	// what the inspector shows for a value is what, typed back, gives the value
	for _, x := range []struct {
		p   Prop
		raw string
	}{{prop(KindBool), `true`}, {prop(KindInt), `7`}, {prop(KindString), `"a b"`}, {prop(KindEnum, "x"), `"x"`}, {prop(KindList), `["a","b"]`},
		{prop(KindMap), `{"low":"a_b","high":"c_d"}`}, {prop(KindJSON), `{"a":[1,2]}`}, {prop(KindIcon), `"star"`}} {
		text := TextFromValue(x.p, json.RawMessage(x.raw))
		back, _, err := ValueFromText(x.p, text)
		if err != nil || string(back) != x.raw {
			t.Errorf("%s %s -> %q -> %s (%v)", x.p.Kind, x.raw, text, back, err)
		}
	}
}
