package config

import (
	"testing"
)

func TestValidateDetailed_Valid(t *testing.T) {
	cfg := DefaultConfig()
	result := cfg.ValidateDetailed()
	if !result.IsValid() {
		t.Errorf("expected valid config, got errors: %v", result.Errors)
	}
}

func TestValidateDetailed_MissingInclude(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Controllers.Include = nil
	result := cfg.ValidateDetailed()
	if result.IsValid() {
		t.Error("expected invalid config")
	}
}

func TestValidateDetailed_DisabledTransformsWarning(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Transforms.Validation = false
	cfg.Transforms.Serialization = false
	result := cfg.ValidateDetailed()
	if len(result.Warnings) == 0 {
		t.Error("expected warning about disabled transforms")
	}
}

func TestValidateDetailed_InvalidVersioningType(t *testing.T) {
	cfg := DefaultConfig()
	cfg.NestJS.Versioning = &VersioningConfig{Type: "INVALID"}
	result := cfg.ValidateDetailed()
	if result.IsValid() {
		t.Error("expected error for invalid versioning type")
	}
}

func TestValidateDetailed_WeirdIncludePattern(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Controllers.Include = []string{"src/controllers"}
	result := cfg.ValidateDetailed()
	if len(result.Warnings) == 0 {
		t.Error("expected warning for pattern without wildcard")
	}
}
