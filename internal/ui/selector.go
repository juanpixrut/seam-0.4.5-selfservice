package ui

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	mi "github.com/ha1tch/minty"

	"github.com/ha1tch/seam-ui/internal/auth"
)

// =============================================================================
// SELECTOR (T-96)
// =============================================================================
// A common modal dialog for choosing ONE record out of a list: a search box,
// a table laid out with the List primitives, Prev/Next paging, and a "choose"
// action per row. Which entity it lists is decided by a server-side registry
// (selectorSources), keyed by a short name. The client sends the registry
// key, never an entity or field name -- the same trust boundary as
// pickerSearchConfigs in asset_picker_search.go (T-63 injection history).
//
// Opening it:
//
//	SeamModal2.open(title, '/selector/users?target=<hidden-input-id>', {maxWidth:'720px'})
//
// Choosing a row calls SeamSelector.choose(row) (web/static/js/app.js), which
// reads data-id / data-label from the row with dataset (never innerHTML),
// dispatches a "seam:selector-chosen" CustomEvent on document with
// detail {key, id, label, target}, and closes the modal. The opener (the
// any-picker widget, T-97) sets its hidden input and visible label from that
// event. Record labels are never interpolated into inline script.
//
// Single select only for now; multi-select is a later extension.

const (
	selectorDefaultPageSize = 10
	selectorMaxPage         = 200 // paging fetches TOP (page*size+1): keep deep pages bounded
	selectorMaxQueryRunes   = 100
)

// selectorTargetPattern limits the opener-supplied target (the id of the
// opener's hidden input) to characters that are safe in an HTML id.
var selectorTargetPattern = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,64}$`)

// Values of UIHandler.selectorUsersScope.
const (
	// SelectorUsersScopeHierarchy (the default) lists only users whose role
	// rank is equal to or lower than the caller's.
	SelectorUsersScopeHierarchy = "hierarchy"
	// SelectorUsersScopeTenant lists every active user of the caller's tenant.
	SelectorUsersScopeTenant = "tenant"
)

// SetSelectorUsersScope sets which users the Users selector lists
// (SelectorUsersScopeHierarchy or SelectorUsersScopeTenant). It reports
// false and changes nothing for any other value. Call it at startup, before
// the handler serves requests; it is not guarded by a lock.
func (h *UIHandler) SetSelectorUsersScope(scope string) bool {
	switch scope {
	case SelectorUsersScopeHierarchy, SelectorUsersScopeTenant:
		h.selectorUsersScope = scope
		return true
	}
	return false
}

// SelectorRow is one selectable record. Label is what the opener shows once
// the row is chosen; Data carries whatever the source's column renderers
// need (never secrets such as password hashes).
type SelectorRow struct {
	ID    int64
	Label string
	Data  map[string]any

	// Value, when set, is what the row hands to the opener in place of ID:
	// a source whose records are not identified by a number (a query-picker
	// with a text value column) uses it.
	Value string
}

// SelectorColumn is one displayed column. LabelKey is an i18n key for the
// header; Cell renders the row's cell (use ListCell / ListCellMuted /
// ListCellRich and friends).
type SelectorColumn struct {
	LabelKey string
	Right    bool
	Cell     func(r SelectorRow, t func(key string, args ...any) string) mi.H
}

// SelectorSource describes one selectable entity. Fetch is a function, not
// a table+column pair, so a source can go through a service (the Users
// source uses auth.UserService and so never touches password hashes).
// Fetch returns at most pageSize rows for the 1-based page and whether
// another page follows.
type SelectorSource struct {
	Key             string
	TitleKey        string
	EmptyIcon       string
	EmptyTitleKey   string
	EmptyMessageKey string
	Columns         []SelectorColumn
	PageSize        int
	Fetch           func(ctx context.Context, h *UIHandler, q string, page, pageSize int) (rows []SelectorRow, hasMore bool, err error)

	// URL, when set, builds the dialog's own links (search and paging) in
	// place of /selector/<Key>: a source served from another route (the
	// query-picker's Browse, picker_query.go) keeps its requests there.
	URL func(q string, page int, target string) string
}

// url is the address of this source's dialog for a search text and a page.
func (s SelectorSource) url(q string, page int, target string) string {
	if s.URL != nil {
		return s.URL(q, page, target)
	}
	return selectorURL(s.Key, q, page, target)
}

// selectorSources is the server-side registry. Add an entry here to make a
// new entity selectable; nothing the client sends can add to it.
var selectorSources = map[string]SelectorSource{
	"users":        usersSelectorSource,
	"assets":       assetsSelectorSource,
	"cost_centers": costCentersSelectorSource,
	"locations":    locationsSelectorSource,
	"custodians":   custodiansSelectorSource,
}

// Selector handles GET /selector/{key}?q=<text>&page=<n>&target=<id>.
// It always answers with an HTML fragment (the modal body, or the results
// block when an htmx search/paging request swaps just that part). An
// unknown key is a 400.
func (h *UIHandler) Selector(w http.ResponseWriter, r *http.Request) {
	src, ok := selectorSources[r.PathValue("key")]
	if !ok {
		http.Error(w, "unknown selector", http.StatusBadRequest)
		return
	}

	query := r.URL.Query()
	q := strings.TrimSpace(query.Get("q"))
	if rs := []rune(q); len(rs) > selectorMaxQueryRunes {
		q = string(rs[:selectorMaxQueryRunes])
	}
	page := 1
	if p, err := strconv.Atoi(query.Get("page")); err == nil && p > 1 {
		page = p
	}
	if page > selectorMaxPage {
		page = selectorMaxPage
	}
	target := query.Get("target")
	if !selectorTargetPattern.MatchString(target) {
		target = ""
	}
	pageSize := src.PageSize
	if pageSize <= 0 {
		pageSize = selectorDefaultPageSize
	}

	rows, hasMore, err := src.Fetch(r.Context(), h, q, page, pageSize)
	if err != nil {
		h.logger.Error("selector fetch failed", "selector", src.Key, "error", err)
		http.Error(w, h.errText(r, "error.search_failed", "selector failed"), http.StatusInternalServerError)
		return
	}

	t := h.translateFunc(r)
	if err := h.page.RenderPartial(w, selectorBody(src, rows, hasMore, q, page, target, t)); err != nil {
		h.logger.Error("selector render failed", "selector", src.Key, "error", err)
	}
}

// selectorURL builds the URL of one selector view. Values are query-encoded;
// empty q/target and page 1 are omitted.
func selectorURL(key, q string, page int, target string) string {
	v := url.Values{}
	if q != "" {
		v.Set("q", q)
	}
	if page > 1 {
		v.Set("page", strconv.Itoa(page))
	}
	if target != "" {
		v.Set("target", target)
	}
	u := "/selector/" + url.PathEscape(key)
	if len(v) > 0 {
		u += "?" + v.Encode()
	}
	return u
}

// selectorBody renders the dialog body: the search box (outside the swapped
// block so it keeps focus while typing) and the results block (table plus
// pager), which htmx search and paging requests replace in place.
func selectorBody(src SelectorSource, rows []SelectorRow, hasMore bool, q string, page int, target string, t func(key string, args ...any) string) mi.H {
	const (
		resultsSel = "#seam-selector-results"
		pagerBtn   = "secondary"
	)
	emptyIcon := src.EmptyIcon
	if emptyIcon == "" {
		emptyIcon = "search"
	}

	return func(b *mi.Builder) mi.Node {
		cols := make([]ListColumn, 0, len(src.Columns)+1)
		for _, c := range src.Columns {
			cols = append(cols, ListColumn{Label: t(c.LabelKey), Right: c.Right})
		}
		cols = append(cols, ListColumn{Label: "", Right: true})

		listRows := make([]mi.H, len(rows))
		for i, row := range rows {
			cells := make([]mi.H, 0, len(src.Columns)+1)
			for _, c := range src.Columns {
				cells = append(cells, c.Cell(row, t))
			}
			cells = append(cells, ListCellRight(Button(t("selector.choose"), "primary",
				mi.Attr("type", "button"),
				mi.Attr("onclick", "event.stopPropagation();SeamSelector.choose(this.closest('tr'))"),
			)))
			rowID := row.Value
			if rowID == "" {
				rowID = strconv.FormatInt(row.ID, 10)
			}
			attrs := []interface{}{
				mi.Attr("data-id", rowID),
				mi.Attr("data-label", row.Label),
				mi.Attr("tabindex", "0"),
				mi.Attr("style", "cursor:pointer"),
				mi.Attr("onclick", "SeamSelector.choose(this)"),
				mi.Attr("onkeydown", "if(event.key==='Enter'){SeamSelector.choose(this)}"),
			}
			listRows[i] = ListRowWithAttrs(i, attrs, cells...)
		}

		table := ListTable(ListTableConfig{
			Columns:      cols,
			EmptyIcon:    emptyIcon,
			EmptyTitle:   t(src.EmptyTitleKey),
			EmptyMessage: t(src.EmptyMessageKey),
			SkipCard:     true,
		}, listRows)(b)

		// Prev / Next driven by hasMore (no total is available).
		prev := Button(t("action.previous"), pagerBtn, mi.Attr("type", "button"), mi.Attr("disabled", "disabled"))
		if page > 1 {
			prev = Button(t("action.previous"), pagerBtn, mi.Attr("type", "button"),
				mi.Attr("hx-get", src.url(q, page-1, target)),
				mi.Attr("hx-target", resultsSel), mi.Attr("hx-select", resultsSel), mi.Attr("hx-swap", "outerHTML"))
		}
		next := Button(t("action.next"), pagerBtn, mi.Attr("type", "button"), mi.Attr("disabled", "disabled"))
		if hasMore {
			next = Button(t("action.next"), pagerBtn, mi.Attr("type", "button"),
				mi.Attr("hx-get", src.url(q, page+1, target)),
				mi.Attr("hx-target", resultsSel), mi.Attr("hx-select", resultsSel), mi.Attr("hx-swap", "outerHTML"))
		}
		pager := b.Div(mi.Class("flex items-center justify-between pt-2"),
			prev(b),
			b.Span(mi.Class("text-sm text-gray-500 dark:text-gray-400"), t("selector.page", page)),
			next(b),
		)

		// The search input carries only the key/target in its URL; htmx adds
		// q=<value> because the input has a name. Typing resets to page 1.
		search := SearchInput("q", t("selector.search.placeholder"), q,
			mi.Attr("hx-get", src.url("", 1, target)),
			mi.Attr("hx-trigger", "keyup changed delay:250ms, search"),
			mi.Attr("hx-target", resultsSel),
			mi.Attr("hx-select", resultsSel),
			mi.Attr("hx-swap", "outerHTML"),
			mi.Attr("autocomplete", "off"),
		)

		return b.Div(
			mi.Attr("id", "seam-selector"),
			mi.Attr("data-selector-key", src.Key),
			mi.Attr("data-selector-target", target),
			mi.Class("space-y-4"),
			search(b),
			b.Div(mi.Attr("id", "seam-selector-results"), table, pager),
		)
	}
}

// =============================================================================
// Users source
// =============================================================================

var errSelectorNoUserService = errors.New("selector: user service not configured")

var usersSelectorSource = SelectorSource{
	Key:             "users",
	TitleKey:        "selector.title.users",
	EmptyIcon:       "people_outline",
	EmptyTitleKey:   "selector.empty_title",
	EmptyMessageKey: "selector.empty_message",
	PageSize:        selectorDefaultPageSize,
	Columns: []SelectorColumn{
		{
			LabelKey: "users.column.user",
			Cell: func(r SelectorRow, t func(key string, args ...any) string) mi.H {
				name, _ := r.Data["name"].(string)
				email, _ := r.Data["email"].(string)
				return ListCellRich(func(b *mi.Builder) mi.Node {
					return b.Div(mi.Class("flex items-center gap-3"),
						b.Div(mi.Class("w-9 h-9 rounded-full bg-indigo-100 dark:bg-indigo-900 flex items-center justify-center flex-shrink-0"),
							b.Span(mi.Class("text-sm font-medium text-indigo-600 dark:text-indigo-400"), userInitialsFromName(name)),
						),
						b.Div(
							b.Div(mi.Class("font-medium text-gray-900 dark:text-white text-sm"), name),
							b.Div(mi.Class("text-xs text-gray-500 dark:text-gray-400"), email),
						),
					)
				})
			},
		},
		{
			LabelKey: "field.role",
			Cell: func(r SelectorRow, t func(key string, args ...any) string) mi.H {
				role, _ := r.Data["role"].(string)
				return ListCellRich(func(b *mi.Builder) mi.Node { return userRoleBadge(b, role, t) })
			},
		},
	},
	Fetch: fetchUsersSelectorRows,
}

// visibleUsersFilter builds the auth.UserFilter for "the users the caller
// may see", the one rule shared by the Users selector and the users picker
// (list AND id lookup, see userVisibleToCaller). ok is false when the caller
// may see nobody: no session claims, a tenant id <= 0 (the tenant condition
// is never dropped), or, in hierarchy scope, an unknown or empty role.
func (h *UIHandler) visibleUsersFilter(ctx context.Context, q string, limit, offset int) (filter auth.UserFilter, ok bool) {
	claims, has := auth.ClaimsFromContext(ctx)
	if !has || claims == nil || claims.TenantID <= 0 {
		return auth.UserFilter{}, false
	}
	active := true
	filter = auth.UserFilter{
		TenantID: claims.TenantID,
		Search:   q,
		Active:   &active,
		Limit:    limit,
		Offset:   offset,
	}
	if h.selectorUsersScope != SelectorUsersScopeTenant {
		roles := auth.RolesAtOrBelow(claims.Role)
		if len(roles) == 0 {
			return auth.UserFilter{}, false
		}
		filter.Roles = roles
	}
	return filter, true
}

// usersSearchScanMax bounds how many visible users listVisibleUsers loads to
// search and page in memory.
const usersSearchScanMax = 1000

// searchFold maps the accented letters of Spanish, Portuguese and French to
// their plain forms, so "jose" finds "Jos\u00e9" and "perez" finds "P\u00e9rez".
var searchFold = strings.NewReplacer(
	"\u00e1", "a", "\u00e0", "a", "\u00e2", "a", "\u00e4", "a", "\u00e3", "a",
	"\u00e9", "e", "\u00e8", "e", "\u00ea", "e", "\u00eb", "e",
	"\u00ed", "i", "\u00ec", "i", "\u00ee", "i", "\u00ef", "i",
	"\u00f3", "o", "\u00f2", "o", "\u00f4", "o", "\u00f6", "o", "\u00f5", "o",
	"\u00fa", "u", "\u00f9", "u", "\u00fb", "u", "\u00fc", "u",
	"\u00f1", "n", "\u00e7", "c",
)

// foldForSearch lower-cases, trims and strips accents, for matching only.
func foldForSearch(s string) string {
	return searchFold.Replace(strings.ToLower(strings.TrimSpace(s)))
}

// listVisibleUsers returns the users the caller may see (visibleUsersFilter's
// rule) whose name or email contains q, ignoring case and accents, ordered by name, after
// skipping offset and at most limit of them (limit <= 0: no limit). The search
// runs here, not in OQL: OQL's LIKE is case-sensitive (typing "ana" never found
// "Ana"), and a name/email OR-LIKE query is not worth depending on for a list
// that is small per tenant. Only the first usersSearchScanMax visible users
// (by name) are searched. Nobody visible gives a nil slice and no error.
func (h *UIHandler) listVisibleUsers(ctx context.Context, q string, limit, offset int) ([]*auth.User, error) {
	filter, ok := h.visibleUsersFilter(ctx, "", usersSearchScanMax, 0)
	if !ok {
		return nil, nil
	}
	users, err := h.userService.ListUsers(ctx, filter)
	if err != nil {
		return nil, err
	}
	needle := foldForSearch(q)
	matched := make([]*auth.User, 0, len(users))
	for _, u := range users {
		if needle == "" ||
			strings.Contains(foldForSearch(u.Name), needle) ||
			strings.Contains(foldForSearch(u.Email), needle) {
			matched = append(matched, u)
		}
	}
	if offset > 0 {
		if offset >= len(matched) {
			return nil, nil
		}
		matched = matched[offset:]
	}
	if limit > 0 && len(matched) > limit {
		matched = matched[:limit]
	}
	return matched, nil
}

// userVisibleToCaller is visibleUsersFilter's rule applied to one already
// loaded user (used by the picker's id lookup, which must not reveal a user
// the list would hide): same tenant, active, and, in hierarchy scope, a role
// at or below the caller's.
func (h *UIHandler) userVisibleToCaller(ctx context.Context, u *auth.User) bool {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok || claims == nil || claims.TenantID <= 0 || u == nil {
		return false
	}
	if u.TenantID != claims.TenantID || !u.Active {
		return false
	}
	if h.selectorUsersScope == SelectorUsersScopeTenant {
		return true
	}
	return auth.RoleAtOrBelow(claims.Role, u.Role)
}

// userSelectorLabel is the text an opener shows for a chosen user: the name
// with the email in parentheses (names are not unique), or just the email
// when there is no name.
func userSelectorLabel(name, email string) string {
	if name == "" {
		return email
	}
	if email == "" {
		return name
	}
	return name + " (" + email + ")"
}

// fetchUsersSelectorRows is the Users source. It goes through
// auth.UserService, so password hashes never leave the auth package, and it
// always applies:
//
//   - the session tenant (T-98: all users belong to the current tenant; the
//     omission of any cross-tenant path is deliberate). A missing session or
//     tenant id <= 0 lists nobody rather than dropping the tenant condition.
//   - active users only.
//   - search by name or email, ignoring case (listVisibleUsers).
//   - by default (SelectorUsersScopeHierarchy) only users whose role rank is
//     equal to or lower than the caller's; an unknown or empty caller role
//     lists nobody. SelectorUsersScopeTenant lifts the role restriction.
//
// The picker's id lookup applies the same rule (userVisibleToCaller, T-97).
func fetchUsersSelectorRows(ctx context.Context, h *UIHandler, q string, page, pageSize int) ([]SelectorRow, bool, error) {
	if h.userService == nil {
		return nil, false, errSelectorNoUserService
	}
	// Limit pageSize+1: the one extra row tells us whether another page exists.
	users, err := h.listVisibleUsers(ctx, q, pageSize+1, (page-1)*pageSize)
	if err != nil {
		if entityNotYetCreated(err) {
			return []SelectorRow{}, false, nil
		}
		return nil, false, err
	}

	hasMore := len(users) > pageSize
	if hasMore {
		users = users[:pageSize]
	}
	rows := make([]SelectorRow, 0, len(users))
	for _, u := range users {
		rows = append(rows, SelectorRow{
			ID:    u.ID,
			Label: userSelectorLabel(u.Name, u.Email),
			Data: map[string]any{
				"name":  u.Name,
				"email": u.Email,
				"role":  u.Role,
			},
		})
	}
	return rows, hasMore, nil
}
