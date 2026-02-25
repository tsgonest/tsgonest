package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
		fmt.Fprintf(os.Stderr, "loaded config from %s\n", filepath.Base(resolvedConfigPath))
	}

	// Step 1: Parse tsconfig using tsgo's native JSONC parser (handles comments, trailing commas, extends).
	tsconfigStart := time.Now()
	tsFS := compiler.CreateDefaultFS()
	host := compiler.CreateDefaultHost(cwd, tsFS)

	fmt.Fprintf(os.Stderr, "compiling with tsconfig: %s\n", tsconfigPath)

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

	// Auto-infer rootDir if not set, so users get flat dist/ output without configuring it.
	// Computes common prefix of all source files (like tsc does).
	if opts.RootDir == "" && opts.OutDir != "" {
		inferredRootDir := pathalias.InferRootDir(parsedConfig.FileNames())
		if inferredRootDir != "" {
			fmt.Fprintf(os.Stderr, "inferred rootDir: %s\n", inferredRootDir)
			opts.RootDir = inferredRootDir
		}
	}

	// Resolve tsconfig path for cache file derivation
	resolvedTsconfigPath := tsconfigPath
	if !filepath.IsAbs(resolvedTsconfigPath) {
		resolvedTsconfigPath = filepath.Join(cwd, resolvedTsconfigPath)
	}
	postCachePath := buildcache.CachePath(opts.OutDir, resolvedTsconfigPath)

	// Clean output directory if requested (using parsed OutDir, no re-parsing needed)
	if clean && opts.OutDir != "" {
		if cleanErr := cleanDir(opts.OutDir); cleanErr != nil {
			fmt.Fprintf(os.Stderr, "warning: clean: %v\n", cleanErr)
		}
		// Also delete the .tsbuildinfo file — otherwise the incremental program
		// thinks nothing changed and won't re-emit the JS files we just deleted.
		tsbuildInfoPath := strings.TrimSuffix(resolvedTsconfigPath, ".json") + ".tsbuildinfo"
		if _, err := os.Stat(tsbuildInfoPath); err == nil {
			os.Remove(tsbuildInfoPath)
			fmt.Fprintf(os.Stderr, "removed %s\n", filepath.Base(tsbuildInfoPath))
		}
		// Also delete the post-processing cache — ensures full rebuild
		buildcache.Delete(postCachePath)
	}
	timing.TSConfig = time.Since(tsconfigStart)

	// Step 2: Create program with the (possibly modified) config.
	programStart := time.Now()
	program, programDiags, err := compiler.CreateProgramFromConfig(true, parsedConfig, host)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if len(programDiags) > 0 {
		fmt.Fprint(os.Stderr, compiler.FormatDiagnostics(programDiags))
		return 1
	}
	timing.Program = time.Since(programStart)

	// Handle --dump-metadata: skip emit, just analyze types
	if dumpMetadata {
		return runDumpMetadata(program, opts)
	}

	// Step 3: Gather diagnostics (forces type checking).
	// If tsconfig has "incremental: true" or "composite: true", use the incremental
	// pipeline — only checks/emits changed files, persists state to .tsbuildinfo.
	pretty := compiler.IsPrettyOutput()
	reportDiag := compiler.CreateDiagnosticReporter(os.Stderr, cwd, pretty)

	isIncremental := opts.IsIncremental()

	var allDiagnostics []*ast.Diagnostic
	var incrProgram *shimincremental.Program

	if isIncremental {
		// Incremental mode: wrap program with incremental state.
		// ReadBuildInfoProgram reads prior state from .tsbuildinfo (if it exists).
		incrProgram = compiler.CreateIncrementalProgram(program, nil, host, parsedConfig)
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
		sharedChecker, checkerRelease = shimcompiler.Program_GetTypeChecker(program, context.Background())
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
			neededTypes = collectNeededTypes(controllers, markerCalls)
		}

		// Collect query/param DTO type names that need coercion
		coercionTypes := collectCoercionTypes(controllers)

		// ── Step 4: Generate companions only for needed types ────────────
		companionStart := time.Now()
		if needCompanions && (neededTypes == nil || len(neededTypes) > 0) {
			companions, typesByFile, compErr := generateCompanionsInMemory(program, cfg, sourceToOutput, sharedChecker, sharedWalker, syntaxErrorFiles, modFmt, neededTypes, coercionTypes)
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
		fmt.Fprintln(os.Stderr, "no files emitted (noEmitOnError)")
		timing.Total = time.Since(buildStart)
		timing.Print()
		return 2
	}

	emittedFiles := emitResult.EmittedFiles
	if len(emittedFiles) > 0 {
		fmt.Fprintf(os.Stderr, "emitted %d file(s)\n", len(emittedFiles))
	} else if !emitResult.EmitSkipped {
		fmt.Fprintln(os.Stderr, "no files emitted")
	}

	// ── Early exit on diagnostic errors ──────────────────────────────────
	// Path aliases were already resolved in the WriteFile callback.
	if hasErrors {
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
		fmt.Fprintf(os.Stderr, "generated %d companion file(s)\n", len(allCompanions))
	}
	if len(controllers) > 0 {
		totalRoutes := 0
		for _, ctrl := range controllers {
			totalRoutes += len(ctrl.Routes)
		}
		fmt.Fprintf(os.Stderr, "found %d controller(s) with %d route(s)\n", len(controllers), totalRoutes)
	}

	// Print controller analyzer warnings (stored during pre-emit analysis),
	// even when zero controllers were extracted.
	for _, w := range controllerWarnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w.Message)
	}

	// Generate OpenAPI document (using pre-analyzed controllers)
	openapiStart := time.Now()
	if cfg != nil && cfg.OpenAPI.Output != "" && len(controllers) > 0 {
		openapiErr := generateOpenAPIFromControllers(controllers, controllerRegistry, cfg, configDir)
		if openapiErr != nil {
			fmt.Fprintf(os.Stderr, "error generating OpenAPI: %v\n", openapiErr)
			return 1
		}
	}
	timing.OpenAPI = time.Since(openapiStart)

	// Generate TypeScript SDK if configured (runs in background goroutine)
	var sdkWg sync.WaitGroup
	var sdkErr error
	if cfg != nil && cfg.SDK.Output != "" {
		sdkInput := cfg.SDK.Input
		if sdkInput == "" {
			// Default: use OpenAPI output as SDK input
			sdkInput = cfg.OpenAPI.Output
		}
		if sdkInput != "" {
			if !filepath.IsAbs(sdkInput) {
				sdkInput = filepath.Join(configDir, sdkInput)
			}
			sdkOutput := cfg.SDK.Output
			if !filepath.IsAbs(sdkOutput) {
				sdkOutput = filepath.Join(configDir, sdkOutput)
			}
			sdkWg.Add(1)
			go func() {
				defer sdkWg.Done()
				// Hash-based skip: compare SHA256 of input against cached hash
				sdkHashPath := filepath.Join(sdkOutput, ".sdk-hash")
				inputHash := hashFileContent(sdkInput)
				if existingHash, err := os.ReadFile(sdkHashPath); err == nil && string(existingHash) == inputHash {
					fmt.Fprintln(os.Stderr, "SDK up to date, skipping generation")
					return
				}
				if err := sdkgen.Generate(sdkInput, sdkOutput); err != nil {
					sdkErr = err
					return
				}
				os.WriteFile(sdkHashPath, []byte(inputHash), 0o644)
				fmt.Fprintf(os.Stderr, "generated SDK: %s\n", cfg.SDK.Output)
			}()
		}
	}

	// Copy static assets if configured
	if assets != "" {
		outDir := determineOutputDir(allCompanions, emittedFiles, cwd)
		count, assetErr := copyAssets(cwd, outDir, assets)
		if assetErr != nil {
			fmt.Fprintf(os.Stderr, "warning: copying assets: %v\n", assetErr)
		} else if count > 0 {
			fmt.Fprintf(os.Stderr, "copied %d asset(s)\n", count)
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
	if cfg != nil && cfg.OpenAPI.Output != "" {
		openapiOutput := cfg.OpenAPI.Output
		if !filepath.IsAbs(openapiOutput) {
			openapiOutput = filepath.Join(configDir, openapiOutput)
		}
		cacheOutputs = append(cacheOutputs, openapiOutput)
	}
	postCache := buildcache.New(configHash, cacheOutputs)
	if saveErr := buildcache.Save(postCachePath, postCache); saveErr != nil {
		fmt.Fprintf(os.Stderr, "warning: saving post-processing cache: %v\n", saveErr)
	}

	timing.Total = time.Since(buildStart)
	timing.Print()

	return 0
}

// cleanDir removes a directory after safety checks.
func cleanDir(outDir string) error {
	if outDir == "/" || outDir == "." || outDir == ".." {
		return fmt.Errorf("refusing to clean dangerous path: %s", outDir)
	}

	if _, err := os.Stat(outDir); os.IsNotExist(err) {
		return nil
	}

	fmt.Fprintf(os.Stderr, "cleaning output directory: %s\n", outDir)
	return os.RemoveAll(outDir)
}

// generateOpenAPIFromControllers generates an OpenAPI 3.1 document from pre-analyzed controllers.
// This avoids creating a duplicate type checker and re-analyzing controllers.
func generateOpenAPIFromControllers(controllers []analyzer.ControllerInfo, registry *metadata.TypeRegistry, cfg *config.Config, configDir string) error {
	// Generate OpenAPI document with versioning and prefix options
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

	// Apply document-level config (title, description, servers, security schemes)
	docCfg := openapi.DocumentConfig{
		Title:       cfg.OpenAPI.Title,
		Description: cfg.OpenAPI.Description,
		Version:     cfg.OpenAPI.Version,
	}
	if cfg.OpenAPI.Contact != nil {
		docCfg.Contact = &openapi.Contact{
			Name:  cfg.OpenAPI.Contact.Name,
			URL:   cfg.OpenAPI.Contact.URL,
			Email: cfg.OpenAPI.Contact.Email,
		}
	}
	if cfg.OpenAPI.License != nil {
		docCfg.License = &openapi.License{
			Name: cfg.OpenAPI.License.Name,
			URL:  cfg.OpenAPI.License.URL,
		}
	}
	for _, s := range cfg.OpenAPI.Servers {
		docCfg.Servers = append(docCfg.Servers, openapi.Server{
			URL:         s.URL,
			Description: s.Description,
		})
	}
	if len(cfg.OpenAPI.SecuritySchemes) > 0 {
		docCfg.SecuritySchemes = make(map[string]*openapi.SecurityScheme)
		for name, s := range cfg.OpenAPI.SecuritySchemes {
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
	outputPath := cfg.OpenAPI.Output
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

	fmt.Fprintf(os.Stderr, "generated OpenAPI document: %s\n", cfg.OpenAPI.Output)
	return nil
}

// collectNeededTypes gathers the set of type names that actually need companion files.
// A type is "needed" if it's referenced as a controller body parameter, return type,
// or in an explicit marker call (tsgonest.validate<T>(), tsgonest.assert<T>(), etc.).
// collectCoercionTypes returns the set of type names used as whole-object
// @Query() or @Param() parameters. These need string→number/boolean coercion
// enabled in their companion assert functions.
func collectCoercionTypes(controllers []analyzer.ControllerInfo) map[string]bool {
	types := make(map[string]bool)
	for _, ctrl := range controllers {
		for _, route := range ctrl.Routes {
			for _, param := range route.Parameters {
				if (param.Category == "query" || param.Category == "param" || param.Category == "headers") && param.Name == "" && param.TypeName != "" {
					types[param.TypeName] = true
				}
			}
		}
	}
	return types
}

func collectNeededTypes(controllers []analyzer.ControllerInfo, markerCalls map[string][]rewrite.MarkerCall) map[string]bool {
	needed := make(map[string]bool)

	// Types from controller routes:
	// - @Body() params need assert companions (validation injection)
	// - Whole-object @Query/@Param/@Headers need assert companions (validation + coercion injection)
	// - Return types need stringify companions (serialization injection)
	// - Individual named scalar @Param/@Query params get inline coercion (no companion needed)
	for _, ctrl := range controllers {
		for _, route := range ctrl.Routes {
			for _, param := range route.Parameters {
				switch param.Category {
				case "body":
					// Use the explicit TypeName from AST analysis first
					if param.TypeName != "" {
						needed[param.TypeName] = true
					}
					// Also scan metadata for nested refs
					collectTypeNamesFromMetadata(&param.Type, needed)
				case "query", "headers", "param":
					// Whole-object params need assert companions
					if param.TypeName != "" && param.Name == "" {
						needed[param.TypeName] = true
					}
				}
			}
			// Return types
			collectTypeNamesFromMetadata(&route.ReturnType, needed)
		}
	}

	// Types from marker calls
	for _, calls := range markerCalls {
		for _, call := range calls {
			if call.TypeName != "" {
				needed[call.TypeName] = true
			}
		}
	}

	if os.Getenv("TSGONEST_DEBUG_COMPANIONS") == "1" {
		fmt.Fprintf(os.Stderr, "debug: needed types (%d): ", len(needed))
		for name := range needed {
			fmt.Fprintf(os.Stderr, "%s ", name)
		}
		fmt.Fprintln(os.Stderr)
	}

	return needed
}

// collectTypeNamesFromMetadata recursively extracts named type references from metadata.
// It checks both .Name (set by type walker for named types) and .Ref (for cross-type references).
func collectTypeNamesFromMetadata(m *metadata.Metadata, names map[string]bool) {
	if m == nil {
		return
	}
	if m.Name != "" {
		names[m.Name] = true
	}
	if m.Ref != "" {
		names[m.Ref] = true
	}
	// For arrays, collect the element type name (e.g., Promise<UserDto[]> → UserDto)
	if m.ElementType != nil {
		if m.ElementType.Name != "" {
			names[m.ElementType.Name] = true
		}
		if m.ElementType.Ref != "" {
			names[m.ElementType.Ref] = true
		}
	}
	// Do NOT recurse into Properties, UnionMembers — nested types are inlined
	// by codegen and don't need separate companion files.
}

// generateCompanionsInMemory generates companion file content in memory without writing to disk.
// Returns both the companion files and a map of source file → type names found in that file.
// Only generates companions for types in the neededTypes set.
// fileTypeInfo holds the type walking results for a source file.
type fileTypeInfo struct {
	sourceName string
	outputBase string
	types      map[string]*metadata.Metadata
}

func generateCompanionsInMemory(program *shimcompiler.Program, cfg *config.Config, sourceToOutput map[string]string, checker *shimchecker.Checker, walker *analyzer.TypeWalker, skipFiles map[string]bool, moduleFormat string, neededTypes map[string]bool, coercionTypes map[string]bool) ([]codegen.CompanionFile, map[string][]string, error) {
	typesByFile := make(map[string][]string)

	// ── Phase 1: Walk types (sequential — uses shared checker) ──────────
	walkStart := time.Now()
	var fileInfos []fileTypeInfo

	for _, sf := range program.GetSourceFiles() {
		if sf.IsDeclarationFile {
			continue
		}
		if skipFiles[sf.FileName()] {
			fmt.Fprintf(os.Stderr, "warning: skipping companion generation for %s (syntax errors)\n", filepath.Base(sf.FileName()))
			continue
		}
		if len(cfg.Transforms.Include) > 0 {
			if !analyzer.MatchesGlob(sf.FileName(), cfg.Transforms.Include, nil) {
				continue
			}
		}
		outputBase, ok := sourceToOutput[sf.FileName()]
		if !ok {
			continue
		}

		types := make(map[string]*metadata.Metadata)
		for _, stmt := range sf.Statements.Nodes {
			switch stmt.Kind {
			case ast.KindTypeAliasDeclaration:
				decl := stmt.AsTypeAliasDeclaration()
				name := decl.Name().Text()
				if neededTypes != nil && !neededTypes[name] {
					continue
				}
				resolvedType := shimchecker.Checker_getTypeFromTypeNode(checker, decl.Type)
				m := walker.WalkNamedType(name, resolvedType)
				types[name] = &m
			case ast.KindInterfaceDeclaration:
				decl := stmt.AsInterfaceDeclaration()
				name := decl.Name().Text()
				if neededTypes != nil && !neededTypes[name] {
					continue
				}
				sym := checker.GetSymbolAtLocation(decl.Name())
				if sym != nil {
					resolvedType := shimchecker.Checker_getDeclaredTypeOfSymbol(checker, sym)
					m := walker.WalkType(resolvedType)
					types[name] = &m
				}
			}
		}

		if len(types) == 0 {
			continue
		}

		// Track type names per source file for companion map building
		var fileTypeNames []string
		for name := range types {
			fileTypeNames = append(fileTypeNames, name)
		}
		typesByFile[sf.FileName()] = fileTypeNames

		fileInfos = append(fileInfos, fileTypeInfo{
			sourceName: sf.FileName(),
			outputBase: outputBase,
			types:      types,
		})
	}
	walkDuration := time.Since(walkStart)

	// Enable string→number/boolean coercion on registry entries for query/param DTOs.
	// This must happen after Phase 1 (types walked into registry) and before Phase 2 (codegen).
	if len(coercionTypes) > 0 {
		registry := walker.Registry()
		for typeName := range coercionTypes {
			if m, ok := registry.Types[typeName]; ok {
				analyzer.AutoEnableCoercion(m)
			}
		}
	}

	// ── Phase 2: Generate companion code (parallel) ──────────────────────
	codegenStart := time.Now()
	registry := walker.Registry()
	companionOpts := codegen.CompanionOptions{
		ModuleFormat:   moduleFormat,
		StandardSchema: cfg.Transforms.StandardSchema,
	}

	type codegenResult struct {
		companions []codegen.CompanionFile
	}
	results := make([]codegenResult, len(fileInfos))

	var wg sync.WaitGroup
	// Use a semaphore to limit concurrency to available CPUs
	sem := make(chan struct{}, runtime.NumCPU())

	for i, fi := range fileInfos {
		wg.Add(1)
		go func(idx int, info fileTypeInfo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[idx] = codegenResult{
				companions: codegen.GenerateCompanionFiles(info.outputBase, info.types, registry, companionOpts),
			}
		}(i, fi)
	}
	wg.Wait()

	// Collect results
	var allCompanions []codegen.CompanionFile
	for _, r := range results {
		allCompanions = append(allCompanions, r.companions...)
	}
	codegenDuration := time.Since(codegenStart)

	if os.Getenv("TSGONEST_DEBUG_COMPANIONS") == "1" {
		fmt.Fprintf(os.Stderr, "companion stats: files=%d companions=%d walk=%s codegen=%s\n",
			len(fileInfos), len(allCompanions), walkDuration, codegenDuration)
	}

	return allCompanions, typesByFile, nil
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

// metadataDump is the JSON output structure for --dump-metadata.
type metadataDump struct {
	Files    []fileDump                    `json:"files"`
	Registry map[string]*metadata.Metadata `json:"registry"`
}

type fileDump struct {
	FileName string                       `json:"fileName"`
	Types    map[string]metadata.Metadata `json:"types"`
}

// runDumpMetadata extracts type metadata from all non-declaration source files
// and outputs it as JSON to stdout.
func runDumpMetadata(program *shimcompiler.Program, opts *core.CompilerOptions) int {
	checker, release := shimcompiler.Program_GetTypeChecker(program, context.Background())
	if checker == nil {
		fmt.Fprintln(os.Stderr, "error: could not get type checker")
		return 1
	}
	defer release()

	walker := analyzer.NewTypeWalker(checker)
	if opts.ExactOptionalPropertyTypes == core.TSTrue {
		walker.SetExactOptionalPropertyTypes(true)
	}

	var files []fileDump
	for _, sf := range program.GetSourceFiles() {
		if sf.IsDeclarationFile {
			continue
		}

		types := make(map[string]metadata.Metadata)

		for _, stmt := range sf.Statements.Nodes {
			switch stmt.Kind {
			case ast.KindTypeAliasDeclaration:
				decl := stmt.AsTypeAliasDeclaration()
				name := decl.Name().Text()
				resolvedType := shimchecker.Checker_getTypeFromTypeNode(checker, decl.Type)
				m := walker.WalkNamedType(name, resolvedType)
				types[name] = m

			case ast.KindInterfaceDeclaration:
				decl := stmt.AsInterfaceDeclaration()
				name := decl.Name().Text()
				sym := checker.GetSymbolAtLocation(decl.Name())
				if sym != nil {
					resolvedType := shimchecker.Checker_getDeclaredTypeOfSymbol(checker, sym)
					types[name] = walker.WalkType(resolvedType)
				}

			case ast.KindClassDeclaration:
				decl := stmt.AsClassDeclaration()
				if decl.Name() != nil {
					name := decl.Name().Text()
					sym := checker.GetSymbolAtLocation(decl.Name())
					if sym != nil {
						resolvedType := shimchecker.Checker_getDeclaredTypeOfSymbol(checker, sym)
						types[name] = walker.WalkType(resolvedType)
					}
				}

			case ast.KindEnumDeclaration:
				decl := stmt.AsEnumDeclaration()
				name := decl.Name().Text()
				sym := checker.GetSymbolAtLocation(decl.Name())
				if sym != nil {
					resolvedType := shimchecker.Checker_getDeclaredTypeOfSymbol(checker, sym)
					types[name] = walker.WalkType(resolvedType)
				}
			}
		}

		if len(types) > 0 {
			files = append(files, fileDump{
				FileName: sf.FileName(),
				Types:    types,
			})
		}
	}

	dump := metadataDump{
		Files:    files,
		Registry: walker.Registry().Types,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(dump); err != nil {
		fmt.Fprintf(os.Stderr, "error encoding JSON: %v\n", err)
		return 1
	}
	return 0
}

// hashFileContent returns the hex-encoded SHA256 hash of a file's content.
// Returns an empty string if the file cannot be read.
func hashFileContent(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
