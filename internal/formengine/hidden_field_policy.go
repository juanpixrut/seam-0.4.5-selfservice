package formengine

// CheckHiddenFieldSubmission applies §15.2's dual policy to a submitted
// form. FORM_ENGINE_SPEC.md's resolved decision: a field whose own
// x-seam-visible-when condition evaluates false against the submitted
// values is, by default, silently discarded — not persisted, not
// validated — since the client hid it and a value showing up for it
// anyway is most often stale DOM state, not user intent. Setting
// x-seam-hidden-reject: true on a specific field inverts that default
// to active rejection (the caller should respond 422) instead, for
// fields where a value arriving while hidden is itself suspicious
// enough to treat as a hard error (see FORM_ENGINE_SPEC.md §15.2 for
// the worked example this was resolved against).
//
// This mirrors the client evaluator's own §3b.6 behaviour server-side,
// so a manipulated DOM — or a client that never ran the JS at all —
// can't smuggle a value through for a field whose visibility condition
// doesn't hold. formengine only renders; it doesn't parse POST bodies
// or persist anything, so this is exported for a caller's handler to
// invoke after parsing submitted values and before persisting them —
// same division of responsibility as the CSRF token (formengine emits
// it, the handler verifies it).
//
// fields should be the form's full field list (Form.Fields); submitted
// should be the parsed POST values, keyed by field name.
func CheckHiddenFieldSubmission(fields []Field, submitted map[string]interface{}) (rejected, discarded []string) {
	for _, f := range fields {
		if f.XSeamVisibleWhen == nil {
			continue // no condition on this field -- nothing to check
		}
		if EvaluateVisibleWhen(f.XSeamVisibleWhen, submitted) {
			continue // condition holds -- field is legitimately visible, submitting a value for it is normal
		}
		v, present := submitted[f.Name]
		if !present || isEmpty(v) {
			continue // nothing meaningful submitted for this field -- nothing to discard or reject
		}
		if f.XSeamHiddenReject {
			rejected = append(rejected, f.Name)
		} else {
			discarded = append(discarded, f.Name)
		}
	}
	return rejected, discarded
}

// RequiredFieldsExemptByPermission returns the names of fields that are
// marked required in the schema but should not be enforced as required
// for this request, because they're hidden from the current user by
// permission (§3c.6): "un campo que el usuario no puede ver no puede
// ser obligatorio para ese usuario." Distinct from
// CheckHiddenFieldSubmission's concern — that's about a field hidden by
// its own visible_when condition (§15.2); this is about a field hidden
// by RenderContext.HiddenFields (§3c, role-based permission), a
// different mechanism entirely (§3c.1's table keeps them explicitly
// separate). A caller building server-side validation typically needs
// both checks, not one instead of the other.
func RequiredFieldsExemptByPermission(fields []Field, hiddenFields []string) []string {
	hidden := toSet(hiddenFields)
	var exempt []string
	for _, f := range fields {
		if f.Required && hidden[f.Name] {
			exempt = append(exempt, f.Name)
		}
	}
	return exempt
}

// in discarded removed. Convenience for the common case (the caller
// just wants the cleaned map, not the list of what was dropped) — a
// caller that needs to log or report what was silently discarded should
// use CheckHiddenFieldSubmission directly instead.
func StripDiscardedFields(submitted map[string]interface{}, discarded []string) map[string]interface{} {
	if len(discarded) == 0 {
		return submitted
	}
	cleaned := make(map[string]interface{}, len(submitted))
	drop := toSet(discarded)
	for k, v := range submitted {
		if drop[k] {
			continue
		}
		cleaned[k] = v
	}
	return cleaned
}
