package config

import (
	"fmt"

	"github.com/ha1tch/seam-ui/internal/features"
)

// featureFields maps each catalogue flag that has a FeatureConfig field to
// that field. Flags without a field (the runtime developer flags) live only
// in the resolved Set.
func (c *Config) featureFields() map[string]*bool {
	return map[string]*bool{
		features.UI:          &c.Features.EnableUI,
		features.API:         &c.Features.EnableAPI,
		features.Webhooks:    &c.Features.EnableWebhooks,
		features.Rules:       &c.Features.EnableRules,
		features.FSM:         &c.Features.EnableFSM,
		features.QR:          &c.Features.EnableQR,
		features.Operational: &c.Features.EnableOperational,
		features.Admin:       &c.Features.EnableAdmin,
		features.WorkOrders:  &c.Features.EnableWorkOrders,
		features.Issues:      &c.Features.EnableIssues,
		features.Demo:        &c.Features.EnableDemo,
		features.DevMode:     &c.Features.DevMode,
		features.VendorJS:    &c.Features.VendorJS,
		features.Accounting:  &c.Features.EnableAccounting,
		features.Developer:   &c.Features.EnableDeveloper,

		features.SearchableSelect:    &c.Features.SearchableSelect,
		features.ReorderRows:         &c.Features.ReorderRows,
		features.DocumentLinksLegacy: &c.Features.DocumentLinksLegacy,
		features.CompressStatic:      &c.Features.CompressStatic,
	}
}

// resolveFeatures runs the flag catalogue over the current configuration
// and the process environment, then writes the result back into the
// FeatureConfig booleans so the rest of the code keeps reading them as
// before.
//
// The FeatureConfig struct is treated as the config-file layer: a field
// whose value differs from the catalogue default counts as set in config.
// (A field equal to its default behaves identically whether set or not.)
// That also keeps hand-built Config values, as used in tests, intact. The
// generic Flags map then overlays it, and environment variables overlay
// both.
//
// On failure the struct is left untouched and the error is held for
// Validate to report, so one startup attempt lists every mistake.
func (c *Config) resolveFeatures() {
	reg := features.Catalog()
	fields := c.featureFields()

	layer := make(map[string]bool)
	for _, f := range reg.Flags() {
		if p, ok := fields[f.Key]; ok && *p != f.Default {
			layer[f.Key] = *p
		}
	}
	for k, v := range c.Features.Flags {
		layer[k] = v
	}

	set, err := reg.Resolve(features.Inputs{Config: layer, AuthEnabled: c.Auth.Enabled})
	if err != nil {
		c.featureErr = fmt.Errorf("feature flags: %w", err)
		return
	}
	for key, p := range fields {
		*p = set.On(key)
	}
	c.FeatureFlags = set
	c.featureErr = nil
}

// Flags returns the resolved feature-flag set, resolving it first if
// LoadFromEnv has not run. It returns nil if resolution failed; Validate
// reports why.
func (c *Config) Flags() *features.Set {
	if c.FeatureFlags == nil && c.featureErr == nil {
		c.resolveFeatures()
	}
	return c.FeatureFlags
}
