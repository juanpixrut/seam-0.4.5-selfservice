package i18n

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"sort"
	"time"
)

// Changes made to the override directory by something other than the editors
// (a text editor, a copy, a checkout) are part of the history too: each load
// that finds a text changed, added or removed records it as a version made
// outside (ChangedOutside), so it can be seen and restored like any other.

// ChangedOutside is the ChangedBy of a version the editors did not make.
const ChangedOutside int64 = -1

type textID struct{ locale, key string }

// lastVersions returns the newest recorded version of every text.
func (l *FileLayer) lastVersions() (map[textID]FileVersion, error) {
	f, err := os.Open(l.historyPath())
	if errors.Is(err, os.ErrNotExist) {
		return map[textID]FileVersion{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[textID]FileVersion{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		var v FileVersion
		if json.Unmarshal(sc.Bytes(), &v) == nil && v.Key != "" {
			out[textID{v.Locale, v.Key}] = v
		}
	}
	return out, sc.Err()
}

// recordOutsideLocked records, after a load, what changed in the directory
// without the editors: data is what the load put in force, previous what was
// in force before (meaningful when wasLoaded). The caller holds writeMu.
//
// A text that has a history is compared with its newest version, so a change
// made while the server was down is recorded at the next start. One without a
// history is compared with what the layer held before, and what it held is
// recorded first, as the editors do; the first load of a directory records
// nothing, since there is nothing to compare it with.
func (l *FileLayer) recordOutsideLocked(data, previous map[string]map[string]string, wasLoaded bool) error {
	lasts, err := l.lastVersions()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	var lines []FileVersion
	add := func(id textID, version int, value string, removed bool) {
		lines = append(lines, FileVersion{Locale: id.locale, Key: id.key, Version: version, Value: value,
			Removed: removed, ChangedBy: ChangedOutside, ChangedAt: now})
	}
	baseline := func(id textID, value string) {
		lines = append(lines, FileVersion{Locale: id.locale, Key: id.key, Version: 1, Value: value})
	}
	for locale, msgs := range data {
		for key, value := range msgs {
			id := textID{locale, key}
			last, has := lasts[id]
			switch {
			case has:
				if last.Removed || last.Value != value {
					add(id, last.Version+1, value, false)
				}
			case wasLoaded:
				if prev, had := previous[locale][key]; !had {
					add(id, 1, value, false)
				} else if prev != value {
					baseline(id, prev)
					add(id, 2, value, false)
				}
			}
		}
	}
	for id, last := range lasts {
		if _, there := data[id.locale][id.key]; !there && !last.Removed {
			add(id, last.Version+1, "", true)
		}
	}
	if wasLoaded {
		for locale, msgs := range previous {
			for key, prev := range msgs {
				id := textID{locale, key}
				_, there := data[locale][key]
				if _, has := lasts[id]; !there && !has {
					baseline(id, prev)
					add(id, 2, "", true)
				}
			}
		}
	}
	if len(lines) == 0 {
		return nil
	}
	sort.Slice(lines, func(i, j int) bool {
		a, b := lines[i], lines[j]
		if a.Locale != b.Locale {
			return a.Locale < b.Locale
		}
		if a.Key != b.Key {
			return a.Key < b.Key
		}
		return a.Version < b.Version
	})
	out := make([]any, len(lines))
	for i, line := range lines {
		out[i] = line
	}
	return appendHistoryLines(l.historyPath(), out...)
}
