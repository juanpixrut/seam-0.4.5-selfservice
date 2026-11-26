// Package dirwatch tells a caller when the files of a directory change, using
// the operating system's own change events where this build has them, with no
// dependency. It exists for the Developer module's override directories: their
// layers are polled, and events make the history of changes made in the files
// exact (two quick saves are two versions) and hot loading immediate, while the
// polling stays in place as the safety net.
//
// Supported reports whether events are available in this build: on Linux
// (inotify). Elsewhere Watch does nothing and returns false, and the caller's
// polling is all there is; macOS and network file systems are the reason the
// layers never rely on events alone.
package dirwatch

import (
	"context"
	"time"
)

// Settle is how long Watch waits after an event before calling back, so the
// several events of one save (create, write, rename) are one call.
const Settle = 20 * time.Millisecond

// Watch calls onChange after something changes in dir or, with subdirs, in
// its immediate sub-directories (including ones created later). Names that
// start with a dot are ignored: the layers' own history and the writers'
// temporary files must not look like a change. It returns at once, reporting
// whether it is watching; the watch ends when ctx is cancelled. onChange is
// called from one goroutine, never concurrently with itself.
func Watch(ctx context.Context, dir string, subdirs bool, onChange func()) bool {
	return watch(ctx, dir, subdirs, onChange)
}

// Mode resolves the developer_watch setting to what will be used: "events" or
// "poll". "" and "auto" mean events where supported.
func Mode(setting string) string {
	if setting == "poll" || !Supported {
		return "poll"
	}
	return "events"
}
