package openapi

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ValidationError represents an OpenAPI spec compliance error.
type ValidationError struct {
	Path    string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Path, e.Message)
}

// ValidateDocument checks an OpenAPI document for spec compliance.
// Returns a list of validation errors, or nil if the document is valid.
func ValidateDocument(doc *Document) []ValidationError {
	var errors []ValidationError

	// Required: openapi version
	if doc.OpenAPI == "" {
		errors = append(errors, ValidationError{Path: "openapi", Message: "required field missing"})
	} else if !strings.HasPrefix(doc.OpenAPI, "3.1") {
		errors = append(errors, ValidationError{Path: "openapi", Message: fmt.Sprintf("expected 3.1.x, got %q", doc.OpenAPI)})
	}

	// Required: info
	if doc.Info.Title == "" {
		errors = append(errors, ValidationError{Path: "info.title", Message: "required field missing"})
	}
	if doc.Info.Version == "" {
		errors = append(errors, ValidationError{Path: "info.version", Message: "required field missing"})
	}

	// Required: paths
	if doc.Paths == nil {
		errors = append(errors, ValidationError{Path: "paths", Message: "required field missing"})
	}

	// Validate paths
	for path, item := range doc.Paths {
		if !strings.HasPrefix(path, "/") {
			errors = append(errors, ValidationError{
				Path:    fmt.Sprintf("paths[%q]", path),
				Message: "path must begin with /",
			})
		}
		errors = append(errors, validatePathItem(path, item)...)
	}

	// Validate components/schemas
	if doc.Components != nil {
		for name, schema := range doc.Components.Schemas {
			errors = append(errors, validateSchema(fmt.Sprintf("components.schemas.%s", name), schema)...)
		}
	}

	// Validate servers
	for i, server := range doc.Servers {
		if server.URL == "" {
			errors = append(errors, ValidationError{
				Path:    fmt.Sprintf("servers[%d].url", i),
				Message: "required field missing",
			})
		}
	}

	return errors
}

func validatePathItem(path string, item *PathItem) []ValidationError {
	var errors []ValidationError
	prefix := fmt.Sprintf("paths[%q]", path)

	ops := map[string]*Operation{
		"get": item.Get, "post": item.Post, "put": item.Put,
		"delete": item.Delete, "patch": item.Patch,
		"head": item.Head, "options": item.Options,
	}

	for method, op := range ops {
		if op == nil {
			continue
		}
		errors = append(errors, validateOperation(fmt.Sprintf("%s.%s", prefix, method), op)...)
	}

	return errors
}

func validateOperation(prefix string, op *Operation) []ValidationError {
	var errors []ValidationError

	// Responses is required
	if op.Responses == nil || len(op.Responses) == 0 {
		errors = append(errors, ValidationError{
			Path:    prefix + ".responses",
			Message: "at least one response is required",
		})
	}

	// Validate parameters
	for i, param := range op.Parameters {
		paramPath := fmt.Sprintf("%s.parameters[%d]", prefix, i)
		if param.Name == "" {
			errors = append(errors, ValidationError{Path: paramPath + ".name", Message: "required field missing"})
		}
		if param.In == "" {
			errors = append(errors, ValidationError{Path: paramPath + ".in", Message: "required field missing"})
		} else if param.In != "query" && param.In != "path" && param.In != "header" && param.In != "cookie" {
			errors = append(errors, ValidationError{
				Path:    paramPath + ".in",
				Message: fmt.Sprintf("invalid value %q, must be query/path/header/cookie", param.In),
			})
		}
		if param.In == "path" && !param.Required {
			errors = append(errors, ValidationError{
				Path:    paramPath + ".required",
				Message: "path parameters must be required",
			})
		}
	}

	// Validate responses
	for code, resp := range op.Responses {
		respPath := fmt.Sprintf("%s.responses[%s]", prefix, code)
		if resp.Description == "" {
			errors = append(errors, ValidationError{Path: respPath + ".description", Message: "required field missing"})
		}
	}

	return errors
}

func validateSchema(prefix string, schema *Schema) []ValidationError {
	var errors []ValidationError

	// A schema with $ref should not have other properties (simplified check)
	if schema.Ref != "" {
		if schema.Type != "" {
			errors = append(errors, ValidationError{
				Path:    prefix,
				Message: "$ref should not be combined with type",
			})
		}
	}

	return errors
}

// ValidateJSON validates raw JSON against OAS 3.1 structural requirements.
func ValidateJSON(jsonData []byte) ([]ValidationError, error) {
	var doc Document
	if err := json.Unmarshal(jsonData, &doc); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}
	return ValidateDocument(&doc), nil
}
