package ui

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/token"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ha1tch/seam-ui/internal/i18n"
	"github.com/ha1tch/seam-ui/internal/overrides"
)

// The export (T-118): what the tenant has overridden, as files, so work done
// with the tools can leave the instance. One zip, two views of the same
// overrides:
//
//	overrides/locales/<locale>.json          only the overridden texts
//	overrides/schemas/<module>/<name>.json   the overriding definitions
//
// in the layout SEAM_LOCALES_DIR and SEAM_SCHEMAS_DIR read, to move them to
// another instance; and
//
//	source/internal/i18n/locales/<locale>.json
//	source/internal/formengine/schemas/<module>/<name>.json
//
// in the layout of the source tree: each built-in locale file with the
// overrides applied (its keys in their original order, new keys at the end),
// and each definition under its package directory, to copy over a checkout.
// Only locale files that differ from the built-in ones are included.
//
// A file override outranks a tenant's (DEC-4), so where both exist the file's
// is the one exported. The runtime switches are ignored: a layer that is
// switched off for comparison is still exported. Nothing is written anywhere;
// the export is audited.

// developerOverrideSet is what the export holds.
type developerOverrideSet struct {
	Texts map[string]map[string]string  // locale -> key -> text
	Forms map[developerSchemaRef][]byte // the overriding definitions
	// Global counts the texts and forms that come from the global scope
	// (they apply to every tenant) and that nothing of the tenant's or of the
	// files replaces.
	GlobalTexts, GlobalForms int
}

// developerOverrides gathers the tenant's overrides from their own sources.
func (h *UIHandler) developerOverrides(ctx context.Context, tenant int64) (developerOverrideSet, error) {
	set := developerOverrideSet{Texts: map[string]map[string]string{}, Forms: map[developerSchemaRef][]byte{}}
	rt := h.config.DevRuntime
	if rt == nil {
		return set, nil
	}
	put := func(locale, key, value string) {
		if set.Texts[locale] == nil {
			set.Texts[locale] = map[string]string{}
		}
		set.Texts[locale][key] = value
	}
	// the global scope first: it is in force for this tenant too, below its own
	// overrides and the files, which replace it here as they do on the pages
	globalText, globalForm := map[[2]string]bool{}, map[developerSchemaRef]bool{}
	if rt.Overrides != nil && h.config.DevSettings.GlobalTenant > 0 {
		rows, err := rt.Overrides.Translations(ctx, overrides.GlobalTenant)
		if err != nil {
			return set, err
		}
		for _, row := range rows {
			put(row.Locale, row.Key, row.Value)
			globalText[[2]string{row.Locale, row.Key}] = true
		}
		forms, err := rt.Overrides.Overrides(ctx, overrides.GlobalTenant)
		if err != nil {
			return set, err
		}
		for _, v := range forms {
			set.Forms[developerSchemaRef{v.Module, v.Name}] = v.Doc
			globalForm[developerSchemaRef{v.Module, v.Name}] = true
		}
	}
	if rt.Overrides != nil && tenant > 0 {
		rows, err := rt.Overrides.Translations(ctx, tenant)
		if err != nil {
			return set, err
		}
		for _, row := range rows {
			put(row.Locale, row.Key, row.Value)
			delete(globalText, [2]string{row.Locale, row.Key})
		}
		forms, err := rt.Overrides.Overrides(ctx, tenant)
		if err != nil {
			return set, err
		}
		for _, v := range forms {
			set.Forms[developerSchemaRef{v.Module, v.Name}] = v.Doc
			delete(globalForm, developerSchemaRef{v.Module, v.Name})
		}
	}
	if rt.Locales != nil && h.translator != nil {
		for _, locale := range h.translator.Locales() {
			m, err := rt.Locales.Overrides(locale)
			if err != nil {
				return set, err
			}
			for k, v := range m {
				put(locale, k, v)
				delete(globalText, [2]string{locale, k})
			}
		}
	}
	if rt.Schemas != nil {
		for _, k := range rt.Schemas.Status().Schemas {
			doc, ok, err := rt.Schemas.Read(k.Module, k.Name)
			if err != nil {
				return set, err
			}
			if ok {
				set.Forms[developerSchemaRef{k.Module, k.Name}] = doc
				delete(globalForm, developerSchemaRef{k.Module, k.Name})
			}
		}
	}
	set.GlobalTexts, set.GlobalForms = len(globalText), len(globalForm)
	return set, nil
}

// exportJSONString encodes s as the locale files do: UTF-8 as it is, and no
// HTML escaping of <, > and &.
func exportJSONString(s string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimRight(buf.String(), "\n")
}

// exportLocaleLines writes keys and their texts as one JSON object, a pair per
// line, in the order given.
func exportLocaleLines(keys []string, text map[string]string) []byte {
	var out bytes.Buffer
	out.WriteString("{\n")
	for i, k := range keys {
		out.WriteString("  " + exportJSONString(k) + ": " + exportJSONString(text[k]))
		if i < len(keys)-1 {
			out.WriteByte(',')
		}
		out.WriteByte('\n')
	}
	out.WriteString("}\n")
	return out.Bytes()
}

// exportSourceLocale returns the built-in file of a locale with the overrides
// applied: every built-in key in its original order, then the new keys,
// sorted. changed is false when the result is the built-in file itself.
func exportSourceLocale(embedded []byte, over map[string]string) (out []byte, changed bool, err error) {
	dec := json.NewDecoder(bytes.NewReader(embedded))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, false, fmt.Errorf("the built-in locale file is not a JSON object")
	}
	var keys []string
	text := map[string]string{}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, false, err
		}
		k, _ := kt.(string)
		var v string
		if err := dec.Decode(&v); err != nil {
			return nil, false, err
		}
		if _, dup := text[k]; !dup {
			keys = append(keys, k)
		}
		text[k] = v
	}
	var added []string
	for k, v := range over {
		old, had := text[k]
		if !had {
			added = append(added, k)
		}
		if !had || old != v {
			changed = true
		}
		text[k] = v
	}
	sort.Strings(added)
	return exportLocaleLines(append(keys, added...), text), changed, nil
}

func exportPretty(doc []byte) []byte {
	var out bytes.Buffer
	if err := json.Indent(&out, bytes.TrimSpace(doc), "", "  "); err != nil {
		return doc
	}
	out.WriteByte('\n')
	return out.Bytes()
}

// developerExportZip builds the archive and says how many texts and forms it holds.
func developerExportZip(set developerOverrideSet, embeddedLocale func(locale string) ([]byte, bool), tenant int64, at time.Time) (data []byte, texts, forms int, err error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(name string, content []byte) error {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: at})
		if err != nil {
			return err
		}
		_, err = w.Write(content)
		return err
	}
	locales := make([]string, 0, len(set.Texts))
	for l := range set.Texts {
		locales = append(locales, l)
	}
	sort.Strings(locales)
	var listing []string
	for _, l := range locales {
		keys := make([]string, 0, len(set.Texts[l]))
		for k := range set.Texts[l] {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		texts += len(keys)
		if err = add("overrides/locales/"+l+".json", exportLocaleLines(keys, set.Texts[l])); err != nil {
			return nil, 0, 0, err
		}
		listing = append(listing, fmt.Sprintf("  locales/%s.json  %d text(s)", l, len(keys)))
		if raw, ok := embeddedLocale(l); ok {
			src, changed, serr := exportSourceLocale(raw, set.Texts[l])
			if serr != nil {
				return nil, 0, 0, serr
			}
			if changed {
				if err = add("source/internal/i18n/locales/"+l+".json", src); err != nil {
					return nil, 0, 0, err
				}
			}
		}
	}
	refs := make([]developerSchemaRef, 0, len(set.Forms))
	for ref := range set.Forms {
		refs = append(refs, ref)
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].String() < refs[j].String() })
	newPackages := map[string]bool{}
	var newForms []string
	for _, ref := range refs {
		forms++
		doc := exportPretty(set.Forms[ref])
		if err = add("overrides/schemas/"+ref.Module+"/"+ref.Name+".json", doc); err != nil {
			return nil, 0, 0, err
		}
		if err = add("source/internal/formengine/schemas/"+ref.Module+"/"+ref.Name+".json", doc); err != nil {
			return nil, 0, 0, err
		}
		kind := "replaces the built-in form"
		if _, builtin := developerEmbeddedSchema(ref.Module, ref.Name); !builtin {
			kind = "a new form: see \"New forms\" below"
			dir := "internal/formengine/schemas/" + ref.Module + "/"
			steps := "  " + ref.String() + "\n"
			switch {
			case developerHasModule(ref.Module):
				steps += "    1. add " + ref.Name + ".json to the go:embed line of the package in " + dir + "\n"
			case token.IsKeyword(ref.Module):
				steps += "    1. " + ref.Module + " cannot be a Go package name: put the file in a package of another name, which embeds it\n"
			default:
				if !newPackages[ref.Module] {
					newPackages[ref.Module] = true
					if err = add("source/"+dir+ref.Module+".go", []byte(exportSchemaPackage(ref.Module))); err != nil {
						return nil, 0, 0, err
					}
				}
				steps += "    1. its package is included (" + dir + ref.Module + ".go, which embeds every .json beside it);\n" +
					"       add the package to developerSchemaModules in internal/ui/developer_catalog.go\n"
			}
			steps += "    2. a page renders it with\n" +
				"       engine.Render(ctx, resolveSchema(ctx, \"" + ref.Module + "\", \"" + ref.Name + "\", " + ref.Module + ".Schema(\"" + ref.Name + "\")), values, errs, rc)\n"
			newForms = append(newForms, steps)
		}
		listing = append(listing, fmt.Sprintf("  schemas/%s/%s.json  %s", ref.Module, ref.Name, kind))
	}
	if len(listing) == 0 {
		listing = []string{"  (nothing is overridden)"}
	}
	global := ""
	if set.GlobalTexts+set.GlobalForms > 0 {
		global = fmt.Sprintf("Included: %d text(s) and %d form(s) of the global scope, which apply to every tenant and which nothing\nof this tenant's replaces.\n", set.GlobalTexts, set.GlobalForms)
	}
	readme := "Seam developer overrides\n" +
		"Tenant " + strconv.FormatInt(tenant, 10) + ", exported " + at.UTC().Format("2006-01-02 15:04") + " UTC\n\n" +
		"overrides/   only what was overridden, in the layout the override directories read:\n" +
		"             copy locales/ into SEAM_LOCALES_DIR and schemas/ into SEAM_SCHEMAS_DIR of another instance.\n" +
		"source/      the same changes in the layout of the source tree: copy it over a checkout.\n" +
		"             A locale file here is the built-in one with the overrides applied (its keys in their\n" +
		"             original order, new keys at the end); only files that differ are included.\n" +
		"             A new key is added to the locales it was written in; add it to the others before a release.\n" +
		"             A new form comes with what the tree needs to embed it; the steps left are under \"New forms\".\n\n" +
		"Where a file override and a tenant override of the same text or form both exist, the file's is exported:\n" +
		"it is the one that applies.\n" + global + "\n" +
		"Contents:\n" + strings.Join(listing, "\n") + "\n"
	if len(newForms) > 0 {
		readme += "\nNew forms (after copying source/ over a checkout):\n" + strings.Join(newForms, "")
	}
	if err = add("README.txt", []byte(readme)); err != nil {
		return nil, 0, 0, err
	}
	if err = zw.Close(); err != nil {
		return nil, 0, 0, err
	}
	return buf.Bytes(), texts, forms, nil
}

// DeveloperExport handles GET /developer/export: the caller's tenant's
// overrides as a zip.
func (h *UIHandler) DeveloperExport(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.developerClaims(w, r)
	if !ok {
		return
	}
	set, err := h.developerOverrides(r.Context(), claims.TenantID)
	if err != nil {
		h.logger.Warn("developer: export failed", "error", err)
		developerRedirect(w, r, "/developer/overview", "error", "export_failed")
		return
	}
	now := time.Now()
	data, texts, forms, err := developerExportZip(set, i18n.EmbeddedLocale, claims.TenantID, now)
	if err != nil {
		h.logger.Warn("developer: export failed", "error", err)
		developerRedirect(w, r, "/developer/overview", "error", "export_failed")
		return
	}
	h.auditLogger.LogDeveloperChange(r.Context(), r, claims.UserID, claims.TenantID, "export", "overrides",
		map[string]any{"texts": texts, "forms": forms})
	name := "seam-overrides-tenant" + strconv.FormatInt(claims.TenantID, 10) + "-" + now.UTC().Format("20060102-1504") + ".zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}
