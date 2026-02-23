package openapi

import (
	"encoding/json"
	"testing"

	"github.com/tsgonest/tsgonest/internal/analyzer"
	"github.com/tsgonest/tsgonest/internal/metadata"
)

// --- 13.1: Document Validation Tests ---

func TestValidateDocument_Valid(t *testing.T) {
	doc := &Document{
		OpenAPI: "3.2.0",
		Info:    Info{Title: "Test API", Version: "1.0.0"},
		Paths: map[string]*PathItem{
			"/users": {
				Get: &Operation{
					OperationID: "listUsers",
					Responses: Responses{
						"200": {Description: "OK"},
					},
				},
			},
		},
	}

	errors := ValidateDocument(doc)
	if len(errors) != 0 {
		t.Errorf("expected no validation errors, got %d:", len(errors))
		for _, e := range errors {
			t.Errorf("  %s", e.Error())
		}
	}
}

func TestValidateDocument_MissingOpenAPI(t *testing.T) {
	doc := &Document{
		OpenAPI: "",
		Info:    Info{Title: "Test API", Version: "1.0.0"},
		Paths:   map[string]*PathItem{},
	}

	errors := ValidateDocument(doc)
	found := false
	for _, e := range errors {
		if e.Path == "openapi" && e.Message == "required field missing" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected validation error for missing openapi field")
	}
}

func TestValidateDocument_WrongOpenAPIVersion(t *testing.T) {
	doc := &Document{
		OpenAPI: "2.0.0",
		Info:    Info{Title: "Test API", Version: "1.0.0"},
		Paths:   map[string]*PathItem{},
	}

	errors := ValidateDocument(doc)
	found := false
	for _, e := range errors {
		if e.Path == "openapi" && e.Message == `expected 3.1.x or 3.2.x, got "2.0.0"` {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected validation error for wrong openapi version, got %v", errors)
	}
}

func TestValidateDocument_MissingTitle(t *testing.T) {
	doc := &Document{
		OpenAPI: "3.2.0",
		Info:    Info{Title: "", Version: "1.0.0"},
		Paths:   map[string]*PathItem{},
	}

	errors := ValidateDocument(doc)
	found := false
	for _, e := range errors {
		if e.Path == "info.title" && e.Message == "required field missing" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected validation error for missing info.title")
	}
}

func TestValidateDocument_MissingVersion(t *testing.T) {
	doc := &Document{
		OpenAPI: "3.2.0",
		Info:    Info{Title: "API", Version: ""},
		Paths:   map[string]*PathItem{},
	}

	errors := ValidateDocument(doc)
	found := false
	for _, e := range errors {
		if e.Path == "info.version" && e.Message == "required field missing" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected validation error for missing info.version")
	}
}

func TestValidateDocument_InvalidPath(t *testing.T) {
	doc := &Document{
		OpenAPI: "3.2.0",
		Info:    Info{Title: "API", Version: "1.0.0"},
		Paths: map[string]*PathItem{
			"users": { // missing leading /
				Get: &Operation{
					Responses: Responses{
						"200": {Description: "OK"},
					},
				},
			},
		},
	}

	errors := ValidateDocument(doc)
	found := false
	for _, e := range errors {
		if e.Path == `paths["users"]` && e.Message == "path must begin with /" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected validation error for path without leading /, got %v", errors)
	}
}

func TestValidateDocument_MissingResponses(t *testing.T) {
	doc := &Document{
		OpenAPI: "3.2.0",
		Info:    Info{Title: "API", Version: "1.0.0"},
		Paths: map[string]*PathItem{
			"/users": {
				Get: &Operation{
					OperationID: "listUsers",
					Responses:   nil,
				},
			},
		},
	}

	errors := ValidateDocument(doc)
	found := false
	for _, e := range errors {
		if e.Message == "at least one response is required" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected validation error for missing responses")
	}
}

func TestValidateDocument_EmptyResponses(t *testing.T) {
	doc := &Document{
		OpenAPI: "3.2.0",
		Info:    Info{Title: "API", Version: "1.0.0"},
		Paths: map[string]*PathItem{
			"/users": {
				Post: &Operation{
					OperationID: "createUser",
					Responses:   Responses{},
				},
			},
		},
	}

	errors := ValidateDocument(doc)
	found := false
	for _, e := range errors {
		if e.Message == "at least one response is required" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected validation error for empty responses map")
	}
}

func TestValidateDocument_PathParamRequired(t *testing.T) {
	doc := &Document{
		OpenAPI: "3.2.0",
		Info:    Info{Title: "API", Version: "1.0.0"},
		Paths: map[string]*PathItem{
			"/users/{id}": {
				Get: &Operation{
					OperationID: "getUser",
					Parameters: []Parameter{
						{Name: "id", In: "path", Required: false, Schema: &Schema{Type: "string"}},
					},
					Responses: Responses{
						"200": {Description: "OK"},
					},
				},
			},
		},
	}

	errors := ValidateDocument(doc)
	found := false
	for _, e := range errors {
		if e.Message == "path parameters must be required" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected validation error for non-required path parameter")
	}
}

func TestValidateDocument_InvalidParamIn(t *testing.T) {
	doc := &Document{
		OpenAPI: "3.2.0",
		Info:    Info{Title: "API", Version: "1.0.0"},
		Paths: map[string]*PathItem{
			"/users": {
				Get: &Operation{
					OperationID: "listUsers",
					Parameters: []Parameter{
						{Name: "filter", In: "body", Schema: &Schema{Type: "string"}},
					},
					Responses: Responses{
						"200": {Description: "OK"},
					},
				},
			},
		},
	}

	errors := ValidateDocument(doc)
	found := false
	for _, e := range errors {
		if e.Message == `invalid value "body", must be query/path/header/cookie` {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected validation error for invalid param.in value, got %v", errors)
	}
}

func TestValidateDocument_MissingParamName(t *testing.T) {
	doc := &Document{
		OpenAPI: "3.2.0",
		Info:    Info{Title: "API", Version: "1.0.0"},
		Paths: map[string]*PathItem{
			"/users": {
				Get: &Operation{
					Parameters: []Parameter{
						{Name: "", In: "query", Schema: &Schema{Type: "string"}},
					},
					Responses: Responses{
						"200": {Description: "OK"},
					},
				},
			},
		},
	}

	errors := ValidateDocument(doc)
	found := false
	for _, e := range errors {
		if e.Message == "required field missing" && e.Path == `paths["/users"].get.parameters[0].name` {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected validation error for missing param name, got %v", errors)
	}
}

func TestValidateDocument_MissingParamIn(t *testing.T) {
	doc := &Document{
		OpenAPI: "3.2.0",
		Info:    Info{Title: "API", Version: "1.0.0"},
		Paths: map[string]*PathItem{
			"/users": {
				Get: &Operation{
					Parameters: []Parameter{
						{Name: "q", In: "", Schema: &Schema{Type: "string"}},
					},
					Responses: Responses{
						"200": {Description: "OK"},
					},
				},
			},
		},
	}

	errors := ValidateDocument(doc)
	found := false
	for _, e := range errors {
		if e.Message == "required field missing" && e.Path == `paths["/users"].get.parameters[0].in` {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected validation error for missing param.in, got %v", errors)
	}
}

func TestValidateDocument_MissingResponseDescription(t *testing.T) {
	doc := &Document{
		OpenAPI: "3.2.0",
		Info:    Info{Title: "API", Version: "1.0.0"},
		Paths: map[string]*PathItem{
			"/users": {
				Get: &Operation{
					Responses: Responses{
						"200": {Description: ""},
					},
				},
			},
		},
	}

	errors := ValidateDocument(doc)
	found := false
	for _, e := range errors {
		if e.Message == "required field missing" && e.Path == `paths["/users"].get.responses[200].description` {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected validation error for missing response description, got %v", errors)
	}
}

func TestValidateDocument_MissingPaths(t *testing.T) {
	doc := &Document{
		OpenAPI: "3.2.0",
		Info:    Info{Title: "API", Version: "1.0.0"},
		Paths:   nil,
	}

	errors := ValidateDocument(doc)
	found := false
	for _, e := range errors {
		if e.Path == "paths" && e.Message == "required field missing" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected validation error for missing paths")
	}
}

func TestValidateDocument_EmptyServerURL(t *testing.T) {
	doc := &Document{
		OpenAPI: "3.2.0",
		Info:    Info{Title: "API", Version: "1.0.0"},
		Paths:   map[string]*PathItem{},
		Servers: []Server{{URL: "", Description: "Production"}},
	}

	errors := ValidateDocument(doc)
	found := false
	for _, e := range errors {
		if e.Path == "servers[0].url" && e.Message == "required field missing" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected validation error for empty server URL")
	}
}

func TestValidateDocument_SchemaRefWithType(t *testing.T) {
	doc := &Document{
		OpenAPI: "3.2.0",
		Info:    Info{Title: "API", Version: "1.0.0"},
		Paths:   map[string]*PathItem{},
		Components: &Components{
			Schemas: map[string]*Schema{
				"BadSchema": {Ref: "#/components/schemas/Other", Type: "string"},
			},
		},
	}

	errors := ValidateDocument(doc)
	found := false
	for _, e := range errors {
		if e.Path == "components.schemas.BadSchema" && e.Message == "$ref should not be combined with type" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected validation error for $ref combined with type, got %v", errors)
	}
}

func TestValidateDocument_MultipleErrors(t *testing.T) {
	doc := &Document{
		OpenAPI: "",
		Info:    Info{Title: "", Version: ""},
		Paths:   nil,
	}

	errors := ValidateDocument(doc)
	// Should have at least: missing openapi, missing title, missing version, missing paths
	if len(errors) < 4 {
		t.Errorf("expected at least 4 validation errors, got %d: %v", len(errors), errors)
	}
}

func TestValidateDocument_ValidWithAllMethods(t *testing.T) {
	doc := &Document{
		OpenAPI: "3.2.0",
		Info:    Info{Title: "API", Version: "1.0.0"},
		Paths: map[string]*PathItem{
			"/resource": {
				Get:     &Operation{Responses: Responses{"200": {Description: "OK"}}},
				Post:    &Operation{Responses: Responses{"201": {Description: "Created"}}},
				Put:     &Operation{Responses: Responses{"200": {Description: "OK"}}},
				Delete:  &Operation{Responses: Responses{"204": {Description: "No Content"}}},
				Patch:   &Operation{Responses: Responses{"200": {Description: "OK"}}},
				Head:    &Operation{Responses: Responses{"200": {Description: "OK"}}},
				Options: &Operation{Responses: Responses{"200": {Description: "OK"}}},
			},
		},
	}

	errors := ValidateDocument(doc)
	if len(errors) != 0 {
		t.Errorf("expected no validation errors for valid doc with all methods, got %d:", len(errors))
		for _, e := range errors {
			t.Errorf("  %s", e.Error())
		}
	}
}

func TestValidateDocument_ValidPathParams(t *testing.T) {
	doc := &Document{
		OpenAPI: "3.2.0",
		Info:    Info{Title: "API", Version: "1.0.0"},
		Paths: map[string]*PathItem{
			"/users/{id}": {
				Get: &Operation{
					Parameters: []Parameter{
						{Name: "id", In: "path", Required: true, Schema: &Schema{Type: "string"}},
					},
					Responses: Responses{"200": {Description: "OK"}},
				},
			},
		},
	}

	errors := ValidateDocument(doc)
	if len(errors) != 0 {
		t.Errorf("expected no validation errors, got %d:", len(errors))
		for _, e := range errors {
			t.Errorf("  %s", e.Error())
		}
	}
}

// --- 13.4: Compliance Tests (generated documents) ---

func TestCompliance_GeneratedSimpleDoc(t *testing.T) {
	registry := metadata.NewTypeRegistry()
	registry.Register("CreateUserDto", &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "CreateUserDto",
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "email", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	})
	registry.Register("UserResponse", &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "UserResponse",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	})

	gen := NewGenerator(registry)
	controllers := []analyzer.ControllerInfo{
		{
			Name: "UserController",
			Path: "users",
			Routes: []analyzer.Route{
				{
					Method:      "GET",
					Path:        "/users",
					OperationID: "findAll",
					ReturnType:  metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindRef, Ref: "UserResponse"}},
					StatusCode:  200,
					Tags:        []string{"User"},
				},
				{
					Method:      "GET",
					Path:        "/users/:id",
					OperationID: "findOne",
					Parameters: []analyzer.RouteParameter{
						{Category: "param", Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "UserResponse"},
					StatusCode: 200,
					Tags:       []string{"User"},
				},
				{
					Method:      "POST",
					Path:        "/users",
					OperationID: "create",
					Parameters: []analyzer.RouteParameter{
						{Category: "body", Name: "", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "CreateUserDto"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "UserResponse"},
					StatusCode: 201,
					Tags:       []string{"User"},
				},
				{
					Method:      "DELETE",
					Path:        "/users/:id",
					OperationID: "remove",
					Parameters: []analyzer.RouteParameter{
						{Category: "param", Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindVoid},
					StatusCode: 204,
					Tags:       []string{"User"},
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	errors := ValidateDocument(doc)

	if len(errors) != 0 {
		t.Errorf("generated simple doc should pass validation, got %d errors:", len(errors))
		for _, e := range errors {
			t.Errorf("  %s", e.Error())
		}
	}
}

func TestCompliance_GeneratedWithErrors(t *testing.T) {
	registry := metadata.NewTypeRegistry()
	registry.Register("BadRequestError", &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "BadRequestError",
		Properties: []metadata.Property{
			{Name: "message", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "statusCode", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
		},
	})

	gen := NewGenerator(registry)
	controllers := []analyzer.ControllerInfo{
		{
			Name: "UserController",
			Path: "users",
			Routes: []analyzer.Route{
				{
					Method:      "POST",
					Path:        "/users",
					OperationID: "create",
					Parameters: []analyzer.RouteParameter{
						{Category: "body", Name: "", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "BadRequestError"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					StatusCode: 201,
					Tags:       []string{"User"},
					ErrorResponses: []analyzer.ErrorResponse{
						{StatusCode: 400, TypeName: "BadRequestError", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "BadRequestError"}},
						{StatusCode: 401, TypeName: "UnauthorizedError", Type: metadata.Metadata{Kind: ""}},
					},
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	errors := ValidateDocument(doc)

	if len(errors) != 0 {
		t.Errorf("generated doc with error responses should pass validation, got %d errors:", len(errors))
		for _, e := range errors {
			t.Errorf("  %s", e.Error())
		}
	}
}

func TestCompliance_GeneratedWithSecurity(t *testing.T) {
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "SecureController",
			Path: "secure",
			Routes: []analyzer.Route{
				{
					Method:      "GET",
					Path:        "/secure/data",
					OperationID: "getData",
					ReturnType:  metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					StatusCode:  200,
					Tags:        []string{"Secure"},
					Security: []analyzer.SecurityRequirement{
						{Name: "bearer", Scopes: []string{}},
					},
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	doc.ApplyConfig(DocumentConfig{
		SecuritySchemes: map[string]*SecurityScheme{
			"bearer": {Type: "http", Scheme: "bearer", BearerFormat: "JWT"},
		},
	})

	errors := ValidateDocument(doc)
	if len(errors) != 0 {
		t.Errorf("generated doc with security should pass validation, got %d errors:", len(errors))
		for _, e := range errors {
			t.Errorf("  %s", e.Error())
		}
	}
}

func TestCompliance_GeneratedWithVersioning(t *testing.T) {
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "UserController",
			Path: "users",
			Routes: []analyzer.Route{
				{
					Method:      "GET",
					Path:        "/users",
					OperationID: "findAllV1",
					ReturnType:  metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					StatusCode:  200,
					Tags:        []string{"User"},
					Version:     "1",
				},
				{
					Method:      "GET",
					Path:        "/users",
					OperationID: "findAllV2",
					ReturnType:  metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					StatusCode:  200,
					Tags:        []string{"User"},
					Version:     "2",
				},
			},
		},
	}

	doc := gen.GenerateWithOptions(controllers, &GenerateOptions{
		GlobalPrefix:   "api",
		VersioningType: "URI",
		VersionPrefix:  "v",
	})

	errors := ValidateDocument(doc)
	if len(errors) != 0 {
		t.Errorf("generated doc with versioning should pass validation, got %d errors:", len(errors))
		for _, e := range errors {
			t.Errorf("  %s", e.Error())
		}
	}
}

func TestCompliance_GeneratedMultiController(t *testing.T) {
	registry := metadata.NewTypeRegistry()
	registry.Register("User", &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "User",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	})
	registry.Register("Post", &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "Post",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "title", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "body", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	})

	gen := NewGenerator(registry)
	controllers := []analyzer.ControllerInfo{
		{
			Name: "UserController",
			Path: "users",
			Routes: []analyzer.Route{
				{
					Method:      "GET",
					Path:        "/users",
					OperationID: "listUsers",
					ReturnType:  metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindRef, Ref: "User"}},
					StatusCode:  200,
					Tags:        []string{"User"},
				},
			},
		},
		{
			Name: "PostController",
			Path: "posts",
			Routes: []analyzer.Route{
				{
					Method:      "GET",
					Path:        "/posts",
					OperationID: "listPosts",
					ReturnType:  metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindRef, Ref: "Post"}},
					StatusCode:  200,
					Tags:        []string{"Post"},
				},
				{
					Method:      "GET",
					Path:        "/posts/:id",
					OperationID: "getPost",
					Parameters: []analyzer.RouteParameter{
						{Category: "param", Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "Post"},
					StatusCode: 200,
					Tags:       []string{"Post"},
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	errors := ValidateDocument(doc)

	if len(errors) != 0 {
		t.Errorf("multi-controller doc should pass validation, got %d errors:", len(errors))
		for _, e := range errors {
			t.Errorf("  %s", e.Error())
		}
	}
}

// --- 13.1: ValidateJSON Tests ---

func TestValidateJSON_ValidJSON(t *testing.T) {
	doc := &Document{
		OpenAPI: "3.2.0",
		Info:    Info{Title: "API", Version: "1.0.0"},
		Paths: map[string]*PathItem{
			"/health": {
				Get: &Operation{
					Responses: Responses{
						"200": {Description: "OK"},
					},
				},
			},
		},
	}
	jsonData, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	errors, err := ValidateJSON(jsonData)
	if err != nil {
		t.Fatalf("ValidateJSON returned error: %v", err)
	}
	if len(errors) != 0 {
		t.Errorf("expected no validation errors from valid JSON, got %d:", len(errors))
		for _, e := range errors {
			t.Errorf("  %s", e.Error())
		}
	}
}

func TestValidateJSON_InvalidJSON(t *testing.T) {
	_, err := ValidateJSON([]byte(`{invalid json`))
	if err == nil {
		t.Error("expected error from invalid JSON, got nil")
	}
}

func TestValidateJSON_MissingFields(t *testing.T) {
	jsonData := []byte(`{"openapi":"3.2.0","info":{"title":"","version":""},"paths":{}}`)
	errors, err := ValidateJSON(jsonData)
	if err != nil {
		t.Fatalf("ValidateJSON returned error: %v", err)
	}
	if len(errors) < 2 {
		t.Errorf("expected at least 2 validation errors (missing title, version), got %d", len(errors))
	}
}

// --- ValidationError.Error() ---

func TestValidationError_Error(t *testing.T) {
	e := ValidationError{Path: "info.title", Message: "required field missing"}
	want := "info.title: required field missing"
	if e.Error() != want {
		t.Errorf("expected %q, got %q", want, e.Error())
	}
}
