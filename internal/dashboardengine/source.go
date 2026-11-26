package dashboardengine

import "context"

// This file defines the "source" side of a widget (DASHBOARD_SPEC.md §5):
// the EntitySource type each widget's source unmarshals into, and the
// EntityResolver interface a caller implements against a real backend
// (xolu, in this project's case — implemented in internal/ui, not here,
// same decoupling as formengine.SourceResolver never importing xolu
// directly).

// EntitySource is source.type: "entity" (§5.1). "audit_log" (§5.2) is not
// in MOD-2's scope -- see MOD-3.
type EntitySource struct {
	Type string `json:"type"` // "entity" (§5.1) or "audit_log" (§5.2, MOD-3)

	Entity     string   `json:"entity,omitempty"`
	Aggregate  string   `json:"aggregate,omitempty"` // "" (rows) | "count" | "group_count"
	GroupBy    string   `json:"group_by,omitempty"`
	Filter     string   `json:"filter,omitempty"`
	Sort       string   `json:"sort,omitempty"`
	Limit      int      `json:"limit,omitempty"`
	DateFields []string `json:"date_fields,omitempty"` // urgency_list only
	WithinDays int      `json:"within_days,omitempty"` // urgency_list only

	// DateFieldLabels maps a date_fields entry to a stable semantic label
	// used for its i18n key ("dashboard.field."+label), decoupled from the
	// entity's own raw field name -- found necessary during live
	// verification: the pre-migration code labelled its two date checks
	// "expiration"/"warranty_end" (dashboard.field.expiration/warranty_end,
	// already translated in every locale file), never the raw xolu column
	// names this source actually queried at the time
	// ("fecha_expiracion"/"fin_garantia"). Using the raw field name
	// directly as the i18n key would have worked then but churned the
	// moment T-40's Asset rename landed and those columns went English --
	// this mapping is what avoided that. Confirmed 2026-08-11: T-40 Block
	// E renamed fecha_expiracion to expiration_date, Block G renamed
	// fin_garantia to warranty_end; assets-dashboard.json's own
	// date_fields/date_field_labels entries were updated to match both
	// times, with zero code changes needed in this file either time --
	// this mapping is what made that possible.
	// is what avoids that. A date field with no entry here falls back to
	// its own raw name as the label (dashboard_resolver.go).
	DateFieldLabels map[string]string `json:"date_field_labels,omitempty"`

	// Enrich, added in DASHBOARD_SPEC.md v0.4: secondary lookups merged
	// into each row after the main query resolves. Found necessary
	// implementing this package against the real Assets dashboard's
	// "recent alerts" widget, which needs three entities and xolu's OQL
	// only supports two-table joins -- see §5.1's own note.
	Enrich []EnrichRule `json:"enrich,omitempty"`

	// Resource, EventTypes, Scope: source.type: "audit_log" only
	// (DASHBOARD_SPEC.md §5.2). Resource matches AuditEvent.Resource
	// ("settings", "user", "profile"); EventTypes, if present, restricts
	// to those event types; Scope is required for this source type --
	// "tenant" (every tenant event -- xolu's own client is already
	// tenant-scoped, so no explicit tenant filtering is needed beyond
	// that) or "self" (further filtered to CurrentUserID).
	Resource   string   `json:"resource,omitempty"`
	EventTypes []string `json:"event_types,omitempty"`
	Scope      string   `json:"scope,omitempty"`

	// CurrentUserID is not part of the JSON schema (json:"-") -- it's
	// populated by the engine itself from RenderContext.UserID right
	// before a source resolves, needed only when Scope == "self". Every
	// widget's source gets this set uniformly rather than only audit_log
	// ones, since an entity source simply never reads it.
	CurrentUserID int64 `json:"-"`
}

// EnrichRule is one entry of EntitySource.Enrich (§5.1).
type EnrichRule struct {
	On     string            `json:"on"`
	From   string            `json:"from"`
	Fields map[string]string `json:"fields"`
}

// Row is one resolved row for a rows-mode source (recent_list,
// urgency_list's own per-field-match rows before threshold evaluation).
// A generic map, not a typed struct, since the fields it carries depend
// entirely on the entity and enrich rules a given widget's source names
// -- this package has no fixed notion of what an "asset" or "alert" row
// looks like, deliberately, same reasoning formengine's own values maps
// stay map[string]interface{} rather than typed per schema.
type Row map[string]any

// GroupCountRow is one bucket of an aggregate: "group_count" source.
type GroupCountRow struct {
	Value string
	Count int
}

// UrgencyRow is one resolved row of a urgency_list source, already
// reduced to "this field, on this entity instance, is due in N days" --
// the shape DashVencimientoRow already had before this migration, kept
// unchanged rather than redesigned, since it's proven against the real
// pre-migration widget.
type UrgencyRow struct {
	EntityID   int64
	EntityName string
	FieldName  string // dashboard_resolver.go's dateFieldLabel(source, rawField) -- source.DateFieldLabels[rawField], falling back to rawField itself; widgets.go looks this up as "dashboard.field."+FieldName
	Date       string // pre-formatted for display (dd/mm/yyyy, matching the pre-migration code's own format)
	DaysLeft   int
}

// EntityResolver resolves an EntitySource to real data. The xolu-backed
// implementation is supplied by the caller via DashboardEngine.Resolver;
// this package has no direct xolu dependency.
type EntityResolver interface {
	// Count resolves aggregate: "count" sources (stat_card, total_card).
	Count(ctx context.Context, s EntitySource) (int, error)

	// GroupCount resolves aggregate: "group_count" sources (breakdown_list).
	GroupCount(ctx context.Context, s EntitySource) ([]GroupCountRow, error)

	// Rows resolves a rows-mode source (recent_list) -- the main query,
	// filtered/sorted/limited, then each Enrich rule's lookup merged in.
	Rows(ctx context.Context, s EntitySource) ([]Row, error)

	// UrgencyRows resolves a urgency_list source -- scans DateFields
	// across every matching entity instance, keeps only rows due within
	// WithinDays, sorted soonest-first, capped at Limit. A distinct
	// method from Rows rather than a special case of it: the per-field
	// date-scanning and day-counting logic doesn't generalise to any
	// other widget's needs, same reasoning DASHBOARD_SPEC.md §5.1 gives
	// for date_fields/within_days being exclusive to this widget type.
	UrgencyRows(ctx context.Context, s EntitySource) ([]UrgencyRow, error)
}
