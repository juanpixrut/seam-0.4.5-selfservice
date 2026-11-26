package designer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// The designer's document model (campaign DEV-21). A form definition is edited
// as an ordered tree of JSON, never decoded into a struct and encoded again, so
// that what the designer does not understand it does not lose: key order, custom
// keys and the parts of the document it has no editor for (such as x-seam-form)
// come back out exactly as they went in, bar whitespace. Every operation checks
// its own input; whether the result is a form the engine accepts is for the
// caller to confirm with formengine.ParseSchema, which the designer page does
// after each operation.

// MaxDocBytes bounds a document the designer will open.
const MaxDocBytes = 1 << 20

var (
	// ErrDoc is returned for a document that cannot be edited.
	ErrDoc = errors.New("designer: not an editable form definition")
	// ErrField is returned for a field name that is not valid, is taken, or is missing.
	ErrField = errors.New("designer: bad field")
	// ErrProp is returned for a property the designer will not set.
	ErrProp = errors.New("designer: bad property")
	// ErrReferenced is returned when a field is named by x-seam-form, which the
	// designer does not yet edit, so removing or renaming it would break the layout.
	ErrReferenced = errors.New("designer: the field is used by the form layout")
)

var fieldNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
var iconNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
var hexColour = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// obj is a JSON object with its keys in document order and its values raw.
type obj struct {
	keys []string
	vals map[string]json.RawMessage
}

func newObj() *obj { return &obj{vals: map[string]json.RawMessage{}} }

func parseObj(data []byte) (*obj, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if d, ok := tok.(json.Delim); err != nil || !ok || d != '{' {
		return nil, fmt.Errorf("%w: not a JSON object", ErrDoc)
	}
	o := newObj()
	for dec.More() {
		kt, err := dec.Token()
		key, ok := kt.(string)
		if err != nil || !ok {
			return nil, fmt.Errorf("%w: bad key", ErrDoc)
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrDoc, err)
		}
		if _, dup := o.vals[key]; dup {
			return nil, fmt.Errorf("%w: the key %q appears twice", ErrDoc, key)
		}
		o.keys = append(o.keys, key)
		o.vals[key] = raw
	}
	if _, err := dec.Token(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDoc, err)
	}
	if dec.More() {
		return nil, fmt.Errorf("%w: text after the object", ErrDoc)
	}
	return o, nil
}

func (o *obj) set(k string, raw json.RawMessage) {
	if _, ok := o.vals[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.vals[k] = raw
}

func (o *obj) del(k string) {
	if _, ok := o.vals[k]; !ok {
		return
	}
	delete(o.vals, k)
	for i, x := range o.keys {
		if x == k {
			o.keys = append(o.keys[:i], o.keys[i+1:]...)
			break
		}
	}
}

func (o *obj) marshal() json.RawMessage {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		b.Write(kb)
		b.WriteByte(':')
		b.Write(o.vals[k])
	}
	b.WriteByte('}')
	return b.Bytes()
}

// Doc is a form definition being edited.
type Doc struct {
	root *obj
}

// Parse opens a form definition.
func Parse(data []byte) (*Doc, error) {
	if len(data) > MaxDocBytes {
		return nil, fmt.Errorf("%w: larger than %d bytes", ErrDoc, MaxDocBytes)
	}
	root, err := parseObj(bytes.TrimSpace(data))
	if err != nil {
		return nil, err
	}
	return &Doc{root: root}, nil
}

// Bytes is the document, indented, with its keys in their original order.
func (d *Doc) Bytes() []byte {
	var out bytes.Buffer
	if err := json.Indent(&out, d.root.marshal(), "", "  "); err != nil {
		return d.root.marshal()
	}
	out.WriteByte('\n')
	return out.Bytes()
}

func (d *Doc) properties() *obj {
	if raw, ok := d.root.vals["properties"]; ok {
		if o, err := parseObj(raw); err == nil {
			return o
		}
	}
	return newObj()
}

func (d *Doc) putProperties(o *obj) { d.root.set("properties", o.marshal()) }

func (d *Doc) required() []string {
	var r []string
	if raw, ok := d.root.vals["required"]; ok {
		_ = json.Unmarshal(raw, &r)
	}
	return r
}

func (d *Doc) putRequired(r []string) {
	if len(r) == 0 {
		d.root.del("required")
		return
	}
	b, _ := json.Marshal(r)
	d.root.set("required", b)
}

// FieldInfo describes one field of the form.
type FieldInfo struct {
	Name     string
	Widget   string
	Title    string
	Required bool
}

// Fields lists the form's fields in order.
func (d *Doc) Fields() []FieldInfo {
	req := map[string]bool{}
	for _, r := range d.required() {
		req[r] = true
	}
	props := d.properties()
	out := make([]FieldInfo, 0, len(props.keys))
	for _, k := range props.keys {
		var f struct {
			Title string `json:"title"`
		}
		_ = json.Unmarshal(props.vals[k], &f)
		out = append(out, FieldInfo{Name: k, Widget: WidgetOf(props.vals[k]), Title: f.Title, Required: req[k]})
	}
	return out
}

// Field returns a field's property object as it is in the document.
func (d *Doc) Field(name string) (json.RawMessage, bool) {
	raw, ok := d.properties().vals[name]
	return raw, ok
}

// Has reports whether the form has the field.
func (d *Doc) Has(name string) bool { _, ok := d.Field(name); return ok }

// referenced reports whether x-seam-form names the field anywhere.
func (d *Doc) referenced(name string) bool {
	raw, ok := d.root.vals["x-seam-form"]
	if !ok {
		return false
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	for {
		tok, err := dec.Token()
		if err != nil {
			return false
		}
		if s, ok := tok.(string); ok && s == name {
			return true
		}
	}
}

// AddField appends a field of the given widget, built from the widget's template.
func (d *Doc) AddField(name, widget string) error {
	if !fieldNamePattern.MatchString(name) {
		return fmt.Errorf("%w: %q must be lower-case letters, digits and underscores, starting with a letter", ErrField, name)
	}
	w, ok := LookupWidget(widget)
	if !ok {
		return fmt.Errorf("%w: unknown widget %q", ErrField, widget)
	}
	props := d.properties()
	if _, taken := props.vals[name]; taken {
		return fmt.Errorf("%w: %q is already a field", ErrField, name)
	}
	props.set(name, json.RawMessage(w.Template))
	d.putProperties(props)
	d.layoutAdd(name) // a form with tabs shows only what a section lists
	return nil
}

// RemoveField deletes a field, and its entry in required.
func (d *Doc) RemoveField(name string) error {
	props := d.properties()
	if _, ok := props.vals[name]; !ok {
		return fmt.Errorf("%w: no field %q", ErrField, name)
	}
	// it leaves its section; anything else of the layout that names it (a
	// visibility condition) still stops the removal
	saved := d.Bytes()
	d.layoutRemove(name)
	if d.referenced(name) {
		d.restore(saved)
		return fmt.Errorf("%w: %q", ErrReferenced, name)
	}
	props.del(name)
	d.putProperties(props)
	var keep []string
	for _, r := range d.required() {
		if r != name {
			keep = append(keep, r)
		}
	}
	d.putRequired(keep)
	return nil
}

// MoveField moves a field by delta places (negative is up), stopping at the ends.
func (d *Doc) MoveField(name string, delta int) error {
	props := d.properties()
	at := -1
	for i, k := range props.keys {
		if k == name {
			at = i
		}
	}
	if at < 0 {
		return fmt.Errorf("%w: no field %q", ErrField, name)
	}
	to := at + delta
	if to < 0 {
		to = 0
	}
	if to > len(props.keys)-1 {
		to = len(props.keys) - 1
	}
	if to == at {
		return nil
	}
	keys := append([]string(nil), props.keys...)
	keys = append(keys[:at], keys[at+1:]...)
	keys = append(keys[:to], append([]string{name}, keys[to:]...)...)
	props.keys = keys
	d.putProperties(props)
	return nil
}

// SetRequired marks a field required or not.
func (d *Doc) SetRequired(name string, on bool) error {
	if !d.Has(name) {
		return fmt.Errorf("%w: no field %q", ErrField, name)
	}
	var out []string
	has := false
	for _, r := range d.required() {
		if r == name {
			has = true
			if !on {
				continue
			}
		}
		out = append(out, r)
	}
	if on && !has {
		out = append(out, name)
	}
	d.putRequired(out)
	return nil
}

// Rename changes a field's name, keeping its place, and its entry in required.
func (d *Doc) Rename(oldName, newName string) error {
	if !fieldNamePattern.MatchString(newName) {
		return fmt.Errorf("%w: %q is not a valid name", ErrField, newName)
	}
	props := d.properties()
	if _, ok := props.vals[oldName]; !ok {
		return fmt.Errorf("%w: no field %q", ErrField, oldName)
	}
	if oldName == newName {
		return nil
	}
	if _, taken := props.vals[newName]; taken {
		return fmt.Errorf("%w: %q is already a field", ErrField, newName)
	}
	// its section follows the new name; anything else of the layout that
	// names it (a visibility condition) still stops the rename
	saved := d.Bytes()
	d.layoutRename(oldName, newName)
	if d.referenced(oldName) {
		d.restore(saved)
		return fmt.Errorf("%w: %q", ErrReferenced, oldName)
	}
	for i, k := range props.keys {
		if k == oldName {
			props.keys[i] = newName
		}
	}
	props.vals[newName] = props.vals[oldName]
	delete(props.vals, oldName)
	d.putProperties(props)
	req := d.required()
	for i, r := range req {
		if r == oldName {
			req[i] = newName
		}
	}
	d.putRequired(req)
	return nil
}

// SetProps sets and clears properties of a field. Each key must be the title or
// a catalogue property that applies to the field's widget, and each value valid
// JSON; every other key of the field, custom ones included, is left alone.
func (d *Doc) SetProps(name string, set map[string]json.RawMessage, unset []string) error {
	props := d.properties()
	raw, ok := props.vals[name]
	if !ok {
		return fmt.Errorf("%w: no field %q", ErrField, name)
	}
	field, err := parseObj(raw)
	if err != nil {
		return err
	}
	widget := WidgetOf(raw)
	allowed := map[string]bool{"title": true}
	for _, p := range PropsFor(widget) {
		allowed[p.Key] = true
	}
	for k, v := range set {
		if !allowed[k] {
			return fmt.Errorf("%w: %q does not apply to a %s field", ErrProp, k, widget)
		}
		if !json.Valid(v) {
			return fmt.Errorf("%w: the value of %q is not JSON", ErrProp, k)
		}
	}
	for _, k := range unset {
		if !allowed[k] {
			return fmt.Errorf("%w: %q does not apply to a %s field", ErrProp, k, widget)
		}
	}
	// in catalogue order, so a property that is new lands where it is expected
	for _, k := range keysInOrder(set) {
		field.set(k, set[k])
	}
	for _, k := range unset {
		field.del(k)
	}
	props.set(name, field.marshal())
	d.putProperties(props)
	return nil
}

func keysInOrder(m map[string]json.RawMessage) []string {
	var out []string
	if _, ok := m["title"]; ok {
		out = append(out, "title")
	}
	for _, p := range props {
		if _, ok := m[p.Key]; ok {
			out = append(out, p.Key)
		}
	}
	return out
}

// ValueFromText turns what was typed in the inspector into the JSON of a
// property. unset reports that the property is to be removed: an empty text, or
// a boolean that is off.
func ValueFromText(p Prop, text string) (raw json.RawMessage, unset bool, err error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, true, nil
	}
	if utf8.RuneCountInString(text) > 4000 {
		return nil, false, fmt.Errorf("%w: %s is too long", ErrProp, p.Title)
	}
	for _, r := range text {
		if r == 0 || (r < 0x20 && r != '\n' && r != '\r' && r != '\t') {
			return nil, false, fmt.Errorf("%w: %s has a control character", ErrProp, p.Title)
		}
	}
	bad := func(what string) (json.RawMessage, bool, error) {
		return nil, false, fmt.Errorf("%w: %s must be %s", ErrProp, p.Title, what)
	}
	switch p.Kind {
	case KindBool:
		switch strings.ToLower(text) {
		case "on", "true", "1", "yes":
			return json.RawMessage("true"), false, nil
		case "off", "false", "0", "no":
			return nil, true, nil
		}
		return bad("on or off")
	case KindInt:
		n, e := strconv.Atoi(text)
		if e != nil || n < 0 || n > 1000000 {
			return bad("a whole number from 0 to 1000000")
		}
		return json.RawMessage(strconv.Itoa(n)), false, nil
	case KindString:
		b, _ := json.Marshal(text)
		return b, false, nil
	case KindEnum:
		for _, v := range p.Enum {
			if v == text {
				b, _ := json.Marshal(v)
				return b, false, nil
			}
		}
		return bad("one of " + strings.Join(p.Enum, ", "))
	case KindColor:
		ok := hexColour.MatchString(text)
		for _, v := range p.Enum {
			ok = ok || v == text
		}
		if !ok {
			return bad("a colour name or #rrggbb")
		}
		b, _ := json.Marshal(text)
		return b, false, nil
	case KindIcon:
		if !iconNamePattern.MatchString(text) {
			return bad("an icon name: lower-case letters, digits and underscores")
		}
		b, _ := json.Marshal(text)
		return b, false, nil
	case KindList:
		var items []string
		for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				if len(line) > 200 {
					return bad("lines of at most 200 characters")
				}
				items = append(items, line)
			}
		}
		if len(items) == 0 {
			return nil, true, nil
		}
		if len(items) > 100 {
			return bad("at most 100 lines")
		}
		b, _ := json.Marshal(items)
		return b, false, nil
	case KindMap:
		m := newObj()
		for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
			if line = strings.TrimSpace(line); line == "" {
				continue
			}
			k, v, found := strings.Cut(line, "=")
			k, v = strings.TrimSpace(k), strings.TrimSpace(v)
			if !found || k == "" || len(k) > 200 || !iconNamePattern.MatchString(v) {
				return bad("lines of value=icon_name")
			}
			if _, dup := m.vals[k]; dup {
				return bad("without a value repeated")
			}
			kb, _ := json.Marshal(v)
			m.set(k, kb)
			if len(m.keys) > 1000 {
				return bad("at most 1000 lines")
			}
		}
		if len(m.keys) == 0 {
			return nil, true, nil
		}
		return m.marshal(), false, nil
	case KindJSON:
		var v interface{}
		if json.Unmarshal([]byte(text), &v) != nil {
			return bad("valid JSON")
		}
		var c bytes.Buffer
		_ = json.Compact(&c, []byte(text))
		return c.Bytes(), false, nil
	}
	return bad("a supported kind")
}

// TextFromValue is the inverse of ValueFromText: what the inspector shows for a
// property's current JSON.
func TextFromValue(p Prop, raw json.RawMessage) string {
	switch p.Kind {
	case KindBool:
		if string(raw) == "true" {
			return "on"
		}
		return ""
	case KindInt:
		return strings.TrimSpace(string(raw))
	case KindString, KindEnum, KindColor, KindIcon:
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return s
		}
	case KindList:
		var items []string
		if json.Unmarshal(raw, &items) == nil {
			return strings.Join(items, "\n")
		}
	case KindMap:
		if o, err := parseObj(raw); err == nil {
			var lines []string
			for _, k := range o.keys {
				var v string
				_ = json.Unmarshal(o.vals[k], &v)
				lines = append(lines, k+"="+v)
			}
			return strings.Join(lines, "\n")
		}
	case KindJSON:
		var c bytes.Buffer
		if json.Compact(&c, raw) == nil {
			return c.String()
		}
	}
	return strings.TrimSpace(string(raw))
}
