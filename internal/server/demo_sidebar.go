package server

import (
	"github.com/ha1tch/seam-ui/internal/modules"
	"github.com/ha1tch/seam-ui/internal/ui"
)

// demoSidebar is the Demo module's left menu: the overview, then one entry
// per page of demonstrations, taken from the pages themselves (ui.DemoGroups)
// so that the menu and the pages cannot disagree.
func demoSidebar() []modules.SidebarItem {
	items := []modules.SidebarItem{
		{Label: "nav.module.demo", Section: true},
		{Label: "demo.overview.title", URL: "/demo", Icon: "science"},
	}
	for _, g := range ui.DemoGroups() {
		items = append(items, modules.SidebarItem{Label: "demo.group." + g.ID + ".title", URL: "/demo/" + g.ID, Icon: g.Icon})
	}
	return items
}
