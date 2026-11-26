// Package dashboardengine renders dashboards from JSON documents extended
// with a single "x-seam-dashboard" root property (docs/DASHBOARD_SPEC.md
// v0.4).
//
// The schema is the only source of truth for a dashboard's sections,
// widgets, and data sources — adding a widget or changing a data source's
// filter is a configuration change, not a code change or redeploy. Same
// motivation as internal/formengine for forms (docs/FORM_ENGINE_SPEC.md),
// deliberately mirrored: this package has no direct xolu dependency
// either — the caller supplies a xolu-backed EntityResolver (source.go),
// same decoupling as formengine's SourceResolver.
//
// Package layout, mirroring formengine's own (engine.go's own doc comment):
//
//	engine.go   — entry point: DashboardEngine, RenderContext, Render
//	schema.go   — JSON document types and parsing
//	source.go   — EntityResolver interface and the data-source types it's
//	              called with (EntitySource, EnrichRule, Row, GroupCountRow)
//	widgets.go  — individual widget implementations (stat_card, recent_list,
//	              urgency_list, total_card, breakdown_list)
//	renderer.go — per-section/per-widget rendering orchestration
//
// MOD-2 scope only (MODULE_DASHBOARD_DEVELOPMENT_PLAN.md): the five
// dashboard.go-derived widget types and the "entity" source type. Not yet
// "audit_log" (MOD-3) or "line_chart" (MOD-9, deferred) — DASHBOARD_
// SPEC.md's own §5.2/§4.6, not part of this package until those stages.
package dashboardengine

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	mi "github.com/ha1tch/minty"
)

// Translator resolves one i18n key for one locale. Deliberately the same
// shape as formengine.Translator and internal/i18n.Translator's own T
// method, so a caller's existing translator already satisfies this
// interface structurally.
type Translator interface {
	T(locale, key string, args ...any) string
}

// PermissionChecker resolves permission-based section visibility (§7.2,
// MOD-3) -- deliberately the same shape as auth.HasPermission's own
// signature, so a thin wrapper around that function is all any caller
// needs to satisfy this interface. This package has no auth dependency
// itself, same decoupling as EntityResolver keeping xolu out and
// Translator keeping internal/i18n out.
type PermissionChecker interface {
	HasPermission(role, permission string) bool
}

// RenderContext carries what a dashboard render needs beyond the schema
// itself: the viewer's locale (for Translator), their role (for
// PermissionChecker/visible_when, §7.2, MOD-3), and their user ID (for
// audit_log sources with scope: "self", §5.2, MOD-3).
type RenderContext struct {
	Locale string
	Role   string
	UserID int64
}

// DashboardEngine renders a dashboard from its JSON document.
type DashboardEngine struct {
	// Resolver resolves each widget's "source" to real data. Required —
	// unlike formengine.FormEngine.Sources (which may be nil if no field
	// uses x-seam-source), every dashboard widget has a source, so a nil
	// Resolver means Render returns an error node for the first widget it
	// tries to draw rather than panicking.
	Resolver EntityResolver

	// Translator resolves i18n keys. May be nil — every resolution path
	// falls back to the schema's own literal key text when Translator is
	// nil or a key doesn't resolve, same fallback formengine already
	// established.
	Translator Translator

	// CacheTTL, if positive, caches each widget's compiled data (compile.go)
	// for this long, keyed by its own EntitySource -- so two widgets with
	// an identical source share one cache entry, and repeated requests
	// within the TTL skip Resolver entirely for that widget. Zero (the
	// default) disables caching -- every render resolves fresh, same
	// behaviour as before this field existed. A few seconds is enough to
	// absorb a burst of concurrent page loads without the dashboard ever
	// showing data more than a few seconds stale.
	CacheTTL time.Duration

	// PermissionChecker resolves permission-based section visibility
	// (§7.2, MOD-3). May be nil -- a section with visible_when set but no
	// PermissionChecker configured is treated as NOT visible (fails
	// closed). This is deliberately the opposite direction from
	// Translator's nil fallback (which shows the raw key -- a cosmetic
	// miss, not a security one): visible_when exists specifically to
	// hide permission-gated content, so a misconfigured checker hiding
	// too much is the safe failure mode, not one that silently exposes
	// admin-only sections to everyone.
	PermissionChecker PermissionChecker

	cacheOnce sync.Once
	cache     *widgetCache
}

func (e *DashboardEngine) widgetCacheInstance() *widgetCache {
	e.cacheOnce.Do(func() {
		e.cache = newWidgetCache(e.CacheTTL)
	})
	return e.cache
}

// Render parses schema (a JSON document with a root "x-seam-dashboard"
// property) and renders it. Returns an error node, not a panic or a Go
// error, if the schema fails to parse or a widget's source fails to
// resolve — a broken dashboard definition should degrade visibly on the
// page, not take the whole page down, same philosophy formengine.Render
// already established for a broken form schema.
func (e *DashboardEngine) Render(ctx context.Context, schema []byte, rc RenderContext) mi.H {
	doc, err := parseDashboard(schema)
	if err != nil {
		return errorNode(fmt.Sprintf("dashboard schema error: %v", err))
	}
	return e.renderDashboard(ctx, doc, rc)
}

func (e *DashboardEngine) t(rc RenderContext, key string, args ...any) string {
	if e.Translator == nil {
		return key
	}
	return e.Translator.T(rc.Locale, key, args...)
}

func (e *DashboardEngine) resolveCount(ctx context.Context, s EntitySource) (int, error) {
	if e.Resolver == nil {
		return 0, errNoResolver()
	}
	return e.Resolver.Count(ctx, s)
}

func (e *DashboardEngine) resolveGroupCount(ctx context.Context, s EntitySource) ([]GroupCountRow, error) {
	if e.Resolver == nil {
		return nil, errNoResolver()
	}
	return e.Resolver.GroupCount(ctx, s)
}

func (e *DashboardEngine) resolveRows(ctx context.Context, s EntitySource) ([]Row, error) {
	if e.Resolver == nil {
		return nil, errNoResolver()
	}
	return e.Resolver.Rows(ctx, s)
}

func (e *DashboardEngine) resolveUrgencyRows(ctx context.Context, s EntitySource) ([]UrgencyRow, error) {
	if e.Resolver == nil {
		return nil, errNoResolver()
	}
	return e.Resolver.UrgencyRows(ctx, s)
}

func errNoResolver() error {
	return fmt.Errorf("dashboardengine: no EntityResolver configured")
}

func errUnknownWidgetType(t string) error {
	return fmt.Errorf("dashboardengine: unknown widget type %q", t)
}

func errorNode(msg string) mi.H {
	return func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("p-4 text-sm text-red-600 dark:text-red-400"), msg)
	}
}

// parseDashboard is a thin wrapper over encoding/json -- unlike
// formengine's schema.go, a dashboard document has no x-seam-* extension
// properties layered onto a third-party JSON Schema base; it's Seam's own
// flat format end to end, so standard struct-tag unmarshaling is enough
// (schema.go).
func parseDashboard(raw []byte) (*Dashboard, error) {
	var wrapper struct {
		XSeamDashboard Dashboard `json:"x-seam-dashboard"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, err
	}
	return &wrapper.XSeamDashboard, nil
}
