package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

// revalidatingStatic makes browsers revalidate every static file against a content ETag.
//
// Files embedded in the binary have no modification time, so http.FileServer sends no Last-Modified
// and no ETag, and nothing else about freshness: a browser then has no way to learn that a file
// changed between two builds served from the same address, and can keep running an old script (the
// uploader's "Add photos" button kept an old stylesheet's behaviour that way). With "no-cache" the
// browser asks every time, and with the ETag the answer is a cheap 304 when nothing changed.
//
// The ETag is weak (the body may later be gzipped) and is the hash of the identity content. Range
// requests and anything but a 200 pass through untouched.
func revalidatingStatic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if (r.Method != http.MethodGet && r.Method != http.MethodHead) || r.Header.Get("Range") != "" {
			next.ServeHTTP(w, r)
			return
		}
		rec := &bufferedResponse{header: http.Header{}, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		for k, v := range rec.header {
			w.Header()[k] = v
		}
		if rec.status == http.StatusOK {
			sum := sha256.Sum256(rec.body.Bytes())
			tag := `W/"` + hex.EncodeToString(sum[:8]) + `"`
			w.Header().Set("ETag", tag)
			w.Header().Set("Cache-Control", "no-cache")
			if etagMatches(r.Header.Get("If-None-Match"), tag) {
				w.Header().Del("Content-Length")
				w.Header().Del("Content-Type")
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
		w.WriteHeader(rec.status)
		if r.Method != http.MethodHead {
			w.Write(rec.body.Bytes())
		}
	})
}

// etagMatches reports whether an If-None-Match header (a list, or *) names tag, weakly.
func etagMatches(header, tag string) bool {
	if header == "" {
		return false
	}
	want := strings.TrimPrefix(tag, "W/")
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if part == "*" || strings.TrimPrefix(part, "W/") == want {
			return true
		}
	}
	return false
}

type bufferedResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (b *bufferedResponse) Header() http.Header         { return b.header }
func (b *bufferedResponse) WriteHeader(status int)      { b.status = status }
func (b *bufferedResponse) Write(p []byte) (int, error) { return b.body.Write(p) }
