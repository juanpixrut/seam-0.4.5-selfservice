package accounting

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
)

// MemStore is an in-memory DocStore. It mimics the behaviour of xolu the
// module relies on (auto ids, versions, atomic batches, explicit-id
// collisions) and is what the unit tests of Service run against.
type MemStore struct {
	mu     sync.Mutex
	nextID map[string]int64
	rows   map[string]map[int64]*memRow
}

type memRow struct {
	version int
	data    map[string]any
}

// NewMemStore returns an empty MemStore.
func NewMemStore() *MemStore {
	return &MemStore{nextID: map[string]int64{}, rows: map[string]map[int64]*memRow{}}
}

// clone deep-copies a record through JSON, which is also how xolu hands data
// back: numbers become float64, nested values become maps and slices. Running
// the in-memory store through the same round trip keeps tests honest about
// types.
func clone(data map[string]any) map[string]any {
	b, err := json.Marshal(data)
	if err != nil {
		panic(fmt.Sprintf("accounting: memstore cannot encode record: %v", err))
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		panic(fmt.Sprintf("accounting: memstore cannot decode record: %v", err))
	}
	return out
}

func (m *MemStore) table(entity string) map[int64]*memRow {
	t := m.rows[entity]
	if t == nil {
		t = map[int64]*memRow{}
		m.rows[entity] = t
	}
	return t
}

func (m *MemStore) insertLocked(entity string, id int64, data map[string]any) (int64, error) {
	t := m.table(entity)
	if id == 0 {
		m.nextID[entity]++
		id = m.nextID[entity]
		for t[id] != nil {
			m.nextID[entity]++
			id = m.nextID[entity]
		}
	} else if t[id] != nil {
		return 0, ErrExists
	} else if id > m.nextID[entity] {
		m.nextID[entity] = id
	}
	t[id] = &memRow{version: 1, data: clone(data)}
	return id, nil
}

// Create implements DocStore.
func (m *MemStore) Create(_ context.Context, entity string, data map[string]any) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.insertLocked(entity, 0, data)
}

// Delete implements DocStore.
func (m *MemStore) Delete(_ context.Context, entity string, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rows[entity][id] == nil {
		return ErrNotFound
	}
	delete(m.rows[entity], id)
	return nil
}

// Get implements DocStore.
func (m *MemStore) Get(_ context.Context, entity string, id int64) (Doc, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.rows[entity][id]
	if r == nil {
		return Doc{}, ErrNotFound
	}
	return Doc{Entity: entity, ID: id, Version: r.version, Data: clone(r.data)}, nil
}

func (m *MemStore) putLocked(entity string, id int64, data map[string]any, expect int) error {
	r := m.rows[entity][id]
	if r == nil {
		return ErrNotFound
	}
	if expect != 0 && r.version != expect {
		return ErrConflict
	}
	r.version++
	r.data = clone(data)
	return nil
}

// Put implements DocStore.
func (m *MemStore) Put(_ context.Context, entity string, id int64, data map[string]any, expect int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.putLocked(entity, id, data, expect)
}

func matches(data map[string]any, id int64, conds []Cond) bool {
	for _, c := range conds {
		var have any
		if c.Field == "id" {
			have = float64(id)
		} else {
			have = data[c.Field]
		}
		if !compare(have, c.Op, c.Value) {
			return false
		}
	}
	return true
}

func toNum(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case Cents:
		return float64(n), true
	}
	return 0, false
}

func compare(have any, op string, want any) bool {
	if hn, ok := toNum(have); ok {
		if wn, ok := toNum(want); ok {
			switch op {
			case "=":
				return hn == wn
			case "!=":
				return hn != wn
			case "<":
				return hn < wn
			case "<=":
				return hn <= wn
			case ">":
				return hn > wn
			case ">=":
				return hn >= wn
			}
		}
		return false
	}
	if hb, ok := have.(bool); ok {
		wb, ok := want.(bool)
		if !ok {
			return false
		}
		switch op {
		case "=":
			return hb == wb
		case "!=":
			return hb != wb
		}
		return false
	}
	hs, _ := have.(string)
	ws, wantIsString := want.(string)
	if have == nil {
		// A missing field equals only the empty string; it never equals a
		// number or a bool, and orders before everything.
		switch op {
		case "=":
			return wantIsString && ws == ""
		case "!=":
			return !(wantIsString && ws == "")
		}
		return false
	}
	switch op {
	case "=":
		return hs == ws
	case "!=":
		return hs != ws
	case "<":
		return hs < ws
	case "<=":
		return hs <= ws
	case ">":
		return hs > ws
	case ">=":
		return hs >= ws
	}
	return false
}

// Find implements DocStore.
func (m *MemStore) Find(_ context.Context, q Query) ([]Doc, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Doc
	for id, r := range m.rows[q.Entity] {
		if matches(r.data, id, q.Where) {
			out = append(out, Doc{Entity: q.Entity, ID: id, Version: r.version, Data: clone(r.data)})
		}
	}
	key := func(d Doc) any {
		if q.OrderBy == "" || q.OrderBy == "id" {
			return float64(d.ID)
		}
		return d.Data[q.OrderBy]
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := key(out[i]), key(out[j])
		less := compare(a, "<", b)
		greater := compare(a, ">", b)
		if !less && !greater {
			less = out[i].ID < out[j].ID
			greater = out[i].ID > out[j].ID
		}
		if q.Desc {
			return greater
		}
		return less
	})
	limit := q.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// Sum implements DocStore.
func (m *MemStore) Sum(_ context.Context, entity, groupBy string, fields []string, where []Cond) ([]map[string]any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	groups := map[string]map[string]float64{}
	var order []string
	for id, r := range m.rows[entity] {
		if !matches(r.data, id, where) {
			continue
		}
		g, _ := r.data[groupBy].(string)
		sums := groups[g]
		if sums == nil {
			sums = map[string]float64{}
			groups[g] = sums
			order = append(order, g)
		}
		for _, f := range fields {
			if n, ok := toNum(r.data[f]); ok {
				sums[f] += n
			}
		}
	}
	sort.Strings(order)
	out := make([]map[string]any, 0, len(order))
	for _, g := range order {
		row := map[string]any{groupBy: g}
		for _, f := range fields {
			row[f] = groups[g][f]
		}
		out = append(out, row)
	}
	return out, nil
}

// Commit implements DocStore. Nothing is written unless every part succeeds.
func (m *MemStore) Commit(_ context.Context, b Batch) ([]int64, error) {
	if len(b.Appends) > MaxBatchAppends {
		return nil, fmt.Errorf("accounting: a batch takes at most %d appends, got %d", MaxBatchAppends, len(b.Appends))
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	// Snapshot what we may touch so a failure part-way rolls back exactly.
	type undo struct {
		entity string
		id     int64
		row    *memRow // nil: the row did not exist
		next   int64
	}
	var undos []undo
	rollback := func() {
		for i := len(undos) - 1; i >= 0; i-- {
			u := undos[i]
			if u.row == nil {
				delete(m.rows[u.entity], u.id)
			} else {
				m.rows[u.entity][u.id] = u.row
			}
			m.nextID[u.entity] = u.next
		}
	}

	if b.Update != nil {
		u := b.Update
		if old := m.rows[u.Entity][u.ID]; old != nil {
			undos = append(undos, undo{u.Entity, u.ID, &memRow{version: old.version, data: old.data}, m.nextID[u.Entity]})
		}
		if err := m.putLocked(u.Entity, u.ID, u.Data, b.ExpectVersion); err != nil {
			return nil, err
		}
	}
	ids := make([]int64, 0, len(b.Appends))
	for _, a := range b.Appends {
		next := m.nextID[a.Entity]
		id, err := m.insertLocked(a.Entity, a.ID, a.Data)
		if err != nil {
			rollback()
			return nil, err
		}
		undos = append(undos, undo{a.Entity, id, nil, next})
		ids = append(ids, id)
	}
	return ids, nil
}
