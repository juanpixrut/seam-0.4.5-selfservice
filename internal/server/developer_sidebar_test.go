package server

import (
	"testing"

	"github.com/ha1tch/seam-ui/internal/features"
)

func TestDeveloperSidebar_TheDesignerIsThereOnlyWhileItsFlagIsOn(t *testing.T) {
	resolve := func(env map[string]string) *features.Set {
		set, err := features.Catalog().Resolve(features.Inputs{AuthEnabled: true, LookupEnv: func(n string) (string, bool) { v, ok := env[n]; return v, ok }})
		if err != nil {
			t.Fatal(err)
		}
		return set
	}
	labels := func(set *features.Set) []string {
		var out []string
		for _, it := range developerSidebar(set) {
			out = append(out, it.Label)
		}
		return out
	}
	has := func(ls []string, l string) bool {
		for _, x := range ls {
			if x == l {
				return true
			}
		}
		return false
	}
	off := labels(resolve(map[string]string{"SEAM_ENABLE_DEVELOPER": "1"}))
	on := labels(resolve(map[string]string{"SEAM_ENABLE_DEVELOPER": "1", "SEAM_DEVELOPER_FORM_DESIGNER": "1"}))
	if has(off, "developer.nav.designer") || !has(on, "developer.nav.designer") || has(labels(nil), "developer.nav.designer") {
		t.Errorf("off %v, on %v", off, on)
	}
	for _, l := range []string{"nav.module.developer", "developer.nav.overview", "developer.nav.translations", "developer.nav.forms", "developer.nav.startup"} {
		if !has(off, l) || !has(on, l) {
			t.Errorf("%s must always be in the menu", l)
		}
	}
	if on[len(on)-1] != "developer.nav.startup" {
		t.Errorf("Startup stays last: %v", on)
	}
}
