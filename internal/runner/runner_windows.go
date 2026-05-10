//go:build windows

package runner

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// generateConsoleCtrlEvent is a package-level variable so tests can inject a
// fake implementation to exercise the error-path without a real console.
var generateConsoleCtrlEvent = func(event uint32, pid uint32) error {
	return windows.GenerateConsoleCtrlEvent(event, pid)
}

const createSuspended = 0x00000004

// startProcess is overridable in tests to exercise the breakaway-denied retry path.
var startProcess = func(cmd *exec.Cmd) error { return cmd.Start() }

var (
	modKernel32      = windows.NewLazySystemDLL("kernel32.dll")
	procResumeThread = modKernel32.NewProc("ResumeThread")
)

// openThreadFn and resumeFn are package-level seams so unit tests can
// inject deterministic open/resume behavior without spawning a real
// suspended Windows process. Production code uses the real syscalls.
var (
	openThreadFn = openThreadForResume
	resumeFn     = resumeThreadChecked
)

func openThreadForResume(tid uint32) (windows.Handle, error) {
	return windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, tid)
}

// resumeThreadChecked invokes ResumeThread and treats the documented
// failure return (-1 / 0xFFFFFFFF) as an error even if the wrapper does
// not surface one. Returns the previous suspend count on success.
func resumeThreadChecked(h windows.Handle) (uint32, error) {
	r1, _, e1 := procResumeThread.Call(uintptr(h))
	if uint32(r1) == 0xFFFFFFFF {
		if e1 != nil && e1 != syscall.Errno(0) {
			return 0, e1
		}
		return 0, fmt.Errorf("ResumeThread failed (handle %v)", h)
	}
	return uint32(r1), nil
}

// Start starts the child process inside a Windows Job Object.
// The process is created suspended, assigned to the Job Object, then resumed.
// This eliminates the race where the child could spawn grandchildren before
// being assigned to the job. The Job Object is configured with
// JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE so all processes in the job are
// terminated when the handle is closed (including on parent death).
//
// CREATE_BREAKAWAY_FROM_JOB is requested first so tsgonest can run inside a
// CI-provided parent Job Object (GitHub Actions, TeamCity, Azure DevOps, some
// IDE task runners) without nested-job conflicts. If the parent job disallows
// breakaway, CreateProcess returns ERROR_ACCESS_DENIED; we then rebuild the
// *exec.Cmd (Start can only be called once) and retry without the flag, which
// matches Node.js's behavior for spawned children on Windows.
func (r *Runner) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.stopped {
		return ErrRunnerStopped
	}

	baseFlags := uint32(syscall.CREATE_NEW_PROCESS_GROUP | createSuspended)

	r.cmd = r.newCmd()
	r.cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: baseFlags | windows.CREATE_BREAKAWAY_FROM_JOB,
	}

	// Create a Job Object with KILL_ON_JOB_CLOSE.
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		r.cmd = nil
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
		r.cmd = nil
		return fmt.Errorf("configuring job object: %w", err)
	}

	if err := startProcess(r.cmd); err != nil {
		if isBreakawayDenied(err) {
			// Parent Job Object disallows breakaway; rebuild *exec.Cmd
			// (Start may only be called once per Cmd) and retry without the
			// CREATE_BREAKAWAY_FROM_JOB flag.
			r.cmd = r.newCmd()
			r.cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: baseFlags}
			err = startProcess(r.cmd)
		}
		if err != nil {
			windows.CloseHandle(job)
			r.cmd = nil
			return fmt.Errorf("starting process: %w", err)
		}
	}

	// Assign the suspended process to the Job Object BEFORE resuming it.
	// This ensures all grandchildren are also in the job.
	procHandle, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false,
		uint32(r.cmd.Process.Pid),
	)
	if err != nil {
		r.cleanupSuspendedChild(job, 0)
		return fmt.Errorf("opening process for job assignment: %w", err)
	}
	if err := windows.AssignProcessToJobObject(job, procHandle); err != nil {
		r.cleanupSuspendedChild(job, procHandle)
		return fmt.Errorf("assigning process to job object: %w", err)
	}
	windows.CloseHandle(procHandle)

	r.jobHandle = uintptr(job)

	// Resume the process now that it's in the Job Object.
	if err := resumeProcessThreads(r.cmd.Process.Pid); err != nil {
		r.cleanupSuspendedChild(job, 0)
		return fmt.Errorf("resuming process: %w", err)
	}

	// Allocate r.done and spawn the wait goroutine ONLY after all error-prone
	// setup has succeeded. If any step above fails, r.done stays as it was
	// (nil on first Start, or the closed channel from the previous run), so
	// Wait() will not block forever waiting for a goroutine that was never
	// spawned. See issue #144.
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
		r.closeJob()
		return nil
	}

	// If r.done is nil, Start() failed after cmd.Start() succeeded but before
	// the wait goroutine was spawned (issue #144). The Start() error path
	// already killed the process, so just clean up handles and return.
	if r.done == nil {
		r.cmd.Process.Kill()
		r.closeJob()
		r.cmd = nil
		return nil
	}

	// Try graceful shutdown via CTRL_BREAK_EVENT.
	// Node.js handles this signal and can run cleanup code.
	if err := generateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(r.cmd.Process.Pid)); err != nil {
		// Signal not delivered (no console attached, wrong process group, etc.).
		// Waiting 5s would be futile — r.done will never close on its own.
		r.cmd.Process.Kill()
		<-r.done
		r.closeJob()
		r.cmd = nil
		return nil
	}

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

// cleanupSuspendedChild reaps a suspended child created by Start() when one of
// the post-Start setup steps (OpenProcess, AssignProcessToJobObject, thread
// resume) fails. TerminateProcess alone leaves the kernel process slot alive
// because *os.Process still owns a handle; without Wait() the handle leaks
// until Runner is garbage collected, and r.cmd would otherwise be left
// pointing at a dead Cmd that subsequent calls would treat as live.
func (r *Runner) cleanupSuspendedChild(job windows.Handle, procHandle windows.Handle) {
	if r.cmd != nil && r.cmd.Process != nil {
		_ = r.cmd.Process.Kill()
		_ = r.cmd.Wait()
	}
	if procHandle != 0 {
		windows.CloseHandle(procHandle)
	}
	if job != 0 {
		windows.CloseHandle(job)
	}
	r.cmd = nil
	r.jobHandle = 0
}

// isBreakawayDenied reports whether err is the ERROR_ACCESS_DENIED that
// CreateProcess returns when CREATE_BREAKAWAY_FROM_JOB is requested but the
// parent Job Object's JOB_OBJECT_LIMIT_BREAKAWAY_OK flag is not set.
func isBreakawayDenied(err error) bool {
	return err != nil && errors.Is(err, syscall.ERROR_ACCESS_DENIED)
}

// resumeProcessThreads enumerates the threads owned by pid and resumes each.
// It distinguishes "no threads found" (snapshot or filter empty), "all resumes
// failed" (every ResumeThread returned -1), and partial success (some resumed,
// some failed — likely benign because the main thread usually succeeds first).
func resumeProcessThreads(pid int) error {
	tids, err := snapshotThreadIDsForPID(uint32(pid))
	if err != nil {
		return err
	}
	return resumeThreadIDs(pid, tids)
}

func snapshotThreadIDsForPID(pid uint32) ([]uint32, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return nil, fmt.Errorf("creating thread snapshot: %w", err)
	}
	defer windows.CloseHandle(snapshot)

	var te windows.ThreadEntry32
	te.Size = uint32(unsafe.Sizeof(te))

	var tids []uint32
	err = windows.Thread32First(snapshot, &te)
	for err == nil {
		if te.OwnerProcessID == pid {
			tids = append(tids, te.ThreadID)
		}
		err = windows.Thread32Next(snapshot, &te)
	}
	return tids, nil
}

func resumeThreadIDs(pid int, tids []uint32) error {
	opened := 0
	resumed := 0
	var firstErr error

	for _, tid := range tids {
		h, openErr := openThreadFn(tid)
		if openErr != nil {
			if firstErr == nil {
				firstErr = openErr
			}
			continue
		}
		opened++
		_, resumeErr := resumeFn(h)
		windows.CloseHandle(h)
		if resumeErr != nil {
			if firstErr == nil {
				firstErr = resumeErr
			}
			continue
		}
		resumed++
	}

	if opened == 0 {
		if firstErr != nil {
			return fmt.Errorf("no threads opened for pid %d: %w", pid, firstErr)
		}
		return fmt.Errorf("no threads found for pid %d", pid)
	}
	if resumed == 0 {
		return fmt.Errorf("failed to resume any thread for pid %d: %w", pid, firstErr)
	}
	if resumed < opened {
		fmt.Fprintf(os.Stderr, "tsgonest: resumed %d/%d threads for pid %d (first failure: %v)\n",
			resumed, opened, pid, firstErr)
	}
	return nil
}
