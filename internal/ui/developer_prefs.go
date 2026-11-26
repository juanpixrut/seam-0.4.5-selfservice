package ui

import (
	"context"
	"net/http"
	"net/url"
	"regexp"

	"github.com/ha1tch/seam-ui/internal/auth"
)

// Per-developer preferences (campaign DEV-29): what a developer last chose
// where the tools offer a choice that is theirs and not the tenant's, so the
// next page opens the same way. Today that is the scope the editors save into
// and the language the Translations list shows.
//
// Where they are kept is a setting (developer_preferences):
//
//	account  (default) with the developer's user record, as two fields of
//	         their own beside the application's preferences, so they follow
//	         the developer to another browser. The account is what the
//	         pages that offer the choices read (one read of the user record
//	         for each such page); the browser's cookie is a copy, brought up
//	         to date on each of them, and what is used when the account
//	         cannot be read. A choice writes both, so a choice made in
//	         another browser is the one this browser opens with next. An
//	         account that has none yet takes what the cookie holds.
//	browser  in a cookie only, limited to the module's pages.
//
// Either way they are not part of the tenant's runtime state (DEC-6): they
// change nothing for anyone else, and losing them costs one click. A choice
// made in the address always wins and becomes the new preference; a
// remembered scope that is not on offer (the directory is gone, the global
// scope is someone else's) is ignored by the editors, which fall back to the
// first.

// The values of the developer_preferences setting.
const (
	DeveloperPreferencesAccount = "account"
	DeveloperPreferencesBrowser = "browser"
)

// The fields of the user record the preferences are kept in.
const (
	developerPrefScopeField  = "developer_scope"
	developerPrefLocaleField = "developer_locale"
)

const developerPrefsCookie = "seam_dev_prefs"

var localePrefRE = regexp.MustCompile(`^[a-z]{2,3}(_[a-z0-9]+)*$`)

// developerPrefs are the remembered choices; an empty one is "no preference".
type developerPrefs struct{ Scope, Locale string }

func readDeveloperPrefs(r *http.Request) developerPrefs {
	c, err := r.Cookie(developerPrefsCookie)
	if err != nil {
		return developerPrefs{}
	}
	v, err := url.ParseQuery(c.Value)
	if err != nil {
		return developerPrefs{}
	}
	return cleanDeveloperPrefs(v.Get("scope"), v.Get("locale"))
}

// cleanDeveloperPrefs keeps only values a preference can have.
func cleanDeveloperPrefs(scope, locale string) developerPrefs {
	p := developerPrefs{Scope: scope, Locale: locale}
	switch p.Scope {
	case scopeTenant, scopeFile, scopeGlobal:
	default:
		p.Scope = ""
	}
	if !localePrefRE.MatchString(p.Locale) {
		p.Locale = ""
	}
	return p
}

func writeDeveloperPrefs(w http.ResponseWriter, r *http.Request, p developerPrefs) {
	v := url.Values{}
	if p.Scope != "" {
		v.Set("scope", p.Scope)
	}
	if p.Locale != "" {
		v.Set("locale", p.Locale)
	}
	http.SetCookie(w, &http.Cookie{Name: developerPrefsCookie, Value: v.Encode(), Path: "/developer", MaxAge: 365 * 24 * 3600,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil})
}

// applyDeveloperPrefs is called first by the pages that offer the choices,
// on a GET: what the address chooses is remembered, and what it leaves out is
// filled in from what was remembered, so the rest of the handler reads the
// address as if the developer had typed it. withLocale says the page has a
// language to choose.
func (h *UIHandler) applyDeveloperPrefs(w http.ResponseWriter, r *http.Request, withLocale bool) {
	if r.Method != http.MethodGet {
		return
	}
	q := r.URL.Query()
	had := readDeveloperPrefs(r)
	// the account's are the ones in force; the cookie follows, and seeds an
	// account that has none
	account, fill, seed := h.developerPrefsInAccount(), false, false
	claims, signedIn := auth.ClaimsFromContext(r.Context())
	if account && signedIn {
		if kept, ok := h.accountDeveloperPrefs(r.Context(), claims.UserID); ok {
			if kept == (developerPrefs{}) {
				seed = had != kept
			} else {
				had, fill = kept, kept != had
			}
		}
	}
	now := had
	changed := false
	switch s := q.Get("scope"); s {
	case scopeTenant, scopeFile, scopeGlobal:
		now.Scope = s
	case "":
		if had.Scope != "" {
			q.Set("scope", had.Scope)
			changed = true
		}
	}
	if withLocale {
		switch l := q.Get("locale"); {
		case l != "" && h.translator != nil && h.translator.HasLocale(l):
			now.Locale = l
		case l == "" && had.Locale != "" && h.translator != nil && h.translator.HasLocale(had.Locale):
			q.Set("locale", had.Locale)
			changed = true
		}
	}
	if changed {
		r.URL.RawQuery = q.Encode()
		r.Form = nil // read again by whoever asks for a form value
	}
	if now != had || fill {
		writeDeveloperPrefs(w, r, now)
	}
	if (now != had || seed) && account && signedIn {
		h.saveAccountDeveloperPrefs(r.Context(), claims.UserID, now)
	}
}

// developerPrefsInAccount reports whether the preferences are kept with the
// user record (the setting's default) and there is somewhere to keep them.
func (h *UIHandler) developerPrefsInAccount() bool {
	return h.config.DevSettings.Preferences != DeveloperPreferencesBrowser && h.xoluClient != nil
}

// accountDeveloperPrefs reads the preferences kept with the user; false when
// the user cannot be read, which leaves the browser's copy in force.
func (h *UIHandler) accountDeveloperPrefs(ctx context.Context, userID int64) (developerPrefs, bool) {
	ent, err := h.xoluClient.Get(ctx, "users", userID)
	if err != nil || ent == nil {
		return developerPrefs{}, false
	}
	return cleanDeveloperPrefs(toString(ent.Data[developerPrefScopeField]), toString(ent.Data[developerPrefLocaleField])), true
}

// saveAccountDeveloperPrefs writes them: a merge of two fields, which leaves
// the rest of the record (and the application's own preferences) alone. A
// failure costs the developer nothing but the copy in another browser.
func (h *UIHandler) saveAccountDeveloperPrefs(ctx context.Context, userID int64, p developerPrefs) {
	if _, err := h.xoluClient.Patch(ctx, "users", userID, map[string]interface{}{
		developerPrefScopeField: p.Scope, developerPrefLocaleField: p.Locale,
	}); err != nil {
		h.logger.Warn("developer: preferences not saved with the account", "user_id", userID, "error", err)
	}
}
