package ui

import (
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"

	"github.com/ha1tch/seam-ui/internal/devstate"
	"github.com/ha1tch/seam-ui/internal/features"
	"github.com/ha1tch/seam-ui/internal/i18n"
	"github.com/ha1tch/seam-ui/internal/schemaoverlay"
)

func runtimeFlags(t *testing.T) []features.Flag {
	t.Helper()
	var out []features.Flag
	for _, f := range features.Catalog().Flags() {
		if f.Mode == features.ModeRuntime {
			out = append(out, f)
		}
	}
	if len(out) == 0 {
		t.Fatal("the catalogue has no runtime flags")
	}
	return out
}

func overviewHTML(data DeveloperPageData) string {
	return mi.RenderToString(DeveloperOverviewPage(data, nil))
}
func startupHTML(data DeveloperPageData) string {
	return mi.RenderToString(DeveloperStartupPage(data, nil))
}
func translationsHTML(data TranslationsPageData) string {
	return mi.RenderToString(DeveloperTranslationsPage(data, nil))
}
func formsHTML(data FormsPageData) string { return mi.RenderToString(DeveloperFormsPage(data, nil)) }

func stubForm(b *mi.Builder) mi.Node { return b.Div(mi.Class("stub-form"), "FORM") }

// ---- Overview ---------------------------------------------------------------

func TestOverview_OneCardPerRuntimeSwitchOnAGrid(t *testing.T) {
	var views []devstate.FlagView
	for _, f := range runtimeFlags(t) {
		views = append(views, devstate.FlagView{Flag: f})
	}
	html := overviewHTML(DeveloperPageData{Runtime: views})
	if !strings.Contains(html, `class="dev-grid"`) {
		t.Errorf("the switches must sit on a grid:\n%s", html)
	}
	for _, f := range runtimeFlags(t) {
		if !strings.Contains(html, f.Key) {
			t.Errorf("no card for %s", f.Key)
		}
	}
	if got := strings.Count(html, `value="on"`); got != len(views) {
		t.Errorf("want %d Enable buttons, got %d", len(views), got)
	}
	if got := strings.Count(html, `value="off"`); got != len(views) {
		t.Errorf("want %d Disable buttons, got %d", len(views), got)
	}
	// each switch returns to the Overview, and a switch with no translation
	// keys still shows the catalogue text, never a bare key
	if got := strings.Count(html, `value="overview"`); got != len(views) {
		t.Errorf("every switch form must say where to return to, got %d", got)
	}
	if !strings.Contains(html, runtimeFlags(t)[0].Description) {
		t.Errorf("a switch with no translation keys must fall back to the catalogue description")
	}
}

func TestOverview_ActiveStateIsMarked(t *testing.T) {
	f := runtimeFlags(t)[0]
	on := overviewHTML(DeveloperPageData{Runtime: []devstate.FlagView{{Flag: f, Value: true}}})
	if !strings.Contains(on, "is-active-on") || strings.Contains(on, "is-active-off") {
		t.Errorf("an enabled switch highlights Enable only")
	}
	off := overviewHTML(DeveloperPageData{Runtime: []devstate.FlagView{{Flag: f, Value: false}}})
	if !strings.Contains(off, "is-active-off") || strings.Contains(off, "is-active-on") {
		t.Errorf("a disabled switch highlights Disable only")
	}
	if !strings.Contains(on, `aria-pressed="true"`) || !strings.Contains(on, `aria-pressed="false"`) {
		t.Errorf("the pair must expose its state to assistive technology")
	}
}

func TestOverview_LockedSwitchesHaveNoControls(t *testing.T) {
	f := runtimeFlags(t)[0]
	for name, fv := range map[string]devstate.FlagView{
		"pinned":   {Flag: f, Pinned: true, Locked: true},
		"inactive": {Flag: f, Inactive: true, Locked: true},
	} {
		html := overviewHTML(DeveloperPageData{Runtime: []devstate.FlagView{fv}})
		if strings.Contains(html, `name="action"`) || strings.Contains(html, "/developer/flags") {
			t.Errorf("%s: a locked switch must not offer controls", name)
		}
		if !strings.Contains(html, "developer.runtime.locked_"+name) {
			t.Errorf("%s: missing the reason", name)
		}
	}
}

func TestOverview_ResetOnlyWhenSetFromThePanel(t *testing.T) {
	f := runtimeFlags(t)[0]
	if !strings.Contains(overviewHTML(DeveloperPageData{Runtime: []devstate.FlagView{{Flag: f, Source: features.SourcePanel}}}), `value="reset"`) {
		t.Errorf("a panel-set switch offers Reset")
	}
	if strings.Contains(overviewHTML(DeveloperPageData{Runtime: []devstate.FlagView{{Flag: f}}}), `value="reset"`) {
		t.Errorf("a switch still at its default has nothing to reset")
	}
}

func TestOverview_LinksToEverySection(t *testing.T) {
	html := overviewHTML(DeveloperPageData{})
	for _, href := range []string{"/developer/translations", "/developer/forms", "/developer/startup"} {
		if !strings.Contains(html, `href="`+href+`"`) {
			t.Errorf("the Overview must link to %s", href)
		}
	}
	if strings.Contains(html, "<table") || strings.Contains(html, "SEAM_ENABLE_") {
		t.Errorf("the Overview is for switches and links, not for the startup table")
	}
}

// ---- Startup ----------------------------------------------------------------

func TestStartup_TableLeavesTheRuntimeSwitchesOut(t *testing.T) {
	var states []features.State
	for _, f := range features.Catalog().Flags() {
		states = append(states, features.State{Flag: f, Value: f.Default})
	}
	html := startupHTML(DeveloperPageData{Flags: states})
	if !strings.Contains(html, "SEAM_ENABLE_DEVELOPER") || !strings.Contains(html, "<table") {
		t.Errorf("the startup flags belong in a table:\n%s", html)
	}
	for _, f := range runtimeFlags(t) {
		if strings.Contains(html, f.Key) {
			t.Errorf("runtime flag %s lives on the Overview and must not be listed as a startup flag", f.Key)
		}
	}
}

// ---- Translations -----------------------------------------------------------

func TestTranslations_LayerCardAndHint(t *testing.T) {
	none := translationsHTML(TranslationsPageData{Page: 1, Pages: 1})
	if strings.Contains(none, "/developer/reload") || !strings.Contains(none, "developer.layers.hint") {
		t.Errorf("an unconfigured layer explains how to turn it on and has nothing to reload:\n%s", none)
	}
	with := translationsHTML(TranslationsPageData{DeveloperPageData: DeveloperPageData{Locales: &i18n.FileStatus{Dir: "/srv/locales"}}, Page: 1, Pages: 1})
	if !strings.Contains(with, `value="locales"`) || !strings.Contains(with, `value="translations"`) {
		t.Errorf("reloading returns to the Translations page:\n%s", with)
	}
}

// Adversarial: directory names and load errors come from the filesystem.
func TestTranslations_HostileLayerTextIsEscaped(t *testing.T) {
	html := translationsHTML(TranslationsPageData{DeveloperPageData: DeveloperPageData{Locales: &i18n.FileStatus{
		Dir: `"><img src=x onerror=alert(1)>`, LastError: `<script>alert(1)</script>`}}, Page: 1, Pages: 1})
	for _, raw := range []string{"<script>alert(1)", `<img src=x`} {
		if strings.Contains(html, raw) {
			t.Errorf("unescaped %q in the page", raw)
		}
	}
}

func TestTranslations_CompletenessTable(t *testing.T) {
	rep := &i18n.Report{Ref: "en", RefKeys: 3, Diffs: []i18n.LocaleDiff{{Locale: "es", Missing: []string{"a", "b"}}, {Locale: "ja"}}}
	html := translationsHTML(TranslationsPageData{DeveloperPageData: DeveloperPageData{Completeness: rep}, Page: 1, Pages: 1})
	if strings.Count(html, "dev-num-warn") != 1 {
		t.Errorf("only the non-zero cell is highlighted:\n%s", html)
	}
	if !strings.Contains(html, "developer.completeness.note") {
		t.Errorf("the reference locale must be named")
	}
}

func TestTranslations_TextListAndPager(t *testing.T) {
	data := TranslationsPageData{Locale: "es", Query: "a&b", Page: 2, Pages: 3, Total: 120,
		Rows: []translationRow{{Key: "k.one", Text: "<b>one</b>", Layer: "embedded"}, {Key: "k.two", Text: "two", Layer: "filesystem"}}}
	html := translationsHTML(data)
	if strings.Contains(html, "<b>one</b>") {
		t.Errorf("a text is data, never markup")
	}
	// page 1 is the default, so "Previous" from page 2 carries no page parameter
	if !strings.Contains(html, `href="/developer/translations?locale=es&amp;q=a%26b#texts"`) || !strings.Contains(html, "page=3") {
		t.Errorf("the pager keeps the filter and links both ways:\n%s", html)
	}
	if !strings.Contains(html, "developer.layer.filesystem") || !strings.Contains(html, "key=k.two") {
		t.Errorf("each row shows its source and links to its editor")
	}
	if strings.Contains(translationsHTML(TranslationsPageData{Page: 1, Pages: 1}), "dev-pager") {
		t.Errorf("an empty list has no pager")
	}
}

func TestTranslations_EditorOnlyOffersWhatCanBeDone(t *testing.T) {
	base := TranslationsPageData{Locale: "es", Page: 1, Pages: 1,
		Editor: &translationEditor{Key: "k.one", Reference: "Hello", Form: stubForm}}
	ro := translationsHTML(base)
	if strings.Contains(ro, "/developer/translations/save") || !strings.Contains(ro, "developer.translations.not_writable") {
		t.Errorf("without a translations directory the editor explains and offers no form:\n%s", ro)
	}
	base.Writable = true
	rw := translationsHTML(base)
	if !strings.Contains(rw, "/developer/translations/save") || strings.Contains(rw, "/developer/translations/remove") {
		t.Errorf("a text with no override can be saved but not removed:\n%s", rw)
	}
	base.Editor.HasOverride = true
	ov := translationsHTML(base)
	if !strings.Contains(ov, "/developer/translations/remove") {
		t.Errorf("an overridden text can be removed")
	}
	for _, name := range []string{`name="locale"`, `name="key"`, `name="q"`, `name="page"`} {
		if !strings.Contains(ov, name) {
			t.Errorf("the editor must carry %s so the POST knows what it edits", name)
		}
	}
}

// ---- Form definitions -------------------------------------------------------

func TestForms_ListShowsSourceAndFlagsUnusedOverrides(t *testing.T) {
	data := FormsPageData{Selected: "system/user_edit", Rows: []formsRow{
		{Ref: developerSchemaRef{"system", "user_edit"}, Layer: "filesystem"},
		{Ref: developerSchemaRef{"system", "user_creat"}, Layer: "filesystem", Unused: true},
	}}
	html := formsHTML(data)
	if !strings.Contains(html, "data-dev-selected") || !strings.Contains(html, "schema=system%2Fuser_edit") {
		t.Errorf("the list marks the open form and links to each:\n%s", html)
	}
	if strings.Count(html, ">developer.forms.custom<") != 1 {
		t.Errorf("only the override that no page uses is flagged")
	}
	if strings.Contains(formsHTML(FormsPageData{}), "<table") {
		t.Errorf("with no forms there is no table")
	}
}

func TestForms_EditorButtonsAndGuards(t *testing.T) {
	ref := developerSchemaRef{"system", "user_edit"}
	data := FormsPageData{Editor: &formsEditor{Ref: ref, Form: stubForm}}
	ro := formsHTML(data)
	if strings.Contains(ro, "/developer/forms/save") || !strings.Contains(ro, "developer.forms.not_writable") {
		t.Errorf("without a form-definitions directory the editor offers no form")
	}
	data.Writable = true
	rw := formsHTML(data)
	if !strings.Contains(rw, `formaction="/developer/forms/preview"`) || !strings.Contains(rw, `action="/developer/forms/save"`) {
		t.Errorf("Preview and Save are two buttons of one form:\n%s", rw)
	}
	if strings.Contains(rw, "/developer/forms/remove") {
		t.Errorf("a form with no override cannot be removed")
	}
	data.Editor.HasOverride = true
	if !strings.Contains(formsHTML(data), "/developer/forms/remove") {
		t.Errorf("an overridden form can be removed")
	}
}

// Adversarial: the validator's message and the engine's failure text are data.
func TestForms_ProblemAndPreviewFailureAreEscaped(t *testing.T) {
	data := FormsPageData{Writable: true, Editor: &formsEditor{Ref: developerSchemaRef{"system", "a"}, Form: stubForm,
		Problem: `<script>alert(1)</script>`, PreviewErr: `<img src=x onerror=alert(1)>`}}
	html := formsHTML(data)
	for _, raw := range []string{"<script>alert(1)", "<img src=x"} {
		if strings.Contains(html, raw) {
			t.Errorf("unescaped %q", raw)
		}
	}
	if !strings.Contains(html, "developer.forms.problem") {
		t.Errorf("a refused definition shows a Problem heading")
	}
}

func TestForms_PreviewAndNotesAreShownWhenThereAreSome(t *testing.T) {
	data := FormsPageData{Writable: true, Editor: &formsEditor{Ref: developerSchemaRef{"system", "a"}, Form: stubForm,
		Previewed: true, PreviewHTML: `<input name="alpha">`, Warnings: []string{"property beta was removed"}}}
	html := formsHTML(data)
	if !strings.Contains(html, `<input name="alpha">`) || !strings.Contains(html, "dev-preview") {
		t.Errorf("the engine's own preview is included as rendered")
	}
	if !strings.Contains(html, "property beta was removed") || !strings.Contains(html, "developer.forms.valid") {
		t.Errorf("a valid definition says so and lists the compatibility notes")
	}
	_ = schemaoverlay.LayerEmbedded
}
