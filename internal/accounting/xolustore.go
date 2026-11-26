package accounting

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	xolu "github.com/ha1tch/xolu/pkg/client"
)

// XoluStore is the DocStore over a real xolu server.
//
// Every record of the module lives in an adm_* entity (docstore.go), global to
// the xolu instance rather than scoped to a Seam tenant: Seam Admin sits above
// the tenants. The one thing xolu enforces for us is the optimistic version,
// which carries the "collected once" and "paid once" guarantees; the rest of
// the rules live in Service.
type XoluStore struct {
	c *xolu.Client
}

// NewXoluStore returns a DocStore over c.
func NewXoluStore(c *xolu.Client) *XoluStore { return &XoluStore{c: c} }

// xolu error codes (pkg/errors) the store translates.
const (
	codeNotFound = "XOLU-ST001"
	codeExists   = "XOLU-ST002"
	codeConflict = "XOLU-ST005"
	codeNoSchema = "XOLU-ST008"
)

// mapErr turns a xolu client error into the module's errors.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	var xe *xolu.Error
	if errors.As(err, &xe) {
		switch {
		case xe.Code == codeNotFound || xe.HTTPStatus == 404:
			return ErrNotFound
		case xe.Code == codeConflict || xe.HTTPStatus == 409 && xe.Code != codeExists:
			return ErrConflict
		case xe.Code == codeExists:
			return ErrExists
		}
	}
	return err
}

// isEmptyEntity reports whether err just means "this entity has never been
// written to", which a Find treats as an empty result.
func isEmptyEntity(err error) bool {
	var xe *xolu.Error
	if errors.As(err, &xe) {
		if xe.Code == codeNotFound || xe.Code == codeNoSchema {
			return true
		}
		m := strings.ToLower(xe.Message)
		return strings.Contains(m, "unknown entity") || strings.Contains(m, "entity not found") ||
			strings.Contains(m, "no such table") || strings.Contains(m, "does not exist")
	}
	return false
}

var identRE = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

func checkIdent(kind, s string) error {
	if !identRE.MatchString(s) {
		return fmt.Errorf("accounting: invalid %s %q", kind, s)
	}
	return nil
}

// Create implements DocStore.
func (x *XoluStore) Create(ctx context.Context, entity string, data map[string]any) (int64, error) {
	if err := checkIdent("entity", entity); err != nil {
		return 0, err
	}
	e, err := x.c.Create(ctx, entity, data)
	if err != nil {
		return 0, mapErr(err)
	}
	return e.ID, nil
}

func versionOf(m map[string]any) int {
	switch v := m["_version"].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	}
	return 0
}

func docFrom(entity string, id int64, m map[string]any) Doc {
	data := make(map[string]any, len(m))
	for k, v := range m {
		if k == "id" || k == "_version" {
			continue
		}
		data[k] = v
	}
	return Doc{Entity: entity, ID: id, Version: versionOf(m), Data: data}
}

// Delete implements DocStore.
func (s *XoluStore) Delete(ctx context.Context, entity string, id int64) error {
	if err := checkIdent("entity", entity); err != nil {
		return err
	}
	return mapErr(s.c.Delete(ctx, entity, id))
}

// Get implements DocStore.
func (x *XoluStore) Get(ctx context.Context, entity string, id int64) (Doc, error) {
	if err := checkIdent("entity", entity); err != nil {
		return Doc{}, err
	}
	e, err := x.c.Get(ctx, entity, id)
	if err != nil {
		return Doc{}, mapErr(err)
	}
	return docFrom(entity, id, e.Data), nil
}

// Put implements DocStore. A non-zero expectVersion makes it conditional
// (xolu checks "_version" in the body and answers 409 on a mismatch).
func (x *XoluStore) Put(ctx context.Context, entity string, id int64, data map[string]any, expectVersion int) error {
	if err := checkIdent("entity", entity); err != nil {
		return err
	}
	body := make(map[string]any, len(data)+1)
	for k, v := range data {
		body[k] = v
	}
	delete(body, "id")
	delete(body, "_version")
	if expectVersion != 0 {
		body["_version"] = expectVersion
	}
	_, err := x.c.Update(ctx, entity, id, body)
	return mapErr(err)
}

// Query runs an OQL query and returns its rows (QueryRunner). A query on an
// entity nothing was ever written to has no rows.
func (x *XoluStore) Query(ctx context.Context, oql string) ([]map[string]any, error) {
	res, err := x.c.OQL(ctx, oql)
	if err != nil {
		if isEmptyEntity(err) {
			return nil, nil
		}
		return nil, mapErr(err)
	}
	return res.Data, nil
}

// oqlLiteral renders a condition value. Strings are quoted and escaped;
// numbers and booleans are rendered from their Go values, never from text.
func oqlLiteral(v any) (string, error) {
	switch t := v.(type) {
	case string:
		return "'" + strings.ReplaceAll(t, "'", "''") + "'", nil
	case int:
		return strconv.Itoa(t), nil
	case int64:
		return strconv.FormatInt(t, 10), nil
	case Cents:
		return strconv.FormatInt(int64(t), 10), nil
	case bool:
		if t {
			return "true", nil
		}
		return "false", nil
	}
	return "", fmt.Errorf("accounting: unsupported condition value %T", v)
}

func oqlWhere(conds []Cond) (string, error) {
	if len(conds) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(conds))
	for _, c := range conds {
		if c.Field != "id" {
			if err := checkIdent("field", c.Field); err != nil {
				return "", err
			}
		}
		switch c.Op {
		case "=", "!=", "<", "<=", ">", ">=":
		default:
			return "", fmt.Errorf("accounting: unsupported operator %q", c.Op)
		}
		lit, err := oqlLiteral(c.Value)
		if err != nil {
			return "", err
		}
		parts = append(parts, c.Field+" "+c.Op+" "+lit)
	}
	return " WHERE " + strings.Join(parts, " AND "), nil
}

// findQuery builds the OQL of a Query.
func findQuery(q Query) (string, error) {
	if err := checkIdent("entity", q.Entity); err != nil {
		return "", err
	}
	limit := q.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	where, err := oqlWhere(q.Where)
	if err != nil {
		return "", err
	}
	order := "id"
	if q.OrderBy != "" && q.OrderBy != "id" {
		if err := checkIdent("order field", q.OrderBy); err != nil {
			return "", err
		}
		order = q.OrderBy
	}
	dir := "ASC"
	if q.Desc {
		dir = "DESC"
	}
	return fmt.Sprintf("SELECT TOP %d * FROM %s%s ORDER BY %s %s", limit, q.Entity, where, order, dir), nil
}

func idOf(row map[string]any) int64 {
	switch v := row["id"].(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	}
	return 0
}

// Find implements DocStore. Ties on the order field are broken by id so that
// the order is stable, as the in-memory store's is.
func (x *XoluStore) Find(ctx context.Context, q Query) ([]Doc, error) {
	oql, err := findQuery(q)
	if err != nil {
		return nil, err
	}
	res, err := x.c.OQL(ctx, oql)
	if err != nil {
		if isEmptyEntity(err) {
			return nil, nil
		}
		return nil, mapErr(err)
	}
	out := make([]Doc, 0, len(res.Data))
	for _, row := range res.Data {
		out = append(out, docFrom(q.Entity, idOf(row), row))
	}
	return out, nil
}

// numeric turns what xolu returns for a SUM (a number, or a decimal string)
// into an int64 cent amount.
func numeric(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(math.Round(n))
	case int64:
		return n
	case int:
		return int64(n)
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(n), 64); err == nil {
			return int64(math.Round(f))
		}
	}
	return 0
}

// sumQuery builds the OQL of a Sum.
func sumQuery(entity, groupBy string, fields []string, where []Cond) (string, error) {
	if err := checkIdent("entity", entity); err != nil {
		return "", err
	}
	if err := checkIdent("group field", groupBy); err != nil {
		return "", err
	}
	cols := []string{groupBy}
	for _, f := range fields {
		if err := checkIdent("sum field", f); err != nil {
			return "", err
		}
		cols = append(cols, fmt.Sprintf("SUM(%s) AS sum_%s", f, f))
	}
	w, err := oqlWhere(where)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("SELECT %s FROM %s%s GROUP BY %s", strings.Join(cols, ", "), entity, w, groupBy), nil
}

// Sum implements DocStore.
func (x *XoluStore) Sum(ctx context.Context, entity, groupBy string, fields []string, where []Cond) ([]map[string]any, error) {
	oql, err := sumQuery(entity, groupBy, fields, where)
	if err != nil {
		return nil, err
	}
	res, err := x.c.OQL(ctx, oql)
	if err != nil {
		if isEmptyEntity(err) {
			return nil, nil
		}
		return nil, mapErr(err)
	}
	out := make([]map[string]any, 0, len(res.Data))
	for _, row := range res.Data {
		r := map[string]any{groupBy: row[groupBy]}
		for _, f := range fields {
			r[f] = numeric(row["sum_"+f])
		}
		out = append(out, r)
	}
	return out, nil
}

// Commit implements DocStore with xolu's atomic commit. It needs the update
// half: every write of the module that appends records also touches one record
// (an item, a settlement, the ledger head), which is what carries the version
// check.
func (x *XoluStore) Commit(ctx context.Context, b Batch) ([]int64, error) {
	if b.Update == nil {
		return nil, errors.New("accounting: a commit needs an update (use the ledger head)")
	}
	if len(b.Appends) > MaxBatchAppends {
		return nil, fmt.Errorf("accounting: a commit holds at most %d appended records", MaxBatchAppends)
	}
	if err := checkIdent("entity", b.Update.Entity); err != nil {
		return nil, err
	}
	req := xolu.CommitRequest{Update: xolu.CommitUpdate{Entity: b.Update.Entity, ID: b.Update.ID, Data: b.Update.Data}}
	if b.ExpectVersion != 0 {
		v := b.ExpectVersion
		req.Update.Version = &v
	}
	for _, a := range b.Appends {
		if err := checkIdent("entity", a.Entity); err != nil {
			return nil, err
		}
		ca := xolu.CommitAppend{Entity: a.Entity, Data: a.Data}
		if a.ID != 0 {
			id := a.ID
			ca.ID = &id
		}
		req.Append = append(req.Append, ca)
	}
	res, err := x.c.Commit(ctx, req)
	if err != nil {
		return nil, mapErr(err)
	}
	ids := make([]int64, 0, len(res.Appended))
	for _, a := range res.Appended {
		ids = append(ids, a.ID)
	}
	if len(ids) != len(b.Appends) {
		return nil, fmt.Errorf("accounting: xolu appended %d of %d records", len(ids), len(b.Appends))
	}
	return ids, nil
}

var _ DocStore = (*XoluStore)(nil)
