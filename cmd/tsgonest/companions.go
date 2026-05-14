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
//
// For Phase 2, each route's parameters and return type are projected into the
// codegen input so the emitted wrapper parses inputs from the Event, runs the
// existing tsgonest-generated `assertXxx` validators, and serialises the
// return value via `stringifyXxx`/`serializeXxx` from the same companions.
func generateMintRegisterCompanions(
	controllers []analyzer.ControllerInfo,
	sourceToOutput map[string]string,
	moduleFormat string,
	registry *metadata.TypeRegistry,
	responseSerializer string,
) []codegen.CompanionFile {
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

		// The Mint register companion lives at codegen.MintRegisterPath(outputBase, ctrl.Name).
		registerCompanionPath := codegen.MintRegisterPath(outputBase, ctrl.Name)

		routes := make([]codegen.MintRouteInfo, 0, len(ctrl.Routes))
		for _, r := range ctrl.Routes {
			route := codegen.MintRouteInfo{
				Method:     r.Method,
				Path:       r.Path,
				MethodName: r.MethodName,
			}

			// Project route parameters into codegen-friendly shape.
			for _, p := range r.Parameters {
				mp := buildMintParam(p, registry, registerCompanionPath)
				if mp == nil {
					continue
				}
				route.Params = append(route.Params, *mp)
			}

			// Project return type.
			projectReturn(&route, &r.ReturnType, registry, registerCompanionPath)

			routes = append(routes, route)
		}

		input := codegen.MintRegisterInput{
			ControllerName:       ctrl.Name,
			ControllerImportPath: importPath,
			Routes:               routes,
			ResponseSerializer:   responseSerializer,
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

// buildMintParam projects an analyzer.RouteParameter into the codegen shape.
// Returns nil when the parameter cannot be represented (e.g. anonymous types
// without a companion; those are intentionally skipped — the handler will see
// whatever the user typed, but validation won't run).
func buildMintParam(p analyzer.RouteParameter, registry *metadata.TypeRegistry, registerCompanionPath string) *codegen.MintParamInfo {
	local := p.LocalName
	if local == "" {
		local = p.Name
	}
	if local == "" {
		// Without a JS-visible name we can't bind the value into the call site.
		return nil
	}

	kind, ok := paramCategoryToKind(p.Category)
	if !ok {
		return nil
	}

	mp := codegen.MintParamInfo{
		Kind:      kind,
		Name:      p.Name,
		LocalName: local,
	}

	// Body with File/FileStream fields → multipart parsing path.
	if kind == codegen.MintParamBody {
		if mb := buildMultipartBody(&p, registry); mb != nil {
			mp.Multipart = mb
			return &mp
		}
	}

	// DTO-shaped param: whole-object @Body / @Query / @Headers with a named type.
	dtoSlot := (kind == codegen.MintParamBody) ||
		((kind == codegen.MintParamQuery || kind == codegen.MintParamHeader) && p.Name == "")
	if dtoSlot && p.TypeName != "" && registry != nil {
		if companionImport := companionImportForType(p.TypeName, registry, registerCompanionPath); companionImport != "" {
			mp.TypeName = p.TypeName
			mp.CompanionImport = companionImport
			return &mp
		}
	}

	// Scalar param (named @Param / @Query / @Headers, or @Body without a type).
	if p.Type.Kind == metadata.KindAtomic {
		mp.Atomic = p.Type.Atomic
		mp.Constraints = p.Type.Constraints
	}
	return &mp
}

// buildMultipartBody inspects a body parameter's type metadata for File or
// FileStream fields. When at least one is found, returns a multipart description
// that the codegen uses instead of the JSON-body path. Returns nil for JSON bodies.
func buildMultipartBody(p *analyzer.RouteParameter, registry *metadata.TypeRegistry) *codegen.MintMultipartBody {
	if p == nil {
		return nil
	}
	obj := resolveObjectMetadata(&p.Type, registry)
	if obj == nil {
		return nil
	}

	hasFile := false
	for _, prop := range obj.Properties {
		k := classifyMultipartFieldKind(&prop.Type)
		if k == multipartFieldKindFile || k == multipartFieldKindFileArray || k == multipartFieldKindFileStream {
			hasFile = true
			break
		}
	}
	if !hasFile {
		return nil
	}

	mb := &codegen.MintMultipartBody{}
	for _, prop := range obj.Properties {
		k := classifyMultipartFieldKind(&prop.Type)
		field := codegen.MintMultipartField{
			Name:     prop.Name,
			Required: prop.Required,
		}
		// File-shaped constraints can live on the leaf type (after branded
		// extraction) or on the property level — combine both.
		field.Constraints = mergeFileConstraints(prop.Type.Constraints, prop.Constraints)
		switch k {
		case multipartFieldKindFile:
			field.Kind = codegen.MintFieldFile
			mb.Fields = append(mb.Fields, field)
		case multipartFieldKindFileArray:
			field.Kind = codegen.MintFieldFileArray
			mb.Fields = append(mb.Fields, field)
		case multipartFieldKindFileStream:
			field.Kind = codegen.MintFieldFileStream
			mb.Streaming = true
			mb.Fields = append(mb.Fields, field)
		case multipartFieldKindScalar:
			field.Kind = codegen.MintFieldScalar
			leaf := scalarLeaf(&prop.Type)
			if leaf != nil {
				field.Atomic = leaf.Atomic
				field.Constraints = mergeFileConstraints(leaf.Constraints, prop.Constraints)
			}
			mb.Fields = append(mb.Fields, field)
		case multipartFieldKindUnknown:
			// Skip — emit no validation for unknown leaf types.
		}
	}

	return mb
}

type multipartFieldKind int

const (
	multipartFieldKindUnknown multipartFieldKind = iota
	multipartFieldKindScalar
	multipartFieldKindFile
	multipartFieldKindFileArray
	multipartFieldKindFileStream
)

// classifyMultipartFieldKind reports how a property of a multipart body should
// be parsed. Unwraps optional/nullable union wrappers.
func classifyMultipartFieldKind(m *metadata.Metadata) multipartFieldKind {
	if m == nil {
		return multipartFieldKindUnknown
	}
	leaf := unwrapOptional(m)
	if leaf == nil {
		return multipartFieldKindUnknown
	}
	switch leaf.Kind {
	case metadata.KindNative:
		switch leaf.NativeType {
		case "File", "Blob":
			return multipartFieldKindFile
		case "FileStream":
			return multipartFieldKindFileStream
		}
	case metadata.KindArray:
		if leaf.ElementType != nil {
			el := unwrapOptional(leaf.ElementType)
			if el != nil && el.Kind == metadata.KindNative && (el.NativeType == "File" || el.NativeType == "Blob") {
				return multipartFieldKindFileArray
			}
		}
	case metadata.KindAtomic, metadata.KindLiteral:
		return multipartFieldKindScalar
	}
	return multipartFieldKindUnknown
}

func scalarLeaf(m *metadata.Metadata) *metadata.Metadata {
	if m == nil {
		return nil
	}
	leaf := unwrapOptional(m)
	if leaf == nil {
		return nil
	}
	if leaf.Kind == metadata.KindAtomic || leaf.Kind == metadata.KindLiteral {
		return leaf
	}
	return nil
}

// unwrapOptional strips union members of `undefined`/`null` to find the
// concrete leaf, e.g. `File | undefined` → `File`.
func unwrapOptional(m *metadata.Metadata) *metadata.Metadata {
	if m == nil {
		return nil
	}
	if m.Kind != metadata.KindUnion {
		return m
	}
	for i := range m.UnionMembers {
		um := &m.UnionMembers[i]
		if um.Kind == metadata.KindAtomic && (um.Atomic == "undefined" || um.Atomic == "null") {
			continue
		}
		if um.Kind == metadata.KindLiteral && um.LiteralValue == nil {
			continue
		}
		return um
	}
	return m
}

// resolveObjectMetadata follows a KindRef to its target in the registry. Returns
// the dereferenced object's metadata, or nil for non-object types.
func resolveObjectMetadata(m *metadata.Metadata, registry *metadata.TypeRegistry) *metadata.Metadata {
	if m == nil {
		return nil
	}
	if m.Kind == metadata.KindObject {
		return m
	}
	if m.Kind == metadata.KindRef && registry != nil {
		if resolved := registry.Types[m.Ref]; resolved != nil && resolved.Kind == metadata.KindObject {
			return resolved
		}
	}
	return nil
}

// mergeFileConstraints overlays per-property constraints onto the leaf-type's
// constraints. The leaf's branded constraints take precedence, but the property
// may add JSDoc constraints; we union them.
func mergeFileConstraints(leaf, prop *metadata.Constraints) *metadata.Constraints {
	if leaf == nil && prop == nil {
		return nil
	}
	out := &metadata.Constraints{}
	if prop != nil {
		*out = *prop
	}
	if leaf != nil {
		if leaf.MaxSize != nil {
			out.MaxSize = leaf.MaxSize
		}
		if leaf.MinSize != nil {
			out.MinSize = leaf.MinSize
		}
		if len(leaf.MimeTypes) > 0 {
			out.MimeTypes = leaf.MimeTypes
		}
		if leaf.MinLength != nil {
			out.MinLength = leaf.MinLength
		}
		if leaf.MaxLength != nil {
			out.MaxLength = leaf.MaxLength
		}
		if leaf.Pattern != nil {
			out.Pattern = leaf.Pattern
		}
		if leaf.Minimum != nil {
			out.Minimum = leaf.Minimum
		}
		if leaf.Maximum != nil {
			out.Maximum = leaf.Maximum
		}
	}
	return out
}

func paramCategoryToKind(category string) (codegen.MintParamKind, bool) {
	switch category {
	case string(analyzer.CategoryBody):
		return codegen.MintParamBody, true
	case string(analyzer.CategoryQuery):
		return codegen.MintParamQuery, true
	case string(analyzer.CategoryParam):
		return codegen.MintParamPathParam, true
	case string(analyzer.CategoryHeaders):
		return codegen.MintParamHeader, true
	}
	return 0, false
}

// companionImportForType returns the relative module specifier to the named
// type's companion file (.tsgonest.js), as seen from the Mint register
// companion file. Empty if the type has no recorded source file (and thus no
// companion will be emitted for it).
func companionImportForType(typeName string, registry *metadata.TypeRegistry, registerCompanionPath string) string {
	outputBase := registry.SourceFile(typeName)
	if outputBase == "" {
		return ""
	}
	dtoCompanion := codegen.CompanionPathForOutput(outputBase, typeName)
	return codegen.RelativeCompanionImport(registerCompanionPath, dtoCompanion)
}

// projectReturn fills the ReturnX fields on a MintRouteInfo from the analyzer's
// return-type metadata.
func projectReturn(route *codegen.MintRouteInfo, ret *metadata.Metadata, registry *metadata.TypeRegistry, registerCompanionPath string) {
	if ret == nil {
		return
	}
	switch ret.Kind {
	case metadata.KindVoid:
		route.ReturnVoid = true
	case metadata.KindAtomic:
		route.ReturnAtomic = ret.Atomic
	case metadata.KindArray:
		if ret.ElementType == nil {
			return
		}
		// Pull the element's named type.
		elemName := elementNamedType(ret.ElementType)
		if elemName == "" {
			return
		}
		if companionImport := companionImportForType(elemName, registry, registerCompanionPath); companionImport != "" {
			route.ReturnTypeName = elemName
			route.ReturnCompanionImport = companionImport
			route.ReturnIsArray = true
		}
	case metadata.KindObject, metadata.KindRef, metadata.KindUnion, metadata.KindIntersection:
		name := namedReturnType(ret)
		if name == "" {
			return
		}
		if companionImport := companionImportForType(name, registry, registerCompanionPath); companionImport != "" {
			route.ReturnTypeName = name
			route.ReturnCompanionImport = companionImport
		}
	}
}

func namedReturnType(m *metadata.Metadata) string {
	if m == nil {
		return ""
	}
	if m.Name != "" {
		return m.Name
	}
	if m.Ref != "" {
		return m.Ref
	}
	return ""
}

func elementNamedType(m *metadata.Metadata) string {
	if m == nil {
		return ""
	}
	if m.Name != "" {
		return m.Name
	}
	if m.Ref != "" {
		return m.Ref
	}
	return ""
}
