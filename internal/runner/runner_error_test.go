package runner

import (
	"runtime"
	"testing"
	"time"
)

// nonexistentBinary is a command name that should not resolve on any platform.
// PATH lookup will fail, so r.cmd.Start() returns an error before the process
// is ever spawned. This is the only Start() error path that is reproducible
// without injecting syscalls, but the post-condition we assert (r.cmd == nil
// after error) must hold uniformly for every Start() error path — including
// the Windows-only OpenProcess / AssignProcessToJobObject / resume failures
// that motivated #155.
const nonexistentBinary = "tsgonest-test-nonexistent-binary-do-not-create"

// TestRunnerStart_ErrorPathClearsCmd verifies that when Start() fails, the
// runner does not retain a stale *exec.Cmd. A non-nil r.cmd after a failed
// Start() would make Running() report true and Stop() try to signal a process
// that was never resumed (or never spawned).
func TestRunnerStart_ErrorPathClearsCmd(t *testing.T) {
	r := New(nonexistentBinary, nil, "")
	err := r.Start()
	if err == nil {
		t.Fatal("expected Start() with nonexistent binary to fail")
	}

	r.mu.Lock()
	cmd := r.cmd
	r.mu.Unlock()

	if cmd != nil {
		t.Errorf("after failed Start(), r.cmd = %p, want nil", cmd)
	}
	if r.Running() {
		t.Error("after failed Start(), Running() should be false")
	}
}

// TestRunnerStart_RepeatedFailureNoHandleLeak hammers Start() with a binary
// that doesn't exist and asserts that (a) r.cmd is always cleared and (b) the
// goroutine count doesn't drift upward across iterations. On Windows, the
// fixed cleanup also reaps the suspended child via cmd.Wait(); a regression
// would surface here as goroutine growth (the background Wait goroutine spun
// up by Start() never terminates because nothing closes its handle).
func TestRunnerStart_RepeatedFailureNoHandleLeak(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping leak stress test in -short mode")
	}

	runtime.GC()
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	const iterations = 50
	for i := 0; i < iterations; i++ {
		r := New(nonexistentBinary, nil, "")
		if err := r.Start(); err == nil {
			t.Fatalf("iteration %d: expected Start() to fail", i)
		}

		r.mu.Lock()
		cmd := r.cmd
		r.mu.Unlock()
		if cmd != nil {
			t.Fatalf("iteration %d: r.cmd not cleared after failed Start()", i)
		}
	}

	runtime.GC()
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	final := runtime.NumGoroutine()
	if delta := final - baseline; delta > 5 {
		t.Errorf("goroutine count grew by %d after %d failed Start() calls (baseline=%d, final=%d) — likely Wait goroutine / handle leak",
			delta, iterations, baseline, final)
	}
}
