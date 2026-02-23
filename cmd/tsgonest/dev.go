package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/tsgonest/tsgonest/internal/config"
	"github.com/tsgonest/tsgonest/internal/runner"
	"github.com/tsgonest/tsgonest/internal/watcher"
)

// runDev implements the "tsgonest dev" command: build, start, and watch+reload.
// Mirrors nest start functionality with additional features:
//   - --debug: pass --inspect to node
//   - --env-file: pass --env-file to node
//   - rs manual restart (stdin listener)
//   - --enable-source-maps auto-enabled
//   - -- passthrough args to child process
func runDev(args []string) int {
	// Split args at "--" to separate our flags from passthrough args
	devArgs, passthroughArgs := splitArgs(args)

	devFlags := flag.NewFlagSet("dev", flag.ExitOnError)

	var (
		configPath          string
		tsconfigPath        string
		execCmd             string
		entryPoint          string
		debugFlag           string
		envFile             string
		preserveWatchOutput bool
		noSourceMaps        bool
	)

	devFlags.StringVar(&configPath, "config", "", "Path to tsgonest config file")
	devFlags.StringVar(&tsconfigPath, "project", "tsconfig.json", "Path to tsconfig.json")
	devFlags.StringVar(&tsconfigPath, "p", "tsconfig.json", "Path to tsconfig.json (shorthand)")
	devFlags.StringVar(&execCmd, "exec", "", "Custom command to run instead of Node.js")
	devFlags.StringVar(&entryPoint, "entry", "", "Entry point file (default: auto-detect from dist)")
	devFlags.StringVar(&debugFlag, "debug", "", "Enable Node.js debugging (use: --debug=9229, --debug=0.0.0.0:9229, or just --debug=true)")
	devFlags.StringVar(&envFile, "env-file", "", "Path to .env file to load")
	devFlags.BoolVar(&preserveWatchOutput, "preserveWatchOutput", false, "Don't clear console between rebuilds")
	devFlags.BoolVar(&noSourceMaps, "no-source-maps", false, "Disable --enable-source-maps")

	devFlags.Usage = func() {
		fmt.Println("Usage: tsgonest dev [flags] [-- <node args>]")
		fmt.Println()
		fmt.Println("Flags:")
		devFlags.PrintDefaults()
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  tsgonest dev")
		fmt.Println("  tsgonest dev --debug")
		fmt.Println("  tsgonest dev --debug 0.0.0.0:9229")
		fmt.Println("  tsgonest dev --env-file .env.local")
		fmt.Println("  tsgonest dev -- --max-old-space-size=4096")
	}

	devFlags.Parse(devArgs)

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: could not get working directory: %v\n", err)
		return 1
	}

	// Load config for entryFile, sourceRoot, manualRestart settings
	var cfg *config.Config
	if configPath != "" {
		resolvedConfigPath := configPath
		if !filepath.IsAbs(resolvedConfigPath) {
			resolvedConfigPath = filepath.Join(cwd, resolvedConfigPath)
		}
		cfg, err = config.Load(resolvedConfigPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
	} else {
		// Try default config file
		defaultPath := filepath.Join(cwd, "tsgonest.config.json")
		if _, statErr := os.Stat(defaultPath); statErr == nil {
			cfg, err = config.Load(defaultPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
		}
	}

	// Resolve entryFile: CLI flag > config > auto-detect
	if entryPoint == "" && cfg != nil && cfg.EntryFile != "" {
		entryPoint = cfg.EntryFile
	}

	// Enable manualRestart from config
	manualRestart := cfg != nil && cfg.ManualRestart

	// Check deleteOutDir from config (acts like --clean for initial build)
	deleteOutDir := cfg != nil && cfg.DeleteOutDir

	// Initial build
	fmt.Fprintln(os.Stderr, "performing initial build...")
	buildArgs := []string{}
	if configPath != "" {
		buildArgs = append(buildArgs, "--config", configPath)
	}
	buildArgs = append(buildArgs, "--project", tsconfigPath)
	if deleteOutDir {
		buildArgs = append(buildArgs, "--clean")
	}

	buildResult := runBuild(buildArgs)
	if buildResult != 0 {
		fmt.Fprintln(os.Stderr, "initial build failed, watching for changes...")
	} else {
		fmt.Fprintln(os.Stderr, "initial build succeeded")
	}

	// Determine entry point (after build, so dist/ exists)
	if entryPoint == "" {
		entryPoint = detectEntryPoint(cwd)
	} else if !filepath.IsAbs(entryPoint) && !strings.HasPrefix(entryPoint, "dist/") {
		// Resolve bare name like "main" → "dist/main.js"
		if !strings.HasSuffix(entryPoint, ".js") {
			entryPoint = entryPoint + ".js"
		}
		// Try dist/<sourceRoot>/<entryFile> first, then dist/<entryFile>
		sourceRoot := "src"
		if cfg != nil && cfg.SourceRoot != "" {
			sourceRoot = cfg.SourceRoot
		}
		withSR := filepath.Join(cwd, "dist", sourceRoot, entryPoint)
		if _, err := os.Stat(withSR); err == nil {
			entryPoint = withSR
		} else {
			entryPoint = filepath.Join(cwd, "dist", entryPoint)
		}
	}

	// Build node args
	var proc *runner.Runner
	if execCmd != "" {
		// Custom exec command
		proc = runner.New("sh", []string{"-c", execCmd}, cwd)
	} else if entryPoint != "" {
		nodeArgs := buildNodeArgs(entryPoint, debugFlag, envFile, noSourceMaps, passthroughArgs)
		proc = runner.New("node", nodeArgs, cwd)
	}

	if proc != nil && buildResult == 0 {
		if execCmd != "" {
			fmt.Fprintf(os.Stderr, "starting: %s\n", execCmd)
		} else {
			fmt.Fprintf(os.Stderr, "starting: node %s\n", strings.Join(buildNodeArgs(entryPoint, debugFlag, envFile, noSourceMaps, passthroughArgs), " "))
		}
		if err := proc.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "error starting process: %v\n", err)
		}
	}

	// Watch for changes
	srcDir := filepath.Join(cwd, "src")
	if cfg != nil && cfg.SourceRoot != "" {
		srcDir = filepath.Join(cwd, cfg.SourceRoot)
	}
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		srcDir = cwd
	}

	rebuild := func(events []watcher.Event) {
		if !preserveWatchOutput {
			// Clear terminal (like tsc --watch)
			fmt.Fprint(os.Stderr, "\033[2J\033[H")
		}

		fmt.Fprintf(os.Stderr, "\ndetected %d change(s), rebuilding...\n", len(events))

		result := runBuild(buildArgs)

		if result != 0 {
			fmt.Fprintln(os.Stderr, "build failed, waiting for changes...")
			if manualRestart {
				fmt.Fprintln(os.Stderr, "To restart at any time, enter \"rs\".")
			}
			return
		}

		if proc != nil {
			fmt.Fprintln(os.Stderr, "restarting...")
			if err := proc.Restart(); err != nil {
				fmt.Fprintf(os.Stderr, "error restarting: %v\n", err)
			}
		}

		if manualRestart {
			fmt.Fprintln(os.Stderr, "To restart at any time, enter \"rs\".")
		}
	}

	w := watcher.New(
		[]string{srcDir},
		[]string{".ts", ".tsx", ".mts", ".cts"},
		100*time.Millisecond,
		rebuild,
	)

	// Ensure child process is cleaned up on panic or unexpected exit.
	// This defer runs LIFO after signal handling, so panics don't leak
	// orphan processes.
	if proc != nil {
		defer func() {
			proc.Stop()
		}()
	}

	// Catch panics to ensure clean shutdown — without this, a panic in
	// rebuild/watcher goroutines could leave node processes running.
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "tsgonest dev: panic: %v\n", r)
		}
	}()

	// Handle SIGINT/SIGTERM/SIGHUP
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "\nshutting down...")
		w.Stop()
		if proc != nil {
			proc.Stop()
		}
	}()

	// Manual restart: listen for "rs" on stdin
	if manualRestart {
		go func() {
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "rs" {
					fmt.Fprintln(os.Stderr, "\nmanual restart triggered...")
					result := runBuild(buildArgs)
					if result != 0 {
						fmt.Fprintln(os.Stderr, "build failed, waiting for changes...")
					} else if proc != nil {
						fmt.Fprintln(os.Stderr, "restarting...")
						if err := proc.Restart(); err != nil {
							fmt.Fprintf(os.Stderr, "error restarting: %v\n", err)
						}
					}
					fmt.Fprintln(os.Stderr, "To restart at any time, enter \"rs\".")
				}
			}
		}()
		fmt.Fprintln(os.Stderr, "To restart at any time, enter \"rs\".")
	}

	fmt.Fprintln(os.Stderr, "watching for changes...")
	w.Watch()

	return 0
}

// buildNodeArgs constructs the arguments for the node process.
// Automatically includes --enable-source-maps, --inspect, --env-file as needed.
func buildNodeArgs(entryPoint string, debugFlag string, envFile string, noSourceMaps bool, passthroughArgs []string) []string {
	var args []string

	// --enable-source-maps (auto-enabled unless disabled)
	if !noSourceMaps {
		args = append(args, "--enable-source-maps")
	}

	// --inspect / --inspect=host:port
	if debugFlag != "" {
		switch debugFlag {
		case "true", "1", "yes":
			args = append(args, "--inspect")
		default:
			args = append(args, "--inspect="+debugFlag)
		}
	}

	// --env-file
	if envFile != "" {
		args = append(args, "--env-file="+envFile)
	}

	// Passthrough args (everything after --)
	args = append(args, passthroughArgs...)

	// Entry point is always last
	args = append(args, entryPoint)

	return args
}

// splitArgs splits args at "--" into our flags and passthrough args.
func splitArgs(args []string) (flags []string, passthrough []string) {
	for i, arg := range args {
		if arg == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
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
