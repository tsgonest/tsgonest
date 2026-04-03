package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Event represents a file change event.
type Event struct {
	Path string
	Op   string // "create", "write", "remove"
}

// DefaultPollInterval is the default polling interval for the polling fallback.
const DefaultPollInterval = 500 * time.Millisecond

// Watcher watches directories for file changes.
// It uses fsnotify (inotify/kqueue/ReadDirectoryChangesW) for near-instant
// event-driven detection. If the OS watch limit is hit, it falls back to
// polling automatically and warns the user.
type Watcher struct {
	dirs         []string
	extensions   []string // e.g., [".ts", ".tsx"]
	debounce     time.Duration
	pollInterval time.Duration
	onChange     func(events []Event)

	mu      sync.Mutex
	pending []Event
	timer   *time.Timer
	stopCh  chan struct{}
}

// New creates a new file watcher.
func New(dirs []string, extensions []string, debounce time.Duration, onChange func(events []Event)) *Watcher {
	return &Watcher{
		dirs:         dirs,
		extensions:   extensions,
		debounce:     debounce,
		pollInterval: DefaultPollInterval,
		onChange:     onChange,
		stopCh:       make(chan struct{}),
	}
}

// SetPollInterval sets the polling interval for the polling fallback.
func (w *Watcher) SetPollInterval(d time.Duration) {
	w.pollInterval = d
}

// Watch starts watching for file changes. This is a blocking call that runs
// until Stop() is called. Tries fsnotify first, falls back to polling if
// the OS watch limit is exhausted.
func (w *Watcher) Watch() error {
	err := w.watchFsnotify()
	if err == nil {
		return nil
	}
	// fsnotify failed — fall back to polling
	return w.watchPolling()
}

// Stop stops the watcher.
func (w *Watcher) Stop() {
	select {
	case <-w.stopCh:
		// already closed
	default:
		close(w.stopCh)
	}
}

// watchFsnotify uses OS-level file system notifications for near-instant
// change detection. Returns a non-nil error if setup fails (e.g., watch
// limit exhausted), in which case the caller should fall back to polling.
func (w *Watcher) watchFsnotify() error {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer fsw.Close()

	// Recursively add all directories under each watched root.
	for _, dir := range w.dirs {
		if err := w.addRecursive(fsw, dir); err != nil {
			return err
		}
	}

	for {
		select {
		case <-w.stopCh:
			return nil

		case ev, ok := <-fsw.Events:
			if !ok {
				return nil
			}
			// Filter by extension
			if !w.matchesExtension(ev.Name) {
				// New directories need to be watched for new files
				if ev.Has(fsnotify.Create) {
					if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
						w.addRecursive(fsw, ev.Name)
					}
				}
				continue
			}

			event := Event{Path: ev.Name, Op: fsnotifyOpToString(ev.Op)}
			w.addPending(event)

		case err, ok := <-fsw.Errors:
			if !ok {
				return nil
			}
			// Log but don't crash — transient errors are common
			fmt.Fprintf(os.Stderr, "watcher: %v\n", err)
		}
	}
}

// addRecursive walks a directory tree and adds each directory to the
// fsnotify watcher. Returns an error if adding any directory fails
// (typically ENOSPC on Linux when the inotify limit is hit).
func (w *Watcher) addRecursive(fsw *fsnotify.Watcher, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible paths
		}
		if info.IsDir() {
			if addErr := fsw.Add(path); addErr != nil {
				printWatchLimitWarning()
				return addErr
			}
		}
		return nil
	})
}

// printWatchLimitWarning prints a one-time warning with a link to
// platform-specific instructions for increasing OS watch limits.
var printWatchLimitWarning = sync.OnceFunc(func() {
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "warning: OS file watch limit reached — falling back to polling.")
	fmt.Fprintln(os.Stderr, "Polling works but uses more CPU and has higher latency (~500ms).")
	fmt.Fprintln(os.Stderr, "To fix, increase your OS watch limit:")
	fmt.Fprintln(os.Stderr, "  https://github.com/paulmillr/chokidar#performance")
	fmt.Fprintln(os.Stderr, "")
})

// watchPolling uses the original polling approach as a fallback.
func (w *Watcher) watchPolling() error {
	snapshot := w.buildSnapshot()

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return nil
		case <-ticker.C:
			newSnapshot := w.buildSnapshot()
			events := w.diff(snapshot, newSnapshot)
			if len(events) > 0 {
				for _, ev := range events {
					w.addPending(ev)
				}
			}
			snapshot = newSnapshot
		}
	}
}

// addPending adds an event to the pending buffer and resets the debounce timer.
func (w *Watcher) addPending(ev Event) {
	w.mu.Lock()
	w.pending = append(w.pending, ev)
	if w.timer != nil {
		w.timer.Stop()
	}
	w.timer = time.AfterFunc(w.debounce, func() {
		w.mu.Lock()
		pending := w.pending
		w.pending = nil
		w.mu.Unlock()
		if len(pending) > 0 {
			w.onChange(pending)
		}
	})
	w.mu.Unlock()
}

// Pending returns true if there are buffered events waiting for the debounce
// timer to fire. Callers can use this to check whether more file changes are
// in-flight before taking action (e.g., restarting a child process).
func (w *Watcher) Pending() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.pending) > 0
}

func (w *Watcher) matchesExtension(path string) bool {
	ext := filepath.Ext(path)
	for _, e := range w.extensions {
		if ext == e {
			return true
		}
	}
	return false
}

func fsnotifyOpToString(op fsnotify.Op) string {
	switch {
	case op.Has(fsnotify.Create):
		return "create"
	case op.Has(fsnotify.Remove) || op.Has(fsnotify.Rename):
		return "remove"
	default:
		return "write"
	}
}

// --- Polling fallback helpers (also used by unit tests) ---

type fileInfo struct {
	modTime time.Time
	size    int64
}

func (w *Watcher) buildSnapshot() map[string]fileInfo {
	snap := make(map[string]fileInfo)
	for _, dir := range w.dirs {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			ext := filepath.Ext(path)
			for _, e := range w.extensions {
				if ext == e {
					snap[path] = fileInfo{modTime: info.ModTime(), size: info.Size()}
					break
				}
			}
			return nil
		})
	}
	return snap
}

func (w *Watcher) diff(old, new map[string]fileInfo) []Event {
	var events []Event

	for path, newInfo := range new {
		if oldInfo, ok := old[path]; ok {
			if newInfo.modTime != oldInfo.modTime || newInfo.size != oldInfo.size {
				events = append(events, Event{Path: path, Op: "write"})
			}
		} else {
			events = append(events, Event{Path: path, Op: "create"})
		}
	}

	for path := range old {
		if _, ok := new[path]; !ok {
			events = append(events, Event{Path: path, Op: "remove"})
		}
	}

	return events
}
