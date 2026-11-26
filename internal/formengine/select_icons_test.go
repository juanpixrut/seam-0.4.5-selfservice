package formengine

import (
	"context"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"
)

func selectSchema(extra string) []byte {
	return []byte(`{"type":"object","properties":{"priority":{"type":"string","title":"Priority","x-seam-input":"select",
	  "enum":["low","medium","high"],` + extra + `"x-seam-placeholder":"Pick"}}}`)
}

func renderSelectSchema(t *testing.T, schema []byte, e *FormEngine) string {
	t.Helper()
	if e == nil {
		e = &FormEngine{}
	}
	return mi.RenderToString(e.Render(context.Background(), schema, map[string]interface{}{"priority": "medium"}, nil, RenderContext{Locale: "en"}))
}

func TestIcons_ParseAcceptsAndRefuses(t *testing.T) {
	good := []string{`"x-seam-option-icons":{"low":"arrow_downward","high":"priority_high"},`, `"x-seam-icon-position":"right",`,
		`"x-seam-icon-position":"left",`, `"x-seam-option-icons":{},`, `"x-seam-option-icons":{"low":"a"},`}
	for _, extra := range good {
		if _, err := ParseSchema(selectSchema(extra)); err != nil {
			t.Errorf("%s must be accepted: %v", extra, err)
		}
	}
	long := strings.Repeat("a", 65)
	bad := []string{
		`"x-seam-option-icons":{"low":"Check"},`, `"x-seam-option-icons":{"low":"check circle"},`, `"x-seam-option-icons":{"low":"\"><script>alert(1)</script>"},`,
		`"x-seam-option-icons":{"low":"1abc"},`, `"x-seam-option-icons":{"low":"a-b"},`, `"x-seam-option-icons":{"low":""},`,
		`"x-seam-option-icons":{"low":"` + long + `"},`, `"x-seam-option-icons":{"low":"é"},`, `"x-seam-option-icons":{"low":5},`,
		`"x-seam-icon-position":"center",`, `"x-seam-icon-position":"LEFT",`, `"x-seam-icon-position":true,`,
	}
	for _, extra := range bad {
		if _, err := ParseSchema(selectSchema(extra)); err == nil {
			t.Errorf("%.60s must be refused when the schema is parsed", extra)
		}
	}
	var many []string
	for i := 0; i < maxOptionIcons+1; i++ {
		many = append(many, `"k`+strings.Repeat("x", 0)+string(rune('a'+i%26))+strings.Repeat("z", i/26)+`":"star"`)
	}
	if _, err := ParseSchema(selectSchema(`"x-seam-option-icons":{` + strings.Join(many, ",") + `},`)); err == nil {
		t.Errorf("more than %d icon entries must be refused", maxOptionIcons)
	}
}

func TestIcons_OptionsCarryTheirIconsAndNoOthers(t *testing.T) {
	html := renderSelectSchema(t, selectSchema(`"x-seam-option-icons":{"low":"arrow_downward","high":"priority_high","ghost":"star"},`), nil)
	if strings.Count(html, `data-icon="arrow_downward"`) != 1 || strings.Count(html, `data-icon="priority_high"`) != 1 {
		t.Errorf("the options that have an icon carry it:\n%s", html)
	}
	if strings.Count(html, "data-icon=") != 2 {
		t.Errorf("the empty option, the option without one and a key that is no option must carry none")
	}
	if strings.Contains(html, `data-icon="star"`) {
		t.Errorf("an icon for a value that is not an option is ignored")
	}
}

func TestIcons_PositionOnlyWhenRightAndThereAreIcons(t *testing.T) {
	icons := `"x-seam-option-icons":{"low":"arrow_downward"},`
	if h := renderSelectSchema(t, selectSchema(icons+`"x-seam-icon-position":"right",`), nil); !strings.Contains(h, `data-ss-icon-pos="right"`) {
		t.Errorf("right is told to the combobox")
	}
	for name, extra := range map[string]string{"left": icons + `"x-seam-icon-position":"left",`, "default": icons,
		"right without icons": `"x-seam-icon-position":"right",`} {
		if h := renderSelectSchema(t, selectSchema(extra), nil); strings.Contains(h, "data-ss-icon-pos") {
			t.Errorf("%s: nothing to say about the position", name)
		}
	}
}

// With no icons the output is exactly what it was.
func TestIcons_UnconfiguredSelectIsUnchanged(t *testing.T) {
	plain := renderSelectSchema(t, selectSchema(""), nil)
	empty := renderSelectSchema(t, selectSchema(`"x-seam-option-icons":{},`), nil)
	if plain != empty {
		t.Errorf("an empty icon map must not change the output")
	}
	if strings.Contains(plain, "data-icon") || strings.Contains(plain, "data-ss-icon-pos") {
		t.Errorf("a select without icons must not output any")
	}
}

func TestIcons_GroupedOptionsCarryTheirIcons(t *testing.T) {
	schema := []byte(`{"type":"object","properties":{"unit":{"type":"string","x-seam-input":"select","x-seam-option-groups":true,
	  "enum":["distance_km","distance_mi","mass_kg"],"x-seam-option-icons":{"distance_km":"straighten","mass_kg":"scale"}}}}`)
	html := renderSelectSchema(t, schema, nil)
	if !strings.Contains(html, "<optgroup") || strings.Count(html, "data-icon=") != 2 {
		t.Errorf("icons survive grouping:\n%s", html)
	}
}

type iconSource struct{ opts []Option }

func (s iconSource) Resolve(ctx context.Context, source string) ([]Option, error) { return s.opts, nil }

// Adversarial: an option from a resolver is data; a hostile icon name is dropped.
func TestIcons_ResolverOptionsAndHostileNames(t *testing.T) {
	e := &FormEngine{Sources: iconSource{[]Option{
		{Value: "1", Label: "Acme", Icon: "business"},
		{Value: "2", Label: "Evil", Icon: `"><img src=x onerror=alert(1)>`},
		{Value: "3", Label: "Odd", Icon: "Has Space"},
		{Value: "4", Label: "<b>Bold</b>", Icon: "star"},
	}}}
	schema := []byte(`{"type":"object","properties":{"vendor":{"type":"integer","x-seam-input":"select","x-seam-source":"entity:vendors"}}}`)
	html := mi.RenderToString(e.Render(context.Background(), schema, nil, nil, RenderContext{Locale: "en"}))
	if !strings.Contains(html, `data-icon="business"`) || !strings.Contains(html, `data-icon="star"`) {
		t.Errorf("a resolver's valid icons are used:\n%s", html)
	}
	for _, raw := range []string{"<img src=x", "onerror=alert", `data-icon="Has Space"`, "<b>Bold</b>"} {
		if strings.Contains(html, raw) {
			t.Errorf("unsafe or unescaped %q in the output", raw)
		}
	}
	if strings.Count(html, "data-icon=") != 2 {
		t.Errorf("only the two valid icons are kept")
	}
}

// With the combobox switched off a native select cannot show icons: the text
// stays and no combobox hint is emitted.
func TestIcons_NativeSelectKeepsItsTextWhenTheComboboxIsOff(t *testing.T) {
	withOptions(t, Options{SearchableSelect: false, SelectSearchMin: 8, ReorderRows: true})
	html := renderSelectSchema(t, selectSchema(`"x-seam-option-icons":{"low":"arrow_downward"},"x-seam-icon-position":"right",`), nil)
	if strings.Contains(html, "data-seam-select") || strings.Contains(html, "data-ss-icon-pos") {
		t.Errorf("no combobox attributes when the combobox is off")
	}
	for _, label := range []string{">low<", ">medium<", ">high<"} {
		if !strings.Contains(html, label) {
			t.Errorf("the native option text must stay: %s", label)
		}
	}
}
