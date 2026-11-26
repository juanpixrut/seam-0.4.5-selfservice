package ui

import (
	"fmt"

	mi "github.com/ha1tch/minty"
)

// ProfileData holds data for the user profile page.
//
// ProfileFormContent/PreferencesFormContent are pre-rendered by the
// Profile handler (routes.go), not built here. formengine's i18n
// resolution (resolveEnumOptionLabel/resolveSelectPlaceholder) needs a
// real Translator wired per request via h.formEngineTranslator() --
// confirmed directly by rendering with a nil-Translator engine and
// finding every select showed raw enum values and the hardcoded-
// Spanish placeholder fallback regardless of locale. A shared package-
// level *formengine.FormEngine{} (masters_pages_dynamic.go's own
// mastersEngine pattern) has this identical flaw; it never surfaced
// there because that file is explicitly not wired into any live route
// (its own doc comment says so). AssetCreateDynamic/AssetUpdateDynamic
// construct their engine per-request with a real translator -- that's
// the pattern this follows instead.
type ProfileData struct {
	User                   UserProfile
	AvailableLanguages     []LanguageOption
	AvailableTimezones     []TimezoneOption
	SavedSection           string // "profile" or "preferences" if just saved
	Errors                 map[string]string
	CSRFToken              string
	ProfileFormContent     mi.H
	PreferencesFormContent mi.H
}

// UserProfile represents user data for the profile page.
type UserProfile struct {
	ID          int64
	Email       string
	Name        string
	Role        string
	Locale      string
	Timezone    string
	DarkMode    string
	EmailAlerts bool
	CompactView bool
	CreatedAt   string
	LastLoginAt string
}

// TimezoneOption represents a timezone option.
type TimezoneOption struct {
	Value    string
	Label    string
	Selected bool
}

// ProfilePage renders the user profile page.
func ProfilePage(data ProfileData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	return func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("space-y-6 max-w-2xl"),
			// Header
			b.Div(mi.Class("flex items-center gap-4"),
				// Avatar placeholder
				b.Div(mi.Class("w-16 h-16 rounded-full bg-indigo-100 dark:bg-indigo-900 flex items-center justify-center"),
					b.Span(mi.Class("text-2xl font-bold text-indigo-600 dark:text-indigo-400"),
						avatarInitials(data.User.Name),
					),
				),
				b.Div(
					b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), data.User.Name),
					b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400"), data.User.Email),
					roleBadge(b, data.User.Role, t),
				),
			),

			// Success message if just saved
			profileSavedMessage(b, data.SavedSection, t),

			// Profile Information (MOD-8: name is formengine-rendered --
			// email/role/member-since stay hand-coded, read-only display,
			// not part of the editable schema; formengine has no
			// readonly-field widget type, confirmed directly against
			// widgets.go's registry before assuming one existed)
			Card(t("profile.info_title"), func(b *mi.Builder) mi.Node {
				return b.Form(mi.Class("space-y-4"), mi.Method("post"), mi.Action("/profile"),
					data.ProfileFormContent(b),
					profileFieldReadonly(b, t("field.email"), data.User.Email),
					profileFieldReadonly(b, t("field.role"), roleDisplayName(data.User.Role, t)),
					profileFieldReadonly(b, t("profile.member_since"), data.User.CreatedAt),
					b.Div(mi.Class("flex justify-end pt-4"),
						b.Button(mi.Type("submit"), mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 font-medium"),
							t("profile.update_button"),
						),
					),
				)
			})(b),

			// Preferences (MOD-8: formengine-rendered)
			Card(t("profile.preferences_title"), func(b *mi.Builder) mi.Node {
				return b.Form(mi.Class("space-y-4"), mi.Method("post"), mi.Action("/profile/preferences"),
					data.PreferencesFormContent(b),
					b.Div(mi.Class("flex justify-end pt-4"),
						b.Button(mi.Type("submit"), mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 font-medium"),
							t("profile.save_preferences"),
						),
					),
				)
			})(b),

			// Security
			Card(t("users.section.security"), func(b *mi.Builder) mi.Node {
				return b.Div(mi.Class("space-y-4"),
					b.Div(mi.Class("flex items-center justify-between p-4 border border-gray-200 dark:border-gray-700 rounded-lg"),
						b.Div(
							b.H4(mi.Class("font-medium text-gray-900 dark:text-white"), t("profile.change_password")),
							b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400"), t("profile.update_password_hint")),
						),
						b.A(mi.Href("/profile/password"), mi.Class("px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg text-sm font-medium hover:bg-gray-50 dark:hover:bg-gray-700"),
							t("profile.change_password"),
						),
					),
					b.Div(mi.Class("flex items-center justify-between p-4 border border-gray-200 dark:border-gray-700 rounded-lg"),
						b.Div(
							b.H4(mi.Class("font-medium text-gray-900 dark:text-white"), t("profile.active_sessions")),
							b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400"),
								t("profile.last_login", data.User.LastLoginAt),
							),
						),
						b.Button(mi.Type("button"), mi.Class("px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg text-sm font-medium hover:bg-gray-50 dark:hover:bg-gray-700"),
							mi.HxPost("/api/v1/auth/logout-all"),
							mi.HxConfirm(t("profile.confirm_sign_out_others")),
							t("profile.sign_out_others"),
						),
					),
				)
			})(b),
		)
	}
}

// PasswordChangeData holds data for the password change page.
//
// FormContent is pre-rendered by the PasswordChange handler
// (routes.go), matching ProfileData's own pattern -- see its doc
// comment for why a real per-request Translator is required.
type PasswordChangeData struct {
	Error       string
	Success     bool
	CSRFToken   string
	FormContent mi.H
}

// PasswordChangePage renders the password change page.
func PasswordChangePage(data PasswordChangeData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	return func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("space-y-6 max-w-md"),
			b.Div(mi.Class("flex items-center gap-4"),
				b.A(mi.Href("/profile"), mi.Class("text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"),
					b.Span(mi.Class("material-icons"), "arrow_back"),
				),
				b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), t("profile.change_password")),
			),

			// Error message
			passwordChangeMessage(b, data, t),

			// Password form (MOD-8: formengine-rendered)
			Card("", func(b *mi.Builder) mi.Node {
				return b.Form(mi.Class("space-y-4"), mi.Method("post"), mi.Action("/profile/password"),
					data.FormContent(b),
					b.Div(mi.Class("flex justify-end gap-3 pt-4"),
						b.A(mi.Href("/profile"), mi.Class("px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg font-medium hover:bg-gray-50 dark:hover:bg-gray-700"),
							t("action.cancel"),
						),
						b.Button(mi.Type("submit"), mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 font-medium"),
							t("profile.change_password"),
						),
					),
				)
			})(b),
		)
	}
}

func passwordChangeMessage(b *mi.Builder, data PasswordChangeData, t func(key string, args ...any) string) mi.Node {
	if data.Success {
		return b.Div(mi.Class("p-4 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg"),
			b.Div(mi.Class("flex items-center gap-2"),
				b.Span(mi.Class("material-icons text-green-600 dark:text-green-400"), "check_circle"),
				b.P(mi.Class("text-sm text-green-700 dark:text-green-300"), t("profile.password_changed")),
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

func avatarInitials(name string) string {
	if name == "" {
		return "?"
	}
	// Get first letter of first and last name
	parts := splitName(name)
	if len(parts) >= 2 {
		return string(parts[0][0]) + string(parts[len(parts)-1][0])
	}
	return string(name[0])
}

func splitName(name string) []string {
	var parts []string
	var current string
	for _, c := range name {
		if c == ' ' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func roleBadge(b *mi.Builder, role string, t func(key string, args ...any) string) mi.Node {
	colorClass := "bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300"
	switch role {
	case "admin":
		colorClass = "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-300"
	case "manager":
		colorClass = "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300"
	case "operator":
		colorClass = "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300"
	}
	return b.Span(mi.Class("inline-flex items-center px-2 py-0.5 rounded text-xs font-medium "+colorClass),
		roleDisplayName(role, t),
	)
}

func roleDisplayName(role string, t func(key string, args ...any) string) string {
	switch role {
	case "admin":
		return t("users.role.administrator")
	case "manager":
		return t("users.role.manager")
	case "operator":
		return t("users.role.operator")
	case "viewer":
		return t("users.role.viewer")
	default:
		return role
	}
}

func profileSavedMessage(b *mi.Builder, section string, t func(key string, args ...any) string) mi.Node {
	if section == "" {
		return b.Span()
	}
	var msg string
	switch section {
	case "profile":
		msg = t("profile.updated_successfully")
	case "preferences":
		msg = t("profile.preferences_saved")
	case "password":
		msg = t("profile.password_changed")
	default:
		msg = t("profile.changes_saved_default")
	}
	return b.Div(mi.Class("p-4 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg"),
		b.Div(mi.Class("flex items-center gap-2"),
			b.Span(mi.Class("material-icons text-green-600 dark:text-green-400"), "check_circle"),
			b.P(mi.Class("text-sm text-green-700 dark:text-green-300"), msg),
		),
	)
}

func profileField(b *mi.Builder, label, name, inputType, value string, err interface{}, required bool) mi.Node {
	inputClass := "w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
	if err != nil {
		inputClass = "w-full px-3 py-2 border border-red-500 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
	}

	var inputNode mi.Node
	if required {
		inputNode = b.Input(mi.Type(inputType), mi.Name(name), mi.ID(name), mi.Value(value), mi.Class(inputClass), mi.Required())
	} else {
		inputNode = b.Input(mi.Type(inputType), mi.Name(name), mi.ID(name), mi.Value(value), mi.Class(inputClass))
	}

	errorNode := b.Span()
	if err != nil {
		errorNode = b.P(mi.Class("mt-1 text-sm text-red-500"), fmt.Sprintf("%v", err))
	}

	return b.Div(
		b.Label(mi.For(name), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"), label),
		inputNode,
		errorNode,
	)
}

func profileFieldReadonly(b *mi.Builder, label, value string) mi.Node {
	return b.Div(
		b.Label(mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"), label),
		b.Div(mi.Class("px-3 py-2 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg text-gray-600 dark:text-gray-400"),
			value,
		),
	)
}

func profileSelect(b *mi.Builder, label, name string, options []TimezoneOption) mi.Node {
	selectOpts := []interface{}{
		mi.Name(name),
		mi.ID(name),
		mi.Class("w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"),
	}
	for _, opt := range options {
		if opt.Selected {
			selectOpts = append(selectOpts, b.Option(mi.Value(opt.Value), mi.Selected(), opt.Label))
		} else {
			selectOpts = append(selectOpts, b.Option(mi.Value(opt.Value), opt.Label))
		}
	}

	return b.Div(
		b.Label(mi.For(name), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"), label),
		b.Select(selectOpts...),
	)
}

func profileToggle(b *mi.Builder, label, name string, checked bool, description string) mi.Node {
	var checkbox mi.Node
	if checked {
		checkbox = b.Input(mi.Type("checkbox"), mi.Name(name), mi.ID(name), mi.Class("sr-only peer"), mi.Checked())
	} else {
		checkbox = b.Input(mi.Type("checkbox"), mi.Name(name), mi.ID(name), mi.Class("sr-only peer"))
	}

	return b.Div(mi.Class("flex items-center justify-between py-2"),
		b.Div(
			b.Label(mi.For(name), mi.Class("text-sm font-medium text-gray-900 dark:text-white"), label),
			b.P(mi.Class("text-xs text-gray-500 dark:text-gray-400"), description),
		),
		b.Label(mi.Class("relative inline-flex items-center cursor-pointer"),
			checkbox,
			b.Div(mi.Class("w-11 h-6 bg-gray-200 peer-focus:ring-2 peer-focus:ring-indigo-500 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-indigo-600")),
		),
	)
}

func languageOptionsToSelect(langs []LanguageOption) []TimezoneOption {
	result := make([]TimezoneOption, len(langs))
	for i, l := range langs {
		result[i] = TimezoneOption{
			Value:    l.Code,
			Label:    l.Name,
			Selected: l.Selected,
		}
	}
	return result
}

// CommonTimezones returns a list of common timezones.
func CommonTimezones(selected string) []TimezoneOption {
	zones := []struct{ value, label string }{
		{"UTC", "UTC"},
		{"America/New_York", "Eastern Time (US)"},
		{"America/Chicago", "Central Time (US)"},
		{"America/Denver", "Mountain Time (US)"},
		{"America/Los_Angeles", "Pacific Time (US)"},
		{"America/Montevideo", "Uruguay"},
		{"America/Sao_Paulo", "Brazil (São Paulo)"},
		{"America/Buenos_Aires", "Argentina"},
		{"America/Santiago", "Chile"},
		{"America/Bogota", "Colombia"},
		{"America/Mexico_City", "Mexico"},
		{"Europe/London", "London"},
		{"Europe/Paris", "Paris"},
		{"Europe/Berlin", "Berlin"},
		{"Europe/Madrid", "Madrid"},
		{"Asia/Tokyo", "Tokyo"},
		{"Asia/Shanghai", "Shanghai"},
		{"Asia/Singapore", "Singapore"},
		{"Australia/Sydney", "Sydney"},
	}

	result := make([]TimezoneOption, len(zones))
	for i, z := range zones {
		result[i] = TimezoneOption{
			Value:    z.value,
			Label:    z.label,
			Selected: z.value == selected,
		}
	}
	return result
}
