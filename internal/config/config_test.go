package config

import (
	"os"
	"path/filepath"
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

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for empty openapi output")
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
