package ui

// docLinkRowRemoveOnclick is the onclick handler for a document-link row's
// remove (x) button. The add button clones an existing row, so before the
// clicked row is removed it stashes a clone on the container
// (c._rowTemplate); the add handler falls back to that stash when no row is
// left, instead of throwing on null. Same handler as formengine's
// rowRemoveOnclick("doc-link-row"), duplicated because that helper is
// unexported.
const docLinkRowRemoveOnclick = `(function(b){
  var r=b.closest('.doc-link-row'),c=r.parentNode;
  if(!c._rowTemplate){c._rowTemplate=r.cloneNode(true);}
  r.remove();
})(this)`
