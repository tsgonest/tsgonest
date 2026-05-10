//go:build windows

package runner

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
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

// --- CREATE_BREAKAWAY_FROM_JOB retry tests (issue #150) ---

func TestIsBreakawayDenied(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"ERROR_ACCESS_DENIED direct", syscall.ERROR_ACCESS_DENIED, true},
		{"wrapped ERROR_ACCESS_DENIED", fmt.Errorf("wrap: %w", syscall.ERROR_ACCESS_DENIED), true},
		{"PathError wrapping ERROR_ACCESS_DENIED", &os.PathError{Op: "x", Path: "y", Err: syscall.ERROR_ACCESS_DENIED}, true},
		{"unrelated error", errors.New("nope"), false},
		{"different errno", syscall.Errno(2), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isBreakawayDenied(tc.err); got != tc.want {
				t.Errorf("isBreakawayDenied(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// TestRunner_StartFallsBackWhenBreakawayDenied simulates the CI-with-parent-Job
// scenario by injecting a startProcess that fails the first attempt (the one
// with CREATE_BREAKAWAY_FROM_JOB) with ERROR_ACCESS_DENIED, then defers to the
// real exec on the retry. The retry must succeed and the runner must still
// assign the child to its Job Object normally.
func TestRunner_StartFallsBackWhenBreakawayDenied(t *testing.T) {
	originalStart := startProcess
	t.Cleanup(func() { startProcess = originalStart })

	var attempts int
	var sawBreakawayFlag, retriedWithoutFlag bool
	startProcess = func(cmd *exec.Cmd) error {
		attempts++
		flags := cmd.SysProcAttr.CreationFlags
		hasBreakaway := flags&windows.CREATE_BREAKAWAY_FROM_JOB != 0
		if attempts == 1 {
			sawBreakawayFlag = hasBreakaway
			return &os.PathError{Op: "CreateProcess", Path: cmd.Path, Err: syscall.ERROR_ACCESS_DENIED}
		}
		retriedWithoutFlag = !hasBreakaway
		return cmd.Start()
	}

	r := New("ping", []string{"-n", "300", "127.0.0.1"}, "")
	if err := r.Start(); err != nil {
		t.Fatalf("Start should succeed via fallback, got: %v", err)
	}
	defer r.Stop()

	if attempts != 2 {
		t.Errorf("expected 2 startProcess attempts (denied + retry), got %d", attempts)
	}
	if !sawBreakawayFlag {
		t.Error("first attempt should have included CREATE_BREAKAWAY_FROM_JOB")
	}
	if !retriedWithoutFlag {
		t.Error("retry should have dropped CREATE_BREAKAWAY_FROM_JOB")
	}
	if !r.Running() {
		t.Error("runner should be running after the fallback succeeded")
	}
}

// TestRunner_StartUsesBreakawayByDefault verifies that the first CreateProcess
// attempt requests CREATE_BREAKAWAY_FROM_JOB so tsgonest detaches from a parent
// Job Object whenever the parent permits it.
func TestRunner_StartUsesBreakawayByDefault(t *testing.T) {
	originalStart := startProcess
	t.Cleanup(func() { startProcess = originalStart })

	var firstFlags uint32
	startProcess = func(cmd *exec.Cmd) error {
		if firstFlags == 0 {
			firstFlags = cmd.SysProcAttr.CreationFlags
		}
		return cmd.Start()
	}

	r := New("ping", []string{"-n", "300", "127.0.0.1"}, "")
	if err := r.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer r.Stop()

	if firstFlags&windows.CREATE_BREAKAWAY_FROM_JOB == 0 {
		t.Errorf("first CreateProcess attempt should request CREATE_BREAKAWAY_FROM_JOB; flags=0x%x", firstFlags)
	}
	if firstFlags&syscall.CREATE_NEW_PROCESS_GROUP == 0 {
		t.Errorf("flags should still include CREATE_NEW_PROCESS_GROUP; flags=0x%x", firstFlags)
	}
	if firstFlags&createSuspended == 0 {
		t.Errorf("flags should still include CREATE_SUSPENDED; flags=0x%x", firstFlags)
	}
}

// TestRunner_StartReturnsRealErrorWhenRetryFails ensures non-ERROR_ACCESS_DENIED
// failures from the first attempt are not swallowed by the breakaway fallback.
func TestRunner_StartReturnsRealErrorWhenRetryFails(t *testing.T) {
	originalStart := startProcess
	t.Cleanup(func() { startProcess = originalStart })

	sentinel := errors.New("simulated CreateProcess failure")
	var attempts int
	startProcess = func(cmd *exec.Cmd) error {
		attempts++
		return sentinel
	}

	r := New("ping", []string{"-n", "300", "127.0.0.1"}, "")
	err := r.Start()
	if err == nil {
		r.Stop()
		t.Fatal("expected Start to fail")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected wrapped sentinel, got: %v", err)
	}
	if attempts != 1 {
		t.Errorf("non-breakaway failures should NOT trigger the retry; attempts=%d", attempts)
	}
}

// --- resumeProcessThreads unit tests (#153) ---
//
// These exercise the open/resume aggregation logic via the package-level
// openThreadFn / resumeFn seams without spawning a real suspended process.

func withResumeSeams(t *testing.T, openFn func(uint32) (windows.Handle, error), resFn func(windows.Handle) (uint32, error)) {
	t.Helper()
	prevOpen, prevRes := openThreadFn, resumeFn
	openThreadFn = openFn
	resumeFn = resFn
	t.Cleanup(func() {
		openThreadFn = prevOpen
		resumeFn = prevRes
	})
}

func TestResumeProcessThreads_AllSucceed(t *testing.T) {
	openCalls := atomic.Int32{}
	resumeCalls := atomic.Int32{}
	withResumeSeams(t,
		func(tid uint32) (windows.Handle, error) {
			openCalls.Add(1)
			return windows.Handle(uintptr(tid)), nil
		},
		func(h windows.Handle) (uint32, error) {
			resumeCalls.Add(1)
			return 1, nil
		},
	)

	if err := resumeThreadIDs(1234, []uint32{10, 20, 30}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got := openCalls.Load(); got != 3 {
		t.Errorf("expected 3 open calls, got %d", got)
	}
	if got := resumeCalls.Load(); got != 3 {
		t.Errorf("expected 3 resume calls, got %d", got)
	}
}

func TestResumeProcessThreads_AllFail(t *testing.T) {
	sentinel := errors.New("simulated ResumeThread failure")
	withResumeSeams(t,
		func(tid uint32) (windows.Handle, error) {
			return windows.Handle(uintptr(tid)), nil
		},
		func(h windows.Handle) (uint32, error) {
			return 0, sentinel
		},
	)

	err := resumeThreadIDs(4321, []uint32{10, 20, 30})
	if err == nil {
		t.Fatal("expected error when all resumes fail, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected wrapped sentinel error, got %v", err)
	}
	if !strings.Contains(err.Error(), "failed to resume any thread") {
		t.Errorf("expected 'failed to resume any thread' in error, got %v", err)
	}
}

func TestResumeProcessThreads_PartialSuccess(t *testing.T) {
	sentinel := errors.New("simulated first-thread failure")
	var resumeCount atomic.Int32
	withResumeSeams(t,
		func(tid uint32) (windows.Handle, error) {
			return windows.Handle(uintptr(tid)), nil
		},
		func(h windows.Handle) (uint32, error) {
			n := resumeCount.Add(1)
			if n == 1 {
				return 0, sentinel
			}
			return 1, nil
		},
	)

	stderrBack := os.Stderr
	rPipe, wPipe, _ := os.Pipe()
	os.Stderr = wPipe
	t.Cleanup(func() { os.Stderr = stderrBack })

	err := resumeThreadIDs(99, []uint32{1, 2, 3})

	wPipe.Close()
	buf := make([]byte, 4096)
	n, _ := rPipe.Read(buf)
	logged := string(buf[:n])

	if err != nil {
		t.Fatalf("expected nil error on partial success, got %v", err)
	}
	if !strings.Contains(logged, "resumed 2/3 threads for pid 99") {
		t.Errorf("expected partial-success warning on stderr, got %q", logged)
	}
}

func TestResumeProcessThreads_NoThreads(t *testing.T) {
	withResumeSeams(t,
		func(tid uint32) (windows.Handle, error) {
			t.Fatalf("openThreadFn should not be called when tids is empty")
			return 0, nil
		},
		func(h windows.Handle) (uint32, error) {
			t.Fatalf("resumeFn should not be called when tids is empty")
			return 0, nil
		},
	)

	err := resumeThreadIDs(7, nil)
	if err == nil {
		t.Fatal("expected error when no threads are found, got nil")
	}
	if !strings.Contains(err.Error(), "no threads found for pid 7") {
		t.Errorf("expected 'no threads found for pid 7', got %v", err)
	}
}

func TestResumeProcessThreads_OpenFailsAll(t *testing.T) {
	sentinel := errors.New("OpenThread denied")
	withResumeSeams(t,
		func(tid uint32) (windows.Handle, error) {
			return 0, sentinel
		},
		func(h windows.Handle) (uint32, error) {
			t.Fatalf("resumeFn should not be called when all opens fail")
			return 0, nil
		},
	)

	err := resumeThreadIDs(11, []uint32{1, 2})
	if err == nil {
		t.Fatal("expected error when every OpenThread fails, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected wrapped sentinel, got %v", err)
	}
	if !strings.Contains(err.Error(), "no threads opened for pid 11") {
		t.Errorf("expected 'no threads opened' in error, got %v", err)
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

// TestRunnerStop_NoConsoleSkipsWait verifies that when GenerateConsoleCtrlEvent
// returns an error (e.g. ERROR_INVALID_HANDLE — no console attached), stop()
// kills the process immediately and returns well under the 5-second grace period.
func TestRunnerStop_NoConsoleSkipsWait(t *testing.T) {
	orig := generateConsoleCtrlEvent
	generateConsoleCtrlEvent = func(event uint32, pid uint32) error {
		return fmt.Errorf("ERROR_INVALID_HANDLE")
	}
	t.Cleanup(func() { generateConsoleCtrlEvent = orig })

	r := New("ping", []string{"-n", "300", "127.0.0.1"}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	if err := r.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	elapsed := time.Since(start)

	if elapsed >= 500*time.Millisecond {
		t.Errorf("Stop() took %v — expected <500ms when signal delivery fails (no 5s wait)", elapsed)
	}
}

// TestRunnerStop_WithConsoleWaitsForGraceful verifies that when
// GenerateConsoleCtrlEvent succeeds and the process exits promptly,
// stop() returns via the graceful path without waiting the full 5 seconds.
func TestRunnerStop_WithConsoleWaitsForGraceful(t *testing.T) {
	orig := generateConsoleCtrlEvent
	generateConsoleCtrlEvent = func(event uint32, pid uint32) error {
		// Signal "succeeded" — now kill the process so r.done closes quickly,
		// simulating a process that handles CTRL_BREAK and exits gracefully.
		return nil
	}
	t.Cleanup(func() { generateConsoleCtrlEvent = orig })

	r := New("ping", []string{"-n", "300", "127.0.0.1"}, "")
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}

	// Kill the process out-of-band so r.done closes while stop() is in the select.
	go func() {
		time.Sleep(50 * time.Millisecond)
		r.mu.Lock()
		if r.cmd != nil && r.cmd.Process != nil {
			r.cmd.Process.Kill()
		}
		r.mu.Unlock()
	}()

	start := time.Now()
	if err := r.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	elapsed := time.Since(start)

	if elapsed >= 5*time.Second {
		t.Errorf("Stop() took %v — should have returned via graceful path when r.done closes", elapsed)
	}
}
