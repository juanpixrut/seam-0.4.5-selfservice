package designer

// The vocabulary of the form designer (campaign DEV-21): every widget the form
// engine can render and every property a field can carry, with what the designer
// needs to offer them: a title, help, the kind of value, the allowed values and
// where each property applies. It is written by hand and held to the engine by
// tests, so that the palette and the inspector are generated from it and cannot
// drift from what the engine understands:
//
//   - every widget used in an embedded schema is here, and each widget's template
//     parses and renders;
//   - every property tag the schema parser reads is here, and every property here
//     is one the parser reads;
//   - classifying a property with WidgetOf gives what the engine itself infers.
//
// The titles and help are English; the designer page looks each up as the
// translation key designer.widget.<name>.title / designer.prop.<key>.help first.

// Kind is the kind of value a property holds. It decides how the inspector edits
// it and how a posted text becomes JSON.
type Kind string

const (
	KindBool   Kind = "bool"   // true; absent means false
	KindInt    Kind = "int"    // a whole number
	KindString Kind = "string" // free text
	KindEnum   Kind = "enum"   // one of Prop.Enum
	KindColor  Kind = "color"  // a name from Prop.Enum, or #rgb / #rrggbb
	KindIcon   Kind = "icon"   // a Material Icons name: lower-case letters, digits, underscores
	KindList   Kind = "list"   // an array of strings, one per line in the inspector
	KindMap    Kind = "map"    // an object of strings, "key=value" per line in the inspector
	KindJSON   Kind = "json"   // any JSON value, edited as JSON text
)

// Prop describes one property of a field.
type Prop struct {
	Key, Title, Help string
	Kind             Kind
	Enum             []string // allowed values of KindEnum; suggested names for KindColor
	Widgets          []string // the widgets it applies to; empty means all of them
	// Validation marks a property the validator enforces (queryfy) and the form
	// engine's own parser does not read: JSON Schema's minLength and maxLength.
	Validation bool
}

// Widget describes one widget.
type Widget struct {
	Name, Title, Icon, Category, Summary string
	// Template is a minimal property that makes the engine render this widget,
	// which the designer adds when the widget is chosen from the palette.
	Template string
}

var widgets = []Widget{
	{Name: "text", Title: "Text", Icon: "short_text", Category: "Text", Summary: "A single line of text.",
		Template: `{"type":"string","title":"Text","x-seam-input":"text"}`},
	{Name: "textarea", Title: "Long text", Icon: "notes", Category: "Text", Summary: "Several lines of text.",
		Template: `{"type":"string","title":"Notes","x-seam-input":"textarea","x-seam-rows":4}`},
	{Name: "password", Title: "Password", Icon: "password", Category: "Text", Summary: "A masked text entry.",
		Template: `{"type":"string","title":"Password","x-seam-input":"password"}`},
	{Name: "email", Title: "Email", Icon: "alternate_email", Category: "Text", Summary: "An email address.",
		Template: `{"type":"string","format":"email","title":"Email"}`},
	{Name: "tel", Title: "Phone", Icon: "call", Category: "Text", Summary: "A telephone number.",
		Template: `{"type":"string","format":"tel","title":"Phone"}`},
	{Name: "url", Title: "Link", Icon: "link", Category: "Text", Summary: "A web address.",
		Template: `{"type":"string","format":"uri","title":"Link"}`},
	{Name: "number", Title: "Number", Icon: "pin", Category: "Numbers and dates", Summary: "A whole or decimal number.",
		Template: `{"type":"integer","title":"Number"}`},
	{Name: "decimal", Title: "Decimal", Icon: "calculate", Category: "Numbers and dates", Summary: "An exact decimal with fixed precision.",
		Template: `{"type":"string","format":"decimal","title":"Amount","decimalPrecision":12,"decimalScale":2}`},
	{Name: "date", Title: "Date", Icon: "event", Category: "Numbers and dates", Summary: "A calendar date.",
		Template: `{"type":"string","format":"date","title":"Date"}`},
	{Name: "datetime-local", Title: "Date and time", Icon: "schedule", Category: "Numbers and dates", Summary: "A date with a time.",
		Template: `{"type":"string","format":"date-time","title":"Date and time"}`},
	{Name: "time", Title: "Time", Icon: "access_time", Category: "Numbers and dates", Summary: "A time of day.",
		Template: `{"type":"string","format":"time","title":"Time"}`},
	{Name: "checkbox", Title: "Checkbox", Icon: "check_box", Category: "Choices", Summary: "A yes or no tick box.",
		Template: `{"type":"boolean","title":"Yes or no"}`},
	{Name: "toggle", Title: "Switch", Icon: "toggle_on", Category: "Choices", Summary: "A yes or no switch.",
		Template: `{"type":"boolean","title":"Switch","x-seam-input":"toggle"}`},
	{Name: "multi-select", Title: "Multiple choice", Icon: "checklist", Category: "Choices", Summary: "Several values from a list, as a group of tick boxes.",
		Template: `{"type":"array","title":"Options","x-seam-input":"multi-select","items":{"type":"string"},"enum":["one","two"]}`},
	{Name: "color", Title: "Colour", Icon: "palette", Category: "Choices", Summary: "A colour chosen with the native picker.",
		Template: `{"type":"string","title":"Colour","x-seam-input":"color"}`},
	{Name: "select", Title: "Select", Icon: "arrow_drop_down_circle", Category: "Choices", Summary: "One value from a list, with search and icons.",
		Template: `{"type":"string","title":"Choice","x-seam-input":"select","enum":["one","two"]}`},
	{Name: "any-picker", Title: "Record picker", Icon: "manage_search", Category: "Pickers", Summary: "Search and choose a record of any entity.",
		Template: `{"type":"integer","title":"Approver","x-seam-input":"any-picker","x-seam-source":"entity:users","x-seam-selector":"users","x-seam-placeholder":"Approver"}`},
	{Name: "query-picker", Title: "Query picker", Icon: "table_rows", Category: "Pickers", Summary: "Search and choose a record from an OQL query, shown with several typed columns.",
		Template: `{"type":"integer","title":"Asset","x-seam-input":"query-picker","x-seam-query":"SELECT id, name, tag_tecnico, status FROM assets WHERE ((name LIKE :q) OR (tag_tecnico LIKE :q)) ORDER BY name ASC","x-seam-columns":[{"field":"name","key":true},{"field":"tag_tecnico","muted":true},{"field":"status","type":"badge"}]}`},
	{Name: "asset-picker", Title: "Asset picker", Icon: "precision_manufacturing", Category: "Pickers", Summary: "Search and choose an asset.",
		Template: `{"type":"integer","title":"Parent ID","x-seam-input":"asset-picker","x-seam-source":"entity:assets","x-seam-relation":"contains"}`},
	{Name: "asset-type-select", Title: "Asset type", Icon: "category", Category: "Pickers", Summary: "Choose an asset type.",
		Template: `{"type":"integer","title":"Asset Type","x-seam-input":"asset-type-select","x-seam-source":"entity:asset-types"}`},
	{Name: "icon-picker", Title: "Icon picker", Icon: "emoji_symbols", Category: "Pickers", Summary: "Search and choose an icon.",
		Template: `{"type":"string","title":"Icon","x-seam-input":"icon-picker"}`},
	{Name: "document-links", Title: "Document links", Icon: "attach_file", Category: "Rich", Summary: "A list of links to documents.",
		Template: `{"type":"array","title":"Loan Links","x-seam-input":"document-links"}`},
	{Name: "signature", Title: "Signature", Icon: "draw", Category: "Rich", Summary: "A drawn signature.",
		Template: `{"type":"string","title":"Technician Signature","x-seam-input":"signature"}`},
	{Name: "barcode", Title: "Barcode", Icon: "qr_code_2", Category: "Rich", Summary: "A barcode drawn from the value.",
		Template: `{"type":"string","title":"Barcode","x-seam-input":"barcode"}`},
	{Name: "gps", Title: "Map location", Icon: "location_on", Category: "Rich", Summary: "A latitude and longitude chosen on a map.",
		Template: `{"type":"string","title":"Location","x-seam-input":"gps"}`},
	{Name: "image", Title: "Photos", Icon: "photo_library", Category: "Rich", Summary: "A drag-and-drop photo gallery; needs a saved record.",
		Template: `{"type":"string","title":"Photos","x-seam-input":"image"}`},
	{Name: "repeatable", Title: "Repeatable rows", Icon: "repeat", Category: "Rich", Summary: "A list of rows, each with its own fields; reorderable and bounded.",
		Template: `{"type":"array","title":"Rows","x-seam-input":"repeatable","items":{"type":"object","properties":{"name":{"type":"string","title":"Name","x-seam-input":"text"}}}}`},
}

var colours = []string{"indigo", "emerald", "amber", "rose", "sky", "violet", "teal", "slate"}

var props = []Prop{
	// every widget
	{Key: "x-seam-help", Title: "Help text", Help: "A line of help shown with the field.", Kind: KindString},
	{Key: "x-seam-help-style", Title: "Help style", Help: "Whether the help is always shown (inline) or on hover (tooltip).", Kind: KindEnum, Enum: []string{"inline", "tooltip"}},
	{Key: "x-seam-visible-when", Title: "Visible when", Help: "A condition on other fields, as JSON, under which the field is shown.", Kind: KindJSON},
	{Key: "x-seam-hidden-reject", Title: "Reject when hidden", Help: "Refuse a value posted for the field while it is hidden.", Kind: KindBool},
	{Key: "x-seam-unit", Title: "Unit", Help: "A unit shown after the control, for example kg. Display only.", Kind: KindString},
	{Key: "x-seam-erp-code", Title: "ERP code", Help: "An external system's own code for the field. Not used by the form.", Kind: KindString},
	{Key: "x-seam-custom-validator", Title: "Custom validator", Help: "The name of a validator registered in the engine.", Kind: KindString},
	// validation (read by the validator, not by the engine's parser)
	{Key: "minLength", Title: "Shortest text", Help: "The fewest characters allowed.", Kind: KindInt, Validation: true,
		Widgets: []string{"text", "textarea", "password", "email", "tel", "url"}},
	{Key: "maxLength", Title: "Longest text", Help: "The most characters allowed.", Kind: KindInt, Validation: true,
		Widgets: []string{"text", "textarea", "password", "email", "tel", "url", "date", "datetime-local", "time"}},
	// text-like
	{Key: "x-seam-placeholder", Title: "Placeholder", Help: "Text shown in an empty control; for a select, the text of its empty option.", Kind: KindString,
		Widgets: []string{"text", "textarea", "password", "email", "tel", "url", "number", "decimal", "select", "any-picker", "icon-picker", "asset-picker", "query-picker"}},
	{Key: "x-seam-autocomplete", Title: "Autocomplete", Help: "The browser autocomplete hint, for example new-password.", Kind: KindString,
		Widgets: []string{"text", "password", "email", "tel", "url"}},
	{Key: "x-seam-input-mask", Title: "Input mask", Help: "9 digit, A letter (upper-cased), a letter, * letter or digit; anything else is a literal.", Kind: KindString,
		Widgets: []string{"text", "tel"}},
	{Key: "x-seam-rows", Title: "Rows", Help: "The height of the text area, in lines.", Kind: KindInt, Widgets: []string{"textarea"}},
	// decimal
	{Key: "decimalPrecision", Title: "Total digits", Help: "The most digits the number may have.", Kind: KindInt, Widgets: []string{"decimal"}},
	{Key: "decimalScale", Title: "Decimal places", Help: "The digits after the decimal point.", Kind: KindInt, Widgets: []string{"decimal"}},
	// select and pickers
	{Key: "x-seam-source", Title: "Source", Help: "Where the options come from, for example entity:users.", Kind: KindString, Widgets: []string{"select", "any-picker", "asset-picker"}},
	{Key: "x-seam-option-groups", Title: "Group options", Help: "Group the options of a select under headings.", Kind: KindBool, Widgets: []string{"select"}},
	{Key: "x-seam-option-icons", Title: "Option icons", Help: "An icon for each option, as value=icon_name, one per line.", Kind: KindMap, Widgets: []string{"select"}},
	{Key: "x-seam-icon-position", Title: "Icon position", Help: "Whether an option's icon is before (left) or after (right) its text.", Kind: KindEnum, Enum: []string{"left", "right"}, Widgets: []string{"select"}},
	{Key: "x-seam-fk-popover", Title: "Preview popover", Help: "The base URL of the entity whose preview a select shows.", Kind: KindString, Widgets: []string{"select"}},
	{Key: "x-seam-selector", Title: "Selector", Help: "The Selector source to browse, for example users; adds a Browse button. On a query-picker, write query to browse the field's own query.", Kind: KindString, Widgets: []string{"any-picker", "query-picker"}},
	{Key: "x-seam-browse", Title: "Browse button", Help: "How the Browse control looks.", Kind: KindEnum, Enum: []string{"button", "icon", "link"}, Widgets: []string{"any-picker", "query-picker"}},
	{Key: "x-seam-highlight", Title: "Match highlight", Help: "How the matched text of a search result is marked.", Kind: KindEnum, Enum: []string{"color", "background", "none"}, Widgets: []string{"any-picker", "icon-picker", "asset-picker", "query-picker"}},
	{Key: "x-seam-search-fields", Title: "Search fields", Help: "The fields the search looks in, one per line.", Kind: KindList, Widgets: []string{"any-picker", "select"}},
	{Key: "x-seam-display-fields", Title: "Display fields", Help: "The fields shown for each result, one per line.", Kind: KindList, Widgets: []string{"any-picker", "select"}},
	{Key: "x-seam-relation", Title: "Relation", Help: "The kind of link of the asset graph to follow.", Kind: KindEnum, Enum: []string{"contains", "uses"}, Widgets: []string{"asset-picker"}},
	// query-picker (docs/QUERY_PICKER.md)
	{Key: "x-seam-query", Title: "Query", Help: "One read-only OQL SELECT with named columns, every LIKE in its own parentheses, an ORDER BY and no TOP. Parameters: :q, :q_raw, :tenant_id, :user_id.", Kind: KindString, Widgets: []string{"query-picker"}},
	{Key: "x-seam-columns", Title: "Columns", Help: "The columns of the dropdown, as a JSON array of {field, type, title, key, muted, lookup, currency, variants}.", Kind: KindJSON, Widgets: []string{"query-picker"}},
	{Key: "x-seam-value-field", Title: "Value column", Help: "The result column stored as the field's value; id when empty.", Kind: KindString, Widgets: []string{"query-picker"}},
	{Key: "x-seam-label-fields", Title: "Label columns", Help: "The result columns shown in the box once a row is chosen, one per line.", Kind: KindList, Widgets: []string{"query-picker"}},
	{Key: "x-seam-dropdown", Title: "Dropdown layout", Help: "table shows a heading and one line per result; rows shows two lines per result. Empty chooses by the field's width.", Kind: KindEnum, Enum: []string{"table", "rows"}, Widgets: []string{"query-picker"}},
	{Key: "x-seam-detail", Title: "Show detail", Help: "Show the other columns of the chosen row under the box.", Kind: KindBool, Widgets: []string{"query-picker"}},
	{Key: "x-seam-limit", Title: "Rows shown", Help: "How many results the dropdown lists: 8 when empty, at most 50.", Kind: KindInt, Widgets: []string{"query-picker"}},
	// barcode
	{Key: "x-seam-barcode-format", Title: "Barcode format", Help: "The symbology of the barcode.", Kind: KindEnum, Enum: []string{"EAN13", "EAN8", "UPC", "UPCE"}, Widgets: []string{"barcode"}},
	{Key: "x-seam-panel", Title: "Map panel", Help: "How the map opens: a dropdown under the pin button, or the full width of the field.", Kind: KindEnum, Enum: []string{"dropdown", "wide"}, Widgets: []string{"gps"}},
	{Key: "x-seam-hide-label", Title: "Hide label", Help: "Keep the label for screen readers only: use it when a section title already names the field.", Kind: KindBool},
	// repeatable
	{Key: "x-seam-layout", Title: "Row layout", Help: "row puts the cells on one line; form makes each row a card with labelled cells.", Kind: KindEnum, Enum: []string{"row", "form"}, Widgets: []string{"repeatable"}},
	{Key: "x-seam-naming", Title: "Field naming", Help: "indexed posts each row under its own index, needed for nested rows and reordering.", Kind: KindEnum, Enum: []string{"indexed"}, Widgets: []string{"repeatable"}},
	{Key: "x-seam-reorder", Title: "Reorderable", Help: "Let the rows be dragged into a new order.", Kind: KindBool, Widgets: []string{"repeatable"}},
	{Key: "minItems", Title: "Fewest rows", Help: "The fewest rows allowed; the list shows at least this many.", Kind: KindInt, Widgets: []string{"repeatable"}},
	{Key: "maxItems", Title: "Most rows", Help: "The most rows allowed; Add is disabled at this number.", Kind: KindInt, Widgets: []string{"repeatable"}},
	{Key: "x-seam-add-label", Title: "Add button text", Help: "The text of the button that adds a row.", Kind: KindString, Widgets: []string{"repeatable"}},
	{Key: "x-seam-add-icon", Title: "Add button icon", Help: "A Material Icons name.", Kind: KindIcon, Widgets: []string{"repeatable"}},
	{Key: "x-seam-add-style", Title: "Add button style", Help: "How the add button looks.", Kind: KindEnum, Enum: []string{"outline", "filled", "link", "icon"}, Widgets: []string{"repeatable"}},
	{Key: "x-seam-remove-label", Title: "Remove button text", Help: "Text beside the remove icon of each row.", Kind: KindString, Widgets: []string{"repeatable"}},
	{Key: "x-seam-remove-icon", Title: "Remove button icon", Help: "A Material Icons name.", Kind: KindIcon, Widgets: []string{"repeatable"}},
	{Key: "x-seam-remove-color", Title: "Remove button colour", Help: "A colour name or #rrggbb.", Kind: KindColor, Enum: colours, Widgets: []string{"repeatable"}},
	{Key: "x-seam-accent", Title: "Accent colour", Help: "The colour of the rows, the add button and the nesting line.", Kind: KindColor, Enum: colours, Widgets: []string{"repeatable"}},
}
