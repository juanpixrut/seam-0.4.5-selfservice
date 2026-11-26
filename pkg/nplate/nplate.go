// Package nplate adds one construct to github.com/ha1tch/xolu/pkg/jsonplate:
// iteration. A jsonplate has exactly one construct, {"$ref": "path"}, and
// deliberately no loop, by design (jsonplate's own docs: "if you need
// conditionals, loops, or computed output, jsonplate is the wrong tool").
// That's the right call for jsonplate's own job (webhook/event payload
// templates, validated once at definition time, safe by construction
// because there's nothing to loop or branch). It's the wrong shape for
// rendering a variable number of dashboard rows from one entity query.
//
// nplate doesn't modify jsonplate or reach into its internals — it composes
// two already-independent, already-tested packages through their public
// APIs: github.com/ha1tch/queryfy's Collect (path resolution, wildcard
// expansion, and per-item iteration — "items[*]" and friends) drives the
// loop; jsonplate.Render does every actual value substitution, both inside
// a loop body and for everything outside one. nplate's own code is the
// glue between the two: recognising the one new construct and wiring it
// up. It re-implements neither path resolution nor $ref substitution.
//
// An nplate document is a jsonplate with one more object shape:
//
//	{"$each": "items[*]", "as": "item", "template": {"name": {"$ref": "item.name"}}}
//
// At render time, "$each"'s path is resolved and iterated via
// queryfy.Collect; for every matched element, "template" (itself a full
// nplate document — nested "$each" is not a special case, it falls out of
// the same recursive walk) is rendered with a data context binding the
// element under "as" (default "item"). The results collect into a JSON
// array in the position the "$each" object occupied. Everything outside
// an "$each" block is untouched by nplate's own walk and is resolved by a
// single final jsonplate.Render pass against the original data, exactly
// as if nplate weren't involved at all.
//
// Example:
//
//	plate := []byte(`{
//	  "widget": "recent_list",
//	  "rows": {"$each": "alerts[*]", "as": "a", "template": {
//	    "title": {"$ref": "a.title"},
//	    "asset": {"$ref": "a.asset_name"}
//	  }}
//	}`)
//	data := map[string]interface{}{
//	  "alerts": []interface{}{
//	    map[string]interface{}{"title": "Temp exceeded", "asset_name": "Chiller 5"},
//	    map[string]interface{}{"title": "Low battery", "asset_name": "Sensor 12"},
//	  },
//	}
//	out, _ := nplate.Render(plate, data)
//	// out == {"widget":"recent_list","rows":[
//	//          {"title":"Temp exceeded","asset":"Chiller 5"},
//	//          {"title":"Low battery","asset":"Sensor 12"}]}
package nplate

import (
	"encoding/json"
	"fmt"

	"github.com/ha1tch/queryfy"
	"github.com/ha1tch/xolu/pkg/jsonplate"
)

// eachKey/asKey/templateKey are the object keys that mark an object as an
// iteration node. An object is treated as one if and only if it has
// exactly these three keys ("as" is required, not optional, at this
// level -- Validate/expand default it to "item" only after confirming the
// shape is otherwise a well-formed $each node; see asEach).
const (
	eachKey     = "$each"
	asKey       = "as"
	templateKey = "template"
)

// defaultAs is the loop-variable binding name used when "as" is omitted.
const defaultAs = "item"

// Render resolves an nplate document against data: every "$each" block is
// expanded (queryfy.Collect drives the iteration, jsonplate.Render
// resolves each item's template), then a final jsonplate.Render pass
// resolves every "$ref" outside an "$each" block, against the original,
// unmodified data -- identical to calling jsonplate.Render directly for a
// document with no "$each" nodes at all.
func Render(plate []byte, data interface{}) ([]byte, error) {
	var root interface{}
	if err := json.Unmarshal(plate, &root); err != nil {
		return nil, fmt.Errorf("nplate: invalid template JSON: %w", err)
	}
	expanded, err := expand(root, data)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(expanded)
	if err != nil {
		return nil, fmt.Errorf("nplate: marshal expanded template: %w", err)
	}
	return jsonplate.Render(raw, data)
}

// Validate checks that plate is well-formed: valid JSON, every "$each"
// object has the right shape (a string path, an optional string "as", and
// a template that is itself valid), and -- delegated entirely to
// jsonplate.Validate -- every "$ref" outside an "$each" block is
// well-formed. Does not check that "$each" paths or "$ref" paths resolve
// against real data; that's only knowable at render time (same split
// jsonplate.Validate/Render already draw).
func Validate(plate []byte) error {
	var root interface{}
	if err := json.Unmarshal(plate, &root); err != nil {
		return fmt.Errorf("nplate: invalid template JSON: %w", err)
	}
	if err := validate(root); err != nil {
		return err
	}
	// A document with every "$each" block replaced by an empty array is
	// structurally identical, for jsonplate.Validate's purposes, to the
	// original outside those blocks -- reuse it rather than re-implement
	// $ref-shape validation a second time.
	stripped, err := stripEach(root)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(stripped)
	if err != nil {
		return fmt.Errorf("nplate: marshal stripped template: %w", err)
	}
	return jsonplate.Validate(raw)
}

// eachNode is one parsed "$each" object.
type eachNode struct {
	Each     string          `json:"$each"`
	As       string          `json:"as"`
	Template json.RawMessage `json:"template"`
}

// asEach reports whether v is a well-formed "$each" object and, if so,
// returns its parsed form. An "$each" object must have exactly the keys
// $each (string), as (string, optional -- absence is not a shape error),
// and template (any valid JSON value). A map containing "$each" among
// other, unrecognised keys is a malformed node, not silently treated as a
// literal -- same discipline jsonplate.asRef already applies to "$ref".
func asEach(v interface{}) (*eachNode, bool, error) {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, false, nil
	}
	rawEach, hasEach := m[eachKey]
	if !hasEach {
		return nil, false, nil
	}
	path, ok := rawEach.(string)
	if !ok {
		return nil, true, fmt.Errorf("nplate: %q must be a string path", eachKey)
	}
	as := defaultAs
	if rawAs, hasAs := m[asKey]; hasAs {
		s, ok := rawAs.(string)
		if !ok {
			return nil, true, fmt.Errorf("nplate: %q must be a string", asKey)
		}
		as = s
	}
	rawTemplate, hasTemplate := m[templateKey]
	if !hasTemplate {
		return nil, true, fmt.Errorf("nplate: %q object missing %q", eachKey, templateKey)
	}
	allowed := map[string]bool{eachKey: true, asKey: true, templateKey: true}
	for k := range m {
		if !allowed[k] {
			return nil, true, fmt.Errorf("nplate: %q object has unrecognised key %q", eachKey, k)
		}
	}
	tmplBytes, err := json.Marshal(rawTemplate)
	if err != nil {
		return nil, true, fmt.Errorf("nplate: marshal %q: %w", templateKey, err)
	}
	return &eachNode{Each: path, As: as, Template: tmplBytes}, true, nil
}

// expand walks node, replacing every well-formed "$each" object with the
// JSON array produced by iterating its path and rendering its template
// once per matched element. Everything else is copied through structurally
// unchanged -- any "$ref" leaves it contains are left for the final
// jsonplate.Render pass in Render, not resolved here.
func expand(node, data interface{}) (interface{}, error) {
	if each, isEach, err := asEach(node); isEach {
		if err != nil {
			return nil, err
		}
		return expandEach(each, data)
	}

	switch n := node.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(n))
		for k, v := range n {
			ev, err := expand(v, data)
			if err != nil {
				return nil, err
			}
			out[k] = ev
		}
		return out, nil
	case []interface{}:
		out := make([]interface{}, len(n))
		for i, v := range n {
			ev, err := expand(v, data)
			if err != nil {
				return nil, err
			}
			out[i] = ev
		}
		return out, nil
	default:
		return node, nil
	}
}

func expandEach(each *eachNode, data interface{}) (interface{}, error) {
	return queryfy.Collect(data, each.Each, func(item interface{}) (interface{}, error) {
		perItem := map[string]interface{}{each.As: item}

		var tmpl interface{}
		if err := json.Unmarshal(each.Template, &tmpl); err != nil {
			return nil, fmt.Errorf("nplate: unmarshal %q template: %w", eachKey, err)
		}
		// Recurse first, so a nested "$each" inside this template unrolls
		// against perItem before jsonplate ever sees the result -- nested
		// iteration is exactly this same function called again, not a
		// separate feature.
		expandedTmpl, err := expand(tmpl, perItem)
		if err != nil {
			return nil, err
		}
		raw, err := json.Marshal(expandedTmpl)
		if err != nil {
			return nil, fmt.Errorf("nplate: marshal expanded item template: %w", err)
		}
		rendered, err := jsonplate.Render(raw, perItem)
		if err != nil {
			return nil, fmt.Errorf("nplate: render item template: %w", err)
		}
		var v interface{}
		if err := json.Unmarshal(rendered, &v); err != nil {
			return nil, fmt.Errorf("nplate: unmarshal rendered item: %w", err)
		}
		return v, nil
	})
}

// validate walks node checking every "$each" object's shape (asEach's own
// error, if any) and recursing into its template; jsonplate's own $ref
// shape checking happens once, in Validate, against the whole
// $each-stripped document -- not duplicated here.
func validate(node interface{}) error {
	if _, isEach, err := asEach(node); isEach {
		if err != nil {
			return err
		}
		m := node.(map[string]interface{})
		var tmpl interface{}
		if err := json.Unmarshal(mustMarshal(m[templateKey]), &tmpl); err != nil {
			return fmt.Errorf("nplate: unmarshal %q template: %w", templateKey, err)
		}
		return validate(tmpl)
	}

	switch n := node.(type) {
	case map[string]interface{}:
		for _, v := range n {
			if err := validate(v); err != nil {
				return err
			}
		}
		return nil
	case []interface{}:
		for _, v := range n {
			if err := validate(v); err != nil {
				return err
			}
		}
		return nil
	default:
		return nil
	}
}

// stripEach replaces every "$each" node with an empty JSON array, so the
// result can be handed to jsonplate.Validate to check every "$ref"
// outside an "$each" block without nplate re-implementing that check.
func stripEach(node interface{}) (interface{}, error) {
	if _, isEach, err := asEach(node); isEach {
		if err != nil {
			return nil, err
		}
		return []interface{}{}, nil
	}
	switch n := node.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(n))
		for k, v := range n {
			sv, err := stripEach(v)
			if err != nil {
				return nil, err
			}
			out[k] = sv
		}
		return out, nil
	case []interface{}:
		out := make([]interface{}, len(n))
		for i, v := range n {
			sv, err := stripEach(v)
			if err != nil {
				return nil, err
			}
			out[i] = sv
		}
		return out, nil
	default:
		return node, nil
	}
}

func mustMarshal(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}
