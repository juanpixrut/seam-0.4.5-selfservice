package i18n

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ha1tch/seam-ui/internal/dirwatch"
)

// This file is the filesystem layer's loader (campaign DEV-12): it reads a
// directory of per-locale JSON files into LayerFilesystem, and can watch the
// directory for changes by polling modification times (no dependency on a
// file-notification library).
//
// Each file is named <locale>.json (en.json, pt_br.json) and has the same
// flat shape as the embedded locale files: {"key": "message", ...}. A file
// lists only the keys it overrides; everything else still comes from the
// layers below it.
//
// Reloading is file by file: a file that cannot be read, is not a flat object
// of texts or is not named after a locale is set aside alone, the texts that
// locale had before stay in force, and the other files load
// (FileStatus.Refused), so a half-saved edit never blanks a language and never
// holds back the others. Only a directory that cannot be read at all leaves
// the whole layer as it was.

var localeFileName = regexp.MustCompile(`^[a-z]{2,3}(_[a-z0-9]+)*$`)

// LoadDir reads every *.json file in dir and returns locale -> key ->
// message. Other files and sub-directories are ignored. All problems are
// reported together, naming the file.
func LoadDir(dir string) (map[string]map[string]string, error) {
	out, problems, err := loadDirFiles(dir)
	if err != nil {
		return nil, err
	}
	if len(problems) > 0 {
		errs := make([]error, 0, len(problems))
		for _, p := range problems {
			errs = append(errs, errors.New(p.Reason))
		}
		return nil, errors.Join(errs...)
	}
	return out, nil
}

// FileRefusal is one file of the directory that is not in force, and why.
type FileRefusal struct {
	File   string // the file's name
	Locale string // the locale it is named after, "" when the name is not one
	Reason string // names the file
	// Kept reports that the texts loaded before for that locale are still in force.
	Kept bool
}

// loadDirFiles is LoadDir file by file: what could be read, and each file
// that could not, with the reason. The error is for a directory that cannot
// be read at all.
func loadDirFiles(dir string) (map[string]map[string]string, []FileRefusal, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("i18n: read locale directory: %w", err)
	}
	out := make(map[string]map[string]string)
	var problems []FileRefusal
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		locale := strings.TrimSuffix(e.Name(), ".json")
		if !localeFileName.MatchString(locale) {
			problems = append(problems, FileRefusal{File: e.Name(), Reason: e.Name() + ": not a valid locale file name (want e.g. en.json or pt_br.json)"})
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if errors.Is(err, os.ErrNotExist) {
			continue // removed while the directory was being read
		}
		if err != nil {
			problems = append(problems, FileRefusal{File: e.Name(), Locale: locale, Reason: fmt.Sprintf("%s: %v", e.Name(), err)})
			continue
		}
		var msgs map[string]string
		if err := json.Unmarshal(raw, &msgs); err != nil {
			problems = append(problems, FileRefusal{File: e.Name(), Locale: locale,
				Reason: fmt.Sprintf("%s: %v (want a flat object of string keys to string messages)", e.Name(), err)})
			continue
		}
		if len(msgs) > 0 {
			out[locale] = msgs
		}
	}
	return out, problems, nil
}

// dirSignature summarises the directory's *.json files by name, size and
// modification time. Two equal signatures mean nothing has changed.
func dirSignature(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var parts []string
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		info, err := e.Info()
		if errors.Is(err, os.ErrNotExist) {
			continue // removed while the directory was being read
		}
		if err != nil {
			return "", err
		}
		parts = append(parts, fmt.Sprintf("%s|%d|%d", e.Name(), info.Size(), info.ModTime().UnixNano()))
	}
	sort.Strings(parts)
	return strings.Join(parts, "\n"), nil
}

// FileStatus describes the filesystem layer for the Developer panel.
type FileStatus struct {
	Dir      string
	LoadedAt time.Time // zero until the first successful load
	Locales  []string  // locales the layer currently overrides, sorted
	Keys     int       // total overridden keys across locales
	// NewKeys lists "locale: key" entries the layer defines that the
	// embedded base does not, which usually means a typo.
	NewKeys []string
	// LastError is the most recent failure, "" if the last attempt worked.
	// While it is set, the previous good layer is still in force.
	LastError string
	// Refused are the files set aside at the last load, sorted by name.
	Refused []FileRefusal
}

// FileLayer keeps LayerFilesystem in step with a directory.
type FileLayer struct {
	t   *Translator
	dir string

	mu     sync.Mutex
	sig    string
	status FileStatus
	last   map[string]map[string]string // what the last load put in force

	// what the directory was last seen to hold, for the history of changes
	// made without the editors
	seen     map[string]map[string]string
	seenSig  string
	observed bool
	history  time.Duration // see SetHistoryInterval
	watch    string        // see SetWatch
}

// SetWatch chooses how Poll follows the directory between its ticks: "auto"
// or "" (change events where this build has them), "events" or "poll". Call
// it before Poll.
func (f *FileLayer) SetWatch(mode string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.watch = mode
}

func (f *FileLayer) watchMode() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.watch
}

// NewFileLayer returns a FileLayer for dir. It does not read anything until
// Reload or Poll.
func NewFileLayer(t *Translator, dir string) *FileLayer {
	return &FileLayer{t: t, dir: dir, status: FileStatus{Dir: dir}}
}

// Reload reads the directory and atomically replaces the filesystem layer.
// On any error the layer is left as it was and the error is also recorded in
// Status.
//
// A text found changed, added or removed by something other than the editors
// is recorded in the history as a version of its own (fileoutside.go).
func (f *FileLayer) Reload() error {
	writeMu.Lock()
	defer writeMu.Unlock()
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reloadLocked(true)
}

// reloadWritten is Reload for the editors' own writes: the caller holds
// writeMu and records the version itself.
func (f *FileLayer) reloadWritten() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reloadLocked(false)
}

func (f *FileLayer) reloadLocked(outside bool) error {
	sig, serr := dirSignature(f.dir)
	accepted, refused, err := loadDirFiles(f.dir)
	if err == nil && serr != nil {
		err = serr
	}
	// a locale whose file is set aside keeps the texts it had: data is what is
	// put in force, seen what the history compares the next look with
	data, seen := make(map[string]map[string]string, len(accepted)), make(map[string]map[string]string, len(accepted))
	for loc, msgs := range accepted {
		data[loc], seen[loc] = msgs, msgs
	}
	for i := range refused {
		loc := refused[i].Locale
		if loc == "" {
			continue
		}
		if prev, ok := f.last[loc]; ok {
			data[loc], refused[i].Kept = prev, true
		}
		if prev, ok := f.seen[loc]; ok {
			seen[loc] = prev
		}
	}
	if err == nil {
		err = f.t.ReloadLayer(LayerFilesystem, 0, data)
	}
	if err != nil {
		f.status.LastError = err.Error()
		// Remember what we looked at so a broken file is reported once, not
		// on every poll; fixing the file changes the signature.
		if serr == nil {
			f.sig = sig
		}
		return err
	}

	f.sig = sig
	f.status.LoadedAt = time.Now()
	f.status.LastError = ""
	f.status.Locales = f.status.Locales[:0]
	f.status.Keys = 0
	f.status.NewKeys = nil
	f.t.mu.RLock()
	for loc, msgs := range data {
		f.status.Locales = append(f.status.Locales, loc)
		f.status.Keys += len(msgs)
		for k := range msgs {
			if _, ok := f.t.messages[loc][k]; !ok {
				f.status.NewKeys = append(f.status.NewKeys, loc+": "+k)
			}
		}
	}
	f.t.mu.RUnlock()
	sort.Strings(f.status.Locales)
	sort.Strings(f.status.NewKeys)
	f.status.Refused = refused
	if outside {
		// a failure to record is not a failure to load: the history is a record
		_ = f.recordOutsideLocked(seen, f.seen, f.observed)
	}
	f.last = data
	f.seen, f.seenSig, f.observed = seen, sig, true
	return nil
}

// Observe records in the history what changed in the directory without the
// editors since it was last looked at, and loads nothing: the layer in force
// is left as it is. The poller calls it while hot loading is switched off, so
// the history does not depend on that switch. A locale whose file cannot be
// read is taken to hold what it held.
func (f *FileLayer) Observe() {
	writeMu.Lock()
	defer writeMu.Unlock()
	f.mu.Lock()
	defer f.mu.Unlock()
	sig, err := dirSignature(f.dir)
	if err != nil || (f.observed && sig == f.seenSig) {
		return
	}
	data, refused, err := loadDirFiles(f.dir)
	if err != nil {
		return
	}
	for _, p := range refused {
		if prev, ok := f.seen[p.Locale]; ok && p.Locale != "" {
			data[p.Locale] = prev
		}
	}
	_ = f.recordOutsideLocked(data, f.seen, f.observed)
	f.seen, f.seenSig, f.observed = data, sig, true
}

// ReloadIfChanged reloads only if the directory's files differ from the last
// look. It reports whether a reload was attempted.
func (f *FileLayer) ReloadIfChanged() (bool, error) {
	writeMu.Lock()
	defer writeMu.Unlock()
	f.mu.Lock()
	defer f.mu.Unlock()
	sig, err := dirSignature(f.dir)
	if err == nil && sig == f.sig && f.status.LoadedAt != (time.Time{}) {
		return false, nil
	}
	if err == nil && sig == f.sig && f.status.LastError != "" {
		return false, nil // same broken state already reported
	}
	return true, f.reloadLocked(true)
}

// Poll checks the directory every interval until ctx is cancelled, reloading
// when something changed. While active returns false the check is skipped
// (the hot-load flag is off); a nil active means always. Poll blocks, so run
// it in a goroutine. The first check happens immediately.
func (f *FileLayer) Poll(ctx context.Context, interval time.Duration, active func() bool) {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	check := func() {
		if active != nil && !active() {
			f.Observe() // nothing is loaded, but a change is still a version
			return
		}
		_, _ = f.ReloadIfChanged() // outcome is in Status
	}
	check()
	// Where the system reports changes, a change is acted on at once: loaded
	// if hot loading is on, recorded in the history either way. The ticks
	// below stay as the safety net (dirwatch says why).
	if dirwatch.Mode(f.watchMode()) == "events" {
		dirwatch.Watch(ctx, f.dir, false, check)
	}
	// Between two checks the directory is looked at more often, for the
	// history only: two changes to a file are two versions unless they fall
	// inside one HistoryInterval.
	look := f.historyInterval()
	if look <= 0 || look > interval {
		look = interval // the history is looked at when the directory is loaded
	}
	tick := time.NewTicker(look)
	defer tick.Stop()
	next := time.Now().Add(interval)
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tick.C:
			if now.Before(next) {
				f.Observe()
				continue
			}
			next = now.Add(interval)
			check()
		}
	}
}

// HistoryInterval is how often a polled directory is looked at for the
// history of changes made without the editors (Observe), whatever the
// interval it is loaded at, unless SetHistoryInterval says otherwise.
const HistoryInterval = 250 * time.Millisecond

// SetHistoryInterval changes how often Poll looks at the directory for the
// history: zero means HistoryInterval, negative means only when it loads.
// Call it before Poll.
func (f *FileLayer) SetHistoryInterval(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.history = d
}

func (f *FileLayer) historyInterval() time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.history == 0 {
		return HistoryInterval
	}
	return f.history
}

// Status returns a copy of the layer's current status.
func (f *FileLayer) Status() FileStatus {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := f.status
	s.Locales = append([]string(nil), s.Locales...)
	s.NewKeys = append([]string(nil), s.NewKeys...)
	s.Refused = append([]FileRefusal(nil), s.Refused...)
	return s
}
