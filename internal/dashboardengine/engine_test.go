package dashboardengine

import (
	"context"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"
)

// fakeResolver is a fixed-answer EntityResolver for tests, the same role
// formengine.StaticSourceResolver plays for form-engine tests — not meant
// for production use.
type fakeResolver struct {
	counts      map[string]int
	groupCounts map[string][]GroupCountRow
	rows        map[string][]Row
	urgencyRows map[string][]UrgencyRow
}

func (f *fakeResolver) Count(ctx context.Context, s EntitySource) (int, error) {
	return f.counts[s.Entity], nil
}
func (f *fakeResolver) GroupCount(ctx context.Context, s EntitySource) ([]GroupCountRow, error) {
	return f.groupCounts[s.Entity], nil
}
func (f *fakeResolver) Rows(ctx context.Context, s EntitySource) ([]Row, error) {
	return f.rows[s.Entity], nil
}
func (f *fakeResolver) UrgencyRows(ctx context.Context, s EntitySource) ([]UrgencyRow, error) {
	return f.urgencyRows[s.Entity], nil
}

func renderToString(t *testing.T, e *DashboardEngine, schema []byte) string {
	t.Helper()
	h := e.Render(context.Background(), schema, RenderContext{Locale: "en"})
	return mi.RenderToString(h)
}

func renderToStringWithContext(t *testing.T, e *DashboardEngine, schema []byte, rc RenderContext) string {
	t.Helper()
	h := e.Render(context.Background(), schema, rc)
	return mi.RenderToString(h)
}

func TestRender_StatCard(t *testing.T) {
	schema := []byte(`{
	  "x-seam-dashboard": {
	    "id": "test", "title": "t", "layout": "stack",
	    "sections": [{"id": "s", "grid": "1col", "widgets": [
	      {"type": "stat_card", "label": "Open Alerts", "icon": "warning", "href": "/alerts", "urgent": true,
	       "source": {"type": "entity", "entity": "alerts", "aggregate": "count"}}
	    ]}]
	  }
	}`)
	e := &DashboardEngine{Resolver: &fakeResolver{counts: map[string]int{"alerts": 3}}}
	out := renderToString(t, e, schema)
	if !strings.Contains(out, "Open Alerts") || !strings.Contains(out, ">3<") {
		t.Errorf("output missing expected label/count: %s", out)
	}
	// urgent + count>0 should apply the red border/text classes
	if !strings.Contains(out, "text-red-600") {
		t.Errorf("urgent stat_card with count>0 should use red text class: %s", out)
	}
}

func TestRender_RecentList(t *testing.T) {
	schema := []byte(`{
	  "x-seam-dashboard": {
	    "id": "test", "title": "t", "layout": "stack",
	    "sections": [{"id": "s", "grid": "1col", "widgets": [
	      {"type": "recent_list", "title": "Recent", "empty_icon": "check", "empty_message": "None",
	       "item_icon": "warning", "primary_field": "asset_name", "secondary_field": "title",
	       "timestamp_field": "created_at", "badge_field": "criticality",
	       "badge_colors": {"High": "danger"},
	       "source": {"type": "entity", "entity": "alerts"}}
	    ]}]
	  }
	}`)
	e := &DashboardEngine{Resolver: &fakeResolver{rows: map[string][]Row{
		"alerts": {{"id": int64(1), "asset_name": "Chiller 5", "title": "Temp exceeded", "criticality": "High", "created_at": "2026-08-02"}},
	}}}
	out := renderToString(t, e, schema)
	for _, want := range []string{"Chiller 5", "Temp exceeded", "2026-08-02", "High"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %s", want, out)
		}
	}
}

func TestRender_RecentList_Empty(t *testing.T) {
	schema := []byte(`{
	  "x-seam-dashboard": {
	    "id": "test", "title": "t", "layout": "stack",
	    "sections": [{"id": "s", "grid": "1col", "widgets": [
	      {"type": "recent_list", "title": "Recent", "empty_icon": "check", "empty_message": "Nothing here",
	       "source": {"type": "entity", "entity": "alerts"}}
	    ]}]
	  }
	}`)
	e := &DashboardEngine{Resolver: &fakeResolver{}}
	out := renderToString(t, e, schema)
	if !strings.Contains(out, "Nothing here") {
		t.Errorf("expected empty-state message in output: %s", out)
	}
}

func TestRender_BreakdownList(t *testing.T) {
	schema := []byte(`{
	  "x-seam-dashboard": {
	    "id": "test", "title": "t", "layout": "stack",
	    "sections": [{"id": "s", "grid": "1col", "widgets": [
	      {"type": "breakdown_list", "title": "By Status", "empty_message": "none",
	       "labels": {"active": "Active"}, "colors": {"active": "green-500"},
	       "source": {"type": "entity", "entity": "assets", "aggregate": "group_count", "group_by": "status"}}
	    ]}]
	  }
	}`)
	e := &DashboardEngine{Resolver: &fakeResolver{groupCounts: map[string][]GroupCountRow{
		"assets": {{Value: "active", Count: 12}},
	}}}
	out := renderToString(t, e, schema)
	if !strings.Contains(out, "Active") || !strings.Contains(out, "bg-green-500") || !strings.Contains(out, ">12<") {
		t.Errorf("output missing expected breakdown content: %s", out)
	}
}

func TestRender_UrgencyList(t *testing.T) {
	maxDays7 := 7
	schema := []byte(`{
	  "x-seam-dashboard": {
	    "id": "test", "title": "t", "layout": "stack",
	    "sections": [{"id": "s", "grid": "1col", "widgets": [
	      {"type": "urgency_list", "title": "Upcoming", "subtitle": "next 30 days", "empty_message": "none",
	       "item_icon": "event",
	       "thresholds": [{"max_days": 7, "class": "urgent", "label": "dashboard.days_urgent"}, {"max_days": null, "class": "normal", "label": "dashboard.days"}],
	       "source": {"type": "entity", "entity": "assets", "date_fields": ["warranty_end"], "within_days": 30}}
	    ]}]
	  }
	}`)
	e := &DashboardEngine{Resolver: &fakeResolver{urgencyRows: map[string][]UrgencyRow{
		"assets": {{EntityID: 9, EntityName: "Forklift 3", FieldName: "warranty_end", Date: "10/08/2026", DaysLeft: 5}},
	}}}
	_ = maxDays7
	out := renderToString(t, e, schema)
	if !strings.Contains(out, "Forklift 3") || !strings.Contains(out, "10/08/2026") {
		t.Errorf("output missing expected urgency row content: %s", out)
	}
	if !strings.Contains(out, "text-red-600") {
		t.Errorf("a 5-day-left row should match the max_days:7 urgent threshold: %s", out)
	}
}

func TestRender_ItemHrefDistinctFromViewAllHref(t *testing.T) {
	// Regression test for a real bug: the header's "view all" link and
	// each row's own link are different base paths in the original
	// dashRecentAlerts ("/alerts?status=open" vs "/alerts/{id}") --
	// conflating them (both reading ViewAllHref) was caught and fixed
	// before shipping, not a hypothetical.
	schema := []byte(`{
	  "x-seam-dashboard": {
	    "id": "test", "title": "t", "layout": "stack",
	    "sections": [{"id": "s", "grid": "1col", "widgets": [
	      {"type": "recent_list", "title": "Recent", "empty_icon": "check", "empty_message": "None",
	       "view_all_href": "/alerts?status=open", "item_href_prefix": "/alerts",
	       "source": {"type": "entity", "entity": "alerts"}}
	    ]}]
	  }
	}`)
	e := &DashboardEngine{Resolver: &fakeResolver{rows: map[string][]Row{
		"alerts": {{"id": int64(5), "title": "x"}},
	}}}
	out := renderToString(t, e, schema)
	if !strings.Contains(out, `href="/alerts?status=open"`) {
		t.Errorf("expected the view-all link to use view_all_href verbatim: %s", out)
	}
	if !strings.Contains(out, `href="/alerts/5"`) {
		t.Errorf("expected the row's own link to be item_href_prefix + \"/\" + id, not view_all_href + \"/\" + id: %s", out)
	}
	if strings.Contains(out, `href="/alerts?status=open/5"`) {
		t.Error("row link incorrectly built from view_all_href -- the exact bug this test guards against")
	}
}

func TestRender_RecentListViewAllTranslated(t *testing.T) {
	schema := []byte(`{
	  "x-seam-dashboard": {
	    "id": "test", "title": "t", "layout": "stack",
	    "sections": [{"id": "s", "grid": "1col", "widgets": [
	      {"type": "recent_list", "title": "Recent", "empty_icon": "check", "empty_message": "None",
	       "view_all_href": "/alerts?status=open", "item_href_prefix": "/alerts",
	       "source": {"type": "entity", "entity": "alerts"}}
	    ]}]
	  }
	}`)
	e := &DashboardEngine{
		Resolver: &fakeResolver{rows: map[string][]Row{
			"alerts": {{"id": int64(1), "title": "x"}},
		}},
		Translator: &testT{messages: map[string]string{
			"dashboard.view_all": "Ver todas →",
		}},
	}
	out := renderToString(t, e, schema)
	if !strings.Contains(out, "Ver todas →") {
		t.Errorf("expected view-all link to be translated to 'Ver todas →', got: %s", out)
	}
}

type testT struct {
	messages map[string]string
}

func (t *testT) T(locale, key string, args ...any) string {
	if v, ok := t.messages[key]; ok {
		return v
	}
	return key
}

type fakePermissionChecker struct {
	granted map[string]bool // "role:permission" -> granted
}

func (f *fakePermissionChecker) HasPermission(role, permission string) bool {
	return f.granted[role+":"+permission]
}

func TestSectionVisible_NoVisibleWhen_AlwaysVisible(t *testing.T) {
	schema := []byte(`{
	  "x-seam-dashboard": {
	    "id": "test", "title": "t", "layout": "stack",
	    "sections": [{"id": "s", "grid": "1col", "widgets": [
	      {"type": "total_card", "label": "X", "source": {"type": "entity", "entity": "assets", "aggregate": "count"}}
	    ]}]
	  }
	}`)
	e := &DashboardEngine{Resolver: &fakeResolver{counts: map[string]int{"assets": 5}}}
	out := renderToString(t, e, schema)
	if !strings.Contains(out, ">5<") {
		t.Errorf("a section with no visible_when should always render: %s", out)
	}
}

func TestSectionVisible_PermissionGranted(t *testing.T) {
	schema := []byte(`{
	  "x-seam-dashboard": {
	    "id": "test", "title": "t", "layout": "stack",
	    "sections": [{"id": "s", "grid": "1col", "visible_when": {"permission": "system.settings.edit"}, "widgets": [
	      {"type": "total_card", "label": "Secret", "source": {"type": "entity", "entity": "assets", "aggregate": "count"}}
	    ]}]
	  }
	}`)
	e := &DashboardEngine{
		Resolver:          &fakeResolver{counts: map[string]int{"assets": 5}},
		PermissionChecker: &fakePermissionChecker{granted: map[string]bool{"admin:system.settings.edit": true}},
	}
	out := renderToStringWithContext(t, e, schema, RenderContext{Locale: "en", Role: "admin"})
	if !strings.Contains(out, "Secret") {
		t.Errorf("section should render when the role holds the required permission: %s", out)
	}
}

func TestSectionVisible_PermissionDenied(t *testing.T) {
	schema := []byte(`{
	  "x-seam-dashboard": {
	    "id": "test", "title": "t", "layout": "stack",
	    "sections": [{"id": "s", "grid": "1col", "visible_when": {"permission": "system.settings.edit"}, "widgets": [
	      {"type": "total_card", "label": "Secret", "source": {"type": "entity", "entity": "assets", "aggregate": "count"}}
	    ]}]
	  }
	}`)
	e := &DashboardEngine{
		Resolver:          &fakeResolver{counts: map[string]int{"assets": 5}},
		PermissionChecker: &fakePermissionChecker{granted: map[string]bool{}}, // viewer holds nothing
	}
	out := renderToStringWithContext(t, e, schema, RenderContext{Locale: "en", Role: "viewer"})
	if strings.Contains(out, "Secret") {
		t.Errorf("section should NOT render when the role lacks the required permission: %s", out)
	}
}

func TestSectionVisible_FailsClosedWhenNoPermissionChecker(t *testing.T) {
	schema := []byte(`{
	  "x-seam-dashboard": {
	    "id": "test", "title": "t", "layout": "stack",
	    "sections": [{"id": "s", "grid": "1col", "visible_when": {"permission": "system.settings.edit"}, "widgets": [
	      {"type": "total_card", "label": "Secret", "source": {"type": "entity", "entity": "assets", "aggregate": "count"}}
	    ]}]
	  }
	}`)
	// No PermissionChecker configured at all -- must fail closed (hide),
	// not open (show), per DashboardEngine.PermissionChecker's own doc
	// comment: a misconfigured checker must never expose gated content.
	e := &DashboardEngine{Resolver: &fakeResolver{counts: map[string]int{"assets": 5}}}
	out := renderToString(t, e, schema)
	if strings.Contains(out, "Secret") {
		t.Errorf("a gated section with no PermissionChecker configured must fail closed: %s", out)
	}
}

func TestSectionVisible_MultipleSections_OnlyGatedOneHidden(t *testing.T) {
	schema := []byte(`{
	  "x-seam-dashboard": {
	    "id": "test", "title": "t", "layout": "stack",
	    "sections": [
	      {"id": "open", "grid": "1col", "widgets": [
	        {"type": "total_card", "label": "AlwaysVisible", "source": {"type": "entity", "entity": "assets", "aggregate": "count"}}
	      ]},
	      {"id": "gated", "grid": "1col", "visible_when": {"permission": "system.user.view"}, "widgets": [
	        {"type": "total_card", "label": "AdminOnly", "source": {"type": "entity", "entity": "assets", "aggregate": "count"}}
	      ]}
	    ]
	  }
	}`)
	e := &DashboardEngine{
		Resolver:          &fakeResolver{counts: map[string]int{"assets": 5}},
		PermissionChecker: &fakePermissionChecker{granted: map[string]bool{}},
	}
	out := renderToStringWithContext(t, e, schema, RenderContext{Locale: "en", Role: "viewer"})
	if !strings.Contains(out, "AlwaysVisible") {
		t.Error("the ungated section should still render")
	}
	if strings.Contains(out, "AdminOnly") {
		t.Error("the gated section should be hidden, and it shouldn't affect the ungated one")
	}
}

func TestRender_TotalCard(t *testing.T) {
	schema := []byte(`{
	  "x-seam-dashboard": {
	    "id": "test", "title": "t", "layout": "stack",
	    "sections": [{"id": "s", "grid": "1col", "widgets": [
	      {"type": "total_card", "label": "Total Assets", "icon": "inventory", "href": "/assets", "view_link_label": "View",
	       "source": {"type": "entity", "entity": "assets", "aggregate": "count"}}
	    ]}]
	  }
	}`)
	e := &DashboardEngine{Resolver: &fakeResolver{counts: map[string]int{"assets": 42}}}
	out := renderToString(t, e, schema)
	if !strings.Contains(out, "Total Assets") || !strings.Contains(out, ">42<") {
		t.Errorf("output missing expected total_card content: %s", out)
	}
}

func TestRender_EmptyDashboard(t *testing.T) {
	schema := []byte(`{"x-seam-dashboard": {"id": "documents-dashboard", "title": "Documents", "layout": "stack", "sections": []}}`)
	e := &DashboardEngine{Resolver: &fakeResolver{}}
	out := renderToString(t, e, schema)
	if !strings.Contains(out, "Documents") {
		t.Errorf("empty dashboard should still show its title: %s", out)
	}
}

func TestRender_NilResolver_ErrorsGracefully(t *testing.T) {
	schema := []byte(`{
	  "x-seam-dashboard": {
	    "id": "test", "title": "t", "layout": "stack",
	    "sections": [{"id": "s", "grid": "1col", "widgets": [
	      {"type": "stat_card", "label": "X", "source": {"type": "entity", "entity": "assets", "aggregate": "count"}}
	    ]}]
	  }
	}`)
	e := &DashboardEngine{}             // no Resolver
	out := renderToString(t, e, schema) // must not panic
	if !strings.Contains(out, "no EntityResolver") {
		t.Errorf("expected a visible error node, got: %s", out)
	}
}

func TestRender_InvalidSchema_ErrorsGracefully(t *testing.T) {
	e := &DashboardEngine{Resolver: &fakeResolver{}}
	out := renderToString(t, e, []byte(`{not json`))
	if !strings.Contains(out, "dashboard schema error") {
		t.Errorf("expected a visible schema-error node, got: %s", out)
	}
}

func TestRender_CachingSkipsResolver(t *testing.T) {
	schema := []byte(`{
	  "x-seam-dashboard": {
	    "id": "test", "title": "t", "layout": "stack",
	    "sections": [{"id": "s", "grid": "1col", "widgets": [
	      {"type": "stat_card", "label": "X", "source": {"type": "entity", "entity": "assets", "aggregate": "count"}}
	    ]}]
	  }
	}`)
	r := &countingResolver{counts: map[string]int{"assets": 10}}
	e := &DashboardEngine{Resolver: r, CacheTTL: 1000000000} // 1s, generous for a fast test
	renderToString(t, e, schema)
	renderToString(t, e, schema)
	if r.calls != 1 {
		t.Errorf("Resolver.Count called %d times across two renders within the TTL, want 1 (second should be a cache hit)", r.calls)
	}
}

type countingResolver struct {
	fakeResolver
	calls  int
	counts map[string]int
}

func (r *countingResolver) Count(ctx context.Context, s EntitySource) (int, error) {
	r.calls++
	return r.counts[s.Entity], nil
}
