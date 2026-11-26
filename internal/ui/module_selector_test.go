package ui

import (
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"

	"github.com/ha1tch/seam-ui/internal/i18n"
	"github.com/ha1tch/seam-ui/internal/modules"
)

func renderSelectorToString(visible []modules.Module) string {
	t := func(key string, args ...any) string { return key }
	return mi.RenderToString(ModuleSelector(visible, "", t))
}

func TestModuleSelector_Empty_RendersNothing(t *testing.T) {
	out := renderSelectorToString(nil)
	if strings.Contains(out, "seam-module-btn") {
		t.Errorf("an empty visible list should render no selector at all, got: %s", out)
	}
}

func TestModuleSelector_GroupsAppsUnderHeader(t *testing.T) {
	visible := []modules.Module{
		{ID: "assets", Group: "apps", Icon: "inventory", Label: "nav.module.assets", Path: "/"},
		{ID: "documents", Group: "apps", Icon: "description", Label: "nav.module.documents", Path: "/documents"},
	}
	out := renderSelectorToString(visible)
	if !strings.Contains(out, "nav.module_group.apps") {
		t.Error("expected the Apps group header to render")
	}
	if !strings.Contains(out, `href="/"`) {
		t.Error("expected Assets' href to be \"/\", its actual Path")
	}
	if !strings.Contains(out, `href="/documents"`) {
		t.Error("expected Documents' href to be \"/documents\"")
	}
	if !strings.Contains(out, "nav.module.assets") || !strings.Contains(out, "nav.module.documents") {
		t.Error("expected both module labels to render")
	}
}

func TestModuleSelector_SystemHasNoGroupHeader(t *testing.T) {
	// §4.1: System is one flat entry, not a group header with children.
	visible := []modules.Module{
		{ID: "system", Group: "system", Icon: "settings", Label: "nav.module.system", Path: "/system"},
	}
	out := renderSelectorToString(visible)
	if strings.Contains(out, "seam-mod-title") || strings.Contains(out, "seam-mod-grid") {
		t.Error("no title and no tiles should render when nothing is in the apps group")
	}
	if !strings.Contains(out, `href="/system"`) {
		t.Error("expected System's own href to render directly, with no group wrapper")
	}
}

func TestModuleSelector_MixedGroups(t *testing.T) {
	visible := []modules.Module{
		{ID: "system", Group: "system", Icon: "settings", Label: "nav.module.system", Path: "/system"},
		{ID: "assets", Group: "apps", Icon: "inventory", Label: "nav.module.assets", Path: "/"},
	}
	out := renderSelectorToString(visible)
	if !strings.Contains(out, "nav.module_group.apps") {
		t.Error("expected the Apps group header when at least one apps-group module is visible")
	}
	// Both must still render regardless of registration order (system
	// was registered first here, apps second).
	if !strings.Contains(out, `href="/system"`) || !strings.Contains(out, `href="/"`) {
		t.Error("expected both System and Assets to render regardless of registration order")
	}
}

func TestModuleSelector_UnrecognisedGroup_NotRendered(t *testing.T) {
	visible := []modules.Module{
		{ID: "mystery", Group: "not-a-real-group", Icon: "help", Label: "nav.module.mystery", Path: "/mystery"},
	}
	out := renderSelectorToString(visible)
	// The selector itself still renders (visible has one entry), but the
	// mystery module's own item must not appear anywhere -- an
	// unrecognised Group is a configuration mistake, not something to
	// guess a rendering location for.
	if strings.Contains(out, "/mystery") {
		t.Error("a module with an unrecognised Group value must not render its own item")
	}
}

// ============================================================================
// Adversarial tests below. ModuleSelector's own doc comment says it "does
// no access checking of its own, it only groups and renders what it's
// given" -- these probe exactly that trust boundary: what happens when
// the caller (which in production is always Registry.VisibleTo's output,
// but the function itself accepts any []modules.Module) hands it
// something a real Registry would never produce.
// ============================================================================

// TestModuleSelector_LabelAndPathAreHTMLEscaped confirms minty's own
// baseline escaping (Element.Render/TextNode.Render, both call
// html.EscapeString -- confirmed directly from minty's source as part
// of the same 2026-08-04 audit that found T-63/T-64) actually protects
// this render path: moduleSelectorItem builds mi.Href(m.Path) as an
// attribute and t(m.Label) as text content, neither via mi.Raw, so both
// should come out escaped. Every real Module today is a compile-time
// literal in server.go, so this isn't reachable with attacker data in
// practice -- but proving the escaping actually happens here, not just
// assuming it because it's "probably fine like everything else," is
// exactly the discipline that caught the Map and FSM pages' real bugs.
func TestModuleSelector_LabelAndPathAreHTMLEscaped(t *testing.T) {
	visible := []modules.Module{
		{ID: "evil", Group: "apps", Icon: "help", Label: `<script>alert(1)</script>`, Path: `/x" onclick="alert(1)`},
	}
	out := renderSelectorToString(visible)
	if strings.Contains(out, "<script>alert(1)</script>") {
		t.Error("Label rendered as live, unescaped HTML instead of escaped text")
	}
	if strings.Contains(out, `onclick="alert(1)`) {
		t.Error("Path broke out of the href attribute into a live onclick handler")
	}
}

// TestModuleSelector_CallerSuppliedDuplicates_BothRender documents a
// real trust-boundary fact: Registry.Register's own dedup (panicking on
// a repeated ID) only protects callers that go through the registry.
// ModuleSelector itself has no dedup of its own -- if some future
// caller ever builds a []modules.Module by hand (bypassing Registry
// entirely, e.g. for a preview/admin tool), duplicate IDs render as
// duplicate items with no warning. Not asserting this is wrong (it may
// never matter, since every real caller today is Registry.VisibleTo's
// output), just that it's the current, real contract.
func TestModuleSelector_CallerSuppliedDuplicates_BothRender(t *testing.T) {
	visible := []modules.Module{
		{ID: "assets", Group: "apps", Icon: "inventory", Label: "nav.module.assets", Path: "/"},
		{ID: "assets", Group: "apps", Icon: "inventory", Label: "nav.module.assets", Path: "/"},
	}
	out := renderSelectorToString(visible)
	if strings.Count(out, `href="/"`) != 2 {
		t.Errorf(`expected 2 occurrences of href="/" (ModuleSelector doesn't dedupe its input), got %d`, strings.Count(out, `href="/"`))
	}
}

// TestModuleSelector_MultipleSystemGroupModules_AllRenderFlat is the
// same "current, real contract" documentation for the system group
// specifically -- §4.1's design intent is one flat System entry, but
// nothing in the code enforces "at most one system-group module."
// Two would both render flat, with no visual distinction between them
// (no group header either way for "system"), which could look like a
// single broken/duplicated entry to a user, not an error to anyone
// registering modules.
func TestModuleSelector_MultipleSystemGroupModules_AllRenderFlat(t *testing.T) {
	visible := []modules.Module{
		{ID: "system", Group: "system", Icon: "settings", Label: "nav.module.system", Path: "/system"},
		{ID: "system2", Group: "system", Icon: "settings", Label: "nav.module.system2", Path: "/system2"},
	}
	out := renderSelectorToString(visible)
	if !strings.Contains(out, `href="/system"`) || !strings.Contains(out, `href="/system2"`) {
		t.Error("expected both system-group modules to render")
	}
	if strings.Contains(out, "seam-mod-title") || strings.Contains(out, "seam-mod-grid") {
		t.Error("no title and no tiles should render when nothing is in the apps group")
	}
}

// TestModuleSelector_EmptyPath_RendersHrefEmptyString documents rather
// than guesses: an empty Path produces href="" (a same-page self-link
// in most browsers), not a panic and not a dropped item. A genuine
// Module misconfiguration (Path never set) degrades to a confusing but
// harmless link, not a broken page.
func TestModuleSelector_EmptyPath_RendersHrefEmptyString(t *testing.T) {
	visible := []modules.Module{
		{ID: "half-configured", Group: "apps", Icon: "help", Label: "nav.module.mystery", Path: ""},
	}
	out := renderSelectorToString(visible)
	if !strings.Contains(out, `href=""`) {
		t.Errorf(`expected href="" for an empty Path, got:\n%s`, out)
	}
}

// TestModuleSelector_HideFromSelector_SuppressesEntryOnly guards the fix
// for a real UX issue we flagged 2026-08-04: layout.go's user-
// avatar dropdown already links straight to /profile, right next to
// where the module selector itself renders -- Account showing up a
// second time here would be redundant, not helpful. Confirms
// HideFromSelector suppresses only the rendered <a> item, not the
// module's presence in the input slice's other properties (this
// function never mutates visible, so that's really just documenting
// intent, but worth asserting directly rather than assuming).
func TestModuleSelector_HideFromSelector_SuppressesEntryOnly(t *testing.T) {
	visible := []modules.Module{
		{ID: "account", Group: "system", Icon: "person", Label: "nav.module.account", Path: "/profile", HideFromSelector: true},
		{ID: "settings", Group: "system", Icon: "settings", Label: "nav.module.settings", Path: "/settings"},
	}
	out := renderSelectorToString(visible)
	if strings.Contains(out, `href="/profile"`) {
		t.Error("Account rendered a selector entry despite HideFromSelector -- the flag isn't being respected")
	}
	if !strings.Contains(out, `href="/settings"`) {
		t.Error("Settings (HideFromSelector: false) should still render normally alongside a hidden sibling")
	}
}

// TestModuleSelector_AllVisibleHidden_RendersNothing confirms the
// post-filter empty case specifically -- distinct from the pre-existing
// "visible has zero entries at all" check, since visible here is
// non-empty going in.
func TestModuleSelector_AllVisibleHidden_RendersNothing(t *testing.T) {
	visible := []modules.Module{
		{ID: "account", Group: "system", Icon: "person", Label: "nav.module.account", Path: "/profile", HideFromSelector: true},
	}
	out := renderSelectorToString(visible)
	if strings.Contains(out, "seam-module-btn") {
		t.Errorf("expected no selector button at all when every visible module is hidden, got:\n%s", out)
	}
}

// The panel: application modules as tiles under the title "Modules", each
// with its icon, name and a line about it; system modules as links under them;
// the module the page belongs to marked as the current one.
func TestModuleSelector_TilesLinksAndTheCurrentModule(t *testing.T) {
	visible := []modules.Module{
		{ID: "assets", Group: "apps", Icon: "inventory_2", Label: "nav.module.assets", Path: "/"},
		{ID: "issues", Group: "apps", Icon: "bug_report", Label: "nav.module.issues", Path: "/issues"},
		{ID: "bare", Group: "apps", Icon: "star", Label: "nav.module.nothing_says_what_it_is", Path: "/bare"},
		{ID: "settings", Group: "system", Icon: "settings", Label: "nav.module.settings", Path: "/settings"},
	}
	tr, err := i18n.New("en")
	if err != nil {
		t.Fatal(err)
	}
	out := mi.RenderToString(ModuleSelector(visible, "issues", tr.ForLocale("en")))
	for _, want := range []string{
		`id="seam-module-btn"`, `aria-expanded="false"`, `aria-controls="seam-module-menu"`, `onclick="SeamModuleSelector.toggle()"`,
		`<div class="seam-mod-title">Modules</div>`, `<span class="seam-mod-btn-label">Modules</span>`,
		`<span class="seam-mod-name">Assets</span>`, `<span class="seam-mod-desc">Track every asset, from location to live status</span>`,
		`>inventory_2</span>`, `>bug_report</span>`,
		`aria-current="page" class="seam-mod-tile is-current" href="/issues"`,
		`class="seam-mod-link" href="/settings"`, `class="seam-mod-foot"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the panel lacks %s", want)
		}
	}
	if n := strings.Count(out, "is-current"); n != 1 {
		t.Errorf("%d entries marked as current, want 1", n)
	}
	// a module with no line about it shows its name alone, never a key
	if n := strings.Count(out, "seam-mod-desc"); n != 2 || strings.Contains(out, ".desc") {
		t.Errorf("%d description lines, want 2 (Assets, Issues) and no untranslated key", n)
	}
	if strings.Contains(out, "Apps") || strings.Contains(out, "Applications") {
		t.Errorf("the menu is called Modules")
	}
	// two columns for several applications, one narrow column for a single one
	if strings.Contains(out, "seam-mod-panel--narrow") {
		t.Errorf("three applications take the wide panel")
	}
	if one := mi.RenderToString(ModuleSelector(visible[1:2], "", tr.ForLocale("en"))); !strings.Contains(one, "seam-mod-panel--narrow") {
		t.Errorf("a single application takes the narrow panel")
	}
	// a system module can be the current one too
	if sys := mi.RenderToString(ModuleSelector(visible, "settings", tr.ForLocale("en"))); !strings.Contains(sys, `class="seam-mod-link is-current"`) || strings.Contains(sys, "seam-mod-tile is-current") {
		t.Errorf("Settings should be the one marked")
	}
}

// Every registered application module has its line in every language, and the
// menu has one name in all of them.
func TestModuleSelector_TextsInEveryLanguage(t *testing.T) {
	for _, locale := range []string{"en", "es", "pt", "ja"} {
		tr, err := i18n.New(locale)
		if err != nil {
			t.Fatal(err)
		}
		tl := tr.ForLocale(locale)
		for _, id := range []string{"assets", "documents", "workorders", "issues", "accounting", "demo", "developer"} {
			if key := "nav.module." + id + ".desc"; tl(key) == key || tl(key) == "" {
				t.Errorf("%s: no text for %s", locale, key)
			}
		}
		if tl("nav.module_group.apps") == "nav.module_group.apps" {
			t.Errorf("%s: the menu has no name", locale)
		}
	}
}

// The module a page belongs to, for the mark in the menu.
func TestActiveModuleID(t *testing.T) {
	cfg := Config{VisibleModules: []modules.Module{
		{ID: "assets", Path: "/"},
		{ID: "issues", Path: "/issues", SidebarItems: []modules.SidebarItem{{Label: "x", URL: "/issue-templates"}}},
		{ID: "demo", Path: "/demo"},
	}}
	for path, want := range map[string]string{"/": "assets", "/assets/3": "assets", "/issues/42": "issues", "/issue-templates": "issues", "/demo/rows": "demo"} {
		if got := activeModuleID(path, cfg); got != want {
			t.Errorf("activeModuleID(%q) = %q, want %q", path, got, want)
		}
	}
	if got := activeModuleID("/anything", Config{}); got != "" {
		t.Errorf("no modules: %q, want none", got)
	}
}
