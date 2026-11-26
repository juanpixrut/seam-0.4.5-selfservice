package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	developerschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/developer"
	"github.com/ha1tch/seam-ui/internal/i18n"
)

func developerSchemaText(t *testing.T, name string) []byte {
	t.Helper()
	return developerschemas.Schema(name)
}

// Source-scanning guards for problems that were found once and must not come
// back. They read the repository's own files, so they fail the moment the
// code drifts.

func productionGoFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no sources in %s", dir)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		out[filepath.Base(f)] = string(b)
	}
	return out
}

// Regression (2026-10-12, DEV-32): a render or parse site that reads an
// embedded schema directly bypasses the Developer module's overrides.
func TestSchemaSites_AllGoThroughResolveSchema(t *testing.T) {
	direct := regexp.MustCompile(`\b(system|master|issue|sensor|workorder|developer|consumable|crew|movement|outsourced|unit|warehouse|action|workordertemplate)schemas\.Schema\("|\basset\.Schema\(\)`)
	exempt := map[string]bool{"demo.go": true, "demo_templates.go": true, "schema_resolve.go": true, "developer_catalog.go": true}
	for name, src := range productionGoFiles(t, ".") {
		if exempt[name] {
			continue
		}
		for i, line := range strings.Split(src, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") || !direct.MatchString(line) {
				continue
			}
			// a builder whose caller wraps the result says so on the line
			if !strings.Contains(line, "resolveSchema(") && !strings.Contains(line, "resolveSchema: applied by the caller") {
				t.Errorf("%s:%d reads an embedded schema without resolveSchema: %s", name, i+1, strings.TrimSpace(line))
			}
		}
	}
}

// Regression (2026-10-12): an idempotent re-run of the migration wrapped the
// calls twice.
func TestSchemaSites_NoDoubleWrap(t *testing.T) {
	for name, src := range productionGoFiles(t, ".") {
		for i, line := range strings.Split(src, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			if strings.Count(line, "resolveSchema(") > 1 {
				t.Errorf("%s:%d wraps a schema more than once: %s", name, i+1, strings.TrimSpace(line))
			}
		}
	}
}

// Every developer route must be guarded by the developer permission: an
// unguarded one would expose the editors.
func TestDeveloperRoutes_AreAllGuarded(t *testing.T) {
	src, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	start := strings.Index(body, "func (r *Router) MountDeveloperRoutes")
	if start < 0 {
		t.Fatal("MountDeveloperRoutes not found")
	}
	end := strings.Index(body[start:], "\n}\n")
	fn := body[start : start+end]
	routes := regexp.MustCompile(`mux\.HandleFunc\("(GET|POST) (/developer[^"]*)", ([^\n]*)\)`).FindAllStringSubmatch(fn, -1)
	if len(routes) < 10 {
		t.Fatalf("expected the developer routes, found %d", len(routes))
	}
	for _, m := range routes {
		if !strings.HasPrefix(m[3], "guard(") {
			t.Errorf("%s %s is not wrapped in the developer guard: %s", m[1], m[2], m[3])
		}
	}
	if !strings.Contains(fn, "auth.PermDeveloperAccess") {
		t.Errorf("the guard must be the developer permission")
	}
}

// Regression: the sidebar's active item is chosen by URL prefix, so every
// left-menu entry must point at a GET route that exists, and no two entries
// may be prefixes of one another.
func TestDeveloperSidebar_PointsAtRoutesThatExist(t *testing.T) {
	// the entries live in developer_sidebar.go (the designer's is conditional on its flag)
	srv, err := os.ReadFile(filepath.Join("..", "server", "developer_sidebar.go"))
	if err != nil {
		t.Fatal(err)
	}
	router, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatal(err)
	}
	items := regexp.MustCompile(`\{Label: "developer\.nav\.[a-z]+", URL: "(/developer[^"]*)"`).FindAllStringSubmatch(string(srv), -1)
	if len(items) < 4 {
		t.Fatalf("expected the developer sidebar items, found %d", len(items))
	}
	for i, a := range items {
		if !strings.Contains(string(router), `mux.HandleFunc("GET `+a[1]+`"`) {
			t.Errorf("sidebar item %s has no GET route", a[1])
		}
		for j, b := range items {
			if i != j && strings.HasPrefix(b[1], a[1]) {
				t.Errorf("sidebar item %s is a prefix of %s: both would look active on the second's page", a[1], b[1])
			}
		}
	}
}

func readLocale(t *testing.T, loc string) map[string]string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "i18n", "locales", loc+".json"))
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]string{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// Regression: a page that asks for a key the locale files lack shows the raw
// key; a translation that drops a placeholder prints "%!d(MISSING)".
func TestDeveloperKeys_CodeAndEveryLocaleAgree(t *testing.T) {
	locales := map[string]map[string]string{}
	for _, l := range []string{"en", "es", "ja", "pt"} {
		locales[l] = readLocale(t, l)
	}
	en := locales["en"]
	need := map[string]bool{}
	lit := regexp.MustCompile(`"(developer\.[A-Za-z0-9_.\-]+)"`)
	for name, src := range productionGoFiles(t, ".") {
		if !strings.HasPrefix(name, "developer") {
			continue
		}
		for _, m := range lit.FindAllStringSubmatch(src, -1) {
			if strings.HasSuffix(m[1], ".") { // a prefix built at run time
				continue
			}
			need[m[1]] = true
		}
	}
	for _, v := range developerMessages {
		need[v] = true
	}
	for _, f := range runtimeFlags(t) {
		need["developer.flag."+f.Key+".title"] = true
		need["developer.flag."+f.Key+".desc"] = true
	}
	for _, layer := range []string{"embedded", "filesystem", "xolu-tenant", "xolu-global"} {
		need["developer.layer."+layer] = true
	}
	for _, form := range []string{"translation_edit", "translation_new", "schema_edit", "form_new"} {
		var doc struct {
			Properties map[string]struct {
				Help string `json:"x-seam-help"`
			} `json:"properties"`
		}
		src := developerSchemaText(t, form)
		if err := json.Unmarshal(src, &doc); err != nil {
			t.Fatal(err)
		}
		// fields the editors hide are never rendered, so they need no title
		hidden := map[string]bool{"translation_edit.locale": true, "translation_edit.key": true, "schema_edit.schema": true}
		for field, p := range doc.Properties {
			if hidden[form+"."+field] {
				continue
			}
			need["developer.forms."+form+".field."+field] = true
			if p.Help != "" {
				need["developer.forms."+form+".field."+field+".help"] = true
			}
		}
	}
	for key := range need {
		for loc, m := range locales {
			v, ok := m[key]
			if !ok || strings.TrimSpace(v) == "" {
				t.Errorf("locale %s lacks %q, which the Developer pages use", loc, key)
				continue
			}
			if loc != "en" && !i18n.SameVerbs(en[key], v) {
				t.Errorf("locale %s changes the placeholders of %q: %q vs %q", loc, key, en[key], v)
			}
		}
	}
	// no locale may carry a developer key the others lack
	for loc, m := range locales {
		for key := range m {
			if strings.HasPrefix(key, "developer.") {
				if _, ok := en[key]; !ok {
					t.Errorf("locale %s has %q, which en lacks", loc, key)
				}
			}
		}
	}
}

// Regression: the layout chrome reaches the translator only through
// chromeTranslate, so a new call site cannot quietly skip the tenant's view.
func TestLayoutChrome_UsesTheTenantAwareTranslator(t *testing.T) {
	for _, name := range []string{"layout.go", "handler.go"} {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") || !strings.Contains(line, "Translator.ForLocale(") {
				continue
			}
			// the one global lookup is chromeTranslate's own fallback; RenderError has no request
			if strings.Contains(line, "return cfg.Translator.ForLocale(cfg.Locale)") || strings.Contains(line, "h.Config.Translator.ForLocale") {
				continue
			}
			t.Errorf("%s:%d reads the global translator directly: %s", name, i+1, strings.TrimSpace(line))
		}
	}
}

// Regression (2026-10-12): the stale "(planned)" lines for tools that exist
// must not come back into any locale.
func TestDeveloperLocales_NoStalePlannedText(t *testing.T) {
	for _, l := range []string{"en", "es", "ja", "pt"} {
		for key := range readLocale(t, l) {
			if strings.HasPrefix(key, "developer.sections.") {
				t.Errorf("locale %s still has %q: those lines announced tools that now exist", l, key)
			}
		}
	}
}
