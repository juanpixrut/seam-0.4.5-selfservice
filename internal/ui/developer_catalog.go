package ui

import (
	"context"
	"regexp"
	"sort"
	"strings"

	asset "github.com/ha1tch/seam-ui/internal/formengine/schemas/asset"
	consumableschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/consumable"
	crewschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/crew"
	developerschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/developer"
	issueschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/issues"
	masterschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/masters"
	movementschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/movement"
	outsourcedschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/outsourced"
	sensorschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/sensors"
	systemschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/system"
	unitschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/unit"
	warehouseschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/warehouse"
	actionschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/wo_action"
	workorderschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/workorders"
	workordertemplateschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/workorders_templates"
)

// The catalogue of form definitions the Developer module can browse and
// override: every embedded schema of every module whose pages render through
// resolveSchema. The Demo module is left out on purpose (its examples are
// fixed, see schema_resolve.go), so an override of a demo form would have no
// effect and the browser must not offer one. The Developer module's own forms
// are in the catalogue: the tools can be edited with themselves.

// developerSchemaRef names one form definition.
type developerSchemaRef struct{ Module, Name string }

func (k developerSchemaRef) String() string { return k.Module + "/" + k.Name }

var developerSchemaModules = []struct {
	module string
	names  func() []string
	get    func(name string) []byte
}{
	{"asset", asset.Names, func(string) []byte { return asset.Schema() }},
	{"consumable", consumableschemas.Names, consumableschemas.Schema},
	{"crew", crewschemas.Names, crewschemas.Schema},
	{"developer", developerschemas.Names, developerschemas.Schema},
	{"issues", issueschemas.Names, issueschemas.Schema},
	{"masters", masterschemas.Names, masterschemas.Schema},
	{"movement", movementschemas.Names, movementschemas.Schema},
	{"outsourced", outsourcedschemas.Names, outsourcedschemas.Schema},
	{"sensors", sensorschemas.Names, sensorschemas.Schema},
	{"system", systemschemas.Names, systemschemas.Schema},
	{"unit", unitschemas.Names, unitschemas.Schema},
	{"warehouse", warehouseschemas.Names, warehouseschemas.Schema},
	{"wo_action", actionschemas.Names, actionschemas.Schema},
	{"workorders", workorderschemas.Names, workorderschemas.Schema},
	{"workorders_templates", workordertemplateschemas.Names, workordertemplateschemas.Schema},
}

// developerSchemaPackagesLeftOut are the directories under
// internal/formengine/schemas that are deliberately not in the catalogue, with
// the reason. A test fails when a directory is in neither list, so a new schema
// package cannot be forgotten (T-116).
var developerSchemaPackagesLeftOut = map[string]string{
	"demo": "its examples are fixed; the Demo page does not go through resolveSchema",
}

// developerStarterForm is the definition a new form starts with: one text
// field, so the designer and the preview have something to show.
const developerStarterForm = `{
  "type": "object",
  "properties": {
    "name": {
      "type": "string",
      "title": "Name",
      "x-seam-input": "text"
    }
  }
}`

// developerNameRE is what a module or a form name may be: the same plain
// identifier the override layers accept (schemaoverlay.CheckDocument).
var developerNameRE = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// developerForm resolves "module/name" to a form the tools can open. A form is
// either built in (its embedded definition is the base an override is compared
// with) or custom: one created with the tools, which exists only as an
// override, for the tenant or in the files, and whose base is the starter
// definition. A custom form is known while an override of it is visible to the
// tenant, or while the tenant's history still holds a version of it, so a
// removed one can be restored. Nothing else is accepted, so the parameter can
// never name a path.
func (h *UIHandler) developerForm(ctx context.Context, tenant int64, s string) (ref developerSchemaRef, base []byte, custom, ok bool) {
	module, name, cut := strings.Cut(s, "/")
	if !cut {
		return developerSchemaRef{}, nil, false, false
	}
	ref = developerSchemaRef{module, name}
	if doc, found := developerEmbeddedSchema(module, name); found {
		return ref, doc, false, true
	}
	if !developerNameRE.MatchString(module) || !developerNameRE.MatchString(name) || len(module) > 64 || len(name) > 64 {
		return developerSchemaRef{}, nil, false, false
	}
	rt := h.config.DevRuntime
	if rt == nil {
		return developerSchemaRef{}, nil, false, false
	}
	if rt.Resolver != nil {
		for _, k := range rt.Resolver.Overridden(tenant) {
			if k.Module == module && k.Name == name {
				return ref, []byte(developerStarterForm), true, true
			}
		}
	}
	if rt.Overrides != nil && tenant > 0 {
		if hist, err := rt.Overrides.History(ctx, tenant, module, name); err == nil && len(hist) > 0 {
			return ref, []byte(developerStarterForm), true, true
		}
	}
	// a file that is in the directory but was set aside can be opened and mended
	if rt.Schemas != nil {
		if _, there, err := rt.Schemas.Read(module, name); err == nil && there {
			return ref, []byte(developerStarterForm), true, true
		}
	}
	return developerSchemaRef{}, nil, false, false
}

// developerCustomForms lists the forms that exist only as overrides visible to
// the tenant, sorted.
func (h *UIHandler) developerCustomForms(tenant int64) []developerSchemaRef {
	rt := h.config.DevRuntime
	if rt == nil || rt.Resolver == nil {
		return nil
	}
	var out []developerSchemaRef
	for _, k := range rt.Resolver.Overridden(tenant) {
		if _, found := developerEmbeddedSchema(k.Module, k.Name); !found {
			out = append(out, developerSchemaRef{k.Module, k.Name})
		}
	}
	return out
}

// developerHasModule reports whether the catalogue has a module of that name.
func developerHasModule(name string) bool {
	for _, m := range developerSchemaModules {
		if m.module == name {
			return true
		}
	}
	return false
}

// developerEmbeddedSchemas lists every form definition, sorted.
func developerEmbeddedSchemas() []developerSchemaRef {
	var out []developerSchemaRef
	for _, m := range developerSchemaModules {
		for _, n := range m.names() {
			out = append(out, developerSchemaRef{m.module, n})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

// developerEmbeddedSchema returns the embedded document for module/name, and
// false when no such form exists. It never panics on a name from the outside
// (the packages' own Schema functions do).
func developerEmbeddedSchema(module, name string) ([]byte, bool) {
	for _, m := range developerSchemaModules {
		if m.module != module {
			continue
		}
		for _, n := range m.names() {
			if n == name {
				return m.get(name), true
			}
		}
	}
	return nil, false
}
