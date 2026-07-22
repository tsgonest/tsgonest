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
		// close(done) first so a Stop/Restart triggered by the callback
		// sees the exited child immediately instead of waiting 5s.
		close(done)
		r.reportExit(cmd)
	}()

	return nil
}

// stop stops the child process gracefully, with a force-kill timeout.
//
// The mutex is released across the up-to-5s graceful-shutdown wait so that
// concurrent callers of Running(), Wait(), Restart() (and even another Stop())
// do not block behind us. r.stopFinished serializes Stop() callers: a second
// Stop() entering while the first is mid-wait waits on the initiator's
// stopFinished channel — which closes only after cleanup completes — so
// secondary callers never observe the runner mid-cleanup or race with a
// follow-up Restart's Start.
func (r *Runner) stop() error {
	r.mu.Lock()

	if r.cmd == nil || r.cmd.Process == nil {
		r.stopped = true
		r.mu.Unlock()
		return nil
	}

	// If r.done is nil, Start() failed after cmd.Start() succeeded but before
	// the wait goroutine was spawned (issue #144). Reap directly and return —
	// there is no wait goroutine to signal, so the stopFinished dance is
	// unnecessary.
	if r.done == nil {
		r.stopped = true
		r.cmd.Process.Kill()
		_, _ = r.cmd.Process.Wait()
		r.cmd = nil
		r.mu.Unlock()
		return nil
	}

	if r.stopFinished != nil {
		wait := r.stopFinished
		r.mu.Unlock()
		<-wait
		return nil
	}

	finished := make(chan struct{})
	r.stopFinished = finished
	r.stopped = true
	cmd := r.cmd
	done := r.done
	r.mu.Unlock()

	pgid, pgidErr := syscall.Getpgid(cmd.Process.Pid)
	if pgidErr == nil {
		if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil && err != syscall.ESRCH {
			r.mu.Lock()
			r.stopFinished = nil
			r.mu.Unlock()
			close(finished)
			return fmt.Errorf("sending SIGTERM to process group: %w", err)
		}
	} else {
		cmd.Process.Signal(syscall.SIGTERM)
	}

	var retErr error
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		if pgidErr == nil {
			if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
				retErr = fmt.Errorf("force-killing process group: %w", err)
			}
		} else {
			cmd.Process.Kill()
		}
		<-done
	}

	r.mu.Lock()
	r.cmd = nil
	r.stopFinished = nil
	r.mu.Unlock()
	close(finished)
	return retErr
}
