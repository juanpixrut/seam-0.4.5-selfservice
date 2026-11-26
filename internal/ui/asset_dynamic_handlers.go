package ui

import (
	"fmt"
	"net/http"

	fe "github.com/ha1tch/seam-ui/internal/formengine"
	asset "github.com/ha1tch/seam-ui/internal/formengine/schemas/asset"
)

// AssetNewDynamic and AssetEditDynamic are the FE-23 GET handlers
// proving the dynamic form engine against the real Asset entity,
// alongside -- not instead of -- AssetNew/AssetEdit. §13.3's
// coexistence model calls for exactly this: a new, parallel path, so
// FE-24 has both a static and a dynamic rendering of the same real
// asset to actually compare, rather than a destructive rewrite that
// leaves nothing to check parity against.
//
// Routed at /assets/dynamic/new and /assets/dynamic/{id}/edit --
// distinct from /assets/new and /assets/{id}/edit, never colliding
// with them.

// AssetNewDynamic handles GET /assets/dynamic/new.
func (h *UIHandler) AssetNewDynamic(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	isPartial := IsHTMXRequest(r)
	e := &fe.FormEngine{Sources: &AssetSourceResolver{Client: h.xoluClient}, Translator: h.formEngineTranslator()}
	rc := fe.RenderContext{CSRFToken: h.GetCSRFToken(w, r), Locale: h.currentLocale(), IsPartial: isPartial, IsEdit: false, Action: "/assets/dynamic", Method: "post"}
	content := e.Render(ctx, resolveSchema(ctx, "asset", "asset", asset.Schema()), nil, nil, rc)
	wrapped := wrapAssetDynamicForm(h.translateFunc(r), content, "/assets/dynamic", "post", isPartial, false, 0, "")

	if isPartial {
		h.page.RenderPartial(w, wrapped)
	} else {
		http.Redirect(w, r, "/assets?create=1", http.StatusSeeOther)
	}
}

// AssetEditDynamic handles GET /assets/dynamic/{id}/edit.
func (h *UIHandler) AssetEditDynamic(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	t := h.translateFunc(r)

	// Path is /assets/dynamic/{id}/edit -- strip the /edit suffix (5
	// chars) same as AssetEdit does for /assets/{id}/edit, then extract
	// against this route's own, longer prefix.
	path := r.URL.Path
	path = path[:len(path)-5]
	id := extractPathID(path, "/assets/dynamic/")
	if id == 0 {
		h.page.RenderError(w, 404, t("error.not_found"), t("asset.not_found"))
		return
	}

	entity, err := h.xoluClient.Get(ctx, "assets", id)
	if err != nil {
		h.page.RenderError(w, 404, t("error.not_found"), t("asset.not_found"))
		return
	}

	values := AssetValuesFromXolu(entity.Data)

	isPartial := IsHTMXRequest(r)
	action := fmt.Sprintf("/assets/dynamic/%d", id)
	e := &fe.FormEngine{Sources: &AssetSourceResolver{Client: h.xoluClient}, Translator: h.formEngineTranslator()}
	rc := fe.RenderContext{CSRFToken: h.GetCSRFToken(w, r), Locale: h.currentLocale(), IsPartial: isPartial, IsEdit: true, EntityID: int64(id), EntityType: "asset", ActiveTab: r.URL.Query().Get("tab"), Action: action, Method: "post"}
	content := e.Render(ctx, resolveSchema(ctx, "asset", "asset", asset.Schema()), values, nil, rc)
	wrapped := wrapAssetDynamicForm(h.translateFunc(r), content, action, "post", isPartial, true, int64(id), assetNameFromValues(values))

	if isPartial {
		h.page.RenderPartial(w, wrapped)
	} else {
		http.Redirect(w, r, "/assets?edit=1", http.StatusSeeOther)
	}
}
