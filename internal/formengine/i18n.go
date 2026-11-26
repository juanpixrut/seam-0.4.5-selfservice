package formengine

import "strings"

// This file implements I18N_SPEC.md §4.1-§4.2: resolving a field's
// title/help, and a tab/section's label, through the i18n Translator
// rather than using the schema's literal text directly. Every
// resolution path degrades gracefully to the schema's own literal
// value when Translator is nil, when module/form can't be determined,
// or when no key in the chain resolves — rendering without any of this
// wired up (the state of every stage before T-13) still works exactly
// as before; nothing here can make a form render worse than it already
// did.

// resolveModuleForm determines {módulo}/{nombre_formulario} for one
// render call, per §4.1's two-level precedence: RenderContext's own
// Module/Form (already present in the engine since stage 1, unwired
// until now) override the schema's x-seam-form.module/name default
// when either is non-empty, independently of the other axis. Both
// empty means key resolution can't build a form-scoped key at all —
// callers of resolveTitle/resolveHelp handle that by falling through
// to the global field.* namespace or the literal text, not by erroring.
func resolveModuleForm(form *Form, rc RenderContext) (module, name string) {
	module = form.Module
	if rc.Module != "" {
		module = rc.Module
	}
	name = form.Name
	if rc.Form != "" {
		name = rc.Form
	}
	return module, name
}

// tryKeys returns the first key in order whose translation resolves to
// something other than the key itself. internal/i18n.Translator.T
// returns the key verbatim as its own last-resort fallback (confirmed
// by reading its source directly, not assumed) — there's no separate
// "does this key exist" method, so "result != key" is the only
// available not-found signal. Safe in practice for this package's key
// shapes specifically: a dotted path like "assets.forms.asset.field.name"
// would never legitimately translate to itself.
func tryKeys(t Translator, locale string, keys ...string) (string, bool) {
	if t == nil {
		return "", false
	}
	for _, key := range keys {
		if key == "" {
			continue
		}
		result := t.T(locale, key)
		if result != key {
			return result, true
		}
	}
	return "", false
}

// resolveTitle implements §4.1's 4-step chain for a field's title.
// rc.Module/rc.Form are expected to already carry the fully-resolved
// values (Render sets them once via resolveModuleForm right after
// parsing the schema) -- this function doesn't re-resolve precedence
// itself, it just builds keys from whatever rc already has.
func resolveTitle(e *FormEngine, rc RenderContext, fieldName, literalTitle string) string {
	var keys []string
	if rc.Module != "" && rc.Form != "" {
		base := rc.Module + ".forms." + rc.Form + ".field." + fieldName
		keys = append(keys, base, base+".label")
	}
	keys = append(keys, "field."+fieldName)
	if resolved, ok := tryKeys(e.Translator, rc.Locale, keys...); ok {
		return resolved
	}
	return literalTitle
}

// resolveHelp implements §4.2's 3-step chain for a field's x-seam-help.
func resolveHelp(e *FormEngine, rc RenderContext, fieldName, literalHelp string) string {
	var keys []string
	if rc.Module != "" && rc.Form != "" {
		keys = append(keys, rc.Module+".forms."+rc.Form+".field."+fieldName+".help")
	}
	keys = append(keys, "field."+fieldName+".help")
	if resolved, ok := tryKeys(e.Translator, rc.Locale, keys...); ok {
		return resolved
	}
	return literalHelp
}

// resolvePlaceholder is a field's x-seam-placeholder in the viewer's language:
// the key <module>.forms.<form>.field.<name>.placeholder, then
// field.<name>.placeholder, then the text written in the schema.
func resolvePlaceholder(e *FormEngine, rc RenderContext, fieldName, literal string) string {
	var keys []string
	if rc.Module != "" && rc.Form != "" {
		keys = append(keys, rc.Module+".forms."+rc.Form+".field."+fieldName+".placeholder")
	}
	keys = append(keys, "field."+fieldName+".placeholder")
	if resolved, ok := tryKeys(e.Translator, rc.Locale, keys...); ok {
		return resolved
	}
	return literal
}

// resolveLabel resolves a tab/section label. Unlike field titles (where
// "title" is schema literal text OR an implicit key derived from the
// field name), §3.4's convention has the schema author write the full,
// explicit key directly as Label (e.g. "assets.forms.asset.tab.principal")
// — so there's no chain to build, just one lookup with the same
// graceful literal-text fallback if it doesn't resolve (e.g. during
// local development before a locale package defines that key yet).
func resolveLabel(e *FormEngine, rc RenderContext, label string) string {
	if resolved, ok := tryKeys(e.Translator, rc.Locale, label); ok {
		return resolved
	}
	return label
}

// resolveEnumOptionLabel resolves a select/enum field's per-value
// display label (T-23, docs/RESOLVED.md), keyed
// "<form>.<field>.<value, lowercased>" -- e.g. "asset.criticality.high"
// for rc.Form="asset", fieldName="criticality", value="High". Matches
// the existing asset.status.*/asset.connectivity.* convention exactly,
// since rc.Form resolves to "asset" for the Asset form (Render sets
// rc.Module/rc.Form from the schema's own x-seam-form before any widget
// renders -- see resolveModuleForm). The stored/submitted value itself
// is never affected by this function; it only resolves the label shown
// next to it. Falls back to the literal value -- the same string used
// as both value and label before this existed -- when Translator is
// nil, rc.Form is empty, or no key resolves, so every enum field
// without value-level i18n keys yet (most of them, today) renders
// exactly as it always did.
// i18nFieldName returns the field name to use for i18n key resolution:
// f.I18nName when set (a repeatable row item, whose Name has been
// rewritten to the array-bracket submission form and is unusable for
// key-building), otherwise f.Name unchanged -- the same value every
// non-repeatable field has always used.
func i18nFieldName(f Field) string {
	if f.I18nName != "" {
		return f.I18nName
	}
	return f.Name
}

func resolveEnumOptionLabel(e *FormEngine, rc RenderContext, fieldName, value string) string {
	if rc.Form == "" || value == "" {
		return value
	}
	key := rc.Form + "." + fieldName + "." + strings.ToLower(value)
	if resolved, ok := tryKeys(e.Translator, rc.Locale, key); ok {
		return resolved
	}
	return value
}

// resolveOptionGroupLabel translates the <optgroup> category label for
// an XSeamOptionGroups field, mirroring resolveEnumOptionLabel's own
// key convention exactly (same rc.Form/fieldName scoping) with a
// ".group." segment to keep group-label keys distinct from the
// option-value keys under the same field. category is already the
// bare prefix (e.g. "distance"), not a full enum value. T-36,
// 2026-08-02.
func resolveOptionGroupLabel(e *FormEngine, rc RenderContext, fieldName, category string) string {
	if rc.Form == "" || category == "" {
		return category
	}
	key := rc.Form + "." + fieldName + ".group." + strings.ToLower(category)
	if resolved, ok := tryKeys(e.Translator, rc.Locale, key); ok {
		return resolved
	}
	return category
}

// resolveSelectPlaceholder resolves the two generic empty-option
// placeholder strings widgets.go's renderSelectWithExtraAttrs uses
// (T-22, docs/RESOLVED.md) -- "field.select.empty" for a plain enum
// select, "field.select.empty_fk" for an x-seam-source (foreign-key)
// select. Global, not per-form -- the placeholder text itself doesn't
// vary by field or entity, only by locale, so one key pair covers every
// select in every form. Same graceful fallback to the literal default
// as every other resolver in this file.
func resolveSelectPlaceholder(e *FormEngine, rc RenderContext, key, literal string) string {
	if resolved, ok := tryKeys(e.Translator, rc.Locale, key); ok {
		return resolved
	}
	return literal
}

// resolveDocumentLinkItemLabel resolves a document-links row's per-field
// placeholder label -- name/url/type -- via "document_link_item.<itemName>".
// One shared key set, not per-outer-field, since document_links/
// lifecycle_links/loan_links all declare the identical {name, url, type}
// item shape (confirmed by reading asset.json directly, not assumed) --
// per-field keys would just duplicate the same three translations three
// times over for no benefit. Same graceful literal-fallback as every
// other resolver in this file: an item type with no matching key (a
// future schema author reusing document-links with different item
// names) falls back to that item's own literal schema title, same as
// before this existed.
func resolveDocumentLinkItemLabel(e *FormEngine, rc RenderContext, itemName, literalTitle string) string {
	key := "document_link_item." + itemName
	if resolved, ok := tryKeys(e.Translator, rc.Locale, key); ok {
		return resolved
	}
	return literalTitle
}

// resolveRowItemLabel is the text that names one cell of a repeatable row (its
// placeholder and accessible name). Order: the item's x-seam-placeholder; the
// i18n key "repeatable_item.<parent>.<item>"; the key "repeatable_item.<item>";
// the item's schema title; the item's name. parent is the repeatable's own
// field name (the original name, not the bracketed submission name).
func resolveRowItemLabel(e *FormEngine, rc RenderContext, parent string, item Field) string {
	// a placeholder written in the schema has its translation under the
	// cell's own key with ".placeholder"
	if item.XSeamPlaceholder != "" {
		if resolved, ok := tryKeys(e.Translator, rc.Locale, "repeatable_item."+parent+"."+item.Name+".placeholder"); ok {
			return resolved
		}
		return item.XSeamPlaceholder
	}
	return resolveRowItemTitle(e, rc, parent, item)
}

// resolveRowItemTitle is resolveRowItemLabel without x-seam-placeholder: the
// visible label of a cell in the "form" layout (a placeholder is an example,
// not a name).
func resolveRowItemTitle(e *FormEngine, rc RenderContext, parent string, item Field) string {
	for _, key := range []string{"repeatable_item." + parent + "." + item.Name, "repeatable_item." + item.Name} {
		if resolved, ok := tryKeys(e.Translator, rc.Locale, key); ok {
			return resolved
		}
	}
	if item.Title != "" {
		return item.Title
	}
	return item.Name
}

// resolveWidgetChrome resolves a single global, non-field-specific
// widget-UI string -- an "Add row" button's label, the asset-picker's
// MVP stopgap note, or anything else that's part of a widget's own
// surrounding chrome rather than schema-driven content. Structurally
// identical to resolveSelectPlaceholder (global key, literal fallback)
// but named for its broader use -- resolveSelectPlaceholder's own name
// and two hardcoded keys stay select-specific, not repurposed here.
func resolveWidgetChrome(e *FormEngine, rc RenderContext, key, literal string) string {
	if resolved, ok := tryKeys(e.Translator, rc.Locale, key); ok {
		return resolved
	}
	return literal
}
