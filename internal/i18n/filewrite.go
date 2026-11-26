package i18n

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"
)

// Writing the filesystem layer (campaign DEV-18). The translation editor
// changes a locale's override file one key at a time through SetOverride and
// RemoveOverride; nothing else in the application writes these files.
//
// Every write is validated before it touches the disk (locale and key shapes,
// a size limit, and the same printf verbs as the reference text, because T
// passes messages to Sprintf), is atomic (a temporary file in the same
// directory, then a rename, so a crash or a concurrent reader never sees half
// a file), never overwrites a file that is not valid JSON (it is the
// developer's own work in progress), and is followed by a reload so the change
// is live at once.

// MaxOverrideLen is the longest override text accepted, in characters.
const MaxOverrideLen = 4000

// The two ways SetOverride refuses a text, so a caller can tell the user which
// without matching on messages.
var (
	// ErrInvalid wraps every refusal for a locale, key or text of the wrong
	// shape: empty, too long, control characters.
	ErrInvalid = errors.New("i18n: invalid override")
	// ErrVerbs wraps the refusal for a text whose printf verbs differ from the
	// original's.
	ErrVerbs = errors.New("i18n: placeholders differ from the original")
)

var (
	localeNameRE  = regexp.MustCompile(`^[a-z]{2,3}(_[a-z0-9]{2,8})?$`)
	overrideKeyRE = regexp.MustCompile(`^[A-Za-z0-9_.\-]{1,200}$`)

	// writeMu serialises every write in the process: the files are small and
	// writes are rare, so one lock is simpler than one per directory.
	writeMu sync.Mutex
)

// newKeyRE is the shape of a key that can be created: lower-case words joined
// by dots, at least two of them, as the keys of the locale files are. It is
// narrower than overrideKeyRE, which only has to accept every existing key.
var newKeyRE = regexp.MustCompile(`^[a-z0-9_-]+(\.[a-z0-9_-]+)+$`)

// ValidNewKey reports whether key has the shape of a key that can be created.
func ValidNewKey(key string) bool {
	return len(key) <= 200 && newKeyRE.MatchString(key)
}

// SameVerbs reports whether two messages use the same printf verbs in the
// same order. Sprintf consumes its arguments in order, so "%s %d" against
// "%d %s" would print "%!d(string=...)": the editor is stricter than the
// completeness report (which compares the verbs as a set) on purpose.
func SameVerbs(a, b string) bool {
	va := verbPattern.FindAllString(strings.ReplaceAll(a, "%%", ""), -1)
	vb := verbPattern.FindAllString(strings.ReplaceAll(b, "%%", ""), -1)
	if len(va) != len(vb) {
		return false
	}
	for i := range va {
		if va[i] != vb[i] {
			return false
		}
	}
	return true
}

func (l *FileLayer) overridePath(locale string) (string, error) {
	if !localeNameRE.MatchString(locale) {
		return "", fmt.Errorf("%w: %q is not a locale name", ErrInvalid, locale)
	}
	return filepath.Join(l.dir, locale+".json"), nil
}

func readOverrideFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	m := map[string]string{}
	if strings.TrimSpace(string(data)) == "" {
		return m, nil
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("i18n: %s is not valid JSON and is left untouched: %w", filepath.Base(path), err)
	}
	return m, nil
}

// writeOverrideFile replaces the file atomically; an empty map removes it.
func writeOverrideFile(path string, m map[string]string) error {
	if len(m) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	data, err := json.MarshalIndent(m, "", "  ") // map keys come out sorted
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // a no-op once renamed
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Overrides returns the keys the locale's override file holds (empty if the
// file does not exist).
func (l *FileLayer) Overrides(locale string) (map[string]string, error) {
	p, err := l.overridePath(locale)
	if err != nil {
		return nil, err
	}
	return readOverrideFile(p)
}

func (l *FileLayer) reloadAfterWrite() error {
	if err := l.reloadWritten(); err != nil {
		return fmt.Errorf("i18n: written, but the layer did not reload: %w", err)
	}
	return nil
}

// CheckOverride validates a locale, key and text for an override and returns
// the text as it is to be stored (a browser's CRLF newlines become LF). It is
// the one definition of what an override may be, shared by the filesystem layer
// and the database store (internal/overrides): shapes and size, no control
// characters, never empty, and, when reference (the text it replaces) is not
// empty, the same printf verbs in the same order. Failures wrap ErrInvalid or
// ErrVerbs.
func CheckOverride(locale, key, value, reference string) (string, error) {
	if !localeNameRE.MatchString(locale) {
		return "", fmt.Errorf("%w: %q is not a locale name", ErrInvalid, locale)
	}
	if !overrideKeyRE.MatchString(key) {
		return "", fmt.Errorf("%w: %q is not a translation key", ErrInvalid, key)
	}
	// A browser submits the newlines of a textarea as CRLF.
	value = strings.ReplaceAll(value, "\r\n", "\n")
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%w: an override cannot be empty (remove it instead)", ErrInvalid)
	}
	if utf8.RuneCountInString(value) > MaxOverrideLen {
		return "", fmt.Errorf("%w: an override is limited to %d characters", ErrInvalid, MaxOverrideLen)
	}
	for _, r := range value {
		if r == 0 || (r < 0x20 && r != '\n' && r != '\t') {
			return "", fmt.Errorf("%w: an override cannot contain control characters", ErrInvalid)
		}
	}
	if reference != "" && !SameVerbs(reference, value) {
		return "", fmt.Errorf("%w: the override must use the same %%s/%%d placeholders as the original text, in the same order", ErrVerbs)
	}
	return value, nil
}

// SetOverride sets one key of one locale in the filesystem layer and reloads
// it. reference is the text the override replaces (usually the embedded
// message); when it is not empty the override must use the same printf verbs.
func (l *FileLayer) SetOverride(locale, key, value, reference string) error {
	return l.SetOverrideAs(locale, key, value, reference, 0)
}

// SetOverrideAs is SetOverride with who made the change, for the history
// (filehistory.go): the version is recorded after the text is written and
// reloaded, and a failure to record it is returned although the text is live.
func (l *FileLayer) SetOverrideAs(locale, key, value, reference string, by int64) error {
	p, err := l.overridePath(locale)
	if err != nil {
		return err
	}
	value, err = CheckOverride(locale, key, value, reference)
	if err != nil {
		return err
	}

	// The reload runs under the lock too, so it never meets another writer
	// halfway through a save.
	writeMu.Lock()
	defer writeMu.Unlock()
	m, err := readOverrideFile(p)
	if err != nil {
		return err
	}
	previous, had := m[key]
	m[key] = value
	if err := writeOverrideFile(p, m); err != nil {
		return err
	}
	if err := l.reloadAfterWrite(); err != nil {
		return err
	}
	return l.recordLocked(locale, key, value, false, by, had, previous)
}

// RemoveOverride removes one key from the locale's override file (deleting
// the file when it was the last) and reloads. Removing an absent key is not an
// error.
func (l *FileLayer) RemoveOverride(locale, key string) error {
	return l.RemoveOverrideAs(locale, key, 0)
}

// RemoveOverrideAs is RemoveOverride with who made the change, for the history.
func (l *FileLayer) RemoveOverrideAs(locale, key string, by int64) error {
	p, err := l.overridePath(locale)
	if err != nil {
		return err
	}
	if !overrideKeyRE.MatchString(key) {
		return fmt.Errorf("%w: %q is not a translation key", ErrInvalid, key)
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	m, err := readOverrideFile(p)
	if err != nil {
		return err
	}
	previous, ok := m[key]
	if !ok {
		return nil
	}
	delete(m, key)
	if err := writeOverrideFile(p, m); err != nil {
		return err
	}
	if err := l.reloadAfterWrite(); err != nil {
		return err
	}
	return l.recordLocked(locale, key, "", true, by, true, previous)
}
