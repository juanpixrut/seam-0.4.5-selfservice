package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Server.Port)
	}

	if cfg.XOLU.URL != "http://localhost:9090" {
		t.Errorf("expected default OLU URL, got %s", cfg.XOLU.URL)
	}

	if cfg.Features.EnableUI != true {
		t.Error("expected EnableUI to be true by default")
	}

	if cfg.Features.EnableAPI != true {
		t.Error("expected EnableAPI to be true by default")
	}

	// Regression: these were missing from Default() causing all operational
	// and admin routes to return 404 on a fresh launch (patched157).
	if cfg.Features.EnableOperational != true {
		t.Error("expected EnableOperational to be true by default — if false, all operational routes return 404")
	}

	if cfg.Features.EnableAdmin != true {
		t.Error("expected EnableAdmin to be true by default — if false, all admin/master-table routes return 404")
	}

	if cfg.Log.Level != "info" {
		t.Errorf("expected default log level 'info', got %s", cfg.Log.Level)
	}
}

// =============================================================================
// SECURITY-CRITICAL DEFAULT TESTS
// =============================================================================
// These tests ensure that security-sensitive defaults fail closed.
// If any of these fail, it's a potential security regression.

func TestDefault_SecurityCriticalDefaults(t *testing.T) {
	cfg := Default()

	// DevMode MUST be false by default
	// If enabled accidentally in production, it enables dangerous fallbacks
	if cfg.Features.DevMode {
		t.Error("SECURITY: DevMode must be false by default")
	}

	// TenantFromClaimsOnly MUST be true by default
	// If false, X-Tenant-ID header can be used to access other tenants
	if !cfg.Auth.TenantFromClaimsOnly {
		t.Error("SECURITY: TenantFromClaimsOnly must be true by default")
	}

	// CSRF MUST be enabled by default when auth is used
	if !cfg.Auth.CSRFEnabled {
		t.Error("SECURITY: CSRFEnabled must be true by default")
	}
}

func TestLoad(t *testing.T) {
	// Create temp config file
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	configJSON := `{
		"server": {
			"port": 9090,
			"read_timeout": 60000000000
		},
		"xolu": {
			"url": "http://olu:9000"
		},
		"features": {
			"enable_ui": false
		},
		"log": {
			"level": "debug"
		}
	}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Server.Port)
	}

	if cfg.Server.ReadTimeout.Duration() != 60*time.Second {
		t.Errorf("expected read timeout 60s, got %v", cfg.Server.ReadTimeout)
	}

	if cfg.XOLU.URL != "http://olu:9000" {
		t.Errorf("expected OLU URL 'http://olu:9000', got %s", cfg.XOLU.URL)
	}

	if cfg.Features.EnableUI != false {
		t.Error("expected EnableUI to be false")
	}

	if cfg.Log.Level != "debug" {
		t.Errorf("expected log level 'debug', got %s", cfg.Log.Level)
	}

	// Verify defaults are preserved for unspecified fields
	if cfg.Features.EnableAPI != true {
		t.Error("expected EnableAPI to remain true (default)")
	}
}

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load("/nonexistent/config.json")
	if err != nil {
		t.Fatalf("Load() should not error on missing file: %v", err)
	}

	// Should return defaults
	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Server.Port)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	if err := os.WriteFile(configPath, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("Load() should error on invalid JSON")
	}
}

func TestLoadStringDurations(t *testing.T) {
	// Test that string durations like "30s" and "24h" are parsed correctly
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	configJSON := `{
		"server": {
			"port": 8080,
			"read_timeout": "30s",
			"write_timeout": "1m",
			"shutdown_timeout": "10s"
		},
		"xolu": {
			"url": "http://localhost:9090",
			"timeout": "30s",
			"retry_delay": "500ms"
		},
		"auth": {
			"enabled": false,
			"token_expiry": "24h",
			"refresh_expiry": "168h",
			"session_duration": "12h"
		}
	}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Server.ReadTimeout.Duration() != 30*time.Second {
		t.Errorf("expected read timeout 30s, got %v", cfg.Server.ReadTimeout)
	}

	if cfg.Server.WriteTimeout.Duration() != time.Minute {
		t.Errorf("expected write timeout 1m, got %v", cfg.Server.WriteTimeout)
	}

	if cfg.XOLU.Timeout.Duration() != 30*time.Second {
		t.Errorf("expected OLU timeout 30s, got %v", cfg.XOLU.Timeout)
	}

	if cfg.XOLU.RetryDelay.Duration() != 500*time.Millisecond {
		t.Errorf("expected retry delay 500ms, got %v", cfg.XOLU.RetryDelay)
	}

	if cfg.Auth.TokenExpiry.Duration() != 24*time.Hour {
		t.Errorf("expected token expiry 24h, got %v", cfg.Auth.TokenExpiry)
	}

	if cfg.Auth.RefreshExpiry.Duration() != 168*time.Hour {
		t.Errorf("expected refresh expiry 168h, got %v", cfg.Auth.RefreshExpiry)
	}

	if cfg.Auth.SessionDuration.Duration() != 12*time.Hour {
		t.Errorf("expected session duration 12h, got %v", cfg.Auth.SessionDuration)
	}
}

func TestLoadFromEnv(t *testing.T) {
	cfg := Default()

	// Set environment variables
	os.Setenv("SEAM_HOST", "0.0.0.0")
	os.Setenv("SEAM_PORT", "3000")
	os.Setenv("XOLU_URL", "http://remote-olu:9000")
	os.Setenv("SEAM_JWT_SECRET", "test-secret")
	os.Setenv("SEAM_LOG_LEVEL", "debug")
	os.Setenv("SEAM_ENABLE_UI", "false")
	os.Setenv("SEAM_RATE_LIMIT", "100")

	defer func() {
		os.Unsetenv("SEAM_HOST")
		os.Unsetenv("SEAM_PORT")
		os.Unsetenv("XOLU_URL")
		os.Unsetenv("SEAM_JWT_SECRET")
		os.Unsetenv("SEAM_LOG_LEVEL")
		os.Unsetenv("SEAM_ENABLE_UI")
		os.Unsetenv("SEAM_RATE_LIMIT")
	}()

	cfg.LoadFromEnv()

	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected host '0.0.0.0', got %s", cfg.Server.Host)
	}

	if cfg.Server.Port != 3000 {
		t.Errorf("expected port 3000, got %d", cfg.Server.Port)
	}

	if cfg.XOLU.URL != "http://remote-olu:9000" {
		t.Errorf("expected OLU URL from env, got %s", cfg.XOLU.URL)
	}

	if cfg.Auth.JWTSecret != "test-secret" {
		t.Errorf("expected JWT secret from env, got %s", cfg.Auth.JWTSecret)
	}

	if cfg.Auth.Enabled != true {
		t.Error("expected auth to be enabled when JWT secret is set")
	}

	if cfg.Log.Level != "debug" {
		t.Errorf("expected log level 'debug', got %s", cfg.Log.Level)
	}

	if cfg.Features.EnableUI != false {
		t.Error("expected EnableUI to be false from env")
	}

	if cfg.Features.RateLimitPerMin != 100 {
		t.Errorf("expected rate limit 100, got %d", cfg.Features.RateLimitPerMin)
	}
}

func TestLoadFromEnv_ForceLanguage(t *testing.T) {
	cfg := Default()

	if cfg.Features.ForceLanguage != "" {
		t.Errorf("expected ForceLanguage empty by default, got %q", cfg.Features.ForceLanguage)
	}

	os.Setenv("SEAM_FORCE_LANGUAGE", "es")
	defer os.Unsetenv("SEAM_FORCE_LANGUAGE")

	cfg.LoadFromEnv()

	if cfg.Features.ForceLanguage != "es" {
		t.Errorf("expected ForceLanguage 'es' from env, got %q", cfg.Features.ForceLanguage)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
	}{
		{
			name:    "valid default config",
			modify:  func(c *Config) {},
			wantErr: false,
		},
		{
			name: "invalid port - too low",
			modify: func(c *Config) {
				c.Server.Port = 0
			},
			wantErr: true,
		},
		{
			name: "invalid port - too high",
			modify: func(c *Config) {
				c.Server.Port = 70000
			},
			wantErr: true,
		},
		{
			name: "missing OLU URL",
			modify: func(c *Config) {
				c.XOLU.URL = ""
			},
			wantErr: true,
		},
		{
			name: "auth enabled without secret",
			modify: func(c *Config) {
				c.Auth.Enabled = true
				c.Auth.JWTSecret = ""
			},
			wantErr: true,
		},
		{
			name: "auth enabled with secret",
			modify: func(c *Config) {
				c.Auth.Enabled = true
				c.Auth.JWTSecret = "secret"
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			tt.modify(cfg)

			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAddress(t *testing.T) {
	cfg := Default()

	if cfg.Address() != ":8080" {
		t.Errorf("expected ':8080', got %s", cfg.Address())
	}

	cfg.Server.Host = "0.0.0.0"
	cfg.Server.Port = 3000

	if cfg.Address() != "0.0.0.0:3000" {
		t.Errorf("expected '0.0.0.0:3000', got %s", cfg.Address())
	}
}

func TestIsTLS(t *testing.T) {
	cfg := Default()

	if cfg.IsTLS() {
		t.Error("expected IsTLS() to be false with no TLS config")
	}

	cfg.Server.TLSCert = "/path/to/cert.pem"
	if cfg.IsTLS() {
		t.Error("expected IsTLS() to be false with only cert")
	}

	cfg.Server.TLSKey = "/path/to/key.pem"
	if !cfg.IsTLS() {
		t.Error("expected IsTLS() to be true with cert and key")
	}
}

func TestAccountingDefaultsAreConservative(t *testing.T) {
	c := Default()
	a := c.Accounting
	if c.Features.EnableAccounting {
		t.Error("the module must be off by default")
	}
	if a.AccountantOperate || a.StaffPostEntries {
		t.Error("duties must be separated by default")
	}
	if a.CommissionBPSReal != 0 || a.CommissionBPSSeamAE != 0 || a.CommissionBPSSeamWeb != 0 {
		t.Error("no commission rate may be invented")
	}
	if a.InstanceURL != "" || a.TenantID != 0 || !a.AESelfRegister || a.DocsDir == "" {
		t.Errorf("unexpected defaults: %+v", a)
	}
}

func TestAccountingEnvAndValidation(t *testing.T) {
	t.Setenv("SEAM_ENABLE_ACCOUNTING", "1")
	t.Setenv("SEAM_ACCOUNTING_ACCOUNTANT_OPERATE", "true")
	t.Setenv("SEAM_ACCOUNTING_COMMISSION_BPS_REAL", "1000")
	t.Setenv("SEAM_ACCOUNTING_TENANT_ID", "3")
	t.Setenv("SEAM_ACCOUNTING_AE_SELF_REGISTER", "0")
	c := Default()
	c.Auth.Enabled = false
	c.LoadFromEnv()
	if !c.Features.EnableAccounting || !c.Accounting.AccountantOperate || c.Accounting.CommissionBPSReal != 1000 ||
		c.Accounting.TenantID != 3 || c.Accounting.AESelfRegister {
		t.Fatalf("env not applied: %+v", c.Accounting)
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, mut := range map[string]func(*Config){
		"bps too high":      func(c *Config) { c.Accounting.CommissionBPSReal = 10001 },
		"negative tenant":   func(c *Config) { c.Accounting.TenantID = -1 },
		"url without key":   func(c *Config) { c.Accounting.InstanceURL = "https://plat.example" },
		"plain http remote": func(c *Config) { c.Accounting.InstanceURL = "http://plat.example"; c.Accounting.InstanceAPIKey = "k" },
		"not a url":         func(c *Config) { c.Accounting.InstanceURL = "ftp://x"; c.Accounting.InstanceAPIKey = "k" },
	} {
		cc := Default()
		cc.Auth.Enabled = false
		cc.Features.EnableAccounting = true
		mut(cc)
		if cc.Validate() == nil {
			t.Errorf("%s: want a validation error", name)
		}
	}
	ok := Default()
	ok.Auth.Enabled = false
	ok.Features.EnableAccounting = true
	ok.Accounting.InstanceURL, ok.Accounting.InstanceAPIKey = "http://127.0.0.1:9000", "k"
	if err := ok.Validate(); err != nil {
		t.Errorf("loopback http must be allowed: %v", err)
	}
}

func TestAccountingUIFlagDefaults(t *testing.T) {
	a := Default().Accounting
	if !a.NotifyPlatform || !a.ShowSyncProblems || !a.Dashboard || a.PageSize != 50 {
		t.Errorf("read-only and convenience flags are on by default: %+v", a)
	}
	if a.StaffEditAEProfiles {
		t.Error("staff must not edit payout details by default")
	}
	if a.AccountantEditFormulas || !a.FormulasOQL {
		t.Errorf("formulas: only the admin edits by default, OQL is allowed: %+v", a)
	}
	t.Setenv("SEAM_ACCOUNTING_STAFF_EDIT_AE", "1")
	t.Setenv("SEAM_ACCOUNTING_NOTIFY_PLATFORM", "0")
	t.Setenv("SEAM_ACCOUNTING_SHOW_SYNC_PROBLEMS", "false")
	t.Setenv("SEAM_ACCOUNTING_DASHBOARD", "false")
	t.Setenv("SEAM_ACCOUNTING_PAGE_SIZE", "25")
	t.Setenv("SEAM_ACCOUNTING_ACCOUNTANT_EDIT_FORMULAS", "1")
	t.Setenv("SEAM_ACCOUNTING_FORMULAS_OQL", "0")
	c := Default()
	c.LoadFromEnv()
	b := c.Accounting
	if !b.AccountantEditFormulas || b.FormulasOQL {
		t.Errorf("formula env not applied: %+v", b)
	}
	if !b.StaffEditAEProfiles || b.NotifyPlatform || b.ShowSyncProblems || b.Dashboard || b.PageSize != 25 {
		t.Errorf("env not applied: %+v", b)
	}
	for _, n := range []int{0, 9, 201} {
		c := Default()
		c.Features.EnableAccounting = true
		c.Auth.Enabled = false
		c.Accounting.PageSize = n
		if c.Validate() == nil {
			t.Errorf("page size %d must be rejected", n)
		}
	}
}
