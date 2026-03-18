//go:build windows

package runner

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Start starts the child process inside a Windows Job Object.
// The Job Object is configured with JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE so
// all child processes are terminated when the job handle is closed (including
// on parent death).
func (r *Runner) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.stopped {
		return ErrRunnerStopped
	}

	r.cmd = r.newCmd()

	// CREATE_NEW_PROCESS_GROUP allows sending CTRL_BREAK_EVENT for graceful shutdown.
	r.cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}

	// Create a Job Object with KILL_ON_JOB_CLOSE.
	// When the last handle to this job is closed (including on parent crash),
	// all processes assigned to the job are terminated.
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return fmt.Errorf("creating job object: %w", err)
	}

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	_, err = windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	)
	if err != nil {
		windows.CloseHandle(job)
		return fmt.Errorf("configuring job object: %w", err)
	}

	r.done = make(chan struct{})

	if err := r.cmd.Start(); err != nil {
		windows.CloseHandle(job)
		return fmt.Errorf("starting process: %w", err)
	}

	// Assign the child process to the Job Object.
	// We need a process HANDLE (not PID) for AssignProcessToJobObject.
	procHandle, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false,
		uint32(r.cmd.Process.Pid),
	)
	if err == nil {
		if assignErr := windows.AssignProcessToJobObject(job, procHandle); assignErr != nil {
			// Assignment failed — job won't manage this process tree,
			// but the process itself is running. Log and continue.
			fmt.Fprintf(r.cmd.Stderr, "warning: could not assign process to job object: %v\n", assignErr)
		}
		windows.CloseHandle(procHandle)
	}

	r.jobHandle = uintptr(job)

	// Wait for process in background
	go func() {
		r.cmd.Wait()
		close(r.done)
	}()

	return nil
}

// stop stops the child process gracefully, with a force-kill timeout.
// It first sends CTRL_BREAK_EVENT for graceful shutdown, then falls back
// to TerminateProcess. The Job Object handle is closed in both paths,
// which also terminates any grandchild processes.
func (r *Runner) stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.stopped = true

	if r.cmd == nil || r.cmd.Process == nil {
		r.closeJob()
		return nil
	}

	// Try graceful shutdown via CTRL_BREAK_EVENT.
	// This works because the child was created with CREATE_NEW_PROCESS_GROUP.
	// Node.js handles this signal and can run cleanup code.
	windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(r.cmd.Process.Pid))

	select {
	case <-r.done:
		r.closeJob()
		r.cmd = nil
		return nil
	case <-time.After(5 * time.Second):
		// Graceful shutdown timed out — force kill
		r.cmd.Process.Kill()
		<-r.done
		r.closeJob()
		r.cmd = nil
		return nil
	}
}

// closeJob closes the Windows Job Object handle if it's open.
func (r *Runner) closeJob() {
	if r.jobHandle != 0 {
		windows.CloseHandle(windows.Handle(r.jobHandle))
		r.jobHandle = 0
	}
}
