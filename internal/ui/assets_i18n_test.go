package ui

import (
	"bytes"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"
)

// TestAssetsListPage_UsesTranslatorNotHardcodedEnglish is a real
// regression guard, added 2026-08-13 after a dedicated sweep (at
// The tech team's own request, distinct from this whole session's internal-
// identifier campaign) found AssetsListPage was the sole page in
// internal/ui hardcoding user-facing strings (Title, EmptyTitle,
// EmptyMessage, four status-filter labels, the type-filter "all"
// label, and the search placeholder) directly in English, bypassing
// t() entirely -- while every sibling field on the very same struct
// literals correctly called it. No test existed to catch this; this
// is that test. Renders with a translator that prefixes every real
// key with "XX:" so a genuinely-wired call is unmistakable from a
// hardcoded literal slipping through unchanged.
func TestAssetsListPage_UsesTranslatorNotHardcodedEnglish(t *testing.T) {
	marker := func(key string, args ...any) string { return "XX:" + key }

	data := AssetListData{
		Assets:   nil, // empty list -- exercises the EmptyTitle/EmptyMessage path
		Total:    0,
		Page:     1,
		PageSize: 20,
		AssetTypes: []AssetTypeOption{
			{ID: 1, Name: "Vehicle"},
		},
	}

	page := AssetsListPage(data, marker)
	var buf bytes.Buffer
	b := &mi.Builder{}
	if err := page(b).Render(&buf); err != nil {
		t.Fatalf("render failed: %v", err)
	}
	html := buf.String()

	// Every one of these keys must appear as its marked form -- proving
	// the corresponding field genuinely calls t(), not a hardcoded
	// English literal that happens to render the same text.
	wantKeys := []string{
		"nav.assets", // Title and pagination Label, both reuse this key
		"assets.empty_title",
		"assets.empty_message",
		"assets.search_placeholder",
		"sensors.filter.all_statuses",
		"sensors.filter.all_types",
		"asset.status.active",
		"asset.status.silent",
		"asset.status.alert",
		"asset.status.lost",
		"pagination.showing", // ListPaginationBar's own count line, fixed alongside this page's own strings after live verification caught it mixing hardcoded Spanish template text with the (by-then-correctly-translated) label
	}
	for _, key := range wantKeys {
		if !strings.Contains(html, "XX:"+key) {
			t.Errorf("expected marked translation for %q in rendered output, not found -- field may be hardcoded", key)
		}
	}

	// The specific hardcoded English strings this test guards against
	// reappearing, verbatim, if this fix is ever reverted.
	hardcodedRegressions := []string{
		"No assets found",
		"Get started by creating your first asset.",
		"Search assets...",
		"All Status",
		"All Types",
		"Mostrando", // ListPaginationBar's own former hardcoded Spanish count template
	}
	for _, s := range hardcodedRegressions {
		if strings.Contains(html, s) {
			t.Errorf("found hardcoded English string %q in rendered output -- should be translated via t()", s)
		}
	}
}
