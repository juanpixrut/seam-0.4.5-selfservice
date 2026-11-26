package ui

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Backend for the "image" formengine widget (T-10). Storage is xolu
// /blob (content-addressed) for the actual bytes, plus a small
// metadata entity (mediaItemsEntity, below) for everything /blob's own
// minimal metadata (key/hash/size/content-type/stored-at, see xolu's
// BlobMeta) doesn't carry: which record and field a file belongs to,
// display order, and who uploaded it. This is the abstraction the tech team
// asked for directly during design: "use blobs in xolu, with enough
// metadata to manage them hierarchically, and to find them and
// retrieve them when we need them... gallery invariants, and
// integration with the Documents module later."
//
// Deliberately a separate package boundary from internal/media, not a
// second implementation of the same thing: internal/media is the
// future contract for the real seam-media stack (seam-filer, thumble,
// fijuggler -- see that package's own doc comment), currently a stub
// returning ErrNotConfigured everywhere. This file is what backs the
// "image" widget TODAY, using xolu directly, structured so that a
// future migration to route through media.Client instead of blobs
// directly is a backend swap behind the same {list, upload, delete}
// shape -- not a widget rewrite. No code here imports internal/media
// or vice versa; the two are intentionally uncoupled until that
// migration is a real, scheduled piece of work.
//
// Injection-safety discipline matches asset_picker_search.go's own
// established rule exactly: owner_type is checked against a hardcoded
// allowlist (never interpolated from the client unchecked), owner_id
// is always parsed as an int64 before use, and owner_field is
// validated against a strict identifier pattern before it can appear
// in an OQL WHERE clause. None of the three ever reach OQL as raw,
// unvalidated client input.

const mediaItemsEntity = "seam_media_items"

// galleryOwnerTypes is the allowlist of entity types the gallery may
// be attached to. Assets is the only real, anticipated consumer right
// now (no schema uses the "image" widget yet, matching every other
// T-10 addition this session -- capability, not content). Extending
// this list is how a future consumer opts in; an unrecognised
// owner_type is a 400, not silently accepted.
//
// Singular ("asset"), not the plural xolu table name ("assets") --
// owner_type here is purely this file's own filtering label (never
// used as an actual xolu entity/table name; the only entity this file
// ever queries directly is mediaItemsEntity itself), so it matches
// RenderContext.EntityType's own established convention
// (engine.go: "'asset' | 'sensor' | 'custodio' | ...") instead of
// introducing a second, competing naming scheme renderImage would
// have to translate between.
var galleryOwnerTypes = map[string]bool{
	"asset": true,
}

// ownerFieldPattern is a conservative identifier shape (matches Go/SQL
// identifier conventions already used throughout this codebase's own
// field names) -- rejects anything that could plausibly break out of
// an OQL string literal or column reference.
var ownerFieldPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// mediaItem is the JSON shape returned to the widget's own JS for both
// the upload response and the list response -- one consistent shape,
// so the client-side code has a single type to render either way.
type mediaItem struct {
	ID          int64  `json:"id"`
	OwnerType   string `json:"owner_type"`
	OwnerID     int64  `json:"owner_id"`
	OwnerField  string `json:"owner_field"`
	BlobKey     string `json:"blob_key"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	SHA256      string `json:"sha256"`
	Position    int    `json:"position"`
	UploadedBy  string `json:"uploaded_by"`
	UploadedAt  string `json:"uploaded_at"`
	URL         string `json:"url"`
}

// maxGalleryUploadBytes bounds a single image upload. 20MB is
// generous for a phone photo (typical JPEG: 2-8MB) with headroom, not
// a considered, tuned limit -- revisit if a real deployment's camera
// output routinely exceeds it.
const maxGalleryUploadBytes = 20 << 20

func validateGalleryOwner(ownerType, ownerIDStr, ownerField string) (int64, error) {
	if !galleryOwnerTypes[ownerType] {
		return 0, fmt.Errorf("unknown owner_type %q", ownerType)
	}
	ownerID, err := strconv.ParseInt(ownerIDStr, 10, 64)
	if err != nil || ownerID <= 0 {
		return 0, fmt.Errorf("invalid owner_id %q", ownerIDStr)
	}
	if !ownerFieldPattern.MatchString(ownerField) {
		return 0, fmt.Errorf("invalid owner_field %q", ownerField)
	}
	return ownerID, nil
}

func galleryBlobURL(key string) string {
	return "/gallery/blob?key=" + url.QueryEscape(key)
}

// GalleryUpload handles POST /gallery/upload (multipart form: file,
// owner_type, owner_id, owner_field). Stores the file in xolu /blob
// under a namespaced key (not content-addressed alone -- see blobKey
// below for why) and creates the metadata row, returning the new
// item's full mediaItem JSON for the widget to append to its gallery
// without a page reload.
func (h *UIHandler) GalleryUpload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseMultipartForm(maxGalleryUploadBytes); err != nil {
		http.Error(w, h.errText(r, "error.file_too_large", "file too large or invalid form"), http.StatusBadRequest)
		return
	}

	ownerType := r.FormValue("owner_type")
	ownerField := r.FormValue("owner_field")
	ownerID, err := validateGalleryOwner(ownerType, r.FormValue("owner_id"), ownerField)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, h.errText(r, "error.missing_file", "missing file"), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read fully: needed to compute the content hash for the blob key
	// and to size-check before ever calling BlobPut, and cheap at this
	// size ceiling (20MB) -- avoids a partial upload landing in blob
	// storage if the size check fails partway through a streamed write.
	data, err := io.ReadAll(io.LimitReader(file, maxGalleryUploadBytes+1))
	if err != nil {
		http.Error(w, h.errText(r, "error.upload_failed", "failed to read upload"), http.StatusInternalServerError)
		return
	}
	if len(data) > maxGalleryUploadBytes {
		http.Error(w, h.errText(r, "error.file_too_large", "file too large"), http.StatusRequestEntityTooLarge)
		return
	}

	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Flat key incorporating owner info -- NOT slash-delimited. xolu's
	// own blob keys are flat, not hierarchical (pkg/client/blob.go's
	// own validateBlobKey doc comment: "Keys are FLAT, not hierarchical
	// -- there is no folder/prefix structure at the storage layer");
	// a '/' or '\' anywhere in the key is rejected outright. An earlier
	// version of this comment claimed a slash-delimited key would
	// support "hierarchical browsing via BlobList's own prefix filter"
	// -- wrong, caught by this file's own test suite actually hitting
	// xolu's real validation (TestGalleryUpload_Success_
	// StoresBlobAndMetadata), not by re-reading the docs first. This
	// package never calls BlobList at all, so that reasoning was both
	// incorrect and describing a code path that doesn't exist. The
	// metadata entity (below) is the actual, sole authoritative index
	// for any owner/field-scoped lookup (listGalleryItems's own OQL
	// query) -- this key's only real job is being a stable, human-
	// legible, collision-resistant identifier for the blob itself.
	blobKey := fmt.Sprintf("gallery-%s-%d-%s-%s", ownerType, ownerID, ownerField, hash)

	if _, err := h.xoluClient.BlobPut(ctx, blobKey, contentType, bytes.NewReader(data)); err != nil {
		h.logger.Error("gallery blob upload failed", "error", err, "owner_type", ownerType, "owner_id", ownerID)
		http.Error(w, h.errText(r, "error.upload_failed", "upload failed"), http.StatusInternalServerError)
		return
	}

	position, err := h.nextGalleryPosition(ctx, ownerType, ownerID, ownerField)
	if err != nil {
		h.logger.Error("gallery position lookup failed", "error", err)
		http.Error(w, h.errText(r, "error.upload_failed", "upload failed"), http.StatusInternalServerError)
		return
	}

	uploadedBy := ""
	if u := h.getCurrentUser(r); u != nil {
		uploadedBy = u.Email
	}
	uploadedAt := time.Now().UTC().Format(time.RFC3339)

	entityData := map[string]any{
		"owner_type":   ownerType,
		"owner_id":     ownerID,
		"owner_field":  ownerField,
		"blob_key":     blobKey,
		"content_type": contentType,
		// len(data), not BlobPut's own response Size field -- xolu's
		// own docs (JSON_SCHEMA.md via BlobPutResult's doc comment)
		// record that the server's PUT response never populates Size,
		// a known gap in xolu itself, not something to route around
		// with an extra BlobHead round-trip when we already have the
		// exact byte count in hand from the read above.
		"size_bytes":  int64(len(data)),
		"sha256":      hash,
		"position":    position,
		"uploaded_by": uploadedBy,
		"uploaded_at": uploadedAt,
	}

	created, err := h.xoluClient.Create(ctx, mediaItemsEntity, entityData)
	if err != nil {
		h.logger.Error("gallery metadata create failed", "error", err)
		http.Error(w, h.errText(r, "error.upload_failed", "upload failed"), http.StatusInternalServerError)
		return
	}

	item := mediaItem{
		ID: created.ID, OwnerType: ownerType, OwnerID: ownerID, OwnerField: ownerField,
		BlobKey: blobKey, ContentType: contentType, SizeBytes: int64(len(data)), SHA256: hash,
		Position: position, UploadedBy: uploadedBy, UploadedAt: uploadedAt,
		URL: galleryBlobURL(blobKey),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

// nextGalleryPosition returns len(existing items) for this
// owner+field -- new uploads append to the end of the gallery. Not
// safe against a genuine race (two simultaneous uploads to the same
// field could compute the same position) -- acceptable for now, since
// position is a display-order hint, not a uniqueness constraint
// anything else depends on; a collision means two items sort adjacent
// rather than one being lost or corrupted.
func (h *UIHandler) nextGalleryPosition(ctx context.Context, ownerType string, ownerID int64, ownerField string) (int, error) {
	items, err := h.listGalleryItems(ctx, ownerType, ownerID, ownerField)
	if err != nil {
		return 0, err
	}
	return len(items), nil
}

// listGalleryItems runs the actual OQL query shared by
// nextGalleryPosition and GalleryList. ownerType and ownerField are
// assumed already validated (validateGalleryOwner) by every caller --
// this function itself does not re-validate, so it must never be
// called with unchecked client input directly.
//
// A genuinely fresh tenant has no rows -- and therefore no entity --
// for seam_media_items yet (xolu creates an entity implicitly on
// first write; there's no separate schema-registration step). Without
// the entityNotYetCreated check below, the very first upload for any
// tenant would fail: GalleryUpload calls nextGalleryPosition (which
// calls this function) before its own first Create of that entity
// ever happens, so the OQL SELECT hits a table that doesn't exist.
// Found live, not by inspection -- confirmed against a real xolu
// instance (XOLU-QL004: entity 'seam_media_items' does not exist),
// exactly the same shape T-28 already fixed for FK-sourced select
// fields on a fresh instance (asset_source_resolver.go's own
// entityNotYetCreated, reused here rather than re-implemented).
func (h *UIHandler) listGalleryItems(ctx context.Context, ownerType string, ownerID int64, ownerField string) ([]mediaItem, error) {
	query := fmt.Sprintf(
		"SELECT * FROM %s WHERE owner_type = '%s' AND owner_id = %d AND owner_field = '%s' ORDER BY position ASC",
		mediaItemsEntity, ownerType, ownerID, ownerField,
	)
	result, err := h.xoluClient.OQL(ctx, query)
	if err != nil {
		if entityNotYetCreated(err) {
			return []mediaItem{}, nil
		}
		return nil, err
	}
	items := make([]mediaItem, 0, len(result.Data))
	for _, row := range result.Data {
		items = append(items, mediaItem{
			ID:          toInt64(row["id"]),
			OwnerType:   toString(row["owner_type"]),
			OwnerID:     toInt64(row["owner_id"]),
			OwnerField:  toString(row["owner_field"]),
			BlobKey:     toString(row["blob_key"]),
			ContentType: toString(row["content_type"]),
			SizeBytes:   toInt64(row["size_bytes"]),
			SHA256:      toString(row["sha256"]),
			Position:    int(toInt64(row["position"])),
			UploadedBy:  toString(row["uploaded_by"]),
			UploadedAt:  toString(row["uploaded_at"]),
			URL:         galleryBlobURL(toString(row["blob_key"])),
		})
	}
	return items, nil
}

// GalleryList handles GET /gallery/items?owner_type=&owner_id=&owner_field=
// -- the widget's own load-time call to populate an existing gallery.
func (h *UIHandler) GalleryList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ownerType := r.URL.Query().Get("owner_type")
	ownerField := r.URL.Query().Get("owner_field")
	ownerID, err := validateGalleryOwner(ownerType, r.URL.Query().Get("owner_id"), ownerField)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	items, err := h.listGalleryItems(ctx, ownerType, ownerID, ownerField)
	if err != nil {
		h.logger.Error("gallery list failed", "error", err)
		http.Error(w, h.errText(r, "error.search_failed", "list failed"), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

// GalleryDelete handles DELETE /gallery/item?id=<id>. Removes the
// metadata row; the underlying blob is left in place, matching xolu's
// own documented BlobDelete semantics ("removes the key alias only...
// GC handles unreferenced blobs separately") -- this handler doesn't
// even call BlobDelete, deliberately: the metadata row is this
// gallery's own authoritative existence record, and removing it is
// sufficient for the item to disappear from every gallery view: xolu's
// GC reclaims the now-unreferenced blob on its own schedule, matching
// how the codebase already lets xolu own that lifecycle rather than
// this handler trying to coordinate a two-step delete.
func (h *UIHandler) GalleryDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, h.errText(r, "error.invalid_id", "invalid id"), http.StatusBadRequest)
		return
	}
	if err := h.xoluClient.Delete(ctx, mediaItemsEntity, id); err != nil {
		h.logger.Error("gallery delete failed", "error", err, "id", id)
		http.Error(w, h.errText(r, "error.delete_failed", "delete failed"), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"id": id, "deleted": true})
}

// GalleryBlobProxy handles GET /gallery/blob?key=<blob_key>, streaming
// the actual image bytes back to the browser for thumbnail/preview
// rendering. A proxy rather than a direct link to xolu: the browser
// never needs xolu's own address or credentials, and this route sits
// behind the same session auth (requireAuth, router.go) every other
// UI route already requires -- an unauthenticated request never
// reaches xolu at all. key is passed through to BlobGet as-is (not
// interpolated into any query string), so no injection surface exists
// here regardless of its shape; an invalid/unknown key simply 404s
// from xolu's own BlobGet error.
func (h *UIHandler) GalleryBlobProxy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	key := r.URL.Query().Get("key")
	if key == "" || strings.TrimSpace(key) == "" {
		http.Error(w, "missing key", http.StatusBadRequest)
		return
	}
	result, err := h.xoluClient.BlobGet(ctx, key)
	if err != nil {
		http.Error(w, h.errText(r, "error.not_found", "not found"), http.StatusNotFound)
		return
	}
	defer result.Body.Close()
	if result.ContentType != "" {
		w.Header().Set("Content-Type", result.ContentType)
	}
	w.Header().Set("Cache-Control", "private, max-age=3600")
	io.Copy(w, result.Body)
}
