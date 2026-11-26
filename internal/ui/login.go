package ui

import (
	"bytes"
	"net/http"

	mi "github.com/ha1tch/minty"

	appversion "github.com/ha1tch/seam-ui/internal/version"
)

// LoginPageData holds data for the login page.
type LoginPageData struct {
	Error     string
	Email     string
	NextURL   string
	AppName   string
	Version   string
	StaticURL string
	CSRFToken string
	Locale    string
}

// LoginPage renders the login page.
func LoginPage(data LoginPageData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	locale := data.Locale
	if locale == "" {
		locale = "en"
	}
	appName := data.AppName
	if appName == "" {
		appName = "Seam AMS"
	}
	version := data.Version
	if version == "" {
		version = appversion.Version
	}
	staticURL := data.StaticURL
	if staticURL == "" {
		staticURL = "/static"
	}

	return func(b *mi.Builder) mi.Node {
		var hiddenNext mi.Node
		if data.NextURL != "" {
			hiddenNext = b.Input(mi.Type("hidden"), mi.Name("next"), mi.Value(data.NextURL))
		} else {
			hiddenNext = b.Text("")
		}

		var errorBlock mi.Node
		if data.Error != "" {
			errorBlock = b.Div(mi.Class("mb-4 p-3 bg-red-50 border border-red-200 rounded-md"),
				b.Div(mi.Class("flex items-center"),
					b.Span(mi.Class("material-icons text-red-500 mr-2 text-sm"), b.Text("error")),
					b.Span(mi.Class("text-sm text-red-700"), b.Text(data.Error)),
				),
			)
		} else {
			errorBlock = b.Text("")
		}

		return b.Html(mi.Lang(locale),
			b.Head(
				b.Meta(mi.Charset("UTF-8")),
				b.Meta(mi.Name("viewport"), mi.Content("width=device-width, initial-scale=1.0")),
				b.Title(t("login.title_prefix")+appName),
				b.Link(mi.Rel("icon"), mi.Type("image/png"), mi.Attr("sizes", "32x32"),
					mi.Href(staticURL+"/icons/icon-32.png")),
				b.Link(mi.Rel("icon"), mi.Type("image/png"), mi.Attr("sizes", "32x32"),
					mi.Href(staticURL+"/icons/icon-32-dark.png"),
					mi.Attr("media", "(prefers-color-scheme: dark)")),
				b.Link(mi.Rel("shortcut icon"), mi.Href(staticURL+"/favicon.ico")),
				b.Link(mi.Rel("apple-touch-icon"), mi.Attr("sizes", "180x180"),
					mi.Href(staticURL+"/icons/icon-180.png")),
				b.Link(mi.Rel("apple-touch-icon"), mi.Attr("sizes", "180x180"),
					mi.Href(staticURL+"/icons/icon-180-dark.png"),
					mi.Attr("media", "(prefers-color-scheme: dark)")),
				b.Link(mi.Rel("stylesheet"), mi.Href(staticURL+"/css/tailwind.css")),
				b.Link(mi.Rel("stylesheet"), mi.Href(staticURL+"/vendor/material-icons.css")),
			),
			b.Body(mi.Class("bg-gray-100 dark:bg-gray-900 min-h-screen flex items-center justify-center"),
				b.Div(mi.Class("w-full max-w-md px-4"),
					// Logo/Header
					b.Div(mi.Class("text-center mb-8"),
						b.A(mi.Href("/"), mi.Class("inline-block mb-2"),
							b.Img(
								mi.Src(staticURL+"/images/logo-160.png"),
								mi.Alt(appName),
								mi.Class("w-32 h-32 object-contain mx-auto dark:hidden"),
							),
							b.Img(
								mi.Src(staticURL+"/images/logo-160-dark.png"),
								mi.Alt(appName),
								mi.Class("w-32 h-32 object-contain mx-auto hidden dark:block"),
							),
						),
					),

					// Login Card
					b.Div(mi.Class("bg-white rounded-lg shadow-md p-8"),
						b.H2(mi.Class("text-xl font-semibold text-gray-900 mb-6"), b.Text(t("login.heading"))),

						// Error message
						errorBlock,

						// Login form
						b.Form(mi.Method("POST"), mi.Action("/login"),
							// Hidden next URL
							hiddenNext,
							b.Input(mi.Type("hidden"), mi.Name("_csrf"), mi.Value(data.CSRFToken)),

							// Email field
							b.Div(mi.Class("mb-4"),
								b.Label(mi.For("email"), mi.Class("block text-sm font-medium text-gray-700 mb-1"),
									b.Text(t("field.email")),
								),
								b.Input(
									mi.Type("email"),
									mi.ID("email"),
									mi.Name("email"),
									mi.Value(data.Email),
									mi.Required(),
									mi.Autofocus(),
									mi.Class("w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"),
									mi.Placeholder(t("login.email_placeholder")),
								),
							),

							// Password field
							b.Div(mi.Class("mb-6"),
								b.Label(mi.For("password"), mi.Class("block text-sm font-medium text-gray-700 mb-1"),
									b.Text(t("users.field.password")),
								),
								b.Input(
									mi.Type("password"),
									mi.ID("password"),
									mi.Name("password"),
									mi.Required(),
									mi.Class("w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"),
									mi.Placeholder(t("login.password_placeholder")),
								),
								b.Div(mi.Class("mt-1 text-right"),
									b.A(mi.Href("/forgot-password"), mi.Class("text-sm text-blue-600 hover:text-blue-500"),
										b.Text(t("login.forgot_password")),
									),
								),
							),

							// Submit button
							b.Button(
								mi.Type("submit"),
								mi.Class("w-full bg-blue-600 text-white py-2 px-4 rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 font-medium"),
								b.Text(t("login.submit")),
							),
						),
					),

					// Footer
					b.Div(mi.Class("text-center mt-6 text-sm text-gray-500"),
						b.Text(appName+" v"+version),
					),
				),
			),
		)
	}
}

// RenderLoginPage renders the login page to the response writer.
func RenderLoginPage(w http.ResponseWriter, data LoginPageData, t func(key string, args ...any) string) {
	var buf bytes.Buffer
	b := &mi.Builder{}

	buf.WriteString("<!DOCTYPE html>\n")
	if err := LoginPage(data, t)(b).Render(&buf); err != nil {
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}
