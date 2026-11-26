package i18n

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// The filesystem layer's history: every text the editors save or remove in the
// override directory is also recorded, as one JSON line, in
// <dir>/.history/texts.jsonl, so an earlier wording can be restored. The
// directory's loaders ignore dot-directories, so the history is never read as
// a locale file and never looks like a change to the hot loader. The first time
// a text that was already there is changed, what was there is recorded first,
// as the version before it. A file changed without the editors is recorded
// when the layer next loads it (fileoutside.go).

// historyDirName is the dot-directory that holds a layer's history.
const historyDirName = ".history"

// MaxFileHistory is how many versions History returns, newest first.
const MaxFileHistory = 50

// FileVersion is one recorded version of a text in the filesystem layer. A
// removal is a version with Removed set and no value. ChangedAt is zero for a
// text that was there before the history began.
type FileVersion struct {
	Locale    string    `json:"locale"`
	Key       string    `json:"key"`
	Version   int       `json:"version"`
	Value     string    `json:"value,omitempty"`
	Removed   bool      `json:"removed,omitempty"`
	ChangedBy int64     `json:"by,omitempty"`
	ChangedAt time.Time `json:"at"`
}

func (l *FileLayer) historyPath() string {
	return filepath.Join(l.dir, historyDirName, "texts.jsonl")
}

// readHistory returns the recorded versions of one text, oldest first. A line
// that cannot be read is skipped: the history is a record, not a source.
func (l *FileLayer) readHistory(locale, key string) ([]FileVersion, error) {
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
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		var v FileVersion
		if json.Unmarshal(sc.Bytes(), &v) != nil || v.Locale != locale || v.Key != key {
			continue
		}
		out = append(out, v)
	}
	return out, sc.Err()
}

func appendHistoryLines(path string, lines ...any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	for _, line := range lines {
		data, err := json.Marshal(line)
		if err != nil {
			f.Close()
			return err
		}
		if _, err := f.Write(append(data, '\n')); err != nil {
			f.Close()
			return err
		}
	}
	return f.Close()
}

// recordLocked appends a version of one text; the caller holds writeMu. had
// and previous are what the file held before the change: when the text has no
// history yet and was already there, that is recorded first.
func (l *FileLayer) recordLocked(locale, key, value string, removed bool, by int64, had bool, previous string) error {
	history, err := l.readHistory(locale, key)
	if err != nil {
		return err
	}
	next := 1
	if n := len(history); n > 0 {
		next = history[n-1].Version + 1
	}
	var lines []any
	if len(history) == 0 && had {
		lines = append(lines, FileVersion{Locale: locale, Key: key, Version: next, Value: previous})
		next++
	}
	lines = append(lines, FileVersion{Locale: locale, Key: key, Version: next, Value: value, Removed: removed, ChangedBy: by, ChangedAt: time.Now().UTC()})
	if err := appendHistoryLines(l.historyPath(), lines...); err != nil {
		return fmt.Errorf("i18n: the change is applied, but its history could not be recorded: %w", err)
	}
	return nil
}

// History returns up to MaxFileHistory recorded versions of one text of the
// filesystem layer, newest first.
func (l *FileLayer) History(locale, key string) ([]FileVersion, error) {
	if !localeNameRE.MatchString(locale) {
		return nil, fmt.Errorf("%w: %q is not a locale name", ErrInvalid, locale)
	}
	if !overrideKeyRE.MatchString(key) {
		return nil, fmt.Errorf("%w: %q is not a translation key", ErrInvalid, key)
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	history, err := l.readHistory(locale, key)
	if err != nil {
		return nil, err
	}
	out := make([]FileVersion, 0, len(history))
	for i := len(history) - 1; i >= 0 && len(out) < MaxFileHistory; i-- {
		out = append(out, history[i])
	}
	return out, nil
}
