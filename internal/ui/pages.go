package ui

import (
	"encoding/json"
	"fmt"
	"html"

	mi "github.com/ha1tch/minty"
)

// =============================================================================
// SENSORS PAGE
// =============================================================================

// SensorsListData holds data for the sensors list page.
type SensorsListData struct {
	Sensors      []SensorRow
	Total        int
	Page         int
	PageSize     int
	TypeFilter   string
	StatusFilter string
	SensorTypes  []string
}

// SensorRow represents a sensor in the list.
type SensorRow struct {
	ID         int64
	ExternalID string
	Name       string
	Type       string
	Protocol   string
	Status     string
	LastSeen   string
}

// SensorsListPage renders the sensors list page.
func SensorsListPage(data SensorsListData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	typeOpts := []SelectOption{{Value: "", Label: t("sensors.filter.all_types")}}
	for _, ty := range data.SensorTypes {
		typeOpts = append(typeOpts, SelectOption{Value: ty, Label: ty, Selected: data.TypeFilter == ty})
	}
	statusOpts := []SelectOption{
		{Value: "", Label: t("sensors.filter.all_statuses")},
		{Value: "active", Label: t("status.active"), Selected: data.StatusFilter == "active"},
		{Value: "inactive", Label: t("status.inactive"), Selected: data.StatusFilter == "inactive"},
		{Value: "pending", Label: t("status.pending"), Selected: data.StatusFilter == "pending"},
	}
	rows := make([]mi.H, len(data.Sensors))
	for i, s := range data.Sensors {
		rows[i] = ListRow(i,
			ListCellLink(s.Name, fmt.Sprintf("/sensors/%d", s.ID)),
			ListCellMuted(s.ExternalID),
			ListCellMuted(s.Type),
			ListCellMuted(s.Protocol),
			ListCellRich(func(b *mi.Builder) mi.Node {
				v := map[string]string{"active": "success", "inactive": "default", "pending": "warning"}[s.Status]
				if v == "" {
					v = "default"
				}
				return Badge(s.Status, v)(b)
			}),
			ListCellMuted(s.LastSeen),
			ListActions(ListActionsConfig{
				T: t,
			}),
		)
	}
	return ListPage(
		ListPageConfig{
			Title:              t("sensors.list_title"),
			Subtitle:           t("sensors.subtitle", data.Total),
			CreateURL:          "/sensors/new",
			CreateLabel:        t("sensors.new"),
			CreateTargetModal2: true, ModalWidth: "660px",
		},
		ListFilterBar(ListFilterConfig{
			Action: "/sensors",
			Filters: []ListFilterField{
				{Name: "type", Options: typeOpts},
				{Name: "status", Options: statusOpts},
			},
		}),
		ListTable(ListTableConfig{
			Columns: []ListColumn{
				{Label: t("field.name")}, {Label: t("sensors.column.external_id")}, {Label: t("field.type")},
				{Label: t("sensors.column.protocol")}, {Label: t("field.status")}, {Label: t("sensors.column.last_seen")},
				{Label: t("asset.list.actions"), Right: true},
			},
			EmptyIcon:    "sensors",
			EmptyTitle:   t("sensors.empty_title"),
			EmptyMessage: t("sensors.empty_message"),
		}, rows),
		nil,
	)
}

func selectOption(b *mi.Builder, value, label, selected string) mi.Node {
	if value == selected {
		return b.Option(mi.Value(value), mi.Selected(), label)
	}
	return b.Option(mi.Value(value), label)
}

// =============================================================================
// RULES PAGE
// =============================================================================

// RulesListData holds data for the rules list page.
type RulesListData struct {
	Rules []RuleRow
	Total int
}

// RuleRow represents a rule in the list.
type RuleRow struct {
	ID          int64
	Name        string
	Description string
	Enabled     bool
	Priority    int
	TriggerType string
	ActionType  string
}

// RulesListPage renders the rules list page.
func RulesListPage(data RulesListData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	rows := make([]mi.H, len(data.Rules))
	for i, r := range data.Rules {
		rows[i] = ListRow(i,
			ListCellRich(func(b *mi.Builder) mi.Node {
				return b.Div(mi.Class("w-7 h-7 rounded-full bg-gray-100 dark:bg-gray-700 flex items-center justify-center text-xs font-medium text-gray-600 dark:text-gray-300"),
					fmt.Sprintf("%d", r.Priority),
				)
			}),
			ListCellRich(func(b *mi.Builder) mi.Node {
				return b.Div(
					b.Div(mi.Class("font-medium text-gray-900 dark:text-white text-sm"), r.Name),
					b.Div(mi.Class("text-xs text-gray-500 dark:text-gray-400 mt-0.5"), r.Description),
				)
			}),
			ListCellMuted(r.TriggerType),
			ListCellMuted(r.ActionType),
			ListCellRich(func(b *mi.Builder) mi.Node {
				if r.Enabled {
					return Badge(t("rules.status.enabled"), "success")(b)
				}
				return Badge(t("rules.status.disabled"), "default")(b)
			}),
			ListActions(ListActionsConfig{
				EditURL:   fmt.Sprintf("/rules/%d", r.ID),
				DeleteURL: fmt.Sprintf("/rules/%d/delete", r.ID),
				T:         t,
			}),
		)
	}
	return ListPage(
		ListPageConfig{
			Title:       t("rules.list_title"),
			Subtitle:    t("rules.subtitle", data.Total),
			CreateURL:   "/rules/new",
			CreateLabel: t("rules.new"),
		},
		nil,
		ListTable(ListTableConfig{
			Columns: []ListColumn{
				{Label: "#"}, {Label: t("rules.column.rule")}, {Label: t("rules.column.trigger")},
				{Label: t("rules.column.action")}, {Label: t("field.status")}, {Label: t("asset.list.actions"), Right: true},
			},
			EmptyIcon:    "rule",
			EmptyTitle:   t("rules.empty_title"),
			EmptyMessage: t("rules.empty_message"),
		}, rows),
		nil,
	)
}

// =============================================================================
// RULE CREATE/EDIT FORM
// =============================================================================

// RuleFormData holds data for the rule create/edit form.
type RuleFormData struct {
	IsEdit         bool
	Rule           RuleDetail
	TriggerTypes   []string
	ActionTypes    []string
	AssetTypes     []AssetTypeOption
	ConditionTypes []string
	Errors         map[string]string
	CSRFToken      string
}

// RuleDetail represents rule details for the form.
type RuleDetail struct {
	ID            int64
	Name          string
	Description   string
	Enabled       bool
	Priority      int
	TriggerType   string
	ConditionType string
	Threshold     float64
	Duration      int
	ActionType    string
	ActionConfig  string
	AssetTypeID   int64
}

// RuleFormPage renders the rule create/edit form.
func RuleFormPage(data RuleFormData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	return func(b *mi.Builder) mi.Node {
		title := t("rules.create_title")
		if data.IsEdit {
			title = t("rules.edit_title")
		}

		return b.Div(mi.Class("space-y-6 max-w-2xl"),
			// Header
			b.Div(
				b.Nav(mi.Class("text-sm text-gray-500 dark:text-gray-400 mb-2"),
					b.A(mi.Href("/rules"), mi.Class("hover:text-gray-700 dark:hover:text-gray-300"), t("rules.list_title")),
					b.Span(mi.Class("mx-2"), "/"),
					b.Span(mi.Class("text-gray-900 dark:text-white"), title),
				),
				b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), title),
			),

			// Form
			b.Form(mi.Class("space-y-6"), mi.Method("post"),
				mi.Action(func() string {
					if data.IsEdit {
						return fmt.Sprintf("/rules/%d", data.Rule.ID)
					}
					return "/rules"
				}()),
				CSRFField(data.CSRFToken)(b),
				// Basic Information
				Card(t("rules.section.basic_info"), func(b *mi.Builder) mi.Node {
					return b.Div(mi.Class("space-y-4"),
						ruleFormField(b, t("field.name"), "name", "text", data.Rule.Name, true),
						ruleFormTextarea(b, t("field.description"), "description", data.Rule.Description),
						b.Div(mi.Class("grid grid-cols-2 gap-4"),
							ruleFormNumber(b, t("field.priority"), "priority", data.Rule.Priority, 1, 100),
							ruleFormToggle(b, t("rules.field.enabled_toggle"), "enabled", data.Rule.Enabled),
						),
					)
				})(b),

				// Trigger Configuration
				Card(t("rules.section.trigger"), func(b *mi.Builder) mi.Node {
					return b.Div(mi.Class("space-y-4"),
						ruleFormSelect(b, t("rules.field.trigger_type"), "trigger_type", data.TriggerTypes, data.Rule.TriggerType),
						ruleFormSelect(b, t("rules.field.asset_type"), "asset_type_id", assetTypeStrings(data.AssetTypes), ""),
					)
				})(b),

				// Condition Builder
				Card(t("rules.section.condition"), func(b *mi.Builder) mi.Node {
					return b.Div(mi.Class("space-y-4"),
						ruleFormSelect(b, t("rules.field.condition_type"), "condition_type", data.ConditionTypes, data.Rule.ConditionType),
						b.Div(mi.Class("grid grid-cols-2 gap-4"),
							ruleFormNumber(b, t("rules.field.threshold"), "threshold", int(data.Rule.Threshold), 0, 10000),
							ruleFormNumber(b, t("rules.field.duration"), "duration", data.Rule.Duration, 0, 86400),
						),
					)
				})(b),

				// Action Configuration
				Card(t("rules.section.action"), func(b *mi.Builder) mi.Node {
					return b.Div(mi.Class("space-y-4"),
						ruleFormSelect(b, t("rules.field.action_type"), "action_type", data.ActionTypes, data.Rule.ActionType),
						ruleFormTextarea(b, t("rules.field.action_config"), "action_config", data.Rule.ActionConfig),
					)
				})(b),

				// Actions
				b.Div(mi.Class("flex items-center justify-between pt-4"),
					ruleDeleteButton(b, data.IsEdit, data.Rule.ID, t),
					b.Div(mi.Class("flex gap-3"),
						b.A(mi.Href("/rules"), mi.Class("px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 font-medium"),
							t("action.cancel"),
						),
						b.Button(mi.Type("submit"), mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 font-medium"),
							conditionalButtonText(data.IsEdit, t("rules.save"), t("rules.create")),
						),
					),
				),
			),
		)
	}
}

func ruleFormField(b *mi.Builder, label, name, inputType, value string, required bool) mi.Node {
	inputClass := "w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"

	var inputNode mi.Node
	if required {
		inputNode = b.Input(mi.Type(inputType), mi.Name(name), mi.ID(name), mi.Value(value), mi.Class(inputClass), mi.Required())
	} else {
		inputNode = b.Input(mi.Type(inputType), mi.Name(name), mi.ID(name), mi.Value(value), mi.Class(inputClass))
	}

	return b.Div(
		b.Label(mi.For(name), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"), label),
		inputNode,
	)
}

func ruleFormTextarea(b *mi.Builder, label, name, value string) mi.Node {
	return b.Div(
		b.Label(mi.For(name), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"), label),
		b.Textarea(mi.Name(name), mi.ID(name), mi.Attr("rows", "3"),
			mi.Class("w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"),
			value,
		),
	)
}

func ruleFormNumber(b *mi.Builder, label, name string, value, min, max int) mi.Node {
	return b.Div(
		b.Label(mi.For(name), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"), label),
		b.Input(mi.Type("number"), mi.Name(name), mi.ID(name),
			mi.Value(fmt.Sprintf("%d", value)),
			mi.Attr("min", fmt.Sprintf("%d", min)),
			mi.Attr("max", fmt.Sprintf("%d", max)),
			mi.Class("w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"),
		),
	)
}

func ruleFormSelect(b *mi.Builder, label, name string, options []string, selected string) mi.Node {
	selectOpts := []interface{}{
		mi.Name(name), mi.ID(name),
		mi.Class("w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"),
	}
	for _, opt := range options {
		if opt == selected {
			selectOpts = append(selectOpts, b.Option(mi.Value(opt), mi.Selected(), opt))
		} else {
			selectOpts = append(selectOpts, b.Option(mi.Value(opt), opt))
		}
	}

	return b.Div(
		b.Label(mi.For(name), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"), label),
		b.Select(selectOpts...),
	)
}

func ruleFormToggle(b *mi.Builder, label, name string, checked bool) mi.Node {
	var checkbox mi.Node
	if checked {
		checkbox = b.Input(mi.Type("checkbox"), mi.Name(name), mi.ID(name), mi.Class("sr-only peer"), mi.Checked())
	} else {
		checkbox = b.Input(mi.Type("checkbox"), mi.Name(name), mi.ID(name), mi.Class("sr-only peer"))
	}

	return b.Div(
		b.Label(mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"), label),
		b.Label(mi.Class("relative inline-flex items-center cursor-pointer"),
			checkbox,
			b.Div(mi.Class("w-11 h-6 bg-gray-200 peer-focus:ring-2 peer-focus:ring-indigo-500 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-indigo-600")),
		),
	)
}

func assetTypeStrings(types []AssetTypeOption) []string {
	result := make([]string, len(types))
	for i, t := range types {
		result[i] = t.Name
	}
	return result
}

func ruleDeleteButton(b *mi.Builder, isEdit bool, ruleID int64, t func(key string, args ...any) string) mi.Node {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	if !isEdit {
		return b.Span()
	}
	return b.Button(
		mi.Type("button"),
		mi.Class("px-4 py-2 text-red-600 hover:text-red-700 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg font-medium"),
		mi.HxDelete(fmt.Sprintf("/api/v1/rules/%d", ruleID)),
		mi.HxConfirm(t("rules.confirm_delete")),
		t("rules.delete"),
	)
}

// =============================================================================
// ALERTS PAGE
// =============================================================================

// AlertsListData holds data for the alerts list page.
type AlertsListData struct {
	Alerts         []AlertRow
	Total          int
	OpenCount      int
	AckCount       int
	StatusFilter   string
	SeverityFilter string
	CSRFToken      string
}

// AlertRow represents an alert in the list.
type AlertRow struct {
	ID        int64
	Title     string
	Message   string
	Severity  string
	Status    string
	Source    string
	AssetID   int64
	CreatedAt string
}

// AlertsListPage renders the alerts list page.
func AlertsListPage(data AlertsListData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	const card = "bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700"
	rows := make([]mi.H, len(data.Alerts))
	for i, a := range data.Alerts {
		rows[i] = ListRow(i,
			// Alert column: icon + title + message (two-line rich cell)
			ListCellRich(func(b *mi.Builder) mi.Node {
				sevIcon, sevCls := "warning", "text-yellow-500"
				if a.Severity == "critical" {
					sevIcon, sevCls = "error", "text-red-500"
				} else if a.Severity == "low" {
					sevIcon, sevCls = "info", "text-blue-500"
				}
				v := map[string]string{"critical": "danger", "high": "danger", "medium": "warning", "low": "info"}[a.Severity]
				if v == "" {
					v = "default"
				}
				msgNode := b.Span()
				if a.Message != "" {
					msgNode = b.P(mi.Class("text-xs text-gray-500 dark:text-gray-400 mt-0.5"), a.Message)
				}
				return b.Div(mi.Class("flex items-start gap-3"),
					b.Span(mi.Class("material-icons text-xl "+sevCls+" mt-0.5 flex-shrink-0"), sevIcon),
					b.Div(
						b.Div(mi.Class("flex items-center gap-2"),
							b.Span(mi.Class("text-sm font-semibold text-gray-900 dark:text-white"), a.Title),
							Badge(a.Severity, v)(b),
						),
						msgNode,
					),
				)
			}),
			ListCellMuted(a.Source),
			ListCellMuted(a.CreatedAt),
			ListCellRight(func(b *mi.Builder) mi.Node {
				if a.Status == "resolved" {
					return b.Span(mi.Class("text-xs text-gray-400 dark:text-gray-500"), t("alerts.resolved"))
				}
				var btns []interface{}
				btns = append(btns, mi.Class("flex items-center justify-end gap-2"))
				if a.Status != "acknowledged" {
					btns = append(btns, b.Form(
						mi.Method("post"),
						mi.Attr("action", fmt.Sprintf("/alerts/%d/acknowledge", a.ID)),
						mi.Class("inline"),
						b.Input(mi.Type("hidden"), mi.Name("_csrf"), mi.Attr("value", data.CSRFToken)),
						b.Button(mi.Type("submit"),
							mi.Class("px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300"),
							t("alerts.acknowledge"),
						),
					))
				}
				btns = append(btns, b.Form(
					mi.Method("post"),
					mi.Attr("action", fmt.Sprintf("/alerts/%d/resolve", a.ID)),
					mi.Class("inline"),
					b.Input(mi.Type("hidden"), mi.Name("_csrf"), mi.Attr("value", data.CSRFToken)),
					b.Button(mi.Type("submit"),
						mi.Class("px-3 py-1.5 text-sm bg-green-600 text-white rounded hover:bg-green-700 font-medium"),
						t("alerts.resolve"),
					),
				))
				return b.Div(btns...)
			}),
		)
	}
	return func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("space-y-6"),
			// Header: title + open/acked stat badges
			b.Div(mi.Class("flex items-center justify-between"),
				b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), t("nav.alerts")),
				b.Div(mi.Class("flex items-center gap-4"),
					alertStatBadge(b, t("alerts.open"), data.OpenCount, "red"),
					alertStatBadge(b, t("alerts.acknowledged"), data.AckCount, "yellow"),
				),
			),
			// Card: status tabs + table (table omits its own card wrapper)
			b.Div(mi.Class(card),
				b.Div(mi.Class("border-b border-gray-200 dark:border-gray-700"),
					b.Nav(mi.Class("flex gap-4 px-4"), mi.Attr("aria-label", t("layout.tabs")),
						statusTab(b, t("alerts.open"), "open", data.StatusFilter),
						statusTab(b, t("alerts.acknowledged"), "acknowledged", data.StatusFilter),
						statusTab(b, t("alerts.resolved"), "resolved", data.StatusFilter),
						statusTab(b, t("users.tab.all"), "", data.StatusFilter),
					),
				),
				ListTable(ListTableConfig{
					Columns: []ListColumn{
						{Label: t("alerts.column.alert")}, {Label: t("alerts.column.source")}, {Label: t("alerts.column.severity")},
						{Label: t("sensors.column.time")}, {Label: t("asset.list.actions"), Right: true},
					},
					EmptyIcon:    "check_circle",
					EmptyTitle:   t("alerts.empty_title"),
					EmptyMessage: t("alerts.empty_message"),
					SkipCard:     true,
				}, rows)(b),
			),
		)
	}
}

func alertStatBadge(b *mi.Builder, label string, count int, color string) mi.Node {
	bgColor := "bg-" + color + "-100 dark:bg-" + color + "-900"
	textColor := "text-" + color + "-800 dark:text-" + color + "-300"
	return b.Div(mi.Class("flex items-center gap-2 px-3 py-1 rounded-full "+bgColor),
		b.Span(mi.Class("text-lg font-semibold "+textColor), fmt.Sprintf("%d", count)),
		b.Span(mi.Class("text-sm "+textColor), label),
	)
}

func statusTab(b *mi.Builder, label, value, current string) mi.Node {
	isActive := value == current
	baseClass := "py-3 px-1 border-b-2 text-sm font-medium"
	if isActive {
		baseClass += " border-indigo-500 text-indigo-600 dark:text-indigo-400"
	} else {
		baseClass += " border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-200"
	}
	return b.A(mi.Href("/alerts?status="+value), mi.Class(baseClass), label)
}

// =============================================================================
// SETTINGS PAGE
// =============================================================================

// SettingsData holds data for the settings page.
//
// GeneralFormContent/NotificationsFormContent/FeaturesFormContent are
// pre-rendered by the Settings handler (routes.go), matching
// ProfileData's own pattern exactly -- see its doc comment for why a
// real per-request Translator, not a shared package-level engine, is
// required for formengine's i18n to work at all.
type SettingsData struct {
	TenantName               string
	Locale                   string
	AvailableLanguages       []LanguageOption
	EmailAlertsEnabled       bool
	SlackEnabled             bool
	WebhooksEnabled          bool
	MapEnabled               bool   // EnableMap feature toggle
	SavedSection             string // "general", "notifications", or "features" if just saved
	CSRFToken                string
	GeneralFormContent       mi.H
	NotificationsFormContent mi.H
	FeaturesFormContent      mi.H
}

// LanguageOption represents a language option for the selector.
type LanguageOption struct {
	Code     string
	Name     string
	Selected bool
}

// SettingsPage renders the settings page.
func SettingsPage(data SettingsData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	return func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("space-y-6"),
			b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), t("nav.settings")),

			// Success message if just saved
			settingsSavedMessage(b, data.SavedSection, t),

			// General settings (MOD-8: formengine-rendered)
			Card(t("settings.section.general"), func(b *mi.Builder) mi.Node {
				return b.Form(mi.Class("space-y-4"), mi.Method("post"), mi.Action("/settings/general"),
					data.GeneralFormContent(b),
					b.Div(mi.Class("flex justify-end pt-4"),
						b.Button(mi.Type("submit"), mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 font-medium"),
							t("assettypes.save_changes"),
						),
					),
				)
			})(b),

			// Notification settings (MOD-8: formengine-rendered)
			Card(t("settings.section.notifications"), func(b *mi.Builder) mi.Node {
				return b.Form(mi.Class("space-y-4"), mi.Method("post"), mi.Action("/settings/notifications"),
					data.NotificationsFormContent(b),
					b.Div(mi.Class("flex justify-end pt-4"),
						b.Button(mi.Type("submit"), mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 font-medium"),
							t("assettypes.save_changes"),
						),
					),
				)
			})(b),

			// Features (MOD-8: formengine-rendered)
			Card(t("settings.section.features"), func(b *mi.Builder) mi.Node {
				return b.Form(mi.Class("space-y-4"), mi.Method("post"), mi.Action("/settings/features"),
					data.FeaturesFormContent(b),
					b.Div(mi.Class("flex justify-end pt-4"),
						b.Button(mi.Type("submit"), mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 font-medium"),
							t("assettypes.save_changes"),
						),
					),
				)
			})(b),

			// Appearance — client-side display preferences stored in localStorage
			Card(t("settings.section.appearance"), func(b *mi.Builder) mi.Node {
				return b.Div(mi.Class("space-y-4"),
					// Transition Duration
					b.Div(mi.Class("flex items-start justify-between gap-4"),
						b.Div(mi.Class("flex-1 min-w-0"),
							b.Div(mi.Class("text-sm font-medium text-gray-900 dark:text-white"), t("settings.field.transition_duration")),
							b.Div(mi.Class("text-xs text-gray-500 dark:text-gray-400 mt-0.5"),
								t("settings.field.transition_duration_hint")),
						),
						b.Select(
							mi.Class("seam-transition-select text-sm border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-1.5 bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-indigo-500 focus:outline-none"),
							mi.Attr("onchange", "SeamUI.setTransition(this.value)"),
							b.Option(mi.Value("0ms"), t("settings.transition.off")),
							b.Option(mi.Value("200ms"), t("settings.transition.fast")),
							b.Option(mi.Value("500ms"), mi.Attr("selected", "selected"), t("settings.transition.normal")),
							b.Option(mi.Value("800ms"), t("settings.transition.slow")),
						),
					),
					// Small script to pre-select the saved value on load
					b.Script(mi.Raw(`(function(){
						var d=localStorage.getItem('seam-transition');
						if(d)document.querySelectorAll('.seam-transition-select').forEach(function(el){el.value=d;});
					})()`)),
				)
			})(b),

			// Data Management
			Card(t("settings.section.data_management"), func(b *mi.Builder) mi.Node {
				return b.Div(mi.Class("space-y-4"),
					b.Div(mi.Class("flex items-center justify-between p-4 border border-gray-200 dark:border-gray-700 rounded-lg"),
						b.Div(
							b.H4(mi.Class("font-medium text-gray-900 dark:text-white"), t("settings.export_data")),
							b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400"), t("settings.export_data_hint")),
						),
						b.A(
							mi.Href("/api/v1/export"),
							mi.Class("px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg text-sm font-medium hover:bg-gray-50 dark:hover:bg-gray-700"),
							mi.Attr("download", ""),
							t("settings.download_backup"),
						),
					),
				)
			})(b),

			// Danger Zone
			Card(t("settings.section.danger_zone"), func(b *mi.Builder) mi.Node {
				return b.Div(mi.Class("p-4 border border-red-200 dark:border-red-800 rounded-lg bg-red-50 dark:bg-red-900/20"),
					b.Div(mi.Class("flex items-center justify-between"),
						b.Div(
							b.H4(mi.Class("font-medium text-red-800 dark:text-red-200"), t("settings.delete_all_data")),
							b.P(mi.Class("text-sm text-red-600 dark:text-red-400"), t("settings.delete_all_data_hint")),
						),
						b.Button(
							mi.Type("button"),
							mi.Class("px-4 py-2 bg-red-600 text-white rounded-lg text-sm font-medium hover:bg-red-700"),
							mi.HxDelete("/api/v1/tenant/data"),
							mi.HxConfirm(t("settings.confirm_delete_all")),
							t("settings.delete_all"),
						),
					),
				)
			})(b),
		)
	}
}

func settingsSavedMessage(b *mi.Builder, section string, t func(key string, args ...any) string) mi.Node {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	if section == "" {
		return b.Span()
	}
	var msg string
	switch section {
	case "general":
		msg = t("settings.saved.general")
	case "notifications":
		msg = t("settings.saved.notifications")
	case "features":
		msg = t("settings.saved.features")
	default:
		msg = t("settings.saved.default")
	}
	return b.Div(mi.Class("p-4 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg"),
		b.Div(mi.Class("flex items-center gap-2"),
			b.Span(mi.Class("material-icons text-green-600 dark:text-green-400"), "check_circle"),
			b.P(mi.Class("text-sm text-green-700 dark:text-green-300"), msg),
		),
	)
}

func languageSelect(b *mi.Builder, label, name string, options []LanguageOption) mi.Node {
	selectOpts := []interface{}{
		mi.Name(name),
		mi.ID(name),
		mi.Class("w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"),
	}
	for _, opt := range options {
		if opt.Selected {
			selectOpts = append(selectOpts, b.Option(mi.Value(opt.Code), mi.Selected(), opt.Name))
		} else {
			selectOpts = append(selectOpts, b.Option(mi.Value(opt.Code), opt.Name))
		}
	}

	return b.Div(
		b.Label(mi.For(name), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"), label),
		b.Select(selectOpts...),
	)
}

func formField(b *mi.Builder, label, name, inputType, value string) mi.Node {
	return b.Div(
		b.Label(mi.For(name), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"), label),
		b.Input(mi.Type(inputType), mi.Name(name), mi.ID(name), mi.Value(value),
			mi.Class("w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"),
		),
	)
}

func toggleField(b *mi.Builder, label, name string, enabled bool, description string) mi.Node {
	var checkbox mi.Node
	if enabled {
		checkbox = b.Input(mi.Type("checkbox"), mi.Name(name), mi.Class("sr-only peer"), mi.Checked())
	} else {
		checkbox = b.Input(mi.Type("checkbox"), mi.Name(name), mi.Class("sr-only peer"))
	}

	return b.Div(mi.Class("flex items-center justify-between py-3 border-b border-gray-100 dark:border-gray-700 last:border-0"),
		b.Div(
			b.Span(mi.Class("text-sm font-medium text-gray-900 dark:text-white"), label),
			b.P(mi.Class("text-xs text-gray-500 dark:text-gray-400"), description),
		),
		b.Label(mi.Class("relative inline-flex items-center cursor-pointer"),
			checkbox,
			b.Div(mi.Class("w-11 h-6 bg-gray-200 peer-focus:ring-2 peer-focus:ring-indigo-500 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-indigo-600")),
		),
	)
}

// =============================================================================
// QR SCAN PAGES
// =============================================================================

// QRScanData holds data for the QR scan landing page.
type QRScanData struct {
	Code           string
	AssetID        int64
	AssetName      string
	AssetType      string
	Status         string
	Location       string
	LastSeen       string
	AssignedTo     string
	RecentActivity []string
}

// QRScanPage renders the QR scan landing page.
func QRScanPage(data QRScanData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	return func(b *mi.Builder) mi.Node {
		statusColor := "bg-green-500"
		statusText := t("asset.status.active")
		switch data.Status {
		case "alert":
			statusColor = "bg-red-500"
			statusText = t("asset.status.alert")
		case "silent":
			statusColor = "bg-yellow-500"
			statusText = t("asset.status.silent")
		case "lost":
			statusColor = "bg-gray-500"
			statusText = t("asset.status.lost")
		case "maintenance":
			statusColor = "bg-blue-500"
			statusText = t("dashboard.status.maintenance")
		default:
			statusText = data.Status
		}

		return b.Div(mi.Class("space-y-6"),
			// Asset card
			b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-xl shadow-lg overflow-hidden"),
				// Status bar
				b.Div(mi.Class("h-2 "+statusColor)),

				b.Div(mi.Class("p-6"),
					// Header
					b.Div(mi.Class("flex items-start justify-between mb-4"),
						b.Div(
							b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), data.AssetName),
							b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400"), data.AssetType),
						),
						b.Div(mi.Class("px-3 py-1 rounded-full text-sm font-medium text-white "+statusColor), statusText),
					),

					// Details grid
					b.Div(mi.Class("grid grid-cols-2 gap-4 mb-6"),
						qrDetailItem(b, t("qr.field.location"), data.Location, "location_on"),
						qrDetailItem(b, t("sensors.column.last_seen"), data.LastSeen, "schedule"),
						qrDetailItem(b, t("qr.field.assigned_to"), data.AssignedTo, "person"),
						qrDetailItem(b, t("qr.field.code"), data.Code, "qr_code_2"),
					),

					// Recent activity
					b.Div(mi.Class("border-t border-gray-200 dark:border-gray-700 pt-4"),
						b.H3(mi.Class("text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"), t("qr.recent_activity")),
						qrActivityList(b, data.RecentActivity, t),
					),
				),
			),

			// Action buttons
			b.Div(mi.Class("grid grid-cols-2 gap-3"),
				b.Button(mi.Class("flex items-center justify-center gap-2 px-4 py-3 bg-green-600 text-white rounded-lg font-medium hover:bg-green-700"),
					b.Span(mi.Class("material-icons"), "check_circle"),
					t("qr.check_in"),
				),
				b.Button(mi.Class("flex items-center justify-center gap-2 px-4 py-3 bg-yellow-600 text-white rounded-lg font-medium hover:bg-yellow-700"),
					b.Span(mi.Class("material-icons"), "report_problem"),
					t("qr.report_issue"),
				),
			),

			// Footer link
			b.Div(mi.Class("text-center"),
				b.A(mi.Href(fmt.Sprintf("/assets/%d", data.AssetID)), mi.Class("text-sm text-indigo-600 dark:text-indigo-400 hover:underline"),
					t("qr.view_full_details"),
				),
			),
		)
	}
}

func qrDetailItem(b *mi.Builder, label, value, icon string) mi.Node {
	if value == "" {
		value = "—"
	}
	return b.Div(mi.Class("flex items-start gap-2"),
		b.Span(mi.Class("material-icons text-gray-400 text-lg"), icon),
		b.Div(
			b.Div(mi.Class("text-xs text-gray-500 dark:text-gray-400"), label),
			b.Div(mi.Class("text-sm font-medium text-gray-900 dark:text-white"), value),
		),
	)
}

func qrActivityList(b *mi.Builder, activities []string, t func(key string, args ...any) string) mi.Node {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	if len(activities) == 0 {
		return b.P(mi.Class("text-sm text-gray-400"), t("qr.no_recent_activity"))
	}

	items := make([]interface{}, len(activities))
	for i, a := range activities {
		items[i] = b.Li(mi.Class("text-sm text-gray-600 dark:text-gray-400"), a)
	}
	return b.Ul(append([]interface{}{mi.Class("space-y-1")}, items...)...)
}

// QRUnassignedPage renders a page for unassigned QR codes.
func QRUnassignedPage(code string, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	return func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("text-center py-8"),
			b.Div(mi.Class("text-6xl mb-4"), "📦"),
			b.H1(mi.Class("text-xl font-bold text-gray-900 dark:text-white mb-2"), t("qr.unassigned_title")),
			b.P(mi.Class("text-gray-500 dark:text-gray-400 mb-4"),
				t("qr.unassigned_message", code),
			),
			b.P(mi.Class("text-sm text-gray-400"),
				t("qr.contact_admin"),
			),
		)
	}
}

// =============================================================================
// MAP VIEW PAGE
// =============================================================================

// MapViewData holds data for the map page.
type MapViewData struct {
	Markers      []MapMarker
	StatusFilter string
	TypeFilter   string
	AssetTypes   []string
}

// MapMarker represents an asset marker on the map.
//
// JSON tags match the field names the client-side JS in MapViewPage
// already expects (m.name/m.status/m.type/m.lat/m.lng/m.id) -- this
// struct is now marshaled directly via encoding/json (see
// MapViewPage's own comment on why), not hand-assembled field by
// field, so the tags are load-bearing, not decoration.
type MapMarker struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Status   string  `json:"status"`
	TypeName string  `json:"type"`
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
}

// MapViewPage renders the map view page.
//
// Two real, separate vulnerabilities were found and fixed here
// 2026-08-04, via adversarial testing -- both via the same root cause
// (asset name/status/type are genuinely user-editable, via the asset
// create/edit forms, and every asset on the map renders one):
//
//  1. markersJSON used to be hand-assembled via fmt.Sprintf(`"name":
//     "%s"`, m.Name) -- no escaping at all. A double quote in an
//     asset's name broke the JSON's own syntax; a "</script>" sequence
//     broke out of the <script> tag the JSON was embedded in entirely.
//     Fixed by marshaling data.Markers via encoding/json instead --
//     Go's json package HTML-escapes <, >, and & by default
//     (json.Marshal's own documented SetEscapeHTML(true) default),
//     specifically for safe embedding in an HTML <script> tag.
//
//  2. Even with (1) fixed, the client-side JS still built each
//     marker's popup via raw string concatenation --
//     marker.bindPopup('<strong>' + m.name + '</strong>...') -- and
//     passed the result to Leaflet's bindPopup(), which renders its
//     argument as HTML. JSON.parse reverses json.Marshal's HTML-
//     escaping (client-side, \u003c becomes a literal "<" again once
//     parsed back into a JS string), so (1) alone does not protect
//     this second, independent step: an asset named
//     <img src=x onerror=alert(1)> would render as a live, executing
//     image tag in the popup, in the browser of every user who opens
//     the map. Fixed by building the popup via DOM APIs (createElement
//     + textContent) instead of an HTML string -- Leaflet's
//     bindPopup() accepts an HTMLElement directly, so the dynamic
//     parts (name, type) never pass through any HTML string at all,
//     escaped or not. The one static structural element (the "view
//     details" link) is built the same way, with the numeric asset ID
//     (never user-controlled) as its only dynamic part.
func MapViewPage(data MapViewData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	return func(b *mi.Builder) mi.Node {
		// json.Marshal(nil slice) produces the literal "null", not "[]"
		// -- markers.forEach would throw client-side on a null value, so
		// the nil/empty case needs its own explicit path, not just a
		// marshal-error fallback (a marshal error is vanishingly
		// unlikely for this struct anyway; the real everyday case is
		// "no assets have coordinates yet").
		markersJSON := "[]"
		if len(data.Markers) > 0 {
			if b, err := json.Marshal(data.Markers); err == nil {
				markersJSON = string(b)
			}
		}

		return b.Div(mi.Class("space-y-4"),
			// Header with filters
			b.Div(mi.Class("flex items-center justify-between"),
				b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), t("map.title")),
				b.Div(mi.Class("flex items-center gap-2"),
					b.Span(mi.Class("text-sm text-gray-500 dark:text-gray-400"),
						t("map.assets_shown", len(data.Markers)),
					),
				),
			),

			// Filter bar
			b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-4"),
				b.Form(mi.Class("flex flex-wrap gap-4"), mi.Method("get"),
					b.Div(mi.Class("flex-1 min-w-[150px]"),
						b.Label(mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"), t("field.status")),
						b.Select(mi.Name("status"), mi.Class("w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-sm"),
							b.Option(mi.Value(""), t("sensors.filter.all_statuses")),
							selectOption(b, "active", t("asset.status.active"), data.StatusFilter),
							selectOption(b, "alert", t("asset.status.alert"), data.StatusFilter),
							selectOption(b, "silent", t("asset.status.silent"), data.StatusFilter),
							selectOption(b, "lost", t("asset.status.lost"), data.StatusFilter),
						),
					),
					b.Div(mi.Class("flex-1 min-w-[150px]"),
						b.Label(mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"), t("field.type")),
						mapTypeSelect(b, data.AssetTypes, data.TypeFilter, t),
					),
					b.Div(mi.Class("flex items-end"),
						b.Button(mi.Type("submit"), mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 text-sm font-medium"),
							t("map.apply"),
						),
					),
				),
			),

			// Map container
			b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 overflow-hidden"),
				b.Div(mi.ID("map"), mi.Class("h-[600px] w-full")),
			),

			// Leaflet — vendored
			b.Link(mi.Rel("stylesheet"), mi.Href("/static/vendor/leaflet@1.9.4.css")),
			b.Script(mi.Src("/static/vendor/leaflet@1.9.4.js")),
			b.Script(mi.Raw(fmt.Sprintf(`
				document.addEventListener('DOMContentLoaded', function() {
					var markers = %s;
					var map = L.map('map').setView([0, 0], 2);
					
					L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
						attribution: '© OpenStreetMap contributors'
					}).addTo(map);
					
					var statusColors = {
						'active': '#22c55e',
						'alert': '#ef4444',
						'silent': '#eab308',
						'lost': '#6b7280',
						'maintenance': '#3b82f6'
					};
					
					var bounds = [];
					markers.forEach(function(m) {
						var color = statusColors[m.status] || '#6b7280';
						var marker = L.circleMarker([m.lat, m.lng], {
							radius: 8,
							fillColor: color,
							color: '#fff',
							weight: 2,
							opacity: 1,
							fillOpacity: 0.8
						}).addTo(map);
						
						// DOM-built, not an HTML string: m.name/m.type are
						// genuinely user-editable (an asset's own name),
						// and bindPopup() renders a string argument as raw
						// HTML -- textContent here means neither field is
						// ever interpreted as markup, no matter what it
						// contains.
						var popup = document.createElement('div');
						var strong = document.createElement('strong');
						strong.textContent = m.name;
						popup.appendChild(strong);
						popup.appendChild(document.createElement('br'));
						popup.appendChild(document.createTextNode(m.type));
						popup.appendChild(document.createElement('br'));
						var link = document.createElement('a');
						link.href = '/assets/' + encodeURIComponent(m.id);
						link.textContent = %q;
						popup.appendChild(link);
						marker.bindPopup(popup);
						bounds.push([m.lat, m.lng]);
					});
					
					if (bounds.length > 0) {
						map.fitBounds(bounds, { padding: [20, 20] });
					}
				});
			`, markersJSON, t("map.view_details")))),
		)
	}
}

func mapTypeSelect(b *mi.Builder, types []string, selected string, t func(key string, args ...any) string) mi.Node {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	options := []interface{}{
		mi.Name("type"),
		mi.Class("w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-sm"),
		b.Option(mi.Value(""), t("sensors.filter.all_types")),
	}
	for _, ty := range types {
		options = append(options, selectOption(b, ty, ty, selected))
	}
	return b.Select(options...)
}

// =============================================================================
// ASSET TYPES PAGE
// =============================================================================

// AssetTypesListData holds data for the asset types page.
type AssetTypesListData struct {
	Types []AssetTypeRow
	Total int
}

// AssetTypeRow represents an asset type in the list.
type AssetTypeRow struct {
	ID          int64
	Name        string
	Description string
	FieldCount  int
	AssetCount  int
}

// AssetTypesListPage renders the asset types management page.
func AssetTypesListPage(data AssetTypesListData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	return func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("space-y-6"),
			// Header
			b.Div(mi.Class("flex items-center justify-between"),
				b.Div(
					b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), t("assettypes.list_title")),
					b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400"), t("assettypes.subtitle")),
				),
				b.A(mi.Href("/asset-types/new"), mi.Class("inline-flex items-center gap-2 px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 text-sm font-medium"),
					b.Span(mi.Class("material-icons text-lg"), "add"),
					t("assettypes.new"),
				),
			),

			// Types grid
			assetTypesGrid(b, data.Types, t),
		)
	}
}

func assetTypesGrid(b *mi.Builder, types []AssetTypeRow, t func(key string, args ...any) string) mi.Node {
	if len(types) == 0 {
		return Card("", EmptyState("category", t("assettypes.empty_title"), t("assettypes.empty_message"), nil))(b)
	}

	cards := make([]interface{}, len(types))
	for i, row := range types {
		cards[i] = assetTypeCard(b, row, t)
	}

	return b.Div(append([]interface{}{mi.Class("grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4")}, cards...)...)
}

func assetTypeCard(b *mi.Builder, row AssetTypeRow, t func(key string, args ...any) string) mi.Node {
	return b.A(mi.Href(fmt.Sprintf("/asset-types/%d", row.ID)),
		mi.Class("block bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-5 hover:shadow-md hover:border-indigo-300 dark:hover:border-indigo-600 transition-all"),
		b.Div(mi.Class("flex items-start justify-between mb-3"),
			b.Div(mi.Class("w-10 h-10 rounded-lg bg-indigo-100 dark:bg-indigo-900 flex items-center justify-center"),
				b.Span(mi.Class("material-icons text-indigo-600 dark:text-indigo-400"), "category"),
			),
			b.Span(mi.Class("material-icons text-gray-400"), "chevron_right"),
		),
		b.H3(mi.Class("font-semibold text-gray-900 dark:text-white mb-1"), row.Name),
		b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400 mb-3 line-clamp-2"), row.Description),
		b.Div(mi.Class("flex items-center gap-4 text-xs text-gray-500 dark:text-gray-400"),
			b.Span(t("assettypes.field_count", row.FieldCount)),
			b.Span("•"),
			b.Span(t("assettypes.asset_count", row.AssetCount)),
		),
	)
}

// sensorFormPartial renders the sensor form as a SeamModal2 partial.
func sensorFormPartial(b *mi.Builder, data SensorFormData, t func(key string, args ...any) string) mi.Node {
	inner := []interface{}{
		CSRFField(data.CSRFToken)(b),
		sensorFormField(b, t("sensors.field.external_id"), "external_id", data.ExternalID, data.Errors["external_id"], true),
		sensorFormField(b, t("sensors.field.type"), "type", data.Type, data.Errors["type"], true),
		sensorFormField(b, t("sensors.field.driver"), "driver", data.Driver, data.Errors["driver"], true),
		sensorFormField(b, t("sensors.field.name_optional"), "name", data.Name, "", false),
	}
	return masterPartialFormWrapper(b, "sensor-form", "/sensors", inner, t)
}

// =============================================================================
// SENSOR DETAIL PAGE
// =============================================================================

// SensorDetailData holds data for the sensor detail page.
type SensorDetailData struct {
	Sensor         SensorInfo
	BoundAssetID   int64
	BoundAssetName string
	RecentEvents   []EventSummary
	TotalEvents    int
	CSRFToken      string
}

// SensorInfo represents detailed sensor information.
type SensorInfo struct {
	ID         int64
	ExternalID string
	Name       string
	Type       string
	Protocol   string
	Status     string
	LastSeen   string
	CreatedAt  string
}

// SensorFormData holds data for the sensor create form.
type SensorFormData struct {
	ExternalID string
	Type       string
	Driver     string
	Name       string
	Errors     map[string]string
	CSRFToken  string
	IsPartial  bool
}

// SensorFormPage renders the sensor create form.
func SensorFormPage(data SensorFormData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	return func(b *mi.Builder) mi.Node {
		if data.IsPartial {
			return sensorFormPartial(b, data, t)
		}
		return b.Div(mi.Class("space-y-6 max-w-2xl"),
			b.Div(mi.Class("flex items-center gap-4"),
				b.A(mi.Href("/sensors"), mi.Class("text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"),
					b.Span(mi.Class("material-icons"), "arrow_back"),
				),
				b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), t("sensors.new")),
			),
			Card("", func(b *mi.Builder) mi.Node {
				return b.Form(mi.Class("space-y-4"), mi.Method("post"), mi.Action("/sensors"),
					CSRFField(data.CSRFToken)(b),
					sensorFormField(b, t("sensors.field.external_id"), "external_id", data.ExternalID, data.Errors["external_id"], true),
					sensorFormField(b, t("sensors.field.type"), "type", data.Type, data.Errors["type"], true),
					sensorFormField(b, t("sensors.field.driver"), "driver", data.Driver, data.Errors["driver"], true),
					sensorFormField(b, t("sensors.field.name_optional"), "name", data.Name, "", false),
					b.Div(mi.Class("flex justify-end gap-3 pt-4"),
						b.A(mi.Href("/sensors"), mi.Class("px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg font-medium hover:bg-gray-50 dark:hover:bg-gray-700"),
							t("action.cancel"),
						),
						b.Button(mi.Type("submit"), mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 font-medium"),
							t("sensors.new"),
						),
					),
				)
			})(b),
		)
	}
}

func sensorFormField(b *mi.Builder, label, name, value, errMsg string, required bool) mi.Node {
	inputClass := "w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
	if errMsg != "" {
		inputClass = "w-full px-3 py-2 border border-red-500 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
	}
	labelNode := b.Label(mi.For(name), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"), label)
	if required {
		labelNode = b.Label(mi.For(name), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"),
			label, b.Span(mi.Class("text-red-500 ml-1"), "*"),
		)
	}
	errNode := b.Span()
	if errMsg != "" {
		errNode = b.P(mi.Class("mt-1 text-sm text-red-500"), errMsg)
	}
	var inputNode mi.Node
	if required {
		inputNode = b.Input(mi.Type("text"), mi.Name(name), mi.ID(name), mi.Value(value), mi.Class(inputClass), mi.Required())
	} else {
		inputNode = b.Input(mi.Type("text"), mi.Name(name), mi.ID(name), mi.Value(value), mi.Class(inputClass))
	}
	return b.Div(labelNode, inputNode, errNode)
}

// SensorDetailPage renders the sensor detail page.
func SensorDetailPage(data SensorDetailData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	return func(b *mi.Builder) mi.Node {
		statusColor := "bg-green-500"
		if data.Sensor.Status == "inactive" {
			statusColor = "bg-gray-400"
		} else if data.Sensor.Status == "pending" {
			statusColor = "bg-yellow-500"
		}

		typeIcon := "sensors"
		switch data.Sensor.Type {
		case "rfid":
			typeIcon = "nfc"
		case "ble":
			typeIcon = "bluetooth"
		case "gps":
			typeIcon = "gps_fixed"
		case "camera":
			typeIcon = "videocam"
		}

		return b.Div(mi.Class("space-y-6"),
			// Breadcrumb
			b.Div(mi.Class("flex items-center gap-2 text-sm text-gray-500"),
				b.A(mi.Href("/sensors"), mi.Class("hover:text-gray-700 dark:hover:text-gray-200"), t("sensors.list_title")),
				b.Span("›"),
				b.Span(mi.Class("text-gray-900 dark:text-white"), data.Sensor.Name),
			),

			// Header card
			b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6"),
				b.Div(mi.Class("flex items-start justify-between"),
					b.Div(mi.Class("flex items-center gap-4"),
						b.Div(mi.Class("w-14 h-14 rounded-xl bg-indigo-100 dark:bg-indigo-900 flex items-center justify-center"),
							b.Span(mi.Class("material-icons text-2xl text-indigo-600 dark:text-indigo-400"), typeIcon),
						),
						b.Div(
							b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), data.Sensor.Name),
							b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400"), data.Sensor.ExternalID),
						),
					),
					b.Div(mi.Class("flex items-center gap-3"),
						b.Div(mi.Class("flex items-center gap-2"),
							b.Div(mi.Class("w-3 h-3 rounded-full "+statusColor)),
							b.Span(mi.Class("text-sm font-medium text-gray-700 dark:text-gray-300"), data.Sensor.Status),
						),
						b.Form(mi.Method("post"),
							mi.Attr("action", fmt.Sprintf("/sensors/%d/delete", data.Sensor.ID)),
							b.Input(mi.Type("hidden"), mi.Name("_csrf"), mi.Attr("value", data.CSRFToken)),
							b.Button(mi.Type("submit"),
								mi.Class("inline-flex items-center gap-1 px-3 py-1.5 text-sm border border-red-300 text-red-600 rounded-lg hover:bg-red-50 dark:hover:bg-red-900/20 font-medium"),
								mi.Attr("onclick", fmt.Sprintf(`event.preventDefault();SeamConfirm.showForm(this.closest('form'),'%s',{dangerName:'%s'})`, t("sensors.confirm_delete"), data.Sensor.Name)),
								b.Span(mi.Class("material-icons text-base"), "delete"),
								t("sensors.detail.delete"),
							),
						),
					),
				),
			),

			// Stats row
			b.Div(mi.Class("grid grid-cols-1 md:grid-cols-4 gap-4"),
				sensorStatCard(b, t("field.type"), data.Sensor.Type, "category"),
				sensorStatCard(b, t("sensors.column.protocol"), data.Sensor.Protocol, "lan"),
				sensorStatCard(b, t("sensors.detail.total_events"), fmt.Sprintf("%d", data.TotalEvents), "show_chart"),
				sensorStatCard(b, t("sensors.column.last_seen"), data.Sensor.LastSeen, "schedule"),
			),

			// Bound asset
			boundAssetCard(b, data.Sensor.ID, data.BoundAssetID, data.BoundAssetName, data.CSRFToken, t),

			// Recent events
			Card(t("sensors.detail.recent_events"), sensorEventsTable(data.RecentEvents, t))(b),
		)
	}
}

func sensorStatCard(b *mi.Builder, label, value, icon string) mi.Node {
	return b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-4"),
		b.Div(mi.Class("flex items-center gap-3"),
			b.Span(mi.Class("material-icons text-gray-400"), icon),
			b.Div(
				b.Div(mi.Class("text-xs text-gray-500 dark:text-gray-400"), label),
				b.Div(mi.Class("font-medium text-gray-900 dark:text-white"), value),
			),
		),
	)
}

// SensorBindFormData holds data for the sensor-bind form (T-19,
// 2026-08-04) -- the first real consumer of the asset-picker widget
// beyond parent_id.
type SensorBindFormData struct {
	SensorID    int64
	SensorName  string
	CSRFToken   string
	FormContent mi.H
}

// SensorBindFormPage renders the sensor-bind form -- a single
// asset-picker field, matching WorkOrderFormPage's own established
// shape for a formengine-schema-driven form embedded in an otherwise
// hand-coded page.
func SensorBindFormPage(data SensorBindFormData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	return func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("space-y-6 max-w-2xl"),
			b.Div(mi.Class("flex items-center gap-4"),
				b.A(mi.Href(fmt.Sprintf("/sensors/%d", data.SensorID)), mi.Class("text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"),
					b.Span(mi.Class("material-icons"), "arrow_back"),
				),
				b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), t("sensors.bind.title")),
			),
			b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400 -mt-4"), data.SensorName),
			Card("", func(b *mi.Builder) mi.Node {
				return b.Form(mi.Class("space-y-4"), mi.Method("post"), mi.Action(fmt.Sprintf("/sensors/%d/bind", data.SensorID)),
					data.FormContent(b),
					b.Div(mi.Class("flex justify-end gap-3 pt-4"),
						b.A(mi.Href(fmt.Sprintf("/sensors/%d", data.SensorID)), mi.Class("px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg font-medium hover:bg-gray-50 dark:hover:bg-gray-700"),
							t("action.cancel"),
						),
						b.Button(mi.Type("submit"), mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 font-medium"),
							t("sensors.bind.action"),
						),
					),
				)
			})(b),
		)
	}
}

func boundAssetCard(b *mi.Builder, sensorID, assetID int64, assetName, csrfToken string, t func(key string, args ...any) string) mi.Node {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	if assetID == 0 {
		return b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6"),
			b.Div(mi.Class("flex items-center justify-between"),
				b.Div(mi.Class("flex items-center gap-3"),
					b.Span(mi.Class("material-icons text-gray-400"), "link_off"),
					b.Div(
						b.Div(mi.Class("font-medium text-gray-900 dark:text-white"), t("sensors.detail.not_bound")),
						b.Div(mi.Class("text-sm text-gray-500 dark:text-gray-400"), t("sensors.detail.not_bound_message")),
					),
				),
				b.A(mi.Href(fmt.Sprintf("/sensors/%d/bind", sensorID)),
					mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg text-sm font-medium hover:bg-indigo-700"),
					t("sensors.detail.bind"),
				),
			),
		)
	}

	return b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6"),
		b.Div(mi.Class("flex items-center justify-between"),
			b.Div(mi.Class("flex items-center gap-3"),
				b.Span(mi.Class("material-icons text-green-500"), "link"),
				b.Div(
					b.Div(mi.Class("text-sm text-gray-500 dark:text-gray-400"), t("sensors.detail.bound_to")),
					b.A(mi.Href(fmt.Sprintf("/assets/%d", assetID)), mi.Class("font-medium text-indigo-600 dark:text-indigo-400 hover:underline"), assetName),
				),
			),
			b.Form(mi.Method("post"), mi.Action(fmt.Sprintf("/sensors/%d/unbind", sensorID)),
				b.Input(mi.Type("hidden"), mi.Name("_csrf"), mi.Value(csrfToken)),
				b.Button(mi.Type("submit"), mi.Class("px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg text-sm font-medium hover:bg-gray-50 dark:hover:bg-gray-700"),
					t("sensors.detail.unbind"),
				),
			),
		),
	)
}

func sensorEventsTable(events []EventSummary, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	return func(b *mi.Builder) mi.Node {
		if len(events) == 0 {
			return b.Div(mi.Class("text-center py-8 text-gray-500 dark:text-gray-400"),
				b.Span(mi.Class("material-icons text-4xl text-gray-300 dark:text-gray-600 mb-2"), "timeline"),
				b.P(t("sensors.detail.no_events")),
			)
		}

		rows := make([]interface{}, len(events))
		for i, e := range events {
			rows[i] = b.Tr(mi.Class("hover:bg-gray-50 dark:hover:bg-gray-700"),
				b.Td(mi.Class("px-4 py-3 text-sm text-gray-900 dark:text-gray-100"), e.Timestamp),
				b.Td(mi.Class("px-4 py-3 text-sm text-gray-900 dark:text-gray-100"), e.TriggerType),
				b.Td(mi.Class("px-4 py-3 text-sm text-gray-500 dark:text-gray-400"), e.Value),
			)
		}

		return b.Table(mi.Class("w-full"),
			b.Thead(mi.Class("bg-gray-50 dark:bg-gray-900"),
				b.Tr(
					b.Th(mi.Class("px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase"), t("sensors.column.time")),
					b.Th(mi.Class("px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase"), t("field.type")),
					b.Th(mi.Class("px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase"), t("sensors.column.value")),
				),
			),
			b.Tbody(append([]interface{}{mi.Class("divide-y divide-gray-200 dark:divide-gray-700")}, rows...)...),
		)
	}
}

// =============================================================================
// ALERT DETAIL PAGE
// =============================================================================

// AlertDetailData holds data for the alert detail page.
type AlertDetailData struct {
	Alert            AlertInfo
	AssetName        string
	AssetGIAI        string
	AssetRegion      string
	AssetCustodian   string
	RuleName         string
	AlertConfig      string // description from alerts master
	AlertType        string
	AlertCriticality string
	History          []AlertHistoryEntry
	CSRFToken        string
}

// AlertInfo represents detailed alert information.
type AlertInfo struct {
	ID        int64
	Title     string
	Message   string
	Severity  string
	Status    string
	Source    string
	AssetID   int64
	RuleID    int64
	AlertaID  int64
	CreatedAt string
	UpdatedAt string
}

// AlertHistoryEntry represents an entry in the alert history.
type AlertHistoryEntry struct {
	Action    string
	User      string
	Timestamp string
	Notes     string
}

// AlertDetailPage renders the alert detail page.
func AlertDetailPage(data AlertDetailData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	return func(b *mi.Builder) mi.Node {
		severityColor := "bg-yellow-500"
		if data.Alert.Severity == "critical" {
			severityColor = "bg-red-500"
		} else if data.Alert.Severity == "low" {
			severityColor = "bg-blue-500"
		}

		statusBadge := Badge(data.Alert.Status, data.Alert.Status)

		return b.Div(mi.Class("space-y-6"),
			// Breadcrumb
			b.Div(mi.Class("flex items-center gap-2 text-sm text-gray-500"),
				b.A(mi.Href("/alerts"), mi.Class("hover:text-gray-700 dark:hover:text-gray-200"), t("nav.alerts")),
				b.Span("›"),
				b.Span(mi.Class("text-gray-900 dark:text-white"), data.Alert.Title),
			),

			// Main alert card
			b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 overflow-hidden"),
				// Severity bar
				b.Div(mi.Class("h-2 "+severityColor)),

				b.Div(mi.Class("p-6"),
					// Header
					b.Div(mi.Class("flex items-start justify-between mb-4"),
						b.Div(
							b.Div(mi.Class("flex items-center gap-2 mb-2"),
								b.H1(mi.Class("text-xl font-bold text-gray-900 dark:text-white"), data.Alert.Title),
								Badge(data.Alert.Severity, data.Alert.Severity)(b),
							),
							b.P(mi.Class("text-gray-600 dark:text-gray-400"), data.Alert.Message),
						),
						statusBadge(b),
					),

					// Info grid
					b.Div(mi.Class("grid grid-cols-2 md:grid-cols-4 gap-4 py-4 border-t border-b border-gray-200 dark:border-gray-700"),
						alertInfoItem(b, t("alerts.column.source"), data.Alert.Source),
						alertInfoItem(b, t("alerts.field.created"), data.Alert.CreatedAt),
						alertInfoItem(b, t("alerts.field.updated"), data.Alert.UpdatedAt),
						alertInfoItem(b, t("field.status"), data.Alert.Status),
					),

					// Asset context (P-22)
					func() mi.Node {
						if data.Alert.AssetID == 0 {
							return b.Span()
						}
						items := []interface{}{mi.Class("mt-4 grid grid-cols-2 md:grid-cols-4 gap-4")}
						items = append(items, alertInfoItem(b, t("alerts.field.asset"), data.AssetName))
						if data.AssetGIAI != "" {
							items = append(items, alertInfoItem(b, "GIAI", data.AssetGIAI))
						}
						if data.AssetRegion != "" {
							items = append(items, alertInfoItem(b, t("masters.location.field.region"), data.AssetRegion))
						}
						if data.AssetCustodian != "" {
							items = append(items, alertInfoItem(b, t("alerts.field.custodian"), data.AssetCustodian))
						}
						return b.Div(items...)
					}(),

					// Alerta config (P-22)
					func() mi.Node {
						if data.AlertConfig == "" {
							return b.Span()
						}
						critClass := map[string]string{"Alta": "text-red-600 dark:text-red-400", "Media": "text-yellow-600 dark:text-yellow-400", "Baja": "text-green-600 dark:text-green-400"}[data.AlertCriticality]
						return b.Div(mi.Class("mt-3 p-3 bg-gray-50 dark:bg-gray-900/40 rounded-lg text-sm"),
							b.Span(mi.Class("font-medium text-gray-700 dark:text-gray-300"), t("alerts.config_label")),
							b.Span(mi.Class("text-gray-600 dark:text-gray-400"), data.AlertType+" — "+data.AlertConfig),
							b.Span(mi.Class("ml-2 font-medium "+critClass), criticalityLabel(t, data.AlertCriticality)),
						)
					}(),

					// Linked items
					b.Div(mi.Class("mt-4 flex flex-wrap gap-4"),
						alertLinkedItem(b, t("alerts.field.asset"), data.AssetName, fmt.Sprintf("/assets/%d", data.Alert.AssetID), data.Alert.AssetID > 0, t),
						alertLinkedItem(b, t("alerts.field.rule"), data.RuleName, fmt.Sprintf("/rules/%d", data.Alert.RuleID), data.Alert.RuleID > 0, t),
					),
				),

				// Acknowledge form (P-23)
				func() mi.Node {
					if data.Alert.Status != "open" {
						return b.Span()
					}
					return b.Div(mi.Class("px-6 py-4 bg-gray-50 dark:bg-gray-900/50 border-t border-gray-200 dark:border-gray-700"),
						b.Form(mi.Method("post"),
							mi.Attr("action", fmt.Sprintf("/alerts/%d/acknowledge", data.Alert.ID)),
							mi.Class("flex items-center gap-3"),
							b.Input(mi.Type("hidden"), mi.Name("_csrf"), mi.Attr("value", data.CSRFToken)),
							b.Input(mi.Type("text"), mi.Name("notes"),
								mi.Attr("placeholder", t("alerts.notes_placeholder")),
								mi.Class("flex-1 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-sm"),
							),
							b.Button(mi.Type("submit"),
								mi.Class("px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 text-sm font-medium whitespace-nowrap"),
								t("alerts.acknowledge_btn"),
							),
						),
					)
				}(),

				// Actions bar
				b.Div(mi.Class("px-6 py-4 bg-gray-50 dark:bg-gray-900/50 border-t border-gray-200 dark:border-gray-700 flex items-center gap-3"),
					b.A(mi.Href("/alerts"),
						mi.Class("inline-flex items-center gap-2 px-4 py-2 text-sm text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg"),
						b.Span(mi.Class("material-icons text-lg"), "arrow_back"),
						t("alerts.back_to_alerts"),
					),
				),
			),

			// History timeline
			Card(t("alerts.history_title"), alertHistoryTimeline(data.History, t))(b),
		)
	}
}

func alertInfoItem(b *mi.Builder, label, value string) mi.Node {
	return b.Div(
		b.Div(mi.Class("text-xs text-gray-500 dark:text-gray-400 mb-1"), label),
		b.Div(mi.Class("text-sm font-medium text-gray-900 dark:text-white"), value),
	)
}

func alertLinkedItem(b *mi.Builder, label, name, href string, hasLink bool, t func(key string, args ...any) string) mi.Node {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	if !hasLink || name == "" {
		return b.Div(mi.Class("flex items-center gap-2 px-3 py-2 bg-gray-100 dark:bg-gray-700 rounded-lg"),
			b.Span(mi.Class("text-xs text-gray-500 dark:text-gray-400"), label+":"),
			b.Span(mi.Class("text-sm text-gray-400"), t("alerts.none")),
		)
	}
	return b.A(mi.Href(href), mi.Class("flex items-center gap-2 px-3 py-2 bg-indigo-50 dark:bg-indigo-900/30 rounded-lg hover:bg-indigo-100 dark:hover:bg-indigo-900/50"),
		b.Span(mi.Class("text-xs text-indigo-600 dark:text-indigo-400"), label+":"),
		b.Span(mi.Class("text-sm font-medium text-indigo-700 dark:text-indigo-300"), name),
	)
}

func alertActionButton(b *mi.Builder, currentStatus, action, label, icon, variant string) mi.Node {
	// Disable certain actions based on status
	disabled := false
	if currentStatus == "resolved" || currentStatus == "dismissed" {
		disabled = true
	}
	if currentStatus == "acknowledged" && action == "acknowledge" {
		disabled = true
	}

	baseClass := "inline-flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-colors"
	variantClass := ""
	switch variant {
	case "success":
		variantClass = "bg-green-600 text-white hover:bg-green-700"
	case "ghost":
		variantClass = "text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700"
	default:
		variantClass = "bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600"
	}

	if disabled {
		return b.Button(mi.Class(baseClass+" opacity-50 cursor-not-allowed "+variantClass), mi.Disabled(),
			b.Span(mi.Class("material-icons text-lg"), icon),
			label,
		)
	}

	return b.Button(mi.Class(baseClass+" "+variantClass),
		b.Span(mi.Class("material-icons text-lg"), icon),
		label,
	)
}

func alertHistoryTimeline(history []AlertHistoryEntry, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	return func(b *mi.Builder) mi.Node {
		if len(history) == 0 {
			return b.Div(mi.Class("text-center py-8 text-gray-500 dark:text-gray-400"),
				b.P(t("alerts.no_history")),
			)
		}

		actionLabel := map[string]string{
			"created": t("alerts.history.created"), "acknowledged": t("alerts.history.acknowledged"),
			"resolved": t("alerts.history.resolved"), "dismissed": t("alerts.history.dismissed"),
		}

		items := make([]interface{}, len(history))
		for i, h := range history {
			actionIcon := "info"
			actionColor := "bg-gray-400"
			switch h.Action {
			case "created":
				actionIcon = "add_circle"
				actionColor = "bg-blue-500"
			case "acknowledged":
				actionIcon = "check_circle"
				actionColor = "bg-yellow-500"
			case "resolved":
				actionIcon = "done_all"
				actionColor = "bg-green-500"
			case "dismissed":
				actionIcon = "cancel"
				actionColor = "bg-gray-500"
			}
			label := actionLabel[h.Action]
			if label == "" {
				label = h.Action
			}

			items[i] = b.Div(mi.Class("flex gap-4"),
				b.Div(mi.Class("flex flex-col items-center"),
					b.Div(mi.Class("w-8 h-8 rounded-full "+actionColor+" flex items-center justify-center"),
						b.Span(mi.Class("material-icons text-white text-sm"), actionIcon),
					),
					b.Div(mi.Class("w-0.5 flex-1 bg-gray-200 dark:bg-gray-700 my-2")),
				),
				b.Div(mi.Class("flex-1 pb-6"),
					b.Div(mi.Class("flex items-center gap-2 mb-1"),
						b.Span(mi.Class("font-medium text-gray-900 dark:text-white"), label),
						b.Span(mi.Class("text-xs text-gray-500 dark:text-gray-400"), h.Timestamp),
					),
					b.P(mi.Class("text-sm text-gray-600 dark:text-gray-400"),
						t("alerts.by")+h.User,
					),
					conditionalNotes(b, h.Notes),
				),
			)
		}

		return b.Div(append([]interface{}{mi.Class("space-y-0")}, items...)...)
	}
}

func conditionalNotes(b *mi.Builder, notes string) mi.Node {
	if notes == "" {
		return b.Span()
	}
	return b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400 mt-1 italic"),
		"\""+notes+"\"",
	)
}

// =============================================================================
// ASSET TYPE EDITOR PAGES
// =============================================================================

// AssetTypeFormData holds data for the asset type create/edit form.
type AssetTypeFormData struct {
	// Form state
	ID        int64
	IsEdit    bool
	IsPartial bool // true when rendered inside SeamModal2
	CSRFToken string
	Error     string

	// Field values
	Name              string
	Slug              string
	Description       string
	Icon              string
	Color             string
	FSMID             string
	ConnectivityTiers map[string]bool
	IdentityFields    []FieldRow
	StateFields       []FieldRow
	// P-21: default alert configuration
	DefaultAlertaID int64
	Alertas         []AlertaRow // dropdown data
}

// FieldRow represents a single field definition in the editor UI.
type FieldRow struct {
	Key      string
	Label    string
	Type     string
	Required bool
}

// AssetTypeDetailData holds data for the asset type detail/view page.
type AssetTypeDetailData struct {
	ID          int64
	Name        string
	Slug        string
	Description string
	Icon        string
	Color       string
	FieldCount  int
	AssetCount  int
	CSRFToken   string
}

// AssetTypeFormPage renders the create or edit form for an asset type.
func AssetTypeFormPage(data AssetTypeFormData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	title := t("assettypes.new_title")
	action := "/asset-types"
	if data.IsEdit {
		title = t("assettypes.edit_title")
		action = fmt.Sprintf("/asset-types/%d", data.ID)
	}

	fieldTypes := []string{"text", "number", "boolean", "date", "datetime", "select", "ref"}
	connectivityOptions := []struct{ Value, Label string }{
		{"qr", t("assettypes.connectivity_tier.qr")},
		{"gateway", t("assettypes.connectivity_tier.gateway")},
		{"lora", t("assettypes.connectivity_tier.lora")},
		{"gps", t("assettypes.connectivity_tier.gps")},
	}

	return func(b *mi.Builder) mi.Node {
		// Partial render: simple form for SeamModal2 (no FSM/field editors)
		if data.IsPartial {
			inner := []interface{}{
				b.Input(mi.Type("hidden"), mi.Name("_csrf"), mi.Attr("value", data.CSRFToken)),
				masterFormError(b, data.Error),
				masterFormInputErr(b, t("field.name"), "name", data.Name, "text", true, ""),
				masterFormInputErr(b, t("field.description"), "description", data.Description, "text", false, ""),
				masterFormInputErr(b, t("assettypes.field.color"), "color", data.Color, "text", false, ""),
				b.Div(mi.Class("mb-4 p-3 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-700 rounded text-xs text-amber-700 dark:text-amber-400"),
					t("assettypes.partial_note"),
				),
			}
			return masterPartialFormWrapper(b, "entity-edit-form", action, inner, t)
		}
		return b.Div(mi.Class("max-w-3xl mx-auto space-y-6"),
			// Header
			b.Div(mi.Class("flex items-center gap-3"),
				b.A(mi.Href("/asset-types"), mi.Class("text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"),
					b.Span(mi.Class("material-icons"), "arrow_back"),
				),
				b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), title),
			),

			// Error banner
			func() mi.Node {
				if data.Error == "" {
					return b.Text("")
				}
				return b.Div(mi.Class("bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-4"),
					b.P(mi.Class("text-sm text-red-700 dark:text-red-300"), data.Error),
				)
			}(),

			// Form
			b.Form(mi.Method("POST"), mi.Action(action), mi.Class("space-y-6"),
				b.Input(mi.Type("hidden"), mi.Name("_csrf"), mi.Value(data.CSRFToken)),

				// Basic info card
				b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6 space-y-4"),
					b.H2(mi.Class("text-lg font-semibold text-gray-900 dark:text-white mb-4"), t("assettypes.section.basic_info")),

					// Name
					b.Div(
						b.Label(mi.For("name"), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"), t("field.name")),
						b.Input(mi.Type("text"), mi.ID("name"), mi.Name("name"), mi.Value(data.Name),
							mi.Required(),
							mi.Class("w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-indigo-500"),
							mi.Placeholder(t("assettypes.name_placeholder")),
						),
					),

					// Slug (hidden on edit)
					func() mi.Node {
						if data.IsEdit {
							return b.Input(mi.Type("hidden"), mi.Name("slug"), mi.Value(data.Slug))
						}
						return b.Div(
							b.Label(mi.For("slug"), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"), t("assettypes.field.slug")),
							b.Input(mi.Type("text"), mi.ID("slug"), mi.Name("slug"), mi.Value(data.Slug),
								mi.Required(),
								mi.Class("w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-indigo-500"),
								mi.Placeholder(t("assettypes.slug_placeholder")),
								mi.Attr("pattern", "^[a-z0-9][a-z0-9-]*[a-z0-9]$"),
							),
							b.P(mi.Class("text-xs text-gray-500 mt-1"), t("assettypes.field.slug_hint")),
						)
					}(),

					// Description
					b.Div(
						b.Label(mi.For("description"), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"), t("field.description")),
						b.Textarea(mi.ID("description"), mi.Name("description"), mi.Rows(3),
							mi.Class("w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-indigo-500"),
							mi.Placeholder(t("assettypes.description_placeholder")),
							b.Text(data.Description),
						),
					),

					// Color
					b.Div(
						b.Label(mi.For("color"), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"), t("assettypes.field.colour")),
						b.Div(mi.Class("flex items-center gap-3"),
							b.Input(mi.Type("color"), mi.ID("color"), mi.Name("color"),
								mi.Value(func() string {
									if data.Color != "" {
										return data.Color
									}
									return "#6366f1"
								}()),
								mi.Class("h-10 w-16 rounded border border-gray-300 dark:border-gray-600 cursor-pointer"),
							),
							b.P(mi.Class("text-xs text-gray-500"), t("assettypes.field.colour_hint")),
						),
					),
				),

				// Connectivity tiers card
				b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6"),
					b.H2(mi.Class("text-lg font-semibold text-gray-900 dark:text-white mb-4"), t("assettypes.section.connectivity")),
					b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400 mb-4"), t("assettypes.connectivity_hint")),
					b.Div(mi.Class("grid grid-cols-2 gap-3"),
						func() mi.Node {
							items := make([]interface{}, len(connectivityOptions))
							for i, opt := range connectivityOptions {
								checked := data.ConnectivityTiers[opt.Value]
								var chk mi.Node
								if checked {
									chk = b.Input(mi.Type("checkbox"), mi.Name("connectivity_tiers"), mi.Value(opt.Value),
										mi.Attr("checked", ""),
										mi.Class("rounded border-gray-300 text-indigo-600"),
									)
								} else {
									chk = b.Input(mi.Type("checkbox"), mi.Name("connectivity_tiers"), mi.Value(opt.Value),
										mi.Class("rounded border-gray-300 text-indigo-600"),
									)
								}
								items[i] = b.Label(mi.Class("flex items-center gap-2 p-3 border border-gray-200 dark:border-gray-600 rounded-lg cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700"),
									chk,
									b.Span(mi.Class("text-sm text-gray-700 dark:text-gray-300"), opt.Label),
								)
							}
							return b.Div(append([]interface{}{mi.Class("grid grid-cols-2 gap-3")}, items...)...)
						}(),
					),
				),

				// Identity fields card
				b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6"),
					b.Div(mi.Class("flex items-center justify-between mb-4"),
						b.Div(
							b.H2(mi.Class("text-lg font-semibold text-gray-900 dark:text-white"), t("assettypes.section.identity_fields")),
							b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400"), t("assettypes.identity_fields_hint")),
						),
						b.Button(mi.Type("button"), mi.ID("add-identity-field"),
							mi.Class("inline-flex items-center gap-1 px-3 py-1.5 text-sm border border-indigo-300 text-indigo-600 rounded-lg hover:bg-indigo-50"),
							b.Span(mi.Class("material-icons text-base"), "add"),
							t("assettypes.add_field"),
						),
					),
					b.Div(mi.ID("identity-fields-list"), mi.Class("space-y-2"),
						func() mi.Node {
							if len(data.IdentityFields) == 0 {
								return b.P(mi.Class("text-sm text-gray-400 italic"), t("assettypes.no_identity_fields"))
							}
							items := make([]interface{}, len(data.IdentityFields))
							for i, f := range data.IdentityFields {
								items[i] = assetTypeFieldRow(b, "identity", f, fieldTypes, t)
							}
							return b.Div(append([]interface{}{mi.Class("space-y-2")}, items...)...)
						}(),
					),
				),

				// State fields card
				b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6"),
					b.Div(mi.Class("flex items-center justify-between mb-4"),
						b.Div(
							b.H2(mi.Class("text-lg font-semibold text-gray-900 dark:text-white"), t("assettypes.section.state_fields")),
							b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400"), t("assettypes.state_fields_hint")),
						),
						b.Button(mi.Type("button"), mi.ID("add-state-field"),
							mi.Class("inline-flex items-center gap-1 px-3 py-1.5 text-sm border border-indigo-300 text-indigo-600 rounded-lg hover:bg-indigo-50"),
							b.Span(mi.Class("material-icons text-base"), "add"),
							t("assettypes.add_field"),
						),
					),
					b.Div(mi.ID("state-fields-list"), mi.Class("space-y-2"),
						func() mi.Node {
							if len(data.StateFields) == 0 {
								return b.P(mi.Class("text-sm text-gray-400 italic"), t("assettypes.no_state_fields"))
							}
							items := make([]interface{}, len(data.StateFields))
							for i, f := range data.StateFields {
								items[i] = assetTypeFieldRow(b, "state", f, fieldTypes, t)
							}
							return b.Div(append([]interface{}{mi.Class("space-y-2")}, items...)...)
						}(),
					),
				),

				// Alert configuration card (P-21)
				func() mi.Node {
					if len(data.Alertas) == 0 {
						return b.Span() // no alertas defined yet — section hidden
					}
					alertOpts := []interface{}{
						mi.Name("default_alerta_id"), mi.ID("default_alerta_id"),
						mi.Class("mt-1 w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm"),
						b.Option(mi.Value("0"), t("assettypes.select_no_default_alert")),
					}
					for _, a := range data.Alertas {
						val := fmt.Sprintf("%d", a.ID)
						label := fmt.Sprintf("%s — %s (%s)", AlertaTypeLabel(a.Type, t), a.Description, AlertaCriticalityLabel(a.Criticality, t))
						if a.ID == data.DefaultAlertaID {
							alertOpts = append(alertOpts, b.Option(mi.Value(val), mi.Selected(), label))
						} else {
							alertOpts = append(alertOpts, b.Option(mi.Value(val), label))
						}
					}
					return b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6"),
						b.H2(mi.Class("text-lg font-semibold text-gray-900 dark:text-white mb-1"), t("assettypes.section.alert_config")),
						b.P(mi.Class("text-sm text-gray-500 dark:text-gray-400 mb-4"), t("assettypes.alert_config_hint")),
						b.Label(mi.For("default_alerta_id"), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300"), t("assettypes.field.default_alert")),
						b.Select(alertOpts...),
					)
				}(),

				// Submit buttons
				b.Div(mi.Class("flex items-center justify-between pt-2"),
					b.A(mi.Href("/asset-types"), mi.Class("px-4 py-2 text-sm text-gray-600 dark:text-gray-400 hover:text-gray-800"),
						t("action.cancel"),
					),
					b.Button(mi.Type("submit"),
						mi.Class("px-6 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 font-medium text-sm"),
						conditionalButtonText(data.IsEdit, t("assettypes.save_changes"), t("assettypes.create")),
					),
				),
			),

			// JS for dynamic field rows
			b.Script(mi.Raw(`
(function() {
  function makeFieldRow(prefix, fieldTypes) {
    var idx = Date.now();
    var row = document.createElement('div');
    row.className = 'flex items-center gap-2 p-3 bg-gray-50 dark:bg-gray-700 rounded-lg';
    row.innerHTML =
      '<input type="text" name="' + prefix + '_field_key[]" placeholder="key (e.g. serial_no)"' +
      ' class="flex-1 min-w-0 px-2 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100" required>' +
      '<input type="text" name="' + prefix + '_field_label[]" placeholder="Label"' +
      ' class="flex-1 min-w-0 px-2 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100" required>' +
      '<select name="' + prefix + '_field_type[]"' +
      ' class="px-2 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100">' +
      fieldTypes.map(function(t){ return '<option value="' + t + '">' + t + '</option>'; }).join('') +
      '</select>' +
      '<label class="flex items-center gap-1 text-xs text-gray-600 dark:text-gray-300">' +
      '<input type="checkbox" name="' + prefix + '_field_required[]" value="1"> `+t("field.required")+`</label>' +
      '<button type="button" class="text-red-400 hover:text-red-600 ml-1" onclick="this.closest(\'div\').remove()">' +
      '<span class="material-icons text-lg">remove_circle_outline</span></button>';
    return row;
  }
  var types = `+fmt.Sprintf("%q", fmt.Sprintf("%s", joinStrings(fieldTypes, ",")))+`;
  var typeArr = types.split(',');
  document.getElementById('add-identity-field').addEventListener('click', function() {
    document.getElementById('identity-fields-list').appendChild(makeFieldRow('identity', typeArr));
  });
  document.getElementById('add-state-field').addEventListener('click', function() {
    document.getElementById('state-fields-list').appendChild(makeFieldRow('state', typeArr));
  });
})();
`)),
		)
	}
}

func assetTypeFieldRow(b *mi.Builder, prefix string, f FieldRow, fieldTypes []string, t func(key string, args ...any) string) mi.Node {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	typeOpts := make([]interface{}, len(fieldTypes))
	for i, ft := range fieldTypes {
		if ft == f.Type {
			typeOpts[i] = b.Option(mi.Value(ft), mi.Attr("selected", ""), b.Text(ft))
		} else {
			typeOpts[i] = b.Option(mi.Value(ft), b.Text(ft))
		}
	}
	var reqCheck mi.Node
	if f.Required {
		reqCheck = b.Input(mi.Type("checkbox"), mi.Name(prefix+"_field_required[]"), mi.Value("1"), mi.Attr("checked", ""))
	} else {
		reqCheck = b.Input(mi.Type("checkbox"), mi.Name(prefix+"_field_required[]"), mi.Value("1"))
	}
	return b.Div(mi.Class("flex items-center gap-2 p-3 bg-gray-50 dark:bg-gray-700 rounded-lg"),
		b.Input(mi.Type("text"), mi.Name(prefix+"_field_key[]"), mi.Value(f.Key),
			mi.Class("flex-1 min-w-0 px-2 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100"),
			mi.Required(),
		),
		b.Input(mi.Type("text"), mi.Name(prefix+"_field_label[]"), mi.Value(f.Label),
			mi.Class("flex-1 min-w-0 px-2 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100"),
			mi.Required(),
		),
		b.Select(append([]interface{}{mi.Name(prefix + "_field_type[]"),
			mi.Class("px-2 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100")},
			typeOpts...)...),
		b.Label(mi.Class("flex items-center gap-1 text-xs text-gray-600 dark:text-gray-300"),
			reqCheck,
			b.Text(t("field.required")),
		),
		b.Button(mi.Type("button"),
			mi.Class("text-red-400 hover:text-red-600 ml-1"),
			mi.Attr("onclick", "this.closest('div').remove()"),
			b.Span(mi.Class("material-icons text-lg"), "remove_circle_outline"),
		),
	)
}

// AssetTypeDetailPage renders the detail view for a single asset type.
func AssetTypeDetailPage(data AssetTypeDetailData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	return func(b *mi.Builder) mi.Node {
		return b.Div(mi.Class("max-w-3xl mx-auto space-y-6"),
			// Header
			b.Div(mi.Class("flex items-center justify-between"),
				b.Div(mi.Class("flex items-center gap-3"),
					b.A(mi.Href("/asset-types"), mi.Class("text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"),
						b.Span(mi.Class("material-icons"), "arrow_back"),
					),
					b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), data.Name),
				),
				b.Div(mi.Class("flex items-center gap-2"),
					b.A(mi.Href(fmt.Sprintf("/asset-types/%d/edit", data.ID)),
						mi.Class("inline-flex items-center gap-1 px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 text-sm font-medium"),
						b.Span(mi.Class("material-icons text-base"), "edit"),
						t("action.edit"),
					),
					// Delete form
					b.Form(mi.Method("POST"), mi.Action(fmt.Sprintf("/asset-types/%d/delete", data.ID)),
						mi.Attr("onsubmit", fmt.Sprintf("event.preventDefault();SeamConfirm.showDanger('%s',function(){this.submit()}.bind(this),{title:'%s'})", t("assettypes.confirm_delete"), t("assettypes.delete_title"))),
						b.Input(mi.Type("hidden"), mi.Name("_csrf"), mi.Value(data.CSRFToken)),
						b.Button(mi.Type("submit"),
							mi.Class("inline-flex items-center gap-1 px-4 py-2 border border-red-300 text-red-600 rounded-lg hover:bg-red-50 text-sm font-medium"),
							b.Span(mi.Class("material-icons text-base"), "delete"),
							t("action.delete"),
						),
					),
				),
			),

			// Info card
			b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6"),
				b.Dl(mi.Class("grid grid-cols-2 gap-4"),
					b.Div(
						b.Dt(mi.Class("text-sm font-medium text-gray-500"), t("assettypes.field.slug")),
						b.Dd(mi.Class("mt-1 text-sm text-gray-900 dark:text-white font-mono"), data.Slug),
					),
					b.Div(
						b.Dt(mi.Class("text-sm font-medium text-gray-500"), t("assettypes.assets_using_type")),
						b.Dd(mi.Class("mt-1 text-sm text-gray-900 dark:text-white"), fmt.Sprintf("%d", data.AssetCount)),
					),
					b.Div(mi.Class("col-span-2"),
						b.Dt(mi.Class("text-sm font-medium text-gray-500"), t("field.description")),
						b.Dd(mi.Class("mt-1 text-sm text-gray-900 dark:text-white"), func() string {
							if data.Description == "" {
								return t("assettypes.no_description")
							}
							return data.Description
						}()),
					),
				),
			),
		)
	}
}

// =============================================================================
// FSM EDITOR PAGE
// =============================================================================

// FSMEditorData holds data for the canvas-based FSM editor.
type FSMEditorData struct {
	FSMID     string
	FSMName   string
	FSMJSON   string // JSON-encoded FSM backup data
	CSRFToken string
}

// FSMEditorPage renders the FSM designer embedded in the app shell.
func FSMEditorPage(data FSMEditorData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	return func(b *mi.Builder) mi.Node {
		title := t("fsm.editor_title")
		if data.FSMName != "" {
			title = data.FSMName
		}
		return b.Div(mi.Class("flex flex-col h-full"),
			// Page header
			b.Div(mi.Class("flex items-center gap-3 px-4 py-3 border-b border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 shrink-0"),
				b.A(mi.Href("/fsm"), mi.Class("text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"),
					b.Span(mi.Class("material-icons text-xl"), "arrow_back"),
				),
				b.H1(mi.Class("text-lg font-semibold text-gray-900 dark:text-white"), title),
			),
			// FSM editor web component — fills remaining height
			b.Div(mi.Class("grow overflow-hidden"),
				// Saved graph data for the Lit element to restore on boot
				b.Script(mi.Type("application/json"), mi.ID("fsm-data"),
					conditionalText(data.FSMJSON != "", data.FSMJSON, "{}"),
				),
				// FSM engine as plain script (exposes one global: SEAM_EDITOR)
				b.Script(mi.Src("/static/js/seam-fsm-editor.js")),
				// Lit shell module (uses window globals set above)
				b.Script(mi.Type("module"), mi.Src("/static/js/fsm-editor.js")),
				// Mount the component
				//
				// mi.Raw bypasses minty's own automatic escaping entirely
				// (Element.Render's html.EscapeString, confirmed directly
				// from minty's source -- that's what Raw explicitly opts
				// out of). data.FSMName is a real FSM's name, genuinely
				// user-editable, same shape of gap as the Map page's asset-
				// name XSS (T-63, found 2026-08-04) -- a double quote in an
				// FSM's name would break out of fsm-name="..." into
				// arbitrary injected attributes. Found while checking every
				// other mi.Raw(fmt.Sprintf(...)) call site in this package
				// after T-63, not by accident. html.EscapeString applied to
				// every interpolated value here, matching exactly what
				// minty's own Element.Render does for every other attribute
				// in this codebase -- FSMID is already digits-only by
				// construction and CSRFToken is server-generated, but both
				// are escaped anyway rather than reasoning per-field about
				// which one is "safe enough" to skip.
				mi.Raw(fmt.Sprintf(
					`<seam-fsm-editor fsm-id="%s" fsm-name="%s" csrf-token="%s" style="display:flex;flex-direction:column;height:100%%"></seam-fsm-editor>`,
					html.EscapeString(data.FSMID), html.EscapeString(data.FSMName), html.EscapeString(data.CSRFToken),
				)),
			),
		)
	}
}
