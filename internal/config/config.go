package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Config represents the tsgonest configuration.
type Config struct {
	Controllers ControllersConfig `json:"controllers"`
	Transforms  TransformsConfig  `json:"transforms"`
	OpenAPI     OpenAPIConfig     `json:"openapi"`
	SDK         SDKConfig         `json:"sdk,omitempty"`
	NestJS      NestJSConfig      `json:"nestjs,omitempty"`

	// Dev/build settings (matching nest-cli.json conventions)
	EntryFile     string `json:"entryFile,omitempty"`     // Entry point name without extension (default: "main")
	SourceRoot    string `json:"sourceRoot,omitempty"`    // Source root directory (default: "src")
	DeleteOutDir  bool   `json:"deleteOutDir,omitempty"`  // Delete output directory before build (like --clean)
	ManualRestart bool   `json:"manualRestart,omitempty"` // Enable "rs" manual restart in dev mode
}

// SDKConfig specifies TypeScript SDK generation settings.
type SDKConfig struct {
	Output string `json:"output,omitempty"` // Output directory for generated SDK (default: "./sdk")
	Input  string `json:"input,omitempty"`  // Path to OpenAPI JSON input (defaults to openapi.output)
}

// ControllersConfig specifies which controller files to analyze.
type ControllersConfig struct {
	Include []string `json:"include"`
	Exclude []string `json:"exclude,omitempty"`
}

// TransformsConfig specifies which code transformations to apply.
type TransformsConfig struct {
	Validation        bool     `json:"validation"`
	Serialization     bool     `json:"serialization"`
	StandardSchema    bool     `json:"standardSchema,omitempty"`    // Generate Standard Schema v1 wrappers (default: false)
	ResponseTypeCheck string   `json:"responseTypeCheck,omitempty"` // "safe" (default), "guard", or "none" — controls type checking on response serialization
	Include           []string `json:"include,omitempty"`           // Glob patterns for source files to generate companions for (e.g., ["src/**/*.dto.ts"])
	Exclude           []string `json:"exclude,omitempty"`           // Type name patterns to exclude from codegen (e.g., "Legacy*", "SomeInternalDto")
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
	// Security defines global security requirements applied to all operations.
	// Routes with @public JSDoc opt out. Example: [{"bearer": []}]
	Security []map[string][]string `json:"security,omitempty"`
	// Tags defines tag descriptions for the OpenAPI document.
	// Tags referenced by controllers are auto-collected; this allows adding descriptions.
	Tags []OpenAPITag `json:"tags,omitempty"`
	// TermsOfService is the URL to the API terms of service.
	TermsOfService string `json:"termsOfService,omitempty"`
}

// OpenAPITag represents a tag with an optional description in the OpenAPI document.
type OpenAPITag struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
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
			Validation:        true,
			Serialization:     true,
			ResponseTypeCheck: "safe",
		},
		OpenAPI: OpenAPIConfig{
			Output: "dist/openapi.json",
		},
	}
}

// Discover searches for a tsgonest config file in the given directory.
// Checks in priority order: tsgonest.config.ts > tsgonest.config.json.
// Returns the full path to the config file, or empty string if none found.
func Discover(dir string) string {
	candidates := []string{
		filepath.Join(dir, "tsgonest.config.ts"),
		filepath.Join(dir, "tsgonest.config.json"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// Load reads and parses a tsgonest config file.
// Supports both JSON (.json) and TypeScript (.ts) formats.
// TypeScript configs are evaluated via Node.js to extract the config object.
func Load(path string) (*Config, error) {
	ext := filepath.Ext(path)
	switch ext {
	case ".ts":
		return LoadTS(path)
	case ".json":
		return LoadJSON(path)
	default:
		return nil, fmt.Errorf("unsupported config file extension %q (expected .ts or .json)", ext)
	}
}

// LoadJSON reads and parses a JSON config file.
func LoadJSON(path string) (*Config, error) {
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

// LoadTS evaluates a TypeScript config file via Node.js and parses the result.
//
// The config file is expected to have a default export (e.g., export default defineConfig({...})).
// The function tries multiple Node.js strategies in order:
//  1. node --import tsx (tsx loader — works with any Node.js version)
//  2. node --experimental-strip-types (Node.js 22.6+ built-in TS support)
//
// Falls back to a clear error message if neither works.
func LoadTS(path string) (*Config, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve config path %q: %w", path, err)
	}

	// Node.js eval script: dynamic import + print JSON to stdout
	// Use file:// URL for cross-platform compatibility (Windows paths with backslashes)
	fileURL := "file://" + absPath
	if os.PathSeparator == '\\' {
		// Windows: convert backslashes to forward slashes
		fileURL = "file:///" + strings.ReplaceAll(absPath, "\\", "/")
	}
	evalScript := fmt.Sprintf(
		`import(%q).then(m => {const c = m.default; if (c === undefined || c === null || typeof c !== "object" || Object.keys(c).length === 0) { process.stderr.write("error: config file must have a non-empty default export (export default { ... })\\n"); process.exit(1); } process.stdout.write(JSON.stringify(c));}).catch(e => { process.stderr.write("error: " + e.message + "\\n"); process.exit(1); })`,
		fileURL,
	)

	configDir := filepath.Dir(absPath)

	// Strategy 1: node --import tsx
	jsonData, err := execNode(configDir, []string{"--import", "tsx", "--input-type=module", "-e", evalScript})
	if err != nil {
		// Strategy 2: node --experimental-strip-types (Node.js 22.6+)
		jsonData, err = execNode(configDir, []string{"--experimental-strip-types", "--no-warnings", "--input-type=module", "-e", evalScript})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate TypeScript config %q: %w\nhint: install tsx (npm i -D tsx) or use Node.js 22.6+ for native TypeScript support", path, err)
	}

	config := DefaultConfig()
	if err := json.Unmarshal(jsonData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config from %q: %w", path, err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config in %q: %w", path, err)
	}

	return &config, nil
}

// execNode runs node with the given arguments and returns stdout bytes.
// Returns an error if the command fails or exits non-zero.
func execNode(dir string, args []string) ([]byte, error) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		return nil, fmt.Errorf("node not found in PATH: %w", err)
	}

	cmd := exec.Command(nodePath, args...)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set a timeout to prevent hanging
	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		if err != nil {
			errMsg := strings.TrimSpace(stderr.String())
			if errMsg != "" {
				return nil, fmt.Errorf("%s", errMsg)
			}
			return nil, err
		}
		return stdout.Bytes(), nil
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		return nil, fmt.Errorf("timed out after 10 seconds")
	}
}

// Validate checks the config for logical errors.
func (c *Config) Validate() error {
	if len(c.Controllers.Include) == 0 {
		return fmt.Errorf("controllers.include must have at least one pattern")
	}

	// OpenAPI output is optional — empty string means "no OpenAPI generation".
	// When set, ensure the output path has a .json extension.
	if c.OpenAPI.Output != "" {
		ext := filepath.Ext(c.OpenAPI.Output)
		if ext != ".json" {
			return fmt.Errorf("openapi.output must have a .json extension, got %q", ext)
		}
	}

	// Validate responseTypeCheck
	switch c.Transforms.ResponseTypeCheck {
	case "", "safe", "guard", "none":
		// valid — empty defaults to "safe"
	default:
		return fmt.Errorf("transforms.responseTypeCheck must be one of \"safe\", \"guard\", \"none\", got %q", c.Transforms.ResponseTypeCheck)
	}

	return nil
}
