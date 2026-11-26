package ui

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	mi "github.com/ha1tch/minty"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

const consumableTypesPath = "/consumable-types"
const consumableTypesEntity = "consumable_types"

func (h *UIHandler) ConsumableTypesList(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	result, err := h.xoluClient.OQL(r.Context(), "SELECT id, name, description FROM consumable_types ORDER BY name")
	if err != nil && !entityNotYetCreated(err) {
		h.logger.Error("failed to list consumable types", "error", err)
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.server_error"))
		return
	}

	var rows []mi.H
	if result != nil {
		for i, row := range result.Data {
			href := fmt.Sprintf("%s/%d", consumableTypesPath, toInt64(row["id"]))
			rows = append(rows, ListRow(i, ListCellLink(toString(row["name"]), href),
				ListCellMuted(toString(row["description"]))))
		}
	}
	title := t("consumable_types.list_title")
	page := ListPage(ListPageConfig{Title: title, CreateURL: consumableTypesPath + "/new", CreateLabel: t("consumable_types.new")},
		nil, ListTable(ListTableConfig{
			Columns:   []ListColumn{{Label: t("field.name")}, {Label: t("field.description")}},
			EmptyIcon: "bubble_chart", EmptyTitle: t("consumable_types.empty"),
		}, rows), nil)
	h.page.Render(w, r, title, consumableTypesPath, page)
}

func (h *UIHandler) ConsumableTypeNew(w http.ResponseWriter, r *http.Request) {
	h.renderConsumableTypeForm(w, r, "", "", "", "")
}

func (h *UIHandler) ConsumableTypeCreate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, t("error.invalid_form_data"), http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.PostForm.Get("name"))
	description := strings.TrimSpace(r.PostForm.Get("description"))
	switch {
	case name == "":
		h.renderConsumableTypeForm(w, r, name, description, t("core.errors.required"), "")
		return
	case utf8.RuneCountInString(name) > 200:
		h.renderConsumableTypeForm(w, r, name, description, t("core.errors.max_length", "200"), "")
		return
	}
	_, err := h.xoluClient.Create(r.Context(), consumableTypesEntity, map[string]any{
		"name": name, "description": description,
	})
	if err != nil {
		h.logger.Error("failed to create consumable type", "error", err)
		h.renderConsumableTypeForm(w, r, name, description, "", t("error.server_error"))
		return
	}
	http.Redirect(w, r, consumableTypesPath, http.StatusSeeOther)
}

func (h *UIHandler) renderConsumableTypeForm(w http.ResponseWriter, r *http.Request, name, description, nameError, message string) {
	t := h.translateFunc(r)
	title := t("consumable_types.new")
	csrf := h.GetCSRFToken(w, r)
	h.page.Render(w, r, title, consumableTypesPath, func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("space-y-6 max-w-2xl"),
			b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), title),
			masterFormCard(b,
				mi.Action(consumableTypesPath),
				CSRFField(csrf)(b),
				masterFormError(b, message),
				masterFormInputErr(b, t("field.name"), "name", name, "text", true, nameError),
				b.Div(
					b.Label(mi.For("description"), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300"), t("field.description")),
					b.Textarea(mi.ID("description"), mi.Name("description"), mi.Attr("rows", "3"),
						mi.Class("mt-1 w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"), description),
				),
				masterFormButtons(b, consumableTypesPath, t),
			),
		)
	})
}

func (h *UIHandler) ConsumableTypeDetail(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("error.not_found"))
		return
	}
	entity, err := h.xoluClient.Get(r.Context(), consumableTypesEntity, id)
	if err != nil {
		var xerr *xolu.Error
		if errors.As(err, &xerr) && xerr.HTTPStatus == http.StatusNotFound {
			h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("error.not_found"))
		} else {
			h.logger.Error("failed to load consumable type", "id", id, "error", err)
			h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.server_error"))
		}
		return
	}
	name := toString(entity.Data["name"])
	description := toString(entity.Data["description"])
	h.page.Render(w, r, name, consumableTypesPath, func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("space-y-6 max-w-2xl"),
			b.A(mi.Href(consumableTypesPath), mi.Class("text-indigo-600 hover:underline"), t("consumable_types.list_title")),
			b.H1(mi.Class("text-2xl font-bold"), name),
			Card("", func(b *mi.Builder) mi.Node {
				return b.Dl(infoRow(b, t("field.description"), description))
			})(b),
		)
	})
}
