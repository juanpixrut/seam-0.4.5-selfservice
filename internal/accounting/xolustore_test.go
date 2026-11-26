package accounting

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	xolu "github.com/ha1tch/xolu/pkg/client"
)

func TestFindQuery(t *testing.T) {
	got, err := findQuery(Query{Entity: EntItem, Where: []Cond{
		eq("state", "collected"), eq("ae_id", int64(7)), {Field: "date", Op: "<=", Value: "2026-10-31"}, eq("flag", true),
	}, OrderBy: "period", Desc: true, Limit: 50})
	must(t, err)
	want := "SELECT TOP 50 * FROM adm_item WHERE state = 'collected' AND ae_id = 7 AND date <= '2026-10-31' AND flag = true ORDER BY period DESC"
	if got != want {
		t.Errorf("\n got %s\nwant %s", got, want)
	}
	got, _ = findQuery(Query{Entity: EntAE})
	if got != "SELECT TOP 1000 * FROM adm_ae ORDER BY id ASC" {
		t.Errorf("defaults: %s", got)
	}
}

// Values are quoted and escaped, identifiers are validated: nothing the
// module is handed can change the shape of the query.
func TestFindQuery_Injection(t *testing.T) {
	got, err := findQuery(Query{Entity: EntAE, Where: []Cond{eq("name", "x' OR 1=1 --")}})
	must(t, err)
	if !strings.Contains(got, "name = 'x'' OR 1=1 --'") {
		t.Errorf("value not escaped: %s", got)
	}
	for _, q := range []Query{
		{Entity: "adm_ae; DROP TABLE users"},
		{Entity: EntAE, Where: []Cond{eq("a b", 1)}},
		{Entity: EntAE, Where: []Cond{{Field: "a", Op: "; --", Value: 1}}},
		{Entity: EntAE, Where: []Cond{eq("a", 1.5)}},
		{Entity: EntAE, OrderBy: "id; --"},
	} {
		if _, err := findQuery(q); err == nil {
			t.Errorf("must refuse %+v", q)
		}
	}
}

func TestSumQuery(t *testing.T) {
	got, err := sumQuery(EntMovement, "account_code", []string{"debit_cents", "credit_cents"},
		[]Cond{{Field: "date", Op: "<=", Value: "2026-10-31"}})
	must(t, err)
	want := "SELECT account_code, SUM(debit_cents) AS sum_debit_cents, SUM(credit_cents) AS sum_credit_cents FROM adm_movement WHERE date <= '2026-10-31' GROUP BY account_code"
	if got != want {
		t.Errorf("\n got %s\nwant %s", got, want)
	}
	if _, err := sumQuery(EntMovement, "a", []string{"x) FROM users --"}, nil); err == nil {
		t.Error("field names are validated")
	}
}

func TestNumeric(t *testing.T) {
	for in, want := range map[any]int64{float64(12): 12, "34": 34, "56.00": 56, int64(7): 7, nil: 0, "x": 0} {
		if got := numeric(in); got != want {
			t.Errorf("numeric(%v) = %d, want %d", in, got, want)
		}
	}
}

// fake xolu: records the requests and answers canned bodies.
type fakeXolu struct {
	t      *testing.T
	method string
	path   string
	body   map[string]any
	status int
	reply  string
}

func (f *fakeXolu) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.method, f.path = r.Method, r.URL.Path
	b, _ := io.ReadAll(r.Body)
	f.body = nil
	_ = json.Unmarshal(b, &f.body)
	w.Header().Set("Content-Type", "application/json")
	if f.status == 0 {
		f.status = 200
	}
	w.WriteHeader(f.status)
	io.WriteString(w, f.reply)
}

func newFake(t *testing.T) (*XoluStore, *fakeXolu) {
	f := &fakeXolu{t: t}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return NewXoluStore(xolu.New(srv.URL)), f
}

func TestXoluStore_GetHidesIDAndKeepsVersion(t *testing.T) {
	st, f := newFake(t)
	f.reply = `{"id":9,"_version":4,"state":"pending","amount":1000}`
	d, err := st.Get(context.Background(), EntItem, 9)
	must(t, err)
	if d.ID != 9 || d.Version != 4 || d.Data["state"] != "pending" || d.Data["_version"] != nil || d.Data["id"] != nil {
		t.Errorf("%+v", d)
	}
}

func TestXoluStore_ErrorMapping(t *testing.T) {
	st, f := newFake(t)
	ctx := context.Background()
	f.status, f.reply = 404, `{"error":{"code":"XOLU-ST001","message":"not found","status":404}}`
	_, err := st.Get(ctx, EntItem, 1)
	wantErr(t, err, ErrNotFound)

	f.status, f.reply = 409, `{"error":{"code":"XOLU-ST005","message":"version conflict","status":409}}`
	wantErr(t, st.Put(ctx, EntItem, 1, map[string]any{"a": 1}, 3), ErrConflict)
	_, err = st.Commit(ctx, Batch{Update: &Doc{Entity: EntItem, ID: 1, Data: map[string]any{}}, ExpectVersion: 2})
	wantErr(t, err, ErrConflict)

	f.status, f.reply = 409, `{"error":{"code":"XOLU-ST002","message":"entity exists","status":409}}`
	_, err = st.Commit(ctx, Batch{Update: &Doc{Entity: EntHead, ID: 1, Data: map[string]any{}},
		Appends: []Append{{Entity: EntClaim, ID: 5, Data: map[string]any{}}}})
	wantErr(t, err, ErrExists)
}

func TestXoluStore_PutSendsTheExpectedVersion(t *testing.T) {
	st, f := newFake(t)
	f.reply = `{"message":"ok"}`
	must(t, st.Put(context.Background(), EntItem, 3, map[string]any{"state": "x", "id": 3, "_version": 99}, 5))
	if f.method != "PUT" || f.path != "/api/v1/adm_item/3" || f.body["_version"] != float64(5) || f.body["id"] != nil {
		t.Errorf("%s %s %v", f.method, f.path, f.body)
	}
	must(t, st.Put(context.Background(), EntItem, 3, map[string]any{"state": "x"}, 0))
	if _, has := f.body["_version"]; has {
		t.Error("an unconditional put sends no version")
	}
}

func TestXoluStore_CommitShape(t *testing.T) {
	st, f := newFake(t)
	f.reply = `{"update":{"entity":"adm_item","id":1,"version":3},"appended":[{"entity":"adm_entry","id":10},{"entity":"adm_line_claim","id":42}]}`
	ids, err := st.Commit(context.Background(), Batch{
		Update: &Doc{Entity: EntItem, ID: 1, Data: map[string]any{"state": "collected"}}, ExpectVersion: 2,
		Appends: []Append{{Entity: EntEntry, Data: map[string]any{"a": 1}}, {Entity: EntClaim, ID: 42, Data: map[string]any{"b": 2}}},
	})
	must(t, err)
	if len(ids) != 2 || ids[0] != 10 || ids[1] != 42 {
		t.Errorf("ids %v", ids)
	}
	up := f.body["update"].(map[string]any)
	ap := f.body["append"].([]any)
	if f.path != "/api/v1/commit" || up["version"] != float64(2) || len(ap) != 2 ||
		ap[0].(map[string]any)["id"] != nil || ap[1].(map[string]any)["id"] != float64(42) {
		t.Errorf("%s %v", f.path, f.body)
	}

	if _, err := st.Commit(context.Background(), Batch{}); err == nil {
		t.Error("a commit without its update half must be refused")
	}
	if _, err := st.Commit(context.Background(), Batch{Update: &Doc{Entity: EntItem, ID: 1}, Appends: make([]Append, MaxBatchAppends+1)}); err == nil {
		t.Error("more than 25 appends must be refused")
	}
}

func TestXoluStore_FindAndSum(t *testing.T) {
	st, f := newFake(t)
	f.reply = `{"status":"ok","data":[{"id":2,"_version":1,"name":"b"},{"id":1,"_version":3,"name":"a"}]}`
	docs, err := st.Find(context.Background(), Query{Entity: EntAE})
	must(t, err)
	if len(docs) != 2 || docs[0].ID != 2 || docs[1].Version != 3 || docs[1].Data["name"] != "a" {
		t.Errorf("%+v", docs)
	}
	if f.path != "/api/v1/oql/query" || !strings.HasPrefix(f.body["query"].(string), "SELECT TOP 1000 * FROM adm_ae") {
		t.Errorf("%s %v", f.path, f.body)
	}

	f.reply = `{"status":"ok","data":[{"account_code":"1.1.1","sum_debit_cents":"10000.00","sum_credit_cents":250}]}`
	rows, err := st.Sum(context.Background(), EntMovement, "account_code", []string{"debit_cents", "credit_cents"}, nil)
	must(t, err)
	if len(rows) != 1 || rows[0]["account_code"] != "1.1.1" || rows[0]["debit_cents"] != int64(10000) || rows[0]["credit_cents"] != int64(250) {
		t.Errorf("%+v", rows)
	}

	// An entity nothing was ever written to is empty, not an error.
	f.status, f.reply = 404, `{"error":{"code":"XOLU-ST008","message":"schema not found","status":404}}`
	docs, err = st.Find(context.Background(), Query{Entity: EntClose})
	if err != nil || len(docs) != 0 {
		t.Errorf("empty entity: %v %v", docs, err)
	}
	f.status, f.reply = 500, `{"error":"boom"}`
	if _, err = st.Find(context.Background(), Query{Entity: EntClose}); err == nil {
		t.Error("a real failure must surface")
	}
}
