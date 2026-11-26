package designer

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

const flatDoc = `{"type":"object","required":["name"],"properties":{"name":{"type":"string","title":"Name","x-seam-input":"text"},"code":{"type":"string","x-seam-input":"text"},"notes":{"type":"string","x-seam-input":"textarea"}}}`

// compactOf is the document without the indentation Bytes adds.
func compactOf(d *Doc) string {
	var b bytes.Buffer
	if err := json.Compact(&b, d.Bytes()); err != nil {
		return string(d.Bytes())
	}
	return b.String()
}

func layoutOf(t *testing.T, d *Doc) []LayoutTab {
	t.Helper()
	l, err := d.Layout()
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func fieldsOf(t *testing.T, d *Doc, tab, section string) []string {
	t.Helper()
	for _, lt := range layoutOf(t, d) {
		if lt.ID != tab {
			continue
		}
		for _, r := range lt.Rows {
			for _, s := range r.Sections {
				if s.ID == section {
					return s.Fields
				}
			}
		}
	}
	t.Fatalf("no section %s/%s", tab, section)
	return nil
}

func TestLayout_FromFlatToTabsAndBack(t *testing.T) {
	d := mustParseDoc(t, flatDoc)
	if d.HasLayout() || layoutOf(t, d) != nil {
		t.Fatalf("a flat form has no layout")
	}
	// the first tab takes every field, so the page shows what it showed
	if err := d.AddTab("general", "General"); err != nil {
		t.Fatal(err)
	}
	if got := fieldsOf(t, d, "general", "general-main"); !reflect.DeepEqual(got, []string{"name", "code", "notes"}) {
		t.Fatalf("the first tab holds every field: %v", got)
	}
	engineAccepts(t, d)
	// a second tab, a section, and fields moved about
	if err := d.AddTab("more", "More"); err != nil {
		t.Fatal(err)
	}
	if err := d.AddSection("general", "codes", "Codes"); err != nil {
		t.Fatal(err)
	}
	if err := d.PlaceField("code", "general", "codes", ""); err != nil {
		t.Fatal(err)
	}
	if err := d.PlaceField("notes", "more", "more-main", ""); err != nil {
		t.Fatal(err)
	}
	if a, b, c := fieldsOf(t, d, "general", "general-main"), fieldsOf(t, d, "general", "codes"), fieldsOf(t, d, "more", "more-main"); !reflect.DeepEqual(a, []string{"name"}) || !reflect.DeepEqual(b, []string{"code"}) || !reflect.DeepEqual(c, []string{"notes"}) {
		t.Fatalf("placed: %v %v %v", a, b, c)
	}
	engineAccepts(t, d)
	// a new field joins the last section; a rename and a removal keep the sections in step
	if err := d.AddField("extra", "text"); err != nil {
		t.Fatal(err)
	}
	if err := d.Rename("code", "sku"); err != nil {
		t.Fatal(err)
	}
	if err := d.RemoveField("notes"); err != nil {
		t.Fatal(err)
	}
	if b, c := fieldsOf(t, d, "general", "codes"), fieldsOf(t, d, "more", "more-main"); !reflect.DeepEqual(b, []string{"sku"}) || !reflect.DeepEqual(c, []string{"extra"}) {
		t.Fatalf("in step: %v %v", b, c)
	}
	engineAccepts(t, d)
	// before another field, and within the section
	if err := d.PlaceField("extra", "general", "general-main", "name"); err != nil {
		t.Fatal(err)
	}
	if err := d.MoveFieldInSection("extra", 1); err != nil {
		t.Fatal(err)
	}
	if got := fieldsOf(t, d, "general", "general-main"); !reflect.DeepEqual(got, []string{"name", "extra"}) {
		t.Fatalf("ordered: %v", got)
	}
	// a section removed hands its fields to the one before it
	if err := d.RemoveSection("general", "codes"); err != nil {
		t.Fatal(err)
	}
	if got := fieldsOf(t, d, "general", "general-main"); !reflect.DeepEqual(got, []string{"name", "extra", "sku"}) {
		t.Fatalf("handed over: %v", got)
	}
	// tabs: renamed, reordered, removed; the last one gone, the form is flat again
	if err := d.SetTab("more", "extra-tab", "Extras"); err != nil {
		t.Fatal(err)
	}
	if err := d.MoveTab("extra-tab", -1); err != nil {
		t.Fatal(err)
	}
	if l := layoutOf(t, d); l[0].ID != "extra-tab" || l[0].Label != "Extras" || l[1].ID != "general" {
		t.Fatalf("tabs: %+v", l)
	}
	if err := d.RemoveTab("general"); err != nil {
		t.Fatal(err)
	}
	if got := fieldsOf(t, d, "extra-tab", "more-main"); !reflect.DeepEqual(got, []string{"name", "extra", "sku"}) {
		t.Fatalf("a removed tab hands its fields over: %v", got)
	}
	engineAccepts(t, d)
	if err := d.RemoveTab("extra-tab"); err != nil {
		t.Fatal(err)
	}
	if d.HasLayout() || strings.Contains(compactOf(d), "x-seam-form") {
		t.Errorf("with no tab left the form is flat and carries no layout: %s", d.Bytes())
	}
	engineAccepts(t, d)
}

func TestLayout_RowsSideBySide(t *testing.T) {
	d := mustParseDoc(t, flatDoc)
	for _, err := range []error{d.AddTab("t", "T"), d.AddSection("t", "left", "Left"), d.AddSection("t", "right", "Right"),
		d.PlaceField("code", "t", "left", ""), d.PlaceField("notes", "t", "right", "")} {
		if err != nil {
			t.Fatal(err)
		}
	}
	if !strings.Contains(compactOf(d), `"sections":[{`) || strings.Contains(compactOf(d), "section_defs") {
		t.Fatalf("one section a row is the plain spelling: %s", d.Bytes())
	}
	// right sits beside left: the rows spelling
	if err := d.MoveSection("t", "right", "join"); err != nil {
		t.Fatal(err)
	}
	l := layoutOf(t, d)
	if len(l[0].Rows) != 2 || len(l[0].Rows[1].Sections) != 2 || l[0].Rows[1].Sections[1].ID != "right" {
		t.Fatalf("joined: %+v", l[0].Rows)
	}
	if !strings.Contains(compactOf(d), "section_defs") {
		t.Fatalf("two sections in a row need the rows spelling: %s", d.Bytes())
	}
	engineAccepts(t, d)
	// widths and a layout
	if err := d.SetSection("t", "left", "Left side", "2-col", "2/3"); err != nil {
		t.Fatal(err)
	}
	l = layoutOf(t, d)
	if s := l[0].Rows[1].Sections[0]; s.Label != "Left side" || s.Layout != "2-col" || s.Width != "2/3" {
		t.Fatalf("set: %+v", s)
	}
	engineAccepts(t, d)
	// a third and no fourth
	for _, err := range []error{d.AddSection("t", "third", ""), d.MoveSection("t", "third", "join"), d.AddSection("t", "fourth", "")} {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := d.MoveSection("t", "fourth", "join"); !errors.Is(err, ErrLayout) {
		t.Errorf("a row holds three sections at most: %v", err)
	}
	// split, up and down
	if err := d.MoveSection("t", "right", "split"); err != nil {
		t.Fatal(err)
	}
	if err := d.MoveSection("t", "right", "up"); err != nil {
		t.Fatal(err)
	}
	order := func() string {
		var ids []string
		for _, r := range layoutOf(t, d)[0].Rows {
			var row []string
			for _, s := range r.Sections {
				row = append(row, s.ID)
			}
			ids = append(ids, strings.Join(row, "+"))
		}
		return strings.Join(ids, " | ")
	}
	if got := order(); got != "t-main | right | left+third | fourth" {
		t.Errorf("rows after split and up: %s", got)
	}
	if err := d.MoveSection("t", "t-main", "down"); err != nil {
		t.Fatal(err)
	}
	if got := order(); got != "right | t-main | left+third | fourth" {
		t.Errorf("rows after down: %s", got)
	}
	engineAccepts(t, d)
	// back to one section a row: the plain spelling again
	for _, err := range []error{d.MoveSection("t", "third", "split"), d.SetSection("t", "left", "Left side", "1-col", "")} {
		if err != nil {
			t.Fatal(err)
		}
	}
	if strings.Contains(compactOf(d), "section_defs") {
		t.Errorf("with one section a row the plain spelling returns: %s", d.Bytes())
	}
	engineAccepts(t, d)
}

func TestLayout_KeepsWhatItDoesNotEdit(t *testing.T) {
	src := `{"type":"object","properties":{"a":{"type":"string","x-seam-input":"text"},"b":{"type":"string","x-seam-input":"text"}},` +
		`"x-seam-form":{"module":"m","name":"n","tabs":[{"id":"one","label":"One","custom":{"k":1},"rows":[{"sections":["s1","s2"],"widths":["1/2","1/2"],"visible_when":{"field":"a","eq":"x"}}],` +
		`"section_defs":[{"id":"s1","label":"S1","attachable":true,"fields":["a"]},{"id":"s2","label":"S2","fields":["b"]}],"details":[{"id":"d","label":"D","source":"/x/{id}"}]}]}}`
	d := mustParseDoc(t, src)
	l := layoutOf(t, d)
	if len(l) != 1 || len(l[0].Rows) != 1 || len(l[0].Rows[0].Sections) != 2 || l[0].Rows[0].Sections[0].Width != "1/2" || !strings.Contains(l[0].Details, `"source":"/x/{id}"`) {
		t.Fatalf("read: %+v", l)
	}
	if err := d.SetSection("one", "s2", "Second", "1-col", "1/2"); err != nil {
		t.Fatal(err)
	}
	out := compactOf(d)
	for _, keep := range []string{`"module":"m"`, `"name":"n"`, `"custom":{"k":1}`, `"attachable":true`, `"visible_when":{"field":"a","eq":"x"}`, `"details":[{"id":"d","label":"D","source":"/x/{id}"}]`, `"label":"Second"`} {
		if !strings.Contains(out, keep) {
			t.Errorf("lost %s in %s", keep, out)
		}
	}
	engineAccepts(t, d)
	// details and embeds are replaced as lists of objects, or removed
	if err := d.SetTabExtras("one", `[ {"id":"d2","label":"D2","source":"/y/{id}"} ]`, ""); err != nil {
		t.Fatal(err)
	}
	if l := layoutOf(t, d); l[0].Details != `[{"id":"d2","label":"D2","source":"/y/{id}"}]` || l[0].Embeds != "" {
		t.Errorf("extras: %+v", l[0])
	}
	if err := d.SetTabExtras("one", "", "[]"); err != nil || strings.Contains(compactOf(d), "details") || strings.Contains(compactOf(d), "embeds") {
		t.Errorf("an empty list removes the key: %v %s", err, d.Bytes())
	}
}

func TestLayout_RefusalsChangeNothing(t *testing.T) {
	d := mustParseDoc(t, flatDoc)
	if err := d.AddTab("t", "T"); err != nil {
		t.Fatal(err)
	}
	before := compactOf(d)
	for name, err := range map[string]error{
		"a bad tab ID":              d.AddTab("Bad ID", "x"),
		"a taken tab ID":            d.AddTab("t", "x"),
		"a label with a line break": d.AddTab("u", "a\nb"),
		"a section in no tab":       d.AddSection("nope", "s", ""),
		"a taken section ID":        d.AddSection("t", "t-main", ""),
		"a bad section ID":          d.AddSection("t", "../x", ""),
		"an unknown layout":         d.SetSection("t", "t-main", "", "9-col", ""),
		"an unknown width":          d.SetSection("t", "t-main", "", "1-col", "5/7"),
		"an unknown section":        d.SetSection("t", "nope", "", "1-col", ""),
		"an unknown move":           d.MoveSection("t", "t-main", "sideways"),
		"joining the first":         d.MoveSection("t", "t-main", "join"),
		"the only section":          d.RemoveSection("t", "t-main"),
		"a field that is not there": d.PlaceField("zzz", "t", "t-main", ""),
		"a place that is not there": d.PlaceField("name", "t", "nope", ""),
		"a tab that is not there":   d.RemoveTab("nope"),
		"details that are no list":  d.SetTabExtras("t", `{"id":"d"}`, ""),
		"embeds of no objects":      d.SetTabExtras("t", "", `["x"]`),
		"a field in no section":     d.MoveFieldInSection("zzz", 1),
	} {
		if !errors.Is(err, ErrLayout) && !errors.Is(err, ErrField) {
			t.Errorf("%s must be refused, got %v", name, err)
		}
	}
	if compactOf(d) != before {
		t.Errorf("a refusal must change nothing:\n%s\n%s", before, d.Bytes())
	}
}
