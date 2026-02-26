package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/microsoft/typescript-go/shim/ast"
	shimchecker "github.com/microsoft/typescript-go/shim/checker"
	shimcompiler "github.com/microsoft/typescript-go/shim/compiler"
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
				// Always walk type aliases with WalkNamedType so they are
				// registered in the TypeRegistry and typeIdToName cache.
				// This ensures sub-field type aliases encountered during
				// property walking of other types resolve to $refs instead
				// of being inlined.
				resolvedType := shimchecker.Checker_getTypeFromTypeNode(checker, decl.Type)
				m := walker.WalkNamedType(name, resolvedType)
				// Only collect for companion generation if needed.
				if neededTypes == nil || neededTypes[name] {
					types[name] = &m
				}
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
