package formengine

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	mi "github.com/ha1tch/minty"
)

// Bounds on the number of rows of a repeatable.
//
// They are configured with the standard JSON Schema keywords on the repeatable's
// property, so one declaration serves the form and the validator (queryfy
// already enforces minItems and maxItems):
//
//	"lines": {"type": "array", "x-seam-input": "repeatable", "minItems": 1, "maxItems": 5, ...}
//
// minItems defaults to 0 and maxItems to unlimited. They must satisfy
// 0 <= minItems <= maxItems <= MaxRepeatableBound and maxItems >= 1, or
// ParseSchema refuses the schema: a repeatable that can never hold a row is a
// mistake to find when the schema is written, not in production.
//
// In the form:
//   - at least max(1, minItems) rows are shown, blank ones padding the list;
//   - the remove button of every row is disabled while the list is at its minimum
//     and the add button while it is at its maximum (SeamRepeatable in app.js
//     enforces the same limits when rows are added and removed);
//   - rows that already exceed the maximum (existing data) are all shown, never
//     hidden, with the add button disabled.
//
// The form is a convenience; the check that counts is the server's: a handler
// calls ValidatePostedBounds with the posted values (it counts the rows of both
// namings, dropping blank ones as everywhere else) or RepeatableBoundsMessage
// when it counts rows itself. With no bound set none of this changes the output.

// MaxRepeatableBound is the largest minItems or maxItems accepted.
const MaxRepeatableBound = 1000

// parseRepeatableBounds checks the schema's minItems and maxItems.
func parseRepeatableBounds(min, max *int) (lo, hi int, err error) {
	if min != nil {
		if *min < 0 || *min > MaxRepeatableBound {
			return 0, 0, fmt.Errorf("minItems must be between 0 and %d, got %d", MaxRepeatableBound, *min)
		}
		lo = *min
	}
	if max != nil {
		if *max < 1 || *max > MaxRepeatableBound {
			return 0, 0, fmt.Errorf("maxItems must be between 1 and %d, got %d", MaxRepeatableBound, *max)
		}
		hi = *max
	}
	if hi > 0 && lo > hi {
		return 0, 0, fmt.Errorf("minItems (%d) is greater than maxItems (%d)", lo, hi)
	}
	return lo, hi, nil
}

// bounded reports whether the field has a minimum or a maximum.
func (f Field) bounded() bool { return f.MinItems > 0 || f.MaxItems > 0 }

// repBoundsAttrs are the rows container's data-min and data-max, which the
// script reads; nil when the repeatable has no bound.
func repBoundsAttrs(f Field) []interface{} {
	var out []interface{}
	if f.MinItems > 0 {
		out = append(out, mi.Attr("data-min", strconv.Itoa(f.MinItems)))
	}
	if f.MaxItems > 0 {
		out = append(out, mi.Attr("data-max", strconv.Itoa(f.MaxItems)))
	}
	return out
}

// padRows returns rows padded with blank rows to max(1, minItems): the first
// render of an empty list has always shown one blank row.
func padRows(f Field, rows []map[string]interface{}) []map[string]interface{} {
	want := f.MinItems
	if want < 1 {
		want = 1
	}
	for len(rows) < want {
		rows = append(rows, map[string]interface{}{})
	}
	return rows
}

// withBoundLocks returns f with the render-time locks set for a list of n rows:
// the remove buttons at the minimum, the add button at the maximum.
func withBoundLocks(f Field, n int) Field {
	f.removeLocked = f.MinItems > 0 && n <= f.MinItems
	f.addLocked = f.MaxItems > 0 && n >= f.MaxItems
	return f
}

// boundedAddOnclick turns a flat repeatable's add handler into one that
// refuses to go past the maximum and updates the buttons afterwards.
func boundedAddOnclick(onclick string) string {
	onclick = strings.Replace(onclick, "if(!c){return;}",
		"if(!c){return;}\n  if(window.SeamRepeatable&&!SeamRepeatable.canAdd(c)){return;}", 1)
	return strings.Replace(onclick, "c.appendChild(t);",
		"c.appendChild(t);\n  if(window.SeamRepeatable){SeamRepeatable.sync(c);}", 1)
}

// rowRemoveOnclickBounded is rowRemoveOnclick for a repeatable with bounds: it
// refuses to go below the minimum and updates the buttons afterwards.
func rowRemoveOnclickBounded(rowClass string) string {
	return fmt.Sprintf(`(function(b){
  var r=b.closest('.%s'),c=r.parentNode;
  if(window.SeamRepeatable&&!SeamRepeatable.canRemove(c)){return;}
  if(!c._rowTemplate){c._rowTemplate=r.cloneNode(true);}
  r.remove();
  if(window.SeamRepeatable){SeamRepeatable.sync(c);}
})(this)`, rowClass)
}

// RepeatableBoundsMessage is the message for a list of n rows that breaks the
// field's bounds, or "" when it does not. The texts are field.repeatable.min_items
// and field.repeatable.max_items.
func RepeatableBoundsMessage(e *FormEngine, rc RenderContext, f Field, n int) string {
	switch {
	case f.MinItems > 0 && n < f.MinItems:
		return fmt.Sprintf(resolveWidgetChrome(e, rc, "field.repeatable.min_items", "At least %d items are required"), f.MinItems)
	case f.MaxItems > 0 && n > f.MaxItems:
		return fmt.Sprintf(resolveWidgetChrome(e, rc, "field.repeatable.max_items", "At most %d items are allowed"), f.MaxItems)
	}
	return ""
}

// flatRowCount counts the rows a flat repeatable was posted with: the entries
// of its first item's array that are not blank (a row whose first field is
// empty is dropped, as everywhere else).
func flatRowCount(post url.Values, f Field) int {
	if len(f.Items) == 0 {
		return 0
	}
	n := 0
	for _, v := range post[f.Name+"_"+f.Items[0].Name+"[]"] {
		if strings.TrimSpace(v) != "" {
			n++
		}
	}
	return n
}

// ValidatePostedBounds checks every bounded top-level repeatable of form against
// the posted values, whichever naming it uses, and returns a message per
// offending field (nil when everything is within bounds). A field that was not
// posted counts as no rows. This is the check a handler calls; the form itself
// only keeps the buttons honest.
func ValidatePostedBounds(e *FormEngine, rc RenderContext, form *Form, post url.Values) map[string]string {
	if form == nil {
		return nil
	}
	var errs map[string]string
	for _, f := range form.Fields {
		if inferWidget(f) != "repeatable" || !f.bounded() {
			continue
		}
		var n int
		if needsIndexedNaming(f) {
			n = len(ParseIndexedRepeatable(post, f))
		} else {
			n = flatRowCount(post, f)
		}
		if msg := RepeatableBoundsMessage(e, rc, f, n); msg != "" {
			if errs == nil {
				errs = map[string]string{}
			}
			errs[f.Name] = msg
		}
	}
	return errs
}
