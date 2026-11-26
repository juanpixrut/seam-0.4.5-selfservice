package server

import (
	"time"

	"github.com/ha1tch/seam-ui/internal/devruntime"
	"github.com/ha1tch/seam-ui/internal/dirwatch"
	"github.com/ha1tch/seam-ui/internal/features"
	"github.com/ha1tch/seam-ui/internal/i18n"
	"github.com/ha1tch/seam-ui/internal/ui"
)

// developerSettings resolves the Developer module's settings that are not
// on/off to the values in force (a zero in the configuration is the default),
// for the Startup page.
func (s *Server) developerSettings() ui.DeveloperSettings {
	f := s.cfg.Features
	out := ui.DeveloperSettings{
		Poll:         time.Duration(f.DeveloperPollMillis) * time.Millisecond,
		History:      time.Duration(f.DeveloperHistoryMillis) * time.Millisecond,
		Sync:         time.Duration(f.DeveloperSyncSeconds) * time.Second,
		GlobalTenant: f.DeveloperGlobalTenant,
		SourceDir:    f.DeveloperSourceDir,

		ModuleAudience: f.DeveloperModuleAudience,
		Watch:          dirwatch.Mode(f.DeveloperWatch),
		Preferences:    f.DeveloperPreferences,
		CustomPages:    f.DeveloperCustomPages,
	}
	if out.Poll == 0 {
		out.Poll = 2 * time.Second
	}
	if out.History == 0 {
		out.History = i18n.HistoryInterval
	}
	if out.History > out.Poll {
		out.History = out.Poll
	}
	if out.Sync == 0 {
		out.Sync = devruntime.DefaultSyncInterval
	}
	if !s.cfg.Flags().On(features.DeveloperInstanceSync) {
		out.Sync = 0
	}
	return out
}
