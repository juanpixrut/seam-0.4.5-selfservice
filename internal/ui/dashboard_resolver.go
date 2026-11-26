package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/dashboardengine"
)

// xoluDashboardResolver implements dashboardengine.EntityResolver against
// a real xolu instance. dashboardengine itself has no xolu dependency
// (same decoupling as formengine.SourceResolver) -- this is the concrete
// implementation UIHandler injects, translating an EntitySource into the
// exact OQL this project's other handlers already write by hand.

// authPermissionChecker implements dashboardengine.PermissionChecker by
// wrapping auth.HasPermission -- the whole adapter, since HasPermission's
// own signature already matches (role string, permission string) modulo
// the auth.Permission wrapper type.
type authPermissionChecker struct{}

func (authPermissionChecker) HasPermission(role, permission string) bool {
	return auth.HasPermission(role, auth.Permission(permission))
}

type xoluDashboardResolver struct {
	h *UIHandler
}

func (r *xoluDashboardResolver) Count(ctx context.Context, s dashboardengine.EntitySource) (int, error) {
	q := "SELECT COUNT(*) as c FROM " + s.Entity
	if s.Filter != "" {
		q += " WHERE " + s.Filter
	}
	res, err := r.h.xoluClient.OQL(ctx, q)
	if err != nil || res == nil || len(res.Data) == 0 {
		return 0, err
	}
	return toInt(res.Data[0]["c"]), nil
}

func (r *xoluDashboardResolver) GroupCount(ctx context.Context, s dashboardengine.EntitySource) ([]dashboardengine.GroupCountRow, error) {
	q := fmt.Sprintf("SELECT %s, COUNT(*) as count FROM %s", s.GroupBy, s.Entity)
	if s.Filter != "" {
		q += " WHERE " + s.Filter
	}
	q += " GROUP BY " + s.GroupBy
	res, err := r.h.xoluClient.OQL(ctx, q)
	if err != nil || res == nil {
		return nil, err
	}
	rows := make([]dashboardengine.GroupCountRow, 0, len(res.Data))
	for _, row := range res.Data {
		rows = append(rows, dashboardengine.GroupCountRow{
			Value: toString(row[s.GroupBy]),
			Count: toInt(row["count"]),
		})
	}
	return rows, nil
}

func (r *xoluDashboardResolver) Rows(ctx context.Context, s dashboardengine.EntitySource) ([]dashboardengine.Row, error) {
	if s.Type == "audit_log" {
		return r.auditLogRows(ctx, s)
	}
	q := "SELECT"
	if s.Limit > 0 {
		q += fmt.Sprintf(" TOP %d", s.Limit)
	}
	q += " * FROM " + s.Entity
	if s.Filter != "" {
		q += " WHERE " + s.Filter
	}
	if s.Sort != "" {
		field, desc := strings.CutPrefix(s.Sort, "-")
		if desc {
			q += " ORDER BY " + field + " DESC"
		} else {
			q += " ORDER BY " + s.Sort
		}
	}
	res, err := r.h.xoluClient.OQL(ctx, q)
	if err != nil || res == nil {
		return nil, err
	}
	rows := make([]dashboardengine.Row, len(res.Data))
	for i, d := range res.Data {
		row := make(dashboardengine.Row, len(d))
		for k, v := range d {
			row[k] = v
		}
		rows[i] = row
	}
	for _, rule := range s.Enrich {
		if err := r.applyEnrich(ctx, rows, rule); err != nil {
			return nil, err
		}
	}
	return rows, nil
}

// applyEnrich implements DASHBOARD_SPEC.md §5.1's enrich mechanism: fetch
// every row of rule.From once, build an id-keyed map, merge rule.Fields
// into each main row by looking up row[rule.On] in that map. Exactly the
// pattern the pre-migration Dashboard handler already hand-wrote for
// alertaLookup/assetNames -- now data-driven instead of duplicated per
// widget.
func (r *xoluDashboardResolver) applyEnrich(ctx context.Context, rows []dashboardengine.Row, rule dashboardengine.EnrichRule) error {
	res, err := r.h.xoluClient.OQL(ctx, "SELECT * FROM "+rule.From)
	if err != nil || res == nil {
		return err
	}
	lookup := make(map[int64]map[string]any, len(res.Data))
	for _, d := range res.Data {
		lookup[toInt64(d["id"])] = d
	}
	for _, row := range rows {
		fk := toInt64(row[rule.On])
		src, ok := lookup[fk]
		if !ok {
			continue
		}
		for from, to := range rule.Fields {
			row[to] = src[from]
		}
	}
	return nil
}

// UrgencyRows ports loadVencimientos (masters_routes.go) exactly --
// same query shape, same 30-day horizon, same ascending sort, same cap --
// now generic over EntitySource.DateFields instead of the two hardcoded
// fecha_expiracion/fin_garantia columns.
func (r *xoluDashboardResolver) UrgencyRows(ctx context.Context, s dashboardengine.EntitySource) ([]dashboardengine.UrgencyRow, error) {
	fields := append([]string{"id", "name"}, s.DateFields...)
	conds := make([]string, len(s.DateFields))
	for i, f := range s.DateFields {
		conds[i] = f + " != ''"
	}
	q := fmt.Sprintf("SELECT %s FROM %s WHERE %s", strings.Join(fields, ", "), s.Entity, strings.Join(conds, " OR "))
	res, err := r.h.xoluClient.OQL(ctx, q)
	if err != nil || res == nil {
		return nil, err
	}

	now := time.Now()
	withinDays := s.WithinDays
	if withinDays == 0 {
		withinDays = 30
	}
	horizon := now.AddDate(0, 0, withinDays)
	const layout = "2006-01-02"

	var out []dashboardengine.UrgencyRow
	for _, row := range res.Data {
		id := toInt64(row["id"])
		name := toString(row["name"])
		for _, field := range s.DateFields {
			raw := toString(row[field])
			if raw == "" {
				continue
			}
			t, err := time.Parse(layout, raw[:min(10, len(raw))])
			if err != nil {
				continue
			}
			if t.After(now) && !t.After(horizon) {
				out = append(out, dashboardengine.UrgencyRow{
					EntityID:   id,
					EntityName: name,
					FieldName:  dateFieldLabel(s, field),
					Date:       t.Format("02/01/2006"),
					DaysLeft:   int(t.Sub(now).Hours() / 24),
				})
			}
		}
	}

	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].DaysLeft < out[j-1].DaysLeft; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	if s.Limit > 0 && len(out) > s.Limit {
		out = out[:s.Limit]
	}
	return out, nil
}

// dateFieldLabel resolves an EntitySource.DateFieldLabels mapping, falling
// back to the raw field name when the field has no explicit label.
func dateFieldLabel(s dashboardengine.EntitySource, field string) string {
	if label, ok := s.DateFieldLabels[field]; ok {
		return label
	}
	return field
}

// auditLogRows resolves a source.type: "audit_log" (DASHBOARD_SPEC.md
// §5.2) via h.auditLogger.List (added alongside this, MOD-3 part 1 --
// nothing queried audit events before). scope: "tenant" relies entirely
// on h.xoluClient already being tenant-scoped (confirmed throughout this
// codebase: no handler anywhere manually filters by tenant_id, the xolu
// client itself does it) -- only scope: "self" adds an explicit UserID
// filter on top of that.
func (r *xoluDashboardResolver) auditLogRows(ctx context.Context, s dashboardengine.EntitySource) ([]dashboardengine.Row, error) {
	filter := auth.AuditFilter{
		Resource: s.Resource,
		Limit:    s.Limit,
	}
	if s.Scope == "self" {
		filter.UserID = s.CurrentUserID
	}
	for _, et := range s.EventTypes {
		filter.EventTypes = append(filter.EventTypes, auth.AuditEventType(et))
	}

	events, err := r.h.auditLogger.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	rows := make([]dashboardengine.Row, len(events))
	for i, ev := range events {
		rows[i] = dashboardengine.Row{
			"id":         ev.ID,
			"event_type": string(ev.EventType),
			"email":      ev.Email,
			"resource":   ev.Resource,
			"action":     ev.Action,
			"details":    ev.Details,
			"timestamp":  ev.Timestamp.Format("02/01/2006 15:04"),
		}
	}
	return rows, nil
}
