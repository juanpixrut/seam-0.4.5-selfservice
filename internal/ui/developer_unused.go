package ui

import (
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/ha1tch/seam-ui/internal/i18n"
	"github.com/ha1tch/seam-ui/internal/i18n/keyusage"
)

// Texts nothing in the source refers to (campaign DEV-19). The Translations
// list offers them as a filter when a source checkout is configured (the
// setting developer_source_dir; `make dev` points it at the working tree).
// The tree is read on demand and what it says is kept for a short while, so a
// page of the list does not read it again and an edit shows within that time.
// The result is a list of candidates (internal/i18n/keyusage says why), which
// is why the filter says "not referenced" and nothing is removed.

// developerSourceScanTTL is how long one reading of the source tree is used.
const developerSourceScanTTL = 30 * time.Second

type developerSourceScan struct {
	mu   sync.Mutex
	dir  string
	at   time.Time
	scan *keyusage.Scan
}

var developerSource developerSourceScan

func (c *developerSourceScan) get(dir string) *keyusage.Scan {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.scan != nil && c.dir == dir && time.Since(c.at) < developerSourceScanTTL {
		return c.scan
	}
	scan, err := keyusage.ScanDir(dir)
	if err != nil {
		return nil // an unreadable tree offers nothing; the next request tries again
	}
	c.dir, c.at, c.scan = dir, time.Now(), scan
	return scan
}

// unreferencedKeys are the keys of the reference locale, as the tenant sees
// it, that the configured source tree does not refer to, sorted. Keys that
// exist only as overrides are left out: they were created on purpose, for code
// that is not written yet.
func (h *UIHandler) unreferencedKeys(_ int64) []string {
	dir := h.config.DevSettings.SourceDir
	if dir == "" || h.translator == nil {
		return nil
	}
	scan := developerSource.get(dir)
	if scan == nil {
		return nil
	}
	raw, ok := i18n.EmbeddedLocale(developerRefLocale)
	if !ok {
		return nil
	}
	var builtIn map[string]string
	if json.Unmarshal(raw, &builtIn) != nil {
		return nil
	}
	keys := make([]string, 0, len(builtIn))
	for k := range builtIn {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return scan.Unreferenced(keys)
}

// translationSource is what the source says about one key, for its editor.
type translationSource struct {
	Rule   string   // a keyusage rule, or "" for nothing
	Detail string   // the starting string, for a key built from one
	Where  []string // "file:line", a few at most
}

// translationSourceOf says how the configured source tree refers to key, or
// nil when no tree is configured or it cannot be read.
func (h *UIHandler) translationSourceOf(key string) *translationSource {
	dir := h.config.DevSettings.SourceDir
	if dir == "" {
		return nil
	}
	scan := developerSource.get(dir)
	if scan == nil {
		return nil
	}
	rule, detail, where := scan.Why(key)
	return &translationSource{Rule: rule, Detail: detail, Where: where}
}
