package formengine

import (
	"context"
	"html"
	"os"
	"regexp"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"

	"github.com/ha1tch/seam-ui/internal/querypicker"
)

// T-111: the "query-picker" widget (docs/QUERY_PICKER.md). The query is
// checked when the schema is parsed and reaches the page only as a sealed
// token; the markup is inert, for SeamWidgets' "query-picker" kind.

const queryPickerSchema = `{
	"type": "object",
	"x-seam-form": {"module": "work", "name": "order"},
	"properties": {
		"asset_id": {"type": "integer", "title": "Asset", "x-seam-input": "query-picker",
			"x-seam-query": "SELECT id, tag_tecnico, name, status FROM assets WHERE ((name LIKE :q) OR (tag_tecnico LIKE :q)) ORDER BY name ASC",
			"x-seam-columns": [{"field": "name", "key": true}, {"field": "tag_tecnico", "muted": true, "title": "Tag literal"}, {"field": "status", "type": "badge"}],
			"x-seam-label-fields": ["tag_tecnico", "name"], "x-seam-limit": 5, "x-seam-dropdown": "rows", "x-seam-detail": true,
			"x-seam-selector": "query"},
		"code": {"type": "string", "x-seam-input": "query-picker",
			"x-seam-query": "SELECT tag_tecnico, name FROM assets WHERE (name LIKE :q) ORDER BY name",
			"x-seam-columns": [{"field": "name"}], "x-seam-value-field": "tag_tecnico"},
		"lines": {"type": "array", "x-seam-input": "repeatable",
			"items": {"type": "object", "properties": {
				"asset_id": {"type": "integer", "x-seam-input": "query-picker",
					"x-seam-query": "SELECT id, name FROM assets WHERE (name LIKE :q) ORDER BY name",
					"x-seam-columns": [{"field": "name"}]},
				"hours": {"type": "integer"}}}}
	}
}`

func queryPickerForm(t *testing.T) *Form {
	t.Helper()
	form, err := ParseSchema([]byte(queryPickerSchema))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	return form
}

func fieldNamed(t *testing.T, form *Form, name string) Field {
	t.Helper()
	for _, f := range form.Fields {
		if f.Name == name {
			return f
		}
	}
	t.Fatalf("no field %q", name)
	return Field{}
}

func TestQueryPicker_Registered(t *testing.T) {
	if widgetRegistry["query-picker"] == nil {
		t.Fatal("query-picker is not registered")
	}
}

func TestParseSchema_BuildsTheQueryPickerSpec(t *testing.T) {
	form := queryPickerForm(t)
	asset := fieldNamed(t, form, "asset_id")
	s := asset.QueryPicker
	if s == nil {
		t.Fatal("asset_id has no QueryPicker spec")
	}
	if !s.ValueNumeric || s.ValueField != "id" || s.Limit != 5 || len(s.Columns) != 3 || strings.Join(s.LabelFields, ",") != "tag_tecnico,name" {
		t.Errorf("spec = %+v", s)
	}
	if asset.XSeamDropdown != "rows" || !asset.XSeamDetail {
		t.Errorf("dropdown %q detail %v", asset.XSeamDropdown, asset.XSeamDetail)
	}
	code := fieldNamed(t, form, "code").QueryPicker
	if code.ValueNumeric || code.ValueField != "tag_tecnico" || code.Limit != querypicker.DefaultLimit {
		t.Errorf("code spec = %+v", code)
	}
	if got := QueryPickerFields(form); len(got) != 3 || got[2].Name != "asset_id" {
		t.Errorf("QueryPickerFields found %d fields, want 3 with the repeatable's cell last", len(got))
	}
}

func TestParseSchema_AQueryThatBreaksARuleFailsTheSchema(t *testing.T) {
	for _, tc := range []struct{ name, field, want string }{
		{"select star", `{"type":"integer","x-seam-input":"query-picker","x-seam-query":"SELECT * FROM assets ORDER BY name","x-seam-columns":[{"field":"name"}]}`, "name the columns"},
		{"bare like", `{"type":"integer","x-seam-input":"query-picker","x-seam-query":"SELECT id, name FROM assets WHERE name LIKE :q OR tag_tecnico LIKE :q ORDER BY name","x-seam-columns":[{"field":"name"}]}`, "every LIKE needs its own parentheses"},
		{"users", `{"type":"integer","x-seam-input":"query-picker","x-seam-query":"SELECT id, name FROM users ORDER BY name","x-seam-columns":[{"field":"name"}]}`, "is not one a query-picker may read"},
		{"no query", `{"type":"integer","x-seam-input":"query-picker","x-seam-columns":[{"field":"name"}]}`, "needs a query"},
		{"no columns", `{"type":"integer","x-seam-input":"query-picker","x-seam-query":"SELECT id, name FROM assets ORDER BY name"}`, "at least one column"},
		{"bad layout", `{"type":"integer","x-seam-input":"query-picker","x-seam-query":"SELECT id, name FROM assets ORDER BY name","x-seam-columns":[{"field":"name"}],"x-seam-dropdown":"grid"}`, "x-seam-dropdown"},
	} {
		_, err := ParseSchema([]byte(`{"type":"object","properties":{"asset_id":` + tc.field + `}}`))
		if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), `"asset_id"`) {
			t.Errorf("%s: ParseSchema = %v, want an error naming the field and containing %q", tc.name, err, tc.want)
		}
	}
}

func TestParseSchema_AQueryOnAnotherWidgetIsIgnored(t *testing.T) {
	form, err := ParseSchema([]byte(`{"type":"object","properties":{"note":{"type":"string","x-seam-query":"DROP TABLE assets"}}}`))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	if form.Fields[0].QueryPicker != nil {
		t.Error("a field that is not a query-picker got a spec")
	}
}

var qpTokenRE = regexp.MustCompile(`data-qp-token="([^"]+)"`)

func renderQueryPickerHTML(f Field, value interface{}, e *FormEngine, rc RenderContext) string {
	return mi.RenderToString(renderQueryPicker(context.Background(), e, f, value, false, rc))
}

func TestQueryPicker_RendersInertMarkupWithASealedToken(t *testing.T) {
	asset := fieldNamed(t, queryPickerForm(t), "asset_id")
	asset.Required = true
	out := renderQueryPickerHTML(asset, float64(7), &FormEngine{}, RenderContext{})
	for _, want := range []string{
		`data-seam-widget="query-picker"`, `data-qp-layout="rows"`, `data-qp-min="2"`, `data-qp-detail="1"`,
		`type="hidden"`, `name="asset_id"`, `id="field-asset_id"`, `value="7"`,
		`type="text"`, `role="combobox"`, `aria-expanded="false"`, `required`,
		`data-qp-browse="1"`, `data-qp-detail-line="1"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s:\n%s", want, out)
		}
	}
	for _, leak := range []string{"SELECT", "FROM assets", "LIKE", "ORDER BY", "<script"} {
		if strings.Contains(out, leak) {
			t.Errorf("the markup contains %q:\n%s", leak, out)
		}
	}
	m := qpTokenRE.FindStringSubmatch(out)
	if m == nil {
		t.Fatal("no token in the markup")
	}
	spec, err := querypicker.Open(html.UnescapeString(m[1]))
	if err != nil {
		t.Fatalf("the token does not open: %v", err)
	}
	if spec.Query != asset.QueryPicker.Query || spec.Limit != 5 || !spec.ValueNumeric {
		t.Errorf("the token seals %+v", spec)
	}
}

func TestQueryPicker_NoBrowseButtonWithoutSelector(t *testing.T) {
	code := fieldNamed(t, queryPickerForm(t), "code")
	out := renderQueryPickerHTML(code, "", &FormEngine{}, RenderContext{})
	if strings.Contains(out, "data-qp-browse") {
		t.Error("a Browse button without x-seam-selector")
	}
	if !strings.Contains(out, `data-qp-layout="auto"`) || strings.Contains(out, "data-qp-detail=") || strings.Contains(out, " required") {
		t.Errorf("defaults are wrong:\n%s", out)
	}
}

func TestQueryPicker_HeadingsComeFromTheFormKeyThenTheSharedKeyThenTheTitleThenTheName(t *testing.T) {
	asset := fieldNamed(t, queryPickerForm(t), "asset_id")
	tr := fakeTranslator{"es": {
		"work.forms.order.field.asset_id.column.name": "Nombre del activo",
		"query_picker.column.name":                    "Nombre",
		"query_picker.column.status":                  "Estado",
	}}
	out := html.UnescapeString(renderQueryPickerHTML(asset, nil, &FormEngine{Translator: tr}, RenderContext{Locale: "es", Module: "work", Form: "order"}))
	for _, want := range []string{`"t":"Nombre del activo"`, `"t":"Tag literal"`, `"t":"Estado"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing heading %s:\n%s", want, out)
		}
	}
	// the headings are sealed with the field, so the Browse dialog shows the same ones
	spec, err := querypicker.Open(qpTokenRE.FindStringSubmatch(out)[1])
	if err != nil {
		t.Fatal(err)
	}
	if spec.Columns[0].Title != "Nombre del activo" || spec.Columns[2].Title != "Estado" {
		t.Errorf("sealed headings = %q, %q", spec.Columns[0].Title, spec.Columns[2].Title)
	}
	if asset.QueryPicker.Columns[0].Title != "" {
		t.Error("rendering changed the parsed field's own columns")
	}
	// no translation at all: the literal title, then the column's name
	plain := html.UnescapeString(renderQueryPickerHTML(asset, nil, &FormEngine{}, RenderContext{}))
	for _, want := range []string{`"t":"name"`, `"t":"Tag literal"`, `"t":"status"`} {
		if !strings.Contains(plain, want) {
			t.Errorf("missing fallback heading %s", want)
		}
	}
}

func TestQueryPicker_ChromeTextsAreTranslated(t *testing.T) {
	asset := fieldNamed(t, queryPickerForm(t), "asset_id")
	tr := fakeTranslator{"es": {"field.query_picker.no_results": "Sin resultados", "field.any_picker.browse": "Explorar"}}
	out := renderQueryPickerHTML(asset, nil, &FormEngine{Translator: tr}, RenderContext{Locale: "es"})
	if !strings.Contains(out, `data-t-none="Sin resultados"`) || !strings.Contains(out, `data-title="Explorar"`) {
		t.Errorf("translated texts missing:\n%s", out)
	}
}

func TestQueryPicker_AValueIsEscapedAndNeverReachesAScript(t *testing.T) {
	code := fieldNamed(t, queryPickerForm(t), "code")
	out := renderQueryPickerHTML(code, `"><script>alert(1)</script>`, &FormEngine{}, RenderContext{})
	if strings.Contains(out, "<script>alert(1)") {
		t.Errorf("the value was not escaped:\n%s", out)
	}
}

func TestQueryPicker_InARepeatableForcesIndexedNaming(t *testing.T) {
	lines := fieldNamed(t, queryPickerForm(t), "lines")
	if !needsIndexedNaming(lines) {
		t.Error("a repeatable holding a query-picker must use indexed naming")
	}
}

func TestQueryPicker_AFieldWithoutASpecSaysSo(t *testing.T) {
	out := renderQueryPickerHTML(Field{Name: "x", XSeamInput: "query-picker"}, nil, &FormEngine{}, RenderContext{})
	if !strings.Contains(out, "has no query") || strings.Contains(out, "data-qp-token") {
		t.Errorf("unexpected markup:\n%s", out)
	}
}

// The manual's examples must be fields the engine accepts, and its list of
// readable entities must be the code's.
func TestQueryPickerManual_ExamplesParseAndTheEntityListIsCurrent(t *testing.T) {
	raw, err := os.ReadFile("../../docs/QUERY_PICKER.md")
	if err != nil {
		t.Fatalf("reading the manual: %v", err)
	}
	doc := string(raw)
	examples := 0
	for _, block := range regexp.MustCompile("(?s)```json\n(.*?)```").FindAllStringSubmatch(doc, -1) {
		if !strings.Contains(block[1], `"x-seam-input"`) {
			continue
		}
		examples++
		form, err := ParseSchema([]byte(`{"type":"object","properties":{` + block[1] + `}}`))
		if err != nil {
			t.Errorf("an example in the manual does not parse: %v\n%s", err, block[1])
			continue
		}
		if len(QueryPickerFields(form)) != 1 {
			t.Errorf("an example in the manual is not a query-picker field:\n%s", block[1])
		}
	}
	if examples < 4 {
		t.Errorf("found %d examples in the manual, want at least 4", examples)
	}
	m := regexp.MustCompile(`(?s)\*\*Entities a picker may read\*\*\n\n(.*?)\n\n`).FindStringSubmatch(doc)
	if m == nil {
		t.Fatal("the manual has no entity list")
	}
	listed := regexp.MustCompile("`([a-z_]+)`").FindAllStringSubmatch(m[1], -1)
	var names []string
	for _, l := range listed {
		names = append(names, l[1])
	}
	if got, want := strings.Join(names, ","), strings.Join(querypicker.AllowedTables(), ","); got != want {
		t.Errorf("the manual lists\n %s\nthe code allows\n %s", got, want)
	}
}
