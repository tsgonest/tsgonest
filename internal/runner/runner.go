package runner

import (
	"os"
	"os/exec"
	"sync"
)

// Runner manages a child Node.js process.
type Runner struct {
	command string
	args    []string
	workDir string

	mu   sync.Mutex
	cmd  *exec.Cmd
	done chan struct{}
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
	cmd.Stdin = os.Stdin
	return cmd
}

// Restart stops and restarts the child process.
func (r *Runner) Restart() error {
	if err := r.Stop(); err != nil {
		return err
	}
	return r.Start()
}

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
