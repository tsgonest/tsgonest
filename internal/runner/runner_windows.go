//go:build windows

package runner

import (
	"fmt"
	"time"
)

// Start starts the child process.
func (r *Runner) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.cmd = r.newCmd()
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

// Stop stops the child process, with a force-kill timeout.
// Windows does not support process groups or SIGTERM, so we kill directly.
func (r *Runner) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cmd == nil || r.cmd.Process == nil {
		return nil
	}

	r.cmd.Process.Kill()

	select {
	case <-r.done:
		return nil
	case <-time.After(5 * time.Second):
		<-r.done
		return nil
	}
}
