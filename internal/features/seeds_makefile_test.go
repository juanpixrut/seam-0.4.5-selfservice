package features

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Guards for the development entry points that sit outside Go: the Makefile's
// `make dev` and the demo seeds' developer login (campaign DEV-23). They read the
// files as they are in the repository, like the other source-reading guards.

func readRepoFile(t *testing.T, rel ...string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(append([]string{"..", ".."}, rel...)...))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// the recipe of a Makefile target: its lines up to the first blank one
func makeRecipe(t *testing.T, mk, target string) string {
	t.Helper()
	i := strings.Index(mk, "\n"+target+":")
	if i < 0 {
		t.Fatalf("the Makefile has no %s target", target)
	}
	rest := mk[i+1:]
	if j := strings.Index(rest, "\n\n"); j >= 0 {
		rest = rest[:j]
	}
	return rest
}

// make dev may only set variables something reads: a flag's environment variable
// (internal/features/catalog.go) or one the configuration loader reads.
func TestMakefileDev_UsesOnlyEnvironmentVariablesTheCodeReads(t *testing.T) {
	known := map[string]bool{}
	for _, m := range regexp.MustCompile(`EnvVar:\s*"(SEAM_[A-Z0-9_]+)"`).FindAllStringSubmatch(readRepoFile(t, "internal", "features", "catalog.go"), -1) {
		known[m[1]] = true
	}
	for _, m := range regexp.MustCompile(`Getenv\("(SEAM_[A-Z0-9_]+)"\)`).FindAllStringSubmatch(readRepoFile(t, "internal", "config", "config.go"), -1) {
		known[m[1]] = true
	}
	if len(known) < 20 {
		t.Fatalf("found only %d known variables: the test has lost its footing", len(known))
	}
	mk := readRepoFile(t, "Makefile")
	recipe := makeRecipe(t, mk, "dev")
	used := regexp.MustCompile(`(SEAM_[A-Z0-9_]+)=`).FindAllStringSubmatch(recipe, -1)
	if len(used) < 4 {
		t.Fatalf("make dev sets %d variables, expected at least the Developer and Demo flags and the two directories", len(used))
	}
	for _, m := range used {
		if !known[m[1]] {
			t.Errorf("make dev sets %s, which nothing in the code reads", m[1])
		}
	}
	for _, want := range []string{"SEAM_ENABLE_DEVELOPER", "SEAM_ENABLE_DEMO", "SEAM_LOCALES_DIR", "SEAM_SCHEMAS_DIR"} {
		if !strings.Contains(recipe, want+"=") {
			t.Errorf("make dev must set %s", want)
		}
	}
	if !regexp.MustCompile(`(?m)^\.PHONY:[^\n]*(\\\n[^\n]*)*\bdev\b`).MatchString(mk) {
		t.Errorf("dev must be listed in .PHONY, or a file called dev would stop it running")
	}
	if !strings.Contains(recipe, "./scripts/launch.sh") {
		t.Errorf("make dev starts everything through scripts/launch.sh")
	}
}

// Every demo seed creates one developer account, in tenant 1, and lists its login
// in the header (which scripts/build-demo.sh reads); the catalogue lists it too.
// The seeds are .seed files (docs/SEED_FORMAT.md): a record's data is written one
// field per line, the logins of the header one login per line.
func TestSeeds_EachCreatesOneDeveloperAccountAndTheCatalogueListsIt(t *testing.T) {
	all, _ := filepath.Glob(filepath.Join("..", "..", "seeds", "seed-0*.seed"))
	var files []string
	for _, f := range all {
		if !strings.Contains(f, ".schemas.") {
			files = append(files, f)
		}
	}
	if len(files) != 7 {
		t.Fatalf("expected the seven demo seeds, found %d", len(files))
	}
	account := regexp.MustCompile(`(?m)^  "role": "developer",?$`)
	login := regexp.MustCompile(`(?m)^\s+\{"email": "dev@ual\.li", "password": "[^"]+", "role": "developer"\},?$`)
	user := regexp.MustCompile(`(?m)^\{"kind": "entity", "entity": "users", "id": \d+, "data": \{$`)
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		src := string(b)
		at := account.FindAllStringIndex(src, -1)
		if len(at) != 1 {
			t.Errorf("%s creates %d developer accounts, want exactly 1", filepath.Base(f), len(at))
			continue
		}
		if n := len(login.FindAllString(src, -1)); n != 1 {
			t.Errorf("%s lists the developer login %d times in its header, want 1", filepath.Base(f), n)
		}
		// the record the role line is in: from the last users record that starts before it, to its end
		starts := user.FindAllStringIndex(src[:at[0][0]], -1)
		if len(starts) == 0 {
			t.Errorf("%s: the developer role is not in a users record", filepath.Base(f))
			continue
		}
		rec := src[starts[len(starts)-1][0]:]
		if end := strings.Index(rec, "\n{"); end >= 0 {
			rec = rec[:end]
		}
		if !strings.Contains(rec, `"email": "dev@ual.li"`) {
			t.Errorf("%s: the developer account is not dev@ual.li", filepath.Base(f))
		}
		if !regexp.MustCompile(`(?m)^  "tenant_id": 1,?$`).MatchString(rec) {
			t.Errorf("%s: the developer account is not in tenant 1", filepath.Base(f))
		}
	}
	cat := readRepoFile(t, "seeds", "SEEDS-CATALOGO.md")
	if !strings.Contains(cat, "dev@ual.li") || !strings.Contains(cat, "| developer |") {
		t.Errorf("the seeds catalogue must list the developer login")
	}
}
