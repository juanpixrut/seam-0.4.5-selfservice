package ui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	mi "github.com/ha1tch/minty"

	"github.com/ha1tch/seam-ui/internal/auth"
	fe "github.com/ha1tch/seam-ui/internal/formengine"
	"github.com/ha1tch/seam-ui/internal/querypicker"
)

// The server side of the "query-picker" form widget (docs/QUERY_PICKER.md).
//
//	GET /picker/query?s=<token>&q=<text>           the first rows matching the text
//	GET /picker/query?s=<token>&value=<value>      the row for one stored value, or none
//	GET /picker/query/browse?s=<token>&q=&page=&target=   the Selector dialog over the same query
//
// The token is the field's specification, sealed by the form engine when it
// rendered the field (querypicker.Seal). The browser cannot read it and cannot
// make one: a request carries no query, no table and no column name, only the
// search text, which reaches the query as an escaped literal.
//
// Every query runs through h.xoluClient, which is scoped to the session's
// tenant like every other query in this package.
//
// Cost (the reason for most of what follows): a search is a LIKE over the whole
// entity. The widget waits before asking and asks for few rows; here, an
// identical search within a few seconds is answered from memory, and a column
// that references a small entity is resolved from a cached copy of that entity
// instead of a JOIN.

const (
	queryPickerMaxRunes     = 100
	queryPickerBrowseSize   = 10
	queryPickerResultTTL    = 5 * time.Second
	queryPickerResultMax    = 1000
	queryPickerLookupTTL    = 60 * time.Second
	queryPickerLookupMaxRow = 2000
)

type queryPickerResponse struct {
	Rows []querypicker.Row `json:"rows"`
	More bool              `json:"more"`
}

// ── caches ───────────────────────────────────────────────────────────────────

type queryPickerResultEntry struct {
	at   time.Time
	body []byte
}

type queryPickerLookupEntry struct {
	at    time.Time
	names map[string]string // id -> text; nil when the entity is too large to hold
}

var queryPickerCache = struct {
	sync.Mutex
	results map[string]queryPickerResultEntry
	lookups map[string]queryPickerLookupEntry
	now     func() time.Time
}{
	results: map[string]queryPickerResultEntry{},
	lookups: map[string]queryPickerLookupEntry{},
	now:     time.Now,
}

// resetQueryPickerCache empties both caches (tests).
func resetQueryPickerCache() {
	queryPickerCache.Lock()
	queryPickerCache.results = map[string]queryPickerResultEntry{}
	queryPickerCache.lookups = map[string]queryPickerLookupEntry{}
	queryPickerCache.Unlock()
}

func queryPickerParams(ctx context.Context, q string) querypicker.Params {
	p := querypicker.Params{Q: q}
	if claims, ok := auth.ClaimsFromContext(ctx); ok {
		p.TenantID, p.UserID = claims.TenantID, claims.UserID
	}
	return p
}

// queryPickerResultKey identifies one search for the result cache: the field,
// the tenant, the text and what kind of request it is. The user is part of it
// only when the query itself depends on the user.
func queryPickerResultKey(spec *querypicker.Spec, p querypicker.Params, kind string) string {
	raw, _ := json.Marshal(spec)
	h := sha256.New()
	h.Write(raw)
	h.Write([]byte{0})
	h.Write([]byte(kind))
	h.Write([]byte{0})
	h.Write([]byte(strconv.FormatInt(p.TenantID, 10)))
	h.Write([]byte{0})
	if strings.Contains(spec.Query, ":user_id") {
		h.Write([]byte(strconv.FormatInt(p.UserID, 10)))
	}
	h.Write([]byte{0})
	h.Write([]byte(p.Q))
	return hex.EncodeToString(h.Sum(nil))
}

func queryPickerCachedResult(key string) ([]byte, bool) {
	queryPickerCache.Lock()
	defer queryPickerCache.Unlock()
	e, ok := queryPickerCache.results[key]
	if !ok || queryPickerCache.now().Sub(e.at) > queryPickerResultTTL {
		return nil, false
	}
	return e.body, true
}

func queryPickerStoreResult(key string, body []byte) {
	queryPickerCache.Lock()
	defer queryPickerCache.Unlock()
	now := queryPickerCache.now()
	if len(queryPickerCache.results) >= queryPickerResultMax {
		for k, e := range queryPickerCache.results {
			if now.Sub(e.at) > queryPickerResultTTL {
				delete(queryPickerCache.results, k)
			}
		}
		if len(queryPickerCache.results) >= queryPickerResultMax {
			queryPickerCache.results = map[string]queryPickerResultEntry{}
		}
	}
	queryPickerCache.results[key] = queryPickerResultEntry{at: now, body: body}
}

// ── running queries ──────────────────────────────────────────────────────────

func (h *UIHandler) queryPickerRun(ctx context.Context, oql string) ([]map[string]any, error) {
	if h.xoluClient == nil {
		return nil, errors.New("no xolu client")
	}
	res, err := h.xoluClient.OQL(ctx, oql)
	if err != nil {
		if entityNotYetCreated(err) {
			return nil, nil
		}
		return nil, err
	}
	return res.Data, nil
}

// queryPickerLookup returns the resolver for a Spec's lookup columns. Each
// referenced entity is read once, with the two columns it needs, and kept per
// tenant for queryPickerLookupTTL. An entity with more rows than
// queryPickerLookupMaxRow is not held: its cells show the id.
func (h *UIHandler) queryPickerLookup(ctx context.Context, spec *querypicker.Spec, tenant int64) func(c querypicker.Column, id string) (string, bool) {
	tables := map[string]map[string]string{}
	for _, c := range spec.Columns {
		if c.Lookup == "" {
			continue
		}
		if _, done := tables[c.Lookup]; done {
			continue
		}
		tables[c.Lookup] = h.queryPickerLookupTable(ctx, c.Lookup, tenant)
	}
	if len(tables) == 0 {
		return nil
	}
	return func(c querypicker.Column, id string) (string, bool) {
		text, ok := tables[c.Lookup][id]
		return text, ok && text != ""
	}
}

func (h *UIHandler) queryPickerLookupTable(ctx context.Context, lookup string, tenant int64) map[string]string {
	key := strconv.FormatInt(tenant, 10) + "|" + lookup
	queryPickerCache.Lock()
	e, ok := queryPickerCache.lookups[key]
	fresh := ok && queryPickerCache.now().Sub(e.at) <= queryPickerLookupTTL
	queryPickerCache.Unlock()
	if fresh {
		return e.names
	}
	entity, column, _ := strings.Cut(lookup, ".")
	rows, err := h.queryPickerRun(ctx, "SELECT TOP "+strconv.Itoa(queryPickerLookupMaxRow+1)+" id, "+column+" FROM "+entity)
	if err != nil {
		h.logger.Error("query-picker lookup failed", "lookup", lookup, "error", err)
		return nil // not cached: the next request tries again
	}
	var names map[string]string
	if len(rows) <= queryPickerLookupMaxRow {
		names = make(map[string]string, len(rows))
		for _, r := range rows {
			names[querypicker.Text(r["id"])] = querypicker.Text(r[column])
		}
	}
	queryPickerCache.Lock()
	queryPickerCache.lookups[key] = queryPickerLookupEntry{at: queryPickerCache.now(), names: names}
	queryPickerCache.Unlock()
	return names
}

func queryPickerText(raw string) string {
	q := strings.TrimSpace(raw)
	if rs := []rune(q); len(rs) > queryPickerMaxRunes {
		q = string(rs[:queryPickerMaxRunes])
	}
	return q
}

// queryPickerSearch returns the first top rows matching q, and whether more exist.
func (h *UIHandler) queryPickerSearch(ctx context.Context, spec *querypicker.Spec, p querypicker.Params, top int) ([]querypicker.Row, bool, error) {
	oql, err := spec.SearchQuery(p, top+1)
	if err != nil {
		return nil, false, err
	}
	raw, err := h.queryPickerRun(ctx, oql)
	if err != nil {
		return nil, false, err
	}
	more := len(raw) > top
	if more {
		raw = raw[:top]
	}
	lookup := h.queryPickerLookup(ctx, spec, p.TenantID)
	rows := make([]querypicker.Row, len(raw))
	for i, r := range raw {
		rows[i] = spec.Row(r, lookup)
	}
	return rows, more, nil
}

// queryPickerFind returns the row the query yields for one value, if any.
func (h *UIHandler) queryPickerFind(ctx context.Context, spec *querypicker.Spec, p querypicker.Params, value string) (querypicker.Row, bool, error) {
	oql, err := spec.LookupQuery(p, value)
	if err != nil {
		return querypicker.Row{}, false, nil // a value of the wrong kind is "not found", not a server error
	}
	raw, err := h.queryPickerRun(ctx, oql)
	if err != nil || len(raw) == 0 {
		return querypicker.Row{}, false, err
	}
	return spec.Row(raw[0], h.queryPickerLookup(ctx, spec, p.TenantID)), true, nil
}

// ── handlers ─────────────────────────────────────────────────────────────────

// PickerQuery handles GET /picker/query.
func (h *UIHandler) PickerQuery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()
	spec, err := querypicker.Open(query.Get("s"))
	if err != nil {
		http.Error(w, "unknown picker", http.StatusBadRequest)
		return
	}
	value, byValue := query.Get("value"), query.Has("value")
	text := queryPickerText(query.Get("q"))
	p := queryPickerParams(ctx, text)
	kind := "q"
	if byValue {
		kind, p.Q = "v", value
	}
	key := queryPickerResultKey(spec, p, kind)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if body, ok := queryPickerCachedResult(key); ok {
		w.Write(body)
		return
	}

	resp := queryPickerResponse{Rows: []querypicker.Row{}}
	switch {
	case byValue:
		if len(value) > 0 && len(value) <= 200 {
			row, found, err := h.queryPickerFind(ctx, spec, queryPickerParams(ctx, ""), value)
			if err != nil {
				h.logger.Error("query-picker lookup failed", "error", err)
				http.Error(w, h.errText(r, "error.search_failed", "search failed"), http.StatusInternalServerError)
				return
			}
			if found {
				resp.Rows = append(resp.Rows, row)
			}
		}
	case len([]rune(text)) >= fe.QueryPickerMinChars:
		rows, more, err := h.queryPickerSearch(ctx, spec, p, spec.Limit)
		if err != nil {
			h.logger.Error("query-picker search failed", "error", err)
			http.Error(w, h.errText(r, "error.search_failed", "search failed"), http.StatusInternalServerError)
			return
		}
		resp.Rows, resp.More = rows, more
	}
	body, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, h.errText(r, "error.search_failed", "search failed"), http.StatusInternalServerError)
		return
	}
	queryPickerStoreResult(key, body)
	w.Write(body)
}

// PickerQueryBrowse handles GET /picker/query/browse: the Selector dialog
// (selector.go) over the field's own query, with its search box and paging.
func (h *UIHandler) PickerQueryBrowse(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()
	token := query.Get("s")
	spec, err := querypicker.Open(token)
	if err != nil {
		http.Error(w, "unknown picker", http.StatusBadRequest)
		return
	}
	q := queryPickerText(query.Get("q"))
	page := 1
	if n, err := strconv.Atoi(query.Get("page")); err == nil && n > 1 {
		page = n
	}
	if page > selectorMaxPage {
		page = selectorMaxPage
	}
	target := query.Get("target")
	if !selectorTargetPattern.MatchString(target) {
		target = ""
	}

	p := queryPickerParams(ctx, q)
	found, more, err := h.queryPickerSearch(ctx, spec, p, page*queryPickerBrowseSize)
	if err != nil {
		h.logger.Error("query-picker browse failed", "error", err)
		http.Error(w, h.errText(r, "error.search_failed", "selector failed"), http.StatusInternalServerError)
		return
	}
	if skip := (page - 1) * queryPickerBrowseSize; skip < len(found) {
		found = found[skip:]
	} else {
		found = nil
	}
	rows := make([]SelectorRow, len(found))
	for i, row := range found {
		cells := map[string]any{}
		for j, c := range row.Cells {
			cells[strconv.Itoa(j)] = c
		}
		rows[i] = SelectorRow{Value: row.Value, Label: row.Label, Data: cells}
	}

	src := SelectorSource{
		Key:             "query",
		EmptyTitleKey:   "selector.empty_title",
		EmptyMessageKey: "selector.empty_message",
		URL: func(q string, page int, target string) string {
			v := url.Values{}
			v.Set("s", token)
			if q != "" {
				v.Set("q", q)
			}
			if page > 1 {
				v.Set("page", strconv.Itoa(page))
			}
			if target != "" {
				v.Set("target", target)
			}
			return "/picker/query/browse?" + v.Encode()
		},
	}
	for i, c := range spec.Columns {
		src.Columns = append(src.Columns, queryPickerSelectorColumn(i, c))
	}
	t := h.translateFunc(r)
	if err := h.page.RenderPartial(w, selectorBody(src, rows, more, q, page, target, t)); err != nil {
		h.logger.Error("query-picker browse render failed", "error", err)
	}
}

// queryPickerSelectorColumn draws column i of a query-picker in the Selector
// dialog with the List cells, typed as in the dropdown. The heading is the one
// sealed with the field, already in the page's language.
func queryPickerSelectorColumn(i int, c querypicker.Column) SelectorColumn {
	heading := c.Title
	if heading == "" {
		heading = c.Field
	}
	right := c.Type == "number" || c.Type == "money"
	return SelectorColumn{
		LabelKey: heading, // not a key: t returns an unknown key unchanged
		Right:    right,
		Cell: func(r SelectorRow, t func(key string, args ...any) string) mi.H {
			cell, _ := r.Data[strconv.Itoa(i)].(querypicker.Cell)
			switch {
			case c.Type == "badge" && cell.V != "":
				variant := cell.B
				if variant == "neutral" {
					variant = ""
				}
				return ListCellRich(Badge(cell.V, variant))
			case c.Type == "icon" && cell.V != "":
				return ListCellRich(func(b *mi.Builder) mi.Node { return b.Span(mi.Class("material-icons text-base"), cell.V) })
			case c.Type == "bool":
				glyph := "remove"
				if cell.V == "true" {
					glyph = "check"
				}
				return ListCellRich(func(b *mi.Builder) mi.Node { return b.Span(mi.Class("material-icons text-base"), glyph) })
			case right:
				return ListCellRight(func(b *mi.Builder) mi.Node {
					return b.Span(mi.Class("text-sm text-gray-900 dark:text-white tabular-nums whitespace-nowrap"), cell.V)
				})
			case c.Muted || c.Type == "date" || c.Type == "datetime":
				return ListCellMuted(cell.V)
			}
			return ListCell(cell.V)
		},
	}
}

// ── submitted values ─────────────────────────────────────────────────────────

// QueryPickerErrors checks every query-picker value of a submitted form: a
// value must be a row the field's query returns for this session, so a
// hand-made request cannot store a record the user could not have chosen.
// values holds the form's fields by name; a repeatable's value is its rows
// (as fe.ParseIndexedForm returns them). Empty values are not checked here:
// "required" is the validator's business. The result maps field names (a
// repeatable's as name[i].item) to a translated message, and is nil when
// every value is acceptable.
func (h *UIHandler) QueryPickerErrors(ctx context.Context, form *fe.Form, values map[string]interface{}, t func(key string, args ...any) string) map[string]string {
	if form == nil {
		return nil
	}
	var errs map[string]string
	fail := func(name string) {
		if errs == nil {
			errs = map[string]string{}
		}
		msg := "field.query_picker.invalid"
		if t != nil {
			msg = t(msg)
		}
		errs[name] = msg
	}
	p := queryPickerParams(ctx, "")
	var walk func(fields []fe.Field, values map[string]interface{}, prefix string)
	walk = func(fields []fe.Field, values map[string]interface{}, prefix string) {
		for _, f := range fields {
			v, present := values[f.Name]
			if !present || v == nil {
				continue
			}
			if f.QueryPicker != nil {
				text := querypicker.Text(v)
				if text == "" {
					continue
				}
				if _, found, err := h.queryPickerFind(ctx, f.QueryPicker, p, text); err != nil || !found {
					if err != nil {
						h.logger.Error("query-picker value check failed", "field", f.Name, "error", err)
					}
					fail(prefix + f.Name)
				}
				continue
			}
			if len(f.Items) == 0 {
				continue
			}
			var rows []map[string]interface{}
			switch list := v.(type) {
			case []map[string]interface{}:
				rows = list
			case []interface{}:
				for _, item := range list {
					if m, ok := item.(map[string]interface{}); ok {
						rows = append(rows, m)
					}
				}
			}
			for i, row := range rows {
				walk(f.Items, row, prefix+f.Name+"["+strconv.Itoa(i)+"].")
			}
		}
	}
	walk(form.Fields, values, "")
	return errs
}
