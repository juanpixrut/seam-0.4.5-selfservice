package ui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func assetFormBody(t *testing.T, path string) string {
	t.Helper()
	srv := newAssetDynamicTestServer(t, 7, map[string]any{"id": float64(7), "name": "Pump 7", "type_id": float64(1), "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z"}, nil)
	defer srv.Close()
	h := newTestHandler(t, srv)
	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	if strings.Contains(path, "/new") {
		h.AssetNewDynamic(w, req)
	} else {
		h.AssetEditDynamic(w, req)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("%s: got %d: %s", path, w.Code, w.Body.String())
	}
	return w.Body.String()
}

// Only a form that creates an asset offers "Create and add photos": it needs a record that does
// not exist yet. The button sets a hidden _then field that belongs to the form.
func TestAssetForm_OffersCreateAndAddPhotosOnlyWhenCreating(t *testing.T) {
	create := assetFormBody(t, "/assets/dynamic/new")
	if !strings.Contains(create, `id="asset-dynamic-then"`) || !strings.Contains(create, "value=&#39;photos&#39;") || !strings.Contains(create, "action.create_and_add_photos") {
		t.Errorf("the create form must offer Create and add photos:\n%s", create)
	}
	if edit := assetFormBody(t, "/assets/dynamic/7/edit"); strings.Contains(edit, "_then") {
		t.Errorf("the edit form must not offer it")
	}
}

// "Create and add photos" saves the asset and answers with its edit form, opened on the
// Map & photos tab, with the uploader pointed at the new asset. "Create" is unchanged.
func TestAssetCreateDynamic_CreateAndAddPhotosStaysOnTheSavedAsset(t *testing.T) {
	post := func(then string) (*httptest.ResponseRecorder, *assetPostTestServer) {
		ts := newAssetPostTestServer(t)
		h := newTestHandler(t, ts.Server)
		form := url.Values{"name": {"Compresor #4"}, "type_id": {"1"}, "criticality": {"High"}}
		if then != "" {
			form.Set("_then", then)
		}
		req := httptest.NewRequest("POST", "/assets/dynamic", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()
		h.AssetCreateDynamic(w, req)
		return w, ts
	}

	w, ts := post("photos")
	defer ts.Close()
	if ts.lastCreateBody == nil {
		t.Fatalf("the asset must be created first")
	}
	if w.Code != http.StatusOK || w.Header().Get("HX-Redirect") != "" {
		t.Fatalf("must answer with the edit form, not redirect: %d %q", w.Code, w.Header().Get("HX-Redirect"))
	}
	if !strings.Contains(w.Header().Get("HX-Trigger"), "seam:record-created") {
		t.Errorf("the page must be told a record was created, HX-Trigger = %q", w.Header().Get("HX-Trigger"))
	}
	body := w.Body.String()
	for _, want := range []string{`owner-id="1"`, `owner-type="asset"`, `"tab-geo"`, "o.click()"} {
		if !strings.Contains(body, want) {
			t.Errorf("edit form of the new asset missing %s", want)
		}
	}

	w2, ts2 := post("")
	defer ts2.Close()
	if w2.Header().Get("HX-Redirect") != "/assets" {
		t.Errorf("plain Create must still go back to the list, HX-Redirect = %q", w2.Header().Get("HX-Redirect"))
	}
}

// ?tab= opens that tab first, but only if it is one of the form's own tabs: an unknown value,
// or one built to break out of the script, never reaches it.
func TestAssetEditDynamic_OpensTheRequestedTabOnlyIfItIsOneOfTheForms(t *testing.T) {
	if body := assetFormBody(t, "/assets/dynamic/7/edit?tab=tab-geo"); !strings.Contains(body, `"tab-geo"`) || !strings.Contains(body, "o.click()") {
		t.Errorf("tab=tab-geo must open the Map & photos tab")
	}
	for _, bad := range []string{"tab=nope", "tab=" + url.QueryEscape(`x"+alert(1)+"`), "tab="} {
		body := assetFormBody(t, "/assets/dynamic/7/edit?"+bad)
		if strings.Contains(body, "o.click()") || strings.Contains(body, "alert(1)") {
			t.Errorf("%s: an unknown tab must be ignored", bad)
		}
	}
	if body := assetFormBody(t, "/assets/dynamic/7/edit"); strings.Contains(body, "o.click()") {
		t.Errorf("without ?tab= the first tab stays the first")
	}
}
