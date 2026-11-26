package ui

import (
	"regexp"
	"strings"
	"testing"
)

// The Assets form's Photos section is titled once, by the engine's section title: the field's own
// label is only there for screen readers.
func TestAssetEditDynamic_PhotosSectionIsTitledOnce(t *testing.T) {
	body := assetFormBody(t, "/assets/dynamic/7/edit")
	label := regexp.MustCompile(`<label[^>]*for="field-photos"[^>]*>`).FindString(body)
	if label == "" || !strings.Contains(label, "sr-only") {
		t.Errorf("the photos label must be screen-reader only, got %q", label)
	}
}
