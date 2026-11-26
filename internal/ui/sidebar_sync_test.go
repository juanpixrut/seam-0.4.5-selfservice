package ui

import (
	"testing"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/modules"
)

// TestActiveModuleSidebarItems_LongestPrefixWins confirms Assets' own
// "/" (the universal fallback, length 1) correctly loses to a more
// specific module whenever that module's own Path also matches --
// exactly the SIDEBAR_SYNC_PLAN_2026-08-06.md resolution rule, tested
// directly rather than only exercised through a live server.
func TestActiveModuleSidebarItems_LongestPrefixWins(t *testing.T) {
	// Deliberately registered with the more specific module FIRST and
	// the universal fallback SECOND -- if resolution incorrectly used
	// "last matching module wins" instead of "longest Path wins," this
	// ordering would make that mistake pass by accident (found exactly
	// this gap in an earlier version of this test, which had the
	// modules in the other order and passed even with the length
	// comparison removed entirely, because registration order alone
	// happened to produce the right answer).
	cfg := Config{
		VisibleModules: []modules.Module{
			{
				ID:   "issues",
				Path: "/issues",
				SidebarItems: []modules.SidebarItem{
					{Label: "sec2", Section: true},
					{Label: "issues.item", URL: "/issues", Icon: "bug_report"},
				},
			},
			{
				ID:   "assets",
				Path: "/",
				SidebarItems: []modules.SidebarItem{
					{Label: "sec", Section: true},
					{Label: "assets.item", URL: "/assets", Icon: "inventory"},
				},
			},
		},
	}

	items := activeModuleSidebarItems("/issues/42", cfg, nil)
	if len(items) != 2 {
		t.Fatalf("expected 2 items (1 section + 1 link) for the issues module, got %d: %+v", len(items), items)
	}
	if items[1].URL != "/issues" {
		t.Errorf("expected the issues module's own item, got %+v -- Assets' \"/\" appears to have incorrectly won despite being shorter and registered after", items[1])
	}
}

// TestActiveModuleSidebarItems_NoMatchingModuleReturnsNil confirms a
// module with zero SidebarItems (Documents, deliberately) or a path
// matching no registered module at all produces no sidebar content,
// not a panic or a stale fallback.
func TestActiveModuleSidebarItems_NoMatchingModuleReturnsNil(t *testing.T) {
	cfg := Config{
		VisibleModules: []modules.Module{
			{ID: "documents", Path: "/documents", SidebarItems: nil},
		},
	}
	items := activeModuleSidebarItems("/documents", cfg, nil)
	if len(items) != 0 {
		t.Errorf("expected no items for a module with no declared SidebarItems, got %+v", items)
	}

	items2 := activeModuleSidebarItems("/nonexistent-path", cfg, nil)
	if len(items2) != 0 {
		t.Errorf("expected no items when no module's Path matches at all, got %+v", items2)
	}
}

// TestActiveModuleSidebarItems_RequiredPermFiltersWithinModule
// confirms a module-internal item gated by RequiredPerm is correctly
// hidden from a role that lacks it, while an ungated item in the same
// module still renders -- the permission-filtering half of
// SIDEBAR_SYNC_PLAN_2026-08-06.md, distinct from module-level
// visibility (cfg.VisibleModules is already filtered before this
// function ever sees it).
func TestActiveModuleSidebarItems_RequiredPermFiltersWithinModule(t *testing.T) {
	cfg := Config{
		UserRole: string(auth.RoleViewer),
		VisibleModules: []modules.Module{
			{
				ID:   "assets",
				Path: "/",
				SidebarItems: []modules.SidebarItem{
					{Label: "sec", Section: true},
					{Label: "always visible", URL: "/assets", Icon: "inventory"},
					{Label: "admin only", URL: "/rules", Icon: "rule", RequiredPerm: string(auth.PermAssetsRuleEdit)},
				},
			},
		},
	}
	items := activeModuleSidebarItems("/", cfg, nil)
	var urls []string
	for _, it := range items {
		if it.URL != "" {
			urls = append(urls, it.URL)
		}
	}
	foundAssets, foundRules := false, false
	for _, u := range urls {
		if u == "/assets" {
			foundAssets = true
		}
		if u == "/rules" {
			foundRules = true
		}
	}
	if !foundAssets {
		t.Error("expected the ungated item to render for a Viewer")
	}
	if foundRules {
		t.Error("expected the RequiredPerm-gated item to be hidden from a Viewer lacking that permission")
	}
}

// TestActiveModuleSidebarItems_MapFilteredWhenDisabled is a direct,
// isolated regression guard for the one runtime-mutable exception
// (EnableMap, toggled from Settings without a server restart) --
// TestMapNavItemDefaultVisible/TestMapNavItemHiddenWhenDisabled
// (vendor_html_test.go) already cover this through a full BaseLayout
// render; this covers the same rule directly against the resolver
// function itself.
func TestActiveModuleSidebarItems_MapFilteredWhenDisabled(t *testing.T) {
	cfg := Config{
		EnableMap: false,
		VisibleModules: []modules.Module{
			{
				ID:   "assets",
				Path: "/",
				SidebarItems: []modules.SidebarItem{
					{Label: "sec", Section: true},
					{Label: "nav.map", URL: "/map", Icon: "map"},
				},
			},
		},
	}
	items := activeModuleSidebarItems("/", cfg, nil)
	for _, it := range items {
		if it.URL == "/map" {
			t.Error("expected the Map item to be filtered out when EnableMap is false")
		}
	}
}
