package ui

import mi "github.com/ha1tch/minty"

// createAndAddPhotosButton is the optional second way to create an asset: it saves the new asset
// and stays in the modal on its edit form, opened on the Map & photos tab, because photos can only
// be attached to a record that exists (AssetCreateDynamic answers the _then value). The plain
// Create button keeps its behaviour (back to the list). In edit mode there is nothing to add.
//
// The footer buttons sit outside the <form> (associated with form="asset-dynamic-form"), and htmx
// sends the request from the form itself, so a value on the button (hx-vals, or its own name) never
// reaches the server. The choice therefore travels in a hidden field associated with the form: this
// button sets it to "photos" when pressed, and the plain Create button (createButtonReset) empties it.
func createAndAddPhotosButton(b *mi.Builder, t func(key string, args ...any) string, action string, isEdit bool) interface{} {
	if isEdit {
		return mi.Raw("")
	}
	return b.Span(mi.Attr("style", "display:contents"),
		b.Input(mi.Type("hidden"), mi.Attr("form", "asset-dynamic-form"), mi.Name("_then"), mi.ID("asset-dynamic-then"), mi.Value("")),
		b.Button(mi.Type("submit"),
			mi.Attr("form", "asset-dynamic-form"),
			mi.Attr("hx-post", action),
			mi.Attr("hx-include", "#asset-dynamic-form"),
			mi.Attr("hx-target", "#seam-modal-body"),
			mi.Attr("hx-swap", "innerHTML"),
			mi.Attr("onclick", "document.getElementById('asset-dynamic-then').value='photos'"),
			mi.Class("px-4 py-2 border border-indigo-600 text-indigo-600 dark:text-indigo-400 dark:border-indigo-400 rounded-lg hover:bg-indigo-50 dark:hover:bg-gray-700 font-medium text-sm"),
			t("action.create_and_add_photos"),
		),
	)
}

// createButtonReset is the onclick of the plain Create/Save button: it empties the _then choice, so
// pressing Create and add photos, being told a field is missing, and then pressing Create does not
// carry the earlier choice along. Harmless where the hidden field does not exist (edit mode).
const createButtonReset = "var t=document.getElementById('asset-dynamic-then');if(t){t.value='';}"
