package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// runMigrate resolves the bundled migrate.cjs script and invokes it via Node.js,
// passing through all CLI arguments, stdin, stdout, and stderr.
//
// Resolution order for migrate.cjs:
//  1. TSGONEST_MIGRATE_SCRIPT env var (explicit override)
//  2. Adjacent to this binary: <binary-dir>/migrate.cjs
//     (works both in npm installs — where the Go binary sits in packages/core/bin/ —
//     and in local dev — where `go build -o packages/core/bin/tsgonest-native`)
//  3. Relative to CWD: node_modules/tsgonest/bin/migrate.cjs
//     (fallback for global installs or unusual layouts)
func runMigrate(args []string) int {
	scriptPath, err := resolveMigrateScript()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tsgonest migrate: %s\n", err)
		fmt.Fprintln(os.Stderr, "Make sure 'tsgonest' is installed and 'node' is in your PATH.")
		return 1
	}

	nodePath, err := exec.LookPath("node")
	if err != nil {
		fmt.Fprintln(os.Stderr, "tsgonest migrate: node not found in PATH")
		fmt.Fprintln(os.Stderr, "Node.js is required for the migrate command.")
		return 1
	}

	// Build argv: node migrate.cjs [user args...]
	nodeArgs := append([]string{scriptPath}, args...)

	cmd := exec.Command(nodePath, nodeArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "tsgonest migrate: failed to execute: %s\n", err)
		return 1
	}
	return 0
}

// resolveMigrateScript finds migrate.cjs using the resolution order documented above.
func resolveMigrateScript() (string, error) {
	// 1. Explicit override via environment variable
	if envPath := os.Getenv("TSGONEST_MIGRATE_SCRIPT"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath, nil
		}
		return "", fmt.Errorf("TSGONEST_MIGRATE_SCRIPT=%q does not exist", envPath)
	}

	// 2. Adjacent to the running binary (npm install layout: bin/ contains both)
	binDir, err := selfDir()
	if err == nil {
		adjacent := filepath.Join(binDir, "migrate.cjs")
		if _, err := os.Stat(adjacent); err == nil {
			return adjacent, nil
		}
		// 2b. Dev layout: binary at repo root, migrate.cjs in packages/core/bin/
		devPath := filepath.Join(binDir, "packages", "core", "bin", "migrate.cjs")
		if _, err := os.Stat(devPath); err == nil {
			return devPath, nil
		}
	}

	// 3. Walk up from the binary directory to find the enclosing node_modules,
	//    then resolve tsgonest/bin/migrate.cjs within it.
	//    This handles both regular npm installs and npx cache layouts:
	//      binary:     .../node_modules/@tsgonest/cli-darwin-arm64/bin/tsgonest
	//      migrate.cjs: .../node_modules/tsgonest/bin/migrate.cjs
	if binDir != "" {
		dir := binDir
		for {
			base := filepath.Base(dir)
			parent := filepath.Dir(dir)
			if base == "node_modules" {
				candidate := filepath.Join(dir, "tsgonest", "bin", "migrate.cjs")
				if _, err := os.Stat(candidate); err == nil {
					return candidate, nil
				}
			}
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	// 4. Fallback: node_modules/tsgonest/bin/migrate.cjs relative to CWD
	cwd, err := os.Getwd()
	if err == nil {
		nmPath := filepath.Join(cwd, "node_modules", "tsgonest", "bin", "migrate.cjs")
		if _, err := os.Stat(nmPath); err == nil {
			return nmPath, nil
		}
	}

	return "", fmt.Errorf("could not find migrate.cjs — looked in binary dir, node_modules tree, and CWD/node_modules/tsgonest/bin/")
}

// selfDir returns the directory containing the currently running executable,
// resolving symlinks (important for npm .bin/ symlinks).
func selfDir() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}

	// Resolve symlinks — npm creates symlinks in node_modules/.bin/
	// that point to the actual binary in packages/core/bin/
	resolved, err := filepath.EvalSymlinks(exePath)
	if err != nil {
		// Fall back to unresolved path
		resolved = exePath
	}

	// On macOS/Linux, also try /proc/self/exe or similar if available
	if runtime.GOOS == "linux" {
		if procPath, err := os.Readlink("/proc/self/exe"); err == nil {
			resolved = procPath
		}
	}

	return filepath.Dir(resolved), nil
}
