package ui

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
)

// Backend for the "asset-picker" formengine widget (T-19, 2026-08-04) --
// a generic, live search endpoint, one implementation shared by every
// x-seam-input: "asset-picker" field regardless of which entity it
// points at (parent_id -> entity:assets today; a future picker field
// for a different entity looks identical in shape). Deliberately does
// NOT trust client-supplied field names for query construction -- the
// picker widget's own x-seam-search-fields/x-seam-display-fields are
// documentation of intent at the schema level, but the actual OQL
// built here comes from this file's own server-side, hardcoded config,
// the same trust boundary asset_source_resolver.go's assetSourceQueries
// already established for the plain FK-select widgets. A client that
// could supply arbitrary field names for a WHERE ... LIKE clause would
// be exactly the injection shape T-63 (this session) found and fixed
// across six other handlers -- not repeating that mistake here.

// pickerSearchConfig pairs one entity: key with the OQL search this
// endpoint runs and a function that turns one result row into the
// {id, label} pair the widget's JS actually needs.
type pickerSearchConfig struct {
	table string // real xolu entity/table name
	// searchColumn is matched against with a case-sensitive LIKE '%q%'
	// (OQL's own LIKE semantics -- confirmed case-sensitive against a
	// real running instance during this session's earlier OQL work,
	// not assumed) plus an exact "id = <q>" branch when q parses as an
	// integer, mirroring the exact query shape T-19's own notes
	// confirmed working.
	searchColumn string
	label        func(row map[string]any) string

	// search and lookup, when non-nil, replace the raw-OQL search and the
	// xolu.Get id lookup respectively (T-97): a source that must go through
	// a service and its own visibility rules (the users source goes through
	// auth.UserService) supplies them instead of table/searchColumn.
	// lookup keeps runPickerIDLookup's exact-id contract: at most one
	// result, matched on id alone, never a substring match.
	search func(ctx context.Context, h *UIHandler, q, excludeRaw string) ([]pickerSearchResult, error)
	lookup func(ctx context.Context, h *UIHandler, id int64) ([]pickerSearchResult, error)
}

var pickerSearchConfigs = map[string]pickerSearchConfig{
	"entity:assets": {
		table:        "assets",
		searchColumn: "name",
		label: func(row map[string]any) string {
			name := toString(row["name"])
			status := toString(row["status"])
			if status == "" {
				return name
			}
			return name + " — " + status
		},
	},
	"entity:users":                usersPickerConfig,
	"entity:cost_centers":         {table: "cost_centers", searchColumn: "name", label: costCenterLabel},
	"entity:locations":            {table: "locations", searchColumn: "short_name", label: locationLabel},
	"entity:custodians":           {table: "custodians", searchColumn: "legal_name", label: custodianLabel},
	"entity:asset_types":          {table: "asset_types", searchColumn: "name", label: assetTypeSelectorLabel},
	"entity:wo_actions":           {table: "wo_actions", searchColumn: "name", label: woActionSelectorLabel},
	"entity:warehouse":            {table: "warehouse", searchColumn: "name", label: warehouseSelectorLabel},
	"entity:unit":                 {table: "unit", searchColumn: "name", label: unitSelectorLabel},
	"entity:consumable_types":     {table: consumableTypesEntity, searchColumn: "name", label: func(row map[string]any) string { return toString(row["name"]) }},
	"entity:consumable":           {table: "consumable", searchColumn: "name", label: consumableSelectorLabel},
	"entity:work_order_templates": {table: "work_order_templates", searchColumn: "name", label: woTemplateSelectorLabel},
	"entity:crews": {table: "crews", searchColumn: "name", label: func(row map[string]any) string {
		return toString(row["name"])
	}},
	"entity:outsourced": {table: "outsourced", searchColumn: "name", label: func(row map[string]any) string {
		return toString(row["name"])
	}},
}

type pickerSearchResult struct {
	ID    int64  `json:"id"`
	Label string `json:"label"`
}

// PickerSearch handles GET /picker/search?entity=<key>&q=<term> (live
// search-as-you-type, up to 20 results) or
// GET /picker/search?entity=<key>&id=<id> (exact-match lookup of one
// record by ID, at most 1 result) -- the asset-picker widget's own JS
// uses the id= form once, on page load, to resolve an already-set
// value's display label (the widget only ever receives the raw ID as
// its Go-rendered value, never a label alongside it) without needing
// any new server-side rendering-time lookup plumbing; q= powers the
// actual live search as the user types. entity must be one of
// pickerSearchConfigs' own keys -- an unrecognised value is a 400, not
// silently ignored.
func (h *UIHandler) PickerSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entity := r.URL.Query().Get("entity")
	q := r.URL.Query().Get("q")
	idParam := r.URL.Query().Get("id")

	cfg, ok := pickerSearchConfigs[entity]
	if !ok {
		http.Error(w, "unknown picker entity", http.StatusBadRequest)
		return
	}

	var results []pickerSearchResult
	var err error
	if idParam != "" {
		results, err = h.runPickerIDLookup(ctx, cfg, idParam)
	} else {
		results, err = h.runPickerSearch(ctx, cfg, q, r.URL.Query().Get("exclude"))
	}
	if err != nil {
		h.logger.Error("picker search failed", "entity", entity, "error", err)
		http.Error(w, h.errText(r, "error.search_failed", "search failed"), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// runPickerIDLookup is the exact-match counterpart to runPickerSearch --
// deliberately NOT the same "id = X OR field LIKE '%X%'" query the live
// search uses, since a substring-matching OR clause could return more
// than the one record a label-resolution lookup actually wants (e.g.
// id=5 would also match any asset with "5" somewhere in its name).
func (h *UIHandler) runPickerIDLookup(ctx context.Context, cfg pickerSearchConfig, idParam string) ([]pickerSearchResult, error) {
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		return []pickerSearchResult{}, nil // a malformed id is "nothing found", not a server error
	}
	if cfg.lookup != nil {
		return cfg.lookup(ctx, h, id)
	}
	entity, ok := xoluEntityForPickerTable(cfg.table)
	if !ok {
		entity = cfg.table
	}
	e, err := h.xoluClient.Get(ctx, entity, id)
	if err != nil {
		if entityNotYetCreated(err) {
			return []pickerSearchResult{}, nil
		}
		return nil, err
	}
	return []pickerSearchResult{{ID: e.ID, Label: cfg.label(e.Data)}}, nil
}

func (h *UIHandler) runPickerSearch(ctx context.Context, cfg pickerSearchConfig, q, excludeRaw string) ([]pickerSearchResult, error) {
	if cfg.search != nil {
		return cfg.search(ctx, h, q, excludeRaw)
	}
	query := "SELECT TOP 20 * FROM " + cfg.table

	var conditions []string
	if q != "" {
		escaped := escapeOQL(q)
		if idVal, err := strconv.ParseInt(q, 10, 64); err == nil {
			conditions = append(conditions, "(id = "+strconv.FormatInt(idVal, 10)+" OR "+cfg.searchColumn+" LIKE '%"+escaped+"%')")
		} else {
			conditions = append(conditions, cfg.searchColumn+" LIKE '%"+escaped+"%'")
		}
	}
	if excludeRaw != "" {
		if excludeID, err := strconv.ParseInt(excludeRaw, 10, 64); err == nil {
			conditions = append(conditions, "id != "+strconv.FormatInt(excludeID, 10))
		}
	}
	if len(conditions) > 0 {
		query += " WHERE " + joinStrings(conditions, " AND ")
	}
	query += " ORDER BY " + cfg.searchColumn

	result, err := h.xoluClient.OQL(ctx, query)
	if err != nil {
		if entityNotYetCreated(err) {
			// Same graceful-degradation reasoning as AssetSourceResolver:
			// a fresh tenant with no rows yet for this table is an
			// empty, functional result set, not a broken search.
			return []pickerSearchResult{}, nil
		}
		return nil, err
	}

	out := make([]pickerSearchResult, 0, len(result.Data))
	for _, row := range result.Data {
		out = append(out, pickerSearchResult{
			ID:    toInt64(row["id"]),
			Label: cfg.label(row),
		})
	}
	return out, nil
}

// xoluEntityForPickerTable exists only because pickerSearchConfig's own
// table field and xolu.Client.Get's entity argument happen to be the
// exact same string for every config so far (both "assets") -- kept as
// an explicit, named mapping rather than assuming table name always
// equals xolu entity name, since that assumption has been wrong
// elsewhere in this codebase before (masters entities' own Spanish/
// English renames, T-40).
func xoluEntityForPickerTable(table string) (string, bool) {
	switch table {
	case "assets":
		return "assets", true
	default:
		return "", false
	}
}
