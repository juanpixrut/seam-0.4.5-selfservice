// Package config provides configuration management for Seam AMS.
package config

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/ha1tch/seam-ui/internal/features"
)

// Duration is a time.Duration that supports JSON unmarshaling from strings.
type Duration time.Duration

// UnmarshalJSON implements json.Unmarshaler for Duration.
// It accepts both string durations ("30s", "5m", "1h") and integer nanoseconds.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}

	switch value := v.(type) {
	case float64:
		*d = Duration(time.Duration(value))
		return nil
	case string:
		dur, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid duration %q: %w", value, err)
		}
		*d = Duration(dur)
		return nil
	default:
		return fmt.Errorf("invalid duration type: %T", v)
	}
}

// MarshalJSON implements json.Marshaler for Duration.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// Duration returns the underlying time.Duration.
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// Config holds all application configuration.
type Config struct {
	// Server settings
	Server ServerConfig `json:"server"`

	// XOLU connection
	XOLU XOLUConfig `json:"xolu"`

	// Authentication
	Auth AuthConfig `json:"auth"`

	// Features
	Features FeatureConfig `json:"features"`

	// Accounting tunes the Accounting & Admin module (features.enable_accounting).
	Accounting AccountingConfig `json:"accounting"`

	// Logging
	Log LogConfig `json:"log"`

	// FeatureFlags is the resolved feature-flag set (see internal/features).
	// LoadFromEnv populates it; Flags() populates it on demand. It is
	// derived, never read from the config file.
	FeatureFlags *features.Set `json:"-"`
	featureErr   error
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host            string   `json:"host"`
	Port            int      `json:"port"`
	ReadTimeout     Duration `json:"read_timeout"`
	WriteTimeout    Duration `json:"write_timeout"`
	ShutdownTimeout Duration `json:"shutdown_timeout"`
	MaxBodySize     int64    `json:"max_body_size"`
	TLSCert         string   `json:"tls_cert"`
	TLSKey          string   `json:"tls_key"`
}

// XOLUConfig holds XOLU server connection settings.
type XOLUConfig struct {
	URL            string   `json:"url"`
	APIKey         string   `json:"api_key"` // Bearer token; required when xolu auth is enabled
	Timeout        Duration `json:"timeout"`
	MaxConnections int      `json:"max_connections"`
	RetryAttempts  int      `json:"retry_attempts"`
	RetryDelay     Duration `json:"retry_delay"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	Enabled              bool     `json:"enabled"`
	JWTSecret            string   `json:"jwt_secret"`
	TokenExpiry          Duration `json:"token_expiry"`
	RefreshExpiry        Duration `json:"refresh_expiry"`
	APIKeyHeader         string   `json:"api_key_header"`
	SessionSecret        string   `json:"session_secret"`
	SessionDuration      Duration `json:"session_duration"`
	CSRFEnabled          bool     `json:"csrf_enabled"`
	TenantFromClaimsOnly bool     `json:"tenant_from_claims_only"` // Default true: ignore X-Tenant-ID header, derive tenant from claims only
}

// FeatureConfig holds feature flags.
type FeatureConfig struct {
	EnableUI          bool   `json:"enable_ui"`
	EnableAPI         bool   `json:"enable_api"`
	EnableWebhooks    bool   `json:"enable_webhooks"`
	EnableRules       bool   `json:"enable_rules"`
	EnableFSM         bool   `json:"enable_fsm"`
	EnableQR          bool   `json:"enable_qr"`
	QRPrefix          string `json:"qr_prefix"`
	QRBaseURL         string `json:"qr_base_url"`
	QRRenderURL       string `json:"qr_render_url"` // 2D-code renderer base URL (empty = no QR images)
	RateLimitPerMin   int    `json:"rate_limit_per_min"`
	DevMode           bool   `json:"dev_mode"`           // DevMode enables development fallbacks - MUST be false in production
	VendorJS          bool   `json:"vendor_js"`          // VendorJS serves JS deps from /static/vendor/ instead of CDN
	SeamAPIURL        string `json:"seam_api_url"`       // URL of seam-api for runtime state lookups (empty = disabled)
	EnableOperational bool   `json:"enable_operational"` // Register operational UI routes (default: true)
	EnableAdmin       bool   `json:"enable_admin"`       // Register admin UI routes (default: true)
	// EnableWorkOrders gates the Work Orders module (2026-08-04) --
	// deliberately opt-in, default false, unlike EnableOperational/
	// EnableAdmin. This is an explicit playground for the Work Orders
	// team (the tech team's own framing), a real, committed, working slice
	// (form + FSM + xolu commit composition, T-67) proving the
	// pattern -- not yet a scoped, production module (T-11's own five
	// open design questions -- Blocks, Scope, Cuadrilla, Empresa, no
	// module name settled -- remain entirely unresolved). Opt-in keeps
	// it out of any deployment that hasn't deliberately turned it on.
	EnableWorkOrders bool `json:"enable_work_orders"`
	// EnableIssues gates the Issues module (docs/ISSUES_MODULE.md). Opt-in,
	// default false, like EnableWorkOrders: the module is being built by its
	// team, and it stays out of any deployment that has not deliberately
	// turned it on (env SEAM_ENABLE_ISSUES=1). Before it had this flag it
	// was part of every instance.
	EnableIssues bool `json:"enable_issues"`
	// EnableDemo gates the Demo module, where new components and fixes
	// are shown to developers (/demo). Opt-in, default false, like
	// EnableWorkOrders: it stays out of any deployment that has not
	// deliberately turned it on (env SEAM_ENABLE_DEMO=1).
	EnableDemo bool `json:"enable_demo"`

	// EnableAccounting gates the Accounting & Admin module (/accounting):
	// Account Executives, collections, commissions, settlements and the
	// books. Opt-in, default false (env SEAM_ENABLE_ACCOUNTING=1): it holds
	// Seam's own finances, so it stays out of any deployment (notably any
	// multi-tenant one) that has not deliberately turned it on.
	EnableAccounting bool `json:"enable_accounting"`

	// UI component switches. Each is read at startup; the defaults are what Seam
	// ships with (see internal/formengine.Options for the engine-side ones).
	//   searchable_select      select widgets become a searchable combobox (default true)
	//   select_search_min      options needed before the combobox shows a search box (default 8; 0 = default)
	//   reorder_rows           honour x-seam-reorder on repeatables (default true); false is a kill switch
	//   document_links_legacy  keep the previous look of document-links (default false)
	//   row_effects            "scoped" (default, Demo only), "all" or "off"
	//   compress_static        gzip text-like files under /static/ (default true)
	//   developer_sync_seconds seconds between an instance's checks for another's changes to the stored Developer state (default 15; 0 = default; 2 to 3600)
	SearchableSelect     bool   `json:"searchable_select"`
	SelectSearchMin      int    `json:"select_search_min"`
	DeveloperSyncSeconds int    `json:"developer_sync_seconds"`
	ReorderRows          bool   `json:"reorder_rows"`
	DocumentLinksLegacy  bool   `json:"document_links_legacy"`
	RowEffects           string `json:"row_effects"`
	CompressStatic       bool   `json:"compress_static"`

	// The Developer module's other settings that are not on/off (see
	// ui_settings.go for their meaning, defaults and bounds).
	DeveloperPollMillis     int    `json:"developer_poll_ms"`
	DeveloperHistoryMillis  int    `json:"developer_history_ms"`
	DeveloperGlobalTenant   int64  `json:"developer_global_tenant"`
	DeveloperSourceDir      string `json:"developer_source_dir,omitempty"`
	DeveloperModuleAudience string `json:"developer_module_audience,omitempty"`
	DeveloperWatch          string `json:"developer_watch,omitempty"`
	DeveloperCustomPages    bool   `json:"developer_custom_pages"`
	DeveloperPreferences    string `json:"developer_preferences,omitempty"`
	// EnableDeveloper gates the Developer module (i18n and form-definition
	// tooling, control panel). Opt-in (env SEAM_ENABLE_DEVELOPER=1) and it
	// refuses to start unless authentication is enabled.
	EnableDeveloper bool `json:"enable_developer"`
	// LocalesDir and SchemasDir are the Developer module's filesystem override
	// directories (env SEAM_LOCALES_DIR, SEAM_SCHEMAS_DIR). Empty means that
	// layer is off. Only read while EnableDeveloper is on.
	LocalesDir string `json:"locales_dir,omitempty"`
	SchemasDir string `json:"schemas_dir,omitempty"`
	// Flags sets any feature flag by its catalogue key (see
	// internal/features), including runtime flags that have no field of
	// their own. A runtime flag set here is pinned: the Developer panel
	// shows it locked.
	Flags         map[string]bool `json:"flags,omitempty"`
	ForceLanguage string          `json:"force_language"` // Startup-only initial locale (e.g. "en"). Only supplies the default before Settings has ever been saved -- a tenant's saved Settings locale, once it exists, is not overridden by this.
}

// AccountingConfig holds the Accounting & Admin module's settings. Every
// default is the conservative choice: nothing is invented (no commission rate,
// no platform), duties are separated (the accountant keeps the books, staff run
// collections and payments) and the module is open to every tenant only on
// single-tenant installs.
type AccountingConfig struct {
	// DocsDir is where uploaded invoices and payment proofs are kept
	// (env SEAM_ACCOUNTING_DOCS_DIR).
	DocsDir string `json:"docs_dir"`

	// TenantID restricts the module to the users of one tenant (env
	// SEAM_ACCOUNTING_TENANT_ID). 0, the default, means no restriction, which
	// is right for a single-tenant install; on a multi-tenant one set it to
	// the tenant that is Seam itself, since the module's records are not
	// tenant-scoped.
	TenantID int64 `json:"tenant_id"`

	// AESelfRegister lets a user with the account_executive role register
	// itself as an AE on first visit (env SEAM_ACCOUNTING_AE_SELF_REGISTER).
	// Default true: the design is "register, then Seam authorizes", and the
	// authorization is the gate. When false, only an AE created by Seam may use
	// the portal.
	AESelfRegister bool `json:"ae_self_register"`

	// AccountantOperate lets the accountant role also run collections and pay
	// settlements (env SEAM_ACCOUNTING_ACCOUNTANT_OPERATE). Default false:
	// whoever keeps the books does not also move the money.
	AccountantOperate bool `json:"accountant_operate"`

	// StaffPostEntries lets the admin_staff role post and reverse manual ledger
	// entries (env SEAM_ACCOUNTING_STAFF_POST_ENTRIES). Default false: free
	// entries are the accountant's. Closing a month stays with the accountant
	// and the admin either way.
	StaffPostEntries bool `json:"staff_post_entries"`

	// StaffEditAEProfiles lets the admin_staff role edit an AE's profile,
	// payout method and payout details (env SEAM_ACCOUNTING_STAFF_EDIT_AE).
	// Default false: where the money goes is the AE's own statement, and a
	// back-office user who could rewrite it could redirect a payment. The
	// admin role can always do it.
	StaffEditAEProfiles bool `json:"staff_edit_ae_profiles"`

	// AccountantEditFormulas lets the accountant role create and change formula
	// rules (env SEAM_ACCOUNTING_ACCOUNTANT_EDIT_FORMULAS). Default false: the
	// rules decide what AEs are paid and what prices are, so changing them is
	// the admin's. The accountant and admin_staff can always see them.
	AccountantEditFormulas bool `json:"accountant_edit_formulas"`

	// FormulasOQL allows the OQL method of formula rules (env
	// SEAM_ACCOUNTING_FORMULAS_OQL). Default true; only users who may edit
	// formulas can write one, queries are limited to a single read-only SELECT
	// over the module's own adm_* tables, and values are bound as parameters.
	// Turn it off to allow only percentage and fixed-amount rules; OQL rules
	// already stored are then neither run nor enabled.
	FormulasOQL bool `json:"formulas_oql"`

	// NotifyPlatform tells the platform when an item is marked collected
	// (env SEAM_ACCOUNTING_NOTIFY_PLATFORM). Default true, and it only has
	// an effect when instance_url is set. Turn it off when the platform
	// learns about payments by itself.
	NotifyPlatform bool `json:"notify_platform"`

	// ShowSyncProblems keeps the result of the last pull in memory and shows
	// what staff must look at (unknown AE, closed month, externally paid
	// items) on the Collections page (env SEAM_ACCOUNTING_SHOW_SYNC_PROBLEMS).
	// Default true: it is read-only information that otherwise only reaches
	// the log. It is lost on restart.
	ShowSyncProblems bool `json:"show_sync_problems"`

	// Dashboard shows summary figures on the module's home page for staff
	// (env SEAM_ACCOUNTING_DASHBOARD). Default true: read-only, and each
	// figure appears only to users who may see its source.
	Dashboard bool `json:"dashboard"`

	// PageSize is the number of rows per page in the module's lists (env
	// SEAM_ACCOUNTING_PAGE_SIZE). Default 50, allowed 10 to 200.
	PageSize int `json:"page_size"`

	// InstanceURL and InstanceAPIKey connect the platform instance Admin pulls
	// tenants and billable items from (env SEAM_ACCOUNTING_INSTANCE_URL /
	// SEAM_ACCOUNTING_INSTANCE_KEY). Empty, the default, means not connected:
	// the pull reports it and nothing else changes. The contract is in
	// docs/ACCOUNTING_ADMIN_MODULE.md.
	InstanceURL     string   `json:"instance_url"`
	InstanceAPIKey  string   `json:"instance_api_key"`
	InstanceTimeout Duration `json:"instance_timeout"`

	// Commission rates in basis points (100 = 1%) of the amount collected, per
	// AE kind (env SEAM_ACCOUNTING_COMMISSION_BPS_REAL / _SEAM_AE / _SEAM_WEB).
	// Default 0 for all: the design leaves the formula outside the schema, and
	// a made-up rate would book liabilities that do not exist. All zero means
	// no commission accrues. They are the fallback: as soon as one commission
	// rule is enabled in the Formulas panel, the rules decide and these are
	// ignored. A custom formula can replace this in code
	// (accounting.WithCalculator).
	CommissionBPSReal    int `json:"commission_bps_real"`
	CommissionBPSSeamAE  int `json:"commission_bps_seam_ae"`
	CommissionBPSSeamWeb int `json:"commission_bps_seam_web"`
}

// defaultAccountingConfig returns the conservative defaults.
func defaultAccountingConfig() AccountingConfig {
	return AccountingConfig{
		DocsDir:          "data/accounting-docs",
		AESelfRegister:   true,
		NotifyPlatform:   true,
		FormulasOQL:      true,
		ShowSyncProblems: true,
		Dashboard:        true,
		PageSize:         50,
		InstanceTimeout:  Duration(15 * time.Second),
	}
}

func envBool(name string, dst *bool) {
	if v := os.Getenv(name); v != "" {
		*dst = v == "true" || v == "1"
	}
}

func envInt(name string, dst *int) {
	if v := os.Getenv(name); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			*dst = n
		}
	}
}

func (a *AccountingConfig) applyEnv() {
	if v := os.Getenv("SEAM_ACCOUNTING_DOCS_DIR"); v != "" {
		a.DocsDir = v
	}
	if v := os.Getenv("SEAM_ACCOUNTING_TENANT_ID"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			a.TenantID = n
		}
	}
	envBool("SEAM_ACCOUNTING_AE_SELF_REGISTER", &a.AESelfRegister)
	envBool("SEAM_ACCOUNTING_ACCOUNTANT_OPERATE", &a.AccountantOperate)
	envBool("SEAM_ACCOUNTING_STAFF_POST_ENTRIES", &a.StaffPostEntries)
	envBool("SEAM_ACCOUNTING_STAFF_EDIT_AE", &a.StaffEditAEProfiles)
	envBool("SEAM_ACCOUNTING_ACCOUNTANT_EDIT_FORMULAS", &a.AccountantEditFormulas)
	envBool("SEAM_ACCOUNTING_FORMULAS_OQL", &a.FormulasOQL)
	envBool("SEAM_ACCOUNTING_NOTIFY_PLATFORM", &a.NotifyPlatform)
	envBool("SEAM_ACCOUNTING_SHOW_SYNC_PROBLEMS", &a.ShowSyncProblems)
	envBool("SEAM_ACCOUNTING_DASHBOARD", &a.Dashboard)
	envInt("SEAM_ACCOUNTING_PAGE_SIZE", &a.PageSize)
	if v := os.Getenv("SEAM_ACCOUNTING_INSTANCE_URL"); v != "" {
		a.InstanceURL = v
	}
	if v := os.Getenv("SEAM_ACCOUNTING_INSTANCE_KEY"); v != "" {
		a.InstanceAPIKey = v
	}
	envInt("SEAM_ACCOUNTING_COMMISSION_BPS_REAL", &a.CommissionBPSReal)
	envInt("SEAM_ACCOUNTING_COMMISSION_BPS_SEAM_AE", &a.CommissionBPSSeamAE)
	envInt("SEAM_ACCOUNTING_COMMISSION_BPS_SEAM_WEB", &a.CommissionBPSSeamWeb)
}

// validate checks the accounting settings (only when the module is on).
func (a AccountingConfig) validate() error {
	for name, v := range map[string]int{"commission_bps_real": a.CommissionBPSReal,
		"commission_bps_seam_ae": a.CommissionBPSSeamAE, "commission_bps_seam_web": a.CommissionBPSSeamWeb} {
		if v < 0 || v > 10000 {
			return fmt.Errorf("accounting.%s must be between 0 and 10000 basis points, got %d", name, v)
		}
	}
	if a.PageSize < 10 || a.PageSize > 200 {
		return fmt.Errorf("accounting.page_size must be between 10 and 200, got %d", a.PageSize)
	}
	if a.TenantID < 0 {
		return fmt.Errorf("accounting.tenant_id must not be negative")
	}
	if a.InstanceURL != "" {
		u, err := url.Parse(a.InstanceURL)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return fmt.Errorf("accounting.instance_url must be an http(s) URL")
		}
		if u.Scheme == "http" && !isLoopbackHost(u.Hostname()) {
			return fmt.Errorf("accounting.instance_url must use https unless it points at this machine")
		}
		if a.InstanceAPIKey == "" {
			return fmt.Errorf("accounting.instance_api_key is required with accounting.instance_url")
		}
	}
	return nil
}

func isLoopbackHost(h string) bool {
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// LogConfig holds logging settings.
type LogConfig struct {
	Level  string `json:"level"`
	Format string `json:"format"`
	Output string `json:"output"`
}

// Default returns the default configuration.
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Host:            "",
			Port:            8080,
			ReadTimeout:     Duration(30 * time.Second),
			WriteTimeout:    Duration(30 * time.Second),
			ShutdownTimeout: Duration(10 * time.Second),
			MaxBodySize:     10 * 1024 * 1024, // 10MB
		},
		XOLU: XOLUConfig{
			URL:            "http://localhost:9090",
			Timeout:        Duration(30 * time.Second),
			MaxConnections: 100,
			RetryAttempts:  3,
			RetryDelay:     Duration(time.Second),
		},
		Auth: AuthConfig{
			Enabled:              false,
			JWTSecret:            "",
			TokenExpiry:          Duration(24 * time.Hour),
			RefreshExpiry:        Duration(7 * 24 * time.Hour),
			APIKeyHeader:         "X-API-Key",
			SessionSecret:        "", // Must be set in production
			SessionDuration:      Duration(24 * time.Hour),
			CSRFEnabled:          true,
			TenantFromClaimsOnly: true, // Secure default: ignore X-Tenant-ID header
		},
		Features: FeatureConfig{
			EnableUI:          true,
			EnableAPI:         true,
			EnableWebhooks:    true,
			EnableRules:       true,
			EnableFSM:         true,
			EnableQR:          true,
			EnableOperational: true,
			EnableAdmin:       true,
			EnableWorkOrders:  false, // opt-in playground, see field doc comment
			EnableIssues:      false, // opt-in, see field doc comment
			EnableDemo:        false, // opt-in, see field doc comment
			SearchableSelect:  true,
			SelectSearchMin:   8,
			ReorderRows:       true,
			RowEffects:        RowEffectsScoped,
			CompressStatic:    true,
			QRPrefix:          "SEAM",
			QRBaseURL:         "https://seam.app/s/",
			RateLimitPerMin:   0, // 0 = unlimited
		},
		Log: LogConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
		Accounting: defaultAccountingConfig(),
	}
}

// Load loads configuration from a JSON file.
func Load(path string) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, use defaults
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	return cfg, nil
}

// LoadFromEnv loads configuration from environment variables.
// Environment variables override values loaded from file.
func (c *Config) LoadFromEnv() {
	// Server
	if v := os.Getenv("SEAM_HOST"); v != "" {
		c.Server.Host = v
	}
	if v := os.Getenv("SEAM_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Server.Port = port
		}
	}
	if v := os.Getenv("SEAM_TLS_CERT"); v != "" {
		c.Server.TLSCert = v
	}
	if v := os.Getenv("SEAM_TLS_KEY"); v != "" {
		c.Server.TLSKey = v
	}

	// XOLU
	if v := os.Getenv("XOLU_URL"); v != "" {
		c.XOLU.URL = v
	}
	if v := os.Getenv("SEAM_XOLU_API_KEY"); v != "" {
		c.XOLU.APIKey = v
	}

	// Auth
	if v := os.Getenv("SEAM_JWT_SECRET"); v != "" {
		c.Auth.JWTSecret = v
		c.Auth.Enabled = true
	}
	if v := os.Getenv("SEAM_AUTH_ENABLED"); v != "" {
		c.Auth.Enabled = v == "true" || v == "1"
	}

	// Features. Flag environment variables (SEAM_ENABLE_*, SEAM_FEATURES,
	// SEAM_PROFILE) are read by internal/features, which also validates them;
	// a bad value is reported by Validate.
	c.resolveFeatures()
	if v := os.Getenv("SEAM_LOCALES_DIR"); v != "" {
		c.Features.LocalesDir = v
	}
	if v := os.Getenv("SEAM_SCHEMAS_DIR"); v != "" {
		c.Features.SchemasDir = v
	}
	if v := os.Getenv("SEAM_API_URL"); v != "" {
		c.Features.SeamAPIURL = v
	}
	if v := os.Getenv("SEAM_FORCE_LANGUAGE"); v != "" {
		c.Features.ForceLanguage = v
	}
	if v := os.Getenv("SEAM_SELECT_SEARCH_MIN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Features.SelectSearchMin = n
		}
	}
	if v := os.Getenv("SEAM_DEVELOPER_SYNC_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Features.DeveloperSyncSeconds = n
		}
	}
	if v := os.Getenv("SEAM_DEVELOPER_POLL_MS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Features.DeveloperPollMillis = n
		}
	}
	if v := os.Getenv("SEAM_DEVELOPER_HISTORY_MS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Features.DeveloperHistoryMillis = n
		}
	}
	if v := os.Getenv("SEAM_DEVELOPER_GLOBAL_TENANT"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			c.Features.DeveloperGlobalTenant = n
		}
	}
	if v := os.Getenv("SEAM_DEVELOPER_SOURCE_DIR"); v != "" {
		c.Features.DeveloperSourceDir = v
	}
	if v := os.Getenv("SEAM_DEVELOPER_MODULE_AUDIENCE"); v != "" {
		c.Features.DeveloperModuleAudience = v
	}
	if v := os.Getenv("SEAM_DEVELOPER_WATCH"); v != "" {
		c.Features.DeveloperWatch = v
	}
	if v := os.Getenv("SEAM_DEVELOPER_CUSTOM_PAGES"); v != "" {
		c.Features.DeveloperCustomPages = v == "1" || v == "true"
	}
	if v := os.Getenv("SEAM_DEVELOPER_PREFERENCES"); v != "" {
		c.Features.DeveloperPreferences = v
	}
	if v := os.Getenv("SEAM_ROW_EFFECTS"); v != "" {
		c.Features.RowEffects = v
	}
	if v := os.Getenv("SEAM_RATE_LIMIT"); v != "" {
		if limit, err := strconv.Atoi(v); err == nil {
			c.Features.RateLimitPerMin = limit
		}
	}

	// Logging
	if v := os.Getenv("SEAM_LOG_LEVEL"); v != "" {
		c.Log.Level = v
	}
	if v := os.Getenv("SEAM_LOG_FORMAT"); v != "" {
		c.Log.Format = v
	}

	c.Accounting.applyEnv()
}

// Validate checks that the configuration is valid.
func (c *Config) Validate() error {
	if c.featureErr != nil {
		return c.featureErr
	}
	// Defence in depth: features.Resolve enforces this too, but Auth can be
	// changed after LoadFromEnv (and runtime editing of translations and
	// forms must never be reachable without authentication).
	if c.Features.EnableDeveloper && !c.Auth.Enabled {
		return fmt.Errorf("the Developer module (SEAM_ENABLE_DEVELOPER) requires authentication to be enabled")
	}

	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Server.Port)
	}

	if c.XOLU.URL == "" {
		return fmt.Errorf("XOLU URL is required")
	}

	if c.Auth.Enabled && c.Auth.JWTSecret == "" {
		return fmt.Errorf("JWT secret is required when auth is enabled")
	}

	if c.Features.EnableAccounting {
		if err := c.Accounting.validate(); err != nil {
			return err
		}
	}

	if err := c.Features.validate(); err != nil {
		return err
	}

	return nil
}

// Address returns the server address string.
func (c *Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// IsTLS returns true if TLS is configured.
func (c *Config) IsTLS() bool {
	return c.Server.TLSCert != "" && c.Server.TLSKey != ""
}
