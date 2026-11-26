package qrcode

import (
	"bytes"
	"strings"
	"testing"
)

func TestGenerator_Generate(t *testing.T) {
	gen := NewGenerator(DefaultConfig())

	code, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	// Check prefix
	if !strings.HasPrefix(code, "SEAM") {
		t.Errorf("code should start with SEAM, got %s", code)
	}

	// Check length (prefix + 12 chars)
	expectedLen := 4 + 12 // "SEAM" + 12 chars
	if len(code) != expectedLen {
		t.Errorf("code length = %d, want %d", len(code), expectedLen)
	}

	// Check uppercase
	if code != strings.ToUpper(code) {
		t.Error("code should be uppercase")
	}
}

func TestGenerator_GenerateUnique(t *testing.T) {
	gen := NewGenerator(DefaultConfig())

	codes := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		code, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate() error: %v", err)
		}

		if codes[code] {
			t.Errorf("duplicate code generated: %s", code)
		}
		codes[code] = true
	}
}

func TestGenerator_GenerateBatch(t *testing.T) {
	gen := NewGenerator(DefaultConfig())

	codes, err := gen.GenerateBatch(100)
	if err != nil {
		t.Fatalf("GenerateBatch() error: %v", err)
	}

	if len(codes) != 100 {
		t.Errorf("expected 100 codes, got %d", len(codes))
	}

	// Check uniqueness
	seen := make(map[string]bool)
	for _, code := range codes {
		if seen[code] {
			t.Errorf("duplicate code in batch: %s", code)
		}
		seen[code] = true
	}
}

func TestGenerator_Validate(t *testing.T) {
	gen := NewGenerator(DefaultConfig())

	// Generate a valid code
	validCode, _ := gen.Generate()

	tests := []struct {
		name    string
		code    string
		wantErr bool
	}{
		{"valid code", validCode, false},
		{"valid lowercase", strings.ToLower(validCode), false},
		{"wrong prefix", "ABC" + validCode[3:], true},
		{"too short", validCode[:5], true},
		{"too long", validCode + "X", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := gen.Validate(tt.code)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(%q) error = %v, wantErr %v", tt.code, err, tt.wantErr)
			}
		})
	}
}

func TestGenerator_ValidateChecksum(t *testing.T) {
	gen := NewGenerator(Config{
		Prefix:   "SEAM",
		Length:   12,
		Checksum: true,
	})

	code, _ := gen.Generate()

	// Valid checksum
	if err := gen.Validate(code); err != nil {
		t.Errorf("valid code rejected: %v", err)
	}

	// Corrupt checksum -- rotate the actual last character to the next
	// one in checksumChars (qrcode.go), wrapping around, rather than a
	// fixed literal. checksumChars is a 34-character alphabet that
	// legitimately produces any of its own characters as a real
	// checksum char, including whatever a fixed literal here would be
	// -- a fixed literal corrupts to the SAME character roughly 1 run
	// in 34, making corrupted == code and the test fail on a code
	// Validate() correctly accepts (T-09, found 2026-07-28, ~1 failure
	// in 31 runs). Rotating guarantees a genuinely different character
	// every time, matching what real corruption (a transmission error,
	// a mistyped digit) actually produces.
	lastChar := code[len(code)-1]
	idx := bytes.IndexByte(checksumChars, lastChar)
	if idx == -1 {
		t.Fatalf("generated code's last character %q isn't in the checksum alphabet -- Generate()/checksumChars may have diverged", string(lastChar))
	}
	rotated := checksumChars[(idx+1)%len(checksumChars)]
	corrupted := code[:len(code)-1] + string(rotated)
	if corrupted == code {
		t.Fatalf("rotation produced the same character (%q) -- checksum alphabet may have only one character", string(rotated))
	}
	if err := gen.Validate(corrupted); err == nil {
		t.Error("corrupted checksum should be rejected")
	}
}

func TestGenerator_URL(t *testing.T) {
	gen := NewGenerator(Config{
		Prefix:  "SEAM",
		Length:  12,
		BaseURL: "https://example.com/scan/",
	})

	code := "SEAMABC123DEF456"
	url := gen.URL(code)

	expected := "https://example.com/scan/SEAMABC123DEF456"
	if url != expected {
		t.Errorf("URL() = %q, want %q", url, expected)
	}
}

func TestGenerator_ParseCode(t *testing.T) {
	gen := NewGenerator(DefaultConfig())

	code, _ := gen.Generate()

	prefix, random, checksum, err := gen.ParseCode(code)
	if err != nil {
		t.Fatalf("ParseCode() error: %v", err)
	}

	if prefix != "SEAM" {
		t.Errorf("prefix = %q, want %q", prefix, "SEAM")
	}

	if len(random) != 11 { // 12 - 1 checksum
		t.Errorf("random length = %d, want 11", len(random))
	}

	if len(checksum) != 1 {
		t.Errorf("checksum length = %d, want 1", len(checksum))
	}
}

func TestGenerator_CustomConfig(t *testing.T) {
	gen := NewGenerator(Config{
		Prefix:   "ACME",
		Length:   8,
		Checksum: false,
	})

	code, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if !strings.HasPrefix(code, "ACME") {
		t.Errorf("code should start with ACME, got %s", code)
	}

	expectedLen := 4 + 8 // "ACME" + 8 chars
	if len(code) != expectedLen {
		t.Errorf("code length = %d, want %d", len(code), expectedLen)
	}
}

func TestGenerator_GenerateLabel(t *testing.T) {
	gen := NewGenerator(DefaultConfig())

	code, _ := gen.Generate()
	label := gen.GenerateLabel(code, "Test Asset", "Equipment", "A test asset")

	if label.Code != code {
		t.Errorf("Code = %q, want %q", label.Code, code)
	}

	if label.AssetName != "Test Asset" {
		t.Errorf("AssetName = %q, want %q", label.AssetName, "Test Asset")
	}

	if label.URL == "" {
		t.Error("URL should not be empty")
	}

	if label.QRData == "" {
		t.Error("QRData should not be empty")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Prefix != "SEAM" {
		t.Errorf("Prefix = %q, want %q", cfg.Prefix, "SEAM")
	}

	if cfg.Length != 12 {
		t.Errorf("Length = %d, want 12", cfg.Length)
	}

	if !cfg.Checksum {
		t.Error("Checksum should be true by default")
	}
}

func BenchmarkGenerator_Generate(b *testing.B) {
	gen := NewGenerator(DefaultConfig())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.Generate()
	}
}

func BenchmarkGenerator_Validate(b *testing.B) {
	gen := NewGenerator(DefaultConfig())
	code, _ := gen.Generate()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.Validate(code)
	}
}
