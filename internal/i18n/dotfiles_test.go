package i18n

import (
	"os"
	"path/filepath"
	"testing"
)

// Regression (2026-10-12): a writer's temporary file in the directory made a
// concurrent reload fail with "not a valid locale file name", which the panel
// showed as a spurious load error. Dot-files are never locale files.
func TestLoadDir_IgnoresDotFiles(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "en.json"), []byte(`{"greet":"Hi"}`), 0o644)
	_ = os.WriteFile(filepath.Join(dir, ".tmp-123.json"), []byte(`{"greet": half`), 0o644)
	_ = os.WriteFile(filepath.Join(dir, ".hidden.json"), []byte(`not json`), 0o644)
	m, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("dot-files must be ignored, got %v", err)
	}
	if len(m) != 1 || m["en"]["greet"] != "Hi" {
		t.Errorf("only en.json is a locale file: %v", m)
	}
}

func TestChangeDetection_IgnoresDotFiles(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "en.json"), []byte(`{"greet":"Hi"}`), 0o644)
	before, _ := dirSignature(dir)
	_ = os.WriteFile(filepath.Join(dir, ".tmp-1.json"), []byte(`{}`), 0o644)
	after, _ := dirSignature(dir)
	if before != after {
		t.Errorf("a temporary file must not look like a change")
	}
}
