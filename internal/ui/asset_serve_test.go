package ui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/i18n"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

// fakeAssetXolu is a small stateful xolu for the end-to-end browser test: it remembers the assets
// created through the real handlers and serves them back (edit form, list). GET /__state shows what
// was created, POST /__reset forgets it.
type fakeAssetXolu struct {
	mu      sync.Mutex
	created []map[string]any
	media   []map[string]any // rows of the photo gallery's metadata entity (seam_media_items)
	blobs   map[string]fakeBlob
}

type fakeBlob struct {
	contentType string
	data        []byte
}

func (f *fakeAssetXolu) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	oql := func(rows []map[string]any) {
		if rows == nil {
			rows = []map[string]any{}
		}
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "data": rows, "stats": map[string]any{}})
	}
	switch {
	case r.URL.Path == "/__state":
		json.NewEncoder(w).Encode(f.created)
	case r.URL.Path == "/__media":
		json.NewEncoder(w).Encode(f.media)
	case r.URL.Path == "/__reset":
		f.created, f.media, f.blobs = nil, nil, nil
		w.Write([]byte("{}"))
	case r.Method == "POST" && r.URL.Path == "/api/v1/blob":
		data, _ := io.ReadAll(r.Body)
		if f.blobs == nil {
			f.blobs = map[string]fakeBlob{}
		}
		key := r.Header.Get("X-Blob-Key")
		f.blobs[key] = fakeBlob{r.Header.Get("Content-Type"), data}
		json.NewEncoder(w).Encode(map[string]any{"key": key, "size": len(data), "created": true})
	case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/v1/blob/"):
		b, ok := f.blobs[strings.TrimPrefix(r.URL.Path, "/api/v1/blob/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", b.contentType)
		w.Write(b.data)
	case r.Method == "POST" && r.URL.Path == "/api/v1/seam_media_items":
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		body["id"] = float64(len(f.media) + 1)
		f.media = append(f.media, body)
		json.NewEncoder(w).Encode(map[string]any{"id": body["id"], "message": "created"})
	case r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/api/v1/seam_media_items/"):
		id, _ := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/api/v1/seam_media_items/"))
		for i, m := range f.media {
			if m["id"] == float64(id) {
				f.media = append(f.media[:i], f.media[i+1:]...)
				break
			}
		}
		w.Write([]byte(`{"message":"deleted"}`))
	case r.URL.Path == "/api/v1/oql/query":
		var body struct {
			Query string `json:"query"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		switch {
		case strings.Contains(body.Query, "FROM seam_media_items"):
			if len(f.media) == 0 { // a fresh tenant: the entity does not exist until the first write
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "XOLU-QL004", "message": "entity 'seam_media_items' does not exist", "status": 400}})
				return
			}
			oql(append([]map[string]any(nil), f.media...))
		case strings.Contains(body.Query, "FROM asset_types"):
			oql([]map[string]any{{"id": float64(1), "name": "Pump"}, {"id": float64(2), "name": "Valve"}})
		case strings.Contains(body.Query, "FROM assets"):
			oql(append([]map[string]any(nil), f.created...))
		default:
			oql(nil)
		}
	case r.Method == "POST" && r.URL.Path == "/api/v1/assets":
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		body["id"] = float64(len(f.created) + 1)
		body["created_at"], body["updated_at"] = "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"
		f.created = append(f.created, body)
		json.NewEncoder(w).Encode(map[string]any{"id": body["id"], "message": "created"})
	case strings.HasPrefix(r.URL.Path, "/api/v1/assets/"):
		id, _ := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/api/v1/assets/"))
		if id < 1 || id > len(f.created) {
			http.NotFound(w, r)
			return
		}
		if r.Method == "PATCH" {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			for k, v := range body {
				f.created[id-1][k] = v
			}
			w.Write([]byte(`{"message":"updated"}`))
			return
		}
		json.NewEncoder(w).Encode(f.created[id-1])
	default:
		http.NotFound(w, r)
	}
}

// TestAssetForm_Serve serves the real Assets handlers over a stateful fake xolu, for
// scripts/browser-test-assets-modal.js. A dormant guard: skipped unless SEAM_ASSET_SERVE_FILE is
// set. It writes three lines to that file (the Seam URL, an auth cookie, the fake xolu URL) and runs
// until the file named <that file>.stop appears (or 150 seconds pass).
func TestAssetForm_Serve(t *testing.T) {
	out := os.Getenv("SEAM_ASSET_SERVE_FILE")
	if out == "" {
		t.Skip("SEAM_ASSET_SERVE_FILE not set")
	}
	fake := httptest.NewServer(&fakeAssetXolu{})
	defer fake.Close()
	translator, err := i18n.New("en")
	if err != nil {
		t.Fatal(err)
	}
	sessionMgr := auth.NewSessionManager(auth.SessionConfig{Secret: "test-secret-serve"})
	cfg := Config{AppName: "Test", Version: "0.0.0", Locale: "en", EnableOperational: true}
	router := NewRouter(xolu.New(fake.URL), cfg, translator, nil, sessionMgr, nil, nil, nil, nil)
	mux := http.NewServeMux()
	router.MountAssetRoutes(mux)
	mux.Handle("/", router)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	cookie := sessionCookie(t, sessionMgr, auth.RoleAdmin)
	if err := os.WriteFile(out, []byte(srv.URL+"\n"+cookie.Name+"="+cookie.Value+"\n"+fake.URL+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(150 * time.Second); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
		if _, err := os.Stat(out + ".stop"); err == nil {
			return
		}
	}
}
