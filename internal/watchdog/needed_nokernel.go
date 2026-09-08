//go:build !linux && !windows

package watchdog

// Needed is true on the platforms with no kernel mechanism for parent-death
// cleanup: macOS and the BSDs have no Pdeathsig and no Job Objects.
const Needed = true
