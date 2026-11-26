package formengine

import (
	"context"
	"fmt"

	mi "github.com/ha1tch/minty"
)

// renderDocumentLinksLegacy is the previous hand-coded look of the "document-links" widget, kept behind
// Options.DocumentLinksLegacy. It implements the "document-links" widget (§4.2's
// document-links entry): a dynamic list of {nombre, url, tipo} rows the
// operator can add and remove. Field naming deliberately matches the
// existing hand-coded DocumentLinksField's parallel-array convention
// (name_nombre[], name_url[], name_tipo[]) rather than a nested-object
// naming scheme — §13.3's migration is meant to convert forms without
// requiring their POST handlers to change in the same pass, and the
// existing handlers already parse this exact field-name shape.
// Generalised over f.Items rather than hardcoding nombre/url/tipo, so
// it isn't silently wrong if a schema ever orders or renames them.
func renderDocumentLinksLegacy(ctx context.Context, e *FormEngine, f Field, value interface{}, hasError bool, rc RenderContext) mi.H {
	return func(b *mi.Builder) mi.Node {
		rows := arrayOfMaps(value)
		if len(rows) == 0 {
			rows = []map[string]interface{}{{}} // always at least one empty row, matching DocumentLinksField
		}
		containerID := "field-" + f.Name + "-container"
		var rowNodes []interface{}
		for _, row := range rows {
			rowNodes = append(rowNodes, documentLinkRow(b, f, row, e, rc))
		}
		addOnclick := fmt.Sprintf(`(function(){
  var c=document.getElementById(%s);
  var src=c._rowTemplate||c.querySelector('.doc-link-row');
  if(!src){return;}
  var t=src.cloneNode(true);
  t.querySelectorAll('input').forEach(function(i){i.value='';});
  c.appendChild(t);
})()`, jsStringLiteral(containerID))
		return b.Div(
			b.Div(append([]interface{}{mi.ID(containerID), mi.Class("space-y-2")}, rowNodes...)...),
			b.Button(mi.Type("button"),
				mi.Class("mt-1 text-sm text-indigo-600 dark:text-indigo-400 hover:underline"),
				mi.Attr("onclick", addOnclick),
				resolveWidgetChrome(e, rc, "action.add_link", "+ Add link"),
			),
		)
	}
}

func documentLinkRow(b *mi.Builder, f Field, row map[string]interface{}, e *FormEngine, rc RenderContext) mi.Node {
	const inputClass = "px-2 py-1 border border-gray-300 dark:border-gray-600 rounded text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-white flex-1"
	var inputs []interface{}
	for _, item := range f.Items {
		inputType := "text"
		if item.Format == "uri" {
			inputType = "url"
		}
		inputs = append(inputs, b.Input(
			mi.Type(inputType),
			mi.Name(f.Name+"_"+item.Name+"[]"),
			mi.Attr("placeholder", resolveDocumentLinkItemLabel(e, rc, item.Name, item.Title)),
			mi.Value(stringValue(row[item.Name])),
			mi.Class(inputClass),
		))
	}
	inputs = append(inputs, b.Button(mi.Type("button"),
		mi.Class("text-red-500 hover:text-red-700 text-sm px-2"),
		mi.Attr("onclick", rowRemoveOnclick("doc-link-row")),
		"\u2715",
	))
	return b.Div(append([]interface{}{mi.Class("flex gap-2 items-start doc-link-row")}, inputs...)...)
}
