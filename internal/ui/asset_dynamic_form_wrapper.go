package ui

import (
	"fmt"

	mi "github.com/ha1tch/minty"
)

// wrapAssetDynamicForm wraps formengine's rendered field content
// (which deliberately never emits a <form> tag itself -- renderForm's
// own comment: "Buttons and the full-page vs. modal wrapper stay the
// caller's job") in the actual submittable form chrome.
//
// This was a real, previously-undiscovered gap, not a stylistic
// choice deferred until now: without this wrapper, the dynamic Asset
// form has no <form> element at all in its rendered HTML -- confirmed
// directly by rendering it and checking, not assumed. No amount of
// backend/logic testing (validation, persistence, translation) would
// ever have caught this, since it's purely about whether the HTML has
// a submit mechanism a browser can actually use.
//
// Mirrors AssetFormPage's own wrapper precisely (internal/ui/
// asset_detail.go), not a simplified reinvention -- deliberately
// reusing its exact pattern rather than the masters module's
// masterPartialFormWrapper, which targets a different modal system
// entirely (SeamModal2/#seam-modal2-body, not SeamModal/
// #seam-modal-body, which is what the Asset list/detail pages
// actually use, confirmed from assets.go/asset_detail.go directly).
// Includes a subtle, easy-to-miss detail present in the static
// original: the submit button, though outside the <form> and linked
// via form="asset-dynamic-form", needs its own duplicate hx-post/
// hx-include/hx-target/hx-swap attributes -- without them, HTMX's own
// click listener can race against the native form-submit event
// triggered by the form= attribute.
//
// action, method, isPartial, isEdit, assetID, and assetName are the
// caller's responsibility to supply correctly -- this function does
// not infer them from context.
//
// t is a translate function (typically h.translator.ForLocale(h.
// currentLocale())) used for this wrapper's own chrome text (buttons,
// delete confirmation) -- deliberately NOT threaded down into
// formengine's own rendered content, which already resolves its own
// translations independently via its own Translator/RenderContext.
// Found and fixed alongside the missing <form> tag above: this
// wrapper's own buttons were hardcoded Spanish text ("Cancelar",
// "Crear Activo") regardless of the requested locale -- confirmed
// directly by rendering the form with SEAM_FORCE_LANGUAGE=en and
// checking the actual output, not assumed from reading the code.
// Deliberately scoped to just this wrapper, not a general i18n sweep:
// AssetFormPage (the static form this dynamic one is intended to
// eventually replace, per T-01/FE-24) has its own, much more extensive
// hardcoded-Spanish field labels throughout, out of scope here since
// investing in that code's i18n isn't worthwhile ahead of its own
// retirement. conditionalDeleteButton/conditionalButtonText (asset_
// detail.go), shared with the static form, are deliberately left
// unchanged rather than given a t parameter that would force every
// existing caller -- including the static form's, which has no
// translator plumbing today -- to supply one; dynamicDeleteButton
// below is a small, separate, translated equivalent used only here.
func wrapAssetDynamicForm(t func(key string, args ...any) string, content mi.H, action, method string, isPartial, isEdit bool, assetID int64, assetName string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	return func(b *mi.Builder) mi.Node {
		var formAttrs []interface{}
		formAttrs = append(formAttrs, mi.Class("space-y-0"), mi.ID("asset-dynamic-form"))
		if isPartial {
			formAttrs = append(formAttrs,
				mi.Attr("hx-post", action),
				mi.Attr("hx-target", "#seam-modal-body"),
				mi.Attr("hx-swap", "innerHTML"),
			)
		} else {
			formAttrs = append(formAttrs, mi.Method(method), mi.Action(action))
		}
		formAttrs = append(formAttrs, content(b))

		saveOrCreateText := conditionalButtonText(isEdit, t("action.save"), t("action.create"))

		if isPartial {
			return b.Div(mi.Class("w-full"),
				b.Form(formAttrs...),
				b.Div(mi.Attr("data-modal-footer", ""), mi.Class("flex items-center justify-between w-full"),
					b.Div(dynamicDeleteButton(b, t, isEdit, assetID, assetName)),
					b.Div(mi.Class("flex gap-3"),
						b.Button(mi.Type("button"),
							mi.Class("px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 font-medium text-sm"),
							mi.Attr("onclick", "SeamModal.close()"),
							t("action.cancel"),
						),
						createAndAddPhotosButton(b, t, action, isEdit),
						b.Button(mi.Type("submit"),
							mi.Attr("form", "asset-dynamic-form"),
							mi.Attr("hx-post", action),
							mi.Attr("hx-include", "#asset-dynamic-form"),
							mi.Attr("hx-target", "#seam-modal-body"),
							mi.Attr("hx-swap", "innerHTML"),
							mi.Attr("onclick", createButtonReset),
							mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 font-medium text-sm"),
							saveOrCreateText,
						),
					),
				),
			)
		}

		// Full-page render: same visual envelope as the static form's
		// own non-modal path, so the dynamic form doesn't look
		// different depending on how it was reached.
		return b.Div(
			mi.Class("min-h-screen bg-gray-100 dark:bg-gray-900 flex items-start justify-center py-8 px-4"),
			b.Div(
				mi.Style("width:1290px;max-width:100%;min-height:500px;max-height:900px;overflow-y:auto;background:white;border-radius:10px;padding:20px 24px;"),
				b.Form(formAttrs...),
				b.Div(
					mi.Class("flex items-center justify-between pt-4 mt-2 border-t border-gray-200 dark:border-gray-700"),
					b.Div(dynamicDeleteButton(b, t, isEdit, assetID, assetName)),
					b.Div(mi.Class("flex gap-3"),
						b.A(mi.Href("/assets"),
							mi.Class("px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 font-medium text-sm"),
							t("action.cancel"),
						),
						b.Button(mi.Type("submit"), mi.Attr("form", "asset-dynamic-form"),
							mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 font-medium text-sm"),
							saveOrCreateText,
						),
					),
				),
			),
		)
	}
}

// dynamicDeleteButton mirrors conditionalDeleteButton (asset_detail.go)
// exactly in shape and behavior, differing only in using t for its two
// user-facing strings instead of hardcoded English -- kept separate
// rather than adding a t parameter to the shared original, which would
// force the static form's own, translator-less call sites to supply
// one too. See wrapAssetDynamicForm's own doc comment for why that
// wasn't done instead.
func dynamicDeleteButton(b *mi.Builder, t func(key string, args ...any) string, isEdit bool, assetID int64, assetName string) mi.Node {
	if !isEdit {
		return b.Span()
	}
	onclick := fmt.Sprintf(`SeamConfirm.showDanger('%s',function(){htmx.ajax('DELETE','/api/v1/assets/%d',{target:document.body,swap:'innerHTML'})},{dangerName:'%s'})`, t("confirm.delete"), assetID, assetName)
	return b.Button(
		mi.Type("button"),
		mi.Class("inline-flex items-center gap-1 px-4 py-2 border border-red-300 text-red-600 rounded-lg hover:bg-red-50 dark:hover:bg-red-900/20 text-sm font-medium"),
		mi.Attr("onclick", onclick),
		b.Span(mi.Class("material-icons text-base"), "delete"),
		t("action.delete"),
	)
}

// assetNameFromValues extracts "name" as a display string from a
// values map for the delete-confirmation dialog's dangerName --
// values comes from AssetValuesFromXolu, so "name" is always the
// English key regardless of what's persisted in xolu.
func assetNameFromValues(values map[string]interface{}) string {
	if values == nil {
		return ""
	}
	if v, ok := values["name"].(string); ok {
		return v
	}
	return fmt.Sprintf("%v", values["name"])
}
