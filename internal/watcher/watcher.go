package watcher

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Event represents a file change event.
type Event struct {
	Path string
	Op   string // "create", "write", "remove"
}

// DefaultPollInterval is the default polling interval for file change detection.
const DefaultPollInterval = 500 * time.Millisecond

// Watcher watches directories for file changes using a polling approach.
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

// SetPollInterval sets the polling interval for file change detection.
func (w *Watcher) SetPollInterval(d time.Duration) {
	w.pollInterval = d
}

// Watch starts polling for file changes. This is a blocking call that runs
// until Stop() is called. Uses a polling approach for simplicity and
// cross-platform compatibility.
func (w *Watcher) Watch() error {
	// Build initial snapshot
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
				w.mu.Lock()
				w.pending = append(w.pending, events...)
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
			snapshot = newSnapshot
		}
	}
}

// Stop stops the watcher.
func (w *Watcher) Stop() {
	close(w.stopCh)
}

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
			// Check if it has a matching extension
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

	// Check for new or modified files
	for path, newInfo := range new {
		if oldInfo, ok := old[path]; ok {
			if newInfo.modTime != oldInfo.modTime || newInfo.size != oldInfo.size {
				events = append(events, Event{Path: path, Op: "write"})
			}
		} else {
			events = append(events, Event{Path: path, Op: "create"})
		}
	}

	// Check for deleted files
	for path := range old {
		if _, ok := new[path]; !ok {
			events = append(events, Event{Path: path, Op: "remove"})
		}
	}

	return events
}
