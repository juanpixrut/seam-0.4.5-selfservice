package main

import "testing"

func TestDisplayWidth(t *testing.T) {
	for text, want := range map[string]int{
		"":                   0,
		"admin":              5,
		"Diego Suárez":       12,
		"Diego Sua\u0301rez": 12, // the accent as a combining mark
		"管理者":                6,
		"運用マネージャー":           16,
		"─│✓":                3,
		"ＡＢ":                 4,
	} {
		if got := displayWidth(text); got != want {
			t.Errorf("displayWidth(%q) = %d, want %d", text, got, want)
		}
	}
}

// Every line of a table has the same display width, and nothing is cut: an
// email or a role longer than a fixed column used to push the next column out
// of line, or to be shortened to something that cannot be typed back.
func TestTableLines_AlignedAndWhole(t *testing.T) {
	header := []string{"ID", "NAME", "EMAIL", "ROLE", "ACTIVE"}
	rows := [][]string{
		{"1", "管理者", "admin@asahi-kenki.example", "admin", "Yes"},
		{"19", "Admin Staff", "accounting.staff@example.com", "admin_staff", "Yes"},
		{"21", "Marta Rivas", "marta.rivas@example.com", "account_executive", "No"},
		{"5", "Facundo Salaberry", "facundo.salaberry.h@estudiantes.utec.edu.uy", "operator", "Yes"},
	}
	lines := tableLines(header, rows)
	if len(lines) != len(rows)+2 {
		t.Fatalf("got %d lines, want %d", len(lines), len(rows)+2)
	}
	want := displayWidth(lines[0])
	for i, l := range lines {
		if displayWidth(l) != want {
			t.Errorf("line %d is %d columns wide, the header is %d:\n%s", i, displayWidth(l), want, l)
		}
	}
	for _, row := range rows {
		found := false
		for _, l := range lines {
			if containsAll(l, row) {
				found = true
			}
		}
		if !found {
			t.Errorf("a row was cut: %v", row)
		}
	}
}

func containsAll(line string, cells []string) bool {
	for _, c := range cells {
		if !contains(line, c) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
