package config

import (
	"strings"
	"testing"

	"github.com/ha1tch/seam-ui/internal/features"
)

// The visual-component booleans live in the feature registry next to the
// developer flags; these tests pin down how the two mechanisms interact.

func loaded(t *testing.T, env map[string]string, mut func(*Config)) *Config {
	t.Helper()
	for _, k := range []string{"SEAM_FEATURES", "SEAM_PROFILE", "SEAM_ALLOW_DEV_IN_PRODUCTION", "SEAM_SEARCHABLE_SELECT", "SEAM_REORDER_ROWS",
		"SEAM_DOCUMENT_LINKS_LEGACY", "SEAM_COMPRESS_STATIC", "SEAM_ENABLE_DEVELOPER", "SEAM_SELECT_SEARCH_MIN", "SEAM_ROW_EFFECTS"} {
		t.Setenv(k, "")
	}
	for k, v := range env {
		t.Setenv(k, v)
	}
	cfg := Default()
	if mut != nil {
		mut(cfg)
	}
	cfg.LoadFromEnv()
	return cfg
}

func TestRegistry_OurFlagsAreCatalogued(t *testing.T) {
	cfg := loaded(t, nil, nil)
	for _, key := range []string{features.SearchableSelect, features.ReorderRows, features.DocumentLinksLegacy, features.CompressStatic} {
		found := false
		for _, st := range cfg.Flags().States() {
			if st.Flag.Key == key {
				found = true
			}
		}
		if !found {
			t.Errorf("flag %q missing from the resolved set (the Developer panel lists this)", key)
		}
	}
	f := cfg.Features
	if !f.SearchableSelect || !f.ReorderRows || f.DocumentLinksLegacy || !f.CompressStatic {
		t.Errorf("defaults changed by the merge: %+v", f)
	}
}

func TestRegistry_FeaturesListTurnsOurFlagsOffAndOn(t *testing.T) {
	cfg := loaded(t, map[string]string{"SEAM_FEATURES": "-searchable_select,-reorder_rows,document_links_legacy,-compress_static"}, nil)
	f := cfg.Features
	if f.SearchableSelect || f.ReorderRows || !f.DocumentLinksLegacy || f.CompressStatic {
		t.Errorf("SEAM_FEATURES not applied to our flags: %+v", f)
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("a valid list must not fail validation: %v", err)
	}
}

// Adversarial: strict parsing and unknown names are refused at startup, never
// guessed at, and a refused configuration still reports every mistake.
func TestRegistry_HostileValuesAreRefused(t *testing.T) {
	for name, env := range map[string]map[string]string{
		"junk boolean":       {"SEAM_SEARCHABLE_SELECT": "maybe"},
		"junk boolean (cmp)": {"SEAM_COMPRESS_STATIC": "yes please"},
		"unknown flag":       {"SEAM_FEATURES": "no_such_flag"},
		"unknown, negated":   {"SEAM_FEATURES": "-no_such_flag"},
		"developer, no auth": {"SEAM_ENABLE_DEVELOPER": "1"},
	} {
		cfg := loaded(t, env, nil)
		if err := cfg.Validate(); err == nil {
			t.Errorf("%s: expected Validate to refuse %v", name, env)
		}
	}
}

func TestRegistry_PrecedenceEnvBeatsFileBeatsDefault(t *testing.T) {
	// config file (the struct and the generic flags map) turns it off ...
	cfg := loaded(t, nil, func(c *Config) {
		c.Features.ReorderRows = false
		c.Features.Flags = map[string]bool{"compress_static": false}
	})
	if cfg.Features.ReorderRows || cfg.Features.CompressStatic {
		t.Errorf("config-file values lost: %+v", cfg.Features)
	}
	// ... and the environment turns it back on
	cfg = loaded(t, map[string]string{"SEAM_REORDER_ROWS": "true", "SEAM_COMPRESS_STATIC": "1"}, func(c *Config) {
		c.Features.ReorderRows = false
		c.Features.Flags = map[string]bool{"compress_static": false}
	})
	if !cfg.Features.ReorderRows || !cfg.Features.CompressStatic {
		t.Errorf("the environment must win over the file: %+v", cfg.Features)
	}
}

// The two non-boolean settings are not flags: the registry must leave them be.
func TestRegistry_NonBooleanSettingsAreUntouched(t *testing.T) {
	cfg := loaded(t, map[string]string{"SEAM_SELECT_SEARCH_MIN": "3", "SEAM_ROW_EFFECTS": "all"}, nil)
	if cfg.Features.SelectSearchMin != 3 || cfg.Features.RowEffects != "all" {
		t.Errorf("settings lost: %+v", cfg.Features)
	}
	cfg = loaded(t, map[string]string{"SEAM_SELECT_SEARCH_MIN": "-9"}, nil)
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "select_search_min") {
		t.Errorf("a negative threshold must still be refused by name: %v", err)
	}
}

// The production profile refuses development-class flags; ours are ordinary
// flags and must neither trip it nor be switched by it.
func TestRegistry_ProductionProfileLeavesOurFlagsAlone(t *testing.T) {
	cfg := loaded(t, map[string]string{"SEAM_PROFILE": "production"}, nil)
	if err := cfg.Validate(); err != nil {
		t.Fatalf("production profile with defaults must validate: %v", err)
	}
	if f := cfg.Features; !f.SearchableSelect || !f.ReorderRows || f.DocumentLinksLegacy || !f.CompressStatic {
		t.Errorf("the production profile changed our flags: %+v", f)
	}
}
