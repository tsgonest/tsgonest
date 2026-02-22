package codegen

import (
	"fmt"
	"testing"

	"github.com/tsgonest/tsgonest/internal/metadata"
)

// Benchmark validation codegen for a simple object
func BenchmarkGenerateValidation_Simple(b *testing.B) {
	registry := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "SimpleDto",
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "age", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "email", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: false},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GenerateValidation("SimpleDto", meta, registry)
	}
}

// Benchmark validation codegen for a complex nested object
func BenchmarkGenerateValidation_Complex(b *testing.B) {
	registry := metadata.NewTypeRegistry()

	addressMeta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "Address",
		Properties: []metadata.Property{
			{Name: "street", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "city", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "state", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "zip", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}
	registry.Register("Address", addressMeta)

	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "UserDto",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "email", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Format: strPtr("email")}},
			{Name: "address", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "Address"}, Required: true},
			{Name: "tags", Type: metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}}, Required: false},
			{Name: "role", Type: metadata.Metadata{Kind: metadata.KindUnion, UnionMembers: []metadata.Metadata{
				{Kind: metadata.KindLiteral, LiteralValue: "admin"},
				{Kind: metadata.KindLiteral, LiteralValue: "user"},
				{Kind: metadata.KindLiteral, LiteralValue: "guest"},
			}}, Required: true},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GenerateValidation("UserDto", meta, registry)
	}
}

// Benchmark serialization codegen
func BenchmarkGenerateSerialization_Simple(b *testing.B) {
	registry := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "SimpleDto",
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "age", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GenerateSerialization("SimpleDto", meta, registry)
	}
}

func BenchmarkGenerateSerialization_Complex(b *testing.B) {
	registry := metadata.NewTypeRegistry()
	addressMeta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "Address",
		Properties: []metadata.Property{
			{Name: "street", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "city", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}
	registry.Register("Address", addressMeta)

	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "UserDto",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "address", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "Address"}, Required: true},
			{Name: "tags", Type: metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}}, Required: false},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GenerateSerialization("UserDto", meta, registry)
	}
}

// Benchmark companion file generation (full pipeline)
func BenchmarkGenerateCompanionFiles(b *testing.B) {
	registry := metadata.NewTypeRegistry()
	types := make(map[string]*metadata.Metadata)

	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("Dto%d", i)
		meta := &metadata.Metadata{
			Kind: metadata.KindObject,
			Name: name,
			Properties: []metadata.Property{
				{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
				{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
				{Name: "value", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: false},
			},
		}
		types[name] = meta
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GenerateCompanionFiles("src/test.ts", types, registry)
	}
}

// Helper
func strPtr(s string) *string { return &s }
