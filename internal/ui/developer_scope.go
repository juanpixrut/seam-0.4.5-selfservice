package ui

import (
	"context"

	mi "github.com/ha1tch/minty"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/devruntime"
	"github.com/ha1tch/seam-ui/internal/overrides"
)

// Scopes of the Developer module's editors (campaign DEV-13, DEV-16).
//
//	tenant  the override is stored in the database for the viewer's tenant only
//	file    the override is a file on disk and applies to every tenant
//
// Which are offered depends on what is configured: tenant needs a store for the
// overrides (xolu), file needs SEAM_LOCALES_DIR or SEAM_SCHEMAS_DIR. A file
// override outranks a tenant's database override of the same thing (DEC-4), so
// the editors say when one hides the other.
//
// A third scope exists where a deployment asks for it (campaign DEV-26):
//
//	global  the override is stored in the database and applies to every tenant,
//	        below each tenant's own overrides
//
// It is offered only to the developers of the tenant named by the setting
// developer_global_tenant (none by default), since what it changes is seen by
// tenants its author does not belong to. Everything else about it is the tenant
// scope: the same store, history, restore and sync, under overrides.GlobalTenant.
const (
	scopeTenant = "tenant"
	scopeFile   = "file"
	scopeGlobal = "global"
)

// scopeStored reports whether the scope's overrides are kept in the database.
func scopeStored(scope string) bool { return scope == scopeTenant || scope == scopeGlobal }

// scopeTenantID is the tenant a stored scope's overrides are kept under: the
// caller's own, or the one that stands for every tenant.
func scopeTenantID(scope string, tenant int64) int64 {
	if scope == scopeGlobal {
		return overrides.GlobalTenant
	}
	return tenant
}

// scopesWithGlobal adds the global scope to the scopes on offer when the
// caller may use it: the scope is switched on, there is a store, and the
// caller's tenant is the one the setting names. It goes after the tenant scope,
// which stays the default.
func (h *UIHandler) scopesWithGlobal(ctx context.Context, scopes []string) []string {
	rt := h.config.DevRuntime
	home := h.config.DevSettings.GlobalTenant
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok || home <= 0 || claims.TenantID != home || rt == nil || rt.Overrides == nil {
		return scopes
	}
	out := make([]string, 0, len(scopes)+1)
	for _, s := range scopes {
		out = append(out, s)
		if s == scopeTenant {
			out = append(out, scopeGlobal)
		}
	}
	return out
}

// developerScopes lists the scopes available for one kind of override, tenant
// first. fileOK says whether that kind's directory is configured.
func developerScopes(rt *devruntime.Runtime, fileOK bool) []string {
	var out []string
	if rt != nil && rt.Overrides != nil {
		out = append(out, scopeTenant)
	}
	if rt != nil && fileOK {
		out = append(out, scopeFile)
	}
	return out
}

// developerScope is the requested scope when it is one of scopes, otherwise the
// first of them ("" when there are none).
func developerScope(requested string, scopes []string) string {
	for _, s := range scopes {
		if s == requested {
			return s
		}
	}
	if len(scopes) > 0 {
		return scopes[0]
	}
	return ""
}

// developerScopeValid reports whether a posted scope is acceptable: absent (the
// default applies) or one of scopes.
func developerScopeValid(posted string, scopes []string) bool {
	if posted == "" {
		return true
	}
	for _, s := range scopes {
		if s == posted {
			return true
		}
	}
	return false
}

// developerScopeTabs is the choice between the scopes as links, each to href
// with its scope added by hrefFor; nothing when there is only one.
func developerScopeTabs(b *mi.Builder, t func(key string, args ...any) string, scopes []string, active string, hrefFor func(scope string) string) []interface{} {
	hint := b.P(mi.Class(devMutedClass), t("developer.scope."+active+".hint"))
	if len(scopes) < 2 {
		return []interface{}{hint}
	}
	items := make([]developerSegItem, 0, len(scopes))
	for _, s := range scopes {
		items = append(items, developerSegItem{Label: t("developer.scope." + s), Href: hrefFor(s), Active: s == active})
	}
	return []interface{}{b.Div(mi.Class("space-y-2"),
		b.Div(mi.Class("text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider"), t("developer.scope.label")),
		developerSeg(b, items), hint)}
}
