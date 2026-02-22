package openapi

import (
	"encoding/json"
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
	if doc.OpenAPI != "3.1.0" {
		t.Errorf("expected openapi='3.1.0', got %q", doc.OpenAPI)
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
	if parsed["openapi"] != "3.1.0" {
		t.Errorf("expected openapi='3.1.0', got %v", parsed["openapi"])
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
		OpenAPI: "3.1.0",
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
