package modules

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestRegistry_VisibleTo_FiltersByHasAccess(t *testing.T) {
	r := NewRegistry()
	r.Register(Module{ID: "assets", Group: "apps", HasAccess: func(role string) bool { return role != "viewer" }})
	r.Register(Module{ID: "documents", Group: "apps", HasAccess: func(role string) bool { return true }})
	r.Register(Module{ID: "system", Group: "system", HasAccess: func(role string) bool { return true }})

	admin := r.VisibleTo("admin")
	if len(admin) != 3 {
		t.Fatalf("admin: got %d visible modules, want 3", len(admin))
	}

	viewer := r.VisibleTo("viewer")
	if len(viewer) != 2 {
		t.Fatalf("viewer: got %d visible modules, want 2 (assets excluded)", len(viewer))
	}
	for _, m := range viewer {
		if m.ID == "assets" {
			t.Error("viewer should not see assets")
		}
	}
}

func TestRegistry_VisibleTo_NilHasAccess_FailsClosed(t *testing.T) {
	r := NewRegistry()
	r.Register(Module{ID: "broken", Group: "apps"}) // no HasAccess set

	visible := r.VisibleTo("admin")
	if len(visible) != 0 {
		t.Errorf("a module with nil HasAccess must be visible to no one, got %d visible", len(visible))
	}
}

func TestRegistry_VisibleTo_PreservesRegistrationOrder(t *testing.T) {
	r := NewRegistry()
	always := func(string) bool { return true }
	r.Register(Module{ID: "system", HasAccess: always})
	r.Register(Module{ID: "assets", HasAccess: always})
	r.Register(Module{ID: "documents", HasAccess: always})

	visible := r.VisibleTo("admin")
	want := []string{"system", "assets", "documents"}
	if len(visible) != len(want) {
		t.Fatalf("got %d modules, want %d", len(visible), len(want))
	}
	for i, id := range want {
		if visible[i].ID != id {
			t.Errorf("visible[%d].ID = %q, want %q", i, visible[i].ID, id)
		}
	}
}

// Home is where "/" goes on an instance without the Assets module: an
// application the viewer has, before the modules every instance has.
func TestRegistry_Home(t *testing.T) {
	yes := func(string) bool { return true }
	no := func(string) bool { return false }
	r := NewRegistry()
	r.Register(Module{ID: "assets", Path: "/", HasAccess: no})
	r.Register(Module{ID: "documents", Path: "/documents", HasAccess: yes})
	r.Register(Module{ID: "issues", Path: "/issues", Primary: true, HasAccessFor: func(tenant int64, role string) bool { return tenant == 7 }})
	r.Register(Module{ID: "accounting", Path: "/accounting", Primary: true, HasAccess: func(role string) bool { return role != "viewer" }})
	for _, c := range []struct {
		tenant int64
		role   string
		want   string
	}{
		{1, "admin", "/accounting"}, // the first application the viewer has
		{7, "admin", "/issues"},     // in registration order
		{1, "viewer", "/documents"}, // no application: the first module with a path
	} {
		if got := r.Home(c.tenant, c.role); got != c.want {
			t.Errorf("Home(%d, %q) = %q, want %q", c.tenant, c.role, got, c.want)
		}
	}
	// "/" is never an answer, and a viewer with nothing gets ""
	empty := NewRegistry()
	empty.Register(Module{ID: "assets", Path: "/", Primary: true, HasAccess: yes})
	if got := empty.Home(1, "admin"); got != "" {
		t.Errorf("Home with only \"/\" = %q, want \"\"", got)
	}
}

func TestRegistry_Register_PanicsOnDuplicateID(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected a panic on duplicate module ID, got none")
		}
	}()
	r := NewRegistry()
	r.Register(Module{ID: "assets"})
	r.Register(Module{ID: "assets"}) // duplicate -- must panic
}

func TestRegistry_All_ReturnsEveryModuleRegardlessOfAccess(t *testing.T) {
	r := NewRegistry()
	r.Register(Module{ID: "assets", HasAccess: func(string) bool { return false }})
	r.Register(Module{ID: "documents", HasAccess: func(string) bool { return false }})

	all := r.All()
	if len(all) != 2 {
		t.Errorf("All() = %d modules, want 2 (unfiltered, unlike VisibleTo)", len(all))
	}
}

func TestRegistry_MountAll_CallsEveryNonNilMountRoutes(t *testing.T) {
	r := NewRegistry()
	calls := 0
	r.Register(Module{ID: "assets", MountRoutes: func(mux *http.ServeMux) { calls++ }})
	r.Register(Module{ID: "documents"}) // nil MountRoutes -- must not panic
	r.Register(Module{ID: "system", MountRoutes: func(mux *http.ServeMux) { calls++ }})

	r.MountAll(http.NewServeMux())
	if calls != 2 {
		t.Errorf("MountAll invoked MountRoutes %d times, want 2 (skipping the nil one)", calls)
	}
}

func TestRegistry_Empty_VisibleToReturnsNoneNotPanic(t *testing.T) {
	r := NewRegistry()
	if got := r.VisibleTo("admin"); len(got) != 0 {
		t.Errorf("empty registry: VisibleTo = %d, want 0", len(got))
	}
}

// ============================================================================
// Adversarial tests below -- deliberately probing what happens with
// malformed callers, zero-value structs, panicking closures, and
// concurrent access, rather than only well-behaved bootstrap sequences.
// ============================================================================

// TestRegistry_ZeroValue_RegisterWorksWithoutNewRegistry guards
// Register's own defensive "if r.seen == nil" init (registry.go) --
// confirms a Registry constructed as var r Registry (bypassing
// NewRegistry entirely, e.g. as a zero-value struct field) doesn't
// panic on its first Register call. Not a hypothetical: NewRegistry
// does nothing NewRegistry-specific besides this init, so anyone
// embedding Registry as a struct field and forgetting to call it would
// hit this path for real.
func TestRegistry_ZeroValue_RegisterWorksWithoutNewRegistry(t *testing.T) {
	var r Registry // deliberately not NewRegistry()
	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("Register on a zero-value Registry panicked: %v", rec)
		}
	}()
	r.Register(Module{ID: "assets"})
	if len(r.All()) != 1 {
		t.Errorf("got %d modules after one Register, want 1", len(r.All()))
	}
}

// TestRegistry_ZeroValue_ReadsWorkBeforeAnyRegister guards VisibleTo/
// All/MountAll against a nil r.modules slice specifically (a zero-value
// Registry that's never had Register called at all) -- Go's nil-slice
// iteration is safe, but this confirms it, not assumes it, for every
// read method this package exposes.
func TestRegistry_ZeroValue_ReadsWorkBeforeAnyRegister(t *testing.T) {
	var r Registry
	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("a read method on a never-registered zero-value Registry panicked: %v", rec)
		}
	}()
	if got := r.VisibleTo("admin"); len(got) != 0 {
		t.Errorf("VisibleTo on empty zero-value registry = %d, want 0", len(got))
	}
	if got := r.All(); len(got) != 0 {
		t.Errorf("All on empty zero-value registry = %d, want 0", len(got))
	}
	r.MountAll(http.NewServeMux()) // must not panic on an empty modules slice
}

// TestRegistry_Register_EmptyStringID_StillDedupes confirms an empty
// string isn't silently treated as "no ID" or skipped by the dedup
// check -- two Register calls with ID: "" must still panic on the
// second, exactly like any other duplicate. A real, if unlikely,
// configuration mistake (a Module literal missing its ID field
// entirely defaults to "") that should fail loudly at startup, not
// silently coexist with a real "" registration.
func TestRegistry_Register_EmptyStringID_StillDedupes(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected a panic registering a second Module with ID \"\", got none")
		}
	}()
	r := NewRegistry()
	r.Register(Module{ID: ""})
	r.Register(Module{ID: ""})
}

// TestRegistry_Register_IDsAreCaseSensitive documents, rather than
// assumes, actual behaviour: "Assets" and "assets" are different keys
// in r.seen (a plain map[string]bool, no normalisation), so both
// registrations succeed as two distinct modules. Not necessarily
// desirable, but this is what the code does today, and a future
// reader/refactor should see this test break if that assumption ever
// silently changes.
func TestRegistry_Register_IDsAreCaseSensitive(t *testing.T) {
	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("registering \"Assets\" and \"assets\" as separate modules should not panic (they are different map keys), got: %v", rec)
		}
	}()
	r := NewRegistry()
	r.Register(Module{ID: "assets"})
	r.Register(Module{ID: "Assets"})
	if len(r.All()) != 2 {
		t.Errorf("got %d modules, want 2 -- \"assets\" and \"Assets\" should be treated as distinct IDs", len(r.All()))
	}
}

// TestRegistry_VisibleTo_PropagatesAPanickingHasAccess documents
// current, real behaviour rather than an aspiration: HasAccess is
// called with no recover() anywhere in VisibleTo, so a module whose
// HasAccess implementation panics (a nil-pointer deref inside a buggy
// closure, for instance) takes down the entire selector render for
// every user, not just fails closed for that one module. Worth knowing
// this is the current contract -- a HasAccess closure is expected to
// be as trustworthy as any other startup-registered code, not
// sandboxed.
func TestRegistry_VisibleTo_PropagatesAPanickingHasAccess(t *testing.T) {
	r := NewRegistry()
	r.Register(Module{ID: "broken", HasAccess: func(string) bool { panic("boom") }})

	defer func() {
		if recover() == nil {
			t.Error("expected VisibleTo to propagate a panic from a broken HasAccess, got none -- if this changes, update this test to match the new (safer) contract deliberately, not by accident")
		}
	}()
	r.VisibleTo("admin")
}

// TestRegistry_MountAll_APanickingMountRoutesStopsLaterModules
// documents the same "no recover, fail loud" contract for MountAll:
// a module whose MountRoutes panics (e.g. a genuine duplicate route
// pattern bug) stops MountAll before any module registered after it
// gets mounted at all. This is arguably the right behaviour for a
// once-at-startup operation (better to crash loudly than silently
// serve a half-mounted app), but it's worth a test making the
// consequence explicit: it's not just "this module fails," it's
// "everything after it in registration order never mounts."
func TestRegistry_MountAll_APanickingMountRoutesStopsLaterModules(t *testing.T) {
	r := NewRegistry()
	firstMounted := false
	laterMounted := false
	r.Register(Module{ID: "first", MountRoutes: func(mux *http.ServeMux) { firstMounted = true }})
	r.Register(Module{ID: "broken", MountRoutes: func(mux *http.ServeMux) { panic("duplicate route pattern") }})
	r.Register(Module{ID: "later", MountRoutes: func(mux *http.ServeMux) { laterMounted = true }})

	func() {
		defer func() { recover() }() // swallow only for this test's own purposes
		r.MountAll(http.NewServeMux())
	}()

	if !firstMounted {
		t.Error("the module registered before the panicking one should still have been mounted")
	}
	if laterMounted {
		t.Error("the module registered after the panicking one should NOT have been mounted -- MountAll has no per-module recovery, a panic stops the whole loop")
	}
}

// TestRegistry_MountAll_RealDuplicateRoutePanicPropagates is the same
// finding as above, but against the real http.ServeMux (not a fake
// panic), to confirm MountAll doesn't accidentally swallow or wrap
// ServeMux's own panic in a way that changes its type or message --
// exactly the failure mode this project cares about catching at
// startup (two modules whose real routes overlap), not masking.
func TestRegistry_MountAll_RealDuplicateRoutePanicPropagates(t *testing.T) {
	r := NewRegistry()
	r.Register(Module{ID: "first", MountRoutes: func(mux *http.ServeMux) {
		mux.HandleFunc("GET /clash", func(w http.ResponseWriter, r *http.Request) {})
	}})
	r.Register(Module{ID: "second", MountRoutes: func(mux *http.ServeMux) {
		mux.HandleFunc("GET /clash", func(w http.ResponseWriter, r *http.Request) {}) // exact duplicate pattern
	}})

	defer func() {
		rec := recover()
		if rec == nil {
			t.Fatal("expected http.ServeMux's own panic on a duplicate pattern to propagate through MountAll, got none")
		}
		if !strings.Contains(fmt.Sprint(rec), "GET /clash") {
			t.Errorf("panic value doesn't mention the clashing pattern, got: %v", rec)
		}
	}()
	r.MountAll(http.NewServeMux())
}

// TestRegistry_VisibleButUnmountable_IsAllowedByTheTypeSystem documents
// a real gap: nothing in Module or Registry prevents registering a
// module whose HasAccess returns true (so it appears in the selector,
// clickable) but whose MountRoutes is nil (so its Path 404s once
// clicked) -- exactly the "ships a broken feature" scenario
// MODULE_SPEC.md's own reasoning for deferring Documents' registration
// until MOD-7 was worried about, but the worry lived in a code
// comment at the call site (internal/server/server.go), not an
// invariant this package itself enforces. This test doesn't assert the
// gap should be closed (that's a design call, not obviously wrong --
// Documents itself was deliberately unregistered rather than
// registered-with-nil for exactly this reason), just that it exists
// and is worth remembering next time a new module is wired up.
func TestRegistry_VisibleButUnmountable_IsAllowedByTheTypeSystem(t *testing.T) {
	r := NewRegistry()
	r.Register(Module{
		ID:          "half-wired",
		Group:       "apps",
		Path:        "/half-wired",
		HasAccess:   func(string) bool { return true },
		MountRoutes: nil, // nothing serves Path at all
	})

	visible := r.VisibleTo("admin")
	if len(visible) != 1 {
		t.Fatalf("got %d visible modules, want 1", len(visible))
	}
	// The registry itself raises no error, no warning, nothing -- this
	// assertion is the whole point: Register/VisibleTo succeed exactly
	// as if this were a fully-wired module.
	mux := http.NewServeMux()
	r.MountAll(mux) // no-op for this module (nil MountRoutes), no panic either
	req := httptest.NewRequest("GET", "/half-wired", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /half-wired: got %d, want 404 -- confirming the visible-but-unmountable module really does 404, not just that the registry allowed registering it", rec.Code)
	}
}

// TestRegistry_ConcurrentVisibleTo_NoRace simulates the actual runtime
// pattern (many simultaneous requests, each calling VisibleTo for its
// own user's role, against one shared *Registry built once at
// startup) -- run with -race to catch a data race, not just a
// deadlock or crash. VisibleTo only reads r.modules; this is the test
// that would catch it if a future change ever made that read unsafe
// (e.g. someone adding a post-startup Register call reachable from a
// request handler).
func TestRegistry_ConcurrentVisibleTo_NoRace(t *testing.T) {
	r := NewRegistry()
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("module-%d", i)
		r.Register(Module{ID: id, Group: "apps", HasAccess: func(role string) bool { return role == "admin" || role == "manager" }})
	}

	var wg sync.WaitGroup
	roles := []string{"admin", "manager", "operator", "viewer", "unknown-role"}
	for i := 0; i < 100; i++ {
		wg.Add(1)
		role := roles[i%len(roles)]
		go func() {
			defer wg.Done()
			_ = r.VisibleTo(role)
			_ = r.All()
		}()
	}
	wg.Wait()
}

// TestRegistry_HasAccess_ReceivesExactlyTheRolePassedIn is a narrow but
// real check: VisibleTo must not normalise, trim, or lowercase role
// before handing it to HasAccess -- if it silently did, a module's own
// HasAccess closure comparing against exact role constants (as every
// real one in this codebase does, e.g. auth.HasModuleAccess) could be
// fooled by whitespace or casing tricks in the opposite direction from
// what's intended.
func TestRegistry_HasAccess_ReceivesExactlyTheRolePassedIn(t *testing.T) {
	r := NewRegistry()
	var received string
	r.Register(Module{ID: "probe", HasAccess: func(role string) bool {
		received = role
		return true
	}})

	weirdRole := "  Admin\t\n"
	r.VisibleTo(weirdRole)
	if received != weirdRole {
		t.Errorf("HasAccess received %q, want the exact input %q -- VisibleTo must not silently transform role", received, weirdRole)
	}
}
