//go:build integration

package formengine

// This is a dormant guard (Part 3 §8 of the working conventions): it does
// not run in the default `go test ./...` invocation. It exists to prove,
// against a real xolu server rather than by inspecting xolu's source, the
// claim recorded in docs/KNOWN_ISSUES.md: POST/GET /api/v1/schema/{entity}
// round-trips x-seam-* extensions intact.
//
// Canonical invocation (requires XOLU_URL pointing at a running xolu with
// auth disabled or a valid credential):
//
//	XOLU_URL=http://localhost:18080 go test -tags integration ./internal/formengine/... -run TestSchemaRoundTrip -v

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"testing"
)

func TestSchemaRoundTrip_XSeamExtensionsSurvive(t *testing.T) {
	baseURL := os.Getenv("XOLU_URL")
	if baseURL == "" {
		t.Skip("XOLU_URL not set; this guard needs a running xolu instance")
	}

	entity := "formengine_roundtrip_test"
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"nombre": map[string]interface{}{
				"type":  "string",
				"title": "Nombre",
			},
			"marca_id": map[string]interface{}{
				"type":          "integer",
				"title":         "Marca",
				"x-seam-input":  "select",
				"x-seam-source": "entity:brands",
			},
		},
		"x-seam-form": map[string]interface{}{
			"width": "md",
			"tabs": []interface{}{
				map[string]interface{}{
					"id":    "tab-principal",
					"label": "test.tab.principal",
				},
			},
		},
	}
	body, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}

	// POST — register the schema.
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/schema/"+entity, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build POST request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/schema/%s: %v", entity, err)
	}
	postBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/v1/schema/%s: status %d, body %s", entity, resp.StatusCode, postBody)
	}

	// GET — read it back.
	resp, err = http.Get(baseURL + "/api/v1/schema/" + entity)
	if err != nil {
		t.Fatalf("GET /api/v1/schema/%s: %v", entity, err)
	}
	getBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/v1/schema/%s: status %d, body %s", entity, resp.StatusCode, getBody)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(getBody, &got); err != nil {
		t.Fatalf("unmarshal GET response: %v (body: %s)", err, getBody)
	}

	// GetEntitySchema-shaped responses wrap the raw schema under a
	// "schema" key (see xolu's pkg/client/schema.go); handle both that
	// and a bare-schema response defensively rather than assuming one.
	sub, ok := got["schema"].(map[string]interface{})
	if ok {
		got = sub
	}

	props, ok := got["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("no properties object in round-tripped schema: %#v", got)
	}
	marca, ok := props["marca_id"].(map[string]interface{})
	if !ok {
		t.Fatalf("marca_id property missing after round-trip: %#v", props)
	}
	if marca["x-seam-input"] != "select" {
		t.Errorf("x-seam-input did not survive: got %v", marca["x-seam-input"])
	}
	if marca["x-seam-source"] != "entity:brands" {
		t.Errorf("x-seam-source did not survive: got %v", marca["x-seam-source"])
	}

	xForm, ok := got["x-seam-form"].(map[string]interface{})
	if !ok {
		t.Fatalf("x-seam-form did not survive at all: %#v", got)
	}
	if xForm["width"] != "md" {
		t.Errorf("x-seam-form.width did not survive: got %v", xForm["width"])
	}
	tabs, ok := xForm["tabs"].([]interface{})
	if !ok || len(tabs) != 1 {
		t.Fatalf("x-seam-form.tabs did not survive: %#v", xForm["tabs"])
	}
	tab0, _ := tabs[0].(map[string]interface{})
	if !reflect.DeepEqual(tab0["id"], "tab-principal") {
		t.Errorf("nested x-seam-form.tabs[0].id did not survive: got %v", tab0["id"])
	}

	fmt.Fprintf(os.Stderr, "round-trip confirmed against %s: x-seam-* extensions intact, including nested x-seam-form.tabs\n", baseURL)
}
