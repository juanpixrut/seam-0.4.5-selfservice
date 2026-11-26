// Package media defines the contract between Seam App and the Seam Media
// subsystem (seam-filer + thumble + fijuggler).
//
// When seam-media is not deployed, NewStub() provides a no-op implementation
// that returns ErrNotConfigured for all operations. The Document Management
// module checks for this error and operates in degraded mode: fijuggler_ref
// and thumble_asset_id remain empty, the document viewer falls back to MIME
// icons, and the "Open file manager" button is disabled.
//
// When seam-media is deployed, replace the stub with NewHTTPClient(url, key).
// See SEAM_MEDIA_SPEC.md for the full integration contract.
package media

import (
	"context"
	"errors"
	"time"
)

// ErrNotConfigured is returned by all stub methods when seam-media is not
// configured. Callers should check with errors.Is(err, media.ErrNotConfigured).
var ErrNotConfigured = errors.New("seam-media not configured")

// FileMetadata describes a file stored in seam-filer.
type FileMetadata struct {
	Path           string // relative path as fijuggler knows it
	MimeType       string // detected from file content, not extension
	SizeBytes      int64
	ContentHash    string // SHA-256 hex
	ThumbleAssetID string // thumble asset ID; empty if not yet registered
	ModifiedAt     time.Time
}

// Client is the interface Seam App uses to interact with Seam Media.
// All file and thumbnail operations go through this interface.
//
// The proxy handler and the /files route use FilerURL() and APIKey() directly
// to forward requests to seam-filer without going through the interface methods.
type Client interface {
	// IsStub returns true when this instance is the no-op stub.
	IsStub() bool

	// FilerURL returns the base URL of seam-filer, used by the proxy handler.
	FilerURL() string

	// APIKey returns the API key for authenticating requests to seam-filer.
	APIKey() string

	// FileMetadata returns extended metadata for a file in seam-filer.
	// The ref is the relative path as fijuggler knows it (fijuggler_ref).
	FileMetadata(ctx context.Context, ref string) (*FileMetadata, error)

	// ThumbleAssetID returns the thumble asset ID for a file given its
	// fijuggler_ref. Returns "" if the file has not been registered with
	// thumble yet — this is not an error condition.
	ThumbleAssetID(ctx context.Context, ref string) (string, error)

	// FileContentURL returns the URL to access the raw file content.
	// May be a direct seam-filer URL or a signed S3 URL depending on
	// the storage backend configuration.
	FileContentURL(ctx context.Context, ref string) (string, error)
}

// stubClient implements Client returning ErrNotConfigured for all operations.
type stubClient struct{}

func (s *stubClient) IsStub() bool     { return true }
func (s *stubClient) FilerURL() string { return "" }
func (s *stubClient) APIKey() string   { return "" }

func (s *stubClient) FileMetadata(_ context.Context, _ string) (*FileMetadata, error) {
	return nil, ErrNotConfigured
}

func (s *stubClient) ThumbleAssetID(_ context.Context, _ string) (string, error) {
	return "", ErrNotConfigured
}

func (s *stubClient) FileContentURL(_ context.Context, _ string) (string, error) {
	return "", ErrNotConfigured
}

// NewStub returns a Client that reports ErrNotConfigured for all operations.
// This is the default in Seam App when seam-media is not configured.
func NewStub() Client { return &stubClient{} }

// NewHTTPClient returns a real Client that communicates with seam-filer via HTTP.
// Use this when seam-media is deployed and configured.
// See SEAM_MEDIA_SPEC.md §9 for the full integration contract.
func NewHTTPClient(filerURL, apiKey string) Client {
	return &httpClient{filerURL: filerURL, apiKey: apiKey}
}

// httpClient is the real implementation. Populated when seam-media is available.
// The body of each method is a placeholder — implement against seam-filer's
// GET /api/info endpoint when integrating.
type httpClient struct {
	filerURL string
	apiKey   string
}

func (c *httpClient) IsStub() bool     { return false }
func (c *httpClient) FilerURL() string { return c.filerURL }
func (c *httpClient) APIKey() string   { return c.apiKey }

func (c *httpClient) FileMetadata(_ context.Context, _ string) (*FileMetadata, error) {
	// TODO: implement — GET {filerURL}/api/info?path={ref}
	// with Authorization: Bearer {apiKey} and X-Seam-Tenant header
	return nil, errors.New("httpClient.FileMetadata not yet implemented")
}

func (c *httpClient) ThumbleAssetID(ctx context.Context, ref string) (string, error) {
	meta, err := c.FileMetadata(ctx, ref)
	if err != nil {
		return "", err
	}
	return meta.ThumbleAssetID, nil
}

func (c *httpClient) FileContentURL(_ context.Context, ref string) (string, error) {
	// TODO: implement — return proxy URL or signed S3 URL
	// For MVP: return {filerURL}/api/files/get?path={ref}
	return c.filerURL + "/api/files/get?path=" + ref, nil
}
