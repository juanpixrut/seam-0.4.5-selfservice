package schemaoverlay

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Writing the filesystem layer (campaign DEV-20). The form-definition editor
// saves and removes whole documents through Save and Remove; nothing else in
// the application writes these files.
//
// A save is validated before it touches the disk (module and name shapes, a
// size limit, and the resolver's validator, which in Seam is the form engine's
// own parser), is atomic (temporary file, then rename), and is followed by a
// reload so the change is live at once. A document that does not validate is
// never written, so the previous good file stays in force.

// MaxSchemaBytes is the largest form definition accepted.
const MaxSchemaBytes = 1 << 20

var (
	// ErrInvalidName wraps the refusal for a module or form name that is not a
	// plain identifier (it would be a path).
	ErrInvalidName = errors.New("schemaoverlay: invalid module or form name")
	// ErrRejected wraps the refusal for a document that is too large or that
	// the validator does not accept; the validator's own message follows.
	ErrRejected = errors.New("schemaoverlay: form definition rejected")
)

// writeMu serialises every write in the process; see i18n's writeMu.
var writeMu sync.Mutex

func (l *FileLayer) schemaPath(module, name string) (string, error) {
	if !identPattern.MatchString(module) || !identPattern.MatchString(name) {
		return "", fmt.Errorf("%w: %q/%q is not a module and form name", ErrInvalidName, module, name)
	}
	return filepath.Join(l.dir, module, name+".json"), nil
}

// Read returns the override document, and false when there is none.
func (l *FileLayer) Read(module, name string) ([]byte, bool, error) {
	p, err := l.schemaPath(module, name)
	if err != nil {
		return nil, false, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// CheckDocument validates a form definition for module/name: both names plain
// identifiers (ErrInvalidName), the size within MaxSchemaBytes and the
// resolver's validator satisfied (ErrRejected, with the validator's message).
// It is the one definition of what an override document may be, shared by the
// filesystem layer and the database store (internal/overrides).
func (r *Resolver) CheckDocument(module, name string, data []byte) error {
	if !identPattern.MatchString(module) || !identPattern.MatchString(name) {
		return fmt.Errorf("%w: %q/%q is not a module and form name", ErrInvalidName, module, name)
	}
	if len(data) == 0 || len(data) > MaxSchemaBytes {
		return fmt.Errorf("%w: a form definition must be between 1 byte and %d bytes", ErrRejected, MaxSchemaBytes)
	}
	if err := r.Validate(data); err != nil {
		return fmt.Errorf("%w: %w", ErrRejected, err)
	}
	return nil
}

// Save validates data and writes it as the override of module/name, then
// reloads the layer. base is the embedded document it replaces; when it is not
// nil the returned warnings say what the override removes or changes
// (Compat). The warnings are advice, not a gate.
func (l *FileLayer) Save(module, name string, data, base []byte) ([]string, error) {
	return l.SaveAs(module, name, data, base, 0, "")
}

// SaveAs is Save with who made the change and a note, for the history
// (filehistory.go): the version is recorded after the definition is written
// and reloaded, and a failure to record it is returned although it is live.
func (l *FileLayer) SaveAs(module, name string, data, base []byte, by int64, note string) ([]string, error) {
	if err := l.r.CheckDocument(module, name, data); err != nil {
		return nil, err
	}
	p, err := l.schemaPath(module, name)
	if err != nil {
		return nil, err
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, data, "", "  "); err != nil {
		return nil, err
	}
	pretty.WriteByte('\n')
	var warnings []string
	if base != nil {
		warnings = Compat(base, data)
	}

	// The reload runs under the lock too, so it never meets another writer
	// halfway through a save.
	writeMu.Lock()
	defer writeMu.Unlock()
	previous, rerr := os.ReadFile(p)
	had := rerr == nil
	if err := writeAtomic(p, pretty.Bytes()); err != nil {
		return nil, err
	}
	if err := l.reloadWritten(); err != nil {
		return warnings, fmt.Errorf("schemaoverlay: written, but the layer did not reload: %w", err)
	}
	return warnings, l.recordLocked(module, name, pretty.Bytes(), false, note, by, had, previous)
}

// Remove deletes the override of module/name and reloads. Removing an absent
// override is not an error.
func (l *FileLayer) Remove(module, name string) error {
	return l.RemoveAs(module, name, 0)
}

// RemoveAs is Remove with who made the change, for the history.
func (l *FileLayer) RemoveAs(module, name string, by int64) error {
	p, err := l.schemaPath(module, name)
	if err != nil {
		return err
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	previous, rerr := os.ReadFile(p)
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := l.reloadWritten(); err != nil {
		return fmt.Errorf("schemaoverlay: removed, but the layer did not reload: %w", err)
	}
	if rerr != nil {
		return nil // nothing was there: no version
	}
	return l.recordLocked(module, name, nil, true, "removed", by, true, previous)
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // a no-op once renamed
	if _, err := tmp.Write(data); err != nil {
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
