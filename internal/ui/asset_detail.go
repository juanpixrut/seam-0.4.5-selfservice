package ui

import (
	"fmt"

	mi "github.com/ha1tch/minty"
)

// AssetDetailData holds data for the asset detail page.
type AssetDetailData struct {
	Asset        AssetDetail
	RecentEvents []EventSummary
}

// EventSummary is a lightweight event view for the asset detail page's
// recent-events list. Moved here from the now-retired internal/ui/
// dashboard.go (attic/old-dashboard/) -- genuinely used outside the old
// dashboard (asset_detail.go, pages.go, routes.go), unlike AlertSummary
// (dashboard.go's own comment already called that one "kept for backward
// compatibility" -- zero real callers found, left retired).
type EventSummary struct {
	ID          int64
	AssetName   string
	TriggerType string
	Value       string
	Timestamp   string
}

// AssetDetail represents full asset information.
type AssetDetail struct {
	ID               int64
	Name             string
	TypeID           int64
	TypeName         string
	Status           string
	QRCode           string
	ConnectivityTier string
	FSMState         string
	GIAI             string
	GRAI             string
	CodigoBarrasRFID string
	TagTecnico       string
	// Block E — Lifecycle
	PurchaseDate        string
	ExpectedArrivalDate string
	ArrivalDate         string
	ObsolescenceDate    string
	ExpirationDate      string
	DecommissionDate    string
	DecommissionReason  string
	LinksCicloVida      []DocumentLink
	// Block B — Characterisation
	BrandID     int64
	ModelID     int64
	Serial1     string
	Serial2     string
	Lot         string
	Criticality string
	// Block C — Location
	LocationID int64
	RegionID   int64
	ParentID   int64
	// Block D — Custody
	CustodianID   int64
	TenureType    string
	LoanExpiry    string
	LinksComodato []DocumentLink
	// Block G — Finance
	PurchasePrice   string
	CurrencyID      int64
	CurrentPrice    string
	CostCenterID    int64
	WarrantyEnd     string
	LinksDocumentos []DocumentLink
	// Block F — Service/IoT
	TempMin            string
	TempMax            string
	Refrigerant        string
	VoltageConsumption string
	Identity           map[string]interface{}
	State              map[string]interface{}
	CreatedAt          string
	UpdatedAt          string
	LastEventTime      string
}

// AssetDetailPage renders the asset detail page.
func AssetDetailPage(data AssetDetailData, t func(key string, args ...any) string) mi.H {
	return func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("space-y-6"),
			// Page header with breadcrumb
			b.Div(mi.Class("flex items-center justify-between"),
				b.Div(
					// Breadcrumb
					b.Nav(mi.Class("text-sm text-gray-500 dark:text-gray-400 mb-2"),
						b.A(mi.Href("/assets"), mi.Class("hover:text-gray-700 dark:hover:text-gray-300"), t("nav.assets")),
						b.Span(mi.Class("mx-2"), "/"),
						b.Span(mi.Class("text-gray-900 dark:text-white"), data.Asset.Name),
					),
					// Title with status
					b.Div(mi.Class("flex items-center gap-3"),
						b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"),
							data.Asset.Name,
						),
						StatusBadge(data.Asset.Status)(b),
					),
				),
				// Actions
				b.Div(mi.Class("flex items-center gap-2"),
					b.Button(
						mi.Class("inline-flex items-center gap-1 px-3 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700"),
						mi.Attr("hx-get", fmt.Sprintf("/assets/dynamic/%d/edit", data.Asset.ID)),
						mi.Attr("hx-target", "#seam-modal-body"),
						mi.Attr("hx-swap", "innerHTML"),
						mi.Attr("onclick", fmt.Sprintf("SeamModal.open('%s')", t("asset.edit"))),
						b.Span(mi.Class("material-icons text-base leading-none"), "edit"),
						t("action.edit"),
					),
					Button(t("action.delete"), "danger",
						mi.Attr("onclick", fmt.Sprintf("SeamConfirm.showDanger('%s',function(){htmx.ajax('DELETE','/api/v1/assets/%d',{target:document.body,swap:'innerHTML'})},{title:'%s'})", t("asset.confirm_delete"), data.Asset.ID, t("asset.delete_title"))),
					)(b),
				),
			),

			// Main content grid
			b.Div(mi.Class("grid grid-cols-1 lg:grid-cols-3 gap-6"),
				// Left column - main info
				b.Div(mi.Class("lg:col-span-2 space-y-6"),
					// Basic info card
					Card(t("asset.info_title"), assetInfoContent(data.Asset, t))(b),

					// Identity fields card
					Card(t("assettypes.section.identity_fields"), assetIdentityContent(data.Asset.Identity, t))(b),

					// State fields card
					Card(t("assettypes.section.state_fields"), assetStateContent(data.Asset.State, t))(b),

					// Recent events
					CardWithActions(t("asset.recent_events"),
						func(b *mi.Builder) mi.Node {
							return b.A(mi.Href(fmt.Sprintf("/events?asset_id=%d", data.Asset.ID)),
								mi.Class("text-sm text-indigo-600 hover:text-indigo-500 dark:text-indigo-400"),
								t("asset.view_all_events"),
							)
						},
						assetEventsContent(data.RecentEvents, t),
					)(b),
				),

				// Right column - sidebar
				b.Div(mi.Class("space-y-6"),
					// QR Code card
					Card(t("asset.qr_code_title"), issueQRCardLoader(data.Asset.ID, t))(b),

					// FSM State card
					Card(t("asset.state_machine_title"), assetFSMContent(data.Asset.FSMState))(b),
				),
			),
		)
	}
}

func assetInfoContent(asset AssetDetail, t func(key string, args ...any) string) mi.H {
	return func(b *mi.Builder) mi.Node {
		return b.Dl(mi.Class("grid grid-cols-2 gap-4"),
			infoRow(b, t("field.type"), asset.TypeName),
			infoRow(b, t("field.status"), asset.Status),
			infoRow(b, t("assets.forms.asset.field.connectivity_tier"), asset.ConnectivityTier),
			infoRow(b, t("field.created"), asset.CreatedAt),
			infoRow(b, t("field.updated"), asset.UpdatedAt),
			infoRow(b, t("asset.last_event"), asset.LastEventTime),
		)
	}
}

func infoRow(b *mi.Builder, label, value string) mi.Node {
	return b.Div(
		b.Dt(mi.Class("text-sm font-medium text-gray-500 dark:text-gray-400"), label),
		b.Dd(mi.Class("mt-1 text-sm text-gray-900 dark:text-white"), value),
	)
}

func assetIdentityContent(identity map[string]interface{}, t func(key string, args ...any) string) mi.H {
	return func(b *mi.Builder) mi.Node {
		if len(identity) == 0 {
			return b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400"),
				t("assettypes.no_identity_fields"),
			)
		}

		rows := make([]interface{}, 0, len(identity))
		for key, value := range identity {
			rows = append(rows, infoRow(b, key, fmt.Sprintf("%v", value)))
		}

		return b.Dl(append([]interface{}{mi.Class("grid grid-cols-2 gap-4")}, rows...)...)
	}
}

func assetStateContent(state map[string]interface{}, t func(key string, args ...any) string) mi.H {
	return func(b *mi.Builder) mi.Node {
		if len(state) == 0 {
			return b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400"),
				t("assettypes.no_state_fields"),
			)
		}
		rows := make([]interface{}, 0, len(state))
		for key, value := range state {
			rows = append(rows, infoRow(b, key, fmt.Sprintf("%v", value)))
		}
		return b.Dl(append([]interface{}{mi.Class("grid grid-cols-2 gap-4")}, rows...)...)
	}
}

func assetEventsContent(events []EventSummary, t func(key string, args ...any) string) mi.H {
	return func(b *mi.Builder) mi.Node {
		if len(events) == 0 {
			return b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400 text-center py-4"),
				t("asset.no_events"),
			)
		}

		items := make([]interface{}, len(events))
		for i, evt := range events {
			items[i] = b.Div(mi.Class("flex items-center justify-between py-2 border-b border-gray-100 dark:border-gray-700 last:border-0"),
				b.Div(
					b.Div(mi.Class("text-sm font-medium text-gray-900 dark:text-white"), evt.TriggerType),
					b.Div(mi.Class("text-xs text-gray-500 dark:text-gray-400"), evt.Value),
				),
				b.Div(mi.Class("text-xs text-gray-400 dark:text-gray-500"), evt.Timestamp),
			)
		}

		return b.Div(items...)
	}
}

func assetQRContent(qrCode string, t func(key string, args ...any) string) mi.H {
	return func(b *mi.Builder) mi.Node {
		if qrCode == "" {
			return b.Div(mi.Class("text-center py-4"),
				b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400 mb-2"), t("asset.no_qr_code")),
				Button(t("asset.generate_qr"), "secondary")(b),
			)
		}

		return b.Div(mi.Class("text-center"),
			// Placeholder for QR code image
			b.Div(mi.Class("w-32 h-32 mx-auto bg-gray-100 dark:bg-gray-700 rounded-lg flex items-center justify-center mb-2"),
				b.Span(mi.Class("material-icons text-4xl text-gray-400"), "qr_code_2"),
			),
			b.P(mi.Class("text-sm font-mono text-gray-600 dark:text-gray-400"), qrCode),
		)
	}
}

func assetFSMContent(state string) mi.H {
	return func(b *mi.Builder) mi.Node {
		if state == "" {
			state = "unknown"
		}

		stateColor := "bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300"
		switch state {
		case "active", "online":
			stateColor = "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300"
		case "alert", "warning":
			stateColor = "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-300"
		case "critical", "offline":
			stateColor = "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-300"
		}

		return b.Div(mi.Class("text-center py-2"),
			b.Div(mi.Class("inline-block px-4 py-2 rounded-full text-sm font-medium "+stateColor),
				state,
			),
		)
	}
}

// AssetTypeOption represents an asset type option for the selector.
type AssetTypeOption struct {
	ID       int64
	Name     string
	Selected bool
}

// assetDynamicFields renders the user-defined identity and state fields from the asset type.
func assetDynamicFields(b *mi.Builder, identityFields, stateFields []FieldRow, values map[string]string, t func(key string, args ...any) string) mi.Node {
	if len(identityFields) == 0 && len(stateFields) == 0 {
		return b.Span() // empty — HTMX will replace this when type is selected
	}
	var sections []interface{}
	if len(identityFields) > 0 {
		var fields []interface{}
		for _, f := range identityFields {
			v := ""
			if values != nil {
				v = values["identity."+f.Key]
			}
			fields = append(fields, assetDynamicField(b, f, "identity."+f.Key, v))
		}
		sections = append(sections, Card(t("assettypes.section.identity_fields"), func(b *mi.Builder) mi.Node {
			inner := []interface{}{mi.Class("space-y-4")}
			inner = append(inner, fields...)
			return b.Div(inner...)
		})(b))
	}
	if len(stateFields) > 0 {
		var fields []interface{}
		for _, f := range stateFields {
			v := ""
			if values != nil {
				v = values["state."+f.Key]
			}
			fields = append(fields, assetDynamicField(b, f, "state."+f.Key, v))
		}
		sections = append(sections, Card(t("assettypes.section.state_fields"), func(b *mi.Builder) mi.Node {
			inner := []interface{}{mi.Class("space-y-4")}
			inner = append(inner, fields...)
			return b.Div(inner...)
		})(b))
	}
	inner := []interface{}{mi.Class("space-y-6")}
	inner = append(inner, sections...)
	return b.Div(inner...)
}

func assetDynamicField(b *mi.Builder, f FieldRow, name, value string) mi.Node {
	label := f.Label
	if label == "" {
		label = f.Key
	}
	inputType := f.Type
	if inputType == "" || inputType == "text" {
		inputType = "text"
	}
	class := "w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
	labelNode := b.Label(mi.For(name), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"), label)
	if f.Required {
		labelNode = b.Label(mi.For(name), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"),
			label, b.Span(mi.Class("text-red-500 ml-1"), "*"))
	}
	var inputNode mi.Node
	switch inputType {
	case "number":
		if f.Required {
			inputNode = b.Input(mi.Type("number"), mi.Name(name), mi.ID(name), mi.Value(value), mi.Class(class), mi.Required())
		} else {
			inputNode = b.Input(mi.Type("number"), mi.Name(name), mi.ID(name), mi.Value(value), mi.Class(class))
		}
	case "boolean":
		checked := value == "true"
		if checked {
			inputNode = b.Input(mi.Type("checkbox"), mi.Name(name), mi.ID(name), mi.Value("true"), mi.Checked(), mi.Class("h-4 w-4 text-indigo-600"))
		} else {
			inputNode = b.Input(mi.Type("checkbox"), mi.Name(name), mi.ID(name), mi.Value("true"), mi.Class("h-4 w-4 text-indigo-600"))
		}
	default:
		if f.Required {
			inputNode = b.Input(mi.Type("text"), mi.Name(name), mi.ID(name), mi.Value(value), mi.Class(class), mi.Required())
		} else {
			inputNode = b.Input(mi.Type("text"), mi.Name(name), mi.ID(name), mi.Value(value), mi.Class(class))
		}
	}
	return b.Div(labelNode, inputNode)
}

func conditionalButtonText(isEdit bool, editText, createText string) string {
	if isEdit {
		return editText
	}
	return createText
}
