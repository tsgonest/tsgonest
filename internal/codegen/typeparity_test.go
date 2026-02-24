package codegen

import (
	"strings"
	"testing"

	"github.com/tsgonest/tsgonest/internal/metadata"
)

// typeStructureCase represents a typia-style type structure test case.
type typeStructureCase struct {
	name     string
	meta     *metadata.Metadata
	registry *metadata.TypeRegistry
	// Expected substrings in validation output
	validateContains []string
	// Expected substrings in serialization output
	serializeContains []string
}

func buildTypeStructureCases() []typeStructureCase {
	cases := []typeStructureCase{
		// 1. Primitive types
		{
			name: "PrimitiveString",
			meta: &metadata.Metadata{Kind: metadata.KindObject, Name: "PrimString", Properties: []metadata.Property{
				{Name: "value", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			}},
			validateContains: []string{"typeof", "string"},
		},
		{
			name: "PrimitiveNumber",
			meta: &metadata.Metadata{Kind: metadata.KindObject, Name: "PrimNumber", Properties: []metadata.Property{
				{Name: "value", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			}},
			validateContains: []string{"typeof", "number"},
		},
		{
			name: "PrimitiveBoolean",
			meta: &metadata.Metadata{Kind: metadata.KindObject, Name: "PrimBool", Properties: []metadata.Property{
				{Name: "value", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "boolean"}, Required: true},
			}},
			validateContains: []string{"typeof", "boolean"},
		},

		// 2. Optional properties
		{
			name: "OptionalProp",
			meta: &metadata.Metadata{Kind: metadata.KindObject, Name: "OptProp", Properties: []metadata.Property{
				{Name: "required", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
				{Name: "optional", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Optional: true}, Required: false},
			}},
			validateContains: []string{"required", "optional"},
		},

		// 3. Nullable types
		{
			name: "NullableString",
			meta: &metadata.Metadata{Kind: metadata.KindObject, Name: "NullStr", Properties: []metadata.Property{
				{Name: "value", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Nullable: true}, Required: true},
			}},
			validateContains: []string{"null"},
		},

		// 4. Literal types
		{
			name: "LiteralString",
			meta: &metadata.Metadata{Kind: metadata.KindObject, Name: "LitStr", Properties: []metadata.Property{
				{Name: "status", Type: metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: "active"}, Required: true},
			}},
			validateContains: []string{"active"},
		},
		{
			name: "LiteralNumber",
			meta: &metadata.Metadata{Kind: metadata.KindObject, Name: "LitNum", Properties: []metadata.Property{
				{Name: "code", Type: metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: float64(200)}, Required: true},
			}},
			validateContains: []string{"200"},
		},

		// 5. Array types
		{
			name: "ArrayOfStrings",
			meta: &metadata.Metadata{Kind: metadata.KindObject, Name: "ArrStr", Properties: []metadata.Property{
				{Name: "items", Type: metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}}, Required: true},
			}},
			validateContains: []string{"Array.isArray"},
		},
		{
			name: "ArrayOfNumbers",
			meta: &metadata.Metadata{Kind: metadata.KindObject, Name: "ArrNum", Properties: []metadata.Property{
				{Name: "values", Type: metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}}, Required: true},
			}},
			validateContains: []string{"Array.isArray"},
		},

		// 6. Union types
		{
			name: "UnionStringNumber",
			meta: &metadata.Metadata{Kind: metadata.KindObject, Name: "UnionSN", Properties: []metadata.Property{
				{Name: "value", Type: metadata.Metadata{Kind: metadata.KindUnion, UnionMembers: []metadata.Metadata{
					{Kind: metadata.KindAtomic, Atomic: "string"},
					{Kind: metadata.KindAtomic, Atomic: "number"},
				}}, Required: true},
			}},
			validateContains: []string{"string", "number"},
		},
		{
			name: "UnionLiterals",
			meta: &metadata.Metadata{Kind: metadata.KindObject, Name: "UnionLit", Properties: []metadata.Property{
				{Name: "role", Type: metadata.Metadata{Kind: metadata.KindUnion, UnionMembers: []metadata.Metadata{
					{Kind: metadata.KindLiteral, LiteralValue: "admin"},
					{Kind: metadata.KindLiteral, LiteralValue: "user"},
					{Kind: metadata.KindLiteral, LiteralValue: "guest"},
				}}, Required: true},
			}},
			validateContains: []string{"admin", "user", "guest"},
		},

		// 7. Nested objects
		{
			name: "NestedObject",
			meta: &metadata.Metadata{Kind: metadata.KindObject, Name: "Outer", Properties: []metadata.Property{
				{Name: "inner", Type: metadata.Metadata{Kind: metadata.KindObject, Properties: []metadata.Property{
					{Name: "x", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
				}}, Required: true},
			}},
			validateContains: []string{"inner", "typeof"},
		},

		// 8. Tuple types
		{
			name: "TupleStringNumber",
			meta: &metadata.Metadata{Kind: metadata.KindObject, Name: "TupSN", Properties: []metadata.Property{
				{Name: "pair", Type: metadata.Metadata{Kind: metadata.KindTuple, Elements: []metadata.TupleElement{
					{Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}},
					{Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}},
				}}, Required: true},
			}},
			validateContains: []string{"Array.isArray", "length"},
		},

		// 9. Enum types
		{
			name: "EnumString",
			meta: &metadata.Metadata{Kind: metadata.KindObject, Name: "EnumStr", Properties: []metadata.Property{
				{Name: "color", Type: metadata.Metadata{Kind: metadata.KindEnum, EnumValues: []metadata.EnumValue{
					{Name: "Red", Value: "red"},
					{Name: "Green", Value: "green"},
					{Name: "Blue", Value: "blue"},
				}}, Required: true},
			}},
			validateContains: []string{"red", "green", "blue"},
		},

		// 10. Ref types (nested named types)
		{
			name: "RefType",
			meta: &metadata.Metadata{Kind: metadata.KindObject, Name: "Parent", Properties: []metadata.Property{
				{Name: "child", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "Child"}, Required: true},
			}},
			registry: func() *metadata.TypeRegistry {
				r := metadata.NewTypeRegistry()
				r.Register("Child", &metadata.Metadata{Kind: metadata.KindObject, Name: "Child", Properties: []metadata.Property{
					{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
				}})
				return r
			}(),
			validateContains: []string{"child"},
		},

		// 11. Constraints
		{
			name: "StringConstraints",
			meta: &metadata.Metadata{Kind: metadata.KindObject, Name: "StrCon", Properties: []metadata.Property{
				{Name: "email", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
					Constraints: &metadata.Constraints{Format: ptrStr("email")}},
				{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
					Constraints: &metadata.Constraints{MinLength: ptrInt(1), MaxLength: ptrInt(100)}},
			}},
			validateContains: []string{"email", "minLength", "maxLength"},
		},
		{
			name: "NumberConstraints",
			meta: &metadata.Metadata{Kind: metadata.KindObject, Name: "NumCon", Properties: []metadata.Property{
				{Name: "age", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true,
					Constraints: &metadata.Constraints{Minimum: ptrFloat(0), Maximum: ptrFloat(150)}},
			}},
			validateContains: []string{"0", "150"},
		},

		// 12. Index signatures (Record<string, number>)
		{
			name: "IndexSignature",
			meta: &metadata.Metadata{Kind: metadata.KindObject, Name: "IdxSig", Properties: []metadata.Property{},
				IndexSignature: &metadata.IndexSignature{
					KeyType:   metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					ValueType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"},
				},
			},
			validateContains: []string{"typeof"},
		},

		// 13. Mixed optional + nullable
		{
			name: "OptionalNullable",
			meta: &metadata.Metadata{Kind: metadata.KindObject, Name: "OptNull", Properties: []metadata.Property{
				{Name: "value", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Optional: true, Nullable: true}, Required: false},
			}},
			validateContains: []string{"value"},
		},

		// 14. Complex: user profile (realistic DTO)
		{
			name: "ComplexUserProfile",
			meta: &metadata.Metadata{Kind: metadata.KindObject, Name: "UserProfile", Properties: []metadata.Property{
				{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
				{Name: "username", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
					Constraints: &metadata.Constraints{MinLength: ptrInt(3), MaxLength: ptrInt(30)}},
				{Name: "email", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
					Constraints: &metadata.Constraints{Format: ptrStr("email")}},
				{Name: "role", Type: metadata.Metadata{Kind: metadata.KindUnion, UnionMembers: []metadata.Metadata{
					{Kind: metadata.KindLiteral, LiteralValue: "admin"},
					{Kind: metadata.KindLiteral, LiteralValue: "user"},
				}}, Required: true},
				{Name: "bio", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Optional: true}, Required: false},
				{Name: "tags", Type: metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}}, Required: false},
				{Name: "metadata", Type: metadata.Metadata{Kind: metadata.KindObject, Properties: []metadata.Property{}}, Required: false},
			}},
			validateContains: []string{"id", "username", "email", "role", "admin", "user"},
		},

		// 15. Discriminated union
		{
			name: "DiscriminatedUnion",
			meta: &metadata.Metadata{Kind: metadata.KindObject, Name: "Event", Properties: []metadata.Property{
				{Name: "event", Type: metadata.Metadata{Kind: metadata.KindUnion,
					UnionMembers: []metadata.Metadata{
						{Kind: metadata.KindRef, Ref: "ClickEvent"},
						{Kind: metadata.KindRef, Ref: "ScrollEvent"},
					},
					Discriminant: &metadata.Discriminant{Property: "type", Mapping: map[string]int{"click": 0, "scroll": 1}},
				}, Required: true},
			}},
			registry: func() *metadata.TypeRegistry {
				r := metadata.NewTypeRegistry()
				r.Register("ClickEvent", &metadata.Metadata{Kind: metadata.KindObject, Name: "ClickEvent", Properties: []metadata.Property{
					{Name: "type", Type: metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: "click"}, Required: true},
					{Name: "x", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
				}})
				r.Register("ScrollEvent", &metadata.Metadata{Kind: metadata.KindObject, Name: "ScrollEvent", Properties: []metadata.Property{
					{Name: "type", Type: metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: "scroll"}, Required: true},
					{Name: "offset", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
				}})
				return r
			}(),
			validateContains: []string{"event"},
		},

		// 16. Empty object
		{
			name:             "EmptyObject",
			meta:             &metadata.Metadata{Kind: metadata.KindObject, Name: "Empty", Properties: []metadata.Property{}},
			validateContains: []string{"typeof", "object"},
		},

		// 17. Bigint property
		{
			name: "BigintProp",
			meta: &metadata.Metadata{Kind: metadata.KindObject, Name: "BigIntProp", Properties: []metadata.Property{
				{Name: "big", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "bigint"}, Required: true},
			}},
			validateContains: []string{"bigint"},
		},

		// 18. Array of objects
		{
			name: "ArrayOfObjects",
			meta: &metadata.Metadata{Kind: metadata.KindObject, Name: "ArrObj", Properties: []metadata.Property{
				{Name: "items", Type: metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindObject, Properties: []metadata.Property{
					{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
				}}}, Required: true},
			}},
			validateContains: []string{"Array.isArray", "id"},
		},

		// 19. Multiple constraints
		{
			name: "MultipleConstraints",
			meta: &metadata.Metadata{Kind: metadata.KindObject, Name: "MultiCon", Properties: []metadata.Property{
				{Name: "score", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true,
					Constraints: &metadata.Constraints{Minimum: ptrFloat(0), Maximum: ptrFloat(100), MultipleOf: ptrFloat(0.5)}},
			}},
			validateContains: []string{"0", "100"},
		},

		// 20. Deep nesting
		{
			name: "DeepNesting",
			meta: &metadata.Metadata{Kind: metadata.KindObject, Name: "Deep", Properties: []metadata.Property{
				{Name: "level1", Type: metadata.Metadata{Kind: metadata.KindObject, Properties: []metadata.Property{
					{Name: "level2", Type: metadata.Metadata{Kind: metadata.KindObject, Properties: []metadata.Property{
						{Name: "value", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
					}}, Required: true},
				}}, Required: true},
			}},
			validateContains: []string{"level1", "level2", "value"},
		},
	}

	return cases
}

func TestTypeStructureParity_Validate(t *testing.T) {
	for _, tc := range buildTypeStructureCases() {
		t.Run(tc.name, func(t *testing.T) {
			registry := tc.registry
			if registry == nil {
				registry = metadata.NewTypeRegistry()
			}

			output := GenerateCompanionSelective(tc.name, tc.meta, registry, true, false)

			if output == "" {
				t.Fatal("expected non-empty output")
			}

			// Verify expected substrings present
			for _, expected := range tc.validateContains {
				if !strings.Contains(output, expected) {
					t.Errorf("expected output to contain %q", expected)
				}
			}

			// Verify basic structure
			if !strings.Contains(output, "export function validate") {
				t.Error("expected validate function export")
			}
			if !strings.Contains(output, "export function assert") {
				t.Error("expected assert function export")
			}

			// Standard Schema is opt-in, verify it's NOT present by default
			if strings.Contains(output, "export const schema") {
				t.Error("expected no schema export (Standard Schema is opt-in)")
			}
		})
	}
}

func TestTypeStructureParity_Serialize(t *testing.T) {
	for _, tc := range buildTypeStructureCases() {
		t.Run(tc.name, func(t *testing.T) {
			registry := tc.registry
			if registry == nil {
				registry = metadata.NewTypeRegistry()
			}

			output := GenerateCompanionSelective(tc.name, tc.meta, registry, false, true)

			if output == "" {
				t.Fatal("expected non-empty output")
			}

			if !strings.Contains(output, "export function serialize") {
				t.Error("expected serialize function export")
			}
		})
	}
}

// Helpers — shared test pointer constructors.
func ptrInt(i int) *int           { return &i }
func ptrFloat(f float64) *float64 { return &f }
func ptrStr(s string) *string     { return &s }
func ptrBool(b bool) *bool        { return &b }
