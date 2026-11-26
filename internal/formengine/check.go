package formengine

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrNotRenderable is wrapped by every problem CheckRenderable reports.
var ErrNotRenderable = errors.New("formengine: the form cannot be rendered")

// sourceWidgets are the widgets whose options come from the engine's
// SourceResolver when the field names an x-seam-source. Other widgets that
// carry the key (any-picker, asset-picker) fetch their records themselves.
var sourceWidgets = map[string]bool{"select": true, "multi-select": true}

// CheckRenderable reports what would make a definition that parses fail when it
// is rendered: a form with no field, a field whose widget the engine does not
// have, and a select whose x-seam-source the page cannot resolve. ParseSchema
// accepts all three, and the engine then draws an error line in place of the
// field (it never panics), which is right for a page but wrong for a save: the
// editors use this so that such a definition is refused before it replaces a
// working form.
//
// sourceOK says whether a source can be resolved where the form is rendered;
// nil skips that check. Fields inside repeatables are checked too. Every
// problem is reported, not only the first.
func CheckRenderable(schema []byte, sourceOK func(source string) bool) error {
	form, err := ParseSchema(schema)
	if err != nil {
		return err
	}
	var problems []string
	if len(form.Fields) == 0 {
		problems = append(problems, "the form has no fields")
	}
	var walk func(prefix string, fields []Field)
	walk = func(prefix string, fields []Field) {
		for _, f := range fields {
			name := prefix + f.Name
			widget := inferWidget(f)
			if _, ok := widgetRegistry[widget]; !ok {
				problems = append(problems, fmt.Sprintf("field %q: the engine has no widget %q", name, widget))
			}
			if f.XSeamSource != "" && sourceWidgets[widget] && sourceOK != nil && !sourceOK(f.XSeamSource) {
				problems = append(problems, fmt.Sprintf("field %q: the source %q is not available to this form", name, f.XSeamSource))
			}
			if len(f.Items) > 0 {
				walk(name+".", f.Items)
			}
		}
	}
	walk("", form.Fields)
	if len(problems) > 0 {
		return fmt.Errorf("%w: %s", ErrNotRenderable, strings.Join(problems, "; "))
	}
	return nil
}

// SourcesOf lists the x-seam-source values the definition's select widgets
// use, sorted, including those inside repeatables. A definition that does not
// parse has none.
func SourcesOf(schema []byte) []string {
	form, err := ParseSchema(schema)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var walk func(fields []Field)
	walk = func(fields []Field) {
		for _, f := range fields {
			if f.XSeamSource != "" && sourceWidgets[inferWidget(f)] {
				seen[f.XSeamSource] = true
			}
			walk(f.Items)
		}
	}
	walk(form.Fields)
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
