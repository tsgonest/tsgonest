package codegen

import (
	"strings"

	"github.com/tsgonest/tsgonest/internal/metadata"
)

// CompanionFile represents a generated companion file.
type CompanionFile struct {
	// Path is the output file path (e.g., "dist/user.dto.validate.js").
	Path string
	// Content is the generated JavaScript source.
	Content string
}

// GenerateCompanionFiles generates consolidated companion files (.tsgonest.js)
// for all named types in the registry. Each companion file contains validation,
// assertion, serialization, and Standard Schema functions for the type.
func GenerateCompanionFiles(sourceFileName string, types map[string]*metadata.Metadata, registry *metadata.TypeRegistry) []CompanionFile {
	var files []CompanionFile

	for typeName, meta := range types {
		// Resolve refs through the registry
		resolved := meta
		if meta.Kind == metadata.KindRef && meta.Ref != "" {
			if regType, ok := registry.Types[meta.Ref]; ok {
				resolved = regType
			}
		}

		// Only generate companions for object types (DTOs)
		if resolved.Kind != metadata.KindObject {
			continue
		}

		// Check for @tsgonest-ignore annotations
		if resolved.Ignore == "all" {
			continue
		}

		// Determine which functions to include based on @tsgonest-ignore
		includeValidation := resolved.Ignore != "validation"
		includeSerialization := resolved.Ignore != "serialization"

		// Generate consolidated companion (.tsgonest.js)
		jsPath := companionPath(sourceFileName, typeName)
		jsContent := GenerateCompanionSelective(typeName, resolved, registry, includeValidation, includeSerialization)
		files = append(files, CompanionFile{
			Path:    jsPath,
			Content: jsContent,
		})

		// Generate type declarations (.tsgonest.d.ts)
		dtsPath := strings.TrimSuffix(jsPath, ".js") + ".d.ts"
		dtsContent := GenerateCompanionTypesSelective(typeName, includeValidation, includeSerialization)
		files = append(files, CompanionFile{
			Path:    dtsPath,
			Content: dtsContent,
		})
	}

	return files
}

// companionPath generates the companion file path from the source file path.
// e.g., "src/user.dto.ts" + "CreateUserDto" → "src/user.dto.CreateUserDto.tsgonest.js"
func companionPath(sourceFileName string, typeName string) string {
	// Strip .ts/.tsx extension
	base := sourceFileName
	for _, ext := range []string{".ts", ".tsx", ".mts", ".cts"} {
		if len(base) > len(ext) && base[len(base)-len(ext):] == ext {
			base = base[:len(base)-len(ext)]
			break
		}
	}
	return base + "." + typeName + ".tsgonest.js"
}

// HelpersFilePath returns the path for the shared helpers file given an output directory.
// The helpers file is named _tsgonest_helpers.js and placed in the output directory.
func HelpersFilePath(outDir string) string {
	if outDir == "" {
		return "_tsgonest_helpers.js"
	}
	if outDir[len(outDir)-1] == '/' {
		return outDir + "_tsgonest_helpers.js"
	}
	return outDir + "/_tsgonest_helpers.js"
}

// GenerateHelpersFile returns the shared helpers file (.js and .d.ts) as CompanionFile entries.
// The outDir parameter is the output directory where companion files are written.
// This should be called once per build, not per source file.
func GenerateHelpersFile(outDir string) []CompanionFile {
	jsPath := HelpersFilePath(outDir)
	dtsPath := strings.TrimSuffix(jsPath, ".js") + ".d.ts"
	return []CompanionFile{
		{Path: jsPath, Content: GenerateHelpers()},
		{Path: dtsPath, Content: GenerateHelpersTypes()},
	}
}
