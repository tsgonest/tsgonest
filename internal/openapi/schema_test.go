package openapi

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tsgonest/tsgonest/internal/analyzer"
	"github.com/tsgonest/tsgonest/internal/metadata"
)

// --- Schema Generation Tests ---

func TestSchemaGenerator_String(t *testing.T) {
	gen := NewSchemaGenerator(metadata.NewTypeRegistry())
	m := &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}
	schema := gen.MetadataToSchema(m)

	if schema.Type != "string" {
		t.Errorf("expected type='string', got %q", schema.Type)
	}
}

func TestSchemaGenerator_Number(t *testing.T) {
	gen := NewSchemaGenerator(metadata.NewTypeRegistry())
	m := &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}
	schema := gen.MetadataToSchema(m)

	if schema.Type != "number" {
		t.Errorf("expected type='number', got %q", schema.Type)
	}
}

func TestSchemaGenerator_Boolean(t *testing.T) {
	gen := NewSchemaGenerator(metadata.NewTypeRegistry())
	m := &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "boolean"}
	schema := gen.MetadataToSchema(m)

	if schema.Type != "boolean" {
		t.Errorf("expected type='boolean', got %q", schema.Type)
	}
}

func TestSchemaGenerator_BigInt(t *testing.T) {
	gen := NewSchemaGenerator(metadata.NewTypeRegistry())
	m := &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "bigint"}
	schema := gen.MetadataToSchema(m)

	if schema.Type != "integer" {
		t.Errorf("expected type='integer', got %q", schema.Type)
	}
	if schema.Format != "int64" {
		t.Errorf("expected format='int64', got %q", schema.Format)
	}
}

func TestSchemaGenerator_LiteralString(t *testing.T) {
	gen := NewSchemaGenerator(metadata.NewTypeRegistry())
	m := &metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: "hello"}
	schema := gen.MetadataToSchema(m)

	if schema.Const != "hello" {
		t.Errorf("expected const='hello', got %v", schema.Const)
	}
}

func TestSchemaGenerator_LiteralNumber(t *testing.T) {
	gen := NewSchemaGenerator(metadata.NewTypeRegistry())
	m := &metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: float64(42)}
	schema := gen.MetadataToSchema(m)

	if schema.Const != float64(42) {
		t.Errorf("expected const=42, got %v", schema.Const)
	}
}

func TestSchemaGenerator_Object(t *testing.T) {
	registry := metadata.NewTypeRegistry()
	gen := NewSchemaGenerator(registry)

	m := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "User",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "email", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: false},
		},
	}

	schema := gen.MetadataToSchema(m)

	// Named object should return a $ref
	if schema.Ref != "#/components/schemas/User" {
		t.Errorf("expected $ref='#/components/schemas/User', got %q", schema.Ref)
	}

	// Check that the schema was registered
	schemas := gen.Schemas()
	userSchema, ok := schemas["User"]
	if !ok {
		t.Fatal("User schema not found in registered schemas")
	}

	if userSchema.Type != "object" {
		t.Errorf("expected type='object', got %q", userSchema.Type)
	}
	if len(userSchema.Properties) != 3 {
		t.Errorf("expected 3 properties, got %d", len(userSchema.Properties))
	}
	if len(userSchema.Required) != 2 {
		t.Errorf("expected 2 required, got %d", len(userSchema.Required))
	}

	// Check property types
	idProp, ok := userSchema.Properties["id"]
	if !ok {
		t.Fatal("'id' property not found")
	}
	if idProp.Type != "number" {
		t.Errorf("expected id type='number', got %q", idProp.Type)
	}
}

func TestSchemaGenerator_Array(t *testing.T) {
	gen := NewSchemaGenerator(metadata.NewTypeRegistry())
	elem := metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}
	m := &metadata.Metadata{Kind: metadata.KindArray, ElementType: &elem}
	schema := gen.MetadataToSchema(m)

	if schema.Type != "array" {
		t.Errorf("expected type='array', got %q", schema.Type)
	}
	if schema.Items == nil {
		t.Fatal("expected items to be set")
	}
	if schema.Items.Type != "string" {
		t.Errorf("expected items type='string', got %q", schema.Items.Type)
	}
}

func TestSchemaGenerator_Tuple(t *testing.T) {
	gen := NewSchemaGenerator(metadata.NewTypeRegistry())
	m := &metadata.Metadata{
		Kind: metadata.KindTuple,
		Elements: []metadata.TupleElement{
			{Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}},
			{Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}},
		},
	}
	schema := gen.MetadataToSchema(m)

	if schema.Type != "array" {
		t.Errorf("expected type='array', got %q", schema.Type)
	}
	if len(schema.PrefixItems) != 2 {
		t.Fatalf("expected 2 prefixItems, got %d", len(schema.PrefixItems))
	}
	if schema.PrefixItems[0].Type != "string" {
		t.Errorf("expected prefixItems[0] type='string', got %q", schema.PrefixItems[0].Type)
	}
	if schema.PrefixItems[1].Type != "number" {
		t.Errorf("expected prefixItems[1] type='number', got %q", schema.PrefixItems[1].Type)
	}
}

func TestSchemaGenerator_UnionLiterals(t *testing.T) {
	gen := NewSchemaGenerator(metadata.NewTypeRegistry())
	m := &metadata.Metadata{
		Kind: metadata.KindUnion,
		UnionMembers: []metadata.Metadata{
			{Kind: metadata.KindLiteral, LiteralValue: "admin"},
			{Kind: metadata.KindLiteral, LiteralValue: "user"},
			{Kind: metadata.KindLiteral, LiteralValue: "guest"},
		},
	}
	schema := gen.MetadataToSchema(m)

	if len(schema.Enum) != 3 {
		t.Fatalf("expected 3 enum values, got %d", len(schema.Enum))
	}
	if schema.Enum[0] != "admin" || schema.Enum[1] != "user" || schema.Enum[2] != "guest" {
		t.Errorf("unexpected enum values: %v", schema.Enum)
	}
}

func TestSchemaGenerator_UnionMixed(t *testing.T) {
	gen := NewSchemaGenerator(metadata.NewTypeRegistry())
	m := &metadata.Metadata{
		Kind: metadata.KindUnion,
		UnionMembers: []metadata.Metadata{
			{Kind: metadata.KindAtomic, Atomic: "string"},
			{Kind: metadata.KindAtomic, Atomic: "number"},
		},
	}
	schema := gen.MetadataToSchema(m)

	if len(schema.AnyOf) != 2 {
		t.Fatalf("expected 2 anyOf members, got %d", len(schema.AnyOf))
	}
	if schema.AnyOf[0].Type != "string" {
		t.Errorf("expected anyOf[0] type='string', got %q", schema.AnyOf[0].Type)
	}
	if schema.AnyOf[1].Type != "number" {
		t.Errorf("expected anyOf[1] type='number', got %q", schema.AnyOf[1].Type)
	}
}

func TestSchemaGenerator_Nullable(t *testing.T) {
	gen := NewSchemaGenerator(metadata.NewTypeRegistry())
	m := &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Nullable: true}
	schema := gen.MetadataToSchema(m)

	// Should be wrapped in anyOf with null
	if len(schema.AnyOf) != 2 {
		t.Fatalf("expected 2 anyOf members (string + null), got %d", len(schema.AnyOf))
	}
	if schema.AnyOf[0].Type != "string" {
		t.Errorf("expected anyOf[0] type='string', got %q", schema.AnyOf[0].Type)
	}
	if schema.AnyOf[1].Type != "null" {
		t.Errorf("expected anyOf[1] type='null', got %q", schema.AnyOf[1].Type)
	}
}

func TestSchemaGenerator_Date(t *testing.T) {
	gen := NewSchemaGenerator(metadata.NewTypeRegistry())
	m := &metadata.Metadata{Kind: metadata.KindNative, NativeType: "Date"}
	schema := gen.MetadataToSchema(m)

	if schema.Type != "string" {
		t.Errorf("expected type='string', got %q", schema.Type)
	}
	if schema.Format != "date-time" {
		t.Errorf("expected format='date-time', got %q", schema.Format)
	}
}

func TestSchemaGenerator_URL(t *testing.T) {
	gen := NewSchemaGenerator(metadata.NewTypeRegistry())
	m := &metadata.Metadata{Kind: metadata.KindNative, NativeType: "URL"}
	schema := gen.MetadataToSchema(m)

	if schema.Type != "string" {
		t.Errorf("expected type='string', got %q", schema.Type)
	}
	if schema.Format != "uri" {
		t.Errorf("expected format='uri', got %q", schema.Format)
	}
}

func TestSchemaGenerator_Enum(t *testing.T) {
	gen := NewSchemaGenerator(metadata.NewTypeRegistry())
	m := &metadata.Metadata{
		Kind: metadata.KindEnum,
		EnumValues: []metadata.EnumValue{
			{Name: "Admin", Value: "admin"},
			{Name: "User", Value: "user"},
		},
	}
	schema := gen.MetadataToSchema(m)

	if len(schema.Enum) != 2 {
		t.Fatalf("expected 2 enum values, got %d", len(schema.Enum))
	}
}

func TestSchemaGenerator_Ref(t *testing.T) {
	registry := metadata.NewTypeRegistry()
	registry.Register("Address", &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "Address",
		Properties: []metadata.Property{
			{Name: "street", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "city", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	})

	gen := NewSchemaGenerator(registry)
	m := &metadata.Metadata{Kind: metadata.KindRef, Ref: "Address"}
	schema := gen.MetadataToSchema(m)

	if schema.Ref != "#/components/schemas/Address" {
		t.Errorf("expected $ref='#/components/schemas/Address', got %q", schema.Ref)
	}

	// Verify it was registered
	schemas := gen.Schemas()
	addrSchema, ok := schemas["Address"]
	if !ok {
		t.Fatal("Address schema not registered")
	}
	if addrSchema.Type != "object" {
		t.Errorf("expected Address type='object', got %q", addrSchema.Type)
	}
}

func TestSchemaGenerator_IntersectionAllOf(t *testing.T) {
	gen := NewSchemaGenerator(metadata.NewTypeRegistry())
	m := &metadata.Metadata{
		Kind: metadata.KindIntersection,
		IntersectionMembers: []metadata.Metadata{
			{Kind: metadata.KindAtomic, Atomic: "string"},
			{Kind: metadata.KindAtomic, Atomic: "number"},
		},
	}
	schema := gen.MetadataToSchema(m)

	if len(schema.AllOf) != 2 {
		t.Fatalf("expected 2 allOf members, got %d", len(schema.AllOf))
	}
}

func TestSchemaGenerator_IndexSignature(t *testing.T) {
	gen := NewSchemaGenerator(metadata.NewTypeRegistry())
	m := &metadata.Metadata{
		Kind:       metadata.KindObject,
		Properties: []metadata.Property{},
		IndexSignature: &metadata.IndexSignature{
			KeyType:   metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
			ValueType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"},
		},
	}
	schema := gen.MetadataToSchema(m)

	if schema.Type != "object" {
		t.Errorf("expected type='object', got %q", schema.Type)
	}
	if schema.AdditionalProperties == nil {
		t.Fatal("expected additionalProperties to be set")
	}
	if schema.AdditionalProperties.Schema == nil {
		t.Fatal("expected additionalProperties to be a schema")
	}
	if schema.AdditionalProperties.Schema.Type != "number" {
		t.Errorf("expected additionalProperties type='number', got %q", schema.AdditionalProperties.Schema.Type)
	}
}

// --- Phase 8: New Constraint OpenAPI Mapping Tests ---

func TestSchemaGenerator_ExclusiveMinMax(t *testing.T) {
	gen := NewSchemaGenerator(metadata.NewTypeRegistry())
	exMin := 0.0
	exMax := 100.0
	m := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name:     "value",
				Type:     metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"},
				Required: true,
				Constraints: &metadata.Constraints{
					ExclusiveMinimum: &exMin,
					ExclusiveMaximum: &exMax,
				},
			},
		},
	}
	schema := gen.MetadataToSchema(m)
	valSchema := schema.Properties["value"]
	if valSchema.ExclusiveMinimum == nil || *valSchema.ExclusiveMinimum != 0 {
		t.Errorf("expected exclusiveMinimum 0, got %v", valSchema.ExclusiveMinimum)
	}
	if valSchema.ExclusiveMaximum == nil || *valSchema.ExclusiveMaximum != 100 {
		t.Errorf("expected exclusiveMaximum 100, got %v", valSchema.ExclusiveMaximum)
	}
}

func TestSchemaGenerator_MultipleOf(t *testing.T) {
	gen := NewSchemaGenerator(metadata.NewTypeRegistry())
	mult := 5.0
	m := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name:     "step",
				Type:     metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"},
				Required: true,
				Constraints: &metadata.Constraints{
					MultipleOf: &mult,
				},
			},
		},
	}
	schema := gen.MetadataToSchema(m)
	valSchema := schema.Properties["step"]
	if valSchema.MultipleOf == nil || *valSchema.MultipleOf != 5 {
		t.Errorf("expected multipleOf 5, got %v", valSchema.MultipleOf)
	}
}

func TestSchemaGenerator_UniqueItems(t *testing.T) {
	gen := NewSchemaGenerator(metadata.NewTypeRegistry())
	unique := true
	elemType := metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}
	m := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name:     "tags",
				Type:     metadata.Metadata{Kind: metadata.KindArray, ElementType: &elemType},
				Required: true,
				Constraints: &metadata.Constraints{
					UniqueItems: &unique,
				},
			},
		},
	}
	schema := gen.MetadataToSchema(m)
	valSchema := schema.Properties["tags"]
	if valSchema.UniqueItems == nil || *valSchema.UniqueItems != true {
		t.Errorf("expected uniqueItems true, got %v", valSchema.UniqueItems)
	}
}

func TestSchemaGenerator_DefaultValue(t *testing.T) {
	gen := NewSchemaGenerator(metadata.NewTypeRegistry())
	def := "10"
	m := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name:     "limit",
				Type:     metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"},
				Required: false,
				Constraints: &metadata.Constraints{
					Default: &def,
				},
			},
		},
	}
	schema := gen.MetadataToSchema(m)
	valSchema := schema.Properties["limit"]
	if valSchema.Default != "10" {
		t.Errorf("expected default '10', got %v", valSchema.Default)
	}
}

func TestSchemaGenerator_ContentMediaType(t *testing.T) {
	gen := NewSchemaGenerator(metadata.NewTypeRegistry())
	cmt := "application/json"
	m := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name:     "payload",
				Type:     metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
				Required: true,
				Constraints: &metadata.Constraints{
					ContentMediaType: &cmt,
				},
			},
		},
	}
	schema := gen.MetadataToSchema(m)
	valSchema := schema.Properties["payload"]
	if valSchema.ContentMediaType != "application/json" {
		t.Errorf("expected contentMediaType 'application/json', got %q", valSchema.ContentMediaType)
	}
}

// --- Document Generator Tests ---

func TestGenerator_SimpleController(t *testing.T) {
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
					Parameters:  nil,
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

	// Verify OpenAPI version
	if doc.OpenAPI != "3.2.0" {
		t.Errorf("expected openapi='3.2.0', got %q", doc.OpenAPI)
	}

	// Verify paths
	if len(doc.Paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(doc.Paths))
	}

	// /users path
	usersPath, ok := doc.Paths["/users"]
	if !ok {
		t.Fatal("expected /users path")
	}
	if usersPath.Get == nil {
		t.Error("expected GET operation on /users")
	}
	if usersPath.Post == nil {
		t.Error("expected POST operation on /users")
	}
	if usersPath.Get != nil && usersPath.Get.OperationID != "findAll" {
		t.Errorf("expected GET operationId='findAll', got %q", usersPath.Get.OperationID)
	}

	// /users/{id} path (NestJS :id → OpenAPI {id})
	usersIdPath, ok := doc.Paths["/users/{id}"]
	if !ok {
		t.Fatal("expected /users/{id} path")
	}
	if usersIdPath.Get == nil {
		t.Error("expected GET operation on /users/{id}")
	}
	if usersIdPath.Delete == nil {
		t.Error("expected DELETE operation on /users/{id}")
	}

	// Verify GET /users/{id} has path parameter
	if usersIdPath.Get != nil {
		if len(usersIdPath.Get.Parameters) != 1 {
			t.Fatalf("expected 1 parameter on GET /users/{id}, got %d", len(usersIdPath.Get.Parameters))
		}
		param := usersIdPath.Get.Parameters[0]
		if param.Name != "id" || param.In != "path" {
			t.Errorf("expected path param 'id', got name=%q in=%q", param.Name, param.In)
		}
		if !param.Required {
			t.Error("path parameter should be required")
		}
	}

	// Verify POST /users has request body
	if usersPath.Post != nil {
		if usersPath.Post.RequestBody == nil {
			t.Fatal("expected request body on POST /users")
		}
		if usersPath.Post.RequestBody.Content == nil {
			t.Fatal("expected content on request body")
		}
		jsonContent, ok := usersPath.Post.RequestBody.Content["application/json"]
		if !ok {
			t.Fatal("expected application/json content type")
		}
		if jsonContent.Schema == nil || jsonContent.Schema.Ref != "#/components/schemas/CreateUserDto" {
			t.Errorf("expected schema $ref to CreateUserDto, got %v", jsonContent.Schema)
		}
	}

	// Verify DELETE /users/{id} has 204 response with no content
	if usersIdPath.Delete != nil {
		resp204, ok := usersIdPath.Delete.Responses["204"]
		if !ok {
			t.Fatal("expected 204 response on DELETE")
		}
		if resp204.Content != nil {
			t.Error("expected no content on 204 response")
		}
	}

	// Verify components/schemas
	if doc.Components == nil || doc.Components.Schemas == nil {
		t.Fatal("expected components/schemas")
	}
	if _, ok := doc.Components.Schemas["UserResponse"]; !ok {
		t.Error("expected UserResponse in components/schemas")
	}
	if _, ok := doc.Components.Schemas["CreateUserDto"]; !ok {
		t.Error("expected CreateUserDto in components/schemas")
	}

	// Verify tags
	if len(doc.Tags) != 1 || doc.Tags[0].Name != "User" {
		t.Errorf("expected tags=[User], got %v", doc.Tags)
	}
}

func TestGenerator_ConvertPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/users", "/users"},
		{"/users/:id", "/users/{id}"},
		{"/users/:userId/posts/:postId", "/users/{userId}/posts/{postId}"},
		{"/", "/"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := convertPath(tt.input); got != tt.want {
				t.Errorf("convertPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGenerator_QueryDecomposition(t *testing.T) {
	registry := metadata.NewTypeRegistry()
	registry.Register("ListQuery", &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "ListQuery",
		Properties: []metadata.Property{
			{Name: "page", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: false},
			{Name: "limit", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: false},
			{Name: "search", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: false},
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
					Parameters: []analyzer.RouteParameter{
						{Category: "query", Name: "", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "ListQuery"}, Required: false},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindVoid},
					StatusCode: 200,
					Tags:       []string{"User"},
				},
			},
		},
	}

	doc := gen.Generate(controllers)

	usersPath, ok := doc.Paths["/users"]
	if !ok || usersPath.Get == nil {
		t.Fatal("expected GET /users")
	}

	// Should have 3 individual query parameters (decomposed from ListQuery object)
	params := usersPath.Get.Parameters
	if len(params) != 3 {
		t.Fatalf("expected 3 query parameters (decomposed), got %d", len(params))
	}

	expectedNames := map[string]bool{"page": true, "limit": true, "search": true}
	for _, p := range params {
		if p.In != "query" {
			t.Errorf("expected in='query', got %q", p.In)
		}
		if !expectedNames[p.Name] {
			t.Errorf("unexpected query param name=%q", p.Name)
		}
		// All are optional
		if p.Required {
			t.Errorf("expected required=false for %q", p.Name)
		}
	}
}

// --- Phase 10.1/10.2: JSDoc Metadata & Error Response Tests ---

func TestGenerator_JSDocMetadataInOperation(t *testing.T) {
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
					OperationID: "findAll",
					Summary:     "List all users",
					Description: "Returns paginated users",
					Deprecated:  true,
					Tags:        []string{"Users", "Admin"},
					Security: []analyzer.SecurityRequirement{
						{Name: "bearer", Scopes: []string{}},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					StatusCode: 200,
				},
			},
		},
	}

	doc := gen.Generate(controllers)

	usersPath, ok := doc.Paths["/users"]
	if !ok || usersPath.Get == nil {
		t.Fatal("expected GET /users")
	}

	op := usersPath.Get
	if op.Summary != "List all users" {
		t.Errorf("expected Summary='List all users', got %q", op.Summary)
	}
	if op.Description != "Returns paginated users" {
		t.Errorf("expected Description='Returns paginated users', got %q", op.Description)
	}
	if !op.Deprecated {
		t.Error("expected Deprecated=true")
	}
	if len(op.Tags) != 2 || op.Tags[0] != "Users" || op.Tags[1] != "Admin" {
		t.Errorf("expected Tags=[Users, Admin], got %v", op.Tags)
	}
	if len(op.Security) != 1 {
		t.Fatalf("expected 1 security req, got %d", len(op.Security))
	}
	bearerScopes, ok := op.Security[0]["bearer"]
	if !ok {
		t.Error("expected 'bearer' key in security")
	}
	if len(bearerScopes) != 0 {
		t.Errorf("expected empty scopes, got %v", bearerScopes)
	}
}

func TestGenerator_ErrorResponses(t *testing.T) {
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
						{Category: "body", Name: "", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
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

	usersPath, ok := doc.Paths["/users"]
	if !ok || usersPath.Post == nil {
		t.Fatal("expected POST /users")
	}

	op := usersPath.Post

	// Check success response
	resp201, ok := op.Responses["201"]
	if !ok {
		t.Fatal("expected 201 response")
	}
	if resp201.Description != "Created" {
		t.Errorf("expected 201 description='Created', got %q", resp201.Description)
	}

	// Check error responses
	resp400, ok := op.Responses["400"]
	if !ok {
		t.Fatal("expected 400 response")
	}
	if resp400.Description != "Bad Request" {
		t.Errorf("expected 400 description='Bad Request', got %q", resp400.Description)
	}
	if resp400.Content == nil {
		t.Fatal("expected 400 response to have content")
	}
	jsonContent, ok := resp400.Content["application/json"]
	if !ok {
		t.Fatal("expected application/json content in 400 response")
	}
	if jsonContent.Schema == nil || jsonContent.Schema.Ref != "#/components/schemas/BadRequestError" {
		t.Errorf("expected 400 schema $ref to BadRequestError, got %v", jsonContent.Schema)
	}

	// Check 401 response (no type, should have no content)
	resp401, ok := op.Responses["401"]
	if !ok {
		t.Fatal("expected 401 response")
	}
	if resp401.Description != "Unauthorized" {
		t.Errorf("expected 401 description='Unauthorized', got %q", resp401.Description)
	}
	if resp401.Content != nil {
		t.Error("expected 401 response to have no content (no type provided)")
	}
}

func TestGenerator_ToJSON(t *testing.T) {
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "HealthController",
			Path: "health",
			Routes: []analyzer.Route{
				{
					Method:      "GET",
					Path:        "/health",
					OperationID: "check",
					ReturnType:  metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					StatusCode:  200,
					Tags:        []string{"Health"},
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	jsonBytes, err := doc.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]any
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("generated JSON is invalid: %v", err)
	}

	// Verify key fields
	if parsed["openapi"] != "3.2.0" {
		t.Errorf("expected openapi='3.2.0', got %v", parsed["openapi"])
	}
	paths, ok := parsed["paths"].(map[string]any)
	if !ok {
		t.Fatal("expected paths object")
	}
	if _, ok := paths["/health"]; !ok {
		t.Error("expected /health path in JSON output")
	}
}

func TestSchemaGenerator_DiscriminatedUnion(t *testing.T) {
	registry := metadata.NewTypeRegistry()
	registry.Register("Cat", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "Cat",
		Properties: []metadata.Property{
			{Name: "type", Type: metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: "cat"}, Required: true},
			{Name: "meow", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "boolean"}, Required: true},
		},
	})
	registry.Register("Dog", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "Dog",
		Properties: []metadata.Property{
			{Name: "type", Type: metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: "dog"}, Required: true},
			{Name: "bark", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "boolean"}, Required: true},
		},
	})

	gen := NewSchemaGenerator(registry)
	m := &metadata.Metadata{
		Kind: metadata.KindUnion,
		UnionMembers: []metadata.Metadata{
			{Kind: metadata.KindRef, Ref: "Cat"},
			{Kind: metadata.KindRef, Ref: "Dog"},
		},
		Discriminant: &metadata.Discriminant{
			Property: "type",
			Mapping: map[string]int{
				"cat": 0,
				"dog": 1,
			},
		},
	}

	schema := gen.MetadataToSchema(m)
	if len(schema.OneOf) != 2 {
		t.Fatalf("expected 2 oneOf members, got %d", len(schema.OneOf))
	}
	if schema.Discriminator == nil {
		t.Fatal("expected discriminator to be set")
	}
	if schema.Discriminator.PropertyName != "type" {
		t.Errorf("expected propertyName='type', got %q", schema.Discriminator.PropertyName)
	}
	if len(schema.Discriminator.Mapping) != 2 {
		t.Errorf("expected 2 mapping entries, got %d", len(schema.Discriminator.Mapping))
	}
	// Verify mapping values point to $ref paths
	if schema.Discriminator.Mapping["cat"] != "#/components/schemas/Cat" {
		t.Errorf("expected mapping[cat]='#/components/schemas/Cat', got %q", schema.Discriminator.Mapping["cat"])
	}
	if schema.Discriminator.Mapping["dog"] != "#/components/schemas/Dog" {
		t.Errorf("expected mapping[dog]='#/components/schemas/Dog', got %q", schema.Discriminator.Mapping["dog"])
	}
}

func TestDocument_ApplyConfig(t *testing.T) {
	doc := &Document{
		OpenAPI: "3.2.0",
		Info:    Info{Title: "API", Version: "1.0.0"},
		Paths:   make(map[string]*PathItem),
	}

	doc.ApplyConfig(DocumentConfig{
		Title:       "My API",
		Description: "A great API",
		Version:     "2.0.0",
		Contact:     &Contact{Name: "Support", Email: "support@example.com"},
		License:     &License{Name: "MIT", URL: "https://opensource.org/licenses/MIT"},
		Servers: []Server{
			{URL: "https://api.example.com", Description: "Production"},
			{URL: "https://staging.example.com", Description: "Staging"},
		},
		SecuritySchemes: map[string]*SecurityScheme{
			"bearer": {Type: "http", Scheme: "bearer", BearerFormat: "JWT"},
		},
	})

	if doc.Info.Title != "My API" {
		t.Errorf("expected title='My API', got %q", doc.Info.Title)
	}
	if doc.Info.Description != "A great API" {
		t.Errorf("expected description='A great API', got %q", doc.Info.Description)
	}
	if doc.Info.Version != "2.0.0" {
		t.Errorf("expected version='2.0.0', got %q", doc.Info.Version)
	}
	if doc.Info.Contact == nil || doc.Info.Contact.Name != "Support" {
		t.Errorf("expected contact.name='Support'")
	}
	if doc.Info.License == nil || doc.Info.License.Name != "MIT" {
		t.Errorf("expected license.name='MIT'")
	}
	if len(doc.Servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(doc.Servers))
	}
	if doc.Servers[0].URL != "https://api.example.com" {
		t.Errorf("expected server[0].url='https://api.example.com', got %q", doc.Servers[0].URL)
	}
	if doc.Components == nil || doc.Components.SecuritySchemes == nil {
		t.Fatal("expected components.securitySchemes")
	}
	bearer, ok := doc.Components.SecuritySchemes["bearer"]
	if !ok {
		t.Fatal("expected 'bearer' security scheme")
	}
	if bearer.Type != "http" || bearer.Scheme != "bearer" || bearer.BearerFormat != "JWT" {
		t.Errorf("expected bearer JWT scheme, got type=%q scheme=%q format=%q", bearer.Type, bearer.Scheme, bearer.BearerFormat)
	}
}

// --- Phase 10.3: Global Prefix & Versioning Tests ---

func TestGenerator_GlobalPrefix(t *testing.T) {
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
					OperationID: "findAll",
					ReturnType:  metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
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
					ReturnType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					StatusCode: 200,
					Tags:       []string{"User"},
				},
			},
		},
	}

	doc := gen.GenerateWithOptions(controllers, &GenerateOptions{
		GlobalPrefix: "api",
	})

	// Verify paths are prefixed with /api
	if _, ok := doc.Paths["/api/users"]; !ok {
		t.Errorf("expected /api/users path, got paths: %v", pathKeys(doc.Paths))
	}
	if _, ok := doc.Paths["/api/users/{id}"]; !ok {
		t.Errorf("expected /api/users/{id} path, got paths: %v", pathKeys(doc.Paths))
	}
	// Ensure original paths are not present
	if _, ok := doc.Paths["/users"]; ok {
		t.Error("unexpected /users path (should be /api/users)")
	}
}

func TestGenerator_URIVersioning(t *testing.T) {
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
					OperationID: "findAll",
					ReturnType:  metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					StatusCode:  200,
					Tags:        []string{"User"},
				},
			},
		},
	}

	doc := gen.GenerateWithOptions(controllers, &GenerateOptions{
		VersioningType: "URI",
		DefaultVersion: "1",
		VersionPrefix:  "v",
	})

	// Verify version prefix in path
	if _, ok := doc.Paths["/v1/users"]; !ok {
		t.Errorf("expected /v1/users path, got paths: %v", pathKeys(doc.Paths))
	}
}

func TestGenerator_URIVersioningDefaultPrefix(t *testing.T) {
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
					OperationID: "findAll",
					ReturnType:  metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					StatusCode:  200,
					Tags:        []string{"User"},
				},
			},
		},
	}

	// Empty VersionPrefix should default to "v"
	doc := gen.GenerateWithOptions(controllers, &GenerateOptions{
		VersioningType: "URI",
		DefaultVersion: "2",
	})

	if _, ok := doc.Paths["/v2/users"]; !ok {
		t.Errorf("expected /v2/users path (default prefix 'v'), got paths: %v", pathKeys(doc.Paths))
	}
}

func TestGenerator_VersionDecorator(t *testing.T) {
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
		VersioningType: "URI",
		VersionPrefix:  "v",
	})

	// Each route should have its own versioned path
	if _, ok := doc.Paths["/v1/users"]; !ok {
		t.Errorf("expected /v1/users path, got paths: %v", pathKeys(doc.Paths))
	}
	if _, ok := doc.Paths["/v2/users"]; !ok {
		t.Errorf("expected /v2/users path, got paths: %v", pathKeys(doc.Paths))
	}

	// Verify operationIds are correct
	v1Path := doc.Paths["/v1/users"]
	if v1Path != nil && v1Path.Get != nil && v1Path.Get.OperationID != "findAllV1" {
		t.Errorf("expected /v1/users GET operationId='findAllV1', got %q", v1Path.Get.OperationID)
	}
	v2Path := doc.Paths["/v2/users"]
	if v2Path != nil && v2Path.Get != nil && v2Path.Get.OperationID != "findAllV2" {
		t.Errorf("expected /v2/users GET operationId='findAllV2', got %q", v2Path.Get.OperationID)
	}
}

func TestGenerator_GlobalPrefixWithURIVersioning(t *testing.T) {
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
					OperationID: "findAll",
					ReturnType:  metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					StatusCode:  200,
					Tags:        []string{"User"},
				},
			},
		},
	}

	doc := gen.GenerateWithOptions(controllers, &GenerateOptions{
		GlobalPrefix:   "api",
		VersioningType: "URI",
		DefaultVersion: "1",
		VersionPrefix:  "v",
	})

	// Version is applied first, then global prefix: /api/v1/users
	if _, ok := doc.Paths["/api/v1/users"]; !ok {
		t.Errorf("expected /api/v1/users path, got paths: %v", pathKeys(doc.Paths))
	}
}

func TestGenerator_HeaderVersioning(t *testing.T) {
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
					OperationID: "findAll",
					ReturnType:  metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					StatusCode:  200,
					Tags:        []string{"User"},
					Version:     "1",
				},
			},
		},
	}

	doc := gen.GenerateWithOptions(controllers, &GenerateOptions{
		VersioningType: "HEADER",
	})

	// Path should NOT have version prefix
	if _, ok := doc.Paths["/users"]; !ok {
		t.Errorf("expected /users path (HEADER versioning doesn't modify path), got paths: %v", pathKeys(doc.Paths))
	}

	// Should have X-API-Version header parameter
	usersPath := doc.Paths["/users"]
	if usersPath == nil || usersPath.Get == nil {
		t.Fatal("expected GET /users")
	}

	found := false
	for _, param := range usersPath.Get.Parameters {
		if param.Name == "X-API-Version" && param.In == "header" {
			found = true
			if !param.Required {
				t.Error("expected X-API-Version to be required")
			}
			if param.Schema == nil || param.Schema.Const != "1" {
				t.Errorf("expected X-API-Version schema const='1', got %v", param.Schema)
			}
		}
	}
	if !found {
		t.Error("expected X-API-Version header parameter")
	}
}

func TestGenerator_NilOptions(t *testing.T) {
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
					OperationID: "findAll",
					ReturnType:  metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					StatusCode:  200,
					Tags:        []string{"User"},
				},
			},
		},
	}

	// nil options should behave like Generate
	doc := gen.GenerateWithOptions(controllers, nil)
	if _, ok := doc.Paths["/users"]; !ok {
		t.Errorf("expected /users path with nil options, got paths: %v", pathKeys(doc.Paths))
	}
}

// pathKeys returns the keys of a Paths map for debugging.
func pathKeys(paths map[string]*PathItem) []string {
	var keys []string
	for k := range paths {
		keys = append(keys, k)
	}
	return keys
}

// --- Phase 10.4: Content Type Support Tests ---

func TestGenerator_TextPlainBody(t *testing.T) {
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "MessageController",
			Path: "messages",
			Routes: []analyzer.Route{
				{
					Method:      "POST",
					Path:        "/messages",
					OperationID: "send",
					Parameters: []analyzer.RouteParameter{
						{Category: "body", Name: "", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindVoid},
					StatusCode: 201,
					Tags:       []string{"Message"},
				},
			},
		},
	}

	doc := gen.Generate(controllers)

	msgsPath, ok := doc.Paths["/messages"]
	if !ok || msgsPath.Post == nil {
		t.Fatal("expected POST /messages")
	}

	rb := msgsPath.Post.RequestBody
	if rb == nil {
		t.Fatal("expected request body")
	}

	// String body should use text/plain content type
	if _, ok := rb.Content["text/plain"]; !ok {
		t.Errorf("expected text/plain content type for string body, got keys: %v", contentTypeKeys(rb.Content))
	}
	if _, ok := rb.Content["application/json"]; ok {
		t.Error("expected no application/json content type for string body")
	}
}

func TestGenerator_ContentTypeOverride(t *testing.T) {
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "UploadController",
			Path: "upload",
			Routes: []analyzer.Route{
				{
					Method:      "POST",
					Path:        "/upload",
					OperationID: "uploadFile",
					Parameters: []analyzer.RouteParameter{
						{
							Category:    "body",
							Name:        "",
							Type:        metadata.Metadata{Kind: metadata.KindObject, Name: "UploadDto"},
							Required:    true,
							ContentType: "multipart/form-data",
						},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindVoid},
					StatusCode: 201,
					Tags:       []string{"Upload"},
				},
			},
		},
	}

	doc := gen.Generate(controllers)

	uploadPath, ok := doc.Paths["/upload"]
	if !ok || uploadPath.Post == nil {
		t.Fatal("expected POST /upload")
	}

	rb := uploadPath.Post.RequestBody
	if rb == nil {
		t.Fatal("expected request body")
	}

	// ContentType override should use multipart/form-data
	if _, ok := rb.Content["multipart/form-data"]; !ok {
		t.Errorf("expected multipart/form-data content type, got keys: %v", contentTypeKeys(rb.Content))
	}
}

func TestGenerator_FormURLEncodedBody(t *testing.T) {
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "FormController",
			Path: "form",
			Routes: []analyzer.Route{
				{
					Method:      "POST",
					Path:        "/form",
					OperationID: "submitForm",
					Parameters: []analyzer.RouteParameter{
						{
							Category:    "body",
							Name:        "",
							Type:        metadata.Metadata{Kind: metadata.KindObject, Name: "FormDto"},
							Required:    true,
							ContentType: "application/x-www-form-urlencoded",
						},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindVoid},
					StatusCode: 201,
					Tags:       []string{"Form"},
				},
			},
		},
	}

	doc := gen.Generate(controllers)

	formPath, ok := doc.Paths["/form"]
	if !ok || formPath.Post == nil {
		t.Fatal("expected POST /form")
	}

	rb := formPath.Post.RequestBody
	if rb == nil {
		t.Fatal("expected request body")
	}

	if _, ok := rb.Content["application/x-www-form-urlencoded"]; !ok {
		t.Errorf("expected application/x-www-form-urlencoded content type, got keys: %v", contentTypeKeys(rb.Content))
	}
}

func TestGenerator_ObjectBodyDefaultsToJSON(t *testing.T) {
	registry := metadata.NewTypeRegistry()
	registry.Register("CreateDto", &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "CreateDto",
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	})
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "ItemController",
			Path: "items",
			Routes: []analyzer.Route{
				{
					Method:      "POST",
					Path:        "/items",
					OperationID: "create",
					Parameters: []analyzer.RouteParameter{
						{Category: "body", Name: "", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "CreateDto"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindVoid},
					StatusCode: 201,
					Tags:       []string{"Item"},
				},
			},
		},
	}

	doc := gen.Generate(controllers)

	itemsPath, ok := doc.Paths["/items"]
	if !ok || itemsPath.Post == nil {
		t.Fatal("expected POST /items")
	}

	rb := itemsPath.Post.RequestBody
	if rb == nil {
		t.Fatal("expected request body")
	}

	// Object/ref body without ContentType override should default to application/json
	if _, ok := rb.Content["application/json"]; !ok {
		t.Errorf("expected application/json content type for object body, got keys: %v", contentTypeKeys(rb.Content))
	}
}

// --- Phase 7: additionalProperties for @strict ---

func TestSchemaGenerator_StrictAdditionalPropertiesFalse(t *testing.T) {
	gen := NewSchemaGenerator(metadata.NewTypeRegistry())
	m := &metadata.Metadata{
		Kind:       metadata.KindObject,
		Strictness: "strict",
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}
	schema := gen.MetadataToSchema(m)

	if schema.AdditionalProperties == nil {
		t.Fatal("expected additionalProperties to be set for @strict")
	}
	if schema.AdditionalProperties.Bool == nil {
		t.Fatal("expected additionalProperties to be a boolean")
	}
	if *schema.AdditionalProperties.Bool != false {
		t.Errorf("expected additionalProperties=false, got %v", *schema.AdditionalProperties.Bool)
	}
}

func TestSchemaGenerator_StripNoAdditionalProperties(t *testing.T) {
	gen := NewSchemaGenerator(metadata.NewTypeRegistry())
	m := &metadata.Metadata{
		Kind:       metadata.KindObject,
		Strictness: "strip",
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}
	schema := gen.MetadataToSchema(m)

	// @strip should NOT set additionalProperties: false (strip is runtime behavior, not schema)
	if schema.AdditionalProperties != nil {
		t.Errorf("expected no additionalProperties for @strip, got %+v", schema.AdditionalProperties)
	}
}

func TestSchemaGenerator_PassthroughNoAdditionalProperties(t *testing.T) {
	gen := NewSchemaGenerator(metadata.NewTypeRegistry())
	m := &metadata.Metadata{
		Kind:       metadata.KindObject,
		Strictness: "passthrough",
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}
	schema := gen.MetadataToSchema(m)

	if schema.AdditionalProperties != nil {
		t.Errorf("expected no additionalProperties for @passthrough, got %+v", schema.AdditionalProperties)
	}
}

func TestSchemaGenerator_StrictWithIndexSig(t *testing.T) {
	// When a @strict type also has an index signature, the index sig takes precedence
	gen := NewSchemaGenerator(metadata.NewTypeRegistry())
	m := &metadata.Metadata{
		Kind:       metadata.KindObject,
		Strictness: "strict",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
		},
		IndexSignature: &metadata.IndexSignature{
			KeyType:   metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
			ValueType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
		},
	}
	schema := gen.MetadataToSchema(m)

	// Index signature should be a schema, not boolean false
	if schema.AdditionalProperties == nil {
		t.Fatal("expected additionalProperties to be set")
	}
	if schema.AdditionalProperties.Schema == nil {
		t.Fatal("expected additionalProperties to be a schema (index sig), not boolean")
	}
	if schema.AdditionalProperties.Schema.Type != "string" {
		t.Errorf("expected additionalProperties schema type='string', got %q", schema.AdditionalProperties.Schema.Type)
	}
}

// contentTypeKeys returns the content type keys from a MediaType map for debugging.
func contentTypeKeys(content map[string]MediaType) []string {
	var keys []string
	for k := range content {
		keys = append(keys, k)
	}
	return keys
}

// --- Feature 1: @hidden / @exclude ---

func TestGenerator_HiddenRouteExcludedFromOpenAPI(t *testing.T) {
	// Routes with @hidden/@exclude JSDoc should not appear in the OpenAPI document.
	// The route analyzer filters them out (returns nil from analyzeMethod),
	// so they never reach the generator. This test verifies the generator
	// correctly produces a doc without hidden routes.
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "ItemController",
			Path: "items",
			Routes: []analyzer.Route{
				{
					Method:      "GET",
					Path:        "/items",
					OperationID: "findAll",
					ReturnType:  metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}},
					StatusCode:  200,
					Tags:        []string{"Item"},
				},
				// Note: a @hidden route would NOT appear here because analyzeMethod returns nil.
				// We intentionally omit "internalMethod" to verify the doc only has visible routes.
			},
		},
	}

	doc := gen.Generate(controllers)

	// Should only have the visible route
	itemsPath, ok := doc.Paths["/items"]
	if !ok {
		t.Fatal("expected /items path")
	}
	if itemsPath.Get == nil {
		t.Error("expected GET /items to be present")
	}
	if itemsPath.Get.OperationID != "findAll" {
		t.Errorf("expected operationId='findAll', got %q", itemsPath.Get.OperationID)
	}

	// Verify no other paths exist
	if len(doc.Paths) != 1 {
		t.Errorf("expected 1 path, got %d", len(doc.Paths))
	}
}

func TestGenerator_MixedHiddenAndVisibleRoutes(t *testing.T) {
	// When a controller has some hidden routes, only visible routes appear.
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "HealthController",
			Path: "health",
			Routes: []analyzer.Route{
				{
					Method:      "GET",
					Path:        "/health",
					OperationID: "check",
					ReturnType:  metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					StatusCode:  200,
					Tags:        []string{"Health"},
				},
				// "debug" and "metrics" are hidden — they would be filtered by analyzeMethod
			},
		},
		{
			Name: "UserController",
			Path: "users",
			Routes: []analyzer.Route{
				{
					Method:      "GET",
					Path:        "/users",
					OperationID: "listUsers",
					ReturnType:  metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}},
					StatusCode:  200,
					Tags:        []string{"User"},
				},
			},
		},
	}

	doc := gen.Generate(controllers)

	if len(doc.Paths) != 2 {
		t.Errorf("expected 2 paths (hidden routes filtered before generator), got %d", len(doc.Paths))
	}
	if _, ok := doc.Paths["/health"]; !ok {
		t.Error("expected /health path")
	}
	if _, ok := doc.Paths["/users"]; !ok {
		t.Error("expected /users path")
	}
}

// --- Feature 2: Enum as $ref ---

func TestSchemaGenerator_NamedEnumUnionAsRef(t *testing.T) {
	// A union of literals with a name (from type alias or TS enum) should be
	// registered as a component schema and returned as $ref.
	registry := metadata.NewTypeRegistry()
	gen := NewSchemaGenerator(registry)

	m := &metadata.Metadata{
		Kind: metadata.KindUnion,
		Name: "OrderStatus",
		UnionMembers: []metadata.Metadata{
			{Kind: metadata.KindLiteral, LiteralValue: "PENDING"},
			{Kind: metadata.KindLiteral, LiteralValue: "SHIPPED"},
			{Kind: metadata.KindLiteral, LiteralValue: "DELIVERED"},
		},
	}

	schema := gen.MetadataToSchema(m)

	// Should return a $ref
	if schema.Ref != "#/components/schemas/OrderStatus" {
		t.Errorf("expected $ref='#/components/schemas/OrderStatus', got %q", schema.Ref)
	}

	// Should be registered in schemas
	schemas := gen.Schemas()
	enumSchema, ok := schemas["OrderStatus"]
	if !ok {
		t.Fatal("expected OrderStatus to be registered in schemas")
	}

	// Should have the enum values
	if len(enumSchema.Enum) != 3 {
		t.Fatalf("expected 3 enum values, got %d", len(enumSchema.Enum))
	}
	expected := []any{"PENDING", "SHIPPED", "DELIVERED"}
	for i, v := range expected {
		if enumSchema.Enum[i] != v {
			t.Errorf("enum[%d]: expected %q, got %v", i, v, enumSchema.Enum[i])
		}
	}
}

func TestSchemaGenerator_UnnamedEnumUnionInline(t *testing.T) {
	// A union of literals WITHOUT a name should remain inline (no $ref).
	registry := metadata.NewTypeRegistry()
	gen := NewSchemaGenerator(registry)

	m := &metadata.Metadata{
		Kind: metadata.KindUnion,
		UnionMembers: []metadata.Metadata{
			{Kind: metadata.KindLiteral, LiteralValue: "yes"},
			{Kind: metadata.KindLiteral, LiteralValue: "no"},
		},
	}

	schema := gen.MetadataToSchema(m)

	// Should NOT be a $ref
	if schema.Ref != "" {
		t.Errorf("expected no $ref for unnamed enum union, got %q", schema.Ref)
	}
	// Should be inline enum
	if len(schema.Enum) != 2 {
		t.Fatalf("expected 2 enum values, got %d", len(schema.Enum))
	}
}

func TestSchemaGenerator_NamedEnumDeduplication(t *testing.T) {
	// The same named enum used in multiple places should produce a single
	// component schema with multiple $ref pointers.
	registry := metadata.NewTypeRegistry()
	gen := NewSchemaGenerator(registry)

	orderStatus := metadata.Metadata{
		Kind: metadata.KindUnion,
		Name: "OrderStatus",
		UnionMembers: []metadata.Metadata{
			{Kind: metadata.KindLiteral, LiteralValue: "PENDING"},
			{Kind: metadata.KindLiteral, LiteralValue: "SHIPPED"},
		},
	}

	// Use in two different objects
	obj1 := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "Order",
		Properties: []metadata.Property{
			{Name: "status", Type: orderStatus, Required: true},
		},
	}
	obj2 := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "OrderHistory",
		Properties: []metadata.Property{
			{Name: "previousStatus", Type: orderStatus, Required: true},
			{Name: "currentStatus", Type: orderStatus, Required: true},
		},
	}

	gen.MetadataToSchema(obj1)
	gen.MetadataToSchema(obj2)

	schemas := gen.Schemas()

	// OrderStatus should appear exactly once
	if _, ok := schemas["OrderStatus"]; !ok {
		t.Fatal("expected OrderStatus in schemas")
	}

	// The Order and OrderHistory schemas should reference OrderStatus via $ref
	orderSchema := schemas["Order"]
	if orderSchema == nil {
		t.Fatal("expected Order in schemas")
	}
	statusProp, ok := orderSchema.Properties["status"]
	if !ok {
		t.Fatal("expected 'status' property in Order schema")
	}
	if statusProp.Ref != "#/components/schemas/OrderStatus" {
		t.Errorf("expected status.$ref='#/components/schemas/OrderStatus', got %q", statusProp.Ref)
	}

	historySchema := schemas["OrderHistory"]
	if historySchema == nil {
		t.Fatal("expected OrderHistory in schemas")
	}
	prevProp, ok := historySchema.Properties["previousStatus"]
	if !ok {
		t.Fatal("expected 'previousStatus' property in OrderHistory schema")
	}
	if prevProp.Ref != "#/components/schemas/OrderStatus" {
		t.Errorf("expected previousStatus.$ref='#/components/schemas/OrderStatus', got %q", prevProp.Ref)
	}
}

func TestSchemaGenerator_NamedEnumKindEnum(t *testing.T) {
	// KindEnum (from actual TS enum declarations) with a name should also be a $ref.
	registry := metadata.NewTypeRegistry()
	gen := NewSchemaGenerator(registry)

	m := &metadata.Metadata{
		Kind: metadata.KindEnum,
		Name: "Color",
		EnumValues: []metadata.EnumValue{
			{Name: "Red", Value: "red"},
			{Name: "Green", Value: "green"},
			{Name: "Blue", Value: "blue"},
		},
	}

	schema := gen.MetadataToSchema(m)

	if schema.Ref != "#/components/schemas/Color" {
		t.Errorf("expected $ref='#/components/schemas/Color', got %q", schema.Ref)
	}

	schemas := gen.Schemas()
	colorSchema, ok := schemas["Color"]
	if !ok {
		t.Fatal("expected Color to be registered in schemas")
	}
	if len(colorSchema.Enum) != 3 {
		t.Fatalf("expected 3 enum values, got %d", len(colorSchema.Enum))
	}
}

func TestSchemaGenerator_NullableNamedEnum(t *testing.T) {
	// A nullable named enum should wrap the $ref in anyOf with null.
	registry := metadata.NewTypeRegistry()
	gen := NewSchemaGenerator(registry)

	m := &metadata.Metadata{
		Kind:     metadata.KindUnion,
		Name:     "Status",
		Nullable: true,
		UnionMembers: []metadata.Metadata{
			{Kind: metadata.KindLiteral, LiteralValue: "active"},
			{Kind: metadata.KindLiteral, LiteralValue: "inactive"},
		},
	}

	schema := gen.MetadataToSchema(m)

	// Should be wrapped in anyOf with null
	if schema.AnyOf == nil || len(schema.AnyOf) != 2 {
		t.Fatalf("expected anyOf with 2 items for nullable enum, got %v", schema.AnyOf)
	}
	if schema.AnyOf[0].Ref != "#/components/schemas/Status" {
		t.Errorf("expected anyOf[0].$ref='#/components/schemas/Status', got %q", schema.AnyOf[0].Ref)
	}
	if schema.AnyOf[1].Type != "null" {
		t.Errorf("expected anyOf[1].type='null', got %q", schema.AnyOf[1].Type)
	}
}

// --- Feature 3: Array query params ---

func TestGenerator_ArrayQueryParam(t *testing.T) {
	// Array-typed query params should have style=form and explode=true.
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "OrderController",
			Path: "orders",
			Routes: []analyzer.Route{
				{
					Method:      "GET",
					Path:        "/orders",
					OperationID: "findOrders",
					Parameters: []analyzer.RouteParameter{
						{
							Category: "query",
							Name:     "",
							Type: metadata.Metadata{
								Kind: metadata.KindObject,
								Properties: []metadata.Property{
									{Name: "page", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: false},
									{Name: "status", Type: metadata.Metadata{
										Kind:        metadata.KindArray,
										ElementType: &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
									}, Required: false},
								},
							},
							Required: false,
						},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}},
					StatusCode: 200,
					Tags:       []string{"Order"},
				},
			},
		},
	}

	doc := gen.Generate(controllers)

	ordersPath, ok := doc.Paths["/orders"]
	if !ok {
		t.Fatal("expected /orders path")
	}
	if ordersPath.Get == nil {
		t.Fatal("expected GET operation on /orders")
	}

	params := ordersPath.Get.Parameters
	if len(params) != 2 {
		t.Fatalf("expected 2 parameters, got %d", len(params))
	}

	// Find the "status" param (array)
	var statusParam *Parameter
	var pageParam *Parameter
	for i := range params {
		switch params[i].Name {
		case "status":
			statusParam = &params[i]
		case "page":
			pageParam = &params[i]
		}
	}

	if statusParam == nil {
		t.Fatal("expected 'status' query parameter")
	}
	if statusParam.Style != "form" {
		t.Errorf("expected status.style='form', got %q", statusParam.Style)
	}
	if statusParam.Explode == nil || !*statusParam.Explode {
		t.Error("expected status.explode=true")
	}
	if statusParam.Schema == nil || statusParam.Schema.Type != "array" {
		t.Error("expected status schema to be array type")
	}

	// Non-array param should NOT have style/explode
	if pageParam == nil {
		t.Fatal("expected 'page' query parameter")
	}
	if pageParam.Style != "" {
		t.Errorf("expected page.style to be empty, got %q", pageParam.Style)
	}
	if pageParam.Explode != nil {
		t.Errorf("expected page.explode to be nil, got %v", *pageParam.Explode)
	}
}

func TestGenerator_ArrayQueryParamJSON(t *testing.T) {
	// Verify the JSON output includes style and explode for array query params.
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "FilterController",
			Path: "filter",
			Routes: []analyzer.Route{
				{
					Method:      "GET",
					Path:        "/filter",
					OperationID: "filter",
					Parameters: []analyzer.RouteParameter{
						{
							Category: "query",
							Name:     "",
							Type: metadata.Metadata{
								Kind: metadata.KindObject,
								Properties: []metadata.Property{
									{Name: "tags", Type: metadata.Metadata{
										Kind:        metadata.KindArray,
										ElementType: &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
									}, Required: false},
								},
							},
							Required: false,
						},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindVoid},
					StatusCode: 200,
					Tags:       []string{"Filter"},
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	jsonBytes, err := doc.ToJSON()
	if err != nil {
		t.Fatalf("failed to serialize to JSON: %v", err)
	}
	jsonStr := string(jsonBytes)

	// The JSON should contain style and explode for the tags param
	if !contains(jsonStr, `"style": "form"`) {
		t.Error("expected JSON to contain style: form")
	}
	if !contains(jsonStr, `"explode": true`) {
		t.Error("expected JSON to contain explode: true")
	}
}

func TestGenerator_ScalarQueryParamNoStyleExplode(t *testing.T) {
	// Scalar query params should NOT have style or explode in the JSON output.
	registry := metadata.NewTypeRegistry()
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
					Parameters: []analyzer.RouteParameter{
						{
							Category: "query",
							Name:     "",
							Type: metadata.Metadata{
								Kind: metadata.KindObject,
								Properties: []metadata.Property{
									{Name: "q", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
									{Name: "limit", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: false},
								},
							},
							Required: true,
						},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindVoid},
					StatusCode: 200,
					Tags:       []string{"Search"},
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	jsonBytes, err := doc.ToJSON()
	if err != nil {
		t.Fatalf("failed to serialize to JSON: %v", err)
	}
	jsonStr := string(jsonBytes)

	// Scalar params should NOT have style or explode
	if contains(jsonStr, `"style"`) {
		t.Error("scalar query params should NOT have style field in JSON")
	}
	if contains(jsonStr, `"explode"`) {
		t.Error("scalar query params should NOT have explode field in JSON")
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// --- Feature 2+3 combined: Named enum in query array ---

func TestGenerator_NamedEnumArrayQueryParam(t *testing.T) {
	// Array of named enum in query params should produce both $ref and style/explode.
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "OrderController",
			Path: "orders",
			Routes: []analyzer.Route{
				{
					Method:      "GET",
					Path:        "/orders",
					OperationID: "findOrders",
					Parameters: []analyzer.RouteParameter{
						{
							Category: "query",
							Name:     "",
							Type: metadata.Metadata{
								Kind: metadata.KindObject,
								Properties: []metadata.Property{
									{Name: "statuses", Type: metadata.Metadata{
										Kind: metadata.KindArray,
										ElementType: &metadata.Metadata{
											Kind: metadata.KindUnion,
											Name: "OrderStatus",
											UnionMembers: []metadata.Metadata{
												{Kind: metadata.KindLiteral, LiteralValue: "PENDING"},
												{Kind: metadata.KindLiteral, LiteralValue: "SHIPPED"},
												{Kind: metadata.KindLiteral, LiteralValue: "DELIVERED"},
											},
										},
									}, Required: false},
								},
							},
							Required: false,
						},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindVoid},
					StatusCode: 200,
					Tags:       []string{"Order"},
				},
			},
		},
	}

	doc := gen.Generate(controllers)

	ordersPath, ok := doc.Paths["/orders"]
	if !ok {
		t.Fatal("expected /orders path")
	}

	params := ordersPath.Get.Parameters
	if len(params) != 1 {
		t.Fatalf("expected 1 parameter, got %d", len(params))
	}

	statusesParam := params[0]
	if statusesParam.Name != "statuses" {
		t.Errorf("expected param name='statuses', got %q", statusesParam.Name)
	}
	if statusesParam.Style != "form" {
		t.Errorf("expected style='form', got %q", statusesParam.Style)
	}
	if statusesParam.Explode == nil || !*statusesParam.Explode {
		t.Error("expected explode=true")
	}

	// The array items should reference OrderStatus
	if statusesParam.Schema == nil || statusesParam.Schema.Type != "array" {
		t.Fatal("expected array schema")
	}
	if statusesParam.Schema.Items == nil {
		t.Fatal("expected items in array schema")
	}
	if statusesParam.Schema.Items.Ref != "#/components/schemas/OrderStatus" {
		t.Errorf("expected items.$ref='#/components/schemas/OrderStatus', got %q", statusesParam.Schema.Items.Ref)
	}

	// OrderStatus should be registered as a component
	if doc.Components == nil || doc.Components.Schemas == nil {
		t.Fatal("expected components.schemas to be present")
	}
	if _, ok := doc.Components.Schemas["OrderStatus"]; !ok {
		t.Error("expected OrderStatus in components.schemas")
	}
}

// --- SSE Support Tests ---

func TestGenerator_SSEEndpoint_BasicEventStream(t *testing.T) {
	// An SSE route should produce text/event-stream response with itemSchema.
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "EventsController",
			Path: "events",
			Routes: []analyzer.Route{
				{
					Method:      "GET",
					Path:        "/events",
					OperationID: "streamEvents",
					ReturnType:  metadata.Metadata{Kind: metadata.KindAny, Name: "MessageEvent"},
					StatusCode:  200,
					Tags:        []string{"Events"},
					IsSSE:       true,
				},
			},
		},
	}

	doc := gen.Generate(controllers)

	// Verify OpenAPI version is 3.2.0
	if doc.OpenAPI != "3.2.0" {
		t.Errorf("expected openapi='3.2.0', got %q", doc.OpenAPI)
	}

	eventsPath, ok := doc.Paths["/events"]
	if !ok {
		t.Fatal("expected /events path")
	}
	if eventsPath.Get == nil {
		t.Fatal("expected GET operation on /events")
	}

	op := eventsPath.Get
	resp, ok := op.Responses["200"]
	if !ok {
		t.Fatal("expected 200 response")
	}

	if resp.Description != "Server-Sent Events stream" {
		t.Errorf("expected description='Server-Sent Events stream', got %q", resp.Description)
	}

	// Check content type is text/event-stream
	sseMedia, ok := resp.Content["text/event-stream"]
	if !ok {
		t.Fatal("expected text/event-stream content type")
	}

	// Should use itemSchema (not schema)
	if sseMedia.Schema != nil {
		t.Error("SSE response should NOT have schema (should use itemSchema)")
	}
	if sseMedia.ItemSchema == nil {
		t.Fatal("SSE response should have itemSchema")
	}

	// Check the SSE event schema structure
	eventSchema := sseMedia.ItemSchema
	if eventSchema.Type != "object" {
		t.Errorf("expected type='object', got %q", eventSchema.Type)
	}
	if len(eventSchema.Required) != 1 || eventSchema.Required[0] != "data" {
		t.Errorf("expected required=['data'], got %v", eventSchema.Required)
	}

	// Check all 4 SSE fields
	if _, ok := eventSchema.Properties["data"]; !ok {
		t.Error("expected 'data' property")
	}
	if _, ok := eventSchema.Properties["event"]; !ok {
		t.Error("expected 'event' property")
	}
	if _, ok := eventSchema.Properties["id"]; !ok {
		t.Error("expected 'id' property")
	}
	retryProp, ok := eventSchema.Properties["retry"]
	if !ok {
		t.Error("expected 'retry' property")
	} else {
		if retryProp.Type != "integer" {
			t.Errorf("expected retry type='integer', got %q", retryProp.Type)
		}
		if retryProp.Minimum == nil || *retryProp.Minimum != 0 {
			t.Error("expected retry minimum=0")
		}
	}
}

func TestGenerator_SSEEndpoint_NoApplicationJSON(t *testing.T) {
	// SSE endpoints should NOT have application/json in their response.
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "EventsController",
			Path: "events",
			Routes: []analyzer.Route{
				{
					Method:      "GET",
					Path:        "/events",
					OperationID: "streamEvents",
					ReturnType:  metadata.Metadata{Kind: metadata.KindAny},
					StatusCode:  200,
					IsSSE:       true,
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	resp := doc.Paths["/events"].Get.Responses["200"]

	if _, ok := resp.Content["application/json"]; ok {
		t.Error("SSE response should NOT contain application/json content type")
	}
	if _, ok := resp.Content["text/event-stream"]; !ok {
		t.Error("SSE response should contain text/event-stream content type")
	}
}

func TestGenerator_SSEEndpoint_WithTypedData(t *testing.T) {
	// When the return type is a known DTO (not MessageEvent), the SSE event
	// schema should have contentMediaType + contentSchema on the data field.
	registry := metadata.NewTypeRegistry()
	registry.Register("ChatEvent", &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "ChatEvent",
		Properties: []metadata.Property{
			{Name: "type", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "payload", Type: metadata.Metadata{Kind: metadata.KindAny}, Required: true},
		},
	})

	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "ChatEventsController",
			Path: "chat/events",
			Routes: []analyzer.Route{
				{
					Method:      "GET",
					Path:        "/chat/events",
					OperationID: "streamChatEvents",
					ReturnType:  metadata.Metadata{Kind: metadata.KindRef, Ref: "ChatEvent"},
					StatusCode:  200,
					Tags:        []string{"Chat"},
					IsSSE:       true,
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	resp := doc.Paths["/chat/events"].Get.Responses["200"]
	sseMedia := resp.Content["text/event-stream"]
	eventSchema := sseMedia.ItemSchema

	dataProp := eventSchema.Properties["data"]
	if dataProp == nil {
		t.Fatal("expected 'data' property")
	}

	// data should have contentMediaType and contentSchema
	if dataProp.ContentMediaType != "application/json" {
		t.Errorf("expected data.contentMediaType='application/json', got %q", dataProp.ContentMediaType)
	}
	if dataProp.ContentSchema == nil {
		t.Fatal("expected data.contentSchema to be set for typed SSE data")
	}
	if dataProp.ContentSchema.Ref != "#/components/schemas/ChatEvent" {
		t.Errorf("expected contentSchema.$ref='#/components/schemas/ChatEvent', got %q", dataProp.ContentSchema.Ref)
	}
}

func TestGenerator_SSEEndpoint_MessageEventGeneric(t *testing.T) {
	// When the return type is NestJS's MessageEvent, data should be a plain
	// string without contentSchema (generic SSE envelope).
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "NotificationController",
			Path: "notifications",
			Routes: []analyzer.Route{
				{
					Method:      "GET",
					Path:        "/notifications/events",
					OperationID: "streamNotifications",
					// Observable<MessageEvent> unwraps to MessageEvent → walker sees object named "MessageEvent"
					ReturnType: metadata.Metadata{Kind: metadata.KindObject, Name: "MessageEvent"},
					StatusCode: 200,
					IsSSE:      true,
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	resp := doc.Paths["/notifications/events"].Get.Responses["200"]
	sseMedia := resp.Content["text/event-stream"]
	eventSchema := sseMedia.ItemSchema

	dataProp := eventSchema.Properties["data"]
	if dataProp == nil {
		t.Fatal("expected 'data' property")
	}

	// Generic MessageEvent → no contentSchema
	if dataProp.ContentMediaType != "" {
		t.Errorf("expected no contentMediaType for generic MessageEvent, got %q", dataProp.ContentMediaType)
	}
	if dataProp.ContentSchema != nil {
		t.Error("expected no contentSchema for generic MessageEvent")
	}
	if dataProp.Type != "string" {
		t.Errorf("expected data type='string', got %q", dataProp.Type)
	}
}

func TestGenerator_SSEEndpoint_JSON(t *testing.T) {
	// Verify the JSON output structure for an SSE endpoint.
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "EventsController",
			Path: "events",
			Routes: []analyzer.Route{
				{
					Method:      "GET",
					Path:        "/events/stream",
					OperationID: "stream",
					ReturnType:  metadata.Metadata{Kind: metadata.KindAny},
					StatusCode:  200,
					IsSSE:       true,
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	jsonBytes, err := doc.ToJSON()
	if err != nil {
		t.Fatalf("failed to serialize: %v", err)
	}
	jsonStr := string(jsonBytes)

	// Should contain text/event-stream
	if !contains(jsonStr, `"text/event-stream"`) {
		t.Error("JSON should contain text/event-stream")
	}
	// Should contain itemSchema
	if !contains(jsonStr, `"itemSchema"`) {
		t.Error("JSON should contain itemSchema")
	}
	// Should contain required data field
	if !contains(jsonStr, `"data"`) {
		t.Error("JSON should contain data field")
	}
	// Should contain event field
	if !contains(jsonStr, `"event"`) {
		t.Error("JSON should contain event field")
	}
	// Should NOT contain application/json for SSE endpoint
	if contains(jsonStr, `"application/json"`) {
		t.Error("SSE endpoint should NOT contain application/json")
	}
}

func TestGenerator_SSEAndRegularEndpoints(t *testing.T) {
	// A controller with both SSE and regular endpoints should produce both
	// text/event-stream and application/json responses appropriately.
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "ChatController",
			Path: "chat",
			Routes: []analyzer.Route{
				{
					Method:      "GET",
					Path:        "/chat/messages",
					OperationID: "getMessages",
					ReturnType:  metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}},
					StatusCode:  200,
					Tags:        []string{"Chat"},
				},
				{
					Method:      "GET",
					Path:        "/chat/events",
					OperationID: "streamEvents",
					ReturnType:  metadata.Metadata{Kind: metadata.KindAny},
					StatusCode:  200,
					Tags:        []string{"Chat"},
					IsSSE:       true,
				},
			},
		},
	}

	doc := gen.Generate(controllers)

	// Regular endpoint should have application/json
	messagesPath := doc.Paths["/chat/messages"]
	if messagesPath == nil || messagesPath.Get == nil {
		t.Fatal("expected /chat/messages GET")
	}
	msgResp := messagesPath.Get.Responses["200"]
	if _, ok := msgResp.Content["application/json"]; !ok {
		t.Error("regular endpoint should have application/json")
	}
	if _, ok := msgResp.Content["text/event-stream"]; ok {
		t.Error("regular endpoint should NOT have text/event-stream")
	}

	// SSE endpoint should have text/event-stream
	eventsPath := doc.Paths["/chat/events"]
	if eventsPath == nil || eventsPath.Get == nil {
		t.Fatal("expected /chat/events GET")
	}
	sseResp := eventsPath.Get.Responses["200"]
	if _, ok := sseResp.Content["text/event-stream"]; !ok {
		t.Error("SSE endpoint should have text/event-stream")
	}
	if _, ok := sseResp.Content["application/json"]; ok {
		t.Error("SSE endpoint should NOT have application/json")
	}
}

func TestGenerator_SSEWithQueryParams(t *testing.T) {
	// SSE endpoints often use query params for auth (token).
	// Verify query params are still decomposed correctly for SSE routes.
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "EventsController",
			Path: "events",
			Routes: []analyzer.Route{
				{
					Method:      "GET",
					Path:        "/events/stream",
					OperationID: "stream",
					Parameters: []analyzer.RouteParameter{
						{
							Category: "query",
							Name:     "",
							Type: metadata.Metadata{
								Kind: metadata.KindObject,
								Properties: []metadata.Property{
									{Name: "token", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
								},
							},
							Required: true,
						},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindAny},
					StatusCode: 200,
					IsSSE:      true,
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	op := doc.Paths["/events/stream"].Get

	// Should have the token query parameter
	if len(op.Parameters) != 1 {
		t.Fatalf("expected 1 parameter, got %d", len(op.Parameters))
	}
	if op.Parameters[0].Name != "token" {
		t.Errorf("expected param name='token', got %q", op.Parameters[0].Name)
	}
	if op.Parameters[0].In != "query" {
		t.Errorf("expected param in='query', got %q", op.Parameters[0].In)
	}

	// Should still have SSE response
	resp := op.Responses["200"]
	if _, ok := resp.Content["text/event-stream"]; !ok {
		t.Error("expected text/event-stream response")
	}
}

func TestSchemaGenerator_ReadOnlyProperty(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	gen := NewSchemaGenerator(reg)

	m := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "UserDto",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true, Readonly: true},
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}

	schema := gen.MetadataToSchema(m)
	// It should return a $ref, look up the registered schema
	registered := gen.Schemas()["UserDto"]
	if registered == nil {
		t.Fatal("expected UserDto to be registered")
	}

	idSchema := registered.Properties["id"]
	if idSchema == nil {
		t.Fatal("expected 'id' property in schema")
	}
	if idSchema.ReadOnly == nil || !*idSchema.ReadOnly {
		t.Error("expected readOnly=true for 'id' property")
	}

	nameSchema := registered.Properties["name"]
	if nameSchema == nil {
		t.Fatal("expected 'name' property in schema")
	}
	if nameSchema.ReadOnly != nil {
		t.Error("expected readOnly to be nil for 'name' property")
	}

	// Verify JSON output contains readOnly
	data, _ := json.Marshal(registered)
	jsonStr := string(data)
	if !strings.Contains(jsonStr, `"readOnly":true`) {
		t.Errorf("expected JSON to contain readOnly:true, got: %s", jsonStr)
	}

	_ = schema
}

func TestGenerator_IgnoreOpenAPI(t *testing.T) {
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "VisibleController",
			Path: "visible",
			Routes: []analyzer.Route{
				{Method: "GET", Path: "/visible", OperationID: "getVisible", StatusCode: 200, ReturnType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Tags: []string{"Visible"}},
			},
		},
		{
			Name:          "HiddenController",
			Path:          "hidden",
			IgnoreOpenAPI: true,
			Routes: []analyzer.Route{
				{Method: "GET", Path: "/hidden", OperationID: "getHidden", StatusCode: 200, ReturnType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Tags: []string{"Hidden"}},
			},
		},
	}

	doc := gen.Generate(controllers)

	if _, ok := doc.Paths["/visible"]; !ok {
		t.Error("expected /visible path to be in OpenAPI document")
	}
	if _, ok := doc.Paths["/hidden"]; ok {
		t.Error("expected /hidden path to be excluded from OpenAPI document (IgnoreOpenAPI=true)")
	}
}

// --- @EventStream SSE Tests ---

func TestGenerator_EventStream_DiscriminatedUnion(t *testing.T) {
	// @EventStream with multiple event types should produce oneOf with
	// discriminator + error variant.
	registry := metadata.NewTypeRegistry()
	registry.Register("UserDto", &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "UserDto",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	})
	registry.Register("DeletePayload", &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "DeletePayload",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	})
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "UserEventController",
			Path: "users",
			Routes: []analyzer.Route{
				{
					Method:        "GET",
					Path:          "/users/events",
					OperationID:   "User_streamUserEvents",
					ReturnType:    metadata.Metadata{Kind: metadata.KindAny}, // placeholder
					StatusCode:    200,
					Tags:          []string{"User"},
					IsSSE:         true,
					IsEventStream: true,
					SSEEventVariants: []analyzer.SSEEventVariant{
						{EventName: "created", DataType: metadata.Metadata{Kind: metadata.KindRef, Ref: "UserDto"}},
						{EventName: "deleted", DataType: metadata.Metadata{Kind: metadata.KindRef, Ref: "DeletePayload"}},
					},
				},
			},
		},
	}

	doc := gen.Generate(controllers)

	resp := doc.Paths["/users/events"].Get.Responses["200"]
	if resp == nil {
		t.Fatal("expected 200 response")
	}

	sseMedia, ok := resp.Content["text/event-stream"]
	if !ok {
		t.Fatal("expected text/event-stream content type")
	}
	if sseMedia.ItemSchema == nil {
		t.Fatal("expected itemSchema")
	}

	schema := sseMedia.ItemSchema

	// Should have oneOf with 3 entries: created, deleted, error
	if len(schema.OneOf) != 3 {
		t.Fatalf("expected 3 oneOf entries (2 events + error), got %d", len(schema.OneOf))
	}

	// Should have discriminator on "event"
	if schema.Discriminator == nil {
		t.Fatal("expected discriminator")
	}
	if schema.Discriminator.PropertyName != "event" {
		t.Errorf("expected discriminator propertyName='event', got %q", schema.Discriminator.PropertyName)
	}

	// Check first variant: created
	created := schema.OneOf[0]
	if created.Type != "object" {
		t.Errorf("expected created type='object', got %q", created.Type)
	}
	if created.Properties["event"].Const != "created" {
		t.Errorf("expected created event.const='created', got %v", created.Properties["event"].Const)
	}
	if created.Properties["data"].ContentSchema == nil {
		t.Error("expected created data.contentSchema")
	}
	if created.Properties["data"].ContentSchema.Ref != "#/components/schemas/UserDto" {
		t.Errorf("expected created data contentSchema $ref to UserDto, got %q", created.Properties["data"].ContentSchema.Ref)
	}

	// Check second variant: deleted
	deleted := schema.OneOf[1]
	if deleted.Properties["event"].Const != "deleted" {
		t.Errorf("expected deleted event.const='deleted', got %v", deleted.Properties["event"].Const)
	}
	if deleted.Properties["data"].ContentSchema.Ref != "#/components/schemas/DeletePayload" {
		t.Errorf("expected deleted data contentSchema $ref to DeletePayload, got %q", deleted.Properties["data"].ContentSchema.Ref)
	}

	// Check error variant (always last)
	errVariant := schema.OneOf[2]
	if errVariant.Properties["event"].Const != "error" {
		t.Errorf("expected error event.const='error', got %v", errVariant.Properties["event"].Const)
	}
	if errVariant.Properties["data"].ContentSchema != nil {
		t.Error("error variant data should NOT have contentSchema (plain string)")
	}
}

func TestGenerator_EventStream_SingleEvent(t *testing.T) {
	// Single typed event: SseEvent<'notification', NotificationDto>
	registry := metadata.NewTypeRegistry()
	registry.Register("NotificationDto", &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "NotificationDto",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "message", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	})
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "NotificationController",
			Path: "notifications",
			Routes: []analyzer.Route{
				{
					Method:        "GET",
					Path:          "/notifications/stream",
					OperationID:   "Notification_streamNotifications",
					ReturnType:    metadata.Metadata{Kind: metadata.KindAny},
					StatusCode:    200,
					Tags:          []string{"Notification"},
					IsSSE:         true,
					IsEventStream: true,
					SSEEventVariants: []analyzer.SSEEventVariant{
						{EventName: "notification", DataType: metadata.Metadata{Kind: metadata.KindRef, Ref: "NotificationDto"}},
					},
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	schema := doc.Paths["/notifications/stream"].Get.Responses["200"].Content["text/event-stream"].ItemSchema

	// oneOf: [notification, error] = 2 entries
	if len(schema.OneOf) != 2 {
		t.Fatalf("expected 2 oneOf entries (1 event + error), got %d", len(schema.OneOf))
	}

	// Discriminator should be present even for single literal event
	if schema.Discriminator == nil || schema.Discriminator.PropertyName != "event" {
		t.Error("expected discriminator on 'event'")
	}

	notification := schema.OneOf[0]
	if notification.Properties["event"].Const != "notification" {
		t.Errorf("expected event.const='notification', got %v", notification.Properties["event"].Const)
	}
}

func TestGenerator_EventStream_GenericString_NoDiscriminator(t *testing.T) {
	// Non-discriminated: SseEvent<string, UserDto> → no discriminator, '*' wildcard
	registry := metadata.NewTypeRegistry()
	registry.Register("UserDto", &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "UserDto",
	})
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "GenericController",
			Path: "generic",
			Routes: []analyzer.Route{
				{
					Method:        "GET",
					Path:          "/generic/stream",
					OperationID:   "Generic_streamGeneric",
					ReturnType:    metadata.Metadata{Kind: metadata.KindAny},
					StatusCode:    200,
					Tags:          []string{"Generic"},
					IsSSE:         true,
					IsEventStream: true,
					SSEEventVariants: []analyzer.SSEEventVariant{
						{EventName: "", DataType: metadata.Metadata{Kind: metadata.KindRef, Ref: "UserDto"}},
					},
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	schema := doc.Paths["/generic/stream"].Get.Responses["200"].Content["text/event-stream"].ItemSchema

	// oneOf: [generic event, error] = 2 entries
	if len(schema.OneOf) != 2 {
		t.Fatalf("expected 2 oneOf entries, got %d", len(schema.OneOf))
	}

	// No discriminator for generic string event name
	if schema.Discriminator != nil {
		t.Error("expected no discriminator for generic string event name")
	}

	// Generic variant should NOT have const on event
	genericVariant := schema.OneOf[0]
	if genericVariant.Properties["event"].Const != nil {
		t.Errorf("expected no const on generic event, got %v", genericVariant.Properties["event"].Const)
	}
}

func TestGenerator_EventStream_LegacySse_NoErrorVariant(t *testing.T) {
	// Legacy @Sse (IsSSE=true, IsEventStream=false) should use the old schema
	// without error variant or oneOf.
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)

	controllers := []analyzer.ControllerInfo{
		{
			Name: "LegacyController",
			Path: "legacy",
			Routes: []analyzer.Route{
				{
					Method:        "GET",
					Path:          "/legacy/events",
					OperationID:   "Legacy_stream",
					ReturnType:    metadata.Metadata{Kind: metadata.KindAny, Name: "MessageEvent"},
					StatusCode:    200,
					Tags:          []string{"Legacy"},
					IsSSE:         true,
					IsEventStream: false,
				},
			},
		},
	}

	doc := gen.Generate(controllers)
	schema := doc.Paths["/legacy/events"].Get.Responses["200"].Content["text/event-stream"].ItemSchema

	// Legacy SSE: no oneOf, just a plain object schema
	if len(schema.OneOf) != 0 {
		t.Errorf("expected no oneOf for legacy @Sse, got %d entries", len(schema.OneOf))
	}
	if schema.Type != "object" {
		t.Errorf("expected type='object', got %q", schema.Type)
	}
	// Should NOT have discriminator
	if schema.Discriminator != nil {
		t.Error("expected no discriminator for legacy @Sse")
	}
}
