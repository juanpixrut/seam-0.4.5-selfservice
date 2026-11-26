package ui

import (
	"fmt"

	mi "github.com/ha1tch/minty"
)

// AssetListData holds data for the assets list page.
type AssetListData struct {
	Assets         []AssetRow
	Total          int
	Page           int
	PageSize       int
	SearchQuery    string
	StatusFilter   string
	TypeFilter     string
	AssetTypes     []AssetTypeOption
	AutoOpenCreate bool // redirect from /assets/new: auto-fire create modal
}

// AssetRow represents an asset in the list.
type AssetRow struct {
	ID               int64
	Name             string
	TypeName         string
	Status           string
	QRCode           string
	LastEventTime    string
	ConnectivityTier string
	Criticality      string
	CustodianName    string
	GIAI             string
}

// AssetsListPage renders the assets list page.
// AssetsListPage renders the assets list page. t is a translate
// function (typically h.translateFunc(r)) for this page's own chrome
// text -- column headers, the New/Edit Asset labels -- found hardcoded
// Spanish regardless of locale (the tech team, testing the Mac demo
// hands-on, 2026-07-31), a genuinely different rendering path from
// formengine's own widgets (already fixed separately) since this page
// is hand-coded HTML, not schema-driven. Column headers reuse the
// exact same "assets.forms.asset.field.*" keys the dynamic form
// already resolves for its own field titles (identical meaning --
// "Criticality" is "Criticality" whether it labels a form field or a
// list column -- so reusing avoids maintaining the same translation
// twice); only "Actions", "New Asset", and "Edit Asset" needed new
// keys, since nothing already covered those.
// criticalityLabel is a stored criticality in the viewer's language. The data
// holds the Spanish words (Alta, Media, Baja; T-137); a value it does not
// know, and any value when there is no translator, is shown as stored.
func criticalityLabel(t func(key string, args ...any) string, stored string) string {
	key := map[string]string{"Alta": "asset.criticality.high", "Media": "asset.criticality.medium", "Baja": "asset.criticality.low"}[stored]
	if key == "" || t == nil {
		return stored
	}
	if s := t(key); s != "" && s != key {
		return s
	}
	return stored
}

func AssetsListPage(data AssetListData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	newAssetLabel := t("asset.new")
	rows := make([]mi.H, len(data.Assets))
	for i, asset := range data.Assets {
		rows[i] = ListRow(i,
			// Asset name: the key link of this navigation list; the whole row opens the
			// asset too (SeamList), so there is no separate "view" button.
			ListCellLink(asset.Name, fmt.Sprintf("/assets/%d", asset.ID)),
			ListCellMuted(asset.TypeName),
			ListCellMuted(asset.GIAI),
			// Estado as pill
			ListCellRich(func(b *mi.Builder) mi.Node {
				return StatusBadge(asset.Status)(b)
			}),
			// Criticality as pill
			ListCellRich(func(b *mi.Builder) mi.Node {
				v := map[string]string{
					"Alta":  "danger",
					"Media": "warning",
					"Baja":  "success",
				}[asset.Criticality]
				if v == "" {
					v = "default"
				}
				return Badge(criticalityLabel(t, asset.Criticality), v)(b)
			}),
			ListCellMuted(asset.CustodianName),
			ListActions(ListActionsConfig{
				EditURL:        fmt.Sprintf("/assets/dynamic/%d/edit", asset.ID),
				EditHTMX:       true,
				EditModalTitle: t("asset.edit"),
				T:              t,
			}),
		)
	}
	// Auto-open create modal when redirected from /assets/new
	autoOpen := func(b *mi.Builder) mi.Node { return b.Span() }
	if data.AutoOpenCreate {
		autoOpen = func(b *mi.Builder) mi.Node {
			return b.Script(mi.Raw(fmt.Sprintf(`(function(){
  window.addEventListener("load",function(){
    if(document.getElementById("seam-modal-body"))return;
    SeamModal.open(%q);
    htmx.ajax("GET","/assets/dynamic/new",{target:"#seam-modal-body",swap:"innerHTML"});
  });
})();`, newAssetLabel)))
		}
	}
	emptyAction := IconButton("add", newAssetLabel, "primary",
		mi.Attr("hx-get", "/assets/dynamic/new"),
		mi.Attr("hx-target", "#seam-modal-body"),
		mi.Attr("hx-swap", "innerHTML"),
		mi.Attr("onclick", fmt.Sprintf("SeamModal.open(%q)", newAssetLabel)),
	)
	return func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("space-y-6"),
			autoOpen(b),
			ListPage(
				ListPageConfig{
					Title:       t("nav.assets"),
					CreateURL:   "/assets/dynamic/new",
					CreateLabel: newAssetLabel,
					CreateHTMX:  true,
				},
				assetsFilterBar(data, t),
				ListTable(ListTableConfig{
					Columns: []ListColumn{
						{Label: t("assets.forms.asset.field.name")}, {Label: t("assets.forms.asset.field.type_id")}, {Label: t("assets.forms.asset.field.giai")},
						{Label: t("assets.forms.asset.field.status")}, {Label: t("assets.forms.asset.field.criticality")}, {Label: t("assets.forms.asset.field.custodian_id")},
						{Label: t("asset.list.actions"), Right: true},
					},
					EmptyIcon: "inventory", EmptyTitle: t("assets.empty_title"),
					EmptyMessage: t("assets.empty_message"),
					EmptyAction:  emptyAction,
				}, rows),
				ListPaginationBar(ListPaginationConfig{
					Page: data.Page, PageSize: data.PageSize,
					Visible: len(data.Assets), Total: data.Total,
					BaseURL: "/assets", Label: t("nav.assets"),
				}, t),
			)(b),
		)
	}
}

func assetsFilterBar(data AssetListData, t func(key string, args ...any) string) mi.H {
	return func(b *mi.Builder) mi.Node {
		// Status options
		statusOptions := []SelectOption{
			{Value: "", Label: t("sensors.filter.all_statuses")},
			{Value: "active", Label: t("asset.status.active"), Selected: data.StatusFilter == "active"},
			{Value: "silent", Label: t("asset.status.silent"), Selected: data.StatusFilter == "silent"},
			{Value: "alert", Label: t("asset.status.alert"), Selected: data.StatusFilter == "alert"},
			{Value: "lost", Label: t("asset.status.lost"), Selected: data.StatusFilter == "lost"},
		}

		// Type options
		typeOptions := []SelectOption{{Value: "", Label: t("sensors.filter.all_types")}}
		for _, at := range data.AssetTypes {
			typeOptions = append(typeOptions, SelectOption{
				Value:    fmt.Sprintf("%d", at.ID),
				Label:    at.Name,
				Selected: data.TypeFilter == fmt.Sprintf("%d", at.ID),
			})
		}

		return b.Form(mi.Method("get"), mi.Action("/assets"), mi.Class("flex items-center gap-4"),
			// Search
			b.Div(mi.Class("flex-1 max-w-md"),
				SearchInput("q", t("assets.search_placeholder"), data.SearchQuery)(b),
			),

			// Status filter
			b.Select(append([]interface{}{
				mi.Name("status"),
				mi.Class("rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm"),
				mi.Attr("onchange", "this.form.submit()"),
			}, selectOptions(b, statusOptions)...)...),

			// Type filter
			b.Select(append([]interface{}{
				mi.Name("type"),
				mi.Class("rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm"),
				mi.Attr("onchange", "this.form.submit()"),
			}, selectOptions(b, typeOptions)...)...),
		)
	}
}

func selectOptions(b *mi.Builder, options []SelectOption) []interface{} {
	result := make([]interface{}, len(options))
	for i, opt := range options {
		attrs := []interface{}{mi.Value(opt.Value)}
		if opt.Selected {
			attrs = append(attrs, mi.Attr("selected", "selected"))
		}
		attrs = append(attrs, opt.Label)
		result[i] = b.Option(attrs...)
	}
	return result
}
