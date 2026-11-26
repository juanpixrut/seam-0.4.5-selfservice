package overrides

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/ha1tch/seam-ui/internal/auth"
)

// fakeXolu is a small in-memory stand-in for xolu's entity API and OQL, strict
// enough to catch a badly quoted query: a WHERE clause that is not exactly
// "col = literal AND col = literal ..." is an error, never a partial match.
type fakeXolu struct {
	mu      sync.Mutex
	next    int64
	tables  map[string]map[int64]map[string]interface{}
	queries []string
}

func newFakeXolu() *fakeXolu {
	return &fakeXolu{tables: map[string]map[int64]map[string]interface{}{}}
}

func (f *fakeXolu) Get(ctx context.Context, entity string, id int64) (*auth.XOLUEntity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	row, ok := f.tables[entity][id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return &auth.XOLUEntity{ID: id, Data: row}, nil
}

func (f *fakeXolu) Create(ctx context.Context, entity string, data map[string]interface{}) (*auth.XOLUEntity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	row := map[string]interface{}{"id": float64(f.next)}
	for k, v := range data {
		row[k] = v
	}
	if f.tables[entity] == nil {
		f.tables[entity] = map[int64]map[string]interface{}{}
	}
	f.tables[entity][f.next] = row
	return &auth.XOLUEntity{ID: f.next, Data: row}, nil
}

func (f *fakeXolu) Update(ctx context.Context, entity string, id int64, data map[string]interface{}) (*auth.XOLUEntity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	row, ok := f.tables[entity][id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	for k, v := range data {
		row[k] = v
	}
	return &auth.XOLUEntity{ID: id, Data: row}, nil
}

func (f *fakeXolu) Delete(ctx context.Context, entity string, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.tables[entity], id)
	return nil
}

var (
	oqlSelect = regexp.MustCompile(`(?s)^SELECT (?:TOP (\d+) )?\* FROM (\w+)(?: WHERE (.+))?$`)
	oqlCond   = regexp.MustCompile(`(\w+) = ('(?:[^']|'')*'|-?\d+)`)
)

func (f *fakeXolu) OQL(ctx context.Context, query string) (*auth.OQLResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queries = append(f.queries, query)
	m := oqlSelect.FindStringSubmatch(query)
	if m == nil {
		return nil, fmt.Errorf("fakeXolu: cannot parse %q", query)
	}
	top, entity, where := 0, m[2], m[3]
	if m[1] != "" {
		top, _ = strconv.Atoi(m[1])
	}
	type cond struct{ col, lit string }
	var conds []cond
	if where != "" {
		var parts []string
		for _, c := range oqlCond.FindAllStringSubmatch(where, -1) {
			conds = append(conds, cond{c[1], c[2]})
			parts = append(parts, c[1]+" = "+c[2])
		}
		if strings.Join(parts, " AND ") != where {
			return nil, fmt.Errorf("fakeXolu: WHERE is not a plain conjunction (bad quoting?): %q", where)
		}
	}
	ids := make([]int64, 0, len(f.tables[entity]))
	for id := range f.tables[entity] {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	var out []map[string]interface{}
	for _, id := range ids {
		row := f.tables[entity][id]
		match := true
		for _, c := range conds {
			if strings.HasPrefix(c.lit, "'") {
				want := strings.ReplaceAll(c.lit[1:len(c.lit)-1], "''", "'")
				if got, _ := row[c.col].(string); got != want {
					match = false
				}
			} else {
				n, _ := strconv.ParseInt(c.lit, 10, 64)
				if asInt64(row[c.col]) != n {
					match = false
				}
			}
		}
		if match {
			cp := map[string]interface{}{}
			for k, v := range row {
				cp[k] = v
			}
			out = append(out, cp)
			if top > 0 && len(out) == top {
				break
			}
		}
	}
	return &auth.OQLResult{Data: out}, nil
}

// errClient is an xolu client that is down.
type errClient struct{}

func (errClient) Get(context.Context, string, int64) (*auth.XOLUEntity, error) {
	return nil, fmt.Errorf("xolu down")
}
func (errClient) Create(context.Context, string, map[string]interface{}) (*auth.XOLUEntity, error) {
	return nil, fmt.Errorf("xolu down")
}
func (errClient) Update(context.Context, string, int64, map[string]interface{}) (*auth.XOLUEntity, error) {
	return nil, fmt.Errorf("xolu down")
}
func (errClient) Delete(context.Context, string, int64) error { return fmt.Errorf("xolu down") }
func (errClient) OQL(context.Context, string) (*auth.OQLResult, error) {
	return nil, fmt.Errorf("xolu down")
}
