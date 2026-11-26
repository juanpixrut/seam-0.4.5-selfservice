package ui

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	fe "github.com/ha1tch/seam-ui/internal/formengine"
	demoschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/demo"
)

func TestXoluSelectorQuery(t *testing.T) {
	s := xoluSelectorSpec{Table: "assets", SearchColumns: []string{"name", "tag_tecnico"}, OrderColumn: "name"}
	for _, c := range []struct {
		q          string
		page, size int
		want       string
	}{
		{"", 1, 10, "SELECT TOP 11 * FROM assets ORDER BY name"},
		{"", 0, 10, "SELECT TOP 11 * FROM assets ORDER BY name"},
		{"pump", 3, 10, "SELECT TOP 31 * FROM assets WHERE (name LIKE '%pump%' OR tag_tecnico LIKE '%pump%') ORDER BY name"},
		{"o'brien", 1, 10, "SELECT TOP 11 * FROM assets WHERE (name LIKE '%o''brien%' OR tag_tecnico LIKE '%o''brien%') ORDER BY name"},
		{"42", 1, 10, "SELECT TOP 11 * FROM assets WHERE (name LIKE '%42%' OR tag_tecnico LIKE '%42%' OR id = 42) ORDER BY name"},
	} {
		if got := s.query(c.q, c.page, c.size); got != c.want {
			t.Errorf("query(%q, %d, %d)\n got  %s\n want %s", c.q, c.page, c.size, got, c.want)
		}
	}
}

func selectorTestRows(n int) []map[string]any {
	rows := make([]map[string]any, n)
	for i := range rows {
		rows[i] = map[string]any{"id": float64(i)}
	}
	return rows
}

func TestSelectorPage(t *testing.T) {
	for _, c := range []struct {
		n, page, size int
		wantLen       int
		wantFirst     int
		wantMore      bool
	}{
		{11, 1, 10, 10, 0, true},
		{10, 1, 10, 10, 0, false},
		{21, 2, 10, 10, 10, true},
		{15, 2, 10, 5, 10, false},
		{3, 5, 10, 0, 0, false},
		{0, 1, 10, 0, 0, false},
		{5, 0, 10, 5, 0, false},
	} {
		rows, more := selectorPage(selectorTestRows(c.n), c.page, c.size)
		if len(rows) != c.wantLen || more != c.wantMore {
			t.Errorf("n=%d page=%d: got %d rows more=%v, want %d more=%v", c.n, c.page, len(rows), more, c.wantLen, c.wantMore)
			continue
		}
		if c.wantLen > 0 && int(rows[0]["id"].(float64)) != c.wantFirst {
			t.Errorf("n=%d page=%d: first id %v, want %d", c.n, c.page, rows[0]["id"], c.wantFirst)
		}
	}
}

func TestSelectorSources_RegisteredAndPickable(t *testing.T) {
	for _, key := range []string{"users", "assets", "cost_centers", "locations", "custodians"} {
		src, ok := selectorSources[key]
		if !ok {
			t.Errorf("selector %q is not registered", key)
			continue
		}
		if src.Key != key || src.TitleKey == "" || src.Fetch == nil || len(src.Columns) == 0 {
			t.Errorf("selector %q is incomplete: %+v", key, src)
		}
		if _, ok := pickerSearchConfigs["entity:"+key]; !ok {
			t.Errorf("no picker search config for entity:%s", key)
		}
	}
}

func TestEntityLabels(t *testing.T) {
	if got := costCenterLabel(map[string]any{"accounting_code": "OP-001", "name": "Operaciones"}); got != "OP-001 — Operaciones" {
		t.Errorf("costCenterLabel = %q", got)
	}
	if got := locationLabel(map[string]any{"short_name": "HQ-MVD", "description": "Sede central"}); got != "HQ-MVD — Sede central" {
		t.Errorf("locationLabel = %q", got)
	}
	if got := locationLabel(map[string]any{"short_name": "HQ-MVD"}); got != "HQ-MVD" {
		t.Errorf("locationLabel without description = %q", got)
	}
	if got := custodianLabel(map[string]any{"legal_name": "Constructec S.A.", "alias": "Constructec"}); got != "Constructec S.A. (Constructec)" {
		t.Errorf("custodianLabel = %q", got)
	}
	if got := custodianLabel(map[string]any{"legal_name": "Same", "alias": "Same"}); got != "Same" {
		t.Errorf("custodianLabel with equal alias = %q", got)
	}
}

// Every key the selectors and the demo tabs use exists in all four locales.
func TestSelectorAndDemoKeys_ExistInEveryLocale(t *testing.T) {
	var keys []string
	for _, src := range selectorSources {
		keys = append(keys, src.TitleKey, src.EmptyTitleKey, src.EmptyMessageKey)
		for _, c := range src.Columns {
			keys = append(keys, c.LabelKey)
		}
	}
	keys = append(keys, "demo.schema.heading")
	for _, n := range demoAllTabs() {
		if n == "components" || n == "repeatable" {
			continue // panels of their own, with their own texts
		}
		keys = append(keys, "demo.tab."+n, "demo."+n+".heading", "demo."+n+".intro", "demo."+n+".notes")
	}
	for _, lang := range []string{"en", "es", "pt", "ja"} {
		raw, err := os.ReadFile("../i18n/locales/" + lang + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]string
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("%s.json: %v", lang, err)
		}
		for _, k := range keys {
			if m[k] == "" {
				t.Errorf("%s.json: missing or empty key %q", lang, k)
			}
		}
	}
}

func walkDemoFields(fields []fe.Field, visit func(fe.Field)) {
	for _, f := range fields {
		visit(f)
		walkDemoFields(f.Items, visit)
	}
}

// Every any-picker in the demo schemas points at a registered source and
// selector, and every icon the repeatables name is a real Material icon.
func TestDemoSchemas_PickersAndIconsAreValid(t *testing.T) {
	raw, err := os.ReadFile("../../web/static/vendor/material-icons-ligatures.txt")
	if err != nil {
		t.Fatal(err)
	}
	icons := map[string]bool{}
	for _, l := range strings.Fields(string(raw)) {
		icons[l] = true
	}
	for _, name := range demoAllTabs() {
		form, err := fe.ParseSchema(demoschemas.Schema(name))
		if err != nil {
			t.Fatalf("schema %s: %v", name, err)
		}
		walkDemoFields(form.Fields, func(f fe.Field) {
			if f.XSeamInput == "any-picker" {
				if _, ok := pickerSearchConfigs[f.XSeamSource]; !ok {
					t.Errorf("%s.%s: no picker config for %q", name, f.Name, f.XSeamSource)
				}
				if f.XSeamSelector != "" {
					if _, ok := selectorSources[f.XSeamSelector]; !ok {
						t.Errorf("%s.%s: unknown selector %q", name, f.Name, f.XSeamSelector)
					}
				}
			}
			for _, ic := range []string{f.XSeamAddIcon, f.XSeamRemoveIcon} {
				if ic != "" && !icons[ic] {
					t.Errorf("%s.%s: unknown Material icon %q", name, f.Name, ic)
				}
			}
		})
	}
}
