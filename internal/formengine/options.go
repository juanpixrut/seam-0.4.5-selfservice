package formengine

import "sync/atomic"

// Options are the process-wide switches of the visual components, set once at
// startup from the feature configuration (internal/config FeatureConfig) and
// read at render time. They are package-level, not fields of FormEngine,
// because a FormEngine is built in a dozen places and every one of them must
// agree. The zero value is not the default: use DefaultOptions.
//
//	SearchableSelect     select widgets become the SeamSelect combobox
//	                     (default on); off leaves the browser's native select.
//	SelectSearchMin      a combobox shows its search box from this many options
//	                     (default 8); 1 means always. Out of range = default.
//	ReorderRows          x-seam-reorder on a repeatable is honoured (default
//	                     on); off ignores the property everywhere, a
//	                     kill switch that leaves schemas untouched.
//	DocumentLinksLegacy  document-links keeps its previous hand-coded look
//	                     (red cross, bare inputs) instead of the repeatable's
//	                     (default off).
type Options struct {
	SearchableSelect    bool
	SelectSearchMin     int
	ReorderRows         bool
	DocumentLinksLegacy bool
}

const (
	defaultSelectSearchMin = 8
	maxSelectSearchMin     = 10000
)

// DefaultOptions are the values Seam ships with.
func DefaultOptions() Options {
	return Options{SearchableSelect: true, SelectSearchMin: defaultSelectSearchMin, ReorderRows: true}
}

// normalized brings out-of-range values back to something safe: a threshold
// below 1 or above maxSelectSearchMin is the default, never "always" or "never"
// by accident.
func (o Options) normalized() Options {
	if o.SelectSearchMin < 1 || o.SelectSearchMin > maxSelectSearchMin {
		o.SelectSearchMin = defaultSelectSearchMin
	}
	return o
}

var current atomic.Pointer[Options]

// SetOptions replaces the process-wide options. Safe to call while rendering.
func SetOptions(o Options) {
	o = o.normalized()
	current.Store(&o)
}

// CurrentOptions returns the options in force (DefaultOptions until SetOptions).
func CurrentOptions() Options {
	if p := current.Load(); p != nil {
		return *p
	}
	return DefaultOptions()
}

// reorderOn reports whether f's rows get reorder handles: the schema asks for
// it and the process-wide switch allows it.
func reorderOn(f Field) bool {
	return f.XSeamReorder && CurrentOptions().ReorderRows
}
