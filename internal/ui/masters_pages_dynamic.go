package ui

// masters_pages_dynamic.go — formengine-backed equivalents of the static
// Region/Moneda/Marca form pages in masters_pages.go, per
// docs/FORMENGINE_DEVELOPMENT_PLAN.md stage 2 (FE-07).
//
// The live /regions, /currencies and /brands routes render their fields with
// these since 2026-11-17 (T-113): the handlers call h.regionFormPage,
// h.monedaFormPage and h.marcaFormPage, which render the form-engine fields
// for the request (its tenant's overrides, its locale) and hand them to the
// page functions of masters_pages.go, which still compose the envelope around
// them (title, error line, CSRF field, buttons, and the modal wrapper for a
// partial). The *FormPageDynamic functions below are the earlier equivalence
// proof (FE-07, FE-08): they render the same fields, in the same order, with
// the same required/value/error state, as the hand-built rows, checked by
// TestMastersFormParity in masters_pages_dynamic_test.go. The hand-built rows
// stay in the page functions for a caller that supplies no fields.

import (
	"context"
	"net/http"

	mi "github.com/ha1tch/minty"

	"github.com/ha1tch/seam-ui/internal/formengine"
	masterschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/masters"
)

// mastersEngine is shared across all three adapters -- none of the
// stage-2 schemas use x-seam-source, so a nil SourceResolver is correct
// today, not a shortcut. Revisit if a later master schema needs one.
var mastersEngine = &formengine.FormEngine{}

func RegionFormPageDynamic(data RegionFormData) mi.H {
	values := map[string]interface{}{"description": data.Description}
	rc := formengine.RenderContext{
		Module: "masters", Form: "region",
		IsEdit: data.IsEdit, IsPartial: data.IsPartial,
		CSRFToken: data.CSRFToken,
	}
	return mastersEngine.Render(context.Background(), resolveSchema(context.Background(), "masters", "region", masterschemas.Schema("region")), values, data.Errors, rc)
}

func MonedaFormPageDynamic(data MonedaFormData) mi.H {
	values := map[string]interface{}{
		"iso_code": data.IsoCode,
		"symbol":   data.Symbol,
	}
	rc := formengine.RenderContext{
		Module: "masters", Form: "moneda",
		IsEdit: data.IsEdit, IsPartial: data.IsPartial,
		CSRFToken: data.CSRFToken,
	}
	return mastersEngine.Render(context.Background(), resolveSchema(context.Background(), "masters", "moneda", masterschemas.Schema("moneda")), values, data.Errors, rc)
}

func MarcaFormPageDynamic(data MarcaFormData) mi.H {
	values := map[string]interface{}{
		"code": data.Code,
		"name": data.Name,
	}
	rc := formengine.RenderContext{
		Module: "masters", Form: "marca",
		IsEdit: data.IsEdit, IsPartial: data.IsPartial,
		CSRFToken: data.CSRFToken,
	}
	return mastersEngine.Render(context.Background(), resolveSchema(context.Background(), "masters", "marca", masterschemas.Schema("marca")), values, data.Errors, rc)
}

// mastersFormFields renders one of the masters forms' fields with the form
// engine, for the request: the tenant's override of the definition if it has
// one, and the labels of masters.forms.<name>.field.* in the request's locale.
// The CSRF field is left to the page function, which already writes it.
func (h *UIHandler) mastersFormFields(r *http.Request, name string, values map[string]interface{}, errs map[string]string, isEdit bool) mi.H {
	ctx := r.Context()
	engine := &formengine.FormEngine{Translator: h.formEngineTranslator()}
	return engine.Render(ctx, resolveSchema(ctx, "masters", name, masterschemas.Schema(name)), values, errs,
		formengine.RenderContext{Locale: h.currentLocale(), Module: "masters", Form: name, IsEdit: isEdit})
}

func (h *UIHandler) regionFormPage(r *http.Request, data RegionFormData, t func(key string, args ...any) string) mi.H {
	data.Fields = h.mastersFormFields(r, "region", map[string]interface{}{"description": data.Description}, data.Errors, data.IsEdit)
	return RegionFormPage(data, t)
}

func (h *UIHandler) monedaFormPage(r *http.Request, data MonedaFormData, t func(key string, args ...any) string) mi.H {
	data.Fields = h.mastersFormFields(r, "moneda", map[string]interface{}{"iso_code": data.IsoCode, "symbol": data.Symbol}, data.Errors, data.IsEdit)
	return MonedaFormPage(data, t)
}

func (h *UIHandler) marcaFormPage(r *http.Request, data MarcaFormData, t func(key string, args ...any) string) mi.H {
	data.Fields = h.mastersFormFields(r, "marca", map[string]interface{}{"code": data.Code, "name": data.Name}, data.Errors, data.IsEdit)
	return MarcaFormPage(data, t)
}
