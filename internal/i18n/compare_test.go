package i18n

import (
	"reflect"
	"testing"
)

func TestFormatVerbs(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"plain", nil},
		{"%d items", []string{"%d"}},
		{"%s has %d of %s", []string{"%d", "%s", "%s"}},
		{"100%% done", nil},
		{"%5.2f%% of %s", []string{"%5.2f", "%s"}},
		{"%-10s|%+d", []string{"%+d", "%-10s"}},
	}
	for _, c := range cases {
		got := formatVerbs(c.in)
		if len(got) == 0 && len(c.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("formatVerbs(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestSameVerbsIgnoresOrder(t *testing.T) {
	if !sameVerbs("%s then %d", "%d luego %s") {
		t.Error("reordered verbs should count as the same set")
	}
	if sameVerbs("%s", "%d") || sameVerbs("%s", "none") || sameVerbs("none", "%s") {
		t.Error("different verb sets must differ")
	}
}

func TestWorthTranslating(t *testing.T) {
	for _, s := range []string{"", "OK", "%d", "42", "—", "ab"} {
		if worthTranslating(s) {
			t.Errorf("%q should not be worth translating", s)
		}
	}
	for _, s := range []string{"Save", "Seam", "Delete asset", "削除する"} {
		if !worthTranslating(s) {
			t.Errorf("%q should be worth translating", s)
		}
	}
}

func TestCompare(t *testing.T) {
	msgs := map[string]map[string]string{
		"en": {"a": "Save", "b": "%d items", "c": "OK", "d": "Delete", "e": "Hello %s"},
		"es": {"a": "Guardar", "b": "%d elementos", "c": "OK", "d": "Delete", "extra": "Sobra", "e": "Hola"},
		"pt": {"a": "Salvar", "b": "itens", "c": "OK", "d": "Excluir", "e": "Olá %s"},
		"ja": {"a": "保存"},
	}
	rep, err := Compare(msgs, "en")
	if err != nil {
		t.Fatal(err)
	}
	if rep.Ref != "en" || rep.RefKeys != 5 || len(rep.Diffs) != 3 {
		t.Fatalf("report header: %+v", rep)
	}
	byLoc := map[string]LocaleDiff{}
	for _, d := range rep.Diffs {
		byLoc[d.Locale] = d
	}
	if rep.Diffs[0].Locale != "es" || rep.Diffs[1].Locale != "ja" || rep.Diffs[2].Locale != "pt" {
		t.Errorf("diffs must be sorted by locale: %v", rep.Diffs)
	}

	es := byLoc["es"]
	if !reflect.DeepEqual(es.Untranslated, []string{"d"}) {
		t.Errorf("es untranslated = %v (c is 'OK', too short to count)", es.Untranslated)
	}
	if !reflect.DeepEqual(es.Extra, []string{"extra"}) {
		t.Errorf("es extra = %v", es.Extra)
	}
	if !reflect.DeepEqual(es.FormatMismatch, []string{"e"}) {
		t.Errorf("es format mismatch = %v (the %%s was dropped)", es.FormatMismatch)
	}
	if len(es.Missing) != 0 {
		t.Errorf("es missing = %v", es.Missing)
	}

	pt := byLoc["pt"]
	if !reflect.DeepEqual(pt.FormatMismatch, []string{"b"}) {
		t.Errorf("pt format mismatch = %v", pt.FormatMismatch)
	}

	ja := byLoc["ja"]
	if !reflect.DeepEqual(ja.Missing, []string{"b", "c", "d", "e"}) {
		t.Errorf("ja missing = %v", ja.Missing)
	}
	if rep.Clean() || ja.Clean() {
		t.Error("report should not be clean")
	}
}

func TestCompareCleanAndErrors(t *testing.T) {
	rep, err := Compare(map[string]map[string]string{
		"en": {"a": "Save", "b": "%d items"},
		"es": {"a": "Guardar", "b": "%d elementos"},
	}, "en")
	if err != nil || !rep.Clean() {
		t.Errorf("clean report: %v, %+v", err, rep)
	}
	if _, err := Compare(map[string]map[string]string{"en": {}}, "fr"); err == nil {
		t.Error("an absent reference locale must be an error")
	}
}

func TestCompareIgnoreIdentical(t *testing.T) {
	msgs := map[string]map[string]string{
		"en": {"brand": "Seam", "word": "Delete"},
		"es": {"brand": "Seam", "word": "Delete"},
	}
	rep, _ := Compare(msgs, "en", CompareOptions{IgnoreIdentical: func(loc, key, msg string) bool { return key == "brand" }})
	if got := rep.Diffs[0].Untranslated; !reflect.DeepEqual(got, []string{"word"}) {
		t.Errorf("untranslated = %v", got)
	}
}

func TestTranslatorCompareSeesOverlays(t *testing.T) {
	tr := newTr()
	base, err := tr.Compare(0, "en")
	if err != nil {
		t.Fatal(err)
	}
	// es lacks "count" and "only_en" in the base fixture.
	if es := base.Diffs[0]; es.Locale != "es" || len(es.Missing) != 2 {
		t.Fatalf("base es diff: %+v", es)
	}

	mustReload(t, tr, LayerFilesystem, 0, layerMsgs("es", "count", "%d elementos", "only_en", "Solo inglés"))
	after, _ := tr.Compare(0, "en")
	if es := after.Diffs[0]; len(es.Missing) != 0 {
		t.Errorf("an overlay that supplies the keys should close the gap: %+v", es)
	}

	// A tenant override with a stray placeholder is caught for that tenant only.
	// ("bye" is used because the filesystem layer above already overrides
	// "count", and the filesystem layer outranks the tenant layer.)
	mustReload(t, tr, LayerXoluTenant, 4, layerMsgs("es", "bye", "Adiós %s"))
	t4, _ := tr.Compare(4, "en")
	if es := t4.Diffs[0]; !reflect.DeepEqual(es.FormatMismatch, []string{"bye"}) {
		t.Errorf("tenant 4: %+v", es)
	}
	if g, _ := tr.Compare(0, "en"); len(g.Diffs[0].FormatMismatch) != 0 {
		t.Errorf("the global view must not see tenant 4's override: %+v", g.Diffs[0])
	}
}
