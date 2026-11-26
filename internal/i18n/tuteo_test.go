package i18n

import (
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"testing"
)

// The Spanish of the interface addresses the person as "tú" (the tuteo of
// Uruguay): "Selecciona", "Ingresa", "quieres", "puedes". Voseo forms
// ("Seleccioná", "Ingresá", "querés", "podés") crept in with several modules
// and were all converted (T-136); this keeps the file in one form of address.
func TestSpanish_UsesTuteoThroughout(t *testing.T) {
	raw, err := os.ReadFile("locales/es.json")
	if err != nil {
		t.Fatal(err)
	}
	var es map[string]string
	if err := json.Unmarshal(raw, &es); err != nil {
		t.Fatal(err)
	}
	// imperatives stressed on the last syllable, and present forms in -ás, -és, -ís
	imperative := regexp.MustCompile(`(?i)(^|[^\p{L}])(seleccion|ingres|revis|intent|indic|complet|configur|cre|guard|gestion|prioriz|us|agreg|quit|carg|verific|contact|prob|busc|mir|cerr|control|adapt|explor|registr|planific|asign)á(l[oa]s?|me|te|se|nos)?([^\p{L}]|$)`)
	other := regexp.MustCompile(`(?i)(^|[^\p{L}])(volvé|tené|hacé|poné|elegí|seguí|abrí|subí|escribí|transferí|definí|podés|querés|tenés|sabés|creás|usás|necesitás|hacés|elegís|vos)([^\p{L}]|$)`)
	var bad []string
	for k, v := range es {
		if imperative.MatchString(v) || other.MatchString(v) {
			bad = append(bad, k+" = "+v)
		}
	}
	sort.Strings(bad)
	for _, b := range bad {
		t.Errorf("voseo in es.json: %s", b)
	}
}
