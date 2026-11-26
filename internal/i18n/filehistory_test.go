package i18n

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileHistory_RecordsEverySaveAndRealRemoval(t *testing.T) {
	l, tr, dir := writerFixture(t)
	if h, err := l.History("en", "greet"); err != nil || len(h) != 0 {
		t.Fatalf("no history yet: %v %v", h, err)
	}
	if err := l.SetOverrideAs("en", "greet", "Hi %s", "Hello %s", 7); err != nil {
		t.Fatal(err)
	}
	if err := l.SetOverride("en", "greet", "Hey %s", "Hello %s"); err != nil {
		t.Fatal(err)
	}
	if err := l.RemoveOverrideAs("en", "greet", 8); err != nil {
		t.Fatal(err)
	}
	if err := l.RemoveOverride("en", "greet"); err != nil { // nothing there: no version
		t.Fatal(err)
	}
	h, err := l.History("en", "greet")
	if err != nil || len(h) != 3 {
		t.Fatalf("three versions, newest first: %+v %v", h, err)
	}
	if !h[0].Removed || h[0].Version != 3 || h[0].ChangedBy != 8 || h[1].Value != "Hey %s" || h[2].Value != "Hi %s" || h[2].ChangedBy != 7 || h[2].ChangedAt.IsZero() {
		t.Errorf("wrong content: %+v", h)
	}
	// other keys and locales keep their own numbering
	if err := l.SetOverride("es", "greet", "Buenas %s", "Hola %s"); err != nil {
		t.Fatal(err)
	}
	if h, _ := l.History("es", "greet"); len(h) != 1 || h[0].Version != 1 {
		t.Errorf("another locale starts at one: %+v", h)
	}
	// the record is not a locale file: the layer still loads, and the hot loader sees no change in it
	if err := l.Reload(); err != nil {
		t.Fatalf("the history must not break the loader: %v", err)
	}
	if changed, err := l.ReloadIfChanged(); err != nil || changed {
		t.Errorf("the history alone is not a change to reload: %v %v", changed, err)
	}
	if got := tr.T("es", "greet", "Ana"); got != "Buenas Ana" {
		t.Errorf("got %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, historyDirName, "texts.jsonl")); err != nil {
		t.Errorf("the history is kept under the dot-directory: %v", err)
	}
	for _, bad := range [][2]string{{"../en", "greet"}, {"en", "a b"}, {"", "greet"}} {
		if _, err := l.History(bad[0], bad[1]); !errors.Is(err, ErrInvalid) {
			t.Errorf("History(%q, %q): %v", bad[0], bad[1], err)
		}
	}
}

// A text written by hand before the tools first changed it is the version
// before theirs, so the hand-written wording can come back.
func TestFileHistory_WhatWasThereIsRecordedFirst(t *testing.T) {
	l, _, dir := writerFixture(t)
	if err := os.WriteFile(filepath.Join(dir, "en.json"), []byte(`{"greet": "Yo %s"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := l.Reload(); err != nil {
		t.Fatal(err)
	}
	if err := l.SetOverrideAs("en", "greet", "Hi %s", "Hello %s", 7); err != nil {
		t.Fatal(err)
	}
	h, _ := l.History("en", "greet")
	if len(h) != 2 || h[1].Value != "Yo %s" || h[1].Version != 1 || !h[1].ChangedAt.IsZero() || h[0].Value != "Hi %s" || h[0].Version != 2 {
		t.Errorf("the hand-written text is version 1, undated: %+v", h)
	}
}

// A damaged line is skipped: the history is a record, never a source.
func TestFileHistory_SurvivesADamagedLine(t *testing.T) {
	l, _, dir := writerFixture(t)
	if err := l.SetOverride("en", "greet", "Hi %s", "Hello %s"); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, historyDirName, "texts.jsonl")
	old, _ := os.ReadFile(p)
	if err := os.WriteFile(p, append([]byte("{not json\n"+strings.Repeat("x", 300)+"\n"), old...), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := l.SetOverride("en", "greet", "Hey %s", "Hello %s"); err != nil {
		t.Fatal(err)
	}
	if h, err := l.History("en", "greet"); err != nil || len(h) != 2 || h[0].Version != 2 {
		t.Errorf("the good lines are still read: %+v %v", h, err)
	}
}
