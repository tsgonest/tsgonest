//go:build windows

package runner

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// processAlive checks whether a process with the given PID exists on Windows.
func processAlive(pid int) bool {
	out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH", "/FO", "CSV").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), fmt.Sprintf("\"%d\"", pid))
}

// forceKill terminates a process by PID on Windows.
func forceKill(pid int) {
	exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid)).Run()
}

// waitForPidFileWindows polls a file until it contains a valid PID or the deadline expires.
func waitForPidFileWindows(t *testing.T, path string, timeout time.Duration) int {
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
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for PID in %s", path)
	return 0
}

// skipUnlessBugTestsWin skips the test unless RUNNER_BUG_TESTS=1 is set.
// Bug-demonstration tests intentionally fail to prove issues exist.
func skipUnlessBugTestsWin(t *testing.T) {
	t.Helper()
	if os.Getenv("RUNNER_BUG_TESTS") != "1" {
		t.Skip("skipping bug demonstration test (set RUNNER_BUG_TESTS=1 to run)")
	}
}

// --- Basic functionality (Windows equivalents of runner_test.go) ---

func TestRunner_StartStop(t *testing.T) {
	// ping is universally available on Windows
	r := New("ping", []string{"-n", "300", "127.0.0.1"}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	if !r.Running() {
		t.Error("expected process to be running")
	}
	if err := r.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestRunner_Restart(t *testing.T) {
	r := New("ping", []string{"-n", "300", "127.0.0.1"}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	if err := r.Restart(); err != nil {
		t.Fatal(err)
	}
	if !r.Running() {
		t.Error("expected running after restart")
	}
	r.Stop()
}

func TestRunner_StopWithoutStart(t *testing.T) {
	r := New("cmd", []string{"/c", "echo hello"}, "")
	if err := r.Stop(); err != nil {
		t.Fatalf("Stop without start should not error: %v", err)
	}
}

func TestRunner_Wait(t *testing.T) {
	// cmd /c echo exits immediately
	r := New("cmd", []string{"/c", "echo hello"}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		r.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Wait timed out")
	}
}

func TestRunner_DisableStdin(t *testing.T) {
	// "more" on Windows reads from stdin; with stdin disabled it gets EOF and exits.
	r := New("more", nil, "")
	r.DisableStdin = true
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		r.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		r.Stop()
		t.Fatal("more should have exited immediately with no stdin")
	}
}

func TestRunner_DisableStdin_DefaultFalse(t *testing.T) {
	r := New("cmd", []string{"/c", "echo hello"}, "")
	if r.DisableStdin {
		t.Error("expected DisableStdin to default to false")
	}
}

func TestRunner_RunningAfterExit(t *testing.T) {
	r := New("cmd", []string{"/c", "exit 0"}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	r.Wait()
	time.Sleep(50 * time.Millisecond)
	if r.Running() {
		t.Error("expected process to not be running after exit")
	}
}

// --- Process leak tests ---

// TestRunner_WindowsStopDoesNotKillProcessTree demonstrates that on Windows,
// Stop() only terminates the direct child via TerminateProcess.
// Grandchild processes (e.g., Node.js spawning workers) are NOT killed.
func TestRunner_WindowsStopDoesNotKillProcessTree(t *testing.T) {
	skipUnlessBugTestsWin(t)
	if os.Getenv("RUNNER_TEST_HELPER") == "win-grandchild" {
		select {} // block forever until killed
	}
	if os.Getenv("RUNNER_TEST_HELPER") == "win-spawn-parent" {
		pidFile := os.Getenv("RUNNER_TEST_PIDFILE")
		child := exec.Command(os.Args[0], "-test.run=^TestRunner_WindowsStopDoesNotKillProcessTree$")
		child.Env = append(os.Environ(), "RUNNER_TEST_HELPER=win-grandchild")
		if err := child.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "spawn grandchild: %v\n", err)
			os.Exit(1)
		}
		os.WriteFile(pidFile, []byte(strconv.Itoa(child.Process.Pid)), 0644)
		child.Wait()
		os.Exit(0)
	}

	pidFile, err := os.CreateTemp("", "runner-win-tree-*")
	if err != nil {
		t.Fatal(err)
	}
	pidFilePath := pidFile.Name()
	pidFile.Close()
	defer os.Remove(pidFilePath)

	// Use t.Setenv so Runner's child inherits these env vars
	t.Setenv("RUNNER_TEST_HELPER", "win-spawn-parent")
	t.Setenv("RUNNER_TEST_PIDFILE", pidFilePath)

	r := New(os.Args[0], []string{"-test.run=^TestRunner_WindowsStopDoesNotKillProcessTree$"}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}

	grandchildPid := waitForPidFileWindows(t, pidFilePath, 15*time.Second)

	if !processAlive(grandchildPid) {
		r.Stop()
		t.Fatalf("grandchild %d should be alive", grandchildPid)
	}

	// Stop Runner — on Windows this only kills the direct child via TerminateProcess
	r.Stop()
	time.Sleep(1 * time.Second)

	if processAlive(grandchildPid) {
		t.Errorf("PROCESS LEAK: grandchild %d still alive after Stop(). "+
			"Windows Stop() only kills the direct child via TerminateProcess — "+
			"grandchild processes are not affected. "+
			"Fix: use Job Objects with JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE.", grandchildPid)
		forceKill(grandchildPid)
	}
}

// TestRunner_WindowsParentDeathOrphansChild demonstrates that child processes
// survive when the parent is killed on Windows, same as the Unix variant.
func TestRunner_WindowsParentDeathOrphansChild(t *testing.T) {
	skipUnlessBugTestsWin(t)
	if os.Getenv("RUNNER_TEST_HELPER") == "win-parent" {
		pidFile := os.Getenv("RUNNER_TEST_PIDFILE")
		// Use ping as a long-lived child (universally available on Windows)
		r := New("ping", []string{"-n", "300", "127.0.0.1"}, "")
		if err := r.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "helper: start error: %v\n", err)
			os.Exit(1)
		}
		os.WriteFile(pidFile, []byte(strconv.Itoa(r.cmd.Process.Pid)), 0644)
		select {} // block forever
	}

	pidFile, err := os.CreateTemp("", "runner-win-orphan-*")
	if err != nil {
		t.Fatal(err)
	}
	pidFilePath := pidFile.Name()
	pidFile.Close()
	defer os.Remove(pidFilePath)

	cmd := exec.Command(os.Args[0], "-test.run=^TestRunner_WindowsParentDeathOrphansChild$")
	cmd.Env = append(os.Environ(), "RUNNER_TEST_HELPER=win-parent", "RUNNER_TEST_PIDFILE="+pidFilePath)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	childPid := waitForPidFileWindows(t, pidFilePath, 15*time.Second)

	if !processAlive(childPid) {
		cmd.Process.Kill()
		cmd.Wait()
		t.Fatalf("child %d should be alive before parent death", childPid)
	}

	// Kill the parent (simulates VSCode task restart)
	cmd.Process.Kill()
	cmd.Wait()
	time.Sleep(1 * time.Second)

	if processAlive(childPid) {
		t.Errorf("PROCESS LEAK: child %d survived parent death on Windows. "+
			"Without Job Objects, there is no mechanism to auto-terminate children "+
			"when the parent dies. "+
			"Fix: use Job Objects with JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE.", childPid)
		forceKill(childPid)
	}
}

// TestRunner_WindowsStopIsImmediate verifies that the 5-second timeout in
// Windows Stop() is dead code. TerminateProcess is synchronous — the process
// is guaranteed dead when Kill() returns. Stop() should complete near-instantly.
func TestRunner_WindowsStopIsImmediate(t *testing.T) {
	r := New("ping", []string{"-n", "300", "127.0.0.1"}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	r.Stop()
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Errorf("Stop() took %v — expected near-instant since TerminateProcess is synchronous. "+
			"The 5-second timeout branch in runner_windows.go is dead code.", elapsed)
	}
}

// TestRunner_WindowsNoGracefulShutdown verifies that Stop() on Windows uses
// hard kill (TerminateProcess) with no graceful shutdown opportunity.
// The process has zero chance to run cleanup code.
func TestRunner_WindowsNoGracefulShutdown(t *testing.T) {
	skipUnlessBugTestsWin(t)
	if os.Getenv("RUNNER_TEST_HELPER") == "win-graceful" {
		markerPath := os.Getenv("RUNNER_TEST_MARKER")
		os.WriteFile(markerPath, []byte("running"), 0644)
		// This defer will NOT run if TerminateProcess is used
		defer os.WriteFile(markerPath, []byte("cleaned-up"), 0644)
		select {}
	}

	markerFile, err := os.CreateTemp("", "runner-win-graceful-*")
	if err != nil {
		t.Fatal(err)
	}
	markerPath := markerFile.Name()
	markerFile.Close()
	defer os.Remove(markerPath)

	t.Setenv("RUNNER_TEST_HELPER", "win-graceful")
	t.Setenv("RUNNER_TEST_MARKER", markerPath)

	r := New(os.Args[0], []string{"-test.run=^TestRunner_WindowsNoGracefulShutdown$"}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}

	// Wait for "running" marker
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		data, _ := os.ReadFile(markerPath)
		if string(data) == "running" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	r.Stop()
	time.Sleep(500 * time.Millisecond)

	data, _ := os.ReadFile(markerPath)
	if string(data) == "cleaned-up" {
		// Would be surprising — means defers ran despite TerminateProcess
		t.Log("unexpected: process ran cleanup — graceful shutdown somehow worked")
	} else if string(data) == "running" {
		t.Errorf("Windows Stop() used hard kill (TerminateProcess) — the child process "+
			"had no opportunity to run cleanup code (close DB connections, flush logs, etc.). "+
			"Fix: send CTRL_BREAK_EVENT for graceful shutdown before falling back to TerminateProcess.")
	}
}

// --- Issue #8: Restart() after Stop() creates unguarded process ---

// TestRunner_RestartAfterStopCreatesOrphanableProcess demonstrates that calling
// Restart() after Stop() happily launches a new process. Same issue as Unix.
func TestRunner_RestartAfterStopCreatesOrphanableProcess(t *testing.T) {
	skipUnlessBugTestsWin(t)
	r := New("ping", []string{"-n", "300", "127.0.0.1"}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	r.Stop()

	if err := r.Restart(); err != nil {
		return
	}
	if r.Running() {
		t.Errorf("Restart() after Stop() launched a new process. "+
			"Fix: add a 'stopped' flag that prevents Start() after explicit Stop().")
		r.Stop()
	}
}

// --- Issue #11: Stop() doesn't reset state ---

// TestRunner_StopDoesNotResetCmd demonstrates that Stop() leaves stale state.
func TestRunner_StopDoesNotResetCmd(t *testing.T) {
	skipUnlessBugTestsWin(t)
	r := New("ping", []string{"-n", "300", "127.0.0.1"}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	r.Stop()

	r.mu.Lock()
	cmdNil := r.cmd == nil
	r.mu.Unlock()

	if !cmdNil {
		t.Errorf("Stop() does not reset r.cmd to nil — stale state remains. "+
			"Fix: set r.cmd = nil at the end of Stop().")
	}
}

// TestRunner_DoubleStopSafe verifies that calling Stop() twice doesn't panic or deadlock.
func TestRunner_DoubleStopSafe(t *testing.T) {
	r := New("ping", []string{"-n", "300", "127.0.0.1"}, "")
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
	case <-time.After(5 * time.Second):
		t.Fatal("second Stop() deadlocked")
	}
}

// TestRunner_ConcurrentStopRestart exercises concurrent Stop+Restart for deadlocks.
func TestRunner_ConcurrentStopRestart(t *testing.T) {
	r := New("ping", []string{"-n", "300", "127.0.0.1"}, "")
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
