package formengine

import (
	"strconv"

	mi "github.com/ha1tch/minty"
)

// Reordering the rows of a repeatable. x-seam-reorder (a boolean on the
// repeatable, off by default) gives every row a drag handle; SeamReorder in
// web/static/js/app.js does the moving (pointer drag, and ArrowUp/ArrowDown on
// the focused handle).
//
//	flat naming     the browser posts the parallel {field}_{item}[] arrays in DOM
//	                order, so moving the rows is all it takes;
//	indexed naming  rows are rebuilt by index, so each row also carries a hidden
//	                name[i][_pos] input. SeamReorder writes the row's position in
//	                the DOM into it after every move and again when the form is
//	                submitted. ParseIndexedRepeatable orders the rows by it when
//	                every row has one, and drops it from the result.
//
// With x-seam-reorder off nothing here changes the output.

// repDragHandle is the drag handle at the start of a row: the usual six-dot
// grip in the row layout, a thin grip bar across the top of the card in the
// form layout (CSS places it).
func repDragHandle(b *mi.Builder, e *FormEngine, rc RenderContext, f Field) mi.Node {
	label := resolveWidgetChrome(e, rc, "action.reorder", "Reorder")
	icon := "drag_indicator"
	if repLayout(f) == "form" {
		icon = "drag_handle"
	}
	return b.Button(mi.Type("button"), mi.Class("fe-drag"),
		mi.Attr("aria-label", label), mi.Attr("title", label),
		b.Span(mi.Class("material-icons"), icon))
}

// repPosInput is the hidden position input of an indexed row.
func repPosInput(b *mi.Builder, name, value string) mi.Node {
	return b.Input(mi.Type("hidden"), mi.Name(name), mi.Value(value))
}

// repReorderAttrs marks the rows container so SeamReorder knows it may move
// its rows; nil when the repeatable is not reorderable.
func repReorderAttrs(f Field) []interface{} {
	if !reorderOn(f) {
		return nil
	}
	return []interface{}{mi.Attr("data-reorder", "1")}
}

// position is the posted _pos of a row node (name[i][_pos]); false when the
// row has none or it is not a plain number.
func (n *indexNode) position() (int, bool) {
	k := n.kids["_pos"]
	if k == nil || !k.leaf || !isIndexKey(k.val) {
		return 0, false
	}
	p, err := strconv.Atoi(k.val)
	return p, err == nil
}
