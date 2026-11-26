// Package developer embeds the formengine JSON Schemas for the Developer
// module's own input forms: the translation editor, the new text, the
// form-definition editor, the new form and the role to view the application as.
// They are schemas, not hand-built HTML, for the
// same reason every other module's forms are: validation, labels and layout
// come from one place, and the module's forms can themselves be overridden
// with the Developer module's own form-definition layer.
package developer

import (
	"embed"
	"sort"
	"strings"
)

//go:embed translation_edit.json translation_new.json schema_edit.json form_new.json view_as.json
var schemaFS embed.FS

// Schema returns the raw JSON Schema bytes for one of the module's forms.
// Panics on an unknown name: a small, fixed, compile-time-known set.
func Schema(name string) []byte {
	data, err := schemaFS.ReadFile(name + ".json")
	if err != nil {
		panic("developer: unknown schema " + name + ": " + err.Error())
	}
	return data
}

// Names lists the embedded schemas, sorted.
func Names() []string {
	entries, _ := schemaFS.ReadDir(".")
	var out []string
	for _, e := range entries {
		if n := e.Name(); strings.HasSuffix(n, ".json") {
			out = append(out, strings.TrimSuffix(n, ".json"))
		}
	}
	sort.Strings(out)
	return out
}
