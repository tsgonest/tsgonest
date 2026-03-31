package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/tsgonest/tsgonest/internal/analyzer"
	"github.com/tsgonest/tsgonest/internal/compiler"
	"github.com/tsgonest/tsgonest/internal/watcher"
)

// runCheck executes the "tsgonest check" command: type checking + tsgonest-specific
// analysis without emitting any files.
func runCheck(args []string) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	configPath := fs.String("config", "", "Path to tsgonest config file")
	tsconfigPath := fs.String("project", "tsconfig.json", "Path to tsconfig.json")
	fs.StringVar(tsconfigPath, "p", "tsconfig.json", "Path to tsconfig.json (shorthand)")
	noCheck := fs.Bool("no-check", false, "Skip semantic type checking (syntax errors still reported)")
	watch := fs.Bool("watch", false, "Watch mode — re-check on file changes")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *watch {
		return runCheckWatch(*configPath, *tsconfigPath, *noCheck)
	}
	return runCheckOnce(*configPath, *tsconfigPath, *noCheck)
}

// runCheckOnce runs a single check pass and returns the exit code.
func runCheckOnce(configPath, tsconfigPath string, noCheck bool) int {
	start := time.Now()
	pretty := compiler.IsPrettyOutput()

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: could not get working directory: %v\n", err)
		return 1
	}

	// Load config
	cfgResult, err := loadOrDiscoverConfig(configPath, cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	cfg := cfgResult.Config
	if cfgResult.Path != "" {
		printStatus(os.Stderr, pretty, "◆", "loaded config from %s", filepath.Base(cfgResult.Path))
	}

	// Parse tsconfig
	tsFS := compiler.CreateDefaultFS()
	host := compiler.CreateDefaultHost(cwd, tsFS)

	parsedConfig, diags, err := compiler.ParseTSConfig(tsFS, cwd, tsconfigPath, host, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if len(diags) > 0 {
		fmt.Fprint(os.Stderr, compiler.FormatDiagnostics(diags))
		return 1
	}

	// Create program
	program, programDiags, err := compiler.CreateProgramFromConfig(false, parsedConfig, host)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if len(programDiags) > 0 {
		fmt.Fprint(os.Stderr, compiler.FormatDiagnostics(programDiags))
		return 1
	}

	// Gather TypeScript diagnostics
	allDiags := compiler.GatherDiagnostics(program, noCheck)
	if len(allDiags) > 0 {
		reportDiag := compiler.CreateDiagnosticReporter(os.Stderr, cwd, pretty)
		for _, d := range allDiags {
			reportDiag(d)
		}
	}
	tsErrorCount := compiler.CountErrors(allDiags)

	// Run tsgonest-specific analysis if config is available and no TS errors
	var controllerWarnings []analyzer.Warning
	var walkerWarnings []string

	needControllers := cfg != nil && len(cfg.Controllers.Include) > 0
	if needControllers && tsErrorCount == 0 {
		sharedChecker, release := program.GetTypeChecker(context.Background())
		if sharedChecker == nil {
			fmt.Fprintln(os.Stderr, "error: could not get type checker")
			return 1
		}
		defer release()

		sharedWalker := analyzer.NewTypeWalker(sharedChecker)

		ca := analyzer.NewControllerAnalyzerWithWalker(program, sharedChecker, sharedWalker)
		controllers := ca.AnalyzeProgram(cfg.Controllers.Include, cfg.Controllers.Exclude)

		controllerWarnings = ca.Warnings()
		walkerWarnings = sharedWalker.Warnings()

		printStatus(os.Stderr, pretty, "✓", "analyzed %d controller(s)", len(controllers))
	}

	// Print tsgonest warnings
	allWarnings := make([]string, 0, len(controllerWarnings)+len(walkerWarnings))
	for _, w := range controllerWarnings {
		allWarnings = append(allWarnings, w.Message)
	}
	allWarnings = append(allWarnings, walkerWarnings...)

	if len(allWarnings) > 0 {
		fmt.Fprintln(os.Stderr)
		for _, w := range allWarnings {
			printStatus(os.Stderr, pretty, "●", "%s", w)
		}
	}

	// Summary
	duration := time.Since(start).Round(time.Millisecond)
	if tsErrorCount > 0 {
		printStatus(os.Stderr, pretty, "✗", "found %d error(s) in %s", tsErrorCount, duration)
		return 1
	}
	if len(allWarnings) > 0 {
		printStatus(os.Stderr, pretty, "✓", "no errors, %d warning(s) in %s", len(allWarnings), duration)
	} else {
		printStatus(os.Stderr, pretty, "✓", "no errors in %s", duration)
	}
	return 0
}

// checkBuilder holds state for watch-mode check rebuilds.
// Each rebuild runs a full check (no emit) — fast enough without incremental.
type checkBuilder struct {
	mu           sync.Mutex
	configPath   string
	tsconfigPath string
	noCheck      bool
}

func (b *checkBuilder) Check() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return runCheckOnce(b.configPath, b.tsconfigPath, b.noCheck)
}

// runCheckWatch runs the check command in watch mode.
func runCheckWatch(configPath, tsconfigPath string, noCheck bool) int {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: could not get working directory: %v\n", err)
		return 1
	}
	pretty := compiler.IsPrettyOutput()

	// Load config to determine source root for watching
	cfgResult, _ := loadOrDiscoverConfig(configPath, cwd)
	srcDir := "src"
	if cfgResult != nil && cfgResult.Config != nil && cfgResult.Config.SourceRoot != "" {
		srcDir = cfgResult.Config.SourceRoot
	}
	if !filepath.IsAbs(srcDir) {
		srcDir = filepath.Join(cwd, srcDir)
	}

	builder := &checkBuilder{
		configPath:   configPath,
		tsconfigPath: tsconfigPath,
		noCheck:      noCheck,
	}

	// Initial check
	printStatus(os.Stderr, pretty, "◆", "running initial check...")
	builder.Check()
	printStatus(os.Stderr, pretty, "◆", "watching for changes...")

	// Set up file watcher
	rebuild := func(events []watcher.Event) {
		fmt.Fprintf(os.Stderr, "\n─────────────────────────────────\n")
		fmt.Fprintf(os.Stderr, "detected %d change(s), rechecking...\n\n", len(events))
		builder.Check()
		printStatus(os.Stderr, pretty, "◆", "watching for changes...")
	}

	w := watcher.New(
		[]string{srcDir},
		[]string{".ts", ".tsx", ".mts", ".cts"},
		100*time.Millisecond,
		rebuild,
	)

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "\nshutting down...")
		w.Stop()
	}()

	w.Watch()
	return 0
}
