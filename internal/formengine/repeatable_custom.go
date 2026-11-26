package formengine

import (
	"regexp"
	"strings"

	mi "github.com/ha1tch/minty"
)

// Repeatable variants: the add and remove buttons and the accent colour of a
// repeatable, driven by these schema properties on the repeatable field:
//
//	x-seam-add-label     text of the add button (literal, not an i18n key)
//	x-seam-add-icon      Material Icons name of the add button
//	x-seam-add-style     "outline" (default) | "filled" | "link" | "icon"
//	x-seam-remove-icon   Material Icons name of the remove button
//	x-seam-remove-label  text beside the remove icon (default: icon only)
//	x-seam-remove-color  colour of the remove button
//	x-seam-accent        colour of the rows, add button and nested rule
//
// Colours are a name from repColors or a #rgb / #rrggbb value, nothing else:
// the value ends up in a style attribute, so anything not recognised is
// ignored (the theme colour stays), never copied through. Icon names must be
// lower-case letters, digits and underscores for the same reason.

// repColors are the named colours a schema can use.
var repColors = map[string]string{
	"indigo":  "#6366f1",
	"emerald": "#10b981",
	"amber":   "#f59e0b",
	"rose":    "#f43f5e",
	"sky":     "#0ea5e9",
	"violet":  "#8b5cf6",
	"teal":    "#14b8a6",
	"slate":   "#64748b",
}

var (
	repHexColor = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)
	repIconName = regexp.MustCompile(`^[a-z0-9_]{1,40}$`)
)

// repColor resolves a colour name or hex value; "" means "not set / invalid".
func repColor(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if c, ok := repColors[v]; ok {
		return c
	}
	if repHexColor.MatchString(v) {
		return v
	}
	return ""
}

// repIcon returns name when it is a plausible Material Icons name, else def.
func repIcon(name, def string) string {
	if repIconName.MatchString(name) {
		return name
	}
	return def
}

// repAddStyle is the add button's style (x-seam-add-style): "outline"
// (default), "filled", "link" or "icon".
func repAddStyle(f Field) string {
	switch f.XSeamAddStyle {
	case "filled", "link", "icon":
		return f.XSeamAddStyle
	}
	return "outline"
}

// repAccentAttrs are the wrapper attributes that carry the accent colour; nil
// when the repeatable has none (the output is then exactly what it was).
func repAccentAttrs(f Field) []interface{} {
	c := repColor(f.XSeamAccent)
	if c == "" {
		return nil
	}
	return []interface{}{mi.Attr("data-fe-accent", "1"), mi.Attr("style", "--fe-accent:"+c)}
}

// repAddButton is the "Add" button of a repeatable: by default the outlined
// button with the add icon, the same look as the Identity Fields' "Add Field".
func repAddButton(b *mi.Builder, e *FormEngine, rc RenderContext, f Field, onclick string) mi.Node {
	label := f.XSeamAddLabel
	if label == "" {
		label = resolveWidgetChrome(e, rc, "action.add", "Add")
	}
	icon := b.Span(mi.Class("material-icons"), repIcon(f.XSeamAddIcon, "add"))
	cls := "fe-btn fe-add"
	attrs := []interface{}{mi.Type("button")}
	switch repAddStyle(f) {
	case "filled":
		cls += " fe-btn--filled"
	case "link":
		cls += " fe-btn--link"
	case "icon":
		cls += " fe-btn--icon"
	}
	attrs = append(attrs, mi.Class(cls), mi.Attr("onclick", onclick))
	if f.addLocked {
		attrs = append(attrs, mi.Attr("disabled", "disabled"))
	}
	if repAddStyle(f) == "icon" {
		attrs = append(attrs, mi.Attr("aria-label", label), mi.Attr("title", label), icon)
	} else {
		attrs = append(attrs, icon, label)
	}
	return b.Button(attrs...)
}

// repRemoveButton is the remove button of a row: by default the red
// remove-circle icon button used by the Identity Fields, with a title and an
// accessible name. A label (x-seam-remove-label) shows beside the icon.
func repRemoveButton(b *mi.Builder, e *FormEngine, rc RenderContext, f Field, onclick string) mi.Node {
	label := f.XSeamRemoveLabel
	visible := label != ""
	if !visible {
		label = resolveWidgetChrome(e, rc, "action.remove", "Remove")
	}
	cls := "fe-remove"
	if visible {
		cls += " fe-remove--text"
	}
	attrs := []interface{}{mi.Type("button"), mi.Class(cls),
		mi.Attr("aria-label", label), mi.Attr("title", label), mi.Attr("onclick", onclick)}
	if f.removeLocked {
		attrs = append(attrs, mi.Attr("disabled", "disabled"))
	}
	if c := repColor(f.XSeamRemoveColor); c != "" {
		attrs = append(attrs, mi.Attr("style", "--fe-remove:"+c+";--fe-remove-hover:"+c))
	}
	attrs = append(attrs, b.Span(mi.Class("material-icons"), repIcon(f.XSeamRemoveIcon, "remove_circle_outline")))
	if visible {
		attrs = append(attrs, b.Span(label))
	}
	return b.Button(attrs...)
}
