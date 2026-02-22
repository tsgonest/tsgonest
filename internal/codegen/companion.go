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

// GenerateCompanionFiles generates validation and serialization companion files
// for all named types in the registry.
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

		// Generate validation companion (unless ignored)
		if resolved.Ignore != "validation" {
			validateJSPath := companionPath(sourceFileName, typeName, "validate")
			validateContent := GenerateValidation(typeName, resolved, registry)
			files = append(files, CompanionFile{
				Path:    validateJSPath,
				Content: validateContent,
			})

			// Generate type declarations (.d.ts) for the validate companion
			dtsPath := strings.TrimSuffix(validateJSPath, ".js") + ".d.ts"
			dtsContent := GenerateValidationTypes(typeName)
			files = append(files, CompanionFile{
				Path:    dtsPath,
				Content: dtsContent,
			})
		}

		// Generate serialization companion (unless ignored)
		if resolved.Ignore != "serialization" {
			serializeContent := GenerateSerialization(typeName, resolved, registry)
			files = append(files, CompanionFile{
				Path:    companionPath(sourceFileName, typeName, "serialize"),
				Content: serializeContent,
			})
		}
	}

	return files
}

// companionPath generates the companion file path from the source file path.
// e.g., "src/user.dto.ts" + "CreateUserDto" + "validate" → "src/user.dto.CreateUserDto.validate.js"
func companionPath(sourceFileName string, typeName string, suffix string) string {
	// Strip .ts/.tsx extension
	base := sourceFileName
	for _, ext := range []string{".ts", ".tsx", ".mts", ".cts"} {
		if len(base) > len(ext) && base[len(base)-len(ext):] == ext {
			base = base[:len(base)-len(ext)]
			break
		}
	}
	return base + "." + typeName + "." + suffix + ".js"
}
