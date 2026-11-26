package accounting

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

// BlobStore keeps the bytes of uploaded documents (invoices, payment proofs).
// The Document record in the DocStore keeps everything about the file except
// its content.
type BlobStore interface {
	Put(ctx context.Context, key string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
	// Delete removes a blob; a missing one is not an error.
	Delete(ctx context.Context, key string) error
}

// ErrNoBlobs is returned when a document is uploaded and no BlobStore is set.
var ErrNoBlobs = errors.New("accounting: document storage is not configured")

var blobKeyRE = regexp.MustCompile(`^[0-9a-f]{24}$`)

// FileBlobs stores each blob as one file in a directory. Keys are the random
// hex names the module generates; anything else is refused, so a key can never
// walk out of the directory.
type FileBlobs struct{ Dir string }

// NewFileBlobs returns a FileBlobs over dir, creating it if needed.
func NewFileBlobs(dir string) (*FileBlobs, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	return &FileBlobs{Dir: dir}, nil
}

func (f *FileBlobs) path(key string) (string, error) {
	if !blobKeyRE.MatchString(key) {
		return "", fmt.Errorf("accounting: invalid blob key")
	}
	return filepath.Join(f.Dir, key), nil
}

// Put writes the blob; an existing key is never overwritten.
func (f *FileBlobs) Put(_ context.Context, key string, data []byte) error {
	p, err := f.path(key)
	if err != nil {
		return err
	}
	fh, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return err
	}
	if _, err := fh.Write(data); err != nil {
		fh.Close()
		os.Remove(p)
		return err
	}
	return fh.Close()
}

// Get reads the blob.
func (f *FileBlobs) Get(_ context.Context, key string) ([]byte, error) {
	p, err := f.path(key)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return b, err
}

// Delete removes the blob; a missing one is not an error.
func (f *FileBlobs) Delete(_ context.Context, key string) error {
	p, err := f.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// MemBlobs is an in-memory BlobStore for tests.
type MemBlobs struct {
	mu sync.Mutex
	m  map[string][]byte
}

// NewMemBlobs returns an empty MemBlobs.
func NewMemBlobs() *MemBlobs { return &MemBlobs{m: map[string][]byte{}} }

// Put implements BlobStore.
func (b *MemBlobs) Put(_ context.Context, key string, data []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.m[key] = append([]byte(nil), data...)
	return nil
}

// Get implements BlobStore.
func (b *MemBlobs) Get(_ context.Context, key string) ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	v, ok := b.m[key]
	if !ok {
		return nil, ErrNotFound
	}
	return append([]byte(nil), v...), nil
}

// Delete implements BlobStore.
func (b *MemBlobs) Delete(_ context.Context, key string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.m, key)
	return nil
}

// Len returns how many blobs are held (tests).
func (b *MemBlobs) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.m)
}
