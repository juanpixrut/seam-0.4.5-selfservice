package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/seam-ui/internal/features"
)

// clearFeatureEnv blanks every variable the flag machinery reads, so a
// test does not depend on the environment it runs in. t.Setenv restores
// the originals afterwards; a blank value is treated as unset.
func clearFeatureEnv(t *testing.T) {
	t.Helper()
	names := []string{features.EnvProfile, features.EnvList, features.EnvAllowDev,
		"SEAM_AUTH_ENABLED", "SEAM_JWT_SECRET"}
	for _, f := range features.Catalog().Flags() {
		if f.EnvVar != "" {
			names = append(names, f.EnvVar)
		}
	}
	for _, n := range names {
		t.Setenv(n, "")
	}
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFeaturesDefaultsUnchanged(t *testing.T) {
	clearFeatureEnv(t)
	cfg := Default()
	cfg.LoadFromEnv()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	f := cfg.Features
	if !f.EnableUI || !f.EnableAPI || !f.EnableOperational || !f.EnableAdmin ||
		!f.EnableWebhooks || !f.EnableRules || !f.EnableFSM || !f.EnableQR {
		t.Error("default-on flags changed")
	}
	if f.EnableWorkOrders || f.EnableIssues || f.EnableDemo || f.EnableDeveloper || f.DevMode || f.VendorJS {
		t.Error("default-off flags changed")
	}
	if cfg.FeatureFlags == nil || cfg.Flags() != cfg.FeatureFlags {
		t.Error("FeatureFlags not populated")
	}
}

func TestFeaturesEnvIsNowStrictAndForgiving(t *testing.T) {
	clearFeatureEnv(t)

	// TRUE, yes and on used to silently mean false.
	for _, v := range []string{"TRUE", "yes", "On", "1", "true"} {
		t.Setenv("SEAM_ENABLE_DEMO", v)
		cfg := Default()
		cfg.LoadFromEnv()
		if err := cfg.Validate(); err != nil || !cfg.Features.EnableDemo {
			t.Errorf("SEAM_ENABLE_DEMO=%q: demo=%v err=%v", v, cfg.Features.EnableDemo, err)
		}
	}
	for _, v := range []string{"FALSE", "no", "off", "0"} {
		t.Setenv("SEAM_ENABLE_UI", v)
		cfg := Default()
		cfg.LoadFromEnv()
		if err := cfg.Validate(); err != nil || cfg.Features.EnableUI {
			t.Errorf("SEAM_ENABLE_UI=%q: ui=%v err=%v", v, cfg.Features.EnableUI, err)
		}
	}

	// Junk is an error rather than a silent false.
	t.Setenv("SEAM_ENABLE_UI", "enabled")
	t.Setenv("SEAM_ENABLE_DEMO", "maybe")
	cfg := Default()
	cfg.LoadFromEnv()
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "SEAM_ENABLE_UI") || !strings.Contains(err.Error(), "SEAM_ENABLE_DEMO") {
		t.Fatalf("want both bad variables named, got %v", err)
	}
	if !cfg.Features.EnableUI {
		t.Error("a failed resolution must leave the struct untouched")
	}
}

func TestFeaturesEnvOverridesFile(t *testing.T) {
	clearFeatureEnv(t)
	path := writeConfig(t, `{"features": {"enable_demo": true, "enable_ui": false, "qr_prefix": "XX"}}`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.LoadFromEnv()
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if !cfg.Features.EnableDemo || cfg.Features.EnableUI || cfg.Features.QRPrefix != "XX" {
		t.Errorf("file values lost: %+v", cfg.Features)
	}

	t.Setenv("SEAM_ENABLE_DEMO", "0")
	t.Setenv("SEAM_ENABLE_UI", "1")
	cfg, _ = Load(path)
	cfg.LoadFromEnv()
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if cfg.Features.EnableDemo || !cfg.Features.EnableUI {
		t.Errorf("env must beat the file: %+v", cfg.Features)
	}
	if cfg.FeatureFlags.Source(features.Demo) != features.SourceEnv {
		t.Errorf("source = %v", cfg.FeatureFlags.Source(features.Demo))
	}
}

func TestFeaturesHandBuiltConfigIsPreserved(t *testing.T) {
	clearFeatureEnv(t)
	cfg := Default()
	cfg.Features.EnableAdmin = false // set in code, not file or env
	cfg.Features.EnableWorkOrders = true
	cfg.LoadFromEnv()
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if cfg.Features.EnableAdmin || !cfg.Features.EnableWorkOrders {
		t.Errorf("LoadFromEnv clobbered values set in code: %+v", cfg.Features)
	}
}

func TestFeaturesListEnv(t *testing.T) {
	clearFeatureEnv(t)
	t.Setenv(features.EnvList, "demo,-admin")
	cfg := Default()
	cfg.LoadFromEnv()
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if !cfg.Features.EnableDemo || cfg.Features.EnableAdmin {
		t.Errorf("SEAM_FEATURES not applied: %+v", cfg.Features)
	}
	t.Setenv(features.EnvList, "demo,nonsense")
	cfg = Default()
	cfg.LoadFromEnv()
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "nonsense") {
		t.Errorf("unknown flag in SEAM_FEATURES should be reported, got %v", err)
	}
}

func TestDeveloperRequiresAuth(t *testing.T) {
	clearFeatureEnv(t)
	t.Setenv("SEAM_ENABLE_DEVELOPER", "1")

	cfg := Default()
	cfg.LoadFromEnv()
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "authentication") {
		t.Fatalf("developer without auth must be refused, got %v", err)
	}

	t.Setenv("SEAM_JWT_SECRET", "s3cret") // enables auth
	cfg = Default()
	cfg.LoadFromEnv()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("developer with auth: %v", err)
	}
	if !cfg.Features.EnableDeveloper || !cfg.Flags().On(features.Developer) {
		t.Error("developer flag not on")
	}
}

func TestDeveloperGuardSurvivesAuthChangedAfterLoad(t *testing.T) {
	clearFeatureEnv(t)
	t.Setenv("SEAM_ENABLE_DEVELOPER", "1")
	t.Setenv("SEAM_JWT_SECRET", "s3cret")
	cfg := Default()
	cfg.LoadFromEnv()
	cfg.Auth.Enabled = false // e.g. a later override
	if err := cfg.Validate(); err == nil {
		t.Error("Validate must re-check the developer/auth rule")
	}
}

func TestFlagsMapPinsRuntimeFlagsFromFile(t *testing.T) {
	clearFeatureEnv(t)
	t.Setenv("SEAM_JWT_SECRET", "s3cret")
	path := writeConfig(t, `{"features": {"enable_developer": true,
		"flags": {"hotload_i18n": false, "overlay_xolu": false}}}`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.LoadFromEnv()
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	fl := cfg.Flags()
	if !fl.Pinned(features.OverlayXolu) {
		t.Error("a runtime flag set in the config file must be pinned")
	}
	yes := true
	if v, src := fl.Runtime(features.OverlayXolu, &yes); v || src != features.SourceConfig {
		t.Errorf("pinned flag: %v from %v", v, src)
	}
}

func TestFlagsMapUnknownKeyIsAnError(t *testing.T) {
	clearFeatureEnv(t)
	path := writeConfig(t, `{"features": {"flags": {"enable_demo": true}}}`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.LoadFromEnv()
	// The generic map is keyed by flag key ("demo"), not config key.
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "enable_demo") {
		t.Errorf("want unknown-flag error naming the key, got %v", err)
	}
}

func TestProductionProfileRefusesDevFlags(t *testing.T) {
	clearFeatureEnv(t)
	t.Setenv(features.EnvProfile, "production")

	cfg := Default()
	cfg.LoadFromEnv()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("plain production: %v", err)
	}

	t.Setenv("SEAM_ENABLE_DEMO", "1")
	cfg = Default()
	cfg.LoadFromEnv()
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "production") {
		t.Errorf("demo under production must be refused, got %v", err)
	}

	// dev_mode arrives via the file and is also dev-class.
	t.Setenv("SEAM_ENABLE_DEMO", "")
	cfg, _ = Load(writeConfig(t, `{"features": {"dev_mode": true}}`))
	cfg.LoadFromEnv()
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "dev_mode") {
		t.Errorf("dev_mode under production must be refused, got %v", err)
	}
}

func TestFlagsOnDemandWithoutLoadFromEnv(t *testing.T) {
	clearFeatureEnv(t)
	cfg := Default()
	if cfg.FeatureFlags != nil {
		t.Fatal("precondition")
	}
	if fl := cfg.Flags(); fl == nil || !fl.On(features.UI) {
		t.Error("Flags() should resolve on demand")
	}
}

func TestExampleConfigStillLoads(t *testing.T) {
	clearFeatureEnv(t)
	cfg, err := Load("../../config.example.json")
	if err != nil {
		t.Skipf("example config not reachable from here: %v", err)
	}
	cfg.LoadFromEnv()
	if err := cfg.Validate(); err != nil {
		t.Errorf("config.example.json must remain valid: %v", err)
	}
}

// Every flag that can be set in the config file must be shown in
// config.example.json, so operators can discover it (campaign DEV-25).
// Runtime flags have no key of their own; they are set through "flags".
func TestExampleConfigDocumentsEveryFlag(t *testing.T) {
	raw, err := os.ReadFile("../../config.example.json")
	if err != nil {
		t.Skipf("example config not reachable from here: %v", err)
	}
	var doc struct {
		Features map[string]json.RawMessage `json:"features"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for _, f := range features.Catalog().Flags() {
		if f.ConfigKey == "" {
			continue
		}
		if _, ok := doc.Features[f.ConfigKey]; !ok {
			t.Errorf("config.example.json does not show %q (flag %q)", f.ConfigKey, f.Key)
		}
	}
	for _, k := range []string{"flags", "locales_dir", "schemas_dir"} {
		if _, ok := doc.Features[k]; !ok {
			t.Errorf("config.example.json does not show %q", k)
		}
	}
}
