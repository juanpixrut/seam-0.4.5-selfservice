package dirwatch

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out: %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestMode(t *testing.T) {
	if Mode("poll") != "poll" {
		t.Errorf("poll is always honoured")
	}
	want := "poll"
	if Supported {
		want = "events"
	}
	for _, s := range []string{"", "auto", "events"} {
		if got := Mode(s); got != want {
			t.Errorf("Mode(%q) = %q, want %q", s, got, want)
		}
	}
}

func TestWatch_ReportsChangesAndIgnoresDotNames(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "system"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	if ok := Watch(ctx, dir, true, func() { calls.Add(1) }); ok != Supported {
		t.Fatalf("Watch = %v, Supported = %v", ok, Supported)
	}
	if !Supported {
		t.Skip("this build has no change events; polling is all there is")
	}
	write := func(rel, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// names starting with a dot never count: the history and temporary files
	if err := os.Mkdir(filepath.Join(dir, ".history"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(".history/forms.jsonl", "x")
	write(".tmp123", "x")
	time.Sleep(4 * Settle)
	if n := calls.Load(); n != 0 {
		t.Fatalf("dot names must not be reported, got %d call(s)", n)
	}
	// a file in the directory, and one in a sub-directory that was already there
	write("en.json", "{}")
	waitFor(t, "a file in the directory", func() bool { return calls.Load() == 1 })
	write("system/profile.json", "{}")
	waitFor(t, "a file in a sub-directory", func() bool { return calls.Load() == 2 })
	// a sub-directory created later is watched too
	if err := os.Mkdir(filepath.Join(dir, "masters"), 0o755); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "a new sub-directory", func() bool { return calls.Load() == 3 })
	write("masters/marca.json", "{}")
	waitFor(t, "a file in the new sub-directory", func() bool { return calls.Load() == 4 })
	// a removal and a rename
	if err := os.Remove(filepath.Join(dir, "en.json")); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "a removal", func() bool { return calls.Load() == 5 })
	// two saves apart by more than the settle time are two calls
	write("system/profile.json", `{"a":1}`)
	waitFor(t, "first save", func() bool { return calls.Load() == 6 })
	write("system/profile.json", `{"a":2}`)
	waitFor(t, "second save", func() bool { return calls.Load() == 7 })
	// after cancel nothing is reported
	cancel()
	time.Sleep(2 * Settle)
	write("es.json", "{}")
	time.Sleep(4 * Settle)
	if n := calls.Load(); n != 7 {
		t.Errorf("after cancel no change is reported, got %d", n)
	}
}
