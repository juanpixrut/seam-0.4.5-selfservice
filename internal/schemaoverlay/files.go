package schemaoverlay

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ha1tch/seam-ui/internal/dirwatch"
)

// The filesystem layer (campaign DEV-15). A directory holds one sub-directory
// per module and one <name>.json file per schema:
//
//	<dir>/system/user_edit.json
//	<dir>/masters/brand.json
//
// Reloading is file by file: an unreadable, malformed or rejected file is set
// aside alone, the definition loaded before it stays in force, and the rest
// of the directory loads (FileStatus.Refused), so a half-saved edit never
// breaks a running form and never holds back the other files. Only a
// directory that cannot be read at all leaves the whole layer as it was.

var identPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// LoadDir reads every <module>/<name>.json under dir. Anything else is
// ignored. All problems are reported together, naming the file.
func LoadDir(dir string) (map[Key][]byte, error) {
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

// loadDirFiles is LoadDir file by file: what could be read, and each file or
// module directory that could not, with the reason (which names the file).
// The error is for a directory that cannot be read at all.
func loadDirFiles(dir string) (map[Key][]byte, []FileRefusal, error) {
	mods, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("schemaoverlay: read schema directory: %w", err)
	}
	out := make(map[Key][]byte)
	var problems []FileRefusal
	for _, m := range mods {
		// Dot-directories (.git) and dot-files (the writers' and editors'
		// temporary files) are never schemas.
		if !m.IsDir() || strings.HasPrefix(m.Name(), ".") {
			continue
		}
		files, err := os.ReadDir(filepath.Join(dir, m.Name()))
		if errors.Is(err, os.ErrNotExist) {
			continue // removed while the directory was being read
		}
		if err != nil {
			problems = append(problems, FileRefusal{Key: Key{Module: m.Name()}, Reason: fmt.Sprintf("%s: %v", m.Name(), err)})
			continue
		}
		for _, f := range files {
			if f.IsDir() || strings.HasPrefix(f.Name(), ".") || !strings.HasSuffix(f.Name(), ".json") {
				continue
			}
			rel := m.Name() + "/" + f.Name()
			name := strings.TrimSuffix(f.Name(), ".json")
			if !identPattern.MatchString(m.Name()) || !identPattern.MatchString(name) {
				problems = append(problems, FileRefusal{Key: Key{Module: m.Name(), Name: name},
					Reason: rel + ": module and schema names must be lower-case letters, digits and underscores"})
				continue
			}
			raw, err := os.ReadFile(filepath.Join(dir, m.Name(), f.Name()))
			if errors.Is(err, os.ErrNotExist) {
				continue // removed while the directory was being read
			}
			if err != nil {
				problems = append(problems, FileRefusal{Key: Key{Module: m.Name(), Name: name}, Reason: fmt.Sprintf("%s: %v", rel, err)})
				continue
			}
			out[Key{Module: m.Name(), Name: name}] = raw
		}
	}
	return out, problems, nil
}

// dirSignature summarises the *.json files two levels down by path, size and
// modification time.
func dirSignature(dir string) (string, error) {
	var parts []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if errors.Is(err, os.ErrNotExist) {
			return nil // removed while the directory was being walked
		}
		if err != nil {
			return err
		}
		// Dot-directories and dot-files (temporary files) are not schemas and
		// must not look like a change while a writer is mid-save.
		if strings.HasPrefix(d.Name(), ".") && path != dir {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}
		info, err := d.Info()
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		parts = append(parts, fmt.Sprintf("%s|%d|%d", filepath.ToSlash(rel), info.Size(), info.ModTime().UnixNano()))
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(parts)
	return strings.Join(parts, "\n"), nil
}

// FileStatus describes the filesystem layer for the Developer panel.
type FileStatus struct {
	Dir      string
	LoadedAt time.Time // zero until the first successful load
	Schemas  []Key     // schemas the layer currently overrides, sorted
	// LastError is the most recent failure, "" if the last attempt worked.
	// While it is set, the previous good layer is still in force.
	LastError string
	// Refused are the files the layer's check set aside at the last load,
	// sorted (see SetCheck).
	Refused []FileRefusal
}

// FileRefusal is one file of the directory that is not in force, and why.
type FileRefusal struct {
	Key    Key
	Reason string
	// Kept reports that the definition loaded before it is still in force.
	Kept bool
}

// FileLayer keeps a Resolver's filesystem layer in step with a directory.
type FileLayer struct {
	r   *Resolver
	dir string

	mu     sync.Mutex
	sig    string
	status FileStatus
	check  func(k Key, doc []byte) error
	last   map[Key][]byte // what the last good load put in force

	// what the directory was last seen to hold (the files that would be put in
	// force), for the history of changes made without the editors
	seen     map[Key][]byte
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

// NewFileLayer returns a FileLayer for dir. It reads nothing until Reload or
// Poll.
func NewFileLayer(r *Resolver, dir string) *FileLayer {
	return &FileLayer{r: r, dir: dir, status: FileStatus{Dir: dir}}
}

// SetCheck installs a check every well-formed definition of the directory
// must pass to be put in force; call it before the first Reload. A file that
// fails it is set aside like one that cannot be read or is not a definition:
// alone, with the definition loaded before it left in force (none, if there
// was none), and listed in Status().Refused with the reason. This is how a
// definition the engine could not render, written into the directory by hand,
// never reaches a page.
func (f *FileLayer) SetCheck(check func(k Key, doc []byte) error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.check = check
}

// Reload reads the directory, validates every schema and atomically replaces
// the filesystem layer. On any error the layer is left as it was and the
// error is recorded in Status.
//
// A definition found changed, added or removed by something other than the
// editors is recorded in the history as a version of its own (filehistory.go).
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
	onDisk, accepted, refused, err := f.readLocked()
	if err == nil && serr != nil {
		err = serr
	}
	data := make(map[Key][]byte, len(accepted)+len(refused))
	for k, raw := range accepted {
		data[k] = raw
	}
	for i := range refused {
		if prev, ok := f.last[refused[i].Key]; ok {
			data[refused[i].Key], refused[i].Kept = prev, true
		}
	}
	if err == nil {
		err = f.r.ReloadFilesystem(data)
	}
	if err != nil {
		f.status.LastError = err.Error()
		if serr == nil {
			f.sig = sig // report a broken file once, not on every poll
		}
		return err
	}
	f.sig = sig
	f.status.LoadedAt = time.Now()
	f.status.LastError = ""
	f.status.Schemas = f.status.Schemas[:0]
	for k := range data {
		f.status.Schemas = append(f.status.Schemas, k)
	}
	sort.Slice(f.status.Schemas, func(i, j int) bool { return lessKey(f.status.Schemas[i], f.status.Schemas[j]) })
	sort.Slice(refused, func(i, j int) bool { return lessKey(refused[i].Key, refused[j].Key) })
	f.status.Refused = refused
	if outside {
		// a failure to record is not a failure to load: the history is a record
		_ = f.recordOutsideLocked(accepted, onDisk, f.seen, f.observed)
	}
	f.last = data
	f.seen, f.seenSig, f.observed = accepted, sig, true
	return nil
}

// readLocked reads the directory and sorts its files: onDisk is every file
// that is there under a name of its own (set aside or not), accepted the ones
// that would be put in force, refused the rest with the reason.
func (f *FileLayer) readLocked() (onDisk, accepted map[Key][]byte, refused []FileRefusal, err error) {
	read, refused, err := loadDirFiles(f.dir)
	if err != nil {
		return nil, nil, nil, err
	}
	onDisk = make(map[Key][]byte, len(read)+len(refused))
	for _, p := range refused {
		onDisk[p.Key] = nil
	}
	accepted = make(map[Key][]byte, len(read))
	for k, raw := range read {
		onDisk[k] = raw
		verr := f.r.Validate(raw)
		if verr != nil {
			verr = fmt.Errorf("%s: %w", k, verr)
		} else if f.check != nil {
			verr = f.check(k, raw)
		}
		if verr != nil {
			refused = append(refused, FileRefusal{Key: k, Reason: verr.Error()})
			continue
		}
		accepted[k] = raw
	}
	return onDisk, accepted, refused, nil
}

// Observe records in the history what changed in the directory without the
// editors since it was last looked at, and loads nothing: the layer in force
// is left as it is. The poller calls it while hot loading is switched off, so
// the history does not depend on that switch.
func (f *FileLayer) Observe() {
	writeMu.Lock()
	defer writeMu.Unlock()
	f.mu.Lock()
	defer f.mu.Unlock()
	sig, err := dirSignature(f.dir)
	if err != nil || (f.observed && sig == f.seenSig) {
		return
	}
	onDisk, accepted, _, err := f.readLocked()
	if err != nil {
		return
	}
	_ = f.recordOutsideLocked(accepted, onDisk, f.seen, f.observed)
	f.seen, f.seenSig, f.observed = accepted, sig, true
}

// ReloadIfChanged reloads only if the directory's files differ from the last
// look, reporting whether a reload was attempted.
func (f *FileLayer) ReloadIfChanged() (bool, error) {
	writeMu.Lock()
	defer writeMu.Unlock()
	f.mu.Lock()
	defer f.mu.Unlock()
	sig, err := dirSignature(f.dir)
	if err == nil && sig == f.sig && (!f.status.LoadedAt.IsZero() || f.status.LastError != "") {
		return false, nil
	}
	return true, f.reloadLocked(true)
}

// Poll checks the directory every interval until ctx is cancelled, reloading
// when something changed. While active returns false the check is skipped
// (the hot-load flag is off); a nil active means always. It blocks, so run
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
		_, _ = f.ReloadIfChanged() // the outcome is recorded in Status
	}
	check()
	// Where the system reports changes, a change is acted on at once: loaded
	// if hot loading is on, recorded in the history either way. The ticks
	// below stay as the safety net (dirwatch says why).
	if dirwatch.Mode(f.watchMode()) == "events" {
		dirwatch.Watch(ctx, f.dir, true, check)
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
	s.Schemas = append([]Key(nil), s.Schemas...)
	s.Refused = append([]FileRefusal(nil), s.Refused...)
	return s
}
