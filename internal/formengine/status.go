package formengine

import "strings"

// resolveFieldError merges the two error sources a field can have: the
// existing errs map (an exact fieldName->message lookup, unchanged
// since before RenderContext.Statuses existed) and rc.Statuses (the
// additive form, for elements whose identifier isn't a valid map key
// on its own -- nested array-item paths specifically). The errs map is
// checked first and wins if both would report something, preserving
// every existing caller's exact behaviour when Statuses is empty (the
// zero value, nil, for every caller before this field existed).
//
// Only "ERR" entries are consulted here -- "WRN"/"INF" are recorded on
// RenderContext.Statuses and matchable by callers that want them
// (statusesForField below), but resolveFieldError's job is specifically
// producing the same "is there a blocking error, and what does it say"
// signal renderFieldRow's existing errMsg/hasError already understand.
// Giving WRN/INF their own rendering treatment is later, separate work.
func resolveFieldError(f Field, errs map[string]string, rc RenderContext) string {
	if msg, ok := errs[f.Name]; ok && msg != "" {
		return msg
	}
	for _, s := range rc.Statuses {
		if s.Status != "ERR" {
			continue
		}
		if statusMatchesField(s.Element, f.Name) {
			return s.Message
		}
	}
	return ""
}

// statusMatchesField reports whether element belongs to fieldName --
// an exact match (scalar fields), or element continuing with "[" (an
// array index, e.g. fieldName="document_links", element=
// "document_links[0].name") or "." (a nested object field, should
// queryfy ever produce that shape for a field in this schema). A plain
// prefix check alone would wrongly match a hypothetical field named
// "document_links_extra" against a "document_links[0].x" status --
// requiring the very next character to be "[" or "." rules that out.
func statusMatchesField(element, fieldName string) bool {
	if element == fieldName {
		return true
	}
	if strings.HasPrefix(element, fieldName+"[") {
		return true
	}
	if strings.HasPrefix(element, fieldName+".") {
		return true
	}
	return false
}
