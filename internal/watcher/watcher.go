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

	"github.com/microsoft/typescript-go/shim/fswatch"
)

// Event represents a file change event.
type Event struct {
	Path string
	Op   string // "create", "write", "remove"
}

// DefaultPollInterval is the default polling interval for the polling fallback.
const DefaultPollInterval = 500 * time.Millisecond

// maxOverflowFailures is the number of consecutive overflow recovery attempts
// allowed before the watcher gives up on the native backend and falls back to
// polling.
const maxOverflowFailures = 2

// rootLivenessPollInterval is how often each watch root is Stat'ed to detect
// rename/removal. Kept as a platform-proof belt on top of fswatch's own
// ErrWatchTerminated delivery; a silently dead watch is the worst failure
// mode for a dev loop.
const rootLivenessPollInterval = 500 * time.Millisecond

// errFallbackToPolling signals the Watch loop that overflow recovery gave up
// and the polling backend should take over.
var errFallbackToPolling = errors.New("watcher: falling back to polling")

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
// It uses typescript-go's fswatch (a Go port of @parcel/watcher: FSEvents on
// macOS, fanotify/inotify on Linux, ReadDirectoryChangesW on Windows) for
// event-driven detection. If the backend is unavailable or repeatedly
// overflows, it falls back to polling automatically and warns the user.
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
// until Stop() is called. Tries the native fswatch backend first, falls back
// to polling if the backend is unavailable, setup fails (OS watch limit), or
// it reports too many overflows in a row.
func (w *Watcher) Watch() error {
	err := w.watchNative()
	if err == nil && !w.shouldFallback() {
		return nil
	}
	// Don't fall back to polling when a watch root vanished — polling the
	// missing path would also produce nothing, and the user needs to know.
	if _, gone := err.(*ErrWatchRootGone); gone {
		return err
	}
	// Native backend failed (or asked us to fall back after repeated
	// overflows); run the polling backend.
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
// roots is renamed or removed while the watcher is running. From that point
// on file changes would go undetected, so a clear error surfaces rather
// than appear to keep working.
type ErrWatchRootGone struct {
	Path string
}

func (e *ErrWatchRootGone) Error() string {
	return fmt.Sprintf("watch root %q was renamed or removed; restart tsgonest dev", e.Path)
}

// watchNative subscribes each watch root recursively via fswatch and blocks
// until Stop, a terminal watch error, or an overflow-triggered fallback.
//
// fswatch handles the messy parts the old hand-rolled backend needed:
// per-subdirectory watch registration, coalescing bursts, symlinked roots
// (events are delivered under the caller-visible path), and root-deletion
// detection (ErrWatchTerminated).
func (w *Watcher) watchNative() error {
	backend := fswatch.Default()
	if !backend.Available() {
		return fswatch.ErrUnavailable
	}

	// Snapshot for overflow recovery and create/write disambiguation.
	w.snapMu.Lock()
	w.prevSnapshot = w.buildSnapshot()
	w.snapMu.Unlock()

	fatalCh := make(chan error, 1)
	fatal := func(err error) {
		select {
		case fatalCh <- err:
		default:
		}
	}

	var watches []fswatch.Watch
	closeAll := func() {
		for _, wt := range watches {
			wt.Close()
		}
	}

	for _, dir := range w.dirs {
		dir := dir
		abs, err := filepath.Abs(dir)
		if err != nil {
			closeAll()
			return err
		}
		wt, err := backend.WatchDirectory(abs, func(events []fswatch.Event, cbErr error) {
			w.handleBatch(dir, events, cbErr, fatal)
		}, fswatch.WithRecursive(), fswatch.WithIgnore(shouldSkipPath))
		if err != nil {
			closeAll()
			if errors.Is(err, syscall.ENOSPC) {
				printWatchLimitWarning()
			}
			return err
		}
		watches = append(watches, wt)
	}
	defer closeAll()

	// Poll-based root liveness check, in addition to fswatch's own
	// ErrWatchTerminated: a periodic os.Stat is platform-proof against
	// missed or coalesced root rename/delete events.
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
				for _, dir := range w.dirs {
					resolved := dir
					if r, err := filepath.EvalSymlinks(dir); err == nil {
						resolved = r
					}
					if _, err := os.Stat(resolved); err != nil && os.IsNotExist(err) {
						select {
						case rootGone <- dir:
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
		case err := <-fatalCh:
			if errors.Is(err, errFallbackToPolling) {
				return nil // fallbackToPolling flag already set
			}
			return err
		}
	}
}

// handleBatch is the fswatch callback for one watch root. It runs on a
// library goroutine and is never invoked concurrently with itself for the
// same watch. Events delivered alongside an error are still processed.
func (w *Watcher) handleBatch(root string, events []fswatch.Event, err error, fatal func(error)) {
	for _, ev := range events {
		w.handleEvent(ev)
	}
	if err == nil {
		return
	}
	if isOverflowError(err) {
		if w.handleOverflow() {
			fatal(errFallbackToPolling)
		}
		return
	}
	if errors.Is(err, fswatch.ErrWatchTerminated) {
		fatal(&ErrWatchRootGone{Path: root})
		return
	}
	fmt.Fprintf(os.Stderr, "watcher: %v\n", err)
}

func (w *Watcher) handleEvent(ev fswatch.Event) {
	path := ev.Path
	if shouldSkipPath(path) {
		return
	}

	if ev.Kind == fswatch.EventDelete {
		if !w.matchesExtension(path) {
			return
		}
		w.snapMu.Lock()
		if w.prevSnapshot != nil {
			delete(w.prevSnapshot, path)
		}
		w.snapMu.Unlock()
		w.addPending(Event{Path: path, Op: "remove"})
		return
	}

	if !w.matchesExtension(path) {
		// A directory appearing (e.g. git checkout renaming a fully
		// populated package into the watch root) may carry files whose
		// events predate the subscription. Synthesize creates for any
		// files not yet in the snapshot.
		if info, statErr := os.Stat(path); statErr == nil && info.IsDir() {
			w.synthesizeCreatesForNewFiles(path)
		}
		return
	}

	w.snapMu.Lock()
	prev, existed := w.prevSnapshot[path]
	w.snapMu.Unlock()
	op := "write"
	if !existed {
		op = "create"
	}
	// FSEvents can replay events from just before subscription (a known
	// parcel-watcher quirk). If the file's stat matches the snapshot, nothing
	// actually changed; drop the stale replay.
	if info, statErr := os.Stat(path); statErr == nil && existed &&
		info.ModTime().Equal(prev.modTime) && info.Size() == prev.size {
		return
	}
	w.recordSnapshotEntry(path)
	w.addPending(Event{Path: path, Op: op})
}

// synthesizeCreatesForNewFiles walks a freshly-appeared directory and pushes
// synthetic create events for matching files the snapshot has never seen.
// The snapshot check deduplicates against events fswatch delivers itself.
func (w *Watcher) synthesizeCreatesForNewFiles(root string) {
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
		if shouldSkipPath(path) || !w.matchesExtension(path) {
			return nil
		}
		w.snapMu.Lock()
		_, known := w.prevSnapshot[path]
		w.snapMu.Unlock()
		if known {
			return nil
		}
		w.recordSnapshotEntry(path)
		w.addPending(Event{Path: path, Op: "create"})
		return nil
	})
}

// handleOverflow runs the snapshot-diff recovery and tracks consecutive
// failures. Returns true if the watcher should fall back to polling.
func (w *Watcher) handleOverflow() bool {
	recovered := w.recoverFromOverflow()

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
			"watcher: overflow recovery failed %d times; falling back to polling\n",
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
// downstream. fswatch keeps its own subdirectory watches alive across an
// overflow, so no re-registration is needed here.
//
// Returns true on a clean re-walk (snapshot replaced).
func (w *Watcher) recoverFromOverflow() bool {
	newSnapshot := w.buildSnapshot()

	w.snapMu.Lock()
	prev := w.prevSnapshot
	w.prevSnapshot = newSnapshot
	w.snapMu.Unlock()

	for _, ev := range w.diff(prev, newSnapshot) {
		w.addPending(ev)
	}
	return true
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

// isOverflowError reports whether err is a kernel-level overflow signal.
// fswatch surfaces these as ErrOverflow ("some changes were missed; rescan").
// ENOSPC (inotify watch-descriptor exhaustion) and string fallbacks are kept
// for defense in depth across backends.
func isOverflowError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, fswatch.ErrOverflow) {
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
// transitioning after an overflow fallback, it seeds from the last-known
// snapshot so the initial poll doesn't synthesize spurious "create" events
// for every file already on disk.
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
		// Resolve symlinks/junctions at the root so the walk covers the real
		// target instead of no-op'ing on a symlinked watch root (filepath.Walk
		// uses Lstat). Keys are rebased back onto the caller-visible root so
		// they compare equal to fswatch event paths, which are delivered under
		// the path the caller subscribed (e.g. /var/... vs /private/var/... on
		// macOS). Nested symlinks are still left alone.
		orig := dir
		resolved := dir
		if r, err := filepath.EvalSymlinks(dir); err == nil {
			resolved = r
		}
		filepath.Walk(resolved, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				base := filepath.Base(path)
				if path != resolved && (skipDirs[base] || (strings.HasPrefix(base, ".") && base != ".")) {
					return filepath.SkipDir
				}
				return nil
			}
			ext := filepath.Ext(path)
			for _, e := range w.extensions {
				if strings.EqualFold(ext, e) {
					key := path
					if resolved != orig && strings.HasPrefix(path, resolved) {
						key = orig + path[len(resolved):]
					}
					snap[key] = fileInfo{modTime: info.ModTime(), size: info.Size()}
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
