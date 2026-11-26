package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/ha1tch/seam-ui/internal/reports"
)

// ReportsHandler handles reporting endpoints.
type ReportsHandler struct {
	*Handler
	reportsSvc *reports.Service
}

// NewReportsHandler creates a new reports handler.
func NewReportsHandler(h *Handler, svc *reports.Service) *ReportsHandler {
	return &ReportsHandler{
		Handler:    h,
		reportsSvc: svc,
	}
}

// Dashboard handles GET /api/v1/reports/dashboard
func (h *ReportsHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := getTenantID(r)

	stats, err := h.reportsSvc.GetDashboardStats(ctx, tenantID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "report_error", err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, stats)
}

// AssetReport handles GET /api/v1/reports/assets
func (h *ReportsHandler) AssetReport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := getTenantID(r)

	tr := parseTimeRange(r)

	report, err := h.reportsSvc.GetAssetReport(ctx, tenantID, tr)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "report_error", err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, report)
}

// EventReport handles GET /api/v1/reports/events
func (h *ReportsHandler) EventReport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := getTenantID(r)

	tr := parseTimeRange(r)

	report, err := h.reportsSvc.GetEventReport(ctx, tenantID, tr)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "report_error", err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, report)
}

// ExportAssets handles GET /api/v1/reports/export/assets
func (h *ReportsHandler) ExportAssets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := getTenantID(r)

	format := getExportFormat(r)

	data, err := h.reportsSvc.ExportAssets(ctx, tenantID, format)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "export_error", err.Error())
		return
	}

	writeExport(w, data, "assets", format)
}

// ExportEvents handles GET /api/v1/reports/export/events
func (h *ReportsHandler) ExportEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := getTenantID(r)

	tr := parseTimeRange(r)
	format := getExportFormat(r)

	data, err := h.reportsSvc.ExportEvents(ctx, tenantID, tr, format)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "export_error", err.Error())
		return
	}

	writeExport(w, data, "events", format)
}

// Helper functions

func parseTimeRange(r *http.Request) reports.TimeRange {
	rangeParam := r.URL.Query().Get("range")

	switch rangeParam {
	case "today":
		return reports.Today()
	case "yesterday":
		return reports.Yesterday()
	case "week":
		return reports.ThisWeek()
	case "month":
		return reports.ThisMonth()
	case "30days":
		return reports.Last30Days()
	case "90days":
		return reports.Last90Days()
	default:
		// Try custom range
		fromStr := r.URL.Query().Get("from")
		toStr := r.URL.Query().Get("to")

		if fromStr != "" && toStr != "" {
			from, err1 := time.Parse("2006-01-02", fromStr)
			to, err2 := time.Parse("2006-01-02", toStr)
			if err1 == nil && err2 == nil {
				return reports.TimeRange{From: from, To: to.Add(24*time.Hour - time.Second)}
			}
		}

		// Default to last 30 days
		return reports.Last30Days()
	}
}

func getExportFormat(r *http.Request) reports.ExportFormat {
	format := r.URL.Query().Get("format")
	switch format {
	case "csv":
		return reports.FormatCSV
	default:
		return reports.FormatJSON
	}
}

func writeExport(w http.ResponseWriter, data []byte, name string, format reports.ExportFormat) {
	timestamp := time.Now().Format("20060102-150405")
	filename := name + "-" + timestamp

	switch format {
	case reports.FormatCSV:
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename="+filename+".csv")
	default:
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename="+filename+".json")
	}

	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Write(data)
}
