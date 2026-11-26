package ui

import (
	"net/http"
	"strings"

	mi "github.com/ha1tch/minty"

	"github.com/ha1tch/seam-ui/internal/formengine/designer"
)

// The designer's layout panel: the block tree above the fields (campaign
// DEV-21, ADMIN_MODULE_SPEC section 8). A form is flat until it is given a
// tab; from then on it is tabs, each a stack of rows, each row one to three
// sections side by side, each section a list of fields. The panel edits that
// tree: tabs and sections are added, named, ordered, put side by side and
// removed, fields are moved between sections by dragging them or with the
// "move to" choice, and a tab's detail panels and embedded forms are edited as
// the JSON lists they are. Every change is one action of the designer's form,
// applied by internal/formengine/designer (layout.go) and then judged by the
// engine like any other, so a layout it would not render is refused.
//
// Dragging needs no script file: a field carries its name, a section (or a
// field, to drop before it) its place, and dropping fills the form's two hidden
// fields and posts it. Everything a drag does can be done with the controls.

// designerDrop is the action a drop posts: the field, and where it landed
// ("tab/section", or "tab/section/field" to go before that field).
const designerDrop = "drop"

func splitPlace(s string) (tab, section, before string) {
	parts := strings.SplitN(s, "/", 3)
	for len(parts) < 3 {
		parts = append(parts, "")
	}
	return parts[0], parts[1], parts[2]
}

// applyLayoutAct applies a layout action; ok is false when the verb is not one.
func applyLayoutAct(doc *designer.Doc, verb, arg string, r *http.Request) (ok bool, err error) {
	tab, sec, _ := splitPlace(arg)
	v := func(name string) string { return strings.TrimSpace(r.FormValue(name)) }
	switch verb {
	case "tab-add":
		return true, doc.AddTab(v("new_tab_id"), v("new_tab_label"))
	case "tab-set":
		return true, doc.SetTab(arg, v("tab_id:"+arg), v("tab_label:"+arg))
	case "tab-up":
		return true, doc.MoveTab(arg, -1)
	case "tab-down":
		return true, doc.MoveTab(arg, 1)
	case "tab-remove":
		return true, doc.RemoveTab(arg)
	case "tab-dup":
		_, err := doc.DuplicateTab(arg)
		return true, err
	case "sec-dup":
		_, err := doc.DuplicateSection(tab, sec)
		return true, err
	case "sec-add":
		return true, doc.AddSection(arg, v("new_sec_id:"+arg), v("new_sec_label:"+arg))
	case "sec-set":
		return true, doc.SetSection(tab, sec, v("sec_label:"+arg), v("sec_layout:"+arg), v("sec_width:"+arg))
	case "sec-up", "sec-down", "sec-join", "sec-split":
		return true, doc.MoveSection(tab, sec, strings.TrimPrefix(verb, "sec-"))
	case "sec-remove":
		return true, doc.RemoveSection(tab, sec)
	case "place":
		t, s, _ := splitPlace(v("place_to:" + arg))
		return true, doc.PlaceField(arg, t, s, "")
	case "fup":
		return true, doc.MoveFieldInSection(arg, -1)
	case "fdown":
		return true, doc.MoveFieldInSection(arg, 1)
	case "extras":
		return true, doc.SetTabExtras(arg, r.FormValue("details:"+arg), r.FormValue("embeds:"+arg))
	case designerDrop:
		t, s, before := splitPlace(v("drop_to"))
		return true, doc.PlaceField(v("drop_field"), t, s, before)
	}
	return false, nil
}

const designerDropJS = "event.preventDefault();event.stopPropagation();var f=document.getElementById('" + designerFormID + "');" +
	"f.elements['drop_field'].value=event.dataTransfer.getData('text/plain');f.elements['drop_to'].value=this.dataset.target;f.submit()"

// dropTarget makes a node a place a field can be dropped on.
func dropTarget(target string) []interface{} {
	return []interface{}{mi.Attr("data-target", target),
		mi.Attr("ondragover", "event.preventDefault();this.classList.add('is-drop')"),
		mi.Attr("ondragleave", "this.classList.remove('is-drop')"),
		mi.Attr("ondrop", designerDropJS)}
}

// developerLayoutPanel renders the layout panel.
func developerLayoutPanel(b *mi.Builder, v designerView, t func(key string, args ...any) string) mi.Node {
	own := mi.Attr("form", designerFormID)
	noValidate := mi.Attr("formnovalidate", "formnovalidate")
	btn := func(act, icon, label string, danger bool) mi.Node {
		look := devIconBtn
		if danger {
			look = devIconDanger
		}
		return b.Button(mi.Type("submit"), own, noValidate, mi.Name("act"), mi.Value(act), mi.Class(look), mi.Attr("title", label), mi.Attr("aria-label", label+" "+act),
			b.Span(mi.Class("material-icons text-lg"), icon))
	}
	text := func(name, value, placeholder, width string) mi.Node {
		return b.Input(mi.Type("text"), own, mi.Name(name), mi.Value(value), mi.Attr("placeholder", placeholder), mi.Attr("aria-label", placeholder),
			mi.Attr("maxlength", "200"), mi.Attr("style", "width:"+width), mi.Class(devInputClass))
	}
	choice := func(name, selected string, values []string, label func(string) string, aria string) mi.Node {
		opts := []interface{}{own, mi.Name(name), mi.Class(devInputClass), mi.Attr("style", "width:auto"), mi.Attr("aria-label", aria)}
		for _, val := range values {
			o := []interface{}{mi.Value(val)}
			if val == selected {
				o = append(o, mi.Attr("selected", "selected"))
			}
			opts = append(opts, b.Option(append(o, label(val))...))
		}
		return b.Select(opts...)
	}
	addTab := b.Div(mi.Class("dev-lt-add"),
		text("new_tab_id", "", t("developer.designer.layout.tab_id"), "9rem"),
		text("new_tab_label", "", t("developer.designer.layout.label"), "12rem"),
		b.Button(mi.Type("submit"), own, noValidate, mi.Name("act"), mi.Value("tab-add"), mi.ID("dev-lt-add-tab"),
			mi.Class("inline-flex items-center gap-2 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700"),
			b.Span(mi.Class("material-icons text-lg"), "add"), b.Span(t("developer.designer.layout.add_tab"))))
	if len(v.Layout) == 0 {
		return developerPanel(b, "account_tree", t("developer.designer.layout"), "", nil, false,
			b.Div(mi.ID("dev-layout"), mi.Class("space-y-3"), b.P(mi.Class(devMutedClass), t("developer.designer.layout.flat")), addTab))
	}
	// every place a field can go, for the "move to" choice
	var places []string
	for _, tab := range v.Layout {
		for _, row := range tab.Rows {
			for _, sec := range row.Sections {
				places = append(places, tab.ID+"/"+sec.ID)
			}
		}
	}
	widgetIcon := map[string]string{}
	for _, f := range v.Fields {
		if w, ok := designer.LookupWidget(f.Widget); ok {
			widgetIcon[f.Name] = w.Icon
		}
	}
	body := []interface{}{mi.ID("dev-layout"), mi.Class("space-y-4")}
	for _, tab := range v.Layout {
		tab := tab
		head := b.Div(mi.Class("dev-lt-head"),
			b.Span(mi.Class("material-icons text-gray-400"), "tab"),
			text("tab_label:"+tab.ID, tab.Label, t("developer.designer.layout.label"), "12rem"),
			text("tab_id:"+tab.ID, tab.ID, t("developer.designer.layout.tab_id"), "9rem"),
			btn("tab-set:"+tab.ID, "check", t("developer.designer.layout.apply"), false),
			btn("tab-up:"+tab.ID, "arrow_upward", t("developer.designer.up"), false),
			btn("tab-down:"+tab.ID, "arrow_downward", t("developer.designer.down"), false),
			btn("tab-dup:"+tab.ID, "content_copy", t("developer.designer.layout.duplicate_tab"), false),
			btn("tab-remove:"+tab.ID, "delete", t("developer.designer.layout.remove_tab"), true))
		rows := []interface{}{mi.Class("space-y-2")}
		for _, row := range tab.Rows {
			cells := []interface{}{mi.Class("dev-lt-row")}
			for _, sec := range row.Sections {
				sec := sec
				place := tab.ID + "/" + sec.ID
				secHead := b.Div(mi.Class("dev-lt-head"),
					text("sec_label:"+place, sec.Label, t("developer.designer.layout.label"), "9rem"),
					choice("sec_layout:"+place, sec.Layout, designer.SectionLayouts, func(s string) string { return s }, t("developer.designer.layout.columns")),
					choice("sec_width:"+place, sec.Width, designer.RowWidths, func(s string) string {
						if s == "" {
							return t("developer.designer.layout.width_auto")
						}
						return s
					}, t("developer.designer.layout.width")),
					btn("sec-set:"+place, "check", t("developer.designer.layout.apply"), false),
					btn("sec-up:"+place, "arrow_upward", t("developer.designer.up"), false),
					btn("sec-down:"+place, "arrow_downward", t("developer.designer.down"), false),
					btn("sec-join:"+place, "view_column", t("developer.designer.layout.join"), false),
					btn("sec-split:"+place, "view_agenda", t("developer.designer.layout.split"), false),
					btn("sec-dup:"+place, "content_copy", t("developer.designer.layout.duplicate_section"), false),
					btn("sec-remove:"+place, "delete", t("developer.designer.layout.remove_section"), true))
				fields := append([]interface{}{mi.Class("dev-lt-fields")}, dropTarget(place)...)
				if len(sec.Fields) == 0 {
					fields = append(fields, b.Span(mi.Class(devMutedClass), t("developer.designer.layout.drop_here")))
				}
				for _, f := range sec.Fields {
					f := f
					cls := "dev-lt-field"
					if f == v.Selected {
						cls += " is-selected"
					}
					icon := widgetIcon[f]
					if icon == "" {
						icon = "help_outline"
					}
					chip := append([]interface{}{mi.Class(cls), mi.Attr("draggable", "true"), mi.Attr("data-field", f),
						mi.Attr("ondragstart", "event.dataTransfer.setData('text/plain',this.dataset.field);event.dataTransfer.effectAllowed='move'")},
						dropTarget(place+"/"+f)...)
					chip = append(chip,
						b.Span(mi.Class("material-icons text-base text-gray-400"), "drag_indicator"),
						b.Button(mi.Type("submit"), own, noValidate, mi.Name("act"), mi.Value("select:"+f), mi.Class("dev-lt-name"),
							b.Span(mi.Class("material-icons text-base"), icon), b.Span(mi.Class("font-mono text-xs"), f)),
						btn("fup:"+f, "keyboard_arrow_up", t("developer.designer.up"), false),
						btn("fdown:"+f, "keyboard_arrow_down", t("developer.designer.down"), false),
						choice("place_to:"+f, place, places, func(s string) string { return s }, t("developer.designer.layout.move_to")),
						btn("place:"+f, "drive_file_move", t("developer.designer.layout.move_to"), false))
					fields = append(fields, b.Div(chip...))
				}
				cells = append(cells, b.Div(mi.Class("dev-lt-sec"), mi.Attr("data-section", place),
					b.Div(mi.Class("font-mono text-xs text-gray-500 dark:text-gray-400"), sec.ID), secHead, b.Div(fields...)))
			}
			rows = append(rows, b.Div(cells...))
		}
		addSec := b.Div(mi.Class("dev-lt-add"),
			text("new_sec_id:"+tab.ID, "", t("developer.designer.layout.section_id"), "9rem"),
			text("new_sec_label:"+tab.ID, "", t("developer.designer.layout.label"), "12rem"),
			btn("sec-add:"+tab.ID, "add", t("developer.designer.layout.add_section"), false))
		extras := b.Details(mi.Class("dev-lt-extras"),
			b.Summary(t("developer.designer.layout.extras")),
			b.Div(mi.Class("space-y-2"),
				b.P(mi.Class(devMutedClass), t("developer.designer.layout.extras_note")),
				b.Label(mi.Class(devLabelClass), t("developer.designer.layout.details")),
				b.Textarea(own, mi.Name("details:"+tab.ID), mi.Attr("rows", "4"), mi.Attr("spellcheck", "false"), mi.Class(devInputClass+" font-mono text-xs"), developerPrettyList(tab.Details)),
				b.Label(mi.Class(devLabelClass), t("developer.designer.layout.embeds")),
				b.Textarea(own, mi.Name("embeds:"+tab.ID), mi.Attr("rows", "4"), mi.Attr("spellcheck", "false"), mi.Class(devInputClass+" font-mono text-xs"), developerPrettyList(tab.Embeds)),
				b.Button(mi.Type("submit"), own, noValidate, mi.Name("act"), mi.Value("extras:"+tab.ID),
					mi.Class("inline-flex items-center gap-2 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700"),
					t("developer.designer.layout.apply"))))
		body = append(body, b.Div(mi.Class("dev-lt-tab"), mi.Attr("data-tab", tab.ID), head, b.Div(rows...), addSec, extras))
	}
	body = append(body, addTab, b.P(mi.Class(devMutedClass), t("developer.designer.layout.hint")))
	return developerPanel(b, "account_tree", t("developer.designer.layout"), "", nil, false, b.Div(body...))
}

// developerPrettyList indents a JSON list for a textarea ("" stays empty).
func developerPrettyList(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	return strings.TrimRight(developerPretty([]byte(raw)), "\n")
}
