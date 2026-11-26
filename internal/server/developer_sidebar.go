package server

import (
	"github.com/ha1tch/seam-ui/internal/features"
	"github.com/ha1tch/seam-ui/internal/modules"
)

// developerSidebar is the Developer module's left menu. The Form designer entry
// is there only while its flag is on, so the menu never points at a page that
// answers 404.
func developerSidebar(flags *features.Set) []modules.SidebarItem {
	items := []modules.SidebarItem{
		{Label: "nav.module.developer", Section: true},
		{Label: "developer.nav.overview", URL: "/developer/overview", Icon: "dashboard"},
		{Label: "developer.nav.translations", URL: "/developer/translations", Icon: "translate"},
		{Label: "developer.nav.forms", URL: "/developer/forms", Icon: "dynamic_form"},
	}
	if flags != nil && flags.On(features.DeveloperFormDesigner) {
		items = append(items, modules.SidebarItem{Label: "developer.nav.designer", URL: "/developer/designer", Icon: "design_services"})
	}
	return append(items, modules.SidebarItem{Label: "developer.nav.startup", URL: "/developer/startup", Icon: "tune"})
}
