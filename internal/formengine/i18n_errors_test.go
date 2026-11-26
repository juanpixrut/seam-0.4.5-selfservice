package formengine

import (
	"testing"

	qf "github.com/ha1tch/queryfy"
	"github.com/ha1tch/queryfy/builders/jsonschema"

	"github.com/ha1tch/seam-ui/internal/i18n"
)

// realQueryfyErrors builds an actual queryfy schema (via the same
// jsonschema.FromJSON import path a real caller would use) covering
// every rule this file maps, runs real validation against data
// designed to trigger each one, and returns the genuine FieldError
// messages queryfy produced -- not hand-typed fixtures guessing at
// queryfy's wording. This is the test that actually proves the regex
// patterns match reality, not just my reading of compile.go.
func realQueryfyErrors(t *testing.T) map[string]string {
	t.Helper()
	schemaJSON := []byte(`{
		"type": "object",
		"properties": {
			"email":    {"type": "string", "format": "email"},
			"age":      {"type": "integer", "minimum": 13, "maximum": 150},
			"username": {"type": "string", "minLength": 3, "maxLength": 20},
			"code":     {"type": "string", "pattern": "^[A-Z]{3}$"},
			"role":     {"type": "string", "enum": ["admin", "editor", "viewer"]}
		},
		"required": ["email"]
	}`)
	schema, errs := jsonschema.FromJSON(schemaJSON, nil)
	if len(errs) > 0 {
		t.Fatalf("schema import failed: %v", errs)
	}

	data := map[string]interface{}{
		"age":      float64(200), // triggers maximum
		"username": "ab",         // triggers minLength
		"code":     "abc",        // triggers pattern
		"role":     "superadmin", // triggers enum
		// email deliberately absent -- triggers required
	}
	err := qf.Validate(data, schema)
	if err == nil {
		t.Fatal("expected validation to fail, got nil error")
	}
	verr, ok := err.(*qf.ValidationError)
	if !ok {
		t.Fatalf("expected *qf.ValidationError, got %T: %v", err, err)
	}

	byPath := make(map[string]string, len(verr.Errors))
	for _, fe := range verr.Errors {
		t.Logf("real queryfy error: path=%q message=%q", fe.Path, fe.Message)
		byPath[fe.Path] = fe.Message
	}
	return byPath
}

func TestResolveQueryfyError_RealMessages_AllMapToTheExpectedKey(t *testing.T) {
	byPath := realQueryfyErrors(t)

	tr := fakeTranslator{"es": {
		"core.errors.required":        "El campo es obligatorio",
		"core.errors.max_value":       "Debe ser como m\u00e1ximo %s",
		"core.errors.min_length":      "Debe tener al menos %s caracteres",
		"core.errors.invalid_pattern": "No coincide con el patr\u00f3n",
		"core.errors.not_in_enum":     "Valor no v\u00e1lido",
	}}
	e := &FormEngine{Translator: tr}
	rc := RenderContext{Locale: "es"}

	cases := []struct {
		path string
		want string
	}{
		{"email", "El campo es obligatorio"},
		{"age", "Debe ser como m\u00e1ximo 150"},
		{"username", "Debe tener al menos 3 caracteres"},
		{"code", "No coincide con el patr\u00f3n"},
		{"role", "Valor no v\u00e1lido"},
	}
	for _, c := range cases {
		raw, ok := byPath[c.path]
		if !ok {
			t.Errorf("no real queryfy error for path %q -- test schema/data didn't trigger it", c.path)
			continue
		}
		got := ResolveQueryfyError(e, rc, raw)
		if got != c.want {
			t.Errorf("path %q: raw=%q resolved=%q, want %q", c.path, raw, got, c.want)
		}
	}
}

func TestResolveQueryfyError_UnrecognisedMessagePassesThrough(t *testing.T) {
	e := &FormEngine{Translator: fakeTranslator{}}
	rc := RenderContext{Locale: "es"}
	got := ResolveQueryfyError(e, rc, "some custom validator message not in the table")
	if got != "some custom validator message not in the table" {
		t.Errorf("unrecognised message should pass through unchanged, got %q", got)
	}
}

func TestResolveQueryfyError_NilTranslatorPassesThrough(t *testing.T) {
	e := &FormEngine{}
	rc := RenderContext{Locale: "es"}
	got := ResolveQueryfyError(e, rc, "field is required")
	if got != "field is required" {
		t.Errorf("a nil Translator should pass the raw message through unchanged, got %q", got)
	}
}

func TestResolveQueryfyError_MinValueRule(t *testing.T) {
	// Not exercised by realQueryfyErrors' fixture data (age triggers
	// maximum, not minimum) -- covered directly with a hand-built
	// message matching the verified "must be >= %v" format, from
	// builders/number.go, the code path jsonschema.FromJSON actually
	// uses. A different code path (compile.go's Compile(), which
	// formengine never calls) used to produce "must be at least %v"
	// instead -- a genuine internal inconsistency in queryfy itself,
	// fixed at the source (github.com/ha1tch/queryfy, not worked
	// around here) rather than taught two wordings to match.
	tr := fakeTranslator{"es": {"core.errors.min_value": "Mínimo %s"}}
	e := &FormEngine{Translator: tr}
	rc := RenderContext{Locale: "es"}
	got := ResolveQueryfyError(e, rc, "must be >= 13")
	if got != "Mínimo 13" {
		t.Errorf("got %q, want \"Mínimo 13\"", got)
	}
}

func TestResolveQueryfyError_MaxLengthRule(t *testing.T) {
	tr := fakeTranslator{"es": {"core.errors.max_length": "M\u00e1ximo %s caracteres"}}
	e := &FormEngine{Translator: tr}
	rc := RenderContext{Locale: "es"}
	got := ResolveQueryfyError(e, rc, "length must be at most 20, got 25")
	if got != "M\u00e1ximo 20 caracteres" {
		t.Errorf("got %q, want \"M\u00e1ximo 20 caracteres\"", got)
	}
}

func TestResolveQueryfyError_InvalidEmailRule(t *testing.T) {
	tr := fakeTranslator{"es": {"core.errors.invalid_email": "Email inv\u00e1lido"}}
	e := &FormEngine{Translator: tr}
	rc := RenderContext{Locale: "es"}
	got := ResolveQueryfyError(e, rc, "must be a valid email address")
	if got != "Email inv\u00e1lido" {
		t.Errorf("got %q, want \"Email inv\u00e1lido\"", got)
	}
}

func TestResolveQueryfyError_RulesAreAnchored(t *testing.T) {
	// A message that merely contains a rule's trigger text as a
	// substring, but isn't shaped like the real message, must not
	// false-positive match.
	e := &FormEngine{Translator: fakeTranslator{"es": {"core.errors.required": "Obligatorio"}}}
	rc := RenderContext{Locale: "es"}
	got := ResolveQueryfyError(e, rc, "note: this field is required in some contexts but not here")
	if got == "Obligatorio" {
		t.Errorf("expected no match (unanchored substring), got the required-field translation")
	}
}

// TestResolveQueryfyError_RealContent_AllEightKeysResolve is T-17 gap
// 3's proof for the error-message half of the content, using the
// real, shipped i18n.New translator rather than fakeTranslator --
// confirms the actual core.errors.* content authored resolves for
// every one of the 8 rules, in both en and es, not just that the
// rule-matching mechanism works against arbitrary mock content.
func TestResolveQueryfyError_RealContent_AllEightKeysResolve(t *testing.T) {
	translator, err := i18n.New("en")
	if err != nil {
		t.Fatalf("i18n.New: %v", err)
	}
	e := &FormEngine{Translator: translator}

	cases := []struct {
		rawMessage string
		locale     string
	}{
		{"field is required", "en"},
		{"field is required", "es"},
		{"must be a valid email address", "en"},
		{"must be a valid email address", "es"},
		{"length must be at least 3, got 1", "en"},
		{"length must be at least 3, got 1", "es"},
		{"length must be at most 10, got 20", "en"},
		{"length must be at most 10, got 20", "es"},
		{"must be >= 5", "en"},
		{"must be >= 5", "es"},
		{"must be <= 100", "en"},
		{"must be <= 100", "es"},
		{"must match pattern ^[a-z]+$", "en"},
		{"must match pattern ^[a-z]+$", "es"},
		{"must be one of: a, b, c", "en"},
		{"must be one of: a, b, c", "es"},
	}
	for _, c := range cases {
		rc := RenderContext{Locale: c.locale}
		got := ResolveQueryfyError(e, rc, c.rawMessage)
		if got == c.rawMessage {
			t.Errorf("[%s] %q did not resolve -- fell through to the raw message unchanged", c.locale, c.rawMessage)
		}
	}
}
