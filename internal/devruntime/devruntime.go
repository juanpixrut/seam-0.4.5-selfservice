// Package devruntime assembles the Developer module's runtime pieces
// (campaign DEV-7 to DEV-12, DEV-14, DEV-15): the persisted flag state, the
// schema resolver, the filesystem layers for translations and schemas, and
// the background pollers that keep those layers current while hot loading is
// switched on.
//
// It exists so that the server needs only a few lines to wire the module in,
// and so that this glue can be tested without the server. It depends on no
// web or form-engine code: the schema validator is passed in.
package devruntime

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/devstate"
	"github.com/ha1tch/seam-ui/internal/features"
	"github.com/ha1tch/seam-ui/internal/i18n"
	"github.com/ha1tch/seam-ui/internal/overrides"
	"github.com/ha1tch/seam-ui/internal/schemaoverlay"
)

// Options configures Start.
type Options struct {
	// Flags is the resolved feature-flag set (config.Config.Flags()).
	Flags *features.Set
	// Translator receives the filesystem layer and per-tenant options.
	Translator *i18n.Translator
	// Client persists panel state in xolu. Ignored if Store is set.
	Client auth.XOLUClient
	// Store overrides the xolu-backed store (tests, or running without xolu).
	Store devstate.Store
	// Overrides stores the tenant-scope translation and form-definition
	// overrides (internal/overrides). Nil means the xolu-backed store when
	// Client is set, and no tenant-scope overrides at all otherwise.
	Overrides overrides.Store
	// ValidateSchema checks a candidate form schema; in Seam it wraps
	// formengine.ParseSchema. Nil checks JSON syntax only.
	ValidateSchema schemaoverlay.Validator
	// LocalesDir and SchemasDir are the filesystem layers' directories
	// (SEAM_LOCALES_DIR, SEAM_SCHEMAS_DIR); empty switches that layer off.
	LocalesDir string
	SchemasDir string
	// GlobalScope switches on the overrides that apply to every tenant
	// (overrides.Service.AllowGlobal, campaign DEV-26).
	GlobalScope bool
	// SchemaCheck, when set, is what a definition found in SchemasDir must
	// pass to be put in force (schemaoverlay.FileLayer.SetCheck): a file that
	// fails it is set aside and reported, and the rest of the directory loads.
	SchemaCheck func(module, name string, doc []byte) error
	// SyncInterval is how often this instance checks whether another has changed
	// the stored state (the panel's switches and the tenant overrides, campaign
	// DEV-28). Zero means DefaultSyncInterval when the state is kept in xolu
	// (Client is set) and no checking otherwise; negative never checks.
	SyncInterval time.Duration
	// PollInterval is how often the directories are checked while the
	// matching hot-load flag is on. Zero means two seconds.
	PollInterval time.Duration
	// HistoryInterval is how often the directories are looked at for the
	// history of changes made in the files. Zero means the layers' default
	// (250 ms); negative means only when a directory is loaded.
	HistoryInterval time.Duration
	// Watch is how the directories are followed between ticks: "auto" or ""
	// (change events where the build has them), "events" or "poll".
	Watch string
	// Logger receives startup and reload diagnostics. Nil means slog.Default.
	Logger *slog.Logger
}

// DefaultSyncInterval is how often an instance checks for another's changes
// when the state is kept in xolu. Each check reads the switch rows and the
// override entities, which is why it is not every two seconds like the
// directory pollers.
const DefaultSyncInterval = 15 * time.Second

// Runtime is the assembled Developer runtime.
type Runtime struct {
	// Manager holds the persisted runtime flag state.
	Manager *devstate.Manager
	// Resolver resolves form schemas through the override layers.
	Resolver *schemaoverlay.Resolver
	// Locales and Schemas are the filesystem layers; nil when the matching
	// directory is not configured.
	Locales *i18n.FileLayer
	Schemas *schemaoverlay.FileLayer
	// Overrides applies and edits the tenant-scope overrides; nil when there is
	// no store for them.
	Overrides *overrides.Service
}

// Start assembles the runtime and launches the pollers. Pollers stop when ctx
// is cancelled. It returns (nil, nil) when the Developer module is off, so
// that nothing is built and nothing can take effect.
//
// A failure to read persisted state or an initial filesystem layer is logged
// and not fatal: the module starts with empty state or without that layer,
// and the panel shows the error. Start returns an error only for programming
// mistakes (missing flags, translator or store).
func Start(ctx context.Context, o Options) (*Runtime, error) {
	if o.Flags == nil {
		return nil, errors.New("devruntime: Flags is required")
	}
	if !o.Flags.On(features.Developer) {
		return nil, nil
	}
	if o.Translator == nil {
		return nil, errors.New("devruntime: Translator is required")
	}
	store := o.Store
	if store == nil {
		if o.Client == nil {
			return nil, errors.New("devruntime: Client or Store is required")
		}
		store = devstate.NewXoluStore(o.Client)
	}
	log := o.Logger
	if log == nil {
		log = slog.Default()
	}

	rt := &Runtime{
		Manager:  devstate.NewManager(o.Flags, store),
		Resolver: schemaoverlay.New(o.ValidateSchema),
	}
	devstate.Wire(rt.Manager, o.Translator, rt.Resolver)
	if err := rt.Manager.Load(ctx); err != nil {
		log.Error("developer: could not read persisted panel state; starting with defaults", "error", err)
	}

	// Tenant overrides are a decision (DeveloperTenantOverrides, off unless chosen):
	// with the flag off no store is read, written or polled, whatever else is set.
	var ostore overrides.Store
	if o.Flags.On(features.DeveloperTenantOverrides) {
		ostore = o.Overrides
		if ostore == nil && o.Client != nil {
			ostore = overrides.NewXoluStore(o.Client)
		}
	}
	if ostore != nil {
		rt.Overrides = overrides.NewService(ostore, o.Translator, rt.Resolver)
		if o.GlobalScope {
			rt.Overrides.AllowGlobal()
		}
		if err := rt.Overrides.Load(ctx); err != nil {
			log.Error("developer: tenant overrides not (all) loaded; the entities may not exist yet", "error", err)
		}
	}

	if o.LocalesDir != "" {
		rt.Locales = i18n.NewFileLayer(o.Translator, o.LocalesDir)
		if err := rt.Locales.Reload(); err != nil {
			log.Error("developer: translation overrides not loaded", "dir", o.LocalesDir, "error", err)
		}
		rt.Locales.SetHistoryInterval(o.HistoryInterval)
		rt.Locales.SetWatch(o.Watch)
		for _, f := range rt.Locales.Status().Refused {
			log.Error("developer: translation override file set aside", "dir", o.LocalesDir, "reason", f.Reason)
		}
		go rt.Locales.Poll(ctx, o.PollInterval, func() bool { return rt.Manager.AnyOn(features.HotloadI18N) })
	}
	if o.SchemasDir != "" {
		rt.Schemas = schemaoverlay.NewFileLayer(rt.Resolver, o.SchemasDir)
		if check := o.SchemaCheck; check != nil {
			rt.Schemas.SetCheck(func(k schemaoverlay.Key, doc []byte) error { return check(k.Module, k.Name, doc) })
		}
		if err := rt.Schemas.Reload(); err != nil {
			log.Error("developer: form-definition overrides not loaded", "dir", o.SchemasDir, "error", err)
		}
		rt.Schemas.SetHistoryInterval(o.HistoryInterval)
		rt.Schemas.SetWatch(o.Watch)
		for _, f := range rt.Schemas.Status().Refused {
			log.Error("developer: form-definition override file set aside", "dir", o.SchemasDir, "reason", f.Reason)
		}
		go rt.Schemas.Poll(ctx, o.PollInterval, func() bool { return rt.Manager.AnyOn(features.HotloadSchemas) })
	}
	every := o.SyncInterval
	if every == 0 && o.Client != nil {
		every = DefaultSyncInterval
	}
	if !o.Flags.On(features.DeveloperInstanceSync) {
		every = -1
	}
	if every > 0 {
		go rt.sync(ctx, every, log)
	}
	log.Warn("Developer module is active: translation and form-definition overrides can change application behaviour at runtime",
		"locales_dir", o.LocalesDir, "schemas_dir", o.SchemasDir)
	return rt, nil
}

// sync keeps this instance current with the others until ctx is cancelled: on
// every tick it asks the manager and the overrides service to re-read the
// stored state, which they apply only if it changed. A failure is logged once
// for as long as it lasts, not every tick.
func (rt *Runtime) sync(ctx context.Context, every time.Duration, log *slog.Logger) {
	tick := time.NewTicker(every)
	defer tick.Stop()
	last := ""
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		var errs []error
		if _, err := rt.Manager.Sync(ctx); err != nil {
			errs = append(errs, err)
		}
		if rt.Overrides != nil {
			if _, err := rt.Overrides.Sync(ctx); err != nil {
				errs = append(errs, err)
			}
		}
		msg := ""
		if len(errs) > 0 {
			msg = errors.Join(errs...).Error()
		}
		if msg != "" && msg != last {
			log.Warn("developer: could not synchronise with the stored state", "error", msg)
		}
		last = msg
	}
}
