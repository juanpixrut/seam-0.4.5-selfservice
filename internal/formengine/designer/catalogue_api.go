package designer

import (
	"encoding/json"

	fe "github.com/ha1tch/seam-ui/internal/formengine"
)

// Widgets returns every widget, in the order the palette shows them.
func Widgets() []Widget { return append([]Widget(nil), widgets...) }

// LookupWidget returns the widget called name.
func LookupWidget(name string) (Widget, bool) {
	for _, w := range widgets {
		if w.Name == name {
			return w, true
		}
	}
	return Widget{}, false
}

// Categories returns the palette's headings in order, each with its widgets.
func Categories() []struct {
	Name    string
	Widgets []Widget
} {
	var out []struct {
		Name    string
		Widgets []Widget
	}
	for _, w := range widgets {
		if n := len(out); n == 0 || out[n-1].Name != w.Category {
			out = append(out, struct {
				Name    string
				Widgets []Widget
			}{Name: w.Category})
		}
		out[len(out)-1].Widgets = append(out[len(out)-1].Widgets, w)
	}
	return out
}

// Props returns every property.
func Props() []Prop { return append([]Prop(nil), props...) }

// LookupProp returns the property called key.
func LookupProp(key string) (Prop, bool) {
	for _, p := range props {
		if p.Key == key {
			return p, true
		}
	}
	return Prop{}, false
}

// PropsFor returns the properties that apply to widget, in catalogue order.
func PropsFor(widget string) []Prop {
	var out []Prop
	for _, p := range props {
		if len(p.Widgets) == 0 {
			out = append(out, p)
			continue
		}
		for _, w := range p.Widgets {
			if w == widget {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

// WidgetOf is the widget the engine renders for a field's property object: its
// x-seam-input when it has one, otherwise what its type and format imply. It
// agrees with the engine's own inference (a test holds it to that).
func WidgetOf(property []byte) string {
	var p struct {
		Type   string   `json:"type"`
		Format string   `json:"format"`
		Enum   []string `json:"enum"`
		Input  string   `json:"x-seam-input"`
	}
	if json.Unmarshal(property, &p) != nil {
		return "text"
	}
	f := fe.Field{JSONType: p.Type, Format: p.Format, Enum: p.Enum, XSeamInput: p.Input}
	return fe.InferWidget(f)
}
