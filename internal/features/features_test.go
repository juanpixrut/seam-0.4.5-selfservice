package features

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func env(m map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		v, ok := m[name]
		return v, ok
	}
}

func mustResolve(t *testing.T, r *Registry, in Inputs) *Set {
	t.Helper()
	s, err := r.Resolve(in)
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	return s
}

func mustFail(t *testing.T, r *Registry, in Inputs, wantSubstrings ...string) {
	t.Helper()
	s, err := r.Resolve(in)
	if err == nil {
		t.Fatalf("Resolve: expected an error, got a Set (%v)", s)
	}
	for _, w := range wantSubstrings {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("error %q does not mention %q", err.Error(), w)
		}
	}
}

func TestParseBool(t *testing.T) {
	good := map[string]bool{
		"1": true, "true": true, "TRUE": true, "True": true, "yes": true, "on": true, " On ": true,
		"0": false, "false": false, "FALSE": false, "no": false, "off": false, " Off ": false,
	}
	for in, want := range good {
		got, err := ParseBool(in)
		if err != nil || got != want {
			t.Errorf("ParseBool(%q) = %v, %v; want %v, nil", in, got, err, want)
		}
	}
	for _, in := range []string{"", "enable", "2", "tru", "y", "enabled", "null"} {
		if _, err := ParseBool(in); err == nil {
			t.Errorf("ParseBool(%q): expected an error", in)
		}
	}
}

func TestParseList(t *testing.T) {
	items, err := ParseList(" Developer, +demo , -work_orders,, ")
	if err != nil {
		t.Fatal(err)
	}
	want := []ListItem{{"developer", true}, {"demo", true}, {"work_orders", false}}
	if len(items) != len(want) {
		t.Fatalf("got %v, want %v", items, want)
	}
	for i := range want {
		if items[i] != want[i] {
			t.Errorf("item %d = %v, want %v", i, items[i], want[i])
		}
	}
	if items, err := ParseList(""); err != nil || len(items) != 0 {
		t.Errorf("ParseList(\"\") = %v, %v; want empty, nil", items, err)
	}
	for _, bad := range []string{"dev-mode", "9lives", "-", "a b", "demo;qr"} {
		if _, err := ParseList(bad); err == nil {
			t.Errorf("ParseList(%q): expected an error", bad)
		}
	}
}

func TestNewRegistryRejectsBadDeclarations(t *testing.T) {
	cases := []struct {
		name  string
		flags []Flag
		profs []Profile
		want  string
	}{
		{"bad key", []Flag{{Key: "Bad-Key"}}, nil, "invalid key"},
		{"empty key", []Flag{{Key: ""}}, nil, "invalid key"},
		{"duplicate key", []Flag{{Key: "a"}, {Key: "a"}}, nil, "more than once"},
		{"bad env name", []Flag{{Key: "a", EnvVar: "lower"}}, nil, "invalid environment variable"},
		{"duplicate env", []Flag{{Key: "a", EnvVar: "X_A"}, {Key: "b", EnvVar: "X_A"}}, nil, "already used"},
		{"duplicate config key", []Flag{{Key: "a", ConfigKey: "k"}, {Key: "b", ConfigKey: "k"}}, nil, "config key"},
		{"unknown requires", []Flag{{Key: "a", Requires: []string{"zzz"}}}, nil, "unknown flag"},
		{"self requires", []Flag{{Key: "a", Requires: []string{"a"}}}, nil, "requires itself"},
		{"bad profile name", []Flag{{Key: "a"}}, []Profile{{Name: "Bad Name"}}, "invalid name"},
		{"duplicate profile", []Flag{{Key: "a"}}, []Profile{{Name: "p"}, {Name: "p"}}, "more than once"},
		{"profile unknown flag", []Flag{{Key: "a"}}, []Profile{{Name: "p", Defaults: map[string]bool{"zzz": true}}}, "unknown flag"},
	}
	for _, c := range cases {
		_, err := NewRegistry(c.flags, c.profs)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v; want it to mention %q", c.name, err, c.want)
		}
	}
}

func TestNewRegistryRequiresMayReferToLaterFlags(t *testing.T) {
	_, err := NewRegistry([]Flag{{Key: "a", Requires: []string{"b"}}, {Key: "b"}}, nil)
	if err != nil {
		t.Fatalf("declaration order must not matter: %v", err)
	}
}

func TestNewRegistryReportsAllProblemsTogether(t *testing.T) {
	_, err := NewRegistry([]Flag{{Key: "A"}, {Key: "b", Requires: []string{"zzz"}}}, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid key") || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("want both problems reported, got %v", err)
	}
}

func TestRegistryReturnsCopies(t *testing.T) {
	r, err := NewRegistry([]Flag{{Key: "a", Requires: nil}, {Key: "b", Requires: []string{"a"}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	fl := r.Flags()
	fl[0].Key = "mutated"
	fl[1].Requires[0] = "mutated"
	if f, _ := r.Lookup("a"); f.Key != "a" {
		t.Error("Flags() leaked the internal slice")
	}
	if f, _ := r.Lookup("b"); f.Requires[0] != "a" {
		t.Error("Flags() leaked an internal Requires slice")
	}
}

// ---- Resolution ----

func testRegistry(t *testing.T) *Registry {
	t.Helper()
	r, err := NewRegistry([]Flag{
		{Key: "on_by_default", EnvVar: "T_ON", ConfigKey: "on_by_default", Default: true},
		{Key: "off_by_default", EnvVar: "T_OFF", ConfigKey: "off_by_default"},
		{Key: "noenv", ConfigKey: "noenv"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestResolveDefaults(t *testing.T) {
	r := testRegistry(t)
	s := mustResolve(t, r, Inputs{LookupEnv: env(nil)})
	if !s.On("on_by_default") || s.On("off_by_default") || s.On("noenv") {
		t.Error("defaults not applied")
	}
	for _, st := range s.States() {
		if st.Source != SourceDefault {
			t.Errorf("%s: source = %v, want default", st.Flag.Key, st.Source)
		}
	}
}

func TestResolvePrecedence(t *testing.T) {
	r := testRegistry(t)

	s := mustResolve(t, r, Inputs{
		Config:    map[string]bool{"off_by_default": true},
		LookupEnv: env(nil),
	})
	if !s.On("off_by_default") || s.Source("off_by_default") != SourceConfig {
		t.Errorf("config: got %v from %v", s.On("off_by_default"), s.Source("off_by_default"))
	}

	// env beats config
	s = mustResolve(t, r, Inputs{
		Config:    map[string]bool{"off_by_default": true},
		LookupEnv: env(map[string]string{"T_OFF": "0"}),
	})
	if s.On("off_by_default") || s.Source("off_by_default") != SourceEnv {
		t.Errorf("env over config: got %v from %v", s.On("off_by_default"), s.Source("off_by_default"))
	}

	// list beats env
	s = mustResolve(t, r, Inputs{
		LookupEnv: env(map[string]string{"T_OFF": "0", EnvList: "off_by_default"}),
	})
	if !s.On("off_by_default") || s.Source("off_by_default") != SourceList {
		t.Errorf("list over env: got %v from %v", s.On("off_by_default"), s.Source("off_by_default"))
	}

	// list can switch off, and can reach flags that have no env var
	s = mustResolve(t, r, Inputs{
		LookupEnv: env(map[string]string{EnvList: "noenv,-on_by_default"}),
	})
	if !s.On("noenv") || s.On("on_by_default") {
		t.Error("list on/off not applied")
	}
}

func TestResolveEmptyEnvIsUnset(t *testing.T) {
	r := testRegistry(t)
	s := mustResolve(t, r, Inputs{LookupEnv: env(map[string]string{"T_ON": "  ", EnvList: ""})})
	if !s.On("on_by_default") || s.Source("on_by_default") != SourceDefault {
		t.Error("blank env var must be treated as unset")
	}
}

func TestResolveReportsEveryProblem(t *testing.T) {
	r := testRegistry(t)
	mustFail(t, r, Inputs{
		Config: map[string]bool{"nope": true},
		LookupEnv: env(map[string]string{
			"T_ON":  "enabled",
			"T_OFF": "maybe",
			EnvList: "ghost",
		}),
	}, "T_ON", "T_OFF", "unknown feature flag \"nope\"", "ghost")
}

func TestResolveRejectsUnknownProfile(t *testing.T) {
	r := Catalog()
	mustFail(t, r, Inputs{LookupEnv: env(map[string]string{EnvProfile: "staging"})}, "unknown profile", "production")
}

func TestRequiresAuth(t *testing.T) {
	r := Catalog()
	in := Inputs{LookupEnv: env(map[string]string{"SEAM_ENABLE_DEVELOPER": "1"})}
	mustFail(t, r, in, "developer", "authentication")
	in.AuthEnabled = true
	s := mustResolve(t, r, in)
	if !s.On(Developer) || s.Source(Developer) != SourceEnv {
		t.Error("developer flag not on with auth enabled")
	}
}

func TestRequiresPrerequisiteFlag(t *testing.T) {
	r := Catalog()
	// Explicitly pinning a runtime developer flag while the module is off
	// is a configuration error: it could never take effect.
	mustFail(t, r, Inputs{AuthEnabled: true,
		LookupEnv: env(map[string]string{EnvList: HotloadI18N})}, "hotload_i18n", "developer")

	// Same with developer on: fine.
	mustResolve(t, r, Inputs{AuthEnabled: true,
		LookupEnv: env(map[string]string{EnvList: "developer," + HotloadI18N})})

	// Default-true runtime flags do not trip the check merely by being on.
	mustResolve(t, r, Inputs{LookupEnv: env(nil)})
	// Re-stating a default-true flag explicitly is harmless.
	mustResolve(t, r, Inputs{LookupEnv: env(map[string]string{EnvList: OverlayXolu})})
}

func TestProfileProductionForbidsDevClass(t *testing.T) {
	r := Catalog()
	prod := map[string]string{EnvProfile: ProfileProduction}

	mustResolve(t, r, Inputs{LookupEnv: env(prod)}) // nothing dev-class on: fine

	for _, v := range []string{"SEAM_ENABLE_DEMO", "SEAM_ENABLE_DEVELOPER"} {
		e := map[string]string{EnvProfile: ProfileProduction, v: "1"}
		mustFail(t, r, Inputs{AuthEnabled: true, LookupEnv: env(e)}, "production", "forbids")
	}
	// dev_mode has no env var; it arrives via config.
	mustFail(t, r, Inputs{Config: map[string]bool{DevMode: true}, LookupEnv: env(prod)}, "dev_mode")

	// Explicit override.
	e := map[string]string{EnvProfile: ProfileProduction, "SEAM_ENABLE_DEMO": "1", EnvAllowDev: "1"}
	mustResolve(t, r, Inputs{LookupEnv: env(e)})

	// Preview-class flags are not forbidden.
	mustResolve(t, r, Inputs{LookupEnv: env(map[string]string{EnvProfile: ProfileProduction, "SEAM_ENABLE_WORK_ORDERS": "1"})})

	// A bad override value is itself reported.
	mustFail(t, r, Inputs{LookupEnv: env(map[string]string{EnvProfile: ProfileProduction, EnvAllowDev: "perhaps"})}, EnvAllowDev)
}

func TestProfileDefaultsAreOverridable(t *testing.T) {
	r := Catalog()
	e := map[string]string{EnvProfile: "Development"} // case-insensitive
	s := mustResolve(t, r, Inputs{AuthEnabled: true, LookupEnv: env(e)})
	if !s.On(Demo) || !s.On(Developer) || s.Source(Demo) != SourceProfile || s.Profile() != ProfileDevelopment {
		t.Error("development profile defaults not applied")
	}
	// explicit setting beats the profile
	e["SEAM_ENABLE_DEVELOPER"] = "0"
	s = mustResolve(t, r, Inputs{AuthEnabled: true, LookupEnv: env(e)})
	if s.On(Developer) || s.Source(Developer) != SourceEnv {
		t.Error("env must beat the profile")
	}
	// the development profile enables developer, which needs auth: the
	// error should say so rather than failing obscurely
	mustFail(t, r, Inputs{AuthEnabled: false, LookupEnv: env(map[string]string{EnvProfile: ProfileDevelopment})}, "authentication")
}

// ---- Set behaviour ----

func TestOnPanicsOnUnknownKey(t *testing.T) {
	s := mustResolve(t, Catalog(), Inputs{LookupEnv: env(nil)})
	defer func() {
		if recover() == nil {
			t.Error("On(unknown) must panic")
		}
	}()
	s.On("typo")
}

func TestRuntimeSemantics(t *testing.T) {
	r := Catalog()
	yes, no := true, false

	// Developer off: every runtime flag is inert, whatever the panel says.
	off := mustResolve(t, r, Inputs{LookupEnv: env(nil)})
	if v, src := off.Runtime(HotloadI18N, &yes); v || src != SourceDefault {
		t.Errorf("inert hotload: got %v/%v, want false/default", v, src)
	}
	// ...and a default-true layer toggle stays at its default, so stale
	// panel state cannot disable overrides in a deployment with the
	// Developer module off.
	if v, _ := off.Runtime(OverlayXolu, &no); !v {
		t.Error("overlay_xolu must stay true while developer is off")
	}

	on := mustResolve(t, r, Inputs{AuthEnabled: true, LookupEnv: env(map[string]string{"SEAM_ENABLE_DEVELOPER": "1"})})
	if v, src := on.Runtime(HotloadI18N, nil); v || src != SourceDefault {
		t.Errorf("no panel value: got %v/%v, want false/default", v, src)
	}
	if v, src := on.Runtime(HotloadI18N, &yes); !v || src != SourcePanel {
		t.Errorf("panel value: got %v/%v, want true/panel", v, src)
	}
	if v, src := on.Runtime(OverlayXolu, &no); v || src != SourcePanel {
		t.Errorf("panel value: got %v/%v, want false/panel", v, src)
	}
	if on.Pinned(HotloadI18N) {
		t.Error("an unpinned flag reported pinned")
	}

	// Pinned by list: panel cannot override.
	pin := mustResolve(t, r, Inputs{AuthEnabled: true, LookupEnv: env(map[string]string{
		EnvList: "developer,-" + OverlayXolu,
	})})
	if !pin.Pinned(OverlayXolu) {
		t.Fatal("expected overlay_xolu to be pinned")
	}
	if v, src := pin.Runtime(OverlayXolu, &yes); v || src != SourceList {
		t.Errorf("pinned: got %v/%v, want false/list", v, src)
	}
}

func TestPinnedIsOnlyForRuntimeFlags(t *testing.T) {
	r := Catalog()
	s := mustResolve(t, r, Inputs{LookupEnv: env(map[string]string{"SEAM_ENABLE_DEMO": "1"})})
	if s.Pinned(Demo) {
		t.Error("static flags are never 'pinned'")
	}
}

func TestRuntimePanicsOnStaticFlag(t *testing.T) {
	s := mustResolve(t, Catalog(), Inputs{LookupEnv: env(nil)})
	defer func() {
		if recover() == nil {
			t.Error("Runtime(static flag) must panic")
		}
	}()
	s.Runtime(Demo, nil)
}

func TestStatesInDeclarationOrder(t *testing.T) {
	r := Catalog()
	s := mustResolve(t, r, Inputs{LookupEnv: env(nil)})
	st := s.States()
	fl := r.Flags()
	if len(st) != len(fl) {
		t.Fatalf("%d states for %d flags", len(st), len(fl))
	}
	for i := range fl {
		if st[i].Flag.Key != fl[i].Key {
			t.Errorf("state %d is %q, want %q", i, st[i].Flag.Key, fl[i].Key)
		}
	}
}

func TestLog(t *testing.T) {
	var buf bytes.Buffer
	l := slog.New(slog.NewTextHandler(&buf, nil))
	r := Catalog()
	s := mustResolve(t, r, Inputs{AuthEnabled: true, LookupEnv: env(map[string]string{
		"SEAM_ENABLE_DEVELOPER": "1", "SEAM_ENABLE_ADMIN": "0",
	})})
	s.Log(l)
	out := buf.String()
	for _, want := range []string{"feature flags resolved", "developer=true(env)", "admin=false(env)", "development-class", "flag=developer"} {
		if !strings.Contains(out, want) {
			t.Errorf("log output missing %q:\n%s", want, out)
		}
	}
	// A default-true dev-class flag (overlay_*) must not trigger the warning.
	if strings.Contains(out, "flag=overlay_xolu") {
		t.Errorf("default-on flag warned about:\n%s", out)
	}
}

// ---- Catalogue regression: names and defaults that already exist ----

func TestCatalogIsValid(t *testing.T) {
	_ = Catalog() // panics if inconsistent
}

// These are the environment variables and defaults config.go has always
// had. Changing any of them silently breaks deployments, so this table is
// deliberately explicit rather than derived from the catalogue.
func TestCatalogPreservesExistingFlags(t *testing.T) {
	want := []struct {
		key, env, cfg string
		def           bool
	}{
		{UI, "SEAM_ENABLE_UI", "enable_ui", true},
		{API, "SEAM_ENABLE_API", "enable_api", true},
		{Webhooks, "", "enable_webhooks", true},
		{Rules, "", "enable_rules", true},
		{FSM, "", "enable_fsm", true},
		{QR, "", "enable_qr", true},
		{Operational, "SEAM_ENABLE_OPERATIONAL", "enable_operational", true},
		{Admin, "SEAM_ENABLE_ADMIN", "enable_admin", true},
		{WorkOrders, "SEAM_ENABLE_WORK_ORDERS", "enable_work_orders", false},
		{Issues, "SEAM_ENABLE_ISSUES", "enable_issues", false},
		{Demo, "SEAM_ENABLE_DEMO", "enable_demo", false},
		{DevMode, "", "dev_mode", false},
		{VendorJS, "", "vendor_js", false},
	}
	r := Catalog()
	for _, w := range want {
		f, ok := r.Lookup(w.key)
		if !ok {
			t.Errorf("flag %q missing", w.key)
			continue
		}
		if f.EnvVar != w.env || f.ConfigKey != w.cfg || f.Default != w.def {
			t.Errorf("%s: env=%q cfg=%q default=%v; want env=%q cfg=%q default=%v",
				w.key, f.EnvVar, f.ConfigKey, f.Default, w.env, w.cfg, w.def)
		}
		if f.Mode != ModeStatic {
			t.Errorf("%s must stay static", w.key)
		}
	}
}

func TestCatalogEveryFlagHasADescription(t *testing.T) {
	for _, f := range Catalog().Flags() {
		if strings.TrimSpace(f.Description) == "" {
			t.Errorf("flag %q has no description", f.Key)
		}
		if f.EnvVar != "" && !strings.HasPrefix(f.EnvVar, "SEAM_") {
			t.Errorf("flag %q: env var %q lacks the SEAM_ prefix", f.Key, f.EnvVar)
		}
	}
}

func TestCatalogRuntimeFlagsAreGatedByDeveloper(t *testing.T) {
	for _, f := range Catalog().Flags() {
		if f.Mode != ModeRuntime {
			continue
		}
		if len(f.Requires) != 1 || f.Requires[0] != Developer {
			t.Errorf("runtime flag %q must require %q, has %v", f.Key, Developer, f.Requires)
		}
	}
}

func TestDefaultSetMatchesOldDefaults(t *testing.T) {
	s := mustResolve(t, Catalog(), Inputs{LookupEnv: env(nil)})
	on := []string{UI, API, Webhooks, Rules, FSM, QR, Operational, Admin}
	off := []string{WorkOrders, Issues, Demo, DevMode, VendorJS, Developer}
	for _, k := range on {
		if !s.On(k) {
			t.Errorf("%s should default on", k)
		}
	}
	for _, k := range off {
		if s.On(k) {
			t.Errorf("%s should default off", k)
		}
	}
}
