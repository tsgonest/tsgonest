package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	shimcompiler "github.com/microsoft/typescript-go/shim/compiler"
	shimincremental "github.com/microsoft/typescript-go/shim/execute/incremental"
	"github.com/tsgonest/tsgonest/internal/compiler"
	"github.com/tsgonest/tsgonest/internal/config"
	"github.com/tsgonest/tsgonest/internal/runner"
	"github.com/tsgonest/tsgonest/internal/watcher"
)

// devBuilder holds build state across dev-mode rebuild cycles.
// It retains both the base compiler program (for UpdateProgram fast path)
// and the incremental program (for diagnostic/emit caching) in memory.
type devBuilder struct {
	mu          sync.Mutex
	incrProgram *shimincremental.Program
	baseProgram *shimcompiler.Program // retained for UpdateProgram fast path
	buildArgs   []string
	cwd         string
}

// Build runs a full build cycle with in-memory incremental reuse.
// Used for multi-file changes or when the fast path isn't applicable.
func (b *devBuilder) Build() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.fullBuild()
}

// BuildSingleFile attempts a fast single-file rebuild using UpdateProgram.
// If the changed file's imports/structure didn't change, this avoids re-reading
// and re-parsing all source files — only the changed file is re-read.
// Falls back to a full build if UpdateProgram can't handle the change.
func (b *devBuilder) BuildSingleFile(changedFile string) int {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.baseProgram == nil {
		return b.fullBuild()
	}

	fs := compiler.CreateDefaultFS()
	updatedProgram, reused := compiler.UpdateProgram(b.baseProgram, changedFile, b.cwd, fs)
	if !reused || updatedProgram == nil {
		// Structural change (new imports, etc.) — fall back to full rebuild
		return b.fullBuild()
	}

	// Fast path: run the full pipeline with the updated program
	b.baseProgram = updatedProgram
	return runBuildWithIncrAndProgram(b.buildArgs, b.incrProgram, &b.incrProgram, updatedProgram)
}

func (b *devBuilder) fullBuild() int {
	code := runBuildWithIncr(b.buildArgs, b.incrProgram, &b.incrProgram)
	// Extract the base program from the incremental program for next UpdateProgram call
	if b.incrProgram != nil {
		b.baseProgram = b.incrProgram.Program()
	}
	return code
}

// devFlags holds parsed CLI flags for the dev command.
// These are parsed once and remain constant across config restarts.
type devFlags struct {
	configPath          string
	tsconfigPath        string
	runtime             string
	execCmd             string
	entryPoint          string
	debugFlag           string
	envFile             string
	preserveWatchOutput bool
	noSourceMaps        bool
	verbose             bool
	passthroughArgs     []string
	cwd                 string
}

// devLoopResult indicates why the dev loop exited.
type devLoopResult int

const (
	devLoopExit    devLoopResult = iota // Normal exit (signal received)
	devLoopRestart                      // Config file changed — restart everything
)

// runDev implements the "tsgonest dev" command: build, start, and watch+reload.
func runDev(args []string) int {
	// Split args at "--" to separate our flags from passthrough args
	devArgs, passthroughArgs := splitArgs(args)

	fs := flag.NewFlagSet("dev", flag.ExitOnError)

	var flags devFlags
	flags.passthroughArgs = passthroughArgs

	fs.StringVar(&flags.configPath, "config", "", "Path to tsgonest config file")
	fs.StringVar(&flags.tsconfigPath, "project", "tsconfig.json", "Path to tsconfig.json")
	fs.StringVar(&flags.tsconfigPath, "p", "tsconfig.json", "Path to tsconfig.json (shorthand)")
	fs.StringVar(&flags.runtime, "runtime", "", "Runtime to use: node (default) or bun")
	fs.StringVar(&flags.execCmd, "exec", "", "Custom command to run instead of the runtime")
	fs.StringVar(&flags.entryPoint, "entry", "", "Entry point file (default: auto-detect from dist)")
	fs.StringVar(&flags.debugFlag, "debug", "", "Enable Node.js debugging (use: --debug=9229, --debug=0.0.0.0:9229, or just --debug=true)")
	fs.StringVar(&flags.envFile, "env-file", "", "Path to .env file to load")
	fs.BoolVar(&flags.preserveWatchOutput, "preserveWatchOutput", false, "Don't clear console between rebuilds")
	fs.BoolVar(&flags.noSourceMaps, "no-source-maps", false, "Disable --enable-source-maps")
	fs.BoolVar(&flags.verbose, "verbose", false, "Print changed file paths that trigger rebuilds")

	fs.Usage = func() {
		fmt.Println("Usage: tsgonest dev [flags] [-- <runtime args>]")
		fmt.Println()
		fmt.Println("Flags:")
		fs.PrintDefaults()
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  tsgonest dev")
		fmt.Println("  tsgonest dev --runtime bun")
		fmt.Println("  tsgonest dev --debug")
		fmt.Println("  tsgonest dev --debug 0.0.0.0:9229")
		fmt.Println("  tsgonest dev --env-file .env.local")
		fmt.Println("  tsgonest dev --verbose")
		fmt.Println("  tsgonest dev -- --max-old-space-size=4096")
	}

	fs.Parse(devArgs)

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: could not get working directory: %v\n", err)
		return 1
	}
	flags.cwd = cwd

	// Handle SIGINT/SIGTERM/SIGHUP across all loop iterations.
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// Run the dev loop. It restarts from scratch when config files change.
	for {
		result := runDevLoop(&flags, sigCh)
		if result != devLoopRestart {
			return 0
		}
		pretty := compiler.IsPrettyOutput()
		printStatus(os.Stderr, pretty, "◆", "config changed, restarting...")
	}
}

// runDevLoop runs one iteration of the dev loop: load config, build, start
// child, watch source files, and poll config files. Returns devLoopRestart
// if a config file changed, or devLoopExit if a signal was received.
func runDevLoop(flags *devFlags, sigCh chan os.Signal) devLoopResult {
	cwd := flags.cwd
	pretty := compiler.IsPrettyOutput()

	// Load config
	cfgResult, cfgErr := loadOrDiscoverConfig(flags.configPath, cwd)
	if cfgErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", cfgErr)
		return devLoopExit
	}
	cfg := cfgResult.Config

	// Resolve entryFile: CLI flag > config > auto-detect
	entryPoint := flags.entryPoint
	if entryPoint == "" && cfg != nil && cfg.EntryFile != "" {
		entryPoint = cfg.EntryFile
	}

	manualRestart := cfg != nil && cfg.ManualRestart
	deleteOutDir := cfg != nil && cfg.DeleteOutDir

	resolvedConfigPath := flags.configPath
	if resolvedConfigPath == "" && cfgResult.Path != "" {
		resolvedConfigPath = cfgResult.Path
	}

	// Build args for watch rebuilds (no --clean).
	// --no-clean suppresses cfg.DeleteOutDir — incremental rebuilds must NOT
	// wipe dist/ or .tsbuildinfo, otherwise (a) the incremental fast path is
	// defeated and (b) a freshly restarted node racing on lazy require() can
	// land in an empty dist/ during the next rebuild's clean window.
	watchBuildArgs := []string{"--no-clean"}
	if resolvedConfigPath != "" {
		watchBuildArgs = append(watchBuildArgs, "--config", resolvedConfigPath)
	}
	watchBuildArgs = append(watchBuildArgs, "--project", flags.tsconfigPath)

	// Fresh devBuilder — no incremental reuse across config restarts
	builder := &devBuilder{buildArgs: watchBuildArgs, cwd: cwd}

	// Initial build
	printStatus(os.Stderr, pretty, "◆", "performing initial build...")
	initialBuildArgs := append([]string{}, watchBuildArgs...)
	if deleteOutDir {
		initialBuildArgs = append(initialBuildArgs, "--clean")
	}

	builder.mu.Lock()
	buildResult := runBuildWithIncr(initialBuildArgs, nil, &builder.incrProgram)
	builder.mu.Unlock()
	if buildResult != 0 {
		printStatus(os.Stderr, pretty, "✗", "initial build failed, watching for changes...")
	} else {
		printStatus(os.Stderr, pretty, "✓", "initial build succeeded")
	}

	// Determine entry point (after build, so dist/ exists)
	if entryPoint == "" {
		entryPoint = detectEntryPoint(cwd)
	} else {
		sourceRoot := "src"
		if cfg != nil && cfg.SourceRoot != "" {
			sourceRoot = cfg.SourceRoot
		}
		entryPoint = resolveEntryPoint(entryPoint, cwd, sourceRoot, os.Stat)
	}

	// Resolve runtime: CLI flag > config > default "node"
	runtimeName := resolveRuntime(flags.runtime, cfg)

	// Build runtime args
	var proc *runner.Runner
	if flags.execCmd != "" {
		shell, shellArgs := pickExecShell(flags.execCmd)
		proc = runner.New(shell, shellArgs, cwd)
	} else if entryPoint != "" {
		runtimeArgs := buildRuntimeArgs(runtimeName, entryPoint, flags.debugFlag, flags.envFile, flags.noSourceMaps, flags.passthroughArgs)
		proc = runner.New(runtimeName, runtimeArgs, cwd)
	}

	if proc != nil {
		proc.DisableStdin = true
	}

	if proc != nil && buildResult == 0 {
		if flags.execCmd != "" {
			printStatus(os.Stderr, pretty, "▶", "starting: %s", flags.execCmd)
		} else {
			runtimeArgs := buildRuntimeArgs(runtimeName, entryPoint, flags.debugFlag, flags.envFile, flags.noSourceMaps, flags.passthroughArgs)
			printStatus(os.Stderr, pretty, "▶", "starting: %s %s", runtimeName, strings.Join(runtimeArgs, " "))
		}
		if err := proc.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "error starting process: %v\n", err)
		}
	}

	// Ensure child cleanup on return (covers both exit and restart paths).
	// Defer registration order matters here — defers run LIFO, so the order is:
	//   1. close(done) — signals helper goroutines to begin shutdown
	//   2. wg.Wait()   — joins the rebuild goroutine before we touch proc
	//   3. proc.Stop() — only after the rebuild goroutine has exited, so it
	//      cannot race with us by calling proc.Restart() after Stop.
	// Registering proc.Stop first means it runs last.
	defer func() {
		if proc != nil {
			proc.Stop()
		}
	}()

	// Catch panics to prevent orphan processes
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "tsgonest dev: panic: %v\n", r)
		}
	}()

	// Watch source files
	srcDir := filepath.Join(cwd, "src")
	if cfg != nil && cfg.SourceRoot != "" {
		srcDir = filepath.Join(cwd, cfg.SourceRoot)
	}
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		srcDir = cwd
	}

	if flags.verbose {
		printStatus(os.Stderr, pretty, "◇", "watching directory: %s", srcDir)
		printStatus(os.Stderr, pretty, "◇", "watching extensions: %v", []string{".ts", ".tsx", ".mts", ".cts"})
	}

	// done is closed when runDevLoop returns, so helper goroutines can exit cleanly.
	// wg tracks the rebuild goroutine so we can join it before proc.Stop runs;
	// otherwise an in-flight rebuild can complete and call proc.Restart() after
	// Stop, spawning an orphan child.
	done := make(chan struct{})
	var wg sync.WaitGroup
	defer wg.Wait()
	defer close(done)

	// Channel-based rebuild loop: the watcher pushes debounced event batches
	// into changeCh. A single goroutine processes them sequentially, draining
	// any batches that arrived during a build before restarting the process.
	// This prevents the race condition where rapid multi-file saves (IDE
	// refactors, save-all) cause premature process restarts with a partially
	// built output directory.
	changeCh := make(chan []watcher.Event, 16)

	w := watcher.New(
		[]string{srcDir},
		[]string{".ts", ".tsx", ".mts", ".cts"},
		300*time.Millisecond,
		func(events []watcher.Event) {
			select {
			case changeCh <- events:
			case <-done:
			}
		},
	)

	// Rebuild loop goroutine. See rebuildLoop for the build/restart strategy.
	var procIface rebuildRestarter
	if proc != nil {
		procIface = proc
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		rebuildLoop(rebuildLoopDeps{
			changeCh:            changeCh,
			done:                done,
			builder:             builder,
			proc:                procIface,
			pending:             w.Pending,
			pretty:              pretty,
			verbose:             flags.verbose,
			preserveWatchOutput: flags.preserveWatchOutput,
			manualRestart:       manualRestart,
		})
	}()

	// Watch config files for changes. When tsconfig.json or tsgonest.config
	// changes, we need to restart the entire dev loop (re-parse config,
	// fresh incremental program, potentially different source root/entry point).
	configChanged := make(chan string, 1)
	var configFiles []string
	if resolvedConfigPath != "" {
		configFiles = append(configFiles, resolvedConfigPath)
	}
	// Resolve tsconfig to absolute path for reliable polling
	tsconfigAbs := flags.tsconfigPath
	if !filepath.IsAbs(tsconfigAbs) {
		tsconfigAbs = filepath.Join(cwd, tsconfigAbs)
	}
	configFiles = append(configFiles, tsconfigAbs)

	// Track why the watcher stopped: signal vs config change.
	// Using atomic bool avoids the race where the signal handler goroutine
	// and the post-Watch select both try to read from configChanged.
	var restartRequested atomic.Bool

	stopConfigPoller := watcher.WatchFiles(configFiles, 500*time.Millisecond, func(path string) {
		select {
		case configChanged <- path:
		default:
			// Already a pending config change — don't block
		}
	})
	defer stopConfigPoller()

	// Signal handler for this loop iteration
	go func() {
		select {
		case <-sigCh:
			fmt.Fprintln(os.Stderr, "\nshutting down...")
			w.Stop()

			stopDone := make(chan struct{})
			go func() {
				if proc != nil {
					proc.Stop()
				}
				close(stopDone)
			}()

			select {
			case <-stopDone:
			case <-sigCh:
				fmt.Fprintln(os.Stderr, "\nforce killing...")
				os.Exit(1)
			}
		case path := <-configChanged:
			// Config changed — stop the watcher to return from Watch()
			restartRequested.Store(true)
			base := filepath.Base(path)
			printStatus(os.Stderr, pretty, "↻", "%s changed, full restart required", base)
			w.Stop()
		case <-done:
			return
		}
	}()

	// Manual restart listener
	if manualRestart {
		go func() {
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				select {
				case <-done:
					return
				default:
				}
				line := strings.TrimSpace(scanner.Text())
				if line == "rs" {
					fmt.Fprintln(os.Stderr, "\nmanual restart triggered...")
					result := builder.Build()
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

	printStatus(os.Stderr, pretty, "◇", "watching for changes...")
	watchErr := w.Watch()

	if restartRequested.Load() {
		return devLoopRestart
	}
	if gone, ok := watchErr.(*watcher.ErrWatchRootGone); ok {
		printStatus(os.Stderr, pretty, "✗", "watched directory %q was renamed or removed; please restart `tsgonest dev`", gone.Path)
	} else if watchErr != nil {
		fmt.Fprintf(os.Stderr, "watcher error: %v\n", watchErr)
	}
	return devLoopExit
}

// rebuildBuilder is the subset of devBuilder used by rebuildLoop. Defined as
// an interface so tests can substitute a fake builder without touching tsgo.
type rebuildBuilder interface {
	Build() int
	BuildSingleFile(path string) int
}

// rebuildRestarter is the subset of *runner.Runner used by rebuildLoop.
// Restart() must NOT be called after the dev loop has signalled shutdown
// (see rebuildLoop), because runner.Runner.Restart clears its internal
// "stopped" flag and would spawn a fresh child after the dev loop's
// deferred Stop has run, leaking an orphan process.
type rebuildRestarter interface {
	Restart() error
}

// rebuildLoopDeps bundles the inputs to rebuildLoop. It exists to keep the
// long parameter list manageable and to give tests a single struct to fill in.
type rebuildLoopDeps struct {
	changeCh            <-chan []watcher.Event
	done                <-chan struct{}
	builder             rebuildBuilder
	proc                rebuildRestarter
	pending             func() bool
	pretty              bool
	verbose             bool
	preserveWatchOutput bool
	manualRestart       bool
}

// rebuildLoop is the body of the dev rebuild goroutine. After each debounced
// batch it builds, then drains any changes that arrived during the build. If
// nothing else is pending it restarts the child process. Crucially it checks
// d.done immediately before calling proc.Restart() — without that check, an
// in-flight build could complete after the dev loop began shutting down and
// spawn a fresh child after proc.Stop, leaking an orphan.
func rebuildLoop(d rebuildLoopDeps) {
	for {
		select {
		case events := <-d.changeCh:
			if !d.preserveWatchOutput {
				fmt.Fprint(os.Stderr, "\033[2J\033[H")
			}

			printStatus(os.Stderr, d.pretty, "◆", "detected %d change(s), rebuilding...", len(events))

			if d.verbose {
				for _, ev := range events {
					fmt.Fprintf(os.Stderr, "  [%s] %s\n", ev.Op, ev.Path)
				}
			}

			var result int
			if len(events) == 1 && events[0].Op == "write" {
				result = d.builder.BuildSingleFile(events[0].Path)
			} else {
				result = d.builder.Build()
			}

			changedDuringBuild := false
		drainLoop:
			for {
				select {
				case <-d.changeCh:
					changedDuringBuild = true
				default:
					break drainLoop
				}
			}
			if d.pending != nil && d.pending() {
				changedDuringBuild = true
			}

			if changedDuringBuild {
				printStatus(os.Stderr, d.pretty, "↻", "changes detected during build, debouncing...")
				continue
			}

			select {
			case <-d.done:
				return
			default:
			}

			if result != 0 {
				printStatus(os.Stderr, d.pretty, "✗", "build failed, watching for changes...")
			} else if d.proc != nil {
				printStatus(os.Stderr, d.pretty, "▶", "restarting...")
				if err := d.proc.Restart(); err != nil {
					fmt.Fprintf(os.Stderr, "error restarting: %v\n", err)
				}
			}

			if d.manualRestart {
				fmt.Fprintln(os.Stderr, "To restart at any time, enter \"rs\".")
			}

		case <-d.done:
			return
		}
	}
}

// resolveRuntime determines the runtime to use.
// Priority: CLI flag > config > default "node".
func resolveRuntime(cliFlag string, cfg *config.Config) string {
	if cliFlag != "" {
		return cliFlag
	}
	if cfg != nil && cfg.Runtime != "" {
		return cfg.Runtime
	}
	return "node"
}

// buildRuntimeArgs constructs the arguments for the runtime process (node or bun).
// Automatically includes runtime-appropriate flags for source maps, debugging, and env files.
func buildRuntimeArgs(runtime, entryPoint string, debugFlag string, envFile string, noSourceMaps bool, passthroughArgs []string) []string {
	var args []string

	isBun := runtime == "bun"

	// --enable-source-maps: Node.js only (Bun enables source maps by default and
	// errors on unknown V8 flags)
	if !isBun && !noSourceMaps {
		args = append(args, "--enable-source-maps")
	}

	// --inspect / --inspect=host:port (compatible with both Node and Bun)
	if debugFlag != "" {
		switch debugFlag {
		case "true", "1", "yes":
			args = append(args, "--inspect")
		default:
			args = append(args, "--inspect="+debugFlag)
		}
	}

	// --env-file (compatible with both Node and Bun)
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

func pickExecShellFor(goos, execCmd string) (string, []string) {
	if goos == "windows" {
		return "cmd", []string{"/C", execCmd}
	}
	return "sh", []string{"-c", execCmd}
}

func pickExecShell(execCmd string) (string, []string) {
	return pickExecShellFor(runtime.GOOS, execCmd)
}

// hasDistPrefix reports whether p already refers to a path inside (or equal to)
// the "dist" directory. Separator-normalized so that Windows backslash paths
// like "dist\main.js" are treated the same as "dist/main.js".
func hasDistPrefix(p string) bool {
	n := filepath.ToSlash(p)
	return strings.HasPrefix(n, "dist/") || n == "dist"
}

// resolveEntryPoint turns a relative, non-absolute entry point string into an
// absolute path under <cwd>/dist. statFn is injectable for testing.
//
// Rules:
//   - Absolute paths are returned unchanged.
//   - Paths that already start with "dist/" (or "dist\" on Windows) are
//     returned unchanged; they will be made absolute by the caller if needed.
//   - Otherwise: append ".js" if missing, probe <cwd>/dist/<sourceRoot>/<entry>,
//     fall back to <cwd>/dist/<entry>.
func resolveEntryPoint(entry, cwd, sourceRoot string, statFn func(string) (os.FileInfo, error)) string {
	if filepath.IsAbs(entry) {
		return entry
	}
	if hasDistPrefix(entry) {
		return entry
	}
	if !strings.HasSuffix(entry, ".js") {
		entry = entry + ".js"
	}
	withSR := filepath.Join(cwd, "dist", sourceRoot, entry)
	if _, err := statFn(withSR); err == nil {
		return withSR
	}
	return filepath.Join(cwd, "dist", entry)
}
