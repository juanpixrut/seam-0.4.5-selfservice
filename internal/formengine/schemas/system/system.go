// Package system embeds the formengine JSON Schemas for the System
// module's form-shaped views (MOD-8, MODULE_DASHBOARD_DEVELOPMENT_PLAN.md
// Stage 8): profile, preferences, and -- as the rest of Stage 8 lands --
// password change, settings (general/notifications/features), and user
// create/edit. Per Stage 8's own scoping (docs/MODULE_DASHBOARD_
// DEVELOPMENT_PLAN.md): the form-shaped parts formengine can build,
// not user listing or a permissions/entitlement matrix, neither of
// which formengine or any existing pattern covers -- those stay
// hand-coded.
//
// Same transitional-bootstrap reasoning as internal/formengine/schemas/
// masters (that package's own doc comment): schema authoring ultimately
// belongs to the Settings module's schema designer (T-08) once it
// exists, not files embedded in the Go binary. Embedding is the
// deliberate shortcut until then, matching masters/asset's own
// precedent rather than inventing a new one.
package system

import "embed"

//go:embed profile.json preferences.json settings_general.json settings_notifications.json settings_features.json password.json user_create.json user_edit.json
var schemaFS embed.FS

// Schema returns the raw JSON Schema bytes for one of this module's
// form-shaped views: "profile", "preferences", and more as Stage 8
// continues. Panics on an unknown name -- this is a small, fixed,
// compile-time-known set, not user input.
func Schema(name string) []byte {
	data, err := schemaFS.ReadFile(name + ".json")
	if err != nil {
		panic("system: unknown schema " + name + ": " + err.Error())
	}
	return data
}
