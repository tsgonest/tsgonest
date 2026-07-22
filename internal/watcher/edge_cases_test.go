package watcher

// Edge-case tests for the watcher. These exercise scenarios uncovered by the
// May 2026 dev-mode audit:
//
//   D. Renaming a watched root silently stops events (no error, no recovery).
//   E. Atomically populating a new subdirectory races against addRecursive,
//      so some files inside don't trigger events.
//   Leak. AfterFunc/backend goroutines surviving start/stop cycles.
//
// Tests marked KnownIssue are documented bugs that pass against the broken
// behavior. When a fix lands, these tests should be flipped to assert
// correctness (and the KnownIssue suffix dropped).

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// goroutineCount returns the current number of goroutines after a brief
// stabilization delay. Useful for leak assertions that should ignore
// short-lived goroutines spawned by the test harness itself.
func goroutineCount() int {
	// Two GCs flush goroutines that exited but haven't been reaped yet.
	runtime.GC()
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	return runtime.NumGoroutine()
}

// drainEvents collects all events delivered within the timeout. Used by tests
// that want to assert on what happened instead of just first-event-wins.
func drainEvents(ch <-chan []Event, timeout time.Duration) []Event {
	var all []Event
	deadline := time.After(timeout)
	for {
		select {
		case batch := <-ch:
			all = append(all, batch...)
		case <-deadline:
			return all
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// D. Watched-root rename
// ─────────────────────────────────────────────────────────────────────────────

// TestWatch_RootRename_SurfacesError verifies that when the watched root is
// renamed out from under the watcher, Watch returns an *ErrWatchRootGone
// instead of silently going deaf. Before the fix, the backend dropped events
// without any signal: macOS (kqueue) stopped delivering events, Linux
// (inotify) auto-removed the watch — either way `tsgonest dev` appeared to
// keep running but never rebuilt again.
func TestWatch_RootRename_SurfacesError(t *testing.T) {
	parent := t.TempDir()
	src := filepath.Join(parent, "src")
	if err := os.Mkdir(src, 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(src, "app.ts"), []byte("const a = 1;"), 0644)

	got := make(chan []Event, 16)
	w := New([]string{src}, []string{".ts"}, 50*time.Millisecond, func(events []Event) {
		got <- events
	})

	watchErr := make(chan error, 1)
	go func() { watchErr <- w.Watch() }()
	defer w.Stop()
	time.Sleep(200 * time.Millisecond)

	// Sanity: a normal write triggers an event.
	os.WriteFile(filepath.Join(src, "app.ts"), []byte("const a = 2;"), 0644)
	select {
	case <-got:
		// good
	case <-time.After(2 * time.Second):
		t.Fatal("baseline write was not detected; test setup is broken")
	}

	// Rename the watched root out from under the watcher.
	renamed := filepath.Join(parent, "src.bak")
	if err := os.Rename(src, renamed); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-watchErr:
		var gone *ErrWatchRootGone
		if !errors.As(err, &gone) {
			t.Fatalf("expected *ErrWatchRootGone, got %T: %v", err, err)
		}
		if gone.Path == "" {
			t.Errorf("ErrWatchRootGone.Path is empty")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Watch did not return ErrWatchRootGone within 3s after the watched root was renamed")
	}
}

// TestWatch_RootRemove_SurfacesError verifies that removing (rmdir) the
// watched root is also detected and surfaces the same ErrWatchRootGone
// instead of silently dropping events. Same root-cause as the rename case
// : root removal and rename must terminate
// the watch loop with a clear error.
func TestWatch_RootRemove_SurfacesError(t *testing.T) {
	parent := t.TempDir()
	src := filepath.Join(parent, "src")
	if err := os.Mkdir(src, 0755); err != nil {
		t.Fatal(err)
	}

	got := make(chan []Event, 16)
	w := New([]string{src}, []string{".ts"}, 50*time.Millisecond, func(events []Event) {
		got <- events
	})

	watchErr := make(chan error, 1)
	go func() { watchErr <- w.Watch() }()
	defer w.Stop()
	time.Sleep(200 * time.Millisecond)

	if err := os.Remove(src); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-watchErr:
		var gone *ErrWatchRootGone
		if !errors.As(err, &gone) {
			t.Fatalf("expected *ErrWatchRootGone, got %T: %v", err, err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Watch did not return ErrWatchRootGone within 3s after the watched root was removed")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// E. Atomically populated subdirectory
// ─────────────────────────────────────────────────────────────────────────────

// TestWatch_AtomicSubdirPopulation_FilesAreSynthesized exercises the
// `git checkout`/`git pull` scenario: a fully-populated directory is renamed
// into the watch root in one syscall. the backend reports an event for the
// directory; the watcher attaches via addRecursive but the kernel never
// generated Create events for files that existed inside the dir before the
// watch was attached. The fix walks the freshly-attached tree and synthesizes
// Create events for every file matching the configured extensions.
func TestWatch_AtomicSubdirPopulation_FilesAreSynthesized(t *testing.T) {
	root := t.TempDir()
	// the backend delivers events under the caller-visible root (fswatch
	// resolves the watch root before adding). On macOS, t.TempDir() returns
	// /var/folders/... which resolves to /private/var/folders/... — assertions
	// must use the resolved form to match the synthesizer's emitted paths.
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}

	staging := filepath.Join(t.TempDir(), "package")
	if err := os.MkdirAll(staging, 0755); err != nil {
		t.Fatal(err)
	}
	const expected = 5
	for i := range expected {
		path := filepath.Join(staging, fmt.Sprintf("file%d.ts", i))
		if err := os.WriteFile(path, []byte("export {}"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// A non-watched extension must NOT be synthesized.
	if err := os.WriteFile(filepath.Join(staging, "README.md"), []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}

	got := make(chan []Event, 64)
	w := New([]string{root}, []string{".ts"}, 50*time.Millisecond, func(events []Event) {
		got <- events
	})
	go w.Watch()
	defer w.Stop()
	time.Sleep(200 * time.Millisecond)

	dest := filepath.Join(root, "package")
	if err := os.Rename(staging, dest); err != nil {
		t.Skipf("cross-device rename not allowed in temp setup: %v", err)
	}

	events := drainEvents(got, 1*time.Second)

	seen := make(map[string]bool)
	for _, ev := range events {
		base := filepath.Base(ev.Path)
		if filepath.Ext(base) == ".ts" && filepath.Dir(ev.Path) == dest {
			seen[base] = true
		}
		if base == "README.md" {
			t.Errorf("synthesizer emitted an event for non-watched extension: %v", ev)
		}
	}

	for i := range expected {
		name := fmt.Sprintf("file%d.ts", i)
		if !seen[name] {
			t.Errorf("expected synthetic event for pre-existing file %q after atomic rename, none received", name)
		}
	}
}

// TestWatch_AtomicSubdirPopulation_DeeplyNested verifies the synthesizer
// walks the entire renamed-in tree, not just the top level. This is the
// realistic shape after `git checkout` of a feature branch that adds a
// multi-level package directory.
func TestWatch_AtomicSubdirPopulation_DeeplyNested(t *testing.T) {
	root := t.TempDir()
	// See note in TestWatch_AtomicSubdirPopulation_FilesAreSynthesized.
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}

	staging := filepath.Join(t.TempDir(), "package")
	want := []string{
		filepath.Join(staging, "a.ts"),
		filepath.Join(staging, "lvl1", "b.ts"),
		filepath.Join(staging, "lvl1", "lvl2", "c.ts"),
		filepath.Join(staging, "lvl1", "lvl2", "lvl3", "d.ts"),
		filepath.Join(staging, "lvl1", "lvl2", "lvl3", "lvl4", "e.ts"),
	}
	for _, p := range want {
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("export {}"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// Skipped subdir — must not be walked.
	skipped := filepath.Join(staging, "lvl1", "node_modules", "ignored.ts")
	if err := os.MkdirAll(filepath.Dir(skipped), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skipped, []byte("export {}"), 0644); err != nil {
		t.Fatal(err)
	}

	got := make(chan []Event, 64)
	w := New([]string{root}, []string{".ts"}, 50*time.Millisecond, func(events []Event) {
		got <- events
	})
	go w.Watch()
	defer w.Stop()
	time.Sleep(200 * time.Millisecond)

	dest := filepath.Join(root, "package")
	if err := os.Rename(staging, dest); err != nil {
		t.Skipf("cross-device rename not allowed in temp setup: %v", err)
	}

	events := drainEvents(got, 1*time.Second)

	expectedPaths := make(map[string]bool, len(want))
	for _, p := range want {
		rel, err := filepath.Rel(staging, p)
		if err != nil {
			t.Fatal(err)
		}
		expectedPaths[filepath.Join(dest, rel)] = false
	}
	for _, ev := range events {
		if _, ok := expectedPaths[ev.Path]; ok {
			expectedPaths[ev.Path] = true
		}
		if filepath.Base(ev.Path) == "ignored.ts" {
			t.Errorf("synthesizer descended into a skipped directory: %v", ev)
		}
	}
	for path, seen := range expectedPaths {
		if !seen {
			t.Errorf("expected synthetic event for nested file %q, none received", path)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Goroutine and timer leaks
// ─────────────────────────────────────────────────────────────────────────────

// TestWatch_GoroutineCountStableAcrossRestarts repeatedly creates and stops
// watchers, asserting that the goroutine count returns to baseline. Catches
// regressions where Stop() forgets to release backend goroutines, or where
// pending AfterFunc timers never get reaped.
func TestWatch_GoroutineCountStableAcrossRestarts(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "app.ts"), []byte("x"), 0644)

	cycle := func() {
		w := New([]string{dir}, []string{".ts"}, 20*time.Millisecond, func(events []Event) {})
		go w.Watch()
		// Let the backend spin up.
		time.Sleep(20 * time.Millisecond)
		w.Stop()
		// Brief gap so Stop's stopCh propagates before the next iter.
		time.Sleep(20 * time.Millisecond)
	}

	// Warm-up: the fswatch runtime lazily starts persistent package-level
	// goroutines (backend event loop, debouncer) on first use. Those are a
	// one-time cost, not a per-cycle leak. Take the baseline after them.
	cycle()
	time.Sleep(100 * time.Millisecond)
	baseline := goroutineCount()

	const iterations = 50
	for range iterations {
		cycle()
	}

	// Backend teardown goroutines can lag Stop; wait for the count to settle.
	var final int
	for range 20 {
		final = goroutineCount()
		if final-baseline <= 5 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	delta := final - baseline

	// Allow some slack for Go runtime housekeeping. Anything more than ~5 is
	// almost certainly a leak.
	if delta > 5 {
		t.Errorf("goroutine count grew by %d after %d watcher cycles (baseline=%d, final=%d)",
			delta, iterations, baseline, final)
	} else {
		t.Logf("goroutine delta after %d cycles: %d (baseline=%d, final=%d)",
			iterations, delta, baseline, final)
	}
}

// TestWatch_AfterFuncTimerNotLeakedOnRapidEvents fires many file changes in
// quick succession and asserts that the timer goroutine count stays bounded.
// The watcher's debounce uses a single replaceable timer (addPending stops the
// previous before launching a new), so worst case is 1 outstanding timer —
// not N.
func TestWatch_AfterFuncTimerNotLeakedOnRapidEvents(t *testing.T) {
	dir := t.TempDir()

	// Slow consumer: only counts events. Doesn't matter for the timer test.
	var fired int32
	w := New([]string{dir}, []string{".ts"}, 100*time.Millisecond, func(events []Event) {
		atomic.AddInt32(&fired, int32(len(events)))
	})
	go w.Watch()
	defer w.Stop()
	time.Sleep(100 * time.Millisecond)

	baseline := goroutineCount()

	// Burst 200 file writes. Each addPending call stops the existing timer
	// and creates a new one. Stop() on a fired timer is a no-op, so the
	// already-fired timer's goroutine completes naturally; pending timers
	// get cancelled.
	for i := range 200 {
		os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%d.ts", i)), []byte("x"), 0644)
	}

	// Let everything settle.
	time.Sleep(500 * time.Millisecond)

	peak := goroutineCount()
	delta := peak - baseline
	if delta > 3 {
		t.Errorf("goroutine delta during burst was %d — timer goroutines may be leaking", delta)
	} else {
		t.Logf("goroutine delta during 200-file burst: %d (baseline=%d, peak=%d, fired=%d)",
			delta, baseline, peak, atomic.LoadInt32(&fired))
	}
}

// TestWatch_StopUnblocksOnChangeCallbackInFlight verifies that Stop() can be
// called while a debounce timer is mid-callback, and that the callback's
// onChange is allowed to complete without deadlocking the caller.
func TestWatch_StopUnblocksOnChangeCallbackInFlight(t *testing.T) {
	dir := t.TempDir()

	callbackEntered := make(chan struct{})
	callbackCanReturn := make(chan struct{})

	w := New([]string{dir}, []string{".ts"}, 30*time.Millisecond, func(events []Event) {
		// Block inside the callback to simulate a slow consumer.
		select {
		case callbackEntered <- struct{}{}:
		default:
		}
		<-callbackCanReturn
	})
	go w.Watch()
	defer close(callbackCanReturn)
	time.Sleep(100 * time.Millisecond)

	// Trigger one event.
	os.WriteFile(filepath.Join(dir, "a.ts"), []byte("x"), 0644)

	select {
	case <-callbackEntered:
		// good
	case <-time.After(2 * time.Second):
		t.Fatal("debounce callback never fired")
	}

	// Stop the watcher while the callback is blocked. Stop should not deadlock.
	stopped := make(chan struct{})
	go func() {
		w.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
		// good — Stop didn't wait for the callback
	case <-time.After(1 * time.Second):
		t.Fatal("Stop deadlocked on in-flight callback (it shouldn't synchronize with the user's onChange)")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Polling fallback edge cases
// ─────────────────────────────────────────────────────────────────────────────

// TestWatch_PollingDetectsContentEqualSizeRewrite verifies the polling fallback
// catches "rewrite the file with same length but different content" — which
// requires mtime to bump (size alone is unchanged). This is the common path for
// editing a single character.
func TestWatch_PollingDetectsContentEqualSizeRewrite(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.ts")
	os.WriteFile(f, []byte("export const x=1;"), 0644)

	w := &Watcher{
		dirs:         []string{dir},
		extensions:   []string{".ts"},
		debounce:     20 * time.Millisecond,
		pollInterval: 30 * time.Millisecond,
	}

	old := w.buildSnapshot()

	// Sleep long enough for mtime to differ.
	time.Sleep(50 * time.Millisecond)
	os.WriteFile(f, []byte("export const x=2;"), 0644)
	new := w.buildSnapshot()

	events := w.diff(old, new)
	if len(events) != 1 || events[0].Op != "write" {
		t.Errorf("expected 1 write event, got %v", events)
	}
}

// TestWatch_PollingMissesIdenticalSizeIdenticalMtimeRewrite locks in the
// known limitation that polling cannot detect a same-size + same-mtime rewrite
// (extremely rare, requires deliberately-frozen mtimes — possible with `touch
// -t` or filesystems with low mtime resolution like FAT32). Documenting it.
func TestWatch_PollingMissesIdenticalSizeIdenticalMtimeRewrite(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.ts")
	if err := os.WriteFile(f, []byte("aaa"), 0644); err != nil {
		t.Fatal(err)
	}

	w := &Watcher{
		dirs:         []string{dir},
		extensions:   []string{".ts"},
		debounce:     20 * time.Millisecond,
		pollInterval: 30 * time.Millisecond,
	}

	// Capture mtime so we can re-stamp the rewrite.
	info, err := os.Stat(f)
	if err != nil {
		t.Fatal(err)
	}
	old := w.buildSnapshot()

	// Rewrite with same length, then force mtime back to the original.
	if err := os.WriteFile(f, []byte("bbb"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(f, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}

	new := w.buildSnapshot()
	events := w.diff(old, new)
	if len(events) != 0 {
		t.Logf("FIXED on this platform: polling now catches frozen-mtime rewrites: %v", events)
	} else {
		t.Logf("KNOWN LIMITATION: polling cannot detect same-size same-mtime rewrites")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// shouldSkipPath edge cases
// ─────────────────────────────────────────────────────────────────────────────

// TestShouldSkipPath_SeparatorNormalization verifies that shouldSkipPath
// correctly handles forward-slash paths (Git Bash on Windows and Linux/macOS),
// OS-native paths built with filepath.Join, and edge cases that must not match.
func TestShouldSkipPath_SeparatorNormalization(t *testing.T) {
	cases := []struct {
		path       string
		want       bool
		descriptor string
	}{
		// Forward-slash paths: the primary regression target.
		// On Windows, Git Bash (and some backends) deliver "C:/..." paths.
		// On Linux/macOS, all paths are already forward-slash.
		{`C:/proj/src/node_modules/foo.ts`, true, "forward-slash: mid-path node_modules"},
		{`C:/proj/node_modules`, true, "forward-slash: node_modules at end (no trailing slash)"},
		{`C:/proj/.git/objects`, true, "forward-slash: .git mid-path"},
		{`C:/proj/dist/main.js`, true, "forward-slash: dist mid-path"},
		{`/home/user/proj/node_modules/foo.ts`, true, "unix: mid-path node_modules"},
		{`/home/user/proj/node_modules`, true, "unix: node_modules at end"},
		{`/home/user/proj/.git/HEAD`, true, "unix: .git mid-path"},
		{`/home/user/proj/build/app.js`, true, "unix: build mid-path"},
		{`/home/user/proj/coverage/lcov.info`, true, "unix: coverage mid-path"},
		{`/home/user/proj/.next/cache/foo`, true, "unix: .next mid-path"},
		{`/home/user/proj/.turbo/cache`, true, "unix: .turbo mid-path"},

		// OS-native paths (filepath.Join uses the platform separator).
		// These exercise the ToSlash conversion on Windows and are no-ops on Linux/macOS.
		{filepath.Join("proj", "src", "node_modules", "foo.ts"), true, "native: mid-path node_modules"},
		{filepath.Join("proj", "node_modules"), true, "native: node_modules at end"},
		{filepath.Join("proj", ".git", "objects"), true, "native: .git mid-path"},
		{filepath.Join("proj", "dist", "main.js"), true, "native: dist mid-path"},
		{filepath.Join("proj", "build", "out.js"), true, "native: build mid-path"},
		{filepath.Join("proj", "coverage", "lcov.info"), true, "native: coverage mid-path"},
		{filepath.Join("proj", ".next", "cache", "foo"), true, "native: .next mid-path"},
		{filepath.Join("proj", ".turbo", "cache"), true, "native: .turbo mid-path"},
		{filepath.Join("proj", "__pycache__", "mod.pyc"), true, "native: __pycache__ mid-path"},

		// Relative paths
		{"node_modules/foo.ts", true, "relative forward-slash: node_modules/foo.ts"},
		{"node_modules", true, "relative: bare node_modules directory name"},

		// Negative cases: must NOT match — substring/suffix traps
		{"/path/notnode_modules/foo.ts", false, "notnode_modules is not a skip dir"},
		{"/path/node_modules_old/foo.ts", false, "node_modules_old suffix trap"},
		{"/path/to/src/app.ts", false, "normal source file"},
		{"/path/to/mynode_modules/foo.ts", false, "prefix+node_modules substring trap"},
		{filepath.Join("proj", "src", "node_modulesFake", "foo.ts"), false, "native: lookalike dir must not match"},
	}

	for _, c := range cases {
		got := shouldSkipPath(c.path)
		if got != c.want {
			t.Errorf("shouldSkipPath(%q) = %v, want %v — %s", c.path, got, c.want, c.descriptor)
		}
	}
}

// TestShouldSkipPath_CaseInsensitive verifies that shouldSkipPath matches skip
// directories regardless of case, so Windows paths like NODE_MODULES or Dist
// are treated the same as their lowercase equivalents.
func TestShouldSkipPath_CaseInsensitive(t *testing.T) {
	cases := []struct {
		path        string
		shouldSkip  bool
		description string
	}{
		{filepath.Join("project", "node_modules", "foo"), true, "lowercase always skipped"},
		{filepath.Join("project", "Node_Modules", "foo"), true, "Windows-style mixed-case"},
		{filepath.Join("project", "NODE_MODULES", "foo"), true, "all-uppercase"},
		{filepath.Join("project", ".git", "objects"), true, ".git always skipped"},
		{filepath.Join("project", "src", "node_modulesFake"), false, "should NOT skip lookalikes"},
	}
	for _, c := range cases {
		got := shouldSkipPath(c.path)
		if got != c.shouldSkip {
			t.Errorf("shouldSkipPath(%q) = %v, want %v — %s", c.path, got, c.shouldSkip, c.description)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Stress: many concurrent file events
// ─────────────────────────────────────────────────────────────────────────────

// ─────────────────────────────────────────────────────────────────────────────
// Overflow recovery (Issue #137)
// ─────────────────────────────────────────────────────────────────────────────

// TestIsOverflowError_RecognizesKernelSignals locks in the detection contract
// across all backends so a backend swap or upstream upgrade can't silently
// make the watcher stop recovering from bursts.
func TestIsOverflowError_RecognizesKernelSignals(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"unrelated", fmt.Errorf("permission denied"), false},
		{"fswatch sentinel", fmtErrf("%w", overflowSentinel()), true},
		{"enospc wrapped", fmtErrf("inotify: %w", syscallENOSPC()), true},
		{"windows-style string", fmt.Errorf("ReadDirectoryChangesW: notify_enum_dir overflow"), true},
		{"raw 1022 string", fmt.Errorf("system call returned error 1022"), true},
		{"buffer overflow string", fmt.Errorf("queue or buffer overflow"), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isOverflowError(c.err); got != c.want {
				t.Errorf("isOverflowError(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}

// TestWatch_OverflowRecovery_SynthesizesMissedEvents primes the snapshot,
// mutates files OUT-OF-BAND so the watcher never sees them (no time.Sleep race —
// we touch files after priming and call the recovery path directly), then
// asserts the recovery synthesizes write/create/remove events for every change.
func TestWatch_OverflowRecovery_SynthesizesMissedEvents(t *testing.T) {
	dir := t.TempDir()
	// buildSnapshot resolves the watch root through EvalSymlinks (e.g. /var ->
	// /private/var on macOS), so events arrive keyed on the resolved path.
	// Resolve here too so the assertions match.
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	keep := filepath.Join(dir, "keep.ts")
	mutate := filepath.Join(dir, "mutate.ts")
	gone := filepath.Join(dir, "gone.ts")

	if err := os.WriteFile(keep, []byte("export const k=1;"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mutate, []byte("export const m=1;"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gone, []byte("export const g=1;"), 0644); err != nil {
		t.Fatal(err)
	}

	got := make(chan []Event, 16)
	w := New([]string{dir}, []string{".ts"}, 30*time.Millisecond, func(events []Event) {
		got <- events
	})
	w.primeSnapshotForTest()

	// Mutate out-of-band: rewrite mutate.ts (size + mtime change), create new.ts,
	// remove gone.ts. None of these are observed live because Watch() isn't running.
	time.Sleep(20 * time.Millisecond)
	if err := os.WriteFile(mutate, []byte("export const m=42;"), 0644); err != nil {
		t.Fatal(err)
	}
	created := filepath.Join(dir, "new.ts")
	if err := os.WriteFile(created, []byte("export const n=1;"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}

	// Trigger the same recovery path the overflow error branch would.
	if w.triggerOverflowForTest() {
		t.Fatalf("single overflow recovery shouldn't trigger fallback")
	}
	if w.overflowFailuresForTest() != 0 {
		t.Errorf("expected failures=0 after clean recovery, got %d", w.overflowFailuresForTest())
	}

	events := drainEvents(got, 500*time.Millisecond)

	byPath := map[string]string{}
	for _, ev := range events {
		byPath[ev.Path] = ev.Op
	}
	if op := byPath[mutate]; op != "write" {
		t.Errorf("expected write event for mutated file, got %q (all=%v)", op, events)
	}
	if op := byPath[created]; op != "create" {
		t.Errorf("expected create event for new file, got %q (all=%v)", op, events)
	}
	if op := byPath[gone]; op != "remove" {
		t.Errorf("expected remove event for deleted file, got %q (all=%v)", op, events)
	}
	if _, present := byPath[keep]; present {
		t.Errorf("did not expect any event for unchanged file, got %q", byPath[keep])
	}
}

// TestWatch_OverflowFallback_SwitchesToPolling forces two consecutive failed
// recoveries and asserts (a) the failure counter increments, (b) the watcher
// flips into polling mode, and (c) Watch() is now serviced by the polling
// backend (it picks up a fresh write).
func TestWatch_OverflowFallback_SwitchesToPolling(t *testing.T) {
	dir := t.TempDir()
	// Match buildSnapshot's EvalSymlinks resolution so polling event paths
	// compare equal to our expected target path.
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	if err := os.WriteFile(filepath.Join(dir, "seed.ts"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	got := make(chan []Event, 16)
	w := New([]string{dir}, []string{".ts"}, 30*time.Millisecond, func(events []Event) {
		got <- events
	})
	w.SetPollInterval(50 * time.Millisecond)
	w.primeSnapshotForTest()

	// Force every recovery attempt to look like it failed even though the
	// re-walk itself succeeded.
	recoverHook = func() bool { return true }
	t.Cleanup(func() { recoverHook = nil })

	if w.triggerOverflowForTest() {
		t.Fatalf("first failure should not trigger fallback (need %d)", maxOverflowFailures)
	}
	if got := w.overflowFailuresForTest(); got != 1 {
		t.Fatalf("after 1 failed recovery, expected failures=1, got %d", got)
	}
	if w.fallbackTriggeredForTest() {
		t.Fatalf("fallback flag set too early")
	}

	if !w.triggerOverflowForTest() {
		t.Fatalf("second failure should trigger fallback")
	}
	if !w.fallbackTriggeredForTest() {
		t.Fatalf("fallbackToPolling should be true after %d failures", maxOverflowFailures)
	}

	// Now run the polling backend (the real Watch() would have done this
	// automatically after watchNative returned). Confirm new writes are
	// still delivered through the polling path.
	stopped := make(chan struct{})
	go func() {
		_ = w.watchPolling()
		close(stopped)
	}()
	defer func() {
		w.Stop()
		<-stopped
	}()

	time.Sleep(100 * time.Millisecond)
	target := filepath.Join(dir, "after-fallback.ts")
	if err := os.WriteFile(target, []byte("y"), 0644); err != nil {
		t.Fatal(err)
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case events := <-got:
			for _, ev := range events {
				if ev.Path == target {
					return
				}
			}
		case <-deadline:
			t.Fatal("polling backend did not deliver an event for the post-fallback write")
		}
	}
}

// helpers used only by TestIsOverflowError_RecognizesKernelSignals.
func fmtErrf(format string, a ...any) error { return fmt.Errorf(format, a...) }
func overflowSentinel() error               { return errOverflow }
func syscallENOSPC() error                  { return errENOSPC }

// TestWatch_BurstWritesNoDataRace runs the watcher under concurrent writes to
// `go test -race` for a few hundred file events. Catches data races in the
// shared `pending` slice.
func TestWatch_BurstWritesNoDataRace(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in -short mode")
	}
	dir := t.TempDir()

	w := New([]string{dir}, []string{".ts"}, 30*time.Millisecond, func(events []Event) {})
	go w.Watch()
	defer w.Stop()
	time.Sleep(100 * time.Millisecond)

	var wg sync.WaitGroup
	for w := range 8 {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := range 50 {
				path := filepath.Join(dir, fmt.Sprintf("w%d_f%d.ts", worker, i))
				_ = os.WriteFile(path, []byte("x"), 0644)
			}
		}(w)
	}
	wg.Wait()
	time.Sleep(200 * time.Millisecond)
}

// ─────────────────────────────────────────────────────────────────────────────
// Symlinked watch root (#152)
// ─────────────────────────────────────────────────────────────────────────────

// TestWatch_SymlinkedRoot_DetectsChanges verifies that native-mode watching
// works when the watch root is itself a symlink (or Windows junction). The
// underlying filepath.Walk uses os.Lstat, which would otherwise see the
// symlink as a non-directory and skip it entirely — meaning fsw.Add never
// runs and the dev loop is silent forever. addRecursive must resolve the
// root via filepath.EvalSymlinks before walking.
func TestWatch_SymlinkedRoot_DetectsChanges(t *testing.T) {
	parent := t.TempDir()
	real := filepath.Join(parent, "real")
	if err := os.Mkdir(real, 0755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(parent, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlink creation not permitted: %v", err)
	}

	got := make(chan []Event, 16)
	w := New([]string{link}, []string{".ts"}, 50*time.Millisecond, func(events []Event) {
		got <- events
	})
	go w.Watch()
	defer w.Stop()
	time.Sleep(200 * time.Millisecond)

	// Write a file in the REAL directory and expect an event.
	if err := os.WriteFile(filepath.Join(real, "app.ts"), []byte("export const x = 1;"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case <-got:
		// good — the backend saw the write through the resolved root
	case <-time.After(2 * time.Second):
		t.Fatal("write to symlinked watch root was not detected; addRecursive likely no-op'd")
	}
}

// TestPolling_SymlinkedRoot_Snapshots verifies the same root-symlink resolution
// works for the polling fallback (buildSnapshot). Without EvalSymlinks at the
// root, the .ts file inside the resolved target would be invisible to the
// snapshot diff and polling would never report a change.
func TestPolling_SymlinkedRoot_Snapshots(t *testing.T) {
	parent := t.TempDir()
	real := filepath.Join(parent, "real")
	if err := os.Mkdir(real, 0755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(parent, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlink creation not permitted: %v", err)
	}

	if err := os.WriteFile(filepath.Join(real, "app.ts"), []byte("export const x = 1;"), 0644); err != nil {
		t.Fatal(err)
	}

	w := &Watcher{
		dirs:         []string{link},
		extensions:   []string{".ts"},
		debounce:     20 * time.Millisecond,
		pollInterval: 30 * time.Millisecond,
	}

	snap := w.buildSnapshot()
	if len(snap) != 1 {
		t.Fatalf("expected 1 file in snapshot through symlinked root, got %d (snap=%v)", len(snap), snap)
	}
}
