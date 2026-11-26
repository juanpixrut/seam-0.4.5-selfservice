package ui

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"github.com/ha1tch/seam-ui/web"
)

// GET /picker/icons?all=1 returns every name as a plain array, for the
// widget's Browse grid.
//
// GET /picker/icons?q=<text> backs the "icon-picker" form widget: a live
// search over the Material Icons ligature names vendored with the app
// (web/static/vendor/material-icons-ligatures.txt, the same list `make
// check-icons` audits against), so only icons the font can draw are offered.
// Case-insensitive; names that start with the text come before names that
// merely contain it; at most iconPickerMaxResults are returned. Open to every
// authenticated user (it exposes only the public icon names).

const iconPickerMaxResults = 30

type iconResult struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

var (
	iconNamesOnce sync.Once
	iconNames     []string
)

// loadIconNames reads the embedded ligature list once.
func loadIconNames() []string {
	iconNamesOnce.Do(func() {
		data, err := web.Static.ReadFile("static/vendor/material-icons-ligatures.txt")
		if err != nil {
			return
		}
		sc := bufio.NewScanner(bytes.NewReader(data))
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			iconNames = append(iconNames, line)
		}
	})
	return iconNames
}

// searchIconNames returns up to limit names matching q (ignoring case):
// prefix matches first, then names that contain it, each group in list order.
func searchIconNames(names []string, q string, limit int) []iconResult {
	out := []iconResult{}
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" || limit <= 0 {
		return out
	}
	var contains []string
	for _, n := range names {
		switch {
		case strings.HasPrefix(n, q):
			if len(out) < limit {
				out = append(out, iconResult{ID: n, Label: n})
			}
		case strings.Contains(n, q):
			contains = append(contains, n)
		}
	}
	for _, n := range contains {
		if len(out) >= limit {
			break
		}
		out = append(out, iconResult{ID: n, Label: n})
	}
	return out
}

// PickerIcons handles GET /picker/icons.
func (h *UIHandler) PickerIcons(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if rs := []rune(q); len(rs) > 50 {
		q = string(rs[:50])
	}
	w.Header().Set("Content-Type", "application/json")
	var payload any = searchIconNames(loadIconNames(), q, iconPickerMaxResults)
	if r.URL.Query().Get("all") == "1" {
		// Every name, for the browse grid (about 25 KB; the client caches it).
		payload = append([]string{}, loadIconNames()...)
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		h.logger.Error("icon picker encode failed", "error", err)
	}
}
