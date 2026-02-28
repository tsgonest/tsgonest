package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if len(cfg.Controllers.Include) != 1 {
		t.Fatalf("expected 1 default include pattern, got %d", len(cfg.Controllers.Include))
	}
	if cfg.Controllers.Include[0] != "src/**/*.controller.ts" {
		t.Fatalf("expected default include pattern 'src/**/*.controller.ts', got %q", cfg.Controllers.Include[0])
	}
	if !cfg.Transforms.Validation {
		t.Fatal("expected validation to be true by default")
	}
	if !cfg.Transforms.Serialization {
		t.Fatal("expected serialization to be true by default")
	}
	if cfg.OpenAPI.Output != "dist/openapi.json" {
		t.Fatalf("expected default openapi output 'dist/openapi.json', got %q", cfg.OpenAPI.Output)
	}
}

func TestLoadValidConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "tsgonest.config.json")
	content := `{
		"controllers": {
			"include": ["src/modules/**/*.controller.ts"],
			"exclude": ["src/**/*.spec.ts"]
		},
		"transforms": {
			"validation": true,
			"serialization": false
		},
		"openapi": {
			"output": "dist/api/openapi.json"
		}
	}`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Controllers.Include) != 1 || cfg.Controllers.Include[0] != "src/modules/**/*.controller.ts" {
		t.Fatalf("unexpected include: %v", cfg.Controllers.Include)
	}
	if len(cfg.Controllers.Exclude) != 1 || cfg.Controllers.Exclude[0] != "src/**/*.spec.ts" {
		t.Fatalf("unexpected exclude: %v", cfg.Controllers.Exclude)
	}
	if !cfg.Transforms.Validation {
		t.Fatal("expected validation to be true")
	}
	if cfg.Transforms.Serialization {
		t.Fatal("expected serialization to be false")
	}
	if cfg.OpenAPI.Output != "dist/api/openapi.json" {
		t.Fatalf("unexpected openapi output: %q", cfg.OpenAPI.Output)
	}
}

func TestLoadPartialConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "tsgonest.config.json")
	content := `{
		"openapi": {
			"output": "out/openapi.json"
		}
	}`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have defaults for unspecified fields
	if len(cfg.Controllers.Include) != 1 || cfg.Controllers.Include[0] != "src/**/*.controller.ts" {
		t.Fatalf("expected default include, got %v", cfg.Controllers.Include)
	}
	if !cfg.Transforms.Validation {
		t.Fatal("expected default validation=true")
	}
	if cfg.OpenAPI.Output != "out/openapi.json" {
		t.Fatalf("expected overridden openapi output, got %q", cfg.OpenAPI.Output)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "tsgonest.config.json")
	if err := os.WriteFile(configPath, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestValidateEmptyInclude(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Controllers.Include = []string{}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for empty include")
	}
}

func TestValidateEmptyOpenAPIOutput(t *testing.T) {
	cfg := DefaultConfig()
	cfg.OpenAPI.Output = ""

	// Empty openapi.output is valid — it means "no OpenAPI generation"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validation error for empty openapi output: %v", err)
	}
}

func TestValidateCompanionsOnlyNoOpenAPI(t *testing.T) {
	cfg := DefaultConfig()
	cfg.OpenAPI.Output = ""
	cfg.Transforms.Validation = true
	cfg.Transforms.Serialization = true

	if err := cfg.Validate(); err != nil {
		t.Fatalf("companions-only config (no OpenAPI) should be valid: %v", err)
	}
}

func TestValidateNonJSONOpenAPIOutput(t *testing.T) {
	cfg := DefaultConfig()
	cfg.OpenAPI.Output = "dist/openapi.yaml"

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for non-json openapi output")
	}
}

func TestValidateValidConfig(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestConfig_NestJSGlobalPrefixAndVersioning(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "tsgonest.config.json")
	content := `{
		"controllers": {
			"include": ["src/**/*.controller.ts"]
		},
		"openapi": {
			"output": "dist/openapi.json"
		},
		"nestjs": {
			"globalPrefix": "api",
			"versioning": {
				"type": "URI",
				"defaultVersion": "1",
				"prefix": "v"
			}
		}
	}`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.NestJS.GlobalPrefix != "api" {
		t.Errorf("expected globalPrefix='api', got %q", cfg.NestJS.GlobalPrefix)
	}
	if cfg.NestJS.Versioning == nil {
		t.Fatal("expected versioning to be set")
	}
	if cfg.NestJS.Versioning.Type != "URI" {
		t.Errorf("expected versioning type='URI', got %q", cfg.NestJS.Versioning.Type)
	}
	if cfg.NestJS.Versioning.DefaultVersion != "1" {
		t.Errorf("expected defaultVersion='1', got %q", cfg.NestJS.Versioning.DefaultVersion)
	}
	if cfg.NestJS.Versioning.Prefix != "v" {
		t.Errorf("expected prefix='v', got %q", cfg.NestJS.Versioning.Prefix)
	}
}

func TestConfig_NestJSDefaults(t *testing.T) {
	cfg := DefaultConfig()

	// NestJS config should have zero values by default
	if cfg.NestJS.GlobalPrefix != "" {
		t.Errorf("expected empty globalPrefix by default, got %q", cfg.NestJS.GlobalPrefix)
	}
	if cfg.NestJS.Versioning != nil {
		t.Errorf("expected nil versioning by default, got %v", cfg.NestJS.Versioning)
	}
}

// requireNode skips the test if node is not available.
func requireNode(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not found in PATH, skipping TypeScript config test")
	}
}

func TestDiscover_TSPriority(t *testing.T) {
	dir := t.TempDir()

	// No config files → empty string
	result := Discover(dir)
	if result != "" {
		t.Fatalf("expected empty string for no config, got %q", result)
	}

	// Only .json → returns .json
	jsonPath := filepath.Join(dir, "tsgonest.config.json")
	os.WriteFile(jsonPath, []byte(`{"controllers":{"include":["src/**/*.controller.ts"]},"openapi":{"output":"dist/openapi.json"}}`), 0o644)
	result = Discover(dir)
	if result != jsonPath {
		t.Fatalf("expected %q, got %q", jsonPath, result)
	}

	// Both .ts and .json → returns .ts (higher priority)
	tsPath := filepath.Join(dir, "tsgonest.config.ts")
	os.WriteFile(tsPath, []byte(`export default {}`), 0o644)
	result = Discover(dir)
	if result != tsPath {
		t.Fatalf("expected .ts to take priority, got %q", result)
	}
}

func TestDiscover_OnlyTS(t *testing.T) {
	dir := t.TempDir()

	tsPath := filepath.Join(dir, "tsgonest.config.ts")
	os.WriteFile(tsPath, []byte(`export default {}`), 0o644)
	result := Discover(dir)
	if result != tsPath {
		t.Fatalf("expected %q, got %q", tsPath, result)
	}
}

func TestLoad_DispatchesByExtension(t *testing.T) {
	// .json should work as before
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "tsgonest.config.json")
	os.WriteFile(jsonPath, []byte(`{
		"controllers": { "include": ["src/**/*.controller.ts"] },
		"openapi": { "output": "dist/openapi.json" }
	}`), 0o644)

	cfg, err := Load(jsonPath)
	if err != nil {
		t.Fatalf("unexpected error loading .json: %v", err)
	}
	if cfg.Controllers.Include[0] != "src/**/*.controller.ts" {
		t.Fatalf("unexpected include: %v", cfg.Controllers.Include)
	}

	// Unsupported extension should error
	yamlPath := filepath.Join(dir, "tsgonest.config.yaml")
	os.WriteFile(yamlPath, []byte(""), 0o644)
	_, err = Load(yamlPath)
	if err == nil {
		t.Fatal("expected error for .yaml extension")
	}
	if !strings.Contains(err.Error(), "unsupported config file extension") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestLoadTS_PlainExport(t *testing.T) {
	requireNode(t)

	dir := t.TempDir()
	tsPath := filepath.Join(dir, "tsgonest.config.ts")
	content := `export default {
  controllers: {
    include: ["src/**/*.controller.ts"],
  },
  transforms: {
    validation: true,
    serialization: false,
  },
  openapi: {
    output: "dist/openapi.json",
    title: "My API",
  },
};
`
	os.WriteFile(tsPath, []byte(content), 0o644)

	cfg, err := LoadTS(tsPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Controllers.Include[0] != "src/**/*.controller.ts" {
		t.Fatalf("unexpected include: %v", cfg.Controllers.Include)
	}
	if !cfg.Transforms.Validation {
		t.Fatal("expected validation=true")
	}
	if cfg.Transforms.Serialization {
		t.Fatal("expected serialization=false")
	}
	if cfg.OpenAPI.Title != "My API" {
		t.Fatalf("expected title='My API', got %q", cfg.OpenAPI.Title)
	}
}

func TestLoadTS_WithDefaults(t *testing.T) {
	requireNode(t)

	dir := t.TempDir()
	tsPath := filepath.Join(dir, "tsgonest.config.ts")
	// Partial config — should merge with defaults
	content := `export default {
  openapi: {
    output: "out/api.json",
  },
};
`
	os.WriteFile(tsPath, []byte(content), 0o644)

	cfg, err := LoadTS(tsPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Defaults should be applied
	if cfg.Controllers.Include[0] != "src/**/*.controller.ts" {
		t.Fatalf("expected default include, got %v", cfg.Controllers.Include)
	}
	if !cfg.Transforms.Validation {
		t.Fatal("expected default validation=true")
	}
	if cfg.OpenAPI.Output != "out/api.json" {
		t.Fatalf("expected output='out/api.json', got %q", cfg.OpenAPI.Output)
	}
}

func TestLoadTS_WithNestJSConfig(t *testing.T) {
	requireNode(t)

	dir := t.TempDir()
	tsPath := filepath.Join(dir, "tsgonest.config.ts")
	content := `export default {
  controllers: {
    include: ["src/**/*.controller.ts"],
  },
  openapi: {
    output: "dist/openapi.json",
  },
  nestjs: {
    globalPrefix: "api",
    versioning: {
      type: "URI",
      defaultVersion: "1",
      prefix: "v",
    },
  },
};
`
	os.WriteFile(tsPath, []byte(content), 0o644)

	cfg, err := LoadTS(tsPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.NestJS.GlobalPrefix != "api" {
		t.Errorf("expected globalPrefix='api', got %q", cfg.NestJS.GlobalPrefix)
	}
	if cfg.NestJS.Versioning == nil {
		t.Fatal("expected versioning to be set")
	}
	if cfg.NestJS.Versioning.Type != "URI" {
		t.Errorf("expected type='URI', got %q", cfg.NestJS.Versioning.Type)
	}
}

func TestLoadTS_NoDefaultExport(t *testing.T) {
	requireNode(t)

	dir := t.TempDir()
	tsPath := filepath.Join(dir, "tsgonest.config.ts")
	content := `const config = { controllers: { include: ["src/**/*.controller.ts"] } };
`
	os.WriteFile(tsPath, []byte(content), 0o644)

	_, err := LoadTS(tsPath)
	if err == nil {
		t.Fatal("expected error for missing default export")
	}
}

func TestLoadTS_InvalidConfig(t *testing.T) {
	requireNode(t)

	dir := t.TempDir()
	tsPath := filepath.Join(dir, "tsgonest.config.ts")
	// Valid syntax but fails validation (empty controllers.include)
	content := `export default {
  controllers: {
    include: [],
  },
  openapi: {
    output: "dist/openapi.json",
  },
};
`
	os.WriteFile(tsPath, []byte(content), 0o644)

	_, err := LoadTS(tsPath)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "controllers.include") {
		t.Fatalf("expected validation error about controllers.include, got: %v", err)
	}
}

func TestLoadConfig_TransformsExclude(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "tsgonest.config.json")
	content := `{
		"controllers": {
			"include": ["src/**/*.controller.ts"]
		},
		"transforms": {
			"validation": true,
			"serialization": true,
			"exclude": ["Legacy*", "Deprecated*"]
		},
		"openapi": {
			"output": "dist/openapi.json"
		}
	}`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Transforms.Exclude) != 2 {
		t.Fatalf("expected 2 exclude patterns, got %d", len(cfg.Transforms.Exclude))
	}
	if cfg.Transforms.Exclude[0] != "Legacy*" {
		t.Errorf("expected first exclude pattern 'Legacy*', got %q", cfg.Transforms.Exclude[0])
	}
	if cfg.Transforms.Exclude[1] != "Deprecated*" {
		t.Errorf("expected second exclude pattern 'Deprecated*', got %q", cfg.Transforms.Exclude[1])
	}
}

func TestLoadConfig_ResponseTypeCheck(t *testing.T) {
	for _, mode := range []string{"safe", "guard", "none"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			configPath := filepath.Join(dir, "tsgonest.config.json")
			content := fmt.Sprintf(`{
				"controllers": { "include": ["src/**/*.controller.ts"] },
				"transforms": { "validation": true, "serialization": true, "responseTypeCheck": %q },
				"openapi": { "output": "dist/openapi.json" }
			}`, mode)
			if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}

			cfg, err := Load(configPath)
			if err != nil {
				t.Fatalf("unexpected error for mode %q: %v", mode, err)
			}
			if cfg.Transforms.ResponseTypeCheck != mode {
				t.Errorf("expected responseTypeCheck=%q, got %q", mode, cfg.Transforms.ResponseTypeCheck)
			}
		})
	}

	// Invalid mode should fail
	t.Run("invalid", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "tsgonest.config.json")
		content := `{
			"controllers": { "include": ["src/**/*.controller.ts"] },
			"transforms": { "validation": true, "serialization": true, "responseTypeCheck": "invalid" },
			"openapi": { "output": "dist/openapi.json" }
		}`
		if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := Load(configPath)
		if err == nil {
			t.Fatal("expected validation error for invalid responseTypeCheck")
		}
		if !strings.Contains(err.Error(), "responseTypeCheck") {
			t.Fatalf("expected error about responseTypeCheck, got: %v", err)
		}
	})
}

func TestLoadConfig_SDKConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "tsgonest.config.json")
	content := `{
		"controllers": { "include": ["src/**/*.controller.ts"] },
		"openapi": { "output": "dist/openapi.json" },
		"sdk": { "output": "./sdk", "input": "external-openapi.json" }
	}`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.SDK.Output != "./sdk" {
		t.Errorf("expected sdk.output='./sdk', got %q", cfg.SDK.Output)
	}
	if cfg.SDK.Input != "external-openapi.json" {
		t.Errorf("expected sdk.input='external-openapi.json', got %q", cfg.SDK.Input)
	}
}

func TestLoadTS_ViaLoadDispatch(t *testing.T) {
	requireNode(t)

	dir := t.TempDir()
	tsPath := filepath.Join(dir, "tsgonest.config.ts")
	content := `export default {
  controllers: {
    include: ["src/**/*.controller.ts"],
  },
  openapi: {
    output: "dist/openapi.json",
  },
};
`
	os.WriteFile(tsPath, []byte(content), 0o644)

	// Load() should dispatch to LoadTS for .ts files
	cfg, err := Load(tsPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Controllers.Include[0] != "src/**/*.controller.ts" {
		t.Fatalf("unexpected include: %v", cfg.Controllers.Include)
	}
}
