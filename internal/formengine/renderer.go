package formengine

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	mi "github.com/ha1tch/minty"
)

// renderForm renders the whole form. When form.Tabs is nil, this is the
// §3.10 flat path (fields in schema.properties order). When form.Tabs is
// populated, it walks tabs → rows → sections → fields per §3.1–§3.4 and
// §3.8. Details, embeds, visible_when, and the tray are later stages —
// see docs/FORMENGINE_DEVELOPMENT_PLAN.md.
func renderForm(ctx context.Context, e *FormEngine, form *Form, values map[string]interface{}, errs map[string]string, rc RenderContext) mi.H {
	return func(b *mi.Builder) mi.Node {
		var children []interface{}

		// CSRF token travels with the schema-driven content, not the
		// caller's page/modal envelope — RenderContext already carries
		// it (§3c.2), and a form that can't produce a valid POST isn't
		// actually wired, just rendered. Buttons and the full-page vs.
		// modal wrapper stay the caller's job (existing masterFormButtons
		// / masterPartialFormWrapper helpers), matching how the static
		// forms already separate "form content" from "form chrome" —
		// neither is schema-driven, so neither belongs in this package.
		if rc.CSRFToken != "" {
			children = append(children, b.Input(mi.Type("hidden"), mi.Name("_csrf"), mi.Value(rc.CSRFToken)))
		}

		if len(form.Tabs) > 0 {
			children = append(children, renderTabbedBody(b, ctx, e, form.Tabs, values, errs, rc))
		} else {
			children = append(children, renderFlatFields(b, ctx, e, form.Fields, values, errs, rc))
		}

		// Emitted unconditionally (not just for tabbed forms — a flat,
		// no-tabs form can still have field-level x-seam-visible-when).
		// A harmless no-op when the form has no visible_when conditions
		// anywhere: querySelectorAll('[data-visible-when]') just finds
		// nothing.
		children = append(children, visibilityScript(b))

		return b.Div(append([]interface{}{mi.Class("space-y-2")}, children...)...)
	}
}

// renderFlatFields is the §3.10 no-tabs path, factored out of renderForm
// so renderForm's tabs/flat branch stays a single readable dispatch.
func renderFlatFields(b *mi.Builder, ctx context.Context, e *FormEngine, fields []Field, values map[string]interface{}, errs map[string]string, rc RenderContext) mi.Node {
	hidden := toSet(rc.HiddenFields)
	readOnly := toSet(rc.ReadOnlyFields)
	var kids []interface{}
	for _, f := range fields {
		if hidden[f.Name] {
			continue
		}
		fieldReadOnly := rc.ReadOnly || readOnly[f.Name]
		kids = append(kids, renderFieldRow(b, ctx, e, f, values, resolveFieldError(f, errs, rc), fieldReadOnly, rc))
	}
	return b.Div(append([]interface{}{mi.Class("space-y-2")}, kids...)...)
}

// renderTabbedBody renders the tab bar and every tab panel (all panels
// render; CSS + the switching script control which is visible — same
// pattern as the existing static Asset form's asset-tab-btn/
// asset-tab-panel convention in internal/ui/asset_detail.go, namespaced
// here as fe-tab-* so the two never collide if both render on one page).
func renderTabbedBody(b *mi.Builder, ctx context.Context, e *FormEngine, tabs []Tab, values map[string]interface{}, errs map[string]string, rc RenderContext) mi.Node {
	hiddenTabs := toSet(rc.HiddenTabs)
	readOnlyTabs := toSet(rc.ReadOnlyTabs)
	hiddenFields := toSet(rc.HiddenFields)

	type visibleTab struct {
		tab      Tab
		readOnly bool
	}
	var visible []visibleTab
	for _, t := range tabs {
		if hiddenTabs[t.ID] {
			continue
		}
		if allTabFieldsHidden(t, hiddenFields) {
			continue
		}
		// Tab-level visible_when (§3b.1 lists tabs among the things that
		// can be conditionally visible) is evaluated once, at render
		// time, and omits the tab like HiddenTabs does — not kept in the
		// DOM for live client-side reactivity the way field/section/row
		// visibility is (§3b.6). Deliberate stage-5 scope decision: no
		// example anywhere in FORM_ENGINE_SPEC.md exercises a
		// conditionally-visible tab, and reconciling live tab
		// visibility with the active/inactive toggling tabSwitchScript
		// already does is real additional complexity with nothing
		// concrete driving it yet. Revisit if a schema actually needs it.
		if t.VisibleWhen != nil && !EvaluateVisibleWhen(t.VisibleWhen, values) {
			continue
		}
		visible = append(visible, visibleTab{tab: t, readOnly: rc.ReadOnly || readOnlyTabs[t.ID]})
	}

	if len(visible) == 0 {
		return b.P(mi.Class("text-sm text-amber-600"), "formengine: no visible tabs (core.errors.no_visible_tabs)")
	}

	var btns []interface{}
	var panels []interface{}
	for i, vt := range visible {
		active := i == 0
		btns = append(btns, tabButton(b, vt.tab.ID, resolveLabel(e, rc, vt.tab.Label), active))
		panels = append(panels, tabPanel(b, ctx, e, vt.tab, values, errs, rc, vt.readOnly))
	}

	// rc.ActiveTab is only honoured when it names one of this form's own tabs, so a value that
	// arrives from a URL can never put anything but a known id into the script below.
	openTab := ""
	for _, tab := range tabs {
		if rc.ActiveTab != "" && tab.ID == rc.ActiveTab {
			openTab = tab.ID
		}
	}
	return b.Div(
		b.Div(append([]interface{}{mi.Class("flex border-b border-gray-200 dark:border-gray-700 overflow-x-auto")}, btns...)...),
		b.Div(append([]interface{}{mi.Class("pt-4")}, panels...)...),
		tabSwitchScript(b, openTab),
	)
}

func allTabFieldsHidden(t Tab, hiddenFields map[string]bool) bool {
	for _, row := range t.Rows {
		for _, sec := range row.Sections {
			for _, f := range sec.Fields {
				if !hiddenFields[f.Name] {
					return false
				}
			}
		}
	}
	return true
}

func tabButton(b *mi.Builder, id, label string, active bool) mi.Node {
	cls := "fe-tab-btn px-4 py-3 text-sm font-medium border-b-2 whitespace-nowrap cursor-pointer transition-colors "
	if active {
		cls += "border-indigo-600 text-indigo-600 dark:text-indigo-400 dark:border-indigo-400"
	} else {
		cls += "border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300"
	}
	return b.Button(mi.Type("button"), mi.Class(cls), mi.Attr("data-tab", id), label)
}

func tabPanel(b *mi.Builder, ctx context.Context, e *FormEngine, t Tab, values map[string]interface{}, errs map[string]string, rc RenderContext, tabReadOnly bool) mi.Node {
	var kids []interface{}
	for i, row := range t.Rows {
		if rowHiddenIndex(rc.HiddenRows[t.ID], i) {
			continue
		}
		node := renderRow(b, ctx, e, row, values, errs, rc, tabReadOnly)
		if node != nil {
			kids = append(kids, node)
		}
	}

	// Details and embeds only render in edit mode (§3.7) — both require
	// an existing entity (EntityID) to build their htmx URLs against.
	// Embeds additionally never render for a read-only tab: they have
	// their own submit cycle (§3.7), and a tab marked read-only shouldn't
	// offer an action that bypasses that.
	if rc.IsEdit {
		rightDetails, bottomDetails := splitDetailsByPosition(t.Details)

		var bottom []interface{}
		for _, d := range bottomDetails {
			bottom = append(bottom, renderDetailPanel(b, e, d, rc))
		}
		if !tabReadOnly {
			for _, em := range t.Embeds {
				bottom = append(bottom, renderEmbedTrigger(b, e, em, rc))
			}
		}

		if len(rightDetails) > 0 {
			var right []interface{}
			for _, d := range rightDetails {
				right = append(right, renderDetailPanel(b, e, d, rc))
			}
			kids = append(kids, b.Div(mi.Class("flex flex-col lg:flex-row gap-4"),
				b.Div(append([]interface{}{mi.Class("flex-1 space-y-4")}, bottom...)...),
				b.Div(append([]interface{}{mi.Class("lg:w-80 space-y-4")}, right...)...),
			))
		} else {
			kids = append(kids, bottom...)
		}
	}

	attrs := []interface{}{mi.ID(t.ID), mi.Class("fe-tab-panel space-y-4")}
	return b.Div(append(attrs, kids...)...)
}

// splitDetailsByPosition separates "right"-positioned panels from
// everything else. "inline" degrades to "bottom" for now — none of the
// schemas this stage targets (sensores_vinculados, custodio_history)
// use it, and a defensible anchor point for true inline placement
// (interspersed between specific rows) needs more of the schema
// vocabulary than a tab-level array currently carries.
func splitDetailsByPosition(details []DetailPanel) (right, bottom []DetailPanel) {
	for _, d := range details {
		if d.Position == "right" {
			right = append(right, d)
		} else {
			bottom = append(bottom, d)
		}
	}
	return right, bottom
}

// substituteID replaces the "{id}" placeholder FORM_ENGINE_SPEC.md §3.5
// and §3.6 use in detail sources and embed endpoints.
func substituteID(urlTemplate string, entityID int64) string {
	return strings.ReplaceAll(urlTemplate, "{id}", strconv.FormatInt(entityID, 10))
}

// renderDetailPanel renders a §3.5 read-only detail panel as an
// htmx-loaded placeholder — the panel's actual content (a table built
// from Columns) is a separate request/response the panel's own handler
// serves; formengine renders one schema at a time and doesn't reach
// across into rendering that response itself.
func renderDetailPanel(b *mi.Builder, e *FormEngine, d DetailPanel, rc RenderContext) mi.Node {
	url := substituteID(d.Source, rc.EntityID)
	// Both "eager" and "lazy" currently use htmx's "revealed" trigger —
	// the spec's own §3.5 example shows exactly this for the eager case.
	// The distinction it draws (loads on tab activation vs. loads on
	// scroll-into-view) isn't yet differentiated at the trigger-attribute
	// level; both behave as scroll-into-view today. Noted here rather
	// than silently treated as equivalent.
	bodyAttrs := []interface{}{
		mi.Class("text-xs text-gray-400 dark:text-gray-500"),
		mi.HxGet(url),
		mi.Attr("hx-trigger", "revealed"),
		mi.Attr("hx-target", "this"),
		mi.Attr("hx-swap", "innerHTML"),
	}
	body := b.Div(append(bodyAttrs, "…")...)

	header := []interface{}{
		mi.Class("text-xs font-semibold text-gray-400 uppercase tracking-wider mb-2"),
		resolveLabel(e, rc, d.Label),
	}

	return b.Div(mi.ID("detail-"+d.ID), mi.Class("border border-gray-200 dark:border-gray-700 rounded-lg p-4"),
		b.H4(header...),
		body,
	)
}

// renderEmbedTrigger renders a §3.6 embed's activation control.
// trigger == "auto" needs the §3b condition evaluator (stage 5) to know
// when to fire — not available yet, so it degrades to a button rather
// than silently never appearing.
// renderEmbedTrigger renders a §3.6 embed's activation control.
// trigger == "auto" needs the §3b condition evaluator (stage 5) to know
// when to fire — not available yet, so it degrades to a button rather
// than silently never appearing.
//
// §3.7 / FE-14: RefreshDetail names a DetailPanel in the same tab that
// should reload after this embed's submit succeeds. formengine renders
// only the trigger here — the embed's actual sub-form content is a
// separate request/render this package doesn't reach into (same
// reasoning as detail panels) — so the reload itself can't be wired
// from here. What formengine can and does do is emit a discoverable
// data-refresh-detail="detail-<id>" attribute naming the target; the
// JS that listens for embed-submit-success and triggers that panel's
// htmx reload is a small addition outside this package (app.js or
// similar), needed when a real embed gets wired end to end — not
// invented speculatively here.
func renderEmbedTrigger(b *mi.Builder, e *FormEngine, em EmbedForm, rc RenderContext) mi.Node {
	url := substituteID(em.Endpoint, rc.EntityID)
	var refreshAttrs []mi.Attribute
	if em.RefreshDetail != "" {
		refreshAttrs = append(refreshAttrs, mi.Attr("data-refresh-detail", "detail-"+em.RefreshDetail))
	}

	if em.Trigger == "inline" {
		attrs := append([]interface{}{
			mi.ID("embed-" + em.ID), mi.Class("border border-gray-200 dark:border-gray-700 rounded-lg p-4"),
			mi.HxGet(url), mi.Attr("hx-trigger", "revealed"), mi.Attr("hx-swap", "innerHTML"),
		}, attrArgs(refreshAttrs)...)
		return b.Div(append(attrs, b.P(mi.Class("text-xs text-gray-400"), "…"))...)
	}

	// "button" (and "auto", degraded) — mode determines how the click
	// opens the sub-form. mode:"drawer" is unresolved, not guessed at:
	// SEAM_MEDIA_SPEC.md §2 documents fijuggler's real integration
	// pattern as SeamModal.open({width:"fullscreen", url:"/files"}) —
	// explicitly NOT embedded inline as a drawer/side-panel — but
	// whether FORM_ENGINE_SPEC.md's embed mode:"drawer" actually means
	// "open fijuggler" or is a general side-panel pattern unrelated to
	// file browsing was left an open question 2026-07-28 (see T-01
	// tracker). Neither of the two concrete embed examples this stage
	// targets (sensor_binding, qr_assign) use "drawer", so this falls
	// back to the modal2 behaviour without that question needing to
	// block here — revisit when a real "drawer" schema shows up.
	label := em.TriggerLabel
	if label == "" {
		label = em.Label
	}
	label = resolveLabel(e, rc, label)
	switch em.Mode {
	case "inline":
		// mode:"inline" with trigger:"button" — click reveals an inline
		// container rather than opening a modal.
		attrs := append([]interface{}{
			mi.Type("button"), mi.Class("text-sm text-indigo-600 dark:text-indigo-400 hover:underline"),
			mi.Attr("onclick", "this.nextElementSibling.style.display='block';this.style.display='none'"),
		}, attrArgs(refreshAttrs)...)
		return b.Button(append(attrs, label)...)
	default: // "modal2", "drawer" (unresolved, falls back — see above)
		onclick := fmt.Sprintf("SeamModal2.open(%s,%s)", jsStringLiteral(label), jsStringLiteral(url))
		attrs := append([]interface{}{
			mi.Type("button"),
			mi.Class("px-3 py-1.5 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700"),
			mi.Attr("onclick", onclick),
		}, attrArgs(refreshAttrs)...)
		return b.Button(append(attrs, label)...)
	}
}

// jsStringLiteral renders a Go string as a single-quoted JS string
// literal for inline onclick="..." attributes, escaping the characters
// that would otherwise break out of the quotes.
func jsStringLiteral(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return "'" + s + "'"
}

func rowHiddenIndex(hiddenRows []int, i int) bool {
	for _, h := range hiddenRows {
		if h == i {
			return true
		}
	}
	return false
}

// widthSpan maps §3.8's width vocabulary to a 12-column grid span.
var widthSpan = map[string]int{
	"full": 12, "1/2": 6, "1/3": 4, "2/3": 8, "1/4": 3, "3/4": 9,
}

// renderRow lays out 1–3 sections side by side on a 12-column grid,
// per §3.8. Returns nil if every section in the row is hidden or ends
// up with no visible fields (§3.8's row auto-collapse rule).
func renderRow(b *mi.Builder, ctx context.Context, e *FormEngine, row Row, values map[string]interface{}, errs map[string]string, rc RenderContext, tabReadOnly bool) mi.Node {
	hiddenSections := toSet(rc.HiddenSections)

	spans := resolveWidths(row.Widths, len(row.Sections))

	var kids []interface{}
	anyVisible := false
	for i, sec := range row.Sections {
		if hiddenSections[sec.ID] {
			continue
		}
		secNode, hadContent := renderSection(b, ctx, e, sec, values, errs, rc, tabReadOnly)
		if !hadContent {
			continue
		}
		anyVisible = true
		span := 12
		if i < len(spans) {
			span = spans[i]
		}
		kids = append(kids, b.Div(mi.Class(fmt.Sprintf("col-span-12 md:col-span-%d", span)), secNode))
	}
	if !anyVisible {
		return nil
	}
	rowAttrs := append([]interface{}{mi.Class("grid grid-cols-12 gap-4")}, attrArgs(visibilityAttrs(row.VisibleWhen, values))...)
	return b.Div(append(rowAttrs, kids...)...)
}

// resolveWidths fills in missing widths per §3.8: explicit widths are
// honoured, and the remaining 12-column budget is split evenly across
// sections that didn't specify one.
func resolveWidths(widths []string, n int) []int {
	spans := make([]int, n)
	used := 0
	missing := 0
	for i := 0; i < n; i++ {
		if i < len(widths) && widths[i] != "" {
			if s, ok := widthSpan[widths[i]]; ok {
				spans[i] = s
				used += s
				continue
			}
		}
		spans[i] = -1 // mark unresolved
		missing++
	}
	if missing > 0 {
		remaining := 12 - used
		if remaining < missing {
			remaining = missing // degrade gracefully rather than emit 0-or-negative spans
		}
		each := remaining / missing
		for i := range spans {
			if spans[i] == -1 {
				spans[i] = each
			}
		}
	}
	return spans
}

// renderSection renders one field group with its §3.4 layout and H3
// header. Returns (node, false) when every field is hidden — callers use
// the bool to implement §3.8's section/row auto-collapse.
func renderSection(b *mi.Builder, ctx context.Context, e *FormEngine, sec Section, values map[string]interface{}, errs map[string]string, rc RenderContext, tabReadOnly bool) (mi.Node, bool) {
	hiddenFields := toSet(rc.HiddenFields)
	readOnlyFields := toSet(rc.ReadOnlyFields)
	readOnlySections := toSet(rc.ReadOnlySections)
	sectionReadOnly := tabReadOnly || readOnlySections[sec.ID]

	var fieldNodes []interface{}
	for _, f := range sec.Fields {
		if hiddenFields[f.Name] {
			continue
		}
		fr := sectionReadOnly || readOnlyFields[f.Name]
		fieldNodes = append(fieldNodes, renderFieldRow(b, ctx, e, f, values, resolveFieldError(f, errs, rc), fr, rc))
	}
	if len(fieldNodes) == 0 {
		return nil, false
	}

	gridCls := "grid gap-x-4 "
	switch sec.Layout {
	case "2-col", "2-col-left", "2-col-right":
		gridCls += "grid-cols-1 md:grid-cols-2"
	default: // "1-col" and unrecognised values fall back to single column
		gridCls += "grid-cols-1"
	}

	var kids []interface{}
	if sec.Label != "" {
		kids = append(kids, b.H3(mi.Class("text-xs font-semibold text-gray-400 uppercase tracking-wider mb-2"), resolveLabel(e, rc, sec.Label)))
	}
	kids = append(kids, b.Div(append([]interface{}{mi.Class(gridCls)}, fieldNodes...)...))

	secAttrs := attrArgs(visibilityAttrs(sec.VisibleWhen, values))
	return b.Div(append(secAttrs, kids...)...), true
}

// tabSwitchScript wires tab-button clicks to show/hide the matching
// panel. Same interaction pattern as the existing static Asset form
// (internal/ui/asset_detail.go), reimplemented here under the fe-tab-*
// namespace rather than shared, since the two don't currently share a
// JS module to import from.
// tabSwitchScript wires tab-button clicks to show/hide the matching
// panel, via the shared window.SeamTabs.wire() helper in app.js — the
// same helper internal/ui/asset_detail.go's static Asset form tabs use.
// Both used to hand-type byte-for-byte identical JS under different
// class-name prefixes (fe-tab-* here, asset-tab-* there); consolidated
// 2026-07-28 into one place so there's nothing left to drift between them.
func tabSwitchScript(b *mi.Builder, openTab string) mi.Node {
	open := ""
	if openTab != "" {
		open = `var o=document.querySelector('.fe-tab-btn[data-tab="'+` + jsStringLiteral(openTab) + `+'"]');if(o){o.click();}`
	}
	// app.js is loaded with defer, so on a full page load this inline script
	// runs before SeamTabs exists: wait for DOMContentLoaded in that case.
	return b.Script(mi.Raw(`(function(){function go(){SeamTabs.wire('.fe-tab-panel','.fe-tab-btn');` + open + `}if(window.SeamTabs){go();}else{document.addEventListener('DOMContentLoaded',go);}})();`))
}

// visibilityScript is the §3b.6 client-side reactive evaluator: finds
// every element carrying data-visible-when, wires change/input listeners
// on the fields each condition references, and re-evaluates + toggles
// display (and required, for field rows — §3b.7) whenever one of those
// fields changes. Mirrors EvaluateVisibleWhen's operator semantics
// (visibility.go) exactly — same operator set, same "unknown operator or
// missing field fails closed" rule — so client and server never disagree
// about what's visible. Emitted once per rendered form; harmless no-op
// when there are no visible_when conditions anywhere in it.
func visibilityScript(b *mi.Builder) mi.Node {
	return b.Script(mi.Raw(`
(function(){
  function readValue(name){
    var el=document.getElementById('field-'+name);
    if(!el)return null;
    if(el.type==='checkbox')return el.checked;
    if(el.type==='number'){var n=parseFloat(el.value);return isNaN(n)?null:n;}
    return el.value===''?null:el.value;
  }
  function isEmpty(v){return v===null||v===undefined||v===''||v===0;}
  function looseEq(a,b){
    if(a===null||b===null)return a===null&&b===null;
    var an=typeof a==='number'?a:parseFloat(a), bn=typeof b==='number'?b:parseFloat(b);
    if(!isNaN(an)&&!isNaN(bn)&&typeof a!=='boolean'&&typeof b!=='boolean')return an===bn;
    return a===b;
  }
  function inList(v,list){
    if(!Array.isArray(list))return false;
    for(var i=0;i<list.length;i++){if(looseEq(v,list[i]))return true;}
    return false;
  }
  function evalOp(op,actual,want){
    switch(op){
      case 'eq': return looseEq(actual,want);
      case 'neq': return !looseEq(actual,want);
      case 'in': return inList(actual,want);
      case 'not_in': return !inList(actual,want);
      case 'empty': return isEmpty(actual);
      case 'not_empty': return !isEmpty(actual);
      case 'gt': case 'gte': case 'lt': case 'lte':
        var af=typeof actual==='number'?actual:parseFloat(actual);
        var wf=typeof want==='number'?want:parseFloat(want);
        if(isNaN(af)||isNaN(wf))return false;
        if(op==='gt')return af>wf; if(op==='gte')return af>=wf;
        if(op==='lt')return af<wf; return af<=wf;
      default: return false; // unknown operator fails closed, same as the Go evaluator
    }
  }
  function evalCond(cond){
    if(!cond)return true;
    if(cond.all_of){for(var i=0;i<cond.all_of.length;i++){if(!evalCond(cond.all_of[i]))return false;}return true;}
    if(cond.any_of){for(var j=0;j<cond.any_of.length;j++){if(evalCond(cond.any_of[j]))return true;}return false;}
    return evalOp(cond.op, readValue(cond.field), cond.value);
  }
  function fieldsOf(cond, out){
    if(!cond)return;
    if(cond.all_of){cond.all_of.forEach(function(c){fieldsOf(c,out);});return;}
    if(cond.any_of){cond.any_of.forEach(function(c){fieldsOf(c,out);});return;}
    if(cond.field)out[cond.field]=true;
  }

  var watchers={}; // fieldName -> [{el, cond}]
  document.querySelectorAll('[data-visible-when]').forEach(function(el){
    var cond;
    try{cond=JSON.parse(el.getAttribute('data-visible-when'));}catch(e){return;}
    var names={};
    fieldsOf(cond, names);
    Object.keys(names).forEach(function(n){
      (watchers[n]=watchers[n]||[]).push({el:el, cond:cond});
    });
  });

  function apply(entry){
    var visible=evalCond(entry.cond);
    entry.el.style.display=visible?'':'none';
    // §3b.7: a field row's own <input>/<select>/<textarea> loses its
    // required attribute while hidden, and gets it back while visible --
    // sourced from data-schema-required (set server-side from the
    // original schema), not reconstructed from DOM transition history.
    // A field that starts hidden never had the required attribute in
    // its initial HTML to begin with, so a "was it required before this
    // transition" marker is never set for it and it would stay
    // non-required forever even after becoming visible -- caught by an
    // actual jsdom run.
    if(entry.el.hasAttribute('data-field-row')){
      var input=entry.el.querySelector('input,select,textarea');
      if(input && entry.el.getAttribute('data-schema-required')==='1'){
        if(visible){input.setAttribute('required','required');}
        else{input.removeAttribute('required');}
      }
    }
  }

  Object.keys(watchers).forEach(function(name){
    var el=document.getElementById('field-'+name);
    if(!el)return;
    el.addEventListener('input',function(){watchers[name].forEach(apply);});
    el.addEventListener('change',function(){watchers[name].forEach(apply);});
  });
})();
`))
}

// renderFieldRow wraps one field's widget with its label, help text, and
// error message. Checkbox/toggle render their own label beside the
// control (selfLabeledWidgets) and skip the top label here.
//
// values is the full current-values snapshot, needed to evaluate the
// field's own x-seam-visible-when condition (§3b.2) at render time — a
// condition-hidden field still renders (display:none, §3b.6), it isn't
// omitted like a permission-hidden one.
func renderFieldRow(b *mi.Builder, ctx context.Context, e *FormEngine, f Field, values map[string]interface{}, errMsg string, readOnly bool, rc RenderContext) mi.Node {
	value := values[f.Name]
	resolvedTitle := resolveTitle(e, rc, f.Name, f.Title)
	resolvedHelp := resolveHelp(e, rc, f.Name, f.XSeamHelp)
	if f.XSeamPlaceholder != "" {
		f.XSeamPlaceholder = resolvePlaceholder(e, rc, f.Name, f.XSeamPlaceholder)
	}
	widgetName := inferWidget(f)
	render, ok := widgetRegistry[widgetName]
	if !ok {
		return unsupportedWidgetNode(b, f, widgetName)
	}

	// §3b.7: required is only HTML-enforced while the condition holds —
	// a hidden required field must not block native form validation.
	// The client evaluator (visibilityScript) keeps this in sync as the
	// condition's truth value changes after load.
	effectiveField := f
	if f.XSeamVisibleWhen != nil && !EvaluateVisibleWhen(f.XSeamVisibleWhen, values) {
		effectiveField.Required = false
	}
	// Real bug, found 2026-08-03: resolvedTitle (above) was computed and
	// then only ever used at fkPopoverAwareLabel's call site below --
	// which selfLabeledWidgets (checkbox/toggle) skip entirely, since
	// they render their own label. Those widgets read f.Title directly
	// (renderCheckboxLike), which without this line is still the raw,
	// untranslated schema literal -- confirmed directly: no toggle/
	// checkbox field anywhere in this codebase had ever been wired to a
	// live route before Profile/Preferences/Settings (MOD-8) today, so
	// nothing had exercised this path. Every widget gets the resolved
	// title from here on, not just the ones that read it via the
	// separate label wrapper.
	effectiveField.Title = resolvedTitle

	hasError := errMsg != ""
	widgetNode := render(ctx, e, effectiveField, value, hasError, rc)(b)
	if readOnly {
		widgetNode = wrapReadOnly(b, widgetNode)
	}
	if f.XSeamUnit != "" {
		widgetNode = wrapWithUnit(b, widgetNode, f.XSeamUnit)
	}

	rowAttrs := append([]interface{}{mi.Class("mb-4"), mi.Attr("data-field-row", f.Name)},
		attrArgs(visibilityAttrs(f.XSeamVisibleWhen, values))...)
	// §3b.7 fix, 2026-07-28: the client evaluator needs to know the
	// field's *original* schema required-ness to restore it correctly
	// the first time a field that started hidden becomes visible — it
	// can't be reconstructed from a "was it required before this
	// transition" DOM marker alone, since a field hidden from the very
	// first render never had `required` in its initial HTML to begin
	// with (caught by an actual jsdom run, not by reading the code).
	if f.XSeamVisibleWhen != nil && f.Required {
		rowAttrs = append(rowAttrs, mi.Attr("data-schema-required", "1"))
	}

	if !selfLabeledWidgets[widgetName] {
		if f.XSeamHideLabel {
			// visually hidden, still read by screen readers and still the field's click target
			rowAttrs = append(rowAttrs, b.Label(mi.For("field-"+f.Name), mi.Class("sr-only"), resolvedTitle))
		} else {
			rowAttrs = append(rowAttrs, fkPopoverAwareLabel(b, f, value, resolvedTitle, resolveWidgetChrome(e, rc, "field.fk_popover.view_edit", "View / edit")))
		}
	}

	rowAttrs = append(rowAttrs, widgetNode)

	if resolvedHelp != "" {
		rowAttrs = append(rowAttrs,
			b.P(mi.Class("mt-1 text-xs text-gray-500 dark:text-gray-400"), resolvedHelp),
		)
	}
	if hasError {
		rowAttrs = append(rowAttrs,
			b.P(mi.Class("mt-1 text-xs text-red-600"), errMsg),
		)
	}

	return b.Div(rowAttrs...)
}

// fkPopoverAwareLabel renders the field's label — plain text for most
// fields, or, when x-seam-fk-popover is set (§7), the richer label with
// an info-icon button that fires SeamPopover.show(this, fieldName).
// Matches internal/ui/asset_detail.go's assetFKSelectWithJump exactly
// (verified against its actual markup before implementing, not
// reconstructed from the spec prose alone): the button is display:none
// unless the current value is a positive ID, since §7.2 says the
// popover only appears once something is actually selected.
func fkPopoverAwareLabel(b *mi.Builder, f Field, value interface{}, title, jumpTitle string) mi.Node {
	// Required indicator driven systematically from f.Required, not
	// baked into the i18n-resolved title text -- resolved 2026-07-28,
	// the design question T-13 carried forward to FE-24. Text-embedded
	// asterisks would need every locale package to remember to add or
	// remove one per field with no single source of truth to check
	// against; f.Required already IS that source of truth, and driving
	// the indicator from it directly means region_id (labelled "*" in
	// the real static form but never actually HTML-required, FE-22's
	// finding) simply doesn't get one here either -- not a special
	// case decided in isolation, just what f.Required already says.
	// f, not effectiveField, deliberately -- same "stable regardless of
	// current visibility" semantic as data-schema-required above.
	displayTitle := title
	if f.Required {
		displayTitle += " *"
	}
	if f.XSeamFKPopover == "" {
		return b.Label(mi.For("field-"+f.Name), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300"), displayTitle)
	}
	jumpStyle := "display:none"
	if isPositiveID(value) {
		jumpStyle = ""
	}
	return b.Label(mi.For("field-"+f.Name),
		mi.Class("flex items-center text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"),
		b.Span(mi.Class("flex-1"), displayTitle),
		b.Button(
			mi.ID("jump-"+f.Name),
			mi.Type("button"),
			mi.Attr("onclick", "SeamPopover.show(this,"+jsStringLiteral(f.Name)+")"),
			mi.Title(jumpTitle),
			mi.Class("text-indigo-500 hover:text-indigo-700 dark:text-indigo-400 dark:hover:text-indigo-300 transition-colors"),
			mi.Attr("style", jumpStyle+";background:none;border:none;padding:0;cursor:pointer"),
			b.Span(mi.Class("material-icons"), mi.Attr("style", "font-size:20px;line-height:1;vertical-align:middle"), "info"),
		),
	)
}

// isPositiveID reports whether value represents an integer > 0 — the
// signal §7.2 uses to decide whether an FK field currently has a real
// selection (vs. the "— Sin asignar —" empty option, which resolves to
// 0 or an empty string, never a positive integer).
func isPositiveID(value interface{}) bool {
	switch v := value.(type) {
	case float64:
		return v > 0
	case int:
		return v > 0
	case int64:
		return v > 0
	case string:
		n, err := strconv.ParseInt(v, 10, 64)
		return err == nil && n > 0
	}
	return false
}

// wrapReadOnly disables interaction with a rendered widget without
// changing its markup structure — a full read-only rendering mode
// (rendering plain text instead of a disabled input) is one of the
// FieldPermission-driven behaviours stage 6 adds; this is the stage-1
// placeholder so ReadOnlyFields/rc.ReadOnly do something visible now
// rather than nothing until stage 6 lands.
func wrapReadOnly(b *mi.Builder, node mi.Node) mi.Node {
	return b.Div(mi.Class("opacity-60 pointer-events-none"), node)
}

// wrapWithUnit appends a unit-of-measure suffix beside a widget's
// control (T-10's "x-seam-unit" extension). A flex row, widget first
// then the unit label -- matches how a plain HTML input+suffix
// convention normally reads left-to-right, and keeps the widget's own
// markup untouched (no widget function needs to know about units at
// all). unit is schema-authored, not user input, so no escaping
// concern beyond minty's own default attribute/text escaping.
func wrapWithUnit(b *mi.Builder, node mi.Node, unit string) mi.Node {
	return b.Div(mi.Class("flex items-center gap-2"),
		b.Div(mi.Class("flex-1"), node),
		b.Span(mi.Class("text-sm text-gray-500 dark:text-gray-400 whitespace-nowrap"), unit),
	)
}

func unsupportedWidgetNode(b *mi.Builder, f Field, widgetName string) mi.Node {
	return b.Div(mi.Class("mb-4"),
		b.P(mi.Class("text-xs text-amber-600"),
			"formengine: field \""+f.Name+"\" needs widget \""+widgetName+"\", not implemented before stage 3/4"),
	)
}

// errorBanner renders a visible, non-crashing error state for a schema
// that failed to parse or compile. See Render's doc comment (engine.go)
// for why this exists instead of a panic or an error return.
func errorBanner(err error) mi.H {
	return func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("rounded-lg border border-red-300 bg-red-50 dark:bg-red-900/20 p-4 text-sm text-red-700 dark:text-red-400"),
			"formengine: "+err.Error(),
		)
	}
}

func toSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, i := range items {
		set[i] = true
	}
	return set
}

// visibilityAttrs returns the attributes an element with a visible_when
// / x-seam-visible-when condition needs: nothing if vw is nil (no
// condition — nothing to toggle), otherwise a data-visible-when carrying
// the condition as JSON (for the client evaluator, §3b.6) plus, when the
// condition doesn't currently hold, an inline display:none (§3b.6 —
// toggled via display, never removed from the DOM, so the client
// evaluator can reveal it again without a round-trip).
func visibilityAttrs(vw *VisibleWhen, values map[string]interface{}) []mi.Attribute {
	if vw == nil {
		return nil
	}
	encoded, err := json.Marshal(vw)
	if err != nil {
		// A condition that fails to re-encode is a formengine bug, not
		// a schema-authoring one (it parsed fine to get here) — degrade
		// to "no condition" rather than fail the whole render over it.
		return nil
	}
	attrs := []mi.Attribute{mi.Attr("data-visible-when", string(encoded))}
	if !EvaluateVisibleWhen(vw, values) {
		attrs = append(attrs, mi.Attr("style", "display:none"))
	}
	return attrs
}
