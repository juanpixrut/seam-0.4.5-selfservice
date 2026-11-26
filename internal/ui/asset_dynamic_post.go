package ui

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	qf "github.com/ha1tch/queryfy"
	fe "github.com/ha1tch/seam-ui/internal/formengine"
	asset "github.com/ha1tch/seam-ui/internal/formengine/schemas/asset"
)

// assetIntegerFields are the schema fields converted to int64 before
// validation/persistence -- confirmed against AssetCreate/AssetUpdate's
// real code directly, not asset.json's own "type" declarations. Every
// other scalar field, INCLUDING the six declared "number" in the schema
// (latitude, longitude, purchase_price, current_price, temp_min,
// temp_max), is persisted and read back as a plain string by the real
// handlers throughout their lifecycle -- confirmed identical in
// AssetCreate, AssetUpdate, and AssetEdit's read path. The dynamic form
// matches that real, coexisting convention rather than the schema's
// stated type, since queryfy's own validator accepts string values for
// "number"/"integer" fields either way (confirmed directly: ValidateWithMode
// returned nil for both string and native-numeric submissions against an
// identical schema) -- so there's no validation reason to convert these,
// only a persistence-consistency one, and the real handlers' own choice
// is strings.
var assetIntegerFields = map[string]bool{
	"type_id":        true,
	"brand_id":       true,
	"model_id":       true,
	"location_id":    true,
	"region_id":      true,
	"parent_id":      true,
	"custodian_id":   true,
	"currency_id":    true,
	"cost_center_id": true,
}

// assetArrayItemKeys names, for each of the four array-typed fields,
// the item keys to read from {field}_{item}[] submission arrays --
// confirmed from repeatable.go's own doc comment and asset_xolu_fields.go's
// item-translation tables, not re-derived here.
var assetArrayItemKeys = map[string][]string{
	"loan_links":      {"name", "url", "type"},
	"lifecycle_links": {"name", "url", "type"},
	"document_links":  {"name", "url", "type"},
	"service_configs": {"measure", "threshold", "alert_id"},
}

// parseAssetFormValues reads a submitted Asset form (r.Form must
// already be populated via r.ParseForm) into an English-keyed values
// map matching asset.json's field names -- the same shape
// AssetValuesToXolu expects, and the same shape FormEngine.Render
// expects for re-rendering with errors.
func parseAssetFormValues(r *http.Request) map[string]interface{} {
	values := make(map[string]interface{})
	for name := range assetFieldToXolu {
		if _, isArray := assetArrayItemKeys[name]; isArray {
			continue
		}
		raw := r.FormValue(name)
		if assetIntegerFields[name] {
			values[name] = toInt64FromString(raw)
			continue
		}
		values[name] = raw
	}
	for fieldName, itemKeys := range assetArrayItemKeys {
		values[fieldName] = parseAssetArrayField(r, fieldName, itemKeys)
	}
	return values
}

// parseAssetArrayField reads {field}_{item}[] submission arrays and
// zips them into row maps, skipping rows whose first item key is empty
// (the same "blank row = not provided" convention the static form's
// own svc_medida loop uses).
func parseAssetArrayField(r *http.Request, fieldName string, itemKeys []string) []interface{} {
	if len(itemKeys) == 0 {
		return nil
	}

	formValues := r.PostForm
	if len(formValues) == 0 {
		formValues = r.Form
	}

	columns := make([][]string, len(itemKeys))
	for i, item := range itemKeys {
		columns[i] = formValues[fieldName+"_"+item+"[]"]
	}

	var rows []interface{}
	for i, first := range columns[0] {
		if strings.TrimSpace(first) == "" {
			continue
		}
		row := make(map[string]interface{}, len(itemKeys))
		for j, item := range itemKeys {
			var val interface{} = ""
			if i < len(columns[j]) {
				val = columns[j][i]
			}

			if item == "required" {
				strVal, _ := val.(string)
				strVal = strings.ToLower(strings.TrimSpace(strVal))
				row[item] = (strVal == "on" || strVal == "true" || strVal == "1")
			} else {
				row[item] = val
			}
		}
		rows = append(rows, row)
	}
	return rows
}

// validateAssetValues validates values against asset.json's compiled
// schema (queryfy.Loose mode, matching the static form's own
// validateForm). Returns both error forms RenderContext now supports,
// built from the same queryfy result: an exact-match map (unchanged
// from before FieldStatus existed, "_" for the top-level/empty-path
// case) for scalar field paths, and a []fe.FieldStatus slice for paths
// queryfy itself produces with "[" or "." in them -- confirmed
// empirically that nested object-in-array constraints (e.g.
// document_links[0].name) actually look like this, not something a
// flat map keyed by field name could ever have matched. Both nil when
// valid. filterEmpty matches validateForm's own convention: empty
// strings and zero int64s mean "not provided" to an HTML form, so
// Optional constraints aren't falsely triggered and Required fields
// correctly produce "field is required" instead of a type mismatch.
func validateAssetValues(e *fe.FormEngine, rc fe.RenderContext, values map[string]interface{}, compiled qf.Schema) (map[string]string, []fe.FieldStatus) {
	filtered := make(map[string]interface{}, len(values))
	for k, v := range values {
		switch val := v.(type) {
		case string:
			if val == "" {
				continue
			}
		case int64:
			if val == 0 {
				continue
			}
		}
		filtered[k] = v
	}

	err := qf.ValidateWithMode(filtered, compiled, qf.Loose)
	if err == nil {
		return nil, nil
	}
	var ve *qf.ValidationError
	if !errors.As(err, &ve) {
		return map[string]string{"_": err.Error()}, nil
	}
	errs := make(map[string]string, len(ve.Errors))
	var statuses []fe.FieldStatus
	for _, fieldErr := range ve.Errors {
		path := fieldErr.Path
		if path == "" {
			path = "_"
		}
		message := fe.ResolveQueryfyError(e, rc, fieldErr.Message)
		if strings.ContainsAny(path, "[.") {
			statuses = append(statuses, fe.FieldStatus{Element: path, Status: "ERR", Message: message})
			continue
		}
		if _, exists := errs[path]; exists {
			continue // first error per path wins, matching validateForm
		}
		errs[path] = message
	}
	return errs, statuses
}

// AssetCreateDynamic handles POST /assets/dynamic.
func (h *UIHandler) AssetCreateDynamic(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	t := h.translateFunc(r)
	if err := r.ParseForm(); err != nil {
		h.page.RenderError(w, http.StatusBadRequest, t("error.bad_request"), t("error.invalid_form_data"))
		return
	}

	values := parseAssetFormValues(r)

	form, err := fe.ParseSchema(resolveSchema(r.Context(), "asset", "asset", asset.Schema()))
	if err != nil {
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.schema_error")+err.Error())
		return
	}

	e := &fe.FormEngine{Sources: &AssetSourceResolver{Client: h.xoluClient}, Translator: h.formEngineTranslator()}
	rc := fe.RenderContext{CSRFToken: h.GetCSRFToken(w, r), Locale: h.currentLocale()}

	rc.IsPartial = IsHTMXRequest(r)
	rc.Action = "/assets/dynamic"
	rc.Method = "post"

	postErrs, postStatuses := validateAssetValues(e, rc, values, form.Compiled)
	if postErrs != nil || postStatuses != nil {
		rc.Statuses = postStatuses
		content := e.Render(ctx, resolveSchema(ctx, "asset", "asset", asset.Schema()), values, postErrs, rc)
		wrapped := wrapAssetDynamicForm(h.translateFunc(r), content, "/assets/dynamic", "post", rc.IsPartial, false, 0, "")
		if rc.IsPartial {
			h.page.RenderPartial(w, wrapped)
		} else {
			h.page.Render(w, r, t("asset.new"), "/assets", wrapped)
		}
		return
	}

	xoluData := AssetValuesToXolu(values)
	created, err := h.xoluClient.Create(ctx, "assets", xoluData)
	if err != nil {
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.asset_create_failed")+err.Error())
		return
	}

	// "Create and add photos": photos can only be attached to a record that exists, so instead of
	// going back to the list, answer with the saved asset's edit form, opened on the Map & photos
	// tab. The plain Create button does not send _then and keeps going back to the list.
	if IsHTMXRequest(r) && r.FormValue("_then") == "photos" && created != nil {
		r2 := r.Clone(ctx)
		r2.Method = http.MethodGet
		r2.URL.Path = fmt.Sprintf("/assets/dynamic/%d/edit", created.ID)
		r2.URL.RawQuery = "tab=tab-geo"
		// tells the page the list behind the modal is stale (SeamModal reloads it on close)
		w.Header().Set("HX-Trigger", fmt.Sprintf(`{"seam:record-created":{"entity":"asset","id":%d}}`, created.ID))
		h.AssetEditDynamic(w, r2)
		return
	}

	if IsHTMXRequest(r) {
		HTMXRedirect(w, "/assets")
	} else {
		http.Redirect(w, r, "/assets", http.StatusSeeOther)
	}
}

// AssetUpdateDynamic handles POST /assets/dynamic/{id}.
func (h *UIHandler) AssetUpdateDynamic(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	t := h.translateFunc(r)
	id := extractPathID(r.URL.Path, "/assets/dynamic/")
	if id == 0 {
		h.page.RenderError(w, 404, t("error.not_found"), t("asset.not_found"))
		return
	}

	if err := r.ParseForm(); err != nil {
		h.page.RenderError(w, http.StatusBadRequest, t("error.bad_request"), t("error.invalid_form_data"))
		return
	}

	values := parseAssetFormValues(r)

	// SchemaForUpdate specifically for validation -- name/type_id/
	// criticality become optional on update, mirroring the static
	// form's own AssetCreateSchema/AssetUpdateSchema split (the tech team,
	// 2026-07-29). Rendering below still uses asset.Schema()
	// unmodified, so the visual "*" indicator is unaffected.
	// derived from the definition in force, so an override of the asset form
	// is what an update is checked against too
	updateSchema, err := asset.SchemaForUpdateOf(resolveSchema(ctx, "asset", "asset", asset.Schema()))
	if err != nil {
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.schema_error")+err.Error())
		return
	}
	form, err := fe.ParseSchema(updateSchema)
	if err != nil {
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.schema_error")+err.Error())
		return
	}

	e := &fe.FormEngine{Sources: &AssetSourceResolver{Client: h.xoluClient}, Translator: h.formEngineTranslator()}
	rc := fe.RenderContext{CSRFToken: h.GetCSRFToken(w, r), Locale: h.currentLocale()}

	action := fmt.Sprintf("/assets/dynamic/%d", id)
	rc.IsPartial = IsHTMXRequest(r)
	rc.IsEdit = true
	rc.EntityID = id
	rc.EntityType = "asset"
	rc.Action = action
	rc.Method = "post"

	postErrs, postStatuses := validateAssetValues(e, rc, values, form.Compiled)
	if postErrs != nil || postStatuses != nil {
		rc.Statuses = postStatuses
		content := e.Render(ctx, resolveSchema(ctx, "asset", "asset", asset.Schema()), values, postErrs, rc)
		wrapped := wrapAssetDynamicForm(h.translateFunc(r), content, action, "post", rc.IsPartial, true, id, assetNameFromValues(values))
		if rc.IsPartial {
			h.page.RenderPartial(w, wrapped)
		} else {
			h.page.Render(w, r, t("asset.edit"), "/assets", wrapped)
		}
		return
	}

	xoluData := AssetValuesToXolu(values)
	// Patch, not Update -- HTTP PUT full-replace vs HTTP PATCH real
	// merge, confirmed directly against the live server (see users.go's
	// identical 2026-08-03 fix). Unlike Profile/Alerts/asset_types etc.,
	// this specific form genuinely submits every one of Asset's fields
	// (all four tabs share one <form>, confirmed live: an unchanged
	// resubmission round-tripped every original field correctly) -- so
	// Update was not currently destructive here. Patched anyway for
	// defense-in-depth: if Asset's schema ever gains a field the form
	// doesn't render yet, Update would silently wipe it on every edit;
	// Patch degrades that to "the new field doesn't update," not
	// "everything else gets deleted."
	if _, err := h.xoluClient.Patch(ctx, "assets", id, xoluData); err != nil {
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.asset_update_failed")+err.Error())
		return
	}

	if IsHTMXRequest(r) {
		HTMXRedirect(w, "/assets")
	} else {
		http.Redirect(w, r, "/assets", http.StatusSeeOther)
	}
}
