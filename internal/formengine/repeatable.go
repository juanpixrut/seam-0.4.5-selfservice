package formengine

import (
	"context"
	"fmt"

	mi "github.com/ha1tch/minty"
)

// renderRepeatable implements the "repeatable" widget (§4.2's
// generalisation of document-links): a dynamic add/remove-row sub-form
// where each row can hold any combination of the standard widgets, not
// just the three fixed text inputs document-links renders. Implemented
// over the existing serviceConfigsField pattern.
//
// Field naming for each row follows {field.Name}_{item.Name}[] — the
// same convention document-links uses, and the only one derivable
// purely from the schema. The current hand-coded serviceConfigsField
// instead uses an arbitrary "svc_" abbreviation with no relationship to
// its real field name ("service_configs") — checked directly against
// its source before writing this, not assumed. That means FE-23's
// handler conversion needs a small, expected update to parse the new
// field names; that's in scope for stage 7 (which explicitly includes
// converting the handler), not a silent regression introduced here.
//
// Known limitation, not fixed here: reusing the standard widgetRegistry
// per item field means every row's copy of a given item carries the
// same id="field-..." (every widgetRenderer derives both name and id
// from the same Field.Name, and only name needs the array-bracket
// rewrite — splitting the two would mean touching every widget's
// signature). Duplicate ids across rows are invalid HTML, though
// functionally harmless here: nothing in this package's JS does a
// global getElementById lookup on a repeatable row's fields, and the
// original hand-coded serviceConfigsField sidesteps the question
// entirely by never setting id on its repeated elements in the first
// place. Filed as a register follow-up rather than solved speculatively
// with no second real caller yet to validate a fix against.
// Row visuals live in web/static/css/app.css (.repeatable-row, .fe-rep-nested,
// .fe-btn, .fe-remove, ...) and take every colour from theme variables, so a
// row is legible in light and dark mode whatever its ancestors set.

// repLayout is the repeatable's layout: "form" or the default "row".
func repLayout(f Field) string {
	if f.XSeamLayout == "form" {
		return "form"
	}
	return "row"
}

// repCell wraps one rendered cell for the "form" layout: a small label above
// the field. Checkboxes and toggles already show their own title, and a nested
// repeatable has its own heading, so those are left as they are. In the
// default "row" layout the cell is returned unchanged.
func repCell(b *mi.Builder, e *FormEngine, rc RenderContext, f Field, item Field, widgetName string, node mi.Node) interface{} {
	if repLayout(f) != "form" {
		return node
	}
	switch widgetName {
	case "repeatable", "checkbox", "toggle":
		return node
	}
	title := resolveRowItemTitle(e, rc, parentName(f), item)
	if item.Required {
		title += " *"
	}
	return b.Div(mi.Class("fe-cell"),
		b.Span(mi.Class("fe-cell-label"), title),
		node,
	)
}

func renderRepeatable(ctx context.Context, e *FormEngine, f Field, value interface{}, hasError bool, rc RenderContext) mi.H {
	if needsIndexedNaming(f) {
		return renderRepeatableIndexed(ctx, e, f, value, hasError, rc)
	}
	return func(b *mi.Builder) mi.Node {
		rows := padRows(f, arrayOfMaps(value)) // a blank row when empty; at least minItems rows
		rf := withBoundLocks(f, len(rows))
		containerID := "field-" + f.Name + "-container"
		var rowNodes []interface{}
		for _, row := range rows {
			rowNodes = append(rowNodes, repeatableRow(ctx, e, b, rf, row, rc))
		}
		// Clone-based add-row JS resets both inputs and selects
		// (selectedIndex=0) — serviceConfigsField's own add-row script
		// already does both, matched here; document-links only needed
		// the input reset since it has no select sub-fields.
		// The rows container is the button's previous sibling, found
		// relative to the button rather than by id: nested repeatables
		// repeat the same container id in every outer row, so a
		// getElementById lookup always hit the FIRST one (the second
		// block's "+ Add" grew the first block's lines). A cloned outer row
		// keeps only the first row of each nested container, so a new
		// block starts with one blank nested row.
		addOnclick := `(function(btn){
  var c=btn.previousElementSibling;
  if(!c){return;}
  var src=c._rowTemplate||c.querySelector('.repeatable-row');
  if(!src){return;}
  var t=src.cloneNode(true);
  t.querySelectorAll('.repeatable-row').forEach(function(n){if(n.parentNode.firstElementChild!==n){n.remove();}});
  t.querySelectorAll('select').forEach(function(s){s.selectedIndex=0;});
  t.querySelectorAll('input').forEach(function(i){i.value='';});
  c.appendChild(t);
})(this)`
		if f.bounded() {
			addOnclick = boundedAddOnclick(addOnclick)
		}
		return b.Div(append(repAccentAttrs(f),
			b.Div(append(rowsAttrs(f, containerID), rowNodes...)...),
			repAddButton(b, e, rc, rf, addOnclick),
		)...)
	}
}

// rowsAttrs are the attributes of a flat repeatable's rows container; the
// layout is only written when it is not the default.
func rowsAttrs(f Field, containerID string) []interface{} {
	attrs := []interface{}{mi.ID(containerID), mi.Class("space-y-2")}
	attrs = append(attrs, repReorderAttrs(f)...)
	attrs = append(attrs, repBoundsAttrs(f)...)
	if repLayout(f) == "form" {
		attrs = append(attrs, mi.Attr("data-layout", "form"))
	}
	return attrs
}

// rowRemoveOnclick returns the onclick handler for the remove (x) button of
// a clone-based add/remove row widget (repeatable rows, document-link rows).
// The add button clones an existing row, so before the clicked row is
// removed it stashes a clone on the container (c._rowTemplate); the add
// handlers fall back to that stash when no row is left. Without it, removing
// the last row left the add button with nothing to clone and it threw on
// null until the page was reloaded.
func rowRemoveOnclick(rowClass string) string {
	return fmt.Sprintf(`(function(b){
  var r=b.closest('.%s'),c=r.parentNode;
  if(!c._rowTemplate){c._rowTemplate=r.cloneNode(true);}
  r.remove();
})(this)`, rowClass)
}

// repeatableRow renders one row: each item field through the normal
// widget dispatch (widgetRegistry + inferWidget), so a repeatable row
// can genuinely contain any widget a schema author puts in items — not
// a hardcoded subset — with its Name rewritten to the array-bracket
// submission convention first.
func repeatableRow(ctx context.Context, e *FormEngine, b *mi.Builder, f Field, row map[string]interface{}, rc RenderContext) mi.Node {
	var cells []interface{}
	if reorderOn(f) {
		cells = append(cells, repDragHandle(b, e, rc, f))
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
		// I18nName preserves the original field name (e.g. "measure") for
		// enum-label resolution -- set before Name is rewritten to the
		// array-bracket submission form below, which resolveEnumOptionLabel
		// must never see (T-18 Stage 9, 2026-08-01).
		itemField.I18nName = item.Name
		itemField.Name = f.Name + "_" + item.Name + "[]"
		itemField.rowLabel = resolveRowItemLabel(e, rc, parentName(f), item)
		if itemField.XSeamPlaceholder != "" {
			// the written placeholder, in the viewer's language (the same lookup)
			itemField.XSeamPlaceholder = itemField.rowLabel
		}
		cells = append(cells, repCell(b, e, rc, f, item, widgetName, render(ctx, e, itemField, row[item.Name], false, rc)(b)))
	}
	removeOnclick := rowRemoveOnclick("repeatable-row")
	if f.bounded() {
		removeOnclick = rowRemoveOnclickBounded("repeatable-row")
	}
	cells = append(cells, repRemoveButton(b, e, rc, f, removeOnclick))
	return b.Div(append([]interface{}{mi.Class("flex gap-2 items-start repeatable-row")}, cells...)...)
}
