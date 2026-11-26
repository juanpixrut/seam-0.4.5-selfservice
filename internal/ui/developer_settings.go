package ui

import (
	"cmp"
	"strconv"
	"time"

	mi "github.com/ha1tch/minty"
)

// DeveloperSettings are the Developer module's settings that are not on/off,
// as resolved at startup (internal/config, ui_settings.go): each is a decision
// a deployment can take differently, with a default. They are shown on the
// Startup page, read-only, beside the flags; changing one is a restart.
type DeveloperSettings struct {
	// Poll is how often an override directory is loaded while its hot-load
	// switch is on; History how often it is looked at for the history
	// (negative: only when it is loaded); Sync how often this instance checks
	// what another changed (zero or negative: never).
	Poll, History, Sync time.Duration
	// GlobalTenant is the tenant whose developers may edit the global scope
	// (0: nobody).
	GlobalTenant int64
	// SourceDir is the source checkout scanned for unreferenced texts ("": none).
	SourceDir string
	// ModuleAudience is who a module switched on at runtime is on for:
	// ModuleAudienceDevelopers (also "") or ModuleAudienceTenant.
	ModuleAudience string
	// Watch is how the override directories are followed for the history:
	// "events" where the system reports changes, otherwise "poll".
	Watch string
	// Preferences is where a developer's last choices are kept:
	// DeveloperPreferencesAccount (also "") or DeveloperPreferencesBrowser.
	Preferences string
	// CustomPages says a form created with the tools has a page of its own.
	CustomPages bool
}

// The audiences of a module switched on at runtime.
const (
	ModuleAudienceDevelopers = "developers"
	ModuleAudienceTenant     = "tenant"
)

// developerSettingsPanel lists the settings with their value and what each
// decides, and the name it is configured under.
func developerSettingsPanel(b *mi.Builder, s DeveloperSettings, t func(key string, args ...any) string) mi.Node {
	ms := func(d time.Duration) string { return strconv.FormatInt(d.Milliseconds(), 10) + " ms" }
	history := ms(s.History)
	if s.History < 0 {
		history = t("developer.setting.on_load")
	}
	sync := strconv.FormatInt(int64(s.Sync/time.Second), 10) + " s"
	if s.Sync <= 0 {
		sync = t("developer.setting.off")
	}
	global := t("developer.setting.none")
	if s.GlobalTenant > 0 {
		global = strconv.FormatInt(s.GlobalTenant, 10)
	}
	source := s.SourceDir
	if source == "" {
		source = t("developer.setting.none")
	}
	custom := t("developer.setting.off")
	if s.CustomPages {
		custom = t("developer.setting.on")
	}
	rows := []struct{ name, value, meaning string }{
		{"developer_poll_ms", ms(s.Poll), t("developer.setting.poll")},
		{"developer_history_ms", history, t("developer.setting.history")},
		{"developer_sync_seconds", sync, t("developer.setting.sync")},
		{"developer_global_tenant", global, t("developer.setting.global_tenant")},
		{"developer_source_dir", source, t("developer.setting.source_dir")},
		{"developer_module_audience", cmp.Or(s.ModuleAudience, ModuleAudienceDevelopers), t("developer.setting.module_audience")},
		{"developer_watch", cmp.Or(s.Watch, "poll"), t("developer.setting.watch")},
		{"developer_preferences", cmp.Or(s.Preferences, DeveloperPreferencesAccount), t("developer.setting.preferences")},
		{"developer_custom_pages", custom, t("developer.setting.custom_pages")},
	}
	var out []mi.H
	for i, r := range rows {
		r := r
		out = append(out, ListRow(i,
			ListCellRich(func(b *mi.Builder) mi.Node { return b.Span(mi.Class(devMonoClass), r.name) }),
			ListCell(r.value),
			ListCellMuted(r.meaning)))
	}
	return developerPanel(b, "settings", t("developer.startup.settings"), t("developer.startup.settings_note"), nil, true,
		b.Div(mi.ID("dev-settings"), ListTable(ListTableConfig{
			Columns: []ListColumn{{Label: t("developer.startup.setting")}, {Label: t("developer.startup.value")}, {Label: t("developer.startup.meaning")}},
		}, out)(b)))
}
