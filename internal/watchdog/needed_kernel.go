//go:build linux || windows

package watchdog

// Needed is false where the kernel already cleans up: Pdeathsig on Linux, the
// Job Object's KILL_ON_JOB_CLOSE on Windows. Injecting a preload flag into the
// app's runtime there would buy nothing.
const Needed = false
