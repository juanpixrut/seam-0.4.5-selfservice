package server

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"
)

const jsBody = "window.Seam = {};\n// padding padding padding padding padding padding padding padding\n"

func staticFS() http.Handler {
	return http.StripPrefix("/static/", http.FileServer(http.FS(fstest.MapFS{
		"app.js":    {Data: []byte(jsBody)},
		"empty.js":  {Data: nil},
		"font.woff": {Data: []byte("not really a font")},
		"logo.png":  {Data: []byte("not really a png")},
	})))
}

func get(h http.Handler, method, path string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func gunzip(t *testing.T, b []byte) string {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("not a gzip stream: %v", err)
	}
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("reading gzip: %v", err)
	}
	return string(out)
}

func TestGzipStatic_CompressesAndRoundTrips(t *testing.T) {
	rec := get(gzipStatic(staticFS()), "GET", "/static/app.js", map[string]string{"Accept-Encoding": "gzip"})
	if rec.Code != 200 || rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("want a gzip 200, got %d %v", rec.Code, rec.Header())
	}
	if rec.Header().Get("Content-Length") != "" {
		t.Errorf("Content-Length of the identity body must be dropped: %v", rec.Header())
	}
	if !strings.Contains(rec.Header().Get("Vary"), "Accept-Encoding") {
		t.Errorf("missing Vary: Accept-Encoding: %v", rec.Header())
	}
	if got := gunzip(t, rec.Body.Bytes()); got != jsBody {
		t.Errorf("round trip differs: %q", got)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Errorf("content type lost: %q", ct)
	}
}

// Adversarial Accept-Encoding values: everything that is not a clear "yes,
// gzip" must get the identity body.
func TestGzipStatic_HostileAcceptEncodingGetsIdentity(t *testing.T) {
	no := []string{
		"", "identity", "br", "deflate", "gzip;q=0", "gzip;q=0.0", "gzip; q=0", "gzip;Q=0",
		"*;q=0", "gzip;q=abc", "gzip;q=2", "gzip;q=-1", "gzip;q=", "gzip, gzip;q=0",
		"gzip;q=0, *", ",,,;;;", ";q=1", "gzipx", "xgzip", "g zip",
		strings.Repeat("a", 5000) + ", gzip",
		strings.Repeat("br,", 3000) + "gzip",
	}
	for _, v := range no {
		rec := get(gzipStatic(staticFS()), "GET", "/static/app.js", map[string]string{"Accept-Encoding": v})
		if rec.Header().Get("Content-Encoding") != "" || rec.Body.String() != jsBody {
			n := len(v)
			if n > 40 {
				v = v[:40] + "..."
			}
			t.Errorf("Accept-Encoding %q (len %d) must get identity, got encoding %q", v, n, rec.Header().Get("Content-Encoding"))
		}
		if !strings.Contains(rec.Header().Get("Vary"), "Accept-Encoding") {
			t.Errorf("identity response must still vary on Accept-Encoding (%q)", v)
		}
	}
	yes := []string{"gzip", "GZIP", "x-gzip", "gzip;q=1", "gzip;q=0.001", "br, gzip", "gzip, br;q=0.5", "*", "br;q=0, *;q=0.5", " gzip ; q=0.8 "}
	for _, v := range yes {
		rec := get(gzipStatic(staticFS()), "GET", "/static/app.js", map[string]string{"Accept-Encoding": v})
		if rec.Header().Get("Content-Encoding") != "gzip" || gunzip(t, rec.Body.Bytes()) != jsBody {
			t.Errorf("Accept-Encoding %q should be served gzip", v)
		}
	}
}

// Adversarial: a megabyte of header must not hang or blow up.
func TestGzipStatic_HugeHeaderIsCheap(t *testing.T) {
	start := time.Now()
	rec := get(gzipStatic(staticFS()), "GET", "/static/app.js", map[string]string{"Accept-Encoding": strings.Repeat("x;q=1,", 200000) + "gzip"})
	if rec.Header().Get("Content-Encoding") != "" {
		t.Errorf("an oversized header must be refused, got %q", rec.Header().Get("Content-Encoding"))
	}
	if time.Since(start) > time.Second {
		t.Errorf("took %v", time.Since(start))
	}
}

func TestGzipStatic_LeavesWhatItShouldNotTouch(t *testing.T) {
	h := gzipStatic(staticFS())
	gz := map[string]string{"Accept-Encoding": "gzip"}

	// Range: a byte range of the identity body, never of the gzip stream
	rec := get(h, "GET", "/static/app.js", map[string]string{"Accept-Encoding": "gzip", "Range": "bytes=0-5"})
	if rec.Code != http.StatusPartialContent || rec.Header().Get("Content-Encoding") != "" || rec.Body.String() != jsBody[:6] {
		t.Errorf("range: got %d enc=%q body=%q", rec.Code, rec.Header().Get("Content-Encoding"), rec.Body.String())
	}
	// HEAD: no body, and no claim of an encoding the body does not have
	rec = get(h, "HEAD", "/static/app.js", gz)
	if rec.Header().Get("Content-Encoding") != "" || rec.Body.Len() != 0 {
		t.Errorf("HEAD: enc=%q body=%d", rec.Header().Get("Content-Encoding"), rec.Body.Len())
	}
	// other methods pass straight through
	if rec = get(h, "POST", "/static/app.js", gz); rec.Header().Get("Content-Encoding") != "" || rec.Header().Get("Vary") != "" {
		t.Errorf("POST must be untouched: %v", rec.Header())
	}
	// already-compressed types: no encoding, no Vary
	for _, p := range []string{"/static/font.woff", "/static/logo.png"} {
		if rec = get(h, "GET", p, gz); rec.Header().Get("Content-Encoding") != "" || rec.Header().Get("Vary") != "" {
			t.Errorf("%s must not be compressed: %v", p, rec.Header())
		}
	}
	// a double extension is judged by the last one
	if rec = get(h, "GET", "/static/app.js.png", gz); rec.Header().Get("Content-Encoding") != "" {
		t.Errorf("app.js.png is a png")
	}
	// errors are not compressed
	if rec = get(h, "GET", "/static/missing.js", gz); rec.Code != 404 || rec.Header().Get("Content-Encoding") != "" {
		t.Errorf("404 must stay plain: %d %v", rec.Code, rec.Header())
	}
	// upper-case extension and a query string
	if rec = get(h, "GET", "/static/app.js?v=1", gz); rec.Header().Get("Content-Encoding") != "gzip" {
		t.Errorf("a query string must not defeat compression")
	}
}

func TestGzipStatic_EmptyFileIsAValidEmptyStream(t *testing.T) {
	rec := get(gzipStatic(staticFS()), "GET", "/static/empty.js", map[string]string{"Accept-Encoding": "gzip"})
	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("headers: %v", rec.Header())
	}
	if got := gunzip(t, rec.Body.Bytes()); got != "" {
		t.Errorf("want an empty body, got %q", got)
	}
}

func TestGzipStatic_RespectsWhatTheHandlerAlreadyDecided(t *testing.T) {
	br := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "br")
		w.Header().Set("Content-Type", "application/javascript")
		w.Write([]byte("pre-compressed"))
	})
	rec := get(gzipStatic(br), "GET", "/a.js", map[string]string{"Accept-Encoding": "gzip"})
	if rec.Header().Get("Content-Encoding") != "br" || rec.Body.String() != "pre-compressed" {
		t.Errorf("an existing Content-Encoding must be left alone: %v %q", rec.Header(), rec.Body.String())
	}
	// no WriteHeader call at all: the first Write is an implicit 200
	implicit := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(jsBody)) })
	rec = get(gzipStatic(implicit), "GET", "/a.js", map[string]string{"Accept-Encoding": "gzip"})
	if rec.Header().Get("Content-Encoding") != "gzip" || gunzip(t, rec.Body.Bytes()) != jsBody {
		t.Errorf("implicit 200 should compress: %v", rec.Header())
	}
	// a strong ETag is weakened: the bytes are no longer the identity body's
	tagged := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Etag", `"abc"`)
		w.Write([]byte(jsBody))
	})
	rec = get(gzipStatic(tagged), "GET", "/a.js", map[string]string{"Accept-Encoding": "gzip"})
	if rec.Header().Get("Etag") != `W/"abc"` {
		t.Errorf("ETag: %q", rec.Header().Get("Etag"))
	}
	// a handler that answers 304/500 explicitly is not compressed
	for _, code := range []int{304, 500} {
		code := code
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) })
		rec = get(gzipStatic(h), "GET", "/a.js", map[string]string{"Accept-Encoding": "gzip"})
		if rec.Code != code || rec.Header().Get("Content-Encoding") != "" {
			t.Errorf("status %d: got %d enc=%q", code, rec.Code, rec.Header().Get("Content-Encoding"))
		}
	}
}

// Adversarial: pooled gzip writers shared across concurrent requests must not
// bleed one response into another (run with -race).
func TestGzipStatic_ConcurrentRequestsDoNotCrossStreams(t *testing.T) {
	h := gzipStatic(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Write([]byte(r.URL.Query().Get("body")))
	}))
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body := strings.Repeat(string(rune('a'+i%26)), 200+i)
			rec := get(h, "GET", "/x.js?body="+body, map[string]string{"Accept-Encoding": "gzip"})
			if got := gunzip(t, rec.Body.Bytes()); got != body {
				t.Errorf("request %d got another request's bytes", i)
			}
		}(i)
	}
	wg.Wait()
}
