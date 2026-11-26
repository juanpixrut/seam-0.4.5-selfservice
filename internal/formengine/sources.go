package formengine

import "context"

// This file resolves x-seam-source values (FORM_ENGINE_SPEC.md §5) to the
// option lists select-FK widgets render. The SourceResolver interface
// itself lives in engine.go, next to FormEngine.Sources.
//
// An xolu-backed implementation (resolving "entity:brands" etc. against
// real master-entity data) lands with stage 2, once there's a real
// handler wiring the engine to actual entities to resolve against —
// building it against no real caller yet risks the exact kind of
// speculative code the T-01 stages are ordered to avoid.

// StaticSourceResolver resolves a fixed set of sources to fixed option
// lists, configured up front. Not meant for production use — its purpose
// is tests and any stage-1/2 development work that needs a SourceResolver
// before the real one exists.
type StaticSourceResolver map[string][]Option

// Resolve implements SourceResolver.
func (r StaticSourceResolver) Resolve(ctx context.Context, source string) ([]Option, error) {
	opts, ok := r[source]
	if !ok {
		return nil, nil // an unconfigured source resolves to no options, not an error
	}
	return opts, nil
}
