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

	r.cmd = r.newCmd()

	// Set process group so we can kill all children
	r.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	r.done = make(chan struct{})

	if err := r.cmd.Start(); err != nil {
		return fmt.Errorf("starting process: %w", err)
	}

	// Wait for process in background
	go func() {
		r.cmd.Wait()
		close(r.done)
	}()

	return nil
}

// Stop stops the child process gracefully, with a force-kill timeout.
func (r *Runner) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cmd == nil || r.cmd.Process == nil {
		return nil
	}

	// Kill the process group
	pgid, err := syscall.Getpgid(r.cmd.Process.Pid)
	if err == nil {
		syscall.Kill(-pgid, syscall.SIGTERM)
	} else {
		r.cmd.Process.Signal(syscall.SIGTERM)
	}

	// Wait for it to stop (with timeout)
	select {
	case <-r.done:
		return nil
	case <-time.After(5 * time.Second):
		// Force kill
		if err == nil {
			syscall.Kill(-pgid, syscall.SIGKILL)
		} else {
			r.cmd.Process.Kill()
		}
		<-r.done
		return nil
	}
}
