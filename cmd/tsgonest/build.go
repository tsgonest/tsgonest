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
	"github.com/tsgonest/tsgonest/internal/codegen"
	"github.com/tsgonest/tsgonest/internal/compiler"
	"github.com/tsgonest/tsgonest/internal/config"
	"github.com/tsgonest/tsgonest/internal/metadata"
	"github.com/tsgonest/tsgonest/internal/openapi"
	"github.com/tsgonest/tsgonest/internal/pathalias"
)

// runBuild executes the full build pipeline:
// compile -> path alias resolution -> companions -> manifest -> OpenAPI -> assets.
func runBuild(args []string) int {
	buildFlags := flag.NewFlagSet("build", flag.ExitOnError)

	var (
		configPath   string
		tsconfigPath string
		dumpMetadata bool
		clean        bool
		assets       string
	)

	buildFlags.StringVar(&configPath, "config", "", "Path to tsgonest config file (tsgonest.config.json)")
	buildFlags.StringVar(&tsconfigPath, "project", "tsconfig.json", "Path to tsconfig.json (or use -p)")
	buildFlags.StringVar(&tsconfigPath, "p", "tsconfig.json", "Path to tsconfig.json (shorthand for --project)")
	buildFlags.BoolVar(&dumpMetadata, "dump-metadata", false, "Dump type metadata as JSON to stdout (debug)")
	buildFlags.BoolVar(&clean, "clean", false, "Clean output directory before building")
	buildFlags.StringVar(&assets, "assets", "", "Glob pattern for static assets to copy to output")

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

	// Load config if specified
	var cfg *config.Config
	configDir := cwd // default: resolve relative paths from CWD
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
		configDir = filepath.Dir(resolvedConfigPath)
		fmt.Fprintf(os.Stderr, "loaded config from %s\n", configPath)
	} else {
		// Try default config file
		defaultPaths := []string{
			filepath.Join(cwd, "tsgonest.config.json"),
		}
		for _, p := range defaultPaths {
			if _, statErr := os.Stat(p); statErr == nil {
				cfg, err = config.Load(p)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error: %v\n", err)
					return 1
				}
				configDir = filepath.Dir(p)
				fmt.Fprintf(os.Stderr, "loaded config from %s\n", filepath.Base(p))
				break
			}
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

	// Clean output directory if requested (using parsed OutDir, no re-parsing needed)
	if clean && opts.OutDir != "" {
		if cleanErr := cleanDir(opts.OutDir); cleanErr != nil {
			fmt.Fprintf(os.Stderr, "warning: clean: %v\n", cleanErr)
		}
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

	// Emit JavaScript
	emitStart := time.Now()
	emittedFiles, emitDiags, err := compiler.EmitProgram(program)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error during emit: %v\n", err)
		return 1
	}
	if len(emitDiags) > 0 {
		fmt.Fprint(os.Stderr, compiler.FormatDiagnostics(emitDiags))
	}
	emitDur := time.Since(emitStart)

	if len(emittedFiles) > 0 {
		fmt.Fprintf(os.Stderr, "emitted %d file(s)\n", len(emittedFiles))
	} else {
		fmt.Fprintln(os.Stderr, "no files emitted")
	}

	// Resolve path aliases in emitted files using the parsed tsconfig paths.
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
	companionStart := time.Now()
	var allCompanions []codegen.CompanionFile
	if needCompanions {
		companions, compErr := generateCompanionsWithWalker(program, cfg, emittedFiles, opts.RootDir, opts.OutDir, sharedChecker, sharedWalker)
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
		// Print warnings, summarizing repeated types
		warnings := ca.Warnings()
		var missingReturnCount int
		for _, w := range warnings {
			if w.Kind == "missing-return-type" {
				missingReturnCount++
			} else {
				fmt.Fprintf(os.Stderr, "warning: %s\n", w.Message)
			}
		}
		if missingReturnCount > 0 {
			fmt.Fprintf(os.Stderr, "warning: %d route(s) have no return type annotation — responses will be untyped in OpenAPI.\n", missingReturnCount)
			fmt.Fprintf(os.Stderr, "         Add explicit return types like Promise<YourDto> for proper documentation and serialization.\n")
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

	totalDur := time.Since(buildStart)

	// Print timing breakdown
	fmt.Fprintf(os.Stderr, "\n--- timing ---\n")
	fmt.Fprintf(os.Stderr, "  tsconfig:      %s\n", tsconfigDur.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "  program:       %s\n", programDur.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "  emit:          %s\n", emitDur.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "  aliases:       %s\n", aliasDur.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "  checker:       %s\n", checkerDur.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "  companions:    %s\n", companionDur.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "  controllers:   %s\n", controllerDur.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "  manifest:      %s\n", manifestDur.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "  openapi:       %s\n", openapiDur.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "  total:         %s\n", totalDur.Round(time.Millisecond))

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

// generateCompanionsWithWalker analyzes types and generates companion files using a shared checker + walker.
func generateCompanionsWithWalker(program *shimcompiler.Program, cfg *config.Config, emittedFiles []string, rootDir, outDir string, checker *shimchecker.Checker, walker *analyzer.TypeWalker) ([]codegen.CompanionFile, error) {
	// Build a map from source file name -> emitted JS path
	// so we can write companions alongside the emitted JS.
	sourceToOutput := buildSourceToOutputMap(program, emittedFiles, rootDir, outDir)

	var allCompanions []codegen.CompanionFile

	for _, sf := range program.GetSourceFiles() {
		if sf.IsDeclarationFile {
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
