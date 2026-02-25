package openapi

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tsgonest/tsgonest/internal/analyzer"
	"github.com/tsgonest/tsgonest/internal/metadata"
)

// ---- Feature Parity Tests ----
// Tests for all features added in the OpenAPI feature parity release.

// --- 1. OperationID format ---

func TestFeature_OperationID_ControllerPrefix(t *testing.T) {
	// The analyzer now produces Controller_method operationIds.
	// Generator should use them as-is.
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "UserController",
			Path: "users",
			Routes: []analyzer.Route{
				{
					Method: "GET", Path: "/users", OperationID: "User_findAll",
					ReturnType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					StatusCode: 200, Tags: []string{"User"},
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	data := requireValidDoc(t, doc)
	assertJSONContains(t, data, `"operationId": "User_findAll"`)
}

func TestFeature_OperationID_Versioned(t *testing.T) {
	// When versioning is active, operationId should be Controller_vN_method
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "UserController",
			Path: "users",
			Routes: []analyzer.Route{
				{
					Method: "GET", Path: "/users", OperationID: "User_v1_findAll",
					ReturnType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					StatusCode: 200, Tags: []string{"User"}, Version: "1",
				},
				{
					Method: "GET", Path: "/users", OperationID: "User_v2_findAll",
					ReturnType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					StatusCode: 200, Tags: []string{"User"}, Version: "2",
				},
			},
		},
	}

	doc := gen.GenerateWithOptions(controllers, &GenerateOptions{
		VersioningType: "URI", VersionPrefix: "v",
	})
	data := requireValidDoc(t, doc)
	assertJSONContains(t, data, `"operationId": "User_v1_findAll"`)
	assertJSONContains(t, data, `"operationId": "User_v2_findAll"`)
}

// --- 2. @throws descriptions ---

func TestFeature_ThrowsDescription(t *testing.T) {
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "UserController",
			Path: "users",
			Routes: []analyzer.Route{
				{
					Method: "GET", Path: "/users/:id", OperationID: "User_getUser",
					ReturnType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					StatusCode: 200, Tags: []string{"User"},
					ErrorResponses: []analyzer.ErrorResponse{
						{StatusCode: 404, TypeName: "NotFound", Description: "The user was not found"},
						{StatusCode: 401, TypeName: "Unauthorized"},
					},
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	data := requireValidDoc(t, doc)

	// 404 should use the custom description
	assertJSONContains(t, data, `"The user was not found"`)
	// 401 should use the default status description
	assertJSONContains(t, data, `"Unauthorized"`)
}

// --- 3/4. Global security + @public ---

func TestFeature_GlobalSecurity(t *testing.T) {
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "UserController",
			Path: "users",
			Routes: []analyzer.Route{
				{
					Method: "GET", Path: "/users", OperationID: "User_findAll",
					ReturnType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					StatusCode: 200, Tags: []string{"User"},
				},
			},
		},
	}

	doc := gen.Generate(controllers)

	// Apply global security via config
	doc.ApplyConfig(DocumentConfig{
		Security: []map[string][]string{
			{"bearer": {}},
		},
		SecuritySchemes: map[string]*SecurityScheme{
			"bearer": {Type: "http", Scheme: "bearer"},
		},
	})

	data := requireValidDoc(t, doc)

	// Document-level security should be present
	assertJSONContains(t, data, `"security"`)
	assertJSONContains(t, data, `"bearer"`)
}

func TestFeature_PublicRoute_OverridesGlobalSecurity(t *testing.T) {
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "AuthController",
			Path: "auth",
			Routes: []analyzer.Route{
				{
					Method: "POST", Path: "/auth/login", OperationID: "Auth_login",
					ReturnType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					StatusCode: 200, Tags: []string{"Auth"},
					IsPublic: true, // @public — no security
				},
				{
					Method: "GET", Path: "/auth/profile", OperationID: "Auth_profile",
					ReturnType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					StatusCode: 200, Tags: []string{"Auth"},
					// No IsPublic — inherits global security
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	doc.ApplyConfig(DocumentConfig{
		Security: []map[string][]string{{"bearer": {}}},
	})

	data := requireValidDoc(t, doc)
	raw := parseJSON(t, data)

	// The login route should have empty security array
	paths := raw["paths"].(map[string]any)
	loginPath := paths["/auth/login"].(map[string]any)
	loginPost := loginPath["post"].(map[string]any)
	loginSec := loginPost["security"].([]any)
	if len(loginSec) != 0 {
		t.Errorf("expected @public route to have empty security array, got %v", loginSec)
	}

	// The profile route should NOT have operation-level security (inherits global)
	profilePath := paths["/auth/profile"].(map[string]any)
	profileGet := profilePath["get"].(map[string]any)
	if _, hasSec := profileGet["security"]; hasSec {
		t.Errorf("expected non-public route to omit operation-level security (inherits global)")
	}
}

// --- 5. Controller-level security ---

func TestFeature_ControllerLevelSecurity(t *testing.T) {
	// Controller-level @security is inherited by methods.
	// This is tested by providing pre-populated Security on routes.
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "AdminController",
			Path: "admin",
			Routes: []analyzer.Route{
				{
					Method: "GET", Path: "/admin/users", OperationID: "Admin_listUsers",
					ReturnType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					StatusCode: 200, Tags: []string{"Admin"},
					Security: []analyzer.SecurityRequirement{{Name: "bearer"}},
				},
				{
					Method: "DELETE", Path: "/admin/users/:id", OperationID: "Admin_deleteUser",
					ReturnType: metadata.Metadata{Kind: metadata.KindVoid},
					StatusCode: 204, Tags: []string{"Admin"},
					Security: []analyzer.SecurityRequirement{{Name: "oauth2", Scopes: []string{"admin"}}},
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	data := requireValidDoc(t, doc)

	// First route should have bearer security
	assertJSONContains(t, data, `"bearer"`)
	// Second route should have oauth2 with admin scope
	assertJSONContains(t, data, `"oauth2"`)
	assertJSONContains(t, data, `"admin"`)
}

// --- 6. Tag descriptions ---

func TestFeature_TagDescriptions(t *testing.T) {
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "UserController", Path: "users",
			Routes: []analyzer.Route{
				{Method: "GET", Path: "/users", OperationID: "listUsers",
					ReturnType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					StatusCode: 200, Tags: []string{"User"}},
			},
		},
	}

	doc := gen.Generate(controllers)
	doc.ApplyConfig(DocumentConfig{
		Tags: []Tag{
			{Name: "User", Description: "User management operations"},
			{Name: "Admin", Description: "Administrative operations"},
		},
	})

	data := requireValidDoc(t, doc)

	// "User" tag should have description from config
	assertJSONContains(t, data, `"User management operations"`)
	// "Admin" tag should be added even though no routes use it
	assertJSONContains(t, data, `"Administrative operations"`)
}

// --- 7. Parameter descriptions ---

func TestFeature_ParameterDescription(t *testing.T) {
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "UserController", Path: "users",
			Routes: []analyzer.Route{
				{
					Method: "GET", Path: "/users/:id", OperationID: "User_getUser",
					Parameters: []analyzer.RouteParameter{
						{
							Category:    "param",
							Name:        "id",
							Type:        metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
							Required:    true,
							Description: "The user's unique identifier",
						},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					StatusCode: 200, Tags: []string{"User"},
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	data := requireValidDoc(t, doc)
	assertJSONContains(t, data, `"description": "The user's unique identifier"`)
}

// --- 8. Property descriptions ---

func TestFeature_PropertyDescription(t *testing.T) {
	registry := metadata.NewTypeRegistry()
	registry.Register("CreateUserDto", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "CreateUserDto",
		Properties: []metadata.Property{
			{
				Name: "email", Required: true,
				Type:        metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
				Description: "The user's email address",
			},
			{
				Name: "name", Required: true,
				Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
				// No description
			},
		},
	})

	gen := NewGenerator(registry)
	controllers := []analyzer.ControllerInfo{
		{
			Name: "UserController", Path: "users",
			Routes: []analyzer.Route{
				{
					Method: "POST", Path: "/users", OperationID: "User_createUser",
					Parameters: []analyzer.RouteParameter{
						{Category: "body", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "CreateUserDto"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindVoid},
					StatusCode: 201, Tags: []string{"User"},
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	data := requireValidDoc(t, doc)
	assertJSONContains(t, data, `"description": "The user's email address"`)
}

// --- 9. termsOfService ---

func TestFeature_TermsOfService(t *testing.T) {
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	doc := gen.Generate(nil)
	doc.ApplyConfig(DocumentConfig{
		Title:          "My API",
		TermsOfService: "https://example.com/tos",
	})

	data, err := doc.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	assertJSONContains(t, data, `"termsOfService": "https://example.com/tos"`)
}

// --- 10. writeOnly ---

func TestFeature_WriteOnly(t *testing.T) {
	registry := metadata.NewTypeRegistry()
	registry.Register("CreateUserDto", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "CreateUserDto",
		Properties: []metadata.Property{
			{
				Name: "password", Required: true,
				Type:      metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
				WriteOnly: true,
			},
		},
	})

	gen := NewGenerator(registry)
	controllers := []analyzer.ControllerInfo{
		{
			Name: "UserController", Path: "users",
			Routes: []analyzer.Route{
				{
					Method: "POST", Path: "/users", OperationID: "User_createUser",
					Parameters: []analyzer.RouteParameter{
						{Category: "body", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "CreateUserDto"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindVoid},
					StatusCode: 201, Tags: []string{"User"},
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	data := requireValidDoc(t, doc)
	assertJSONContains(t, data, `"writeOnly": true`)
}

// --- 11. Schema examples ---

func TestFeature_PropertyExample(t *testing.T) {
	registry := metadata.NewTypeRegistry()
	email := "user@example.com"
	registry.Register("CreateUserDto", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "CreateUserDto",
		Properties: []metadata.Property{
			{
				Name: "email", Required: true,
				Type:    metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
				Example: &email,
			},
		},
	})

	gen := NewGenerator(registry)
	controllers := []analyzer.ControllerInfo{
		{
			Name: "UserController", Path: "users",
			Routes: []analyzer.Route{
				{
					Method: "POST", Path: "/users", OperationID: "User_createUser",
					Parameters: []analyzer.RouteParameter{
						{Category: "body", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "CreateUserDto"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindVoid},
					StatusCode: 201, Tags: []string{"User"},
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	data := requireValidDoc(t, doc)
	assertJSONContains(t, data, `"example": "user@example.com"`)
}

// --- 12. Vendor extensions ---

func TestFeature_VendorExtensions(t *testing.T) {
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "UserController", Path: "users",
			Routes: []analyzer.Route{
				{
					Method: "GET", Path: "/users", OperationID: "User_findAll",
					ReturnType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					StatusCode: 200, Tags: []string{"User"},
					Extensions: map[string]string{
						"x-internal": "true",
						"x-codegen":  "UserListResponse",
					},
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	data, err := doc.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	assertJSONContains(t, data, `"x-internal": "true"`)
	assertJSONContains(t, data, `"x-codegen": "UserListResponse"`)
}

// --- 13. Multiple response codes ---

func TestFeature_MultipleResponseCodes(t *testing.T) {
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "UserController", Path: "users",
			Routes: []analyzer.Route{
				{
					Method: "POST", Path: "/users", OperationID: "User_createUser",
					ReturnType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					StatusCode: 201,
					Tags:       []string{"User"},
					AdditionalResponses: []analyzer.AdditionalResponse{
						{
							StatusCode:  200,
							ReturnType:  metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
							Description: "User already exists",
						},
					},
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	data := requireValidDoc(t, doc)

	raw := parseJSON(t, data)
	paths := raw["paths"].(map[string]any)
	usersPath := paths["/users"].(map[string]any)
	post := usersPath["post"].(map[string]any)
	responses := post["responses"].(map[string]any)

	if _, ok := responses["201"]; !ok {
		t.Error("expected 201 response")
	}
	if _, ok := responses["200"]; !ok {
		t.Error("expected 200 additional response")
	}

	resp200 := responses["200"].(map[string]any)
	if desc, ok := resp200["description"].(string); !ok || desc != "User already exists" {
		t.Errorf("expected description 'User already exists', got %q", desc)
	}
}

// --- Combined: realistic controller with all features ---

func TestFeature_RealisticController_AllFeatures(t *testing.T) {
	registry := metadata.NewTypeRegistry()

	email := "user@example.com"
	registry.Register("CreateUserDto", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "CreateUserDto",
		Properties: []metadata.Property{
			{Name: "email", Required: true,
				Type:        metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
				Description: "User's email address",
				Example:     &email,
				Constraints: &metadata.Constraints{Format: strPtr("email")}},
			{Name: "password", Required: true,
				Type:        metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
				WriteOnly:   true,
				Constraints: &metadata.Constraints{MinLength: intPtr(8)}},
			{Name: "name", Required: true,
				Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}},
		},
	})

	registry.Register("UserResponse", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "UserResponse",
		Properties: []metadata.Property{
			{Name: "id", Required: true, Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"},
				Readonly: true},
			{Name: "email", Required: true, Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}},
			{Name: "name", Required: true, Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}},
		},
	})

	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "UserController", Path: "users",
			Routes: []analyzer.Route{
				{
					Method: "GET", Path: "/users", OperationID: "User_findAll",
					Summary: "List all users", Description: "Returns a paginated list of users",
					ReturnType: metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindRef, Ref: "UserResponse"}},
					StatusCode: 200, Tags: []string{"User"},
					// Inherits global security
				},
				{
					Method: "GET", Path: "/users/:id", OperationID: "User_findOne",
					Summary: "Get a user by ID",
					Parameters: []analyzer.RouteParameter{
						{Category: "param", Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
							Required: true, Description: "The user's unique identifier"},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "UserResponse"},
					StatusCode: 200, Tags: []string{"User"},
					ErrorResponses: []analyzer.ErrorResponse{
						{StatusCode: 404, TypeName: "NotFound", Description: "User not found"},
					},
					Extensions: map[string]string{"x-cache": "60s"},
				},
				{
					Method: "POST", Path: "/users", OperationID: "User_create",
					Summary: "Create a new user",
					Parameters: []analyzer.RouteParameter{
						{Category: "body", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "CreateUserDto"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "UserResponse"},
					StatusCode: 201, Tags: []string{"User"},
					ErrorResponses: []analyzer.ErrorResponse{
						{StatusCode: 409, TypeName: "Conflict", Description: "User with this email already exists"},
						{StatusCode: 400, TypeName: "BadRequest"},
					},
				},
				{
					Method: "POST", Path: "/users/login", OperationID: "User_login",
					Summary:    "Login",
					ReturnType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					StatusCode: 200, Tags: []string{"Auth"},
					IsPublic: true, // No auth needed
				},
			},
		},
	}

	doc := gen.GenerateWithOptions(controllers, nil)
	doc.ApplyConfig(DocumentConfig{
		Title:          "User Service API",
		Version:        "2.0.0",
		TermsOfService: "https://example.com/terms",
		Security:       []map[string][]string{{"bearer": {}}},
		SecuritySchemes: map[string]*SecurityScheme{
			"bearer": {Type: "http", Scheme: "bearer", BearerFormat: "JWT"},
		},
		Tags: []Tag{
			{Name: "User", Description: "User management endpoints"},
			{Name: "Auth", Description: "Authentication endpoints"},
		},
	})

	data := requireValidDoc(t, doc)
	raw := parseJSON(t, data)

	// Check info
	info := raw["info"].(map[string]any)
	if info["title"] != "User Service API" {
		t.Errorf("expected title 'User Service API', got %v", info["title"])
	}
	if info["termsOfService"] != "https://example.com/terms" {
		t.Errorf("expected termsOfService, got %v", info["termsOfService"])
	}

	// Check global security
	docSec := raw["security"].([]any)
	if len(docSec) != 1 {
		t.Errorf("expected 1 global security entry, got %d", len(docSec))
	}

	// Check tag descriptions
	tags := raw["tags"].([]any)
	tagFound := false
	for _, tag := range tags {
		tagObj := tag.(map[string]any)
		if tagObj["name"] == "User" && tagObj["description"] == "User management endpoints" {
			tagFound = true
		}
	}
	if !tagFound {
		t.Error("expected User tag with description")
	}

	// Check login route is public (empty security array)
	paths := raw["paths"].(map[string]any)
	loginPath := paths["/users/login"].(map[string]any)
	loginOp := loginPath["post"].(map[string]any)
	loginSec := loginOp["security"].([]any)
	if len(loginSec) != 0 {
		t.Errorf("expected empty security on public route, got %v", loginSec)
	}

	// Check findOne has parameter description
	assertJSONContains(t, data, `"description": "The user's unique identifier"`)

	// Check findOne has vendor extension
	assertJSONContains(t, data, `"x-cache": "60s"`)

	// Check custom error descriptions
	assertJSONContains(t, data, `"User not found"`)
	assertJSONContains(t, data, `"User with this email already exists"`)

	// Check property annotations in CreateUserDto schema
	assertJSONContains(t, data, `"description": "User's email address"`)
	assertJSONContains(t, data, `"example": "user@example.com"`)
	assertJSONContains(t, data, `"writeOnly": true`)

	// Check operationId format
	assertJSONContains(t, data, `"operationId": "User_findAll"`)
	assertJSONContains(t, data, `"operationId": "User_findOne"`)
	assertJSONContains(t, data, `"operationId": "User_create"`)
	assertJSONContains(t, data, `"operationId": "User_login"`)
}

// --- parseThrowsTag unit tests ---

func TestFeature_ParseThrowsTag_WithDescription(t *testing.T) {
	// Test via analyzer package — but since parseThrowsTag is not exported,
	// we test through the generator using pre-constructed ErrorResponse with Description.
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "TestController", Path: "test",
			Routes: []analyzer.Route{
				{
					Method: "GET", Path: "/test", OperationID: "test",
					ReturnType: metadata.Metadata{Kind: metadata.KindVoid},
					StatusCode: 200, Tags: []string{"Test"},
					ErrorResponses: []analyzer.ErrorResponse{
						{StatusCode: 404, TypeName: "NotFoundError", Description: "The requested resource does not exist"},
						{StatusCode: 422, TypeName: "ValidationError"}, // no description
					},
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	data := requireValidDoc(t, doc)

	// Verify custom description is used for 404
	var raw map[string]any
	json.Unmarshal(data, &raw)
	paths := raw["paths"].(map[string]any)
	testPath := paths["/test"].(map[string]any)
	getOp := testPath["get"].(map[string]any)
	responses := getOp["responses"].(map[string]any)

	resp404 := responses["404"].(map[string]any)
	if resp404["description"] != "The requested resource does not exist" {
		t.Errorf("expected custom 404 description, got %v", resp404["description"])
	}

	resp422 := responses["422"].(map[string]any)
	if resp422["description"] != "OK" { // default since 422 isn't in statusDescription map
		t.Errorf("expected default 422 description, got %v", resp422["description"])
	}
}

// --- Helpers ---

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

// assertJSONNotContains checks that the JSON string does NOT contain a substring.
func assertJSONNotContains(t *testing.T, data []byte, substr string) {
	t.Helper()
	if strings.Contains(string(data), substr) {
		t.Errorf("expected JSON to NOT contain %q", substr)
	}
}
