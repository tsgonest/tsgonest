//go:build !windows && !linux

package runner

import "syscall"

// sysProcAttr returns process attributes for non-Linux Unix systems (macOS, BSDs).
// Pdeathsig is not available on these platforms — there is no kernel mechanism
// to auto-terminate children when the parent dies.
func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true,
	}
}
