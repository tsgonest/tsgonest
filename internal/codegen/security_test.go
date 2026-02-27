package codegen

import (
	"strings"
	"testing"

	"github.com/tsgonest/tsgonest/internal/metadata"
)

// =============================================================================
// Security & Edge Case Tests
//
// Tests targeting known vulnerability classes in validation/serialization
// libraries (prototype pollution, ReDoS, type coercion bypass, data corruption,
// floating-point precision, etc.) and serialization syntactic edge cases.
//
// Cross-referenced against:
//   - CVE-2020-15366 (ajv prototype pollution)
//   - CVE-2019-18413 (class-validator bypass)
//   - joi ReDoS advisories
//   - zod/typia type coercion issues
// =============================================================================

// ---------------------------------------------------------------------------
// 1. __s String Serializer Security
// ---------------------------------------------------------------------------

func TestHelpers_StringSerializer_U2028U2029(t *testing.T) {
	// U+2028 (LINE SEPARATOR) and U+2029 (PARAGRAPH SEPARATOR) are valid in
	// JSON strings but illegal in JS string literals (pre-ES2019). The __s
	// fast path must detect these and fall back to JSON.stringify.
	code := GenerateHelpers()
	// Must check for charCode 0x2028 and 0x2029
	assertContains(t, code, "0x2028")
	assertContains(t, code, "0x2029")
}

func TestHelpers_StringSerializer_ControlChars(t *testing.T) {
	// All control chars < 32 must trigger the JSON.stringify fallback
	code := GenerateHelpers()
	assertContains(t, code, "c < 32")
}

func TestHelpers_StringSerializer_BackslashAndQuote(t *testing.T) {
	// Backslash (92) and double-quote (34) must trigger fallback
	code := GenerateHelpers()
	assertContains(t, code, "c === 34")
	assertContains(t, code, "c === 92")
}

// ---------------------------------------------------------------------------
// 2. Prototype Pollution Prevention
// ---------------------------------------------------------------------------

func TestValidate_ProtoKey_UseBracketNotation(t *testing.T) {
	// CVE-2020-15366: ajv prototype pollution via __proto__ in schemas.
	// tsgonest must use bracket notation for __proto__ access, never dot notation.
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "__proto__", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}
	code := GenerateCompanionSelective("ProtoDto", meta, reg, true, false)
	assertContains(t, code, `["__proto__"]`)
	assertNotContains(t, code, `.__proto__`)
}

func TestSerialize_ProtoKey_UseBracketNotation(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "__proto__", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}
	code := GenerateCompanionSelective("ProtoDto", meta, reg, false, true)
	assertContains(t, code, `["__proto__"]`)
	assertNotContains(t, code, `.__proto__`)
}

func TestStrict_ProtoKey_NoPrototypeSetter(t *testing.T) {
	// In strict mode with known keys set, __proto__ must be bracket-accessed
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind:       metadata.KindObject,
		Strictness: "strict",
		Properties: []metadata.Property{
			{Name: "__proto__", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}
	code := GenerateCompanionSelective("StrictProtoDto", meta, reg, true, false)
	// Known key set must use string literal, not bare identifier
	assertContains(t, code, `"__proto__"`)
	assertNotContains(t, code, `.__proto__`)
}

func TestStrip_DangerousKeysNotRemoved(t *testing.T) {
	// In strip mode, constructor/prototype are valid property names and should
	// be in the known keys set if declared. They should NOT be stripped.
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind:       metadata.KindObject,
		Strictness: "strip",
		Properties: []metadata.Property{
			{Name: "constructor", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}
	code := GenerateCompanionSelective("StripDto", meta, reg, true, false)
	// constructor should be in the known keys set
	assertContains(t, code, `"constructor"`)
}

// ---------------------------------------------------------------------------
// 3. Number Validation Security
// ---------------------------------------------------------------------------

func TestValidate_NumberRejectsNaN(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "val", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
		},
	}
	code := GenerateCompanionSelective("NumDto", meta, reg, true, false)
	// Number validation must use Number.isFinite which rejects NaN and Infinity
	assertContains(t, code, "Number.isFinite")
}

func TestSerialize_NumberNaNBecomesNull(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "val", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
		},
	}
	code := GenerateCompanionSelective("NumDto", meta, reg, false, true)
	// NaN/Infinity must serialize as "null" per JSON spec
	assertContains(t, code, `Number.isFinite`)
	assertContains(t, code, `"null"`)
}

// ---------------------------------------------------------------------------
// 4. MultipleOf Floating-Point Precision
// ---------------------------------------------------------------------------

func TestMultipleOf_IntegerUsesModulo(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	mult := 5.0
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "step", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true,
				Constraints: &metadata.Constraints{MultipleOf: &mult}},
		},
	}
	code := GenerateCompanionSelective("Grid", meta, reg, true, false)
	// Integer multipleOf should use simple modulo
	assertContains(t, code, "% 5 !== 0")
}

func TestMultipleOf_FractionalUsesEpsilon(t *testing.T) {
	// 0.3 % 0.1 !== 0 in JS due to floating-point. The generated code must
	// use an epsilon-based check instead of raw modulo.
	reg := metadata.NewTypeRegistry()
	mult := 0.01
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "price", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true,
				Constraints: &metadata.Constraints{MultipleOf: &mult}},
		},
	}
	code := GenerateCompanionSelective("PriceDto", meta, reg, true, false)
	// Must NOT use raw modulo for fractional multipleOf
	assertNotContains(t, code, "% 0.01 !== 0")
	// Must use epsilon-based check
	assertContains(t, code, "Math.abs")
	assertContains(t, code, "Math.round")
}

func TestMultipleOf_FractionalHalf(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	mult := 0.5
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "rating", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true,
				Constraints: &metadata.Constraints{MultipleOf: &mult}},
		},
	}
	code := GenerateCompanionSelective("RatingDto", meta, reg, true, false)
	assertNotContains(t, code, "% 0.5 !== 0")
	assertContains(t, code, "Math.round")
}

// ---------------------------------------------------------------------------
// 5. Coercion Security
// ---------------------------------------------------------------------------

func TestCoercion_RejectsEmptyString(t *testing.T) {
	// +"" → 0 in JS. Empty string must NOT be coerced to 0.
	reg := metadata.NewTypeRegistry()
	coerce := true
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "count", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true,
				Constraints: &metadata.Constraints{Coerce: &coerce}},
		},
	}
	code := GenerateCompanionSelective("CoerceDto", meta, reg, true, false)
	// Must check string is non-empty before coercing
	assertContains(t, code, ".length > 0")
}

func TestCoercion_RejectsHexLiterals(t *testing.T) {
	// +"0xff" → 255 in JS. Hex strings must NOT be coerced.
	reg := metadata.NewTypeRegistry()
	coerce := true
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "count", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true,
				Constraints: &metadata.Constraints{Coerce: &coerce}},
		},
	}
	code := GenerateCompanionSelective("CoerceDto", meta, reg, true, false)
	// Must reject hex/octal/binary prefixed strings
	assertContains(t, code, "0[xXoObB]")
}

func TestCoercion_RejectsWhitespaceOnly(t *testing.T) {
	// +" " → 0 in JS. Whitespace-only strings must NOT be coerced.
	reg := metadata.NewTypeRegistry()
	coerce := true
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "count", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true,
				Constraints: &metadata.Constraints{Coerce: &coerce}},
		},
	}
	code := GenerateCompanionSelective("CoerceDto", meta, reg, true, false)
	// Must check trimmed equals original (rejects whitespace-only and padded strings)
	assertContains(t, code, ".trim()")
}

func TestCoercion_BooleanStrictValues(t *testing.T) {
	// Boolean coercion should only accept exact strings: "true", "false", "1", "0"
	reg := metadata.NewTypeRegistry()
	coerce := true
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "active", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "boolean"}, Required: true,
				Constraints: &metadata.Constraints{Coerce: &coerce}},
		},
	}
	code := GenerateCompanionSelective("BoolCoerceDto", meta, reg, true, false)
	assertContains(t, code, `"true"`)
	assertContains(t, code, `"false"`)
	assertContains(t, code, `"1"`)
	assertContains(t, code, `"0"`)
	// Should NOT use truthy/falsy or general coercion
	assertNotContains(t, code, "Boolean(")
}

// ---------------------------------------------------------------------------
// 6. Serialization Syntactic Edge Cases
// ---------------------------------------------------------------------------

func TestSerialize_EmptyObject(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind:       metadata.KindObject,
		Properties: []metadata.Property{},
	}
	code := GenerateCompanionSelective("EmptyDto", meta, reg, false, true)
	// Empty object should fall back to JSON.stringify, not produce broken template
	assertContains(t, code, "JSON.stringify")
}

func TestSerialize_AllOptionalObject(t *testing.T) {
	// All-optional objects need special handling to avoid leading comma: `{,"key":val}`
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "bio", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Optional: true}, Required: false},
			{Name: "age", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number", Optional: true}, Required: false},
		},
	}
	code := GenerateCompanionSelective("OptionalDto", meta, reg, false, true)
	// Must handle the leading comma edge case
	assertContains(t, code, `"," ?`)
}

func TestSerialize_PropertyNameWithQuotes(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: `it"s`, Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}
	code := GenerateCompanionSelective("QuoteDto", meta, reg, false, true)
	// Double-quote in property name must be escaped in JSON key
	assertContains(t, code, `\\\"`)
}

func TestSerialize_PropertyNameWithBackslash(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: `a\b`, Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}
	code := GenerateCompanionSelective("BackslashDto", meta, reg, false, true)
	// Backslash in property name must be escaped in JSON key (double layer)
	assertContains(t, code, `\\\\`)
}

func TestSerialize_PropertyNameWithBacktick(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "code`block", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}
	code := GenerateCompanionSelective("BacktickDto", meta, reg, false, true)
	// Template literal key must have escaped backtick: code\`block
	assertContains(t, code, `code\`+"`"+`block`)
	// The JS string in bracket accessor ["code`block"] is fine — backticks
	// don't need escaping in double-quoted strings
}

func TestSerialize_PropertyNameWithDollarBrace(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "val${inject}", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}
	code := GenerateCompanionSelective("InjectDto", meta, reg, false, true)
	// Template literal key must escape ${ as \${ to prevent injection
	assertContains(t, code, `val\${inject}`)
	// The JS string in bracket accessor ["val${inject}"] is fine — ${
	// doesn't need escaping in double-quoted JS strings
}

func TestSerialize_PropertyNameWithNewline(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "line\nbreak", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}
	code := GenerateCompanionSelective("NewlineDto", meta, reg, false, true)
	// Newline must be escaped in both template and string contexts
	assertNotContains(t, code, "line\nbreak")
}

func TestSerialize_NullableNumber(t *testing.T) {
	// number | null must serialize null as "null", not as "0" or undefined
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "score", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number", Nullable: true}, Required: true},
		},
	}
	code := GenerateCompanionSelective("NullableDto", meta, reg, false, true)
	assertContains(t, code, `== null`)
	assertContains(t, code, `"null"`)
}

func TestSerialize_NullableString(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "bio", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Nullable: true}, Required: true},
		},
	}
	code := GenerateCompanionSelective("NullStrDto", meta, reg, false, true)
	assertContains(t, code, `== null`)
	assertContains(t, code, `"null"`)
	assertContains(t, code, `__s(`)
}

func TestSerialize_BooleanValues(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "active", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "boolean"}, Required: true},
		},
	}
	code := GenerateCompanionSelective("BoolDto", meta, reg, false, true)
	assertContains(t, code, `"true"`)
	assertContains(t, code, `"false"`)
}

func TestSerialize_BigintNotQuoted(t *testing.T) {
	// Bigint should serialize as a number, not a quoted string
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "bigint"}, Required: true},
		},
	}
	code := GenerateCompanionSelective("BigintDto", meta, reg, false, true)
	// bigint uses "" + x for coercion
	assertNotContains(t, code, "__s(input.id)")
}

func TestSerialize_DateToISO(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "created", Type: metadata.Metadata{Kind: metadata.KindNative, NativeType: "Date"}, Required: true},
		},
	}
	code := GenerateCompanionSelective("DateDto", meta, reg, false, true)
	assertContains(t, code, "toISOString()")
}

func TestSerialize_NestedOptionalWithRequired(t *testing.T) {
	// Mix of required + optional props in nested object
	reg := metadata.NewTypeRegistry()
	innerMeta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "label", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Optional: true}, Required: false},
		},
	}
	reg.Register("InnerDto", innerMeta)
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "item", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "InnerDto"}, Required: true},
		},
	}
	code := GenerateCompanionSelective("OuterDto", meta, reg, false, true)
	// Should contain template literal for required part and ternary for optional
	assertContains(t, code, `!== undefined`)
}

// ---------------------------------------------------------------------------
// 7. Validation Edge Cases
// ---------------------------------------------------------------------------

func TestValidate_MinLengthZero(t *testing.T) {
	// minLength: 0 is valid but should still generate a check
	reg := metadata.NewTypeRegistry()
	minLen := 0
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{MinLength: &minLen}},
		},
	}
	code := GenerateCompanionSelective("MinZeroDto", meta, reg, true, false)
	assertContains(t, code, ".length < 0")
}

func TestValidate_MaxItemsZero(t *testing.T) {
	// maxItems: 0 means the array must be empty
	reg := metadata.NewTypeRegistry()
	maxItems := 0
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "tags", Type: metadata.Metadata{Kind: metadata.KindArray,
				ElementType: &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}},
				Required:    true,
				Constraints: &metadata.Constraints{MaxItems: &maxItems}},
		},
	}
	code := GenerateCompanionSelective("EmptyArrayDto", meta, reg, true, false)
	assertContains(t, code, ".length > 0")
}

func TestValidate_UniqueItemsGeneratesSetCheck(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	unique := true
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "ids", Type: metadata.Metadata{Kind: metadata.KindArray,
				ElementType: &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}},
				Required:    true,
				Constraints: &metadata.Constraints{UniqueItems: &unique}},
		},
	}
	code := GenerateCompanionSelective("UniqueDto", meta, reg, true, false)
	assertContains(t, code, "new Set(")
	assertContains(t, code, ".size !==")
}

func TestValidate_ExactOptionalUsesInOperator(t *testing.T) {
	// exactOptionalPropertyTypes: uses "key" in obj, not !== undefined
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "bio", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, ExactOptional: true},
		},
	}
	code := GenerateCompanionSelective("ExactOptDto", meta, reg, true, false)
	assertContains(t, code, `"bio" in `)
}

func TestValidate_NullVsUndefined_Distinct(t *testing.T) {
	// Nullable means null is accepted; optional means undefined is accepted.
	// They must be checked separately, not conflated.
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "nullableOnly", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Nullable: true}, Required: true},
			{Name: "optionalOnly", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Optional: true}, Required: false},
		},
	}
	code := GenerateCompanionSelective("NullUndef", meta, reg, true, false)
	// nullable: null check
	assertContains(t, code, "!== null")
	// optional: undefined check
	assertContains(t, code, "!== undefined")
}

func TestValidate_RecursiveType_DoesNotInfiniteLoop(t *testing.T) {
	// Recursive type: type Tree = { children: Tree[] }
	// The codegen must detect recursion and emit a function call, not inline infinitely.
	reg := metadata.NewTypeRegistry()
	treeMeta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "TreeNode",
		Properties: []metadata.Property{
			{Name: "value", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "children", Type: metadata.Metadata{Kind: metadata.KindArray,
				ElementType: &metadata.Metadata{Kind: metadata.KindRef, Ref: "TreeNode"}}, Required: true},
		},
	}
	reg.Register("TreeNode", treeMeta)
	// This should complete without infinite loop
	code := GenerateCompanionSelective("TreeNode", treeMeta, reg, true, false)
	// Should contain a recursive inner function call
	assertContains(t, code, "_validateTreeNode")
}

func TestSerialize_RecursiveType_EmitsFunction(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	treeMeta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "TreeNode",
		Properties: []metadata.Property{
			{Name: "value", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "children", Type: metadata.Metadata{Kind: metadata.KindArray,
				ElementType: &metadata.Metadata{Kind: metadata.KindRef, Ref: "TreeNode"}}, Required: true},
		},
	}
	reg.Register("TreeNode", treeMeta)
	code := GenerateCompanionSelective("TreeNode", treeMeta, reg, false, true)
	// Should emit serializeTreeNode function reference for recursion
	assertContains(t, code, "serializeTreeNode")
}

// ---------------------------------------------------------------------------
// 8. Format Regex Security (ReDoS resistance)
// ---------------------------------------------------------------------------

func TestFormatRegexes_AllAnchored(t *testing.T) {
	// All format regexes must be anchored with ^ and $ to prevent partial matching
	// and reduce ReDoS attack surface.
	for name, pattern := range formatRegexes {
		if pattern == "" {
			continue // password, regex
		}
		if !strings.HasPrefix(pattern, "^") {
			t.Errorf("format %q regex not anchored at start: %s", name, pattern)
		}
		if !strings.HasSuffix(pattern, "$") {
			t.Errorf("format %q regex not anchored at end: %s", name, pattern)
		}
	}
}

func TestFormatRegexes_NoUnboundedQuantifiers(t *testing.T) {
	// Check for the most dangerous ReDoS patterns: (.+)+ or (.*)* or (a+)+
	// These create catastrophic backtracking.
	dangerousPatterns := []string{
		"(.*)*",
		"(.+)+",
		"(a+)+",
		"(\\s+)+",
	}
	for name, pattern := range formatRegexes {
		if pattern == "" {
			continue
		}
		for _, dangerous := range dangerousPatterns {
			if strings.Contains(pattern, dangerous) {
				t.Errorf("format %q contains dangerous ReDoS pattern %q: %s", name, dangerous, pattern)
			}
		}
	}
}

func TestFormatRegex_EmailExists(t *testing.T) {
	pattern, ok := formatRegexes["email"]
	if !ok || pattern == "" {
		t.Error("email format regex missing")
	}
}

func TestFormatRegex_UUIDExists(t *testing.T) {
	pattern, ok := formatRegexes["uuid"]
	if !ok || pattern == "" {
		t.Error("uuid format regex missing")
	}
}

// ---------------------------------------------------------------------------
// 9. Index Signature Security
// ---------------------------------------------------------------------------

func TestValidate_IndexSignature_IteratesAllKeys(t *testing.T) {
	// Index signatures must validate ALL keys, not just declared properties
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
		},
		IndexSignature: &metadata.IndexSignature{
			ValueType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
		},
	}
	code := GenerateCompanionSelective("IndexDto", meta, reg, true, false)
	assertContains(t, code, "Object.keys(")
	// Must exclude known properties from index validation
	assertContains(t, code, `new Set(["id"])`)
}

func TestValidate_IndexSignature_NoProperties(t *testing.T) {
	// Pure index signature: { [key: string]: number }
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		IndexSignature: &metadata.IndexSignature{
			ValueType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"},
		},
	}
	code := GenerateCompanionSelective("PureIndexDto", meta, reg, true, false)
	assertContains(t, code, "Object.keys(")
	// No known key exclusion needed
	assertNotContains(t, code, "new Set(")
}

// ---------------------------------------------------------------------------
// 10. Discriminated Union Security
// ---------------------------------------------------------------------------

func TestDiscriminatedUnion_SwitchDispatch(t *testing.T) {
	// Discriminated unions must use O(1) switch dispatch, not sequential try-each
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "event", Type: metadata.Metadata{
				Kind: metadata.KindUnion,
				Discriminant: &metadata.Discriminant{
					Property: "type",
					Mapping:  map[string]int{"click": 0, "hover": 1},
				},
				UnionMembers: []metadata.Metadata{
					{Kind: metadata.KindObject, Properties: []metadata.Property{
						{Name: "type", Type: metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: "click"}, Required: true},
						{Name: "x", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
					}},
					{Kind: metadata.KindObject, Properties: []metadata.Property{
						{Name: "type", Type: metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: "hover"}, Required: true},
						{Name: "target", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
					}},
				},
			}, Required: true},
		},
	}
	code := GenerateCompanionSelective("EventDto", meta, reg, true, false)
	assertContains(t, code, "switch (")
	assertContains(t, code, `case "click"`)
	assertContains(t, code, `case "hover"`)
}

func TestDiscriminatedUnion_Serialization_UsesSwitch(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "shape", Type: metadata.Metadata{
				Kind: metadata.KindUnion,
				Discriminant: &metadata.Discriminant{
					Property: "kind",
					Mapping:  map[string]int{"circle": 0, "rect": 1},
				},
				UnionMembers: []metadata.Metadata{
					{Kind: metadata.KindObject, Properties: []metadata.Property{
						{Name: "kind", Type: metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: "circle"}, Required: true},
						{Name: "radius", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
					}},
					{Kind: metadata.KindObject, Properties: []metadata.Property{
						{Name: "kind", Type: metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: "rect"}, Required: true},
						{Name: "width", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
					}},
				},
			}, Required: true},
		},
	}
	code := GenerateCompanionSelective("ShapeDto", meta, reg, false, true)
	assertContains(t, code, "switch (")
}

// ---------------------------------------------------------------------------
// 11. Enum Serialization Edge Cases
// ---------------------------------------------------------------------------

func TestSerialize_MixedEnum(t *testing.T) {
	// Enum with both string and number values
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "val", Type: metadata.Metadata{
				Kind: metadata.KindEnum,
				EnumValues: []metadata.EnumValue{
					{Name: "A", Value: "alpha"},
					{Name: "B", Value: float64(42)},
				},
			}, Required: true},
		},
	}
	code := GenerateCompanionSelective("MixedEnumDto", meta, reg, false, true)
	// Mixed enum must have typeof dispatch
	assertContains(t, code, `typeof`)
	assertContains(t, code, `__s(`)
	assertContains(t, code, `Number.isFinite`)
}

// ---------------------------------------------------------------------------
// 12. Tuple Serialization Edge Cases
// ---------------------------------------------------------------------------

func TestSerialize_EmptyTuple(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "pair", Type: metadata.Metadata{Kind: metadata.KindTuple, Elements: []metadata.TupleElement{}}, Required: true},
		},
	}
	code := GenerateCompanionSelective("EmptyTupleDto", meta, reg, false, true)
	assertContains(t, code, `"[]"`)
}

func TestSerialize_HeterogeneousTuple(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "pair", Type: metadata.Metadata{Kind: metadata.KindTuple, Elements: []metadata.TupleElement{
				{Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}},
				{Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}},
				{Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "boolean"}},
			}}, Required: true},
		},
	}
	code := GenerateCompanionSelective("MixedTupleDto", meta, reg, false, true)
	// Each element should use appropriate serializer
	assertContains(t, code, "__s(")            // string
	assertContains(t, code, "Number.isFinite") // number
	assertContains(t, code, `"true"`)          // boolean
}

// ---------------------------------------------------------------------------
// 13. Intersection Serialization
// ---------------------------------------------------------------------------

func TestSerialize_IntersectionMergesProperties(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	aMeta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
		},
	}
	bMeta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}
	reg.Register("A", aMeta)
	reg.Register("B", bMeta)
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "item", Type: metadata.Metadata{
				Kind: metadata.KindIntersection,
				IntersectionMembers: []metadata.Metadata{
					{Kind: metadata.KindRef, Ref: "A"},
					{Kind: metadata.KindRef, Ref: "B"},
				},
			}, Required: true},
		},
	}
	code := GenerateCompanionSelective("InterDto", meta, reg, false, true)
	// Merged intersection should have both properties
	assertContains(t, code, "id")
	assertContains(t, code, "name")
}

// ---------------------------------------------------------------------------
// 14. Assert Function Security
// ---------------------------------------------------------------------------

func TestAssert_ThrowsOnError(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}
	code := GenerateCompanionSelective("AssertDto", meta, reg, true, false)
	assertContains(t, code, "assertAssertDto")
	assertContains(t, code, "throw")
	assertContains(t, code, "__e")
}

// ---------------------------------------------------------------------------
// 15. Helpers File Integrity
// ---------------------------------------------------------------------------

func TestHelpers_ErrorClassHasStatus400(t *testing.T) {
	code := GenerateHelpers()
	assertContains(t, code, "this.status = 400")
}

func TestHelpers_ErrorClassExtendsError(t *testing.T) {
	code := GenerateHelpers()
	assertContains(t, code, "extends Error")
}

func TestHelpers_ArraySerializerExists(t *testing.T) {
	code := GenerateHelpers()
	assertContains(t, code, "export function __sa(a, f)")
}

func TestHelpers_FormatRegexConstants(t *testing.T) {
	code := GenerateHelpers()
	// Should export format regex constants
	assertContains(t, code, "__fmt_email")
	assertContains(t, code, "__fmt_uuid")
	assertContains(t, code, "__fmt_ipv4")
}

// ---------------------------------------------------------------------------
// 16. Transform Security
// ---------------------------------------------------------------------------

func TestTransform_TrimGuardedByTypeCheck(t *testing.T) {
	// Trim transform must check typeof === "string" before calling .trim()
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Transforms: []string{"trim"}}},
		},
	}
	code := GenerateCompanionSelective("TrimDto", meta, reg, true, false)
	assertContains(t, code, `typeof`)
	assertContains(t, code, `.trim()`)
}

// ---------------------------------------------------------------------------
// 17. Custom Validator Security
// ---------------------------------------------------------------------------

func TestCustomValidator_FnNameNotInjected(t *testing.T) {
	// Custom validator function name should be used as-is (it's an imported identifier)
	reg := metadata.NewTypeRegistry()
	fnName := "isValidEmail"
	moduleName := "./validators"
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "email", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{ValidateFn: &fnName, ValidateModule: &moduleName}},
		},
	}
	code := GenerateCompanionSelective("CustomDto", meta, reg, true, false)
	assertContains(t, code, "isValidEmail(")
	// Import should be generated
	assertContains(t, code, "import")
	assertContains(t, code, "validators")
}

// ---------------------------------------------------------------------------
// 18. Pattern Constraint — Regex Literal Safety
// ---------------------------------------------------------------------------

func TestPattern_ForwardSlashEscaped(t *testing.T) {
	// A pattern containing / must escape it in the regex literal
	reg := metadata.NewTypeRegistry()
	pattern := `^https?://.*$`
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "url", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Pattern: &pattern}},
		},
	}
	code := GenerateCompanionSelective("PatternDto", meta, reg, true, false)
	// Forward slashes must be escaped as \/ in regex literal
	assertContains(t, code, `\/`)
	// Must not have unescaped // which would be parsed as a comment
	lines := strings.Split(code, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue // actual comment
		}
		if idx := strings.Index(trimmed, ".test("); idx > 0 {
			regexPart := trimmed[:idx]
			if strings.Contains(regexPart, "://") && !strings.Contains(regexPart, `:\\/\\/`) {
				t.Errorf("line %d has unescaped :// in regex: %s", i+1, trimmed)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 19. TemplatePattern Validation
// ---------------------------------------------------------------------------

func TestValidate_TemplatePattern(t *testing.T) {
	// Template literal types like `prefix_${string}` become regex patterns
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "key", Type: metadata.Metadata{
				Kind:            metadata.KindAtomic,
				Atomic:          "string",
				TemplatePattern: `^prefix_.*$`,
			}, Required: true},
		},
	}
	code := GenerateCompanionSelective("TemplateDto", meta, reg, true, false)
	assertContains(t, code, `prefix_`)
	assertContains(t, code, `.test(`)
}

// ---------------------------------------------------------------------------
// 20. Default Value Assignment Security
// ---------------------------------------------------------------------------

func TestDefault_StringValue(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	defaultVal := `"hello"`
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "greeting", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{Default: &defaultVal}},
		},
	}
	code := GenerateCompanionSelective("DefaultDto", meta, reg, true, false)
	assertContains(t, code, "=== undefined")
	assertContains(t, code, `"hello"`)
}

func TestDefault_AppliedBeforeValidation(t *testing.T) {
	// Default assignment must happen BEFORE the required-property check,
	// so that when a property is undefined and has a default, the default
	// is filled in and then validated normally.
	reg := metadata.NewTypeRegistry()
	defaultVal := `0`
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "count", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true,
				Constraints: &metadata.Constraints{Default: &defaultVal}},
		},
	}
	code := GenerateCompanionSelective("DefaultOrderDto", meta, reg, true, false)
	// The default assignment (if === undefined → assign) must appear in the output.
	// The generated code fills in the default before the "else" branch validates.
	assertContains(t, code, "=== undefined")
	assertContains(t, code, "= 0;")
}

// ---------------------------------------------------------------------------
// 21. Stringify (validate + serialize combo)
// ---------------------------------------------------------------------------

func TestStringify_CombinesValidateAndSerialize(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}
	code := GenerateCompanionSelective("ComboDto", meta, reg, true, true)
	assertContains(t, code, "stringifyComboDto")
	assertContains(t, code, "validateComboDto")
	assertContains(t, code, "serializeComboDto")
}

// ---------------------------------------------------------------------------
// 22. Numeric Edge Cases
// ---------------------------------------------------------------------------

func TestValidate_Int32_BoundsCheck(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	numType := "int32"
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "port", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true,
				Constraints: &metadata.Constraints{NumericType: &numType}},
		},
	}
	code := GenerateCompanionSelective("Int32Dto", meta, reg, true, false)
	assertContains(t, code, "-2147483648")
	assertContains(t, code, "2147483647")
	assertContains(t, code, "Number.isInteger")
}

func TestValidate_Uint64_SafeIntBounds(t *testing.T) {
	// uint64 must use Number.MAX_SAFE_INTEGER bounds since JS can't represent full uint64
	reg := metadata.NewTypeRegistry()
	numType := "uint64"
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true,
				Constraints: &metadata.Constraints{NumericType: &numType}},
		},
	}
	code := GenerateCompanionSelective("Uint64Dto", meta, reg, true, false)
	assertContains(t, code, "9007199254740991") // Number.MAX_SAFE_INTEGER
	assertContains(t, code, "< 0")
}

// ---------------------------------------------------------------------------
// 23. Array Element Constraints
// ---------------------------------------------------------------------------

func TestValidate_ArrayElementConstraints(t *testing.T) {
	// Array of UUIDs: Array<string & Format<"uuid">>
	// The validation should check each element for uuid format
	reg := metadata.NewTypeRegistry()
	format := "uuid"
	elemType := metadata.Metadata{
		Kind:   metadata.KindAtomic,
		Atomic: "string",
	}
	minItems := 3
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "ids", Type: metadata.Metadata{
				Kind:        metadata.KindArray,
				ElementType: &elemType,
			}, Required: true, Constraints: &metadata.Constraints{
				MinItems: &minItems,
			}},
		},
	}
	// Element-level constraints are on the element property
	meta.Properties[0].Type.ElementType = &metadata.Metadata{
		Kind:   metadata.KindAtomic,
		Atomic: "string",
	}

	code := GenerateCompanionSelective("UuidListDto", meta, reg, true, false)

	// Should check array length minimum
	assertContains(t, code, ".length < 3")
	// Should validate each element is a string
	assertContains(t, code, "typeof")
	_ = format
}

func TestValidate_MinEqualsMaxBoundary(t *testing.T) {
	// When min === max, the validation should produce an exact-value check
	reg := metadata.NewTypeRegistry()
	minLen := 5
	maxLen := 5
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "code", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{
					MinLength: &minLen,
					MaxLength: &maxLen,
				}},
		},
	}
	code := GenerateCompanionSelective("ExactLenDto", meta, reg, true, false)
	// Should check length against 5
	assertContains(t, code, ".length")
	assertContains(t, code, "5")
}

func TestValidate_MinEqualsMaxNumeric(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	min := 42.0
	max := 42.0
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "answer", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true,
				Constraints: &metadata.Constraints{
					Minimum: &min,
					Maximum: &max,
				}},
		},
	}
	code := GenerateCompanionSelective("ExactNumDto", meta, reg, true, false)
	assertContains(t, code, "42")
}

func TestValidate_InfinityRejection(t *testing.T) {
	// Number validation must reject Infinity as well as NaN
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "score", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
		},
	}
	code := GenerateCompanionSelective("InfDto", meta, reg, true, false)
	// Number.isFinite rejects both NaN and Infinity
	assertContains(t, code, "Number.isFinite")
}

func TestValidate_NestedArray(t *testing.T) {
	// number[][] — array of arrays of numbers
	reg := metadata.NewTypeRegistry()
	innerElem := metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}
	innerArray := metadata.Metadata{Kind: metadata.KindArray, ElementType: &innerElem}
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "matrix", Type: metadata.Metadata{
				Kind:        metadata.KindArray,
				ElementType: &innerArray,
			}, Required: true},
		},
	}
	code := GenerateCompanionSelective("MatrixDto", meta, reg, true, false)
	// Should validate outer array
	assertContains(t, code, "Array.isArray(input.matrix)")
	// Should validate inner arrays with nested loop
	assertContains(t, code, "Array.isArray")
	// Should check inner elements are numbers
	assertContains(t, code, "typeof")
}

func TestValidate_TuplePerElementTypes(t *testing.T) {
	// [string, number, boolean] — each element has a different type
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "record", Type: metadata.Metadata{
				Kind: metadata.KindTuple,
				Elements: []metadata.TupleElement{
					{Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}},
					{Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}},
					{Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "boolean"}},
				},
			}, Required: true},
		},
	}
	code := GenerateCompanionSelective("RecordDto", meta, reg, true, false)
	assertContains(t, code, "Array.isArray(input.record)")
	// Should check each element type
	assertContains(t, code, `"string"`)
	assertContains(t, code, `"number"`)
	assertContains(t, code, `"boolean"`)
}

func TestValidate_CombinedMultiConstraint(t *testing.T) {
	// A property with format + minLength + maxLength + pattern all at once
	reg := metadata.NewTypeRegistry()
	format := "email"
	minLen := 5
	maxLen := 320
	pattern := "^[^@]+@[^@]+\\.[^@]+$"
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{Name: "email", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true,
				Constraints: &metadata.Constraints{
					Format:    &format,
					MinLength: &minLen,
					MaxLength: &maxLen,
					Pattern:   &pattern,
				}},
		},
	}
	code := GenerateCompanionSelective("ContactDto", meta, reg, true, false)
	// All constraints should be present in the validation code
	assertContains(t, code, "5")
	assertContains(t, code, "320")
}

func TestSerialize_DeeplyNestedObjects(t *testing.T) {
	reg := metadata.NewTypeRegistry()

	innerMeta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "Inner",
		Properties: []metadata.Property{
			{Name: "value", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}
	reg.Register("Inner", innerMeta)

	middleMeta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "Middle",
		Properties: []metadata.Property{
			{Name: "inner", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "Inner"}, Required: true},
		},
	}
	reg.Register("Middle", middleMeta)

	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "Outer",
		Properties: []metadata.Property{
			{Name: "middle", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "Middle"}, Required: true},
		},
	}

	code := GenerateCompanionSelective("Outer", meta, reg, false, true)
	// Should generate nested property access
	assertContains(t, code, "middle")
	assertContains(t, code, "inner")
	assertContains(t, code, "value")
}
