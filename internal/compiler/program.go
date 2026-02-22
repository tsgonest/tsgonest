package compiler

import (
	"context"
	"errors"
	"fmt"

	"github.com/microsoft/typescript-go/shim/ast"
	shimcompiler "github.com/microsoft/typescript-go/shim/compiler"
	"github.com/microsoft/typescript-go/shim/core"
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
func ParseTSConfig(fs vfs.FS, cwd string, tsconfigPath string, host shimcompiler.CompilerHost) (*tsoptions.ParsedCommandLine, []Diagnostic, error) {
	resolvedConfigPath := tspath.ResolvePath(cwd, tsconfigPath)
	if !fs.FileExists(resolvedConfigPath) {
		return nil, nil, fmt.Errorf("could not find tsconfig at %v", resolvedConfigPath)
	}

	configParseResult, diagnostics := tsoptions.GetParsedCommandLineOfConfigFile(tsconfigPath, &core.CompilerOptions{}, nil, host, nil)

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
	parsedConfig, diags, err := ParseTSConfig(fs, cwd, tsconfigPath, host)
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

// EmitProgram writes the compiled JavaScript output to disk using tsgo's emitter.
func EmitProgram(program *shimcompiler.Program) ([]string, []Diagnostic, error) {
	result := program.Emit(context.Background(), shimcompiler.EmitOptions{})

	var diags []Diagnostic
	if len(result.Diagnostics) > 0 {
		diags = convertDiagnostics(result.Diagnostics)
	}

	return result.EmittedFiles, diags, nil
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
