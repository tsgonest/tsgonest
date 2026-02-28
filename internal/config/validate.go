package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidationResult holds config validation results.
type ValidationResult struct {
	Errors   []string
	Warnings []string
}

// ValidateDetailed performs thorough config validation with suggestions.
func (c *Config) ValidateDetailed() *ValidationResult {
	result := &ValidationResult{}

	// Controllers
	if len(c.Controllers.Include) == 0 {
		result.Errors = append(result.Errors, "controllers.include: at least one pattern required")
	}
	for _, pattern := range c.Controllers.Include {
		if !strings.Contains(pattern, "*") && !strings.HasSuffix(pattern, ".ts") {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("controllers.include: pattern %q doesn't contain a wildcard or .ts extension — did you mean %q?", pattern, pattern+"/**/*.controller.ts"))
		}
	}

	// OpenAPI — output is optional (empty means "no OpenAPI generation")
	if c.OpenAPI.Output != "" {
		ext := filepath.Ext(c.OpenAPI.Output)
		if ext != ".json" && ext != ".yaml" && ext != ".yml" {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("openapi.output: extension %q is unusual — expected .json, .yaml, or .yml", ext))
		}
	}

	// Transforms
	if !c.Transforms.Validation && !c.Transforms.Serialization {
		result.Warnings = append(result.Warnings,
			"transforms: both validation and serialization are disabled — no companion files will be generated")
	}

	// NestJS
	if c.NestJS.Versioning != nil {
		vType := strings.ToUpper(c.NestJS.Versioning.Type)
		if vType != "URI" && vType != "HEADER" && vType != "MEDIA_TYPE" && vType != "" {
			result.Errors = append(result.Errors,
				fmt.Sprintf("nestjs.versioning.type: invalid value %q — must be URI, HEADER, or MEDIA_TYPE", c.NestJS.Versioning.Type))
		}
	}

	return result
}

// IsValid returns true if there are no errors.
func (r *ValidationResult) IsValid() bool {
	return len(r.Errors) == 0
}
