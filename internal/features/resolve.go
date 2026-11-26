package features

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
)

// Source says which layer decided a flag's value.
type Source int

const (
	// SourceDefault means the flag's own default.
	SourceDefault Source = iota
	// SourceProfile means the selected profile.
	SourceProfile
	// SourceConfig means the config file's features block.
	SourceConfig
	// SourceEnv means the flag's individual environment variable.
	SourceEnv
	// SourceList means the SEAM_FEATURES list.
	SourceList
	// SourcePanel means a value persisted from the Developer panel. It is
	// only ever returned by Set.Runtime.
	SourcePanel
)

// String returns the source name.
func (s Source) String() string {
	switch s {
	case SourceDefault:
		return "default"
	case SourceProfile:
		return "profile"
	case SourceConfig:
		return "config"
	case SourceEnv:
		return "env"
	case SourceList:
		return "list"
	case SourcePanel:
		return "panel"
	}
	return fmt.Sprintf("source(%d)", int(s))
}

// Inputs carries everything Resolve reads from outside the registry.
type Inputs struct {
	// Config holds flag values from the config file, keyed by flag Key
	// (not ConfigKey). Only flags actually present in the file belong
	// here, so that "not set" stays distinguishable from "set to false".
	Config map[string]bool
	// LookupEnv reads the environment. Nil means os.LookupEnv.
	LookupEnv func(name string) (string, bool)
	// AuthEnabled reports whether authentication is on, for RequiresAuth.
	AuthEnabled bool
}

// Set is the immutable result of resolution. It is safe for concurrent
// use.
type Set struct {
	reg     *Registry
	values  map[string]bool
	sources map[string]Source
	profile string
}

// State is one flag's resolved state, for display.
type State struct {
	Flag   Flag
	Value  bool
	Source Source
	// Pinned is true for a runtime flag whose value was fixed at startup
	// by config, environment or list, so the panel cannot change it.
	Pinned bool
}

// Resolve applies the precedence rules and safety constraints. All
// problems found are reported together, so one startup attempt shows
// every mistake.
func (r *Registry) Resolve(in Inputs) (*Set, error) {
	lookup := in.LookupEnv
	if lookup == nil {
		lookup = os.LookupEnv
	}
	getenv := func(name string) string {
		v, ok := lookup(name)
		if !ok {
			return ""
		}
		return strings.TrimSpace(v)
	}

	var errs []error
	values := make(map[string]bool, len(r.flags))
	sources := make(map[string]Source, len(r.flags))
	for _, f := range r.flags {
		values[f.Key] = f.Default
		sources[f.Key] = SourceDefault
	}

	// 2. Profile.
	var prof *Profile
	profName := strings.ToLower(getenv(EnvProfile))
	if profName != "" {
		p, ok := r.profiles[profName]
		if !ok {
			errs = append(errs, fmt.Errorf("%s: unknown profile %q (known: %s)",
				EnvProfile, profName, strings.Join(r.ProfileNames(), ", ")))
		} else {
			prof = &p
			for k, v := range p.Defaults {
				values[k] = v
				sources[k] = SourceProfile
			}
		}
	}

	// 3. Config file.
	cfgKeys := make([]string, 0, len(in.Config))
	for k := range in.Config {
		cfgKeys = append(cfgKeys, k)
	}
	sort.Strings(cfgKeys)
	for _, k := range cfgKeys {
		if _, ok := r.index[k]; !ok {
			errs = append(errs, fmt.Errorf("config: unknown feature flag %q", k))
			continue
		}
		values[k] = in.Config[k]
		sources[k] = SourceConfig
	}

	// 4. Individual environment variables.
	for _, f := range r.flags {
		if f.EnvVar == "" {
			continue
		}
		raw := getenv(f.EnvVar)
		if raw == "" {
			continue
		}
		b, err := ParseBool(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", f.EnvVar, err))
			continue
		}
		values[f.Key] = b
		sources[f.Key] = SourceEnv
	}

	// 5. SEAM_FEATURES list.
	if raw := getenv(EnvList); raw != "" {
		items, err := ParseList(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", EnvList, err))
		} else {
			for _, it := range items {
				if _, ok := r.index[it.Key]; !ok {
					errs = append(errs, fmt.Errorf("%s: unknown feature flag %q", EnvList, it.Key))
					continue
				}
				values[it.Key] = it.On
				sources[it.Key] = SourceList
			}
		}
	}

	// Safety constraints.
	allowDev := false
	if raw := getenv(EnvAllowDev); raw != "" {
		b, err := ParseBool(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", EnvAllowDev, err))
		} else {
			allowDev = b
		}
	}
	for _, f := range r.flags {
		if !values[f.Key] {
			continue
		}
		if f.RequiresAuth && !in.AuthEnabled {
			errs = append(errs, fmt.Errorf("flag %q requires authentication to be enabled", f.Key))
		}
		explicit := sources[f.Key] >= SourceConfig
		if explicit && values[f.Key] != f.Default {
			for _, req := range f.Requires {
				if !values[req] {
					errs = append(errs, fmt.Errorf("flag %q requires flag %q to be on", f.Key, req))
				}
			}
		}
		// Only a dev-class flag switched on beyond its baseline counts; a
		// flag that is on by default is ordinary behaviour.
		if prof != nil && !allowDev && !f.Default {
			for _, c := range prof.Forbid {
				if f.Class == c {
					errs = append(errs, fmt.Errorf("profile %q forbids %s-class flag %q (set %s=1 to override)",
						prof.Name, c, f.Key, EnvAllowDev))
				}
			}
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return &Set{reg: r, values: values, sources: sources, profile: profName}, nil
}

func (s *Set) mustFlag(key string) Flag {
	f, ok := s.reg.Lookup(key)
	if !ok {
		// Keys are compile-time constants; an unknown one is a
		// programming error, and failing loudly beats silently
		// returning false for a typo. Same precedent as the schema
		// packages' Schema(name).
		panic("features: unknown flag " + fmt.Sprintf("%q", key))
	}
	return f
}

// On reports the startup value of a flag. It panics on an unknown key.
// For a runtime flag this is only the startup value; use Runtime for the
// effective one.
func (s *Set) On(key string) bool {
	s.mustFlag(key)
	return s.values[key]
}

// Source reports which layer decided the flag's startup value.
func (s *Set) Source(key string) Source {
	s.mustFlag(key)
	return s.sources[key]
}

// Profile returns the active profile name, or "".
func (s *Set) Profile() string { return s.profile }

// Pinned reports whether key is a runtime flag whose startup value was
// fixed by config, environment or list.
func (s *Set) Pinned(key string) bool {
	f := s.mustFlag(key)
	if f.Mode != ModeRuntime {
		return false
	}
	switch s.sources[key] {
	case SourceConfig, SourceEnv, SourceList:
		return true
	}
	return false
}

// Runtime returns the effective value of a runtime flag, given the value
// persisted from the Developer panel (nil if none). Order:
//
//  1. If a prerequisite flag (Requires) is off, the flag is inert and its
//     default applies. This is what makes stale persisted panel state
//     harmless when the Developer module is switched off.
//  2. If the flag is pinned at startup, the pinned value applies.
//  3. Otherwise the panel value, if any.
//  4. Otherwise the startup value (default or profile).
//
// It panics if key is unknown or not a runtime flag.
func (s *Set) Runtime(key string, panel *bool) (bool, Source) {
	f := s.mustFlag(key)
	if f.Mode != ModeRuntime {
		panic("features: flag " + fmt.Sprintf("%q", key) + " is not a runtime flag")
	}
	for _, req := range f.Requires {
		if !s.values[req] {
			return f.Default, SourceDefault
		}
	}
	if s.Pinned(key) {
		return s.values[key], s.sources[key]
	}
	if panel != nil {
		return *panel, SourcePanel
	}
	return s.values[key], s.sources[key]
}

// States returns every flag's startup state in declaration order.
func (s *Set) States() []State {
	out := make([]State, 0, len(s.reg.flags))
	for _, f := range s.reg.Flags() {
		out = append(out, State{
			Flag:   f,
			Value:  s.values[f.Key],
			Source: s.sources[f.Key],
			Pinned: s.Pinned(f.Key),
		})
	}
	return out
}

// Log writes the resolved flags to l: one summary line, plus one warning
// for each development-class flag that is on beyond its default.
func (s *Set) Log(l *slog.Logger) {
	var enabled, overridden []string
	for _, st := range s.States() {
		if st.Value {
			enabled = append(enabled, st.Flag.Key)
		}
		if st.Source != SourceDefault {
			overridden = append(overridden, fmt.Sprintf("%s=%t(%s)", st.Flag.Key, st.Value, st.Source))
		}
		if st.Value && !st.Flag.Default && st.Flag.Class == ClassDev {
			l.Warn("development-class feature flag enabled; do not use in production",
				"flag", st.Flag.Key, "source", st.Source.String())
		}
	}
	l.Info("feature flags resolved",
		"profile", s.profile,
		"enabled", strings.Join(enabled, ","),
		"overridden", strings.Join(overridden, ","))
}
