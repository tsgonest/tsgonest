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

	// OpenAPI is the legacy single-output config, populated from the first element
	// of OpenAPIOutputs for backward compatibility. Not directly JSON-deserialized.
	OpenAPI OpenAPIConfig `json:"-"`

	// OpenAPIOutputs is the resolved list of OpenAPI output configurations.
	// Populated by resolveOpenAPI() from OpenAPIRaw after JSON unmarshaling.
	OpenAPIOutputs []OpenAPIOutputConfig `json:"-"`

	// OpenAPIRaw holds the raw JSON for the "openapi" field.
	// Can be a single object or an array. Resolved by resolveOpenAPI().
	OpenAPIRaw json.RawMessage `json:"openapi,omitempty"`

	SDK    SDKConfig    `json:"sdk,omitempty"`
	NestJS NestJSConfig `json:"nestjs,omitempty"`

	// Dev/build settings (matching nest-cli.json conventions)
	EntryFile     string `json:"entryFile,omitempty"`     // Entry point name without extension (default: "main")
	SourceRoot    string `json:"sourceRoot,omitempty"`    // Source root directory (default: "src")
	Runtime       string `json:"runtime,omitempty"`       // Runtime to use for dev command: "node" (default) or "bun"
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
	Validation         bool     `json:"validation"`
	Serialization      bool     `json:"serialization"`
	StandardSchema     bool     `json:"standardSchema,omitempty"`     // Generate Standard Schema v1 wrappers (default: false)
	ResponseSerializer string   `json:"responseSerializer,omitempty"` // "guard" (default), "safe", or "none" — controls type checking on response serialization
	Include            []string `json:"include,omitempty"`            // Glob patterns for source files to generate companions for (e.g., ["src/**/*.dto.ts"])
	Exclude            []string `json:"exclude,omitempty"`            // Type name patterns to exclude from codegen (e.g., "Legacy*", "SomeInternalDto")
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

// OpenAPIOutputConfig represents a single OpenAPI output specification.
// Used in both single-output and multi-output modes.
type OpenAPIOutputConfig struct {
	// Name is the logical name for this output (e.g., "public", "internal").
	// Optional for single-output; useful for --name flag in multi-output mode.
	Name string `json:"name,omitempty"`

	// Output path for the generated OpenAPI document.
	Output string `json:"output"`

	// Controllers overrides the top-level controllers.include/exclude for this output.
	// When nil, inherits from the top-level controllers config.
	Controllers *ControllersConfig `json:"controllers,omitempty"`

	// IncludeTags keeps only routes matching at least one of these tags.
	IncludeTags []string `json:"includeTags,omitempty"`
	// ExcludeTags removes routes matching any of these tags.
	ExcludeTags []string `json:"excludeTags,omitempty"`

	// SDK configures per-output SDK generation. When set, SDK is generated
	// from this output's OpenAPI spec.
	SDK *SDKOutputConfig `json:"sdk,omitempty"`

	// Document metadata (same fields as OpenAPIConfig).
	Title           string                           `json:"title,omitempty"`
	Description     string                           `json:"description,omitempty"`
	Version         string                           `json:"version,omitempty"`
	Contact         *OpenAPIContact                  `json:"contact,omitempty"`
	License         *OpenAPILicense                  `json:"license,omitempty"`
	Servers         []OpenAPIServer                  `json:"servers,omitempty"`
	SecuritySchemes map[string]OpenAPISecurityScheme `json:"securitySchemes,omitempty"`
	Security        []map[string][]string            `json:"security,omitempty"`
	Tags            []OpenAPITag                     `json:"tags,omitempty"`
	TermsOfService  string                           `json:"termsOfService,omitempty"`
}

// SDKOutputConfig specifies per-output SDK generation settings.
type SDKOutputConfig struct {
	Output  string `json:"output"`            // SDK output directory for this OpenAPI output
	TSEnums bool   `json:"tsEnums,omitempty"` // Emit TypeScript enum declarations instead of union types
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
// When loading from JSON/TS, resolveOpenAPI() overrides OpenAPI/OpenAPIOutputs
// from the raw JSON. For non-loading use cases, defaults are pre-populated.
func DefaultConfig() Config {
	defaultOutput := OpenAPIOutputConfig{Output: "dist/openapi.json"}
	return Config{
		Controllers: ControllersConfig{
			Include: []string{"src/**/*.controller.ts"},
		},
		Transforms: TransformsConfig{
			Validation:         true,
			Serialization:      true,
			ResponseSerializer: "guard",
		},
		OpenAPI:        outputToLegacyConfig(defaultOutput),
		OpenAPIOutputs: []OpenAPIOutputConfig{defaultOutput},
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

	if err := config.resolveOpenAPI(); err != nil {
		return nil, fmt.Errorf("invalid config in %q: %w", path, err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config in %q: %w", path, err)
	}

	return &config, nil
}

// LoadTS evaluates a TypeScript config file via Node.js (or Bun) and parses the result.
//
// The config file is expected to have a default export (e.g., export default defineConfig({...})).
// The function tries multiple strategies in order:
//  1. bun -e (if Bun is available — runs TS natively, no loaders needed)
//  2. node --import tsx (tsx loader — works with any Node.js version)
//  3. node --experimental-strip-types (Node.js 22.6+ built-in TS support)
//
// Falls back to a clear error message if none work.
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
		`import(%q).then(m => {`+
			`let c = m.default;`+
			// Unwrap double-default: tsx can wrap defineConfig() exports as { default: { ...config } }
			`if (c && typeof c === "object" && "default" in c && typeof c.default === "object" && c.default !== null) { c = c.default; }`+
			`if (c === undefined || c === null || typeof c !== "object" || Object.keys(c).length === 0) {`+
			`process.stderr.write("error: config file must have a non-empty default export (export default { ... })\\n"); process.exit(1); }`+
			`process.stdout.write(JSON.stringify(c));`+
			`}).catch(e => { process.stderr.write("error: " + e.message + "\\n"); process.exit(1); })`,
		fileURL,
	)

	configDir := filepath.Dir(absPath)

	// Strategy 1: bun -e (Bun runs TS natively, no loaders needed)
	jsonData, err := execRuntime("bun", configDir, []string{"-e", evalScript})
	if err != nil {
		// Strategy 2: node --import tsx
		jsonData, err = execRuntime("node", configDir, []string{"--import", "tsx", "--input-type=module", "-e", evalScript})
	}
	if err != nil {
		// Strategy 3: node --experimental-strip-types (Node.js 22.6+)
		jsonData, err = execRuntime("node", configDir, []string{"--experimental-strip-types", "--no-warnings", "--input-type=module", "-e", evalScript})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate TypeScript config %q: %w\nhint: install tsx (npm i -D tsx), use Node.js 22.6+, or install Bun for native TypeScript support", path, err)
	}

	config := DefaultConfig()
	if err := json.Unmarshal(jsonData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config from %q: %w", path, err)
	}

	if err := config.resolveOpenAPI(); err != nil {
		return nil, fmt.Errorf("invalid config in %q: %w", path, err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config in %q: %w", path, err)
	}

	return &config, nil
}

// execRuntime runs a runtime binary (node, bun) with the given arguments and returns stdout bytes.
// Returns an error if the command fails or exits non-zero.
func execRuntime(runtime string, dir string, args []string) ([]byte, error) {
	binPath, err := exec.LookPath(runtime)
	if err != nil {
		return nil, fmt.Errorf("%s not found in PATH: %w", runtime, err)
	}

	cmd := exec.Command(binPath, args...)
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

// resolveOpenAPI parses the raw JSON "openapi" field into OpenAPIOutputs.
// Supports both a single object (backward-compatible) and an array of outputs.
// Must be called after JSON unmarshaling and before Validate().
func (c *Config) resolveOpenAPI() error {
	if len(c.OpenAPIRaw) == 0 || string(c.OpenAPIRaw) == "null" {
		// No openapi specified — use default single output
		c.OpenAPIOutputs = []OpenAPIOutputConfig{{Output: "dist/openapi.json"}}
		c.OpenAPI = outputToLegacyConfig(c.OpenAPIOutputs[0])
		return nil
	}

	trimmed := bytes.TrimSpace(c.OpenAPIRaw)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		// Array mode: multiple outputs
		var outputs []OpenAPIOutputConfig
		if err := json.Unmarshal(c.OpenAPIRaw, &outputs); err != nil {
			return fmt.Errorf("parsing openapi array: %w", err)
		}
		c.OpenAPIOutputs = outputs
	} else {
		// Single object mode (backward-compatible)
		var output OpenAPIOutputConfig
		if err := json.Unmarshal(c.OpenAPIRaw, &output); err != nil {
			return fmt.Errorf("parsing openapi config: %w", err)
		}
		c.OpenAPIOutputs = []OpenAPIOutputConfig{output}
	}

	// Populate legacy OpenAPI from first output for backward compat
	if len(c.OpenAPIOutputs) > 0 {
		c.OpenAPI = outputToLegacyConfig(c.OpenAPIOutputs[0])
	}

	return nil
}

// outputToLegacyConfig converts an OpenAPIOutputConfig to the legacy OpenAPIConfig struct.
func outputToLegacyConfig(o OpenAPIOutputConfig) OpenAPIConfig {
	return OpenAPIConfig{
		Output:          o.Output,
		Title:           o.Title,
		Description:     o.Description,
		Version:         o.Version,
		Contact:         o.Contact,
		License:         o.License,
		Servers:         o.Servers,
		SecuritySchemes: o.SecuritySchemes,
		Security:        o.Security,
		Tags:            o.Tags,
		TermsOfService:  o.TermsOfService,
	}
}

// Validate checks the config for logical errors.
func (c *Config) Validate() error {
	if len(c.Controllers.Include) == 0 {
		return fmt.Errorf("controllers.include must have at least one pattern")
	}

	// Validate each OpenAPI output
	outputPaths := make(map[string]bool)
	outputNames := make(map[string]bool)
	for i, o := range c.OpenAPIOutputs {
		if o.Output == "" {
			continue // empty output means "skip this output"
		}
		ext := filepath.Ext(o.Output)
		if ext != ".json" {
			return fmt.Errorf("openapi[%d].output must have a .json extension, got %q", i, ext)
		}
		if outputPaths[o.Output] {
			return fmt.Errorf("duplicate openapi output path: %q", o.Output)
		}
		outputPaths[o.Output] = true
		if o.Name != "" {
			if outputNames[o.Name] {
				return fmt.Errorf("duplicate openapi output name: %q", o.Name)
			}
			outputNames[o.Name] = true
		}
	}

	// Validate runtime
	switch c.Runtime {
	case "", "node", "bun":
		// valid — empty defaults to "node"
	default:
		return fmt.Errorf("runtime must be one of \"node\", \"bun\", got %q", c.Runtime)
	}

	// Validate responseSerializer
	switch c.Transforms.ResponseSerializer {
	case "", "safe", "guard", "none":
		// valid — empty defaults to "guard"
	default:
		return fmt.Errorf("transforms.responseSerializer must be one of \"guard\", \"safe\", \"none\", got %q", c.Transforms.ResponseSerializer)
	}

	return nil
}
