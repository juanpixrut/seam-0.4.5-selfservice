package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFeatureDefaults_AreTheShippedChoices(t *testing.T) {
	f := Default().Features
	if !f.SearchableSelect || f.SelectSearchMin != 8 || !f.ReorderRows || f.DocumentLinksLegacy || f.RowEffects != RowEffectsScoped || !f.CompressStatic {
		t.Errorf("unexpected defaults: %+v", f)
	}
	if f.EnableDemo {
		t.Errorf("the demo module stays opt-in")
	}
}

// A config file that does not mention the new keys keeps every default, and one
// that sets a key to false really turns it off.
func TestFeatureFlags_FileOverridesOnlyWhatItNames(t *testing.T) {
	dir := t.TempDir()
	write := func(body string) string {
		p := filepath.Join(dir, "c.json")
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	cfg, err := Load(write(`{"features": {"enable_demo": true}}`))
	if err != nil {
		t.Fatal(err)
	}
	f := cfg.Features
	if !f.SearchableSelect || !f.ReorderRows || !f.CompressStatic || f.SelectSearchMin != 8 || f.RowEffects != RowEffectsScoped {
		t.Errorf("unmentioned keys must keep their defaults: %+v", f)
	}
	cfg, err = Load(write(`{"features": {"searchable_select": false, "reorder_rows": false, "compress_static": false, "document_links_legacy": true, "select_search_min": 3, "row_effects": "all"}}`))
	if err != nil {
		t.Fatal(err)
	}
	f = cfg.Features
	if f.SearchableSelect || f.ReorderRows || f.CompressStatic || !f.DocumentLinksLegacy || f.SelectSearchMin != 3 || f.RowEffects != "all" {
		t.Errorf("explicit values must win: %+v", f)
	}
}

func TestFeatureFlags_EnvOverrides(t *testing.T) {
	for k, v := range map[string]string{
		"SEAM_SEARCHABLE_SELECT": "0", "SEAM_REORDER_ROWS": "false", "SEAM_COMPRESS_STATIC": "0",
		"SEAM_DOCUMENT_LINKS_LEGACY": "1", "SEAM_SELECT_SEARCH_MIN": "4", "SEAM_ROW_EFFECTS": "off",
	} {
		t.Setenv(k, v)
	}
	cfg := Default()
	cfg.LoadFromEnv()
	f := cfg.Features
	if f.SearchableSelect || f.ReorderRows || f.CompressStatic || !f.DocumentLinksLegacy || f.SelectSearchMin != 4 || f.RowEffects != "off" {
		t.Errorf("env overrides not applied: %+v", f)
	}
}

// Adversarial environment values: garbage numbers are ignored (the default
// stays), garbage modes and out-of-range numbers are refused by Validate, and
// the casing and padding of a mode never matters.
func TestFeatureFlags_HostileValues(t *testing.T) {
	t.Setenv("SEAM_SELECT_SEARCH_MIN", "abc")
	cfg := Default()
	cfg.LoadFromEnv()
	if cfg.Features.SelectSearchMin != 8 {
		t.Errorf("a non-number must be ignored, got %d", cfg.Features.SelectSearchMin)
	}
	cases := []struct {
		name string
		mut  func(*FeatureConfig)
		bad  bool
	}{
		{"min 0 means default", func(f *FeatureConfig) { f.SelectSearchMin = 0 }, false},
		{"min 1", func(f *FeatureConfig) { f.SelectSearchMin = 1 }, false},
		{"min 10000", func(f *FeatureConfig) { f.SelectSearchMin = 10000 }, false},
		{"min negative", func(f *FeatureConfig) { f.SelectSearchMin = -1 }, true},
		{"min too large", func(f *FeatureConfig) { f.SelectSearchMin = 10001 }, true},
		{"min int overflow-ish", func(f *FeatureConfig) { f.SelectSearchMin = 1 << 40 }, true},
		{"mode ALL", func(f *FeatureConfig) { f.RowEffects = "ALL" }, false},
		{"mode padded", func(f *FeatureConfig) { f.RowEffects = "  off " }, false},
		{"mode empty", func(f *FeatureConfig) { f.RowEffects = "" }, false},
		{"mode bogus", func(f *FeatureConfig) { f.RowEffects = "sometimes" }, true},
		{"mode injection", func(f *FeatureConfig) { f.RowEffects = `all" onload="x` }, true},
	}
	for _, c := range cases {
		cfg := Default()
		cfg.XOLU.URL = "http://x"
		c.mut(&cfg.Features)
		err := cfg.Validate()
		if (err != nil) != c.bad {
			t.Errorf("%s: Validate error = %v, want error: %v", c.name, err, c.bad)
		}
		if err != nil && !strings.Contains(err.Error(), "features.") {
			t.Errorf("%s: the error should name the setting: %v", c.name, err)
		}
	}
	for in, want := range map[string]string{"all": "all", " ALL ": "all", "off": "off", "": "scoped", "scoped": "scoped", "bogus": "scoped", `all" onload="x`: "scoped"} {
		if got := NormalizeRowEffects(in); got != want {
			t.Errorf("NormalizeRowEffects(%q) = %q, want %q", in, got, want)
		}
	}
}
