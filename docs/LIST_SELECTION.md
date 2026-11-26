# How a list row is selected -- criteria

Status: proposed and applied to the Assets and Sensors lists and, automatically, to
every list that uses `ListCellLink` (the masters). Written in English per the
repository's documentation convention.

## The problem

Seam's lists selected their rows in three different ways: the whole row (the Selector
dialog), a link on the key field (the masters), or only a button at the far right (the
Assets list, the most used one, and Sensors). The same gesture did different things in
different lists.

## The criteria

1. **Navigation lists** (a row stands for a record that has a detail page): the key
   field (name, code, description) is a real link, and the **whole row is clickable**
   and follows that link. Edit/Delete stay as secondary icon buttons on the right;
   there is no "view" button, because the row and the key link already do that.
2. **Selection lists** (choose one record to fill a field: the Selector dialog): the
   whole row chooses, and a Choose button stays for keyboard and touch users.
3. **Cross-reference links** (an event row linking to its asset) do not make the row
   clickable: the row is not the thing the link points to.
4. **Lists whose rows open an editor in a modal** (admin users): not changed yet; the
   same rule would apply (row opens the modal that Edit opens).
5. **Never hijack an interactive element**: a click on a link, button, input, select,
   label or anything marked `data-no-row-click` keeps its own behaviour; a click that
   ends a text selection does nothing; Ctrl/Cmd/Shift-click opens the link in a new tab.
6. **The key link is the keyboard and screen-reader target** (Tab, Enter); the row click
   is a pointer convenience. A clickable row shows the pointer cursor.

## How it works

`ListCellLink(text, href)` renders the key link with `data-row-link`. `SeamList`
(`web/static/js/app.js`) listens for clicks on table rows that contain such a link and
applies rule 5; `tr:has(a[data-row-link])` gets the pointer cursor (`app.css`). A list
becomes consistent by using `ListCellLink` for its key field; nothing else to wire.

## Audit

| List | Before | Now |
| --- | --- | --- |
| Assets | view/edit buttons only | key link + row click, view button removed |
| Sensors | view button only | key link + row click, view button removed |
| Regions, currencies, cost centres, brands, custodians, alert types | key link only | key link + row click |
| Events | link to the asset | unchanged (rule 3) |
| Admin users | edit button (modal) | unchanged (rule 4) |
| Selector dialog | row + Choose button | unchanged (rule 2) |
