package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	mi "github.com/ha1tch/minty"
	"github.com/ha1tch/seam-ui/internal/auth"
	fe "github.com/ha1tch/seam-ui/internal/formengine"
	actionschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/wo_action"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

type WOAction struct {
	ID          int64
	Name        string
	Code        string
	Description string
	Enabled     bool
	Parameters  []WOActionParameter
}

type WOActionParameter struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Entity   string `json:"entity"`
	Multiple bool   `json:"multiple"`
	Required bool   `json:"required"`
}

func (h *UIHandler) WOActionList(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()

	result, err := h.xoluClient.OQL(
		ctx,
		"SELECT TOP 100 * FROM wo_actions ORDER BY id DESC",
	)
	if err != nil {
		h.logger.Error("failed to list WOActions", "error", err)
		result = nil
	}

	var actions []WOActionListItem

	if result != nil {
		for _, row := range result.Data {

			actions = append(actions, WOActionListItem{
				ID:      toInt64(row["id"]),
				Name:    toString(row["name"]),
				Code:    toString(row["code"]),
				Enabled: toBool(row["enabled"]),
			})
		}
	}

	data := WOActionsListData{WOActions: actions}
	h.page.Render(w, r, t("wo_actions.list_title"), "/wo-actions", WOActionsListPage(data, t))

}

func (h *UIHandler) WOActionNew(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	csrfToken := h.GetCSRFToken(w, r)

	formEng := &fe.FormEngine{
		Translator: h.formEngineTranslator(),
	}
	rc := fe.RenderContext{Locale: h.currentLocale(), CSRFToken: csrfToken, Module: "wo-actions", Form: "wo-action"}
	formContent := formEng.Render(ctx, resolveSchema(ctx, "wo_action", "create", actionschemas.Schema("create")), nil, nil, rc)

	data := WOActionFormData{CSRFToken: csrfToken, FormContent: formContent}
	h.page.Render(w, r, t("wo_actions.new"), "/wo-actions/new", WOActionFormPage(data, t))
}

func parseWOActionParameters(r *http.Request, fields []fe.Field) ([]interface{}, error) {
	var parameterField *fe.Field

	for i := range fields {
		if fields[i].Name == "parameters" {
			parameterField = &fields[i]
			break
		}
	}

	if parameterField == nil {
		return nil, fmt.Errorf("parameters field not found in WOAction schema")
	}

	if parameterField.XSeamNaming != "indexed" {
		return nil, fmt.Errorf(
			"WOAction parameters must declare x-seam-naming: indexed",
		)
	}

	values := r.PostForm
	if len(values) == 0 {
		values = r.Form
	}

	normalized := make(url.Values)

	optionsPattern := regexp.MustCompile(
		`^parameters\[([0-9]+)\]\[options\]\[\]$`,
	)

	for key, entries := range values {
		// Evitar guardar una lista vacía desde un formulario antiguo.
		if strings.HasPrefix(key, "parameters_") {
			return nil, fmt.Errorf(
				"received flat parameter names; reload the indexed form",
			)
		}

		if !strings.HasPrefix(key, "parameters[") {
			continue
		}

		match := optionsPattern.FindStringSubmatch(key)

		if match == nil {
			normalized[key] = append([]string(nil), entries...)
			continue
		}

		// Convertir la lista de opciones a índices explícitos.
		// Así el parser compartido no pierde valores al leer vals[0].
		optionIndex := 0

		for _, entry := range entries {
			option := strings.TrimSpace(entry)
			if option == "" {
				continue
			}

			indexedKey := fmt.Sprintf(
				"parameters[%s][options][%d]",
				match[1],
				optionIndex,
			)

			normalized.Set(indexedKey, option)
			optionIndex++
		}
	}

	rows := fe.ParseIndexedRepeatable(normalized, *parameterField)

	for _, item := range rows {
		row, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid parsed WOAction parameter")
		}

		if row["options"] == nil {
			row["options"] = []interface{}{}
		}
	}

	return rows, nil
}

func (h *UIHandler) WOActionCreate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, h.errText(r, "error.invalid_form_data", "Invalid form data"), http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	code := strings.TrimSpace(r.FormValue("code"))
	enabled := r.FormValue("enabled") == "on"

	csrfToken := h.GetCSRFToken(w, r)
	errors := map[string]string{}

	if name == "" {
		errors["name"] = t("wo_actions.error.name_required")
	} else if utf8.RuneCountInString(name) > 200 {
		errors["name"] = t("wo_actions.error.name_too_long")
	}

	if code == "" {
		errors["code"] = t("wo_actions.error.code_required")
	} else if utf8.RuneCountInString(code) > 200 {
		errors["code"] = t("wo_actions.error.code_too_long")
	}

	form, err := fe.ParseSchema(resolveSchema(ctx, "wo_action", "create", actionschemas.Schema("create")))
	if err != nil {
		h.logger.Error("failed to parse WOAction schema", "error", err)
		http.Error(w, "Invalid schema", http.StatusInternalServerError)
		return
	}

	rawParameters, err := parseWOActionParameters(r, form.Fields)
	if err != nil {
		h.logger.Error("failed to parse WOAction parameters", "error", err)
		errors["parameters"] = t("wo_actions.error.invalid_parameter")
	}

	parametersItems, parametersData, parameterErrors := normalizeWOActionParameters(rawParameters, t)

	for field, message := range parameterErrors {
		errors[field] = message
	}

	if len(errors) > 0 {
		formEng := &fe.FormEngine{
			Translator: h.formEngineTranslator(),
		}
		rc := fe.RenderContext{Locale: h.currentLocale(), CSRFToken: csrfToken, Module: "wo-actions", Form: "wo-action"}

		formContent := formEng.Render(ctx, resolveSchema(ctx, "wo_action", "create", actionschemas.Schema("create")),
			map[string]interface{}{
				"name":        name,
				"description": description,
				"code":        code,
				"enabled":     enabled,
				"parameters":  parametersItems,
			}, errors, rc)

		data := WOActionFormData{CSRFToken: csrfToken, FormContent: formContent, Errors: errors}
		h.page.Render(w, r, t("wo_actions.new"), "/wo-actions", WOActionFormPage(data, t))
		return
	}

	actionData := map[string]any{
		"name":        name,
		"description": description,
		"code":        code,
		"enabled":     enabled,
		"parameters":  parametersData,
	}
	fmt.Printf("WOAction payload: %#v\n", actionData)
	entity, err := h.xoluClient.Create(ctx, "wo_actions", actionData)
	if err != nil {
		h.logger.Error("failed to create WOAction", "error", err)
		http.Error(w, h.errText(r, "error.save_failed", "Failed to save WOAction"), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/wo-actions/"+strconv.FormatInt(entity.ID, 10), http.StatusSeeOther)
}

func decodeWOActionParameters(value any) ([]WOActionParameter, error) {
	if value == nil {
		return nil, nil
	}

	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	var rows []struct {
		Key      string `json:"key"`
		Name     string `json:"name"`
		Type     string `json:"type"`
		Entity   string `json:"entity"`
		Multiple any    `json:"multiple"`
		Required any    `json:"required"`
	}

	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}

	boolValue := func(value any) bool {
		switch v := value.(type) {
		case bool:
			return v
		case string:
			switch strings.ToLower(strings.TrimSpace(v)) {
			case "on", "true", "1":
				return true
			}
		}
		return false
	}

	parameters := make([]WOActionParameter, 0, len(rows))
	for _, row := range rows {
		parameters = append(parameters, WOActionParameter{
			Key:      row.Key,
			Name:     row.Name,
			Type:     row.Type,
			Entity:   row.Entity,
			Multiple: boolValue(row.Multiple),
			Required: boolValue(row.Required),
		})
	}

	return parameters, nil
}

func (h *UIHandler) WOActionDetail(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("error.not_found"))
		return
	}

	entity, err := h.xoluClient.Get(ctx, "wo_actions", id)
	if err != nil {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("error.not_found"))
		return
	}

	parameters, err := decodeWOActionParameters(entity.Data["parameters"])
	if err != nil {
		h.logger.Error(
			"failed to decode WOAction parameters",
			"id", id,
			"error", err,
		)
		http.Error(w, h.errText(r, "error.load_failed", "Failed to load WOAction"), http.StatusInternalServerError)
		return
	}
	action := WOAction{
		ID:          entity.ID,
		Name:        toString(entity.Data["name"]),
		Description: toString(entity.Data["description"]),
		Code:        toString(entity.Data["code"]),
		Enabled:     toBool(entity.Data["enabled"]),
		Parameters:  parameters,
	}
	data := WOActionDetailData{WOAction: action, CSRFToken: h.GetCSRFToken(w, r)}
	h.page.Render(w, r, t("wo_action.detail_title"), "/wo-actions", WOActionDetailPage(data, t))
}

type WOActionListItem struct {
	ID      int64
	Name    string
	Code    string
	Enabled bool
}

type WOActionsListData struct {
	WOActions []WOActionListItem
}

func WOActionsListPage(data WOActionsListData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}

	rows := make([]mi.H, len(data.WOActions))

	for i, wh := range data.WOActions {
		enabledLabel := t("wo_action.inactive")
		var variant = "default"

		if wh.Enabled {
			enabledLabel = t("wo_action.enabled")
			variant = "success"
		}

		rows[i] = ListRow(i,
			// Nombre del almacén
			ListCellLink(
				wh.Name,
				fmt.Sprintf("/wo-actions/%d", wh.ID),
			),

			// Habilitado
			ListCellRich(func(b *mi.Builder) mi.Node {
				return Badge(enabledLabel, variant)(b)
			}),

			// Acciones
			ListActions(ListActionsConfig{
				T: t,
			}),
		)
	}

	return ListPage(
		ListPageConfig{
			Title:       t("wo_actions.list_title"),
			Subtitle:    t("wo_actions.subtitle"),
			CreateURL:   "/wo-actions/new",
			CreateLabel: t("wo_actions.new"),
		},
		ListTable(ListTableConfig{
			Columns: []ListColumn{
				{Label: t("field.name")},
				{Label: t("wo_action.field.status")},
				{Label: t("asset.list.actions"), Right: true},
			},
			EmptyIcon:    "pending_actions",
			EmptyTitle:   t("wo_actions.list_title"),
			EmptyMessage: t("wo_action.empty"),
		}, rows),
		nil,
		nil)
}

type WOActionFormData struct {
	ID           int64
	IsPartial    bool
	CSRFToken    string
	FormContent  mi.H
	Errors       map[string]string
	LockedAction *WOAction
}

func WOActionFormPage(data WOActionFormData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}

	title := t("wo_actions.new")
	submitLabel := t("wo_action.action.create")
	actionURL := "/wo-actions"
	cancelURL := "/wo-actions"

	if data.ID > 0 {
		title = t("wo_action.edit")
		submitLabel = t("action.save")
		actionURL = fmt.Sprintf("/wo-actions/%d", data.ID)
		cancelURL = actionURL
	}

	formID := "wo-action-form"

	return func(b *mi.Builder) mi.Node {
		children := []interface{}{
			mi.ID(formID),
			mi.Class("space-y-4"),
			mi.Method("post"),
			mi.Action(actionURL),
		}

		if data.IsPartial {
			children = append(children,
				mi.Attr("hx-post", actionURL),
				mi.Attr("hx-target", "#seam-modal-body"),
				mi.Attr("hx-swap", "innerHTML settle:0ms"),
			)
		}

		if message := data.Errors["_"]; message != "" {
			children = append(children,
				b.P(mi.Attr("role", "alert"), mi.Class("text-sm text-red-600 dark:text-red-400"), message),
			)
		}

		if data.LockedAction != nil {
			children = append(children,
				b.P(mi.Class("text-sm text-gray-700 dark:text-gray-200"), t("wo_action.edit.contract_locked")),
			)
		}

		children = append(children, data.FormContent(b))

		if locked := data.LockedAction; locked != nil {
			children = append(children,
				b.Div(mi.Class("space-y-3"),
					b.P(mi.Class("text-sm font-medium text-gray-700 dark:text-gray-300"), t("wo_action.forms.wo_action.field.code")),
					b.P(mi.Class("text-gray-900 dark:text-white break-all"), locked.Code),
					b.H2(mi.Class("font-medium text-gray-900 dark:text-white"), t("wo_action.parameters.title")),
					woActionParametersContent(locked.Parameters, t)(b),
				),
			)
		}

		form := b.Form(children...)

		actions := func(b *mi.Builder) mi.Node {
			cancelClass := "px-4 py-2 border border-gray-300 " +
				"dark:border-gray-600 text-gray-700 dark:text-gray-300 " +
				"rounded-lg font-medium"

			cancel := b.A(mi.Href(cancelURL), mi.Class(cancelClass), t("action.cancel"))

			if data.IsPartial {
				cancel = b.Button(mi.Type("button"), mi.Class(cancelClass), mi.Attr("onclick", "SeamModal.close()"), t("action.cancel"))
			}

			return b.Div(
				mi.Class("flex justify-end gap-3 pt-4 w-full"),
				cancel,
				b.Button(mi.Type("submit"), mi.Attr("form", formID),
					mi.Class(
						"px-4 py-2 bg-indigo-600 text-white rounded-lg "+
							"hover:bg-indigo-700 font-medium",
					),
					submitLabel,
				),
			)
		}

		if data.IsPartial {
			return b.Div(form, b.Div(mi.Attr("data-modal-footer", ""), actions(b)))
		}

		return b.Div(
			mi.Class("space-y-6 max-w-2xl"),
			b.Div(
				mi.Class("flex items-center gap-4"),
				b.A(
					mi.Href(cancelURL),
					mi.Class("text-gray-500 dark:text-gray-400"),
					b.Span(mi.Class("material-icons"), "arrow_back"),
				),
				b.H1(
					mi.Class("text-2xl font-bold text-gray-900 dark:text-white"),
					title,
				),
			),
			Card("", func(b *mi.Builder) mi.Node {
				return b.Div(form, actions(b))
			})(b),
		)
	}
}

type WOActionDetailData struct {
	WOAction  WOAction
	CSRFToken string
}

func WOActionDetailPage(data WOActionDetailData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	wh := data.WOAction
	enabledLabel := t("wo_action.inactive")
	var variant = "default"

	statusLabel := t("wo_action.enable")
	statusMessage := t("wo_action.confirm_enable")
	statusVariant := "success"

	if wh.Enabled {
		enabledLabel = t("wo_action.enabled")
		variant = "success"
		statusLabel = t("wo_action.disable")
		statusMessage = t("wo_action.confirm_disable")
		statusVariant = "danger"
	}

	return func(b *mi.Builder) mi.Node {

		return b.Div(mi.Class("space-y-6"),
			b.Div(mi.Class("flex items-center justify-between"),
				b.Div(
					// Breadcrumb
					b.Nav(mi.Class("text-sm text-gray-500 dark:text-gray-400 mb-2"),
						b.A(mi.Href("/wo-actions"), mi.Class("hover:text-gray-700 dark:hover:text-gray-300"), t("wo_actions.list_title")),
						b.Span(mi.Class("mx-2"), "/"),
						b.Span(mi.Class("text-gray-900 dark:text-white"), wh.Name),
					),
					// Title with status
					b.Div(mi.Class("flex items-center gap-3"),
						b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"),
							wh.Name,
						),
						Badge(enabledLabel, variant)(b),
					),
				),
				// Actions
				b.Div(mi.Class("flex items-center gap-2"),
					b.Button(
						mi.Class("inline-flex items-center gap-1 px-3 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700"),
						mi.Attr("hx-get", fmt.Sprintf("/wo-actions/%d/edit", wh.ID)),
						mi.Attr("hx-target", "#seam-modal-body"),
						mi.Attr("hx-swap", "innerHTML"),
						mi.Attr("onclick", fmt.Sprintf("SeamModal.open(%q)", t("wo_action.edit"))),
						b.Span(mi.Class("material-icons text-base leading-none"), "edit"),
						t("action.edit"),
					),
					Button(
						statusLabel,
						statusVariant,
						mi.Type("button"),
						mi.Attr("onclick", fmt.Sprintf(`
							SeamConfirm.show(%q, function () {
								htmx.ajax('POST', '/wo-actions/%d/enabled', {
									target: document.body,
									swap: 'none',
									values: {
										enabled: %q,
										_csrf: %q
									}
								});
							}, {
								title: %q,
								okLabel: %q,
								danger: %t
							});
						`,
							statusMessage,
							wh.ID,
							strconv.FormatBool(!wh.Enabled),
							data.CSRFToken,
							statusLabel,
							statusLabel,
							wh.Enabled,
						)),
					)(b),
					Button(t("action.delete"), "danger", mi.Type("button"), mi.Attr("onclick", fmt.Sprintf(`
							SeamConfirm.showDanger(%q, function () {
								htmx.ajax('POST', '/wo-actions/%d/delete', {
									target: '#wo-action-delete-feedback',
									swap: 'innerHTML',
									values: {
										_csrf: %q
									}
								});
							}, {
								title: %q,
								okLabel: %q
							});
						`,
						t("wo_actions.confirm_delete"),
						wh.ID,
						data.CSRFToken,
						t("wo_action.delete_title"),
						t("action.delete"),
					)),
					)(b),
				),
			),

			b.Div(mi.ID("wo-action-delete-feedback")),
			b.Div(mi.Class("space-y-6"),

				// Información: los 2/3 de ancho.
				b.Div(mi.Class("grid grid-cols-1 lg:grid-cols-3 gap-6"),
					b.Div(mi.Class("lg:col-span-2"),
						Card(
							t("wo_actions.info_title"),
							woActionInfoContent(wh, t),
						)(b),
					),
				),

				// Parámetros: debajo de la información y a todo el ancho.
				Card(t("wo_action.parameters.title"), woActionParametersContent(wh.Parameters, t))(b),
			),
		)
	}
}

func woActionInfoContent(wh WOAction, t func(key string, args ...any) string) mi.H {
	return func(b *mi.Builder) mi.Node {
		description := wh.Description
		if description == "" {
			description = t("wo_action.no_description")
		}

		return b.Dl(mi.Class("grid grid-cols-1 sm:grid-cols-2 gap-4"),
			infoRow(b, t("field.name"), wh.Name),
			infoRow(b, t("wo_action.enabled"), woActionEnabledLabel(wh.Enabled, t)),

			b.Div(mi.Class("sm:col-span-2"),
				infoRow(b, t("field.description"), description),
			),
			infoRow(b, t("wo_action.forms.wo_action.field.code"), wh.Code),
		)
	}
}

func woActionEnabledLabel(enabled bool, t func(key string, args ...any) string) string {
	if enabled {
		return t("wo_action.enabled")
	}
	return t("wo_action.inactive")
}

func woActionParametersContent(parameters []WOActionParameter, t func(key string, args ...any) string) mi.H {
	if len(parameters) == 0 {
		return func(b *mi.Builder) mi.Node {
			return b.P(
				mi.Class("text-sm text-gray-500 dark:text-gray-400"),
				t("wo_action.parameters.empty"),
			)
		}
	}

	textCell := func(value string) mi.H {
		return ListCellRich(func(b *mi.Builder) mi.Node {
			return b.Span(
				mi.Class("text-gray-900 dark:text-gray-100"),
				value,
			)
		})
	}

	boolLabel := func(value bool) string {
		if value {
			return t("wo_action.parameters.yes")
		}
		return t("wo_action.parameters.no")
	}

	rows := make([]mi.H, len(parameters))

	for i, parameter := range parameters {
		entityLabel := "—"
		if parameter.Type == "asset" && parameter.Entity != "" {
			entityLabel = parameter.Entity
		}

		rows[i] = ListRow(
			i,
			textCell(parameter.Name),
			textCell(parameter.Key),
			textCell(parameter.Type),
			textCell(entityLabel),
			textCell(boolLabel(parameter.Multiple)),
			textCell(boolLabel(parameter.Required)),
		)
	}

	table := ListTable(ListTableConfig{
		Columns: []ListColumn{
			{Label: t("field.name")},
			{Label: t("wo_action.parameters.key")},
			{Label: t("wo_action.parameters.type")},
			{Label: t("wo_action.parameters.entity")},
			{Label: t("wo_action.parameters.multiple")},
			{Label: t("wo_action.parameters.required")},
		},
	}, rows)

	return func(b *mi.Builder) mi.Node {
		return b.Div(
			mi.Class("w-full overflow-x-auto"),
			table(b),
		)
	}
}

// WOActionEditEntity checks administrative access and loads the latest
// WO Action data through OQL. Edit, update, and enabled-state handlers share
// this lookup to distinguish missing records from backend failures.
func (h *UIHandler) woActionEditEntity(w http.ResponseWriter, r *http.Request) (*xolu.Entity, bool) {
	t := h.translateFunc(r)

	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims.UserID <= 0 {
		http.Error(w, t("error.unauthorized"), http.StatusUnauthorized)
		return nil, false
	}

	if !claims.HasRole(auth.RoleAdmin) {
		http.Error(w, t("error.forbidden"), http.StatusForbidden)
		return nil, false
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, t("wo_action.not_found"), http.StatusNotFound)
		return nil, false
	}

	result, err := h.xoluClient.OQL(r.Context(), fmt.Sprintf("SELECT TOP 1 * FROM wo_actions WHERE id = %d", id))

	if err != nil {
		if entityNotYetCreated(err) {
			http.Error(w, t("wo_action.not_found"), http.StatusNotFound)
			return nil, false
		}

		h.logger.Error("failed to load work order action", "wo_action_id", id, "error", err)

		http.Error(w, t("wo_action.error.load_failed"), http.StatusInternalServerError)
		return nil, false
	}

	if result == nil || len(result.Data) == 0 {
		http.Error(w, t("wo_action.not_found"), http.StatusNotFound)
		return nil, false
	}

	return &xolu.Entity{
		ID:   id,
		Data: result.Data[0],
	}, true
}

// WOActionSetEnabled handles POST /wo-actions/{id}/enabled.
//
// The request specifies the desired state instead of toggling the stored
// value, so repeating the same request does not reverse the previous change.
func (h *UIHandler) WOActionSetEnabled(w http.ResponseWriter, r *http.Request) {
	entity, ok := h.woActionEditEntity(w, r)
	if !ok {
		return
	}

	t := h.translateFunc(r)

	if err := r.ParseForm(); err != nil {
		http.Error(w, t("error.invalid_form_data"), http.StatusBadRequest)
		return
	}

	var enabled bool

	switch r.PostForm.Get("enabled") {
	case "true":
		enabled = true
	case "false":
		enabled = false
	default:
		http.Error(w, t("error.invalid_form_data"), http.StatusBadRequest)
		return
	}

	if _, err := h.xoluClient.Patch(r.Context(), "wo_actions", entity.ID, map[string]any{"enabled": enabled}); err != nil {
		h.logger.Error("failed to change wo_action enabled state", "wo_action_id", entity.ID, "enabled", enabled, "error", err)

		http.Error(w, t("wo_action.error.update_failed"), http.StatusInternalServerError)
		return
	}

	detailURL := fmt.Sprintf("/wo-actions/%d", entity.ID)

	if IsHTMXRequest(r) {
		HTMXRedirect(w, detailURL)
		return
	}

	http.Redirect(w, r, detailURL, http.StatusSeeOther)
}

// woActionInUse checks every stored template version for references to an
// action. Disabled or outdated templates still prevent deletion.
//
// Query or decoding failures must not be interpreted as "unused".
func (h *UIHandler) woActionInUse(ctx context.Context, actionID int64) (bool, error) {
	result, err := h.xoluClient.OQL(ctx, "SELECT id, actions FROM work_order_templates")
	if err != nil {
		if entityNotYetCreated(err) {
			return false, nil
		}
		return false, fmt.Errorf("query action references: %w", err)
	}

	if result == nil {
		return false, fmt.Errorf("missing action reference query result")
	}

	for _, row := range result.Data {
		actions, err := decodeWorkOrderTemplateActions(row["actions"])
		if err != nil {
			return false, fmt.Errorf("decode actions in template %d: %w", toInt64(row["id"]), err)
		}

		for _, action := range actions {
			if action.ActionID == actionID {
				return true, nil
			}
		}
	}

	return false, nil
}

// WOActionDelete handles POST /wo-actions/{id}/delete.
// Only actions without references from stored templates may be deleted.
func (h *UIHandler) WOActionDelete(w http.ResponseWriter, r *http.Request) {
	entity, ok := h.woActionEditEntity(w, r)
	if !ok {
		return
	}

	t := h.translateFunc(r)
	ctx := r.Context()

	inUse, err := h.woActionInUse(ctx, entity.ID)
	if err != nil {
		h.logger.Error("failed to check WOAction references before deletion", "wo_action_id", entity.ID, "error", err)

		h.renderWOActionDeleteError(w, r, t("wo_action.error.references_load_failed"), http.StatusInternalServerError)
		return
	}

	if inUse {
		h.renderWOActionDeleteError(w, r, t("wo_action.error.in_use"), http.StatusConflict)
		return
	}

	if err := h.xoluClient.Delete(ctx, "wo_actions", entity.ID); err != nil {
		h.logger.Error("failed to delete WOAction", "wo_action_id", entity.ID, "error", err)

		h.renderWOActionDeleteError(w, r, t("wo_action.error.delete_failed"), http.StatusInternalServerError)
		return
	}

	if IsHTMXRequest(r) {
		HTMXRedirect(w, "/wo-actions")
		return
	}

	http.Redirect(w, r, "/wo-actions", http.StatusSeeOther)
}

// renderWOActionDeleteError preserves the detail page for HTMX requests,
// displaying the rejection in its feedback container.
func (h *UIHandler) renderWOActionDeleteError(w http.ResponseWriter, r *http.Request, message string, status int) {
	if IsHTMXRequest(r) {
		h.page.RenderPartial(w, func(b *mi.Builder) mi.Node {
			return b.Div(
				mi.Attr("role", "alert"),
				mi.Class(
					"rounded-lg border border-red-300 dark:border-red-700 "+
						"bg-red-50 dark:bg-red-900/20 p-4",
				),
				b.P(mi.Class("text-sm text-red-800 dark:text-white"), message),
			)
		})
		return
	}

	t := h.translateFunc(r)
	h.page.RenderError(w, status, t("error.generic"), message)
}

// normalizeWOActionParameters validates submitted parameter rows and builds
// separate representations for form rendering and persistence.
func normalizeWOActionParameters(rawParameters []interface{}, t func(key string, args ...any) string) ([]interface{}, []interface{}, map[string]string) {
	formRows := make([]interface{}, 0, len(rawParameters))
	storedRows := make([]interface{}, 0, len(rawParameters))
	errs := make(map[string]string)
	seenKeys := make(map[string]bool)

	keyPattern := regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

	textValue := func(value interface{}) string {
		text, _ := value.(string)
		return strings.TrimSpace(text)
	}

	setError := func(key string) {
		if _, exists := errs["parameters"]; !exists {
			errs["parameters"] = t(key)
		}
	}

	for _, item := range rawParameters {
		row, ok := item.(map[string]interface{})
		if !ok {
			setError("wo_actions.error.invalid_parameter")
			continue
		}

		key := textValue(row["key"])
		name := textValue(row["name"])
		fieldType := textValue(row["type"])

		entityName := ""
		if fieldType == "asset" {
			entityName = "assets"
		}

		var rawOptions []interface{}

		switch options := row["options"].(type) {
		case nil:
		case []string:
			for _, option := range options {
				rawOptions = append(rawOptions, option)
			}
		case []interface{}:
			rawOptions = options
		default:
			setError("wo_actions.error.invalid_parameter")
		}

		// The multi-select renderer expects a JSON-style array.
		options := make([]interface{}, 0, 2)
		multiple := false
		required := false

		for _, value := range rawOptions {
			option, ok := value.(string)
			if !ok {
				setError("wo_actions.error.invalid_parameter")
				continue
			}

			switch option {
			case "multiple":
				multiple = true
				options = append(options, option)
			case "required":
				required = true
				options = append(options, option)
			default:
				setError("wo_actions.error.invalid_parameter")
			}
		}

		formRows = append(formRows, map[string]interface{}{
			"key":      key,
			"name":     name,
			"type":     fieldType,
			"entity":   entityName,
			"options":  options,
			"multiple": multiple,
			"required": required,
		})

		storedRows = append(storedRows, map[string]interface{}{
			"key":      key,
			"name":     name,
			"type":     fieldType,
			"entity":   entityName,
			"multiple": multiple,
			"required": required,
		})

		switch {
		case key == "":
			setError("wo_actions.error.parameter_key_required")
		case utf8.RuneCountInString(key) > 100:
			setError("wo_actions.error.parameter_key_too_long")
		case !keyPattern.MatchString(key):
			setError("wo_actions.error.parameter_key_invalid")
		case seenKeys[key]:
			setError("wo_actions.error.parameter_key_duplicate")
		}

		if key != "" {
			seenKeys[key] = true
		}

		if name == "" {
			setError("wo_actions.error.parameter_name_required")
		} else if utf8.RuneCountInString(name) > 200 {
			setError("wo_actions.error.parameter_name_too_long")
		}

		if !validWOFieldType(fieldType) {
			setError("wo_actions.error.parameter_type_invalid")
		}
	}

	return formRows, storedRows, errs
}

// woActionParameterFormRows restores the checkbox-group values expected by
// the parameter editor from the stored boolean flags.
func woActionParameterFormRows(parameters []WOActionParameter) []interface{} {
	rows := make([]interface{}, 0, len(parameters))

	for _, parameter := range parameters {
		options := make([]interface{}, 0, 2)

		if parameter.Multiple {
			options = append(options, "multiple")
		}
		if parameter.Required {
			options = append(options, "required")
		}

		rows = append(rows, map[string]interface{}{
			"key":     parameter.Key,
			"name":    parameter.Name,
			"type":    parameter.Type,
			"entity":  parameter.Entity,
			"options": options,
		})
	}

	return rows
}

// WOActionEdit handles GET /wo-actions/{id}/edit.
func (h *UIHandler) WOActionEdit(w http.ResponseWriter, r *http.Request) {
	entity, ok := h.woActionEditEntity(w, r)
	if !ok {
		return
	}

	t := h.translateFunc(r)

	inUse, err := h.woActionInUse(r.Context(), entity.ID)
	if err != nil {
		h.logger.Error("failed to check WOAction usage before editing", "wo_action_id", entity.ID, "error", err)
		http.Error(w, t("wo_action.error.usage_load_failed"), http.StatusInternalServerError)
		return
	}

	parameters, err := decodeWOActionParameters(entity.Data["parameters"])
	if err != nil {
		h.logger.Error("failed to decode WOAction parameters for editing", "wo_action_id", entity.ID, "error", err)
		http.Error(w, t("wo_action.error.load_failed"), http.StatusInternalServerError)
		return
	}

	values := map[string]interface{}{
		"name":        toString(entity.Data["name"]),
		"description": toString(entity.Data["description"]),
		"code":        toString(entity.Data["code"]),
		"parameters":  woActionParameterFormRows(parameters),
	}

	h.renderWOActionEditForm(w, r, entity, inUse, values, nil)
}

// WOActionUpdate handles POST /wo-actions/{id}.
// Referenced actions retain their stored code and parameter definitions.
func (h *UIHandler) WOActionUpdate(w http.ResponseWriter, r *http.Request) {
	entity, ok := h.woActionEditEntity(w, r)
	if !ok {
		return
	}

	t := h.translateFunc(r)
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(
			w,
			t("error.invalid_form_data"),
			http.StatusBadRequest,
		)
		return
	}

	// Recheck usage on submission; do not trust the state of the opened form.
	inUse, err := h.woActionInUse(ctx, entity.ID)
	if err != nil {
		h.logger.Error("failed to check WOAction usage before updating", "wo_action_id", entity.ID, "error", err)
		http.Error(w, t("wo_action.error.usage_load_failed"), http.StatusInternalServerError)
		return
	}

	name := strings.TrimSpace(r.PostForm.Get("name"))
	description := strings.TrimSpace(r.PostForm.Get("description"))

	values := map[string]interface{}{
		"name":        name,
		"description": description,
	}
	fieldErrors := make(map[string]string)

	if name == "" {
		fieldErrors["name"] = t("wo_actions.error.name_required")
	} else if utf8.RuneCountInString(name) > 200 {
		fieldErrors["name"] = t("wo_actions.error.name_too_long")
	}

	changes := map[string]any{
		"name":        name,
		"description": description,
	}

	if inUse {
		// A previously editable form may have become locked while open.
		for key := range r.PostForm {
			if key == "code" ||
				key == "parameters" ||
				strings.HasPrefix(key, "parameters[") ||
				strings.HasPrefix(key, "parameters_") {
				fieldErrors["_"] = t("wo_action.error.contract_locked")
				break
			}
		}
	} else {
		code := strings.TrimSpace(r.PostForm.Get("code"))
		values["code"] = code

		if code == "" {
			fieldErrors["code"] = t("wo_actions.error.code_required")
		} else if utf8.RuneCountInString(code) > 200 {
			fieldErrors["code"] = t("wo_actions.error.code_too_long")
		}

		form, err := fe.ParseSchema(resolveSchema(ctx, "wo_action", "edit", actionschemas.Schema("edit")))
		if err != nil {
			h.logger.Error("failed to parse WOAction edit schema", "error", err)
			http.Error(
				w,
				t("wo_action.error.load_failed"),
				http.StatusInternalServerError,
			)
			return
		}

		rawParameters, parseErr := parseWOActionParameters(r, form.Fields)
		formRows, storedRows, parameterErrors :=
			normalizeWOActionParameters(rawParameters, t)

		values["parameters"] = formRows

		for field, message := range parameterErrors {
			fieldErrors[field] = message
		}

		if parseErr != nil {
			h.logger.Warn("failed to parse edited WOAction parameters", "wo_action_id", entity.ID, "error", parseErr)
			fieldErrors["parameters"] = t("wo_actions.error.invalid_parameter")
		}

		changes["code"] = code
		changes["parameters"] = storedRows
	}

	if len(fieldErrors) > 0 {
		h.renderWOActionEditForm(
			w, r, entity, inUse, values, fieldErrors,
		)
		return
	}

	if _, err := h.xoluClient.Patch(
		ctx, "wo_actions", entity.ID, changes,
	); err != nil {
		h.logger.Error("failed to update WOAction", "wo_action_id", entity.ID, "error", err)

		fieldErrors["_"] = t("wo_action.error.update_failed")
		h.renderWOActionEditForm(w, r, entity, inUse, values, fieldErrors)
		return
	}

	detailURL := fmt.Sprintf("/wo-actions/%d", entity.ID)

	if IsHTMXRequest(r) {
		HTMXRedirect(w, detailURL)
		return
	}

	http.Redirect(w, r, detailURL, http.StatusSeeOther)
}

// renderWOActionEditForm renders the same edit form for initial loading and
// validation failures. HTMX requests receive modal content; direct navigation
// receives a complete page.
func (h *UIHandler) renderWOActionEditForm(w http.ResponseWriter, r *http.Request, entity *xolu.Entity, inUse bool, values map[string]interface{}, fieldErrors map[string]string) {
	t := h.translateFunc(r)
	csrfToken := h.GetCSRFToken(w, r)
	isPartial := IsHTMXRequest(r)

	formEng := &fe.FormEngine{Translator: h.formEngineTranslator()}

	rc := fe.RenderContext{
		Locale:     h.currentLocale(),
		CSRFToken:  csrfToken,
		Module:     "wo-actions",
		Form:       "wo-action",
		IsEdit:     true,
		EntityID:   entity.ID,
		EntityType: "wo_actions",
		IsPartial:  isPartial,
	}

	var lockedAction *WOAction

	if inUse {
		parameters, err := decodeWOActionParameters(entity.Data["parameters"])
		if err != nil {
			h.logger.Error("failed to decode locked WOAction parameters", "wo_action_id", entity.ID, "error", err)
			http.Error(w, t("wo_action.error.load_failed"), http.StatusInternalServerError)
			return
		}

		rc.HiddenFields = []string{"code", "parameters"}

		lockedAction = &WOAction{
			Code:       toString(entity.Data["code"]),
			Parameters: parameters,
		}
	}

	content := formEng.Render(r.Context(), resolveSchema(r.Context(), "wo_action", "edit", actionschemas.Schema("edit")), values, fieldErrors, rc)

	data := WOActionFormData{
		ID:           entity.ID,
		IsPartial:    isPartial,
		CSRFToken:    csrfToken,
		FormContent:  content,
		Errors:       fieldErrors,
		LockedAction: lockedAction,
	}

	page := WOActionFormPage(data, t)

	if isPartial {
		h.page.RenderPartial(w, page)
		return
	}

	h.page.Render(w, r, t("wo_action.edit"), "/wo-actions", page)
}
