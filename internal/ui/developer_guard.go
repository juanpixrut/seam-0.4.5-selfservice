package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/ha1tch/seam-ui/internal/auth"
	systemschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/system"
)

// This file holds the user-administration guards for the developer role
// (campaign DEV-6). Developer accounts can be created and managed only by
// developers; otherwise an admin could reset a developer's password, or
// promote anyone past admin, and so reach the Developer module.

// callerIdentity returns the caller's user ID and role from the request
// context. Both are zero values when there are no claims (auth disabled),
// which the rules in package auth treat as holding no privilege.
func callerIdentity(r *http.Request) (userID int64, role string) {
	if claims, ok := auth.ClaimsFromContext(r.Context()); ok {
		return claims.UserID, claims.Role
	}
	return 0, ""
}

// withEnumValue returns schema with value added to the enum of its property
// prop, changing nothing else: the document is returned byte for byte but for
// one insertion, because decoding and encoding it would reorder its keys and
// lose its formatting. It is returned unchanged when the property has no enum,
// already lists the value, or the document is not valid JSON.
func withEnumValue(schema []byte, prop, value string) []byte {
	type frame struct {
		obj, wantKey bool
		key          string
		n            int  // elements seen (arrays)
		has          bool // value is already one of them
	}
	dec := json.NewDecoder(bytes.NewReader(schema))
	var st []*frame
	valueDone := func() {
		if n := len(st); n > 0 {
			if f := st[n-1]; f.obj {
				f.wantKey = true
			} else {
				f.n++
			}
		}
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			return schema // the end without finding it, or not JSON
		}
		switch v := tok.(type) {
		case json.Delim:
			switch v {
			case '{':
				st = append(st, &frame{obj: true, wantKey: true})
			case '[':
				st = append(st, &frame{})
			default: // '}' or ']'
				n := len(st)
				f := st[n-1]
				// root{ "properties": { prop: { "enum": [ ...here
				if v == ']' && n == 4 && st[0].key == "properties" && st[1].key == prop && st[2].key == "enum" {
					if f.has {
						return schema
					}
					quoted, _ := json.Marshal(value)
					if f.n > 0 {
						quoted = append([]byte(","), quoted...)
					}
					at := int(dec.InputOffset()) - 1 // the ']' itself
					out := make([]byte, 0, len(schema)+len(quoted))
					return append(append(append(out, schema[:at]...), quoted...), schema[at:]...)
				}
				st = st[:n-1]
				valueDone()
			}
		case string:
			if n := len(st); n > 0 {
				f := st[n-1]
				if f.obj && f.wantKey {
					f.key, f.wantKey = v, false
					continue
				}
				if !f.obj && v == value {
					f.has = true
				}
			}
			valueDone()
		default:
			valueDone()
		}
	}
}

// userFormSchema is the user_create or user_edit schema as the caller may use
// it: its role select offers the developer role only to a caller who may assign
// it (auth.CanAssignRole), so nobody is shown a choice they would be refused.
func (h *UIHandler) userFormSchema(ctx context.Context, r *http.Request, name string) []byte {
	schema := resolveSchema(ctx, "system", name, systemschemas.Schema(name))
	if callerCanAssign(r, auth.RoleDeveloper) {
		schema = withEnumValue(schema, "role", auth.RoleDeveloper)
	}
	return schema
}

// callerCanAssign reports whether the caller may give targetRole to a user.
func callerCanAssign(r *http.Request, targetRole string) bool {
	_, role := callerIdentity(r)
	return auth.CanAssignRole(role, targetRole)
}

// denyManagingUser answers 403 and returns true when the caller may not
// manage a user whose current role is targetRole. Handlers call it as
//
//	if h.denyManagingUser(w, r, role) { return }
func (h *UIHandler) denyManagingUser(w http.ResponseWriter, r *http.Request, targetRole string) bool {
	userID, role := callerIdentity(r)
	if auth.CanManageUser(role, targetRole) {
		return false
	}
	if h.auditLogger != nil {
		h.auditLogger.LogAccessDenied(r.Context(), r, userID, r.URL.Path, string(auth.PermDeveloperAccess))
	}
	h.Error403(w, r)
	return true
}
