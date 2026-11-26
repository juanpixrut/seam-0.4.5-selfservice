package auth

import (
	"errors"
	"strings"

	xolu "github.com/ha1tch/xolu/pkg/client"
)

// EntityNotYetCreated reports whether err is xolu's answer to an OQL query
// that names an entity with no rows yet. xolu has no "create entity" step: an
// entity exists from its first row, and until then a SELECT on it fails with
// XOLU-QL004 "entity '<name>' does not exist" (unless the server runs with
// XOLU_OQL_UNKNOWN_ENTITY=empty). For a store that reads before it writes,
// that answer means "nothing stored yet", not a failure.
//
// The code alone is too broad to go by: XOLU-QL004 is xolu's general "query
// failed" bucket. The message comes from one place in xolu
// (pkg/oql/validator.go).
func EntityNotYetCreated(err error) bool {
	var xerr *xolu.Error
	if !errors.As(err, &xerr) {
		return false
	}
	return xerr.Code == "XOLU-QL004" && strings.Contains(xerr.Message, "does not exist")
}
