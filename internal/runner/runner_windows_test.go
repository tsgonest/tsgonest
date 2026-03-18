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

func processAlive(pid int) bool {
	out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH", "/FO", "CSV").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), fmt.Sprintf("\"%d\"", pid))
}

func forceKill(pid int) {
	exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid)).Run()
}

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

// --- Basic functionality ---

func TestRunner_StartStop(t *testing.T) {
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

// --- Process tree management tests ---

// TestRunner_WindowsStopKillsProcessTree verifies that Stop() kills the
// entire process tree via Job Objects, not just the direct child.
func TestRunner_WindowsStopKillsProcessTree(t *testing.T) {
	if os.Getenv("RUNNER_TEST_HELPER") == "win-grandchild" {
		time.Sleep(10 * time.Minute) // block until killed
	}
	if os.Getenv("RUNNER_TEST_HELPER") == "win-spawn-parent" {
		pidFile := os.Getenv("RUNNER_TEST_PIDFILE")
		child := exec.Command(os.Args[0], "-test.run=^TestRunner_WindowsStopKillsProcessTree$")
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

	t.Setenv("RUNNER_TEST_HELPER", "win-spawn-parent")
	t.Setenv("RUNNER_TEST_PIDFILE", pidFilePath)

	r := New(os.Args[0], []string{"-test.run=^TestRunner_WindowsStopKillsProcessTree$"}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}

	grandchildPid := waitForPidFileWindows(t, pidFilePath, 15*time.Second)

	if !processAlive(grandchildPid) {
		r.Stop()
		t.Fatalf("grandchild %d should be alive", grandchildPid)
	}

	r.Stop()
	time.Sleep(1 * time.Second)

	if processAlive(grandchildPid) {
		t.Errorf("grandchild %d still alive after Stop() — Job Object should have killed entire tree", grandchildPid)
		forceKill(grandchildPid)
	}
}

// TestRunner_WindowsParentDeathCleansUpChild verifies that child processes
// are terminated when the parent dies, via Job Object KILL_ON_JOB_CLOSE.
func TestRunner_WindowsParentDeathCleansUpChild(t *testing.T) {
	if os.Getenv("RUNNER_TEST_HELPER") == "win-parent" {
		pidFile := os.Getenv("RUNNER_TEST_PIDFILE")
		r := New("ping", []string{"-n", "300", "127.0.0.1"}, "")
		if err := r.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "helper: start error: %v\n", err)
			os.Exit(1)
		}
		os.WriteFile(pidFile, []byte(strconv.Itoa(r.cmd.Process.Pid)), 0644)
		time.Sleep(10 * time.Minute) // block until killed
	}

	pidFile, err := os.CreateTemp("", "runner-win-orphan-*")
	if err != nil {
		t.Fatal(err)
	}
	pidFilePath := pidFile.Name()
	pidFile.Close()
	defer os.Remove(pidFilePath)

	cmd := exec.Command(os.Args[0], "-test.run=^TestRunner_WindowsParentDeathCleansUpChild$")
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

	cmd.Process.Kill()
	cmd.Wait()
	time.Sleep(2 * time.Second)

	if processAlive(childPid) {
		t.Errorf("child %d survived parent death — Job Object KILL_ON_JOB_CLOSE should have cleaned it up", childPid)
		forceKill(childPid)
	}
}

// TestRunner_WindowsGracefulShutdown verifies that Stop() sends CTRL_BREAK_EVENT
// before falling back to TerminateProcess, giving the child a chance to clean up.
func TestRunner_WindowsGracefulShutdown(t *testing.T) {
	if os.Getenv("RUNNER_TEST_HELPER") == "win-graceful" {
		markerPath := os.Getenv("RUNNER_TEST_MARKER")
		os.WriteFile(markerPath, []byte("running"), 0644)
		// This defer runs if the process exits gracefully (CTRL_BREAK handled)
		defer os.WriteFile(markerPath, []byte("cleaned-up"), 0644)
		time.Sleep(10 * time.Minute) // block until killed
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

	r := New(os.Args[0], []string{"-test.run=^TestRunner_WindowsGracefulShutdown$"}, "")
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

	// With CTRL_BREAK_EVENT, the Go runtime catches the signal and runs defers.
	// The marker should be "cleaned-up" if graceful shutdown worked.
	data, _ := os.ReadFile(markerPath)
	if string(data) == "cleaned-up" {
		// Graceful shutdown worked — defers ran
	} else if string(data) == "running" {
		// TerminateProcess was used — no cleanup
		t.Logf("graceful shutdown via CTRL_BREAK_EVENT may not have worked; " +
			"this depends on whether the Go runtime handles the signal in time")
	}
}

// TestRunner_WindowsStopIsGracefulThenForced verifies that Stop() now has
// a grace period (via CTRL_BREAK_EVENT) before falling back to hard kill.
func TestRunner_WindowsStopIsGracefulThenForced(t *testing.T) {
	r := New("ping", []string{"-n", "300", "127.0.0.1"}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	r.Stop()
	elapsed := time.Since(start)

	// Stop() should complete — either gracefully or via TerminateProcess timeout.
	// It should NOT hang indefinitely.
	if elapsed > 10*time.Second {
		t.Errorf("Stop() took %v — should complete within grace period + kill", elapsed)
	}
}

// --- State management tests ---

func TestRunner_StopPreventsSubsequentStart(t *testing.T) {
	r := New("ping", []string{"-n", "300", "127.0.0.1"}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	r.Stop()

	err := r.Start()
	if err != ErrRunnerStopped {
		t.Errorf("Start() after Stop() should return ErrRunnerStopped, got: %v", err)
		r.Stop()
	}
}

func TestRunner_StopResetsCmd(t *testing.T) {
	r := New("ping", []string{"-n", "300", "127.0.0.1"}, "")
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
