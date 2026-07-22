package rewrite

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	shimcompiler "github.com/microsoft/typescript-go/shim/compiler"
	"github.com/tsgonest/tsgonest/internal/analyzer"
	"github.com/tsgonest/tsgonest/internal/pathalias"
)

// DiagnosticSeverity classifies a rewrite diagnostic.
//
// Error means the rewriter was supposed to inject something load-bearing
// (validation, serialization) but couldn't locate the target. Silent no-op
// here = unvalidated input hitting handlers, so the build must fail.
//
// Warning means a recoverable degradation: the file still works correctly
// without the missed injection (e.g. a single method couldn't be located,
// other methods still got their injections).
type DiagnosticSeverity int

const (
	DiagnosticWarning DiagnosticSeverity = iota
	DiagnosticError
)

// RewriteDiagnostic is a single message produced by the rewriter when it
// couldn't apply an intended injection.
type RewriteDiagnostic struct {
	Severity   DiagnosticSeverity
	OutputFile string // emit-time JS path (where the rewrite was attempted)
	Class      string // controller class name
	Method     string // route method name; empty for class-level diagnostics
	Reason     string // human-readable explanation
}

// IsError returns true if the diagnostic should fail the build.
func (d RewriteDiagnostic) IsError() bool {
	return d.Severity == DiagnosticError
}

// Format returns a single-line human-readable form.
func (d RewriteDiagnostic) Format() string {
	prefix := "warning"
	if d.Severity == DiagnosticError {
		prefix = "error"
	}
	target := d.Class
	if d.Method != "" {
		target = d.Class + "." + d.Method
	}
	if target == "" {
		return fmt.Sprintf("%s: %s — %s", prefix, d.OutputFile, d.Reason)
	}
	return fmt.Sprintf("%s: %s (%s) — %s", prefix, d.OutputFile, target, d.Reason)
}

// normalizeSlashes converts all backslashes to forward slashes regardless of OS.
// filepath.ToSlash only converts os.PathSeparator (no-op on Linux for '\').
// tsgo always emits forward-slash paths, but Go's filepath.Join uses OS-native
// separators, so we must normalize unconditionally.
func normalizeSlashes(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}

// RewriteContext holds all data needed by the WriteFile callback to perform
// inline rewrites during emit. All fields are read-only after construction,
// making the callback safe for concurrent use from multiple goroutines.
type RewriteContext struct {
	// CompanionMap maps typeName → companion file absolute path.
	CompanionMap map[string]string

	// MarkerCalls maps source file path → ordered list of marker calls.
	MarkerCalls map[string][]MarkerCall

	// Controllers holds analyzed controller info.
	Controllers []analyzer.ControllerInfo

	// ControllerSourceFiles maps source file paths that are controllers.
	ControllerSourceFiles map[string]bool

	// PathResolver resolves tsconfig path aliases in imports.
	PathResolver *pathalias.PathResolver

	// SourceToOutput maps source file path → output JS path.
	SourceToOutput map[string]string

	// OutputToSource is the reverse map (receives output path in WriteFile).
	OutputToSource map[string]string

	// ModuleFormat is "esm" or "cjs".
	ModuleFormat string

	// HelpersPath is the absolute path to the _tsgonest_helpers.js file.
	// Used for inline scalar coercion imports in controllers.
	HelpersPath string

	// diagMu guards diagnostics. WriteFile callbacks may run concurrently
	// across goroutines during emit, so appends must be synchronized.
	diagMu      sync.Mutex
	diagnostics []RewriteDiagnostic
}

// addDiagnostic appends a diagnostic in a thread-safe way.
func (ctx *RewriteContext) addDiagnostic(d RewriteDiagnostic) {
	ctx.diagMu.Lock()
	ctx.diagnostics = append(ctx.diagnostics, d)
	ctx.diagMu.Unlock()
}

// Diagnostics returns a copy of all diagnostics collected during emit.
// Safe to call after emit completes.
func (ctx *RewriteContext) Diagnostics() []RewriteDiagnostic {
	ctx.diagMu.Lock()
	defer ctx.diagMu.Unlock()
	out := make([]RewriteDiagnostic, len(ctx.diagnostics))
	copy(out, ctx.diagnostics)
	return out
}

// HasErrors returns true if any collected diagnostic has error severity.
func (ctx *RewriteContext) HasErrors() bool {
	ctx.diagMu.Lock()
	defer ctx.diagMu.Unlock()
	for _, d := range ctx.diagnostics {
		if d.Severity == DiagnosticError {
			return true
		}
	}
	return false
}

// MakeWriteFile returns a WriteFile callback that applies all rewrites during emit.
// The callback:
// 1. Resolves path aliases
// 2. Rewrites marker function calls with companion imports
// 3. Injects @Body() validation into controller methods
// 4. Writes the transformed file to disk
func (ctx *RewriteContext) MakeWriteFile() shimcompiler.WriteFile {
	return func(fileName string, text string, data *shimcompiler.WriteFileData) error {
		if strings.HasSuffix(fileName, ".js") {
			// 1. Path alias resolution
			if ctx.PathResolver != nil && ctx.PathResolver.HasAliases() {
				text = ctx.PathResolver.ResolveImports(text, fileName)
			}

			// 2. Marker call rewriting
			sourcePath := ctx.OutputToSource[normalizeSlashes(fileName)]
			if markers, ok := ctx.MarkerCalls[sourcePath]; ok && len(markers) > 0 {
				warn := func(reason string) {
					ctx.addDiagnostic(RewriteDiagnostic{
						Severity:   DiagnosticWarning,
						OutputFile: fileName,
						Reason:     reason,
					})
				}
				text = rewriteMarkers(text, fileName, markers, ctx.CompanionMap, ctx.ModuleFormat, warn)
			}

			// 3. Controller body validation injection
			if ctx.ControllerSourceFiles[sourcePath] {
				// Find controllers matching this source file
				var matchingControllers []analyzer.ControllerInfo
				for _, ctrl := range ctx.Controllers {
					if normalizeSlashes(ctrl.SourceFile) == sourcePath {
						matchingControllers = append(matchingControllers, ctrl)
					}
				}
				if len(matchingControllers) > 0 {
					func() {
						defer func() {
							if r := recover(); r != nil {
								fmt.Fprintf(os.Stderr, "error: controller rewrite failed for %s: %v\n", fileName, r)
								os.Exit(1)
							}
						}()
						text = rewriteController(text, fileName, matchingControllers, ctx.CompanionMap, ctx.ModuleFormat, ctx.addDiagnostic)
					}()
				}
			}
		}
		// Write to disk (replicates default tsgo behavior)
		return writeFileToDisk(fileName, text)
	}
}

// writeFileToDisk writes a file to disk, creating parent directories as needed.
// This replicates the default behavior of tsgo's host.WriteFile.
func writeFileToDisk(fileName string, text string) error {
	dir := filepath.Dir(fileName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(fileName, []byte(text), 0644)
}

// BuildOutputToSourceMap creates the reverse mapping from output file paths
// to source file paths.
func BuildOutputToSourceMap(sourceToOutput map[string]string) map[string]string {
	result := make(map[string]string, len(sourceToOutput))
	for src, out := range sourceToOutput {
		// The sourceToOutput map uses a fake .ts extension for companion path generation.
		// Convert to .js for matching against WriteFile callback paths.
		jsPath := out
		if strings.HasSuffix(jsPath, ".ts") {
			jsPath = jsPath[:len(jsPath)-3] + ".js"
		}
		// Normalize to forward slashes for consistent matching with tsgo's
		// WriteFile callback which always uses forward slashes.
		result[normalizeSlashes(jsPath)] = normalizeSlashes(src)
	}
	return result
}

// BuildControllerSourceFiles creates a set of source file paths that contain controllers.
func BuildControllerSourceFiles(controllers []analyzer.ControllerInfo) map[string]bool {
	result := make(map[string]bool, len(controllers))
	for _, ctrl := range controllers {
		// Normalize to forward slashes for consistent matching with tsgo paths.
		result[normalizeSlashes(ctrl.SourceFile)] = true
	}
	return result
}

// BuildCompanionMap creates a mapping from type name to companion file absolute path.
// It uses the same naming convention as codegen.companionPath.
func BuildCompanionMap(sourceToOutput map[string]string, typesByFile map[string][]string) map[string]string {
	result := make(map[string]string)
	for sourceFile, typeNames := range typesByFile {
		outputBase, ok := sourceToOutput[sourceFile]
		if !ok {
			continue
		}
		for _, typeName := range typeNames {
			// Generate companion path: strip .ts extension, add .TypeName.tsgonest.js
			base := outputBase
			for _, ext := range []string{".ts", ".tsx", ".mts", ".cts"} {
				if strings.HasSuffix(base, ext) {
					base = base[:len(base)-len(ext)]
					break
				}
			}
			companionPath := normalizeSlashes(base) + "." + typeName + ".tsgonest.js"
			result[typeName] = companionPath
		}
	}
	return result
}

// DetectModuleFormat detects whether the output uses ESM or CJS based on
// tsconfig module setting. Returns "esm" or "cjs".
func DetectModuleFormat(moduleKind string) string {
	moduleKind = strings.ToLower(moduleKind)
	switch moduleKind {
	case "commonjs", "cjs":
		return "cjs"
	default:
		// ESM is the default for modern TypeScript/Node.js
		return "esm"
	}
}
