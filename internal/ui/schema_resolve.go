package ui

import (
	"context"
	"sync/atomic"

	"github.com/ha1tch/seam-ui/internal/auth"
	fe "github.com/ha1tch/seam-ui/internal/formengine"
	"github.com/ha1tch/seam-ui/internal/schemaoverlay"
)

// The Developer module's form-definition overrides (internal/schemaoverlay)
// reach the UI through resolveSchema: every place that renders or parses an
// embedded schema passes it through here, so an override of the same
// module/name, for the request's tenant, replaces the embedded document.
//
// The resolver is process-wide, installed once at startup by the server
// (SetSchemaResolver) and only when the Developer module is on. With no
// resolver installed resolveSchema returns the embedded schema unchanged, so a
// deployment without the module pays one atomic load per render.
//
// Module and name are the schema's package directory and file name under
// internal/formengine/schemas, for example ("system", "user_edit").
var schemaResolver atomic.Pointer[schemaoverlay.Resolver]

// SetSchemaResolver installs the resolver, or removes it with nil.
// With a resolver the engine's renders are observed too, so the Developer tools
// can name the translation keys each form's page asks for (developer_trace.go).
func SetSchemaResolver(r *schemaoverlay.Resolver) {
	schemaResolver.Store(r)
	if r == nil {
		fe.SetRenderObserver(nil)
		return
	}
	fe.SetRenderObserver(developerTrace.observe)
}

// resolveSchema returns the override for module/name, or base.
func resolveSchema(ctx context.Context, module, name string, base []byte) []byte {
	r := schemaResolver.Load()
	if r == nil {
		return base
	}
	var tenant int64
	if claims, ok := auth.ClaimsFromContext(ctx); ok {
		tenant = claims.TenantID
	}
	doc := r.Resolve(tenant, module, name, base)
	developerTrace.noteDoc(module, name, doc)
	return doc
}
