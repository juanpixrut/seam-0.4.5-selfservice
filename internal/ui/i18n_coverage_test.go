package ui

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	mi "github.com/ha1tch/minty"

	"github.com/ha1tch/seam-ui/internal/auth"
	fe "github.com/ha1tch/seam-ui/internal/formengine"
	"github.com/ha1tch/seam-ui/internal/i18n"
)

// formRenderNames is the module and form each embedded definition is rendered
// with, where the file does not say it itself (x-seam-form) and its directory
// and name are not it.
var formRenderNames = map[string][2]string{
	"crew/create": {"crew", "crew"}, "issues/create": {"issues", "issue"}, "issues/assign": {"issues", "assign"},
	"system/user_create": {"system", "user"}, "system/user_edit": {"system", "user"},
	"consumable/create": {"consumable", "consumable"}, "consumable/edit": {"consumable", "consumable"}, "movement/create": {"movement", "movement"},
	"unit/create": {"unit", "unit"}, "unit/edit": {"unit", "unit"}, "warehouse/create": {"warehouse", "warehouse"}, "warehouse/edit": {"warehouse", "warehouse"},
	"wo_action/create": {"wo-actions", "wo-action"}, "wo_action/edit": {"wo-actions", "wo-action"},
}

// Every label, option, button and help text of every application form comes
// out translated: rendered in Japanese, none is left without a Japanese
// character. What is allowed to stay: acronyms and identifiers that are the
// same in every language, and the Demo module's forms, a developer's tool
// kept in English (T-135).
func TestForms_NoTextIsLeftUntranslated(t *testing.T) {
	tr, err := i18n.New("ja")
	if err != nil {
		t.Fatal(err)
	}
	same := map[string]bool{"GIAI": true, "GRAI": true, "JSON *": true, "JSON": true, "UTC": true, "URL": true}
	zone := regexp.MustCompile(`^[A-Z][A-Za-z_]+/[A-Za-z_]+$`) // time zone identifiers (T-58)
	files, _ := filepath.Glob("../formengine/schemas/*/*.json")
	if len(files) < 30 {
		t.Fatalf("only %d form definitions found", len(files))
	}
	sort.Strings(files)
	tag := regexp.MustCompile(`<(label|legend|th|button|option|span class="fe-cell-label"|p class="[^"]*fe-help[^"]*"|small)[^>]*>([^<]{2,})<`)
	ja := regexp.MustCompile(`[\x{3040}-\x{30ff}\x{4e00}-\x{9fff}]`)
	word := regexp.MustCompile(`[A-Za-zÁÉÍÓÚáéíóúñ]{3,}`)
	for _, f := range files {
		dir, stem := filepath.Base(filepath.Dir(f)), strings.TrimSuffix(filepath.Base(f), ".json")
		if dir == "demo" {
			continue
		}
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var head struct {
			Form struct{ Module, Name string } `json:"x-seam-form"`
		}
		_ = json.Unmarshal(raw, &head)
		mod, form := dir, stem
		if o, ok := formRenderNames[dir+"/"+stem]; ok {
			mod, form = o[0], o[1]
		}
		if head.Form.Module != "" {
			mod, form = head.Form.Module, head.Form.Name
		}
		eng := &fe.FormEngine{Translator: tr, Sources: fe.StaticSourceResolver{}}
		html := mi.RenderToString(eng.Render(context.Background(), raw, nil, nil, fe.RenderContext{Locale: "ja", Module: mod, Form: form}))
		for _, m := range tag.FindAllStringSubmatch(html, -1) {
			s := strings.TrimSpace(m[2])
			if ja.MatchString(s) || !word.MatchString(s) || same[s] || zone.MatchString(s) || strings.HasPrefix(s, "formengine:") {
				continue
			}
			t.Errorf("%s/%s (rendered as %s.%s): %q has no translation", dir, stem, mod, form, s)
		}
	}
}

// The frame of every page (skip link, menus, notifications) and the texts the
// scripts show come from the locale files, in every language.
func TestChromeAndScriptTexts_AreTranslated(t *testing.T) {
	english := map[string]string{}
	for _, locale := range []string{"en", "es", "pt", "ja"} {
		tr, err := i18n.New(locale)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{AppName: "Test", Locale: locale, Translator: tr}
		for _, key := range []string{"layout.skip_to_content", "layout.open_menu", "layout.notifications", "layout.user_profile", "layout.logout", "layout.tabs"} {
			got := chromeText(cfg, key, "FALLBACK")
			if got == "FALLBACK" || got == key {
				t.Errorf("%s: no text for %s", locale, key)
			}
			if locale == "en" {
				english[key] = got
			} else if key != "layout.tabs" && got == english[key] {
				t.Errorf("%s: %s is still the English %q", locale, key, got)
			}
		}
		var texts map[string]string
		if err := json.Unmarshal([]byte(scriptTextsFor(cfg)), &texts); err != nil {
			t.Fatalf("%s: %v", locale, err)
		}
		for _, st := range scriptTexts {
			if texts[st[0]] == "" {
				t.Errorf("%s: the scripts get no text for %q (%s)", locale, st[0], st[1])
			}
		}
		if !strings.Contains(texts["delete-named"], "{name}") {
			t.Errorf("%s: the delete question must keep {name}: %q", locale, texts["delete-named"])
		}
		// the page carries them
		page := mi.RenderToString(BaseLayout(cfg, "T", "/", func(b *mi.Builder) mi.Node { return b.Div() }))
		if !strings.Contains(page, `data-i18n-texts="`) || strings.Contains(page, ">Skip to main content<") != (locale == "en") {
			t.Errorf("%s: the page does not carry the script texts or the translated skip link", locale)
		}
	}
	// without a translator: English, never a key
	if got := chromeText(Config{}, "layout.logout", "Log Out"); got != "Log Out" {
		t.Errorf("without a translator: %q", got)
	}
	if got := scriptTextsFor(Config{}); got != "{}" {
		t.Errorf("without a translator the scripts keep their own English, got %s", got)
	}
}

// The scripts ask SeamText for their texts and hold none in Spanish.
func TestScripts_TakeTheirTextsFromThePage(t *testing.T) {
	raw, err := os.ReadFile("../../web/static/js/app.js")
	if err != nil {
		t.Fatal(err)
	}
	js := string(raw)
	for _, st := range scriptTexts {
		if strings.HasPrefix(st[0], "upload-") {
			continue
		}
		if !strings.Contains(js, "SeamText('"+st[0]+"'") {
			t.Errorf("app.js does not ask for %q", st[0])
		}
	}
	if strings.Contains(js, "Error al cargar") || !strings.Contains(js, "data-i18n-texts") {
		t.Errorf("app.js still holds a Spanish text, or does not read the page's texts")
	}
	up, err := os.ReadFile("../../web/static/js/file-upload.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"upload-load-failed", "upload-failed", "upload-delete-failed"} {
		if !strings.Contains(string(up), "SeamText('"+name+"'") {
			t.Errorf("file-upload.js does not ask for %q", name)
		}
	}
}

// A stored criticality, a date, the pager and an error reply follow the language.
func TestStoredValuesDatesAndReplies_FollowTheLanguage(t *testing.T) {
	day := time.Date(2026, time.November, 2, 0, 0, 0, 0, time.UTC)
	want := map[string][3]string{
		"en": {"High", "2 Nov 2026", "Previous"},
		"es": {"Alta", "2 nov 2026", "Anterior"},
		"pt": {"Alta", "2 nov 2026", "Anterior"},
		"ja": {"高", "2026年11月2日", "前へ"},
	}
	for locale, w := range want {
		tr, err := i18n.New(locale)
		if err != nil {
			t.Fatal(err)
		}
		tl := tr.ForLocale(locale)
		if got := criticalityLabel(tl, "Alta"); got != w[0] {
			t.Errorf("%s: criticality Alta shown as %q, want %q", locale, got, w[0])
		}
		if got := formatDay(tl, day); got != w[1] {
			t.Errorf("%s: date %q, want %q", locale, got, w[1])
		}
		if pager := mi.RenderToString(Pagination(2, 3, "/x", tl)); !strings.Contains(pager, w[2]) {
			t.Errorf("%s: the pager lacks %q", locale, w[2])
		}
	}
	if criticalityLabel(nil, "Alta") != "Alta" || criticalityLabel(func(k string, _ ...any) string { return k }, "Unknown") != "Unknown" {
		t.Errorf("an unknown value, or no translator, shows the stored word")
	}
	if formatDay(nil, day) != "2 Nov 2026" {
		t.Errorf("without a translator the date is the English form")
	}
	if pager := mi.RenderToString(Pagination(2, 3, "/x", nil)); !strings.Contains(pager, "Previous") || !strings.Contains(pager, "Next") {
		t.Errorf("without a translator the pager is in English")
	}
	// an error reply: translated with a translator, the English literal without
	es, _ := i18n.New("es")
	h := NewUIHandler(nil, Config{AppName: "T", Locale: "es"}, es, nil, nil, nil, nil, nil, nil)
	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{UserID: 1, TenantID: 1, Role: "admin"}))
	if got := h.errText(req, "error.save_failed", "Failed to save"); got != "No se pudieron guardar los cambios" {
		t.Errorf("es: %q", got)
	}
	bare := NewUIHandler(nil, Config{AppName: "T"}, nil, nil, nil, nil, nil, nil, nil)
	if got := bare.errText(req, "error.save_failed", "Failed to save"); got != "Failed to save" {
		t.Errorf("without a translator: %q", got)
	}
}
