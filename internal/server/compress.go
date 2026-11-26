package server

import (
	"compress/gzip"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
)

// gzipStatic compresses text-like static files (JS, CSS, SVG, JSON, ...) for
// clients that accept gzip. Everything it is unsure about is served untouched:
//
//   - methods other than GET and HEAD, and file types that are already
//     compressed (fonts, images, archives);
//   - HEAD requests (same headers as GET would carry apart from the body);
//   - Range requests (a byte range of the identity body, not of the gzip);
//   - responses that are not a plain 200 (404, 304, 416 ...) or that already
//     carry a Content-Encoding or Content-Range;
//   - clients whose Accept-Encoding does not list gzip with a quality above 0,
//     or whose header is absurdly long.
//
// Compressible responses always carry Vary: Accept-Encoding so a shared cache
// never hands a gzip body to a client that did not ask for one.
func gzipStatic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if (r.Method != http.MethodGet && r.Method != http.MethodHead) || !compressibleExt[strings.ToLower(path.Ext(r.URL.Path))] {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Add("Vary", "Accept-Encoding")
		if r.Method == http.MethodHead || r.Header.Get("Range") != "" || !acceptsGzip(r.Header.Get("Accept-Encoding")) {
			next.ServeHTTP(w, r)
			return
		}
		gw := &gzipResponseWriter{ResponseWriter: w}
		defer gw.close()
		next.ServeHTTP(gw, r)
	})
}

var compressibleExt = map[string]bool{
	".js": true, ".css": true, ".svg": true, ".json": true, ".txt": true, ".html": true,
	".htm": true, ".xml": true, ".map": true, ".csv": true, ".md": true,
}

// maxAcceptEncodingLen bounds the header we are willing to parse.
const maxAcceptEncodingLen = 4096

// acceptsGzip reads an Accept-Encoding header (RFC 9110 §12.5.3) as far as
// gzip is concerned: an explicit "gzip" (or "x-gzip") entry decides, else "*"
// does; a quality of 0, a quality that does not parse or is outside 0..1, an
// empty header or an oversized one all mean no.
func acceptsGzip(h string) bool {
	if h == "" || len(h) > maxAcceptEncodingLen {
		return false
	}
	explicit, star := -1.0, -1.0
	for _, part := range strings.Split(h, ",") {
		name, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			continue
		}
		q := 1.0
		for _, p := range strings.Split(params, ";") {
			k, v, ok := strings.Cut(strings.TrimSpace(p), "=")
			if !ok || strings.ToLower(strings.TrimSpace(k)) != "q" {
				continue
			}
			f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil || f < 0 || f > 1 {
				f = 0
			}
			q = f
		}
		switch name {
		case "gzip", "x-gzip":
			explicit = q
		case "*":
			star = q
		}
	}
	if explicit >= 0 {
		return explicit > 0
	}
	return star > 0
}

var gzipPool = sync.Pool{New: func() any { return gzip.NewWriter(nil) }}

// gzipResponseWriter decides at WriteHeader time (or at the first Write, for a
// handler that never calls it) whether this response is compressed.
type gzipResponseWriter struct {
	http.ResponseWriter
	gz      *gzip.Writer
	decided bool
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	if w.decided {
		return
	}
	w.decided = true
	h := w.Header()
	if code == http.StatusOK && h.Get("Content-Encoding") == "" && h.Get("Content-Range") == "" {
		h.Del("Content-Length")
		if et := h.Get("Etag"); et != "" && !strings.HasPrefix(et, "W/") {
			h.Set("Etag", "W/"+et) // the bytes differ from the identity body: weaken
		}
		h.Set("Content-Encoding", "gzip")
		w.gz = gzipPool.Get().(*gzip.Writer)
		w.gz.Reset(w.ResponseWriter)
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if !w.decided {
		w.WriteHeader(http.StatusOK)
	}
	if w.gz != nil {
		return w.gz.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

// close finishes the gzip stream (an empty body still gets a valid empty one)
// and returns the writer to the pool.
func (w *gzipResponseWriter) close() {
	if w.gz != nil {
		_ = w.gz.Close()
		gzipPool.Put(w.gz)
		w.gz = nil
	}
}
