package ui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	xolu "github.com/ha1tch/xolu/pkg/client"
)

// ── masterID (tests 1–2) ──────────────────────────────────────────────────────

func TestMasterID_ValidPaths(t *testing.T) {
	cases := []struct {
		path   string
		prefix string
		suffix string
		want   int64
	}{
		{"/regions/42/edit", "/regions/", "/edit", 42},
		{"/regions/42/delete", "/regions/", "/delete", 42},
		{"/regions/42", "/regions/", "", 42},
		{"/cost-centers/1/edit", "/cost-centers/", "/edit", 1},
		{"/properties/associations/99/delete", "/properties/associations/", "/delete", 99},
		{"/brands/1000000", "/brands/", "", 1000000},
	}
	for _, c := range cases {
		got := masterID(c.path, c.prefix, c.suffix)
		if got != c.want {
			t.Errorf("masterID(%q, %q, %q) = %d, want %d", c.path, c.prefix, c.suffix, got, c.want)
		}
	}
}

func TestMasterID_InvalidPaths(t *testing.T) {
	cases := []struct {
		path   string
		prefix string
		suffix string
	}{
		{"", "/regions/", ""},
		{"/regions/", "/regions/", ""},
		{"/regions/abc/edit", "/regions/", "/edit"},
		{"/regions/-1/edit", "/regions/", "/edit"},
		{"/regions/0/edit", "/regions/", "/edit"},
		{"/regions/42.5/edit", "/regions/", "/edit"},
	}
	for _, c := range cases {
		got := masterID(c.path, c.prefix, c.suffix)
		if got != 0 {
			t.Errorf("masterID(%q, %q, %q) = %d, want 0", c.path, c.prefix, c.suffix, got)
		}
	}
}

// ── parseDocumentLinks (tests 3–5) ───────────────────────────────────────────

func makeFormWithLinks(prefix string, rows []struct{ nombre, url, tipo string }) *http.Request {
	form := url.Values{}
	for _, r := range rows {
		form.Add(prefix+"_nombre[]", r.nombre)
		form.Add(prefix+"_url[]", r.url)
		form.Add(prefix+"_tipo[]", r.tipo)
	}
	req, _ := http.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.ParseForm()
	return req
}

func TestParseDocumentLinks_Normal(t *testing.T) {
	req := makeFormWithLinks("links_comodato", []struct{ nombre, url, tipo string }{
		{"Contrato", "https://example.com/contrato.pdf", "PDF"},
		{"Acta", "https://example.com/acta.pdf", "PDF"},
	})
	links := parseDocumentLinks(req, "links_comodato")
	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d", len(links))
	}
	if links[0].Name != "Contrato" || links[0].URL != "https://example.com/contrato.pdf" {
		t.Errorf("unexpected first link: %+v", links[0])
	}
	if links[1].Name != "Acta" {
		t.Errorf("unexpected second link name: %q", links[1].Name)
	}
}

func TestParseDocumentLinks_SkipBlankRows(t *testing.T) {
	req := makeFormWithLinks("links_ciclo_vida", []struct{ nombre, url, tipo string }{
		{"Doc1", "https://example.com/doc1.pdf", ""},
		{"", "", ""},    // blank row — should be skipped
		{"", "", "PDF"}, // also blank (nombre and url empty) — skipped
	})
	links := parseDocumentLinks(req, "links_ciclo_vida")
	if len(links) != 1 {
		t.Fatalf("expected 1 link after skipping blanks, got %d: %+v", len(links), links)
	}
}

func TestParseDocumentLinks_PartialRow(t *testing.T) {
	// Row with nombre but no url should be kept — operator may fill URL later
	req := makeFormWithLinks("links_docs", []struct{ nombre, url, tipo string }{
		{"Solo nombre", "", ""},
	})
	links := parseDocumentLinks(req, "links_docs")
	if len(links) != 1 {
		t.Fatalf("expected 1 link for partial row, got %d", len(links))
	}
	if links[0].Name != "Solo nombre" {
		t.Errorf("unexpected name: %q", links[0].Name)
	}
}

// ── toInt64FromString (test 6) ────────────────────────────────────────────────

func TestToInt64FromString(t *testing.T) {
	cases := []struct {
		input string
		want  int64
	}{
		{"42", 42},
		{"0", 0},
		{"", 0},
		{"abc", 0},
		{"-1", -1},
		{"9999999999", 9999999999},
		{"  ", 0},
	}
	for _, c := range cases {
		got := toInt64FromString(c.input)
		if got != c.want {
			t.Errorf("toInt64FromString(%q) = %d, want %d", c.input, got, c.want)
		}
	}
}

// ── toFloatFromString (test 7) ────────────────────────────────────────────────

func TestToFloatFromString(t *testing.T) {
	cases := []struct {
		input string
		want  float64
	}{
		{"3.14", 3.14},
		{"0", 0},
		{"", 0},
		{"abc", 0},
		{"-2.5", -2.5},
		{"1000", 1000},
	}
	for _, c := range cases {
		got := toFloatFromString(c.input)
		if got != c.want {
			t.Errorf("toFloatFromString(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

// ── shared test helpers (moved from the now-retired internal/ui/
// dashboard_test.go, attic/old-dashboard/ -- these three are genuinely
// used by other test files, unlike that file's dashboard-specific
// dateFromNow/mockOLUForDashboard, which stay retired) ────────────────────

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsHelper(s, sub))
}

func containsHelper(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func newTestHandler(t *testing.T, server *httptest.Server) *UIHandler {
	t.Helper()
	client := xolu.New(server.URL)
	cfg := Config{AppName: "Test", Version: "0.0.0"}
	return &UIHandler{
		xoluClient: client,
		page:       NewPageHandler(cfg),
	}
}
