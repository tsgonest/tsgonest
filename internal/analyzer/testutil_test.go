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
	checker, release := shimcompiler.Program_GetTypeChecker(program, context.Background())
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
				resolvedType := shimchecker.Checker_getTypeFromTypeNode(env.checker, decl.Type)
				return walker.WalkType(resolvedType)
			}
		}
		if stmt.Kind == ast.KindInterfaceDeclaration {
			decl := stmt.AsInterfaceDeclaration()
			if decl.Name().Text() == typeName {
				sym := env.checker.GetSymbolAtLocation(decl.Name())
				if sym != nil {
					resolvedType := shimchecker.Checker_getDeclaredTypeOfSymbol(env.checker, sym)
					return walker.WalkType(resolvedType)
				}
			}
		}
		if stmt.Kind == ast.KindEnumDeclaration {
			decl := stmt.AsEnumDeclaration()
			if decl.Name().Text() == typeName {
				sym := env.checker.GetSymbolAtLocation(decl.Name())
				if sym != nil {
					resolvedType := shimchecker.Checker_getDeclaredTypeOfSymbol(env.checker, sym)
					return walker.WalkType(resolvedType)
				}
			}
		}
		if stmt.Kind == ast.KindClassDeclaration {
			decl := stmt.AsClassDeclaration()
			if decl.Name() != nil && decl.Name().Text() == typeName {
				sym := env.checker.GetSymbolAtLocation(decl.Name())
				if sym != nil {
					resolvedType := shimchecker.Checker_getDeclaredTypeOfSymbol(env.checker, sym)
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
				resolvedType := shimchecker.Checker_getTypeFromTypeNode(env.checker, decl.Type)
				m := walker.WalkType(resolvedType)
				return m, walker.Registry()
			}
		}
		if stmt.Kind == ast.KindInterfaceDeclaration {
			decl := stmt.AsInterfaceDeclaration()
			if decl.Name().Text() == typeName {
				sym := env.checker.GetSymbolAtLocation(decl.Name())
				if sym != nil {
					resolvedType := shimchecker.Checker_getDeclaredTypeOfSymbol(env.checker, sym)
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
					resolvedType := shimchecker.Checker_getDeclaredTypeOfSymbol(env.checker, sym)
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
					resolvedType := shimchecker.Checker_getDeclaredTypeOfSymbol(env.checker, sym)
					m := walker.WalkType(resolvedType)
					return m, walker.Registry()
				}
			}
		}
	}

	t.Fatalf("type %q not found in source file", typeName)
	return metadata.Metadata{}, nil
}
