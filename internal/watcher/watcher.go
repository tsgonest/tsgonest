package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// skipDirs are directory base names that should never be watched or walked.
// These are either non-source content or hidden directories that produce
// noise (node_modules, .git) or are too large to watch efficiently.
var skipDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	".next":        true,
	".turbo":       true,
	"dist":         true,
	"build":        true,
	"coverage":     true,
	"__pycache__":  true,
}

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
	// Don't fall back to polling when a watch root vanished — polling the
	// missing path would also produce nothing, and the user needs to know.
	if _, gone := err.(*ErrWatchRootGone); gone {
		return err
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

// ErrWatchRootGone is returned by Watch when one of the configured watch
// roots is renamed or removed while the watcher is running. fsnotify's
// behavior in this case is platform-dependent and silent: macOS (kqueue)
// stops delivering events, Linux (inotify) auto-removes the watch. Either
// way, file changes go undetected from that point on, so we surface a
// clear error rather than appear to keep working.
type ErrWatchRootGone struct {
	Path string
}

func (e *ErrWatchRootGone) Error() string {
	return fmt.Sprintf("watch root %q was renamed or removed; restart tsgonest dev", e.Path)
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

	// Capture the watch roots, normalized, so we can detect when one of
	// them is renamed or removed mid-flight. Symlink-resolved paths are
	// preferred because fsnotify reports events using the resolved path
	// on platforms where the watch was added via a symlinked input.
	roots := make(map[string]string, len(w.dirs))
	for _, dir := range w.dirs {
		roots[normalizeWatchPath(dir)] = dir
	}

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
			// Ignore Chmod-only events — these fire on all platforms for
			// non-content changes: macOS Spotlight/xattr, Windows Defender
			// scans, Linux permission/ownership changes. Only suppress when
			// Chmod is the sole operation; combined Write|Chmod passes through.
			if ev.Op == fsnotify.Chmod {
				continue
			}

			if ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename) {
				if rootPath, gone := matchedRoot(roots, ev.Name); gone {
					return &ErrWatchRootGone{Path: rootPath}
				}
			}

			// Skip events from directories that shouldn't be watched
			// (e.g., node_modules created inside src/ at runtime).
			if shouldSkipPath(ev.Name) {
				continue
			}

			// Filter by extension
			if !w.matchesExtension(ev.Name) {
				// New directories need to be watched for new files
				if ev.Has(fsnotify.Create) {
					if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
						if addErr := w.addRecursive(fsw, ev.Name); addErr == nil {
							w.synthesizeCreatesForExistingFiles(ev.Name)
						}
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
// fsnotify watcher. Skips directories in the skipDirs set and hidden
// directories (prefixed with "."). Returns an error if adding any
// directory fails (typically ENOSPC on Linux when the inotify limit is hit).
//
// The root is resolved through filepath.EvalSymlinks so that watch roots
// which are themselves symlinks or Windows junctions (mklink /J, OneDrive
// Documents/Desktop redirection, monorepo layout symlinks) are walked as
// their real targets. filepath.Walk uses os.Lstat, which reports symlinks
// as non-directories regardless of target — without this resolution the
// walk would no-op silently and no fsnotify.Add would ever run.
// Symlinks discovered nested inside the tree are NOT followed, to avoid
// cycles and double-watches.
func (w *Watcher) addRecursive(fsw *fsnotify.Watcher, root string) error {
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible paths
		}
		if info.IsDir() {
			base := filepath.Base(path)
			// Skip known non-source directories and hidden dirs (except the root itself)
			if path != root && (skipDirs[base] || (strings.HasPrefix(base, ".") && base != ".")) {
				return filepath.SkipDir
			}
			if addErr := fsw.Add(path); addErr != nil {
				printWatchLimitWarning()
				return addErr
			}
		}
		return nil
	})
}

// synthesizeCreatesForExistingFiles walks a freshly-attached directory and
// pushes synthetic Create events for every file already inside that matches
// the configured extensions. This is the fix for atomically populated
// directories (e.g., `git checkout` / `git pull` renaming a fully-populated
// package directory into the watch root): the kernel-level Create events for
// those pre-existing files happened before fsnotify was watching the dir, so
// fsnotify never reported them. Without synthesis, dev mode would silently
// miss the new files until they were touched.
func (w *Watcher) synthesizeCreatesForExistingFiles(root string) {
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != root {
				base := filepath.Base(path)
				if skipDirs[base] || (strings.HasPrefix(base, ".") && base != ".") {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if shouldSkipPath(path) {
			return nil
		}
		if !w.matchesExtension(path) {
			return nil
		}
		w.addPending(Event{Path: path, Op: "create"})
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
		if strings.EqualFold(ext, e) {
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

// normalizeWatchPath returns a canonical absolute form of a watch root
// for comparison against fsnotify event paths. fsnotify reports event
// paths as they were passed to Add (no symlink resolution), so we only
// Abs+Clean here — resolving symlinks would make /var/foo (the path the
// user added on macOS) miss events reported under that same /var/foo.
func normalizeWatchPath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(p)
}

// matchedRoot reports whether eventPath equals one of the registered
// watch roots, returning the original (unnormalized) root path so error
// messages reflect what the user passed in.
func matchedRoot(roots map[string]string, eventPath string) (string, bool) {
	norm := normalizeWatchPath(eventPath)
	if orig, ok := roots[norm]; ok {
		return orig, true
	}
	return "", false
}

// shouldSkipPath returns true if the path contains a directory segment
// that should be ignored (e.g., node_modules, .git).
// Normalizes separators to "/" before matching so that Windows native paths
// (backslash) and Git Bash / forward-slash paths both match correctly.
// Comparison is case-insensitive so Windows paths like NODE_MODULES or Dist
// are treated the same as their lowercase equivalents (skipDirs keys are
// already lowercase).
func shouldSkipPath(path string) bool {
	normalized := strings.ToLower(filepath.ToSlash(path))
	for dir := range skipDirs {
		seg := "/" + dir + "/"
		if strings.Contains(normalized, seg) ||
			strings.HasSuffix(normalized, "/"+dir) ||
			normalized == dir ||
			strings.HasPrefix(normalized, dir+"/") {
			return true
		}
	}
	return false
}

// --- Polling fallback helpers (also used by unit tests) ---

type fileInfo struct {
	modTime time.Time
	size    int64
}

func (w *Watcher) buildSnapshot() map[string]fileInfo {
	snap := make(map[string]fileInfo)
	for _, dir := range w.dirs {
		// Mirror addRecursive: resolve symlinks/junctions at the root so the
		// polling fallback walks the real target instead of no-op'ing on a
		// symlinked watch root. Nested symlinks are still left alone.
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			dir = resolved
		}
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				base := filepath.Base(path)
				if path != dir && (skipDirs[base] || (strings.HasPrefix(base, ".") && base != ".")) {
					return filepath.SkipDir
				}
				return nil
			}
			ext := filepath.Ext(path)
			for _, e := range w.extensions {
				if strings.EqualFold(ext, e) {
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
