package codegen

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

// Manifest is the __tsgonest_manifest.json structure.
// It maps DTO type names to their companion file paths and function names.
type Manifest struct {
	Validators  map[string]ManifestEntry `json:"validators"`
	Serializers map[string]ManifestEntry `json:"serializers"`
	Schemas     map[string]ManifestEntry `json:"schemas"`
	Routes      map[string]RouteMapping  `json:"routes,omitempty"`
}

// RouteMapping maps a controller method to its return type name and whether
// it returns an array. The key in the manifest is "ControllerName.methodName".
type RouteMapping struct {
	// ReturnType is the DTO type name (e.g., "UserResponse").
	ReturnType string `json:"returnType"`
	// IsArray indicates the method returns an array of ReturnType.
	IsArray bool `json:"isArray,omitempty"`
}

// ManifestEntry points to a companion file and exported function.
type ManifestEntry struct {
	File string `json:"file"`
	Fn   string `json:"fn"`
}

// GenerateManifest creates a manifest from a list of companion files.
// The outputDir is used to compute relative paths from the manifest location.
// companionFiles are the CompanionFile structs from GenerateCompanionFiles.
// routeMap is an optional map of "ControllerName.methodName" → RouteMapping
// populated from NestJS controller analysis. Pass nil to omit.
func GenerateManifest(companionFiles []CompanionFile, outputDir string, routeMap map[string]RouteMapping) *Manifest {
	m := &Manifest{
		Validators:  make(map[string]ManifestEntry),
		Serializers: make(map[string]ManifestEntry),
		Schemas:     make(map[string]ManifestEntry),
		Routes:      routeMap,
	}

	for _, cf := range companionFiles {
		typeName, category := parseCompanionPath(cf.Path)
		if typeName == "" || category == "" {
			continue
		}

		// Compute relative path from outputDir to the companion file
		relPath, err := filepath.Rel(outputDir, cf.Path)
		if err != nil {
			// Fallback: use the full path
			relPath = cf.Path
		}

		// Normalize to forward slashes for JSON/Node.js compatibility
		relPath = filepath.ToSlash(relPath)
		// Prefix with ./ if not already
		if !strings.HasPrefix(relPath, "./") && !strings.HasPrefix(relPath, "../") {
			relPath = "./" + relPath
		}

		switch category {
		case "validate":
			m.Validators[typeName] = ManifestEntry{
				File: relPath,
				Fn:   "assert" + typeName,
			}
			// Also add Standard Schema entry from same file
			m.Schemas[typeName] = ManifestEntry{
				File: relPath,
				Fn:   "schema" + typeName,
			}
		case "serialize":
			m.Serializers[typeName] = ManifestEntry{
				File: relPath,
				Fn:   "serialize" + typeName,
			}
		}
	}

	return m
}

// ManifestJSON serializes the manifest to pretty-printed JSON.
func ManifestJSON(m *Manifest) ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

// parseCompanionPath extracts the type name and category from a companion file path.
// e.g., "dist/user.dto.CreateUserDto.validate.js" → ("CreateUserDto", "validate")
// e.g., "dist/user.dto.UserResponse.serialize.js" → ("UserResponse", "serialize")
func parseCompanionPath(path string) (typeName string, category string) {
	base := filepath.Base(path)

	// Strip .js extension
	if !strings.HasSuffix(base, ".js") {
		return "", ""
	}
	base = base[:len(base)-3]

	// Split by dots: [..., TypeName, category]
	parts := strings.Split(base, ".")
	if len(parts) < 3 {
		return "", ""
	}

	category = parts[len(parts)-1]
	typeName = parts[len(parts)-2]

	if category != "validate" && category != "serialize" {
		return "", ""
	}

	return typeName, category
}
