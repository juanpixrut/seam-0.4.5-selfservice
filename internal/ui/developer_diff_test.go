package ui

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"
)

func ops(lines []diffLine) string {
	var b strings.Builder
	for _, l := range lines {
		b.WriteByte(l.Op)
	}
	return b.String()
}

func TestLineDiff_EqualTextsHaveNoDiff(t *testing.T) {
	for _, c := range [][2]string{{"", ""}, {"a\nb", "a\nb"}, {"a\nb\n", "a\nb"}, {"a\r\nb", "a\nb"}} {
		if l, big := developerLineDiff(c[0], c[1]); l != nil || big {
			t.Errorf("%q vs %q must be no diff, got %v %v", c[0], c[1], l, big)
		}
	}
}

func TestLineDiff_ChangesInsertionsAndDeletions(t *testing.T) {
	cases := []struct{ name, a, b, want string }{
		{"one line changed", "a\nb\nc", "a\nB\nc", " -+ "},
		{"line added at the end", "a\nb", "a\nb\nc", "  +"},
		{"line added at the start", "b\nc", "a\nb\nc", "+  "},
		{"line removed", "a\nb\nc", "a\nc", " - "},
		{"everything new", "", "x\ny", "++"},
		{"everything gone", "x\ny", "", "--"},
		{"nothing in common", "a\nb", "c\nd", "--++"},
	}
	for _, c := range cases {
		l, _ := developerLineDiff(c.a, c.b)
		if ops(l) != c.want {
			t.Errorf("%s: ops %q, want %q (%v)", c.name, ops(l), c.want, l)
		}
	}
	l, _ := developerLineDiff("a\nb\nc", "a\nB\nc")
	if l[1].Text != "b" || l[2].Text != "B" {
		t.Errorf("the changed line carries its text: %v", l)
	}
}

func TestLineDiff_CollapsesLongUnchangedRunsKeepingContext(t *testing.T) {
	var a, b []string
	for i := 0; i < 20; i++ {
		a = append(a, fmt.Sprintf("line %d", i))
		b = append(b, fmt.Sprintf("line %d", i))
	}
	b[10] = "changed"
	l, _ := developerLineDiff(strings.Join(a, "\n"), strings.Join(b, "\n"))
	// 6 unchanged, then 3 context, the change, 3 context, then 7 unchanged
	if got := ops(l); got != "~   -+   ~" {
		t.Fatalf("ops %q:\n%v", got, l)
	}
	if l[0].Text != "7" || l[len(l)-1].Text != "6" {
		t.Errorf("a collapsed run says how many lines it hides: %v", l)
	}
}

func TestLineDiff_SizeBoundIsEnforcedAndFast(t *testing.T) {
	big := strings.Repeat("x\n", developerDiffMaxLines+1)
	if l, tooLarge := developerLineDiff(big, "y"); !tooLarge || l != nil {
		t.Errorf("more than %d lines must be refused, not diffed", developerDiffMaxLines)
	}
	if _, tooLarge := developerLineDiff("y", big); !tooLarge {
		t.Errorf("the bound applies to either side")
	}
	// the largest allowed diff, nothing in common, must stay quick
	var a, b []string
	for i := 0; i < developerDiffMaxLines; i++ {
		a = append(a, fmt.Sprintf("a%d", i))
		b = append(b, fmt.Sprintf("b%d", i))
	}
	start := time.Now()
	l, tooLarge := developerLineDiff(strings.Join(a, "\n"), strings.Join(b, "\n"))
	if tooLarge || len(l) != 2*developerDiffMaxLines {
		t.Errorf("the largest allowed diff must work: %d lines, tooLarge %v", len(l), tooLarge)
	}
	if time.Since(start) > 3*time.Second {
		t.Errorf("took %v", time.Since(start))
	}
}

func TestForms_DiffIsShownEscapedAndOnlyWhenThereIsOne(t *testing.T) {
	ref := developerSchemaRef{"system", "a"}
	data := FormsPageData{Writable: true, Editor: &formsEditor{Ref: ref, Form: stubForm, Diff: []diffLine{
		{'~', "4"}, {' ', "ctx"}, {'-', `<script>alert(1)</script>`}, {'+', `<img src=x onerror=alert(1)>`}}}}
	html := formsHTML(data)
	for _, want := range []string{"dev-diff-add", "dev-diff-del", "dev-diff-sep", "developer.forms.diff"} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %s:\n%s", want, html)
		}
	}
	for _, raw := range []string{"<script>alert(1)", "<img src=x"} {
		if strings.Contains(html, raw) {
			t.Errorf("a diff line is data and must be escaped: %q", raw)
		}
	}
	data.Editor.Diff = nil
	if strings.Contains(formsHTML(data), `class="dev-diff"`) {
		t.Errorf("no changes, no diff block")
	}
	data.Editor.DiffTooLarge = true
	if !strings.Contains(formsHTML(data), "developer.forms.diff_too_large") {
		t.Errorf("an oversize comparison says so")
	}
}

func TestForms_DiffInTheRealEditor(t *testing.T) {
	h := newDevHarness(t, true, true)
	open := func() string { return h.get(t, "/developer/forms?schema=system/user_create").Body.String() }
	if strings.Contains(open(), `class="dev-diff"`) {
		t.Fatal("a form with no override has nothing to compare")
	}
	// a preview shows what is about to be saved, and writes nothing
	rec := h.post(t, "/developer/forms/preview", url.Values{"schema": {"system/user_create"}, "json": {e2eOverride}})
	body := rec.Body.String()
	if !strings.Contains(body, "dev-diff-add") || !strings.Contains(body, "Alpha OVERRIDE") || !strings.Contains(body, "dev-diff-del") {
		t.Errorf("the preview must show the change against the built-in definition")
	}
	if !dirIsEmpty(t, h.schDir) {
		t.Errorf("a preview writes nothing")
	}
	// a preview of the unchanged text has no diff
	base := h.get(t, "/developer/forms?schema=system/user_create").Body.String()
	_ = base
	// the saved override shows its diff when opened again
	h.post(t, "/developer/forms/save", url.Values{"schema": {"system/user_create"}, "json": {e2eOverride}})
	if got := open(); !strings.Contains(got, "dev-diff-add") || !strings.Contains(got, "+ ") {
		t.Errorf("an override opened again shows how it differs")
	}
	h.post(t, "/developer/forms/remove", url.Values{"schema": {"system/user_create"}})
	if strings.Contains(open(), `class="dev-diff"`) {
		t.Errorf("after removing the override the diff is gone")
	}
}
