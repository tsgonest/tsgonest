//go:build !windows

package watchdog

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// A test binary cannot dispatch the hidden subcommand, so the guard is run
// through a test that checks for a marker in the environment, the way the
// runner's parent-death tests re-run themselves.
func testHelperCommand(childPid int) (*exec.Cmd, error) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestParentGuardHelper$")
	cmd.Env = append(os.Environ(),
		"WATCHDOG_TEST_HELPER=guard",
		"WATCHDOG_TEST_CHILD="+strconv.Itoa(childPid),
	)
	return cmd, nil
}

func TestParentGuardHelper(t *testing.T) {
	if os.Getenv("WATCHDOG_TEST_HELPER") != "guard" {
		t.Skip("helper entry point, only meaningful when re-run by Attach")
	}
	os.Exit(Main([]string{os.Getenv("WATCHDOG_TEST_CHILD")}))
}

// groupMembersExcept lists the pids in a process group other than the given
// one, via ps: the guard should be the only other member.
func groupMembersExcept(t *testing.T, pgid, except int) []int {
	t.Helper()
	out, err := exec.Command("ps", "-axo", "pid=,pgid=").Output()
	if err != nil {
		t.Fatalf("ps: %v", err)
	}
	var members []int
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		pid, _ := strconv.Atoi(fields[0])
		g, _ := strconv.Atoi(fields[1])
		if g == pgid && pid != except && syscall.Kill(pid, 0) == nil {
			members = append(members, pid)
		}
	}
	if len(members) == 0 {
		t.Logf("no other members in group %d:\n%s", pgid, out)
	}
	return members
}
