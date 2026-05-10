package runner

import (
	"runtime"
	"testing"
	"time"
)

// TestRunner_StartFailureDoesNotLeakDone is the regression test for issue #144.
//
// Before the fix, runner_windows.go::Start() allocated r.done before several
// error-prone steps (cmd.Start, OpenProcess, AssignProcessToJobObject,
// resumeProcessThreads). On any of those failures the wait goroutine that
// closes r.done was never spawned, so a subsequent r.Wait() would block
// forever.
//
// The fix moves r.done allocation (and the wait goroutine spawn) to the very
// last step of Start(), after all error-prone setup has succeeded. The same
// shape was applied to runner_unix.go for symmetry.
//
// This test reproduces the failure mode by pointing Start() at a binary that
// does not exist (cmd.Start fails) and asserts that Wait() returns promptly.
func TestRunner_StartFailureDoesNotLeakDone(t *testing.T) {
	r := New("/this/path/should/not/exist/tsgonest-runner-test-bogus", nil, "")

	if err := r.Start(); err == nil {
		r.Stop()
		t.Fatal("Start() should have failed for nonexistent binary")
	}

	done := make(chan struct{})
	go func() {
		r.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("Wait() hung after Start() failure — r.done was leaked")
	}
}

// TestRunner_StartFailureNoGoroutineLeak stress-tests issue #144 by repeatedly
// failing Start() and asserting that no wait goroutines accumulate. Before the
// fix, every failed Start() would also strand a closed-but-unreachable r.done
// channel and (on Windows) a half-initialized cmd; after the fix, neither
// resource is allocated until Start() succeeds end-to-end.
func TestRunner_StartFailureNoGoroutineLeak(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping goroutine-leak stress test in -short mode")
	}

	runtime.GC()
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	const iterations = 100
	for i := 0; i < iterations; i++ {
		r := New("/this/path/should/not/exist/tsgonest-runner-test-bogus", nil, "")
		if err := r.Start(); err == nil {
			r.Stop()
			t.Fatalf("iteration %d: Start() should have failed", i)
		}

		done := make(chan struct{})
		go func() {
			r.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(1 * time.Second):
			t.Fatalf("iteration %d: Wait() hung after Start() failure", i)
		}
	}

	runtime.GC()
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	final := runtime.NumGoroutine()
	delta := final - baseline
	if delta > 5 {
		t.Errorf("goroutine count grew by %d after %d failed Start() iterations (baseline=%d, final=%d)",
			delta, iterations, baseline, final)
	}
}
