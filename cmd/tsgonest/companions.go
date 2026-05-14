package main

import (
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
	shimscanner "github.com/microsoft/typescript-go/shim/scanner"
	"github.com/tsgonest/tsgonest/internal/analyzer"
	"github.com/tsgonest/tsgonest/internal/codegen"
	"github.com/tsgonest/tsgonest/internal/config"
	"github.com/tsgonest/tsgonest/internal/metadata"
	"github.com/tsgonest/tsgonest/internal/rewrite"
)

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
				if (param.Category == string(analyzer.CategoryQuery) || param.Category == string(analyzer.CategoryParam) || param.Category == string(analyzer.CategoryHeaders)) && param.Name == "" && param.TypeName != "" {
					types[param.TypeName] = true
				}
				// FormData body params need coercion too — multer parses fields as strings
				if param.Category == string(analyzer.CategoryBody) && param.ContentType == "multipart/form-data" && param.TypeName != "" {
					types[param.TypeName] = true
				}
			}
		}
	}
	return types
}

func collectNeededTypes(controllers []analyzer.ControllerInfo, markerCalls map[string][]rewrite.MarkerCall, excludePatterns []string) map[string]bool {
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

			// SSE event variant data types need companions for per-event
			// validation (assert) and serialization (stringify).
			for i := range route.SSEEventVariants {
				collectTypeNamesFromMetadata(&route.SSEEventVariants[i].DataType, needed)
			}
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

	// Filter out excluded type name patterns
	if len(excludePatterns) > 0 {
		for name := range needed {
			if analyzer.MatchesTypeNamePattern(name, excludePatterns) {
				delete(needed, name)
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

// inlineBodyType holds a synthesized inline body type and its source file.
type inlineBodyType struct {
	typeName   string
	sourceFile string
	meta       *metadata.Metadata
}

// collectInlineBodyTypes gathers body parameters with inline (synthesized) types
// that need companion files but won't be found by scanning top-level declarations.
// These are inline object types like `body: {file: File, id: string}` that the
// analyzer gave a synthetic name (e.g., "__UploadController_upload_Body").
func collectInlineBodyTypes(controllers []analyzer.ControllerInfo) []inlineBodyType {
	var types []inlineBodyType
	for _, ctrl := range controllers {
		for _, route := range ctrl.Routes {
			for _, param := range route.Parameters {
				if param.Category == string(analyzer.CategoryBody) && param.TypeName != "" && param.Type.Kind == metadata.KindObject && strings.HasPrefix(param.TypeName, "__") {
					m := param.Type
					types = append(types, inlineBodyType{
						typeName:   param.TypeName,
						sourceFile: ctrl.SourceFile,
						meta:       &m,
					})
				}
			}
		}
	}
	return types
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

func generateCompanionsInMemory(program *shimcompiler.Program, cfg *config.Config, sourceToOutput map[string]string, checker *shimchecker.Checker, walker *analyzer.TypeWalker, skipFiles map[string]bool, moduleFormat string, neededTypes map[string]bool, coercionTypes map[string]bool, inlineTypes []inlineBodyType) ([]codegen.CompanionFile, map[string][]string, error) {
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
				// Skip types matching exclude patterns
				if len(cfg.Transforms.Exclude) > 0 && analyzer.MatchesTypeNamePattern(name, cfg.Transforms.Exclude) {
					continue
				}
				// Only walk types referenced by controllers or marker calls.
				// Sub-field type aliases (e.g., Address inside UserDto) are
				// discovered and registered on-the-fly by the walker's
				// Type_alias recovery at depth > 1 — no blanket pre-walk needed.
				if neededTypes != nil && !neededTypes[name] {
					continue
				}
				line := shimscanner.GetECMALineOfPosition(sf, decl.Name().Pos())
				walker.SetRootContext(fmt.Sprintf("%s (%s:%d)", name, sf.FileName(), line+1))
				resolvedType := checker.GetTypeFromTypeNode(decl.Type)
				m := walker.WalkNamedType(name, resolvedType)
				walker.SetRootContext("")
				types[name] = &m
			case ast.KindInterfaceDeclaration:
				decl := stmt.AsInterfaceDeclaration()
				name := decl.Name().Text()
				// Skip types matching exclude patterns
				if len(cfg.Transforms.Exclude) > 0 && analyzer.MatchesTypeNamePattern(name, cfg.Transforms.Exclude) {
					continue
				}
				if neededTypes != nil && !neededTypes[name] {
					continue
				}
				line := shimscanner.GetECMALineOfPosition(sf, decl.Name().Pos())
				walker.SetRootContext(fmt.Sprintf("%s (%s:%d)", name, sf.FileName(), line+1))
				sym := checker.GetSymbolAtLocation(decl.Name())
				if sym != nil {
					resolvedType := checker.GetDeclaredTypeOfSymbol(sym)
					m := walker.WalkType(resolvedType)
					types[name] = &m
				}
				walker.SetRootContext("")
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

		// Record source file → output base mapping in the registry for each type.
		// This is used by codegen to compute import paths for cross-companion refs.
		{
			reg := walker.Registry()
			for name := range types {
				reg.SourceFiles[name] = outputBase
			}
		}

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

	// Inject inline body types (synthesized from inline parameter type annotations).
	// These aren't found by the top-level declaration scan, so we register them
	// manually and add them to fileInfos for companion generation.
	if len(inlineTypes) > 0 {
		reg := walker.Registry()
		for _, it := range inlineTypes {
			reg.Register(it.typeName, it.meta)
			// Record source file mapping for cross-companion import resolution
			if outputBase, ok := sourceToOutput[it.sourceFile]; ok {
				reg.SourceFiles[it.typeName] = outputBase
			}
			// Apply coercion if needed (FormData inline types always need it)
			if coercionTypes[it.typeName] {
				analyzer.AutoEnableCoercion(it.meta)
			}
			// Add to fileInfos so a companion file is generated.
			// Find or create the fileTypeInfo for this source file.
			outputBase, ok := sourceToOutput[it.sourceFile]
			if !ok {
				continue
			}
			found := false
			for i := range fileInfos {
				if fileInfos[i].sourceName == it.sourceFile {
					fileInfos[i].types[it.typeName] = it.meta
					typesByFile[it.sourceFile] = append(typesByFile[it.sourceFile], it.typeName)
					found = true
					break
				}
			}
			if !found {
				fileInfos = append(fileInfos, fileTypeInfo{
					sourceName: it.sourceFile,
					outputBase: outputBase,
					types:      map[string]*metadata.Metadata{it.typeName: it.meta},
				})
				typesByFile[it.sourceFile] = []string{it.typeName}
			}
		}
	}

	// ── Phase 2: Generate companion code (parallel) ──────────────────────
	codegenStart := time.Now()
	registry := walker.Registry()
	companionOpts := codegen.CompanionOptions{
		ModuleFormat:       moduleFormat,
		StandardSchema:     cfg.Transforms.StandardSchema,
		ResponseSerializer: cfg.Transforms.ResponseSerializer,
		SourceToOutput:     sourceToOutput,
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

// generateMintRegisterCompanions emits a `register{Controller}(app)` companion
// file for every controller whose @Controller decorator came from @mintkit/*.
// The companion sits next to the controller's emitted JS and is generated
// regardless of whether the controller has any DTO types — Phase 1 hello-world
// controllers have no @Body/@Query/@Param and thus produce no type companions.
func generateMintRegisterCompanions(controllers []analyzer.ControllerInfo, sourceToOutput map[string]string, moduleFormat string) []codegen.CompanionFile {
	var out []codegen.CompanionFile
	isCJS := moduleFormat == "cjs"

	for _, ctrl := range controllers {
		if ctrl.Framework != "mint" {
			continue
		}
		outputBase, ok := sourceToOutput[ctrl.SourceFile]
		if !ok {
			continue
		}

		// Strip .ts to get the controller's emitted-JS base path (e.g. dist/hello.controller).
		controllerBase := strings.TrimSuffix(outputBase, ".ts")
		importPath := "./" + filepath.Base(controllerBase)

		routes := make([]codegen.MintRouteInfo, 0, len(ctrl.Routes))
		for _, r := range ctrl.Routes {
			routes = append(routes, codegen.MintRouteInfo{
				Method:     r.Method,
				Path:       r.Path,
				MethodName: r.MethodName,
			})
		}

		input := codegen.MintRegisterInput{
			ControllerName:       ctrl.Name,
			ControllerImportPath: importPath,
			Routes:               routes,
		}

		jsPath := codegen.MintRegisterPath(outputBase, ctrl.Name)
		jsContent := codegen.GenerateMintRegister(input)
		dtsPath := strings.TrimSuffix(jsPath, ".js") + ".d.ts"
		dtsContent := codegen.GenerateMintRegisterTypes(ctrl.Name)

		if isCJS {
			jsContent = codegen.ConvertToCommonJS(jsContent)
			dtsContent = codegen.ConvertDtsToCommonJS(dtsContent)
		}

		out = append(out, codegen.CompanionFile{Path: jsPath, Content: jsContent})
		out = append(out, codegen.CompanionFile{Path: dtsPath, Content: dtsContent})
	}

	return out
}
