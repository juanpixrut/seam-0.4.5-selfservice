package formengine

import (
	"context"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"
)

// x-seam-hide-label keeps a field's label for screen readers only: a field that a section title
// already names (the Assets form's photo gallery) is not titled twice.
func TestHideLabel_LabelIsScreenReaderOnly(t *testing.T) {
	form, err := ParseSchema([]byte(`{"type":"object","properties":{
		"photos":{"type":"string","title":"Photos","x-seam-input":"image","x-seam-hide-label":true},
		"name":{"type":"string","title":"Name"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	row := func(name string) string {
		for _, f := range form.Fields {
			if f.Name == name {
				return mi.RenderToString(func(b *mi.Builder) mi.Node {
					return renderFieldRow(b, context.Background(), &FormEngine{}, f, nil, "", false, RenderContext{})
				})
			}
		}
		t.Fatalf("no field %s", name)
		return ""
	}
	photos, name := row("photos"), row("name")
	if !strings.Contains(photos, `for="field-photos"`) || !strings.Contains(photos, "sr-only") || strings.Contains(photos, "font-medium") {
		t.Errorf("the photos label must be there for screen readers but not visible:\n%s", photos)
	}
	if strings.Contains(name, "sr-only") || !strings.Contains(name, "font-medium") {
		t.Errorf("an ordinary field keeps its visible label:\n%s", name)
	}
}
