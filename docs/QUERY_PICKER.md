# query-picker: mini manual

Updated: 2026-11-17

Status: **built** (T-111). Verified against a live xolu 0.30.44 and in a browser,
light and dark; see "How it was verified". Written in English per the repository's
documentation convention.

## What it is

A form engine widget for choosing ONE record from the result of an OQL query.
Where `any-picker` shows an id and a label from one entity, `query-picker` shows
several columns of different types (text, numbers, money, dates, badges) in a
dropdown that looks like a small list, and stores one column as the field's value.

Use it when the person choosing needs more than a name to pick correctly: an asset
with its type, status and price, a model with its brand.

Use `any-picker` when a name is enough. It is simpler.

Try it: the "Query picker" tab of the Pickers page of the Demo module (`/demo/pickers?tab=query_picker`, `features.enable_demo`).

## Quick start

```json
"asset_id": {
  "type": "integer",
  "title": "Asset",
  "x-seam-input": "query-picker",
  "x-seam-query": "SELECT id, tag_tecnico, name, type_id, purchase_price, status FROM assets WHERE ((name LIKE :q) OR (tag_tecnico LIKE :q)) ORDER BY name ASC",
  "x-seam-label-fields": ["tag_tecnico", "name"],
  "x-seam-columns": [
    {"field": "tag_tecnico", "muted": true},
    {"field": "name", "key": true},
    {"field": "type_id", "lookup": "asset_types.name"},
    {"field": "purchase_price", "type": "money", "currency": "USD"},
    {"field": "status", "type": "badge", "variants": {"active": "success", "maintenance": "warning"}}
  ]
}
```

That is a complete field. The person types, a dropdown lists matching assets with
five columns, and choosing a row stores its `id`.

## Schema keys

| Key | Required | Meaning |
|---|---|---|
| `x-seam-input` | yes | `"query-picker"` |
| `x-seam-query` | yes | The OQL query. See "Writing the query". |
| `x-seam-columns` | yes | The columns to show, in order, 1 to 12. See "Columns". |
| `x-seam-value-field` | no | The result column stored as the field's value. Default `id`. |
| `x-seam-label-fields` | no | Columns joined with " · " to form the text shown in the box after choosing. Default: the column marked `key`, else the first column. |
| `x-seam-dropdown` | no | `"table"` or `"rows"`. Default: table when the field is wide enough for its columns, rows otherwise. |
| `x-seam-detail` | no | `true` shows the other columns of the chosen row in a line under the box. Default `false`. |
| `x-seam-limit` | no | Rows in the dropdown. Default 8, at most 50. |
| `x-seam-selector` | no | `"query"` adds a Browse button that opens the Selector dialog on the field's own query, with search and paging. |
| `x-seam-browse` | no | How the Browse button looks: `button` (default), `icon`, `link`. Same as `any-picker`. |
| `x-seam-highlight` | no | How the match is marked: `color` (default), `background`, `none`. Same as `any-picker`. |
| `x-seam-placeholder` | no | Placeholder of the search box. |

`required` works as on any field. The field's JSON `type` decides how a value is
compared: `integer` and `number` as a number, anything else as text.

## Columns

Each entry of `x-seam-columns`:

| Property | Meaning |
|---|---|
| `field` | Name of a column the query returns. Required. |
| `type` | `text` (default), `number`, `money`, `date`, `datetime`, `bool`, `badge`, `icon`. |
| `title` | Literal column heading, used only when no translation key exists. |
| `key` | `true` on the main column: shown in bold, first line of the rows layout. At most one. |
| `muted` | `true` shows the cell in the secondary color (codes, dates). |
| `lookup` | `"<entity>.<column>"`: the cell holds an id and shows that column of the referenced record. See "Lookups instead of joins". |
| `currency` | For `money`: an ISO code, or the name of a result column that holds one. |
| `variants` | For `badge`: a map from value to `success`, `warning`, `danger`, `info` or `neutral`. Unlisted values get the neutral badge. |

How each type is drawn:

| Type | Table layout | Notes |
|---|---|---|
| `text` | left | the match is marked |
| `number` | right, tabular figures | |
| `money` | right, tabular figures | two decimals, currency code before the amount |
| `date` | left, muted | `YYYY-MM-DD` |
| `datetime` | left, muted | `YYYY-MM-DD HH:MM` |
| `bool` | a tick or a dash | |
| `badge` | the standard badge | |
| `icon` | a Material icon by name | |

In the rows layout the `key` column is the first line; text, money and dates form
the second line, joined with " · "; numbers, badges and icons sit at the right.

## Writing the query

The query is checked when the schema is parsed and again every time it runs. A
query that breaks a rule makes the schema fail to load, and the Developer module
refuses to save it; the message names the field and the rule.

**Rules**

1. One `SELECT`, starting the text. No `;`, no comments, no subquery, no `UNION`,
   `INTERSECT` or `EXCEPT`, and nothing that changes data.
2. **Name the columns. `*` in the SELECT list is refused.** xolu cannot push
   `SELECT *` down and reads the whole entity to answer it (T-110).
3. Every `LIKE` in its own parentheses: `((a LIKE :q) OR (b LIKE :q))`, and
   `(a LIKE :q)` even when it stands alone. In xolu 0.30.44 a `LIKE` followed by
   `AND` or `OR` without them matches nothing (T-109). The guard refuses a `LIKE`
   that shares its parentheses with an `AND` or `OR`, so this cannot happen silently.
4. Do not filter business entities by tenant. Seam's xolu client is already scoped
   to the session's tenant, and entities such as `assets` have no `tenant_id`
   column: a condition on it matches nothing. `:tenant_id` exists for the few
   entities that store one.
5. Tables must be entities a picker may read (see the list below). `users`,
   credentials, settings and the accounting tables are not among them. Tables are
   written plainly, never quoted or bracketed, and joined with `JOIN`, not a comma.
6. At most one `JOIN`, and only when a lookup cannot do the job.
7. Do not write `TOP`, `LIMIT` or `OFFSET`. The server adds `TOP` from `x-seam-limit`.
8. Include an `ORDER BY`, so the rows shown are the first ones and not arbitrary ones.
9. No `GROUP BY` or `HAVING`.
10. An expression in the SELECT list needs `AS <name>`; a plain column, with or
    without a table alias, does not. Two columns may not share a name.
11. The result must include the value column and every column named in
    `x-seam-columns` and `x-seam-label-fields`.
12. At most 2,000 characters.

**Parameters**

Values reach the query only as named parameters, filled in as escaped literals,
and only outside quotes.

| Parameter | Value |
|---|---|
| `:q` | The search text wrapped for `LIKE`: `'%text%'`. |
| `:q_raw` | The search text as typed, for an exact comparison. |
| `:tenant_id` | The session's tenant, for entities that store a `tenant_id`. See rule 4. |
| `:user_id` | The session's user. |

**Entities a picker may read**

`alert_types`, `alerts`, `asset_types`, `assets`, `brands`, `consumable`,
`consumable_types`, `cost_centers`, `crews`, `currencies`, `custodians`, `issue_templates`,
`issues`, `locations`, `models`, `outsourced`, `prop_definitions`, `regions`, `sensors`,
`unit`, `warehouse`, `wo_actions`, `work_order_templates`, `work_orders`.

A module that owns another entity adds it at start-up with
`querypicker.AllowTable("<entity>")`.

## How the query stays on the server

The browser never receives the query and never sends one. When the form engine
renders the field it seals the field's specification (query, columns, limit) into
an opaque token: compressed, then encrypted and authenticated with a key only the
server holds. The page carries that token; each search sends it back with the
typed text. The server opens the token, checks the query against the rules again,
and runs it. A token that was altered, or made elsewhere, is refused.

The key is derived from the deployment's session secret (or the JWT secret), so
every instance, and one instance after a restart, can open each other's tokens.
With no secret configured the key is random per process: a page rendered before a
restart must then be reloaded.

## Lookups instead of joins

A column that holds an id of a small entity (a type, a brand, a location) should
use `lookup`, not a `JOIN`:

```json
{"field": "type_id", "lookup": "asset_types.name"}
```

The server reads the referenced entity once (`id` and that one column), keeps it
for 60 seconds per tenant, and fills in the names. The query stays on one table.
An entity with more than 2,000 rows is not held; its cells then show the id, and
a `JOIN` is the right tool.

Measured on xolu 0.30.44 with 10,000 assets (median, sequential requests on a
2-core machine; relative cost only):

| Picker query | No entity schema | With entity schema |
|---|---|---|
| One table, named columns | 25.9 ms | 2.6 ms |
| Same with a JOIN to the type | 70.4 ms | 11.3 ms |
| The whole lookup table | 1.6 ms | 0.8 ms |

Use a `JOIN` only when the search itself must look at the second table's columns.
xolu supports `INNER`, `LEFT`, `RIGHT` and `FULL OUTER` between two plain
entities; not three tables, not `CROSS JOIN`, not a subquery on either side.

## Keeping it cheap

`LIKE '%text%'` reads every row of the entity on each request. The widget and the
server limit how often that happens:

- the widget waits 250 ms after the last keystroke;
- it needs at least 2 characters (the server enforces the same minimum);
- an answer overtaken by a newer request is dropped, and the request is cancelled;
- an identical search within 5 seconds is answered from memory, shared by every
  user of the tenant unless the query uses `:user_id`;
- the dropdown asks for `x-seam-limit` rows plus one, never a count.

What the schema author controls:

- name only the columns the dropdown shows;
- prefer `lookup` to `JOIN`;
- search as few columns as give a useful match;
- register an xolu entity schema for large entities (see
  the `schema` records of `seeds/seed-01-constructec.seed` and T-110).

## What the person sees

| State | Behavior |
|---|---|
| Typing | After two characters the dropdown opens under the box. The match is marked in each text cell. |
| Table layout | A heading row and one row per result. Up and Down move, Enter chooses, Esc closes. |
| Rows layout | Two lines per result, for narrow fields and repeatable cells. |
| More results than shown | The footer reads "Showing the first 8. More match: keep typing", and mentions Browse when the field has it. There is no total: counting would cost a second query per keystroke. |
| No results | "No results". |
| Chosen | The box shows the label. With `x-seam-detail`, a line under it shows the other columns. |
| Text edited after choosing | The stored value is cleared until a row is chosen again. |
| Browse | The Selector dialog opens with the same columns and headings, a search box and Previous/Next, ten rows per page. |

The dropdown is wider than the box when its columns need the room, and never wider
than the window.

## Values and validation

- The field stores the value column of the chosen row and nothing else.
- Queries run through the session's tenant-scoped xolu client, like every other
  Seam query.
- When a form opens with a stored value, the widget asks the server for that
  value's row and shows its label (one request per field). A record the query no
  longer returns keeps its raw value and is marked "no longer available"; it is
  not cleared.
- **Checking a submitted value is the handler's call.** `h.QueryPickerErrors(ctx,
  form, values, t)` runs each field's query restricted to the posted value and
  reports any value the query would not return, so a hand-made request cannot
  store a record the person could not have chosen. It walks repeatables. A handler
  that saves a form with query-picker fields must call it, next to its other
  validation; the Demo's echo page does.

## Translations

Column headings are looked up in this order:

1. `<module>.forms.<form>.field.<field>.column.<column>`
2. `query_picker.column.<column>`
3. the column's `title`
4. the column name

The headings are resolved when the field is rendered and sealed with it, so the
dropdown and the Browse dialog show the same ones.

Widget texts are under `field.query_picker.*` (`more`, `more_browse`, `no_results`,
`unavailable`, `failed`, `keys`, `select`, `invalid`). Every key a shipped schema
uses goes in all four locale files (`en`, `es`, `pt`, `ja`).

## In a repeatable

`query-picker` works as a repeatable cell. A repeatable that holds one uses indexed
field names (`lines[0][asset_id]`), like one that holds an `any-picker`. Rows added
after load get a working picker. With no layout set, a cell uses the rows layout
unless it is wide enough for the table.

## Examples

**Rows layout, a lookup, no Browse**

```json
"model_id": {
  "type": "integer",
  "title": "Model",
  "x-seam-input": "query-picker",
  "x-seam-query": "SELECT id, name, brand_id FROM models WHERE (name LIKE :q) ORDER BY name ASC",
  "x-seam-columns": [
    {"field": "name", "key": true},
    {"field": "brand_id", "lookup": "brands.name"}
  ],
  "x-seam-dropdown": "rows"
}
```

**A join, because the search must match the type's name; Browse as an icon**

```json
"asset_id": {
  "type": "integer",
  "title": "Asset by type",
  "x-seam-input": "query-picker",
  "x-seam-query": "SELECT a.id, a.name, t.name AS type_name, a.status FROM assets a JOIN asset_types t ON a.type_id = t.id WHERE ((a.name LIKE :q) OR (t.name LIKE :q)) ORDER BY a.name ASC",
  "x-seam-columns": [
    {"field": "name", "key": true},
    {"field": "type_name"},
    {"field": "status", "type": "badge"}
  ],
  "x-seam-limit": 5,
  "x-seam-selector": "query",
  "x-seam-browse": "icon"
}
```

**A text value column**

```json
"asset_tag": {
  "type": "string",
  "title": "Asset tag",
  "x-seam-input": "query-picker",
  "x-seam-query": "SELECT tag_tecnico, name FROM assets WHERE ((name LIKE :q) OR (tag_tecnico LIKE :q)) ORDER BY name ASC",
  "x-seam-value-field": "tag_tecnico",
  "x-seam-columns": [
    {"field": "tag_tecnico", "key": true},
    {"field": "name"}
  ]
}
```

## Checklist before shipping a field

- [ ] Columns named, no `*`
- [ ] Every `LIKE` in its own parentheses
- [ ] No tenant filter on business entities
- [ ] `ORDER BY` present, no `TOP`
- [ ] Lookups used for id columns; at most one `JOIN`
- [ ] Value column and every shown column are in the `SELECT`
- [ ] At most one column marked `key`
- [ ] Headings translated in the four locales
- [ ] The handler that saves the form calls `QueryPickerErrors`
- [ ] Tried in a narrow column and inside a repeatable if it will be used there

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| The form shows an error instead of fields | The query breaks a rule; the message names the field and the rule. |
| The dropdown is always empty | A filter on a column the entity does not have, such as `tenant_id` on `assets` (rule 4), or the entity has no rows yet. |
| Rows appear in an unexpected order | The `ORDER BY` names another column. |
| A column shows an id | It needs `lookup`, the lookup's entity or column name is wrong, or the entity has more than 2,000 rows. |
| The dropdown is slow | `LIKE` over a large entity without an entity schema, a `JOIN` where a lookup would do, or too many search columns. |
| "no longer available" under the box | The stored record no longer matches the query (for example it is no longer active). |
| The search fails after a restart | No session or JWT secret is configured, so the key changed; reload the page. |
| Submit is rejected | The posted value is not a row the query returns for this session. |

## Where the code is

| Part | File |
|---|---|
| Query rules, builders, sealing, row formatting | `internal/querypicker/` |
| Checks shared with accounting's OQL formulas | `internal/oqlguard/` |
| Schema keys and renderer | `internal/formengine/query_picker.go`, `schema.go` |
| Search, value lookup, Browse, submitted-value check | `internal/ui/picker_query.go` |
| Dropdown | `SeamWidgets`' `query-picker` kind in `web/static/js/app.js`; `.fe-qp-*` in `web/static/css/app.css` |
| Demo | `internal/formengine/schemas/demo/query_picker.json` |

## How it was verified

- Unit tests in `internal/querypicker`, `internal/formengine` and `internal/ui`
  (the last against a fake xolu that records every query).
- A real xolu 0.30.44 seeded with `seeds/seed-01-constructec.py`, a real Seam with
  the Demo module on, and a headless Chrome driving the Demo tab: 53 checks over
  typing, keyboard and pointer choice, both layouts, lookups, the join, Browse and
  its paging, a repeatable row added after load, the dark theme, submit, a forged
  value, a stale value, a tampered token, an injection attempt and a request with
  no session. That script is not in the repository yet (T-111).

## Not included

- Choosing several records.
- Parameters taken from other fields of the form (a picker that depends on another
  field).
- Creating a record from the dropdown.
- Graph (Sulpher) queries.
- A total count of matches.
- Dates and amounts formatted for the session locale.

## Related

- `docs/SELECTOR_SPEC.md`: the dialog Browse opens.
- `docs/FORM_ENGINE_SPEC.md`: `any-picker` and the other widgets.
- `docs/LIST_SELECTION.md`: how list rows are selected.
- `docs/TRACKING.md`: T-109, T-110, T-111.
