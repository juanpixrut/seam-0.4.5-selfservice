package dashboardengine

import (
	"context"

	mi "github.com/ha1tch/minty"
)

// This file walks a parsed Dashboard and renders it -- resolving each
// widget's source via DashboardEngine.Resolver, then dispatching to the
// matching widgets.go function. Mirrors DashboardPage's own top-level
// composition in internal/ui/dashboard.go exactly: one outer space-y-6
// div, each section either a grid wrapper (multi-widget rows) or the
// widget's own node placed directly (single-widget rows -- the
// pre-migration code never wrapped dashRecentAlerts/dashVencimientos in
// an extra div, and 1col sections don't either here).

func (e *DashboardEngine) renderDashboard(ctx context.Context, d *Dashboard, rc RenderContext) mi.H {
	return func(b *mi.Builder) mi.Node {
		if len(d.Sections) == 0 {
			return e.renderEmptyDashboard(b, d, rc)
		}
		children := make([]interface{}, 0, len(d.Sections)+1)
		children = append(children, b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), e.t(rc, d.Title)))
		for _, sec := range d.Sections {
			children = append(children, e.renderSection(ctx, b, sec, rc)...)
		}
		return b.Div(append([]interface{}{mi.Class("space-y-6")}, children...)...)
	}
}

// renderSection returns the section's own node(s) to append directly to
// the dashboard's child list -- a slice, not a single node, since a 1col
// section contributes its one widget's node bare (no wrapper), while
// 2col/3col sections contribute a single grid-wrapper div containing all
// their widgets.
func (e *DashboardEngine) renderSection(ctx context.Context, b *mi.Builder, sec Section, rc RenderContext) []interface{} {
	if !e.sectionVisible(sec, rc) {
		return nil
	}
	nodes := make([]interface{}, 0, len(sec.Widgets))
	for _, w := range sec.Widgets {
		nodes = append(nodes, e.renderWidget(ctx, b, w, rc))
	}
	switch sec.Grid {
	case "3col":
		return []interface{}{b.Div(append([]interface{}{mi.Class("grid grid-cols-1 md:grid-cols-3 gap-4")}, nodes...)...)}
	case "2col":
		return []interface{}{b.Div(append([]interface{}{mi.Class("grid grid-cols-1 md:grid-cols-2 gap-4 pt-4 border-t border-gray-200 dark:border-gray-700")}, nodes...)...)}
	default: // "1col" -- no wrapper, matching dashRecentAlerts/dashVencimientos's own un-wrapped placement pre-migration
		return nodes
	}
}

func (e *DashboardEngine) renderWidget(ctx context.Context, b *mi.Builder, w Widget, rc RenderContext) mi.Node {
	tw := e.translateWidget(w, rc)
	compiled, err := e.resolveAndCompile(ctx, w, rc)
	if err != nil {
		return errorWidget(b, err)
	}
	switch w.Type {
	case "stat_card":
		return statCardWidget(b, tw, compiled)
	case "total_card":
		return totalCardWidget(b, tw, compiled)
	case "breakdown_list":
		return breakdownListWidget(b, tw, compiled)
	case "recent_list":
		tf := func(key string, args ...any) string { return e.t(rc, key, args...) }
		return recentListWidget(b, tw, tf, compiled)
	case "urgency_list":
		tf := func(key string, args ...any) string { return e.t(rc, key, args...) }
		return urgencyListWidget(b, tw, tf, compiled)
	default:
		return errorWidget(b, errUnknownWidgetType(w.Type))
	}
}

// resolveAndCompile is the resolve -> cache -> compile pipeline every
// widget type goes through uniformly: check the cache (cache.go) first;
// on a miss, call the one EntityResolver method the widget's type needs,
// convert the typed result to plain data, compile it through that type's
// fixed nplate template (compile.go), and cache the compiled result
// before returning it. A cache hit skips Resolver entirely -- the only
// place in this package where that happens.
func (e *DashboardEngine) resolveAndCompile(ctx context.Context, w Widget, rc RenderContext) (map[string]any, error) {
	w.Source.CurrentUserID = rc.UserID // §5.2 scope: "self" -- harmless no-op for every other source type

	key := sourceCacheKey(w.Type, w.Source)
	if key != "" {
		if cached, ok := e.widgetCacheInstance().get(key); ok {
			return cached, nil
		}
	}

	var (
		compiled map[string]any
		err      error
	)
	switch w.Type {
	case "stat_card", "total_card":
		var count int
		count, err = e.resolveCount(ctx, w.Source)
		if err == nil {
			compiled, err = compileScalar(w.Type, count)
		}
	case "breakdown_list":
		var rows []GroupCountRow
		rows, err = e.resolveGroupCount(ctx, w.Source)
		if err == nil {
			compiled, err = compileRows(w.Type, groupCountRowsToMaps(rows))
		}
	case "recent_list":
		// recent_list is the only widget type an "entity" or "audit_log"
		// source can back (§5.2's own note) -- both resolve through the
		// same EntityResolver.Rows method; which query actually runs is
		// the resolver implementation's job (w.Source.Type), not this
		// package's.
		var rows []Row
		rows, err = e.resolveRows(ctx, w.Source)
		if err == nil {
			compiled, err = compileRows(w.Type, rowsToMaps(rows))
		}
	case "urgency_list":
		var rows []UrgencyRow
		rows, err = e.resolveUrgencyRows(ctx, w.Source)
		if err == nil {
			compiled, err = compileRows(w.Type, urgencyRowsToMaps(rows))
		}
	default:
		return nil, errUnknownWidgetType(w.Type)
	}
	if err != nil {
		return nil, err
	}

	if key != "" {
		e.widgetCacheInstance().set(key, compiled)
	}
	return compiled, nil
}

// sectionVisible implements §7.2: a section with no VisibleWhen is always
// visible. One with VisibleWhen.Permission is visible only if
// PermissionChecker confirms the viewer's role holds it -- fails closed
// (not visible) if PermissionChecker is nil, per DashboardEngine.
// PermissionChecker's own doc comment.
func (e *DashboardEngine) sectionVisible(sec Section, rc RenderContext) bool {
	if sec.VisibleWhen == nil {
		return true
	}
	if e.PermissionChecker == nil {
		return false
	}
	return e.PermissionChecker.HasPermission(rc.Role, sec.VisibleWhen.Permission)
}

// translateWidget resolves every i18n-key field on w through Translator
// once, up front, so widgets.go's rendering functions work with already-
// resolved display strings and stay free of translation concerns --
// same separation formengine's renderer.go keeps between resolving a
// field's label and rendering its widget.
func (e *DashboardEngine) translateWidget(w Widget, rc RenderContext) Widget {
	out := w
	out.Label = e.t(rc, w.Label)
	out.Title = e.t(rc, w.Title)
	out.Subtitle = e.t(rc, w.Subtitle)
	out.EmptyMessage = e.t(rc, w.EmptyMessage)
	out.ViewLinkLabel = e.t(rc, w.ViewLinkLabel)
	if w.Labels != nil {
		labels := make(map[string]string, len(w.Labels))
		for k, v := range w.Labels {
			labels[k] = e.t(rc, v)
		}
		out.Labels = labels
	}
	return out
}

func errorWidget(b *mi.Builder, err error) mi.Node {
	return b.Div(mi.Class("p-4 text-sm text-red-600 dark:text-red-400 border border-red-200 dark:border-red-800 rounded-lg"), err.Error())
}

// renderEmptyDashboard is DASHBOARD_SPEC.md §2.2's zero-section case --
// centred module name + a fixed message, both i18n keys.
func (e *DashboardEngine) renderEmptyDashboard(b *mi.Builder, d *Dashboard, rc RenderContext) mi.Node {
	return b.Div(mi.Class("flex flex-col items-center justify-center py-24 text-center"),
		b.Div(mi.Class("text-lg font-medium text-gray-900 dark:text-white mb-2"), e.t(rc, d.Title)),
		b.Div(mi.Class("text-sm text-gray-500 dark:text-gray-400"), e.t(rc, "dashboard.empty_message")),
	)
}
