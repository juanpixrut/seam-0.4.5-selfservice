//go:build linux

package dirwatch

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Supported reports whether this build can watch a directory through events.
const Supported = true

const watchMask = syscall.IN_CLOSE_WRITE | syscall.IN_MOVED_TO | syscall.IN_MOVED_FROM | syscall.IN_DELETE | syscall.IN_CREATE

func watch(ctx context.Context, dir string, subdirs bool, onChange func()) bool {
	fd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC | syscall.IN_NONBLOCK)
	if err != nil {
		return false
	}
	root, err := syscall.InotifyAddWatch(fd, dir, watchMask)
	if err != nil {
		syscall.Close(fd)
		return false
	}
	// as a file of the runtime's poller, so that closing it ends a blocked read
	f := os.NewFile(uintptr(fd), "inotify")
	var mu sync.Mutex
	parents := map[int32]string{int32(root): dir} // watch descriptor -> directory
	addSub := func(name string) {
		if !subdirs || strings.HasPrefix(name, ".") {
			return
		}
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err != nil || !st.IsDir() {
			return
		}
		if wd, err := syscall.InotifyAddWatch(fd, p, watchMask); err == nil {
			mu.Lock()
			parents[int32(wd)] = p
			mu.Unlock()
		}
	}
	if subdirs {
		if entries, err := os.ReadDir(dir); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					addSub(e.Name())
				}
			}
		}
	}

	changed := make(chan struct{}, 1)
	go func() { // reader
		buf := make([]byte, 64*1024)
		for {
			n, err := f.Read(buf)
			if err != nil {
				return // closed
			}
			relevant := false
			for off := 0; off+syscall.SizeofInotifyEvent <= n; {
				wd := int32(binary.LittleEndian.Uint32(buf[off:]))
				mask := binary.LittleEndian.Uint32(buf[off+4:])
				nameLen := int(binary.LittleEndian.Uint32(buf[off+12:]))
				name := strings.TrimRight(string(buf[off+syscall.SizeofInotifyEvent:off+syscall.SizeofInotifyEvent+nameLen]), "\x00")
				off += syscall.SizeofInotifyEvent + nameLen
				if strings.HasPrefix(name, ".") {
					continue
				}
				mu.Lock()
				parent := parents[wd]
				mu.Unlock()
				if mask&syscall.IN_ISDIR != 0 {
					if parent == dir && mask&(syscall.IN_CREATE|syscall.IN_MOVED_TO) != 0 {
						addSub(name) // a new module directory: its files are watched from now on
						relevant = true
					}
					continue
				}
				relevant = true
			}
			if relevant {
				select {
				case changed <- struct{}{}:
				default:
				}
			}
		}
	}()
	go func() { // caller
		defer f.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case <-changed:
				select {
				case <-ctx.Done():
					return
				case <-time.After(Settle):
				}
				onChange()
			}
		}
	}()
	return true
}
