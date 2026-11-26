package ui

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// FSMExportPNG handles POST /fsm/export/png.
// Accepts an SVG request body, rasterises it via resvg, and returns a PNG.
// Requires resvg to be installed and on PATH.
func (h *UIHandler) FSMExportPNG(w http.ResponseWriter, r *http.Request) {
	// Locate resvg
	resvgPath, err := exec.LookPath("resvg")
	if err != nil {
		http.Error(w,
			"resvg not found — install resvg (https://github.com/RazrFalcon/resvg) and ensure it is on PATH",
			http.StatusServiceUnavailable)
		return
	}

	// Read SVG body (10 MB cap)
	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
	if err != nil || len(body) == 0 {
		http.Error(w, "failed to read SVG body", http.StatusBadRequest)
		return
	}

	// Write SVG to a temporary directory
	tmpDir, err := os.MkdirTemp("", "seam-fsm-*")
	if err != nil {
		h.logger.Error("FSMExportPNG: failed to create temp dir", "error", err)
		http.Error(w, h.errText(r, "error.server_error", "internal error"), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tmpDir)

	svgPath := filepath.Join(tmpDir, "fsm.svg")
	pngPath := filepath.Join(tmpDir, "fsm.png")

	if err := os.WriteFile(svgPath, body, 0o600); err != nil {
		h.logger.Error("FSMExportPNG: failed to write SVG", "error", err)
		http.Error(w, h.errText(r, "error.server_error", "internal error"), http.StatusInternalServerError)
		return
	}

	// Run resvg: white background so transparent node interiors render cleanly
	cmd := exec.CommandContext(r.Context(), resvgPath,
		"--background", "white",
		svgPath, pngPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		h.logger.Error("FSMExportPNG: resvg failed", "error", err, "output", string(out))
		http.Error(w, "PNG conversion failed: "+string(out), http.StatusInternalServerError)
		return
	}

	// Read and serve the PNG
	png, err := os.ReadFile(pngPath)
	if err != nil {
		h.logger.Error("FSMExportPNG: failed to read PNG output", "error", err)
		http.Error(w, h.errText(r, "error.server_error", "internal error"), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Disposition", `attachment; filename="fsm-diagram.png"`)
	w.Header().Set("Content-Length", strconv.Itoa(len(png)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(png)
}
