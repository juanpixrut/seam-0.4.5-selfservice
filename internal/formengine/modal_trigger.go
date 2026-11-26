package formengine

import (
	"encoding/json"
	"fmt"

	mi "github.com/ha1tch/minty"
)

// widthPixels maps §3.9's x-seam-form.width vocabulary to the literal
// CSS width SeamModal2.open's opts.maxWidth expects. Confirmed against
// SeamModal2's actual implementation in web/static/js/app.js, which
// does `_modal.style.width = opts.maxWidth` directly — a CSS length,
// not a Tailwind class name, even though §3.9's own table expresses the
// same vocabulary as Tailwind max-w-* classes for the non-modal
// (full-page) rendering case. Approximate pixel values taken from that
// same table.
var widthPixels = map[string]string{
	"sm": "512px", "md": "672px", "lg": "896px", "xl": "1152px", "2xl": "1280px",
}

// ModalTriggerAttrs returns the htmx/onclick attributes for a button
// that opens schema's form in whichever modal its x-seam-form.width
// calls for (§15.4, confirmed decision): "fullscreen" opens SeamModal,
// anything else opens SeamModal2 sized from §3.9's width table.
// Handlers never choose which modal to use directly — the schema does,
// via this function, so changing width in the schema changes trigger
// behaviour with no handler code change.
//
// modal_width (§3.9's deprecated alias for width) is honoured with the
// same precedence parseSchema itself would apply: width wins if both
// are present.
func ModalTriggerAttrs(schema []byte, url, label string) []mi.Attribute {
	width := formWidth(schema)
	escLabel := jsStringLiteral(label)

	if width == "fullscreen" {
		return []mi.Attribute{
			mi.Attr("hx-get", url),
			mi.Attr("hx-target", "#seam-modal-body"),
			mi.Attr("hx-swap", "innerHTML"),
			mi.Attr("onclick", "SeamModal.open("+escLabel+")"),
		}
	}

	px, ok := widthPixels[width]
	if !ok {
		px = widthPixels["md"] // §3.9's stated default width
	}
	onclick := fmt.Sprintf("SeamModal2.open(%s,%s,{maxWidth:%s})",
		escLabel, jsStringLiteral(url), jsStringLiteral(px))
	return []mi.Attribute{mi.Attr("onclick", onclick)}
}

// formWidth reads x-seam-form.width (or the deprecated modal_width
// alias) directly from raw schema bytes — ModalTriggerAttrs is meant to
// be callable by a handler building a trigger button (e.g. on a listing
// page) before it necessarily calls Render() for that schema at all, so
// this doesn't go through the main parseSchema/Form pipeline.
func formWidth(raw []byte) string {
	var doc struct {
		XSeamForm *struct {
			Width      string `json:"width"`
			ModalWidth string `json:"modal_width"`
		} `json:"x-seam-form"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || doc.XSeamForm == nil {
		return ""
	}
	if doc.XSeamForm.Width != "" {
		return doc.XSeamForm.Width
	}
	return doc.XSeamForm.ModalWidth
}
