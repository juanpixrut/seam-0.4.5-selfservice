package i18n

import (
	"os"
	"path/filepath"
	"testing"
)

func writeLocaleFile(t *testing.T, dir, locale, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, locale+".json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A text changed, added or removed in the directory without the editors is a
// version of its own, made outside.
func TestFileHistory_ChangesMadeOutsideAreRecorded(t *testing.T) {
	l, tr, dir := writerFixture(t)
	writeLocaleFile(t, dir, "en", `{"bye": "Goodbye"}`)
	if err := l.Reload(); err != nil {
		t.Fatal(err)
	}
	if h, _ := l.History("en", "bye"); len(h) != 0 {
		t.Fatalf("the first load records nothing: %+v", h)
	}
	writeLocaleFile(t, dir, "en", `{"bye": "See you", "greet": "Hi %s"}`)
	if err := l.Reload(); err != nil {
		t.Fatal(err)
	}
	h, _ := l.History("en", "bye")
	if len(h) != 2 || h[1].Value != "Goodbye" || !h[1].ChangedAt.IsZero() || h[0].Value != "See you" || h[0].ChangedBy != ChangedOutside {
		t.Fatalf("a hand change is version 2 after what was there: %+v", h)
	}
	if h, _ := l.History("en", "greet"); len(h) != 1 || h[0].ChangedBy != ChangedOutside || h[0].Value != "Hi %s" {
		t.Fatalf("a text added by hand is version 1: %+v", h)
	}
	if err := l.Reload(); err != nil {
		t.Fatal(err)
	}
	if h, _ := l.History("en", "bye"); len(h) != 2 {
		t.Errorf("an unchanged text is not recorded again: %+v", h)
	}
	// the editors' own saves are recorded once, by them
	if err := l.SetOverrideAs("en", "bye", "Ciao", "Bye", 7); err != nil {
		t.Fatal(err)
	}
	if err := l.Reload(); err != nil {
		t.Fatal(err)
	}
	if h, _ := l.History("en", "bye"); len(h) != 3 || h[0].ChangedBy != 7 {
		t.Errorf("a save by the tools is one version, theirs: %+v", h)
	}
	// removed by hand, with the file
	if err := os.Remove(filepath.Join(dir, "en.json")); err != nil {
		t.Fatal(err)
	}
	if err := l.Reload(); err != nil {
		t.Fatal(err)
	}
	if h, _ := l.History("en", "bye"); len(h) != 4 || !h[0].Removed || h[0].ChangedBy != ChangedOutside {
		t.Errorf("a text removed by hand is a removal: %+v", h)
	}
	if got := tr.T("en", "bye"); got != "Bye" {
		t.Errorf("got %q", got)
	}
	// a change made while nothing was running is found by a new layer
	writeLocaleFile(t, dir, "en", `{"greet": "Hey %s"}`)
	l2 := NewFileLayer(tr, dir)
	if err := l2.Reload(); err != nil {
		t.Fatal(err)
	}
	if h, _ := l2.History("en", "greet"); len(h) < 2 || h[0].ChangedBy != ChangedOutside || h[0].Value != "Hey %s" {
		t.Errorf("a change made while the server was down is recorded at the next start: %+v", h)
	}
	// while hot loading is off the directory is still watched for the history
	writeLocaleFile(t, dir, "en", `{"greet": "Yo %s"}`)
	l2.Observe()
	h, _ = l2.History("en", "greet")
	if h[0].Value != "Yo %s" || h[0].ChangedBy != ChangedOutside {
		t.Errorf("a change seen while nothing is loaded is a version: %+v", h[0])
	}
	if got := tr.T("en", "greet", "Ann"); got != "Hey Ann" {
		t.Errorf("observing loads nothing, got %q", got)
	}
	l2.Observe()
	if h2, _ := l2.History("en", "greet"); len(h2) != len(h) {
		t.Errorf("observing twice records once")
	}
}
