package config

import (
	"fmt"
	"strings"
)

// Row-effects modes (FeatureConfig.RowEffects): where repeatable rows grow in
// and shrink out. "scoped" (the default) limits it to elements marked
// data-fe-fx, today the Demo page; "all" turns it on everywhere; "off" nowhere.
const (
	RowEffectsScoped = "scoped"
	RowEffectsAll    = "all"
	RowEffectsOff    = "off"
)

// maxSelectSearchMin bounds FeatureConfig.SelectSearchMin.
const maxSelectSearchMin = 10000

// The Developer module's settings that are not on/off. Each is a decision a
// deployment can take differently; 0 (or "") always means the default.
//
//	developer_poll_ms        SEAM_DEVELOPER_POLL_MS. How often an override directory is
//	                         checked and loaded while its hot-load switch is on. Default
//	                         2000: an edit shows within about two seconds and the check is
//	                         a directory listing.
//	developer_history_ms     SEAM_DEVELOPER_HISTORY_MS. How often an override directory is
//	                         looked at for the history of changes made in the files, whatever
//	                         the hot-load switch says. Default 250: two saves of one file are
//	                         two versions unless they are closer than that. -1 looks only when
//	                         the directory is loaded. The directories are polled, not watched
//	                         through file-system events: that would be exact, at the price of
//	                         a dependency and of behaviour that differs between Linux, macOS
//	                         and network file systems; a shorter interval is the cheap way to
//	                         get closer.
//	developer_global_tenant  SEAM_DEVELOPER_GLOBAL_TENANT. The tenant whose developers may
//	                         edit overrides that apply to every tenant (the global scope).
//	                         Default 0: nobody, the global scope is not offered.
//	developer_source_dir     SEAM_DEVELOPER_SOURCE_DIR. A checkout of the source tree the
//	                         Translations section may scan for texts no code refers to.
//	                         Default empty: the list is not offered (a deployed binary has no
//	                         source beside it).
//	developer_module_audience SEAM_DEVELOPER_MODULE_AUDIENCE. Who a module switched on from
//	                         the panel is on for: "developers" (default), the tenant's
//	                         developers only, since the switch is how a module the
//	                         deployment has not enabled gets tried; or "tenant", every user
//	                         of the tenant with access to it.
//	developer_watch          SEAM_DEVELOPER_WATCH. How the override directories are followed
//	                         for the history: "auto" (default) uses the system's change
//	                         events where this build has them (Linux) and looks every
//	                         developer_history_ms elsewhere; "events" and "poll" force one.
//	                         Events make two quick saves two versions; the polling look
//	                         stays as the safety net either way.
//	developer_custom_pages   SEAM_DEVELOPER_CUSTOM_PAGES. Whether a form created with the
//	                         tools gets a page of its own, /forms/<module>/<name>, whose
//	                         submissions are kept. Default false: it is a page the tenant's
//	                         users can reach, so a deployment opts in.
//	developer_preferences    SEAM_DEVELOPER_PREFERENCES. Where a developer's last choices
//	                         are kept: "account" (default), with the user, so they follow
//	                         the developer to another browser; or "browser", a cookie only.
const (
	minDeveloperPollMillis    = 100
	maxDeveloperPollMillis    = 60000
	minDeveloperHistoryMillis = 20
)

// The bounds of FeatureConfig.DeveloperSyncSeconds (0 means the default, 15).
const (
	minDeveloperSyncSeconds = 2
	maxDeveloperSyncSeconds = 3600
)

// NormalizeRowEffects maps any input to one of the three modes; anything
// unrecognised (including empty) is the default, "scoped".
func NormalizeRowEffects(s string) string {
	switch v := strings.ToLower(strings.TrimSpace(s)); v {
	case RowEffectsAll, RowEffectsOff:
		return v
	}
	return RowEffectsScoped
}

// validate checks the UI feature switches. SelectSearchMin 0 means "unset"
// (the component falls back to its default of 8); negative or absurd values are
// refused at startup rather than silently clamped.
func (f FeatureConfig) validate() error {
	if f.SelectSearchMin < 0 || f.SelectSearchMin > maxSelectSearchMin {
		return fmt.Errorf("features.select_search_min must be between 1 and %d (0 = default), got %d", maxSelectSearchMin, f.SelectSearchMin)
	}
	if n := f.DeveloperSyncSeconds; n != 0 && (n < minDeveloperSyncSeconds || n > maxDeveloperSyncSeconds) {
		return fmt.Errorf("features.developer_sync_seconds must be between %d and %d (0 = default), got %d", minDeveloperSyncSeconds, maxDeveloperSyncSeconds, n)
	}
	if n := f.DeveloperPollMillis; n != 0 && (n < minDeveloperPollMillis || n > maxDeveloperPollMillis) {
		return fmt.Errorf("features.developer_poll_ms must be between %d and %d (0 = default), got %d", minDeveloperPollMillis, maxDeveloperPollMillis, n)
	}
	if n := f.DeveloperHistoryMillis; n != 0 && n != -1 && (n < minDeveloperHistoryMillis || n > maxDeveloperPollMillis) {
		return fmt.Errorf("features.developer_history_ms must be between %d and %d (0 = default, -1 = only when the directory is loaded), got %d", minDeveloperHistoryMillis, maxDeveloperPollMillis, n)
	}
	switch f.DeveloperModuleAudience {
	case "", "developers", "tenant":
	default:
		return fmt.Errorf("features.developer_module_audience must be \"developers\" or \"tenant\", got %q", f.DeveloperModuleAudience)
	}
	switch f.DeveloperWatch {
	case "", "auto", "events", "poll":
	default:
		return fmt.Errorf("features.developer_watch must be \"auto\", \"events\" or \"poll\", got %q", f.DeveloperWatch)
	}
	switch f.DeveloperPreferences {
	case "", "account", "browser":
	default:
		return fmt.Errorf("features.developer_preferences must be \"account\" or \"browser\", got %q", f.DeveloperPreferences)
	}
	if f.DeveloperGlobalTenant < 0 {
		return fmt.Errorf("features.developer_global_tenant must be a tenant ID, or 0 for none, got %d", f.DeveloperGlobalTenant)
	}
	switch strings.ToLower(strings.TrimSpace(f.RowEffects)) {
	case "", RowEffectsScoped, RowEffectsAll, RowEffectsOff:
	default:
		return fmt.Errorf("features.row_effects must be %q, %q or %q, got %q", RowEffectsScoped, RowEffectsAll, RowEffectsOff, f.RowEffects)
	}
	return nil
}
