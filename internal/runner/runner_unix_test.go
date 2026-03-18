//go:build !windows

package runner

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestRunner_StopKillsProcessGroup verifies that Stop() kills the entire
// process group, including grandchild processes spawned by the direct child.
func TestRunner_StopKillsProcessGroup(t *testing.T) {
	pidFile, err := os.CreateTemp("", "runner-pgid-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(pidFile.Name())
	pidFile.Close()

	// sh spawns a background sleep; echo its PID to a file; wait keeps sh alive
	script := fmt.Sprintf(`sleep 300 & echo $! > %s; wait`, pidFile.Name())
	r := New("sh", []string{"-c", script}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}

	grandchildPid := waitForPidFile(t, pidFile.Name(), 5*time.Second)

	// Verify grandchild is alive
	if err := syscall.Kill(grandchildPid, 0); err != nil {
		r.Stop()
		t.Fatalf("grandchild %d should be alive: %v", grandchildPid, err)
	}

	// Stop should kill the entire process group (sh + sleep)
	r.Stop()
	time.Sleep(200 * time.Millisecond)

	if err := syscall.Kill(grandchildPid, 0); err == nil {
		t.Errorf("grandchild process %d still alive after Stop() — process group kill failed", grandchildPid)
		syscall.Kill(grandchildPid, syscall.SIGKILL)
	}
}

// TestRunner_ParentDeathOrphansChild demonstrates the process leak bug:
// when the parent process is killed with SIGKILL (simulating VSCode task restart),
// child processes started with Setpgid survive as orphans because there is no
// Pdeathsig or other parent-death notification mechanism.
func TestRunner_ParentDeathOrphansChild(t *testing.T) {
	if os.Getenv("RUNNER_TEST_HELPER") == "parent" {
		// Helper subprocess: start a child via Runner, write its PID, then block.
		pidFilePath := os.Getenv("RUNNER_TEST_PIDFILE")
		r := New("sleep", []string{"300"}, "")
		if err := r.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "helper: start error: %v\n", err)
			os.Exit(1)
		}
		os.WriteFile(pidFilePath, []byte(fmt.Sprintf("%d", r.cmd.Process.Pid)), 0644)
		// Block forever — the test will SIGKILL us
		select {}
	}

	pidFile, err := os.CreateTemp("", "runner-orphan-test-*")
	if err != nil {
		t.Fatal(err)
	}
	pidFilePath := pidFile.Name()
	pidFile.Close()
	defer os.Remove(pidFilePath)

	// Start a helper process that acts as "tsgonest dev"
	cmd := exec.Command(os.Args[0], "-test.run=^TestRunner_ParentDeathOrphansChild$")
	cmd.Env = append(os.Environ(), "RUNNER_TEST_HELPER=parent", "RUNNER_TEST_PIDFILE="+pidFilePath)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	childPid := waitForPidFile(t, pidFilePath, 5*time.Second)

	// Verify child is alive before we kill the parent
	if err := syscall.Kill(childPid, 0); err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		t.Fatalf("child %d should be alive before parent death: %v", childPid, err)
	}

	// SIGKILL the parent — simulates ungraceful termination (e.g., VSCode killing the task)
	cmd.Process.Signal(syscall.SIGKILL)
	cmd.Wait()

	// Give the OS time to reparent orphans
	time.Sleep(500 * time.Millisecond)

	// The child should be dead if parent-death signaling works.
	// Without Pdeathsig (or equivalent), the child survives as an orphan — this is the bug.
	err = syscall.Kill(childPid, 0)
	if err == nil {
		t.Errorf("PROCESS LEAK: child process %d survived after parent was killed with SIGKILL. "+
			"This confirms the orphan bug: when tsgonest dev is killed ungracefully "+
			"(e.g., VSCode task restart), the node child process is not cleaned up.", childPid)
		syscall.Kill(childPid, syscall.SIGKILL)
	}
}

// TestRunner_StopKillsNestedProcessTree verifies that Stop() kills a deeper
// process tree (parent -> child -> grandchild), not just direct children.
func TestRunner_StopKillsNestedProcessTree(t *testing.T) {
	pidFile, err := os.CreateTemp("", "runner-nested-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(pidFile.Name())
	pidFile.Close()

	// Create a 3-level process tree:
	// sh -c "sh -c 'sleep 300 & echo PID > file; wait' & wait"
	inner := fmt.Sprintf(`sleep 300 & echo $! > %s; wait`, pidFile.Name())
	script := fmt.Sprintf(`sh -c '%s' & wait`, inner)
	r := New("sh", []string{"-c", script}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}

	deepPid := waitForPidFile(t, pidFile.Name(), 5*time.Second)

	if err := syscall.Kill(deepPid, 0); err != nil {
		r.Stop()
		t.Fatalf("deep child %d should be alive: %v", deepPid, err)
	}

	r.Stop()
	time.Sleep(200 * time.Millisecond)

	if err := syscall.Kill(deepPid, 0); err == nil {
		t.Errorf("deep child %d still alive after Stop() — nested process tree not fully killed", deepPid)
		syscall.Kill(deepPid, syscall.SIGKILL)
	}
}

// TestRunner_ForceKillAfterSIGTERMTimeout verifies that Stop() escalates to
// SIGKILL when the process ignores SIGTERM and the 5-second timeout expires.
func TestRunner_ForceKillAfterSIGTERMTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow test (requires 5s SIGTERM timeout)")
	}

	// The shell traps SIGTERM (ignores it). When SIGTERM kills the inner sleep,
	// the loop restarts it, keeping the shell alive until SIGKILL.
	script := `trap '' TERM; while true; do sleep 300 & wait $!; done`
	r := New("sh", []string{"-c", script}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}

	// Give the shell time to set up the trap and start sleep
	time.Sleep(200 * time.Millisecond)

	start := time.Now()
	r.Stop()
	elapsed := time.Since(start)

	// Should take ~5 seconds (SIGTERM grace) before SIGKILL
	if elapsed < 4*time.Second {
		t.Errorf("Stop() returned in %v — expected ~5s for SIGTERM timeout before SIGKILL", elapsed)
	}
	if elapsed > 8*time.Second {
		t.Errorf("Stop() took %v — SIGKILL should have killed within the timeout", elapsed)
	}
}

// TestRunner_DoubleStopSafe verifies that calling Stop() twice on a
// started-then-stopped runner doesn't panic, error, or deadlock.
func TestRunner_DoubleStopSafe(t *testing.T) {
	r := New("sleep", []string{"300"}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	if err := r.Stop(); err != nil {
		t.Fatalf("first Stop: %v", err)
	}
	// Second stop must complete without deadlock
	done := make(chan error, 1)
	go func() {
		done <- r.Stop()
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("second Stop: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("second Stop() deadlocked")
	}
}

// TestRunner_ConcurrentStopRestart exercises the race between a signal handler
// calling Stop() and a rebuild callback calling Restart() at the same time.
// Neither should panic or deadlock.
func TestRunner_ConcurrentStopRestart(t *testing.T) {
	r := New("sleep", []string{"300"}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}

	errs := make(chan error, 2)
	go func() { errs <- r.Stop() }()
	go func() { errs <- r.Restart() }()

	for i := 0; i < 2; i++ {
		select {
		case err := <-errs:
			if err != nil {
				t.Logf("concurrent op error (may be expected): %v", err)
			}
		case <-time.After(15 * time.Second):
			t.Fatal("deadlock: concurrent Stop/Restart didn't complete within 15s")
		}
	}
	// Clean up whatever's left
	r.Stop()
}

// --- Issue #8: Restart() after Stop() creates unguarded process ---

// TestRunner_RestartAfterStopCreatesOrphanableProcess demonstrates that calling
// Restart() after Stop() happily launches a new process. In tsgonest dev, if a
// rebuild calls Restart() after the signal handler's Stop(), the new process
// can escape cleanup.
func TestRunner_RestartAfterStopCreatesOrphanableProcess(t *testing.T) {
	r := New("sleep", []string{"300"}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	// Simulate signal handler calling Stop()
	r.Stop()

	// Simulate rebuild callback calling Restart() after shutdown
	if err := r.Restart(); err != nil {
		return // Restart refused — would be correct behavior
	}
	if r.Running() {
		t.Errorf("Restart() after Stop() launched a new process. "+
			"In tsgonest dev, a rebuild can race with the signal handler: "+
			"signal handler calls Stop(), then rebuild calls Restart(), "+
			"starting a new child that the shutdown path won't clean up. "+
			"Fix: add a 'stopped' flag that prevents Start() after explicit Stop().")
		r.Stop() // clean up
	}
}

// --- Issue #11: Stop() doesn't reset state ---

// TestRunner_StopDoesNotResetCmd demonstrates that Stop() leaves stale state:
// r.cmd and r.cmd.Process still point to the dead process after Stop().
// A second Stop() sends signals to a dead PID instead of being a clean no-op.
func TestRunner_StopDoesNotResetCmd(t *testing.T) {
	r := New("sleep", []string{"300"}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	r.Stop()

	r.mu.Lock()
	cmdNil := r.cmd == nil
	r.mu.Unlock()

	if !cmdNil {
		t.Errorf("Stop() does not reset r.cmd to nil — stale state remains. "+
			"A second Stop() will attempt Getpgid + Kill on a dead PID "+
			"instead of being a clean no-op. "+
			"Fix: set r.cmd = nil at the end of Stop().")
	}
}

// --- Issue #7: Single-shot signal handler ---

// TestRunner_SingleShotSignalHandlerPattern demonstrates the pattern used in
// dev.go: reading one signal from a channel, then exiting. A second signal
// sent during Stop()'s 5-second SIGTERM grace period is silently dropped
// instead of force-killing immediately.
func TestRunner_SingleShotSignalHandlerPattern(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow test (requires SIGTERM timeout)")
	}

	// Reproduce the dev.go signal handling pattern
	sigCh := make(chan os.Signal, 1)

	secondSignalHandled := make(chan bool, 1)

	// Single-shot handler (mirrors dev.go:265-272)
	go func() {
		<-sigCh
		// "shutting down..." — now Stop() runs for up to 5s
		secondSignalHandled <- false
	}()

	// Send first signal
	sigCh <- syscall.SIGINT

	// Try to send second signal (user pressing Ctrl+C again)
	// With buffer=1, one more signal fits. A third would be dropped.
	sigCh <- syscall.SIGINT

	// The second signal sits in the buffer — nobody reads it.
	// In dev.go, this means the user's second Ctrl+C is silently eaten
	// and they're stuck waiting for the 5s SIGTERM timeout.
	select {
	case handled := <-secondSignalHandled:
		if !handled {
			t.Errorf("single-shot signal handler: second signal was NOT handled. "+
				"In tsgonest dev, pressing Ctrl+C twice during the 5s SIGTERM grace period "+
				"silently drops the second signal instead of force-killing immediately. "+
				"Fix: loop in the signal handler goroutine; on second signal, send SIGKILL.")
		}
	case <-time.After(1 * time.Second):
		t.Errorf("single-shot signal handler: goroutine exited after first signal. "+
			"Second signal sits unread in the buffered channel.")
	}
}

// --- Issue #9: Stop() SIGKILL path blocks forever holding mutex ---

// TestRunner_StopSIGKILLPathHasNoTimeout verifies the SIGKILL escalation path
// works, while documenting that it blocks unconditionally on <-r.done with no
// timeout while holding the mutex. In practice SIGKILL always works on Unix,
// but if cmd.Wait() ever hangs, the entire Runner would deadlock.
func TestRunner_StopSIGKILLPathHasNoTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow test (requires 5s SIGTERM timeout)")
	}

	// Use a SIGTERM-resistant process so Stop() reaches the SIGKILL path
	script := `trap '' TERM; while true; do sleep 300 & wait $!; done`
	r := New("sh", []string{"-c", script}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)

	// Stop() should complete: SIGTERM -> 5s timeout -> SIGKILL -> done
	// The final <-r.done after SIGKILL has NO timeout (runner_unix.go:64).
	// We verify it works but flag the structural risk.
	done := make(chan error, 1)
	go func() {
		done <- r.Stop()
	}()

	select {
	case <-done:
		// SIGKILL worked. But the code path has no timeout — if cmd.Wait()
		// ever hangs (zombie process, kernel bug), Stop() blocks forever
		// while holding the mutex, deadlocking the entire Runner.
		t.Logf("SIGKILL path completed, but runner_unix.go:64 has no timeout on "+
			"the final <-r.done — a hung Wait() would deadlock the Runner. "+
			"Fix: add a timeout after SIGKILL.")
	case <-time.After(10 * time.Second):
		t.Fatalf("Stop() blocked for >10s after SIGKILL — deadlock confirmed")
	}
}

// --- Issue #10: SIGTERM/SIGKILL errors silently swallowed ---

// TestRunner_StopSwallowsSignalErrors demonstrates that Stop() discards the
// return values of syscall.Kill. If the process is already dead or we lack
// permissions, the caller has no way to know.
func TestRunner_StopSwallowsSignalErrors(t *testing.T) {
	r := New("sleep", []string{"300"}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}

	// Kill the process out-of-band so Stop()'s SIGTERM hits a dead PID
	syscall.Kill(r.cmd.Process.Pid, syscall.SIGKILL)
	r.Wait()

	// Stop() should return an error indicating the process was already dead,
	// but it swallows all signal errors and always returns nil
	err := r.Stop()
	if err == nil {
		t.Errorf("Stop() returned nil after sending SIGTERM to a dead process. "+
			"syscall.Kill errors (ESRCH, EPERM) are silently discarded in runner_unix.go:48,60. "+
			"Fix: check and return signal errors, or at minimum log them.")
	}
}

// waitForPidFile polls a file until it contains a valid PID or the deadline expires.
func waitForPidFile(t *testing.T, path string, timeout time.Duration) int {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		data, _ := os.ReadFile(path)
		if s := strings.TrimSpace(string(data)); s != "" {
			pid, _ := strconv.Atoi(s)
			if pid > 0 {
				return pid
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for PID in %s", path)
	return 0
}
