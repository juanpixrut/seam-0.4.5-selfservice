package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/ha1tch/seam-ui/internal/auth"
	fe "github.com/ha1tch/seam-ui/internal/formengine"
	"github.com/ha1tch/seam-ui/internal/querypicker"
)

// T-111: the server side of the query-picker widget. The fake xolu records
// every OQL query it is sent and answers by table, so the tests can assert
// both what was asked and how often.

type qpXolu struct {
	sync.Mutex
	queries []string
	answer  func(query string) []map[string]any
}

func (x *qpXolu) sent() []string {
	x.Lock()
	defer x.Unlock()
	return append([]string(nil), x.queries...)
}

func newQueryPickerHandler(t *testing.T, answer func(query string) []map[string]any) (*UIHandler, *qpXolu) {
	t.Helper()
	resetQueryPickerCache()
	x := &qpXolu{answer: answer}
	server := mockOLU(t, map[string]http.HandlerFunc{
		"POST /api/v1/oql/query": func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Query string `json:"query"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			if strings.Contains(body.Query, "tenant_settings") { // NewUIHandler reads its settings; not the picker's
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]any{"status": "completed", "data": []map[string]any{}})
				return
			}
			x.Lock()
			x.queries = append(x.queries, body.Query)
			x.Unlock()
			rows := x.answer(body.Query)
			if rows == nil {
				rows = []map[string]any{}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"status": "completed", "data": rows})
		},
	})
	t.Cleanup(server.Close)
	return newHandler(t, server), x
}

func qpToken(t *testing.T, s *querypicker.Spec) string {
	t.Helper()
	token, err := querypicker.Seal(s)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	return token
}

func qpAssetSpec() *querypicker.Spec {
	return &querypicker.Spec{
		Query:        "SELECT id, name, type_id, status FROM assets WHERE ((name LIKE :q)) ORDER BY name ASC",
		ValueNumeric: true,
		Limit:        2,
		Columns: []querypicker.Column{
			{Field: "name", Key: true, Title: "Name"},
			{Field: "type_id", Lookup: "asset_types.name", Title: "Type"},
			{Field: "status", Type: "badge", Variants: map[string]string{"active": "success"}, Title: "Status"},
		},
	}
}

func qpAssets(n int) []map[string]any {
	names := []string{"Bulldozer", "Chiller", "Excavator", "Generator"}
	out := []map[string]any{}
	for i := 0; i < n && i < len(names); i++ {
		out = append(out, map[string]any{"id": float64(i + 1), "name": names[i], "type_id": float64(1 + i%2), "status": "active"})
	}
	return out
}

func qpAnswer(assets int) func(string) []map[string]any {
	return func(q string) []map[string]any {
		switch {
		case strings.Contains(q, "FROM asset_types"):
			return []map[string]any{{"id": float64(1), "name": "Heavy"}, {"id": float64(2), "name": "Climate"}}
		case strings.Contains(q, "AND (id = 3)"):
			return []map[string]any{{"id": float64(3), "name": "Excavator", "type_id": float64(1), "status": "active"}}
		case strings.Contains(q, "AND (id = "):
			return nil
		}
		return qpAssets(assets)
	}
}

func qpGet(h *UIHandler, handler func(http.ResponseWriter, *http.Request), path string, v url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", path+"?"+v.Encode(), nil)
	req = req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{UserID: 42, TenantID: 5, Role: "operator"}))
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func qpDecode(t *testing.T, rec *httptest.ResponseRecorder) queryPickerResponse {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var res queryPickerResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return res
}

func TestPickerQuery_SearchAsksForOneRowMoreThanItShows(t *testing.T) {
	h, x := newQueryPickerHandler(t, qpAnswer(3))
	res := qpDecode(t, qpGet(h, h.PickerQuery, "/picker/query", url.Values{"s": {qpToken(t, qpAssetSpec())}, "q": {"o'r"}}))

	if len(res.Rows) != 2 || !res.More {
		t.Fatalf("rows = %d, more = %v; want 2 rows and more", len(res.Rows), res.More)
	}
	first := res.Rows[0]
	if first.Value != "1" || first.Label != "Bulldozer" || first.Cells[1].V != "Heavy" || first.Cells[2].B != "success" {
		t.Errorf("first row = %+v", first)
	}
	sent := x.sent()
	want := "SELECT TOP 3 id, name, type_id, status FROM assets WHERE ((name LIKE '%o''r%')) ORDER BY name ASC"
	if len(sent) != 2 || sent[0] != want {
		t.Fatalf("queries sent = %q\nwant first %q", sent, want)
	}
	if sent[1] != "SELECT TOP 2001 id, name FROM asset_types" {
		t.Errorf("lookup query = %q", sent[1])
	}
}

func TestPickerQuery_FewerRowsThanTheLimitMeansNoMore(t *testing.T) {
	h, _ := newQueryPickerHandler(t, qpAnswer(2))
	res := qpDecode(t, qpGet(h, h.PickerQuery, "/picker/query", url.Values{"s": {qpToken(t, qpAssetSpec())}, "q": {"ab"}}))
	if len(res.Rows) != 2 || res.More {
		t.Errorf("rows = %d, more = %v", len(res.Rows), res.More)
	}
}

func TestPickerQuery_TextShorterThanTheMinimumAsksNothing(t *testing.T) {
	h, x := newQueryPickerHandler(t, qpAnswer(3))
	for _, q := range []string{"", "a", "  a  "} {
		res := qpDecode(t, qpGet(h, h.PickerQuery, "/picker/query", url.Values{"s": {qpToken(t, qpAssetSpec())}, "q": {q}}))
		if len(res.Rows) != 0 {
			t.Errorf("q=%q returned rows", q)
		}
	}
	if n := len(x.sent()); n != 0 {
		t.Errorf("%d queries were sent for text below the minimum", n)
	}
}

func TestPickerQuery_AnIdenticalSearchIsAnsweredFromMemory(t *testing.T) {
	h, x := newQueryPickerHandler(t, qpAnswer(3))
	v := url.Values{"s": {qpToken(t, qpAssetSpec())}, "q": {"ex"}}
	a := qpGet(h, h.PickerQuery, "/picker/query", v).Body.String()
	b := qpGet(h, h.PickerQuery, "/picker/query", v).Body.String()
	if a != b || len(x.sent()) != 2 {
		t.Errorf("the repeated search sent queries again: %q", x.sent())
	}
	// a different text is a different search; the lookup table is still held
	qpGet(h, h.PickerQuery, "/picker/query", url.Values{"s": v["s"], "q": {"ge"}})
	sent := x.sent()
	if len(sent) != 3 || strings.Contains(sent[2], "asset_types") {
		t.Errorf("after a new text: %q", sent)
	}
}

func TestPickerQuery_ATokenSealedAgainIsTheSameField(t *testing.T) {
	h, x := newQueryPickerHandler(t, qpAnswer(3))
	qpGet(h, h.PickerQuery, "/picker/query", url.Values{"s": {qpToken(t, qpAssetSpec())}, "q": {"ex"}})
	qpGet(h, h.PickerQuery, "/picker/query", url.Values{"s": {qpToken(t, qpAssetSpec())}, "q": {"ex"}})
	if n := len(x.sent()); n != 2 {
		t.Errorf("two tokens of one field were cached apart: %d queries", n)
	}
}

func TestPickerQuery_ValueLookup(t *testing.T) {
	h, x := newQueryPickerHandler(t, qpAnswer(3))
	token := qpToken(t, qpAssetSpec())
	res := qpDecode(t, qpGet(h, h.PickerQuery, "/picker/query", url.Values{"s": {token}, "value": {"3"}}))
	if len(res.Rows) != 1 || res.Rows[0].Value != "3" || res.Rows[0].Label != "Excavator" || res.Rows[0].Cells[1].V != "Heavy" {
		t.Fatalf("rows = %+v", res.Rows)
	}
	if got := x.sent()[0]; got != "SELECT TOP 1 id, name, type_id, status FROM assets WHERE (((name LIKE '%%'))) AND (id = 3) ORDER BY name ASC" {
		t.Errorf("lookup query = %q", got)
	}
	if res := qpDecode(t, qpGet(h, h.PickerQuery, "/picker/query", url.Values{"s": {token}, "value": {"99"}})); len(res.Rows) != 0 {
		t.Errorf("an unknown value returned %+v", res.Rows)
	}
	before := len(x.sent())
	if res := qpDecode(t, qpGet(h, h.PickerQuery, "/picker/query", url.Values{"s": {token}, "value": {"3 OR 1=1"}})); len(res.Rows) != 0 {
		t.Errorf("a non-number returned %+v", res.Rows)
	}
	if len(x.sent()) != before {
		t.Error("a non-number value reached xolu")
	}
}

func TestPickerQuery_RefusesWhatItDidNotSeal(t *testing.T) {
	h, x := newQueryPickerHandler(t, qpAnswer(3))
	token := qpToken(t, qpAssetSpec())
	for name, bad := range map[string]string{"empty": "", "junk": "SELECT id FROM users", "cut": token[:len(token)-6]} {
		rec := qpGet(h, h.PickerQuery, "/picker/query", url.Values{"s": {bad}, "q": {"ab"}})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s token: status %d", name, rec.Code)
		}
		if rec := qpGet(h, h.PickerQueryBrowse, "/picker/query/browse", url.Values{"s": {bad}}); rec.Code != http.StatusBadRequest {
			t.Errorf("%s token on browse: status %d", name, rec.Code)
		}
	}
	if n := len(x.sent()); n != 0 {
		t.Errorf("%d queries were sent for refused tokens", n)
	}
}

func TestPickerQuery_ParametersComeFromTheSession(t *testing.T) {
	h, x := newQueryPickerHandler(t, qpAnswer(1))
	spec := &querypicker.Spec{
		Query:   "SELECT id, name FROM issues WHERE tenant_id = :tenant_id AND owner = :user_id AND ((name LIKE :q)) ORDER BY name",
		Columns: []querypicker.Column{{Field: "name"}},
	}
	qpGet(h, h.PickerQuery, "/picker/query", url.Values{"s": {qpToken(t, spec)}, "q": {"ab"}, "tenant_id": {"99"}, "user_id": {"1"}})
	if got := x.sent()[0]; !strings.Contains(got, "tenant_id = 5 AND owner = 42") {
		t.Errorf("query = %q", got)
	}
}

func TestPickerQueryBrowse_ListsAPageWithTheSealedHeadings(t *testing.T) {
	h, x := newQueryPickerHandler(t, func(q string) []map[string]any {
		if strings.Contains(q, "asset_types") {
			return []map[string]any{{"id": float64(1), "name": "Heavy"}}
		}
		rows := []map[string]any{}
		for i := 1; i <= 21; i++ {
			rows = append(rows, map[string]any{"id": float64(i), "name": "Asset " + string(rune('A'+i-1)), "type_id": float64(1), "status": "active"})
		}
		return rows
	})
	token := qpToken(t, qpAssetSpec())
	rec := qpGet(h, h.PickerQueryBrowse, "/picker/query/browse", url.Values{"s": {token}, "page": {"2"}, "target": {"seam-qp-3"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if got := x.sent()[0]; !strings.HasPrefix(got, "SELECT TOP 21 id, name") {
		t.Errorf("page 2 asked %q, want TOP 21 (two pages plus one)", got)
	}
	if n := strings.Count(body, `data-id="`); n != 10 {
		t.Errorf("page 2 lists %d rows, want 10", n)
	}
	for _, want := range []string{`data-id="11"`, `data-label="Asset K"`, ">Name<", ">Type<", ">Status<", "Heavy", "/picker/query/browse?", "page=3", "target=seam-qp-3"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(body, `data-id="1"`) || strings.Contains(body, "/selector/query") {
		t.Error("page 2 shows a row of page 1, or links to /selector/")
	}
	if strings.Contains(body, "SELECT") || strings.Contains(body, "FROM assets") {
		t.Error("the dialog shows the query")
	}
}

func TestQueryPickerErrors_AcceptsRowsOfTheQueryAndReportsTheRest(t *testing.T) {
	h, _ := newQueryPickerHandler(t, qpAnswer(3))
	form, err := fe.ParseSchema([]byte(`{"type":"object","properties":{
		"asset_id":{"type":"integer","x-seam-input":"query-picker","x-seam-query":"SELECT id, name, type_id, status FROM assets WHERE ((name LIKE :q)) ORDER BY name ASC","x-seam-columns":[{"field":"name"}]},
		"other_id":{"type":"integer","x-seam-input":"query-picker","x-seam-query":"SELECT id, name, type_id, status FROM assets WHERE ((name LIKE :q)) ORDER BY name ASC","x-seam-columns":[{"field":"name"}]},
		"note":{"type":"string"},
		"lines":{"type":"array","x-seam-input":"repeatable","items":{"type":"object","properties":{
			"asset_id":{"type":"integer","x-seam-input":"query-picker","x-seam-query":"SELECT id, name, type_id, status FROM assets WHERE ((name LIKE :q)) ORDER BY name ASC","x-seam-columns":[{"field":"name"}]}}}}
	}}`))
	if err != nil {
		t.Fatal(err)
	}
	ctx := auth.ContextWithClaims(httptest.NewRequest("GET", "/", nil).Context(), &auth.Claims{UserID: 42, TenantID: 5})
	tr := func(key string, args ...any) string { return "T:" + key }

	ok := map[string]interface{}{"asset_id": "3", "other_id": "", "note": "x",
		"lines": []map[string]interface{}{{"asset_id": "3"}, {"asset_id": ""}}}
	if errs := h.QueryPickerErrors(ctx, form, ok, tr); errs != nil {
		t.Errorf("acceptable values were reported: %v", errs)
	}
	bad := map[string]interface{}{"asset_id": "99", "other_id": "3 OR 1=1",
		"lines": []interface{}{map[string]interface{}{"asset_id": "3"}, map[string]interface{}{"asset_id": "98"}}}
	errs := h.QueryPickerErrors(ctx, form, bad, tr)
	if len(errs) != 3 || errs["asset_id"] != "T:field.query_picker.invalid" || errs["other_id"] == "" || errs["lines[1].asset_id"] == "" {
		t.Errorf("errors = %v", errs)
	}
	if h.QueryPickerErrors(ctx, nil, bad, tr) != nil {
		t.Error("a nil form reported errors")
	}
}
