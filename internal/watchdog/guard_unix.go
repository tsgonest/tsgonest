//go:build !windows

package watchdog

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

// How long the guard gives the app to leave on SIGTERM before SIGKILL, the
// same grace Runner.Stop gives it.
const gracePeriod = 5 * time.Second

// How often the guard checks whether the child is still there, so it goes
// away on its own once the child does.
const livenessInterval = 500 * time.Millisecond

// HelperCommand builds the guard process for a child. The default re-runs
// this binary with the hidden subcommand; tests swap it for their own test
// binary, which cannot dispatch subcommands.
var HelperCommand = func(childPid int) (*exec.Cmd, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("finding own executable: %w", err)
	}
	return exec.Command(self, Command, strconv.Itoa(childPid)), nil
}

// Attach starts a guard for a running child and returns a release function.
// The child must have been started with Setpgid, so its pid is its process
// group: the guard joins that group, which is what makes a Runner.Stop or
// Runner.Kill of the group take the guard down with the child.
//
// Release closes tsgonest's end of the pipe. Called once the child has exited
// it lets the guard see EOF, find the child gone, and leave; not calling it
// (tsgonest died) is the whole point.
func Attach(childPid int) (release func(), err error) {
	guardEnd, ourEnd, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("creating parent-death pipe: %w", err)
	}
	defer guardEnd.Close()

	cmd, err := HelperCommand(childPid)
	if err != nil {
		ourEnd.Close()
		return nil, err
	}
	// fd 3 in the guard. os.Pipe sets close-on-exec, and ExtraFiles clears it
	// only for this end, so no later child of ours inherits the write end and
	// keeps the pipe open on our behalf.
	cmd.ExtraFiles = []*os.File{guardEnd}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: childPid}
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		ourEnd.Close()
		return nil, fmt.Errorf("starting parent-death guard: %w", err)
	}
	// Reap it whenever it goes, so it never lingers as a zombie.
	go cmd.Wait()

	return func() { ourEnd.Close() }, nil
}

// Main is the guard's entry point: `tsgonest __parent-guard <child pid>`, with
// the pipe on fd 3.
func Main(args []string) int {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "usage: tsgonest %s <child pid>\n", Command)
		return 2
	}
	childPid, err := strconv.Atoi(args[0])
	if err != nil || childPid <= 0 {
		fmt.Fprintf(os.Stderr, "%s: bad child pid %q\n", Command, args[0])
		return 2
	}
	parent := os.NewFile(3, "tsgonest")
	if parent == nil {
		fmt.Fprintf(os.Stderr, "%s: no pipe on fd 3\n", Command)
		return 2
	}
	// The group is the target of every signal the runner and the guard itself
	// send, and the guard is in it. It leaves through the liveness check
	// instead, within livenessInterval of the child; SIGKILL to the group
	// still takes it out, which is fine either way. Set here, in the guard's
	// own process, never inside Guard: a disposition is inherited by every
	// child spawned afterwards.
	signal.Ignore(syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	Guard(childPid, parent, os.Stderr)
	return 0
}

// Guard blocks until either the child is gone or the pipe reads EOF. On EOF
// with the child still alive, tsgonest died first: the child's group gets
// SIGTERM, the grace period, then SIGKILL.
func Guard(childPid int, parent io.Reader, stderr io.Writer) {
	eof := make(chan struct{})
	go func() {
		io.Copy(io.Discard, parent)
		close(eof)
	}()

	tick := time.NewTicker(livenessInterval)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			if !alive(childPid) {
				return
			}
		case <-eof:
			if !alive(childPid) {
				return
			}
			fmt.Fprintln(stderr, "tsgonest dev is gone; stopping the app it left running")
			syscall.Kill(-childPid, syscall.SIGTERM)
			deadline := time.Now().Add(gracePeriod)
			for time.Now().Before(deadline) {
				if !alive(childPid) {
					return
				}
				time.Sleep(50 * time.Millisecond)
			}
			// Takes this process with it, which is the intended end.
			syscall.Kill(-childPid, syscall.SIGKILL)
			return
		}
	}
}

// alive reports whether the child still exists. Once its parent is gone the
// child is reparented and reaped by init, so a dead child answers ESRCH here
// rather than lingering as a zombie.
func alive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}
