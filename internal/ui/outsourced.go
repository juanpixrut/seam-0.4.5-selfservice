package ui

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	mi "github.com/ha1tch/minty"
	"github.com/ha1tch/seam-ui/internal/auth"
	fe "github.com/ha1tch/seam-ui/internal/formengine"
	outsourcedschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/outsourced"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

type Outsourced struct {
	ID                     int64
	Name                   string
	Description            string
	Signature              string
	SignatureClarification string
}

func (h *UIHandler) OutsourcedList(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()

	result, err := h.xoluClient.OQL(ctx, "SELECT TOP 100 * FROM outsourced ORDER BY id DESC")
	var outsourced []Outsourced
	if err == nil {
		for _, row := range result.Data {
			outsourced = append(outsourced, Outsourced{
				ID:                     toInt64(row["id"]),
				Name:                   toString(row["name"]),
				Description:            toString(row["description"]),
				Signature:              toString(row["signature"]),
				SignatureClarification: toString(row["signature_clarification"]),
			})
		}
	} else {
		h.logger.Error("failed to list Outsourced", "error", err)
	}

	for i := range outsourced {
		if outsourced[i].ID > 0 {
			if outsource, err := h.xoluClient.Get(ctx, "outsourced", outsourced[i].ID); err == nil {
				outsourced[i].Name = toString(outsource.Data["name"])
			}
		}
	}

	data := OutsourcedListData{
		Outsourceds: outsourced,
		CSRFToken:   h.GetCSRFToken(w, r),
	}
	h.page.Render(w, r, t("outsourced.list_title"), "/outsourced", OutsourcedListPage(data, t))
}

func (h *UIHandler) OutsourcedNew(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	csrfToken := h.GetCSRFToken(w, r)

	formEng := &fe.FormEngine{
		Translator: h.formEngineTranslator(),
	}
	rc := fe.RenderContext{Locale: h.currentLocale(), CSRFToken: csrfToken, Module: "outsourced", Form: "outsourced"}
	formContent := formEng.Render(ctx, resolveSchema(ctx, "outsourced", "create", outsourcedschemas.Schema("create")), nil, nil, rc)

	data := OutsourcedFormData{CSRFToken: csrfToken, FormContent: formContent}
	h.page.Render(w, r, t("outsourced.new"), "/outsourced", OutsourcedFormPage(data, t))
}

func (h *UIHandler) OutsourcedCreate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, h.errText(r, "error.invalid_form_data", "Invalid form data"), http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	description := r.FormValue("description")
	Signature := r.FormValue("signature")
	SignatureClarification := r.FormValue("signature_clarification")
	csrfToken := h.GetCSRFToken(w, r)

	errors := map[string]string{}
	if name == "" {
		errors["name"] = t("outsourced.error.name_required")
	}

	if len(errors) > 0 {
		formEng := &fe.FormEngine{
			Translator: h.formEngineTranslator(),
		}
		rc := fe.RenderContext{Locale: h.currentLocale(), CSRFToken: csrfToken, Module: "outsourced", Form: "outsourced"}

		formValues := map[string]interface{}{
			"name":                    name,
			"description":             description,
			"signature":               Signature,
			"signature_clarification": SignatureClarification,
		}

		formContent := formEng.Render(ctx, resolveSchema(ctx, "outsourced", "create", outsourcedschemas.Schema("create")), formValues, errors, rc)
		data := OutsourcedFormData{CSRFToken: csrfToken, FormContent: formContent, Errors: errors}
		h.page.Render(w, r, t("outsourced.new"), "/outsourced", OutsourcedFormPage(data, t))
		return
	}

	outsourcedData := map[string]any{
		"name":                    name,
		"description":             description,
		"signature":               Signature,
		"signature_clarification": SignatureClarification,
	}

	h.logger.Info("Form payload parsed", "form", r.Form)

	_, err := h.xoluClient.Create(ctx, "outsourced", outsourcedData)
	if err != nil {
		h.logger.Error("failed to create outsourced", "error", err)
		http.Error(w, h.errText(r, "error.save_failed", "Failed to save outsourced"), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/outsourced", http.StatusSeeOther)
}

func (h *UIHandler) OutsourcedDetail(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("error.not_found"))
		return
	}

	entity, err := h.xoluClient.Get(ctx, "outsourced", id)
	if err != nil {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("error.not_found"))
		return
	}

	outsourced := Outsourced{
		ID:                     entity.ID,
		Name:                   toString(entity.Data["name"]),
		Description:            toString(entity.Data["description"]),
		Signature:              toString(entity.Data["signature"]),
		SignatureClarification: toString(entity.Data["signature_clarification"]),
	}

	data := OutsourcedDetailData{
		Outsourced: outsourced,
		CSRFToken:  h.GetCSRFToken(w, r),
	}
	if claims, ok := auth.ClaimsFromContext(ctx); ok && auth.HasPermission(claims.Role, auth.PermAdministrationUserEdit) {
		engine := &fe.FormEngine{Translator: h.formEngineTranslator()}
		values := map[string]any{}
		if userID := toInt64(entity.Data["responsible_user_id"]); userID > 0 {
			values["responsible_user_id"] = userID
		}
		rc := fe.RenderContext{Locale: h.currentLocale(), CSRFToken: h.GetCSRFToken(w, r), Module: "outsourced", Form: "account"}
		data.AccountForm = engine.Render(ctx, resolveSchema(ctx, "outsourced", "account", outsourcedschemas.Schema("account")), values, nil, rc)
	}
	h.page.Render(w, r, t("outsourced.detail_title"), "/outsourced", OutsourcedDetailPage(data, t))
}

func (h *UIHandler) OutsourcedLinkAccount(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, t("error.not_found"), http.StatusNotFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, t("error.invalid_form_data"), http.StatusBadRequest)
		return
	}
	userID := int64(0)
	if raw := r.PostForm.Get("responsible_user_id"); raw != "" {
		userID, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || userID <= 0 {
			http.Error(w, t("error.invalid_form_data"), http.StatusBadRequest)
			return
		}
		if _, err := h.xoluClient.Get(ctx, "users", userID); err != nil {
			var xerr *xolu.Error
			if errors.As(err, &xerr) && xerr.HTTPStatus == http.StatusNotFound {
				http.Error(w, t("error.not_found"), http.StatusNotFound)
			} else {
				h.logger.Error("failed to load outsourced representative account", "user_id", userID, "error", err)
				http.Error(w, t("error.server_error"), http.StatusInternalServerError)
			}
			return
		}
	}
	if _, err := h.xoluClient.Patch(ctx, "outsourced", id, map[string]any{"responsible_user_id": userID}); err != nil {
		var xerr *xolu.Error
		if errors.As(err, &xerr) && xerr.HTTPStatus == http.StatusNotFound {
			http.Error(w, t("error.not_found"), http.StatusNotFound)
		} else {
			h.logger.Error("failed to link outsourced representative account", "id", id, "error", err)
			http.Error(w, t("error.server_error"), http.StatusInternalServerError)
		}
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/outsourced/%d", id), http.StatusSeeOther)
}

type OutsourcedListData struct {
	Outsourceds []Outsourced
	CSRFToken   string
}

func OutsourcedListPage(data OutsourcedListData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}

	rows := make([]mi.H, len(data.Outsourceds))

	for i, out := range data.Outsourceds {

		rows[i] = ListRow(i,
			ListCellLink(
				out.Name,
				fmt.Sprintf("/outsourced/%d", out.ID),
			),
			ListCellMuted(out.Description),
			ListCellRight(func(b *mi.Builder) mi.Node {
				btnClass := "p-1 rounded text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700 "

				return b.Div(mi.Class("flex items-center justify-end gap-1"),
					b.Button(
						mi.Type("button"),
						mi.Class(btnClass+"hover:text-indigo-600 dark:hover:text-indigo-400"),
						b.Span(
							mi.Class("material-icons text-lg"),
							mi.Attr("title", t("action.edit")),
							"edit",
						),
						mi.Attr("hx-get", fmt.Sprintf("/outsourced/dynamic/%d/edit?return_to=/outsourced", out.ID)),
						mi.Attr("hx-target", "#seam-modal-body"),
						mi.Attr("hx-swap", "innerHTML"),
						mi.Attr("onclick", fmt.Sprintf("SeamModal.open(%q)", t("action.edit"))),
					),
					b.Form(
						mi.Method("post"),
						mi.Attr("action", fmt.Sprintf("/outsourced/%d/delete", out.ID)),
						mi.Class("inline"),
						b.Input(
							mi.Type("hidden"),
							mi.Name("_csrf"),
							mi.Value(data.CSRFToken),
						),
						b.Button(
							mi.Type("submit"),
							mi.Class(btnClass+"hover:text-red-600 dark:hover:text-red-400"),
							mi.Attr("onclick", fmt.Sprintf(
								"event.preventDefault();SeamConfirm.showForm(this.form,%q,{dangerName:%q})",
								t("confirm.delete"),
								out.Name,
							)),
							b.Span(
								mi.Class("material-icons text-lg"),
								mi.Attr("title", t("action.delete")),
								"delete",
							),
						),
					),
				)
			}),
		)
	}

	return ListPage(
		ListPageConfig{
			Title:       t("outsourced.list_title"),
			CreateURL:   "/outsourced/new",
			CreateLabel: t("outsourced.new"),
		},
		ListTable(ListTableConfig{
			Columns: []ListColumn{
				{Label: t("field.name")},
				{Label: t("outsourced.description")},
				{Label: t("asset.list.actions"), Right: true},
			},
			EmptyIcon:    "assignment_ind",
			EmptyTitle:   t("outsourced.list_title"),
			EmptyMessage: t("outsourced.empty"),
		}, rows),
		nil,
		nil)
}

type OutsourcedFormData struct {
	CSRFToken   string
	FormContent mi.H
	Errors      map[string]string
}

func OutsourcedFormPage(data OutsourcedFormData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	return func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("space-y-6 max-w-2xl"),
			b.Div(mi.Class("flex items-center gap-4"),
				b.A(mi.Href("/outsourced"), mi.Class("text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"),
					b.Span(mi.Class("material-icons"), "arrow_back"),
				),
				b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), t("outsourced.new")),
			),
			Card("", func(b *mi.Builder) mi.Node {
				return b.Form(mi.Class("space-y-4"), mi.Method("post"), mi.Action("/outsourced"),
					data.FormContent(b),
					b.Div(mi.Class("flex justify-end gap-3 pt-4"),
						b.A(mi.Href("/outsourced"), mi.Class("px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg font-medium hover:bg-gray-50 dark:hover:bg-gray-700"),
							t("action.cancel"),
						),
						b.Button(mi.Type("submit"), mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 font-medium"),
							t("outsourced.action.create"),
						),
					),
				)
			})(b),
		)
	}
}

type OutsourcedDetailData struct {
	Outsourced  Outsourced
	CSRFToken   string
	AccountForm mi.H
}

func OutsourcedDetailPage(data OutsourcedDetailData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	iss := data.Outsourced

	return func(b *mi.Builder) mi.Node {
		var account mi.Node
		if data.AccountForm != nil {
			account = b.Form(mi.Class("space-y-3"), mi.Method("post"), mi.Action(fmt.Sprintf("/outsourced/%d/account", iss.ID)),
				data.AccountForm(b),
				b.Button(mi.Type("submit"), mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg"), t("action.save")),
			)
		}
		return b.Div(mi.Class("space-y-6 max-w-2xl"),
			b.Div(mi.Class("flex items-center justify-between"),
				b.Div(mi.Class("flex items-center gap-4"),
					b.A(
						mi.Href("/outsourced"),
						mi.Class("text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"),
						b.Span(mi.Class("material-icons"), "arrow_back"),
					),
					b.H1(
						mi.Class("text-2xl font-bold text-gray-900 dark:text-white"),
						iss.Name,
					),
				),
				b.Div(mi.Class("flex items-center gap-2"),
					b.Button(
						mi.Type("button"),
						mi.Class("inline-flex items-center gap-1 px-3 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700"),
						b.Span(mi.Class("material-icons text-base leading-none"), "edit"),
						t("action.edit"),
						mi.Attr("hx-get", fmt.Sprintf("/outsourced/dynamic/%d/edit?return_to=/outsourced/%d", iss.ID, iss.ID)),
						mi.Attr("hx-target", "#seam-modal-body"),
						mi.Attr("hx-swap", "innerHTML"),
						mi.Attr("onclick", fmt.Sprintf("SeamModal.open(%q)", t("action.edit"))),
					),
					b.Form(
						mi.Method("post"),
						mi.Attr("action", fmt.Sprintf("/outsourced/%d/delete", iss.ID)),
						mi.Class("inline"),
						b.Input(
							mi.Type("hidden"),
							mi.Name("_csrf"),
							mi.Value(data.CSRFToken),
						),
						Button(
							t("action.delete"),
							"danger",
							mi.Type("submit"),
							mi.Attr("onclick", fmt.Sprintf(
								"event.preventDefault();SeamConfirm.showForm(this.form,%q,{dangerName:%q})",
								t("confirm.delete"),
								iss.Name,
							)),
						)(b),
					),
				),
			),
			Card("", func(b *mi.Builder) mi.Node {
				return b.Div(mi.Class("space-y-6"),
					b.Div(
						b.H3(mi.Class("text-xs font-semibold text-gray-500 uppercase tracking-wider mb-1 dark:text-gray-400"), t("outsourced.description")),
						b.Div(mi.Class("text-sm text-gray-700 dark:text-gray-300"), iss.Description),
					),

					b.Div(
						b.H3(
							mi.Class("text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2 dark:text-gray-400"),
							t("outsourced.signature"),
						),
						b.Div(
							mi.Class("inline-block max-w-full rounded-lg border border-gray-200 bg-white p-3"),
							b.Img(
								mi.Src(iss.Signature),
								mi.Alt(t("outsourced.responsible_signature")),
								mi.Class("block max-w-full h-auto"),
							),
						),
					),

					b.Div(
						b.H3(
							mi.Class("text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2 dark:text-gray-400"),
							t("outsourced.signature_clarification"),
						),
						b.Div(
							mi.Class("text-sm text-gray-700 dark:text-gray-300"),
							iss.SignatureClarification,
						),
					),
				)
			})(b),
			account,
		)
	}
}

func (h *UIHandler) OutsourcedDelete(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("error.not_found"))
		return
	}

	if err := h.xoluClient.Delete(ctx, "outsourced", id); err != nil {
		h.logger.Error("failed to delete outsourced", "outsourced_id", id, "error", err)
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.delete_failed"))
		return
	}

	http.Redirect(w, r, "/outsourced", http.StatusSeeOther)
}

func (h *UIHandler) OutsourcedEditDynamic(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("error.not_found"))
		return
	}

	entity, err := h.xoluClient.Get(ctx, "outsourced", id)
	if err != nil {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("error.not_found"))
		return
	}

	values := map[string]interface{}{
		"name":                    toString(entity.Data["name"]),
		"description":             toString(entity.Data["description"]),
		"signature":               toString(entity.Data["signature"]),
		"signature_clarification": toString(entity.Data["signature_clarification"]),
	}

	action := fmt.Sprintf("/outsourced/dynamic/%d/update", id)
	formEng := &fe.FormEngine{Translator: h.formEngineTranslator()}
	rc := fe.RenderContext{
		Locale:     h.currentLocale(),
		CSRFToken:  h.GetCSRFToken(w, r),
		Module:     "outsourced",
		Form:       "outsourced",
		Action:     action,
		Method:     "post",
		IsPartial:  true,
		IsEdit:     true,
		EntityID:   id,
		EntityType: "outsourced",
	}
	formContent := formEng.Render(ctx, resolveSchema(ctx, "outsourced", "create", outsourcedschemas.Schema("create")), values, nil, rc)

	returnTo := outsourcedEditReturnTo(id, r.URL.Query().Get("return_to"))
	h.page.RenderPartial(w, OutsourcedEditModalPage(id, returnTo, formContent, t))
}

func outsourcedEditReturnTo(id int64, requested string) string {
	if requested == "/outsourced" || requested == fmt.Sprintf("/outsourced/%d", id) {
		return requested
	}
	return fmt.Sprintf("/outsourced/%d", id)
}

func OutsourcedEditModalPage(id int64, returnTo string, formContent mi.H, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}

	action := fmt.Sprintf("/outsourced/dynamic/%d/update", id)

	return func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("w-full"),
			b.H2(mi.Class("text-lg font-semibold text-gray-900 dark:text-white mb-4"), t("action.edit")),
			b.Form(
				mi.ID("outsourced-edit-form"),
				mi.Method("post"),
				mi.Action(action),
				mi.Class("space-y-4"),
				mi.Attr("hx-post", action),
				mi.Attr("hx-target", "#seam-modal-body"),
				mi.Attr("hx-swap", "innerHTML"),
				formContent(b),
				b.Input(mi.Type("hidden"), mi.Name("return_to"), mi.Value(returnTo)),
				b.Div(mi.Class("flex justify-end gap-3 pt-4"),
					b.Button(
						mi.Type("button"),
						mi.Class("px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700"),
						mi.Attr("onclick", "SeamModal.close()"),
						t("action.cancel"),
					),
					b.Button(
						mi.Type("submit"),
						mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 font-medium"),
						t("action.save"),
					),
				),
			),
		)
	}
}

func (h *UIHandler) OutsourcedUpdateDynamic(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("error.not_found"))
		return
	}

	if err := r.ParseForm(); err != nil {
		h.page.RenderError(w, http.StatusBadRequest, t("error.generic"), t("error.invalid_form_data"))
		return
	}

	values := map[string]interface{}{
		"name":                    r.FormValue("name"),
		"description":             r.FormValue("description"),
		"signature":               r.FormValue("signature"),
		"signature_clarification": r.FormValue("signature_clarification"),
	}

	validationErrors := map[string]string{}
	if values["name"] == "" {
		validationErrors["name"] = t("outsourced.error.name_required")
	}

	returnTo := outsourcedEditReturnTo(id, r.FormValue("return_to"))
	action := fmt.Sprintf("/outsourced/dynamic/%d/update", id)

	if len(validationErrors) > 0 {
		formEng := &fe.FormEngine{Translator: h.formEngineTranslator()}
		rc := fe.RenderContext{
			Locale:     h.currentLocale(),
			CSRFToken:  h.GetCSRFToken(w, r),
			Module:     "outsourced",
			Form:       "outsourced",
			Action:     action,
			Method:     "post",
			IsPartial:  true,
			IsEdit:     true,
			EntityID:   id,
			EntityType: "outsourced",
		}
		formContent := formEng.Render(ctx, resolveSchema(ctx, "outsourced", "create", outsourcedschemas.Schema("create")), values, validationErrors, rc)
		h.page.RenderPartial(w, OutsourcedEditModalPage(id, returnTo, formContent, t))
		return
	}

	if _, err := h.xoluClient.Patch(ctx, "outsourced", id, values); err != nil {
		h.logger.Error("failed to update outsourced", "outsourced_id", id, "error", err)
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.update_failed"))
		return
	}

	if IsHTMXRequest(r) {
		w.Header().Set("HX-Redirect", returnTo)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	http.Redirect(w, r, returnTo, http.StatusSeeOther)
}
