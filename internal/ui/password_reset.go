package ui

import (
	mi "github.com/ha1tch/minty"
)

// ForgotPasswordData holds data for the forgot password page.
type ForgotPasswordData struct {
	Email     string
	Error     string
	Success   bool
	CSRFToken string
	Locale    string
}

// ForgotPasswordPage renders the forgot password page.
func ForgotPasswordPage(data ForgotPasswordData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	return func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("min-h-screen flex items-center justify-center bg-gray-100 dark:bg-gray-900 px-4"),
			b.Div(mi.Class("max-w-md w-full space-y-8"),
				// Header
				b.Div(mi.Class("text-center"),
					b.H1(mi.Class("text-3xl font-bold text-gray-900 dark:text-white"), t("password_reset.forgot_title")),
					b.P(mi.Class("mt-2 text-gray-600 dark:text-gray-400"),
						t("password_reset.forgot_subtitle"),
					),
				),

				// Success message
				forgotPasswordSuccess(b, data, t),

				// Error message
				forgotPasswordError(b, data),

				// Form (only show if not success)
				forgotPasswordForm(b, data, t),

				// Back to login link
				b.Div(mi.Class("text-center"),
					b.A(mi.Href("/login"), mi.Class("text-sm text-indigo-600 hover:text-indigo-500 dark:text-indigo-400"),
						t("password_reset.back_to_login"),
					),
				),
			),
		)
	}
}

func forgotPasswordSuccess(b *mi.Builder, data ForgotPasswordData, t func(key string, args ...any) string) mi.Node {
	if !data.Success {
		return b.Text("")
	}
	return b.Div(mi.Class("bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg p-4"),
		b.Div(mi.Class("flex"),
			b.Span(mi.Class("material-icons text-green-600 dark:text-green-400 mr-3"), "check_circle"),
			b.Div(
				b.P(mi.Class("text-sm font-medium text-green-800 dark:text-green-200"),
					t("password_reset.check_email"),
				),
				b.P(mi.Class("text-sm text-green-700 dark:text-green-300 mt-1"),
					t("password_reset.forgot_success_message"),
				),
			),
		),
	)
}

func forgotPasswordError(b *mi.Builder, data ForgotPasswordData) mi.Node {
	if data.Error == "" {
		return b.Text("")
	}
	return b.Div(mi.Class("bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-4"),
		b.Div(mi.Class("flex"),
			b.Span(mi.Class("material-icons text-red-600 dark:text-red-400 mr-3"), "error"),
			b.P(mi.Class("text-sm text-red-700 dark:text-red-300"), data.Error),
		),
	)
}

func forgotPasswordForm(b *mi.Builder, data ForgotPasswordData, t func(key string, args ...any) string) mi.Node {
	if data.Success {
		return b.Text("")
	}
	return b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm p-8"),
		b.Form(mi.Class("space-y-6"), mi.Method("post"), mi.Action("/forgot-password"),
			b.Input(mi.Type("hidden"), mi.Name("_csrf"), mi.Value(data.CSRFToken)),
			b.Div(
				b.Label(mi.For("email"), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300"),
					t("field.email"),
				),
				b.Input(
					mi.Type("email"),
					mi.Name("email"),
					mi.ID("email"),
					mi.Value(data.Email),
					mi.Class("mt-1 w-full px-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-indigo-500 focus:border-transparent"),
					mi.Placeholder(t("password_reset.email_placeholder")),
					mi.Required(),
					mi.Attr("autocomplete", "email"),
				),
			),
			b.Button(
				mi.Type("submit"),
				mi.Class("w-full flex justify-center py-3 px-4 border border-transparent rounded-lg text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"),
				t("password_reset.send_link"),
			),
		),
	)
}

// ResetPasswordData holds data for the reset password page.
type ResetPasswordData struct {
	Token     string
	Email     string
	Error     string
	Success   bool
	Invalid   bool // Token is invalid or expired
	CSRFToken string
	Locale    string
}

// ResetPasswordPage renders the reset password page.
func ResetPasswordPage(data ResetPasswordData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	return func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("min-h-screen flex items-center justify-center bg-gray-100 dark:bg-gray-900 px-4"),
			b.Div(mi.Class("max-w-md w-full space-y-8"),
				// Header
				b.Div(mi.Class("text-center"),
					b.H1(mi.Class("text-3xl font-bold text-gray-900 dark:text-white"), t("password_reset.reset_title")),
					resetPasswordSubtitle(b, data, t),
				),

				// Success message
				resetPasswordSuccess(b, data, t),

				// Invalid token message
				resetPasswordInvalid(b, data, t),

				// Error message
				resetPasswordError(b, data),

				// Form
				resetPasswordForm(b, data, t),
			),
		)
	}
}

func resetPasswordSubtitle(b *mi.Builder, data ResetPasswordData, t func(key string, args ...any) string) mi.Node {
	if data.Success || data.Invalid {
		return b.Text("")
	}
	return b.P(mi.Class("mt-2 text-gray-600 dark:text-gray-400"),
		t("password_reset.reset_subtitle"),
	)
}

func resetPasswordSuccess(b *mi.Builder, data ResetPasswordData, t func(key string, args ...any) string) mi.Node {
	if !data.Success {
		return b.Text("")
	}
	return b.Div(mi.Class("space-y-4"),
		b.Div(mi.Class("bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg p-4"),
			b.Div(mi.Class("flex"),
				b.Span(mi.Class("material-icons text-green-600 dark:text-green-400 mr-3"), "check_circle"),
				b.Div(
					b.P(mi.Class("text-sm font-medium text-green-800 dark:text-green-200"),
						t("password_reset.reset_success_title"),
					),
					b.P(mi.Class("text-sm text-green-700 dark:text-green-300 mt-1"),
						t("password_reset.reset_success_message"),
					),
				),
			),
		),
		b.Div(mi.Class("text-center"),
			b.A(mi.Href("/login"), mi.Class("inline-flex items-center px-4 py-2 border border-transparent rounded-lg text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700"),
				t("password_reset.go_to_login"),
			),
		),
	)
}

func resetPasswordInvalid(b *mi.Builder, data ResetPasswordData, t func(key string, args ...any) string) mi.Node {
	if !data.Invalid {
		return b.Text("")
	}
	return b.Div(mi.Class("space-y-4"),
		b.Div(mi.Class("bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-4"),
			b.Div(mi.Class("flex"),
				b.Span(mi.Class("material-icons text-yellow-600 dark:text-yellow-400 mr-3"), "warning"),
				b.Div(
					b.P(mi.Class("text-sm font-medium text-yellow-800 dark:text-yellow-200"),
						t("password_reset.invalid_title"),
					),
					b.P(mi.Class("text-sm text-yellow-700 dark:text-yellow-300 mt-1"),
						t("password_reset.invalid_message"),
					),
				),
			),
		),
		b.Div(mi.Class("text-center space-x-4"),
			b.A(mi.Href("/forgot-password"), mi.Class("inline-flex items-center px-4 py-2 border border-transparent rounded-lg text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700"),
				t("password_reset.request_new_link"),
			),
			b.A(mi.Href("/login"), mi.Class("text-sm text-indigo-600 hover:text-indigo-500 dark:text-indigo-400"),
				t("password_reset.back_to_login_plain"),
			),
		),
	)
}

func resetPasswordError(b *mi.Builder, data ResetPasswordData) mi.Node {
	if data.Error == "" || data.Invalid || data.Success {
		return b.Text("")
	}
	return b.Div(mi.Class("bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-4"),
		b.Div(mi.Class("flex"),
			b.Span(mi.Class("material-icons text-red-600 dark:text-red-400 mr-3"), "error"),
			b.P(mi.Class("text-sm text-red-700 dark:text-red-300"), data.Error),
		),
	)
}

func resetPasswordForm(b *mi.Builder, data ResetPasswordData, t func(key string, args ...any) string) mi.Node {
	if data.Success || data.Invalid {
		return b.Text("")
	}
	return b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm p-8"),
		b.Form(mi.Class("space-y-6"), mi.Method("post"), mi.Action("/reset-password"),
			// Hidden token and CSRF fields
			b.Input(mi.Type("hidden"), mi.Name("token"), mi.Value(data.Token)),
			b.Input(mi.Type("hidden"), mi.Name("_csrf"), mi.Value(data.CSRFToken)),

			// Email display (read-only)
			b.Div(
				b.Label(mi.For("email"), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300"),
					t("field.email"),
				),
				b.Input(
					mi.Type("email"),
					mi.ID("email"),
					mi.Value(data.Email),
					mi.Class("mt-1 w-full px-4 py-3 border border-gray-200 dark:border-gray-600 rounded-lg bg-gray-50 dark:bg-gray-700 text-gray-500 dark:text-gray-400"),
					mi.Attr("readonly", "readonly"),
				),
			),

			// New password
			b.Div(
				b.Label(mi.For("password"), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300"),
					t("users.field.new_password"),
				),
				b.Input(
					mi.Type("password"),
					mi.Name("password"),
					mi.ID("password"),
					mi.Class("mt-1 w-full px-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-indigo-500 focus:border-transparent"),
					mi.Placeholder(t("password_reset.new_password_placeholder")),
					mi.Required(),
					mi.Attr("minlength", "8"),
					mi.Attr("autocomplete", "new-password"),
				),
				b.P(mi.Class("mt-1 text-xs text-gray-500 dark:text-gray-400"),
					t("users.min_chars_hint"),
				),
			),

			// Confirm password
			b.Div(
				b.Label(mi.For("confirm_password"), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300"),
					t("profile.confirm_new_password"),
				),
				b.Input(
					mi.Type("password"),
					mi.Name("confirm_password"),
					mi.ID("confirm_password"),
					mi.Class("mt-1 w-full px-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-indigo-500 focus:border-transparent"),
					mi.Placeholder(t("password_reset.confirm_password_placeholder")),
					mi.Required(),
					mi.Attr("minlength", "8"),
					mi.Attr("autocomplete", "new-password"),
				),
			),

			b.Button(
				mi.Type("submit"),
				mi.Class("w-full flex justify-center py-3 px-4 border border-transparent rounded-lg text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"),
				t("password_reset.reset_title"),
			),
		),
	)
}

// RenderForgotPasswordPage renders the forgot password page as a standalone HTML page.
func RenderForgotPasswordPage(w interface{ Write([]byte) (int, error) }, data ForgotPasswordData, t func(key string, args ...any) string) error {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	b := &mi.Builder{}
	locale := data.Locale
	if locale == "" {
		locale = "en"
	}

	page := b.Html(mi.Lang(locale),
		b.Head(
			b.Meta(mi.Charset("utf-8")),
			b.Meta(mi.Name("viewport"), mi.Content("width=device-width, initial-scale=1")),
			b.Title(t("password_reset.forgot_title_prefix")+"Seam AMS"),
			b.Link(mi.Rel("stylesheet"), mi.Href("/static/css/tailwind.css")),
			b.Link(mi.Rel("stylesheet"), mi.Href("/static/vendor/material-icons.css")),
		),
		b.Body(mi.Class("bg-gray-100 dark:bg-gray-900"),
			ForgotPasswordPage(data, t)(b),
		),
	)

	w.Write([]byte("<!DOCTYPE html>\n"))
	return page.Render(w)
}

// RenderResetPasswordPage renders the reset password page as a standalone HTML page.
func RenderResetPasswordPage(w interface{ Write([]byte) (int, error) }, data ResetPasswordData, t func(key string, args ...any) string) error {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	b := &mi.Builder{}
	locale := data.Locale
	if locale == "" {
		locale = "en"
	}

	page := b.Html(mi.Lang(locale),
		b.Head(
			b.Meta(mi.Charset("utf-8")),
			b.Meta(mi.Name("viewport"), mi.Content("width=device-width, initial-scale=1")),
			b.Title(t("password_reset.reset_title_prefix")+"Seam AMS"),
			b.Link(mi.Rel("stylesheet"), mi.Href("/static/css/tailwind.css")),
			b.Link(mi.Rel("stylesheet"), mi.Href("/static/vendor/material-icons.css")),
		),
		b.Body(mi.Class("bg-gray-100 dark:bg-gray-900"),
			ResetPasswordPage(data, t)(b),
		),
	)

	w.Write([]byte("<!DOCTYPE html>\n"))
	return page.Render(w)
}
