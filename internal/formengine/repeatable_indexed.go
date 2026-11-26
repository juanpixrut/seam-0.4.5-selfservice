package formengine

import (
	"context"
	"net/url"
	"sort"
	"strconv"
	"strings"

	mi "github.com/ha1tch/minty"
)

// Indexed naming for "repeatable" fields.
//
// The original repeatable convention is flat parallel arrays,
// {field}_{item}[] (see renderRepeatable). It cannot represent a repeatable
// inside a repeatable (which line belongs to which block?) and it cannot host
// widgets whose script looks elements up by id (every row repeats the ids).
// A repeatable therefore switches to INDEXED naming, with names such as
//
//	blocks[0][name]   blocks[0][lines][1][name]
//
// when any of these holds (needsIndexedNaming): x-seam-naming is "indexed", it
// is nested inside an indexed repeatable, or one of its items is itself a
// repeatable or one of the widgets SeamWidgets/SeamPicker initialise from data
// attributes (any-picker, asset-picker, icon-picker, gps, signature, barcode, or a
// text/tel with an input mask): a flat row is cloned live, which would copy an
// already-initialised widget, where an indexed row comes from a pristine template. Every other repeatable keeps the flat names, so
// existing forms and handlers (Asset service configurations) are unchanged.
//
// Rows are added from a pristine <template> rendered next to the rows, not by
// cloning a live row: the template holds one blank row whose names carry an
// index token (__D0__ for the outer level, __D1__ for the next, ...), and
// SeamRepeatable.add (web/static/js/app.js) substitutes a fresh index and
// initialises any pickers in the new row. Indices only grow (a per-container
// counter), so DOM order equals ascending index order and removing a row never
// renumbers the others. A template always exists, so adding after removing
// every row (T-94) and nested rows work without any stash.
//
// ParseIndexedRepeatable / ParseIndexedForm rebuild the structure from the
// posted values.

// needsIndexedNaming reports whether f uses indexed naming (see above).
func needsIndexedNaming(f Field) bool {
	if f.indexedNaming || f.XSeamNaming == "indexed" {
		return true
	}
	for _, item := range f.Items {
		switch inferWidget(item) {
		case "repeatable", "any-picker", "asset-picker", "icon-picker", "query-picker", "gps", "signature", "barcode":
			return true
		}
		if item.XSeamInputMask != "" {
			return true
		}
	}
	return false
}

// parentName is the repeatable's original field name (not the bracketed
// submission name), for i18n keys.
func parentName(f Field) string {
	if f.I18nName != "" {
		return f.I18nName
	}
	return f.Name
}

func indexToken(depth int) string { return "__D" + strconv.Itoa(depth) + "__" }

// renderRepeatableIndexed renders a repeatable in indexed mode. f.Name is the
// full base name ("blocks", or "blocks[3][lines]" for a nested one).
func renderRepeatableIndexed(ctx context.Context, e *FormEngine, f Field, value interface{}, hasError bool, rc RenderContext) mi.H {
	return func(b *mi.Builder) mi.Node {
		token := indexToken(f.indexDepth)
		rows := padRows(f, arrayOfMaps(value)) // a blank row when empty; at least minItems rows
		rf := withBoundLocks(f, len(rows))
		var rowNodes []interface{}
		for i, row := range rows {
			rowNodes = append(rowNodes, indexedRow(ctx, e, b, rf, row, strconv.Itoa(i), rc))
		}
		blank := mi.RenderToString(func(b2 *mi.Builder) mi.Node {
			return indexedRow(ctx, e, b2, f, nil, token, rc)
		})

		attrs := []interface{}{
			mi.Attr("data-rep", "1"),
			mi.Attr("data-token", token),
			mi.Attr("data-next", strconv.Itoa(len(rows))),
		}
		attrs = append(attrs, repAccentAttrs(f)...)
		if f.indexedNaming { // nested: indented under a rule (CSS), with a heading
			heading := f.rowLabel
			if heading == "" {
				heading = f.Title
			}
			attrs = append(attrs, mi.Class("fe-rep-nested"))
			if heading != "" {
				attrs = append(attrs, b.Div(mi.Class("fe-rep-heading"), heading))
			}
		}
		rowAttrs := []interface{}{mi.Class("rep-rows space-y-2")}
		rowAttrs = append(rowAttrs, repReorderAttrs(f)...)
		rowAttrs = append(rowAttrs, repBoundsAttrs(f)...)
		if repLayout(f) == "form" {
			rowAttrs = append(rowAttrs, mi.Attr("data-layout", "form"))
		}
		attrs = append(attrs,
			b.Div(append(rowAttrs, rowNodes...)...),
			mi.Raw("<template>"+blank+"</template>"),
			repAddButton(b, e, rc, rf, "SeamRepeatable.add(this)"),
		)
		return b.Div(attrs...)
	}
}

// indexedRow renders one row. idx is the row's index as text: "0", "1", ...
// for real rows, or the level's token for the template row.
func indexedRow(ctx context.Context, e *FormEngine, b *mi.Builder, f Field, row map[string]interface{}, idx string, rc RenderContext) mi.Node {
	prefix := f.Name + "[" + idx + "]"
	var cells []interface{}
	if reorderOn(f) {
		cells = append(cells, repDragHandle(b, e, rc, f), repPosInput(b, prefix+"[_pos]", idx))
	}
	for _, item := range f.Items {
		widgetName := inferWidget(item)
		render, ok := widgetRegistry[widgetName]
		if !ok {
			cells = append(cells, unsupportedWidgetNode(b, item, widgetName))
			continue
		}
		itemField := item
		if widgetName == "checkbox" || widgetName == "toggle" {
			itemField.Title = resolveRowItemTitle(e, rc, parentName(f), item)
		}
		itemField.I18nName = item.Name
		itemField.Name = prefix + "[" + item.Name + "]"
		itemField.rowLabel = resolveRowItemLabel(e, rc, parentName(f), item)
		if itemField.XSeamPlaceholder != "" {
			// the written placeholder, in the viewer's language (the same lookup)
			itemField.XSeamPlaceholder = itemField.rowLabel
		}
		if widgetName == "repeatable" {
			itemField.indexedNaming = true
			itemField.indexDepth = f.indexDepth + 1
		}
		var v interface{}
		if row != nil {
			v = row[item.Name]
		}
		cells = append(cells, repCell(b, e, rc, f, item, widgetName, render(ctx, e, itemField, v, false, rc)(b)))
	}
	cells = append(cells, repRemoveButton(b, e, rc, f, "SeamRepeatable.remove(this)"))
	return b.Div(append([]interface{}{mi.Class("flex gap-2 items-start repeatable-row")}, cells...)...)
}

// ---------------------------------------------------------------------------
// Parsing posted values
// ---------------------------------------------------------------------------

// indexNode is one node of the tree built from bracketed field names.
type indexNode struct {
	kids map[string]*indexNode
	leaf bool
	val  string
}

// insert adds val at the path segs. A path that would make one node both a
// value and a branch is ignored (malformed input, not worth guessing at).
func (n *indexNode) insert(segs []string, val string) {
	if len(segs) == 0 {
		if n.kids == nil {
			n.leaf, n.val = true, val
		}
		return
	}
	if n.leaf {
		return
	}
	if n.kids == nil {
		n.kids = map[string]*indexNode{}
	}
	child := n.kids[segs[0]]
	if child == nil {
		child = &indexNode{}
		n.kids[segs[0]] = child
	}
	child.insert(segs[1:], val)
}

func isIndexKey(s string) bool {
	if s == "" || len(s) > 9 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// toValue turns a node into a string (leaf), a []interface{} ordered by
// numeric index (all keys numeric) or a map[string]interface{}.
func (n *indexNode) toValue() interface{} {
	if n.leaf {
		return n.val
	}
	keys := make([]string, 0, len(n.kids))
	numeric := len(n.kids) > 0
	for k := range n.kids {
		keys = append(keys, k)
		if !isIndexKey(k) {
			numeric = false
		}
	}
	if numeric {
		// Rows that all carry a posted position (x-seam-reorder) go by it, then by
		// index; otherwise by index alone.
		byPos := true
		for _, k := range keys {
			if _, ok := n.kids[k].position(); !ok {
				byPos = false
				break
			}
		}
		sort.Slice(keys, func(i, j int) bool {
			a, _ := strconv.Atoi(keys[i])
			c, _ := strconv.Atoi(keys[j])
			if byPos {
				pa, _ := n.kids[keys[i]].position()
				pc, _ := n.kids[keys[j]].position()
				if pa != pc {
					return pa < pc
				}
			}
			return a < c
		})
		out := make([]interface{}, 0, len(keys))
		for _, k := range keys {
			out = append(out, n.kids[k].toValue())
		}
		return out
	}
	out := make(map[string]interface{}, len(keys))
	for _, k := range keys {
		if k == "_pos" { // the reorder bookkeeping, not a field
			continue
		}
		out[k] = n.kids[k].toValue()
	}
	return out
}

// bracketSegments splits "[2][lines][5][name]" into its segments. ok is
// false for anything that is not a run of non-empty [segment] groups.
func bracketSegments(s string) (segs []string, ok bool) {
	for len(s) > 0 {
		if s[0] != '[' {
			return nil, false
		}
		end := strings.IndexByte(s, ']')
		if end < 2 {
			return nil, false
		}
		seg := s[1:end]
		if strings.ContainsRune(seg, '[') {
			return nil, false
		}
		segs = append(segs, seg)
		s = s[end+1:]
	}
	return segs, len(segs) > 0
}

// ParseIndexedRepeatable rebuilds the rows of the indexed repeatable f from
// posted form values. Rows come back ordered by numeric index (so 2 sorts
// before 10) as a []interface{} of map[string]interface{}, nested repeatables
// as the same shape under their field name, which is what the renderer takes
// back as a value. Values are strings as posted. Following the established
// rule for repeatables, a row whose FIRST field is empty is dropped, at every
// level. Keys that do not parse as f.Name[idx][field]... are ignored. The
// result is never nil.
func ParseIndexedRepeatable(form url.Values, f Field) []interface{} {
	root := &indexNode{}
	for key, vals := range form {
		if !strings.HasPrefix(key, f.Name+"[") {
			continue
		}
		segs, ok := bracketSegments(key[len(f.Name):])
		if !ok {
			continue
		}
		val := ""
		if len(vals) > 0 {
			val = vals[0]
		}
		root.insert(segs, val)
	}
	rows, _ := root.toValue().([]interface{})
	return pruneRows(rows, f)
}

func pruneRows(rows []interface{}, f Field) []interface{} {
	out := []interface{}{}
	for _, r := range rows {
		m, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		for _, item := range f.Items {
			if inferWidget(item) != "repeatable" {
				continue
			}
			nested, _ := m[item.Name].([]interface{})
			m[item.Name] = pruneRows(nested, item)
		}
		if len(f.Items) > 0 && inferWidget(f.Items[0]) != "repeatable" {
			if first, _ := m[f.Items[0].Name].(string); strings.TrimSpace(first) == "" {
				continue
			}
		}
		out = append(out, m)
	}
	return out
}

// ParseIndexedForm rebuilds every indexed repeatable among fields that has
// at least one posted value, keyed by field name. Flat repeatables are not
// included (their handlers read the {field}_{item}[] arrays as before).
func ParseIndexedForm(form url.Values, fields []Field) map[string]interface{} {
	out := map[string]interface{}{}
	for _, f := range fields {
		if inferWidget(f) != "repeatable" || !needsIndexedNaming(f) {
			continue
		}
		present := false
		for k := range form {
			if strings.HasPrefix(k, f.Name+"[") {
				present = true
				break
			}
		}
		if present {
			out[f.Name] = ParseIndexedRepeatable(form, f)
		}
	}
	return out
}
