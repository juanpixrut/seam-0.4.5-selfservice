package schemaoverlay

import (
	"os"
	"path/filepath"
	"testing"
)

// Regression (2026-10-12): see the i18n twin of this test.
func TestLoadDir_IgnoresDotFilesAndDotDirectories(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "system"), 0o755)
	_ = os.MkdirAll(filepath.Join(dir, ".git"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "system", "user_edit.json"), []byte(goodDoc), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "system", ".tmp-9.json"), []byte(`{half`), 0o644)
	_ = os.WriteFile(filepath.Join(dir, ".git", "HEAD.json"), []byte(`x`), 0o644)
	m, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("dot-files and dot-directories must be ignored, got %v", err)
	}
	if len(m) != 1 {
		t.Errorf("only system/user_edit.json is a schema: %v", m)
	}
}

func TestChangeDetection_IgnoresDotFiles(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "system"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "system", "a.json"), []byte(goodDoc), 0o644)
	before, _ := dirSignature(dir)
	_ = os.WriteFile(filepath.Join(dir, "system", ".tmp-1.json"), []byte(`{}`), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, ".git"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".git", "x.json"), []byte(`{}`), 0o644)
	after, _ := dirSignature(dir)
	if before != after {
		t.Errorf("temporary files must not look like a change")
	}
}
