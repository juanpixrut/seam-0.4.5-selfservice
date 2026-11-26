package formengine

import (
	"strings"
	"testing"
)

// The add button finds its rows container relative to itself (the button's
// previous sibling), not by id: nested repeatables repeat the container id
// in every outer row, and a getElementById lookup always hit the first one.
func TestRepeatable_AddFindsItsOwnContainerNotByID(t *testing.T) {
	html := renderRepeatableRow(t, nil)
	start := strings.Index(html, "(function(btn){")
	if start < 0 {
		t.Fatalf("expected an add handler taking the button as its argument:\n%s", html)
	}
	end := strings.Index(html[start:], "})(this)")
	if end < 0 {
		t.Fatalf("expected the add handler to be called with the button itself:\n%s", html)
	}
	add := html[start : start+end]
	if !strings.Contains(add, "btn.previousElementSibling") {
		t.Errorf("expected the add handler to use the button's previous sibling:\n%s", add)
	}
	if strings.Contains(add, "getElementById") {
		t.Errorf("the add handler must not look its container up by id:\n%s", add)
	}
}

// A cloned outer row keeps one row per nested container, so a new block
// starts with a single blank nested row.
func TestRepeatable_AddTrimsNestedRowsInTheClone(t *testing.T) {
	html := renderRepeatableRow(t, nil)
	if !strings.Contains(html, "n.parentNode.firstElementChild!==n") {
		t.Errorf("expected the add handler to drop nested rows after the first:\n%s", html)
	}
	// The trim runs before inputs are reset and before the clone is appended.
	ti := strings.Index(html, "firstElementChild")
	ai := strings.Index(html, "c.appendChild(t)")
	if ti < 0 || ai < 0 || ti > ai {
		t.Errorf("the nested-row trim must happen before the clone is appended")
	}
}
