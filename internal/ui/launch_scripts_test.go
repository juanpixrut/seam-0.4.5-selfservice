package ui

import (
	"os"
	"strings"
	"testing"
)

// The photo gallery stores its files in xolu's /blob, which xolu leaves off unless started with
// XOLU_BLOB_ENABLED=true; without it every upload fails with "upload failed". The lifecycle of an
// issue is a xolu state machine under /api/v2, which xolu serves only with
// XOLU_API_V2_ENABLED=true; without it reporting an issue answers 500. Everything in this repo
// that starts xolu for users must turn both on.
//
// How xolu is started is written once, in scripts/lib/xolu-env.sh. The launcher, the demo builder
// (when it loads the data, and in the START.sh and START.cmd it writes) take it from there; the
// compose file repeats it. scripts/tests/test_makefile.py holds the whole list; this test holds
// the two settings a module of Seam fails without.
func TestLaunchers_StartXoluWithWhatSeamNeeds(t *testing.T) {
	for _, c := range []struct {
		file  string
		count int // how many times the text must be there
		want  string
		why   string
	}{
		{"../../scripts/lib/xolu-env.sh", 1, "\nXOLU_BLOB_ENABLED=true\n", "xolu would start without /blob"},
		{"../../scripts/lib/xolu-env.sh", 1, "\nXOLU_API_V2_ENABLED=true\n", "xolu would start without /api/v2"},
		{"../../scripts/launch.sh", 1, `xolu_env_export "$XOLU_PORT" "$DATA_DIR/xolu"`, "the launcher must start xolu from the shared list"},
		{"../../scripts/build-demo.sh", 1, `xolu_env_export "$SEED_PORT" "$SEED_TMPDIR/xolu"`, "the data must be loaded into a xolu started from the shared list"},
		{"../../scripts/build-demo.sh", 1, `ENV_CMD="$(xolu_env_pairs `, "START.cmd must be written from the shared list"},
		{"../../scripts/build-demo.sh", 1, `ENV_SH="$(xolu_env_pairs `, "START.sh must be written from the shared list"},
		{"../../docker-compose.yml", 1, `XOLU_BLOB_ENABLED=true`, "xolu would start without /blob"},
		{"../../docker-compose.yml", 1, `XOLU_API_V2_ENABLED=true`, "xolu would start without /api/v2"},
	} {
		b, err := os.ReadFile(c.file)
		if err != nil {
			t.Fatalf("%s: %v", c.file, err)
		}
		if n := strings.Count(string(b), c.want); n != c.count {
			t.Errorf("%s: found %d x %q, want %d: %s", c.file, n, c.want, c.count, c.why)
		}
	}
	// and nothing sets xolu up on the side, where the list would not reach it
	for _, file := range []string{"../../scripts/launch.sh", "../../scripts/build-demo.sh", "../../Makefile"} {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				continue
			}
			if strings.Contains(line, "_ENABLED=") && strings.Contains(line, "OLU_") {
				t.Errorf("%s sets a xolu setting itself: %q; scripts/lib/xolu-env.sh is the place", file, strings.TrimSpace(line))
			}
		}
	}
}
