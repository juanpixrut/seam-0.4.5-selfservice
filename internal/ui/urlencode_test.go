package ui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestUrlEncode_NonASCIICorrectlyEncoded is the direct regression guard
// for T-31: the previous hand-rolled encoder passed every non-ASCII
// byte through unchanged, meaning any translated error message in
// Japanese, or Spanish/Portuguese with an accented character, carried
// raw UTF-8 straight into a redirect's query string. Confirms real
// examples from all three affected locales are now correctly
// percent-encoded, not just proven for a synthetic case.
func TestUrlEncode_NonASCIICorrectlyEncoded(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"japanese", "無効な認証情報です"},                     // "invalid credentials", ja
		{"spanish-accented", "Contraseña incorrecta"}, // "incorrect password", es
		{"portuguese-accented", "Sessão expirada"},    // "session expired", pt
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			encoded := urlEncode(c.input)
			for _, r := range c.input {
				if r > 127 && strings.ContainsRune(encoded, r) {
					t.Errorf("encoded output still contains a raw non-ASCII rune %q -- input %q wasn't actually encoded:\n%s", r, c.input, encoded)
				}
			}
			// Round-trip: a correctly-encoded string must decode back to
			// the exact original, confirming this isn't just "no raw
			// bytes visible" by coincidence.
			decoded, err := url.QueryUnescape(encoded)
			if err != nil {
				t.Fatalf("encoded output isn't valid query-escaped data: %v\nencoded: %s", err, encoded)
			}
			if decoded != c.input {
				t.Errorf("round-trip mismatch: got %q, want %q", decoded, c.input)
			}
		})
	}
}

// TestUrlEncode_SpaceEncodesToPlus confirms the one behavior explicitly
// worth preserving from the old implementation: url.QueryEscape's own
// default (space -> '+') already matches what every existing call site
// expects, so nothing about their own output changed for this specific
// character even though the whole implementation was replaced.
func TestUrlEncode_SpaceEncodesToPlus(t *testing.T) {
	if got := urlEncode("hello world"); got != "hello+world" {
		t.Errorf("expected space to encode as '+' (matching the previous implementation's own behavior), got %q", got)
	}
}

// TestUrlEncode_QueryStringInjectionCharactersEscaped confirms the
// characters the old encoder DID handle (&, =, ?, /, :) are still
// correctly escaped -- proving the replacement isn't a regression on
// the cases that already worked, only a fix for the cases that didn't.
func TestUrlEncode_QueryStringInjectionCharactersEscaped(t *testing.T) {
	encoded := urlEncode("a&b=c?d/e:f")
	for _, ch := range []string{"&", "=", "?", "/", ":"} {
		if strings.Contains(encoded, ch) {
			t.Errorf("encoded output still contains the raw special character %q:\n%s", ch, encoded)
		}
	}
}

// TestLoginFailure_EmailQueryParamIsEncoded is the adversarial one: an
// email value containing query-string-structural characters (&, =) --
// found alongside T-31, a real, previously-unencoded gap: every one of
// this handler's failure-redirect lines concatenated the submitted
// email directly into the query string with no encoding at all, even
// with the old, broken urlEncode() in place elsewhere on the same
// line. A submitted "email" of x@example.com&admin=true could inject
// an extra query parameter into the redirect target. Confirms the
// literal injected structure doesn't appear unescaped in the actual
// redirect Location header.
func TestLoginFailure_EmailQueryParamIsEncoded(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{})
	defer server.Close()
	h := newHandler(t, server)

	req := httptest.NewRequest("POST", "/login", strings.NewReader(url.Values{
		"email":    {"x@example.com&admin=true"},
		"password": {""}, // empty password -> the email_password_required failure path
	}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.LoginSubmit(rec, req)

	loc := rec.Header().Get("Location")
	if loc == "" {
		t.Fatal("expected a redirect Location header")
	}
	if strings.Contains(loc, "&admin=true") {
		t.Errorf("the submitted email's own \"&admin=true\" appears as a live, unescaped query parameter in the redirect target -- query-string injection via email: %s", loc)
	}
	// Parsing the redirect target's query string should show exactly
	// the two real parameters (error, email), not a third, injected one.
	parsed, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("redirect Location isn't a valid URL: %v (%s)", err, loc)
	}
	if parsed.Query().Get("admin") != "" {
		t.Errorf("an \"admin\" query parameter exists on the redirect target at all -- it should not, the email's own content should be inert data, not query structure: %s", loc)
	}
}
