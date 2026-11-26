package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	mi "github.com/ha1tch/minty"
	"github.com/ha1tch/seam-ui/internal/auth"
	"github.com/ha1tch/seam-ui/internal/dashboardengine"
	"github.com/ha1tch/seam-ui/internal/dashboardengine/schemas"
	"github.com/ha1tch/seam-ui/internal/features"
	fe "github.com/ha1tch/seam-ui/internal/formengine"
	sensorschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/sensors"
	systemschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/system"
	"github.com/ha1tch/seam-ui/internal/i18n"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

// UIHandler handles UI page routes.
type UIHandler struct {
	page            *PageHandler
	xoluClient      *xolu.Client
	config          Config
	configMu        sync.RWMutex // guards config.TenantName and config.Locale
	logger          *slog.Logger
	translator      *i18n.Translator
	userService     *auth.UserService
	sessionManager  *auth.SessionManager
	csrfManager     *auth.CSRFManager
	loginLimiter    *auth.LoginLimiter
	auditLogger     *auth.AuditLogger
	resetManager    *auth.PasswordResetManager
	dashboardEngine *dashboardengine.DashboardEngine

	// acct is the Accounting & Admin module's state (accounting_ui.go).
	acct acctState

	// selectorUsersScope selects which users the Users selector lists
	// (T-96): "" or SelectorUsersScopeHierarchy (default) = users at or
	// below the caller's role; SelectorUsersScopeTenant = every active
	// user of the tenant. Set once at startup via SetSelectorUsersScope.
	selectorUsersScope string
}

// NewUIHandler creates a new UI handler.
func NewUIHandler(client *xolu.Client, cfg Config, translator *i18n.Translator, userService *auth.UserService, sessionMgr *auth.SessionManager, csrfMgr *auth.CSRFManager, loginLimiter *auth.LoginLimiter, auditLogger *auth.AuditLogger, resetMgr *auth.PasswordResetManager) *UIHandler {
	cfg.Translator = translator
	h := &UIHandler{
		page:           NewPageHandler(cfg),
		xoluClient:     client,
		config:         cfg,
		logger:         slog.Default(),
		translator:     translator,
		userService:    userService,
		sessionManager: sessionMgr,
		csrfManager:    csrfMgr,
		loginLimiter:   loginLimiter,
		auditLogger:    auditLogger,
		resetManager:   resetMgr,
	}
	// dashboardEngine is held long-lived on h, not constructed per-request
	// the way FormEngine is elsewhere in this file -- its cache (MOD-2,
	// internal/dashboardengine/cache.go) needs to persist across requests
	// to do anything at all; a fresh instance per call would reset
	// cacheOnce every time and never actually cache.
	h.dashboardEngine = &dashboardengine.DashboardEngine{
		Resolver:          &xoluDashboardResolver{h: h},
		Translator:        h.dashboardEngineTranslator(),
		PermissionChecker: authPermissionChecker{},
		CacheTTL:          5 * time.Second,
	}
	// Load tenant name and locale persisted in xolu so the top bar shows the
	// correct values immediately, without requiring a config-file restart.
	h.loadPersistedSettings(context.Background())
	// Background refresh: picks up changes (e.g. seed runs after server starts)
	// every 30 seconds without requiring a restart.
	h.startSettingsRefresh()
	return h
}

// AvailableLanguages returns language options from the i18n translator.
func (h *UIHandler) AvailableLanguages(selectedLocale string) []LanguageOption {
	if h.translator == nil {
		// Fallback if no translator
		return []LanguageOption{
			{Code: "en", Name: "English", Selected: selectedLocale == "en"},
		}
	}

	locales := h.translator.Locales()
	options := make([]LanguageOption, 0, len(locales))

	// Map locale codes to display names
	names := map[string]string{
		"en": "English",
		"es": "Español",
		"pt": "Português",
		"fr": "Français",
		"de": "Deutsch",
		"it": "Italiano",
		"ja": "日本語",
		"zh": "中文",
		"ko": "한국어",
		"ar": "العربية",
		"ru": "Русский",
	}

	for _, locale := range locales {
		name, ok := names[locale]
		if !ok {
			name = locale // Use code if no display name
		}
		options = append(options, LanguageOption{
			Code:     locale,
			Name:     name,
			Selected: locale == selectedLocale,
		})
	}

	return options
}

// ValidateLocale checks if a locale is available.
func (h *UIHandler) ValidateLocale(locale string) bool {
	if h.translator == nil {
		return locale == "en"
	}
	return h.translator.HasLocale(locale)
}

// Dashboard handles GET /
// Dashboard handles GET / -- the Assets dashboard. Renders via
// dashboardengine (MOD-2, DASHBOARD_SPEC.md), not dashboard.go's own
// hand-written queries -- see internal/dashboardengine/schemas'
// assets-dashboard.json for the declarative definition this now renders
// from. dashboard.go itself stays in the tree (§2 below), unused by this
// handler but not yet deleted, until parity is proven live -- same
// coexist-then-retire discipline T-01/FE-24 already established for the
// static Asset form.
func (h *UIHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	// Without the Assets module (enable_operational off) this page has
	// nothing to show: "/" goes to the viewer's first application instead,
	// or to their profile when they have none.
	if !h.config.EnableOperational {
		home := ""
		if claims, ok := auth.ClaimsFromContext(r.Context()); ok && h.page.ModuleRegistry != nil {
			home = h.page.ModuleRegistry.Home(claims.TenantID, claims.Role)
		}
		if home == "" {
			home = "/profile"
		}
		http.Redirect(w, r, home, http.StatusSeeOther)
		return
	}
	t := h.translateFunc(r)
	locale := h.config.Locale
	rc := dashboardengine.RenderContext{Locale: locale}
	if claims, ok := auth.ClaimsFromContext(r.Context()); ok {
		rc.Role = claims.Role
		rc.UserID = claims.UserID
	}
	content := h.dashboardEngine.Render(r.Context(), schemas.Assets(), rc)
	h.page.Render(w, r, t("nav.dashboard"), "/", content)
}

// DocumentsDashboard handles GET /documents -- the Documents module's own
// landing page (MODULE_SPEC.md §7.2, MOD-7), mirroring Dashboard's
// shape exactly but rendering schemas.Documents() at "/documents".
// schemas.Documents() has "sections": [] (DASHBOARD_SPEC.md §2.2's
// empty-dashboard case), so this always renders the centred module-name
// + "no data" message, never widget content -- Documents has no widgets
// defined at all yet (DOCUMENTS_MODULE_SPEC.md v0.5 is a design document,
// T-04 to build it is still open). Reuses nav.module.documents for the
// page title too, rather than inventing a documents.dashboard.title key
// with nothing unique to say -- same reasoning DASHBOARD_SPEC.md §2.2
// gives for the dashboard's own internal empty-state text reusing the
// module's Label key.
func (h *UIHandler) DocumentsDashboard(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	locale := h.config.Locale
	rc := dashboardengine.RenderContext{Locale: locale}
	if claims, ok := auth.ClaimsFromContext(r.Context()); ok {
		rc.Role = claims.Role
		rc.UserID = claims.UserID
	}
	content := h.dashboardEngine.Render(r.Context(), schemas.Documents(), rc)
	h.page.Render(w, r, t("nav.module.documents"), "/documents", content)
}

// AssetsList handles GET /assets
func (h *UIHandler) AssetsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query params
	page := queryInt(r, "page", 1)
	pageSize := 20
	offset := (page - 1) * pageSize
	search := r.URL.Query().Get("q")
	status := r.URL.Query().Get("status")
	typeFilter := r.URL.Query().Get("type")
	criticidadFilter := r.URL.Query().Get("criticidad")

	// Build query (OQL uses TOP n, not LIMIT/OFFSET)
	fetchCount := pageSize + offset
	query := "SELECT TOP " + strconv.Itoa(fetchCount) + " * FROM assets"
	conditions := []string{}

	if search != "" {
		conditions = append(conditions, "name LIKE '%"+escapeOQL(search)+"%'")
	}
	if status != "" {
		conditions = append(conditions, "status = '"+escapeOQL(status)+"'")
	}
	if typeFilter != "" && validOQLIntFilter(typeFilter) {
		conditions = append(conditions, "type_id = "+typeFilter)
	}
	if criticidadFilter != "" {
		conditions = append(conditions, "criticality = '"+escapeOQL(criticidadFilter)+"'")
	}

	if len(conditions) > 0 {
		query += " WHERE " + joinStrings(conditions, " AND ")
	}

	query += " ORDER BY updated_at DESC"

	// Fetch assets
	data := AssetListData{
		Page:         page,
		PageSize:     pageSize,
		SearchQuery:  search,
		StatusFilter: status,
		TypeFilter:   typeFilter,
	}

	result, err := h.xoluClient.OQL(ctx, query)
	if err == nil {
		// Build custodio name lookup for this page's results
		custodioNames := map[int64]string{}
		if cr, _ := h.xoluClient.OQL(ctx, "SELECT id, legal_name FROM custodians"); cr != nil {
			for _, row := range cr.Data {
				custodioNames[toInt64(row["id"])] = toString(row["legal_name"])
			}
		}
		// Apply offset by slicing (OQL doesn't support OFFSET)
		rows := result.Data
		if offset > 0 && offset < len(rows) {
			rows = rows[offset:]
		} else if offset >= len(rows) {
			rows = []map[string]any{}
		}
		if len(rows) > pageSize {
			rows = rows[:pageSize]
		}
		for _, row := range rows {
			cid := toInt64(row["custodian_id"])
			data.Assets = append(data.Assets, AssetRow{
				ID:               toInt64(row["id"]),
				Name:             toString(row["name"]),
				TypeName:         toString(row["type_name"]),
				Status:           toString(row["status"]),
				QRCode:           toString(row["qr_code"]),
				ConnectivityTier: toString(row["connectivity_tier"]),
				LastEventTime:    toString(row["last_event_time"]),
				Criticality:      toString(row["criticality"]),
				GIAI:             toString(row["giai"]),
				CustodianName:    custodioNames[cid],
			})
		}
	}

	// Get total count
	countResult, err := h.xoluClient.OQL(ctx, "SELECT COUNT(*) as total FROM assets")
	if err == nil && len(countResult.Data) > 0 {
		data.Total = toInt(countResult.Data[0]["total"])
	}

	// Get asset types for filter
	typesResult, err := h.xoluClient.OQL(ctx, "SELECT id, name FROM asset_types ORDER BY name")
	if err == nil {
		for _, row := range typesResult.Data {
			data.AssetTypes = append(data.AssetTypes, AssetTypeOption{
				ID:   toInt64(row["id"]),
				Name: toString(row["name"]),
			})
		}
	}

	data.AutoOpenCreate = r.URL.Query().Get("create") == "1"

	// Render
	h.page.Render(w, r, h.translateFunc(r)("nav.assets"), "/assets", AssetsListPage(data, h.translateFunc(r)))
}

// AssetDetail handles GET /assets/{id}
func (h *UIHandler) AssetDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	t := h.translateFunc(r)

	// Extract ID from path
	id := extractPathID(r.URL.Path, "/assets/")
	if id == 0 {
		h.page.RenderError(w, 404, t("error.not_found"), t("asset.not_found"))
		return
	}

	// Fetch asset
	entity, err := h.xoluClient.Get(ctx, "assets", id)
	if err != nil {
		h.page.RenderError(w, 404, t("error.not_found"), t("asset.not_found"))
		return
	}

	data := AssetDetailData{
		Asset: AssetDetail{
			ID:               entity.ID,
			Name:             toString(entity.Data["name"]),
			TypeID:           toInt64(entity.Data["type_id"]),
			TypeName:         toString(entity.Data["type_name"]),
			Status:           toString(entity.Data["status"]),
			QRCode:           toString(entity.Data["qr_code"]),
			ConnectivityTier: toString(entity.Data["connectivity_tier"]),
			FSMState:         toString(entity.Data["fsm_state"]),
			GIAI:             toString(entity.Data["giai"]),
			GRAI:             toString(entity.Data["grai"]),
			CodigoBarrasRFID: toString(entity.Data["codigo_barras_rfid"]),
			TagTecnico:       toString(entity.Data["tag_tecnico"]),
			// Block E
			PurchaseDate:        toString(entity.Data["purchase_date"]),
			ExpectedArrivalDate: toString(entity.Data["expected_arrival_date"]),
			ArrivalDate:         toString(entity.Data["arrival_date"]),
			ObsolescenceDate:    toString(entity.Data["obsolescence_date"]),
			ExpirationDate:      toString(entity.Data["expiration_date"]),
			DecommissionDate:    toString(entity.Data["decommission_date"]),
			DecommissionReason:  toString(entity.Data["decommission_reason"]),
			CreatedAt:           entity.CreatedAt.Format("2006-01-02 15:04"),
			UpdatedAt:           entity.UpdatedAt.Format("2006-01-02 15:04"),
		},
	}

	// Extract identity and state fields
	if v, ok := entity.Data["identity_fields"].(map[string]interface{}); ok {
		data.Asset.Identity = v
	}
	if v, ok := entity.Data["state_fields"].(map[string]interface{}); ok {
		data.Asset.State = v
	}
	// Parse lifecycle_links JSON array
	if raw, ok := entity.Data["lifecycle_links"].([]interface{}); ok {
		for _, item := range raw {
			if m, ok := item.(map[string]interface{}); ok {
				data.Asset.LinksCicloVida = append(data.Asset.LinksCicloVida, DocumentLink{
					Name: toString(m["name"]),
					URL:  toString(m["url"]),
					Type: toString(m["type"]),
				})
			}
		}
	}

	// S-11: Fetch runtime FSM state from seam-api when GIAI is set
	if data.Asset.GIAI != "" && h.config.SeamAPIURL != "" {
		if fsm, ok := fetchRuntimeState(h.config.SeamAPIURL, data.Asset.GIAI); ok {
			data.Asset.FSMState = fsm
		}
	}

	// Fetch recent events for this asset
	result, err := h.xoluClient.OQL(ctx, "SELECT TOP 5 * FROM events WHERE asset_id = "+strconv.FormatInt(id, 10)+" ORDER BY timestamp DESC")
	if err == nil {
		for _, row := range result.Data {
			data.RecentEvents = append(data.RecentEvents, EventSummary{
				ID:          toInt64(row["id"]),
				TriggerType: toString(row["trigger_type"]),
				Value:       toString(row["value"]),
				Timestamp:   toString(row["timestamp"]),
			})
		}
	}

	// Render
	h.page.Render(w, r, data.Asset.Name, "/assets", AssetDetailPage(data, t))
}

// EventsList handles GET /events
func (h *UIHandler) EventsList(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()

	// Parse query params
	page := queryInt(r, "page", 1)
	pageSize := 50
	offset := (page - 1) * pageSize
	assetFilter := r.URL.Query().Get("asset_id")
	typeFilter := r.URL.Query().Get("type")
	fromDate := r.URL.Query().Get("from")
	toDate := r.URL.Query().Get("to")

	// Build query (OQL uses TOP n, not LIMIT/OFFSET)
	fetchCount := pageSize + offset
	query := "SELECT TOP " + strconv.Itoa(fetchCount) + " e.*, a.name as asset_name FROM events e JOIN assets a ON e.asset_id = a.id"
	conditions := []string{}

	if assetFilter != "" && validOQLIntFilter(assetFilter) {
		conditions = append(conditions, "e.asset_id = "+assetFilter)
	}
	if typeFilter != "" {
		conditions = append(conditions, "e.trigger_type = '"+escapeOQL(typeFilter)+"'")
	}
	if fromDate != "" {
		conditions = append(conditions, "e.timestamp >= '"+escapeOQL(fromDate)+"'")
	}
	if toDate != "" {
		conditions = append(conditions, "e.timestamp <= '"+escapeOQL(toDate)+"'")
	}

	if len(conditions) > 0 {
		query += " WHERE " + joinStrings(conditions, " AND ")
	}

	query += " ORDER BY e.timestamp DESC"

	data := EventsListData{
		Page:        page,
		PageSize:    pageSize,
		AssetFilter: assetFilter,
		TypeFilter:  typeFilter,
		FromDate:    fromDate,
		ToDate:      toDate,
		TriggerTypes: []string{
			"sensor_driver",
			"custom_driver",
			"qr_scan",
			"api_call",
			"mobile_app",
			"lora_notification",
			"gateway_poll",
		},
	}

	result, err := h.xoluClient.OQL(ctx, query)
	if err == nil {
		// Apply offset by slicing (OQL doesn't support OFFSET)
		rows := result.Data
		if offset > 0 && offset < len(rows) {
			rows = rows[offset:]
		} else if offset >= len(rows) {
			rows = []map[string]any{}
		}
		if len(rows) > pageSize {
			rows = rows[:pageSize]
		}
		for _, row := range rows {
			data.Events = append(data.Events, EventRow{
				ID:          toInt64(row["id"]),
				AssetID:     toInt64(row["asset_id"]),
				AssetName:   toString(row["asset_name"]),
				TriggerType: toString(row["trigger_type"]),
				Value:       toString(row["value"]),
				Timestamp:   toString(row["timestamp"]),
				Processed:   toBool(row["processed"]),
			})
		}
	}

	// Get total count
	countResult, err := h.xoluClient.OQL(ctx, "SELECT COUNT(*) as total FROM events")
	if err == nil && len(countResult.Data) > 0 {
		data.Total = toInt(countResult.Data[0]["total"])
	}

	// Render
	h.page.Render(w, r, t("events.list_title"), "/events", EventsListPage(data, t))
}

// SensorsList handles GET /sensors
func (h *UIHandler) SensorsList(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()

	page := queryInt(r, "page", 1)
	pageSize := 20
	offset := (page - 1) * pageSize
	typeFilter := r.URL.Query().Get("type")
	statusFilter := r.URL.Query().Get("status")

	// OQL uses TOP n, not LIMIT/OFFSET
	fetchCount := pageSize + offset
	query := "SELECT TOP " + strconv.Itoa(fetchCount) + " * FROM sensors"
	conditions := []string{}

	if typeFilter != "" {
		conditions = append(conditions, "type = '"+escapeOQL(typeFilter)+"'")
	}
	if statusFilter != "" {
		conditions = append(conditions, "status = '"+escapeOQL(statusFilter)+"'")
	}

	if len(conditions) > 0 {
		query += " WHERE " + joinStrings(conditions, " AND ")
	}
	query += " ORDER BY last_seen DESC"

	data := SensorsListData{
		Page:         page,
		PageSize:     pageSize,
		TypeFilter:   typeFilter,
		StatusFilter: statusFilter,
		SensorTypes:  []string{"rfid", "ble", "gps", "nfc", "barcode", "camera", "generic"},
	}

	result, err := h.xoluClient.OQL(ctx, query)
	if err == nil {
		// Apply offset by slicing (OQL doesn't support OFFSET)
		rows := result.Data
		if offset > 0 && offset < len(rows) {
			rows = rows[offset:]
		} else if offset >= len(rows) {
			rows = []map[string]any{}
		}
		if len(rows) > pageSize {
			rows = rows[:pageSize]
		}
		for _, row := range rows {
			data.Sensors = append(data.Sensors, SensorRow{
				ID:         toInt64(row["id"]),
				ExternalID: toString(row["external_id"]),
				Name:       toString(row["name"]),
				Type:       toString(row["type"]),
				Protocol:   toString(row["protocol"]),
				Status:     toString(row["status"]),
				LastSeen:   toString(row["last_seen"]),
			})
		}
	}

	countResult, err := h.xoluClient.OQL(ctx, "SELECT COUNT(*) as total FROM sensors")
	if err == nil && len(countResult.Data) > 0 {
		data.Total = toInt(countResult.Data[0]["total"])
	}

	h.page.Render(w, r, t("sensors.list_title"), "/sensors", SensorsListPage(data, t))
}

// RulesList handles GET /rules
func (h *UIHandler) RulesList(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()

	data := RulesListData{}

	result, err := h.xoluClient.OQL(ctx, "SELECT * FROM rules ORDER BY priority ASC")
	if err == nil {
		for _, row := range result.Data {
			data.Rules = append(data.Rules, RuleRow{
				ID:          toInt64(row["id"]),
				Name:        toString(row["name"]),
				Description: toString(row["description"]),
				Enabled:     toBool(row["enabled"]),
				Priority:    toInt(row["priority"]),
				TriggerType: toString(row["trigger_type"]),
				ActionType:  toString(row["action_type"]),
			})
		}
	}
	data.Total = len(data.Rules)

	h.page.Render(w, r, t("rules.list_title"), "/rules", RulesListPage(data, t))
}

// RuleNew handles GET /rules/new
func (h *UIHandler) RuleNew(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()

	data := RuleFormData{
		IsEdit:         false,
		TriggerTypes:   []string{"event", "threshold", "schedule", "geofence"},
		ActionTypes:    []string{"notify", "webhook", "update_asset", "create_alert"},
		ConditionTypes: []string{"value_gt", "value_lt", "value_eq", "duration_exceeded", "change_rate"},
		Rule: RuleDetail{
			Enabled:  true,
			Priority: 50,
		},
		CSRFToken: h.GetCSRFToken(w, r),
	}

	// Fetch asset types
	typesResult, _ := h.xoluClient.OQL(ctx, "SELECT id, name FROM asset_types ORDER BY name")
	if typesResult != nil {
		for _, row := range typesResult.Data {
			data.AssetTypes = append(data.AssetTypes, AssetTypeOption{
				ID:   toInt64(row["id"]),
				Name: toString(row["name"]),
			})
		}
	}

	h.page.Render(w, r, t("rules.create_title"), "/rules", RuleFormPage(data, t))
}

// RuleEdit handles GET /rules/{id}
func (h *UIHandler) RuleEdit(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()

	id := extractPathID(r.URL.Path, "/rules/")
	if id == 0 {
		h.page.RenderError(w, 404, t("error.not_found"), t("rules.not_found"))
		return
	}

	rule, err := h.xoluClient.Get(ctx, "rules", id)
	if err != nil {
		h.page.RenderError(w, 404, t("error.not_found"), t("rules.not_found"))
		return
	}

	data := RuleFormData{
		IsEdit:         true,
		TriggerTypes:   []string{"event", "threshold", "schedule", "geofence"},
		ActionTypes:    []string{"notify", "webhook", "update_asset", "create_alert"},
		ConditionTypes: []string{"value_gt", "value_lt", "value_eq", "duration_exceeded", "change_rate"},
		CSRFToken:      h.GetCSRFToken(w, r),
		Rule: RuleDetail{
			ID:            rule.ID,
			Name:          toString(rule.Data["name"]),
			Description:   toString(rule.Data["description"]),
			Enabled:       toBool(rule.Data["enabled"]),
			Priority:      toInt(rule.Data["priority"]),
			TriggerType:   toString(rule.Data["trigger_type"]),
			ConditionType: toString(rule.Data["condition_type"]),
			Threshold:     toFloat(rule.Data["threshold"]),
			Duration:      toInt(rule.Data["duration"]),
			ActionType:    toString(rule.Data["action_type"]),
			ActionConfig:  toString(rule.Data["action_config"]),
		},
	}

	// Fetch asset types
	typesResult, _ := h.xoluClient.OQL(ctx, "SELECT id, name FROM asset_types ORDER BY name")
	if typesResult != nil {
		for _, row := range typesResult.Data {
			data.AssetTypes = append(data.AssetTypes, AssetTypeOption{
				ID:   toInt64(row["id"]),
				Name: toString(row["name"]),
			})
		}
	}

	h.page.Render(w, r, t("rules.edit_title")+": "+data.Rule.Name, "/rules", RuleFormPage(data, t))
}

// RuleCreate handles POST /rules
func (h *UIHandler) RuleCreate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		h.page.RenderError(w, http.StatusBadRequest, t("error.bad_request"), t("error.invalid_form_data"))
		return
	}

	name := r.FormValue("name")
	if name == "" {
		data := RuleFormData{
			IsEdit:         false,
			TriggerTypes:   []string{"event", "threshold", "schedule", "geofence"},
			ActionTypes:    []string{"notify", "webhook", "update_asset", "create_alert"},
			ConditionTypes: []string{"value_gt", "value_lt", "value_eq", "duration_exceeded", "change_rate"},
			Errors:         map[string]string{"name": t("error.required")},
			CSRFToken:      h.GetCSRFToken(w, r),
		}
		h.page.Render(w, r, t("rules.create_title"), "/rules", RuleFormPage(data, t))
		return
	}

	ruleData := map[string]any{
		"name":           name,
		"description":    r.FormValue("description"),
		"enabled":        r.FormValue("enabled") == "on",
		"priority":       toIntFromString(r.FormValue("priority"), 50),
		"trigger_type":   r.FormValue("trigger_type"),
		"asset_type_id":  r.FormValue("asset_type_id"),
		"condition_type": r.FormValue("condition_type"),
		"threshold":      toFloatFromString(r.FormValue("threshold")),
		"duration":       toIntFromString(r.FormValue("duration"), 0),
		"action_type":    r.FormValue("action_type"),
		"action_config":  r.FormValue("action_config"),
		"created_at":     time.Now().UTC().Format(time.RFC3339),
	}

	if errs := validateForm(ruleData, schemaRuleCreate); errs != nil {
		h.page.RenderError(w, http.StatusUnprocessableEntity, t("error.validation"), firstError(errs))
		return
	}
	_, err := h.xoluClient.Create(ctx, "rules", ruleData)
	if err != nil {
		h.logger.Error("failed to create rule", "error", err)
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.create_failed")+err.Error())
		return
	}

	http.Redirect(w, r, "/rules", http.StatusSeeOther)
}

// RuleUpdate handles POST /rules/{id}
func (h *UIHandler) RuleUpdate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := extractPathID(r.URL.Path, "/rules/")
	if id == 0 {
		h.page.RenderError(w, 404, t("error.not_found"), t("rules.not_found"))
		return
	}

	if err := r.ParseForm(); err != nil {
		h.page.RenderError(w, http.StatusBadRequest, t("error.bad_request"), t("error.invalid_form_data"))
		return
	}

	ruleData := map[string]any{
		"name":           r.FormValue("name"),
		"description":    r.FormValue("description"),
		"enabled":        r.FormValue("enabled") == "on",
		"priority":       toIntFromString(r.FormValue("priority"), 50),
		"trigger_type":   r.FormValue("trigger_type"),
		"asset_type_id":  r.FormValue("asset_type_id"),
		"condition_type": r.FormValue("condition_type"),
		"threshold":      toFloatFromString(r.FormValue("threshold")),
		"duration":       toIntFromString(r.FormValue("duration"), 0),
		"action_type":    r.FormValue("action_type"),
		"action_config":  r.FormValue("action_config"),
		"updated_at":     time.Now().UTC().Format(time.RFC3339),
	}

	if errs := validateForm(ruleData, schemaRuleCreate); errs != nil {
		h.page.RenderError(w, http.StatusUnprocessableEntity, t("error.validation"), firstError(errs))
		return
	}
	// Patch, not Update -- HTTP PUT full-replace vs HTTP PATCH real
	// merge, confirmed directly against the live server (see users.go's
	// identical 2026-08-03 fix). ruleData omits created_at, which Update
	// would have silently wiped on every edit.
	_, err := h.xoluClient.Patch(ctx, "rules", id, ruleData)
	if err != nil {
		h.logger.Error("failed to update rule", "error", err, "id", id)
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.update_failed")+err.Error())
		return
	}

	http.Redirect(w, r, "/rules", http.StatusSeeOther)
}

// FSMEditorNew handles GET /fsm/editor (new FSM — canvas editor)
func (h *UIHandler) FSMEditorNew(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	cfg := h.config
	cfg.FullHeight = true
	cfg.ExtraHead = LitImportMap(cfg) // provides 'lit' for the Lit shell

	data := FSMEditorData{
		CSRFToken: h.GetCSRFToken(w, r),
	}
	h.page.RenderWithConfig(w, r, cfg, t("fsm.editor_title"), "/fsm/editor", FSMEditorPage(data, t))
}

// FSMEditorEdit handles GET /fsm/editor/{id} (edit existing FSM)
func (h *UIHandler) FSMEditorEdit(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := extractPathID(r.URL.Path, "/fsm/editor/")
	if id == 0 {
		h.page.RenderError(w, 404, t("error.not_found"), t("fsm.not_found"))
		return
	}

	entity, err := h.xoluClient.Get(ctx, "fsm_machines", id)
	if err != nil {
		h.page.RenderError(w, 404, t("error.not_found"), t("fsm.not_found"))
		return
	}

	cfg := h.config
	cfg.FullHeight = true
	cfg.ExtraHead = LitImportMap(cfg)

	graphJSON := ""
	if v, ok := entity.Data["graph_json"].(string); ok {
		graphJSON = v
	}

	data := FSMEditorData{
		FSMID:     fmt.Sprintf("%d", id),
		FSMName:   toString(entity.Data["name"]),
		FSMJSON:   graphJSON,
		CSRFToken: h.GetCSRFToken(w, r),
	}
	h.page.RenderWithConfig(w, r, cfg, data.FSMName+" — "+t("fsm.editor_title"), "/fsm/editor", FSMEditorPage(data, t))
}

// AlertsList handles GET /alerts
func (h *UIHandler) AlertsList(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()

	statusFilter := r.URL.Query().Get("status")
	if statusFilter == "" {
		statusFilter = "open"
	}
	severityFilter := r.URL.Query().Get("severity")

	query := "SELECT * FROM alerts"
	conditions := []string{}

	if statusFilter != "" {
		conditions = append(conditions, "status = '"+escapeOQL(statusFilter)+"'")
	}
	if severityFilter != "" {
		conditions = append(conditions, "severity = '"+escapeOQL(severityFilter)+"'")
	}

	if len(conditions) > 0 {
		query += " WHERE " + joinStrings(conditions, " AND ")
	}
	query = "SELECT TOP 100 * FROM alerts" + query[len("SELECT * FROM alerts"):] + " ORDER BY created_at DESC"

	data := AlertsListData{
		StatusFilter:   statusFilter,
		SeverityFilter: severityFilter,
	}

	result, err := h.xoluClient.OQL(ctx, query)
	if err == nil {
		for _, row := range result.Data {
			data.Alerts = append(data.Alerts, AlertRow{
				ID:        toInt64(row["id"]),
				Title:     toString(row["title"]),
				Message:   toString(row["message"]),
				Severity:  toString(row["severity"]),
				Status:    toString(row["status"]),
				Source:    toString(row["source"]),
				AssetID:   toInt64(row["asset_id"]),
				CreatedAt: toString(row["created_at"]),
			})
		}
	}
	data.Total = len(data.Alerts)

	// Count by status
	openResult, _ := h.xoluClient.OQL(ctx, "SELECT COUNT(*) as c FROM alerts WHERE status = 'open'")
	if openResult != nil && len(openResult.Data) > 0 {
		data.OpenCount = toInt(openResult.Data[0]["c"])
	}
	ackResult, _ := h.xoluClient.OQL(ctx, "SELECT COUNT(*) as c FROM alerts WHERE status = 'acknowledged'")
	if ackResult != nil && len(ackResult.Data) > 0 {
		data.AckCount = toInt(ackResult.Data[0]["c"])
	}

	data.CSRFToken = h.GetCSRFToken(w, r)
	h.page.Render(w, r, t("nav.alerts"), "/alerts", AlertsListPage(data, t))
}

// Settings handles GET /settings
func (h *UIHandler) Settings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	t := h.translateFunc(r)
	savedSection := r.URL.Query().Get("saved")

	h.configMu.RLock()
	mapEnabled := h.config.EnableMap
	h.configMu.RUnlock()

	csrfToken := h.GetCSRFToken(w, r)

	// MOD-8: formengine-rendered, with a real Translator wired per
	// request -- see ProfileData's own doc comment (profile.go) for why
	// a shared package-level engine is the wrong pattern, and
	// renderFieldRow's own fix (renderer.go, 2026-08-03) for a real bug
	// this uncovered: self-labeled widgets (toggle/checkbox) never used
	// the resolved i18n title before today, since nothing had wired one
	// into a live route until Profile/Preferences did.
	formEng := &fe.FormEngine{Translator: h.formEngineTranslator()}
	rc := fe.RenderContext{Locale: h.currentLocale(), CSRFToken: csrfToken}
	rc.Module, rc.Form = "system", "general"
	generalContent := formEng.Render(ctx, resolveSchema(ctx, "system", "settings_general", systemschemas.Schema("settings_general")),
		map[string]interface{}{"tenant_name": h.config.TenantName, "locale": h.config.Locale}, nil, rc)
	rc.Form = "notifications"
	notificationsContent := formEng.Render(ctx, resolveSchema(ctx, "system", "settings_notifications", systemschemas.Schema("settings_notifications")),
		map[string]interface{}{"email_alerts": true, "slack_enabled": false, "webhook_enabled": true}, nil, rc)
	rc.Form = "features"
	featuresContent := formEng.Render(ctx, resolveSchema(ctx, "system", "settings_features", systemschemas.Schema("settings_features")),
		map[string]interface{}{"map_enabled": mapEnabled}, nil, rc)

	data := SettingsData{
		TenantName:               h.config.TenantName,
		Locale:                   h.config.Locale,
		EmailAlertsEnabled:       true,
		SlackEnabled:             false,
		WebhooksEnabled:          true,
		MapEnabled:               mapEnabled,
		SavedSection:             savedSection,
		CSRFToken:                csrfToken,
		GeneralFormContent:       generalContent,
		NotificationsFormContent: notificationsContent,
		FeaturesFormContent:      featuresContent,
	}
	h.page.Render(w, r, t("nav.settings"), "/settings", SettingsPage(data, t))
}

// SettingsSaveGeneral handles POST /settings/general
func (h *UIHandler) SettingsSaveGeneral(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, h.errText(r, "error.invalid_form_data", "Invalid form data"), http.StatusBadRequest)
		return
	}

	tenantName := r.FormValue("tenant_name")
	locale := r.FormValue("locale")

	// Validate locale against available locales
	if !h.ValidateLocale(locale) {
		locale = "en"
	}

	// Save to xolu as tenant_settings
	_, err := h.xoluClient.OQL(ctx, "DELETE FROM tenant_settings WHERE key = 'general'")
	if err != nil {
		h.logger.Error("failed to clear old settings", "error", err)
	}

	settings := map[string]interface{}{
		"key":         "general",
		"tenant_name": tenantName,
		"locale":      locale,
		"updated_at":  time.Now().UTC().Format(time.RFC3339),
	}

	_, err = h.xoluClient.Create(ctx, "tenant_settings", settings)
	if err != nil {
		h.logger.Error("failed to save settings", "error", err)
		http.Error(w, h.errText(r, "error.save_failed", "Failed to save settings"), http.StatusInternalServerError)
		return
	}

	// Update in-memory config — must update both UIHandler.config and
	// PageHandler.Config since Render() reads from the latter.
	h.configMu.Lock()
	h.config.TenantName = tenantName
	h.config.Locale = locale
	h.page.Config.TenantName = tenantName
	h.page.Config.Locale = locale
	h.configMu.Unlock()

	if claims, ok := auth.ClaimsFromContext(ctx); ok {
		h.auditLogger.LogSettingsChanged(ctx, r, claims.UserID, claims.Email, "general")
	}

	// Redirect back to settings with success
	http.Redirect(w, r, "/settings?saved=general", http.StatusSeeOther)
}

// SettingsSaveNotifications handles POST /settings/notifications
func (h *UIHandler) SettingsSaveNotifications(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, h.errText(r, "error.invalid_form_data", "Invalid form data"), http.StatusBadRequest)
		return
	}

	// Checkboxes only send value when checked
	emailAlerts := r.FormValue("email_alerts") == "on"
	slackEnabled := r.FormValue("slack_enabled") == "on"
	webhooksEnabled := r.FormValue("webhook_enabled") == "on"

	// Save to xolu
	_, err := h.xoluClient.OQL(ctx, "DELETE FROM tenant_settings WHERE key = 'notifications'")
	if err != nil {
		h.logger.Error("failed to clear old notification settings", "error", err)
	}

	settings := map[string]interface{}{
		"key":              "notifications",
		"email_alerts":     emailAlerts,
		"slack_enabled":    slackEnabled,
		"webhooks_enabled": webhooksEnabled,
		"updated_at":       time.Now().UTC().Format(time.RFC3339),
	}

	_, err = h.xoluClient.Create(ctx, "tenant_settings", settings)
	if err != nil {
		h.logger.Error("failed to save notification settings", "error", err)
		http.Error(w, h.errText(r, "error.save_failed", "Failed to save settings"), http.StatusInternalServerError)
		return
	}

	if claims, ok := auth.ClaimsFromContext(ctx); ok {
		h.auditLogger.LogSettingsChanged(ctx, r, claims.UserID, claims.Email, "notifications")
	}

	http.Redirect(w, r, "/settings?saved=notifications", http.StatusSeeOther)
}

// SettingsSaveFeatures handles POST /settings/features
func (h *UIHandler) SettingsSaveFeatures(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, h.errText(r, "error.invalid_form_data", "Invalid form data"), http.StatusBadRequest)
		return
	}

	mapEnabled := r.FormValue("map_enabled") == "on"

	_, err := h.xoluClient.OQL(ctx, "DELETE FROM tenant_settings WHERE key = 'features'")
	if err != nil {
		h.logger.Error("failed to clear feature settings", "error", err)
	}

	settings := map[string]interface{}{
		"key":         "features",
		"map_enabled": mapEnabled,
		"updated_at":  time.Now().UTC().Format(time.RFC3339),
	}
	_, err = h.xoluClient.Create(ctx, "tenant_settings", settings)
	if err != nil {
		h.logger.Error("failed to save feature settings", "error", err)
		http.Error(w, h.errText(r, "error.save_failed", "Failed to save settings"), http.StatusInternalServerError)
		return
	}

	h.configMu.Lock()
	h.config.EnableMap = mapEnabled
	h.page.Config.EnableMap = mapEnabled
	h.configMu.Unlock()

	if claims, ok := auth.ClaimsFromContext(ctx); ok {
		h.auditLogger.LogSettingsChanged(ctx, r, claims.UserID, claims.Email, "features")
	}

	http.Redirect(w, r, "/settings?saved=features", http.StatusSeeOther)
}

// QRScan handles GET /s/{code} - public QR scan landing page
func (h *UIHandler) QRScan(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()

	// Extract code from path /s/{code}
	code := extractPathSegment(r.URL.Path, "/s/")
	if code == "" {
		h.page.RenderPublicError(w, 404, t("qr.invalid_title"), t("qr.invalid_message"))
		return
	}

	// Look up QR code -> asset. Per-asset issue QR codes (issue_qr_codes)
	// take precedence over the legacy qr_codes table; a revoked code
	// explains itself instead of looking like a broken link.
	assetID, found, revoked := h.resolveScanCode(ctx, code)
	if revoked {
		h.page.RenderPublicError(w, http.StatusGone, t("qr.revoked_title"), t("qr.revoked_message"))
		return
	}
	if !found {
		h.page.RenderPublicError(w, 404, t("qr.not_found_title"), t("qr.not_found_message"))
		return
	}
	if assetID == 0 {
		// QR code exists but not assigned to asset
		h.page.RenderPublic(w, t("qr.unassigned_title"), QRUnassignedPage(code, t))
		return
	}

	// Fetch asset details
	asset, err := h.xoluClient.Get(ctx, "assets", assetID)
	if err != nil {
		h.page.RenderPublicError(w, 404, t("qr.asset_not_found_title"), t("qr.asset_not_found_message"))
		return
	}

	// Record scan event
	_, _ = h.xoluClient.Create(ctx, "events", map[string]interface{}{
		"asset_id":     assetID,
		"trigger_type": "qr_scan",
		"value":        map[string]interface{}{"code": code, "source": "web"},
	})

	// Build scan data
	data := QRScanData{
		Code:       code,
		AssetID:    assetID,
		AssetName:  toString(asset.Data["name"]),
		AssetType:  toString(asset.Data["type_name"]),
		Status:     toString(asset.Data["status"]),
		Location:   toString(asset.Data["location"]),
		LastSeen:   asset.UpdatedAt.Format("2006-01-02 15:04"),
		AssignedTo: toString(asset.Data["assigned_to"]),
	}

	// Fetch recent events
	eventsResult, _ := h.xoluClient.OQL(ctx, "SELECT TOP 3 trigger_type, timestamp FROM events WHERE asset_id = "+strconv.FormatInt(assetID, 10)+" ORDER BY timestamp DESC")
	if eventsResult != nil {
		for _, row := range eventsResult.Data {
			data.RecentActivity = append(data.RecentActivity, toString(row["trigger_type"])+" - "+toString(row["timestamp"]))
		}
	}

	// Without the Issues module there is nothing to report to: the code
	// still identifies the asset, so its page shows the asset (see
	// MountIssueQRReportRoute for why this asks the start-time flag only).
	if !h.config.EnableIssues {
		h.page.RenderPublic(w, data.AssetName, QRScanPage(data, t))
		return
	}

	// The landing is the anonymous report form. The category is the one
	// frozen into the code; only a legacy qr_codes row (which carries none)
	// falls back to the asset.
	assetCategory := h.issueQRCodeCategory(ctx, code, asset)

	h.RenderIssueQRReportLanding(w, r, data, assetCategory)
}

// MapView handles GET /map
func (h *UIHandler) MapView(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	h.configMu.RLock()
	mapEnabled := h.config.EnableMap
	h.configMu.RUnlock()
	if !mapEnabled {
		http.Redirect(w, r, "/assets", http.StatusSeeOther)
		return
	}
	ctx := r.Context()

	statusFilter := r.URL.Query().Get("status")
	typeFilter := r.URL.Query().Get("type")

	// Fetch assets with coordinates (OQL uses TOP n)
	query := "SELECT TOP 500 id, name, status, type_name, latitude, longitude FROM assets WHERE latitude IS NOT NULL AND longitude IS NOT NULL"
	if statusFilter != "" {
		query += " AND status = '" + escapeOQL(statusFilter) + "'"
	}
	if typeFilter != "" {
		query += " AND type_name = '" + escapeOQL(typeFilter) + "'"
	}

	data := MapViewData{
		StatusFilter: statusFilter,
		TypeFilter:   typeFilter,
	}

	result, err := h.xoluClient.OQL(ctx, query)
	if err == nil {
		for _, row := range result.Data {
			lat := toFloat(row["latitude"])
			lng := toFloat(row["longitude"])
			if lat != 0 && lng != 0 {
				data.Markers = append(data.Markers, MapMarker{
					ID:       toInt64(row["id"]),
					Name:     toString(row["name"]),
					Status:   toString(row["status"]),
					TypeName: toString(row["type_name"]),
					Lat:      lat,
					Lng:      lng,
				})
			}
		}
	}

	// Get asset types for filter
	typesResult, _ := h.xoluClient.OQL(ctx, "SELECT DISTINCT type_name FROM assets WHERE type_name IS NOT NULL")
	if typesResult != nil {
		for _, row := range typesResult.Data {
			if name := toString(row["type_name"]); name != "" {
				data.AssetTypes = append(data.AssetTypes, name)
			}
		}
	}

	h.page.Render(w, r, t("map.title"), "/map", MapViewPage(data, t))
}

// AssetTypeFields handles GET /assets/type-fields?type_id=N
// Returns an HTMX partial with the dynamic fields for the selected asset type.
func (h *UIHandler) AssetTypeFields(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	t := h.translateFunc(r)
	typeIDStr := r.URL.Query().Get("type_id")
	if typeIDStr == "" {
		// No type selected — return empty div
		h.page.RenderPartial(w, func(b *mi.Builder) mi.Node { return b.Span() })
		return
	}
	typeID, err := strconv.ParseInt(typeIDStr, 10, 64)
	if err != nil || typeID == 0 {
		h.page.RenderPartial(w, func(b *mi.Builder) mi.Node { return b.Span() })
		return
	}
	at, err := h.xoluClient.Get(ctx, "asset_types", typeID)
	if err != nil {
		h.page.RenderPartial(w, func(b *mi.Builder) mi.Node { return b.Span() })
		return
	}
	identityFields := extractFieldRows(at.Data["identity_template"])
	stateFields := extractFieldRows(at.Data["state_template"])
	h.page.RenderPartial(w, func(b *mi.Builder) mi.Node {
		return assetDynamicFields(b, identityFields, stateFields, nil, t)
	})
}

// SensorNew handles GET /sensors/new
func (h *UIHandler) SensorNew(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	if IsHTMXRequest(r) {
		h.page.RenderPartial(w, SensorFormPage(SensorFormData{IsPartial: true, CSRFToken: h.GetCSRFToken(w, r)}, t))
		return
	}
	h.page.Render(w, r, t("sensors.new"), "/sensors", SensorFormPage(SensorFormData{CSRFToken: h.GetCSRFToken(w, r)}, t))
}

// SensorCreate handles POST /sensors
func (h *UIHandler) SensorCreate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		h.page.RenderError(w, http.StatusBadRequest, t("error.bad_request"), t("error.invalid_form_data"))
		return
	}

	externalID := r.FormValue("external_id")
	sensorType := r.FormValue("type")
	driver := r.FormValue("driver")
	name := r.FormValue("name")

	errors := make(map[string]string)
	if externalID == "" {
		errors["external_id"] = t("error.required")
	}
	if sensorType == "" {
		errors["type"] = t("error.required")
	}
	if driver == "" {
		errors["driver"] = t("error.required")
	}

	if len(errors) > 0 {
		h.page.Render(w, r, t("sensors.new"), "/sensors", SensorFormPage(SensorFormData{
			ExternalID: externalID,
			Type:       sensorType,
			Driver:     driver,
			Name:       name,
			Errors:     errors,
			CSRFToken:  h.GetCSRFToken(w, r),
		}, t))
		return
	}

	sensorData := map[string]any{
		"external_id": externalID,
		"type":        sensorType,
		"driver":      driver,
		"name":        name,
		"status":      "active",
	}

	if errs := validateForm(sensorData, schemaSensorCreate); errs != nil {
		h.page.RenderError(w, http.StatusUnprocessableEntity, t("error.validation"), firstError(errs))
		return
	}
	_, err := h.xoluClient.Create(ctx, "sensors", sensorData)
	if err != nil {
		h.logger.Error("failed to create sensor", "error", err)
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.create_failed")+err.Error())
		return
	}

	http.Redirect(w, r, "/sensors", http.StatusSeeOther)
}

// AssetTypesList handles GET /asset-types
func (h *UIHandler) AssetTypesList(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()

	data := AssetTypesListData{}

	result, err := h.xoluClient.OQL(ctx, "SELECT * FROM asset_types ORDER BY name")
	if err == nil {
		for _, row := range result.Data {
			data.Types = append(data.Types, AssetTypeRow{
				ID:          toInt64(row["id"]),
				Name:        toString(row["name"]),
				Description: toString(row["description"]),
				FieldCount:  toInt(row["field_count"]),
				AssetCount:  toInt(row["asset_count"]),
			})
		}
	}
	data.Total = len(data.Types)

	h.page.Render(w, r, t("assettypes.list_title"), "/asset-types", AssetTypesListPage(data, t))
}

// AssetTypeNew handles GET /asset-types/new
func (h *UIHandler) AssetTypeNew(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	data := AssetTypeFormData{
		CSRFToken:         h.GetCSRFToken(w, r),
		Color:             "#6366f1",
		ConnectivityTiers: map[string]bool{},
		Alertas:           h.loadAlertas(ctx),
	}
	h.page.Render(w, r, t("assettypes.new_title"), "/asset-types", AssetTypeFormPage(data, t))
}

// AssetTypeCreate handles POST /asset-types
func (h *UIHandler) AssetTypeCreate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		h.page.RenderError(w, http.StatusBadRequest, t("error.bad_request"), t("error.invalid_form_data"))
		return
	}

	identityTemplate := parseFieldsFromForm(r, "identity")
	stateTemplate := parseFieldsFromForm(r, "state")
	tiers := r.Form["connectivity_tiers"]

	data := map[string]any{
		"name":               r.FormValue("name"),
		"slug":               r.FormValue("slug"),
		"description":        r.FormValue("description"),
		"color":              r.FormValue("color"),
		"connectivity_tiers": tiers,
		"identity_template":  identityTemplate,
		"state_template":     stateTemplate,
		"default_alerta_id":  toInt64FromString(r.FormValue("default_alerta_id")),
	}

	if errs := validateForm(data, schemaAssetTypeCreate); errs != nil {
		h.page.RenderError(w, http.StatusUnprocessableEntity, t("error.validation"), firstError(errs))
		return
	}
	_, err := h.xoluClient.Create(ctx, "asset_types", data)
	if err != nil {
		formData := AssetTypeFormData{
			CSRFToken:   h.GetCSRFToken(w, r),
			Error:       t("error.create_failed") + err.Error(),
			Name:        r.FormValue("name"),
			Slug:        r.FormValue("slug"),
			Description: r.FormValue("description"),
			Color:       r.FormValue("color"),
		}
		h.page.Render(w, r, t("assettypes.new_title"), "/asset-types", AssetTypeFormPage(formData, t))
		return
	}

	http.Redirect(w, r, "/asset-types", http.StatusSeeOther)
}

// AssetTypeDetail handles GET /asset-types/{id}
func (h *UIHandler) AssetTypeDetail(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := extractPathID(r.URL.Path, "/asset-types/")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("assettypes.not_found"))
		return
	}

	at, err := h.xoluClient.Get(ctx, "asset_types", id)
	if err != nil {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("assettypes.not_found"))
		return
	}

	// Count assets of this type
	assetCount := 0
	countResult, _ := h.xoluClient.OQL(ctx, "SELECT COUNT(*) as count FROM assets WHERE type_id = "+strconv.FormatInt(id, 10))
	if countResult != nil && len(countResult.Data) > 0 {
		assetCount = toInt(countResult.Data[0]["count"])
	}

	data := AssetTypeDetailData{
		ID:          at.ID,
		Name:        toString(at.Data["name"]),
		Slug:        toString(at.Data["slug"]),
		Description: toString(at.Data["description"]),
		Color:       toString(at.Data["color"]),
		AssetCount:  assetCount,
		CSRFToken:   h.GetCSRFToken(w, r),
	}

	// Count fields
	if it, ok := at.Data["identity_template"].(map[string]any); ok {
		data.FieldCount += len(it)
	}
	if st, ok := at.Data["state_template"].(map[string]any); ok {
		data.FieldCount += len(st)
	}

	h.page.Render(w, r, data.Name, "/asset-types", AssetTypeDetailPage(data, t))
}

// AssetTypeEdit handles GET /asset-types/{id}/edit
func (h *UIHandler) AssetTypeEdit(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	// Path: /asset-types/{id}/edit — strip "/edit" suffix then extract id
	trimmed := r.URL.Path
	if len(trimmed) > 5 && trimmed[len(trimmed)-5:] == "/edit" {
		trimmed = trimmed[:len(trimmed)-5]
	}
	id := extractPathID(trimmed, "/asset-types/")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("assettypes.not_found"))
		return
	}

	at, err := h.xoluClient.Get(ctx, "asset_types", id)
	if err != nil {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("assettypes.not_found"))
		return
	}

	// Parse existing field templates into FieldRow slices
	identityFields := extractFieldRows(at.Data["identity_template"])
	stateFields := extractFieldRows(at.Data["state_template"])

	// Build connectivity tiers map
	tiers := map[string]bool{}
	if ts, ok := at.Data["connectivity_tiers"].([]any); ok {
		for _, tv := range ts {
			if s, ok := tv.(string); ok {
				tiers[s] = true
			}
		}
	}

	data := AssetTypeFormData{
		ID:                id,
		IsEdit:            true,
		CSRFToken:         h.GetCSRFToken(w, r),
		Name:              toString(at.Data["name"]),
		Slug:              toString(at.Data["slug"]),
		Description:       toString(at.Data["description"]),
		Color:             toString(at.Data["color"]),
		ConnectivityTiers: tiers,
		IdentityFields:    identityFields,
		StateFields:       stateFields,
		DefaultAlertaID:   toInt64(at.Data["default_alerta_id"]),
		Alertas:           h.loadAlertas(ctx),
	}

	data.IsPartial = IsHTMXRequest(r)
	if data.IsPartial {
		h.page.RenderPartial(w, AssetTypeFormPage(data, t))
	} else {
		h.page.Render(w, r, t("assettypes.edit_title")+": "+data.Name, "/asset-types", AssetTypeFormPage(data, t))
	}
}

// AssetTypeUpdate handles POST /asset-types/{id}
func (h *UIHandler) AssetTypeUpdate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := extractPathID(r.URL.Path, "/asset-types/")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("assettypes.not_found"))
		return
	}

	if err := r.ParseForm(); err != nil {
		h.page.RenderError(w, http.StatusBadRequest, t("error.bad_request"), t("error.invalid_form_data"))
		return
	}

	identityTemplate := parseFieldsFromForm(r, "identity")
	stateTemplate := parseFieldsFromForm(r, "state")
	tiers := r.Form["connectivity_tiers"]

	data := map[string]any{
		"name":               r.FormValue("name"),
		"description":        r.FormValue("description"),
		"color":              r.FormValue("color"),
		"connectivity_tiers": tiers,
		"identity_template":  identityTemplate,
		"state_template":     stateTemplate,
		"default_alerta_id":  toInt64FromString(r.FormValue("default_alerta_id")),
	}

	if errs := validateForm(data, schemaAssetTypeUpdate); errs != nil {
		h.page.RenderError(w, http.StatusUnprocessableEntity, t("error.validation"), firstError(errs))
		return
	}
	// Patch, not Update -- HTTP PUT full-replace vs HTTP PATCH real
	// merge, confirmed directly against the live server (see users.go's
	// identical 2026-08-03 fix). data carries 7 of a real asset_type's
	// 9 fields (name/description/color/connectivity_tiers/
	// identity_template/state_template/default_alerta_id) -- missing
	// slug and fsm_id, confirmed directly against a real stored
	// asset_type record. Update would have silently wiped those two on
	// every edit.
	_, err := h.xoluClient.Patch(ctx, "asset_types", id, data)
	if err != nil {
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.update_failed")+err.Error())
		return
	}

	if IsHTMXRequest(r) {
		HTMXTrigger(w, "seamModal2Saved")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/asset-types/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// AssetTypeDelete handles POST /asset-types/{id}/delete
func (h *UIHandler) AssetTypeDelete(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	trimmed := r.URL.Path
	if len(trimmed) > 7 && trimmed[len(trimmed)-7:] == "/delete" {
		trimmed = trimmed[:len(trimmed)-7]
	}
	id := extractPathID(trimmed, "/asset-types/")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("assettypes.not_found"))
		return
	}

	// Guard: refuse if assets use this type
	countResult, _ := h.xoluClient.OQL(ctx, "SELECT COUNT(*) as count FROM assets WHERE type_id = "+strconv.FormatInt(id, 10))
	if countResult != nil && len(countResult.Data) > 0 && toInt(countResult.Data[0]["count"]) > 0 {
		h.page.RenderError(w, http.StatusConflict, t("error.cannot_delete"), t("assettypes.in_use"))
		return
	}

	if err := h.xoluClient.Delete(ctx, "asset_types", id); err != nil {
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.delete_failed"))
		return
	}

	http.Redirect(w, r, "/asset-types", http.StatusSeeOther)
}

// parseFieldsFromForm builds an identity_template or state_template map from
// multi-value form arrays: {prefix}_field_key[], {prefix}_field_label[],
// {prefix}_field_type[], {prefix}_field_required[].
func parseFieldsFromForm(r *http.Request, prefix string) map[string]any {
	keys := r.Form[prefix+"_field_key[]"]
	labels := r.Form[prefix+"_field_label[]"]
	types := r.Form[prefix+"_field_type[]"]
	// required is a checkbox so we must check key presence differently
	requiredSet := map[string]bool{}
	for i, key := range keys {
		if i < len(r.Form[prefix+"_field_required[]"]) {
			requiredSet[key] = true
		}
	}
	template := map[string]any{}
	for i, key := range keys {
		if key == "" {
			continue
		}
		label := ""
		if i < len(labels) {
			label = labels[i]
		}
		typ := "text"
		if i < len(types) && types[i] != "" {
			typ = types[i]
		}
		template[key] = map[string]any{
			"type":     typ,
			"label":    label,
			"required": requiredSet[key],
		}
	}
	return template
}

// extractFieldRows converts a template map[string]any into a sorted FieldRow slice.
func extractFieldRows(template any) []FieldRow {
	m, ok := template.(map[string]any)
	if !ok {
		return nil
	}
	rows := make([]FieldRow, 0, len(m))
	for key, val := range m {
		def, ok := val.(map[string]any)
		if !ok {
			continue
		}
		rows = append(rows, FieldRow{
			Key:      key,
			Label:    toString(def["label"]),
			Type:     toString(def["type"]),
			Required: toBool(def["required"]),
		})
	}
	return rows
}

// SensorDelete handles POST /sensors/{id}/delete
func (h *UIHandler) SensorDelete(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id := extractPathID(r.URL.Path, "/sensors/")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("sensors.not_found"))
		return
	}
	if err := h.xoluClient.Delete(ctx, "sensors", id); err != nil {
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.delete_failed"))
		return
	}
	http.Redirect(w, r, "/sensors", http.StatusSeeOther)
}

// SensorBindForm handles GET /sensors/{id}/bind -- the first real
// consumer of the asset-picker widget beyond parent_id (T-19,
// 2026-08-04), and the first real implementation of the "Bind" button
// SensorDetailPage's boundAssetCard has rendered, inert, since before
// this session (no onclick, no form, no href -- a real, confirmed
// dead button, not a hypothetical one).
func (h *UIHandler) SensorBindForm(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("sensors.not_found"))
		return
	}
	sensor, err := h.xoluClient.Get(ctx, "sensors", id)
	if err != nil {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("sensors.not_found"))
		return
	}

	csrfToken := h.GetCSRFToken(w, r)
	formEng := &fe.FormEngine{Translator: h.formEngineTranslator()}
	rc := fe.RenderContext{Locale: h.currentLocale(), CSRFToken: csrfToken, Module: "assets", Form: "sensor_bind"}
	formContent := formEng.Render(ctx, resolveSchema(ctx, "sensors", "bind", sensorschemas.Schema("bind")), nil, nil, rc)

	data := SensorBindFormData{
		SensorID:   id,
		SensorName: toString(sensor.Data["name"]),
		CSRFToken:  csrfToken, FormContent: formContent,
	}
	h.page.Render(w, r, t("sensors.bind.title"), "/sensors", SensorBindFormPage(data, t))
}

// SensorBind handles POST /sensors/{id}/bind -- creates the
// sensor_bindings row directly (sensor_id, asset_id, created_at).
// Deliberately omits binding_metadata (role/protocol, the seed
// scripts' own richer shape, seeds/seed-*.py's seed_sensor_bindings) --
// enrichment data, not core to the binding itself, and asking for it
// up front in a first, minimal working bind flow is scope this item
// doesn't need to carry. A real, later refinement, not silently
// dropped -- noted in the register entry, not just here.
func (h *UIHandler) SensorBind(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("sensors.not_found"))
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, h.errText(r, "error.invalid_form_data", "Invalid form data"), http.StatusBadRequest)
		return
	}
	assetID, err := strconv.ParseInt(r.FormValue("asset_id"), 10, 64)
	if err != nil || assetID == 0 {
		sensor, _ := h.xoluClient.Get(ctx, "sensors", id)
		csrfToken := h.GetCSRFToken(w, r)
		formEng := &fe.FormEngine{Translator: h.formEngineTranslator()}
		rc := fe.RenderContext{Locale: h.currentLocale(), CSRFToken: csrfToken, Module: "assets", Form: "sensor_bind"}
		formContent := formEng.Render(ctx, resolveSchema(ctx, "sensors", "bind", sensorschemas.Schema("bind")), nil,
			map[string]string{"asset_id": t("sensors.bind.error.asset_required")}, rc)
		data := SensorBindFormData{
			SensorID: id, SensorName: toString(sensor.Data["name"]),
			CSRFToken: csrfToken, FormContent: formContent,
		}
		h.page.Render(w, r, t("sensors.bind.title"), "/sensors", SensorBindFormPage(data, t))
		return
	}

	if _, err := h.xoluClient.Create(ctx, "sensor_bindings", map[string]any{
		"sensor_id":  id,
		"asset_id":   assetID,
		"created_at": time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		h.logger.Error("failed to create sensor binding", "sensor_id", id, "asset_id", assetID, "error", err)
		http.Error(w, h.errText(r, "error.save_failed", "Failed to bind sensor"), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/sensors/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// SensorUnbind handles POST /sensors/{id}/unbind -- deletes every
// sensor_bindings row for this sensor (not just the one SensorDetail's
// own SELECT TOP 1 happens to show), so a direct unbind action can't
// leave a stray duplicate behind if one somehow existed.
func (h *UIHandler) SensorUnbind(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("sensors.not_found"))
		return
	}

	result, err := h.xoluClient.OQL(ctx, "SELECT id FROM sensor_bindings WHERE sensor_id = "+strconv.FormatInt(id, 10))
	if err != nil {
		h.logger.Error("failed to list sensor bindings for unbind", "sensor_id", id, "error", err)
		http.Error(w, h.errText(r, "error.save_failed", "Failed to unbind sensor"), http.StatusInternalServerError)
		return
	}
	for _, row := range result.Data {
		bindingID := toInt64(row["id"])
		if bindingID == 0 {
			continue
		}
		if err := h.xoluClient.Delete(ctx, "sensor_bindings", bindingID); err != nil {
			h.logger.Error("failed to delete sensor binding", "binding_id", bindingID, "sensor_id", id, "error", err)
		}
	}
	http.Redirect(w, r, "/sensors/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// SensorDetail handles GET /sensors/{id}
func (h *UIHandler) SensorDetail(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()

	id := extractPathID(r.URL.Path, "/sensors/")
	if id == 0 {
		h.page.RenderError(w, 404, t("error.not_found"), t("sensors.not_found"))
		return
	}

	// Fetch sensor
	sensor, err := h.xoluClient.Get(ctx, "sensors", id)
	if err != nil {
		h.page.RenderError(w, 404, t("error.not_found"), t("sensors.not_found"))
		return
	}

	data := SensorDetailData{
		Sensor: SensorInfo{
			ID:         sensor.ID,
			ExternalID: toString(sensor.Data["external_id"]),
			Name:       toString(sensor.Data["name"]),
			Type:       toString(sensor.Data["type"]),
			Protocol:   toString(sensor.Data["protocol"]),
			Status:     toString(sensor.Data["status"]),
			LastSeen:   sensor.UpdatedAt.Format("2006-01-02 15:04:05"),
			CreatedAt:  sensor.CreatedAt.Format("2006-01-02 15:04"),
		},
	}

	// Get bound asset if any
	bindingResult, _ := h.xoluClient.OQL(ctx, "SELECT TOP 1 a.id, a.name FROM sensor_bindings sb JOIN assets a ON sb.asset_id = a.id WHERE sb.sensor_id = "+strconv.FormatInt(id, 10))
	if bindingResult != nil && len(bindingResult.Data) > 0 {
		data.BoundAssetID = toInt64(bindingResult.Data[0]["id"])
		data.BoundAssetName = toString(bindingResult.Data[0]["name"])
	}

	// Get recent events from this sensor
	eventsResult, _ := h.xoluClient.OQL(ctx, "SELECT TOP 10 * FROM events WHERE sensor_id = "+strconv.FormatInt(id, 10)+" ORDER BY timestamp DESC")
	if eventsResult != nil {
		for _, row := range eventsResult.Data {
			data.RecentEvents = append(data.RecentEvents, EventSummary{
				ID:          toInt64(row["id"]),
				TriggerType: toString(row["trigger_type"]),
				Value:       toString(row["value"]),
				Timestamp:   toString(row["timestamp"]),
			})
		}
	}

	// Get event stats
	statsResult, _ := h.xoluClient.OQL(ctx, "SELECT COUNT(*) as total FROM events WHERE sensor_id = "+strconv.FormatInt(id, 10))
	if statsResult != nil && len(statsResult.Data) > 0 {
		data.TotalEvents = toInt(statsResult.Data[0]["total"])
	}

	data.CSRFToken = h.GetCSRFToken(w, r)
	h.page.Render(w, r, data.Sensor.Name, "/sensors", SensorDetailPage(data, t))
}

// AlertDetail handles GET /alerts/{id}
func (h *UIHandler) AlertDetail(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()

	id := extractPathID(r.URL.Path, "/alerts/")
	if id == 0 {
		h.page.RenderError(w, 404, t("error.not_found"), t("alerts.not_found"))
		return
	}

	alert, err := h.xoluClient.Get(ctx, "alerts", id)
	if err != nil {
		h.page.RenderError(w, 404, t("error.not_found"), t("alerts.not_found"))
		return
	}

	data := AlertDetailData{
		Alert: AlertInfo{
			ID:        alert.ID,
			Title:     toString(alert.Data["title"]),
			Message:   toString(alert.Data["message"]),
			Severity:  toString(alert.Data["severity"]),
			Status:    toString(alert.Data["status"]),
			Source:    toString(alert.Data["source"]),
			AssetID:   toInt64(alert.Data["asset_id"]),
			RuleID:    toInt64(alert.Data["rule_id"]),
			AlertaID:  toInt64(alert.Data["alerta_id"]),
			CreatedAt: alert.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt: alert.UpdatedAt.Format("2006-01-02 15:04:05"),
		},
		CSRFToken: h.GetCSRFToken(w, r),
	}

	// Resolve asset context (P-22)
	if data.Alert.AssetID > 0 {
		asset, _ := h.xoluClient.Get(ctx, "assets", data.Alert.AssetID)
		if asset != nil {
			data.AssetName = toString(asset.Data["name"])
			data.AssetGIAI = toString(asset.Data["giai"])
			// Resolve region name
			if rid := toInt64(asset.Data["region_id"]); rid > 0 {
				if reg, e := h.xoluClient.Get(ctx, "regions", rid); e == nil {
					data.AssetRegion = toString(reg.Data["description"])
				}
			}
			// Resolve custodian name
			if cid := toInt64(asset.Data["custodian_id"]); cid > 0 {
				if cust, e := h.xoluClient.Get(ctx, "custodians", cid); e == nil {
					data.AssetCustodian = toString(cust.Data["legal_name"])
				}
			}
		}
	}

	// Resolve rule name
	if data.Alert.RuleID > 0 {
		rule, _ := h.xoluClient.Get(ctx, "rules", data.Alert.RuleID)
		if rule != nil {
			data.RuleName = toString(rule.Data["name"])
		}
	}

	// Resolve alerta master config (P-22)
	if data.Alert.AlertaID > 0 {
		alerta, _ := h.xoluClient.Get(ctx, "alert_types", data.Alert.AlertaID)
		if alerta != nil {
			data.AlertConfig = toString(alerta.Data["description"])
			data.AlertType = toString(alerta.Data["type"])
			data.AlertCriticality = toString(alerta.Data["criticality"])
		}
	}

	// Get alert history
	historyResult, _ := h.xoluClient.OQL(ctx, "SELECT TOP 20 * FROM alert_history WHERE alert_id = "+strconv.FormatInt(id, 10)+" ORDER BY timestamp DESC")
	if historyResult != nil {
		for _, row := range historyResult.Data {
			data.History = append(data.History, AlertHistoryEntry{
				Action:    toString(row["action"]),
				User:      toString(row["user"]),
				Timestamp: toString(row["timestamp"]),
				Notes:     toString(row["notes"]),
			})
		}
	}

	h.page.Render(w, r, t("alerts.detail_title_prefix")+data.Alert.Title, "/alerts", AlertDetailPage(data, t))
}

// AlertAcknowledge handles POST /alerts/{id}

// AlertAcknowledge handles POST /alerts/{id}/acknowledge (P-23)
func (h *UIHandler) AlertAcknowledge(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	t := h.translateFunc(r)

	// Extract numeric ID from path: /alerts/{id}/acknowledge
	path := r.URL.Path
	const suffix = "/acknowledge"
	if len(path) > len(suffix) && path[len(path)-len(suffix):] == suffix {
		path = path[:len(path)-len(suffix)]
	}
	id := extractPathID(path, "/alerts/")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("alerts.not_found"))
		return
	}

	if err := r.ParseForm(); err != nil {
		h.page.RenderError(w, http.StatusBadRequest, t("error.bad_request"), t("error.invalid_form_data"))
		return
	}

	notes := r.FormValue("notes")

	// Patch, not Update -- HTTP PUT full-replace vs HTTP PATCH real
	// merge, confirmed directly against the live server (see users.go's
	// identical 2026-08-03 fix). A real alert has 9 fields
	// (alerta_id/asset_id/asset_name/created_at/id/message/severity/
	// source/status/title); sending only {status} via Update would wipe
	// the other 8, turning the alert into a near-empty stub.
	_, err := h.xoluClient.Patch(ctx, "alerts", id, map[string]any{
		"status": "acknowledged",
	})
	if err != nil {
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.alert_ack_failed"))
		return
	}

	// Append history entry
	userEmail := ""
	if user, ok := auth.UserFromContext(ctx); ok {
		userEmail = user.Email
	}
	_, _ = h.xoluClient.Create(ctx, "alert_history", map[string]any{
		"alert_id":  id,
		"action":    "acknowledged",
		"user":      userEmail,
		"timestamp": nowISO(),
		"notes":     notes,
	})

	http.Redirect(w, r, fmt.Sprintf("/alerts/%d", id), http.StatusSeeOther)
}

// AlertResolve handles POST /alerts/{id}/resolve
func (h *UIHandler) AlertResolve(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	t := h.translateFunc(r)
	path := r.URL.Path
	const suffix = "/resolve"
	if len(path) > len(suffix) && path[len(path)-len(suffix):] == suffix {
		path = path[:len(path)-len(suffix)]
	}
	id := extractPathID(path, "/alerts/")
	if id == 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("alerts.not_found"))
		return
	}
	// Patch, not Update -- see AlertAcknowledge's identical comment above.
	_, err := h.xoluClient.Patch(ctx, "alerts", id, map[string]any{"status": "resolved"})
	if err != nil {
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.alert_resolve_failed"))
		return
	}
	userEmail := ""
	if user, ok := auth.UserFromContext(ctx); ok {
		userEmail = user.Email
	}
	_, _ = h.xoluClient.Create(ctx, "alert_history", map[string]any{
		"alert_id":  id,
		"action":    "resolved",
		"user":      userEmail,
		"timestamp": nowISO(),
		"notes":     "",
	})
	http.Redirect(w, r, "/alerts", http.StatusSeeOther)
}

func extractPathSegment(path, prefix string) string {
	if len(path) <= len(prefix) {
		return ""
	}
	rest := path[len(prefix):]
	// Find end (next / or end of string)
	end := 0
	for end < len(rest) && rest[end] != '/' {
		end++
	}
	return rest[:end]
}

func toFloat(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return 0
	}
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

func queryInt(r *http.Request, key string, defaultVal int) int {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultVal
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return i
}

func extractPathID(path, prefix string) int64 {
	if len(path) <= len(prefix) {
		return 0
	}
	rest := path[len(prefix):]
	// Find end of ID (next / or end of string)
	end := 0
	for end < len(rest) && rest[end] != '/' {
		end++
	}
	id, err := strconv.ParseInt(rest[:end], 10, 64)
	if err != nil {
		return 0
	}
	return id
}

func nowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += sep + parts[i]
	}
	return result
}

// escapeOQL escapes a string for safe embedding inside a single-quoted
// OQL string literal. Same logic as internal/auth's own escapeOQL
// (store_xolu.go) -- duplicated rather than imported across packages
// for something this small, matching how toString/toFloat/toInt64 are
// each already local to this package rather than shared.
//
// Added 2026-08-04, found via adversarial testing: every list-with-
// filters handler in this file (AssetsList, the events/sensors/alerts
// listings, MapView, AdminUsersList) built its OQL WHERE clause via raw
// string concatenation of URL query parameters, completely unescaped --
// query += " AND status = '" + statusFilter + "'" with statusFilter
// read straight from r.URL.Query().Get("status"). A single quote in
// any of these parameters breaks out of the intended string literal
// into the WHERE clause itself. internal/auth already had this exact
// escaping pattern for its own OQL query construction; this package
// simply never had an equivalent, despite building OQL queries from
// user input in at least six places.
func escapeOQL(s string) string {
	result := make([]byte, 0, len(s)*2)
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			result = append(result, '\'', '\'')
		} else {
			result = append(result, s[i])
		}
	}
	return string(result)
}

// validOQLIntFilter reports whether s is safe to embed, unquoted, in an
// OQL numeric comparison (e.g. "type_id = " + s) -- strconv.Atoi is the
// actual safety check; escapeOQL's quote-doubling doesn't help an
// unquoted numeric context at all, since there's no quote to escape out
// of in the first place. Found alongside escapeOQL: "type_id = "+
// typeFilter and "e.asset_id = "+assetFilter both had this exact gap,
// worse than the quoted string cases since no escaping technique
// applies here -- only a type check does.
func validOQLIntFilter(s string) bool {
	if s == "" {
		return true // caller's own emptiness check decides whether to include the condition at all
	}
	_, err := strconv.Atoi(s)
	return err == nil
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func toInt(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case string:
		value, err := strconv.Atoi(strings.TrimSpace(n))
		if err != nil {
			return 0
		}
		return value
	default:
		return 0
	}
}

func toInt64(v interface{}) int64 {
	switch n := v.(type) {
	case int:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	case string:
		value, err := strconv.ParseInt(strings.TrimSpace(n), 10, 64)
		if err != nil {
			return 0
		}
		return value
	default:
		return 0
	}
}

func toBool(v interface{}) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

// =============================================================================
// USER PROFILE HANDLERS
// =============================================================================

// Profile handles GET /profile
func (h *UIHandler) Profile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	t := h.translateFunc(r)
	savedSection := r.URL.Query().Get("saved")

	// Get current user from context (set by auth middleware)
	// For now, use a mock user or fetch from session
	userID := h.getCurrentUserID(r)

	// Fetch user from xolu
	user, err := h.xoluClient.Get(ctx, "users", userID)
	if err != nil {
		// Create default user if not found (for development)
		user = &xolu.Entity{
			ID: userID,
			Data: map[string]interface{}{
				"email":        "user@example.com",
				"name":         "Demo User",
				"role":         "admin",
				"locale":       "en",
				"timezone":     "UTC",
				"dark_mode":    "system",
				"email_alerts": true,
				"compact_view": false,
			},
		}
	}

	userProfile := UserProfile{
		ID:          user.ID,
		Email:       toString(user.Data["email"]),
		Name:        toString(user.Data["name"]),
		Role:        toString(user.Data["role"]),
		Locale:      toString(user.Data["locale"]),
		Timezone:    toString(user.Data["timezone"]),
		DarkMode:    toString(user.Data["dark_mode"]),
		EmailAlerts: toBool(user.Data["email_alerts"]),
		CompactView: toBool(user.Data["compact_view"]),
		CreatedAt:   formatDay(t, user.CreatedAt),
		LastLoginAt: formatTimeAgo(toString(user.Data["last_login_at"]), t),
	}
	csrfToken := h.GetCSRFToken(w, r)

	// MOD-8: formengine-rendered, with a real Translator wired per
	// request -- see ProfileData's own doc comment for why a shared
	// package-level engine (masters_pages_dynamic.go's own precedent)
	// is the wrong pattern here.
	formEng := &fe.FormEngine{Translator: h.formEngineTranslator()}
	rc := fe.RenderContext{Locale: h.currentLocale(), CSRFToken: csrfToken}
	rc.Module, rc.Form = "system", "profile"
	profileContent := formEng.Render(ctx, resolveSchema(ctx, "system", "profile", systemschemas.Schema("profile")),
		map[string]interface{}{"name": userProfile.Name}, nil, rc)
	rc.Form = "preferences"
	preferencesContent := formEng.Render(ctx, resolveSchema(ctx, "system", "preferences", systemschemas.Schema("preferences")),
		map[string]interface{}{
			"locale":       userProfile.Locale,
			"timezone":     userProfile.Timezone,
			"dark_mode":    userProfile.DarkMode,
			"email_alerts": userProfile.EmailAlerts,
			"compact_view": userProfile.CompactView,
		}, nil, rc)

	data := ProfileData{
		User:                   userProfile,
		SavedSection:           savedSection,
		CSRFToken:              csrfToken,
		ProfileFormContent:     profileContent,
		PreferencesFormContent: preferencesContent,
	}

	h.page.Render(w, r, t("profile.page_title"), "/profile", ProfilePage(data, t))
}

// ProfileSave handles POST /profile
func (h *UIHandler) ProfileSave(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, h.errText(r, "error.invalid_form_data", "Invalid form data"), http.StatusBadRequest)
		return
	}

	userID := h.getCurrentUserID(r)
	name := r.FormValue("name")

	if name == "" {
		http.Error(w, h.errText(r, "error.name_required", "Name is required"), http.StatusBadRequest)
		return
	}

	// Update user in xolu
	// Patch, not Update -- Update is xolu's HTTP PUT (full document
	// replace: confirmed directly, PUT with only {name, updated_at}
	// wipes every other field, including email and password_hash,
	// leaving the account unable to log in). Patch is HTTP PATCH (a
	// real partial merge, confirmed directly: PATCH with one field adds
	// it without touching anything else). Found 2026-08-03, reported by
	// The tech team after using the Settings/Profile forms on a real
	// deployment -- reproduced directly in this session too, on a test
	// user edited via AdminUserUpdate (same bug, same root cause,
	// documented on that call site below).
	_, err := h.xoluClient.Patch(ctx, "users", userID, map[string]interface{}{
		"name":       name,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		h.logger.Error("failed to update user profile", "error", err, "user_id", userID)
		// Try to create user if not exists
		_, err = h.xoluClient.Create(ctx, "users", map[string]interface{}{
			"id":           userID,
			"name":         name,
			"email":        "user@example.com",
			"role":         "admin",
			"locale":       "en",
			"timezone":     "UTC",
			"dark_mode":    "system",
			"email_alerts": true,
			"compact_view": false,
			"created_at":   time.Now().UTC().Format(time.RFC3339),
		})
		if err != nil {
			h.logger.Error("failed to create user", "error", err)
		}
	}

	http.Redirect(w, r, "/profile?saved=profile", http.StatusSeeOther)
}

// ProfilePreferencesSave handles POST /profile/preferences
func (h *UIHandler) ProfilePreferencesSave(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, h.errText(r, "error.invalid_form_data", "Invalid form data"), http.StatusBadRequest)
		return
	}

	userID := h.getCurrentUserID(r)

	locale := r.FormValue("locale")
	timezone := r.FormValue("timezone")
	darkMode := r.FormValue("dark_mode")
	emailAlerts := r.FormValue("email_alerts") == "on"
	compactView := r.FormValue("compact_view") == "on"

	// Validate locale against available locales
	if !h.ValidateLocale(locale) {
		locale = "en"
	}

	// Validate dark mode
	validDarkModes := map[string]bool{"system": true, "light": true, "dark": true}
	if !validDarkModes[darkMode] {
		darkMode = "system"
	}

	// Update user preferences in xolu
	// Patch, not Update -- see ProfileSave's identical comment above for
	// why (HTTP PUT full-replace vs HTTP PATCH real merge, confirmed
	// directly against the live server).
	_, err := h.xoluClient.Patch(ctx, "users", userID, map[string]interface{}{
		"locale":       locale,
		"timezone":     timezone,
		"dark_mode":    darkMode,
		"email_alerts": emailAlerts,
		"compact_view": compactView,
		"updated_at":   time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		h.logger.Error("failed to update user preferences", "error", err, "user_id", userID)
	} else {
		h.auditLogger.LogPreferencesChanged(ctx, r, userID)
	}

	// Also update the config so UI reflects changes immediately
	h.config.Locale = locale

	http.Redirect(w, r, "/profile?saved=preferences", http.StatusSeeOther)
}

// PasswordChange handles GET /profile/password
func (h *UIHandler) PasswordChange(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	t := h.translateFunc(r)
	errMsg := r.URL.Query().Get("error")
	success := r.URL.Query().Get("success") == "true"
	csrfToken := h.GetCSRFToken(w, r)

	// MOD-8: formengine-rendered, with a real Translator wired per
	// request -- see ProfileData's own doc comment for why. Never
	// passes any submitted password value back into values, even on a
	// validation error -- renderPassword (widgets.go) doesn't read it
	// either way, but the caller not offering one at all is the
	// clearer signal that re-populating a password field is never the
	// right behaviour.
	formEng := &fe.FormEngine{Translator: h.formEngineTranslator()}
	rc := fe.RenderContext{Locale: h.currentLocale(), CSRFToken: csrfToken}
	rc.Module, rc.Form = "system", "password"
	formContent := formEng.Render(ctx, resolveSchema(ctx, "system", "password", systemschemas.Schema("password")), nil, nil, rc)

	data := PasswordChangeData{
		Error:       errMsg,
		Success:     success,
		CSRFToken:   csrfToken,
		FormContent: formContent,
	}

	h.page.Render(w, r, t("profile.change_password"), "/profile", PasswordChangePage(data, t))
}

// PasswordChangeSave handles POST /profile/password
func (h *UIHandler) PasswordChangeSave(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, h.errText(r, "error.invalid_form_data", "Invalid form data"), http.StatusBadRequest)
		return
	}

	currentPassword := r.FormValue("current_password")
	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")

	// Validate inputs
	if currentPassword == "" || newPassword == "" || confirmPassword == "" {
		http.Redirect(w, r, "/profile/password?error=All+fields+are+required", http.StatusSeeOther)
		return
	}

	if len(newPassword) < 8 {
		http.Redirect(w, r, "/profile/password?error=Password+must+be+at+least+8+characters", http.StatusSeeOther)
		return
	}

	if newPassword != confirmPassword {
		http.Redirect(w, r, "/profile/password?error=Passwords+do+not+match", http.StatusSeeOther)
		return
	}

	// Change password via UserService
	if h.userService != nil {
		userID := h.getCurrentUserID(r)
		err := h.userService.ChangePassword(r.Context(), userID, currentPassword, newPassword)
		if err != nil {
			if err == auth.ErrInvalidPassword {
				http.Redirect(w, r, "/profile/password?error=Current+password+is+incorrect", http.StatusSeeOther)
				return
			}
			h.logger.Error("failed to change password", "error", err)
			http.Redirect(w, r, "/profile/password?error=Failed+to+change+password", http.StatusSeeOther)
			return
		}
	}

	http.Redirect(w, r, "/profile?saved=password", http.StatusSeeOther)
}

// =============================================================================
// ADMIN: USER MANAGEMENT HANDLERS
// =============================================================================

// AdminUsersList handles GET /admin/users
func (h *UIHandler) AdminUsersList(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	roleFilter := r.URL.Query().Get("role")

	// Build OQL query (uses TOP n, not LIMIT)
	query := "SELECT TOP 100 * FROM users ORDER BY name ASC"
	if roleFilter != "" {
		query = "SELECT TOP 100 * FROM users WHERE role = '" + escapeOQL(roleFilter) + "' ORDER BY name ASC"
	}

	result, err := h.xoluClient.OQL(ctx, query)
	if err != nil {
		h.logger.Error("failed to list users", "error", err)
	}

	users := make([]UserRow, 0)
	_, callerRole := callerIdentity(r)
	if result != nil {
		for _, row := range result.Data {
			// Developer accounts are invisible to everyone but developers.
			if !auth.CanManageUser(callerRole, toString(row["role"])) {
				continue
			}
			users = append(users, UserRow{
				ID:          toInt64(row["id"]),
				Email:       toString(row["email"]),
				Name:        toString(row["name"]),
				Role:        toString(row["role"]),
				Active:      toBool(row["active"]),
				LastLoginAt: formatTimeAgo(toString(row["last_login_at"]), t),
				CreatedAt:   toString(row["created_at"]),
			})
		}
	}

	// Check permissions for current user
	currentUser := h.getCurrentUser(r)
	canCreate := auth.HasPermission(currentUser.Role, auth.PermAdministrationUserCreate)
	canEdit := auth.HasPermission(currentUser.Role, auth.PermAdministrationUserEdit)
	canDelete := auth.HasPermission(currentUser.Role, auth.PermAdministrationUserDelete)

	data := UsersListData{
		Users:      users,
		Total:      len(users),
		RoleFilter: roleFilter,
		CanCreate:  canCreate,
		CanEdit:    canEdit,
		CanDelete:  canDelete,
	}

	h.page.Render(w, r, t("users.list_title"), "/admin/users", UsersListPage(data, t))
}

// AdminUserNew handles GET /admin/users/new
func (h *UIHandler) AdminUserNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	t := h.translateFunc(r)
	csrfToken := h.GetCSRFToken(w, r)

	// MOD-8: formengine-rendered, real Translator per request -- see
	// ProfileData's own doc comment for why.
	formEng := &fe.FormEngine{Translator: h.formEngineTranslator()}
	rc := fe.RenderContext{Locale: h.currentLocale(), CSRFToken: csrfToken, Module: "system", Form: "user"}
	formContent := formEng.Render(ctx, h.userFormSchema(ctx, r, "user_create"), nil, nil, rc)

	data := UserFormData{
		IsEdit:      false,
		User:        UserFormValues{Active: true},
		CSRFToken:   csrfToken,
		FormContent: formContent,
	}

	h.page.Render(w, r, t("users.new"), "/admin/users", UserFormPage(data, t))
}

// AdminUserCreate handles POST /admin/users
func (h *UIHandler) AdminUserCreate(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, h.errText(r, "error.invalid_form_data", "Invalid form data"), http.StatusBadRequest)
		return
	}

	email := r.FormValue("email")
	name := r.FormValue("name")
	role := r.FormValue("role")
	password := r.FormValue("password")
	passwordConfirm := r.FormValue("password_confirm")
	csrfToken := h.GetCSRFToken(w, r)

	// Shared by both re-render points below (client-side validation
	// failure and the server-side "email already exists" check) -- one
	// real Translator constructed once per request, not duplicated.
	// Password/password_confirm are deliberately never included in
	// values: renderPassword doesn't read them either way, but not
	// offering them is the clearer signal that re-populating a
	// password field after a failed submission is never correct.
	rerenderCreateForm := func(errs map[string]string) {
		formEng := &fe.FormEngine{Translator: h.formEngineTranslator()}
		rc := fe.RenderContext{Locale: h.currentLocale(), CSRFToken: csrfToken, Module: "system", Form: "user"}
		formContent := formEng.Render(ctx, h.userFormSchema(ctx, r, "user_create"),
			map[string]interface{}{"email": email, "name": name, "role": role}, errs, rc)
		data := UserFormData{
			IsEdit:      false,
			User:        UserFormValues{Email: email, Name: name, Role: role, Active: true},
			Errors:      errs,
			CSRFToken:   csrfToken,
			FormContent: formContent,
		}
		h.page.Render(w, r, t("users.new"), "/admin/users", UserFormPage(data, t))
	}

	// Validate
	errors := make(map[string]string)
	if email == "" {
		errors["email"] = t("users.error.email_required")
	}
	if name == "" {
		errors["name"] = t("users.error.name_required")
	}
	if role == "" {
		errors["role"] = t("users.error.role_required")
	} else if !auth.IsValidRole(role) || !callerCanAssign(r, role) {
		// Found 2026-08-04, adversarial testing: without this check, a
		// submitted role outside the four known values reached the
		// fallback path (h.userService == nil) with zero validation --
		// written straight to storage. The h.userService != nil path was
		// already protected by UserService.CreateUser's own
		// auth.IsValidRole check, but that path's error handling for
		// ErrInvalidRole fell through to a generic 500, not a real
		// validation error either. One check here now covers both paths
		// uniformly, before either is reached, matching
		// AdminUserUpdate's existing schema-enforced role enum
		// (UserUpdateSchema's own .Enum(...), which this handler never
		// had an equivalent of).
		errors["role"] = t("users.error.role_invalid")
	}
	if password == "" {
		errors["password"] = t("users.error.password_required")
	} else if len(password) < 8 {
		errors["password"] = t("users.error.password_min_length")
	}
	if password != passwordConfirm {
		errors["password_confirm"] = t("users.error.passwords_no_match")
	}

	if len(errors) > 0 {
		rerenderCreateForm(errors)
		return
	}

	// Create user via UserService (handles password hashing)
	if h.userService != nil {
		// Get tenant ID from authenticated user's session - REQUIRED
		claims, ok := auth.ClaimsFromContext(ctx)
		if !ok || claims.TenantID == 0 {
			h.logger.Error("cannot create user without valid session with tenant")
			http.Error(w, h.errText(r, "error.auth_required", "Authentication required"), http.StatusUnauthorized)
			return
		}
		tenantID := claims.TenantID

		newUser, err := h.userService.CreateUser(ctx, auth.CreateUserInput{
			TenantID: tenantID,
			Email:    email,
			Name:     name,
			Password: password,
			Role:     role,
		})
		if err != nil {
			h.logger.Error("failed to create user", "error", err)
			if err == auth.ErrUserExists {
				errors["email"] = t("users.error.email_exists")
				// Fixed 2026-08-03, found while rewriting this handler for
				// MOD-8: this specific re-render path never set CSRFToken at
				// all (both other re-render points did) -- resubmitting the
				// re-rendered form would always have failed CSRF validation.
				// rerenderCreateForm always sets it now, consistently.
				rerenderCreateForm(errors)
				return
			}
			http.Error(w, h.errText(r, "error.save_failed", "Failed to create user"), http.StatusInternalServerError)
			return
		}
		h.auditLogger.LogUserCreated(ctx, r, claims.UserID, newUser.ID, email, role)
	} else {
		// Fallback: Create directly in xolu (no password hashing)
		claims, ok := auth.ClaimsFromContext(ctx)
		if !ok {
			h.logger.Error("cannot create user without valid session")
			http.Error(w, h.errText(r, "error.auth_required", "Authentication required"), http.StatusUnauthorized)
			return
		}
		userData := map[string]interface{}{
			"email":      email,
			"name":       name,
			"role":       role,
			"active":     true,
			"created_at": time.Now().UTC().Format(time.RFC3339),
		}

		entity, err := h.xoluClient.Create(ctx, "users", userData)
		if err != nil {
			h.logger.Error("failed to create user", "error", err)
			http.Error(w, h.errText(r, "error.save_failed", "Failed to create user"), http.StatusInternalServerError)
			return
		}
		h.auditLogger.LogUserCreated(ctx, r, claims.UserID, entity.ID, email, role)
	}

	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

// AdminUserEdit handles GET /admin/users/{id}
func (h *UIHandler) AdminUserEdit(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, h.errText(r, "error.invalid_id", "Invalid user ID"), http.StatusBadRequest)
		return
	}

	user, err := h.xoluClient.Get(ctx, "users", id)
	if err != nil {
		http.Error(w, h.errText(r, "users.error.not_found", "User not found"), http.StatusNotFound)
		return
	}

	role := toString(user.Data["role"])
	if h.denyManagingUser(w, r, role) {
		return
	}
	name := toString(user.Data["name"])
	active := toBool(user.Data["active"])
	csrfToken := h.GetCSRFToken(w, r)

	// MOD-8: formengine-rendered, real Translator per request. Email
	// isn't part of user_edit.json's schema at all (stays hand-coded,
	// disabled, rendered by UserFormPage itself via
	// userEmailFieldIfEdit) -- only name/role/active go through
	// formengine here.
	formEng := &fe.FormEngine{Translator: h.formEngineTranslator()}
	rc := fe.RenderContext{Locale: h.currentLocale(), CSRFToken: csrfToken, Module: "system", Form: "user"}
	formContent := formEng.Render(ctx, h.userFormSchema(ctx, r, "user_edit"),
		map[string]interface{}{"name": name, "role": role, "active": active}, nil, rc)

	data := UserFormData{
		IsEdit:    true,
		CSRFToken: csrfToken,
		User: UserFormValues{
			ID:     user.ID,
			Email:  toString(user.Data["email"]),
			Name:   name,
			Role:   role,
			Active: active,
		},
		FormContent: formContent,
	}

	h.page.Render(w, r, t("users.edit_title"), "/admin/users", UserFormPage(data, t))
}

// AdminUserUpdate handles POST /admin/users/{id}
func (h *UIHandler) AdminUserUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, h.errText(r, "error.invalid_id", "Invalid user ID"), http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, h.errText(r, "error.invalid_form_data", "Invalid form data"), http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	role := r.FormValue("role")
	active := r.FormValue("active") == "on"

	// Fetch the existing record first -- needed to detect a role change
	// (LogRoleChanged needs the old role, not just the new one) and to
	// build LogUserUpdated's changes map. Not fetched before this pass
	// added the audit calls that need it.
	existing, err := h.xoluClient.Get(ctx, "users", id)
	if err != nil {
		http.Error(w, h.errText(r, "users.error.not_found", "User not found"), http.StatusNotFound)
		return
	}
	oldRole := toString(existing.Data["role"])
	targetEmail := toString(existing.Data["email"])
	if h.denyManagingUser(w, r, oldRole) {
		return
	}
	// Only developers may grant the developer role. Other invalid roles
	// keep falling through to the validation below (422), unchanged.
	if role == auth.RoleDeveloper && role != oldRole && !callerCanAssign(r, role) {
		h.Error403(w, r)
		return
	}
	// The role select offers Developer to a caller who may assign it
	// (userFormSchema), so the role submitted is the one chosen. The one thing
	// refused is a developer removing their own developer role: with nobody
	// else to restore it that could leave the module with no developer at all.
	if callerID, _ := callerIdentity(r); h.config.flagOn(features.DeveloperProtectOwnRole) && callerID == id && oldRole == auth.RoleDeveloper && role != auth.RoleDeveloper {
		http.Error(w, h.errText(r, "users.error.own_developer_role", "A developer cannot remove their own developer role"), http.StatusUnprocessableEntity)
		return
	}

	// Update user
	userData := map[string]interface{}{
		"name":       name,
		"role":       role,
		"active":     active,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	}

	if errs := validateForm(map[string]any{"name": name, "role": role}, schemaUserUpdate); errs != nil {
		http.Error(w, firstError(errs), http.StatusUnprocessableEntity)
		return
	}
	// Patch, not Update -- HTTP PUT full-replace vs HTTP PATCH real
	// merge, confirmed directly against the live server. This exact
	// call, with this exact partial userData (name/role/active only),
	// is what reproduced the tech team's report: saving the edit-user form
	// wiped email and password_hash off the target account, leaving it
	// unable to log in. Root-caused and fixed 2026-08-03; see
	// ProfileSave/ProfilePreferencesSave for the other two call sites
	// with the identical bug.
	_, err = h.xoluClient.Patch(ctx, "users", id, userData)
	if err != nil {
		h.logger.Error("failed to update user", "error", err, "user_id", id)
		http.Error(w, h.errText(r, "error.save_failed", "Failed to update user"), http.StatusInternalServerError)
		return
	}

	if claims, ok := auth.ClaimsFromContext(ctx); ok {
		h.auditLogger.LogUserUpdated(ctx, r, claims.UserID, id, targetEmail, map[string]any{
			"name": name, "role": role, "active": active,
		})
		if role != oldRole {
			h.auditLogger.LogRoleChanged(ctx, r, claims.UserID, id, targetEmail, oldRole, role)
		}
	}

	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

// AdminUserPasswordReset handles GET /admin/users/{id}/password
func (h *UIHandler) AdminUserPasswordReset(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, h.errText(r, "error.invalid_id", "Invalid user ID"), http.StatusBadRequest)
		return
	}

	user, err := h.xoluClient.Get(ctx, "users", id)
	if err != nil {
		http.Error(w, h.errText(r, "users.error.not_found", "User not found"), http.StatusNotFound)
		return
	}

	errMsg := r.URL.Query().Get("error")
	success := r.URL.Query().Get("success") == "true"

	if h.denyManagingUser(w, r, toString(user.Data["role"])) {
		return
	}

	data := AdminPasswordResetData{
		UserID:    id,
		UserName:  toString(user.Data["name"]),
		Error:     errMsg,
		Success:   success,
		CSRFToken: h.GetCSRFToken(w, r),
	}

	h.page.Render(w, r, t("users.reset_password"), "/admin/users", AdminPasswordResetPage(data, t))
}

// AdminUserPasswordResetSave handles POST /admin/users/{id}/password
func (h *UIHandler) AdminUserPasswordResetSave(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, h.errText(r, "error.invalid_id", "Invalid user ID"), http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, h.errText(r, "error.invalid_form_data", "Invalid form data"), http.StatusBadRequest)
		return
	}

	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")

	// Developer accounts may only be reset by developers. The target is read
	// here because everything below acts on the bare ID; if it cannot be
	// read, refuse rather than guess its role.
	if target, gerr := h.xoluClient.Get(ctx, "users", id); gerr != nil {
		http.Error(w, h.errText(r, "users.error.not_found", "User not found"), http.StatusNotFound)
		return
	} else if h.denyManagingUser(w, r, toString(target.Data["role"])) {
		return
	}

	if len(newPassword) < 8 {
		http.Redirect(w, r, "/admin/users/"+idStr+"/password?error=Password+must+be+at+least+8+characters", http.StatusSeeOther)
		return
	}

	if newPassword != confirmPassword {
		http.Redirect(w, r, "/admin/users/"+idStr+"/password?error=Passwords+do+not+match", http.StatusSeeOther)
		return
	}

	// Reset password via UserService (admin reset - no old password required)
	if h.userService != nil {
		err := h.userService.SetPassword(ctx, id, newPassword)
		if err != nil {
			h.logger.Error("failed to reset password", "error", err, "user_id", id)
			http.Redirect(w, r, "/admin/users/"+idStr+"/password?error=Failed+to+reset+password", http.StatusSeeOther)
			return
		}
		targetEmail := ""
		if target, gerr := h.xoluClient.Get(ctx, "users", id); gerr == nil {
			targetEmail = toString(target.Data["email"])
		}
		h.auditLogger.LogPasswordChanged(ctx, r, id, targetEmail, true)
	}

	http.Redirect(w, r, "/admin/users/"+idStr+"/password?success=true", http.StatusSeeOther)
}

// formatTimeAgo formats a time string as relative time.
func formatTimeAgo(ts string, t func(key string, args ...any) string) string {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	if ts == "" {
		return t("users.time.never")
	}
	parsed, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}

	diff := time.Since(parsed)
	switch {
	case diff < time.Minute:
		return t("users.time.just_now")
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins == 1 {
			return t("users.time.minute_ago")
		}
		return t("users.time.minutes_ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return t("users.time.hour_ago")
		}
		return t("users.time.hours_ago", hours)
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		if days == 1 {
			return t("users.time.yesterday")
		}
		return t("users.time.days_ago", days)
	default:
		return formatDay(t, parsed)
	}
}

// getCurrentUserID returns the current user's ID.
// In production, this would come from the auth middleware/session.
func (h *UIHandler) getCurrentUserID(r *http.Request) int64 {
	// Try to get from claims first (production path)
	if claims, ok := auth.ClaimsFromContext(r.Context()); ok {
		return claims.UserID
	}

	// Development fallback - ONLY if DevMode is enabled
	if h.config.DevMode {
		if cookie, err := r.Cookie("seam_user_id"); err == nil {
			if id, err := strconv.ParseInt(cookie.Value, 10, 64); err == nil {
				return id
			}
		}
		// Dev mode fallback to user 1
		return 1
	}

	// Production: no valid session means no user ID
	return 0
}

// getCurrentUser returns the current user from context or session.
func (h *UIHandler) getCurrentUser(r *http.Request) *auth.User {
	// Try to get from context first (set by auth middleware)
	if user, ok := auth.UserFromContext(r.Context()); ok {
		return user
	}

	// Try to get from claims in context (production path)
	if claims, ok := auth.ClaimsFromContext(r.Context()); ok {
		return &auth.User{
			ID:       claims.UserID,
			TenantID: claims.TenantID,
			Email:    claims.Email,
			Role:     claims.Role,
			Active:   true,
		}
	}

	// Development fallback - ONLY if DevMode is enabled
	if h.config.DevMode {
		userID := h.getCurrentUserID(r)
		if h.xoluClient != nil && userID > 0 {
			entity, err := h.xoluClient.Get(r.Context(), "users", userID)
			if err == nil {
				return &auth.User{
					ID:       entity.ID,
					TenantID: toInt64(entity.Data["tenant_id"]),
					Email:    toString(entity.Data["email"]),
					Name:     toString(entity.Data["name"]),
					Role:     toString(entity.Data["role"]),
					Active:   toBool(entity.Data["active"]),
				}
			}
		}
		// Dev mode fallback to admin
		return &auth.User{
			ID:     1,
			Role:   auth.RoleAdmin,
			Active: true,
		}
	}

	// Production: no valid session means no user
	return nil
}

// =============================================================================
// AUTHENTICATION HANDLERS
// =============================================================================

// LoginPage handles GET /login
func (h *UIHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	// If already logged in, redirect to home
	if h.sessionManager != nil {
		if _, err := h.sessionManager.ValidateSession(r); err == nil {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
	}

	t := h.translateFunc(r)
	data := LoginPageData{
		Error:     r.URL.Query().Get("error"),
		Email:     r.URL.Query().Get("email"),
		NextURL:   r.URL.Query().Get("next"),
		AppName:   h.config.AppName,
		Version:   h.config.Version,
		StaticURL: h.config.StaticURL,
		CSRFToken: h.GetCSRFToken(w, r),
		Locale:    h.currentLocale(),
	}

	RenderLoginPage(w, data, t)
}

// LoginSubmit handles POST /login
func (h *UIHandler) LoginSubmit(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/login?error="+urlEncode(t("error.invalid_form_data")), http.StatusSeeOther)
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")
	nextURL := r.FormValue("next")

	if nextURL == "" {
		nextURL = "/"
	}

	// Validate input
	if email == "" || password == "" {
		http.Redirect(w, r, "/login?error="+urlEncode(t("login.error.email_password_required"))+"&email="+urlEncode(email), http.StatusSeeOther)
		return
	}

	// Check rate limiting before attempting authentication
	if h.loginLimiter != nil {
		allowed, _, lockMsg := h.loginLimiter.CheckAllowed(email)
		if !allowed {
			h.logger.Warn("login blocked by rate limiter", "email", email)
			// Audit log: login locked
			if h.auditLogger != nil {
				h.auditLogger.LogLoginLocked(r.Context(), r, email, 0)
			}
			http.Redirect(w, r, "/login?error="+urlEncode(lockMsg)+"&email="+urlEncode(email), http.StatusSeeOther)
			return
		}
	}

	// Authenticate user
	if h.userService == nil {
		h.logger.Error("login attempt with no user service configured")
		http.Redirect(w, r, "/login?error="+urlEncode(t("login.error.not_configured"))+"&email="+urlEncode(email), http.StatusSeeOther)
		return
	}

	user, err := h.userService.Authenticate(r.Context(), email, password)
	if err != nil {
		// Audit log: login failed
		if h.auditLogger != nil {
			h.auditLogger.LogLogin(r.Context(), r, 0, email, false, "Invalid credentials")
		}

		// Record failed attempt
		if h.loginLimiter != nil {
			locked, duration := h.loginLimiter.RecordFailure(email)
			if locked {
				h.logger.Warn("account locked due to failed attempts", "email", email, "duration", duration)
				if h.auditLogger != nil {
					h.auditLogger.LogLoginLocked(r.Context(), r, email, duration)
				}
				http.Redirect(w, r, "/login?error="+urlEncode(t("login.error.too_many_attempts", duration.String()))+"&email="+urlEncode(email), http.StatusSeeOther)
				return
			}
			remaining := h.loginLimiter.RemainingAttempts(email)
			h.logger.Warn("login failed", "email", email, "error", err, "remaining_attempts", remaining)
		} else {
			h.logger.Warn("login failed", "email", email, "error", err)
		}
		http.Redirect(w, r, "/login?error="+urlEncode(t("login.error.invalid_credentials"))+"&email="+urlEncode(email), http.StatusSeeOther)
		return
	}

	// Check if account is active
	if !user.Active {
		h.logger.Warn("login attempt for disabled account", "email", email)
		// Audit log: login failed (disabled account)
		if h.auditLogger != nil {
			h.auditLogger.LogLogin(r.Context(), r, user.ID, email, false, "Account disabled")
		}
		http.Redirect(w, r, "/login?error="+urlEncode(t("login.error.account_disabled"))+"&email="+urlEncode(email), http.StatusSeeOther)
		return
	}

	// Clear failed attempts on successful authentication
	if h.loginLimiter != nil {
		h.loginLimiter.RecordSuccess(email)
	}

	// Create session
	if h.sessionManager != nil {
		if err := h.sessionManager.CreateSession(w, user); err != nil {
			h.logger.Error("failed to create session", "error", err)
			http.Redirect(w, r, "/login?error="+urlEncode(t("login.error.session_failed"))+"&email="+urlEncode(email), http.StatusSeeOther)
			return
		}
	}

	// Update last login time
	if h.userService != nil {
		_ = h.userService.UpdateLastLogin(r.Context(), user.ID)
	}

	// Audit log: login success
	if h.auditLogger != nil {
		h.auditLogger.LogLogin(r.Context(), r, user.ID, user.Email, true, "")
	}

	h.logger.Info("user logged in", "user_id", user.ID, "email", user.Email)
	http.Redirect(w, r, nextURL, http.StatusSeeOther)
}

// Logout handles GET /logout
func (h *UIHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Get user info for logging before clearing session
	var userID int64
	var email string
	if h.sessionManager != nil {
		if session, err := h.sessionManager.ValidateSession(r); err == nil {
			userID = session.UserID
			email = session.Email
		}
		h.sessionManager.ClearSession(w)
	}

	if userID > 0 {
		// Audit log: logout
		if h.auditLogger != nil {
			h.auditLogger.LogLogout(r.Context(), r, userID, email)
		}
		h.logger.Info("user logged out", "user_id", userID)
	}

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// ForgotPasswordPage handles GET /forgot-password
func (h *UIHandler) ForgotPasswordPage(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	data := ForgotPasswordData{
		Email:     r.URL.Query().Get("email"),
		CSRFToken: h.GetCSRFToken(w, r),
		Locale:    h.currentLocale(),
	}
	RenderForgotPasswordPage(w, data, t)
}

// ForgotPasswordSubmit handles POST /forgot-password
func (h *UIHandler) ForgotPasswordSubmit(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	if err := r.ParseForm(); err != nil {
		RenderForgotPasswordPage(w, ForgotPasswordData{Error: t("error.invalid_form_data"), Locale: h.currentLocale()}, t)
		return
	}

	email := r.FormValue("email")
	if email == "" {
		RenderForgotPasswordPage(w, ForgotPasswordData{Error: t("users.error.email_required"), Locale: h.currentLocale()}, t)
		return
	}

	// Generate reset token if user exists
	if h.resetManager != nil && h.userService != nil {
		user, err := h.userService.GetUserByEmail(r.Context(), email)
		if err == nil && user != nil && user.Active {
			token, err := h.resetManager.GenerateToken(user.ID, email)
			if err == nil {
				// In production, send email here
				// For now, log the token (development only)
				h.logger.Info("password reset requested",
					"email", email,
					"user_id", user.ID,
					"reset_url", "/reset-password?token="+token,
				)

				// Audit log
				if h.auditLogger != nil {
					h.auditLogger.Log(r.Context(), &auth.AuditEvent{
						EventType: auth.AuditPasswordResetReq,
						UserID:    user.ID,
						Email:     email,
						IPAddress: getClientIP(r),
						Success:   true,
						Details:   "Password reset requested",
					})
				}
			}
		}
	}

	// Always show success to prevent email enumeration
	RenderForgotPasswordPage(w, ForgotPasswordData{
		Email:   email,
		Success: true,
		Locale:  h.currentLocale(),
	}, t)
}

// ResetPasswordPage handles GET /reset-password
func (h *UIHandler) ResetPasswordPage(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	locale := h.currentLocale()
	token := r.URL.Query().Get("token")
	if token == "" {
		RenderResetPasswordPage(w, ResetPasswordData{Invalid: true, Locale: locale}, t)
		return
	}

	// Validate token
	if h.resetManager == nil {
		RenderResetPasswordPage(w, ResetPasswordData{Invalid: true, Locale: locale}, t)
		return
	}

	resetToken, err := h.resetManager.ValidateToken(token)
	if err != nil {
		RenderResetPasswordPage(w, ResetPasswordData{Invalid: true, Locale: locale}, t)
		return
	}

	RenderResetPasswordPage(w, ResetPasswordData{
		Token:     token,
		Email:     resetToken.Email,
		CSRFToken: h.GetCSRFToken(w, r),
		Locale:    locale,
	}, t)
}

// ResetPasswordSubmit handles POST /reset-password
func (h *UIHandler) ResetPasswordSubmit(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	locale := h.currentLocale()
	if err := r.ParseForm(); err != nil {
		RenderResetPasswordPage(w, ResetPasswordData{Error: t("error.invalid_form_data"), Locale: locale}, t)
		return
	}

	token := r.FormValue("token")
	password := r.FormValue("password")
	confirmPassword := r.FormValue("confirm_password")

	if token == "" {
		RenderResetPasswordPage(w, ResetPasswordData{Invalid: true, Locale: locale}, t)
		return
	}

	// Validate token first
	if h.resetManager == nil {
		RenderResetPasswordPage(w, ResetPasswordData{Invalid: true, Locale: locale}, t)
		return
	}

	resetToken, err := h.resetManager.ValidateToken(token)
	if err != nil {
		RenderResetPasswordPage(w, ResetPasswordData{Invalid: true, Locale: locale}, t)
		return
	}

	// Validate passwords
	if password == "" {
		RenderResetPasswordPage(w, ResetPasswordData{
			Token:  token,
			Email:  resetToken.Email,
			Error:  t("users.error.password_required"),
			Locale: locale,
		}, t)
		return
	}

	if len(password) < 8 {
		RenderResetPasswordPage(w, ResetPasswordData{
			Token:  token,
			Email:  resetToken.Email,
			Error:  t("users.error.password_min_length"),
			Locale: locale,
		}, t)
		return
	}

	if password != confirmPassword {
		RenderResetPasswordPage(w, ResetPasswordData{
			Token:  token,
			Email:  resetToken.Email,
			Error:  t("users.error.passwords_no_match"),
			Locale: locale,
		}, t)
		return
	}

	// Reset password
	if h.userService != nil {
		if err := h.userService.SetPassword(r.Context(), resetToken.UserID, password); err != nil {
			h.logger.Error("failed to set password", "error", err)
			RenderResetPasswordPage(w, ResetPasswordData{
				Token:  token,
				Email:  resetToken.Email,
				Error:  t("password_reset.reset_failed"),
				Locale: locale,
			}, t)
			return
		}
	}

	// Mark token as used
	_ = h.resetManager.UseToken(token)

	// Audit log
	if h.auditLogger != nil {
		h.auditLogger.Log(r.Context(), &auth.AuditEvent{
			EventType: auth.AuditPasswordReset,
			UserID:    resetToken.UserID,
			Email:     resetToken.Email,
			IPAddress: getClientIP(r),
			Success:   true,
			Details:   "Password reset completed",
		})
	}

	h.logger.Info("password reset completed", "email", resetToken.Email, "user_id", resetToken.UserID)

	RenderResetPasswordPage(w, ResetPasswordData{
		Success: true,
		Locale:  locale,
	}, t)
}

// getClientIP extracts the client IP from the request.
func getClientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	addr := r.RemoteAddr
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}

// Error403 renders a 403 Forbidden error page.
func (h *UIHandler) Error403(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	h.page.RenderError(w, 403, t("error.forbidden"), t("error.access_denied_message"))
}

// ErrorCSRF renders a CSRF validation error page.
func (h *UIHandler) ErrorCSRF(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	h.page.RenderError(w, 403, t("error.security_error_title"), t("error.session_expired_message"))
}

// GetCSRFToken returns the CSRF token for the current request.
// The token should be included in forms as a hidden field.
// formEngineTranslator returns h.translator as a formengine.Translator,
// or a genuinely nil interface if h.translator is nil -- never assign
// h.translator directly to a formengine.Translator field. h.translator
// is a *i18n.Translator; assigning a nil *i18n.Translator straight into
// an interface field produces a "typed nil" (a non-nil interface
// wrapping a nil pointer), so formengine's own e.Translator == nil
// checks silently evaluate false and the next call into it panics on
// the nil receiver. Caught by a real test crash (SIGSEGV), not by
// static review -- go vet doesn't catch this pattern.
func (h *UIHandler) formEngineTranslator() fe.Translator {
	if h.translator == nil {
		return nil
	}
	return h.translator
}

// dashboardEngineTranslator mirrors formEngineTranslator's own reasoning
// exactly -- dashboardengine.Translator has the identical shape, and the
// same typed-nil panic risk applies to it verbatim. See
// formEngineTranslator's own comment for the full explanation; not
// repeated here since it would just be a copy.
func (h *UIHandler) dashboardEngineTranslator() dashboardengine.Translator {
	if h.translator == nil {
		return nil
	}
	return h.translator
}

// errText is the text of an error reply (http.Error) in the viewer's language:
// the translation of key, or fallback, in English, when there is no translator
// or the key is missing. The replies that name a program fault for a developer
// (an unknown picker, a schema that does not parse) stay as they are.
func (h *UIHandler) errText(r *http.Request, key, fallback string) string {
	if s := h.translateFunc(r)(key); s != "" && s != key {
		return s
	}
	return fallback
}

// translateFunc returns a translate function bound to the current
// locale, for chrome text outside formengine's own rendering (e.g.
// wrapAssetDynamicForm's buttons) -- nil-safe, matching
// formEngineTranslator's own reasoning: h.translator can be nil (see
// that method's comment for why assigning it into an interface field
// directly, rather than through a nil check like this one, produces a
// typed-nil panic), and *i18n.Translator.T/.ForLocale are not
// nil-receiver-safe themselves (confirmed by reading their source: T
// takes t.mu.RLock() unconditionally, which panics on a nil t). When
// h.translator is nil, returns a function that echoes the key back
// unchanged -- the same graceful degradation i18n.Translator.T itself
// falls back to for an unresolved key, so chrome text is never blank
// or panics, just untranslated.
//
// With the Developer module on, a request that carries a tenant is translated
// through that tenant's view, so the module's per-tenant switches (filesystem
// and xolu overrides, show-keys) and tenant overrides apply to the page body
// (DEV-31). With the module off, or without a tenant, it is the global lookup,
// exactly as before. The nav and layout chrome still use the global lookup.
func (h *UIHandler) translateFunc(r *http.Request) func(key string, args ...any) string {
	if h.translator == nil {
		return func(key string, args ...any) string { return key }
	}
	if r != nil && h.config.DevRuntime != nil {
		if claims, ok := auth.ClaimsFromContext(r.Context()); ok && claims.TenantID > 0 {
			return h.translator.ForTenant(claims.TenantID).ForLocale(h.currentLocale())
		}
	}
	return h.translator.ForLocale(h.currentLocale())
}

// currentLocale returns the tenant's current UI locale
// (h.config.Locale) under its guarding lock -- the same value
// Settings/Profile's language selector reads and writes, and
// SEAM_FORCE_LANGUAGE seeds at startup before Settings has ever been
// saved. Small on purpose: every RenderContext-constructing handler
// needs this exact read, and the lock/unlock boilerplate is
// error-prone to repeat by hand (T-17 gap 2's fix).
func (h *UIHandler) currentLocale() string {
	h.configMu.RLock()
	defer h.configMu.RUnlock()
	return h.config.Locale
}

func (h *UIHandler) GetCSRFToken(w http.ResponseWriter, r *http.Request) string {
	if h.csrfManager == nil {
		return ""
	}
	token, _ := h.csrfManager.GetToken(w, r)
	return token
}

// loadPersistedSettings loads tenant_settings from xolu into both the
// UIHandler config and the PageHandler config so that top-bar rendering
// picks up the saved values without requiring a config-file restart.
// Safe to call from the background refresh goroutine.
func (h *UIHandler) loadPersistedSettings(ctx context.Context) {
	if h.xoluClient == nil {
		return
	}
	result, err := h.xoluClient.OQL(ctx, "SELECT TOP 1 * FROM tenant_settings WHERE key = 'general'")
	if err != nil || result == nil || len(result.Data) == 0 {
		return
	}
	row := result.Data[0]

	h.configMu.Lock()
	defer h.configMu.Unlock()

	if name := toString(row["tenant_name"]); name != "" {
		h.config.TenantName = name
		h.page.Config.TenantName = name
	}
	if locale := toString(row["locale"]); locale != "" {
		h.config.Locale = locale
		h.page.Config.Locale = locale
	}

	// Load feature flags
	featResult, err := h.xoluClient.OQL(ctx, "SELECT TOP 1 * FROM tenant_settings WHERE key = 'features'")
	if err == nil && featResult != nil && len(featResult.Data) > 0 {
		feat := featResult.Data[0]
		if v, ok := feat["map_enabled"].(bool); ok {
			h.config.EnableMap = v
			h.page.Config.EnableMap = v
		}
	}
}

// startSettingsRefresh starts a background goroutine that re-loads
// tenant_settings from xolu every 30 seconds. This ensures the tenant name
// appears in the top bar even when the seed runs after the server started,
// without requiring a restart.
func (h *UIHandler) startSettingsRefresh() {
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for range t.C {
			h.loadPersistedSettings(context.Background())
		}
	}()
}

// urlEncode encodes a string for use in a URL query parameter value.
// Was a hand-rolled encoder covering only a fixed set of ASCII
// characters (space/&/=/?//:), passing every other byte through
// unchanged, including all non-ASCII ones (T-31, found 2026-07-xx,
// fixed 2026-08-05) -- meaning every translated error message
// redirected through here in Japanese, or Spanish/Portuguese with any
// accented character, carried raw, un-encoded UTF-8 bytes straight
// into a URL query string on every login-error redirect, one of the
// most frequently hit paths in three of this project's four supported
// locales. url.QueryEscape is Go's own correct, complete
// implementation -- including encoding space as '+', matching this
// function's own previous behavior for that one case, so every
// existing call site's expectations are preserved exactly.
func urlEncode(s string) string {
	return url.QueryEscape(s)
}

func toIntFromString(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}

// seamAPIClient is the single, named point of contact for reading runtime
// operational state from seam-api (formerly shelf-api) -- deliberately not
// a bare http.Get call inline in a page handler. If seam-api itself ever
// needs the same per-account instance resolution xolu is moving toward
// (see docs/PROVISIONING_PLAN_2026-08-05.md), this is the one place that
// would need to change, not something to go find scattered across the
// codebase. Not a resolver itself -- no caching, no multi-instance
// awareness, no retry -- deliberately minimal until any of that is
// actually needed; building it now would be guessing at a shape nolu
// hasn't settled yet, the same mistake already caught once in this
// session's own planning (docs/PROVISIONING_PLAN_2026-08-05.md's
// correction).
//
// The one concrete improvement over the function this replaces: a real
// timeout. The bare http.Get it used had none -- a slow or hung seam-api
// would have hung every asset detail page load along with it.
type seamAPIClient struct {
	httpClient *http.Client
}

// defaultSeamAPIClient is the single shared instance -- a package-level
// var rather than a UIHandler field, deliberately: adding a field means
// touching NewUIHandler's constructor and every test that calls it, for
// a client with no state worth threading through per-handler-instance
// (no per-tenant config, nothing SeamAPIURL itself doesn't already
// carry as a plain parameter). Revisit if that stops being true.
var defaultSeamAPIClient = &seamAPIClient{
	httpClient: &http.Client{Timeout: 5 * time.Second},
}

// FetchRuntimeState retrieves the current FSM state for an asset
// identified by its GIAI, from the given seam-api base URL. Returns the
// state string and true on success, empty string and false on any
// error (network failure, asset not found, timeout, etc.).
func (c *seamAPIClient) FetchRuntimeState(seamAPIURL, giai string) (string, bool) {
	url := fmt.Sprintf("%s/api/v1/assets/%s", seamAPIURL, giai)
	resp, err := c.httpClient.Get(url) //nolint:gosec // URL is operator-configured
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false
	}
	var result struct {
		FSMState string `json:"fsm_state"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", false
	}
	return result.FSMState, result.FSMState != ""
}

// fetchRuntimeState is the existing call sites' own entry point,
// unchanged in shape (same signature, same two-value return) -- only
// its own implementation now delegates to seamAPIClient instead of
// calling http.Get directly. Errors are intentionally silent -- the UI
// degrades gracefully when seam-api is absent (standalone mode or
// SEAM_API_URL unset) or unreachable.
func fetchRuntimeState(seamAPIURL, giai string) (string, bool) {
	return defaultSeamAPIClient.FetchRuntimeState(seamAPIURL, giai)
}
