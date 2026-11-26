package features

// Flag keys. Code asks for flags by these constants, never by string
// literal, so a typo is a compile error rather than a silent false.
const (
	UI          = "ui"
	API         = "api"
	Webhooks    = "webhooks"
	Rules       = "rules"
	FSM         = "fsm"
	QR          = "qr"
	Operational = "operational"
	Admin       = "admin"
	WorkOrders  = "work_orders"
	Issues      = "issues"
	Demo        = "demo"
	DevMode     = "dev_mode"
	VendorJS    = "vendor_js"
	Accounting  = "accounting"

	// Visual-component switches (read by internal/formengine and the server).
	SearchableSelect    = "searchable_select"
	ReorderRows         = "reorder_rows"
	DocumentLinksLegacy = "document_links_legacy"
	CompressStatic      = "compress_static"

	// Developer is the master switch for the Developer module. While it is
	// off, every runtime developer flag below is inert.
	Developer = "developer"

	// Runtime developer flags, editable from the Developer panel.
	HotloadI18N       = "hotload_i18n"
	HotloadSchemas    = "hotload_schemas"
	OverlayFilesystem = "overlay_filesystem"
	OverlayXolu       = "overlay_xolu"
	ShowI18NKeys      = "show_i18n_keys"

	// Runtime module switches (DEV-27): a module that is off at startup can
	// be switched on for one tenant from the Developer panel. Every module
	// with a start-time flag of the preview or dev class has one.
	ModuleDemo       = "module_demo"
	ModuleWorkOrders = "module_work_orders"
	ModuleIssues     = "module_issues"
)

// Decisions of the Developer module, each a flag so that a deployment can choose
// differently. Every one needs the Developer module on, and every one is of the
// developer class, so the production profile refuses to switch them on (and the
// module itself). The default of each is reasoned:
//
//	DeveloperViewAs          on. It can only lower privilege, only for a developer, only
//	                         while the module is on; it is how roles get tested; and the
//	                         notice that it is active is always shown.
//	DeveloperTenantOverrides off, on in the development profile. It changes what a tenant's
//	                         real users see and every instance reads its entities, so a
//	                         deployment opts in. Its entities and the sync between instances
//	                         have been run against a live xolu (T-106, 2026-11-17: two
//	                         instances; texts, forms and switches), so that is no longer a
//	                         reason to keep it off.
//	DeveloperInstanceSync    on. Instances that disagree about stored overrides are a
//	                         correctness hazard and a check every 15 seconds is cheap.
//	DeveloperChangeNotice    on. It stops translation keys, or a layer switched off, being
//	                         mistaken for a defect. Off is for demos and screenshots.
//	DeveloperProtectOwnRole  on. It prevents locking the module out of its only developer.
//	                         Off is for throwaway sandboxes.
//	DeveloperFormDesigner    off, on in the development profile. New and incomplete.
//	DesignerLivePreview      on. Cheap, and what makes the designer usable; off is for
//	                         forms so large that rendering after every change is slow.
//	DeveloperModuleSwitches  on. It lets the panel switch the Demo, Work Orders and Issues modules on
//	                         for one tenant without a restart. Nothing changes until someone
//	                         uses a switch; a module on at startup stays on for everyone. Off
//	                         leaves those modules' routes unmounted unless their own flag is on.
//
// Not flags, on purpose: the tenant of an override always comes from the session,
// file overrides outrank a tenant's, and file and database overrides share their
// validation. Those are invariants, not choices.
const (
	DeveloperViewAs          = "developer_view_as"
	DeveloperTenantOverrides = "developer_tenant_overrides"
	DeveloperInstanceSync    = "developer_instance_sync"
	DeveloperChangeNotice    = "developer_change_notice"
	DeveloperProtectOwnRole  = "developer_protect_own_role"
	DeveloperFormDesigner    = "developer_form_designer"
	DesignerLivePreview      = "designer_live_preview"
	DeveloperModuleSwitches  = "developer_module_switches"
)

// DefaultOf is the catalogue's default for a flag: what it is when nothing sets
// it, for code that runs without a resolved flag set (tests, hand-built configs).
func DefaultOf(key string) bool {
	for _, f := range catalogFlags() {
		if f.Key == key {
			return f.Default
		}
	}
	return false
}

// Profile names.
const (
	ProfileProduction  = "production"
	ProfileDevelopment = "development"
	ProfileDemo        = "demo"
)

// Catalog returns Seam's flag registry. The first twelve flags are the
// boolean fields of config.FeatureConfig, with the same environment
// variable names, config keys and defaults they have always had; the
// rest belong to the Developer module.
//
// It panics if the catalogue is internally inconsistent, which would be a
// programming error caught by the package tests.
func Catalog() *Registry {
	r, err := NewRegistry(catalogFlags(), catalogProfiles())
	if err != nil {
		panic("features: invalid catalogue: " + err.Error())
	}
	return r
}

func catalogFlags() []Flag {
	return []Flag{
		{Key: UI, EnvVar: "SEAM_ENABLE_UI", ConfigKey: "enable_ui", Default: true,
			Description: "Serve the web UI."},
		{Key: API, EnvVar: "SEAM_ENABLE_API", ConfigKey: "enable_api", Default: true,
			Description: "Serve the REST API."},
		{Key: Webhooks, ConfigKey: "enable_webhooks", Default: true,
			Description: "Run the webhook service."},
		{Key: Rules, ConfigKey: "enable_rules", Default: true,
			Description: "Enable the rules engine."},
		{Key: FSM, ConfigKey: "enable_fsm", Default: true,
			Description: "Enable finite-state-machine support."},
		{Key: QR, ConfigKey: "enable_qr", Default: true,
			Description: "Enable QR code generation."},
		{Key: Operational, EnvVar: "SEAM_ENABLE_OPERATIONAL", ConfigKey: "enable_operational", Default: true,
			Description: "Register the operational UI routes."},
		{Key: Admin, EnvVar: "SEAM_ENABLE_ADMIN", ConfigKey: "enable_admin", Default: true,
			Description: "Register the admin UI routes."},
		{Key: WorkOrders, EnvVar: "SEAM_ENABLE_WORK_ORDERS", ConfigKey: "enable_work_orders", Class: ClassPreview,
			Description: "Work Orders module (opt-in playground)."},
		// Issues follows the lifecycle Work Orders follows (docs/ISSUES_MODULE.md):
		// off unless a deployment asks for it, while the module is being built.
		{Key: Issues, EnvVar: "SEAM_ENABLE_ISSUES", ConfigKey: "enable_issues", Class: ClassPreview,
			Description: "Issues module (opt-in while it is being built)."},
		{Key: Demo, EnvVar: "SEAM_ENABLE_DEMO", ConfigKey: "enable_demo", Class: ClassDev,
			Description: "Demo module showing new components and fixes to developers."},
		{Key: DevMode, ConfigKey: "dev_mode", Class: ClassDev,
			Description: "Development fallbacks in the UI. Must be off in production."},
		{Key: VendorJS, ConfigKey: "vendor_js",
			Description: "Serve JavaScript dependencies from /static/vendor/ instead of a CDN."},
		{Key: Accounting, EnvVar: "SEAM_ENABLE_ACCOUNTING", ConfigKey: "enable_accounting",
			Description: "Accounting & Admin module (/accounting)."},
		{Key: SearchableSelect, EnvVar: "SEAM_SEARCHABLE_SELECT", ConfigKey: "searchable_select", Default: true,
			Description: "Turn select widgets into a searchable combobox (the native select stays the form control)."},
		{Key: ReorderRows, EnvVar: "SEAM_REORDER_ROWS", ConfigKey: "reorder_rows", Default: true,
			Description: "Honour x-seam-reorder on repeatables. Off is a kill switch; schemas stay untouched."},
		{Key: DocumentLinksLegacy, EnvVar: "SEAM_DOCUMENT_LINKS_LEGACY", ConfigKey: "document_links_legacy",
			Description: "Keep the previous hand-coded look of the document-links widget."},
		{Key: CompressStatic, EnvVar: "SEAM_COMPRESS_STATIC", ConfigKey: "compress_static", Default: true,
			Description: "Gzip text-like files under /static/."},

		{Key: Developer, EnvVar: "SEAM_ENABLE_DEVELOPER", ConfigKey: "enable_developer",
			Class: ClassDev, RequiresAuth: true,
			Description: "Developer module: i18n and form-definition tooling and the control panel. Requires authentication."},

		{Key: HotloadI18N, Class: ClassDev, Mode: ModeRuntime, Requires: []string{Developer},
			Description: "Reload translation overrides without a restart."},
		{Key: HotloadSchemas, Class: ClassDev, Mode: ModeRuntime, Requires: []string{Developer},
			Description: "Reload form-definition overrides without a restart."},
		{Key: OverlayFilesystem, Default: true, Class: ClassDev, Mode: ModeRuntime, Requires: []string{Developer},
			Description: "Apply filesystem overrides (switch off to test against the other layers)."},
		{Key: OverlayXolu, Default: true, Class: ClassDev, Mode: ModeRuntime, Requires: []string{Developer},
			Description: "Apply xolu overrides (switch off to test against the embedded base)."},
		{Key: ShowI18NKeys, Class: ClassDev, Mode: ModeRuntime, Requires: []string{Developer},
			Description: "Show translation keys instead of translated text."},

		{Key: DeveloperViewAs, EnvVar: "SEAM_DEVELOPER_VIEW_AS", Default: true, Class: ClassDev, Requires: []string{Developer},
			Description: "Let a developer view the application as a user with a lower role."},
		{Key: DeveloperTenantOverrides, EnvVar: "SEAM_DEVELOPER_TENANT_OVERRIDES", Class: ClassDev, Requires: []string{Developer},
			Description: "Store translation and form-definition overrides per tenant in xolu and apply them."},
		{Key: DeveloperInstanceSync, EnvVar: "SEAM_DEVELOPER_INSTANCE_SYNC", Default: true, Class: ClassDev, Requires: []string{Developer},
			Description: "Check periodically for changes other instances made to the stored switches and overrides."},
		{Key: DeveloperChangeNotice, EnvVar: "SEAM_DEVELOPER_CHANGE_NOTICE", Default: true, Class: ClassDev, Requires: []string{Developer},
			Description: "Show every page of a tenant a notice while a switch that changes what its users see is on."},
		{Key: DeveloperProtectOwnRole, EnvVar: "SEAM_DEVELOPER_PROTECT_OWN_ROLE", Default: true, Class: ClassDev, Requires: []string{Developer},
			Description: "Refuse a developer removing their own developer role."},
		{Key: DeveloperFormDesigner, EnvVar: "SEAM_DEVELOPER_FORM_DESIGNER", Class: ClassDev, Requires: []string{Developer},
			Description: "Form designer: build and change form definitions with a visual editor."},
		{Key: DesignerLivePreview, EnvVar: "SEAM_DESIGNER_LIVE_PREVIEW", Default: true, Class: ClassDev, Requires: []string{DeveloperFormDesigner},
			Description: "Render the form beside the designer after every change."},
		{Key: DeveloperModuleSwitches, EnvVar: "SEAM_DEVELOPER_MODULE_SWITCHES", Default: true, Class: ClassDev, Requires: []string{Developer},
			Description: "Let the Developer panel switch the Demo, Work Orders and Issues modules on for a tenant at runtime."},
		// all three are honoured only while DeveloperModuleSwitches is on (the
		// gate is where the modules are mounted and asked for access)
		{Key: ModuleDemo, Class: ClassDev, Mode: ModeRuntime, Requires: []string{Developer},
			Description: "Demo module for this tenant, when it is not on at startup."},
		{Key: ModuleWorkOrders, Class: ClassDev, Mode: ModeRuntime, Requires: []string{Developer},
			Description: "Work Orders module for this tenant, when it is not on at startup."},
		{Key: ModuleIssues, Class: ClassDev, Mode: ModeRuntime, Requires: []string{Developer},
			Description: "Issues module for this tenant, when it is not on at startup."},
	}
}

func catalogProfiles() []Profile {
	return []Profile{
		{Name: ProfileProduction, Forbid: []Class{ClassDev}},
		{Name: ProfileDevelopment, Defaults: map[string]bool{Demo: true, Developer: true, DeveloperTenantOverrides: true, DeveloperFormDesigner: true}},
		{Name: ProfileDemo, Defaults: map[string]bool{Demo: true}},
	}
}
