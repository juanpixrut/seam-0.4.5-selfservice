package ui

import (
	"fmt"

	mi "github.com/ha1tch/minty"
)

// =============================================================================
// ADMIN: USER MANAGEMENT
// =============================================================================

// UsersListData holds data for the users list page.
type UsersListData struct {
	Users      []UserRow
	Total      int
	RoleFilter string
	CanCreate  bool
	CanEdit    bool
	CanDelete  bool
}

// UserRow represents a user in the list.
type UserRow struct {
	ID          int64
	Email       string
	Name        string
	Role        string
	Active      bool
	LastLoginAt string
	CreatedAt   string
}

// UsersListPage renders the users list page.
func UsersListPage(data UsersListData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	const card = "bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700"
	rows := make([]mi.H, len(data.Users))
	for i, user := range data.Users {
		rows[i] = ListRow(i,
			// Avatar + name + email composite cell
			ListCellRich(func(b *mi.Builder) mi.Node {
				return b.Div(mi.Class("flex items-center gap-3"),
					b.Div(mi.Class("w-9 h-9 rounded-full bg-indigo-100 dark:bg-indigo-900 flex items-center justify-center flex-shrink-0"),
						b.Span(mi.Class("text-sm font-medium text-indigo-600 dark:text-indigo-400"),
							userInitialsFromName(user.Name),
						),
					),
					b.Div(
						b.Div(mi.Class("font-medium text-gray-900 dark:text-white text-sm"), user.Name),
						b.Div(mi.Class("text-xs text-gray-500 dark:text-gray-400"), user.Email),
					),
				)
			}),
			ListCellRich(func(b *mi.Builder) mi.Node { return userRoleBadge(b, user.Role, t) }),
			ListCellRich(func(b *mi.Builder) mi.Node { return userStatusBadge(b, user.Active, t) }),
			ListCellMuted(user.LastLoginAt),
			ListActions(ListActionsConfig{
				EditURL: func() string {
					if data.CanEdit {
						return fmt.Sprintf("/admin/users/%d", user.ID)
					}
					return ""
				}(),
				DeleteURL: func() string {
					if data.CanDelete {
						return fmt.Sprintf("/admin/users/%d/delete", user.ID)
					}
					return ""
				}(),
				T: t,
			}),
		)
	}
	createURL := ""
	if data.CanCreate {
		createURL = "/admin/users/new"
	}
	return func(b *mi.Builder) mi.Node {
		// ListPage header handles the conditional create button
		header := ListPage(ListPageConfig{
			Title: t("users.list_title"), CreateURL: createURL,
			CreateLabel: t("users.new"), CreateIcon: "person_add",
		}, nil, nil, nil)(b)
		// Card: role tabs + table (table omits its own card wrapper)
		table := b.Div(mi.Class(card),
			b.Div(mi.Class("border-b border-gray-200 dark:border-gray-700"),
				b.Nav(mi.Class("flex gap-4 px-4"), mi.Attr("aria-label", t("layout.tabs")),
					userRoleTab(b, t("users.tab.all"), "", data.RoleFilter),
					userRoleTab(b, t("users.tab.admins"), "admin", data.RoleFilter),
					userRoleTab(b, t("users.tab.managers"), "manager", data.RoleFilter),
					userRoleTab(b, t("users.tab.operators"), "operator", data.RoleFilter),
					userRoleTab(b, t("users.tab.viewers"), "viewer", data.RoleFilter),
				),
			),
			ListTable(ListTableConfig{
				Columns: []ListColumn{
					{Label: t("users.column.user")}, {Label: t("field.role")}, {Label: t("field.status")},
					{Label: t("users.column.last_login")}, {Label: t("asset.list.actions"), Right: true},
				},
				EmptyIcon: "people_outline", EmptyTitle: t("users.empty_title"),
				EmptyMessage: t("users.empty_message"),
				SkipCard:     true,
			}, rows)(b),
		)
		return b.Div(mi.Class("space-y-6"), header, table)
	}
}

func userRoleTab(b *mi.Builder, label, role, current string) mi.Node {
	active := role == current
	baseClass := "py-3 px-1 border-b-2 text-sm font-medium transition-colors"
	if active {
		return b.A(mi.Href("/admin/users?role="+role), mi.Class(baseClass+" border-indigo-500 text-indigo-600 dark:text-indigo-400"),
			label,
		)
	}
	return b.A(mi.Href("/admin/users?role="+role), mi.Class(baseClass+" border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300"),
		label,
	)
}

func userInitialsFromName(name string) string {
	if name == "" {
		return "?"
	}
	parts := splitName(name)
	if len(parts) >= 2 {
		return string(parts[0][0]) + string(parts[len(parts)-1][0])
	}
	return string(name[0])
}

func userRoleBadge(b *mi.Builder, role string, t func(key string, args ...any) string) mi.Node {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	colorClass := "bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300"
	label := role
	switch role {
	case "admin":
		colorClass = "bg-red-100 text-red-800 dark:bg-red-900/50 dark:text-red-300"
		label = t("users.role.admin")
	case "manager":
		colorClass = "bg-blue-100 text-blue-800 dark:bg-blue-900/50 dark:text-blue-300"
		label = t("users.role.manager")
	case "operator":
		colorClass = "bg-green-100 text-green-800 dark:bg-green-900/50 dark:text-green-300"
		label = t("users.role.operator")
	case "viewer":
		colorClass = "bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300"
		label = t("users.role.viewer")
	}
	return b.Span(mi.Class("inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium "+colorClass),
		label,
	)
}

func userStatusBadge(b *mi.Builder, active bool, t func(key string, args ...any) string) mi.Node {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	if active {
		return b.Span(mi.Class("inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900/50 dark:text-green-300"),
			b.Span(mi.Class("w-1.5 h-1.5 rounded-full bg-green-500")),
			t("status.active"),
		)
	}
	return b.Span(mi.Class("inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300"),
		b.Span(mi.Class("w-1.5 h-1.5 rounded-full bg-gray-400")),
		t("status.disabled"),
	)
}

func userActions(b *mi.Builder, user UserRow, canEdit, canDelete bool) mi.Node {
	if !canEdit && !canDelete {
		return b.Span(mi.Class("text-gray-400"), "-")
	}

	actions := []interface{}{mi.Class("flex items-center gap-2")}

	if canEdit {
		actions = append(actions,
			b.A(mi.Href(fmt.Sprintf("/admin/users/%d", user.ID)), mi.Class("text-indigo-600 hover:text-indigo-800 dark:text-indigo-400 dark:hover:text-indigo-300"),
				b.Span(mi.Class("material-icons text-lg"), "edit"),
			),
		)
	}

	if canDelete && user.Role != "admin" { // Can't delete admins from list
		actions = append(actions,
			b.Button(
				mi.Type("button"),
				mi.Class("text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-300"),
				mi.HxDelete(fmt.Sprintf("/api/v1/users/%d", user.ID)),
				mi.HxConfirm(fmt.Sprintf("Are you sure you want to delete %s?", user.Name)),
				mi.HxTarget("closest tr"),
				mi.HxSwap("outerHTML"),
				b.Span(mi.Class("material-icons text-lg"), "delete"),
			),
		)
	}

	return b.Div(actions...)
}

// =============================================================================
// USER CREATE/EDIT FORM
// =============================================================================

// UserFormData holds data for the user create/edit form.
// UserFormData holds data for the user create/edit form.
//
// FormContent is pre-rendered by the AdminUserNew/AdminUserCreate/
// AdminUserEdit handlers (routes.go) -- create and edit use different
// formengine schemas (user_create.json has email/password fields edit
// doesn't; user_edit.json has active edit has and create doesn't), so
// this is always exactly the right one for IsEdit's value, not a
// single shared render. Matches ProfileData's own pattern -- see its
// doc comment for why a real per-request Translator is required.
type UserFormData struct {
	IsEdit      bool
	User        UserFormValues
	Errors      map[string]string
	CSRFToken   string
	FormContent mi.H
}

// UserFormValues holds the form field values.
type UserFormValues struct {
	ID     int64
	Email  string
	Name   string
	Role   string
	Active bool
}

// UserFormPage renders the user create/edit form.
func UserFormPage(data UserFormData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	return func(b *mi.Builder) mi.Node {
		title := t("users.new")
		if data.IsEdit {
			title = t("users.edit_title")
		}

		action := "/admin/users"
		if data.IsEdit {
			action = fmt.Sprintf("/admin/users/%d", data.User.ID)
		}

		return b.Div(mi.Class("space-y-6 max-w-2xl"),
			// Header
			b.Div(mi.Class("flex items-center gap-4"),
				b.A(mi.Href("/admin/users"), mi.Class("text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"),
					b.Span(mi.Class("material-icons"), "arrow_back"),
				),
				b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), title),
			),

			// Form (MOD-8: formengine-rendered)
			Card("", func(b *mi.Builder) mi.Node {
				return b.Form(mi.Class("space-y-4"), mi.Method("post"), mi.Action(action),
					// Email stays hand-coded on edit: read-only/disabled
					// display, matching Profile's own precedent -- formengine
					// has no readonly widget type (confirmed directly against
					// widgets.go's registry before assuming one existed). On
					// create, email is a real, editable formengine field
					// (part of user_create.json), rendered inline below via
					// data.FormContent, not here.
					userEmailFieldIfEdit(b, data, t),
					data.FormContent(b),
					b.Div(mi.Class("flex justify-end gap-3 pt-4"),
						b.A(mi.Href("/admin/users"), mi.Class("px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg font-medium hover:bg-gray-50 dark:hover:bg-gray-700"),
							t("action.cancel"),
						),
						b.Button(mi.Type("submit"), mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 font-medium"),
							conditionalText(data.IsEdit, t("assettypes.save_changes"), t("users.create")),
						),
					),
				)
			})(b),

			// Admin actions (edit only)
			userAdminActions(b, data, t),
		)
	}
}

// userEmailFieldIfEdit renders email as a disabled (not just visually
// read-only) input, matching the exact old hand-coded behaviour, for
// IsEdit only -- on create, email is a real, editable field, part of
// user_create.json's schema and rendered via data.FormContent instead.
// formengine has no disabled/readonly widget type (confirmed directly
// against widgets.go's registry, same finding as Profile's readonly
// fields), so this stays hand-coded here rather than attempting a
// formengine field with no way to express "disabled".
func userEmailFieldIfEdit(b *mi.Builder, data UserFormData, t func(key string, args ...any) string) mi.Node {
	if !data.IsEdit {
		return b.Span()
	}
	return b.Div(
		b.Label(mi.For("email_display"), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"),
			t("field.email"),
		),
		b.Input(mi.Type("email"), mi.ID("email_display"), mi.Value(data.User.Email),
			mi.Class("w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-gray-100 dark:bg-gray-800 text-gray-900 dark:text-gray-100 cursor-not-allowed"),
			mi.Disabled(),
		),
	)
}

func userAdminActions(b *mi.Builder, data UserFormData, t func(key string, args ...any) string) mi.Node {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	if !data.IsEdit {
		return b.Span()
	}

	return b.Div(mi.Class("space-y-4"),
		// Reset Password
		Card(t("users.section.security"), func(b *mi.Builder) mi.Node {
			return b.Div(mi.Class("space-y-4"),
				b.Div(mi.Class("flex items-center justify-between"),
					b.Div(
						b.H4(mi.Class("font-medium text-gray-900 dark:text-white"), t("users.reset_password")),
						b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400"), t("users.reset_password_hint")),
					),
					b.A(mi.Href(fmt.Sprintf("/admin/users/%d/password", data.User.ID)), mi.Class("px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg text-sm font-medium hover:bg-gray-50 dark:hover:bg-gray-700"),
						t("users.reset_password"),
					),
				),
			)
		})(b),

		// Danger Zone
		Card(t("settings.section.danger_zone"), func(b *mi.Builder) mi.Node {
			return b.Div(mi.Class("p-4 border border-red-200 dark:border-red-800 rounded-lg bg-red-50 dark:bg-red-900/20"),
				b.Div(mi.Class("flex items-center justify-between"),
					b.Div(
						b.H4(mi.Class("font-medium text-red-800 dark:text-red-200"), t("users.delete_user")),
						b.P(mi.Class("text-sm text-red-600 dark:text-red-400"), t("users.delete_user_hint")),
					),
					b.Button(
						mi.Type("button"),
						mi.Class("px-4 py-2 bg-red-600 text-white rounded-lg text-sm font-medium hover:bg-red-700"),
						mi.HxDelete(fmt.Sprintf("/api/v1/users/%d", data.User.ID)),
						mi.HxConfirm(t("users.confirm_delete_user")),
						t("users.delete_user"),
					),
				),
			)
		})(b),
	)
}

func conditionalText(condition bool, ifTrue, ifFalse string) string {
	if condition {
		return ifTrue
	}
	return ifFalse
}

// =============================================================================
// ADMIN PASSWORD RESET
// =============================================================================

// AdminPasswordResetData holds data for admin password reset page.
type AdminPasswordResetData struct {
	UserID    int64
	UserName  string
	Error     string
	Success   bool
	CSRFToken string
}

// AdminPasswordResetPage renders the admin password reset page.
func AdminPasswordResetPage(data AdminPasswordResetData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	return func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("space-y-6 max-w-md"),
			// Header
			b.Div(mi.Class("flex items-center gap-4"),
				b.A(mi.Href(fmt.Sprintf("/admin/users/%d", data.UserID)), mi.Class("text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"),
					b.Span(mi.Class("material-icons"), "arrow_back"),
				),
				b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), t("users.reset_password")),
			),

			// Message
			adminPasswordMessage(b, data, t),

			// Info
			b.Div(mi.Class("text-sm text-gray-600 dark:text-gray-400"),
				t("users.setting_new_password_for"),
				b.Span(mi.Class("font-medium text-gray-900 dark:text-white"), data.UserName),
			),

			// Form
			Card("", func(b *mi.Builder) mi.Node {
				return b.Form(mi.Class("space-y-4"), mi.Method("post"), mi.Action(fmt.Sprintf("/admin/users/%d/password", data.UserID)),
					CSRFField(data.CSRFToken)(b),
					b.Div(
						b.Label(mi.For("new_password"), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"),
							t("users.field.new_password"),
						),
						b.Input(
							mi.Type("password"),
							mi.Name("new_password"),
							mi.ID("new_password"),
							mi.Class("w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"),
							mi.Required(),
							mi.Attr("minlength", "8"),
						),
					),
					b.Div(
						b.Label(mi.For("confirm_password"), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"),
							t("users.field.confirm_password"),
						),
						b.Input(
							mi.Type("password"),
							mi.Name("confirm_password"),
							mi.ID("confirm_password"),
							mi.Class("w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"),
							mi.Required(),
						),
					),
					b.P(mi.Class("text-xs text-gray-500 dark:text-gray-400"), t("users.min_chars_hint")),
					b.Div(mi.Class("flex justify-end gap-3 pt-4"),
						b.A(mi.Href(fmt.Sprintf("/admin/users/%d", data.UserID)), mi.Class("px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg font-medium hover:bg-gray-50 dark:hover:bg-gray-700"),
							t("action.cancel"),
						),
						b.Button(mi.Type("submit"), mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 font-medium"),
							t("users.set_password"),
						),
					),
				)
			})(b),
		)
	}
}

func adminPasswordMessage(b *mi.Builder, data AdminPasswordResetData, t func(key string, args ...any) string) mi.Node {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	if data.Success {
		return b.Div(mi.Class("p-4 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg"),
			b.Div(mi.Class("flex items-center gap-2"),
				b.Span(mi.Class("material-icons text-green-600 dark:text-green-400"), "check_circle"),
				b.P(mi.Class("text-sm text-green-700 dark:text-green-300"), t("users.reset_success")),
			),
		)
	}
	if data.Error != "" {
		return b.Div(mi.Class("p-4 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg"),
			b.Div(mi.Class("flex items-center gap-2"),
				b.Span(mi.Class("material-icons text-red-600 dark:text-red-400"), "error"),
				b.P(mi.Class("text-sm text-red-700 dark:text-red-300"), data.Error),
			),
		)
	}
	return b.Span()
}
