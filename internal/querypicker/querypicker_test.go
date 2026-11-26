package querypicker

import (
	"strings"
	"testing"
)

func cols(names ...string) []Column {
	out := make([]Column, len(names))
	for i, n := range names {
		out[i] = Column{Field: n}
	}
	return out
}

const goodQuery = "SELECT id, name, tag_tecnico, status FROM assets WHERE status = 'active' AND ((name LIKE :q) OR (tag_tecnico LIKE :q)) ORDER BY name ASC"

func TestValidate_AcceptsTheManualExamples(t *testing.T) {
	for _, tc := range []struct {
		q    string
		cols []Column
	}{
		{goodQuery, cols("name", "tag_tecnico", "status")},
		{"SELECT id, name, brand_id FROM models WHERE (name LIKE :q) ORDER BY name ASC",
			[]Column{{Field: "name", Key: true}, {Field: "brand_id", Lookup: "brands.name"}}},
		{"SELECT a.id, a.name, t.name AS type_name FROM assets a JOIN asset_types t ON a.type_id = t.id WHERE ((a.name LIKE :q) OR (t.name LIKE :q)) ORDER BY a.name ASC",
			cols("name", "type_name")},
		{"select id, name from brands order by name", cols("name")},
	} {
		s := &Spec{Query: tc.q, Columns: tc.cols}
		if err := s.Validate(); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", tc.q, err)
		}
	}
}

func TestValidate_RefusesEachBrokenRule(t *testing.T) {
	for _, tc := range []struct{ name, q, want string }{
		{"star", "SELECT * FROM assets ORDER BY name", "name the columns"},
		{"qualified star", "SELECT a.* FROM assets a ORDER BY a.name", "name the columns"},
		{"bare like then or", "SELECT id, name FROM assets WHERE name LIKE :q OR tag_tecnico LIKE :q ORDER BY name", "every LIKE needs its own parentheses"},
		{"like inside a shared group", "SELECT id, name FROM assets WHERE (name LIKE :q OR tag_tecnico LIKE :q) ORDER BY name", "every LIKE needs its own parentheses"},
		{"bare like alone", "SELECT id, name FROM assets WHERE name LIKE :q ORDER BY name", "every LIKE needs its own parentheses"},
		{"like then and", "SELECT id, name FROM assets WHERE name LIKE :q AND status = 'active' ORDER BY name", "every LIKE needs its own parentheses"},
		{"top", "SELECT TOP 5 id, name FROM assets ORDER BY name", "do not write TOP"},
		{"limit", "SELECT id, name FROM assets ORDER BY name LIMIT 5", "do not write LIMIT"},
		{"no order by", "SELECT id, name FROM assets WHERE (name LIKE :q)", "needs an ORDER BY"},
		{"two joins", "SELECT a.id, a.name FROM assets a JOIN asset_types t ON a.type_id = t.id JOIN brands b ON a.brand_id = b.id ORDER BY a.name", "at most one JOIN"},
		{"users", "SELECT id, name FROM users ORDER BY name", `entity "users" is not one`},
		{"joined users", "SELECT a.id, a.name FROM assets a JOIN users u ON a.custodian_id = u.id ORDER BY a.name", `entity "users" is not one`},
		{"comma join", "SELECT a.id, a.name FROM assets a, users u ORDER BY a.name", "one statement"},
		{"update", "UPDATE assets SET name = 'x'", "must start with SELECT"},
		{"delete inside", "SELECT id, name FROM assets WHERE id IN (DELETE FROM assets) ORDER BY name", "may only read"},
		{"semicolon", "SELECT id, name FROM assets ORDER BY name; DROP TABLE assets", "one statement"},
		{"comment", "SELECT id, name FROM assets -- x\n ORDER BY name", "one statement"},
		{"subquery", "SELECT id, name FROM assets WHERE id IN (SELECT asset_id FROM alerts) ORDER BY name", "single SELECT"},
		{"union", "SELECT id, name FROM assets UNION SELECT id, name FROM brands ORDER BY name", "single SELECT"},
		{"group by", "SELECT status, name FROM assets GROUP BY status ORDER BY name", "GROUP BY"},
		{"unknown parameter", "SELECT id, name FROM assets WHERE (name LIKE :term) ORDER BY name", "unknown parameter :term"},
		{"expression without a name", "SELECT id, UPPER(name) FROM assets ORDER BY name", `needs "AS <name>"`},
		{"same name twice", "SELECT a.id, a.name, t.name FROM assets a JOIN asset_types t ON a.type_id = t.id ORDER BY a.name", "two columns named"},
		{"open quote", "SELECT id, name FROM assets WHERE status = 'active ORDER BY name", "one statement"},
		{"bracketed", "SELECT id, name FROM [users] ORDER BY name", "one statement"},
		{"too long", "SELECT id, name FROM assets WHERE status = '" + strings.Repeat("x", MaxQueryChars) + "' ORDER BY name", "longer than"},
	} {
		s := &Spec{Query: tc.q, Columns: cols("name")}
		err := s.Validate()
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: Validate = %v, want an error containing %q", tc.name, err, tc.want)
		}
	}
}

func TestValidate_TextInsideAStringIsNotCode(t *testing.T) {
	s := &Spec{Query: "SELECT id, name FROM assets WHERE status = 'x LIKE y OR ; -- DROP TOP *' ORDER BY name", Columns: cols("name")}
	if err := s.Validate(); err != nil {
		t.Errorf("Validate = %v, want nil: the keywords are inside a string literal", err)
	}
}

func TestValidate_ColumnAndFieldChecks(t *testing.T) {
	for _, tc := range []struct {
		name string
		s    Spec
		want string
	}{
		{"column the query does not return", Spec{Query: goodQuery, Columns: cols("serial_1")}, `does not return a column named "serial_1"`},
		{"unknown type", Spec{Query: goodQuery, Columns: []Column{{Field: "name", Type: "chart"}}}, "unknown type"},
		{"two keys", Spec{Query: goodQuery, Columns: []Column{{Field: "name", Key: true}, {Field: "status", Key: true}}}, "only one column"},
		{"bad lookup", Spec{Query: goodQuery, Columns: []Column{{Field: "name", Lookup: "brands"}}}, "lookup must be"},
		{"lookup into users", Spec{Query: goodQuery, Columns: []Column{{Field: "name", Lookup: "users.email"}}}, "is not one a query-picker may read"},
		{"value field", Spec{Query: goodQuery, Columns: cols("name"), ValueField: "code"}, "x-seam-value-field"},
		{"label field", Spec{Query: goodQuery, Columns: cols("name"), LabelFields: []string{"nope"}}, "x-seam-label-fields"},
		{"limit", Spec{Query: goodQuery, Columns: cols("name"), Limit: 500}, "x-seam-limit"},
		{"no columns", Spec{Query: goodQuery}, "at least one column"},
		{"badge variant", Spec{Query: goodQuery, Columns: []Column{{Field: "status", Type: "badge", Variants: map[string]string{"active": "green"}}}}, "unknown badge variant"},
	} {
		s := tc.s
		err := s.Validate()
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: Validate = %v, want an error containing %q", tc.name, err, tc.want)
		}
	}
}

func TestValidate_FillsDefaults(t *testing.T) {
	s := &Spec{Query: goodQuery, Columns: []Column{{Field: "tag_tecnico"}, {Field: "name", Key: true}}}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	if s.ValueField != "id" || s.Limit != DefaultLimit || len(s.LabelFields) != 1 || s.LabelFields[0] != "name" || s.Columns[0].Type != "text" {
		t.Errorf("defaults not filled: %+v", s)
	}
}

func TestSearchQuery_AddsTopAndBindsEscapedText(t *testing.T) {
	s := &Spec{Query: goodQuery, Columns: cols("name")}
	got, err := s.SearchQuery(Params{Q: "O'Brien"}, 9)
	if err != nil {
		t.Fatal(err)
	}
	want := "SELECT TOP 9 id, name, tag_tecnico, status FROM assets WHERE status = 'active' AND ((name LIKE '%O''Brien%') OR (tag_tecnico LIKE '%O''Brien%')) ORDER BY name ASC"
	if got != want {
		t.Errorf("SearchQuery =\n %s\nwant\n %s", got, want)
	}
}

func TestSearchQuery_AnInjectionAttemptStaysInsideTheLiteral(t *testing.T) {
	s := &Spec{Query: goodQuery, Columns: cols("name")}
	got, err := s.SearchQuery(Params{Q: "x') OR (1=1) --"}, 9)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "LIKE '%x'') OR (1=1) --%')") {
		t.Errorf("the search text escaped its literal: %s", got)
	}
}

func TestSearchQuery_ParametersInsideAStringAreLeftAlone(t *testing.T) {
	s := &Spec{Query: "SELECT id, name FROM assets WHERE status = ':q' AND tenant_ref = :tenant_id AND owner = :user_id AND code = :q_raw ORDER BY name", Columns: cols("name")}
	got, err := s.SearchQuery(Params{Q: "ab", TenantID: 7, UserID: 42}, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := "SELECT TOP 3 id, name FROM assets WHERE status = ':q' AND tenant_ref = 7 AND owner = 42 AND code = 'ab' ORDER BY name"
	if got != want {
		t.Errorf("SearchQuery =\n %s\nwant\n %s", got, want)
	}
}

func TestLookupQuery(t *testing.T) {
	for _, tc := range []struct {
		name  string
		s     Spec
		value string
		want  string
	}{
		{"with a where, numeric", Spec{Query: goodQuery, Columns: cols("name"), ValueNumeric: true}, "12",
			"SELECT TOP 1 id, name, tag_tecnico, status FROM assets WHERE (status = 'active' AND ((name LIKE '%%') OR (tag_tecnico LIKE '%%'))) AND (id = 12) ORDER BY name ASC"},
		{"without a where", Spec{Query: "SELECT id, name FROM brands ORDER BY name", Columns: cols("name"), ValueNumeric: true}, "3",
			"SELECT TOP 1 id, name FROM brands WHERE (id = 3) ORDER BY name"},
		{"text value, qualified column", Spec{Query: "SELECT a.tag_tecnico AS tag, a.name FROM assets a WHERE (a.name LIKE :q) ORDER BY a.name", Columns: cols("name"), ValueField: "tag"}, "X'1",
			"SELECT TOP 1 a.tag_tecnico AS tag, a.name FROM assets a WHERE ((a.name LIKE '%%')) AND (a.tag_tecnico = 'X''1') ORDER BY a.name"},
	} {
		s := tc.s
		got, err := s.LookupQuery(Params{Q: "ignored"}, tc.value)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: LookupQuery =\n %s\nwant\n %s", tc.name, got, tc.want)
		}
	}
}

func TestLookupQuery_RefusesANonNumberForANumericField(t *testing.T) {
	s := &Spec{Query: goodQuery, Columns: cols("name"), ValueNumeric: true}
	if _, err := s.LookupQuery(Params{}, "1 OR 1=1"); err == nil {
		t.Error("LookupQuery accepted a non-number for a numeric value field")
	}
}

func TestSealOpen_RoundTripAndTampering(t *testing.T) {
	s := &Spec{Query: goodQuery, Columns: []Column{{Field: "name", Key: true}, {Field: "status", Type: "badge", Variants: map[string]string{"active": "success"}}}, ValueNumeric: true, Limit: 5}
	token, err := Seal(s)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(token, "assets") || strings.Contains(token, "SELECT") {
		t.Error("the token shows the query")
	}
	got, err := Open(token)
	if err != nil {
		t.Fatal(err)
	}
	if got.Query != s.Query || got.Limit != 5 || !got.ValueNumeric || got.Columns[1].Variants["active"] != "success" || got.ValueField != "id" {
		t.Errorf("Open returned %+v", got)
	}
	again, _ := Seal(s)
	if again == token {
		t.Error("two seals of one Spec are identical: the nonce is not random")
	}
	flipped := []byte(token)
	if flipped[len(flipped)/2] == 'A' {
		flipped[len(flipped)/2] = 'B'
	} else {
		flipped[len(flipped)/2] = 'A'
	}
	for name, bad := range map[string]string{"flipped": string(flipped), "empty": "", "junk": "not-a-token", "truncated": token[:len(token)/2]} {
		if _, err := Open(bad); err == nil {
			t.Errorf("Open accepted a %s token", name)
		}
	}
}

func TestSealOpen_AnotherKeyCannotOpen(t *testing.T) {
	s := &Spec{Query: goodQuery, Columns: cols("name")}
	token, err := Seal(s)
	if err != nil {
		t.Fatal(err)
	}
	keyMu.Lock()
	saved := key
	keyMu.Unlock()
	defer func() { keyMu.Lock(); key = saved; keyMu.Unlock() }()
	SetKey([]byte("a deployment secret"))
	if _, err := Open(token); err == nil {
		t.Error("a token sealed under one key opened under another")
	}
	mine, err := Seal(s)
	if err != nil {
		t.Fatal(err)
	}
	SetKey([]byte("a deployment secret")) // the same secret gives the same key
	if _, err := Open(mine); err != nil {
		t.Errorf("a token did not open under the key derived from the same secret: %v", err)
	}
}

func TestRow_FormatsEachType(t *testing.T) {
	s := &Spec{
		Query: "SELECT id, code, description, warehouse_id, stock, unit_cost, cur, last_count, seen_at, active, status, glyph FROM consumable ORDER BY description",
		Columns: []Column{
			{Field: "code", Muted: true},
			{Field: "description", Key: true},
			{Field: "warehouse_id", Lookup: "warehouse.name"},
			{Field: "stock", Type: "number"},
			{Field: "unit_cost", Type: "money", Currency: "cur"},
			{Field: "last_count", Type: "date"},
			{Field: "seen_at", Type: "datetime"},
			{Field: "active", Type: "bool"},
			{Field: "status", Type: "badge", Variants: map[string]string{"low": "warning"}},
			{Field: "glyph", Type: "icon"},
		},
		LabelFields: []string{"code", "description"},
	}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	raw := map[string]any{
		"id": float64(231), "code": "FLT-0231", "description": "Hydraulic filter", "warehouse_id": float64(2),
		"stock": float64(6), "unit_cost": "74.9", "cur": "USD", "last_count": "2026-11-03T10:20:30Z",
		"seen_at": "2026-11-03T10:20:30Z", "active": true, "status": "low", "glyph": "build",
	}
	row := s.Row(raw, func(c Column, id string) (string, bool) {
		if c.Lookup == "warehouse.name" && id == "2" {
			return "Central", true
		}
		return "", false
	})
	want := []Cell{{V: "FLT-0231"}, {V: "Hydraulic filter"}, {V: "Central"}, {V: "6"}, {V: "USD 74.90"}, {V: "2026-11-03"}, {V: "2026-11-03 10:20"}, {V: "true"}, {V: "low", B: "warning"}, {V: "build"}}
	if row.Value != "231" || row.Label != "FLT-0231 · Hydraulic filter" {
		t.Errorf("value %q label %q", row.Value, row.Label)
	}
	for i, w := range want {
		if row.Cells[i] != w {
			t.Errorf("cell %d (%s) = %+v, want %+v", i, s.Columns[i].Field, row.Cells[i], w)
		}
	}
	miss := s.Row(map[string]any{"id": float64(1), "warehouse_id": float64(9), "status": "odd"}, nil)
	if miss.Cells[2].V != "9" || miss.Cells[8].B != "neutral" || miss.Cells[4].V != "" {
		t.Errorf("a lookup miss, an unlisted badge value or an empty amount is wrong: %+v", miss.Cells)
	}
}
