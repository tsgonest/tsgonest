package rewrite

import (
	"os"
	"path/filepath"
	"strings"

	shimcompiler "github.com/microsoft/typescript-go/shim/compiler"
	"github.com/tsgonest/tsgonest/internal/analyzer"
	"github.com/tsgonest/tsgonest/internal/pathalias"
)

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
}

// MakeWriteFile returns a WriteFile callback that applies all rewrites during emit.
// The callback:
// 1. Resolves path aliases
// 2. Rewrites marker function calls with companion imports
// 3. Injects @Body() validation into controller methods
// 4. Writes the transformed file to disk
func (ctx *RewriteContext) MakeWriteFile() shimcompiler.WriteFile {
	return func(fileName string, text string, bom bool, data *shimcompiler.WriteFileData) error {
		if strings.HasSuffix(fileName, ".js") {
			// 1. Path alias resolution
			if ctx.PathResolver != nil && ctx.PathResolver.HasAliases() {
				text = ctx.PathResolver.ResolveImports(text, fileName)
			}

			// 2. Marker call rewriting
			sourcePath := ctx.OutputToSource[fileName]
			if markers, ok := ctx.MarkerCalls[sourcePath]; ok && len(markers) > 0 {
				text = rewriteMarkers(text, fileName, markers, ctx.CompanionMap, ctx.ModuleFormat)
			}

			// 3. Controller body validation injection
			if ctx.ControllerSourceFiles[sourcePath] {
				// Find controllers matching this source file
				var matchingControllers []analyzer.ControllerInfo
				for _, ctrl := range ctx.Controllers {
					if ctrl.SourceFile == sourcePath {
						matchingControllers = append(matchingControllers, ctrl)
					}
				}
				if len(matchingControllers) > 0 {
					text = rewriteController(text, fileName, matchingControllers, ctx.CompanionMap, ctx.ModuleFormat)
				}
			}
		}

		// Write to disk (replicates default tsgo behavior)
		return writeFileToDisk(fileName, text, bom)
	}
}

// writeFileToDisk writes a file to disk, creating parent directories as needed.
// This replicates the default behavior of tsgo's host.WriteFile.
func writeFileToDisk(fileName string, text string, writeByteOrderMark bool) error {
	dir := filepath.Dir(fileName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	content := text
	if writeByteOrderMark {
		content = "\xEF\xBB\xBF" + content
	}

	return os.WriteFile(fileName, []byte(content), 0644)
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
		result[jsPath] = src
	}
	return result
}

// BuildControllerSourceFiles creates a set of source file paths that contain controllers.
func BuildControllerSourceFiles(controllers []analyzer.ControllerInfo) map[string]bool {
	result := make(map[string]bool, len(controllers))
	for _, ctrl := range controllers {
		result[ctrl.SourceFile] = true
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
			companionPath := base + "." + typeName + ".tsgonest.js"
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
