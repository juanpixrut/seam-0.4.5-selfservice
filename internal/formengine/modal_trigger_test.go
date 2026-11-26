package formengine

import (
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"
)

func renderAttrs(attrs []mi.Attribute) string {
	h := func(b *mi.Builder) mi.Node {
		args := make([]interface{}, len(attrs))
		for i, a := range attrs {
			args[i] = a
		}
		return b.Button(args...)
	}
	return mi.RenderToString(h)
}

func TestModalTriggerAttrs_Fullscreen_UsesSeamModal(t *testing.T) {
	schema := []byte(`{"x-seam-form": {"width": "fullscreen"}}`)
	html := renderAttrs(ModalTriggerAttrs(schema, "/assets/new", "Nuevo Activo"))
	if !strings.Contains(html, `hx-get="/assets/new"`) {
		t.Errorf("expected hx-get for the fullscreen (SeamModal) path:\n%s", html)
	}
	if !strings.Contains(html, `hx-target="#seam-modal-body"`) {
		t.Errorf("expected hx-target=#seam-modal-body:\n%s", html)
	}
	if !strings.Contains(html, "SeamModal.open(") {
		t.Errorf("expected SeamModal.open(...), not SeamModal2:\n%s", html)
	}
	if strings.Contains(html, "SeamModal2") {
		t.Errorf("fullscreen must not use SeamModal2:\n%s", html)
	}
}

func TestModalTriggerAttrs_NonFullscreen_UsesSeamModal2(t *testing.T) {
	schema := []byte(`{"x-seam-form": {"width": "2xl"}}`)
	html := renderAttrs(ModalTriggerAttrs(schema, "/assets/new", "Nuevo Activo"))
	if !strings.Contains(html, "SeamModal2.open(") {
		t.Errorf("expected SeamModal2.open(...):\n%s", html)
	}
	if strings.Contains(html, "hx-get") {
		t.Errorf("SeamModal2 path handles its own fetch internally -- no hx-get needed:\n%s", html)
	}
	if !strings.Contains(html, "1280px") {
		t.Errorf("expected the 2xl width mapped to 1280px:\n%s", html)
	}
}

func TestModalTriggerAttrs_NoWidth_DefaultsToMd(t *testing.T) {
	schema := []byte(`{"x-seam-form": {}}`)
	html := renderAttrs(ModalTriggerAttrs(schema, "/x", "Label"))
	if !strings.Contains(html, "672px") {
		t.Errorf("expected the md default (672px) when width is unset:\n%s", html)
	}
}

func TestModalTriggerAttrs_DeprecatedModalWidthHonoured(t *testing.T) {
	schema := []byte(`{"x-seam-form": {"modal_width": "lg"}}`)
	html := renderAttrs(ModalTriggerAttrs(schema, "/x", "Label"))
	if !strings.Contains(html, "896px") {
		t.Errorf("expected modal_width:lg (deprecated alias) to resolve to 896px:\n%s", html)
	}
}

func TestModalTriggerAttrs_WidthTakesPrecedenceOverModalWidth(t *testing.T) {
	// §3.9: "Si ambas están presentes, width tiene precedencia."
	schema := []byte(`{"x-seam-form": {"width": "sm", "modal_width": "2xl"}}`)
	html := renderAttrs(ModalTriggerAttrs(schema, "/x", "Label"))
	if !strings.Contains(html, "512px") {
		t.Errorf("width (sm -> 512px) should win over modal_width (2xl):\n%s", html)
	}
	if strings.Contains(html, "1280px") {
		t.Errorf("modal_width should be ignored once width is present:\n%s", html)
	}
}

func TestModalTriggerAttrs_LabelAndURLEscaped(t *testing.T) {
	schema := []byte(`{"x-seam-form": {"width": "md"}}`)
	html := renderAttrs(ModalTriggerAttrs(schema, "/x", "It's a test"))
	// minty HTML-entity-encodes apostrophes in attribute values (&#39;),
	// so the JS-escaped backslash-quote from jsStringLiteral shows up as
	// \&#39; in the serialised HTML -- browsers decode &#39; back to '
	// before the JS parses it, so this is the correct encoded form, not
	// a literal backslash-apostrophe.
	if !strings.Contains(html, `It\&#39;s a test`) {
		t.Errorf("expected the apostrophe in the label to be JS-escaped then HTML-entity-encoded:\n%s", html)
	}
}
