package compiler

import (
	"context"
	"errors"
	"fmt"

	"github.com/microsoft/typescript-go/shim/ast"
	shimcompiler "github.com/microsoft/typescript-go/shim/compiler"
	"github.com/microsoft/typescript-go/shim/core"
	shimincremental "github.com/microsoft/typescript-go/shim/execute/incremental"
	"github.com/microsoft/typescript-go/shim/tsoptions"
	"github.com/microsoft/typescript-go/shim/tspath"
	"github.com/microsoft/typescript-go/shim/vfs"
)

// Diagnostic represents a compilation diagnostic message.
type Diagnostic struct {
	FilePath string
	Message  string
}

func (d Diagnostic) String() string {
	if d.FilePath != "" {
		return fmt.Sprintf("%s: %s", d.FilePath, d.Message)
	}
	return d.Message
}

// CreateProgramResult contains the program and the parsed tsconfig for downstream use.
type CreateProgramResult struct {
	Program      *shimcompiler.Program
	ParsedConfig *tsoptions.ParsedCommandLine
}

// ParseTSConfig parses a tsconfig.json file using tsgo's native JSONC parser.
// Handles comments, trailing commas, and extends chains automatically.
// Returns the parsed config for inspection/modification before creating a program.
//
// If cliOverrides is non-nil, those compiler options take precedence over tsconfig
// values (same as tsc CLI flags overriding tsconfig.json).
func ParseTSConfig(fs vfs.FS, cwd string, tsconfigPath string, host shimcompiler.CompilerHost, cliOverrides *core.CompilerOptions) (*tsoptions.ParsedCommandLine, []Diagnostic, error) {
	resolvedConfigPath := tspath.ResolvePath(cwd, tsconfigPath)
	if !fs.FileExists(resolvedConfigPath) {
		return nil, nil, fmt.Errorf("could not find tsconfig at %v", resolvedConfigPath)
	}

	if cliOverrides == nil {
		cliOverrides = &core.CompilerOptions{}
	}

	configParseResult, diagnostics := tsoptions.GetParsedCommandLineOfConfigFile(tsconfigPath, cliOverrides, nil, host, nil)

	if len(diagnostics) > 0 {
		return nil, convertDiagnostics(diagnostics), nil
	}

	if configParseResult != nil && len(configParseResult.Errors) > 0 {
		return nil, convertDiagnostics(configParseResult.Errors), nil
	}

	return configParseResult, nil, nil
}

// CreateProgramFromConfig creates a TypeScript program from an already-parsed tsconfig.
// The caller can modify parsedConfig.CompilerOptions() before calling this
// (e.g., to inject an inferred rootDir).
func CreateProgramFromConfig(singleThreaded bool, parsedConfig *tsoptions.ParsedCommandLine, host shimcompiler.CompilerHost) (*shimcompiler.Program, []Diagnostic, error) {
	opts := shimcompiler.ProgramOptions{
		Config:                      parsedConfig,
		SingleThreaded:              core.TSTrue,
		Host:                        host,
		UseSourceOfProjectReference: true,
	}
	if !singleThreaded {
		opts.SingleThreaded = core.TSFalse
	}

	program := shimcompiler.NewProgram(opts)
	if program == nil {
		return nil, nil, errors.New("failed to create program")
	}

	programDiags := program.GetProgramDiagnostics()
	if len(programDiags) > 0 {
		return nil, convertDiagnostics(programDiags), nil
	}

	program.BindSourceFiles()

	return program, nil, nil
}

// CreateProgram creates a TypeScript program from a tsconfig.json file.
// Convenience wrapper that parses config and creates program in one step.
func CreateProgram(singleThreaded bool, fs vfs.FS, cwd string, tsconfigPath string, host shimcompiler.CompilerHost) (*CreateProgramResult, []Diagnostic, error) {
	parsedConfig, diags, err := ParseTSConfig(fs, cwd, tsconfigPath, host, nil)
	if err != nil || len(diags) > 0 {
		return nil, diags, err
	}

	program, programDiags, err := CreateProgramFromConfig(singleThreaded, parsedConfig, host)
	if err != nil || len(programDiags) > 0 {
		return nil, programDiags, err
	}

	return &CreateProgramResult{
		Program:      program,
		ParsedConfig: parsedConfig,
	}, nil, nil
}

// EmitResult wraps tsgo's EmitResult with our additions.
type EmitResult struct {
	EmittedFiles []string
	Diagnostics  []*ast.Diagnostic
	EmitSkipped  bool
}

// EmitProgram writes the compiled JavaScript output to disk using tsgo's emitter.
// Returns the raw EmitResult so callers can check EmitSkipped.
// If writeFile is non-nil, it is used as a custom callback for writing files
// instead of the default host.WriteFile — this enables inline rewriting during emit.
func EmitProgram(program *shimcompiler.Program, writeFile ...shimcompiler.WriteFile) *EmitResult {
	opts := shimcompiler.EmitOptions{}
	if len(writeFile) > 0 && writeFile[0] != nil {
		opts.WriteFile = writeFile[0]
	}
	result := program.Emit(context.Background(), opts)
	return &EmitResult{
		EmittedFiles: result.EmittedFiles,
		Diagnostics:  result.Diagnostics,
		EmitSkipped:  result.EmitSkipped,
	}
}

// GatherDiagnostics collects all diagnostics from a program using tsgo's
// GetDiagnosticsOfAnyProgram — the same cascade tsgo itself uses:
//
//	config → syntactic → program → bind → options → global → semantic → declaration
//
// When noCheck=true, only syntactic diagnostics are collected — this avoids
// creating checkers for all files, which would hang on large projects.
func GatherDiagnostics(program *shimcompiler.Program, noCheck bool) []*ast.Diagnostic {
	ctx := context.Background()

	if noCheck {
		return shimcompiler.Program_GetSyntacticDiagnostics(program, ctx, nil)
	}

	return shimcompiler.GetDiagnosticsOfAnyProgram(
		ctx,
		program,
		nil,   // file=nil → all files
		false, // skipNoEmitCheckForDtsDiagnostics
		func(ctx context.Context, file *ast.SourceFile) []*ast.Diagnostic {
			// Bind diagnostics are gathered by the program's checker internally;
			// we pass a no-op here because BindSourceFiles was already called.
			// tsgo's own EmitFilesAndReportErrors does call GetBindDiagnostics
			// but it's primarily for timing — the bind step already ran.
			return nil
		},
		func(ctx context.Context, file *ast.SourceFile) []*ast.Diagnostic {
			return shimcompiler.Program_GetSemanticDiagnostics(program, ctx, file)
		},
	)
}

// GetSyntacticDiagnostics returns parse errors for all source files.
func GetSyntacticDiagnostics(program *shimcompiler.Program) []*ast.Diagnostic {
	ctx := context.Background()
	return shimcompiler.Program_GetSyntacticDiagnostics(program, ctx, nil)
}

// CreateIncrementalProgram wraps a compiler.Program with incremental state.
// Reads prior state from .tsbuildinfo on disk. For subsequent calls (watch mode),
// pass the previous incremental.Program as oldProgram instead of nil.
func CreateIncrementalProgram(
	program *shimcompiler.Program,
	oldProgram *shimincremental.Program,
	host shimcompiler.CompilerHost,
	parsedConfig *tsoptions.ParsedCommandLine,
) *shimincremental.Program {
	// On first call (or CLI build), read old state from .tsbuildinfo
	if oldProgram == nil {
		reader := shimincremental.NewBuildInfoReader(host)
		oldProgram = shimincremental.ReadBuildInfoProgram(parsedConfig, reader, host)
		// oldProgram may still be nil if no .tsbuildinfo exists — that's OK
	}
	incrHost := shimincremental.CreateHost(host)
	return shimincremental.NewProgram(program, oldProgram, incrHost, false)
}

// EmitIncrementalProgram emits only changed files through the incremental program.
// Also writes the updated .tsbuildinfo file to disk.
// If writeFile is non-nil, it is used as a custom callback for writing files
// instead of the default host.WriteFile — this enables inline rewriting during emit.
func EmitIncrementalProgram(incrProgram *shimincremental.Program, writeFile ...shimcompiler.WriteFile) *EmitResult {
	opts := shimcompiler.EmitOptions{}
	if len(writeFile) > 0 && writeFile[0] != nil {
		opts.WriteFile = writeFile[0]
	}
	result := incrProgram.Emit(context.Background(), opts)
	return &EmitResult{
		EmittedFiles: result.EmittedFiles,
		Diagnostics:  result.Diagnostics,
		EmitSkipped:  result.EmitSkipped,
	}
}

// GatherIncrementalDiagnostics collects diagnostics from an incremental program.
// The incremental program's GetSemanticDiagnostics only checks affected (changed) files,
// returning cached results for unchanged files.
//
// When noCheck=true, only syntactic diagnostics are collected — this avoids
// creating checkers for all files, which would hang on large projects.
func GatherIncrementalDiagnostics(incrProgram *shimincremental.Program, noCheck bool) []*ast.Diagnostic {
	ctx := context.Background()

	if noCheck {
		return incrProgram.GetSyntacticDiagnostics(ctx, nil)
	}

	return shimcompiler.GetDiagnosticsOfAnyProgram(
		ctx,
		incrProgram,
		nil,   // file=nil → all files
		false, // skipNoEmitCheckForDtsDiagnostics
		func(ctx context.Context, file *ast.SourceFile) []*ast.Diagnostic {
			return incrProgram.GetBindDiagnostics(ctx, file)
		},
		func(ctx context.Context, file *ast.SourceFile) []*ast.Diagnostic {
			return incrProgram.GetSemanticDiagnostics(ctx, file)
		},
	)
}

// GetSyntacticDiagnosticsIncremental returns parse errors from an incremental program.
func GetSyntacticDiagnosticsIncremental(incrProgram *shimincremental.Program) []*ast.Diagnostic {
	ctx := context.Background()
	return incrProgram.GetSyntacticDiagnostics(ctx, nil)
}

// GetSourceFiles returns the source files from a program, excluding declaration files.
func GetSourceFiles(program *shimcompiler.Program) []*ast.SourceFile {
	var files []*ast.SourceFile
	for _, f := range program.GetSourceFiles() {
		if !f.IsDeclarationFile {
			files = append(files, f)
		}
	}
	return files
}

// convertDiagnostics converts tsgo diagnostics to our Diagnostic type.
func convertDiagnostics(tsdiags []*ast.Diagnostic) []Diagnostic {
	diags := make([]Diagnostic, len(tsdiags))
	for i, d := range tsdiags {
		var filePath string
		if d.File() != nil {
			filePath = d.File().FileName()
		}
		diags[i] = Diagnostic{
			FilePath: filePath,
			Message:  d.String(),
		}
	}
	return diags
}

// FormatDiagnostics formats diagnostics into human-readable strings.
func FormatDiagnostics(diags []Diagnostic) string {
	var result string
	for _, d := range diags {
		result += d.String() + "\n"
	}
	return result
}
