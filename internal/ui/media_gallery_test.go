package ui

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// newMultipartUploadRequest builds a real multipart/form-data POST
// request the way a browser's FormData would -- image bytes are a
// tiny, real (if trivial) byte sequence, not a placeholder string,
// since GalleryUpload actually reads and sha256-hashes the body.
func newMultipartUploadRequest(t *testing.T, fields map[string]string, filename string, fileBytes []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("WriteField(%s): %v", k, err)
		}
	}
	if filename != "" {
		part, err := w.CreateFormFile("file", filename)
		if err != nil {
			t.Fatalf("CreateFormFile: %v", err)
		}
		if _, err := part.Write(fileBytes); err != nil {
			t.Fatalf("write file bytes: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	req := httptest.NewRequest("POST", "/gallery/upload", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

// TestGalleryList_EntityNotYetCreated_ReturnsEmptyNotError is the
// same fix T-28 already established for FK-sourced select fields
// (asset_source_resolver.go's entityNotYetCreated, reused directly
// here), applied to the gallery: a genuinely fresh tenant has never
// created a seam_media_items row, so xolu's OQL engine rejects a
// SELECT against a table that doesn't exist yet. Found live against a
// real xolu instance during this widget's build (not by inspection) --
// without this handling, the very first image upload for any tenant
// would fail outright, since GalleryUpload's own nextGalleryPosition
// call runs before its first Create of that entity ever happens.
func TestGalleryList_EntityNotYetCreated_ReturnsEmptyNotError(t *testing.T) {
	server := newTestOQLErrorServer(t, 400, "XOLU-QL004", "entity 'seam_media_items' does not exist")
	defer server.Close()
	h := newHandler(t, server)

	req := httptest.NewRequest("GET", "/gallery/items?owner_type=asset&owner_id=42&owner_field=photos", nil)
	rec := httptest.NewRecorder()
	h.GalleryList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200 (empty gallery, not an error); body: %s", rec.Code, rec.Body.String())
	}
	var items []mediaItem
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("got %d items, want 0", len(items))
	}
}

// TestGalleryUpload_FirstEverUpload_EntityNotYetCreated confirms the
// actual bug's real shape: GalleryUpload's own position-lookup call
// must not fail just because this is the very first upload ever for a
// tenant.
func TestGalleryUpload_FirstEverUpload_EntityNotYetCreated(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{
		"POST /api/v1/oql/query": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code": "XOLU-QL004", "message": "entity 'seam_media_items' does not exist", "status": 400,
				},
			})
		},
		"POST /api/v1/blob": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"key": r.Header.Get("X-Blob-Key"), "sha256": "abc123", "size": 0, "created": true,
			})
		},
		"POST /api/v1/seam_media_items": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"id": 1, "message": "created"})
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	req := newMultipartUploadRequest(t, map[string]string{
		"owner_type": "asset", "owner_id": "1", "owner_field": "photos",
	}, "photo.jpg", []byte("fake-jpeg-bytes"))
	rec := httptest.NewRecorder()
	h.GalleryUpload(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("first-ever upload: got status %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var item mediaItem
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}
	if item.Position != 0 {
		t.Errorf("first-ever upload's position = %d, want 0", item.Position)
	}
}

// TestGalleryUpload_UnknownOwnerType_Returns400 confirms owner_type is
// checked against the allowlist before anything else runs -- no blob
// write, no entity create for a request naming an owner_type this
// file doesn't recognise.
func TestGalleryUpload_UnknownOwnerType_Returns400(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{})
	defer server.Close()
	h := newHandler(t, server)

	req := newMultipartUploadRequest(t, map[string]string{
		"owner_type": "not_a_real_type", "owner_id": "1", "owner_field": "photos",
	}, "photo.jpg", []byte("fake-jpeg-bytes"))
	rec := httptest.NewRecorder()
	h.GalleryUpload(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("unknown owner_type: got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// TestGalleryUpload_InvalidOwnerField_Returns400 is the injection-
// safety test: an owner_field shaped to break out of the OQL WHERE
// clause listGalleryItems (and nextGalleryPosition, which it backs)
// builds by string interpolation must be rejected before it ever
// reaches a query string, not escaped after the fact.
func TestGalleryUpload_InvalidOwnerField_Returns400(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{})
	defer server.Close()
	h := newHandler(t, server)

	adversarial := "photos' OR '1'='1"
	req := newMultipartUploadRequest(t, map[string]string{
		"owner_type": "asset", "owner_id": "1", "owner_field": adversarial,
	}, "photo.jpg", []byte("fake-jpeg-bytes"))
	rec := httptest.NewRecorder()
	h.GalleryUpload(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("adversarial owner_field: got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// TestGalleryUpload_ZeroOrNegativeOwnerID_Returns400 confirms the
// owner_id boundary: zero and negative are rejected, not treated as
// "no owner" or silently coerced.
func TestGalleryUpload_ZeroOrNegativeOwnerID_Returns400(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{})
	defer server.Close()
	h := newHandler(t, server)

	for _, id := range []string{"0", "-1", "not-a-number"} {
		req := newMultipartUploadRequest(t, map[string]string{
			"owner_type": "asset", "owner_id": id, "owner_field": "photos",
		}, "photo.jpg", []byte("fake-jpeg-bytes"))
		rec := httptest.NewRecorder()
		h.GalleryUpload(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("owner_id=%q: got status %d, want %d", id, rec.Code, http.StatusBadRequest)
		}
	}
}

// TestGalleryUpload_Success_StoresBlobAndMetadata is the full
// round-trip: a valid upload writes the blob, creates the metadata
// entity row, and returns the item's JSON with a working blob URL.
func TestGalleryUpload_Success_StoresBlobAndMetadata(t *testing.T) {
	var blobPutCalled, entityCreateCalled bool
	server := mockOLU(t, map[string]http.HandlerFunc{
		"POST /api/v1/blob": func(w http.ResponseWriter, r *http.Request) {
			blobPutCalled = true
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"key": r.Header.Get("X-Blob-Key"), "sha256": "abc123", "size": 0, "created": true,
			})
		},
		"POST /api/v1/seam_media_items": func(w http.ResponseWriter, r *http.Request) {
			entityCreateCalled = true
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"id": 7, "message": "created"})
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	req := newMultipartUploadRequest(t, map[string]string{
		"owner_type": "asset", "owner_id": "42", "owner_field": "photos",
	}, "photo.jpg", []byte("fake-jpeg-bytes"))
	rec := httptest.NewRecorder()
	h.GalleryUpload(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if !blobPutCalled {
		t.Error("expected BlobPut to be called")
	}
	if !entityCreateCalled {
		t.Error("expected the seam_media_items entity to be created")
	}
	var item mediaItem
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}
	if item.ID != 7 {
		t.Errorf("item.ID = %d, want 7", item.ID)
	}
	if item.OwnerType != "asset" || item.OwnerID != 42 || item.OwnerField != "photos" {
		t.Errorf("owner fields not round-tripped correctly: %+v", item)
	}
	if !strings.Contains(item.URL, "/gallery/blob?key=") {
		t.Errorf("item.URL = %q, expected a /gallery/blob proxy URL", item.URL)
	}
	if item.SizeBytes != int64(len("fake-jpeg-bytes")) {
		t.Errorf("item.SizeBytes = %d, want %d (must come from the actual read bytes, not BlobPutResult.Size, which xolu's own server never populates)", item.SizeBytes, len("fake-jpeg-bytes"))
	}
}

// TestGalleryList_UnknownOwnerType_Returns400 mirrors the upload-side
// validation test for the list endpoint.
func TestGalleryList_UnknownOwnerType_Returns400(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{})
	defer server.Close()
	h := newHandler(t, server)

	req := httptest.NewRequest("GET", "/gallery/items?owner_type=bogus&owner_id=1&owner_field=photos", nil)
	rec := httptest.NewRecorder()
	h.GalleryList(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// TestGalleryList_Success_ReturnsItemsInPositionOrder confirms the
// OQL query's own field mapping round-trips correctly and items come
// back ordered by position.
func TestGalleryList_Success_ReturnsItemsInPositionOrder(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{
		"POST /api/v1/oql/query": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"status": "ok",
				"data": []map[string]any{
					{"id": 1, "owner_type": "asset", "owner_id": 42, "owner_field": "photos",
						"blob_key": "gallery/asset/42/photos/hash1", "content_type": "image/jpeg",
						"size_bytes": 100, "sha256": "hash1", "position": 0,
						"uploaded_by": "a@b.com", "uploaded_at": "2026-08-17T00:00:00Z"},
					{"id": 2, "owner_type": "asset", "owner_id": 42, "owner_field": "photos",
						"blob_key": "gallery/asset/42/photos/hash2", "content_type": "image/jpeg",
						"size_bytes": 200, "sha256": "hash2", "position": 1,
						"uploaded_by": "a@b.com", "uploaded_at": "2026-08-17T00:01:00Z"},
				},
			})
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	req := httptest.NewRequest("GET", "/gallery/items?owner_type=asset&owner_id=42&owner_field=photos", nil)
	rec := httptest.NewRecorder()
	h.GalleryList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var items []mediaItem
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Position != 0 || items[1].Position != 1 {
		t.Errorf("items not in position order: %+v", items)
	}
}

// TestGalleryDelete_InvalidID_Returns400 confirms a non-numeric or
// non-positive id is rejected before any Delete call is attempted.
func TestGalleryDelete_InvalidID_Returns400(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{})
	defer server.Close()
	h := newHandler(t, server)

	for _, id := range []string{"0", "-1", "abc", ""} {
		qs := url.Values{"id": {id}}.Encode()
		req := httptest.NewRequest("DELETE", "/gallery/item?"+qs, nil)
		rec := httptest.NewRecorder()
		h.GalleryDelete(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("id=%q: got status %d, want %d", id, rec.Code, http.StatusBadRequest)
		}
	}
}

// TestGalleryDelete_Success confirms a valid id calls through to
// xolu's Delete and returns the expected confirmation JSON.
func TestGalleryDelete_Success(t *testing.T) {
	var deleteCalled bool
	server := mockOLU(t, map[string]http.HandlerFunc{
		"DELETE /api/v1/seam_media_items/7": func(w http.ResponseWriter, r *http.Request) {
			deleteCalled = true
			w.WriteHeader(http.StatusOK)
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	req := httptest.NewRequest("DELETE", "/gallery/item?id=7", nil)
	rec := httptest.NewRecorder()
	h.GalleryDelete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if !deleteCalled {
		t.Error("expected xolu's Delete endpoint to be called")
	}
}

// TestGalleryBlobProxy_MissingKey_Returns400 confirms an empty key is
// rejected before any call to xolu.
func TestGalleryBlobProxy_MissingKey_Returns400(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{})
	defer server.Close()
	h := newHandler(t, server)

	req := httptest.NewRequest("GET", "/gallery/blob", nil)
	rec := httptest.NewRecorder()
	h.GalleryBlobProxy(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// TestGalleryBlobProxy_Success_StreamsContentType confirms the proxy
// forwards the blob's own content-type and body without requiring the
// browser to ever talk to xolu directly.
func TestGalleryBlobProxy_Success_StreamsContentType(t *testing.T) {
	server := mockOLU(t, map[string]http.HandlerFunc{
		"GET /api/v1/blob/": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write([]byte("fake-jpeg-bytes"))
		},
	})
	defer server.Close()
	h := newHandler(t, server)

	req := httptest.NewRequest("GET", "/gallery/blob?key=gallery-asset-42-photos-hash1", nil)
	rec := httptest.NewRecorder()
	h.GalleryBlobProxy(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Errorf("Content-Type = %q, want image/jpeg", got)
	}
	if rec.Body.String() != "fake-jpeg-bytes" {
		t.Errorf("body = %q, want the blob's actual bytes", rec.Body.String())
	}
}
