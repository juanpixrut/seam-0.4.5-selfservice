package ui

import (
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"
)

// ListRow must keep rendering exactly what it did before ListRowWithAttrs
// existed: it now delegates, and nil attrs must add nothing.
func TestListRow_DelegatesWithoutExtraAttributes(t *testing.T) {
	plain := mi.RenderToString(ListRow(0, ListCell("a"), ListCellMuted("b")))
	viaAttrs := mi.RenderToString(ListRowWithAttrs(0, nil, ListCell("a"), ListCellMuted("b")))
	if plain != viaAttrs {
		t.Errorf("ListRow and ListRowWithAttrs(nil) differ:\n%s\n%s", plain, viaAttrs)
	}
	empty := mi.RenderToString(ListRowWithAttrs(0, []interface{}{}, ListCell("a"), ListCellMuted("b")))
	if plain != empty {
		t.Errorf("empty attrs must equal nil attrs:\n%s\n%s", plain, empty)
	}
}

func TestListRowWithAttrs_AddsAttributesAndKeepsStripe(t *testing.T) {
	attrs := []interface{}{mi.Attr("data-id", "42"), mi.Attr("data-label", "Ana")}

	even := mi.RenderToString(ListRowWithAttrs(0, attrs, ListCell("x")))
	odd := mi.RenderToString(ListRowWithAttrs(1, attrs, ListCell("x")))

	for name, html := range map[string]string{"even": even, "odd": odd} {
		if !strings.Contains(html, `data-id="42"`) || !strings.Contains(html, `data-label="Ana"`) {
			t.Errorf("%s row lost its attributes: %s", name, html)
		}
	}
	if !strings.Contains(even, "bg-white") || strings.Contains(even, "bg-gray-50") {
		t.Errorf("even row stripe wrong: %s", even)
	}
	if !strings.Contains(odd, "bg-gray-50") {
		t.Errorf("odd row stripe wrong: %s", odd)
	}
}

// A hostile attribute value (a record label) must not break out of the
// attribute. This is the property the Selector relies on when it puts
// data-label on each row.
func TestListRowWithAttrs_HostileAttributeValueIsEscaped(t *testing.T) {
	hostile := `"><script>alert(1)</script>`
	html := mi.RenderToString(ListRowWithAttrs(0, []interface{}{mi.Attr("data-label", hostile)}, ListCell("x")))
	if strings.Contains(html, "<script>") {
		t.Errorf("attribute value broke out of its attribute: %s", html)
	}
}

// The key link of a navigation list marks its row as clickable (SeamList).
func TestListCellLink_CarriesTheRowLinkMarker(t *testing.T) {
	html := mi.RenderToString(ListRow(0, ListCellLink("Pump 7", "/assets/7")))
	for _, want := range []string{`href="/assets/7"`, `data-row-link="1"`, ">Pump 7<"} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %s: %s", want, html)
		}
	}
}
