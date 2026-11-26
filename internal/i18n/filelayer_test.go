package i18n

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var writeCounter int64

// writeLocale writes a file and gives it a fresh, strictly increasing
// modification time, so a change is detectable even when the size and the
// filesystem's timestamp granularity would hide it.
func writeLocale(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	writeCounter++
	mt := time.Now().Add(time.Duration(writeCounter) * time.Hour)
	if err := os.Chtimes(p, mt, mt); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDir(t *testing.T) {
	dir := t.TempDir()
	writeLocale(t, dir, "en.json", `{"greet": "Hey", "bye": "Later"}`)
	writeLocale(t, dir, "pt_br.json", `{"greet": "Oi"}`)
	writeLocale(t, dir, "es.json", `{}`) // contributes nothing
	writeLocale(t, dir, "README.txt", `not json at all`)
	if err := os.Mkdir(filepath.Join(dir, "sub.json"), 0o700); err != nil { // a directory, ignored
		t.Fatal(err)
	}

	got, err := LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["en"]["greet"] != "Hey" || got["en"]["bye"] != "Later" || got["pt_br"]["greet"] != "Oi" {
		t.Errorf("LoadDir = %v", got)
	}
	if _, ok := got["es"]; ok {
		t.Error("an empty file should not create a locale")
	}
}

func TestLoadDirReportsEveryProblemNamingTheFile(t *testing.T) {
	dir := t.TempDir()
	writeLocale(t, dir, "en.json", `{"greet": `)              // truncated
	writeLocale(t, dir, "es.json", `{"greet": 42}`)           // wrong value type
	writeLocale(t, dir, "Bad Name.json", `{"a": "b"}`)        // not a locale name
	writeLocale(t, dir, "pt.json", `["not", "an", "object"]`) // wrong shape

	_, err := LoadDir(dir)
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, name := range []string{"en.json", "es.json", "Bad Name.json", "pt.json"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error does not mention %s:\n%v", name, err)
		}
	}
}

func TestLoadDirMissingDirectory(t *testing.T) {
	if _, err := LoadDir(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("expected an error for a missing directory")
	}
}

func TestFileLayerReloadAppliesAndReportsStatus(t *testing.T) {
	dir := t.TempDir()
	tr := newTr()
	fl := NewFileLayer(tr, dir)
	writeLocale(t, dir, "en.json", `{"greet": "Hey", "gretting": "typo key"}`)
	writeLocale(t, dir, "es.json", `{"greet": "Ey"}`)

	if err := fl.Reload(); err != nil {
		t.Fatal(err)
	}
	if got := tr.T("en", "greet"); got != "Hey" {
		t.Errorf("T = %q", got)
	}
	if got := tr.T("es", "greet"); got != "Ey" {
		t.Errorf("T(es) = %q", got)
	}
	st := fl.Status()
	if st.Dir != dir || st.LoadedAt.IsZero() || st.LastError != "" || st.Keys != 3 {
		t.Errorf("status = %+v", st)
	}
	if len(st.Locales) != 2 || st.Locales[0] != "en" || st.Locales[1] != "es" {
		t.Errorf("Locales = %v", st.Locales)
	}
	if len(st.NewKeys) != 1 || st.NewKeys[0] != "en: gretting" {
		t.Errorf("NewKeys should flag the key absent from the base: %v", st.NewKeys)
	}
}

func TestFileLayerFailedReloadKeepsThePreviousLayer(t *testing.T) {
	dir := t.TempDir()
	tr := newTr()
	fl := NewFileLayer(tr, dir)
	writeLocale(t, dir, "en.json", `{"greet": "Hey"}`)
	if err := fl.Reload(); err != nil {
		t.Fatal(err)
	}
	loaded := fl.Status().LoadedAt

	// A half-saved edit is set aside alone: that language keeps its texts,
	// and a good edit of another language made at the same time still loads.
	writeLocale(t, dir, "en.json", `{"greet": "Hey", `)
	writeLocale(t, dir, "es.json", `{"greet": "Buenas"}`)
	if err := fl.Reload(); err != nil {
		t.Fatalf("a file set aside does not fail the load: %v", err)
	}
	if got := tr.T("en", "greet"); got != "Hey" {
		t.Errorf("a broken file must keep that language's previous texts, got %q", got)
	}
	if got := tr.T("es", "greet"); got != "Buenas" {
		t.Errorf("the broken file held back another language, got %q", got)
	}
	st := fl.Status()
	if len(st.Refused) != 1 || st.Refused[0].File != "en.json" || !st.Refused[0].Kept || st.Refused[0].Reason == "" || st.LastError != "" || st.LoadedAt.Before(loaded) {
		t.Errorf("status after a file is set aside = %+v", st)
	}
	if h, _ := fl.History("en", "greet"); len(h) != 0 {
		t.Errorf("a broken file is not a removal of its texts: %+v", h)
	}

	// Fixing the file recovers and clears the error.
	writeLocale(t, dir, "en.json", `{"greet": "Hello again"}`)
	if err := fl.Reload(); err != nil {
		t.Fatal(err)
	}
	if got := tr.T("en", "greet"); got != "Hello again" || fl.Status().LastError != "" || len(fl.Status().Refused) != 0 {
		t.Errorf("after fix: %q, %+v", got, fl.Status())
	}
}

func TestFileLayerMissingDirectoryKeepsLayer(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "locales")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	tr := newTr()
	fl := NewFileLayer(tr, dir)
	writeLocale(t, dir, "en.json", `{"greet": "Hey"}`)
	if err := fl.Reload(); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := fl.Reload(); err == nil {
		t.Error("expected an error for a vanished directory")
	}
	if got := tr.T("en", "greet"); got != "Hey" {
		t.Errorf("layer lost when the directory vanished: %q", got)
	}
}

func TestFileLayerEmptiedDirectoryClearsLayer(t *testing.T) {
	dir := t.TempDir()
	tr := newTr()
	fl := NewFileLayer(tr, dir)
	writeLocale(t, dir, "en.json", `{"greet": "Hey"}`)
	if err := fl.Reload(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "en.json")); err != nil {
		t.Fatal(err)
	}
	if err := fl.Reload(); err != nil {
		t.Fatal(err)
	}
	if got := tr.T("en", "greet"); got != "Hello" {
		t.Errorf("deleting the file should drop its overrides, got %q", got)
	}
}

func TestFileLayerReloadIfChanged(t *testing.T) {
	dir := t.TempDir()
	tr := newTr()
	fl := NewFileLayer(tr, dir)
	writeLocale(t, dir, "en.json", `{"greet": "A"}`)

	if did, err := fl.ReloadIfChanged(); !did || err != nil {
		t.Fatalf("first call: did=%v err=%v", did, err)
	}
	if did, _ := fl.ReloadIfChanged(); did {
		t.Error("nothing changed, yet a reload was attempted")
	}
	writeLocale(t, dir, "en.json", `{"greet": "B"}`) // same size, new mtime
	if did, err := fl.ReloadIfChanged(); !did || err != nil {
		t.Fatalf("after change: did=%v err=%v", did, err)
	}
	if got := tr.T("en", "greet"); got != "B" {
		t.Errorf("T = %q", got)
	}

	// A broken file is reported once, not on every poll.
	writeLocale(t, dir, "en.json", `{broken`)
	if did, err := fl.ReloadIfChanged(); !did || err != nil || len(fl.Status().Refused) != 1 || tr.T("en", "greet") != "B" {
		t.Fatalf("broken file: did=%v err=%v %+v", did, err, fl.Status())
	}
	if did, _ := fl.ReloadIfChanged(); did {
		t.Error("an unchanged broken file must not be retried on every poll")
	}
	writeLocale(t, dir, "en.json", `{"greet": "C"}`)
	if did, err := fl.ReloadIfChanged(); !did || err != nil {
		t.Fatalf("after fix: did=%v err=%v", did, err)
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestFileLayerPollHonoursTheActiveSwitchAndStops(t *testing.T) {
	dir := t.TempDir()
	tr := newTr()
	fl := NewFileLayer(tr, dir)
	var active atomic.Bool

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		fl.Poll(ctx, 5*time.Millisecond, active.Load)
		close(done)
	}()

	// Switched off: a file appears and nothing happens.
	writeLocale(t, dir, "en.json", `{"greet": "Hot"}`)
	time.Sleep(60 * time.Millisecond)
	if got := tr.T("en", "greet"); got != "Hello" {
		t.Errorf("polling ran while inactive: %q", got)
	}

	// Switched on: the pending change is picked up.
	active.Store(true)
	waitFor(t, "the file to load once active", func() bool { return tr.T("en", "greet") == "Hot" })

	// Edits keep flowing in.
	writeLocale(t, dir, "en.json", `{"greet": "Hotter"}`)
	waitFor(t, "the edit to load", func() bool { return tr.T("en", "greet") == "Hotter" })

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Poll did not stop when its context was cancelled")
	}
}

func TestFileLayerStatusReturnsCopies(t *testing.T) {
	dir := t.TempDir()
	tr := newTr()
	fl := NewFileLayer(tr, dir)
	writeLocale(t, dir, "en.json", `{"greet": "Hey"}`)
	if err := fl.Reload(); err != nil {
		t.Fatal(err)
	}
	st := fl.Status()
	st.Locales[0] = "mutated"
	if fl.Status().Locales[0] != "en" {
		t.Error("Status leaked its internal slice")
	}
}
