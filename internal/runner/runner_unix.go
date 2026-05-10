//go:build !windows

package runner

import (
	"fmt"
	"syscall"
	"time"
)

// Start starts the child process.
func (r *Runner) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.stopped {
		return ErrRunnerStopped
	}

	r.cmd = r.newCmd()
	r.cmd.SysProcAttr = sysProcAttr()

	if err := r.cmd.Start(); err != nil {
		r.cmd = nil
		return fmt.Errorf("starting process: %w", err)
	}

	// Allocate r.done and spawn the wait goroutine ONLY after all error-prone
	// setup has succeeded. If cmd.Start fails, r.done stays as it was so
	// Wait() will not block forever on a goroutine that was never spawned.
	// See issue #144.
	r.done = make(chan struct{})
	r.alive.Store(true)

	// Wait for process in background
	cmd := r.cmd
	done := r.done
	go func() {
		cmd.Wait()
		r.alive.Store(false)
		close(done)
	}()

	return nil
}

// stop stops the child process gracefully, with a force-kill timeout.
func (r *Runner) stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.stopped = true

	if r.cmd == nil || r.cmd.Process == nil {
		return nil
	}

	// If r.done is nil, Start() failed after cmd.Start() succeeded but before
	// the wait goroutine was spawned (issue #144). Reap directly and return.
	if r.done == nil {
		r.cmd.Process.Kill()
		_, _ = r.cmd.Process.Wait()
		r.cmd = nil
		return nil
	}

	// Kill the process group
	pgid, pgidErr := syscall.Getpgid(r.cmd.Process.Pid)
	if pgidErr == nil {
		if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil && err != syscall.ESRCH {
			return fmt.Errorf("sending SIGTERM to process group: %w", err)
		}
	} else {
		r.cmd.Process.Signal(syscall.SIGTERM)
	}

	// Wait for it to stop (with timeout)
	select {
	case <-r.done:
		r.cmd = nil
		return nil
	case <-time.After(5 * time.Second):
		// Force kill
		if pgidErr == nil {
			if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
				r.cmd = nil
				return fmt.Errorf("force-killing process group: %w", err)
			}
		} else {
			r.cmd.Process.Kill()
		}
		<-r.done
		r.cmd = nil
		return nil
	}
}
