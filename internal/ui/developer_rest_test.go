package ui

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"

	fe "github.com/ha1tch/seam-ui/internal/formengine"
)

// A file that is not even a definition is set aside alone: the rest of the
// directory loads and what was in force for that form stays.
func TestDirectory_AMalformedFileDoesNotHoldBackTheOthers(t *testing.T) {
	h, _ := scopedHarness(t)
	inForce := func(module, name string) string { return string(h.rt.Resolver.Resolve(1, module, name, nil)) }
	h.writeSchema(t, "system", "user_create", e2eOverride)
	if err := h.rt.Schemas.Reload(); err != nil {
		t.Fatal(err)
	}
	h.writeSchema(t, "system", "user_create", `{"type":"object","prop`)                              // half saved
	h.writeSchema(t, "masters", "marca", strings.Replace(e2eOverride, "Alpha OVERRIDE", "Brand", 1)) // good, at the same time
	h.writeSchema(t, "system", "Bad-Name", e2eOverride)                                              // no form can have this name
	if err := h.rt.Schemas.Reload(); err != nil {
		t.Fatalf("files set aside do not fail the load: %v", err)
	}
	st := h.rt.Schemas.Status()
	if len(st.Refused) != 2 || st.LastError != "" {
		t.Fatalf("two files are set aside: %+v", st)
	}
	if !strings.Contains(inForce("system", "user_create"), "Alpha OVERRIDE") || !strings.Contains(inForce("masters", "marca"), "Brand") {
		t.Errorf("the half-saved form keeps its last good definition and the other file loads")
	}
	ov := h.get(t, "/developer/overview").Body.String()
	if !strings.Contains(ov, `id="dev-refused-schemas"`) || !strings.Contains(ov, "system/user_create: ") || !strings.Contains(ov, "Bad-Name") {
		t.Errorf("the Overview names both files")
	}
	if list := h.get(t, "/developer/forms?q=bad").Body.String(); strings.Contains(list, "schema=system%2FBad-Name") || strings.Contains(list, "schema=system/Bad-Name") {
		t.Errorf("a name no form can have is not offered as a form")
	}
	if strings.Contains(ov, "system/user_create: system/user_create") {
		t.Errorf("the file is named once")
	}
	rec := h.get(t, "/developer/forms?schema=system/user_create&scope=file")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `{&#34;type&#34;:&#34;object&#34;,&#34;prop`) {
		t.Errorf("the editor opens the half-saved file as it is, to be mended (%d)", rec.Code)
	}
}

// What the tools learn about a form's page is kept across a restart where
// there is a file to keep it in.
func TestForms_LearntPageIdentitiesOutliveARestart(t *testing.T) {
	h, _ := designerHarness(t)
	developerTrace.reset()
	t.Cleanup(developerTrace.reseed)
	file := filepath.Join(h.schDir, ".history", "pages.json")
	SetDeveloperTraceFile(file)
	base, _ := developerEmbeddedSchema("consumable", "create")
	_ = mi.RenderToString((&fe.FormEngine{}).Render(ctx_, resolveSchema(ctx_, "consumable", "create", base), nil, nil,
		fe.RenderContext{Module: "consumable", Form: "consumable"}))
	raw, err := os.ReadFile(file)
	if err != nil || !strings.Contains(string(raw), `"consumable/create"`) {
		t.Fatalf("the identity is written when it is learnt: %v %s", err, raw)
	}
	// the directory's loaders do not take the record for a definition
	if err := h.rt.Schemas.Reload(); err != nil || len(h.rt.Schemas.Status().Refused) != 0 {
		t.Errorf("the record is not a schema: %v %+v", err, h.rt.Schemas.Status().Refused)
	}
	// a new process
	developerTrace.reset()
	if _, seen := developerTrace.identity(developerSchemaRef{"consumable", "create"}); seen {
		t.Fatal("reset")
	}
	SetDeveloperTraceFile(file)
	id, seen := developerTrace.identity(developerSchemaRef{"consumable", "create"})
	if !seen || id.Module != "consumable" || id.Form != "consumable" {
		t.Errorf("after a restart the identity is known without opening the page: %+v %v", id, seen)
	}
	if ed := h.get(t, "/developer/forms?schema=consumable/create").Body.String(); strings.Contains(ed, "follow the naming convention") {
		t.Errorf("and the editor no longer falls back to the convention")
	}
	// a damaged record is ignored, not fatal
	if err := os.WriteFile(file, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	developerTrace.reset()
	SetDeveloperTraceFile(file)
	if _, seen := developerTrace.identity(developerSchemaRef{"consumable", "create"}); seen {
		t.Errorf("a damaged record teaches nothing")
	}
}

// The try page fills any list it can and names the ones it cannot, instead
// of showing an engine error.
func TestForms_TryPageListsNeverBreakTheForm(t *testing.T) {
	h, _ := designerHarness(t)
	rec := h.get(t, "/developer/forms/try?schema=issues/assign")
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `id="dev-try"`) || strings.Contains(body, "formengine:") {
		t.Fatalf("a form with a list its own page fills is shown without an engine error (%d)", rec.Code)
	}
	if !strings.Contains(body, "entity:users") {
		t.Errorf("a list that came back empty is named")
	}
	s := &developerTrySources{h: h.ui, empty: map[string]bool{}}
	for _, src := range []string{"entity:users; DROP", "other:thing", "entity:Bad-Name", "entity:"} {
		if opts, err := s.Resolve(ctx_, src); err != nil || len(opts) != 0 || !s.empty[src] {
			t.Errorf("%q must resolve to an empty, named list: %v %v", src, opts, err)
		}
	}
}

// The export brings what the tree needs to embed a new form, and says what
// is left to do.
func TestExport_ANewFormComesWithItsPackage(t *testing.T) {
	h, _ := scopedHarness(t)
	for _, f := range [][2]string{{"inspections", "checklist"}, {"inspections", "round"}, {"masters", "supplier"}} {
		if loc := location(h.post(t, "/developer/forms/create", url.Values{"module": {f[0]}, "name": {f[1]}, "scope": {"tenant"}})); !strings.Contains(loc, f[1]) || strings.Contains(loc, "error=") {
			t.Fatalf("create %v: %s", f, loc)
		}
	}
	files := zipFiles(t, h.get(t, "/developer/export").Body.Bytes())
	pkg, ok := files["source/internal/formengine/schemas/inspections/inspections.go"]
	if !ok {
		t.Fatalf("the package of a new module is included")
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), "inspections.go", pkg, parser.AllErrors)
	if err != nil || parsed.Name.Name != "inspections" || !strings.Contains(pkg, "//go:embed *.json") || !strings.Contains(pkg, "func Schema(") || !strings.Contains(pkg, "func Names(") {
		t.Errorf("it is a Go file of that package with the two functions every schema package has: %v", err)
	}
	if _, ok := files["source/internal/formengine/schemas/masters/masters.go"]; ok {
		t.Errorf("a package the tree already has is not replaced")
	}
	readme := files["README.txt"]
	for _, want := range []string{"New forms", "inspections/checklist", "developerSchemaModules", "add supplier.json to the go:embed line",
		`resolveSchema(ctx, "inspections", "round", inspections.Schema("round"))`} {
		if !strings.Contains(readme, want) {
			t.Errorf("the note lacks %q:\n%s", want, readme)
		}
	}
	if n := strings.Count(readme, "its package is included"); n != 2 {
		t.Errorf("each new form of a new module points at the one package file, got %d", n)
	}
}

// The identities the build carries name real forms, and agree with every
// page whose source states its form and its identity as literals.
func TestSeed_PageIdentitiesAgreeWithTheSource(t *testing.T) {
	developerTrace.reseed()
	t.Cleanup(developerTrace.reseed)
	var seed map[string]json.RawMessage
	if err := json.Unmarshal(developerPagesSeed, &seed); err != nil || len(seed) < 25 {
		t.Fatalf("the seed must be a table of identities: %v (%d)", err, len(seed))
	}
	for name := range seed {
		module, form, ok := cutRef(name)
		if _, known := developerEmbeddedSchema(module, form); !ok || !known {
			t.Errorf("%s is not a built-in form", name)
		}
		if ids := developerTrace.identities(developerSchemaRef{module, form}); len(ids) == 0 {
			t.Errorf("%s has no identity at start", name)
		}
	}
	// every built-in form has an identity: none is left to the naming convention
	for _, ref := range developerEmbeddedSchemas() {
		if len(developerTrace.identities(ref)) == 0 {
			t.Errorf("%s is not in the table", ref)
		}
	}
	if ids := developerTrace.identities(developerSchemaRef{"workorders_templates", "create"}); len(ids) != 2 {
		t.Errorf("a form rendered by two pages has both identities: %+v", ids)
	}
	// a function that resolves forms and builds exactly one literal identity
	// renders those forms with it
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	resolve := regexp.MustCompile(`resolveSchema\(\s*ctx,\s*"([a-z_]+)",\s*"([a-z_0-9]+)"`)
	moduleRE, formRE := regexp.MustCompile(`\bModule:\s*"([a-z_-]+)"`), regexp.MustCompile(`\bForm:\s*"([a-z_0-9-]+)"`)
	checked := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") || strings.HasPrefix(f, "developer") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, fn := range strings.Split(string(src), "\nfunc ") {
			refs := resolve.FindAllStringSubmatch(fn, -1)
			mods, forms := moduleRE.FindAllStringSubmatch(fn, -1), formRE.FindAllStringSubmatch(fn, -1)
			if len(refs) == 0 || len(mods) == 0 || len(forms) == 0 {
				continue
			}
			one := true
			for i := range mods {
				one = one && mods[i][1] == mods[0][1]
			}
			for i := range forms {
				one = one && forms[i][1] == forms[0][1]
			}
			if !one {
				continue
			}
			for _, r := range refs {
				name := r[1] + "/" + r[2]
				want := developerRenderIdentity{mods[0][1], forms[0][1]}
				got := developerTrace.identities(developerSchemaRef{r[1], r[2]})
				found := false
				for _, id := range got {
					found = found || id == want
				}
				if !found {
					t.Errorf("%s: %s is rendered as %+v, the seed says %+v (update developer_pages_seed.json)", f, name, want, got)
				}
				checked++
			}
		}
	}
	if checked < 8 {
		t.Errorf("the check found only %d literal sites; it has stopped seeing the pages", checked)
	}
}

// A broken locale file is set aside alone and the Overview says so.
func TestDirectory_ABrokenLocaleFileDoesNotHoldBackTheOthers(t *testing.T) {
	h, _ := scopedHarness(t)
	write := func(locale, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(h.locDir, locale+".json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("en", `{"developer.nav.forms": "Forms, first"}`)
	if err := h.rt.Locales.Reload(); err != nil {
		t.Fatal(err)
	}
	write("en", `{"developer.nav.forms": "Forms, `) // half saved
	write("es", `{"developer.nav.forms": "Formularios, a mano"}`)
	if err := h.rt.Locales.Reload(); err != nil {
		t.Fatalf("a file set aside does not fail the load: %v", err)
	}
	if got := h.tr.T("en", "developer.nav.forms"); got != "Forms, first" {
		t.Errorf("the language keeps its texts, got %q", got)
	}
	if got := h.tr.T("es", "developer.nav.forms"); got != "Formularios, a mano" {
		t.Errorf("the other language loads, got %q", got)
	}
	ov := h.get(t, "/developer/overview").Body.String()
	if !strings.Contains(ov, `id="dev-refused-locales"`) || !strings.Contains(ov, "en.json: ") || strings.Contains(ov, `id="dev-refused-schemas"`) {
		t.Errorf("the Overview names the locale file set aside, on its own layer")
	}
}

func TestTry_RowsSwitchedOffAreLeftOut(t *testing.T) {
	for _, c := range []struct {
		row map[string]any
		off bool
	}{
		{map[string]any{"id": 1, "name": "a"}, false},
		{map[string]any{"enabled": true}, false},
		{map[string]any{"enabled": false}, true},
		{map[string]any{"active": false}, true},
		{map[string]any{"is_active": false, "enabled": true}, true},
		{map[string]any{"enabled": "false"}, false}, // not a stored boolean: offered
	} {
		if got := developerTryRowIsOff(c.row); got != c.off {
			t.Errorf("%v: off=%v, want %v", c.row, got, c.off)
		}
	}
}
