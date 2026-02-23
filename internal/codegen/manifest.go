package codegen

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

// Manifest is the __tsgonest_manifest.json structure.
// It maps DTO type names to their companion file paths and function names.
type Manifest struct {
	Version    int                          `json:"version"`
	Companions map[string]CompanionManifest `json:"companions"`
	Routes     map[string]RouteMapping      `json:"routes,omitempty"`
}

// CompanionManifest holds all function references for a single type's companion file.
type CompanionManifest struct {
	File      string `json:"file"`
	Validate  string `json:"validate"`
	Assert    string `json:"assert"`
	Serialize string `json:"serialize"`
	Schema    string `json:"schema"`
}

// RouteMapping maps a controller method to its return type name and whether
// it returns an array. The key in the manifest is "ControllerName.methodName".
type RouteMapping struct {
	// ReturnType is the DTO type name (e.g., "UserResponse").
	ReturnType string `json:"returnType"`
	// IsArray indicates the method returns an array of ReturnType.
	IsArray bool `json:"isArray,omitempty"`
}

// GenerateManifest creates a manifest from a list of companion files.
// The outputDir is used to compute relative paths from the manifest location.
// companionFiles are the CompanionFile structs from GenerateCompanionFiles.
// routeMap is an optional map of "ControllerName.methodName" → RouteMapping
// populated from NestJS controller analysis. Pass nil to omit.
func GenerateManifest(companionFiles []CompanionFile, outputDir string, routeMap map[string]RouteMapping) *Manifest {
	m := &Manifest{
		Version:    1,
		Companions: make(map[string]CompanionManifest),
		Routes:     routeMap,
	}

	for _, cf := range companionFiles {
		typeName := parseCompanionPath(cf.Path)
		if typeName == "" {
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

		m.Companions[typeName] = CompanionManifest{
			File:      relPath,
			Validate:  "validate" + typeName,
			Assert:    "assert" + typeName,
			Serialize: "serialize" + typeName,
			Schema:    "schema" + typeName,
		}
	}

	return m
}

// ManifestJSON serializes the manifest to pretty-printed JSON.
func ManifestJSON(m *Manifest) ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

// parseCompanionPath extracts the type name from a consolidated companion file path.
// e.g., "dist/user.dto.CreateUserDto.tsgonest.js" → "CreateUserDto"
// Only processes .tsgonest.js files, skips .tsgonest.d.ts files.
func parseCompanionPath(path string) string {
	base := filepath.Base(path)

	// Only process .tsgonest.js files (not .d.ts)
	if !strings.HasSuffix(base, ".tsgonest.js") {
		return ""
	}
	// Strip .tsgonest.js suffix
	base = base[:len(base)-len(".tsgonest.js")]

	// Split by dots: [..., TypeName]
	parts := strings.Split(base, ".")
	if len(parts) < 2 {
		return ""
	}

	// TypeName is the last segment
	return parts[len(parts)-1]
}
