package ui

import (
	"bytes"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	fe "github.com/ha1tch/seam-ui/internal/formengine"
	"github.com/ha1tch/seam-ui/internal/i18n"
	"github.com/ha1tch/seam-ui/internal/overrides"
	"github.com/ha1tch/seam-ui/internal/schemaoverlay"
)

// What the editors check and explain beyond what the override layers do.
//
// The gate (T-115). The layers accept any definition the engine can parse, and
// the engine can parse definitions it cannot render: a form with no field, a
// field with a widget it does not have, a select whose source the page cannot
// resolve. The engine then draws an error line where the field should be,
// which is right for a page and wrong for a save. developerFormCheck is run by
// every way the tools write a definition (the JSON editor, the designer, a
// preview, a restore, a new form), so such a definition is refused with the
// reason. The schemas directory runs it when it loads (DeveloperFileCheck):
// a file put there by hand that fails it is set aside, alone, and the forms
// list and the Overview say so; what was in force for that form stays. The
// tenant layer is loaded as stored: only the tools write it, and they are
// gated; the forms list still checks what is in force there.
//
// Which sources a form may use. A select's x-seam-source is resolved by the
// SourceResolver of the page that renders the form, and pages differ: the
// asset forms use the application's resolver, some forms have their own, most
// have none. So an override may use the sources its built-in definition
// already uses (its page resolves those), the asset forms may use any source
// of the application's resolver, and so may a custom form, whose page does not
// exist yet and is expected to use that resolver.

// developerResolverModules are the modules whose pages render with the
// application's source resolver (AssetSourceResolver).
var developerResolverModules = map[string]bool{"asset": true}

func developerSourceOK(ref developerSchemaRef, base []byte, custom bool) func(source string) bool {
	allowed := map[string]bool{}
	if !custom {
		for _, s := range fe.SourcesOf(base) {
			allowed[s] = true
		}
	}
	if custom || developerResolverModules[ref.Module] {
		for s := range assetSourceQueries {
			allowed[s] = true
		}
	}
	return func(source string) bool { return allowed[source] }
}

// developerFormCheck reports why the definition could not be rendered where
// the form is used, or nil.
func developerFormCheck(ref developerSchemaRef, base []byte, custom bool, doc []byte) error {
	return fe.CheckRenderable(doc, developerSourceOK(ref, base, custom))
}

// DeveloperFileCheck is the gate as the schemas directory applies it when it
// loads (devruntime.Options.SchemaCheck): a definition found there that the
// engine could not render is set aside, so it never reaches a page, whoever
// wrote it.
func DeveloperFileCheck(module, name string, doc []byte) error {
	base, builtIn := developerEmbeddedSchema(module, name)
	return developerFormCheck(developerSchemaRef{module, name}, base, !builtIn, doc)
}

// developerProblemText is an error of the gate as the page shows it.
func developerProblemText(err error) string {
	return developerProblemString(err.Error())
}

func developerProblemString(s string) string {
	s = strings.TrimPrefix(s, fe.ErrNotRenderable.Error()+": ")
	s = strings.TrimPrefix(s, schemaoverlay.ErrRejected.Error()+": ")
	return developerTruncate(s, 600)
}

// isNotRenderable reports whether err is a refusal of the gate.
func isNotRenderable(err error) bool { return errors.Is(err, fe.ErrNotRenderable) }

// ---- what a definition uses ----

// developerFormUsesMax bounds what one form reports for a search.
const developerFormUsesMax = 6

// developerFormUses says what in a definition matches a search (lower-case):
// the widgets its fields use, their sources and their names, each as
// "widget:<name>", "source:<name>" or "field:<name>", in the definition's
// order and without repeats. The rows of a repeatable field count too. It is
// how the forms list answers "which forms use this widget, or this list".
func developerFormUses(doc []byte, needle string) []string {
	if needle == "" || len(doc) == 0 {
		return nil
	}
	form, err := fe.ParseSchema(doc)
	if err != nil {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	add := func(kind, value string) {
		if value == "" || !strings.Contains(strings.ToLower(value), needle) {
			return
		}
		if s := kind + ":" + value; !seen[s] && len(out) < developerFormUsesMax {
			seen[s] = true
			out = append(out, s)
		}
	}
	var walk func(fields []fe.Field)
	walk = func(fields []fe.Field) {
		for _, f := range fields {
			add("widget", fe.InferWidget(f))
			add("source", f.XSeamSource)
			name := f.Name
			if f.I18nName != "" {
				name = f.I18nName
			}
			add("field", name)
			walk(f.Items)
		}
	}
	walk(form.Fields)
	return out
}

// ---- labels that come from a translation key ----

// formLabelKey is a field whose label a translation key supplies.
type formLabelKey struct{ Field, Key string }

// developerLabelKeys lists the fields of a definition whose label comes from a
// translation key, with the key. The engine prefers a key to the definition's
// title (FORM_ENGINE_SPEC section 4.1), so changing such a field's title in an
// override changes nothing on the page; the editors say so and link to the
// text instead.
//
// The key is "<module>.forms.<form>.field.<name>", where module and form are
// what the page passes to the engine, which is not always the definition's own
// package and file name (system/user_create is rendered as system.forms.user).
// So the pair is found, not assumed: among the pairs that have a key for any of
// the definition's fields, the ones whose module is the definition's (give or
// take a final "s") or the one its x-seam-form declares, the pair that covers
// most fields wins, a pair named like the form breaking a tie. A field without
// such a key but with a general "field.<name>" key is listed with that one.
func (h *UIHandler) developerLabelKeys(tenant int64, ref developerSchemaRef, doc []byte) []formLabelKey {
	if h.translator == nil {
		return nil
	}
	form, err := fe.ParseSchema(doc)
	if err != nil {
		return nil
	}
	texts := h.translator.Effective(tenant, developerRefLocale)
	if ids := developerTrace.identities(ref); len(ids) > 0 {
		// the form's pages are known: the engine's own chain, with each
		// identity it is rendered under (a field can have a key per page)
		var out []formLabelKey
		for _, f := range form.Fields {
			listed := map[string]bool{}
			for _, id := range ids {
				base := id.Module + ".forms." + id.Form + ".field."
				for _, k := range []string{base + f.Name, base + f.Name + ".label", "field." + f.Name} {
					if _, ok := texts[k]; ok {
						if !listed[k] {
							listed[k] = true
							out = append(out, formLabelKey{f.Name, k})
						}
						break
					}
				}
			}
		}
		return out
	}
	fields := map[string]bool{}
	for _, f := range form.Fields {
		fields[f.Name] = true
	}
	modules := map[string]bool{ref.Module: true, ref.Module + "s": true, strings.TrimSuffix(ref.Module, "s"): true}
	if form.Module != "" {
		modules[form.Module] = true
	}
	type pair struct{ module, form string }
	count := map[pair]int{}
	for key := range texts {
		module, rest, ok := strings.Cut(key, ".forms.")
		if !ok || !modules[module] {
			continue
		}
		formName, field, ok := strings.Cut(rest, ".field.")
		if !ok {
			continue
		}
		field = strings.TrimSuffix(field, ".label")
		if fields[field] {
			count[pair{module, formName}]++
		}
	}
	var best pair
	bestScore := 0
	for p, n := range count {
		score := n * 4
		if p.form == ref.Name || (form.Name != "" && p.form == form.Name) {
			score += 2
		}
		if p.module == ref.Module || (form.Module != "" && p.module == form.Module) {
			score++
		}
		if score > bestScore || (score == bestScore && (p.module+p.form) < (best.module+best.form)) {
			best, bestScore = p, score
		}
	}
	var out []formLabelKey
	for _, f := range form.Fields {
		if bestScore > 0 {
			k := best.module + ".forms." + best.form + ".field." + f.Name
			if _, ok := texts[k]; ok {
				out = append(out, formLabelKey{f.Name, k})
				continue
			}
			if _, ok := texts[k+".label"]; ok {
				out = append(out, formLabelKey{f.Name, k + ".label"})
				continue
			}
		}
		if _, ok := texts["field."+f.Name]; ok {
			out = append(out, formLabelKey{f.Name, "field." + f.Name})
		}
	}
	return out
}

// ---- the history of the file scope, in the shape the pages already draw ----

func fileSchemaHistory(l *schemaoverlay.FileLayer, ref developerSchemaRef) []overrides.SchemaVersion {
	if l == nil {
		return nil
	}
	hist, err := l.History(ref.Module, ref.Name)
	if err != nil {
		return nil
	}
	out := make([]overrides.SchemaVersion, 0, len(hist))
	for _, v := range hist {
		out = append(out, overrides.SchemaVersion{Module: v.Module, Name: v.Name, Version: v.Version, Doc: []byte(v.Doc),
			Removed: v.Removed, Note: v.Note, ChangedBy: v.ChangedBy, ChangedAt: v.ChangedAt})
	}
	return out
}

func fileTextHistory(l *i18n.FileLayer, locale, key string) []overrides.TranslationVersion {
	if l == nil {
		return nil
	}
	hist, err := l.History(locale, key)
	if err != nil {
		return nil
	}
	out := make([]overrides.TranslationVersion, 0, len(hist))
	for _, v := range hist {
		out = append(out, overrides.TranslationVersion{Locale: v.Locale, Key: v.Key, Version: v.Version, Value: v.Value,
			Removed: v.Removed, ChangedBy: v.ChangedBy, ChangedAt: v.ChangedAt})
	}
	return out
}

// sameJSONDoc reports whether two documents are the same JSON, whatever their
// spacing: a file saved by the tools is indented, the version kept may not be.
func sameJSONDoc(a, b []byte) bool {
	var x, y bytes.Buffer
	if json.Compact(&x, a) != nil || json.Compact(&y, b) != nil {
		return bytes.Equal(bytes.TrimSpace(a), bytes.TrimSpace(b))
	}
	return bytes.Equal(x.Bytes(), y.Bytes())
}

// ---- the state of one text in every language ----

// Text states, as the last part of their translation key.
const (
	textTranslated   = "translated"
	textUntranslated = "untranslated" // there, but identical to the reference text
	textMissing      = "missing"      // not there: the reference text is shown
)

// translationLocaleState is how one language stands for a key.
type translationLocaleState struct{ Locale, State string }

// translationLocaleStates says, for one key, which languages have their own
// text, which only repeat the reference text and which have none, so a text
// written in one language is not forgotten in the others.
func (h *UIHandler) translationLocaleStates(tenant int64, key string) []translationLocaleState {
	if h.translator == nil {
		return nil
	}
	locales := h.translator.Locales()
	sort.Strings(locales)
	ref, hasRef := h.translator.Effective(tenant, developerRefLocale)[key]
	out := make([]translationLocaleState, 0, len(locales))
	for _, l := range locales {
		text, ok := h.translator.Effective(tenant, l)[key]
		state := textTranslated
		switch {
		case !ok:
			state = textMissing
		case l != developerRefLocale && hasRef && text == ref:
			state = textUntranslated
		}
		out = append(out, translationLocaleState{l, state})
	}
	return out
}
