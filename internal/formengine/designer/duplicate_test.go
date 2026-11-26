package designer

import (
	"errors"
	"reflect"
	"testing"
)

func namesOf(d *Doc) []string {
	var out []string
	for _, f := range d.Fields() {
		out = append(out, f.Name)
	}
	return out
}

func TestDuplicateField_FlatForm(t *testing.T) {
	d, err := Parse([]byte(flatDoc))
	if err != nil {
		t.Fatal(err)
	}
	to, err := d.DuplicateField("name")
	if err != nil || to != "name_2" {
		t.Fatalf("copy = %q, %v", to, err)
	}
	if got := namesOf(d); !reflect.DeepEqual(got, []string{"name", "name_2", "code", "notes"}) {
		t.Errorf("the copy follows its original: %v", got)
	}
	a, _ := d.Field("name")
	b, _ := d.Field("name_2")
	if string(a) != string(b) {
		t.Errorf("the copy has the original's definition: %s / %s", a, b)
	}
	for _, f := range d.Fields() {
		if f.Name == "name_2" && !f.Required {
			t.Errorf("the copy of a required field is required")
		}
	}
	// a copy of a copy counts on, and a taken name is skipped
	if to, _ := d.DuplicateField("name_2"); to != "name_3" {
		t.Errorf("copy of name_2 = %q, want name_3", to)
	}
	if to, _ := d.DuplicateField("name"); to != "name_4" {
		t.Errorf("third copy of name = %q, want name_4", to)
	}
	if _, err := d.DuplicateField("nothing"); !errors.Is(err, ErrField) {
		t.Errorf("an unknown field is refused: %v", err)
	}
}

func TestDuplicateField_InItsSection(t *testing.T) {
	d, _ := Parse([]byte(flatDoc))
	if err := d.AddTab("general", "General"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.DuplicateField("name"); err != nil {
		t.Fatal(err)
	}
	if got := fieldsOf(t, d, "general", "general-main"); !reflect.DeepEqual(got, []string{"name", "name_2", "code", "notes"}) {
		t.Errorf("the copy sits after its original in the section: %v", got)
	}
}

func TestDuplicateSection_CopiesItsFields(t *testing.T) {
	d, _ := Parse([]byte(flatDoc))
	if err := d.AddTab("general", "General"); err != nil {
		t.Fatal(err)
	}
	if err := d.SetSection("general", "general-main", "Main", "2-col", ""); err != nil {
		t.Fatal(err)
	}
	to, err := d.DuplicateSection("general", "general-main")
	if err != nil || to != "general-main-2" {
		t.Fatalf("copy = %q, %v", to, err)
	}
	if got := fieldsOf(t, d, "general", to); !reflect.DeepEqual(got, []string{"name_2", "code_2", "notes_2"}) {
		t.Errorf("the copy holds copies of the fields: %v", got)
	}
	if got := fieldsOf(t, d, "general", "general-main"); !reflect.DeepEqual(got, []string{"name", "code", "notes"}) {
		t.Errorf("the original keeps its fields: %v", got)
	}
	for _, r := range layoutOf(t, d)[0].Rows {
		for _, s := range r.Sections {
			if s.ID == to && (s.Label != "Main" || s.Layout != "2-col") {
				t.Errorf("the copy has the label and layout: %+v", s)
			}
		}
	}
	before := compactOf(d)
	if _, err := d.DuplicateSection("general", "nothing"); !errors.Is(err, ErrLayout) || compactOf(d) != before {
		t.Errorf("an unknown section is refused and nothing changes: %v", err)
	}
}

func TestDuplicateTab_KeepsRowsAndExtras(t *testing.T) {
	d, _ := Parse([]byte(flatDoc))
	if err := d.AddTab("general", "General"); err != nil {
		t.Fatal(err)
	}
	if err := d.AddSection("general", "extra", "Extra"); err != nil {
		t.Fatal(err)
	}
	if err := d.PlaceField("notes", "general", "extra", ""); err != nil {
		t.Fatal(err)
	}
	if err := d.MoveSection("general", "extra", "join"); err != nil {
		t.Fatal(err)
	}
	if err := d.SetTabExtras("general", `[{"id":"history","source":"x"}]`, ""); err != nil {
		t.Fatal(err)
	}
	to, err := d.DuplicateTab("general")
	if err != nil || to != "general-2" {
		t.Fatalf("copy = %q, %v", to, err)
	}
	l := layoutOf(t, d)
	if len(l) != 2 || l[1].ID != "general-2" || l[1].Label != "General" {
		t.Fatalf("tabs: %+v", l)
	}
	c := l[1]
	if len(c.Rows) != 1 || len(c.Rows[0].Sections) != 2 {
		t.Fatalf("the copy keeps the two sections side by side and has no section of its own left: %+v", c.Rows)
	}
	if got := c.Rows[0].Sections[0].Fields; !reflect.DeepEqual(got, []string{"name_2", "code_2"}) {
		t.Errorf("first section: %v", got)
	}
	if got := c.Rows[0].Sections[1].Fields; !reflect.DeepEqual(got, []string{"notes_2"}) {
		t.Errorf("second section: %v", got)
	}
	if c.Details != l[0].Details || c.Details == "" {
		t.Errorf("the details are copied: %q", c.Details)
	}
	if len(namesOf(d)) != 6 {
		t.Errorf("six fields: %v", namesOf(d))
	}
	before := compactOf(d)
	if _, err := d.DuplicateTab("nothing"); !errors.Is(err, ErrLayout) || compactOf(d) != before {
		t.Errorf("an unknown tab is refused and nothing changes: %v", err)
	}
}
