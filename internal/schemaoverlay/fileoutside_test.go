package schemaoverlay

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ha1tch/seam-ui/internal/dirwatch"
)

func writeSchemaFile(t *testing.T, dir, module, name, doc string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, module), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, module, name+".json"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A definition changed, added or removed in the directory without the editors
// is a version of its own, made outside.
func TestFileHistory_ChangesMadeOutsideAreRecorded(t *testing.T) {
	l, _, dir := historyFixture(t)
	doc := func(title string) string { return `{"type":"object","title":"` + title + `","properties":{}}` }
	// what is there at the first load has nothing to be compared with
	writeSchemaFile(t, dir, "system", "old", doc("old"))
	if err := l.Reload(); err != nil {
		t.Fatal(err)
	}
	if h, _ := l.History("system", "old"); len(h) != 0 {
		t.Fatalf("the first load records nothing: %+v", h)
	}
	// changed by hand: what was there first, then the change
	writeSchemaFile(t, dir, "system", "old", doc("old, by hand"))
	// added by hand
	writeSchemaFile(t, dir, "system", "added", doc("added"))
	if err := l.Reload(); err != nil {
		t.Fatal(err)
	}
	h, _ := l.History("system", "old")
	if len(h) != 2 || h[1].Version != 1 || !h[1].ChangedAt.IsZero() || h[0].Version != 2 || h[0].ChangedBy != ChangedOutside || h[0].ChangedAt.IsZero() {
		t.Fatalf("a hand change is version 2 after what was there: %+v", h)
	}
	if h, _ := l.History("system", "added"); len(h) != 1 || h[0].ChangedBy != ChangedOutside {
		t.Fatalf("a file added by hand is version 1: %+v", h)
	}
	// loading again without a change records nothing
	if err := l.Reload(); err != nil {
		t.Fatal(err)
	}
	if h, _ := l.History("system", "old"); len(h) != 2 {
		t.Errorf("an unchanged file is not recorded again: %+v", h)
	}
	// the editors' own saves are recorded once, by them
	if _, err := l.SaveAs("system", "old", []byte(doc("by the tools")), nil, 7, "note"); err != nil {
		t.Fatal(err)
	}
	if err := l.Reload(); err != nil {
		t.Fatal(err)
	}
	if h, _ := l.History("system", "old"); len(h) != 3 || h[0].ChangedBy != 7 {
		t.Errorf("a save by the tools is one version, theirs: %+v", h)
	}
	// removed by hand
	if err := os.Remove(filepath.Join(dir, "system", "old.json")); err != nil {
		t.Fatal(err)
	}
	if err := l.Reload(); err != nil {
		t.Fatal(err)
	}
	if h, _ := l.History("system", "old"); len(h) != 4 || !h[0].Removed || h[0].ChangedBy != ChangedOutside {
		t.Errorf("a file removed by hand is a removal: %+v", h)
	}
	// a change made while nothing was running is found by a new layer
	writeSchemaFile(t, dir, "system", "added", doc("changed while down"))
	l2 := NewFileLayer(New(objectValidator), dir)
	if err := l2.Reload(); err != nil {
		t.Fatal(err)
	}
	if h, _ := l2.History("system", "added"); len(h) != 2 || h[0].ChangedBy != ChangedOutside {
		t.Errorf("a change made while the server was down is recorded at the next start: %+v", h)
	}
}

// The layer's check sets a file aside without stopping the rest.
func TestFileLayer_CheckSetsAFileAsideAlone(t *testing.T) {
	l, r, dir := historyFixture(t)
	bad := errors.New("cannot be shown")
	l.SetCheck(func(k Key, doc []byte) error {
		if string(doc) == `{"type":"object","title":"bad","properties":{}}` {
			return bad
		}
		return nil
	})
	good := `{"type":"object","title":"good","properties":{}}`
	writeSchemaFile(t, dir, "system", "a", good)
	writeSchemaFile(t, dir, "system", "b", `{"type":"object","title":"bad","properties":{}}`)
	if err := l.Reload(); err != nil {
		t.Fatalf("a file set aside is not a failed load: %v", err)
	}
	st := l.Status()
	if len(st.Refused) != 1 || st.Refused[0].Key != (Key{Module: "system", Name: "b"}) || st.Refused[0].Kept || st.Refused[0].Reason != bad.Error() || st.LastError != "" {
		t.Fatalf("status: %+v", st)
	}
	if string(r.Resolve(0, "system", "a", nil)) != good || r.Resolve(0, "system", "b", nil) != nil {
		t.Errorf("the good file is in force, the other is not")
	}
	// a file that was good keeps its last good definition
	writeSchemaFile(t, dir, "system", "a", `{"type":"object","title":"bad","properties":{}}`)
	if err := l.Reload(); err != nil {
		t.Fatal(err)
	}
	st = l.Status()
	if len(st.Refused) != 2 || !st.Refused[0].Kept || string(r.Resolve(0, "system", "a", nil)) != good {
		t.Errorf("what was in force stays: %+v", st.Refused)
	}
	if h, _ := l.History("system", "a"); len(h) != 0 {
		t.Errorf("a file set aside was never in force and is not a version: %+v", h)
	}
	// mended
	writeSchemaFile(t, dir, "system", "a", good)
	writeSchemaFile(t, dir, "system", "b", good)
	if err := l.Reload(); err != nil {
		t.Fatal(err)
	}
	if st := l.Status(); len(st.Refused) != 0 || len(st.Schemas) != 2 {
		t.Errorf("after mending: %+v", st)
	}
	// a malformed file is set aside alone too, and the check is not asked
	writeSchemaFile(t, dir, "system", "c", `{`)
	writeSchemaFile(t, dir, "system", "a", `{"type":"object","title":"newer","properties":{}}`)
	if err := l.Reload(); err != nil {
		t.Fatalf("a malformed file does not stop the load: %v", err)
	}
	if st := l.Status(); len(st.Refused) != 1 || st.Refused[0].Key != (Key{Module: "system", Name: "c"}) || len(st.Schemas) != 2 {
		t.Errorf("only the malformed file is set aside: %+v", st)
	}
	if string(r.Resolve(0, "system", "a", nil)) != `{"type":"object","title":"newer","properties":{}}` {
		t.Errorf("the other files load")
	}
	// while hot loading is off the directory is still watched for the history
	writeSchemaFile(t, dir, "system", "b", `{"type":"object","title":"seen, not loaded","properties":{}}`)
	l.Observe()
	if h, _ := l.History("system", "b"); len(h) == 0 || h[0].ChangedBy != ChangedOutside {
		t.Errorf("a change seen while nothing is loaded is a version: %+v", h)
	}
	if string(r.Resolve(0, "system", "b", nil)) != good {
		t.Errorf("observing loads nothing")
	}
	n := len(func() []FileVersion { h, _ := l.History("system", "b"); return h }())
	l.Observe()
	if h, _ := l.History("system", "b"); len(h) != n {
		t.Errorf("observing twice records once")
	}
}

// With change events, two saves of a file made in quick succession are two
// versions, although the layer is loaded only once in a long while.
func TestPoll_EventsMakeQuickSavesSeparateVersions(t *testing.T) {
	if dirwatch.Mode("") != "events" {
		t.Skip("this build has no change events")
	}
	l, r, dir := historyFixture(t)
	doc := func(title string) string { return `{"type":"object","title":"` + title + `","properties":{}}` }
	writeSchemaFile(t, dir, "system", "quick", doc("zero"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l.SetHistoryInterval(-1) // no looks between loads: only events can see the saves
	go l.Poll(ctx, time.Hour, func() bool { return true })
	wait := func(what string, cond func() bool) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for !cond() {
			if time.Now().After(deadline) {
				t.Fatalf("timed out: %s", what)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	wait("the first load", func() bool { return string(r.Resolve(0, "system", "quick", nil)) == doc("zero") })
	for i, title := range []string{"one", "two", "three"} {
		writeSchemaFile(t, dir, "system", "quick", doc(title))
		wait("save "+title, func() bool { return string(r.Resolve(0, "system", "quick", nil)) == doc(title) })
		// the baseline, then one version per save
		wait("version of "+title, func() bool { h, _ := l.History("system", "quick"); return len(h) == i+2 })
	}
	h, _ := l.History("system", "quick")
	if len(h) != 4 || h[0].ChangedBy != ChangedOutside || h[3].ChangedAt.IsZero() == false {
		t.Errorf("three saves are three versions after what was there: %+v", h)
	}
}
