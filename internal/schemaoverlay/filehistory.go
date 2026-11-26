package schemaoverlay

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// The filesystem layer's history: every definition the editors save or remove
// in the override directory is also recorded, as one JSON line, in
// <dir>/.history/forms.jsonl, so an earlier version can be restored. The
// directory's loaders ignore dot-directories, so the history is never read as
// a definition and never looks like a change to the hot loader. The first time
// a definition that was already there is changed, what was there is recorded
// first, as the version before it. A file changed without the editors is
// recorded when the layer next loads it (fileoutside.go).

const (
	historyDirName = ".history"
	// MaxFileHistory is how many versions History returns, newest first.
	MaxFileHistory = 50
	// maxFileNote is the longest note kept with a version.
	maxFileNote = 200
)

// FileVersion is one recorded version of a definition in the filesystem layer.
// A removal is a version with Removed set and no document. ChangedAt is zero
// for a definition that was there before the history began.
type FileVersion struct {
	Module    string    `json:"module"`
	Name      string    `json:"name"`
	Version   int       `json:"version"`
	Doc       string    `json:"doc,omitempty"`
	Removed   bool      `json:"removed,omitempty"`
	Note      string    `json:"note,omitempty"`
	ChangedBy int64     `json:"by,omitempty"`
	ChangedAt time.Time `json:"at"`
}

func (l *FileLayer) historyPath() string {
	return filepath.Join(l.dir, historyDirName, "forms.jsonl")
}

// readHistory returns the recorded versions of one definition, oldest first. A
// line that cannot be read is skipped: the history is a record, not a source.
func (l *FileLayer) readHistory(module, name string) ([]FileVersion, error) {
	f, err := os.Open(l.historyPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []FileVersion
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 4*MaxSchemaBytes)
	for sc.Scan() {
		var v FileVersion
		if json.Unmarshal(sc.Bytes(), &v) != nil || v.Module != module || v.Name != name {
			continue
		}
		out = append(out, v)
	}
	return out, sc.Err()
}

func cleanFileNote(note string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(note) {
		if r >= 0x20 && r != 0x7f {
			b.WriteRune(r)
		}
	}
	n := b.String()
	if utf8.RuneCountInString(n) > maxFileNote {
		n = string([]rune(n)[:maxFileNote])
	}
	return n
}

// recordLocked appends a version of one definition; the caller holds writeMu.
// had and previous are what the directory held before the change: when the
// definition has no history yet and was already there, that is recorded first.
func (l *FileLayer) recordLocked(module, name string, doc []byte, removed bool, note string, by int64, had bool, previous []byte) error {
	history, err := l.readHistory(module, name)
	if err != nil {
		return err
	}
	next := 1
	if n := len(history); n > 0 {
		next = history[n-1].Version + 1
	}
	var lines []FileVersion
	if len(history) == 0 && had {
		lines = append(lines, FileVersion{Module: module, Name: name, Version: next, Doc: string(previous)})
		next++
	}
	lines = append(lines, FileVersion{Module: module, Name: name, Version: next, Doc: string(doc), Removed: removed,
		Note: cleanFileNote(note), ChangedBy: by, ChangedAt: time.Now().UTC()})
	fail := func(err error) error {
		return fmt.Errorf("schemaoverlay: the change is applied, but its history could not be recorded: %w", err)
	}
	p := l.historyPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fail(err)
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fail(err)
	}
	for _, line := range lines {
		data, err := json.Marshal(line)
		if err == nil {
			_, err = f.Write(append(data, '\n'))
		}
		if err != nil {
			f.Close()
			return fail(err)
		}
	}
	if err := f.Close(); err != nil {
		return fail(err)
	}
	return nil
}

// History returns up to MaxFileHistory recorded versions of one definition of
// the filesystem layer, newest first.
func (l *FileLayer) History(module, name string) ([]FileVersion, error) {
	if _, err := l.schemaPath(module, name); err != nil {
		return nil, err
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	history, err := l.readHistory(module, name)
	if err != nil {
		return nil, err
	}
	out := make([]FileVersion, 0, len(history))
	for i := len(history) - 1; i >= 0 && len(out) < MaxFileHistory; i-- {
		out = append(out, history[i])
	}
	return out, nil
}
