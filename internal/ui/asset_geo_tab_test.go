package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ha1tch/seam-ui/internal/formengine/schemas/asset"
)

// The schema's tabs: "Map & photos" comes right after Location, with the GPS field and
// the photo gallery in a section each.
func TestAssetSchema_MapAndPhotosTab(t *testing.T) {
	var doc struct {
		Form struct {
			Tabs []struct {
				ID       string `json:"id"`
				Label    string `json:"label"`
				Sections []struct {
					ID     string   `json:"id"`
					Fields []string `json:"fields"`
				} `json:"sections"`
			} `json:"tabs"`
		} `json:"x-seam-form"`
	}
	if err := json.Unmarshal(asset.Schema(), &doc); err != nil {
		t.Fatal(err)
	}
	idx := map[string]int{}
	for i, tab := range doc.Form.Tabs {
		idx[tab.ID] = i
	}
	geo, ok := idx["tab-geo"]
	if !ok {
		t.Fatalf("no tab-geo in %v", idx)
	}
	if geo != idx["tab-loc"]+1 {
		t.Errorf("tab-geo must follow tab-loc: %v", idx)
	}
	tab := doc.Form.Tabs[geo]
	if tab.Label != "assets.forms.asset.tab.geo" || len(tab.Sections) != 2 ||
		strings.Join(tab.Sections[0].Fields, ",") != "gps_location" || strings.Join(tab.Sections[1].Fields, ",") != "photos" {
		t.Errorf("unexpected tab-geo: %+v", tab)
	}
}

// Every schema field is persisted on the asset except photos, which lives in /blob and
// the media gallery's own entity, never on the asset row.
func TestAssetSchema_EveryFieldPersistedExceptPhotos(t *testing.T) {
	var doc struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(asset.Schema(), &doc); err != nil {
		t.Fatal(err)
	}
	for name := range doc.Properties {
		_, mapped := assetFieldToXolu[name]
		if name == "photos" && mapped {
			t.Errorf("photos is stored by the media gallery, it must not be mapped onto the asset row")
		}
		if name != "photos" && !mapped {
			t.Errorf("schema field %q has no entry in assetFieldToXolu", name)
		}
	}
	if len(assetFieldToXolu) != len(doc.Properties)-1 {
		t.Errorf("assetFieldToXolu has %d entries, the schema has %d fields (one is photos)", len(assetFieldToXolu), len(doc.Properties))
	}
}

// The edit form: the GPS field opens on the stored point, the gallery gets the owner it
// needs (type "asset", this asset's id, the field name), and the related-asset picker
// carries its stored id.
func TestAssetEditDynamic_MapPhotosAndParentPicker(t *testing.T) {
	assetData := map[string]any{
		"id": float64(7), "name": "Pump 7", "type_id": float64(1), "parent_id": float64(3),
		"gps_location": "-34.9011,-56.1645", "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
	}
	srv := newAssetDynamicTestServer(t, 7, assetData, nil)
	defer srv.Close()
	h := newTestHandler(t, srv)
	req := httptest.NewRequest("GET", "/assets/dynamic/7/edit", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	h.AssetEditDynamic(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{
		`data-seam-widget="gps"`, `name="gps_location"`, `value="-34.9011,-56.1645"`,
		`<seam-file-upload owner-type="asset" owner-id="7" owner-field="photos"`,
		`data-seam-picker="1"`, `data-entity="entity:assets"`, `data-selector="assets"`, `name="parent_id"`, `value="3"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("edit form missing %s", want)
		}
	}
}

// A new, unsaved asset has no id to attach photos to: the gallery says so.
func TestAssetNewDynamic_PhotosNeedASavedRecord(t *testing.T) {
	srv := newAssetDynamicTestServer(t, 0, nil, nil)
	defer srv.Close()
	h := newTestHandler(t, srv)
	req := httptest.NewRequest("GET", "/assets/dynamic/new", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	h.AssetNewDynamic(w, req)
	body := w.Body.String()
	if !strings.Contains(body, "Save the record first to add photos") || strings.Contains(body, "<seam-file-upload") {
		t.Errorf("create mode must show the save-first message, not the uploader")
	}
	if !strings.Contains(body, `data-seam-widget="gps"`) {
		t.Errorf("the GPS field must be on the create form too")
	}
}

// Posted gps_location is read and stored under the same key; photos is never read.
func TestAssetPost_GPSLocationIsPersisted(t *testing.T) {
	form := url.Values{"name": {"Pump 7"}, "gps_location": {"-34.901100,-56.164500"}, "photos": {"x"}}
	req := httptest.NewRequest("POST", "/assets/dynamic/7", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	values := parseAssetFormValues(req)
	if values["gps_location"] != "-34.901100,-56.164500" {
		t.Errorf("gps_location = %v", values["gps_location"])
	}
	if _, ok := values["photos"]; ok {
		t.Errorf("photos must not be read from the form")
	}
	if got := AssetValuesToXolu(values)["gps_location"]; got != "-34.901100,-56.164500" {
		t.Errorf("stored as %v", got)
	}
}

// The related-asset picker opens its list on focus, which sends an empty query: the
// server must answer it with the first assets, and an id lookup must resolve the label
// of the stored value.
func TestPickerSearch_AssetsEmptyQueryTypedQueryAndIDLookup(t *testing.T) {
	rows := map[string][]map[string]any{
		"SELECT TOP 20 * FROM assets ORDER BY name":                        {{"id": float64(3), "name": "Pump A", "status": "Active"}, {"id": float64(4), "name": "Pump B"}},
		"SELECT TOP 20 * FROM assets WHERE name LIKE '%pu%' ORDER BY name": {{"id": float64(3), "name": "Pump A", "status": "Active"}},
	}
	srv := newAssetDynamicTestServer(t, 3, map[string]any{"id": float64(3), "name": "Pump A", "status": "Active"}, rows)
	defer srv.Close()
	h := newTestHandler(t, srv)
	get := func(q string) []map[string]any {
		req := httptest.NewRequest("GET", "/picker/search?"+q, nil)
		w := httptest.NewRecorder()
		h.PickerSearch(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: got %d %s", q, w.Code, w.Body.String())
		}
		var out []map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s: %v: %s", q, err, w.Body.String())
		}
		return out
	}
	if got := get("entity=entity:assets&q="); len(got) != 2 || got[0]["label"] != "Pump A \u2014 Active" || got[1]["label"] != "Pump B" {
		t.Errorf("empty query: %v", got)
	}
	if got := get("entity=entity:assets&q=pu"); len(got) != 1 || got[0]["id"] != float64(3) {
		t.Errorf("typed query: %v", got)
	}
	if got := get("entity=entity:assets&id=3"); len(got) != 1 || got[0]["label"] != "Pump A \u2014 Active" {
		t.Errorf("id lookup: %v", got)
	}
}
