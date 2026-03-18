package runner

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
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

	mu        sync.Mutex
	cmd       *exec.Cmd
	done      chan struct{}
	stopped   bool    // set by Stop(); prevents Start() after explicit shutdown
	jobHandle uintptr // Windows Job Object handle; unused on other platforms
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
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cmd == nil || r.cmd.Process == nil {
		return false
	}
	return r.cmd.ProcessState == nil || !r.cmd.ProcessState.Exited()
}
