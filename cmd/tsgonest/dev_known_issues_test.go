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
	"strings"
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
// B. Stdin scanner goroutine multiplies on config restart
// ─────────────────────────────────────────────────────────────────────────────

// When manualRestart is true, runDevLoop spawns a goroutine that calls
// bufio.NewScanner(os.Stdin) and loops on scanner.Scan(). scanner.Scan()
// blocks in a kernel syscall; closing `done` cannot interrupt it. On a
// config-change restart, runDevLoop returns and a new iteration spawns
// another scanner. The previous scanner is still blocked on Stdin. Each
// config change adds one more scanner. They all race for stdin reads.
//
// Reproduction: configure manualRestart: true, save tsconfig.json N times
// (N config restarts). Type "rs". One of the N+1 scanners wins; the others
// silently consume keystrokes that the user expected the active loop to see.
//
// Fix: hoist the stdin scanner above runDevLoop so a single goroutine
// survives across config restarts, and have it send "rs" events on a channel
// that the active runDevLoop iteration selects on.
func TestDevLoop_StdinScannerLeaksAcrossConfigRestarts_KnownIssue(t *testing.T) {
	t.Skip("KNOWN ISSUE B: bufio.Scanner on os.Stdin is uninterruptible; each config-change restart leaks a scanner goroutine. Fix in dev.go runDev (lift scanner above the loop).")
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
