package designer

import (
	"fmt"
	"regexp"
	"strconv"
)

// Duplicating blocks. A field's name is unique in a form and a block's ID is
// unique in its layout, so a copy is the same thing under the next free name:
// "phone" gives "phone_2", then "phone_3"; a section "contact" gives
// "contact-2". The copy of a section or a tab holds copies of its fields, not
// the fields themselves, since a field has one place. Labels are copied as
// they are: telling the two apart is the next edit.

var (
	fieldCopyRE = regexp.MustCompile(`^(.*)_(\d+)$`)
	blockCopyRE = regexp.MustCompile(`^(.*)-(\d+)$`)
)

// nextFree returns stem+sep+n for the first n, from 2, that taken refuses;
// a name that already ends in a number counts on from it.
func nextFree(name, sep string, re *regexp.Regexp, limit int, taken func(string) bool) (string, error) {
	stem, n := name, 2
	if m := re.FindStringSubmatch(name); m != nil {
		if k, err := strconv.Atoi(m[2]); err == nil && k >= 2 && k < 10000 {
			stem, n = m[1], k+1
		}
	}
	for ; n < 10000; n++ {
		c := stem + sep + strconv.Itoa(n)
		if limit > 0 && len(c) > limit {
			break
		}
		if !taken(c) {
			return c, nil
		}
	}
	return "", fmt.Errorf("%w: no free name for a copy of %q", ErrField, name)
}

// copyField adds a copy of a field after it, with its required mark, and
// returns the copy's name. The copy has no place in the layout yet.
func (d *Doc) copyField(name string) (string, error) {
	raw, ok := d.Field(name)
	if !ok {
		return "", fmt.Errorf("%w: no field %q", ErrField, name)
	}
	to, err := nextFree(name, "_", fieldCopyRE, 64, d.Has)
	if err != nil {
		return "", err
	}
	required := false
	at := -1
	for i, f := range d.Fields() {
		if f.Name == name {
			at, required = i, f.Required
		}
	}
	props := d.properties()
	props.set(to, append([]byte(nil), raw...))
	d.putProperties(props)
	if back := len(d.Fields()) - 2 - at; back > 0 {
		if err := d.MoveField(to, -back); err != nil {
			return "", err
		}
	}
	if required {
		if err := d.SetRequired(to, true); err != nil {
			return "", err
		}
	}
	return to, nil
}

// placeOf finds the section a field is in, and the field after it there.
func placeOf(tabs []LayoutTab, name string) (tab, section, next string, ok bool) {
	for _, t := range tabs {
		for _, r := range t.Rows {
			for _, s := range r.Sections {
				for i, f := range s.Fields {
					if f != name {
						continue
					}
					if i+1 < len(s.Fields) {
						next = s.Fields[i+1]
					}
					return t.ID, s.ID, next, true
				}
			}
		}
	}
	return "", "", "", false
}

// DuplicateField adds a copy of a field right after it: in the list, and in
// its section when the form has a layout. It returns the copy's name.
func (d *Doc) DuplicateField(name string) (string, error) {
	var to string
	err := d.atomically(func() error {
		var err error
		if to, err = d.copyField(name); err != nil {
			return err
		}
		tabs, err := d.Layout()
		if err != nil {
			return err
		}
		if tab, sec, next, ok := placeOf(tabs, name); ok {
			return d.PlaceField(to, tab, sec, next)
		}
		return nil
	})
	return to, err
}

func (d *Doc) sectionTaken(id string) bool {
	_, tabs, err := d.tabs()
	return err != nil || sectionIDs(tabs)[id]
}

// copySection adds to tab `into` a copy of a section, with copies of its
// fields, as a row of its own at the end, and returns the copy's ID.
func (d *Doc) copySection(into string, src LayoutSection) (string, error) {
	id, err := nextFree(src.ID, "-", blockCopyRE, 0, d.sectionTaken)
	if err != nil {
		return "", err
	}
	if err := d.AddSection(into, id, src.Label); err != nil {
		return "", err
	}
	if err := d.SetSection(into, id, src.Label, src.Layout, ""); err != nil {
		return "", err
	}
	for _, f := range src.Fields {
		if !d.Has(f) {
			continue // a name the layout lists and the form does not have
		}
		c, err := d.copyField(f)
		if err != nil {
			return "", err
		}
		if err := d.PlaceField(c, into, id, ""); err != nil {
			return "", err
		}
	}
	return id, nil
}

// DuplicateSection adds, at the end of the tab, a copy of a section holding
// copies of its fields. It returns the copy's ID.
func (d *Doc) DuplicateSection(tabID, id string) (string, error) {
	var to string
	err := d.atomically(func() error {
		tabs, err := d.Layout()
		if err != nil {
			return err
		}
		for _, t := range tabs {
			if t.ID != tabID {
				continue
			}
			for _, r := range t.Rows {
				for _, s := range r.Sections {
					if s.ID == id {
						to, err = d.copySection(tabID, s)
						return err
					}
				}
			}
		}
		return fmt.Errorf("%w: no section %q in tab %q", ErrLayout, id, tabID)
	})
	return to, err
}

// DuplicateTab adds, after the last tab, a copy of a tab: its sections in
// the same rows and widths, copies of their fields, and its details and
// embeds. It returns the copy's ID.
func (d *Doc) DuplicateTab(id string) (string, error) {
	var to string
	err := d.atomically(func() error {
		tabs, err := d.Layout()
		if err != nil {
			return err
		}
		var src *LayoutTab
		taken := map[string]bool{}
		for i := range tabs {
			taken[tabs[i].ID] = true
			if tabs[i].ID == id {
				src = &tabs[i]
			}
		}
		if src == nil {
			return fmt.Errorf("%w: no tab %q", ErrLayout, id)
		}
		if to, err = nextFree(id, "-", blockCopyRE, 0, func(c string) bool { return taken[c] }); err != nil {
			return err
		}
		if err := d.AddTab(to, src.Label); err != nil {
			return err
		}
		// AddTab gives the new tab a section of its own; it goes once the
		// copies are in
		made, err := d.Layout()
		if err != nil {
			return err
		}
		placeholder := ""
		for _, t := range made {
			if t.ID == to && len(t.Rows) > 0 && len(t.Rows[0].Sections) > 0 {
				placeholder = t.Rows[0].Sections[0].ID
			}
		}
		copied := 0
		for _, r := range src.Rows {
			var ids []string
			for i, s := range r.Sections {
				c, err := d.copySection(to, s)
				if err != nil {
					return err
				}
				copied++
				ids = append(ids, c)
				if i > 0 {
					if err := d.MoveSection(to, c, "join"); err != nil {
						return err
					}
				}
			}
			for i, s := range r.Sections {
				if len(ids) > 1 && s.Width != "" {
					if err := d.SetSection(to, ids[i], s.Label, s.Layout, s.Width); err != nil {
						return err
					}
				}
			}
		}
		if copied > 0 && placeholder != "" {
			if err := d.RemoveSection(to, placeholder); err != nil {
				return err
			}
		}
		if src.Details != "" || src.Embeds != "" {
			return d.SetTabExtras(to, src.Details, src.Embeds)
		}
		return nil
	})
	return to, err
}
