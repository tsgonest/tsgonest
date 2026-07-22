package runner

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
)

// Runner manages a child Node.js process.
type Runner struct {
	command string
	args    []string
	workDir string

	// DisableStdin prevents the child process from inheriting stdin.
	// When true, the child's stdin is set to nil (os.DevNull).
	// This is needed in dev mode so the parent can read stdin for
	// manual restart ("rs") commands without the child consuming input.
	DisableStdin bool

	// OnUnexpectedExit is invoked from the wait goroutine when the child
	// exits without Stop or Restart having initiated it (a crash, a clean
	// self-exit, or an external kill). The exit code is -1 when the child
	// was terminated by a signal. Set it before the first Start; it is not
	// synchronized against later mutation.
	OnUnexpectedExit func(exitCode int)

	mu      sync.Mutex
	cmd     *exec.Cmd
	done    chan struct{}
	stopped bool // set by Stop(); prevents Start() after explicit shutdown
	// stopFinished is non-nil while a stop() invocation is mid-flight.
	// It closes once the initiating stop() has finished both the kill wait
	// and its post-wait state cleanup. Concurrent Stop() callers see a
	// non-nil stopFinished and wait on it instead of issuing duplicate
	// signals. This is what releases the mutex across the up-to-5s wait
	// without exposing a window where another goroutine can observe the
	// runner mid-cleanup.
	stopFinished chan struct{}
	jobHandle    uintptr // Windows Job Object handle; unused on other platforms

	// alive is read by Running() without holding r.mu. The stdlib mutates
	// cmd.ProcessState from inside cmd.Wait() without our lock, so reading
	// ProcessState from Running() races with the Wait goroutine. Track liveness
	// explicitly: set true after a successful cmd.Start(), cleared by the Wait
	// goroutine on exit.
	alive atomic.Bool
}

// New creates a new process runner.
func New(command string, args []string, workDir string) *Runner {
	return &Runner{
		command: command,
		args:    args,
		workDir: workDir,
	}
}

func (r *Runner) newCmd() *exec.Cmd {
	cmd := exec.Command(r.command, r.args...)
	if r.workDir != "" {
		cmd.Dir = r.workDir
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if !r.DisableStdin {
		cmd.Stdin = os.Stdin
	}
	return cmd
}

// Restart stops and restarts the child process.
func (r *Runner) Restart() error {
	if err := r.Stop(); err != nil {
		return err
	}
	// Clear the stopped flag so Start() proceeds.
	// Stop() sets stopped=true to prevent accidental restarts after shutdown,
	// but Restart() explicitly intends to keep running.
	r.mu.Lock()
	if r.stopped {
		r.stopped = false
	}
	r.mu.Unlock()
	return r.Start()
}

// Stop stops the child process. After Stop(), Start() will return an error
// unless Restart() is used (which clears the stopped flag).
func (r *Runner) Stop() error {
	return r.stop()
}

// ErrRunnerStopped is returned by Start() when the runner has been explicitly
// stopped and a non-Restart Start() is attempted.
var ErrRunnerStopped = fmt.Errorf("runner has been stopped")

// Wait blocks until the child process exits.
func (r *Runner) Wait() {
	r.mu.Lock()
	done := r.done
	r.mu.Unlock()
	if done != nil {
		<-done
	}
}

// Running returns true if the child process is running.
func (r *Runner) Running() bool {
	return r.alive.Load()
}

// reportExit classifies a child exit after cmd.Wait returns. Exits initiated
// by Stop or Restart are expected (stop() sets stopped and stopFinished under
// r.mu before signaling the child); anything else fires OnUnexpectedExit.
// The callback runs without holding r.mu.
func (r *Runner) reportExit(cmd *exec.Cmd) {
	cb := r.OnUnexpectedExit
	if cb == nil {
		return
	}
	r.mu.Lock()
	expected := r.stopped || r.stopFinished != nil
	r.mu.Unlock()
	if expected {
		return
	}
	code := -1
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	}
	cb(code)
}
