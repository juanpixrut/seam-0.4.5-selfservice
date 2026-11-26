// Package qrcode provides QR code generation and management for Seam AMS.
package qrcode

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Common errors
var (
	ErrInvalidCode  = errors.New("invalid QR code")
	ErrCodeNotFound = errors.New("QR code not found")
	ErrCodeExpired  = errors.New("QR code expired")
	ErrCodeInUse    = errors.New("QR code already in use")
)

// Code represents a QR code for asset identification.
type Code struct {
	ID          int64     `json:"id"`
	TenantID    int64     `json:"tenant_id"`
	Code        string    `json:"code"`
	AssetID     int64     `json:"asset_id,omitempty"`
	AssetTypeID int64     `json:"asset_type_id,omitempty"`
	Status      string    `json:"status"` // available, assigned, disabled
	BatchID     string    `json:"batch_id,omitempty"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	AssignedAt  time.Time `json:"assigned_at,omitempty"`
}

// Status constants
const (
	StatusAvailable = "available"
	StatusAssigned  = "assigned"
	StatusDisabled  = "disabled"
)

// Config holds QR code configuration.
type Config struct {
	// Code format settings
	Prefix   string // Prefix for codes (e.g., "SEAM")
	Length   int    // Length of random part
	Checksum bool   // Include checksum digit

	// URL settings
	BaseURL string // Base URL for scan redirect

	// Expiry settings
	DefaultTTL time.Duration // Default time-to-live for codes
}

// DefaultConfig returns default QR code configuration.
func DefaultConfig() Config {
	return Config{
		Prefix:     "SEAM",
		Length:     12,
		Checksum:   true,
		BaseURL:    "https://seam.app/s/",
		DefaultTTL: 0, // No expiry
	}
}

// Generator generates unique QR codes.
type Generator struct {
	config Config
}

// NewGenerator creates a new QR code generator.
func NewGenerator(cfg Config) *Generator {
	return &Generator{config: cfg}
}

// Generate creates a new unique QR code.
func (g *Generator) Generate() (string, error) {
	// Generate random bytes
	randomLen := g.config.Length
	if g.config.Checksum {
		randomLen-- // Reserve space for checksum
	}

	bytes := make([]byte, randomLen)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generating random bytes: %w", err)
	}

	// Encode as base32-like (alphanumeric, avoiding confusing chars)
	code := g.config.Prefix + encodeAlphanumeric(bytes, randomLen)

	// Add checksum if enabled
	if g.config.Checksum {
		code += g.checksum(code)
	}

	return strings.ToUpper(code), nil
}

// GenerateBatch creates multiple unique QR codes.
func (g *Generator) GenerateBatch(count int) ([]string, error) {
	codes := make([]string, count)
	seen := make(map[string]bool)

	for i := 0; i < count; i++ {
		for attempts := 0; attempts < 100; attempts++ {
			code, err := g.Generate()
			if err != nil {
				return nil, err
			}

			if !seen[code] {
				seen[code] = true
				codes[i] = code
				break
			}
		}

		if codes[i] == "" {
			return nil, errors.New("failed to generate unique code")
		}
	}

	return codes, nil
}

// Validate checks if a QR code is valid.
func (g *Generator) Validate(code string) error {
	code = strings.ToUpper(strings.TrimSpace(code))

	// Check prefix
	if !strings.HasPrefix(code, g.config.Prefix) {
		return ErrInvalidCode
	}

	// Check length
	expectedLen := len(g.config.Prefix) + g.config.Length
	if len(code) != expectedLen {
		return ErrInvalidCode
	}

	// Verify checksum if enabled
	if g.config.Checksum {
		expectedChecksum := g.checksum(code[:len(code)-1])
		actualChecksum := string(code[len(code)-1])
		if expectedChecksum != actualChecksum {
			return ErrInvalidCode
		}
	}

	return nil
}

// URL returns the full scan URL for a code.
func (g *Generator) URL(code string) string {
	return g.config.BaseURL + code
}

// checksum calculates a simple checksum digit.
func (g *Generator) checksum(code string) string {
	sum := 0
	for i, c := range code {
		sum += int(c) * (i + 1)
	}
	return string(checksumChars[sum%len(checksumChars)])
}

// Character set for code generation (no 0/O, 1/I/L confusion)
var codeChars = []byte("23456789ABCDEFGHJKMNPQRSTUVWXYZ")
var checksumChars = []byte("0123456789ABCDEFGHJKMNPQRSTUVWXYZ")

// encodeAlphanumeric encodes bytes as alphanumeric string.
func encodeAlphanumeric(data []byte, length int) string {
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		idx := int(data[i%len(data)]) % len(codeChars)
		result[i] = codeChars[idx]
	}
	return string(result)
}

// ParseCode extracts components from a QR code.
func (g *Generator) ParseCode(code string) (prefix, random, checksum string, err error) {
	if err := g.Validate(code); err != nil {
		return "", "", "", err
	}

	prefix = g.config.Prefix
	if g.config.Checksum {
		random = code[len(prefix) : len(code)-1]
		checksum = string(code[len(code)-1])
	} else {
		random = code[len(prefix):]
	}

	return prefix, random, checksum, nil
}

// ScanResult represents the result of scanning a QR code.
type ScanResult struct {
	Code      string                 `json:"code"`
	Valid     bool                   `json:"valid"`
	AssetID   int64                  `json:"asset_id,omitempty"`
	Asset     map[string]interface{} `json:"asset,omitempty"`
	Error     string                 `json:"error,omitempty"`
	ScannedAt time.Time              `json:"scanned_at"`
}

// ScanRecord records a QR code scan event.
type ScanRecord struct {
	ID        int64     `json:"id"`
	Code      string    `json:"code"`
	AssetID   int64     `json:"asset_id,omitempty"`
	TenantID  int64     `json:"tenant_id"`
	Location  *Location `json:"location,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
	IPAddress string    `json:"ip_address,omitempty"`
	ScannedAt time.Time `json:"scanned_at"`
}

// Location represents geographic coordinates.
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Accuracy  float64 `json:"accuracy,omitempty"`
}

// Label represents printable label data.
type Label struct {
	Code        string `json:"code"`
	URL         string `json:"url"`
	AssetName   string `json:"asset_name,omitempty"`
	AssetType   string `json:"asset_type,omitempty"`
	Description string `json:"description,omitempty"`
	QRData      string `json:"qr_data"` // Base64 encoded QR image
}

// GenerateLabel creates label data for printing.
func (g *Generator) GenerateLabel(code string, assetName, assetType, description string) *Label {
	return &Label{
		Code:        code,
		URL:         g.URL(code),
		AssetName:   assetName,
		AssetType:   assetType,
		Description: description,
		QRData:      generateQRBase64(g.URL(code)),
	}
}

// generateQRBase64 generates a QR code image as base64.
// In production, use a proper QR library like github.com/skip2/go-qrcode
func generateQRBase64(data string) string {
	// Placeholder - returns empty string
	// Real implementation would generate actual QR image
	placeholder := fmt.Sprintf("QR:%s", data)
	return base64.StdEncoding.EncodeToString([]byte(placeholder))
}
