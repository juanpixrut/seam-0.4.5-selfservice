// Package features resolves Seam's deployment feature flags.
//
// It replaces the habit of one hand-written environment block per flag
// (each accepting only the literal strings "true" and "1", so that TRUE,
// yes and on silently meant false) with a single declarative table, one
// strict parser, and one place where precedence and safety constraints
// are enforced.
//
// Flags are resolved once at startup into an immutable Set. Precedence,
// lowest to highest:
//
//  1. the flag's own default
//  2. the profile selected by SEAM_PROFILE (if any)
//  3. the config file's features block
//  4. the flag's individual environment variable (e.g. SEAM_ENABLE_DEMO)
//  5. the SEAM_FEATURES list (e.g. "developer,demo,-work_orders")
//
// A flag is either static (fixed for the life of the process) or runtime
// (may additionally be changed from the Developer control panel; see
// Set.Runtime). A runtime flag whose value was fixed by the config file,
// an environment variable or SEAM_FEATURES is pinned: the panel shows it
// as locked and cannot change it.
//
// The package depends only on the standard library.
package features

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Environment variables read by Resolve in addition to each flag's own.
const (
	// EnvProfile selects a profile by name (e.g. "production").
	EnvProfile = "SEAM_PROFILE"
	// EnvList is a comma-separated list of flags to switch on, or off
	// with a leading "-" (e.g. "developer,demo,-work_orders").
	EnvList = "SEAM_FEATURES"
	// EnvAllowDev overrides a profile's ban on development-class flags.
	EnvAllowDev = "SEAM_ALLOW_DEV_IN_PRODUCTION"
)

// Class describes how risky or mature a flag is. Profiles may forbid
// whole classes.
type Class int

const (
	// ClassStable flags are ordinary, supported switches.
	ClassStable Class = iota
	// ClassPreview flags enable features that are opt-in while maturing.
	ClassPreview
	// ClassDev flags enable development tooling that must not be on in
	// production.
	ClassDev
)

// String returns the class name.
func (c Class) String() string {
	switch c {
	case ClassStable:
		return "stable"
	case ClassPreview:
		return "preview"
	case ClassDev:
		return "dev"
	}
	return fmt.Sprintf("class(%d)", int(c))
}

// Mode says whether a flag can change while the process runs.
type Mode int

const (
	// ModeStatic flags are fixed at startup. Routes are registered once
	// and net/http's ServeMux cannot unregister them, so most module
	// flags are static.
	ModeStatic Mode = iota
	// ModeRuntime flags may also be changed from the Developer panel.
	ModeRuntime
)

// String returns the mode name.
func (m Mode) String() string {
	switch m {
	case ModeStatic:
		return "static"
	case ModeRuntime:
		return "runtime"
	}
	return fmt.Sprintf("mode(%d)", int(m))
}

// Flag declares one boolean feature flag.
type Flag struct {
	// Key is the flag's identity: lower-case letters, digits and
	// underscores, starting with a letter. It is what SEAM_FEATURES and
	// Set.On use.
	Key string
	// EnvVar is the individual environment variable, or "" for none.
	EnvVar string
	// ConfigKey is the JSON key in the config file's features block, or
	// "" if the flag cannot be set there. Informational here; the config
	// package maps it into Inputs.Config.
	ConfigKey string
	// Default is the value when nothing else sets the flag.
	Default bool
	// Description is one line of human-readable documentation.
	Description string
	Class       Class
	Mode        Mode
	// RequiresAuth makes it a startup error for the flag to be on while
	// authentication is disabled.
	RequiresAuth bool
	// Requires lists flags that must be on for this one to take effect.
	// Explicitly switching this flag on, away from its default, while a
	// prerequisite is off is a startup error; switching it off never is, so a
	// standing "=0" in a deployment's environment survives the module being
	// turned off. At runtime, Set.Runtime treats the flag as inert (its
	// default) while a prerequisite is off.
	Requires []string
}

// Profile is a named bundle of defaults plus a safety rule.
type Profile struct {
	Name string
	// Defaults overrides flag defaults (precedence level 2).
	Defaults map[string]bool
	// Forbid lists classes that may not be switched on while this profile
	// is active, unless EnvAllowDev is set.
	Forbid []Class
}

// Registry is the validated, immutable set of flag and profile
// declarations.
type Registry struct {
	flags    []Flag
	index    map[string]int
	profiles map[string]Profile
}

var (
	keyPattern     = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	envPattern     = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
	profilePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
)

// NewRegistry validates the declarations and returns a Registry. All
// problems found are reported together.
func NewRegistry(flags []Flag, profiles []Profile) (*Registry, error) {
	r := &Registry{
		flags:    make([]Flag, len(flags)),
		index:    make(map[string]int, len(flags)),
		profiles: make(map[string]Profile, len(profiles)),
	}
	var errs []error
	envs := map[string]string{}
	cfgKeys := map[string]string{}

	for i, f := range flags {
		f.Requires = append([]string(nil), f.Requires...)
		r.flags[i] = f

		if !keyPattern.MatchString(f.Key) {
			errs = append(errs, fmt.Errorf("flag #%d: invalid key %q (want lower-case letters, digits and underscores)", i+1, f.Key))
			continue
		}
		if _, dup := r.index[f.Key]; dup {
			errs = append(errs, fmt.Errorf("flag %q declared more than once", f.Key))
			continue
		}
		r.index[f.Key] = i

		if f.EnvVar != "" {
			if !envPattern.MatchString(f.EnvVar) {
				errs = append(errs, fmt.Errorf("flag %q: invalid environment variable name %q", f.Key, f.EnvVar))
			} else if other, dup := envs[f.EnvVar]; dup {
				errs = append(errs, fmt.Errorf("flag %q: environment variable %s already used by flag %q", f.Key, f.EnvVar, other))
			} else {
				envs[f.EnvVar] = f.Key
			}
		}
		if f.ConfigKey != "" {
			if other, dup := cfgKeys[f.ConfigKey]; dup {
				errs = append(errs, fmt.Errorf("flag %q: config key %q already used by flag %q", f.Key, f.ConfigKey, other))
			} else {
				cfgKeys[f.ConfigKey] = f.Key
			}
		}
	}

	// Requires is checked after every key is known, so declaration order
	// does not matter.
	for _, f := range r.flags {
		for _, req := range f.Requires {
			switch {
			case req == f.Key:
				errs = append(errs, fmt.Errorf("flag %q requires itself", f.Key))
			default:
				if _, ok := r.index[req]; !ok {
					errs = append(errs, fmt.Errorf("flag %q requires unknown flag %q", f.Key, req))
				}
			}
		}
	}

	for _, p := range profiles {
		if !profilePattern.MatchString(p.Name) {
			errs = append(errs, fmt.Errorf("profile: invalid name %q", p.Name))
			continue
		}
		if _, dup := r.profiles[p.Name]; dup {
			errs = append(errs, fmt.Errorf("profile %q declared more than once", p.Name))
			continue
		}
		cp := Profile{
			Name:     p.Name,
			Defaults: make(map[string]bool, len(p.Defaults)),
			Forbid:   append([]Class(nil), p.Forbid...),
		}
		for k, v := range p.Defaults {
			if _, ok := r.index[k]; !ok {
				errs = append(errs, fmt.Errorf("profile %q sets unknown flag %q", p.Name, k))
				continue
			}
			cp.Defaults[k] = v
		}
		r.profiles[p.Name] = cp
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return r, nil
}

// Flags returns the declarations in declaration order.
func (r *Registry) Flags() []Flag {
	out := make([]Flag, len(r.flags))
	for i, f := range r.flags {
		f.Requires = append([]string(nil), f.Requires...)
		out[i] = f
	}
	return out
}

// Lookup returns the declaration for key.
func (r *Registry) Lookup(key string) (Flag, bool) {
	i, ok := r.index[key]
	if !ok {
		return Flag{}, false
	}
	f := r.flags[i]
	f.Requires = append([]string(nil), f.Requires...)
	return f, true
}

// ProfileNames returns the declared profile names, sorted.
func (r *Registry) ProfileNames() []string {
	names := make([]string, 0, len(r.profiles))
	for n := range r.profiles {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// ParseBool parses a boolean flag value strictly. It accepts true/false,
// 1/0, yes/no and on/off in any letter case, ignoring surrounding space,
// and rejects everything else (the old behaviour was to treat anything but
// "true" or "1" as false without complaint).
func ParseBool(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	}
	return false, fmt.Errorf("invalid boolean %q (use true/false, 1/0, yes/no or on/off)", s)
}

// ListItem is one entry of a SEAM_FEATURES list.
type ListItem struct {
	Key string
	On  bool
}

// ParseList parses a SEAM_FEATURES value: comma-separated flag keys, each
// optionally prefixed "+" (on, the default) or "-" (off). Empty entries
// are ignored. Keys are lower-cased.
func ParseList(s string) ([]ListItem, error) {
	var items []ListItem
	var errs []error
	for _, raw := range strings.Split(s, ",") {
		tok := strings.ToLower(strings.TrimSpace(raw))
		if tok == "" {
			continue
		}
		on := true
		switch tok[0] {
		case '-':
			on = false
			tok = strings.TrimSpace(tok[1:])
		case '+':
			tok = strings.TrimSpace(tok[1:])
		}
		if !keyPattern.MatchString(tok) {
			errs = append(errs, fmt.Errorf("invalid flag name %q in list", strings.TrimSpace(raw)))
			continue
		}
		items = append(items, ListItem{Key: tok, On: on})
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return items, nil
}
