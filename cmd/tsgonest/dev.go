package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/tsgonest/tsgonest/internal/runner"
	"github.com/tsgonest/tsgonest/internal/watcher"
)

// runDev implements the "tsgonest dev" command: build, start, and watch+reload.
// It depends on runBuild (defined in main.go or build.go by the CLI refactoring agent).
func runDev(args []string) int {
	devFlags := flag.NewFlagSet("dev", flag.ExitOnError)

	var (
		configPath   string
		tsconfigPath string
		execCmd      string
		entryPoint   string
	)

	devFlags.StringVar(&configPath, "config", "", "Path to tsgonest config file")
	devFlags.StringVar(&tsconfigPath, "project", "tsconfig.json", "Path to tsconfig.json")
	devFlags.StringVar(&tsconfigPath, "p", "tsconfig.json", "Path to tsconfig.json (shorthand)")
	devFlags.StringVar(&execCmd, "exec", "", "Custom command to run instead of Node.js")
	devFlags.StringVar(&entryPoint, "entry", "", "Entry point file (default: auto-detect from dist)")

	devFlags.Parse(args)

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: could not get working directory: %v\n", err)
		return 1
	}

	// Initial build
	fmt.Fprintln(os.Stderr, "performing initial build...")
	buildArgs := []string{}
	if configPath != "" {
		buildArgs = append(buildArgs, "--config", configPath)
	}
	buildArgs = append(buildArgs, "--project", tsconfigPath)

	buildResult := runBuild(buildArgs)
	if buildResult != 0 {
		fmt.Fprintln(os.Stderr, "initial build failed, watching for changes...")
	} else {
		fmt.Fprintln(os.Stderr, "initial build succeeded")
	}

	// Determine entry point
	if entryPoint == "" {
		entryPoint = detectEntryPoint(cwd)
	}

	// Start the application
	var proc *runner.Runner
	if execCmd != "" {
		// Custom exec command
		proc = runner.New("sh", []string{"-c", execCmd}, cwd)
	} else if entryPoint != "" {
		proc = runner.New("node", []string{entryPoint}, cwd)
	}

	if proc != nil && buildResult == 0 {
		if execCmd != "" {
			fmt.Fprintf(os.Stderr, "starting: %s\n", execCmd)
		} else {
			fmt.Fprintf(os.Stderr, "starting: node %s\n", entryPoint)
		}
		if err := proc.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "error starting process: %v\n", err)
		}
	}

	// Watch for changes
	srcDir := filepath.Join(cwd, "src")
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		srcDir = cwd
	}

	w := watcher.New(
		[]string{srcDir},
		[]string{".ts", ".tsx", ".mts", ".cts"},
		100*time.Millisecond,
		func(events []watcher.Event) {
			fmt.Fprintf(os.Stderr, "\ndetected %d change(s), rebuilding...\n", len(events))

			result := runBuild(buildArgs)

			if result != 0 {
				fmt.Fprintln(os.Stderr, "build failed, waiting for changes...")
				return
			}

			if proc != nil {
				fmt.Fprintln(os.Stderr, "restarting...")
				if err := proc.Restart(); err != nil {
					fmt.Fprintf(os.Stderr, "error restarting: %v\n", err)
				}
			}
		},
	)

	// Handle SIGINT/SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "\nshutting down...")
		w.Stop()
		if proc != nil {
			proc.Stop()
		}
	}()

	fmt.Fprintln(os.Stderr, "watching for changes...")
	w.Watch()

	return 0
}

// detectEntryPoint tries to determine the entry point from common output
// locations (NestJS convention: dist/main.js, general: dist/index.js).
func detectEntryPoint(cwd string) string {
	// Try dist/main.js (NestJS convention)
	nestEntry := filepath.Join(cwd, "dist", "main.js")
	if _, err := os.Stat(nestEntry); err == nil {
		return nestEntry
	}

	// Try dist/index.js
	indexEntry := filepath.Join(cwd, "dist", "index.js")
	if _, err := os.Stat(indexEntry); err == nil {
		return indexEntry
	}

	return ""
}
