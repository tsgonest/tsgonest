package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/microsoft/typescript-go/shim/ast"
	shimchecker "github.com/microsoft/typescript-go/shim/checker"
	shimcompiler "github.com/microsoft/typescript-go/shim/compiler"
	"github.com/microsoft/typescript-go/shim/core"
	shimincremental "github.com/microsoft/typescript-go/shim/execute/incremental"
	shimtsoptions "github.com/microsoft/typescript-go/shim/tsoptions"
	"github.com/tsgonest/tsgonest/internal/analyzer"
	"github.com/tsgonest/tsgonest/internal/buildcache"
	"github.com/tsgonest/tsgonest/internal/codegen"
	"github.com/tsgonest/tsgonest/internal/compiler"
	"github.com/tsgonest/tsgonest/internal/config"
	"github.com/tsgonest/tsgonest/internal/metadata"
	"github.com/tsgonest/tsgonest/internal/openapi"
	"github.com/tsgonest/tsgonest/internal/pathalias"
	"github.com/tsgonest/tsgonest/internal/rewrite"
	"github.com/tsgonest/tsgonest/internal/sdkgen"
)

// buildFlags holds the parsed flags from the build command line.
// Tsgonest-specific flags are separated from tsgo compiler flags.
type buildFlags struct {
	ConfigPath   string
	TsconfigPath string
	DumpMetadata bool
	Clean        bool
	Assets       string
	NoCheck      bool
	TsgoArgs     []string // flags to forward to tsgo's ParseCommandLine
}

// parseBuildArgs separates tsgonest-specific flags from tsgo compiler flags.
// Tsgonest flags (--config, --project, --clean, etc.) are consumed and stored
// in the returned buildFlags. Everything else is collected in TsgoArgs for
// forwarding to tsgo's ParseCommandLine.
func parseBuildArgs(args []string) buildFlags {
	f := buildFlags{
		TsconfigPath: "tsconfig.json",
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--config":
			if i+1 < len(args) {
				i++
				f.ConfigPath = args[i]
			}
		case "--project", "-p":
			if i+1 < len(args) {
				i++
				f.TsconfigPath = args[i]
			}
		case "--dump-metadata":
			f.DumpMetadata = true
		case "--clean":
			f.Clean = true
		case "--assets":
			if i+1 < len(args) {
				i++
				f.Assets = args[i]
			}
		case "--no-check":
			f.NoCheck = true
		default:
			// Not a tsgonest flag — pass through to tsgo
			f.TsgoArgs = append(f.TsgoArgs, arg)
		}
	}

	return f
}

// parseTsgoFlags parses tsgo compiler flags via tsgo's own ParseCommandLine.
// Returns the parsed CompilerOptions overrides, or errors if any flag is invalid.
func parseTsgoFlags(tsgoArgs []string) (*core.CompilerOptions, []string) {
	if len(tsgoArgs) == 0 {
		return nil, nil
	}

	cliFS := compiler.CreateDefaultFS()
	cliHost := compiler.CreateDefaultHost("", cliFS)
	parsedCLI := shimtsoptions.ParseCommandLine(tsgoArgs, cliHost)
	if parsedCLI != nil && len(parsedCLI.Errors) > 0 {
		var errs []string
		for _, d := range parsedCLI.Errors {
			errs = append(errs, d.String())
		}
		return nil, errs
	}
	if parsedCLI != nil {
		return parsedCLI.CompilerOptions(), nil
	}
	return nil, nil
}

// runBuild executes the full build pipeline:
// diagnostics -> compile -> path alias resolution -> companions -> OpenAPI -> assets.
//
// Exit codes (matching tsgo):
//
//	0 = success, no errors
//	1 = diagnostics present, outputs generated
//	2 = diagnostics present, outputs skipped (e.g. noEmitOnError)
func runBuild(args []string) int {
	return runBuildWithIncr(args, nil, nil)
}

// runBuildWithIncr is the extended build function for dev mode. It accepts an optional
// old incremental program for in-memory reuse, bypassing .tsbuildinfo disk reads.
// If outIncrProgram is non-nil, the created incremental program is stored there for
// reuse on the next rebuild cycle.
func runBuildWithIncr(args []string, oldIncrProgram *shimincremental.Program, outIncrProgram **shimincremental.Program) int {
	return runBuildWithIncrAndProgram(args, oldIncrProgram, outIncrProgram, nil)
}

// sweepBuildCacheTmp removes any orphaned .tsgonest-cache.tmp files left by a
// previous build that was interrupted before the atomic rename completed.
// This is cheap and idempotent; errors are silently ignored.
func sweepBuildCacheTmp(outDir string) {
	if outDir == "" {
		return
	}
	// Match both ".tsgonest-cache.tmp" and numbered variants like ".tsgonest-cache.tmp.123".
	for _, pattern := range []string{
		filepath.Join(outDir, ".tsgonest-cache.tmp"),
		filepath.Join(outDir, ".tsgonest-cache.tmp.*"),
	} {
		matches, _ := filepath.Glob(pattern)
		for _, m := range matches {
			_ = os.Remove(m)
		}
	}
}

// runBuildWithIncrAndProgram is the core build pipeline. When prebuiltProgram is
// non-nil (from UpdateProgram fast path), it skips CreateProgramFromConfig —
// avoiding re-reading and re-parsing all source files.
func runBuildWithIncrAndProgram(args []string, oldIncrProgram *shimincremental.Program, outIncrProgram **shimincremental.Program, prebuiltProgram *shimcompiler.Program) int {
	flags := parseBuildArgs(args)

	configPath := flags.ConfigPath
	tsconfigPath := flags.TsconfigPath
	dumpMetadata := flags.DumpMetadata
	clean := flags.Clean
	assets := flags.Assets
	noCheck := flags.NoCheck

	// Parse tsgo flags via tsgo's own command-line parser.
	// This handles --strict, --noEmit, --target, --module, etc.
	// Any flag not recognized by tsgonest above is treated as a tsgo compiler flag.
	var cliOverrides *core.CompilerOptions
	if len(flags.TsgoArgs) > 0 {
		overrides, errs := parseTsgoFlags(flags.TsgoArgs)
		if len(errs) > 0 {
			for _, e := range errs {
				fmt.Fprintf(os.Stderr, "error: %s\n", e)
			}
			return 1
		}
		cliOverrides = overrides
	}

	buildStart := time.Now()
	timing := &TimingReport{}
	pretty := compiler.IsPrettyOutput()

	// Resolve working directory
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: could not get working directory: %v\n", err)
		return 1
	}

	// Load config if specified, or auto-discover in CWD.
	cfgResult, err := loadOrDiscoverConfig(configPath, cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	cfg := cfgResult.Config
	resolvedConfigPath := cfgResult.Path
	configDir := cfgResult.Dir
	if resolvedConfigPath != "" {
		printStatus(os.Stderr, pretty, "◆", "loaded config from %s", filepath.Base(resolvedConfigPath))
	}

	// Honor deleteOutDir from config (same as nest-cli.json convention).
	// The --clean CLI flag always wins, but deleteOutDir in config enables it too.
	if !clean && cfg != nil && cfg.DeleteOutDir {
		clean = true
	}

	// Step 1: Parse tsconfig using tsgo's native JSONC parser (handles comments, trailing commas, extends).
	tsconfigStart := time.Now()
	tsFS := compiler.CreateDefaultFS()
	host := compiler.CreateDefaultHost(cwd, tsFS)

	printStatus(os.Stderr, pretty, "◆", "compiling with tsconfig: %s", tsconfigPath)

	parsedConfig, diags, err := compiler.ParseTSConfig(tsFS, cwd, tsconfigPath, host, cliOverrides)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if len(diags) > 0 {
		fmt.Fprint(os.Stderr, compiler.FormatDiagnostics(diags))
		return 1
	}

	opts := parsedConfig.CompilerOptions()
	modFmt := rewrite.DetectModuleFormat(moduleFormatFromOpts(opts))

	// Resolve tsconfig path for cache file derivation
	resolvedTsconfigPath := tsconfigPath
	if !filepath.IsAbs(resolvedTsconfigPath) {
		resolvedTsconfigPath = filepath.Join(cwd, resolvedTsconfigPath)
	}
	postCachePath := buildcache.CachePath(opts.OutDir, resolvedTsconfigPath)

	// Sweep orphaned .tmp files before any build logic touches the cache.
	sweepBuildCacheTmp(opts.OutDir)

	// Clean output directory if requested (using parsed OutDir, no re-parsing needed)
	if clean && opts.OutDir != "" {
		tsbuildInfoPath := tsbuildInfoPathFromTsconfig(resolvedTsconfigPath)
		// Full clean: remove output directory and .tsbuildinfo to force a complete rebuild.
		// Without deleting .tsbuildinfo, the incremental compiler thinks files are up-to-date
		// and skips emit — which means controller rewriting (in the WriteFile callback) never runs.
		if cleanErr := cleanDir(opts.OutDir); cleanErr != nil {
			fmt.Fprintf(os.Stderr, "warning: clean: %v\n", cleanErr)
		}
		// Delete .tsbuildinfo so incremental compilation starts fresh
		os.Remove(tsbuildInfoPath)
		// Delete the post-processing cache — ensures full companion/OpenAPI regeneration
		buildcache.Delete(postCachePath)
	}
	timing.TSConfig = time.Since(tsconfigStart)

	// Step 2: Create program with the (possibly modified) config.
	// When prebuiltProgram is provided (from UpdateProgram fast path),
	// skip the expensive CreateProgramFromConfig that re-reads all files.
	programStart := time.Now()
	var program *shimcompiler.Program
	if prebuiltProgram != nil {
		program = prebuiltProgram
	} else {
		var programDiags []compiler.Diagnostic
		program, programDiags, err = compiler.CreateProgramFromConfig(false, parsedConfig, host)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		if len(programDiags) > 0 {
			fmt.Fprint(os.Stderr, compiler.FormatDiagnostics(programDiags))
			return 1
		}
	}
	timing.Program = time.Since(programStart)

	// Handle --dump-metadata: skip emit, just analyze types
	if dumpMetadata {
		return runDumpMetadata(program, opts)
	}

	// Step 3: Gather diagnostics (forces type checking).
	// If tsconfig has "incremental: true" or "composite: true", use the incremental
	// pipeline — only checks/emits changed files, persists state to .tsbuildinfo.
	reportDiag := compiler.CreateDiagnosticReporter(os.Stderr, cwd, pretty)

	isIncremental := opts.IsIncremental()

	var allDiagnostics []*ast.Diagnostic
	var incrProgram *shimincremental.Program

	if isIncremental {
		// Incremental mode: wrap program with incremental state.
		// ReadBuildInfoProgram reads prior state from .tsbuildinfo (if it exists).
		incrProgram = compiler.CreateIncrementalProgram(program, oldIncrProgram, host, parsedConfig)
		// Store the incremental program for dev-mode reuse across rebuilds
		if outIncrProgram != nil {
			*outIncrProgram = incrProgram
		}
		fmt.Fprintln(os.Stderr, "incremental build enabled")

		diagStart := time.Now()
		allDiagnostics = compiler.GatherIncrementalDiagnostics(incrProgram, noCheck)
		timing.Diagnostics = time.Since(diagStart)
	} else {
		diagStart := time.Now()
		allDiagnostics = compiler.GatherDiagnostics(program, noCheck)
		timing.Diagnostics = time.Since(diagStart)
	}

	// Check for errors before proceeding to analysis
	hasPreEmitErrors := compiler.CountErrors(allDiagnostics) > 0

	// ── Pre-emit analysis ────────────────────────────────────────────────
	// Move checker, companion generation, controller analysis, and marker
	// extraction BEFORE emit so the WriteFile callback has all data it needs.
	// This works because the checker is available after GatherDiagnostics.

	needCompanions := cfg != nil && (cfg.Transforms.Validation || cfg.Transforms.Serialization)
	needControllers := cfg != nil && len(cfg.Controllers.Include) > 0

	// Track files with syntax errors — skip companion generation for them
	syntaxErrorFiles := compiler.FilesWithSyntaxErrors(
		compiler.GetSyntacticDiagnostics(program),
	)

	// Build path alias resolver (used in WriteFile callback instead of post-emit)
	var pathResolver *pathalias.PathResolver
	if opts.Paths != nil && opts.Paths.Size() > 0 {
		pathsMap := make(map[string][]string)
		for k, v := range opts.Paths.Entries() {
			pathsMap[k] = v
		}
		pathResolver = pathalias.NewPathResolver(pathalias.Config{
			PathsBaseDir: opts.GetPathsBasePath(cwd),
			OutDir:       opts.OutDir,
			RootDir:      opts.RootDir,
			Paths:        pathsMap,
		})
	}

	var sharedChecker *shimchecker.Checker
	var sharedWalker *analyzer.TypeWalker
	var checkerRelease func()
	var allCompanions []codegen.CompanionFile
	var controllers []analyzer.ControllerInfo
	var controllerRegistry *metadata.TypeRegistry
	var controllerWarnings []analyzer.Warning
	var rewriteCtx *rewrite.RewriteContext

	// Only do pre-emit analysis if no errors (type checker data may be unreliable)
	if !hasPreEmitErrors && (needCompanions || needControllers) {
		checkerStart := time.Now()
		sharedChecker, checkerRelease = program.GetTypeChecker(context.Background())
		if sharedChecker == nil {
			fmt.Fprintln(os.Stderr, "error: could not get type checker")
			return 1
		}
		defer checkerRelease()
		sharedWalker = analyzer.NewTypeWalker(sharedChecker)
		if opts.ExactOptionalPropertyTypes == core.TSTrue {
			sharedWalker.SetExactOptionalPropertyTypes(true)
		}
		timing.Checker = time.Since(checkerStart)

		// Build source→output map (needed before emit for companion path computation)
		sourceToOutput := buildSourceToOutputMapFromConfig(program, opts.RootDir, opts.OutDir)

		// ── Step 1: Analyze controllers to discover needed types ─────────
		// No blanket pre-registration pass — the walker discovers and registers
		// sub-field type aliases on-the-fly via Type_alias recovery (depth > 1).
		// This keeps the walk scope controller-driven: only types reachable from
		// the API surface are walked, preventing transitive crawls into
		// node_modules (e.g., Zod, AI SDK, React types).
		controllerStart := time.Now()
		if needControllers {
			ca := analyzer.NewControllerAnalyzerWithWalker(program, sharedChecker, sharedWalker)
			controllers = ca.AnalyzeProgram(cfg.Controllers.Include, cfg.Controllers.Exclude)
			controllerRegistry = ca.Registry()
			controllerWarnings = ca.Warnings()
		}
		timing.Controllers = time.Since(controllerStart)

		// ── Step 2: Extract marker calls to discover explicitly used types ─
		markerCalls := make(map[string][]rewrite.MarkerCall)
		for _, sf := range program.GetSourceFiles() {
			if sf.IsDeclarationFile {
				continue
			}
			calls := rewrite.ExtractMarkerCalls(sf, sharedChecker)
			if len(calls) > 0 {
				markerCalls[sf.FileName()] = calls
			}
		}

		// ── Step 3: Collect the set of type names that actually need companions ─
		// Only types referenced by controllers, marker calls, or transforms.include get companions.
		var neededTypes map[string]bool
		if len(cfg.Transforms.Include) > 0 {
			// When transforms.include is set, generate for ALL matching types (nil = no filter)
			neededTypes = nil
		} else {
			neededTypes = collectNeededTypes(controllers, markerCalls, cfg.Transforms.Exclude)
		}

		// Collect query/param DTO type names that need coercion
		coercionTypes := collectCoercionTypes(controllers)

		// Collect inline body types (synthesized names for anonymous object type params)
		inlineTypes := collectInlineBodyTypes(controllers)

		// ── Step 4: Generate companions only for needed types ────────────
		companionStart := time.Now()
		if needCompanions && (neededTypes == nil || len(neededTypes) > 0) {
			companions, typesByFile, compErr := generateCompanionsInMemory(program, cfg, sourceToOutput, sharedChecker, sharedWalker, syntaxErrorFiles, modFmt, neededTypes, coercionTypes, inlineTypes)
			if compErr != nil {
				fmt.Fprintf(os.Stderr, "error generating companions: %v\n", compErr)
				return 1
			}
			allCompanions = companions

			// Build companion map for rewriting
			companionMap := rewrite.BuildCompanionMap(sourceToOutput, typesByFile)

			// Build RewriteContext
			rewriteCtx = &rewrite.RewriteContext{
				CompanionMap:   companionMap,
				MarkerCalls:    markerCalls,
				PathResolver:   pathResolver,
				ModuleFormat:   rewrite.DetectModuleFormat(moduleFormatFromOpts(opts)),
				SourceToOutput: sourceToOutput,
				OutputToSource: rewrite.BuildOutputToSourceMap(sourceToOutput),
			}
		}
		timing.Companions = time.Since(companionStart)

		// Attach controller data to rewrite context
		if needControllers {
			if rewriteCtx != nil {
				rewriteCtx.Controllers = controllers
				rewriteCtx.ControllerSourceFiles = rewrite.BuildControllerSourceFiles(controllers)
			} else {
				// Controllers without companions — still need rewrite context for controller injection
				rewriteCtx = &rewrite.RewriteContext{
					CompanionMap:          make(map[string]string),
					MarkerCalls:           markerCalls,
					Controllers:           controllers,
					ControllerSourceFiles: rewrite.BuildControllerSourceFiles(controllers),
					PathResolver:          pathResolver,
					ModuleFormat:          rewrite.DetectModuleFormat(moduleFormatFromOpts(opts)),
					SourceToOutput:        sourceToOutput,
					OutputToSource:        rewrite.BuildOutputToSourceMap(sourceToOutput),
				}
			}
		}
	}

	// If we only have path aliases but no companions/controllers, still set up
	// a minimal rewrite context for alias resolution in the WriteFile callback.
	if rewriteCtx == nil && pathResolver != nil && pathResolver.HasAliases() {
		rewriteCtx = &rewrite.RewriteContext{
			CompanionMap:          make(map[string]string),
			MarkerCalls:           make(map[string][]rewrite.MarkerCall),
			PathResolver:          pathResolver,
			ModuleFormat:          "esm",
			SourceToOutput:        make(map[string]string),
			OutputToSource:        make(map[string]string),
			ControllerSourceFiles: make(map[string]bool),
		}
	}

	// ── Step 4: Emit with WriteFile callback ─────────────────────────────
	// The WriteFile callback applies path alias resolution, marker rewriting,
	// and controller body validation injection during emit — zero extra I/O.
	var emitResult *compiler.EmitResult

	// Build the WriteFile callback
	var writeFile shimcompiler.WriteFile
	if rewriteCtx != nil {
		writeFile = rewriteCtx.MakeWriteFile()
	}

	if isIncremental {
		emitStart := time.Now()
		emitResult = compiler.EmitIncrementalProgram(incrProgram, writeFile)
		timing.Emit = time.Since(emitStart)

		// Guard against stale .tsbuildinfo: if incremental emit reports zero files
		// but expected output files are missing on disk (e.g. dist/ was deleted while
		// .tsbuildinfo survived), delete .tsbuildinfo and re-create the incremental
		// program from scratch so the fresh emit also writes a valid .tsbuildinfo.
		if len(emitResult.EmittedFiles) == 0 && !emitResult.EmitSkipped {
			if missing := checkExpectedOutputsMissing(program, opts.RootDir, opts.OutDir); len(missing) > 0 {
				tsbuildInfoPath := tsbuildInfoPathFromTsconfig(resolvedTsconfigPath)
				if err := os.Remove(tsbuildInfoPath); err != nil && !os.IsNotExist(err) {
					fmt.Fprintf(os.Stderr, "warning: could not remove stale .tsbuildinfo: %v\n", err)
				}
				printStatus(os.Stderr, pretty, "◆", "stale .tsbuildinfo: %d output file(s) missing, forcing full emit", len(missing))
				// Create a completely fresh program and host. The first emit
				// mutates the program's internal emit state, so reusing it
				// would still produce 0 emitted files. A fresh host also
				// avoids any cached .tsbuildinfo reads.
				freshFS := compiler.CreateDefaultFS()
				freshHost := compiler.CreateDefaultHost(cwd, freshFS)
				freshProgram, _, freshErr := compiler.CreateProgramFromConfig(false, parsedConfig, freshHost)
				if freshErr == nil && freshProgram != nil {
					incrProgram = compiler.CreateIncrementalProgram(freshProgram, nil, freshHost, parsedConfig)
				} else {
					// Fallback: reuse existing program (best effort)
					incrProgram = compiler.CreateIncrementalProgram(program, nil, host, parsedConfig)
				}
				if outIncrProgram != nil {
					*outIncrProgram = incrProgram
				}
				emitStart = time.Now()
				emitResult = compiler.EmitIncrementalProgram(incrProgram, writeFile)
				timing.Emit += time.Since(emitStart)
			}
		}
	} else {
		emitStart := time.Now()
		emitResult = compiler.EmitProgram(program, writeFile)
		timing.Emit = time.Since(emitStart)
	}

	// Append emit diagnostics (declaration transform errors, write errors)
	allDiagnostics = append(allDiagnostics, emitResult.Diagnostics...)
	allDiagnostics = shimcompiler.SortAndDeduplicateDiagnostics(allDiagnostics)

	// Report all diagnostics
	for _, d := range allDiagnostics {
		reportDiag(d)
	}

	// Drain rewrite diagnostics (controller class/method not located in emit).
	// Errors are load-bearing — silent miss = unvalidated input hits handlers
	// (issue #114). Warnings are recoverable degradations on individual methods.
	var rewriteErrors []rewrite.RewriteDiagnostic
	var rewriteWarnings []string
	if rewriteCtx != nil {
		for _, d := range rewriteCtx.Diagnostics() {
			if d.IsError() {
				rewriteErrors = append(rewriteErrors, d)
			} else {
				rewriteWarnings = append(rewriteWarnings, d.Format())
			}
		}
	}

	// Error summary (pretty mode only)
	if pretty {
		compiler.WriteErrorSummary(os.Stderr, allDiagnostics, cwd)
	}

	// Determine exit status (matching tsgo):
	// - EmitSkipped + errors → exit 2
	// - Errors present → exit 1
	// - No errors → continue to post-emit steps
	hasErrors := compiler.CountErrors(allDiagnostics) > 0
	if emitResult.EmitSkipped && hasErrors {
		// noEmitOnError triggered — no files written
		printStatus(os.Stderr, pretty, "✗", "no files emitted (noEmitOnError)")
		timing.Total = time.Since(buildStart)
		timing.Print()
		return 2
	}

	emittedFiles := emitResult.EmittedFiles
	if len(emittedFiles) > 0 {
		printStatus(os.Stderr, pretty, "✓", "emitted %d file(s)", len(emittedFiles))
	} else if !emitResult.EmitSkipped {
		printStatus(os.Stderr, pretty, "·", "no files emitted")
	}

	// ── Early exit on diagnostic errors ──────────────────────────────────
	// Path aliases were already resolved in the WriteFile callback.
	if hasErrors {
		timing.Total = time.Since(buildStart)
		timing.Print()
		return 1
	}

	// Rewrite errors fail the whole build. These mean the analyzer recognized
	// a controller but the rewriter couldn't locate its class in emitted JS,
	// so validation/serialization injection was skipped — not safe to ship.
	if len(rewriteErrors) > 0 {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "✗ %d controller rewrite error(s) — these are load-bearing injections that were skipped:\n", len(rewriteErrors))
		for _, e := range rewriteErrors {
			fmt.Fprintf(os.Stderr, "  %s\n", e.Format())
		}
		fmt.Fprintln(os.Stderr)
		timing.Total = time.Since(buildStart)
		timing.Print()
		return 1
	}

	// ── Generate a single shared helpers file at the output root ────────
	if len(allCompanions) > 0 {
		helpersRoot := opts.OutDir
		if helpersRoot == "" {
			helpersRoot = determineOutputDir(allCompanions, emittedFiles, cwd)
		}
		// Ensure helpersRoot is absolute for correct relative path computation
		if !filepath.IsAbs(helpersRoot) {
			helpersRoot = filepath.Join(cwd, helpersRoot)
		}
		allCompanions = append(allCompanions, codegen.GenerateHelpersFile(helpersRoot, modFmt)...)
		helpersDir := helpersRoot
		// Fix relative import paths in each companion: "./_tsgonest_helpers.js" → correct relative path
		for i := range allCompanions {
			comp := &allCompanions[i]
			if strings.HasSuffix(comp.Path, "_tsgonest_helpers.js") || strings.HasSuffix(comp.Path, "_tsgonest_helpers.d.ts") {
				continue
			}
			compDir := filepath.Dir(comp.Path)
			if compDir == helpersDir {
				continue // same directory, "./" is already correct
			}
			rel, err := filepath.Rel(compDir, helpersDir)
			if err != nil {
				continue
			}
			relImport := filepath.ToSlash(rel) + "/_tsgonest_helpers.js"
			if !strings.HasPrefix(relImport, ".") {
				relImport = "./" + relImport
			}
			comp.Content = strings.Replace(comp.Content, "\"./_tsgonest_helpers.js\"", "\""+relImport+"\"", 1)
		}
	}

	// ── Write companion files to disk (Option B: batch after emit) ──────
	companionWriteStart := time.Now()
	for _, comp := range allCompanions {
		dir := filepath.Dir(comp.Path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "error creating dir %s: %v\n", dir, err)
			return 1
		}
		if err := os.WriteFile(comp.Path, []byte(comp.Content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing %s: %v\n", comp.Path, err)
			return 1
		}
	}
	if dur := time.Since(companionWriteStart); dur > time.Millisecond {
		// Only report if it takes noticeable time
		timing.Companions += dur
	}

	// ── Post-processing cache check ──────────────────────────────────────
	var configHash string
	if resolvedConfigPath != "" {
		configHash = buildcache.HashFile(resolvedConfigPath)
	}

	noFilesEmitted := len(emittedFiles) == 0 && !emitResult.EmitSkipped
	if noFilesEmitted && !clean {
		existingCache := buildcache.Load(postCachePath)
		if existingCache != nil && existingCache.IsValid(configHash) {
			fmt.Fprintln(os.Stderr, "no changes detected, outputs up to date")
			timing.Total = time.Since(buildStart)
			timing.Print()
			return 0
		}
	}

	// Print deferred status messages (only when we're past the cache check)
	if len(allCompanions) > 0 {
		printStatus(os.Stderr, pretty, "✓", "generated %d companion file(s)", len(allCompanions))
	}
	if len(controllers) > 0 {
		totalRoutes := 0
		for _, ctrl := range controllers {
			totalRoutes += len(ctrl.Routes)
		}
		printStatus(os.Stderr, pretty, "✓", "found %d controller(s) with %d route(s)", len(controllers), totalRoutes)
	}

	// Collect all warning messages for deferred printing after status lines.
	var allWarnings []string
	for _, w := range controllerWarnings {
		allWarnings = append(allWarnings, w.Message)
	}
	if sharedWalker != nil {
		allWarnings = append(allWarnings, sharedWalker.Warnings()...)
	}
	allWarnings = append(allWarnings, rewriteWarnings...)

	// Generate OpenAPI documents (using pre-analyzed controllers)
	openapiStart := time.Now()
	if cfg != nil && len(cfg.OpenAPIOutputs) > 0 && len(controllers) > 0 {
		for i := range cfg.OpenAPIOutputs {
			outputCfg := &cfg.OpenAPIOutputs[i]
			if outputCfg.Output == "" {
				continue
			}
			// Post-filter controllers for this output
			filtered := controllers
			if outputCfg.Controllers != nil || len(outputCfg.IncludeTags) > 0 || len(outputCfg.ExcludeTags) > 0 {
				filterOpts := openapi.FilterOptions{
					IncludeTags: outputCfg.IncludeTags,
					ExcludeTags: outputCfg.ExcludeTags,
				}
				if outputCfg.Controllers != nil {
					filterOpts.ControllerInclude = outputCfg.Controllers.Include
					filterOpts.ControllerExclude = outputCfg.Controllers.Exclude
				}
				filtered = openapi.FilterControllers(controllers, filterOpts)
			}
			if len(filtered) == 0 {
				continue
			}
			if err := generateOpenAPIFromOutput(filtered, controllerRegistry, outputCfg, cfg, configDir); err != nil {
				label := outputCfg.Output
				if outputCfg.Name != "" {
					label = outputCfg.Name + " (" + outputCfg.Output + ")"
				}
				fmt.Fprintf(os.Stderr, "error generating OpenAPI %s: %v\n", label, err)
				return 1
			}
		}
	}
	timing.OpenAPI = time.Since(openapiStart)

	// Generate TypeScript SDK if configured (runs in background goroutine).
	// Supports both top-level sdk.output (legacy) and per-output sdk.output.
	var sdkWg sync.WaitGroup
	var sdkMu sync.Mutex
	var sdkErr error

	// Build shared SDK options from config
	var sdkOpts *sdkgen.GenerateOptions
	if cfg != nil {
		sdkOpts = &sdkgen.GenerateOptions{
			GlobalPrefix: cfg.NestJS.GlobalPrefix,
		}
		if cfg.NestJS.Versioning != nil && cfg.NestJS.Versioning.Prefix != "" {
			sdkOpts.VersionPrefix = cfg.NestJS.Versioning.Prefix
		}
	}

	// Per-output SDK generation
	if cfg != nil {
		for _, oc := range cfg.OpenAPIOutputs {
			if oc.SDK == nil || oc.SDK.Output == "" || oc.Output == "" {
				continue
			}
			sdkInput := oc.Output
			if !filepath.IsAbs(sdkInput) {
				sdkInput = filepath.Join(configDir, sdkInput)
			}
			sdkOutput := oc.SDK.Output
			if !filepath.IsAbs(sdkOutput) {
				sdkOutput = filepath.Join(configDir, sdkOutput)
			}
			label := oc.Output
			if oc.Name != "" {
				label = oc.Name
			}
			sdkWg.Add(1)
			go func(input, output, label string) {
				defer sdkWg.Done()
				sdkHashPath := filepath.Join(output, ".sdk-hash")
				inputHash := hashFileContent(input)
				if existingHash, err := os.ReadFile(sdkHashPath); err == nil && string(existingHash) == inputHash {
					printStatus(os.Stderr, pretty, "·", "SDK (%s) up to date, skipping", label)
					return
				}
				sdkWarnings, err := sdkgen.Generate(input, output, sdkOpts)
				if err != nil {
					sdkMu.Lock()
					sdkErr = err
					sdkMu.Unlock()
					return
				}
				os.WriteFile(sdkHashPath, []byte(inputHash), 0o644)
				printStatus(os.Stderr, pretty, "✓", "generated SDK: %s → %s", label, output)
				// Surface SDK warnings alongside other build warnings
				if len(sdkWarnings) > 0 {
					sdkMu.Lock()
					allWarnings = append(allWarnings, sdkWarnings...)
					sdkMu.Unlock()
				}
			}(sdkInput, sdkOutput, label)
		}
	}

	// Copy static assets if configured
	if assets != "" {
		outDir := determineOutputDir(allCompanions, emittedFiles, cwd)
		count, assetErr := copyAssets(cwd, outDir, assets)
		if assetErr != nil {
			fmt.Fprintf(os.Stderr, "warning: copying assets: %v\n", assetErr)
		} else if count > 0 {
			printStatus(os.Stderr, pretty, "✓", "copied %d asset(s)", count)
		}
	}

	// Wait for background SDK generation to complete
	sdkWg.Wait()
	if sdkErr != nil {
		fmt.Fprintf(os.Stderr, "error generating SDK: %v\n", sdkErr)
		return 1
	}

	// ── Save post-processing cache ─────────────────────────────────────
	// Record what we just built so the next incremental warm build can skip
	// post-processing when nothing changed.
	var cacheOutputs []string
	if cfg != nil {
		for _, oc := range cfg.OpenAPIOutputs {
			if oc.Output == "" {
				continue
			}
			p := oc.Output
			if !filepath.IsAbs(p) {
				p = filepath.Join(configDir, p)
			}
			cacheOutputs = append(cacheOutputs, p)
		}
	}
	postCache := buildcache.New(configHash, cacheOutputs)
	if saveErr := buildcache.Save(postCachePath, postCache); saveErr != nil {
		fmt.Fprintf(os.Stderr, "warning: saving post-processing cache: %v\n", saveErr)
	}

	// Print warnings after all status messages, before timing.
	if len(allWarnings) > 0 {
		yellow, white, blue, dim, bold, reset := "", "", "", "", "", ""
		if pretty {
			yellow = "\u001b[93m"
			white = "\u001b[97m"
			blue = "\u001b[94m"
			dim = "\u001b[2m"
			bold = "\u001b[1m"
			reset = "\u001b[0m"
		}

		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "%s%s▲ %d warning(s)%s\n", bold, yellow, len(allWarnings), reset)
		fmt.Fprintln(os.Stderr)
		for _, msg := range allWarnings {
			short := strings.ReplaceAll(msg, cwd+"/", "")
			var loc, desc string
			if l, d, ok := strings.Cut(short, " — "); ok {
				loc, desc = l, d
			} else if idx := strings.LastIndex(short, ") "); idx != -1 {
				loc, desc = short[:idx+1], short[idx+2:]
			} else {
				loc = short
			}
			coloredLoc := formatLocation(loc, white, bold, blue, reset)
			if desc != "" {
				fmt.Fprintf(os.Stderr, "  %s●%s %s\n    %s%s%s\n\n", yellow, reset, coloredLoc, dim, desc, reset)
			} else {
				fmt.Fprintf(os.Stderr, "  %s●%s %s\n\n", yellow, reset, coloredLoc)
			}
		}
	}

	timing.Total = time.Since(buildStart)
	timing.Print()

	return 0
}

// tsbuildInfoPathFromTsconfig derives the .tsbuildinfo path from a resolved tsconfig path.
// e.g., "/project/tsconfig.json" → "/project/tsconfig.tsbuildinfo"
func tsbuildInfoPathFromTsconfig(resolvedTsconfigPath string) string {
	return strings.TrimSuffix(resolvedTsconfigPath, ".json") + ".tsbuildinfo"
}

// validateOutDir checks that outDir is safe to remove. This is the LAST line of
// defense before os.RemoveAll — do NOT weaken these checks.
func validateOutDir(outDir string) error {
	if outDir == "" {
		return fmt.Errorf("refusing to clean empty outDir")
	}

	clean := filepath.Clean(outDir)

	// Reject bare navigation tokens before making the path absolute —
	// "." and ".." are almost certainly user mistakes, and their resolved
	// value changes with the working directory at runtime.
	if clean == "." || clean == ".." {
		return fmt.Errorf("refusing to clean dangerous path %q", outDir)
	}

	abs, err := filepath.Abs(clean)
	if err != nil {
		return fmt.Errorf("resolving outDir: %w", err)
	}

	// Strip volume prefix (e.g. "C:" on Windows, "" on Unix), then verify
	// that something meaningful remains below the root or volume root.
	vol := filepath.VolumeName(abs)
	rest := strings.TrimPrefix(abs, vol)
	rest = strings.Trim(rest, string(filepath.Separator))
	if rest == "" || rest == "." || rest == ".." {
		return fmt.Errorf("refusing to clean dangerous path %q", outDir)
	}

	// Refuse to wipe the user's home directory.
	if home, err := os.UserHomeDir(); err == nil {
		if absHome, err := filepath.Abs(home); err == nil && abs == absHome {
			return fmt.Errorf("refusing to clean home directory %q", outDir)
		}
	}

	// Refuse to wipe the current working directory.
	if cwd, err := os.Getwd(); err == nil {
		if absCwd, err := filepath.Abs(cwd); err == nil && abs == absCwd {
			return fmt.Errorf("refusing to clean current working directory %q", outDir)
		}
	}

	return nil
}

// cleanDir removes a directory after safety checks.
func cleanDir(outDir string) error {
	if err := validateOutDir(outDir); err != nil {
		return err
	}

	if _, err := os.Stat(outDir); os.IsNotExist(err) {
		return nil
	}

	printStatus(os.Stderr, compiler.IsPrettyOutput(), "◆", "cleaning output directory: %s", outDir)
	return os.RemoveAll(outDir)
}

// smartCleanDir removes all files in outDir but preserves .tsbuildinfo,
// so incremental compilation can still reuse cached data from the previous build.
func smartCleanDir(outDir string, tsbuildInfoPath string) error {
	if err := validateOutDir(outDir); err != nil {
		return err
	}

	if _, err := os.Stat(outDir); os.IsNotExist(err) {
		return nil
	}

	printStatus(os.Stderr, compiler.IsPrettyOutput(), "◆", "cleaning output directory: %s (preserving .tsbuildinfo)", outDir)

	entries, err := os.ReadDir(outDir)
	if err != nil {
		return err
	}
	absTsbuildInfo, _ := filepath.Abs(tsbuildInfoPath)
	for _, entry := range entries {
		entryPath := filepath.Join(outDir, entry.Name())
		absEntry, _ := filepath.Abs(entryPath)
		if absEntry == absTsbuildInfo {
			continue // preserve .tsbuildinfo
		}
		if err := os.RemoveAll(entryPath); err != nil {
			return err
		}
	}
	return nil
}

// generateOpenAPIFromOutput generates an OpenAPI 3.2 document for a single output configuration.
func generateOpenAPIFromOutput(controllers []analyzer.ControllerInfo, registry *metadata.TypeRegistry, outputCfg *config.OpenAPIOutputConfig, cfg *config.Config, configDir string) error {
	// Each output gets its own generator (and SchemaGenerator) so only
	// schemas referenced by its filtered controllers end up in components/schemas.
	gen := openapi.NewGenerator(registry)

	var genOpts *openapi.GenerateOptions
	if cfg.NestJS.Versioning != nil || cfg.NestJS.GlobalPrefix != "" {
		genOpts = &openapi.GenerateOptions{
			GlobalPrefix: cfg.NestJS.GlobalPrefix,
		}
		if cfg.NestJS.Versioning != nil {
			genOpts.VersioningType = cfg.NestJS.Versioning.Type
			genOpts.DefaultVersion = cfg.NestJS.Versioning.DefaultVersion
			genOpts.VersionPrefix = cfg.NestJS.Versioning.Prefix
		}
	}
	doc := gen.GenerateWithOptions(controllers, genOpts)

	// Apply document-level config from the per-output config
	docCfg := openapi.DocumentConfig{
		Title:          outputCfg.Title,
		Description:    outputCfg.Description,
		Version:        outputCfg.Version,
		TermsOfService: outputCfg.TermsOfService,
		Security:       outputCfg.Security,
	}
	for _, t := range outputCfg.Tags {
		docCfg.Tags = append(docCfg.Tags, openapi.Tag{
			Name:        t.Name,
			Description: t.Description,
		})
	}
	if outputCfg.Contact != nil {
		docCfg.Contact = &openapi.Contact{
			Name:  outputCfg.Contact.Name,
			URL:   outputCfg.Contact.URL,
			Email: outputCfg.Contact.Email,
		}
	}
	if outputCfg.License != nil {
		docCfg.License = &openapi.License{
			Name: outputCfg.License.Name,
			URL:  outputCfg.License.URL,
		}
	}
	for _, s := range outputCfg.Servers {
		docCfg.Servers = append(docCfg.Servers, openapi.Server{
			URL:         s.URL,
			Description: s.Description,
		})
	}
	if len(outputCfg.SecuritySchemes) > 0 {
		docCfg.SecuritySchemes = make(map[string]*openapi.SecurityScheme)
		for name, s := range outputCfg.SecuritySchemes {
			docCfg.SecuritySchemes[name] = &openapi.SecurityScheme{
				Type:         s.Type,
				Scheme:       s.Scheme,
				BearerFormat: s.BearerFormat,
				In:           s.In,
				Name:         s.Name,
				Description:  s.Description,
			}
		}
	}
	doc.ApplyConfig(docCfg)

	// Serialize to JSON
	jsonBytes, err := doc.ToJSON()
	if err != nil {
		return fmt.Errorf("serializing OpenAPI document: %w", err)
	}

	// Resolve output path relative to config file directory
	outputPath := outputCfg.Output
	if !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(configDir, outputPath)
	}

	// Create output directory if needed
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating output directory %s: %w", dir, err)
	}

	// Write the file
	if err := os.WriteFile(outputPath, jsonBytes, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", outputPath, err)
	}

	label := outputCfg.Output
	if outputCfg.Name != "" {
		label = outputCfg.Name + " → " + outputCfg.Output
	}
	printStatus(os.Stderr, compiler.IsPrettyOutput(), "✓", "generated OpenAPI document: %s", label)
	return nil
}

// buildSourceToOutputMapFromConfig creates a mapping from source .ts file paths to their
// expected output paths, computed from tsconfig rootDir/outDir without needing emitted files.
func buildSourceToOutputMapFromConfig(program *shimcompiler.Program, rootDir, outDir string) map[string]string {
	result := make(map[string]string)

	for _, sf := range program.GetSourceFiles() {
		if sf.IsDeclarationFile {
			continue
		}
		srcName := sf.FileName()

		srcNoExt := srcName
		for _, ext := range []string{".ts", ".tsx", ".mts", ".cts"} {
			if strings.HasSuffix(srcNoExt, ext) {
				srcNoExt = srcNoExt[:len(srcNoExt)-len(ext)]
				break
			}
		}

		var outputPath string
		if rootDir != "" && outDir != "" {
			rel, err := filepath.Rel(rootDir, srcNoExt)
			if err == nil && !strings.HasPrefix(rel, "..") {
				outputPath = filepath.Join(outDir, rel)
			}
		}
		if outputPath == "" && outDir != "" {
			outputPath = filepath.Join(outDir, filepath.Base(srcNoExt))
		}
		if outputPath == "" {
			outputPath = srcNoExt
		}

		// Store with .ts extension (for companionPath to strip)
		result[srcName] = outputPath + ".ts"
	}

	return result
}

// moduleFormatFromOpts determines the module format from compiler options.
func moduleFormatFromOpts(opts *core.CompilerOptions) string {
	if opts.Module != 0 {
		// Module values: 1=CommonJS, 2=AMD, 3=UMD, 4=System, 5=ES2015, 6=ES2020, etc.
		// Values >= 5 are ESM variants
		if opts.Module == 1 {
			return "commonjs"
		}
	}
	return "esm"
}

// discoverJSFiles walks the output directory and returns all .js file paths.
// Used when incremental mode emitted no files (nothing changed in TS) but we
// still need to run post-processing (cache miss). The companion generator needs
// a list of JS files to build the source→output mapping.
func discoverJSFiles(outDir string) []string {
	var files []string
	filepath.Walk(outDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if !info.IsDir() && strings.HasSuffix(path, ".js") {
			files = append(files, path)
		}
		return nil
	})
	return files
}

// checkExpectedOutputsMissing checks whether the expected .js output files for
// the program's source files exist on disk. Returns the list of missing paths.
// Used to detect stale .tsbuildinfo when dist/ was deleted but .tsbuildinfo survived.
func checkExpectedOutputsMissing(program *shimcompiler.Program, rootDir, outDir string) []string {
	if outDir == "" {
		return nil
	}
	sourceToOutput := buildSourceToOutputMapFromConfig(program, rootDir, outDir)
	var missing []string
	for _, outputTS := range sourceToOutput {
		// buildSourceToOutputMapFromConfig stores paths with .ts extension;
		// the actual emitted file has .js extension.
		jsPath := strings.TrimSuffix(outputTS, ".ts") + ".js"
		if _, err := os.Stat(jsPath); os.IsNotExist(err) {
			missing = append(missing, jsPath)
		}
	}
	return missing
}

// determineOutputDir figures out the output directory from emitted files or companion paths.
func determineOutputDir(companions []codegen.CompanionFile, emittedFiles []string, cwd string) string {
	// First try: use the directory of the first emitted JS file
	for _, f := range emittedFiles {
		if strings.HasSuffix(f, ".js") {
			return filepath.Dir(f)
		}
	}

	// Fallback: use the directory of the first companion file
	if len(companions) > 0 {
		return filepath.Dir(companions[0].Path)
	}

	// Last resort: cwd/dist
	return filepath.Join(cwd, "dist")
}
