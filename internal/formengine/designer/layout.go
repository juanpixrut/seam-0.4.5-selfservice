package designer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// The layout of a form: the block tree above its fields (FORM_ENGINE_SPEC
// section 3, ADMIN_MODULE_SPEC section 8). x-seam-form.tabs is a list of tabs;
// a tab is a list of rows; a row holds one to three sections side by side; a
// section lists fields. A tab may also carry detail panels and embedded forms.
//
// The engine reads two spellings of a tab: "sections" (each section a row of
// its own) and "rows" with "section_defs" (rows that name their sections). The
// designer reads both into one model and writes whichever the tab needs: the
// plain spelling while every row has one section and nothing of its own, the
// rows spelling otherwise. Everything it does not edit is kept as it was: the
// other keys of x-seam-form, of a tab, of a section and of a row (visibility
// conditions among them) pass through untouched, in their order.
//
// A form without tabs is flat: the engine renders its fields in order. The
// first tab created takes every field into one section, so nothing disappears
// from the page; removing the last tab makes the form flat again. While a form
// has tabs every field belongs to exactly one section: a new field joins the
// last section, and a section that is removed hands its fields to a neighbour.

// ErrLayout is wrapped by every refusal of a layout operation.
var ErrLayout = errors.New("designer: bad layout")

// MaxRowSections is how many sections a row holds (FORM_ENGINE_SPEC 3.8).
const MaxRowSections = 3

var blockIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

// SectionLayouts are the layouts a section may have; the first is the default.
var SectionLayouts = []string{"1-col", "2-col", "2-col-left", "2-col-right"}

// RowWidths are the widths a section may take in a row; "" lets the engine split.
var RowWidths = []string{"", "full", "1/2", "1/3", "2/3", "1/4", "3/4"}

// LayoutSection is one section as the designer shows it.
type LayoutSection struct {
	ID, Label, Layout string
	Width             string // its width in the row, "" for automatic
	Fields            []string
}

// LayoutRow is one row of a tab.
type LayoutRow struct{ Sections []LayoutSection }

// LayoutTab is one tab. Details and Embeds are the tab's detail panels and
// embedded forms as JSON arrays ("" when it has none).
type LayoutTab struct {
	ID, Label       string
	Rows            []LayoutRow
	Details, Embeds string
}

// lSection, lRow and lTab are the same tree with every block's own object
// kept, so what is not edited survives.
type lSection struct {
	o     *obj
	width string
}

type lRow struct {
	o        *obj // the row's own object in the rows spelling; nil for a plain section
	sections []*lSection
}

type lTab struct {
	o    *obj
	rows []*lRow
}

func strOf(o *obj, key string) string {
	var s string
	if raw, ok := o.vals[key]; ok {
		_ = json.Unmarshal(raw, &s)
	}
	return s
}

func listOf(o *obj, key string) []string {
	var out []string
	if raw, ok := o.vals[key]; ok {
		_ = json.Unmarshal(raw, &out)
	}
	return out
}

func setStr(o *obj, key, value string) {
	raw, _ := json.Marshal(value)
	o.set(key, raw)
}

func setList(o *obj, key string, values []string) {
	if values == nil {
		values = []string{}
	}
	raw, _ := json.Marshal(values)
	o.set(key, raw)
}

func rawArray(items []json.RawMessage) json.RawMessage {
	var b strings.Builder
	b.WriteByte('[')
	for i, it := range items {
		if i > 0 {
			b.WriteByte(',')
		}
		b.Write(it)
	}
	b.WriteByte(']')
	return json.RawMessage(b.String())
}

// form returns x-seam-form as an object (a new one when the document has none).
func (d *Doc) form() (*obj, error) {
	raw, ok := d.root.vals["x-seam-form"]
	if !ok {
		return newObj(), nil
	}
	o, err := parseObj(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: x-seam-form is not an object", ErrLayout)
	}
	return o, nil
}

// tabs reads the layout.
func (d *Doc) tabs() (*obj, []*lTab, error) {
	form, err := d.form()
	if err != nil {
		return nil, nil, err
	}
	var raws []json.RawMessage
	if raw, ok := form.vals["tabs"]; ok {
		if json.Unmarshal(raw, &raws) != nil {
			return nil, nil, fmt.Errorf("%w: x-seam-form.tabs is not a list", ErrLayout)
		}
	}
	var out []*lTab
	for _, tr := range raws {
		to, err := parseObj(tr)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: a tab is not an object", ErrLayout)
		}
		tab := &lTab{o: to}
		var rowRaws, secRaws, defRaws []json.RawMessage
		_ = json.Unmarshal(to.vals["rows"], &rowRaws)
		_ = json.Unmarshal(to.vals["sections"], &secRaws)
		_ = json.Unmarshal(to.vals["section_defs"], &defRaws)
		if len(rowRaws) > 0 {
			defs := map[string]*obj{}
			for _, dr := range defRaws {
				so, err := parseObj(dr)
				if err != nil {
					return nil, nil, fmt.Errorf("%w: tab %q: a section is not an object", ErrLayout, strOf(to, "id"))
				}
				defs[strOf(so, "id")] = so
			}
			for _, rr := range rowRaws {
				ro, err := parseObj(rr)
				if err != nil {
					return nil, nil, fmt.Errorf("%w: tab %q: a row is not an object", ErrLayout, strOf(to, "id"))
				}
				row := &lRow{o: ro}
				widths := listOf(ro, "widths")
				for i, sid := range listOf(ro, "sections") {
					so, ok := defs[sid]
					if !ok {
						return nil, nil, fmt.Errorf("%w: tab %q: a row names the section %q, which is not defined", ErrLayout, strOf(to, "id"), sid)
					}
					sec := &lSection{o: so}
					if i < len(widths) {
						sec.width = widths[i]
					}
					row.sections = append(row.sections, sec)
				}
				tab.rows = append(tab.rows, row)
			}
		} else {
			for _, sr := range secRaws {
				so, err := parseObj(sr)
				if err != nil {
					return nil, nil, fmt.Errorf("%w: tab %q: a section is not an object", ErrLayout, strOf(to, "id"))
				}
				tab.rows = append(tab.rows, &lRow{sections: []*lSection{{o: so}}})
			}
		}
		out = append(out, tab)
	}
	return form, out, nil
}

// putTabs writes the layout back; with no tabs the form is flat again.
func (d *Doc) putTabs(form *obj, tabs []*lTab) {
	if len(tabs) == 0 {
		form.del("tabs")
		if len(form.keys) == 0 {
			d.root.del("x-seam-form")
		} else {
			d.root.set("x-seam-form", form.marshal())
		}
		return
	}
	var tabRaws []json.RawMessage
	for _, tab := range tabs {
		plain := true
		for _, row := range tab.rows {
			if len(row.sections) != 1 || row.sections[0].width != "" {
				plain = false
			}
			if row.o != nil {
				for _, k := range row.o.keys {
					if k != "sections" && k != "widths" {
						plain = false // the row has something of its own (a condition)
					}
				}
			}
		}
		if plain {
			var secs []json.RawMessage
			for _, row := range tab.rows {
				secs = append(secs, row.sections[0].o.marshal())
			}
			tab.o.del("rows")
			tab.o.del("section_defs")
			tab.o.set("sections", rawArray(secs))
		} else {
			var rows, defs []json.RawMessage
			for _, row := range tab.rows {
				ro := row.o
				if ro == nil {
					ro = newObj()
				}
				var ids, widths []string
				anyWidth := false
				for _, sec := range row.sections {
					ids = append(ids, strOf(sec.o, "id"))
					widths = append(widths, sec.width)
					anyWidth = anyWidth || sec.width != ""
					defs = append(defs, sec.o.marshal())
				}
				setList(ro, "sections", ids)
				if anyWidth {
					for i, w := range widths {
						if w == "" {
							widths[i] = "full"
							if len(widths) == 2 {
								widths[i] = "1/2"
							} else if len(widths) == 3 {
								widths[i] = "1/3"
							}
						}
					}
					setList(ro, "widths", widths)
				} else {
					ro.del("widths")
				}
				rows = append(rows, ro.marshal())
			}
			tab.o.del("sections")
			tab.o.set("rows", rawArray(rows))
			tab.o.set("section_defs", rawArray(defs))
		}
		tabRaws = append(tabRaws, tab.o.marshal())
	}
	form.set("tabs", rawArray(tabRaws))
	d.root.set("x-seam-form", form.marshal())
}

// Layout returns the form's layout; nil when the form is flat.
func (d *Doc) Layout() ([]LayoutTab, error) {
	_, tabs, err := d.tabs()
	if err != nil {
		return nil, err
	}
	var out []LayoutTab
	for _, tab := range tabs {
		lt := LayoutTab{ID: strOf(tab.o, "id"), Label: strOf(tab.o, "label")}
		if raw, ok := tab.o.vals["details"]; ok {
			lt.Details = string(raw)
		}
		if raw, ok := tab.o.vals["embeds"]; ok {
			lt.Embeds = string(raw)
		}
		for _, row := range tab.rows {
			var lr LayoutRow
			for _, sec := range row.sections {
				layout := strOf(sec.o, "layout")
				if layout == "" {
					layout = SectionLayouts[0]
				}
				lr.Sections = append(lr.Sections, LayoutSection{ID: strOf(sec.o, "id"), Label: strOf(sec.o, "label"), Layout: layout,
					Width: sec.width, Fields: listOf(sec.o, "fields")})
			}
			lt.Rows = append(lt.Rows, lr)
		}
		out = append(out, lt)
	}
	return out, nil
}

// HasLayout reports whether the form has tabs.
func (d *Doc) HasLayout() bool {
	_, tabs, err := d.tabs()
	return err == nil && len(tabs) > 0
}

// atomically runs a layout change and puts the document back as it was when
// the change is refused.
func (d *Doc) atomically(change func() error) error {
	saved := d.Bytes()
	if err := change(); err != nil {
		if back, perr := Parse(saved); perr == nil {
			*d = *back
		}
		return err
	}
	return nil
}

func findTab(tabs []*lTab, id string) (int, *lTab) {
	for i, t := range tabs {
		if strOf(t.o, "id") == id {
			return i, t
		}
	}
	return -1, nil
}

// findSection returns the row and position of a section of the tab.
func findSection(tab *lTab, id string) (row, pos int) {
	for r, lr := range tab.rows {
		for p, sec := range lr.sections {
			if strOf(sec.o, "id") == id {
				return r, p
			}
		}
	}
	return -1, -1
}

func sectionIDs(tabs []*lTab) map[string]bool {
	out := map[string]bool{}
	for _, t := range tabs {
		for _, r := range t.rows {
			for _, s := range r.sections {
				out[strOf(s.o, "id")] = true
			}
		}
	}
	return out
}

func newSection(id, label string, fields []string) *lSection {
	o := newObj()
	setStr(o, "id", id)
	setStr(o, "label", label)
	setList(o, "fields", fields)
	return &lSection{o: o}
}

func checkBlock(kind, id, label string) error {
	if !blockIDPattern.MatchString(id) {
		return fmt.Errorf("%w: the %s ID %q must be lower-case letters, digits, hyphens and underscores, starting with a letter", ErrLayout, kind, id)
	}
	if len(label) > 200 || strings.ContainsAny(label, "\x00\r\n") {
		return fmt.Errorf("%w: the %s label is too long or has line breaks", ErrLayout, kind)
	}
	return nil
}

// lastSection is where a field with no place goes.
func lastSection(tabs []*lTab) *lSection {
	for i := len(tabs) - 1; i >= 0; i-- {
		rows := tabs[i].rows
		for r := len(rows) - 1; r >= 0; r-- {
			if n := len(rows[r].sections); n > 0 {
				return rows[r].sections[n-1]
			}
		}
	}
	return nil
}

// AddTab adds a tab with one empty section. The first tab of a flat form
// takes every field, so the form shows what it showed.
func (d *Doc) AddTab(id, label string) error {
	return d.atomically(func() error {
		if err := checkBlock("tab", id, label); err != nil {
			return err
		}
		form, tabs, err := d.tabs()
		if err != nil {
			return err
		}
		if i, _ := findTab(tabs, id); i >= 0 {
			return fmt.Errorf("%w: there is already a tab %q", ErrLayout, id)
		}
		secID := id + "-main"
		for n := 2; sectionIDs(tabs)[secID]; n++ {
			secID = fmt.Sprintf("%s-main-%d", id, n)
		}
		var fields []string
		if len(tabs) == 0 {
			for _, f := range d.Fields() {
				fields = append(fields, f.Name)
			}
		}
		to := newObj()
		setStr(to, "id", id)
		setStr(to, "label", label)
		tabs = append(tabs, &lTab{o: to, rows: []*lRow{{sections: []*lSection{newSection(secID, "", fields)}}}})
		d.putTabs(form, tabs)
		return nil
	})
}

// SetTab changes a tab's label and, when newID differs, its ID.
func (d *Doc) SetTab(id, newID, label string) error {
	return d.atomically(func() error {
		if newID == "" {
			newID = id
		}
		if err := checkBlock("tab", newID, label); err != nil {
			return err
		}
		form, tabs, err := d.tabs()
		if err != nil {
			return err
		}
		_, tab := findTab(tabs, id)
		if tab == nil {
			return fmt.Errorf("%w: no tab %q", ErrLayout, id)
		}
		if newID != id {
			if i, _ := findTab(tabs, newID); i >= 0 {
				return fmt.Errorf("%w: there is already a tab %q", ErrLayout, newID)
			}
			setStr(tab.o, "id", newID)
		}
		setStr(tab.o, "label", label)
		d.putTabs(form, tabs)
		return nil
	})
}

// MoveTab moves a tab one place towards the start (delta < 0) or the end.
func (d *Doc) MoveTab(id string, delta int) error {
	return d.atomically(func() error {
		form, tabs, err := d.tabs()
		if err != nil {
			return err
		}
		i, _ := findTab(tabs, id)
		j := i + sign(delta)
		if i < 0 {
			return fmt.Errorf("%w: no tab %q", ErrLayout, id)
		}
		if j < 0 || j >= len(tabs) {
			return nil
		}
		tabs[i], tabs[j] = tabs[j], tabs[i]
		d.putTabs(form, tabs)
		return nil
	})
}

func sign(n int) int {
	if n < 0 {
		return -1
	}
	return 1
}

// RemoveTab removes a tab. Its fields move to the last section of the tab
// before it (or after it); removing the only tab makes the form flat.
func (d *Doc) RemoveTab(id string) error {
	return d.atomically(func() error {
		form, tabs, err := d.tabs()
		if err != nil {
			return err
		}
		i, tab := findTab(tabs, id)
		if tab == nil {
			return fmt.Errorf("%w: no tab %q", ErrLayout, id)
		}
		var fields []string
		for _, r := range tab.rows {
			for _, s := range r.sections {
				fields = append(fields, listOf(s.o, "fields")...)
			}
		}
		tabs = append(tabs[:i], tabs[i+1:]...)
		if len(tabs) > 0 && len(fields) > 0 {
			neighbour := i - 1
			if neighbour < 0 {
				neighbour = 0
			}
			dst := lastSection(tabs[neighbour : neighbour+1])
			if dst == nil {
				dst = lastSection(tabs)
			}
			if dst == nil {
				return fmt.Errorf("%w: the fields of %q have no section to move to; remove them or add a section first", ErrLayout, id)
			}
			setList(dst.o, "fields", append(listOf(dst.o, "fields"), fields...))
		}
		d.putTabs(form, tabs)
		return nil
	})
}

// AddSection adds an empty section to a tab, in a row of its own at the end.
func (d *Doc) AddSection(tabID, id, label string) error {
	return d.atomically(func() error {
		if err := checkBlock("section", id, label); err != nil {
			return err
		}
		form, tabs, err := d.tabs()
		if err != nil {
			return err
		}
		_, tab := findTab(tabs, tabID)
		if tab == nil {
			return fmt.Errorf("%w: no tab %q", ErrLayout, tabID)
		}
		if sectionIDs(tabs)[id] {
			return fmt.Errorf("%w: there is already a section %q", ErrLayout, id)
		}
		tab.rows = append(tab.rows, &lRow{sections: []*lSection{newSection(id, label, nil)}})
		d.putTabs(form, tabs)
		return nil
	})
}

// SetSection changes a section's label, layout and width in its row.
func (d *Doc) SetSection(tabID, id, label, layout, width string) error {
	return d.atomically(func() error {
		if err := checkBlock("section", id, label); err != nil {
			return err
		}
		if !inList(SectionLayouts, layout) {
			return fmt.Errorf("%w: %q is not a section layout", ErrLayout, layout)
		}
		if !inList(RowWidths, width) {
			return fmt.Errorf("%w: %q is not a width", ErrLayout, width)
		}
		form, tabs, err := d.tabs()
		if err != nil {
			return err
		}
		_, tab := findTab(tabs, tabID)
		if tab == nil {
			return fmt.Errorf("%w: no tab %q", ErrLayout, tabID)
		}
		r, p := findSection(tab, id)
		if r < 0 {
			return fmt.Errorf("%w: no section %q in tab %q", ErrLayout, id, tabID)
		}
		sec := tab.rows[r].sections[p]
		setStr(sec.o, "label", label)
		if layout == SectionLayouts[0] {
			sec.o.del("layout")
		} else {
			setStr(sec.o, "layout", layout)
		}
		sec.width = width
		d.putTabs(form, tabs)
		return nil
	})
}

func inList(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// MoveSection moves a section within its tab. "up" and "down" move it one row
// (into a row of its own); "join" puts it beside the section before it, in
// that section's row; "split" gives it a row of its own after its row.
func (d *Doc) MoveSection(tabID, id, how string) error {
	return d.atomically(func() error {
		form, tabs, err := d.tabs()
		if err != nil {
			return err
		}
		_, tab := findTab(tabs, tabID)
		if tab == nil {
			return fmt.Errorf("%w: no tab %q", ErrLayout, tabID)
		}
		r, p := findSection(tab, id)
		if r < 0 {
			return fmt.Errorf("%w: no section %q in tab %q", ErrLayout, id, tabID)
		}
		row := tab.rows[r]
		sec := row.sections[p]
		alone := len(row.sections) == 1
		detach := func() { // take the section out of its row; an emptied row goes
			row.sections = append(row.sections[:p], row.sections[p+1:]...)
			if len(row.sections) == 0 {
				tab.rows = append(tab.rows[:r], tab.rows[r+1:]...)
			}
		}
		insertRow := func(at int) {
			sec.width = ""
			nr := &lRow{sections: []*lSection{sec}}
			tab.rows = append(tab.rows, nil)
			copy(tab.rows[at+1:], tab.rows[at:])
			tab.rows[at] = nr
		}
		switch how {
		case "up":
			if alone && r == 0 {
				return nil
			}
			detach()
			if alone {
				insertRow(r - 1)
			} else {
				insertRow(r)
			}
		case "down":
			if alone && r == len(tab.rows)-1 {
				return nil
			}
			detach()
			insertRow(r + 1)
		case "split":
			if alone {
				return nil
			}
			detach()
			insertRow(r + 1)
		case "join":
			if p > 0 {
				return nil // already beside the one before it
			}
			if r == 0 {
				return fmt.Errorf("%w: %q is the first section of the tab: there is none before it to sit beside", ErrLayout, id)
			}
			prev := tab.rows[r-1]
			if len(prev.sections) >= MaxRowSections {
				return fmt.Errorf("%w: a row holds at most %d sections", ErrLayout, MaxRowSections)
			}
			if !alone {
				return fmt.Errorf("%w: split %q from its row first", ErrLayout, id)
			}
			detach()
			sec.width = ""
			for _, s := range prev.sections {
				s.width = ""
			}
			prev.sections = append(prev.sections, sec)
		default:
			return fmt.Errorf("%w: unknown move %q", ErrLayout, how)
		}
		d.putTabs(form, tabs)
		return nil
	})
}

// RemoveSection removes a section. Its fields move to the section before it
// in the tab (or after it); the only section of a tab cannot be removed while
// it has fields: remove the tab, or move the fields.
func (d *Doc) RemoveSection(tabID, id string) error {
	return d.atomically(func() error {
		form, tabs, err := d.tabs()
		if err != nil {
			return err
		}
		_, tab := findTab(tabs, tabID)
		if tab == nil {
			return fmt.Errorf("%w: no tab %q", ErrLayout, tabID)
		}
		r, p := findSection(tab, id)
		if r < 0 {
			return fmt.Errorf("%w: no section %q in tab %q", ErrLayout, id, tabID)
		}
		var flat []*lSection
		at := -1
		for ri, row := range tab.rows {
			for pi, s := range row.sections {
				if ri == r && pi == p {
					at = len(flat)
				}
				flat = append(flat, s)
			}
		}
		fields := listOf(flat[at].o, "fields")
		if len(flat) == 1 {
			if len(fields) > 0 {
				return fmt.Errorf("%w: %q is the only section of the tab and has fields; remove the tab, or move the fields first", ErrLayout, id)
			}
			return fmt.Errorf("%w: a tab needs a section; remove the tab instead", ErrLayout)
		}
		if len(fields) > 0 {
			dst := at - 1
			if dst < 0 {
				dst = 1
			}
			setList(flat[dst].o, "fields", append(listOf(flat[dst].o, "fields"), fields...))
		}
		row := tab.rows[r]
		row.sections = append(row.sections[:p], row.sections[p+1:]...)
		if len(row.sections) == 0 {
			tab.rows = append(tab.rows[:r], tab.rows[r+1:]...)
		} else if len(row.sections) == 1 {
			row.sections[0].width = ""
		}
		d.putTabs(form, tabs)
		return nil
	})
}

// PlaceField puts a field in a section: at the end, or before the field
// `before` of that section. It leaves whatever section it was in.
func (d *Doc) PlaceField(name, tabID, sectionID, before string) error {
	return d.atomically(func() error {
		if !d.Has(name) {
			return fmt.Errorf("%w: no field %q", ErrField, name)
		}
		form, tabs, err := d.tabs()
		if err != nil {
			return err
		}
		_, tab := findTab(tabs, tabID)
		if tab == nil {
			return fmt.Errorf("%w: no tab %q", ErrLayout, tabID)
		}
		r, p := findSection(tab, sectionID)
		if r < 0 {
			return fmt.Errorf("%w: no section %q in tab %q", ErrLayout, sectionID, tabID)
		}
		if before == name {
			return nil
		}
		unplace(tabs, name)
		dst := tab.rows[r].sections[p]
		fields := listOf(dst.o, "fields")
		at := len(fields)
		for i, f := range fields {
			if f == before && before != "" {
				at = i
			}
		}
		fields = append(fields, "")
		copy(fields[at+1:], fields[at:])
		fields[at] = name
		setList(dst.o, "fields", fields)
		d.putTabs(form, tabs)
		return nil
	})
}

// unplace takes a field out of every section.
func unplace(tabs []*lTab, name string) {
	for _, t := range tabs {
		for _, r := range t.rows {
			for _, s := range r.sections {
				fields := listOf(s.o, "fields")
				kept := fields[:0:0]
				for _, f := range fields {
					if f != name {
						kept = append(kept, f)
					}
				}
				if len(kept) != len(fields) {
					setList(s.o, "fields", kept)
				}
			}
		}
	}
}

// MoveFieldInSection moves a field one place within its section.
func (d *Doc) MoveFieldInSection(name string, delta int) error {
	return d.atomically(func() error {
		form, tabs, err := d.tabs()
		if err != nil {
			return err
		}
		for _, t := range tabs {
			for _, r := range t.rows {
				for _, s := range r.sections {
					fields := listOf(s.o, "fields")
					for i, f := range fields {
						if f != name {
							continue
						}
						j := i + sign(delta)
						if j < 0 || j >= len(fields) {
							return nil
						}
						fields[i], fields[j] = fields[j], fields[i]
						setList(s.o, "fields", fields)
						d.putTabs(form, tabs)
						return nil
					}
				}
			}
		}
		return fmt.Errorf("%w: %q is in no section", ErrLayout, name)
	})
}

// SetTabExtras replaces a tab's detail panels and embedded forms, each a JSON
// array of objects ("" or "[]" for none). Their content is the engine's to
// judge: the designer checks only that each is a list of objects.
func (d *Doc) SetTabExtras(tabID, details, embeds string) error {
	return d.atomically(func() error {
		form, tabs, err := d.tabs()
		if err != nil {
			return err
		}
		_, tab := findTab(tabs, tabID)
		if tab == nil {
			return fmt.Errorf("%w: no tab %q", ErrLayout, tabID)
		}
		for key, text := range map[string]string{"details": details, "embeds": embeds} {
			text = strings.TrimSpace(text)
			if text == "" {
				tab.o.del(key)
				continue
			}
			var items []map[string]json.RawMessage
			if json.Unmarshal([]byte(text), &items) != nil {
				return fmt.Errorf("%w: %s must be a JSON list of objects", ErrLayout, key)
			}
			if len(items) == 0 {
				tab.o.del(key)
				continue
			}
			var compact bytes.Buffer
			if err := json.Compact(&compact, []byte(text)); err != nil {
				return fmt.Errorf("%w: %s: %v", ErrLayout, key, err)
			}
			tab.o.set(key, json.RawMessage(compact.Bytes()))
		}
		d.putTabs(form, tabs)
		return nil
	})
}

// layoutAdd puts a new field in the last section of a form with tabs.
func (d *Doc) layoutAdd(name string) {
	form, tabs, err := d.tabs()
	if err != nil || len(tabs) == 0 {
		return
	}
	if dst := lastSection(tabs); dst != nil {
		setList(dst.o, "fields", append(listOf(dst.o, "fields"), name))
		d.putTabs(form, tabs)
	}
}

// layoutRename and layoutRemove keep the sections in step with a field that
// is renamed or removed.
func (d *Doc) layoutRename(oldName, newName string) {
	form, tabs, err := d.tabs()
	if err != nil || len(tabs) == 0 {
		return
	}
	for _, t := range tabs {
		for _, r := range t.rows {
			for _, s := range r.sections {
				fields := listOf(s.o, "fields")
				changed := false
				for i, f := range fields {
					if f == oldName {
						fields[i], changed = newName, true
					}
				}
				if changed {
					setList(s.o, "fields", fields)
				}
			}
		}
	}
	d.putTabs(form, tabs)
}

func (d *Doc) layoutRemove(name string) {
	form, tabs, err := d.tabs()
	if err != nil || len(tabs) == 0 {
		return
	}
	unplace(tabs, name)
	d.putTabs(form, tabs)
}

// restore puts the document back to a saved state.
func (d *Doc) restore(saved []byte) {
	if back, err := Parse(saved); err == nil {
		*d = *back
	}
}
