package ui

import (
	"context"
	"net/http"

	mi "github.com/ha1tch/minty"

	"github.com/ha1tch/seam-ui/internal/auth"
	fe "github.com/ha1tch/seam-ui/internal/formengine"
	developerschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/developer"
)

// View-as (campaign DEV-22): a developer looks at the application as a user with
// a lower role would. The role travels in the signed session (auth.SessionData
// .ViewAs); Router.contextFor makes the request's claims carry it; the routes
// below and the layout notice read the real role. The handlers here only choose
// and leave; they are reachable while viewing as any role, because the Developer
// module checks the real one.

// DeveloperViewAs handles POST /developer/view-as: field "role" is the role to
// view the application as. Choosing the developer role leaves view-as.
func (h *UIHandler) DeveloperViewAs(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, developerMaxBody)
	if err := r.ParseForm(); err != nil {
		developerRedirect(w, r, "/developer/overview", "error", "bad_request")
		return
	}
	role := r.FormValue("role")
	if role == auth.RoleDeveloper {
		role = ""
	}
	h.setViewAs(w, r, role)
}

// DeveloperViewAsExit handles GET /developer/view-as/exit, the Exit of the
// notice. It is a link, like Logout, and harmless as one: all it can do is give
// the caller back their own role.
func (h *UIHandler) DeveloperViewAsExit(w http.ResponseWriter, r *http.Request) {
	h.setViewAs(w, r, "")
}

func (h *UIHandler) setViewAs(w http.ResponseWriter, r *http.Request, role string) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || h.sessionManager == nil {
		h.Error403(w, r)
		return
	}
	real := auth.RealRole(r.Context(), claims)
	// leaving is always allowed; choosing a role needs the flag and the right
	if role != "" && !h.config.mayViewAs(real, role) {
		developerRedirect(w, r, "/developer/overview", "error", "bad_request")
		return
	}
	if err := h.sessionManager.SetViewAs(w, r, role); err != nil {
		h.logger.Warn("developer: view-as not set", "error", err)
		developerRedirect(w, r, "/developer/overview", "error", "save_failed")
		return
	}
	action, msg := "set", "view_as_set"
	if role == "" {
		action, msg = "exit", "view_as_exit"
	}
	h.auditLogger.LogDeveloperChange(r.Context(), r, claims.UserID, claims.TenantID, "view_as", role,
		map[string]any{"action": action, "real_role": real})
	developerRedirect(w, r, "/developer/overview", "msg", msg)
}

// viewAsForm is the role select of the Overview's View as section.
// viewAsSource is the form's list of roles (x-seam-source in view_as.json):
// the roles this instance lets a developer view as, named as everywhere else.
const viewAsSource = "developer:view-as-roles"

type viewAsSources struct {
	roles []string
	t     func(key string, args ...any) string
}

// Resolve implements formengine.SourceResolver.
func (s viewAsSources) Resolve(_ context.Context, source string) ([]fe.Option, error) {
	if source != viewAsSource {
		return []fe.Option{}, nil
	}
	opts := make([]fe.Option, 0, len(s.roles))
	for _, role := range s.roles {
		opts = append(opts, fe.Option{Value: role, Label: s.t("user.role." + role)})
	}
	return opts, nil
}

func (h *UIHandler) viewAsForm(ctx context.Context, data DeveloperPageData) mi.H {
	engine := &fe.FormEngine{Translator: h.formEngineTranslator(),
		Sources: viewAsSources{roles: h.config.viewAsRoles(), t: h.translateFunc(nil)}}
	role := data.ViewAs
	if role == "" {
		role = auth.RoleViewer
	}
	return engine.Render(ctx, resolveSchema(ctx, "developer", "view_as", developerschemas.Schema("view_as")),
		map[string]interface{}{"role": role}, nil,
		fe.RenderContext{Locale: h.currentLocale(), CSRFToken: data.CSRFToken, Module: "developer", Form: "view_as"})
}

func developerViewAsSection(b *mi.Builder, data DeveloperPageData, t func(key string, args ...any) string) mi.Node {
	var actions, body []interface{}
	if data.ViewAs != "" {
		actions = append(actions, b.A(mi.Href("/developer/view-as/exit"), mi.Class(devLinkClass),
			b.Span(mi.Class("material-icons text-lg"), "close"), b.Span(t("developer.viewas.exit"))))
		body = append(body, Alert(t("developer.viewas.current", t("user.role."+data.ViewAs)), "info", false)(b))
	}
	body = append(body, b.Form(mi.Class("space-y-4"), mi.Method("post"), mi.Action("/developer/view-as"),
		CSRFField(data.CSRFToken)(b), data.ViewAsForm(b),
		b.Div(IconButton("visibility", t("developer.viewas.apply"), "primary", mi.Type("submit"))(b))))
	return developerPanel(b, "manage_accounts", t("developer.viewas.section"), t("developer.viewas.note"), actions, false, body...)
}
