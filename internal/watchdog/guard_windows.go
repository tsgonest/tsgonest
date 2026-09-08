//go:build windows

package watchdog

import (
	"fmt"
	"os"
)

// Attach is never called on Windows: the Job Object already kills the app
// when tsgonest goes, so Needed is false and the runner does not ask.
func Attach(childPid int) (release func(), err error) {
	return nil, fmt.Errorf("parent-death guard is not needed on windows")
}

// Main exists so the hidden subcommand dispatches on every platform.
func Main(args []string) int {
	fmt.Fprintf(os.Stderr, "%s is not used on windows\n", Command)
	return 2
}
