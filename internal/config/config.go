package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config represents the tsgonest configuration.
type Config struct {
	Controllers ControllersConfig `json:"controllers"`
	Transforms  TransformsConfig  `json:"transforms"`
	OpenAPI     OpenAPIConfig     `json:"openapi"`
	NestJS      NestJSConfig      `json:"nestjs,omitempty"`

	// Dev/build settings (matching nest-cli.json conventions)
	EntryFile     string `json:"entryFile,omitempty"`     // Entry point name without extension (default: "main")
	SourceRoot    string `json:"sourceRoot,omitempty"`    // Source root directory (default: "src")
	DeleteOutDir  bool   `json:"deleteOutDir,omitempty"`  // Delete output directory before build (like --clean)
	ManualRestart bool   `json:"manualRestart,omitempty"` // Enable "rs" manual restart in dev mode
}

// ControllersConfig specifies which controller files to analyze.
type ControllersConfig struct {
	Include []string `json:"include"`
	Exclude []string `json:"exclude,omitempty"`
}

// TransformsConfig specifies which code transformations to apply.
type TransformsConfig struct {
	Validation    bool     `json:"validation"`
	Serialization bool     `json:"serialization"`
	Include       []string `json:"include,omitempty"` // Glob patterns for source files to generate companions for (e.g., ["src/**/*.dto.ts"])
	Exclude       []string `json:"exclude,omitempty"` // Type name patterns to exclude from codegen (e.g., "Legacy*", "SomeInternalDto")
}

// OpenAPIConfig specifies OpenAPI generation settings.
type OpenAPIConfig struct {
	Output          string                           `json:"output"`
	Title           string                           `json:"title,omitempty"`
	Description     string                           `json:"description,omitempty"`
	Version         string                           `json:"version,omitempty"`
	Contact         *OpenAPIContact                  `json:"contact,omitempty"`
	License         *OpenAPILicense                  `json:"license,omitempty"`
	Servers         []OpenAPIServer                  `json:"servers,omitempty"`
	SecuritySchemes map[string]OpenAPISecurityScheme `json:"securitySchemes,omitempty"`
}

// OpenAPIContact holds contact info for the OpenAPI document.
type OpenAPIContact struct {
	Name  string `json:"name,omitempty"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

// OpenAPILicense holds license info for the OpenAPI document.
type OpenAPILicense struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// OpenAPIServer represents an API server in the OpenAPI document.
type OpenAPIServer struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

// OpenAPISecurityScheme represents a security scheme in the OpenAPI document.
type OpenAPISecurityScheme struct {
	Type         string `json:"type"`
	Scheme       string `json:"scheme,omitempty"`
	BearerFormat string `json:"bearerFormat,omitempty"`
	In           string `json:"in,omitempty"`
	Name         string `json:"name,omitempty"`
	Description  string `json:"description,omitempty"`
}

// NestJSConfig specifies NestJS-specific settings like global prefix and versioning.
type NestJSConfig struct {
	GlobalPrefix string            `json:"globalPrefix,omitempty"`
	Versioning   *VersioningConfig `json:"versioning,omitempty"`
}

// VersioningConfig specifies API versioning settings.
type VersioningConfig struct {
	Type           string `json:"type"`                     // "URI" (default), "HEADER", "MEDIA_TYPE", "CUSTOM"
	DefaultVersion string `json:"defaultVersion,omitempty"` // e.g., "1"
	Prefix         string `json:"prefix,omitempty"`         // default "v" for URI versioning
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Controllers: ControllersConfig{
			Include: []string{"src/**/*.controller.ts"},
		},
		Transforms: TransformsConfig{
			Validation:    true,
			Serialization: true,
		},
		OpenAPI: OpenAPIConfig{
			Output: "dist/openapi.json",
		},
	}
}

// Load reads and parses a tsgonest config file.
// Currently supports JSON format only. TypeScript config support will be added later.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %q: %w", path, err)
	}

	config := DefaultConfig()
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %q: %w", path, err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config in %q: %w", path, err)
	}

	return &config, nil
}

// Validate checks the config for logical errors.
func (c *Config) Validate() error {
	if len(c.Controllers.Include) == 0 {
		return fmt.Errorf("controllers.include must have at least one pattern")
	}

	if c.OpenAPI.Output == "" {
		return fmt.Errorf("openapi.output must not be empty")
	}

	// Ensure the output path has a .json extension
	ext := filepath.Ext(c.OpenAPI.Output)
	if ext != ".json" {
		return fmt.Errorf("openapi.output must have a .json extension, got %q", ext)
	}

	return nil
}
