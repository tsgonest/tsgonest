package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/microsoft/typescript-go/shim/ast"
	shimchecker "github.com/microsoft/typescript-go/shim/checker"
	shimcompiler "github.com/microsoft/typescript-go/shim/compiler"
	"github.com/tsgonest/tsgonest/internal/analyzer"
	"github.com/tsgonest/tsgonest/internal/buildcache"
	"github.com/tsgonest/tsgonest/internal/codegen"
	"github.com/tsgonest/tsgonest/internal/compiler"
	"github.com/tsgonest/tsgonest/internal/config"
	"github.com/tsgonest/tsgonest/internal/metadata"
	"github.com/tsgonest/tsgonest/internal/openapi"
	"github.com/tsgonest/tsgonest/internal/pathalias"
)

// runBuild executes the full build pipeline:
// diagnostics -> compile -> path alias resolution -> companions -> manifest -> OpenAPI -> assets.
//
// Exit codes (matching tsgo):
//
//	0 = success, no errors
//	1 = diagnostics present, outputs generated
//	2 = diagnostics present, outputs skipped (e.g. noEmitOnError)
func runBuild(args []string) int {
	buildFlags := flag.NewFlagSet("build", flag.ExitOnError)

	var (
		configPath   string
		tsconfigPath string
		dumpMetadata bool
		clean        bool
		assets       string
		noCheck      bool
	)

	buildFlags.StringVar(&configPath, "config", "", "Path to tsgonest config file (tsgonest.config.ts or .json)")
	buildFlags.StringVar(&tsconfigPath, "project", "tsconfig.json", "Path to tsconfig.json (or use -p)")
	buildFlags.StringVar(&tsconfigPath, "p", "tsconfig.json", "Path to tsconfig.json (shorthand for --project)")
	buildFlags.BoolVar(&dumpMetadata, "dump-metadata", false, "Dump type metadata as JSON to stdout (debug)")
	buildFlags.BoolVar(&clean, "clean", false, "Clean output directory before building")
	buildFlags.StringVar(&assets, "assets", "", "Glob pattern for static assets to copy to output")
	buildFlags.BoolVar(&noCheck, "no-check", false, "Skip type checking (syntax errors are still reported)")

	buildFlags.Usage = func() {
		fmt.Println("Usage: tsgonest build [flags]")
		fmt.Println()
		fmt.Println("Flags:")
		buildFlags.PrintDefaults()
	}

	buildFlags.Parse(args)

	buildStart := time.Now()

	// Resolve working directory
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: could not get working directory: %v\n", err)
		return 1
	}

	// Load config if specified.
	// resolvedConfigPath is hoisted to function scope — needed later for cache hashing.
	var cfg *config.Config
	var resolvedConfigPath string
	configDir := cwd // default: resolve relative paths from CWD
	if configPath != "" {
		resolvedConfigPath = configPath
		if !filepath.IsAbs(resolvedConfigPath) {
			resolvedConfigPath = filepath.Join(cwd, resolvedConfigPath)
		}
		cfg, err = config.Load(resolvedConfigPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		configDir = filepath.Dir(resolvedConfigPath)
		fmt.Fprintf(os.Stderr, "loaded config from %s\n", configPath)
	} else {
		// Auto-discover config file: tsgonest.config.ts > tsgonest.config.json
		if p := config.Discover(cwd); p != "" {
			cfg, err = config.Load(p)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			resolvedConfigPath = p
			configDir = filepath.Dir(p)
			fmt.Fprintf(os.Stderr, "loaded config from %s\n", filepath.Base(p))
		}
	}

	// Step 1: Parse tsconfig using tsgo's native JSONC parser (handles comments, trailing commas, extends).
	tsconfigStart := time.Now()
	tsFS := compiler.CreateDefaultFS()
	host := compiler.CreateDefaultHost(cwd, tsFS)

	fmt.Fprintf(os.Stderr, "compiling with tsconfig: %s\n", tsconfigPath)

	parsedConfig, diags, err := compiler.ParseTSConfig(tsFS, cwd, tsconfigPath, host)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if len(diags) > 0 {
		fmt.Fprint(os.Stderr, compiler.FormatDiagnostics(diags))
		return 1
	}

	opts := parsedConfig.CompilerOptions()

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
	postCachePath := buildcache.CachePath(resolvedTsconfigPath)

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
	tsconfigDur := time.Since(tsconfigStart)

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
	programDur := time.Since(programStart)

	// Handle --dump-metadata: skip emit, just analyze types
	if dumpMetadata {
		return runDumpMetadata(program)
	}

	// Step 3: Gather diagnostics + emit.
	// If tsconfig has "incremental: true" or "composite: true", use the incremental
	// pipeline — only checks/emits changed files, persists state to .tsbuildinfo.
	pretty := compiler.IsPrettyOutput()
	reportDiag := compiler.CreateDiagnosticReporter(os.Stderr, cwd, pretty)

	isIncremental := opts.IsIncremental()

	var allDiagnostics []*ast.Diagnostic
	var emitResult *compiler.EmitResult
	var diagDur, emitDur time.Duration

	if isIncremental {
		// Incremental mode: wrap program with incremental state.
		// ReadBuildInfoProgram reads prior state from .tsbuildinfo (if it exists).
		incrProgram := compiler.CreateIncrementalProgram(program, nil, host, parsedConfig)
		fmt.Fprintln(os.Stderr, "incremental build enabled")

		diagStart := time.Now()
		allDiagnostics = compiler.GatherIncrementalDiagnostics(incrProgram, noCheck)
		diagDur = time.Since(diagStart)

		// Emit through incremental program — only changed files + writes .tsbuildinfo
		emitStart := time.Now()
		emitResult = compiler.EmitIncrementalProgram(incrProgram)
		emitDur = time.Since(emitStart)
	} else {
		diagStart := time.Now()
		allDiagnostics = compiler.GatherDiagnostics(program, noCheck)
		diagDur = time.Since(diagStart)

		// Emit JavaScript (non-incremental)
		emitStart := time.Now()
		emitResult = compiler.EmitProgram(program)
		emitDur = time.Since(emitStart)
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
	// - No errors → continue to NestJS analysis
	hasErrors := compiler.CountErrors(allDiagnostics) > 0
	if emitResult.EmitSkipped && hasErrors {
		// noEmitOnError triggered — no files written
		fmt.Fprintln(os.Stderr, "no files emitted (noEmitOnError)")
		totalDur := time.Since(buildStart)
		printTiming(tsconfigDur, programDur, diagDur, emitDur, 0, 0, 0, 0, 0, 0, totalDur)
		return 2
	}

	emittedFiles := emitResult.EmittedFiles
	if len(emittedFiles) > 0 {
		fmt.Fprintf(os.Stderr, "emitted %d file(s)\n", len(emittedFiles))
	} else if !emitResult.EmitSkipped {
		fmt.Fprintln(os.Stderr, "no files emitted")
	}

	// ── Early exit on diagnostic errors ──────────────────────────────────
	// If TypeScript has errors, skip all post-processing (companions, controllers,
	// manifest, OpenAPI). The type checker data may be incomplete/unreliable, and
	// tsgonest warnings are noise when the user needs to fix TS errors first.
	// Path aliases are still resolved since emitted JS needs correct imports.
	if hasErrors {
		// Resolve path aliases so the emitted JS is usable despite errors
		aliasStart := time.Now()
		if opts.Paths != nil && opts.Paths.Size() > 0 {
			pathsMap := make(map[string][]string)
			for k, v := range opts.Paths.Entries() {
				pathsMap[k] = v
			}
			pathsBaseDir := opts.GetPathsBasePath(cwd)

			resolver := pathalias.NewPathResolver(pathalias.Config{
				PathsBaseDir: pathsBaseDir,
				OutDir:       opts.OutDir,
				RootDir:      opts.RootDir,
				Paths:        pathsMap,
			})
			count, aliasErr := resolver.ResolveAllEmittedFiles(emittedFiles)
			if aliasErr != nil {
				fmt.Fprintf(os.Stderr, "warning: path alias resolution: %v\n", aliasErr)
			} else if count > 0 {
				fmt.Fprintf(os.Stderr, "resolved path aliases in %d file(s)\n", count)
			}
		}
		aliasDur := time.Since(aliasStart)

		totalDur := time.Since(buildStart)
		printTiming(tsconfigDur, programDur, diagDur, emitDur, aliasDur, 0, 0, 0, 0, 0, totalDur)
		return 1
	}

	// ── Post-processing cache check ──────────────────────────────────────
	// When incremental mode reports no emitted files (nothing changed in TS),
	// AND the tsgonest config + critical output files are unchanged, we can
	// skip all expensive post-processing (aliases, checker, companions,
	// controller analysis, manifest, OpenAPI).
	//
	// If ANY condition fails → full rebuild. No partial invalidation.
	var configHash string
	if resolvedConfigPath != "" {
		configHash = buildcache.HashFile(resolvedConfigPath)
	}

	noFilesEmitted := len(emittedFiles) == 0 && !emitResult.EmitSkipped
	if noFilesEmitted && !clean {
		existingCache := buildcache.Load(postCachePath)
		if existingCache != nil && existingCache.IsValid(configHash) {
			// All checks passed — skip post-processing entirely
			fmt.Fprintln(os.Stderr, "no changes detected, outputs up to date")

			totalDur := time.Since(buildStart)
			printTiming(tsconfigDur, programDur, diagDur, emitDur, 0, 0, 0, 0, 0, 0, totalDur)

			return 0
		}
		// Cache miss/invalid — fall through to full post-processing
	}

	// When the cache is invalid but no files were emitted (incremental warm rebuild
	// with cache miss — e.g., output file deleted, config changed), we still need
	// the emitted file list to build the source→output map for companion generation.
	// In this case, enumerate existing .js files in the outDir.
	effectiveEmittedFiles := emittedFiles
	if noFilesEmitted && opts.OutDir != "" {
		existingJS := discoverJSFiles(opts.OutDir)
		if len(existingJS) > 0 {
			effectiveEmittedFiles = existingJS
		}
	}

	// Track files with syntax errors — skip companion generation for them
	syntaxErrorFiles := compiler.FilesWithSyntaxErrors(
		compiler.GetSyntacticDiagnostics(program),
	)

	// Resolve path aliases in emitted files using the parsed tsconfig paths.
	// Only process actually emitted files (not discovered ones — they were already resolved).
	aliasStart := time.Now()
	if opts.Paths != nil && opts.Paths.Size() > 0 {
		pathsMap := make(map[string][]string)
		for k, v := range opts.Paths.Entries() {
			pathsMap[k] = v
		}
		pathsBaseDir := opts.GetPathsBasePath(cwd)

		resolver := pathalias.NewPathResolver(pathalias.Config{
			PathsBaseDir: pathsBaseDir,
			OutDir:       opts.OutDir,
			RootDir:      opts.RootDir,
			Paths:        pathsMap,
		})
		count, aliasErr := resolver.ResolveAllEmittedFiles(emittedFiles)
		if aliasErr != nil {
			fmt.Fprintf(os.Stderr, "warning: path alias resolution: %v\n", aliasErr)
		} else if count > 0 {
			fmt.Fprintf(os.Stderr, "resolved path aliases in %d file(s)\n", count)
		}
	}
	aliasDur := time.Since(aliasStart)

	// Create a single shared type checker + walker for both companion generation
	// and controller analysis. Sharing the walker's registry means types already
	// walked during companion generation (DTOs) short-circuit to KindRef during
	// controller return type analysis — avoiding redundant deep type walks.
	checkerStart := time.Now()
	needCompanions := cfg != nil && (cfg.Transforms.Validation || cfg.Transforms.Serialization)
	needControllers := cfg != nil && len(cfg.Controllers.Include) > 0

	var sharedChecker *shimchecker.Checker
	var sharedWalker *analyzer.TypeWalker
	var checkerRelease func()
	if needCompanions || needControllers {
		sharedChecker, checkerRelease = shimcompiler.Program_GetTypeChecker(program, context.Background())
		if sharedChecker == nil {
			fmt.Fprintln(os.Stderr, "error: could not get type checker")
			return 1
		}
		defer checkerRelease()
		sharedWalker = analyzer.NewTypeWalker(sharedChecker)
	}
	checkerDur := time.Since(checkerStart)

	// Generate companion files (validation + serialization)
	// Skip files with syntax errors — their AST/type metadata may be unreliable.
	// Use effectiveEmittedFiles (which includes discovered files on cache-miss warm builds).
	companionStart := time.Now()
	var allCompanions []codegen.CompanionFile
	if needCompanions {
		companions, compErr := generateCompanionsWithWalker(program, cfg, effectiveEmittedFiles, opts.RootDir, opts.OutDir, sharedChecker, sharedWalker, syntaxErrorFiles)
		if compErr != nil {
			fmt.Fprintf(os.Stderr, "error generating companions: %v\n", compErr)
			return 1
		}
		allCompanions = companions
		if len(companions) > 0 {
			fmt.Fprintf(os.Stderr, "generated %d companion file(s)\n", len(companions))
		}
	}
	companionDur := time.Since(companionStart)

	// Analyze controllers using the shared checker + walker.
	// Types already registered by companion generation will short-circuit to KindRef.
	controllerStart := time.Now()
	var controllers []analyzer.ControllerInfo
	var controllerRegistry *metadata.TypeRegistry
	if needControllers {
		ca := analyzer.NewControllerAnalyzerWithWalker(program, sharedChecker, sharedWalker)
		controllers = ca.AnalyzeProgram(cfg.Controllers.Include, cfg.Controllers.Exclude)
		controllerRegistry = ca.Registry()
		// Print warnings
		warnings := ca.Warnings()
		for _, w := range warnings {
			fmt.Fprintf(os.Stderr, "warning: %s\n", w.Message)
		}
		if len(controllers) > 0 {
			totalRoutes := 0
			for _, ctrl := range controllers {
				totalRoutes += len(ctrl.Routes)
			}
			fmt.Fprintf(os.Stderr, "found %d controller(s) with %d route(s)\n", len(controllers), totalRoutes)
		}
	}
	controllerDur := time.Since(controllerStart)

	// Generate manifest file (using pre-analyzed controllers for route map)
	manifestStart := time.Now()
	if len(allCompanions) > 0 {
		var routeMap map[string]codegen.RouteMapping
		if len(controllers) > 0 {
			routeMap = buildRouteMapFromControllers(controllers)
		}
		manifestDir := opts.OutDir
		if manifestDir == "" {
			manifestDir = determineOutputDir(allCompanions, emittedFiles, cwd)
		}
		manifestErr := generateManifest(allCompanions, manifestDir, routeMap)
		if manifestErr != nil {
			fmt.Fprintf(os.Stderr, "error generating manifest: %v\n", manifestErr)
			return 1
		}
	}
	manifestDur := time.Since(manifestStart)

	// Generate OpenAPI document (using pre-analyzed controllers)
	openapiStart := time.Now()
	if cfg != nil && cfg.OpenAPI.Output != "" && len(controllers) > 0 {
		openapiErr := generateOpenAPIFromControllers(controllers, controllerRegistry, cfg, configDir)
		if openapiErr != nil {
			fmt.Fprintf(os.Stderr, "error generating OpenAPI: %v\n", openapiErr)
			return 1
		}
	}
	openapiDur := time.Since(openapiStart)

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
	if len(allCompanions) > 0 {
		manifestDir := opts.OutDir
		if manifestDir == "" {
			manifestDir = determineOutputDir(allCompanions, emittedFiles, cwd)
		}
		manifestPath := filepath.Join(manifestDir, "__tsgonest_manifest.json")
		cacheOutputs = append(cacheOutputs, manifestPath)
	}
	postCache := buildcache.New(configHash, cacheOutputs)
	if saveErr := buildcache.Save(postCachePath, postCache); saveErr != nil {
		fmt.Fprintf(os.Stderr, "warning: saving post-processing cache: %v\n", saveErr)
	}

	totalDur := time.Since(buildStart)

	// Print timing breakdown
	printTiming(tsconfigDur, programDur, diagDur, emitDur, aliasDur, checkerDur, companionDur, controllerDur, manifestDur, openapiDur, totalDur)

	return 0
}

// printTiming outputs the build timing breakdown to stderr.
func printTiming(tsconfig, program, diag, emit, aliases, checker, companions, controllers, manifest, openapi, total time.Duration) {
	fmt.Fprintf(os.Stderr, "\n--- timing ---\n")
	fmt.Fprintf(os.Stderr, "  tsconfig:      %s\n", tsconfig.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "  program:       %s\n", program.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "  diagnostics:   %s\n", diag.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "  emit:          %s\n", emit.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "  aliases:       %s\n", aliases.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "  checker:       %s\n", checker.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "  companions:    %s\n", companions.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "  controllers:   %s\n", controllers.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "  manifest:      %s\n", manifest.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "  openapi:       %s\n", openapi.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "  total:         %s\n", total.Round(time.Millisecond))
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

// generateCompanionsWithWalker analyzes types and generates companion files using a shared checker + walker.
// Files in skipFiles (source files with syntax errors) are excluded from companion generation.
func generateCompanionsWithWalker(program *shimcompiler.Program, cfg *config.Config, emittedFiles []string, rootDir, outDir string, checker *shimchecker.Checker, walker *analyzer.TypeWalker, skipFiles map[string]bool) ([]codegen.CompanionFile, error) {
	// Build a map from source file name -> emitted JS path
	// so we can write companions alongside the emitted JS.
	sourceToOutput := buildSourceToOutputMap(program, emittedFiles, rootDir, outDir)

	var allCompanions []codegen.CompanionFile

	for _, sf := range program.GetSourceFiles() {
		if sf.IsDeclarationFile {
			continue
		}

		// Skip files with syntax errors — their type metadata may be wrong
		if skipFiles[sf.FileName()] {
			fmt.Fprintf(os.Stderr, "warning: skipping companion generation for %s (syntax errors)\n", filepath.Base(sf.FileName()))
			continue
		}

		// Filter source files using transforms.include patterns (if specified)
		if len(cfg.Transforms.Include) > 0 {
			if !analyzer.MatchesGlob(sf.FileName(), cfg.Transforms.Include, nil) {
				continue
			}
		}

		// Determine output base path for this source file
		outputBase, ok := sourceToOutput[sf.FileName()]
		if !ok {
			continue
		}

		// Collect named types from this file
		types := make(map[string]*metadata.Metadata)
		for _, stmt := range sf.Statements.Nodes {
			switch stmt.Kind {
			case ast.KindTypeAliasDeclaration:
				decl := stmt.AsTypeAliasDeclaration()
				name := decl.Name().Text()
				resolvedType := shimchecker.Checker_getTypeFromTypeNode(checker, decl.Type)
				m := walker.WalkNamedType(name, resolvedType)
				types[name] = &m
			case ast.KindInterfaceDeclaration:
				decl := stmt.AsInterfaceDeclaration()
				name := decl.Name().Text()
				sym := checker.GetSymbolAtLocation(decl.Name())
				if sym != nil {
					resolvedType := shimchecker.Checker_getDeclaredTypeOfSymbol(checker, sym)
					m := walker.WalkType(resolvedType)
					types[name] = &m
				}
			case ast.KindClassDeclaration:
				decl := stmt.AsClassDeclaration()
				if decl.Name() != nil {
					// Skip controller classes — they have @Controller() decorator
					// and their companion files are useless (validates method names as any).
					if isControllerClass(stmt) {
						continue
					}
					name := decl.Name().Text()
					sym := checker.GetSymbolAtLocation(decl.Name())
					if sym != nil {
						resolvedType := shimchecker.Checker_getDeclaredTypeOfSymbol(checker, sym)
						m := walker.WalkType(resolvedType)
						types[name] = &m
					}
				}
			}
		}

		if len(types) == 0 {
			continue
		}

		// Generate companion files using the output base path
		companions := codegen.GenerateCompanionFiles(outputBase, types, walker.Registry())
		for _, comp := range companions {
			dir := filepath.Dir(comp.Path)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return allCompanions, fmt.Errorf("creating dir %s: %w", dir, err)
			}
			if err := os.WriteFile(comp.Path, []byte(comp.Content), 0644); err != nil {
				return allCompanions, fmt.Errorf("writing %s: %w", comp.Path, err)
			}
			allCompanions = append(allCompanions, comp)
		}
	}

	return allCompanions, nil
}

// generateManifest creates the __tsgonest_manifest.json file in the output directory.
func generateManifest(companions []codegen.CompanionFile, outputDir string, routeMap map[string]codegen.RouteMapping) error {
	m := codegen.GenerateManifest(companions, outputDir, routeMap)
	jsonBytes, err := codegen.ManifestJSON(m)
	if err != nil {
		return fmt.Errorf("serializing manifest: %w", err)
	}

	manifestPath := filepath.Join(outputDir, "__tsgonest_manifest.json")

	// Create output directory if needed
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("creating output directory %s: %w", outputDir, err)
	}

	if err := os.WriteFile(manifestPath, jsonBytes, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", manifestPath, err)
	}

	fmt.Fprintf(os.Stderr, "generated manifest: %s\n", manifestPath)
	return nil
}

// buildRouteMapFromControllers builds a route map from pre-analyzed controllers.
// Maps "ControllerName.methodName" → RouteMapping with the return type name.
func buildRouteMapFromControllers(controllers []analyzer.ControllerInfo) map[string]codegen.RouteMapping {
	if len(controllers) == 0 {
		return nil
	}

	routeMap := make(map[string]codegen.RouteMapping)
	for _, ctrl := range controllers {
		for _, route := range ctrl.Routes {
			// Determine return type name
			typeName := resolveReturnTypeName(&route.ReturnType)
			if typeName == "" {
				continue
			}

			key := ctrl.Name + "." + route.OperationID
			isArray := route.ReturnType.Kind == metadata.KindArray
			routeMap[key] = codegen.RouteMapping{
				ReturnType: typeName,
				IsArray:    isArray,
			}
		}
	}

	if len(routeMap) == 0 {
		return nil
	}
	return routeMap
}

// resolveReturnTypeName extracts the DTO type name from a route's return type metadata.
// For arrays, it returns the element type name.
// Returns empty string for primitive/any/void types.
func resolveReturnTypeName(m *metadata.Metadata) string {
	switch m.Kind {
	case metadata.KindRef:
		return m.Ref
	case metadata.KindObject:
		return m.Name
	case metadata.KindArray:
		if m.ElementType != nil {
			return resolveReturnTypeName(m.ElementType)
		}
	}
	return ""
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

// buildSourceToOutputMap creates a mapping from source .ts file paths to their
// emitted .js file paths (without extension), based on the emitted file list.
//
// It maps by relative path from rootDir to avoid collisions when multiple source
// files share the same base name (e.g., multiple index.ts or types.ts files).
func buildSourceToOutputMap(program *shimcompiler.Program, emittedFiles []string, rootDir, outDir string) map[string]string {
	result := make(map[string]string)

	// Build set of emitted .js files keyed by their path relative to outDir (without extension).
	// e.g., "auth/dto/auth.dto" -> "/abs/dist/auth/dto/auth.dto"
	jsFiles := make(map[string]string) // relative (no ext) -> full path (no ext)
	for _, f := range emittedFiles {
		if !strings.HasSuffix(f, ".js") {
			continue
		}
		base := f[:len(f)-3]
		if outDir != "" {
			rel, err := filepath.Rel(outDir, base)
			if err == nil {
				jsFiles[rel] = base
				continue
			}
		}
		// Fallback to base name if we can't compute relative path
		jsFiles[filepath.Base(base)] = base
	}

	// Match source files to their emitted JS counterparts.
	// For each source file, compute its path relative to rootDir. The compiler
	// mirrors this relative structure in the outDir.
	for _, sf := range program.GetSourceFiles() {
		if sf.IsDeclarationFile {
			continue
		}
		srcName := sf.FileName()

		// Strip TS extension to get the base without extension
		srcNoExt := srcName
		for _, ext := range []string{".ts", ".tsx", ".mts", ".cts"} {
			if strings.HasSuffix(srcNoExt, ext) {
				srcNoExt = srcNoExt[:len(srcNoExt)-len(ext)]
				break
			}
		}

		// Compute relative path from rootDir
		var lookupKey string
		if rootDir != "" {
			rel, err := filepath.Rel(rootDir, srcNoExt)
			if err == nil {
				lookupKey = rel
			}
		}
		if lookupKey == "" {
			lookupKey = filepath.Base(srcNoExt)
		}

		// Find matching emitted file
		if jsBase, ok := jsFiles[lookupKey]; ok {
			result[srcName] = jsBase + ".ts" // fake .ts extension for companionPath to strip
		}
	}

	return result
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
func runDumpMetadata(program *shimcompiler.Program) int {
	checker, release := shimcompiler.Program_GetTypeChecker(program, context.Background())
	if checker == nil {
		fmt.Fprintln(os.Stderr, "error: could not get type checker")
		return 1
	}
	defer release()

	walker := analyzer.NewTypeWalker(checker)

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

// isControllerClass checks if a class declaration has a @Controller() decorator.
// Used to skip companion generation for NestJS controller classes, which produce
// useless validators (they validate method names as any-typed properties).
func isControllerClass(classNode *ast.Node) bool {
	for _, dec := range classNode.Decorators() {
		info := analyzer.ParseDecorator(dec)
		if info != nil && analyzer.IsControllerDecorator(info) {
			return true
		}
	}
	return false
}
