package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	mi "github.com/ha1tch/minty"
	fe "github.com/ha1tch/seam-ui/internal/formengine"
	crewschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/crew"
)

type Crew struct {
	ID            int64
	Name          string
	Description   string
	ResponsibleId int64
	Members       []int64
}

func (h *UIHandler) CrewList(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()

	result, err := h.xoluClient.OQL(ctx, "SELECT TOP 100 * FROM crews ORDER BY id DESC")
	var crews []Crew
	if err == nil {
		for _, row := range result.Data {
			crews = append(crews, Crew{
				ID:            toInt64(row["id"]),
				Name:          toString(row["name"]),
				Description:   toString(row["description"]),
				ResponsibleId: toInt64(row["responsible_user_id"]),
				Members:       toInt64Slice(row["members"]),
			})
		}
	} else {
		h.logger.Error("failed to list crews", "error", err)
	}

	for i := range crews {
		if crews[i].ID > 0 {
			if crew, err := h.xoluClient.Get(ctx, "crews", crews[i].ID); err == nil {
				crews[i].Name = toString(crew.Data["name"])
			}
		}
	}

	data := CrewsListData{
		Crews:     crews,
		CSRFToken: h.GetCSRFToken(w, r),
	}
	h.page.Render(w, r, t("crews.list_title"), "/crews", CrewsListPage(data, t))
}

func (h *UIHandler) CrewNew(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	csrfToken := h.GetCSRFToken(w, r)

	formEng := &fe.FormEngine{
		Translator: h.formEngineTranslator(),
	}
	rc := fe.RenderContext{Locale: h.currentLocale(), CSRFToken: csrfToken, Module: "crew", Form: "crew"}
	formContent := formEng.Render(ctx, resolveSchema(ctx, "crew", "create", crewschemas.Schema("create")), nil, nil, rc)

	data := CrewFormData{CSRFToken: csrfToken, FormContent: formContent}
	h.page.Render(w, r, t("crews.new"), "/crews/new", CrewFormPage(data, t))
}

func (h *UIHandler) CrewCreate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, h.errText(r, "error.invalid_form_data", "Invalid form data"), http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	description := r.FormValue("description")
	responsible := r.FormValue("responsible_user_id")
	csrfToken := h.GetCSRFToken(w, r)

	var responsibleID *int64
	if responsible != "" {
		if id, err := strconv.ParseInt(responsible, 10, 64); err == nil && id > 0 {
			responsibleID = &id
		}
	}

	memberRows := fe.ParseIndexedRepeatable(r.Form, fe.Field{Name: "members"})

	var members []int64
	seenMembers := make(map[int64]bool)

	for _, item := range memberRows {
		if row, ok := item.(map[string]interface{}); ok {
			if val, ok := row["member"]; ok {
				var memberID int64
				switch v := val.(type) {
				case string:
					if id, err := strconv.ParseInt(v, 10, 64); err == nil {
						memberID = id
					}
				case int64:
					memberID = v
				case float64:
					memberID = int64(v)
				case int:
					memberID = int64(v)
				}

				if memberID > 0 && !seenMembers[memberID] {
					seenMembers[memberID] = true
					members = append(members, memberID)
				}
			}
		}
	}

	errors := map[string]string{}
	if name == "" {
		errors["name"] = t("crew.error.name_required")
	}

	if responsibleID == nil {
		errors["responsible_user_id"] = t("crew.error.responsible_required")
	}

	if len(members) < 2 {
		errors["members"] = t("crew.error.min_members")
	}

	if len(errors) > 0 {
		formEng := &fe.FormEngine{
			Translator: h.formEngineTranslator(),
		}
		rc := fe.RenderContext{Locale: h.currentLocale(), CSRFToken: csrfToken, Module: "crew", Form: "crew"}

		formContent := formEng.Render(ctx, resolveSchema(ctx, "crew", "create", crewschemas.Schema("create")),
			map[string]interface{}{
				"name":                name,
				"description":         description,
				"responsible_user_id": responsible,
				"members":             memberRows,
			}, errors, rc)

		data := CrewFormData{CSRFToken: csrfToken, FormContent: formContent, Errors: errors}
		h.page.Render(w, r, t("crews.new"), "/crews", CrewFormPage(data, t))
		return
	}

	if responsibleID != nil && !seenMembers[*responsibleID] {
		members = append(members, *responsibleID)
		seenMembers[*responsibleID] = true
	}

	crewData := map[string]any{
		"name":                name,
		"description":         description,
		"responsible_user_id": responsibleID,
		"members":             members,
	}

	entity, err := h.xoluClient.Create(ctx, "crews", crewData)
	if err != nil {
		h.logger.Error("failed to create crew", "error", err)
		http.Error(w, h.errText(r, "error.save_failed", "Failed to save crew"), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/crew/"+strconv.FormatInt(entity.ID, 10), http.StatusSeeOther)
}

func (h *UIHandler) CrewDetail(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("error.not_found"))
		return
	}

	entity, err := h.xoluClient.Get(ctx, "crews", id)
	if err != nil {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("error.not_found"))
		return
	}

	crew := Crew{
		ID:            entity.ID,
		Name:          toString(entity.Data["name"]),
		Description:   toString(entity.Data["description"]),
		ResponsibleId: toInt64(entity.Data["responsible_user_id"]),
		Members:       toInt64Slice(entity.Data["members"]),
	}

	userIDsMap := make(map[int64]bool)
	if crew.ResponsibleId > 0 {
		userIDsMap[crew.ResponsibleId] = true
	}
	for _, memberID := range crew.Members {
		if memberID > 0 {
			userIDsMap[memberID] = true
		}
	}

	membersMap := make(map[int64]string)
	if len(userIDsMap) > 0 {
		var ids []string
		for userID := range userIDsMap {
			ids = append(ids, fmt.Sprintf("%d", userID))
		}

		query := fmt.Sprintf("SELECT id, name FROM users WHERE id IN (%s)", strings.Join(ids, ","))
		if result, err := h.xoluClient.OQL(ctx, query); err == nil {
			for _, row := range result.Data {
				uID := toInt64(row["id"])
				uName := toString(row["name"])
				membersMap[uID] = uName
			}
		} else {
			h.logger.Error("failed to fetch user names for crew detail", "error", err)
		}
	}
	data := CrewDetailData{Crew: crew, CSRFToken: h.GetCSRFToken(w, r), ResponsibleName: membersMap[crew.ResponsibleId], MembersMap: membersMap}
	h.page.Render(w, r, t("crew.detail_title"), "/crews", CrewDetailPage(data, t))
}

type CrewsListData struct {
	Crews     []Crew
	CSRFToken string
}

func CrewsListPage(data CrewsListData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}

	rows := make([]mi.H, len(data.Crews))

	for i, crw := range data.Crews {

		rows[i] = ListRow(i,
			ListCellLink(
				crw.Name,
				fmt.Sprintf("/crew/%d", crw.ID),
			),
			ListCellMuted(crw.Description),
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
						mi.Attr("hx-get", fmt.Sprintf("/crews/dynamic/%d/edit?return_to=/crews", crw.ID)),
						mi.Attr("hx-target", "#seam-modal-body"),
						mi.Attr("hx-swap", "innerHTML"),
						mi.Attr("onclick", fmt.Sprintf("SeamModal.open(%q)", t("action.edit"))),
					),
					b.Form(
						mi.Method("post"),
						mi.Attr("action", fmt.Sprintf("/crew/%d/delete", crw.ID)),
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
								crw.Name,
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
			Title:       t("crews.list_title"),
			CreateURL:   "/crews/new",
			CreateLabel: t("crews.new"),
		},
		ListTable(ListTableConfig{
			Columns: []ListColumn{
				{Label: t("field.name")},
				{Label: t("crews.description")},
				{Label: t("asset.list.actions"), Right: true},
			},
			EmptyIcon:    "groups",
			EmptyTitle:   t("crews.list_title"),
			EmptyMessage: t("crews.empty"),
		}, rows),
		nil,
		nil)
}

type CrewFormData struct {
	CSRFToken   string
	FormContent mi.H
	Errors      map[string]string
}

func CrewFormPage(data CrewFormData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	return func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("space-y-6 max-w-2xl"),
			b.Div(mi.Class("flex items-center gap-4"),
				b.A(mi.Href("/crews"), mi.Class("text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"),
					b.Span(mi.Class("material-icons"), "arrow_back"),
				),
				b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), t("crews.new")),
			),
			Card("", func(b *mi.Builder) mi.Node {
				return b.Form(mi.Class("space-y-4"), mi.Method("post"), mi.Action("/crews"),
					data.FormContent(b),
					b.Div(mi.Class("flex justify-end gap-3 pt-4"),
						b.A(mi.Href("/crews"), mi.Class("px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg font-medium hover:bg-gray-50 dark:hover:bg-gray-700"),
							t("action.cancel"),
						),
						b.Button(mi.Type("submit"), mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 font-medium"),
							t("crews.action.create"),
						),
					),
				)
			})(b),
		)
	}
}

type CrewDetailData struct {
	Crew            Crew
	CSRFToken       string
	ResponsibleName string
	MembersMap      map[int64]string
}

func CrewDetailPage(data CrewDetailData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	iss := data.Crew

	return func(b *mi.Builder) mi.Node {
		var memberNodes []interface{}

		for _, memberID := range iss.Members {
			memberName := data.MembersMap[memberID]

			memberNodes = append(memberNodes,
				b.Span(
					mi.Class("inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-medium bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-200 border border-gray-200 dark:border-gray-700"),
					b.Span(mi.Class("material-icons text-sm text-gray-400"), "person"),
					memberName,
				),
			)
		}

		respName := data.ResponsibleName

		return b.Div(mi.Class("space-y-6 max-w-2xl"),
			b.Div(mi.Class("flex items-center justify-between"),
				b.Div(mi.Class("flex items-center gap-4"),
					b.A(
						mi.Href("/crews"),
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
						mi.Attr("hx-get", fmt.Sprintf("/crews/dynamic/%d/edit?return_to=/crew/%d", iss.ID, iss.ID)),
						mi.Attr("hx-target", "#seam-modal-body"),
						mi.Attr("hx-swap", "innerHTML"),
						mi.Attr("onclick", fmt.Sprintf("SeamModal.open(%q)", t("action.edit"))),
					),
					b.Form(
						mi.Method("post"),
						mi.Attr("action", fmt.Sprintf("/crew/%d/delete", iss.ID)),
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
						b.H3(mi.Class("text-xs font-semibold text-gray-500 uppercase tracking-wider mb-1 dark:text-gray-400"), t("crews.description")),
						b.Div(mi.Class("text-sm text-gray-700 dark:text-gray-300"), iss.Description),
					),

					b.Div(
						b.H3(mi.Class("text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2 dark:text-gray-400"), t("crews.responsible")),
						b.Div(mi.Class("flex items-center gap-2"),
							b.Span(
								mi.Class("inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-medium bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-200 border border-gray-200 dark:border-gray-700"),
								b.Span(mi.Class("material-icons text-sm text-gray-400"), "star"),
								respName,
							),
						),
					),

					b.Div(
						b.H3(mi.Class("text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2 dark:text-gray-400"), t("crews.members")),
						b.Div(memberNodes...),
					),
				)
			})(b),
		)
	}
}

func (h *UIHandler) CrewDelete(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("error.not_found"))
		return
	}

	if err := h.xoluClient.Delete(ctx, "crews", id); err != nil {
		h.logger.Error("failed to delete crew", "crew_id", id, "error", err)
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.delete_failed"))
		return
	}

	http.Redirect(w, r, "/crews", http.StatusSeeOther)
}

func (h *UIHandler) CrewEditDynamic(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("error.not_found"))
		return
	}

	entity, err := h.xoluClient.Get(ctx, "crews", id)
	if err != nil {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("error.not_found"))
		return
	}

	members := toInt64Slice(entity.Data["members"])
	memberRows := make([]interface{}, 0, len(members))
	for _, memberID := range members {
		memberRows = append(memberRows, map[string]interface{}{
			"member": strconv.FormatInt(memberID, 10),
		})
	}

	values := map[string]interface{}{
		"name":                toString(entity.Data["name"]),
		"description":         toString(entity.Data["description"]),
		"responsible_user_id": strconv.FormatInt(toInt64(entity.Data["responsible_user_id"]), 10),
		"members":             memberRows,
	}

	action := fmt.Sprintf("/crews/dynamic/%d", id)
	formEng := &fe.FormEngine{Translator: h.formEngineTranslator()}
	rc := fe.RenderContext{
		Locale:     h.currentLocale(),
		CSRFToken:  h.GetCSRFToken(w, r),
		Module:     "crew",
		Form:       "crew",
		Action:     action,
		Method:     "post",
		IsPartial:  true,
		IsEdit:     true,
		EntityID:   id,
		EntityType: "crew",
	}
	formContent := formEng.Render(ctx, resolveSchema(ctx, "crew", "create", crewschemas.Schema("create")), values, nil, rc)

	returnTo := crewEditReturnTo(id, r.URL.Query().Get("return_to"))
	h.page.RenderPartial(w, CrewEditModalPage(id, returnTo, formContent, t))
}

func crewEditReturnTo(id int64, requested string) string {
	if requested == "/crews" || requested == fmt.Sprintf("/crew/%d", id) {
		return requested
	}
	return fmt.Sprintf("/crew/%d", id)
}

func CrewEditModalPage(id int64, returnTo string, formContent mi.H, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}

	action := fmt.Sprintf("/crews/dynamic/%d", id)

	return func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("w-full"),
			b.H2(mi.Class("text-lg font-semibold text-gray-900 dark:text-white mb-4"), t("action.edit")),
			b.Form(
				mi.ID("crew-edit-form"),
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

func (h *UIHandler) CrewUpdateDynamic(w http.ResponseWriter, r *http.Request) {
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

	name := r.FormValue("name")
	description := r.FormValue("description")
	responsible := r.FormValue("responsible_user_id")
	memberRows := fe.ParseIndexedRepeatable(r.Form, fe.Field{Name: "members"})

	var responsibleID *int64
	if parsed, err := strconv.ParseInt(responsible, 10, 64); err == nil && parsed > 0 {
		responsibleID = &parsed
	}

	members := make([]int64, 0, len(memberRows))
	seen := make(map[int64]bool)
	for _, item := range memberRows {
		row, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		memberID := toInt64(row["member"])
		if memberID > 0 && !seen[memberID] {
			seen[memberID] = true
			members = append(members, memberID)
		}
	}

	errors := map[string]string{}
	if name == "" {
		errors["name"] = t("crew.error.name_required")
	}
	if responsibleID == nil {
		errors["responsible_user_id"] = t("crew.error.responsible_required")
	}
	if len(members) < 2 {
		errors["members"] = t("crew.error.min_members")
	}

	returnTo := crewEditReturnTo(id, r.FormValue("return_to"))
	values := map[string]interface{}{
		"name":                name,
		"description":         description,
		"responsible_user_id": responsible,
		"members":             memberRows,
	}

	if len(errors) > 0 {
		formEng := &fe.FormEngine{Translator: h.formEngineTranslator()}
		rc := fe.RenderContext{
			Locale:     h.currentLocale(),
			CSRFToken:  h.GetCSRFToken(w, r),
			Module:     "crew",
			Form:       "crew",
			Action:     fmt.Sprintf("/crews/dynamic/%d", id),
			Method:     "post",
			IsPartial:  true,
			IsEdit:     true,
			EntityID:   id,
			EntityType: "crew",
		}
		formContent := formEng.Render(ctx, resolveSchema(ctx, "crew", "create", crewschemas.Schema("create")), values, errors, rc)
		h.page.RenderPartial(w, CrewEditModalPage(id, returnTo, formContent, t))
		return
	}

	if responsibleID != nil && !seen[*responsibleID] {
		members = append(members, *responsibleID)
	}

	_, err = h.xoluClient.Patch(ctx, "crews", id, map[string]any{
		"name":                name,
		"description":         description,
		"responsible_user_id": responsibleID,
		"members":             members,
	})
	if err != nil {
		h.logger.Error("failed to update crew", "crew_id", id, "error", err)
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

// AUX FUNC
func toInt64Slice(v any) []int64 {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case string:
		var result []int64
		if err := json.Unmarshal([]byte(val), &result); err == nil {
			return result
		}

	case []byte:
		var result []int64
		if err := json.Unmarshal(val, &result); err == nil {
			return result
		}

	case []any:
		result := make([]int64, 0, len(val))
		for _, item := range val {
			result = append(result, toInt64(item))
		}
		return result

	case []int64:
		return val
	}

	return nil
}
