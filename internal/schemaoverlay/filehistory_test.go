package schemaoverlay

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func historyFixture(t *testing.T) (*FileLayer, *Resolver, string) {
	t.Helper()
	dir := t.TempDir()
	r := New(objectValidator)
	return NewFileLayer(r, dir), r, dir
}

func TestFileHistory_RecordsEverySaveAndRealRemoval(t *testing.T) {
	l, r, dir := historyFixture(t)
	doc := func(title string) []byte { return []byte(`{"type":"object","title":"` + title + `","properties":{}}`) }
	if h, err := l.History("system", "user"); err != nil || len(h) != 0 {
		t.Fatalf("no history yet: %v %v", h, err)
	}
	if _, err := l.SaveAs("system", "user", doc("one"), nil, 7, "  first\ttry\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Save("system", "user", doc("two"), nil); err != nil {
		t.Fatal(err)
	}
	if err := l.RemoveAs("system", "user", 8); err != nil {
		t.Fatal(err)
	}
	if err := l.Remove("system", "user"); err != nil { // nothing there: no version
		t.Fatal(err)
	}
	h, err := l.History("system", "user")
	if err != nil || len(h) != 3 {
		t.Fatalf("three versions, newest first: %d %v", len(h), err)
	}
	if !h[0].Removed || h[0].Version != 3 || h[0].ChangedBy != 8 || h[0].Doc != "" || !strings.Contains(h[1].Doc, `"two"`) ||
		!strings.Contains(h[2].Doc, `"one"`) || h[2].ChangedBy != 7 || h[2].Note != "firsttry" || h[2].ChangedAt.IsZero() {
		t.Errorf("wrong content: %+v", h)
	}
	if _, err := l.Save("system", "other", doc("x"), nil); err != nil {
		t.Fatal(err)
	}
	if h, _ := l.History("system", "other"); len(h) != 1 || h[0].Version != 1 {
		t.Errorf("another form starts at one: %+v", h)
	}
	// the record is not a definition: the layer still loads, and the hot loader sees no change in it
	if err := l.Reload(); err != nil {
		t.Fatalf("the history must not break the loader: %v", err)
	}
	if changed, err := l.ReloadIfChanged(); err != nil || changed {
		t.Errorf("the history alone is not a change to reload: %v %v", changed, err)
	}
	if got := r.Overridden(1); len(got) != 1 || got[0].Name != "other" {
		t.Errorf("only real definitions are overrides: %v", got)
	}
	if _, err := os.Stat(filepath.Join(dir, historyDirName, "forms.jsonl")); err != nil {
		t.Errorf("the history is kept under the dot-directory: %v", err)
	}
	for _, bad := range [][2]string{{"../x", "user"}, {"system", "a/b"}, {"", "user"}} {
		if _, err := l.History(bad[0], bad[1]); !errors.Is(err, ErrInvalidName) {
			t.Errorf("History(%q, %q): %v", bad[0], bad[1], err)
		}
	}
}

// A definition written by hand before the tools first changed it is the
// version before theirs, so the hand-written one can come back.
func TestFileHistory_WhatWasThereIsRecordedFirst(t *testing.T) {
	l, _, dir := historyFixture(t)
	if err := os.MkdirAll(filepath.Join(dir, "system"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "system", "user.json"), []byte(`{"type":"object","title":"by hand","properties":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := l.SaveAs("system", "user", []byte(`{"type":"object","title":"by the tools","properties":{}}`), nil, 7, ""); err != nil {
		t.Fatal(err)
	}
	h, _ := l.History("system", "user")
	if len(h) != 2 || !strings.Contains(h[1].Doc, "by hand") || h[1].Version != 1 || !h[1].ChangedAt.IsZero() || h[0].Version != 2 {
		t.Errorf("the hand-written definition is version 1, undated: %+v", h)
	}
}

// A refused document writes nothing, not even a version.
func TestFileHistory_ARefusalRecordsNothing(t *testing.T) {
	l, _, _ := historyFixture(t)
	if _, err := l.Save("system", "user", []byte(`[1,2]`), nil); !errors.Is(err, ErrRejected) {
		t.Fatalf("want a refusal, got %v", err)
	}
	if h, _ := l.History("system", "user"); len(h) != 0 {
		t.Errorf("a refusal must not be a version: %+v", h)
	}
}
