//go:build !windows

package watchdog

import (
	"io"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// A child in its own process group, the way the runner starts one.
func startGroupLeader(t *testing.T, script string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("sh", []string{"-c", script}...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting child: %v", err)
	}
	go cmd.Wait()
	t.Cleanup(func() { syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) })
	return cmd
}

func waitUntilGone(t *testing.T, pid int, within time.Duration, what string) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if syscall.Kill(pid, 0) != nil {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("%s (pid %d) still alive after %v", what, pid, within)
}

// The pipe closing with the child alive means tsgonest died: the guard has to
// take the child's whole group down, grandchildren included.
func TestGuard_StopsTheGroupWhenThePipeCloses(t *testing.T) {
	child := startGroupLeader(t, "sleep 300 & wait")
	pid := child.Process.Pid
	time.Sleep(100 * time.Millisecond)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	returned := make(chan struct{})
	go func() {
		Guard(pid, r, io.Discard)
		close(returned)
	}()

	// While tsgonest lives, the guard leaves the child alone.
	time.Sleep(300 * time.Millisecond)
	if syscall.Kill(pid, 0) != nil {
		t.Fatal("guard killed the child while the pipe was still open")
	}

	w.Close()
	waitUntilGone(t, pid, 3*time.Second, "child")
	select {
	case <-returned:
	case <-time.After(3 * time.Second):
		t.Fatal("guard did not return after stopping the child")
	}
}

// A child that ignores SIGTERM gets the grace period and then SIGKILL, and the
// guard must not hang on it.
func TestGuard_EscalatesToSIGKILL(t *testing.T) {
	if testing.Short() {
		t.Skip("waits out the 5s grace period")
	}
	child := startGroupLeader(t, "trap '' TERM; sleep 300 & wait")
	pid := child.Process.Pid
	time.Sleep(100 * time.Millisecond)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	go Guard(pid, r, io.Discard)
	w.Close()

	start := time.Now()
	waitUntilGone(t, pid, gracePeriod+3*time.Second, "SIGTERM-ignoring child")
	if time.Since(start) < gracePeriod-time.Second {
		t.Errorf("child died in %v, before the grace period; SIGKILL came too early", time.Since(start))
	}
}

// The child leaving on its own, or through Runner.Stop, must not leave a
// guard behind waiting on a pipe nobody will close.
func TestGuard_ReturnsWhenTheChildExitsFirst(t *testing.T) {
	child := startGroupLeader(t, "sleep 0.2")
	pid := child.Process.Pid

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	returned := make(chan struct{})
	go func() {
		Guard(pid, r, io.Discard)
		close(returned)
	}()

	select {
	case <-returned:
	case <-time.After(3 * time.Second):
		t.Fatal("guard kept waiting after the child exited")
	}
}

// Attach spawns a real guard process in the child's group, and releasing our
// end is what lets it go once the child has.
func TestAttach_GuardJoinsTheGroupAndLeavesWithTheChild(t *testing.T) {
	real := HelperCommand
	HelperCommand = testHelperCommand
	defer func() { HelperCommand = real }()

	child := startGroupLeader(t, "sleep 300 & wait")
	pid := child.Process.Pid
	time.Sleep(100 * time.Millisecond)

	release, err := Attach(pid)
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	time.Sleep(300 * time.Millisecond)

	guards := groupMembersExcept(t, pid, pid)
	if len(guards) == 0 {
		t.Fatal("no guard process found in the child's process group")
	}
	// Stop the child the way the runner would, then release: the guard sees
	// the child gone and exits without killing anything.
	syscall.Kill(-pid, syscall.SIGTERM)
	waitUntilGone(t, pid, 3*time.Second, "child")
	release()
	for _, g := range guards {
		waitUntilGone(t, g, 3*time.Second, "guard")
	}
}

// The real parent dying, rather than releasing politely, is what the guard is
// for: the runner's TestRunner_WatchParentDeathCleansUpChildOnSIGKILL covers
// that with a SIGKILLed parent process and a plain `sleep` child.
