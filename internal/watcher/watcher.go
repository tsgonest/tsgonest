package watcher

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
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

// maxOverflowFailures is the number of consecutive overflow recovery attempts
// allowed before the watcher gives up on fsnotify and falls back to polling.
const maxOverflowFailures = 2

// rootLivenessPollInterval is how often the fsnotify backend Stats each watch
// root to detect rename/removal. Required because Windows
// ReadDirectoryChangesW does not emit any event for the watched directory
// itself when it is renamed or deleted.
const rootLivenessPollInterval = 500 * time.Millisecond

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

	snapMu            sync.Mutex
	prevSnapshot      map[string]fileInfo
	overflowFailures  int
	fallbackToPolling bool
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
// the OS watch limit is exhausted or if fsnotify reports too many buffer
// overflows in a row (Windows ReadDirectoryChangesW or inotify queue).
func (w *Watcher) Watch() error {
	err := w.watchFsnotify()
	if err == nil && !w.shouldFallback() {
		return nil
	}
	// Don't fall back to polling when a watch root vanished — polling the
	// missing path would also produce nothing, and the user needs to know.
	if _, gone := err.(*ErrWatchRootGone); gone {
		return err
	}
	// fsnotify failed (or asked us to fall back after repeated overflows) —
	// run the polling backend.
	return w.watchPolling()
}

func (w *Watcher) shouldFallback() bool {
	w.snapMu.Lock()
	defer w.snapMu.Unlock()
	return w.fallbackToPolling
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
//
// On a buffer-overflow error from the kernel (Windows ERROR_NOTIFY_ENUM_DIR
// surfaced as fsnotify.ErrEventOverflow, or inotify IN_Q_OVERFLOW), the
// watcher re-walks the watched roots and synthesizes events for any files
// whose modTime/size differs from the in-memory snapshot. After
// maxOverflowFailures consecutive failed recoveries it gives up and lets
// Watch() fall back to polling.
func (w *Watcher) watchFsnotify() error {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer fsw.Close()

	// Capture the watch roots, normalized, so we can detect when one of
	// them is renamed or removed mid-flight. The key is symlink-resolved
	// because addRecursive passes the EvalSymlinks-resolved path to
	// fsnotify, and fsnotify reports event paths exactly as added — so the
	// lookup side must match. The map value is the original (unresolved)
	// dir so error messages show what the user configured.
	roots := make(map[string]string, len(w.dirs))
	rootResolved := make([][2]string, 0, len(w.dirs))
	for _, dir := range w.dirs {
		resolved := dir
		if r, err := filepath.EvalSymlinks(dir); err == nil {
			resolved = r
		}
		roots[normalizeWatchPath(resolved)] = dir
		rootResolved = append(rootResolved, [2]string{resolved, dir})
	}

	// Recursively add all directories under each watched root.
	for _, dir := range w.dirs {
		if err := w.addRecursive(fsw, dir); err != nil {
			return err
		}
	}

	w.snapMu.Lock()
	w.prevSnapshot = w.buildSnapshot()
	w.snapMu.Unlock()

	// Poll-based root liveness check. fsnotify's Rename/Remove signal for the
	// watched root itself is platform-dependent: Linux (inotify) emits
	// IN_MOVE_SELF/IN_DELETE_SELF, but Windows ReadDirectoryChangesW watches
	// by HANDLE — renaming or removing the watched directory does NOT generate
	// any event for the dir itself. A periodic os.Stat covers Windows and
	// hardens the other platforms against missed/coalesced events.
	rootGone := make(chan string, 1)
	pollDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(rootLivenessPollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-pollDone:
				return
			case <-ticker.C:
				for _, pair := range rootResolved {
					if _, err := os.Stat(pair[0]); err != nil && os.IsNotExist(err) {
						select {
						case rootGone <- pair[1]:
						default:
						}
						return
					}
				}
			}
		}
	}()
	defer close(pollDone)

	for {
		select {
		case <-w.stopCh:
			return nil

		case path := <-rootGone:
			return &ErrWatchRootGone{Path: path}

		case ev, ok := <-fsw.Events:
			if !ok {
				return nil
			}
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

			if !w.matchesExtension(ev.Name) {
				if ev.Has(fsnotify.Create) {
					if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
						if addErr := w.addRecursive(fsw, ev.Name); addErr == nil {
							w.synthesizeCreatesForExistingFiles(ev.Name)
						}
					}
				}
				continue
			}

			w.recordSnapshotEntry(ev.Name)
			event := Event{Path: ev.Name, Op: fsnotifyOpToString(ev.Op)}
			w.addPending(event)

		case err, ok := <-fsw.Errors:
			if !ok {
				return nil
			}
			if isOverflowError(err) {
				if w.handleOverflow(fsw) {
					return nil
				}
				continue
			}
			fmt.Fprintf(os.Stderr, "watcher: %v\n", err)
		}
	}
}

// handleOverflow runs the snapshot-diff recovery and tracks consecutive
// failures. Returns true if the caller should exit watchFsnotify so the
// outer Watch() can fall back to polling.
func (w *Watcher) handleOverflow(fsw *fsnotify.Watcher) bool {
	recovered := w.recoverFromOverflow(fsw)

	w.snapMu.Lock()
	if recovered {
		w.overflowFailures = 0
	} else {
		w.overflowFailures++
	}
	failures := w.overflowFailures
	w.snapMu.Unlock()

	if failures >= maxOverflowFailures {
		fmt.Fprintf(os.Stderr,
			"watcher: fsnotify overflow recovery failed %d times; falling back to polling\n",
			failures)
		w.snapMu.Lock()
		w.fallbackToPolling = true
		w.snapMu.Unlock()
		return true
	}
	return false
}

// recoverFromOverflow re-walks every watched root, diffs against the
// in-memory snapshot, and pushes synthesized Create/Write/Remove events
// downstream. It also re-attaches fsw to any directories that appeared
// since the last walk so subsequent events do not get dropped.
//
// Returns true on a clean re-walk (no errors, snapshot replaced). Returns
// false if any root failed to walk — the caller treats this as a recovery
// failure and may fall back to polling on repeated misses.
func (w *Watcher) recoverFromOverflow(fsw *fsnotify.Watcher) bool {
	ok := true
	if fsw != nil {
		for _, dir := range w.dirs {
			if err := w.addRecursive(fsw, dir); err != nil {
				ok = false
			}
		}
	}

	newSnapshot := w.buildSnapshot()

	w.snapMu.Lock()
	prev := w.prevSnapshot
	w.prevSnapshot = newSnapshot
	w.snapMu.Unlock()

	for _, ev := range w.diff(prev, newSnapshot) {
		w.addPending(ev)
	}
	return ok
}

func (w *Watcher) recordSnapshotEntry(path string) {
	info, err := os.Stat(path)
	if err != nil {
		w.snapMu.Lock()
		if w.prevSnapshot != nil {
			delete(w.prevSnapshot, path)
		}
		w.snapMu.Unlock()
		return
	}
	if info.IsDir() {
		return
	}
	w.snapMu.Lock()
	if w.prevSnapshot == nil {
		w.prevSnapshot = make(map[string]fileInfo)
	}
	w.prevSnapshot[path] = fileInfo{modTime: info.ModTime(), size: info.Size()}
	w.snapMu.Unlock()
}

// isOverflowError reports whether err is a kernel-level overflow signal
// from any fsnotify backend. Windows ReadDirectoryChangesW reports
// ERROR_NOTIFY_ENUM_DIR (1022) when its 64KB buffer overflows during a
// burst (large git checkout, pnpm install). Linux inotify reports
// IN_Q_OVERFLOW when the per-instance event queue fills. Both surface as
// fsnotify.ErrEventOverflow; we also accept ENOSPC on Linux (inotify
// watch-descriptor exhaustion) and a string fallback for any future
// backend that wraps differently.
func isOverflowError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, fsnotify.ErrEventOverflow) {
		return true
	}
	if errors.Is(err, syscall.ENOSPC) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "overflow") ||
		strings.Contains(msg, "notify_enum_dir") ||
		strings.Contains(msg, "error 1022")
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

// watchPolling uses the original polling approach as a fallback. When
// transitioning from fsnotify after an overflow fallback, it seeds from the
// last-known fsnotify snapshot so the initial poll doesn't synthesize
// spurious "create" events for every file already on disk.
func (w *Watcher) watchPolling() error {
	w.snapMu.Lock()
	snapshot := w.prevSnapshot
	w.snapMu.Unlock()
	if snapshot == nil {
		snapshot = w.buildSnapshot()
	}

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
			w.snapMu.Lock()
			w.prevSnapshot = newSnapshot
			w.snapMu.Unlock()
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
