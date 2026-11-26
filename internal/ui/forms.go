package ui

import (
	"strconv"

	mi "github.com/ha1tch/minty"
)

// =============================================================================
// FORM COMPONENTS
// =============================================================================

// FormField creates a form field with label and input.
func FormField(label, name, inputType string, attrs ...mi.Attribute) mi.H {
	return func(b *mi.Builder) mi.Node {
		inputID := "field-" + name

		baseAttrs := []mi.Attribute{
			mi.Type(inputType),
			mi.Name(name),
			mi.ID(inputID),
			mi.Class("mt-1 block w-full rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"),
		}
		allAttrs := append(baseAttrs, attrs...)

		return b.Div(mi.Class("mb-4"),
			b.Label(mi.For(inputID), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300"),
				label,
			),
			b.Input(allAttrs...),
		)
	}
}

// FormFieldWithHelp creates a form field with help text.
func FormFieldWithHelp(label, name, inputType, help string, attrs ...mi.Attribute) mi.H {
	return func(b *mi.Builder) mi.Node {
		inputID := "field-" + name

		baseAttrs := []mi.Attribute{
			mi.Type(inputType),
			mi.Name(name),
			mi.ID(inputID),
			mi.Class("mt-1 block w-full rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"),
			mi.Attr("aria-describedby", inputID+"-help"),
		}
		allAttrs := append(baseAttrs, attrs...)

		return b.Div(mi.Class("mb-4"),
			b.Label(mi.For(inputID), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300"),
				label,
			),
			b.Input(allAttrs...),
			b.P(mi.ID(inputID+"-help"), mi.Class("mt-1 text-xs text-gray-500 dark:text-gray-400"),
				help,
			),
		)
	}
}

// FormFieldWithError creates a form field with error state.
func FormFieldWithError(label, name, inputType, errorMsg string, attrs ...mi.Attribute) mi.H {
	return func(b *mi.Builder) mi.Node {
		inputID := "field-" + name

		inputClass := "mt-1 block w-full rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2"
		if errorMsg != "" {
			inputClass += " border-red-500 focus:ring-red-500 focus:border-red-500"
		} else {
			inputClass += " border border-gray-300 dark:border-gray-600 focus:ring-indigo-500 focus:border-indigo-500"
		}
		inputClass += " bg-white dark:bg-gray-700"

		baseAttrs := []mi.Attribute{
			mi.Type(inputType),
			mi.Name(name),
			mi.ID(inputID),
			mi.Class(inputClass),
		}
		if errorMsg != "" {
			baseAttrs = append(baseAttrs, mi.Attr("aria-invalid", "true"))
			baseAttrs = append(baseAttrs, mi.Attr("aria-describedby", inputID+"-error"))
		}
		allAttrs := append(baseAttrs, attrs...)

		children := []interface{}{
			mi.Class("mb-4"),
			b.Label(mi.For(inputID), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300"),
				label,
			),
			b.Input(allAttrs...),
		}

		if errorMsg != "" {
			children = append(children,
				b.P(mi.ID(inputID+"-error"), mi.Class("mt-1 text-xs text-red-500"),
					errorMsg,
				),
			)
		}

		return b.Div(children...)
	}
}

// SelectOption represents an option in a select field.
type SelectOption struct {
	Value    string
	Label    string
	Selected bool
	Disabled bool
}

// FormSelect creates a select dropdown field.
func FormSelect(label, name string, options []SelectOption, attrs ...mi.Attribute) mi.H {
	return func(b *mi.Builder) mi.Node {
		inputID := "field-" + name

		optionNodes := make([]interface{}, len(options))
		for i, opt := range options {
			optChildren := []interface{}{mi.Value(opt.Value)}
			if opt.Selected {
				optChildren = append(optChildren, mi.Attr("selected", "selected"))
			}
			if opt.Disabled {
				optChildren = append(optChildren, mi.Attr("disabled", "disabled"))
			}
			optChildren = append(optChildren, opt.Label)
			optionNodes[i] = b.Option(optChildren...)
		}

		selectChildren := []interface{}{
			mi.Name(name),
			mi.ID(inputID),
			mi.Class("mt-1 block w-full rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"),
		}
		for _, attr := range attrs {
			selectChildren = append(selectChildren, attr)
		}
		selectChildren = append(selectChildren, optionNodes...)

		return b.Div(mi.Class("mb-4"),
			b.Label(mi.For(inputID), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300"),
				label,
			),
			b.Select(selectChildren...),
		)
	}
}

// FormTextarea creates a textarea field.
func FormTextarea(label, name string, rows int, attrs ...mi.Attribute) mi.H {
	return func(b *mi.Builder) mi.Node {
		inputID := "field-" + name

		textareaChildren := []interface{}{
			mi.Name(name),
			mi.ID(inputID),
			mi.Attr("rows", strconv.Itoa(rows)),
			mi.Class("mt-1 block w-full rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"),
		}
		for _, attr := range attrs {
			textareaChildren = append(textareaChildren, attr)
		}

		return b.Div(mi.Class("mb-4"),
			b.Label(mi.For(inputID), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300"),
				label,
			),
			b.Textarea(textareaChildren...),
		)
	}
}

// FormCheckbox creates a checkbox field.
func FormCheckbox(label, name string, checked bool, attrs ...mi.Attribute) mi.H {
	return func(b *mi.Builder) mi.Node {
		inputID := "field-" + name

		baseAttrs := []mi.Attribute{
			mi.Type("checkbox"),
			mi.Name(name),
			mi.ID(inputID),
			mi.Class("h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-500"),
		}
		if checked {
			baseAttrs = append(baseAttrs, mi.Checked())
		}
		allAttrs := append(baseAttrs, attrs...)

		return b.Div(mi.Class("mb-4 flex items-center gap-2"),
			b.Input(allAttrs...),
			b.Label(mi.For(inputID), mi.Class("text-sm text-gray-700 dark:text-gray-300"),
				label,
			),
		)
	}
}

// FormRadio creates a radio button group.
func FormRadio(label, name string, options []SelectOption) mi.H {
	return func(b *mi.Builder) mi.Node {
		radioNodes := make([]interface{}, len(options))
		for i, opt := range options {
			inputID := "field-" + name + "-" + opt.Value

			inputAttrs := []mi.Attribute{
				mi.Type("radio"),
				mi.Name(name),
				mi.ID(inputID),
				mi.Value(opt.Value),
				mi.Class("h-4 w-4 border-gray-300 text-indigo-600 focus:ring-indigo-500"),
			}
			if opt.Selected {
				inputAttrs = append(inputAttrs, mi.Checked())
			}

			radioNodes[i] = b.Div(mi.Class("flex items-center gap-2"),
				b.Input(inputAttrs...),
				b.Label(mi.For(inputID), mi.Class("text-sm text-gray-700 dark:text-gray-300"),
					opt.Label,
				),
			)
		}

		return b.Div(mi.Class("mb-4"),
			b.Span(mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"),
				label,
			),
			b.Div(append([]interface{}{mi.Class("space-y-2")}, radioNodes...)...),
		)
	}
}

// =============================================================================
// FORM CONTAINERS
// =============================================================================

// Form creates a form element with CSRF protection.
func Form(action, method string, csrfToken string, content mi.H) mi.H {
	return func(b *mi.Builder) mi.Node {
		return b.Form(
			mi.Action(action),
			mi.Method(method),
			mi.Class("space-y-4"),
			// CSRF token
			b.Input(mi.Type("hidden"), mi.Name("_csrf"), mi.Value(csrfToken)),
			content(b),
		)
	}
}

// HTMXForm creates a form that submits via HTMX.
func HTMXForm(action, method, target string, csrfToken string, content mi.H) mi.H {
	return func(b *mi.Builder) mi.Node {
		return b.Form(
			mi.Class("space-y-4"),
			mi.Attr("hx-"+method, action),
			mi.Attr("hx-target", target),
			mi.Attr("hx-swap", "innerHTML"),
			b.Input(mi.Type("hidden"), mi.Name("_csrf"), mi.Value(csrfToken)),
			content(b),
		)
	}
}

// FormActions creates a form actions row (buttons).
func FormActions(children ...mi.H) mi.H {
	return func(b *mi.Builder) mi.Node {
		nodes := make([]interface{}, len(children)+1)
		nodes[0] = mi.Class("flex items-center justify-end gap-3 pt-4 border-t border-gray-200 dark:border-gray-700")
		for i, child := range children {
			nodes[i+1] = child(b)
		}

		return b.Div(nodes...)
	}
}

// =============================================================================
// SEARCH
// =============================================================================

// SearchInput creates a search input with icon.
func SearchInput(name, placeholder string, value string, attrs ...mi.Attribute) mi.H {
	return func(b *mi.Builder) mi.Node {
		baseAttrs := []mi.Attribute{
			mi.Type("search"),
			mi.Name(name),
			mi.Placeholder(placeholder),
			mi.Value(value),
			mi.Class("w-full pl-10 pr-4 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"),
		}
		allAttrs := append(baseAttrs, attrs...)

		return b.Div(mi.Class("relative"),
			b.Input(allAttrs...),
			b.Span(mi.Class("absolute left-3 top-1/2 -translate-y-1/2 material-icons text-gray-400 text-lg"),
				"search",
			),
		)
	}
}

// FilterBar creates a filter bar with search and filters.
func FilterBar(searchName, searchPlaceholder string, filters mi.H) mi.H {
	return func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("flex items-center gap-4 mb-6"),
			b.Div(mi.Class("flex-1 max-w-md"),
				SearchInput(searchName, searchPlaceholder, "")(b),
			),
			filters(b),
		)
	}
}
