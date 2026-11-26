package formengine

import (
	"context"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"
)

type hostileTranslator struct{}

func (hostileTranslator) T(locale, key string, args ...any) string {
	switch key {
	case "select.search", "select.no_results", "action.reorder":
		return `"><script>alert(1)</script><x a="`
	}
	return key
}

// Adversarial: a translation string reaches attributes; it must be escaped.
func TestChromeTranslationsAreEscapedInAttributes(t *testing.T) {
	e := &FormEngine{Translator: hostileTranslator{}}
	rc := RenderContext{Locale: "en"}
	f := Field{Name: "unit", Title: "Unit", JSONType: "string", XSeamInput: "select", Enum: []string{"a"}}
	html := mi.RenderToString(renderSelect(context.Background(), e, f, "", false, rc))
	rf := reorderField(t, `"x-seam-reorder": true,`)
	html += mi.RenderToString(renderRepeatable(context.Background(), e, rf, nil, false, rc))
	if strings.Contains(html, "<script>alert(1)") {
		t.Errorf("a translation broke out of its attribute:\n%s", html)
	}
	if !strings.Contains(html, "&lt;script&gt;") && !strings.Contains(html, "&#34;&gt;&lt;script") {
		t.Errorf("expected the hostile text, escaped, in the output:\n%s", html)
	}
}
