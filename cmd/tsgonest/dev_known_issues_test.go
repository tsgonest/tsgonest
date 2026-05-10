package main

// Documentation tests for known dev/watch-mode bugs surfaced by the May 2026
// audit. Each test references a finding (A/B/C/F) from the audit, asserts the
// observable broken state where possible, and points at the location of the
// fix that needs to land.
//
// Most of these are skipped because they exercise the full runDevLoop with
// real child processes — too heavy for unit tests. They serve as a clear
// catalogue: when each fix lands, flip the corresponding Skip + assertion.

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tsgonest/tsgonest/internal/watcher"
)

// ─────────────────────────────────────────────────────────────────────────────
// A. Defer-order can spawn an orphan child after Stop()  — FIXED (#129)
// ─────────────────────────────────────────────────────────────────────────────

// fakeBuilder lets us simulate a slow rebuild without invoking tsgo. The
// build pauses on buildStarted/releaseBuild so the test can interleave a
// shutdown with an in-flight build.
type fakeBuilder struct {
	buildStarted chan struct{}
	releaseBuild chan struct{}
	calls        atomic.Int32
}

func (f *fakeBuilder) Build() int {
	f.calls.Add(1)
	if f.buildStarted != nil {
		select {
		case f.buildStarted <- struct{}{}:
		default:
		}
	}
	if f.releaseBuild != nil {
		<-f.releaseBuild
	}
	return 0
}

func (f *fakeBuilder) BuildSingleFile(_ string) int { return f.Build() }

// fakeRestarter records every Restart() call so the test can assert that
// no restart happens after shutdown was signalled.
type fakeRestarter struct {
	mu       sync.Mutex
	restarts int
}

func (f *fakeRestarter) Restart() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.restarts++
	return nil
}

func (f *fakeRestarter) Count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.restarts
}

// TestDevLoop_RebuildGoroutineDoesNotRestartAfterShutdown reproduces the
// orphan-child race from issue #129 and asserts the fix:
//
//  1. Trigger a rebuild.
//  2. Wait until the build is mid-flight.
//  3. Close `done` (simulating runDevLoop's deferred close).
//  4. Release the build so it completes.
//  5. The goroutine must NOT call proc.Restart() — its post-build done check
//     must short-circuit, otherwise a fresh child would be spawned after
//     proc.Stop has run, leaking an orphan.
//  6. The goroutine must exit (verified via wg.Wait inside a timeout).
func TestDevLoop_RebuildGoroutineDoesNotRestartAfterShutdown(t *testing.T) {
	builder := &fakeBuilder{
		buildStarted: make(chan struct{}, 1),
		releaseBuild: make(chan struct{}),
	}
	restarter := &fakeRestarter{}
	changeCh := make(chan []watcher.Event, 1)
	done := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		rebuildLoop(rebuildLoopDeps{
			changeCh: changeCh,
			done:     done,
			builder:  builder,
			proc:     restarter,
			pending:  func() bool { return false },
		})
	}()

	changeCh <- []watcher.Event{{Path: "src/a.ts", Op: "write"}}

	select {
	case <-builder.buildStarted:
	case <-time.After(2 * time.Second):
		close(done)
		close(builder.releaseBuild)
		wg.Wait()
		t.Fatal("build never started")
	}

	close(done)
	close(builder.releaseBuild)

	exited := make(chan struct{})
	go func() {
		wg.Wait()
		close(exited)
	}()
	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		t.Fatal("rebuild goroutine did not exit after done was closed")
	}

	if got := restarter.Count(); got != 0 {
		t.Fatalf("proc.Restart() called %d time(s) after shutdown — orphan child would have been spawned", got)
	}
}

// TestDevLoop_RebuildGoroutineNoRestartUnderRaceStress hammers the same
// shutdown race many times. If the done check were removed (or the WaitGroup
// join missing), some iterations would race between build completion and
// shutdown and call Restart().
func TestDevLoop_RebuildGoroutineNoRestartUnderRaceStress(t *testing.T) {
	const iterations = 50

	for i := 0; i < iterations; i++ {
		builder := &fakeBuilder{
			buildStarted: make(chan struct{}, 1),
			releaseBuild: make(chan struct{}),
		}
		restarter := &fakeRestarter{}
		changeCh := make(chan []watcher.Event, 1)
		done := make(chan struct{})

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			rebuildLoop(rebuildLoopDeps{
				changeCh: changeCh,
				done:     done,
				builder:  builder,
				proc:     restarter,
				pending:  func() bool { return false },
			})
		}()

		changeCh <- []watcher.Event{{Path: "src/a.ts", Op: "write"}}

		select {
		case <-builder.buildStarted:
		case <-time.After(2 * time.Second):
			close(done)
			close(builder.releaseBuild)
			wg.Wait()
			t.Fatalf("iter %d: build never started", i)
		}

		// Close done while the build is mid-flight, then release the build
		// so it finishes and reaches the post-build branch. The goroutine
		// must observe done and skip proc.Restart(), even though releaseBuild
		// is closed on the very next instruction. Without a defensive done
		// check before Restart, the goroutine's prior post-drainLoop state
		// would race ahead and spawn a fresh child.
		close(done)
		close(builder.releaseBuild)

		exited := make(chan struct{})
		go func() {
			wg.Wait()
			close(exited)
		}()
		select {
		case <-exited:
		case <-time.After(2 * time.Second):
			t.Fatalf("iter %d: rebuild goroutine did not exit", i)
		}

		if got := restarter.Count(); got != 0 {
			t.Fatalf("iter %d: proc.Restart() called %d time(s) after shutdown — orphan-child race regressed", i, got)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// B. Stdin scanner goroutine multiplies on config restart  — FIXED (#130)
// ─────────────────────────────────────────────────────────────────────────────

// Issue #130 fix: a single long-lived stdin scanner lives in runDev and
// forwards "rs" presses to the active runDevLoop iteration via a shared
// rsCh channel. Each iteration's manual-restart listener (runRsListener)
// terminates cleanly when its `done` channel closes. As a result, repeated
// config-change restarts must NOT accumulate goroutines.
//
// We can't directly count "scanner goroutines blocked on os.Stdin" in a unit
// test (that's the whole point — they're uninterruptible), so we exercise
// the contract that replaced the leaky scanner: runRsListener is the only
// per-iteration goroutine spawned by the manual-restart path, and it must
// exit when done closes. After N simulated restarts, total goroutine count
// must be flat.
func TestDevLoop_StdinScannerSurvivesAcrossConfigRestarts(t *testing.T) {
	const restarts = 8

	// Snapshot baseline goroutine count after the runtime stabilises.
	settle()
	before := runtime.NumGoroutine()

	rsCh := make(chan struct{}, 1) // shared — same shape as runDev's rsCh

	for i := 0; i < restarts; i++ {
		done := make(chan struct{})
		started := make(chan struct{})
		go func() {
			close(started)
			runRsListener(done, rsCh, true, func() {})
		}()
		<-started
		// Simulate a runDevLoop iteration ending (config change → return).
		close(done)
		// Give the listener a moment to observe the close and exit.
		settle()
	}

	after := runtime.NumGoroutine()
	if after > before+1 {
		t.Errorf("goroutine leak across %d simulated config-change restarts: before=%d after=%d (delta=%d)\nA single long-lived stdin scanner must survive restarts without spawning new goroutines per iteration. See issue #130.",
			restarts, before, after, after-before)
	}
}

// Regression test: across N simulated config-change restarts, an "rs" press
// pushed onto the shared rsCh must reach the currently-active iteration's
// listener (not a stale leaked scanner from a previous iteration).
func TestDevLoop_StdinScannerForwardsRsAcrossRestarts(t *testing.T) {
	const restarts = 5

	rsCh := make(chan struct{}, 1) // shared across iterations, like runDev's rsCh

	for i := 0; i < restarts; i++ {
		done := make(chan struct{})
		var triggered atomic.Int32
		fired := make(chan struct{}, 1)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			runRsListener(done, rsCh, true, func() {
				triggered.Add(1)
				select {
				case fired <- struct{}{}:
				default:
				}
			})
		}()

		// Push "rs" — like the long-lived scanner does on a real keystroke.
		select {
		case rsCh <- struct{}{}:
		case <-time.After(time.Second):
			t.Fatalf("iteration %d: rsCh send blocked — listener not draining", i)
		}

		// The active iteration's listener must consume it.
		select {
		case <-fired:
		case <-time.After(2 * time.Second):
			t.Fatalf("iteration %d: 'rs' press never reached the active listener (would happen if a stale scanner from a prior iteration intercepted it)", i)
		}
		if got := triggered.Load(); got != 1 {
			t.Fatalf("iteration %d: onRestart fired %d times, want 1", i, got)
		}

		close(done)
		wg.Wait()
	}
}

// settle yields enough that goroutines closing in response to a `done` close
// have a chance to actually exit before NumGoroutine is sampled.
func settle() {
	for i := 0; i < 20; i++ {
		runtime.Gosched()
		time.Sleep(5 * time.Millisecond)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// C. "rs" + file-change → double proc.Restart()
// ─────────────────────────────────────────────────────────────────────────────

// The rebuild goroutine has a drain check: after each build, it checks
// changeCh and w.Pending(); if more changes are in flight, it skips
// proc.Restart() and waits for the next batch. The manual-restart "rs"
// goroutine has no equivalent check. If the user types "rs" during a slow
// rebuild, both flows complete and both call proc.Restart() back-to-back.
// Result: a brief port-already-in-use window or a double restart flicker.
//
// Reproduction: trigger a slow file-change rebuild, then type "rs" before
// it completes. Observe the child restarted twice.
//
// Fix: route "rs" through the same changeCh (or a sibling channel) that the
// rebuild goroutine drains, so all restart triggers go through one serializer.
func TestDevLoop_ManualRestartCanDoubleRestart_KnownIssue(t *testing.T) {
	t.Skip("KNOWN ISSUE C: 'rs' restart path has no drain check; can double-restart with a concurrent file-change rebuild. Fix in dev.go runDevLoop manual-restart branch.")
}

// ─────────────────────────────────────────────────────────────────────────────
// F. --exec flag shell selection by GOOS
// ─────────────────────────────────────────────────────────────────────────────

func TestDevLoop_ExecFlagShellSelection(t *testing.T) {
	cmd := "echo hello"

	prog, args := pickExecShellFor("linux", cmd)
	if prog != "sh" {
		t.Errorf("linux: expected prog %q, got %q", "sh", prog)
	}
	if len(args) != 2 || args[0] != "-c" || args[1] != cmd {
		t.Errorf("linux: expected args %v, got %v", []string{"-c", cmd}, args)
	}

	prog, args = pickExecShellFor("darwin", cmd)
	if prog != "sh" {
		t.Errorf("darwin: expected prog %q, got %q", "sh", prog)
	}
	if len(args) != 2 || args[0] != "-c" || args[1] != cmd {
		t.Errorf("darwin: expected args %v, got %v", []string{"-c", cmd}, args)
	}

	prog, args = pickExecShellFor("windows", cmd)
	if prog != "cmd" {
		t.Errorf("windows: expected prog %q, got %q", "cmd", prog)
	}
	if len(args) != 2 || args[0] != "/C" || args[1] != cmd {
		t.Errorf("windows: expected args %v, got %v", []string{"/C", cmd}, args)
	}
}
