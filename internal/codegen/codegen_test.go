package codegen

import (
	"strings"
	"testing"

	"github.com/tsgonest/tsgonest/internal/metadata"
)

// --- Validation codegen tests ---

func TestValidateSimpleObject(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "CreateUserDto",
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "age", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
		},
	}

	code := GenerateCompanionSelective("CreateUserDto", meta, reg, true, false)

	assertContains(t, code, "export function validateCreateUserDto(input)")
	assertContains(t, code, "typeof input !== \"object\"")
	assertContains(t, code, "typeof input.name !== \"string\"")
	assertContains(t, code, "typeof input.age !== \"number\"")
	assertContains(t, code, "export function assertCreateUserDto(input)")
}

func TestValidateOptionalProp(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "bio", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Optional: true}, Required: false},
		},
	}

	code := GenerateCompanionSelective("Profile", meta, reg, true, false)

	// Required prop should have undefined check
	assertContains(t, code, "input.name === undefined")
	// Optional prop should have if (input.bio !== undefined)
	assertContains(t, code, "input.bio !== undefined")
}

func TestValidateNullableProp(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "value", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Nullable: true}, Required: true},
		},
	}

	code := GenerateCompanionSelective("Box", meta, reg, true, false)
	assertContains(t, code, "input.value !== null")
}

func TestValidateArray(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	elemType := metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "tags", Type: metadata.Metadata{Kind: metadata.KindArray, ElementType: &elemType}, Required: true},
		},
	}

	code := GenerateCompanionSelective("Post", meta, reg, true, false)
	assertContains(t, code, "Array.isArray(input.tags)")
	// Depth is 1 inside object check, so loop var is i1
	assertContains(t, code, "typeof input.tags[i1] !== \"string\"")
}

func TestValidateLiteralUnion(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "status", Type: metadata.Metadata{
				Kind: metadata.KindUnion,
				UnionMembers: []metadata.Metadata{
					{Kind: metadata.KindLiteral, LiteralValue: "active"},
					{Kind: metadata.KindLiteral, LiteralValue: "inactive"},
				},
			}, Required: true},
		},
	}

	code := GenerateCompanionSelective("Account", meta, reg, true, false)
	assertContains(t, code, "\"active\"")
	assertContains(t, code, "\"inactive\"")
	assertContains(t, code, ".includes(input.status)")
}

func TestValidateLiteralUnionEscapedQuotes(t *testing.T) {
	// Regression test: literal union expected message must not contain
	// unescaped double quotes that break the JS string literal.
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "role", Type: metadata.Metadata{
				Kind: metadata.KindUnion,
				UnionMembers: []metadata.Metadata{
					{Kind: metadata.KindLiteral, LiteralValue: "admin"},
					{Kind: metadata.KindLiteral, LiteralValue: "moderator"},
					{Kind: metadata.KindLiteral, LiteralValue: "user"},
				},
			}, Required: true},
		},
	}

	code := GenerateCompanionSelective("UserRole", meta, reg, true, false)

	// The expected message must have escaped quotes so the JS is valid.
	// e.g. expected: "one of \"admin\" | \"moderator\" | \"user\""
	assertContains(t, code, `one of \"admin\" | \"moderator\" | \"user\"`)

	// Must NOT contain unescaped double quotes inside a string
	// (the broken pattern: expected: "one of "admin" | ...)
	assertNotContains(t, code, `expected: "one of "admin"`)
}

func TestValidateNestedObject(t *testing.T) {
	reg := metadata.NewTypeRegistry()

	addressMeta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "Address",
		Properties: []metadata.Property{
			{Name: "street", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "city", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}
	reg.Register("Address", addressMeta)

	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "address", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "Address"}, Required: true},
		},
	}

	code := GenerateCompanionSelective("User", meta, reg, true, false)
	assertContains(t, code, "typeof input.address !== \"object\"")
	assertContains(t, code, "typeof input.address.street !== \"string\"")
	assertContains(t, code, "typeof input.address.city !== \"string\"")
}

func TestValidateNativeDate(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "createdAt", Type: metadata.Metadata{Kind: metadata.KindNative, NativeType: "Date"}, Required: true},
		},
	}

	code := GenerateCompanionSelective("Event", meta, reg, true, false)
	assertContains(t, code, "instanceof Date")
}

func TestValidateTuple(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "point", Type: metadata.Metadata{
				Kind: metadata.KindTuple,
				Elements: []metadata.TupleElement{
					{Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}},
					{Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}},
				},
			}, Required: true},
		},
	}

	code := GenerateCompanionSelective("Shape", meta, reg, true, false)
	assertContains(t, code, "Array.isArray(input.point)")
	assertContains(t, code, "input.point[0]")
	assertContains(t, code, "input.point[1]")
}

// --- Serialization codegen tests ---

func TestSerializeSimpleObject(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "UserResponse",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}

	code := GenerateCompanionSelective("UserResponse", meta, reg, false, true)
	assertContains(t, code, "export function serializeUserResponse(input)")
	assertContains(t, code, "__s(input.name)")
	assertContains(t, code, `\"id\"`)
}

func TestSerializeWithDate(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "createdAt", Type: metadata.Metadata{Kind: metadata.KindNative, NativeType: "Date"}, Required: true},
		},
	}

	code := GenerateCompanionSelective("Event", meta, reg, false, true)
	assertContains(t, code, "toISOString()")
}

func TestSerializeArray(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	elemType := metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "scores", Type: metadata.Metadata{Kind: metadata.KindArray, ElementType: &elemType}, Required: true},
		},
	}

	code := GenerateCompanionSelective("Result", meta, reg, false, true)
	// Array serialization uses for-loop IIFE instead of .map().join()
	assertContains(t, code, "for (var")
	assertContains(t, code, `+= ","`)
	assertContains(t, code, `+ "]"`)
}

func TestSerializeOptionalProps(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "UpdateUserDto",
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Optional: true}, Required: false},
			{Name: "age", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number", Optional: true}, Required: false},
		},
	}

	code := GenerateCompanionSelective("UpdateUserDto", meta, reg, false, true)

	// Should NOT fall back to JSON.stringify for the whole object
	assertNotContains(t, code, "return JSON.stringify(input)")
	// Should have conditional key inclusion
	assertContains(t, code, "input.name !== undefined")
	assertContains(t, code, "input.age !== undefined")
	// Should use ternary-based optional inclusion with comma-separated keys
	assertContains(t, code, `",\"name\":"`)
	assertContains(t, code, `",\"age\":"`)


}

func TestSerializeMixedOptionalRequired(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "Profile",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "bio", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Optional: true}, Required: false},
		},
	}

	code := GenerateCompanionSelective("Profile", meta, reg, false, true)

	// Required props should be in the template literal portion
	assertContains(t, code, `\"id\":`)
	assertContains(t, code, `\"name\":`)
	// Optional prop should have conditional check
	assertContains(t, code, "input.bio !== undefined")
}

func TestSerializeNestedObject(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	addressMeta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "Address",
		Properties: []metadata.Property{
			{Name: "city", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}
	reg.Register("Address", addressMeta)

	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "address", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "Address"}, Required: true},
		},
	}

	code := GenerateCompanionSelective("User", meta, reg, false, true)
	assertContains(t, code, "__s(input.address.city)")
}

func TestCompanionPath(t *testing.T) {
	tests := []struct {
		source   string
		typeName string
		expected string
	}{
		{"src/user.dto.ts", "CreateUserDto", "src/user.dto.CreateUserDto.tsgonest.js"},
		{"src/user.dto.tsx", "User", "src/user.dto.User.tsgonest.js"},
		{"src/app.ts", "Config", "src/app.Config.tsgonest.js"},
	}

	for _, tc := range tests {
		got := companionPath(tc.source, tc.typeName)
		if got != tc.expected {
			t.Errorf("companionPath(%q, %q) = %q, want %q", tc.source, tc.typeName, got, tc.expected)
		}
	}
}

// --- Constraint validation codegen tests ---

func TestValidateMinMax(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	min := 0.0
	max := 150.0
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name:     "age",
				Type:     metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"},
				Required: true,
				Constraints: &metadata.Constraints{
					Minimum: &min,
					Maximum: &max,
				},
			},
		},
	}

	code := GenerateCompanionSelective("Person", meta, reg, true, false)
	assertContains(t, code, "input.age < 0")
	assertContains(t, code, "input.age > 150")
	assertContains(t, code, "minimum 0")
	assertContains(t, code, "maximum 150")
}

func TestValidateMinMaxLength(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	minLen := 1
	maxLen := 255
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name:     "name",
				Type:     metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
				Required: true,
				Constraints: &metadata.Constraints{
					MinLength: &minLen,
					MaxLength: &maxLen,
				},
			},
		},
	}

	code := GenerateCompanionSelective("User", meta, reg, true, false)
	assertContains(t, code, "input.name.length < 1")
	assertContains(t, code, "input.name.length > 255")
	assertContains(t, code, "minLength 1")
	assertContains(t, code, "maxLength 255")
}

func TestValidatePattern(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	pattern := "^[a-z]+$"
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name:     "slug",
				Type:     metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
				Required: true,
				Constraints: &metadata.Constraints{
					Pattern: &pattern,
				},
			},
		},
	}

	code := GenerateCompanionSelective("Post", meta, reg, true, false)
	assertContains(t, code, "/^[a-z]+$/.test(input.slug)")
}

func TestValidateFormatEmail(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	format := "email"
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name:     "email",
				Type:     metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
				Required: true,
				Constraints: &metadata.Constraints{
					Format: &format,
				},
			},
		},
	}

	code := GenerateCompanionSelective("Contact", meta, reg, true, false)
	assertContains(t, code, "format email")
	assertContains(t, code, "@")
}

func TestValidateConstraintsOnOptionalProp(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	maxLen := 500
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name:     "bio",
				Type:     metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Optional: true},
				Required: false,
				Constraints: &metadata.Constraints{
					MaxLength: &maxLen,
				},
			},
		},
	}

	code := GenerateCompanionSelective("Profile", meta, reg, true, false)
	// Should only check constraint when value is present
	assertContains(t, code, "input.bio !== undefined")
	assertContains(t, code, "input.bio.length > 500")
}

// --- Phase 8: New constraint validation codegen tests ---

func TestValidateExclusiveMinMax(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	exMin := 0.0
	exMax := 100.0
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name:     "threshold",
				Type:     metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"},
				Required: true,
				Constraints: &metadata.Constraints{
					ExclusiveMinimum: &exMin,
					ExclusiveMaximum: &exMax,
				},
			},
		},
	}

	code := GenerateCompanionSelective("Config", meta, reg, true, false)
	assertContains(t, code, "input.threshold <= 0")
	assertContains(t, code, "input.threshold >= 100")
	assertContains(t, code, "exclusiveMinimum 0")
	assertContains(t, code, "exclusiveMaximum 100")
}

func TestValidateMultipleOf(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	mult := 5.0
	meta := &metadata.Metadata{
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

	code := GenerateCompanionSelective("Grid", meta, reg, true, false)
	assertContains(t, code, "input.step % 5 !== 0")
	assertContains(t, code, "multipleOf 5")
}

func TestValidateNumericTypeInt32(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	numType := "int32"
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name:     "port",
				Type:     metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"},
				Required: true,
				Constraints: &metadata.Constraints{
					NumericType: &numType,
				},
			},
		},
	}

	code := GenerateCompanionSelective("Server", meta, reg, true, false)
	assertContains(t, code, "Number.isInteger(input.port)")
	assertContains(t, code, "-2147483648")
	assertContains(t, code, "2147483647")
	assertContains(t, code, "int32")
}

func TestValidateNumericTypeUint32(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	numType := "uint32"
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name:     "count",
				Type:     metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"},
				Required: true,
				Constraints: &metadata.Constraints{
					NumericType: &numType,
				},
			},
		},
	}

	code := GenerateCompanionSelective("Counter", meta, reg, true, false)
	assertContains(t, code, "Number.isInteger(input.count)")
	assertContains(t, code, "input.count < 0")
	assertContains(t, code, "4294967295")
	assertContains(t, code, "uint32")
}

func TestValidateUniqueItems(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	unique := true
	elemType := metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}
	meta := &metadata.Metadata{
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

	code := GenerateCompanionSelective("TagList", meta, reg, true, false)
	assertContains(t, code, "new Set(input.tags).size !== input.tags.length")
	assertContains(t, code, "uniqueItems")
}

// --- Phase 8: Format validation codegen tests ---

func TestValidateFormatIPv4(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	format := "ipv4"
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "ip", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Format: &format}},
		},
	}
	code := GenerateCompanionSelective("Server", meta, reg, true, false)
	assertContains(t, code, "format ipv4")
	assertContains(t, code, ".test(input.ip)")
}

func TestValidateFormatIPv6(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	format := "ipv6"
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "ip", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Format: &format}},
		},
	}
	code := GenerateCompanionSelective("Network", meta, reg, true, false)
	assertContains(t, code, "format ipv6")
	assertContains(t, code, ".test(input.ip)")
}

func TestValidateFormatDate(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	format := "date"
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "birthday", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Format: &format}},
		},
	}
	code := GenerateCompanionSelective("Person", meta, reg, true, false)
	assertContains(t, code, "format date")
	assertContains(t, code, ".test(input.birthday)")
}

func TestValidateFormatTime(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	format := "time"
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "scheduledAt", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Format: &format}},
		},
	}
	code := GenerateCompanionSelective("Event", meta, reg, true, false)
	assertContains(t, code, "format time")
	assertContains(t, code, ".test(input.scheduledAt)")
}

func TestValidateFormatDuration(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	format := "duration"
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "ttl", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Format: &format}},
		},
	}
	code := GenerateCompanionSelective("Cache", meta, reg, true, false)
	assertContains(t, code, "format duration")
	assertContains(t, code, ".test(input.ttl)")
}

func TestValidateFormatByte(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	format := "byte"
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "data", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Format: &format}},
		},
	}
	code := GenerateCompanionSelective("Payload", meta, reg, true, false)
	assertContains(t, code, "format byte")
	assertContains(t, code, ".test(input.data)")
}

func TestValidateFormatHostname(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	format := "hostname"
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "host", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Format: &format}},
		},
	}
	code := GenerateCompanionSelective("Endpoint", meta, reg, true, false)
	assertContains(t, code, "format hostname")
	assertContains(t, code, ".test(input.host)")
}

func TestValidateFormatJsonPointer(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	format := "json-pointer"
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "path", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Format: &format}},
		},
	}
	code := GenerateCompanionSelective("JsonRef", meta, reg, true, false)
	assertContains(t, code, "format json-pointer")
	assertContains(t, code, ".test(input.path)")
}

func TestValidateFormatPassword(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	format := "password"
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "secret", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Format: &format}},
		},
	}
	code := GenerateCompanionSelective("Auth", meta, reg, true, false)
	// password format should NOT emit any regex check
	assertNotContains(t, code, "format password")
}

func TestValidateFormatRegex(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	format := "regex"
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "pattern", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Format: &format}},
		},
	}
	code := GenerateCompanionSelective("Filter", meta, reg, true, false)
	// regex format should use try/catch, not regex
	assertContains(t, code, "new RegExp(input.pattern)")
	assertContains(t, code, "format regex")
}

func TestValidateFormatUriTemplate(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	format := "uri-template"
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "template", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Format: &format}},
		},
	}
	code := GenerateCompanionSelective("Link", meta, reg, true, false)
	assertContains(t, code, "format uri-template")
	assertContains(t, code, ".test(input.template)")
}

// --- Phase 9: Zod-Elegant Validation API codegen tests ---

func TestValidateTransformTrim(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Transforms: []string{"trim"}},
			},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	assertContains(t, code, "input.name = input.name.trim()")
	// trim should appear before the string type check wouldn't break
	assertContains(t, code, "typeof input.name === \"string\"")
}

func TestValidateTransformToLowerCase(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name: "email", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Transforms: []string{"toLowerCase"}},
			},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	assertContains(t, code, "input.email = input.email.toLowerCase()")
}

func TestValidateTransformToUpperCase(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name: "code", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Transforms: []string{"toUpperCase"}},
			},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	assertContains(t, code, "input.code = input.code.toUpperCase()")
}

func TestValidateStartsWith(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	sw := "http"
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name: "url", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{StartsWith: &sw},
			},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	assertContains(t, code, `input.url.startsWith("http")`)
	assertContains(t, code, `startsWith http`)
}

func TestValidateEndsWith(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	ew := ".ts"
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name: "file", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{EndsWith: &ew},
			},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	assertContains(t, code, `input.file.endsWith(".ts")`)
}

func TestValidateIncludes(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	inc := "@"
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name: "email", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Includes: &inc},
			},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	assertContains(t, code, `input.email.includes("@")`)
}

func TestValidateUppercase(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	b := true
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name: "code", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Uppercase: &b},
			},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	assertContains(t, code, `input.code !== input.code.toUpperCase()`)
	assertContains(t, code, `"uppercase"`)
}

func TestValidateLowercase(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	b := true
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name: "slug", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Lowercase: &b},
			},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	assertContains(t, code, `input.slug !== input.slug.toLowerCase()`)
	assertContains(t, code, `"lowercase"`)
}

func TestValidateStrictMode(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind:       metadata.KindObject,
		Strictness: "strict",
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "age", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	assertContains(t, code, "Object.keys(input)")
	assertContains(t, code, `"name"`)
	assertContains(t, code, `"age"`)
	assertContains(t, code, `"known property"`)
}

func TestValidateStripMode(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind:       metadata.KindObject,
		Strictness: "strip",
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	assertContains(t, code, "Object.keys(input)")
	assertContains(t, code, "delete input[")
}

func TestValidateCustomErrorMessage(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	sw := "http"
	errMsg := "URL must start with http"
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name: "url", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{StartsWith: &sw, ErrorMessage: &errMsg},
			},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	assertContains(t, code, `URL must start with http`)
}

func TestValidateCustomErrorMessage_AllConstraints(t *testing.T) {
	// Verify @error works on EVERY constraint type, not just startsWith/endsWith/includes/uppercase/lowercase
	tests := []struct {
		name        string
		constraints metadata.Constraints
		propType    metadata.Metadata
		errMsg      string
		contains    string // what should appear in generated code
		notContains string // default message that should NOT appear
	}{
		{
			name:        "minimum",
			constraints: metadata.Constraints{Minimum: ptrFloat(0)},
			propType:    metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"},
			errMsg:      "must be non-negative",
			contains:    "must be non-negative",
			notContains: "minimum 0",
		},
		{
			name:        "maximum",
			constraints: metadata.Constraints{Maximum: ptrFloat(100)},
			propType:    metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"},
			errMsg:      "cannot exceed 100",
			contains:    "cannot exceed 100",
			notContains: "maximum 100",
		},
		{
			name:        "exclusiveMinimum",
			constraints: metadata.Constraints{ExclusiveMinimum: ptrFloat(0)},
			propType:    metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"},
			errMsg:      "must be positive",
			contains:    "must be positive",
			notContains: "exclusiveMinimum 0",
		},
		{
			name:        "exclusiveMaximum",
			constraints: metadata.Constraints{ExclusiveMaximum: ptrFloat(100)},
			propType:    metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"},
			errMsg:      "must be less than 100",
			contains:    "must be less than 100",
			notContains: "exclusiveMaximum 100",
		},
		{
			name:        "multipleOf",
			constraints: metadata.Constraints{MultipleOf: ptrFloat(5)},
			propType:    metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"},
			errMsg:      "must be a multiple of 5",
			contains:    "must be a multiple of 5",
			notContains: "multipleOf 5",
		},
		{
			name:        "numericType int32",
			constraints: metadata.Constraints{NumericType: ptrStr("int32")},
			propType:    metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"},
			errMsg:      "must be a 32-bit integer",
			contains:    "must be a 32-bit integer",
			notContains: `"int32"`,
		},
		{
			name:        "minLength",
			constraints: metadata.Constraints{MinLength: ptrInt(1)},
			propType:    metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
			errMsg:      "cannot be empty",
			contains:    "cannot be empty",
			notContains: "minLength 1",
		},
		{
			name:        "maxLength",
			constraints: metadata.Constraints{MaxLength: ptrInt(50)},
			propType:    metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
			errMsg:      "too long",
			contains:    "too long",
			notContains: "maxLength 50",
		},
		{
			name:        "pattern",
			constraints: metadata.Constraints{Pattern: ptrStr("^[a-z]+$")},
			propType:    metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
			errMsg:      "must be lowercase letters only",
			contains:    "must be lowercase letters only",
			notContains: `pattern ^[a-z]+$`,
		},
		{
			name:        "format email",
			constraints: metadata.Constraints{Format: ptrStr("email")},
			propType:    metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
			errMsg:      "must be a valid email address",
			contains:    "must be a valid email address",
			notContains: "format email",
		},
		{
			name:        "minItems",
			constraints: metadata.Constraints{MinItems: ptrInt(1)},
			propType:    metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}},
			errMsg:      "at least one item required",
			contains:    "at least one item required",
			notContains: "minItems 1",
		},
		{
			name:        "maxItems",
			constraints: metadata.Constraints{MaxItems: ptrInt(10)},
			propType:    metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}},
			errMsg:      "too many items",
			contains:    "too many items",
			notContains: "maxItems 10",
		},
		{
			name:        "uniqueItems",
			constraints: metadata.Constraints{UniqueItems: ptrBool(true)},
			propType:    metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}},
			errMsg:      "no duplicates allowed",
			contains:    "no duplicates allowed",
			notContains: `"uniqueItems"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := metadata.NewTypeRegistry()
			c := tt.constraints
			c.ErrorMessage = &tt.errMsg
			meta := &metadata.Metadata{
				Kind: metadata.KindObject,
				Properties: []metadata.Property{
					{Name: "val", Type: tt.propType, Required: true, Constraints: &c},
				},
			}
			code := GenerateCompanionSelective("Dto", meta, reg, true, false)
			assertContains(t, code, tt.contains)
			assertNotContains(t, code, tt.notContains)
		})
	}
}

func TestValidateDefaultRuntime(t *testing.T) {
	tests := []struct {
		name     string
		propType metadata.Metadata
		defVal   string
		contains string
	}{
		{
			name:     "string default",
			propType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
			defVal:   "hello",
			contains: `= "hello"`,
		},
		{
			name:     "string default with quotes",
			propType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
			defVal:   `"world"`,
			contains: `= "world"`,
		},
		{
			name:     "number default",
			propType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"},
			defVal:   "42",
			contains: `= 42`,
		},
		{
			name:     "boolean default true",
			propType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "boolean"},
			defVal:   "true",
			contains: `= true`,
		},
		{
			name:     "boolean default false",
			propType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "boolean"},
			defVal:   "false",
			contains: `= false`,
		},
		{
			name:     "null default",
			propType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Nullable: true},
			defVal:   "null",
			contains: `= null`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := metadata.NewTypeRegistry()
			propType := tt.propType
			propType.Optional = true // defaults only make sense on optional props
			meta := &metadata.Metadata{
				Kind: metadata.KindObject,
				Properties: []metadata.Property{
					{
						Name: "val", Type: propType, Required: false,
						Constraints: &metadata.Constraints{Default: &tt.defVal},
					},
				},
			}
			code := GenerateCompanionSelective("Dto", meta, reg, true, false)
			// Should contain the default assignment
			assertContains(t, code, tt.contains)
			// Should be inside an `if (x.val === undefined)` block
			assertContains(t, code, "input.val === undefined")
		})
	}
}

func TestValidateTemplateLiteral(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name: "key", Required: true,
				Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", TemplatePattern: "^prefix_.*$"},
			},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	// Should contain a regex test for the template literal pattern
	assertContains(t, code, `/^prefix_.*$/.test(`)
	assertContains(t, code, `"pattern ^prefix_.*$"`)
}

func TestValidateTemplateLiteral_MultiInterpolation(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name: "email", Required: true,
				Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", TemplatePattern: `^.*@.*\..*$`},
			},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	assertContains(t, code, `.test(`)
	assertContains(t, code, `pattern`)
}

func TestValidateFormatNanoid(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	f := "nanoid"
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Format: &f}},
		},
	}
	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	assertContains(t, code, "format nanoid")
}

func TestValidateFormatJwt(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	f := "jwt"
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "token", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Format: &f}},
		},
	}
	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	assertContains(t, code, "format jwt")
}

func TestValidateFormatUlid(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	f := "ulid"
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Format: &f}},
		},
	}
	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	assertContains(t, code, "format ulid")
}

func TestValidateFormatMac(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	f := "mac"
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "addr", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Format: &f}},
		},
	}
	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	assertContains(t, code, "format mac")
}

func TestValidateFormatCidrv4(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	f := "cidrv4"
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "subnet", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Format: &f}},
		},
	}
	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	assertContains(t, code, "format cidrv4")
}

func TestCompanionIgnoreAll(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	types := map[string]*metadata.Metadata{
		"IgnoredDto": {Kind: metadata.KindObject, Ignore: "all", Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		}},
		"VisibleDto": {Kind: metadata.KindObject, Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		}},
	}

	files := GenerateCompanionFiles("test.ts", types, reg)
	for _, f := range files {
		if strings.Contains(f.Path, "IgnoredDto") {
			t.Errorf("expected IgnoredDto to be excluded, but found file: %s", f.Path)
		}
	}
	// VisibleDto should have 2 files (tsgonest.js + tsgonest.d.ts)
	visibleCount := 0
	for _, f := range files {
		if strings.Contains(f.Path, "VisibleDto") {
			visibleCount++
		}
	}
	if visibleCount != 2 {
		t.Errorf("expected 2 files for VisibleDto, got %d", visibleCount)
	}
}

func TestCompanionIgnoreValidation(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	types := map[string]*metadata.Metadata{
		"Dto": {Kind: metadata.KindObject, Ignore: "validation", Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		}},
	}

	files := GenerateCompanionFiles("test.ts", types, reg)
	// With consolidated companion files, the .tsgonest.js file is still generated
	// (it contains serialization even when validation is ignored)
	tsgonestCount := 0
	for _, f := range files {
		if strings.HasSuffix(f.Path, ".tsgonest.js") {
			tsgonestCount++
		}
	}
	if tsgonestCount != 1 {
		t.Errorf("expected 1 .tsgonest.js file, got %d", tsgonestCount)
	}
}

// --- Phase 11: Standard Schema v1 codegen tests ---

func TestValidation_StandardSchemaWrapper(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "TestDto",
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}

	code := GenerateCompanionSelective("TestDto", meta, reg, true, false)

	// Should contain Standard Schema wrapper export
	assertContains(t, code, "export const schemaTestDto")
	// Should contain ~standard property
	assertContains(t, code, "\"~standard\"")
	// Should contain version 1
	assertContains(t, code, "version: 1,")
	// Should contain vendor
	assertContains(t, code, "vendor: \"tsgonest\",")
	// Should contain validate method
	assertContains(t, code, "validate(value)")
	// Should reference the validateTestDto function
	assertContains(t, code, "validateTestDto(value)")
	// Should return value on success
	assertContains(t, code, "return { value: result.data };")
	// Should map errors to issues on failure
	assertContains(t, code, "issues: result.errors.map")
}

func TestValidation_StandardSchemaWrapperSyntax(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "Foo",
		Properties: []metadata.Property{
			{Name: "x", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
		},
	}

	code := GenerateCompanionSelective("Foo", meta, reg, true, false)

	// The wrapper should end with }; (closing the const object)
	assertContains(t, code, "};")
	// Should contain the schema comment
	assertContains(t, code, "// Standard Schema v1 wrapper")
	// Should contain path mapping logic
	assertContains(t, code, "e.path.split(\".\").map(k => ({ key: k }))")
}

func TestGenerateCompanionTypes(t *testing.T) {
	output := GenerateCompanionTypes("UserDto")

	assertContains(t, output, "export declare function validateUserDto")
	assertContains(t, output, "export declare function assertUserDto")
	assertContains(t, output, "export declare function serializeUserDto")
	assertNotContains(t, output, "deserializeUserDto")
	assertContains(t, output, "export declare const schemaUserDto")
	assertContains(t, output, "StandardSchemaV1Props")
	assertContains(t, output, "readonly version: 1;")
	assertContains(t, output, "readonly vendor: string;")
	assertContains(t, output, "(value: unknown)")
}

func TestGenerateCompanionTypes_ReturnTypes(t *testing.T) {
	output := GenerateCompanionTypes("Foo")

	// validate returns a discriminated union result
	assertContains(t, output, "success: true; data: Foo")
	assertContains(t, output, "success: false; errors: Array<")
	// assert returns the type directly
	assertContains(t, output, "assertFoo(input: unknown): Foo")
	// serialize takes the type and returns string
	assertContains(t, output, "serializeFoo(input: Foo): string")
	// no deserialize
	assertNotContains(t, output, "deserializeFoo")
	// schema has ~standard
	assertContains(t, output, "\"~standard\": StandardSchemaV1Props")
}

func TestCompanion_GeneratesDTS(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	types := map[string]*metadata.Metadata{
		"Foo": {
			Kind: metadata.KindObject, Name: "Foo",
			Properties: []metadata.Property{
				{Name: "x", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			},
		},
	}

	files := GenerateCompanionFiles("src/foo.ts", types, reg)

	// Should have .tsgonest.js and .tsgonest.d.ts
	hasTsgonestJS := false
	hasTsgonestDTS := false
	for _, f := range files {
		if strings.HasSuffix(f.Path, ".tsgonest.js") {
			hasTsgonestJS = true
		}
		if strings.HasSuffix(f.Path, ".tsgonest.d.ts") {
			hasTsgonestDTS = true
			// Verify the content includes declarations
			assertContains(t, f.Content, "export declare function validateFoo")
			assertContains(t, f.Content, "export declare const schemaFoo")
		}
	}
	if !hasTsgonestJS {
		t.Error("expected .tsgonest.js companion file")
	}
	if !hasTsgonestDTS {
		t.Error("expected .tsgonest.d.ts companion file")
	}
}

func TestCompanion_DTSPath(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	types := map[string]*metadata.Metadata{
		"Bar": {
			Kind: metadata.KindObject, Name: "Bar",
			Properties: []metadata.Property{
				{Name: "y", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			},
		},
	}

	files := GenerateCompanionFiles("src/bar.dto.ts", types, reg)

	for _, f := range files {
		if strings.HasSuffix(f.Path, ".tsgonest.d.ts") {
			expected := "src/bar.dto.Bar.tsgonest.d.ts"
			if f.Path != expected {
				t.Errorf("expected DTS path %q, got %q", expected, f.Path)
			}
			return
		}
	}
	t.Error("no .tsgonest.d.ts file found")
}

func TestCompanion_DTSWhenValidationIgnored(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	types := map[string]*metadata.Metadata{
		"Dto": {Kind: metadata.KindObject, Ignore: "validation", Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		}},
	}

	files := GenerateCompanionFiles("test.ts", types, reg)

	// With consolidated companions, .d.ts is always generated (it covers serialize too)
	var hasDTS bool
	for _, f := range files {
		if strings.HasSuffix(f.Path, ".d.ts") {
			hasDTS = true
			// When validation is ignored, the .d.ts should have serialize but NOT validate
			assertContains(t, f.Content, "serialize")
			assertNotContains(t, f.Content, "validate")
			assertNotContains(t, f.Content, "assert")
		}
	}
	if !hasDTS {
		t.Error("expected .d.ts file to be generated even when validation is ignored")
	}
}

// --- Phase 4: Index Signature Validation Tests ---

func TestValidateIndexSignature_StringValues(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
		},
		IndexSignature: &metadata.IndexSignature{
			KeyType:   metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
			ValueType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	// Should iterate over Object.keys
	assertContains(t, code, "Object.keys(")
	// Should exclude the declared property "id"
	assertContains(t, code, `"id"`)
	// Should check typeof === "string" for index sig values
	assertContains(t, code, `typeof`)
}

func TestValidateIndexSignature_NumberValues(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
		IndexSignature: &metadata.IndexSignature{
			KeyType:   metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
			ValueType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	assertContains(t, code, "Object.keys(")
	// Should validate values as numbers
	assertContains(t, code, `"number"`)
}

func TestValidateIndexSignature_NoProperties(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		IndexSignature: &metadata.IndexSignature{
			KeyType:   metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
			ValueType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	// Should iterate over Object.keys without known key exclusion set
	assertContains(t, code, "Object.keys(")
	// Should NOT contain a Set for known keys (no properties to exclude)
	assertNotContains(t, code, "new Set(")
}

func TestValidateIndexSignature_ObjectValues(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		IndexSignature: &metadata.IndexSignature{
			KeyType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
			ValueType: metadata.Metadata{
				Kind: metadata.KindObject,
				Properties: []metadata.Property{
					{Name: "value", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
				},
			},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	// Should validate nested object properties
	assertContains(t, code, "Object.keys(")
	assertContains(t, code, ".value")
}

func TestValidateIndexSignature_WithDeclaredPropsExcluded(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "age", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
		},
		IndexSignature: &metadata.IndexSignature{
			KeyType:   metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
			ValueType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "boolean"},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	// Should have a Set with both declared property names
	assertContains(t, code, `"name"`)
	assertContains(t, code, `"age"`)
	assertContains(t, code, "new Set(")
	// Should validate remaining keys as booleans
	assertContains(t, code, `"boolean"`)
}

// --- Phase 5: Discriminated Union Optimization Tests ---

func TestValidateDiscriminatedUnion_SwitchCodegen(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name:     "payment",
				Required: true,
				Type: metadata.Metadata{
					Kind: metadata.KindUnion,
					Discriminant: &metadata.Discriminant{
						Property: "type",
						Mapping:  map[string]int{"card": 0, "bank": 1, "crypto": 2},
					},
					UnionMembers: []metadata.Metadata{
						{
							Kind: metadata.KindObject,
							Properties: []metadata.Property{
								{Name: "type", Required: true, Type: metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: "card"}},
								{Name: "cardNumber", Required: true, Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}},
							},
						},
						{
							Kind: metadata.KindObject,
							Properties: []metadata.Property{
								{Name: "type", Required: true, Type: metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: "bank"}},
								{Name: "routingNumber", Required: true, Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}},
							},
						},
						{
							Kind: metadata.KindObject,
							Properties: []metadata.Property{
								{Name: "type", Required: true, Type: metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: "crypto"}},
								{Name: "walletAddress", Required: true, Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}},
							},
						},
					},
				},
			},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	// Should use switch instead of try-each
	assertContains(t, code, "switch (")
	assertContains(t, code, `["type"]`)
	// Should have cases for each discriminant value
	assertContains(t, code, `case "bank"`)
	assertContains(t, code, `case "card"`)
	assertContains(t, code, `case "crypto"`)
	// Should have a default case
	assertContains(t, code, "default:")
	// Should NOT use the try-each union pattern
	assertNotContains(t, code, "_uv") // union valid var from try-each
}

func TestValidateNonDiscriminatedUnion_StillUseTryEach(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name:     "value",
				Required: true,
				Type: metadata.Metadata{
					Kind: metadata.KindUnion,
					// No Discriminant — should use try-each
					UnionMembers: []metadata.Metadata{
						{Kind: metadata.KindAtomic, Atomic: "string"},
						{Kind: metadata.KindAtomic, Atomic: "number"},
					},
				},
			},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	// Should NOT use switch
	assertNotContains(t, code, "switch (")
	// Should use the try-each union pattern
	assertContains(t, code, "_uv")
}

func TestValidateDiscriminatedUnion_DefaultCaseError(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name:     "shape",
				Required: true,
				Type: metadata.Metadata{
					Kind: metadata.KindUnion,
					Discriminant: &metadata.Discriminant{
						Property: "kind",
						Mapping:  map[string]int{"circle": 0, "square": 1},
					},
					UnionMembers: []metadata.Metadata{
						{
							Kind: metadata.KindObject,
							Properties: []metadata.Property{
								{Name: "kind", Required: true, Type: metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: "circle"}},
								{Name: "radius", Required: true, Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}},
							},
						},
						{
							Kind: metadata.KindObject,
							Properties: []metadata.Property{
								{Name: "kind", Required: true, Type: metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: "square"}},
								{Name: "side", Required: true, Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}},
							},
						},
					},
				},
			},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	// Default case should include error about expected discriminant values
	assertContains(t, code, "default:")
	assertContains(t, code, `"circle"`)
	assertContains(t, code, `"square"`)
}

// --- Per-Constraint Error Tests ---

func TestValidatePerConstraintError_Format(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	f := "email"
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name: "email", Required: true,
				Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
				Constraints: &metadata.Constraints{
					Format: &f,
					Errors: map[string]string{"format": "Must be a valid email address"},
				},
			},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	// Per-constraint error should appear instead of default
	assertContains(t, code, "Must be a valid email address")
	assertNotContains(t, code, "format email")
}

func TestValidatePerConstraintError_MultipleConstraints(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	f := "email"
	minLen := 5
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name: "email", Required: true,
				Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
				Constraints: &metadata.Constraints{
					Format:    &f,
					MinLength: &minLen,
					Errors: map[string]string{
						"format":    "Must be a valid email",
						"minLength": "Email is too short",
					},
				},
			},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	// Both per-constraint errors should appear
	assertContains(t, code, "Must be a valid email")
	assertContains(t, code, "Email is too short")
	// Default messages should NOT appear
	assertNotContains(t, code, "format email")
	assertNotContains(t, code, "minLength 5")
}

func TestValidatePerConstraintError_FallsBackToGlobal(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	f := "email"
	minLen := 5
	globalErr := "Invalid input"
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name: "email", Required: true,
				Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
				Constraints: &metadata.Constraints{
					Format:       &f,
					MinLength:    &minLen,
					ErrorMessage: &globalErr,
					// Only format has per-constraint error; minLength falls back to global
					Errors: map[string]string{
						"format": "Must be a valid email",
					},
				},
			},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	// format check uses per-constraint error
	assertContains(t, code, "Must be a valid email")
	// minLength falls back to global ErrorMessage
	assertContains(t, code, "Invalid input")
}

func TestValidatePerConstraintError_Minimum(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	min := 0.0
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name: "age", Required: true,
				Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"},
				Constraints: &metadata.Constraints{
					Minimum: &min,
					Errors:  map[string]string{"minimum": "Age must be non-negative"},
				},
			},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	assertContains(t, code, "Age must be non-negative")
	assertNotContains(t, code, "minimum 0")
}

// --- Phase 8: Coercion Tests ---

func TestValidateCoercion_StringToNumber(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	coerce := true
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name: "page", Required: true,
				Type:        metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"},
				Constraints: &metadata.Constraints{Coerce: &coerce},
			},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	// Should emit string→number coercion
	assertContains(t, code, "typeof input.page === \"string\"")
	assertContains(t, code, "+input.page")
	assertContains(t, code, "Number.isNaN")
}

func TestValidateCoercion_StringToBoolean(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	coerce := true
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name: "active", Required: true,
				Type:        metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "boolean"},
				Constraints: &metadata.Constraints{Coerce: &coerce},
			},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	// Should emit string→boolean coercion
	assertContains(t, code, `=== "true"`)
	assertContains(t, code, `=== "false"`)
	assertContains(t, code, "= true")
	assertContains(t, code, "= false")
}

func TestValidateCoercion_NotAppliedToString(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	coerce := true
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name: "name", Required: true,
				Type:        metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
				Constraints: &metadata.Constraints{Coerce: &coerce},
			},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)
	// String type has no coercion — it's already a string
	assertNotContains(t, code, "Number.isNaN")
	assertNotContains(t, code, `=== "true"`)
}

// --- Validate<typeof fn> codegen tests ---

func TestValidateCustomFn_EmitsFunctionCall(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name:     "card",
				Type:     metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
				Required: true,
				Constraints: &metadata.Constraints{
					ValidateFn:     ptrStr("isValidCard"),
					ValidateModule: ptrStr("./validators/credit-card"),
				},
			},
		},
	}

	code := GenerateCompanionSelective("Payment", meta, reg, true, false)
	// Should emit import statement at the top
	assertContains(t, code, `import { isValidCard } from "./validators/credit-card";`)
	// Should emit function call in validation
	assertContains(t, code, `if (!isValidCard(input.card))`)
	// Should push error on validation failure
	assertContains(t, code, `validate(isValidCard)`)
}

func TestValidateCustomFn_WithCustomError(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name:     "email",
				Type:     metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
				Required: true,
				Constraints: &metadata.Constraints{
					ValidateFn:     ptrStr("isValidEmail"),
					ValidateModule: ptrStr("./validators/email"),
					Errors:         map[string]string{"validate": "Must be a valid email address"},
				},
			},
		},
	}

	code := GenerateCompanionSelective("Contact", meta, reg, true, false)
	assertContains(t, code, `import { isValidEmail } from "./validators/email";`)
	assertContains(t, code, `if (!isValidEmail(input.email))`)
	// Should use per-constraint error, not default
	assertContains(t, code, `Must be a valid email address`)
}

func TestValidateCustomFn_MultipleValidators(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name:     "card",
				Type:     metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
				Required: true,
				Constraints: &metadata.Constraints{
					ValidateFn:     ptrStr("isValidCard"),
					ValidateModule: ptrStr("./validators/credit-card"),
				},
			},
			{
				Name:     "email",
				Type:     metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
				Required: true,
				Constraints: &metadata.Constraints{
					ValidateFn:     ptrStr("isValidEmail"),
					ValidateModule: ptrStr("./validators/email"),
				},
			},
		},
	}

	code := GenerateCompanionSelective("Order", meta, reg, true, false)
	// Both imports should be emitted
	assertContains(t, code, `import { isValidCard } from "./validators/credit-card";`)
	assertContains(t, code, `import { isValidEmail } from "./validators/email";`)
	// Both function calls should be emitted
	assertContains(t, code, `if (!isValidCard(input.card))`)
	assertContains(t, code, `if (!isValidEmail(input.email))`)
}

func TestValidateCustomFn_NoImportWithoutModule(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name:     "value",
				Type:     metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
				Required: true,
				Constraints: &metadata.Constraints{
					ValidateFn: ptrStr("customCheck"),
					// No ValidateModule — shouldn't generate import
				},
			},
		},
	}

	code := GenerateCompanionSelective("TestDto", meta, reg, true, false)
	// Should still emit function call
	assertContains(t, code, `if (!customCheck(input.value))`)
	// Should NOT emit import (no module)
	assertNotContains(t, code, "import {")
}

func TestValidateCustomFn_StripsTsExtension(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name:     "data",
				Type:     metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
				Required: true,
				Constraints: &metadata.Constraints{
					ValidateFn:     ptrStr("validateData"),
					ValidateModule: ptrStr("./utils/validators.ts"),
				},
			},
		},
	}

	code := GenerateCompanionSelective("InputDto", meta, reg, true, false)
	// Should strip .ts extension from import path
	assertContains(t, code, `import { validateData } from "./utils/validators";`)
	assertNotContains(t, code, `validators.ts`)
}

// --- Helper ---

// ── Union serialization tests ──────────────────────────────────────────

func TestSerializeDiscriminatedUnion(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	cardMeta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "type", Type: metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: "card"}, Required: true},
			{Name: "last4", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}
	bankMeta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "type", Type: metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: "bank"}, Required: true},
			{Name: "routing", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}
	reg.Register("CardPayment", cardMeta)
	reg.Register("BankPayment", bankMeta)

	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "payment", Type: metadata.Metadata{
				Kind: metadata.KindUnion,
				UnionMembers: []metadata.Metadata{
					{Kind: metadata.KindRef, Ref: "CardPayment"},
					{Kind: metadata.KindRef, Ref: "BankPayment"},
				},
				Discriminant: &metadata.Discriminant{
					Property: "type",
					Mapping:  map[string]int{"card": 0, "bank": 1},
				},
			}, Required: true},
		},
	}

	code := GenerateCompanionSelective("Order", meta, reg, false, true)

	// Should use switch on discriminant for each branch
	assertContains(t, code, `switch (`)
	assertContains(t, code, `["type"]`)
	assertContains(t, code, `case "card"`)
	assertContains(t, code, `case "bank"`)
	// Each case should serialize properly (not just JSON.stringify the whole union)
	assertContains(t, code, `__s(input.payment.last4)`)
	assertContains(t, code, `__s(input.payment.routing)`)
}

func TestSerializeLiteralUnion(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "role", Type: metadata.Metadata{
				Kind: metadata.KindUnion,
				UnionMembers: []metadata.Metadata{
					{Kind: metadata.KindLiteral, LiteralValue: "admin"},
					{Kind: metadata.KindLiteral, LiteralValue: "user"},
					{Kind: metadata.KindLiteral, LiteralValue: "guest"},
				},
			}, Required: true},
		},
	}

	code := GenerateCompanionSelective("User", meta, reg, false, true)

	// All-string literal union should use __s() directly
	assertContains(t, code, "__s(input.role)")
	assertNotContains(t, code, "JSON.stringify(input.role)")
}

func TestSerializeNullableUnion(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	addrMeta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "city", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}
	reg.Register("Address", addrMeta)

	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "address", Type: metadata.Metadata{
				Kind: metadata.KindUnion,
				UnionMembers: []metadata.Metadata{
					{Kind: metadata.KindRef, Ref: "Address"},
					{Kind: metadata.KindLiteral, LiteralValue: nil},
				},
			}, Required: true},
		},
	}

	code := GenerateCompanionSelective("User", meta, reg, false, true)

	// Should use null check, not JSON.stringify fallback
	assertContains(t, code, `== null ? "null"`)
	assertContains(t, code, `__s(input.address.city)`)
	assertNotContains(t, code, "JSON.stringify(input.address)")
}

func TestSerializeAtomicUnion(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "value", Type: metadata.Metadata{
				Kind: metadata.KindUnion,
				UnionMembers: []metadata.Metadata{
					{Kind: metadata.KindAtomic, Atomic: "string"},
					{Kind: metadata.KindAtomic, Atomic: "number"},
				},
			}, Required: true},
		},
	}

	code := GenerateCompanionSelective("Flexible", meta, reg, false, true)

	// Should use typeof dispatch, not JSON.stringify
	assertContains(t, code, `typeof input.value === "string"`)
	assertContains(t, code, `__s(input.value)`)
	assertNotContains(t, code, "JSON.stringify(input.value)")
}

// --- is() function codegen tests ---

func TestGenerateIsFunction_Simple(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "UserDto",
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "age", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "active", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "boolean"}, Required: true},
		},
	}

	code := GenerateCompanionSelective("UserDto", meta, reg, true, false)

	// Should have is() function with && chain
	assertContains(t, code, "export function isUserDto(input)")
	assertContains(t, code, `typeof input === "object" && input !== null`)
	assertContains(t, code, `typeof input.name === "string"`)
	assertContains(t, code, `typeof input.age === "number" && Number.isFinite(input.age)`)
	assertContains(t, code, `typeof input.active === "boolean"`)
	// The is() function should use return with a single expression
	assertContains(t, code, "return (typeof input")
}

func TestGenerateIsFunction_Nullable(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Nullable: true}, Required: true},
			{Name: "bio", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Optional: true}, Required: false},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)

	assertContains(t, code, "export function isDto(input)")
	// Nullable should have null check
	assertContains(t, code, "input.name === null || typeof input.name === \"string\"")
	// Optional should have undefined check
	assertContains(t, code, "input.bio === undefined || typeof input.bio === \"string\"")
}

func TestGenerateIsFunction_Constraints(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	minLen := 1
	maxLen := 255
	min := 0.0
	max := 150.0
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name: "name", Required: true,
				Type:        metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
				Constraints: &metadata.Constraints{MinLength: &minLen, MaxLength: &maxLen},
			},
			{
				Name: "age", Required: true,
				Type:        metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"},
				Constraints: &metadata.Constraints{Minimum: &min, Maximum: &max},
			},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)

	assertContains(t, code, "export function isDto(input)")
	// Constraints should be inlined in the is function
	assertContains(t, code, "input.name.length >= 1")
	assertContains(t, code, "input.name.length <= 255")
	assertContains(t, code, "input.age >= 0")
	assertContains(t, code, "input.age <= 150")
}

func TestGenerateIsFunction_Union(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name:     "role",
				Required: true,
				Type: metadata.Metadata{
					Kind: metadata.KindUnion,
					UnionMembers: []metadata.Metadata{
						{Kind: metadata.KindLiteral, LiteralValue: "admin"},
						{Kind: metadata.KindLiteral, LiteralValue: "moderator"},
						{Kind: metadata.KindLiteral, LiteralValue: "user"},
					},
				},
			},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)

	assertContains(t, code, "export function isDto(input)")
	// Literal unions should use direct === checks (no .includes())
	assertContains(t, code, `input.role === "admin"`)
	assertContains(t, code, `input.role === "moderator"`)
	assertContains(t, code, `input.role === "user"`)
}

func TestGenerateIsFunction_Recursive(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "children", Type: metadata.Metadata{
				Kind: metadata.KindArray,
				ElementType: &metadata.Metadata{Kind: metadata.KindRef, Ref: "Department"},
			}, Required: true},
		},
	}
	reg.Types["Department"] = meta

	code := GenerateCompanionSelective("Department", meta, reg, true, false)

	assertContains(t, code, "export function isDepartment(input)")
	// Recursive reference should call isDepartment
	assertContains(t, code, "isDepartment(")
	assertContains(t, code, ".every(")
}

// --- Standalone assert() tests ---

func TestGenerateStandaloneAssert(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "age", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)

	assertContains(t, code, "export function assertDto(input)")
	// Should throw TypeError directly, not wrap validate
	assertContains(t, code, "throw new TypeError")
	// The old assert wrapped validate — the new one should NOT have "result = validateDto"
	assertNotContains(t, code, "const result = validateDto(input)")
	// The assert function should contain its own checks and return input
	assertContains(t, code, "return input;")
}

func TestGenerateStandaloneAssert_Recursive(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "children", Type: metadata.Metadata{
				Kind: metadata.KindArray,
				ElementType: &metadata.Metadata{Kind: metadata.KindRef, Ref: "Dept"},
			}, Required: true},
		},
	}
	reg.Types["Dept"] = meta

	code := GenerateCompanionSelective("Dept", meta, reg, true, false)

	// Should have inner function with path parameter
	assertContains(t, code, "_assertDept(input, _path)")
	assertContains(t, code, "export function assertDept(input)")
	assertContains(t, code, `_assertDept(input, "input")`)
}

// --- Recursive validate optimization tests ---

func TestGenerateValidate_RecursiveOptimized(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "children", Type: metadata.Metadata{
				Kind: metadata.KindArray,
				ElementType: &metadata.Metadata{Kind: metadata.KindRef, Ref: "Dept"},
			}, Required: true},
		},
	}
	reg.Types["Dept"] = meta

	code := GenerateCompanionSelective("Dept", meta, reg, true, false)

	// Should use inner function pattern (no regex path rewrite, no spread)
	assertContains(t, code, "_validateDept(input, _path, errors)")
	assertContains(t, code, `_validateDept(input, "input", errors)`)
	// Should NOT use the old expensive pattern
	assertNotContains(t, code, ".replace(/^input/")
	assertNotContains(t, code, "...r.errors")
}

// --- stringify() function tests ---

func TestGenerateStringify(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, true)

	assertContains(t, code, "export function stringifyDto(input)")
	assertContains(t, code, "validateDto(input)")
	assertContains(t, code, "serializeDto(input)")
	assertContains(t, code, "Serialization type check failed for Dto")
}

func TestGenerateStringify_NotGeneratedWithoutSerialization(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, true, false)

	assertNotContains(t, code, "export function stringifyDto(input)")
}

// --- Enum serialization tests ---

func TestEnumSerialization_AllStrings(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "status", Type: metadata.Metadata{
				Kind: metadata.KindEnum,
				EnumValues: []metadata.EnumValue{
					{Name: "Active", Value: "active"},
					{Name: "Inactive", Value: "inactive"},
				},
			}, Required: true},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, false, true)

	// String enum should use __s(), not JSON.stringify
	assertContains(t, code, "__s(input.status)")
	assertNotContains(t, code, "JSON.stringify(input.status)")
}

func TestEnumSerialization_AllNumbers(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "level", Type: metadata.Metadata{
				Kind: metadata.KindEnum,
				EnumValues: []metadata.EnumValue{
					{Name: "Low", Value: float64(1)},
					{Name: "High", Value: float64(2)},
				},
			}, Required: true},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, false, true)

	// Numeric enum should use coercion, not JSON.stringify
	assertContains(t, code, `"" + input.level`)
	assertNotContains(t, code, "JSON.stringify(input.level)")
}

// --- Serialization __s() simplification tests ---

func TestSerializationHelper_NoRegexBranch(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}

	code := GenerateCompanionSelective("Dto", meta, reg, false, true)

	// Should have simplified __s() without the regex branch
	assertContains(t, code, "function __s(s)")
	assertNotContains(t, code, "s.length > 64")
	assertNotContains(t, code, "__esc")
}

// --- Companion types (.d.ts) tests ---

func TestGenerateCompanionTypes_NewFunctions(t *testing.T) {
	code := GenerateCompanionTypesSelective("UserDto", true, true)

	assertContains(t, code, "export declare function isUserDto(input: unknown): input is UserDto;")
	assertContains(t, code, "export declare function stringifyUserDto(input: UserDto): string;")
}

func TestGenerateCompanionTypes_NoStringifyWithoutSerialization(t *testing.T) {
	code := GenerateCompanionTypesSelective("UserDto", true, false)

	assertContains(t, code, "export declare function isUserDto(input: unknown): input is UserDto;")
	assertNotContains(t, code, "stringifyUserDto")
}

// --- Transform codegen tests ---

func TestTransformSimpleObject(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "UserResponse",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "email", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}

	code := GenerateCompanionSelective("UserResponse", meta, reg, false, true)

	assertContains(t, code, "export function transformUserResponse(input)")
	assertContains(t, code, "id: input.id")
	assertContains(t, code, "name: input.name")
	assertContains(t, code, "email: input.email")
}

func TestTransformOptionalProps(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "Profile",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "bio", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Optional: true}, Required: false},
		},
	}

	code := GenerateCompanionSelective("Profile", meta, reg, false, true)

	assertContains(t, code, "export function transformProfile(input)")
	assertContains(t, code, "id: input.id")
	assertContains(t, code, "if (input.bio !== undefined)")
	assertContains(t, code, "_r.bio =")
}

func TestTransformNestedObject(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	addressMeta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "Address",
		Properties: []metadata.Property{
			{Name: "street", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "city", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}
	reg.Register("Address", addressMeta)

	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "User",
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "address", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "Address"}, Required: true},
		},
	}

	code := GenerateCompanionSelective("User", meta, reg, false, true)

	assertContains(t, code, "export function transformUser(input)")
	assertContains(t, code, "name: input.name")
	// Nested object should be inlined since Address is not recursive
	assertContains(t, code, "street: input.address.street")
	assertContains(t, code, "city: input.address.city")
}

func TestTransformArray(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "Response",
		Properties: []metadata.Property{
			{Name: "items", Type: metadata.Metadata{
				Kind: metadata.KindArray,
				ElementType: &metadata.Metadata{
					Kind: metadata.KindObject,
					Name: "Item",
					Properties: []metadata.Property{
						{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
						{Name: "label", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
					},
				},
			}, Required: true},
		},
	}

	code := GenerateCompanionSelective("Response", meta, reg, false, true)

	assertContains(t, code, "export function transformResponse(input)")
	assertContains(t, code, ".map(")
	assertContains(t, code, "id:")
	assertContains(t, code, "label:")
}

func TestTransformNullable(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "Box",
		Properties: []metadata.Property{
			{Name: "value", Type: metadata.Metadata{
				Kind: metadata.KindAtomic, Atomic: "string", Nullable: true,
			}, Required: true},
		},
	}

	code := GenerateCompanionSelective("Box", meta, reg, false, true)

	assertContains(t, code, "export function transformBox(input)")
	assertContains(t, code, "== null ? null :")
}

func TestTransformRecursiveType(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "TreeNode",
		Properties: []metadata.Property{
			{Name: "value", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "child", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "TreeNode", Nullable: true}, Required: true},
		},
	}
	reg.Register("TreeNode", meta)

	code := GenerateCompanionSelective("TreeNode", meta, reg, false, true)

	assertContains(t, code, "export function transformTreeNode(input)")
	assertContains(t, code, "transformTreeNode(input.child)")
}

func TestTransformAtomicPassthrough(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	e := NewEmitter()
	ctx := &transformCtx{generating: map[string]bool{}}
	expr := generateTransformExpr("input.value", &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, reg, 0, ctx)
	_ = e
	if expr != "input.value" {
		t.Errorf("expected atomic passthrough, got: %s", expr)
	}

	expr = generateTransformExpr("input.count", &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, reg, 0, ctx)
	if expr != "input.count" {
		t.Errorf("expected atomic passthrough, got: %s", expr)
	}
}

func TestTransformTypeDeclaration(t *testing.T) {
	code := GenerateCompanionTypesSelective("UserDto", true, true)
	assertContains(t, code, "export declare function transformUserDto(input: UserDto): UserDto;")
}

func TestTransformTypeDeclaration_NoTransformWithoutSerialization(t *testing.T) {
	code := GenerateCompanionTypesSelective("UserDto", true, false)
	assertNotContains(t, code, "transformUserDto")
}

func assertContains(t *testing.T, haystack string, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected output to contain %q.\nGot:\n%s", needle, haystack)
	}
}

func assertNotContains(t *testing.T, haystack string, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Errorf("expected output NOT to contain %q.\nGot:\n%s", needle, haystack)
	}
}
