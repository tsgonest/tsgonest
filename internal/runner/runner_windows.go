//go:build windows

package runner

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const createSuspended = 0x00000004

// Start starts the child process inside a Windows Job Object.
// The process is created suspended, assigned to the Job Object, then resumed.
// This eliminates the race where the child could spawn grandchildren before
// being assigned to the job. The Job Object is configured with
// JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE so all processes in the job are
// terminated when the handle is closed (including on parent death).
func (r *Runner) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.stopped {
		return ErrRunnerStopped
	}

	r.cmd = r.newCmd()

	// CREATE_NEW_PROCESS_GROUP: allows sending CTRL_BREAK_EVENT for graceful shutdown.
	// CREATE_SUSPENDED: process starts frozen so we can assign the Job Object before
	// any child processes are spawned.
	r.cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | createSuspended,
	}

	// Create a Job Object with KILL_ON_JOB_CLOSE.
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

	// Assign the suspended process to the Job Object BEFORE resuming it.
	// This ensures all grandchildren are also in the job.
	procHandle, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false,
		uint32(r.cmd.Process.Pid),
	)
	if err == nil {
		windows.AssignProcessToJobObject(job, procHandle)
		windows.CloseHandle(procHandle)
	}

	r.jobHandle = uintptr(job)

	// Resume the process now that it's in the Job Object.
	if err := resumeProcessThreads(r.cmd.Process.Pid); err != nil {
		// If resume fails, kill the process to avoid a suspended orphan.
		r.cmd.Process.Kill()
		windows.CloseHandle(job)
		r.jobHandle = 0
		return fmt.Errorf("resuming process: %w", err)
	}

	// Wait for process in background
	go func() {
		r.cmd.Wait()
		close(r.done)
	}()

	return nil
}

// stop stops the child process gracefully, with a force-kill timeout.
func (r *Runner) stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.stopped = true

	if r.cmd == nil || r.cmd.Process == nil {
		r.closeJob()
		return nil
	}

	// Try graceful shutdown via CTRL_BREAK_EVENT.
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

// closeJob closes the Windows Job Object handle.
// With JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE, this terminates all processes in the job.
func (r *Runner) closeJob() {
	if r.jobHandle != 0 {
		windows.CloseHandle(windows.Handle(r.jobHandle))
		r.jobHandle = 0
	}
}

// resumeProcessThreads enumerates and resumes all threads of a suspended process.
func resumeProcessThreads(pid int) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return fmt.Errorf("creating thread snapshot: %w", err)
	}
	defer windows.CloseHandle(snapshot)

	var te windows.ThreadEntry32
	te.Size = uint32(unsafe.Sizeof(te))

	err = windows.Thread32First(snapshot, &te)
	for err == nil {
		if te.OwnerProcessID == uint32(pid) {
			th, thErr := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, te.ThreadID)
			if thErr == nil {
				windows.ResumeThread(th)
				windows.CloseHandle(th)
			}
		}
		err = windows.Thread32Next(snapshot, &te)
	}
	return nil
}
