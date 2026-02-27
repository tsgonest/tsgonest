package sdkgen

import (
	"testing"
)

func TestSchemaToTS_Primitives(t *testing.T) {
	tests := []struct {
		name string
		node *SchemaNode
		want string
	}{
		{"string", &SchemaNode{Type: "string"}, "string"},
		{"number", &SchemaNode{Type: "number"}, "number"},
		{"integer", &SchemaNode{Type: "integer"}, "number"},
		{"boolean", &SchemaNode{Type: "boolean"}, "boolean"},
		{"null", &SchemaNode{Type: "null"}, "null"},
		{"nil node", nil, "unknown"},
		{"unknown type", &SchemaNode{Type: ""}, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SchemaToTS(tt.node, nil)
			if got != tt.want {
				t.Errorf("SchemaToTS() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSchemaToTS_Ref(t *testing.T) {
	node := &SchemaNode{Ref: "UserResponse"}
	got := SchemaToTS(node, nil)
	if got != "UserResponse" {
		t.Errorf("expected UserResponse, got %q", got)
	}
}

func TestSchemaToTS_Array(t *testing.T) {
	node := &SchemaNode{
		Type:  "array",
		Items: &SchemaNode{Ref: "Order"},
	}
	got := SchemaToTS(node, nil)
	if got != "Order[]" {
		t.Errorf("expected Order[], got %q", got)
	}
}

func TestSchemaToTS_ArrayOfPrimitives(t *testing.T) {
	node := &SchemaNode{
		Type:  "array",
		Items: &SchemaNode{Type: "string"},
	}
	got := SchemaToTS(node, nil)
	if got != "string[]" {
		t.Errorf("expected string[], got %q", got)
	}
}

func TestSchemaToTS_Enum(t *testing.T) {
	node := &SchemaNode{
		Enum: []any{"active", "inactive", "archived"},
	}
	got := SchemaToTS(node, nil)
	want := `"active" | "inactive" | "archived"`
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestSchemaToTS_Const(t *testing.T) {
	node := &SchemaNode{Const: "physical"}
	got := SchemaToTS(node, nil)
	if got != `"physical"` {
		t.Errorf("expected \"physical\", got %q", got)
	}
}

func TestSchemaToTS_Object(t *testing.T) {
	node := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"name": {Type: "string"},
			"age":  {Type: "number"},
		},
		Required: []string{"name"},
	}
	got := SchemaToTS(node, nil)
	want := "{ age?: number; name: string }"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestSchemaToTS_Record(t *testing.T) {
	node := &SchemaNode{
		Type:                 "object",
		AdditionalProperties: &SchemaNode{Type: "string"},
	}
	got := SchemaToTS(node, nil)
	want := "Record<string, string>"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestSchemaToTS_AnyOf(t *testing.T) {
	node := &SchemaNode{
		AnyOf: []*SchemaNode{
			{Type: "string"},
			{Type: "number"},
		},
	}
	got := SchemaToTS(node, nil)
	want := "(string | number)"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestSchemaToTS_Nullable(t *testing.T) {
	node := &SchemaNode{
		AnyOf: []*SchemaNode{
			{Type: "string"},
			{Type: "null"},
		},
	}
	got := SchemaToTS(node, nil)
	want := "string | null"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestSchemaToTS_AllOf(t *testing.T) {
	node := &SchemaNode{
		AllOf: []*SchemaNode{
			{Ref: "BaseEntity"},
			{Ref: "UserFields"},
		},
	}
	got := SchemaToTS(node, nil)
	want := "(BaseEntity & UserFields)"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestSchemaToTS_ArrayOfUnion(t *testing.T) {
	node := &SchemaNode{
		Type: "array",
		Items: &SchemaNode{
			AnyOf: []*SchemaNode{
				{Type: "string"},
				{Type: "number"},
			},
		},
	}
	got := SchemaToTS(node, nil)
	want := "(string | number)[]"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestGenerateInterface_Object(t *testing.T) {
	node := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"id":   {Type: "string"},
			"name": {Type: "string"},
			"age":  {Type: "number"},
		},
		Required: []string{"id", "name"},
	}
	got := GenerateInterface("User", node, nil)
	if got == "" {
		t.Fatal("expected non-empty output")
	}
	// Should contain interface declaration
	if !contains(got, "export interface User {") {
		t.Errorf("expected interface declaration, got:\n%s", got)
	}
	// Required fields should not have ?
	if !contains(got, "  id: string;") {
		t.Errorf("expected 'id: string;', got:\n%s", got)
	}
	// Optional fields should have ?
	if !contains(got, "  age?: number;") {
		t.Errorf("expected 'age?: number;', got:\n%s", got)
	}
}

func TestGenerateInterface_Enum(t *testing.T) {
	node := &SchemaNode{
		Type: "string",
		Enum: []any{"active", "inactive"},
	}
	got := GenerateInterface("Status", node, nil)
	if !contains(got, "export type Status = ") {
		t.Errorf("expected type alias for enum, got:\n%s", got)
	}
}

func TestSchemaToTS_Binary(t *testing.T) {
	// format: binary → Blob
	node := &SchemaNode{Type: "string", Format: "binary"}
	got := SchemaToTS(node, nil)
	if got != "Blob" {
		t.Errorf("expected Blob, got %q", got)
	}
}

func TestSchemaToTS_ArrayOfBinary(t *testing.T) {
	// array of format: binary → Blob[]
	node := &SchemaNode{
		Type:  "array",
		Items: &SchemaNode{Type: "string", Format: "binary"},
	}
	got := SchemaToTS(node, nil)
	if got != "Blob[]" {
		t.Errorf("expected Blob[], got %q", got)
	}
}

func TestGenerateInterface_WithJSDoc(t *testing.T) {
	node := &SchemaNode{
		Type:        "object",
		Description: "Represents an order in the system",
		Properties: map[string]*SchemaNode{
			"id":     {Type: "string", Description: "Unique order identifier"},
			"status": {Type: "string", Description: "Current order status"},
			"total":  {Type: "number"},
		},
		Required: []string{"id", "status"},
	}
	got := GenerateInterface("Order", node, nil)

	// Interface-level JSDoc
	if !contains(got, "/** Represents an order in the system */") {
		t.Errorf("expected interface JSDoc, got:\n%s", got)
	}
	// Property-level JSDoc
	if !contains(got, "  /** Unique order identifier */") {
		t.Errorf("expected id property JSDoc, got:\n%s", got)
	}
	if !contains(got, "  /** Current order status */") {
		t.Errorf("expected status property JSDoc, got:\n%s", got)
	}
	// total has no description, should not have JSDoc
	if contains(got, "total */") {
		t.Errorf("total should not have JSDoc, got:\n%s", got)
	}
}

func TestGenerateInterface_TypeAlias_WithJSDoc(t *testing.T) {
	node := &SchemaNode{
		Type:        "string",
		Description: "Order status enum",
		Enum:        []any{"pending", "shipped", "delivered"},
	}
	got := GenerateInterface("OrderStatus", node, nil)

	if !contains(got, "/** Order status enum */") {
		t.Errorf("expected type alias JSDoc, got:\n%s", got)
	}
	if !contains(got, "export type OrderStatus =") {
		t.Errorf("expected type alias, got:\n%s", got)
	}
}

func TestGenerateInterface_MultilineJSDoc(t *testing.T) {
	node := &SchemaNode{
		Type:        "object",
		Description: "A complex type.\nUsed for various purposes.\nHandle with care.",
		Properties: map[string]*SchemaNode{
			"id": {Type: "string"},
		},
		Required: []string{"id"},
	}
	got := GenerateInterface("Complex", node, nil)

	if !contains(got, "/**\n * A complex type.\n * Used for various purposes.\n * Handle with care.\n */") {
		t.Errorf("expected multi-line JSDoc, got:\n%s", got)
	}
}

func TestSchemaToTS_EmptyObject(t *testing.T) {
	// Object with no properties and no additionalProperties
	node := &SchemaNode{Type: "object"}
	got := SchemaToTS(node, nil)
	if got != "Record<string, unknown>" {
		t.Errorf("expected Record<string, unknown>, got %q", got)
	}
}

func TestSchemaToTS_NestedObject(t *testing.T) {
	node := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"address": {
				Type: "object",
				Properties: map[string]*SchemaNode{
					"street": {Type: "string"},
					"city":   {Type: "string"},
				},
				Required: []string{"street"},
			},
		},
		Required: []string{"address"},
	}
	got := SchemaToTS(node, nil)
	// Should produce inline nested object
	if got == "" {
		t.Fatal("expected non-empty output")
	}
	if !contains(got, "address: {") {
		t.Errorf("expected nested object, got %q", got)
	}
	if !contains(got, "street: string") {
		t.Errorf("expected street: string, got %q", got)
	}
}

func TestSchemaToTS_DeeplyNestedArray(t *testing.T) {
	// string[][]
	node := &SchemaNode{
		Type: "array",
		Items: &SchemaNode{
			Type:  "array",
			Items: &SchemaNode{Type: "string"},
		},
	}
	got := SchemaToTS(node, nil)
	if got != "string[][]" {
		t.Errorf("expected string[][], got %q", got)
	}
}

func TestSchemaToTS_MixedEnum(t *testing.T) {
	node := &SchemaNode{
		Enum: []any{"a", float64(1), true, nil},
	}
	got := SchemaToTS(node, nil)
	want := `"a" | 1 | true | null`
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestSchemaToTS_ConstNumber(t *testing.T) {
	node := &SchemaNode{Const: float64(42)}
	got := SchemaToTS(node, nil)
	if got != "42" {
		t.Errorf("expected 42, got %q", got)
	}
}

func TestSchemaToTS_ConstBool(t *testing.T) {
	node := &SchemaNode{Const: true}
	got := SchemaToTS(node, nil)
	if got != "true" {
		t.Errorf("expected true, got %q", got)
	}

	node = &SchemaNode{Const: false}
	got = SchemaToTS(node, nil)
	if got != "false" {
		t.Errorf("expected false, got %q", got)
	}
}

func TestGenerateInterface_PropertyWithSpaces(t *testing.T) {
	node := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"first name":   {Type: "string"},
			"phone number": {Type: "string"},
			"email":        {Type: "string"},
		},
		Required: []string{"email"},
	}
	got := GenerateInterface("ContactInfo", node, nil)

	// Property names with spaces must be quoted
	if !contains(got, `  "first name"?: string;`) {
		t.Errorf("expected quoted 'first name', got:\n%s", got)
	}
	if !contains(got, `  "phone number"?: string;`) {
		t.Errorf("expected quoted 'phone number', got:\n%s", got)
	}
	// Normal identifiers should NOT be quoted
	if !contains(got, "  email: string;") {
		t.Errorf("expected unquoted 'email', got:\n%s", got)
	}
}

func TestSchemaToTS_InlineObjectWithSpaces(t *testing.T) {
	node := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"tax rate": {Type: "number"},
			"currency": {Type: "string"},
		},
		Required: []string{"currency"},
	}
	got := SchemaToTS(node, nil)

	if !contains(got, `"tax rate"?: number`) {
		t.Errorf("expected quoted 'tax rate' in inline object, got %q", got)
	}
	if !contains(got, "currency: string") {
		t.Errorf("expected unquoted 'currency' in inline object, got %q", got)
	}
}

func TestTsPropertyKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"name", "name"},
		{"$ref", "$ref"},
		{"_private", "_private"},
		{"first name", `"first name"`},
		{"phone number", `"phone number"`},
		{"tax rate", `"tax rate"`},
		{"first.name", `"first.name"`},
		{"user-id", `"user-id"`},
		{"filter[name]", `"filter[name]"`},
		{"123start", `"123start"`},
		{"", `""`},
		// Escaping edge cases
		{`say "hello"`, `"say \"hello\""`},
		{`back\slash`, `"back\\slash"`},
		{"new\nline", `"new\nline"`},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := tsPropertyKey(tt.input)
			if got != tt.want {
				t.Errorf("tsPropertyKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTsPropAccess(t *testing.T) {
	tests := []struct {
		accessor string
		name     string
		want     string
	}{
		{"obj", "name", "obj.name"},
		{"obj", "first.name", `obj["first.name"]`},
		{"obj", "user-id", `obj["user-id"]`},
		{"options", "page size", `options["page size"]`},
		{"options", "filter[name]", `options["filter[name]"]`},
		{"obj", `say "hi"`, `obj["say \"hi\""]`},
		{"obj", `back\slash`, `obj["back\\slash"]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tsPropAccess(tt.accessor, tt.name)
			if got != tt.want {
				t.Errorf("tsPropAccess(%q, %q) = %q, want %q", tt.accessor, tt.name, got, tt.want)
			}
		})
	}
}

func TestTsOptionalAccess(t *testing.T) {
	tests := []struct {
		accessor string
		name     string
		want     string
	}{
		{"obj", "name", "obj?.name"},
		{"obj", "first.name", `obj?.["first.name"]`},
		{"obj", "user-id", `obj?.["user-id"]`},
		{"options.query", "page size", `options.query?.["page size"]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tsOptionalAccess(tt.accessor, tt.name)
			if got != tt.want {
				t.Errorf("tsOptionalAccess(%q, %q) = %q, want %q", tt.accessor, tt.name, got, tt.want)
			}
		})
	}
}

// TestGenerateInterface_RecordWithAdditionalProperties verifies that an object type
// with additionalProperties but no declared properties is emitted as an interface
// with an index signature (not a type alias with Record<>). This is critical for
// self-referential types like Prisma's JsonObject where a type alias would cause
// TS2456: "Type alias circularly references itself".
func TestGenerateInterface_RecordWithAdditionalProperties(t *testing.T) {
	node := &SchemaNode{
		Type: "object",
		AdditionalProperties: &SchemaNode{
			AnyOf: []*SchemaNode{
				{Type: "string"},
				{Type: "number"},
				{Ref: "FlexObject"}, // self-reference
			},
		},
	}
	got := GenerateInterface("FlexObject", node, nil)
	// Should emit interface, NOT type alias
	if contains(got, "export type FlexObject") {
		t.Errorf("should NOT emit type alias (causes TS2456 for self-referential types), got:\n%s", got)
	}
	if !contains(got, "export interface FlexObject") {
		t.Errorf("expected interface declaration, got:\n%s", got)
	}
	if !contains(got, "[key: string]:") {
		t.Errorf("expected index signature, got:\n%s", got)
	}
}

// TestGenerateInterface_RecordNonSelfReferential verifies that object types
// with additionalProperties still emit correctly (as interface with index signature).
func TestGenerateInterface_RecordNonSelfReferential(t *testing.T) {
	node := &SchemaNode{
		Type:                 "object",
		AdditionalProperties: &SchemaNode{Type: "string"},
	}
	got := GenerateInterface("Config", node, nil)
	// Should emit interface with index signature
	if !contains(got, "export interface Config") {
		t.Errorf("expected interface declaration, got:\n%s", got)
	}
	if !contains(got, "[key: string]: string") {
		t.Errorf("expected index signature with string value, got:\n%s", got)
	}
}

// TestGenerateInterface_RecordWithDescription verifies JSDoc is preserved.
func TestGenerateInterface_RecordWithDescription(t *testing.T) {
	node := &SchemaNode{
		Type:                 "object",
		Description:          "A flexible key-value store",
		AdditionalProperties: &SchemaNode{Type: "number"},
	}
	got := GenerateInterface("Metrics", node, nil)
	if !contains(got, "A flexible key-value store") {
		t.Errorf("expected JSDoc description, got:\n%s", got)
	}
	if !contains(got, "export interface Metrics") {
		t.Errorf("expected interface declaration, got:\n%s", got)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
