package dashboardengine

// This file defines the Go struct shape of an "x-seam-dashboard" JSON
// document (DASHBOARD_SPEC.md §2-§6). Every struct field maps directly to
// one JSON property of the same name; the section numbers below cite
// where each property is specified.

// Dashboard is the root x-seam-dashboard object (§2.1).
type Dashboard struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Layout   string    `json:"layout"`
	Sections []Section `json:"sections"`
}

// Section is one row of the dashboard (§3).
type Section struct {
	ID      string   `json:"id"`
	Grid    string   `json:"grid"`
	Widgets []Widget `json:"widgets"`

	// VisibleWhen implements §7.2's permission-based visibility only
	// (MOD-3) -- field-value visibility (§7.1) reuses internal/condition
	// the same way x-seam-visible-when already does, but DASHBOARD_
	// SPEC.md itself notes that's "not identified as needed today," so
	// it stays unimplemented until a real case appears, matching this
	// project's own discipline of not building the speculative half of
	// a spec just because the other half became real.
	VisibleWhen *VisibleWhen `json:"visible_when,omitempty"`
}

// VisibleWhen is §7.2's permission-condition form:
// {"visible_when": {"permission": "system.settings.edit"}}. The
// field/operator/value form (§7.1) is a different, unimplemented shape --
// see Section.VisibleWhen's own comment for why.
type VisibleWhen struct {
	Permission string `json:"permission"`
}

// Widget covers all six §4 widget types in one struct (Go has no clean
// sum-type equivalent for JSON without real cost); Type selects which
// fields renderer.go actually reads for a given instance. Comments below
// group fields by which widget type(s) use them.
type Widget struct {
	Type string `json:"type"` // "stat_card" | "recent_list" | "urgency_list" | "total_card" | "breakdown_list" (§4.1-4.5; "line_chart", §4.6, not in MOD-2)

	// stat_card (§4.1), total_card (§4.4)
	Label         string `json:"label,omitempty"`
	Icon          string `json:"icon,omitempty"`
	Href          string `json:"href,omitempty"`
	Urgent        bool   `json:"urgent,omitempty"`
	ViewLinkLabel string `json:"view_link_label,omitempty"`

	// recent_list (§4.2), urgency_list (§4.3), breakdown_list (§4.5)
	Title        string `json:"title,omitempty"`
	Subtitle     string `json:"subtitle,omitempty"`
	ViewAllHref  string `json:"view_all_href,omitempty"`
	EmptyIcon    string `json:"empty_icon,omitempty"`
	EmptyMessage string `json:"empty_message,omitempty"`
	ItemIcon     string `json:"item_icon,omitempty"`

	// recent_list, urgency_list -- the href each individual row links to
	// is item_href_prefix + "/" + the row's id, deliberately distinct
	// from view_all_href: the original dashRecentAlerts linked its
	// header's "view all" to "/alerts?status=open" but each row to
	// "/alerts/{id}" -- two different base paths, not the same value
	// reused. Conflating them was a real bug caught before this shipped,
	// not a hypothetical -- worth recording here so the distinction
	// isn't lost to a future edit.
	ItemHrefPrefix string `json:"item_href_prefix,omitempty"`

	// recent_list only
	PrimaryField           string            `json:"primary_field,omitempty"`
	SecondaryField         string            `json:"secondary_field,omitempty"`
	SecondaryFallbackField string            `json:"secondary_fallback_field,omitempty"`
	TimestampField         string            `json:"timestamp_field,omitempty"`
	BadgeField             string            `json:"badge_field,omitempty"`
	BadgeColors            map[string]string `json:"badge_colors,omitempty"`

	// urgency_list only. Row field names (date, field_name, entity_id,
	// entity_name) are fixed by UrgencyRow's own shape (source.go), not
	// configurable per widget -- DASHBOARD_SPEC.md §4.3 originally listed
	// date_field/label_field as widget properties for this; dropped here,
	// unused by any actual rendering code once built, since the compiled
	// row shape from urgencyRowsToMaps (compile.go) never varied per
	// widget instance to begin with.
	Thresholds []Threshold `json:"thresholds,omitempty"`

	// breakdown_list only. Compiled rows always use "value"/"count"
	// (compile.go's groupCountRowsToMaps), matching GroupCountRow's fixed
	// shape -- a widget-level "group_field" naming which row field to use
	// would be redundant with source.group_by, same vestigial-property
	// class as urgency_list's removed date_field/label_field (§4.3's own
	// note). Never added here to begin with, once that pattern was caught.
	Labels map[string]string `json:"labels,omitempty"`
	Colors map[string]string `json:"colors,omitempty"`

	Source EntitySource `json:"source"`
}

// Threshold is one urgency_list day-count bracket (§4.3). MaxDays is a
// pointer so JSON `null` (meaning "unbounded, the catch-all bracket") is
// distinguishable from an absent/zero value.
type Threshold struct {
	MaxDays *int   `json:"max_days"`
	Class   string `json:"class"`
	Label   string `json:"label"`
}
