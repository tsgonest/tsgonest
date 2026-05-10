//go:build !windows

package runner

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// goroutineCount returns the current goroutine count after a brief
// stabilization. Used by leak assertions to ignore Go runtime noise.
func goroutineCount() int {
	runtime.GC()
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	return runtime.NumGoroutine()
}

func TestRunner_StartStop(t *testing.T) {
	r := New("sleep", []string{"10"}, "")
	if err := r.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if !r.Running() {
		t.Error("expected process to be running")
	}
	if err := r.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestRunner_Restart(t *testing.T) {
	r := New("sleep", []string{"10"}, "")
	if err := r.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := r.Restart(); err != nil {
		t.Fatalf("Restart failed: %v", err)
	}
	if !r.Running() {
		t.Error("expected process to be running after restart")
	}
	r.Stop()
}

func TestRunner_StopWithoutStart(t *testing.T) {
	r := New("echo", []string{"hello"}, "")
	// Should not panic
	if err := r.Stop(); err != nil {
		t.Fatalf("Stop without start should not error: %v", err)
	}
}

func TestRunner_Wait(t *testing.T) {
	// Run a short command and wait for it to finish
	r := New("sleep", []string{"0.1"}, "")
	if err := r.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	done := make(chan struct{})
	go func() {
		r.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Process exited naturally
	case <-time.After(3 * time.Second):
		t.Fatal("Wait timed out")
	}
}

func TestRunner_DisableStdin(t *testing.T) {
	// With DisableStdin=true, the child process should NOT receive stdin.
	// We verify this by running "cat" which reads from stdin and checking
	// that it exits immediately (no stdin = EOF = exit).
	r := New("cat", nil, "")
	r.DisableStdin = true
	if err := r.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	done := make(chan struct{})
	go func() {
		r.Wait()
		close(done)
	}()

	select {
	case <-done:
		// cat received EOF and exited — DisableStdin worked
	case <-time.After(3 * time.Second):
		r.Stop()
		t.Fatal("cat should have exited immediately with no stdin (DisableStdin=true)")
	}
}

func TestRunner_DisableStdin_DefaultFalse(t *testing.T) {
	// By default, DisableStdin should be false
	r := New("echo", []string{"hello"}, "")
	if r.DisableStdin {
		t.Error("expected DisableStdin to default to false")
	}
}

// TestRunner_Running_DataRace stress-tests Running() against Restart() to
// guarantee the lock-free atomic-based liveness flag stays race-free under
// `go test -race`. The previous implementation read r.cmd.ProcessState from
// Running() while the Wait goroutine wrote it via cmd.Wait(); this test would
// have flagged that race. Run with: go test -race ./internal/runner/.
func TestRunner_Running_DataRace(t *testing.T) {
	r := New("sleep", []string{"10"}, "")
	if err := r.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer r.Stop()

	var (
		stop    atomic.Bool
		wg      sync.WaitGroup
		reads   atomic.Int64
		restart atomic.Int64
	)

	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				_ = r.Running()
				reads.Add(1)
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		deadline := time.Now().Add(500 * time.Millisecond)
		for time.Now().Before(deadline) {
			if err := r.Restart(); err != nil {
				t.Errorf("Restart failed: %v", err)
				return
			}
			restart.Add(1)
		}
		stop.Store(true)
	}()

	wg.Wait()

	if restart.Load() < 2 {
		t.Fatalf("expected at least 2 Restart() cycles to exercise the race window, got %d", restart.Load())
	}
	if reads.Load() < 1000 {
		t.Fatalf("expected at least 1000 Running() reads to exercise the race window, got %d", reads.Load())
	}
}

// TestRunner_GoroutineCountStableAcrossManyRestarts verifies that the cmd.Wait
// goroutine spawned in Start() is reliably reaped by Stop(), so a long-running
// dev session that hits Restart() thousands of times doesn't slowly accumulate
// dead Wait goroutines.
func TestRunner_GoroutineCountStableAcrossManyRestarts(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping goroutine-leak stress test in -short mode")
	}

	baseline := goroutineCount()

	const cycles = 50
	r := New("sleep", []string{"30"}, "")
	if err := r.Start(); err != nil {
		t.Fatalf("initial Start failed: %v", err)
	}
	for range cycles {
		if err := r.Restart(); err != nil {
			t.Fatalf("Restart failed: %v", err)
		}
	}
	r.Stop()

	final := goroutineCount()
	delta := final - baseline
	if delta > 5 {
		t.Errorf("goroutine count grew by %d after %d Restart cycles (baseline=%d, final=%d) — likely Wait goroutine leak",
			delta, cycles, baseline, final)
	} else {
		t.Logf("goroutine delta after %d Restart cycles: %d (baseline=%d, final=%d)",
			cycles, delta, baseline, final)
	}
}

// TestRunner_StopAfterExitDoesNotLeakWaitGoroutine verifies that when the
// child process exits naturally before Stop() is called, the Wait goroutine
// still terminates and Stop() doesn't deadlock.
func TestRunner_StopAfterExitDoesNotLeakWaitGoroutine(t *testing.T) {
	baseline := goroutineCount()

	for range 20 {
		r := New("true", nil, "")
		if err := r.Start(); err != nil {
			t.Fatalf("Start failed: %v", err)
		}
		// Let the process exit naturally.
		time.Sleep(10 * time.Millisecond)
		// Stop after exit — must be a no-op, not a hang.
		stopped := make(chan struct{})
		go func() {
			r.Stop()
			close(stopped)
		}()
		select {
		case <-stopped:
		case <-time.After(2 * time.Second):
			t.Fatal("Stop after natural exit deadlocked")
		}
	}

	delta := goroutineCount() - baseline
	if delta > 3 {
		t.Errorf("goroutine delta after 20 short-lived processes: %d (likely Wait goroutine leak)", delta)
	}
}

func TestRunner_RunningAfterExit(t *testing.T) {
	// Run a command that exits quickly
	r := New("true", nil, "")
	if err := r.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	r.Wait()

	// Give a moment for ProcessState to be set
	time.Sleep(50 * time.Millisecond)

	if r.Running() {
		t.Error("expected process to not be running after exit")
	}
}
