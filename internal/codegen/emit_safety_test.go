package codegen

import (
	"strings"
	"testing"

	"github.com/tsgonest/tsgonest/internal/metadata"
)

// =============================================================================
// Emit Safety Tests
//
// These tests verify that code generation produces valid JavaScript for edge-case
// property names, string values, regex patterns, and other ECMAScript quirks.
// =============================================================================

// ---------------------------------------------------------------------------
// 1. jsStringEscape — must handle all JS-problematic characters
// ---------------------------------------------------------------------------

func TestJsStringEscape_Newline(t *testing.T) {
	got := jsStringEscape("line1\nline2")
	want := `line1\nline2`
	if got != want {
		t.Errorf("jsStringEscape(newline) = %q, want %q", got, want)
	}
}

func TestJsStringEscape_CarriageReturn(t *testing.T) {
	got := jsStringEscape("line1\rline2")
	want := `line1\rline2`
	if got != want {
		t.Errorf("jsStringEscape(CR) = %q, want %q", got, want)
	}
}

func TestJsStringEscape_Tab(t *testing.T) {
	got := jsStringEscape("col1\tcol2")
	want := `col1\tcol2`
	if got != want {
		t.Errorf("jsStringEscape(tab) = %q, want %q", got, want)
	}
}

func TestJsStringEscape_NullByte(t *testing.T) {
	got := jsStringEscape("null\x00byte")
	// \x00 or \0 are both acceptable; we use \x00 for control chars
	if !strings.Contains(got, `\x00`) && !strings.Contains(got, `\0`) {
		t.Errorf("jsStringEscape(null byte) = %q, want \\x00 or \\0", got)
	}
	// Must not contain a literal null byte
	if strings.ContainsRune(got, '\x00') {
		t.Error("jsStringEscape(null byte) contains literal null byte")
	}
}

func TestJsStringEscape_LineSeparator(t *testing.T) {
	// U+2028 LINE SEPARATOR — breaks JS string literals pre-ES2019
	got := jsStringEscape("before\u2028after")
	want := `before\u2028after`
	if got != want {
		t.Errorf("jsStringEscape(U+2028) = %q, want %q", got, want)
	}
}

func TestJsStringEscape_ParagraphSeparator(t *testing.T) {
	// U+2029 PARAGRAPH SEPARATOR — breaks JS string literals pre-ES2019
	got := jsStringEscape("before\u2029after")
	want := `before\u2029after`
	if got != want {
		t.Errorf("jsStringEscape(U+2029) = %q, want %q", got, want)
	}
}

func TestJsStringEscape_ControlChars(t *testing.T) {
	// All ASCII control characters < 0x20 (except \n, \r, \t which have named escapes)
	// should be hex-escaped.
	for c := byte(0); c < 0x20; c++ {
		if c == '\n' || c == '\r' || c == '\t' {
			continue // tested separately
		}
		input := string([]byte{c})
		got := jsStringEscape(input)
		if got == input {
			t.Errorf("jsStringEscape(0x%02x) was not escaped, got %q", c, got)
		}
		// Must not contain the raw control character
		if strings.ContainsRune(got, rune(c)) {
			t.Errorf("jsStringEscape(0x%02x) still contains raw control char", c)
		}
	}
}

func TestJsStringEscape_BackslashAndQuote(t *testing.T) {
	// Existing behavior must be preserved
	got := jsStringEscape(`back\slash "quotes"`)
	want := `back\\slash \"quotes\"`
	if got != want {
		t.Errorf("jsStringEscape(backslash+quotes) = %q, want %q", got, want)
	}
}

func TestJsStringEscape_SafeCharsUnchanged(t *testing.T) {
	// Normal alphanumeric + common symbols must pass through unchanged
	input := "hello_world-123 $foo.bar"
	got := jsStringEscape(input)
	if got != input {
		t.Errorf("jsStringEscape(safe chars) = %q, want %q", got, input)
	}
}

// ---------------------------------------------------------------------------
// 2. JSON key escaping for template literals (serialize.go uses template literals)
//
// When serialization emits `{"key":${...}}` inside a JS template literal,
// the key needs two layers of escaping:
//   Layer 1 — JSON: \ → \\, " → \", control chars → \uXXXX
//   Layer 2 — template literal: \ → \\, ` → \`, ${ → \${
// ---------------------------------------------------------------------------

func TestJsonKeyInTemplate_SimpleKey(t *testing.T) {
	got := jsonKeyInTemplate("name")
	if got != "name" {
		t.Errorf("jsonKeyInTemplate(name) = %q, want %q", got, "name")
	}
}

func TestJsonKeyInTemplate_DoubleQuote(t *testing.T) {
	// Property name: foo"bar
	// JSON key content: foo\"bar  (2 chars: \, ")
	// Template escape:  foo\\\"bar (\ → \\, " → \" already from JSON, but the \ needs \\)
	got := jsonKeyInTemplate(`foo"bar`)
	want := `foo\\\"bar`
	if got != want {
		t.Errorf("jsonKeyInTemplate(double quote) = %q, want %q", got, want)
	}
}

func TestJsonKeyInTemplate_Backslash(t *testing.T) {
	// Property name contains literal backslash: foo\bar
	// JSON key: foo\\bar (\ → \\)
	// Template: foo\\\\bar (\\ → \\\\, each \ becomes \\)
	got := jsonKeyInTemplate(`foo\bar`)
	want := `foo\\\\bar`
	if got != want {
		t.Errorf("jsonKeyInTemplate(backslash) = %q, want %q", got, want)
	}
}

func TestJsonKeyInTemplate_Newline(t *testing.T) {
	// Property name with literal newline
	// JSON: \n → \\n in template source
	got := jsonKeyInTemplate("foo\nbar")
	want := `foo\\nbar`
	if got != want {
		t.Errorf("jsonKeyInTemplate(newline) = %q, want %q", got, want)
	}
}

func TestJsonKeyInTemplate_Backtick(t *testing.T) {
	// Backtick would close the template literal — must be escaped
	got := jsonKeyInTemplate("foo`bar")
	if !strings.Contains(got, "\\`") {
		t.Errorf("jsonKeyInTemplate(backtick) = %q, must contain \\`", got)
	}
	// Must not contain raw backtick
	if strings.ContainsRune(got, '`') && !strings.Contains(got, "\\`") {
		t.Error("jsonKeyInTemplate(backtick) has unescaped backtick")
	}
}

func TestJsonKeyInTemplate_DollarBrace(t *testing.T) {
	// ${ would start template interpolation — must be escaped
	got := jsonKeyInTemplate("foo${bar")
	if strings.Contains(got, "${") && !strings.Contains(got, "\\${") {
		t.Errorf("jsonKeyInTemplate(${) = %q, contains unescaped ${", got)
	}
}

func TestJsonKeyInTemplate_Tab(t *testing.T) {
	got := jsonKeyInTemplate("foo\tbar")
	want := `foo\\tbar`
	if got != want {
		t.Errorf("jsonKeyInTemplate(tab) = %q, want %q", got, want)
	}
}

func TestJsonKeyInTemplate_AllSpecialCombined(t *testing.T) {
	// Stress test: property name with multiple problematic characters
	input := "a\"b\\c\nd`e${f"
	got := jsonKeyInTemplate(input)
	// Must not contain any raw problematic characters
	if strings.ContainsRune(got, '\n') {
		t.Error("contains raw newline")
	}
	if strings.ContainsAny(got, "`") {
		// Check it's escaped
		for i, r := range got {
			if r == '`' && (i == 0 || got[i-1] != '\\') {
				t.Error("contains unescaped backtick")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 3. JSON key escaping for string literals (serialize.go optional properties)
//
// When serialization emits ',\"KEY\":' + value as a JS string concatenation,
// the key needs:
//   Layer 1 — JSON: \ → \\, " → \", control chars
//   Layer 2 — JS string: \ → \\, " → \"
// ---------------------------------------------------------------------------

func TestJsonKeyInString_SimpleKey(t *testing.T) {
	got := jsonKeyInString("name")
	if got != "name" {
		t.Errorf("jsonKeyInString(name) = %q, want %q", got, "name")
	}
}

func TestJsonKeyInString_DoubleQuote(t *testing.T) {
	got := jsonKeyInString(`foo"bar`)
	want := `foo\\\"bar`
	if got != want {
		t.Errorf("jsonKeyInString(double quote) = %q, want %q", got, want)
	}
}

func TestJsonKeyInString_Backslash(t *testing.T) {
	got := jsonKeyInString(`foo\bar`)
	want := `foo\\\\bar`
	if got != want {
		t.Errorf("jsonKeyInString(backslash) = %q, want %q", got, want)
	}
}

func TestJsonKeyInString_Newline(t *testing.T) {
	got := jsonKeyInString("foo\nbar")
	want := `foo\\nbar`
	if got != want {
		t.Errorf("jsonKeyInString(newline) = %q, want %q", got, want)
	}
}

func TestJsonKeyInString_Tab(t *testing.T) {
	got := jsonKeyInString("foo\tbar")
	want := `foo\\tbar`
	if got != want {
		t.Errorf("jsonKeyInString(tab) = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// 4. jsObjectKey — __proto__ must use computed property name
// ---------------------------------------------------------------------------

func TestJsObjectKey_Proto(t *testing.T) {
	got := jsObjectKey("__proto__")
	// Must use computed property name to avoid prototype setter
	if got != `["__proto__"]` {
		t.Errorf("jsObjectKey(__proto__) = %q, want %q", got, `["__proto__"]`)
	}
}

func TestJsObjectKey_NormalIdentifier(t *testing.T) {
	got := jsObjectKey("name")
	if got != "name" {
		t.Errorf("jsObjectKey(name) = %q, want %q", got, "name")
	}
}

func TestJsObjectKey_NonIdentifier(t *testing.T) {
	got := jsObjectKey("my key")
	want := `"my key"`
	if got != want {
		t.Errorf("jsObjectKey(my key) = %q, want %q", got, want)
	}
}

func TestJsObjectKey_QuoteInKey(t *testing.T) {
	got := jsObjectKey(`my"key`)
	// Must be quoted and the inner quote escaped
	if !strings.Contains(got, `\"`) {
		t.Errorf("jsObjectKey(my\"key) = %q, must contain escaped quote", got)
	}
}

// ---------------------------------------------------------------------------
// 5. escapeForRegexLiteral — forward slash must be escaped in /pattern/
// ---------------------------------------------------------------------------

func TestEscapeForRegexLiteral_NoSlash(t *testing.T) {
	got := escapeForRegexLiteral(`^[a-z]+$`)
	want := `^[a-z]+$`
	if got != want {
		t.Errorf("escapeForRegexLiteral(no slash) = %q, want %q", got, want)
	}
}

func TestEscapeForRegexLiteral_SingleSlash(t *testing.T) {
	got := escapeForRegexLiteral(`https?://.*`)
	want := `https?:\/\/.*`
	if got != want {
		t.Errorf("escapeForRegexLiteral(url) = %q, want %q", got, want)
	}
}

func TestEscapeForRegexLiteral_AlreadyEscaped(t *testing.T) {
	// Already-escaped slashes must not be double-escaped
	got := escapeForRegexLiteral(`a\/b`)
	want := `a\/b`
	if got != want {
		t.Errorf("escapeForRegexLiteral(already escaped) = %q, want %q", got, want)
	}
}

func TestEscapeForRegexLiteral_MixedSlashes(t *testing.T) {
	// Mix of escaped and unescaped slashes
	got := escapeForRegexLiteral(`a\/b/c`)
	want := `a\/b\/c`
	if got != want {
		t.Errorf("escapeForRegexLiteral(mixed) = %q, want %q", got, want)
	}
}

func TestEscapeForRegexLiteral_TrailingBackslash(t *testing.T) {
	// Trailing backslash should be preserved (even though it's a broken regex)
	got := escapeForRegexLiteral(`abc\`)
	want := `abc\`
	if got != want {
		t.Errorf("escapeForRegexLiteral(trailing backslash) = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// 6. Integration: validate companion with special property names
// ---------------------------------------------------------------------------

func TestEmitSafety_Validate_DoubleQuoteProperty(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "QuoteDto",
		Properties: []metadata.Property{
			{Name: `it's "fine"`, Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}

	code := GenerateCompanionSelective("QuoteDto", meta, reg, true, false)

	// Must use bracket notation with escaped quotes
	assertContains(t, code, `["it's \"fine\""]`)
	// Must NOT contain unescaped quote that would break JS
	assertNotContains(t, code, `input.it's "fine"`)
}

func TestEmitSafety_Validate_NewlineProperty(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "NewlineDto",
		Properties: []metadata.Property{
			{Name: "line\none", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}

	code := GenerateCompanionSelective("NewlineDto", meta, reg, true, false)

	// Must use bracket notation with escaped newline
	assertContains(t, code, `["line\none"]`)
	// Must NOT contain a literal newline inside a string literal
	assertNoRawNewlineInStrings(t, code)
}

func TestEmitSafety_Validate_BackslashProperty(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "BackslashDto",
		Properties: []metadata.Property{
			{Name: `path\to`, Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}

	code := GenerateCompanionSelective("BackslashDto", meta, reg, true, false)

	// Must use bracket notation with escaped backslash
	assertContains(t, code, `["path\\to"]`)
	assertNotContains(t, code, `input.path\to`)
}

func TestEmitSafety_Validate_TabProperty(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "TabDto",
		Properties: []metadata.Property{
			{Name: "col\tcol", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}

	code := GenerateCompanionSelective("TabDto", meta, reg, true, false)
	assertContains(t, code, `["col\tcol"]`)
}

// ---------------------------------------------------------------------------
// 7. Integration: serialize companion with special property names
// ---------------------------------------------------------------------------

func TestEmitSafety_Serialize_DoubleQuoteProperty(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "QuoteDto",
		Properties: []metadata.Property{
			{Name: `say "hi"`, Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}

	code := GenerateCompanionSelective("QuoteDto", meta, reg, false, true)

	// In the serialization template literal, the JSON key must be properly
	// double-escaped so the template evaluates to valid JSON:
	// Template source: \"say \\\"hi\\\"\":${...}
	// Template output: "say \"hi\"":VALUE  (valid JSON key)
	assertContains(t, code, `\\\"`)
	// Must NOT contain a raw unescaped " that would break the JSON key
	assertNotContains(t, code, `\"say "hi"\":`)
}

func TestEmitSafety_Serialize_BackslashProperty(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "BackslashDto",
		Properties: []metadata.Property{
			{Name: `path\to`, Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}

	code := GenerateCompanionSelective("BackslashDto", meta, reg, false, true)

	// The JSON key for "path\to" needs \\, then template-escaped to \\\\
	assertContains(t, code, `\\\\`)
}

func TestEmitSafety_Serialize_NewlineProperty(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "NewlineDto",
		Properties: []metadata.Property{
			{Name: "line\none", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}

	code := GenerateCompanionSelective("NewlineDto", meta, reg, false, true)

	// In the template literal, a newline in the JSON key must be \\n
	assertContains(t, code, `\\n`)
	assertNoRawNewlineInStrings(t, code)
}

func TestEmitSafety_Serialize_BacktickProperty(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "BacktickDto",
		Properties: []metadata.Property{
			{Name: "my`prop", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}

	code := GenerateCompanionSelective("BacktickDto", meta, reg, false, true)

	// Backtick inside a template literal must be escaped as \`
	assertContains(t, code, "\\`")
}

func TestEmitSafety_Serialize_DollarBraceProperty(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "DollarBraceDto",
		Properties: []metadata.Property{
			{Name: "a${b", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}

	code := GenerateCompanionSelective("DollarBraceDto", meta, reg, false, true)

	// ${ inside a template literal must be escaped as \${
	assertContains(t, code, "\\${")
}

func TestEmitSafety_Serialize_OptionalPropertyWithQuote(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "OptQuoteDto",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: `label "x"`, Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Optional: true}, Required: false},
		},
	}

	code := GenerateCompanionSelective("OptQuoteDto", meta, reg, false, true)

	// The optional key literal uses JS string concatenation: ",\"label \\\"x\\\"\":" + value
	// The inner quotes must be double-escaped for JSON-in-JS-string
	assertContains(t, code, `\\\"`)
}

// ---------------------------------------------------------------------------
// 8. Integration: transform companion with special property names
// ---------------------------------------------------------------------------

func TestEmitSafety_Transform_ProtoProperty(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "ProtoDto",
		Properties: []metadata.Property{
			{Name: "__proto__", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}

	code := generateTransformExpr("input", meta, reg, 0, &transformCtx{generating: map[string]bool{}})

	// Must use computed property name ["__proto__"] to avoid prototype setter
	assertContains(t, code, `["__proto__"]`)
	// Must NOT use bare __proto__: which triggers prototype setter in object literals
	assertNotContainsBareProto(t, code)
}

func TestEmitSafety_Transform_QuoteProperty(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "QuoteDto",
		Properties: []metadata.Property{
			{Name: `say "hi"`, Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}

	code := generateTransformExpr("input", meta, reg, 0, &transformCtx{generating: map[string]bool{}})

	// Must use bracket access for property with quotes
	assertContains(t, code, `["say \"hi\""]`)
	// Must use properly escaped object key
	assertContains(t, code, `"say \"hi\""`)
}

func TestEmitSafety_Transform_OptionalProtoProperty(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "OptProtoDto",
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "__proto__", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Optional: true}, Required: false},
		},
	}

	code := generateTransformExpr("input", meta, reg, 0, &transformCtx{generating: map[string]bool{}})

	// Must use computed property name even for optional __proto__
	assertContains(t, code, `["__proto__"]`)
	assertNotContainsBareProto(t, code)
}

// ---------------------------------------------------------------------------
// 9. Integration: is() function with special property names
// ---------------------------------------------------------------------------

func TestEmitSafety_Is_NewlineProperty(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "NewlineDto",
		Properties: []metadata.Property{
			{Name: "line\none", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}

	e := NewEmitter()
	generateIsFunction(e, "NewlineDto", meta, reg, &validateCtx{generating: map[string]bool{}})
	code := e.String()

	// Must use bracket notation with escaped newline
	assertContains(t, code, `["line\none"]`)
	assertNoRawNewlineInStrings(t, code)
}

func TestEmitSafety_Is_DoubleQuoteProperty(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "QuoteDto",
		Properties: []metadata.Property{
			{Name: `it's "fine"`, Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}

	e := NewEmitter()
	generateIsFunction(e, "QuoteDto", meta, reg, &validateCtx{generating: map[string]bool{}})
	code := e.String()

	assertContains(t, code, `["it's \"fine\""]`)
}

// ---------------------------------------------------------------------------
// 10. Regex pattern constraints with slashes
// ---------------------------------------------------------------------------

func TestEmitSafety_Validate_PatternWithSlash(t *testing.T) {
	minLen := 1
	pattern := `https?://.*`
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "UrlDto",
		Properties: []metadata.Property{
			{
				Name:     "url",
				Type:     metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
				Required: true,
				Constraints: &metadata.Constraints{
					MinLength: &minLen,
					Pattern:   &pattern,
				},
			},
		},
	}

	code := GenerateCompanionSelective("UrlDto", meta, reg, true, false)

	// The forward slashes must be escaped in the regex literal:
	// /https?:\/\/.*/ NOT /https?://.*/
	assertContains(t, code, `\/\/`)
	// Must NOT contain unescaped // inside a regex literal (would start a comment)
	assertNotContainsRegexComment(t, code)
}

func TestEmitSafety_Is_PatternWithSlash(t *testing.T) {
	pattern := `^/api/v[0-9]+/.*$`
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "PathDto",
		Properties: []metadata.Property{
			{
				Name:     "path",
				Type:     metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
				Required: true,
				Constraints: &metadata.Constraints{
					Pattern: &pattern,
				},
			},
		},
	}

	e := NewEmitter()
	generateIsFunction(e, "PathDto", meta, reg, &validateCtx{generating: map[string]bool{}})
	code := e.String()

	// Slashes in the pattern must be escaped
	assertContains(t, code, `\/api\/v`)
	assertNotContainsRegexComment(t, code)
}

// ---------------------------------------------------------------------------
// 11. Discriminated union with non-identifier discriminant property
// ---------------------------------------------------------------------------

func TestEmitSafety_Validate_DiscriminantWithSpaces(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "EventDto",
		Properties: []metadata.Property{
			{Name: "event type", Type: metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: "click"}, Required: true},
		},
	}
	reg.Types["ClickEvent"] = meta

	unionMeta := &metadata.Metadata{
		Kind: metadata.KindUnion,
		UnionMembers: []metadata.Metadata{
			{Kind: metadata.KindRef, Ref: "ClickEvent"},
		},
		Discriminant: &metadata.Discriminant{
			Property: "event type",
			Mapping:  map[string]int{"click": 0},
		},
	}

	code := GenerateCompanionSelective("EventUnion", unionMeta, reg, true, false)

	// The discriminant accessor must use bracket notation for "event type"
	assertContains(t, code, `["event type"]`)
	// Must NOT use dot notation for non-identifier
	assertNotContains(t, code, `.event type`)
}

func TestEmitSafety_Serialize_DiscriminantWithQuote(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	objMeta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "QuoteEvent",
		Properties: []metadata.Property{
			{Name: `my"type`, Type: metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: "a"}, Required: true},
			{Name: "data", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}
	reg.Types["QuoteEvent"] = objMeta

	unionMeta := &metadata.Metadata{
		Kind: metadata.KindUnion,
		UnionMembers: []metadata.Metadata{
			{Kind: metadata.KindRef, Ref: "QuoteEvent"},
		},
		Discriminant: &metadata.Discriminant{
			Property: `my"type`,
			Mapping:  map[string]int{"a": 0},
		},
	}

	code := GenerateCompanionSelective("QuoteUnion", unionMeta, reg, false, true)

	// The discriminant property contains a double quote — it must be properly escaped
	// in bracket notation
	assertContains(t, code, `["my\"type"]`)
}

// ---------------------------------------------------------------------------
// 12. Unicode property names (emoji, non-Latin)
// ---------------------------------------------------------------------------

func TestEmitSafety_Validate_UnicodePropertyName(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "UnicodeDto",
		Properties: []metadata.Property{
			{Name: "名前", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "🎉party", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}

	code := GenerateCompanionSelective("UnicodeDto", meta, reg, true, false)

	// Non-ASCII identifiers should use bracket notation since isJSIdentifier
	// only allows ASCII letters
	assertContains(t, code, `["名前"]`)
	assertContains(t, code, `["🎉party"]`)
}

// ---------------------------------------------------------------------------
// 13. Empty string and numeric-start property names
// ---------------------------------------------------------------------------

func TestEmitSafety_Transform_EmptyAndNumericProps(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "WeirdDto",
		Properties: []metadata.Property{
			{Name: "", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "0first", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "123", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "boolean"}, Required: true},
		},
	}

	code := generateTransformExpr("input", meta, reg, 0, &transformCtx{generating: map[string]bool{}})

	// Empty string: must be quoted
	assertContains(t, code, `"": input[""]`)
	// Numeric-start: must be quoted key and bracket access
	assertContains(t, code, `"0first"`)
	assertContains(t, code, `input["0first"]`)
	assertContains(t, code, `"123"`)
	assertContains(t, code, `input["123"]`)
}

// ---------------------------------------------------------------------------
// 14. Comprehensive stress test: all quirky properties in one DTO
// ---------------------------------------------------------------------------

func TestEmitSafety_StressTest_AllQuirkyProperties(t *testing.T) {
	reg := metadata.NewTypeRegistry()
	meta := &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "StressDto",
		Properties: []metadata.Property{
			{Name: "normal", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "with space", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: `with"quote`, Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "with\\backslash", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "with\nnewline", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "with\ttab", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "with`backtick", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "with${dollar", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "__proto__", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "0start", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "名前", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	}

	// Generate validate companion (validate + assert + is)
	validateCode := GenerateCompanionSelective("StressDto", meta, reg, true, false)

	// Generate serialize companion
	serializeCode := GenerateCompanionSelective("StressDto", meta, reg, false, true)

	// Generate transform expression
	transformCode := generateTransformExpr("input", meta, reg, 0, &transformCtx{generating: map[string]bool{}})

	// Basic sanity: generated code must not be empty
	if len(validateCode) == 0 {
		t.Fatal("validate code is empty")
	}
	if len(serializeCode) == 0 {
		t.Fatal("serialize code is empty")
	}
	if len(transformCode) == 0 {
		t.Fatal("transform code is empty")
	}

	// Normal property uses dot notation
	assertContains(t, validateCode, "input.normal")
	assertContains(t, serializeCode, "input.normal")
	assertContains(t, transformCode, "input.normal")

	// Space property uses bracket notation everywhere
	for _, code := range []string{validateCode, serializeCode, transformCode} {
		assertContains(t, code, `["with space"]`)
	}

	// No raw newlines inside string/template literals
	assertNoRawNewlineInStrings(t, validateCode)
	assertNoRawNewlineInStrings(t, serializeCode)

	// __proto__ must use computed key in transform
	assertNotContainsBareProto(t, transformCode)
	assertContains(t, transformCode, `["__proto__"]`)
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func assertNoRawNewlineInStrings(t *testing.T, code string) {
	t.Helper()
	// Check that no JS string literal (between unescaped quotes) contains a raw newline.
	// Simplified check: look for patterns like "...\n..." where \n is a real newline
	// between characters that suggest we're inside a string.
	lines := strings.Split(code, "\n")
	for i, line := range lines {
		// A line that starts inside a string literal would indicate a raw newline
		// breaking a string. Simple heuristic: count unescaped quotes.
		quoteCount := 0
		escaped := false
		for _, c := range line {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				quoteCount++
			}
		}
		// Odd number of unescaped quotes on a line suggests a broken string
		if quoteCount%2 != 0 {
			// Skip lines that are part of template literals (contain backticks)
			if !strings.Contains(line, "`") {
				t.Errorf("line %d has odd number of unescaped quotes (likely broken string from raw newline): %s", i+1, line)
			}
		}
	}
}

func assertNotContainsBareProto(t *testing.T, code string) {
	t.Helper()
	// Check that __proto__ is not used as a bare object literal key.
	// Bare: `__proto__: ` — triggers prototype setter
	// Safe:  `["__proto__"]: ` — computed property name
	// Safe:  `obj.__proto__` or `obj["__proto__"]` — property access (not literal)
	//
	// We look for the pattern: __proto__: (space then colon) that is NOT
	// preceded by a quote (which would mean it's inside ["__proto__"])
	idx := 0
	for {
		pos := strings.Index(code[idx:], "__proto__:")
		if pos == -1 {
			break
		}
		absPos := idx + pos
		// Check if this is preceded by a quote (part of computed key syntax)
		if absPos > 0 && code[absPos-1] == '"' {
			idx = absPos + len("__proto__:")
			continue
		}
		// Check if preceded by dot (property access, not object key)
		if absPos > 0 && code[absPos-1] == '.' {
			idx = absPos + len("__proto__:")
			continue
		}
		t.Errorf("found bare __proto__: as object key at position %d — would trigger prototype setter", absPos)
		break
	}
}

func assertNotContainsRegexComment(t *testing.T, code string) {
	t.Helper()
	// Check for // inside regex literals that would be parsed as a comment.
	// Look for patterns like /...//.../ which is suspicious.
	// A proper regex literal with URL patterns should have \/\/ not //
	lines := strings.Split(code, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip comment lines
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		// Look for regex-like context followed by //
		// Simple heuristic: if a line has /...//.../ it's likely a broken regex
		if idx := strings.Index(trimmed, ".test("); idx > 0 {
			// Find the regex pattern before .test(
			regexPart := trimmed[:idx]
			if strings.Contains(regexPart, "//") {
				t.Errorf("line %d appears to have unescaped // in regex literal: %s", i+1, trimmed)
			}
		}
	}
}
