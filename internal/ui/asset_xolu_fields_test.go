package ui

import (
	"reflect"
	"testing"
)

func TestAssetFieldToXolu_AllSchemaFieldsCovered(t *testing.T) {
	// Every field name formengine's asset schema actually defines must
	// appear in this table -- confirmed by cross-checking against
	// asset.json directly (see the CHANGELOG entry for this file), but
	// re-asserted here as a standing invariant, not just a one-time check.
	want := []string{
		"name", "type_id", "status", "criticality", "connectivity_tier",
		"giai", "grai", "barcode_rfid", "technical_tag", "brand_id",
		"model_id", "serial_1", "serial_2", "lot", "location_id",
		"region_id", "latitude", "longitude", "parent_id", "custodian_id",
		"tenure_type", "loan_expiry", "loan_links", "purchase_date",
		"expected_arrival_date", "arrival_date", "obsolescence_date",
		"expiration_date", "decommission_date", "decommission_reason",
		"lifecycle_links", "purchase_price", "currency_id", "current_price",
		"cost_center_id", "warranty_end", "document_links", "temp_min",
		"temp_max", "refrigerant", "voltage_consumption", "service_configs",
		"gps_location",
	}
	if len(want) != 43 {
		t.Fatalf("test fixture itself has %d fields, want 43 -- fix the test", len(want))
	}
	for _, f := range want {
		if _, ok := assetFieldToXolu[f]; !ok {
			t.Errorf("assetFieldToXolu missing %q", f)
		}
	}
	if len(assetFieldToXolu) != 43 {
		t.Errorf("assetFieldToXolu has %d entries, want 43", len(assetFieldToXolu))
	}
}

func TestAssetFieldToXolu_IsOneToOne(t *testing.T) {
	// If this ever fails, assetFieldFromXolu's init() would already
	// have panicked before the test binary even started -- this test
	// exists to make that failure mode show up as a normal test failure
	// with a clear message, not a mysterious binary-won't-start crash.
	seen := make(map[string]string)
	for en, es := range assetFieldToXolu {
		if existing, ok := seen[es]; ok {
			t.Errorf("xolu key %q claimed by both %q and %q", es, existing, en)
		}
		seen[es] = en
	}
}

func TestAssetFieldFromXolu_IsExactReverse(t *testing.T) {
	if len(assetFieldFromXolu) != len(assetFieldToXolu) {
		t.Fatalf("assetFieldFromXolu has %d entries, assetFieldToXolu has %d", len(assetFieldFromXolu), len(assetFieldToXolu))
	}
	for en, es := range assetFieldToXolu {
		if got := assetFieldFromXolu[es]; got != en {
			t.Errorf("assetFieldFromXolu[%q] = %q, want %q", es, got, en)
		}
	}
}

func TestAssetValuesToXolu_ScalarFields(t *testing.T) {
	in := map[string]interface{}{
		"name":         "Compresor #4",
		"criticality":  "Alta",
		"custodian_id": float64(7),
	}
	got := AssetValuesToXolu(in)
	want := map[string]interface{}{
		"name":         "Compresor #4",
		"criticality":  "Alta",
		"custodian_id": float64(7),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("AssetValuesToXolu(%v) = %v, want %v", in, got, want)
	}
}

func TestAssetValuesFromXolu_ScalarFields(t *testing.T) {
	// criticality's value is still translated (T-23, docs/RESOLVED.md):
	// asset.json's own enum is English as of T-23, so xolu's stored
	// Spanish "Alta" must come back as "High" for the edit form's select
	// to correctly pre-select the matching English option. The KEY
	// itself no longer translates (T-40 Block B, this session) -- xolu's
	// own column is "criticality" now too; only the value still does.
	in := map[string]interface{}{
		"name":         "Compresor #4",
		"criticality":  "Alta",
		"custodian_id": float64(7),
	}
	got := AssetValuesFromXolu(in)
	want := map[string]interface{}{
		"name":         "Compresor #4",
		"criticality":  "High",
		"custodian_id": float64(7),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("AssetValuesFromXolu(%v) = %v, want %v", in, got, want)
	}
}

func TestAssetValues_RoundTrip_AllScalarFields(t *testing.T) {
	// Every English field name, translated to xolu and back, must
	// return exactly the field name it started as -- the actual
	// invariant this whole table exists to guarantee.
	for en := range assetFieldToXolu {
		in := map[string]interface{}{en: "some-value"}
		xolu := AssetValuesToXolu(in)
		back := AssetValuesFromXolu(xolu)
		if _, ok := back[en]; !ok {
			t.Errorf("round-trip lost field %q entirely: xolu=%v back=%v", en, xolu, back)
			continue
		}
		if back[en] != "some-value" {
			t.Errorf("round-trip on %q: got %v, want \"some-value\"", en, back[en])
		}
	}
}

func TestAssetEnumValues_RoundTrip(t *testing.T) {
	// The three enum fields' actual English tokens, both directions --
	// the specific behaviour T-23 (docs/RESOLVED.md) added.
	cases := []struct {
		field   string
		xoluKey string
		english string
		spanish string
	}{
		{"criticality", "criticality", "High", "Alta"},
		{"criticality", "criticality", "Medium", "Media"},
		{"criticality", "criticality", "Low", "Baja"},
		{"tenure_type", "tenure_type", "Owned", "Propiedad"},
		{"tenure_type", "tenure_type", "Loaned", "Comodato"},
		{"tenure_type", "tenure_type", "Borrowed", "Préstamo"},
		{"tenure_type", "tenure_type", "Rented", "Alquiler"},
		{"decommission_reason", "decommission_reason", "Sale", "Venta"},
		{"decommission_reason", "decommission_reason", "Scrap", "Desguace"},
		{"decommission_reason", "decommission_reason", "Theft", "Robo"},
		{"decommission_reason", "decommission_reason", "Donation", "Donación"},
		{"decommission_reason", "decommission_reason", "Other", "Otros"},
	}
	for _, c := range cases {
		t.Run(c.field+"="+c.english, func(t *testing.T) {
			xolu := AssetValuesToXolu(map[string]interface{}{c.field: c.english})
			if got := xolu[c.xoluKey]; got != c.spanish {
				t.Errorf("ToXolu(%s=%q): got %s=%v, want %s=%q", c.field, c.english, c.xoluKey, got, c.xoluKey, c.spanish)
			}
			back := AssetValuesFromXolu(map[string]interface{}{c.xoluKey: c.spanish})
			if got := back[c.field]; got != c.english {
				t.Errorf("FromXolu(%s=%q): got %s=%v, want %s=%q", c.xoluKey, c.spanish, c.field, got, c.field, c.english)
			}
		})
	}
}

func TestAssetEnumValues_UnrecognisedValuePassesThrough(t *testing.T) {
	// Garbage that isn't a recognised token in either direction's table
	// (e.g. a value that fails schema validation before ever reaching
	// this code, or data from an unrelated source) must not be silently
	// mangled -- pass through exactly as given.
	xolu := AssetValuesToXolu(map[string]interface{}{"criticality": "Extrema"})
	if got := xolu["criticality"]; got != "Extrema" {
		t.Errorf("ToXolu unrecognised value: got %v, want \"Extrema\" unchanged", got)
	}
	back := AssetValuesFromXolu(map[string]interface{}{"criticality": "Extrema"})
	if got := back["criticality"]; got != "Extrema" {
		t.Errorf("FromXolu unrecognised value: got %v, want \"Extrema\" unchanged", got)
	}
}

func TestAssetValuesToXolu_DocumentLinksArray(t *testing.T) {
	in := map[string]interface{}{
		"loan_links": []interface{}{
			map[string]interface{}{"name": "Contrato", "url": "https://x/c.pdf", "type": "pdf"},
		},
	}
	got := AssetValuesToXolu(in)
	links, ok := got["loan_links"].([]interface{})
	if !ok || len(links) != 1 {
		t.Fatalf("expected loan_links with 1 row, got %v", got)
	}
	row := links[0].(map[string]interface{})
	want := map[string]interface{}{"name": "Contrato", "url": "https://x/c.pdf", "type": "pdf"}
	if !reflect.DeepEqual(row, want) {
		t.Errorf("row = %v, want %v", row, want)
	}
}

func TestAssetValuesFromXolu_DocumentLinksArray(t *testing.T) {
	in := map[string]interface{}{
		"lifecycle_links": []interface{}{
			map[string]interface{}{"name": "Manual", "url": "https://x/m.pdf", "type": ""},
		},
	}
	got := AssetValuesFromXolu(in)
	links, ok := got["lifecycle_links"].([]interface{})
	if !ok || len(links) != 1 {
		t.Fatalf("expected lifecycle_links with 1 row, got %v", got)
	}
	row := links[0].(map[string]interface{})
	want := map[string]interface{}{"name": "Manual", "url": "https://x/m.pdf", "type": ""}
	if !reflect.DeepEqual(row, want) {
		t.Errorf("row = %v, want %v", row, want)
	}
}

func TestAssetValuesToXolu_ServiceConfigsArray(t *testing.T) {
	in := map[string]interface{}{
		"service_configs": []interface{}{
			map[string]interface{}{"measure": "Kms", "threshold": float64(10000), "alert_id": float64(1)},
		},
	}
	got := AssetValuesToXolu(in)
	rows, ok := got["service_configs"].([]interface{})
	if !ok || len(rows) != 1 {
		t.Fatalf("expected service_configs with 1 row, got %v", got)
	}
	row := rows[0].(map[string]interface{})
	want := map[string]interface{}{"measure": "Kms", "threshold": float64(10000), "alert_id": float64(1)}
	if !reflect.DeepEqual(row, want) {
		t.Errorf("row = %v, want %v", row, want)
	}
}

func TestAssetValues_RoundTrip_ArrayFields(t *testing.T) {
	in := map[string]interface{}{
		"document_links": []interface{}{
			map[string]interface{}{"name": "Factura", "url": "https://x/f.pdf", "type": "pdf"},
			map[string]interface{}{"name": "Foto", "url": "https://x/p.jpg", "type": ""},
		},
	}
	xolu := AssetValuesToXolu(in)
	back := AssetValuesFromXolu(xolu)
	if !reflect.DeepEqual(back, in) {
		t.Errorf("round-trip changed the value:\n  original: %v\n  back:     %v", in, back)
	}
}

func TestAssetValuesToXolu_UnrecognisedFieldPassesThrough(t *testing.T) {
	in := map[string]interface{}{"some_future_field": "value"}
	got := AssetValuesToXolu(in)
	if got["some_future_field"] != "value" {
		t.Errorf("unrecognised field should pass through unchanged, got %v", got)
	}
}

func TestAssetValuesToXolu_EmptyMap(t *testing.T) {
	got := AssetValuesToXolu(map[string]interface{}{})
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestAssetValuesToXolu_NilValuePreserved(t *testing.T) {
	in := map[string]interface{}{"parent_id": nil}
	got := AssetValuesToXolu(in)
	if v, ok := got["parent_id"]; !ok || v != nil {
		t.Errorf("expected parent_id=nil, got %v (present=%v)", v, ok)
	}
}
