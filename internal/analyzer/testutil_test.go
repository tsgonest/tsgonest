package analyzer_test

import (
	"context"
	"path"
	"runtime"
	"testing"

	"github.com/microsoft/typescript-go/shim/ast"
	"github.com/microsoft/typescript-go/shim/bundled"
	shimchecker "github.com/microsoft/typescript-go/shim/checker"
	shimcompiler "github.com/microsoft/typescript-go/shim/compiler"
	"github.com/microsoft/typescript-go/shim/core"
	"github.com/microsoft/typescript-go/shim/tsoptions"
	"github.com/microsoft/typescript-go/shim/tspath"
	"github.com/tsgonest/tsgonest/internal/analyzer"
	"github.com/tsgonest/tsgonest/internal/metadata"
	"github.com/tsgonest/tsgonest/internal/testutil"
)

// walkerTestDir returns the absolute path to testdata/walker/.
func walkerTestDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return path.Join(path.Dir(filename), "..", "..", "testdata", "walker")
}

// walkerEnv holds a tsgo program, checker, and source file for type walker tests.
type walkerEnv struct {
	program    *shimcompiler.Program
	checker    *shimchecker.Checker
	sourceFile *ast.SourceFile
	release    func()
}

// setupWalker creates a tsgo program from inline TypeScript source code,
// obtains the type checker, and returns the environment for testing.
// The caller must call env.release() when done.
func setupWalker(t *testing.T, tsSource string) *walkerEnv {
	t.Helper()

	rootDir := walkerTestDir()
	fileName := "test.ts"
	filePath := tspath.ResolvePath(rootDir, fileName)

	// Create virtual filesystem with the inline source
	virtualFiles := map[string]string{
		filePath: tsSource,
	}
	fs := testutil.NewDefaultOverlayVFS(virtualFiles)
	host := shimcompiler.NewCompilerHost(rootDir, fs, bundled.LibPath(), nil, nil)

	// Parse tsconfig
	configParseResult, diags := tsoptions.GetParsedCommandLineOfConfigFile(
		"tsconfig.json", &core.CompilerOptions{}, nil, host, nil,
	)
	if len(diags) > 0 {
		t.Fatalf("tsconfig parse errors: %v", diags[0].String())
	}

	// Create program
	program := shimcompiler.NewProgram(shimcompiler.ProgramOptions{
		Config:                      configParseResult,
		SingleThreaded:              core.TSTrue,
		Host:                        host,
		UseSourceOfProjectReference: true,
	})
	if program == nil {
		t.Fatal("failed to create program")
	}
	program.BindSourceFiles()

	// Get the source file
	sourceFile := program.GetSourceFile(fileName)
	if sourceFile == nil {
		t.Fatalf("source file %q not found in program", fileName)
	}

	// Get the type checker
	checker, release := program.GetTypeChecker(context.Background())
	if checker == nil {
		t.Fatal("failed to get type checker")
	}

	return &walkerEnv{
		program:    program,
		checker:    checker,
		sourceFile: sourceFile,
		release:    release,
	}
}

// setupWalkerMultiFile creates a tsgo program from multiple virtual files.
// files is a map of relative file names (e.g., "decorators.ts", "controller.ts") to their content.
// mainFile is the file name to return as the primary source file for analysis.
func setupWalkerMultiFile(t *testing.T, files map[string]string, mainFile string) *walkerEnv {
	t.Helper()

	rootDir := walkerTestDir()

	virtualFiles := make(map[string]string)
	for name, content := range files {
		virtualFiles[tspath.ResolvePath(rootDir, name)] = content
	}
	fs := testutil.NewDefaultOverlayVFS(virtualFiles)
	host := shimcompiler.NewCompilerHost(rootDir, fs, bundled.LibPath(), nil, nil)

	configParseResult, diags := tsoptions.GetParsedCommandLineOfConfigFile(
		"tsconfig.json", &core.CompilerOptions{}, nil, host, nil,
	)
	if len(diags) > 0 {
		t.Fatalf("tsconfig parse errors: %v", diags[0].String())
	}

	program := shimcompiler.NewProgram(shimcompiler.ProgramOptions{
		Config:                      configParseResult,
		SingleThreaded:              core.TSTrue,
		Host:                        host,
		UseSourceOfProjectReference: true,
	})
	if program == nil {
		t.Fatal("failed to create program")
	}
	program.BindSourceFiles()

	sourceFile := program.GetSourceFile(mainFile)
	if sourceFile == nil {
		t.Fatalf("source file %q not found in program", mainFile)
	}

	checker, release := program.GetTypeChecker(context.Background())
	if checker == nil {
		t.Fatal("failed to get type checker")
	}

	return &walkerEnv{
		program:    program,
		checker:    checker,
		sourceFile: sourceFile,
		release:    release,
	}
}

// walkExportedType looks up an exported type alias by name in the source file
// and walks it through the TypeWalker, returning the resulting Metadata.
func (env *walkerEnv) walkExportedType(t *testing.T, typeName string) metadata.Metadata {
	t.Helper()

	walker := analyzer.NewTypeWalker(env.checker)

	// Walk all top-level statements looking for the type alias
	for _, stmt := range env.sourceFile.Statements.Nodes {
		if stmt.Kind == ast.KindTypeAliasDeclaration {
			decl := stmt.AsTypeAliasDeclaration()
			if decl.Name().Text() == typeName {
				resolvedType := env.checker.GetTypeFromTypeNode(decl.Type)
				return walker.WalkType(resolvedType)
			}
		}
		if stmt.Kind == ast.KindInterfaceDeclaration {
			decl := stmt.AsInterfaceDeclaration()
			if decl.Name().Text() == typeName {
				sym := env.checker.GetSymbolAtLocation(decl.Name())
				if sym != nil {
					resolvedType := env.checker.GetDeclaredTypeOfSymbol(sym)
					return walker.WalkType(resolvedType)
				}
			}
		}
		if stmt.Kind == ast.KindEnumDeclaration {
			decl := stmt.AsEnumDeclaration()
			if decl.Name().Text() == typeName {
				sym := env.checker.GetSymbolAtLocation(decl.Name())
				if sym != nil {
					resolvedType := env.checker.GetDeclaredTypeOfSymbol(sym)
					return walker.WalkType(resolvedType)
				}
			}
		}
		if stmt.Kind == ast.KindClassDeclaration {
			decl := stmt.AsClassDeclaration()
			if decl.Name() != nil && decl.Name().Text() == typeName {
				sym := env.checker.GetSymbolAtLocation(decl.Name())
				if sym != nil {
					resolvedType := env.checker.GetDeclaredTypeOfSymbol(sym)
					return walker.WalkType(resolvedType)
				}
			}
		}
	}

	t.Fatalf("type %q not found in source file", typeName)
	return metadata.Metadata{}
}

// walkExportedTypeWithRegistryOnly walks the type and returns only the registry.
func (env *walkerEnv) walkExportedTypeWithRegistryOnly(t *testing.T, typeName string) *metadata.TypeRegistry {
	t.Helper()
	_, reg := env.walkExportedTypeWithRegistry(t, typeName)
	return reg
}

// walkExportedTypeWithRegistry is like walkExportedType but also returns the TypeRegistry.
func (env *walkerEnv) walkExportedTypeWithRegistry(t *testing.T, typeName string) (metadata.Metadata, *metadata.TypeRegistry) {
	t.Helper()

	walker := analyzer.NewTypeWalker(env.checker)

	for _, stmt := range env.sourceFile.Statements.Nodes {
		if stmt.Kind == ast.KindTypeAliasDeclaration {
			decl := stmt.AsTypeAliasDeclaration()
			if decl.Name().Text() == typeName {
				resolvedType := env.checker.GetTypeFromTypeNode(decl.Type)
				m := walker.WalkType(resolvedType)
				return m, walker.Registry()
			}
		}
		if stmt.Kind == ast.KindInterfaceDeclaration {
			decl := stmt.AsInterfaceDeclaration()
			if decl.Name().Text() == typeName {
				sym := env.checker.GetSymbolAtLocation(decl.Name())
				if sym != nil {
					resolvedType := env.checker.GetDeclaredTypeOfSymbol(sym)
					m := walker.WalkType(resolvedType)
					return m, walker.Registry()
				}
			}
		}
		if stmt.Kind == ast.KindEnumDeclaration {
			decl := stmt.AsEnumDeclaration()
			if decl.Name().Text() == typeName {
				sym := env.checker.GetSymbolAtLocation(decl.Name())
				if sym != nil {
					resolvedType := env.checker.GetDeclaredTypeOfSymbol(sym)
					m := walker.WalkType(resolvedType)
					return m, walker.Registry()
				}
			}
		}
		if stmt.Kind == ast.KindClassDeclaration {
			decl := stmt.AsClassDeclaration()
			if decl.Name() != nil && decl.Name().Text() == typeName {
				sym := env.checker.GetSymbolAtLocation(decl.Name())
				if sym != nil {
					resolvedType := env.checker.GetDeclaredTypeOfSymbol(sym)
					m := walker.WalkType(resolvedType)
					return m, walker.Registry()
				}
			}
		}
	}

	t.Fatalf("type %q not found in source file", typeName)
	return metadata.Metadata{}, nil
}

// walkAliasType walks a top-level type alias via its referencing AST position
// to mirror production parameter/return-type walks (which call WalkTypeNode on
// the type annotation node). Requires the source file to contain a probe
// declaration `declare const __probe: <typeName>;` so the helper can locate an
// AST type node that references the alias by name.
func (env *walkerEnv) walkAliasType(t *testing.T, typeName string) metadata.Metadata {
	t.Helper()
	return env.walkProbeType(t, "__probe", typeName)
}

// walkProbeType locates `declare const <probeName>: <expectedAlias>;` in the
// source file and walks the variable's type annotation via WalkTypeNode. This
// exercises the same AST-driven code path used in production. expectedAlias is
// optional — pass "" to skip the alias-name assertion.
func (env *walkerEnv) walkProbeType(t *testing.T, probeName string, expectedAlias string) metadata.Metadata {
	t.Helper()

	walker := analyzer.NewTypeWalker(env.checker)

	for _, stmt := range env.sourceFile.Statements.Nodes {
		if stmt == nil || stmt.Kind != ast.KindVariableStatement {
			continue
		}
		vs := stmt.AsVariableStatement()
		if vs == nil || vs.DeclarationList == nil {
			continue
		}
		declList := vs.DeclarationList.AsVariableDeclarationList()
		if declList == nil {
			continue
		}
		for _, declNode := range declList.Declarations.Nodes {
			if declNode == nil || declNode.Kind != ast.KindVariableDeclaration {
				continue
			}
			decl := declNode.AsVariableDeclaration()
			if decl == nil || decl.Name() == nil {
				continue
			}
			if decl.Name().Text() != probeName {
				continue
			}
			typeNode := decl.Type
			if typeNode == nil {
				t.Fatalf("probe %q has no type annotation", probeName)
			}
			if expectedAlias != "" {
				if typeNode.Kind != ast.KindTypeReference {
					t.Fatalf("probe %q type annotation is not a TypeReference (kind=%v)", probeName, typeNode.Kind)
				}
				ref := typeNode.AsTypeReferenceNode()
				if ref == nil || ref.TypeName == nil || ref.TypeName.Text() != expectedAlias {
					t.Fatalf("probe %q expected to reference %q", probeName, expectedAlias)
				}
			}
			return walker.WalkTypeNode(typeNode)
		}
	}

	t.Fatalf("probe variable %q not found in source file", probeName)
	return metadata.Metadata{}
}

// walkAllNamedTypes walks all type aliases in the source file using WalkNamedType
// (matching production behavior in generateCompanionsInMemory), then returns the
// walker for inspection. This is critical for testing sub-field type registration.
func (env *walkerEnv) walkAllNamedTypes(t *testing.T) *analyzer.TypeWalker {
	t.Helper()

	walker := analyzer.NewTypeWalker(env.checker)

	for _, stmt := range env.sourceFile.Statements.Nodes {
		if stmt.Kind == ast.KindTypeAliasDeclaration {
			decl := stmt.AsTypeAliasDeclaration()
			// Skip generic type aliases (matching build.go pre-registration pass)
			if decl.TypeParameters != nil {
				continue
			}
			name := decl.Name().Text()
			resolvedType := env.checker.GetTypeFromTypeNode(decl.Type)
			walker.WalkNamedType(name, resolvedType)
		}
		if stmt.Kind == ast.KindInterfaceDeclaration {
			decl := stmt.AsInterfaceDeclaration()
			// Skip generic interfaces (matching production behavior: only concrete
			// instantiations are walked, not generic templates with type parameters).
			if decl.TypeParameters != nil {
				continue
			}
			sym := env.checker.GetSymbolAtLocation(decl.Name())
			if sym != nil {
				resolvedType := env.checker.GetDeclaredTypeOfSymbol(sym)
				walker.WalkType(resolvedType)
			}
		}
	}

	return walker
}
