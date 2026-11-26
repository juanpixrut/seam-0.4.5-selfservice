package formengine

import "testing"

func TestStatusMatchesField_ExactMatch(t *testing.T) {
	if !statusMatchesField("name", "name") {
		t.Error("exact match should match")
	}
}

func TestStatusMatchesField_ArrayIndexPath(t *testing.T) {
	if !statusMatchesField("document_links[0].name", "document_links") {
		t.Error("document_links[0].name should match document_links")
	}
	if !statusMatchesField("document_links[3].url", "document_links") {
		t.Error("any index should match, not just [0]")
	}
}

func TestStatusMatchesField_DottedNestedPath(t *testing.T) {
	if !statusMatchesField("address.city", "address") {
		t.Error("address.city should match address")
	}
}

func TestStatusMatchesField_RejectsFalsePositivePrefix(t *testing.T) {
	// The exact case a naive prefix check would get wrong: a longer
	// field name that happens to start with the shorter one shouldn't
	// match unless the next character is genuinely "[" or ".".
	if statusMatchesField("document_links_extra[0].x", "document_links") {
		t.Error("document_links_extra[...] must NOT match document_links -- false positive prefix collision")
	}
	if statusMatchesField("document_linksish", "document_links") {
		t.Error("document_linksish must NOT match document_links")
	}
}

func TestStatusMatchesField_UnrelatedFieldDoesNotMatch(t *testing.T) {
	if statusMatchesField("email", "name") {
		t.Error("unrelated elements must not match")
	}
}

func TestResolveFieldError_ErrsMapWinsWhenBothApply(t *testing.T) {
	f := Field{Name: "name"}
	errs := map[string]string{"name": "from the map"}
	rc := RenderContext{Statuses: []FieldStatus{
		{Element: "name", Status: "ERR", Message: "from statuses"},
	}}
	got := resolveFieldError(f, errs, rc)
	if got != "from the map" {
		t.Errorf("got %q, want the errs map's message to win", got)
	}
}

func TestResolveFieldError_FallsBackToStatusesWhenErrsMapEmpty(t *testing.T) {
	f := Field{Name: "document_links"}
	rc := RenderContext{Statuses: []FieldStatus{
		{Element: "document_links[0].name", Status: "ERR", Message: "row 0 needs a name"},
	}}
	got := resolveFieldError(f, nil, rc)
	if got != "row 0 needs a name" {
		t.Errorf("got %q, want the matching Statuses entry's message", got)
	}
}

func TestResolveFieldError_IgnoresNonErrSeverities(t *testing.T) {
	f := Field{Name: "purge_backup"}
	rc := RenderContext{Statuses: []FieldStatus{
		{Element: "purge_backup", Status: "WRN", Message: "default destructive value left as-is"},
	}}
	got := resolveFieldError(f, nil, rc)
	if got != "" {
		t.Errorf("got %q, want empty -- WRN must not produce a blocking error", got)
	}
}

func TestResolveFieldError_NoMatchReturnsEmpty(t *testing.T) {
	f := Field{Name: "title"}
	rc := RenderContext{Statuses: []FieldStatus{
		{Element: "unrelated_field", Status: "ERR", Message: "x"},
	}}
	got := resolveFieldError(f, nil, rc)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestResolveFieldError_EmptyEverything(t *testing.T) {
	f := Field{Name: "name"}
	if got := resolveFieldError(f, nil, RenderContext{}); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}
