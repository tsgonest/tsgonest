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

	code := GenerateValidation("CreateUserDto", meta, reg)

	assertContains(t, code, "export function validateCreateUserDto(input)")
	assertContains(t, code, "typeof input !== \"object\"")
	assertContains(t, code, "typeof input.name !== \"string\"")
	assertContains(t, code, "typeof input.age !== \"number\"")
	assertContains(t, code, "export function assertCreateUserDto(input)")
	assertContains(t, code, "export function deserializeCreateUserDto(json)")
	assertContains(t, code, "JSON.parse(json)")
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

	code := GenerateValidation("Profile", meta, reg)

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

	code := GenerateValidation("Box", meta, reg)
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

	code := GenerateValidation("Post", meta, reg)
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

	code := GenerateValidation("Account", meta, reg)
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

	code := GenerateValidation("UserRole", meta, reg)

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

	code := GenerateValidation("User", meta, reg)
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

	code := GenerateValidation("Event", meta, reg)
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

	code := GenerateValidation("Shape", meta, reg)
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

	code := GenerateSerialization("UserResponse", meta, reg)
	assertContains(t, code, "export function serializeUserResponse(input)")
	assertContains(t, code, "__jsonStr(input.name)")
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

	code := GenerateSerialization("Event", meta, reg)
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

	code := GenerateSerialization("Result", meta, reg)
	assertContains(t, code, ".map(")
	assertContains(t, code, ".join(\",\")")
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

	code := GenerateSerialization("UpdateUserDto", meta, reg)

	// Should NOT fall back to JSON.stringify for the whole object
	assertNotContains(t, code, "return JSON.stringify(input)")
	// Should have conditional key inclusion
	assertContains(t, code, "input.name !== undefined")
	assertContains(t, code, "input.age !== undefined")
	// Should produce push-based approach
	assertContains(t, code, ".push(")
	assertContains(t, code, ".join(\",\")")
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

	code := GenerateSerialization("Profile", meta, reg)

	// Required props should always be pushed (without conditional)
	assertContains(t, code, `_p0.push("\"id\":`)
	assertContains(t, code, `_p0.push("\"name\":`)
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

	code := GenerateSerialization("User", meta, reg)
	assertContains(t, code, "__jsonStr(input.address.city)")
}

func TestCompanionPath(t *testing.T) {
	tests := []struct {
		source   string
		typeName string
		suffix   string
		expected string
	}{
		{"src/user.dto.ts", "CreateUserDto", "validate", "src/user.dto.CreateUserDto.validate.js"},
		{"src/user.dto.tsx", "User", "serialize", "src/user.dto.User.serialize.js"},
		{"src/app.ts", "Config", "validate", "src/app.Config.validate.js"},
	}

	for _, tc := range tests {
		got := companionPath(tc.source, tc.typeName, tc.suffix)
		if got != tc.expected {
			t.Errorf("companionPath(%q, %q, %q) = %q, want %q", tc.source, tc.typeName, tc.suffix, got, tc.expected)
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

	code := GenerateValidation("Person", meta, reg)
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

	code := GenerateValidation("User", meta, reg)
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

	code := GenerateValidation("Post", meta, reg)
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

	code := GenerateValidation("Contact", meta, reg)
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

	code := GenerateValidation("Profile", meta, reg)
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

	code := GenerateValidation("Config", meta, reg)
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

	code := GenerateValidation("Grid", meta, reg)
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

	code := GenerateValidation("Server", meta, reg)
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

	code := GenerateValidation("Counter", meta, reg)
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

	code := GenerateValidation("TagList", meta, reg)
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
	code := GenerateValidation("Server", meta, reg)
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
	code := GenerateValidation("Network", meta, reg)
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
	code := GenerateValidation("Person", meta, reg)
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
	code := GenerateValidation("Event", meta, reg)
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
	code := GenerateValidation("Cache", meta, reg)
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
	code := GenerateValidation("Payload", meta, reg)
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
	code := GenerateValidation("Endpoint", meta, reg)
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
	code := GenerateValidation("JsonRef", meta, reg)
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
	code := GenerateValidation("Auth", meta, reg)
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
	code := GenerateValidation("Filter", meta, reg)
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
	code := GenerateValidation("Link", meta, reg)
	assertContains(t, code, "format uri-template")
	assertContains(t, code, ".test(input.template)")
}

// --- Manifest generation tests ---

func TestManifestBasic(t *testing.T) {
	companions := []CompanionFile{
		{Path: "/project/dist/user.dto.CreateUserDto.validate.js", Content: "..."},
		{Path: "/project/dist/user.dto.CreateUserDto.serialize.js", Content: "..."},
		{Path: "/project/dist/user.dto.UserResponse.validate.js", Content: "..."},
		{Path: "/project/dist/user.dto.UserResponse.serialize.js", Content: "..."},
	}

	m := GenerateManifest(companions, "/project/dist", nil)

	// Check validators
	if len(m.Validators) != 2 {
		t.Fatalf("expected 2 validators, got %d", len(m.Validators))
	}
	if v, ok := m.Validators["CreateUserDto"]; !ok {
		t.Error("missing validator for CreateUserDto")
	} else {
		if v.Fn != "assertCreateUserDto" {
			t.Errorf("validator fn = %q, want %q", v.Fn, "assertCreateUserDto")
		}
		if v.File != "./user.dto.CreateUserDto.validate.js" {
			t.Errorf("validator file = %q, want %q", v.File, "./user.dto.CreateUserDto.validate.js")
		}
	}
	if v, ok := m.Validators["UserResponse"]; !ok {
		t.Error("missing validator for UserResponse")
	} else {
		if v.Fn != "assertUserResponse" {
			t.Errorf("validator fn = %q, want %q", v.Fn, "assertUserResponse")
		}
	}

	// Check serializers
	if len(m.Serializers) != 2 {
		t.Fatalf("expected 2 serializers, got %d", len(m.Serializers))
	}
	if s, ok := m.Serializers["CreateUserDto"]; !ok {
		t.Error("missing serializer for CreateUserDto")
	} else {
		if s.Fn != "serializeCreateUserDto" {
			t.Errorf("serializer fn = %q, want %q", s.Fn, "serializeCreateUserDto")
		}
		if s.File != "./user.dto.CreateUserDto.serialize.js" {
			t.Errorf("serializer file = %q, want %q", s.File, "./user.dto.CreateUserDto.serialize.js")
		}
	}
}

func TestManifestEmpty(t *testing.T) {
	m := GenerateManifest(nil, "/project/dist", nil)

	if len(m.Validators) != 0 {
		t.Errorf("expected 0 validators, got %d", len(m.Validators))
	}
	if len(m.Serializers) != 0 {
		t.Errorf("expected 0 serializers, got %d", len(m.Serializers))
	}
}

func TestManifestJSON(t *testing.T) {
	companions := []CompanionFile{
		{Path: "/dist/dto.User.validate.js", Content: "..."},
		{Path: "/dist/dto.User.serialize.js", Content: "..."},
	}

	m := GenerateManifest(companions, "/dist", nil)
	data, err := ManifestJSON(m)
	if err != nil {
		t.Fatal(err)
	}

	jsonStr := string(data)
	assertContains(t, jsonStr, `"validators"`)
	assertContains(t, jsonStr, `"serializers"`)
	assertContains(t, jsonStr, `"User"`)
	assertContains(t, jsonStr, `"assertUser"`)
	assertContains(t, jsonStr, `"serializeUser"`)
}

func TestManifestRelativePaths(t *testing.T) {
	companions := []CompanionFile{
		{Path: "/project/dist/src/user.dto.Dto.validate.js", Content: "..."},
	}

	m := GenerateManifest(companions, "/project/dist", nil)

	v := m.Validators["Dto"]
	if v.File != "./src/user.dto.Dto.validate.js" {
		t.Errorf("expected relative path ./src/user.dto.Dto.validate.js, got %q", v.File)
	}
}

func TestParseCompanionPath(t *testing.T) {
	tests := []struct {
		path         string
		wantType     string
		wantCategory string
	}{
		{"dist/user.dto.CreateUserDto.validate.js", "CreateUserDto", "validate"},
		{"dist/user.dto.UserResponse.serialize.js", "UserResponse", "serialize"},
		{"dist/app.Config.validate.js", "Config", "validate"},
		{"dist/invalid.js", "", ""},
		{"dist/file.unknown.js", "", ""},
	}

	for _, tc := range tests {
		typeName, category := parseCompanionPath(tc.path)
		if typeName != tc.wantType {
			t.Errorf("parseCompanionPath(%q) typeName = %q, want %q", tc.path, typeName, tc.wantType)
		}
		if category != tc.wantCategory {
			t.Errorf("parseCompanionPath(%q) category = %q, want %q", tc.path, category, tc.wantCategory)
		}
	}
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

	code := GenerateValidation("Dto", meta, reg)
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

	code := GenerateValidation("Dto", meta, reg)
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

	code := GenerateValidation("Dto", meta, reg)
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

	code := GenerateValidation("Dto", meta, reg)
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

	code := GenerateValidation("Dto", meta, reg)
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

	code := GenerateValidation("Dto", meta, reg)
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

	code := GenerateValidation("Dto", meta, reg)
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

	code := GenerateValidation("Dto", meta, reg)
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

	code := GenerateValidation("Dto", meta, reg)
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

	code := GenerateValidation("Dto", meta, reg)
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

	code := GenerateValidation("Dto", meta, reg)
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
			code := GenerateValidation("Dto", meta, reg)
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
			code := GenerateValidation("Dto", meta, reg)
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

	code := GenerateValidation("Dto", meta, reg)
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

	code := GenerateValidation("Dto", meta, reg)
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
	code := GenerateValidation("Dto", meta, reg)
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
	code := GenerateValidation("Dto", meta, reg)
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
	code := GenerateValidation("Dto", meta, reg)
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
	code := GenerateValidation("Dto", meta, reg)
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
	code := GenerateValidation("Dto", meta, reg)
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
	// VisibleDto should have 3 files (validate.js + validate.d.ts + serialize.js)
	visibleCount := 0
	for _, f := range files {
		if strings.Contains(f.Path, "VisibleDto") {
			visibleCount++
		}
	}
	if visibleCount != 3 {
		t.Errorf("expected 3 files for VisibleDto, got %d", visibleCount)
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
	for _, f := range files {
		if strings.Contains(f.Path, "validate") {
			t.Errorf("expected validate file to be excluded, but found: %s", f.Path)
		}
	}
	// Should still have serialize
	serCount := 0
	for _, f := range files {
		if strings.Contains(f.Path, "serialize") {
			serCount++
		}
	}
	if serCount != 1 {
		t.Errorf("expected 1 serialize file, got %d", serCount)
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

	code := GenerateValidation("TestDto", meta, reg)

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

	code := GenerateValidation("Foo", meta, reg)

	// The wrapper should end with }; (closing the const object)
	assertContains(t, code, "};")
	// Should contain the schema comment
	assertContains(t, code, "// Standard Schema v1 wrapper")
	// Should contain path mapping logic
	assertContains(t, code, "e.path.split(\".\").map(k => ({ key: k }))")
}

func TestManifest_SchemaEntries(t *testing.T) {
	companions := []CompanionFile{
		{Path: "/project/dist/user.dto.CreateUserDto.validate.js", Content: "..."},
		{Path: "/project/dist/user.dto.CreateUserDto.serialize.js", Content: "..."},
	}

	m := GenerateManifest(companions, "/project/dist", nil)

	if len(m.Schemas) != 1 {
		t.Fatalf("expected 1 schema entry, got %d", len(m.Schemas))
	}
	entry, ok := m.Schemas["CreateUserDto"]
	if !ok {
		t.Fatal("expected CreateUserDto in schemas")
	}
	if entry.Fn != "schemaCreateUserDto" {
		t.Errorf("expected Fn='schemaCreateUserDto', got %q", entry.Fn)
	}
	if entry.File != "./user.dto.CreateUserDto.validate.js" {
		t.Errorf("expected File='./user.dto.CreateUserDto.validate.js', got %q", entry.File)
	}
}

func TestManifest_SchemaEntriesMultiple(t *testing.T) {
	companions := []CompanionFile{
		{Path: "/project/dist/user.dto.CreateUserDto.validate.js", Content: "..."},
		{Path: "/project/dist/user.dto.CreateUserDto.serialize.js", Content: "..."},
		{Path: "/project/dist/user.dto.UserResponse.validate.js", Content: "..."},
		{Path: "/project/dist/user.dto.UserResponse.serialize.js", Content: "..."},
	}

	m := GenerateManifest(companions, "/project/dist", nil)

	if len(m.Schemas) != 2 {
		t.Fatalf("expected 2 schema entries, got %d", len(m.Schemas))
	}

	if _, ok := m.Schemas["CreateUserDto"]; !ok {
		t.Error("expected CreateUserDto in schemas")
	}
	if _, ok := m.Schemas["UserResponse"]; !ok {
		t.Error("expected UserResponse in schemas")
	}
}

func TestManifest_SchemaInJSON(t *testing.T) {
	companions := []CompanionFile{
		{Path: "/dist/dto.User.validate.js", Content: "..."},
		{Path: "/dist/dto.User.serialize.js", Content: "..."},
	}

	m := GenerateManifest(companions, "/dist", nil)
	data, err := ManifestJSON(m)
	if err != nil {
		t.Fatal(err)
	}

	jsonStr := string(data)
	assertContains(t, jsonStr, `"schemas"`)
	assertContains(t, jsonStr, `"schemaUser"`)
}

func TestManifest_EmptySchemas(t *testing.T) {
	m := GenerateManifest(nil, "/project/dist", nil)
	if len(m.Schemas) != 0 {
		t.Errorf("expected 0 schema entries, got %d", len(m.Schemas))
	}
}

func TestManifest_RouteMap(t *testing.T) {
	companions := []CompanionFile{
		{Path: "/project/dist/user.dto.UserResponse.validate.js", Content: "..."},
		{Path: "/project/dist/user.dto.UserResponse.serialize.js", Content: "..."},
	}

	routeMap := map[string]RouteMapping{
		"UserController.findAll": {ReturnType: "UserResponse", IsArray: true},
		"UserController.findOne": {ReturnType: "UserResponse", IsArray: false},
	}

	m := GenerateManifest(companions, "/project/dist", routeMap)

	if m.Routes == nil {
		t.Fatal("expected routes map, got nil")
	}
	if len(m.Routes) != 2 {
		t.Fatalf("expected 2 route entries, got %d", len(m.Routes))
	}
	if r, ok := m.Routes["UserController.findAll"]; !ok {
		t.Error("missing route UserController.findAll")
	} else {
		if r.ReturnType != "UserResponse" {
			t.Errorf("returnType = %q, want %q", r.ReturnType, "UserResponse")
		}
		if !r.IsArray {
			t.Error("expected IsArray=true for findAll")
		}
	}
	if r, ok := m.Routes["UserController.findOne"]; !ok {
		t.Error("missing route UserController.findOne")
	} else {
		if r.ReturnType != "UserResponse" {
			t.Errorf("returnType = %q, want %q", r.ReturnType, "UserResponse")
		}
		if r.IsArray {
			t.Error("expected IsArray=false for findOne")
		}
	}
}

func TestManifest_RouteMapJSON(t *testing.T) {
	companions := []CompanionFile{
		{Path: "/dist/dto.User.validate.js", Content: "..."},
		{Path: "/dist/dto.User.serialize.js", Content: "..."},
	}

	routeMap := map[string]RouteMapping{
		"UserController.findOne": {ReturnType: "User"},
	}

	m := GenerateManifest(companions, "/dist", routeMap)
	data, err := ManifestJSON(m)
	if err != nil {
		t.Fatal(err)
	}

	jsonStr := string(data)
	assertContains(t, jsonStr, `"routes"`)
	assertContains(t, jsonStr, `"UserController.findOne"`)
	assertContains(t, jsonStr, `"returnType": "User"`)
}

func TestManifest_NilRouteMapOmitted(t *testing.T) {
	companions := []CompanionFile{
		{Path: "/dist/dto.User.validate.js", Content: "..."},
	}

	m := GenerateManifest(companions, "/dist", nil)
	data, err := ManifestJSON(m)
	if err != nil {
		t.Fatal(err)
	}

	jsonStr := string(data)
	// Routes should be omitted from JSON when nil
	if strings.Contains(jsonStr, `"routes"`) {
		t.Error("expected routes to be omitted from JSON when nil")
	}
}

func TestGenerateValidationTypes(t *testing.T) {
	output := GenerateValidationTypes("UserDto")

	assertContains(t, output, "export declare function validateUserDto")
	assertContains(t, output, "export declare function assertUserDto")
	assertContains(t, output, "export declare function deserializeUserDto")
	assertContains(t, output, "export declare const schemaUserDto")
	assertContains(t, output, "StandardSchemaV1Props")
	assertContains(t, output, "readonly version: 1;")
	assertContains(t, output, "readonly vendor: string;")
	assertContains(t, output, "(value: unknown)")
}

func TestGenerateValidationTypes_ReturnTypes(t *testing.T) {
	output := GenerateValidationTypes("Foo")

	// validate returns a discriminated union result
	assertContains(t, output, "success: true; data: Foo")
	assertContains(t, output, "success: false; errors: Array<")
	// assert returns the type directly
	assertContains(t, output, "assertFoo(input: unknown): Foo")
	// deserialize takes a string
	assertContains(t, output, "deserializeFoo(json: string): Foo")
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

	// Should have validate.js, validate.d.ts, and serialize.js
	hasValidateJS := false
	hasValidateDTS := false
	hasSerializeJS := false
	for _, f := range files {
		if strings.HasSuffix(f.Path, ".validate.js") {
			hasValidateJS = true
		}
		if strings.HasSuffix(f.Path, ".validate.d.ts") {
			hasValidateDTS = true
			// Verify the content includes declarations
			assertContains(t, f.Content, "export declare function validateFoo")
			assertContains(t, f.Content, "export declare const schemaFoo")
		}
		if strings.HasSuffix(f.Path, ".serialize.js") {
			hasSerializeJS = true
		}
	}
	if !hasValidateJS {
		t.Error("expected .validate.js companion file")
	}
	if !hasValidateDTS {
		t.Error("expected .validate.d.ts companion file")
	}
	if !hasSerializeJS {
		t.Error("expected .serialize.js companion file")
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
		if strings.HasSuffix(f.Path, ".validate.d.ts") {
			expected := "src/bar.dto.Bar.validate.d.ts"
			if f.Path != expected {
				t.Errorf("expected DTS path %q, got %q", expected, f.Path)
			}
			return
		}
	}
	t.Error("no .validate.d.ts file found")
}

func TestCompanion_NoDTSWhenValidationIgnored(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	types := map[string]*metadata.Metadata{
		"Dto": {Kind: metadata.KindObject, Ignore: "validation", Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		}},
	}

	files := GenerateCompanionFiles("test.ts", types, reg)

	for _, f := range files {
		if strings.HasSuffix(f.Path, ".d.ts") {
			t.Errorf("expected no .d.ts file when validation is ignored, but found: %s", f.Path)
		}
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

	code := GenerateValidation("Dto", meta, reg)
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

	code := GenerateValidation("Dto", meta, reg)
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

	code := GenerateValidation("Dto", meta, reg)
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

	code := GenerateValidation("Dto", meta, reg)
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

	code := GenerateValidation("Dto", meta, reg)
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

	code := GenerateValidation("Dto", meta, reg)
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

	code := GenerateValidation("Dto", meta, reg)
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

	code := GenerateValidation("Dto", meta, reg)
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

	code := GenerateValidation("Dto", meta, reg)
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

	code := GenerateValidation("Dto", meta, reg)
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

	code := GenerateValidation("Dto", meta, reg)
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

	code := GenerateValidation("Dto", meta, reg)
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

	code := GenerateValidation("Dto", meta, reg)
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

	code := GenerateValidation("Dto", meta, reg)
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

	code := GenerateValidation("Dto", meta, reg)
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

	code := GenerateValidation("Payment", meta, reg)
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

	code := GenerateValidation("Contact", meta, reg)
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

	code := GenerateValidation("Order", meta, reg)
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

	code := GenerateValidation("TestDto", meta, reg)
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

	code := GenerateValidation("InputDto", meta, reg)
	// Should strip .ts extension from import path
	assertContains(t, code, `import { validateData } from "./utils/validators";`)
	assertNotContains(t, code, `validators.ts`)
}

// --- Helper ---

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
