package ui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	xolu "github.com/ha1tch/xolu/pkg/client"
)

// Exercise the actual Xolu client without opening sockets.
type templateTestTransport func(*http.Request) (*http.Response, error)

func (f templateTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func templateTestHandler(t *testing.T, respond http.HandlerFunc) *UIHandler {
	t.Helper()
	client := xolu.New("http://xolu.test", xolu.WithHTTPClient(&http.Client{
		Transport: templateTestTransport(func(r *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()
			if r.URL.Path == "/api/v1/oql/query" {
				oqlEmpty(rec)
			} else {
				respond(rec, r)
			}
			return rec.Result(), nil
		}),
	}))
	return NewUIHandler(client, Config{AppName: "Test"}, nil, nil, nil, nil, nil, nil, nil)
}

const templateTestEnabledAction = `{"id":7,"name":"Avisar","enabled":true,"parameters":[]}`

func templateTestValues() url.Values {
	return url.Values{
		"name":                                               {"Inspección"},
		"minAssets":                                          {"0"},
		"maxAssets":                                          {"0"},
		"sequence_length":                                    {"4"},
		"header_items[0][tag]":                               {"temperatura"},
		"header_items[0][type]":                              {"number"},
		"header_items[0][to_fill_out]":                       {"upon_executing"},
		"workflow_states_name[]":                             {"Pendiente", "Finalizada"},
		"blocks[2][name]":                                    {"Inspeccion"},
		"blocks[2][responsible_type]":                        {"user"},
		"blocks[2][lines][4][name]":                          {"presion"},
		"blocks[2][lines][4][type]":                          {"number"},
		"actions[1][trigger_point]":                          {"after_work_order"},
		"actions[1][action]":                                 {"7"},
		"rule_item[3][name]":                                 {"ALERTA"},
		"rule_item[3][trigger]":                              {"on_evaluate_header"},
		"rule_item[3][conditions][2][field]":                 {"header:temperatura"},
		"rule_item[3][conditions][2][operator]":              {"greater_than"},
		"rule_item[3][conditions][2][comparison_value]":      {"80"},
		"rule_item[3][conditions][2][logical_operator_next]": {"None"},
		"rule_item[3][target_field]":                         {"block:Inspeccion:line:presion"},
		"rule_item[3][action_type]":                          {"show_field"},
		"rule_item[3][stop_execution]":                       {"on"},
		"rule_item[3][message_type]":                         {"WARNING"},
		"rule_item[3][message_text]":                         {"Revisar temperatura"},
	}
}

func TestWorkOrderTemplateRequiresConsumable(t *testing.T) {
	for _, tc := range []struct {
		name             string
		allowConsumables bool
		allowAny         bool
		consumableID     string
		wantError        bool
	}{
		{name: "disabled"},
		{name: "required", allowConsumables: true, wantError: true},
		{name: "allow any", allowConsumables: true, allowAny: true},
		{name: "selected", allowConsumables: true, consumableID: "9"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			saved := false
			h := templateTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.Method + " " + r.URL.Path {
				case "GET /api/v1/wo_actions/7":
					io.WriteString(w, templateTestEnabledAction)
				case "GET /api/v1/consumable/9":
					io.WriteString(w, `{"id":9,"name":"Filtro"}`)
				case "POST /api/v1/work_order_templates":
					saved = true
					oqlCreated(w, 42)
				default:
					t.Fatalf("unexpected Xolu request: %s %s", r.Method, r.URL.Path)
				}
			})
			values := templateTestValues()
			values.Set("consumables_allowed", map[bool]string{true: "on"}[tc.allowConsumables])
			values.Set("add_any_consumables_allowed", map[bool]string{true: "on"}[tc.allowAny])
			values.Set("consumables[0][consumable_id]", tc.consumableID)
			if tc.consumableID != "" {
				values.Set("consumables[0][quantity]", "1")
			}

			next := htmxPost(t, h.WorkOrdersTemplatesValidateStep1, "/workorders-templates/validate-step-1", values)
			create := htmxPost(t, h.WorkOrderTemplateCreate, "/workorders-templates", values)
			if tc.wantError {
				const message = "workorders-templates.error.consumable_required"
				if saved || !strings.Contains(next.Body.String(), message) || !strings.Contains(create.Body.String(), message) {
					t.Fatal("expected the missing consumable error at both steps and no save")
				}
			} else if !strings.Contains(next.Header().Get("HX-Trigger"), "woTemplateStep1Valid") || !saved {
				t.Fatalf("expected both steps to succeed: next=%d, create=%d, saved=%t", next.Code, create.Code, saved)
			}
		})
	}
}

func TestWorkOrderTemplateCreatePersistsReferences(t *testing.T) {
	for _, htmx := range []bool{false, true} {
		t.Run(map[bool]string{false: "normal", true: "modal"}[htmx], func(t *testing.T) {
			var saved map[string]interface{}
			h := templateTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.Method + " " + r.URL.Path {
				case "GET /api/v1/wo_actions/7":
					io.WriteString(w, templateTestEnabledAction)
				case "POST /api/v1/work_order_templates":
					if err := json.NewDecoder(r.Body).Decode(&saved); err != nil {
						t.Fatal(err)
					}
					oqlCreated(w, 42)
				default:
					t.Fatalf("unexpected Xolu request: %s %s", r.Method, r.URL.Path)
				}
			})
			var rec *httptest.ResponseRecorder
			if htmx {
				rec = htmxPost(t, h.WorkOrderTemplateCreate, "/workorders-templates", templateTestValues())
			} else {
				rec = postForm(t, h.WorkOrderTemplateCreate, "/workorders-templates", templateTestValues())
			}
			if saved == nil {
				t.Fatalf("template was not saved: %s", rec.Body.String())
			}
			action := saved["actions"].([]interface{})[0].(map[string]interface{})
			if action["action"] != float64(7) || action["trigger_point"] != "after_work_order" {
				t.Fatalf("action reference lost: %#v", action)
			}
			rule := saved["rule_item"].([]interface{})[0].(map[string]interface{})
			condition := rule["conditions"].([]interface{})[0].(map[string]interface{})
			if condition["field"] != "header:temperatura" || condition["comparison_value"] != "80" || rule["target_field"] != "block:Inspeccion:line:presion" || rule["stop_execution"] != true {
				t.Fatalf("rule references lost: %#v", rule)
			}
			if htmx {
				if rec.Code != http.StatusOK || rec.Header().Get("HX-Redirect") != "/workorders-templates/42" || rec.Header().Get("HX-Trigger") != "seamModal2Saved" || rec.Body.Len() != 0 {
					t.Fatalf("expected modal close and page navigation: %d %v %s", rec.Code, rec.Header(), rec.Body.String())
				}
			} else {
				assertRedirectsTo(t, rec, "/workorders-templates/42")
			}
		})
	}
}

func TestWorkOrderTemplateCreateInvalidReferencesStayInModal(t *testing.T) {
	for _, tc := range []struct{ name, key, value, errorKey string }{
		{"condition", "rule_item[3][conditions][2][field]", "header:deleted", "missing_field"},
		{"target", "rule_item[3][target_field]", "block:Other:line:presion", "target_field_invalid"},
		{"action", "actions[1][action]", "999", "action_unavailable"},
		{"action_id", "actions[1][action]", "garbage", "invalid_action"},
		{"operator", "rule_item[3][conditions][2][operator]", "INVALID", "invalid_option"},
		{"missing_field", "rule_item[3][conditions][2][field]", "", "field_not_selected"},
		{"missing_name", "rule_item[3][name]", "", "rule_name_required"},
		{"missing_trigger", "actions[1][trigger_point]", "", "missing_trigger"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := templateTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "POST" {
					t.Fatal("invalid template must not be saved")
				}
				if r.URL.Path == "/api/v1/wo_actions/7" {
					io.WriteString(w, templateTestEnabledAction)
					return
				}
				w.WriteHeader(http.StatusNotFound)
				io.WriteString(w, `{"error":{"code":"NOT_FOUND","message":"Not found"}}`)
			})
			values := templateTestValues()
			values.Set(tc.key, tc.value)
			rec := htmxPost(t, h.WorkOrderTemplateCreate, "/workorders-templates", values)
			assertPartial(t, rec)
			body := rec.Body.String()
			for _, want := range []string{
				"wo-template-step2-form",
				"header:temperatura",
				"block:Inspeccion:line:presion",
				`value="80"`,
				"workorders-templates.error." + tc.errorKey,
			} {
				if !strings.Contains(body, want) {
					t.Errorf("modal lost %q: %s", want, body)
				}
			}
			if strings.Contains(body, "wo-template-step1-form\" class=") || rec.Header().Get("HX-Redirect") != "" || rec.Header().Get("HX-Trigger") != "" {
				t.Fatal("invalid save navigated or rendered the first page in the modal")
			}
		})
	}
}

func TestWorkOrderTemplateCreateEmptySecondStep(t *testing.T) {
	var saved map[string]interface{}
	h := templateTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Fatalf("empty action must not be looked up: %s", r.URL)
		}
		json.NewDecoder(r.Body).Decode(&saved)
		oqlCreated(w, 42)
	})
	values := templateTestValues()
	for key := range values {
		if strings.HasPrefix(key, "actions[") || strings.HasPrefix(key, "rule_item[") {
			delete(values, key)
		}
	}
	values.Set("actions[0][trigger_point]", "before_work_order")
	values.Set("actions[0][action]", "")
	rec := htmxPost(t, h.WorkOrderTemplateCreate, "/workorders-templates", values)
	if saved == nil {
		t.Fatalf("empty optional rows prevented save: %s", rec.Body.String())
	}
	if len(saved["actions"].([]interface{})) != 0 || len(saved["rule_item"].([]interface{})) != 0 {
		t.Fatal("blank rows must not be stored")
	}
}

func TestWorkOrderTemplateCreateStorageFailureKeepsModalValues(t *testing.T) {
	h := templateTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			io.WriteString(w, templateTestEnabledAction)
			return
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		io.WriteString(w, `{"error":{"code":"INVALID","message":"Cannot create template"}}`)
	})
	rec := htmxPost(t, h.WorkOrderTemplateCreate, "/workorders-templates", templateTestValues())
	assertPartial(t, rec)
	for _, want := range []string{"role=\"alert\"", "wo-template-step2-form", "error.create_failed", "ALERTA", "value=\"80\"", "header:temperatura"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("save failure lost %q", want)
		}
	}
	if rec.Header().Get("HX-Redirect") != "" || rec.Header().Get("HX-Trigger") != "" {
		t.Fatal("failed create must keep the modal open")
	}
}

func TestWorkOrderTemplateStep2UsesPostedFields(t *testing.T) {
	h := templateTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/api/v1/wo_actions/7" {
			io.WriteString(w, templateTestEnabledAction)
			return
		}
		t.Fatalf("unexpected Xolu request: %s %s", r.Method, r.URL.Path)
	})
	values := templateTestValues()
	values.Set("header_items[0][tag]", "temp & presión")
	values.Set("blocks[2][name]", "Bloque: A")
	values.Set("blocks[2][lines][4][name]", "línea/1")
	rec := htmxPost(t, h.WorkOrdersTemplatesStep2, "/workorders-templates/step-2", values)
	assertPartial(t, rec)
	for _, ref := range []string{"header:" + url.QueryEscape("temp & presión"), "block:" + url.QueryEscape("Bloque: A") + ":line:" + url.QueryEscape("línea/1")} {
		if !strings.Contains(rec.Body.String(), ref) {
			t.Errorf("missing encoded field reference %s", ref)
		}
	}
}
