package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/seam-ui/internal/auth"
)

// TestDemoKitchen_DumpPage writes each page of the Demo module next to the
// file named by SEAM_DEMO_DUMP (build/demo.html gives build/demo-inputs.html,
// build/demo-pickers.html and build/demo-rows.html), for
// scripts/browser-test-demo.js. A dormant guard: skipped unless the variable
// is set.
func TestDemoKitchen_DumpPage(t *testing.T) {
	out := os.Getenv("SEAM_DEMO_DUMP")
	if out == "" {
		t.Skip("SEAM_DEMO_DUMP not set")
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = kitchenPage(t) // the static source resolver the browser test expects, until the test ends
	ext := filepath.Ext(out)
	for _, g := range demoGroups {
		name := strings.TrimSuffix(out, ext) + "-" + g.ID + ext
		if err := os.WriteFile(name, []byte(demoGroupBody(t, g.ID, auth.RoleViewer)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
