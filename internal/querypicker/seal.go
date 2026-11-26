package querypicker

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"sync"
)

// A Spec reaches the browser sealed: compressed, then encrypted and
// authenticated with a key only the server holds. The browser sends the token
// back with each search; it can neither read the query nor make one of its own.
//
// The key is random per process unless SetKey installs one derived from the
// deployment's secret, which is what lets several instances, or one instance
// across a restart, open each other's tokens.

// MaxTokenChars bounds a token so it fits a request URL.
const MaxTokenChars = 6000

var (
	keyMu sync.RWMutex
	key   = randomKey()
	aad   = []byte("seam-query-picker-v1")
)

// ErrToken is returned for a token this server did not seal.
var ErrToken = errors.New("querypicker: invalid token")

func randomKey() []byte {
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		panic("querypicker: no random source: " + err.Error())
	}
	return k
}

// SetKey derives the sealing key from secret. An empty secret keeps the
// per-process random key.
func SetKey(secret []byte) {
	if len(secret) == 0 {
		return
	}
	sum := sha256.Sum256(append(append([]byte{}, aad...), secret...))
	keyMu.Lock()
	key = sum[:]
	keyMu.Unlock()
}

func gcm() (cipher.AEAD, error) {
	keyMu.RLock()
	k := key
	keyMu.RUnlock()
	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// Seal returns the token for a validated Spec.
func Seal(s *Spec) (string, error) {
	if err := s.ready(); err != nil {
		return "", err
	}
	plain, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	var zipped bytes.Buffer
	zw := gzip.NewWriter(&zipped)
	if _, err := zw.Write(plain); err != nil {
		return "", err
	}
	if err := zw.Close(); err != nil {
		return "", err
	}
	aead, err := gcm()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(aead.Seal(nonce, nonce, zipped.Bytes(), aad))
	if len(token) > MaxTokenChars {
		return "", errors.New("querypicker: the field's query and columns are too large")
	}
	return token, nil
}

// Open returns the Spec a token seals, validated again.
func Open(token string) (*Spec, error) {
	if token == "" || len(token) > MaxTokenChars {
		return nil, ErrToken
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, ErrToken
	}
	aead, err := gcm()
	if err != nil {
		return nil, err
	}
	if len(raw) < aead.NonceSize() {
		return nil, ErrToken
	}
	zipped, err := aead.Open(nil, raw[:aead.NonceSize()], raw[aead.NonceSize():], aad)
	if err != nil {
		return nil, ErrToken
	}
	zr, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		return nil, ErrToken
	}
	plain, err := io.ReadAll(io.LimitReader(zr, 64<<10))
	if err != nil {
		return nil, ErrToken
	}
	var s Spec
	if err := json.Unmarshal(plain, &s); err != nil {
		return nil, ErrToken
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return &s, nil
}
