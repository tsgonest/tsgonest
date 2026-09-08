//go:build !windows

package runner

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/tsgonest/tsgonest/internal/watchdog"
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

	script := fmt.Sprintf(`sleep 300 & echo $! > %s; wait`, pidFile.Name())
	r := New("sh", []string{"-c", script}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}

	grandchildPid := waitForPidFile(t, pidFile.Name(), 5*time.Second)

	if err := syscall.Kill(grandchildPid, 0); err != nil {
		r.Stop()
		t.Fatalf("grandchild %d should be alive: %v", grandchildPid, err)
	}

	r.Stop()
	time.Sleep(200 * time.Millisecond)

	if err := syscall.Kill(grandchildPid, 0); err == nil {
		t.Errorf("grandchild process %d still alive after Stop() — process group kill failed", grandchildPid)
		syscall.Kill(grandchildPid, syscall.SIGKILL)
	}
}

// TestRunner_ParentDeathCleansUpChild verifies that when the parent process is
// killed with SIGKILL, the child process is also terminated (via Pdeathsig on Linux).
// macOS has no Pdeathsig, so it is skipped here and covered instead by
// TestRunner_WatchParentDeathCleansUpChildOnSIGKILL, which does the same job
// through a guard process.
func TestRunner_ParentDeathCleansUpChild(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("Pdeathsig not available on macOS — WatchParentDeath covers this case")
	}
	if os.Getenv("RUNNER_TEST_HELPER") == "parent" {
		pidFilePath := os.Getenv("RUNNER_TEST_PIDFILE")
		r := New("sleep", []string{"300"}, "")
		if err := r.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "helper: start error: %v\n", err)
			os.Exit(1)
		}
		os.WriteFile(pidFilePath, []byte(fmt.Sprintf("%d", r.cmd.Process.Pid)), 0644)
		select {}
	}

	pidFile, err := os.CreateTemp("", "runner-orphan-test-*")
	if err != nil {
		t.Fatal(err)
	}
	pidFilePath := pidFile.Name()
	pidFile.Close()
	defer os.Remove(pidFilePath)

	cmd := exec.Command(os.Args[0], "-test.run=^TestRunner_ParentDeathCleansUpChild$")
	cmd.Env = append(os.Environ(), "RUNNER_TEST_HELPER=parent", "RUNNER_TEST_PIDFILE="+pidFilePath)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	childPid := waitForPidFile(t, pidFilePath, 5*time.Second)

	if err := syscall.Kill(childPid, 0); err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		t.Fatalf("child %d should be alive before parent death: %v", childPid, err)
	}

	cmd.Process.Signal(syscall.SIGKILL)
	cmd.Wait()

	time.Sleep(500 * time.Millisecond)

	err = syscall.Kill(childPid, 0)
	if err == nil {
		t.Errorf("child process %d survived after parent was killed with SIGKILL — Pdeathsig not working", childPid)
		syscall.Kill(childPid, syscall.SIGKILL)
	}
}

func TestRunner_StopKillsNestedProcessTree(t *testing.T) {
	pidFile, err := os.CreateTemp("", "runner-nested-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(pidFile.Name())
	pidFile.Close()

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

func TestRunner_ForceKillAfterSIGTERMTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow test (requires 5s SIGTERM timeout)")
	}

	script := `trap '' TERM; while true; do sleep 300 & wait $!; done`
	r := New("sh", []string{"-c", script}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)

	start := time.Now()
	r.Stop()
	elapsed := time.Since(start)

	if elapsed < 4*time.Second {
		t.Errorf("Stop() returned in %v — expected ~5s for SIGTERM timeout before SIGKILL", elapsed)
	}
	if elapsed > 8*time.Second {
		t.Errorf("Stop() took %v — SIGKILL should have killed within the timeout", elapsed)
	}
}

func TestRunner_DoubleStopSafe(t *testing.T) {
	r := New("sleep", []string{"300"}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	if err := r.Stop(); err != nil {
		t.Fatalf("first Stop: %v", err)
	}
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
	r.Stop()
}

// TestRunner_StopPreventsSubsequentStart verifies that after Stop(),
// calling Start() directly returns ErrRunnerStopped.
// Restart() should still work because it clears the stopped flag.
func TestRunner_StopPreventsSubsequentStart(t *testing.T) {
	r := New("sleep", []string{"300"}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	r.Stop()

	// Direct Start() after Stop() should be refused
	err := r.Start()
	if err != ErrRunnerStopped {
		t.Errorf("Start() after Stop() should return ErrRunnerStopped, got: %v", err)
		r.Stop()
	}

	// Restart() should work because it clears the stopped flag
	r2 := New("sleep", []string{"300"}, "")
	r2.Start()
	r2.Stop()
	if err := r2.Restart(); err != nil {
		t.Errorf("Restart() after Stop() should work, got: %v", err)
	}
	r2.Stop()
}

// TestRunner_StopResetsCmd verifies that Stop() resets r.cmd to nil.
func TestRunner_StopResetsCmd(t *testing.T) {
	r := New("sleep", []string{"300"}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	r.Stop()

	r.mu.Lock()
	cmdNil := r.cmd == nil
	r.mu.Unlock()

	if !cmdNil {
		t.Errorf("Stop() should reset r.cmd to nil")
	}
}

// A child that ignores SIGTERM ends up on the SIGKILL path; that path waits on
// the wait goroutine with no timeout of its own, so it must not hang.
func TestRunner_StopSIGKILLPathHasNoTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow test (requires 5s SIGTERM timeout)")
	}

	script := `trap '' TERM; while true; do sleep 300 & wait $!; done`
	r := New("sh", []string{"-c", script}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)

	done := make(chan error, 1)
	go func() {
		done <- r.Stop()
	}()

	select {
	case <-done:
		// SIGKILL path completed successfully
	case <-time.After(10 * time.Second):
		t.Fatalf("Stop() blocked for >10s after SIGKILL — deadlock")
	}
}

// TestRunner_StopReturnsErrorForNonESRCH verifies that Stop() properly
// handles signal errors. ESRCH (process already dead) is tolerated,
// but other errors are surfaced.
func TestRunner_StopHandlesDeadProcess(t *testing.T) {
	r := New("sleep", []string{"300"}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}

	// Kill the process out-of-band
	syscall.Kill(r.cmd.Process.Pid, syscall.SIGKILL)
	r.Wait()

	// Stop() on an already-dead process should not panic.
	// ESRCH is tolerated (process is dead = desired outcome).
	err := r.Stop()
	if err != nil {
		t.Errorf("Stop() on dead process should succeed (ESRCH is tolerated), got: %v", err)
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

// Kill is what the second Ctrl-C runs right before os.Exit: the whole process
// group must be gone when it returns, or the child outlives tsgonest on its
// port and the next `tsgonest dev` fails with EADDRINUSE.
func TestRunner_KillLeavesNoProcessGroupBehind(t *testing.T) {
	pidFile, err := os.CreateTemp("", "runner-kill-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(pidFile.Name())
	pidFile.Close()

	// The child traps SIGTERM, the way a server mid-shutdown ignores it, so
	// only SIGKILL can take it down.
	script := fmt.Sprintf(`trap '' TERM; sleep 300 & echo $! > %s; wait`, pidFile.Name())
	r := New("sh", []string{"-c", script}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	childPid := r.cmd.Process.Pid
	grandchildPid := waitForPidFile(t, pidFile.Name(), 5*time.Second)

	start := time.Now()
	r.Kill()
	if took := time.Since(start); took > 2*time.Second {
		t.Errorf("Kill took %v; it must not wait out a graceful timeout", took)
	}
	time.Sleep(100 * time.Millisecond)

	if err := syscall.Kill(childPid, 0); err == nil {
		t.Errorf("child %d still alive after Kill()", childPid)
		syscall.Kill(childPid, syscall.SIGKILL)
	}
	if err := syscall.Kill(grandchildPid, 0); err == nil {
		t.Errorf("grandchild %d still alive after Kill() — process group kill failed", grandchildPid)
		syscall.Kill(grandchildPid, syscall.SIGKILL)
	}
	if r.Running() {
		t.Error("Running() still true after Kill()")
	}
}

func TestRunner_KillWithoutStart(t *testing.T) {
	r := New("sleep", []string{"300"}, "")
	r.Kill()
	if err := r.Start(); err != ErrRunnerStopped {
		t.Fatalf("Start after Kill: got %v, want ErrRunnerStopped", err)
		r.Stop()
	}
}

// The macOS gap. Pdeathsig does not exist there, so a SIGKILLed parent used to
// leave the child running with no parent and still holding its port, which is
// the EADDRINUSE on the next `tsgonest dev`. With WatchParentDeath a guard
// process beside the child stops it when the parent goes. The child here is a
// plain sleep: it knows nothing and needs to know nothing.
func TestRunner_WatchParentDeathCleansUpChildOnSIGKILL(t *testing.T) {
	if os.Getenv("RUNNER_TEST_HELPER") == "guard" {
		pid, _ := strconv.Atoi(os.Getenv("RUNNER_TEST_CHILD"))
		os.Exit(watchdog.Main([]string{strconv.Itoa(pid)}))
	}
	if os.Getenv("RUNNER_TEST_HELPER") == "watchparent" {
		pidFilePath := os.Getenv("RUNNER_TEST_PIDFILE")
		// The test binary cannot dispatch the hidden subcommand: re-run this
		// test with a marker instead, the branch above.
		watchdog.HelperCommand = func(childPid int) (*exec.Cmd, error) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestRunner_WatchParentDeathCleansUpChildOnSIGKILL$")
			cmd.Env = append(os.Environ(), "RUNNER_TEST_HELPER=guard", "RUNNER_TEST_CHILD="+strconv.Itoa(childPid))
			return cmd, nil
		}
		r := New("sleep", []string{"300"}, "")
		r.WatchParentDeath = true
		if err := r.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "helper: start error: %v\n", err)
			os.Exit(1)
		}
		os.WriteFile(pidFilePath, []byte(fmt.Sprintf("%d", r.cmd.Process.Pid)), 0644)
		select {}
	}

	pidFile, err := os.CreateTemp("", "runner-watchparent-test-*")
	if err != nil {
		t.Fatal(err)
	}
	pidFilePath := pidFile.Name()
	pidFile.Close()
	defer os.Remove(pidFilePath)

	cmd := exec.Command(os.Args[0], "-test.run=^TestRunner_WatchParentDeathCleansUpChildOnSIGKILL$")
	cmd.Env = append(os.Environ(), "RUNNER_TEST_HELPER=watchparent", "RUNNER_TEST_PIDFILE="+pidFilePath)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	childPid := waitForPidFile(t, pidFilePath, 5*time.Second)
	if err := syscall.Kill(childPid, 0); err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		t.Fatalf("child %d should be alive before the parent dies: %v", childPid, err)
	}

	cmd.Process.Signal(syscall.SIGKILL)
	cmd.Wait()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(childPid, 0); err != nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Errorf("child %d survived the parent being SIGKILLed", childPid)
	syscall.Kill(childPid, syscall.SIGKILL)
}
