package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func serveStatic(t *testing.T, files fstest.MapFS, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	h := revalidatingStatic(http.FileServer(http.FS(files)))
	req := httptest.NewRequest("GET", path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// Embedded files have no modification time, so without this a browser could keep an old script from
// another build served at the same address: every static file is revalidated against its content.
func TestRevalidatingStatic_ETagNoCacheAnd304(t *testing.T) {
	files := fstest.MapFS{"js/a.js": {Data: []byte("console.log(1)")}}
	first := serveStatic(t, files, "/js/a.js", nil)
	tag := first.Header().Get("ETag")
	if first.Code != 200 || tag == "" || first.Header().Get("Cache-Control") != "no-cache" || first.Body.String() != "console.log(1)" {
		t.Fatalf("first response: %d tag=%q cc=%q body=%q", first.Code, tag, first.Header().Get("Cache-Control"), first.Body.String())
	}
	again := serveStatic(t, files, "/js/a.js", map[string]string{"If-None-Match": tag})
	if again.Code != http.StatusNotModified || again.Body.Len() != 0 {
		t.Errorf("unchanged file must answer 304 with no body, got %d (%d bytes)", again.Code, again.Body.Len())
	}
	files["js/a.js"] = &fstest.MapFile{Data: []byte("console.log(2)")}
	changed := serveStatic(t, files, "/js/a.js", map[string]string{"If-None-Match": tag})
	if changed.Code != 200 || changed.Header().Get("ETag") == tag || changed.Body.String() != "console.log(2)" {
		t.Errorf("a changed file must be sent again with a new ETag, got %d tag=%q", changed.Code, changed.Header().Get("ETag"))
	}
}

func TestRevalidatingStatic_LeavesOtherResponsesAlone(t *testing.T) {
	files := fstest.MapFS{"js/a.js": {Data: []byte("0123456789")}}
	if rec := serveStatic(t, files, "/js/missing.js", nil); rec.Code != 404 || rec.Header().Get("ETag") != "" {
		t.Errorf("404 must pass through untouched, got %d etag=%q", rec.Code, rec.Header().Get("ETag"))
	}
	if rec := serveStatic(t, files, "/js/a.js", map[string]string{"Range": "bytes=0-3"}); rec.Code != http.StatusPartialContent || rec.Header().Get("ETag") != "" || rec.Body.String() != "0123" {
		t.Errorf("Range must pass through untouched, got %d etag=%q body=%q", rec.Code, rec.Header().Get("ETag"), rec.Body.String())
	}
}
