package schemaoverlay

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	baseSchema = `{"type":"object","properties":{"name":{"type":"string"},"age":{"type":"integer"}},"required":["name"]}`
	schemaA    = `{"type":"object","properties":{"name":{"type":"string"}}}`
	schemaB    = `{"type":"object","properties":{"name":{"type":"string"},"nick":{"type":"string"}}}`
	schemaC    = `{"type":"object","properties":{"name":{"type":"string"},"city":{"type":"string"}}}`
)

var userEdit = Key{"system", "user_edit"}

func rejectMarked(b []byte) error {
	if strings.Contains(string(b), "REJECT") {
		return errors.New("rejected by validator")
	}
	return nil
}

func TestResolveFallsBackToBase(t *testing.T) {
	r := New(nil)
	if got := string(r.Resolve(1, "system", "user_edit", []byte(baseSchema))); got != baseSchema {
		t.Errorf("got %s", got)
	}
	if r.EffectiveLayer(1, "system", "user_edit") != LayerEmbedded {
		t.Error("with no overrides the effective layer is the embedded base")
	}
}

func TestLayerPrecedenceAndTenantIsolation(t *testing.T) {
	r := New(nil)
	base := []byte(baseSchema)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	must(r.ReloadXoluGlobal(map[Key][]byte{userEdit: []byte(schemaA)}))
	if got := string(r.Resolve(7, "system", "user_edit", base)); got != schemaA {
		t.Errorf("global over base: %s", got)
	}
	must(r.ReloadXoluTenant(7, map[Key][]byte{userEdit: []byte(schemaB)}))
	if got := string(r.Resolve(7, "system", "user_edit", base)); got != schemaB {
		t.Errorf("tenant over global: %s", got)
	}
	must(r.ReloadFilesystem(map[Key][]byte{userEdit: []byte(schemaC)}))
	if got := string(r.Resolve(7, "system", "user_edit", base)); got != schemaC {
		t.Errorf("filesystem over tenant: %s", got)
	}

	// Another tenant, and the global view, never see tenant 7's override.
	must(r.ReloadFilesystem(nil))
	if got := string(r.Resolve(8, "system", "user_edit", base)); got != schemaA {
		t.Errorf("tenant 8 should see the global override: %s", got)
	}
	if got := string(r.Resolve(0, "system", "user_edit", base)); got != schemaA {
		t.Errorf("global view should see the global override: %s", got)
	}
	if r.EffectiveLayer(7, "system", "user_edit") != LayerXoluTenant ||
		r.EffectiveLayer(8, "system", "user_edit") != LayerXoluGlobal {
		t.Error("EffectiveLayer disagrees with Resolve")
	}
}

func TestOverridesReplaceWholeAndOnlyTheirKey(t *testing.T) {
	r := New(nil)
	if err := r.ReloadFilesystem(map[Key][]byte{userEdit: []byte(schemaA)}); err != nil {
		t.Fatal(err)
	}
	other := []byte(`{"type":"object"}`)
	if got := r.Resolve(1, "system", "user_create", other); string(got) != string(other) {
		t.Errorf("an override for one schema leaked into another: %s", got)
	}
	if got := r.Resolve(1, "masters", "user_edit", other); string(got) != string(other) {
		t.Errorf("module must be part of the key: %s", got)
	}
}

func TestValidationGateRefusesBadSchemasAndKeepsTheLastGoodLayer(t *testing.T) {
	r := New(rejectMarked)
	good := map[Key][]byte{userEdit: []byte(schemaA)}
	if err := r.ReloadFilesystem(good); err != nil {
		t.Fatal(err)
	}

	bad := map[Key][]byte{
		{"system", "user_edit"}:   []byte(schemaB),
		{"system", "broken_json"}: []byte(`{"type": `),
		{"system", "marked"}:      []byte(`{"x":"REJECT"}`),
	}
	err := r.ReloadFilesystem(bad)
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"system/broken_json", "not valid JSON", "system/marked", "rejected by validator"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q:\n%v", want, err)
		}
	}
	// All-or-nothing: even the valid schemaB in the rejected batch did not land.
	if got := string(r.Resolve(1, "system", "user_edit", []byte(baseSchema))); got != schemaA {
		t.Errorf("a rejected reload changed the layer: %s", got)
	}
}

func TestValidateAndReloadCopyTheirInput(t *testing.T) {
	r := New(nil)
	if err := r.Validate([]byte(`nope`)); err == nil {
		t.Error("Validate must reject invalid JSON")
	}
	in := []byte(schemaA)
	if err := r.ReloadFilesystem(map[Key][]byte{userEdit: in}); err != nil {
		t.Fatal(err)
	}
	in[2] = 'X' // mutate the caller's slice
	if got := string(r.Resolve(1, "system", "user_edit", nil)); got != schemaA {
		t.Errorf("caller mutation leaked in: %s", got)
	}
}

func TestReloadXoluTenantNeedsAPositiveTenant(t *testing.T) {
	r := New(nil)
	for _, id := range []int64{0, -3} {
		if err := r.ReloadXoluTenant(id, map[Key][]byte{userEdit: []byte(schemaA)}); err == nil {
			t.Errorf("tenant %d should be refused", id)
		}
	}
	if err := r.ReloadXoluTenant(2, map[Key][]byte{userEdit: []byte(schemaA)}); err != nil {
		t.Fatal(err)
	}
	if err := r.ReloadXoluTenant(2, nil); err != nil { // clears
		t.Fatal(err)
	}
	if got := string(r.Resolve(2, "system", "user_edit", []byte(baseSchema))); got != baseSchema {
		t.Errorf("cleared tenant layer still overriding: %s", got)
	}
}

func TestOptionsAndExplain(t *testing.T) {
	r := New(nil)
	_ = r.ReloadFilesystem(map[Key][]byte{userEdit: []byte(schemaC)})
	_ = r.ReloadXoluTenant(5, map[Key][]byte{userEdit: []byte(schemaB)})
	_ = r.ReloadXoluGlobal(map[Key][]byte{userEdit: []byte(schemaA)})
	base := []byte(baseSchema)

	c := r.Explain(5, "system", "user_edit")
	if len(c) != 3 || c[0].Layer != LayerFilesystem || c[1].Layer != LayerXoluTenant || c[2].Layer != LayerXoluGlobal {
		t.Fatalf("Explain = %+v", c)
	}
	if !c[0].Effective || c[1].Effective || c[2].Effective {
		t.Errorf("only the top layer is effective: %+v", c)
	}

	r.SetOptions(5, Options{DisableFilesystem: true})
	if got := string(r.Resolve(5, "system", "user_edit", base)); got != schemaB {
		t.Errorf("filesystem disabled: %s", got)
	}
	if got := string(r.Resolve(6, "system", "user_edit", base)); got != schemaC {
		t.Errorf("options must be per tenant: %s", got)
	}
	c = r.Explain(5, "system", "user_edit")
	if !c[0].Disabled || c[0].Effective || !c[1].Effective {
		t.Errorf("disabled handling: %+v", c)
	}

	r.SetOptions(5, Options{DisableFilesystem: true, DisableXolu: true})
	if got := string(r.Resolve(5, "system", "user_edit", base)); got != baseSchema {
		t.Errorf("all layers disabled should give the base: %s", got)
	}
	if r.EffectiveLayer(5, "system", "user_edit") != LayerEmbedded {
		t.Error("EffectiveLayer should be embedded with every layer disabled")
	}
	if (r.Options(5) != Options{DisableFilesystem: true, DisableXolu: true}) {
		t.Errorf("Options = %+v", r.Options(5))
	}
	r.SetOptions(5, Options{})
	if (r.Options(5) != Options{}) {
		t.Error("zero Options should clear")
	}
}

func TestExplainAgreesWithResolve(t *testing.T) {
	r := New(nil)
	_ = r.ReloadFilesystem(map[Key][]byte{{"a", "x"}: []byte(`{"v":1}`)})
	_ = r.ReloadXoluTenant(3, map[Key][]byte{{"a", "y"}: []byte(`{"v":2}`)})
	_ = r.ReloadXoluGlobal(map[Key][]byte{{"a", "y"}: []byte(`{"v":3}`), {"a", "z"}: []byte(`{"v":4}`)})
	base := []byte(`{"v":0}`)
	for _, tenant := range []int64{0, 3, 9} {
		for _, k := range []Key{{"a", "x"}, {"a", "y"}, {"a", "z"}, {"a", "none"}} {
			want := string(base)
			for _, c := range r.Explain(tenant, k.Module, k.Name) {
				if c.Effective {
					want = string(c.Schema)
				}
			}
			if got := string(r.Resolve(tenant, k.Module, k.Name, base)); got != want {
				t.Errorf("tenant %d %s: Resolve=%s Explain=%s", tenant, k, got, want)
			}
		}
	}
}

func TestOverridden(t *testing.T) {
	r := New(nil)
	_ = r.ReloadFilesystem(map[Key][]byte{{"system", "b"}: []byte(`{}`)})
	_ = r.ReloadXoluGlobal(map[Key][]byte{{"masters", "a"}: []byte(`{}`), {"system", "b"}: []byte(`{}`)})
	_ = r.ReloadXoluTenant(4, map[Key][]byte{{"system", "a"}: []byte(`{}`)})

	want4 := []Key{{"masters", "a"}, {"system", "a"}, {"system", "b"}}
	if got := r.Overridden(4); !reflect.DeepEqual(got, want4) {
		t.Errorf("Overridden(4) = %v, want %v", got, want4)
	}
	want0 := []Key{{"masters", "a"}, {"system", "b"}}
	if got := r.Overridden(0); !reflect.DeepEqual(got, want0) {
		t.Errorf("Overridden(0) = %v, want %v", got, want0)
	}
}

func TestConcurrentReloadAndResolveIsAtomic(t *testing.T) {
	r := New(nil)
	gens := []map[Key][]byte{
		{{"m", "a"}: []byte(`{"g":1}`), {"m", "b"}: []byte(`{"g":1}`)},
		{{"m", "a"}: []byte(`{"g":2}`), {"m", "b"}: []byte(`{"g":2}`)},
	}
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			_ = r.ReloadFilesystem(gens[i%2])
		}
	}()

	var mixed atomic.Int64
	var rwg sync.WaitGroup
	for n := 0; n < 4; n++ {
		rwg.Add(1)
		go func() {
			defer rwg.Done()
			for i := 0; i < 3000; i++ {
				// One consistent view per call via Overridden+Explain is not
				// atomic across calls; Resolve of a single key always is.
				a := string(r.Resolve(0, "m", "a", nil))
				if a != "" && a != `{"g":1}` && a != `{"g":2}` {
					mixed.Add(1)
				}
			}
		}()
	}
	rwg.Wait()
	close(stop)
	wg.Wait()
	if mixed.Load() != 0 {
		t.Errorf("%d reads returned something that is no complete generation", mixed.Load())
	}
}

// ---- Compat ----

func TestCompat(t *testing.T) {
	cases := []struct {
		name     string
		override string
		want     []string
	}{
		{"identical", baseSchema, nil},
		{"property removed", `{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`,
			[]string{`property "age" was removed`}},
		{"type changed", `{"type":"object","properties":{"name":{"type":"string"},"age":{"type":"string"}},"required":["name"]}`,
			[]string{`property "age" changed type from integer to string`}},
		{"became required", `{"type":"object","properties":{"name":{"type":"string"},"age":{"type":"integer"}},"required":["name","age"]}`,
			[]string{`property "age" became required`}},
		{"new optional property is fine", `{"type":"object","properties":{"name":{"type":"string"},"age":{"type":"integer"},"x":{"type":"string"}},"required":["name"]}`, nil},
		{"several", `{"type":"object","properties":{"name":{"type":"number"}},"required":["name","zed"]}`,
			[]string{`property "age" was removed`, `property "name" changed type from string to number`, `property "zed" became required`}},
	}
	for _, c := range cases {
		got := Compat([]byte(baseSchema), []byte(c.override))
		if len(got) == 0 && len(c.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	if w := Compat([]byte(`{bad`), []byte(baseSchema)); len(w) != 1 || !strings.Contains(w[0], "base schema") {
		t.Errorf("unreadable base: %v", w)
	}
	if w := Compat([]byte(baseSchema), []byte(`{bad`)); len(w) != 1 || !strings.Contains(w[0], "override") {
		t.Errorf("unreadable override: %v", w)
	}
}

// ---- Filesystem layer ----

var writeCounter int64

func writeSchema(t *testing.T, dir, module, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, module), 0o700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, module, name+".json")
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
	writeSchema(t, dir, "system", "user_edit", schemaA)
	writeSchema(t, dir, "masters", "brand", schemaB)
	if err := os.WriteFile(filepath.Join(dir, "top_level.json"), []byte(`{}`), 0o600); err != nil { // ignored: not in a module dir
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "system", "notes.txt"), []byte(`x`), 0o600); err != nil { // ignored
		t.Fatal(err)
	}
	got, err := LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || string(got[userEdit]) != schemaA || string(got[Key{"masters", "brand"}]) != schemaB {
		t.Errorf("LoadDir = %v", got)
	}
}

func TestLoadDirRejectsBadNamesAndMissingDir(t *testing.T) {
	dir := t.TempDir()
	writeSchema(t, dir, "system", "Bad-Name", schemaA)
	if _, err := LoadDir(dir); err == nil || !strings.Contains(err.Error(), "system/Bad-Name.json") {
		t.Errorf("expected the bad file to be named, got %v", err)
	}
	if _, err := LoadDir(filepath.Join(dir, "nope")); err == nil {
		t.Error("expected an error for a missing directory")
	}
}

func TestFileLayerReloadValidatesAndKeepsLastGood(t *testing.T) {
	dir := t.TempDir()
	r := New(rejectMarked)
	fl := NewFileLayer(r, dir)
	base := []byte(baseSchema)

	writeSchema(t, dir, "system", "user_edit", schemaA)
	writeSchema(t, dir, "masters", "brand", schemaB)
	if err := fl.Reload(); err != nil {
		t.Fatal(err)
	}
	if got := string(r.Resolve(1, "system", "user_edit", base)); got != schemaA {
		t.Errorf("Resolve = %s", got)
	}
	st := fl.Status()
	if st.LoadedAt.IsZero() || st.LastError != "" ||
		!reflect.DeepEqual(st.Schemas, []Key{{"masters", "brand"}, {"system", "user_edit"}}) {
		t.Errorf("status = %+v", st)
	}

	// An edit the validator rejects is set aside alone: what was in force for
	// that form stays, and a good edit made at the same time still loads.
	writeSchema(t, dir, "system", "user_edit", `{"x":"REJECT"}`)
	writeSchema(t, dir, "masters", "brand", schemaC)
	if err := fl.Reload(); err != nil {
		t.Fatalf("a file set aside does not fail the load: %v", err)
	}
	if got := string(r.Resolve(1, "system", "user_edit", base)); got != schemaA {
		t.Errorf("a rejected edit replaced the live schema: %s", got)
	}
	if got := string(r.Resolve(1, "masters", "brand", base)); got != schemaC {
		t.Errorf("the rejected file held back another one: %s", got)
	}
	if st := fl.Status(); len(st.Refused) != 1 || st.Refused[0].Key != (Key{"system", "user_edit"}) || !st.Refused[0].Kept || st.Refused[0].Reason == "" {
		t.Errorf("the rejection should be recorded in Status: %+v", st)
	}

	writeSchema(t, dir, "system", "user_edit", schemaC)
	if err := fl.Reload(); err != nil || fl.Status().LastError != "" || len(fl.Status().Refused) != 0 {
		t.Fatalf("recovery: %v %+v", err, fl.Status())
	}
	if got := string(r.Resolve(1, "system", "user_edit", base)); got != schemaC {
		t.Errorf("Resolve after fix = %s", got)
	}
}

func TestFileLayerReloadIfChanged(t *testing.T) {
	dir := t.TempDir()
	r := New(nil)
	fl := NewFileLayer(r, dir)
	writeSchema(t, dir, "system", "user_edit", schemaA)

	if did, err := fl.ReloadIfChanged(); !did || err != nil {
		t.Fatalf("first: did=%v err=%v", did, err)
	}
	if did, _ := fl.ReloadIfChanged(); did {
		t.Error("nothing changed, yet a reload was attempted")
	}
	writeSchema(t, dir, "system", "user_edit", schemaB)
	if did, err := fl.ReloadIfChanged(); !did || err != nil {
		t.Fatalf("after change: did=%v err=%v", did, err)
	}
	writeSchema(t, dir, "system", "user_edit", `{broken`)
	if did, err := fl.ReloadIfChanged(); !did || err != nil || len(fl.Status().Refused) != 1 {
		t.Fatalf("broken: did=%v err=%v %+v", did, err, fl.Status())
	}
	if got := string(r.Resolve(1, "system", "user_edit", nil)); got != schemaB {
		t.Errorf("a broken file replaced the live schema: %s", got)
	}
	if did, _ := fl.ReloadIfChanged(); did {
		t.Error("an unchanged broken file must not be retried on every poll")
	}
}

func TestFileLayerPollHonoursActiveAndStops(t *testing.T) {
	dir := t.TempDir()
	r := New(nil)
	fl := NewFileLayer(r, dir)
	var active atomic.Bool

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		fl.Poll(ctx, 5*time.Millisecond, active.Load)
		close(done)
	}()

	writeSchema(t, dir, "system", "user_edit", schemaA)
	time.Sleep(60 * time.Millisecond)
	if got := r.Resolve(1, "system", "user_edit", []byte(baseSchema)); string(got) != baseSchema {
		t.Errorf("polling ran while inactive: %s", got)
	}

	active.Store(true)
	waitFor(t, "load once active", func() bool {
		return string(r.Resolve(1, "system", "user_edit", []byte(baseSchema))) == schemaA
	})
	writeSchema(t, dir, "system", "user_edit", schemaB)
	waitFor(t, "edit to load", func() bool {
		return string(r.Resolve(1, "system", "user_edit", []byte(baseSchema))) == schemaB
	})

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Poll did not stop on cancel")
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
