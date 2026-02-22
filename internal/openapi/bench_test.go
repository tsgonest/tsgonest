package openapi

import (
	"fmt"
	"testing"

	"github.com/tsgonest/tsgonest/internal/analyzer"
	"github.com/tsgonest/tsgonest/internal/metadata"
)

func BenchmarkGenerate_SingleController(b *testing.B) {
	registry := metadata.NewTypeRegistry()
	registry.Register("CreateUserDto", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "CreateUserDto",
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "email", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	})
	registry.Register("UserResponse", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "UserResponse",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	})

	controllers := []analyzer.ControllerInfo{
		{
			Name: "UserController", Path: "users",
			Routes: []analyzer.Route{
				{Method: "GET", Path: "/users", OperationID: "findAll", ReturnType: metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindRef, Ref: "UserResponse"}}, StatusCode: 200, Tags: []string{"User"}},
				{Method: "POST", Path: "/users", OperationID: "create", Parameters: []analyzer.RouteParameter{{Category: "body", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "CreateUserDto"}, Required: true}}, ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "UserResponse"}, StatusCode: 201, Tags: []string{"User"}},
				{Method: "GET", Path: "/users/:id", OperationID: "findOne", Parameters: []analyzer.RouteParameter{{Category: "param", Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true}}, ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "UserResponse"}, StatusCode: 200, Tags: []string{"User"}},
				{Method: "DELETE", Path: "/users/:id", OperationID: "remove", Parameters: []analyzer.RouteParameter{{Category: "param", Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true}}, ReturnType: metadata.Metadata{Kind: metadata.KindVoid}, StatusCode: 204, Tags: []string{"User"}},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen := NewGenerator(registry)
		gen.Generate(controllers)
	}
}

func BenchmarkGenerate_MultiController(b *testing.B) {
	registry := metadata.NewTypeRegistry()
	// Register 20 types
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("Dto%d", i)
		registry.Register(name, &metadata.Metadata{
			Kind: metadata.KindObject, Name: name,
			Properties: []metadata.Property{
				{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
				{Name: "value", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			},
		})
	}

	// Create 5 controllers with 4 routes each
	var controllers []analyzer.ControllerInfo
	for c := 0; c < 5; c++ {
		ctrl := analyzer.ControllerInfo{
			Name: fmt.Sprintf("Controller%d", c),
			Path: fmt.Sprintf("resource%d", c),
		}
		for r := 0; r < 4; r++ {
			ctrl.Routes = append(ctrl.Routes, analyzer.Route{
				Method:      "GET",
				Path:        fmt.Sprintf("/resource%d/route%d", c, r),
				OperationID: fmt.Sprintf("op%d_%d", c, r),
				ReturnType:  metadata.Metadata{Kind: metadata.KindRef, Ref: fmt.Sprintf("Dto%d", (c*4+r)%20)},
				StatusCode:  200,
				Tags:        []string{fmt.Sprintf("Resource%d", c)},
			})
		}
		controllers = append(controllers, ctrl)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen := NewGenerator(registry)
		gen.Generate(controllers)
	}
}

func BenchmarkToJSON(b *testing.B) {
	registry := metadata.NewTypeRegistry()
	gen := NewGenerator(registry)
	// Create a simple doc
	controllers := []analyzer.ControllerInfo{
		{Name: "HealthController", Path: "health", Routes: []analyzer.Route{
			{Method: "GET", Path: "/health", OperationID: "check", ReturnType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, StatusCode: 200, Tags: []string{"Health"}},
		}},
	}
	doc := gen.Generate(controllers)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		doc.ToJSON()
	}
}

func BenchmarkMetadataToSchema_Complex(b *testing.B) {
	registry := metadata.NewTypeRegistry()
	registry.Register("SubType", &metadata.Metadata{
		Kind: metadata.KindObject, Name: "SubType",
		Properties: []metadata.Property{
			{Name: "x", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "y", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: false},
		},
	})

	m := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "Complex",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "sub", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "SubType"}, Required: true},
			{Name: "tags", Type: metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}}, Required: false},
			{Name: "role", Type: metadata.Metadata{Kind: metadata.KindUnion, UnionMembers: []metadata.Metadata{
				{Kind: metadata.KindLiteral, LiteralValue: "admin"},
				{Kind: metadata.KindLiteral, LiteralValue: "user"},
			}}, Required: true},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen := NewSchemaGenerator(registry)
		gen.MetadataToSchema(m)
	}
}
