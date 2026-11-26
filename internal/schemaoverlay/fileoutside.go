package schemaoverlay

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Changes made to the override directory by something other than the editors
// (a text editor, a copy, a checkout) are part of the history too: each load
// that finds a definition changed, added or removed records it as a version
// made outside (ChangedOutside), so it can be seen and restored like any other.

// ChangedOutside is the ChangedBy of a version the editors did not make: the
// file was changed, added or removed in the directory by something else.
const ChangedOutside int64 = -1

func sameDoc(a string, b []byte) bool {
	var x, y bytes.Buffer
	if json.Compact(&x, []byte(a)) != nil || json.Compact(&y, b) != nil {
		return strings.TrimSpace(a) == strings.TrimSpace(string(b))
	}
	return bytes.Equal(x.Bytes(), y.Bytes())
}

// lastVersions returns the newest recorded version of every definition.
func (l *FileLayer) lastVersions() (map[Key]FileVersion, error) {
	f, err := os.Open(l.historyPath())
	if errors.Is(err, os.ErrNotExist) {
		return map[Key]FileVersion{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[Key]FileVersion{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 4*MaxSchemaBytes)
	for sc.Scan() {
		var v FileVersion
		if json.Unmarshal(sc.Bytes(), &v) == nil && v.Module != "" {
			out[Key{Module: v.Module, Name: v.Name}] = v
		}
	}
	return out, sc.Err()
}

// recordOutsideLocked records, after a load, what changed in the directory
// without the editors: accepted is what the load put in force from the files,
// onDisk every well-formed file found, previous what was in force before
// (meaningful when wasLoaded). The caller holds writeMu.
//
// A definition that has a history is compared with its newest version, so a
// change made while the server was down is recorded at the next start. One
// without a history is compared with what the layer held before, and what it
// held is recorded first, as the editors do; the first load of a directory
// records nothing, since there is nothing to compare it with.
func (l *FileLayer) recordOutsideLocked(accepted, onDisk, previous map[Key][]byte, wasLoaded bool) error {
	lasts, err := l.lastVersions()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	var lines []FileVersion
	add := func(k Key, version int, doc []byte, removed bool) {
		lines = append(lines, FileVersion{Module: k.Module, Name: k.Name, Version: version, Doc: string(doc),
			Removed: removed, ChangedBy: ChangedOutside, ChangedAt: now})
	}
	baseline := func(k Key, doc []byte) {
		lines = append(lines, FileVersion{Module: k.Module, Name: k.Name, Version: 1, Doc: string(doc)})
	}
	for k, raw := range accepted {
		last, has := lasts[k]
		switch {
		case has:
			if last.Removed || !sameDoc(last.Doc, raw) {
				add(k, last.Version+1, raw, false)
			}
		case wasLoaded:
			if prev, had := previous[k]; !had {
				add(k, 1, raw, false)
			} else if !sameDoc(string(prev), raw) {
				baseline(k, prev)
				add(k, 2, raw, false)
			}
		}
	}
	for k, last := range lasts {
		if _, there := onDisk[k]; !there && !last.Removed {
			add(k, last.Version+1, nil, true)
		}
	}
	if wasLoaded {
		for k, prev := range previous {
			_, there := onDisk[k]
			if _, has := lasts[k]; !there && !has {
				baseline(k, prev)
				add(k, 2, nil, true)
			}
		}
	}
	if len(lines) == 0 {
		return nil
	}
	sort.Slice(lines, func(i, j int) bool {
		a, b := lines[i], lines[j]
		if a.Module != b.Module {
			return a.Module < b.Module
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Version < b.Version
	})
	p := l.historyPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	for _, line := range lines {
		data, err := json.Marshal(line)
		if err == nil {
			_, err = f.Write(append(data, '\n'))
		}
		if err != nil {
			f.Close()
			return err
		}
	}
	return f.Close()
}
