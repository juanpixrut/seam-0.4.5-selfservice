package features

import (
	"strings"
	"testing"
)

func resolveWith(t *testing.T, env map[string]string) (*Set, error) {
	t.Helper()
	return Catalog().Resolve(Inputs{AuthEnabled: true, LookupEnv: func(n string) (string, bool) { v, ok := env[n]; return v, ok }})
}

// The defaults are decisions, written down with their reasons in catalog.go. This
// pins them: changing one has to be done on purpose, here and there.
func TestDeveloperDecisionFlags_DefaultsAreTheReasonedOnes(t *testing.T) {
	want := []struct {
		key               string
		def, inDevProfile bool
		requires          string
	}{
		{DeveloperViewAs, true, true, Developer},
		{DeveloperTenantOverrides, false, true, Developer},
		{DeveloperInstanceSync, true, true, Developer},
		{DeveloperChangeNotice, true, true, Developer},
		{DeveloperProtectOwnRole, true, true, Developer},
		{DeveloperFormDesigner, false, true, Developer},
		{DesignerLivePreview, true, true, DeveloperFormDesigner},
	}
	reg := Catalog()
	for _, w := range want {
		f, ok := reg.Lookup(w.key)
		if !ok {
			t.Errorf("%s is not in the catalogue", w.key)
			continue
		}
		if f.Default != w.def || f.Class != ClassDev || f.Mode == ModeRuntime || len(f.Requires) != 1 || f.Requires[0] != w.requires || f.EnvVar == "" || f.Description == "" {
			t.Errorf("%s: %+v", w.key, f)
		}
		if got := DefaultOf(w.key); got != w.def {
			t.Errorf("DefaultOf(%s) = %v, want %v", w.key, got, w.def)
		}
		// with the module on and nothing else said, the flag has its default
		set, err := resolveWith(t, map[string]string{"SEAM_ENABLE_DEVELOPER": "1", "SEAM_DEVELOPER_FORM_DESIGNER": map[bool]string{true: "1", false: "0"}[w.requires == DeveloperFormDesigner]})
		if err != nil {
			t.Fatal(err)
		}
		if set.On(w.key) != w.def {
			t.Errorf("%s: with only the module on it is %v, want %v", w.key, set.On(w.key), w.def)
		}
		// the development profile turns the module on and moves two of them
		dev, err := resolveWith(t, map[string]string{"SEAM_PROFILE": "development"})
		if err != nil {
			t.Fatal(err)
		}
		if dev.On(w.key) != w.inDevProfile {
			t.Errorf("%s under the development profile is %v, want %v", w.key, dev.On(w.key), w.inDevProfile)
		}
	}
	if DefaultOf("no_such_flag") {
		t.Errorf("an unknown flag defaults to off")
	}
}

func TestDeveloperDecisionFlags_ProductionRefusesTheModuleButNotTheDefaults(t *testing.T) {
	if _, err := resolveWith(t, map[string]string{"SEAM_PROFILE": "production"}); err != nil {
		t.Errorf("the production profile with nothing set must resolve (defaults that are on must not trip it): %v", err)
	}
	if _, err := resolveWith(t, map[string]string{"SEAM_PROFILE": "production", "SEAM_ENABLE_DEVELOPER": "1"}); err == nil {
		t.Errorf("production must still refuse the module")
	}
	if _, err := resolveWith(t, map[string]string{"SEAM_PROFILE": "production", "SEAM_ENABLE_DEVELOPER": "1", "SEAM_ALLOW_DEV_IN_PRODUCTION": "1"}); err != nil {
		t.Errorf("and allow it when told to: %v", err)
	}
}

func TestDeveloperDecisionFlags_PrerequisitesAreEnforced(t *testing.T) {
	for _, env := range []map[string]string{
		{"SEAM_DEVELOPER_TENANT_OVERRIDES": "1"},
		{"SEAM_DEVELOPER_FORM_DESIGNER": "1"},
	} {
		if _, err := resolveWith(t, env); err == nil {
			t.Errorf("%v must be refused: a decision cannot be taken about a module that is off", env)
		}
	}
	for _, env := range []map[string]string{
		// switching a decision off is never an error, even with its prerequisite off:
		// a standing "=0" must survive the module being turned off
		{"SEAM_DEVELOPER_VIEW_AS": "0"},
		{"SEAM_ENABLE_DEVELOPER": "1", "SEAM_DESIGNER_LIVE_PREVIEW": "0"},
		{"SEAM_ENABLE_DEVELOPER": "1", "SEAM_DEVELOPER_TENANT_OVERRIDES": "1"},
		{"SEAM_ENABLE_DEVELOPER": "1", "SEAM_DEVELOPER_VIEW_AS": "0", "SEAM_DEVELOPER_CHANGE_NOTICE": "0"},
		{"SEAM_ENABLE_DEVELOPER": "1", "SEAM_DEVELOPER_FORM_DESIGNER": "1", "SEAM_DESIGNER_LIVE_PREVIEW": "0"},
		{"SEAM_FEATURES": "developer,developer_tenant_overrides,-developer_instance_sync"},
	} {
		if _, err := resolveWith(t, env); err != nil {
			t.Errorf("%v must resolve: %v", env, err)
		}
	}
	set, err := resolveWith(t, map[string]string{"SEAM_FEATURES": "developer,developer_tenant_overrides,-developer_instance_sync"})
	if err != nil || !set.On(DeveloperTenantOverrides) || set.On(DeveloperInstanceSync) {
		t.Errorf("SEAM_FEATURES must be able to choose them: %v", err)
	}
}

func TestDeveloperDecisionFlags_EveryDecisionIsDocumentedWhereItIsDecided(t *testing.T) {
	// the reasoning lives in a comment above the constants; each flag must be named in it
	for _, key := range []string{DeveloperViewAs, DeveloperTenantOverrides, DeveloperInstanceSync, DeveloperChangeNotice, DeveloperProtectOwnRole, DeveloperFormDesigner, DesignerLivePreview} {
		f, _ := Catalog().Lookup(key)
		if strings.TrimSpace(f.Description) == "" || !strings.HasSuffix(f.Description, ".") {
			t.Errorf("%s needs a one-line description ending in a full stop: %q", key, f.Description)
		}
	}
}
