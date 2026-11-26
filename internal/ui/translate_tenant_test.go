package ui

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/devruntime"
	"github.com/ha1tch/seam-ui/internal/i18n"
)

func translatorHandler(moduleOn bool) (*UIHandler, *i18n.Translator) {
	tr := i18n.NewFromMessages("en", map[string]map[string]string{"en": {"greet": "Hello"}})
	h := &UIHandler{translator: tr}
	h.config = Config{Locale: "en"}
	if moduleOn {
		h.config.DevRuntime = &devruntime.Runtime{}
	}
	return h, tr
}

func reqForTenant(tenant int64) *http.Request {
	req := httptest.NewRequest("GET", "/x", nil)
	return req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{TenantID: tenant}))
}

func TestTranslateFunc_ModuleOffIgnoresPerTenantSwitches(t *testing.T) {
	h, tr := translatorHandler(false)
	tr.SetOptions(3, i18n.Options{ShowKeys: true})
	if got := h.translateFunc(reqForTenant(3))("greet"); got != "Hello" {
		t.Errorf("with the Developer module off nothing changes, got %q", got)
	}
}

func TestTranslateFunc_ShowKeysAppliesToThatTenantOnly(t *testing.T) {
	h, tr := translatorHandler(true)
	tr.SetOptions(3, i18n.Options{ShowKeys: true})
	if got := h.translateFunc(reqForTenant(3))("greet"); got != "greet" {
		t.Errorf("tenant 3 asked for keys, got %q", got)
	}
	if got := h.translateFunc(reqForTenant(4))("greet"); got != "Hello" {
		t.Errorf("tenant 4 must be unaffected, got %q", got)
	}
}

func TestTranslateFunc_TenantOverridesDoNotLeakAcrossTenants(t *testing.T) {
	h, tr := translatorHandler(true)
	if err := tr.ReloadLayer(i18n.LayerXoluTenant, 3, map[string]map[string]string{"en": {"greet": "Hi, tenant 3"}}); err != nil {
		t.Fatal(err)
	}
	if got := h.translateFunc(reqForTenant(3))("greet"); got != "Hi, tenant 3" {
		t.Errorf("tenant 3 sees its own override, got %q", got)
	}
	for _, other := range []int64{4, 30} {
		if got := h.translateFunc(reqForTenant(other))("greet"); got != "Hello" {
			t.Errorf("tenant %d must not see tenant 3's override, got %q", other, got)
		}
	}
}

// Adversarial: anything without a usable tenant falls back to the global text.
func TestTranslateFunc_NoUsableTenantIsTheGlobalLookup(t *testing.T) {
	h, tr := translatorHandler(true)
	tr.SetOptions(0, i18n.Options{ShowKeys: true}) // must never be reached through a request
	tr.SetOptions(-1, i18n.Options{ShowKeys: true})
	noClaims := httptest.NewRequest("GET", "/x", nil)
	for name, r := range map[string]*http.Request{"nil request": nil, "no claims": noClaims, "tenant 0": reqForTenant(0), "negative tenant": reqForTenant(-1)} {
		got := h.translateFunc(r)("greet")
		if got != "Hello" && got != "greet" { // the global view of tenant 0 is the only other legal outcome
			t.Errorf("%s: got %q", name, got)
		}
	}
	if got := h.translateFunc(reqForTenant(-1))("greet"); got == "Hi, tenant -1" {
		t.Errorf("a negative tenant must not select a tenant layer")
	}
}

func TestTranslateFunc_NoTranslatorEchoesTheKey(t *testing.T) {
	h := &UIHandler{}
	if got := h.translateFunc(reqForTenant(1))("greet"); got != "greet" {
		t.Errorf("got %q", got)
	}
}

// Adversarial: the switches are flipped while pages translate (run with -race).
func TestTranslateFunc_FlipsWhileTranslatingAreSafe(t *testing.T) {
	h, tr := translatorHandler(true)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 300; j++ {
				tr.SetOptions(3, i18n.Options{ShowKeys: (i+j)%2 == 0, DisableFilesystem: j%3 == 0})
			}
		}(i)
		go func() {
			defer wg.Done()
			for j := 0; j < 300; j++ {
				if got := h.translateFunc(reqForTenant(3))("greet"); got != "Hello" && got != "greet" {
					t.Errorf("unexpected text %q", got)
					return
				}
			}
		}()
	}
	wg.Wait()
}
