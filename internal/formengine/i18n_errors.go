package formengine

import "regexp"

// This file implements I18N_SPEC.md §4.4: mapping a queryfy validation
// error's message to a translated, parameterised string via the
// configured Translator.
//
// The patterns below match queryfy's actual wire format for schemas
// built via jsonschema.FromJSON specifically (the import formengine's
// own parseSchema uses, confirmed by reading schema.go directly), not
// §4.4's own prose descriptions.
//
// Numeric min/max ("must be >= /<= %v") surfaced a genuine bug in
// queryfy itself while building this: compile.go's Compile() path (an
// optional performance pre-compilation step formengine never calls)
// re-implemented the same min/max check with different wording
// ("must be at least/at most %v") instead of reusing or matching
// builders/number.go's original. Same schema, same violation, two
// different messages depending on whether a caller compiled the
// schema first -- a real internal inconsistency, not a deliberate
// design difference. Fixed at the source, 2026-07-28
// (github.com/ha1tch/queryfy, compile.go's compileNumber), not worked
// around here by teaching this file two wordings for one error.
// Every other rule (required, email, length, pattern, enum) was
// confirmed identical between the two code paths individually, not
// assumed from the one mismatch found.
//
// Deliberately NOT covered here, since Asset's schema doesn't currently
// use them and there's no real caller to verify the mapping against:
// array-length constraints ("must have at least N items" / "at most N
// items", compile.go's ValidateArray path) and queryfy's pattern-name
// interpolation for invalid_pattern (§4.4's own table says no params
// for that key, matched exactly here, even though the raw message does
// carry the pattern string).

// queryfyErrorRule pairs one regex (matched against the FULL error
// message, anchored) with the i18n key it maps to. Capture groups, if
// any, become T()'s positional interpolation args in the order they
// appear in the pattern -- Translator.T uses fmt.Sprintf(msg, args...)
// under the hood (confirmed by reading its source), so the resolved
// locale string needs %s/%d-style verbs, not {min}/{max}-style named
// placeholders -- §4.4's {min}/{max} notation in the spec is
// describing intent, not literal template syntax.
type queryfyErrorRule struct {
	pattern *regexp.Regexp
	key     string
}

var queryfyErrorRules = []queryfyErrorRule{
	{regexp.MustCompile(`^field is required$`), "core.errors.required"},
	{regexp.MustCompile(`^must be a valid email address$`), "core.errors.invalid_email"},
	// Length rules matched before the numeric min/max rules below --
	// not because of ambiguity (the prefixes are textually distinct:
	// "length must be..." vs "must be..."), but so the ordering reads
	// as deliberate rather than incidental if these ever need a third,
	// genuinely overlapping rule added later.
	{regexp.MustCompile(`^length must be at least (\d+), got \d+$`), "core.errors.min_length"},
	{regexp.MustCompile(`^length must be at most (\d+), got \d+$`), "core.errors.max_length"},
	{regexp.MustCompile(`^must be >= ([\d.]+)$`), "core.errors.min_value"},
	{regexp.MustCompile(`^must be <= ([\d.]+)$`), "core.errors.max_value"},
	{regexp.MustCompile(`^must match pattern `), "core.errors.invalid_pattern"},
	{regexp.MustCompile(`^must be one of: `), "core.errors.not_in_enum"},
}

// ResolveQueryfyError implements §4.4: maps a queryfy FieldError's raw
// Message to a translated string via e.Translator, with rule-specific
// parameters interpolated positionally. Falls back to rawMessage
// unchanged when Translator is nil, or when the message doesn't match
// any known rule (an error shape this table hasn't been taught yet,
// or genuinely arbitrary text from a custom validator) -- same
// "always show something real, never a blank or a crash" principle as
// every other resolution path in this package.
func ResolveQueryfyError(e *FormEngine, rc RenderContext, rawMessage string) string {
	if e.Translator == nil {
		return rawMessage
	}
	for _, rule := range queryfyErrorRules {
		matches := rule.pattern.FindStringSubmatch(rawMessage)
		if matches == nil {
			continue
		}
		args := make([]any, len(matches)-1)
		for i, m := range matches[1:] {
			args[i] = m
		}
		return e.Translator.T(rc.Locale, rule.key, args...)
	}
	return rawMessage
}
