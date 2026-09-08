// Package watchdog stops the app when tsgonest dies without warning.
//
// Linux has Pdeathsig and Windows has Job Objects, so on those platforms the
// kernel cleans up for us. macOS and the BSDs have neither: a `tsgonest dev`
// that is SIGKILLed, panics, or is torn down by a task runner leaves the app
// running with no parent, still holding its port, and the next `tsgonest dev`
// fails with EADDRINUSE.
//
// The guard is a second copy of this binary. It holds the read end of a pipe
// whose write end tsgonest keeps for the child's lifetime, and it sits in the
// child's process group. When the pipe reads EOF, tsgonest is gone, and the
// guard stops the group the way Runner.Stop would have. Nothing is loaded into
// the app and its argv is untouched, so it works the same for node, bun and
// `--exec` alike.
package watchdog

// Command is the hidden subcommand that runs the guard. main dispatches it;
// it is never listed in help.
const Command = "__parent-guard"
