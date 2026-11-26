package ui

import (
	"context"
	"errors"
	"strconv"

	"github.com/ha1tch/seam-ui/internal/auth"
)

// "entity:users" for the picker backend (T-97). Unlike entity:assets it does
// not build OQL here: it goes through auth.UserService (so password hashes
// never leave the auth package) and applies the same visibility rule as the
// Users selector (T-96): the caller's tenant, active users only, and, in the
// default hierarchy scope, only roles at or below the caller's. The rule is
// applied to the live search AND to the id lookup, so the lookup cannot be
// used to read a user the list would hide.

const pickerUsersMaxResults = 20

var usersPickerConfig = pickerSearchConfig{
	table:        "users",
	searchColumn: "name",
	label: func(row map[string]any) string {
		return userSelectorLabel(toString(row["name"]), toString(row["email"]))
	},
	search: searchUsersForPicker,
	lookup: lookupUserForPicker,
}

// searchUsersForPicker is the live search: up to pickerUsersMaxResults users
// whose name or email contains q (ignoring case), ordered by name. excludeRaw (a user id to
// leave out) is honoured when it parses as an integer.
func searchUsersForPicker(ctx context.Context, h *UIHandler, q, excludeRaw string) ([]pickerSearchResult, error) {
	if h.userService == nil {
		return nil, errSelectorNoUserService
	}
	// One extra row so that excluding an id still leaves a full page.
	users, err := h.listVisibleUsers(ctx, q, pickerUsersMaxResults+1, 0)
	if err != nil {
		if entityNotYetCreated(err) {
			return []pickerSearchResult{}, nil
		}
		return nil, err
	}
	var excludeID int64
	var exclude bool
	if excludeRaw != "" {
		if id, perr := strconv.ParseInt(excludeRaw, 10, 64); perr == nil {
			excludeID, exclude = id, true
		}
	}
	out := make([]pickerSearchResult, 0, len(users))
	for _, u := range users {
		if exclude && u.ID == excludeID {
			continue
		}
		if len(out) == pickerUsersMaxResults {
			break
		}
		out = append(out, pickerSearchResult{ID: u.ID, Label: userSelectorLabel(u.Name, u.Email)})
	}
	return out, nil
}

// lookupUserForPicker resolves one user by exact id (the widget's label
// lookup for an already-set value). A user the caller may not see is "not
// found", indistinguishable from a missing id.
func lookupUserForPicker(ctx context.Context, h *UIHandler, id int64) ([]pickerSearchResult, error) {
	if h.userService == nil {
		return nil, errSelectorNoUserService
	}
	u, err := h.userService.GetUser(ctx, id)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) || entityNotYetCreated(err) {
			return []pickerSearchResult{}, nil
		}
		return nil, err
	}
	if !h.userVisibleToCaller(ctx, u) {
		return []pickerSearchResult{}, nil
	}
	return []pickerSearchResult{{ID: u.ID, Label: userSelectorLabel(u.Name, u.Email)}}, nil
}
