package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ha1tch/seam-ui/internal/auth"
	fe "github.com/ha1tch/seam-ui/internal/formengine"
	demoschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/demo"
)

// =============================================================================
// DEMO RECORDS (the "Repeatables" tab's Save button)
// =============================================================================
// The demo can persist what the repeatable form posts, to show that nested
// values (blocks -> lines) survive a round trip through fe.ParseIndexedForm.
// It is kept apart from real data on purpose:
//
//   - one dedicated xolu entity, demoEntity ("demo_templates"). No other
//     module reads or writes it, and the demo never touches any other entity
//     (demo_templates_test.go asserts that on every request the handlers make).
//   - every record carries demo=true and its tenant_id; listing, deleting and
//     resetting are filtered by the caller's tenant, and delete/reset refuse
//     any record that is not marked demo=true.
//   - the Demo module itself is opt-in (EnableDemo).
//
// Record shape: {tenant_id, demo: true, name, blocks: [{name, responsible_id,
// lines: [{name, type}]}], workflow_states: [{name}], created_by, created_at}.

const (
	demoEntity    = "demo_templates"
	demoMaxListed = 20
	demoMaxReset  = 200
)

// DemoTemplate is one saved demo record, ready to display.
type DemoTemplate struct {
	ID        int64
	Name      string
	CreatedAt string
	Data      string // the structured part (blocks, workflow_states) as indented JSON
}

// demoTenant returns the caller's tenant id; ok is false without session
// claims or with a tenant id <= 0 (the demo never falls back to "no tenant").
func demoTenant(ctx context.Context) (tenantID int64, userID int64, ok bool) {
	claims, has := auth.ClaimsFromContext(ctx)
	if !has || claims == nil || claims.TenantID <= 0 {
		return 0, 0, false
	}
	return claims.TenantID, claims.UserID, true
}

func demoTemplateFromRow(row map[string]any) DemoTemplate {
	blocks, states := row["blocks"], row["workflow_states"]
	if blocks == nil {
		blocks = []any{}
	}
	if states == nil {
		states = []any{}
	}
	data := ""
	if out, err := json.MarshalIndent(map[string]any{"blocks": blocks, "workflow_states": states}, "", "  "); err == nil {
		data = string(out)
	}
	return DemoTemplate{
		ID:        toInt64(row["id"]),
		Name:      toString(row["name"]),
		CreatedAt: toString(row["created_at"]),
		Data:      data,
	}
}

// demoRows returns the caller's tenant's demo records, newest first, up to
// limit. A missing entity (nothing ever saved) is an empty list, not an error.
func (h *UIHandler) demoRows(ctx context.Context, tenantID int64, limit int) ([]map[string]any, error) {
	q := fmt.Sprintf("SELECT TOP %d * FROM %s WHERE tenant_id = %d ORDER BY id DESC", limit, demoEntity, tenantID)
	res, err := h.xoluClient.OQL(ctx, q)
	if err != nil {
		if entityNotYetCreated(err) {
			return nil, nil
		}
		return nil, err
	}
	return res.Data, nil
}

// demoSavedTemplates lists the tenant's saved demo records for the page. A
// failure is logged and shown as "nothing saved": the demo page must still load.
func (h *UIHandler) demoSavedTemplates(ctx context.Context, tenantID int64) []DemoTemplate {
	rows, err := h.demoRows(ctx, tenantID, demoMaxListed)
	if err != nil {
		h.logger.Error("demo: listing demo records failed", "error", err)
		return nil
	}
	out := make([]DemoTemplate, 0, len(rows))
	for _, row := range rows {
		out = append(out, demoTemplateFromRow(row))
	}
	return out
}

// DemoTemplateSave handles POST /demo/templates: it rebuilds the nested
// structure the repeatable form posted and stores it as one demo record.
func (h *UIHandler) DemoTemplateSave(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Error(w, h.errText(r, "error.invalid_form_data", "bad form"), http.StatusBadRequest)
		return
	}
	tenantID, userID, ok := demoTenant(ctx)
	if !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	name := strings.TrimSpace(r.PostForm.Get("name"))
	if name == "" {
		h.page.RenderError(w, http.StatusBadRequest, t("error.generic"), t("demo.save.name_required"))
		return
	}

	form, err := fe.ParseSchema(demoschemas.Schema("repeatable"))
	if err != nil {
		h.logger.Error("demo: parsing the repeatable schema failed", "error", err)
		http.Error(w, h.errText(r, "error.server_error", "internal error"), http.StatusInternalServerError)
		return
	}
	// blocks: indexed names, rebuilt with their nested lines.
	blocks := fe.ParseIndexedForm(r.PostForm, form.Fields)["blocks"]
	if blocks == nil {
		blocks = []interface{}{}
	}
	// workflow_states: the flat {field}_{item}[] convention, unchanged.
	states := []interface{}{}
	for _, v := range r.PostForm["workflow_states_name[]"] {
		if v = strings.TrimSpace(v); v != "" {
			states = append(states, map[string]interface{}{"name": v})
		}
	}

	if _, err := h.xoluClient.Create(ctx, demoEntity, map[string]any{
		"tenant_id":       tenantID,
		"demo":            true,
		"name":            name,
		"blocks":          blocks,
		"workflow_states": states,
		"created_by":      userID,
		"created_at":      time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		h.logger.Error("demo: saving a demo record failed", "error", err)
		http.Error(w, h.errText(r, "error.save_failed", "failed to save demo record"), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/demo/rows?tab=repeatable", http.StatusSeeOther)
}

// DemoTemplateDelete handles POST /demo/templates/{id}/delete. It deletes the
// record only if it belongs to the caller's tenant AND is marked demo=true;
// anything else is "not found".
func (h *UIHandler) DemoTemplateDelete(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	tenantID, _, ok := demoTenant(ctx)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if !ok || err != nil || id <= 0 {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("error.not_found"))
		return
	}
	entity, err := h.xoluClient.Get(ctx, demoEntity, id)
	if err != nil || !demoRecordOwned(entity.Data, tenantID) {
		h.page.RenderError(w, http.StatusNotFound, t("error.not_found"), t("error.not_found"))
		return
	}
	if err := h.xoluClient.Delete(ctx, demoEntity, id); err != nil {
		h.logger.Error("demo: deleting a demo record failed", "error", err)
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.delete_failed"))
		return
	}
	http.Redirect(w, r, "/demo/rows?tab=repeatable", http.StatusSeeOther)
}

// DemoTemplateReset handles POST /demo/templates/reset: it deletes the
// caller's tenant's demo records (up to demoMaxReset per call), skipping
// anything not marked demo=true.
func (h *UIHandler) DemoTemplateReset(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)
	ctx := r.Context()
	tenantID, _, ok := demoTenant(ctx)
	if !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	rows, err := h.demoRows(ctx, tenantID, demoMaxReset)
	if err != nil {
		h.logger.Error("demo: listing demo records for reset failed", "error", err)
		h.page.RenderError(w, http.StatusInternalServerError, t("error.generic"), t("error.delete_failed"))
		return
	}
	for _, row := range rows {
		id := toInt64(row["id"])
		if id <= 0 || !demoRecordOwned(row, tenantID) {
			continue
		}
		if err := h.xoluClient.Delete(ctx, demoEntity, id); err != nil {
			h.logger.Error("demo: deleting a demo record failed", "id", id, "error", err)
		}
	}
	http.Redirect(w, r, "/demo/rows?tab=repeatable", http.StatusSeeOther)
}

// demoRecordOwned reports whether a record is a demo record of the tenant.
func demoRecordOwned(data map[string]any, tenantID int64) bool {
	isDemo, _ := data["demo"].(bool)
	return isDemo && toInt64(data["tenant_id"]) == tenantID
}
