package openapi

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tsgonest/tsgonest/internal/analyzer"
	"github.com/tsgonest/tsgonest/internal/metadata"
)

// ---- Helpers ----

func intgStrPtr(s string) *string     { return &s }
func intgIntPtr(i int) *int           { return &i }
func intgFloatPtr(f float64) *float64 { return &f }

// requireValidDoc validates a Document passes compliance and returns the JSON bytes.
func requireValidDoc(t *testing.T, doc *Document) []byte {
	t.Helper()
	errs := ValidateDocument(doc)
	if len(errs) > 0 {
		t.Errorf("document has %d compliance errors:", len(errs))
		for _, e := range errs {
			t.Errorf("  %s", e.Error())
		}
		t.FailNow()
	}
	data, err := doc.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}
	return data
}

// parseJSON unmarshals JSON into a generic map.
func parseJSON(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	return raw
}

// assertJSONContains checks that the JSON string contains a substring.
func assertJSONContains(t *testing.T, data []byte, substr string) {
	t.Helper()
	if !strings.Contains(string(data), substr) {
		t.Errorf("expected JSON to contain %q", substr)
	}
}

// ---- Integration Tests ----

func TestIntegration_CRUDController(t *testing.T) {
	// Full CRUD controller with body, param, query params
	registry := metadata.NewTypeRegistry()

	registry.Register("CreateItemDto", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "CreateItemDto",
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{MinLength: intgIntPtr(1), MaxLength: intgIntPtr(200)}},
			{Name: "description", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Optional: true}, Required: false},
			{Name: "price", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true,
				Constraints: &metadata.Constraints{Minimum: intgFloatPtr(0)}},
			{Name: "tags", Type: metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}}, Required: false},
		},
	})

	registry.Register("UpdateItemDto", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "UpdateItemDto",
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Optional: true}, Required: false},
			{Name: "description", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Optional: true}, Required: false},
			{Name: "price", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number", Optional: true}, Required: false},
		},
	})

	registry.Register("ItemResponse", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "ItemResponse",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "description", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Optional: true}, Required: false},
			{Name: "price", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "tags", Type: metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}}, Required: false},
			{Name: "createdAt", Type: metadata.Metadata{Kind: metadata.KindNative, NativeType: "Date"}, Required: true},
		},
	})

	registry.Register("ListQuery", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "ListQuery",
		Properties: []metadata.Property{
			{Name: "page", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: false},
			{Name: "limit", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: false},
			{Name: "search", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: false},
			{Name: "sortBy", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: false},
		},
	})

	gen := NewGenerator(registry)
	controllers := []analyzer.ControllerInfo{
		{
			Name: "ItemController",
			Path: "items",
			Routes: []analyzer.Route{
				{
					Method:      "GET",
					Path:        "/items",
					OperationID: "listItems",
					Summary:     "List all items",
					Description: "Returns a paginated list of items",
					Parameters: []analyzer.RouteParameter{
						{Category: "query", Name: "", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "ListQuery"}, Required: false},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindRef, Ref: "ItemResponse"}},
					StatusCode: 200,
					Tags:       []string{"Item"},
				},
				{
					Method:      "GET",
					Path:        "/items/:id",
					OperationID: "getItem",
					Summary:     "Get a single item by ID",
					Parameters: []analyzer.RouteParameter{
						{Category: "param", Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "ItemResponse"},
					StatusCode: 200,
					Tags:       []string{"Item"},
					ErrorResponses: []analyzer.ErrorResponse{
						{StatusCode: 404, TypeName: "", Type: metadata.Metadata{Kind: ""}},
					},
				},
				{
					Method:      "POST",
					Path:        "/items",
					OperationID: "createItem",
					Summary:     "Create a new item",
					Parameters: []analyzer.RouteParameter{
						{Category: "body", Name: "", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "CreateItemDto"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "ItemResponse"},
					StatusCode: 201,
					Tags:       []string{"Item"},
					ErrorResponses: []analyzer.ErrorResponse{
						{StatusCode: 400, TypeName: "", Type: metadata.Metadata{Kind: ""}},
					},
				},
				{
					Method:      "PATCH",
					Path:        "/items/:id",
					OperationID: "updateItem",
					Summary:     "Update an existing item",
					Parameters: []analyzer.RouteParameter{
						{Category: "param", Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
						{Category: "body", Name: "", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "UpdateItemDto"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "ItemResponse"},
					StatusCode: 200,
					Tags:       []string{"Item"},
				},
				{
					Method:      "DELETE",
					Path:        "/items/:id",
					OperationID: "deleteItem",
					Summary:     "Delete an item",
					Parameters: []analyzer.RouteParameter{
						{Category: "param", Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindVoid},
					StatusCode: 204,
					Tags:       []string{"Item"},
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	data := requireValidDoc(t, doc)

	// Verify all CRUD paths
	raw := parseJSON(t, data)
	paths := raw["paths"].(map[string]any)

	if _, ok := paths["/items"]; !ok {
		t.Error("expected /items path")
	}
	if _, ok := paths["/items/{id}"]; !ok {
		t.Error("expected /items/{id} path")
	}

	// Verify /items has GET and POST
	itemsPath := paths["/items"].(map[string]any)
	if _, ok := itemsPath["get"]; !ok {
		t.Error("expected GET on /items")
	}
	if _, ok := itemsPath["post"]; !ok {
		t.Error("expected POST on /items")
	}

	// Verify /items/{id} has GET, PATCH, DELETE
	itemsIdPath := paths["/items/{id}"].(map[string]any)
	if _, ok := itemsIdPath["get"]; !ok {
		t.Error("expected GET on /items/{id}")
	}
	if _, ok := itemsIdPath["patch"]; !ok {
		t.Error("expected PATCH on /items/{id}")
	}
	if _, ok := itemsIdPath["delete"]; !ok {
		t.Error("expected DELETE on /items/{id}")
	}

	// Verify query param decomposition on GET /items
	getOp := itemsPath["get"].(map[string]any)
	params, ok := getOp["parameters"].([]any)
	if !ok || len(params) < 3 {
		t.Errorf("expected at least 3 decomposed query params on GET /items, got %v", len(params))
	}

	// Verify components
	components := raw["components"].(map[string]any)
	schemas := components["schemas"].(map[string]any)
	for _, name := range []string{"CreateItemDto", "UpdateItemDto", "ItemResponse"} {
		if _, ok := schemas[name]; !ok {
			t.Errorf("expected schema %q in components", name)
		}
	}

	// Verify error responses present
	getItemOp := itemsIdPath["get"].(map[string]any)
	getItemResps := getItemOp["responses"].(map[string]any)
	if _, ok := getItemResps["404"]; !ok {
		t.Error("expected 404 error response on GET /items/{id}")
	}

	// Verify summary on operations
	assertJSONContains(t, data, `"summary"`)
	assertJSONContains(t, data, `"List all items"`)
}

func TestIntegration_AuthController(t *testing.T) {
	// Controller with security, @throws for 401
	registry := metadata.NewTypeRegistry()

	registry.Register("LoginDto", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "LoginDto",
		Properties: []metadata.Property{
			{Name: "email", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Format: intgStrPtr("email")}},
			{Name: "password", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{MinLength: intgIntPtr(8)}},
		},
	})

	registry.Register("TokenResponse", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "TokenResponse",
		Properties: []metadata.Property{
			{Name: "accessToken", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "refreshToken", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "expiresIn", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
		},
	})

	registry.Register("UserProfile", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "UserProfile",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "email", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "role", Type: metadata.Metadata{Kind: metadata.KindUnion, UnionMembers: []metadata.Metadata{
				{Kind: metadata.KindLiteral, LiteralValue: "admin"},
				{Kind: metadata.KindLiteral, LiteralValue: "user"},
			}}, Required: true},
		},
	})

	registry.Register("UnauthorizedError", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "UnauthorizedError",
		Properties: []metadata.Property{
			{Name: "message", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "statusCode", Type: metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: float64(401)}, Required: true},
		},
	})

	gen := NewGenerator(registry)
	controllers := []analyzer.ControllerInfo{
		{
			Name: "AuthController",
			Path: "auth",
			Routes: []analyzer.Route{
				{
					Method:      "POST",
					Path:        "/auth/login",
					OperationID: "login",
					Summary:     "Authenticate user",
					Parameters: []analyzer.RouteParameter{
						{Category: "body", Name: "", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "LoginDto"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "TokenResponse"},
					StatusCode: 200,
					Tags:       []string{"Auth"},
					ErrorResponses: []analyzer.ErrorResponse{
						{StatusCode: 401, TypeName: "UnauthorizedError", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "UnauthorizedError"}},
					},
				},
				{
					Method:      "GET",
					Path:        "/auth/profile",
					OperationID: "getProfile",
					Summary:     "Get current user profile",
					ReturnType:  metadata.Metadata{Kind: metadata.KindRef, Ref: "UserProfile"},
					StatusCode:  200,
					Tags:        []string{"Auth"},
					Security: []analyzer.SecurityRequirement{
						{Name: "bearer", Scopes: []string{}},
					},
					ErrorResponses: []analyzer.ErrorResponse{
						{StatusCode: 401, TypeName: "UnauthorizedError", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "UnauthorizedError"}},
					},
				},
				{
					Method:      "POST",
					Path:        "/auth/logout",
					OperationID: "logout",
					Summary:     "Logout current user",
					ReturnType:  metadata.Metadata{Kind: metadata.KindVoid},
					StatusCode:  204,
					Tags:        []string{"Auth"},
					Security: []analyzer.SecurityRequirement{
						{Name: "bearer", Scopes: []string{}},
					},
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	doc.ApplyConfig(DocumentConfig{
		Title:       "Auth API",
		Description: "Authentication service",
		Version:     "1.0.0",
		SecuritySchemes: map[string]*SecurityScheme{
			"bearer": {Type: "http", Scheme: "bearer", BearerFormat: "JWT"},
		},
	})

	data := requireValidDoc(t, doc)

	// Verify security requirement on profile endpoint
	assertJSONContains(t, data, `"security"`)
	assertJSONContains(t, data, `"bearer"`)

	// Verify 401 error responses
	raw := parseJSON(t, data)
	paths := raw["paths"].(map[string]any)

	loginPath := paths["/auth/login"].(map[string]any)
	loginOp := loginPath["post"].(map[string]any)
	loginResps := loginOp["responses"].(map[string]any)
	if _, ok := loginResps["401"]; !ok {
		t.Error("expected 401 response on POST /auth/login")
	}

	profilePath := paths["/auth/profile"].(map[string]any)
	profileOp := profilePath["get"].(map[string]any)
	profileResps := profileOp["responses"].(map[string]any)
	if _, ok := profileResps["401"]; !ok {
		t.Error("expected 401 response on GET /auth/profile")
	}

	// Verify security schemes in components
	components := raw["components"].(map[string]any)
	secSchemes := components["securitySchemes"].(map[string]any)
	if _, ok := secSchemes["bearer"]; !ok {
		t.Error("expected bearer security scheme")
	}

	// Verify the 401 response has the UnauthorizedError schema
	resp401 := loginResps["401"].(map[string]any)
	content401 := resp401["content"].(map[string]any)
	jsonContent := content401["application/json"].(map[string]any)
	schema := jsonContent["schema"].(map[string]any)
	if ref, ok := schema["$ref"].(string); !ok || !strings.Contains(ref, "UnauthorizedError") {
		t.Errorf("expected 401 response schema to reference UnauthorizedError, got %v", schema)
	}
}

func TestIntegration_VersionedAPI(t *testing.T) {
	// Multiple versions of the same endpoint
	registry := metadata.NewTypeRegistry()

	registry.Register("UserV1", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "UserV1",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	})

	registry.Register("UserV2", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "UserV2",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true}, // Changed to UUID
			{Name: "firstName", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "lastName", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "email", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
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
					OperationID: "listUsersV1",
					ReturnType:  metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindRef, Ref: "UserV1"}},
					StatusCode:  200,
					Tags:        []string{"User"},
					Version:     "1",
				},
				{
					Method:      "GET",
					Path:        "/users",
					OperationID: "listUsersV2",
					ReturnType:  metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindRef, Ref: "UserV2"}},
					StatusCode:  200,
					Tags:        []string{"User"},
					Version:     "2",
				},
				{
					Method:      "GET",
					Path:        "/users/:id",
					OperationID: "getUserV1",
					Parameters: []analyzer.RouteParameter{
						{Category: "param", Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "UserV1"},
					StatusCode: 200,
					Tags:       []string{"User"},
					Version:    "1",
				},
				{
					Method:      "GET",
					Path:        "/users/:id",
					OperationID: "getUserV2",
					Parameters: []analyzer.RouteParameter{
						{Category: "param", Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "UserV2"},
					StatusCode: 200,
					Tags:       []string{"User"},
					Version:    "2",
				},
			},
		},
	}

	doc := gen.GenerateWithOptions(controllers, &GenerateOptions{
		GlobalPrefix:   "api",
		VersioningType: "URI",
		VersionPrefix:  "v",
	})

	data := requireValidDoc(t, doc)
	raw := parseJSON(t, data)
	paths := raw["paths"].(map[string]any)

	// Verify versioned paths exist
	expectedPaths := []string{"/api/v1/users", "/api/v2/users", "/api/v1/users/{id}", "/api/v2/users/{id}"}
	for _, p := range expectedPaths {
		if _, ok := paths[p]; !ok {
			t.Errorf("expected versioned path %q, got paths: %v", p, getKeys(paths))
		}
	}

	// Verify both schemas registered
	components := raw["components"].(map[string]any)
	schemas := components["schemas"].(map[string]any)
	if _, ok := schemas["UserV1"]; !ok {
		t.Error("expected UserV1 schema")
	}
	if _, ok := schemas["UserV2"]; !ok {
		t.Error("expected UserV2 schema")
	}
}

func TestIntegration_NestedDTOs(t *testing.T) {
	// DTOs with nested objects, arrays of objects, refs
	registry := metadata.NewTypeRegistry()

	registry.Register("Address", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "Address",
		Properties: []metadata.Property{
			{Name: "street", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "city", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "state", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "zip", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Pattern: intgStrPtr("^\\d{5}(-\\d{4})?$")}},
			{Name: "country", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	})

	registry.Register("PhoneNumber", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "PhoneNumber",
		Properties: []metadata.Property{
			{Name: "type", Type: metadata.Metadata{Kind: metadata.KindUnion, UnionMembers: []metadata.Metadata{
				{Kind: metadata.KindLiteral, LiteralValue: "home"},
				{Kind: metadata.KindLiteral, LiteralValue: "work"},
				{Kind: metadata.KindLiteral, LiteralValue: "mobile"},
			}}, Required: true},
			{Name: "number", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	})

	registry.Register("CreateContactDto", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "CreateContactDto",
		Properties: []metadata.Property{
			{Name: "firstName", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "lastName", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "email", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Format: intgStrPtr("email")}},
			{Name: "address", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "Address"}, Required: true},
			{Name: "phones", Type: metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindRef, Ref: "PhoneNumber"}}, Required: false},
			{Name: "notes", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Optional: true}, Required: false},
		},
	})

	registry.Register("ContactResponse", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "ContactResponse",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "firstName", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "lastName", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "email", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "address", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "Address"}, Required: true},
			{Name: "phones", Type: metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindRef, Ref: "PhoneNumber"}}, Required: false},
		},
	})

	gen := NewGenerator(registry)
	controllers := []analyzer.ControllerInfo{
		{
			Name: "ContactController",
			Path: "contacts",
			Routes: []analyzer.Route{
				{
					Method:      "POST",
					Path:        "/contacts",
					OperationID: "createContact",
					Parameters: []analyzer.RouteParameter{
						{Category: "body", Name: "", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "CreateContactDto"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "ContactResponse"},
					StatusCode: 201,
					Tags:       []string{"Contact"},
				},
				{
					Method:      "GET",
					Path:        "/contacts/:id",
					OperationID: "getContact",
					Parameters: []analyzer.RouteParameter{
						{Category: "param", Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "ContactResponse"},
					StatusCode: 200,
					Tags:       []string{"Contact"},
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	data := requireValidDoc(t, doc)
	raw := parseJSON(t, data)

	// Verify all nested schemas are registered
	components := raw["components"].(map[string]any)
	schemas := components["schemas"].(map[string]any)
	expectedSchemas := []string{"Address", "PhoneNumber", "CreateContactDto", "ContactResponse"}
	for _, name := range expectedSchemas {
		if _, ok := schemas[name]; !ok {
			t.Errorf("expected schema %q in components", name)
		}
	}

	// Verify Address schema has pattern constraint
	addrSchema := schemas["Address"].(map[string]any)
	props := addrSchema["properties"].(map[string]any)
	zipProp := props["zip"].(map[string]any)
	if _, ok := zipProp["pattern"]; !ok {
		t.Error("expected pattern constraint on Address.zip")
	}

	// Verify PhoneNumber has enum for type
	phoneSchema := schemas["PhoneNumber"].(map[string]any)
	phoneProps := phoneSchema["properties"].(map[string]any)
	typeProp := phoneProps["type"].(map[string]any)
	if _, ok := typeProp["enum"]; !ok {
		t.Error("expected enum on PhoneNumber.type")
	}

	// Verify CreateContactDto.address is a $ref
	createSchema := schemas["CreateContactDto"].(map[string]any)
	createProps := createSchema["properties"].(map[string]any)
	addrProp := createProps["address"].(map[string]any)
	if ref, ok := addrProp["$ref"].(string); !ok || !strings.Contains(ref, "Address") {
		t.Errorf("expected $ref to Address in CreateContactDto.address, got %v", addrProp)
	}
}

func TestIntegration_PaginatedList(t *testing.T) {
	// Generic-like paginated response with metadata
	registry := metadata.NewTypeRegistry()

	registry.Register("PaginationMeta", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "PaginationMeta",
		Properties: []metadata.Property{
			{Name: "total", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "page", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "limit", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "totalPages", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "hasNext", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "boolean"}, Required: true},
			{Name: "hasPrev", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "boolean"}, Required: true},
		},
	})

	registry.Register("Product", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "Product",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "price", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true,
				Constraints: &metadata.Constraints{Minimum: intgFloatPtr(0)}},
			{Name: "sku", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Pattern: intgStrPtr("^[A-Z]{2,4}-\\d{4,8}$")}},
		},
	})

	// PaginatedProductList is an inline object (no ref pattern for generics)
	registry.Register("PaginatedProductList", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "PaginatedProductList",
		Properties: []metadata.Property{
			{Name: "data", Type: metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindRef, Ref: "Product"}}, Required: true},
			{Name: "meta", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "PaginationMeta"}, Required: true},
		},
	})

	gen := NewGenerator(registry)
	controllers := []analyzer.ControllerInfo{
		{
			Name: "ProductController",
			Path: "products",
			Routes: []analyzer.Route{
				{
					Method:      "GET",
					Path:        "/products",
					OperationID: "listProducts",
					Summary:     "List products with pagination",
					Parameters: []analyzer.RouteParameter{
						{Category: "query", Name: "page", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: false},
						{Category: "query", Name: "limit", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: false},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "PaginatedProductList"},
					StatusCode: 200,
					Tags:       []string{"Product"},
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	data := requireValidDoc(t, doc)
	raw := parseJSON(t, data)

	// Verify all schemas registered
	components := raw["components"].(map[string]any)
	schemas := components["schemas"].(map[string]any)
	for _, name := range []string{"PaginationMeta", "Product", "PaginatedProductList"} {
		if _, ok := schemas[name]; !ok {
			t.Errorf("expected schema %q", name)
		}
	}

	// Verify pagination response shape
	paginatedSchema := schemas["PaginatedProductList"].(map[string]any)
	paginatedProps := paginatedSchema["properties"].(map[string]any)

	// data should be array of $ref Product
	dataProp := paginatedProps["data"].(map[string]any)
	if dataProp["type"] != "array" {
		t.Errorf("expected data type=array, got %v", dataProp["type"])
	}

	// meta should be $ref to PaginationMeta
	metaProp := paginatedProps["meta"].(map[string]any)
	if ref, ok := metaProp["$ref"].(string); !ok || !strings.Contains(ref, "PaginationMeta") {
		t.Errorf("expected meta to ref PaginationMeta, got %v", metaProp)
	}

	// Verify query params
	paths := raw["paths"].(map[string]any)
	productsPath := paths["/products"].(map[string]any)
	getOp := productsPath["get"].(map[string]any)
	params := getOp["parameters"].([]any)
	if len(params) != 2 {
		t.Errorf("expected 2 query params, got %d", len(params))
	}
}

func TestIntegration_FileUpload(t *testing.T) {
	// multipart/form-data endpoint
	registry := metadata.NewTypeRegistry()

	registry.Register("UploadDto", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "UploadDto",
		Properties: []metadata.Property{
			{Name: "file", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "description", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Optional: true}, Required: false},
			{Name: "tags", Type: metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}}, Required: false},
		},
	})

	registry.Register("UploadResponse", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "UploadResponse",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "url", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Format: intgStrPtr("url")}},
			{Name: "size", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "mimeType", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	})

	gen := NewGenerator(registry)
	controllers := []analyzer.ControllerInfo{
		{
			Name: "UploadController",
			Path: "uploads",
			Routes: []analyzer.Route{
				{
					Method:      "POST",
					Path:        "/uploads",
					OperationID: "uploadFile",
					Summary:     "Upload a file",
					Parameters: []analyzer.RouteParameter{
						{
							Category:    "body",
							Name:        "",
							Type:        metadata.Metadata{Kind: metadata.KindRef, Ref: "UploadDto"},
							Required:    true,
							ContentType: "multipart/form-data",
						},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "UploadResponse"},
					StatusCode: 201,
					Tags:       []string{"Upload"},
				},
				{
					Method:      "GET",
					Path:        "/uploads/:id",
					OperationID: "getUpload",
					Parameters: []analyzer.RouteParameter{
						{Category: "param", Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "UploadResponse"},
					StatusCode: 200,
					Tags:       []string{"Upload"},
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	data := requireValidDoc(t, doc)
	raw := parseJSON(t, data)

	// Verify POST /uploads uses multipart/form-data
	paths := raw["paths"].(map[string]any)
	uploadsPath := paths["/uploads"].(map[string]any)
	postOp := uploadsPath["post"].(map[string]any)
	reqBody := postOp["requestBody"].(map[string]any)
	content := reqBody["content"].(map[string]any)

	if _, ok := content["multipart/form-data"]; !ok {
		t.Errorf("expected multipart/form-data content type, got keys: %v", getKeys(content))
	}
	if _, ok := content["application/json"]; ok {
		t.Error("should NOT have application/json for multipart upload")
	}
}

func TestIntegration_SearchEndpoint(t *testing.T) {
	// Complex query parameter decomposition
	registry := metadata.NewTypeRegistry()

	registry.Register("SearchQuery", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "SearchQuery",
		Properties: []metadata.Property{
			{Name: "q", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "category", Type: metadata.Metadata{Kind: metadata.KindUnion, UnionMembers: []metadata.Metadata{
				{Kind: metadata.KindLiteral, LiteralValue: "electronics"},
				{Kind: metadata.KindLiteral, LiteralValue: "clothing"},
				{Kind: metadata.KindLiteral, LiteralValue: "books"},
				{Kind: metadata.KindLiteral, LiteralValue: "all"},
			}}, Required: false},
			{Name: "minPrice", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: false},
			{Name: "maxPrice", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: false},
			{Name: "inStock", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "boolean"}, Required: false},
			{Name: "sortBy", Type: metadata.Metadata{Kind: metadata.KindUnion, UnionMembers: []metadata.Metadata{
				{Kind: metadata.KindLiteral, LiteralValue: "price"},
				{Kind: metadata.KindLiteral, LiteralValue: "name"},
				{Kind: metadata.KindLiteral, LiteralValue: "rating"},
			}}, Required: false},
			{Name: "order", Type: metadata.Metadata{Kind: metadata.KindUnion, UnionMembers: []metadata.Metadata{
				{Kind: metadata.KindLiteral, LiteralValue: "asc"},
				{Kind: metadata.KindLiteral, LiteralValue: "desc"},
			}}, Required: false},
		},
	})

	registry.Register("SearchResult", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "SearchResult",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "price", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "score", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
		},
	})

	gen := NewGenerator(registry)
	controllers := []analyzer.ControllerInfo{
		{
			Name: "SearchController",
			Path: "search",
			Routes: []analyzer.Route{
				{
					Method:      "GET",
					Path:        "/search",
					OperationID: "search",
					Summary:     "Search products",
					Parameters: []analyzer.RouteParameter{
						{Category: "query", Name: "", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "SearchQuery"}, Required: false},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindRef, Ref: "SearchResult"}},
					StatusCode: 200,
					Tags:       []string{"Search"},
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	data := requireValidDoc(t, doc)
	raw := parseJSON(t, data)

	// Verify query decomposition produced individual params
	paths := raw["paths"].(map[string]any)
	searchPath := paths["/search"].(map[string]any)
	getOp := searchPath["get"].(map[string]any)
	params := getOp["parameters"].([]any)

	// Should have 7 individual query parameters decomposed from SearchQuery
	if len(params) != 7 {
		t.Errorf("expected 7 decomposed query params, got %d", len(params))
	}

	// Verify parameter names
	paramNames := make(map[string]bool)
	for _, p := range params {
		param := p.(map[string]any)
		paramNames[param["name"].(string)] = true
		// All should be in=query
		if param["in"] != "query" {
			t.Errorf("expected in=query, got %v", param["in"])
		}
	}

	expectedNames := []string{"q", "category", "minPrice", "maxPrice", "inStock", "sortBy", "order"}
	for _, name := range expectedNames {
		if !paramNames[name] {
			t.Errorf("expected query param %q not found", name)
		}
	}

	// Verify 'q' is required (rest are optional)
	for _, p := range params {
		param := p.(map[string]any)
		if param["name"] == "q" {
			if param["required"] != true {
				t.Error("expected 'q' param to be required")
			}
		}
	}
}

func TestIntegration_WebhookCallback(t *testing.T) {
	// Discriminated union in request body
	registry := metadata.NewTypeRegistry()

	registry.Register("PaymentSucceeded", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "PaymentSucceeded",
		Properties: []metadata.Property{
			{Name: "event", Type: metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: "payment.succeeded"}, Required: true},
			{Name: "amount", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "currency", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "transactionId", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	})

	registry.Register("PaymentFailed", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "PaymentFailed",
		Properties: []metadata.Property{
			{Name: "event", Type: metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: "payment.failed"}, Required: true},
			{Name: "reason", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "errorCode", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	})

	registry.Register("RefundProcessed", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "RefundProcessed",
		Properties: []metadata.Property{
			{Name: "event", Type: metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: "refund.processed"}, Required: true},
			{Name: "originalTransactionId", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "refundAmount", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
		},
	})

	// Webhook event is a discriminated union
	webhookEventMeta := metadata.Metadata{
		Kind: metadata.KindUnion,
		UnionMembers: []metadata.Metadata{
			{Kind: metadata.KindRef, Ref: "PaymentSucceeded"},
			{Kind: metadata.KindRef, Ref: "PaymentFailed"},
			{Kind: metadata.KindRef, Ref: "RefundProcessed"},
		},
		Discriminant: &metadata.Discriminant{
			Property: "event",
			Mapping: map[string]int{
				"payment.succeeded": 0,
				"payment.failed":    1,
				"refund.processed":  2,
			},
		},
	}

	gen := NewGenerator(registry)
	controllers := []analyzer.ControllerInfo{
		{
			Name: "WebhookController",
			Path: "webhooks",
			Routes: []analyzer.Route{
				{
					Method:      "POST",
					Path:        "/webhooks/payment",
					OperationID: "handlePaymentWebhook",
					Summary:     "Handle payment webhook callback",
					Parameters: []analyzer.RouteParameter{
						{Category: "body", Name: "", Type: webhookEventMeta, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindVoid},
					StatusCode: 200,
					Tags:       []string{"Webhook"},
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	data := requireValidDoc(t, doc)

	// Verify discriminator in JSON output
	assertJSONContains(t, data, `"discriminator"`)
	assertJSONContains(t, data, `"propertyName"`)
	assertJSONContains(t, data, `"event"`)
	assertJSONContains(t, data, `"oneOf"`)

	// Verify all event schemas registered
	raw := parseJSON(t, data)
	components := raw["components"].(map[string]any)
	schemas := components["schemas"].(map[string]any)
	for _, name := range []string{"PaymentSucceeded", "PaymentFailed", "RefundProcessed"} {
		if _, ok := schemas[name]; !ok {
			t.Errorf("expected schema %q in components", name)
		}
	}

	// Verify the discriminator mapping
	paths := raw["paths"].(map[string]any)
	webhookPath := paths["/webhooks/payment"].(map[string]any)
	postOp := webhookPath["post"].(map[string]any)
	reqBody := postOp["requestBody"].(map[string]any)
	content := reqBody["content"].(map[string]any)
	jsonContent := content["application/json"].(map[string]any)
	bodySchema := jsonContent["schema"].(map[string]any)

	// Should have oneOf
	oneOf, ok := bodySchema["oneOf"].([]any)
	if !ok {
		t.Fatal("expected oneOf in webhook body schema")
	}
	if len(oneOf) != 3 {
		t.Errorf("expected 3 oneOf members, got %d", len(oneOf))
	}

	// Should have discriminator
	disc, ok := bodySchema["discriminator"].(map[string]any)
	if !ok {
		t.Fatal("expected discriminator in webhook body schema")
	}
	if disc["propertyName"] != "event" {
		t.Errorf("expected discriminator propertyName=event, got %v", disc["propertyName"])
	}
}

func TestIntegration_MultiControllerFullPipeline(t *testing.T) {
	// Comprehensive multi-controller test simulating a full API
	registry := metadata.NewTypeRegistry()

	// User DTOs
	registry.Register("User", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "User",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "email", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	})

	// Post DTOs
	registry.Register("CreatePostDto", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "CreatePostDto",
		Properties: []metadata.Property{
			{Name: "title", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{MinLength: intgIntPtr(1), MaxLength: intgIntPtr(200)}},
			{Name: "body", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "published", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "boolean"}, Required: false},
		},
	})

	registry.Register("PostResponse", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "PostResponse",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "title", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "body", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "published", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "boolean"}, Required: true},
			{Name: "author", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "User"}, Required: true},
			{Name: "createdAt", Type: metadata.Metadata{Kind: metadata.KindNative, NativeType: "Date"}, Required: true},
		},
	})

	// Comment DTOs
	registry.Register("CreateCommentDto", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "CreateCommentDto",
		Properties: []metadata.Property{
			{Name: "text", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{MinLength: intgIntPtr(1), MaxLength: intgIntPtr(1000)}},
		},
	})

	registry.Register("CommentResponse", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "CommentResponse",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "text", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "author", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "User"}, Required: true},
			{Name: "createdAt", Type: metadata.Metadata{Kind: metadata.KindNative, NativeType: "Date"}, Required: true},
		},
	})

	gen := NewGenerator(registry)
	controllers := []analyzer.ControllerInfo{
		{
			Name: "UserController",
			Path: "users",
			Routes: []analyzer.Route{
				{Method: "GET", Path: "/users", OperationID: "listUsers",
					ReturnType: metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindRef, Ref: "User"}},
					StatusCode: 200, Tags: []string{"User"}},
				{Method: "GET", Path: "/users/:id", OperationID: "getUser",
					Parameters: []analyzer.RouteParameter{
						{Category: "param", Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "User"},
					StatusCode: 200, Tags: []string{"User"}},
			},
		},
		{
			Name: "PostController",
			Path: "posts",
			Routes: []analyzer.Route{
				{Method: "GET", Path: "/posts", OperationID: "listPosts",
					ReturnType: metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindRef, Ref: "PostResponse"}},
					StatusCode: 200, Tags: []string{"Post"}},
				{Method: "POST", Path: "/posts", OperationID: "createPost",
					Parameters: []analyzer.RouteParameter{
						{Category: "body", Name: "", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "CreatePostDto"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "PostResponse"},
					StatusCode: 201, Tags: []string{"Post"}},
				{Method: "GET", Path: "/posts/:id", OperationID: "getPost",
					Parameters: []analyzer.RouteParameter{
						{Category: "param", Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "PostResponse"},
					StatusCode: 200, Tags: []string{"Post"}},
				{Method: "DELETE", Path: "/posts/:id", OperationID: "deletePost",
					Parameters: []analyzer.RouteParameter{
						{Category: "param", Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindVoid},
					StatusCode: 204, Tags: []string{"Post"},
					Security: []analyzer.SecurityRequirement{
						{Name: "bearer", Scopes: []string{}},
					}},
			},
		},
		{
			Name: "CommentController",
			Path: "comments",
			Routes: []analyzer.Route{
				{Method: "GET", Path: "/posts/:postId/comments", OperationID: "listComments",
					Parameters: []analyzer.RouteParameter{
						{Category: "param", Name: "postId", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindRef, Ref: "CommentResponse"}},
					StatusCode: 200, Tags: []string{"Comment"}},
				{Method: "POST", Path: "/posts/:postId/comments", OperationID: "createComment",
					Parameters: []analyzer.RouteParameter{
						{Category: "param", Name: "postId", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
						{Category: "body", Name: "", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "CreateCommentDto"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "CommentResponse"},
					StatusCode: 201, Tags: []string{"Comment"}},
			},
		},
	}

	doc := gen.Generate(controllers)
	doc.ApplyConfig(DocumentConfig{
		Title:       "Blog API",
		Description: "A full-featured blog API",
		Version:     "1.0.0",
		Servers: []Server{
			{URL: "https://api.example.com", Description: "Production"},
		},
		SecuritySchemes: map[string]*SecurityScheme{
			"bearer": {Type: "http", Scheme: "bearer", BearerFormat: "JWT"},
		},
	})

	data := requireValidDoc(t, doc)
	raw := parseJSON(t, data)

	// Verify top-level info
	info := raw["info"].(map[string]any)
	if info["title"] != "Blog API" {
		t.Errorf("expected title=Blog API, got %v", info["title"])
	}

	// Verify servers
	servers := raw["servers"].([]any)
	if len(servers) != 1 {
		t.Errorf("expected 1 server, got %d", len(servers))
	}

	// Verify all expected paths
	paths := raw["paths"].(map[string]any)
	expectedPaths := []string{"/users", "/users/{id}", "/posts", "/posts/{id}", "/posts/{postId}/comments"}
	for _, p := range expectedPaths {
		if _, ok := paths[p]; !ok {
			t.Errorf("expected path %q", p)
		}
	}

	// Verify all tags
	tags := raw["tags"].([]any)
	tagNames := make(map[string]bool)
	for _, tag := range tags {
		tagObj := tag.(map[string]any)
		tagNames[tagObj["name"].(string)] = true
	}
	for _, expected := range []string{"User", "Post", "Comment"} {
		if !tagNames[expected] {
			t.Errorf("expected tag %q", expected)
		}
	}

	// Verify all schemas
	components := raw["components"].(map[string]any)
	schemas := components["schemas"].(map[string]any)
	expectedSchemas := []string{"User", "CreatePostDto", "PostResponse", "CreateCommentDto", "CommentResponse"}
	for _, s := range expectedSchemas {
		if _, ok := schemas[s]; !ok {
			t.Errorf("expected schema %q", s)
		}
	}

	// Verify security on delete post
	postsIdPath := paths["/posts/{id}"].(map[string]any)
	deleteOp := postsIdPath["delete"].(map[string]any)
	security, ok := deleteOp["security"].([]any)
	if !ok || len(security) == 0 {
		t.Error("expected security on DELETE /posts/{id}")
	}

	// Verify JSON round-trip
	var roundTripped Document
	if err := json.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("failed to unmarshal generated JSON: %v", err)
	}
	data2, err := roundTripped.ToJSON()
	if err != nil {
		t.Fatalf("failed to re-serialize: %v", err)
	}
	if string(data) != string(data2) {
		t.Error("JSON round-trip produced different output")
	}
}

// ---- @Header() Response Headers in OpenAPI ----

func TestIntegration_ResponseHeaders(t *testing.T) {
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "CacheController",
			Path: "data",
			Routes: []analyzer.Route{
				{
					Method:      "GET",
					Path:        "/data",
					OperationID: "getData",
					ReturnType:  metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					StatusCode:  200,
					Tags:        []string{"Data"},
					ResponseHeaders: []analyzer.ResponseHeader{
						{Name: "Cache-Control", Value: "no-cache"},
						{Name: "X-Request-Id", Value: "abc123"},
					},
				},
			},
		},
	}

	doc := gen.GenerateWithOptions(controllers, nil)
	data := requireValidDoc(t, doc)

	raw := parseJSON(t, data)
	paths := raw["paths"].(map[string]any)
	pathItem := paths["/data"].(map[string]any)
	getOp := pathItem["get"].(map[string]any)
	responses := getOp["responses"].(map[string]any)
	resp200 := responses["200"].(map[string]any)

	headers, ok := resp200["headers"].(map[string]any)
	if !ok {
		t.Fatal("expected headers in 200 response")
	}

	if _, ok := headers["Cache-Control"]; !ok {
		t.Error("expected Cache-Control header")
	}
	if _, ok := headers["X-Request-Id"]; !ok {
		t.Error("expected X-Request-Id header")
	}

	// Verify header schema has enum with the static value
	cacheHeader := headers["Cache-Control"].(map[string]any)
	schema := cacheHeader["schema"].(map[string]any)
	enumVal := schema["enum"].([]any)
	if len(enumVal) != 1 || enumVal[0] != "no-cache" {
		t.Errorf("expected enum=['no-cache'], got %v", enumVal)
	}
}

// ---- @Redirect() in OpenAPI ----

func TestIntegration_Redirect(t *testing.T) {
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "AuthController",
			Path: "auth",
			Routes: []analyzer.Route{
				{
					Method:      "GET",
					Path:        "/auth/google",
					OperationID: "googleLogin",
					ReturnType:  metadata.Metadata{Kind: metadata.KindVoid},
					StatusCode:  200,
					Tags:        []string{"Auth"},
					Redirect: &analyzer.RedirectInfo{
						URL:        "https://accounts.google.com",
						StatusCode: 301,
					},
				},
			},
		},
	}

	doc := gen.GenerateWithOptions(controllers, nil)
	data := requireValidDoc(t, doc)

	raw := parseJSON(t, data)
	paths := raw["paths"].(map[string]any)
	pathItem := paths["/auth/google"].(map[string]any)
	getOp := pathItem["get"].(map[string]any)
	responses := getOp["responses"].(map[string]any)

	// Should have both 200 (void) and 301 (redirect) responses
	resp301, ok := responses["301"].(map[string]any)
	if !ok {
		t.Fatal("expected 301 response for @Redirect")
	}

	if resp301["description"] != "Moved Permanently" {
		t.Errorf("expected description='Moved Permanently', got %q", resp301["description"])
	}

	headers := resp301["headers"].(map[string]any)
	location := headers["Location"].(map[string]any)
	locSchema := location["schema"].(map[string]any)
	enumVal := locSchema["enum"].([]any)
	if len(enumVal) != 1 || enumVal[0] != "https://accounts.google.com" {
		t.Errorf("expected Location enum=['https://accounts.google.com'], got %v", enumVal)
	}
}

// ---- @Version() Array with URI Versioning in OpenAPI ----

func TestIntegration_VersionArray_URI(t *testing.T) {
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
					OperationID: "User_findAll",
					ReturnType:  metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					StatusCode:  200,
					Tags:        []string{"User"},
					Version:     "1",
					Versions:    []string{"1", "2"},
				},
			},
		},
	}

	doc := gen.GenerateWithOptions(controllers, &GenerateOptions{
		VersioningType: "URI",
	})
	data := requireValidDoc(t, doc)

	raw := parseJSON(t, data)
	paths := raw["paths"].(map[string]any)

	// Should produce two versioned paths
	if _, ok := paths["/v1/users"]; !ok {
		t.Error("expected path /v1/users")
	}
	if _, ok := paths["/v2/users"]; !ok {
		t.Error("expected path /v2/users")
	}
	if len(paths) != 2 {
		t.Errorf("expected 2 paths, got %d: %v", len(paths), getKeys(paths))
	}
}

func TestIntegration_VersionArray_Header(t *testing.T) {
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
					OperationID: "User_findAll",
					ReturnType:  metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					StatusCode:  200,
					Tags:        []string{"User"},
					Version:     "1",
					Versions:    []string{"1", "2"},
				},
			},
		},
	}

	doc := gen.GenerateWithOptions(controllers, &GenerateOptions{
		VersioningType: "HEADER",
	})
	data := requireValidDoc(t, doc)

	// For header versioning, the same path should be used
	// but there should be version header parameters
	assertJSONContains(t, data, `"X-API-Version"`)

	// With HEADER versioning, multiple versions on the same path
	// will use the same PathItem, so only 1 path
	raw := parseJSON(t, data)
	paths := raw["paths"].(map[string]any)
	if _, ok := paths["/users"]; !ok {
		t.Error("expected path /users")
	}

	// The last version in the iteration wins for the operation on the same PathItem.
	// Verify the version parameter is present.
	pathItem := paths["/users"].(map[string]any)
	getOp := pathItem["get"].(map[string]any)
	params := getOp["parameters"].([]any)
	found := false
	for _, p := range params {
		pm := p.(map[string]any)
		if pm["name"] == "X-API-Version" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected X-API-Version parameter in operation")
	}
}

// --- Bug 1: PaginatedResponse<T> Generic Collapse in OpenAPI ---

// TestIntegration_GenericPaginatedResponse_DistinctSchemas verifies that
// when two controllers return PaginatedResponse<T> with different T, the
// OpenAPI document contains distinct response schemas with correct items types.
func TestIntegration_GenericPaginatedResponse_DistinctSchemas(t *testing.T) {
	registry := metadata.NewTypeRegistry()

	// Register the two concrete inner types
	registry.Register("CampaignResponse", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "CampaignResponse",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "budget", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
		},
	})
	registry.Register("AdSetResponse", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "AdSetResponse",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "targeting", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "status", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	})

	// Register two distinct paginated schemas (what the fix should produce)
	registry.Register("PaginatedResponse_CampaignResponse", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "PaginatedResponse_CampaignResponse",
		Properties: []metadata.Property{
			{Name: "items", Type: metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindRef, Ref: "CampaignResponse"}}, Required: true},
			{Name: "totalCount", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "currentPage", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
		},
	})
	registry.Register("PaginatedResponse_AdSetResponse", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "PaginatedResponse_AdSetResponse",
		Properties: []metadata.Property{
			{Name: "items", Type: metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindRef, Ref: "AdSetResponse"}}, Required: true},
			{Name: "totalCount", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "currentPage", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
		},
	})

	gen := NewGenerator(registry)
	controllers := []analyzer.ControllerInfo{
		{
			Name: "FacebookAdsController",
			Path: "facebook-ads",
			Routes: []analyzer.Route{
				{
					Method:      "GET",
					Path:        "/facebook-ads/campaigns",
					OperationID: "listCampaigns",
					ReturnType:  metadata.Metadata{Kind: metadata.KindRef, Ref: "PaginatedResponse_CampaignResponse"},
					StatusCode:  200,
					Tags:        []string{"FacebookAds"},
				},
				{
					Method:      "GET",
					Path:        "/facebook-ads/ad-sets",
					OperationID: "listAdSets",
					ReturnType:  metadata.Metadata{Kind: metadata.KindRef, Ref: "PaginatedResponse_AdSetResponse"},
					StatusCode:  200,
					Tags:        []string{"FacebookAds"},
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	data := requireValidDoc(t, doc)
	raw := parseJSON(t, data)

	// Both paginated schemas should exist in components/schemas
	components := raw["components"].(map[string]any)
	schemas := components["schemas"].(map[string]any)

	campaignSchema, hasCampaign := schemas["PaginatedResponse_CampaignResponse"]
	adSetSchema, hasAdSet := schemas["PaginatedResponse_AdSetResponse"]

	if !hasCampaign {
		t.Error("expected PaginatedResponse_CampaignResponse schema")
	}
	if !hasAdSet {
		t.Error("expected PaginatedResponse_AdSetResponse schema")
	}

	if hasCampaign && hasAdSet {
		// Verify their items properties reference different types
		campaignProps := campaignSchema.(map[string]any)["properties"].(map[string]any)
		adSetProps := adSetSchema.(map[string]any)["properties"].(map[string]any)

		campaignItems := campaignProps["items"].(map[string]any)
		adSetItems := adSetProps["items"].(map[string]any)

		campaignItemsRef := campaignItems["items"].(map[string]any)["$ref"].(string)
		adSetItemsRef := adSetItems["items"].(map[string]any)["$ref"].(string)

		if campaignItemsRef == adSetItemsRef {
			t.Errorf("GENERIC COLLAPSE: both paginated schemas reference the same items type %q", campaignItemsRef)
		}

		if !strings.Contains(campaignItemsRef, "CampaignResponse") {
			t.Errorf("campaign items should reference CampaignResponse, got %q", campaignItemsRef)
		}
		if !strings.Contains(adSetItemsRef, "AdSetResponse") {
			t.Errorf("ad set items should reference AdSetResponse, got %q", adSetItemsRef)
		}
	}

	// Verify routes reference different schemas
	paths := raw["paths"].(map[string]any)
	campaignsPath := paths["/facebook-ads/campaigns"].(map[string]any)
	adSetsPath := paths["/facebook-ads/ad-sets"].(map[string]any)

	campaignRef := extractResponseSchemaRef(t, campaignsPath["get"].(map[string]any))
	adSetRef := extractResponseSchemaRef(t, adSetsPath["get"].(map[string]any))

	if campaignRef == adSetRef {
		t.Errorf("GENERIC COLLAPSE: both routes reference the same response schema %q", campaignRef)
	}
}

// TestIntegration_NamedArrayType_NoDoubleNesting verifies that when a named type
// resolves to an array of objects, the OpenAPI schema for that type is the inner
// object, not an array wrapper.
func TestIntegration_NamedArrayType_NoDoubleNesting(t *testing.T) {
	registry := metadata.NewTypeRegistry()

	// Register ShipmentItemSnapshot as an OBJECT (unwrapped from its array alias)
	// This is what the type walker fix should produce
	registry.Register("ShipmentItemSnapshot", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "ShipmentItemSnapshot",
		Properties: []metadata.Property{
			{Name: "variantId", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "productId", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "quantity", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "price", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
		},
	})

	registry.Register("ShipmentResponse", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "ShipmentResponse",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "items", Type: metadata.Metadata{
				Kind:        metadata.KindArray,
				ElementType: &metadata.Metadata{Kind: metadata.KindRef, Ref: "ShipmentItemSnapshot"},
			}, Required: true},
		},
	})

	gen := NewGenerator(registry)
	controllers := []analyzer.ControllerInfo{
		{
			Name: "ShippingController",
			Path: "shipping",
			Routes: []analyzer.Route{
				{
					Method:      "GET",
					Path:        "/shipping/shipments/{id}",
					OperationID: "getShipment",
					ReturnType:  metadata.Metadata{Kind: metadata.KindRef, Ref: "ShipmentResponse"},
					StatusCode:  200,
					Tags:        []string{"Shipping"},
					Parameters:  []analyzer.RouteParameter{{Category: "path", Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true}},
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	data := requireValidDoc(t, doc)
	raw := parseJSON(t, data)

	components := raw["components"].(map[string]any)
	schemas := components["schemas"].(map[string]any)

	// ShipmentItemSnapshot should be an OBJECT schema, not an array
	snapshotSchema, ok := schemas["ShipmentItemSnapshot"].(map[string]any)
	if !ok {
		t.Fatal("expected ShipmentItemSnapshot schema")
	}
	if snapshotSchema["type"] != "object" {
		t.Errorf("DOUBLE-NESTING BUG: ShipmentItemSnapshot schema type should be 'object', got %v", snapshotSchema["type"])
	}

	// ShipmentResponse.items should be array of $ref ShipmentItemSnapshot
	shipmentSchema := schemas["ShipmentResponse"].(map[string]any)
	shipmentProps := shipmentSchema["properties"].(map[string]any)
	itemsProp := shipmentProps["items"].(map[string]any)
	if itemsProp["type"] != "array" {
		t.Errorf("items should be type=array, got %v", itemsProp["type"])
	}
	itemsItems := itemsProp["items"].(map[string]any)
	if ref, ok := itemsItems["$ref"].(string); !ok || !strings.Contains(ref, "ShipmentItemSnapshot") {
		t.Errorf("items.items should $ref ShipmentItemSnapshot, got %v", itemsItems)
	}
}

// extractResponseSchemaRef extracts the $ref from an operation's 200 response content.
func extractResponseSchemaRef(t *testing.T, operation map[string]any) string {
	t.Helper()
	responses, ok := operation["responses"].(map[string]any)
	if !ok {
		t.Fatal("no responses in operation")
		return ""
	}
	resp200, ok := responses["200"].(map[string]any)
	if !ok {
		t.Fatal("no 200 response")
		return ""
	}
	content, ok := resp200["content"].(map[string]any)
	if !ok {
		t.Fatal("no content in 200 response")
		return ""
	}
	jsonContent, ok := content["application/json"].(map[string]any)
	if !ok {
		t.Fatal("no application/json content")
		return ""
	}
	schema, ok := jsonContent["schema"].(map[string]any)
	if !ok {
		t.Fatal("no schema in response")
		return ""
	}
	ref, _ := schema["$ref"].(string)
	return ref
}

// ---- Helper ----

func getKeys(m map[string]any) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
