package ui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestConsumableTypeRegistrationAndSelection(t *testing.T) {
	var savedType, savedConsumable map[string]any
	h := templateTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "POST /api/v1/consumable_types":
			if err := json.NewDecoder(r.Body).Decode(&savedType); err != nil {
				t.Fatal(err)
			}
			oqlCreated(w, 7)
		case "GET /api/v1/consumable_types/7":
			io.WriteString(w, `{"id":7,"name":"Lubricantes"}`)
		case "GET /api/v1/consumable_types/99":
			w.WriteHeader(http.StatusNotFound)
		case "GET /api/v1/unit/4":
			io.WriteString(w, `{"id":4,"name":"Litro"}`)
		case "POST /api/v1/consumable":
			if err := json.NewDecoder(r.Body).Decode(&savedConsumable); err != nil {
				t.Fatal(err)
			}
			oqlCreated(w, 10)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	rec := postForm(t, h.ConsumableTypeCreate, "/consumable-types", url.Values{
		"name": {" Lubricantes "}, "description": {"Aceites y grasas"}, "enabled": {"on"},
	})
	assertRedirectsTo(t, rec, "/consumable-types")
	if len(savedType) != 2 || savedType["name"] != "Lubricantes" || savedType["description"] != "Aceites y grasas" {
		t.Fatalf("unexpected consumable type data: %v", savedType)
	}

	rec = httptest.NewRecorder()
	h.ConsumableNew(rec, httptest.NewRequest(http.MethodGet, "/consumable/new", nil))
	if !strings.Contains(rec.Body.String(), `data-entity="entity:consumable_types"`) {
		t.Fatal("new consumable form is missing the catalog picker")
	}

	fields := url.Values{"name": {"Aceite"}, "consumable_unit": {"4"}, "consumable_type": {"99"}}
	rec = postForm(t, h.ConsumableCreate, "/consumable", fields)
	if savedConsumable != nil || !strings.Contains(rec.Body.String(), "consumable_types.invalid") {
		t.Fatal("a nonexistent consumable type must produce a form error")
	}

	fields.Set("consumable_type", "7")
	rec = postForm(t, h.ConsumableCreate, "/consumable", fields)
	assertRedirectsTo(t, rec, "/consumable/10")
	if savedConsumable["consumable_type_id"] != float64(7) {
		t.Fatalf("consumable does not reference the catalog: %v", savedConsumable)
	}
	if _, found := savedConsumable["consumable_type"]; found {
		t.Fatal("new consumable stored a legacy type")
	}
}
